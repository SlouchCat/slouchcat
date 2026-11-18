package metadata

import (
	"context"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The administration's view of the metadata settings (getMetadataSettings, updateMetadataSettings;
// concept 10 5.1, ADR 0018), on top of Settings and ApplyMetadata. A folder's own languages and
// images are part of the folder (saveLibrary, concept 24, ADR 0036).

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
	return s.withCounts(ctx, st.toModel())
}

// withCounts adds the affected titles of every folder and the titles with local image files.
func (s *Service) withCounts(ctx context.Context, m model.MetadataSettings) (model.MetadataSettings, error) {
	n, err := s.CountApplyTitles(ctx, nil)
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
		// The titles outside the library keep their certificates: their ages follow at once.
		if err := s.recomputeExternalAges(ctx, m.RatingCountry); err != nil {
			return model.MetadataSettings{}, err
		}
		_, err = s.QueueRatingsReread(ctx)
	} else {
		_, err = s.QueueRatingsRereadPending(ctx)
	}
	if err != nil {
		return model.MetadataSettings{}, err
	}
	return s.MetadataSettings(ctx)
}

// Apply is ApplyMetadata for the administration (*model.ValidationError).
func (s *Service) Apply(ctx context.Context, req model.ApplyMetadataRequest) (model.ApplyMetadataResult, error) {
	id, n, err := s.ApplyMetadata(ctx, ApplyRequest{RootIDs: req.RootIDs, What: req.What})
	if err != nil {
		return model.ApplyMetadataResult{}, err
	}
	return model.ApplyMetadataResult{JobID: id, Titles: n}, nil
}
