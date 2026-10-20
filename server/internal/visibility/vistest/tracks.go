package vistest

import (
	"context"
	"database/sql"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Fixture data of M4 (design sections 2, 5 and 7): tracks of the media files (plan 10.3: two audio
// tracks, an embedded text subtitle, a bitmap subtitle and a sidecar file), an intro and outro of an
// episode and profiles with playback limits.

// FileTracks are the track ids of one fixture media file (AddTracks).
type FileTracks struct {
	AudioDefault int64 // AAC, stream index 1, default
	AudioAlt     int64 // AC3 5.1, stream index 2
	Text         int64 // embedded SubRip, stream index 3, English
	Bitmap       int64 // PGS, stream index 4, German
	Sidecar      int64 // SubRip sidecar <stem>.de.srt next to the file
	SidecarRel   string
}

// SidecarContent is the content of every sidecar file AddTracks writes.
const SidecarContent = "1\n00:00:01,000 --> 00:00:02,000\nHallo\n"

// AddTracks gives the media file of every playable title (movies, episodes, extras at the time of
// the call) the tracks of plan 10.3 and writes the sidecar file; it returns the track ids by media
// file id. The stored probes stay as they are, so playback decisions do not change.
func (f *Fixture) AddTracks(t testing.TB) map[string]FileTracks {
	t.Helper()
	ctx := context.Background()
	out := map[string]FileTracks{}
	for _, m := range f.Playable() {
		dir := f.MediaDir
		if m.OtherLibrary {
			dir = f.OtherMediaDir
		}
		stem := strings.TrimSuffix(m.RelPath, path.Ext(m.RelPath))
		ft := FileTracks{SidecarRel: stem + ".de.srt"}
		writeFile(t, filepath.Join(dir, filepath.FromSlash(ft.SidecarRel)), []byte(SidecarContent))
		for _, tr := range []struct {
			id                     *int64
			index                  int64
			kind, codec, lang, src string
			rel                    string
			isDefault              int64
		}{
			{&ft.AudioDefault, 1, "audio", "aac", "de", "embedded", "", 1},
			{&ft.AudioAlt, 2, "audio", "ac3", "en", "embedded", "", 0},
			{&ft.Text, 3, "subtitle", "subrip", "en", "embedded", "", 0},
			{&ft.Bitmap, 4, "subtitle", "hdmv_pgs_subtitle", "de", "embedded", "", 0},
			{&ft.Sidecar, 0, "subtitle", "subrip", "de", "sidecar", ft.SidecarRel, 0},
		} {
			err := f.DB.Write.QueryRowContext(ctx, `INSERT INTO tracks (media_file_id, stream_index, kind, codec, language, source, rel_path, is_default)
			                                       VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
				m.FileID, tr.index, tr.kind, tr.codec, tr.lang, tr.src, sql.NullString{String: tr.rel, Valid: tr.rel != ""}, tr.isDefault).Scan(tr.id)
			if err != nil {
				t.Fatal(err)
			}
		}
		out[m.FileID] = ft
	}
	return out
}

// AddSegments stores an intro (5–35 s, fingerprint) and an outro (the last two minutes, chapter)
// for an episode (W-9).
func (f *Fixture) AddSegments(t testing.TB, itemID string) model.Segments {
	t.Helper()
	intro := model.Segment{StartMs: 5_000, EndMs: 35_000, Source: "fingerprint"}
	outro := model.Segment{StartMs: DurationMs - 120_000, EndMs: DurationMs, Source: "chapter"}
	for kind, s := range map[string]model.Segment{"intro": intro, "outro": outro} {
		exec(t, f.DB, `INSERT INTO media_segments (item_id, kind, start_ms, end_ms, source, confidence, created_at) VALUES (?, ?, ?, ?, ?, 0.9, 1)`,
			itemID, kind, s.StartMs, s.EndMs, s.Source)
	}
	return model.Segments{Intro: &intro, Outro: &outro}
}

// AddLimitedProfile adds a profile with playback limits (W-8, K-5) to the owner account: max 18,
// all libraries, language de.
func (f *Fixture) AddLimitedProfile(t testing.TB, name string, limits model.PlaybackLimits) model.Profile {
	t.Helper()
	col := func(v int) sql.NullInt64 { return sql.NullInt64{Int64: int64(v), Valid: v > 0} }
	p, err := appq.New(f.DB.Write).InsertProfile(context.Background(), appq.InsertProfileParams{
		AccountID: f.Account.ID, Name: name, MaxRating: 18, Language: "de", AllLibraries: 1, Now: 1,
		MaxBitrateKbps: col(limits.MaxBitrateKbps), MaxHeight: col(limits.MaxHeight), MaxStreams: col(limits.MaxStreams),
		WatchLimitMinutes: col(limits.WatchLimitMinutes),
	})
	if err != nil {
		t.Fatal(err)
	}
	return model.Profile{ID: p.ID, AccountID: f.Account.ID, Name: name, MaxRating: 18, Language: "de", AllLibraries: true,
		LibraryIDs: []int64{}, Limits: limits, Effective: limits}
}
