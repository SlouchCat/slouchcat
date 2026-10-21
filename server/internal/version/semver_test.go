package version

import "testing"

func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
		ok   bool
	}{
		{"0.9.0", "0.9.0", 0, true},
		{"v0.9.0", "0.9.0", 0, true},
		{"0.9.0", "0.10.0", -1, true},
		{"1.0.0", "0.99.99", 1, true},
		{"0.9.1", "0.9.0", 1, true},
		{"0.9.0-rc.1", "0.9.0", -1, true},
		{"0.9.0", "0.9.0-rc.1", 1, true},
		{"0.9.0-rc.2", "0.9.0-rc.10", -1, true},
		{"0.9.0-alpha", "0.9.0-alpha.1", -1, true},
		{"0.9.0-1", "0.9.0-alpha", -1, true},
		{"0.9.0-beta", "0.9.0-alpha", 1, true},
		{"0.9.0+build.5", "0.9.0+other", 0, true},
		// git describe: commits after the tag and local changes are build metadata (review M-3).
		{"v0.8.0-5-g81fcf30", "0.8.0", 0, true},
		{"v0.9.0-3-gabc1234-dirty", "0.9.0", 0, true},
		{"v0.9.0-dirty", "0.9.0", 0, true},
		{"v0.9.0-rc.1-2-gabc1234", "0.9.0-rc.1", 0, true},
		{"v0.9.0-rc.1-2-gabc1234", "0.9.0", -1, true},
		{"v0.9.0-3-gabc1234", "0.9.1", -1, true},
		{"dev", "0.9.0", 0, false},
		{"0.9.0", "81fcf30-dirty", 0, false},
		{"0.9", "0.9.0", 0, false},
		{"01.9.0", "0.9.0", 0, false},
		{"0.9.0-", "0.9.0", 0, false},
		{"0.9.0-01", "0.9.0", 0, false},
		{"0.9.0+", "0.9.0", 0, false},
		{"0.9.0-rc..1", "0.9.0", 0, false},
	} {
		got, ok := Compare(tc.a, tc.b)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Compare(%q, %q) = %d, %v; want %d, %v", tc.a, tc.b, got, ok, tc.want, tc.ok)
		}
	}
}
