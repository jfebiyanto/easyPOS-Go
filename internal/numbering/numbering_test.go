package numbering

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jfebiyanto/easyPOS-Go/internal/db"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/database"
	"github.com/jfebiyanto/easyPOS-Go/internal/testutil"
)

func TestFormat(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		c    db.NumberingConfig
		n    int64
		want string
	}{
		{db.NumberingConfig{TransactionType: "sales", Prefix: "INV", Padding: 4, Separator: "/", ResetPeriod: "monthly"}, 7, "INV/202611/0007"},
		{db.NumberingConfig{TransactionType: "sales", Prefix: "INV", Padding: 3, Separator: "-", ResetPeriod: "daily"}, 12, "INV-20261105-012"},
		{db.NumberingConfig{TransactionType: "sales", Prefix: "INV", Padding: 4, Separator: "/", ResetPeriod: "yearly"}, 1, "INV/2026/0001"},
		{db.NumberingConfig{TransactionType: "sales", Prefix: "INV", Padding: 4, Separator: "/", ResetPeriod: "never"}, 12345, "INV/12345"},
		{db.NumberingConfig{TransactionType: "goods_receipt", Prefix: "", Padding: 2, Separator: "", ResetPeriod: "never"}, 0, "GOO/01"},
	}
	for _, c := range cases {
		if got := Format(c.c, c.n, now); got != c.want {
			t.Errorf("Format = %q, want %q", got, c.want)
		}
	}
}

func TestNextParalelDanReset(t *testing.T) {
	pool, _, seed := testutil.NewDB(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

	const n = 50
	got := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
				var err error
				got[i], err = Next(ctx, tx, seed.Company, "sales", now)
				return err
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	sort.Strings(got)
	for i, g := range got {
		want := Format(db.NumberingConfig{TransactionType: "sales", Prefix: "SAL", Padding: 4, Separator: "/", ResetPeriod: "monthly"}, int64(i+1), now)
		if g != want {
			t.Fatalf("nomor ke-%d = %q, want %q (duplikat atau lompat)", i+1, g, want)
		}
	}

	// ganti bulan: counter kembali ke 1
	var next string
	err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
		var err error
		next, err = Next(ctx, tx, seed.Company, "sales", now.AddDate(0, 1, 0))
		return err
	})
	if err != nil || next != "SAL/202612/0001" {
		t.Fatalf("reset periode: %q %v", next, err)
	}
	// tipe tak dikenal
	err = database.InTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := Next(ctx, tx, seed.Company, "bukan_tipe", now)
		return err
	})
	if err != ErrUnknownType {
		t.Fatalf("want ErrUnknownType, got %v", err)
	}
}
