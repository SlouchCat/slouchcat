package metadata

import (
	"context"
	"testing"
)

// The original language of a book (concept 19 D5.3) is set and locked like a film's; lifting it
// reads the files again: a refresh of a book queues the probes of its parts and no match, which
// would end it skipped (unsupported_kind) and keep image.fetch away.
func TestBookLanguageAndRefresh(t *testing.T) {
	t.Parallel()
	e := audioEnv(t, t.TempDir())
	ctx := context.Background()
	e.addAudio(t, idBookFolder, "audiobook", 3, nil, nil, nil)
	e.addMediaFile(t, "01JTESTFILE0000000000000LA", idBookFolder, "Autorin/Buch/buch.m4b")
	en := "en"
	if err := e.svc.SetItemOriginalLanguage(ctx, idBookFolder, &en); err != nil {
		t.Fatal(err)
	}
	var lang, locks string
	if err := e.db.Read.QueryRowContext(ctx, `SELECT original_language, locked_fields FROM items WHERE id = ?`, idBookFolder).Scan(&lang, &locks); err != nil ||
		lang != "en" || locks != `["original_language"]` {
		t.Fatalf("after setting: %q %s (%v)", lang, locks, err)
	}
	if err := e.svc.SetItemOriginalLanguage(ctx, idBookFolder, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Refresh(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	var matches, probes int
	if err := e.db.Read.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE type = ?), count(*) FILTER (WHERE type = ?) FROM jobs`,
		JobMatch, JobProbeFile).Scan(&matches, &probes); err != nil || matches != 0 || probes != 1 {
		t.Errorf("jobs after lifting the language and a refresh: %d matches, %d probes (%v), want 0 and 1", matches, probes, err)
	}
	if err := e.db.Read.QueryRowContext(ctx, `SELECT locked_fields FROM items WHERE id = ?`, idBookFolder).Scan(&locks); err != nil || locks != `[]` {
		t.Errorf("locks after lifting: %s (%v)", locks, err)
	}
}
