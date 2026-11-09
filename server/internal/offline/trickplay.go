package offline

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Trickplay of a download (concept 18, 6.8): the preparation copies the complete preview set of the
// file into <dir>/<id>/trickplay/ (manifest.json and the sheets), so the offline budget counts it
// and the maintenance of the server cache cannot take it away. The manifest of the download names
// the sheets with the download token; getDownloadTrickplaySheet serves them.

// trickplayDir is the folder of the preview set inside the directory of a download.
const trickplayDir = "trickplay"

// OpenTrickplaySheet opens sheet n of the preview set of a grant (getDownloadTrickplaySheet); a
// sheet the download does not carry is model.ErrNotFound.
func (s *Service) OpenTrickplaySheet(ctx context.Context, g model.DownloadGrant, n int) (model.StreamSource, error) {
	if _, err := s.prepared(ctx, g.DownloadID); err != nil {
		return model.StreamSource{}, err
	}
	if n < 0 {
		return model.StreamSource{}, model.ErrNotFound
	}
	name := library.TrickplaySheetName(n)
	return s.open(g.DownloadID+"/"+trickplayDir+"/"+name, "image/webp", name, model.ErrNotFound)
}

// downloadTrickplay reads the preview set copied into download id (nil without one, or with an
// unreadable manifest) with the sheet URLs of the download token.
func downloadTrickplay(root *os.Root, id, token string) *model.Trickplay {
	b, err := root.ReadFile(id + "/" + trickplayDir + "/manifest.json")
	if err != nil {
		return nil
	}
	var m library.TrickplayManifest
	if json.Unmarshal(b, &m) != nil || m.Columns <= 0 || m.Rows <= 0 {
		return nil
	}
	f, err := root.Open(id + "/" + trickplayDir)
	if err != nil {
		return nil
	}
	names, err := f.Readdirnames(0)
	_ = f.Close()
	if err != nil {
		return nil
	}
	var sheets []int
	for _, name := range names {
		base, ok := strings.CutSuffix(name, ".webp")
		if n, err := strconv.Atoi(base); ok && err == nil && n >= 0 && n < m.Sheets() && strconv.Itoa(n) == base {
			sheets = append(sheets, n)
		}
	}
	slices.Sort(sheets)
	out := &model.Trickplay{
		Version: m.Version, State: library.TrickplayComplete, IntervalMs: m.IntervalMs, TileWidth: m.TileWidth,
		TileHeight: m.TileHeight, Columns: m.Columns, Rows: m.Rows, PointsMs: m.PointsMs,
		Sheets: make([]model.TrickplaySheet, 0, len(sheets)),
	}
	for _, n := range sheets {
		out.Sheets = append(out.Sheets, model.TrickplaySheet{
			Index: n, URL: apiBase + "/downloads/" + id + "/trickplay/" + strconv.Itoa(n) + "?dt=" + token,
		})
	}
	return out
}
