// Package sales implements POS checkout (A8): server-side pricing, PPN, stock and numbering in one transaction.
package sales

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/catalog"
	"github.com/jfebiyanto/easyPOS-Go/internal/inventory"
	"github.com/jfebiyanto/easyPOS-Go/internal/loyalty"
	"github.com/jfebiyanto/easyPOS-Go/internal/numbering"
)

var (
	ErrInvalid           = errors.New("transaksi tidak valid")
	ErrInsufficientStock = errors.New("stok tidak cukup")
	ErrUnderpaid         = errors.New("pembayaran kurang dari total")
	ErrNotFound          = errors.New("transaksi tidak ditemukan")
)

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, now: time.Now} }

type Cashier struct {
	ID, Name, CompanyID, StoreID string
}

type LineInput struct {
	ProductID string `json:"product_id"`
	Qty       string `json:"qty"`
	Discount  string `json:"discount"`
}

type PaymentInput struct {
	PaymentTypeID string `json:"payment_type_id"`
	Amount        string `json:"amount"`
}

type CheckoutInput struct {
	ClientUUID string         `json:"client_uuid"`
	CustomerID string         `json:"customer_id"`
	Lines      []LineInput    `json:"lines"`
	Payments   []PaymentInput `json:"payments"`
}

type Line struct {
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Qty       string `json:"qty"`
	UnitPrice string `json:"unit_price"`
	Discount  string `json:"discount"`
	LineTotal string `json:"line_total"`
}

type Payment struct {
	Type   string `json:"type"`
	Amount string `json:"amount"`
	Status string `json:"status"`
}

type Sale struct {
	ID            string    `json:"id"`
	Number        string    `json:"number"`
	BusinessDate  string    `json:"business_date"`
	SoldAt        time.Time `json:"sold_at"`
	Cashier       string    `json:"cashier"`
	Status        string    `json:"status"`
	Subtotal      string    `json:"subtotal"`
	DiscountTotal string    `json:"discount_total"`
	TaxTotal      string    `json:"tax_total"`
	Total         string    `json:"total"`
	PaidTotal     string    `json:"paid_total"`
	ChangeTotal   string    `json:"change_total"`
	PPNInclusive  bool      `json:"ppn_inclusive"`
	Customer      string    `json:"customer,omitempty"`
	PointsEarned  int       `json:"points_earned,omitempty"`
	Lines         []Line    `json:"lines,omitempty"`
	Payments      []Payment `json:"payments,omitempty"`
	Replayed      bool      `json:"replayed,omitempty"`
}

type priced struct {
	in         LineInput
	qty, disc  decimal.Decimal
	price      decimal.Decimal
	gross, tax decimal.Decimal
	sku, name  string
	track      bool
}

func parsePositive(s string) (decimal.Decimal, bool) {
	d, err := decimal.NewFromString(s)
	return d, err == nil && d.IsPositive()
}

// Checkout is idempotent on ClientUUID (FR-A8-13): replays return the original sale.
func (s *Service) Checkout(ctx context.Context, c Cashier, in CheckoutInput) (*Sale, error) {
	if in.ClientUUID == "" || len(in.Lines) == 0 || len(in.Payments) == 0 {
		return nil, ErrInvalid
	}
	if existing, err := s.byClientUUID(ctx, c.CompanyID, in.ClientUUID); err != nil {
		return nil, err
	} else if existing != nil {
		existing.Replayed = true
		return existing, nil
	}

	sale, err := s.checkoutTx(ctx, c, in)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" { // concurrent replay of the same client_uuid
		if existing, e := s.byClientUUID(ctx, c.CompanyID, in.ClientUUID); e == nil && existing != nil {
			existing.Replayed = true
			return existing, nil
		}
	}
	if errors.As(err, &pg) && (pg.Code == "22P02" || pg.Code == "23514") {
		return nil, ErrInvalid
	}
	return sale, err
}

func (s *Service) checkoutTx(ctx context.Context, c Cashier, in CheckoutInput) (*Sale, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var ppnEnabled, inclusive, allowNeg bool
	var rate decimal.Decimal
	var rateStr string
	if err := tx.QueryRow(ctx, `SELECT ppn_enabled, ppn_inclusive, allow_negative_stock, ppn_rate::text FROM companies WHERE id=$1`,
		c.CompanyID).Scan(&ppnEnabled, &inclusive, &allowNeg, &rateStr); err != nil {
		return nil, err
	}
	rate, _ = decimal.NewFromString(rateStr)
	if !ppnEnabled {
		rate = decimal.Zero
	}

	var customer any
	custName := ""
	if in.CustomerID != "" {
		var active bool
		if err := tx.QueryRow(ctx, `SELECT name, is_active FROM customers WHERE id=$1 AND company_id=$2`, in.CustomerID, c.CompanyID).Scan(&custName, &active); err != nil || !active {
			return nil, ErrInvalid
		}
		customer = in.CustomerID
	}

	// Merge duplicate products, then price every line from the server-side master (never trust client prices).
	merged := map[string]*priced{}
	var ids []string
	for _, l := range in.Lines {
		q, ok := parsePositive(l.Qty)
		if !ok {
			return nil, ErrInvalid
		}
		d := decimal.Zero
		if l.Discount != "" {
			if d, err = decimal.NewFromString(l.Discount); err != nil || d.IsNegative() {
				return nil, ErrInvalid
			}
		}
		if p, ok := merged[l.ProductID]; ok {
			p.qty, p.disc = p.qty.Add(q), p.disc.Add(d)
			continue
		}
		merged[l.ProductID] = &priced{in: l, qty: q, disc: d}
		ids = append(ids, l.ProductID)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := merged[id]
		var price string
		err := tx.QueryRow(ctx, `SELECT sku, name, base_price::text, track_stock FROM products
			WHERE id=$1 AND company_id=$2 AND is_active`, id, c.CompanyID).Scan(&p.sku, &p.name, &price, &p.track)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalid
		}
		if err != nil {
			return nil, err
		}
		if price, err = catalog.UnitPrice(ctx, tx, c.StoreID, id, numbering.BusinessDate(s.now()), p.qty, price); err != nil {
			return nil, err
		}
		p.price, _ = decimal.NewFromString(price)
		p.gross = p.price.Mul(p.qty).Sub(p.disc)
		if p.gross.IsNegative() {
			return nil, ErrInvalid
		}
		if inclusive { // tax embedded in price
			p.tax = p.gross.Mul(rate).Div(decimal.NewFromInt(100).Add(rate))
		} else {
			p.tax = p.gross.Mul(rate).Div(decimal.NewFromInt(100))
		}
	}

	// Stock: lock balances in stable order, reject shortages unless negative stock is allowed (FR-A8-11).
	var tracked []string
	for _, id := range ids {
		if merged[id].track {
			tracked = append(tracked, id)
		}
	}
	if len(tracked) > 0 {
		bal, err := inventory.LockBalances(ctx, tx, c.StoreID, tracked)
		if err != nil {
			return nil, err
		}
		for _, id := range tracked {
			if !allowNeg && bal[id].LessThan(merged[id].qty) {
				return nil, fmt.Errorf("%w: %s (tersedia %s)", ErrInsufficientStock, merged[id].name, bal[id].String())
			}
		}
	}

	// PPN is summed unrounded per line and rounded once at document level (locked constraint 4).
	subtotal, discTotal, taxSum := decimal.Zero, decimal.Zero, decimal.Zero
	for _, id := range ids {
		p := merged[id]
		subtotal = subtotal.Add(p.gross)
		discTotal = discTotal.Add(p.disc)
		taxSum = taxSum.Add(p.tax)
	}
	subtotal, discTotal = subtotal.Round(2), discTotal.Round(2)
	taxTotal := taxSum.Round(2)
	total := subtotal
	if !inclusive {
		total = subtotal.Add(taxTotal)
	}

	paid, cash := decimal.Zero, decimal.Zero
	for _, p := range in.Payments {
		a, ok := parsePositive(p.Amount)
		if !ok {
			return nil, ErrInvalid
		}
		paid = paid.Add(a)
		var isCash bool
		if err := tx.QueryRow(ctx, `SELECT is_cash FROM payment_types WHERE id=$1 AND company_id=$2 AND is_active`,
			p.PaymentTypeID, c.CompanyID).Scan(&isCash); err != nil {
			return nil, ErrInvalid
		}
		if isCash {
			cash = cash.Add(a)
		}
	}
	if paid.LessThan(total) {
		return nil, ErrUnderpaid
	}
	change := paid.Sub(total)
	if change.GreaterThan(cash) { // change can only be given from cash
		return nil, ErrInvalid
	}

	now := s.now()
	number, err := numbering.Next(ctx, tx, c.CompanyID, "sale", "INV", now)
	if err != nil {
		return nil, err
	}
	sale := &Sale{Number: number, BusinessDate: numbering.BusinessDate(now), SoldAt: now, Cashier: c.Name, Status: "completed",
		Subtotal: subtotal.String(), DiscountTotal: discTotal.String(), TaxTotal: taxTotal.String(), Total: total.String(),
		PaidTotal: paid.String(), ChangeTotal: change.String(), PPNInclusive: inclusive}
	if err := tx.QueryRow(ctx, `INSERT INTO sales (company_id, store_id, number, client_uuid, business_date, cashier_id,
			subtotal, discount_total, tax_total, total, paid_total, change_total, sold_at, customer_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id::text`,
		c.CompanyID, c.StoreID, number, in.ClientUUID, sale.BusinessDate, c.ID,
		sale.Subtotal, sale.DiscountTotal, sale.TaxTotal, sale.Total, sale.PaidTotal, sale.ChangeTotal, now, customer).Scan(&sale.ID); err != nil {
		return nil, err
	}
	for _, id := range ids {
		p := merged[id]
		if _, err := tx.Exec(ctx, `INSERT INTO sale_lines (sale_id, product_id, sku_snapshot, name_snapshot, qty, unit_price, discount, tax_rate, tax_amount, line_total)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, sale.ID, id, p.sku, p.name, p.qty.String(), p.price.String(),
			p.disc.String(), rate.String(), p.tax.String(), p.gross.Round(2).String()); err != nil {
			return nil, err
		}
		sale.Lines = append(sale.Lines, Line{SKU: p.sku, Name: p.name, Qty: p.qty.String(), UnitPrice: p.price.String(),
			Discount: p.disc.String(), LineTotal: p.gross.Round(2).String()})
		if p.track {
			if err := inventory.Record(ctx, tx, c.StoreID, id, "sale", p.qty.Neg(), "sale", sale.ID, number, c.ID); err != nil {
				return nil, err
			}
		}
	}
	for _, p := range in.Payments {
		var name string
		if err := tx.QueryRow(ctx, `INSERT INTO sale_payments (sale_id, payment_type_id, amount) VALUES ($1,$2,$3)
			RETURNING (SELECT name FROM payment_types WHERE id=$2)`, sale.ID, p.PaymentTypeID, p.Amount).Scan(&name); err != nil {
			return nil, err
		}
		sale.Payments = append(sale.Payments, Payment{Type: name, Amount: p.Amount, Status: "confirmed"})
	}
	sale.Customer = custName
	if in.CustomerID != "" { // members earn points on the final total (stock was locked before the customer: stable lock order)
		pts, err := loyalty.Earn(ctx, tx, in.CustomerID, sale.ID, number, c.ID, total)
		if err != nil {
			return nil, err
		}
		if pts > 0 {
			sale.PointsEarned = pts
			if _, err := tx.Exec(ctx, `UPDATE sales SET points_earned=$2 WHERE id=$1`, sale.ID, pts); err != nil {
				return nil, err
			}
		}
	}
	if err := audit.Record(ctx, tx, audit.Entry{CompanyID: c.CompanyID, Actor: audit.Actor{ID: c.ID, Name: c.Name},
		Action: "sale.create", SubjectType: "sale", SubjectID: sale.ID, Description: number,
		Properties: map[string]any{"total": sale.Total, "store_id": c.StoreID}}); err != nil {
		return nil, err
	}
	return sale, tx.Commit(ctx)
}

const saleCols = `s.id::text, s.number, s.business_date::text, s.sold_at, u.name, s.status, s.subtotal::text, s.discount_total::text,
	s.tax_total::text, s.total::text, s.paid_total::text, s.change_total::text, c.ppn_inclusive, COALESCE(cu.name,''), s.points_earned`

func scanSale(row pgx.Row) (*Sale, error) {
	var x Sale
	err := row.Scan(&x.ID, &x.Number, &x.BusinessDate, &x.SoldAt, &x.Cashier, &x.Status, &x.Subtotal, &x.DiscountTotal,
		&x.TaxTotal, &x.Total, &x.PaidTotal, &x.ChangeTotal, &x.PPNInclusive, &x.Customer, &x.PointsEarned)
	return &x, err
}

func (s *Service) byClientUUID(ctx context.Context, companyID, clientUUID string) (*Sale, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id::text FROM sales WHERE company_id=$1 AND client_uuid=$2`, companyID, clientUUID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, companyID, "", id)
}

// Get loads a sale with lines and payments. A non-empty storeID restricts access to that store.
func (s *Service) Get(ctx context.Context, companyID, storeID, id string) (*Sale, error) {
	sale, err := scanSale(s.pool.QueryRow(ctx, `SELECT `+saleCols+` FROM sales s JOIN users u ON u.id=s.cashier_id
		JOIN companies c ON c.id=s.company_id LEFT JOIN customers cu ON cu.id=s.customer_id WHERE s.id=$1 AND s.company_id=$2 AND ($3='' OR s.store_id::text=$3)`, id, companyID, storeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT sku_snapshot, name_snapshot, qty::text, unit_price::text, discount::text, line_total::text
		FROM sale_lines WHERE sale_id=$1 ORDER BY sku_snapshot`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l Line
		if err := rows.Scan(&l.SKU, &l.Name, &l.Qty, &l.UnitPrice, &l.Discount, &l.LineTotal); err != nil {
			rows.Close()
			return nil, err
		}
		sale.Lines = append(sale.Lines, l)
	}
	rows.Close()
	prow, err := s.pool.Query(ctx, `SELECT pt.name, sp.amount::text, sp.status FROM sale_payments sp
		JOIN payment_types pt ON pt.id=sp.payment_type_id WHERE sp.sale_id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer prow.Close()
	for prow.Next() {
		var p Payment
		if err := prow.Scan(&p.Type, &p.Amount, &p.Status); err != nil {
			return nil, err
		}
		sale.Payments = append(sale.Payments, p)
	}
	return sale, prow.Err()
}

// ListToday returns today's sales for the store (Jakarta date), newest first.
func (s *Service) ListToday(ctx context.Context, companyID, storeID string) ([]Sale, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+saleCols+` FROM sales s JOIN users u ON u.id=s.cashier_id
		JOIN companies c ON c.id=s.company_id LEFT JOIN customers cu ON cu.id=s.customer_id WHERE s.company_id=$1 AND s.store_id=$2 AND s.business_date=$3::date
		ORDER BY s.sold_at DESC LIMIT 200`, companyID, storeID, numbering.BusinessDate(s.now()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Sale{}
	for rows.Next() {
		x, err := scanSale(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

type PaymentType struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	IsCash bool   `json:"is_cash"`
}

func (s *Service) PaymentTypes(ctx context.Context, companyID string) ([]PaymentType, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, code, name, is_cash FROM payment_types WHERE company_id=$1 AND is_active ORDER BY is_cash DESC, name`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PaymentType{}
	for rows.Next() {
		var p PaymentType
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.IsCash); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
