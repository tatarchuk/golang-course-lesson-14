// Package migrations embeds the SQL migration files into the binary, so the application
// needs no migrations directory on disk at runtime. MIGRATIONS_PATH remains an optional
// override that applies the files of a directory instead (development only).
package migrations

import "embed"

// FS holds every *.sql file of this directory; the golang-migrate iofs source reads the
// up/down pairs from it. This Go file is not embedded, so the set is SQL only.
//
//go:embed *.sql
var FS embed.FS
