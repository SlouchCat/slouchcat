package browse

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// ClassifyRoots turns the configured directories (SLOUCHCAT_BROWSE_ROOTS) into the roots the
// dialog starts from. It runs once at start-up and never in a request: the wizard's root list
// answers before any account exists, and it must not be the one unauthenticated operation that
// reaches the file system per call (E5). A root that is missing or no directory is logged and
// left out, so a typo in the configuration does not keep the server from coming up.
func ClassifyRoots(dirs []string, log *slog.Logger) []model.BrowseRoot {
	out := make([]model.BrowseRoot, 0, len(dirs))
	for i, dir := range dirs {
		fi, err := os.Stat(dir)
		switch {
		case err != nil:
			log.Warn("browse root skipped: cannot be read", "path", dir, "err", err)
			continue
		case !fi.IsDir():
			log.Warn("browse root skipped: not a directory", "path", dir)
			continue
		}
		root := model.BrowseRoot{
			ID: "c" + strconv.Itoa(i), Path: dir, Browsable: true, Source: model.BrowseRootConfigured,
		}
		fsType, err := paths.DetectFS(dir)
		if err != nil {
			// The directory is there; only its class is unknown. The dialog works without it.
			log.Warn("browse root: file system not detected", "path", dir, "err", err)
		}
		root.FSType = string(fsType)
		_, root.Warning = library.WatchMode(string(fsType))
		out = append(out, root)
	}
	return out
}

// ToRoots turns the roots of the API into the roots List takes.
func ToRoots(roots []model.BrowseRoot) []Root {
	out := make([]Root, 0, len(roots))
	for _, r := range roots {
		out = append(out, Root{ID: r.ID, Dir: r.Path, Browsable: r.Browsable,
			LibraryID: r.LibraryID, LibraryName: r.LibraryName})
	}
	return out
}
