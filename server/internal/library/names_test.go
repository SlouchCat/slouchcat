package library

import (
	"fmt"
	"testing"
)

// nameCase is one path of the parser corpus. year 0 means "no year".
type nameCase struct {
	path   string
	title  string
	year   int
	tmdb   int64
	imdb   string
	extra  bool
	sample bool
}

// movieCorpus covers Plex, Jellyfin, Radarr and scene naming, the placeholder tree of
// testdata/generate-library-tree.py and the real folders of the owner's test library.
var movieCorpus = []nameCase{
	// Plex/Radarr: one folder per movie.
	{path: "Alien (1979)/Alien (1979).mkv", title: "Alien", year: 1979},
	{path: "The Matrix (1999)/The Matrix (1999) - 1080p.mkv", title: "The Matrix", year: 1999},
	{path: "Inception (2010) [tmdb-27205]/Inception (2010) [tmdb-27205].mkv", title: "Inception", year: 2010, tmdb: 27205},
	{path: "Blade Runner 2049 (2017)/Blade Runner 2049 (2017).mkv", title: "Blade Runner 2049", year: 2017},
	{path: "1917 (2019)/1917 (2019) - 2160p.mkv", title: "1917", year: 2019},
	{path: "Apollo 13 (1995)/Apollo 13 (1995).mkv", title: "Apollo 13", year: 1995},
	{path: "It (2017)/It (2017).mkv", title: "It", year: 2017},
	{path: "Us (2019)/Us (2019) - 1080p.mkv", title: "Us", year: 2019},
	{path: "Her (2013)/Her (2013).mkv", title: "Her", year: 2013},
	{path: "Arrival (2016)/Arrival (2016) Bluray-1080p.mkv", title: "Arrival", year: 2016},
	{path: "Arrival (2016)/Arrival.2016.Bluray-1080p.mkv", title: "Arrival", year: 2016},
	{path: "Knives Out (2019) - 1080p/Knives Out (2019).mkv", title: "Knives Out", year: 2019},
	{path: "Movies/Alien (1979)/Alien (1979).mkv", title: "Alien", year: 1979},
	{path: "A/Alien (1979)/Alien (1979).mkv", title: "Alien", year: 1979},
	{path: "Parasite (2019)/Parasite.2019.KOREAN.1080p.mkv", title: "Parasite", year: 2019},
	{path: "Frozen (2013)/Frozen.(2013).mkv", title: "Frozen", year: 2013},
	{path: "Up/Up (2009).mkv", title: "Up", year: 2009},
	{path: "Casablanca/Casablanca.mkv", title: "Casablanca"},
	{path: "Cars (2006)/Cars (2006) [tmdb-920].mkv", title: "Cars", year: 2006, tmdb: 920},

	// Owner's real library (Radarr with German DL releases, NFS share).
	{path: "Ford v Ferrari (2019) [imdb-tt1950186]/Ford v Ferrari (2019) [imdb-tt1950186] - [German DL][Bluray-1080p][DTS 5.1][x264]-COiNCiDENCE.mkv", title: "Ford v Ferrari", year: 2019, imdb: "tt1950186"},
	{path: "13 Minutes (2021) [tmdb-787723]/13 Minutes (2021) [imdb-tt9274670] - [Bluray-1080p][AC3 5.1][x265]-omikron.mkv", title: "13 Minutes", year: 2021, tmdb: 787723, imdb: "tt9274670"},
	{path: "Dangerous Animals (2025) [tmdb-1285965]/Dangerous Animals (2025) [imdb-tt32299316] - [German DL][Bluray-1080p][EAC3 5.1][x265]-VECTOR.mkv", title: "Dangerous Animals", year: 2025, tmdb: 1285965, imdb: "tt32299316"},
	{path: "Dark Phoenix (2019) [imdb-tt6565702]/Dark Phoenix (2019) [imdb-tt6565702] - [Bluray-1080p][AC3 5.1][x265]-FuN.mkv", title: "Dark Phoenix", year: 2019, imdb: "tt6565702"},
	{path: "Dead Poets Society (1989) [tmdb-207]/Dead Poets Society (1989) [imdb-tt0097165] - [Bluray-1080p][AC3 5.1][x265]-FX.mkv", title: "Dead Poets Society", year: 1989, tmdb: 207, imdb: "tt0097165"},
	{path: "Freaks Youre One of Us (2020) [tmdb-658777]/Freaks Youre One of Us (2020) [imdb-tt12875782] - [WEBDL-1080p][EAC3 Atmos 5.1][x264]-WAYNE.mkv", title: "Freaks Youre One of Us", year: 2020, tmdb: 658777, imdb: "tt12875782"},
	{path: "Hollow Man (2000) [imdb-tt0164052]/Hollow Man (2000) [imdb-tt0164052] - [German DL][Bluray-1080p][AC3 5.1][x264]-c0nFuSed.mkv", title: "Hollow Man", year: 2000, imdb: "tt0164052"},
	{path: "Horrible Bosses 2 (2014) [tmdb-227159]/Horrible Bosses 2 (2014) [imdb-tt2170439] - [Bluray-1080p][EAC3 5.1][h265]-WOTT.mkv", title: "Horrible Bosses 2", year: 2014, tmdb: 227159, imdb: "tt2170439"},
	{path: "Hot Shots! (1991) [imdb-tt0102059]/Hot Shots! (1991) [imdb-tt0102059] - [German DL][Bluray-1080p][DTS 5.1][x264]-DETAiLS.mkv", title: "Hot Shots!", year: 1991, imdb: "tt0102059"},
	{path: "How the Grinch Stole Christmas (2000) [imdb-tt0170016]/How the Grinch Stole Christmas (2000) [imdb-tt0170016] - [Bluray-1080p][EAC3 5.1][x265]-VECTOR.mkv", title: "How the Grinch Stole Christmas", year: 2000, imdb: "tt0170016"},
	{path: "Jonah Hex (2010) [imdb-tt1075747]/Jonah Hex (2010) [imdb-tt1075747] - [Bluray-1080p][AC3 5.1][x265]-FuN.mkv", title: "Jonah Hex", year: 2010, imdb: "tt1075747"},
	{path: "Jurassic Park (1993) [imdb-tt0107290]/Jurassic Park (1993) [imdb-tt0107290] - [Bluray-1080p][AC3 5.1][x265]-FuN.mkv", title: "Jurassic Park", year: 1993, imdb: "tt0107290"},

	// Jellyfin curly ids and editions.
	{path: "Avatar (2009) {tmdb-19995}/Avatar (2009) {tmdb-19995}.mkv", title: "Avatar", year: 2009, tmdb: 19995},
	{path: "Avatar (2009) {imdb-tt0499549}/Avatar (2009).mkv", title: "Avatar", year: 2009, imdb: "tt0499549"},
	{path: "Blade Runner (1982) {edition-Final Cut}/Blade Runner (1982) {edition-Final Cut} - 1080p.mkv", title: "Blade Runner", year: 1982},
	{path: "Apocalypse Now (1979) {tmdb-28}/Apocalypse Now (1979) {edition-Redux} {tmdb-28}.mkv", title: "Apocalypse Now", year: 1979, tmdb: 28},
	{path: "Aliens (1986) [tmdb-679]/Aliens (1986) {edition-Director's Cut} - 2160p.mkv", title: "Aliens", year: 1986, tmdb: 679},
	{path: "Dune (2021) [imdbid-tt1160419]/Dune (2021) [imdbid-tt1160419].mkv", title: "Dune", year: 2021, imdb: "tt1160419"},
	{path: "Dune (2021) [tmdbid-438631]/Dune (2021).mkv", title: "Dune", year: 2021, tmdb: 438631},
	{path: "Up (2009) [tmdb-14160]/Up (2009) [tmdb-14160] - [WEBDL-2160p].mkv", title: "Up", year: 2009, tmdb: 14160},
	{path: "Tenet (2020) [tmdb-577922]/Tenet (2020) [Bluray-2160p][HDR10][TrueHD Atmos 7.1][x265]-GROUP.mkv", title: "Tenet", year: 2020, tmdb: 577922},
	{path: "Brave (2012) {tmdb-62177} {imdb-tt1217209}/Brave (2012).mkv", title: "Brave", year: 2012, tmdb: 62177, imdb: "tt1217209"},
	{path: "Amélie (2001) [tmdb-194]/Amélie (2001) [tmdb-194] - 1080p.mkv", title: "Amélie", year: 2001, tmdb: 194},
	{path: "The Hobbit - An Unexpected Journey (2012)/The Hobbit - An Unexpected Journey (2012) {edition-Extended Edition} - 1080p.mkv", title: "The Hobbit - An Unexpected Journey", year: 2012},
	{path: "Coco (2017) [German DL]/Coco (2017) [German DL].mkv", title: "Coco", year: 2017},
	{path: "Der Untergang (2004)/Der Untergang (2004) - [German][Bluray-1080p].mkv", title: "Der Untergang", year: 2004},

	// Files directly in the root.
	{path: "Alien (1979).mkv", title: "Alien", year: 1979},
	{path: "The Dark Knight (2008) - 1080p.mkv", title: "The Dark Knight", year: 2008},
	{path: "Interstellar (2014) [imdb-tt0816692].mp4", title: "Interstellar", year: 2014, imdb: "tt0816692"},
	{path: "Casablanca.mkv", title: "Casablanca"},
	{path: "Metropolis - 1080p.mkv", title: "Metropolis"},
	{path: "Midsommar [2019].mkv", title: "Midsommar", year: 2019},
	{path: "Midsommar (2019) (Director's Cut).mkv", title: "Midsommar", year: 2019},
	{path: "Frozen.(2013).1080p.mkv", title: "Frozen", year: 2013},
	{path: "Barbie 2023 1080p WEBRip x264.mkv", title: "Barbie", year: 2023},
	{path: "Oppenheimer 2023 2160p.mkv", title: "Oppenheimer", year: 2023},
	{path: "Fight Club 1999 [1080p].mkv", title: "Fight Club", year: 1999},
	{path: "Movies/Gladiator.2000.1080p.mkv", title: "Gladiator", year: 2000},
	{path: "Action/Die Hard (1988).mkv", title: "Die Hard", year: 1988},

	// Scene names.
	{path: "Parasite.2019.1080p.BluRay.x264-GROUP.mkv", title: "Parasite", year: 2019},
	{path: "The.Lord.of.the.Rings.The.Fellowship.of.the.Ring.2001.EXTENDED.1080p.BluRay.x264-SiNNERS.mkv", title: "The Lord of the Rings The Fellowship of the Ring", year: 2001},
	{path: "Blade.Runner.2049.2017.2160p.UHD.BluRay.x265-TERMiNAL.mkv", title: "Blade Runner 2049", year: 2017},
	{path: "1917.2019.1080p.WEB-DL.DD5.1.H264-FGT.mkv", title: "1917", year: 2019},
	{path: "2001.A.Space.Odyssey.1968.1080p.BluRay.x264.mkv", title: "2001 A Space Odyssey", year: 1968},
	{path: "Mad_Max_Fury_Road_2015_720p.mkv", title: "Mad Max Fury Road", year: 2015},
	{path: "Joker.2019.German.DL.1080p.BluRay.x264-DETAiLS.mkv", title: "Joker", year: 2019},
	{path: "Die.Hard.1988.REMUX.1080p.mkv", title: "Die Hard", year: 1988},
	{path: "Amelie.2001.FRENCH.1080p.mkv", title: "Amelie", year: 2001},
	{path: "Toy.Story.4.2019.HDR.2160p.WEB.H265-GROUP.mkv", title: "Toy Story 4", year: 2019},
	{path: "Wonder.Woman.1984.2020.1080p.mkv", title: "Wonder Woman 1984", year: 2020},
	{path: "Movie.Title.2010.PROPER.720p.HDTV.x264.mkv", title: "Movie Title", year: 2010},
	{path: "Some.Film.1999.DVDRip.XviD-GRP.avi", title: "Some Film", year: 1999},
	{path: "Heat.1995.Directors.Cut.1080p.mkv", title: "Heat", year: 1995},
	{path: "Nosferatu.1080p.BluRay.x264.mkv", title: "Nosferatu"},
	{path: "Spider-Man.Into.the.Spider-Verse.2018.1080p.mkv", title: "Spider-Man Into the Spider-Verse", year: 2018},
	{path: "Tom.and.Jerry.2021.HDRip.XviD-EVO.avi", title: "Tom and Jerry", year: 2021},
	{path: "The.Irishman.2019.NF.WEBRip.1080p.mkv", title: "The Irishman", year: 2019},
	{path: "Once.Upon.a.Time.in.Hollywood.2019.1080p.mkv", title: "Once Upon a Time in Hollywood", year: 2019},
	{path: "Avengers.Endgame.2019.IMAX.2160p.mkv", title: "Avengers Endgame", year: 2019},
	{path: "No.Time.To.Die.2021.MULTi.2160p.mkv", title: "No Time To Die", year: 2021},
	{path: "Pulp_Fiction_1994.mkv", title: "Pulp Fiction", year: 1994},
	{path: "Godzilla.Minus.One.2023.JAPANESE.mkv", title: "Godzilla Minus One", year: 2023},
	{path: "Ocean's.Eleven.2001.1080p.mkv", title: "Ocean's Eleven", year: 2001},
	{path: "Avatar.2009.[tmdb-19995].1080p.mkv", title: "Avatar", year: 2009, tmdb: 19995},
	{path: "Inception.2010.1080p.BluRay.x264-SPARKS/Inception.2010.1080p.BluRay.x264-SPARKS.mkv", title: "Inception", year: 2010},
	{path: "The.Social.Network.2010.720p.BluRay.x264-GROUP/tsn-720p.mkv", title: "The Social Network", year: 2010},
	{path: "Dune.Part.Two.2024.2160p.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265-FLUX.mkv", title: "Dune Part Two", year: 2024},
	{path: "Everything.Everywhere.All.at.Once.2022.1080p.AMZN.WEB-DL.mkv", title: "Everything Everywhere All at Once", year: 2022},

	// Umlauts, apostrophes, commas, ampersands and punctuation.
	{path: "Das Boot (1981)/Das Boot (1981).mkv", title: "Das Boot", year: 1981},
	{path: "Die fabelhafte Welt der Amélie (2001)/Die fabelhafte Welt der Amélie (2001).mkv", title: "Die fabelhafte Welt der Amélie", year: 2001},
	{path: "Ein Mädchen aus Zürich (1955)/Ein Mädchen aus Zürich (1955).mkv", title: "Ein Mädchen aus Zürich", year: 1955},
	{path: "Schindler's List (1993)/Schindler's List (1993).mkv", title: "Schindler's List", year: 1993},
	{path: "Don't Breathe (2016)/Don't Breathe (2016) - 1080p.mkv", title: "Don't Breathe", year: 2016},
	{path: "Crouching Tiger, Hidden Dragon (2000)/Crouching Tiger, Hidden Dragon (2000).mkv", title: "Crouching Tiger, Hidden Dragon", year: 2000},
	{path: "Fast & Furious (2009)/Fast & Furious (2009).mkv", title: "Fast & Furious", year: 2009},
	{path: "Mr. & Mrs. Smith (2005)/Mr. & Mrs. Smith (2005).mkv", title: "Mr. & Mrs. Smith", year: 2005},
	{path: "S.W.A.T. (2003)/S.W.A.T. (2003).mkv", title: "S.W.A.T.", year: 2003},
	{path: "Léon The Professional (1994)/Léon The Professional (1994).mkv", title: "Léon The Professional", year: 1994},
	{path: "Ödipussi (1988)/Ödipussi (1988).mkv", title: "Ödipussi", year: 1988},
	{path: "WALL·E (2008)/WALL·E (2008).mkv", title: "WALL·E", year: 2008},
	{path: "Star Wars - Episode IV - A New Hope (1977)/Star Wars - Episode IV - A New Hope (1977).mkv", title: "Star Wars - Episode IV - A New Hope", year: 1977},
	{path: "Mission: Impossible (1996)/Mission: Impossible (1996).mkv", title: "Mission: Impossible", year: 1996},
	{path: "[REC] (2007)/[REC] (2007).mkv", title: "[REC]", year: 2007},
	{path: "(500) Days of Summer (2009)/(500) Days of Summer (2009).mkv", title: "(500) Days of Summer", year: 2009},
	{path: "Se7en (1995)/Se7en (1995).mkv", title: "Se7en", year: 1995},
	{path: "Kill Bill - Vol. 1 (2003)/Kill Bill - Vol. 1 (2003).mkv", title: "Kill Bill - Vol. 1", year: 2003},
	{path: "X-Men (2000)/X-Men (2000).mkv", title: "X-Men", year: 2000},
	{path: "Spider-Man Into the Spider-Verse (2018)/Spider-Man Into the Spider-Verse (2018).mkv", title: "Spider-Man Into the Spider-Verse", year: 2018},
	{path: "Ocean's Eleven (2001)/Ocean's Eleven (2001).mkv", title: "Ocean's Eleven", year: 2001},
	{path: "Alien³ (1992)/Alien³ (1992).mkv", title: "Alien³", year: 1992},
	{path: "Good Bye, Lenin! (2003)/Good Bye, Lenin! (2003).mp4", title: "Good Bye, Lenin!", year: 2003},
	{path: "Achtung, fertig, Charlie! (2003)/Achtung, fertig, Charlie! (2003).wmv", title: "Achtung, fertig, Charlie!", year: 2003},

	// German and Swiss titles with the other video containers.
	{path: "Lola rennt (1998)/Lola rennt (1998).avi", title: "Lola rennt", year: 1998},
	{path: "Das Leben der Anderen (2006)/Das Leben der Anderen (2006).m4v", title: "Das Leben der Anderen", year: 2006},
	{path: "Im Westen nichts Neues (2022)/Im Westen nichts Neues (2022) - 2160p.mkv", title: "Im Westen nichts Neues", year: 2022},
	{path: "Toni Erdmann (2016)/Toni Erdmann (2016).webm", title: "Toni Erdmann", year: 2016},
	{path: "Heidi (2015)/Heidi (2015).ts", title: "Heidi", year: 2015},
	{path: "Die Schweizermacher (1978)/Die Schweizermacher (1978).mpg", title: "Die Schweizermacher", year: 1978},
	{path: "Höhenfeuer (1985)/Höhenfeuer (1985).mpeg", title: "Höhenfeuer", year: 1985},
	{path: "Grounding (2006)/Grounding (2006).m2ts", title: "Grounding", year: 2006},
	{path: "Mein Name ist Eugen (2005)/Mein Name ist Eugen (2005).mov", title: "Mein Name ist Eugen", year: 2005},

	// Parts and multi-file movies.
	{path: "Dune (2021)/Dune (2021) - Part 1.mkv", title: "Dune", year: 2021},
	{path: "Dune Part Two (2024)/Dune Part Two (2024).mkv", title: "Dune Part Two", year: 2024},
	{path: "Harry Potter and the Deathly Hallows - Part 1 (2010)/Harry Potter and the Deathly Hallows - Part 1 (2010).mkv", title: "Harry Potter and the Deathly Hallows - Part 1", year: 2010},
	{path: "The Shadow - Part II (2003) {tmdb-12345}/The Shadow - Part II (2003).mkv", title: "The Shadow - Part II", year: 2003, tmdb: 12345},
	{path: "Lawrence of Arabia (1962)/Lawrence of Arabia (1962) - cd1.avi", title: "Lawrence of Arabia", year: 1962},
	{path: "Lawrence of Arabia (1962)/Lawrence of Arabia (1962) - cd2.avi", title: "Lawrence of Arabia", year: 1962},
	{path: "Gone with the Wind (1939)/CD1/Gone with the Wind (1939) cd1.avi", title: "Gone with the Wind", year: 1939},
	{path: "Gone with the Wind (1939)/Disc 2/Gone with the Wind (1939) cd2.avi", title: "Gone with the Wind", year: 1939},
	{path: "Movie.Name.2005.cd1.avi", title: "Movie Name", year: 2005},
	{path: "Old Film/Old Film - cd1.avi", title: "Old Film"},
	{path: "Old.Film.part1.avi", title: "Old Film"},

	// Several versions of one movie.
	{path: "Avatar (2009)/Avatar (2009) - 1080p.mkv", title: "Avatar", year: 2009},
	{path: "Avatar (2009)/Avatar (2009) - 2160p.mkv", title: "Avatar", year: 2009},
	{path: "Avatar (2009)/Avatar (2009) - 4K.mkv", title: "Avatar", year: 2009},

	// Extras (folders and suffixes); a top-level folder named like an extras folder is a category.
	{path: "Alien (1979)/Trailers/Alien Official Trailer.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Featurettes/Featurette 1.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Behind The Scenes/Making Of.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Deleted Scenes/Scene 1.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Interviews/Ridley.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Scenes/Chestburster.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Shorts/Short.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Extras/Bonus.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Others/Gag Reel.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Alien (1979)-trailer.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Alien-featurette.mp4", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Alien-short.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Alien-deleted.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Alien-behindthescenes.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Alien-interview.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Alien-scene.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/Alien-other.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/trailer.mp4", title: "Alien", year: 1979, extra: true},
	{path: "Alien (1979)/TRAILERS/Teaser.mkv", title: "Alien", year: 1979, extra: true},
	{path: "Shorts/Paperman (2012).mkv", title: "Paperman", year: 2012},
	{path: "Extras/Film (2000)/Film (2000).mkv", title: "Film", year: 2000},

	// Samples.
	{path: "Parasite.2019.1080p.BluRay.x264-GROUP/Sample/parasite.sample.mkv", title: "Parasite", year: 2019, sample: true},
	{path: "Movie (2010)/Movie (2010)-sample.mkv", title: "Movie", year: 2010, sample: true},
	{path: "Movie (2010)/sample.mkv", title: "Movie", year: 2010, sample: true},
	{path: "Movie (2010)/sample-movie.mkv", title: "Movie", year: 2010, sample: true},
	{path: "Inception.2010.1080p.BluRay.x264-SPARKS/sample-inception.mkv", title: "Inception", year: 2010, sample: true},

	// Placeholder tree (testdata/generate-library-tree.py).
	{path: "Movies/The Silent River (1987)/The Silent River (1987).mkv", title: "The Silent River", year: 1987},
	{path: "Movies/Crimson Empire (2003) {tmdb-554433}/Crimson Empire (2003) - 2160p.mkv", title: "Crimson Empire", year: 2003, tmdb: 554433},
	{path: "Movies/Harbor of the Hidden Station (1999)/Harbor of the Hidden Station (1999).mkv", title: "Harbor of the Hidden Station", year: 1999},
	{path: "Movies/The Dragon & the Comet (2012)/The Dragon & the Comet (2012) - 720p.mkv", title: "The Dragon & the Comet", year: 2012},
	{path: "Movies/Labyrinth (1986)/Labyrinth (1986).mkv", title: "Labyrinth", year: 1986},
	{path: "Movies/Golden Machine 3 (2015) {tmdb-10001}/Golden Machine 3 (2015).mkv", title: "Golden Machine 3", year: 2015, tmdb: 10001},
	{path: "Movies/The Witness's Letter (1977)/The Witness's Letter (1977).mkv", title: "The Witness's Letter", year: 1977},
	{path: "Movies/Echo - Part IV (2021)/Echo - Part IV (2021).mkv", title: "Echo - Part IV", year: 2021},
	{path: "Movies/Grüne Alpenglühen, Süsse Bärgli (1968)/Grüne Alpenglühen, Süsse Bärgli (1968) - 1080p.mkv", title: "Grüne Alpenglühen, Süsse Bärgli", year: 1968},
	{path: "Movies/Café Fjord (1990)/Café Fjord (1990).mkv", title: "Café Fjord", year: 1990},
	{path: "Movies/Señor Øresund (2005) {tmdb-1199999}/Señor Øresund (2005).mkv", title: "Señor Øresund", year: 2005, tmdb: 1199999},
	{path: "Movies/Naïve Château (2020)/Naïve Château (2020).mkv", title: "Naïve Château", year: 2020},
	{path: "Movies/Über Mädchen (1951)/Über Mädchen (1951).mkv", title: "Über Mädchen", year: 1951},
	{path: "Movies/The Bärgli's Zürichsee (2010)/The Bärgli's Zürichsee (2010) - 720p.mkv", title: "The Bärgli's Zürichsee", year: 2010},
	{path: "Movies/Lucky Lighthouse, Bitter Diner (1972) {tmdb-777777}/Lucky Lighthouse, Bitter Diner (1972) - 1080p.mkv", title: "Lucky Lighthouse, Bitter Diner", year: 1972, tmdb: 777777},
	{path: "Movies/Iron Kingdom 2 (1960)/Iron Kingdom 2 (1960).mkv", title: "Iron Kingdom 2", year: 1960},
	{path: "Movies/The Silent River (1987)/Trailers/The Silent River (1987) Official Trailer.mkv", title: "The Silent River", year: 1987, extra: true},
	{path: "Movies/The Silent River (1987)/The Silent River (1987)-trailer.mkv", title: "The Silent River", year: 1987, extra: true},
	{path: "Movies/The Silent River (1987)/Featurettes/Featurette 2.mkv", title: "The Silent River", year: 1987, extra: true},
	{path: "Movies/The Silent River (1987)/Behind The Scenes/Making Of.mkv", title: "The Silent River", year: 1987, extra: true},

	// Generated test media (testdata/media/Movies).
	{path: "Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024).mp4", title: "Slouchcat Test H264", year: 2024},
	{path: "Slouchcat Test HDR10 4K (2024)/Slouchcat Test HDR10 4K (2024).mkv", title: "Slouchcat Test HDR10 4K", year: 2024},
	{path: "Slouchcat Test AC3 51 (2024)/Slouchcat Test AC3 51 (2024).mkv", title: "Slouchcat Test AC3 51", year: 2024},
	{path: "Slouchcat Test HEVC 10bit (2024)/Slouchcat Test HEVC 10bit (2024).mkv", title: "Slouchcat Test HEVC 10bit", year: 2024},
}

func TestParseMoviePathCorpus(t *testing.T) {
	t.Parallel()
	if len(movieCorpus) < 150 {
		t.Fatalf("corpus has %d paths, want at least 150", len(movieCorpus))
	}
	matched := 0
	for _, c := range movieCorpus {
		got := ParseMoviePath(c.path)
		year := 0
		if got.Year != nil {
			year = *got.Year
		}
		if got.Title == c.title && year == c.year && got.TMDBID == c.tmdb && got.IMDBID == c.imdb &&
			got.Extra == c.extra && got.Sample == c.sample {
			matched++
			continue
		}
		t.Logf("mismatch %q:\n  got  title=%q year=%d tmdb=%d imdb=%q extra=%v sample=%v\n  want title=%q year=%d tmdb=%d imdb=%q extra=%v sample=%v",
			c.path, got.Title, year, got.TMDBID, got.IMDBID, got.Extra, got.Sample,
			c.title, c.year, c.tmdb, c.imdb, c.extra, c.sample)
	}
	ratio := float64(matched) / float64(len(movieCorpus))
	msg := fmt.Sprintf("name parser corpus: %d/%d exact matches (%.1f %%)", matched, len(movieCorpus), ratio*100)
	t.Log(msg)
	if ratio < 0.98 {
		t.Fatal(msg + ", want at least 98 %")
	}
}

func TestVersionLabel(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Avatar (2009)/Avatar (2009) - 1080p.mkv":                                                    "1080p",
		"Avatar (2009)/Avatar (2009) - 4K.mkv":                                                       "2160p",
		"Avatar (2009)/Avatar (2009).mkv":                                                            "",
		"Aliens (1986) [tmdb-679]/Aliens (1986) {edition-Director's Cut} - 2160p.mkv":                "2160p Director's Cut",
		"Blade Runner (1982) {edition-Final Cut}/Blade Runner (1982).mkv":                            "Final Cut",
		"Ford v Ferrari (2019)/Ford v Ferrari (2019) - [German DL][Bluray-1080p][DTS 5.1][x264].mkv": "1080p",
		"Parasite.2019.720p.BluRay.x264-GROUP.mkv":                                                   "720p",
		"Movie (2019) UHD/Movie (2019).mkv":                                                          "2160p",
	}
	for p, want := range cases {
		if got := ParseMoviePath(p).VersionLabel; got != want {
			t.Errorf("VersionLabel(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestIsVideoAndIgnored(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"a.mkv", "a.MP4", "a.m4v", "a.mov", "a.avi", "a.wmv", "a.webm", "a.ts", "a.m2ts", "a.mpg", "a.mpeg"} {
		if !IsVideo(n) {
			t.Errorf("IsVideo(%q) = false", n)
		}
	}
	for _, n := range []string{"movie.nfo", "folder.jpg", "a.de.srt", "a.bif", "a.ass", "logo.png", "a", "mkv"} {
		if IsVideo(n) {
			t.Errorf("IsVideo(%q) = true", n)
		}
	}
	for _, n := range []string{".hidden", ".DS_Store", "@eaDir", "#recycle", "lost+found", "BDMV", "VIDEO_TS"} {
		if !Ignored(n) {
			t.Errorf("Ignored(%q) = false", n)
		}
	}
	for _, n := range []string{"Alien (1979)", "Trailers", "a.mkv"} {
		if Ignored(n) {
			t.Errorf("Ignored(%q) = true", n)
		}
	}
}

func TestSortTitle(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"The Matrix": "matrix", "A Quiet Place": "quiet place", "An Education": "education",
		"Der Untergang": "untergang", "Die Hard": "hard", "Das Boot": "boot", "Ein Mädchen": "mädchen",
		"Eine Frau": "frau", "Alien": "alien", "The": "the", "Theory": "theory", "Dune Part Two": "dune part two",
	}
	for in, want := range cases {
		if got := SortTitle(in); got != want {
			t.Errorf("SortTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
