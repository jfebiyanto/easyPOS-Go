package crud

import (
	"fmt"
	"strings"
	"sync"
)

type fkRef struct{ Table, Col string }

type usageCache struct {
	mu sync.Mutex
	m  map[string][]fkRef
}

// Label ramah untuk tabel yang merujuk; selain ini dipakai nama tabel apa adanya.
var usageLabels = map[string]string{
	"products": "produk", "uom_groups": "UoM group", "uom_group_lines": "baris UoM group", "product_barcodes": "barcode",
	"price_list_items": "item price list", "stores": "gerai", "users": "pengguna", "sales": "transaksi penjualan",
	"sale_lines": "baris penjualan", "promotions": "promosi", "customers": "pelanggan", "company_details": "profil perusahaan",
	"payment_types": "tipe pembayaran", "sale_payments": "pembayaran penjualan", "warehouse_stocks": "stok gudang",
	"stock_ledger": "kartu stok", "price_lists": "price list", "currency_denominations": "denominasi",
	"store_price_lists": "price list gerai", "payment_type_store": "tipe pembayaran gerai",
}

// Tabel yang tidak menghalangi penghapusan (hanya jejak/log).
var usageIgnore = map[string]bool{"activity_logs": true}

func (e *Engine) refs(c *Ctx, table string) ([]fkRef, error) {
	e.usage.mu.Lock()
	defer e.usage.mu.Unlock()
	if r, ok := e.usage.m[table]; ok {
		return r, nil
	}
	rows, err := c.Q.Query(c.Ctx, `
SELECT c.conrelid::regclass::text, a.attname
FROM pg_constraint c JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
WHERE c.contype = 'f' AND c.confrelid = $1::regclass AND array_length(c.conkey, 1) = 1 AND c.confdeltype <> 'c'`, "public."+table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []fkRef
	for rows.Next() {
		var r fkRef
		if err := rows.Scan(&r.Table, &r.Col); err != nil {
			return nil, err
		}
		if !usageIgnore[r.Table] {
			out = append(out, r)
		}
	}
	if e.usage.m == nil {
		e.usage.m = map[string][]fkRef{}
	}
	e.usage.m[table] = out
	return out, rows.Err()
}

// UsageMessage mengembalikan pesan "X masih dipakai n Y" bila id direferensikan tabel lain, atau "".
func (e *Engine) UsageMessage(c *Ctx, table, noun, id string) (string, error) {
	refs, err := e.refs(c, table)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, r := range refs {
		var n int
		if err := c.Q.QueryRow(c.Ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE "%s"=$1::uuid`, r.Table, r.Col), id).Scan(&n); err != nil {
			return "", err
		}
		if n > 0 {
			label := usageLabels[r.Table]
			if label == "" {
				label = strings.ReplaceAll(r.Table, "_", " ")
			}
			parts = append(parts, fmt.Sprintf("%d %s", n, label))
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return noun + " masih dipakai " + strings.Join(parts, ", ") + ".", nil
}

func (e *Engine) usageMessage(c *Ctx, r *Resource, id string) (string, error) {
	return e.UsageMessage(c, r.Table, r.Noun, id)
}
