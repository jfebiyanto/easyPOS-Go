// Package config loads runtime configuration from environment variables.
package config

import "github.com/caarlos0/env/v11"

type Config struct {
	HTTPAddr       string `env:"EASYPOS_HTTP_ADDR" envDefault:":8080"`
	DatabaseURL    string `env:"EASYPOS_DATABASE_URL,required"`
	SecureCookies  bool   `env:"EASYPOS_SECURE_COOKIES" envDefault:"false"`
	MigrateOnStart bool   `env:"EASYPOS_MIGRATE_ON_START" envDefault:"true"`

	// Optional first-run bootstrap: creates a company and an Owner when no user exists.
	BootstrapCompany  string `env:"EASYPOS_BOOTSTRAP_COMPANY" envDefault:"easyPOS"`
	BootstrapUsername string `env:"EASYPOS_BOOTSTRAP_OWNER_USERNAME"`
	BootstrapPassword string `env:"EASYPOS_BOOTSTRAP_OWNER_PASSWORD"`
	BootstrapName     string `env:"EASYPOS_BOOTSTRAP_OWNER_NAME" envDefault:"Owner"`
}

func Load() (Config, error) {
	var c Config
	err := env.Parse(&c)
	return c, err
}
