package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Person and collection images (concept 17 stage 3, sections 5.1 and 5.2, ADR 0026). The job
// people.images fetches the TMDB portrait of every person with a role on a title that is not
// deleted (w185, kept as fetched in JPEG, WebP derived through ffmpeg) and the TMDB poster of every
// TMDB collection with a member that is not deleted (w500 fetched, w400 derived as JPEG and WebP)
// into the cache directory: people/<id>/w185.{jpg,webp} and collections/<id>/w400.{jpg,webp}. The
// rows live in person_images and collection_images (migration 00040); the downloads go through
// the CDN limiter of the client (Client.DownloadCDNImage), not the API limiter.
//
// One run is one job under the key people.images: first the people, then the collections, each in
// batches of peopleImagesBatch by id (queries/system/people_images.sql says what is open). A 404,
// an empty or undecodable answer records a 'missing' row, so no run asks again before the row is
// older than peopleImagesMissingAge. Any other failure (5xx, 429 after the retries, a timeout, an
// ffmpeg error) writes nothing; peopleImagesFailures of them in a row end the run with an error
// and the queue retries it after its backoff. After the last batch the run starts another pass
// while open work is left that did not fail in this run (a match or the backfill meanwhile; a new
// job would be swallowed by the running one), at most peopleImagesPasses passes in all.
//
// The job is queued after a match that wrote credits or a collection (applyTitle) and at the end
// of QueuePendingMetadata when something is open, so the first full scan after the update fetches
// the whole stock. people.images_backfill is a schedule type without a handler (off by default):
// it deletes the 'missing' rows and queues people.images, the button «Jetzt ausführen» of the
// jobs page.
const (
	JobPeopleImages         = "people.images"
	JobPeopleImagesBackfill = "people.images_backfill"
	peopleImagesKey         = "people.images"
	// peopleImagesPriority is below image.fetch: the portraits are bulk work.
	peopleImagesPriority = 1
	// peopleImagesBackfillEvery is the default interval of the backfill (its schedule is off).
	peopleImagesBackfillEvery = 7 * 24 * time.Hour
	peopleImagesBatch         = 200
	// peopleImagesFailures is how many downloads or conversions in a row may fail before the run
	// stops with an error.
	peopleImagesFailures = 3
	// peopleImagesPasses bounds the passes of one run.
	peopleImagesPasses = 3
	// peopleImagesMissingAge is how long a failed fetch ('missing') stands before a run asks again.
	peopleImagesMissingAge = 30 * 24 * time.Hour

	peopleImagesDir      = "people"
	collectionImagesDir  = "collections"
	personImageSize      = "w185"
	collectionFetchSize  = "w500"
	collectionImageSize  = "w400"
	collectionImageWidth = 400
)

// errPeopleImagesFailures: peopleImagesFailures downloads or conversions in a row failed.
var errPeopleImagesFailures = errors.New("image downloads fail")

// enqueuePeopleImages queues people.images at prio and returns the job id; a queued or running
// job holds the key, so ten matches make one job.
func (s *Service) enqueuePeopleImages(ctx context.Context, q *jobs.Queue, prio int) (int64, error) {
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobPeopleImages, Priority: prio, IdempotencyKey: peopleImagesKey})
	if err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", JobPeopleImages, err)
	}
	return id, nil
}

// QueuePeopleImages queues people.images when a person or a collection is open and reports
// whether it did (QueuePendingMetadata, after every full scan).
func (s *Service) QueuePeopleImages(ctx context.Context) (bool, error) {
	open, err := s.peopleImagesLeft(ctx, nil)
	if err != nil || !open {
		return false, err
	}
	_, err = s.enqueuePeopleImages(ctx, s.queue, peopleImagesPriority)
	return err == nil, err
}

// runPeopleImagesBackfill is the Run of people.images_backfill: it deletes the 'missing' rows of
// both tables and queues people.images at prio (a waiting job is raised to it; a running one takes
// the work in its next pass).
func (s *Service) runPeopleImagesBackfill(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	people, err := s.write.DeleteMissingPersonImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: delete missing person images: %w", JobPeopleImagesBackfill, err)
	}
	colls, err := s.write.DeleteMissingCollectionImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: delete missing collection images: %w", JobPeopleImagesBackfill, err)
	}
	id, err := s.enqueuePeopleImages(ctx, q, prio)
	if err != nil {
		return nil, err
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobPeopleImages, err)
	}
	s.log.InfoContext(ctx, "person images backfill", "missing_people", people, "missing_collections", colls)
	return []int64{id}, nil
}

// imagesRun is the state of one run of people.images.
type imagesRun struct {
	failed   map[string]bool // subject keys that failed in this run
	failures int             // failures in a row
	fetched  int             // subjects with new rows (ok or missing)
	dropped  int             // subjects whose rows went
}

// imageSubject is a person or a collection as one pass sees it.
type imageSubject struct {
	key        string // person:<id>, collection:<id>
	dir        string // below the cache root
	source     string // the provider path; "" when there is none
	open       bool   // the open condition of the query
	jpegStatus string // "" without a JPEG row
	jpegRel    string
	webpRel    string
	fetchSize  string // the TMDB size downloaded
	size       string // the stored size and file name
	width      int    // 0: the JPEG stays as fetched and WebP gets its width; else both are scaled to it

	upsert   func(ctx context.Context, format, status string, f *storedFile) error
	drop     func(ctx context.Context) error // every row
	dropWebP func(ctx context.Context) error
}

// storedFile is one stored file of a subject.
type storedFile struct {
	rel, hash, mediaType string
	width, height, bytes int
}

// handlePeopleImages is the handler of people.images.
func (s *Service) handlePeopleImages(ctx context.Context, _ *jobs.Job) error {
	if s.cacheE != nil {
		return s.cacheE
	}
	root, err := paths.Open(s.cache)
	if err != nil {
		return fmt.Errorf("%s: %w", JobPeopleImages, err)
	}
	defer root.Close()
	run := &imagesRun{failed: map[string]bool{}}
	for range peopleImagesPasses {
		if err := s.peopleImagesPass(ctx, root, run); err != nil {
			return err
		}
		left, err := s.peopleImagesLeft(ctx, run.failed)
		if err != nil {
			return err
		}
		if !left {
			break
		}
	}
	if run.fetched == 0 && run.dropped == 0 && len(run.failed) == 0 {
		jobs.Skip(ctx)
	}
	s.log.InfoContext(ctx, "person and collection images done", "fetched", run.fetched, "dropped", run.dropped, "failed", len(run.failed))
	return nil
}

// missingBefore is the fetched_at before which a 'missing' row is open again.
func (s *Service) missingBefore() int64 {
	return s.now().Add(-peopleImagesMissingAge).UnixMilli()
}

// peopleImagesLeft reports whether a person or a collection is open that is not in failed.
func (s *Service) peopleImagesLeft(ctx context.Context, failed map[string]bool) (bool, error) {
	// Among len(failed)+1 open subjects of a table at least one did not fail, if there is one.
	limit := int64(len(failed) + 1)
	people, err := s.read.ListOpenPersonImages(ctx, systemq.ListOpenPersonImagesParams{MissingBefore: s.missingBefore(), LimitCount: limit})
	if err != nil {
		return false, fmt.Errorf("list open person images: %w", err)
	}
	for _, id := range people {
		if !failed[personKey(id)] {
			return true, nil
		}
	}
	colls, err := s.read.ListOpenCollectionImages(ctx, systemq.ListOpenCollectionImagesParams{MissingBefore: s.missingBefore(), LimitCount: limit})
	if err != nil {
		return false, fmt.Errorf("list open collection images: %w", err)
	}
	for _, id := range colls {
		if !failed[collectionKey(id)] {
			return true, nil
		}
	}
	return false, nil
}

func personKey(id int64) string { return "person:" + strconv.FormatInt(id, 10) }

func collectionKey(id string) string { return "collection:" + id }

// peopleImagesPass walks every person and then every collection that is a candidate.
func (s *Service) peopleImagesPass(ctx context.Context, root *os.Root, run *imagesRun) error {
	np, err := s.read.CountPersonImageCandidates(ctx)
	if err != nil {
		return fmt.Errorf("count person image candidates: %w", err)
	}
	nc, err := s.read.CountCollectionImageCandidates(ctx)
	if err != nil {
		return fmt.Errorf("count collection image candidates: %w", err)
	}
	total, done := np+nc, int64(0)
	cutoff := s.missingBefore()
	var afterPerson int64
	for {
		rows, err := s.read.ListPersonImageCandidates(ctx, systemq.ListPersonImageCandidatesParams{MissingBefore: cutoff, AfterID: afterPerson, LimitCount: peopleImagesBatch})
		if err != nil {
			return fmt.Errorf("list person image candidates: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			afterPerson = r.ID
			jobs.Progress(ctx, done, max(total, done), "")
			if err := s.processSubject(ctx, root, run, s.personSubject(r)); err != nil {
				return err
			}
			done++
		}
	}
	afterColl := ""
	for {
		rows, err := s.read.ListCollectionImageCandidates(ctx, systemq.ListCollectionImageCandidatesParams{MissingBefore: cutoff, AfterID: afterColl, LimitCount: peopleImagesBatch})
		if err != nil {
			return fmt.Errorf("list collection image candidates: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			afterColl = r.ID
			jobs.Progress(ctx, done, max(total, done), "")
			if !isULID(r.ID) {
				done++
				continue // the id names a directory; a collection id is always a ULID
			}
			if err := s.processSubject(ctx, root, run, s.collectionSubject(r)); err != nil {
				return err
			}
			done++
		}
	}
	jobs.Progress(ctx, done, done, "")
	return nil
}

// personSubject describes a person row: portrait w185, JPEG as fetched.
func (s *Service) personSubject(r systemq.ListPersonImageCandidatesRow) imageSubject {
	id := r.ID
	return imageSubject{
		key: personKey(id), dir: peopleImagesDir + "/" + strconv.FormatInt(id, 10), source: r.ProfilePath.String,
		open: r.IsOpen == 1, jpegStatus: r.JpegStatus.String, jpegRel: r.JpegRelPath.String, webpRel: r.WebpRelPath.String,
		fetchSize: personImageSize, size: personImageSize,
		upsert: func(ctx context.Context, format, status string, f *storedFile) error {
			p := systemq.UpsertPersonImageParams{PersonID: id, Format: format, Status: status, Source: r.ProfilePath.String,
				Size: personImageSize, Now: s.now().UnixMilli()}
			p.RelPath, p.ContentHash, p.MediaType, p.Width, p.Height, p.Bytes = f.columns()
			return s.write.UpsertPersonImage(ctx, p)
		},
		drop: func(ctx context.Context) error { return s.write.DeletePersonImages(ctx, id) },
		dropWebP: func(ctx context.Context) error {
			return s.write.DeletePersonImage(ctx, systemq.DeletePersonImageParams{PersonID: id, Format: string(transcode.ImageWebP)})
		},
	}
}

// collectionSubject describes a collection row: poster w500 fetched, w400 derived in both formats.
func (s *Service) collectionSubject(r systemq.ListCollectionImageCandidatesRow) imageSubject {
	id := r.ID
	return imageSubject{
		key: collectionKey(id), dir: collectionImagesDir + "/" + id, source: r.PosterPath.String,
		open: r.IsOpen == 1, jpegStatus: r.JpegStatus.String, jpegRel: r.JpegRelPath.String, webpRel: r.WebpRelPath.String,
		fetchSize: collectionFetchSize, size: collectionImageSize, width: collectionImageWidth,
		upsert: func(ctx context.Context, format, status string, f *storedFile) error {
			p := systemq.UpsertCollectionImageParams{CollectionID: id, Format: format, Status: status, Source: r.PosterPath.String,
				Size: collectionImageSize, Now: s.now().UnixMilli()}
			p.RelPath, p.ContentHash, p.MediaType, p.Width, p.Height, p.Bytes = f.columns()
			return s.write.UpsertCollectionImage(ctx, p)
		},
		drop: func(ctx context.Context) error { return s.write.DeleteCollectionImages(ctx, id) },
		dropWebP: func(ctx context.Context) error {
			return s.write.DeleteCollectionImage(ctx, systemq.DeleteCollectionImageParams{CollectionID: id, Format: string(transcode.ImageWebP)})
		},
	}
}

// columns returns the file columns of a row; all NULL for a 'missing' row (f nil).
func (f *storedFile) columns() (rel, hash, mediaType sql.NullString, width, height, size sql.NullInt64) {
	if f == nil {
		return
	}
	return sql.NullString{String: f.rel, Valid: true}, sql.NullString{String: f.hash, Valid: true},
		sql.NullString{String: f.mediaType, Valid: true}, sql.NullInt64{Int64: int64(f.width), Valid: true},
		sql.NullInt64{Int64: int64(f.height), Valid: true}, sql.NullInt64{Int64: int64(f.bytes), Valid: true}
}

// pending reports whether a subject needs work: the query found it open, or an 'ok' row's file is
// gone.
func (sub imageSubject) pending(exists func(rel string) bool) bool {
	if sub.open {
		return true
	}
	return sub.jpegStatus == "ok" && (!exists(sub.jpegRel) || !exists(sub.webpRel))
}

// processSubject brings the rows and files of one subject up to date.
func (s *Service) processSubject(ctx context.Context, root *os.Root, run *imagesRun, sub imageSubject) error {
	if err := ctx.Err(); err != nil {
		return err // a pause or a cancel
	}
	if run.failed[sub.key] || !sub.pending(rootExists(root)) {
		return nil
	}
	if sub.source == "" {
		// The path is gone: so are the rows and the files.
		if err := sub.drop(ctx); err != nil {
			return fmt.Errorf("%s: delete rows of %s: %w", JobPeopleImages, sub.key, err)
		}
		s.removeSubjectDir(root, sub)
		run.dropped++
		return nil
	}
	if !tmdbImagePath(sub.source) {
		return s.markMissing(ctx, root, run, sub)
	}
	data, _, err := s.client.DownloadCDNImage(ctx, sub.fetchSize, sub.source)
	switch {
	case errors.Is(err, ErrNotFound):
		return s.markMissing(ctx, root, run, sub)
	case err != nil:
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return s.failSubject(run, sub, err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return s.markMissing(ctx, root, run, sub) // empty or not an image
	}
	jpeg, webp, err := s.storeSubjectFiles(ctx, root, sub, data, cfg.Width, format)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return s.failSubject(run, sub, err)
	}
	if err := sub.upsert(ctx, string(transcode.ImageJPEG), "ok", jpeg); err != nil {
		return fmt.Errorf("%s: store jpeg row of %s: %w", JobPeopleImages, sub.key, err)
	}
	if err := sub.upsert(ctx, string(transcode.ImageWebP), "ok", webp); err != nil {
		return fmt.Errorf("%s: store webp row of %s: %w", JobPeopleImages, sub.key, err)
	}
	run.failures = 0
	run.fetched++
	return nil
}

// failSubject counts a failure that writes nothing; peopleImagesFailures in a row end the run.
func (s *Service) failSubject(run *imagesRun, sub imageSubject, err error) error {
	run.failed[sub.key] = true
	run.failures++
	s.log.Warn("person or collection image failed", "subject", sub.key, "err", err)
	if run.failures >= peopleImagesFailures {
		return fmt.Errorf("%s: %d %w in a row: %w", JobPeopleImages, run.failures, errPeopleImagesFailures, err)
	}
	return nil
}

// markMissing records a failed fetch as one 'missing' JPEG row and removes the files.
func (s *Service) markMissing(ctx context.Context, root *os.Root, run *imagesRun, sub imageSubject) error {
	if err := sub.dropWebP(ctx); err != nil {
		return fmt.Errorf("%s: delete webp row of %s: %w", JobPeopleImages, sub.key, err)
	}
	if err := sub.upsert(ctx, string(transcode.ImageJPEG), "missing", nil); err != nil {
		return fmt.Errorf("%s: store missing row of %s: %w", JobPeopleImages, sub.key, err)
	}
	s.removeSubjectDir(root, sub)
	run.failures = 0
	run.fetched++
	return nil
}

// removeSubjectDir deletes the files of a subject; a missing directory is fine.
func (s *Service) removeSubjectDir(root *os.Root, sub imageSubject) {
	if err := root.RemoveAll(sub.dir); err != nil {
		s.log.Warn("remove person or collection images", "subject", sub.key, "err", err)
	}
}

// tmdbImagePath accepts a TMDB image path: one segment after the slash, no query.
func tmdbImagePath(p string) bool {
	return len(p) > 1 && p[0] == '/' && !strings.ContainsAny(p[1:], "/\\?#%") && !strings.Contains(p, "..")
}

// storeSubjectFiles writes the JPEG and the WebP of a subject from the fetched data of the given
// width and format (image.DecodeConfig): a person's JPEG stays as fetched and its WebP gets the
// same width; otherwise ffmpeg scales both formats to sub.width (or the fetched width). ffmpeg
// reads the data with the demuxer of format. The files are scaled to temporary names and renamed
// into place only when both exist.
func (s *Service) storeSubjectFiles(ctx context.Context, root *os.Root, sub imageSubject, data []byte, fetchedWidth int, format string) (jpeg, webp *storedFile, err error) {
	if err := root.MkdirAll(sub.dir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("create image dir: %w", err)
	}
	src, err := tempName(sub.dir)
	if err != nil {
		return nil, nil, err
	}
	if err := writeFile(root, sub.dir, src, data); err != nil {
		return nil, nil, err
	}
	tmps := []string{src}
	defer func() {
		for _, t := range tmps {
			if rmErr := root.Remove(t); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
				s.log.Warn("remove temporary image", "subject", sub.key, "err", rmErr)
			}
		}
	}()
	keep := sub.width == 0 && format == "jpeg"
	width := sub.width
	if width == 0 {
		width = fetchedWidth
	}
	formats := []imageFormat{imageFormats[1]} // WebP
	if !keep {
		formats = imageFormats // JPEG and WebP
	}
	in, err := paths.Resolve(s.cache, src)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve image: %w", err)
	}
	outs := make([]transcode.ImageOutput, 0, len(formats))
	rels := make([]string, 0, len(formats))
	for _, f := range formats {
		tmp, err := tempName(sub.dir)
		if err != nil {
			return nil, nil, err
		}
		tmps = append(tmps, tmp)
		abs, err := paths.Resolve(s.cache, tmp)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve temporary image: %w", err)
		}
		outs = append(outs, transcode.ImageOutput{Path: abs, Width: width, Format: f.format})
		rels = append(rels, tmp)
	}
	if err := s.runner.ScaleImage(ctx, in, transcode.ImageDemuxer(format), outs); err != nil {
		return nil, nil, fmt.Errorf("scale %s: %w", sub.key, err)
	}
	files := map[transcode.ImageFormat]*storedFile{}
	moves := map[string]string{} // temporary name → final name
	if keep {
		files[transcode.ImageJPEG], err = describeFile(data, "image/jpeg", sub.dir+"/"+sub.size+".jpg")
		if err != nil {
			return nil, nil, err
		}
		moves[src] = files[transcode.ImageJPEG].rel
	}
	for i, f := range formats {
		out, err := root.ReadFile(rels[i])
		if err != nil {
			return nil, nil, fmt.Errorf("read scaled image: %w", err)
		}
		if files[f.format], err = describeFile(out, f.mediaType, sub.dir+"/"+sub.size+"."+f.ext); err != nil {
			return nil, nil, err
		}
		moves[rels[i]] = files[f.format].rel
	}
	for tmp, rel := range moves {
		if err := root.Rename(tmp, rel); err != nil {
			return nil, nil, fmt.Errorf("rename image: %w", err)
		}
	}
	return files[transcode.ImageJPEG], files[transcode.ImageWebP], nil
}

// describeFile returns the row values of an image file to be stored at rel.
func describeFile(data []byte, mediaType, rel string) (*storedFile, error) {
	cfg, err := imageConfig(data, mediaType)
	if err != nil {
		return nil, fmt.Errorf("decode %s header: %w", mediaType, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || len(data) == 0 {
		return nil, fmt.Errorf("empty %s image", mediaType)
	}
	sum := sha256.Sum256(data)
	return &storedFile{rel: rel, hash: hex.EncodeToString(sum[:]), mediaType: mediaType, width: cfg.Width, height: cfg.Height, bytes: len(data)}, nil
}
