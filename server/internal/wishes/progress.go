package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/arr"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// The download progress from the queue of Radarr and Sonarr (concept 25 step 4.6d, architecture
// 22; N-82 to N-87). The job wishes.progress reads, every minute while wishes are «In Beschaffung»
// and «Download-Fortschritt zeigen» is on, the queue of each service once (all pages), finds the
// title of each wish once (movie?tmdbId, series?tvdbId; the file state again at most every
// 5 minutes) and keeps one row per wish in wish_progress. The pages read only that table
// (ProgressOf, AdminProgressOf) and never call Radarr or Sonarr. The progress is a display only:
// it never changes the state of a wish (N-87). Entries without a wish are not kept; paths are
// never stored.

// JobProgress is the job type of the download progress.
const JobProgress = "wishes.progress"

const (
	// progressEvery is the pace while wishes are in procurement (architecture 13). The scheduler
	// allows no interval below jobs.MinEvery, so the schedule (progressSchedule) only starts the
	// chain and every run queues the next one after progressEvery.
	progressEvery    = time.Minute
	progressSchedule = jobs.MinEvery
	progressPriority = 4
	// progressGap: a run that comes less than this after the last one ends without a call (the
	// schedule and the chain may meet).
	progressGap = 45 * time.Second
	// progressStale: older rows show nothing to viewers (N-85).
	progressStale = 5 * time.Minute
	// titleEvery: how often the file state of a title is read again (architecture 22.4).
	titleEvery       = 5 * time.Minute
	maxAdminMessage  = 500
	settingLastRun   = "arr.progress_run" // the last run, Unix milliseconds
	progressStateKey = JobProgress
)

// Stages and errors of wish_progress.
const (
	StageSearch   = "search"
	StageDownload = "download"
	StageImport   = "import"
	StageStalled  = "stalled"

	ProgressUnreachable = "unreachable"
	ProgressNotFound    = "not_found"
	ProgressIncomplete  = "incomplete"
)

// ProgressEpisodes is «x von y Folgen da» of a series wish.
type ProgressEpisodes struct {
	Done, Total int
}

// WishProgress is what viewers see of the progress (WishProgress of the contract): never a
// message of the service, never the service, never a path (N-83).
type WishProgress struct {
	Stage      string // search, download, import, stalled
	Percent    *int   // download only
	ETASeconds *int   // download only
	Episodes   *ProgressEpisodes
	ReadAt     time.Time
}

// AdminWishProgress is the progress for the administration (AdminWishProgress of the contract).
type AdminWishProgress struct {
	WishProgress        // Stage "" when nothing is known
	Service      string // radarr or sonarr
	Message      string // the service's message, shortened, without paths
	Error        string // unreachable, not_found, incomplete
	Stale        bool   // ReadAt is older than 5 minutes: «Fortschritt nicht lesbar»
}

// ArrProgressStatus is the progress part of ArrServiceStatus.
type ArrProgressStatus struct {
	Enabled    bool       // «Download-Fortschritt zeigen»
	LastReadAt *time.Time // the last read of the queue of the service
	Error      string     // unreachable while a row could not be read
	Tracked    int        // wishes with something to show
}

func (s *Service) registerProgressType(r *jobs.Registry) {
	r.Register(jobs.Type{
		Name: JobProgress, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: progressPriority, Handler: s.handleProgress, Run: s.runProgress, Every: progressSchedule,
	})
}

// runProgress is the schedule: it queues a run only while wishes are in procurement and the
// switch is on; with the switch off it empties the table.
func (s *Service) runProgress(ctx context.Context, _ *jobs.Queue, prio int) ([]int64, error) {
	on, err := s.boolState(ctx, SettingShowProgress, true)
	if err != nil {
		return nil, err
	}
	if !on {
		return nil, s.clearProgress(ctx)
	}
	n, err := s.read.CountProcurementWishes(ctx)
	if err != nil || n == 0 || s.queue == nil {
		return nil, err
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobProgress, Priority: prio, IdempotencyKey: progressStateKey, Payload: []byte(`{}`), MaxAttempts: 1})
	if err != nil {
		return nil, fmt.Errorf("enqueue %s: %w", JobProgress, err)
	}
	return []int64{id}, nil
}

func (s *Service) clearProgress(ctx context.Context) error {
	if err := s.write.DeleteAllWishProgress(ctx); err != nil {
		return fmt.Errorf("clear wish progress: %w", err)
	}
	return nil
}

// handleProgress runs one read. Errors of a service end in the rows, not in the job: the next run
// comes in a minute (architecture 13, no repetition through the queue).
func (s *Service) handleProgress(ctx context.Context, _ *jobs.Job) error {
	on, err := s.boolState(ctx, SettingShowProgress, true)
	if err != nil {
		return err
	}
	if !on {
		return s.clearProgress(ctx)
	}
	if err := s.write.DeleteClosedWishProgress(ctx); err != nil {
		return fmt.Errorf("delete closed wish progress: %w", err)
	}
	wishes, err := s.read.ListProcurementWishes(ctx)
	if err != nil {
		return fmt.Errorf("list wishes in procurement: %w", err)
	}
	if len(wishes) == 0 {
		return nil
	}
	now := s.now()
	if last, err := s.state(ctx, settingLastRun); err != nil {
		return err
	} else if ms, ok := parseMillis(last); ok && now.Sub(time.UnixMilli(ms)) < progressGap && now.Sub(time.UnixMilli(ms)) >= 0 {
		return nil
	}
	if err := s.setState(ctx, s.write, settingLastRun, fmt.Sprint(now.UnixMilli())); err != nil {
		return err
	}
	byService := map[arr.Kind][]wishq.ListProcurementWishesRow{}
	for _, w := range wishes {
		k := arr.Radarr
		if w.Media == mediaShow {
			k = arr.Sonarr
		}
		byService[k] = append(byService[k], w)
	}
	for _, kind := range []arr.Kind{arr.Radarr, arr.Sonarr} {
		if len(byService[kind]) > 0 {
			if err := s.progressService(ctx, kind, byService[kind]); err != nil {
				return err
			}
		}
	}
	s.queueNextProgress(ctx, now)
	return nil
}

// queueNextProgress queues the run a minute later; each run has its own key, so the queue does
// not hand back the running job.
func (s *Service) queueNextProgress(ctx context.Context, now time.Time) {
	if s.queue == nil {
		return
	}
	key := fmt.Sprintf("%s@%d", JobProgress, now.Add(progressEvery).Unix()/60)
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobProgress, Priority: progressPriority, IdempotencyKey: key,
		Payload: []byte(`{}`), Delay: progressEvery, MaxAttempts: 1}); err != nil {
		s.log.WarnContext(ctx, "next progress run not queued", "err", err)
	}
}

func parseMillis(v string) (int64, bool) {
	var ms int64
	if _, err := fmt.Sscan(v, &ms); err != nil || ms <= 0 {
		return 0, false
	}
	return ms, true
}

// progressService reads the queue of one service once and writes the row of each wish.
func (s *Service) progressService(ctx context.Context, kind arr.Kind, wishes []wishq.ListProcurementWishesRow) error {
	c, _, err := s.arrClient(ctx, kind, true)
	var q arr.Queue
	if err == nil {
		q, err = c.Queue(ctx)
		s.arrFailed(kind, err)
	}
	if err != nil {
		s.recordArr(ctx, kind, err)
		return s.unreadable(ctx, kind, wishes)
	}
	byTitle := map[int][]arr.QueueRecord{}
	for _, r := range q.Records {
		byTitle[r.TitleID] = append(byTitle[r.TitleID], r)
	}
	var lastErr error
	for i, w := range wishes {
		row, err := s.progressOf(ctx, c, kind, w, byTitle)
		if err != nil {
			s.arrFailed(kind, err)
			if serviceDown(err) {
				s.recordArr(ctx, kind, err)
				return s.unreadable(ctx, kind, wishes[i:])
			}
			lastErr = err
			if err := s.unreadable(ctx, kind, wishes[i:i+1]); err != nil {
				return err
			}
			continue
		}
		if err := s.write.UpsertWishProgress(ctx, row); err != nil {
			return fmt.Errorf("store wish progress: %w", err)
		}
	}
	s.recordArr(ctx, kind, lastErr)
	return nil
}

// unreadable marks the rows of wishes the service could not be read for (architecture 22.5).
func (s *Service) unreadable(ctx context.Context, kind arr.Kind, wishes []wishq.ListProcurementWishesRow) error {
	for _, w := range wishes {
		if err := s.write.SetWishProgressUnreadable(ctx, wishq.SetWishProgressUnreadableParams{WishID: w.ID, Service: string(kind), Now: s.now().UnixMilli()}); err != nil {
			return fmt.Errorf("store wish progress: %w", err)
		}
	}
	return nil
}

// progressOf derives the row of one wish (architecture 22.2): stalled before download before
// import before search.
func (s *Service) progressOf(ctx context.Context, c *arr.Client, kind arr.Kind, w wishq.ListProcurementWishesRow,
	byTitle map[int][]arr.QueueRecord,
) (wishq.UpsertWishProgressParams, error) {
	now := s.now()
	row := wishq.UpsertWishProgressParams{WishID: w.ID, Service: string(kind), ReadAt: now.UnixMilli(),
		HasFile: w.HasFile, EpisodesDone: w.EpisodesDone, EpisodesTotal: w.EpisodesTotal, TitleReadAt: w.TitleReadAt}
	titleID := int(w.ArrTitleID)
	var open map[int]bool // the wished seasons still in procurement (Sonarr)
	var wished []int      // every wished season that is not closed without the title
	if kind == arr.Sonarr {
		seasons, err := s.read.ListWishSeasons(ctx, w.ID)
		if err != nil {
			return row, fmt.Errorf("list wish seasons: %w", err)
		}
		open = map[int]bool{}
		for _, se := range seasons {
			switch se.State {
			case "declined", "failed", "withdrawn":
				continue
			case "available":
			default:
				open[int(se.SeasonNumber)] = true
			}
			wished = append(wished, int(se.SeasonNumber))
		}
	}
	incomplete := false
	due := titleID == 0 || !w.TitleReadAt.Valid || now.Sub(time.UnixMilli(w.TitleReadAt.Int64)) >= titleEvery
	if due {
		switch kind {
		case arr.Radarr:
			m, err := c.MovieByTMDB(ctx, int(w.TmdbID))
			if errors.Is(err, arr.ErrNotFound) {
				return notFoundRow(row), nil
			}
			if err != nil {
				return row, err
			}
			titleID = m.ID
			row.HasFile = boolInt(m.HasFile)
		default:
			if !w.TvdbID.Valid {
				return notFoundRow(row), nil
			}
			se, err := c.SeriesByTVDB(ctx, int(w.TvdbID.Int64))
			if errors.Is(err, arr.ErrNotFound) {
				return notFoundRow(row), nil
			}
			if err != nil {
				return row, err
			}
			titleID = se.ID
			done, total, complete, known := seasonFiles(se, wished, open)
			row.HasFile = boolInt(complete)
			row.EpisodesDone, row.EpisodesTotal = sql.NullInt64{}, sql.NullInt64{}
			if known {
				row.EpisodesDone = sql.NullInt64{Int64: int64(done), Valid: true}
				row.EpisodesTotal = sql.NullInt64{Int64: int64(total), Valid: true}
			} else {
				incomplete = true
			}
		}
		row.TitleReadAt = sql.NullInt64{Int64: now.UnixMilli(), Valid: true}
	} else if kind == arr.Sonarr && !w.EpisodesTotal.Valid {
		incomplete = true
	}
	row.ArrTitleID = sql.NullInt64{Int64: int64(titleID), Valid: true}

	var records, problems []arr.QueueRecord
	for _, r := range byTitle[titleID] {
		if kind == arr.Sonarr {
			if !r.SeasonKnown {
				incomplete = true // seasonNumber missing: the entry cannot be mapped (architecture 14)
				continue
			}
			if !open[r.SeasonNumber] {
				continue
			}
		}
		records = append(records, r)
		if r.TrackedDownloadStatus == "warning" || r.TrackedDownloadStatus == "error" {
			problems = append(problems, r)
		}
	}
	switch {
	case len(problems) > 0:
		row.Stage = sql.NullString{String: StageStalled, Valid: true}
		row.AdminMessage = nullString(adminMessage(problems))
	case len(records) > 0:
		row.Stage = sql.NullString{String: StageDownload, Valid: true}
		row.Percent, row.EtaSeconds = downloadFigures(records, now)
		row.AdminMessage = nullString(adminMessage(records))
	case row.HasFile.Valid && row.HasFile.Int64 == 1:
		row.Stage = sql.NullString{String: StageImport, Valid: true}
	default:
		row.Stage = sql.NullString{String: StageSearch, Valid: true}
	}
	if incomplete {
		row.Error = sql.NullString{String: ProgressIncomplete, Valid: true}
	}
	return row, nil
}

func notFoundRow(row wishq.UpsertWishProgressParams) wishq.UpsertWishProgressParams {
	row.ArrTitleID, row.HasFile, row.EpisodesDone, row.EpisodesTotal, row.TitleReadAt = sql.NullInt64{}, sql.NullInt64{}, sql.NullInt64{}, sql.NullInt64{}, sql.NullInt64{}
	row.Error = sql.NullString{String: ProgressNotFound, Valid: true}
	return row
}

func boolInt(b bool) sql.NullInt64 {
	if b {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

// seasonFiles counts the episodes with a file of the wished seasons (episodeFileCount against
// episodeCount, the aired episodes Sonarr counts) and whether the open seasons are complete; known
// is false when Sonarr sent no statistics for a wished season.
func seasonFiles(se arr.Series, wished []int, open map[int]bool) (done, total int, complete, known bool) {
	stats := map[int]*arr.Statistics{}
	for _, s := range se.Seasons {
		stats[s.Number] = s.Statistics
	}
	known, complete = len(wished) > 0, len(open) > 0
	for _, n := range wished {
		st := stats[n]
		if st == nil {
			known, complete = false, false
			continue
		}
		done += st.EpisodeFileCount
		total += st.EpisodeCount
		if open[n] && (st.EpisodeCount == 0 || st.EpisodeFileCount < st.EpisodeCount) {
			complete = false
		}
	}
	return done, total, complete, known
}

// downloadFigures are the percent over the entries with sizes and the longest time left.
func downloadFigures(records []arr.QueueRecord, now time.Time) (percent, eta sql.NullInt64) {
	var size, left int64
	var longest time.Duration
	etaKnown := false
	for _, r := range records {
		if r.SizeKnown {
			size += r.Size
			left += min(r.SizeLeft, r.Size)
		}
		d, ok := r.TimeLeft, r.TimeLeftKnown
		if !ok && !r.EstimatedEnd.IsZero() && r.EstimatedEnd.After(now) {
			d, ok = r.EstimatedEnd.Sub(now), true
		}
		if ok {
			etaKnown = true
			longest = max(longest, d)
		}
	}
	if size > 0 {
		percent = sql.NullInt64{Int64: (size - left) * 100 / size, Valid: true}
	}
	if etaKnown {
		eta = sql.NullInt64{Int64: int64(longest / time.Second), Valid: true}
	}
	return percent, eta
}

// pathPattern finds every word with a slash or a backslash in a message of the service: Unix,
// Windows and UNC paths, also a path cut by a blank ("Film (2024)/film.mkv") and addresses.
var pathPattern = regexp.MustCompile(`[^\s"'<>]*[\\/][^\s"'<>]*`)

// stripPaths replaces every path with "[…]" (architecture 22.5).
func stripPaths(msg string) string {
	return pathPattern.ReplaceAllString(msg, "[…]")
}

// adminMessage joins the messages of the entries and their download programs, without paths, at
// most maxAdminMessage characters.
func adminMessage(records []arr.QueueRecord) string {
	var parts []string
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(stripPaths(p))
		if p != "" && !seen[p] {
			seen[p] = true
			parts = append(parts, p)
		}
	}
	for _, r := range records {
		add(r.ErrorMessage)
		for _, m := range r.StatusMessages {
			for _, line := range m.Messages {
				add(line)
			}
		}
		if r.DownloadClient != "" {
			add("(" + r.DownloadClient + ")")
		}
	}
	msg := strings.Join(parts, "; ")
	if rs := []rune(msg); len(rs) > maxAdminMessage {
		msg = string(rs[:maxAdminMessage-1]) + "…"
	}
	return msg
}

// ProgressOf returns the progress viewers see of the wishes (listWishes, getExternalTitle): only
// rows with a stage, not stale, readable, with the switch on. It reads the table only.
func (s *Service) ProgressOf(ctx context.Context, wishIDs []int64) (map[int64]WishProgress, error) {
	all, err := s.AdminProgressOf(ctx, wishIDs)
	if err != nil {
		return nil, err
	}
	out := map[int64]WishProgress{}
	for id, p := range all {
		if p.Stage == "" || p.Stale || p.Error == ProgressUnreachable || p.Error == ProgressNotFound {
			continue
		}
		out[id] = p.WishProgress
	}
	return out, nil
}

// AdminProgressOf returns the progress of the wishes for the administration (listAdminWishes),
// with service, message, error and staleness; nothing with the switch off. It reads the table only.
func (s *Service) AdminProgressOf(ctx context.Context, wishIDs []int64) (map[int64]AdminWishProgress, error) {
	out := map[int64]AdminWishProgress{}
	if len(wishIDs) == 0 {
		return out, nil
	}
	if on, err := s.boolState(ctx, SettingShowProgress, true); err != nil || !on {
		return out, err
	}
	rows, err := s.read.ListWishProgressOf(ctx, wishIDs)
	if err != nil {
		return nil, fmt.Errorf("list wish progress: %w", err)
	}
	now := s.now()
	for _, r := range rows {
		p := AdminWishProgress{Service: r.Service, Message: r.AdminMessage.String, Error: r.Error.String}
		p.Stage, p.ReadAt = r.Stage.String, time.UnixMilli(r.ReadAt)
		p.Stale = now.Sub(p.ReadAt) > progressStale
		if r.Percent.Valid {
			v := int(r.Percent.Int64)
			p.Percent = &v
		}
		if r.EtaSeconds.Valid {
			v := int(r.EtaSeconds.Int64)
			p.ETASeconds = &v
		}
		if r.EpisodesTotal.Valid && r.EpisodesDone.Valid {
			p.Episodes = &ProgressEpisodes{Done: int(r.EpisodesDone.Int64), Total: int(r.EpisodesTotal.Int64)}
		}
		out[r.WishID] = p
	}
	return out, nil
}

// progressStatus is the progress part of the status of a service; it reads the table only.
func (s *Service) progressStatus(ctx context.Context, kind arr.Kind) (ArrProgressStatus, error) {
	on, err := s.boolState(ctx, SettingShowProgress, true)
	if err != nil {
		return ArrProgressStatus{}, err
	}
	st := ArrProgressStatus{Enabled: on}
	rows, err := s.read.ProgressByService(ctx)
	if err != nil {
		return ArrProgressStatus{}, fmt.Errorf("count wish progress: %w", err)
	}
	for _, r := range rows {
		if r.Service != string(kind) {
			continue
		}
		st.Tracked = int(r.Tracked)
		if r.LastReadAt > 0 {
			t := time.UnixMilli(r.LastReadAt)
			st.LastReadAt = &t
		}
		if r.Unreachable > 0 {
			st.Error = ProgressUnreachable
		}
	}
	return st, nil
}
