// Package numbering issues document numbers safely under concurrency (A10).
package numbering

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var jakarta = func() *time.Location {
	l, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return l
}()

// Next returns e.g. INV-202611-000042. The counter row is locked FOR UPDATE and
// resets each calendar month in Asia/Jakarta (FR-A10-02, FR-A10-03).
func Next(ctx context.Context, tx pgx.Tx, companyID, docType, prefix string, now time.Time) (string, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO numbering_configs (company_id, doc_type, prefix)
		VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, companyID, docType, prefix); err != nil {
		return "", err
	}
	var pfx, last string
	var pad int
	var n int64
	if err := tx.QueryRow(ctx, `SELECT prefix, padding, last_period, last_number FROM numbering_configs
		WHERE company_id=$1 AND doc_type=$2 FOR UPDATE`, companyID, docType).Scan(&pfx, &pad, &last, &n); err != nil {
		return "", err
	}
	period := now.In(jakarta).Format("200601")
	if period != last {
		n = 0
	}
	n++
	if _, err := tx.Exec(ctx, `UPDATE numbering_configs SET last_period=$3, last_number=$4
		WHERE company_id=$1 AND doc_type=$2`, companyID, docType, period, n); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%0*d", pfx, period, pad, n), nil
}

// BusinessDate is the Jakarta calendar date (end-of-day posting dates come in a later phase).
func BusinessDate(now time.Time) string { return now.In(jakarta).Format("2006-01-02") }

// Now is the clock used by callers that need a period; kept here so tests and services agree.
func Now() time.Time { return time.Now() }
