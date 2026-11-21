package api

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// Compatibility rule for the operations the Android apps call (docs/dev/VERTRAEGLICHKEIT.md,
// concept 27 11.5, question F-9). TestXClientsCompatibility compares api/openapi.yaml with a frozen
// state of every operation that carries x-clients, and of the public operations in
// appPublicOperations (testdata/x-clients-frozen.json, taken from the last published contract
// version), and fails on a breaking change:
//   - the operation disappears, moves to another method or path, or x-clients drops a client;
//   - a request gains a required field or parameter, or a request field changes its type;
//   - an enum value of a request field or parameter disappears;
//   - a success response disappears, one of its fields disappears or changes its type, or a
//     required response field becomes optional.
//
// A deliberate break goes into testdata/x-clients-exceptions.json with the end of its transition
// period; it passes from that day on. An exception that no longer matches a problem fails the test,
// so the list shrinks again once the state is frozen anew.
//
// Freezing (`task server:freeze:x-clients REF=<git ref>`): with SLOUCHCAT_FREEZE_X_CLIENTS set to
// the path of an openapi.yaml, the test writes the frozen state of that file instead of comparing.

const (
	xClientsFrozenFile     = "testdata/x-clients-frozen.json"
	xClientsExceptionsFile = "testdata/x-clients-exceptions.json"
	xClientsFormat         = "slouchcat-x-clients-v1"
	xClientsFreezeEnv      = "SLOUCHCAT_FREEZE_X_CLIENTS"
	xClientsFreezeRefEnv   = "SLOUCHCAT_FREEZE_X_CLIENTS_REF"
	// schemaDepth bounds the walk through nested schemas; recursive schemas stop at their $ref.
	schemaDepth = 32
)

// appPublicOperations are the public operations the apps call. Public operations carry no
// x-clients (ADR 0040), so the rule names them here with the apps that call them; an x-clients of
// their own wins over this list.
var appPublicOperations = map[string][]string{
	"getHealth":               {"app", "tv"},
	"proveServer":             {"app", "tv"},
	"appLogin":                {"app", "tv"},
	"redeemPairingVoucher":    {"tv"},
	"checkOfflineRevocations": {"app"},
}

type frozenContract struct {
	Format          string                     `json:"format"`
	Source          string                     `json:"source"`
	ContractVersion string                     `json:"contractVersion"`
	Operations      map[string]frozenOperation `json:"operations"`
}

type frozenOperation struct {
	Method  string   `json:"method"`
	Path    string   `json:"path"`
	Clients []string `json:"clients"`
	// Request: every field and parameter with its type ("body.$.email", "query.limit"), the
	// required ones, and the allowed values of the enums.
	RequestFields   map[string]string   `json:"requestFields"`
	RequestRequired []string            `json:"requestRequired"`
	RequestEnums    map[string][]string `json:"requestEnums"`
	// Success responses with a JSON body, by status code.
	Responses map[string]frozenResponse `json:"responses"`
}

type frozenResponse struct {
	Fields   map[string]string `json:"fields"`
	Required []string          `json:"required"`
}

type xClientsException struct {
	Operation      string `json:"operation"`
	Problem        string `json:"problem"`
	TransitionEnds string `json:"transitionEnds"` // YYYY-MM-DD: the break is allowed from this day on
	Reason         string `json:"reason"`
}

// freezeXClients builds the frozen state of every operation in doc that carries x-clients.
func freezeXClients(doc map[string]any, source string) frozenContract {
	info := stringMap(doc["info"])
	version, _ := info["version"].(string)
	out := frozenContract{Format: xClientsFormat, Source: source, ContractVersion: version, Operations: map[string]frozenOperation{}}
	for path, rawItem := range stringMap(doc["paths"]) {
		item := stringMap(rawItem)
		for _, method := range httpMethods {
			op := stringMap(item[method])
			if op == nil {
				continue
			}
			clients := stringList(op["x-clients"])
			id, _ := op["operationId"].(string)
			if len(clients) == 0 {
				clients = slices.Clone(appPublicOperations[id])
			}
			if len(clients) == 0 || id == "" {
				continue
			}
			sort.Strings(clients)
			out.Operations[id] = freezeOperation(doc, strings.ToUpper(method), path, clients, item, op)
		}
	}
	return out
}

func freezeOperation(doc map[string]any, method, path string, clients []string, item, op map[string]any) frozenOperation {
	fo := frozenOperation{
		Method: method, Path: path, Clients: clients,
		RequestFields: map[string]string{}, RequestEnums: map[string][]string{}, Responses: map[string]frozenResponse{},
	}
	required := map[string]bool{}
	params := append(anyList(item["parameters"]), anyList(op["parameters"])...)
	for _, raw := range params {
		p := stringMap(derefNode(doc, raw))
		in, _ := p["in"].(string)
		name, _ := p["name"].(string)
		if in == "" || name == "" {
			continue
		}
		w := newSchemaWalk(doc)
		w.walk(p["schema"], in+"."+name, map[string]bool{}, 0, false)
		maps.Copy(fo.RequestFields, w.fields)
		maps.Copy(fo.RequestEnums, w.enums)
		if req, _ := p["required"].(bool); req || in == "path" {
			required[in+"."+name] = true
		}
	}
	body := stringMap(derefNode(doc, op["requestBody"]))
	if schema, ok := jsonSchema(body); ok {
		w := newSchemaWalk(doc)
		w.walk(schema, "body.$", map[string]bool{}, 0, false)
		maps.Copy(fo.RequestFields, w.fields)
		maps.Copy(fo.RequestEnums, w.enums)
		for k := range w.required {
			required[k] = true
		}
		if req, _ := body["required"].(bool); req {
			required["body.$"] = true
		}
	}
	fo.RequestRequired = sortedKeys(required)
	for code, raw := range stringMap(op["responses"]) {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		schema, ok := jsonSchema(stringMap(derefNode(doc, raw)))
		if !ok {
			continue
		}
		w := newSchemaWalk(doc)
		w.walk(schema, "$", map[string]bool{}, 0, false)
		fo.Responses[code] = frozenResponse{Fields: w.fields, Required: sortedKeys(w.required)}
	}
	return fo
}

// schemaWalk flattens a schema into field paths ("$.items[].id") with their type, the required
// paths and the enum values.
type schemaWalk struct {
	doc      map[string]any
	fields   map[string]string
	required map[string]bool
	enums    map[string][]string
}

func newSchemaWalk(doc map[string]any) *schemaWalk {
	return &schemaWalk{doc: doc, fields: map[string]string{}, required: map[string]bool{}, enums: map[string][]string{}}
}

// walk records node at path. optional is set inside oneOf and anyOf: a field required there is
// required only in one branch, so it does not count as required for the whole schema.
func (w *schemaWalk) walk(node any, path string, refs map[string]bool, depth int, optional bool) {
	s := stringMap(node)
	if s == nil || depth > schemaDepth {
		return
	}
	if ref, ok := s["$ref"].(string); ok {
		if refs[ref] {
			w.addType(path, "object")
			return
		}
		inner := maps.Clone(refs)
		inner[ref] = true
		w.walk(resolve(w.doc, ref), path, inner, depth+1, optional)
		return
	}
	for _, sub := range anyList(s["allOf"]) {
		w.walk(sub, path, refs, depth+1, optional)
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		for _, sub := range anyList(s[key]) {
			w.walk(sub, path, refs, depth+1, true)
		}
	}
	if t := schemaType(s); t != "" {
		w.addType(path, t)
	}
	if values := anyList(s["enum"]); len(values) > 0 {
		for _, v := range values {
			if v != nil {
				w.enums[path] = appendUnique(w.enums[path], fmt.Sprint(v))
			}
		}
	}
	props := stringMap(s["properties"])
	for name, sub := range props {
		w.walk(sub, path+"."+name, refs, depth+1, optional)
	}
	if !optional {
		for _, name := range stringList(s["required"]) {
			w.required[path+"."+name] = true
		}
	}
	if items, ok := s["items"]; ok {
		w.walk(items, path+"[]", refs, depth+1, optional)
	}
	if extra := stringMap(s["additionalProperties"]); extra != nil {
		w.walk(extra, path+"{}", refs, depth+1, optional)
	}
}

// addType records t for path; two schemas that give one path different types (allOf, oneOf) give
// both, sorted, so the result does not depend on the order of the walk.
func (w *schemaWalk) addType(path, t string) {
	old, ok := w.fields[path]
	if !ok {
		w.fields[path] = t
		return
	}
	parts := strings.Split(old, "|")
	if slices.Contains(parts, t) {
		return
	}
	parts = append(parts, t)
	sort.Strings(parts)
	w.fields[path] = strings.Join(parts, "|")
}

// schemaType is the JSON type of s without "null"; "object" for a schema with properties only.
func schemaType(s map[string]any) string {
	switch t := s["type"].(type) {
	case string:
		return t
	case []any:
		var names []string
		for _, v := range t {
			if name, ok := v.(string); ok && name != "null" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		return strings.Join(names, "|")
	}
	if s["properties"] != nil {
		return "object"
	}
	return ""
}

// compareXClients returns one message per breaking change from frozen to current, sorted.
func compareXClients(frozen, current frozenContract) []string {
	var problems []string
	add := func(id, format string, args ...any) {
		problems = append(problems, id+": "+fmt.Sprintf(format, args...))
	}
	for id, old := range frozen.Operations {
		cur, ok := current.Operations[id]
		if !ok {
			add(id, "operation removed or without x-clients")
			continue
		}
		if cur.Method != old.Method || cur.Path != old.Path {
			add(id, "moved from %s %s to %s %s", old.Method, old.Path, cur.Method, cur.Path)
		}
		for _, c := range old.Clients {
			if !slices.Contains(cur.Clients, c) {
				add(id, "x-clients drops %s", c)
			}
		}
		oldRequired := map[string]bool{}
		for _, r := range old.RequestRequired {
			oldRequired[r] = true
		}
		for _, r := range cur.RequestRequired {
			if oldRequired[r] {
				continue
			}
			// A required field inside an object the request did not have before is no break: an
			// old client does not send the object. The object itself is checked on its own.
			if parent, isField := parentPath(r); isField {
				if _, known := old.RequestFields[parent]; !known {
					continue
				}
			}
			add(id, "request requires %s", r)
		}
		for path, t := range old.RequestFields {
			if nt, ok := cur.RequestFields[path]; ok && nt != t {
				add(id, "request field %s changes type from %s to %s", path, t, nt)
			}
		}
		for path, values := range old.RequestEnums {
			now, ok := cur.RequestEnums[path]
			if !ok {
				continue // no enum any more, or no field: every old value is still accepted or ignored
			}
			for _, v := range values {
				if !slices.Contains(now, v) {
					add(id, "request enum %s drops %s", path, v)
				}
			}
		}
		for code, resp := range old.Responses {
			nr, ok := cur.Responses[code]
			if !ok {
				add(id, "response %s removed", code)
				continue
			}
			for path, t := range resp.Fields {
				nt, ok := nr.Fields[path]
				switch {
				case !ok:
					add(id, "response %s drops %s", code, path)
				case nt != t:
					add(id, "response %s field %s changes type from %s to %s", code, path, t, nt)
				}
			}
			for _, r := range resp.Required {
				if _, still := nr.Fields[r]; still && !slices.Contains(nr.Required, r) {
					add(id, "response %s field %s is no longer required", code, r)
				}
			}
		}
	}
	sort.Strings(problems)
	return problems
}

// applyXClientsExceptions returns the problems that no exception allows on today, an error for each
// exception whose transition period has not ended yet, and an error for each exception that matches
// no problem.
func applyXClientsExceptions(problems []string, exceptions []xClientsException, today time.Time) []string {
	var errs []string
	used := make([]bool, len(exceptions))
	for _, p := range problems {
		matched := false
		for i, e := range exceptions {
			if p != e.Operation+": "+e.Problem {
				continue
			}
			matched, used[i] = true, true
			ends, err := time.Parse(time.DateOnly, e.TransitionEnds)
			switch {
			case err != nil:
				errs = append(errs, fmt.Sprintf("exception %q: transitionEnds %q is no date (YYYY-MM-DD)", p, e.TransitionEnds))
			case today.Before(ends):
				errs = append(errs, fmt.Sprintf("%s: breaking change before the end of its transition period on %s", p, e.TransitionEnds))
			}
		}
		if !matched {
			errs = append(errs, p+" (breaking change of an operation with x-clients, docs/dev/VERTRAEGLICHKEIT.md)")
		}
	}
	for i, e := range exceptions {
		if !used[i] {
			errs = append(errs, fmt.Sprintf("exception %s: %s matches no change any more; remove it", e.Operation, e.Problem))
		}
	}
	return errs
}

func TestXClientsCompatibility(t *testing.T) {
	if src := os.Getenv(xClientsFreezeEnv); src != "" {
		freezeXClientsFile(t, src, os.Getenv(xClientsFreezeRefEnv))
		return
	}
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	current := freezeXClients(parseYAML(t, data), "api/openapi.yaml")
	var frozen frozenContract
	readJSON(t, xClientsFrozenFile, &frozen)
	if frozen.Format != xClientsFormat || frozen.Operations == nil {
		t.Fatalf("%s: format %q, want %q with an operations object", xClientsFrozenFile, frozen.Format, xClientsFormat)
	}
	var exceptions []xClientsException
	readJSON(t, xClientsExceptionsFile, &exceptions)
	for _, e := range applyXClientsExceptions(compareXClients(frozen, current), exceptions, time.Now().UTC()) {
		t.Error(e)
	}
}

func freezeXClientsFile(t *testing.T, src, ref string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if ref == "" {
		ref = src
	}
	frozen := freezeXClients(parseYAML(t, data), ref)
	out, err := json.MarshalIndent(frozen, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xClientsFrozenFile, append(out, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("froze %d operations with x-clients of contract %s (%s) into %s",
		len(frozen.Operations), frozen.ContractVersion, ref, xClientsFrozenFile)
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

const compatBase = `
openapi: 3.1.0
info: {version: 1.0.0}
paths:
  /things/{id}:
    parameters:
      - {name: id, in: path, required: true, schema: {type: string}}
    get:
      operationId: getThing
      x-clients: [app, tv]
      parameters:
        - $ref: "#/components/parameters/Sort"
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {$ref: "#/components/schemas/Thing"}
        "404": {description: missing}
    post:
      operationId: changeThing
      x-clients: [app]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name: {type: string}
                mode: {type: string, enum: [fast, slow]}
      responses:
        "204": {description: done}
  /admin/things:
    get:
      operationId: listThingsAdmin
      responses:
        "200": {description: ok}
  /healthz:
    get:
      operationId: getHealth
      x-visibility: public
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                required: [status]
                properties:
                  status: {type: string, enum: [ok]}
                  version: {type: string}
components:
  parameters:
    Sort:
      name: sort
      in: query
      schema: {type: string, enum: [name, date]}
  schemas:
    Thing:
      type: object
      required: [id, title]
      properties:
        id: {type: string}
        title: {type: string}
        size: {type: integer}
        parent: {$ref: "#/components/schemas/Thing"}
        tags:
          type: array
          items: {type: string}
`

func compatProblems(t *testing.T, mutate func(string) string) []string {
	t.Helper()
	base := freezeXClients(parseYAML(t, []byte(compatBase)), "base")
	changed := freezeXClients(parseYAML(t, []byte(mutate(compatBase))), "changed")
	return compareXClients(base, changed)
}

func TestXClientsFreezeShape(t *testing.T) {
	t.Parallel()
	f := freezeXClients(parseYAML(t, []byte(compatBase)), "base")
	if got := sortedKeys(f.Operations); !slices.Equal(got, []string{"changeThing", "getHealth", "getThing"}) {
		t.Fatalf("operations %v, want changeThing, getHealth (public app operation) and getThing, not the admin operation", got)
	}
	if got := f.Operations["getHealth"].Clients; !slices.Equal(got, []string{"app", "tv"}) {
		t.Errorf("getHealth clients %v, want those of appPublicOperations", got)
	}
	get := f.Operations["getThing"]
	if !slices.Equal(get.Clients, []string{"app", "tv"}) || get.Method != http.MethodGet || get.Path != "/things/{id}" {
		t.Errorf("getThing = %+v", get)
	}
	if !slices.Equal(get.RequestRequired, []string{"path.id"}) {
		t.Errorf("getThing required %v, want [path.id]", get.RequestRequired)
	}
	if !slices.Equal(get.RequestEnums["query.sort"], []string{"name", "date"}) {
		t.Errorf("getThing enums %v", get.RequestEnums)
	}
	resp := get.Responses["200"]
	for path, want := range map[string]string{"$": "object", "$.id": "string", "$.size": "integer", "$.tags[]": "string", "$.parent": "object"} {
		if resp.Fields[path] != want {
			t.Errorf("getThing 200 field %s = %q, want %q", path, resp.Fields[path], want)
		}
	}
	if _, ok := get.Responses["404"]; ok {
		t.Error("getThing: an error response was frozen")
	}
	if got := f.Operations["changeThing"].RequestRequired; !slices.Equal(got, []string{"body.$", "body.$.name", "path.id"}) {
		t.Errorf("changeThing required %v", got)
	}
}

func TestXClientsCompatibilityDetectsBreaks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(string) string
		want   []string
	}{
		{"operation removed", replace("operationId: changeThing\n      x-clients: [app]\n", "operationId: changeThing\n"),
			[]string{"changeThing: operation removed or without x-clients"}},
		{"client dropped", replace("x-clients: [app, tv]", "x-clients: [app]"),
			[]string{"getThing: x-clients drops tv"}},
		{"moved", replace("/things/{id}:", "/stuff/{id}:"),
			[]string{"changeThing: moved from POST /things/{id} to POST /stuff/{id}", "getThing: moved from GET /things/{id} to GET /stuff/{id}"}},
		{"new required request field", replace("required: [name]", "required: [name, mode]"),
			[]string{"changeThing: request requires body.$.mode"}},
		{"new required query parameter", replace("      name: sort\n      in: query\n", "      name: sort\n      in: query\n      required: true\n"),
			[]string{"getThing: request requires query.sort"}},
		{"request enum value dropped", replace("enum: [fast, slow]", "enum: [fast]"),
			[]string{"changeThing: request enum body.$.mode drops slow"}},
		{"parameter enum value dropped", replace("enum: [name, date]", "enum: [name]"),
			[]string{"getThing: request enum query.sort drops date"}},
		{"request field type changed", replace("name: {type: string}", "name: {type: integer}"),
			[]string{"changeThing: request field body.$.name changes type from string to integer"}},
		{"response field dropped", replace("        size: {type: integer}\n", ""),
			[]string{"getThing: response 200 drops $.size"}},
		{"response field type changed", replace("size: {type: integer}", "size: {type: string}"),
			[]string{"getThing: response 200 field $.size changes type from integer to string"}},
		{"public app operation drops a field", replace("                  version: {type: string}\n", ""),
			[]string{"getHealth: response 200 drops $.version"}},
		{"response no longer required", replace("required: [id, title]", "required: [id]"),
			[]string{"getThing: response 200 field $.title is no longer required"}},
		{"success response removed", replace(`        "200":
          description: ok
          content:`, `        "201":
          description: ok
          content:`),
			[]string{"getThing: response 200 removed"}},
	}
	for _, c := range cases {
		got := compatProblems(t, c.mutate)
		want := slices.Clone(c.want)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s:\n got  %q\n want %q", c.name, got, want)
		}
	}
}

func TestXClientsCompatibilityAllowsAdditions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(string) string
	}{
		{"new optional request field", replace("mode: {type: string, enum: [fast, slow]}", "mode: {type: string, enum: [fast, slow]}\n                note: {type: string}")},
		{"new optional object with a required field", replace("mode: {type: string, enum: [fast, slow]}",
			"mode: {type: string, enum: [fast, slow]}\n                extra:\n                  type: object\n                  required: [level]\n                  properties:\n                    level: {type: integer}")},
		{"new request enum value", replace("enum: [fast, slow]", "enum: [fast, slow, auto]")},
		{"new response field", replace("size: {type: integer}", "size: {type: integer}\n        color: {type: string}")},
		{"optional response field becomes required", replace("required: [id, title]", "required: [id, title, size]")},
		{"request field no longer required", replace("required: [name]", "required: []")},
		{"new client", replace("x-clients: [app]", "x-clients: [app, tv]")},
		{"new operation", replace("  /admin/things:\n    get:\n      operationId: listThingsAdmin\n",
			"  /admin/things:\n    get:\n      operationId: listThingsAdmin\n      x-clients: [app]\n")},
	}
	for _, c := range cases {
		if got := compatProblems(t, c.mutate); len(got) > 0 {
			t.Errorf("%s: additive change reported as break: %q", c.name, got)
		}
	}
}

func TestXClientsExceptions(t *testing.T) {
	t.Parallel()
	problems := []string{"getThing: x-clients drops tv", "getThing: response 200 drops $.size"}
	exceptions := []xClientsException{
		{Operation: "getThing", Problem: "x-clients drops tv", TransitionEnds: "2027-03-01", Reason: "test"},
		{Operation: "changeThing", Problem: "operation removed or without x-clients", TransitionEnds: "2027-01-01", Reason: "test"},
	}
	before := time.Date(2027, 2, 28, 0, 0, 0, 0, time.UTC)
	after := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	got := applyXClientsExceptions(problems, exceptions, before)
	want := []string{
		"getThing: x-clients drops tv: breaking change before the end of its transition period on 2027-03-01",
		"getThing: response 200 drops $.size (breaking change of an operation with x-clients, docs/dev/VERTRAEGLICHKEIT.md)",
		"exception changeThing: operation removed or without x-clients matches no change any more; remove it",
	}
	if !slices.Equal(got, want) {
		t.Errorf("before the end:\n got  %q\n want %q", got, want)
	}
	got = applyXClientsExceptions(problems[:1], exceptions[:1], after)
	if len(got) > 0 {
		t.Errorf("on the end date the break must pass, got %q", got)
	}
	got = applyXClientsExceptions(problems[:1], []xClientsException{{Operation: "getThing", Problem: "x-clients drops tv", TransitionEnds: "1. März"}}, after)
	if len(got) != 1 || !strings.Contains(got[0], "is no date") {
		t.Errorf("a broken date must fail, got %q", got)
	}
}

func replace(old, repl string) func(string) string {
	return func(s string) string {
		if !strings.Contains(s, old) {
			panic("compat test: " + old + " not in the base contract")
		}
		return strings.Replace(s, old, repl, 1)
	}
}

// parentPath returns the path of the object that holds the field path ("body.$.a.b" gives
// "body.$.a"); false for a parameter or the body itself.
func parentPath(path string) (string, bool) {
	if !strings.HasPrefix(path, "body.$.") {
		return "", false
	}
	return path[:strings.LastIndex(path, ".")], true
}

func jsonSchema(node map[string]any) (any, bool) {
	media := stringMap(stringMap(node["content"])["application/json"])
	if media == nil || media["schema"] == nil {
		return nil, false
	}
	return media["schema"], true
}

// derefNode follows the $ref chain of a parameter, request body or response.
func derefNode(doc map[string]any, node any) any {
	for range schemaDepth {
		m := stringMap(node)
		ref, ok := m["$ref"].(string)
		if !ok {
			return node
		}
		node = resolve(doc, ref)
	}
	return node
}

// stringMap returns node as a map with string keys; YAML gives map[any]any for mappings with
// keys that are no strings, for example unquoted status codes.
func stringMap(node any) map[string]any {
	switch n := node.(type) {
	case map[string]any:
		return n
	case map[any]any:
		out := make(map[string]any, len(n))
		for k, v := range n {
			out[fmt.Sprint(k)] = v
		}
		return out
	}
	return nil
}

func anyList(node any) []any {
	l, _ := node.([]any)
	return l
}

func stringList(node any) []string {
	var out []string
	for _, v := range anyList(node) {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func appendUnique(list []string, v string) []string {
	if slices.Contains(list, v) {
		return list
	}
	return append(list, v)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
