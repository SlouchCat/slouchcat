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
	// Tables first with their rows, so triggers and views never see a half-filled table.
	for _, o := range objects {
		if o.typ != "table" {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s;\n", strings.TrimSpace(o.sql)); err != nil {
			return err
		}
		if err := dumpRows(ctx, db, o.name, w); err != nil {
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
	return objects, nil
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
