package admin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// fakeSystem builds /dev, /sys/class/drm and the inotify limit in a temporary directory.
func fakeSystem(t *testing.T, nvidia, dri bool, vendor, maxWatches string) SystemSource {
	t.Helper()
	root := t.TempDir()
	src := SystemSource{
		DevDir: filepath.Join(root, "dev"), DRMDir: filepath.Join(root, "sys", "class", "drm"),
		InotifyFile: filepath.Join(root, "max_user_watches"),
	}
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(src.DevDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if nvidia {
		write(filepath.Join(src.DevDir, "nvidia0"), "")
	}
	if dri {
		write(filepath.Join(src.DevDir, "dri", "renderD128"), "")
		if vendor != "" {
			write(filepath.Join(src.DRMDir, "renderD128", "device", "vendor"), vendor+"\n")
		}
	}
	if maxWatches != "" {
		write(src.InotifyFile, maxWatches+"\n")
	}
	return src
}

// TestSetupSystemInfo (design section 5, A-1): the detected backend and the visible devices name
// the Compose override file the wizard shows.
func TestSetupSystemInfo(t *testing.T) {
	caps := func(b transcode.Backend, tonemapper string) transcode.Capabilities {
		return transcode.Capabilities{Backend: b, HardwareEncode: b != transcode.BackendSoftware, Tonemapper: tonemapper,
			HWTonemap: tonemapper != transcode.TonemapCPU, Version: "ffmpeg version 7.1"}
	}
	for name, tc := range map[string]struct {
		caps           transcode.Capabilities
		nvidia, dri    bool
		vendor, want   string
		wantTonemapper string
	}{
		"nvenc":                     {caps: caps(transcode.BackendNVENC, transcode.TonemapCUDA), nvidia: true, want: "nvidia", wantTonemapper: "cuda"},
		"qsv":                       {caps: caps(transcode.BackendQSV, transcode.TonemapCPU), dri: true, vendor: "0x8086", want: "intel", wantTonemapper: "cpu"},
		"vaapi on intel":            {caps: caps(transcode.BackendVAAPI, "vaapi"), dri: true, vendor: "0x8086", want: "intel", wantTonemapper: "vaapi"},
		"vaapi on amd":              {caps: caps(transcode.BackendVAAPI, "vaapi"), dri: true, vendor: "0x1002", want: "amd", wantTonemapper: "vaapi"},
		"software, nvidia visible":  {caps: caps(transcode.BackendSoftware, transcode.TonemapCPU), nvidia: true, want: "nvidia", wantTonemapper: "cpu"},
		"software, render node":     {caps: caps(transcode.BackendSoftware, transcode.TonemapCPU), dri: true, want: "intel", wantTonemapper: "cpu"},
		"software, amd render node": {caps: caps(transcode.BackendSoftware, transcode.TonemapCPU), dri: true, vendor: "0x1002", want: "amd", wantTonemapper: "cpu"},
		"none":                      {caps: caps(transcode.BackendSoftware, transcode.TonemapCPU), want: "", wantTonemapper: "cpu"},
		"detection failed":          {caps: transcode.Capabilities{}, want: "", wantTonemapper: "cpu"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			src := fakeSystem(t, tc.nvidia, tc.dri, tc.vendor, "524288")
			src.Caps, src.Version, src.StartedAt = tc.caps, "0.9.0+abc1234", time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
			src.ConfigDir, src.CacheDir = t.TempDir(), t.TempDir()
			info, err := f.svc.WithSystem(src).SystemInfo(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			g := info.GPU
			if g.ComposeOverride != tc.want || g.DeviceNvidia != tc.nvidia || g.DeviceDRI != tc.dri || g.Tonemapper != tc.wantTonemapper {
				t.Errorf("gpu = %+v, want override %q, nvidia %v, dri %v, tonemapper %s", g, tc.want, tc.nvidia, tc.dri, tc.wantTonemapper)
			}
			wantBackend := string(tc.caps.Backend)
			if wantBackend == "" {
				wantBackend = "software"
			}
			if g.Backend != wantBackend || g.HardwareEncode != tc.caps.HardwareEncode || g.FFmpegVersion != tc.caps.Version {
				t.Errorf("gpu = %+v, want backend %s", g, wantBackend)
			}
			if info.Version != "0.9.0+abc1234" || !info.StartedAt.Equal(src.StartedAt) || info.ConfigFS == "" || info.CacheFS == "" {
				t.Errorf("info = %+v", info)
			}
		})
	}
}

// The inotify limit and its use: a low limit or more than 80 % used warns; an unreadable limit
// reports 0 without a warning.
func TestSystemInfoInotify(t *testing.T) {
	for name, tc := range map[string]struct {
		limit   string
		watched int64
		wantMax int64
		pct     float64
		warn    bool
	}{
		"plenty":     {limit: "524288", watched: 1000, wantMax: 524288, pct: 1000 * 100.0 / 524288},
		"low limit":  {limit: "8192", watched: 10, wantMax: 8192, pct: 10 * 100.0 / 8192, warn: true},
		"nearly all": {limit: "100000", watched: 90000, wantMax: 100000, pct: 90, warn: true},
		"unreadable": {limit: "", watched: 5},
		"garbage":    {limit: "lots", watched: 5},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			src := fakeSystem(t, false, false, "", tc.limit)
			src.WatchedDirs = func() int64 { return tc.watched }
			info, err := f.svc.WithSystem(src).SystemInfo(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			in := info.Inotify
			if in.MaxUserWatches != tc.wantMax || in.WatchedDirs != tc.watched || in.Warn != tc.warn || in.UsagePercent != tc.pct {
				t.Errorf("inotify = %+v, want max %d, watched %d, %.3f %%, warn %v", in, tc.wantMax, tc.watched, tc.pct, tc.warn)
			}
		})
	}
	// Without a system source (the wizard before main wired it, tests): defaults, no error.
	info, err := newFixture(t).svc.SystemInfo(context.Background())
	if err != nil || info.Version != "dev" || info.GPU.Backend != "software" || info.ConfigFS != "unknown" {
		t.Errorf("info without source = %+v, %v", info, err)
	}
}
