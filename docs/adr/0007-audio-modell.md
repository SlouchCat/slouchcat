# ADR 0007: Hörbücher und Podcasts im generischen Titelmodell

- Status: angenommen (20. Oktober 2026); eine Zeile fortgeschrieben durch **ADR 0015** (29. Oktober
  2026): eine Podcast-Bibliothek darf Ordner mit `content: podcast` tragen (Audiobookshelf-Layout), der
  offene Punkt «lokale Podcast-Ordner» unten ist gebaut
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitte 3.1 (Modul `audio`), 3.2, 3.3, 3.4, 5.7 AN-2 und M6;
  Entscheid 9 (Teil Hörbücher und Podcasts) in `docs/planning/04-entscheidungen.md`; Spec H-1 bis H-4, O-1, O-3;
  Entwurf in den Sitzungsnotizen `m6-design.md`
- Betroffene Spec-IDs: H-1 bis H-4, O-1, O-3, K-3

## Kontext

M6 bringt zwei Medientypen ohne Video in ein Modell, das bisher auf Filme und Serien zugeschnitten war: ein
Titel hatte genau eine abspielbare Datei (weitere Dateien sind Versionen, B-5), die Wiedergabe ging immer vom
Videostream aus, und Verfügbarkeit hing an Dateien. Vier Fragen waren zu entscheiden:

1. Wie wird ein Hörbuch aus einem **Ordner mit zehn MP3-Dateien** zu einem Titel mit einer Zeitachse, auf der
   Fortschritt (H-3), Kapitel (H-1) und Download (O-1) liegen?
2. Wo läuft der Code? Der Plan nennt ein Modul `audio`, CLAUDE.md Regel 2 zählt die Pakete mit Titel-SQL
   abschliessend auf, und `audio` steht nicht darin.
3. Wie kommen Podcast-Episoden ohne Datei (nur `enclosure_url`) zum Hörer und auf das Gerät, ohne dass der
   Client die Sichtbarkeit umgeht oder die Adresse des Haushalts beim Anbieter landet?
4. Woher kommt die Altersstufe ohne externen Dienst (AN-2)?

Verworfen: (a) je Teil ein eigener Titel (`audiobook_part`) – neuer `kind`-Wert, der Fortschritt zerfällt in
zehn Zeilen, die Abnahme «Fortschritt mitten im Kapitel» (H-3) müsste über Titelgrenzen rechnen. (b) Die
Teile als Versionen (B-5) – Versionen sind Alternativen, keine Folge. (c) Der Client wechselt die Datei am
Teilende selbst und rechnet Positionen um – jeder Client (Web, später TV) müsste die Logik nachbauen, Spulen über
eine Teilgrenze bräuchte zwei Wiedergabesitzungen, und der Offline-Player müsste zehn Dateien verwalten.
(d) Ein Paket `audio` mit Datenbankzugriff – hätte Regel 2, die Lint-Listen (`depguard`, `itemsaccess`) und die
sqlc-Konfiguration geändert. (e) Enclosure-URL direkt an den Client – kein 404 für unsichtbare Episoden auf
dem Stream (Plan 10.2 verlangt es für «Stream-Start»), Auto-Downloads bräuchten trotzdem einen Serverabruf.

## Entscheid

| Punkt | Entscheid |
|---|---|
| Ein Buch, ein Titel | Ein `audiobook` ist **ein** Item mit einer **buchweiten Zeitachse**. Mehrere Dateien sind **Teile** (`media_files.part_index`, 1-basiert, natürliche Sortierung nach `track`-Tag und Dateiname), keine Versionen. `items.duration_ms` ist die Summe. Regel je Verzeichnis: eine Audiodatei → ein Buch; mehrere Dateien, alle `.m4b` → je Datei ein Buch; sonst alle Dateien des Verzeichnisses ein Buch |
| Kapitel | Tabelle `chapters(item_id, idx, title, start_ms, end_ms, media_file_id, file_start_ms)`, buchweit; Quelle sind die Kapitelmarken der Datei(en), sonst ein Kapitel je Teil (Titel aus Tag, Dateiname oder «Teil n»), sonst ein Kapitel über das ganze Buch. Nach jeder Probe eines Teils wird die Liste neu gerechnet, sobald alle Teile probiert sind. Der Vertrag liefert sie in `ItemDetail.chapters`, `PlaybackInfo.chapters` und `DownloadManifest.chapters` |
| Eine Zeitachse auch beim Streamen | Ein Buch mit **einem** Teil spielt direkt (AAC in MP4/M4B oder MP3, wenn das Gerät kann), sonst als Audio-HLS. Ein Buch mit **mehreren** Teilen läuft **immer** als HLS über eine Concat-Quelle in `transcode` (`Source.Parts`, ffmpeg `-f concat`); Fortschritt, Kapitel, Spulen und der Offline-Download (eine fMP4-Datei) sehen dieselbe Zeitachse. Kosten 0.25 Einheiten je Strom; MP3-Teile werden zu AAC, M4A-Teile kopiert |
| Paket | Kein Paket `audio` mit Datenbankzugriff. Reine Logik liegt in `server/internal/feed` (RSS, ohne DB) und in `library/audiobook*.go` (Gruppierung, Tags, Kapitelrechnung); Items schreiben `library` (Hörbücher) und `metadata` (Podcast-Feeds). Regel 2 und die Lint-Listen bleiben unverändert; Plan 3.3 nennt dieselben Pakete |
| Metadaten ohne externen Dienst | Vorrang: Audiobookshelf `metadata.json` im Buchordner → Tags der ersten Datei (`artist` Autor, `composer` Sprecher, `show`/`episode_id` Reihe und Band) → Ordnernamen `Autor/[Reihe/][NN - ]Titel`. Cover aus `cover.jpg`/`folder.jpg`/`poster.jpg` (`cover_path`) oder aus dem eingebetteten Bild (`cover_path = embedded:<media_file_id>`, Extraktion nur in `transcode`). Autor und Sprecher sind Spalten an `items` (`author`, `narrator`, `series_name`, `series_index`), keine `credits` (die brauchen eine TMDB-ID); Suche über Autoren kommt mit FTS in M7 |
| Podcasts | Ein `podcast` ist ein Container-Item mit `feed_url`, `feed_etag`, `feed_modified`, `feed_checked_at`, `feed_error` in einer Bibliothek der Art `podcasts` **ohne Wurzel** (seit ADR 0015 auch mit Podcast-Ordnern; `feed_url` darf dann `NULL` sein, `feed_origin` sagt, ob der Feed von Hand oder aus einer `metadata.json` kommt). Episoden sind `podcast_episode` mit `guid`, `published_at`, `enclosure_url/type/length`, `duration_ms`, `explicit`; Identität je Podcast ist die GUID (sonst die Enclosure-URL). Abruf alle **55 Minuten** (mit dem 5-Minuten-Ticker und der Job-Wartezeit unter den 65 Minuten von H-2) über einen Ticker im `metadata`-Dienst, der Jobs `podcast.fetch` einreiht (bedingtes GET mit ETag/Last-Modified, ≤ 2000 Episoden, Episoden verschwinden nicht aus der Bibliothek, wenn der Feed sie kürzt). Der Erstabruf beim Abonnieren ist synchron, damit der Admin Fehler sofort sieht |
| Episoden streamen und laden über den Server | Eine Episode ohne Datei spielt als `direct`-Wiedergabesitzung mit `media_file_id ''`; der Stream-Endpunkt reicht `Range` an die Enclosure durch (Proxy mit Token und Sichtbarkeit wie jeder Stream, 502 `enclosure_unavailable` bei Fehlern). Der Download-Job lädt die Enclosure zuerst in den Cache und encodiert daraus (`downloads.source_url`, `media_file_id` nullable). Der Haushalt zeigt dem Podcast-Anbieter so nur die Server-Adresse |
| Adressen im Heimnetz | Feed, Enclosures und Cover ruft der Server über einen gemeinsamen Wächter ab (`internal/netguard`), der nach der DNS-Auflösung (also auch nach Weiterleitungen und bei DNS-Rebinding) Loopback-, private (RFC 1918, Unique-Local), Link-Local- und unspezifizierte Adressen verweigert, denn diese Adressen bestimmt der Herausgeber des Feeds und nicht der Admin. Liegt der Host der Feed-URL selbst im Heimnetz (ein bewusst abonnierter LAN- oder Loopback-Feed wie der E2E-Mock auf 127.0.0.1), dürfen auch seine Enclosures und Cover dort liegen; Fehlermeldungen nennen von diesen URLs nur Schema und Host |
| Altersstufe (AN-2) | `libraries.default_rating` gilt für jeden Titel einer Audio-Bibliothek (`rating_source 'library'`, Standard 0); eine Änderung der Standardstufe zieht alle Titel mit dieser Quelle mit. `itunes:explicit=true` setzt Podcast oder Episode auf 18 (`rating_source 'feed'`), `explicit` aus `metadata.json` ebenso (`'tags'`). Eine manuelle Stufe darf nur **über** dem Standard liegen (422 `rating_below_library_default`), analog AN-9. Trigger und `visible_items` bleiben unverändert: Episoden erben vom Podcast, der Podcast ist ein Container mit `min_child_rating` |
| Audio-Downloads | Dieselben drei Stufen wie Video, bei Audio AAC 128/96/64 kbit/s (Kopie, wenn die Quelle AAC innerhalb 1.1× ist); Manifest mit `mimeType audio/mp4` und Kapiteln; `allowed_profile_ids` wie bisher (H-4). Auto-Downloads neuer Episoden (O-3) gelten für Podcasts, in denen das Profil in den letzten 60 Tagen eine Episode begonnen hat (die Favoriten kommen in M7) |
| Geschwindigkeit je Titel (H-3) | `progress.playback_rate` (0.5–3.0) je Profil und Titel; der Client meldet die Rate mit dem Fortschrittsereignis (`playbackRate`) und liest sie aus `Progress.playbackRate` |

## Folgen

- Vertrag 0.7.0 (81 Operationen): `LibraryKind` mit `audiobooks` und `podcasts`, `updateLibrary`, `listPodcasts`,
  `createPodcast`, `deletePodcast`, `refreshPodcast` (admin), `listPodcastEpisodes` (scoped),
  `PlaybackInfo.mediaKind`, Audio-Felder an `ItemRef` und `ItemDetail`.
- Migration 00014 baut `libraries` und `downloads` neu auf (SQLite kann CHECK und NOT NULL nicht ändern) und
  erweitert die `media_files`-Trigger auf `podcast_episode`, damit eine spätere lokale Episodendatei zählt.
- Der Client enthält keine Alters- oder Sichtbarkeitsregel (Prinzip 1): explicit-Episoden fehlen in jeder
  Antwort für Profile unter 18, auch in `listPodcastEpisodes` und in den Auto-Downloads.
- Offen für spätere Versionen: Podcast-Suche (Apple, Podcast Index) statt reiner Feed-URL (Entscheid 7), ein
  OPF-Import, Atom-Feeds. Lokale Podcast-Ordner (Audiobookshelf-Downloads) als Bibliothekswurzel sind seit
  ADR 0015 gebaut (Frage 13, F-1 und F-6).
