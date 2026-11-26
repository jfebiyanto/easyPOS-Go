// Package pricing: menentukan harga satuan dari price list (fungsi murni, tanpa DB).
// Dipakai halaman Price List (endpoint uji) dan POS fase 3.
package pricing

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/uom"
)

const (
	SourcePriceListUnit = "price_list_unit" // item price list khusus satuan yang diminta
	SourcePriceListBase = "price_list_base" // item satuan dasar × faktor
	SourceBasePrice     = "base_price"      // harga dasar produk × faktor
)

var ErrQty = errors.New("qty harus lebih dari nol")

// Item price list. UnitID uuid.Nil = harga per satuan dasar.
type Item struct {
	UnitID uuid.UUID
	MinQty int
	Price  decimal.Decimal
}

type PriceList struct {
	Active      bool
	From, Until *time.Time // tanggal berlaku (inklusif); nil = tanpa batas
	Items       []Item
}

type Product struct {
	BasePrice decimal.Decimal // per satuan dasar
	Group     uom.Group       // BaseUnitID = satuan dasar produk; Lines kosong bila tanpa group
}

type Result struct {
	UnitPrice decimal.Decimal // per satuan yang diminta, 2 desimal
	Source    string
}

func day(t time.Time) string { return t.Format("2006-01-02") }

// effective: price list dipakai bila ada, aktif, dan `date` (tanggal di zona aplikasi) dalam rentang berlaku.
func (pl *PriceList) effective(date time.Time) bool {
	if pl == nil || !pl.Active {
		return false
	}
	d := day(date)
	return (pl.From == nil || d >= day(*pl.From)) && (pl.Until == nil || d <= day(*pl.Until))
}

// ResolvePrice: unit uuid.Nil = satuan dasar. Urutan: item satuan itu (min_qty terbesar ≤ qty) →
// item satuan dasar (min_qty ≤ qty×faktor) × faktor → base_price × faktor.
func ResolvePrice(pl *PriceList, p Product, unit uuid.UUID, qty decimal.Decimal, date time.Time) (Result, error) {
	if !qty.IsPositive() {
		return Result{}, ErrQty
	}
	if unit == uuid.Nil {
		unit = p.Group.BaseUnitID
	}
	factor, err := p.Group.Factor(unit)
	if err != nil {
		return Result{}, err
	}
	if pl.effective(date) {
		if it, ok := best(pl.Items, func(i Item) bool { return i.UnitID == unit && unit != p.Group.BaseUnitID }, qty); ok {
			return Result{it.Price.Round(2), SourcePriceListUnit}, nil
		}
		isBase := func(i Item) bool { return i.UnitID == uuid.Nil || i.UnitID == p.Group.BaseUnitID }
		if it, ok := best(pl.Items, isBase, qty.Mul(factor)); ok {
			return Result{it.Price.Mul(factor).Round(2), SourcePriceListBase}, nil
		}
	}
	return Result{p.BasePrice.Mul(factor).Round(2), SourceBasePrice}, nil
}

// best: item yang lolos `match` dengan min_qty ≤ qty; min_qty terbesar menang.
func best(items []Item, match func(Item) bool, qty decimal.Decimal) (Item, bool) {
	var out Item
	found := false
	for _, it := range items {
		if match(it) && decimal.NewFromInt(int64(it.MinQty)).LessThanOrEqual(qty) && (!found || it.MinQty > out.MinQty) {
			out, found = it, true
		}
	}
	return out, found
}
