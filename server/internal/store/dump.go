package store

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Dump writes a plain SQL snapshot of db (schema in creation order, then the rows of every
// table, including goose_db_version) that sqlite can replay into an empty database. Frozen
// milestone states live in testdata/db/<Mx>.sql (plan 6.2 point 4); the migration test loads each
// of them and runs all newer migrations on top. The format is deliberately simple: one statement
// per line, values as SQL literals, so the files diff well.
func Dump(ctx context.Context, db *sql.DB, w io.Writer) error {
	objects, err := schemaObjects(ctx, db)
	if err != nil {
		return err
	}
	// Tables first with their rows, so triggers and views never see a half-filled table. An
	// external-content virtual table (FTS5 with content=) holds no rows of its own: it is rebuilt
	// from its content table once that table is filled.
	var rebuild []string
	for _, o := range objects {
		if o.typ != "table" {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s;\n", strings.TrimSpace(o.sql)); err != nil {
			return err
		}
		if externalContent(o.sql) {
			rebuild = append(rebuild, o.name)
			continue
		}
		if err := dumpRows(ctx, db, o.name, w); err != nil {
			return err
		}
	}
	for _, name := range rebuild {
		quoted := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
		if _, err := fmt.Fprintf(w, "INSERT INTO %s(%s) VALUES ('rebuild');\n", quoted, quoted); err != nil {
			return err
		}
	}
	for _, o := range objects {
		if o.typ == "table" {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s;\n", strings.TrimSpace(o.sql)); err != nil {
			return err
		}
	}
	return nil
}

type schemaObject struct{ typ, name, table, sql string }

// externalContent reports whether a CREATE VIRTUAL TABLE statement declares an FTS5 table over a
// separate content table (content='...').
func externalContent(createSQL string) bool {
	upper := strings.ToUpper(createSQL)
	return strings.HasPrefix(strings.TrimSpace(upper), "CREATE VIRTUAL TABLE") && strings.Contains(strings.ReplaceAll(upper, " ", ""), "CONTENT='")
}

func schemaObjects(ctx context.Context, db *sql.DB) ([]schemaObject, error) {
	rows, err := db.QueryContext(ctx, `SELECT type, name, tbl_name, sql FROM sqlite_master
		WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY rowid`)
	if err != nil {
		return nil, fmt.Errorf("dump schema: %w", err)
	}
	defer rows.Close()
	var objects []schemaObject
	for rows.Next() {
		var o schemaObject
		if err := rows.Scan(&o.typ, &o.name, &o.table, &o.sql); err != nil {
			return nil, fmt.Errorf("dump schema: %w", err)
		}
		objects = append(objects, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dump schema: %w", err)
	}
	return withoutShadowTables(objects), nil
}

// withoutShadowTables drops the shadow tables of virtual tables (FTS5 keeps its content in
// search_index_data, _idx, _content, _docsize and _config): CREATE VIRTUAL TABLE recreates them
// on replay, and the rows of the virtual table itself are dumped as plain INSERTs.
func withoutShadowTables(objects []schemaObject) []schemaObject {
	var virtual []string
	for _, o := range objects {
		if o.typ == "table" && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(o.sql)), "CREATE VIRTUAL TABLE") {
			virtual = append(virtual, o.name)
		}
	}
	kept := objects[:0]
	for _, o := range objects {
		shadow := false
		for _, v := range virtual {
			if o.typ == "table" && o.name != v && strings.HasPrefix(o.name, v+"_") {
				shadow = true
			}
		}
		if !shadow {
			kept = append(kept, o)
		}
	}
	return kept
}

func dumpRows(ctx context.Context, db *sql.DB, table string, w io.Writer) error {
	quoted := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+quoted) //nolint:gosec // G202: identifier quoted, read-only dump
	if err != nil {
		return fmt.Errorf("dump %s: %w", table, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("dump %s: %w", table, err)
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("dump %s: %w", table, err)
		}
		lits := make([]string, len(vals))
		for i, v := range vals {
			lits[i] = literal(v)
		}
		if _, err := fmt.Fprintf(w, "INSERT INTO %s VALUES (%s);\n", quoted, strings.Join(lits, ", ")); err != nil {
			return err
		}
	}
	return rows.Err()
}

func literal(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		if x {
			return "1"
		}
		return "0"
	case []byte:
		return "X'" + hex.EncodeToString(x) + "'"
	case string:
		return "'" + strings.ReplaceAll(x, "'", "''") + "'"
	default:
		return "'" + strings.ReplaceAll(fmt.Sprint(x), "'", "''") + "'"
	}
}
