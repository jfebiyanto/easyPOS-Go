// Package reporting builds the sales reports (B6, first cut: JSON summary, no export yet).
package reporting

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type Totals struct {
	Transactions int    `json:"transactions"`
	Items        string `json:"items"`
	Revenue      string `json:"revenue"`
	Tax          string `json:"tax"`
	AvgBasket    string `json:"avg_basket"`
	Returns      int    `json:"returns"`
	ReturnTotal  string `json:"return_total"`
	NetRevenue   string `json:"net_revenue"`
}

type DayRow struct {
	Date         string `json:"date"`
	Transactions int    `json:"transactions"`
	Revenue      string `json:"revenue"`
}

type ProductRow struct {
	SKU     string `json:"sku"`
	Name    string `json:"name"`
	Qty     string `json:"qty"`
	Revenue string `json:"revenue"`
}

type Summary struct {
	From, To string       `json:"-"`
	Totals   Totals       `json:"totals"`
	Daily    []DayRow     `json:"daily"`
	Top      []ProductRow `json:"top_products"`
}

// Sales summarises completed sales between two business dates (inclusive).
// storeID "" means all stores of the company. Void sales are excluded.
func (s *Service) Sales(ctx context.Context, companyID, storeID, from, to string) (*Summary, error) {
	out := &Summary{Daily: []DayRow{}, Top: []ProductRow{}}
	err := s.pool.QueryRow(ctx, `SELECT count(*), COALESCE(sum(total),0)::text, COALESCE(sum(tax_total),0)::text,
			COALESCE(round(avg(total),2),0)::text,
			COALESCE((SELECT sum(l.qty) FROM sale_lines l JOIN sales x ON x.id=l.sale_id
				WHERE x.company_id=$1 AND x.status='completed' AND ($2='' OR x.store_id::text=$2)
				AND x.business_date BETWEEN $3::date AND $4::date),0)::text
		FROM sales WHERE company_id=$1 AND status='completed' AND ($2='' OR store_id::text=$2)
		AND business_date BETWEEN $3::date AND $4::date`, companyID, storeID, from, to).
		Scan(&out.Totals.Transactions, &out.Totals.Revenue, &out.Totals.Tax, &out.Totals.AvgBasket, &out.Totals.Items)
	if err != nil {
		return nil, err
	}
	// Returns are dated by their own creation day in Jakarta, and reduce net revenue (FR-B6-02).
	if err := s.pool.QueryRow(ctx, `SELECT count(*), COALESCE(sum(total),0)::text FROM sales_returns
		WHERE company_id=$1 AND ($2='' OR store_id::text=$2)
		AND (created_at AT TIME ZONE 'Asia/Jakarta')::date BETWEEN $3::date AND $4::date`, companyID, storeID, from, to).
		Scan(&out.Totals.Returns, &out.Totals.ReturnTotal); err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT ($1::numeric - $2::numeric)::text`, out.Totals.Revenue, out.Totals.ReturnTotal).Scan(&out.Totals.NetRevenue); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT business_date::text, count(*), sum(total)::text FROM sales
		WHERE company_id=$1 AND status='completed' AND ($2='' OR store_id::text=$2) AND business_date BETWEEN $3::date AND $4::date
		GROUP BY business_date ORDER BY business_date`, companyID, storeID, from, to)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d DayRow
		if err := rows.Scan(&d.Date, &d.Transactions, &d.Revenue); err != nil {
			rows.Close()
			return nil, err
		}
		out.Daily = append(out.Daily, d)
	}
	rows.Close()
	trows, err := s.pool.Query(ctx, `SELECT l.sku_snapshot, max(l.name_snapshot), sum(l.qty)::text, sum(l.line_total)::text
		FROM sale_lines l JOIN sales s ON s.id=l.sale_id
		WHERE s.company_id=$1 AND s.status='completed' AND ($2='' OR s.store_id::text=$2) AND s.business_date BETWEEN $3::date AND $4::date
		GROUP BY l.sku_snapshot ORDER BY sum(l.qty) DESC, sum(l.line_total) DESC LIMIT 10`, companyID, storeID, from, to)
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	for trows.Next() {
		var p ProductRow
		if err := trows.Scan(&p.SKU, &p.Name, &p.Qty, &p.Revenue); err != nil {
			return nil, err
		}
		out.Top = append(out.Top, p)
	}
	return out, trows.Err()
}
