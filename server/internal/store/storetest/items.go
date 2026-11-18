package storetest

import (
	"context"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/store"
)

// AddBareTitle inserts a film or series row without files into a library «Test» (created when
// missing), for the tests of packages that may not write title tables themselves (CLAUDE.md rule
// 2) but keep references to items, such as wishes.item_id. kind is movie or series.
func AddBareTitle(ctx context.Context, db *store.DB, id, kind string) error {
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO libraries (id, name, kind, created_at, updated_at)
	                                         VALUES (9000, 'Test', 'video', 1, 1) ON CONFLICT (id) DO NOTHING`); err != nil {
		return fmt.Errorf("test library: %w", err)
	}
	if _, err := db.Write.ExecContext(ctx, `INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at)
	                                         VALUES (?, ?, 9000, ?, ?, 1, 1, 1) ON CONFLICT (id) DO NOTHING`, id, kind, id, id); err != nil {
		return fmt.Errorf("test title %s: %w", id, err)
	}
	return nil
}

// MarkTitleAvailable makes a bare title playable as far as visible_items is concerned
// (available = 1), for tests that run the real visibility over it.
func MarkTitleAvailable(ctx context.Context, db *store.DB, id string) error {
	if _, err := db.Write.ExecContext(ctx, `UPDATE items SET available = 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("title %s available: %w", id, err)
	}
	return nil
}
