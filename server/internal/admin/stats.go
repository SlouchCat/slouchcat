package admin

import (
	"context"
	"errors"
	"fmt"
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

	// The four aggregations read the reader pool side by side: a period of a year over a million
	// events takes about a second each (docs/perf/M8.md).
	var (
		wg                               sync.WaitGroup
		top                              []systemq.StatsTopTitlesRow
		profiles                         []systemq.StatsWatchTimeByProfileRow
		modes                            systemq.StatsPlaybackModesRow
		devices                          []systemq.StatsDevicesRow
		errTop, errProf, errMode, errDev error
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
	wg.Wait()
	if err := errors.Join(errTop, errProf, errMode, errDev); err != nil {
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
		Devices: make([]model.StatDevice, 0, len(devices)),
	}
	for _, t := range top {
		st := model.StatTitle{ItemID: t.ItemID, Title: t.Title, Kind: t.Kind, Plays: int(t.Plays), WatchedMs: t.WatchedMs}
		if t.Year.Valid {
			year := int(t.Year.Int64)
			st.Year = &year
		}
		out.TopTitles = append(out.TopTitles, st)
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

// share is part/whole, 0 without a whole.
func share(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}
