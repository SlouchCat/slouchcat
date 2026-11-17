# Konzept 25 · Bestandsanalyse: was es heute gibt

> **Kurz gesagt.** Slouchcat kennt heute nur Titel, die in der eigenen Bibliothek stehen. Das Spotlight
> holt zwar eine Trakt-Liste mit Titeln, die fehlen, zeigt diese aber nur in der Verwaltung. Es gibt kein
> Wunschrecht, keine Freigabe für fehlende Titel, keinen Haken «neuer Film ist da» und keinen Weg ins
> Heimnetz zu einem Dienst wie Jellyseerr. Jellyseerr selbst kann Wünsche im Namen eines Benutzers
> anlegen, kennt aber keine Profile, keinen Änderungs-Feed und keinen Ablehnungsgrund. Abschnitt 20
> fasst die Einschränkungen zusammen, die das Konzept bestimmen.

Stand: `main` `ba6efe31`, Vertrag 1.40.0 (`api/openapi.yaml:4`), letzte Migration 00065, gelesen am
16. November 2026. Pfade gelten ab der Wurzel des Repos. Jellyseerr-Belege beziehen sich auf den
Quellcode von Seerr v3.5.2 (Tag `v3.5.2`, Repo `seerr-team/seerr`), verglichen mit Jellyseerr v2.7.3.
Die ausführlichen Analyseberichte liegen im Unterordner `anhang/`: [Anhang A](anhang/A-spotlight-startseite.md)
(Spotlight und Startseite), [Anhang B](anhang/B-konten-benachrichtigungen.md) (Konten und Benachrichtigungen),
[Anhang C](anhang/C-integrationen-jobs-regeln.md) (Integrationen, Jobs, Regeln) und
[Anhang D](anhang/D-jellyseerr-api.md) (Jellyseerr-API). Dieses Papier fasst sie zusammen.

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

Was sich mit Frage 33 ändert, steht in `11-entscheide-und-adr.md`. Der Code verhindert das Vorhaben
nicht, die Papiere schon.

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
Abschnitt 8.

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
  Gesehen, Sammlungen und Benachrichtigungen.
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
  `tmdb_id` (`server/internal/metadata/tvdb.go:430-434`). Sie trifft das Spotlight nie und gälte als
  fehlend.
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

## 10. Webhooks von Radarr und Sonarr

- Nur eingehend: `POST /webhooks/{token}`, `public` (`server/internal/api/webhook_handlers.go:45-54`).
  Slouchcat ruft Radarr und Sonarr nie auf (`server/internal/integrations/integrations.go:5-6`).
- Wirkung: ein Pfad-Scan `scan.path` (`server/internal/integrations/receive.go:88-92`).
- **Ohne IDs:** Das Modell liest nur Ereignis, Film- und Serienpfad
  (`server/internal/model/integrations.go:45-51`). TMDB- oder TVDB-ID werden nicht gelesen.

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
| Im Namen eines Benutzers handeln | zusätzlich `X-API-User: <id>`; dann gelten dessen Rechte, Kontingent und Auto-Freigabe | `server/middleware/auth.ts` Z. 9–34; `server/entity/MediaRequest.ts` Z. 107–167, 340–353 |
| Wunsch anlegen | `POST /request` mit `mediaType`, `mediaId` (TMDB-ID), bei Serien `seasons` und `tvdbId` | `server/interfaces/api/requestInterfaces.ts` Z. 16–30 |
| Zustand lesen | `GET /request` (Seiten, `sort=modified`), `GET /request/{id}`, `GET /movie\|tv/{tmdbId}` mit `mediaInfo.requests` | `server/routes/request.ts` Z. 38–311, 438–468; `server/entity/Media.ts` Z. 98–115 |
| Freigeben, ablehnen | `POST /request/{id}/approve\|decline`, nur bei `PENDING` | `server/routes/request.ts` Z. 794–848 |
| Zurückziehen | `DELETE /request/{id}`, eigener Wunsch nur bei `PENDING` | `server/routes/request.ts` Z. 719–752 |
| Erneut senden | `POST /request/{id}/retry`, nur bei `FAILED` | `server/routes/request.ts` Z. 754–792 |
| Benutzer lesen | `GET /user` mit Suche, Rechten (Bitmaske), Kontingent | `server/routes/user/index.ts` Z. 36–166; `server/lib/permissions.ts` Z. 1–32 |
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

## 18. Anknüpfungspunkte

- `spotlight_entries` hält schon die Kandidaten «in Trakt, nicht in der Bibliothek».
- `spotlight_overrides` zeigt das Muster einer Tabelle ohne FK auf `items`, die Abrufe überlebt.
- `internal/trakt` ist ein reiner HTTP-Client ohne DB mit Rate-Limit: Vorbild für `internal/seerr`.
- Der TMDB-Client holt schon Details, `external_ids`, Freigaben und `seasons[].episode_count`
  (`server/internal/metadata/tmdb.go`).
- `canRecommend` zeigt das Muster «Server liefert `can…`, Client zeigt nur».
- `notify.Emit` mit Profilpräfix und Kanälen ist allgemein (`server/internal/notify/dispatch.go:74-121`).
- Die Stelle des Hakens `WithRelink` nach dem Match ist der natürliche Auslöser für «Titel zugeordnet».

## 19. Was dieses Papier nicht prüft

Laufzeitverhalten (keine Tests, keine Instanz von Jellyseerr angesprochen), die Zahl der Serien ohne
`tmdb_id` auf CT 190, die Sprache der Trakt-Titel und ob jede JSON-Antwort `Cache-Control: no-store` trägt.

## 20. Technische Einschränkungen, die das Konzept bestimmen

| Nr. | Einschränkung | Folge im Konzept |
|---|---|---|
| E-1 | Wünschen ist Nicht-Ziel (Abschnitt 1). | Frage 33, ADR 0037 und 0038 vor jedem Bau (Stufe 0). |
| E-2 | `ItemRef` und `visible_items` gelten nur für Bibliothekstitel. | Eigenes Schema `ExternalTitleRef`, eigene Sichtbarkeitsregel in `visibility` für fehlende Titel. |
| E-3 | Fehlende Titel haben keine Freigabe. | Freigabe aus TMDB nach ADR 0034, unbekannt = 18; nicht sichtbar heisst 404. |
| E-4 | Spotlight-Einträge tragen nur TMDB-ID, Titel, Jahr; die Tabelle wird bei jedem Lauf ersetzt und bei Fehlern geleert. | Eigene Tabelle `external_titles`; Wunschzustand nie an `spotlight_entries` hängen. |
| E-5 | Abgleich nur über TMDB-ID; TVDB-zuerst-Serien haben keine `tmdb_id`. | Trakt-Einträge speichern TVDB- und IMDb-ID; Abgleich über alle drei. |
| E-6 | Die Bildstrecke der App setzt eine `items.id` voraus; der Browser lädt nie direkt von TMDB. | Eigener Bildweg `/titles/{media}/{tmdbId}/images/…` über den Bildspeicher. |
| E-7 | Die Datenbank kennt keine Gesamtzahl der Staffeln. | `series_catalog` für Serien in der Bibliothek. |
| E-8 | Kein Wunschrecht, keine Einzelschalter. | Neue Spalten `can_request` je Konto und Profil; Server liefert `canRequest` mit Grund. |
| E-9 | `netguard` sperrt das Heimnetz. | Eigener Client `internal/seerr` mit ausdrücklicher LAN-Erlaubnis, ohne Weiterleitungen. |
| E-10 | Kein Haken für neue Filme und Serien; Freigabe kommt erst nach dem Match. | Haken nach dem Match plus stündlicher Nachlauf `wishes.reconcile`. |
| E-11 | Radarr/Sonarr-Webhooks tragen keine IDs. | Verfügbarkeit nicht aus dem Webhook ableiten. |
| E-12 | Eingang kennt nur `new_episode` mit Pflicht-`item_id`; Kanal-Ereignisse sind per CHECK fest. | Neubau von `notifications` und `deliveries` nach Vorlage; neue Arten und Ereignisse. |
| E-13 | Gewöhnliche Konten haben ab Werk keine Inhaltsmeldung auf Kanälen. | Standard für das neue Ereignis bewusst setzen (F-14). |
| E-14 | Externe Kanäle gehören dem Konto; jedes Gerät des Kontos sieht Titel aller Profile. | Regel für PIN-Profile ohne Titel (F-15). |
| E-15 | Jellyseerr kennt keine Profile, keine fremden Kennungen, keine Mitwünsche. | Zuordnung und Mitwunsch lokal in Slouchcat. |
| E-16 | Kein Änderungs-Feed, kein Sammelabruf, teures `GET /request`. | Abgleich alle 5 min mit `sort=modified` und Abbruch; Zeiger rückt je Seite vor; Erstabgleich ohne Seitengrenze im Hintergrund. |
| E-17 | Webhook von Jellyseerr: ein Ziel, keine Signatur, keine Benutzer-ID. | Nur Anstoss zum Abgleich, Stufe 4 (F-18). |
| E-18 | `X-API-User` ist undokumentiert. | Verbindungstest prüft es mit `GET /auth/me` (F-3). |
| E-19 | CSRF-Schutz in Jellyseerr verhindert Schreiben. | Verbindungstest meldet es. |
| E-20 | `/api` ist im Service Worker NetworkOnly. | Wünsche nur online, kein Offline-Zustand. |
| E-21 | Regel 2: Titel-SQL nur in erlaubten Paketen. | `internal/wishes` fragt `items` nicht ab; `visibility` und `metadata` liefern die Antworten. |
| E-22 | Benutzer- und Request-IDs gelten nur in einer Jellyseerr-Instanz; eine neu aufgesetzte Instanz vergibt sie neu. | Fingerabdruck der Instanz; bei Abweichung halten Übermittlung und Abgleich an (`04-benutzer-profilmapping.md`, Abschnitt 8.4). |
| E-23 | Die TMDB-Suche liefert keine Freigaben. | Für Profile unter 18 holt der Server je Treffer die Details (höchstens 20, zwischengespeichert in `external_titles`). |
