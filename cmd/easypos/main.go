package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // Asia/Jakarta without OS tzdata

	"github.com/spf13/cobra"

	"github.com/jfebiyanto/easyPOS-Go/internal/httpapi"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/config"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/database"
	"github.com/jfebiyanto/easyPOS-Go/web"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	root := &cobra.Command{Use: "easypos", SilenceUsage: true}
	root.AddCommand(serveCmd(), migrateCmd(), healthcheckCmd())
	if err := root.Execute(); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func migrateCmd() *cobra.Command {
	return &cobra.Command{Use: "migrate", Short: "Apply database migrations", RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		pool, err := database.Open(cmd.Context(), cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		return database.Migrate(pool)
	}}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{Use: "serve", Short: "Run the HTTP server", RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		pool, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		if cfg.MigrateOnStart {
			if err := database.Migrate(pool); err != nil {
				return err
			}
		}

		srv := httpapi.New(pool, cfg.SecureCookies, web.Dist())
		if cfg.BootstrapUsername != "" && cfg.BootstrapPassword != "" {
			created, err := srv.Identity().Bootstrap(ctx, cfg.BootstrapCompany, cfg.BootstrapName, cfg.BootstrapUsername, cfg.BootstrapPassword)
			if err != nil {
				return err
			}
			if created {
				slog.Info("bootstrap owner created", "username", cfg.BootstrapUsername)
			}
		}

		hs := &http.Server{Addr: cfg.HTTPAddr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
		go func() {
			<-ctx.Done()
			sctx, c := context.WithTimeout(context.Background(), 15*time.Second)
			defer c()
			_ = hs.Shutdown(sctx)
		}()
		slog.Info("listening", "addr", cfg.HTTPAddr)
		if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}}
}

// healthcheckCmd lets the scratch-based image probe itself (no wget/curl inside).
func healthcheckCmd() *cobra.Command {
	return &cobra.Command{Use: "healthcheck", Hidden: true, RunE: func(*cobra.Command, []string) error {
		c := http.Client{Timeout: 3 * time.Second}
		resp, err := c.Get("http://127.0.0.1:8080/healthz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.New("unhealthy")
		}
		return nil
	}}
}
