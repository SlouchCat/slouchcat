package admin

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// System state of M8 (A-1, A-3, design 3.3): version, transcode hardware with the Docker Compose
// override that passes the GPU into the container, the inotify watch limit with its use, and the
// file systems of the config and cache directories. Nothing of it is secret, so the wizard shows it
// before an account exists. The device and limit probes read fixed system paths, no user data, so
// they do not go through package paths.

// SystemSource is what main found at start.
type SystemSource struct {
	Version   string
	StartedAt time.Time
	ConfigDir string
	CacheDir  string
	// Caps are the capabilities of transcode.Detect; a zero value means the detection failed
	// (software encoding, CPU tonemapping).
	Caps transcode.Capabilities
	// WatchedDirs counts the directories of the library watchers (library.Service.WatchedDirs);
	// nil counts 0, as in the wizard.
	WatchedDirs func() int64
	// The probed paths; empty means the real ones. Tests point them into a temporary directory.
	DevDir      string // /dev
	DRMDir      string // /sys/class/drm
	InotifyFile string // /proc/sys/fs/inotify/max_user_watches
}

const (
	defaultDevDir      = "/dev"
	defaultDRMDir      = "/sys/class/drm"
	defaultInotifyFile = "/proc/sys/fs/inotify/max_user_watches"

	// inotifyMinWatches is the limit below which the wizard and the dashboard show the sysctl
	// command; inotifyWarnPercent the use above which they do.
	inotifyMinWatches  = 65536
	inotifyWarnPercent = 80

	// PCI vendor ids of the render node (/sys/class/drm/renderD*/device/vendor).
	vendorIntel = "0x8086"
	vendorAMD   = "0x1002"
)

// Compose override files (deploy/compose.<name>.yaml) of GPUInfo.ComposeOverride.
const (
	overrideNvidia = "nvidia"
	overrideIntel  = "intel"
	overrideAMD    = "amd"
)

// WithSystem sets the start state of the server and returns the receiver.
func (s *Service) WithSystem(src SystemSource) *Service {
	s.ops.system = src
	return s
}

// SystemInfo returns version, transcode hardware, inotify limit and file systems; no secrets.
func (s *Service) SystemInfo(context.Context) (model.SystemInfo, error) {
	src := s.ops.system
	return model.SystemInfo{
		Version: s.version(), StartedAt: src.StartedAt, GPU: gpuInfo(src), Inotify: inotifyInfo(src),
		ConfigFS: fsName(src.ConfigDir), CacheFS: fsName(src.CacheDir),
	}, nil
}

// gpuInfo maps the capabilities and looks for the GPU devices in the container.
func gpuInfo(src SystemSource) model.GPUInfo {
	c := src.Caps
	g := model.GPUInfo{
		Backend: string(c.Backend), HardwareEncode: c.HardwareEncode, Tonemapper: c.Tonemapper, HWTonemap: c.HWTonemap,
		FFmpegVersion: c.Version,
	}
	if g.Backend == "" {
		g.Backend = string(transcode.BackendSoftware)
	}
	if g.Tonemapper == "" {
		g.Tonemapper = transcode.TonemapCPU
	}
	dev := orDefault(src.DevDir, defaultDevDir)
	_, err := os.Stat(filepath.Join(dev, "nvidia0"))
	g.DeviceNvidia = err == nil
	nodes, _ := filepath.Glob(filepath.Join(dev, "dri", "renderD*"))
	g.DeviceDRI = len(nodes) > 0
	vendor := ""
	if g.DeviceDRI {
		vendor = renderVendor(orDefault(src.DRMDir, defaultDRMDir), filepath.Base(nodes[0]))
	}
	g.ComposeOverride = composeOverride(g.Backend, g.DeviceNvidia, g.DeviceDRI, vendor)
	return g
}

// composeOverride names the Compose override file that passes the GPU into the container (design
// section 1): from the backend (nvenc → nvidia, qsv → intel, vaapi → intel on an Intel render
// node, else amd); without a hardware backend from the devices (/dev/nvidia0 → nvidia, a render
// node → amd when its vendor is AMD, else intel); "" when no GPU device is visible.
func composeOverride(backend string, nvidia, dri bool, vendor string) string {
	switch transcode.Backend(backend) {
	case transcode.BackendNVENC:
		return overrideNvidia
	case transcode.BackendQSV:
		return overrideIntel
	case transcode.BackendVAAPI:
		if vendor == vendorIntel {
			return overrideIntel
		}
		return overrideAMD
	}
	switch {
	case nvidia:
		return overrideNvidia
	case dri && vendor == vendorAMD:
		return overrideAMD
	case dri:
		return overrideIntel
	}
	return ""
}

// renderVendor reads the PCI vendor id of a render node; "" when unreadable.
func renderVendor(drmDir, node string) string {
	data, err := os.ReadFile(filepath.Join(drmDir, node, "device", "vendor")) //nolint:gosec // G304: fixed sysfs layout below the configured DRM directory
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(string(data)))
}

// inotifyInfo reads the watch limit and relates the watcher's directories to it.
func inotifyInfo(src SystemSource) model.InotifyInfo {
	var info model.InotifyInfo
	if data, err := os.ReadFile(orDefault(src.InotifyFile, defaultInotifyFile)); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil && n > 0 {
			info.MaxUserWatches = n
		}
	}
	if src.WatchedDirs != nil {
		info.WatchedDirs = src.WatchedDirs()
	}
	if info.MaxUserWatches > 0 {
		info.UsagePercent = float64(info.WatchedDirs) * 100 / float64(info.MaxUserWatches)
		info.Warn = info.MaxUserWatches < inotifyMinWatches || info.UsagePercent > inotifyWarnPercent
	}
	return info
}

// fsName is the file system class of dir; "unknown" when undetected.
func fsName(dir string) string {
	if dir == "" {
		return string(paths.FSUnknown)
	}
	fs, err := paths.DetectFS(dir)
	if err != nil {
		return string(paths.FSUnknown)
	}
	return string(fs)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
