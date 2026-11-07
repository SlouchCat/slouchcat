package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeDockerBinary puts a docker script first on PATH that records its arguments in
// $HOME/args (HOME is one of the few variables dockerEnv passes on) and runs body.
func fakeDockerBinary(t *testing.T, body string) (home string) {
	t.Helper()
	bin, home := t.TempDir(), t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HOME/args\"\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", home)
	return home
}

// readArgs returns the arguments the fake docker binary got.
func readArgs(t *testing.T, home string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "args"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// The real output of compose config for compose.yaml + compose.intel.yaml + compose.updater.yaml
// (docker compose v5.6.1): with depends_on the updater service is part of it, and
// config --images printed two lines.
func TestDockerCLIComposeImageTwoServices(t *testing.T) {
	fixture, err := filepath.Abs(filepath.Join("testdata", "compose-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	home := fakeDockerBinary(t, "cat '"+fixture+"'")
	p := Project{Name: "slouchcat", WorkingDir: "/srv/slouchcat/deploy",
		ConfigFiles: []string{"/srv/slouchcat/deploy/compose.yaml", "/srv/slouchcat/deploy/compose.updater.yaml"},
		EnvFiles:    []string{"/srv/slouchcat/deploy/.env"}}
	ref, err := dockerCLI{}.ComposeImage(t.Context(), p, service)
	if err != nil || ref != "ghcr.io/slouchcat/slouchcat:1.15.0" {
		t.Fatalf("ComposeImage = %q, %v", ref, err)
	}
	want := append(p.args(), "config", "--format", "json", "slouchcat")
	if got := readArgs(t, home); !slices.Equal(got, want) {
		t.Errorf("docker args = %q\nwant %q", got, want)
	}
}

func TestComposeImage(t *testing.T) {
	for _, tc := range []struct {
		name, out string
		ok        bool
	}{
		{"one service", `{"services":{"slouchcat":{"image":"ghcr.io/slouchcat/slouchcat:1.16.0"}}}`, true},
		{"missing service", `{"services":{"updater":{"image":"ghcr.io/slouchcat/slouchcat-updater:1.16.0"}}}`, false},
		{"no image", `{"services":{"slouchcat":{"build":{"context":"."}}}}`, false},
		{"line break", `{"services":{"slouchcat":{"image":"a:1\nb:2"}}}`, false},
		{"images list", "ghcr.io/slouchcat/slouchcat-updater:1.16.0\nghcr.io/slouchcat/slouchcat:1.16.0\n", false},
	} {
		ref, err := composeImage([]byte(tc.out), service)
		if (err == nil) != tc.ok {
			t.Errorf("%s: composeImage = %q, %v", tc.name, ref, err)
		}
	}
}
