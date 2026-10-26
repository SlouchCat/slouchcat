package visibility

import (
	"context"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// TrailerFlags says for each of the given titles whether a trailer is stored (M11, D3: the start
// page fills ItemRef.hasTrailer for the rows spotlight and picks only). It answers in one query;
// every id gets an entry, false for a title without a trailer and for one the profile may not see.
func (r *Reader) TrailerFlags(ctx context.Context, s Scope, ids []string) (map[string]bool, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = false
	}
	if len(ids) == 0 {
		return out, nil
	}
	with, err := r.q.ListVisibleTrailerItems(ctx, scopedq.ListVisibleTrailerItemsParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return nil, fmt.Errorf("list trailer items: %w", err)
	}
	for _, id := range with {
		out[id] = true
	}
	return out, nil
}
