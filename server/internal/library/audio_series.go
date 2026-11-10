package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/library/audioseries"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Audiobook series (concept 19 stage D2, D4.4). A book belongs to the series of its library whose
// key (audioseries.Names.Key: normalised series name plus normalised first author) it has;
// RelinkAudioSeries is the one place that computes the key and sets items.audio_series_id, called
// in the transaction of every writer of a book's series name or author: the scan
// (finishAudiobook), the editor and «Reihe bearbeiten» (package admin). The id of a series does
// not follow its key, so a new list of suffixes or a renamed series keeps the address of its page.

// relinkChunk bounds the ids of one statement of RelinkAudioSeries (the first link reads every
// book of the server).
const relinkChunk = 500

// RelinkAudioSeries links the given titles to their series inside the transaction of q. A book
// with series name gets the series of its library with its key; a book without one and every other
// kind get none. The id stays with the books (D4.4): when the new key already has a series, the
// book moves there; else, when every book of its old series that is not deleted moves to the same
// new key, the old series keeps its id and takes the new key; else a new series with a new id
// begins. The name of every series it touched follows the most common spelling of its books without
// suffix, unless it is locked. An emptied series stays until the sweep (DeleteOrphanAudioSeries).
func RelinkAudioSeries(ctx context.Context, q *systemq.Queries, itemIDs []string, now time.Time) error {
	return relinkAudioSeries(ctx, q, audioseries.Default(), itemIDs, now)
}

// relinkAudioSeries is RelinkAudioSeries with the suffix list of names (the test of D4.4 swaps it).
func relinkAudioSeries(ctx context.Context, q *systemq.Queries, names audioseries.Names, itemIDs []string, now time.Time) error {
	var rows []systemq.ListAudioSeriesLinkItemsRow
	for chunk := range slices.Chunk(itemIDs, relinkChunk) {
		got, err := q.ListAudioSeriesLinkItems(ctx, chunk)
		if err != nil {
			return fmt.Errorf("load books for their series: %w", err)
		}
		rows = append(rows, got...)
	}
	keys := make([]string, len(rows))
	byOld := map[string][]int{} // old series → the rows linked to it
	for i, r := range rows {
		if r.Kind == "audiobook" {
			keys[i] = names.Key(r.SeriesName.String, r.Author.String)
		}
		if r.AudioSeriesID.Valid {
			byOld[r.AudioSeriesID.String] = append(byOld[r.AudioSeriesID.String], i)
		}
	}
	ms := now.UnixMilli()
	touched := map[string]bool{}

	// An old series whose live books all move to one new key takes that key along (keeps its id),
	// as long as no other series of the library holds it.
	for _, old := range slices.Sorted(maps.Keys(byOld)) {
		idx := byOld[old]
		key, live := keys[idx[0]], 0
		for _, i := range idx {
			if keys[i] != key {
				key = ""
				break
			}
			if !rows[i].DeletedAt.Valid {
				live++
			}
		}
		if key == "" {
			continue
		}
		cur, err := q.GetAudioSeries(ctx, old)
		if err != nil {
			return fmt.Errorf("load series %s: %w", old, err)
		}
		if cur.SeriesKey == key {
			continue
		}
		total, err := q.CountAudioSeriesLiveItems(ctx, sql.NullString{String: old, Valid: true})
		if err != nil {
			return fmt.Errorf("count books of series %s: %w", old, err)
		}
		if int(total) != live {
			continue // books outside this call stay: the moving ones go to another series
		}
		if _, err := q.GetAudioSeriesByKey(ctx, systemq.GetAudioSeriesByKeyParams{LibraryID: cur.LibraryID, SeriesKey: key}); err == nil {
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("find series by key: %w", err)
		}
		if err := q.SetAudioSeriesKey(ctx, systemq.SetAudioSeriesKeyParams{SeriesKey: key, Now: ms, ID: old}); err != nil {
			return fmt.Errorf("move key of series %s: %w", old, err)
		}
		touched[old] = true
	}

	// Every book to the series of its key, created when the library has none.
	for i, r := range rows {
		target := sql.NullString{}
		if key := keys[i]; key != "" {
			s, err := q.GetAudioSeriesByKey(ctx, systemq.GetAudioSeriesByKeyParams{LibraryID: r.LibraryID, SeriesKey: key})
			switch {
			case err == nil:
				target = sql.NullString{String: s.ID, Valid: true}
			case errors.Is(err, sql.ErrNoRows):
				id := ulid.MustNew(ulid.Timestamp(now), ulid.DefaultEntropy()).String()
				if err := q.InsertAudioSeries(ctx, systemq.InsertAudioSeriesParams{
					ID: id, LibraryID: r.LibraryID, SeriesKey: key, Name: names.Display(r.SeriesName.String), Now: ms,
				}); err != nil {
					return fmt.Errorf("insert series: %w", err)
				}
				target = sql.NullString{String: id, Valid: true}
			default:
				return fmt.Errorf("find series by key: %w", err)
			}
			touched[target.String] = true
		}
		if target == r.AudioSeriesID {
			continue
		}
		if r.AudioSeriesID.Valid {
			touched[r.AudioSeriesID.String] = true
		}
		if err := q.SetItemAudioSeries(ctx, systemq.SetItemAudioSeriesParams{SeriesID: target, ID: r.ID}); err != nil {
			return fmt.Errorf("link book %s to its series: %w", r.ID, err)
		}
	}

	for _, id := range slices.Sorted(maps.Keys(touched)) {
		if err := refreshAudioSeriesName(ctx, q, names, id, ms); err != nil {
			return err
		}
	}
	return nil
}

// refreshAudioSeriesName sets the name of a series that is not locked to the most common spelling
// of its books without suffix (audioseries.Names.Display); a series without books keeps its name.
func refreshAudioSeriesName(ctx context.Context, q *systemq.Queries, names audioseries.Names, id string, ms int64) error {
	spellings, err := q.ListAudioSeriesSpellings(ctx, sql.NullString{String: id, Valid: true})
	if err != nil {
		return fmt.Errorf("list spellings of series %s: %w", id, err)
	}
	var shown []string
	for _, sp := range spellings {
		for range sp.N {
			shown = append(shown, names.Display(sp.SeriesName))
		}
	}
	name := audioseries.MostCommon(shown)
	if name == "" {
		return nil
	}
	if err := q.SetAudioSeriesName(ctx, systemq.SetAudioSeriesNameParams{Name: name, Now: ms, ID: id}); err != nil {
		return fmt.Errorf("name series %s: %w", id, err)
	}
	return nil
}

// SettingAudioSeriesLinked marks that the books that existed before migration 00045 were linked
// to their series once.
const SettingAudioSeriesLinked = "library.audio_series_linked"

// RelinkAudioSeriesOnce links every audiobook (soft-deleted ones too) to its series at the first
// start after migration 00045, in one transaction, and remembers in the settings that it did;
// later starts do nothing (D4.4). It reports whether it linked.
func (s *Service) RelinkAudioSeriesOnce(ctx context.Context) (bool, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	if _, err := q.GetSystemSetting(ctx, SettingAudioSeriesLinked); err == nil {
		return false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read %s: %w", SettingAudioSeriesLinked, err)
	}
	ids, err := q.ListAudiobookIDs(ctx)
	if err != nil {
		return false, fmt.Errorf("list audiobooks: %w", err)
	}
	now := s.now()
	if err := RelinkAudioSeries(ctx, q, ids, now); err != nil {
		return false, err
	}
	if err := q.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{Key: SettingAudioSeriesLinked, Value: "1", Now: now.UnixMilli()}); err != nil {
		return false, fmt.Errorf("write %s: %w", SettingAudioSeriesLinked, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit audio series link: %w", err)
	}
	s.log.InfoContext(ctx, "audiobooks linked to their series", "books", len(ids))
	return true, nil
}
