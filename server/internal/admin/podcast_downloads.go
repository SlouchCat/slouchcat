package admin

import (
	"context"
	"errors"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Stored podcast episodes (concept 19 part A, section 11; ADR 0029): the setting of a podcast
// library and its folder check, the estimate before a setting that loads once, and the buttons
// that load episodes are package library's; the own setting of a podcast is package metadata's,
// which owns the subscription.

// PodcastStorage is implemented by package library.
type PodcastStorage interface {
	// PodcastFolderOf runs the folder check of the folder for new subscriptions of a podcast
	// library (model.ErrNotFound, model.ErrRootRequired).
	PodcastFolderOf(ctx context.Context, libraryID int64) (model.PodcastFolder, error)
	// SetPodcastDownloads stores the setting of a podcast library and runs the planner
	// (model.ErrNotFound, *model.ValidationError, model.ErrRootRequired, model.ErrRootNotWritable).
	SetPodcastDownloads(ctx context.Context, libraryID int64, d model.PodcastDownloads) (model.PodcastDownloads, error)
	// EstimatePodcastDownloads answers what an imagined setting would load once (model.ErrNotFound).
	EstimatePodcastDownloads(ctx context.Context, libraryID int64, req model.PodcastDownloadEstimateRequest) (model.PodcastDownloadEstimate, error)
	// DownloadPodcastEpisodes queues the episodes without file of a podcast, or the given ones,
	// and returns how many (model.ErrNotFound).
	DownloadPodcastEpisodes(ctx context.Context, podcastID string, episodeIDs []string) (int, error)
}

// WithPodcastStorage sets the stored podcast episodes of package library and returns the receiver.
func (s *Service) WithPodcastStorage(p PodcastStorage) *Service {
	s.podcastStorage = p
	return s
}

// UpdatePodcast stores the own setting of stored episodes of a podcast (updatePodcast).
func (s *Service) UpdatePodcast(ctx context.Context, id string, upd model.PodcastDownloadsUpdate) (model.AdminPodcast, error) {
	if s.podcasts == nil {
		return model.AdminPodcast{}, ErrNotConfigured
	}
	return s.podcasts.UpdatePodcastDownloads(ctx, id, upd)
}

// DownloadPodcastEpisodes queues the episodes without file of a podcast, or the given ones
// (downloadPodcastEpisodes).
func (s *Service) DownloadPodcastEpisodes(ctx context.Context, podcastID string, episodeIDs []string) (int, error) {
	if s.podcastStorage == nil {
		return 0, ErrNotConfigured
	}
	return s.podcastStorage.DownloadPodcastEpisodes(ctx, podcastID, episodeIDs)
}

// EstimatePodcastDownloads answers what an imagined setting of a podcast library or of one of its
// podcasts would load once (estimatePodcastDownloads). Errors: *model.ValidationError (mode,
// scope invalid, inherit only for a podcast; lastN out_of_range), model.ErrNotFound.
func (s *Service) EstimatePodcastDownloads(ctx context.Context, libraryID int64, req model.PodcastDownloadEstimateRequest) (model.PodcastDownloadEstimate, error) {
	var verr *model.ValidationError
	podcast := req.PodcastID != ""
	switch req.Mode {
	case model.PodcastSave, model.PodcastLink:
	case model.PodcastInherit:
		if !podcast {
			verr = verr.Add("mode", "invalid")
		}
	default:
		verr = verr.Add("mode", "invalid")
	}
	switch req.Scope {
	case model.PodcastScopeNew, model.PodcastScopeLastN, model.PodcastScopeAll:
	case model.PodcastInherit:
		if !podcast {
			verr = verr.Add("scope", "invalid")
		}
	default:
		verr = verr.Add("scope", "invalid")
	}
	if req.LastN != 0 && (req.LastN < 1 || req.LastN > model.PodcastLastNMax) {
		verr = verr.Add("lastN", "out_of_range")
	}
	if verr != nil {
		return model.PodcastDownloadEstimate{}, verr
	}
	if s.podcastStorage == nil {
		return model.PodcastDownloadEstimate{}, ErrNotConfigured
	}
	if req.LastN == 0 && req.Scope == model.PodcastScopeLastN {
		d, err := s.libraryPodcastDownloads(ctx, libraryID)
		if err != nil {
			return model.PodcastDownloadEstimate{}, err
		}
		req.LastN = d.LastN
	}
	return s.podcastStorage.EstimatePodcastDownloads(ctx, libraryID, req)
}

// libraryPodcastDownloads is the stored setting of a podcast library (model.ErrNotFound for
// another kind).
func (s *Service) libraryPodcastDownloads(ctx context.Context, libraryID int64) (model.PodcastDownloads, error) {
	lib, err := s.libraries.Get(ctx, libraryID)
	if err != nil {
		return model.PodcastDownloads{}, err
	}
	if lib.PodcastDownloads == nil {
		return model.PodcastDownloads{}, model.ErrNotFound
	}
	return *lib.PodcastDownloads, nil
}

// withPodcastFolder adds the folder check to a podcast library (getLibrary); a library without
// a podcast folder has none.
func (s *Service) withPodcastFolder(ctx context.Context, lib model.Library) (model.Library, error) {
	if lib.Kind != model.LibraryPodcasts || s.podcastStorage == nil {
		return lib, nil
	}
	folder, err := s.podcastStorage.PodcastFolderOf(ctx, lib.ID)
	switch {
	case errors.Is(err, model.ErrRootRequired):
		return lib, nil
	case err != nil:
		return model.Library{}, err
	}
	lib.PodcastFolder = &folder
	return lib, nil
}

// setPodcastDownloads stores the setting of stored episodes of a podcast library (updateLibrary);
// the field errors of package library carry the prefix podcastDownloads.
func (s *Service) setPodcastDownloads(ctx context.Context, libraryID int64, d model.PodcastDownloads) error {
	if s.podcastStorage == nil {
		return ErrNotConfigured
	}
	_, err := s.podcastStorage.SetPodcastDownloads(ctx, libraryID, d)
	var verr *model.ValidationError
	if errors.As(err, &verr) {
		out := &model.ValidationError{}
		for _, f := range verr.Fields {
			out = out.Add("podcastDownloads."+f.Field, f.Code)
		}
		return out
	}
	return err
}
