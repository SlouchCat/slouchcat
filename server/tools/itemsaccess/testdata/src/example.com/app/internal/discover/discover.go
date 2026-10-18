package discover

import (
	"example.com/app/internal/visibility"
)

const listQuery = "SELECT id, title FROM items WHERE kind = 'movie'" // want `SQL "FROM items" accesses a title table`

var raw = /* want `SQL "JOIN media_files" accesses a title table` */ `
	SELECT f.rel_path
	FROM visible_items v
	JOIN media_files f ON f.item_id = v.item_id`

var concatenated = "DELETE " + "FROM " + "tracks WHERE id = ?" // want `SQL "FROM tracks" accesses a title table`

var quoted = "insert into \"items\" (id) values (1)" // want `SQL "into \\"items" accesses a title table`

func dynamic(table string) string {
	return "SELECT * FROM " + table // not a constant: no report
}

var harmless = []string{
	"SELECT * FROM visible_items",
	"SELECT * FROM items_fts", // other table
	"update profiles set name = ?",
	"items from the library",
}

func unfiltered() visibility.Scope {
	return visibility.System("discover wants everything") // want `visibility.System creates an unfiltered scope`
}

var systemFunc = visibility.System // want `visibility.System creates an unfiltered scope`

func filtered() visibility.Scope {
	return visibility.Profile(1)
}
