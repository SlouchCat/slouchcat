package metadata

import (
	"context"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
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
// returns the stored view; nothing is queued (F-8).
func (s *Service) UpdateMetadataSettings(ctx context.Context, m model.MetadataSettings) (model.MetadataSettings, error) {
	if err := s.UpdateSettings(ctx, settingsFromModel(m)); err != nil {
		return model.MetadataSettings{}, err
	}
	return s.MetadataSettings(ctx)
}

// LibraryMetadata returns the deviation of a library with its effective values; model.ErrNotFound
// for an unknown library.
func (s *Service) LibraryMetadata(ctx context.Context, libraryID int64) (model.LibraryMetadata, error) {
	if _, err := s.applyLibraries(ctx, []int64{libraryID}); err != nil {
		return model.LibraryMetadata{}, err
	}
	d, err := s.LibraryDeviation(ctx, libraryID)
	if err != nil {
		return model.LibraryMetadata{}, err
	}
	st, err := s.Settings(ctx)
	if err != nil {
		return model.LibraryMetadata{}, err
	}
	out := model.LibraryMetadata{LibraryID: libraryID, TextLangs: d.TextLangs}
	if d.Images != nil {
		out.Images = &model.LibraryImages{Langs: d.Images.Langs, Kinds: nonNil(d.Images.Kinds)}
	}
	eff, err := s.withCounts(ctx, d.apply(st).toModel(), []int64{libraryID})
	if err != nil {
		return model.LibraryMetadata{}, err
	}
	out.Effective, out.AffectedTitles = eff, eff.AffectedTitles
	return out, nil
}

// UpdateLibraryMetadata stores the deviation of a library (model.ErrNotFound,
// *model.ValidationError) and returns the view.
func (s *Service) UpdateLibraryMetadata(ctx context.Context, libraryID int64, textLangs *[]string, images *model.LibraryImages) (model.LibraryMetadata, error) {
	if _, err := s.applyLibraries(ctx, []int64{libraryID}); err != nil {
		return model.LibraryMetadata{}, err
	}
	d := LibraryDeviation{TextLangs: textLangs}
	if images != nil {
		d.Images = &LibraryImages{Langs: images.Langs, Kinds: images.Kinds}
	}
	if err := s.UpdateLibraryDeviation(ctx, libraryID, d); err != nil {
		return model.LibraryMetadata{}, err
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
