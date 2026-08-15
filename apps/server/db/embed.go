// Package db holds the SQL assets that ship inside the binary.
package db

import "embed"

// Migrations are embedded so `shorturl migrate` works from the distroless
// image, which has no source tree and no writable filesystem.
//
//go:embed migrations/*.sql
var Migrations embed.FS
