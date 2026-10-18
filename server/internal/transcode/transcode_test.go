package transcode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeBinary writes an executable shell script that stands in for ffprobe.
func fakeBinary(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffprobe")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVersion(t *testing.T) {
	bin := fakeBinary(t, `[ "$1" = "-version" ] || exit 2
echo "ffprobe version 7.1.2-Jellyfin Copyright (c) 2007-2025 the FFmpeg developers"
echo "built with gcc 14"
`)
	got, err := Runner{FFprobePath: bin}.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := "ffprobe version 7.1.2-Jellyfin Copyright (c) 2007-2025 the FFmpeg developers"; got != want {
		t.Fatalf("Version() = %q, want %q", got, want)
	}
}

func TestVersionErrors(t *testing.T) {
	failing := fakeBinary(t, "echo 'cannot load libva' >&2\nexit 1\n")
	if _, err := (Runner{FFprobePath: failing}).Version(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "cannot load libva") {
		t.Fatalf("want error with stderr, got %v", err)
	}
	if _, err := (Runner{}).Version(context.Background()); err == nil {
		t.Fatal("want error for an unconfigured path")
	}
	if _, err := (Runner{FFprobePath: filepath.Join(t.TempDir(), "missing")}).Version(context.Background()); err == nil {
		t.Fatal("want error for a missing binary")
	}
}
