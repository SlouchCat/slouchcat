package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Podcast folders (ADR 0015, concept 4.5): a root with content podcast holds one Audiobookshelf
// folder per podcast, each with its audio files, a metadata.json (title, author, description,
// genres, explicit, feedURL, imageURL, language) and a cover image. Every first-level folder is a
// podcast, every audio file in it an episode with a file; files directly in the root are passed
// over (a podcast is always a folder). The feed named in metadata.json is adopted (F-6): a
// subscription of the library with the same address takes the episodes, otherwise the podcast is
// created with the feed and feed_origin 'file', and the fetch runs like for any subscription.
//
// The tags of the files are only known after the probe (like for audiobooks); finishPodcastEpisode
// in probe.go reads them and pairs the file with a feed episode when exactly one fits.

// podcastMeta is what the folder of a podcast says about it.
type podcastMeta struct {
	title, author, description string
	lang                       string // de or en, empty when unknown
	genres                     []string
	explicit                   bool
	feedURL                    string // the feedURL of metadata.json, as written
	cover                      string // rel path of the folder image below the root
}

// podcastFolderKey names the podcast of an audio file in a root: its first-level folder.
func podcastFolderKey(rootID int64, rel string) string {
	first, _, ok := strings.Cut(rel, "/")
	if !ok {
		return "" // a file directly in the root is no episode
	}
	return titleFolderKey(rootID, first)
}

// podcastFiles reads the folder of a podcast: metadata.json with the same size cap as for books
// and the folder image. The folder name is the title when the file has none.
func podcastFiles(r *os.Root, dir string) podcastMeta {
	m := podcastMeta{title: path.Base(dir)}
	entries, err := fs.ReadDir(r.FS(), dir)
	if err != nil {
		return m
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && !Ignored(e.Name()) {
			names = append(names, e.Name())
		}
	}
	for _, want := range coverNames {
		if i := slices.IndexFunc(names, func(n string) bool { return strings.EqualFold(n, want) }); i >= 0 {
			m.cover = path.Join(dir, names[i])
			break
		}
	}
	if i := slices.IndexFunc(names, func(n string) bool { return strings.EqualFold(n, "metadata.json") }); i >= 0 {
		name := path.Join(dir, names[i])
		if fi, err := r.Stat(name); err == nil && fi.Mode().IsRegular() && fi.Size() <= maxMetadataJSON {
			if data, err := r.ReadFile(name); err == nil {
				if pm, ok := podcastJSON(data); ok {
					if pm.title == "" {
						pm.title = m.title
					}
					pm.cover = m.cover
					return pm
				}
			}
		}
	}
	return m
}

// podcastJSON parses the metadata.json of a podcast folder (Audiobookshelf's podcast schema:
// author is a string, feedURL and imageURL are URLs); ok is false for a file that does not parse.
func podcastJSON(data []byte) (podcastMeta, bool) {
	var raw struct {
		Title       string            `json:"title"`
		Author      json.RawMessage   `json:"author"`
		Description string            `json:"description"`
		Genres      []json.RawMessage `json:"genres"`
		FeedURL     string            `json:"feedURL"`
		Language    string            `json:"language"`
		Explicit    json.RawMessage   `json:"explicit"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return podcastMeta{}, false
	}
	m := podcastMeta{
		title: strings.TrimSpace(raw.Title), description: strings.TrimSpace(raw.Description),
		lang: language(raw.Language), feedURL: strings.TrimSpace(raw.FeedURL),
	}
	if name, _ := entryName(raw.Author); name != "" {
		m.author = name
	} else {
		m.author = joinNames(splitList(raw.Author))
	}
	for _, g := range raw.Genres {
		if n, _ := entryName(g); n != "" {
			m.genres = append(m.genres, n)
		}
	}
	var explicit any
	if json.Unmarshal(raw.Explicit, &explicit) == nil {
		switch v := explicit.(type) {
		case bool:
			m.explicit = v
		case string:
			m.explicit = strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
		}
	}
	return m, true
}

// splitList reads a JSON list; anything else is the empty list.
func splitList(raw json.RawMessage) []json.RawMessage {
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	return list
}

// podcastCache holds the folder metadata read in one batch, by folder key.
type podcastCache map[string]*podcastMeta

// describePodcastFile reads the folder metadata of a new audio file of a podcast folder.
func describePodcastFile(r *os.Root, rootID int64, d diskFile, cache podcastCache) fileInfo {
	key := podcastFolderKey(rootID, d.rel)
	m, ok := cache[key]
	if !ok {
		first, _, _ := strings.Cut(d.rel, "/")
		pm := podcastFiles(r, first)
		m = &pm
		cache[key] = m
	}
	return fileInfo{folder: key, podcast: m}
}

// feedFromFile validates a feedURL read from a file (concept 9.1): http or https, no credentials,
// and never a host in the local network -- the LAN exception of package netguard is for the
// administrator, not for a file on the NAS. It returns the cleaned URL, or the reason it was
// refused ("" when it is usable).
func feedFromFile(ctx context.Context, raw string) (feedURL, refused string) {
	if raw == "" {
		return "", ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", "feed_url_not_allowed"
	}
	if netguard.LocalHost(ctx, u.String()) {
		return "", "feed_url_not_allowed: " + netguard.Site(u.String())
	}
	return u.String(), ""
}

// podcastItem returns the podcast of a new audio file of a podcast folder: the podcast that owns
// the folder (a soft-deleted one comes back), else the subscription of the library with the feed
// named in metadata.json (adopted: its episodes hang on it from now on), else a new podcast from
// the folder. Then the episode of the file: named after the file until its probe reads the tags,
// dated by the file's mtime. It returns the episode id.
func (w *batchWriter) podcastItem(ctx context.Context, op scanOp, info fileInfo) (string, error) {
	if info.folder == "" {
		return "", nil // a file directly in the root
	}
	m := info.podcast
	if m == nil {
		m = &podcastMeta{title: path.Base(path.Dir(op.disk.rel))}
	}
	podcastID := w.ix().byFolder[info.folder]
	if podcastID != "" && w.ix().kinds[podcastID] == "podcast" {
		if err := w.restore(ctx, podcastID, false); err != nil {
			return "", err
		}
	} else {
		id, err := w.findOrCreatePodcast(ctx, info.folder, m)
		if err != nil {
			return "", err
		}
		podcastID = id
	}
	return w.podcastEpisode(ctx, podcastID, op)
}

// findOrCreatePodcast adopts the library's subscription with the feed of the folder or creates
// the podcast from the folder's metadata.
func (w *batchWriter) findOrCreatePodcast(ctx context.Context, folder string, m *podcastMeta) (string, error) {
	feedURL, refused := feedFromFile(ctx, m.feedURL)
	if feedURL != "" {
		id, err := w.q.PodcastFindByFeed(ctx, systemq.PodcastFindByFeedParams{LibraryID: w.lib.ID, FeedUrl: nullString(feedURL)})
		switch {
		case err == nil:
			// Adopted (F-6): the subscription made by hand or the podcast of another folder with
			// the same feed takes the episodes; a second folder with one feed is noted on the
			// podcast for the administration.
			if m.cover != "" {
				if err := w.q.PodcastSetCoverPath(ctx, systemq.PodcastSetCoverPathParams{CoverPath: nullString(m.cover), Now: w.ms, ID: id}); err != nil {
					return "", fmt.Errorf("set podcast cover: %w", err)
				}
			}
			roots, err := w.q.PodcastCountRootsOfPodcast(ctx, nullString(id))
			if err != nil {
				return "", fmt.Errorf("count folders of podcast: %w", err)
			}
			if roots >= 1 && !w.index.created[id] {
				w.adopted = append(w.adopted, id)
			}
			w.ix().kinds[id] = "podcast"
			w.ix().addFolder(id, folder)
			return id, nil
		case !errors.Is(err, sql.ErrNoRows):
			return "", fmt.Errorf("find podcast by feed: %w", err)
		}
	}
	genres, err := genresJSON(m.genres)
	if err != nil {
		return "", err
	}
	rating, source := bookRating(w.lib, m.explicit)
	id := w.newID()
	origin := sql.NullString{}
	if feedURL != "" {
		origin = nullString(netguard.FeedOriginFile)
	}
	if _, err := w.q.PodcastInsertFromFolder(ctx, systemq.PodcastInsertFromFolderParams{
		ID: id, LibraryID: w.lib.ID, Title: m.title, SortTitle: SortTitle(m.title), Author: nullString(m.author), Genres: genres,
		Explicit: boolInt(m.explicit), CoverPath: nullString(m.cover), FeedUrl: nullString(feedURL), FeedOrigin: origin,
		FeedError: nullString(refused), OwnRating: rating, RatingSource: source, Now: sql.NullInt64{Int64: w.ms, Valid: true},
	}); err != nil {
		return "", fmt.Errorf("insert podcast: %w", err)
	}
	if m.description != "" {
		lang := m.lang
		if lang == "" {
			lang = "en"
		}
		for _, l := range []string{lang, "en"} {
			if err := w.q.UpsertItemText(ctx, systemq.UpsertItemTextParams{ItemID: id, Lang: l, Title: m.title, Overview: nullString(m.description)}); err != nil {
				return "", fmt.Errorf("insert podcast text: %w", err)
			}
		}
	}
	if m.cover != "" {
		if err := enqueueTx(ctx, w.q, w.now, imageJob(id)); err != nil {
			return "", err
		}
	}
	w.index.created[id] = true
	w.ix().kinds[id] = "podcast"
	w.ix().addFolder(id, folder)
	w.stats.items++
	return id, nil
}

// podcastEpisode creates the episode of a new file below its podcast. The name is the file's stem
// until the probe reads the tags; the date is the file's mtime. Episodes of a podcast that
// existed before this scan are announced as new (WithNewEpisodes); those of a podcast the scan
// created are not -- the first scan of a folder with thousands of files is no news.
func (w *batchWriter) podcastEpisode(ctx context.Context, podcastID string, op scanOp) (string, error) {
	title := strings.TrimSuffix(path.Base(op.disk.rel), path.Ext(op.disk.rel))
	id := w.newID()
	if err := w.q.PodcastInsertFolderEpisode(ctx, systemq.PodcastInsertFolderEpisodeParams{
		ID: id, ParentID: nullString(podcastID), LibraryID: w.lib.ID, Title: title, SortTitle: SortTitle(title),
		PublishedAt: sql.NullInt64{Int64: op.disk.mtime, Valid: op.disk.mtime > 0},
		Now:         sql.NullInt64{Int64: w.ms, Valid: true},
	}); err != nil {
		return "", fmt.Errorf("insert podcast episode: %w", err)
	}
	w.ix().kinds[id] = "podcast_episode"
	w.ix().parent[id] = podcastID
	w.stats.items++
	if !w.index.created[podcastID] {
		w.episodes = append(w.episodes, id)
	}
	return id, nil
}

// noteAdoptedPodcasts records on a podcast that a second folder brought the same feed (the
// Podcasts page shows feed_error until the next fetch clears it) and logs it; after the batch.
func (s *Service) noteAdoptedPodcasts(ctx context.Context, libraryID int64, ids []string) {
	for _, id := range slices.Compact(slices.Sorted(slices.Values(ids))) {
		s.log.WarnContext(ctx, "two folders share one podcast feed", "library_id", libraryID, "item_id", id)
		if err := systemq.New(s.db.Write).PodcastSetFeedError(ctx, systemq.PodcastSetFeedErrorParams{
			FeedError: nullString("two_folders_one_feed"), Now: s.now().UnixMilli(), ID: id,
		}); err != nil {
			s.log.WarnContext(ctx, "note shared feed", "item_id", id, "err", err)
		}
	}
}
