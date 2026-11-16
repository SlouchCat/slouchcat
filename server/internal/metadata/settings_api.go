package metadata

import (
	"context"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The administration's view of the metadata settings (getMetadataSettings, updateMetadataSettings,
// getLibraryMetadata, updateLibraryMetadata; concept 10 5.1, ADR 0018), on top of Settings,
// LibraryDeviation and ApplyMetadata.

// toModel maps Settings onto the administration's view (without the counts).
func (s Settings) toModel() model.MetadataSettings {
	out := model.MetadataSettings{
		TextLangs: slices.Clone(s.TextLangs), ImageLangs: map[string][]string{}, ImageKinds: slices.Clone(nonNil(s.ImageKinds)),
		ImageMinWidth: map[string]int{}, ImageByVote: s.ImageByVote, LocalImages: s.LocalImages, RatingCountry: s.RatingCountry,
		TMDBIncludeAdult: s.TMDBIncludeAdult, FanartEnabled: s.FanartEnabled, KnownLanguages: KnownLanguages(),
		RatingSystems: model.RatingSystems(),
	}
	for _, k := range ChainImageKinds {
		out.ImageLangs[k] = slices.Clone(nonNil(s.ImageLangs[k]))
	}
	for _, k := range MinWidthImageKinds {
		out.ImageMinWidth[k] = s.ImageMinWidth[k]
	}
	return out
}

// settingsFromModel maps the administration's update onto Settings.
func settingsFromModel(m model.MetadataSettings) Settings {
	st := Settings{
		TextLangs: slices.Clone(m.TextLangs), ImageLangs: map[string][]string{}, ImageKinds: slices.Clone(nonNil(m.ImageKinds)),
		ImageMinWidth: map[string]int{}, ImageByVote: m.ImageByVote, LocalImages: m.LocalImages, RatingCountry: m.RatingCountry,
		TMDBIncludeAdult: m.TMDBIncludeAdult, FanartEnabled: m.FanartEnabled,
	}
	for _, k := range ChainImageKinds {
		st.ImageLangs[k] = slices.Clone(nonNil(m.ImageLangs[k]))
	}
	for _, k := range MinWidthImageKinds {
		st.ImageMinWidth[k] = m.ImageMinWidth[k]
	}
	return st
}

// MetadataSettings returns the server's settings with the counts of the "Anwenden" section.
func (s *Service) MetadataSettings(ctx context.Context) (model.MetadataSettings, error) {
	st, err := s.Settings(ctx)
	if err != nil {
		return model.MetadataSettings{}, err
	}
	return s.withCounts(ctx, st.toModel(), nil)
}

// withCounts adds the affected titles (of the libraries, or of every library) and the titles with
// local image files.
func (s *Service) withCounts(ctx context.Context, m model.MetadataSettings, libraryIDs []int64) (model.MetadataSettings, error) {
	n, err := s.CountApplyTitles(ctx, libraryIDs)
	if err != nil {
		return m, err
	}
	m.AffectedTitles = n
	local, err := s.read.CountItemsWithLocalImages(ctx)
	if err != nil {
		return m, fmt.Errorf("count titles with local images: %w", err)
	}
	m.LocalImageTitles = int(local)
	return m, nil
}

// UpdateMetadataSettings validates and stores the server's settings (*model.ValidationError) and
// returns the stored view. Nothing is queued (F-8), except for the rating country: after a change
// metadata.ratings_reread reads every rating anew (concept 22 F-16), when there are titles. A
// chain that could not be queued is not lost: the titles count as unread for the new country
// (SettingRatingsRereadDone) until a chain finishes, so the next save or the next start queues
// it (QueueRatingsRereadPending).
func (s *Service) UpdateMetadataSettings(ctx context.Context, m model.MetadataSettings) (model.MetadataSettings, error) {
	before, err := s.Settings(ctx)
	if err != nil {
		return model.MetadataSettings{}, err
	}
	if err := s.UpdateSettings(ctx, settingsFromModel(m)); err != nil {
		return model.MetadataSettings{}, err
	}
	if before.RatingCountry != m.RatingCountry {
		_, err = s.QueueRatingsReread(ctx)
	} else {
		_, err = s.QueueRatingsRereadPending(ctx)
	}
	if err != nil {
		return model.MetadataSettings{}, err
	}
	return s.MetadataSettings(ctx)
}

// LibraryMetadata returns what the video folders of a library set themselves, read from the first
// of them (migration 00065 copied a library's deviation onto each; updateLibraryMetadata writes
// all of them), with its effective values; model.ErrNotFound for an unknown library. The operation
// leaves with contract 1.40.0 (concept 24 3.6).
func (s *Service) LibraryMetadata(ctx context.Context, libraryID int64) (model.LibraryMetadata, error) {
	if _, err := s.applyLibraries(ctx, []int64{libraryID}); err != nil {
		return model.LibraryMetadata{}, err
	}
	roots, err := appq.New(s.db.Read).ListLibraryRoots(ctx, libraryID)
	if err != nil {
		return model.LibraryMetadata{}, fmt.Errorf("roots of library %d: %w", libraryID, err)
	}
	var d model.RootMetadata
	for _, r := range roots {
		if isVideoContent(r.Content) {
			d = s.rootSettings(ctx, r.ID)
			break
		}
	}
	st, err := s.Settings(ctx)
	if err != nil {
		return model.LibraryMetadata{}, err
	}
	out := model.LibraryMetadata{LibraryID: libraryID, TextLangs: d.TextLangs}
	if d.Images != nil {
		out.Images = &model.LibraryImages{Langs: d.Images.Langs, Kinds: nonNil(d.Images.Kinds)}
	}
	eff, err := s.withCounts(ctx, applyRoot(st, d).toModel(), []int64{libraryID})
	if err != nil {
		return model.LibraryMetadata{}, err
	}
	out.Effective, out.AffectedTitles = eff, eff.AffectedTitles
	return out, nil
}

// UpdateLibraryMetadata writes the same own settings onto every video folder of a library
// (model.ErrNotFound, *model.ValidationError) and returns the view.
func (s *Service) UpdateLibraryMetadata(ctx context.Context, libraryID int64, textLangs *[]string, images *model.LibraryImages) (model.LibraryMetadata, error) {
	if _, err := s.applyLibraries(ctx, []int64{libraryID}); err != nil {
		return model.LibraryMetadata{}, err
	}
	d := model.RootMetadata{TextLangs: textLangs}
	if images != nil {
		d.Images = &model.RootImages{Langs: images.Langs, Kinds: images.Kinds}
	}
	if verr := ValidateRootMetadata(d); verr != nil {
		return model.LibraryMetadata{}, verr
	}
	roots, err := appq.New(s.db.Read).ListLibraryRoots(ctx, libraryID)
	if err != nil {
		return model.LibraryMetadata{}, fmt.Errorf("roots of library %d: %w", libraryID, err)
	}
	tl, im := model.RootMetadataColumns(d)
	for _, r := range roots {
		if !isVideoContent(r.Content) {
			continue
		}
		if err := appq.New(s.db.Write).UpdateLibraryRootMetadata(ctx, appq.UpdateLibraryRootMetadataParams{TextLangs: tl, Images: im, ID: r.ID, LibraryID: libraryID}); err != nil {
			return model.LibraryMetadata{}, fmt.Errorf("write folder metadata %d: %w", r.ID, err)
		}
	}
	return s.LibraryMetadata(ctx, libraryID)
}

// Apply is ApplyMetadata for the administration (*model.ValidationError).
func (s *Service) Apply(ctx context.Context, req model.ApplyMetadataRequest) (model.ApplyMetadataResult, error) {
	id, n, err := s.ApplyMetadata(ctx, ApplyRequest{LibraryIDs: req.LibraryIDs, What: req.What})
	if err != nil {
		return model.ApplyMetadataResult{}, err
	}
	return model.ApplyMetadataResult{JobID: id, Titles: n}, nil
}
