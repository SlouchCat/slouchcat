package itemsaccess

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestViolations(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer, "example.com/app/internal/discover")
}

func TestAllowedPackages(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer,
		"example.com/app/internal/library", "example.com/app/internal/visibility",
		"example.com/app/internal/store/systemq")
}

func TestAllowedFlagOverride(t *testing.T) {
	// Package reports contains title SQL and no want comments: it passes only when allowed.
	if err := Analyzer.Flags.Set("allowed", "reports"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Analyzer.Flags.Set("allowed", DefaultAllowed) })
	analysistest.Run(t, analysistest.TestData(), Analyzer, "example.com/app/internal/reports")
}

func TestIsAllowed(t *testing.T) {
	names := []string{"store", "jobs"}
	for path, want := range map[string]bool{
		"m/internal/store":          true,
		"m/internal/store/systemq":  true,
		"m/internal/jobs_test":      true,
		"m/internal/storefront":     false,
		"m/internal/discover":       false,
		"m/cmd/slouchcat":           false,
		"m/internal/discover/store": false,
		"m/internal/api/gen":        false,
	} {
		if got := isAllowed(path, names); got != want {
			t.Errorf("isAllowed(%q) = %v, want %v", path, got, want)
		}
	}
}
