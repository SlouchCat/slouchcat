package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// M8 usage statistics (plan M8, A-8; design 3.6). The administration view is unfiltered, so titles
// are plain StatTitles (id, title, kind), never ItemRefs with scoped image URLs.

// defaultStatsDays is the period without the query parameter days.
const defaultStatsDays = 30

func (e serviceProblem) VisitGetStatsResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) GetStats(ctx context.Context, req gen.GetStatsRequestObject) (gen.GetStatsResponseObject, error) {
	days := defaultStatsDays
	if req.Params.Days != nil {
		days = int(*req.Params.Days)
	}
	st, err := s.opts.Admin.Stats(ctx, days)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.Stats{
		Days:      gen.StatsDays(st.Days),
		TopTitles: make([]gen.StatTitle, 0, len(st.TopTitles)),
		WatchTime: make([]gen.StatProfile, 0, len(st.WatchTime)),
		Playback: gen.StatPlayback{
			Direct: st.Playback.Direct, Remux: st.Playback.Remux, Transcode: st.Playback.Transcode,
			TranscodeShare: st.Playback.TranscodeShare, HardwareShare: st.Playback.HardwareShare,
		},
		Devices: make([]gen.StatDevice, 0, len(st.Devices)),
	}
	for _, t := range st.TopTitles {
		out.TopTitles = append(out.TopTitles, gen.StatTitle{
			ItemId: t.ItemID, Title: t.Title, Kind: gen.ItemKind(t.Kind), Year: t.Year, Plays: t.Plays, WatchedMs: t.WatchedMs,
		})
	}
	for _, p := range st.WatchTime {
		out.WatchTime = append(out.WatchTime, gen.StatProfile{
			ProfileId: p.ProfileID, ProfileName: p.ProfileName, AccountEmail: p.AccountEmail, WatchedMs: p.WatchedMs, Plays: p.Plays,
		})
	}
	for _, d := range st.Devices {
		out.Devices = append(out.Devices, gen.StatDevice{Type: gen.StatDeviceType(d.Type), Count: d.Count, WatchedMs: d.WatchedMs})
	}
	return gen.GetStats200JSONResponse(out), nil
}
