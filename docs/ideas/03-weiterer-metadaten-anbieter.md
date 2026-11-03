# Idee 03: Weiterer Metadaten-Anbieter für fehlende Altersfreigaben

- Status: Entwurf (aufgenommen am 3. November 2026, noch kein Interview, kein Konzept)
- Frühestens: nach Version 1.0
- Issue: [#71](https://github.com/SlouchCat/slouchcat/issues/71)
- Herkunft: Marcos Wunsch bei der Abnahme K12-9 am 3. November 2026
- Vorbild: Scraper-Liste in Emby und Jellyfin

## 1. Anlass

Im Pflege-Reiter «Altersfreigabe fehlt» (früher «Ohne Freigabe») stehen auf CT 190 17 Filme, für die
TMDB in keinem der Länder DE, AT, CH, GB und US eine Freigabe führt. Sie bleiben unter FSK 18 gesperrt
(AN-1). Marco möchte solche Titel mit einem weiteren Scraper zuordnen oder abrufen können.

## 2. Heute

| Punkt | Stand |
|---|---|
| Anbieter | TMDB (Filme und Serien, Freigaben), TVDB (nur Serien, optional); AniList und Fribb füllen und ordnen nur zu |
| Freigabekette | erstes Land einstellbar, dann DE, AT, CH, GB, US; Aufrunden auf die nächste FSK-Stufe (Entscheid 6, Frage 16) |
| Zielsystem | nur FSK; ein Wechsel des Systems ist in Idee 01 ausgeschlossen |
| Entscheide von Marco | die fünf Länder reichen («Reicht so»); die bestehende Sammelaktion «Freigabe zuweisen» reicht |

## 3. Idee

Ein dritter Metadaten-Anbieter, Kandidat OMDb (IMDb-Ids, Feld `Rated` = US MPAA/TV) oder ein ähnlicher.
Er wäre im Zuordnungsdialog wählbar und liefert eine Freigabe, wenn die Kette über die fünf Länder nichts
findet. Titel ohne Freigabe blieben wie bisher gesperrt, bis eine Stufe feststeht (AN-1, K-3: eine
gespeicherte Stufe wird nie gesenkt).

## 4. Offene Fragen

1. **Welcher Anbieter?** OMDb oder ein anderer; Abdeckung der 17 Filme vorab prüfen.
2. **Schlüssel und Kosten:** eigener Schlüssel je Installation (Entscheid 7, AN-15); Tageskontingent
   (Idee 01 nennt für OMDb 1'000 pro Tag).
3. **Nur Freigabequelle oder auch Erkenner?** Identität über IMDb-Id, Texte, Bilder.
4. **Abbildung auf FSK:** MPAA/TV-Stufen laufen schon über das Aufrunden der US-Stufen; ein
   Anbieter-Wert müsste dieselbe Tabelle nutzen.
5. **ADR 0006:** «beide abfragen und zusammenführen» war verworfen, ADR 0015 hat es für die Anbieterliste
   je Ordner angenommen. Passt ein dritter Anbieter in diese Liste ohne Umbau?
6. **Entscheid 7 / AN-15:** «weitere Anbieter später ohne Umbau» deckt die Schnittstelle
   `metadata.Provider`. Die Spec nennt «Plugins» als Non-Goal (Regel 11): der Anbieter bleibt fest im Code,
   kein Plugin-System.

## 5. Verweise

- `docs/adr/0006-metadaten-anbieter-trailer.md`, `docs/adr/0015-bibliotheken-ordner-scraper.md`
- `docs/planning/04-entscheidungen.md` (Entscheid 6, Entscheid 7, Frage 16)
- `docs/planning/10-metadaten-einstellungen.md`, `docs/ideas/01-metadaten-einstellungen.md` (OMDb als
  nicht übernommener Rest)
- `server/internal/metadata/ratings.go`, `server/internal/metadata/provider.go`
