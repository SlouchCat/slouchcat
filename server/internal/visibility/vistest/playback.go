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

// Session inserts a browser session of profile p and returns it (playback sessions reference it).
func (f *Fixture) Session(t testing.TB, p model.Profile) model.Session {
	t.Helper()
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	expires := time.Now().Add(30 * 24 * time.Hour)
	row, err := appq.New(f.DB.Write).InsertSession(context.Background(), appq.InsertSessionParams{
		TokenHash: hex.EncodeToString(raw), AccountID: f.Account.ID, ProfileID: p.ID,
		Now: time.Now().UnixMilli(), ExpiresAt: expires.UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return model.Session{ID: row.ID, Account: f.Account, Profile: p, ExpiresAt: expires}
}
