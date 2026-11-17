package arr

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSlug(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Kinder", "kinder"},
		{"Noah", "noah"},
		{"Jürg", "juerg"},
		{"Öli & Ärni", "oeli-aerni"},
		{"Grüsse", "gruesse"},
		{"Strasse", "strasse"},
		{"Straße", "strasse"},
		{"STRAẞE", "strasse"},
		{"ÄÖÜ", "aeoeue"},
		{"Zoë", "zoe"},
		{"José", "jose"},
		{"François", "francois"},
		{"Søren", "soren"},
		{"Łukasz", "lukasz"},
		{"Ærø", "aero"},
		{"Kim 2", "kim-2"},
		{"  Mia  Lou  ", "mia-lou"},
		{"mia_lou", "mia-lou"},
		{"--a--b--", "a-b"},
		{"Mia 🐱 Lou", "mia-lou"},
		{"🐱🐶", ""},
		{"   ", ""},
		{"!!!---???", ""},
		{"", ""},
		{"SC-Test2", "sc-test2"},
		{"Ünd", "uend"},  // composed Ü
		{"Ünd", "uend"}, // decomposed Ü (U + combining diaeresis)
		{"Кирилл", ""},   // no Latin letter
		{"Anna (Gäste)", "anna-gaeste"},
	} {
		if got := Slug(tc.in); got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if got := Slug(tc.in); got != "" && !radarrLabel(got) {
			t.Errorf("Slug(%q) = %q does not pass Radarr's rule", tc.in, got)
		}
	}
}

// radarrLabel is Radarr's rule ^[a-z0-9-]+$ (I-52), written out so the test does not reuse Slug.
func radarrLabel(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return s != ""
}

func TestProfileLabel(t *testing.T) {
	for _, tc := range []struct {
		prefix, name, want string
		ok                 bool
	}{
		{"sc-", "Kinder", "sc-kinder", true},
		{"sc-", "Jürg Müller", "sc-juerg-mueller", true},
		{"sc-", "🐱", "", false},
		{"sc-", "   ", "", false},
		{"sc-", "", "", false},
		{"SC_", "Noah", "sc-noah", true},
		{"", "Noah", "noah", true},
		{"slouchcat", "Mia", "slouchcatmia", true},
	} {
		got, ok := ProfileLabel(tc.prefix, tc.name)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ProfileLabel(%q, %q) = %q, %v; want %q, %v", tc.prefix, tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestValidLabel(t *testing.T) {
	for _, ok := range []string{"sc-kinder", "a", "0", "sc-kinder-grosseltern", "123-abc"} {
		if err := ValidLabel(ok); err != nil {
			t.Errorf("ValidLabel(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Sc-kinder", "sc kinder", "sc_kinder", "-sc", "sc-", "sc--kinder", "jürg", "🐱", " sc"} {
		if err := ValidLabel(bad); err == nil {
			t.Errorf("ValidLabel(%q) = nil, want an error", bad)
		}
	}
}

func TestParseTimeLeft(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"00:12:00", 12 * time.Minute, true},
		{"01:02:03", time.Hour + 2*time.Minute + 3*time.Second, true},
		{"1.02:03:04", 26*time.Hour + 3*time.Minute + 4*time.Second, true},
		{"12.00:00:00", 12 * 24 * time.Hour, true},
		{"00:00:00", 0, true},
		{"00:00:01.5000000", 1500 * time.Millisecond, true},
		{" 00:01:00 ", time.Minute, true},
		{"", 0, false},
		{"   ", 0, false},
		{"-00:01:00", 0, false},
		{"12:00", 0, false},
		{"aa:bb:cc", 0, false},
		{"00:60:00", 0, false},
		{"00:00:60", 0, false},
		{"1.:00:00", 0, false},
		{"00:+1:00", 0, false},
		{"999999999.00:00:00", 0, false},
	} {
		got, ok := ParseTimeLeft(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseTimeLeft(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestKeyCache(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	fetches := 0
	fail := false
	k := NewKeyCache(func(context.Context) (Credentials, error) {
		fetches++
		if fail {
			return Credentials{}, errors.New("seerr down")
		}
		return Credentials{BaseURL: "http://radarr.lan:7878", Key: Secret("k" + string(rune('0'+fetches)))}, nil
	}, WithKeyClock(func() time.Time { return now }))
	ctx := context.Background()

	c, err := k.Get(ctx)
	if err != nil || c.Key.Reveal() != "k1" || fetches != 1 {
		t.Fatalf("first Get = %v, %v, fetches %d", c, err, fetches)
	}
	now = now.Add(9*time.Minute + 59*time.Second)
	if c, _ := k.Get(ctx); c.Key.Reveal() != "k1" || fetches != 1 {
		t.Errorf("within 10 min: key %s, fetches %d", c.Key.Reveal(), fetches)
	}
	now = now.Add(time.Second)
	if c, _ := k.Get(ctx); c.Key.Reveal() != "k2" || fetches != 2 {
		t.Errorf("after 10 min: key %s, fetches %d", c.Key.Reveal(), fetches)
	}
	k.Invalidate() // after a 401
	if c, _ := k.Get(ctx); c.Key.Reveal() != "k3" || fetches != 3 {
		t.Errorf("after Invalidate: key %s, fetches %d", c.Key.Reveal(), fetches)
	}
	fail = true
	k.Invalidate()
	if _, err := k.Get(ctx); err == nil {
		t.Error("failed fetch gave no error")
	}
	if _, err := k.Get(ctx); err == nil || fetches != 5 {
		t.Errorf("failed fetch is kept: err %v, fetches %d", err, fetches)
	}
}
