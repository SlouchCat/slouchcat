package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// metadata.apply (concept 10 section 6.5, ADR 0018): the event job that brings existing titles
// up to the current metadata settings. The administrator queues it with applyMetadata ("Jetzt
// anwenden", F-8); saving a setting never queues it. The handler walks the matched films and
// series of the chosen libraries and queues one child per title:
//
//   - what contains "texts": metadata.match (a new fetch as refreshItem does, without the probes),
//     because only the match asks the providers per language and a folder with one provider runs
//     no fill request (1.1); the match queues fill and image.fetch as always.
//   - what is ["images"]: metadata.fill with the images flag (fillImages: it fetches the records
//     even for a folder with one provider, where the ordinary fill makes no request) and
//     image.fetch, which decide the image fields again from the candidates.
//
// Children carry the idempotency keys of their kind (match:<id>, fill:<id>, image:<id>), so a
// second apply does not double what still waits (RaiseQueuedPriority lifts a waiting child to the
// apply's priority). The parent run is done after seconds and reports "n children queued"; the
// children show their own progress on the jobs page (listJobTypes). There is no schedule (E-7).
const (
	JobApply      = "metadata.apply"
	applyPriority = 1 // behind the interactive jobs, before the bulk of the fillers

	ApplyTexts  = "texts"
	ApplyImages = "images"
)

// applyPayload is the payload of metadata.apply.
type applyPayload struct {
	LibraryIDs []int64  `json:"libraryIds,omitempty"`
	What       []string `json:"what"`
}

// ApplyRequest is what applyMetadata queues.
type ApplyRequest struct {
	LibraryIDs []int64  // nil or empty = every library
	What       []string // texts, images
}

// ApplyMetadata validates the request, counts the affected titles and queues one metadata.apply
// run (idempotent per scope while it waits). It returns the job id and the number of titles.
func (s *Service) ApplyMetadata(ctx context.Context, req ApplyRequest) (int64, int, error) {
	var verr *model.ValidationError
	if len(req.What) == 0 {
		verr = verr.Add("what", "required")
	}
	for _, w := range req.What {
		if w != ApplyTexts && w != ApplyImages {
			verr = verr.Add("what", "invalid_value")
			break
		}
	}
	libs, err := s.applyLibraries(ctx, req.LibraryIDs)
	if errors.Is(err, model.ErrNotFound) {
		verr = verr.Add("libraryIds", "unknown_library")
	} else if err != nil {
		return 0, 0, err
	}
	if verr != nil {
		return 0, 0, verr
	}
	titles, err := s.CountApplyTitles(ctx, libs)
	if err != nil {
		return 0, 0, err
	}
	what := slices.Clone(req.What)
	sort.Strings(what)
	payload, err := json.Marshal(applyPayload{LibraryIDs: req.LibraryIDs, What: what})
	if err != nil {
		return 0, 0, fmt.Errorf("%s payload: %w", JobApply, err)
	}
	keyLibs := make([]string, 0, len(req.LibraryIDs))
	for _, id := range req.LibraryIDs {
		keyLibs = append(keyLibs, strconv.FormatInt(id, 10))
	}
	sort.Strings(keyLibs)
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobApply, Priority: applyPriority, Payload: payload,
		IdempotencyKey: "apply:" + strings.Join(what, "+") + ":" + strings.Join(keyLibs, ","),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("enqueue %s: %w", JobApply, err)
	}
	return id, titles, nil
}

// applyLibraries resolves the libraries of a request: the given ids (model.ErrNotFound when one
// is unknown), or every library.
func (s *Service) applyLibraries(ctx context.Context, ids []int64) ([]int64, error) {
	all, err := s.read.ListLibraryIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	if len(ids) == 0 {
		return all, nil
	}
	for _, id := range ids {
		if !slices.Contains(all, id) {
			return nil, model.ErrNotFound
		}
	}
	return ids, nil
}

// CountApplyTitles counts the matched films and series of the libraries (every library when
// libraryIDs is empty): what an apply would queue, the number the settings pages show (F-8).
func (s *Service) CountApplyTitles(ctx context.Context, libraryIDs []int64) (int, error) {
	if len(libraryIDs) == 0 {
		var err error
		if libraryIDs, err = s.read.ListLibraryIDs(ctx); err != nil {
			return 0, fmt.Errorf("list libraries: %w", err)
		}
	}
	total := 0
	for _, id := range libraryIDs {
		n, err := s.read.CountLibraryItemsForApply(ctx, id)
		if err != nil {
			return 0, fmt.Errorf("count titles of library %d: %w", id, err)
		}
		total += int(n)
	}
	return total, nil
}

func (s *Service) handleApply(ctx context.Context, j *jobs.Job) error {
	var p applyPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("%s payload: %w", JobApply, err)
	}
	libs, err := s.applyLibraries(ctx, p.LibraryIDs)
	if errors.Is(err, model.ErrNotFound) {
		return nil // a library deleted since the request: nothing to do
	}
	if err != nil {
		return err
	}
	texts := slices.Contains(p.What, ApplyTexts)
	total, err := s.CountApplyTitles(ctx, libs)
	if err != nil {
		return err
	}
	done := 0
	for _, lib := range libs {
		items, err := s.read.ListApplyItems(ctx, lib)
		if err != nil {
			return fmt.Errorf("%s: list titles of library %d: %w", JobApply, lib, err)
		}
		for _, it := range items {
			if err := ctx.Err(); err != nil {
				return err
			}
			if texts {
				if err := s.enqueueRaised(ctx, JobMatch, "match:", it.ID, applyPriority, false); err != nil {
					return err
				}
			} else {
				if err := s.enqueueRaised(ctx, JobFill, "fill:images:", it.ID, applyPriority, true); err != nil {
					return err
				}
				if err := s.enqueueRaised(ctx, JobImageFetch, "image:", it.ID, applyPriority, false); err != nil {
					return err
				}
			}
			done++
			jobs.Progress(ctx, int64(done), int64(total), fmt.Sprintf("%d Titel eingereiht", done))
		}
	}
	s.log.Info("metadata apply queued its children", "libraries", len(libs), "titles", done, "what", p.What)
	return nil
}

// enqueueRaised queues a child job with its idempotency key and lifts a waiting one of the same
// key to prio (RaiseQueuedPriority, decision of 26 October 2026): a second apply doubles nothing.
// images marks the fill of an images apply (itemPayload.Images); its key "fill:images:<id>" keeps
// it apart from the fill a match queues, which would otherwise keep its payload without the flag.
func (s *Service) enqueueRaised(ctx context.Context, jobType, keyPrefix, itemID string, prio int, images bool) error {
	payload, err := json.Marshal(itemPayload{ItemID: itemID, Images: images})
	if err != nil {
		return fmt.Errorf("%s payload: %w", jobType, err)
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: jobType, Priority: prio, IdempotencyKey: keyPrefix + itemID, Payload: payload})
	if err != nil {
		return fmt.Errorf("enqueue %s %s: %w", jobType, itemID, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, prio); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("raise %s %s: %w", jobType, itemID, err)
	}
	return nil
}
