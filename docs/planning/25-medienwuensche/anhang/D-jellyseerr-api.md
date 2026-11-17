# D – Jellyseerr/Seerr-API für die Anbindung von Slouchcat

Stand der Recherche: 16. November 2026. Primärquelle ist der Quellcode von **Seerr v3.5.2** (Tag `v3.5.2`,
Commit `18a90b4e`, veröffentlicht 12.11.2026), geholt über `gh api` / `raw.githubusercontent.com`.
Vergleich mit **Jellyseerr v2.7.3** (letzte 2.x-Version, 14.08.2025, gleiches Repo, Tag `v2.7.3`).

Abkürzungen für Quellen (alle Zeilenangaben beziehen sich auf Tag `v3.5.2`, sofern nicht anders vermerkt):

- `GH:` = `https://github.com/seerr-team/seerr/blob/v3.5.2/`
- `SPEC` = `GH:seerr-api.yml` (OpenAPI 3, `info.version: '1.0.0'`)
- `DOCS:` = `https://github.com/seerr-team/seerr/blob/develop/docs/` (Quelle von docs.seerr.dev)

Kennzeichnung: **belegt** = im Code oder in der Spec nachgelesen (mit Stelle). **abgeleitet** = folgt
zwingend aus dem gelesenen Code, aber nicht gegen eine laufende Instanz geprüft. **nicht verifiziert** =
Vermutung oder nur Doku ohne Codebeleg.

Wichtig vorweg: Die OpenAPI-Datei ist an mehreren Stellen **ungenau oder veraltet** (Enum-Werte,
Fehlercodes, fehlende Felder). Massgeblich ist der Code. Die Unterschiede sind unten jeweils vermerkt.

---

## 1. Projektstand

| Aussage | Status |
|---|---|
| Das Repo `Fallenbagel/jellyseerr` leitet auf `seerr-team/seerr` um (GitHub-API liefert für beide `full_name: seerr-team/seerr`). Standardzweig `develop`, nicht archiviert, letzter Push 16.11.2026. | belegt (`gh api repos/Fallenbagel/jellyseerr`) |
| `sct/overseerr` ist **archiviert** (letzter Push 15.02.2026, letzte Version v1.35.0 vom 15.02.2026). | belegt (`gh api repos/sct/overseerr`) |
| Seerr v3.0.0 erschien am 14.02.2026; aktuelle Versionen: v3.5.2 (12.11.2026), v3.5.1 (21.10.), v3.5.0 (28.09.). | belegt (`gh api .../releases`) |
| Die API-Datei heisst jetzt `seerr-api.yml` (in v2.7.3: `jellyseerr-api.yml`; `overseerr-api.yml` existiert im Repo nicht mehr). | belegt |
| Migration: «Whether you come from Overseerr or Jellyseerr, you don't need to perform any manual migration steps» – automatisch beim ersten Start. Image `ghcr.io/seerr-team/seerr`. | belegt (`DOCS:migration-guide.mdx` Z. 12–13, 75) |
| Die Pfade bleiben unter `/api/v1/...` (`server.use('/api/v1', routes)`). | belegt (`GH:server/index.ts` Z. 264) |

**Pfadvergleich v2.7.3 ↔ v3.5.2** (automatischer Abgleich der `paths:` beider OpenAPI-Dateien, 152 → 168 Pfade):

- Entfernt: nur `/tv/{tvId}/season/{seasonId}` – ersetzt durch `/tv/{tvId}/season/{seasonNumber}` (nur Parametername).
- Neu u. a.: `/blocklist`, `/blocklist/{tmdbId}`, `/blocklist/collection/{collectionId}`, `/overrideRule/advancedRequest`, `/user/jellyfin/{jellyfinUserId}`, Jellyfin-QuickConnect-Routen, `/settings/metadatas`.
- `/blacklist` wird noch bedient, aber als veraltet markiert (`Deprecation`-Header, `sunsetDate: '2026-06-01'`, Datum ist überschritten, Route trotzdem noch eingebunden). belegt (`GH:server/routes/index.ts` Z. 161–171)

**Bekannte Brüche 2.x → 3.x** (alle belegt durch Vergleich der Tags):

- «Blacklist» → «Blocklist»: `MediaStatus.BLACKLISTED` → `BLOCKLISTED` (Zahlenwert 6 bleibt), Berechtigungen `MANAGE_BLACKLIST`/`VIEW_BLACKLIST` → `MANAGE_BLOCKLIST`/`VIEW_BLOCKLIST` (Werte bleiben). Fehlertext `This media is blacklisted.` → `This media is blocklisted.`
- Webhook-Variablen `*_settings_discordId` → `*_settings_discordIds` (Plural, JSON-Array). Neu: `media_imdbid`, `media_jellyfinMediaId`, `media_plexRatingKey(4k)`, `requestedBy_jellyfinUserId`.
- `ignoreQuota` am Request gibt es erst in 3.x (in v2.7.3 0 Treffer in `MediaRequest.ts`).
- Approve/Decline nur noch für `PENDING` (409 sonst); in v2.7.3 gab es diese Sperre nicht.
- `middleware/auth.ts` ist zwischen v2.7.3 und v3.5.2 **byte-gleich** (`diff` ohne Ausgabe).

---

## 2. Authentifizierung

### 2.1 API-Schlüssel und `X-API-User` – belegt

`GH:server/middleware/auth.ts` Z. 9–34, wörtlich:

```ts
if (req.header('X-API-Key') === settings.main.apiKey) {
  const userRepository = getRepository(User);

  let userId = 1; // Work on original administrator account

  // If a User ID is provided, we will act on that user's behalf
  if (req.header('X-API-User')) {
    userId = Number(req.header('X-API-User'));
  }

  user = await userRepository.findOne({ where: { id: userId } });
} else if (req.session?.userId) {
```

Folgen (abgeleitet aus diesem Code):

- Nur **ein** API-Schlüssel (`settings.main.apiKey`), kein Schlüssel pro Benutzer.
- Ohne `X-API-User` handelt der Aufruf als **Benutzer 1** (Eigentümer, hat `ADMIN`).
- Mit `X-API-User: <id>` handelt der Aufruf **vollständig als dieser Benutzer**: dessen Berechtigungen,
  Kontingente und Auto-Approve gelten. Keine Prüfung, ob der Header erlaubt ist; der Schlüssel genügt.
- Unbekannte ID → `req.user` bleibt leer → jede geschützte Route antwortet
  `403 {"message":"You do not have permission to access this endpoint"}` (Z. 43–56).
- `X-API-User` steht **nicht** in der OpenAPI-Datei und nicht in den Docs (grep über `seerr-api.yml`
  und `docs/`: 0 Treffer). Es ist ein undokumentiertes, aber seit Overseerr bestehendes Verhalten.

### 2.2 OpenAPI-Beschreibung – belegt

`SPEC` Z. 5–11: «Two primary authentication methods are supported: Cookie Authentication … `/auth/plex` or
`/auth/local` … API Key Authentication … passing an `X-Api-Key` header». Security-Schemes Z. 2166–2175:
`cookieAuth` (Cookie `connect.sid`) und `apiKey` (Header `X-Api-Key`). Globale Vorgabe Z. 8323–8325.

### 2.3 Sitzungs-Login – belegt

`GH:server/routes/auth.ts`: `POST /auth/plex` (Z. 69), `POST /auth/jellyfin` (Z. 255, auch Emby),
`POST /auth/local` (Z. 862, Body `{email, password}`, nur wenn `settings.main.localLogin`, sonst
`500 "Password sign-in is disabled."`), `POST /auth/logout`, `GET /auth/me`, Jellyfin-QuickConnect.
Für eine Server-zu-Server-Anbindung braucht Slouchcat keine Sitzung (abgeleitet).

### 2.4 CSRF – belegt / abgeleitet

`GH:server/index.ts` Z. 195–214: Ist `settings.network.csrfProtection` an, wird `csurf` global
vorgeschaltet; es gibt **keine Ausnahme für `X-Api-Key`**. Die UI sagt dazu wörtlich:
«Set external API access to read-only (requires HTTPS)» (`src/i18n/locale/en.json`,
`SettingsNetwork.csrfProtectionTip`). Folge: Mit CSRF-Schutz scheitern alle schreibenden API-Aufrufe
(POST/PUT/DELETE) von Slouchcat (abgeleitet). Standard ist aus (nicht im Detail verifiziert).

### 2.5 Request-Validierung – belegt / abgeleitet

`GH:server/index.ts` Z. 239–244: `OpenApiValidator.middleware({ apiSpec, validateRequests: true })`.
Anfragen werden also gegen `seerr-api.yml` geprüft. Unbekannte **Query**-Parameter weist
`express-openapi-validator` standardmässig mit 400 ab; zusätzliche Body-Felder sind zulässig, sofern
das Schema sie nicht verbietet (nicht verifiziert, Bibliotheks-Standard). Swagger-UI läuft auf jeder
Instanz unter `/api-docs` (Z. 238).

---

## 3. Wunsch anlegen: `POST /api/v1/request`

### 3.1 Body – belegt

`GH:server/interfaces/api/requestInterfaces.ts` Z. 16–30:

```ts
export type MediaRequestBody = {
  mediaType: MediaType;      // 'movie' | 'tv'
  mediaId: number;           // TMDB-ID
  tvdbId?: number;
  seasons?: number[] | 'all';
  is4k?: boolean;
  serverId?: number;
  profileId?: number;
  profileName?: string;
  rootFolder?: string;
  languageProfileId?: number;
  userId?: number;
  tags?: number[];
  ignoreQuota?: boolean;
};
```

`SPEC` Z. 6585–6637: Pflicht nur `mediaType`, `mediaId`. `tags` und `profileName` fehlen in der Spec,
stehen aber im Code (Abweichung).

### 3.2 `userId` – fremder Benutzer – belegt

`GH:server/entity/MediaRequest.ts` Z. 87–101:

```ts
if (
  requestBody.userId &&
  !requestUser.hasPermission([
    Permission.MANAGE_USERS,
    Permission.MANAGE_REQUESTS,
  ])
) {
  throw new RequestPermissionError(
    'You do not have permission to modify the request user.'
  );
} else if (requestBody.userId) {
  requestUser = await userRepository.findOneOrFail({ where: { id: requestBody.userId } });
}
```

- Der Aufrufer braucht **`MANAGE_USERS` UND `MANAGE_REQUESTS`** (Standardprüfung `type: 'and'`,
  `GH:server/lib/permissions.ts` Z. 50, 65) oder `ADMIN` (Z. 60–62, 73).
- `requestedBy` wird dann der Zielbenutzer (Z. 338, 453: `requestedBy: requestUser`).

### 3.3 Wessen Regeln gelten? – belegt, wichtig

| Prüfung | gilt für | Stelle |
|---|---|---|
| Darf Filme/Serien anfragen (`REQUEST`, `REQUEST_MOVIE`, `REQUEST_TV`, 4K-Varianten) | **Zielbenutzer** (`requestUser`) | Z. 107–139 |
| Kontingent (Quota) | **Zielbenutzer** (`requestUser.getQuota()`) | Z. 141–167, 440–446 |
| Quota umgehen (`ignoreQuota`) | nur wenn **Aufrufer** `MANAGE_REQUESTS` hat | Z. 143–155 |
| Auto-Approve (Status `APPROVED` statt `PENDING`) | **Aufrufer** (`user.hasPermission([AUTO_APPROVE…, MANAGE_REQUESTS])`) | Z. 340–353, 455–468, 493–506 |
| `modifiedBy` | Aufrufer, wenn auto-approved | Z. 354–367 |
| Server/Profil/Root-Ordner/Tags übernehmen | nur wenn **Aufrufer** `MANAGE_REQUESTS` oder `REQUEST_ADVANCED` | Z. 265–294 |
| Override-Regeln | werden mit `requestUser` ausgewertet | Z. 296–303 |

**Folge (abgeleitet):** Wer mit dem Admin-Schlüssel ohne `X-API-User` und mit `userId` anfragt, erzeugt
**immer auto-genehmigte** Wünsche (Benutzer 1 hat `ADMIN`, das alle Prüfungen bestätigt). Eine
Freigabe-Schleife in Jellyseerr gibt es dann nicht. Wer stattdessen `X-API-User: <ziel>` setzt (ohne
`userId`), bekommt genau das Verhalten, als hätte der Benutzer selbst geklickt (eigene Rechte, eigenes
Auto-Approve, eigene Quota).

### 3.4 Antwort und Fehler – belegt

`GH:server/routes/request.ts` Z. 313–346: Erfolg `201` mit dem `MediaRequest`-Objekt. Fehlerabbildung:

| Fehler | HTTP | Meldung (wörtlich, `MediaRequest.ts`) |
|---|---|---|
| `RequestPermissionError` | 403 | `You do not have permission to modify the request user.` / `You do not have permission to make movie requests.` / `… 4K movie requests.` / `… series requests.` / `You do not have permission to bypass user quota limits.` |
| `QuotaRestrictedError` | 403 | `Movie Quota exceeded.` / `Series Quota exceeded.` |
| `BlocklistedMediaError` | 403 | `This media is blocklisted.` (Z. 198) |
| `DuplicateMediaRequestError` | 409 | `Request for this media already exists.` (Z. 243–245) / `Auto-request for this media and user already exists.` (Z. 259–261) |
| `NoSeasonsAvailableError` | **202** (!) | `No seasons available to request` (Z. 439) |
| sonst (z. B. TMDB-Fehler, unbekannte `userId` → `findOneOrFail`) | 500 | Fehlertext |
| kein Benutzer | 401 | `You must be logged in to request media.` |

Fehlerformat: `{"message": "...", "errors": [...]}` (`GH:server/index.ts` Z. 282–285).
Die Spec dokumentiert **nur** `201` (Abweichung).

### 3.5 Bereits vorhanden / bereits angefragt – belegt

- **Film:** Es wird nur `existing[0]` geprüft; ist dieser Wunsch weder `DECLINED` noch `COMPLETED` →
  409 (Z. 229–246). Ein Film mit Status `AVAILABLE`, dessen alter Wunsch `COMPLETED` ist, kann also
  **erneut angefragt werden** (abgeleitet). Ist er schon verfügbar, setzt der Radarr-Schritt den neuen
  Wunsch sofort auf `COMPLETED` (`GH:server/subscriber/MediaRequestSubscriber.ts` Z. 325–334:
  «Media already exists, marking request as COMPLETED»).
- **Duplikat ist global, nicht pro Benutzer:** Ein zweiter Benutzer kann denselben Film nicht zusätzlich
  anfragen (409). Es gibt **keine** «Ich auch»-Funktion (abgeleitet aus Z. 218–246).
- **Blocklist:** Media mit `status = BLOCKLISTED` → 403.
- Jede Anfrage ruft TMDB ab (Z. 169–172); ohne TMDB-Zugang schlägt sie mit 500 fehl (abgeleitet).

---

## 4. Serien und Staffeln

- `seasons: 'all'` → alle TMDB-Staffeln **ausser Staffel 0** und ausser Staffeln ohne Folgen
  (`MediaRequest.ts` Z. 383–391). belegt
- `seasons: [0, …]` ist nur wirksam mit `settings.main.enableSpecialEpisodes`; sonst wird 0 still
  entfernt (Z. 392–394). belegt
- Bereits angefragte (nicht `DECLINED`/`COMPLETED`) und bereits vorhandene Staffeln (Staffelstatus ≠
  `UNKNOWN`/`DELETED`) werden **still herausgefiltert**; bleibt nichts übrig → 202
  `No seasons available to request` (Z. 396–439). Kommentar im Code: «we just throw out any duplicates
  but still approve the request». belegt
- **Weitere Staffeln derselben Serie** → **neuer, eigener Request** mit nur den neuen Staffeln (kein
  Fehler, kein Anhängen an den alten). abgeleitet aus Z. 396–514
- `partialRequestsEnabled` («Allow Partial Series Requests», Standard `true`) wirkt **nur im
  Frontend** (`src/components/RequestModal/TvRequestModal.tsx`); im Server wird die Einstellung nur
  gespeichert und ausgeliefert (grep: nur `server/lib/settings/index.ts`). Die API nimmt beliebige
  Staffellisten an. belegt
- Am **Request**: `seasons[]` = `SeasonRequest {id, seasonNumber, status: MediaRequestStatus, createdAt,
  updatedAt}` (`GH:server/entity/SeasonRequest.ts` Z. 16–37). belegt
- An **Media**: `seasons[]` = `Season {id, seasonNumber, status: MediaStatus, status4k: MediaStatus, …}`
  (`GH:server/entity/Season.ts` Z. 16–40), eager geladen (`Media.ts` Z. 150–155). belegt
- `PARTIALLY_AVAILABLE` für eine Staffel: wenn 0 < vorhandene Folgen < Gesamtfolgen
  (`baseScanner.ts` Z. 404–414). belegt
- Ein Serien-Request wird `COMPLETED`, sobald **alle** seine Staffeln `AVAILABLE` oder `DELETED` sind
  (`GH:server/subscriber/MediaSubscriber.ts` Z. 79–126). belegt
- Nachträgliches Ändern: `PUT /request/{id}` mit `seasons` (nur `PENDING`, siehe Abschnitt 8). belegt
- **TVDB-ID:** Fehlt sie bei TMDB und in `media.tvdbId`, werden beim Senden an Sonarr **Media und Request
  gelöscht** und `TVDB ID not found` geworfen (`MediaRequestSubscriber.ts` Z. 618–625). `Media.tvdbId`
  ist `unique` (`Media.ts` Z. 127–129). belegt

---

## 5. Status-Enums – belegt

`GH:server/constants/media.ts` Z. 1–22 (v3.5.2):

```ts
export enum MediaRequestStatus { PENDING = 1, APPROVED, DECLINED, FAILED, COMPLETED }
// = 1, 2, 3, 4, 5
export enum MediaStatus { UNKNOWN = 1, PENDING, PROCESSING, PARTIALLY_AVAILABLE, AVAILABLE, BLOCKLISTED, DELETED }
// = 1, 2, 3, 4, 5, 6, 7
```

v2.7.3: identisch, nur `BLACKLISTED` statt `BLOCKLISTED` (Wert 6). belegt

**Spec-Fehler:** `SPEC` Z. 1266 beschreibt `MediaRequest.status` nur mit «1 = PENDING APPROVAL, 2 =
APPROVED, 3 = DECLINED» (4 und 5 fehlen); Z. 1317 beschreibt `MediaInfo.status` mit «6 = `DELETED`» –
**falsch**, im Code ist 6 = `BLOCKLISTED`, 7 = `DELETED`. belegt

Übergänge (belegt, `MediaRequestSubscriber.ts`):
- Request `APPROVED` → Media `PROCESSING` (wenn nicht schon `AVAILABLE`/`PARTIALLY_AVAILABLE`/`PROCESSING`), Z. 902–908.
- Request `DECLINED` → Media `UNKNOWN` (wenn nicht `DELETED`), Z. 914–917.
- Senden an Radarr/Sonarr scheitert → Request `FAILED` + Benachrichtigung `MEDIA_FAILED`, Z. 432–500, 804–873.
- Media `AVAILABLE`/`DELETED` → zugehörige `APPROVED`/`FAILED`-Requests → `COMPLETED` (`MediaSubscriber.ts` Z. 52–136).

---

## 6. Abfragen

### 6.1 `GET /api/v1/request` – belegt (`request.ts` Z. 38–311)

- Paginierung: `take` (Standard 10), `skip`. Antwort `{pageInfo:{pages,pageSize,results,page}, results, serviceErrors}`.
- `filter`: `approved`/`processing` → `APPROVED`; `pending`; `unavailable` (PENDING+APPROVED, Media nicht AVAILABLE); `failed`; `completed`/`available`/`deleted` → `COMPLETED` (mit Media-Filter AVAILABLE bzw. DELETED); Standard alle.
- Media mit Status `BLOCKLISTED` werden **nie** gelistet (Standard-Mediafilter ohne 6, Z. 101–109).
- `sort`: `modified` → `request.updatedAt`, `added` → `createdAt`, sonst `request.id` (Spec sagt Standard `added` – Abweichung). `sortDirection` `asc`/`desc` (Standard DESC).
- `requestedBy=<userId>`: nur mit `MANAGE_REQUESTS` oder `REQUEST_VIEW`; sonst 403 für fremde IDs, eigene Requests automatisch.
- `mediaType`: `movie`/`tv`/`all`.
- Bei **jedem** Aufruf fragt Seerr alle Radarr/Sonarr nach Qualitätsprofilen (Z. 196–224) – teuer, abgeleitet.

### 6.2 Weitere – belegt

- `GET /request/{id}`: eigene oder mit `MANAGE_REQUESTS`/`REQUEST_VIEW`; sonst 403; unbekannt 404 (Z. 438–468).
- `GET /request/count`: **keine** Berechtigungsprüfung über Anmeldung hinaus; liefert globale Zahlen `{total, movie, tv, pending, approved, declined, processing, available, completed}` (Z. 348–436).
- `GET /movie/{tmdbId}` und `GET /tv/{tmdbId}`: TMDB-Daten plus `mediaInfo` aus `Media.getMedia()` mit `requests` und `issues` (eager: `seasons`) (`Media.ts` Z. 98–115, `movie.ts` Z. 25, `tv.ts` Z. 33). Jeder Aufruf geht an TMDB.
- `GET /media`: `take` (Standard 20), `skip`, `filter` = `available`/`partial`/`allavailable`/`processing`/`pending`, `sort` = `modified` (updatedAt DESC) oder `mediaAdded` (`media.ts` Z. 24–96). Liefert Media-Zeilen **ohne TMDB-Aufruf**.
- `GET /search?query=…`: TMDB-Suche, Treffer angereichert mit `mediaInfo` via `getRelatedMedia` (`search.ts` Z. 36).

### 6.3 Was es NICHT gibt – abgeleitet aus dem gesamten Routing

- **Kein Bündelabruf nach Liste von TMDB-IDs.** `getRelatedMedia` kann es intern (`Media.ts` Z. 37–60), ist aber nur über Such-/Discover-Routen erreichbar. Einziger Bündelweg: `GET /media` seitenweise durchlaufen (alle Media-Einträge, `tmdbId`, `status`, `status4k`, `seasons`) oder `GET /request` seitenweise.
- **Kein Änderungs-Feed** («seit Zeitpunkt X»): kein `since`/`updatedAfter`-Parameter. Ersatz: `sort=modified` (absteigend) und abbrechen, sobald `updatedAt` älter als der letzte Abgleich ist. Unbekannte Query-Parameter würden vom Validator abgelehnt (Abschnitt 2.5).
- Kein Filter nach `tmdbId` an `/request` oder `/media`.

---

## 7. Benutzer

### 7.1 Felder – belegt (`GH:server/entity/User.ts` Z. 56–161)

`id, email, plexUsername, jellyfinUsername, username, userType, plexId, jellyfinUserId, avatar,
requestCount, movieQuotaLimit, movieQuotaDays, tvQuotaLimit, tvQuotaDays, permissions, createdAt,
updatedAt, settings, displayName` (berechnet), `warnings`. Gefiltert für Fremde: `email, plexId,
password, …, settings` (Z. 43–50; `GET /user/{id}` zeigt sie nur Eigentümer oder `MANAGE_USERS`,
`user/index.ts` Z. 412–426).
`UserType`: `PLEX = 1, LOCAL = 2, JELLYFIN = 3, EMBY = 4` (`GH:server/constants/user.ts`).

### 7.2 Endpunkte – belegt (`GH:server/routes/user/index.ts`)

- `GET /user`: `take`, `skip`, `q` (sucht in username, email, plexUsername, jellyfinUsername), `sort` (`created, updated, requests, displayname, usertype, role`), `sortDirection`, `includeIds` (Z. 36–166; Spec Z. 4298ff).
- `POST /user` (MANAGE_USERS), Z. 170–226:
  - `email` = `body.email || body.username`; Duplikat → `409 "User already exists with submitted email."`.
  - Ohne `password`: **E-Mail-Benachrichtigung muss aktiv sein**, sonst 500 `Email notifications must be enabled`; mit `password` geht es ohne SMTP.
  - `permissions` aus dem Body wird **ignoriert**; neue Benutzer bekommen `settings.main.defaultPermissions` (Standard `Permission.REQUEST` = 32, `settings/index.ts` Z. 415). Die Spec führt `permissions` als Feld (Abweichung). Rechte danach per `POST /user/{id}/settings/permissions` (MANAGE_USERS, `usersettings.ts` Z. 763–769) oder `PUT /user/{id}`.
  - Keine Formatprüfung der E-Mail im Code (Spec: `type: string` ohne `format`). Ob der Validator ein Format prüft: nicht verifiziert.
  - `userType` immer `LOCAL`.
- `PUT /user/{id}` (MANAGE_USERS): `username`, `permissions` (Z. 567–616). Admin-Rechte vergibt nur Benutzer 1 (`canMakePermissionsChange`, Z. 510–514).
- `POST /user/{id}/settings/main`: u. a. `username, email, locale, …, movieQuotaLimit, movieQuotaDays, tvQuotaLimit, tvQuotaDays` (Quota nur mit passenden Rechten, `usersettings.ts` Z. 77–118).
- `DELETE /user/{id}` (MANAGE_USERS): Benutzer 1 und Admins nicht löschbar (Z. 619–680). **Requests werden mitgelöscht** (`MediaRequest.requestedBy` mit `onDelete: 'CASCADE'`, `MediaRequest.ts` Z. 532–537).
- `POST /user/import-from-jellyfin` `{jellyfinUserIds: string[]}` und `/import-from-plex` `{plexIds}`: nur mit angebundenem Jellyfin/Plex (nutzen den Admin-Zugang von Benutzer 1, Z. 683–830).
- `GET /user/jellyfin/{jellyfinUserId}`: Seerr-Benutzer zu Jellyfin-ID (neu in 3.x).
- `GET /user/{id}/requests`: `take` (20), `skip`; eigene oder MANAGE_REQUESTS/REQUEST_VIEW; Sortierung fest `id DESC` (Z. 454–508).
- `GET /user/{id}/quota`: eigene oder **MANAGE_USERS und MANAGE_REQUESTS**; Antwort `{movie:{days,limit,used,remaining,restricted}, tv:{…}}` (Z. 832–862, `userInterfaces.ts` Z. 14–25).

### 7.3 Berechtigungs-Bitmaske – belegt (`GH:server/lib/permissions.ts` Z. 1–32)

```
NONE 0 · ADMIN 2 · MANAGE_SETTINGS 4 · MANAGE_USERS 8 · MANAGE_REQUESTS 16 · REQUEST 32 · VOTE 64
AUTO_APPROVE 128 · AUTO_APPROVE_MOVIE 256 · AUTO_APPROVE_TV 512 · REQUEST_4K 1024 · REQUEST_4K_MOVIE 2048
REQUEST_4K_TV 4096 · REQUEST_ADVANCED 8192 · REQUEST_VIEW 16384 · AUTO_APPROVE_4K 32768
AUTO_APPROVE_4K_MOVIE 65536 · AUTO_APPROVE_4K_TV 131072 · REQUEST_MOVIE 262144 · REQUEST_TV 524288
MANAGE_ISSUES 1048576 · VIEW_ISSUES 2097152 · CREATE_ISSUES 4194304 · AUTO_REQUEST 8388608
AUTO_REQUEST_MOVIE 16777216 · AUTO_REQUEST_TV 33554432 · RECENT_VIEW 67108864 · WATCHLIST_VIEW 134217728
MANAGE_BLOCKLIST 268435456 · VIEW_BLOCKLIST 1073741824
```

(536870912 ist unbelegt.) `ADMIN` besteht jede Prüfung (Z. 60–62, 73).

### 7.4 Feld für fremde Kennung / Notiz – belegt: **gibt es nicht**

- `User`: kein freies Feld für eine externe Kennung. `jellyfinUserId`/`plexId` sind an die Anmeldung gebunden. `UserSettings` (`GH:server/entity/UserSettings.ts` Z. 37–146) enthält nur Locale, Regionen, PGP-Schlüssel, Discord-IDs, Pushbullet/Pushover/Telegram-Kennungen, Watchlist-Sync, Benachrichtigungsarten. Missbrauch etwa von `telegramChatId` als Notizfeld wäre technisch möglich, aber zweckfremd (nicht empfohlen).
- `MediaRequest`: kein Kommentar-/Notizfeld (`MediaRequest.ts` Z. 518–618). `tags` sind numerische Radarr/Sonarr-Tag-IDs und werden an Radarr/Sonarr weitergegeben.
- Issues/IssueComments haben Freitext, sind aber Störungsmeldungen zu Medien, kein Request-Anhang.
- Bleibt: Slouchcat merkt sich die Zuordnung (Seerr-User-ID, Request-ID) selbst.

---

## 8. Freigabe, Ablehnung, Wiederholung, Löschen, Ändern – belegt (`request.ts`)

- `POST /request/{id}/approve|decline` (MANAGE_REQUESTS), Z. 794–848. Andere Werte → 400 `Status must be approve or decline.` Nur `PENDING` → sonst 409 `Only pending requests can be approved or declined.` (neu in 3.x). **Kein Ablehnungsgrund**: Body wird nicht gelesen, kein Feld `declineReason` im Entity.
- `POST /request/{id}/retry` (MANAGE_REQUESTS), Z. 754–792: nur `FAILED` → sonst 409 `Only failed requests can be retried.`; setzt `APPROVED` und sendet erneut an *arr.
- `DELETE /request/{id}`, Z. 719–752: MANAGE_REQUESTS, oder eigener Request im Status `PENDING`; sonst **401** (nicht 403) `You do not have permission to delete this request.`; Erfolg 204; unbekannt 404.
- `PUT /request/{id}`, Z. 470–717: nur `PENDING` (409 sonst). `userId` umhängen braucht MANAGE_USERS und MANAGE_REQUESTS; Quota des neuen Besitzers wird voll geprüft. Film: `serverId, profileId, rootFolder, tags`. Serie: zusätzlich `languageProfileId`, Pflicht `seasons` (leer → 400 `Missing seasons. If you want to cancel a series request, use the DELETE method.`).
- `POST /media/{mediaId}/{status}` und `DELETE /media/{mediaId}` siehe Abschnitt 11.

---

## 9. Webhook-Benachrichtigung

### 9.1 Ereignistypen – belegt (`GH:server/lib/notifications/index.ts` Z. 6–20)

```
NONE 0 · MEDIA_PENDING 2 · MEDIA_APPROVED 4 · MEDIA_AVAILABLE 8 · MEDIA_FAILED 16 · TEST_NOTIFICATION 32
MEDIA_DECLINED 64 · MEDIA_AUTO_APPROVED 128 · ISSUE_CREATED 256 · ISSUE_COMMENT 512 · ISSUE_RESOLVED 1024
ISSUE_REOPENED 2048 · MEDIA_AUTO_REQUESTED 4096
```

Die Auswahl im Webhook ist die Bitmaske `types`. `MEDIA_AUTO_REQUESTED` wird mit `notifySystem = false`
gesendet und erreicht den Webhook daher **nie** (`MediaRequest.ts` Z. 760–766, `webhook.ts` Z. 173–177).
Bei einer Freigabe für schon verfügbare Medien kommt `MEDIA_AVAILABLE` statt `MEDIA_APPROVED`
(`MediaRequest.ts` Z. 677–691). belegt

### 9.2 Variablen – belegt (`GH:server/lib/notifications/agents/webhook.ts` Z. 17–64, vollständig)

```
notification_type event subject message image
notifyuser_username notifyuser_email notifyuser_avatar notifyuser_settings_discordIds notifyuser_settings_telegramChatId
media_imdbid media_tmdbid media_tvdbid media_type media_jellyfinMediaId media_status media_status4k
media_plexRatingKey media_plexRatingKey4k
request_id requestedBy_jellyfinUserId requestedBy_username requestedBy_email requestedBy_avatar
requestedBy_settings_discordIds requestedBy_settings_telegramChatId
issue_id issue_type issue_status reportedBy_username reportedBy_email reportedBy_avatar
reportedBy_settings_discordIds reportedBy_settings_telegramChatId
comment_message commentedBy_username commentedBy_email commentedBy_avatar
commentedBy_settings_discordIds commentedBy_settings_telegramChatId
```

Sonderschlüssel (nur als JSON-Schlüssel): `{{media}}`, `{{request}}`, `{{issue}}`, `{{comment}}`, `{{extra}}`
(Z. 86–122). `{{extra}}` liefert das `extra`-Array, bei Serien `[{"name":"Requested Seasons","value":"1, 2"}]`
(`MediaRequest.ts` Z. 814–821).

Was es **nicht** gibt (belegt durch die vollständige Liste):
- **keine Seerr-Benutzer-ID** des Anfragenden (`requestedBy_id` fehlt) – nur Anzeigename, E-Mail, Avatar, Jellyfin-User-ID;
- **kein Request-Status** (`request_status` fehlt) – der Status ergibt sich nur aus `notification_type`;
- keine Staffelstatus-Liste, nur der Text in `extra`.
- `requestedBy_username` ist in Wahrheit `displayName` (Z. 42).
- `{{request}}`/`{{media}}` sind **keine Objekt-Dumps**: Der Code übernimmt die vom Admin geschriebene Untervorlage bzw. `null` (Z. 98–105). Die Doku («The relevant request object», `DOCS:using-seerr/notifications/webhook.md` Z. 91) ist hier irreführend.
- Ersetzung in JSON-Werten mit `String.replace` ohne `g`: je Variable nur das **erste** Vorkommen pro Zeichenkette (Z. 127). abgeleitet
- Die mitgelieferte Standardvorlage nutzt noch `requestedBy_settings_discordId` (Singular), das nicht mehr ersetzt wird (`settings/index.ts` Z. 544, Base64). abgeleitet

### 9.3 Zustellung – belegt

- `POST` an `webhookUrl` mit JSON-Body; optional Header `Authorization` (`authHeader`) und `customHeaders[]` (Z. 202–227).
- Optional (experimentell) Variablen in der URL (`supportVariables`, URL-codiert), Z. 186–200.
- Kein Wiederholen bei Fehler: nur Logeintrag, `return false` (Z. 233–243). Keine Signatur (HMAC).

### 9.4 Einrichten per API – belegt

`GET`/`POST /api/v1/settings/notifications/webhook` und `POST …/webhook/test`
(`GH:server/routes/settings/notifications.ts` Z. 276–330), unter `/settings` also **nur mit ADMIN**
(`routes/index.ts` Z. 156). Body:
`{enabled, embedPoster, types, options:{webhookUrl, jsonPayload (JSON als String), authHeader, customHeaders, supportVariables}}`.
**Nur ein Webhook-Ziel**: `settings.notifications.agents.webhook` ist ein einzelnes Objekt; `POST`
**überschreibt** es komplett. Nutzt Marco den Webhook schon für etwas anderes, verdrängt Slouchcat es (abgeleitet).

---

## 10. Sonstiges

- `GET /status`: **ohne Anmeldung** (`security: []`, `SPEC` Z. 2177–2182); Antwort `{version, commitTag, updateAvailable?, commitsBehind?, restartRequired}` (`routes/index.ts` Z. 50–102). belegt
- `GET /settings/public`: ohne Anmeldung (Z. 113–123); enthält u. a. `applicationTitle, applicationUrl, localLogin, mediaServerType, partialRequestsEnabled, enableSpecialEpisodes, movie4kEnabled, series4kEnabled, emailEnabled, userEmailRequired, locale`. belegt
- `GET /settings/main` (inkl. `apiKey`, `applicationUrl`, `defaultPermissions`): nur ADMIN. belegt
- `GET /service/radarr` und `/service/sonarr`: für **jeden angemeldeten Benutzer**; liefert `id, name, is4k, isDefault, activeDirectory, activeProfileId, activeTags` ohne API-Schlüssel (`service.ts` Z. 14–30). belegt
- Rate-Limits: nur auf `GET /settings/logs` (50/min, `settings/index.ts` Z. 602). Sonst keine. belegt
- Radarr/Sonarr nicht eingerichtet: Request wird angelegt (und ggf. `APPROVED`), Log `No Radarr server configured, skipping request processing`; Media bleibt `PROCESSING` (`MediaRequestSubscriber.ts` Z. 195–205, 902–908). belegt
- Blocklist (`/blocklist`, MANAGE_BLOCKLIST / VIEW_BLOCKLIST): blockiert Anfragen mit 403 (Abschnitt 3.4). belegt
- Watchlist-Autorequest (`AUTO_REQUEST*`): **nur für Plex-Benutzer** mit `plexToken` (`GH:server/lib/watchlistsync.ts` Z. 24–64). Für Jellyfin/lokale Benutzer gibt es keine automatische Anfrage aus einer Watchlist. belegt
- Einrichtung: Der Einrichtungsassistent kennt nur Plex, Jellyfin und Emby (`src/components/Setup/SetupLogin.tsx`); `mediaServerType` startet als `NOT_CONFIGURED` (`settings/index.ts` Z. 433). Ob eine Instanz ohne Medienserver dauerhaft betrieben werden kann: nicht verifiziert.
- Jobs per API auslösen: `POST /settings/jobs/{jobId}/run` (ADMIN, `SPEC` Z. 3114). belegt

---

## 11. Verfügbarkeit

### 11.1 Wer setzt `AVAILABLE`? – belegt, Annahme im Auftrag ist falsch

Nicht nur der Medienserver-Scan. **Auch der Radarr- und der Sonarr-Scan setzen `AVAILABLE`**, sobald die
Datei bzw. alle Folgen vorhanden sind:

- Radarr: `const processing = !radarrMovie.hasFile && radarrMovie.monitored;` und `hasFile` werden an
  `processMovie` übergeben (`GH:server/lib/scanners/radarr/index.ts` Z. 164, 174–182). Dort:
  `!processing && hasFile ? MediaStatus.AVAILABLE : …` (`baseScanner.ts` Z. 152–162).
- Sonarr: `episodes = totalAvailableEpisodes`, `totalEpisodes = statistics.totalEpisodeCount`
  (`sonarr/index.ts` Z. 248–251) → Staffel `AVAILABLE` bei vollständig, `PARTIALLY_AVAILABLE` bei teilweise
  (`baseScanner.ts` Z. 404–414).
- Gleiches Verhalten schon in v2.7.3 (`radarr/index.ts`: `processing: !radarrMovie.hasFile`).
- Radarr- und Sonarr-Scan laufen unabhängig vom Medienserver-Typ (`server/job/schedule.ts` Z. 156, 171),
  Standard täglich 04:00 bzw. 04:30 (`settings/index.ts` Z. 590–595), nur für Server mit «Enable Scan»
  (`syncEnabled`, `radarr/index.ts` Z. 109).

**Folge (abgeleitet):** Liegen die Titel nur in Slouchcat, aber werden sie über Radarr/Sonarr geladen,
wird der Status nach dem nächsten *arr-Scan `AVAILABLE` – ohne Jellyfin/Plex. Dauerhaft `PROCESSING`
bleibt er nur, wenn Radarr/Sonarr den Titel nicht mit Datei führen (z. B. manuell abgelegte Dateien oder
`syncEnabled` aus).

### 11.2 Availability-Sync – belegt

`GH:server/lib/availabilitySync.ts`: Der tägliche Job (05:00) bricht ohne Plex/Jellyfin/Emby-Admin ab
(`default: logger.error('An admin is not configured.'); … return;`, Z. 85–127). Mit Medienserver prüft er
`AVAILABLE`-Titel gegen Medienserver **oder** Radarr/Sonarr und stuft herab, wenn beide den Titel nicht
haben (Z. 152–220). Ein von aussen gesetztes `AVAILABLE` ohne Datei in Radarr und ohne Eintrag im
Medienserver kann dieser Job also wieder zurücknehmen (abgeleitet).

### 11.3 Verfügbarkeit per API setzen – belegt

`POST /api/v1/media/{mediaId}/{status}` (`GH:server/routes/media.ts` Z. 98–168), Berechtigung
**`MANAGE_REQUESTS`**. `mediaId` ist die **interne Seerr-Media-ID** (`mediaInfo.id`), **nicht** die TMDB-ID.
`status`: `available | partial | processing | pending | unknown`. Body: `is4k` (wird als
`String(req.body.is4k) === 'true'` gelesen), bei Serien optional `seasons: [{seasonNumber}]` – nur bei
`available` werden diese Staffeln auf `AVAILABLE` gesetzt (fehlende Staffelzeilen werden angelegt).
`partial` bei Filmen → 400 `Only series can be set to be partially available`. Unbekannte Media-ID → 404
`Media does not exist.`

Abweichungen: Die Spec listet zusätzlich `deleted` (`SPEC` Z. 7403); im Code gibt es dafür **keinen**
`case` – der Aufruf speichert unverändert und meldet 200 (abgeleitet). `seasons` fehlt in der Spec.

Voraussetzung: Es muss schon eine Media-Zeile geben (entsteht beim ersten Request oder Scan). Für einen
Titel ohne Zeile gibt es keinen Weg, per API eine anzulegen, ausser über einen Request (abgeleitet).
Setzen auf `AVAILABLE` schliesst zugehörige Requests als `COMPLETED` ab und löst `MEDIA_AVAILABLE` aus
(`MediaSubscriber.ts` Z. 52–136, `MediaRequestSubscriber.ts` Z. 60–100). belegt

---

## 12. Bewertung der Zuordnungsvarianten

Grundlage: Seerr-Benutzer lassen sich per API anlegen (`POST /user` mit `password`, ohne SMTP), Rechte per
`POST /user/{id}/settings/permissions` setzen, finden per `GET /user?q=<email>`. Eine fremde Kennung lässt
sich nicht am Benutzer speichern; Slouchcat braucht eine eigene Zuordnungstabelle (Seerr-User-ID ↔
Slouchcat-Konto/Profil). Handeln im Namen: `X-API-Key` + `X-API-User: <seerrUserId>` (empfohlen, echtes
Benutzerverhalten) oder Admin-Schlüssel + `userId` im Body (dann immer auto-genehmigt).

**(A) Ein Seerr-Benutzer je Slouchcat-Konto**
- Möglich: anlegen, Rechte, Quota je Konto, `X-API-User`, `GET /request?requestedBy=`, Webhook über `requestedBy_email`.
- Passt zur Quota- und Duplikatlogik (global je Titel, nicht je Benutzer).
- Profile innerhalb des Kontos sind in Seerr nicht unterscheidbar; die Zuordnung Wunsch→Profil hält Slouchcat selbst (Request-ID beim Anlegen merken).
- Wenn Konten schon als Jellyfin-/Plex-Benutzer in Seerr existieren: Zuordnung per `GET /user?q=` oder `GET /user/jellyfin/{id}` statt neu anlegen. Gefahr von Doppelkonten bei gleicher E-Mail (409).

**(B) Ein Seerr-Benutzer je Slouchcat-Profil**
- Technisch möglich, aber: je Profil eine eindeutige E-Mail nötig (409 bei Gleichheit) → künstliche Adressen; viele lokale Benutzer ohne echten Login; Quota je Profil statt je Haushalt.
- Ein zweites Profil kann denselben Titel nicht zusätzlich anfragen (409 / Staffeln still gefiltert) – der Wunsch hängt immer am ersten Profil.
- Profil löschen → Seerr-Benutzer löschen **löscht dessen Requests mit** (CASCADE). Umhängen vorher nur für `PENDING` möglich (`PUT /request/{id}` mit `userId`).
- Webhook liefert keine Seerr-User-ID; Zuordnung nur über E-Mail/Anzeigename oder über die Request-ID.

**(C) Hybrid** (Konto = Seerr-Benutzer, Profil nur in Slouchcat)
- Möglich und am wenigsten reibungsvoll: Seerr kennt nur Konten; Slouchcat speichert je Request-ID das auslösende Profil und gleicht Status über `GET /request?sort=modified` oder Webhook (`request_id`) ab.
- Optional ein technischer Sammelbenutzer für Konten ohne eigenes Seerr-Konto; dann aber Quota und Benachrichtigungen gesammelt.

Nicht möglich in allen Varianten: Profil-/Fremdkennung an Benutzer oder Request speichern; Ablehnungsgrund
lesen; Status vieler TMDB-IDs in einem Aufruf; Änderungs-Feed mit `since`; mehrere Webhook-Ziele;
mehrere Benutzer an einem Wunsch.
