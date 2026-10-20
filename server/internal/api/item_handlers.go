package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of the start page, titles, images, playback and progress (contract tag items). They
// pass the profile of the session to the catalog and playback services, which read titles only
// through the visibility layer; an invisible title answers 404 not_found.

var (
	errNoSession       = errors.New("scoped handler without session")
	errNoItemsServices = errors.New("catalog or playback service not configured")
)

func (e serviceProblem) VisitGetHomeResponse(w http.ResponseWriter) error         { return e.visit(w) }
func (e serviceProblem) VisitGetItemResponse(w http.ResponseWriter) error         { return e.visit(w) }
func (e serviceProblem) VisitStartPlaybackResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitReportProgressResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitGetItemTrailersResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetPersonResponse(w http.ResponseWriter) error       { return e.visit(w) }

// itemSession returns the session of a scoped request; the scoped gate only lets requests with a
// session through.
func (s *server) itemSession(ctx context.Context) (model.Session, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		return model.Session{}, errNoSession
	}
	if s.opts.Catalog == nil || s.opts.Playback == nil {
		return model.Session{}, errNoItemsServices
	}
	return session, nil
}

func (s *server) GetHome(ctx context.Context, _ gen.GetHomeRequestObject) (gen.GetHomeResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.opts.Catalog.Home(ctx, session.Profile)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	page := gen.HomePage{Rows: make([]gen.HomeRow, 0, len(rows))}
	for _, row := range rows {
		items := make([]gen.ItemRef, 0, len(row.Items))
		for _, it := range row.Items {
			items = append(items, toItemRef(session.Profile.ID, it))
		}
		page.Rows = append(page.Rows, gen.HomeRow{Key: gen.HomeRowKey(row.Key), Items: items})
	}
	return gen.GetHome200JSONResponse(page), nil
}

func (s *server) GetItem(ctx context.Context, req gen.GetItemRequestObject) (gen.GetItemResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	d, err := s.opts.Catalog.Item(ctx, session.Profile, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	pid := session.Profile.ID
	ref := toItemRef(pid, d.ItemSummary)
	out := gen.ItemDetail{
		Id:             ref.Id,
		Kind:           ref.Kind,
		Title:          ref.Title,
		Year:           ref.Year,
		RuntimeMinutes: ref.RuntimeMinutes,
		Rating:         ref.Rating,
		Images:         ref.Images,
		Accent:         ref.Accent,
		Progress:       ref.Progress,
		ParentId:       ref.ParentId,
		SeasonNumber:   ref.SeasonNumber,
		EpisodeNumber:  ref.EpisodeNumber,
		EpisodeEnd:     ref.EpisodeEnd,
		SeriesTitle:    ref.SeriesTitle,
		ExtraKind:      ref.ExtraKind,
		ChildCount:     ref.ChildCount,
		Author:         ref.Author,
		PublishedAt:    ref.PublishedAt,
		DurationMs:     ref.DurationMs,
		Explicit:       ref.Explicit,
		Narrator:       optString(d.Narrator),
		SeriesName:     optString(d.SeriesName),
		EpisodeCount:   d.EpisodeCount,
		Podcast:        optItemRef(pid, d.Podcast),
		OriginalTitle:  optString(d.OriginalTitle),
		Tagline:        optString(d.Tagline),
		Overview:       optString(d.Overview),
		Genres:         d.Genres,
		TmdbId:         d.TMDBID,
		TvdbId:         d.TVDBID,
		MediaFiles:     make([]gen.MediaFileInfo, 0, len(d.MediaFiles)),
		Seasons:        toItemRefs(pid, d.Seasons, d.Kind == "series"),
		Episodes:       toItemRefs(pid, d.Episodes, d.Kind == "season" || d.Kind == "podcast"),
		Extras:         toItemRefs(pid, d.Extras, d.Kind == "movie" || d.Kind == "series" || d.Kind == "season"),
		Series:         optItemRef(pid, d.Series),
		Season:         optItemRef(pid, d.Season),
		NextEpisode:    optItemRef(pid, d.NextEpisode),
		TrailerCount:   &d.TrailerCount,
	}
	if out.Genres == nil {
		out.Genres = []string{}
	}
	if d.SeriesIndex != nil {
		idx := float32(*d.SeriesIndex)
		out.SeriesIndex = &idx
	}
	if d.Kind == "audiobook" {
		chapters := make([]gen.Chapter, 0, len(d.Chapters))
		for _, c := range d.Chapters {
			chapters = append(chapters, gen.Chapter{StartMs: c.StartMs, EndMs: c.EndMs, Title: optString(c.Title)})
		}
		out.Chapters = &chapters
	}
	if day, err := time.Parse(time.DateOnly, d.ReleaseDate); err == nil {
		out.ReleaseDate = &openapi_types.Date{Time: day}
	}
	for _, f := range d.MediaFiles {
		out.MediaFiles = append(out.MediaFiles, toMediaFileInfo(f))
	}
	cast := make([]gen.CastMember, 0, len(d.Cast))
	for _, c := range d.Cast {
		cast = append(cast, gen.CastMember{PersonId: c.PersonID, Name: c.Name, Role: gen.CastMemberRole(c.Role), Character: optString(c.Character)})
	}
	out.Cast = &cast
	return gen.GetItem200JSONResponse(out), nil
}

// toItemRefs maps a list of the detail; a list that does not apply to the kind is left out.
func toItemRefs(profileID int64, items []model.ItemSummary, applies bool) *[]gen.ItemRef {
	if !applies {
		return nil
	}
	out := make([]gen.ItemRef, 0, len(items))
	for _, it := range items {
		out = append(out, toItemRef(profileID, it))
	}
	return &out
}

func optItemRef(profileID int64, it *model.ItemSummary) *gen.ItemRef {
	if it == nil {
		return nil
	}
	ref := toItemRef(profileID, *it)
	return &ref
}

func (s *server) GetItemTrailers(ctx context.Context, req gen.GetItemTrailersRequestObject) (gen.GetItemTrailersResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	trailers, err := s.opts.Catalog.Trailers(ctx, session.Profile, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.GetItemTrailers200JSONResponse, 0, len(trailers))
	for _, t := range trailers {
		out = append(out, toTrailer(t))
	}
	return out, nil
}

func (s *server) GetPerson(ctx context.Context, req gen.GetPersonRequestObject) (gen.GetPersonResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.opts.Catalog.Person(ctx, session.Profile, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.GetPerson200JSONResponse{Id: p.ID, Name: p.Name}
	out.Credits = make([]struct {
		Character *string               `json:"character,omitempty"`
		Item      gen.ItemRef           `json:"item"`
		Role      gen.PersonCreditsRole `json:"role"`
	}, 0, len(p.Credits))
	for _, c := range p.Credits {
		out.Credits = append(out.Credits, struct {
			Character *string               `json:"character,omitempty"`
			Item      gen.ItemRef           `json:"item"`
			Role      gen.PersonCreditsRole `json:"role"`
		}{Character: optString(c.Character), Item: toItemRef(session.Profile.ID, c.Item), Role: gen.PersonCreditsRole(c.Role)})
	}
	return out, nil
}

// toTrailer maps a trailer with its IFrame URL (youtube-nocookie.com).
func toTrailer(t model.Trailer) gen.Trailer {
	return gen.Trailer{
		Id: t.ID, Lang: t.Lang, YoutubeId: t.YouTubeID, EmbedUrl: t.EmbedURL(), Title: optString(t.Title),
		Source: gen.TrailerSource(t.Source), Official: t.Official,
	}
}

func (s *server) StartPlayback(ctx context.Context, req gen.StartPlaybackRequestObject) (gen.StartPlaybackResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	var in model.PlaybackRequest
	if b := req.Body; b != nil {
		in.MediaFileID = deref(b.MediaFileId)
		in.DeviceID = deref(b.DeviceId)
		in.Capabilities = fromCapabilities(b.Capabilities)
		in.AudioTrack = b.AudioTrack
		in.SubtitleTrack = deref(b.SubtitleTrack)
		if b.SubtitleDelivery != nil {
			in.SubtitleDelivery = string(*b.SubtitleDelivery)
		}
		if b.Quality != nil {
			in.Quality = string(*b.Quality)
		}
		in.PositionMs = b.PositionMs
	}
	info, err := s.opts.Playback.Start(ctx, session, req.Id, in)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.StartPlayback200JSONResponse(toPlaybackInfo(session.Profile.ID, info)), nil
}

func (s *server) ReportProgress(ctx context.Context, req gen.ReportProgressRequestObject) (gen.ReportProgressResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("body", "required")), nil
	}
	b := req.Body
	p, err := s.opts.Playback.Report(ctx, session.Profile, req.Id, model.ProgressEvent{
		MediaFileID:       deref(b.MediaFileId),
		PlaybackSessionID: deref(b.PlaybackSessionId),
		PositionMs:        b.PositionMs,
		DurationMs:        b.DurationMs,
		PlayedMs:          b.PlayedMs,
		Seq:               b.Seq,
		State:             string(b.State),
		ClientTime:        b.ClientTime,
		PlaybackRate:      rate32(b.PlaybackRate),
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ReportProgress200JSONResponse(toProgress(p)), nil
}

// Image kinds and sizes of the contract (M1b image pipeline, plan 3.1): posters in w200, w400 and
// w800, backdrops in w780, w1280 and w1920, each stored as WebP and JPEG.
var (
	imageKinds = map[string]bool{"poster": true, "backdrop": true}
	imageSizes = map[string]map[string]bool{
		"poster":   {"w200": true, "w400": true, "w800": true},
		"backdrop": {"w780": true, "w1280": true, "w1920": true},
	}
)

// getItemImage serves a poster or backdrop with scope (binary endpoint, CLAUDE.md rule 6). The
// query parameter v is the content hash and only busts caches. The format follows the Accept
// header (WebP when listed, else JPEG), so the answer varies by Accept.
func (s *server) getItemImage(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFrom(r.Context())
	if !ok || s.opts.Catalog == nil {
		s.p.internal(w, r, errNoItemsServices)
		return
	}
	id, kind, size := chi.URLParam(r, "id"), chi.URLParam(r, "kind"), chi.URLParam(r, "size")
	if !imageKinds[kind] || !imageSizes[kind][size] {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	w.Header().Add("Vary", "Accept")
	src, err := s.opts.Catalog.Image(r.Context(), session.Profile, id, kind, size, acceptsWebP(r.Header.Values("Accept")))
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", src.MediaType)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

// getItemStream serves the media file of a direct-play session with Range support (binary
// endpoint, CLAUDE.md rule 6), authorised by the stream token (streamGrant): http.ServeContent
// answers 206 for a satisfiable range and 416 otherwise. A token of another title answers 404.
func (s *server) getItemStream(w http.ResponseWriter, r *http.Request) {
	g, ok := grantFrom(r.Context())
	if !ok || s.opts.Playback == nil {
		s.p.internal(w, r, errNoItemsServices)
		return
	}
	if chi.URLParam(r, "id") != g.ItemID {
		s.p.write(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	src, err := s.opts.Playback.OpenGrant(r.Context(), g)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	if rv, ok := src.File.(interface{ Done() <-chan struct{} }); ok {
		defer abortOnRevoke(w, rv.Done())()
	}
	w.Header().Set("Content-Type", src.MediaType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

// abortOnRevoke ends a running response once done is closed (plan 5.4: a revoked playback stops
// within a second). A browser keeps "bytes=0-" open and stops reading while its buffer is full, so
// the handler may sit in a blocked write that no read error reaches: an expired write deadline
// fails that write, ServeContent returns and the server closes the connection. The returned
// function must run before the handler returns (the ResponseController is not used after that).
func abortOnRevoke(w http.ResponseWriter, done <-chan struct{}) func() {
	rc := http.NewResponseController(w)
	finished := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-done:
			_ = rc.SetWriteDeadline(time.Now()) // ErrNotSupported for writers without deadlines
		case <-finished:
		}
	})
	return func() {
		close(finished)
		wg.Wait()
	}
}

// imageURL is the scoped image URL of an item with the content hash for cache busting and the
// profile id p, so a browser cache never serves one profile's image to another (the server ignores p).
func imageURL(itemID string, img model.Image, profileID int64) string {
	return BasePath + "/items/" + url.PathEscape(itemID) + "/images/" + img.Kind + "/" + img.Size + "?v=" + url.QueryEscape(img.ContentHash) +
		"&p=" + strconv.FormatInt(profileID, 10)
}

func toImageRef(itemID string, profileID int64, img *model.Image) *gen.ImageRef {
	if img == nil {
		return nil
	}
	return &gen.ImageRef{Url: imageURL(itemID, *img, profileID), Width: img.Width, Height: img.Height}
}

func toItemRef(profileID int64, it model.ItemSummary) gen.ItemRef {
	ref := gen.ItemRef{
		Id:             it.ID,
		Kind:           gen.ItemKind(it.Kind),
		Title:          it.Title,
		Year:           it.Year,
		RuntimeMinutes: it.RuntimeMinutes,
		Images: gen.ItemImages{
			Poster:        toImageRef(it.ID, profileID, it.Poster),
			PosterSmall:   toImageRef(it.ID, profileID, it.PosterSmall),
			PosterLarge:   toImageRef(it.ID, profileID, it.PosterLarge),
			Backdrop:      toImageRef(it.ID, profileID, it.Backdrop),
			BackdropSmall: toImageRef(it.ID, profileID, it.BackdropSmall),
			BackdropLarge: toImageRef(it.ID, profileID, it.BackdropLarge),
		},
	}
	if it.Author != "" {
		ref.Author = &it.Author
	}
	if it.PublishedAt != nil {
		t := it.PublishedAt.UTC()
		ref.PublishedAt = &t
	}
	ref.DurationMs = it.DurationMs
	if it.Explicit {
		e := true
		ref.Explicit = &e
	}
	if it.Accent != "" {
		ref.Accent = &it.Accent
	}
	if it.Rating != nil {
		r := gen.Rating(*it.Rating)
		ref.Rating = &r
	}
	if it.Progress != nil {
		p := toProgress(*it.Progress)
		ref.Progress = &p
	}
	if it.ParentID != "" {
		ref.ParentId = &it.ParentID
	}
	ref.SeasonNumber, ref.EpisodeNumber, ref.EpisodeEnd, ref.ChildCount = it.SeasonNumber, it.EpisodeNumber, it.EpisodeEnd, it.ChildCount
	ref.SeriesTitle = optString(it.SeriesTitle)
	if it.ExtraKind != "" {
		k := gen.ExtraKind(it.ExtraKind)
		ref.ExtraKind = &k
	}
	return ref
}

func toProgress(p model.Progress) gen.Progress {
	out := gen.Progress{
		PositionMs: p.PositionMs,
		DurationMs: p.DurationMs,
		Percent:    float32(p.Percent()),
		Completed:  p.Completed,
		UpdatedAt:  p.UpdatedAt.UTC(),
		WatchTime:  toWatchTime(p.WatchTime),
	}
	if p.PlaybackRate != nil {
		r := float32(*p.PlaybackRate)
		out.PlaybackRate = &r
	}
	return out
}

// rate32 converts an optional playback rate of the contract (float32) to the model (float64).
func rate32(v *float32) *float64 {
	if v == nil {
		return nil
	}
	r := float64(*v)
	return &r
}

func toMediaFileInfo(f model.MediaFile) gen.MediaFileInfo {
	out := gen.MediaFileInfo{
		Id:           f.ID,
		Container:    f.Container,
		SizeBytes:    f.SizeBytes,
		Available:    f.Available,
		MimeType:     optString(f.MimeType),
		VideoCodec:   optString(f.VideoCodec),
		AudioCodec:   optString(f.AudioCodec),
		VersionLabel: optString(f.VersionLabel),
	}
	if f.Width > 0 && f.Height > 0 {
		out.Width, out.Height = &f.Width, &f.Height
	}
	if f.DurationMs > 0 {
		out.DurationMs = &f.DurationMs
	}
	if f.Bitrate > 0 {
		out.Bitrate = &f.Bitrate
	}
	if f.HDR != "" {
		hdr := gen.MediaFileInfoHdr(f.HDR)
		out.Hdr = &hdr
	}
	if f.Tracks != nil {
		tracks := toTrackList(*f.Tracks)
		out.Tracks = &tracks
	}
	return out
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
