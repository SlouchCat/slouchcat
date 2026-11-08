package radio

import (
	"context"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Listen time (concept 16 section 7.10, build plan M-2): counted from the distinct segments a
// listen session fetched (each once while it is in the window, also behind live), not from heartbeats, so a locked iPhone whose timers sleep still counts.
// Each segment credits its duration to the quarter hour of its delivery (split at a quarter
// boundary); credits are summed in memory per profile and quarter and written at most every 30 s
// (sweep) and when a session ends. Segments fetched while the last heartbeat said paused do not
// count (hls.js fills its buffer ahead after a pause). The K-5 limit sees the time through
// SumPlayedMs; at the limit every listen session of the profile ends (ended_by limit).

// quarterMs is the length of a quarter hour in ms (radio_listen_time.quarter = ms / quarterMs).
const quarterMs = 900_000

type creditKey struct {
	profileID int64
	quarter   int64
}

// creditSegment credits segment n to the session's profile the first time the session fetches it.
// The session keeps the numbers it credited while they are in the window (oldest: the first
// segment of the window), not a high-water mark: a listener who seeks back fetches older segments
// for the first time, and they count (critic B-1).
func (s *Service) creditSegment(ls *listenSession, n int64, dur time.Duration, oldest int64) {
	s.rel.mu.Lock()
	defer s.rel.mu.Unlock()
	if _, ok := ls.counted[n]; ok {
		return
	}
	for k := range ls.counted {
		if k < oldest {
			delete(ls.counted, k)
		}
	}
	ls.counted[n] = struct{}{}
	if !ls.playing {
		return
	}
	from := s.now().UnixMilli()
	for left := dur.Milliseconds(); left > 0; {
		q := from / quarterMs
		part := min(left, (q+1)*quarterMs-from)
		s.rel.credits[creditKey{ls.profile.ID, q}] += part
		from += part
		left -= part
	}
}

// flushListenTime writes the credited listen time and ends the listen sessions of a profile that
// reached its daily limit.
func (s *Service) flushListenTime(ctx context.Context) {
	s.rel.mu.Lock()
	credits := s.rel.credits
	s.rel.credits = map[creditKey]int64{}
	profiles := map[int64]model.Profile{}
	for _, ls := range s.rel.sessions {
		profiles[ls.profile.ID] = ls.profile
	}
	s.rel.mu.Unlock()
	if len(credits) == 0 {
		return
	}
	q := appq.New(s.db.Write)
	touched := map[int64]bool{}
	for k, ms := range credits {
		if err := q.AddRadioListenTime(ctx, appq.AddRadioListenTimeParams{ProfileID: k.profileID, Quarter: k.quarter, PlayedMs: ms}); err != nil {
			s.log.WarnContext(ctx, "add radio listen time", "profile_id", k.profileID, "err", err)
			continue
		}
		touched[k.profileID] = true
	}
	for id := range touched {
		if p, ok := profiles[id]; ok {
			if _, err := s.checkLimit(ctx, p); err != nil {
				s.log.WarnContext(ctx, "radio watch limit", "profile_id", id, "err", err)
			}
		}
	}
}

// checkLimit returns the watch time of the profile (nil without a daily limit) and ends its listen
// sessions once nothing remains.
func (s *Service) checkLimit(ctx context.Context, p model.Profile) (*model.WatchTime, error) {
	if s.o.watch == nil {
		return nil, nil
	}
	wt, err := s.o.watch.WatchTime(ctx, p, s.now())
	if err != nil || wt == nil || wt.RemainingMinutes > 0 {
		return wt, err
	}
	if err := s.EndProfile(ctx, p.ID, model.StoppedByLimit); err != nil {
		return wt, err
	}
	s.log.InfoContext(ctx, "watch-time limit reached, radio ended", "profile_id", p.ID,
		"limit_minutes", wt.LimitMinutes, "used_minutes", wt.UsedMinutes)
	return wt, nil
}

// transcodeKbps is the AAC bitrate a relay encodes a source of kbps with.
func transcodeKbps(kbps int) int {
	return transcode.RadioOutputFor(transcode.RadioInput{Codec: "mp3", BitrateKbps: kbps}).BitrateKbps
}
