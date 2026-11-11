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
	// CheckPodcastDownloads answers whether the setting of a podcast library is taken, with its
	// errors (model.ErrNotFound, *model.ValidationError, model.ErrRootRequired,
	// model.ErrRootNotWritable), folder check included, and stores nothing.
	CheckPodcastDownloads(ctx context.Context, libraryID int64, d model.PodcastDownloads) error
	// StorePodcastDownloads stores a setting CheckPodcastDownloads took and runs the planner; it
	// runs no second folder check, so it fails only on an error of the database or a change of the
	// library in between.
	StorePodcastDownloads(ctx context.Context, libraryID int64, d model.PodcastDownloads) (model.PodcastDownloads, error)
	// EstimatePodcastDownloads answers what an imagined setting would load once (model.ErrNotFound).
	EstimatePodcastDownloads(ctx context.Context, libraryID int64, req model.PodcastDownloadEstimateRequest) (model.PodcastDownloadEstimate, error)
	// DownloadPodcastEpisodes queues the episodes without file of a podcast, or the given ones,
	// and returns how many (model.ErrNotFound).
	DownloadPodcastEpisodes(ctx context.Context, podcastID string, episodeIDs []string) (int, error)
	// PodcastEpisodeList returns one page of the episodes of a podcast with their state (the page
	// «Podcast»; model.ErrNotFound, *model.ValidationError).
	PodcastEpisodeList(ctx context.Context, podcastID, filter string, offset, limit int) (model.AdminPodcastEpisodePage, error)
	// CancelPodcastDownload cancels the download of an episode (model.ErrNotFound).
	CancelPodcastDownload(ctx context.Context, podcastID, episodeID string) error
	// PodcastDownloadQueue returns the downloads that run or wait (the section «Downloads»).
	PodcastDownloadQueue(ctx context.Context, limit int) (model.PodcastDownloadPage, error)
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

// PodcastEpisodes returns one page of the episodes of a podcast with the state of their file
// (listAdminPodcastEpisodes).
func (s *Service) PodcastEpisodes(ctx context.Context, podcastID, filter string, offset, limit int) (model.AdminPodcastEpisodePage, error) {
	if s.podcastStorage == nil {
		return model.AdminPodcastEpisodePage{}, ErrNotConfigured
	}
	return s.podcastStorage.PodcastEpisodeList(ctx, podcastID, filter, offset, limit)
}

// CancelPodcastDownload cancels the download of an episode (cancelPodcastDownload).
func (s *Service) CancelPodcastDownload(ctx context.Context, podcastID, episodeID string) error {
	if s.podcastStorage == nil {
		return ErrNotConfigured
	}
	return s.podcastStorage.CancelPodcastDownload(ctx, podcastID, episodeID)
}

// PodcastDownloadQueue returns the downloads of podcast episodes that run or wait
// (listPodcastDownloads).
func (s *Service) PodcastDownloadQueue(ctx context.Context, limit int) (model.PodcastDownloadPage, error) {
	if s.podcastStorage == nil {
		return model.PodcastDownloadPage{}, ErrNotConfigured
	}
	return s.podcastStorage.PodcastDownloadQueue(ctx, limit)
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

// checkPodcastDownloads asks whether package library takes the setting of stored episodes of a
// podcast library (updateLibrary, before its transaction), setPodcastDownloads stores it (after
// the commit); the field errors of package library carry the prefix podcastDownloads.
func (s *Service) checkPodcastDownloads(ctx context.Context, libraryID int64, d model.PodcastDownloads) error {
	if s.podcastStorage == nil {
		return ErrNotConfigured
	}
	return prefixPodcastFields(s.podcastStorage.CheckPodcastDownloads(ctx, libraryID, d))
}

func (s *Service) setPodcastDownloads(ctx context.Context, libraryID int64, d model.PodcastDownloads) error {
	if s.podcastStorage == nil {
		return ErrNotConfigured
	}
	_, err := s.podcastStorage.StorePodcastDownloads(ctx, libraryID, d)
	return prefixPodcastFields(err)
}

// prefixPodcastFields puts the prefix podcastDownloads. on the field errors of package library.
func prefixPodcastFields(err error) error {
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
