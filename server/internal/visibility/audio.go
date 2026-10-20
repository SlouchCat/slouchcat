package visibility

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Audiobooks (M6, design 3.1). Stream A fills the detail: book-wide chapters from the chapters
// table, author, narrator, series and volume, and the parts in order (queries/scoped/audio.sql).

// audiobookDetail completes the detail of an audiobook. The base commit leaves the summary fields
// (author, duration) as mapped from the row and adds nothing else.
func (r *Reader) audiobookDetail(_ context.Context, _ Scope, _ int64, _ scopedq.Item, d *model.ItemDetail) error {
	d.Chapters = []model.Chapter{}
	return nil
}
