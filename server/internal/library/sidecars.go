package library

import (
	"context"
	"database/sql"
	"fmt"
	"path"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Sidecar subtitle sync (I-5, M4 design section 5). A subtitle that Bazarr places next to a media
// file reaches the tracks table without probing the file again: full scans and path scans list the
// subtitle files of every directory they read (rootScan.subs), and syncSidecars compares them with
// the sidecar rows of the probed media files in those directories, inserting new sidecars and
// deleting the rows of vanished ones. The watcher reports the directory (scan.path), so a new
// .de.srt appears within the watcher latency. Files that are not probed yet get their sidecars
// with the probe (handleProbe).

// sidecarChange is the sidecar difference of one media file.
type sidecarChange struct {
	fileID string
	want   []sidecar
}

// syncSidecars brings the sidecar rows of the root's media files in the directories the scan read
// (scope nil: all of them) in line with the subtitle files on disk.
func (s *Service) syncSidecars(ctx context.Context, rs *rootScan, scope *dirScope) error {
	sq := systemq.New(s.db.Read)
	files, err := sq.ListMediaFilesByRoot(ctx, rs.root.ID) // after the scan's renames and moves
	if err != nil {
		return fmt.Errorf("list media files of root %d: %w", rs.root.ID, err)
	}
	rows, err := sq.ListSidecarTracksByRoot(ctx, rs.root.ID)
	if err != nil {
		return fmt.Errorf("list sidecar tracks of root %d: %w", rs.root.ID, err)
	}
	have := map[string][]string{}
	for _, r := range rows {
		have[r.MediaFileID] = append(have[r.MediaFileID], r.RelPath.String)
	}
	var changes []sidecarChange
	for _, f := range files {
		dir := path.Dir(f.RelPath)
		if f.MissingSince.Valid || !f.ProbedAt.Valid || f.ProbeError.Valid || (scope != nil && !scope.listed[dir]) {
			continue
		}
		want := sidecarsOf(f.RelPath, rs.subs[dir])
		if !sameSidecars(want, have[f.ID]) {
			changes = append(changes, sidecarChange{fileID: f.ID, want: want})
		}
	}
	if len(changes) == 0 {
		return nil
	}
	added, removed, err := s.applySidecars(ctx, changes)
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "sidecar subtitles synchronised", "root_id", rs.root.ID, "files", len(changes), "added", added, "removed", removed)
	return nil
}

// sameSidecars reports whether the rows (rel paths) match the sidecars on disk.
func sameSidecars(want []sidecar, have []string) bool {
	if len(want) != len(have) {
		return false
	}
	set := make(map[string]bool, len(have))
	for _, rel := range have {
		set[rel] = true
	}
	for _, sc := range want {
		if !set[sc.rel] {
			return false
		}
	}
	return true
}

// applySidecars applies the changes in one write transaction. The rows of each file are read
// again inside it, so a probe that replaced the tracks in the meantime is not duplicated.
func (s *Service) applySidecars(ctx context.Context, changes []sidecarChange) (added, removed int, err error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	for _, c := range changes {
		rows, err := q.ListMediaFileSidecarTracks(ctx, c.fileID)
		if err != nil {
			return 0, 0, fmt.Errorf("list sidecar tracks: %w", err)
		}
		wanted := map[string]bool{}
		for _, sc := range c.want {
			wanted[sc.rel] = true
		}
		present := map[string]bool{}
		for _, r := range rows {
			if !wanted[r.RelPath.String] || present[r.RelPath.String] {
				if err := q.DeleteTrack(ctx, r.ID); err != nil {
					return 0, 0, fmt.Errorf("delete sidecar track: %w", err)
				}
				removed++
				continue
			}
			present[r.RelPath.String] = true
		}
		for _, sc := range c.want {
			if present[sc.rel] {
				continue
			}
			title := ""
			if sc.hearingImpaired {
				title = "SDH"
			}
			if err := q.InsertSidecarTrack(ctx, systemq.InsertSidecarTrackParams{
				MediaFileID: c.fileID, Codec: sql.NullString{String: sc.codec, Valid: true},
				Language: sql.NullString{String: sc.lang, Valid: sc.lang != ""}, Title: sql.NullString{String: title, Valid: title != ""},
				RelPath: sql.NullString{String: sc.rel, Valid: true}, IsForced: boolInt(sc.forced),
			}); err != nil {
				return 0, 0, fmt.Errorf("insert sidecar track: %w", err)
			}
			added++
		}
	}
	return added, removed, tx.Commit()
}
