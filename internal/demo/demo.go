// Package demo seeds sample data so the app can be shown without manual setup.
package demo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/catalog"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/loyalty"
)

const Password = "demo1234"

// Seed is safe to run once; it refuses when products already exist.
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	var companyID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM companies ORDER BY created_at LIMIT 1`).Scan(&companyID); err != nil {
		return errors.New("belum ada perusahaan: jalankan server sekali dengan bootstrap Owner dulu")
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM products WHERE company_id=$1`, companyID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errors.New("data demo sudah ada (produk tidak kosong)")
	}

	var storeID string
	if err := pool.QueryRow(ctx, `INSERT INTO stores (company_id, code, name) VALUES ($1,'GRY-01','Gerai Utama')
		ON CONFLICT (company_id, code) DO UPDATE SET name=EXCLUDED.name RETURNING id::text`, companyID).Scan(&storeID); err != nil {
		return err
	}
	ids := identity.NewService(pool)
	actor := audit.Actor{Name: "seed-demo"}
	for _, u := range []struct{ name, username, role string }{
		{"Budi Manajer", "manager", identity.RoleStoreManager},
		{"Sari Kasir", "kasir", identity.RoleCashier},
	} {
		if _, err := ids.Create(ctx, actor, identity.CreateParams{CompanyID: companyID, Name: u.name, Username: u.username,
			Password: Password, Role: u.role, StoreID: &storeID}); err != nil {
			return fmt.Errorf("buat user %s: %w", u.username, err)
		}
	}
	for _, pt := range []struct {
		code, name string
		cash       bool
	}{{"CASH", "Tunai", true}, {"QRIS", "QRIS", false}, {"EDC", "Kartu Debit/Kredit", false}} {
		if _, err := pool.Exec(ctx, `INSERT INTO payment_types (company_id, code, name, is_cash) VALUES ($1,$2,$3,$4)`,
			companyID, pt.code, pt.name, pt.cash); err != nil {
			return err
		}
	}

	if err := loyalty.EnsureDefaults(ctx, pool, companyID); err != nil {
		return err
	}
	ly := loyalty.NewService(pool)
	for _, c := range []loyalty.Input{{Name: "Ani Wijaya", Phone: "081234560001", IsMember: true}, {Name: "Rudi Hartono", Phone: "081234560002", IsMember: true}} {
		if _, err := ly.Create(ctx, actor, companyID, c); err != nil {
			return fmt.Errorf("pelanggan %s: %w", c.Name, err)
		}
	}

	cat := catalog.NewService(pool)
	unit := map[string]string{}
	for _, u := range [][2]string{{"pcs", "Pcs"}, {"btl", "Botol"}, {"bks", "Bungkus"}} {
		r, err := cat.CreateUnit(ctx, actor, companyID, u[0], u[1])
		if err != nil {
			return err
		}
		unit[u[0]] = r.ID
	}
	group := map[string]string{}
	for _, g := range [][2]string{{"MNM", "Minuman"}, {"SNK", "Makanan Ringan"}, {"KBT", "Kebutuhan Harian"}} {
		r, err := cat.CreateGroup(ctx, actor, companyID, g[0], g[1])
		if err != nil {
			return err
		}
		group[g[0]] = r.ID
	}
	products := []struct{ sku, barcode, name, grp, unit, price string }{
		{"MNM-001", "8991001000011", "Air Mineral 600ml", "MNM", "btl", "4000"},
		{"MNM-002", "8991001000028", "Teh Botol 450ml", "MNM", "btl", "6500"},
		{"MNM-003", "8991001000035", "Kopi Susu Kaleng", "MNM", "pcs", "9500"},
		{"MNM-004", "8991001000042", "Jus Jeruk 250ml", "MNM", "btl", "7500"},
		{"SNK-001", "8991002000018", "Keripik Singkong 100g", "SNK", "bks", "12000"},
		{"SNK-002", "8991002000025", "Biskuit Coklat", "SNK", "bks", "8500"},
		{"SNK-003", "8991002000032", "Kacang Garuda 200g", "SNK", "bks", "15000"},
		{"SNK-004", "8991002000049", "Wafer Keju", "SNK", "pcs", "3500"},
		{"KBT-001", "8991003000014", "Mie Instan Goreng", "KBT", "bks", "3500"},
		{"KBT-002", "8991003000021", "Beras Premium 5kg", "KBT", "bks", "78000"},
		{"KBT-003", "8991003000038", "Minyak Goreng 1L", "KBT", "btl", "19500"},
		{"KBT-004", "8991003000045", "Gula Pasir 1kg", "KBT", "bks", "17500"},
		{"KBT-005", "8991003000052", "Sabun Mandi Cair 250ml", "KBT", "btl", "22000"},
		{"KBT-006", "8991003000069", "Tisu Wajah", "KBT", "pcs", "11000"},
	}
	for _, p := range products {
		g, bc := group[p.grp], p.barcode
		id, err := cat.Create(ctx, actor, companyID, catalog.ProductInput{SKU: p.sku, Barcode: &bc, Name: p.name, GroupID: &g,
			UnitID: unit[p.unit], BasePrice: p.price, TrackStock: true, IsActive: true})
		if err != nil {
			return fmt.Errorf("produk %s: %w", p.sku, err)
		}
		if err := cat.Adjust(ctx, actor, companyID, storeID, id, "50", "Stok awal demo"); err != nil {
			return err
		}
	}
	return nil
}
