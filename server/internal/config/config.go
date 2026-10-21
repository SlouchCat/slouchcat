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
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/logging"
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
	SpikeDir  string         // Directory with the offline spike files; empty disables the endpoint (M1a only).
	Workers   int            // Number of job workers.
	// HW selects the transcode backend: auto (detect), nvenc, qsv, vaapi or software (M1b).
	HW string
	// HDRTonemap selects where HDR → SDR tonemapping runs: auto, gpu or cpu (M4, transcode.TonemapPref*).
	HDRTonemap string
	// TrustedProxies are the networks whose X-Forwarded-For header the server believes (reverse
	// proxy in front); empty ignores the header.
	TrustedProxies []netip.Prefix
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
	// TraktClientID enables the daily Trakt lists (M7, AN-3, I-7); empty = off, the two Trakt rows
	// stay hidden. TraktBaseURL overrides the API base for mocks (development and tests only).
	TraktClientID string
	TraktBaseURL  string
}

// TranscodeDir is the work directory of HLS output: <cache>/transcode, wiped at every start.
func (c Config) TranscodeDir() string { return filepath.Join(c.CacheDir, "transcode") }

// DownloadsDir holds the prepared downloads (M5, AN-20): <cache>/downloads. Unlike the transcode
// directory it survives restarts, so an interrupted preparation continues.
func (c Config) DownloadsDir() string { return filepath.Join(c.CacheDir, "downloads") }

// DefaultDownloadBudgetGB is the download budget without SLOUCHCAT_DOWNLOAD_BUDGET_GB (decision 8).
const DefaultDownloadBudgetGB = 100

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
	spikeDir := fs.String("spike-dir", env("SPIKE_DIR", ""), "directory with the offline spike files, empty = disabled (env SLOUCHCAT_SPIKE_DIR)")
	workers := fs.Int("workers", 0, "number of job workers, 0 = automatic (env SLOUCHCAT_WORKERS)")
	hw := fs.String("hw", env("HW", "auto"), "transcode backend: auto, nvenc, qsv, vaapi or software (env SLOUCHCAT_HW)")
	hdrTonemap := fs.String("hdr-tonemap", env("HDR_TONEMAP", "auto"), "HDR to SDR tonemapping: auto, gpu or cpu (env SLOUCHCAT_HDR_TONEMAP)")
	proxies := fs.String("trusted-proxies", env("TRUSTED_PROXIES", ""),
		"reverse proxies whose X-Forwarded-For is trusted: empty = none, private = RFC 1918, loopback and fc00::/7, or comma-separated CIDRs (env SLOUCHCAT_TRUSTED_PROXIES)")
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
	cfg.HW, cfg.TranscodeBudget, cfg.DownloadBudgetGB = *hw, *budget, *downloadBudget
	cfg.HDRTonemap = strings.ToLower(strings.TrimSpace(*hdrTonemap))
	cfg.TVDBAPIKey, cfg.TVDBPIN = strings.TrimSpace(env("TVDB_API_KEY", "")), strings.TrimSpace(env("TVDB_PIN", ""))
	cfg.YouTubeAPIKey = strings.TrimSpace(env("YOUTUBE_API_KEY", ""))
	cfg.TraktClientID = strings.TrimSpace(env("TRAKT_CLIENT_ID", ""))
	cfg.TraktBaseURL = strings.TrimSpace(env("TRAKT_BASE_URL", ""))
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
	if *spikeDir != "" {
		if cfg.SpikeDir, err = absDir("spike dir", *spikeDir); err != nil {
			return Config{}, err
		}
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
		slog.String("spike_dir", c.SpikeDir),
		slog.Int("workers", c.Workers),
		slog.String("hw", c.HW),
		slog.String("hdr_tonemap", c.HDRTonemap),
		slog.Any("trusted_proxies", c.TrustedProxies),
		slog.Float64("transcode_budget", c.TranscodeBudget),
		slog.Int("download_budget_gb", c.DownloadBudgetGB),
		slog.Bool("tvdb_configured", c.TVDBAPIKey != ""),
		slog.Bool("youtube_configured", c.YouTubeAPIKey != ""),
		slog.Bool("trakt_configured", c.TraktClientID != ""),
		slog.String("trailer_ai", c.TrailerAI),
	)
}

// privateNetworks are the networks of the value "private": RFC 1918, loopback, IPv6 loopback and
// unique local addresses.
var privateNetworks = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "::1/128", "fc00::/7"}

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
