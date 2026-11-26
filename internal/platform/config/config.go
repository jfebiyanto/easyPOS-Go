// Package config memuat konfigurasi dari environment (prefix EASYPOS_).
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DatabaseURL  string `env:"EASYPOS_DATABASE_URL,required"`
	HTTPAddr     string `env:"EASYPOS_HTTP_ADDR" envDefault:":8080"`
	AppTimezone  string `env:"EASYPOS_APP_TIMEZONE" envDefault:"Asia/Jakarta"`
	Env          string `env:"EASYPOS_ENV" envDefault:"dev"`
	CookieSecure bool   `env:"EASYPOS_COOKIE_SECURE"`
	StorageDir   string `env:"EASYPOS_STORAGE_DIR" envDefault:"./storage"` // gambar produk, dll.
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return c, err
	}
	if c.Env != "dev" && c.Env != "prod" {
		return c, fmt.Errorf("EASYPOS_ENV harus dev atau prod, dapat %q", c.Env)
	}
	if _, err := time.LoadLocation(c.AppTimezone); err != nil {
		return c, fmt.Errorf("EASYPOS_APP_TIMEZONE tidak valid: %w", err)
	}
	return c, nil
}

func (c Config) Location() *time.Location {
	loc, _ := time.LoadLocation(c.AppTimezone)
	return loc
}
