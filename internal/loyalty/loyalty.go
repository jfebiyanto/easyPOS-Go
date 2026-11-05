// Package loyalty manages customers, member levels and the points ledger (B5).
package loyalty

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/numbering"
)

var (
	ErrInvalid        = errors.New("data pelanggan tidak valid")
	ErrDuplicatePhone = errors.New("nomor HP sudah terdaftar")
	ErrNotFound       = errors.New("pelanggan tidak ditemukan")
)

var phoneRe = regexp.MustCompile(`^\+?[0-9]{8,15}$`)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// EnsureDefaults creates the default member levels (FR-A2-08) for a company.
func EnsureDefaults(ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, companyID string) error {
	_, err := q.Exec(ctx, `INSERT INTO member_levels (company_id, name, min_points, earn_amount_per_point)
		VALUES ($1,'Silver',0,10000),($1,'Gold',100,8000),($1,'Platinum',500,6000) ON CONFLICT DO NOTHING`, companyID)
	return err
}

type Customer struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Address  string `json:"address"`
	IsMember bool   `json:"is_member"`
	Level    string `json:"level"`
	Points   int    `json:"points"`
	IsActive bool   `json:"is_active"`
}

type Input struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Address  string `json:"address"`
	IsMember bool   `json:"is_member"`
	IsActive *bool  `json:"is_active"`
}

func normPhone(p string) string {
	return strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "").Replace(strings.TrimSpace(p))
}

func (in *Input) normalize() error {
	in.Name, in.Phone = strings.TrimSpace(in.Name), normPhone(in.Phone)
	if in.Name == "" || !phoneRe.MatchString(in.Phone) {
		return ErrInvalid
	}
	return nil
}

func mapErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrDuplicatePhone
	}
	return err
}

const custCols = `c.id::text, c.code, c.name, COALESCE(c.phone,''), COALESCE(c.email,''), COALESCE(c.address,''),
	c.is_member, COALESCE(l.name,''), c.points_balance, c.is_active`

func scan(row pgx.Row) (*Customer, error) {
	var c Customer
	err := row.Scan(&c.ID, &c.Code, &c.Name, &c.Phone, &c.Email, &c.Address, &c.IsMember, &c.Level, &c.Points, &c.IsActive)
	return &c, err
}

func (s *Service) Create(ctx context.Context, a audit.Actor, companyID string, in Input) (*Customer, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	code, err := numbering.Next(ctx, tx, companyID, "customer", "CUS", numbering.Now())
	if err != nil {
		return nil, err
	}
	var levelID *string
	if in.IsMember {
		var id string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM member_levels WHERE company_id=$1 ORDER BY min_points LIMIT 1`, companyID).Scan(&id); err != nil {
			return nil, errors.New("level member belum dikonfigurasi")
		}
		levelID = &id
	}
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO customers (company_id, code, name, phone, email, address, is_member, member_level_id)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8) RETURNING id::text`,
		companyID, code, in.Name, in.Phone, in.Email, in.Address, in.IsMember, levelID).Scan(&id); err != nil {
		return nil, mapErr(err)
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: "customer.create",
		SubjectType: "customer", SubjectID: id, Properties: map[string]any{"code": code, "is_member": in.IsMember}}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, companyID, id)
}

func (s *Service) Get(ctx context.Context, companyID, id string) (*Customer, error) {
	c, err := scan(s.pool.QueryRow(ctx, `SELECT `+custCols+` FROM customers c LEFT JOIN member_levels l ON l.id=c.member_level_id
		WHERE c.id=$1 AND c.company_id=$2`, id, companyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// Search matches phone, code or name; used by the POS customer dialog and the admin list.
func (s *Service) Search(ctx context.Context, companyID, q string) ([]Customer, error) {
	q = strings.TrimSpace(q)
	rows, err := s.pool.Query(ctx, `SELECT `+custCols+` FROM customers c LEFT JOIN member_levels l ON l.id=c.member_level_id
		WHERE c.company_id=$1 AND ($2='' OR c.phone LIKE $2||'%' OR c.code ILIKE $2 OR c.name ILIKE '%'||$2||'%')
		ORDER BY c.name LIMIT 100`, companyID, normPhone(q))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Customer{}
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Service) Update(ctx context.Context, a audit.Actor, companyID, id string, in Input) error {
	if err := in.normalize(); err != nil {
		return err
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var wasMember bool
	if err := tx.QueryRow(ctx, `SELECT is_member FROM customers WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(&wasMember); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE customers SET name=$3, phone=$4, email=NULLIF($5,''), address=NULLIF($6,''), is_member=$7, is_active=$8,
		member_level_id = CASE WHEN $7 AND member_level_id IS NULL THEN
			(SELECT id FROM member_levels WHERE company_id=$2 ORDER BY min_points LIMIT 1) ELSE member_level_id END
		WHERE id=$1 AND company_id=$2`, id, companyID, in.Name, in.Phone, in.Email, in.Address, in.IsMember, active); err != nil {
		return mapErr(err)
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: "customer.update", SubjectType: "customer", SubjectID: id}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// move applies a signed point change inside tx: locks the customer, writes the ledger and re-evaluates
// the level (levels only go up, FR-B5-05). Balance never drops below zero.
func move(ctx context.Context, tx pgx.Tx, customerID, typ string, delta int, refType, refID, note, actorID string) (applied int, err error) {
	var bal int
	var member bool
	if err = tx.QueryRow(ctx, `SELECT points_balance, is_member FROM customers WHERE id=$1 FOR UPDATE`, customerID).Scan(&bal, &member); err != nil {
		return 0, err
	}
	if !member {
		return 0, nil
	}
	if bal+delta < 0 {
		delta = -bal
	}
	if delta == 0 {
		return 0, nil
	}
	bal += delta
	var actor any
	if actorID != "" {
		actor = actorID
	}
	if _, err = tx.Exec(ctx, `INSERT INTO loyalty_ledger (customer_id, type, points, balance_after, ref_type, ref_id, note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::uuid)`, customerID, typ, delta, bal, refType, refID, note, actor); err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, `UPDATE customers c SET points_balance=$2, member_level_id = COALESCE((
			SELECT l.id FROM member_levels l WHERE l.company_id=c.company_id AND l.min_points <= $2
			AND l.min_points >= COALESCE((SELECT min_points FROM member_levels WHERE id=c.member_level_id),0)
			ORDER BY l.min_points DESC LIMIT 1), c.member_level_id) WHERE c.id=$1`, customerID, bal)
	return delta, err
}

// Earn credits points for a completed sale. Rate depends on the customer's level at sale time (FR-A2-09).
func Earn(ctx context.Context, tx pgx.Tx, customerID, saleID, number, actorID string, total decimal.Decimal) (int, error) {
	var per string
	var member bool
	err := tx.QueryRow(ctx, `SELECT l.earn_amount_per_point::text, c.is_member FROM customers c
		JOIN member_levels l ON l.id=c.member_level_id WHERE c.id=$1`, customerID).Scan(&per, &member)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !member) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	rate, _ := decimal.NewFromString(per)
	pts := int(total.Div(rate).Floor().IntPart())
	if pts <= 0 {
		return 0, nil
	}
	return move(ctx, tx, customerID, "earn", pts, "sale", saleID, number, actorID)
}

// Reverse removes points because of a sales return (FR-B1-05).
func Reverse(ctx context.Context, tx pgx.Tx, customerID, returnID, number, actorID string, points int) (int, error) {
	if points <= 0 {
		return 0, nil
	}
	n, err := move(ctx, tx, customerID, "reverse", -points, "sales_return", returnID, number, actorID)
	return -n, err
}

// Adjust is a manual correction by a manager and requires a reason (FR-B5-07).
func (s *Service) Adjust(ctx context.Context, a audit.Actor, companyID, id string, delta int, reason string) error {
	if delta == 0 || strings.TrimSpace(reason) == "" {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT is_member FROM customers WHERE id=$1 AND company_id=$2`, id, companyID).Scan(&ok); err != nil {
		return ErrNotFound
	}
	if !ok {
		return ErrInvalid
	}
	if _, err := move(ctx, tx, id, "adjust", delta, "manual", "", reason, a.ID); err != nil {
		return err
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: "loyalty.adjust", SubjectType: "customer",
		SubjectID: id, Description: reason, Properties: map[string]any{"delta": delta}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReverseVoid takes back the points earned by a sale that was voided.
func ReverseVoid(ctx context.Context, tx pgx.Tx, customerID, saleID, number, actorID string, points int) (int, error) {
	if points <= 0 {
		return 0, nil
	}
	n, err := move(ctx, tx, customerID, "void", -points, "sale", saleID, number, actorID)
	return -n, err
}
