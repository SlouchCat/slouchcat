package api

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// model.UILanguages is the one list the server checks a profile language against (concept 22
// section 4.3); it must be exactly the enum Language of the contract, which the web app ships a
// language pack for.
func TestUILanguagesMatchContract(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	language, _ := schemas["Language"].(map[string]any)
	values, _ := language["enum"].([]any)
	var contract []string
	for _, v := range values {
		contract = append(contract, fmt.Sprint(v))
	}
	if !slices.Equal(contract, model.UILanguages) {
		t.Errorf("contract Language = %v, model.UILanguages = %v", contract, model.UILanguages)
	}
	for _, code := range model.UILanguages {
		if !gen.Language(code).Valid() {
			t.Errorf("generated Language does not know %q", code)
		}
	}
}
