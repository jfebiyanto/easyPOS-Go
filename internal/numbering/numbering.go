// Package numbering: penomoran transaksi (port NumberingConfig.php Laravel).
package numbering

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jfebiyanto/easyPOS-Go/internal/db"
)

// Type memuat urutan dan label tipe transaksi.
type Type struct{ Key, Label string }

var Types = []Type{
	{"sales", "Sales"},
	{"sales_return", "Return"},
	{"stock_adjustment", "Stock Counting"},
	{"stock_request", "Stock Request"},
	{"stock_transfer_shipment", "Stock Transfer Shipment"},
	{"stock_transfer_receipt", "Stock Transfer Receipt"},
	{"stock_transfer_return", "Stock Transfer Return"},
	{"goods_receipt", "Goods Receipt"},
	{"goods_issue", "Goods Issue"},
	{"stock_counting", "Stock Counting"},
	{"purchase_order", "Purchase Order"},
	{"purchase_receipt", "Purchase Receipt"},
	{"purchase_return", "Purchase Return"},
}

var ResetPeriods = map[string]bool{"daily": true, "monthly": true, "yearly": true, "never": true}

var ErrUnknownType = errors.New("tipe transaksi tidak dikenal")

func Known(t string) bool {
	for _, x := range Types {
		if x.Key == t {
			return true
		}
	}
	return false
}

func defaultPrefix(t string) string {
	if len(t) > 3 {
		t = t[:3]
	}
	return strings.ToUpper(t)
}

func ts(t time.Time) sql.NullTime { return sql.NullTime{Time: t.UTC(), Valid: true} }

// EnsureAll membuat konfigurasi default untuk semua tipe yang belum ada.
func EnsureAll(ctx context.Context, tx db.DBTX, companyID uuid.UUID, now time.Time) error {
	q := db.New(tx)
	for _, t := range Types {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if err := q.InsertNumberingIfMissing(ctx, db.InsertNumberingIfMissingParams{
			ID: id, CompanyID: companyID, TransactionType: t.Key, Prefix: defaultPrefix(t.Key), CreatedAt: ts(now)}); err != nil {
			return err
		}
	}
	return nil
}

// PeriodPart: bagian periode pada nomor (kosong bila reset 'never').
func PeriodPart(resetPeriod string, now time.Time) string {
	switch resetPeriod {
	case "daily":
		return now.Format("20060102")
	case "monthly":
		return now.Format("200601")
	case "yearly":
		return now.Format("2006")
	}
	return ""
}

// Format menyusun nomor: prefix{sep}period{sep}padded.
func Format(c db.NumberingConfig, number int64, now time.Time) string {
	if number < 1 {
		number = 1
	}
	prefix := c.Prefix
	if prefix == "" {
		prefix = defaultPrefix(c.TransactionType)
	}
	parts := []string{prefix}
	if p := PeriodPart(c.ResetPeriod, now); p != "" {
		parts = append(parts, p)
	}
	pad := int(c.Padding)
	if pad < 1 {
		pad = 1
	}
	parts = append(parts, fmt.Sprintf("%0*d", pad, number))
	sep := c.Separator
	if sep == "" {
		sep = "/"
	}
	return strings.Join(parts, sep)
}

// Next mengambil nomor berikutnya secara atomik (SELECT ... FOR UPDATE).
// now harus sudah dalam zona waktu aplikasi; commit tx oleh pemanggil.
func Next(ctx context.Context, tx pgx.Tx, companyID uuid.UUID, typ string, now time.Time) (string, error) {
	if !Known(typ) {
		return "", ErrUnknownType
	}
	q := db.New(tx)
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	if err := q.InsertNumberingIfMissing(ctx, db.InsertNumberingIfMissingParams{
		ID: id, CompanyID: companyID, TransactionType: typ, Prefix: defaultPrefix(typ), CreatedAt: ts(now)}); err != nil {
		return "", err
	}
	c, err := q.GetNumberingForUpdate(ctx, db.GetNumberingForUpdateParams{CompanyID: companyID, TransactionType: typ})
	if err != nil {
		return "", err
	}
	period := PeriodPart(c.ResetPeriod, now)
	// Reset hanya saat periode berganti (last_period sudah pernah terisi).
	isNew := c.ResetPeriod != "never" && c.LastPeriod.Valid && c.LastPeriod.String != period
	next := c.LastNumber + 1
	if isNew {
		next = 1
	}
	if err := q.UpdateNumberingCounter(ctx, db.UpdateNumberingCounterParams{
		ID: c.ID, LastNumber: next, LastPeriod: pgText(period), UpdatedAt: ts(now)}); err != nil {
		return "", err
	}
	return Format(c, next, now), nil
}
