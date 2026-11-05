package httpapi_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/catalog"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/sales"
)

type posFixture struct {
	fixture
	cat     *catalog.Service
	sales   *sales.Service
	cashier sales.Cashier
	cash    string
	qris    string
	unit    string
}

func setupPOS(t *testing.T) posFixture {
	t.Helper()
	f := setup(t)
	ctx := context.Background()
	p := posFixture{fixture: f, cat: catalog.NewService(f.pool), sales: sales.NewService(f.pool)}
	u := f.mk(t, "kasir", identity.RoleCashier)
	p.cashier = sales.Cashier{ID: u.ID, Name: u.Name, CompanyID: f.companyID, StoreID: f.storeID}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.pool.QueryRow(ctx, `INSERT INTO payment_types (company_id, code, name, is_cash) VALUES ($1,'CASH','Tunai',true) RETURNING id::text`, f.companyID).Scan(&p.cash))
	must(f.pool.QueryRow(ctx, `INSERT INTO payment_types (company_id, code, name, is_cash) VALUES ($1,'QRIS','QRIS',false) RETURNING id::text`, f.companyID).Scan(&p.qris))
	ref, err := p.cat.CreateUnit(ctx, audit.Actor{}, f.companyID, "pcs", "Pcs")
	must(err)
	p.unit = ref.ID
	return p
}

func (p posFixture) product(t *testing.T, sku, price string, stock string) string {
	t.Helper()
	id, err := p.cat.Create(context.Background(), audit.Actor{}, p.companyID, catalog.ProductInput{
		SKU: sku, Name: "Produk " + sku, UnitID: p.unit, BasePrice: price, TrackStock: true, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if stock != "" {
		if err := p.cat.Adjust(context.Background(), audit.Actor{}, p.companyID, p.storeID, id, stock, "stok awal"); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func (p posFixture) buy(prod, qty, pay string) sales.CheckoutInput {
	return sales.CheckoutInput{ClientUUID: uuid.NewString(),
		Lines:    []sales.LineInput{{ProductID: prod, Qty: qty}},
		Payments: []sales.PaymentInput{{PaymentTypeID: p.cash, Amount: pay}}}
}

func (p posFixture) onHand(t *testing.T, prod string) string {
	t.Helper()
	var v string
	if err := p.pool.QueryRow(context.Background(), `SELECT on_hand::text FROM store_stocks WHERE store_id=$1 AND product_id=$2`, p.storeID, prod).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return decimal.RequireFromString(v).String()
}

func TestPPNSummedThenRoundedAtDocumentLevel(t *testing.T) {
	p := setupPOS(t)
	a, b, c := p.product(t, "A", "150", "10"), p.product(t, "B", "150", "10"), p.product(t, "C", "150", "10")
	in := sales.CheckoutInput{ClientUUID: uuid.NewString(),
		Lines:    []sales.LineInput{{ProductID: a, Qty: "1"}, {ProductID: b, Qty: "1"}, {ProductID: c, Qty: "1"}},
		Payments: []sales.PaymentInput{{PaymentTypeID: p.cash, Amount: "500"}}}
	s, err := p.sales.Checkout(context.Background(), p.cashier, in)
	if err != nil {
		t.Fatal(err)
	}
	// 3 x 150 x 11/111 = 44.5946 -> 44.59 (per-line rounding would give 44.58).
	if s.TaxTotal != "44.59" || s.Total != "450" || s.ChangeTotal != "50" {
		t.Fatalf("tax=%s total=%s change=%s", s.TaxTotal, s.Total, s.ChangeTotal)
	}

	// Exclusive: tax is added on top.
	if _, err := p.pool.Exec(context.Background(), `UPDATE companies SET ppn_inclusive=false`); err != nil {
		t.Fatal(err)
	}
	s, err = p.sales.Checkout(context.Background(), p.cashier, sales.CheckoutInput{ClientUUID: uuid.NewString(),
		Lines:    []sales.LineInput{{ProductID: a, Qty: "1"}, {ProductID: b, Qty: "1"}, {ProductID: c, Qty: "1"}},
		Payments: []sales.PaymentInput{{PaymentTypeID: p.cash, Amount: "500"}}})
	if err != nil {
		t.Fatal(err)
	}
	if s.TaxTotal != "49.5" || s.Total != "499.5" {
		t.Fatalf("exclusive tax=%s total=%s", s.TaxTotal, s.Total)
	}
}

func TestCheckoutIsIdempotent(t *testing.T) {
	p := setupPOS(t)
	prod := p.product(t, "IDEM", "1000", "5")
	in := p.buy(prod, "2", "5000")
	first, err := p.sales.Checkout(context.Background(), p.cashier, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.sales.Checkout(context.Background(), p.cashier, in)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.Number != first.Number || second.ID != first.ID {
		t.Fatalf("replay returned a different sale: %+v vs %+v", second, first)
	}
	if got := p.onHand(t, prod); got != "3" {
		t.Fatalf("stock deducted more than once: %s", got)
	}
}

func TestStockRulesAndPayments(t *testing.T) {
	p := setupPOS(t)
	ctx := context.Background()
	prod := p.product(t, "S1", "1000", "2")
	if _, err := p.sales.Checkout(ctx, p.cashier, p.buy(prod, "3", "9000")); !errors.Is(err, sales.ErrInsufficientStock) {
		t.Fatalf("oversell: %v", err)
	}
	if _, err := p.pool.Exec(ctx, `UPDATE companies SET allow_negative_stock=true`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.sales.Checkout(ctx, p.cashier, p.buy(prod, "3", "3000")); err != nil {
		t.Fatalf("negative stock allowed: %v", err)
	}
	if got := p.onHand(t, prod); got != "-1" {
		t.Fatalf("on hand = %s", got)
	}
	if _, err := p.sales.Checkout(ctx, p.cashier, p.buy(prod, "1", "500")); !errors.Is(err, sales.ErrUnderpaid) {
		t.Fatalf("underpaid: %v", err)
	}
	// Overpaying with a non-cash method cannot produce change.
	in := p.buy(prod, "1", "1000")
	in.Payments = []sales.PaymentInput{{PaymentTypeID: p.qris, Amount: "2000"}}
	if _, err := p.sales.Checkout(ctx, p.cashier, in); !errors.Is(err, sales.ErrInvalid) {
		t.Fatalf("non-cash change: %v", err)
	}
}

func TestConcurrentCheckoutsNeverOversell(t *testing.T) {
	p := setupPOS(t)
	prod := p.product(t, "RACE", "1000", "5")
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, rejected := 0, 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.sales.Checkout(context.Background(), p.cashier, p.buy(prod, "1", "1000"))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, sales.ErrInsufficientStock):
				rejected++
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 5 || rejected != 7 {
		t.Fatalf("ok=%d rejected=%d, want 5/7", ok, rejected)
	}
	if got := p.onHand(t, prod); got != "0" {
		t.Fatalf("on hand = %s", got)
	}
	var n int
	_ = p.pool.QueryRow(context.Background(), `SELECT count(DISTINCT number) FROM sales`).Scan(&n)
	if n != 5 {
		t.Fatalf("distinct numbers = %d", n)
	}
}

func TestSKUImmutableAndLedgerAppendOnly(t *testing.T) {
	p := setupPOS(t)
	ctx := context.Background()
	id := p.product(t, "KEEP", "1000", "1")
	if _, err := p.pool.Exec(ctx, `UPDATE products SET sku='CHANGED' WHERE id=$1`, id); err == nil {
		t.Fatal("sku update should be rejected by the database")
	}
	if _, err := p.pool.Exec(ctx, `DELETE FROM stock_ledger`); err == nil {
		t.Fatal("ledger delete should be rejected")
	}
	if _, err := p.cat.Create(ctx, audit.Actor{}, p.companyID, catalog.ProductInput{SKU: "KEEP", Name: "dup", UnitID: p.unit, BasePrice: "1"}); !errors.Is(err, catalog.ErrDuplicate) {
		t.Fatalf("duplicate sku: %v", err)
	}
}

func TestPOSRoutesOverHTTP(t *testing.T) {
	p := setupPOS(t)
	prod := p.product(t, "H1", "2000", "4")
	owner := p.mk(t, "boss", identity.RoleOwner)
	_ = owner

	_, cashierCk := p.login(t, "kasir", "secret-pass")
	rec := p.postJSON("/api/pos/sales", cashierCk, fmt.Sprintf(`{"client_uuid":%q,"lines":[{"product_id":%q,"qty":"1"}],"payments":[{"payment_type_id":%q,"amount":"2000"}]}`,
		uuid.NewString(), prod, p.cash))
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "INV-") {
		t.Fatalf("checkout = %d %s", rec.Code, rec.Body.String())
	}
	// Owner has no store: POS is off limits; cashier cannot reach admin catalog writes.
	_, ownerCk := p.login(t, "boss", "secret-pass")
	if got := p.get("/api/pos/products", ownerCk).Code; got != http.StatusForbidden {
		t.Fatalf("owner on POS = %d, want 403", got)
	}
	if got := p.postJSON("/api/admin/products", cashierCk, `{}`).Code; got != http.StatusForbidden {
		t.Fatalf("cashier on admin catalog = %d, want 403", got)
	}
	if got := p.get("/api/admin/reports/sales", ownerCk); got.Code != 200 || !strings.Contains(got.Body.String(), `"transactions":1`) {
		t.Fatalf("report = %d %s", got.Code, got.Body.String())
	}
}
