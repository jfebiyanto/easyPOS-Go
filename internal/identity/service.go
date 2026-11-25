// Package identity: login, token (kompatibel Sanctum), sesi, dan gerai aktif.
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/db"
)

var (
	ErrInvalidCredentials = errors.New("Username atau password salah, atau akun nonaktif.")
	ErrForbidden          = errors.New("akses ditolak")
	ErrUnauthenticated    = errors.New("tidak terautentikasi")
)

// ClientKinds yang valid saat login.
var ClientKinds = map[string]bool{"backoffice": true, "pos": true, "api": true}

type Service struct {
	Pool *pgxpool.Pool
	Now  func() time.Time // UTC
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool, Now: func() time.Time { return time.Now().UTC() }}
}

type StoreRef struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

type Session struct {
	User        db.User
	TokenID     int64
	ClientKind  string
	ActiveStore *StoreRef
}

type LoginInput struct {
	Username, Password, DeviceName, Client string
}

type LoginResult struct {
	Token   string // "{id}|{plain}"
	Session Session
}

var (
	dummyOnce sync.Once
	dummyHash []byte
)

func hashToken(plain string) string {
	h := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(h[:])
}

func newPlainToken() (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 40)
	limit := big.NewInt(int64(len(alphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), nil
}

func ts(t time.Time) sql.NullTime { return sql.NullTime{Time: t.UTC(), Valid: true} }

func (s *Service) logFailed(ctx context.Context, in LoginInput, u *db.User) {
	e := audit.Entry{Action: "auth.login_failed", ActorName: in.Username, Description: "Login gagal untuk " + in.Username,
		Properties: map[string]any{"client": in.Client}}
	if u != nil {
		e.CompanyID, e.ActorID = u.CompanyID, u.ID
	}
	_ = audit.Log(ctx, s.Pool, e)
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	if !ClientKinds[in.Client] {
		return nil, ErrInvalidCredentials
	}
	q := db.New(s.Pool)
	u, err := q.GetUserByUsername(ctx, in.Username)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	hash := []byte(u.Password)
	if !found {
		// Tetap jalankan bcrypt agar waktu respons tidak membocorkan keberadaan user.
		dummyOnce.Do(func() { dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy"), 12) })
		hash = dummyHash
	}
	pwOK := bcrypt.CompareHashAndPassword(hash, []byte(in.Password)) == nil
	if !found || !pwOK || !u.IsActive {
		if found {
			s.logFailed(ctx, in, &u)
		} else {
			s.logFailed(ctx, in, nil)
		}
		return nil, ErrInvalidCredentials
	}
	if in.Client == "backoffice" && u.Role == RoleCashier {
		return nil, fmt.Errorf("%w: peran ini tidak punya akses backoffice", ErrForbidden)
	}

	storeID := u.StoreID
	if !storeID.Valid {
		if u.Role != RoleOwner {
			return nil, fmt.Errorf("%w: akun belum terhubung ke gerai", ErrForbidden)
		}
		if id, err := q.FirstStoreByCompany(ctx, u.CompanyID); err == nil {
			storeID = uuid.NullUUID{UUID: id, Valid: true}
		}
	}
	plain, err := newPlainToken()
	if err != nil {
		return nil, err
	}
	name := in.DeviceName
	if name == "" {
		name = in.Client
	}
	id, err := q.InsertToken(ctx, db.InsertTokenParams{
		TokenableID: u.ID, Name: name, Token: hashToken(plain), ClientKind: in.Client,
		ActiveStoreID: storeID, CreatedAt: ts(s.Now()),
	})
	if err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.Pool, audit.Entry{CompanyID: u.CompanyID, ActorID: u.ID, ActorName: u.Name, Action: "auth.login",
		SubjectType: "user", SubjectID: u.ID, Description: u.Username + " login", Properties: map[string]any{"client": in.Client}, StoreID: storeID.UUID})
	sess := Session{User: u, TokenID: id, ClientKind: in.Client}
	sess.ActiveStore = s.store(ctx, storeID, u.CompanyID)
	return &LoginResult{Token: strconv.FormatInt(id, 10) + "|" + plain, Session: sess}, nil
}

func (s *Service) store(ctx context.Context, id uuid.NullUUID, company uuid.UUID) *StoreRef {
	if !id.Valid {
		return nil
	}
	st, err := db.New(s.Pool).GetStore(ctx, id.UUID)
	if err != nil || st.CompanyID != company || !st.IsActive {
		return nil
	}
	return &StoreRef{ID: st.ID, Code: st.Code, Name: st.Name}
}

// Authenticate memvalidasi token "{id}|{plain}" dan memuat sesi.
func (s *Service) Authenticate(ctx context.Context, raw string) (*Session, error) {
	idStr, plain, ok := strings.Cut(raw, "|")
	if !ok || plain == "" {
		return nil, ErrUnauthenticated
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	q := db.New(s.Pool)
	t, err := q.GetToken(ctx, id)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	if subtle.ConstantTimeCompare([]byte(hashToken(plain)), []byte(t.Token)) != 1 {
		return nil, ErrUnauthenticated
	}
	now := s.Now()
	if t.ExpiresAt.Valid && !t.ExpiresAt.Time.After(now) {
		return nil, ErrUnauthenticated
	}
	u, err := q.GetUserByID(ctx, t.TokenableID)
	if err != nil || !u.IsActive {
		return nil, ErrUnauthenticated
	}
	_ = q.TouchToken(ctx, db.TouchTokenParams{ID: id, LastUsedAt: ts(now)})
	active := t.ActiveStoreID
	if u.Role != RoleOwner {
		// Non-owner selalu terikat ke gerai users.store_id saat ini (bisa berubah setelah login).
		if active != u.StoreID {
			active = u.StoreID
			if err := q.SetTokenStore(ctx, db.SetTokenStoreParams{ID: id, ActiveStoreID: active, UpdatedAt: ts(now)}); err != nil {
				return nil, err
			}
		}
	}
	st := s.store(ctx, active, u.CompanyID)
	if st == nil && active.Valid {
		// Gerai aktif sudah dihapus/nonaktif: kosongkan.
		if err := q.SetTokenStore(ctx, db.SetTokenStoreParams{ID: id, UpdatedAt: ts(now)}); err != nil {
			return nil, err
		}
	}
	return &Session{User: u, TokenID: id, ClientKind: t.ClientKind, ActiveStore: st}, nil
}

func (s *Service) Logout(ctx context.Context, sess *Session) error {
	if err := db.New(s.Pool).DeleteToken(ctx, sess.TokenID); err != nil {
		return err
	}
	return audit.Log(ctx, s.Pool, audit.Entry{CompanyID: sess.User.CompanyID, ActorID: sess.User.ID, ActorName: sess.User.Name,
		Action: "auth.logout", SubjectType: "user", SubjectID: sess.User.ID, Description: sess.User.Username + " logout", StoreID: StoreID(sess)})
}

// StoreID mengembalikan id gerai aktif, atau uuid.Nil.
func StoreID(sess *Session) uuid.UUID {
	if sess.ActiveStore == nil {
		return uuid.Nil
	}
	return sess.ActiveStore.ID
}

// Accessible: owner = semua gerai aktif perusahaan; lainnya hanya gerainya sendiri.
func (s *Service) Accessible(ctx context.Context, u db.User) ([]StoreRef, error) {
	if u.Role == RoleOwner {
		rows, err := db.New(s.Pool).ListStoresByCompany(ctx, u.CompanyID)
		if err != nil {
			return nil, err
		}
		out := make([]StoreRef, 0, len(rows))
		for _, r := range rows {
			out = append(out, StoreRef{ID: r.ID, Code: r.Code, Name: r.Name})
		}
		return out, nil
	}
	out := []StoreRef{}
	if st := s.store(ctx, u.StoreID, u.CompanyID); st != nil {
		out = append(out, *st)
	}
	return out, nil
}

func (s *Service) SwitchStore(ctx context.Context, sess *Session, storeID uuid.UUID) (*StoreRef, error) {
	u := sess.User
	if u.Role != RoleOwner && (!u.StoreID.Valid || u.StoreID.UUID != storeID) {
		return nil, ErrForbidden
	}
	st := s.store(ctx, uuid.NullUUID{UUID: storeID, Valid: true}, u.CompanyID)
	if st == nil {
		return nil, ErrForbidden
	}
	err := db.New(s.Pool).SetTokenStore(ctx, db.SetTokenStoreParams{ID: sess.TokenID, ActiveStoreID: uuid.NullUUID{UUID: storeID, Valid: true}, UpdatedAt: ts(s.Now())})
	if err != nil {
		return nil, err
	}
	return st, nil
}

type Company struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (s *Service) Company(ctx context.Context, id uuid.UUID) (Company, error) {
	c, err := db.New(s.Pool).GetCompany(ctx, id)
	return Company{ID: c.ID, Name: c.Name}, err
}

// RevokeAll menghapus semua token milik user (dipakai saat nonaktif / reset password).
func RevokeAll(ctx context.Context, q db.DBTX, userID uuid.UUID) error {
	return db.New(q).DeleteUserTokens(ctx, userID)
}
