// Package db embeds the SQL migrations so they ship inside the binary.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
