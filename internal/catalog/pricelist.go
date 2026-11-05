package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
)

// Tier is a quantity-break price: from MinQty upwards the unit price is Price.
type Tier struct {
	MinQty string `json:"min_qty"`
	Price  string `json:"price"`
}

type PriceList struct {
	ID             string  `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	EffectiveFrom  *string `json:"effective_from"`
	EffectiveUntil *string `json:"effective_until"`
	IsActive       bool    `json:"is_active"`
	Items          int     `json:"items"`
}

type PriceListInput struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	EffectiveFrom  *string `json:"effective_from"`
	EffectiveUntil *string `json:"effective_until"`
	IsActive       bool    `json:"is_active"`
}

type PriceItem struct {
	ProductID string `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	BasePrice string `json:"base_price"`
	MinQty    string `json:"min_qty"`
	Price     string `json:"price"`
}

type PriceItemInput struct {
	ProductID string `json:"product_id"`
	MinQty    string `json:"min_qty"`
	Price     string `json:"price"`
}

func (in *PriceListInput) normalize() error {
	in.Code, in.Name = strings.TrimSpace(in.Code), strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return ErrInvalid
	}
	for _, d := range []**string{&in.EffectiveFrom, &in.EffectiveUntil} {
		if *d == nil || strings.TrimSpace(**d) == "" {
			*d = nil
			continue
		}
		if _, err := time.Parse("2006-01-02", **d); err != nil {
			return ErrInvalid
		}
	}
	return nil
}

func priceErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return ErrDuplicate
		case "23514", "23503", "22P02":
			return ErrInvalid
		}
	}
	return err
}

const priceListCols = `pl.id::text, pl.code, pl.name, pl.effective_from::text, pl.effective_until::text, pl.is_active,
	(SELECT count(*) FROM price_list_items i WHERE i.price_list_id=pl.id)`

func (s *Service) PriceLists(ctx context.Context, companyID string) ([]PriceList, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+priceListCols+` FROM price_lists pl WHERE pl.company_id=$1 ORDER BY pl.code`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PriceList{}
	for rows.Next() {
		var p PriceList
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.EffectiveFrom, &p.EffectiveUntil, &p.IsActive, &p.Items); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) SavePriceList(ctx context.Context, a audit.Actor, companyID, id string, in PriceListInput) (string, error) {
	if err := in.normalize(); err != nil {
		return "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	action := "price_list.update"
	if id == "" {
		action = "price_list.create"
		err = tx.QueryRow(ctx, `INSERT INTO price_lists (company_id, code, name, effective_from, effective_until, is_active)
			VALUES ($1,$2,$3,$4::date,$5::date,$6) RETURNING id::text`, companyID, in.Code, in.Name, in.EffectiveFrom, in.EffectiveUntil, in.IsActive).Scan(&id)
	} else {
		var n int64
		tag, e := tx.Exec(ctx, `UPDATE price_lists SET code=$3, name=$4, effective_from=$5::date, effective_until=$6::date, is_active=$7
			WHERE id=$1 AND company_id=$2`, id, companyID, in.Code, in.Name, in.EffectiveFrom, in.EffectiveUntil, in.IsActive)
		err = e
		if err == nil {
			if n = tag.RowsAffected(); n == 0 {
				return "", ErrNotFound
			}
		}
	}
	if err != nil {
		return "", priceErr(err)
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: action, SubjectType: "price_list", SubjectID: id,
		Description: in.Code, Properties: map[string]any{"name": in.Name, "active": in.IsActive}}); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

func (s *Service) PriceItems(ctx context.Context, companyID, listID string) ([]PriceItem, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id::text, p.sku, p.name, p.base_price::text, i.min_qty::text, i.price::text
		FROM price_list_items i JOIN price_lists pl ON pl.id=i.price_list_id JOIN products p ON p.id=i.product_id
		WHERE pl.id=$1 AND pl.company_id=$2 ORDER BY p.sku, i.min_qty`, listID, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PriceItem{}
	for rows.Next() {
		var it PriceItem
		if err := rows.Scan(&it.ProductID, &it.SKU, &it.Name, &it.BasePrice, &it.MinQty, &it.Price); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// SetPriceItem upserts one tier of a product (price "" removes it).
func (s *Service) SetPriceItem(ctx context.Context, a audit.Actor, companyID, listID string, in PriceItemInput) error {
	q, err := decimal.NewFromString(strings.TrimSpace(in.MinQty))
	if err != nil || !q.IsPositive() {
		return ErrInvalid
	}
	remove := strings.TrimSpace(in.Price) == ""
	var price decimal.Decimal
	if !remove {
		if price, err = decimal.NewFromString(strings.TrimSpace(in.Price)); err != nil || price.IsNegative() {
			return ErrInvalid
		}
		price = price.Round(2)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM price_lists WHERE id=$1 AND company_id=$2)
		AND EXISTS (SELECT 1 FROM products WHERE id=$3 AND company_id=$2)`, listID, companyID, in.ProductID).Scan(&ok); err != nil {
		return priceErr(err)
	}
	if !ok {
		return ErrNotFound
	}
	if remove {
		_, err = tx.Exec(ctx, `DELETE FROM price_list_items WHERE price_list_id=$1 AND product_id=$2 AND min_qty=$3`, listID, in.ProductID, q.String())
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO price_list_items (price_list_id, product_id, min_qty, price) VALUES ($1,$2,$3,$4)
			ON CONFLICT (price_list_id, product_id, min_qty) DO UPDATE SET price=EXCLUDED.price`, listID, in.ProductID, q.String(), price.String())
	}
	if err != nil {
		return priceErr(err)
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: "price_list.item", SubjectType: "price_list", SubjectID: listID,
		Description: in.ProductID, Properties: map[string]any{"min_qty": q.String(), "price": in.Price}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AssignStorePriceList sets the sales price list of a store (listID "" clears it).
func (s *Service) AssignStorePriceList(ctx context.Context, a audit.Actor, companyID, storeID, listID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lid any
	if listID != "" {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM price_lists WHERE id=$1 AND company_id=$2)`, listID, companyID).Scan(&ok); err != nil || !ok {
			return ErrInvalid
		}
		lid = listID
	}
	tag, err := tx.Exec(ctx, `UPDATE stores SET sales_price_list_id=$3 WHERE id=$1 AND company_id=$2`, storeID, companyID, lid)
	if err != nil {
		return priceErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: "store.price_list", SubjectType: "store", SubjectID: storeID,
		Properties: map[string]any{"price_list_id": listID}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type StorePriceList struct {
	StoreID     string  `json:"store_id"`
	Store       string  `json:"store"`
	PriceListID *string `json:"price_list_id"`
}

func (s *Service) StorePriceLists(ctx context.Context, companyID string) ([]StorePriceList, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, name, sales_price_list_id::text FROM stores WHERE company_id=$1 AND is_active ORDER BY code`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StorePriceList{}
	for rows.Next() {
		var x StorePriceList
		if err := rows.Scan(&x.StoreID, &x.Store, &x.PriceListID); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// storeTiers loads the tiers of the store's active price list, keyed by product id (lowest min_qty first).
func storeTiers(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, storeID, today string) (map[string][]Tier, error) {
	rows, err := q.Query(ctx, `SELECT i.product_id::text, i.min_qty::text, i.price::text
		FROM stores st JOIN price_lists pl ON pl.id=st.sales_price_list_id AND pl.is_active
			AND (pl.effective_from IS NULL OR pl.effective_from <= $2::date) AND (pl.effective_until IS NULL OR pl.effective_until >= $2::date)
		JOIN price_list_items i ON i.price_list_id=pl.id WHERE st.id=$1 ORDER BY i.product_id, i.min_qty`, storeID, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]Tier{}
	for rows.Next() {
		var id string
		var t Tier
		if err := rows.Scan(&id, &t.MinQty, &t.Price); err != nil {
			return nil, err
		}
		out[id] = append(out[id], t)
	}
	return out, rows.Err()
}

// WithTiers attaches the store's quantity-break prices to products for the POS (FR-A7-05).
func (s *Service) WithTiers(ctx context.Context, storeID, today string, list []Product) error {
	tiers, err := storeTiers(ctx, s.pool, storeID, today)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].Tiers = tiers[list[i].ID]
	}
	return nil
}

// UnitPrice is the price a store charges for qty of a product: the highest tier whose min_qty <= qty,
// otherwise the product's base price (expired or missing price lists fall back to base_price).
func UnitPrice(ctx context.Context, tx pgx.Tx, storeID, productID, today string, qty decimal.Decimal, base string) (string, error) {
	var price string
	err := tx.QueryRow(ctx, `SELECT i.price::text FROM stores st
		JOIN price_lists pl ON pl.id=st.sales_price_list_id AND pl.is_active
			AND (pl.effective_from IS NULL OR pl.effective_from <= $4::date) AND (pl.effective_until IS NULL OR pl.effective_until >= $4::date)
		JOIN price_list_items i ON i.price_list_id=pl.id AND i.product_id=$2 AND i.min_qty <= $3
		WHERE st.id=$1 ORDER BY i.min_qty DESC LIMIT 1`, storeID, productID, qty.String(), today).Scan(&price)
	if errors.Is(err, pgx.ErrNoRows) {
		return base, nil
	}
	return price, err
}
