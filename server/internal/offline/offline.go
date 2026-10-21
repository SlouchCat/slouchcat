// Package offline prepares downloads for offline playback and keeps devices in step with the server
// (plan M5, O-1 to O-7, AN-4, AN-13, AN-18 to AN-20; M5 design sections 2 and 3.1 to 3.5). M6 adds
// audiobooks and podcast episodes (audio.go: AAC 128/96/64 kbit/s, enclosure fetch) and the
// auto-download of new podcast episodes (O-3).
//
// A download is one title on one device (table downloads, UNIQUE device and title). The job
// download.prepare encodes it with a fixed download profile (high, medium, low: H.264 and AAC in
// fragmented MP4, constant bit rate so the size is predictable) into <dir>/<id>/media.mp4 with a
// fragment index <dir>/<id>/index.jsonl and the text subtitles as WebVTT sidecars below
// <dir>/<id>/subs. The job waits while the prepared copies would exceed the download budget, yields
// to live playback (the transcode manager preempts its lease; it pauses and continues by itself)
// and resumes after a crash at the last complete segment.
//
// The device fetches the file with a download token (dt, design section 2) that names the profile
// it was issued to; every request checks that the profile still belongs to the account and sees
// the title. syncOffline records complete fetches, applies the auto-download rules, prepares
// evicted copies the device still lacks, recomputes for every download which profiles of the
// account may see it (visibility.AllowedProfiles) and answers with the revocation list: the
// tombstones the trigger downloads_tombstone writes for every deleted download, also through
// cascades (guest revoked, device forgotten, title removed).
//
// Visibility is always decided on the server through package visibility; the client only looks up
// allowedProfileIds (plan principle 1).
package offline

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// PlaybackPort is the remaining watch time of a profile for the offline library (AN-18); package
// playback implements it.
type PlaybackPort interface {
	OfflineWatchTime(ctx context.Context, profile model.Profile, now time.Time) (*model.OfflineWatchTime, error)
}

// PinPort is the offline PIN material and the profile list of an account (AN-19); package auth
// implements it.
type PinPort interface {
	OfflinePins(ctx context.Context, accountID int64, profileIDs []int64) (map[int64]model.OfflinePin, error)
	ListMyProfiles(ctx context.Context, accountID int64) ([]model.Profile, error)
}

// Encoder is the part of transcode.Runner the preparation uses: the segment-wise download encode
// (M5 stream A), the keyframe index of a remux, the WebVTT conversion of text subtitles and the
// probe of a fetched enclosure (M6).
type Encoder interface {
	Probe(ctx context.Context, path string) (transcode.Probe, error)
	Download(ctx context.Context, caps transcode.Capabilities, src transcode.Source, plan transcode.Plan, workDir string,
		startSegment int, onInit func(init []byte) error,
		onSegment func(n int, data []byte, start, dur time.Duration) error) error
	Keyframes(ctx context.Context, path string) ([]time.Duration, error)
	Subtitles(ctx context.Context, path string, streamIndex int) ([]byte, error)
	SubtitleStreams(ctx context.Context, path string, streamIndexes []int) (map[int][]byte, error)
	SubtitlesFile(ctx context.Context, path string) ([]byte, error)
}

// Lease is admission budget of one download encode (transcode.DownloadLease).
type Lease interface {
	// Preempted is closed when live playback needs the units.
	Preempted() <-chan struct{}
	// Release returns the units; it is idempotent.
	Release()
}

// Admission reserves transcode budget for download encodes (transcode.Manager.AcquireDownload with
// the cost of transcode.Manager.AdmissionCost, which counts HDR with CPU tonemapping like live
// playback).
type Admission interface {
	AdmissionCost(p transcode.Plan) float64
	AcquireDownload(ctx context.Context, id string, cost float64) (Lease, error)
}

// Timing of the preparation and the cleanup (M5 design 3.3, AN-20).
const (
	JobPrepare      = "download.prepare" // job type of the preparation
	jobPriority     = 5
	budgetWait      = 5 * time.Minute    // a preparation that does not fit the budget tries again
	sourceWait      = 5 * time.Minute    // a preparation whose source cannot be read (yet) tries again
	preemptWait     = 30 * time.Second   // a preempted preparation continues
	cleanupInterval = 10 * time.Minute   // Run
	fetchedKeep     = 24 * time.Hour     // server copy after the device fetched it
	unfetchedKeep   = 7 * 24 * time.Hour // server copy that no device fetched
	tombstoneKeep   = 400 * 24 * time.Hour
	readNoticeKeep  = 90 * 24 * time.Hour // read inbox entries (M7 notifications), by their age
	autoWindow      = 60 * 24 * time.Hour // series watched in this window get auto-downloads (O-3)
	writeBatch      = 500                 // rows per transaction (plan 3.6)
	autoRetryWait   = 6 * time.Hour       // a failed auto-download is tried again after this wait
)

// admissionPatience is how long admission may take before the download shows paused (live
// playback holds the transcode budget).
const admissionPatience = 500 * time.Millisecond

// MaxHeld is the most ids a device reports in one sync or revocation check (contract).
const MaxHeld = 2000

// downloadID matches a download id (ULID).
var downloadID = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// ErrNoPins is returned by Sync when no PinPort is configured.
var ErrNoPins = errors.New("offline: no profile source configured (WithPins)")

// Service is the offline API and the preparation job.
type Service struct {
	db    *store.DB
	read  *systemq.Queries
	write *systemq.Queries
	vis   *visibility.Reader
	audio audioReader // parts and chapters of audiobooks (visibility.Reader; a fake in tests)
	queue *jobs.Queue
	enc   Encoder
	adm   Admission
	caps  transcode.Capabilities
	log   *slog.Logger
	now   func() time.Time

	dir      string
	budget   int64
	key      []byte
	playback PlaybackPort
	pins     PinPort
}

// Option configures New.
type Option func(*Service)

// WithClock replaces the clock (tests).
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// WithDir sets the directory of the prepared downloads (<cache>/downloads).
func WithDir(dir string) Option { return func(s *Service) { s.dir = dir } }

// WithBudget caps the space of the prepared copies in bytes (AN-20); 0 = no cap.
func WithBudget(bytes int64) Option { return func(s *Service) { s.budget = max(bytes, 0) } }

// WithTokenKey sets the HMAC key of download tokens (keyring.Derive("download-token")).
func WithTokenKey(key []byte) Option { return func(s *Service) { s.key = key } }

// WithManager lets download encodes take admission budget from the transcode manager (live
// playback preempts them, plan 3.5) and encode with its capabilities. Without it the encodes use
// the software backend and run without admission.
func WithManager(m *transcode.Manager, caps transcode.Capabilities) Option {
	return func(s *Service) {
		if m != nil {
			s.adm = managerAdmission{m: m}
		}
		s.caps = caps
	}
}

// WithAdmission replaces the admission control (tests); nil runs without admission.
func WithAdmission(a Admission) Option { return func(s *Service) { s.adm = a } }

// WithEncoder replaces the encoder of the preparation (tests).
func WithEncoder(e Encoder) Option { return func(s *Service) { s.enc = e } }

// WithPlayback sets the watch-time source of the sync (package playback).
func WithPlayback(p PlaybackPort) Option { return func(s *Service) { s.playback = p } }

// WithPins sets the profile and PIN source of the sync (package auth).
func WithPins(p PinPort) Option { return func(s *Service) { s.pins = p } }

// New returns the offline service on db. runner encodes the downloads; queue receives the
// preparation jobs, which a worker of RegisterJobs runs.
func New(db *store.DB, vis *visibility.Reader, queue *jobs.Queue, runner transcode.Runner, log *slog.Logger, opts ...Option) *Service {
	s := &Service{
		db: db, read: systemq.New(db.Read), write: systemq.New(db.Write), vis: vis, queue: queue, enc: runner,
		caps: transcode.Capabilities{Backend: transcode.BackendSoftware}, log: log, now: time.Now,
	}
	if vis != nil {
		s.audio = vis
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// managerAdmission adapts transcode.Manager to Admission.
type managerAdmission struct{ m *transcode.Manager }

func (a managerAdmission) AdmissionCost(p transcode.Plan) float64 { return a.m.AdmissionCost(p) }

func (a managerAdmission) AcquireDownload(ctx context.Context, id string, cost float64) (Lease, error) {
	l, err := a.m.AcquireDownload(ctx, id, cost)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// ms converts a time to Unix milliseconds.
func ms(t time.Time) int64 { return t.UnixMilli() }
