package library

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // registers JPEG for image.DecodeConfig (local poster.jpg)
	_ "image/png"  // registers PNG for image.DecodeConfig (local logo.png)
	"io"
	"os"
	"path"
	"strings"

	"encoding/binary"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Local image files (concept 10 section 4.6, ADR 0018): the scanner, which walks every folder
// anyway, lists the image files next to the media (Emby's and Kodi's names) and writes them to
// item_local_images with the dimensions from the file header and size/mtime for change
// detection. metadata.fill reads the table as the source "local"; no job touches the share for
// that. Everything is read through the root of the library folder (CLAUDE.md rule 4): no "..",
// no symlink out of the root, only the names below, only the header of a file.
//
// Assignment (4.6):
//
//	film, the only film in its folder   poster > folder; fanart > backdrop; logo > clearlogo
//	film among several in one folder    <stem>-poster, <stem>-fanart, <stem>-logo only
//	series (folder of tvshow.nfo)       poster > folder; fanart > backdrop; logo > clearlogo
//	season                              seasonNN-poster in the series folder, else poster/folder
//	                                    in the season folder
//	episode                             <stem>-thumb (its still, kind backdrop)
//
// landscape.jpg (Emby's thumb) is not read (no thumb, concept 1.3). Extensions: jpg, jpeg, png,
// webp.

// localImageExts are the accepted extensions of a local image file (metadata's imageExt).
var localImageExts = map[string]bool{"jpg": true, "jpeg": true, "png": true, "webp": true}

// localImageNames are the generic names of a title folder per kind, in order of precedence.
var localImageNames = map[string][]string{
	"poster":   {"poster", "folder"},
	"backdrop": {"fanart", "backdrop"},
	"logo":     {"logo", "clearlogo"},
}

// maxLocalImageHeader bounds the bytes read for the dimensions of a local image.
const maxLocalImageHeader = 64 << 10

// isLocalImage reports whether a file name is a candidate image file (collected by walkRoot).
func isLocalImage(name string) bool {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	return localImageExts[ext] && !Ignored(name)
}

// localImage is one image file assigned to a title.
type localImage struct {
	itemID string
	kind   string
	rel    string
}

// stemOf is a file's path without extension.
func stemOf(rel string) string { return strings.TrimSuffix(rel, path.Ext(rel)) }

// pickNamed returns the first file of dir whose stem (lower case) is one of names, in the order
// of names; "" when none. imgs lists the image files of dir (names only).
func pickNamed(dir string, imgs []string, names []string) string {
	for _, want := range names {
		for _, f := range imgs {
			if strings.ToLower(stemOf(f)) == want {
				return path.Join(dir, f)
			}
		}
	}
	return ""
}

// assignLocalImages maps the image files of a root (imgs: dir -> file names, as walkRoot saw
// them) onto the titles of its media files: one file per title and kind at most. content is the
// root's content.
func assignLocalImages(content string, files []systemq.ListRootFilesWithItemsRow, imgs map[string][]string) []localImage {
	var out []localImage
	seen := map[string]bool{} // item/kind done
	add := func(itemID, kind, rel string) {
		if rel == "" || seen[itemID+"/"+kind] {
			return
		}
		seen[itemID+"/"+kind] = true
		out = append(out, localImage{itemID: itemID, kind: kind, rel: rel})
	}
	switch content {
	case model.RootSeries:
		for _, f := range files {
			if f.Kind != "episode" {
				continue
			}
			p := ParseSeriesPath(f.RelPath)
			if p.Series == nil {
				continue
			}
			dir := path.Dir(f.RelPath)
			stem := strings.ToLower(path.Base(stemOf(f.RelPath)))
			add(f.ItemID, "backdrop", pickNamed(dir, imgs[dir], []string{stem + "-thumb"}))
			ep := p.Series
			seriesDir := ep.SeriesDir
			if f.ParentID.Valid && seriesDir != "" {
				// Season: seasonNN-poster in the series folder, else poster/folder in the season folder.
				seasonID := f.ParentID.String
				name := fmt.Sprintf("season%02d-poster", ep.Season)
				if ep.Season == 0 {
					name = "season-specials-poster"
				}
				rel := pickNamed(seriesDir, imgs[seriesDir], []string{name})
				if rel == "" && ep.SeasonDir != "" {
					rel = pickNamed(ep.SeasonDir, imgs[ep.SeasonDir], localImageNames["poster"])
				}
				add(seasonID, "poster", rel)
			}
		}
	default: // movie
		perDir := map[string]int{} // distinct films per folder (the versions of one film count once)
		filmsIn := map[string]map[string]bool{}
		for _, f := range files {
			if f.Kind == "movie" {
				folder := movieFolder(f.RelPath)
				if filmsIn[folder] == nil {
					filmsIn[folder] = map[string]bool{}
				}
				if !filmsIn[folder][f.ItemID] {
					filmsIn[folder][f.ItemID] = true
					perDir[folder]++
				}
			}
		}
		for _, f := range files {
			if f.Kind != "movie" {
				continue
			}
			folder := movieFolder(f.RelPath)
			if folder == "" {
				// A film directly in the root: only its own <stem>-* files.
				folder = path.Dir(f.RelPath)
				perDir[folder] = 2
			}
			dir := path.Dir(f.RelPath)
			stem := strings.ToLower(path.Base(stemOf(f.RelPath)))
			for kind, names := range localImageNames {
				var rel string
				if perDir[folder] == 1 {
					rel = pickNamed(folder, imgs[folder], names)
				}
				if rel == "" {
					rel = pickNamed(dir, imgs[dir], []string{stem + "-" + names[0]})
				}
				add(f.ItemID, kind, rel)
			}
		}
	}
	return out
}

// seriesLocalImages assigns the generic files of the series folder (tvshow.nfo's folder) to the
// series items of a root: the folder of each series is the SeriesDir of its episodes.
func seriesLocalImages(files []systemq.ListRootFilesWithItemsRow, seriesOf map[string]string, imgs map[string][]string) []localImage {
	var out []localImage
	done := map[string]bool{}
	for _, f := range files {
		if f.Kind != "episode" || !f.ParentID.Valid {
			continue
		}
		seriesID := seriesOf[f.ParentID.String]
		if seriesID == "" || done[seriesID] {
			continue
		}
		p := ParseSeriesPath(f.RelPath)
		if p.Series == nil || p.Series.SeriesDir == "" {
			continue
		}
		done[seriesID] = true
		for kind, names := range localImageNames {
			if rel := pickNamed(p.Series.SeriesDir, imgs[p.Series.SeriesDir], names); rel != "" {
				out = append(out, localImage{itemID: seriesID, kind: kind, rel: rel})
			}
		}
	}
	return out
}

// imageHeader reads width and height of an image file through the root, from its header only.
func imageHeader(r *os.Root, rel string) (w, h int, size, mtime int64, err error) {
	f, err := r.Open(rel)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return 0, 0, 0, 0, fmt.Errorf("not a regular file: %s", rel)
	}
	head, err := io.ReadAll(io.LimitReader(f, maxLocalImageHeader))
	if err != nil {
		return 0, 0, 0, 0, err
	}
	var cfg image.Config
	if strings.EqualFold(path.Ext(rel), ".webp") {
		cfg, err = webpConfig(head)
	} else {
		cfg, _, err = image.DecodeConfig(bytes.NewReader(head))
	}
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("decode header of %s: %w", rel, err)
	}
	return cfg.Width, cfg.Height, fi.Size(), fi.ModTime().UnixMilli(), nil
}

// webpConfig reads the dimensions of a WebP file from its RIFF header: VP8 (lossy), VP8L
// (lossless) or VP8X (extended) chunk, the three container forms of the format.
func webpConfig(b []byte) (image.Config, error) {
	if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return image.Config{}, fmt.Errorf("not a webp file")
	}
	switch string(b[12:16]) {
	case "VP8 ":
		if len(b) < 30 {
			break
		}
		w := int(binary.LittleEndian.Uint16(b[26:28]) & 0x3fff)
		h := int(binary.LittleEndian.Uint16(b[28:30]) & 0x3fff)
		return image.Config{Width: w, Height: h}, nil
	case "VP8L":
		if len(b) < 25 || b[20] != 0x2f {
			break
		}
		bits := binary.LittleEndian.Uint32(b[21:25])
		return image.Config{Width: int(bits&0x3fff) + 1, Height: int((bits>>14)&0x3fff) + 1}, nil
	case "VP8X":
		if len(b) < 30 {
			break
		}
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
		return image.Config{Width: w + 1, Height: h + 1}, nil
	}
	return image.Config{}, fmt.Errorf("unknown webp header")
}

// syncLocalImages brings item_local_images of a root in line with the image files the walk saw
// (rs.imgs): new and changed files are read (header only), rows of vanished files are deleted. A
// scope (path scan) limits the work to the listed directories. The metadata.fill of a title picks
// the rows up on its next run; this sync queues nothing by itself.
func (s *Service) syncLocalImages(ctx context.Context, rs *rootScan, scope *dirScope) error {
	if rs.imgs == nil {
		return nil
	}
	sq := systemq.New(s.db.Read)
	files, err := sq.ListRootFilesWithItems(ctx, rs.root.ID)
	if err != nil {
		return fmt.Errorf("list files of root %d: %w", rs.root.ID, err)
	}
	have, err := sq.ListLocalImagesByRoot(ctx, rs.root.ID)
	if err != nil {
		return fmt.Errorf("list local images of root %d: %w", rs.root.ID, err)
	}
	want := assignLocalImages(rs.root.Content, files, rs.imgs)
	if rs.root.Content == model.RootSeries {
		seriesOf := map[string]string{}
		seasons := map[string]bool{}
		for _, f := range files {
			if f.Kind == "episode" && f.ParentID.Valid {
				seasons[f.ParentID.String] = true
			}
		}
		if len(seasons) > 0 {
			rows, err := sq.ListItemParents(ctx, keys(seasons))
			if err != nil {
				return fmt.Errorf("list seasons of root %d: %w", rs.root.ID, err)
			}
			for _, r := range rows {
				seriesOf[r.ID] = r.ParentID.String
			}
		}
		want = append(want, seriesLocalImages(files, seriesOf, rs.imgs)...)
	}
	inScope := func(rel string) bool { return scope == nil || scope.listed[path.Dir(rel)] }

	r, err := paths.Open(rs.proot)
	if err != nil {
		return err
	}
	defer r.Close()
	existing := map[string]systemq.ItemLocalImage{}
	for _, h := range have {
		existing[h.ItemID+"/"+h.Kind+"/"+h.RelPath] = h
	}
	wanted := map[string]bool{}
	var upserts []systemq.UpsertItemLocalImageParams
	for _, w := range want {
		if !inScope(w.rel) {
			continue
		}
		key := w.itemID + "/" + w.kind + "/" + w.rel
		wanted[key] = true
		width, height, size, mtime, err := imageHeader(r, w.rel)
		if err != nil {
			s.log.WarnContext(ctx, "local image not readable, skipped", "root_id", rs.root.ID, "path", w.rel, "err", err)
			continue
		}
		if h, ok := existing[key]; ok && h.Size == size && h.Mtime == mtime {
			continue
		}
		upserts = append(upserts, systemq.UpsertItemLocalImageParams{ItemID: w.itemID, Kind: w.kind, RelPath: w.rel,
			RootID: rs.root.ID, Width: int64(width), Height: int64(height), Size: size, Mtime: mtime})
	}
	var deletes []systemq.DeleteItemLocalImageParams
	for key, h := range existing {
		if !wanted[key] && inScope(h.RelPath) {
			deletes = append(deletes, systemq.DeleteItemLocalImageParams{ItemID: h.ItemID, Kind: h.Kind, RelPath: h.RelPath})
		}
	}
	if len(upserts) == 0 && len(deletes) == 0 {
		return nil
	}
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	for _, u := range upserts {
		if err := q.UpsertItemLocalImage(ctx, u); err != nil {
			return fmt.Errorf("write local image: %w", err)
		}
	}
	for _, d := range deletes {
		if err := q.DeleteItemLocalImage(ctx, d); err != nil {
			return fmt.Errorf("delete local image: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit local images: %w", err)
	}
	s.log.InfoContext(ctx, "local images synchronised", "root_id", rs.root.ID, "written", len(upserts), "removed", len(deletes))
	return nil
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
