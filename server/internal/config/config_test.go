package config

import (
	"log/slog"
	"net/netip"
	"path/filepath"
	"reflect"
	"strings"
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
		LogFormat: logging.FormatJSON, FFmpeg: "ffmpeg", FFprobe: "ffprobe", Workers: cfg.Workers, HW: "auto", HDRTonemap: "auto", TrailerAI: "off",
		DownloadBudgetGB: DefaultDownloadBudgetGB}
	if cfg.Workers < 2 || cfg.Workers > 4 {
		t.Fatalf("automatic workers = %d, want 2..4", cfg.Workers)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestPrecedenceFlagsOverEnv(t *testing.T) {
	env := envMap(map[string]string{
		"SLOUCHCAT_LISTEN":             "127.0.0.1:9000",
		"SLOUCHCAT_CONFIG_DIR":         "/srv/config",
		"SLOUCHCAT_LOG_LEVEL":          "debug",
		"SLOUCHCAT_LOG_FORMAT":         "text",
		"SLOUCHCAT_DEV":                "true",
		"SLOUCHCAT_FFPROBE":            "/usr/lib/jellyfin-ffmpeg/ffprobe",
		"SLOUCHCAT_SPIKE_DIR":          "/srv/spike",
		"SLOUCHCAT_WORKERS":            "3",
		"SLOUCHCAT_HW":                 "nvenc",
		"SLOUCHCAT_HDR_TONEMAP":        "gpu",
		"SLOUCHCAT_TRANSCODE_BUDGET":   "6.5",
		"SLOUCHCAT_DOWNLOAD_BUDGET_GB": "250",
		"SLOUCHCAT_TVDB_API_KEY":       " tvdb-key ",
		"SLOUCHCAT_TVDB_PIN":           "1234",
		"SLOUCHCAT_YOUTUBE_API_KEY":    "yt-key",
		"SLOUCHCAT_TRAKT_CLIENT_ID":    "trakt-id",
		"SLOUCHCAT_TRAKT_BASE_URL":     "http://127.0.0.1:9/trakt",
		"SLOUCHCAT_TRAILER_AI":         "off",
	})
	cfg, err := Load([]string{"--listen", ":7000", "--cache-dir", "rel/cache", "--workers", "6", "--hw", "software", "--hdr-tonemap", "cpu",
		"--trailer-ai", "Experimental"}, env)
	if err != nil {
		t.Fatal(err)
	}
	absCache, _ := filepath.Abs("rel/cache")
	want := Config{Listen: ":7000", ConfigDir: "/srv/config", CacheDir: absCache, LogLevel: slog.LevelDebug,
		LogFormat: logging.FormatText, Dev: true, FFmpeg: "ffmpeg", FFprobe: "/usr/lib/jellyfin-ffmpeg/ffprobe",
		SpikeDir: "/srv/spike", Workers: 6, HW: "software", HDRTonemap: "cpu", TranscodeBudget: 6.5, DownloadBudgetGB: 250,
		TVDBAPIKey: "tvdb-key", TVDBPIN: "1234", YouTubeAPIKey: "yt-key", TrailerAI: "experimental",
		TraktClientID: "trakt-id", TraktBaseURL: "http://127.0.0.1:9/trakt"}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
	// The startup log line names which providers are configured, never their keys.
	logged := cfg.LogValue().String()
	for _, secret := range []string{"tvdb-key", "1234", "yt-key"} {
		if strings.Contains(logged, secret) {
			t.Errorf("LogValue contains the secret %q: %s", secret, logged)
		}
	}
	if !strings.Contains(logged, "tvdb_configured=true") || !strings.Contains(logged, "youtube_configured=true") {
		t.Errorf("LogValue without provider state: %s", logged)
	}
	if cfg.TranscodeDir() != filepath.Join(absCache, "transcode") {
		t.Errorf("transcode dir = %q", cfg.TranscodeDir())
	}
	if cfg.DownloadsDir() != filepath.Join(absCache, "downloads") {
		t.Errorf("downloads dir = %q", cfg.DownloadsDir())
	}
}

func TestValidation(t *testing.T) {
	cases := map[string]struct {
		args []string
		env  map[string]string
	}{
		"bad level":            {args: []string{"--log-level", "loud"}},
		"bad format":           {args: []string{"--log-format", "xml"}},
		"bad listen":           {args: []string{"--listen", "8096"}},
		"empty config":         {args: []string{"--config-dir", ""}},
		"same dirs":            {args: []string{"--config-dir", "/data", "--cache-dir", "/data"}},
		"bad dev env":          {env: map[string]string{"SLOUCHCAT_DEV": "maybe"}},
		"bad workers":          {env: map[string]string{"SLOUCHCAT_WORKERS": "many"}},
		"empty ffmpeg":         {args: []string{"--ffmpeg", ""}},
		"bad hw":               {env: map[string]string{"SLOUCHCAT_HW": "cuda"}},
		"bad hdr tonemap":      {env: map[string]string{"SLOUCHCAT_HDR_TONEMAP": "fast"}},
		"bad budget":           {env: map[string]string{"SLOUCHCAT_TRANSCODE_BUDGET": "lots"}},
		"negative budget":      {args: []string{"--transcode-budget", "-1"}},
		"bad download budget":  {env: map[string]string{"SLOUCHCAT_DOWNLOAD_BUDGET_GB": "big"}},
		"zero download budget": {args: []string{"--download-budget-gb", "0"}},
		"unknown flag":         {args: []string{"--nope"}},
		"extra arguments":      {args: []string{"serve"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(tc.args, envMap(tc.env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestTrustedProxies(t *testing.T) {
	cfg, err := Load(nil, envMap(nil))
	if err != nil || len(cfg.TrustedProxies) != 0 {
		t.Fatalf("default: %v, %v (want none: X-Forwarded-For ignored)", cfg.TrustedProxies, err)
	}
	cfg, err = Load(nil, envMap(map[string]string{"SLOUCHCAT_TRUSTED_PROXIES": "private"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.0.9", "127.0.0.1", "::1", "fd12:3456::1"} {
		if !trusts(cfg.TrustedProxies, ip) {
			t.Errorf("private must trust %s", ip)
		}
	}
	for _, ip := range []string{"172.32.0.1", "11.0.0.1", "8.8.8.8", "2001:db8::1", "fe80::1"} {
		if trusts(cfg.TrustedProxies, ip) {
			t.Errorf("private must not trust %s", ip)
		}
	}
	// A flag wins over the environment; CIDRs and single addresses mix, blanks are ignored.
	cfg, err = Load([]string{"--trusted-proxies", " 203.0.113.0/24 , 2001:db8::7,,198.51.100.4 "}, envMap(map[string]string{"SLOUCHCAT_TRUSTED_PROXIES": "private"}))
	if err != nil {
		t.Fatal(err)
	}
	for ip, want := range map[string]bool{"203.0.113.77": true, "2001:db8::7": true, "2001:db8::8": false, "198.51.100.4": true, "198.51.100.5": false, "10.0.0.1": false} {
		if trusts(cfg.TrustedProxies, ip) != want {
			t.Errorf("trusts(%s) = %v, want %v", ip, !want, want)
		}
	}
	for _, bad := range []string{"nonsense", "10.0.0.0/33", "10.0.0.1, x"} {
		if _, err := Load([]string{"--trusted-proxies", bad}, envMap(nil)); err == nil {
			t.Errorf("trusted proxies %q accepted", bad)
		}
	}
}

func trusts(prefixes []netip.Prefix, ip string) bool {
	addr := netip.MustParseAddr(ip)
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
