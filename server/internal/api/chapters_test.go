package api

import (
	"encoding/json"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// toChapters sends auto and imageUrl only when set (contract 1.12.0) and never a nil list.
func TestToChapters(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(toChapters([]model.Chapter{
		{StartMs: 0, EndMs: 600_000, Auto: true},
		{StartMs: 600_000, EndMs: 900_000, Title: "Credits", ImageURL: "/api/v1/items/x/chapters/1/image?mediaFileId=y"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"auto":true,"endMs":600000,"startMs":0},` +
		`{"endMs":900000,"imageUrl":"/api/v1/items/x/chapters/1/image?mediaFileId=y","startMs":600000,"title":"Credits"}]`
	if string(b) != want {
		t.Errorf("chapters = %s, want %s", b, want)
	}
	if b, _ := json.Marshal(toChapters(nil)); string(b) != "[]" {
		t.Errorf("nil chapters = %s", b)
	}
}
