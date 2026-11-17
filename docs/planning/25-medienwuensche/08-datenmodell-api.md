# Konzept 25 · Datenmodell und API

> **Kurz gesagt.** Neue Tabellen halten die Jellyseerr-Benutzer, die Zuordnung, die Wünsche mit Staffeln,
> Wünschern und Verlauf sowie die Daten fehlender Titel (`external_titles`) und den Staffelkatalog vorhandener
> Serien (`series_catalog`). Bestehende Tabellen mit CHECK-Bedingungen (`notifications`, `deliveries`,
> `accounts`, `profiles`, `home_rows`, `webhook_integrations`) baut der Bau nach der Vorlage neu, nur vorwärts.
> Der Vertrag bekommt Zuschauer-Operationen (`scoped`), eine Konto-Übersicht (`scoped` mit Hauptprofil) und
> Verwaltungs-Operationen (`admin`, ohne `ItemRef`); fehlende Titel erscheinen als eigenes Schema
> `ExternalTitleRef` neben `ItemRef`.
> Stufe 1 bleibt im Vertrag nicht brechend; Stufen 2, 3 und 4 erweitern Enums und sind nach der Praxis des
> Projekts brechend.

Grundlage: [Anhang F](anhang/F-entscheid-blatt.md) Abschnitte 5 und 6. Architektur und Begründungen: `03-architektur.md`. Bauschritte:
`09-umsetzungsplan.md`. Was über Anhang F hinausgeht, ist als **Vorschlag** gekennzeichnet. Typen nach SQLite
`STRICT`: Zeitstempel `INTEGER` wie die übrigen Tabellen, Wahrheitswerte `INTEGER` 0/1, `items.id` ist `TEXT`,
`accounts.id` und `profiles.id` sind `INTEGER` (`server/migrations/00015_discover.sql:329-341`,
`00063_minimum_age_accounts.sql:27-45`).

## 1. Übersicht

### 1.1 Neue und geänderte Tabellen

| Tabelle | neu/geändert | Stufe | Besitzer (schreibt) |
|---|---|---|---|
| `seerr_users` | neu | 1 | `wishes` |
| `wish_account_map` | neu | 1 | `wishes` |
| `wish_profile_map` | neu | 1 | `wishes` |
| `accounts` (+ `can_request`) | Neubau | 1 | `admin`, `auth` |
| `profiles` (+ `can_request`) | Neubau | 1 | `auth` |
| `invitations` (+ `can_request`) | Spalte | 1 | `auth` |
| `settings` (`seerr.*`, `wishes.*`) | Schlüssel | 1 | `admin` |
| `seerr_log` | neu | 1 | `wishes` |
| `wishes` | neu | 2 | `wishes` |
| `wish_seasons` | neu | 2 | `wishes` |
| `wish_requesters` | neu | 2 | `wishes` |
| `wish_events` | neu | 2 | `wishes` |
| `external_titles`, `external_title_texts` | neu | 2 | `metadata` |
| `external_title_images` | neu | 2 | `metadata` |
| `notifications` | Neubau | 2 | `notify` |
| `deliveries` | Neubau | 2 | `notify` |
| `account_channels` | Daten (Ereignisse rückfüllen) | 2 | Migration, dann `notify` |
| `spotlight_entries` (+ `tvdb_id`, `imdb_id`) | Spalten | 3 | `metadata` |
| `spotlight_config` (+ `show_missing`, `max_missing`, `missing_order`) | Spalten | 3 | `admin` |
| `home_rows` (CHECK + `wishes`) | Neubau | 3 | `admin` |
| `series_catalog` | neu | 4 | `metadata` |
| `webhook_integrations` (CHECK + `jellyseerr`) | Neubau | 4 | `integrations` |

### 1.2 Diagramm: Beziehungen

```mermaid
erDiagram
  accounts ||--o{ profiles : "hat"
  accounts ||--o| wish_account_map : "zugeordnet"
  profiles ||--o| wish_profile_map : "eigener Benutzer"
  seerr_users ||--o{ wish_account_map : "gehört zu"
  seerr_users ||--o{ wish_profile_map : "gehört zu"
  seerr_users ||--o{ wishes : "fordert an"
  wishes ||--o{ wish_seasons : "Staffeln"
  wishes ||--o{ wish_requesters : "Wünscher"
  wishes ||--o{ wish_events : "Verlauf"
  profiles ||--o{ wish_requesters : "wünscht"
  accounts ||--o{ wish_requesters : "Konto des Profils"
  items |o--o{ wishes : "erfüllt durch"
  external_titles ||--o{ external_title_texts : "Texte je Sprache"
  external_titles ||--o{ external_title_images : "Bilder"
  items ||--o| series_catalog : "Staffelkatalog"
  profiles ||--o{ notifications : "Eingang"
  wishes |o--o{ notifications : "betrifft"
  items |o--o{ notifications : "verweist auf"

  wishes {
    int id PK
    text media
    int tmdb_id
    int tvdb_id
    text state
    int seerr_request_id
    int seerr_user_id FK
    text item_id FK
  }
  wish_seasons {
    int wish_id PK
    int season_number PK
    int tmdb_id
    int seerr_request_id
    text state
  }
  wish_requesters {
    int wish_id PK
    int profile_id PK
    int account_id FK
    text role
  }
  seerr_users {
    int seerr_id PK
    text display_name
    int permissions
    int created_at
    int gone_at
  }
  external_titles {
    text media PK
    int tmdb_id PK
    int tvdb_id
    int min_age
  }
  series_catalog {
    text item_id PK
    int tmdb_id
    text status
  }
```

`external_titles` hat bewusst keinen Fremdschlüssel zu `wishes` oder `items`: Ein fehlender Titel existiert
unabhängig davon, ob ihn jemand wünscht (Muster `spotlight_overrides`, [Anhang A](anhang/A-spotlight-startseite.md) 1.2).

## 2. Tabellen im Einzelnen

### 2.1 `seerr_users` (Stufe 1)

Zweck: Abbild der Jellyseerr-Benutzer für Zuordnung, Vorschläge und Hinweise im Dialog. Quelle `GET /user`,
`GET /user/{id}/quota` ([Anhang D](anhang/D-jellyseerr-api.md) 7).

| Spalte | Typ | Bedingung |
|---|---|---|
| `seerr_id` | INTEGER | PK, die ID in Jellyseerr |
| `display_name` | TEXT | NOT NULL, `length` 1–200 |
| `email` | TEXT | NULL erlaubt, COLLATE NOCASE |
| `user_type` | INTEGER | NOT NULL, CHECK IN (1, 2, 3, 4) (Plex, lokal, Jellyfin, Emby) |
| `permissions` | INTEGER | NOT NULL, CHECK ≥ 0 (Bitmaske, Anhang D 7.3) |
| `movie_quota_limit`, `movie_quota_days` | INTEGER | NULL = kein Kontingent |
| `tv_quota_limit`, `tv_quota_days` | INTEGER | NULL = kein Kontingent |
| `movie_quota_used`, `tv_quota_used` | INTEGER | **Vorschlag**, aus `/quota`, NULL = unbekannt |
| `created_at` | INTEGER | NULL; `createdAt` des Benutzers in Jellyseerr, Teil des Fingerabdrucks der Instanz (`03-architektur.md`, Abschnitt 7.4) |
| `fetched_at` | INTEGER | NOT NULL |
| `gone_at` | INTEGER | NULL; gesetzt, wenn der Benutzer in Jellyseerr fehlt |

Index `seerr_users_email (email)` für Vorschläge. Zeilen werden nie gelöscht, solange eine Zuordnung oder ein
Wunsch auf sie zeigt; sonst nach 30 Tagen mit `gone_at` (**Vorschlag**).

Abgeleitete Fakten für die Oberfläche (im Server, nicht im Client): «darf Filme anfordern» = `ADMIN` (2) oder
`REQUEST` (32) oder `REQUEST_MOVIE` (262144); «darf Serien» entsprechend mit `REQUEST_TV` (524288);
«Auto-Freigabe» = `ADMIN`, `MANAGE_REQUESTS` (16), `AUTO_APPROVE` (128) bzw. `AUTO_APPROVE_MOVIE` (256) /
`AUTO_APPROVE_TV` (512) (Anhang D 3.3, 7.3).

### 2.2 `wish_account_map` und `wish_profile_map` (Stufe 1)

| Tabelle | Spalte | Typ | Bedingung |
|---|---|---|---|
| `wish_account_map` | `account_id` | INTEGER | PK, → `accounts(id)` ON DELETE CASCADE |
| | `seerr_user_id` | INTEGER | NOT NULL, UNIQUE, → `seerr_users(seerr_id)` |
| | `set_at` | INTEGER | NOT NULL |
| | `set_by` | INTEGER | → `accounts(id)` ON DELETE SET NULL |
| `wish_profile_map` | `profile_id` | INTEGER | PK, → `profiles(id)` ON DELETE CASCADE |
| | `seerr_user_id` | INTEGER | NOT NULL, → `seerr_users(seerr_id)` |
| | `set_at` | INTEGER | NOT NULL |

Regel «Ein Jellyseerr-Benutzer gehört höchstens zu einem Konto, Profile desselben Kontos dürfen ihn teilen»
(Anhang F 2.4): `UNIQUE` deckt nur die Kontozuordnung. Den Rest prüft der Dienst in derselben Transaktion und
antwortet 409 `seerr_user_taken` (`04-benutzer-profilmapping.md`, Abschnitt 5). **Vorschlag:** zusätzlich ein
Trigger `BEFORE INSERT/UPDATE` auf beiden Tabellen als zweite Sicherung. Fehlt eine Zeile in
`wish_profile_map`, gilt das Profil als «Wie Konto». Die Spalte `wishes.seerr_user_id` friert den Benutzer je
Wunsch ein; spätere Zuordnungsänderungen hängen nichts um (`04-benutzer-profilmapping.md`, Abschnitt 8.1).

### 2.3 `accounts.can_request`, `profiles.can_request`, `invitations.can_request` (Stufe 1)

| Tabelle | Spalte | Bedingung | Rückfüllung |
|---|---|---|---|
| `accounts` | `can_request INTEGER NOT NULL DEFAULT 0 CHECK (can_request IN (0, 1))` | Deckel, setzt nur der Administrator | Besitzerkonto 1, alle anderen 0 |
| `profiles` | `can_request INTEGER NOT NULL DEFAULT 0 CHECK (can_request IN (0, 1))` | setzt das Hauptprofil | Hauptprofile 1, übrige 0 |
| `invitations` | `can_request INTEGER NOT NULL DEFAULT 0 CHECK (can_request IN (0, 1))` | geht beim Annehmen an das neue Konto | 0 |

Wirksam ist `accounts.can_request = 1 AND profiles.can_request = 1` (Anhang F 2.6). Neue Konten bekommen den Wert
aus `wishes.new_accounts_can_request`, neue Hauptprofile 1, weitere Profile 0.

Anhang F sieht für `accounts` und `profiles` einen **Tabellenneubau** vor. Der Neubau folgt dem Muster von
00063 und 00061: Die View `visible_items` und die Trigger auf diesen Tabellen werden in derselben Migration
neu angelegt (`00063_minimum_age_accounts.sql:86-112`). **Vorschlag:** Der Bau prüft zuerst, ob `ALTER TABLE …
ADD COLUMN` mit konstantem `DEFAULT` und `CHECK` genügt (SQLite erlaubt das); dann entfällt der Neubau samt View.
Für `invitations` reicht `ADD COLUMN` in jedem Fall.

### 2.4 `wishes` (Stufe 2)

Zweck: ein Wunsch = ein Film oder ein Satz Staffeln einer Serie. Ein Film hat genau einen Request in
Jellyseerr; ein Serienwunsch kann auf mehrere Requests verteilt sein (je Staffel in `wish_seasons`, Abschnitt
2.5).

| Spalte | Typ | Bedingung |
|---|---|---|
| `id` | INTEGER | PK |
| `media` | TEXT | NOT NULL, CHECK IN ('movie', 'show') |
| `tmdb_id` | INTEGER | NOT NULL, > 0 |
| `tvdb_id` | INTEGER | NULL; bei `show` vor dem Senden Pflicht (Dienst) |
| `imdb_id` | TEXT | NULL |
| `title` | TEXT | NOT NULL (Titel zum Zeitpunkt des Wunsches, Sprache des Besitzers) |
| `year` | INTEGER | NULL |
| `origin` | TEXT | NOT NULL, CHECK IN ('slouchcat', 'jellyseerr') |
| `seerr_request_id` | INTEGER | NULL; nur bei Filmen (CHECK `media = 'movie' OR seerr_request_id IS NULL`); Serien führen die Request-ID je Staffel |
| `seerr_media_id` | INTEGER | NULL (interne Media-ID von Jellyseerr, nicht TMDB, Anhang D 11.3) |
| `seerr_user_id` | INTEGER | NULL, → `seerr_users(seerr_id)` ON DELETE SET NULL |
| `state` | TEXT | NOT NULL, CHECK IN ('queued', 'pending', 'processing', 'partial', 'available', 'declined', 'failed', 'withdrawn'); CHECK `state <> 'partial' OR media = 'show'` |
| `state_detail` | TEXT | NULL; **Vorschlag** Werte `importing` («wird eingelesen»), `not_found_in_library` (Diagnose) |
| `fail_reason` | TEXT | NULL; CHECK IN ('quota', 'blocklisted', 'no_permission', 'arr_failed', 'unreachable_24h', 'seerr_request_gone', 'seerr_media_deleted', 'seerr_user_gone', 'tvdb_id_missing', 'account_deleted') (**Vorschlag** Liste); CHECK `(state = 'failed') = (fail_reason IS NOT NULL)` |
| `attempts` | INTEGER | NOT NULL DEFAULT 0, ≥ 0 |
| `next_attempt_at` | INTEGER | NULL; bei `queued` und bei gesetztem `withdraw_pending` |
| `withdraw_pending` | INTEGER | NOT NULL DEFAULT 0, 0/1; «in Jellyseerr noch zu löschen» nach dem Zurückziehen (`03-architektur.md`, Abschnitt 6.6) |
| `checked_at` | INTEGER | NULL; letzte Einzelprüfung `GET /request/{id}`, höchstens einmal je 24 h (`03-architektur.md`, Abschnitt 7.1) |
| `item_id` | TEXT | NULL, → `items(id)` ON DELETE SET NULL |
| `created_at`, `updated_at` | INTEGER | NOT NULL |
| `submitted_at`, `approved_at`, `available_at`, `closed_at` | INTEGER | NULL |
| `seerr_updated_at` | INTEGER | NULL (`updatedAt` des Requests) |

Schlüssel und Indizes:

| Name | Definition | Zweck |
|---|---|---|
| `wishes_open_movie` | UNIQUE (`tmdb_id`) WHERE `media = 'movie' AND state IN ('queued', 'pending', 'processing')` | höchstens ein offener Filmwunsch (Anhang F 2.11) |
| `wishes_request` | UNIQUE (`seerr_request_id`) WHERE `seerr_request_id IS NOT NULL` | ein Film-Request gehört höchstens einem Wunsch; geschlossene Wünsche eines gelöschten Kontos geben ihre ID an den Verlauf ab |
| `wishes_withdraw` | (`next_attempt_at`) WHERE `withdraw_pending = 1` | offene Löschaufträge |
| `wishes_open` | (`media`, `tmdb_id`) WHERE `state IN ('queued', 'pending', 'processing', 'partial')` | Wunschzustand am Titel, Haken `WithMatched` |
| `wishes_open_tvdb` | (`tvdb_id`) WHERE `media = 'show' AND state IN (…offen…)` | Abgleich TVDB-zuerst-Serien |
| `wishes_due` | (`next_attempt_at`) WHERE `state = 'queued'` | Ausgangsfach |
| `wishes_admin` | (`state`, `updated_at` DESC) | Verwaltung «Anfragen» |

`item_id` schreibt `wishes` als Wert, den `visibility` geliefert hat. Das Paket liest `items` nicht (Regel 2).

### 2.5 `wish_seasons` (Stufe 2)

| Spalte | Typ | Bedingung |
|---|---|---|
| `wish_id` | INTEGER | NOT NULL, → `wishes(id)` ON DELETE CASCADE |
| `season_number` | INTEGER | NOT NULL, ≥ 1 (Staffel 0 nie, Anhang F 2.15) |
| `tmdb_id` | INTEGER | NOT NULL (Kopie aus `wishes` für die Eindeutigkeit) |
| `seerr_request_id` | INTEGER | NULL; der Request in Jellyseerr, der diese Staffel trägt. **Nicht** eindeutig über die Tabelle, weil ein Request mehrere Staffeln trägt |
| `seerr_updated_at` | INTEGER | NULL (`updatedAt` dieses Requests) |
| `state` | TEXT | NOT NULL, CHECK IN ('queued', 'pending', 'processing', 'available', 'declined', 'failed', 'withdrawn') |
| `available_at` | INTEGER | NULL |
| `notified_at` | INTEGER | NULL; wann die Staffelmeldung erfolgt ist (Doppelschutz, `06-benachrichtigungen.md`, Abschnitt 7) |

PK (`wish_id`, `season_number`). Teilindex `wish_seasons_open` UNIQUE (`tmdb_id`, `season_number`) WHERE
`state IN ('queued', 'pending', 'processing')`. Index `wish_seasons_request` (`seerr_request_id`) für den Abgleich.

Die Kopie `tmdb_id` schliesst eine Lücke in Anhang F (als Redaktionsentscheid vom 17. November 2026 gesetzt): Anhang F
verlangt «eine offene Zeile je (`tmdb_id`, `season_number`)», aber ein Index in SQLite kann nicht über zwei
Tabellen gehen. Ein Trigger `BEFORE INSERT` setzt sie aus `wishes` und verweigert Abweichungen (**Vorschlag**).

**Staffelstatus.** Ob eine Staffel `available` ist, rechnet `wishes` aus der eigenen Bibliothek, nicht aus
Jellyseerr: Die Zahl der abspielbaren, für das Profil sichtbaren Folgen (`visibility.VisibleSeasons`) erreicht
die Folgenzahl laut `series_catalog` bzw. vor Stufe 4 laut `external_titles.seasons`; bei laufenden Staffeln die
bisher ausgestrahlten Folgen (`06-benachrichtigungen.md`, Abschnitt 3.3). Die Zustände `pending` und
`processing` je Staffel kommen aus dem Request dieser Staffel; `media.seasons[].status` aus dem Abgleich liefert
nur den Zusatz «wird eingelesen».

### 2.6 `wish_requesters` (Stufe 2)

| Spalte | Typ | Bedingung |
|---|---|---|
| `wish_id` | INTEGER | NOT NULL, → `wishes(id)` ON DELETE CASCADE |
| `profile_id` | INTEGER | NOT NULL, → `profiles(id)` ON DELETE CASCADE |
| `account_id` | INTEGER | NOT NULL, → `accounts(id)` ON DELETE CASCADE |
| `role` | TEXT | NOT NULL, CHECK IN ('owner', 'joined') |
| `created_at` | INTEGER | NOT NULL |
| `notified_at` | INTEGER | NULL; wann «verfügbar» für dieses Profil gemeldet wurde. Massgeblich für den Doppelschutz (`06-benachrichtigungen.md`, Abschnitt 7) |
| `closed_at` | INTEGER | NULL (**Vorschlag**, `03-architektur.md` 8.3) |

PK (`wish_id`, `profile_id`). Teilindex `wish_requesters_owner` UNIQUE (`wish_id`) WHERE `role = 'owner'`.
Index `wish_requesters_profile` (`profile_id`, `created_at` DESC) für «Meine Wünsche».
Wird ein Profil gelöscht, gehen seine Wünsche an das Hauptprofil des Kontos; das Löschen in `auth` ruft dafür
vorher `wishes` auf, weil CASCADE die Zeile sonst entfernt. Wird ein Konto gelöscht, gehen Wünsche mit
Mitwünschern anderer Konten an den ältesten dieser Mitwünscher; die übrigen schliesst Slouchcat: offene werden
`failed` mit `fail_reason = account_deleted`, alle bekommen `wishes.closed_at`, und die Request-IDs wandern in
`wish_events`. In Jellyseerr bleiben die Requests und erscheinen ab Stufe 4 als «nicht zuordenbar» (beides
Redaktionsentscheid vom 17. November 2026, `04-benutzer-profilmapping.md`, Abschnitt 10).

### 2.7 `wish_events` (Stufe 2)

| Spalte | Typ | Bedingung |
|---|---|---|
| `id` | INTEGER | PK |
| `wish_id` | INTEGER | NOT NULL, → `wishes(id)` ON DELETE CASCADE |
| `at` | INTEGER | NOT NULL |
| `kind` | TEXT | NOT NULL, `^[a-z_]+$`, ohne Enum (Muster Aktivitätsprotokoll, `docs/adr/0019-verwaltungsleiste-aktivitaet-neustart.md:49-58`) |
| `detail` | TEXT | NULL, JSON (z. B. `{"from":"pending","to":"processing"}`), ohne Geheimnisse |

Index (`wish_id`, `at`). Arten (**Vorschlag**): `created`, `joined`, `submitted`, `submit_failed`, `attached`,
`state_changed`, `season_available`, `withdrawn`, `withdraw_deleted`, `reopened`, `retried`, `imported`,
`owner_changed`, `closed`, `approved_by_admin`, `declined_by_admin`.
Aufräumen (**Vorschlag**): abgeschlossene Wünsche samt Verlauf nach 365 Tagen.

### 2.8 `seerr_log` (Stufe 1)

Zweck: Band «Protokoll» (Anhang F 4: «letzte 20 Aufrufe und Fehler»), auch nach einem Neustart. Anhang F nennt
dafür keine Tabelle; sie ist ein Redaktionsentscheid vom 17. November 2026.
Spalten `id` PK, `at`, `method`, `path` (ohne Query), `status` (NULL bei Netzfehler), `duration_ms`, `error`
(ohne Host-Pfad und Schlüssel). Ein Trigger hält 20 Zeilen wie `webhook_events` (`server/migrations/00017_integrations.sql:47-57`).

### 2.9 `external_titles`, `external_title_texts`, `external_title_images` (Stufe 2)

`external_titles`:

| Spalte | Typ | Bedingung |
|---|---|---|
| `media` | TEXT | NOT NULL, CHECK IN ('movie', 'show') |
| `tmdb_id` | INTEGER | NOT NULL |
| `tvdb_id`, `imdb_id` | INTEGER, TEXT | NULL |
| `original_title` | TEXT | NULL |
| `year` | INTEGER | NULL |
| `release_date` | TEXT | NULL, ISO-Datum |
| `runtime_minutes` | INTEGER | NULL |
| `genres` | TEXT | NOT NULL DEFAULT '[]', JSON-Array |
| `certifications` | TEXT | NOT NULL DEFAULT '{}', JSON Land → Freigabe roh |
| `min_age` | INTEGER | NULL = unbekannt (zählt als 18); 0–18; `metadata` rechnet es mit `ageFromCertifications` (`server/internal/metadata/ratings.go:113-125`), derselben Funktion wie für Bibliothekstitel: Länderreihenfolge nach ADR 0034, das erste Land mit bekanntem Wert gilt |
| `poster_path`, `backdrop_path`, `logo_path` | TEXT | NULL, TMDB-Pfade (nie an den Browser) |
| `trailers` | TEXT | NOT NULL DEFAULT '[]', JSON (YouTube-Schlüssel, Sprache) |
| `status` | TEXT | NULL (TMDB, z. B. `Returning Series`) |
| `seasons` | TEXT | NOT NULL DEFAULT '[]', JSON `[{number, episodeCount, airDate}]`, nur `show` |
| `next_episode` | TEXT | NULL, JSON `{season, episode, airDate}` |
| `last_episode` | TEXT | NULL, JSON `{season, episode, airDate}` der zuletzt ausgestrahlten Folge (**Vorschlag**, für «bisher erschienene Folgen», `06-benachrichtigungen.md`, Abschnitt 3.3) |
| `fetched_at` | INTEGER | NOT NULL |

PK (`media`, `tmdb_id`). Indizes `external_titles_tvdb (tvdb_id)`, `external_titles_imdb (imdb_id)`.
`min_age` rechnet `metadata` beim Abruf und neu, wenn sich das Freigabeland ändert (**Vorschlag**: im selben
Job wie «Freigaben neu lesen»).

`external_title_texts`: PK (`media`, `tmdb_id`, `language`), `title` NOT NULL, `overview`, `tagline`;
Fremdschlüssel (`media`, `tmdb_id`) → `external_titles` ON DELETE CASCADE.

`external_title_images` (Lücke in Anhang F, Redaktionsentscheid; Muster `images` und 00040): PK `id`, (`media`, `tmdb_id`) →
`external_titles` CASCADE, `kind` CHECK IN ('poster', 'backdrop', 'logo'), `size`, `format`, `lang`,
`source_url`, `rel_path`, `content_hash`, `media_type`, `width`, `height`, `bytes`, `fetched_at`; UNIQUE
(`media`, `tmdb_id`, `kind`, `size`, `format`).

Aufräumen (**Vorschlag**): Zeilen, die weder in `spotlight_entries` stehen noch einen offenen oder in den
letzten 30 Tagen geschlossenen Wunsch haben und 30 Tage nicht abgerufen wurden, löscht `titles.refresh`.

### 2.10 `series_catalog` (Stufe 4)

| Spalte | Typ | Bedingung |
|---|---|---|
| `item_id` | TEXT | PK, → `items(id)` ON DELETE CASCADE (nur `kind = 'series'`, Dienst) |
| `tmdb_id` | INTEGER | NOT NULL |
| `status` | TEXT | NULL |
| `in_production` | INTEGER | NOT NULL DEFAULT 0, 0/1 |
| `seasons` | TEXT | NOT NULL DEFAULT '[]', JSON `[{number, episodeCount, airDate}]` |
| `next_episode` | TEXT | NULL, JSON |
| `last_episode` | TEXT | NULL, JSON wie bei `external_titles` (**Vorschlag**) |
| `fetched_at` | INTEGER | NOT NULL |

Schreibt `metadata` (`series.catalog`), liest `visibility`. Serien ohne `tmdb_id` bekommen keinen Eintrag
(**Vorschlag:** die Serienseite zeigt dann keine fehlenden Staffeln).

### 2.11 Erweiterungen bestehender Tabellen

| Tabelle | Änderung | Weg | Stufe |
|---|---|---|---|
| `notifications` | `kind` CHECK IN ('new_episode', 'wish_available', 'wish_season_available', 'wish_approved', 'wish_declined', 'wish_failed'); `item_id` NULL erlaubt; neu `wish_id` → `wishes(id)` ON DELETE CASCADE; neu `dedupe_key` TEXT NOT NULL; UNIQUE (`profile_id`, `dedupe_key`) statt UNIQUE (`profile_id`, `kind`, `item_id`); CHECK `kind = 'new_episode' OR wish_id IS NOT NULL`; CHECK `item_id IS NOT NULL OR wish_id IS NOT NULL` | Neubau; bestehende Zeilen bekommen `dedupe_key = 'episode:' || item_id` (wie `06-benachrichtigungen.md`, Abschnitt 11); Indizes `notifications_inbox`, `notifications_unread`, `notifications_item`, `notifications_parent` neu, dazu `notifications_wish` (`wish_id`). Neuer Leseweg: Einträge ohne `item_id` sind sichtbar, wenn das Profil in `wish_requesters` dieses Wunsches steht; Liste, Zähler und «gelesen» zählen beide Arten (`06-benachrichtigungen.md`, Abschnitt 8) | 2 |
| `deliveries` | `event` CHECK + 'wish_available', 'wish_status', 'wish_review' | Neubau wie 00018 | 2 |
| `account_channels` | `events` JSON: Vorgaben nach F-14 (`wish_available` an ab Stufe 2; `wish_review` an für Administratoren ab Stufe 4, wenn es einen Erzeuger hat) | Datenmigration je Stufe für **bestehende** Konten, `ChannelEnsureDefaults` für neue; Einzelheiten `06-benachrichtigungen.md`, Abschnitt 6 | 2, 4 |
| `spotlight_entries` | `tvdb_id INTEGER`, `imdb_id TEXT`; Index (`media`, `tvdb_id`) | `ADD COLUMN` (Tabelle wird bei jedem Abruf ersetzt, `metadata/trakt.go:278-313`) | 3 |
| `spotlight_config` | `show_missing INTEGER NOT NULL DEFAULT 0 CHECK IN (0,1)`, `max_missing INTEGER NOT NULL DEFAULT 5 CHECK BETWEEN 0 AND 10`, `missing_order TEXT NOT NULL DEFAULT 'trakt' CHECK IN ('trakt','after')` | `ADD COLUMN` | 3 |
| `home_rows` | `key` CHECK + 'wishes'; neue Zeile `wishes` | Neubau wie 00035/00039; neue Zeile direkt nach «Weiterschauen» (wie im Mockup); **Vorschlag:** ausgeblendet, wenn schon 15 Reihen sichtbar sind (wie `07-administration.md`, Abschnitt 10) | 3 |
| `webhook_integrations` | `kind` CHECK + 'jellyseerr' | Neubau; `webhook_events` bleibt | 4 |

**Vorschlag** (Abweichung von der Reihenfolge der Stufen, aber nicht von Anhang F): Der CHECK von `notifications` und
`deliveries` nimmt schon in Stufe 2 alle Arten auf, auch die erst in Stufe 4 erzeugten. So braucht es nur einen
Neubau. Der Vertrag nimmt die Werte erst mit ihrem Erzeuger auf (ADR 0009: kein Ereignis ohne Erzeuger).

### 2.12 Einstellungen (`settings`)

| Schlüssel | Wert | Vorgabe | Ort in der Verwaltung |
|---|---|---|---|
| `seerr.enabled` | 0/1 | 0 | Integrationen › Jellyseerr, «Jellyseerr verwenden» |
| `seerr.url` | Adresse | leer | «Adresse» |
| `seerr.api_key` | verschlüsselt (`v1:`) | leer | «API-Schlüssel» |
| `seerr.version` | Text | leer | Anzeige im Test |
| `seerr.last_sync` | Zeitstempel | leer | Band «Abgleich» (Zeiger des Abgleichs; leer heisst: Erstabgleich nötig) |
| `seerr.sync_pass` | JSON `{oben, bis, erreicht, seite}` | leer | Stand des laufenden Durchgangs, nach jeder Seite gespeichert (`03-architektur.md`, Abschnitt 7.2) |
| `seerr.instance` | JSON `{adminEmail, adminCreatedAt}` | leer | Fingerabdruck der Instanz (**Vorschlag**, `03-architektur.md`, Abschnitt 7.4) |
| `seerr.last_success`, `seerr.last_attempt`, `seerr.last_error` | Zeitstempel, Text | leer | Zustand (Muster `trakt_last_*`, `metadata/trakt.go:41-44`) |
| `wishes.enabled` | 0/1 | 0 (Redaktionsentscheid; das Mockup zeigt «an» nur zur Vorführung) | Medien › Wünsche › Einstellungen, «Wünsche aktiv»; einschaltbar nur nach gelungenem Test (**Vorschlag**, T-1) |
| `wishes.allow_movies`, `wishes.allow_shows` | 0/1 | 1, 1 | «Filme erlauben», «Serien erlauben» |
| `wishes.from_spotlight`, `wishes.from_search`, `wishes.from_detail` | 0/1 | 1, 1, 1 | «Anfordern aus …» |
| `wishes.already_requested` | `join`/`status` | `join` (F-7) | «Bereits angeforderte Titel» |
| `wishes.unmapped` | `deny`/`fallback` | `deny` (F-4) | «Konten ohne Zuordnung» |
| `wishes.shared_seerr_user` | Zahl | leer | Auswahl Sammelbenutzer |
| `wishes.new_accounts_can_request` | 0/1 | 0 | «Neue Konten dürfen anfordern» |
| `wishes.allow_withdraw` | 0/1 | 1 (F-17) | «Zurückziehen erlauben» |
| `wishes.pin_external_hide_title` | 0/1 | 1 (F-15) | «Externe Nachrichten bei PIN-Profilen ohne Titel» |

Die Einstellungen unter Wünsche › Einstellungen gelten sofort (`setWishSettings` je Änderung, kein «Speichern»).
Die Takte der Jobs stehen in `job_schedules` (`docs/adr/0017-scheduler-fortschritt-pause-abbruch.md`), nicht in `settings`. Die Spotlight-Werte stehen in
`spotlight_config` (ein Ort je Einstellung, Anhang F 4).

## 3. Migrationen

### 3.1 Regeln (CLAUDE.md Regel 5, `server/migrations/TEMPLATE.sql.txt`)

- Jede Datei beginnt mit `-- +goose Up`, `-- +goose NO TRANSACTION`, `PRAGMA foreign_keys=OFF;`, dann `BEGIN;`
  … `COMMIT;`, dann `PRAGMA foreign_key_check;` und `PRAGMA foreign_keys=ON;`.
- Nur vorwärts, keine Down-Migration, eine committete Migration wird nie geändert.
- Neubau: neue Tabelle `…_new` anlegen, Daten kopieren, alte löschen, umbenennen, Indizes, Trigger und
  abhängige Views neu anlegen.
- Vor jedem Lauf sichert der Server mit `VACUUM INTO` (fünf behalten).
- Je Stufe ein eingefrorener DB-Stand in `testdata/db/` (heute 46 Dateien) mit Test `frozen_<name>_test.go`
  nach dem Muster `server/internal/store/frozen_detailseiten3_test.go`, dazu je Migration ein
  `migration<nummer>_test.go` (Muster `migration00065_test.go`).

### 3.2 Reihenfolge je Stufe (Nummern vergibt der Bau, nächste freie heute 00066)

| Stufe | Migration (Kurzname, Vorschlag) | Inhalt |
|---|---|---|
| 1 | `wishes_mapping` | `seerr_users`, `wish_account_map`, `wish_profile_map`, Trigger der Kontoregel, `seerr_log` |
| 1 | `wishes_rights` | `accounts`/`profiles` + `can_request` (Neubau oder `ADD COLUMN`, 2.3), `visible_items` neu, `invitations` + `can_request`, Rückfüllung |
| 2 | `wishes` | `wishes` (mit `withdraw_pending`, `checked_at`), `wish_seasons` (mit `seerr_request_id`, `notified_at`, + Trigger), `wish_requesters` (mit `notified_at`), `wish_events` |
| 2 | `external_titles` | `external_titles`, `external_title_texts`, `external_title_images` |
| 2 | `wishes_notifications` | Neubau `notifications`, `deliveries`; Rückfüllung `account_channels` |
| 3 | `spotlight_missing` | Spalten an `spotlight_entries`, `spotlight_config`; Neubau `home_rows` mit Zeile `wishes` |
| 4 | `series_catalog` | `series_catalog`; Datenmigration `account_channels` (`wish_review` für bestehende Administratorkonten) |
| 4 | `webhook_jellyseerr` | Neubau `webhook_integrations` |

Eingefrorene Stände (**Vorschlag** für Namen): `testdata/db/wuensche1.sql` nach Stufe 1, `wuensche2.sql`,
`wuensche3.sql`, `wuensche4.sql`, je mit Konten, Profilen, Zuordnungen und Wünschen in allen Zuständen.

## 4. API

### 4.1 Konventionen

Alle Pfade unter `/api/v1`. Fehler als RFC 9457 `application/problem+json` mit `code`
(`api/openapi.yaml:39-41`, Schema `Problem` Z. 10651); Validierungsfehler 422 `validation_failed` mit
`errors[]`; Mutationen brauchen `X-Requested-With: Slouchcat`, sonst 403 `csrf_header_missing`; Blättern mit
`offset` (≥ 0) und `limit` (1–100, Vorgabe 50). Nicht sichtbar heisst 404. Jede Operation trägt `x-visibility` und
steht im Vertragstest (Abschnitt 4.7). Bestehende gemeinsame Antworten (`Unauthenticated`, `AdminOnly`,
`PrimaryProfileRequired`, Z. 10172ff.) werden wiederverwendet. Zuviele Versuche: 429 `too_many_attempts` mit
`Retry-After` (Muster Z. 261).

### 4.2 Neue Schemas (Kern)

| Schema | Felder (Auswahl) |
|---|---|
| `ExternalTitleRef` | `media`, `tmdbId`, `title`, `year?`, `minAge?`, `images {poster?, backdrop?, logo?}`, `hasTrailer`, `note?` (Spotlight-Notiz), `wish? {id, state, stateDetail?}`, `canRequest`, `requestReason?` |
| `ExternalTitle` | alles aus `ExternalTitleRef` + `originalTitle?`, `overview?`, `tagline?`, `releaseDate?`, `runtimeMinutes?`, `genres[]`, `status?`, `seasons[] {number, episodeCount, airDate?, state}`, `nextEpisode?`, `trailers[]`, `request {canRequest, reason?, seerrUser?, autoApprove?, quota?}` |
| `Wish` | `id`, `media`, `tmdbId`, `title`, `year?`, `state`, `stateDetail?`, `failReason?`, `failText?`, `origin`, `role` (`owner`/`joined`), `seasons[] {number, state}`, `external?` (`ExternalTitleRef`), `item?` (`ItemRef`), `canWithdraw`, `canRequestAgain`, `hint?` (z. B. `already_approved` nach einem zu späten Zurückziehen), `createdAt`, `updatedAt`, `availableAt?`, `events[]` (höchstens 20). Nur in scoped Antworten |
| `WishPage` | `items[]`, `total`, `counts {open, pending, processing, partial, available, closed}` |
| `WishCreate` | `media`, `tmdbId`, `seasons?` (`"all"` oder Zahlenliste), `source?` (**Vorschlag**: `spotlight`, `search`, `detail`, `list`) |
| `WishRights` | `enabled`, `canRequest`, `reason?` (Enum, 4.4), `movies`, `shows` |
| `SeerrSettings` | `enabled`, `url`, `apiKey` (`writeOnly`), `apiKeyHint?`, `source` (`env`/`stored`/`none`) |
| `SeerrStatus` | `state` (`not_configured`, `connected`, `key_invalid`, `unreachable`, `instance_changed`, `disabled`), `version?`, `lastSync?`, `nextSync?`, `initialSync? {page, pages}`, `counts {open, queued, unmapped}`, `log[]` (20) |
| `SeerrTestResult` | `checks[] {key, ok, detail?}` mit `key` IN (`reachable`, `key_valid`, `version`, `act_as_user`, `instance`, `csrf_off`, `radarr`, `sonarr`); `version` mit `ok = false` und `detail = not_verified` bei Versionen unter Jellyseerr 2.7 bzw. Seerr 3.0 (Warnung) |
| `SeerrUser` | `id`, `displayName`, `email?`, `canRequestMovies`, `canRequestShows`, `autoApprove`, `quota?`, `gone`, `mappedTo? {accountId, email}` |
| `WishMappings` | `accounts[] {accountId, email, seerrUserId?, suggestion?, profiles[] {profileId, name, seerrUserId?, mode}}`, `mapped`, `total` |
| `AdminWishTitle` | `itemId?` (Zeichenkette, keine `ItemRef`), `media`, `tmdbId`, `tvdbId?`, `imdbId?`, `title`, `year?`, `posterUrl?` (fehlender Titel: `getAdminExternalTitleImage`; vorhandener: `getAdminItemImage`) |
| `AdminWish` | `id`, `title: AdminWishTitle`, `state`, `stateDetail?`, `failReason?`, `origin`, `seasons[] {number, state, seerrRequestId?}`, `requesters[] {accountEmail, profileName, role}`, `seerrUser?`, `seerrRequestId?`, `diagnosis?`, `seerrLink?`, `events[]`, `createdAt`, `updatedAt`. **Keine** `ItemRef` und keine scoped Adresse (Regel 7, Muster `adminCurationAnswer`) |
| `WishSettings` | alle `wishes.*` aus 2.12 |

### 4.3 Beispiel `ExternalTitleRef` (Beispielwerte)

```json
{
  "media": "movie",
  "tmdbId": 1061474,
  "title": "Superman",
  "year": 2025,
  "minAge": 12,
  "images": {
    "poster": "/api/v1/titles/movie/1061474/images/poster/w400",
    "backdrop": "/api/v1/titles/movie/1061474/images/backdrop/w1280",
    "logo": "/api/v1/titles/movie/1061474/images/logo/w400"
  },
  "hasTrailer": true,
  "wish": { "id": 42, "state": "processing", "stateDetail": "importing" },
  "canRequest": false,
  "requestReason": "already_requested"
}
```

Die Bildadressen zeigen immer auf Slouchcat, nie auf TMDB (Anhang F 2.12).

### 4.4 Beispiel `createWish`

Anfrage (Profil Noah, Konto marco, Jellyseerr-Benutzer `kinder`):

```http
POST /api/v1/wishes
Content-Type: application/json
X-Requested-With: Slouchcat

{ "media": "show", "tmdbId": 153312, "seasons": [1, 2], "source": "detail" }
```

Antwort 201:

```json
{
  "id": 57,
  "media": "show",
  "tmdbId": 153312,
  "title": "Tulsa King",
  "year": 2022,
  "state": "pending",
  "origin": "slouchcat",
  "role": "owner",
  "seasons": [ { "number": 1, "state": "pending" }, { "number": 2, "state": "pending" } ],
  "canWithdraw": true,
  "canRequestAgain": false,
  "createdAt": "2026-11-16T19:04:11Z",
  "updatedAt": "2026-11-16T19:04:12Z",
  "events": [
    { "at": "2026-11-16T19:04:11Z", "kind": "created" },
    { "at": "2026-11-16T19:04:12Z", "kind": "submitted" }
  ]
}
```

Antworten von `createWish`:

| Status | `code` | Wann |
|---|---|---|
| 201 | – | neuer Wunsch (Zustand `queued`, `pending`, `processing` oder `failed` mit Grund, `03-architektur.md` 6.1) |
| 200 | – | bestehender Wunsch: dasselbe Profil fragt erneut (idempotent) oder Mitwunsch (`role: joined`) |
| 403 | `wish_not_allowed` | kein Recht; Erweiterung `reason` IN (`disabled`, `account_not_allowed`, `profile_not_allowed`, `no_mapping`, `seerr_user_gone` (diese fünf wie `04-benutzer-profilmapping.md`, Abschnitt 9), `movies_not_allowed`, `shows_not_allowed`, `source_not_allowed`, `tvdb_id_missing`, `status_only`) (**Vorschlag** Liste; `status_only` = Einstellung «Nur Zustand zeigen») |
| 404 | `not_found` | Titel bei TMDB unbekannt oder für das Profil nicht sichtbar (Freigabe zu hoch, unbekannt bei Profil unter 18, in der Bibliothek, aber nicht sichtbar) |
| 409 | `already_available` | in der Bibliothek und sichtbar; Erweiterung `item` (`ItemRef`) |
| 422 | `validation_failed` | `media` falsch, `tmdbId` ≤ 0, Staffel 0, Staffeln bei Film, keine wählbare Staffel |
| 429 | `too_many_attempts` | Rate-Limit je Profil (**Vorschlag** 20/h), `Retry-After` |
| 503 | `tmdb_unreachable` | Titel nicht im Cache und TMDB antwortet nicht |

`403` ist hier zulässig, weil es um das **Recht** zu wünschen geht, nicht um die Sichtbarkeit eines Titels.

### 4.5 Neue Operationen

| operationId | Methode, Pfad | `x-visibility` | Anfrage | Antworten | Stufe |
|---|---|---|---|---|---|
| `getSeerrSettings` | GET `/admin/seerr` | admin | – | 200 `SeerrSettings` | 1 |
| `updateSeerrSettings` | PUT `/admin/seerr` | admin | `SeerrSettings` (leerer `apiKey` behält ihn) | 200; 422 `validation_failed`, `seerr_url_invalid`; 409 `env_wins` | 1 |
| `testSeerrConnection` | POST `/admin/seerr/test` | admin | optional ungespeicherte `url`, `apiKey`. Weicht der Host von `url` vom gespeicherten ab, ist `apiKey` Pflicht (422 `seerr_key_required`); der gespeicherte Schlüssel geht nie an eine neue Adresse | 200 `SeerrTestResult` (auch bei Fehlschlägen einzelner Punkte) | 1 |
| `getSeerrStatus` | GET `/admin/seerr/status` | admin | – | 200 `SeerrStatus` | 1 |
| `syncWishesNow` | POST `/admin/seerr/sync` | admin | – | 202; 409 `job_type_paused`; 422 `seerr_not_configured` | 2 |
| `listSeerrUsers` | GET `/admin/wishes/mappings/users` | admin | – | 200 `SeerrUser[]` | 1 |
| `refreshSeerrUsers` | POST `/admin/wishes/mappings/users/refresh` | admin | – | 202; 422 `seerr_not_configured` | 1 |
| `getWishMappings` | GET `/admin/wishes/mappings` | admin | – | 200 `WishMappings` | 1 |
| `setAccountWishMapping` | PUT `/admin/wishes/mappings/accounts/{accountId}` | admin | `{seerrUserId: int\|null}` | 200; 404; 409 `seerr_user_taken` | 1 |
| `setProfileWishMapping` | PUT `/admin/wishes/mappings/profiles/{profileId}` | admin | `{mode: same_as_account\|own, seerrUserId?}` | 200; 404; 409 `seerr_user_taken` | 1 |
| `applyWishMappingSuggestions` | POST `/admin/wishes/mappings/suggestions` | admin | `{accountIds[]}` | 200 `WishMappings`; 409 bei Konflikt (nichts übernommen) | 1 (**Vorschlag**, für «Alle Vorschläge übernehmen» in einem Zug) |
| `getWishSettings` | GET `/admin/wishes/settings` | admin | – | 200 `WishSettings` | 1 |
| `setWishSettings` | PUT `/admin/wishes/settings` | admin | `WishSettings` | 200; 422 | 1 |
| `listWishes` | GET `/wishes?state=&offset=&limit=` | scoped | Filter `open`, `available`, `closed`, `all` | 200 `WishPage` (nur eigene, auch Mitwünsche) | 2 |
| `createWish` | POST `/wishes` | scoped | `WishCreate` | siehe 4.4 | 2 |
| `withdrawWish` | DELETE `/wishes/{id}` | scoped | – | 204; 404 (fremd oder unbekannt); 409 `wish_not_withdrawable`; 403 `wish_not_allowed` (`reason: withdraw_disabled`) | 2 |
| `listAccountWishes` | GET `/account/wishes` | scoped, zusätzlich Hauptprofil (`requirePrimary`, `server/internal/auth/profiles.go:40`) | wie `listWishes` | 200 `WishPage` mit `profileName` je Eintrag; 403 `primary_profile_required`. Die Antwort trägt `ItemRef` und ist darum `scoped` (Regel 7) | 2 |
| `getExternalTitle` | GET `/titles/{media}/{tmdbId}` | scoped | – | 200 `ExternalTitle` (mit `lastWish?` {state, at, reason} eines geschlossenen Wunsches); 404 (auch ohne Recht oder wenn alle drei Stellen aus sind, ausser bei eigenem Wunsch); 409 `already_available` mit `item` (Client leitet auf `/items/{id}`); 503 `tmdb_unreachable` | 2 |
| `searchExternalTitles` | GET `/titles/search?q=&media=&offset=&limit=` | scoped | `q` 2–100 Zeichen | 200 `{items: ExternalTitleRef[]}`; leer ohne Recht oder mit «Anfordern aus der Suche» aus (F-9); 429; 503 `tmdb_unreachable`. Die TMDB-Suche liefert keine Freigaben: Für Profile unter 18 holt der Server die Details der ersten 20 Treffer (zwischengespeichert in `external_titles`); Treffer ohne bekannte Freigabe gelten als 18 und fehlen diesen Profilen (`03-architektur.md`, Abschnitt 16) | 2 |
| `getExternalTitleImage` | GET `/titles/{media}/{tmdbId}/images/{kind}/{size}` | scoped | `kind` = `ImageKind`, `size` wie `getItemImage` (w200 … w3840) | 200 Bild; 404 (auch nicht sichtbar oder ohne Recht, ausser bei eigenem Wunsch); 503 | 2 |
| `getAdminExternalTitleImage` | GET `/admin/titles/{media}/{tmdbId}/images/{kind}/{size}` | admin | wie oben | 200 Bild; 404; 503. Bild fehlender Titel für die Verwaltung (Anfragen, Spotlight), ohne Profilbezug, Muster `getAdminItemImage` | 2 |
| `listAdminWishes` | GET `/admin/wishes?state=&unmapped=&q=&offset=&limit=` | admin | Filter | 200 `{items: AdminWish[], total, counts}` | 2 |
| `retryWish` | POST `/admin/wishes/{id}/retry` | admin | – | 202; 404; 409 `wish_not_retryable` | 2 |
| `confirmSeerrInstance` | POST `/admin/seerr/instance` | admin | `{action: confirm\|reset}` | 200 `SeerrStatus`; 409 `instance_unchanged`. Bestätigt den neuen Fingerabdruck (Erstabgleich folgt) oder löscht alle Zuordnungen (**Vorschlag**, `03-architektur.md`, Abschnitt 7.4) | 1 |
| `importSeerrRequests` | POST `/admin/wishes/mappings/import` | admin | `{seerrUserId?}` (leer: alle zugeordneten) | 202; 422 `seerr_not_configured`. Reiht `wishes.import` ein (F-21) | 4 |
| `countSeerrRequests` | GET `/admin/wishes/mappings/users/{seerrUserId}/requests/count` | admin | – | 200 `{count}` aus `GET /request?requestedBy=…&take=1` (`pageInfo.results`), für die Meldung nach einer Zuordnung (**Vorschlag**) | 4 |
| `approveWish` | POST `/admin/wishes/{id}/approve` | admin | – | 200 `AdminWish`; 409 `wish_not_pending`; 502 `seerr_failed`; 503 `seerr_unreachable` | 4 |
| `declineWish` | POST `/admin/wishes/{id}/decline` | admin | – | wie `approveWish` | 4 |

Pfade für Stufe 1 folgen Anhang F («`/admin/seerr…`», «`/admin/wishes/mappings…`»); die genaue Form ist ein
**Vorschlag**. `getExternalTitleImage` ist ein `http.Handler` nach Regel 6, im Vertrag beschrieben wie
`getItemImage` (`api/openapi.yaml:1438`), mit `Cache-Control` wie die Titelbilder.

`approveWish`/`declineWish` rufen `POST /request/{id}/approve|decline` auf. **Vorschlag:** mit `X-API-User` des
Jellyseerr-Benutzers, der dem handelnden Administratorkonto zugeordnet ist, wenn er `MANAGE_REQUESTS` oder
`ADMIN` hat; sonst ohne Header (Benutzer 1).

### 4.6 Erweiterte Operationen

| operationId | Erweiterung | Stufe | brechend? |
|---|---|---|---|
| `getSession` | `SessionInfo.wishes` (`WishRights`) | 1 | nein (optionales Feld) |
| `listAccounts`, `updateAccountAccess` (Konten) | `canRequest`, Zuordnung `seerrUser? {id, displayName}`, Chip-Zustand `wishState` (`allowed`, `not_allowed`, `mapping_missing`) | 1 | nein |
| `createProfile`, `updateProfile`, `listProfiles` | `canRequest` | 1 | nein |
| `createInvitation`, `listInvitations` | `canRequest` | 1 | nein |
| `getDashboard` | Kennzahl «Offene Wünsche», Hinweise, Integrationsstatus `seerr` | 1/2 | Enum des Integrationsschlüssels: ja, nur Verwaltung |
| `listNotifications` | `Notification.kind` + `wish_available` (2), weitere in 4; `item` wird optional, neu `wish` | 2, 4 | **ja** (`item` ist heute Pflicht, `api/openapi.yaml:16683`) |
| Kanal-Ereignisse (`EventType`) | + `wish_available` (2), `wish_status`, `wish_review` (4, wie `06-benachrichtigungen.md`, Abschnitt 2) | 2, 4 | **ja** (Enum in Antworten) |
| `getHome`, `listHomeRow` | **Vorschlag:** `HomeRow.external[]` mit `{position, title: ExternalTitleRef}`; `items` bleibt reine `ItemRef`-Liste | 3 | nein (neues optionales Feld) |
| `getHomeRows`, `setHomeRows`, `HomeRowKey` | + `wishes` | 3 | **ja** (Präzedenz 1.19.0, `api/openapi.yaml:54`) |
| `getSpotlightConfig`, `setSpotlightConfig` | `showMissing`, `maxMissing`, `missingOrder`, Zähler | 3 | nein |
| `getSpotlightEntries`, `setSpotlightEntryOverride` | `posterUrl` auch für fehlende, immer über `getAdminExternalTitleImage` (keine scoped Adresse in Verwaltungsantworten); `wish?` ohne `ItemRef`; Aktion «Anfordern» über `createWish` des aktiven Profils (= Hauptprofil) | 3 | nein |
| `search` | unverändert; fehlende Titel über `searchExternalTitles` | – | – |
| `getItem` | `ItemDetail.catalog? {seasons[] {number, episodeCount, state}, status, nextEpisode?}`, `canRequestSeasons` | 4 | nein |
| `receiveWebhook`, `createIntegration`, `listIntegrations` | Art `jellyseerr` | 4 | Enum `kind`: ja, nur Verwaltung |

**Widerspruch zu Anhang F:** Anhang F nennt die Erweiterung von `getHome` «nicht brechend». Das stimmt nur, wenn
fehlende Titel in einem **eigenen** Feld kommen (Vorschlag oben). Die neue Reihe `wishes` erweitert `HomeRowKey`;
das Projekt hat neue Schlüssel bisher als brechend gezählt (1.15.0, 1.19.0, 1.23.0). Ebenso brechend sind die
Enums der Benachrichtigungen in Stufe 2. Folge für die Vertragsfassung:

| Stufe | Vertrag |
|---|---|
| 1 | Minor, nicht brechend |
| 2 | Minor, brechend (Benachrichtigungen) |
| 3 | Minor, brechend (`HomeRowKey`) |
| 4 | Minor, brechend (`EventType`, Webhook-Art) |

Nach «Vertrag = Tag» wird jede Anhebung beim Merge getaggt (`docs/dev/HANDOVER.md:120-127`). Die Nummern vergibt
der Bau.

### 4.7 Vertragstest (Regel 7)

Jede neue Operation bekommt einen Eintrag in `TestContractCoversEveryOperation`
(`server/internal/api/contract_access_test.go:494`) und in der passenden Gruppe:

| Gruppe | Operationen |
|---|---|
| `TestContractAdminOperations` (Z. 655) | alle `admin` aus 4.5, dazu `getAdminExternalTitleImage` |
| Verwaltungsantworten der Wünsche (Muster `adminCurationAnswer`, `server/internal/api/contract_curation_test.go:30-43`) | `listAdminWishes`, `approveWish`, `declineWish`, `getSpotlightEntries`: keine `ItemRef` und keine scoped Adresse in der Antwort |
| scoped-Fälle | `listWishes`, `createWish`, `withdrawWish`, `getExternalTitle`, `searchExternalTitles`, `getExternalTitleImage`, `listAccountWishes`: ohne Profil 403 `profile_required`, fremder Wunsch 404, zu hohe Freigabe 404, Profil ohne Recht 404 (ausser bei eigenem Wunsch); `listAccountWishes` mit weiterem Profil 403 `primary_profile_required` |
| `TestContractPublicOperations` (Z. 747) | `receiveWebhook` mit Art `jellyseerr` (Stufe 4) |
| Altersfälle (Muster `TestContractRatingChange`, Z. 798) | fehlender Titel mit `min_age` 16 für Profil 12: 404 bei `getExternalTitle`, Bild, `createWish`; unbekannte Freigabe für Profil 12: 404 |

## 5. Jellyseerr-Endpunkte (streng nach Anhang D)

Header überall: `X-Api-Key`. Spalte «Header» nennt nur, ob zusätzlich `X-API-User` gesetzt wird.

| Zweck | Methode, Pfad | Header | Slouchcat liest | Fallstricke |
|---|---|---|---|---|
| Erreichbar, Version | GET `/api/v1/status` | keiner nötig (ohne Anmeldung, Anhang D 10) | `version`, `commitTag` | – |
| Schlüssel prüfen, Fingerabdruck | GET `/api/v1/auth/me` | ohne | `id` (= 1), `displayName`, `email`, `createdAt` | falscher Schlüssel: 403 `You do not have permission to access this endpoint` (Anhang D 2.1) |
| `X-API-User` prüfen | GET `/api/v1/auth/me` | `X-API-User` | `id` = gesendete ID | undokumentiert (Anhang D 2.1); unbekannte ID: 403 |
| Radarr/Sonarr vorhanden | GET `/api/v1/service/radarr`, `/service/sonarr` | ohne | Anzahl Einträge, `isDefault` | nur Hinweis; Slouchcat spricht Radarr/Sonarr nie an |
| CSRF erkennen | POST `/api/v1/request` mit `{}` | ohne | Status 400 (aus) oder 403 (an) | **abgeleitet**, nicht gegen eine Instanz geprüft (Anhang D 2.4, 2.5) |
| Benutzerliste | GET `/api/v1/user?take=100&skip=n&sort=created` | ohne | `id`, `displayName`, `email`, `userType`, `permissions`, `movieQuotaLimit/Days`, `tvQuotaLimit/Days` | E-Mail nur mit `MANAGE_USERS` sichtbar (Benutzer 1 hat es); `permissions` als Zahl |
| Kontingent | GET `/api/v1/user/{id}/quota` | ohne | `movie {limit, days, used, remaining, restricted}`, `tv {…}` | braucht `MANAGE_USERS` und `MANAGE_REQUESTS` (Benutzer 1 hat `ADMIN`) |
| Vorprüfung vor dem Senden (ab Versuch 2) und vor dem Zurückziehen | GET `/api/v1/movie/{tmdbId}`, `/api/v1/tv/{tmdbId}` | `X-API-User` (Senden, Zurückziehen), ohne (Anhängen nach 409 und 202) | `mediaInfo.id`, `mediaInfo.status`, `mediaInfo.requests[] {id, status, requestedBy.id, seasons[]}`, `mediaInfo.seasons[]`, `externalIds.tvdbId` | jeder Aufruf geht an TMDB (Anhang D 6.2): nur im Job, nie im Seitenaufbau. **Nicht verifiziert:** `externalIds.tvdbId` in `GET /tv/{id}` belegt Anhang D nicht; Slouchcat nimmt die TVDB-ID aus `external_titles` und liest dieses Feld nur zur Kontrolle |
| Wunsch anlegen | POST `/api/v1/request` | `X-API-User` | `id`, `status`, `media.id`, `media.status`, `updatedAt`, `seasons[]` | **ohne `userId`**; `seasons` als Zahlenliste; `tvdbId` mitsenden; 409 und 202 sind Erfolg; 403-Texte unterscheiden (Anhang D 3.4); Spec dokumentiert nur 201 |
| Abgleich | GET `/api/v1/request?take=50&skip=n&sort=modified&sortDirection=desc` (ohne `filter`; Standard alle) | ohne | `results[] {id, status, updatedAt, requestedBy.id, media {id, tmdbId, tvdbId, mediaType, status, seasons[]}, seasons[]}`, `pageInfo` | kein `since`; unbekannte Parameter → 400; Sperrliste-Medien fehlen; jeder Aufruf fragt alle Radarr/Sonarr ab (Anhang D 6.1). **Nicht verifiziert:** dass `media` mit `seasons[]` in den Ergebnissen steht; gegen eine Instanz prüfen, sonst Medienstatus aus der Einzelprüfung |
| Übernahme (F-21) | GET `/api/v1/request?requestedBy=<id>&take=50&skip=n` | ohne (Benutzer 1 hat `MANAGE_REQUESTS`) | wie Abgleich; mit `take=1` nur `pageInfo.results` für die Zahl | `requestedBy` nur mit `MANAGE_REQUESTS` oder `REQUEST_VIEW` (Anhang D 6.1) |
| Einzelprüfung | GET `/api/v1/request/{id}` | ohne | wie oben | 404 = gelöscht (auch durch Löschen des Benutzers, CASCADE, Anhang D 7.2) |
| Zurückziehen | DELETE `/api/v1/request/{id}` | `X-API-User` | Status 204 | nur eigener Request im Status PENDING; sonst **401**, nicht 403 (Anhang D 8); 401 → Zustand neu lesen, Wunsch wieder öffnen; 404 → schon weg; 403 → wie Senden mit `GET /auth/me` unterscheiden (`03-architektur.md`, Abschnitt 6.6) |
| Freigeben, Ablehnen | POST `/api/v1/request/{id}/approve`, `/decline` | `X-API-User` eines Admins oder ohne (4.5) | neuer Status | nur `MANAGE_REQUESTS`; in 3.x nur PENDING, sonst 409; kein Ablehnungsgrund (Anhang D 8) |
| Wiederholen | POST `/api/v1/request/{id}/retry` | ohne | neuer Status | nur FAILED, sonst 409 (Anhang D 8) |
| Link «In Jellyseerr öffnen» | – (kein Aufruf) | – | Adresse `seerr.url` + `/movie/{tmdbId}` bzw. `/tv/{tmdbId}` | **Vorschlag**: Pfad der Jellyseerr-Oberfläche, nicht geprüft |

Nicht verwendet, weil es sie nicht gibt oder weil sie stören: Bündelabruf nach TMDB-IDs, Änderungs-Feed, Feld
für fremde Kennungen an Benutzer oder Request, Ablehnungsgrund, mehrere Webhook-Ziele, `userId` im Body,
`ignoreQuota`, `POST /media/{id}/{status}` (Slouchcat setzt in Jellyseerr nie einen Medienstatus),
`POST /settings/notifications/webhook` (würde ein bestehendes Webhook-Ziel überschreiben, Anhang D 9.4).
