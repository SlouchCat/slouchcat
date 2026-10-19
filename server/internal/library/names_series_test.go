package library

import (
	"fmt"
	"reflect"
	"testing"
)

// seriesCase is one path of the series corpus. year 0 means "no year"; skip means the file is no
// episode and no extra with a parent (ParsedName.Series == nil).
type seriesCase struct {
	path        string
	series      string
	year        int
	tvdb, tmdb  int64
	imdb        string
	season      int
	episode     int
	end         int
	title       string
	date        string
	extra       string // extra kind; "" for an episode
	seasonLevel bool   // extra below a season folder
	skip        bool
}

// seriesCorpus covers Sonarr, Plex, Jellyfin, scene and daily naming, specials, multi-episode
// files, umlauts and extras, plus the placeholder tree of testdata/generate-library-tree.py and
// the generated test media.
var seriesCorpus = []seriesCase{
	// Sonarr standard and Plex.
	{path: "Breaking Bad (2008)/Season 01/Breaking Bad (2008) - S01E01 - Pilot.mkv", series: "Breaking Bad", year: 2008, season: 1, episode: 1, title: "Pilot"},
	{path: "Breaking Bad (2008)/Season 01/Breaking Bad - S01E02 - Cat's in the Bag... - 1080p.mkv", series: "Breaking Bad", year: 2008, season: 1, episode: 2, title: "Cat's in the Bag..."},
	{path: "Breaking Bad (2008) {tvdb-81189}/Season 02/Breaking Bad (2008) - S02E03 - Bit by a Dead Bee [WEBDL-1080p][EAC3 5.1][h264]-NTb.mkv", series: "Breaking Bad", year: 2008, tvdb: 81189, season: 2, episode: 3, title: "Bit by a Dead Bee"},
	{path: "The Office (US) (2005) [tvdb-73244]/Season 03/The Office (US) (2005) - S03E01 - Gay Witch Hunt.mkv", series: "The Office (US)", year: 2005, tvdb: 73244, season: 3, episode: 1, title: "Gay Witch Hunt"},
	{path: "Dark (2017) [tmdb-70523]/Season 1/Dark (2017) - S01E01 - Secrets.mkv", series: "Dark", year: 2017, tmdb: 70523, season: 1, episode: 1, title: "Secrets"},
	{path: "Dark (2017) {imdb-tt5753856}/Season 3/Dark - S03E08 - The Paradise.mkv", series: "Dark", year: 2017, imdb: "tt5753856", season: 3, episode: 8, title: "The Paradise"},
	{path: "Severance (2022) {tvdb-371980} {tmdb-95396}/Season 01/Severance - S01E01 - Good News About Hell.mkv", series: "Severance", year: 2022, tvdb: 371980, tmdb: 95396, season: 1, episode: 1, title: "Good News About Hell"},
	{path: "Show (2020)/Season 01/Show (2020) - s01e01 - Title.mkv", series: "Show", year: 2020, season: 1, episode: 1, title: "Title"},
	{path: "Show (2020)/Season 01/Show (2020) - s01e01.mkv", series: "Show", year: 2020, season: 1, episode: 1},
	{path: "Show (2020)/Season 01/s01e05.mkv", series: "Show", year: 2020, season: 1, episode: 5},
	{path: "Show (2020)/Season 01/S1E5 - Title.mkv", series: "Show", year: 2020, season: 1, episode: 5, title: "Title"},
	{path: "Show (2020)/Season 01/Show S01 E05 Title.mkv", series: "Show", year: 2020, season: 1, episode: 5, title: "Title"},
	{path: "Show (2020)/Season 10/Show - S10E105 - Long Run.mkv", series: "Show", year: 2020, season: 10, episode: 105, title: "Long Run"},
	{path: "Show (2020)/Staffel 2/Show - S02E01 - Neuanfang.mkv", series: "Show", year: 2020, season: 2, episode: 1, title: "Neuanfang"},
	{path: "Show (2020)/Saison 3/Show - S03E01.mkv", series: "Show", year: 2020, season: 3, episode: 1},
	{path: "Show (2020)/S01/Show - S01E01.mkv", series: "Show", year: 2020, season: 1, episode: 1},
	{path: "Show (2020)/Show - S01E01 - No Season Folder.mkv", series: "Show", year: 2020, season: 1, episode: 1, title: "No Season Folder"},
	{path: "Show (2020)/Season 01/Show - S01E01 - Part 1 - The Beginning.mkv", series: "Show", year: 2020, season: 1, episode: 1, title: "Part 1 - The Beginning"},
	{path: "Show (2020)/Season 01/Show - S01E01 - Title {edition-Extended}.mkv", series: "Show", year: 2020, season: 1, episode: 1, title: "Title"},
	{path: "Doctor Who (2005)/Season 01/Doctor Who (2005) - S01E01 - Rose (1).mkv", series: "Doctor Who", year: 2005, season: 1, episode: 1, title: "Rose (1)"},
	{path: "S.W.A.T. (2017)/Season 01/S.W.A.T. (2017) - S01E01 - Pilot.mkv", series: "S.W.A.T.", year: 2017, season: 1, episode: 1, title: "Pilot"},
	{path: "Marvel's Agents of S.H.I.E.L.D. (2013)/Season 01/Marvel's Agents of S.H.I.E.L.D. - S01E01 - Pilot.mkv", series: "Marvel's Agents of S.H.I.E.L.D.", year: 2013, season: 1, episode: 1, title: "Pilot"},
	{path: "Law & Order (1990)/Season 01/Law & Order - S01E01 - Prescription for Death.mkv", series: "Law & Order", year: 1990, season: 1, episode: 1, title: "Prescription for Death"},
	{path: "Mr. Robot (2015)/Season 01/Mr. Robot - S01E01 - eps1.0_hellofriend.mov.mkv", series: "Mr. Robot", year: 2015, season: 1, episode: 1, title: "eps1.0 hellofriend.mov"},
	{path: "Lost/Season 1/Lost - S01E01 - Pilot (1).avi", series: "Lost", season: 1, episode: 1, title: "Pilot (1)"},
	{path: "TV/Lost/Season 1/Lost - S01E02.mkv", series: "Lost", season: 1, episode: 2},
	{path: "24 (2001)/Season 1/24 - S01E01 - 12-00 A.M.-1-00 A.M..mkv", series: "24", year: 2001, season: 1, episode: 1, title: "12-00 A.M.-1-00 A.M."},
	{path: "1883 (2021)/Season 01/1883 - S01E01 - 1883.mkv", series: "1883", year: 2021, season: 1, episode: 1, title: "1883"},

	// Multi-episode files.
	{path: "Show (2020)/Season 01/Show - S01E01-E02 - Double.mkv", series: "Show", year: 2020, season: 1, episode: 1, end: 2, title: "Double"},
	{path: "Show (2020)/Season 01/Show - S01E01E02.mkv", series: "Show", year: 2020, season: 1, episode: 1, end: 2},
	{path: "Show (2020)/Season 01/Show - S01E01-02 - Double.mkv", series: "Show", year: 2020, season: 1, episode: 1, end: 2, title: "Double"},
	{path: "Show (2020)/Season 01/Show - S01E01-E02-E03.mkv", series: "Show", year: 2020, season: 1, episode: 1, end: 3},
	{path: "Show (2020)/Season 01/Show.S01E03E04.720p.HDTV.x264-GRP.mkv", series: "Show", year: 2020, season: 1, episode: 3, end: 4},
	{path: "Show (2020)/Season 01/Show - S01E01 - 1080p.mkv", series: "Show", year: 2020, season: 1, episode: 1},
	{path: "Show (2020)/Season 01/Show - S01E09-1080p.mkv", series: "Show", year: 2020, season: 1, episode: 9},
	{path: "Show (2020)/Season 01/Show - 1x01-02 - Double.mkv", series: "Show", year: 2020, season: 1, episode: 1, end: 2, title: "Double"},

	// 1x02 and words.
	{path: "Friends/Season 1/Friends - 1x01 - The One Where Monica Gets a Roommate.avi", series: "Friends", season: 1, episode: 1, title: "The One Where Monica Gets a Roommate"},
	{path: "Friends/Season 10/Friends 10x17.avi", series: "Friends", season: 10, episode: 17},
	{path: "Show (2020)/Show - Season 2 Episode 3 - Title.mkv", series: "Show", year: 2020, season: 2, episode: 3, title: "Title"},
	{path: "Show.Name.1x05.HDTV.XviD-LOL.avi", series: "Show Name", season: 1, episode: 5},

	// E-only markers: season from the folder, else 1.
	{path: "Show (2020)/Season 02/E05 - Title.mkv", series: "Show", year: 2020, season: 2, episode: 5, title: "Title"},
	{path: "Show (2020)/Season 02/Show - Ep05.mkv", series: "Show", year: 2020, season: 2, episode: 5},
	{path: "Show (2020)/Season 03/Episode 07.mkv", series: "Show", year: 2020, season: 3, episode: 7},
	{path: "Mini (2019)/Mini - E03 - Finale.mkv", series: "Mini", year: 2019, season: 1, episode: 3, title: "Finale"},

	// Specials.
	{path: "Show (2020)/Specials/Show - S00E01 - Christmas.mkv", series: "Show", year: 2020, season: 0, episode: 1, title: "Christmas"},
	{path: "Show (2020)/Season 00/Show - S00E02.mkv", series: "Show", year: 2020, season: 0, episode: 2},
	{path: "Show (2020)/Season 0/Show - S00E03.mkv", series: "Show", year: 2020, season: 0, episode: 3},
	{path: "Show (2020)/Specials/E04.mkv", series: "Show", year: 2020, season: 0, episode: 4},
	{path: "Show (2020)/Show - S00E05 - Behind It.mkv", series: "Show", year: 2020, season: 0, episode: 5, title: "Behind It"},

	// Daily shows.
	{path: "The Daily Show (1996)/Season 2024/The Daily Show - 2024-03-15 - Guest Name.mkv", series: "The Daily Show", year: 1996, season: 2024, episode: 315, title: "Guest Name", date: "2024-03-15"},
	{path: "The Daily Show (1996)/The Daily Show 2024.11.05 Election Night 720p WEB h264-GRP.mkv", series: "The Daily Show", year: 1996, season: 2024, episode: 1105, title: "Election Night", date: "2024-11-05"},
	{path: "Tagesschau/Tagesschau 2023-12-31.mp4", series: "Tagesschau", season: 2023, episode: 1231, date: "2023-12-31"},
	{path: "Late.Show.2025.01.20.Guest.1080p.WEB.h264-GRP.mkv", series: "Late Show", season: 2025, episode: 120, title: "Guest", date: "2025-01-20"},

	// Scene names and release folders.
	{path: "The.Expanse.S02E05.1080p.WEB-DL.DD5.1.H264-GRP.mkv", series: "The Expanse", season: 2, episode: 5},
	{path: "The.Expanse.S02E05.Home.1080p.WEB-DL.DD5.1.H264-GRP.mkv", series: "The Expanse", season: 2, episode: 5, title: "Home"},
	{path: "The Expanse/Season 2/The.Expanse.S02E06.720p.HDTV.x264-KILLERS.mkv", series: "The Expanse", season: 2, episode: 6},
	{path: "The Expanse/Season 2/The.Expanse.S02E07.720p.HDTV.x264-KILLERS/the.expanse.s02e07.720p.hdtv.x264-killers.mkv", series: "The Expanse", season: 2, episode: 7},
	{path: "The Expanse/Season 2/The.Expanse.S02E08.720p.HDTV.x264-KILLERS/abc123.mkv", series: "The Expanse", season: 2, episode: 8},
	{path: "Game.of.Thrones.S08E06.The.Iron.Throne.2160p.UHD.BluRay.x265-GRP/Game.of.Thrones.S08E06.The.Iron.Throne.2160p.UHD.BluRay.x265-GRP.mkv", series: "Game of Thrones", season: 8, episode: 6, title: "The Iron Throne"},
	{path: "Westworld.2016.S01E01.1080p.BluRay.x264-ROVERS.mkv", series: "Westworld", year: 2016, season: 1, episode: 1},
	{path: "Babylon.Berlin.S01E01.German.DL.1080p.WEB.x264-GRP.mkv", series: "Babylon Berlin", season: 1, episode: 1},
	{path: "show_name_s01e01_title.mkv", series: "show name", season: 1, episode: 1, title: "title"},
	{path: "Show Name - S01E01.mkv", series: "Show Name", season: 1, episode: 1},
	{path: "Show Name (2019) - S01E01 - Pilot.mp4", series: "Show Name", year: 2019, season: 1, episode: 1, title: "Pilot"},

	// Umlauts and accents.
	{path: "Türkisch für Anfänger (2006)/Staffel 1/Türkisch für Anfänger - S01E01 - Lena.mkv", series: "Türkisch für Anfänger", year: 2006, season: 1, episode: 1, title: "Lena"},
	{path: "Der Bergdoktor (2008)/Staffel 01/Der Bergdoktor - S01E01 - Folge Eins.mkv", series: "Der Bergdoktor", year: 2008, season: 1, episode: 1, title: "Folge Eins"},
	{path: "Bärgli & Söhne (2010)/Season 01/Bärgli & Söhne - S01E02 - Grüezi.mkv", series: "Bärgli & Söhne", year: 2010, season: 1, episode: 2, title: "Grüezi"},
	{path: "Café Élysée (2015)/Season 01/Café Élysée - S01E01.mkv", series: "Café Élysée", year: 2015, season: 1, episode: 1},
	{path: "Señor Øresund (2005) {tvdb-123456}/Season 02/Señor Øresund (2005) - S02E10 - Fjord.mkv", series: "Señor Øresund", year: 2005, tvdb: 123456, season: 2, episode: 10, title: "Fjord"},
	{path: "Die Märchenbraut (1979)/Staffel 01/Die Märchenbraut - S01E05.avi", series: "Die Märchenbraut", year: 1979, season: 1, episode: 5},

	// Placeholder tree and generated test media.
	{path: "Burning Kingdom (2003) {tvdb-128168}/Season 01/Burning Kingdom (2003) - S01E02 - Exit (I).mkv", series: "Burning Kingdom", year: 2003, tvdb: 128168, season: 1, episode: 2, title: "Exit (I)"},
	{path: "Orchard (1994)/Specials/Orchard (1994) - S00E01 - Ghosts.mkv", series: "Orchard", year: 1994, season: 0, episode: 1, title: "Ghosts"},
	{path: "Garden - Part I (2006) {tvdb-419631}/Season 05/Garden - Part I (2006) - S05E20 - Homecoming (II).mkv", series: "Garden - Part I", year: 2006, tvdb: 419631, season: 5, episode: 20, title: "Homecoming (II)"},
	{path: "Slouchcat Test Show (2024)/Season 01/Slouchcat Test Show (2024) S01E01.mp4", series: "Slouchcat Test Show", year: 2024, season: 1, episode: 1},
	{path: "Slouchcat Test Show (2024)/Season 01/Slouchcat Test Show (2024) S01E03-E04.mp4", series: "Slouchcat Test Show", year: 2024, season: 1, episode: 3, end: 4},
	{path: "Slouchcat Test Show (2024)/Specials/Slouchcat Test Show (2024) S00E01.mp4", series: "Slouchcat Test Show", year: 2024, season: 0, episode: 1},

	// Extras.
	{path: "Show (2020)/Trailers/Show Trailer.mkv", series: "Show", year: 2020, extra: "trailer"},
	{path: "Show (2020)/Featurettes/Making Of.mkv", series: "Show", year: 2020, extra: "featurette"},
	{path: "Show (2020)/Behind The Scenes/On Set.mkv", series: "Show", year: 2020, extra: "behind_the_scenes"},
	{path: "Show (2020)/Interviews/Cast.mkv", series: "Show", year: 2020, extra: "interview"},
	{path: "Show (2020)/Deleted Scenes/Cut.mkv", series: "Show", year: 2020, extra: "deleted_scene"},
	{path: "Show (2020)/Extras/Bonus.mkv", series: "Show", year: 2020, extra: "other"},
	{path: "Show (2020)/Season 01/Featurettes/Season Recap.mkv", series: "Show", year: 2020, season: 1, extra: "featurette", seasonLevel: true},
	{path: "Show (2020)/Season 02/Trailers/Season 2 Trailer.mkv", series: "Show", year: 2020, season: 2, extra: "trailer", seasonLevel: true},
	{path: "Show (2020)/Season 01/Show - S01E01-trailer.mkv", series: "Show", year: 2020, season: 1, extra: "trailer", seasonLevel: true},
	{path: "Show (2020)/Show (2020)-trailer.mkv", series: "Show", year: 2020, extra: "trailer"},
	{path: "Show (2020)/Specials/Shorts/Short One.mkv", series: "Show", year: 2020, season: 0, extra: "short", seasonLevel: true},
	{path: "Show (2020) {tvdb-99}/Scenes/Best Scene.mkv", series: "Show", year: 2020, tvdb: 99, extra: "scene"},

	// No episode: skipped.
	{path: "Show (2020)/Season 01/Show - Recap.mkv", skip: true},
	{path: "Show (2020)/Some Movie.mkv", skip: true},
	{path: "Trailers/Show Trailer.mkv", skip: true},
	{path: "Random File.mkv", skip: true},
	{path: "Show-trailer.mkv", skip: true},
}

func TestParseSeriesPathCorpus(t *testing.T) {
	if len(seriesCorpus) < 80 {
		t.Fatalf("corpus has %d paths, want at least 80", len(seriesCorpus))
	}
	matched := 0
	for _, c := range seriesCorpus {
		got := ParseSeriesPath(c.path)
		var g seriesCase
		g.path = c.path
		if got.Series == nil {
			g.skip = true
		} else {
			e := got.Series
			g.series, g.tvdb, g.tmdb, g.imdb = e.SeriesTitle, e.TVDBID, e.TMDBID, e.IMDBID
			if e.SeriesYear != nil {
				g.year = *e.SeriesYear
			}
			g.extra = got.ExtraKind
			if got.Extra {
				g.seasonLevel = e.SeasonDir != ""
				if g.seasonLevel {
					g.season = e.Season
				}
			} else {
				g.season, g.episode, g.end, g.title, g.date = e.Season, e.Episode, e.EpisodeEnd, e.Title, e.Date
			}
		}
		if g == c {
			matched++
			continue
		}
		t.Logf("mismatch %q:\n  got  %+v\n  want %+v", c.path, g, c)
	}
	ratio := float64(matched) / float64(len(seriesCorpus))
	msg := fmt.Sprintf("series name parser corpus: %d/%d exact matches (%.1f %%)", matched, len(seriesCorpus), ratio*100)
	t.Log(msg)
	if ratio < 0.98 {
		t.Fatal(msg + ", want at least 98 %")
	}
}

func TestParseSeriesPathDetails(t *testing.T) {
	p := ParseSeriesPath("Show (2020) {tvdb-1}/Season 01/Show - S01E01 - Pilot - 2160p {edition-Extended}.mkv")
	if p.Series == nil || p.VersionLabel != "2160p Extended" || p.Series.SeriesDir != "Show (2020) {tvdb-1}" ||
		p.Series.SeasonDir != "Show (2020) {tvdb-1}/Season 01" {
		t.Errorf("parsed = %+v, series %+v", p, p.Series)
	}
	x := ParseSeriesPath("Show (2020)/Season 01/Show - S01E01-trailer.mkv")
	if !x.Extra || x.ExtraTitle != "Show - S01E01" || x.ExtraKind != "trailer" {
		t.Errorf("extra = %+v", x)
	}
	if s := ParseSeriesPath("Show (2020)/Season 01/Sample/show-s01e01-sample.mkv"); !s.Sample {
		t.Errorf("sample not detected: %+v", s)
	}
	if r := ParseSeriesPath("Show.S01E01.mkv"); r.Series == nil || r.Series.SeriesDir != "" || r.Series.SeriesTitle != "Show" {
		t.Errorf("root file = %+v", r)
	}
}

func TestExtraKinds(t *testing.T) {
	cases := map[string]struct{ kind, title string }{
		"Alien (1979)/Trailers/Teaser.mkv":                 {"trailer", "Teaser"},
		"Alien (1979)/Featurettes/Making Of.mkv":           {"featurette", "Making Of"},
		"Alien (1979)/Deleted Scenes/Facehugger.mkv":       {"deleted_scene", "Facehugger"},
		"Alien (1979)/Behind The Scenes/On Set.mkv":        {"behind_the_scenes", "On Set"},
		"Alien (1979)/Interviews/Ridley.mkv":               {"interview", "Ridley"},
		"Alien (1979)/Scenes/Chestburster.mkv":             {"scene", "Chestburster"},
		"Alien (1979)/Shorts/Short.mkv":                    {"short", "Short"},
		"Alien (1979)/Others/Gag Reel.mkv":                 {"other", "Gag Reel"},
		"Alien (1979)/Alien (1979)-trailer.mkv":            {"trailer", "Alien (1979)"},
		"Alien (1979)/Alien (1979) - trailer.mkv":          {"trailer", "Alien (1979)"},
		"Alien (1979)/Alien-deletedscene.mkv":              {"deleted_scene", "Alien"},
		"Alien (1979)/Alien-behindthescenes.mkv":           {"behind_the_scenes", "Alien"},
		"Alien (1979)/trailer.mp4":                         {"trailer", "trailer"},
		"Alien (1979)/Trailers/Alien (1979)-trailer.mp4":   {"trailer", "Alien (1979)"},
		"Alien (1979)/Featurettes/Making Of-interview.mkv": {"featurette", "Making Of"},
	}
	for p, want := range cases {
		got := ParseMoviePath(p)
		if !got.Extra || got.ExtraKind != want.kind || got.ExtraTitle != want.title {
			t.Errorf("ParseMoviePath(%q) = extra %v kind %q title %q, want %q %q", p, got.Extra, got.ExtraKind, got.ExtraTitle, want.kind, want.title)
		}
	}
	folders := map[string]string{
		"Alien (1979)/Trailers/Teaser.mkv":      "Alien (1979)",
		"Alien (1979)/Alien (1979)-trailer.mkv": "Alien (1979)",
		"Alien (1979)/Alien (1979).mkv":         "Alien (1979)",
		"Dune (2021)/CD1/Dune.mkv":              "Dune (2021)",
		"Trailers/Teaser.mkv":                   "",
		"Alien (1979).mkv":                      "",
	}
	for p, want := range folders {
		if got := movieFolder(p); got != want {
			t.Errorf("movieFolder(%q) = %q, want %q", p, got, want)
		}
	}
}

// Sonarr/Kodi/Jellyfin tvshow.nfo (shortened).
const tvshowNFO = `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<tvshow>
  <title>Dark</title>
  <originaltitle>Dark</originaltitle>
  <plot>Ein Kind verschwindet …</plot>
  <mpaa>DE-16</mpaa>
  <premiered>2017-12-01</premiered>
  <uniqueid type="tvdb" default="true">334824</uniqueid>
  <uniqueid type="tmdb">70523</uniqueid>
  <uniqueid type="imdb">tt5753856</uniqueid>
  <genre>Drama</genre>
  <season>-1</season>
  <episode>-1</episode>
  <actor><name>Louis Hofmann</name><role>Jonas</role></actor>
</tvshow>`

const episodeNFO = `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<episodedetails>
  <title>Geheimnisse</title>
  <season>1</season>
  <episode>1</episode>
  <aired>2017-12-01</aired>
  <plot>Mikkel verschwindet.</plot>
  <mpaa>FSK 12</mpaa>
  <uniqueid type="tvdb">6252148</uniqueid>
</episodedetails>`

func TestParseSeriesNFO(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   NFO
		rating *int
	}{
		{"tvshow sonarr", tvshowNFO, NFO{
			Kind: NFOTVShow, Title: "Dark", OriginalTitle: "Dark", Year: 2017, Plot: "Ein Kind verschwindet …",
			MPAA: "DE-16", Premiered: "2017-12-01", TVDBID: 334824, TMDBID: 70523, IMDBID: "tt5753856",
			Genres: []string{"Drama"},
		}, ip(16)},
		{"episodedetails", episodeNFO, NFO{
			Kind: NFOEpisode, Title: "Geheimnisse", Season: ip(1), Episode: ip(1), Aired: "2017-12-01", Year: 2017,
			Plot: "Mikkel verschwindet.", MPAA: "FSK 12", TVDBID: 6252148,
		}, ip(12)},
		{"tvshow year field", `<tvshow><title>Lost</title><year>2004</year><premiered>2004-09-22</premiered></tvshow>`,
			NFO{Kind: NFOTVShow, Title: "Lost", Year: 2004, Premiered: "2004-09-22"}, nil},
		{"tvshow certification", `<tvshow><title>Dark</title><certification>Germany:16 / US:TV-MA</certification></tvshow>`,
			NFO{Kind: NFOTVShow, Title: "Dark", Certification: "Germany:16 / US:TV-MA"}, ip(16)},
		{"tvshow legacy tvdbid", `<tvshow><title>Lost</title><tvdbid>73739</tvdbid></tvshow>`,
			NFO{Kind: NFOTVShow, Title: "Lost", TVDBID: 73739}, nil},
		{"tvshow us rating only", `<tvshow><mpaa>TV-14</mpaa></tvshow>`, NFO{Kind: NFOTVShow, MPAA: "TV-14"}, nil},
		{"episode specials", `<episodedetails><title>Weihnachten</title><season>0</season><episode>3</episode></episodedetails>`,
			NFO{Kind: NFOEpisode, Title: "Weihnachten", Season: ip(0), Episode: ip(3)}, nil},
		{"multi-episode nfo reads the first", `<episodedetails><title>Teil 1</title><episode>1</episode></episodedetails>
<episodedetails><title>Teil 2</title><episode>2</episode></episodedetails>`,
			NFO{Kind: NFOEpisode, Title: "Teil 1", Episode: ip(1)}, nil},
		{"episode invalid aired", `<episodedetails><aired>gestern</aired><season>x</season></episodedetails>`,
			NFO{Kind: NFOEpisode}, nil},
		{"episode with bom and junk", "\xEF\xBB\xBFjunk\n<episodedetails><title>Pilot</title></episodedetails>",
			NFO{Kind: NFOEpisode, Title: "Pilot"}, nil},
		{"tvshow truncated", `<tvshow><title>Dark</title><year>20`, NFO{Kind: NFOTVShow, Title: "Dark"}, nil},
		{"season.nfo root", `<season><title>Staffel 1</title></season>`, NFO{}, nil},
	}
	if len(cases) < 10 {
		t.Fatalf("%d series NFO cases, want at least 10", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseNFO([]byte(c.in))
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ParseNFO =\n  %+v\nwant\n  %+v", got, c.want)
			}
			r := got.GermanRating()
			switch {
			case r == nil && c.rating == nil:
			case r == nil || c.rating == nil || *r != *c.rating:
				t.Errorf("GermanRating = %v, want %v", deref(r), deref(c.rating))
			}
		})
	}
}
