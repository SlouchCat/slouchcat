package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Home folder of a title (concept 24 section 3.2, ADR 0036). Every title has the folder of its
// oldest present file as its home folder (items.root_id); a series and a podcast count the files
// of their episodes, and seasons, episodes and extras carry the home folder of their title. The
// folder's own languages and images (metadata.Effective) and its provider list apply to the titles
// whose home folder it is.
//
// The scan sets it when it creates a title (InsertItem, InsertAudiobook, PodcastInsertFromFolder;
// a child takes its parent's). rehome determines it again once a title has no present file left in
// its home folder but one elsewhere: a folder removed, a folder with another content, files moved
// to another folder, a title that returns. A title keeps its home folder as long as a file of it is
// present there, even when a file elsewhere is older; a new path of a folder keeps the folder's id
// and so the home folder (ADR 0023).

// rehome determines the home folder of the top-level title id again in the caller's transaction:
// when the title has no present file in its home folder (or none) but one elsewhere, the folder of
// the oldest present file becomes its home folder, the title with its seasons, episodes and extras
// takes that folder's provider list, and a live title queues its match (the fill follows it) -- an
// audiobook its book match by the rules of queueBookMatch. A title without any present file keeps
// what it has. It reports whether the home folder changed. spread writes an unchanged home folder
// and its provider list onto the children too (a relink moves children of another title below it).
func rehome(ctx context.Context, q *systemq.Queries, now time.Time, id string, spread bool) (bool, error) {
	item, err := q.GetItemSystem(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("home folder of %s: %w", id, err)
	}
	roots, err := q.ListTitlePresentRoots(ctx, id)
	if err != nil {
		return false, fmt.Errorf("home folder of %s: present files: %w", id, err)
	}
	if len(roots) == 0 || item.RootID.Valid && slices.Contains(roots, item.RootID.Int64) {
		if spread && item.RootID.Valid {
			if _, err := q.SetTitleHome(ctx, systemq.SetTitleHomeParams{RootID: item.RootID, Now: now.UnixMilli(), TitleID: id}); err != nil {
				return false, fmt.Errorf("spread home folder of %s: %w", id, err)
			}
		}
		return false, nil
	}
	target := roots[0]
	if _, err := q.SetTitleHome(ctx, systemq.SetTitleHomeParams{RootID: sql.NullInt64{Int64: target, Valid: true}, Now: now.UnixMilli(), TitleID: id}); err != nil {
		return false, fmt.Errorf("set home folder of %s: %w", id, err)
	}
	if item.DeletedAt.Valid {
		return true, nil
	}
	switch item.Kind {
	case "movie", "series":
		if err := enqueueTx(ctx, q, now, matchJob(id)); err != nil {
			return false, err
		}
	case "audiobook":
		providers, err := q.GetLibraryRootProviders(ctx, target)
		if err != nil {
			return false, fmt.Errorf("sources of folder %d: %w", target, err)
		}
		if err := queueBookMatch(ctx, q, item, model.ParseProviders(providers), now); err != nil {
			return false, err
		}
	}
	return true, nil
}

// RehomeTx is rehome for another package (the merge of package admin moves files and children
// below a title and spreads its home folder onto them, ADR 0036).
func RehomeTx(ctx context.Context, q *systemq.Queries, now time.Time, id string, spread bool) (bool, error) {
	return rehome(ctx, q, now, id, spread)
}

// titleOf returns the top-level title of item id: the item itself, its parent or its grandparent
// (a film, series, audiobook or podcast). An unknown id is its own title; rehome skips it.
func titleOf(ctx context.Context, q *systemq.Queries, id string) (string, error) {
	for range 2 {
		item, err := q.GetItemSystem(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return id, nil
		}
		if err != nil {
			return "", fmt.Errorf("title of %s: %w", id, err)
		}
		if !item.ParentID.Valid {
			return id, nil
		}
		id = item.ParentID.String
	}
	return id, nil
}

// rehomeAll runs rehome for every id and returns how many titles moved.
func rehomeAll(ctx context.Context, q *systemq.Queries, now time.Time, ids []string) (int, error) {
	moved := 0
	for _, id := range ids {
		ok, err := rehome(ctx, q, now, id, false)
		if err != nil {
			return moved, err
		}
		if ok {
			moved++
		}
	}
	return moved, nil
}
