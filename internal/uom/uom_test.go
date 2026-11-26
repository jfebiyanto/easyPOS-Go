package uom

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestFactorDanToBase(t *testing.T) {
	pcs, dus, pak, asing := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	g := Group{BaseUnitID: pcs, Lines: []Line{
		{UnitID: dus, AltQty: d("1"), BaseQty: d("12")}, // 1 Dus = 12 Pcs
		{UnitID: pak, AltQty: d("2"), BaseQty: d("5")},  // 2 Pak = 5 Pcs
		{UnitID: uuid.New(), AltQty: d("3"), BaseQty: d("1")},
	}}
	cases := []struct {
		name       string
		unit       uuid.UUID
		qty        string
		wantFactor string
		wantBase   string
	}{
		{"satuan dasar", pcs, "7", "1", "7"},
		{"Dus", dus, "2", "12", "24"},
		{"Pak: faktor pecahan", pak, "3", "2.5", "7.5"},
		{"Dus pecahan", dus, "0.5", "12", "6"},
	}
	for _, c := range cases {
		f, err := g.Factor(c.unit)
		if err != nil || !f.Equal(d(c.wantFactor)) {
			t.Errorf("%s: factor %v err %v, want %s", c.name, f, err, c.wantFactor)
		}
		b, err := g.ToBase(d(c.qty), c.unit)
		if err != nil || !b.Equal(d(c.wantBase)) {
			t.Errorf("%s: base %v err %v, want %s", c.name, b, err, c.wantBase)
		}
	}
	// faktor berulang (1/3) tidak kehilangan presisi pada ToBase: 3 satuan = 1 Pcs
	third := g.Lines[2].UnitID
	if b, _ := g.ToBase(d("3"), third); !b.Equal(d("1")) {
		t.Errorf("3 × (1/3) = %v, want 1", b)
	}
	if _, err := g.Factor(asing); !errors.Is(err, ErrUnknownUnit) {
		t.Errorf("satuan asing harus ErrUnknownUnit: %v", err)
	}
	if _, err := g.ToBase(d("1"), asing); !errors.Is(err, ErrUnknownUnit) {
		t.Errorf("ToBase satuan asing: %v", err)
	}
	if !g.Has(pcs) || !g.Has(dus) || g.Has(asing) {
		t.Error("Has salah")
	}
}
