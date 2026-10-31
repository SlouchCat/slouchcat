package metadata

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The image choice per title (concept 10 6.4, 7.3, ADR 0018): the candidates of every source for
// one kind, asked live at the providers and cached for ten minutes per title and kind; the
// administrator's choice locks the kind (item_images.chosen = 1 plus locked_fields), and lifting
// the lock lets the automatic choice run again.

const (
	// candidateTTL is how long the candidates of a title and kind stay in memory (6.4).
	candidateTTL = 10 * time.Minute
	// maxCandidateEntries bounds the cache; the oldest entries go first.
	maxCandidateEntries = 256
	// previewSize is the size of a provider's preview the administrator's browser loads (6.4).
	previewSize = "w300"
	// maxLocalPreviewBytes is the size limit of a local file served as preview (9.2, like the
	// originals of the pipeline).
	maxLocalPreviewBytes = 20 << 20
	// codeSourceUnknown: setItemImage named a source that is no candidate of the title (9.5).
	codeSourceUnknown = "source_unknown"
)

// choosableKinds are the kinds the administrator can choose per item kind (7.3): the episode still
// is not chosen by hand (40'000 episodes; a refresh brings the provider's).
var choosableKinds = map[string][]string{
	"movie":  {"poster", "backdrop", "logo"},
	"series": {"poster", "backdrop", "logo"},
	"season": {"poster"},
}

type candidateEntry struct {
	list    model.ImageCandidateList
	expires time.Time
}

// candidateKey is the cache key of a title and kind.
func candidateKey(itemID, kind string) string { return itemID + "/" + kind }

// cachedCandidates returns the list of a title and kind while it is fresh.
func (s *Service) cachedCandidates(itemID, kind string) (model.ImageCandidateList, bool) {
	s.candMu.Lock()
	defer s.candMu.Unlock()
	e, ok := s.candCache[candidateKey(itemID, kind)]
	if !ok || s.now().After(e.expires) {
		return model.ImageCandidateList{}, false
	}
	return e.list, true
}

func (s *Service) storeCandidates(itemID, kind string, list model.ImageCandidateList) {
	s.candMu.Lock()
	defer s.candMu.Unlock()
	if s.candCache == nil {
		s.candCache = map[string]candidateEntry{}
	}
	now := s.now()
	if len(s.candCache) >= maxCandidateEntries {
		for k, e := range s.candCache {
			if now.After(e.expires) {
				delete(s.candCache, k)
			}
		}
		for len(s.candCache) >= maxCandidateEntries {
			oldest := ""
			for k, e := range s.candCache {
				if oldest == "" || e.expires.Before(s.candCache[oldest].expires) {
					oldest = k
				}
			}
			delete(s.candCache, oldest)
		}
	}
	s.candCache[candidateKey(itemID, kind)] = candidateEntry{list: list, expires: now.Add(candidateTTL)}
}

// choosable checks the kind for the item and returns the item (model.ErrNotFound,
// *model.ValidationError kind invalid_value).
func (s *Service) choosable(ctx context.Context, itemID, kind string) (systemq.Item, error) {
	item, err := s.liveItem(ctx, itemID)
	if err != nil {
		return systemq.Item{}, err
	}
	if !slices.Contains(choosableKinds[item.Kind], kind) {
		var v *model.ValidationError
		return systemq.Item{}, v.Add("kind", "invalid_value")
	}
	return item, nil
}

// ImageCandidates lists the candidates of every source for one kind of a title (6.4): the image
// files next to the media first, then the providers of the folder in list order (TMDB without
// language filter, TVDB artworks of a series); within a source by the kind's language chain,
// then votes and width. refresh skips the cache. A provider that fails is named in Unavailable
// instead of failing the list.
func (s *Service) ImageCandidates(ctx context.Context, itemID, kind string, refresh bool) (model.ImageCandidateList, error) {
	item, err := s.choosable(ctx, itemID, kind)
	if err != nil {
		return model.ImageCandidateList{}, err
	}
	if !refresh {
		if list, ok := s.cachedCandidates(item.ID, kind); ok {
			return s.markChosen(ctx, item.ID, kind, list)
		}
	}
	list, err := s.collectCandidates(ctx, item, kind)
	if err != nil {
		return model.ImageCandidateList{}, err
	}
	s.storeCandidates(item.ID, kind, list)
	return s.markChosen(ctx, item.ID, kind, list)
}

// markChosen sets Chosen on the candidate the title's item_images row names (a copy of list).
func (s *Service) markChosen(ctx context.Context, itemID, kind string, list model.ImageCandidateList) (model.ImageCandidateList, error) {
	rows, err := s.read.ListItemImagesByItem(ctx, itemID)
	if err != nil {
		return model.ImageCandidateList{}, fmt.Errorf("image candidates of %s: %w", itemID, err)
	}
	out := model.ImageCandidateList{Candidates: slices.Clone(list.Candidates), Unavailable: list.Unavailable}
	for _, r := range rows {
		if r.Kind != kind {
			continue
		}
		for i := range out.Candidates {
			c := &out.Candidates[i]
			c.Chosen = c.Provider == r.Provider && c.Source == r.Source
		}
	}
	return out, nil
}

// collectCandidates asks every source of the title for the kind.
func (s *Service) collectCandidates(ctx context.Context, item systemq.Item, kind string) (model.ImageCandidateList, error) {
	out := model.ImageCandidateList{Candidates: []model.ImageCandidate{}, Unavailable: []string{}}
	settings, err := s.Effective(ctx, item.LibraryID)
	if err != nil {
		return out, err
	}
	chain := settings.ImageLangs[kind]
	if kind != "poster" && kind != "backdrop" && kind != "logo" || len(chain) == 0 {
		chain = settings.ImageLangs["poster"]
	}

	local, err := s.read.ListItemLocalImages(ctx, item.ID)
	if err != nil {
		return out, fmt.Errorf("image candidates of %s: local files: %w", item.ID, err)
	}
	var locals []model.ImageCandidate
	for _, c := range localCandidates(local) {
		if c.Kind == kind {
			locals = append(locals, model.ImageCandidate{Provider: ImageSourceLocal, Width: c.Width, Height: c.Height, Source: c.Source})
		}
	}
	out.Candidates = append(out.Candidates, sortCandidates(locals, chain)...)

	series := item
	if item.Kind == "season" {
		if !item.ParentID.Valid {
			return out, nil
		}
		if series, err = s.read.GetItemSystem(ctx, item.ParentID.String); err != nil {
			return out, fmt.Errorf("image candidates of %s: series: %w", item.ID, err)
		}
	}
	order := model.ParseProviders(series.Providers)
	if len(order) == 0 {
		order = []string{model.ProviderTMDB}
	}
	for _, p := range order {
		var found []model.ImageCandidate
		var err error
		switch p {
		case model.ProviderTMDB:
			if !series.TmdbID.Valid || !s.tmdb.Configured(ctx) {
				continue
			}
			found, err = s.tmdbCandidates(ctx, item, series.TmdbID.Int64, kind)
		case model.ProviderTVDB:
			if !series.TvdbID.Valid || series.Kind != "series" || item.Kind == "season" || !s.tvdb.Configured(ctx) {
				continue
			}
			found, err = s.tvdbCandidates(ctx, series.TvdbID.Int64, kind)
		default:
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			s.log.Warn("image candidates: provider failed", "item_id", item.ID, "kind", kind, "provider", p, "err", err)
			out.Unavailable = append(out.Unavailable, p)
			continue
		}
		out.Candidates = append(out.Candidates, sortCandidates(found, chain)...)
	}
	return out, nil
}

// tmdbCandidates asks TMDB for every image of the title (all languages) and keeps the kind.
func (s *Service) tmdbCandidates(ctx context.Context, item systemq.Item, tmdbID int64, kind string) ([]model.ImageCandidate, error) {
	c, err := s.tmdb.keyed(ctx)
	if err != nil {
		return nil, err
	}
	p := "/movie/" + strconv.FormatInt(tmdbID, 10)
	switch item.Kind {
	case "series":
		p = "/tv/" + strconv.FormatInt(tmdbID, 10)
	case "season":
		p = "/tv/" + strconv.FormatInt(tmdbID, 10) + "/season/" + strconv.FormatInt(item.SeasonNumber.Int64, 10)
	}
	imgs, err := c.AllImages(ctx, p)
	if err != nil {
		return nil, err
	}
	var out []model.ImageCandidate
	for _, ic := range tmdbImageCandidates(imgs, nil) {
		if ic.Kind != kind {
			continue
		}
		preview := ic.Source
		if strings.HasSuffix(strings.ToLower(preview), ".svg") {
			preview = preview[:len(preview)-4] + ".png"
		}
		out = append(out, model.ImageCandidate{Provider: model.ProviderTMDB, Lang: ic.Lang, Width: ic.Width, Height: ic.Height,
			Score: ic.Votes, VoteCount: ic.VoteCount, Source: ic.Source, PreviewURL: s.client.imageURL(previewSize, preview)})
	}
	return out, nil
}

// tvdbCandidates reads the artworks of a series at TVDB and keeps the kind.
func (s *Service) tvdbCandidates(ctx context.Context, tvdbID int64, kind string) ([]model.ImageCandidate, error) {
	all, err := s.tvdb.Artworks(ctx, tvdbID)
	if err != nil {
		return nil, err
	}
	var out []model.ImageCandidate
	for _, ic := range all {
		if ic.Kind == kind {
			out = append(out, model.ImageCandidate{Provider: model.ProviderTVDB, Lang: ic.Lang, Width: ic.Width, Height: ic.Height,
				Score: ic.Votes, Source: ic.Source, PreviewURL: ic.Source})
		}
	}
	return out, nil
}

// sortCandidates orders the candidates of one source by the position of their language in the
// chain (languages outside it last), then votes, vote count and width, highest first.
func sortCandidates(list []model.ImageCandidate, chain []string) []model.ImageCandidate {
	rank := func(lang string) int {
		if i := slices.Index(chain, lang); i >= 0 {
			return i
		}
		return len(chain)
	}
	slices.SortStableFunc(list, func(a, b model.ImageCandidate) int {
		return cmp.Or(cmp.Compare(rank(a.Lang), rank(b.Lang)), cmp.Compare(b.Score, a.Score),
			cmp.Compare(b.VoteCount, a.VoteCount), cmp.Compare(b.Width, a.Width))
	})
	return list
}

// SetItemImage chooses a candidate for a kind of a title and locks the kind (setItemImage, 6.3):
// item_images gets the candidate with chosen = 1, locked_fields the kind, the provenance the
// source; image.fetch fetches it at the manual priority. The source must be a candidate of the
// title; an expired cache is refilled before a source counts as unknown (9.5). Errors:
// model.ErrNotFound, *model.ValidationError (kind invalid_value, source source_unknown).
func (s *Service) SetItemImage(ctx context.Context, itemID, kind, provider, source string) error {
	item, err := s.choosable(ctx, itemID, kind)
	if err != nil {
		return err
	}
	cand, ok := s.findCandidate(item.ID, kind, provider, source)
	if !ok {
		list, err := s.collectCandidates(ctx, item, kind)
		if err != nil {
			return err
		}
		s.storeCandidates(item.ID, kind, list)
		if cand, ok = s.findCandidate(item.ID, kind, provider, source); !ok {
			var v *model.ValidationError
			return v.Add("source", codeSourceUnknown)
		}
	}
	now := s.now().UnixMilli()
	err = s.inTx(ctx, func(q *systemq.Queries) error {
		if err := q.UpsertItemImage(ctx, systemq.UpsertItemImageParams{ItemID: item.ID, Kind: kind, Provider: cand.Provider,
			Lang: cand.Lang, Source: cand.Source, Chosen: 1, Now: now}); err != nil {
			return err
		}
		if err := q.SetItemLockedFields(ctx, systemq.SetItemLockedFieldsParams{ID: item.ID,
			LockedFields: addLocked(item.LockedFields, kind), Now: now}); err != nil {
			return err
		}
		return q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{ItemID: item.ID, Field: kind, Lang: "", Provider: cand.Provider})
	})
	if err != nil {
		return fmt.Errorf("set %s image of %s: %w", kind, item.ID, err)
	}
	s.log.Info("image chosen", "item_id", item.ID, "kind", kind, "provider", cand.Provider)
	return s.enqueueImages(ctx, item.ID, matchManualPriority)
}

// findCandidate looks a source up in the cached candidates of the title and kind.
func (s *Service) findCandidate(itemID, kind, provider, source string) (model.ImageCandidate, bool) {
	list, ok := s.cachedCandidates(itemID, kind)
	if !ok {
		return model.ImageCandidate{}, false
	}
	for _, c := range list.Candidates {
		if c.Provider == provider && c.Source == source {
			return c, true
		}
	}
	return model.ImageCandidate{}, false
}

// ClearItemImage lifts the choice and the lock of a kind (clearItemImage): the item_images row
// goes, the kind leaves locked_fields, and a manual match of the title (of the series for a
// season) decides the kind again; the match queues image.fetch (papier 14: the match, not fill,
// because fill of a title with one source decides no images). Errors: model.ErrNotFound,
// *model.ValidationError (kind invalid_value).
func (s *Service) ClearItemImage(ctx context.Context, itemID, kind string) error {
	item, err := s.choosable(ctx, itemID, kind)
	if err != nil {
		return err
	}
	now := s.now().UnixMilli()
	err = s.inTx(ctx, func(q *systemq.Queries) error {
		if err := q.DeleteItemImage(ctx, systemq.DeleteItemImageParams{ItemID: item.ID, Kind: kind}); err != nil {
			return err
		}
		return q.SetItemLockedFields(ctx, systemq.SetItemLockedFieldsParams{ID: item.ID,
			LockedFields: removeLocked(item.LockedFields, kind), Now: now})
	})
	if err != nil {
		return fmt.Errorf("clear %s image of %s: %w", kind, item.ID, err)
	}
	s.log.Info("image choice cleared", "item_id", item.ID, "kind", kind)
	target := item.ID
	if item.Kind == "season" && item.ParentID.Valid {
		target = item.ParentID.String
	}
	return s.enqueueManualMatch(ctx, target)
}

// removeLocked returns the locked_fields JSON without names.
func removeLocked(raw string, names ...string) string {
	var cur []string
	if err := json.Unmarshal([]byte(raw), &cur); err != nil {
		return raw
	}
	cur = slices.DeleteFunc(cur, func(n string) bool { return slices.Contains(names, n) })
	if cur == nil {
		cur = []string{}
	}
	out, err := json.Marshal(cur)
	if err != nil {
		return raw
	}
	return string(out)
}

// ItemImages returns the decided image source per choosable kind of a title for the title page
// (AdminItem.images): provider, language, source and preview of the item_images row, chosen and
// locked; a locked kind without a row is a "lock without choice" (an upload or an old lock).
func (s *Service) ItemImages(ctx context.Context, itemID string) ([]model.AdminItemImage, error) {
	item, err := s.liveItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.ListItemImagesByItem(ctx, item.ID)
	if err != nil {
		return nil, fmt.Errorf("item images of %s: %w", item.ID, err)
	}
	byKind := map[string]systemq.ItemImage{}
	for _, r := range rows {
		byKind[r.Kind] = r
	}
	locked := lockedFields(item.LockedFields)
	out := []model.AdminItemImage{}
	for _, kind := range choosableKinds[item.Kind] {
		r, ok := byKind[kind]
		if !ok && !locked[kind] {
			continue
		}
		img := model.AdminItemImage{Kind: kind, Locked: locked[kind]}
		if ok {
			img.Provider, img.Lang, img.Source, img.Chosen = r.Provider, r.Lang, r.Source, r.Chosen == 1
			img.PreviewURL = s.previewOf(r.Provider, r.Source)
		}
		out = append(out, img)
	}
	return out, nil
}

// previewOf is the provider's preview address of a source ("" for a local file: the API adds the
// address of the preview handler).
func (s *Service) previewOf(provider, source string) string {
	switch {
	case provider == ImageSourceLocal || strings.HasPrefix(source, coverFilePrefix):
		return ""
	case strings.HasPrefix(source, "/"):
		if strings.HasSuffix(strings.ToLower(source), ".svg") {
			source = source[:len(source)-4] + ".png"
		}
		return s.client.imageURL(previewSize, source)
	default:
		return source
	}
}

// LocalImagePreview opens a local candidate of a title for the preview handler
// (getItemImageCandidatePreview, 9.2): only a source the scanner listed for the title is served,
// read through package paths below its root, at most 20 MB; anything else is model.ErrNotFound.
func (s *Service) LocalImagePreview(ctx context.Context, itemID, source string) (model.StreamSource, error) {
	item, err := s.liveItem(ctx, itemID)
	if err != nil {
		return model.StreamSource{}, err
	}
	rows, err := s.read.ListItemLocalImages(ctx, item.ID)
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("local preview of %s: %w", item.ID, err)
	}
	var row *systemq.ItemLocalImage
	for i := range rows {
		if localImageSource(rows[i].RelPath, rows[i].Size, rows[i].Mtime) == source {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		return model.StreamSource{}, model.ErrNotFound
	}
	root, err := s.localImageRoot(ctx, item.ID, row.RelPath)
	if err != nil {
		return model.StreamSource{}, err
	}
	if root == nil {
		return model.StreamSource{}, model.ErrNotFound
	}
	defer root.Close()
	f, err := root.Open(row.RelPath)
	if errors.Is(err, fs.ErrNotExist) {
		return model.StreamSource{}, model.ErrNotFound
	}
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("local preview of %s: %w", item.ID, err)
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxLocalPreviewBytes {
		_ = f.Close()
		return model.StreamSource{}, model.ErrNotFound
	}
	mediaType := map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}[strings.ToLower(path.Ext(row.RelPath))]
	if mediaType == "" {
		_ = f.Close()
		return model.StreamSource{}, model.ErrNotFound
	}
	return model.StreamSource{File: f, MediaType: mediaType, Size: fi.Size(), ModTime: fi.ModTime(), Name: "local image preview"}, nil
}
