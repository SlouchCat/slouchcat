# Konzept 25 · Bestandsanalyse: was es heute gibt

> **Kurz gesagt.** Slouchcat kennt heute nur Titel, die in der eigenen Bibliothek stehen. Das Spotlight
> holt zwar eine Trakt-Liste mit Titeln, die fehlen, zeigt diese aber nur in der Verwaltung. Es gibt kein
> Wunschrecht, keine Freigabe für fehlende Titel, keinen Haken «neuer Film ist da» und keinen Weg ins
> Heimnetz zu einem Dienst wie Jellyseerr. Jellyseerr selbst kann Wünsche im Namen eines Benutzers
> anlegen, kennt aber keine Profile, keinen Änderungs-Feed und keinen Ablehnungsgrund. Für Fassung 3 ist
> nachgelesen: Jellyseerr kann Benutzer mit Passwort anlegen, ihr Passwort und ihr Limit setzen, Discover-Listen
> und eine Suche liefern, führt jeden Titel über die TMDB-ID und kann keine Tags in Radarr oder Sonarr anlegen.
> Lesend gegen Marcos Radarr und Sonarr geprüft: Beide melden von sich aus nur über einen Webhook; ihre API
> beantwortet nur Fragen, liefert dafür aber die Warteschlange mit dem Download-Fortschritt. Die Webhook-Meldungen
> tragen schon Kennungen und Dateipfad, die Slouchcat heute nicht liest. Abschnitt 20 fasst die Einschränkungen
> zusammen, die das Konzept bestimmen.

Stand: `main` `ba6efe31`, Vertrag 1.40.0 (`api/openapi.yaml:4`), letzte Migration 00065, gelesen am
16. November 2026. Pfade gelten ab der Wurzel des Repos. Jellyseerr-Belege beziehen sich auf den
Quellcode von Seerr v3.5.2 (Tag `v3.5.2`, Repo `seerr-team/seerr`), verglichen mit Jellyseerr v2.7.3.
Die ausführlichen Analyseberichte liegen im Unterordner `anhang/`: [Anhang A](anhang/A-spotlight-startseite.md)
(Spotlight und Startseite), [Anhang B](anhang/B-konten-benachrichtigungen.md) (Konten und Benachrichtigungen),
[Anhang C](anhang/C-integrationen-jobs-regeln.md) (Integrationen, Jobs, Regeln) und
[Anhang D](anhang/D-jellyseerr-api.md) (Jellyseerr-API). Was für Fassung 3 am 17. November 2026 dazu nachgelesen
wurde, steht in [Anhang H](anhang/H-nachtrag-17-november.md), Abschnitte 3 und 8.3 (hier in den Abschnitten 7,
10 und 17.4 bis 17.8). Dieses Papier fasst sie zusammen. Alles zu Jellyseerr ist im Quellcode nachgelesen, aber
**nicht gegen eine laufende Instanz geprüft**. Radarr und Sonarr sind dagegen mit lesenden Aufrufen gegen Marcos
Instanzen geprüft (Abschnitt 10.3).

## 1. Papierlage: Wünschen ist heute ein Nicht-Ziel

Ein **Nicht-Ziel** (Non-Goal) ist etwas, das die Spec ausdrücklich ausschliesst. Ein Agent darf es
nicht bauen, bevor Marco es im Entscheidungsprotokoll aufhebt.

| Stelle | Wortlaut (gekürzt) |
|---|---|
| `docs/spec/01-anforderungen-v1.md:46` | Wünschen erfolgt ausserhalb von Slouchcat direkt in Seerr; kein Seerr-Bereich |
| `docs/spec/01-anforderungen-v1.md:65` | Freigabe-Anfragen von Kinderprofilen an den Hauptbenutzer |
| `docs/spec/01-anforderungen-v1.md:66-67` | eigene Wunschverwaltung; Beschaffungsstatus und Benachrichtigung bei Verfügbarkeit |
| `docs/spec/01-anforderungen-v1.md:69` | Steuerung von Radarr, Sonarr, SABnzbd |
| `docs/planning/04-entscheidungen.md:19` | Frage 10: I-3, I-4, AN-5, AN-16 gestrichen; Wünschen «Kandidat für die Ideensammlung» |
| `docs/planning/04-entscheidungen.md:20` | Frage 11, S-4: Titel ohne Bibliothekstreffer erscheinen nie auf der Startseite, in der Verwaltung aber schon |
| `docs/adr/0011-spotlight.md:99` | verworfen: Schalter «auch Titel ohne Bibliothekstreffer zeigen» |
| `CLAUDE.md:57-59` | Regel 11: «eigene Wunschverwaltung, Steuerung von Arr/Seerr» |
| ADR 0010 | «Slouchcat never calls Radarr or Sonarr and registers nothing there» ([Anhang C](anhang/C-integrationen-jobs-regeln.md) 1) |

Was sich mit Frage 33 ändert, steht in `11-entscheide-und-adr.md`. Der Code verhindert das Vorhaben
nicht, die Papiere schon. Fassung 3 braucht weitere, eng gefasste Ausnahmen (Anhang H 5 und 8.5): Benutzer in
Jellyseerr anlegen und pflegen (E-Mail, Passwort, Rechte, Limit), in Radarr und Sonarr Tags setzen und die
Warteschlange lesen, und den Beschaffungsstand (Spec Zeile 67) beim Wunsch zeigen, nicht am Titel der Bibliothek
und nicht in der Suche. S-4 (Zeile oben) bleibt ohne Ausnahme. Auch das ist nur Vorschlag; dieses Papier ändert
keine Regel.

## 2. Spotlight und Trakt

**Trakt** ist ein Dienst mit öffentlichen Ranglisten von Filmen und Serien. Das **Spotlight** ist die
grosse Bühne oben auf der Startseite mit einer Posterreihe darunter.

- **Eine** öffentliche Trakt-Liste, gewählt in der Verwaltung (`spotlight_config`, eine einzige Zeile).
  Sieben Listenarten (`server/internal/trakt/trakt.go:31-40,93-101`).
- **Datenmodell:** `spotlight_config` (u. a. `item_count` 5–20, `min_hits` 0–10, `interval_hours`
  6–48, `rotation_seconds` 3–15), `spotlight_entries` (`list_key`, `media`, `position`, `tmdb_id`,
  `title`, `year`, `fetched_at`), `spotlight_overrides` (ausblenden, anheften, Notiz; **kein** FK auf
  `items`) (`server/migrations/00021_spotlight.sql:73-126`, `server/migrations/00026_library_views.sql:22`).
- **Abgleich nur über die TMDB-ID.** Der Trakt-Client liest aus `ids` nur `tmdb`
  (`server/internal/trakt/trakt.go:270-273`). IMDb-, TVDB- und Trakt-ID werden verworfen. Einträge ohne
  TMDB-ID fallen weg (`:184-185,392`).
- **Fehlende Einträge werden gespeichert, aber nie gezeigt.** Der Abruf schreibt alle Einträge
  (`server/internal/metadata/trakt.go:291-298`). Die Startseite verbindet sie mit einem inneren Join mit
  `items` und `visible_items` (`server/queries/scoped/spotlight.sql:9-16`), fehlende fallen also heraus.
  Die Verwaltung listet sie mit «Nicht in der Bibliothek», ohne Poster und ohne Knöpfe
  (`web/src/features/admin/spotlight/columns.tsx:106,126-129`), obwohl der Server ein Anheften auch ohne
  Treffer annimmt (`server/internal/admin/spotlight.go:178-189`).
- **Je Eintrag** nur Medienart, Position, TMDB-ID, Trakt-Titel, Jahr. Kein Poster, keine Beschreibung,
  keine Freigabe (`server/internal/trakt/trakt.go:186-192`).
- **Job** `trakt.refresh` alle `interval_hours`, 100 Einträge je Medienart, 1 Anfrage/s. Ein Lauf ersetzt
  die Tabelle; ein Fehlschlag **leert** sie (`server/internal/metadata/trakt.go:278-313`). Einträge älter
  als 2 × `interval_hours` erscheinen nicht (`server/internal/visibility/discover.go:395`).
- **Ordnung:** Dedup, Angeheftete einfügen, `min_hits` prüfen, auf `item_count` schneiden
  (`server/internal/visibility/discover.go:383-447`). `visibility.MaxRowItems = 20`
  (`server/internal/visibility/visibility.go:31-32`).

Die vollständige Liste der heutigen Optionen der Verwaltung steht in `05-spotlight-integration.md`
Abschnitt 4.

## 3. Startseite

- Elf Reihenarten (`HomeRowKey`, `api/openapi.yaml:12134-12142`; CHECK
  `server/migrations/00039_radio_stage2.sql:65-67`), höchstens 15 sichtbare
  (`server/internal/model/model.go:997-999`). Reihen laden parallel (`server/internal/discover/discover.go:240-270`).
- `getHome` liefert `HomePage {rows, heroRow, heroStart, heroIntervalSeconds}`
  (`api/openapi.yaml:1234-1300`). Titel sind `ItemRef`, also **nur Bibliothekstitel**, ohne `tmdbId`
  (`api/openapi.yaml:11677-11804`).
- **Bühne** `web/src/ui/HeroCarousel.tsx`, eingebunden in `web/src/features/catalog/HomePage.tsx:586-616`.
  Knöpfe «Abspielen», «Details», «Trailer», «Pause»; Posterreihe darunter stellt einen Titel ins Band.
  An Postern Stift, Merken, Gesehen-Haken (`HomePage.tsx:317-339`).
- **Caching:** `homeQuery` mit `staleTime` 30 s (`web/src/features/catalog/queries.ts:28-55`).
  **Prepaint** (Vormalen): Die Server-Hülle `GET /` malt das erste Bühnenbild vor und lädt `/home` vor,
  nur wenn Spotlight die erste Reihe ist (`server/internal/api/preload.go:24-27,100,194`). Bilder: aktive
  Folie sofort, die nächsten zwei danach (`HeroCarousel.tsx:210-215`).
- Bühne und Prepaint setzen abspielbare Bibliothekstitel voraus (`HeroCarousel.tsx:19-51`).

## 4. Detailseiten

- Eine Seite `web/src/features/catalog/ItemPage.tsx` für alle Arten (Route `/items/$itemId`,
  `web/src/app/router.tsx:361-365`). Knöpfe in `DetailActions.tsx:165-340`.
- Bei Serien ein Staffelregal (`SeasonShelf.tsx`). `ItemDetail.seasons` sind die **sichtbaren**
  Staffeln (`api/openapi.yaml:11871-11882`).
- **Die Datenbank kennt keine Gesamtzahl der Staffeln oder Folgen**, keinen Status «läuft noch» und
  keine nächste Ausstrahlung (`server/migrations/00003_libraries_items.sql:46-76`). TMDB liefert
  `seasons[].episode_count` (`server/internal/metadata/tmdb.go:239-245`), genutzt nur für
  Staffelposter (`server/internal/metadata/match.go:493-496`). «Teilweise vorhanden» ist heute nicht
  erkennbar.

## 5. Suche

- `search` (POST `/search`, `scoped`) sucht im Volltextindex und verbindet immer mit `items` und
  `visible_items` (`server/internal/visibility/search.go:176-186,209-229`). **Nur Bibliothek.**
- Suchseite `web/src/features/discover/SearchPage.tsx:154-340` mit Abschnitten «Titel», «Personen»,
  «Sammlungen».
- **Eine Seite «Entdecken» gibt es nicht.** Der Ordner `features/discover` hält Suche, Merkliste,
  Gesehen, Sammlungen und Benachrichtigungen. Fassung 3 schlägt einen Bereich «Wünsche» mit «Entdecken»,
  «Filme», «Serien» und «Suche» vor; seine Daten kämen aus Jellyseerr (Abschnitt 17.6).
- TMDB-Suche gibt es serverseitig nur für den Administrator (`searchMetadata`,
  `api/openapi.yaml:4963-4985`).

## 6. Externe IDs und ihre Lücke

| Spalte | Herkunft | Beleg |
|---|---|---|
| `items.tmdb_id` | Ordner-/Dateiname, NFO, TMDB-Match | `server/migrations/00003_libraries_items.sql:59,81` |
| `items.imdb_id` | Tag, NFO, TMDB, TVDB | `server/migrations/00003_libraries_items.sql:60,82` |
| `items.tvdb_id` | Tag, NFO, TMDB, TVDB-Match | `server/migrations/00008_library_complete.sql:23,32` |
| Trakt-ID | nicht vorhanden | – |

- Gesetzt beim Scan (Name oder NFO) oder nach `metadata.match` (`server/queries/system/metadata.sql:4-26`).
- **Lücke:** Eine Serie, die TVDB zuerst zuordnet, bekommt `tvdb_id` und `imdb_id`, aber **keine**
  `tmdb_id` (`server/internal/metadata/tvdb.go:430-434`). Sie trifft den Trakt-Eintrag nie: Sie fehlt heute im
  Spotlight, obwohl sie in der Bibliothek steht, und die Verwaltung führt sie als «Nicht in der Bibliothek».
- Eine allgemeine Abfrage «Bibliothekstitel zu einer externen ID» fehlt.

## 7. Konten, Profile, Rechte

- Drei Ebenen: **Administrator** (Server), **Konto** (E-Mail, Passwort, eigene Profile, Kanäle),
  **Profil** (schaut) (`docs/adr/0014-benutzerverwaltung.md:66`). Das **Hauptprofil** (`is_default`)
  verwaltet die Profile des Kontos (`docs/adr/0005-profilwahl-sitzung.md:34`).
- `accounts`: u. a. `role`, `is_owner`, `max_rating`, `all_libraries`
  (`server/migrations/00063_minimum_age_accounts.sql:27-45`). `profiles`: u. a. `max_rating`,
  `pin_hash`, `language` (`server/migrations/00061_profiles_open_language.sql:34-57`).
- **Kein Merkmal «Kinderprofil».** Ein Kinderprofil ist ein Profil mit niedriger Altersstufe.
- **Keine Einzelschalter.** Rechte ergeben sich aus Rolle, Hauptprofil, Sichtgrenzen und
  Wiedergabegrenzen. Einziges `can…`-Feld im Vertrag ist `canRecommend` (`api/openapi.yaml:11832`); der
  Server prüft, ein Nein antwortet 404 (`docs/adr/0028-user-empfehlungen-von-profil-zu-profil.md:51-55`).
- Kontogrenzen setzt nur der Administrator im Formular «Zugriff» (`updateAccountAccess`,
  `web/src/features/admin/accounts/AccessForm.tsx`); Profilwerte das Hauptprofil im Profildialog
  (`ProfileForm.tsx`).
- Sitzung und Stufen: `authorize` setzt `x-visibility` durch (`server/internal/api/session.go:88-133`);
  `scoped` verlangt ein aktives Profil, `account` nur eine Sitzung.
- **Einladung annehmen:** Operation `acceptInvitation` (`api/openapi.yaml:1168 ff.`), Seite `/invite/$token`
  (`web/src/app/router.tsx:237`). Neue Konten entstehen nur so.
- **Wann der Server das Passwort im Klartext sieht:** genau in drei Momenten, beim Annehmen der Einladung, bei
  jeder Anmeldung und bei `updateAccount` (eigenes Konto, `web/src/features/admin/accounts/OwnAccountForm.tsx`).
  (Anhang H 3.6).
- **Mindestlänge** des Passworts: 10 Zeichen (`server/internal/auth/auth.go:40`).
- Folge für Fassung 3: Will Slouchcat dasselbe Passwort in Jellyseerr setzen, geht das nur in diesen drei
  Momenten. Dazwischen hat der Server das Passwort nicht.

## 8. Jugendschutz

- Alle Reihen und die Suche lesen über die View `visible_items`: nicht gelöscht, `available = 1`,
  `COALESCE(effective_rating, 18) <= min(Profil, Konto)`, freigegebene Bibliotheken
  (`server/migrations/00063_minimum_age_accounts.sql:86-112`).
- Freigaben sind Mindestalter 0–18 je Freigabeland. Lesereihenfolge: gewähltes Freigabeland, dann DE, AT, CH,
  GB, US; das erste Land mit bekanntem Wert gilt (`server/internal/metadata/ratings.go:113-125`,
  `ageFromCertifications`). Innerhalb eines Lands gilt der strengste Wert über die Anbieter. **Unbekannt = 18**
  (`docs/adr/0034-freigabelaender-und-altersstufen-je-land.md:28-38`, Reihenfolge Z. 32).
- **`visible_items` gilt nur für Bibliothekstitel.** Freigaben hängen an `items`
  (`server/migrations/00064_item_certifications.sql:26-33`). Ein fehlender Titel hat keine Freigabe und
  kann die View nicht passieren.
- Nicht sichtbar heisst 404, nie 403 (`CLAUDE.md` Regel 2).

## 9. Benachrichtigungen

| Teil | Heute | Beleg |
|---|---|---|
| **Eingang** (In-App) | je Profil, nur Art `new_episode`, `item_id` Pflicht, eindeutig je (`profile_id`, `kind`, `item_id`) | `server/migrations/00015_discover.sql:329-341` |
| **Kanäle** | E-Mail, Web-Push, Telegram, je **Konto** (`account_channels`) | `server/migrations/00016_admin.sql:27-35` |
| **Ereignisse** | `new_content`, `errors`, `invitations`, `announcements`; CHECK in `deliveries.event` | `server/internal/model/notify.go:42-47`, `server/migrations/00018_drop_acquisition_channel.sql:21-36` |
| **Standardkanäle** | gewöhnliches Konto: nur E-Mail an, nur `announcements`; Administrator: alle aus | `server/queries/system/notify_channels.sql:7-18` |
| **Betreff** | `[Profilname]` davor bei Profil-Ereignissen | `server/internal/notify/dispatch.go:102-109` |
| **Sichtbarkeit** | einmal beim Erzeugen geprüft, nicht beim Senden | `server/internal/notify/notify.go:186-220` |

- Ein gewöhnliches Konto bekommt `new_content` ab Werk auf keinem Kanal.
- Kanäle ändern darf nur das Hauptprofil (`server/internal/notify/channels.go:100-109`).
- Eine Regel für PIN-Profile oder «Titel extern nur, wenn …» gibt es nicht.
- Die App setzt die Art fest voraus: `NotificationsPage.tsx` zeigt immer «Neue Folge».
- Der Zähler der Glocke fragt alle 30 s (`web/src/features/discover/queries.ts:106-115`).

## 10. Radarr und Sonarr: Webhooks und API

### 10.1 Heute in Slouchcat

- Nur eingehend: `POST /webhooks/{token}`, `public` (`server/internal/api/webhook_handlers.go:45-54`). Die
  Webhooks richtet der Administrator in Radarr und Sonarr von Hand ein (Seite «Webhooks von Radarr und Sonarr»).
  Slouchcat ruft Radarr und Sonarr heute nie auf (`server/internal/integrations/integrations.go:5-6`; ADR 0010,
  Anhang C 1).
- Wirkung: ein Ordner-Scan `scan.path` mit Entprellung 2 s und Priorität 6
  (`server/internal/integrations/receive.go:88-92`; Anhang C 1).
- **Slouchcat liest nur einen Teil der Meldung:** Das Modell liest nur Ereignis, Film- und Serienpfad
  (`server/internal/model/integrations.go:45-51`). Kennungen und der genaue Dateipfad bleiben ungenutzt
  (Anhang H 8.3).

### 10.2 Was die Webhook-Meldungen tragen

Gelesen in den aufgezeichneten Meldungen `testdata/webhooks/radarr-6.4.4.10685-download.json` und
`testdata/webhooks/sonarr-4.0.20.3014-download.json` (Anhang H 8.3):

| Dienst | Felder (Auswahl) |
|---|---|
| Radarr | `movie.tmdbId`, `movie.imdbId`, `movie.folderPath`, `movieFile.path` |
| Sonarr | `series.tvdbId`, `series.tmdbId`, `series.imdbId`, `series.path`, `episodeFile.path`, `episodes[]` |

Die Meldung nennt also schon, welcher Titel es ist und welche Datei neu ist. Slouchcat nutzt davon heute nur den
Ordner.

### 10.3 Geprüft gegen Marcos Radarr und Sonarr

Geprüft gegen Radarr 6.4.4.10685 und Sonarr 4.0.20.3014, nur mit lesenden Aufrufen. Gelesen wurden Versionen,
Feldnamen und Anzahlen, keine Schlüssel und keine Pfade (Anhang H 8.3).

| Fakt | Beleg |
|---|---|
| Von sich aus melden Radarr und Sonarr nur über «Connect» (Webhook, Skript und Ähnliches). Die API ist rein abfragend. Wer ohne Webhook auskommen will, muss in einem Takt nachfragen. | beide Instanzen |
| Ein Webhook hat die Felder `url`, `method`, `username`, `password`, `headers` und Schalter je Ereignis (`onDownload`, `onUpgrade`, `onRename`; bei Radarr `onMovieDelete`, `onMovieFileDelete`; bei Sonarr `onImportComplete`, `onSeriesDelete`, `onEpisodeFileDelete` und weitere). | `GET /api/v3/notification`, `/api/v3/notification/schema` |
| Die Warteschlange trägt je Eintrag `size`, `sizeleft`, `timeleft`, `estimatedCompletionTime`, `status`, `trackedDownloadState`, `trackedDownloadStatus`, `errorMessage`, `statusMessages`, `downloadClient`, bei Radarr `movieId`. Bei Sonarr war sie beim Test leer; `seriesId`, `episodeId`, `seasonNumber` stehen nur in der Dokumentation (**nicht geprüft**). | `GET /api/v3/queue` |
| Je Import: `date`, `movieId` (Radarr) oder `seriesId` und `episodeId` (Sonarr), in `data` `importedPath`, `droppedPath`, `fileId`. | `GET /api/v3/history/since?date=…&eventType=downloadFolderImported` |
| Film: `hasFile`, `movieFile.path`, `path`, `tags`, `tmdbId`, `imdbId`. Serie: `path`, `seasons`, `statistics` (`episodeFileCount`, `episodeCount`, `percentOfEpisodes`), `tags`, `tvdbId`, `tmdbId`, `imdbId`. | `GET /api/v3/movie?tmdbId=…`, `GET /api/v3/series?tvdbId=…` |
| Tags: `id`, `label`. Stammordner: `path`, `accessible`, `freeSpace`. | `GET /api/v3/tag`, `GET /api/v3/rootfolder` |
| Schreibende Aufrufe (`POST /api/v3/tag`, `PUT /api/v3/movie/editor`, `PUT /api/v3/series/editor`) wurden nicht ausgeführt. | **nicht geprüft** |

### 10.4 Folge

- **Radarr und Sonarr können nur über einen Webhook von sich aus melden; die API beantwortet nur Fragen.** Eine
  «direkte» Anbindung ohne Webhook hiesse, im Takt nachzufragen: langsamer als die Sofortmeldung und mit
  dauernder Grundlast.
- Marco hat entschieden, dass die Webhooks bleiben, wie sie heute sind, von Hand eingerichtet (E-6). Slouchcat
  trägt in Radarr und Sonarr keinen Webhook ein. Die Wahl «API und Webhook» (Slouchcat richtet den Webhook selbst
  ein, holt Verpasstes über `history/since` nach) und «Nur API, ohne Webhook» hat er nicht genommen.
- Die API nutzt Slouchcat nur für zwei Dinge: den Profil-Tag (Marco, E-2) und das Lesen des
  Download-Fortschritts (Marco, E-7). Gelesen werden die Warteschlange und der Dateistand der gewünschten Titel
  (`queue`, `movie?tmdbId`, `series?tvdbId`); den Dateistand braucht es für «Wird gesucht» und «Wird
  eingelesen» (Umfang des Lesens Redaktion, Anhang H 9, K-5). Der Import in die Bibliothek kommt weiter über
  den Webhook; gelesen wird nur für die Anzeige am Wunsch. Beides ist eine Ausnahme von ADR 0010 (Anhang H 8.5).
- Für schnelleres Einlesen braucht es keine neue Anbindung: Die Webhook-Meldung nennt schon Kennung und
  Dateipfad (Abschnitt 10.2). Ob Slouchcat sie nutzt, ist ein Vorschlag der Redaktion und offen als F-33
  (Anhang H, N-88).

## 11. Ausgehende Aufrufe und `netguard`

- `netguard` sperrt Ziele im Heimnetz (LAN): Loopback, 10/8, 172.16/12, 192.168/16, CGNAT und weitere,
  geprüft nach der DNS-Auflösung und bei jeder Weiterleitung (`server/internal/netguard/netguard.go:29-82`).
  `http://192.168.1.x:5055` würde abgewiesen.
- Erlaubte LAN-Wege heute: Podcast-Feeds, die der Administrator abonniert (`netguard.go:113-128`),
  SMTP-Server (`server/internal/notify/email/email.go:55-56,124-130`), Updater ohne Weiterleitungen
  (`server/internal/admin/update_install.go:107-113`).
- Gegenbeispiel: eigene Radiosender lehnen LAN-Adressen ab (`server/internal/radio/own.go:31-37`).
- Rate-Limits über `ratelimit` (`server/internal/ratelimit/ratelimit.go:1-30`); Fehlertexte nennen nur
  Schema und Host (`netguard.go:130-150`).

## 12. Secrets

- AES-256-GCM, Schlüssel in `<config>/secret.key`, nie in der DB (`server/internal/secrets/secrets.go:1-67`).
- Gespeichert in `settings` über `setSecret` (`server/internal/admin/settings.go:630-640`); Antwort
  nennt nur Quelle und die letzten vier Zeichen (`settings.go:56,192-212`). Umgebung gewinnt.
- Redaction-Liste (Regel 10) deckt `apikey` und damit `X-Api-Key` ab
  (`server/internal/logging/logging.go:43-56`).
- Fassung 3 schickt Passwörter an Jellyseerr und liest die Schlüssel von Radarr und Sonarr aus Jellyseerr.
  Anhang H 5 nennt dafür neue Schlüssel für die Redaction-Liste: `password`, `newPassword` und die Schlüssel von
  Radarr und Sonarr.

## 13. Hintergrundjobs

- Queue mit Idempotenz-Schlüssel, Backoff 10 s·2^(n−1), Standard 5 Versuche, Priorität 0–9
  (`docs/adr/0002-job-queue.md:54-85`, `server/internal/jobs/queue.go:11-29,76-80`).
- Scheduler mit Intervallen von 5 min bis 7 d, Zeitfenster, Pause, «Jetzt ausführen»
  (`server/internal/jobs/scheduler.go:53-54`, `server/internal/jobs/registry.go:15-110`).
- Eine neue Jobart braucht Konstante, Handler, Registrierung, Verdrahtung in
  `server/cmd/slouchcat/main.go:418-443` und Texte `admin.jobs.types.*`. Keine Migration.

## 14. Vom Scan bis «abspielbar»

1. Auslöser: inotify, Poller (NFS/SMB, mindestens 45 s) oder Webhook → `scan.path`; Vollscan alle 6 h
   (`server/internal/library/doc.go:88-97,143-148`).
2. Scan: neuer Film über TMDB-, IMDb-ID oder Titel und Jahr; sonst neuer Titel plus `metadata.match`
   (`server/internal/library/doc.go:21-42`).
3. Sichtbar nach dem Scan-Commit, für Profile unter 18 aber erst mit Freigabe (View `COALESCE(…, 18)`).
4. `metadata.match` schreibt die IDs, danach der Haken `WithRelink`
   (`server/internal/metadata/match.go:312-330`).
- **Kein Haken für neue Filme oder neue Serien.** `WithNewEpisodes` meldet nur Folgen und feuert vor dem
  Match (`server/internal/library/write.go:280-295`). Das Ereignis `item.available` aus dem Plan
  (`docs/planning/03-plan-v1.md:164`) existiert im Code nicht.
- Verzögerung: Webhook ~2 s plus Scan; SMB-Poller ab 45 s; ohne Watcher bis 6 h; Match hängt an der Queue.

## 15. Verwaltungsrahmen

Sechs Bereiche (ADR 0035, `web/src/features/admin/sections.ts:15-25,59-91`):

| Bereich | Seiten |
|---|---|
| Übersicht | `/admin` |
| Medien | Bibliotheken, Metadaten, Pflege, Podcasts, Radio, Hörbuch-Quellen |
| Nutzer & Geräte | Konten, Geräte, Streams |
| Startseite | Startseite, Spotlight, User-Empfehlungen, Sammlungen, Meine Medien, Ankündigungen |
| Automatisierung | Hintergrundjobs, Integrationen (sechs Kacheln, `services.tsx:12`), Benachrichtigungen |
| System | Statistiken, Sicherung, Server |

Regel des Umbaus: keine Funktion und keine Option darf verloren gehen (`docs/adr/0035-verwaltung-in-sechs-bereichen.md:21-22`).

## 16. API-Konventionen, Migrationen, PWA

- Jede Operation trägt `x-visibility` (`api/openapi.yaml:9-16`); Antworten mit `ItemRef` sind `scoped`;
  jede Operation braucht einen Eintrag im Vertragstest
  (`server/internal/api/contract_access_test.go:494`).
- Fehler als `application/problem+json` mit `code` (`api/openapi.yaml:39-41`); Nicht-GET braucht
  `X-Requested-With: Slouchcat` (`:33-35`); Seiten mit `offset`/`limit`.
- Bilder und Streams als `http.Handler`, JSON über den strict server (`CLAUDE.md` Regel 6).
- Migrationen nur vorwärts nach `server/migrations/TEMPLATE.sql.txt`.
- **PWA:** Der Service Worker cacht nur Bilder unter `/api/v1/items/{id}/images/…` (CacheFirst, 300
  Einträge, 30 Tage); alles andere unter `/api` ist NetworkOnly (`web/vite.config.ts:141-161`).
  Regel 12: Der Client entscheidet keine Sichtbarkeit (`docs/planning/03-plan-v1.md:38`).

## 17. Jellyseerr/Seerr: was die API wirklich kann

**Jellyseerr** ist ein Dienst zum Wünschen von Filmen und Serien. Er reicht freigegebene Wünsche an
**Radarr** (Filme) und **Sonarr** (Serien) weiter, die sie beschaffen. Seit Februar 2026 heisst das
Projekt **Seerr** (3.x); das alte Repo leitet um, die Pfade bleiben unter `/api/v1`
(`server/index.ts` Z. 264). Quelle für alles Folgende: Seerr v3.5.2.

### 17.1 Was geht

| Fähigkeit | Wie | Quelle (Seerr v3.5.2) |
|---|---|---|
| Anmelden als Dienst | ein Admin-Schlüssel je Instanz, Header `X-Api-Key` | `server/middleware/auth.ts` Z. 9–34 |
| Im Namen eines Benutzers handeln | zusätzlich `X-API-User: <id>`; dann gelten dessen Rechte, Limit und Auto-Freigabe | `server/middleware/auth.ts` Z. 9–34; `server/entity/MediaRequest.ts` Z. 107–167, 340–353 |
| Wunsch anlegen | `POST /request` mit `mediaType`, `mediaId` (TMDB-ID), bei Serien `seasons` und `tvdbId` | `server/interfaces/api/requestInterfaces.ts` Z. 16–30 |
| Zustand lesen | `GET /request` (Seiten, `sort=modified`), `GET /request/{id}`, `GET /movie\|tv/{tmdbId}` mit `mediaInfo.requests` | `server/routes/request.ts` Z. 38–311, 438–468; `server/entity/Media.ts` Z. 98–115 |
| Freigeben, ablehnen | `POST /request/{id}/approve\|decline`, nur bei `PENDING` | `server/routes/request.ts` Z. 794–848 |
| Zurückziehen | `DELETE /request/{id}`, eigener Wunsch nur bei `PENDING` | `server/routes/request.ts` Z. 719–752 |
| Erneut senden | `POST /request/{id}/retry`, nur bei `FAILED` | `server/routes/request.ts` Z. 754–792 |
| Benutzer lesen | `GET /user` mit Suche, Rechten (Bitmaske), Limit (Quota) | `server/routes/user/index.ts` Z. 36–166; `server/lib/permissions.ts` Z. 1–32 |
| Benutzer anlegen, Passwort, Rechte, Limit setzen | `POST /user`, `POST /user/{id}/settings/password`, `/settings/permissions`, `/settings/main` (Abschnitt 17.4) | Anhang H 3.1 |
| Limit lesen | `GET /user/{id}/quota` (Abschnitt 17.5) | Anhang H 3.2 |
| Listen zum Stöbern und Suche | `GET /discover/…`, `GET /search` (Abschnitt 17.6) | Anhang H 3.3 |
| Zugang zu Radarr und Sonarr lesen | `GET /settings/radarr`, `/settings/sonarr` (Abschnitt 17.8) | Anhang H 3.5 |
| Version ohne Anmeldung | `GET /status` | `server/routes/index.ts` Z. 50–102 |
| Webhook bei Ereignissen | ein Ziel, Ereignisse als Bitmaske | `server/lib/notifications/agents/webhook.ts` Z. 17–64 |

Zustände: Request `PENDING 1, APPROVED 2, DECLINED 3, FAILED 4, COMPLETED 5`; Medium `UNKNOWN 1,
PENDING 2, PROCESSING 3, PARTIALLY_AVAILABLE 4, AVAILABLE 5, BLOCKLISTED 6, DELETED 7`
(`server/constants/media.ts` Z. 1–22). Die OpenAPI-Datei von Seerr ist hier ungenau; massgeblich ist der Code.

### 17.2 Was es NICHT gibt

| Fehlt | Folge | Quelle |
|---|---|---|
| `X-API-User` in Doku und Spec | Verhalten ist undokumentiert, aber seit Overseerr unverändert; `auth.ts` ist zwischen v2.7.3 und v3.5.2 byte-gleich | `seerr-api.yml` (0 Treffer), `server/middleware/auth.ts` |
| Feld für fremde Kennungen am Benutzer oder Request | Slouchcat merkt sich Profil und Request-ID selbst | `server/entity/User.ts` Z. 56–161; `server/entity/MediaRequest.ts` Z. 518–618 |
| Mehrere Benutzer an einem Wunsch | Duplikat ist global: zweiter Wunsch → 409 | `server/entity/MediaRequest.ts` Z. 218–246 |
| Ablehnungsgrund | «Abgelehnt» ohne Text | `server/routes/request.ts` Z. 794–848 |
| Sammelabruf nach TMDB-IDs | Zustand nur seitenweise über `/request` oder `/media` | `server/entity/Media.ts` Z. 37–60 (nur intern) |
| Änderungs-Feed «seit X» | Ersatz: `sort=modified` absteigend, abbrechen bei älterem `updatedAt`; unbekannte Parameter → 400 | `server/routes/request.ts` Z. 38–311; `server/index.ts` Z. 239–244 |
| Einzelne Folgen anfordern | nur ganze Staffeln; Staffel 0 und leere Staffeln fallen bei `all` weg | `server/entity/MediaRequest.ts` Z. 383–439 |
| Mehrere Webhook-Ziele, Signatur, Wiederholung, Benutzer-ID in der Nutzlast | Webhook taugt nur als Anstoss | `server/lib/notifications/agents/webhook.ts` Z. 17–64, 202–243; `server/routes/settings/notifications.ts` Z. 276–330 |
| Schreiben bei eingeschaltetem CSRF-Schutz | alle POST/DELETE von Slouchcat scheitern | `server/index.ts` Z. 195–214 |
| Route, um Tags in Radarr oder Sonarr anzulegen | Jellyseerr liest Tags nur; den Profil-Tag muss Slouchcat selbst in Radarr und Sonarr anlegen | `server/routes/service.ts` Z. 32–162 (Anhang H 3.5) |
| Passwort von Benutzer 1 ohne sein altes Passwort setzen | Benutzer 1 bleibt beim Passwortabgleich aussen vor | `server/routes/user/usersettings.ts` Z. 222–232 (Anhang H 3.1) |

### 17.3 Fallen

- **`userId` im Body statt `X-API-User`:** Dann entscheidet der Aufrufer (Benutzer 1, `ADMIN`) über die
  Auto-Freigabe. Jeder Wunsch wäre sofort genehmigt (`server/entity/MediaRequest.ts` Z. 87–101, 340–353).
- **TVDB-ID fehlt bei einer Serie:** Beim Senden an Sonarr löscht Seerr Medium und Request
  (`server/subscriber/MediaRequestSubscriber.ts` Z. 618–625).
- **Kein Staffelrest:** Sind alle gewählten Staffeln schon angefragt oder da, antwortet Seerr mit **202**
  «No seasons available to request» (`server/entity/MediaRequest.ts` Z. 396–439).
- **«AVAILABLE» kommt auch ohne Medienserver:** Der Radarr- und Sonarr-Scan von Seerr setzt es, sobald
  die Datei da ist (`server/lib/scanners/radarr/index.ts` Z. 164–182, `server/lib/scanners/baseScanner.ts`
  Z. 152–162). Das heisst nicht, dass Slouchcat die Datei schon eingelesen hat.
- **Benutzer löschen löscht seine Requests** (`server/entity/MediaRequest.ts` Z. 532–537).
- **`GET /request` fragt bei jedem Aufruf alle Radarr/Sonarr ab** (`server/routes/request.ts` Z. 196–224):
  sparsam abfragen.
- **`POST /user/{id}/settings/main` schreibt immer alle Felder:** Was im Body fehlt, wird geleert. Vorher lesen,
  dann alles zurückschicken (Abschnitt 17.4).
- **`tags` im Request ersetzen die Standard-Tags** des Servers und wirken nur mit `MANAGE_REQUESTS` oder
  `REQUEST_ADVANCED` (Abschnitt 17.8).
- **Abweichende Staffelzählung:** Steht Jellyseerr auf TMDB, kann seine Staffelliste von der in Sonarr und in der
  Bibliothek abweichen (Abschnitt 17.7).

### 17.4 Benutzer anlegen, Passwort, Rechte

Quelle Anhang H 3.1, `GH:` = Seerr v3.5.2.

| Fakt | Beleg | Folge |
|---|---|---|
| `POST /user {email, username, password}` legt einen lokalen Benutzer an. Mit `password` braucht Jellyseerr kein SMTP. `permissions` im Body wird ignoriert, es gelten die Standardrechte. Gleiche E-Mail → 409. | Anhang D 7.2 | Slouchcat kann je Konto einen Benutzer eröffnen; 409 heisst «gibt es schon»: verknüpfen, wenn ein Administrator eröffnet; sonst nur ein Vorschlag (N-2a, `04-benutzer-profilmapping.md`, Abschnitt 12.5). |
| `POST /user/{id}/settings/password {newPassword}`, mindestens 8 Zeichen. Mit `MANAGE_USERS` setzt man das Passwort eines **anderen** Benutzers ohne das alte. Das Passwort von Benutzer 1 ändert nur Benutzer 1, das eines Administrators nur er selbst oder Benutzer 1. Für das eigene Passwort verlangt Jellyseerr das alte. | `GH:server/routes/user/usersettings.ts` Z. 222–261 | Der Schlüssel handelt als Benutzer 1 (Anhang D 2.1): Slouchcat kann jedes Passwort setzen, **ausser das von Benutzer 1**. |
| Slouchcat verlangt 10 Zeichen, Jellyseerr 8. | `server/internal/auth/auth.go:40` | Jedes Slouchcat-Passwort passt. |
| `POST /user/{id}/settings/main` schreibt `username`, `email`, Sprache, Regionen und die vier Limit-Felder **immer alle** aus dem Body. Die Limit-Felder nimmt Jellyseerr nur an, wenn der Zielbenutzer **nicht** `MANAGE_USERS` hat und der Aufrufer nicht er selbst ist. | `usersettings.ts` Z. 77–150, 120–129 | Vorher `GET /user/{id}/settings/main`, übrige Felder unverändert zurückschicken. Administratoren bekommen kein Limit. |
| `POST /user/{id}/settings/permissions {permissions}` setzt die Rechte. | `usersettings.ts` Z. 763–808 | Rechte setzt ein zweiter Aufruf nach dem Anlegen. |
| «Lokale Anmeldung» ist ab Werk an (`localLogin: true`) und steht in `GET /settings/public`. Ist sie aus, kann sich ein lokaler Benutzer nicht anmelden. | `GH:server/lib/settings/index.ts` Z. 423 | Der Verbindungstest muss sie prüfen. |

### 17.5 Limit (Quota)

Quelle Anhang H 3.2, `GH:server/entity/User.ts` Z. 285–390.

- Vier Felder je Benutzer: `movieQuotaLimit`, `movieQuotaDays`, `tvQuotaLimit`, `tvQuotaDays`. Fehlt ein Feld,
  gilt der Vorgabewert der Instanz (`defaultQuotas`).
- Gezählt wird **rollierend**: Requests des Benutzers mit `createdAt` innerhalb der letzten `…Days` Tage. Filme:
  Zahl der Requests. Serien: **Summe der Staffeln** dieser Requests. Abgelehnte Requests (`DECLINED`) zählen
  nicht, fehlgeschlagene zählen. Ein gelöschter Request zählt nicht mehr, weil es ihn nicht mehr gibt.
- Wer `MANAGE_USERS` hat (also auch Administratoren), hat kein Limit (Z. 290–296).
- `GET /user/{id}/quota` liefert je Art `days`, `limit`, `used`, `remaining`, `restricted` (Anhang D 7.2).
- Ist das Limit erreicht, antwortet `POST /request` mit 403 `Movie Quota exceeded.` oder `Series Quota exceeded.`
  (Anhang D 3.4).
- Folge: Jellyseerr zählt auch Wünsche, die jemand direkt in Jellyseerr stellt. Einen Zeitpunkt «nächster Platz
  frei» liefert Jellyseerr nicht; Slouchcat muss ihn aus den Requests errechnen.

### 17.6 Listen zum Stöbern und Suche

Quelle Anhang H 3.3, `GH:server/routes/discover.ts`.

| Zweck | Route | Parameter (Auswahl) |
|---|---|---|
| Im Trend | `GET /discover/trending` (Z. 729) | `page`, `mediaType`, `timeWindow`, `language` |
| Filme | `GET /discover/movies` (Z. 105) | `page`, `sortBy`, `genre`, `language`, `certification`, `certificationGte/Lte`, `certificationCountry` u. a. |
| Demnächst (Filme) | `GET /discover/movies/upcoming` (Z. 367) | `page`, `language` |
| Serien | `GET /discover/tv` (Z. 418) | wie Filme, dazu `firstAirDateGte/Lte`, `status` |
| Demnächst (Serien) | `GET /discover/tv/upcoming` (Z. 679) | `page`, `language` |
| Genre-Kacheln | `GET /discover/genreslider/movie`, `/genreslider/tv` (Z. 855, 899) | `language` |

- Jede Liste ist ein TMDB-Abruf; die Treffer tragen `mediaInfo` (Zustand in Jellyseerr).
- Die Startseite von Jellyseerr zeigt ab Werk zwölf Reihen (`GH:server/constants/discover.ts`), darunter
  «Recently Added», «Recent Requests», «Plex Watchlist», «Studios» und «Networks»; sie lassen sich in Jellyseerr
  umstellen.
- `GET /search?query=…` sucht bei TMDB, mit vier Präfixen: `tmdb:<id>`, `imdb:<tt…>`, `tvdb:<id>`,
  `year:<jjjj>` (`GH:server/lib/search.ts` Z. 46, 94, 132, 170). `tvdb:` löst über TMDB «find by external id»
  auf.
- Folge: Eine Seite wie die Startseite von Jellyseerr lässt sich aus diesen Routen bauen. `mediaInfo` sagt nichts
  über die Bibliothek von Slouchcat; den Abgleich muss Slouchcat selbst machen. Freigaben liefern die Listen
  nicht je Treffer; für Profile mit Altersgrenze hilft `certificationLte` als Vorfilter.

### 17.7 TVDB in Jellyseerr

Quelle Anhang H 3.4.

- Jellyseerr führt jeden Titel über die **TMDB-ID**. Die TVDB-ID einer Serie nimmt es aus TMDB
  (`external_ids.tvdb_id`, `GH:server/api/tvdb/index.ts` Z. 550–552) und gibt sie an Sonarr weiter. Sonarr
  arbeitet mit der TVDB-ID. Fehlt sie, löscht Jellyseerr den Request beim Senden (Abschnitt 17.3).
- Seit 3.x gibt es die Einstellung «Metadaten-Anbieter» für Serien und Anime: `tmdb` (ab Werk) oder `tvdb`
  (`GH:server/lib/settings/index.ts` Z. 111–118, 460–463; Route `/settings/metadatas`). Mit `tvdb` lädt
  Jellyseerr die Serie weiter über die TMDB-ID, ersetzt aber die **Staffelliste** durch die von TVDB
  (`GH:server/api/tvdb/index.ts` Z. 136–260; `GH:server/api/metadata.ts` Z. 8–39).
- Folge: Der Schlüssel bleibt immer die TMDB-ID. Was sich unterscheiden kann, ist die Staffelzählung: TMDB und
  TVDB teilen manche Serien verschieden auf. Sonarr zählt nach TVDB, die Bibliothek von Slouchcat ordnet Serien
  TVDB-zuerst zu (Abschnitt 6, `server/internal/metadata/tvdb.go:430-434`).

### 17.8 Tags und der Zugang zu Radarr und Sonarr

Quelle Anhang H 3.5.

- `tags` im Body von `POST /request` sind numerische Tag-IDs von Radarr/Sonarr. Jellyseerr übernimmt sie nur,
  wenn der **anfordernde** Benutzer `MANAGE_REQUESTS` oder `REQUEST_ADVANCED` hat
  (`GH:server/entity/MediaRequest.ts` Z. 265–294). Sie **ersetzen** dann die Standard-Tags des Servers
  (`GH:server/subscriber/MediaRequestSubscriber.ts` Z. 299–302, 706–707).
- Jellyseerr kann selbst taggen: Mit «Tag Requests» am Radarr- oder Sonarr-Server legt es je anforderndem
  Benutzer den Tag `<Benutzer-ID>-<Anzeigename>` an und hängt ihn an (Z. 346–380). Das ist der Benutzer des
  Kontos, nicht das Profil.
- Jellyseerr liest die Tag-Liste (`GET /service/radarr/{id}`, `/service/sonarr/{id}`,
  `GH:server/routes/service.ts` Z. 32–162), bietet aber **keine Route, um Tags anzulegen**.
- `GET /settings/radarr` und `/settings/sonarr` geben die Servereinträge vollständig zurück, mit `hostname`,
  `port`, `useSsl`, `baseUrl`, `apiKey`, `tags`, `tagRequests`, `isDefault`, `is4k`
  (`GH:server/routes/settings/radarr.ts` Z. 9–13; `GH:server/lib/settings/index.ts` Z. 68–87).
- Radarr und Sonarr (API v3): `GET /api/v3/tag`, `POST /api/v3/tag {label}`, `GET /api/v3/movie?tmdbId=`,
  `PUT /api/v3/movie/editor {movieIds, tags, applyTags: "add"}`, `GET /api/v3/series?tvdbId=`,
  `PUT /api/v3/series/editor {seriesIds, tags, applyTags: "add"}`. Die lesenden Aufrufe sind gegen Marcos
  Instanzen geprüft (Abschnitt 10.3), die schreibenden stammen aus der öffentlichen Dokumentation und sind
  **nicht geprüft**. Tag-Namen: nur Kleinbuchstaben, Ziffern, Bindestrich. Sonarr kennt Tags nur an der Serie,
  nicht an der Staffel.
- Folge: Einen Tag je Profil kann nur Slouchcat selbst setzen, direkt in Radarr und Sonarr. Den Zugang dazu kann
  es aus Jellyseerr lesen, ohne dass der Administrator einen Schlüssel eintippt. Docker-interne Adressen wie
  `http://radarr:7878` sind von Slouchcat aus womöglich nicht erreichbar.

## 18. Anknüpfungspunkte

- `spotlight_entries` hält schon die Kandidaten «in Trakt, nicht in der Bibliothek».
- `spotlight_overrides` zeigt das Muster einer Tabelle ohne FK auf `items`, die Abrufe überlebt.
- `internal/trakt` ist ein reiner HTTP-Client ohne DB mit Rate-Limit: Vorbild für `internal/seerr`.
- Der TMDB-Client holt schon Details, `external_ids`, Freigaben und `seasons[].episode_count`
  (`server/internal/metadata/tmdb.go`).
- `canRecommend` zeigt das Muster «Server liefert `can…`, Client zeigt nur».
- `notify.Emit` mit Profilpräfix und Kanälen ist allgemein (`server/internal/notify/dispatch.go:74-121`).
- Die Stelle des Hakens `WithRelink` nach dem Match ist der natürliche Auslöser für «Titel zugeordnet».
- `acceptInvitation`, die Anmeldung und `updateAccount` sind die drei Stellen, an denen Slouchcat das Passwort an
  Jellyseerr weitergeben kann (Abschnitt 7).
- `GET /settings/radarr` und `/settings/sonarr` von Jellyseerr liefern den Zugang zu Radarr und Sonarr
  (Abschnitt 17.8). Derselbe Zugang liest die Warteschlange für den Download-Fortschritt (`GET /api/v3/queue`,
  Abschnitt 10.3).
- Die Webhook-Meldungen von Radarr und Sonarr tragen schon Kennungen und Dateipfad (Abschnitt 10.2); heute liest
  Slouchcat davon nur den Ordner.

## 19. Was dieses Papier nicht prüft

Laufzeitverhalten (keine Tests, keine Instanz von Jellyseerr angesprochen), die Zahl der Serien ohne
`tmdb_id` auf CT 190, die Sprache der Trakt-Titel und ob jede JSON-Antwort `Cache-Control: no-store` trägt.
Für Fassung 3 ausserdem nicht geprüft: ob Marcos Instanz «Lokale Anmeldung» an hat, auf welchem
Metadaten-Anbieter sie für Serien steht, ob «Tag Requests» an ist und ob Slouchcat die Adressen von Radarr und
Sonarr aus Jellyseerr erreicht. Von Radarr und Sonarr sind nur lesende Aufrufe geprüft; die schreibenden Aufrufe
für den Tag und die Felder einer Sonarr-Warteschlange mit Einträgen sind nicht geprüft (Abschnitt 10.3).

## 20. Technische Einschränkungen, die das Konzept bestimmen

Die Einschränkungen heissen TE-1 bis TE-32. «E-1» bis «E-7» stehen in diesem Konzept nur für Marcos Antworten
(Anhang H, Abschnitt 9, K-9).

| Nr. | Einschränkung | Folge im Konzept |
|---|---|---|
| TE-1 | Wünschen ist Nicht-Ziel (Abschnitt 1). | Frage 33, ADR 0037 und 0038 vor jedem Bau (Stufe 0). |
| TE-2 | `ItemRef` und `visible_items` gelten nur für Bibliothekstitel. | Eigenes Schema `ExternalTitleRef`, eigene Sichtbarkeitsregel in `visibility` für fehlende Titel. |
| TE-3 | Fehlende Titel haben keine Freigabe. | Freigabe aus TMDB nach ADR 0034, unbekannt = 18; nicht sichtbar heisst 404. |
| TE-4 | Spotlight-Einträge tragen nur TMDB-ID, Titel, Jahr; die Tabelle wird bei jedem Lauf ersetzt und bei Fehlern geleert. | Eigene Tabelle `external_titles`; Wunschzustand nie an `spotlight_entries` hängen. |
| TE-5 | Abgleich nur über TMDB-ID; TVDB-zuerst-Serien haben keine `tmdb_id` und fehlen heute im Spotlight. | Trakt-Einträge speichern TVDB- und IMDb-ID; Abgleich über alle drei, bei Serien TVDB zuerst wie die Bibliothek (`05-spotlight-integration.md`, Abschnitt 3). |
| TE-6 | Die Bildstrecke der App setzt eine `items.id` voraus; der Browser lädt nie direkt von TMDB. | Eigener Bildweg `/titles/{media}/{tmdbId}/images/…` über den Bildspeicher. |
| TE-7 | Die Datenbank kennt keine Gesamtzahl der Staffeln. | `series_catalog` für Serien in der Bibliothek. |
| TE-8 | Kein Wunschrecht, keine Einzelschalter. | Neue Spalten `can_request` je Konto und Profil; Server liefert `canRequest` mit Grund. |
| TE-9 | `netguard` sperrt das Heimnetz. | Eigener Client `internal/seerr` mit ausdrücklicher LAN-Erlaubnis, ohne Weiterleitungen. |
| TE-10 | Kein Haken für neue Filme und Serien; Freigabe kommt erst nach dem Match. | Haken nach dem Match plus stündlicher Nachlauf `wishes.reconcile`. |
| TE-11 | Die Webhook-Meldungen von Radarr und Sonarr tragen Kennungen und Dateipfad, Slouchcat liest heute nur Ereignis und Ordner (Abschnitt 10). | Verfügbarkeit weiter erst nach dem Match feststellen. Kennung und Pfad für einen schnellen Import zu nutzen, ist offen (F-33). |
| TE-12 | Eingang kennt nur `new_episode` mit Pflicht-`item_id`; Kanal-Ereignisse sind per CHECK fest. | Neubau von `notifications` und `deliveries` nach Vorlage; neue Arten und Ereignisse. |
| TE-13 | Gewöhnliche Konten haben ab Werk keine Inhaltsmeldung auf Kanälen. | Standard für das neue Ereignis bewusst setzen (F-14). |
| TE-14 | Externe Kanäle gehören dem Konto; jedes Gerät des Kontos sieht Titel aller Profile. | Regel für PIN-Profile ohne Titel (F-15). |
| TE-15 | Jellyseerr kennt keine Profile, keine fremden Kennungen, keine Mitwünsche. | Profil und Mitwunsch lokal in Slouchcat; nach aussen zeigt ein Profil-Tag in Radarr und Sonarr das Profil. |
| TE-16 | Kein Änderungs-Feed, kein Sammelabruf, teures `GET /request`. | Abgleich alle 5 min mit `sort=modified` und Abbruch; Zeiger rückt je Seite vor; Erstabgleich ohne Seitengrenze im Hintergrund. |
| TE-17 | Webhook von Jellyseerr: ein Ziel, keine Signatur, keine Benutzer-ID. | Nur Anstoss zum Abgleich, Stufe 4 (F-18). |
| TE-18 | `X-API-User` ist undokumentiert. | Verbindungstest prüft es mit `GET /auth/me` (F-3). |
| TE-19 | CSRF-Schutz in Jellyseerr verhindert Schreiben. | Verbindungstest meldet es. |
| TE-20 | `/api` ist im Service Worker NetworkOnly. | Wünsche nur online, kein Offline-Zustand. |
| TE-21 | Regel 2: Titel-SQL nur in erlaubten Paketen. | `internal/wishes` fragt `items` nicht ab; `visibility` und `metadata` liefern die Antworten. |
| TE-22 | Benutzer- und Request-IDs gelten nur in einer Jellyseerr-Instanz; eine neu aufgesetzte Instanz vergibt sie neu. | Fingerabdruck der Instanz; bei Abweichung halten Übermittlung und Abgleich an (`04-benutzer-profilmapping.md`, Abschnitt 8.4). |
| TE-23 | Die TMDB-Suche liefert keine Freigaben. | Für Profile unter 18 holt der Server je Treffer die Details (höchstens 20, zwischengespeichert in `external_titles`). |
| TE-24 | Der Server sieht das Passwort nur beim Annehmen der Einladung, bei der Anmeldung und beim Ändern (Abschnitt 7). | Passwort nur in diesen Momenten an Jellyseerr; sonst Zufallspasswort und «bei der nächsten Anmeldung übernehmen» (`04-benutzer-profilmapping.md`, Abschnitt 13). |
| TE-25 | Das Passwort von Benutzer 1 lässt sich nicht ohne das alte setzen (Abschnitt 17.4). | Benutzer 1 bleibt beim Passwortabgleich aussen vor. |
| TE-26 | `settings/main` schreibt alle Felder; Limit-Felder nur für Benutzer ohne `MANAGE_USERS` (Abschnitt 17.4). | Lesen vor Schreiben; Administratoren ohne Limit. |
| TE-27 | Jellyseerr zählt das Limit selbst, rollierend, Staffeln einzeln, auch direkte Wünsche (Abschnitt 17.5). | Slouchcat stellt das Limit ein und zeigt den Stand; zählen lässt es Jellyseerr. |
| TE-28 | Jellyseerr legt keine Tags an; `tags` im Request brauchen erweiterte Rechte und ersetzen die Standard-Tags (Abschnitt 17.8). | Profil-Tag direkt in Radarr und Sonarr; das braucht eine Ausnahme von ADR 0010 und Regel 11 (Anhang H 5). |
| TE-29 | Jellyseerr führt Titel über die TMDB-ID; die Staffelliste kann aus TMDB oder TVDB kommen (Abschnitt 17.7). | Staffeln im Dialog aus Jellyseerr; Empfehlung, Jellyseerr auf TVDB zu stellen (F-29). |
| TE-30 | Discover- und Suchrouten von Jellyseerr liefern TMDB-Listen mit dem Zustand in Jellyseerr, nicht in Slouchcat (Abschnitt 17.6). | Bereich «Wünsche» aus diesen Routen mit Zwischenspeicher; Abgleich mit der Bibliothek in Slouchcat. |
| TE-31 | Radarr und Sonarr melden von sich aus nur über einen Webhook; die API beantwortet nur Fragen (Abschnitt 10.3). | Webhooks bleiben wie heute (Marco, E-6); der Import kommt weiter über sie. Den Download-Fortschritt fragt Slouchcat im Takt ab: Warteschlange und Dateistand der gewünschten Titel (`queue`, `movie?tmdbId`, `series?tvdbId`), nur solange es Wünsche in Beschaffung gibt (Job `wishes.progress`, Anhang H N-81, N-84). |
| TE-32 | Den Beschaffungsstand zeigt Slouchcat heute nirgends; er ist Nicht-Ziel (Spec Zeile 67). | Ausnahme nur beim Wunsch, nicht am Titel der Bibliothek und nicht in der Suche (Marco, E-7; Anhang H N-82). |
