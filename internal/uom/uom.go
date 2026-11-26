// Package uom: konversi satuan berdasarkan UoM Group (pola SAP B1: alt_qty satuan ini = base_qty satuan dasar).
// Dipakai POS dan inventori: stok selalu dalam satuan dasar.
package uom

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/db"
)

var (
	ErrUnknownUnit = errors.New("satuan bukan anggota UoM group")
	ErrNotFound    = errors.New("UoM group tidak ditemukan")
)

type Line struct {
	UnitID          uuid.UUID
	AltQty, BaseQty decimal.Decimal
}

type Group struct {
	BaseUnitID uuid.UUID
	Lines      []Line // tanpa baris satuan dasar; satuan dasar selalu 1:1
}

// Factor = base_qty / alt_qty: berapa satuan dasar dalam 1 satuan ini. Satuan dasar = 1.
func (g Group) Factor(unit uuid.UUID) (decimal.Decimal, error) {
	if unit == g.BaseUnitID {
		return decimal.NewFromInt(1), nil
	}
	for _, l := range g.Lines {
		if l.UnitID == unit && l.AltQty.IsPositive() {
			return l.BaseQty.Div(l.AltQty), nil
		}
	}
	return decimal.Zero, ErrUnknownUnit
}

// ToBase mengubah qty dalam satuan `unit` ke satuan dasar (3 desimal). Dihitung qty*base/alt agar tidak
// kehilangan presisi pada faktor berulang (mis. 3 Pak dengan 2 Pak = 5 Pcs = 7.5).
func (g Group) ToBase(qty decimal.Decimal, unit uuid.UUID) (decimal.Decimal, error) {
	if unit == g.BaseUnitID {
		return qty, nil
	}
	for _, l := range g.Lines {
		if l.UnitID == unit && l.AltQty.IsPositive() {
			return qty.Mul(l.BaseQty).Div(l.AltQty).Round(3), nil
		}
	}
	return decimal.Zero, ErrUnknownUnit
}

// Has: apakah satuan anggota group (termasuk satuan dasar).
func (g Group) Has(unit uuid.UUID) bool {
	_, err := g.Factor(unit)
	return err == nil
}

// Load membaca group milik company beserta baris non-dasar.
func Load(ctx context.Context, q db.DBTX, company, group uuid.UUID) (Group, error) {
	var g Group
	if err := q.QueryRow(ctx, "SELECT base_unit_id FROM uom_groups WHERE id=$1 AND company_id=$2", group, company).Scan(&g.BaseUnitID); err != nil {
		return Group{}, ErrNotFound
	}
	rows, err := q.Query(ctx, "SELECT unit_id, alt_qty, base_qty FROM uom_group_lines WHERE uom_group_id=$1 AND unit_id<>$2", group, g.BaseUnitID)
	if err != nil {
		return Group{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var l Line
		if err := rows.Scan(&l.UnitID, &l.AltQty, &l.BaseQty); err != nil {
			return Group{}, err
		}
		g.Lines = append(g.Lines, l)
	}
	return g, rows.Err()
}
