// Package catalog manages units, product groups and the global product master (A4-A6).
package catalog

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/inventory"
)

var (
	ErrInvalid   = errors.New("data tidak valid")
	ErrDuplicate = errors.New("kode, SKU, atau barcode sudah dipakai")
	ErrNotFound  = errors.New("data tidak ditemukan")
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type Ref struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type Product struct {
	ID         string  `json:"id"`
	SKU        string  `json:"sku"`
	Barcode    *string `json:"barcode"`
	Name       string  `json:"name"`
	GroupID    *string `json:"group_id"`
	GroupName  string  `json:"group_name"`
	UnitID     string  `json:"unit_id"`
	UnitCode   string  `json:"unit_code"`
	BasePrice  string  `json:"base_price"`
	TrackStock bool    `json:"track_stock"`
	IsActive   bool    `json:"is_active"`
	OnHand     *string `json:"on_hand,omitempty"`
	Tiers      []Tier  `json:"tiers,omitempty"`
}

func mapErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return ErrDuplicate
		case "23514", "23502", "22P02", "23503":
			return ErrInvalid
		}
	}
	return err
}

func (s *Service) listRefs(ctx context.Context, table, companyID string) ([]Ref, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, code, name FROM `+table+` WHERE company_id=$1 ORDER BY code`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ref{}
	for rows.Next() {
		var r Ref
		if err := rows.Scan(&r.ID, &r.Code, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) createRef(ctx context.Context, table, entity string, actor audit.Actor, companyID, code, name string) (*Ref, error) {
	code, name = strings.TrimSpace(code), strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	r := Ref{Code: code, Name: name}
	if err := tx.QueryRow(ctx, `INSERT INTO `+table+` (company_id, code, name) VALUES ($1,$2,$3) RETURNING id::text`,
		companyID, code, name).Scan(&r.ID); err != nil {
		return nil, mapErr(err)
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: actor, Action: entity + ".create",
		SubjectType: entity, SubjectID: r.ID, Properties: map[string]any{"code": code}}); err != nil {
		return nil, err
	}
	return &r, tx.Commit(ctx)
}

func (s *Service) Units(ctx context.Context, company string) ([]Ref, error) {
	return s.listRefs(ctx, "units", company)
}
func (s *Service) Groups(ctx context.Context, company string) ([]Ref, error) {
	return s.listRefs(ctx, "product_groups", company)
}
func (s *Service) CreateUnit(ctx context.Context, a audit.Actor, company, code, name string) (*Ref, error) {
	return s.createRef(ctx, "units", "unit", a, company, code, name)
}
func (s *Service) CreateGroup(ctx context.Context, a audit.Actor, company, code, name string) (*Ref, error) {
	return s.createRef(ctx, "product_groups", "product_group", a, company, code, name)
}

// List returns products; when storeID is set, on_hand for that store is included.
func (s *Service) List(ctx context.Context, companyID, q string, activeOnly bool, storeID string) ([]Product, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id::text, p.sku, p.barcode, p.name, p.product_group_id::text,
			COALESCE(g.name,''), p.unit_id::text, u.code, p.base_price::text, p.track_stock, p.is_active,
			CASE WHEN $4 = '' THEN NULL ELSE COALESCE(ss.on_hand,0)::text END
		FROM products p
		JOIN units u ON u.id = p.unit_id
		LEFT JOIN product_groups g ON g.id = p.product_group_id
		LEFT JOIN store_stocks ss ON ss.product_id = p.id AND ss.store_id::text = $4
		WHERE p.company_id=$1 AND (NOT $3 OR p.is_active)
		  AND ($2 = '' OR p.sku ILIKE '%'||$2||'%' OR p.name ILIKE '%'||$2||'%' OR p.barcode = $2)
		ORDER BY p.name LIMIT 200`, companyID, q, activeOnly, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.SKU, &p.Barcode, &p.Name, &p.GroupID, &p.GroupName, &p.UnitID, &p.UnitCode,
			&p.BasePrice, &p.TrackStock, &p.IsActive, &p.OnHand); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type ProductInput struct {
	SKU        string  `json:"sku"`
	Barcode    *string `json:"barcode"`
	Name       string  `json:"name"`
	GroupID    *string `json:"group_id"`
	UnitID     string  `json:"unit_id"`
	BasePrice  string  `json:"base_price"`
	TrackStock bool    `json:"track_stock"`
	IsActive   bool    `json:"is_active"`
}

func (in *ProductInput) normalize() error {
	in.SKU, in.Name = strings.TrimSpace(in.SKU), strings.TrimSpace(in.Name)
	if in.Barcode != nil {
		b := strings.TrimSpace(*in.Barcode)
		if b == "" {
			in.Barcode = nil
		} else {
			in.Barcode = &b
		}
	}
	if in.GroupID != nil && *in.GroupID == "" {
		in.GroupID = nil
	}
	price, err := decimal.NewFromString(in.BasePrice)
	if err != nil || price.IsNegative() || in.Name == "" || in.UnitID == "" {
		return ErrInvalid
	}
	in.BasePrice = price.Round(2).String()
	return nil
}

func (s *Service) Create(ctx context.Context, a audit.Actor, companyID string, in ProductInput) (string, error) {
	if err := in.normalize(); err != nil || in.SKU == "" {
		return "", ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO products (company_id, product_group_id, unit_id, sku, barcode, name, base_price, track_stock, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text`,
		companyID, in.GroupID, in.UnitID, in.SKU, in.Barcode, in.Name, in.BasePrice, in.TrackStock, in.IsActive).Scan(&id); err != nil {
		return "", mapErr(err)
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: "product.create",
		SubjectType: "product", SubjectID: id, Properties: map[string]any{"sku": in.SKU, "base_price": in.BasePrice}}); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

// Update edits everything except the SKU, which is immutable (FR-A6-02/03).
func (s *Service) Update(ctx context.Context, a audit.Actor, companyID, id string, in ProductInput) error {
	if err := in.normalize(); err != nil {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var oldPrice string
	if err := tx.QueryRow(ctx, `SELECT base_price::text FROM products WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(&oldPrice); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE products SET product_group_id=$3, unit_id=$4, barcode=$5, name=$6, base_price=$7,
		track_stock=$8, is_active=$9, updated_at=now() WHERE id=$1 AND company_id=$2`,
		id, companyID, in.GroupID, in.UnitID, in.Barcode, in.Name, in.BasePrice, in.TrackStock, in.IsActive); err != nil {
		return mapErr(err)
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: "product.update",
		SubjectType: "product", SubjectID: id, Properties: map[string]any{"old_price": oldPrice, "new_price": in.BasePrice, "is_active": in.IsActive}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Adjust sets the on-hand quantity of a product in a store to qtyAfter via a ledger entry (B3, simplified).
func (s *Service) Adjust(ctx context.Context, a audit.Actor, companyID, storeID, productID, qtyAfter, reason string) error {
	after, err := decimal.NewFromString(qtyAfter)
	if err != nil || after.IsNegative() || strings.TrimSpace(reason) == "" {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products p, stores st WHERE p.id=$1 AND p.company_id=$3 AND st.id=$2 AND st.company_id=$3)`,
		productID, storeID, companyID).Scan(&ok); err != nil || !ok {
		return ErrNotFound
	}
	bal, err := inventory.LockBalances(ctx, tx, storeID, []string{productID})
	if err != nil {
		return err
	}
	delta := after.Sub(bal[productID])
	typ := "adjustment"
	if err := inventory.Record(ctx, tx, storeID, productID, typ, delta, "adjustment", "", reason, a.ID); err != nil {
		return err
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: "stock.adjust", SubjectType: "product",
		SubjectID: productID, Description: reason, Properties: map[string]any{"store_id": storeID, "before": bal[productID].String(), "after": after.String()}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
