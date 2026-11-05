package sales

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/inventory"
	"github.com/jfebiyanto/easyPOS-Go/internal/loyalty"
	"github.com/jfebiyanto/easyPOS-Go/internal/numbering"
)

var (
	ErrAlreadyVoid = errors.New("transaksi sudah dibatalkan (void)")
	ErrHasReturns  = errors.New("transaksi sudah memiliki retur, tidak bisa di-void")
	ErrVoidDay     = errors.New("void hanya bisa dilakukan pada hari transaksi, gunakan retur untuk hari berikutnya")
)

// Row is one sale in the transaction list.
type Row struct {
	Number   string    `json:"number"`
	SoldAt   time.Time `json:"sold_at"`
	Store    string    `json:"store"`
	Cashier  string    `json:"cashier"`
	Customer string    `json:"customer"`
	Total    string    `json:"total"`
	Status   string    `json:"status"`
	Reason   string    `json:"void_reason,omitempty"`
	CanVoid  bool      `json:"can_void"`
}

// List returns sales between two business dates, newest first. storeID "" means every store.
func (s *Service) List(ctx context.Context, companyID, storeID, from, to string) ([]Row, error) {
	today := numbering.BusinessDate(s.now())
	rows, err := s.pool.Query(ctx, `SELECT s.number, s.sold_at, st.name, u.name, COALESCE(c.name,''), s.total::text, s.status,
			COALESCE(s.void_reason,''), s.business_date::text,
			EXISTS (SELECT 1 FROM sales_returns r WHERE r.original_sale_id=s.id)
		FROM sales s JOIN stores st ON st.id=s.store_id JOIN users u ON u.id=s.cashier_id LEFT JOIN customers c ON c.id=s.customer_id
		WHERE s.company_id=$1 AND ($2='' OR s.store_id::text=$2) AND s.business_date BETWEEN $3::date AND $4::date
		ORDER BY s.sold_at DESC LIMIT 500`, companyID, storeID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Row{}
	for rows.Next() {
		var r Row
		var bd string
		var hasRet bool
		if err := rows.Scan(&r.Number, &r.SoldAt, &r.Store, &r.Cashier, &r.Customer, &r.Total, &r.Status, &r.Reason, &bd, &hasRet); err != nil {
			return nil, err
		}
		r.CanVoid = r.Status == "completed" && !hasRet && bd == today
		out = append(out, r)
	}
	return out, rows.Err()
}

// Void cancels a completed sale of the same business day that has no returns: stock goes back, earned
// points are taken back and the payments are marked void. storeID "" means the actor may void in any store.
func (s *Service) Void(ctx context.Context, a audit.Actor, companyID, storeID, number, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id, status, bd, saleStore string
	var cust *string
	var points int
	if err := tx.QueryRow(ctx, `SELECT id::text, status, business_date::text, store_id::text, customer_id::text, points_earned
		FROM sales WHERE company_id=$1 AND number=$2 AND ($3='' OR store_id::text=$3) FOR UPDATE`,
		companyID, strings.TrimSpace(number), storeID).Scan(&id, &status, &bd, &saleStore, &cust, &points); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != "completed" {
		return ErrAlreadyVoid
	}
	var hasRet bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sales_returns WHERE original_sale_id=$1)`, id).Scan(&hasRet); err != nil {
		return err
	}
	if hasRet {
		return ErrHasReturns
	}
	if bd != numbering.BusinessDate(s.now()) {
		return ErrVoidDay
	}

	rows, err := tx.Query(ctx, `SELECT l.product_id::text, sum(l.qty)::text FROM sale_lines l JOIN products p ON p.id=l.product_id
		WHERE l.sale_id=$1 AND p.track_stock GROUP BY l.product_id ORDER BY l.product_id`, id)
	if err != nil {
		return err
	}
	type back struct {
		product string
		qty     decimal.Decimal
	}
	var backs []back
	for rows.Next() {
		var b back
		var q string
		if err := rows.Scan(&b.product, &q); err != nil {
			rows.Close()
			return err
		}
		b.qty, _ = decimal.NewFromString(q)
		backs = append(backs, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	// Same lock order as checkout and returns: stock rows (sorted by product), then the customer.
	sort.Slice(backs, func(i, j int) bool { return backs[i].product < backs[j].product })
	if len(backs) > 0 {
		ids := make([]string, len(backs))
		for i, b := range backs {
			ids[i] = b.product
		}
		if _, err := inventory.LockBalances(ctx, tx, saleStore, ids); err != nil {
			return err
		}
	}
	var number2 string
	if err := tx.QueryRow(ctx, `SELECT number FROM sales WHERE id=$1`, id).Scan(&number2); err != nil {
		return err
	}
	for _, b := range backs {
		if err := inventory.Record(ctx, tx, saleStore, b.product, "sale_void", b.qty, "sale", id, number2, a.ID); err != nil {
			return err
		}
	}
	taken := 0
	if cust != nil && points > 0 {
		if taken, err = loyalty.ReverseVoid(ctx, tx, *cust, id, number2, a.ID, points); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE sale_payments SET status='void' WHERE sale_id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE sales SET status='void', void_reason=$2, voided_by=$3, voided_at=now() WHERE id=$1`, id, reason, a.ID); err != nil {
		return err
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: companyID, Actor: a, Action: "sale.void", SubjectType: "sale", SubjectID: id,
		Description: number2, Properties: map[string]any{"reason": reason, "points_taken_back": taken}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
