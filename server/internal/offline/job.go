package offline

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// The job download.prepare (M5 design 3.3): budget wait, plan, admission, segment-wise append to
// media.mp4 and index.jsonl with resume after a crash, pause on preemption, WebVTT sidecars, ready.

// errDeleted stops a preparation whose download was deleted meanwhile.
var errDeleted = errors.New("offline: download deleted during its preparation")

// RegisterJobs registers download.prepare on w. The server runs a worker of its own for it
// (download-1), so long encodes never hold up scans and probes.
func (s *Service) RegisterJobs(w *jobs.Worker) {
	w.Handle(JobPrepare, s.handlePrepare)
}

func (s *Service) handlePrepare(ctx context.Context, j *jobs.Job) error {
	var p preparePayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("decode %s payload: %w", JobPrepare, err)
	}
	if !downloadID.MatchString(p.DownloadID) {
		return fmt.Errorf("%s payload: invalid download id %q", JobPrepare, p.DownloadID)
	}
	return s.prepare(ctx, j, p.DownloadID)
}

// prepare runs one attempt of the preparation of a download.
func (s *Service) prepare(ctx context.Context, j *jobs.Job, id string) error {
	row, err := s.write.GetDownload(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		s.removeFiles(ctx, id)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get download: %w", err)
	}
	switch row.State {
	case model.DownloadReady, model.DownloadFailed, model.DownloadEvicted:
		return nil
	}
	visible, err := s.visibleToRequester(ctx, row)
	if err != nil {
		return err
	}
	if !visible {
		s.log.InfoContext(ctx, "download not visible to its profile", "download_id", id, "item_id", row.ItemID)
		s.removeFiles(ctx, id)
		return s.setState(ctx, id, model.DownloadFailed, model.DownloadErrNotVisible)
	}
	if s.budget > 0 {
		used, err := s.write.SumServerCopies(ctx, id)
		if err != nil {
			return fmt.Errorf("sum server copies: %w", err)
		}
		// A download larger than the whole budget still runs when nothing else is stored.
		if used > 0 && used+row.EstimatedBytes > s.budget {
			s.log.InfoContext(ctx, "download budget full, waiting", "download_id", id, "used_bytes", used,
				"estimated_bytes", row.EstimatedBytes, "budget_bytes", s.budget)
			if err := s.setState(ctx, id, model.DownloadQueued, model.DownloadErrBudgetFull); err != nil {
				return err
			}
			return s.enqueue(ctx, id, nextKey(id, j.IdempotencyKey), budgetWait)
		}
	}
	return s.encode(ctx, j, row)
}

// visibleToRequester reports whether the requesting profile still sees the title; a download whose
// profile was deleted needs any profile of the account that sees it.
func (s *Service) visibleToRequester(ctx context.Context, row systemq.Download) (bool, error) {
	if row.ProfileID.Valid {
		err := s.vis.Visible(ctx, visibility.Profile(model.Profile{ID: row.ProfileID.Int64}), row.ItemID)
		if errors.Is(err, model.ErrNotFound) {
			return false, nil
		}
		return err == nil, err
	}
	allowed, err := s.vis.AllowedProfiles(ctx, visibility.System("offline: download without profile"), row.AccountID, []string{row.ItemID})
	if err != nil {
		return false, fmt.Errorf("allowed profiles: %w", err)
	}
	return len(allowed[row.ItemID]) > 0, nil
}

// setState sets the state and error code (empty = none) of a download.
func (s *Service) setState(ctx context.Context, id, state, code string) error {
	_, err := s.write.SetDownloadState(context.WithoutCancel(ctx), systemq.SetDownloadStateParams{
		State: state, Error: sql.NullString{String: code, Valid: code != ""}, Now: ms(s.now()), ID: id,
	})
	if err != nil {
		return fmt.Errorf("set download state %s: %w", state, err)
	}
	return nil
}

// planned is the source and the plan of a preparation.
type planned struct {
	src    transcode.Source
	plan   transcode.Plan
	choice choice
	root   string // library root of the source (sidecar subtitles)
	fileID string
}

// planFor resolves the source through package paths and plans the download; a remux without
// keyframe index computes it first (and stores it), else the video is encoded.
func (s *Service) planFor(ctx context.Context, row systemq.Download) (planned, error) {
	m, err := s.write.GetDownloadSource(ctx, row.MediaFileID)
	if err != nil {
		return planned{}, fmt.Errorf("get download source: %w", err)
	}
	probe := parseProbe(m.Probe.String)
	if probe == nil {
		return planned{}, fmt.Errorf("media file %s is not probed yet", m.ID)
	}
	root, err := paths.NewRoot(m.RootPath)
	if err != nil {
		return planned{}, err
	}
	abs, err := paths.Resolve(root, m.RelPath)
	if err != nil {
		return planned{}, fmt.Errorf("resolve media file %s: %w", m.ID, err)
	}
	file := model.MediaFile{
		ID: m.ID, VideoCodec: m.VideoCodec.String, AudioCodec: m.AudioCodec.String, Width: int(m.Width.Int64),
		Height: int(m.Height.Int64), DurationMs: m.DurationMs.Int64, SizeBytes: m.Size,
	}
	prof, ok := model.DownloadProfileFor(row.Quality)
	if !ok {
		return planned{}, fmt.Errorf("download %s has unknown quality %q", row.ID, row.Quality)
	}
	c := decide(file, probe, prof, int(row.AudioStream))
	src := transcode.Source{Path: abs, Probe: *probe}
	if c.Target.Mode == transcode.ModeRemux {
		if src.Keyframes = s.keyframes(ctx, m.ID, abs, m.Keyframes); len(src.Keyframes) == 0 {
			if ctx.Err() != nil {
				return planned{}, ctx.Err()
			}
			c = decideWith(file, probe, prof, int(row.AudioStream), false)
		}
	}
	plan, err := transcode.NewPlan(src, c.Target)
	if err != nil {
		return planned{}, fmt.Errorf("plan download %s: %w", row.ID, err)
	}
	return planned{src: src, plan: plan, choice: c, root: m.RootPath, fileID: m.ID}, nil
}

// keyframes returns the keyframe index of a media file for a remux: the stored one, else computed
// by the encoder and stored for playback too; nil when neither works.
func (s *Service) keyframes(ctx context.Context, fileID, abs string, blob []byte) []time.Duration {
	if len(blob) > 0 {
		if kf, err := library.DecodeKeyframes(blob); err == nil && len(kf) > 0 {
			return kf
		}
	}
	kf, err := s.enc.Keyframes(ctx, abs)
	if err != nil || len(kf) == 0 {
		s.log.InfoContext(ctx, "keyframe index unavailable, encoding the download", "media_file_id", fileID, "err", err)
		return nil
	}
	if err := library.StoreKeyframes(ctx, s.db.Write, fileID, kf, s.now()); err != nil {
		s.log.WarnContext(ctx, "store keyframe index", "media_file_id", fileID, "err", err)
	}
	return kf
}

// copyFiles are the open media file and index of a preparation with the readable index.
type copyFiles struct {
	media, index *os.File
	ix           index
	hasHeader    bool
}

func (c *copyFiles) Close() {
	_ = c.media.Close()
	_ = c.index.Close()
}

// openCopy opens media.mp4 and index.jsonl of a download for appending. It keeps the readable
// prefix of the index and cuts both files to it (a crash in the middle of a write), so the
// preparation continues after the last complete segment.
func openCopy(root *os.Root, id string) (*copyFiles, error) {
	if err := root.MkdirAll(id, 0o750); err != nil {
		return nil, err
	}
	media, err := root.OpenFile(id+"/"+mediaFile, os.O_RDWR|os.O_CREATE, 0o640)
	if err != nil {
		return nil, err
	}
	idx, err := root.OpenFile(id+"/"+indexFile, os.O_RDWR|os.O_CREATE, 0o640)
	if err != nil {
		_ = media.Close()
		return nil, err
	}
	c := &copyFiles{media: media, index: idx}
	info, err := media.Stat()
	if err != nil {
		c.Close()
		return nil, err
	}
	data, err := io.ReadAll(idx)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.ix, c.hasHeader = parseIndex(data, info.Size())
	if !c.hasHeader {
		c.ix = index{}
	}
	if err := media.Truncate(c.mediaEnd()); err != nil {
		c.Close()
		return nil, err
	}
	if err := idx.Truncate(c.ix.bytes); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// mediaEnd is the length of the media file the index covers.
func (c *copyFiles) mediaEnd() int64 {
	if !c.hasHeader {
		return 0
	}
	return c.ix.end()
}

// writeInit writes the init segment and the index header of a fresh copy.
func (c *copyFiles) writeInit(init []byte, h indexHeader) error {
	if len(init) == 0 {
		return errors.New("empty init segment")
	}
	if _, err := c.media.WriteAt(init, 0); err != nil {
		return err
	}
	if err := c.media.Sync(); err != nil {
		return err
	}
	h.Init = int64(len(init))
	line, err := json.Marshal(h)
	if err != nil {
		return err
	}
	if _, err := c.index.WriteAt(append(line, '\n'), 0); err != nil {
		return err
	}
	c.ix = index{header: h, bytes: int64(len(line) + 1)}
	c.hasHeader = true
	return nil
}

// appendSegment appends media segment n and its index line; the media data is synced first, so an
// index entry never names bytes that are not on disk.
func (c *copyFiles) appendSegment(n int, data []byte, start, dur time.Duration) error {
	if !c.hasHeader {
		return errors.New("media segment before the init segment")
	}
	if n != len(c.ix.entries) {
		return fmt.Errorf("segment %d out of order, expected %d", n, len(c.ix.entries))
	}
	if len(data) == 0 {
		return fmt.Errorf("segment %d is empty", n)
	}
	e := indexEntry{N: n, StartMs: start.Milliseconds(), DurationMs: max(dur.Milliseconds(), 1), Offset: c.ix.end(), Length: int64(len(data))}
	if _, err := c.media.WriteAt(data, e.Offset); err != nil {
		return err
	}
	if err := c.media.Sync(); err != nil {
		return err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := c.index.WriteAt(append(line, '\n'), c.ix.bytes); err != nil {
		return err
	}
	c.ix.entries = append(c.ix.entries, e)
	c.ix.bytes += int64(len(line) + 1)
	return nil
}

// encode plans the download, takes admission budget and appends the segments the encoder delivers.
func (s *Service) encode(ctx context.Context, j *jobs.Job, row systemq.Download) error {
	id := row.ID
	p, err := s.planFor(ctx, row)
	if err != nil {
		return s.attemptFailed(ctx, j, row, err)
	}
	root, err := s.root()
	if err != nil {
		return err
	}
	defer root.Close()
	files, err := openCopy(root, id)
	if err != nil {
		return s.attemptFailed(ctx, j, row, fmt.Errorf("open prepared copy: %w", err))
	}
	defer files.Close()
	start := len(files.ix.entries)
	total := len(p.plan.Segments)
	if start > total {
		return s.attemptFailed(ctx, j, row, fmt.Errorf("index has %d segments, the plan %d", start, total))
	}
	videoKbps := p.plan.VideoBitrate / 1000
	if p.choice.Copy {
		videoKbps = p.choice.VideoKbps
	}
	n, err := s.write.StartDownloadPreparation(ctx, systemq.StartDownloadPreparationParams{
		Mode: sql.NullString{String: string(p.plan.Target.Mode), Valid: true}, Width: nullInt(p.plan.Width), Height: nullInt(p.plan.Height),
		VideoBitrate: nullInt(videoKbps), AudioBitrate: nullInt(p.choice.AudioKbps),
		DurationMs:    sql.NullInt64{Int64: p.plan.Duration.Milliseconds(), Valid: true},
		SegmentsTotal: sql.NullInt64{Int64: int64(total), Valid: true}, SegmentsDone: int64(start), Now: ms(s.now()), ID: id,
	})
	if err != nil {
		return fmt.Errorf("start download preparation: %w", err)
	}
	if n == 0 {
		s.removeFiles(ctx, id)
		return nil
	}
	header := indexHeader{Codecs: transcode.CodecsString(p.plan, p.src), Width: p.plan.Width, Height: p.plan.Height}

	var lease Lease
	if s.adm != nil && start < total {
		if lease, err = s.adm.AcquireDownload(ctx, id, s.adm.AdmissionCost(p.plan)); err != nil {
			if ctx.Err() != nil {
				_ = s.setState(ctx, id, model.DownloadQueued, "")
				return ctx.Err()
			}
			return s.attemptFailed(ctx, j, row, fmt.Errorf("acquire download budget: %w", err))
		}
		defer lease.Release()
	}
	work, err := s.copyDir(id)
	if err != nil {
		return s.attemptFailed(ctx, j, row, err)
	}
	s.log.InfoContext(ctx, "download preparation started", "download_id", id, "mode", string(p.plan.Target.Mode),
		"segments", total, "start_segment", start, "cost", p.plan.Cost)

	encCtx, cancel := context.WithCancel(ctx)
	var preempted atomic.Bool
	var watch sync.WaitGroup
	defer watch.Wait()
	defer cancel()
	if lease != nil {
		watch.Go(func() {
			select {
			case <-lease.Preempted():
				preempted.Store(true)
				cancel()
			case <-encCtx.Done():
			}
		})
	}
	onInit := func(init []byte) error {
		if files.hasHeader {
			return nil // resumed: the init segment is already in the file
		}
		return files.writeInit(init, header)
	}
	onSegment := func(n int, data []byte, start, dur time.Duration) error {
		if err := files.appendSegment(n, data, start, dur); err != nil {
			return err
		}
		rows, err := s.write.SetDownloadProgress(ctx, systemq.SetDownloadProgressParams{SegmentsDone: int64(n + 1), Now: ms(s.now()), ID: id})
		if err != nil {
			return fmt.Errorf("record segment %d: %w", n, err)
		}
		if rows == 0 {
			return errDeleted
		}
		return nil
	}
	if start < total {
		err = s.enc.Download(encCtx, s.caps, p.src, p.plan, work, start, onInit, onSegment)
	}
	switch {
	case err == nil:
	case errors.Is(err, errDeleted):
		s.removeFiles(ctx, id)
		return nil
	case preempted.Load():
		s.log.InfoContext(ctx, "download paused for live playback", "download_id", id, "segments_done", len(files.ix.entries))
		if lease != nil {
			lease.Release()
		}
		if err := s.setState(ctx, id, model.DownloadPaused, ""); err != nil {
			return err
		}
		return s.enqueue(ctx, id, nextKey(id, j.IdempotencyKey), preemptWait)
	case ctx.Err() != nil:
		_ = s.setState(ctx, id, model.DownloadQueued, "")
		return ctx.Err()
	default:
		return s.attemptFailed(ctx, j, row, fmt.Errorf("encode download %s: %w", id, err))
	}
	if got := len(files.ix.entries); got != total {
		return s.attemptFailed(ctx, j, row, fmt.Errorf("encoder delivered %d of %d segments", got, total))
	}
	if err := s.sidecars(ctx, root, id, p); err != nil {
		if ctx.Err() != nil {
			_ = s.setState(ctx, id, model.DownloadQueued, "")
			return ctx.Err()
		}
		return s.attemptFailed(ctx, j, row, err)
	}
	size := files.ix.end()
	rows, err := s.write.SetDownloadReady(ctx, systemq.SetDownloadReadyParams{
		SizeBytes: sql.NullInt64{Int64: size, Valid: true}, SegmentsDone: int64(total), Now: ms(s.now()), ID: id,
	})
	if err != nil {
		return fmt.Errorf("set download ready: %w", err)
	}
	if rows == 0 {
		s.removeFiles(ctx, id)
		return nil
	}
	s.log.InfoContext(ctx, "download ready", "download_id", id, "size_bytes", size, "estimated_bytes", row.EstimatedBytes)
	return nil
}

// copyDir is the absolute directory of a download; the encoder works in its subdirectory encode/
// (transcode.Runner.Download), next to media.mp4, index.jsonl and subs/.
func (s *Service) copyDir(id string) (string, error) {
	r, err := paths.NewRoot(s.dir)
	if err != nil {
		return "", err
	}
	return paths.Resolve(r, id)
}

// sidecars converts every text subtitle track of the source (embedded and sidecar files) to a
// WebVTT file subs/<track id>.vtt; bitmap tracks are skipped (not available offline). A track that
// fails to convert is logged and left out.
func (s *Service) sidecars(ctx context.Context, root *os.Root, id string, p planned) error {
	tracks, err := s.write.ListDownloadSubtitleTracks(ctx, p.fileID)
	if err != nil {
		return fmt.Errorf("list subtitle tracks: %w", err)
	}
	if err := root.MkdirAll(id+"/"+subsDir, 0o750); err != nil {
		return fmt.Errorf("create subtitle directory: %w", err)
	}
	for _, t := range tracks {
		if !visibility.TextSubtitle(t.Codec.String) {
			continue
		}
		data, err := s.convert(ctx, p, t)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			s.log.WarnContext(ctx, "download subtitle not converted", "download_id", id, "track_id", t.ID, "err", err)
			continue
		}
		name := id + "/" + subsDir + "/" + strconv.FormatInt(t.ID, 10) + ".vtt"
		if err := root.WriteFile(name+".tmp", data, 0o640); err != nil {
			return fmt.Errorf("write subtitle %d: %w", t.ID, err)
		}
		if err := root.Rename(name+".tmp", name); err != nil {
			return fmt.Errorf("write subtitle %d: %w", t.ID, err)
		}
	}
	return nil
}

// convert runs the WebVTT conversion of one text track; a sidecar file is resolved below the
// library root through package paths.
func (s *Service) convert(ctx context.Context, p planned, t systemq.Track) ([]byte, error) {
	if t.Source == "sidecar" {
		root, err := paths.NewRoot(p.root)
		if err != nil {
			return nil, err
		}
		abs, err := paths.Resolve(root, t.RelPath.String)
		if err != nil {
			return nil, fmt.Errorf("resolve sidecar subtitle: %w", err)
		}
		return s.enc.SubtitlesFile(ctx, abs)
	}
	return s.enc.Subtitles(ctx, p.src.Path, int(t.StreamIndex))
}

// attemptFailed records a failed attempt: the job retries with backoff, the download waits as
// queued; after the last attempt the download is failed with encode_failed.
func (s *Service) attemptFailed(ctx context.Context, j *jobs.Job, row systemq.Download, cause error) error {
	if ctx.Err() != nil {
		_ = s.setState(ctx, row.ID, model.DownloadQueued, "")
		return ctx.Err()
	}
	if j.Attempts >= j.MaxAttempts {
		s.log.WarnContext(ctx, "download preparation failed", "download_id", row.ID, "attempts", j.Attempts, "err", cause)
		if err := s.setState(ctx, row.ID, model.DownloadFailed, model.DownloadErrEncodeFailed); err != nil {
			return errors.Join(cause, err)
		}
		return cause
	}
	if err := s.setState(ctx, row.ID, model.DownloadQueued, ""); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func nullInt(v int) sql.NullInt64 { return sql.NullInt64{Int64: int64(v), Valid: v > 0} }
