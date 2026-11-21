package seal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func keys(t *testing.T, current string, versions ...string) *Keys {
	t.Helper()
	f := File{Current: current, Keys: map[string]string{}}
	for _, v := range versions {
		f.Keys[v] = NewKey()
	}
	k, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestRoundTrip(t *testing.T) {
	k := keys(t, "1", "1")
	c := Content{Token: "fcm-token", VAPIDKey: "BPub"}
	s, err := k.Seal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "1.") || strings.Contains(s, "fcm-token") {
		t.Fatalf("sealed = %q", s)
	}
	got, err := k.Open(s)
	if err != nil || got != c {
		t.Fatalf("Open = %+v, %v", got, err)
	}
	if s2, _ := k.Seal(c); s2 == s {
		t.Fatal("two seals of the same content are equal (nonce reused)")
	}
}

// D-10: every way an address fails to open is the forwarder's own error, never "gone".
func TestOpenFailures(t *testing.T) {
	old := keys(t, "1", "1")
	s, _ := old.Seal(Content{Token: "t", VAPIDKey: "k"})
	other := keys(t, "1", "1") // same version name, different key: a wrong key file
	if _, err := other.Open(s); !errors.Is(err, ErrOpen) {
		t.Errorf("wrong key: %v", err)
	}
	newer := keys(t, "2", "2") // only a later version: the old one was dropped
	if _, err := newer.Open(s); !errors.Is(err, ErrUnknownVersion) {
		t.Errorf("unknown version: %v", err)
	}
	var none *Keys
	if _, err := none.Open(s); !errors.Is(err, ErrNoKey) {
		t.Errorf("no keys: %v", err)
	}
	if _, err := none.Seal(Content{}); !errors.Is(err, ErrNoKey) {
		t.Errorf("seal without keys: %v", err)
	}
	for _, bad := range []string{"", "1", "1.", "1.!!!", "UPPER.abc", "1." + strings.Repeat("A", 10)} {
		if _, err := old.Open(bad); err == nil {
			t.Errorf("Open(%q) succeeded", bad)
		}
	}
	// The version is bound: a blob of version 1 does not open when relabelled as version 2,
	// even when both keys are the same.
	same := NewKey()
	both, _ := Parse(File{Current: "1", Keys: map[string]string{"1": same, "2": same}})
	s1, _ := both.Seal(Content{Token: "t", VAPIDKey: "k"})
	if _, err := both.Open("2" + strings.TrimPrefix(s1, "1")); !errors.Is(err, ErrOpen) {
		t.Errorf("relabelled version: %v", err)
	}
	raw := []byte(s1)
	raw[len(raw)-2] ^= 1
	if _, err := both.Open(string(raw)); err == nil {
		t.Error("tampered blob opened")
	}
}

func TestRotationKeepsOldAddresses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sealing-keys.json")
	if err := AddVersion(path, "1"); err != nil {
		t.Fatal(err)
	}
	k1, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s1, _ := k1.Seal(Content{Token: "t1", VAPIDKey: "k"})
	if err := AddVersion(path, "2"); err != nil {
		t.Fatal(err)
	}
	if err := AddVersion(path, "2"); err == nil {
		t.Fatal("an existing version was overwritten")
	}
	k2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if k2.Current() != "2" || k2.Versions() != 2 {
		t.Fatalf("current %q, %d versions", k2.Current(), k2.Versions())
	}
	if c, err := k2.Open(s1); err != nil || c.Token != "t1" {
		t.Fatalf("old address after rotation: %+v, %v", c, err)
	}
	s2, _ := k2.Seal(Content{Token: "t2", VAPIDKey: "k"})
	if !strings.HasPrefix(s2, "2.") {
		t.Fatalf("new address %q not sealed with the current version", s2)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode %v, want 0600", fi.Mode().Perm())
	}
}

func TestParseRejects(t *testing.T) {
	for name, f := range map[string]File{
		"no current":    {Current: "9", Keys: map[string]string{"1": NewKey()}},
		"short key":     {Current: "1", Keys: map[string]string{"1": "AAAA"}},
		"bad version":   {Current: "a.b", Keys: map[string]string{"a.b": NewKey()}},
		"empty":         {},
		"upper version": {Current: "V1", Keys: map[string]string{"V1": NewKey()}},
	} {
		if _, err := Parse(f); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("missing file accepted")
	}
}
