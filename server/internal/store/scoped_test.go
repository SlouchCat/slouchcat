package store

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every sqlc query in queries/scoped must join the view visible_items (plan 3.3): the generated
// package scopedq is the only read path of the visibility package, so a query without the join
// would leak titles to every profile.
func TestScopedQueriesJoinVisibleItems(t *testing.T) {
	dir := filepath.Join("..", "..", "queries", "scoped")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	queryHeader := regexp.MustCompile(`(?m)^-- name: (\w+)`)
	found := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		parts := queryHeader.Split(string(data), -1)
		names := queryHeader.FindAllStringSubmatch(string(data), -1)
		for i, name := range names {
			found++
			body := parts[i+1]
			if !strings.Contains(body, "visible_items") {
				t.Errorf("%s: query %s does not join visible_items", e.Name(), name[1])
			}
		}
	}
	if found == 0 {
		t.Fatal("no scoped queries found")
	}
}
