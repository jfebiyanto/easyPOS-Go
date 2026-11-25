// Package testutil: DB Postgres sungguhan untuk tes integrasi.
// Setiap panggilan NewDB membuat database sementara (di server EASYPOS_TEST_DATABASE_URL),
// menjalankan migrate up, dan menghapusnya saat tes selesai — aman untuk paket paralel.
package testutil

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jfebiyanto/easyPOS-Go/db"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/database"
)

//go:embed php_hash.txt
var phpHash string

// Password semua user seed; hash-nya dibuat PHP password_hash (prefix $2y$).
const Password = "rahasia123"

var counter atomic.Int64

type Seed struct {
	Company                           uuid.UUID
	StoreA, StoreB                    uuid.UUID
	Owner, Manager, Cashier, Inactive uuid.UUID
}

// NewDB membuat DB bersih + seed. Tes dilewati bila EASYPOS_TEST_DATABASE_URL tidak diset.
func NewDB(t *testing.T) (*pgxpool.Pool, string, Seed) {
	t.Helper()
	base := os.Getenv("EASYPOS_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("EASYPOS_TEST_DATABASE_URL tidak diset")
	}
	ctx := context.Background()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("easypos_t_%d_%d", time.Now().UnixNano(), counter.Add(1))
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	dsn := u.String()
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Close()
	})

	m, err := database.NewMigrator(dsn, db.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
	_ = m.Close()

	pool, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, dsn, seed(t, pool)
}

func id() uuid.UUID { v, _ := uuid.NewV7(); return v }

func seed(t *testing.T, pool *pgxpool.Pool) Seed {
	t.Helper()
	ctx := context.Background()
	s := Seed{Company: id(), StoreA: id(), StoreB: id(), Owner: id(), Manager: id(), Cashier: id(), Inactive: id()}
	now := time.Now().UTC()
	exec := func(q string, args ...any) {
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO company_details (id, name, created_at, updated_at) VALUES ($1, 'PT Uji', $2, $2)", s.Company, now)
	exec("INSERT INTO stores (id, company_id, code, name, created_at, updated_at) VALUES ($1, $2, 'AAA', 'Gerai A', $3, $3), ($4, $2, 'BBB', 'Gerai B', $3, $3)",
		s.StoreA, s.Company, now, s.StoreB)
	hash := strings.TrimSpace(phpHash)
	user := func(uid uuid.UUID, store any, username, role string, active bool) {
		exec("INSERT INTO users (id, company_id, store_id, name, username, role, is_active, password, created_at, updated_at) VALUES ($1,$2,$3,$4,$4,$5,$6,$7,$8,$8)",
			uid, s.Company, store, username, role, active, hash, now)
	}
	user(s.Owner, nil, "owner", "owner", true)
	user(s.Manager, s.StoreA, "manager", "store_manager", true)
	user(s.Cashier, s.StoreA, "cashier", "cashier", true)
	user(s.Inactive, s.StoreA, "nonaktif", "cashier", false)
	return s
}
