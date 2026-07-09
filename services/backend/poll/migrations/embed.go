// Package migrations embeds the SQL migration files so the service can apply its
// own schema on startup without shipping loose files.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
