package vistest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Helpers for the playback tests (M1b): they change title rows, which only packages allowed by the
// itemsaccess analyzer may do, so they live here.

// SetMediaFile replaces the container, codecs, stored probe (JSON, "" = NULL) and keyframe index of
// a fixture media file, so a test can present any source format to the playback decision.
func (f *Fixture) SetMediaFile(t testing.TB, fileID, container, videoCodec, audioCodec, probe string, keyframes []byte) {
	t.Helper()
	exec(t, f.DB, `UPDATE media_files SET container = ?, video_codec = ?, audio_codec = ?, probe = ?, keyframes = ? WHERE id = ?`,
		container, videoCodec, audioCodec, sql.NullString{String: probe, Valid: probe != ""}, keyframes, fileID)
}

// SetRating changes the own rating of a title (nil = unrated); the triggers update the effective
// rating, so visibility changes from the next query on.
func (f *Fixture) SetRating(t testing.TB, itemID string, rating *int) {
	t.Helper()
	exec(t, f.DB, `UPDATE items SET own_rating = ? WHERE id = ?`, rating, itemID)
}

// Session inserts a browser session of profile p (of the owner account, or of the guest account
// after AddGuest) and returns it; playback sessions reference it.
func (f *Fixture) Session(t testing.TB, p model.Profile) model.Session {
	t.Helper()
	account := f.Account
	if f.Guest.ID != 0 && p.AccountID == f.Guest.ID {
		account = f.Guest
	}
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	expires := time.Now().Add(30 * 24 * time.Hour)
	row, err := appq.New(f.DB.Write).InsertSession(context.Background(), appq.InsertSessionParams{
		TokenHash: hex.EncodeToString(raw), AccountID: account.ID, ProfileID: sql.NullInt64{Int64: p.ID, Valid: true},
		Now: time.Now().UnixMilli(), ExpiresAt: expires.UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return model.Session{ID: row.ID, Account: account, Profile: p, ExpiresAt: expires}
}

// ItemFlags returns the locked_fields JSON and the rating_source of a title, which tests of the
// rating override read back.
func (f *Fixture) ItemFlags(t testing.TB, itemID string) (lockedFields, ratingSource string) {
	t.Helper()
	var src sql.NullString
	if err := f.DB.Read.QueryRowContext(context.Background(), `SELECT locked_fields, rating_source FROM items WHERE id = ?`, itemID).Scan(&lockedFields, &src); err != nil {
		t.Fatal(err)
	}
	return lockedFields, src.String
}

// AddLoneExtraSeries adds the series "LX" rated 0 to library 1 whose only child is a trailer extra
// with a media file (design 1: an extra alone keeps a series unavailable). It returns the series
// and the extra; neither is part of All.
func (f *Fixture) AddLoneExtraSeries(t testing.TB) (series, extra Movie) {
	t.Helper()
	zero := ptr(0)
	series = f.addMovie(t, Movie{Kind: "series", Label: "LX", Rating: zero, Container: true, Effective: zero, Unavailable: true},
		f.libraryID, f.rootID, f.MediaDir, 40)
	extra = f.addMovie(t, Movie{Kind: "extra", Label: "LT", ParentID: series.ID, Effective: zero, RelPath: "Lone/Trailers/Lone.mp4"},
		f.libraryID, f.rootID, f.MediaDir, 41)
	exec(t, f.DB, `UPDATE items SET extra_kind = 'trailer' WHERE id = ?`, extra.ID)
	return series, extra
}

// ContainerState returns the available flag and min_child_rating (nil = NULL) of a title.
func (f *Fixture) ContainerState(t testing.TB, itemID string) (available bool, minChild *int) {
	t.Helper()
	var avail int
	var mc sql.NullInt64
	if err := f.DB.Read.QueryRowContext(context.Background(), `SELECT available, min_child_rating FROM items WHERE id = ?`, itemID).Scan(&avail, &mc); err != nil {
		t.Fatal(err)
	}
	if mc.Valid {
		v := int(mc.Int64)
		minChild = &v
	}
	return avail == 1, minChild
}
