// Package returns implements sales returns (B1): quantity limits, window, stock back, refund and points reversal.
package returns

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/inventory"
	"github.com/jfebiyanto/easyPOS-Go/internal/loyalty"
	"github.com/jfebiyanto/easyPOS-Go/internal/numbering"
)

var (
	ErrInvalid     = errors.New("data retur tidak valid")
	ErrNotFound    = errors.New("transaksi asal tidak ditemukan")
	ErrVoided      = errors.New("transaksi sudah dibatalkan (void), tidak bisa diretur")
	ErrWindow      = errors.New("melewati batas waktu retur")
	ErrOverReturn  = errors.New("qty retur melebihi sisa yang bisa diretur")
	ErrNothingLeft = errors.New("tidak ada item yang bisa diretur")
)

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, now: time.Now} }

type Actor struct{ ID, Name, CompanyID, StoreID string }

// LookupLine is one original sale line with how much can still be returned and its per-unit refund value.
type LookupLine struct {
	LineID     string `json:"line_id"`
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	Qty        string `json:"qty"`
	Returned   string `json:"returned"`
	Returnable string `json:"returnable"`
	UnitRefund string `json:"unit_refund"`
}

type Lookup struct {
	SaleID       string       `json:"sale_id"`
	Number       string       `json:"number"`
	SoldAt       time.Time    `json:"sold_at"`
	Total        string       `json:"total"`
	Customer     string       `json:"customer"`
	WindowEndsAt time.Time    `json:"window_ends_at"`
	Lines        []LookupLine `json:"lines"`
}

type saleInfo struct {
	id, customerID, customerName string
	number                       string
	total, subtotal              decimal.Decimal
	pointsEarned                 int
	soldAt                       time.Time
	status                       string
	window                       int
}

// load fetches the original sale by receipt number inside the staff member's store.
func load(ctx context.Context, q pgx.Tx, a Actor, number string, lock bool) (*saleInfo, error) {
	var si saleInfo
	var total, sub string
	var cust, custName *string
	sql := `SELECT s.id::text, s.number, s.total::text, s.subtotal::text, s.points_earned, s.sold_at, s.status,
			s.customer_id::text, c.name, co.return_window_days
		FROM sales s JOIN companies co ON co.id=s.company_id LEFT JOIN customers c ON c.id=s.customer_id
		WHERE s.company_id=$1 AND s.store_id=$2 AND s.number=$3`
	if lock {
		sql += ` FOR UPDATE OF s`
	}
	if err := q.QueryRow(ctx, sql, a.CompanyID, a.StoreID, strings.TrimSpace(number)).
		Scan(&si.id, &si.number, &total, &sub, &si.pointsEarned, &si.soldAt, &si.status, &cust, &custName, &si.window); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	si.total, _ = decimal.NewFromString(total)
	si.subtotal, _ = decimal.NewFromString(sub)
	if cust != nil {
		si.customerID = *cust
	}
	if custName != nil {
		si.customerName = *custName
	}
	return &si, nil
}

type lineRow struct {
	id, productID, sku, name string
	qty, returned            decimal.Decimal
	gross, tax               decimal.Decimal
	track                    bool
}

func (l lineRow) returnable() decimal.Decimal { return l.qty.Sub(l.returned) }

// unitRefund = (line total after discount, plus tax when the sale was tax-exclusive) / qty.
func (l lineRow) unitRefund(exclusive bool) decimal.Decimal {
	v := l.gross
	if exclusive {
		v = v.Add(l.tax)
	}
	return v.Div(l.qty)
}

func loadLines(ctx context.Context, tx pgx.Tx, saleID string) ([]lineRow, error) {
	rows, err := tx.Query(ctx, `SELECT l.id::text, l.product_id::text, l.sku_snapshot, l.name_snapshot, l.qty::text,
			COALESCE((SELECT sum(r.qty) FROM sales_return_lines r WHERE r.sale_line_id=l.id),0)::text,
			l.line_total::text, l.tax_amount::text, p.track_stock
		FROM sale_lines l JOIN products p ON p.id=l.product_id WHERE l.sale_id=$1 ORDER BY l.sku_snapshot, l.id`, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []lineRow
	for rows.Next() {
		var l lineRow
		var q, r, g, t string
		if err := rows.Scan(&l.id, &l.productID, &l.sku, &l.name, &q, &r, &g, &t, &l.track); err != nil {
			return nil, err
		}
		l.qty, _ = decimal.NewFromString(q)
		l.returned, _ = decimal.NewFromString(r)
		l.gross, _ = decimal.NewFromString(g)
		l.tax, _ = decimal.NewFromString(t)
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Service) checkWindow(si *saleInfo) error {
	if si.status != "completed" {
		return ErrVoided
	}
	if s.now().After(si.soldAt.AddDate(0, 0, si.window)) {
		return ErrWindow
	}
	return nil
}

// Lookup shows what can still be returned for a receipt number (FR-B1-01/02).
func (s *Service) Lookup(ctx context.Context, a Actor, number string) (*Lookup, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	si, err := load(ctx, tx, a, number, false)
	if err != nil {
		return nil, err
	}
	if err := s.checkWindow(si); err != nil {
		return nil, err
	}
	lines, err := loadLines(ctx, tx, si.id)
	if err != nil {
		return nil, err
	}
	excl := si.total.GreaterThan(si.subtotal)
	out := &Lookup{SaleID: si.id, Number: si.number, SoldAt: si.soldAt, Total: si.total.String(), Customer: si.customerName,
		WindowEndsAt: si.soldAt.AddDate(0, 0, si.window)}
	for _, l := range lines {
		out.Lines = append(out.Lines, LookupLine{LineID: l.id, SKU: l.sku, Name: l.name, Qty: l.qty.String(),
			Returned: l.returned.String(), Returnable: l.returnable().String(), UnitRefund: l.unitRefund(excl).Round(2).String()})
	}
	return out, nil
}

type LineInput struct {
	LineID string `json:"line_id"`
	Qty    string `json:"qty"`
}

type Input struct {
	ClientUUID   string      `json:"client_uuid"`
	SaleNumber   string      `json:"sale_number"`
	Reason       string      `json:"reason"`
	RefundTypeID string      `json:"refund_payment_type_id"`
	Lines        []LineInput `json:"lines"`
}

type Result struct {
	ID             string `json:"id"`
	Number         string `json:"number"`
	SaleNumber     string `json:"sale_number"`
	Total          string `json:"total"`
	PointsDeducted int    `json:"points_deducted"`
	Replayed       bool   `json:"replayed,omitempty"`
}

// Create is idempotent on ClientUUID. The original sale row is locked so two concurrent returns
// cannot together exceed the sold quantity.
func (s *Service) Create(ctx context.Context, a Actor, in Input) (*Result, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if in.ClientUUID == "" || in.Reason == "" || in.RefundTypeID == "" || len(in.Lines) == 0 {
		return nil, ErrInvalid
	}
	if r, err := s.byClientUUID(ctx, a.CompanyID, in.ClientUUID); err != nil || r != nil {
		return r, err
	}
	res, err := s.create(ctx, a, in)
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			if r, e := s.byClientUUID(ctx, a.CompanyID, in.ClientUUID); e == nil && r != nil {
				return r, nil
			}
		case "22P02", "23514", "23503":
			return nil, ErrInvalid
		}
	}
	return res, err
}

func (s *Service) create(ctx context.Context, a Actor, in Input) (*Result, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	si, err := load(ctx, tx, a, in.SaleNumber, true)
	if err != nil {
		return nil, err
	}
	if err := s.checkWindow(si); err != nil {
		return nil, err
	}
	lines, err := loadLines(ctx, tx, si.id)
	if err != nil {
		return nil, err
	}
	byID := map[string]*lineRow{}
	for i := range lines {
		byID[lines[i].id] = &lines[i]
	}
	excl := si.total.GreaterThan(si.subtotal)

	var priorTotal string
	var priorPoints int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(total),0)::text, COALESCE(sum(points_deducted),0) FROM sales_returns WHERE original_sale_id=$1`,
		si.id).Scan(&priorTotal, &priorPoints); err != nil {
		return nil, err
	}
	prior, _ := decimal.NewFromString(priorTotal)

	type pick struct {
		l   *lineRow
		qty decimal.Decimal
		amt decimal.Decimal
	}
	var picks []pick
	seen := map[string]bool{}
	refund := decimal.Zero
	for _, li := range in.Lines {
		q, err := decimal.NewFromString(li.Qty)
		if err != nil || !q.IsPositive() || seen[li.LineID] {
			return nil, ErrInvalid
		}
		seen[li.LineID] = true
		l, ok := byID[li.LineID]
		if !ok {
			return nil, ErrInvalid
		}
		if q.GreaterThan(l.returnable()) {
			return nil, fmt.Errorf("%w: %s (sisa %s)", ErrOverReturn, l.name, l.returnable().String())
		}
		amt := l.unitRefund(excl).Mul(q)
		picks = append(picks, pick{l, q, amt})
		refund = refund.Add(amt)
	}
	refund = refund.Round(2)
	if max := si.total.Sub(prior); refund.GreaterThan(max) { // absorb sub-cent rounding on the last return (FR-B1-07)
		if refund.Sub(max).GreaterThan(decimal.NewFromFloat(0.05)) {
			return nil, ErrOverReturn
		}
		refund = max
	}

	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM payment_types WHERE id=$1 AND company_id=$2 AND is_active)`, in.RefundTypeID, a.CompanyID).Scan(&ok); err != nil || !ok {
		return nil, ErrInvalid
	}

	number, err := numbering.Next(ctx, tx, a.CompanyID, "return", "RTN", s.now())
	if err != nil {
		return nil, err
	}

	// Stock back in (lock balances in stable order first, then customer: same order as checkout).
	sort.Slice(picks, func(i, j int) bool { return picks[i].l.productID < picks[j].l.productID })
	var tracked []string
	for _, p := range picks {
		if p.l.track {
			tracked = append(tracked, p.l.productID)
		}
	}
	if len(tracked) > 0 {
		if _, err := inventory.LockBalances(ctx, tx, a.StoreID, tracked); err != nil {
			return nil, err
		}
	}

	// Points: reverse proportionally to the cumulative value returned, never more than earned.
	pts := 0
	if si.customerID != "" && si.pointsEarned > 0 && si.total.IsPositive() {
		cum := prior.Add(refund)
		target := decimal.NewFromInt(int64(si.pointsEarned)).Mul(cum).Div(si.total).Floor().IntPart()
		pts = int(target) - priorPoints
		if pts < 0 {
			pts = 0
		}
	}

	var retID string
	var cust any
	if si.customerID != "" {
		cust = si.customerID
	}
	if err := tx.QueryRow(ctx, `INSERT INTO sales_returns (company_id, store_id, number, client_uuid, original_sale_id, customer_id, reason, total, refund_payment_type_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id::text`, a.CompanyID, a.StoreID, number, in.ClientUUID, si.id, cust, in.Reason,
		refund.String(), in.RefundTypeID, a.ID).Scan(&retID); err != nil {
		return nil, err
	}
	for _, p := range picks {
		if _, err := tx.Exec(ctx, `INSERT INTO sales_return_lines (sales_return_id, sale_line_id, product_id, qty, line_total) VALUES ($1,$2,$3,$4,$5)`,
			retID, p.l.id, p.l.productID, p.qty.String(), p.amt.Round(2).String()); err != nil {
			return nil, err
		}
		if p.l.track {
			if err := inventory.Record(ctx, tx, a.StoreID, p.l.productID, "sale_return", p.qty, "sales_return", retID, number, a.ID); err != nil {
				return nil, err
			}
		}
	}
	deducted := 0
	if si.customerID != "" && pts > 0 {
		if deducted, err = loyalty.Reverse(ctx, tx, si.customerID, retID, number, a.ID, pts); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE sales_returns SET points_deducted=$2 WHERE id=$1`, retID, deducted); err != nil {
		return nil, err
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: a.CompanyID, Actor: audit.Actor{ID: a.ID, Name: a.Name}, Action: "sale.return",
		SubjectType: "sales_return", SubjectID: retID, Description: number,
		Properties: map[string]any{"sale": si.number, "total": refund.String(), "reason": in.Reason, "points_deducted": deducted}}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Result{ID: retID, Number: number, SaleNumber: si.number, Total: refund.String(), PointsDeducted: deducted}, nil
}

func (s *Service) byClientUUID(ctx context.Context, companyID, clientUUID string) (*Result, error) {
	var r Result
	err := s.pool.QueryRow(ctx, `SELECT r.id::text, r.number, s.number, r.total::text, r.points_deducted FROM sales_returns r
		JOIN sales s ON s.id=r.original_sale_id WHERE r.company_id=$1 AND r.client_uuid=$2`, companyID, clientUUID).
		Scan(&r.ID, &r.Number, &r.SaleNumber, &r.Total, &r.PointsDeducted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Replayed = true
	return &r, nil
}
