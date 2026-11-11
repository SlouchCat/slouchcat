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
//     an empty local directory, and the server would write onto the drive of its database;
//  2. creates .slouchcat-schreibtest, sets a hard link on it (the download enters its file with
//     one, 6.2 step 6) and removes both;
//  3. wants the test file readable for everyone after the mode of 6.4 (0664).
//
// The check writes only its own dot files, which the scanner, the watcher and the poller pass over
// (Ignored).

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
	if fsType, err := paths.FSTypeAt(pr, "."); err != nil || string(fsType) != root.FsType {
		out.Problem = model.PodcastFolderNotMounted
		return out
	}
	r, err := paths.Open(pr)
	if err != nil {
		out.Problem = model.PodcastFolderNotMounted
		return out
	}
	defer r.Close()
	if mounted, err := s.knownPodcastFolderThere(ctx, r, root.ID); err != nil || !mounted {
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

// knownPodcastFolderThere reports whether at least one folder the database knows in the root is
// on disk; a root without any known folder (a new, empty podcast folder) passes.
func (s *Service) knownPodcastFolderThere(ctx context.Context, r *os.Root, rootID int64) (bool, error) {
	known, err := systemq.New(s.db.Read).PodcastKnownFolders(ctx, rootID)
	if err != nil {
		return false, err
	}
	if len(known) == 0 {
		return true, nil
	}
	for _, f := range known {
		if fi, err := r.Lstat(f); err == nil && fi.IsDir() {
			return true, nil
		}
	}
	return false, nil
}

// podcastWriteTest writes the test file with a hard link in the root of r and removes both; it
// returns the problem found, "" when the folder takes stored episodes.
func (s *Service) podcastWriteTest(r *os.Root) string {
	link := podcastWriteTest + ".link"
	// Leftovers of a check that was cut short.
	for _, name := range []string{link, podcastWriteTest} {
		if err := r.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return model.PodcastFolderReadOnly
		}
	}
	f, err := r.OpenFile(podcastWriteTest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, podcastFileMode)
	if err != nil {
		return model.PodcastFolderReadOnly
	}
	_ = f.Close()
	defer func() { _ = r.Remove(podcastWriteTest) }()
	if err := r.Chmod(podcastWriteTest, podcastFileMode); err != nil {
		return model.PodcastFolderPermissions
	}
	if err := s.linkFile(r, podcastWriteTest, link); err != nil {
		return model.PodcastFolderNoHardLinks
	}
	_ = r.Remove(link)
	if fi, err := r.Stat(podcastWriteTest); err != nil || fi.Mode().Perm()&0o444 != 0o444 {
		return model.PodcastFolderPermissions
	}
	return ""
}

// linkFile sets a hard link newname on oldname inside r (the test seam podcastLink replaces it).
func (s *Service) linkFile(r *os.Root, oldname, newname string) error {
	if s.podcastLink != nil {
		return s.podcastLink(r, oldname, newname)
	}
	return r.Link(oldname, newname)
}
