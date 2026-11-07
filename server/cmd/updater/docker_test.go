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

func TestLastLine(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"pulling\nError response from daemon: manifest unknown\n", "Error response from daemon: manifest unknown"},
		{"a\r\nfailed to read .env: line 3: SLOUCHCAT_SMTP_PASSWORD=hunter2 unexpected character\r\n\r\n",
			"failed to read .env: line 3: SLOUCHCAT_SMTP_PASSWORD=[redacted] unexpected character"},
		{`invalid "TOKEN='a b c'" and KEY="x y", next=z`, `invalid "TOKEN=[redacted]" and KEY=[redacted], next=[redacted]`},
		{"unterminated PW=\"abc def", "unterminated PW=[redacted]"},
		{"https://example.test/m.json?token=abc", "https://example.test/m.json?token=[redacted]"},
		{"", ""},
		{strings.Repeat("x", 600), strings.Repeat("x", maxLogLine)},
	} {
		if got := lastLine([]byte(tc.in)); got != tc.want {
			t.Errorf("lastLine(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A failed command reports its exit status and only its redacted last line, never earlier lines.
func TestDockerCLIFailedOutput(t *testing.T) {
	fakeDockerBinary(t, `echo "SLOUCHCAT_TMDB_API_KEY=first-secret" >&2
echo "line 7: SLOUCHCAT_SMTP_PASSWORD=second-secret: invalid" >&2
exit 3`)
	var gotErr error
	var gotLine string
	d := dockerCLI{failed: func(_ []string, err error, last string) { gotErr, gotLine = err, last }}
	if err := d.Compose(t.Context(), Project{Name: "slouchcat", WorkingDir: "/srv"}, "pull", service); err == nil {
		t.Fatal("Compose succeeded")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "exit status 3") {
		t.Errorf("err = %v, want exit status 3", gotErr)
	}
	if gotLine != "line 7: SLOUCHCAT_SMTP_PASSWORD=[redacted] invalid" { // the value runs to the next space
		t.Errorf("last line = %q", gotLine)
	}
}
