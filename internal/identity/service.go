// Package identity implements authentication, roles and the anti-lockout rule (A1).
package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
)

const (
	RoleOwner        = "owner"
	RoleStoreManager = "store_manager"
	RoleCashier      = "cashier"
)

var (
	ErrInvalidCredentials = errors.New("username atau password salah")
	ErrLastOwner          = errors.New("minimal harus ada satu Owner aktif")
	ErrStoreRequired      = errors.New("kasir dan store manager wajib ditugaskan ke satu gerai")
	ErrNotFound           = errors.New("user tidak ditemukan")
)

type User struct {
	ID        string
	CompanyID string
	StoreID   *string
	Name      string
	Username  string
	Role      string
	IsActive  bool
}

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func HashPassword(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

// dummyHash keeps login timing similar whether or not the username exists.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("x"), bcrypt.DefaultCost)

// Login verifies credentials. Inactive accounts are rejected (FR-A1-05).
func (s *Service) Login(ctx context.Context, username, password string) (*User, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, `SELECT id, company_id, store_id, name, username, role, is_active, password_hash
		FROM users WHERE username = $1`, username).
		Scan(&u.ID, &u.CompanyID, &u.StoreID, &u.Name, &u.Username, &u.Role, &u.IsActive, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || !u.IsActive {
		return nil, ErrInvalidCredentials
	}
	return &u, nil
}

func (s *Service) Get(ctx context.Context, id string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `SELECT id, company_id, store_id, name, username, role, is_active
		FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.CompanyID, &u.StoreID, &u.Name, &u.Username, &u.Role, &u.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

type CreateParams struct {
	CompanyID, Name, Username, Password, Role string
	StoreID                                   *string
}

func (s *Service) Create(ctx context.Context, actor audit.Actor, p CreateParams) (*User, error) {
	if p.Role != RoleOwner && p.StoreID == nil {
		return nil, ErrStoreRequired
	}
	hash, err := HashPassword(p.Password)
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	u := User{CompanyID: p.CompanyID, StoreID: p.StoreID, Name: p.Name, Username: p.Username, Role: p.Role, IsActive: true}
	if err := tx.QueryRow(ctx, `INSERT INTO users (company_id, store_id, name, username, role, password_hash)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, p.CompanyID, p.StoreID, p.Name, p.Username, p.Role, hash).Scan(&u.ID); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: p.CompanyID, Actor: actor, Action: "user.create",
		SubjectType: "user", SubjectID: u.ID, Properties: map[string]any{"role": p.Role, "username": p.Username}}); err != nil {
		return nil, err
	}
	return &u, tx.Commit(ctx)
}

// Update changes role/active state. Any change that would leave zero active
// Owners is rejected (FR-A1-06). Active owner rows are locked in id order first,
// so concurrent demotions serialize without deadlocking.
func (s *Service) Update(ctx context.Context, actor audit.Actor, id, role string, active bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var companyID, curRole string
	var storeID *string
	err = tx.QueryRow(ctx, `SELECT company_id, role, store_id FROM users WHERE id = $1`, id).
		Scan(&companyID, &curRole, &storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if role != RoleOwner && storeID == nil {
		return ErrStoreRequired
	}

	// Lock every active owner of the company in a stable order.
	rows, err := tx.Query(ctx, `SELECT id FROM users WHERE company_id=$1 AND role='owner' AND is_active ORDER BY id FOR UPDATE`, companyID)
	if err != nil {
		return err
	}
	owners := map[string]bool{}
	for rows.Next() {
		var oid string
		if err := rows.Scan(&oid); err != nil {
			rows.Close()
			return err
		}
		owners[oid] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Re-read under lock: another transaction may have changed this user meanwhile.
	var curActive bool
	if err := tx.QueryRow(ctx, `SELECT role, is_active FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&curRole, &curActive); err != nil {
		return err
	}
	if curRole == RoleOwner && curActive && (role != RoleOwner || !active) && len(owners) <= 1 {
		return ErrLastOwner
	}
	return s.apply(ctx, tx, actor, companyID, id, role, active)
}

func (s *Service) apply(ctx context.Context, tx pgx.Tx, actor audit.Actor, companyID, id, role string, active bool) error {
	if _, err := tx.Exec(ctx, `UPDATE users SET role=$2, is_active=$3, updated_at=now() WHERE id=$1`, id, role, active); err != nil {
		return err
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: actor, Action: "user.update",
		SubjectType: "user", SubjectID: id, Properties: map[string]any{"role": role, "is_active": active}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Bootstrap creates the first company and Owner when the users table is empty.
func (s *Service) Bootstrap(ctx context.Context, company, name, username, password string) (bool, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil || n > 0 {
		return false, err
	}
	var companyID string
	if err := s.pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ($1) RETURNING id`, company).Scan(&companyID); err != nil {
		return false, err
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO member_levels (company_id, name, min_points, earn_amount_per_point)
		VALUES ($1,'Silver',0,10000),($1,'Gold',100,8000),($1,'Platinum',500,6000) ON CONFLICT DO NOTHING`, companyID); err != nil {
		return false, err
	}
	_, err := s.Create(ctx, audit.Actor{Name: "bootstrap"}, CreateParams{
		CompanyID: companyID, Name: name, Username: username, Password: password, Role: RoleOwner})
	return err == nil, err
}
