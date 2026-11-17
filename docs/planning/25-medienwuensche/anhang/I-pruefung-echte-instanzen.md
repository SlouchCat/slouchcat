# Konzept 25 · Anhang I: Prüfung an den echten Instanzen (17. November 2026)

> **Kurz gesagt.** Vor dem Bau der Benutzerpflege wurden die Annahmen aus Anhang D und Anhang H an Marcos Seerr,
> Radarr und Sonarr geprüft: lesend, und mit seiner Erlaubnis (Anhang H 11.1, B-3 und B-4) auch schreibend mit zwei
> Testbenutzern und einem Test-Tag, die danach wieder gelöscht wurden. Es wurde kein Wunsch angelegt. Dieser Anhang
> hält fest, was die Instanzen wirklich antworten. Wo er Anhang D oder H widerspricht, gilt dieser Anhang.

Geprüft mit zwei Skripten ausserhalb des Repos. Ausgegeben wurden nur Statuscodes, Feldnamen und Zahlen, keine
Schlüssel, Adressen, Namen oder E-Mail-Adressen. Stand der Instanzen: Seerr 3.5.1, Radarr 6.4.4.10685, Sonarr
4.0.20.3014.

## 1. Seerr: Verbindung und Anmeldung

| Nr. | Befund | Folge für den Bau |
|---|---|---|
| I-1 | `GET /status` antwortet ohne Schlüssel mit `version`, `commitTag`, `updateAvailable`, `commitsBehind`, `restartRequired` | Versionsprüfung wie geplant |
| I-2 | Falscher Schlüssel: **403** `{"status": 403, "error": "You do not have permission to access this endpoint"}`. Kein Schlüssel: **401** `{"message": "cookie 'connect.sid' required", "errors": […]}` | Der Fehlertext steht im Feld `error`, nicht in `message`; der Client liest beide Felder (so gebaut) |
| I-3 | `X-API-User` mit unbekannter Benutzer-ID: dieselbe 403-Antwort wie bei falschem Schlüssel | Die beiden Fälle unterscheidet nur ein zweiter Aufruf als Administrator (`GET /auth/me` ohne `X-API-User`): gelingt er, ist der Benutzer weg |
| I-4 | `X-API-User` mit der ID eines Benutzers: `GET /auth/me` liefert diesen Benutzer | «Als dieser Benutzer handeln» (F-3) bestätigt |
| I-5 | Der Benutzer zum Schlüssel ist Benutzer 1 mit `permissions = 2` (ADMIN) | wie angenommen |
| I-6 | `GET /settings/public`: `localLogin`, `mediaServerLogin`, `mediaServerType`, `initialized`, `partialRequestsEnabled`, `enableSpecialEpisodes`, `locale` u. a. | Prüfpunkt «Lokale Anmeldung in Seerr» liest `localLogin` |
| I-7 | `GET /settings/main` (nur Administrator) nennt `defaultPermissions` und `defaultQuotas {movie {quotaLimit, quotaDays}, tv {…}}`. Ein Feld `csrfProtection` gibt es dort in 3.5.1 nicht | CSRF nicht aus dieser Antwort lesen |
| I-8 | `GET /settings/metadatas`: `{"tv": "tmdb", "anime": "tmdb"}` | Feldnamen bestätigt; Prüfpunkt «Staffeln aus TVDB» zeigt hier heute den Hinweis (F-29) |
| I-9 | `GET /settings/radarr` und `/settings/sonarr`: je eine Liste mit `id`, `name`, `hostname`, `port`, `useSsl`, `baseUrl`, `apiKey`, `isDefault`, `is4k`, `tagRequests`, `tags`, `syncEnabled`, `preventSearch`, `activeProfileId`, `activeDirectory`; bei Sonarr dazu die Anime-Felder | Zugang aus Seerr lesen (F-30) geht; die Server-ID kann `0` sein, also nie auf «ungleich 0» prüfen |

## 2. Seerr: Benutzer

| Nr. | Befund | Folge für den Bau |
|---|---|---|
| I-10 | `mediaServerType` der Instanz ist 3 (Emby). **Alle bestehenden Benutzer haben `userType = 4` (Emby)** und melden sich über Emby an; einen lokalen Benutzer gab es vor dem Test nicht | Für jeden bestehenden Benutzer gilt N-91/N-6: verknüpfen ja, Passwort nie. «Dasselbe Login» gibt es auf dieser Instanz nur für Benutzer, die Slouchcat neu eröffnet |
| I-11 | `GET /user?take=&skip=`: `pageInfo {pages, pageSize, results, page}` und `results[]` mit `id`, `email`, `username`, `displayName`, `userType`, `permissions`, `requestCount`, `movieQuotaLimit`, `movieQuotaDays`, `tvQuotaLimit`, `tvQuotaDays`, `jellyfinUsername`, `jellyfinUserId`, `plexUsername`, `createdAt`, `updatedAt` | Emby-Benutzer tragen ihre Kennung in den `jellyfin*`-Feldern; die Art sagt nur `userType`. Die IDs haben Lücken |
| I-12 | `POST /user {email, username, password}` → **201**. Die Antwort enthält **keine E-Mail** und kein Passwort; `userType = 2` (lokal); `permissions` ist der Wert aus `defaultPermissions` der Instanz, ein mitgeschicktes `permissions` wird übergangen | Rechte nach dem Anlegen eigens setzen; E-Mail nicht aus der Antwort lesen |
| I-13 | Auf dieser Instanz ist `defaultPermissions = 1048736` = `REQUEST` (32) + `AUTO_APPROVE` (128) + `MANAGE_ISSUES` (1048576). Jeder neue Benutzer bekommt also die automatische Freigabe | Schreibt Slouchcat nur `REQUEST`, warten Wünsche eröffneter Benutzer auf eine Freigabe, anders als bei allen bestehenden Benutzern. **Frage an Marco** (Abschnitt 6) |
| I-14 | Dieselbe E-Mail in anderer Gross-/Kleinschreibung: **409** `{"message": "User already exists with submitted email.", "errors": ["USER_EXISTS"]}` | wie angenommen; E-Mail immer kleingeschrieben senden und vergleichen |
| I-15 | `GET /user/{id}/settings/main`: `username`, `email`, vier Limit-Felder und `globalMovieQuotaLimit`, `globalMovieQuotaDays`, `globalTvQuotaLimit`, `globalTvQuotaDays` | – |
| I-16 | `POST /user/{id}/settings/main` mit allen gelesenen Feldern (auch den `global*`-Feldern) → 200. Die Antwort nennt `username`, `email`, `locale`, `discoverRegion`, `streamingRegion`, `originalLanguage`, `watchlistSyncMovies`, `watchlistSyncTv`, aber **nicht die Limit-Felder** | Limit nach dem Schreiben über `GET /user/{id}/quota` oder `GET /user/{id}` prüfen |
| I-17 | Fehlen `username` und `email` im Body, bleiben beide in 3.5.1 erhalten (die Antwort zeigt `username: null`, gespeichert bleibt der alte Wert) | Trotzdem immer lesen und vollständig zurückschreiben (ältere Fassungen leeren die Felder, Anhang H 3.1) |
| I-18 | E-Mail eines anderen Benutzers in `settings/main`: **400** `{"message": "INVALID_EMAIL"}`, der Benutzer bleibt unverändert | eigene Fehlerklasse im Client; Fall N-95 (Hinweis an die Verwaltung) |
| I-19 | `POST /user/{id}/settings/permissions {permissions}` → 200 `{permissions}` | – |
| I-20 | `POST /user/{id}/settings/password {newPassword}` als Administrator → **204** ohne Body; danach gelingt `POST /auth/local` mit dem neuen Passwort (200), mit dem alten 403 `{"message": "Access denied."}`. Zu kurz (unter 8 Zeichen): 400 `{"message": "Password must be at least 8 characters."}`. `GET …/settings/password` → `{"hasPassword": true}` | wie angenommen. Die Abweisung für Benutzer 1 und andere Administratoren wurde **nicht** geprüft (kein Versuch an echten Benutzern); sie bleibt Annahme aus dem Quellcode |
| I-21 | Nach einer E-Mail-Änderung über `settings/main` gelingt die Anmeldung mit der neuen Adresse | E-Mail-Änderung an Seerr weitergeben wie geplant |
| I-22 | `DELETE /user/{id}` → 200, danach `GET /user/{id}` → 404 `{"message": "User not found."}` | – |
| I-23 | Ein Benutzer mit `permissions = 0` kann über `X-API-User` lesen: `GET /request` (nur eigene), `/discover/*`, `/search`, `/movie/{id}`, `/user/{id}/quota`, sogar `GET /user`; `GET /settings/radarr` → 403 | Entdecken und Suche gehen auch für Konten ohne Wunschrecht technisch; die Sperre (F-9) setzt Slouchcat selbst durch |

## 3. Seerr: Limit

| Nr. | Befund | Folge für den Bau |
|---|---|---|
| I-30 | `GET /user/{id}/quota`: `{movie {days, limit, used, remaining, restricted}, tv {…}}` | Form bestätigt |
| I-31 | Limit **0** heisst unbegrenzt: `limit: 0`, `restricted: false`, das Feld `remaining` **fehlt** | «Unbegrenzt» = 0 schreiben; `remaining` ist optional |
| I-32 | Limit **null** heisst «Vorgabe der Instanz»: die Antwort zeigt dann die Werte aus `defaultQuotas` | «Standard» in Slouchcat schreibt deshalb immer die eigenen Zahlen, nie null |
| I-33 | Die Instanz hat eine eigene Vorgabe (`defaultQuotas`) und einzelne Benutzer mit eigenen Limits, einer davon mit 0 (unbegrenzt) | Überschreibt Slouchcat beim Verknüpfen das Limit, gehen diese Einstellungen verloren. **Frage an Marco** (Abschnitt 6) |
| I-34 | Ein Benutzer mit `MANAGE_USERS` zeigt `limit: 0` ohne `remaining`, auch wenn Limit-Felder gespeichert sind; Seerr nimmt das Schreiben der Felder trotzdem an (200) | «Kein Limit» anzeigen und nichts schreiben, wie geplant |
| I-35 | Der Benutzer selbst kann seinen Stand lesen (`X-API-User`) | – |

## 4. Seerr: Titel, Suche, Entdecken, Anfragen

| Nr. | Befund | Folge für den Bau |
|---|---|---|
| I-40 | `GET /tv/{tmdbId}`: `seasons[] {id, seasonNumber, episodeCount, airDate, name, overview, posterPath}`, `externalIds {tvdbId (Zahl), imdbId, …}`, `contentRatings.results[] {iso_3166_1, rating}`, `status`, `lastEpisodeToAir`, `nextEpisodeToAir`, `mediaInfo` | Staffel 0 («Extras») steht in der Liste und wird nicht angeboten |
| I-41 | `GET /movie/{tmdbId}`: `externalIds {imdbId, …}` ohne TVDB; `releases.results[] {iso_3166_1, release_dates[] {certification, type, …}}`; `mediaInfo` mit `status`, `requests`, `tmdbId`, `tvdbId` | – |
| I-42 | Suche: `tmdb:603`, `TMDB:603`, `tvdb:121361`, `imdb:tt0133093` finden den Titel; `tmdb: 603` (mit Leerzeichen) findet nichts. `tmdb:603` liefert **mehrere Treffer** (Film, Serie und Person mit dieser ID) | Präfix ohne Leerzeichen bilden; Treffer nach `mediaType` und ID filtern |
| I-43 | Antwort von Suche und Entdecken: `page`, `totalPages`, `totalResults`, `results[]` mit `id`, `mediaType`, `title` oder `name`, `posterPath`, `backdropPath`, `releaseDate` oder `firstAirDate`, `genreIds`, `mediaInfo`; `/discover/movies` und `/discover/tv` zusätzlich `keywords` | – |
| I-44 | `certificationCountry` und `certificationLte` nimmt `/discover/movies` und `/discover/tv` an; `certification.lte` → 400 «Unknown query parameter» | nur die bekannten Parameter senden (Anhang D 2.5) |
| I-45 | `/discover/genreslider/movie|tv` → Liste `{id, name, backdrops}`; `/genres/movie|tv` → Liste `{id, name}` | – |
| I-46 | `GET /request`: `pageInfo` und `results[]` mit `id`, `status`, `type`, `media {id, mediaType, tmdbId, tvdbId, status, …}`, `seasons[]`, `seasonCount`, `requestedBy`, `modifiedBy`, `tags`, `canRemove`, `createdAt`, `updatedAt`; mit `requestedBy=` zusätzlich `serviceErrors {radarr, sonarr}`. `GET /request/count` → `total`, `movie`, `tv`, `pending`, `approved`, `declined`, `processing`, `available`, `completed` | Der Request-Zustand 5 («abgeschlossen») kommt vor und gilt als freigegeben |
| I-47 | Unbekannte Request-ID: 404 `{"message": "Request not found."}` | – |

`POST /request` wurde nicht aufgerufen (kein echter Wunsch). Die Antworten 201, 202, 409 und die 403-Texte beim
Anfordern bleiben Annahme aus dem Quellcode (Anhang D).

## 5. Radarr und Sonarr: Tags und Warteschlange

| Nr. | Befund | Folge für den Bau |
|---|---|---|
| I-50 | `POST /api/v3/tag {label}` → **201** `{id, label}`; dasselbe Label noch einmal → 201 mit **derselben ID** (kein Doppel, kein Fehler) | Anlegen ist wiederholbar; trotzdem zuerst die Liste lesen |
| I-51 | Beide schreiben das Label klein (`SC-Test2` → `sc-test2`) | Namen klein bilden und klein vergleichen |
| I-52 | **Radarr** lässt nur `^[a-z0-9-]+$` zu: Leerzeichen, Umlaute und Unterstrich → 400 `[{"propertyName": "Label", "errorMessage": "Allowed characters a-z, 0-9 and -", …}]`. **Sonarr** nimmt sie an | Profilnamen für den Tag einheitlich umsetzen: klein, Umlaute ausschreiben (ä → ae, ö → oe, ü → ue, ss), alles andere ausser a–z und 0–9 wird `-`, mehrere `-` zu einem, Rand ohne `-`; leer geworden → `profil-<id>` |
| I-53 | `GET /api/v3/tag/detail/{id}`: `movieIds` bzw. `seriesIds` | zeigt, ob ein Tag an Titeln hängt |
| I-54 | `DELETE /api/v3/tag/{id}` → 200 (nur im Test benutzt; Slouchcat löscht nie einen Tag) | – |
| I-55 | In beiden gibt es schon Tags der Form `<Zahl>-<Name>`: Seerr hängt sie selbst an (`tagRequests` ist an) | Der Profil-Tag kommt dazu, er ersetzt nichts |
| I-56 | Radarr-Warteschlange `GET /api/v3/queue`: `page`, `pageSize`, `totalRecords`, `records[]` mit `movieId`, `size`, `sizeleft`, `timeleft` (`hh:mm:ss`), `estimatedCompletionTime`, `status`, `trackedDownloadState` (gesehen: `importBlocked`), `trackedDownloadStatus` (gesehen: `warning`), `errorMessage`, `statusMessages[] {title, messages}`, `downloadId`, `protocol`, `title` | Ein fertig geladener, aber nicht eingelesener Eintrag (`status: completed`, `importBlocked`, `warning`) zählt als «Download verzögert» |
| I-57 | Die Sonarr-Warteschlange war leer | Die Feldnamen bei Sonarr bleiben ungeprüft (wie Anhang H 8.3) |

Angehängt wurde der Test-Tag an keinen Titel; der Aufruf zum Anhängen (`PUT /api/v3/movie/editor`,
`PUT /api/v3/series/editor` mit `applyTags: "add"`) ist damit weiter ungeprüft.

## 6. Fragen an Marco aus dieser Prüfung und seine Antworten

Gestellt und beantwortet am 17. November 2026, nach der Prüfung.

| Nr. | Frage | Marcos Antwort |
|---|---|---|
| I-F1 | Welche Rechte bekommt ein Seerr-Benutzer, den Slouchcat neu eröffnet (I-13)? | **«Wie Seerrs Vorgabe»:** was Seerr jedem neuen Benutzer gibt, auf seiner Instanz also Anfordern mit automatischer Freigabe |
| I-F2 | Sollen die Wünsche des Sammelbenutzers «slouchcat-gaeste» automatisch freigegeben werden? | **«Ja, wie Seerrs Vorgabe»** |
| I-F3 | Was gilt beim Verknüpfen, wenn der Benutzer in Seerr schon ein eigenes Limit hat (I-33)? | **«Limit übernehmen»:** als eigenes Limit des Kontos; Benutzer ohne eigenes Limit bekommen den Slouchcat-Standard |

Daraus folgen diese Regeln. Sie gehen Anhang H und den Papieren 03, 04 und 08 vor, wo diese etwas anderes sagen.

| Nr. | Regel | Von |
|---|---|---|
| N-110 | **Rechte eröffneter Benutzer (I-F1).** Slouchcat lässt einem neu eröffneten Benutzer die Rechte, die Seerr ihm beim Anlegen gibt (`defaultPermissions` der Instanz), und schreibt nicht mehr `REQUEST` darüber. Hat das Konto kein Wunschrecht, setzt Slouchcat die Rechte auf 0; bekommt es das Recht wieder, schreibt Slouchcat den aktuellen Wert von `defaultPermissions` (aus `GET /settings/main`). Verknüpfte Benutzer behalten ihre Rechte immer (wie bisher). | Marco (I-F1), Ablauf Redaktion |
| N-111 | **Sammelbenutzer (I-F2).** «slouchcat-gaeste» behält ebenfalls die Rechte aus `defaultPermissions`. Das ersetzt in N-101 «Recht `REQUEST`, keine automatische Freigabe». Ob ein Wunsch auf eine Freigabe wartet, entscheidet damit bei allen Benutzern allein Seerr. | Marco (I-F2) |
| N-112 | **Limit beim Verknüpfen (I-F3).** Wird ein Konto mit einem bestehenden Seerr-Benutzer verknüpft (über die E-Mail, von Hand, beim Annehmen einer Einladung oder nach einer E-Mail-Änderung) und hat das Konto in Slouchcat noch kein eigenes Limit, liest Slouchcat die vier Limit-Felder des Benutzers: sind alle leer, gilt «Standard» und Slouchcat schreibt die Vorgabe; sind beide Limits 0, gilt «Unbegrenzt»; sonst gilt «Eigenes» mit den Werten aus Seerr, wobei ein leeres Feld den Wert der Vorgabe bekommt. Werte ausserhalb der Grenzen von Slouchcat (Limit 0 bis 100, Tage 1 bis 90) werden auf die Grenze gesetzt und in Seerr nachgeführt. Hat das Konto schon ein eigenes Limit in Slouchcat, gewinnt dieses. Für Benutzer mit `MANAGE_USERS` bleibt es bei «Kein Limit». Das ersetzt «verknüpfter Benutzer: alter Wert überschrieben» im Umsetzungsplan (DoD 1.5c). | Marco (I-F3), Einzelheiten Redaktion |
| N-113 | **Name des Profil-Tags (I-52).** Der Tag wird aus Präfix und Profilname so gebildet, dass Radarr ihn annimmt: klein, ä → ae, ö → oe, ü → ue, scharfes S und ss → ss, andere Buchstaben mit Zeichen auf den Grundbuchstaben, alles ausser a–z und 0–9 wird `-`, mehrere `-` zu einem, kein `-` am Rand; bleibt nichts übrig, heisst der Tag `<präfix>profil-<id>`. Ein von Hand gesetzter Tag-Name je Profil (F-25) wird nach derselben Regel geprüft. | Redaktion |
| N-114 | **Unbekannter Benutzer oder falscher Schlüssel (I-2, I-3).** Auf ein 403 «permission to access this endpoint» bei einem Aufruf im Namen eines Benutzers folgt ein Aufruf als Administrator; gelingt er, gilt der Benutzer als weg (`seerr_user_gone`), sonst der Schlüssel als ungültig. | Redaktion |
