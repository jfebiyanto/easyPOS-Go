// Package db menyematkan migration goose ke dalam biner.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
