// Package config loads the server configuration from command-line flags and environment
// variables (prefix SLOUCHCAT_). Flags win over environment variables, which win over defaults.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/notify/email"
)

// EnvPrefix is the prefix of every environment variable the server reads.
const EnvPrefix = "SLOUCHCAT_"

// Config is the validated server configuration.
type Config struct {
	Listen    string         // TCP address of the HTTP server, for example ":8096".
	ConfigDir string         // Absolute path of the persistent directory (database, backups).
	CacheDir  string         // Absolute path of the regenerable cache directory.
	LogLevel  slog.Level     // Minimum level that is logged.
	LogFormat logging.Format // json or text.
	Dev       bool           // Development mode.
	FFmpeg    string         // ffmpeg binary (name on PATH or absolute path).
	FFprobe   string         // ffprobe binary (name on PATH or absolute path).
	Workers   int            // Number of job workers.
	// HW selects the transcode backend: auto (detect), nvenc, qsv, vaapi or software (M1b).
	HW string
	// HDRTonemap selects where HDR → SDR tonemapping runs: auto, gpu or cpu (M4, transcode.TonemapPref*).
	HDRTonemap string
	// TrustedProxies are the networks whose X-Forwarded-For header the server believes (reverse
	// proxy in front); empty ignores the header.
	TrustedProxies []netip.Prefix
	// BrowseRoots are the directories the folder browser of the library form may list, and the
	// directories a new library root must lie below (idea 02, E4 and E14). Absolute, cleaned and
	// without duplicates; the default is DefaultBrowseRoots.
	BrowseRoots []string
	// TranscodeBudget overrides the admission budget of the transcode backend; 0 = backend default.
	TranscodeBudget float64
	// DownloadBudgetGB caps the prepared downloads in <cache>/downloads (AN-20, decision 8: 100 GB);
	// when full, new preparations wait. The admin setting follows in M8.
	DownloadBudgetGB int
	// TVDBAPIKey and TVDBPIN enable the optional TVDB provider for series (M3); empty = off.
	// Secrets come from the environment only: command-line flags show up in process lists.
	TVDBAPIKey string
	TVDBPIN    string
	// YouTubeAPIKey enables the YouTube step of the trailer chain (M3); empty = off.
	YouTubeAPIKey string
	// TrailerAI selects the AI step of the trailer chain; "off" (default) is the only provider in M3.
	TrailerAI string
	// TraktClientID enables the daily Trakt list (M7, M11, AN-3, I-7); empty = off, the spotlight row
	// stays hidden. TraktBaseURL overrides the API base for mocks (development and tests only).
	TraktClientID string
	TraktBaseURL  string
	// SMTP* configure the e-mail server of the notifications (M8, A-6); empty SMTPHost = none (the
	// admin may store one in the settings). SMTPTLS is starttls (default), tls or none; SMTPPort
	// defaults to 587, 465 or 25 by SMTPTLS when a host is set. Secrets come from the environment only.
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPTLS      string
	// TelegramBotToken enables the Telegram channel (M8); TelegramBaseURL points the bot client at a
	// mock and is honoured in development mode only (TelegramAPIBase).
	TelegramBotToken string
	TelegramBaseURL  string
	// UpdateManifestURL is the release manifest of the update check (M8, A-5); another value than
	// DefaultUpdateManifestURL is honoured in development mode only (ManifestURL).
	UpdateManifestURL string
	// PublicURL is the address of the server for links in e-mails and messages (M8); it wins over the
	// stored setting. Empty = the setting or no links.
	PublicURL string
}

// DefaultUpdateManifestURL is the release manifest the update check reads (M8, A-5).
const DefaultUpdateManifestURL = "https://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json"

// TelegramAPIBase returns the base URL of the Telegram bot API client: TelegramBaseURL in
// development mode (mocks), else empty, which means the real API.
func (c Config) TelegramAPIBase() string {
	if !c.Dev {
		return ""
	}
	return c.TelegramBaseURL
}

// ManifestURL returns the release manifest the update check reads: UpdateManifestURL in development
// mode, else DefaultUpdateManifestURL.
func (c Config) ManifestURL() string {
	if !c.Dev || c.UpdateManifestURL == "" {
		return DefaultUpdateManifestURL
	}
	return c.UpdateManifestURL
}

// TranscodeDir is the work directory of HLS output: <cache>/transcode, wiped at every start.
func (c Config) TranscodeDir() string { return filepath.Join(c.CacheDir, "transcode") }

// DownloadsDir holds the prepared downloads (M5, AN-20): <cache>/downloads. Unlike the transcode
// directory it survives restarts, so an interrupted preparation continues.
func (c Config) DownloadsDir() string { return filepath.Join(c.CacheDir, "downloads") }

// DefaultDownloadBudgetGB is the download budget without SLOUCHCAT_DOWNLOAD_BUDGET_GB (decision 8).
const DefaultDownloadBudgetGB = 100

// DefaultBrowseRoots is the value of SLOUCHCAT_BROWSE_ROOTS when it is not set: the media mount of
// deploy/compose.yaml, which is the only one a normal installation has (idea 02, E4).
const DefaultBrowseRoots = "/media"

// hwBackends are the values of SLOUCHCAT_HW / --hw.
var hwBackends = map[string]bool{"auto": true, "nvenc": true, "qsv": true, "vaapi": true, "software": true}

// hdrTonemaps are the values of SLOUCHCAT_HDR_TONEMAP / --hdr-tonemap.
var hdrTonemaps = map[string]bool{"auto": true, "gpu": true, "cpu": true}

// Load parses args (without the program name and subcommand) and the environment read through
// getenv, then validates the result.
func Load(args []string, getenv func(string) string) (Config, error) {
	fs := flag.NewFlagSet("slouchcat", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	env := func(name, def string) string {
		if v := getenv(EnvPrefix + name); v != "" {
			return v
		}
		return def
	}
	listen := fs.String("listen", env("LISTEN", ":8096"), "HTTP listen address (env SLOUCHCAT_LISTEN)")
	configDir := fs.String("config-dir", env("CONFIG_DIR", "/config"), "persistent data directory (env SLOUCHCAT_CONFIG_DIR)")
	cacheDir := fs.String("cache-dir", env("CACHE_DIR", "/cache"), "cache directory (env SLOUCHCAT_CACHE_DIR)")
	logLevel := fs.String("log-level", env("LOG_LEVEL", "info"), "debug, info, warn or error (env SLOUCHCAT_LOG_LEVEL)")
	logFormat := fs.String("log-format", env("LOG_FORMAT", string(logging.FormatJSON)), "json or text (env SLOUCHCAT_LOG_FORMAT)")
	devDefault, err := parseBool(env("DEV", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("%sDEV: %w", EnvPrefix, err)
	}
	dev := fs.Bool("dev", devDefault, "development mode (env SLOUCHCAT_DEV)")
	ffmpeg := fs.String("ffmpeg", env("FFMPEG", "ffmpeg"), "ffmpeg binary (env SLOUCHCAT_FFMPEG)")
	ffprobe := fs.String("ffprobe", env("FFPROBE", "ffprobe"), "ffprobe binary (env SLOUCHCAT_FFPROBE)")
	workers := fs.Int("workers", 0, "number of job workers, 0 = automatic (env SLOUCHCAT_WORKERS)")
	hw := fs.String("hw", env("HW", "auto"), "transcode backend: auto, nvenc, qsv, vaapi or software (env SLOUCHCAT_HW)")
	hdrTonemap := fs.String("hdr-tonemap", env("HDR_TONEMAP", "auto"), "HDR to SDR tonemapping: auto, gpu or cpu (env SLOUCHCAT_HDR_TONEMAP)")
	proxies := fs.String("trusted-proxies", env("TRUSTED_PROXIES", ""),
		"reverse proxies whose X-Forwarded-For is trusted: empty = none, private = RFC 1918, loopback and fc00::/7, or comma-separated CIDRs (env SLOUCHCAT_TRUSTED_PROXIES)")
	browseRoots := fs.String("browse-roots", env("BROWSE_ROOTS", DefaultBrowseRoots),
		"directories the folder browser may list and new library folders must lie below, comma-separated (env SLOUCHCAT_BROWSE_ROOTS)")
	budget := fs.Float64("transcode-budget", 0, "transcode admission budget, 0 = backend default (env SLOUCHCAT_TRANSCODE_BUDGET)")
	downloadBudget := fs.Int("download-budget-gb", DefaultDownloadBudgetGB, "space for prepared downloads in GB (env SLOUCHCAT_DOWNLOAD_BUDGET_GB)")
	trailerAI := fs.String("trailer-ai", env("TRAILER_AI", "off"), "AI step of the trailer search: off (env SLOUCHCAT_TRAILER_AI)")
	if v := env("TRANSCODE_BUDGET", ""); v != "" {
		b, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Config{}, fmt.Errorf("%sTRANSCODE_BUDGET: invalid number %q", EnvPrefix, v)
		}
		*budget = b
	}
	if v := env("DOWNLOAD_BUDGET_GB", ""); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("%sDOWNLOAD_BUDGET_GB: invalid number %q", EnvPrefix, v)
		}
		*downloadBudget = n
	}
	if v := env("WORKERS", ""); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("%sWORKERS: invalid count %q", EnvPrefix, v)
		}
		*workers = n
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(os.Stderr)
			fs.PrintDefaults()
		}
		return Config{}, err
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	cfg := Config{Listen: *listen, LogFormat: logging.Format(*logFormat), Dev: *dev,
		FFmpeg: *ffmpeg, FFprobe: *ffprobe, Workers: *workers}
	if cfg.FFmpeg == "" || cfg.FFprobe == "" {
		return Config{}, errors.New("ffmpeg and ffprobe must not be empty")
	}
	if cfg.Workers < 0 {
		return Config{}, errors.New("workers must not be negative")
	}
	if cfg.TrustedProxies, err = ParseTrustedProxies(*proxies); err != nil {
		return Config{}, fmt.Errorf("trusted proxies: %w", err)
	}
	if cfg.BrowseRoots, err = ParseBrowseRoots(*browseRoots); err != nil {
		return Config{}, fmt.Errorf("browse roots: %w", err)
	}
	cfg.HW, cfg.TranscodeBudget, cfg.DownloadBudgetGB = *hw, *budget, *downloadBudget
	cfg.HDRTonemap = strings.ToLower(strings.TrimSpace(*hdrTonemap))
	cfg.TVDBAPIKey, cfg.TVDBPIN = strings.TrimSpace(env("TVDB_API_KEY", "")), strings.TrimSpace(env("TVDB_PIN", ""))
	cfg.YouTubeAPIKey = strings.TrimSpace(env("YOUTUBE_API_KEY", ""))
	cfg.TraktClientID = strings.TrimSpace(env("TRAKT_CLIENT_ID", ""))
	cfg.TraktBaseURL = strings.TrimSpace(env("TRAKT_BASE_URL", ""))
	if err := loadNotifyConfig(&cfg, env); err != nil {
		return Config{}, err
	}
	cfg.TrailerAI = strings.ToLower(strings.TrimSpace(*trailerAI))
	if cfg.TrailerAI == "" {
		cfg.TrailerAI = "off"
	}
	if !hwBackends[cfg.HW] {
		return Config{}, fmt.Errorf("hw %q: want auto, nvenc, qsv, vaapi or software", cfg.HW)
	}
	if !hdrTonemaps[cfg.HDRTonemap] {
		return Config{}, fmt.Errorf("hdr-tonemap %q: want auto, gpu or cpu", cfg.HDRTonemap)
	}
	if cfg.TranscodeBudget < 0 || math.IsNaN(cfg.TranscodeBudget) || math.IsInf(cfg.TranscodeBudget, 0) {
		return Config{}, errors.New("transcode budget must be a finite number >= 0")
	}
	if cfg.DownloadBudgetGB < 1 || cfg.DownloadBudgetGB > 100_000 {
		return Config{}, errors.New("download budget must be between 1 and 100000 GB")
	}
	if cfg.Workers == 0 {
		cfg.Workers = max(2, min(4, runtime.NumCPU()))
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(*logLevel)); err != nil {
		return Config{}, fmt.Errorf("log level: %w", err)
	}
	if cfg.LogFormat != logging.FormatJSON && cfg.LogFormat != logging.FormatText {
		return Config{}, fmt.Errorf("log format %q: want json or text", *logFormat)
	}
	if _, _, err := net.SplitHostPort(cfg.Listen); err != nil {
		return Config{}, fmt.Errorf("listen address: %w", err)
	}
	if cfg.ConfigDir, err = absDir("config dir", *configDir); err != nil {
		return Config{}, err
	}
	if cfg.CacheDir, err = absDir("cache dir", *cacheDir); err != nil {
		return Config{}, err
	}
	if cfg.ConfigDir == cfg.CacheDir {
		return Config{}, errors.New("config dir and cache dir must differ")
	}
	return cfg, nil
}

// LogValue groups the effective configuration for the startup log line.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("listen", c.Listen),
		slog.String("config_dir", c.ConfigDir),
		slog.String("cache_dir", c.CacheDir),
		slog.String("log_level", c.LogLevel.String()),
		slog.String("log_format", string(c.LogFormat)),
		slog.Bool("dev", c.Dev),
		slog.String("ffmpeg", c.FFmpeg),
		slog.String("ffprobe", c.FFprobe),
		slog.Int("workers", c.Workers),
		slog.String("hw", c.HW),
		slog.String("hdr_tonemap", c.HDRTonemap),
		slog.Any("trusted_proxies", c.TrustedProxies),
		slog.Any("browse_roots", c.BrowseRoots),
		slog.Float64("transcode_budget", c.TranscodeBudget),
		slog.Int("download_budget_gb", c.DownloadBudgetGB),
		slog.Bool("tvdb_configured", c.TVDBAPIKey != ""),
		slog.Bool("youtube_configured", c.YouTubeAPIKey != ""),
		slog.Bool("trakt_configured", c.TraktClientID != ""),
		slog.String("trailer_ai", c.TrailerAI),
		slog.Bool("smtp_configured", c.SMTPHost != ""),
		slog.String("smtp_tls", c.SMTPTLS),
		slog.Bool("telegram_configured", c.TelegramBotToken != ""),
		slog.String("public_url", c.PublicURL),
		slog.Bool("update_manifest_default", c.ManifestURL() == DefaultUpdateManifestURL),
	)
}

// loadNotifyConfig reads the e-mail server, the Telegram bot, the update manifest and the public URL
// (M8) into cfg and validates them.
func loadNotifyConfig(cfg *Config, env func(name, def string) string) error {
	cfg.SMTPHost = strings.TrimSpace(env("SMTP_HOST", ""))
	cfg.SMTPUser = strings.TrimSpace(env("SMTP_USER", ""))
	cfg.SMTPPassword = env("SMTP_PASSWORD", "")
	cfg.SMTPFrom = strings.TrimSpace(env("SMTP_FROM", ""))
	cfg.SMTPTLS = strings.ToLower(strings.TrimSpace(env("SMTP_TLS", email.TLSStartTLS)))
	defaultPort, ok := email.DefaultPort(cfg.SMTPTLS)
	if !ok {
		return fmt.Errorf("%sSMTP_TLS %q: want starttls, tls or none", EnvPrefix, cfg.SMTPTLS)
	}
	if v := strings.TrimSpace(env("SMTP_PORT", "")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("%sSMTP_PORT: invalid port %q", EnvPrefix, v)
		}
		cfg.SMTPPort = n
	} else if cfg.SMTPHost != "" {
		cfg.SMTPPort = defaultPort
	}
	cfg.TelegramBotToken = strings.TrimSpace(env("TELEGRAM_BOT_TOKEN", ""))
	cfg.TelegramBaseURL = strings.TrimSpace(env("TELEGRAM_BASE_URL", ""))
	cfg.UpdateManifestURL = strings.TrimSpace(env("UPDATE_MANIFEST_URL", DefaultUpdateManifestURL))
	if cfg.UpdateManifestURL == "" {
		cfg.UpdateManifestURL = DefaultUpdateManifestURL
	}
	if !httpURL(cfg.UpdateManifestURL) {
		return fmt.Errorf("%sUPDATE_MANIFEST_URL: want an http or https URL", EnvPrefix)
	}
	cfg.PublicURL = strings.TrimRight(strings.TrimSpace(env("PUBLIC_URL", "")), "/")
	if cfg.PublicURL != "" {
		u, err := url.Parse(cfg.PublicURL)
		if err != nil || !httpURL(cfg.PublicURL) || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return fmt.Errorf("%sPUBLIC_URL: want an http or https URL without query, fragment or credentials", EnvPrefix)
		}
	}
	return nil
}

// httpURL reports whether s is an absolute http or https URL with a host.
func httpURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// privateNetworks are the networks of the value "private": RFC 1918, loopback, IPv6 loopback and
// unique local addresses.
var privateNetworks = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "::1/128", "fc00::/7"}

// TrustedProxyWarning reports why the configured trusted proxies are a risk, or "" when they are
// not one. SLOUCHCAT_TRUSTED_PROXIES=private trusts every sender out of a whole private network,
// and in a household the clients live in those networks themselves: a device in the same network
// can then set X-Forwarded-For and hand the server any client address it likes. The limit on login
// attempts is keyed on that address, so a fresh value per attempt empties it (security review of
// 23 October 2026, M-1). That is only safe while nothing but the reverse proxy can reach this
// port, which the server cannot see from the inside -- hence a warning and not a refusal. Naming
// the proxy's own network instead (for example 172.18.0.0/16, `docker network inspect` prints it)
// ends both the risk and this warning.
func (c Config) TrustedProxyWarning() string {
	for _, p := range c.TrustedProxies {
		for _, wide := range privateNetworks {
			if w, err := netip.ParsePrefix(wide); err == nil && p == w.Masked() {
				return "trusted_proxies covers the whole private network " + p.String() +
					": every device in it can set X-Forwarded-For and so pick its own client address," +
					" which empties the limit on login attempts. Safe only while nothing but the reverse" +
					" proxy reaches this port (deploy/compose.https-only.yaml). Name the proxy's own" +
					" network as a CIDR instead."
			}
		}
	}
	return ""
}

// BrowseRootWarning reports why the configured browsable roots are a risk, or "" when they are
// not one. Two cases are a risk. An empty list switches the rule off that a new library folder
// must lie below a browsable root, so a library may be created anywhere the server can read. And
// the root "/" turns the folder browser into a file manager for the whole container. Both end at
// /config, where the database, the backups and secret.key live. The server cannot tell a
// deliberate choice from a slip, so it warns and goes on, as it does for
// SLOUCHCAT_TRUSTED_PROXIES.
func (c Config) BrowseRootWarning() string {
	if len(c.BrowseRoots) == 0 {
		return "browse_roots is empty: the folder browser has nothing to list, and a library may" +
			" be created on any directory the server can read, /config with the database, the" +
			" backups and secret.key included. Name the media mounts instead, for example /media."
	}
	for _, root := range c.BrowseRoots {
		if root == "/" {
			return "browse_roots contains \"/\": the folder browser then lists the whole container," +
				" including /config with the database, the backups and secret.key, and a library may be" +
				" created there. Name the media mounts instead, for example /media."
		}
	}
	return ""
}

// ParseBrowseRoots parses SLOUCHCAT_BROWSE_ROOTS: a comma-separated list of absolute directories,
// cleaned and without duplicates; empty means no root at all, which switches the folder browser
// off and lets every valid path through when a library is created (idea 02, E14). Whether a root
// exists is not checked here: a missing one is logged and skipped at start-up, so a typo does not
// keep the server from coming up.
func ParseBrowseRoots(value string) ([]string, error) {
	out := []string{}
	for item := range strings.SplitSeq(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !filepath.IsAbs(item) {
			return nil, fmt.Errorf("%q is not an absolute path", item)
		}
		if clean := filepath.Clean(item); !slices.Contains(out, clean) {
			out = append(out, clean)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// ParseTrustedProxies parses SLOUCHCAT_TRUSTED_PROXIES: empty = no proxy is trusted, "private" =
// privateNetworks, otherwise a comma-separated list of CIDRs or single addresses.
func ParseTrustedProxies(value string) ([]netip.Prefix, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	items := strings.Split(value, ",")
	if strings.EqualFold(value, "private") {
		items = privateNetworks
	}
	out := make([]netip.Prefix, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if p, err := netip.ParsePrefix(item); err == nil {
			out = append(out, p.Masked())
			continue
		}
		ip, err := netip.ParseAddr(item)
		if err != nil {
			return nil, fmt.Errorf("%q is neither a CIDR nor an address", item)
		}
		out = append(out, netip.PrefixFrom(ip.Unmap(), ip.Unmap().BitLen()))
	}
	return out, nil
}

func absDir(name, dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("%s must not be empty", name)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return abs, nil
}

func parseBool(s string) (bool, error) {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("invalid boolean %q", s)
	}
	return b, nil
}
