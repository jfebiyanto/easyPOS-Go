// Package web menyematkan hasil build frontend (web/dist).
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
