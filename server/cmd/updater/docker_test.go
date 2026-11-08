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

// The fields of docker container inspect the updater reads, in the places Docker puts them. A
// crash loop between two restarts reads running and restarting (seen on CT 190).
func TestDockerCLIContainer(t *testing.T) {
	fakeDockerBinary(t, `cat <<'JSON'
[{"Id":"abc","Image":"sha256:img","RestartCount":4,
  "State":{"Status":"restarting","Running":true,"Restarting":true,"ExitCode":1,"Health":{"Status":"unhealthy","FailingStreak":3}},
  "Config":{"Image":"ghcr.io/slouchcat/slouchcat:1.16.0","Labels":{"org.opencontainers.image.version":"1.16.0"}},
  "HostConfig":{"RestartPolicy":{"Name":"on-failure","MaximumRetryCount":5}}}]
JSON`)
	c, err := dockerCLI{}.Container(t.Context(), "slouchcat")
	if err != nil {
		t.Fatal(err)
	}
	want := Container{Image: "ghcr.io/slouchcat/slouchcat:1.16.0", Labels: map[string]string{labelVersion: "1.16.0"},
		Health: "unhealthy", Status: "restarting", ExitCode: 1, RestartCount: 4, RestartPolicy: "on-failure", MaxRetries: 5}
	if c.ImageID != "sha256:img" {
		t.Errorf("ImageID = %q", c.ImageID)
	}
	if c.Image != want.Image || c.Labels[labelVersion] != "1.16.0" || !c.Running || !c.Restarting || c.Health != want.Health || c.Status != want.Status ||
		c.ExitCode != want.ExitCode || c.RestartCount != want.RestartCount || c.RestartPolicy != want.RestartPolicy || c.MaxRetries != want.MaxRetries {
		t.Errorf("Container = %+v\nwant %+v", c, want)
	}
	if !c.GaveUp() {
		t.Error("a container restarting for the fourth time has not given up")
	}
}

// A failed call returns its last stderr line, redacted and cut at maxErrorLine, and tells a full
// disk from any stderr line.
func TestDockerCLICommandError(t *testing.T) {
	fakeDockerBinary(t, `echo "write /var/lib/docker/tmp: no space left on device" >&2
echo "failed: SLOUCHCAT_SMTP_PASSWORD=secret `+strings.Repeat("x", 400)+`" >&2
exit 1`)
	err := dockerCLI{}.Compose(t.Context(), Project{Name: "slouchcat", WorkingDir: "/srv"}, "pull", service)
	line := errorLine(err)
	if !diskFull(err) || len(line) != maxErrorLine || !strings.HasPrefix(line, "failed: SLOUCHCAT_SMTP_PASSWORD=[redacted] xxx") {
		t.Errorf("err = %v, line %q, disk full %v", err, line, diskFull(err))
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("the error carries the secret: %v", err)
	}
	fakeDockerBinary(t, `echo "toomanyrequests" >&2; exit 1`)
	err = dockerCLI{}.Compose(t.Context(), Project{Name: "slouchcat", WorkingDir: "/srv"}, "pull", service)
	if diskFull(err) || errorLine(err) != "toomanyrequests" || failedMessage("pull failed", err) != "pull failed: toomanyrequests" {
		t.Errorf("err = %v", err)
	}
}

func TestDockerCLIImagesAndRun(t *testing.T) {
	home := fakeDockerBinary(t, `printf 'sha256:a\tghcr.io/slouchcat/slouchcat\t1.16.0\tsha256:d1\nbroken line\nsha256:b\tghcr.io/slouchcat/slouchcat\t<none>\t<none>\n'`)
	list, err := dockerCLI{}.Images(t.Context(), "ghcr.io/slouchcat/slouchcat")
	want := []Image{{"sha256:a", "ghcr.io/slouchcat/slouchcat", "1.16.0", "sha256:d1"}, {"sha256:b", "ghcr.io/slouchcat/slouchcat", "<none>", "<none>"}}
	if err != nil || !slices.Equal(list, want) {
		t.Errorf("Images = %v, %v", list, err)
	}
	if got := readArgs(t, home); !slices.Equal(got, []string{"image", "ls", "--no-trunc", "--digests", "--format", imageFormat, "ghcr.io/slouchcat/slouchcat"}) {
		t.Errorf("args = %q", got)
	}
	if _, err := (dockerCLI{}).Images(t.Context(), ""); err == nil {
		t.Error("Images without a repository would list every image")
	}
	if err := (dockerCLI{}).RemoveImage(t.Context(), "ghcr.io/slouchcat/slouchcat:1.14.0"); err != nil {
		t.Fatal(err)
	}
	if got := readArgs(t, home); !slices.Equal(got, []string{"image", "rm", "ghcr.io/slouchcat/slouchcat:1.14.0"}) {
		t.Errorf("args = %q", got)
	}
	if err := (dockerCLI{}).Run(t.Context(), "-d", "--rm", "img", "self-update"); err != nil {
		t.Fatal(err)
	}
	if got := readArgs(t, home); !slices.Equal(got, []string{"run", "-d", "--rm", "img", "self-update"}) {
		t.Errorf("args = %q", got)
	}
}

// Logs reads both streams and returns the last lines, redacted; RemoveContainer forces only when
// asked.
func TestDockerCLILogsAndRemoveContainer(t *testing.T) {
	home := fakeDockerBinary(t, `echo "one"; echo "two SLOUCHCAT_SMTP_PASSWORD=hunter2"; echo; echo "three" >&2`)
	lines, err := dockerCLI{}.Logs(t.Context(), helperName, 2)
	if err != nil || !slices.Equal(lines, []string{"two SLOUCHCAT_SMTP_PASSWORD=[redacted]", "three"}) {
		t.Errorf("Logs = %q, %v", lines, err)
	}
	if got := readArgs(t, home); !slices.Equal(got, []string{"logs", "--tail", "2", helperName}) {
		t.Errorf("args = %q", got)
	}
	for _, force := range []bool{false, true} {
		if err := (dockerCLI{}).RemoveContainer(t.Context(), helperName, force); err != nil {
			t.Fatal(err)
		}
		want := []string{"container", "rm", helperName}
		if force {
			want = []string{"container", "rm", "--force", helperName}
		}
		if got := readArgs(t, home); !slices.Equal(got, want) {
			t.Errorf("args = %q, want %q", got, want)
		}
	}
}
