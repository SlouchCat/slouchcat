package metadata

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The cover choice of an audiobook (concept 19 D5.3, PD-2): the folder images and the embedded
// picture are the candidates, square as the files are; a chosen one stays after the next scan
// (writeBook queues image.fetch with a changed folder cover), and lifting the choice brings the
// cover of the files back.
func TestBookCoverChoice(t *testing.T) {
	t.Parallel()
	media, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "media", "Audiobooks"))
	if err != nil {
		t.Fatal(err)
	}
	m4b := filepath.Join(media, "Slouchcat Test Author", "Slouchcat Test Audio Series", "01 - Slouchcat Test Audiobook M4B",
		"Slouchcat Test Audiobook M4B.m4b")
	data, err := os.ReadFile(m4b)
	if err != nil {
		t.Fatalf("test media missing (run `task testdata:media` in the repository root): %v", err)
	}
	root := t.TempDir()
	book := filepath.Join(root, "Autorin", "Buch")
	for name, content := range map[string][]byte{"buch.m4b": data, "cover.jpg": jpegBytes(t, 500, 500), "Rueckseite.jpg": jpegBytes(t, 400, 400),
		"notes.txt": []byte("no image"), ".hidden.jpg": jpegBytes(t, 10, 10)} {
		if err := os.MkdirAll(book, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(book, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e := audioEnv(t, root)
	ctx := context.Background()
	const fileID = "01JTESTFILE0000000000000CV"
	e.addAudio(t, idBookFolder, "audiobook", 3, nil, "Autorin/Buch/cover.jpg", nil)
	e.addMediaFile(t, fileID, idBookFolder, "Autorin/Buch/buch.m4b")
	mustExec(t, e.db.Write, `UPDATE media_files SET probe = '{"streams":[{"index":0,"type":"audio","codec":"aac"},
		{"index":1,"type":"video","codec":"mjpeg","width":600,"height":600,"attachedPic":true}]}' WHERE id = ?`, fileID)

	kinds, err := e.svc.ChoosableImageKinds(ctx, idBookFolder)
	if err != nil || !slices.Equal(kinds, []string{"poster"}) {
		t.Fatalf("kinds %v (%v), want poster", kinds, err)
	}
	list, err := e.svc.ImageCandidates(ctx, idBookFolder, "poster", false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range list.Candidates {
		mark := ""
		if c.Chosen {
			mark = "*"
		}
		got = append(got, c.Provider+":"+unversioned(c.Source)+":"+itoa(c.Width)+"x"+itoa(c.Height)+mark)
	}
	want := []string{"local:file:Autorin/Buch/cover.jpg:500x500*", "local:file:Autorin/Buch/Rueckseite.jpg:400x400",
		"embedded:embedded:" + fileID + ":600x600"}
	if !slices.Equal(got, want) {
		t.Fatalf("candidates\n  %v\nwant\n  %v", got, want)
	}
	if err := e.svc.fetchImages(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	e.checkCover(t, idBookFolder, "file:Autorin/Buch/cover.jpg?", 500, 500)

	// The previews: a folder image and the embedded picture of the candidates, nothing else.
	for _, c := range list.Candidates {
		src, err := e.svc.LocalImagePreview(ctx, idBookFolder, c.Source)
		if err != nil {
			t.Fatalf("preview of %s: %v", c.Source, err)
		}
		head, _ := io.ReadAll(src.File)
		_ = src.File.Close()
		if src.MediaType != "image/jpeg" || http.DetectContentType(head) != "image/jpeg" {
			t.Errorf("preview of %s: %s, %s", c.Source, src.MediaType, http.DetectContentType(head))
		}
	}
	for _, bad := range []string{"file:Autorin/Buch/notes.txt?1-1", "file:Autorin/Buch/.hidden.jpg?1-1", "file:../x.jpg?1-1",
		"embedded:01JTESTFILE0000000000000XX?1"} {
		if _, err := e.svc.LocalImagePreview(ctx, idBookFolder, bad); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("preview of %s: %v, want not found", bad, err)
		}
	}

	// Choose the back: image.fetch takes it, and a scan that changed the folder cover keeps it.
	back := list.Candidates[1]
	if err := e.svc.SetItemImage(ctx, idBookFolder, "poster", back.Provider, back.Source); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	e.checkCover(t, idBookFolder, "file:Autorin/Buch/Rueckseite.jpg?", 400, 400)
	if err := os.WriteFile(filepath.Join(book, "cover.jpg"), jpegBytes(t, 300, 300), 0o600); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(book, "cover.jpg"), later, later); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	e.checkCover(t, idBookFolder, "file:Autorin/Buch/Rueckseite.jpg?", 400, 400)
	imgs, err := e.svc.ItemImages(ctx, idBookFolder)
	if err != nil || len(imgs) != 1 || !imgs[0].Chosen || !imgs[0].Locked || imgs[0].Source != back.Source {
		t.Fatalf("item images %+v (%v), want the chosen back", imgs, err)
	}

	// The embedded picture, then back to the cover of the files.
	emb := list.Candidates[2]
	if err := e.svc.SetItemImage(ctx, idBookFolder, "poster", emb.Provider, emb.Source); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	e.checkCover(t, idBookFolder, "embedded:"+fileID+"?", 600, 600)
	if err := e.svc.ClearItemImage(ctx, idBookFolder, "poster"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.match(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := e.db.Read.QueryRowContext(ctx, `SELECT metadata_state FROM items WHERE id = ?`, idBookFolder).Scan(&state); err != nil || state != "matched" {
		t.Errorf("state after the match of a book: %q (%v), want matched", state, err)
	}
	if err := e.svc.fetchImages(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	e.checkCover(t, idBookFolder, "file:Autorin/Buch/cover.jpg?", 300, 300)
	imgs, err = e.svc.ItemImages(ctx, idBookFolder)
	if err != nil || len(imgs) != 1 || imgs[0].Chosen || imgs[0].Locked || imgs[0].Provider != ImageSourceLocal ||
		!strings.HasPrefix(imgs[0].Source, "file:Autorin/Buch/cover.jpg?") {
		t.Fatalf("item images %+v (%v), want the cover of the files", imgs, err)
	}

	// Another title's file beside the .m4b: the folder images are no longer the book's (the scan's
	// rule; parts of other formats own their folder).
	e.addAudio(t, idBookEmbedded, "audiobook", 3, nil, nil, nil)
	e.addMediaFile(t, "01JTESTFILE0000000000000CW", idBookEmbedded, "Autorin/Buch/zweites.m4b")
	list, err = e.svc.ImageCandidates(ctx, idBookFolder, "poster", true)
	if err != nil || len(list.Candidates) != 1 || list.Candidates[0].Provider != ImageSourceEmbedded {
		t.Errorf("candidates in a shared folder: %+v (%v), want only the embedded picture", list.Candidates, err)
	}
}
