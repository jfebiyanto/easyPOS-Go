package httpapi

import (
	"net/http"
	"strconv"

	"github.com/xuri/excelize/v2"
)

func num(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// exportSales streams the sales report and transaction list as an Excel workbook.
func (s *Server) exportSales(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r.Context())
	from, to, ok := dateRange(w, r)
	if !ok {
		return
	}
	scope := storeScope(r)
	sum, err := s.reports.Sales(r.Context(), u.CompanyID, scope, from, to)
	if err != nil {
		fail(w, err)
		return
	}
	txs, err := s.sales.List(r.Context(), u.CompanyID, scope, from, to)
	if err != nil {
		fail(w, err)
		return
	}

	f := excelize.NewFile()
	defer f.Close()
	head, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Color: []string{"#D6E2F3"}, Pattern: 1}})
	money, _ := f.NewStyle(&excelize.Style{NumFmt: 3})
	write := func(sheet string, header []string, rows [][]any, moneyCols ...int) {
		for i, h := range header {
			c, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = f.SetCellValue(sheet, c, h)
			_ = f.SetCellStyle(sheet, c, c, head)
			col, _ := excelize.ColumnNumberToName(i + 1)
			_ = f.SetColWidth(sheet, col, col, 22)
		}
		for ri, row := range rows {
			for ci, v := range row {
				c, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
				_ = f.SetCellValue(sheet, c, v)
			}
		}
		for _, mc := range moneyCols {
			a, _ := excelize.CoordinatesToCellName(mc, 2)
			b, _ := excelize.CoordinatesToCellName(mc, len(rows)+1)
			if len(rows) > 0 {
				_ = f.SetCellStyle(sheet, a, b, money)
			}
		}
	}

	f.SetSheetName("Sheet1", "Ringkasan")
	t := sum.Totals
	write("Ringkasan", []string{"Keterangan", "Nilai"}, [][]any{
		{"Periode", from + " s/d " + to},
		{"Omzet", num(t.Revenue)}, {"Transaksi", t.Transactions}, {"Item terjual", num(t.Items)},
		{"Rata-rata belanja", num(t.AvgBasket)}, {"PPN", num(t.Tax)},
		{"Jumlah retur", t.Returns}, {"Nilai retur", num(t.ReturnTotal)}, {"Omzet bersih", num(t.NetRevenue)},
	})
	_ = f.SetCellStyle("Ringkasan", "B3", "B3", money)
	_ = f.SetCellStyle("Ringkasan", "B6", "B7", money)
	_ = f.SetCellStyle("Ringkasan", "B9", "B10", money)

	_, _ = f.NewSheet("Per hari")
	var daily [][]any
	for _, d := range sum.Daily {
		daily = append(daily, []any{d.Date, d.Transactions, num(d.Revenue)})
	}
	write("Per hari", []string{"Tanggal", "Transaksi", "Omzet"}, daily, 3)

	_, _ = f.NewSheet("Produk terlaris")
	var top [][]any
	for _, p := range sum.Top {
		top = append(top, []any{p.SKU, p.Name, num(p.Qty), num(p.Revenue)})
	}
	write("Produk terlaris", []string{"SKU", "Produk", "Qty", "Omzet"}, top, 4)

	_, _ = f.NewSheet("Transaksi")
	var tx [][]any
	for _, x := range txs {
		status := "Selesai"
		if x.Status == "void" {
			status = "Void: " + x.Reason
		}
		tx = append(tx, []any{x.Number, x.SoldAt.In(jakarta).Format("2006-01-02 15:04"), x.Store, x.Cashier, x.Customer, num(x.Total), status})
	}
	write("Transaksi", []string{"No. struk", "Waktu", "Gerai", "Kasir", "Pelanggan", "Total", "Status"}, tx, 6)

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="laporan-penjualan-`+from+`_`+to+`.xlsx"`)
	_ = f.Write(w)
}
