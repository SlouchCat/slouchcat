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
| N-2 | Gibt es in Seerr schon einen Benutzer mit derselben E-Mail (Vergleich ohne Gross-/Kleinschreibung), verknüpft Slouchcat ihn, statt einen zweiten anzulegen. Für Konten ohne Seerr-Benutzer gibt es «In Seerr eröffnen» (einzeln) und «Alle eröffnen». Eingeschränkt durch N-2a (Abschnitt 9, K-1): Tippt das Konto die Adresse selbst ein, entsteht nur ein Vorschlag, den ein Administrator bestätigt. | Marco (E-4), Einschränkung Redaktion |
| N-3 | Slouchcat speichert das Passwort **nie im Klartext**. Es gibt es nur in den drei Momenten weiter, in denen es der Server ohnehin sieht: Einladung annehmen, Passwort ändern oder zurücksetzen, Anmeldung. | Redaktion |
| N-4 | Kann Slouchcat das Passwort nicht sofort weitergeben (Seerr nicht erreichbar, Konto bestand schon, Benutzer nachträglich eröffnet), bekommt der Seerr-Benutzer zuerst ein Zufallspasswort und den Vermerk «bei der nächsten Anmeldung übernehmen». Bei der nächsten Anmeldung in Slouchcat setzt Slouchcat das Passwort in Seerr. | Redaktion |
| N-5 | Passwort ändern oder zurücksetzen in Slouchcat → sofort `POST /user/{id}/settings/password`. Schlägt das fehl, gilt N-4. E-Mail ändern in Slouchcat → auch in Seerr. | Marco (E-1), Ablauf Redaktion |
| N-6 | Kein Abgleich bei: Benutzer 1 (technisch nicht möglich, 3.1), anderen Administratoren von Seerr (bewusst ausgenommen, «bleibt in Seerr») und Benutzern, die sich über Jellyfin, Emby oder Plex anmelden. | Redaktion |
| N-7 | Ein Passwort, das jemand direkt in Seerr ändert, bemerkt Slouchcat nicht. Die Verwaltung bietet «Passwort bei der nächsten Anmeldung neu übernehmen». | Marco kannte die Folge (E-1), Knopf Redaktion |
| N-8 | Schalter in Wünsche › Einstellungen: «Neue Konten in Seerr eröffnen» und «Passwort abgleichen», beide ab Werk an, sobald Seerr eingerichtet ist. | Redaktion |
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
| N-41 | Die Zuordnung «Profil sendet als anderer Seerr-Benutzer» aus Fassung 2 bleibt als zugeklappte **Ausnahme**. Grund: Nur so braucht der Wunsch eines Kinderprofils eine Freigabe, wenn das Konto selbst automatisch freigegeben wird (etwa das Konto des Administrators). Für so ein Profil gelten Freigabe und Limit des Ausnahme-Benutzers. | Redaktion (Frage F-27) |

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
| F-23 | Für wen eröffnet Slouchcat einen Seerr-Benutzer? | Für jedes Konto, sobald Seerr eingerichtet ist; ohne Wunschrecht ohne Rechte in Seerr (N-9). |
| F-24 | Passwort bei verknüpften, schon bestehenden Seerr-Benutzern übernehmen? | Ja, bei der nächsten Anmeldung, ausser bei Administratoren und bei Anmeldung über Jellyfin/Plex (N-4, N-6). |
| F-25 | Name des Profil-Tags | `sc-<profilname>`, Präfix einstellbar, je Profil überschreibbar; Tags werden nie entfernt (N-21, N-26). |
| F-26 | Vorgabe des Limits ab Werk | 10 Filme in 7 Tagen, 5 Staffeln in 7 Tagen; Administratoren ohne Limit (N-31, N-34). |
| F-27 | Ausnahme «Profil sendet als anderer Seerr-Benutzer» behalten? | Ja, zugeklappt. Sonst sind Kinderprofile im Konto des Administrators ohne Freigabe und ohne Limit (N-41). |
| F-28 | Reihen auf «Entdecken» | Die sieben aus N-51, fest. Nicht die Reihen-Einstellung von Seerr übernehmen. |
| F-29 | Seerr auf TVDB-Staffeln umstellen? | Ja (N-64). Das stellt Marco in Seerr selbst um. |
| F-30 | Radarr und Sonarr: Zugang aus Seerr lesen oder eigene Felder? | Aus Seerr lesen, eigene Adresse nur als Ausweg (N-23). |
| F-31 | Was geschieht mit dem Seerr-Benutzer, wenn das Konto in Slouchcat gelöscht wird? | Bleibt, ohne Rechte, mit Zufallspasswort (N-10). |
| F-32 | Name in der Oberfläche: «Jellyseerr/Seerr»? Marco schreibt «Seerr», das Projekt heisst seit 3.0 so. | **Entschieden durch E-9 (Abschnitt 10):** «Seerr». Papiere und Mockup sind umbenannt. |

## 7. Neue und geänderte Mockup-Seiten

| Seite | Was |
|---|---|
| M-31 Wünsche · Entdecken (neu) | Reihen, Genre-Kacheln, Suchfeld |
| M-32 Wünsche · Filme (neu) | Raster, Filter, Sortierung, «Mehr laden» |
| M-33 Wünsche · Serien (neu) | wie M-32 |
| M-34 Wünsche · Suche (neu) | Suche, auch nach Kennung |
| M-35 Einladung annehmen (heute vorhanden, im Mockup neu) | Hinweis «Dein Login gilt auch für Seerr», Eröffnung beim Absenden |
| M-08 Meine Wünsche | Reiter, Limit-Anzeige |
| M-13 Dialog «Anfordern» | Limit: frei, erreicht, zu viele Staffeln |
| M-07 Suche | Verweis auf die Wunsch-Suche |
| M-20 Übersicht | Hinweise: Konto ohne Benutzer, Limit erreicht, Tag nicht gesetzt |
| M-22 Integrationen › Seerr | neue Prüfungen, Bänder «Benutzer und Limit», «Radarr und Sonarr (Profil-Tags)» |
| M-23 Anfragen | Profil-Tag je Wunsch, Band «Limit je Konto» |
| M-24 Seerr-Benutzer (vorher «Zuordnung») | je Konto: Zustand, Passwort, Rechte, Limit; je Profil: Tag und Ausnahme |
| M-25 Einstellungen | Limit ab Werk, Konten in Seerr, Profil-Tag, Schalter für die neuen Seiten |
| M-26 Konten | Limit je Konto, Seerr-Benutzer, Einladung mit Limit, Passwortänderung |
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
| M-22 Integrationen › Seerr | Band «Radarr und Sonarr (Profil-Tags und Fortschritt)», Prüfungen umbenannt |
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
| K-1 | Blocker (von Marco anders entschieden, siehe Abschnitt 10, E-8) | **Übernahme eines fremden Seerr-Benutzers.** Wer eine Einladung annimmt, tippt seine E-Mail frei ein; niemand prüft sie. Mit N-2 würde Slouchcat einen bestehenden Seerr-Benutzer mit dieser Adresse verknüpfen und ihm das neue Passwort setzen. Dasselbe ginge über eine geänderte E-Mail. Das Mockup verriet zudem, ob es die Adresse in Seerr gibt. | **N-2a (neu):** Automatisch verknüpft wird nur, wenn ein Administrator handelt («In Seerr eröffnen», «Alle eröffnen») oder bestätigt. Findet Slouchcat beim Annehmen einer Einladung oder nach einer E-Mail-Änderung des Kontos einen Benutzer mit gleicher Adresse, entsteht nur ein **Vorschlag** (Zustand «Verknüpfung vorgeschlagen»). Bis ein Administrator ihn unter «Seerr-Benutzer» bestätigt, gibt es keine Verknüpfung, kein Passwort an den bestehenden Benutzer und kein Anfordern. «Alle eröffnen» bestätigt keine Vorschläge. Das Konto erfährt nie, ob Seerr die Adresse kennt. Das schränkt E-4 für diesen einen Fall ein; E-4 fragte nach Konten und Benutzern, «die es heute schon gibt», und dort verknüpft weiter der Knopf des Administrators. Offen als Teil von F-24. |
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
| K-14 | Minor | «Bis dahin kann das Konto nicht anfordern» war unklar. | Anfordern geht, sobald der Seerr-Benutzer besteht. Das Passwort spielt dafür keine Rolle. |
| K-15 | Minor | Eröffnet das Einrichten von Seerr bestehende Konten von selbst? | Nein. Bestehende Konten eröffnet der Administrator mit «Alle eröffnen» oder einzeln (wie in E-4 beschrieben). Von selbst eröffnet nur das Annehmen einer Einladung. F-23 entsprechend. |
| K-16 | Minor | Die Reihe «Erfüllte Wünsche» braucht Daten aus den Wunsch-Tabellen, die nur `wishes` lesen darf. | Das Paket der Startseite bekommt die Titel-IDs vom Dienst `wishes` und prüft die Sichtbarkeit über `visibility` (Regel 2). |
| K-17 | Minor | AK-177 liess Mia mitwünschen, obwohl sie kein Wunschrecht hat. | Beispiel mit einem Profil, das anfordern darf (Noah). |
| K-18 | Minor | Zwei Profile mit gleichem Namen: Beide sollten einen Zusatz bekommen, aber Tags werden nie umbenannt. | **N-21 genauer:** Das erste Profil behält den einfachen Namen (`sc-kinder`), ein später angelegtes mit gleichem Namen bekommt den Kontonamen dazu (`sc-kinder-grosseltern`). |
| K-19 | Minor | Datenschutz: Alle Profile teilen das Seerr-Login des Kontos; wer sich dort anmeldet, sieht auch die Wünsche eines PIN-Profils. Seerr verschickt zudem eigene E-Mails mit Titel. | Als bekannte Grenze in `04` und `06`, als Hinweis in Wünsche › Einstellungen. Empfehlung: in Seerr die eigenen Benachrichtigungen für eröffnete Benutzer ausgeschaltet lassen. |
| K-20 | Minor | ADR 0038 «Folgen» nannte die neue Startreihe nicht. | Ergänzt (F-34). |
| K-21 | Minor | Ein zurückgezogener Wunsch, dessen Request einem anderen gehört, wurde pauschal «In Beschaffung». | Der Zustand folgt dem Request (`pending` oder `processing`). |
| K-22 | Minor | `11` Abschnitt 0 nannte `internal/arr` «nur für Tags». | Tags und Lesen des Fortschritts. |
| K-23 | Minor | Die Schlüssel von Radarr und Sonarr würden jede Minute neu aus Seerr geholt. | Im Speicher 10 Minuten halten; neu lesen erst nach Ablauf oder nach einer Antwort 401. |


---

## 10. Dritte Runde: vier Weichen (17. November 2026)

Marco fragte nach Fassung 3: «Hast du noch offene Fragen oder Unklarheiten?» Vier Weichen wurden ihm
vorgelegt. Dieser Abschnitt gilt vor den Abschnitten 4 bis 9, wo er ihnen widerspricht.

### 10.1 Rückfragen und Marcos Antworten

| Nr. | Frage | Zur Wahl standen | Marcos Antwort |
|---|---|---|---|
| E-8 | Nimmt jemand eine Einladung an und tippt eine E-Mail ein, die es in Seerr schon gibt: Soll Slouchcat den bestehenden Seerr-Benutzer sofort verknüpfen oder erst nach deiner Bestätigung? | «Erst nach Bestätigung» (empfohlen) · «Sofort verknüpfen» | **Sofort verknüpfen** |
| E-9 | Wie soll der Dienst in der Oberfläche von Slouchcat heissen? | «Seerr» · «Jellyseerr» | **Seerr** |
| E-10 | Die Startseite zeigt nur Vorhandenes. Was soll mit der neuen Startreihe für Wünsche geschehen? | «Erfüllte Wünsche» · «Keine Reihe» · «Auch offene Wünsche» | **Erfüllte Wünsche** |
| E-11 | Die Webhook-Meldung von Radarr und Sonarr nennt schon Dateipfad und Kennung. Soll Slouchcat das für einen Sofort-Import nutzen? | «Ja, nach Messung» · «Ja, fest einplanen» · «Nein» | **Ja, nach Messung** |

Die Beschreibungen der gewählten Antworten, wie Marco sie gesehen hat:

- **E-8:** Wie in deiner früheren Antwort, auch bei Einladungen. Einfacher, aber wer eine fremde E-Mail
  eintippt, bekommt den Seerr-Benutzer dieser Person samt ihren Anfragen.
- **E-9:** So heisst das Projekt seit Version 3, und so schreibst du es. Ich benenne Papiere und Mockup um.
- **E-10:** Die Reihe zeigt nur Wünsche, die in den letzten 14 Tagen wahr wurden, also Titel der Bibliothek.
  Offene Wünsche stehen unter «Wünsche».
- **E-11:** Kommt als eigener Schritt in den Bauplan. Zuerst wird gemessen, wo heute die Zeit bleibt. Die
  Webhooks selbst bleiben, wie sie sind.

### 10.2 Was gilt

| Nr. | Regel | Von |
|---|---|---|
| N-90 | **N-2a (Abschnitt 9, K-1) ist zurückgezogen.** Gibt es in Seerr schon einen Benutzer mit derselben E-Mail, verknüpft Slouchcat ihn sofort, auch wenn das Konto die Adresse beim Annehmen einer Einladung selbst eingetippt hat. Den Zustand «Verknüpfung vorgeschlagen», die Aktion «Verknüpfung bestätigen» und die Operation dafür gibt es nicht. | Marco (E-8) |
| N-91 | Mit der Verknüpfung geht das Passwort an den bestehenden Benutzer, wie bei jedem anderen (E-1, N-3 bis N-5). Ausgenommen bleiben Benutzer 1, andere Administratoren von Seerr und Benutzer mit Anmeldung über Jellyfin, Emby oder Plex (N-6). Ein Administrator von Seerr lässt sich so also nicht übernehmen. | folgt aus E-1 und E-8; Ausnahme N-6 Redaktion |
| N-92 | Das Risiko ist bekannt und von Marco so gewählt: Wer eine Einladung hat und eine fremde Adresse eintippt, bekommt den Seerr-Benutzer dieser Person samt ihren Anfragen. Einladungen stellt nur ein Administrator aus. Das Papier nennt das Risiko als bekannte Grenze. | Marco (E-8) |
| N-93 | Damit so eine Verknüpfung nicht unbemerkt bleibt: Die Übersicht der Verwaltung zeigt sieben Tage lang den Hinweis «Neu verknüpft: {{Konto}} mit dem bestehenden Seerr-Benutzer {{Name}}», mit Link auf «Seerr-Benutzer», wo sich die Verknüpfung lösen lässt. Das Protokoll hält sie fest. | Redaktion (Empfehlung; rest von F-24) |
| N-94 | Das neue Konto erfährt weiterhin nicht, ob Seerr die Adresse schon kannte: Die Meldung nach dem Annehmen ist in beiden Fällen dieselbe («In Seerr hast du jetzt ein Konto mit derselben E-Mail-Adresse.»). Sie nennt das Passwort nicht, weil Slouchcat es bei einem verknüpften Administrator oder bei Anmeldung über Jellyfin, Emby oder Plex nicht setzt (N-91). | Redaktion |
| N-95 | Ändert ein Konto ohne Seerr-Benutzer seine E-Mail auf eine Adresse, die es in Seerr gibt, gilt dasselbe wie bei der Einladung: sofort verknüpfen. Hat das Konto schon einen Seerr-Benutzer, ändert Slouchcat dessen E-Mail; lehnt Seerr ab, weil die Adresse vergeben ist, meldet Slouchcat das und lässt den Benutzer, wie er ist. | Redaktion (folgt E-8) |
| N-96 | In der Oberfläche heisst der Dienst «Seerr». Papiere und Mockup sind umbenannt. «Jellyseerr» steht nur noch, wo eine Version 2.x gemeint ist, und in den eingefrorenen Anhängen A bis G. Das Konzept heisst «Wünsche über Seerr». | Marco (E-9) |
| N-97 | Die Startreihe heisst «Erfüllte Wünsche» und zeigt nur Wünsche, die in den letzten 14 Tagen wahr wurden (N-74). | Marco (E-10) |
| N-98 | Der Sofort-Import (N-88) kommt als eigener Schritt in den Bauplan: Slouchcat liest Dateipfad und Kennung aus der Webhook-Meldung, liest nur diese Datei mit Vorrang ein und ordnet sie über die Kennung zu. Vorher wird gemessen, wo heute die Zeit bleibt (`docs/perf`); zeigt die Messung keinen Gewinn, entfällt der Schritt. Die Webhooks bleiben, wie sie sind (N-80). | Marco (E-11) |

### 10.3 Fragen

| Nr. | Stand |
|---|---|
| F-24 | Die Verknüpfung bei der Einladung ist durch E-8 entschieden: sofort. Offen bleibt als Empfehlung: Passwort auch bei nachträglich eröffneten und verknüpften Konten übernehmen (K-11) und der Hinweis an die Administratoren (N-93). |
| F-32 | Entschieden durch E-9: «Seerr». |
| F-33 | Entschieden durch E-11: ja, nach Messung. |
| F-34 | Entschieden durch E-10: «Erfüllte Wünsche». |

Offen sind damit 29 Fragen (F-1 bis F-34 ohne F-8, F-20, F-32, F-33, F-34) und T-1, T-2.

### 10.4 Mockup

Der Zustand «Verknüpfung vorgeschlagen» ist entfernt; M-35 verknüpft sofort und zeigt in beiden Fällen dieselbe
Meldung; M-20 zeigt den Hinweis «Neu verknüpft»; überall steht «Seerr».

---

## 11. Bauauftrag und Antworten auf die offenen Fragen (17. November 2026)

Marco hat am 17. November 2026 den Bau bestellt: «Starte mit dem Bau mit Konzept 25. Mach alles in einem Zug und
voll automatisch. Falls du CI Test machst prüfe regelmässig ob diese fertig. Falls du doch noch fragen hast, stell
diese.» Auf die Rückfrage, wie mit den 29 offenen Fragen umzugehen sei, wählte er «Zeig sie mir zuerst» und hat
sie danach alle beantwortet. Dieser Abschnitt gilt vor allen früheren, wo er ihnen widerspricht.

### 11.1 Antworten zum Ablauf

| Nr. | Frage | Marcos Antwort |
|---|---|---|
| B-1 | Wie veröffentlichen? | **Ein Release am Schluss:** alle Stufen in einem Zweig, ein Pull Request, eine Version (ersetzt die Empfehlung zu F-22). |
| B-2 | Darf die Bau-Session selbst mergen und taggen? | **Nein, Marco gibt frei.** Gebaut wird bis zum grünen Pull Request; ein Zweig-Image für CT 190 ist möglich. |
| B-3 | Prüfen an Marcos echtem Seerr? | **Auch schreibend:** ein Testbenutzer wird angelegt und wieder gelöscht; kein echter Wunsch. Adresse und Schlüssel trägt Marco in `deploy/.env` ein (`SEERR_URL`, `SEERR_API_KEY`). |
| B-4 | Schreibend in Radarr und Sonarr testen? | **Ja:** ein Tag «sc-test», an keinen Titel gehängt, danach gelöscht. |

### 11.2 Antworten auf die offenen Fragen

Die Spalte «Empfehlung?» sagt, ob Marco der Empfehlung des Papiers gefolgt ist.

| Frage | Marcos Antwort | Empfehlung? |
|---|---|---|
| F-1 | «Wünsche» für die Funktion, «anfordern» auf Knöpfen | ja |
| F-2 | nicht eigens gefragt: in der Sache durch E-1 (ein Seerr-Benutzer je Konto) und F-27 (keine Ausnahme je Profil) geklärt | – |
| F-3 | Slouchcat handelt als der Benutzer (`X-API-User`), der Verbindungstest prüft es | ja |
| F-4 | **Sammelbenutzer ab Werk.** Slouchcat legt in Seerr den Benutzer «slouchcat-gaeste» an (ohne Anmeldung, ohne automatische Freigabe); über ihn senden Konten ohne eigenen Seerr-Benutzer. Für ihn gilt das Standard-Limit, gemeinsam für alle diese Konten | **nein** |
| F-5 | Wunschrecht je Konto (Administrator, Deckel) und je Profil (Hauptprofil); ab Werk Besitzerkonto und Hauptprofile an, neue Konten und weitere Profile aus | ja |
| F-6 | Kinderprofile mit Recht dürfen anfordern; die Freigabe entscheidet Seerr am Benutzer des Kontos | ja |
| F-7 | Mitwünschen | ja |
| F-9 | Wer nicht anfordern darf, sieht fehlende Titel und den Bereich «Wünsche» gar nicht; Ausnahme eigene Wünsche | ja |
| F-10 | Anfordern über einen Dialog | ja |
| F-11 | Bibliothekssuche über TMDB mit eigenem Schlüssel, Seite «Suche» über Seerr | ja |
| F-12 | Serien staffelweise | ja |
| F-13 | «Verfügbar» heisst: Slouchcat kann den Titel für das Profil abspielen | ja |
| F-14 | Drei eigene Kanal-Ereignisse: «verfügbar» (an), «freigegeben, abgelehnt, fehlgeschlagen» (aus), «Freigabe nötig» (an, nur Administratoren) | ja |
| F-15 | Externe Nachrichten von PIN-Profilen ohne Titel, abschaltbar | ja |
| F-16 | «Freigeben» und «Ablehnen» in der Verwaltung, als Durchreichen an Seerr | ja |
| F-17 | Zurückziehen, solange nicht freigegeben | ja |
| F-18 | Seerr-Webhook als Anstoss; der Abgleich alle 5 Minuten bleibt die Grundlage | ja |
| F-19 | Verbindung unter Integrationen, tägliche Arbeit unter Medien › Wünsche | ja |
| F-21 | Bestehende Anfragen verknüpfter Seerr-Benutzer übernehmen («In Seerr angelegt») | ja |
| F-22 | **Ein Release am Schluss** (B-1) | **nein** |
| F-23 | Für jedes Konto ein Seerr-Benutzer; ohne Wunschrecht ohne Rechte; bestehende Konten über «Alle eröffnen» | ja |
| F-24 | **Passwort nur für eingeladene und von Slouchcat eröffnete Benutzer.** Ein bestehender Seerr-Benutzer, den ein Administrator verknüpft, behält sein Passwort. (Die Verknüpfung beim Annehmen einer Einladung bleibt E-8: sofort, mit Passwort.) | **nein** |
| F-25 | Tag `sc-<profilname>`, Präfix einstellbar, je Profil überschreibbar, nie entfernt | ja |
| F-26 | Limit ab Werk: 10 Filme und 5 Staffeln je 7 Tage | ja |
| F-27 | **Keine Ausnahme je Profil.** Alle Profile senden als Benutzer des Kontos. Die Einstellung «Profil sendet als anderer Seerr-Benutzer» wird nicht gebaut | **nein** |
| F-28 | «Entdecken» mit sieben festen Reihen | ja |
| F-29 | Marco stellt Seerr auf TVDB-Staffeln um; Slouchcat ändert die Einstellung nie, der Test zeigt einen Hinweis | ja |
| F-30 | Zugang zu Radarr und Sonarr aus Seerr lesen; die Adresse gilt nach einem «Testen», eigene Adresse als Ausweg | ja |
| F-31 | Der Seerr-Benutzer eines gelöschten Kontos bleibt; ein eröffneter ohne Rechte und mit Zufallspasswort, ein verknüpfter unverändert | ja |
| T-1 | «Wünsche aktiv» lässt sich nur nach einem gelungenen Test einschalten; ab Werk aus | ja |
| T-2 | Die Frist der Übermittlung ruht, solange Seerr in Slouchcat ausgeschaltet ist | ja |

Damit ist keine Frage mehr offen.

### 11.3 Was die vier abweichenden Antworten ändern

| Nr. | Regel | Von |
|---|---|---|
| N-100 | **Keine Zuordnung je Profil (F-27).** Die Tabelle `wish_profile_map`, die Auswahl «Sendet als» und alle Regeln dazu entfallen. Ein Wunsch geht immer als der Seerr-Benutzer des Kontos, sonst als Sammelbenutzer. Folge, die Marco kennt: Profile im Konto eines Seerr-Administrators fordern ohne Freigabe und ohne Limit an; begrenzen lässt sich das nur über das Wunschrecht des Profils. | Marco (F-27) |
| N-101 | **Sammelbenutzer ab Werk (F-4).** Sobald Seerr eingerichtet und «Wünsche aktiv» an ist, legt Slouchcat in Seerr den lokalen Benutzer «slouchcat-gaeste» an (E-Mail `slouchcat-gaeste@slouchcat.invalid`, Zufallspasswort, Recht `REQUEST`, keine automatische Freigabe) und schreibt ihm das Standard-Limit. Konten ohne eigenen Seerr-Benutzer («nicht eröffnet», «wartet») senden über ihn. Der Schalter «Konten ohne Seerr-Benutzer» steht ab Werk auf «Sammelbenutzer»; die andere Wahl ist «Nicht erlauben». Einen anderen Benutzer als Sammelbenutzer zu wählen, ist nicht nötig und entfällt. | Marco (F-4), Einzelheiten (E-Mail, Zeitpunkt) Redaktion |
| N-102 | Wünsche über den Sammelbenutzer merkt sich Slouchcat wie alle anderen mit Konto und Profil; in Seerr steht «slouchcat-gaeste». Bekommt das Konto später einen eigenen Benutzer, bleiben seine alten Wünsche beim Sammelbenutzer. Übernommen (F-21) werden Anfragen des Sammelbenutzers nicht. | Redaktion |
| N-103 | **Passwort (F-24).** Slouchcat setzt das Passwort in Seerr nur bei Benutzern, die es selbst eröffnet hat (beim Annehmen einer Einladung, über «In Seerr eröffnen», über «Alle eröffnen»), und bei einem bestehenden Benutzer, der beim Annehmen einer Einladung oder nach einer E-Mail-Änderung des Kontos verknüpft wurde (E-8). Verknüpft ein Administrator ein bestehendes Konto über die E-Mail oder von Hand mit einem bestehenden Seerr-Benutzer, steht der Passwortstand auf «Wird nicht abgeglichen»; er kann den Abgleich je Konto einschalten. | Marco (F-24), Schalter je Konto Redaktion |
| N-104 | **Ein Release (F-22, B-1).** Die Stufen 0 bis 5 bleiben die Reihenfolge des Baus, aber es gibt einen Zweig, einen Pull Request und eine Version. Ein eingefrorener Datenbankstand am Schluss genügt. | Marco (B-1) |

---

## 12. Nach den Prüfungen des Baus

Dieser Abschnitt ändert keine der Antworten oben und keinen der älteren Abschnitte. Er zeigt nur, wo der Bau
eine Regel dieses Anhangs nach den Prüfungen vom 17. November 2026 enger gefasst oder anders benannt hat und wo
das ausgeschrieben steht. Entschieden hat das die Redaktion, nicht Marco.

| Thema | Was sich ändert | Von | Ausgeschrieben in |
|---|---|---|---|
| Verknüpfen privilegierter Seerr-Benutzer (E-8, N-90, N-91, N-95) | Beim Annehmen einer Einladung und nach einer E-Mail-Änderung verknüpft Slouchcat einen gefundenen Seerr-Benutzer **nicht**, wenn er Benutzer 1 ist oder eines der Rechte ADMIN, MANAGE_USERS, MANAGE_SETTINGS oder MANAGE_REQUESTS hat. Das Konto wartet mit dem Grund `seerr_user_privileged`; ein Administrator kann den Benutzer von Hand wählen (dann ohne Passwort- und E-Mail-Abgleich). Das engt E-8 («sofort verknüpfen») für diese Benutzer ein. Grund ist die Zusage aus N-91, dass sich ein Administrator von Seerr so nicht übernehmen lässt: Sie hielt nur für das Passwort, nicht für die Verknüpfung samt Anfragen | Sicherheitskorrektur der Redaktion nach der Sicherheitsprüfung des Baus; Marco ist informiert (Meldung der Bau-Session vom 17. November 2026) | `03-architektur.md` Abschnitte 15.4, 18.1, 18.2; `04-benutzer-profilmapping.md` Abschnitte 12.2, 12.3, 12.5; ADR 0037, Nachtrag |
| Weitere Schutzregeln für Passwort und E-Mail | jedes Verwaltungsrecht zählt als «Administrator»; frische Prüfung vor jedem Setzen; eine Übergabe je Konto zugleich; Domain `slouchcat.invalid` für Konten gesperrt; «gleicher Host» vergleicht auch das Schema; kein Senden als ein Seerr-Benutzer, der inzwischen einem anderen Konto gehört oder privilegiert ist | Redaktion (Sicherheitsprüfung) | `03-architektur.md` Abschnitt 15.4; `04-benutzer-profilmapping.md` Abschnitt 13.3 |
| Namen der Umgebungsvariablen (B-3) | Der Server liest `SLOUCHCAT_SEERR_URL` und `SLOUCHCAT_SEERR_API_KEY`. `SEERR_URL` und `SEERR_API_KEY` aus B-3 heissen so nur in `deploy/.env` des Entwicklungsrechners, für die Prüfskripte von Anhang I | so gebaut | `07-administration.md` Abschnitt 4.1; Anhang I Abschnitt 4b |
| Abweichungen vom Mockup | «Titel A–Z» entfällt; keine Altersfreigabe auf der Vorschauseite; Nachladen beim Scrollen statt «Mehr laden»; «Seerr-Benutzer» als Tabelle | Redaktion bzw. so gebaut (Gründe dort) | `mockup-abdeckung.md` Abschnitt 11 |
