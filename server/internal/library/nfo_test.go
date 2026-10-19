package library

import (
	"reflect"
	"testing"
)

// Jellyfin/Radarr NFO as written into the owner's library (shortened, with BOM).
const jellyfinNFO = "\xEF\xBB\xBF" + `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<movie>
  <plot>Etwa zehn Jahre nachdem sie es mit dem mächtigen Mutanten Apocalypse aufgenommen haben …</plot>
  <lockdata>false</lockdata>
  <dateadded>2020-04-16 18:14:22</dateadded>
  <title>X-Men: Dark Phoenix</title>
  <originaltitle>Dark Phoenix</originaltitle>
  <trailer>plugin://plugin.video.youtube/play/?video_id=Af2euSk-oeo</trailer>
  <rating>5.964</rating>
  <year>2019</year>
  <mpaa>FSK-12</mpaa>
  <imdbid>tt6565702</imdbid>
  <tmdbid>320288</tmdbid>
  <premiered>2019-06-05</premiered>
  <releasedate>2019-06-05</releasedate>
  <runtime>114</runtime>
  <tagline>Ein Phönix erhebt sich, die X-Men zu vernichten.</tagline>
  <genre>Abenteuer</genre>
  <genre>Action</genre>
  <genre>Science Fiction</genre>
  <studio />
  <art>
    <poster>/mnt/medien/Filme/Dark Phoenix (2019) [imdb-tt6565702]/folder.jpg</poster>
  </art>
  <id>tt6565702</id>
  <set><name>X-Men Filmreihe</name></set>
  <fileinfo><streamdetails><video><codec>hevc</codec></video></streamdetails></fileinfo>
</movie>`

// Kodi v19 NFO with <uniqueid>.
const kodiNFO = `<?xml version="1.0" encoding="UTF-8" standalone="yes" ?>
<movie>
    <title>Alien</title>
    <originaltitle>Alien</originaltitle>
    <year>1979</year>
    <plot>In deep space, the crew of the commercial starship Nostromo is awakened …</plot>
    <runtime>117</runtime>
    <mpaa>Rated R</mpaa>
    <certification>US:R / Germany:16</certification>
    <uniqueid type="imdb">tt0078748</uniqueid>
    <uniqueid type="tmdb" default="true">348</uniqueid>
    <genre>Horror / Science Fiction</genre>
    <premiered>1979-05-25</premiered>
</movie>`

func ip(n int) *int { return &n }

func TestParseNFO(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   NFO
		rating *int
	}{
		{"jellyfin radarr", jellyfinNFO, NFO{
			Title: "X-Men: Dark Phoenix", OriginalTitle: "Dark Phoenix", Year: 2019,
			Plot:    "Etwa zehn Jahre nachdem sie es mit dem mächtigen Mutanten Apocalypse aufgenommen haben …",
			Tagline: "Ein Phönix erhebt sich, die X-Men zu vernichten.", Runtime: 114, Premiered: "2019-06-05",
			MPAA: "FSK-12", TMDBID: 320288, IMDBID: "tt6565702", Genres: []string{"Abenteuer", "Action", "Science Fiction"},
		}, ip(12)},
		{"kodi uniqueid", kodiNFO, NFO{
			Title: "Alien", OriginalTitle: "Alien", Year: 1979,
			Plot:    "In deep space, the crew of the commercial starship Nostromo is awakened …",
			Runtime: 117, Premiered: "1979-05-25", MPAA: "Rated R", Certification: "US:R / Germany:16",
			TMDBID: 348, IMDBID: "tt0078748", Genres: []string{"Horror", "Science Fiction"},
		}, ip(16)},
		{"uniqueid without type is imdb", `<movie><uniqueid>tt0133093</uniqueid></movie>`, NFO{IMDBID: "tt0133093"}, nil},
		{"uniqueid overrides legacy tmdbid", `<movie><tmdbid>1</tmdbid><uniqueid type="tmdb">603</uniqueid></movie>`, NFO{TMDBID: 603}, nil},
		{"legacy id field", `<movie><id>tt0111161</id><tmdbid>278</tmdbid></movie>`, NFO{IMDBID: "tt0111161", TMDBID: 278}, nil},
		{"empty file", ``, NFO{}, nil},
		{"only whitespace", "  \n\t ", NFO{}, nil},
		{"url nfo", "https://www.themoviedb.org/movie/603-the-matrix\n", NFO{}, nil},
		{"url after xml", "<movie><title>Heat</title><year>1995</year></movie>\nhttps://www.imdb.com/title/tt0113277/", NFO{Title: "Heat", Year: 1995}, nil},
		{"junk before xml", "garbage line\n<movie><title>Heat</title></movie>", NFO{Title: "Heat"}, nil},
		{"binary junk", "\x00\x01\x02\xff\xfe", NFO{}, nil},
		{"truncated xml", "<movie><title>Heat</title><year>19", NFO{Title: "Heat"}, nil},
		{"other root ignored", "<musicvideo><title>Dark</title></musicvideo>", NFO{}, nil},
		{"latin1 declared", `<?xml version="1.0" encoding="ISO-8859-1"?><movie><title>Heat</title></movie>`, NFO{}, nil},
		{"html entities", `<movie><title>Fast &amp; Furious</title><plot>Caf&eacute;</plot></movie>`, NFO{Title: "Fast & Furious", Plot: "Café"}, nil},
		{"cdata", `<movie><plot><![CDATA[A <b>bold</b> plot]]></plot></movie>`, NFO{Plot: "A <b>bold</b> plot"}, nil},
		{"invalid year and runtime", `<movie><year>19x9</year><runtime>abc</runtime></movie>`, NFO{}, nil},
		{"runtime with unit", `<movie><runtime>98 min</runtime></movie>`, NFO{Runtime: 98}, nil},
		{"invalid imdb id", `<movie><imdbid>12345</imdbid></movie>`, NFO{}, nil},
		{"fsk with space", `<movie><mpaa>FSK 16</mpaa></movie>`, NFO{MPAA: "FSK 16"}, ip(16)},
		{"de prefix", `<movie><mpaa>DE:18</mpaa></movie>`, NFO{MPAA: "DE:18"}, ip(18)},
		{"germany prefix", `<movie><certification>Germany:6</certification></movie>`, NFO{Certification: "Germany:6"}, ip(6)},
		{"fsk glued", `<movie><mpaa>FSK16</mpaa></movie>`, NFO{MPAA: "FSK16"}, ip(16)},
		{"ab", `<movie><mpaa>ab 12</mpaa></movie>`, NFO{MPAA: "ab 12"}, ip(12)},
		{"fsk ab jahren", `<movie><mpaa>FSK ab 16 Jahren</mpaa></movie>`, NFO{MPAA: "FSK ab 16 Jahren"}, ip(16)},
		{"fsk 0", `<movie><mpaa>FSK 0</mpaa></movie>`, NFO{MPAA: "FSK 0"}, ip(0)},
		{"invalid german level", `<movie><mpaa>FSK 14</mpaa></movie>`, NFO{MPAA: "FSK 14"}, nil},
		{"us rating only", `<movie><mpaa>PG-13</mpaa></movie>`, NFO{MPAA: "PG-13"}, nil},
		{"bare number is not german", `<movie><mpaa>16</mpaa></movie>`, NFO{MPAA: "16"}, nil},
		{"uk rating", `<movie><certification>UK:15</certification></movie>`, NFO{Certification: "UK:15"}, nil},
	}
	if len(cases) < 20 {
		t.Fatalf("%d NFO cases, want at least 20", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseNFO([]byte(c.in))
			// Every case is a movie NFO; the kind is checked once here, the fields below.
			if got.Kind != "" && got.Kind != NFOMovie {
				t.Errorf("Kind = %q, want movie or empty", got.Kind)
			}
			got.Kind = ""
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

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
