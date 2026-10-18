// Package migrations embeds the forward-only SQL migrations of the Slouchcat database.
// Every file follows TEMPLATE.sql.txt; the store package runs them with goose at startup.
package migrations

import "embed"

// FS holds all migration files (NNNNN_name.sql).
//
//go:embed *.sql
var FS embed.FS
