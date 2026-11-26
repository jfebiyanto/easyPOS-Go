package pricing

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/uom"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func date(s string) time.Time    { t, _ := time.Parse("2006-01-02", s); return t }
func tp(s string) *time.Time     { t := date(s); return &t }

func TestResolvePrice(t *testing.T) {
	pcs, dus, pak := uuid.New(), uuid.New(), uuid.New()
	grp := uom.Group{BaseUnitID: pcs, Lines: []uom.Line{
		{UnitID: dus, AltQty: d("1"), BaseQty: d("12")},
		{UnitID: pak, AltQty: d("2"), BaseQty: d("5")}, // faktor 2.5
	}}
	prod := Product{BasePrice: d("1000"), Group: grp}
	noGroup := Product{BasePrice: d("1000"), Group: uom.Group{BaseUnitID: pcs}}
	pl := &PriceList{Active: true, From: tp("2026-01-01"), Until: tp("2026-12-31"), Items: []Item{
		{UnitID: uuid.Nil, MinQty: 1, Price: d("900")},
		{UnitID: uuid.Nil, MinQty: 10, Price: d("800")},
		{UnitID: uuid.Nil, MinQty: 24, Price: d("700")},
		{UnitID: dus, MinQty: 1, Price: d("10000")},
		{UnitID: dus, MinQty: 5, Price: d("9000")},
	}}
	inactive := *pl
	inactive.Active = false

	tests := []struct {
		name  string
		pl    *PriceList
		prod  Product
		unit  uuid.UUID
		qty   string
		on    string
		price string
		src   string
		err   error
	}{
		{"satuan dasar, tier terkecil", pl, prod, pcs, "1", "2026-06-01", "900.00", SourcePriceListBase, nil},
		{"unit nil = satuan dasar", pl, prod, uuid.Nil, "9", "2026-06-01", "900.00", SourcePriceListBase, nil},
		{"min_qty terbesar yang terpenuhi", pl, prod, pcs, "10", "2026-06-01", "800.00", SourcePriceListBase, nil},
		{"qty melewati tier tertinggi", pl, prod, pcs, "100", "2026-06-01", "700.00", SourcePriceListBase, nil},
		{"item khusus satuan dus", pl, prod, dus, "1", "2026-06-01", "10000.00", SourcePriceListUnit, nil},
		{"tier dus min_qty 5", pl, prod, dus, "5", "2026-06-01", "9000.00", SourcePriceListUnit, nil},
		{"pak tanpa item: harga dasar × faktor 2.5, tier lihat qty dasar", pl, prod, pak, "4", "2026-06-01", "2000.00", SourcePriceListBase, nil}, // 4 pak = 10 pcs → 800 × 2.5
		{"pak qty kecil", pl, prod, pak, "1", "2026-06-01", "2250.00", SourcePriceListBase, nil},                                                  // 2.5 pcs → tier 1 = 900 × 2.5
		{"price list nonaktif → base_price × faktor", &inactive, prod, dus, "1", "2026-06-01", "12000.00", SourceBasePrice, nil},
		{"sebelum effective_from", pl, prod, pcs, "1", "2025-12-31", "1000.00", SourceBasePrice, nil},
		{"hari pertama berlaku", pl, prod, pcs, "1", "2026-01-01", "900.00", SourcePriceListBase, nil},
		{"hari terakhir berlaku", pl, prod, pcs, "1", "2026-12-31", "900.00", SourcePriceListBase, nil},
		{"setelah effective_until", pl, prod, pcs, "1", "2027-01-01", "1000.00", SourceBasePrice, nil},
		{"tanpa price list", nil, prod, dus, "2", "2026-06-01", "12000.00", SourceBasePrice, nil},
		{"produk tanpa group, satuan dasar", pl, noGroup, pcs, "1", "2026-06-01", "900.00", SourcePriceListBase, nil},
		{"satuan bukan anggota group", pl, noGroup, dus, "1", "2026-06-01", "", "", uom.ErrUnknownUnit},
		{"qty nol", pl, prod, pcs, "0", "2026-06-01", "", "", ErrQty},
		{"qty desimal di bawah tier", pl, prod, pcs, "0.5", "2026-06-01", "1000.00", SourceBasePrice, nil},
		{"price list tanpa item", &PriceList{Active: true}, prod, dus, "1", "2026-06-01", "12000.00", SourceBasePrice, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolvePrice(tc.pl, tc.prod, tc.unit, d(tc.qty), date(tc.on))
			if err != tc.err {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if err == nil && (got.UnitPrice.StringFixed(2) != tc.price || got.Source != tc.src) {
				t.Fatalf("got %s %s, want %s %s", got.UnitPrice.StringFixed(2), got.Source, tc.price, tc.src)
			}
		})
	}
}
