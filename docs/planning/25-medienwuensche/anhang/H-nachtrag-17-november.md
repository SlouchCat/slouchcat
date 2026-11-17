# H – Nachtrag vom 17. November 2026: Marcos Rückmeldung zu Fassung 2

Dieses Blatt hält fest, was Marco nach der Durchsicht von Fassung 2 geschrieben und auf vier Rückfragen
geantwortet hat, welche Fakten dazu neu im Seerr-Code nachgelesen wurden und was daraus für Fassung 3 folgt.
Wo die Papiere `01` bis `11` abweichen, gilt das Papier.

Kennzeichnung in diesem Blatt:

- **Marco** = steht wörtlich in seiner Nachricht oder in einer seiner vier Antworten (Abschnitte 1 und 2).
- **Redaktion** = Entscheid der Hauptsession, damit Papiere und Mockup dasselbe sagen. Das sind Empfehlungen.
  Marco hat dazu nichts entschieden; die gewichtigen stehen als Fragen F-23 bis F-32 in
  `11-entscheide-und-adr.md`.
- **belegt** = im Quellcode von Seerr v3.5.2 nachgelesen (Stelle steht dabei). **nicht geprüft** = nicht gegen
  eine laufende Instanz getestet.

---

## 1. Marcos Rückmeldung (wörtlich)

> Sieht schonmal sehr gut aus. Folgends hätte ich noch gerne.
>
> Wenn ein User über Slouchcat eingeladen wird, soll der Hauptbenutzer auch in Seerr eröffnet werden mit den
> gleichen Login Daten. Die darunterliegenden Profile benötigen kein Login sondern lediglich eine Tag oder
> ähnlich damit man sieht wer hat was angefordert.
>
> Ich möchte pro User in Slouchcat eine Limitierung. z.B. max 10 Filme pro Woche rollierend und 5 Staffeln pro
> Woche rollierend etc.
>
> Ich hätte gerne zusätzlich eine Seite für Wünsche. Analog der Startseite von Seerr. Discover, Movies, Series
> und Search.
>
> Spotlight Serie mit TVDB sollen auch unterstützt werden. Benutzt Seerr auch tvdb? Es soll konsistent sein.
>
> Bei Unklarheiten Fragen. Bitte die Mockups entsprechend ergänzen.

Dazu zwei Bildschirmfotos von Seerr: die Seitenleiste mit «Discover», «Movies», «Series» und die Seite «Series»
als Posterraster mit Zustandsmarken.

## 2. Vier Rückfragen und Marcos Antworten

| Nr. | Frage | Zur Wahl standen | Marcos Antwort |
|---|---|---|---|
| E-1 | Sollen sich eingeladene Benutzer mit E-Mail und Passwort von Slouchcat auch direkt bei Seerr anmelden können? | «Ja, gleiches Passwort» · «Nur Benutzer, ohne Anmeldung» | **Ja, gleiches Passwort** |
| E-2 | Wo soll man sehen, welches Profil einen Titel angefordert hat? | «Nur in Slouchcat» · «Auch in Seerr» (eigener Seerr-Benutzer je Profil) · «Auch in Radarr/Sonarr» (als Tag; Slouchcat spricht Radarr und Sonarr dafür direkt an, das bricht die heutige Regel) | **Auch in Radarr/Sonarr** |
| E-3 | Das Limit gilt je Konto für alle Profile gemeinsam. Was soll dazu zählen? | «Alles, Seerr zählt» · «Nur Wünsche über Slouchcat» | **Alles, Seerr zählt** |
| E-4 | Was soll mit Konten und Seerr-Benutzern geschehen, die es heute schon gibt? | «Über gleiche E-Mail verknüpfen» · «Immer von Hand zuordnen» | **Über gleiche E-Mail verknüpfen** |

Die Beschreibungen der gewählten Antworten, wie Marco sie gesehen hat:

- **E-1:** Beim Annehmen der Einladung legt Slouchcat den Seerr-Benutzer mit gleicher E-Mail und gleichem
  Passwort an. Bei jeder Passwortänderung in Slouchcat gleicht es Seerr ab. Das Passwort geht dabei einmal im
  Klartext an Seerr. Ändert jemand das Passwort direkt in Seerr, laufen die beiden auseinander.
- **E-2:** Als Tag am Film oder an der Serie. Dafür müsste Slouchcat Radarr und Sonarr direkt ansprechen. Das
  bricht die heutige Regel, dass Slouchcat das nicht tut.
- **E-3:** Du stellst das Limit in Slouchcat ein, Slouchcat schreibt es in den Seerr-Benutzer. Seerr zählt
  rollierend und rechnet auch Wünsche mit, die jemand direkt in Seerr stellt. Abgelehnte Wünsche und Mitwünsche
  zählen nicht.
- **E-4:** Gibt es in Seerr schon einen Benutzer mit derselben E-Mail, verknüpft Slouchcat ihn, statt einen
  zweiten anzulegen. Für Konten ohne Treffer gibt es in der Verwaltung den Knopf «In Seerr eröffnen».

## 3. Neue Fakten

Quelle wie in Anhang D: Seerr v3.5.2, `GH:` = `https://github.com/seerr-team/seerr/blob/v3.5.2/`. Alles in
diesem Abschnitt ist **belegt**, aber **nicht geprüft** gegen Marcos Instanz.

### 3.1 Benutzer anlegen, Passwort, Rechte

- `POST /user` legt einen lokalen Benutzer an: `email`, `username`, `password`. Mit `password` braucht Seerr kein
  SMTP. `permissions` im Body wird ignoriert, es gelten die Standardrechte. Gleiche E-Mail → 409 (Anhang D 7.2).
- `POST /user/{id}/settings/password {newPassword}`: mindestens 8 Zeichen. Ruft jemand mit `MANAGE_USERS` die
  Route für einen **anderen** Benutzer auf, setzt Seerr das Passwort ohne das alte zu verlangen
  (`GH:server/routes/user/usersettings.ts` Z. 234–248). Das Passwort von Benutzer 1 ändert nur Benutzer 1 selbst,
  das eines Administrators nur er selbst oder Benutzer 1 (Z. 222–232). Für das **eigene** Passwort verlangt Seerr
  das alte (Z. 250–261).
- Der API-Schlüssel ohne `X-API-User` handelt als Benutzer 1 (Anhang D 2.1). Folge: Slouchcat kann das Passwort
  jedes Benutzers setzen, **ausser das von Benutzer 1** (dafür müsste es dessen altes Passwort kennen).
- `POST /user/{id}/settings/main` schreibt `username`, `email`, Sprache, Regionen und die vier Quota-Felder
  **immer alle** aus dem Body; was fehlt, wird geleert (Z. 77–150). Slouchcat liest darum vorher
  `GET /user/{id}/settings/main` und schickt die übrigen Felder unverändert zurück. Die Quota-Felder nimmt Seerr
  nur an, wenn der Zielbenutzer **nicht** `MANAGE_USERS` hat und der Aufrufer nicht er selbst ist (Z. 120–129).
- `POST /user/{id}/settings/permissions {permissions}` setzt die Rechte (Z. 763–808).
- «Lokale Anmeldung» ist ab Werk an (`localLogin: true`, `GH:server/lib/settings/index.ts` Z. 423) und steht in
  `GET /settings/public`. Ist sie aus, kann sich ein lokaler Benutzer nicht anmelden (Anhang D, `POST /auth/local`).
- Slouchcat verlangt 10 Zeichen (`server/internal/auth/auth.go:40`), Seerr 8. Jedes Slouchcat-Passwort passt.

### 3.2 Limit (Quota)

`GH:server/entity/User.ts` Z. 285–390:

- Vier Felder je Benutzer: `movieQuotaLimit`, `movieQuotaDays`, `tvQuotaLimit`, `tvQuotaDays`. Fehlt ein Feld,
  gilt der Vorgabewert aus den Seerr-Einstellungen (`defaultQuotas`).
- Gezählt wird **rollierend**: Requests des Benutzers mit `createdAt` innerhalb der letzten `…Days` Tage.
  Filme: Anzahl Requests. Serien: **Summe der Staffeln** dieser Requests. Abgelehnte Requests (`DECLINED`)
  zählen nicht. Fehlgeschlagene zählen.
- Wer `MANAGE_USERS` hat (also auch Administratoren), hat kein Limit (Z. 290–296).
- `GET /user/{id}/quota` liefert je Art `days, limit, used, remaining, restricted` (Anhang D 7.2).
- Ist das Limit erreicht, antwortet `POST /request` mit 403 `Movie Quota exceeded.` oder `Series Quota exceeded.`
  (Anhang D 3.4).

### 3.3 Discover und Suche

`GH:server/routes/discover.ts`:

| Zweck | Route | Parameter (Auswahl) |
|---|---|---|
| Im Trend | `GET /discover/trending` (Z. 729) | `page`, `mediaType` (`all`, `movie`, `tv`), `timeWindow` (`day`, `week`), `language` |
| Filme | `GET /discover/movies` (Z. 105) | `page`, `sortBy`, `genre`, `studio`, `language`, `primaryReleaseDateGte/Lte`, `withRuntimeGte/Lte`, `voteAverageGte/Lte`, `watchProviders`, `watchRegion`, `keywords`, `certification`, `certificationGte/Lte`, `certificationCountry` |
| Demnächst (Filme) | `GET /discover/movies/upcoming` (Z. 367) | `page`, `language` |
| Serien | `GET /discover/tv` (Z. 418) | wie Filme, dazu `firstAirDateGte/Lte`, `status` |
| Demnächst (Serien) | `GET /discover/tv/upcoming` (Z. 679) | `page`, `language` |
| Genre-Kacheln | `GET /discover/genreslider/movie`, `/genreslider/tv` (Z. 855, 899) | `language` |
| nach Genre, Sprache, Studio, Sender, Stichwort | `/discover/movies/genre/{id}`, `/tv/genre/{id}`, `/movies/language/{l}`, `/tv/language/{l}`, `/movies/studio/{id}`, `/tv/network/{id}`, `/keyword/{id}/movies` | `page`, `language` |

- Jede Liste ist ein TMDB-Abruf; die Treffer tragen `mediaInfo` (Zustand in Seerr) aus `getRelatedMedia`.
- Die Startseite von Seerr zeigt ab Werk zwölf Reihen (`GH:server/constants/discover.ts`): Recently Added,
  Recent Requests, Plex Watchlist, Trending, Popular Movies, Movie Genres, Upcoming Movies, Studios, Popular
  Series, Series Genres, Upcoming Series, Networks. Die Reihen lassen sich in Seerr umstellen (`/settings/discover`).
- `GET /search?query=…` sucht bei TMDB. Vier Präfixe (`GH:server/lib/search.ts` Z. 46, 94, 132, 170):
  `tmdb:<id>`, `imdb:<tt…>`, `tvdb:<id>`, `year:<jjjj>`. `tvdb:` löst über TMDB «find by external id» auf
  (Z. 131–168).

### 3.4 TVDB in Seerr

- Seerr führt jeden Titel über die **TMDB-ID**. Die TVDB-ID einer Serie nimmt es aus TMDB
  (`external_ids.tvdb_id`, `GH:server/api/tvdb/index.ts` Z. 550–552) und gibt sie an Sonarr weiter. Sonarr selbst
  arbeitet mit der TVDB-ID. Fehlt sie, löscht Seerr den Request beim Senden (Anhang D, Abschnitt 4).
- Seit 3.x gibt es die Einstellung «Metadaten-Anbieter» für Serien und Anime: `tmdb` (ab Werk) oder `tvdb`
  (`GH:server/lib/settings/index.ts` Z. 111–118, 460–463; Route `/settings/metadatas`). Mit `tvdb` lädt Seerr die
  Serie weiter über die TMDB-ID, ersetzt aber die **Staffelliste** durch die von TVDB
  (`enrichTmdbShowWithTvdbData`, `GH:server/api/tvdb/index.ts` Z. 136–260; `GH:server/api/metadata.ts` Z. 8–39).
- Folge: Der Schlüssel bleibt immer die TMDB-ID. Was sich unterscheiden kann, ist die **Staffelzählung**: TMDB und
  TVDB teilen manche Serien verschieden auf. Sonarr zählt nach TVDB, die Slouchcat-Bibliothek ordnet Serien
  TVDB-zuerst zu (`server/internal/metadata/tvdb.go:430-434`).

### 3.5 Tags

- `tags` im Body von `POST /request` sind numerische Tag-IDs von Radarr/Sonarr. Seerr übernimmt sie nur, wenn der
  **anfordernde** Benutzer `MANAGE_REQUESTS` oder `REQUEST_ADVANCED` hat
  (`GH:server/entity/MediaRequest.ts` Z. 265–294). Sie **ersetzen** dann die Standard-Tags des Servers
  (`GH:server/subscriber/MediaRequestSubscriber.ts` Z. 299–302, 706–707).
- Seerr kann selbst taggen: Mit «Tag Requests» am Radarr- oder Sonarr-Server legt es je anforderndem Benutzer den
  Tag `<Benutzer-ID>-<Anzeigename>` an und hängt ihn an (Z. 346–380). Das ist der Benutzer des **Kontos**, nicht
  das Profil.
- Seerr liest die Tag-Liste (`GET /service/radarr/{id}`, `/service/sonarr/{id}`,
  `GH:server/routes/service.ts` Z. 32–162), bietet aber **keine Route, um Tags anzulegen**.
- `GET /settings/radarr` und `/settings/sonarr` geben die Servereinträge vollständig zurück, mit `hostname`,
  `port`, `useSsl`, `baseUrl`, `apiKey`, `tags`, `tagRequests`, `isDefault`, `is4k`
  (`GH:server/routes/settings/radarr.ts` Z. 9–13; `DVRSettings`, `GH:server/lib/settings/index.ts` Z. 68–87).
- Radarr und Sonarr (API v3, öffentliche Dokumentation, **nicht geprüft**): `GET /api/v3/tag`,
  `POST /api/v3/tag {label}`, `GET /api/v3/movie?tmdbId=`, `PUT /api/v3/movie/editor {movieIds, tags, applyTags: "add"}`,
  `GET /api/v3/series?tvdbId=`, `PUT /api/v3/series/editor {seriesIds, tags, applyTags: "add"}`. Tag-Namen: nur
  Kleinbuchstaben, Ziffern, Bindestrich. Sonarr kennt Tags nur an der Serie, nicht an der Staffel.

### 3.6 Slouchcat heute

- Einladung annehmen: Operation `acceptInvitation` (`api/openapi.yaml:1168 ff.`), Seite `/invite/$token`
  (`web/src/app/router.tsx:237`). Der Server sieht das Passwort hier im Klartext, ebenso bei jeder Anmeldung und
  bei `updateAccount` (eigenes Konto, `web/src/features/admin/accounts/OwnAccountForm.tsx`).
- ADR 0010: «Slouchcat never calls Radarr or Sonarr and registers nothing there» (Anhang C 1). E-2 ändert das.

---

## 4. Was für Fassung 3 gilt

### 4.1 Konto und Seerr-Benutzer (E-1, E-4)

| Nr. | Regel | Von |
|---|---|---|
| N-1 | Nimmt jemand eine Einladung an, legt Slouchcat im selben Zug den Seerr-Benutzer an: gleiche E-Mail, gleiches Passwort, `username` = Teil der E-Mail vor dem @. | Marco (Nachricht, E-1) |
| N-2 | Gibt es in Seerr schon einen Benutzer mit derselben E-Mail (Vergleich ohne Gross-/Kleinschreibung), verknüpft Slouchcat ihn, statt einen zweiten anzulegen. Für Konten ohne Seerr-Benutzer gibt es «In Jellyseerr eröffnen» (einzeln) und «Alle eröffnen». Eingeschränkt durch N-2a (Abschnitt 9, K-1): Tippt das Konto die Adresse selbst ein, entsteht nur ein Vorschlag, den ein Administrator bestätigt. | Marco (E-4), Einschränkung Redaktion |
| N-3 | Slouchcat speichert das Passwort **nie im Klartext**. Es gibt es nur in den drei Momenten weiter, in denen es der Server ohnehin sieht: Einladung annehmen, Passwort ändern oder zurücksetzen, Anmeldung. | Redaktion |
| N-4 | Kann Slouchcat das Passwort nicht sofort weitergeben (Seerr nicht erreichbar, Konto bestand schon, Benutzer nachträglich eröffnet), bekommt der Seerr-Benutzer zuerst ein Zufallspasswort und den Vermerk «bei der nächsten Anmeldung übernehmen». Bei der nächsten Anmeldung in Slouchcat setzt Slouchcat das Passwort in Seerr. | Redaktion |
| N-5 | Passwort ändern oder zurücksetzen in Slouchcat → sofort `POST /user/{id}/settings/password`. Schlägt das fehl, gilt N-4. E-Mail ändern in Slouchcat → auch in Seerr. | Marco (E-1), Ablauf Redaktion |
| N-6 | Kein Abgleich bei: Benutzer 1 (technisch nicht möglich, 3.1), anderen Administratoren von Seerr (bewusst ausgenommen, «bleibt in Jellyseerr») und Benutzern, die sich über Jellyfin, Emby oder Plex anmelden. | Redaktion |
| N-7 | Ein Passwort, das jemand direkt in Seerr ändert, bemerkt Slouchcat nicht. Die Verwaltung bietet «Passwort bei der nächsten Anmeldung neu übernehmen». | Marco kannte die Folge (E-1), Knopf Redaktion |
| N-8 | Schalter in Wünsche › Einstellungen: «Neue Konten in Jellyseerr eröffnen» und «Passwort abgleichen», beide ab Werk an, sobald Jellyseerr eingerichtet ist. | Redaktion |
| N-9 | Rechte des eröffneten Benutzers: mit Wunschrecht des Kontos `REQUEST`, ohne Wunschrecht keine (`permissions = 0`). Ändert der Administrator das Wunschrecht, zieht Slouchcat die Rechte nach. Rechte von Benutzern, die es schon gab, fasst Slouchcat nicht an. | Redaktion |
| N-10 | Löscht der Administrator ein Konto in Slouchcat, bleibt der Seerr-Benutzer (mit ihm würden seine Requests gelöscht, Anhang D 7.2). Hat Slouchcat ihn eröffnet, nimmt es ihm die Rechte und setzt ein Zufallspasswort; einen Benutzer, den es schon gab, lässt es unverändert (Abschnitt 9, K-2). | Redaktion |
| N-11 | Voraussetzung «Lokale Anmeldung» in Seerr: Der Verbindungstest prüft sie und warnt. | Redaktion |
| N-12 | Das Passwort geht im Body an Seerr. Läuft die Verbindung über `http://` ausserhalb des eigenen Netzes, warnt der Verbindungstest. Body und Passwort stehen nie im Protokoll (`CLAUDE.md` Regel 10; neue Schlüssel `password`, `newPassword` in die Redaction-Liste). | Redaktion |

Ablauf der Eröffnung (Job `seerr.provision`, ausgelöst durch die Einladung, den Knopf oder den Abgleich):

1. `GET /user?q=<E-Mail>`; exakter Treffer → verknüpfen (N-2), weiter bei 4.
2. Sonst `POST /user {email, username, password}`; 409 → wie Treffer behandeln.
3. `POST /user/{id}/settings/permissions` (N-9).
4. `GET` und `POST /user/{id}/settings/main` mit dem Limit (Abschnitt 4.3), nur für Benutzer ohne `MANAGE_USERS`.
5. Zustand am Konto speichern: eröffnet, verknüpft, von Hand zugeordnet, wartet; dazu der Stand des Passworts.

### 4.2 Profil-Tag (Nachricht, E-2)

| Nr. | Regel | Von |
|---|---|---|
| N-20 | Profile bekommen in Seerr keinen eigenen Benutzer und kein Login. Slouchcat merkt sich je Wunsch das Profil (wie in Fassung 2) und setzt zusätzlich einen Tag am Film in Radarr oder an der Serie in Sonarr. | Marco (Nachricht, E-2) |
| N-21 | Name des Tags: Präfix + Profilname in Kleinbuchstaben, nur a–z, 0–9 und Bindestrich. Präfix ab Werk `sc-` (einstellbar). Beispiel: `sc-noah`. Haben zwei Profile denselben Namen, behält das erste den einfachen Namen und das später angelegte bekommt den Kontonamen dazu (`sc-kinder-grosseltern`; Abschnitt 9, K-18). Je Profil lässt sich der Name überschreiben. | Redaktion |
| N-22 | Slouchcat spricht Radarr und Sonarr dafür direkt an, aber **nur** für zwei Dinge: Tag anlegen und Tag an den Titel hängen. Es fügt nichts hinzu, löscht nichts, sucht nichts und ändert keine Qualität. | Marco (E-2), Abgrenzung Redaktion |
| N-23 | Adresse und Schlüssel von Radarr und Sonarr liest Slouchcat aus Seerr (`GET /settings/radarr`, `/settings/sonarr`, Standardserver ohne 4K). Der Administrator tippt keinen Schlüssel ein. Erreicht Slouchcat die Adresse nicht (etwa Docker-interne Namen), trägt er eine eigene Adresse ein. Die Schlüssel hält Slouchcat nur im Speicher. | Redaktion |
| N-24 | Zeitpunkt: sobald der Wunsch «In Beschaffung» ist, also Seerr den Titel an Radarr oder Sonarr übergeben hat. Job `wishes.tag`, läuft mit dem Abgleich. Ein Mitwunsch bekommt seinen Tag nachträglich. | Redaktion |
| N-25 | Schlägt das Setzen fehl, ändert das nichts am Wunsch. Die Verwaltung zeigt «Tag noch nicht gesetzt», der nächste Abgleich holt es nach. | Redaktion |
| N-26 | Slouchcat entfernt Tags nie. | Redaktion |
| N-27 | Wünsche, die direkt in Seerr gestellt wurden, haben kein Profil und bekommen keinen Profil-Tag. | Redaktion |
| N-28 | Ergänzend empfohlen: «Tag Requests» in Seerr einschalten. Dann trägt jeder Titel auch den Tag des Kontos (`4-grosseltern`). Der Verbindungstest zeigt, ob es an ist. | Redaktion |

Verworfen: der Weg über `tags` im Seerr-Request. Er braucht für jeden Benutzer das Recht `REQUEST_ADVANCED`,
ersetzt die Standard-Tags des Servers, kann einem Mitwunsch keinen Tag nachreichen, und der Tag müsste in Radarr
und Sonarr schon bestehen, weil Seerr keine Tags anlegt (3.5).

### 4.3 Limit (Nachricht, E-3)

| Nr. | Regel | Von |
|---|---|---|
| N-30 | Je Konto ein Limit für Filme und eines für Staffeln, je mit Anzahl und Zeitraum in Tagen, rollierend. Es gilt für alle Profile des Kontos gemeinsam. | Marco (Nachricht, E-3) |
| N-31 | Eingestellt wird in Slouchcat: Vorgabe in Wünsche › Einstellungen, abweichend je Konto unter Konten («Standard», «Eigenes», «Unbegrenzt»). Vorgabe ab Werk: 10 Filme in 7 Tagen, 5 Staffeln in 7 Tagen (Marcos Beispiel). 0 heisst ohne Limit. | Marco (Beispielwerte), Aufbau Redaktion |
| N-32 | Slouchcat schreibt das Limit in den Seerr-Benutzer des Kontos. **Seerr zählt** und setzt durch: Auch Wünsche, die jemand direkt in Seerr stellt, zählen mit. | Marco (E-3) |
| N-33 | Es zählt nicht: ein abgelehnter Wunsch, ein Mitwunsch (er erzeugt keinen Request), ein zurückgezogener Wunsch (der Request wird in Seerr gelöscht). Ein fehlgeschlagener Request zählt (3.2). | Marco (E-3: abgelehnt, Mitwunsch), Rest folgt aus 3.2 |
| N-34 | Administratoren haben kein Limit: Seerr nimmt Benutzer mit `MANAGE_USERS` aus. Die Oberfläche sagt das. | folgt aus 3.2 |
| N-35 | Slouchcat liest den Stand bei jedem Abgleich und nach jedem Wunsch (`GET /user/{id}/quota`) und rechnet Wünsche dazu, die noch auf die Übermittlung warten. Wann der nächste Platz frei wird, errechnet Slouchcat aus den Requests des Benutzers (ältester im Zeitraum + Tage). | Redaktion |
| N-36 | Ist das Limit erreicht, bleibt der Titel sichtbar. Der Dialog «Anfordern» zeigt, wie viel frei ist, und sperrt den Knopf mit dem Zeitpunkt, an dem der nächste Platz frei wird. Mitwünschen geht weiter. Bei Serien zählt jede gewählte Staffel. | Redaktion |
| N-37 | Für einen bestehenden, verknüpften Seerr-Benutzer eines Kontos überschreibt Slouchcat das Limit, das dort stand: Für Konten gibt es genau einen Ort, an dem das Limit eingestellt wird. In einen Ausnahme-Benutzer (N-41) schreibt Slouchcat kein Limit; dort gilt, was in Seerr steht. | Redaktion |
| N-38 | Kein Teil-Limit je Profil. | Redaktion |

### 4.4 Ausnahme je Profil (bleibt aus Fassung 2)

| Nr. | Regel | Von |
|---|---|---|
| N-40 | Der Normalfall ist neu: Alle Profile senden als Benutzer des Kontos (Variante A aus `04`), das Profil steht am Wunsch und als Tag. | Marco (Nachricht) |
| N-41 | Die Zuordnung «Profil sendet als anderer Jellyseerr-Benutzer» aus Fassung 2 bleibt als zugeklappte **Ausnahme**. Grund: Nur so braucht der Wunsch eines Kinderprofils eine Freigabe, wenn das Konto selbst automatisch freigegeben wird (etwa das Konto des Administrators). Für so ein Profil gelten Freigabe und Limit des Ausnahme-Benutzers. | Redaktion (Frage F-27) |

### 4.5 Bereich «Wünsche» wie die Startseite von Seerr (Nachricht)

| Nr. | Regel | Von |
|---|---|---|
| N-50 | Der Eintrag «Wünsche» der Seitenleiste führt in einen Bereich mit fünf Reitern: «Entdecken», «Filme», «Serien», «Suche», «Meine Wünsche». | Marco (Nachricht: Discover, Movies, Series, Search), «Meine Wünsche» als fünfter Reiter Redaktion |
| N-51 | «Entdecken» zeigt Reihen wie Seerr: Im Trend, Beliebte Filme, Film-Genres, Demnächst im Kino, Beliebte Serien, Serien-Genres, Demnächst als Serie. Die Reihen «Recently Added», «Recent Requests», «Plex Watchlist», «Studios», «Networks» entfallen. | Redaktion (Frage F-28) |
| N-52 | «Filme» und «Serien» sind Posterraster mit Sortierung (Beliebtheit, Neueste zuerst, Titel), Genre-Filter und «Vorhandene ausblenden». Der Filter ist zugeklappt und zeigt seine Wahl im Kopf. | Redaktion |
| N-53 | «Suche» sucht in allem, was TMDB kennt, auch nach Kennung: `tmdb:`, `tvdb:`, `imdb:`. Personen zeigt sie nicht. | Redaktion |
| N-54 | Die Daten holt der Server über die Discover- und Suchrouten von Seerr (3.3), im Namen des Seerr-Benutzers, mit Zwischenspeicher (1 Stunde je Liste, Seite, Sprache und Altersstufe; Job `wishes.discover`). Der Client spricht nie mit Seerr oder TMDB. | Redaktion |
| N-55 | Titel, die in der Bibliothek stehen, erscheinen auch, mit Marke und Link zur Detailseite. Massgeblich ist der Abgleich von Slouchcat (TMDB, TVDB, IMDb), nicht `mediaInfo` von Seerr. | Redaktion |
| N-56 | Es gilt F-9: Den Bereich sieht nur ein Profil, das anfordern kann. Alle anderen sehen höchstens «Meine Wünsche». Die Altersstufe gilt für jeden Titel (unbekannt zählt als 18); für Profile mit Altersgrenze fragt der Server Seerr mit `certificationLte` und prüft jeden Treffer selbst nach. | Redaktion (folgt aus F-9 und der Jugendschutzregel) |
| N-57 | Die Bibliothekssuche (M-07) bleibt, wie in Fassung 2. Neu verweist sie auf die Wunsch-Suche. Schalter «Seiten Entdecken, Filme, Serien und Suche» in Wünsche › Einstellungen. | Redaktion |
| N-58 | Kein Verstoss gegen das Nicht-Ziel «algorithmische Empfehlungen»: Die Listen sind öffentliche TMDB-Ranglisten, für alle gleich. S-4 («Titel ohne Bibliothekstreffer nie auf der Startseite») bleibt, die Seiten liegen unter «Wünsche». | Redaktion |

### 4.6 TVDB, durchgängig (Nachricht)

Antwort auf Marcos Frage: **Seerr führt jeden Titel über die TMDB-ID. TVDB nutzt es für zwei Dinge: Es gibt die
TVDB-ID an Sonarr weiter, und es kann die Staffelliste aus TVDB lesen, wenn man das einstellt.**

| Nr. | Regel | Von |
|---|---|---|
| N-60 | Jede Serie ausserhalb der Bibliothek trägt drei IDs: TMDB, TVDB, IMDb. Quelle: Trakt (liefert alle drei), sonst TMDB `external_ids`. | Fassung 2 (`03`, `05`), unverändert |
| N-61 | Abgleich mit der Bibliothek bei Serien: **TVDB zuerst**, dann TMDB, dann IMDb, so wie die Bibliothek Serien zuordnet. Bei Filmen: TMDB, dann IMDb. Fassung 2 nannte TMDB zuerst. | Redaktion (Marco: «konsistent») |
| N-62 | Ein Spotlight-Eintrag, zu dem Trakt nur die TVDB-ID liefert, wird über TMDB «find» aufgelöst. Gelingt das nicht, gilt er nicht als fehlend (er wäre nicht anforderbar) und steht in der Verwaltung unter den nicht zuordenbaren Einträgen. | Redaktion |
| N-63 | Die Staffeln im Dialog «Anfordern» kommen aus Seerr (`GET /tv/{tmdbId}`): Angeboten wird genau, was Seerr bestellen kann. | Redaktion |
| N-64 | Empfehlung an Marco: in Seerr den Metadaten-Anbieter für Serien auf TVDB stellen. Dann zählen Seerr, Sonarr und Slouchcat Staffeln gleich. Der Verbindungstest liest die Einstellung (`GET /settings/metadatas`) und zeigt einen Hinweis, wenn sie auf TMDB steht. | Redaktion (Frage F-29) |
| N-65 | Der Wunsch geht immer mit `tvdbId` an Seerr (Fassung 2). | Fassung 2 |

### 4.7 Stufen

Redaktion: Die Stufen 0 bis 4 aus `09-umsetzungsplan.md` bleiben. Eröffnung, Passwortabgleich und Limit gehören
zu Stufe 1 (ohne sie lässt sich nicht sinnvoll anfordern). Der Profil-Tag gehört zu Stufe 4. Neu ist Stufe 5
«Wünsche-Seiten» (Entdecken, Filme, Serien, Suche).

---

## 5. Was sich an früheren Entscheiden zusätzlich ändert

Fassung 2 schlug eine Ausnahme vom Nicht-Ziel «Wünschen» vor. Fassung 3 braucht zwei weitere Änderungen. Auch sie
sind nur Vorschlag; dieses Konzept ändert keine Regel.

| Stelle | Heute | Mit Fassung 3 |
|---|---|---|
| ADR 0010 | «Slouchcat never calls Radarr or Sonarr and registers nothing there» | Ausnahme: Slouchcat legt Tags an und hängt sie an Titel (N-22). Sonst bleibt der Satz. |
| `CLAUDE.md` Regel 11, Spec «Non-Goals» | «Steuerung von Arr/Seerr» ist Nicht-Ziel | Zwei eng gefasste Ausnahmen: Benutzer in Seerr anlegen und pflegen (E-Mail, Passwort, Rechte, Limit), Tags in Radarr und Sonarr setzen. Qualitätsprofile, Warteschlangen, Suchen, Löschen bleiben Nicht-Ziel. |
| `CLAUDE.md` Regel 10 | Redaction-Liste | Neue Schlüssel `password`, `newPassword`, Radarr- und Sonarr-Schlüssel. |

## 6. Neue offene Fragen

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-23 | Für wen eröffnet Slouchcat einen Seerr-Benutzer? | Für jedes Konto, sobald Jellyseerr eingerichtet ist; ohne Wunschrecht ohne Rechte in Seerr (N-9). |
| F-24 | Passwort bei verknüpften, schon bestehenden Seerr-Benutzern übernehmen? | Ja, bei der nächsten Anmeldung, ausser bei Administratoren und bei Anmeldung über Jellyfin/Plex (N-4, N-6). |
| F-25 | Name des Profil-Tags | `sc-<profilname>`, Präfix einstellbar, je Profil überschreibbar; Tags werden nie entfernt (N-21, N-26). |
| F-26 | Vorgabe des Limits ab Werk | 10 Filme in 7 Tagen, 5 Staffeln in 7 Tagen; Administratoren ohne Limit (N-31, N-34). |
| F-27 | Ausnahme «Profil sendet als anderer Jellyseerr-Benutzer» behalten? | Ja, zugeklappt. Sonst sind Kinderprofile im Konto des Administrators ohne Freigabe und ohne Limit (N-41). |
| F-28 | Reihen auf «Entdecken» | Die sieben aus N-51, fest. Nicht die Reihen-Einstellung von Seerr übernehmen. |
| F-29 | Seerr auf TVDB-Staffeln umstellen? | Ja (N-64). Das stellt Marco in Seerr selbst um. |
| F-30 | Radarr und Sonarr: Zugang aus Seerr lesen oder eigene Felder? | Aus Seerr lesen, eigene Adresse nur als Ausweg (N-23). |
| F-31 | Was geschieht mit dem Seerr-Benutzer, wenn das Konto in Slouchcat gelöscht wird? | Bleibt, ohne Rechte, mit Zufallspasswort (N-10). |
| F-32 | Name in der Oberfläche: «Jellyseerr» oder «Seerr»? Marco schreibt «Seerr», das Projekt heisst seit 3.0 so. | «Seerr». Papiere und Mockup sagen bis zur Antwort weiter «Jellyseerr», damit sie einheitlich bleiben. |

## 7. Neue und geänderte Mockup-Seiten

| Seite | Was |
|---|---|
| M-31 Wünsche · Entdecken (neu) | Reihen, Genre-Kacheln, Suchfeld |
| M-32 Wünsche · Filme (neu) | Raster, Filter, Sortierung, «Mehr laden» |
| M-33 Wünsche · Serien (neu) | wie M-32 |
| M-34 Wünsche · Suche (neu) | Suche, auch nach Kennung |
| M-35 Einladung annehmen (heute vorhanden, im Mockup neu) | Hinweis «Dein Login gilt auch für Jellyseerr», Eröffnung beim Absenden |
| M-08 Meine Wünsche | Reiter, Limit-Anzeige |
| M-13 Dialog «Anfordern» | Limit: frei, erreicht, zu viele Staffeln |
| M-07 Suche | Verweis auf die Wunsch-Suche |
| M-20 Übersicht | Hinweise: Konto ohne Benutzer, Limit erreicht, Tag nicht gesetzt |
| M-22 Integrationen › Jellyseerr | neue Prüfungen, Bänder «Benutzer und Limit», «Radarr und Sonarr (Profil-Tags)» |
| M-23 Anfragen | Profil-Tag je Wunsch, Band «Limit je Konto» |
| M-24 Jellyseerr-Benutzer (vorher «Zuordnung») | je Konto: Zustand, Passwort, Rechte, Limit; je Profil: Tag und Ausnahme |
| M-25 Einstellungen | Limit ab Werk, Konten in Jellyseerr, Profil-Tag, Schalter für die neuen Seiten |
| M-26 Konten | Limit je Konto, Jellyseerr-Benutzer, Einladung mit Limit, Passwortänderung |
| M-27 Spotlight | IDs je Eintrag, Band «Abgleich über TMDB, TVDB und IMDb» |
| M-30 Hintergrundjobs | `seerr.provision`, `wishes.tag`, `wishes.discover` |
| M-00 Steuerung | Limit ausschöpfen, Radarr/Sonarr ausfallen lassen, Staffelquelle von Seerr, Einladung annehmen, Anmeldung spielen |

---

## 8. Zweite Rückmeldung vom 17. November 2026

Marco hat das Mockup der Fassung 3 angesehen und zwei Dinge geändert. Dieser Abschnitt gilt vor den Abschnitten
4 bis 7, wo er ihnen widerspricht.

### 8.1 Marcos Nachricht (wörtlich)

> Folgende Anpassunge wünsche ich:
>
> In Spotlight soll nicht angezeigt werden was fehlt. Nur vorhandene Filme und Serien sollen auf der Startseite
> angezeigt werden. In der Verwaltung ist es gut so wie du es geändert hat.
>
> Wäre es noch Sinnvoll dass wir Radarr und Sonarr direkt auch implementieren? Damit läuft die Synchronisierung
> schneller und die beiden können Slouchcat direkt mitteilen wann und wo der Film nun vorhanden ist. D.h. es
> sollte rasendschnell gehen, dass der Film oder Serie in Slouchcat importiert wird. Ev. nehmen wir die beiden
> Webhooks wieder raus und machen es direkt mit API.
>
> Nimm das mit auf ins Konzept und stelle Fragen bei Unklarheiten und passe das Mockup an

### 8.2 Drei Rückfragen und Marcos Antworten

| Nr. | Frage | Zur Wahl standen | Marcos Antwort |
|---|---|---|---|
| E-5 | Fehlende Titel erscheinen nicht mehr im Spotlight der Startseite. Sollen die fehlenden Titel der Trakt-Liste stattdessen unter Wünsche › Entdecken als eigene Reihe stehen? | «Ja, als Reihe» · «Nein, nirgends» | **Nein, nirgends** |
| E-6 | Radarr und Sonarr melden von sich aus nur per Webhook; über die API muss Slouchcat nachfragen. Wie soll Slouchcat angebunden sein? | «API und Webhook» (Slouchcat richtet den Webhook selbst ein, holt Verpasstes per API nach) · «Nur API, ohne Webhook» (regelmässiges Abfragen) · «Webhooks wie heute» | **Webhooks wie heute** |
| E-7 | Soll Slouchcat bei Wünschen den Download-Fortschritt aus Radarr und Sonarr zeigen, zum Beispiel «Wird heruntergeladen, 45 %, noch 12 Minuten»? | «Ja, bei Wünschen» · «Nein» | **Ja, bei Wünschen** |

Die Beschreibungen der gewählten Antworten, wie Marco sie gesehen hat:

- **E-5:** Fehlende Titel der Trakt-Liste sieht nur der Administrator in der Verwaltung. Anfordern geht über
  Entdecken, Filme, Serien und Suche.
- **E-6:** Du richtest die Webhooks weiter von Hand ein. Die API nutzt Slouchcat nur für die Profil-Tags.
- **E-7:** Unter «Meine Wünsche» und in der Verwaltung. Die Anzeige des Beschaffungsstands ist heute ein
  Nicht-Ziel; das wäre eine weitere Ausnahme.

E-6 und E-7 zusammen heissen: Die API von Radarr und Sonarr dient zwei Dingen, dem Profil-Tag und dem Lesen des
Download-Fortschritts.

### 8.3 Neue Fakten

**Geprüft gegen Marcos Instanzen** (Radarr 6.4.4.10685, Sonarr 4.0.20.3014; nur lesende Aufrufe; gelesen wurden
Versionen, Feldnamen und Anzahlen, keine Schlüssel und keine Pfade):

- Radarr und Sonarr melden von sich aus nur über «Connect» (Webhook, Skript und Ähnliches). Die API ist rein
  abfragend. Wer ohne Webhook auskommen will, muss in einem Takt nachfragen.
- Ein Webhook hat in beiden die Felder `url`, `method`, `username`, `password`, `headers` und Schalter je
  Ereignis (`onDownload`, `onUpgrade`, `onRename`, bei Radarr `onMovieDelete`, `onMovieFileDelete`, bei Sonarr
  `onImportComplete`, `onSeriesDelete`, `onEpisodeFileDelete` und weitere). Abrufbar über
  `GET /api/v3/notification` und `/api/v3/notification/schema`.
- `GET /api/v3/queue`: ein Eintrag trägt `size`, `sizeleft`, `timeleft`, `estimatedCompletionTime`, `status`,
  `trackedDownloadState`, `trackedDownloadStatus`, `errorMessage`, `statusMessages`, `downloadClient`, bei
  Radarr `movieId`. In Sonarr war die Warteschlange beim Test leer; die Felder `seriesId`, `episodeId`,
  `seasonNumber` stehen in der Dokumentation, sind aber **nicht geprüft**.
- `GET /api/v3/history/since?date=…&eventType=downloadFolderImported`: je Import `date`, `movieId` (Radarr)
  oder `seriesId` und `episodeId` (Sonarr), in `data` `importedPath`, `droppedPath`, `fileId`.
- `GET /api/v3/movie?tmdbId=…`: `hasFile`, `movieFile.path`, `path`, `tags`, `tmdbId`, `imdbId`.
  `GET /api/v3/series?tvdbId=…`: `path`, `seasons`, `statistics` (`episodeFileCount`, `episodeCount`,
  `percentOfEpisodes`), `tags`, `tvdbId`, `tmdbId`, `imdbId`.
- `GET /api/v3/tag`: `id`, `label`. `GET /api/v3/rootfolder`: `path`, `accessible`, `freeSpace`.
- Schreibende Aufrufe (`POST /api/v3/tag`, `PUT /api/v3/movie/editor`, `PUT /api/v3/series/editor`) wurden
  nicht ausgeführt und bleiben **nicht geprüft**.

**Gelesen im Repo:**

- Die aufgezeichneten Webhook-Meldungen (`testdata/webhooks/radarr-6.4.4.10685-download.json`,
  `sonarr-4.0.20.3014-download.json`) tragen bei Radarr `movie.tmdbId`, `movie.imdbId`, `movie.folderPath`,
  `movieFile.path`, bei Sonarr `series.tvdbId`, `series.tmdbId`, `series.imdbId`, `series.path`,
  `episodeFile.path`, `episodes[]`.
- Slouchcat liest davon heute nur `eventType` und den Ordner und reiht einen Ordner-Scan ein (`scan.path`,
  Entprellung 2 s, Priorität 6; Anhang C 1). Die Kennungen und der genaue Dateipfad bleiben ungenutzt.

### 8.4 Was gilt

**Spotlight**

| Nr. | Regel | Von |
|---|---|---|
| N-70 | Startseite (Bühne und Posterreihe) und Spotlight-Liste zeigen nur Titel der Bibliothek, wie heute. S-4 bleibt uneingeschränkt. Fehlende Einträge der Trakt-Liste erscheinen in der Zuschauer-App nirgends, auch nicht als Reihe unter «Entdecken». | Marco (Nachricht, E-5) |
| N-71 | Verwaltung › Spotlight bleibt, wie sie in Fassung 3 war: Einträge der Liste auch wenn sie fehlen, mit Poster, Zustand und der Aktion «Anfordern», dazu die IDs je Eintrag und das Band «Abgleich über TMDB, TVDB und IMDb». | Marco (Nachricht: «In der Verwaltung ist es gut so») |
| N-72 | Es entfallen: die Schalter «Fehlende Titel zeigen», «Höchstens fehlende Titel», «Reihenfolge» (Spotlight), «Aus dem Spotlight» (Wünsche › Einstellungen), die Regel «erste Folie immer vorhanden» und alles aus `05`, was fehlende Titel auf der Bühne beschreibt. F-8 ist durch E-5 entschieden. Das Band der Verwaltung heisst «Fehlende Einträge» und stellt nichts mehr ein. | Redaktion (folgt aus N-70) |
| N-73 | Vom Spotlight bleibt als Bauarbeit: Vorhandene Serien werden auch über die TVDB- und die IMDb-ID erkannt (N-60 bis N-62). Heute fehlen TVDB-zuerst-Serien im Spotlight, obwohl sie in der Bibliothek stehen. Dazu die Liste der fehlenden Einträge in der Verwaltung. | Marco (erste Rückmeldung: «Spotlight Serie mit TVDB»), Zuschnitt Redaktion |
| N-74 | Die Startreihe «Deine Wünsche» (F-20) zeigt nur noch Wünsche, die in den letzten 14 Tagen wahr wurden, also Titel der Bibliothek, und heisst «Erfüllte Wünsche». Offene Wünsche stehen unter «Wünsche › Meine Wünsche»; ihre Zahl steht am Eintrag «Wünsche» der Seitenleiste. | Redaktion (liest Marcos Satz «Nur vorhandene Filme und Serien sollen auf der Startseite angezeigt werden»; offen als F-34) |
| N-75 | ADR 0038 löst S-4 nicht mehr ab. Titel ausserhalb der Bibliothek gibt es nur noch im Bereich «Wünsche», im Abschnitt «Noch nicht in der Bibliothek» der Suche (Schalter), auf der Vorschauseite und als fehlende Staffeln einer vorhandenen Serie. | Redaktion (folgt aus N-70) |

**Radarr und Sonarr**

| Nr. | Regel | Von |
|---|---|---|
| N-80 | Die Webhooks von Radarr und Sonarr bleiben, wie sie heute sind: von Hand eingerichtet, Seite «Webhooks von Radarr und Sonarr» unverändert, eingehender Teil von ADR 0010 unverändert. Slouchcat trägt in Radarr und Sonarr keinen Webhook ein. | Marco (E-6) |
| N-81 | Die API von Radarr und Sonarr nutzt Slouchcat für genau zwei Dinge: den Profil-Tag (N-22) und das Lesen des Download-Fortschritts. Gelesen werden die Warteschlange und der Dateistand der gewünschten Titel (Abschnitt 9, K-5). | Marco (E-6, E-7), Umfang des Lesens Redaktion |
| N-82 | Der Fortschritt steht beim Wunsch: unter «Meine Wünsche», auf der Wunschkarte der Vorschauseite und in der Verwaltung unter Anfragen. Nicht an Titeln der Bibliothek, nicht in Reihen, nicht in der Suche. | Marco (E-7: «bei Wünschen»), Abgrenzung Redaktion |
| N-83 | Vier Anzeigen, solange ein Wunsch «In Beschaffung» ist: «Wird gesucht» (noch nichts in der Warteschlange), «Wird heruntergeladen · n % · noch m min» (Warteschlange), «Wird eingelesen» (Radarr oder Sonarr haben importiert, Slouchcat hat den Titel noch nicht), «Download verzögert» (die Warteschlange meldet eine Warnung oder einen Fehler). Die Zuschauer-App zeigt nur diese Worte; die Meldung von Radarr oder Sonarr sieht nur die Verwaltung. Bei Serien dazu «x von y Folgen da». | Redaktion |
| N-84 | Job `wishes.progress`: jede Minute, nur solange es Wünsche in Beschaffung gibt, ein Abruf der Warteschlange je Dienst. Das Ergebnis liegt im Zwischenspeicher; kein Aufruf an Radarr oder Sonarr im Seitenaufbau. | Redaktion |
| N-85 | Sind Radarr oder Sonarr nicht erreichbar, zeigt die Zuschauer-App nichts Zusätzliches (der Wunsch bleibt «In Beschaffung»), die Verwaltung «Fortschritt nicht lesbar». | Redaktion |
| N-86 | Schalter «Download-Fortschritt zeigen» in Wünsche › Einstellungen, ab Werk an. | Redaktion |
| N-87 | Der Fortschritt ist nur Anzeige. Die Zustände des Wunsches ändern sich nicht; «verfügbar» heisst weiter: Slouchcat kann den Titel abspielen (F-13). | Redaktion |
| N-88 | Vorschlag für Marcos Ziel «rasend schnell», ohne neue Anbindung: Die Webhook-Meldung nennt schon den genauen Dateipfad und die Kennung (8.3). Slouchcat liest beides, liest nur diese Datei mit Vorrang ein, ordnet sie über die Kennung zu (ohne Titelsuche) und prüft sofort die offenen Wünsche. Vorher messen, wo heute die Zeit bleibt (`docs/perf`). | Redaktion (offen als F-33; nicht Teil von E-6) |

Von Marco nicht gewählt (E-6) und damit verworfen: «API und Webhook» (Slouchcat trägt den Webhook selbst ein und
holt Verpasstes über `history/since` nach) und «Nur API, ohne Webhook» (Abfragen im Takt; langsamer als die
Sofortmeldung, dauernde Grundlast).

### 8.5 Was sich an früheren Entscheiden ändert (ergänzt Abschnitt 5)

| Stelle | Heute | Mit Fassung 3 |
|---|---|---|
| S-4 «Titel ohne Bibliothekstreffer nie auf der Startseite» | gilt | gilt weiter, ohne Ausnahme (N-70). Fassung 2 wollte sie für das Spotlight ablösen. |
| Spec `:67` «Beschaffungsstatus aus Radarr/Sonarr am Titel und in der Suche» | Nicht-Ziel | Ausnahme nur beim Wunsch (N-82). Am Titel der Bibliothek und in der Suche bleibt es Nicht-Ziel. |
| ADR 0010 | «Slouchcat never calls Radarr or Sonarr» | zwei eng gefasste Ausnahmen: Tag setzen (N-22), Warteschlange und Dateistand der gewünschten Titel lesen (N-81). Der Webhook-Empfang bleibt (N-80). |

### 8.6 Offene Fragen: geändert und neu

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-8 | Spotlight mit fehlenden Titeln | **Entschieden durch E-5:** keine fehlenden Titel im Spotlight. |
| F-20 | Startreihe für Wünsche | geändert, siehe F-34. |
| F-33 | Sofort-Import über Pfad und Kennung aus der Webhook-Meldung? | Ja, nach einer Messung (N-88). |
| F-34 | Startreihe «Erfüllte Wünsche» (nur Vorhandenes) statt «Deine Wünsche» (auch offene)? | Ja (N-74). Alternative: Reihe ganz weglassen. |

### 8.7 Stufen (ergänzt 4.7)

Redaktion: Stufe 3 («Spotlight») schrumpft auf den Abgleich über TVDB und IMDb und die fehlenden Einträge in
der Verwaltung. Der Download-Fortschritt kommt mit dem Profil-Tag in Stufe 4 (beide brauchen den Zugang zu
Radarr und Sonarr). F-33 wäre ein eigener Schritt nach der Messung.

### 8.8 Mockup-Seiten (ergänzt Abschnitt 7)

| Seite | Was |
|---|---|
| M-01 Startseite | Spotlight nur mit Vorhandenem, wie heute; Reihe «Erfüllte Wünsche» |
| M-02 Spotlight-Liste | unverändert gegenüber heute: nur Titel der Bibliothek |
| M-08 Meine Wünsche, Wunschkarte auf M-03/M-04 | Download-Fortschritt |
| M-22 Integrationen › Jellyseerr | Band «Radarr und Sonarr (Profil-Tags und Fortschritt)», Prüfungen umbenannt |
| M-23 Anfragen | Fortschritt je Wunsch, Meldung von Radarr oder Sonarr in den Einzelheiten |
| M-25 Einstellungen | Schalter «Aus dem Spotlight» entfällt; neu «Download-Fortschritt zeigen» |
| M-27 Spotlight | Band «Fehlende Einträge» ohne Schalter; Einträge und IDs wie in Fassung 3 |
| M-28 Startseite | Reihe «Erfüllte Wünsche» |
| M-30 Hintergrundjobs | `wishes.progress` |
| M-00 Steuerung | «Download weiter», «Download hängt» |

---

## 9. Kritik an Fassung 3 und Redaktionsentscheide

Ein Kritiker hat die Papiere der Fassung 3 gegen dieses Blatt, `CLAUDE.md`, den Vertrag und das Mockup gelesen.
Urteil: **REVISE**, 1 Blocker, 4 Major, 18 Minor. Alle 23 Befunde sind eingearbeitet. Die Entscheide in diesem
Abschnitt hat die Redaktion getroffen; **Marco hat dazu nichts entschieden.** Wo ein Entscheid eine Antwort von
Marco berührt, steht es dabei.

| Nr. | Schwere | Befund | Redaktionsentscheid |
|---|---|---|---|
| K-1 | Blocker | **Übernahme eines fremden Jellyseerr-Benutzers.** Wer eine Einladung annimmt, tippt seine E-Mail frei ein; niemand prüft sie. Mit N-2 würde Slouchcat einen bestehenden Jellyseerr-Benutzer mit dieser Adresse verknüpfen und ihm das neue Passwort setzen. Dasselbe ginge über eine geänderte E-Mail. Das Mockup verriet zudem, ob es die Adresse in Jellyseerr gibt. | **N-2a (neu):** Automatisch verknüpft wird nur, wenn ein Administrator handelt («In Jellyseerr eröffnen», «Alle eröffnen») oder bestätigt. Findet Slouchcat beim Annehmen einer Einladung oder nach einer E-Mail-Änderung des Kontos einen Benutzer mit gleicher Adresse, entsteht nur ein **Vorschlag** (Zustand «Verknüpfung vorgeschlagen»). Bis ein Administrator ihn unter «Jellyseerr-Benutzer» bestätigt, gibt es keine Verknüpfung, kein Passwort an den bestehenden Benutzer und kein Anfordern. «Alle eröffnen» bestätigt keine Vorschläge. Das Konto erfährt nie, ob Jellyseerr die Adresse kennt. Das schränkt E-4 für diesen einen Fall ein; E-4 fragte nach Konten und Benutzern, «die es heute schon gibt», und dort verknüpft weiter der Knopf des Administrators. Offen als Teil von F-24. |
| K-2 | Major | Konto löschen: N-9 («Rechte von Benutzern, die es schon gab, fasst Slouchcat nicht an») gegen N-10. | N-9 gilt. **N-10 neu:** Nur einem Benutzer, den Slouchcat eröffnet hat, nimmt Slouchcat die Rechte und setzt ein Zufallspasswort. Verknüpfte und von Hand gewählte Benutzer bleiben unverändert. |
| K-3 | Major | Der Chip «Dein Wunsch ist da» stand auch auf der Spotlight-Bühne. | N-70 gilt: Die Bühne bleibt wie heute, ohne Wunsch-Chip und ohne Staffelhinweis. Der Chip steht auf der Detailseite (M-05) und in der Reihe «Erfüllte Wünsche». |
| K-4 | Major | Die Staffelliste für eine Serie der Bibliothek hatte keinen Weg im Vertrag. | `getWishSeasons` antwortet auch für eine vorhandene Serie mit fehlenden Staffeln; ihre 404-Fälle stehen eigens in `08`. |
| K-5 | Major | Die Ausnahme in Regel 11 und ADR 0010 nannte nur die Warteschlange. «Wird gesucht» und «Wird eingelesen» brauchen aber auch den Dateistand des Titels. | **N-81 genauer:** Slouchcat liest in Radarr und Sonarr die Warteschlange und den Dateistand der gewünschten Titel (`queue`, `movie?tmdbId`, `series?tvdbId`) und setzt den Profil-Tag. Mehr nicht. Der Satz «Nach Importen fragt Slouchcat nicht ab» heisst richtig: Der Import in die Bibliothek kommt weiter über den Webhook; gelesen wird nur für die Anzeige am Wunsch. |
| K-6 | Minor | `09` zitierte 8.7 mit «Stufe 2». | Satz gestrichen; es ist Stufe 3. |
| K-7 | Minor | `07` gab der Seite «Webhooks» einen neuen Einleitungssatz, `11` nannte ihn Vorschlag; N-80 sagt «unverändert». | Die Seite bleibt wörtlich, wie sie ist. Kein neuer Satz. |
| K-8 | Minor | `GET /api/v3/queue` liefert Seiten. | Alle Seiten lesen oder `pageSize` setzen; vor dem Bau gegen die Instanz prüfen (nicht geprüft). |
| K-9 | Minor | `01` Abschnitt 20 nummerierte seine Einschränkungen E-1 bis E-…; das kollidiert mit Marcos Antworten E-1 bis E-7 und mit Spec-IDs. | Die Einschränkungen heissen neu TE-1 bis TE-…; Spec-IDs tragen das Wort «Spec» davor. E-1 bis E-7 sind nur noch Marcos Antworten. |
| K-10 | Minor | Zählung der offenen Fragen. | Es gibt F-1 bis F-34. F-8 ist durch E-5 entschieden, F-20 durch F-34 ersetzt: offen sind 32, dazu T-1 und T-2. |
| K-11 | Minor | F-24 las E-1 als «gilt für alle Konten». | E-1 fragte nach eingeladenen Benutzern. Dass der Abgleich auch für nachträglich eröffnete und verknüpfte Konten gilt, ist eine Lesart der Redaktion (F-24). |
| K-12 | Minor | `07` sagte «bei Abweichung gilt das Mockup». | Papier und Mockup sind angeglichen; bei Abweichung gilt das Papier. |
| K-13 | Minor | Routen des Bereichs. | Der Bereich liegt unter `/discover` («Entdecken», «Filme», «Serien», «Suche»); «Meine Wünsche» bleibt `/wishes`. |
| K-14 | Minor | «Bis dahin kann das Konto nicht anfordern» war unklar. | Anfordern geht, sobald der Jellyseerr-Benutzer besteht. Das Passwort spielt dafür keine Rolle. |
| K-15 | Minor | Eröffnet das Einrichten von Jellyseerr bestehende Konten von selbst? | Nein. Bestehende Konten eröffnet der Administrator mit «Alle eröffnen» oder einzeln (wie in E-4 beschrieben). Von selbst eröffnet nur das Annehmen einer Einladung. F-23 entsprechend. |
| K-16 | Minor | Die Reihe «Erfüllte Wünsche» braucht Daten aus den Wunsch-Tabellen, die nur `wishes` lesen darf. | Das Paket der Startseite bekommt die Titel-IDs vom Dienst `wishes` und prüft die Sichtbarkeit über `visibility` (Regel 2). |
| K-17 | Minor | AK-177 liess Mia mitwünschen, obwohl sie kein Wunschrecht hat. | Beispiel mit einem Profil, das anfordern darf (Noah). |
| K-18 | Minor | Zwei Profile mit gleichem Namen: Beide sollten einen Zusatz bekommen, aber Tags werden nie umbenannt. | **N-21 genauer:** Das erste Profil behält den einfachen Namen (`sc-kinder`), ein später angelegtes mit gleichem Namen bekommt den Kontonamen dazu (`sc-kinder-grosseltern`). |
| K-19 | Minor | Datenschutz: Alle Profile teilen das Jellyseerr-Login des Kontos; wer sich dort anmeldet, sieht auch die Wünsche eines PIN-Profils. Jellyseerr verschickt zudem eigene E-Mails mit Titel. | Als bekannte Grenze in `04` und `06`, als Hinweis in Wünsche › Einstellungen. Empfehlung: in Jellyseerr die eigenen Benachrichtigungen für eröffnete Benutzer ausgeschaltet lassen. |
| K-20 | Minor | ADR 0038 «Folgen» nannte die neue Startreihe nicht. | Ergänzt (F-34). |
| K-21 | Minor | Ein zurückgezogener Wunsch, dessen Request einem anderen gehört, wurde pauschal «In Beschaffung». | Der Zustand folgt dem Request (`pending` oder `processing`). |
| K-22 | Minor | `11` Abschnitt 0 nannte `internal/arr` «nur für Tags». | Tags und Lesen des Fortschritts. |
| K-23 | Minor | Die Schlüssel von Radarr und Sonarr würden jede Minute neu aus Jellyseerr geholt. | Im Speicher 10 Minuten halten; neu lesen erst nach Ablauf oder nach einer Antwort 401. |
