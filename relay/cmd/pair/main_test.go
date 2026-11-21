package main

import (
	"testing"
	"time"
)

// RELAY_PAIR_TTL: five minutes by default, shorter for tests, never longer (K-1, D-1).
func TestPairTTL(t *testing.T) {
	for _, c := range []struct {
		env  string
		want time.Duration
		ok   bool
	}{
		{"", 5 * time.Minute, true},
		{"30s", 30 * time.Second, true},
		{"5m", 5 * time.Minute, true},
		{"5m1s", 0, false},
		{"1h", 0, false},
		{"-1m", 0, false},
		{"soon", 0, false},
	} {
		t.Setenv("RELAY_PAIR_TTL", c.env)
		got, err := pairTTL()
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("RELAY_PAIR_TTL=%q: %s, %v", c.env, got, err)
		}
	}
}
