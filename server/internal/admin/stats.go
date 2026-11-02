package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Usage statistics (A-8, design 3.6), unfiltered over every profile. Watch time is the played time
// of the progress events whose server time lies in the period; a play is a playback session, an
// event without one counts on its own. The share of the modes comes from the playback sessions
// started in the period.

// topTitlesLimit is the length of the most watched list (contract maxItems).
const topTitlesLimit = 20

// maxTimeWindowDays bounds the watch-time grid: events older than 90 days are condensed into days
// (progress_days, migration 00019) and have lost their time of day.
const maxTimeWindowDays = 90

// quarterMs is a quarter hour in milliseconds, the unit of StatsWatchTimeByQuarter.
const quarterMs = int64(15 * time.Minute / time.Millisecond)

// Stats returns the statistics of the last days days (7, 30, 90 or 365) up to now.
func (s *Service) Stats(ctx context.Context, days int) (model.Stats, error) {
	switch days {
	case 7, 30, 90, 365:
	default:
		return model.Stats{}, (&model.ValidationError{}).Add("days", "invalid_value")
	}
	to := s.now()
	// [from, to]: an event stamped in this millisecond counts.
	from, toMs := to.Add(-time.Duration(days)*24*time.Hour).UnixMilli(), to.UnixMilli()+1
	window := min(days, maxTimeWindowDays)
	fromWindow := to.Add(-time.Duration(window) * 24 * time.Hour).UnixMilli()

	// The five aggregations read the reader pool side by side: a period of a year over a million
	// events takes about a second each (docs/perf/M8.md).
	var (
		wg                                        sync.WaitGroup
		top                                       []systemq.StatsTopTitlesRow
		profiles                                  []systemq.StatsWatchTimeByProfileRow
		modes                                     systemq.StatsPlaybackModesRow
		devices                                   []systemq.StatsDevicesRow
		quarters                                  []systemq.StatsWatchTimeByQuarterRow
		errTop, errProf, errMode, errDev, errQuar error
	)
	wg.Go(func() {
		top, errTop = s.sys.StatsTopTitles(ctx, systemq.StatsTopTitlesParams{FromMs: from, ToMs: toMs, LimitCount: topTitlesLimit})
	})
	wg.Go(func() {
		profiles, errProf = s.sys.StatsWatchTimeByProfile(ctx, systemq.StatsWatchTimeByProfileParams{FromMs: from, ToMs: toMs})
	})
	wg.Go(func() {
		modes, errMode = s.sys.StatsPlaybackModes(ctx, systemq.StatsPlaybackModesParams{FromMs: from, ToMs: toMs})
	})
	wg.Go(func() {
		devices, errDev = s.sys.StatsDevices(ctx, systemq.StatsDevicesParams{FromMs: from, ToMs: toMs})
	})
	wg.Go(func() {
		quarters, errQuar = s.sys.StatsWatchTimeByQuarter(ctx, systemq.StatsWatchTimeByQuarterParams{FromMs: fromWindow, ToMs: toMs})
	})
	wg.Wait()
	if err := errors.Join(errTop, errProf, errMode, errDev, errQuar); err != nil {
		return model.Stats{}, fmt.Errorf("stats: %w", err)
	}

	out := model.Stats{
		Days:      days,
		TopTitles: make([]model.StatTitle, 0, len(top)),
		WatchTime: make([]model.StatProfile, 0, len(profiles)),
		Playback: model.StatPlayback{
			Direct: int(modes.Direct), Remux: int(modes.Remux), Transcode: int(modes.Transcode),
			TranscodeShare: share(modes.Transcode, modes.Direct+modes.Remux+modes.Transcode),
			HardwareShare:  share(modes.TranscodeHardware, modes.Transcode),
		},
		Devices:                make([]model.StatDevice, 0, len(devices)),
		WatchTimeByQuarterHour: weekGrid(quarters, s.zone),
		TimeWindowDays:         window,
		TimeZone:               s.zoneName(),
	}
	imageIDs := make([]string, 0, len(top))
	for _, t := range top {
		st := model.StatTitle{ItemID: t.ItemID, Title: t.Title, Kind: t.Kind, Plays: int(t.Plays), WatchedMs: t.WatchedMs, ImageItemID: t.ItemID}
		if t.Year.Valid {
			year := int(t.Year.Int64)
			st.Year = &year
		}
		if t.SeriesID.Valid {
			st.ImageItemID = t.SeriesID.String // an episode shows the images of its series (concept 12)
		}
		imageIDs = append(imageIDs, st.ImageItemID)
		out.TopTitles = append(out.TopTitles, st)
	}
	posters, err := posterHashes(ctx, s.sys, imageIDs)
	if err != nil {
		return model.Stats{}, fmt.Errorf("stats posters: %w", err)
	}
	backdrops, err := backdropHashes(ctx, s.sys, imageIDs)
	if err != nil {
		return model.Stats{}, fmt.Errorf("stats backdrops: %w", err)
	}
	for i := range out.TopTitles {
		t := &out.TopTitles[i]
		t.PosterHash, t.BackdropHash = posters[t.ImageItemID], backdrops[t.ImageItemID]
	}
	for _, p := range profiles {
		out.WatchTime = append(out.WatchTime, model.StatProfile{ProfileID: p.ProfileID, ProfileName: p.ProfileName,
			AccountEmail: p.AccountEmail, WatchedMs: p.WatchedMs, Plays: int(p.Plays)})
	}
	for _, d := range devices {
		out.Devices = append(out.Devices, model.StatDevice{Type: d.DeviceType, Count: int(d.Devices), WatchedMs: d.WatchedMs})
	}
	return out, nil
}

// weekGrid adds the played time of each UTC quarter hour to its quarter hour of the week in loc:
// model.StatsQuarterHours values, Monday 00:00 first. Quarter hours instead of hours keep zones
// with a half or quarter hour offset right; across a DST switch the local clock decides (the hour
// that repeats in autumn counts twice into the same slot, the skipped hour in spring stays empty).
func weekGrid(quarters []systemq.StatsWatchTimeByQuarterRow, loc *time.Location) []int64 {
	grid := make([]int64, model.StatsQuarterHours)
	for _, q := range quarters {
		t := time.UnixMilli(q.Quarter * quarterMs).In(loc)
		weekday := (int(t.Weekday()) + 6) % 7 // Monday = 0
		grid[weekday*96+t.Hour()*4+t.Minute()/15] += q.WatchedMs
	}
	return grid
}

// zoneName is the IANA name of the time zone of the grid. time.Local is named "Local", so its
// name comes from TZ (set in the container, deploy/compose.yaml) or the target of /etc/localtime;
// "UTC" when neither names one.
func (s *Service) zoneName() string {
	if name := s.zone.String(); name != "Local" {
		return name
	}
	if tz := strings.TrimPrefix(s.getenv("TZ"), ":"); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			return tz
		}
	}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if _, name, ok := strings.Cut(target, "zoneinfo/"); ok {
			return name
		}
	}
	return "UTC"
}

// share is part/whole, 0 without a whole.
func share(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}
