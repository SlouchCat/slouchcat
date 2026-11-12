package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The folder check of a podcast folder (concept 19 part A, 5.4; ADR 0029, decision 2). The server
// writes stored episodes only into a folder that passes it: the page «Podcasts» shows its result,
// a setting «Speichern» is refused without it (root_not_writable), and every download runs it
// first. The check
//
//  1. compares the file system with the one the folder had when it was added (library_roots.fs_type)
//     and wants at least one folder the database knows on disk: an NAS that is not mounted leaves
//     an empty local directory, and the server would write onto the drive of its database. A file
//     system of unknown type proves nothing by its type, so such a root needs a known folder;
//  2. creates .slouchcat-schreibtest-<ulid> (a name of its own, so checks that run at once do not
//     disturb each other), sets a hard link on it (the download enters its file with one, 6.2
//     step 6) and removes both;
//  3. wants the test file readable for everyone after the mode of 6.4 (0664).
//
// The check writes only its own dot files, which the scanner, the watcher and the poller pass over
// (Ignored), and removes no other; the sweep removes test files a crash left after seven days. The
// folder of a podcast below the root is judged by its rights (podcastFolderProblem), and the
// download probes it before it loads.

// Names and modes of what the server writes into a podcast folder (6.4).
const (
	podcastWriteTest = ".slouchcat-schreibtest"
	podcastFileMode  = 0o664
)

// PodcastFolderOf checks the folder for new subscriptions of a podcast library (its
// podcast_root_id, else its first podcast folder): Library.podcastFolder of getLibrary.
// model.ErrNotFound for an unknown library or one of another kind, model.ErrRootRequired for a
// library without a podcast folder.
func (s *Service) PodcastFolderOf(ctx context.Context, libraryID int64) (model.PodcastFolder, error) {
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && lib.Kind != model.LibraryPodcasts) {
		return model.PodcastFolder{}, model.ErrNotFound
	}
	if err != nil {
		return model.PodcastFolder{}, fmt.Errorf("get library: %w", err)
	}
	root, err := s.podcastTargetRoot(ctx, lib)
	if err != nil {
		return model.PodcastFolder{}, err
	}
	return s.CheckPodcastFolder(ctx, root), nil
}

// podcastTargetRoot is the folder for new subscriptions of a podcast library: podcast_root_id
// when it still names a podcast folder of the library, else the podcast folder with the lowest
// id; model.ErrRootRequired without one.
func (s *Service) podcastTargetRoot(ctx context.Context, lib appq.Library) (appq.LibraryRoot, error) {
	roots, err := appq.New(s.db.Read).ListLibraryRoots(ctx, lib.ID)
	if err != nil {
		return appq.LibraryRoot{}, fmt.Errorf("list roots of library %d: %w", lib.ID, err)
	}
	var first *appq.LibraryRoot
	for i, r := range roots {
		if r.Content != model.RootPodcast {
			continue
		}
		if lib.PodcastRootID.Valid && r.ID == lib.PodcastRootID.Int64 {
			return r, nil
		}
		if first == nil || r.ID < first.ID {
			first = &roots[i]
		}
	}
	if first == nil {
		return appq.LibraryRoot{}, model.ErrRootRequired
	}
	return *first, nil
}

// CheckPodcastFolder runs the folder check on a library root (5.4) and returns its result; a root
// that holds no podcasts is never writable for the server (read_only).
func (s *Service) CheckPodcastFolder(ctx context.Context, root appq.LibraryRoot) model.PodcastFolder {
	out := model.PodcastFolder{RootID: root.ID, Path: root.Path}
	if root.Content != model.RootPodcast {
		out.Problem = model.PodcastFolderReadOnly
		return out
	}
	pr, err := paths.NewRoot(root.Path)
	if err != nil {
		out.Problem = model.PodcastFolderNotMounted
		return out
	}
	fsType, err := s.fsTypeAt(pr)
	if err != nil || string(fsType) != root.FsType {
		out.Problem = model.PodcastFolderNotMounted
		return out
	}
	r, err := paths.Open(pr)
	if err != nil {
		out.Problem = model.PodcastFolderNotMounted
		return out
	}
	defer r.Close()
	if mounted, err := s.knownPodcastFolderThere(ctx, r, root.ID, fsType == paths.FSUnknown); err != nil || !mounted {
		if err != nil {
			s.log.WarnContext(ctx, "known podcast folders", "root_id", root.ID, "err", err)
		}
		out.Problem = model.PodcastFolderNotMounted
		return out
	}
	if _, free, err := s.diskSpace(root.Path); err == nil {
		out.FreeBytes = free
	}
	if out.Problem = s.podcastWriteTest(r); out.Problem == "" {
		out.Writable = true
	}
	return out
}

// fsTypeAt is paths.FSTypeAt of the root itself (the test seam podcastFSType replaces it).
func (s *Service) fsTypeAt(pr paths.Root) (paths.FSType, error) {
	if s.podcastFSType != nil {
		return s.podcastFSType(pr)
	}
	return paths.FSTypeAt(pr, ".")
}

// knownPodcastFolderThere reports whether at least one folder the database knows in the root is
// on disk; a root without any known folder (a new, empty podcast folder) passes, unless its file
// system is of unknown type (then nothing shows that it is mounted).
func (s *Service) knownPodcastFolderThere(ctx context.Context, r *os.Root, rootID int64, unknownFS bool) (bool, error) {
	known, err := systemq.New(s.db.Read).PodcastKnownFolders(ctx, rootID)
	if err != nil {
		return false, err
	}
	if len(known) == 0 {
		return !unknownFS, nil
	}
	for _, f := range known {
		if fi, err := r.Lstat(f); err == nil && fi.IsDir() {
			return true, nil
		}
	}
	return false, nil
}

// podcastWriteTest writes a test file of its own name with a hard link in the root of r and
// removes both; it returns the problem found, "" when the folder takes stored episodes.
func (s *Service) podcastWriteTest(r *os.Root) string {
	name := podcastWriteTest + "-" + s.newID()
	link := name + ".link"
	f, err := r.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, podcastFileMode)
	if err != nil {
		return model.PodcastFolderReadOnly
	}
	_ = f.Close()
	defer func() { _ = r.Remove(name) }() //nolint:forbidigo // the own probe files of the podcast folder check, never a file of a title (concept 19 F-4, ADR 0032)
	if err := r.Chmod(name, podcastFileMode); err != nil {
		return model.PodcastFolderPermissions
	}
	if err := s.linkFile(r, name, link); err != nil {
		return model.PodcastFolderNoHardLinks
	}
	_ = r.Remove(link) //nolint:forbidigo // the own probe files of the podcast folder check, never a file of a title (concept 19 F-4, ADR 0032)
	if fi, err := r.Stat(name); err != nil || fi.Mode().Perm()&0o444 != 0o444 {
		return model.PodcastFolderPermissions
	}
	return ""
}

// podcastFolderProblem judges the folder of a podcast below a root that passed the check, without
// writing: not_mounted when it is not there as a directory, permissions when its owner, group and
// mode do not let this process create files (mayWriteDir); "" otherwise.
func podcastFolderProblem(r *os.Root, folder string) string {
	fi, err := r.Lstat(folder)
	switch {
	case err != nil || !fi.IsDir():
		return model.PodcastFolderNotMounted
	case !mayWriteDir(fi):
		return model.PodcastFolderPermissions
	}
	return ""
}

// podcastWriteProbe creates and removes the own file name in a podcast folder (O_EXCL through the
// root; an own leftover of that name goes first): the download's test that it may write there
// before it loads (5.4), where the folder check only tried the root.
func podcastWriteProbe(r *os.Root, name string) bool {
	if err := r.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) { //nolint:forbidigo // the own probe files of the podcast folder check, never a file of a title (concept 19 F-4, ADR 0032)
		return false
	}
	f, err := r.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, podcastFileMode)
	if err != nil {
		return false
	}
	_ = f.Close()
	return r.Remove(name) == nil //nolint:forbidigo // the own probe files of the podcast folder check, never a file of a title (concept 19 F-4, ADR 0032)
}

// linkFile sets a hard link newname on oldname inside r (the test seam podcastLink replaces it).
func (s *Service) linkFile(r *os.Root, oldname, newname string) error {
	if s.podcastLink != nil {
		return s.podcastLink(r, oldname, newname)
	}
	return r.Link(oldname, newname)
}
