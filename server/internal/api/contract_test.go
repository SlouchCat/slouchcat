package api

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Contract rules for api/openapi.yaml (CLAUDE.md rule 7, plan section 3.3):
//   - every operation carries x-visibility with one of the allowed values;
//   - every operation whose responses reference the schema ItemRef, directly or through any
//     chain of $ref, is scoped;
//   - every operation whose path contains /images/ has the path parameter {id}, so no image is
//     delivered without the item whose visibility decides it.

var (
	visibilities = []string{"public", "account", "scoped", "system", "admin"}
	httpMethods  = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
)

const itemRefPointer = "#/components/schemas/ItemRef"

// checkContract returns one message per rule violation in doc, sorted.
func checkContract(doc map[string]any) []string {
	var problems []string
	paths, _ := doc["paths"].(map[string]any)
	for path, item := range paths {
		ops, _ := item.(map[string]any)
		for _, method := range httpMethods {
			op, ok := ops[method].(map[string]any)
			if !ok {
				continue
			}
			name := fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			vis, _ := op["x-visibility"].(string)
			switch {
			case vis == "":
				problems = append(problems, name+": missing x-visibility")
				continue
			case !slices.Contains(visibilities, vis):
				problems = append(problems, fmt.Sprintf("%s: x-visibility %q not in %v", name, vis, visibilities))
				continue
			}
			if vis != "scoped" && references(doc, op["responses"], itemRefPointer, map[string]bool{}) {
				problems = append(problems, fmt.Sprintf("%s: responses contain ItemRef, x-visibility must be scoped, is %q", name, vis))
			}
			if strings.Contains(path, "/images/") && !strings.Contains(path, "{id}") {
				problems = append(problems, name+": image path without the item path parameter {id}")
			}
		}
	}
	sort.Strings(problems)
	return problems
}

// references reports whether node contains a $ref to target, following local references.
func references(doc map[string]any, node any, target string, seen map[string]bool) bool {
	switch n := node.(type) {
	case map[string]any:
		if ref, ok := n["$ref"].(string); ok {
			if ref == target {
				return true
			}
			if !seen[ref] {
				seen[ref] = true
				if references(doc, resolve(doc, ref), target, seen) {
					return true
				}
			}
		}
		for _, v := range n {
			if references(doc, v, target, seen) {
				return true
			}
		}
	case map[any]any: // mappings with non-string keys, for example unquoted status codes
		for _, v := range n {
			if references(doc, v, target, seen) {
				return true
			}
		}
	case []any:
		for _, v := range n {
			if references(doc, v, target, seen) {
				return true
			}
		}
	}
	return false
}

// resolve follows a local JSON pointer such as "#/components/schemas/Health".
func resolve(doc map[string]any, ref string) any {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	var cur any = doc
	for _, part := range strings.Split(ref[2:], "/") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[strings.NewReplacer("~1", "/", "~0", "~").Replace(part)]
	}
	return cur
}

func parseYAML(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestContractOpenAPI(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	if paths, _ := doc["paths"].(map[string]any); len(paths) == 0 {
		t.Fatal("api/openapi.yaml has no paths")
	}
	for _, p := range checkContract(doc) {
		t.Error(p)
	}
}

func TestContractRejectsViolations(t *testing.T) {
	t.Parallel()
	doc := parseYAML(t, []byte(`
openapi: 3.1.0
paths:
  /no-marker:
    get:
      operationId: noMarker
      responses:
        "200": {description: ok}
  /typo:
    post:
      x-visibility: private
      responses:
        "204": {description: ok}
  /leak:
    get:
      x-visibility: public
      responses:
        200:
          $ref: "#/components/responses/ItemList"
  /images/{kind}:
    get:
      x-visibility: scoped
      responses:
        "200": {description: ok}
  /items/{id}/images/{kind}:
    get:
      x-visibility: scoped
      responses:
        "200": {description: ok}
  /fine:
    get:
      x-visibility: scoped
      responses:
        "200":
          $ref: "#/components/responses/ItemList"
  /health:
    get:
      x-visibility: public
      responses:
        "200":
          content:
            application/json:
              schema: {$ref: "#/components/schemas/Loop"}
components:
  responses:
    ItemList:
      description: list
      content:
        application/json:
          schema: {$ref: "#/components/schemas/Page"}
  schemas:
    Page:
      type: object
      properties:
        items: {type: array, items: {$ref: "#/components/schemas/ItemRef"}}
    ItemRef: {type: object}
    Loop:
      type: object
      properties:
        self: {$ref: "#/components/schemas/Loop"}
`))
	got := checkContract(doc)
	t.Logf("violations:\n%s", strings.Join(got, "\n"))
	want := []string{
		"GET /images/{kind}: image path without the item path parameter {id}",
		`GET /leak: responses contain ItemRef, x-visibility must be scoped, is "public"`,
		"GET /no-marker: missing x-visibility",
		`POST /typo: x-visibility "private" not in [public account scoped system admin]`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

// The binary routes mounted by hand carry the same x-visibility as the contract.
func TestBinaryRoutesMatchContract(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	paths, _ := doc["paths"].(map[string]any)
	for route, want := range binaryRoutes {
		method, pattern, _ := strings.Cut(route, " ")
		item, _ := paths[strings.TrimPrefix(pattern, BasePath)].(map[string]any)
		op, _ := item[strings.ToLower(method)].(map[string]any)
		if got, _ := op["x-visibility"].(string); got != want {
			t.Errorf("%s: contract says %q, binaryRoutes says %q", route, got, want)
		}
	}
}

// K-1: profiles carry no e-mail and no password; a profile request needs only the name.
func TestContractProfileHasNoCredentials(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	for _, name := range []string{"Profile", "ProfileSummary", "ProfileRequest", "ProfileUpdate"} {
		schema, _ := resolve(doc, "#/components/schemas/"+name).(map[string]any)
		props, _ := schema["properties"].(map[string]any)
		if len(props) == 0 {
			t.Fatalf("schema %s has no properties", name)
		}
		for key := range props {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "mail") || strings.Contains(lower, "password") {
				t.Errorf("schema %s has a credential field %q (K-1)", name, key)
			}
		}
	}
	req, _ := resolve(doc, "#/components/schemas/ProfileRequest").(map[string]any)
	var required []string
	if list, ok := req["required"].([]any); ok {
		for _, r := range list {
			required = append(required, fmt.Sprint(r))
		}
	}
	if !slices.Equal(required, []string{"name"}) {
		t.Errorf("ProfileRequest.required = %v, want [name] (K-1: image and everything else optional)", required)
	}
}

// I-7: Trakt is reached with the server key only. No profile schema and no operation under
// /profiles or /auth mentions Trakt; the word appears only in the admin Trakt operations, their
// TraktStatus schema, the spotlight configuration and its entries (M11: the Trakt list the start
// page row shows), the start page rows and the server settings of the administration (M8: the
// stored client id and its integration state).
func TestContractNoTraktPerProfile(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	schemas, _ := resolve(doc, "#/components/schemas").(map[string]any)
	for name, schema := range schemas {
		lower := strings.ToLower(name)
		mentions := strings.Contains(strings.ToLower(fmt.Sprint(schema)), "trakt")
		switch {
		case strings.HasPrefix(lower, "profile") || strings.HasPrefix(lower, "guest") || strings.HasPrefix(lower, "invitation") ||
			strings.HasPrefix(lower, "session") || strings.HasPrefix(lower, "account"):
			if mentions {
				t.Errorf("schema %s mentions Trakt (I-7: no Trakt account per profile)", name)
			}
		case strings.HasPrefix(lower, "trakt") || strings.HasPrefix(lower, "homerow") ||
			strings.HasPrefix(lower, "spotlight") || strings.HasPrefix(lower, "adminsettings") ||
			lower == "integrationstatus":
		default:
			if mentions {
				t.Errorf("schema %s mentions Trakt; only the admin state, the spotlight, the home rows and the admin settings may", name)
			}
		}
	}
	paths, _ := doc["paths"].(map[string]any)
	for path, item := range paths {
		mentions := strings.Contains(strings.ToLower(fmt.Sprint(item)), "trakt")
		if strings.HasPrefix(path, "/admin/trakt") || strings.HasPrefix(path, "/admin/spotlight") || path == "/home" {
			continue
		}
		if mentions {
			t.Errorf("operation %s mentions Trakt; only /admin/trakt*, /admin/spotlight* and /home may", path)
		}
	}
	if _, ok := paths["/admin/trakt"]; !ok {
		t.Error("missing /admin/trakt")
	}
}
