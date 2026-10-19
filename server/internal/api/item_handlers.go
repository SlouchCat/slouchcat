package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
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

func (e serviceProblem) VisitGetHomeResponse(w http.ResponseWriter) error        { return e.visit(w) }
func (e serviceProblem) VisitGetItemResponse(w http.ResponseWriter) error        { return e.visit(w) }
func (e serviceProblem) VisitStartPlaybackResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitReportProgressResponse(w http.ResponseWriter) error { return e.visit(w) }

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
			items = append(items, toItemRef(it))
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
	ref := toItemRef(d.ItemSummary)
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
		OriginalTitle:  optString(d.OriginalTitle),
		Tagline:        optString(d.Tagline),
		Overview:       optString(d.Overview),
		Genres:         d.Genres,
		TmdbId:         d.TMDBID,
		MediaFiles:     make([]gen.MediaFileInfo, 0, len(d.MediaFiles)),
	}
	if out.Genres == nil {
		out.Genres = []string{}
	}
	if day, err := time.Parse(time.DateOnly, d.ReleaseDate); err == nil {
		out.ReleaseDate = &openapi_types.Date{Time: day}
	}
	for _, f := range d.MediaFiles {
		out.MediaFiles = append(out.MediaFiles, toMediaFileInfo(f))
	}
	return gen.GetItem200JSONResponse(out), nil
}

func (s *server) StartPlayback(ctx context.Context, req gen.StartPlaybackRequestObject) (gen.StartPlaybackResponseObject, error) {
	session, err := s.itemSession(ctx)
	if err != nil {
		return nil, err
	}
	var fileID, deviceID string
	if req.Body != nil {
		fileID = deref(req.Body.MediaFileId)
		deviceID = deref(req.Body.DeviceId)
	}
	info, err := s.opts.Playback.Start(ctx, session.Profile, req.Id, fileID, deviceID)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.StartPlayback200JSONResponse(gen.PlaybackInfo{
		Mode:             gen.PlaybackInfoMode(info.Mode),
		Item:             toItemRef(info.Item),
		MediaFile:        toMediaFileInfo(info.MediaFile),
		StreamUrl:        info.StreamURL,
		ResumePositionMs: info.ResumePositionMs,
	}), nil
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
		MediaFileID: deref(b.MediaFileId),
		PositionMs:  b.PositionMs,
		DurationMs:  b.DurationMs,
		PlayedMs:    b.PlayedMs,
		Seq:         b.Seq,
		State:       string(b.State),
		ClientTime:  b.ClientTime,
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

// getItemStream serves the media file with Range support (binary endpoint, CLAUDE.md rule 6):
// http.ServeContent answers 206 for a satisfiable range and 416 otherwise.
func (s *server) getItemStream(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFrom(r.Context())
	if !ok || s.opts.Playback == nil {
		s.p.internal(w, r, errNoItemsServices)
		return
	}
	src, err := s.opts.Playback.Open(r.Context(), session.Profile, chi.URLParam(r, "id"), r.URL.Query().Get("file"))
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", src.MediaType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

// imageURL is the scoped image URL of an item with the content hash for cache busting.
func imageURL(itemID string, img model.Image) string {
	return BasePath + "/items/" + url.PathEscape(itemID) + "/images/" + img.Kind + "/" + img.Size + "?v=" + url.QueryEscape(img.ContentHash)
}

func toImageRef(itemID string, img *model.Image) *gen.ImageRef {
	if img == nil {
		return nil
	}
	return &gen.ImageRef{Url: imageURL(itemID, *img), Width: img.Width, Height: img.Height}
}

func toItemRef(it model.ItemSummary) gen.ItemRef {
	ref := gen.ItemRef{
		Id:             it.ID,
		Kind:           gen.ItemKind(it.Kind),
		Title:          it.Title,
		Year:           it.Year,
		RuntimeMinutes: it.RuntimeMinutes,
		Images: gen.ItemImages{
			Poster:        toImageRef(it.ID, it.Poster),
			PosterSmall:   toImageRef(it.ID, it.PosterSmall),
			PosterLarge:   toImageRef(it.ID, it.PosterLarge),
			Backdrop:      toImageRef(it.ID, it.Backdrop),
			BackdropSmall: toImageRef(it.ID, it.BackdropSmall),
			BackdropLarge: toImageRef(it.ID, it.BackdropLarge),
		},
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
	return ref
}

func toProgress(p model.Progress) gen.Progress {
	return gen.Progress{
		PositionMs: p.PositionMs,
		DurationMs: p.DurationMs,
		Percent:    float32(p.Percent()),
		Completed:  p.Completed,
		UpdatedAt:  p.UpdatedAt.UTC(),
	}
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
	return out
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
