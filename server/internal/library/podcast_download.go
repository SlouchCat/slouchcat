package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/fetch"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// The download of a stored podcast episode, podcast.download (concept 19 part A, 6.2; ADR 0029,
// decisions 1, 2 and 4). One job per episode in the pool podcast (five workers), key
// podcast-dl:<episodeId>, payload without any address:
//
//  1. check: episode and podcast live, no file row that is there, the effective setting still asks
//     for it (or a button did); the folder check (5.4) and the free space, else RetryLater in 30
//     minutes without using an attempt, for at most 24 hours;
//  2. folder: the one of the podcast's newest file, never created anew; only a podcast without any
//     file gets <folder for new subscriptions>/<title>/ with metadata.json and a cover;
//  3. load into .slouchcat-<id>.part in that folder (package fetch: resume, 2 GiB, netguard), under
//     the two counters of the library and of the enclosure's host;
//  4. probe: an audio stream, a duration and a scanned extension, else not_audio; then the tags
//     (6.5), falling back to the untagged file when the tagged one does not probe;
//  5. and 6. under the scan lock of the library: check again, set a hard link on the first free name
//     (never a rename, which would replace a file of the same name) and enter the media file row at
//     the feed episode (a removed row of the episode is revived), with the probe;
//  7. remove the own .slouchcat-* files.
//
// A failed last attempt records its code in podcast_download_failures. Until a file is entered the
// episode plays from its enclosure.

// JobPodcastDownload is the job type of the download; payload podcastDownloadPayload.
const JobPodcastDownload = "podcast.download"

const (
	// prioPodcastDownload is the priority of a download the planner queued.
	prioPodcastDownload = 2
	// podcastWaitRetry and podcastWaitMax bound the wait for a folder that is not there, not
	// writable or full: RetryLater every 30 minutes, at most 24 hours, then failed.
	podcastWaitRetry = 30 * time.Minute
	podcastWaitMax   = 24 * time.Hour
	// podcastLockWait is how long the download waits for the scan lock before it tries again in
	// podcastLockRetry; the loaded file stays.
	podcastLockWait  = 10 * time.Minute
	podcastLockRetry = time.Minute
	// podcastLeftoverAge is the age after which the sweep removes a .slouchcat-* file no active
	// download holds.
	podcastLeftoverAge = 7 * 24 * time.Hour
	// podcastOwnPrefix starts every file the server writes into a podcast folder until it is
	// entered; the leading dot keeps scanner, watcher and poller away (Ignored).
	podcastOwnPrefix = ".slouchcat-"
	podcastDirMode   = 0o775
	// maxPodcastNames bounds the names tried for one episode (decision 5: date, then " (n)").
	maxPodcastNames = 100
)

// Free-space floor of a podcast folder before a download (6.2, step 1; like trickplay): the smaller
// of 10 GiB and 10 % of the drive.
const (
	podcastFloorBytes   = 10 << 30
	podcastFloorPercent = 10
)

// Failure codes of a download (6.2, «Fehler»); never an address (CLAUDE.md rule 10).
const (
	PodcastErrEnclosureUnavailable = "enclosure_unavailable"
	PodcastErrTooLarge             = "too_large"
	PodcastErrNotAudio             = "not_audio"
	PodcastErrNoSpace              = "no_space"
	PodcastErrRootNotWritable      = "root_not_writable"
	PodcastErrRootMissing          = "root_missing"
	// PodcastErrUnsupportedFormat: the enclosure is audio the scanner does not read (O-3); the
	// episode stays a link.
	PodcastErrUnsupportedFormat = "unsupported_format"
)

// podcastDownloadPayload is the payload of podcast.download: the episode, whether a button asked
// for it (it is loaded whatever the setting and the riegel say), whether a fetch reported it as new
// (a smaller scope does not cancel it), and since when it waits for its folder or for space (Unix
// ms; 0 while it does not wait). Never an address.
type podcastDownloadPayload struct {
	EpisodeID    string `json:"episodeId"`
	Manual       bool   `json:"manual,omitempty"`
	New          bool   `json:"new,omitempty"`
	WaitingSince int64  `json:"waitingSince,omitempty"`
}

func podcastDownloadKey(episodeID string) string { return "podcast-dl:" + episodeID }

// podcastDownloadError carries the failure code of a download; a final one ends the job at once.
type podcastDownloadError struct {
	code  string
	err   error
	final bool
}

func (e *podcastDownloadError) Error() string { return e.code + ": " + e.err.Error() }
func (e *podcastDownloadError) Unwrap() error { return e.err }

func downloadFailed(code string, err error) error { return &podcastDownloadError{code: code, err: err} }

// downloadFailedFinal fails the job at once, whatever attempts it has left.
func downloadFailedFinal(code string, err error) error {
	return jobs.Final(&podcastDownloadError{code: code, err: err, final: true})
}

// podcastSpaceOK reports whether a drive with total bytes has free bytes above the floor; a drive
// whose size is unknown passes.
func podcastSpaceOK(total, free int64) bool {
	if total <= 0 {
		return true
	}
	return free >= min(int64(podcastFloorBytes), total*podcastFloorPercent/100)
}

// podcastOwnNames are the files of one download in its folder.
type podcastOwnNames struct{ part, validator, loaded, tagged, probe string }

func podcastOwn(folder, episodeID string) podcastOwnNames {
	base := folder + "/" + podcastOwnPrefix + episodeID
	return podcastOwnNames{part: base + ".part", validator: base + ".validator", loaded: base + ".loaded", tagged: base + ".tagged",
		probe: base + ".probe"}
}

// remove deletes the own files of a download (those that are there).
func (n podcastOwnNames) remove(r *os.Root, keepPart bool) {
	for _, name := range []string{n.loaded, n.tagged, n.probe, n.validator, n.part} {
		if keepPart && (name == n.part || name == n.validator) {
			continue
		}
		_ = r.Remove(name)
	}
}

// podcastSlots are the two counters of the downloads (6.2, «Warteschlange»): per library as many as
// its podcast_parallel says, per host of the enclosure (before redirects) one. A worker that waits
// keeps its place in the pool.
type podcastSlots struct {
	mu   sync.Mutex
	lib  map[int64]int
	host map[string]bool
	wake chan struct{}
}

func newPodcastSlots() *podcastSlots {
	return &podcastSlots{lib: map[int64]int{}, host: map[string]bool{}, wake: make(chan struct{})}
}

// acquire waits for a slot of the library (at most limit at once) and of the host; release gives
// both back.
func (p *podcastSlots) acquire(ctx context.Context, libraryID int64, limit int, host string) (release func(), err error) {
	for {
		p.mu.Lock()
		if p.lib[libraryID] < max(limit, 1) && !p.host[host] {
			p.lib[libraryID]++
			p.host[host] = true
			p.mu.Unlock()
			var once sync.Once
			return func() { once.Do(func() { p.release(libraryID, host) }) }, nil
		}
		wake := p.wake
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-wake:
		}
	}
}

func (p *podcastSlots) release(libraryID int64, host string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lib[libraryID]--
	delete(p.host, host)
	close(p.wake)
	p.wake = make(chan struct{})
}

// handlePodcastDownload is the handler of podcast.download. A failure of the last attempt (or a
// final one) records its code in podcast_download_failures.
func (s *Service) handlePodcastDownload(ctx context.Context, j *jobs.Job) error {
	var p podcastDownloadPayload
	if err := decodePayload(j, &p); err != nil {
		return jobs.Final(err)
	}
	err := s.podcastDownload(ctx, j, &p)
	var later *jobs.RetryLaterError
	if err == nil || errors.As(err, &later) || ctx.Err() != nil {
		return err
	}
	code := PodcastErrEnclosureUnavailable
	var de *podcastDownloadError
	final := errors.As(err, &de) && de.final
	if de != nil {
		code = de.code
	}
	if final || j.Attempts >= j.MaxAttempts {
		if rerr := systemq.New(s.db.Write).PodcastRecordDownloadFailure(context.WithoutCancel(ctx), systemq.PodcastRecordDownloadFailureParams{
			ItemID: p.EpisodeID, Error: code, Now: s.now().UnixMilli(),
		}); rerr != nil {
			s.log.WarnContext(ctx, "record podcast download failure", "item_id", p.EpisodeID, "err", rerr)
		}
		s.log.WarnContext(ctx, "podcast episode not stored", "item_id", p.EpisodeID, "code", code)
	}
	return err
}

// podcastTarget is the folder a download writes into: a root and a first-level folder; create when
// the podcast has no file anywhere and the folder is to be made.
type podcastTarget struct {
	root   appq.LibraryRoot
	folder string
	create bool
}

// errPodcastFolderMissing: the folder of the podcast that the database knows is not there.
var errPodcastFolderMissing = errors.New("podcast folder missing")

// podcastDownload runs one attempt of a download.
func (s *Service) podcastDownload(ctx context.Context, j *jobs.Job, p *podcastDownloadPayload) error {
	ep, set, _, ok, err := s.podcastDownloadWanted(ctx, p.EpisodeID, p.Manual)
	if err != nil {
		return err
	}
	if !ok {
		jobs.Skip(ctx)
		return nil
	}
	if !podcastFormatKnown(ep.EnclosureUrl.String, ep.EnclosureType.String) {
		return downloadFailedFinal(PodcastErrUnsupportedFormat, errors.New("the scanner does not read the enclosure's format"))
	}
	target, err := s.podcastDownloadTarget(ctx, set)
	switch {
	case errors.Is(err, errPodcastFolderMissing), errors.Is(err, model.ErrRootRequired):
		return s.podcastWait(ctx, j, p, PodcastErrRootMissing)
	case err != nil:
		return err
	}
	switch check := s.CheckPodcastFolder(ctx, target.root); {
	case check.Problem == model.PodcastFolderNotMounted:
		return s.podcastWait(ctx, j, p, PodcastErrRootMissing)
	case !check.Writable:
		return s.podcastWait(ctx, j, p, PodcastErrRootNotWritable)
	}
	if total, free, err := s.diskSpace(target.root.Path); err == nil && !podcastSpaceOK(total, free) {
		return s.podcastWait(ctx, j, p, PodcastErrNoSpace)
	}
	pr, err := paths.NewRoot(target.root.Path)
	if err != nil {
		return err
	}
	r, err := paths.Open(pr)
	if err != nil {
		return s.podcastWait(ctx, j, p, PodcastErrRootMissing)
	}
	defer r.Close()
	if target.create {
		if target.folder, err = s.createPodcastFolder(ctx, r, target.root, set); err != nil {
			return err
		}
	} else if fi, err := r.Lstat(target.folder); err != nil || !fi.IsDir() {
		return s.podcastWait(ctx, j, p, PodcastErrRootMissing)
	}

	own := podcastOwn(target.folder, ep.ID)
	if !podcastWriteProbe(r, own.probe) {
		// The root passed, the podcast's own folder does not take files (made by another user).
		return s.podcastWait(ctx, j, p, PodcastErrRootNotWritable)
	}
	contentType, err := s.loadPodcastEpisode(ctx, r, own, ep, set)
	if err != nil {
		return err
	}
	src, ext, probe, err := s.checkPodcastEpisode(ctx, pr, r, own, ep, set, contentType)
	if err != nil {
		return err
	}

	lockCtx, cancel := context.WithTimeout(ctx, podcastLockWait)
	unlock, err := s.lockScan(lockCtx, set.LibraryID)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return jobs.RetryLater(podcastLockRetry, "scan of the library running")
	}
	defer unlock()
	entered, err := s.enterPodcastFile(ctx, r, target, ep.ID, p.Manual, src, ext, probe)
	if err != nil {
		return err
	}
	own.remove(r, false)
	if !entered {
		jobs.Skip(ctx)
	}
	return nil
}

// podcastDownloadWanted loads the episode and its podcast's setting and reports whether a download
// is still wanted (ok): episode and podcast live with a feed, an enclosure, no file row that is
// there, and «Speichern» in effect unless a button asked (manual); a removed episode (all its rows
// missing) only on a button, then revive names one of its rows.
func (s *Service) podcastDownloadWanted(ctx context.Context, episodeID string, manual bool) (
	ep systemq.Item, set systemq.PodcastEffectiveSettingsRow, revive string, ok bool, err error,
) {
	read := systemq.New(s.db.Read)
	ep, err = read.GetItemSystem(ctx, episodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return ep, set, "", false, nil
	}
	if err != nil {
		return ep, set, "", false, fmt.Errorf("load episode %s: %w", episodeID, err)
	}
	if ep.Kind != "podcast_episode" || ep.DeletedAt.Valid || !ep.EnclosureUrl.Valid || !ep.ParentID.Valid {
		return ep, set, "", false, nil
	}
	sets, err := read.PodcastEffectiveSettings(ctx, systemq.PodcastEffectiveSettingsParams{PodcastID: ep.ParentID.String})
	if err != nil {
		return ep, set, "", false, fmt.Errorf("setting of podcast %s: %w", ep.ParentID.String, err)
	}
	if len(sets) != 1 || (!manual && sets[0].EffectiveMode != model.PodcastSave) {
		return ep, set, "", false, nil
	}
	files, err := read.PodcastEpisodeFiles(ctx, ep.ID)
	if err != nil {
		return ep, set, "", false, fmt.Errorf("files of episode %s: %w", ep.ID, err)
	}
	for _, f := range files {
		if !f.MissingSince.Valid {
			return ep, set, "", false, nil
		}
		revive = f.ID
	}
	if revive != "" && !manual {
		return ep, set, "", false, nil
	}
	return ep, sets[0], revive, true, nil
}

// podcastWait defers a download whose folder is missing, not writable or full (6.2, step 1): in
// podcastWaitRetry without using an attempt, for at most podcastWaitMax since the first wait, then
// the job fails with code.
func (s *Service) podcastWait(ctx context.Context, j *jobs.Job, p *podcastDownloadPayload, code string) error {
	now := s.now().UnixMilli()
	if p.WaitingSince == 0 {
		p.WaitingSince = now
		raw, err := json.Marshal(p)
		if err != nil {
			return err
		}
		if _, err := s.queue.SetRunningPayload(ctx, j.ID, raw); err != nil {
			return err
		}
	}
	if time.Duration(now-p.WaitingSince)*time.Millisecond >= podcastWaitMax {
		return downloadFailedFinal(code, fmt.Errorf("waited %v", podcastWaitMax))
	}
	return jobs.RetryLater(podcastWaitRetry, code)
}

// podcastDownloadTarget is the folder of a download (6.2, step 2): the one of the podcast's newest
// file that is there, else the folder a download created for it before (pending), else a new one
// below the library's folder for new subscriptions. A known folder is never created anew.
func (s *Service) podcastDownloadTarget(ctx context.Context, set systemq.PodcastEffectiveSettingsRow) (podcastTarget, error) {
	read := systemq.New(s.db.Read)
	newest, err := read.PodcastNewestFolder(ctx, sql.NullString{String: set.PodcastID, Valid: true})
	if err != nil {
		return podcastTarget{}, fmt.Errorf("folder of podcast %s: %w", set.PodcastID, err)
	}
	rootID, folder := int64(0), ""
	switch {
	case len(newest) == 1:
		rootID, folder = newest[0].RootID, newest[0].Folder
	case set.PendingRootID.Valid && set.PendingFolder.Valid:
		rootID, folder = set.PendingRootID.Int64, set.PendingFolder.String
	default:
		lib, err := appq.New(s.db.Read).GetLibrary(ctx, set.LibraryID)
		if err != nil {
			return podcastTarget{}, fmt.Errorf("get library %d: %w", set.LibraryID, err)
		}
		root, err := s.podcastTargetRoot(ctx, lib)
		if err != nil {
			return podcastTarget{}, err
		}
		return podcastTarget{root: root, create: true}, nil
	}
	root, err := appq.New(s.db.Read).GetLibraryRoot(ctx, rootID)
	if errors.Is(err, sql.ErrNoRows) {
		return podcastTarget{}, errPodcastFolderMissing
	}
	if err != nil {
		return podcastTarget{}, fmt.Errorf("get root %d: %w", rootID, err)
	}
	return podcastTarget{root: root, folder: folder}, nil
}

// createPodcastFolder gives a podcast without any file its folder (6.2, step 2) and returns it.
// Creations are serialized (podcastFolderMu), and under the lock the setting is read again: a
// download that waited there takes the folder another one made meanwhile. Otherwise the folder is
// the podcast's title by Audiobookshelf's rule below the root (within 255 bytes UTF-16 and 250
// bytes UTF-8), with metadata.json (Audiobookshelf's podcast schema with feedURL, so a later scan
// finds the subscription) and the cover. A folder of that name is taken when it is there and the
// database knows it for the podcast's feed (an earlier subscription, its episodes deleted), or when
// it is no folder the database knows and holds no metadata.json of another feed; a known folder of
// another podcast, a known folder that is gone (never made anew) or the metadata.json of another
// feed makes the name " (2)", " (3)" and so on. An existing metadata.json is never overwritten.
// The folder is recorded as pending until its first file is entered.
func (s *Service) createPodcastFolder(ctx context.Context, r *os.Root, root appq.LibraryRoot, set systemq.PodcastEffectiveSettingsRow) (string, error) {
	s.podcastFolderMu.Lock()
	folder, made, err := s.makePodcastFolder(ctx, r, root, set)
	s.podcastFolderMu.Unlock()
	if err != nil {
		return "", err
	}
	if made {
		s.fetchPodcastCover(ctx, r, folder, set)
		s.log.InfoContext(ctx, "podcast folder created", "item_id", set.PodcastID, "root_id", root.ID)
	}
	return folder, nil
}

// makePodcastFolder is createPodcastFolder under the lock, without the cover; made says that the
// folder was chosen here (not by a download meanwhile).
func (s *Service) makePodcastFolder(ctx context.Context, r *os.Root, root appq.LibraryRoot, set systemq.PodcastEffectiveSettingsRow) (string, bool, error) {
	if fresh, ok, err := s.podcastSetting(ctx, set.PodcastID); err != nil {
		return "", false, err
	} else if ok {
		set = fresh
		again, err := s.podcastDownloadTarget(ctx, set)
		if err != nil {
			return "", false, err
		}
		if !again.create {
			if again.root.ID != root.ID {
				return "", false, jobs.RetryLater(podcastLockRetry, "the folder of the podcast changed")
			}
			return again.folder, false, nil
		}
	}
	base := strings.TrimLeftFunc(absSanitize(set.Title), func(c rune) bool { return c == '.' || jsSpace(c) })
	if podcastKey(base) == "" {
		base = "Podcast " + set.PodcastID
	}
	read := systemq.New(s.db.Read)
	ofFeed, err := read.PodcastFoldersOf(ctx, systemq.PodcastFoldersOfParams{PodcastID: set.PodcastID, LibraryID: set.LibraryID, FeedUrl: set.FeedUrl})
	if err != nil {
		return "", false, fmt.Errorf("folders of the feed: %w", err)
	}
	folder := ""
	for n := 1; n <= maxPodcastNames && folder == ""; n++ {
		tail := ""
		if n > 1 {
			tail = " (" + strconv.Itoa(n) + ")"
		}
		name := fitName(base, tail)
		if slices.Contains(ofFeed, systemq.PodcastFoldersOfRow{RootID: root.ID, Folder: name}) {
			if fi, err := r.Lstat(name); err == nil && fi.IsDir() {
				folder = name
			}
			continue
		}
		known, err := read.PodcastFilesInFolder(ctx, systemq.PodcastFilesInFolderParams{RootID: root.ID, Prefix: name + "/", Upper: name + "0"})
		if err != nil {
			return "", false, fmt.Errorf("files of folder: %w", err)
		}
		if len(known) > 0 {
			continue
		}
		switch err := r.Mkdir(name, podcastDirMode); {
		case err == nil:
			folder = name
			if err := r.Chmod(folder, podcastDirMode); err != nil {
				s.log.WarnContext(ctx, "set the mode of a new podcast folder", "item_id", set.PodcastID, "err", err)
			}
		case errors.Is(err, fs.ErrExist):
			if data, err := r.ReadFile(name + "/metadata.json"); err == nil {
				if m, ok := podcastJSON(data); !ok || m.feedURL != set.FeedUrl.String {
					continue
				}
			}
			folder = name
		default:
			return "", false, err
		}
	}
	if folder == "" {
		return "", false, errors.New("no free folder name for the podcast")
	}
	if err := systemq.New(s.db.Write).PodcastSetPendingFolder(ctx, systemq.PodcastSetPendingFolderParams{
		ItemID: set.PodcastID, RootID: sql.NullInt64{Int64: root.ID, Valid: true}, Folder: sql.NullString{String: folder, Valid: true},
		Now: s.now().UnixMilli(),
	}); err != nil {
		return "", false, fmt.Errorf("record the new podcast folder: %w", err)
	}
	s.writePodcastMetadata(ctx, r, folder, set)
	return folder, true, nil
}

// writePodcastMetadata writes metadata.json into a new podcast folder unless one is there: first
// under an own name, then a hard link puts it in place (it fails when the name is taken), so
// nobody reads half a file and an existing one is never replaced.
func (s *Service) writePodcastMetadata(ctx context.Context, r *os.Root, folder string, set systemq.PodcastEffectiveSettingsRow) {
	meta := map[string]any{
		"tags": []string{}, "title": set.Title, "author": set.Author.String, "description": "", "releaseDate": nil,
		"genres": []string{}, "feedURL": set.FeedUrl.String, "imageURL": nullable(set.CoverUrl), "itunesPageURL": nil,
		"itunesId": nil, "itunesArtistId": nil, "explicit": set.Explicit == 1, "language": nil, "type": "episodic",
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	final := folder + "/metadata.json"
	if _, err := r.Lstat(final); err == nil {
		return
	}
	tmp := folder + "/" + podcastOwnPrefix + set.PodcastID + "-metadata.json"
	_ = r.Remove(tmp)
	f, err := r.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, podcastFileMode)
	if err != nil {
		s.log.WarnContext(ctx, "write metadata.json of a new podcast folder", "item_id", set.PodcastID, "err", err)
		return
	}
	defer func() { _ = r.Remove(tmp) }()
	_, werr := f.Write(data)
	if err := errors.Join(werr, f.Close(), r.Chmod(tmp, podcastFileMode)); err != nil {
		s.log.WarnContext(ctx, "write metadata.json of a new podcast folder", "item_id", set.PodcastID, "err", err)
		return
	}
	if err := s.linkFile(r, tmp, final); err != nil && !errors.Is(err, fs.ErrExist) {
		s.log.WarnContext(ctx, "write metadata.json of a new podcast folder", "item_id", set.PodcastID, "err", err)
	}
}

func nullable(v sql.NullString) any {
	if !v.Valid || v.String == "" {
		return nil
	}
	return v.String
}

// coverExts name the cover of a new podcast folder by the media type of the image.
var coverExts = map[string]string{"image/jpeg": "jpg", "image/png": "png"}

// fetchPodcastCover loads the podcast's cover into a new folder (cover.jpg, cover.png for a PNG)
// through the guarded client, unless the folder has one; a failure is logged.
func (s *Service) fetchPodcastCover(ctx context.Context, r *os.Root, folder string, set systemq.PodcastEffectiveSettingsRow) {
	if !set.CoverUrl.Valid || set.CoverUrl.String == "" {
		return
	}
	for _, name := range coverNames {
		if _, err := r.Lstat(folder + "/" + name); err == nil {
			return
		}
	}
	own := podcastOwn(folder, set.PodcastID+"-cover")
	defer own.remove(r, false)
	res, err := fetch.Fetch(ctx, fetch.Request{
		Client: fetch.Clients().For(ctx, set.FeedUrl.String, set.FeedOrigin.String), URL: set.CoverUrl.String, Root: r,
		Part: own.part, Validator: own.validator, Final: own.loaded, FileMode: podcastFileMode, DirMode: podcastDirMode, Log: s.log,
	})
	if err != nil {
		s.log.WarnContext(ctx, "load the cover of a new podcast folder", "item_id", set.PodcastID, "err", err)
		return
	}
	ext := coverExts[strings.ToLower(strings.TrimSpace(strings.Split(res.ContentType, ";")[0]))]
	if ext == "" {
		ext = "jpg"
	}
	if err := s.linkFile(r, own.loaded, folder+"/cover."+ext); err != nil && !errors.Is(err, fs.ErrExist) {
		s.log.WarnContext(ctx, "store the cover of a new podcast folder", "item_id", set.PodcastID, "err", err)
	}
}

// loadPodcastEpisode loads the enclosure into the own files of the download (step 3) under the two
// counters and returns the media type of the answer. A body over 2 GiB fails at once (too_large);
// a cancel by the administration removes the partial file, a pause keeps it for the resume.
func (s *Service) loadPodcastEpisode(ctx context.Context, r *os.Root, own podcastOwnNames, ep systemq.Item, set systemq.PodcastEffectiveSettingsRow) (string, error) {
	host := ""
	if u, err := url.Parse(ep.EnclosureUrl.String); err == nil {
		host = strings.ToLower(u.Hostname())
	}
	release, err := s.podcastSlots.acquire(ctx, set.LibraryID, int(set.PodcastParallel), host)
	if err != nil {
		return "", err
	}
	defer release()
	res, err := fetch.Fetch(ctx, fetch.Request{
		Client: fetch.Clients().For(ctx, set.FeedUrl.String, set.FeedOrigin.String), URL: ep.EnclosureUrl.String, Root: r,
		Part: own.part, Validator: own.validator, Final: own.loaded, FileMode: podcastFileMode, DirMode: podcastDirMode, Log: s.log,
		Progress: func(have, total int64) { jobs.Progress(ctx, have, max(total, 0), ep.Title) },
	})
	switch {
	case err == nil:
		jobs.Progress(ctx, res.Bytes, res.Bytes, ep.Title)
		return res.ContentType, nil
	case errors.Is(context.Cause(ctx), jobs.ErrCancelled):
		own.remove(r, false)
		return "", err
	case ctx.Err() != nil:
		return "", err
	case errors.Is(err, fetch.ErrTooLarge):
		own.remove(r, false)
		return "", downloadFailedFinal(PodcastErrTooLarge, err)
	case errors.Is(err, fetch.ErrUnavailable):
		return "", downloadFailed(PodcastErrEnclosureUnavailable, err)
	case errors.Is(err, syscall.ENOSPC):
		return "", downloadFailed(PodcastErrNoSpace, err)
	case errors.Is(err, syscall.EROFS), errors.Is(err, fs.ErrPermission):
		return "", downloadFailed(PodcastErrRootNotWritable, err)
	}
	return "", err
}

// checkPodcastEpisode probes the loaded file (step 4) and tags it (4b): it returns the own file to
// enter (the tagged one when it probes, else the loaded one), its extension and its probe. A file
// without an audio stream or a duration, or of a format the scanner does not read, is removed and
// fails at once (not_audio, unsupported_format).
func (s *Service) checkPodcastEpisode(ctx context.Context, pr paths.Root, r *os.Root, own podcastOwnNames, ep systemq.Item,
	set systemq.PodcastEffectiveSettingsRow, contentType string,
) (src, ext string, probe transcode.Probe, err error) {
	abs, err := paths.Resolve(pr, own.loaded)
	if err != nil {
		return "", "", probe, err
	}
	probe, err = s.probe.Probe(ctx, abs)
	var fileErr *transcode.FileError
	if err != nil && (ctx.Err() != nil || !errors.As(err, &fileErr)) {
		return "", "", probe, fmt.Errorf("probe stored episode %s: %s", ep.ID, strings.ReplaceAll(err.Error(), pr.Dir(), ""))
	}
	audio := mainAudio(probe.Streams)
	if err != nil || audio == nil || probe.Format.Duration <= 0 {
		own.remove(r, false)
		return "", "", probe, downloadFailedFinal(PodcastErrNotAudio, errors.New("the enclosure is no audio file"))
	}
	if ext = podcastExt(ep.EnclosureUrl.String, contentType, probe.Format.Name, audio.Codec); ext == "" {
		own.remove(r, false)
		return "", "", probe, downloadFailedFinal(PodcastErrUnsupportedFormat, errors.New("the scanner does not read the format"))
	}
	src = own.loaded
	tagged, err := paths.Resolve(pr, own.tagged)
	if err != nil {
		return "", "", probe, err
	}
	tags := transcode.AudioTags{Title: ep.Title, Album: set.Title, Artist: set.Author.String}
	if ep.PublishedAt.Valid {
		tags.Date = time.UnixMilli(ep.PublishedAt.Int64).UTC().Format(time.RFC3339)
	}
	switch err := s.tagPodcastAudio(ctx, r, abs, own.tagged, ext, tags); {
	case errors.Is(err, transcode.ErrNotTaggable):
		_ = r.Remove(own.tagged)
	case err != nil:
		s.log.WarnContext(ctx, "tags of a stored episode not written, the untagged file is entered", "item_id", ep.ID, "err",
			strings.ReplaceAll(err.Error(), pr.Dir(), ""))
		_ = r.Remove(own.tagged)
	default:
		tp, err := s.probe.Probe(ctx, tagged)
		if err == nil && mainAudio(tp.Streams) != nil && tp.Format.Duration > 0 {
			src, probe = own.tagged, tp
		} else {
			s.log.WarnContext(ctx, "tagged copy of a stored episode does not probe, the untagged file is entered", "item_id", ep.ID)
			_ = r.Remove(own.tagged)
		}
	}
	if err := r.Chmod(src, podcastFileMode); err != nil {
		s.log.WarnContext(ctx, "set the mode of a stored episode", "item_id", ep.ID, "err", err)
	}
	return src, ext, probe, nil
}

// tagPodcastAudio writes the tagged copy of in (absolute) into the own file name below r: an own
// old copy goes first (a symlink of that name is removed, not followed), the new one is created
// with O_EXCL through the root, and ffmpeg writes into it through transcode.Runner.TagAudio (the
// test seam podcastTag replaces it), with the server's cache for formats that need to seek.
func (s *Service) tagPodcastAudio(ctx context.Context, r *os.Root, in, name, ext string, tags transcode.AudioTags) error {
	if err := r.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := r.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, podcastFileMode)
	if err != nil {
		return err
	}
	tag := s.probe.TagAudio
	if s.podcastTag != nil {
		tag = func(ctx context.Context, in string, out io.Writer, ext string, tags transcode.AudioTags, _ string) error {
			return s.podcastTag(ctx, in, out, ext, tags)
		}
	}
	return errors.Join(tag(ctx, in, f, ext, tags, s.cacheDir), f.Close())
}

// enterPodcastFile enters an own file as the media file of a feed episode (steps 5 and 6), under
// the scan lock the caller holds: it checks again that the episode is still wanted and has no file
// (Audiobookshelf or the pairing may have brought one), sets a hard link on the first name that is
// free on disk and in media_files, and inserts the row with the probe in one transaction (or revives
// a removed row of the episode). entered is false when the episode is no longer wanted; the caller
// removes the own files either way.
func (s *Service) enterPodcastFile(ctx context.Context, r *os.Root, target podcastTarget, episodeID string, manual bool,
	src, ext string, probe transcode.Probe,
) (entered bool, err error) {
	ep, set, revive, ok, err := s.podcastDownloadWanted(ctx, episodeID, manual)
	if err != nil || !ok {
		return false, err
	}
	published := s.now()
	if ep.PublishedAt.Valid {
		published = time.UnixMilli(ep.PublishedAt.Int64)
	}
	name := podcastFileName(ep.Title, ext, published)
	read := systemq.New(s.db.Read)
	linked := ""
	for n := 0; n < maxPodcastNames && linked == ""; n++ {
		rel := target.folder + "/" + podcastNameVariant(name, published, n)
		taken, err := read.PodcastRelPathTaken(ctx, systemq.PodcastRelPathTakenParams{RootID: target.root.ID, RelPath: rel, OwnID: revive})
		if err != nil {
			return false, fmt.Errorf("name check: %w", err)
		}
		if taken == 1 {
			continue
		}
		switch err := s.linkFile(r, src, rel); {
		case err == nil:
			linked = rel
		case errors.Is(err, fs.ErrExist):
		default:
			return false, fmt.Errorf("link stored episode %s: %w", ep.ID, err)
		}
	}
	if linked == "" {
		return false, fmt.Errorf("no free name for stored episode %s", ep.ID)
	}
	if err := s.insertPodcastFile(ctx, r, target.root, set.PodcastID, ep.ID, linked, ext, probe, revive); err != nil {
		if rerr := r.Remove(linked); rerr != nil {
			s.log.WarnContext(ctx, "remove the link of a stored episode that was not entered", "item_id", ep.ID, "err", rerr)
		}
		return false, err
	}
	s.log.InfoContext(ctx, "podcast episode stored", "item_id", ep.ID, "root_id", target.root.ID)
	return true, nil
}

// insertPodcastFile writes the media file row of a stored episode with its probe and its audio
// tracks, clears the episode's failure and the podcast's pending folder, in one transaction.
func (s *Service) insertPodcastFile(ctx context.Context, r *os.Root, root appq.LibraryRoot, podcastID, episodeID, rel, ext string,
	probe transcode.Probe, revive string,
) error {
	fi, err := r.Stat(rel)
	if err != nil {
		return err
	}
	hash, err := PartialHash(r, rel)
	if err != nil {
		return err
	}
	id := fileIdentity(fi)
	raw, err := json.Marshal(probe)
	if err != nil {
		return fmt.Errorf("encode probe: %w", err)
	}
	now := s.now().UnixMilli()
	size, mtime := fi.Size(), fi.ModTime().UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	fileID := revive
	if revive != "" {
		n, err := q.PodcastReviveMediaFile(ctx, systemq.PodcastReviveMediaFileParams{
			RootID: root.ID, RelPath: rel, Dev: toInt64(id.dev), Inode: toInt64(id.inode), Size: size, Mtime: mtime,
			PartialHash: nullString(hash), Container: ext, Now: now, ID: revive, ItemID: episodeID,
		})
		if err != nil {
			return fmt.Errorf("revive media file: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("media file %s of episode %s changed meanwhile", revive, episodeID)
		}
	} else {
		fileID = s.newID()
		if _, err := q.InsertMediaFile(ctx, systemq.InsertMediaFileParams{
			ID: fileID, ItemID: episodeID, RootID: root.ID, RelPath: rel, Dev: toInt64(id.dev), Inode: toInt64(id.inode),
			Size: size, Mtime: mtime, PartialHash: nullString(hash), Container: ext, Now: now,
		}); err != nil {
			return fmt.Errorf("insert media file: %w", err)
		}
	}
	params := systemq.UpdateMediaFileProbeParams{
		Container:  canonicalContainer(probe.Format.Name, ext),
		DurationMs: sql.NullInt64{Int64: probe.Format.Duration.Milliseconds(), Valid: probe.Format.Duration > 0},
		Probe:      sql.NullString{String: string(raw), Valid: true}, Now: sql.NullInt64{Int64: now, Valid: true},
		ID: fileID, Size: size, Mtime: mtime,
	}
	if a := mainAudio(probe.Streams); a != nil {
		params.AudioCodec = sql.NullString{String: a.Codec, Valid: a.Codec != ""}
	}
	if _, err := q.UpdateMediaFileProbe(ctx, params); err != nil {
		return fmt.Errorf("store probe: %w", err)
	}
	if err := q.DeleteMediaFileTracks(ctx, fileID); err != nil {
		return fmt.Errorf("delete tracks: %w", err)
	}
	for _, st := range probe.Streams {
		if st.Type != "audio" || st.AttachedPic {
			continue
		}
		lang := strings.ToLower(st.Language)
		if lang == "und" {
			lang = ""
		}
		if err := q.InsertTrack(ctx, systemq.InsertTrackParams{
			MediaFileID: fileID, StreamIndex: int64(st.Index), Kind: st.Type, Codec: nullString(st.Codec),
			Language: nullString(lang), Title: nullString(st.Title), Source: "embedded", IsDefault: boolInt(st.Default),
		}); err != nil {
			return fmt.Errorf("insert track: %w", err)
		}
	}
	if err := q.PodcastClearDownloadFailure(ctx, episodeID); err != nil {
		return fmt.Errorf("clear download failure: %w", err)
	}
	if err := q.PodcastClearPendingFolder(ctx, systemq.PodcastClearPendingFolderParams{Now: now, ItemID: podcastID}); err != nil {
		return fmt.Errorf("clear pending folder: %w", err)
	}
	return tx.Commit()
}

// sweepPodcastLeftovers removes the .slouchcat-* files in podcast folders that are older than
// podcastLeftoverAge and belong to no queued or running download (a crash, a full drive; 6.2,
// «Aufräumen»). Only the first level below a podcast root and the root itself are looked at.
func (s *Service) sweepPodcastLeftovers(ctx context.Context) error {
	roots, err := appq.New(s.db.Read).ListAllLibraryRoots(ctx)
	if err != nil {
		return fmt.Errorf("list roots: %w", err)
	}
	read := systemq.New(s.db.Read)
	before := s.now().Add(-podcastLeftoverAge)
	removed := 0
	for _, root := range roots {
		if root.Content != model.RootPodcast {
			continue
		}
		pr, err := paths.NewRoot(root.Path)
		if err != nil {
			continue
		}
		names, err := paths.ReadDirNames(pr, ".", 0)
		if err != nil {
			continue // not mounted: nothing to sweep
		}
		r, err := paths.Open(pr)
		if err != nil {
			continue
		}
		dirs := []string{"."}
		for _, n := range names {
			if !Ignored(n) {
				dirs = append(dirs, n)
			}
		}
		for _, dir := range dirs {
			entries := names
			if dir != "." {
				if entries, err = paths.ReadDirNames(pr, dir, 0); err != nil {
					continue
				}
			}
			for _, n := range entries {
				if !strings.HasPrefix(n, podcastOwnPrefix) {
					continue
				}
				rel := path.Join(dir, n)
				fi, err := r.Lstat(rel)
				if err != nil || !fi.Mode().IsRegular() || fi.ModTime().After(before) {
					continue
				}
				id, _, _ := strings.Cut(strings.TrimPrefix(n, podcastOwnPrefix), ".")
				id, _, _ = strings.Cut(id, "-")
				if _, err := read.GetActiveJobIDByKey(ctx, sql.NullString{String: podcastDownloadKey(id), Valid: true}); err == nil {
					continue
				} else if !errors.Is(err, sql.ErrNoRows) {
					_ = r.Close()
					return fmt.Errorf("active download of %s: %w", id, err)
				}
				if err := r.Remove(rel); err == nil {
					removed++
				}
			}
		}
		_ = r.Close()
	}
	if removed > 0 {
		s.log.InfoContext(ctx, "podcast download leftovers removed", "files", removed)
	}
	return nil
}
