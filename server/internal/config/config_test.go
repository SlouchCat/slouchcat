package config

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/logging"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaults(t *testing.T) {
	cfg, err := Load(nil, envMap(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Listen: ":8096", ConfigDir: "/config", CacheDir: "/cache", LogLevel: slog.LevelInfo,
		LogFormat: logging.FormatJSON, FFmpeg: "ffmpeg", FFprobe: "ffprobe", Workers: cfg.Workers}
	if cfg.Workers < 2 || cfg.Workers > 4 {
		t.Fatalf("automatic workers = %d, want 2..4", cfg.Workers)
	}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestPrecedenceFlagsOverEnv(t *testing.T) {
	env := envMap(map[string]string{
		"SLOUCHCAT_LISTEN":     "127.0.0.1:9000",
		"SLOUCHCAT_CONFIG_DIR": "/srv/config",
		"SLOUCHCAT_LOG_LEVEL":  "debug",
		"SLOUCHCAT_LOG_FORMAT": "text",
		"SLOUCHCAT_DEV":        "true",
		"SLOUCHCAT_FFPROBE":    "/usr/lib/jellyfin-ffmpeg/ffprobe",
		"SLOUCHCAT_SPIKE_DIR":  "/srv/spike",
		"SLOUCHCAT_WORKERS":    "3",
	})
	cfg, err := Load([]string{"--listen", ":7000", "--cache-dir", "rel/cache", "--workers", "6"}, env)
	if err != nil {
		t.Fatal(err)
	}
	absCache, _ := filepath.Abs("rel/cache")
	want := Config{Listen: ":7000", ConfigDir: "/srv/config", CacheDir: absCache, LogLevel: slog.LevelDebug,
		LogFormat: logging.FormatText, Dev: true, FFmpeg: "ffmpeg", FFprobe: "/usr/lib/jellyfin-ffmpeg/ffprobe",
		SpikeDir: "/srv/spike", Workers: 6}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestValidation(t *testing.T) {
	cases := map[string]struct {
		args []string
		env  map[string]string
	}{
		"bad level":       {args: []string{"--log-level", "loud"}},
		"bad format":      {args: []string{"--log-format", "xml"}},
		"bad listen":      {args: []string{"--listen", "8096"}},
		"empty config":    {args: []string{"--config-dir", ""}},
		"same dirs":       {args: []string{"--config-dir", "/data", "--cache-dir", "/data"}},
		"bad dev env":     {env: map[string]string{"SLOUCHCAT_DEV": "maybe"}},
		"bad workers":     {env: map[string]string{"SLOUCHCAT_WORKERS": "many"}},
		"empty ffmpeg":    {args: []string{"--ffmpeg", ""}},
		"unknown flag":    {args: []string{"--nope"}},
		"extra arguments": {args: []string{"serve"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(tc.args, envMap(tc.env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
