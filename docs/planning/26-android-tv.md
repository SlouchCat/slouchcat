# Konzept: Android-TV-App – native App für den Fernseher mit eigenem Player, Kopplung per Code und Mehrkanalton

- **Status: Entwurf, Fassung 3 (19. November 2026). Nicht freigegeben.** Fassung 2 hat das
  Kritiker-Review der Fassung 1 eingearbeitet (REVISE, 0 Blocker, 9 Major, 8 Minor), Fassung 3
  die zweite Lesung (REVISE, 0 Blocker, 2 Major, 6 Minor); beides in Abschnitt 14. Gebaut wird
  erst nach Marcos Freigabe und auf seinen eigenen Auftrag je Stufe.
- **Auslöser:** Marco am 19. November 2026: «Kannst du mir ein slouchcat überall Accounts erstellen
  welche ich für die künftige AndroidTV App benötige?» und danach «Wir starten mit AndroidTV». Auf
  die Rückfrage, was damit gemeint ist, hat er «Konzept starten» gewählt.
- **Verankert gegen:** `main` **`d70c1c79`** («Merge pull request #188», Release 1.41.2),
  Vertragsfassung **1.41.0** (`api/openapi.yaml:4`), letzte Migration
  **`00081_wishes_request_open.sql`**, letzte ADR **`0038`**, letzte Frage im
  Entscheidungsprotokoll **33**.
- **Nummern:** Dieses Papier ist **Konzept 26** und reserviert **Frage 34** sowie **ADR 0039**
  («Android-TV-App als nativer Client im selben Repo»), **ADR 0040** («Gerätekopplung und
  Gerätesitzung») und **ADR 0041** («Ton- und Spurfähigkeiten des Geräts in der
  Wiedergabe-Entscheidung»). Die ADR entstehen mit dem Bau. Migrations- und Versionsnummern
  vergibt der Bau.
- Pfade ohne Präfix liegen unter `web/src/` oder `server/`. **Nach Namen suchen, nicht nach
  Zeile**; Zeilen beziehen sich auf den Anker.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
  Dieses Papier schlägt Änderungen an früheren Entscheiden und an `CLAUDE.md` vor (Abschnitt 11);
  es ändert selbst keine Regel.

> **Dieses Papier ändert keine Zeile Code.** Marcos Entscheide stehen in Abschnitt 1 (E-1 bis
> E-7) und nur dort. Alles andere ist Vorschlag der Redaktion, auch wo es nicht eigens dabeisteht.
> Aussagen über den Code stammen vom Anker, Aussagen über Android und Google Play aus einer
> Recherche vom 19. November 2026 (Anhang A). Was dort nicht belegt werden konnte, heisst im Text
> «unbelegt» und wird in Stufe 0 auf echten Geräten gemessen.

## 1. Was Marco will

Marco hat am 19. November 2026 sechs Weichen über Rückfragen gestellt. Die Tabelle nennt die
gewählte Antwort wörtlich.

| Nr. | Frage | Marcos Wahl | Empfehlung war |
|---|---|---|---|
| E-1 | Auf welchen Geräten soll die App laufen und getestet werden? | «Chromecast / Google TV Streamer», «Nvidia Shield TV», «Fernseher mit Google TV». Fire TV nicht gewählt. | keine |
| E-2 | Wer soll die App bekommen? | «Erst privat, später öffentlich» | keine |
| E-3 | Wie soll die App gebaut sein? | **«Ganz nativ»** | Mischform (Web-Oberfläche, nativer Player) |
| E-4 | Wie meldet man sich am Fernseher an? | «Code und Passwort» | Code und Passwort |
| E-5 | Was kann die erste Ausgabe ausser Filmen und Serien? | «Hörbücher und Podcasts», «Webradio», «Wünsche über Seerr» | keine |
| E-6 | Mehrkanalton unverändert an eine Tonanlage weitergeben? | «Ja, von Anfang an» | Ja, von Anfang an |
| E-7 | (ohne Frage, von Marco selbst) | «Die AndroidTV App braucht auch kein Admin Bereich. Nur der Nutzer Bereich» | – |

Zu E-5: Die Frage hat vorgegeben, dass Filme und Serien gesetzt sind. Dass die Verwaltung im
Browser bleibt, war zuerst ebenfalls Vorgabe der Frage; Marco hat es danach mit E-7 selbst
festgelegt.

Dazu Marcos Hinweis vom selben Tag: «Da dies für alle verfügbar sein soll, muss ja auch irgendwo
der Server eingetragen werden können.» Das ist Abschnitt 4.1.

Stand der Konten am selben Tag: Für das Projekt gibt es ein eigenes Google-Konto und darauf ein
Entwicklerkonto in der Google Play Console (privates Konto). Google prüft die Identität noch; bis
dahin lässt sich dort keine App anlegen. Ein Entwicklerkonto bei Amazon gibt es nicht.

## 2. Heute

### 2.1 Regeln

TV-Apps und native Apps sind heute ein Nicht-Ziel:

- Spec, Abschnitt «Non-Goals»: «TV-Apps (Android TV, Apple TV, webOS, Tizen) und native iOS-/Android-Apps
  (spätere Versionen).» Die Übersichtstabelle der Spec nennt sie «zurückgestellt (Runde 3)».
- `CLAUDE.md`, Regel 11, zählt «TV-Apps, native Apps» unter den Nicht-Zielen auf.
- Plan, Abschnitt 1 («Nicht eingeplant»), führt TV-Apps und native Apps ebenfalls auf.

Die Spec hat den Weg aber offen gehalten. Kriterium C-5 verlangt einen Vertrag, der «so vollständig
[ist], dass ein Fremd-Client alle Zuschauerfunktionen umsetzen kann», Plan P-16 prüft das mit
einem Fremd-Client, der nur `openapi.yaml` kennt, und Plan 3.8 Punkt 7 sieht für solche Clients
Bearer-Token vor.

### 2.2 Anmeldung und Sitzung

- Anmelden: `login` (`POST /auth/login`) mit E-Mail und Passwort. Die Sitzung ist ein Cookie
  (`slouchcat_session`, `HttpOnly`, `SameSite=Lax`, `Secure` nur über TLS), 30 Tage gleitend
  (`SessionLifetime`, `server/internal/auth/auth.go`).
- Ein zweites Cookie `slouchcat_device` (400 Tage) ist die Gerätekennung. Eine Sitzung gehört zu
  dem Gerät, von dem die Anmeldung kam.
- Nach dem Anmelden hat die Sitzung noch kein Profil (ADR 0005). Profilwahl: `listMyProfiles`,
  `selectProfile` mit PIN. Die PIN hat 4 bis 8 Ziffern. Fehlversuche zählen je Profil und Gerät
  der Sitzung, dazu gilt eine Obergrenze von 20 je Stunde und Profil über alle Geräte.
- Jede schreibende Anfrage braucht den Kopf `X-Requested-With: Slouchcat`, sonst 403
  `csrf_header_missing`. Eine Prüfung von `Origin` oder `Referer` gibt es nicht.
- **Bearer-Token** sind im Vertrag als Schema `bearerToken` angelegt, aber keine Operation nutzt
  sie und der Server liest den Kopf `Authorization` nicht. ADR 0005 hält fest, dass sie an
  derselben Sitzungstabelle hängen und die Profilwahl erben sollen.
- Eine Kopplung per Code oder QR gibt es nicht.
- **Kontoebene:** 24 Operationen tragen `account`. Darunter sind `updateAccount`, die
  Profilpflege (`createProfile`, `updateProfile`, `deleteProfile`, `setProfilePin`,
  `removeProfilePin`, `setOfflinePin`, `removeOfflinePin`), die Meldekanäle (`updateMyChannel`,
  `createTelegramLink`, `createPushSubscription` und ihre Gegenstücke) und `getWatchReport`.
  Die Profilpflege verlangt nur, dass das Hauptprofil aktiv ist; hat es keine PIN, steht sie jeder
  Sitzung des Kontos offen.
- **Geräteliste:** `listDevices` und `deleteDevice` sind `admin`. Ein Konto ohne
  Verwaltungsrechte kann seine Geräte weder sehen noch abmelden. Name und Typ eines Geräts
  schreibt der Server erst beim ersten Abspielstart. `deleteDevice` entzieht die Sitzungen,
  beendet laufende Streams und setzt die PIN-Zähler zurück. Der Typ `tv` ist in der Geräteliste
  der Verwaltung schon vorgesehen (`DevicesPage.tsx`).
- **Benachrichtigungen** gehören zu einem Profil, nicht zum Konto, und jede Art verlangt einen
  Titel oder einen Wunsch (CHECK in Migration `00070`). Eine Meldung «beim Konto» gibt es nicht.
- **Adresse des Aufrufers:** Ohne eingetragene `TrustedProxies` sieht der Server hinter einem
  Reverse-Proxy nur die Adresse des Proxys.

Ein Client ohne Browser kann sich schon heute anmelden: Der Vertragstest
(`server/cmd/apicheck/client.go`) tut das mit einem Cookie-Speicher und dem CSRF-Kopf.

### 2.3 Wiedergabe

- `startPlayback` (`POST /items/{id}/play`) liefert `PlaybackInfo`: `mode` ist `direct` (Datei mit
  Range) oder `hls` (fMP4, als `remux` oder `transcode`).
- **Der Server entscheidet** (`server/internal/playback/decide.go`). Der Client meldet nur, was er
  kann: `PlaybackCapabilities` mit `containers`, `video`, `audio`, `hls` (`native`, `mse`, `none`),
  `maxWidth`, `maxHeight`, `hdr`, `name`, `type`. Der Typ `tv` ist im Vertrag schon vorgesehen.
- Medien-Adressen tragen ein Stream-Token `st` und brauchen keine Sitzung; Plan 5.4 hat es für
  native Player entworfen. Radio nutzt ein eigenes Token `rt`. Bilder und Trickplay-Bögen brauchen
  die Sitzung.
- **Ton:**
  - Bei Direktwiedergabe und Remux kopiert der Server die Tonspur, wenn das Gerät den Codec meldet
    (`audioOK`), unabhängig von der Kanalzahl.
  - Bei einem Transcode kopiert er nur AAC (`audio == "aac" && audioOK` in `decide.go`). Alles
    andere wird neu kodiert.
  - Neu kodierter Ton neben einem Videobild ist immer Stereo-AAC (`outputChannels`,
    `server/internal/transcode/pipeline.go`), mit der Bitrate für Stereo.
  - Die Codec-Angabe in der HLS-Liste (`server/internal/transcode/plan.go`) stimmt für AAC und
    für kopiertes Dolby Digital und Dolby Digital Plus. Für kopiertes DTS, TrueHD, FLAC und Opus
    nennt sie fälschlich AAC (`mp4a.40.2`).
  - Die Rechnung für die Bandbreite geht von 160 kbit/s Ton aus.
  - Eine Kanalzahl kann der Client nicht melden.
- **Tonspur wechseln:** Eine andere als die Standardspur führt zu einem Remux (Grund
  `audio_track`). Die HLS-Ausgabe trägt immer genau eine Tonspur.
- **Untertitel:** Textspuren liefert der Server als WebVTT, die ASS-Gestaltung fällt weg. Bildspuren
  (PGS, DVD) werden eingebrannt, das heisst immer Transcode.
- **HDR:** geht nur mit `hdr: true` unverändert durch, als ein einziges Ja oder Nein für HDR10,
  HLG und Dolby Vision. Dolby Vision Profil 5 und 7 nie.
- **`hls`:** `native` meldet heute nur Safari auf dem iPhone und löst dort Sonderfälle aus
  (`directVideo`, `hdrRemux`: HEVC nur über einen Remux). `mse` meldet jeder andere Browser,
  auch Safari auf dem Mac (`hlsSupport` in `features/player/capabilities.ts`). Mit `mse` bekommt
  ein Client HEVC direkt und HEVC-HDR im Remux.
- Fortschritt: `POST /items/{id}/progress` alle 10 Sekunden, bei Pause und am Ende. Der Player
  fragt alle 4 Sekunden den Zustand seiner Wiedergabesitzung ab und beendet sie beim Verlassen.
- `PlaybackInfo` trägt Kapitel, Marken für Vorspann und Abspann (`segments`), nächste und vorige
  Folge und den Verweis auf Trickplay.
- Bilder tragen das Profil in der Adresse (`p=`); beim Profilwechsel leert der Browser seinen
  Speicher über `Clear-Site-Data` (ADR 0005).

### 2.4 Fernbedienung in der Web-App

Die Web-App ist mit Pfeiltasten bedienbar (`web/src/ui/spatial.tsx`, Bibliothek
`@noriginmedia/norigin-spatial-navigation`), Escape und Backspace gelten als Zurück-Taste, und rund
100 Playwright-Dateien prüfen das (Regel 8, C-4). Eine Erkennung «das ist ein Fernseher» gibt es
nicht; `deviceType()` meldet nie `tv`.

Für dieses Konzept ist das nur Vorbild: Mit E-3 baut die TV-App ihre Oberfläche neu. Was die
Web-App an Fokusregeln gelernt hat (Zurück-Verhalten, Fokus nach dem Schliessen eines Menüs,
Reihen mit Erinnerung an die letzte Kachel), ist Vorgabe für die TV-Seiten.

### 2.5 Vertrag und Versionen

- Vertrag 1.41.0 mit rund 307 Operationen in 24'629 Zeilen: 179 `admin`, 92 `scoped`,
  24 `account`, 8 `public`, 4 `owner`.
- Regel «Vertrag = Tag»: Jede Erhöhung von `info.version` bekommt ein Release-Tag. Das
  Release-Workflow reagiert auf Tags der Form `v*.*.*`.
- **Es gibt keine Verträglichkeitsregel zwischen Client und Server.** Die Web-App reist im selben
  Container wie der Server; ein Release hat den Vertrag darum schon gebrochen (zuletzt 1.40.0),
  ohne dass ein Client stehen blieb.
- `getHealth` (`GET /healthz`) liefert `version` als Baukennung des Servers (zum Beispiel
  `0.1.0+abc1234`), nicht die Vertragsfassung, und keinen Namen des Servers.
- `task gen` erzeugt Go- und TypeScript-Code; `gen:check` prüft in der CI, dass nichts abweicht.
- Die CI kennt nur einen Filter «nur Dokumente». Jeder andere Pull Request läuft durch alle Jobs;
  das Gate `ci` zählt die Jobs in seiner `needs`-Liste auf.

### 2.6 Vorgaben von Google (Recherche, Anhang A)

- **Privates Entwicklerkonto:** Vor der öffentlichen Freigabe verlangt Google einen geschlossenen
  Test mit mindestens 12 Testern, die 14 Tage am Stück angemeldet bleiben. Ob das für reine
  TV-Apps gilt, ist unbelegt; das Papier rechnet damit.
- **Interne Testspur:** ohne Vorbedingung, bis 100 Tester je App über eine Liste von E-Mail-Adressen,
  kein Ablaufdatum genannt. Ob die Installation aus dieser Spur auf einem Fernseher klappt, ist
  unbelegt.
- **TV-Prüfung** (TV-Qualitätsliste): Eintrag für den TV-Startbildschirm (`LEANBACK_LAUNCHER`),
  Banner 320 × 180 mit App-Namen, alles mit der Fernbedienung bedienbar, Zurück führt bis zum
  Startbildschirm, Taste Wiedergabe/Pause wirkt, kein Bildschirmschoner während der Wiedergabe,
  **Test-Zugangsdaten für Googles Prüfer**.
- **Technik:** App-Bundle (AAB) ist Pflicht; Ziel-API mindestens 34. Aus derselben
  TV-Qualitätsliste (Punkte TV-PS und TV-G6, vom Rechercheur gelesen, hier nicht nachgeprüft):
  `minSdk` höchstens 31, und seit 1. August 2026 Unterstützung für 32 und 64 Bit sowie für
  16-KB-Speicherseiten.
- **Bausteine:** «Compose for TV» (`androidx.tv:tv-material` 1.1.0) ist stabil, das ältere Leanback
  ist abgekündigt. Der Player-Baukasten Media3/ExoPlayer steht bei 1.11.1.

## 3. Die App

### 3.1 Bauweise (E-3)

Eine eigene Android-App in Kotlin:

| Teil | Baustein | Grund |
|---|---|---|
| Oberfläche | Jetpack Compose mit «Compose for TV» | von Google empfohlen, Leanback ist abgekündigt |
| Video und Ton | AndroidX Media3 (ExoPlayer) | spielt MKV, MP4, TS, HLS; nutzt die Decoder des Geräts |
| Netz | OkHttp, jede Anfrage trägt das Gerätetoken | ein Ort für die Anmeldung |
| API | aus `api/openapi.yaml` erzeugter Kotlin-Client | der Vertrag bleibt die einzige Quelle |
| Bilder | Coil über denselben OkHttp-Client | Bilder brauchen die Sitzung |

**Kein FFmpeg in der App (Vorschlag, Frage F-12).** Was das Gerät nicht selbst abspielen oder
durchreichen kann, rechnet der Server um. Das hält die App klein, erspart eigenen C-Code samt der
Pflicht zu 16-KB-Speicherseiten und vermeidet Lizenzfragen der FFmpeg-Bibliotheken.

**Was «ganz nativ» kostet,** damit es beim Freigeben klar ist: Jede Zuschauerseite gibt es danach
zweimal. Jedes künftige Konzept, das eine Zuschauerseite ändert, braucht einen TV-Teil mit eigenem
Bau, eigenen Tests und eigener Abnahme. Regel 9 («Admin und Zuschauer nutzen dieselben Komponenten
aus `web/src/ui`») kann für die TV-App nicht gelten; an ihre Stelle treten eigene Bauteile mit
denselben Tokens und Texten (Abschnitt 3.5).

### 3.2 Platz im Repo und Werkzeuge

Vorschlag (Frage F-1): ein Ordner `android/` im bestehenden Repo.

- Der Vertrag, die Texte und die Design-Tokens liegen schon hier. In einem zweiten Repo müssten
  diese drei Quellen bei jeder Änderung hinüberkopiert werden.
- Ein Pull Request kann Server und App gemeinsam ändern und wird gemeinsam geprüft.
- **Erzeugter Code wird nicht eingecheckt.** Der Kotlin-Client, die Farb- und Massangaben und die
  Textdateien entstehen beim Bau der App im Bauordner. Regel 1 bekommt darum keine neuen
  Verzeichnisse, `task gen` und `gen:check` bleiben, wie sie sind, und wer nur den Server oder die
  Web-App ändert, braucht kein JDK. Bricht eine Vertragsänderung die App, zeigt es der Bau der App
  in der CI.
- Werkzeuge: JDK und Android-SDK ohne sudo unter `~/.local`, Gradle über den Wrapper im Repo,
  Versionen gepinnt wie bei Go und Node. `task android:check` und `task android:test` als
  Einstieg; `task ci` ruft beide auf. Gradle- und Emulatorläufe warten wie die anderen schweren
  Läufe auf einen Platz (`scripts/heavy-slot.sh`).
- CI: ein neuer Job `android`, eingetragen in die `needs`-Liste des Gates `ci`. Ob er bei jedem
  Pull Request läuft oder einen eigenen Pfadfilter bekommt (heute gibt es nur «nur Dokumente»),
  entscheidet Stufe 2a nach der gemessenen Laufzeit.
- **Releases der App** bekommen eigene Tags der Form `tv-v1.0.0`. Sie berühren weder das
  Release-Workflow des Servers (`v*.*.*`) noch die Regel «Vertrag = Tag».
- **Lizenz:** Das Repo steht unter AGPL-3.0, die App damit auch. Der Store-Eintrag verweist auf
  den Quellcode. Media3 und Compose stehen unter Apache 2.0.

Welcher Generator den Kotlin-Client erzeugt, entscheidet Stufe 0 nach einem Versuch mit dem echten
Vertrag (24'629 Zeilen, mit `oneOf` und `nullable`). Die App braucht nur die Operationen, die eine
Gerätesitzung aufrufen darf (Abschnitt 4.5).

### 3.3 Seiten

| TV-Seite | Vorbild in der Web-App | Stufe |
|---|---|---|
| Serveradresse | neu | 2a |
| Anmelden (Code, QR, E-Mail und Passwort) | `/login` | 2a |
| Profilwahl mit PIN | `/profiles` | 2a |
| Start (Spotlight, Reihen) | `/` | 2b |
| Bibliothek (Raster, Filter, Sortierung) | `/libraries/$libraryId` | 2b |
| Film, Serie, Staffel, Folge | `/items/$itemId` | 2b |
| Person | `/people/$id` | 2b |
| Suche | `/search` | 2b |
| Merkliste, Gesehen, Sammlungen, Listen, Empfehlungen | `/watchlist`, `/watched`, `/collections`, `/lists/$key`, `/picks` | 2b |
| Einstellungen der Wiedergabe | `/settings/playback` | 3 |
| Videoplayer | `/play/$itemId` | 3 |
| Hörbuch, Hörbuch-Reihe, Podcast, Tonplayer | `/libraries/$id/series/$sid`, Tonplayer | 4 |
| Webradio | `/radio` | 4 |
| Wünsche, Entdecken, Titel ausserhalb der Bibliothek | `/wishes`, `/discover…`, `/titles/$media/$tmdbId` | 5 |
| Benachrichtigungen (nur die Liste) | `/notifications` | 5 |

**Nicht in der App:** die Verwaltung (`/admin/*`, E-7), Ersteinrichtung und Einladungen, Profile
anlegen und ändern, Meldekanäle, Downloads und Offline, Web-Push. Wer das braucht, nimmt den
Browser. Die Verwaltung auszuschliessen ist Marcos Entscheid; die übrigen Punkte sind Vorschlag.

### 3.4 Bedienung und Gestaltung

- Alles mit Steuerkreuz, OK und Zurück. Zurück schliesst zuerst Menüs, dann die Seite, auf der
  Startseite die App (Vorgabe von Google).
- Der Fokus ist immer sichtbar. Jede Reihe merkt sich ihre letzte Kachel.
- Texteingabe nur an drei Stellen: Serveradresse, Anmeldung mit Passwort, Suche. Die Suche bekommt
  zusätzlich die Spracheingabe des Geräts, wo sie vorhanden ist.
- Vorschlag: Marcos Gestaltungsregeln vom 16. November 2026 für die Web-App auch hier anwenden.
  Klappbares ist zu, Kontrast hoch (Rahmen mindestens 3:1, gedämpfter Text mindestens 8:1).
- Schrift und Abstände für Sofa-Abstand: eigene Tokens für den Fernseher, keine Zahl im Code.

**Mockup vor dem Bau (Frage F-2):** Vorschlag ist ein klickbares Mockup in 1920 × 1080, mit den
Pfeiltasten bedienbar, für Anmeldung, Profilwahl, Start, Bibliothek, Detailseite und
Player-Bedienung. Marco gibt es frei, bevor Stufe 2b beginnt.

### 3.5 Texte und Farben (Regel 9)

- **Texte:** Quelle bleiben `web/src/locales/*.json` mit ihren 23 Sprachen. Ein Generator schreibt
  daraus beim Bau die Android-Dateien `strings.xml`. Texte, die nur der Fernseher braucht, kommen
  unter einem eigenen Namensraum in dieselben Dateien, damit der Übersetzungsweg aus ADR 0033 und
  `docs/i18n/README.md` einer bleibt. Vor jedem Release der App werden die Übersetzungen
  nachgezogen, wie heute vor einem Release des Servers.
- Der Generator muss übertragen:
  - Mehrzahlformen (`_one`, `_other` …) nach `<plurals>`
  - Platzhalter `{{name}}` nach `%1$s`
  - die Ergänzungsdatei `de-DE` nach `values-de-rDE`
  - Sprachen mit Region (`pt-BR`, `zh-TW` …) nach `values-pt-rBR`, `values-zh-rTW`
- **Farben und Masse:** `packages/design-tokens` baut schon mit Style Dictionary (`build.mjs`). Ein
  weiteres Ausgabeformat erzeugt Kotlin-Konstanten für Compose. Dabei werden `rem` in `dp` und `sp`
  und Farben mit Deckkraft (`#RRGGBBAA`) in die Reihenfolge von Android (ARGB) umgerechnet.
- **Wächter:** Android Lint meldet festen Text als Fehler; eine Lint-Regel verbietet Farbwerte
  ausserhalb des erzeugten Codes.
- **Regel 12 gilt unverändert:** Die App entscheidet nichts über Sichtbarkeit oder Alter. Sie zeigt,
  was der Server liefert; nicht sichtbar heisst 404.

## 4. Anmeldung (E-4)

### 4.1 Serveradresse

Beim ersten Start fragt die App nach der Adresse des Servers, prüft sie mit `getHealth` und zeigt
Adresse und Version. Die Adresse bleibt gespeichert; wechseln geht in den Einstellungen.

- Eine automatische Suche im Heimnetz ist nicht Teil der ersten Ausgabe (Frage F-5): Der Server
  läuft in Docker, dort kommen die üblichen Suchverfahren ohne Zusatzeinrichtung nicht aus dem
  Container heraus.
- Android blockiert unverschlüsseltes HTTP ab Werk. Viele Heimserver laufen ohne TLS im LAN. Die
  App muss HTTP darum ausdrücklich erlauben (Frage F-6).

### 4.2 Das Gerätetoken

Die TV-App bekommt nie eine volle Sitzung. Sie bekommt ein **Gerätetoken**:

- 256 Bit Zufall, im Server nur gehasht gespeichert (Plan 3.8 Punkt 2), in der App verschlüsselt
  abgelegt.
- Die App sendet es als `Authorization: Bearer`. Damit wird das Schema `bearerToken` des Vertrags
  erstmals benutzt, so wie Plan 3.8 Punkt 7 und ADR 0005 es vorgesehen haben: Das Token hängt an
  der Sitzungstabelle und erbt die Profilwahl.
- Es gehört zu genau einem Gerät. Der Server legt die Gerätezeile mit Name und Typ `tv` an, sobald
  die Kopplung beginnt, nicht erst beim ersten Abspielstart. PIN-Sperre und «Gerät vergessen»
  treffen so den Fernseher und nicht das Handy, mit dem bestätigt wurde.
- Es öffnet eine **Gerätesitzung** mit eingeschränkten Rechten (Abschnitt 4.5).
- Auch Bilder, Trickplay-Bögen und der Radio-Start nehmen das Token an.

Regeln für den Bearer-Weg im Server, damit er den Cookie-Weg nicht stört:

- Eine Anfrage mit Bearer bekommt nie ein `Set-Cookie`, weder für die Sitzung noch für das
  Gerät, und ein 401 löscht kein Cookie. Heute tut die Middleware alle drei Dinge für jede
  Anfrage.
- Kommen Cookie und Bearer zusammen, lehnt der Server ab. Kein echter Client sendet beides.
- Der CSRF-Kopf `X-Requested-With: Slouchcat` bleibt Pflicht für jede schreibende Anfrage; die
  App sendet ihn. Es gibt keine Ausnahme für Bearer.
- Die Gültigkeit steht je Sitzung in der Tabelle. Heute gilt ein fester Wert für alle
  (`SessionLifetime`); eine eigene Dauer für Gerätesitzungen (Frage F-4) braucht eine Spalte.

Ein Gerätetoken entsteht auf zwei Wegen: durch die Kopplung (4.3) oder durch Anmelden mit
Passwort in der TV-App (4.4).

### 4.3 Kopplung per Code

Ablauf, angelehnt an RFC 8628 («Device Authorization Grant»):

1. Der Fernseher bittet den Server um eine Kopplung und nennt seinen Gerätenamen.
2. Der Server antwortet mit einem **Gerätecode** (lang, geheim, bleibt im Fernseher) und einem
   **Benutzercode** (kurz, steht am Bildschirm), gültig 10 Minuten.
3. Der Fernseher zeigt den Benutzercode gross an, dazu einen QR-Code auf die neue Web-Seite
   `/link` des Servers.
4. Marco öffnet die Seite am Handy. Ist er dort nicht angemeldet, meldet er sich zuerst an. Die
   Seite zeigt, woher die Anfrage kommt, und fragt: «Diesen Fernseher anmelden?»
5. Der Fernseher fragt alle 5 Sekunden nach. Nach der Bestätigung bekommt er auf seine eigene
   Nachfrage hin das Gerätetoken und zeigt die Profilwahl.

Neue Operationen (Namen legt der Bau fest):

| Operation | `x-visibility` | Zweck |
|---|---|---|
| Kopplung beginnen | `public` | Gerätezeile anlegen, Gerätecode und Benutzercode ausgeben |
| Kopplung abholen | `public` | mit dem Gerätecode nachfragen; liefert nach der Bestätigung einmal das Gerätetoken |
| Kopplung ansehen | `account` | Herkunft, Alter und gemeldeten Namen zum Benutzercode zeigen |
| Kopplung bestätigen oder ablehnen | `account` | das Gerätetoken freigeben |
| Gerät mit Passwort anmelden | `public` | 4.4 |
| Meine Geräte: Liste und Abmelden | `account` | 4.5 |

Die Web-Seiten `/link` und «Meine Geräte» sind neu und bekommen ihren Tastaturtest (Regel 8).

«Kopplung beginnen» ist öffentlich und schreibt trotzdem in die Datenbank. Darum gilt:

- Die Kennung des Geräts vergibt der Server beim Beginnen und bindet sie an die Kopplung. Sie
  kommt nicht aus einem Cookie.
- Läuft eine Kopplung ab oder wird sie abgelehnt, löscht der Server ihre Gerätezeile wieder.
- Die Obergrenze aus K-4 zählt diese Zeilen mit.

### 4.4 E-Mail und Passwort

Die TV-App ruft dafür nicht `login` auf, sondern eine neue Operation «Gerät mit Passwort
anmelden»: E-Mail, Passwort und Gerätename hinein, Gerätetoken heraus. Sie teilt sich **denselben
Zähler** für Fehlversuche mit `login`; ein eigener Zähler würde die Zahl der Rateversuche
verdoppeln. (Der Zähler von `login` hängt heute nur an der Adresse des Aufrufers, eine Sperre je
Konto gibt es nicht; `docs/dev/SICHERHEITSREVIEW-1.0.md`.) So hält der Fernseher auch auf diesem
Weg nur ein Gerätetoken.

Ehrlich gesagt: Das ist eine Entscheidung der App, keine Schranke des Servers. Wer das Passwort
kennt, kann sich mit jedem Programm über `login` voll anmelden. Die Einschränkung der
Gerätesitzung schützt gegen eine untergeschobene Kopplung und gegen einen Fernseher, der in
fremde Hände kommt, nicht gegen ein bekanntes Passwort.

Googles Prüfer brauchen diesen Weg später (Abschnitt 8.2).

### 4.5 Gerätesitzung

Eine Gerätesitzung darf nur, was auf einer **Erlaubnisliste** steht. Alles andere beantwortet der
Server mit 403.

- **Erlaubt auf Kontoebene:** `logout`, `getSession`, `listMyProfiles`, `selectProfile`,
  `listAccountLibraries`.
- **Erlaubt mit Profil:** die `scoped`-Operationen zum Schauen, Hören, Suchen, Merken und Wünschen.
- **Nicht erlaubt:** alles mit `admin` und `owner`; `updateAccount`; die Profilpflege samt PINs;
  die Meldekanäle samt Telegram und Web-Push; `getWatchReport`; Downloads und Offline.
- **Ebenfalls nicht erlaubt, und das ist wesentlich:** «Kopplung ansehen», «Kopplung bestätigen
  oder ablehnen» und «Meine Geräte». Sonst könnte ein gekoppeltes Gerät das nächste freigeben
  oder die echten Fernseher abmelden.
- **Wo die Liste steht:** Vorschlag ist eine zweite Angabe je Operation im Vertrag, neben
  `x-visibility`, die sagt, ob eine Gerätesitzung die Operation aufrufen darf. Fehlt sie, gilt
  nein. Der Schema-Test verlangt sie für jede `account`- und `scoped`-Operation, damit jede neue
  Operation die Frage beantworten muss. Machbar ist das: Die Middleware liest die Sichtbarkeit
  schon heute zur Laufzeit aus dem Vertrag, und der Codegenerator übergeht solche Angaben.
- **Streaming-Routen:** Bilder, Trickplay, Personen-, Sammlungs- und Radiobilder nehmen ihre
  Sichtbarkeit nicht aus dem Vertrag, sondern aus einer von Hand gepflegten Tabelle
  (`binaryRoutes`, `server/internal/api/api.go`). Die Angabe muss auch dort stehen, und der
  403-Test aus Abschnitt 9 muss diese Routen einschliessen.

**Meine Geräte:** Jedes Konto sieht in seinen Einstellungen im Browser die eigenen gekoppelten
Geräte (Name, Typ, Herkunft der Kopplung, letzter Kontakt) und kann jedes abmelden. Heute kann das
nur die Verwaltung (Frage F-15).

**Gültigkeit:** Wie lange ein Gerätetoken ohne Benutzung gültig bleibt, ist Frage F-4.

**Restrisiko, damit es benannt ist:** Eine Gerätesitzung kann alles ansehen, was die Profile des
Kontos ohne PIN dürfen, und sie kann Wünsche anlegen. Ein Wunsch löst über Seerr einen echten
Download aus; das Limit je Konto aus Konzept 25 begrenzt die Menge (Frage F-16).

### 4.6 Sicherheit der Kopplung

Die Kopplung ist ein neuer Weg zu einer Sitzung und damit der heikelste Teil dieses Konzepts.

| Nr. | Regel | Schützt gegen |
|---|---|---|
| K-1 | Benutzercode: 8 Zeichen aus 20 Buchstaben ohne Vokale und ohne verwechselbare Zeichen (Muster aus RFC 8628), Anzeige `XXXX-XXXX` | Tippfehler, anstössige Wörter |
| K-2 | Gerätecode und Gerätetoken: 256 Bit Zufall, im Server nur gehasht gespeichert | Auslesen der Datenbank |
| K-3 | Kopplung gültig 10 Minuten, einmal verwendbar; das Gerätetoken wird genau einmal ausgegeben | Wiederverwendung |
| K-4 | Obergrenze für offene Kopplungen, gesamt und je Aufrufer; das Abholen antwortet mit «langsamer», wenn zu oft gefragt wird | Überfluten der Tabelle über die öffentliche Operation |
| K-5 | Die Seite `/link` bestätigt nie von selbst. Sie zeigt die **Herkunft der Anfrage** («aus deinem Heimnetz» oder «aus dem Internet», mit Adresse), ihr Alter und den Namen, den das Gerät **selbst angibt**, dazu den Satz «Bestätige nur, wenn dieser Code jetzt auf deinem eigenen Fernseher steht.» | untergeschobene Links |
| K-6 | Der QR-Code bringt den Benutzercode nur mit, wenn die Kopplung aus dem Heimnetz begonnen wurde. Sonst muss man den Code abtippen und sieht eine Warnung (Frage F-3) | Phishing mit einem Klick |
| K-7 | Schalter in der Verwaltung «Kopplung nur aus dem Heimnetz» (Frage F-3) | Kopplung durch Fremde aus dem Internet |
| K-8 | Gerätesitzung nur mit Erlaubnisliste (4.5) | Schaden, falls doch ein fremdes Gerät gekoppelt wird |
| K-9 | Jede bestätigte Kopplung geht als Meldung über den Kanal des Kontos hinaus (E-Mail, Telegram; `notify`) und steht in «Meine Geräte» | unbemerkte Kopplung |
| K-10 | `devicecode`, `usercode` und das Gerätetoken kommen in die Redaktionsliste der Logs (Regel 10) | Geheimnisse in Logs |

Zu K-5 bis K-7: Eine private Adresse allein beweist kein Heimnetz. Hinter einem Reverse-Proxy
ohne eingetragene `TrustedProxies` (zum Beispiel Caddy in Docker, der beschriebene Weg zu HTTPS)
kommt **jede** Anfrage von der privaten Adresse des Proxys, auch die aus dem Internet; und mit
der Voreinstellung `private` kann ein Gerät im LAN über `X-Forwarded-For` jede Adresse
vortäuschen. Als «Heimnetz» gilt eine Anfrage darum nur, wenn drei Dinge zusammen stimmen:

1. Die Adresse des Aufrufers ist privat.
2. Die Anfrage trägt keinen Weiterleitungskopf (`X-Forwarded-For`, `Forwarded`) von einer
   Gegenstelle, die nicht als Proxy eingetragen ist.
3. Die Adresse liegt in einem Bereich, den der Administrator ausdrücklich als Heimnetz
   eingetragen hat.

Sonst gilt die Herkunft als unbekannt und wird wie «Internet» behandelt; die Seite sagt das.
Ohne eingetragenen Heimnetz-Bereich gibt es also kein «Heimnetz». Der Server warnt beim Start,
wenn der Schalter aus K-7 an ist und kein Bereich eingetragen ist. Dies wäre die erste Stelle im
Server, die dem lokalen Netz vertraut (`docs/dev/SICHERHEITSREVIEW-1.0.md`); ADR 0040 hält das
fest.

Zu K-9: Die Meldung muss ausserhalb der App ankommen. Benachrichtigungen in der App gehören zu
einem Profil, und die gekoppelte Sitzung könnte sie als gelesen markieren. Hat ein Konto keinen
Kanal eingerichtet, bleibt «Meine Geräte» der einzige Ort; das Papier nennt das als Grenze.

**Bekannte Grenze:** Ein Angreifer kann eine Kopplung auf seinem eigenen Gerät beginnen und einem
angemeldeten Kontoinhaber Link oder Code schicken. Bestätigt dieser trotz K-5 und K-6, schaut der
Angreifer mit (Restrisiko in 4.5). K-8 und K-9 begrenzen den Schaden und machen ihn sichtbar, sie
verhindern ihn nicht. Dieses Muster ist bei jeder Kopplung per Code so.

## 5. Wiedergabe

### 5.1 Der Videoplayer

Der Player der TV-App soll dasselbe können wie der Web-Player (Konzepte 13, 18 und 23):

- Wiedergabe, Pause, Springen um 10 Sekunden mit Links und Rechts, Spulen mit Vorschaubildern
  (Trickplay), Kapitel
- Menüs für Tonspur, Untertitel und Qualität
- Vorspann und Abspann überspringen, Countdown zur nächsten Folge, vorige und nächste Folge
- Fortschritt melden, Abbruch durch den Server erkennen, Sitzung beim Verlassen beenden
- eine Info-Anzeige mit Codec, Bitrate und Abspielart

Dazu kommt, was ein Fernseher verlangt: Die Tasten Wiedergabe/Pause, Vor und Zurück der
Fernbedienung wirken (Media3 `MediaSession`), und während der Wiedergabe startet kein
Bildschirmschoner.

### 5.2 Was die App dem Server meldet

Die App füllt `PlaybackCapabilities` aus dem, was das Gerät wirklich kann (Decoderliste des Geräts,
Anzeige, Tonausgang), mit `type: tv`, einem Namen wie «Chromecast · Android TV 14» und **`hls: mse`**.
Der Wert `mse` trifft das Verhalten, das ein ExoPlayer braucht (HEVC direkt, HEVC-HDR im Remux);
`native` bleibt Safari auf dem iPhone vorbehalten. Die Beschreibung des Feldes im Vertrag wird
entsprechend ergänzt, sein Verhalten nicht.

Die App muss auch `containers` senden, mit den Namen, die der Server kennt (`mp4`, `mkv`, `webm`,
`ts`, `m2ts`, `avi`; `server/internal/library/probe.go`). Ohne diese Liste spielt der Server nur
MP4 direkt.

Der Vertrag braucht drei Ergänzungen. Alle sind zusätzlich; ein Client, der sie nicht sendet,
bekommt das heutige Verhalten.

| Ergänzung | Bedeutung | Wirkung im Server |
|---|---|---|
| höchste Kanalzahl des Tonausgangs | 2, 6 oder 8 | beim Neu-Kodieren Mehrkanal statt Stereo (5.3) |
| «wählt Spuren selbst» | der Player wählt Ton- und Untertitelspur in der Originaldatei | **nur bei Direktwiedergabe:** eine andere Tonspur erzwingt keinen Remux mehr |
| Untertitelformate, die der Player selbst zeichnet | zum Beispiel `ass`, `pgs` | **nur bei Direktwiedergabe:** diese Spuren werden nicht mehr eingebrannt (5.4) |

Die zweite und dritte Ergänzung wirken nur bei Direktwiedergabe, weil die HLS-Ausgabe genau eine
Tonspur und keine Bildspuren trägt.

### 5.3 Mehrkanalton (E-6)

Die App meldet in `audio` nur Formate, die das Gerät selbst dekodiert oder an die Tonanlage
durchreicht. Drei Fälle:

| Fall | Heute | Neu |
|---|---|---|
| **Direktwiedergabe** | Ton bleibt, wie er ist | unverändert; im Server nichts zu tun |
| **Remux** (Video kopiert) | gemeldeter Ton wird kopiert; für DTS, TrueHD, FLAC und Opus nennt die HLS-Liste fälschlich AAC | Codec-Angabe in der HLS-Liste richtigstellen |
| **Transcode** (Video neu kodiert) | nur AAC wird kopiert, alles andere wird Stereo-AAC | (a) gemeldetes Dolby Digital und Dolby Digital Plus neben dem neuen Video **kopieren**; (b) was neu kodiert werden muss, bei 6 gemeldeten Kanälen in 5.1 kodieren, mit passender Bitrate |

Zu (a): Das hält den Originalton für die Tonanlage, ohne dass der Server einen neuen Encoder
braucht. Welche Formate die HLS-Ausgabe (fMP4) tragen kann und ExoPlayer daraus abspielt, misst
Stufe 0; erwartet werden AAC, Dolby Digital und Dolby Digital Plus. Im Server sind zwei Stellen
zu ändern: die Bedingung für die Tonkopie beim Transcode (`audio == "aac" && audioOK` in
`decide.go`) und die Bandbreitenrechnung. Sie geht von 160 kbit/s Ton aus; kopiertes Dolby
Digital Plus kann 640 kbit/s haben und würde die Grenzen eines Profils sonst unbemerkt
überschreiten. Die Rechnung muss die echte Bitrate der kopierten Spur nehmen.

Zu (b): Vorschlag für das Format ist Dolby Digital (AC-3), wenn das Gerät es meldet, weil es
Soundbars und Receiver auch über den Rückkanal des Fernsehers (ARC) erreicht; sonst AAC 5.1.
Stufe 0 misst, welches der beiden auf den drei Geräten sauber läuft. Die heutige Bitrate für
Stereo reicht für 5.1 nicht; der Bau legt eigene Werte fest.

Welche Formate ein Gerät durchreicht, hängt am Gerät und an der Tonanlage; eine verlässliche
Aussage von Google dazu war nicht zu finden (unbelegt). Die App bekommt darum in den Einstellungen
der Wiedergabe einen Schalter «Ton»: automatisch, immer Stereo, Formate von Hand. So kommt man
weiter, wenn die Erkennung am Gerät falsch liegt.

### 5.4 Untertitel

- Text (SRT, WebVTT, ASS): ExoPlayer zeichnet sie selbst. Die ASS-Gestaltung bleibt nach Googles
  Liste nur zum Teil erhalten.
- Bild (PGS, DVD): In Googles Formatliste stehen sie nicht; ein anderes Projekt gibt an, ExoPlayer
  könne PGS (unbelegt). Stufe 0 misst das. Wenn es trägt, meldet die App `pgs`, und der Server
  brennt diese Spuren bei Direktwiedergabe nicht mehr ein. Wenn nicht, und immer bei HLS, bleibt
  es beim Einbrennen wie im Browser.

### 5.5 HDR

Die App meldet `hdr: true`, wenn Gerät und Anzeige HDR können. Das eine Ja oder Nein kann HDR10,
HLG und Dolby Vision nicht unterscheiden; ein Gerät, das nur HDR10 kann, bekäme auch
Dolby-Vision-Dateien mit HDR10-Grundschicht unverändert. Stufe 0 prüft, ob das auf den drei
Geräten ein Problem ist. Dolby Vision Profil 5 und 7 bleiben wie heute ausgenommen. Das Umschalten
der Bildfrequenz des Fernsehers auf die des Films ist nicht Teil der ersten Ausgabe (Frage F-8).

### 5.6 Tonplayer (Stufe 4)

Hörbücher, Podcasts und Webradio nutzen dieselben Endpunkte wie die Web-App (Radio mit seinem
eigenen Token `rt`) und einen eigenen Tonbildschirm mit Titelbild, Kapiteln oder Titelanzeige.
Hörbücher aus mehreren Dateien liefert der Server schon heute als eine HLS-Zeitachse. Der Ton
läuft weiter, solange die App im Vordergrund ist; der Bildschirmschoner des Geräts darf hier
kommen. Weiterlaufen im Hintergrund ist nicht Teil der ersten Ausgabe.

### 5.7 Profilwechsel

Der Browser leert beim Profilwechsel seinen Speicher (`Clear-Site-Data`, ADR 0005). Die App muss
das selbst tun: Der Bildspeicher von Coil unterscheidet nach der vollen Adresse samt `p=` und
wird beim Profilwechsel und beim Abmelden geleert, ebenso alle gemerkten Listen.

## 6. Wünsche über Seerr (Stufe 5)

Die App zeigt den Bereich «Wünsche» wie die Web-App (Konzept 25): Entdecken, Filme, Serien, Suche,
die Seite eines Titels ausserhalb der Bibliothek, Anfordern, Zurückziehen, Stand und Fortschritt.
Neue Endpunkte braucht es dafür nicht. Freigeben und Ablehnen für Administratoren bleibt im
Browser, weil eine Gerätesitzung keine Verwaltungsrechte hat (4.5).

## 7. Änderungen am Server im Überblick

| Was | Wo | Art |
|---|---|---|
| Bearer: der Server liest `Authorization`, auch für Bilder, Trickplay und Radio-Start | Sitzungs-Middleware | neu |
| Gerätetoken und Gerätesitzung mit Erlaubnisliste; Angabe je Operation im Vertrag samt Schema-Test | Vertrag, Middleware, Migration | neu |
| Kopplung: beginnen, abholen, ansehen, bestätigen/ablehnen; Tabelle für offene Kopplungen; Gerätezeile mit Name und Typ schon bei der Kopplung | Vertrag, `auth`, Migration | neu |
| Gerät mit Passwort anmelden | Vertrag, `auth` | neu |
| Meine Geräte: Liste und Abmelden je Konto, Seite in den Einstellungen | Vertrag, `web/src` | neu |
| Web-Seite `/link`, Texte in allen Sprachen, Tastaturtests | `web/src` | neu |
| Meldung «Neues Gerät gekoppelt» über den Kanal des Kontos | `notify` | neu |
| Schalter «Kopplung nur aus dem Heimnetz», Eintrag des Heimnetz-Bereichs, Warnung beim Start | Verwaltung, `config` | neu |
| `PlaybackCapabilities`: drei Ergänzungen (5.2), Beschreibung von `hls` | Vertrag, `playback/decide.go` | zusätzlich |
| Ton: richtige Codec-Angabe in der HLS-Liste, Kopieren von Dolby Digital (Plus) beim Transcode samt Bandbreitenrechnung mit echter Bitrate, Mehrkanal beim Neu-Kodieren | `playback/decide.go`, `playback/quality.go`, `transcode/plan.go`, `transcode/pipeline.go` | Verhalten ändert sich nur für Clients, die diese Formate oder Kanäle melden; die Codec-Angabe wird für alle richtig |
| Sitzungen: Gültigkeit je Sitzung statt eines festen Werts; Bearer-Weg ohne `Set-Cookie` | `auth`, Middleware, Migration | neu |
| `getHealth`: Feld für die Vertragsfassung | Vertrag | zusätzlich |
| Redaktionsliste der Logs | `internal/logging` | Regel 10 |

Alles zusammen ist eine zusätzliche Vertragsänderung (keine brechende). Sie wird mit Stufe 1
gebaut, bevor die App sie braucht.

## 8. Verteilung (E-2)

### 8.1 Privat

1. **Während des Baus:** Installation über ADB im Heimnetz. Marco schaltet dafür auf jedem Gerät
   einmal die Entwickleroptionen ein.
2. **Sobald Google die Identität bestätigt hat:** interne Testspur im Play Store. Marco trägt die
   Google-Adressen von Familie und Freunden ein (bis 100), sie melden sich über einen Link an und
   finden die App danach im Play Store des Fernsehers. Ob der letzte Schritt auf einem Fernseher so
   klappt, ist unbelegt und wird beim ersten Hochladen geprüft.

Für die Testspur braucht es einen festen Paketnamen (Frage F-7), ein App-Bundle und einen
Signaturschlüssel. **Der Paketname lässt sich nach dem ersten Hochladen nie mehr ändern.** Die
Wegwerf-App aus Stufe 0 trägt darum einen eigenen Namen und wird nie hochgeladen.

### 8.2 Öffentlich (spätere Stufe)

- Geschlossener Test mit 12 Testern über 14 Tage (privates Konto).
- TV-Prüfung nach Googles Liste (2.6).
- Store-Eintrag: Banner, Bildschirmfotos, Beschreibung, Datenschutzerklärung, Angaben zur
  Datensicherheit, Alterseinstufung, Verweis auf den Quellcode (AGPL).
- **Googles Prüfer brauchen einen erreichbaren Server mit Test-Zugang** (Frage F-10). Slouchcat ist
  selbst gehostet; ohne einen Vorführserver im Internet kann Google die App nicht prüfen.

### 8.3 Schlüssel

Den Schlüssel zum Hochladen erzeugt Marco selbst. Er liegt nie im Repo. Ob das Hochladen von Hand
oder aus der CI geschieht, ist Frage F-11.

### 8.4 Versionen und Verträglichkeit

Heute gehören Client und Server zum selben Release. Mit einer App im Play Store gilt das nicht
mehr: Der Fernseher aktualisiert die App von selbst, der Server wird von Hand gehoben, und
umgekehrt.

Vorschlag:

- `getHealth` bekommt ein Feld mit der Vertragsfassung. Die App kennt die kleinste Fassung, die sie
  braucht. Ist der Server zu alt, zeigt sie «Server zu alt, bitte aktualisieren» statt halb zu
  funktionieren.
- Unbekannte Felder ignoriert die App.
- **Neue Regel neben «Vertrag = Tag»:** Operationen, die eine Gerätesitzung aufrufen darf, brechen
  nicht mehr von einem Release aufs nächste. Eine brechende Änderung braucht eine Übergangszeit,
  in der der Server alt und neu liefert. Wie lang, ist Frage F-9. Die Angabe aus 4.5 sagt zugleich,
  welche Operationen das betrifft.

Solange die App nur privat verteilt wird, hat Marco beide Seiten in der Hand; zwingend wird die
Regel mit der öffentlichen Stufe. Sie von Anfang an einzuhalten, kostet aber am wenigsten.

## 9. Prüfung

| Ebene | Was | Entspricht |
|---|---|---|
| Unit | Zustände der Seiten, Fähigkeitsprofil, Fehlerbilder | Vitest der Web-App |
| Oberfläche | je Seite ein Test mit Tastenereignissen des Steuerkreuzes: jeder Knopf erreichbar, Fokus sichtbar, Zurück führt hinaus, keine Fokusfalle | Regel 8 |
| Vertrag | die App gegen einen echten Server mit Testdaten, wie die e2e-Läufe | `apicheck` |
| Server | Kopplung, Gerätetoken und Erlaubnisliste im Vertragstest (`TestContractCoversEveryOperation`); ein Test, der jede nicht erlaubte Operation mit einem Gerätetoken aufruft und 403 erwartet; Entscheid-Tests für die neuen Fähigkeiten | wie heute |
| Geräte | Abnahme durch Marco auf Chromecast, Shield und Fernseher, je Stufe eine Liste | Abnahme auf CT 190 |

Die Teststufen aus `CLAUDE.md` gelten weiter. Ob die Oberflächentests in der CI auf einem
Emulator laufen können und wie lange das dauert, misst Stufe 2a und schreibt es nach
`docs/perf/`.

## 10. Stufen

Jede Stufe ist ein eigener Auftrag von Marco. Stufe 1 und alle Stufen der App sind je ein Release.

| Stufe | Inhalt | Beleg | Braucht |
|---|---|---|---|
| **0 Messung** | Wegwerf-App mit eigenem Paketnamen, per ADB auf den drei Geräten, kein Eingriff ins Produkt. Misst: Direktwiedergabe (MKV, HEVC, HDR, Dolby-Vision-Datei auf HDR10-Gerät), Tonformate und Durchreichen je Gerät, Dolby Digital und AAC 5.1 in fMP4-HLS, PGS und ASS, Tempo von Compose auf dem schwächsten Gerät, Generator für den Kotlin-Client. | Bericht `docs/perf/K26-android-tv.md` mit Entscheid je Punkt | Entwickleroptionen auf den Geräten |
| **1 Server** | alles aus Abschnitt 7 | Vertragstest; ein Prüfprogramm nach Art von `apicheck`, das einen Fernseher spielt (koppeln, abholen, Profil wählen, Abspielstart mit Mehrkanal-Profil); Tastaturtests für `/link` und «Meine Geräte»; Abnahme der Kopplung auf CT 190 mit dem Prüfprogramm | Ergebnisse aus Stufe 0 (Tonformat, PGS); F-3, F-4, F-15, F-16 |
| **2a Gerüst** | Projekt `android/`, Werkzeuge, CI-Job, Generatoren für Client, Tokens und Texte, Serveradresse, Anmeldung auf beiden Wegen, Profilwahl mit PIN | App per ADB auf dem Fernseher: koppeln und Profil wählen; gemessene Laufzeit des CI-Jobs | Stufe 1; F-1, F-5, F-6, F-13 |
| **2b Seiten** | Start, Bibliothek, Detailseiten, Person, Suche, Listen | Steuerkreuz-Test je Seite; Abnahme auf den drei Geräten: blättern ohne Abspielen | freigegebenes Mockup (F-2) |
| **3 Videoplayer** | Player vollständig, Mehrkanalton, HDR, Einstellungen der Wiedergabe | Abnahmeliste je Gerät und Tonanlage; **Vorab-Ausgabe** für Familie und Freunde über die interne Testspur | bestätigte Identität bei Google; F-7, F-8, F-11 |
| **4 Ton** | Hörbücher, Podcasts, Webradio | Abnahme auf den Geräten | |
| **5 Wünsche** | Bereich «Wünsche», Benachrichtigungen | Abnahme mit echtem Seerr; **erste Ausgabe nach E-5 vollständig** | |
| **6 Öffentlich** | Geschlossener Test, TV-Prüfung, Store-Eintrag, Vorführserver, Verträglichkeitsregel verbindlich | App im Play Store | F-9, F-10 |

Reihenfolge: Stufe 0 kommt vor Stufe 1, weil Stufe 1 das Tonformat und die Untertitel-Angabe
baut, über die Stufe 0 entscheidet. Der Server liefert Dolby Digital und AAC 5.1 in HLS erst
nach Stufe 1; Stufe 0 misst darum mit Proben, die von Hand mit ffmpeg und den Schaltern der
heutigen Pipeline (`server/internal/transcode/pipeline.go`) erzeugt werden. Das Mockup entsteht
neben Stufe 1 oder 2a.

## 11. Änderungen an früheren Entscheiden und Regeln

Alle erst mit dem Bau und auf Marcos Wort, nicht mit diesem Papier.

| Stelle | Heute | Neu |
|---|---|---|
| Spec «Non-Goals» und Übersichtstabelle («zurückgestellt») | TV-Apps (Android TV, Apple TV, webOS, Tizen) und native Apps sind Nicht-Ziel | Android TV ist Ziel (Frage 34). Apple TV, webOS, Tizen, Fire TV und native Apps für Handy und Tablet bleiben Nicht-Ziele |
| Plan Abschnitt 1 («Nicht eingeplant») | TV-Apps, native Apps | Android TV herausgenommen |
| `CLAUDE.md` Regel 11 | «TV-Apps, native Apps» in der Liste der Nicht-Ziele | benannte Ausnahme: die Android-TV-App nach Frage 34 und ADR 0039 |
| `CLAUDE.md` Regel 7 | jede Operation trägt `x-visibility` | dazu: jede `account`- und `scoped`-Operation sagt, ob eine Gerätesitzung sie aufrufen darf |
| `CLAUDE.md` Regel 8 | Playwright-Tastaturtest je neuer Seite | dazu: je TV-Seite ein Steuerkreuz-Test |
| `CLAUDE.md` Regel 9 | Farben aus Tokens, Texte aus `web/src/locales`, gemeinsame Bauteile | gilt für `android/` mit erzeugten Farben und Texten; der Satz zu `web/src/ui` gilt nur für die Web-App |
| `CLAUDE.md` Werkzeuge | Go, Node, pnpm, `task` | dazu JDK, Android-SDK, Gradle |
| Plan 3.8 Punkt 7 | «Fremd-Clients per Bearer» (nie gebaut) | gebaut, aber nur als Gerätetoken für Gerätesitzungen |
| ADR 0005 | eine Sitzungsart | Nachtrag: Gerätesitzung mit Erlaubnisliste (ADR 0040) |
| Regel «Vertrag = Tag» | ein Tag je Vertragsfassung | bleibt; daneben neu die Übergangszeit für Operationen der Gerätesitzung (8.4, ADR 0039) |

## 12. Risiken

| Nr. | Risiko | Folge | Gegenmittel |
|---|---|---|---|
| R-1 | Zweite Oberfläche | jede Zuschauer-Funktion doppelt, dauerhaft | TV-Teil als fester Abschnitt in jedem künftigen Konzept |
| R-2 | Schwache Sticks | ruckelnde Oberfläche | Stufe 0 misst auf dem schwächsten Gerät; Bildgrössen vom Server passend anfordern |
| R-3 | Tonformate je Gerät verschieden | kein Ton oder Stereo trotz Tonanlage | Schalter «Ton» (5.3), Messung je Gerät |
| R-4 | Kopplung als neuer Weg zur Sitzung | Mitschauen und Wünschen durch untergeschobenen Code | K-1 bis K-10, Erlaubnisliste, «Meine Geräte» |
| R-5 | Vertrag bricht eine installierte App | App auf dem Fernseher geht nicht mehr | Verträglichkeitsregel (8.4) |
| R-6 | Google lehnt die App ab oder verlangt mehr | öffentliche Stufe verzögert sich | private Verteilung hängt nicht davon ab |
| R-7 | Identitätsprüfung bei Google dauert | interne Testspur später | bis dahin ADB |
| R-8 | 12 Tester über 14 Tage | öffentliche Stufe braucht Mitmacher | Familie und Freunde aus der privaten Stufe |
| R-9 | Zweite Werkzeugkette (JDK, Android-SDK, Gradle) | längere CI, mehr Pflege | Laufzeit in Stufe 2a messen, danach über einen Pfadfilter entscheiden |
| R-10 | Texte in 23 Sprachen auch für die TV-App | mehr Übersetzungsarbeit je Funktion | eine Quelle, ein Übersetzungsweg (3.5) |
| R-11 | Bearer im Server ist ein neuer Anmeldeweg für alle Operationen | Fehler dort träfe mehr als die TV-App | Bearer nur für Gerätetoken, Erlaubnisliste im Vertrag, Test über alle Operationen (Abschnitt 9) |

## 13. Fragen

Bis zur Antwort gilt die Empfehlung. Die Bau-Session legt offene Fragen vor ihrer Stufe vor.

| Nr. | Frage | Empfehlung | Fällig vor |
|---|---|---|---|
| F-1 | App im selben Repo (`android/`) oder in einem eigenen? | selbes Repo | Stufe 2a |
| F-2 | Klickbares Mockup der TV-Oberfläche vor dem Bau? | ja | Stufe 2b |
| F-3 | Kopplung aus dem Internet: erlauben? Und darf der QR-Code den Benutzercode mitbringen? | ab Werk nur aus dem Heimnetz, in der Verwaltung umschaltbar; der QR-Code bringt den Code nur im Heimnetz mit. Folge: Der Administrator muss den Adressbereich seines Heimnetzes einmal eintragen (4.6). Bis dahin meldet man den Fernseher mit Passwort an | Stufe 1 |
| F-4 | Wie lange bleibt ein gekoppelter Fernseher ohne Benutzung angemeldet? Sitzungen im Browser: 30 Tage gleitend. | 400 Tage gleitend, wie das Geräte-Cookie | Stufe 1 |
| F-5 | Serveradresse nur von Hand oder auch Suche im Heimnetz? | erste Ausgabe von Hand | Stufe 2a |
| F-6 | Unverschlüsseltes HTTP zum Server erlauben? | ja, mit Hinweis, wenn die Adresse nicht im Heimnetz liegt | Stufe 2a |
| F-7 | Paketname der App (unveränderlich nach dem ersten Hochladen) | `com.slouchcat.app`, neutral, falls später eine Handy-App denselben Store-Eintrag nutzen soll | erstes Hochladen (Stufe 3) |
| F-8 | Dolby Vision Profil 5 und 7, getrennte Meldung für HDR10, HLG und Dolby Vision, Umschalten der Bildfrequenz in der ersten Ausgabe? | nein, spätere Stufe; ausser Stufe 0 zeigt ein Problem mit dem einen HDR-Schalter | Stufe 3 |
| F-9 | Übergangszeit für brechende Vertragsänderungen an Operationen der Gerätesitzung | zwei Minor-Releases des Servers | Stufe 6, empfohlen ab Stufe 2a |
| F-10 | Vorführserver für Googles Prüfer: wo und mit welchem Inhalt? | kleiner Server im Internet mit frei lizenzierten Testfilmen, nur für die Prüfung | Stufe 6 |
| F-11 | Hochladen in den Play Store von Hand oder aus der CI (braucht einen Dienstschlüssel von Google als Secret)? | von Hand, solange privat | Stufe 3 |
| F-12 | FFmpeg in der App, um Tonformate selbst zu dekodieren? | nein; der Server rechnet um, was das Gerät nicht kann | Stufe 3 |
| F-13 | Kleinste Android-Version (`minSdk`) | nach Stufe 0 aus den drei Geräten ableiten | Stufe 2a |
| F-14 | Reihe «Weiterschauen» auf dem Startbildschirm von Google TV? Sie zeigt Titel ausserhalb der App, für alle im Raum sichtbar. | nicht in der ersten Ausgabe | nach Stufe 5 |
| F-15 | «Meine Geräte» für jedes Konto (sehen und abmelden), nicht nur in der Verwaltung? | ja, mit Stufe 1 | Stufe 1 |
| F-16 | Darf eine Gerätesitzung Wünsche anlegen? Ein untergeschobenes Gerät könnte bis zum Limit des Kontos Downloads auslösen. | ja, E-5 verlangt Wünsche am Fernseher; das Limit je Konto begrenzt den Schaden | Stufe 1 |
| F-17 | Fire TV | eigenes Konzept nach Stufe 6. Neue Fire-TV-Geräte mit Vega OS führen keine Android-Apps aus | – |

## 14. Kritiker-Review (Fassung 1)

Ein getrennter Kritiker hat Fassung 1 am 19. November 2026 gegen den Code am Anker gelesen:
**REVISE, 0 Blocker, 9 Major, 8 Minor.** Die Redaktion hat die tragenden Aussagen am Code
nachgeprüft (Tonkopie beim Transcode, `getHealth`, Zeilenzahl des Vertrags, `hlsSupport`, Arten der
Benachrichtigungen, Liste der `account`-Operationen) und alle Befunde eingearbeitet.

| Nr. | Stufe | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Major | Sperrliste der Gerätesitzung zu kurz: Profilpflege, PINs, Meldekanäle, Sehbericht und Offline standen offen | 4.5: Erlaubnisliste im Vertrag |
| 2 | Major | Meldung «beim Konto» gibt es nicht; die gekoppelte Sitzung könnte sie als gelesen markieren | K-9: Meldung über E-Mail oder Telegram |
| 3 | Major | «Anmeldung in der TV-App ergibt eine Gerätesitzung» war über `login` nicht durchsetzbar | 4.4: eigene Operation, ehrliche Einordnung |
| 4 | Major | Cookie statt Bearer widersprach Plan 3.8 Punkt 7 und ADR 0005, ohne Zeile in Abschnitt 11 | 4.2: Gerätetoken als Bearer; Abschnitt 11 |
| 5 | Major | Schutz gegen untergeschobene Codes schwach: Gerätename vom Angreifer gewählt, QR-Code mit Code macht es zu einem Klick, Adressen hinter Proxy unbrauchbar | K-5 bis K-7, F-3, Restrisiko in 4.5 |
| 6 | Major | Geräteliste nur für die Verwaltung; Name und Typ erst beim Abspielstart; Sitzung hätte am Handy statt am Fernseher gehangen | 4.2, 4.3, «Meine Geräte» |
| 7 | Major | Mehrkanalton: beim Transcode wird nur AAC kopiert; falsche Codec-Angabe in der HLS-Liste; Bitrate für Stereo | 2.3, 5.3, Abschnitt 7 |
| 8 | Major | Stufen: 0 und 1 hängen doch zusammen; Testspur in Stufe 0 hätte den Paketnamen festgelegt; Stufe 2 zu gross; Belege fehlten | Abschnitt 10, 8.1 |
| 9 | Major | Lücken beim Bau: JDK für `task gen`, CI ohne Pfadfilter, `heavy-slot`, Tags der App, Übersetzungen, AGPL | 3.2, 3.5, 8.2 |
| 10 | Minor | Ein eigenes Kennzeichen für native Player ist unnötig; `mse` passt | 2.3, 5.2 |
| 11 | Minor | Code-Fakten: Zeilenzahl des Vertrags, `getHealth` ohne Vertragsfassung, Fundstellen im Plan, PIN-Zählung, Radio-Token, `deleteDevice` | 2.1 bis 2.5, 8.4 |
| 12 | Minor | Fehlende Zeilen in Abschnitt 11; Regel «Vertrag = Tag» falsch wiedergegeben | Abschnitt 11 |
| 13 | Minor | Zuschreibung: «erste Ausgabe» in Stufe 3, Gestaltungsregeln, Verwaltung in Marcos Spalte | Abschnitte 1, 3.4, 10 |
| 14 | Minor | `minSdk` und 16-KB-Seiten ohne erkennbare Quelle | 2.6: Quelle genannt, als nicht nachgeprüft markiert |
| 15 | Minor | Ratenbegrenzung zielte auf das falsche Risiko | K-4 |
| 16 | Minor | Einzelheiten der Umwandlung von Texten und Tokens fehlten | 3.5 |
| 17 | Minor | Bildspeicher beim Profilwechsel; ein HDR-Schalter für drei Verfahren | 5.5, 5.7, F-8 |

**Zweite Lesung (Fassung 2), derselbe Kritiker: REVISE, 0 Blocker, 2 Major, 6 Minor.** 14 der
17 Befunde galten als erledigt, drei als nur teilweise (Nr. 1, 5, 7); sie stecken in den neuen
Befunden. Die Zuschreibung an Marco war sauber.

| Nr. | Stufe | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Major | Die Erkennung «Heimnetz» versagt hinter einem Reverse-Proxy: dort kommt jede Anfrage von einer privaten Adresse | 4.6: drei Bedingungen, eingetragener Heimnetz-Bereich, Warnung beim Start; F-3 |
| 2 | Major | Erlaubnisliste: Kopplung bestätigen und «Meine Geräte» müssen einer Gerätesitzung verwehrt sein; die Streaming-Routen nehmen ihre Sichtbarkeit aus `binaryRoutes` | 4.5 |
| 3 | Minor | Bearer-Weg: kein `Set-Cookie`, Vorrang bei Cookie und Bearer, CSRF-Kopf, Gültigkeit je Sitzung | 4.2, Abschnitt 7 |
| 4 | Minor | «Kopplung beginnen» schreibt ohne Anmeldung eine Gerätezeile | 4.3 |
| 5 | Minor | Die App muss `containers` senden, sonst nur MP4 direkt | 5.2 |
| 6 | Minor | Kopiertes Dolby Digital (Plus) sprengt die Bandbreitenrechnung; die Codec-Angabe dafür stimmt schon | 2.3, 5.3, Abschnitt 7 |
| 7 | Minor | Stufe 0 bekommt Mehrkanal-HLS noch nicht vom Server | Abschnitt 10 |
| 8 | Minor | Anmelden mit Passwort braucht denselben Zähler wie `login` | 4.4 |

Die Einarbeitung der zweiten Lesung hat der Kritiker nicht mehr gegengelesen.

## Anhang A. Quellen der Recherche (19. November 2026)

- TV-Qualitätsliste: <https://developer.android.com/docs/quality-guidelines/tv-app-quality>
- Ziel-API: <https://support.google.com/googleplay/android-developer/answer/11926878>
- Testpflicht für private Konten: <https://support.google.com/googleplay/android-developer/answer/14151465>
- Testspuren: <https://support.google.com/googleplay/android-developer/answer/9845334>
- Compose for TV: <https://developer.android.com/jetpack/androidx/releases/tv>
- Leanback: <https://developer.android.com/jetpack/androidx/releases/leanback>
- Media3: <https://developer.android.com/jetpack/androidx/releases/media3>
- ExoPlayer, unterstützte Formate: <https://developer.android.com/media/media3/exoplayer/supported-formats>
- FFmpeg-Erweiterung von Media3: <https://github.com/androidx/media/blob/release/libraries/decoder_ffmpeg/README.md>
- Kopplung per Code als Vorbild (Jellyfin Quick Connect): <https://jellyfin.org/docs/general/server/quick-connect/>
- Entwicklerprüfung für Installationen ausserhalb des Stores: <https://support.google.com/android-developer-console/answer/16561738>
- Fire OS: <https://developer.amazon.com/docs/fire-tv/fire-os-overview.html>
- Vega OS: <https://developer.amazon.com/apps-and-games/blogs/2025/09/announcing-vega-os>

Unbelegt geblieben: Verhalten der internen Testspur auf einem Fernseher, Durchreichen von
Tonformaten je Gerät, PGS in ExoPlayer, Tempo von Compose auf schwachen Sticks, ob die Testpflicht
für reine TV-Apps gilt. Bis auf den ersten und den letzten Punkt misst Stufe 0.
