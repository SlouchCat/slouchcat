package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// UpdateAudioSeries is «Reihe bearbeiten» (contract 1.25.0, concept 19 D5.4): it sets the displayed
// name of a series and locks it. With writeToVolumes it also writes the name into the series name
// of every book of the series the profile may see, exactly as the editor does (writeMetadataEditTx:
// the field gets its lock, its provenance goes), and relinks those books in the same transaction
// (library.RelinkAudioSeriesKeeping): the books keep the id of their series, also when a book the
// profile does not see or that is unavailable keeps the old name (it leaves with its own next
// relink), or move into the series that already holds their new key. The name is locked on the
// series the books belong to afterwards, which the answer describes; the series they left is
// deleted at once when no row of items holds it any more. Errors: model.ErrNotFound,
// *model.ValidationError (name required, too_long).
func (s *Service) UpdateAudioSeries(ctx context.Context, profileID int64, seriesID, name string, writeToVolumes bool) (model.AdminAudioSeries, error) {
	name = strings.TrimSpace(name)
	var verr *model.ValidationError
	switch {
	case name == "":
		verr = verr.Add("name", "required")
	case utf8.RuneCountInString(name) > model.MaxAudioSeriesName:
		verr = verr.Add("name", "too_long")
	}
	if verr != nil {
		return model.AdminAudioSeries{}, verr
	}
	lang := ""
	if writeToVolumes {
		var err error
		if lang, err = s.serverTextLang(ctx); err != nil {
			return model.AdminAudioSeries{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.AdminAudioSeries{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	series, err := q.GetAudioSeries(ctx, seriesID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AdminAudioSeries{}, model.ErrNotFound
	}
	if err != nil {
		return model.AdminAudioSeries{}, fmt.Errorf("load series: %w", err)
	}
	now := s.now()
	target := series.ID
	if writeToVolumes {
		ids, err := q.ListVisibleAudioSeriesVolumeIDs(ctx, systemq.ListVisibleAudioSeriesVolumeIDsParams{
			ProfileID: profileID, SeriesID: sql.NullString{String: series.ID, Valid: true},
		})
		if err != nil {
			return model.AdminAudioSeries{}, fmt.Errorf("list volumes: %w", err)
		}
		edit := model.ItemMetadataEdit{SeriesName: &name}
		for _, id := range ids {
			if err := s.writeMetadataEditTx(ctx, q, id, lang, edit, []string{model.MetaFieldSeriesName}); err != nil {
				return model.AdminAudioSeries{}, err
			}
		}
		if err := library.RelinkAudioSeriesKeeping(ctx, q, series.ID, ids, now); err != nil {
			return model.AdminAudioSeries{}, err
		}
		// One series name and one series give one key: the books are in one series now.
		if len(ids) > 0 {
			rows, err := q.ListAudioSeriesLinkItems(ctx, ids[:1])
			if err != nil {
				return model.AdminAudioSeries{}, fmt.Errorf("load volume: %w", err)
			}
			if len(rows) == 1 && rows[0].AudioSeriesID.Valid {
				target = rows[0].AudioSeriesID.String
			}
		}
		if target != series.ID {
			left, err := q.CountAudioSeriesRows(ctx, sql.NullString{String: series.ID, Valid: true})
			if err != nil {
				return model.AdminAudioSeries{}, fmt.Errorf("count books of the old series: %w", err)
			}
			if left == 0 {
				if err := q.DeleteAudioSeries(ctx, series.ID); err != nil {
					return model.AdminAudioSeries{}, fmt.Errorf("delete the emptied series: %w", err)
				}
			}
		}
	}
	// Only the series the books are in gets the name: a series they left keeps following its books.
	if err := q.LockAudioSeriesName(ctx, systemq.LockAudioSeriesNameParams{Name: name, Now: now.UnixMilli(), ID: target}); err != nil {
		return model.AdminAudioSeries{}, fmt.Errorf("lock series name: %w", err)
	}
	out, err := q.GetAudioSeries(ctx, target)
	if err != nil {
		return model.AdminAudioSeries{}, fmt.Errorf("load series: %w", err)
	}
	n, err := q.CountAudioSeriesLiveItems(ctx, sql.NullString{String: target, Valid: true})
	if err != nil {
		return model.AdminAudioSeries{}, fmt.Errorf("count books: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.AdminAudioSeries{}, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "audio series renamed", "series_id", series.ID, "now_in", target, "write_to_volumes", writeToVolumes)
	return model.AdminAudioSeries{ID: out.ID, LibraryID: out.LibraryID, Name: out.Name, NameLocked: out.NameLocked == 1, VolumeCount: int(n)}, nil
}
