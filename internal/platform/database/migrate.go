package database

import (
	"context"
	"database/sql"
	"io/fs"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Migrator menjalankan migration goose yang di-embed.
type Migrator struct {
	p  *goose.Provider
	db *sql.DB
}

func NewMigrator(url string, migrations fs.FS) (*Migrator, error) {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, sub)
	if err != nil {
		return nil, err
	}
	return &Migrator{p: p, db: sqlDB}, nil
}

func (m *Migrator) Close() error { return m.db.Close() }

func (m *Migrator) Up(ctx context.Context) ([]*goose.MigrationResult, error) { return m.p.Up(ctx) }

// Down membatalkan satu migration terakhir.
func (m *Migrator) Down(ctx context.Context) (*goose.MigrationResult, error) { return m.p.Down(ctx) }

func (m *Migrator) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	return m.p.Status(ctx)
}

var _ = stdlib.GetDefaultDriver
