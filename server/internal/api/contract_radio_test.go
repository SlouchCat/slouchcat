package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Webradio contract (concept 16 stage 1, contract 1.17.0): no radio schema and no radio operation
// refers to ItemRef, directly or through a chain of $ref (12.3: "Antworten enthalten kein ItemRef").
// A station is not a title; an ItemRef would carry scoped title image URLs into the radio views.
func TestRadioContractHasNoItemRef(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	var radioSchemas int
	for name, schema := range schemas {
		if !strings.HasPrefix(name, "Radio") && !strings.HasPrefix(name, "AdminRadio") {
			continue
		}
		radioSchemas++
		if references(doc, schema, itemRefPointer, map[string]bool{}) {
			t.Errorf("schema %s refers to ItemRef", name)
		}
	}
	if radioSchemas < 20 {
		t.Errorf("%d radio schemas in the contract, want the full set of contract 1.17.0", radioSchemas)
	}
	paths, _ := doc["paths"].(map[string]any)
	var radioOps int
	for path, item := range paths {
		if !strings.HasPrefix(path, "/radio/") && !strings.HasPrefix(path, "/admin/radio/") {
			continue
		}
		ops, _ := item.(map[string]any)
		for _, method := range httpMethods {
			op, ok := ops[method].(map[string]any)
			if !ok {
				continue
			}
			radioOps++
			if references(doc, op, itemRefPointer, map[string]bool{}) {
				t.Errorf("%s %s refers to ItemRef", strings.ToUpper(method), path)
			}
		}
	}
	if radioOps != 30 {
		t.Errorf("%d radio operations, want the 30 of contract 1.17.0", radioOps)
	}
}

// The radio errors carry what the client needs: radio_capacity the numbers of the relays (running,
// max), radio_catalog_loading the pause in Retry-After.
func TestRadioErrorExtensions(t *testing.T) {
	t.Parallel()
	p := problems{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	p.mapError(rec, httptest.NewRequest(http.MethodPost, "/api/v1/radio/stations/1/listen", nil), &model.RadioCapacityError{Running: 7, Max: 7})
	var body struct {
		Code       string         `json:"code"`
		Extensions map[string]int `json:"extensions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusConflict || body.Code != "radio_capacity" || body.Extensions["running"] != 7 || body.Extensions["max"] != 7 {
		t.Errorf("capacity: status %d, body %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	p.mapError(rec, httptest.NewRequest(http.MethodGet, "/api/v1/radio/stations", nil), model.ErrRadioCatalogLoading)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "30" {
		t.Errorf("catalog loading: status %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}
