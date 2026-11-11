package metadata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// After migration 00049 the server removes the image folders of every podcast episode once
// (decision of 11 November 2026): live and soft-deleted episodes lose theirs, the podcast and
// every other title keep theirs, and a second start removes nothing.
func TestPurgeEpisodeImagesOnce(t *testing.T) {
	t.Parallel()
	e := audioEnv(t, t.TempDir())
	ctx := context.Background()
	const idDeleted = "01JTESTEPISODE00000000000D"
	e.addAudio(t, idPodcast, "podcast", 4, nil, nil, "https://img.example.test/podcast.jpg")
	e.addAudio(t, idEpisode, "podcast_episode", 4, idPodcast, nil, nil)
	e.addAudio(t, idDeleted, "podcast_episode", 4, idPodcast, nil, nil)
	mustExec(t, e.db.Write, `UPDATE items SET deleted_at = 1 WHERE id = ?`, idDeleted)
	dir := func(id string) string { return filepath.Join(e.cache, imagesDir, id) }
	write := func(id string) {
		t.Helper()
		if err := os.MkdirAll(dir(id), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir(id), "poster-w400.jpg"), []byte("jpeg"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	exists := func(id string) bool {
		_, err := os.Stat(dir(id))
		return err == nil
	}
	for _, id := range []string{idPodcast, idEpisode, idDeleted, idBookFolder} {
		write(id)
	}

	n, err := e.svc.PurgeEpisodeImagesOnce(ctx)
	if err != nil || n != 2 {
		t.Fatalf("purge = %d, %v, want 2 folders", n, err)
	}
	if exists(idEpisode) || exists(idDeleted) {
		t.Errorf("an episode kept its image folder")
	}
	if !exists(idPodcast) || !exists(idBookFolder) {
		t.Errorf("the podcast or another title lost its image folder")
	}
	var v string
	if err := e.db.Read.QueryRow(`SELECT value FROM settings WHERE key = ?`, SettingEpisodeImagesPurged).Scan(&v); err != nil || v != "1" {
		t.Errorf("setting %s = %q, %v", SettingEpisodeImagesPurged, v, err)
	}

	// Once: the next start leaves everything as it is.
	write(idEpisode)
	if n, err := e.svc.PurgeEpisodeImagesOnce(ctx); err != nil || n != 0 || !exists(idEpisode) {
		t.Errorf("second purge = %d, %v, folder there %v", n, err, exists(idEpisode))
	}
}
