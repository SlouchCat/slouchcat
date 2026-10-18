package library

import "example.com/app/internal/visibility"

const upsert = "INSERT INTO items (id, kind) VALUES (?, ?) ON CONFLICT DO NOTHING"

const files = "UPDATE media_files SET missing_since = ? WHERE root_id = ?"

func scan() visibility.Scope {
	return visibility.System("library scan")
}
