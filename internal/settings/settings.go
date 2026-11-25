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

// Definition mendeskripsikan satu kunci. Saat ini hanya tipe enum.
type Definition struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Type    string   `json:"type"`
	Options []string `json:"options"`
	Default string   `json:"default"`
}

const KeySerialNotInStock = "pos.serial_not_in_stock"

var Definitions = []Definition{
	{Key: KeySerialNotInStock, Label: "Serial tidak ada di stok", Type: "enum",
		Options: []string{"reject", "manager_approval", "allow"}, Default: "reject"},
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
