# ADR 0006: Metadaten-Anbieter-Schnittstelle, TVDB optional, Trailer-Kette mit Tageskontingent

- Status: angenommen (19. Oktober 2026); **teilweise ersetzt durch ADR 0015** (29. Oktober 2026): die
  Reihenfolge der Anbieter ist nicht mehr fest im Code, sondern je Ordner eine geordnete Liste; TVDB ist
  als Erkenner für Serienordner wählbar; die zwei ersten «Verworfenen Alternativen» unten sind damit
  angenommen. Schnittstelle, Serien-Job, Trailer-Kette, Tageskontingent und Schlüsselregel gelten weiter
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitte 3.1, 3.2, 5.3 und M3; Entscheid 7 in
  `docs/planning/04-entscheidungen.md` (Antwort auf Frage 7); M3-Entwurf, Abschnitte 3.1 und 3.2
- Betroffene Spec-IDs: B-3, B-6, B-8, W-10, AN-8, AN-15

## Kontext

Bis M2 kannte Slouchcat nur Filme und genau einen Anbieter: Der Metadaten-Job sprach direkt mit dem
TMDB-Client, und `image.fetch` holte für die Bildpfade den Film ein zweites Mal ab. M3 bringt Serien
mit Staffeln, Folgen und Specials, Besetzung, Trailer und die manuelle Zuordnung. Zu entscheiden war:

1. Wie hängen mehrere Anbieter am Metadaten-Job, ohne dass jeder weitere Anbieter den Job umbaut
   (Entscheid 7: «TMDB Hauptquelle, TVDB optional hinter einer Anbieter-Schnittstelle, weitere Anbieter
   später ohne Umbau»)?
2. Wann fragt Slouchcat TheTVDB, das einen eigenen, teils kostenpflichtigen Schlüssel braucht?
3. Woher kommen Trailer, wenn TMDB keinen nennt, ohne dass ein Kontingent überzogen oder etwas von
   YouTube heruntergeladen wird?
4. Was geschieht mit dem Wunsch, Trailer später mit einem KI-Anbieter zu suchen?

## Entscheid

| Punkt | Entscheid |
|---|---|
| Schnittstelle | `metadata.Provider` mit `SearchMovie`, `SearchSeries`, `Movie`, `Series`, `Season`, `Episode`, `Name`, `Configured`. Die Datensätze tragen Texte je Sprache (de, en), Freigaben je Land für die AN-8-Abbildung (auch US-TV-Stufen), Besetzung (höchstens 15 Darsteller mit Reihenfolge und Rolle, Regie, Drehbuch, Idee), Bildpfade und externe IDs. Der Job kennt nur die Schnittstelle; ein weiterer Anbieter ist eine weitere Implementierung |
| TMDB zuerst | Filme kommen nur von TMDB. Serien: TMDB über `tmdb_id`, sonst `/find` mit `tvdb_id` oder `imdb_id` (auch wenn der Ordner nur `{tvdb-…}` trägt), sonst Titelsuche mit Jahr ±1 |
| TVDB optional | TVDB v4 nur mit eigenem Schlüssel (`SLOUCHCAT_TVDB_API_KEY`, optional PIN); ohne Schlüssel ist der Anbieter nicht konfiguriert und wird nie gefragt. Gefragt wird TVDB, wenn TMDB nichts findet, oder wenn der Administrator eine TVDB-ID von Hand zugeordnet hat. Ein TVDB-Treffer schreibt `tvdb_id` und `rating_source` mit Präfix (`tvdb:DE`). Anmeldung per `POST /login`, das Token wird gecacht und nach einem 401 einmal erneuert; eigener Begrenzer 5 Anfragen/s |
| Serien-Job | Ein Job je Serie füllt Serie, jede vorhandene Staffel und jede vorhandene Folge (eine Transaktion je Staffel, höchstens 500 Zeilen); Folgen, die danach noch fehlen, bekommen eigene Jobs. Die Bildpfade landen am Item (`tmdb_poster_path`, `tmdb_backdrop_path`), `image.fetch` liest sie dort |
| Trailer-Kette | Job `trailer.find` (Priorität 1) nach jedem erfolgreichen Abgleich, Schritte nur für noch fehlende Sprachen: (1) TMDB-Videos, offizielle zuerst, dann die neuesten, einer je Videosprache; (2) YouTube Data API v3 für Deutsch und Englisch mit Dauer- und Titelfilter; (3) KI-Anbieter. Automatische Zeilen werden ersetzt, manuelle bleiben. Gespeichert wird nur die Video-ID; abgespielt wird über `youtube-nocookie.com`, nie heruntergeladen |
| Tageskontingent | `ratelimit.Quota` zählt die YouTube-Einheiten (10'000 pro Tag, Suche 100, Videodetails 1, Neubeginn Mitternacht pazifischer Zeit). Ist es leer oder meldet Google `quotaExceeded`, bleibt der Titel `pending` und der Job endet ohne Fehler; `QueuePendingTrailers` reiht ihn nach dem nächsten Scan wieder ein |
| KI aus | `AITrailerProvider` als Schnittstelle mit einer leeren Implementierung; `SLOUCHCAT_TRAILER_AI=off` ist Standard und einziger Wert in M3, andere Werte erzeugen eine Warnung und bleiben wirkungslos |
| Schlüssel | Je Installation eigene Schlüssel (Entscheid 7), nur in `deploy/.env` oder später verschlüsselt in den Einstellungen (M8). Schlüssel stehen nur im `Authorization`-Header oder im Login-Körper, bei YouTube im Abfrageparameter `key`; der Log-Handler maskiert `key`, `apikey`, `api_key` und die Attributnamen `youtube_api_key`, `tvdb_api_key`, `tvdb_pin` |

## Verworfene Alternativen

- **TVDB als Hauptquelle für Serien** (wie Sonarr): bessere Folgenzählung bei Anime und Tages-Shows,
  aber ein zweiter Pflichtschlüssel mit Lizenzkosten und deutsche Texte nur, wo die Community sie
  pflegt. TMDB deckt den Haushalt ab und liefert dieselben Bilder und Freigaben wie für Filme.
  **Abgelöst durch ADR 0015 (29. Oktober 2026):** mit einer Anime-Bibliothek trifft der Grund zu;
  TVDB ist je Serienordner als Erkenner wählbar, TMDB bleibt die Vorgabe.
- **Beide Anbieter abfragen und zusammenführen**: doppelte Anfragen bei jedem Titel, Widersprüche bei
  Freigaben und Folgennummern, ohne klaren Gewinn. **Abgelöst durch ADR 0015 (29. Oktober 2026):**
  je Ordner eine geordnete Liste; Anfragen nur für angekreuzte Anbieter, Folgennummern allein vom
  Erkenner, Freigaben nie gesenkt, Herkunft je Feld in `item_field_sources`.
- **Ein Job je Folge**: bei 60'000 Dateien zehntausende Jobs und doppelt so viele Anfragen; die
  Staffelabfrage liefert alle Folgen in einer Antwort.
- **YouTube ohne Kontingentzähler**, nur auf `quotaExceeded` reagieren: der Server würde jeden Tag in
  die Sperre laufen und Folgefehler erzeugen; der Zähler hält die Suchen planbar, die Antwort von
  Google bleibt die Absicherung nach einem Neustart.
- **Scraping der YouTube-Webseite oder Download der Trailer**: verstösst gegen die Nutzungsbedingungen
  und bringt Speicher- und Rechtsfragen; die Video-ID genügt.
- **KI-Suche jetzt einbauen**: kein Bedarf, solange TMDB und YouTube fast alles finden, und ein
  weiterer externer Dienst mit Kosten; die Schnittstelle hält den Platz frei.

## Folgen

- Neue Anbieter (Trakt, Fanart.tv, Podcasts in M6) implementieren `Provider` oder `TrailerProvider`;
  der Job und die Bildpipeline bleiben gleich.
- TVDB-Personen haben keine TMDB-ID und werden in M3 nicht gespeichert (`people.tmdb_id` ist der
  Schlüssel); TVDB-Genres sind nur englisch, TVDB-Staffeln haben keine eigenen Texte («Staffel N»).
- Titel aus M1/M2 ohne gespeicherten Bildpfad behalten ihre Bilder; `image.fetch` nimmt dafür die Quelle
  des gespeicherten Originals, ein Aktualisieren schreibt die Pfade nach.
- Die Freigabe-Abbildung (AN-8) ist anbieterneutral (`FSKFromCertifications`); `rating_source` nennt
  Anbieter und Land.
- Die Administration (`AssignMatch`, `Refresh`, `Search`, manuelle Trailer) ruft Funktionen des Pakets
  `metadata`; die Einstellungsseite für Schlüssel folgt in M8.
