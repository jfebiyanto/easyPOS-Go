// Package inventory maintains store stock balances and the append-only ledger (A9).
package inventory

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// Record appends a ledger entry (qty is signed) and updates the cached balance.
// The caller must already hold the store_stocks row lock when validating balances.
func Record(ctx context.Context, tx pgx.Tx, storeID, productID, typ string, qty decimal.Decimal, refType, refID, note, actorID string) error {
	var actor any
	if actorID != "" {
		actor = actorID
	}
	if _, err := tx.Exec(ctx, `INSERT INTO store_stocks (store_id, product_id, on_hand) VALUES ($1,$2,$3)
		ON CONFLICT (store_id, product_id) DO UPDATE SET on_hand = store_stocks.on_hand + EXCLUDED.on_hand`,
		storeID, productID, qty.String()); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO stock_ledger (store_id, product_id, type, qty, ref_type, ref_id, note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::uuid)`, storeID, productID, typ, qty.String(), refType, refID, note, actor)
	return err
}

// LockBalances ensures a row exists for each product and locks them in a stable order
// (sorted by id) so concurrent checkouts cannot deadlock. Returns on_hand by product id.
func LockBalances(ctx context.Context, tx pgx.Tx, storeID string, productIDs []string) (map[string]decimal.Decimal, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO store_stocks (store_id, product_id)
		SELECT $1, unnest($2::uuid[]) ON CONFLICT DO NOTHING`, storeID, productIDs); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT product_id::text, on_hand::text FROM store_stocks
		WHERE store_id=$1 AND product_id = ANY($2::uuid[]) ORDER BY product_id FOR UPDATE`, storeID, productIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]decimal.Decimal{}
	for rows.Next() {
		var id, v string
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		out[id], _ = decimal.NewFromString(v)
	}
	return out, rows.Err()
}
