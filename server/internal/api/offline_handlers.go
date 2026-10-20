package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of the offline operations (contract tag offline, M5 design 2): download options and
// creation, deletion, the manifest, the device sync, the auto-download rule and the public
// revocation check. The binary file and subtitle endpoints live in download_binary.go; the offline
// PIN and the offline events in offline_pin_events.go.

func (e serviceProblem) VisitGetDownloadOptionsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitCreateDownloadsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitDeleteDownloadResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitGetDownloadManifestResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSyncOfflineResponse(w http.ResponseWriter) error      { return e.visit(w) }
func (e serviceProblem) VisitGetDownloadRulesResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetDownloadRulesResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitCheckOfflineRevocationsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// offlineSession returns the session of a scoped offline request.
func (s *server) offlineSession(ctx context.Context) (model.Session, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		return model.Session{}, errNoSession
	}
	if s.opts.Offline == nil {
		return model.Session{}, errNoOffline
	}
	return session, nil
}

func (s *server) GetDownloadOptions(ctx context.Context, req gen.GetDownloadOptionsRequestObject) (gen.GetDownloadOptionsResponseObject, error) {
	session, err := s.offlineSession(ctx)
	if err != nil {
		return nil, err
	}
	o, err := s.opts.Offline.Options(ctx, session, req.Id, deref(req.Params.MediaFileId))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	pid := session.Profile.ID
	out := gen.GetDownloadOptions200JSONResponse{
		MediaFileId: optString(o.MediaFileID), Qualities: make([]gen.DownloadQualityOption, 0, len(o.Qualities)),
		AudioTracks: toTracks(o.AudioTracks), Subtitles: toTracks(o.Subtitles),
	}
	if o.MediaFileID != "" || o.DurationMs > 0 {
		d := o.DurationMs
		out.DurationMs = &d
	}
	if o.MediaFileID == "" {
		n := o.EpisodeCount
		out.EpisodeCount = &n
	}
	for _, q := range o.Qualities {
		out.Qualities = append(out.Qualities, gen.DownloadQualityOption{
			Quality: gen.DownloadQuality(q.Quality), Width: q.Width, Height: q.Height, VideoBitrate: q.VideoBitrate,
			AudioBitrate: q.AudioBitrate, EstimatedBytes: q.EstimatedBytes, Copy: q.Copy,
		})
	}
	if o.Existing != nil {
		st := toDownloadState(pid, *o.Existing)
		out.Existing = &st
	}
	return out, nil
}

func (s *server) CreateDownloads(ctx context.Context, req gen.CreateDownloadsRequestObject) (gen.CreateDownloadsResponseObject, error) {
	session, err := s.offlineSession(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	b := req.Body
	list, err := s.opts.Offline.Create(ctx, session, req.Id, model.DownloadRequest{
		Quality: string(b.Quality), AudioTrack: b.AudioTrack, AudioLanguage: deref(b.AudioLanguage), MediaFileID: deref(b.MediaFileId),
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateDownloads201JSONResponse{Downloads: toDownloadStates(session.Profile.ID, list)}, nil
}

func (s *server) DeleteDownload(ctx context.Context, req gen.DeleteDownloadRequestObject) (gen.DeleteDownloadResponseObject, error) {
	session, err := s.offlineSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.opts.Offline.Delete(ctx, session, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteDownload204Response{}, nil
}

func (s *server) GetDownloadManifest(ctx context.Context, req gen.GetDownloadManifestRequestObject) (gen.GetDownloadManifestResponseObject, error) {
	session, err := s.offlineSession(ctx)
	if err != nil {
		return nil, err
	}
	m, err := s.opts.Offline.Manifest(ctx, session, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.GetDownloadManifest200JSONResponse{
		Id: m.ID, ItemId: m.ItemID, SizeBytes: m.SizeBytes, DurationMs: m.DurationMs, MimeType: m.MimeType, Codecs: m.Codecs,
		Width: m.Width, Height: m.Height, InitLength: m.InitLength, FileUrl: m.FileURL,
		Segments: make([]gen.DownloadSegment, 0, len(m.Segments)), Subtitles: make([]gen.DownloadSubtitle, 0, len(m.Subtitles)),
	}
	for _, seg := range m.Segments {
		out.Segments = append(out.Segments, gen.DownloadSegment{StartMs: seg.StartMs, DurationMs: seg.DurationMs, Offset: seg.Offset, Length: seg.Length})
	}
	if m.Audio != nil {
		a := toTrack(*m.Audio)
		out.Audio = &a
	}
	for _, sub := range m.Subtitles {
		out.Subtitles = append(out.Subtitles, gen.DownloadSubtitle{Track: toTrack(sub.Track), Url: sub.URL})
	}
	return out, nil
}

func (s *server) SyncOffline(ctx context.Context, req gen.SyncOfflineRequestObject) (gen.SyncOfflineResponseObject, error) {
	session, err := s.offlineSession(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	in := model.OfflineSyncRequest{Held: make([]model.HeldDownload, 0, len(req.Body.Held))}
	for _, h := range req.Body.Held {
		in.Held = append(in.Held, model.HeldDownload{ID: h.Id, Complete: h.Complete})
	}
	if req.Body.Network != nil {
		in.Network = string(*req.Body.Network)
	}
	sync, err := s.opts.Offline.Sync(ctx, session, in)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.SyncOffline200JSONResponse{
		ServerTime: sync.ServerTime.UTC(), Downloads: toDownloadStates(session.Profile.ID, sync.Downloads),
		Revoked: nonNilStrings(sync.Revoked), Profiles: make([]gen.OfflineProfile, 0, len(sync.Profiles)),
		Rules: toDownloadRules(sync.Rules),
	}
	for _, p := range sync.Profiles {
		op := gen.OfflineProfile{Id: p.ID, Name: p.Name, Language: gen.Language(p.Language), IsPrimary: p.IsPrimary, HasPin: p.HasPin,
			PinSalt: optString(p.PinSalt), PinIterations: optPositive(p.PinIterations)}
		if p.Accent != "" {
			a := gen.Accent(p.Accent)
			op.Accent = &a
		}
		if v := p.OfflinePin; v != nil {
			op.OfflinePin = &gen.OfflinePinVerifier{Salt: v.Salt, Iterations: v.Iterations, Hash: v.Hash}
		}
		if w := p.WatchTime; w != nil {
			op.WatchTime = &gen.OfflineWatchTime{LimitMinutes: w.LimitMinutes, RemainingMinutes: w.RemainingMinutes, ResetsAt: w.ResetsAt.UTC()}
		}
		out.Profiles = append(out.Profiles, op)
	}
	return out, nil
}

func (s *server) GetDownloadRules(ctx context.Context, _ gen.GetDownloadRulesRequestObject) (gen.GetDownloadRulesResponseObject, error) {
	session, err := s.offlineSession(ctx)
	if err != nil {
		return nil, err
	}
	r, err := s.opts.Offline.Rules(ctx, session)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetDownloadRules200JSONResponse(toDownloadRules(r)), nil
}

func (s *server) SetDownloadRules(ctx context.Context, req gen.SetDownloadRulesRequestObject) (gen.SetDownloadRulesResponseObject, error) {
	session, err := s.offlineSession(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	b := req.Body
	r, err := s.opts.Offline.SetRules(ctx, session, model.DownloadRules{
		Enabled: b.Enabled, NextEpisodes: b.NextEpisodes, NewPodcastEpisodes: b.NewPodcastEpisodes, WifiOnly: b.WifiOnly,
		MaxCount: b.MaxCount, Quality: string(b.Quality),
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetDownloadRules200JSONResponse(toDownloadRules(r)), nil
}

// CheckOfflineRevocations answers the revocation list of the device named by the cookie
// slouchcat_device (public): a device whose sessions are gone (guest revoked) still learns which
// of its downloads to delete. Without the cookie the list is empty.
func (s *server) CheckOfflineRevocations(ctx context.Context, req gen.CheckOfflineRevocationsRequestObject) (gen.CheckOfflineRevocationsResponseObject, error) {
	if s.opts.Offline == nil {
		return nil, errNoOffline
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	device := ""
	if r := requestFrom(ctx); r != nil {
		if c, err := r.Cookie(DeviceCookie); err == nil && deviceIDPattern.MatchString(c.Value) {
			device = c.Value
		}
	}
	revoked, err := s.opts.Offline.Revocations(ctx, device, req.Body.Ids)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CheckOfflineRevocations200JSONResponse{Revoked: nonNilStrings(revoked)}, nil
}

func toDownloadStates(profileID int64, list []model.DownloadState) []gen.DownloadState {
	out := make([]gen.DownloadState, 0, len(list))
	for _, st := range list {
		out = append(out, toDownloadState(profileID, st))
	}
	return out
}

// toDownloadState maps a download; the title (ItemRef) only travels when the service set it, that
// is for the active profile in allowedProfileIds.
func toDownloadState(profileID int64, st model.DownloadState) gen.DownloadState {
	out := gen.DownloadState{
		Id: st.ID, State: gen.DownloadStateValue(st.State), Quality: gen.DownloadQuality(st.Quality), Auto: st.Auto,
		AllowedProfileIds: nonNil(st.AllowedProfileIDs), EstimatedBytes: st.EstimatedBytes, SegmentsDone: st.SegmentsDone,
		SegmentsTotal: optPositive(st.SegmentsTotal), Error: optString(st.Error), CreatedAt: st.CreatedAt.UTC(),
		Item: optItemRef(profileID, st.Item),
	}
	if !st.ReadyAt.IsZero() {
		at := st.ReadyAt.UTC()
		out.ReadyAt = &at
	}
	if st.SizeBytes > 0 {
		size := st.SizeBytes
		out.SizeBytes = &size
	}
	return out
}

func toDownloadRules(r model.DownloadRules) gen.DownloadRules {
	return gen.DownloadRules{Enabled: r.Enabled, NextEpisodes: r.NextEpisodes, NewPodcastEpisodes: r.NewPodcastEpisodes,
		WifiOnly: r.WifiOnly, MaxCount: r.MaxCount, Quality: gen.DownloadQuality(r.Quality)}
}

func toTracks(list []model.Track) []gen.Track {
	out := make([]gen.Track, 0, len(list))
	for _, t := range list {
		out = append(out, toTrack(t))
	}
	return out
}
