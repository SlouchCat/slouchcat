package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// contract is the part of api/openapi.yaml this client needs: the server's base path, the API
// version and every operation with its path template, method and x-visibility marker.
type contract struct {
	title      string
	version    string
	basePath   string
	operations []operation
}

// operation is one path item method of the contract.
type operation struct {
	id         string
	method     string
	path       string // path template as the contract writes it, e.g. /items/{id}/play
	visibility string
	pattern    *regexp.Regexp
}

// methods the client may use; any other key of a path item (parameters, summary, …) is ignored.
var methods = []string{"get", "post", "put", "patch", "delete", "head", "options"}

// placeholder matches a path parameter of a path template, e.g. {id} in /items/{id}/play.
var placeholder = regexp.MustCompile(`\{[^/{}]+\}`)

// contractDoc is the shape of openapi.yaml this client reads. Everything else stays a raw node:
// the client needs the operations, not the schemas.
type contractDoc struct {
	Info struct {
		Title   string `yaml:"title"`
		Version string `yaml:"version"`
	} `yaml:"info"`
	Servers []struct {
		URL string `yaml:"url"`
	} `yaml:"servers"`
	Paths map[string]map[string]yaml.Node `yaml:"paths"`
}

// operationDoc is the part of a path item method the client reads.
type operationDoc struct {
	OperationID string `yaml:"operationId"`
	Visibility  string `yaml:"x-visibility"`
}

// loadContract reads api/openapi.yaml and builds the operation list.
func loadContract(path string) (*contract, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: the contract file comes from a flag of this developer tool
	if err != nil {
		return nil, fmt.Errorf("read contract: %w", err)
	}
	var doc contractDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse contract %s: %w", path, err)
	}
	if len(doc.Servers) == 0 {
		return nil, fmt.Errorf("contract %s names no server", path)
	}
	c := &contract{
		title:      doc.Info.Title,
		version:    doc.Info.Version,
		basePath:   strings.TrimRight(doc.Servers[0].URL, "/"),
		operations: nil,
	}
	for template, item := range doc.Paths {
		for _, method := range methods {
			node, ok := item[method]
			if !ok {
				continue
			}
			var op operationDoc
			if err := node.Decode(&op); err != nil {
				return nil, fmt.Errorf("parse %s %s: %w", strings.ToUpper(method), template, err)
			}
			c.operations = append(c.operations, operation{
				id:         op.OperationID,
				method:     strings.ToUpper(method),
				path:       template,
				visibility: op.Visibility,
				pattern:    pathPattern(template),
			})
		}
	}
	sort.Slice(c.operations, func(i, j int) bool {
		if c.operations[i].path != c.operations[j].path {
			return c.operations[i].path < c.operations[j].path
		}
		return c.operations[i].method < c.operations[j].method
	})
	if len(c.operations) == 0 {
		return nil, fmt.Errorf("contract %s holds no operation", path)
	}
	return c, nil
}

// pathPattern turns a path template into the regexp that its concrete paths match: every path
// parameter stands for exactly one segment, everything else is literal.
func pathPattern(template string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString(`\A`)
	last := 0
	for _, m := range placeholder.FindAllStringIndex(template, -1) {
		b.WriteString(regexp.QuoteMeta(template[last:m[0]]))
		b.WriteString(`[^/]+`)
		last = m[1]
	}
	b.WriteString(regexp.QuoteMeta(template[last:]))
	b.WriteString(`\z`)
	return regexp.MustCompile(b.String())
}

// errUnknownOperation reports a call the contract does not describe. A third-party client that
// knows only the contract must never make one, so every such call fails the run.
var errUnknownOperation = errors.New("path and method are not in the contract")

// lookup finds the operation of a concrete request. A literal path wins over a path template
// (/admin/items/search before /admin/items/{id}).
func (c *contract) lookup(method, path string) (operation, error) {
	for _, op := range c.operations {
		if op.method == method && op.path == path {
			return op, nil
		}
	}
	for _, op := range c.operations {
		if op.method == method && op.pattern.MatchString(path) {
			return op, nil
		}
	}
	return operation{}, fmt.Errorf("%s %s: %w", method, path, errUnknownOperation)
}
