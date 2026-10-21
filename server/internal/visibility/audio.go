package visibility

import (
	"context"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Audiobooks (M6, design 3.1): the book-wide chapters from the chapters table and the parts of a
// book in playback order (queries/scoped/audio.sql). Author, narrator, series and volume come
// from the item row (detailFields).

// audiobookDetail completes the detail of an audiobook with its book-wide chapters.
func (r *Reader) audiobookDetail(ctx context.Context, _ Scope, pid int64, it scopedq.Item, d *model.ItemDetail) error {
	chapters, err := r.chapters(ctx, pid, it.ID)
	if err != nil {
		return err
	}
	d.Chapters = chapters
	return nil
}

// Chapters returns the book-wide chapters of a visible audiobook in order; empty for a visible
// title without chapters (another kind, or a book whose parts are not all probed yet). A title the
// profile may not see is model.ErrNotFound.
func (r *Reader) Chapters(ctx context.Context, s Scope, itemID string) ([]model.Chapter, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	if _, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: itemID}); err != nil {
		return nil, notFound(err)
	}
	return r.chapters(ctx, pid, itemID)
}

func (r *Reader) chapters(ctx context.Context, pid int64, itemID string) ([]model.Chapter, error) {
	rows, err := r.q.ListVisibleChapters(ctx, scopedq.ListVisibleChaptersParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}
	out := make([]model.Chapter, 0, len(rows))
	for _, c := range rows {
		out = append(out, model.Chapter{StartMs: c.StartMs, EndMs: c.EndMs, Title: c.Title})
	}
	return out, nil
}

// Parts returns the present media files of a visible audiobook in playback order (part_index), each
// with its library root; a book of one file has one part. A title the profile may not see, and a
// title without a present file, is model.ErrNotFound.
func (r *Reader) Parts(ctx context.Context, s Scope, itemID string) ([]MediaFileLocation, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListVisibleAudiobookParts(ctx, scopedq.ListVisibleAudiobookPartsParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return nil, fmt.Errorf("list audiobook parts: %w", err)
	}
	if len(rows) == 0 {
		return nil, model.ErrNotFound
	}
	out := make([]MediaFileLocation, 0, len(rows))
	for _, f := range rows {
		out = append(out, MediaFileLocation{ItemID: f.ItemID, RootPath: f.RootPath, RelPath: f.RelPath, File: mediaFile(scopedq.ListMediaFilesForItemRow{
			ID: f.ID, ItemID: f.ItemID, RelPath: f.RelPath, Size: f.Size, Mtime: f.Mtime, Container: f.Container,
			VideoCodec: f.VideoCodec, AudioCodec: f.AudioCodec, Width: f.Width, Height: f.Height, DurationMs: f.DurationMs,
			VersionLabel: f.VersionLabel, MissingSince: f.MissingSince, CreatedAt: f.CreatedAt, RootPath: f.RootPath,
		})})
	}
	return out, nil
}
