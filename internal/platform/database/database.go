// Package database opens the pgx pool and runs embedded goose migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/jfebiyanto/easyPOS-Go/db"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func Migrate(pool *pgxpool.Pool) error {
	goose.SetBaseFS(db.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	var sqlDB *sql.DB = stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	return goose.Up(sqlDB, "migrations")
}
