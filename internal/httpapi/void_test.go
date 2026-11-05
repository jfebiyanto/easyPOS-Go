package httpapi_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/returns"
	"github.com/jfebiyanto/easyPOS-Go/internal/sales"
)

func (p posFixture) voidActor() audit.Actor { return audit.Actor{ID: p.cashier.ID, Name: "Manager"} }

func TestVoidRestoresStockPointsAndPayments(t *testing.T) {
	p := setupPOS(t)
	ctx := context.Background()
	prod := p.product(t, "V1", "50000", "10")
	c := p.member(t, "081200000101")
	s := p.sell(t, c.ID, prod, "4", "200000") // stock 6, 20 points

	if err := p.sales.Void(ctx, p.voidActor(), p.companyID, p.storeID, s.Number, "  "); !errors.Is(err, sales.ErrInvalid) {
		t.Fatalf("void without reason: %v", err)
	}
	if err := p.sales.Void(ctx, p.voidActor(), p.companyID, p.storeID, s.Number, "salah input"); err != nil {
		t.Fatal(err)
	}
	if got := p.onHand(t, prod); got != "10" {
		t.Fatalf("stock after void = %s, want 10", got)
	}
	if pts, _ := p.points(t, c.ID); pts != 0 {
		t.Fatalf("points after void = %d, want 0", pts)
	}
	var st, pay string
	if err := p.pool.QueryRow(ctx, `SELECT s.status, (SELECT string_agg(DISTINCT status, ',') FROM sale_payments WHERE sale_id=s.id) FROM sales s WHERE s.id=$1`, s.ID).Scan(&st, &pay); err != nil || st != "void" || pay != "void" {
		t.Fatalf("status %q payments %q err %v", st, pay, err)
	}
	if err := p.sales.Void(ctx, p.voidActor(), p.companyID, p.storeID, s.Number, "lagi"); !errors.Is(err, sales.ErrAlreadyVoid) {
		t.Fatalf("second void: %v", err)
	}
	// Void sales no longer count in reports and cannot be returned.
	var n int
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM activity_logs WHERE action='sale.void'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows %d err %v", n, err)
	}
	if _, err := p.returns.Lookup(ctx, p.actor(), s.Number); !errors.Is(err, returns.ErrVoided) {
		t.Fatalf("lookup void sale: %v", err)
	}
}

func TestVoidRules(t *testing.T) {
	p := setupPOS(t)
	ctx := context.Background()
	prod := p.product(t, "V2", "10000", "10")
	s := p.sell(t, "", prod, "2", "20000")

	if err := p.sales.Void(ctx, p.voidActor(), p.companyID, "00000000-0000-0000-0000-000000000000", s.Number, "x"); !errors.Is(err, sales.ErrNotFound) {
		t.Fatalf("other store: %v", err)
	}
	if err := p.sales.Void(ctx, p.voidActor(), p.companyID, p.storeID, "INV-NOPE", "x"); !errors.Is(err, sales.ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}

	// A sale with a return cannot be voided.
	lk, _ := p.returns.Lookup(ctx, p.actor(), s.Number)
	if _, err := p.returns.Create(ctx, p.actor(), p.ret(s.Number, "rusak", returns.LineInput{LineID: lk.Lines[0].LineID, Qty: "1"})); err != nil {
		t.Fatal(err)
	}
	if err := p.sales.Void(ctx, p.voidActor(), p.companyID, p.storeID, s.Number, "x"); !errors.Is(err, sales.ErrHasReturns) {
		t.Fatalf("void with return: %v", err)
	}

	// Only on the day of the sale.
	s2 := p.sell(t, "", prod, "1", "10000")
	if _, err := p.pool.Exec(ctx, `UPDATE sales SET business_date = business_date - 1 WHERE id=$1`, s2.ID); err != nil {
		t.Fatal(err)
	}
	if err := p.sales.Void(ctx, p.voidActor(), p.companyID, p.storeID, s2.Number, "x"); !errors.Is(err, sales.ErrVoidDay) {
		t.Fatalf("void next day: %v", err)
	}
}

func TestVoidRoutesAndList(t *testing.T) {
	p := setupPOS(t)
	prod := p.product(t, "V3", "10000", "10")
	s := p.sell(t, "", prod, "1", "10000")
	p.mk(t, "mgr", "store_manager")
	if _, err := p.pool.Exec(context.Background(), `UPDATE users SET store_id=$1 WHERE username='mgr'`, p.storeID); err != nil {
		t.Fatal(err)
	}
	_, mgr := p.login(t, "mgr", "secret-pass")
	_, cashier := p.login(t, "kasir", "secret-pass")

	if rec := p.postJSON("/api/admin/sales/"+s.Number+"/void", cashier, `{"reason":"x"}`); rec.Code != 403 {
		t.Fatalf("cashier void = %d, want 403", rec.Code)
	}
	list := p.get("/api/admin/sales", mgr)
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"can_void":true`) {
		t.Fatalf("list = %d %s", list.Code, list.Body)
	}
	if rec := p.postJSON("/api/admin/sales/"+s.Number+"/void", mgr, `{"reason":"batal"}`); rec.Code != 200 {
		t.Fatalf("manager void = %d %s", rec.Code, rec.Body)
	}
	if rec := p.postJSON("/api/admin/sales/"+s.Number+"/void", mgr, `{"reason":"batal"}`); rec.Code != 422 {
		t.Fatalf("second void = %d, want 422", rec.Code)
	}
	if list := p.get("/api/admin/sales", mgr); !strings.Contains(list.Body.String(), `"status":"void"`) || strings.Contains(list.Body.String(), `"can_void":true`) {
		t.Fatalf("list after void: %s", list.Body)
	}
}

func TestExcelExport(t *testing.T) {
	p := setupPOS(t)
	prod := p.product(t, "X1", "10000", "10")
	p.sell(t, "", prod, "3", "30000")
	p.mk(t, "mgr", "store_manager")
	if _, err := p.pool.Exec(context.Background(), `UPDATE users SET store_id=$1 WHERE username='mgr'`, p.storeID); err != nil {
		t.Fatal(err)
	}
	_, mgr := p.login(t, "mgr", "secret-pass")
	_, cashier := p.login(t, "kasir", "secret-pass")

	if rec := p.get("/api/admin/reports/sales.xlsx", cashier); rec.Code != 403 {
		t.Fatalf("cashier export = %d, want 403", rec.Code)
	}
	if rec := p.get("/api/admin/reports/sales.xlsx?from=bogus", mgr); rec.Code != 422 {
		t.Fatalf("bad date = %d, want 422", rec.Code)
	}
	rec := p.get("/api/admin/reports/sales.xlsx", mgr)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), ".xlsx") {
		t.Fatalf("export = %d %v", rec.Code, rec.Header())
	}
	wb, err := excelize.OpenReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := wb.GetSheetList(); strings.Join(got, ",") != "Ringkasan,Per hari,Produk terlaris,Transaksi" {
		t.Fatalf("sheets = %v", got)
	}
	if v, _ := wb.GetCellValue("Ringkasan", "B2"); v == "" {
		t.Fatal("summary empty")
	}
	rows, _ := wb.GetRows("Transaksi")
	if len(rows) != 2 || rows[1][6] != "Selesai" {
		t.Fatalf("transaksi rows = %v", rows)
	}
}
