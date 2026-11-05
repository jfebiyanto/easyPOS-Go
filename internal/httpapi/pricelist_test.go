package httpapi_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/catalog"
)

func TestPriceListTiersApplyAtCheckoutAndFallBack(t *testing.T) {
	p := setupPOS(t)
	ctx := context.Background()
	a := audit.Actor{ID: p.cashier.ID, Name: "Owner"}
	prod := p.product(t, "PL1", "10000", "100")

	id, err := p.cat.SavePriceList(ctx, a, p.companyID, "", catalog.PriceListInput{Code: "GROSIR", Name: "Grosir", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.cat.SavePriceList(ctx, a, p.companyID, "", catalog.PriceListInput{Code: "GROSIR", Name: "Dup", IsActive: true}); !errors.Is(err, catalog.ErrDuplicate) {
		t.Fatalf("duplicate code: %v", err)
	}
	for _, it := range []catalog.PriceItemInput{{ProductID: prod, MinQty: "1", Price: "9000"}, {ProductID: prod, MinQty: "10", Price: "8000"}} {
		if err := p.cat.SetPriceItem(ctx, a, p.companyID, id, it); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.cat.SetPriceItem(ctx, a, p.companyID, id, catalog.PriceItemInput{ProductID: prod, MinQty: "0", Price: "1"}); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("zero min qty: %v", err)
	}

	// Not assigned to the store yet: base price.
	if s := p.sell(t, "", prod, "10", "100000"); s.Total != "100000.00" && s.Total != "100000" {
		t.Fatalf("unassigned total = %s", s.Total)
	}
	if err := p.cat.AssignStorePriceList(ctx, a, p.companyID, p.storeID, id); err != nil {
		t.Fatal(err)
	}
	if s := p.sell(t, "", prod, "2", "100000"); s.Lines[0].UnitPrice != "9000" {
		t.Fatalf("tier 1 price = %s", s.Lines[0].UnitPrice)
	}
	if s := p.sell(t, "", prod, "10", "100000"); s.Lines[0].UnitPrice != "8000" {
		t.Fatalf("tier 10 price = %s", s.Lines[0].UnitPrice)
	}
	// A product without items in the list keeps its base price.
	other := p.product(t, "PL2", "5000", "10")
	if s := p.sell(t, "", other, "1", "5000"); s.Lines[0].UnitPrice != "5000" {
		t.Fatalf("other product price = %s", s.Lines[0].UnitPrice)
	}

	// Expired or inactive lists fall back to the base price.
	if _, err := p.pool.Exec(ctx, `UPDATE price_lists SET effective_until = current_date - 1 WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if s := p.sell(t, "", prod, "2", "100000"); s.Lines[0].UnitPrice != "10000" {
		t.Fatalf("expired list price = %s", s.Lines[0].UnitPrice)
	}
	if _, err := p.pool.Exec(ctx, `UPDATE price_lists SET effective_until = NULL, is_active=false WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if s := p.sell(t, "", prod, "2", "100000"); s.Lines[0].UnitPrice != "10000" {
		t.Fatalf("inactive list price = %s", s.Lines[0].UnitPrice)
	}

	// Lists of another company cannot be assigned.
	var otherCo, otherList string
	_ = p.pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ('Lain') RETURNING id::text`).Scan(&otherCo)
	_ = p.pool.QueryRow(ctx, `INSERT INTO price_lists (company_id, code, name) VALUES ($1,'X','X') RETURNING id::text`, otherCo).Scan(&otherList)
	if err := p.cat.AssignStorePriceList(ctx, a, p.companyID, p.storeID, otherList); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("cross-company assign: %v", err)
	}
}

func TestPriceListRoutes(t *testing.T) {
	p := setupPOS(t)
	prod := p.product(t, "PL3", "10000", "10")
	p.mk(t, "own", "owner")
	p.mk(t, "mgr", "store_manager")
	if _, err := p.pool.Exec(context.Background(), `UPDATE users SET store_id=$1 WHERE username='mgr'`, p.storeID); err != nil {
		t.Fatal(err)
	}
	_, owner := p.login(t, "own", "secret-pass")
	_, mgr := p.login(t, "mgr", "secret-pass")
	_, cashier := p.login(t, "kasir", "secret-pass")

	if rec := p.postJSON("/api/admin/price-lists", mgr, `{"code":"A","name":"A","is_active":true}`); rec.Code != 403 {
		t.Fatalf("manager create = %d, want 403", rec.Code)
	}
	if rec := p.get("/api/admin/price-lists", cashier); rec.Code != 403 {
		t.Fatalf("cashier list = %d, want 403", rec.Code)
	}
	rec := p.postJSON("/api/admin/price-lists", owner, `{"code":"A","name":"A","is_active":true}`)
	if rec.Code != 200 {
		t.Fatalf("owner create = %d %s", rec.Code, rec.Body)
	}
	var out struct{ ID string }
	_ = jsonDecode(rec.Body, &out)
	if rec := p.putJSON("/api/admin/price-lists/"+out.ID+"/items", owner, `{"product_id":"`+prod+`","min_qty":"5","price":"7000"}`); rec.Code != 200 {
		t.Fatalf("set item = %d %s", rec.Code, rec.Body)
	}
	if rec := p.putJSON("/api/admin/stores/"+p.storeID+"/price-list", owner, `{"price_list_id":"`+out.ID+`"}`); rec.Code != 200 {
		t.Fatalf("assign = %d %s", rec.Code, rec.Body)
	}
	if rec := p.get("/api/admin/price-lists/"+out.ID+"/items", mgr); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"price":"7000`) {
		t.Fatalf("items = %d %s", rec.Code, rec.Body)
	}
	// The POS sees the tiers for its store.
	if rec := p.get("/api/pos/products", cashier); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tiers":[{"min_qty":"5`) {
		t.Fatalf("pos products = %d %s", rec.Code, rec.Body)
	}
}
