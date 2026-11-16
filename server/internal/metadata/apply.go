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
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// metadata.apply (concept 10 section 6.5, ADR 0018): the event job that brings existing titles
// up to the current metadata settings. The administrator queues it with applyMetadata ("Jetzt
// anwenden", F-8); saving a setting never queues it. The handler walks the matched films and
// series whose home folder is one of the chosen folders (concept 24 3.3, ADR 0036) and queues one
// child per title:
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

// applyPayload is the payload of metadata.apply. RootIDs names the folders (empty = every folder).
// LibraryIDs is the scope of a run queued before contract 1.40.0: handleApply turns it into the
// folders of those libraries and never reads it as "every folder".
type applyPayload struct {
	RootIDs    []int64  `json:"rootIds,omitempty"`
	LibraryIDs []int64  `json:"libraryIds,omitempty"`
	What       []string `json:"what"`
}

// ApplyRequest is what applyMetadata queues.
type ApplyRequest struct {
	RootIDs []int64  // nil or empty = every folder
	What    []string // texts, images
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
	roots, err := s.applyRoots(ctx, req.RootIDs)
	if errors.Is(err, model.ErrNotFound) {
		verr = verr.Add("rootIds", "unknown_root")
	} else if err != nil {
		return 0, 0, err
	}
	if verr != nil {
		return 0, 0, verr
	}
	titles, err := s.CountApplyTitles(ctx, roots)
	if err != nil {
		return 0, 0, err
	}
	what := slices.Clone(req.What)
	sort.Strings(what)
	scope := slices.Clone(req.RootIDs)
	slices.Sort(scope)
	scope = slices.Compact(scope)
	payload, err := json.Marshal(applyPayload{RootIDs: scope, What: what})
	if err != nil {
		return 0, 0, fmt.Errorf("%s payload: %w", JobApply, err)
	}
	keyRoots := make([]string, 0, len(scope))
	for _, id := range scope {
		keyRoots = append(keyRoots, strconv.FormatInt(id, 10))
	}
	// "roots:" keeps the key apart from a waiting run of contract 1.38.0, whose key named libraries.
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobApply, Priority: applyPriority, Payload: payload,
		IdempotencyKey: "apply:" + strings.Join(what, "+") + ":roots:" + strings.Join(keyRoots, ","),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("enqueue %s: %w", JobApply, err)
	}
	return id, titles, nil
}

// applyRoots checks the folders of a request: the given ids (model.ErrNotFound when one is
// unknown); nil for every title.
func (s *Service) applyRoots(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	all, err := s.read.ListRootIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	for _, id := range ids {
		if !slices.Contains(all, id) {
			return nil, model.ErrNotFound
		}
	}
	return ids, nil
}

// CountApplyTitles counts the matched films and series whose home folder is one of the folders;
// with rootIDs empty every one, also those without a home folder: what an apply would queue, the
// number the settings page shows (F-8).
func (s *Service) CountApplyTitles(ctx context.Context, rootIDs []int64) (int, error) {
	if len(rootIDs) == 0 {
		n, err := s.read.CountAllItemsForApply(ctx)
		if err != nil {
			return 0, fmt.Errorf("count titles: %w", err)
		}
		return int(n), nil
	}
	total := 0
	for _, id := range rootIDs {
		n, err := s.read.CountRootItemsForApply(ctx, id)
		if err != nil {
			return 0, fmt.Errorf("count titles of folder %d: %w", id, err)
		}
		total += int(n)
	}
	return total, nil
}

// runRoots resolves the folders of a queued run: nil and true for every title. A run of contract
// 1.38.0 names libraries: their folders now, none when they have none or are gone (never "every
// folder"). Folders removed since the request drop out; when none is left there is nothing to do
// (nil, false).
func (s *Service) runRoots(ctx context.Context, p applyPayload) ([]int64, bool, error) {
	ids := p.RootIDs
	if len(ids) == 0 && len(p.LibraryIDs) > 0 {
		var err error
		if ids, err = s.read.ListRootIDsOfLibraries(ctx, p.LibraryIDs); err != nil {
			return nil, false, fmt.Errorf("folders of libraries %v: %w", p.LibraryIDs, err)
		}
		if len(ids) == 0 {
			return nil, false, nil
		}
	}
	if len(ids) == 0 {
		return nil, true, nil
	}
	all, err := s.read.ListRootIDs(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("list folders: %w", err)
	}
	known := make([]int64, 0, len(ids))
	for _, id := range ids {
		if slices.Contains(all, id) {
			known = append(known, id)
		}
	}
	return known, len(known) > 0, nil
}

func (s *Service) handleApply(ctx context.Context, j *jobs.Job) error {
	var p applyPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("%s payload: %w", JobApply, err)
	}
	roots, ok, err := s.runRoots(ctx, p)
	if err != nil {
		return err
	}
	if !ok {
		s.log.Info("metadata apply found none of its folders", "roots", p.RootIDs, "libraries", p.LibraryIDs)
		return nil // the folders or libraries went since the request: nothing to do
	}
	texts := slices.Contains(p.What, ApplyTexts)
	total, err := s.CountApplyTitles(ctx, roots)
	if err != nil {
		return err
	}
	done := 0
	batches := make([][]systemq.ListApplyItemsRow, 0, max(len(roots), 1))
	if len(roots) == 0 {
		all, err := s.read.ListAllApplyItems(ctx)
		if err != nil {
			return fmt.Errorf("%s: list titles: %w", JobApply, err)
		}
		batch := make([]systemq.ListApplyItemsRow, 0, len(all))
		for _, it := range all {
			batch = append(batch, systemq.ListApplyItemsRow(it))
		}
		batches = append(batches, batch)
	}
	for _, root := range roots {
		items, err := s.read.ListApplyItems(ctx, root)
		if err != nil {
			return fmt.Errorf("%s: list titles of folder %d: %w", JobApply, root, err)
		}
		batches = append(batches, items)
	}
	for _, items := range batches {
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
	s.log.Info("metadata apply queued its children", "folders", len(roots), "every_title", len(roots) == 0, "titles", done, "what", p.What)
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
