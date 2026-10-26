package api

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// homeRowsCheck matches the key list of the CHECK constraint of the rebuilt home_rows table.
var homeRowsCheck = regexp.MustCompile(`(?s)CREATE TABLE home_rows_new \(\s*key\s+TEXT\s+PRIMARY KEY CHECK \(key IN \(([^)]*)\)\)`)

// E-9, ADR 0011: the start page row keys live in three places, the HomeRowKey enum of the
// contract, model.HomeRowKeys and the CHECK of home_rows in migration 00021. All three name the
// same set, so a key cannot be accepted by one layer and refused by another.
func TestHomeRowKeysAgree(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	schema, _ := resolve(parseYAML(t, data), "#/components/schemas/HomeRowKey").(map[string]any)
	values, _ := schema["enum"].([]any)
	var contract []string
	for _, v := range values {
		contract = append(contract, fmt.Sprint(v))
	}

	migration, err := os.ReadFile("../../migrations/00021_spotlight.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := homeRowsCheck.FindSubmatch(migration)
	if m == nil {
		t.Fatal("migration 00021 has no CHECK (key IN (...)) on home_rows_new")
	}
	var check []string
	for _, k := range strings.Split(string(m[1]), ",") {
		check = append(check, strings.Trim(strings.TrimSpace(k), "'"))
	}

	want := slices.Sorted(slices.Values(model.HomeRowKeys))
	if len(slices.Compact(slices.Clone(want))) != 8 {
		t.Fatalf("model.HomeRowKeys = %v, want eight distinct keys", model.HomeRowKeys)
	}
	for place, got := range map[string][]string{"contract HomeRowKey": contract, "migration 00021 CHECK": check} {
		if got := slices.Sorted(slices.Values(got)); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want the keys of model.HomeRowKeys %v", place, got, want)
		}
	}
}
