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
		DownloadBudgetGB: DefaultDownloadBudgetGB, SMTPTLS: "starttls", UpdateManifestURL: DefaultUpdateManifestURL,
		BrowseRoots: []string{DefaultBrowseRoots}}
	if cfg.Workers < 2 || cfg.Workers > 4 {
		t.Fatalf("automatic workers = %d, want 2..4", cfg.Workers)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestPrecedenceFlagsOverEnv(t *testing.T) {
	env := envMap(map[string]string{
		"SLOUCHCAT_LISTEN":              "127.0.0.1:9000",
		"SLOUCHCAT_CONFIG_DIR":          "/srv/config",
		"SLOUCHCAT_LOG_LEVEL":           "debug",
		"SLOUCHCAT_LOG_FORMAT":          "text",
		"SLOUCHCAT_DEV":                 "true",
		"SLOUCHCAT_FFPROBE":             "/usr/lib/jellyfin-ffmpeg/ffprobe",
		"SLOUCHCAT_WORKERS":             "3",
		"SLOUCHCAT_HW":                  "nvenc",
		"SLOUCHCAT_HDR_TONEMAP":         "gpu",
		"SLOUCHCAT_TRANSCODE_BUDGET":    "6.5",
		"SLOUCHCAT_DOWNLOAD_BUDGET_GB":  "250",
		"SLOUCHCAT_TVDB_API_KEY":        " tvdb-key ",
		"SLOUCHCAT_TVDB_PIN":            "1234",
		"SLOUCHCAT_YOUTUBE_API_KEY":     "yt-key",
		"SLOUCHCAT_TRAKT_CLIENT_ID":     "trakt-id",
		"SLOUCHCAT_TRAKT_BASE_URL":      "http://127.0.0.1:9/trakt",
		"SLOUCHCAT_TRAILER_AI":          "off",
		"SLOUCHCAT_SMTP_HOST":           " smtp.example.test ",
		"SLOUCHCAT_SMTP_USER":           "mailer",
		"SLOUCHCAT_SMTP_PASSWORD":       "smtp-FAKE-password",
		"SLOUCHCAT_SMTP_FROM":           "Slouchcat <media@example.test>",
		"SLOUCHCAT_SMTP_TLS":            "TLS",
		"SLOUCHCAT_TELEGRAM_BOT_TOKEN":  "123456:bot-FAKE-token",
		"SLOUCHCAT_TELEGRAM_BASE_URL":   "http://127.0.0.1:9/telegram",
		"SLOUCHCAT_UPDATE_MANIFEST_URL": "http://127.0.0.1:9/manifest.json",
		"SLOUCHCAT_PUBLIC_URL":          "https://media.example.test/",
	})
	cfg, err := Load([]string{"--listen", ":7000", "--cache-dir", "rel/cache", "--workers", "6", "--hw", "software", "--hdr-tonemap", "cpu",
		"--trailer-ai", "Experimental"}, env)
	if err != nil {
		t.Fatal(err)
	}
	absCache, _ := filepath.Abs("rel/cache")
	want := Config{Listen: ":7000", ConfigDir: "/srv/config", CacheDir: absCache, LogLevel: slog.LevelDebug,
		LogFormat: logging.FormatText, Dev: true, FFmpeg: "ffmpeg", FFprobe: "/usr/lib/jellyfin-ffmpeg/ffprobe",
		Workers: 6, HW: "software", HDRTonemap: "cpu", TranscodeBudget: 6.5, DownloadBudgetGB: 250,
		TVDBAPIKey: "tvdb-key", TVDBPIN: "1234", YouTubeAPIKey: "yt-key", TrailerAI: "experimental",
		TraktClientID: "trakt-id", TraktBaseURL: "http://127.0.0.1:9/trakt",
		SMTPHost: "smtp.example.test", SMTPPort: 465, SMTPUser: "mailer", SMTPPassword: "smtp-FAKE-password",
		SMTPFrom: "Slouchcat <media@example.test>", SMTPTLS: "tls", TelegramBotToken: "123456:bot-FAKE-token",
		TelegramBaseURL: "http://127.0.0.1:9/telegram", UpdateManifestURL: "http://127.0.0.1:9/manifest.json",
		PublicURL: "https://media.example.test", BrowseRoots: []string{DefaultBrowseRoots}}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
	// The startup log line names which providers are configured, never their keys.
	logged := cfg.LogValue().String()
	for _, secret := range []string{"tvdb-key", "1234", "yt-key", "smtp-FAKE-password", "bot-FAKE-token", "mailer"} {
		if strings.Contains(logged, secret) {
			t.Errorf("LogValue contains the secret %q: %s", secret, logged)
		}
	}
	if !strings.Contains(logged, "tvdb_configured=true") || !strings.Contains(logged, "youtube_configured=true") ||
		!strings.Contains(logged, "smtp_configured=true") || !strings.Contains(logged, "telegram_configured=true") {
		t.Errorf("LogValue without provider state: %s", logged)
	}
	// Development mode honours the mock URLs of the Telegram bot and the update manifest.
	if cfg.TelegramAPIBase() != "http://127.0.0.1:9/telegram" || cfg.ManifestURL() != "http://127.0.0.1:9/manifest.json" {
		t.Errorf("dev mode: telegram %q, manifest %q", cfg.TelegramAPIBase(), cfg.ManifestURL())
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
		"bad smtp tls":         {env: map[string]string{"SLOUCHCAT_SMTP_TLS": "ssl"}},
		"bad smtp port":        {env: map[string]string{"SLOUCHCAT_SMTP_PORT": "70000"}},
		"smtp port not number": {env: map[string]string{"SLOUCHCAT_SMTP_PORT": "submission"}},
		"bad manifest url":     {env: map[string]string{"SLOUCHCAT_UPDATE_MANIFEST_URL": "ftp://example.test/manifest.json"}},
		"bad public url":       {env: map[string]string{"SLOUCHCAT_PUBLIC_URL": "media.example.test"}},
		"public url query":     {env: map[string]string{"SLOUCHCAT_PUBLIC_URL": "https://media.example.test/?x=1"}},
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

// M8: the e-mail server defaults its port by the encryption mode; the mock URLs of the Telegram bot
// and the update manifest are ignored outside development mode.
func TestNotifyConfig(t *testing.T) {
	for mode, port := range map[string]int{"": 587, "starttls": 587, "tls": 465, "none": 25} {
		cfg, err := Load(nil, envMap(map[string]string{"SLOUCHCAT_SMTP_HOST": "smtp.example.test", "SLOUCHCAT_SMTP_TLS": mode}))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.SMTPPort != port {
			t.Errorf("SMTP_TLS %q: port %d, want %d", mode, cfg.SMTPPort, port)
		}
	}
	cfg, err := Load(nil, envMap(map[string]string{"SLOUCHCAT_SMTP_PORT": "2525", "SLOUCHCAT_SMTP_HOST": "smtp.example.test"}))
	if err != nil || cfg.SMTPPort != 2525 || cfg.SMTPTLS != "starttls" {
		t.Errorf("explicit port: %+v, %v", cfg, err)
	}
	cfg, err = Load(nil, envMap(map[string]string{
		"SLOUCHCAT_TELEGRAM_BASE_URL": "http://127.0.0.1:9/telegram", "SLOUCHCAT_UPDATE_MANIFEST_URL": "http://127.0.0.1:9/manifest.json",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramAPIBase() != "" || cfg.ManifestURL() != DefaultUpdateManifestURL {
		t.Errorf("without --dev: telegram %q, manifest %q; want the real API and the default manifest", cfg.TelegramAPIBase(), cfg.ManifestURL())
	}
	if logged := cfg.LogValue().String(); !strings.Contains(logged, "update_manifest_default=true") || !strings.Contains(logged, "smtp_configured=false") {
		t.Errorf("LogValue: %s", logged)
	}
}

// A whole private network as a trusted proxy is warned about, a named one is not (M-1 of the
// security review of 23 October 2026).
func TestTrustedProxyWarning(t *testing.T) {
	for _, tc := range []struct {
		value string
		warn  bool
	}{
		{"", false},
		{"private", true},
		{"192.168.0.0/16", true},
		{"10.0.0.0/8, 172.18.0.0/16", true},
		{"172.18.0.0/16", false},
		{"172.18.0.2", false},
	} {
		proxies, err := ParseTrustedProxies(tc.value)
		if err != nil {
			t.Fatalf("%q: %v", tc.value, err)
		}
		got := Config{TrustedProxies: proxies}.TrustedProxyWarning()
		if (got != "") != tc.warn {
			t.Errorf("TrustedProxies %q: warning %q, want warning = %v", tc.value, got, tc.warn)
		}
	}
}

func TestBrowseRoots(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  []string
	}{
		{"", nil},
		{"   ", nil},
		{",,", nil},
		{"/media", []string{"/media"}},
		{"/media, /mnt/nas/", []string{"/media", "/mnt/nas"}},
		{"/media,/media/", []string{"/media"}},
		{"/media/./Filme", []string{"/media/Filme"}},
	} {
		got, err := ParseBrowseRoots(tc.value)
		if err != nil {
			t.Errorf("ParseBrowseRoots(%q): %v", tc.value, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseBrowseRoots(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
	for _, value := range []string{"media", "./media", "/media,relative"} {
		if _, err := ParseBrowseRoots(value); err == nil {
			t.Errorf("ParseBrowseRoots(%q): expected an error for a path that is not absolute", value)
		}
	}
	// A root that does not exist is no error here: it is logged and skipped at start-up, so a typo
	// does not keep the server from coming up (idea 02, 6.1).
	if got, err := ParseBrowseRoots("/does/not/exist"); err != nil || !reflect.DeepEqual(got, []string{"/does/not/exist"}) {
		t.Errorf("ParseBrowseRoots of a missing directory = %v, %v; want it kept", got, err)
	}
}

func TestBrowseRootsFromFlagAndEnv(t *testing.T) {
	env := envMap(map[string]string{"SLOUCHCAT_BROWSE_ROOTS": "/srv/media"})
	cfg, err := Load(nil, env)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.BrowseRoots, []string{"/srv/media"}) {
		t.Errorf("env: browse roots = %v", cfg.BrowseRoots)
	}
	cfg, err = Load([]string{"--browse-roots", "/a,/b"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.BrowseRoots, []string{"/a", "/b"}) {
		t.Errorf("flag: browse roots = %v", cfg.BrowseRoots)
	}
	if _, err := Load([]string{"--browse-roots", "relative"}, envMap(nil)); err == nil {
		t.Error("expected an error for a browse root that is not absolute")
	}
	// Switched off: the folder browser has nothing to list and library paths are not restricted.
	cfg, err = Load([]string{"--browse-roots", ""}, envMap(nil))
	if err != nil || cfg.BrowseRoots != nil {
		t.Errorf("empty browse roots = %v, %v", cfg.BrowseRoots, err)
	}
}

func TestBrowseRootWarning(t *testing.T) {
	for _, tc := range []struct {
		value string
		warn  bool
	}{
		{"/media", false},
		// A value that falls apart into no root at all switches the rule off; that must be said.
		{"", true},
		{"  ", true},
		{",,", true},
		{"/media,/", true},
		{"/", true},
	} {
		roots, err := ParseBrowseRoots(tc.value)
		if err != nil {
			t.Fatalf("%q: %v", tc.value, err)
		}
		if got := (Config{BrowseRoots: roots}).BrowseRootWarning(); (got != "") != tc.warn {
			t.Errorf("BrowseRoots %q: warning %q, want warning = %v", tc.value, got, tc.warn)
		}
	}
}
