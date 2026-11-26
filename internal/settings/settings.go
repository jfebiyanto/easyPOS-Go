// Package settings: Konfigurasi Sistem (tabel system_settings), kunci bertipe + default.
package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jfebiyanto/easyPOS-Go/internal/db"
)

// Definition mendeskripsikan satu kunci: tipe "enum" (Options) atau "int" (Min..Max).
type Definition struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Type    string   `json:"type"`
	Options []string `json:"options"`
	Default any      `json:"default"`
	Min     int      `json:"min,omitempty"`
	Max     int      `json:"max,omitempty"`
}

const (
	KeySerialNotInStock = "pos.serial_not_in_stock"
	KeyIdleLockMinutes  = "security.idle_lock_minutes"
)

var Definitions = []Definition{
	{Key: KeySerialNotInStock, Label: "Serial tidak ada di stok", Type: "enum",
		Options: []string{"reject", "manager_approval", "allow"}, Default: "reject"},
	{Key: KeyIdleLockMinutes, Label: "Kunci otomatis setelah idle (menit)", Type: "int", Min: 1, Max: 480, Default: 15},
}

func find(key string) (Definition, bool) {
	for _, d := range Definitions {
		if d.Key == key {
			return d, true
		}
	}
	return Definition{}, false
}

// Validate memeriksa seluruh perubahan; error berisi pesan yang aman ditampilkan.
func Validate(changes map[string]any) error {
	for k, v := range changes {
		d, ok := find(k)
		if !ok {
			return fmt.Errorf("kunci konfigurasi tidak dikenal: %s", k)
		}
		if d.Type == "int" {
			n, ok := v.(float64) // JSON number
			if !ok || n != float64(int(n)) || int(n) < d.Min || int(n) > d.Max {
				return fmt.Errorf("%s harus bilangan bulat %d–%d", d.Label, d.Min, d.Max)
			}
			continue
		}
		s, ok := v.(string)
		if !ok || !slices.Contains(d.Options, s) {
			return fmt.Errorf("nilai tidak valid untuk %s", k)
		}
	}
	return nil
}

// GetAll mengembalikan semua kunci terdefinisi: nilai tersimpan atau default.
func GetAll(ctx context.Context, tx db.DBTX, companyID uuid.UUID) (map[string]any, error) {
	rows, err := db.New(tx).ListSettings(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, d := range Definitions {
		out[d.Key] = d.Default
	}
	for _, r := range rows {
		if _, ok := find(r.Key); !ok {
			continue
		}
		var v any
		if json.Unmarshal(r.Value, &v) == nil {
			out[r.Key] = v
		}
	}
	return out, nil
}

// Set menyimpan perubahan (sudah divalidasi) dalam transaksi pemanggil.
func Set(ctx context.Context, tx pgx.Tx, companyID, userID uuid.UUID, changes map[string]any, now time.Time) error {
	q := db.New(tx)
	for k, v := range changes {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if err := q.UpsertSetting(ctx, db.UpsertSettingParams{
			CompanyID: companyID, Key: k, Value: b,
			UpdatedBy: uuid.NullUUID{UUID: userID, Valid: true},
			UpdatedAt: sql.NullTime{Time: now.UTC(), Valid: true},
		}); err != nil {
			return err
		}
	}
	return nil
}

// IdleLock mengembalikan batas idle (kunci sesi) company; default bila belum diatur.
func IdleLock(ctx context.Context, tx db.DBTX, companyID uuid.UUID) time.Duration {
	all, err := GetAll(ctx, tx, companyID)
	n, ok := all[KeyIdleLockMinutes].(float64)
	if err != nil || !ok || n < 1 {
		n = 15
	}
	return time.Duration(n) * time.Minute
}
