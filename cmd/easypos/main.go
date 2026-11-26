// Perintah easypos: serve, migrate, create-owner, version.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/jfebiyanto/easyPOS-Go/db"
	sqlcdb "github.com/jfebiyanto/easyPOS-Go/internal/db"
	"github.com/jfebiyanto/easyPOS-Go/internal/httpapi"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/numbering"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/config"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/database"
	"github.com/jfebiyanto/easyPOS-Go/web"
)

var version = "0.1.0-fase0"

func main() {
	root := &cobra.Command{Use: "easypos", SilenceUsage: true}
	root.AddCommand(serveCmd(), migrateCmd(), createOwnerCmd(),
		&cobra.Command{Use: "version", Run: func(*cobra.Command, []string) { fmt.Println("easypos", version) }})
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{Use: "serve", Short: "Jalankan server HTTP", RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		pool, err := database.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		dist, err := fs.Sub(web.Dist, "dist")
		if err != nil {
			return err
		}
		log := slog.Default()
		srv := &http.Server{Addr: cfg.HTTPAddr, ReadHeaderTimeout: 10 * time.Second, Handler: httpapi.New(httpapi.Deps{
			Pool: pool, Identity: identity.New(pool), Log: log, Loc: cfg.Location(), CookieSecure: cfg.CookieSecure, Web: dist, StorageDir: cfg.StorageDir})}
		go func() {
			<-ctx.Done()
			sctx, c := context.WithTimeout(context.Background(), 10*time.Second)
			defer c()
			_ = srv.Shutdown(sctx)
		}()
		log.Info("server berjalan", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}}
}

func migrateCmd() *cobra.Command {
	run := func(fn func(ctx context.Context, m *database.Migrator) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			m, err := database.NewMigrator(cfg.DatabaseURL, db.Migrations)
			if err != nil {
				return err
			}
			defer m.Close()
			return fn(cmd.Context(), m)
		}
	}
	c := &cobra.Command{Use: "migrate", Short: "Migration database (goose)"}
	c.AddCommand(
		&cobra.Command{Use: "up", RunE: run(func(ctx context.Context, m *database.Migrator) error {
			res, err := m.Up(ctx)
			for _, r := range res {
				fmt.Printf("OK   %s (%s)\n", r.Source.Path, r.Duration)
			}
			if err == nil && len(res) == 0 {
				fmt.Println("tidak ada migration baru")
			}
			return err
		})},
		&cobra.Command{Use: "down", Short: "Batalkan satu migration terakhir", RunE: run(func(ctx context.Context, m *database.Migrator) error {
			r, err := m.Down(ctx)
			if err == nil {
				fmt.Printf("DOWN %s\n", r.Source.Path)
			}
			return err
		})},
		&cobra.Command{Use: "status", RunE: run(func(ctx context.Context, m *database.Migrator) error {
			st, err := m.Status(ctx)
			for _, s := range st {
				fmt.Printf("%-8s %s\n", s.State, s.Source.Path)
			}
			return err
		})},
	)
	return c
}

func createOwnerCmd() *cobra.Command {
	var company, storeCode, storeName, username, password, name string
	c := &cobra.Command{Use: "create-owner", Short: "Buat perusahaan, gerai, dan user owner pada DB kosong", RunE: func(cmd *cobra.Command, _ []string) error {
		if company == "" || storeCode == "" || storeName == "" || username == "" || len(password) < 8 {
			return errors.New("--company, --store-code, --store-name, --username wajib; --password minimal 8 karakter")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		pool, err := database.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return err
		}
		now := sql.NullTime{Time: time.Now().UTC(), Valid: true}
		return database.InTx(ctx, pool, func(tx pgx.Tx) error {
			var n int
			if err := tx.QueryRow(ctx, "SELECT count(*) FROM company_details").Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return errors.New("database sudah berisi perusahaan; create-owner hanya untuk DB kosong")
			}
			q := sqlcdb.New(tx)
			cid, _ := uuid.NewV7()
			sid, _ := uuid.NewV7()
			uid, _ := uuid.NewV7()
			if err := q.InsertCompany(ctx, sqlcdb.InsertCompanyParams{ID: cid, Name: company, CreatedAt: now}); err != nil {
				return err
			}
			if err := q.InsertStore(ctx, sqlcdb.InsertStoreParams{ID: sid, CompanyID: cid, Code: storeCode, Name: storeName, CreatedAt: now}); err != nil {
				return err
			}
			if err := q.InsertUser(ctx, sqlcdb.InsertUserParams{ID: uid, CompanyID: cid, StoreID: uuid.NullUUID{UUID: sid, Valid: true},
				Name: name, Username: username, Role: identity.RoleOwner, Password: string(hash), CreatedAt: now}); err != nil {
				return err
			}
			if err := numbering.EnsureAll(ctx, tx, cid, now.Time); err != nil {
				return err
			}
			fmt.Printf("OK: perusahaan %q, gerai %s, owner %q dibuat\n", company, storeCode, username)
			return nil
		})
	}}
	f := c.Flags()
	f.StringVar(&company, "company", "", "nama perusahaan")
	f.StringVar(&storeCode, "store-code", "", "kode gerai, mis. UTM")
	f.StringVar(&storeName, "store-name", "", "nama gerai")
	f.StringVar(&username, "username", "owner", "username owner")
	f.StringVar(&password, "password", "", "password owner (min. 8 karakter)")
	f.StringVar(&name, "name", "Owner", "nama tampilan owner")
	return c
}
