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
		LogFormat: logging.FormatJSON}
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
	})
	cfg, err := Load([]string{"--listen", ":7000", "--cache-dir", "rel/cache"}, env)
	if err != nil {
		t.Fatal(err)
	}
	absCache, _ := filepath.Abs("rel/cache")
	want := Config{Listen: ":7000", ConfigDir: "/srv/config", CacheDir: absCache, LogLevel: slog.LevelDebug,
		LogFormat: logging.FormatText, Dev: true}
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
