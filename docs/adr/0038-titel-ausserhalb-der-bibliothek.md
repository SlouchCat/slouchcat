# ADR 0038: Titel ausserhalb der Bibliothek

- Status: angenommen (17. November 2026, Stufe 0 von Konzept 25, mit Frage 33 und ihrem Nachtrag); **löst S-4 nicht ab**: «Titel ohne
  Bibliothekstreffer erscheinen nie auf der Startseite» (Frage 11, ADR 0011) gilt ohne Ausnahme (Marco, E-5).
  **Ergänzt ADR 0034** (Freigabeländer) um Titel ohne Eintrag in `items`.
- Quelle: Konzept `docs/planning/25-medienwuensche/` (vor allem `02-funktionskonzept.md` und
  `05-spotlight-integration.md`, Fassung 3), **Frage 33** mit Nachtrag in `docs/planning/04-entscheidungen.md`
  (F-9, F-11, F-12, F-28, F-29 am 17. November 2026 nach Empfehlung beantwortet; F-8 durch E-5 und F-34 durch E-10
  entschieden); Marcos Rückmeldungen vom 17. November 2026 (Wünsche-Seiten, TVDB; keine fehlenden Titel im
  Spotlight), `docs/planning/25-medienwuensche/anhang/H-nachtrag-17-november.md`; die Anhänge dort heissen hier
  «Anhang D» usw.
- Vertrag und Migrationen: vergibt der Bau; erscheint mit ADR 0037 in einem Release (F-22, B-1)
- Betroffene Spec-IDs: Spec E-1, Spec E-4, Spec E-7
- Harte Regeln: 2, 5, 6 (Bilder als `http.Handler`), 7, 12

## Kontext

Bis 1.40 kennt Slouchcat nur Titel der eigenen Bibliothek. Alles hängt an `items`: Freigabe
(`effective_rating`, `item_certifications`), Bilder, Merkliste, Empfehlungen, Eingang. Das Spotlight speichert für
Trakt-Einträge ohne Treffer nur TMDB-ID, Titel und Jahr und zeigt sie nur in der Verwaltung (S-4). Wer einen Titel
anfordern soll, muss ihn aber sehen: mit Poster, Beschreibung, Freigabe und Trailer, in der Suche und auf einer
eigenen Seite, seit Marcos erster Rückmeldung auch auf eigenen Seiten «Entdecken», «Filme», «Serien» und «Suche»
wie bei Seerr. Auf der Startseite will Marco nur vorhandene Titel (zweite Rückmeldung, E-5). Dabei darf kein Profil einen Titel sehen, den es nach seiner Altersstufe nicht sehen
dürfte, und der Browser soll keine Bilder direkt bei TMDB holen.

## Entscheid

1. **Eigene Daten für fehlende Titel:** `external_titles` (TMDB: Titel je Sprache in `external_title_texts`, Jahr,
   Beschreibung, Laufzeit, Genres, Freigabe, Poster, Hintergrund, Logo, Trailer; bei Serien Staffeln mit Folgenzahl
   und Datum, Status, nächste Folge). Aufgefrischt von `titles.refresh` (24 h, nach `trakt.refresh`). Fehlende
   Titel kommen nie in `items`.
2. **Jugendschutz im Server:** Die TMDB-Freigabe wird in der Länderreihenfolge nach ADR 0034 gelesen
   (gewähltes Freigabeland, dann DE, AT, CH, GB, US; das erste Land mit bekanntem Wert gilt, dieselbe Funktion
   wie für Bibliothekstitel; innerhalb eines Lands der strengste Wert über die Anbieter) und gegen
   min(Profil, Konto) geprüft. **Unbekannt zählt als 18.** Was ein Profil nicht sehen dürfte oder nicht anfordern
   kann, existiert für es nicht: 404 auf der Vorschauseite und bei Bildern, kein Treffer in Suche und auf den
   Wünsche-Seiten (F-9; Ausnahme: Titel mit eigenem Wunsch). Die Regel wohnt in `visibility`. Weil die TMDB-Suche keine
   Freigaben liefert, holt der Server für Profile unter 18 die Details der ersten 20 Treffer.
3. **Bildweg:** Bilder fehlender Titel laufen über Slouchcats eigenen Bildspeicher und die Route
   `GET /titles/{media}/{tmdbId}/images/{kind}/{size}` (`http.Handler`, `scoped`, im Vertrag beschrieben), nie
   direkt von TMDB im Browser. Die Verwaltung nutzt die eigene Route `GET /admin/titles/{media}/{tmdbId}/images/…`
   (`admin`), weil Verwaltungsantworten keine scoped Adressen tragen.
4. **Vertrag:** fehlende Titel als eigenes Schema `ExternalTitleRef` neben `ItemRef` (nicht brechend);
   `getExternalTitle` und `searchExternalTitles` sind `scoped`; die Vorschauseite liegt unter
   `/titles/{movie|show}/{tmdbId}`.
5. **Wo Titel ausserhalb der Bibliothek erscheinen (S-4 bleibt):** nur im Bereich «Wünsche», im Abschnitt «Noch
   nicht in der Bibliothek» der Suche, auf der Vorschauseite und als fehlende Staffeln einer vorhandenen Serie.
   **Nie auf der Startseite** (Bühne, Posterreihe, Reihen) und nie in der Spotlight-Liste, auch nicht für Profile,
   die anfordern können, und nicht als Reihe unter «Entdecken» (E-5). Fehlende Einträge der Trakt-Liste sieht nur
   der Administrator unter Verwaltung › Spotlight, mit Poster, Zustand, IDs und «Anfordern». Die Startreihe
   «Erfüllte Wünsche» zeigt nur Titel der Bibliothek (E-10).
6. **Abgleich über mehrere IDs, in fester Reihenfolge:** Filme über TMDB-, dann IMDb-ID; Serien wie die
   Bibliothek über TVDB-, dann TMDB-, dann IMDb-ID (Marco: «konsistent»). Trakt-Einträge speichern neu TVDB- und
   IMDb-ID; ein Eintrag nur mit TVDB-ID wird über TMDB «find» aufgelöst, sonst gilt er nicht als fehlend.
   Vorhandene Serien, die nur eine TVDB-ID haben, gelten so nicht fälschlich als fehlend. Der Schlüssel zu
   Seerr bleibt die TMDB-ID, bei Serien mit `tvdbId`.
7. **Suche:** Unter der Bibliothekssuche ein Abschnitt «Noch nicht in der Bibliothek» aus der TMDB-Suche über den
   eigenen Schlüssel, gefiltert nach Bibliothek, Freigabe und Wunschrecht, mit Verweis auf die Wunsch-Suche.
8. **Wünsche-Seiten:** Unter «Wünsche» zeigen «Entdecken» (sieben feste Reihen), «Filme», «Serien» und «Suche»
   Titel ausserhalb der Bibliothek. Die Listen kommen über Seerr (Discover- und Suchrouten) mit einem
   Zwischenspeicher von 1 Stunde je Liste, Seite, Sprache und Altersstufe; Titeldaten und Freigaben bleiben in
   `external_titles`. Jeder Treffer läuft durch dieselbe Jugendschutzregel; für Profile mit Altersgrenze filtert
   Seerr mit `certificationLte` vor, Slouchcat prüft nach. Titel aus der Bibliothek erscheinen mit Marke und
   Link; massgeblich ist der Abgleich von Slouchcat, nicht `mediaInfo` von Seerr. Die Listen sind
   öffentliche Ranglisten für alle und keine algorithmische Empfehlung. Auf der Startseite erscheinen sie nicht.
9. **Serien in der Bibliothek:** `series_catalog` hält die Staffelliste laut TMDB, Status und nächste Folge; damit
   zeigt die Serienseite fehlende Staffeln und «2 von 4 Staffeln vorhanden».
10. **Keine Merkliste, kein Gesehen, kein Empfehlen** für fehlende Titel; diese hängen an Bibliothekstiteln.

## Verworfen

| Alternative | Grund |
|---|---|
| fehlende Titel als Zeilen in `items` | bräche `visible_items`, Scans, Zählungen und alle Fremdschlüssel auf `items` |
| Titeldaten über Seerr (`GET /movie/{id}`) | jeder Aufruf geht ohnehin an TMDB; die Anzeige hinge an der Erreichbarkeit von Seerr. Für die Listen der Wünsche-Seiten gilt das bewusst nicht: Sie sollen zeigen, was Seerr anbietet, und haben einen Zwischenspeicher |
| `mediaInfo` von Seerr für «in der Bibliothek» | Seerr setzt «verfügbar» nach seinem eigenen Scan, nicht nach dem Abgleich von Slouchcat (Anhang D 11.1) |
| Reihen-Einstellung von Seerr übernehmen | Reihen ohne Bedeutung in Slouchcat (Plex Watchlist, Studios, Networks), F-28 |
| Bilder direkt von `image.tmdb.org` im Browser | verrät dem Anbieter die Zuschauer, umgeht den Bild-Cache der App |
| fehlende Titel ohne Freigabe zeigen | widerspricht AN-1 (unbekannt = gesperrt unter 18) |
| fehlende Titel im Spotlight mit Schalter und Höchstzahl (Fassung 2, Fassung 3 vor der zweiten Rückmeldung) | Marco: «In Spotlight soll nicht angezeigt werden was fehlt» (E-5) |
| fehlende Einträge der Trakt-Liste als Reihe unter «Entdecken» | Marco: «Nein, nirgends» (E-5) |
| Schalter je Profil für fehlende Titel | das Wunschrecht entscheidet schon (F-9) |

## Folgen

- Neue Tabellen `external_titles`, `external_title_texts`, `external_title_images`, `series_catalog`,
  `wish_discover_pages`; `spotlight_entries` bekommt `tvdb_id` und `imdb_id`, damit vorhandene Titel auch über TVDB
  und IMDb erkannt werden; `spotlight_config` bleibt unverändert.
- Die Wünsche-Seiten belasten Seerr und TMDB; der Zwischenspeicher und der Job `wishes.discover` halten das klein.
- Mehr TMDB-Aufrufe über den bestehenden Limiter; der Bildspeicher wächst um die fehlenden Einträge des Spotlights
  (nur für die Verwaltung) und die Treffer der Wünsche-Seiten.
- **Vorschlag:** Einträge in `external_titles` ohne Bezug (nicht mehr im Spotlight, kein Wunsch, 30 Tage nicht
  gesucht) räumt `titles.refresh` weg.
- Die Spotlight-Verwaltung zeigt für fehlende Einträge Poster, Zustand, IDs und «Anfordern»; im Spotlight ändert
  sich für Zuschauer nur, dass TVDB-zuerst-Serien erscheinen.
- Die Startseite bekommt die neue Reihe «Erfüllte Wünsche» (Marco, E-10): Wünsche des Profils, die in den letzten 14 Tagen
  erfüllt wurden, also nur Titel der Bibliothek (`ItemRef`). Die Titel-IDs liefert `wishes`, die Sichtbarkeit prüft
  `visibility`. Der neue Schlüssel in `HomeRowKey` macht Stufe 3 brechend; ohne die Reihe wäre sie es nicht.
