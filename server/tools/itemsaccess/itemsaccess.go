// Package itemsaccess defines an analyzer that keeps title data behind the visibility layer
// (plan section 3.3, CLAUDE.md rule 2).
//
// Outside the allowed packages it reports
//
//   - string constants (literals, raw strings and constant concatenations) with SQL that reads
//     or writes the tables items, media_files or tracks, and
//   - uses of the function System of a package whose import path ends in /visibility, which
//     creates an unfiltered scope.
//
// A package is allowed when its import path contains /internal/<name> for one of the names in
// the -allowed flag, including subpackages (store/systemq holds the generated system queries).
package itemsaccess

import (
	"go/ast"
	"go/constant"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// DefaultAllowed lists the packages that may touch title tables directly.
const DefaultAllowed = "visibility,admin,library,metadata,jobs,notify,offline,store"

// Analyzer reports direct title table access outside the allowed packages.
var Analyzer = &analysis.Analyzer{
	Name:     "itemsaccess",
	Doc:      "reports SQL on items, media_files or tracks and visibility.System outside the allowed packages",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var allowed string

func init() {
	Analyzer.Flags.StringVar(&allowed, "allowed", DefaultAllowed,
		"comma-separated package names under internal/ that may access title tables")
}

var titleSQL = regexp.MustCompile("(?i)\\b(from|join|into|update)\\s+[\"'`\\[]?(items|media_files|tracks)\\b")

func run(pass *analysis.Pass) (any, error) {
	if isAllowed(pass.Pkg.Path(), strings.Split(allowed, ",")) {
		return nil, nil
	}
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	filter := []ast.Node{(*ast.BasicLit)(nil), (*ast.BinaryExpr)(nil), (*ast.Ident)(nil)}
	ins.Nodes(filter, func(n ast.Node, push bool) bool {
		if !push {
			return false
		}
		switch n := n.(type) {
		case *ast.Ident:
			if isVisibilitySystem(pass.TypesInfo.Uses[n]) {
				pass.Reportf(n.Pos(), "visibility.System creates an unfiltered scope; it is allowed only in %s", allowed)
			}
		case *ast.BasicLit, *ast.BinaryExpr:
			tv, ok := pass.TypesInfo.Types[n.(ast.Expr)]
			if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
				return true // not a string constant: inspect the operands of a concatenation
			}
			if m := titleSQL.FindString(constant.StringVal(tv.Value)); m != "" {
				pass.Reportf(n.Pos(), "SQL %q accesses a title table; query through the visibility package instead", m)
			}
			return false // the whole constant was checked
		}
		return true
	})
	return nil, nil
}

// isAllowed reports whether path is one of the allowed packages under internal/ or below one.
func isAllowed(path string, names []string) bool {
	path = strings.TrimSuffix(path, "_test")
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if strings.Contains(path+"/", "/internal/"+name+"/") {
			return true
		}
	}
	return false
}

func isVisibilitySystem(obj types.Object) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Name() != "System" || fn.Pkg() == nil {
		return false
	}
	if sig, ok := fn.Type().(*types.Signature); !ok || sig.Recv() != nil {
		return false
	}
	return strings.HasSuffix(fn.Pkg().Path(), "/visibility")
}
