package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/loyalty"
	"github.com/jfebiyanto/easyPOS-Go/internal/returns"
	"github.com/jfebiyanto/easyPOS-Go/internal/sales"
)

func (p posFixture) actor() returns.Actor {
	return returns.Actor{ID: p.cashier.ID, Name: p.cashier.Name, CompanyID: p.companyID, StoreID: p.storeID}
}

func (p posFixture) member(t *testing.T, phone string) *loyalty.Customer {
	t.Helper()
	c, err := loyalty.NewService(p.pool).Create(context.Background(), audit.Actor{}, p.companyID,
		loyalty.Input{Name: "Member " + phone, Phone: phone, IsMember: true})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (p posFixture) sell(t *testing.T, custID string, prod, qty, pay string) *sales.Sale {
	t.Helper()
	in := p.buy(prod, qty, pay)
	in.CustomerID = custID
	s, err := p.sales.Checkout(context.Background(), p.cashier, in)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (p posFixture) ret(number, reason string, lines ...returns.LineInput) returns.Input {
	return returns.Input{ClientUUID: uuid.NewString(), SaleNumber: number, Reason: reason, RefundTypeID: p.cash, Lines: lines}
}

func (p posFixture) points(t *testing.T, id string) (int, string) {
	t.Helper()
	c, err := loyalty.NewService(p.pool).Get(context.Background(), p.companyID, id)
	if err != nil {
		t.Fatal(err)
	}
	return c.Points, c.Level
}

func TestMemberEarnsPointsByLevelAndUpgrades(t *testing.T) {
	p := setupPOS(t)
	prod := p.product(t, "P1", "1000000", "20")
	c := p.member(t, "081200000001")

	s := p.sell(t, c.ID, prod, "1", "1000000") // Silver: Rp10.000/point -> 100 points
	if s.PointsEarned != 100 {
		t.Fatalf("earned %d, want 100", s.PointsEarned)
	}
	if pts, lvl := p.points(t, c.ID); pts != 100 || lvl != "Gold" {
		t.Fatalf("after first sale: %d %s, want 100 Gold", pts, lvl)
	}
	s = p.sell(t, c.ID, prod, "1", "1000000") // Gold: Rp8.000/point -> 125 points
	if s.PointsEarned != 125 {
		t.Fatalf("gold earned %d, want 125", s.PointsEarned)
	}
	// Non-member customers earn nothing; anonymous sales still work.
	if _, err := p.pool.Exec(context.Background(), `UPDATE customers SET is_member=false WHERE id=$1`, c.ID); err != nil {
		t.Fatal(err)
	}
	if s = p.sell(t, c.ID, prod, "1", "1000000"); s.PointsEarned != 0 {
		t.Fatalf("non-member earned %d", s.PointsEarned)
	}
	if s = p.sell(t, "", prod, "1", "1000000"); s.PointsEarned != 0 || s.Customer != "" {
		t.Fatalf("anonymous sale: %+v", s)
	}
}

func TestDuplicatePhoneRejectedAndAdjustNeedsReason(t *testing.T) {
	p := setupPOS(t)
	ly := loyalty.NewService(p.pool)
	c := p.member(t, "081200000002")
	if _, err := ly.Create(context.Background(), audit.Actor{}, p.companyID, loyalty.Input{Name: "Dup", Phone: "0812-0000-0002", IsMember: true}); !errors.Is(err, loyalty.ErrDuplicatePhone) {
		t.Fatalf("duplicate phone: %v", err)
	}
	if _, err := ly.Create(context.Background(), audit.Actor{}, p.companyID, loyalty.Input{Name: "Bad", Phone: "abc"}); !errors.Is(err, loyalty.ErrInvalid) {
		t.Fatalf("bad phone: %v", err)
	}
	if err := ly.Adjust(context.Background(), audit.Actor{Name: "m"}, p.companyID, c.ID, 50, " "); !errors.Is(err, loyalty.ErrInvalid) {
		t.Fatalf("adjust without reason: %v", err)
	}
	if err := ly.Adjust(context.Background(), audit.Actor{Name: "m"}, p.companyID, c.ID, 120, "kompensasi"); err != nil {
		t.Fatal(err)
	}
	if pts, lvl := p.points(t, c.ID); pts != 120 || lvl != "Gold" {
		t.Fatalf("after adjust: %d %s", pts, lvl)
	}
	// Level never drops automatically (FR-B5-05), points never go negative.
	if err := ly.Adjust(context.Background(), audit.Actor{Name: "m"}, p.companyID, c.ID, -500, "koreksi"); err != nil {
		t.Fatal(err)
	}
	if pts, lvl := p.points(t, c.ID); pts != 0 || lvl != "Gold" {
		t.Fatalf("after big negative: %d %s, want 0 Gold", pts, lvl)
	}
}

func TestReturnRestocksRefundsAndReversesPoints(t *testing.T) {
	p := setupPOS(t)
	ctx := context.Background()
	prod := p.product(t, "R1", "50000", "10")
	c := p.member(t, "081200000003")
	s := p.sell(t, c.ID, prod, "4", "200000") // 200.000 -> 20 points; stock 10 -> 6
	if s.PointsEarned != 20 {
		t.Fatalf("earned %d", s.PointsEarned)
	}

	lk, err := p.returns.Lookup(ctx, p.actor(), s.Number)
	if err != nil {
		t.Fatal(err)
	}
	if len(lk.Lines) != 1 || lk.Lines[0].Returnable != "4" || lk.Lines[0].UnitRefund != "50000" {
		t.Fatalf("lookup: %+v", lk.Lines)
	}
	line := lk.Lines[0].LineID

	if _, err := p.returns.Create(ctx, p.actor(), p.ret(s.Number, "rusak", returns.LineInput{LineID: line, Qty: "5"})); !errors.Is(err, returns.ErrOverReturn) {
		t.Fatalf("over return: %v", err)
	}
	r1, err := p.returns.Create(ctx, p.actor(), p.ret(s.Number, "rusak", returns.LineInput{LineID: line, Qty: "1"}))
	if err != nil {
		t.Fatal(err)
	}
	if r1.Total != "50000" || r1.PointsDeducted != 5 || !strings.HasPrefix(r1.Number, "RTN-") {
		t.Fatalf("first return: %+v", r1)
	}
	if got := p.onHand(t, prod); got != "7" {
		t.Fatalf("stock after return = %s, want 7", got)
	}
	// Remaining 3 units: cumulative reversal reaches all 20 points, no rounding drift.
	r2, err := p.returns.Create(ctx, p.actor(), p.ret(s.Number, "salah beli", returns.LineInput{LineID: line, Qty: "3"}))
	if err != nil {
		t.Fatal(err)
	}
	if r2.Total != "150000" || r2.PointsDeducted != 15 {
		t.Fatalf("second return: %+v", r2)
	}
	if pts, _ := p.points(t, c.ID); pts != 0 {
		t.Fatalf("points after full return = %d", pts)
	}
	if _, err := p.returns.Create(ctx, p.actor(), p.ret(s.Number, "lagi", returns.LineInput{LineID: line, Qty: "1"})); !errors.Is(err, returns.ErrOverReturn) {
		t.Fatalf("return beyond sold: %v", err)
	}
	if got := p.onHand(t, prod); got != "10" {
		t.Fatalf("stock = %s, want 10", got)
	}
}

func TestReturnRulesWindowVoidIdempotencyStore(t *testing.T) {
	p := setupPOS(t)
	ctx := context.Background()
	prod := p.product(t, "R2", "10000", "10")
	s := p.sell(t, "", prod, "2", "20000")
	lk, _ := p.returns.Lookup(ctx, p.actor(), s.Number)
	line := lk.Lines[0].LineID

	in := p.ret(s.Number, "x", returns.LineInput{LineID: line, Qty: "1"})
	first, err := p.returns.Create(ctx, p.actor(), in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := p.returns.Create(ctx, p.actor(), in)
	if err != nil || !again.Replayed || again.Number != first.Number {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if got := p.onHand(t, prod); got != "9" {
		t.Fatalf("replay changed stock: %s", got)
	}

	if _, err := p.returns.Create(ctx, p.actor(), returns.Input{ClientUUID: uuid.NewString(), SaleNumber: s.Number, RefundTypeID: p.cash,
		Lines: []returns.LineInput{{LineID: line, Qty: "1"}}}); !errors.Is(err, returns.ErrInvalid) {
		t.Fatalf("missing reason: %v", err)
	}
	if _, err := p.returns.Lookup(ctx, returns.Actor{CompanyID: p.companyID, StoreID: uuid.NewString()}, s.Number); !errors.Is(err, returns.ErrNotFound) {
		t.Fatalf("other store must not see the sale: %v", err)
	}
	if _, err := p.returns.Lookup(ctx, p.actor(), "INV-NOPE"); !errors.Is(err, returns.ErrNotFound) {
		t.Fatalf("unknown number: %v", err)
	}

	if _, err := p.pool.Exec(ctx, `UPDATE sales SET sold_at = now() - interval '31 days' WHERE id=$1`, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.returns.Create(ctx, p.actor(), p.ret(s.Number, "x", returns.LineInput{LineID: line, Qty: "1"})); !errors.Is(err, returns.ErrWindow) {
		t.Fatalf("outside window: %v", err)
	}
	if _, err := p.pool.Exec(ctx, `UPDATE sales SET sold_at=now(), status='void' WHERE id=$1`, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.returns.Create(ctx, p.actor(), p.ret(s.Number, "x", returns.LineInput{LineID: line, Qty: "1"})); !errors.Is(err, returns.ErrVoided) {
		t.Fatalf("void sale: %v", err)
	}
	_ = time.Now
}

func TestExclusiveTaxRefundIncludesTax(t *testing.T) {
	p := setupPOS(t)
	ctx := context.Background()
	if _, err := p.pool.Exec(ctx, `UPDATE companies SET ppn_inclusive=false`); err != nil {
		t.Fatal(err)
	}
	prod := p.product(t, "R3", "100000", "5")
	s := p.sell(t, "", prod, "1", "111000") // 100.000 + 11% = 111.000
	lk, _ := p.returns.Lookup(ctx, p.actor(), s.Number)
	r, err := p.returns.Create(ctx, p.actor(), p.ret(s.Number, "batal", returns.LineInput{LineID: lk.Lines[0].LineID, Qty: "1"}))
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != "111000" {
		t.Fatalf("refund = %s, want 111000", r.Total)
	}
}

func TestConcurrentReturnsCannotExceedSold(t *testing.T) {
	p := setupPOS(t)
	ctx := context.Background()
	prod := p.product(t, "R4", "10000", "20")
	c := p.member(t, "081200000004")
	s := p.sell(t, c.ID, prod, "3", "30000")
	lk, _ := p.returns.Lookup(ctx, p.actor(), s.Number)
	line := lk.Lines[0].LineID

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.returns.Create(ctx, p.actor(), p.ret(s.Number, "race", returns.LineInput{LineID: line, Qty: "1"}))
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				ok++
			} else if !errors.Is(err, returns.ErrOverReturn) {
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 3 {
		t.Fatalf("%d returns succeeded, want exactly 3", ok)
	}
	if got := p.onHand(t, prod); got != "20" {
		t.Fatalf("stock = %s, want 20", got)
	}
	if pts, _ := p.points(t, c.ID); pts != 0 {
		t.Fatalf("points = %d, want 0 (all 3 points reversed)", pts)
	}
}

func TestCustomerAndReturnRoutesOverHTTP(t *testing.T) {
	p := setupPOS(t)
	prod := p.product(t, "H2", "20000", "5")
	p.mk(t, "mgr", identity.RoleStoreManager)
	_, ck := p.login(t, "kasir", "secret-pass")

	rec := p.postJSON("/api/pos/customers", ck, `{"name":"Tono","phone":"081299990000"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"level":"Silver"`) {
		t.Fatalf("register = %d %s", rec.Code, rec.Body.String())
	}
	if got := p.postJSON("/api/pos/customers", ck, `{"name":"Tono2","phone":"081299990000"}`).Code; got != http.StatusConflict {
		t.Fatalf("duplicate phone = %d, want 409", got)
	}
	if got := p.get("/api/pos/customers?q=0812999", ck); !strings.Contains(got.Body.String(), "Tono") {
		t.Fatalf("search: %s", got.Body.String())
	}
	if got := p.postJSON("/api/admin/customers", ck, `{}`).Code; got != http.StatusForbidden {
		t.Fatalf("cashier on admin customers = %d, want 403", got)
	}
	s := p.sell(t, "", prod, "1", "20000")
	if got := p.get("/api/pos/returns/lookup?number="+s.Number, ck); got.Code != 200 || !strings.Contains(got.Body.String(), "line_id") {
		t.Fatalf("lookup = %d %s", got.Code, got.Body.String())
	}
	if got := p.get("/api/pos/returns/lookup?number=NOPE", ck).Code; got != http.StatusNotFound {
		t.Fatalf("unknown receipt = %d, want 404", got)
	}
	_, mck := p.login(t, "mgr", "secret-pass")
	if got := p.postJSON("/api/admin/customers", mck, `{"name":"Dewi","phone":"081211112222","is_member":true}`).Code; got != http.StatusCreated {
		t.Fatalf("manager create = %d", got)
	}
}
