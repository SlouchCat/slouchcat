package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers and mappings of M4 (design sections 2 and 8): tracks (getItemTracks), WebVTT subtitles
// (getPlaybackSubtitle, binary), the watch-time report (getWatchReport), the device and stream
// overviews of the administration, and the contract types of limits, tracks, qualities, chapters,
// segments and watch time.

func (e serviceProblem) VisitGetItemTracksResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitGetWatchReportResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListDevicesResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitDeleteDeviceResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitListStreamsResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitStopStreamResponse(w http.ResponseWriter) error     { return e.visit(w) }

func (s *server) GetItemTracks(ctx context.Context, req gen.GetItemTracksRequestObject) (gen.GetItemTracksResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	list, err := s.opts.Playback.Tracks(ctx, session.Profile, req.Id, deref(req.Params.MediaFileId))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetItemTracks200JSONResponse(toTrackList(list)), nil
}

func (s *server) GetWatchReport(ctx context.Context, req gen.GetWatchReportRequestObject) (gen.GetWatchReportResponseObject, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		return nil, errNoSession
	}
	if s.opts.Playback == nil {
		return nil, errNoItemsServices
	}
	rep, err := s.opts.Playback.WatchReport(ctx, session, req.Id, deref(req.Params.Days))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.GetWatchReport200JSONResponse{ProfileId: rep.ProfileID, LimitMinutes: optPositive(rep.LimitMinutes)}
	out.Days = make([]struct {
		Date    openapi_types.Date `json:"date"`
		Minutes int                `json:"minutes"`
	}, 0, len(rep.Days))
	for _, d := range rep.Days {
		day, err := time.Parse(time.DateOnly, d.Date)
		if err != nil {
			return nil, err
		}
		out.Days = append(out.Days, struct {
			Date    openapi_types.Date `json:"date"`
			Minutes int                `json:"minutes"`
		}{Date: openapi_types.Date{Time: day}, Minutes: d.Minutes})
	}
	return out, nil
}

func (s *server) ListDevices(ctx context.Context, _ gen.ListDevicesRequestObject) (gen.ListDevicesResponseObject, error) {
	list, err := s.opts.Admin.ListDevices(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListDevices200JSONResponse, 0, len(list))
	for _, d := range list {
		dev := gen.AdminDevice{
			Id: d.ID, Name: optString(d.Name), UserAgent: optString(d.UserAgent), OwnerEmail: optString(d.OwnerEmail),
			OwnerProfile: optString(d.OwnerProfile), CreatedAt: d.CreatedAt.UTC(), LastSeenAt: d.LastSeenAt.UTC(),
			CapabilitiesAt: utc(d.CapabilitiesAt), ActiveStreams: d.ActiveStreams,
		}
		if d.Type != "" {
			typ := gen.AdminDeviceType(d.Type)
			dev.Type = &typ
		}
		if c := d.Capabilities; c != nil {
			caps := toCapabilities(*c)
			dev.Capabilities = &caps
		}
		out = append(out, dev)
	}
	return out, nil
}

func (s *server) DeleteDevice(ctx context.Context, req gen.DeleteDeviceRequestObject) (gen.DeleteDeviceResponseObject, error) {
	if err := s.opts.Admin.DeleteDevice(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteDevice204Response{}, nil
}

func (s *server) ListStreams(ctx context.Context, _ gen.ListStreamsRequestObject) (gen.ListStreamsResponseObject, error) {
	list, err := s.opts.Admin.ListStreams(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListStreams200JSONResponse, 0, len(list))
	for _, st := range list {
		item := gen.AdminStream{
			Id: st.ID, AccountEmail: st.AccountEmail, ProfileName: st.ProfileName, ItemId: st.ItemID, ItemTitle: st.ItemTitle,
			MediaFileId: st.MediaFileID, Mode: gen.AdminStreamMode(st.Mode), Backend: optString(st.Backend), Hardware: st.Hardware,
			Width: optPositive(st.Width), Height: optPositive(st.Height), Bitrate: optPositive(st.Bitrate), Reason: optString(st.Reason),
			StartedAt: st.StartedAt.UTC(), LastActivityAt: st.LastActivityAt.UTC(), PositionMs: st.PositionMs, DurationMs: st.DurationMs,
		}
		if st.Quality != "" {
			q := gen.Quality(st.Quality)
			item.Quality = &q
		}
		if d := st.Device; d != nil {
			item.Device = &struct {
				Id   string  `json:"id"`
				Name *string `json:"name,omitempty"`
			}{Id: d.ID, Name: optString(d.Name)}
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *server) StopStream(ctx context.Context, req gen.StopStreamRequestObject) (gen.StopStreamResponseObject, error) {
	if err := s.opts.Admin.StopStream(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.StopStream204Response{}, nil
}

// getPlaybackSubtitle serves the selected text subtitle track of a playback session as WebVTT
// (binary endpoint, CLAUDE.md rule 6), authorised by the stream token like the HLS endpoints. The
// path id must be the session the token names; a track the playback did not select for text
// delivery answers 404.
func (s *server) getPlaybackSubtitle(w http.ResponseWriter, r *http.Request) {
	g, ok := grantFrom(r.Context())
	if !ok || s.opts.Playback == nil {
		s.p.internal(w, r, errNoItemsServices)
		return
	}
	name := chi.URLParam(r, "track")
	track, err := strconv.ParseInt(strings.TrimSuffix(name, ".vtt"), 10, 64)
	if chi.URLParam(r, "id") != g.ID || !strings.HasSuffix(name, ".vtt") || err != nil || track <= 0 {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	src, err := s.opts.Playback.Subtitle(r.Context(), g, track)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", src.MediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

func optPositive(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

// toPlaybackLimits maps limits to the contract (absent = no limit).
func toPlaybackLimits(l model.PlaybackLimits) gen.PlaybackLimits {
	out := gen.PlaybackLimits{MaxBitrateKbps: optPositive(l.MaxBitrateKbps), MaxStreams: optPositive(l.MaxStreams),
		WatchLimitMinutes: optPositive(l.WatchLimitMinutes)}
	if l.MaxHeight > 0 {
		h := gen.OutputHeight(l.MaxHeight)
		out.MaxHeight = &h
	}
	return out
}

func toEffectiveLimits(l model.PlaybackLimits) gen.EffectiveLimits {
	return gen.EffectiveLimits{MaxBitrateKbps: optPositive(l.MaxBitrateKbps), MaxHeight: optPositive(l.MaxHeight),
		MaxStreams: optPositive(l.MaxStreams), WatchLimitMinutes: optPositive(l.WatchLimitMinutes)}
}

// fromPlaybackLimits maps request limits; nil keeps the stored ones. An explicit 0 or a negative
// number reaches the validation of package auth as an invalid value.
func fromPlaybackLimits(l *gen.PlaybackLimits) *model.PlaybackLimits {
	if l == nil {
		return nil
	}
	val := func(v *int) int {
		switch {
		case v == nil:
			return 0
		case *v <= 0:
			return -1
		}
		return *v
	}
	out := model.PlaybackLimits{MaxBitrateKbps: val(l.MaxBitrateKbps), MaxStreams: val(l.MaxStreams), WatchLimitMinutes: val(l.WatchLimitMinutes)}
	if l.MaxHeight != nil {
		out.MaxHeight = int(*l.MaxHeight)
		if out.MaxHeight <= 0 {
			out.MaxHeight = -1
		}
	}
	return &out
}

func toWatchTime(w *model.WatchTime) *gen.WatchTime {
	if w == nil {
		return nil
	}
	return &gen.WatchTime{LimitMinutes: w.LimitMinutes, UsedMinutes: w.UsedMinutes, RemainingMinutes: w.RemainingMinutes}
}

func toTrack(t model.Track) gen.Track {
	out := gen.Track{
		Id: t.ID, Index: t.Index, Kind: gen.TrackKind(t.Kind), Codec: t.Codec, Language: optString(t.Language), Title: optString(t.Title),
		Default: t.Default, Forced: t.Forced, Source: gen.TrackSource(t.Source), Text: t.Text, Channels: optPositive(t.Channels),
	}
	return out
}

func toTrackList(l model.TrackList) gen.TrackList {
	out := gen.TrackList{MediaFileId: l.MediaFileID, Audio: make([]gen.Track, 0, len(l.Audio)), Subtitles: make([]gen.Track, 0, len(l.Subtitles))}
	for _, t := range l.Audio {
		out.Audio = append(out.Audio, toTrack(t))
	}
	for _, t := range l.Subtitles {
		out.Subtitles = append(out.Subtitles, toTrack(t))
	}
	return out
}

func toCapabilities(c model.PlaybackCapabilities) gen.PlaybackCapabilities {
	out := gen.PlaybackCapabilities{Video: nonNilStrings(c.Video), Audio: nonNilStrings(c.Audio), Hls: gen.PlaybackCapabilitiesHls(c.HLS),
		MaxWidth: optPositive(c.MaxWidth), MaxHeight: optPositive(c.MaxHeight), Name: optString(c.Name)}
	if len(c.Containers) > 0 {
		containers := c.Containers
		out.Containers = &containers
	}
	if c.HDR {
		hdr := true
		out.Hdr = &hdr
	}
	if c.Type != "" {
		typ := gen.PlaybackCapabilitiesType(c.Type)
		out.Type = &typ
	}
	return out
}

func fromCapabilities(c *gen.PlaybackCapabilities) *model.PlaybackCapabilities {
	if c == nil {
		return nil
	}
	out := &model.PlaybackCapabilities{Video: c.Video, Audio: c.Audio, HLS: string(c.Hls), MaxWidth: deref(c.MaxWidth),
		MaxHeight: deref(c.MaxHeight), HDR: deref(c.Hdr), Name: deref(c.Name)}
	if c.Containers != nil {
		out.Containers = *c.Containers
	}
	if c.Type != nil {
		out.Type = string(*c.Type)
	}
	return out
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// toPlaybackInfo maps the description of a started playback.
func toPlaybackInfo(profileID int64, info model.PlaybackInfo) gen.PlaybackInfo {
	out := gen.PlaybackInfo{
		Mode:              gen.PlaybackInfoMode(info.Mode),
		Item:              toItemRef(profileID, info.Item),
		MediaFile:         toMediaFileInfo(info.MediaFile),
		StreamUrl:         info.StreamURL,
		ResumePositionMs:  info.ResumePositionMs,
		PlaybackSessionId: info.PlaybackSessionID,
		Tracks:            toTrackList(info.Tracks),
		SubtitleUrl:       optString(info.SubtitleURL),
		Qualities:         make([]gen.QualityOption, 0, len(info.Qualities)),
		Chapters:          make([]gen.Chapter, 0, len(info.Chapters)),
		NextEpisode:       optItemRef(profileID, info.NextEpisode),
		WatchTime:         toWatchTime(info.WatchTime),
	}
	if info.Tracks.MediaFileID == "" {
		out.Tracks.MediaFileId = info.MediaFile.ID
	}
	sel := info.Selection
	if sel.AudioTrack >= 0 && len(info.Tracks.Audio) > 0 {
		a := sel.AudioTrack
		out.Selection.AudioTrack = &a
	}
	if sel.SubtitleTrack > 0 {
		id := sel.SubtitleTrack
		d := gen.TrackSelectionSubtitleDelivery(sel.SubtitleDelivery)
		out.Selection.SubtitleTrack, out.Selection.SubtitleDelivery = &id, &d
	}
	for _, q := range info.Qualities {
		out.Qualities = append(out.Qualities, gen.QualityOption{Quality: gen.Quality(q.Quality), Width: optPositive(q.Width),
			Height: optPositive(q.Height), Bitrate: optPositive(q.Bitrate), Available: q.Available, Current: q.Current})
	}
	for _, c := range info.Chapters {
		out.Chapters = append(out.Chapters, gen.Chapter{StartMs: c.StartMs, EndMs: c.EndMs, Title: optString(c.Title)})
	}
	segment := func(s *model.Segment) *gen.Segment {
		if s == nil {
			return nil
		}
		return &gen.Segment{StartMs: s.StartMs, EndMs: s.EndMs, Source: gen.SegmentSource(s.Source)}
	}
	out.Segments = gen.Segments{Intro: segment(info.Segments.Intro), Outro: segment(info.Segments.Outro)}
	if d := info.Device; d != nil {
		out.Device = &struct {
			Id   string  `json:"id"`
			Name *string `json:"name,omitempty"`
		}{Id: d.ID, Name: optString(d.Name)}
	}
	if tr := info.Transcoding; tr != nil {
		t := gen.Transcoding{Mode: gen.TranscodingMode(tr.Mode), Backend: optString(tr.Backend), Hardware: tr.Hardware, Reason: tr.Reason,
			Width: optPositive(tr.Width), Height: optPositive(tr.Height), Bitrate: optPositive(tr.Bitrate)}
		if tr.Quality != "" {
			q := gen.Quality(tr.Quality)
			t.Quality = &q
		}
		if tr.Tonemap {
			tm := true
			t.Tonemap = &tm
		}
		out.Transcoding = &t
	}
	return out
}
