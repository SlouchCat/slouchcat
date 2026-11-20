# Konzept: Android-TV-App – native App für den Fernseher mit eigenem Player, Einrichten per Scan und Mehrkanalton

- **Status: Entwurf, Fassung 5 (19. November 2026). Nicht freigegeben.** Fassung 2 und 3 haben
  zwei Kritiker-Lesungen eingearbeitet (Abschnitt 14). Danach hat Marco mit E-10 bis E-12 die
  Anmeldung und die Reihenfolge neu festgelegt: Einrichten per Scan mit einer nativen Handy-App
  über einen Dienst im Internet, die Handy-App wird zuerst gebaut. **Fassung 4 gleicht dieses
  Papier an Konzept 27 «Native Android-Handy-App» an** (dort Fassung 3, Zweig
  `claude/konzept-android-handy`, Stand `b468ee57`, ebenfalls nicht freigegeben) und arbeitet die
  Befunde ein, die der Kritiker dort zu Abschnitt 4 dieses Papiers hatte (2 Major, 7 Minor).
  **Fassung 5** arbeitet die dritte Lesung dieses Papiers ein (REVISE, 0 Blocker, 1 Major,
  8 Minor; Abschnitt 14). Der Kritiker hat dabei Anhang B gegen den Vertrag geprüft: alle 116
  Operationen stehen genau einmal und mit der richtigen Sichtbarkeit darin. Die Einarbeitung der
  dritten Lesung ist nicht gegengelesen. Verweise «K27 4.2» meinen Abschnitte von Konzept 27. Gebaut
  wird erst nach Marcos Freigabe und auf seinen eigenen Auftrag je Stufe.
- **Auslöser:** Marco am 19. November 2026: «Kannst du mir ein slouchcat überall Accounts erstellen
  welche ich für die künftige AndroidTV App benötige?» und danach «Wir starten mit AndroidTV». Auf
  die Rückfrage, was damit gemeint ist, hat er «Konzept starten» gewählt.
- **Verankert gegen:** `main` **`d70c1c79`** («Merge pull request #188», Release 1.41.2),
  Vertragsfassung **1.41.0** (`api/openapi.yaml:4`), letzte Migration
  **`00081_wishes_request_open.sql`**, letzte ADR **`0038`**, letzte Frage im
  Entscheidungsprotokoll **33**.
- **Nummern:** Dieses Papier ist **Konzept 26** und reserviert **Frage 34** sowie **ADR 0039**
  bis **0042**. Drei davon beschreiben das gemeinsame Fundament beider Apps und entstehen mit dem
  Bau von Konzept 27: ADR 0039 («Android-Apps als native Clients im selben Repo»), ADR 0040
  («Sitzungsarten für native Apps und Gerätekopplung»), ADR 0042 («Dienst im Internet für
  Vermittlung und Push»). Bei der TV-App bleibt **ADR 0041** («Ton- und Spurfähigkeiten des
  Geräts in der Wiedergabe-Entscheidung»). Konzept 27 hat Frage 35 und ADR 0043, 0044 reserviert.
  Migrations- und Versionsnummern vergibt der Bau.
- Pfade ohne Präfix liegen unter `web/src/` oder `server/`. **Nach Namen suchen, nicht nach
  Zeile**; Zeilen beziehen sich auf den Anker.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
  Dieses Papier schlägt Änderungen an früheren Entscheiden und an `CLAUDE.md` vor (Abschnitt 11);
  es ändert selbst keine Regel.

> **Dieses Papier ändert keine Zeile Code.** Marcos Entscheide stehen in Abschnitt 1 (E-1 bis
> E-12) und nur dort; E-13 bis E-20 stehen in Konzept 27 und sind hier nur genannt. Alles andere ist Vorschlag der Redaktion, auch wo es nicht eigens dabeisteht.
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
| E-8 | (ohne Frage, von Marco selbst) | «Profile verwalten sollte die App können. Einladungen ebenfalls nicht und Downloads auch nicht. Downloads ist nur wichtig für die Handy Apps.» | Vorschlag war: Profile nur im Browser |
| E-9 | Wie sichert die TV-App das Verwalten der Profile ab (PINs entfernen, Altersfreigaben ändern)? | **«Nur das Hauptprofil das Profile erstellen oder ändern. Falls ein Kinder Profil ist, soll lediglich ein Pin empfohlen werden. Jedes Gerät darf jedoch Profil verwalten (nur mit dem Hauptprofil)»** | Mit PIN des Hauptprofils als Pflicht |
| E-10 | (von Marco selbst) Danach: Welche App am Handy soll scannen? | «Eigentlich möchte ich es so: User lädt sich die Android App herunter und konfiguriert alles. Danach kann er mit dieser App den QR Code auf dem TV scannen und auf dem TV wird alles eingerichtet (sofern mit Internet verbunden).» Auf die Frage: **«Neue native Android-App»** | Bestehende Web-App |
| E-11 | Wie kommen Serveradresse und Anmeldung nach dem Scan auf den Fernseher? | **«Über einen Dienst im Internet»** | Direkt im selben WLAN |
| E-12 | In welcher Reihenfolge wird gebaut? | **«Volle Handy-App zuerst»** | Kleine Begleit-App zusammen mit der TV-App |

Zu E-5: Die Frage hat vorgegeben, dass Filme und Serien gesetzt sind. Dass die Verwaltung im
Browser bleibt, war zuerst ebenfalls Vorgabe der Frage; Marco hat es danach mit E-7 selbst
festgelegt.

Dazu Marcos Hinweis vom selben Tag: «Da dies für alle verfügbar sein soll, muss ja auch irgendwo
der Server eingetragen werden können.» Das ist Abschnitt 4.1.

Zu E-8: Marco spricht von «Handy Apps». Mit E-10 und E-12 hat er die native Android-Handy-App
danach selbst zum Ziel gemacht; sie bekommt ein eigenes Konzept 27 und wird vor der TV-App gebaut.

Zu E-10 bis E-12: Die Folgen standen in den Fragen. Zur Handy-App: «heute ein Nicht-Ziel und wäre
ein zweites grosses Projekt mit eigenem Konzept», «iPhone-Nutzer hätten nichts davon». Zum Dienst:
«Du müsstest diesen Dienst für alle Slouchcat-Nutzer dauerhaft betreiben», «Cloud ist heute ein
Nicht-Ziel», «Fällt er aus, kann niemand per Scan einrichten». Zur Reihenfolge: «Längster Weg bis
zu einer App auf deinem Fernseher». Marcos erster Satz des Tages, «Wir starten mit AndroidTV»,
ist damit überholt.

Zu E-9: Marco hat die Folge gekannt; sie stand in der Frage («Hat das Hauptprofil keine PIN, kann
jeder am Fernseher und jedes fälschlich gekoppelte Gerät PINs entfernen und Altersfreigaben
ändern»). Sie steht als bekannte Grenze in Abschnitt 4.5.

**Aus Konzept 27**, soweit es den Fernseher betrifft (Marcos Wahl wörtlich; die Fragen und Folgen
stehen in K27 Abschnitt 1.2):

| Nr. | Frage | Marcos Wahl |
|---|---|---|
| E-16 | Wer darf einen Fernseher per QR-Scan mit einem Konto verbinden? | «Nur die Handy-App» |
| E-17 | Wie kommen Meldungen aufs Handy, wenn die App zu ist? | «Push über eigenen Dienst» |
| E-18 | Darf sich die Handy-App über unverschlüsseltes HTTP mit einem Server verbinden? | «HTTP überall mit Warnung» |
| E-19 | Wo läuft der Vermittlungsdienst? | eigene Antwort, sinngemäss: vorläufig auf eigener Hardware, später auf einem Mietserver |
| E-20 | Eine App für Handy und Fernseher im Play Store oder zwei? | «Eine App: com.slouchcat.app» |

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
  Reverse-Proxy nur die Adresse des Proxys. Das zählt für alles, was je Adresse begrenzt wird,
  zum Beispiel die Fehlversuche beim Anmelden (4.4).

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
  Test mit mindestens 12 Testern, die 14 Tage am Stück angemeldet bleiben. Er fällt mit der
  Handy-App an (K27 11.3). Ob er für die TV-Geräteart erneut anfällt, ist unbelegt.
- **Interne Testspur:** bis 100 Tester je App über eine Liste von E-Mail-Adressen, kein
  Ablaufdatum genannt. Die Hilfeseite nennt keine Vorbedingung; ob ein Konto, dessen
  Identitätsprüfung noch läuft, schon hochladen kann, ist unbelegt. Ob die Installation aus dieser
  Spur auf einem Fernseher klappt, ist ebenfalls unbelegt.
- **Eine App, zwei Gerätearten:** Die TV-App kommt als zweite Geräteart mit eigener
  Veröffentlichungsspur in den Store-Eintrag der Handy-App (E-20, K27 11.1).
- **TV-Prüfung** (TV-Qualitätsliste): Eintrag für den TV-Startbildschirm (`LEANBACK_LAUNCHER`),
  Banner 320 × 180 mit App-Namen, alles mit der Fernbedienung bedienbar, Zurück führt bis zum
  Startbildschirm, Taste Wiedergabe/Pause wirkt, kein Bildschirmschoner während der Wiedergabe,
  **Test-Zugangsdaten für Googles Prüfer**.
- **Technik:** App-Bundle (AAB) ist Pflicht; Ziel-API für TV-Apps mindestens 34 (für Handy-Apps
  36). Aus derselben TV-Qualitätsliste (Punkte TV-PS und TV-G6, vom Rechercheur gelesen, hier nicht
  nachgeprüft): `minSdk` höchstens 31, und seit 1. August 2026 Unterstützung für 32 und 64 Bit
  sowie für 16-KB-Speicherseiten. Mit Ziel-API 37 braucht eine App die Berechtigung
  `ACCESS_LOCAL_NETWORK`, um Adressen im Heimnetz zu erreichen (K27 2.7).
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

Das regelt Konzept 27 für beide Apps (K27 Abschnitt 5); die Handy-App wird zuerst gebaut und legt
es an. Für die TV-App heisst das:

- Sie ist das Modul **`app-tv`** im Projekt `android/` und nutzt die gemeinsamen Bausteine:
  Kotlin-Client aus dem Vertrag, Anmeldung und Netz, Player-Grundlage (Media3), Farben und Masse
  aus den Tokens, Texte, das Paket für die Kopplung. Ihre Seiten baut sie selbst mit «Compose for
  TV»; Oberflächen teilen die beiden Apps nicht.
- Paketname **`com.slouchcat.app`**, derselbe wie bei der Handy-App (Marcos Entscheid E-20 in
  K27). Die TV-App ist ein eigenes Artefakt mit eigener Versionsnummer und kommt als zweite
  Geräteart in denselben Store-Eintrag.
- Erzeugter Code wird nicht eingecheckt; Client, Farben und Texte entstehen beim Bau der App.
  Regel 1 bekommt keine neuen Verzeichnisse, und wer nur Server oder Web-App ändert, braucht kein
  JDK.
- Werkzeuge, `task android:check`, `task android:test`, der CI-Job `android` und die Frage eines
  Pfadfilters stehen in K27 5.7. Ob Oberflächentests in der CI auf einem Emulator laufen und wie
  lange, misst dort Stufe 3.
- Releases der TV-App bekommen Tags der Form `tv-v1.0.0`. Sie berühren weder das Release-Workflow
  des Servers (`v*.*.*`) noch die Regel «Vertrag = Tag».
- Das Repo steht unter AGPL-3.0, die App damit auch. Media3 und Compose stehen unter Apache 2.0.

### 3.3 Seiten

| TV-Seite | Vorbild in der Web-App | Stufe |
|---|---|---|
| Einrichten per Scan: QR-Code für die Handy-App (E-10) | neu | 2a |
| Einrichten von Hand: Serveradresse, E-Mail und Passwort | `/login` | 2a |
| Profilwahl mit PIN | `/profiles` | 2a |
| Start (Spotlight, Reihen) | `/` | 2b |
| Bibliothek (Raster, Filter, Sortierung) | `/libraries/$libraryId` | 2b |
| Film, Serie, Staffel, Folge | `/items/$itemId` | 2b |
| Person | `/people/$id` | 2b |
| Suche | `/search` | 2b |
| Merkliste, Gesehen, Sammlungen, Listen, Empfehlungen | `/watchlist`, `/watched`, `/collections`, `/lists/$key`, `/picks` | 2b |
| Profile verwalten: anlegen, ändern, löschen, PIN setzen und entfernen (E-8). Ohne Sehbericht (Frage F-18) und ohne Offline-PIN (E-8), die die Web-Seite an derselben Stelle zeigt | `/settings/profiles` | 2b |
| Einstellungen der Wiedergabe | `/settings/playback` | 3 |
| Videoplayer | `/play/$itemId` | 3 |
| Hörbuch, Hörbuch-Reihe, Podcast, Tonplayer | `/libraries/$id/series/$sid`, Tonplayer | 4 |
| Webradio | `/radio` | 4 |
| Wünsche, Entdecken, Titel ausserhalb der Bibliothek | `/wishes`, `/discover…`, `/titles/$media/$tmdbId` | 5 |
| Benachrichtigungen (nur die Liste) | `/notifications` | 5 |

**Nicht in der App:**

- nach Marcos Entscheid: die Verwaltung (`/admin/*`, E-7), Einladungen und Downloads samt Offline
  (E-8)
- als Vorschlag: Ersteinrichtung, Meldekanäle samt Telegram und Web-Push, Sehbericht,
  Kontodaten ändern (Frage F-18)

Wer das braucht, nimmt den Browser.

### 3.4 Bedienung und Gestaltung

- Alles mit Steuerkreuz, OK und Zurück. Zurück schliesst zuerst Menüs, dann die Seite, auf der
  Startseite die App (Vorgabe von Google).
- Der Fokus ist immer sichtbar. Jede Reihe merkt sich ihre letzte Kachel.
- Texteingabe nur an vier Stellen: Serveradresse und Anmeldung beim Einrichten von Hand, Suche,
  Namen beim Verwalten der Profile. Die Suche bekommt zusätzlich die Spracheingabe des Geräts, wo
  sie vorhanden ist.
- Vorschlag: Marcos Gestaltungsregeln vom 16. November 2026 für die Web-App auch hier anwenden.
  Klappbares ist zu, Kontrast hoch (Rahmen mindestens 3:1, gedämpfter Text mindestens 8:1).
- Schrift und Abstände für Sofa-Abstand: eigene Tokens für den Fernseher, keine Zahl im Code.

**Mockup vor dem Bau (Frage F-2):** Vorschlag ist ein klickbares Mockup in 1920 × 1080, mit den
Pfeiltasten bedienbar, für Einrichten per Scan und von Hand, Profilwahl, Profile verwalten, Start,
Bibliothek, Detailseite und Player-Bedienung. Marco gibt es frei, bevor Stufe 2b beginnt.

### 3.5 Texte und Farben (Regel 9)

Die Generatoren baut Konzept 27 (K27 5.4 bis 5.6). Für die TV-App gilt:

- **Texte:** Quelle bleiben `web/src/locales/*.json` (22 Sprachen und die Ergänzung `de-DE`). Der
  Generator schreibt daraus beim Bau `strings.xml`. Texte, die nur der Fernseher braucht, kommen
  unter einem eigenen Namensraum in dieselben Dateien; der Übersetzungsweg aus ADR 0033 und
  `docs/i18n/README.md` bleibt einer. Vor jedem Release der App werden die Übersetzungen
  nachgezogen.
- **Farben und Masse:** aus `packages/design-tokens`, dazu eigene Tokens für den Fernseher
  (Schrift und Abstände für Sofa-Abstand).
- **Wächter:** Fester Text und Farbwerte ausserhalb des erzeugten Codes sind ein Fehler. Ob die
  mitgelieferte Prüfung von Android das für Compose leistet, ist unbelegt; sonst braucht es eine
  eigene Lint-Regel (K27 5.6).
- **Regel 12 gilt unverändert:** Die App entscheidet nichts über Sichtbarkeit oder Alter. Sie zeigt,
  was der Server liefert; nicht sichtbar heisst 404.

## 4. Anmeldung (E-4, E-10, E-11, E-16)

Der Nutzer richtet zuerst die **Android-Handy-App** ein und scannt damit den QR-Code am
Fernseher; der Fernseher richtet sich danach selbst ein (E-10 bis E-12). Dieser Abschnitt
beschreibt, was die **TV-App** dafür tut. Was Handy-App, Server und Dienst dafür tun, steht in
Konzept 27 (K27) und wird dort gebaut; dieser Abschnitt verweist darauf, statt es zu wiederholen.

Es gibt zwei Wege:

| Weg | Was der Nutzer tut | Braucht |
|---|---|---|
| **Per Scan** (Hauptweg, E-10) | QR-Code am Fernseher mit der Handy-App scannen, bestätigen | Handy-App, Vermittlungsdienst, Internet an beiden Geräten |
| **Von Hand** (Ausweichweg) | Serveradresse, E-Mail und Passwort am Fernseher eintippen | nichts weiter |

Die Bestätigung im Browser über eine Seite `/link` mit abgetipptem Code, wie sie die Fassungen 1
bis 3 vorsahen, entfällt. Das ist aus E-10 und E-16 abgeleitet, nicht eigens entschieden
(Frage F-3).

### 4.1 Serveradresse

- **Per Scan:** Die Handy-App gibt die Adresse mit, unter der sie selbst den Server erreicht. Am
  Fernseher tippt man nichts. Erreicht der Fernseher diese Adresse nicht (zum Beispiel eine
  VPN-Adresse des Handys), zeigt er sie an und bietet das Einrichten von Hand an (K27 Abschnitt 10).
- **Von Hand:** Die TV-App fragt nach der Adresse, prüft sie mit `getHealth` und zeigt Adresse,
  Version und ob die Verbindung verschlüsselt ist.

Die Adresse bleibt gespeichert; wechseln geht in den Einstellungen.

- **Unverschlüsseltes HTTP (Frage F-6):** Für die Handy-App hat Marco «HTTP überall mit Warnung»
  gewählt (E-18 in K27). Vorschlag: Die TV-App hält es gleich, damit ein Server, den das Handy
  erreicht, auch am Fernseher geht. Die Regel sitzt im gemeinsamen Netzbaustein beider Apps.
- **Android 17:** Sobald die App auf API 37 zielt, braucht sie die Berechtigung
  `ACCESS_LOCAL_NETWORK`, um einen Server im Heimnetz zu erreichen (K27 Abschnitt 6). Die erste
  Ausgabe der TV-App zielt auf API 34.

### 4.2 Gerätesitzung und Gerätetoken

Mit den Apps gibt es drei Sitzungsarten: Browser-Sitzung, App-Sitzung (Handy) und
**Gerätesitzung** (Fernseher). Die Tabelle dazu, die Regeln für den Bearer-Weg und die
Gerätekennung stehen in K27 4.1 bis 4.3. Für den Fernseher gilt:

- Die TV-App bekommt nie eine volle Sitzung, sondern ein **Gerätetoken**: 256 Bit Zufall, im
  Server nur gehasht, in der App verschlüsselt abgelegt, gesendet als `Authorization: Bearer`.
- Die Art steht in der Sitzung, und der Träger muss dazu passen: Ein Gerätetoken im Cookie wird
  abgelehnt, ebenso ein Sitzungs-Cookie zusammen mit einem Bearer.
- Der Server legt die Gerätezeile mit Name und Art `tv` an, sobald er das Token ausgibt. Der Name
  ist der, den der Fernseher beim Einrichten nennt; der Abspielstart überschreibt ihn nicht mehr.
- Der CSRF-Kopf `X-Requested-With: Slouchcat` bleibt Pflicht für jede schreibende Anfrage.
- Auch die Streaming-Routen aus `binaryRoutes` (Bilder, Vorschaubilder, Radio-Bilder) nehmen das
  Token an. Medien-Adressen brauchen es nicht; sie tragen `st` oder `rt`.
- **Das Token läuft nicht ab** (Marcos Antwort auf F-4: «Unbegrenzt»). Im Server ist das ein
  Ablauf weit in der Zukunft, den die Benutzung nicht verschiebt.
- **Abgemeldet wird ein Fernseher** unter «Meine Geräte», in der Verwaltung («Gerät vergessen»)
  oder durch eine Passwortänderung, die heute jede andere Sitzung entzieht (K27 Frage F-24). Nach
  einer Passwortänderung wird also jeder Fernseher neu verbunden.
- Ein Fernseher ohne Kontakt seit 400 Tagen verlöre heute seine Gerätezeile durch die Aufräumung,
  obwohl er angemeldet bliebe. Stufe 1 von K27 nimmt Geräte mit einer Sitzung ohne Ablauf davon
  aus.

### 4.3 Einrichten per Scan: die Seite des Fernsehers

1. Die TV-App erzeugt ein Schlüsselpaar, das nur für diese eine Kopplung lebt, und ein
   **Abholgeheimnis**, das den Fernseher nie verlässt.
2. Sie legt beim Vermittlungsdienst einen Treffpunkt an und nennt dabei nur den Hash des
   Abholgeheimnisses. Sie bekommt eine **Treffpunkt-Kennung**, gültig 5 Minuten.
3. Der Fernseher zeigt einen QR-Code. Er ist eine `https`-Adresse auf eine Hilfeseite; hinter dem
   `#` stehen die Adresse des Dienstes, die Kennung, der öffentliche Schlüssel und eine
   Fassungsnummer. Wer ihn mit der Kamera statt mit der Slouchcat-App scannt, landet auf «Öffne die
   Slouchcat-App und wähle Fernseher verbinden». Daneben steht derselbe Satz am Bildschirm.
4. Der Nutzer scannt den Code in der Handy-App und bestätigt. Die Handy-App holt beim Server
   einen Kopplungsgutschein für den Schlüssel des Fernsehers, verschlüsselt Serveradresse und
   Gutschein für diesen Schlüssel und legt das Paket beim Dienst ab (K27 Abschnitt 10 und 7.2).
5. Der Fernseher fragt beim Dienst nach, holt das Paket mit dem Abholgeheimnis ab und
   entschlüsselt es.
6. Er schickt Gutschein, öffentlichen Schlüssel, seinen Gerätenamen und die Fassung seines
   Systems **direkt an den Server**; die Art `tv` steckt im Gutschein. Der Server prüft,
   dass der Gutschein an genau diesen Schlüssel gebunden ist, und antwortet mit dem Gerätetoken,
   verschlüsselt für diesen Schlüssel. Lesen kann es nur der Fernseher.
7. Der Fernseher zeigt die Profilwahl und dabei **Serveradresse und Konto**, mit denen er
   verbunden ist.

Läuft etwas nicht (Dienst nicht erreichbar, Treffpunkt abgelaufen, Server nicht erreichbar),
zeigt der Fernseher einen neuen Code oder das Einrichten von Hand.

Das Protokoll des Dienstes steht in K27 7.1, die Verschlüsselung (HPKE, RFC 9180) in K27 7.2. Der
Dienst sieht vom Paket nur Geheimtext und spricht nie mit dem Server des Nutzers.

### 4.4 Einrichten von Hand

Die TV-App fragt Serveradresse, E-Mail und Passwort ab und ruft die Operation **«App anmelden»**
mit der Art `tv` auf (K27 4.2). Es ist dieselbe Operation, mit der sich die Handy-App anmeldet;
die Art `tv` ergibt eine Gerätesitzung. Fehlversuche zählen im selben Topf wie bei `login`; er
zählt je Adresse des Aufrufers und gilt auch für Ersteinrichtung, PIN und Einladungen.

Dieser Weg braucht keine Handy-App und keinen Dienst. Er ist der Ausweichweg, wenn der Dienst
ausfällt, für Nutzer ohne Android-Handy und für Googles Prüfer (Abschnitt 8.2).

Ehrlich gesagt: Welche Art ein Client nennt, entscheidet der Client. Wer das Passwort kennt, kann
sich mit jedem Programm über `login` voll anmelden. Die Gerätesitzung begrenzt den Schaden eines
verlorenen Geräts oder Tokens und einer untergeschobenen Kopplung, nicht den eines bekannten
Passworts.

### 4.5 Was eine Gerätesitzung darf

Eine Gerätesitzung darf nur, was auf einer **Erlaubnisliste** steht. Alles andere beantwortet der
Server mit 403. Im Vertrag steht das als Liste `x-clients` je Operation (K27 4.4): `tv` nennt die
Operationen, die ein Fernseher aufrufen darf. Fehlt die Angabe, darf er nicht.

**Anhang B zählt jede der 116 Operationen des Nutzerbereichs auf.** Die Zusammenfassung:

| Bereich | Fernseher | Grund |
|---|---|---|
| Sitzung, Profilwahl | ja | nötig |
| Profile verwalten samt PIN | ja, wenn das Hauptprofil aktiv ist | E-8, E-9; die Bedingung prüft der Server schon heute |
| Stöbern, Suchen, Abspielen, Fortschritt, Merkliste, «Gesehen», Sammlungen | ja | E-5 |
| Hörbücher, Podcasts, Webradio | ja | E-5 |
| Wünsche: ansehen, anlegen, zurückziehen | ja | E-5, Antwort auf F-16 |
| Empfehlen, Benachrichtigungen lesen und als gelesen markieren, Ankündigungen bestätigen, Vorlieben der Wiedergabe | ja | Vorschlag: gehört zum Nutzerbereich (E-7) |
| Downloads und Offline samt Offline-PIN | nein | E-8 |
| Verwaltung (`admin`, `owner`) | nein | E-7 |
| Kontodaten, Meldekanäle samt Telegram und Web-Push, Sehbericht | nein | Vorschlag (Frage F-18) |
| Kopplungsgutschein ausstellen, «Meine Geräte» | nein | E-16; sonst könnte ein gekoppeltes Gerät das nächste freigeben oder die echten Fernseher abmelden |

**Meine Geräte** (Marcos Antwort auf F-15: «Ja»): Jedes Konto sieht in den Einstellungen der
Web-App und der Handy-App seine Geräte und kann jedes abmelden (K27 4.5). Weil Gerätetoken nicht
ablaufen, ist das neben der Passwortänderung der Weg, ein altes Gerät loszuwerden.

**PIN-Empfehlung (E-9):** Gibt es im Konto ein Profil mit Altersgrenze und hat das Hauptprofil
keine PIN, empfiehlt die App beim Verwalten der Profile, dem Hauptprofil eine PIN zu geben. Sie
verlangt es nicht. Deutung der Redaktion: Marcos «ein Pin empfohlen» meint die PIN des
Hauptprofils, weil nur sie verhindert, dass ein Kind ins Hauptprofil wechselt; Marco kann das beim
Freigeben richtigstellen. Ob Web-App und Handy-App denselben Hinweis bekommen, ist Frage F-19.

**Restrisiko, damit es benannt ist:** Eine Gerätesitzung kann alles ansehen, was die Profile des
Kontos ohne PIN dürfen. Sie kann Wünsche anlegen; ein Wunsch löst über Seerr einen echten Download
aus, das Limit je Konto aus Konzept 25 begrenzt die Menge. Sie kann Titel empfehlen und
Benachrichtigungen in der App als gelesen markieren; die Meldung aus K-7 kommt darum ausserhalb
der App.

**Bekannte Grenze nach E-9:** Hat das Hauptprofil keine PIN, kann jeder mit der Fernbedienung
und jedes fälschlich gekoppelte Gerät ins Hauptprofil wechseln und dort Profile anlegen, ändern
und löschen, PINs anderer Profile entfernen und Altersfreigaben heben. Im Browser ist das heute
genauso. Der Kritiker hatte dafür eine Sperre verlangt (Abschnitt 14, Runde 1, Befund 1); Marco
hat sich am 19. November 2026 in Kenntnis der Folge dagegen entschieden. Schutz bietet allein
eine PIN am Hauptprofil.

### 4.6 Sicherheit der Kopplung

Die Kopplung ist ein neuer Weg zu einer Sitzung und damit der heikelste Teil dieses Konzepts.

| Nr. | Regel | Schützt gegen |
|---|---|---|
| K-1 | Treffpunkt-Kennung: 128 Bit Zufall, gültig 5 Minuten. **Abholen kann nur, wer das Abholgeheimnis kennt;** es steht nicht im QR-Code | Raten; Abholen oder Verbrauchen durch jemanden, der den QR-Code fotografiert |
| K-2 | Das Paket ist für den Schlüssel des Fernsehers verschlüsselt und an die Kennung gebunden. Der Dienst sieht nur Geheimtext, dazu Adressen und Zeitpunkt der beiden Geräte | Mitlesen durch den Dienst oder auf dem Weg |
| K-3 | Kopplungsgutschein: 256 Bit Zufall, im Server nur gehasht, gültig 2 Minuten, einmal verwendbar, an den Schlüssel des Fernsehers gebunden. Das Gerätetoken kommt verschlüsselt für diesen Schlüssel zurück | abgefangene Gutscheine |
| K-4 | Dienst: Paket höchstens 4 KB, Obergrenze für offene Treffpunkte gesamt und je Aufrufer, kein Konto, keine Datenbank, nichts bleibt nach Abholen oder Ablauf | Missbrauch als Speicher, Überfluten |
| K-5 | Die Handy-App nimmt den Code nur aus ihrem eigenen Scanner an, nie als Link, zeigt Konto und Server und fragt mit Warnung nach | untergeschobene QR-Codes |
| K-6 | Gerätesitzung nur mit Erlaubnisliste (4.5) | Schaden, falls doch ein fremdes Gerät gekoppelt wird |
| K-7 | Jede Kopplung geht als Meldung über die Kanäle des Kontos hinaus (E-Mail, Telegram, mit E-17 auch Push aufs Handy) und steht in «Meine Geräte». Die Meldung nennt, was der Server selbst weiss, und kennzeichnet den Namen als Angabe des Geräts | unbemerkte Kopplung |
| K-8 | Gutschein, Gerätetoken, Abholgeheimnis und Treffpunkt-Kennung kommen in die Redaktionsliste der Logs (Regel 10) und erscheinen in keinem Log der App | Geheimnisse in Logs |
| K-9 | Der Fernseher zeigt nach dem Einlösen Serveradresse und Konto | ein untergeschobenes Paket bleibt nicht unbemerkt |

Bekannte Grenzen:

- **Untergeschobener QR-Code:** Ein Angreifer kann auf seinem eigenen Gerät einen QR-Code erzeugen
  und ihn einem Kontoinhaber als Bild schicken. Scannt dieser ihn in der Handy-App und bestätigt
  trotz K-5, schaut der Angreifer mit (Restrisiko in 4.5). K-6 und K-7 begrenzen den Schaden und
  machen ihn sichtbar, sie verhindern ihn nicht.
- **Untergeschobenes Paket:** Ablegen kann jeder, der die Kennung kennt: wer den QR-Code
  fotografiert, und der Dienst selbst. Er kann dem Fernseher ein eigenes Paket unterschieben,
  bevor das echte Handy seines ablegt, und ihn so mit einem **fremden** Server und Konto
  verbinden. Lesen kann er dabei nichts. K-9 macht es sichtbar; verhindern lässt es sich nicht.
  Der Dienst meldet dem echten Handy in diesem Fall, dass am Treffpunkt schon ein Paket liegt.
  Vorschlag für K27: Die Handy-App warnt dann, dass der Fernseher womöglich schon mit einem
  fremden Server verbunden ist, und verweist auf die Anzeige am Fernseher.
- **HTTP und ein Token ohne Ablauf:** Läuft der Server ohne TLS, reist das Gerätetoken nach dem
  Einlösen in jeder Anfrage offen. Wer mitliest, hat ein Token, das gilt, bis jemand den
  Fernseher abmeldet oder das Passwort ändert. Die Meldung aus K-7 schlägt dabei nicht an, weil
  kein neues Gerät entsteht. Ein Fernseher steht fast immer im Heimnetz; die Lage ist dort
  günstiger als beim Handy, aber nicht anders.
- **Server weist sich aus:** K27 schlägt vor, dass die App bei einem Server über HTTP zuerst
  prüft, ob es der richtige ist (K27 Abschnitt 6, Frage F-38). Wird das bejaht, prüft auch die
  TV-App so; der Schlüssel des Servers reist im Paket der Kopplung mit.

Der Kritiker hat diesen Abschnitt in der Fassung vom 19. November 2026 zusammen mit K27 gelesen
(2 Major, 7 Minor; Abschnitt 14).

### 4.7 Der Dienst im Internet (E-11, E-17, E-19)

Marco hat am 19. November 2026 «Über einen Dienst im Internet» gewählt, in Kenntnis der Folgen
aus der Frage: Er betreibt den Dienst dauerhaft für alle Slouchcat-Nutzer, «Cloud» ist heute ein
Nicht-Ziel, und fällt der Dienst aus, kann niemand per Scan einrichten. Die Frage nannte ausserdem
«er sieht ihre Serveradressen»; mit der Verschlüsselung aus K-2 sieht die Vermittlung sie nicht.

Alles Weitere steht in K27 Abschnitt 7 und wird dort gebaut. Für die TV-App zählt:

- Es sind zwei getrennt betriebene Dienste: die Vermittlung beim Einrichten (E-11) und die
  Weiterleitung von Push-Meldungen an die Handy-App (E-17). Die TV-App braucht nur die
  Vermittlung.
- Marco am 20. November 2026, sinngemäss (das Repo ist öffentlich, darum ohne Ortsangaben): Die
  zwei Dienste sollen vorerst auf zwei eigenen Containern bei ihm laufen und sich ohne Mühe auf
  zwei Mietserver verschieben lassen. Jeder Dienst läuft also vorläufig auf einem eigenen
  Container zu Hause und später auf einem eigenen Mietserver (E-19). Zu Hause sind sie über seinen vorhandenen Reverse-Proxy
  erreichbar; so hat er es am selben Tag gewählt, gegen die Empfehlung eines Tunnels. Wie der
  Aufbau den Umzug einfach hält (dasselbe Image und dieselbe Compose-Datei an beiden Orten, kein
  Zustand auf der Platte, feste Namen mit kurzer DNS-Lebensdauer), beschreibt K27 7.5.
- Folge für die TV-App: Während ein Dienst umzieht, können Handy und Fernseher für kurze Zeit bei
  verschiedenen Instanzen der Vermittlung landen. Eine Kopplung scheitert dann einmal; der
  Fernseher zeigt einen neuen Code (4.3).
- Vorschlag aus K27, dort noch offene Fragen: Der Dienst hat einen festen Namen, der in den Apps
  eingebaut ist (K27 F-37), und die Adresse der Vermittlung ist in den erweiterten Einstellungen
  änderbar (K27 F-21). Handy und Fernseher müssen dann denselben Dienst eingestellt haben, weil
  die Handy-App einen QR-Code mit einem anderen Dienst ablehnt.
- Was der Dienst sieht, steht in K27 7.6; was bei einem Ausfall bleibt, in K27 7.7. Für den
  Fernseher bleibt das Einrichten von Hand. Eingerichtete Fernseher merken von einem Ausfall
  nichts.

Der Dienst ist eine benannte Ausnahme vom Nicht-Ziel «Cloud» (Abschnitt 11) und bekommt ADR 0042.

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
entsprechend ergänzt, sein Verhalten nicht. Der Name aus dem Fähigkeitsprofil überschreibt den
Gerätenamen nicht mehr; es bleibt der Name vom Einrichten (K27 4.2).

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

Der Serverteil für Anmeldung, Sitzungsarten, Kopplung, «Meine Geräte», Meldung «Neues Gerät»,
Vertragsfassung in `getHealth` und Redaktionsliste wird mit **Stufe 1 von Konzept 27** gebaut
(K27 15.1), der Dienst mit dessen Stufe 2. Die TV-App findet beides fertig vor.

**Dazu gehört die Angabe `tv` an den 93 Operationen aus Anhang B.** K27 Stufe 1 schafft die
Gerätesitzung und den Test, der jede Operation mit einem TV-Token aufruft. Ohne die Angabe bekäme
eine Gerätesitzung auf alles 403, auch auf die Profilwahl, und der Test hätte nichts zu prüfen.
Anhang B ist darum die Vorlage für K27 Stufe 1, und Frage F-18 wird dort fällig.

Bei der TV-App bleibt:

| Was | Wo | Art |
|---|---|---|
| `PlaybackCapabilities`: drei Ergänzungen (5.2), Beschreibung von `hls` | Vertrag, `playback/decide.go` | zusätzlich |
| Ton: richtige Codec-Angabe in der HLS-Liste, Kopieren von Dolby Digital (Plus) beim Transcode samt Bandbreitenrechnung mit echter Bitrate, Mehrkanal beim Neu-Kodieren | `playback/decide.go`, `playback/quality.go`, `transcode/plan.go`, `transcode/pipeline.go` | Verhalten ändert sich nur für Clients, die diese Formate oder Kanäle melden; die Codec-Angabe wird für alle richtig |

Das ist eine zusätzliche Vertragsänderung (keine brechende) und ein eigenes Release des Servers
(Stufe 1 dieses Papiers).

## 8. Verteilung (E-2, E-20)

Den Weg in den Play Store geht zuerst die Handy-App (K27 Abschnitt 11). Die TV-App hängt sich an.

### 8.1 Privat

1. **Während des Baus:** Installation über ADB im Heimnetz. Marco schaltet dafür auf jedem Gerät
   einmal die Entwickleroptionen ein. Entwicklungsbauten tragen einen anderen Paketnamen und
   werden nie hochgeladen.
2. **Interne Testspur:** Paketname, Signatur und Testspur stehen dann schon von der Handy-App.
   Die TV-App kommt als zweite Geräteart mit eigener Veröffentlichungsspur in denselben Eintrag.
   Diese Wahl ist nach Angabe von K27 unumkehrbar. Ob die Installation aus der internen Testspur
   auf einem Fernseher klappt, ist unbelegt und wird beim ersten Hochladen der TV-App geprüft.

Ob ein Entwicklerkonto, dessen Identitätsprüfung noch läuft, schon in die interne Testspur
hochladen kann, ist unbelegt. Die Hilfeseite nennt für die Spur keine Vorbedingung.

### 8.2 Öffentlich (spätere Stufe)

- Der geschlossene Test mit 12 Testern über 14 Tage fällt mit der Handy-App an. Ob er für die
  TV-Geräteart erneut anfällt, ist unbelegt.
- TV-Prüfung nach Googles Liste (2.6): Banner, Bedienung mit der Fernbedienung, Zurück bis zum
  Startbildschirm, Taste Wiedergabe/Pause, kein Bildschirmschoner während der Wiedergabe.
- Store-Eintrag: Datenschutzerklärung, Angaben zur Datensicherheit und Alterseinstufung stehen
  von der Handy-App; dazu kommen Banner und Bildschirmfotos für den Fernseher.
- **Googles Prüfer brauchen einen erreichbaren Server mit Test-Zugang** (Frage F-10). Der
  Vorführserver wird mit der Handy-App zuerst fällig; die TV-Prüfung nutzt denselben. Die Prüfer
  richten den Fernseher von Hand ein (4.4).

### 8.3 Schlüssel

Wie bei der Handy-App (K27 11.2): Google hält den Schlüssel, mit dem die App ausgeliefert wird,
Marco den Upload-Schlüssel. Er liegt nie im Repo.

### 8.4 Versionen und Verträglichkeit

Die Regeln stehen in K27 11.5 und gelten für die TV-App gleich:

- `getHealth` nennt die Vertragsfassung; ist der Server zu alt, zeigt die App «Server zu alt,
  bitte aktualisieren». Ist die App zu alt, weist der Server sie mit einem eigenen Fehler ab.
- Unbekannte Felder und Werte verträgt die App.
- Operationen mit `x-clients` brechen nicht mehr von einem Release aufs nächste; eine brechende
  Änderung braucht eine Übergangszeit. K27 schlägt sie nach Zeit vor (mindestens 90 Tage, und
  mindestens 30 Tage, nachdem eine App-Version im Store ist, die das Neue kann), nicht mehr nach
  Releases wie die Fassungen 1 bis 3 dieses Papiers (Frage F-9).

## 9. Prüfung

| Ebene | Was | Entspricht |
|---|---|---|
| Unit | Zustände der Seiten, Fähigkeitsprofil, Fehlerbilder | Vitest der Web-App |
| Oberfläche | je Seite ein Test mit Tastenereignissen des Steuerkreuzes: jeder Knopf erreichbar, Fokus sichtbar, Zurück führt hinaus, keine Fokusfalle | Regel 8 |
| Vertrag | die App gegen einen echten Server mit Testdaten, wie die e2e-Läufe | `apicheck` |
| Server | Entscheid-Tests für die neuen Fähigkeiten und den Ton. Die Tests für Sitzungsarten, Kopplung und die Erlaubnisliste samt `tv` nach Anhang B baut K27 (dort Abschnitt 12) | wie heute |
| Geräte | Abnahme durch Marco auf Chromecast, Shield und Fernseher, je Stufe eine Liste | Abnahme auf CT 190 |

Die Teststufen aus `CLAUDE.md` gelten weiter. Ob die Oberflächentests in der CI auf einem
Emulator laufen können und wie lange das dauert, misst Stufe 3 von K27.

## 10. Stufen

**Reihenfolge (E-12).** Zuerst entsteht die native Android-Handy-App nach Konzept 27, danach die
TV-App. Die Handy-App legt das gemeinsame Fundament: Serverteil (K27 Stufe 1), Dienst (K27
Stufe 2), Projekt `android/` mit Client, Texten und Farben (K27 Stufe 3). **Die TV-App kann nach
Stufe 3 von K27 beginnen;** für das Einrichten per Scan braucht sie auch dessen Stufen 2 und 9.
Stufe 0 hängt nicht an der Handy-App.

Jede Stufe ist ein eigener Auftrag von Marco. Stufe 1 ist ein Release des Servers, ab Stufe 2a
ist jede Stufe eine Ausgabe der TV-App.

| Stufe | Inhalt | Beleg | Braucht |
|---|---|---|---|
| **0 Messung** | Wegwerf-App mit eigenem Paketnamen, per ADB auf den drei Fernsehern, kein Eingriff ins Produkt. Misst, was nur den Fernseher betrifft: Direktwiedergabe (MKV, HEVC, HDR, Dolby-Vision-Datei auf HDR10-Gerät), Tonformate und Durchreichen je Gerät, Dolby Digital und AAC 5.1 in fMP4-HLS, PGS und ASS, Tempo von «Compose for TV» auf dem schwächsten Gerät. Generator und Verschlüsselung misst Stufe 0 von K27. | Bericht `docs/perf/K26-android-tv.md` mit Entscheid je Punkt | Entwickleroptionen auf den Geräten; Frage 34 im Entscheidungsprotokoll und die Ausnahme in Regel 11 |
| **1 Server: Ton und Fähigkeiten** | Abschnitt 7: drei Ergänzungen am Fähigkeitsprofil, Codec-Angabe, Tonkopie beim Transcode, Mehrkanal beim Neu-Kodieren | Entscheid-Tests; ein Prüfprogramm startet die Wiedergabe mit einem Mehrkanal-Profil; Abnahme auf CT 190 | Ergebnisse aus Stufe 0 (Tonformat, PGS) |
| **2a Gerüst** | Modul `app-tv`: Einrichten per Scan (QR-Code zeigen, Paket holen, einlösen), Einrichten von Hand, Profilwahl mit PIN | App per ADB auf dem Fernseher: von Hand einrichten und Profil wählen; per Scan mit der Handy-App | K27 Stufen 1 bis 3, für den Scan auch 2 und 9; F-6 |
| **2b Seiten** | Start, Bibliothek, Detailseiten, Person, Suche, Listen, Profile verwalten | Steuerkreuz-Test je Seite; Abnahme auf den drei Geräten: blättern ohne Abspielen | freigegebenes Mockup (F-2) |
| **3 Videoplayer** | Player vollständig, Mehrkanalton, HDR, Einstellungen der Wiedergabe | Abnahmeliste je Gerät und Tonanlage; **Vorab-Ausgabe** für Familie und Freunde über die interne Testspur | Stufe 1; K27 Stufe 5 (erstes Hochladen: Paketname und Testspur stehen erst dann); F-8 |
| **4 Ton** | Hörbücher, Podcasts, Webradio | Abnahme auf den Geräten | |
| **5 Wünsche** | Bereich «Wünsche», Benachrichtigungen, Ankündigungen | Abnahme mit echtem Seerr; **erste Ausgabe nach E-5 vollständig** | |
| **6 Öffentlich** | TV-Geräteart mit eigener Veröffentlichungsspur, TV-Prüfung, Banner und Bildschirmfotos | TV-App im Play Store | geschlossener Test und Vorführserver aus K27 Stufe 10 laufen oder sind erledigt; F-10 |

Zur Reihenfolge innerhalb dieses Papiers:

- Stufe 0 kommt vor Stufe 1, weil Stufe 1 das Tonformat und die Untertitel-Angabe baut, über die
  Stufe 0 entscheidet. Der Server liefert Dolby Digital und AAC 5.1 in HLS erst nach Stufe 1;
  Stufe 0 misst darum mit Proben, die von Hand mit ffmpeg und den Schaltern der heutigen Pipeline
  (`server/internal/transcode/pipeline.go`) erzeugt werden.
- Stufe 0 ist die erste Arbeit an einer nativen App für den Fernseher. Frage 34 und die Ausnahme
  in Regel 11 müssen darum vorher stehen, wie bei Konzept 25 die Regeländerung mit Stufe 0 kam.
- Das Mockup entsteht neben Stufe 1 oder 2a.

## 11. Änderungen an früheren Entscheiden und Regeln

Alle erst mit dem Bau und auf Marcos Wort, nicht mit diesem Papier. Die Regeländerungen für das
gemeinsame Fundament schlägt K27 Abschnitt 14 im Wortlaut vor; die Tabelle nennt sie hier, damit
der Zusammenhang sichtbar bleibt.

| Stelle | Heute | Neu |
|---|---|---|
| Spec «Non-Goals» und Übersichtstabelle («zurückgestellt») | TV-Apps (Android TV, Apple TV, webOS, Tizen) und native Apps sind Nicht-Ziel | Android TV ist Ziel (Frage 34). Apple TV, webOS, Tizen, Fire TV und native iOS-Apps bleiben Nicht-Ziele; die native Android-Handy-App wird Ziel und bekommt Konzept 27 (E-10, E-12) |
| Plan Abschnitt 1 («Nicht eingeplant») | TV-Apps, native Apps | Android TV herausgenommen |
| `CLAUDE.md` Regel 11 | «TV-Apps, native Apps» in der Liste der Nicht-Ziele | benannte Ausnahme für beide Android-Apps (Frage 34 und 35, ADR 0039) |
| `CLAUDE.md` Regel 7 | jede Operation trägt `x-visibility` | dazu: jede `account`- und `scoped`-Operation trägt `x-clients` und nennt darin die Bearer-Arten, die sie aufrufen dürfen (K27 4.4) |
| `CLAUDE.md` Regel 8 | Playwright-Tastaturtest je neuer Seite | dazu: je TV-Seite ein Steuerkreuz-Test |
| `CLAUDE.md` Regel 9 | Farben aus Tokens, Texte aus `web/src/locales`, gemeinsame Bauteile | gilt für `android/` mit erzeugten Farben und Texten; der Satz zu `web/src/ui` gilt nur für die Web-App |
| `CLAUDE.md` Werkzeuge | Go, Node, pnpm, `task` | dazu JDK, Android-SDK, Gradle |
| Plan 3.8 Punkt 7 | «Fremd-Clients per Bearer» (nie gebaut) | gebaut für die App-Sitzung des Handys und die Gerätesitzung des Fernsehers (K27 4.1) |
| ADR 0005 | eine Sitzungsart | Nachtrag: drei Sitzungsarten, die Art an den Träger gebunden (ADR 0040) |
| Regel «Vertrag = Tag» | ein Tag je Vertragsfassung | bleibt; daneben neu die Übergangszeit für alle Operationen mit `x-clients`, also den ganzen Nutzerbereich (8.4, K27 11.5) |
| Nicht-Ziel «Cloud» (Spec, `CLAUDE.md` Regel 11) | kein Dienst ausserhalb des eigenen Servers | benannte Ausnahme: der Dienst im Internet für das Einrichten per Scan (E-11) und für Push an die Handy-App (E-17), ADR 0042. Er hält keine Nutzerdaten und sieht keine Inhalte |

## 12. Risiken

| Nr. | Risiko | Folge | Gegenmittel |
|---|---|---|---|
| R-1 | Zweite Oberfläche | jede Zuschauer-Funktion doppelt, dauerhaft | TV-Teil als fester Abschnitt in jedem künftigen Konzept |
| R-2 | Schwache Sticks | ruckelnde Oberfläche | Stufe 0 misst auf dem schwächsten Gerät; Bildgrössen vom Server passend anfordern |
| R-3 | Tonformate je Gerät verschieden | kein Ton oder Stereo trotz Tonanlage | Schalter «Ton» (5.3), Messung je Gerät |
| R-4 | Kopplung als neuer Weg zur Sitzung | Mitschauen und Wünschen durch untergeschobenen Code; ohne PIN am Hauptprofil auch Profile ändern (bekannte Grenze nach E-9) | K-1 bis K-9, Erlaubnisliste (Anhang B), «Meine Geräte», PIN-Empfehlung |
| R-5 | Vertrag bricht eine installierte App | App auf dem Fernseher geht nicht mehr | Verträglichkeitsregel (8.4) |
| R-6 | Google lehnt die App ab oder verlangt mehr | öffentliche Stufe verzögert sich | private Verteilung hängt nicht davon ab |
| R-7 | Identitätsprüfung bei Google dauert | interne Testspur später | bis dahin ADB |
| R-8 | 12 Tester über 14 Tage | öffentliche Stufe braucht Mitmacher | Familie und Freunde aus der privaten Stufe |
| R-9 | Zweite Werkzeugkette (JDK, Android-SDK, Gradle) | längere CI, mehr Pflege | Stufe 3 von K27 misst die Laufzeit; danach Entscheid über einen Pfadfilter |
| R-10 | Texte in 22 Sprachen auch für die TV-App | mehr Übersetzungsarbeit je Funktion | eine Quelle, ein Übersetzungsweg (3.5) |
| R-11 | Bearer im Server ist ein neuer Anmeldeweg für alle Operationen | Fehler dort träfe mehr als die Apps | Bearer nur für App-Sitzung und Gerätesitzung, `x-clients` im Vertrag, Test über alle Operationen (K27 4.4) |
| R-12 | Vermittlungsdienst als dauernde Betriebspflicht (E-11) | Ausfall: kein Einrichten per Scan, für alle Nutzer; Missbrauch; Pflicht zur Datenschutzerklärung | Einrichten von Hand bleibt; Dienst hält nichts dauerhaft; Überwachung (F-20) |
| R-13 | Die TV-App wartet auf die Handy-App (E-12) | deutlich späterer Start der TV-App | Stufe 0 schon vorher; die TV-App kann nach Stufe 3 von K27 beginnen, nicht erst nach der fertigen Handy-App |
| R-14 | Scan nur mit Android-Handy (E-10, E-16) | iPhone-Nutzer tippen am Fernseher | Einrichten von Hand (4.4) |
| R-15 | Gerätetoken laufen nie ab (F-4) | vergessene oder weitergegebene Geräte behalten Zugang | «Meine Geräte», Meldung bei jeder Kopplung |

## 13. Fragen

Bis zur Antwort gilt die Empfehlung. Die Bau-Session legt offene Fragen vor ihrer Stufe vor.
Beantwortet sind F-4, F-7, F-15, F-16 und F-20; F-3 ist abgeleitet und wartet auf Marcos
Bestätigung. F-1, F-9, F-11, F-12, F-13 und F-21 betreffen das
gemeinsame Fundament; sie stehen jetzt in Konzept 27 und werden dort beantwortet. Offen für die
TV-App sind F-2, F-5, F-6, F-8, F-10, F-14, F-17, F-18 und F-19.

| Nr. | Frage | Empfehlung | Fällig vor |
|---|---|---|---|
| F-1 | App im selben Repo (`android/`) oder in einem eigenen? | steht in K27 (dort Empfehlung: selbes Repo) | K27 |
| F-2 | Klickbares Mockup der TV-Oberfläche vor dem Bau? | ja | Stufe 2b |
| F-3 | Soll es neben Scan und Passwort die Bestätigung im Browser per Code geben? | **Abgeleitet, nicht entschieden:** nein. Marco hatte bei E-4 «Code und Passwort» gewählt, danach mit E-10 den Scan mit der Handy-App beschrieben und bei E-16 «Nur die Handy-App» gewählt. E-16 fragte, wer per QR-Scan verbinden darf, nicht ob es daneben einen Code im Browser gibt | Marcos Freigabe |
| F-4 | Wie lange bleibt ein gekoppelter Fernseher ohne Benutzung angemeldet? | **Beantwortet: «Unbegrenzt»** (Empfehlung war 400 Tage) | – |
| F-5 | Serveradresse: zusätzlich zu Scan und Eingabe von Hand eine Suche im Heimnetz? | nein | Stufe 2a |
| F-6 | Unverschlüsseltes HTTP zum Server auch am Fernseher? Für die Handy-App gilt «HTTP überall mit Warnung» (E-18). | wie am Handy, damit derselbe Server an beiden Geräten geht | Stufe 2a |
| F-7 | Paketname der App | **Beantwortet in K27: «Eine App: com.slouchcat.app»** (E-20) | – |
| F-8 | Dolby Vision Profil 5 und 7, getrennte Meldung für HDR10, HLG und Dolby Vision, Umschalten der Bildfrequenz in der ersten Ausgabe? | nein, spätere Stufe; ausser Stufe 0 zeigt ein Problem mit dem einen HDR-Schalter | Stufe 3 |
| F-9 | Übergangszeit für brechende Vertragsänderungen | steht in K27 (dort Empfehlung nach Zeit: mindestens 90 Tage) | K27 |
| F-10 | Vorführserver für Googles Prüfer: wo und mit welchem Inhalt? | kleiner Server im Internet mit frei lizenzierten Testfilmen; wird mit der Handy-App zuerst fällig | K27 Stufe 10 |
| F-11 | Hochladen in den Play Store von Hand oder aus der CI? | steht in K27 | K27 |
| F-12 | FFmpeg in der App, um Tonformate selbst zu dekodieren? | steht in K27 (dort Empfehlung: nein) | K27 |
| F-13 | Kleinste Android-Version (`minSdk`) | steht in K27; für den Fernseher nach Stufe 0 aus den drei Geräten ableiten | Stufe 2a |
| F-14 | Reihe «Weiterschauen» auf dem Startbildschirm von Google TV? Sie zeigt Titel ausserhalb der App, für alle im Raum sichtbar. | nicht in der ersten Ausgabe | nach Stufe 5 |
| F-15 | «Meine Geräte» für jedes Konto (sehen und abmelden), nicht nur in der Verwaltung? | **Beantwortet: «Ja»** | – |
| F-16 | Darf eine Gerätesitzung Wünsche anlegen? | **Beantwortet: «Ja»** | – |
| F-17 | Fire TV | eigenes Konzept nach Stufe 6. Neue Fire-TV-Geräte mit Vega OS führen keine Android-Apps aus | – |
| F-18 | Meldekanäle (E-Mail, Telegram), Sehbericht und Kontodaten auch in der TV-App? | nein, Browser; die Meldekanäle bleiben einer Gerätesitzung verwehrt, damit die Meldung aus K-7 nicht umgeleitet werden kann | K27 Stufe 1 (dort wird die Erlaubnisliste gebaut) |
| F-19 | PIN-Empfehlung aus E-9 auch in der Web-App und der Handy-App zeigen? | ja, gleicher Hinweis an gleicher Stelle | K27 Stufe 4 |
| F-20 | Vermittlungsdienst: Wo läuft er? | **Beantwortet in K27** (E-19): vorläufig auf eigener Hardware, später auf einem Mietserver | – |
| F-21 | Adresse des Vermittlungsdienstes in den Apps änderbar? | steht in K27 (dort Empfehlung: ja, nur für die Vermittlung) | K27 |

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

**Nachtrag nach den Lesungen:** Marcos Entscheide E-8 und E-9 erlauben einer Gerätesitzung die
Profilpflege unter der Bedingung des Browsers. Das nimmt die Sperre aus Befund 1 der ersten
Runde für die Profilpflege bewusst zurück; die Folge steht als bekannte Grenze in Abschnitt 4.5.
Für Meldekanäle, Sehbericht, Kontodaten, Offline, Kopplung und «Meine Geräte» bleibt die Sperre.
Dieser Nachtrag ist nicht vom Kritiker gelesen.

**Zweiter Nachtrag (E-10 bis E-12):** Abschnitt 4 ist neu geschrieben: Einrichten per Scan mit
der Handy-App über einen Vermittlungsdienst, Gutschein statt Benutzercode, keine Seite `/link`.
Damit sind aus der zweiten Lesung Befund 1 (Heimnetz-Erkennung) und Befund 4 (öffentliches
Beginnen schreibt eine Gerätezeile) gegenstandslos; Befund 2, 3 und 8 gelten weiter und stehen in
4.2, 4.4 und 4.5. **Neu und ungeprüft** sind der Vermittlungsdienst, die Verschlüsselung des
Pakets, der Gutschein mit Schlüsselnachweis und die Abhängigkeit von Konzept 27. Vor der Freigabe
braucht das eine neue Lesung durch den Kritiker.

**Dritter Nachtrag (Fassung 4): Lesung von Abschnitt 4 zusammen mit Konzept 27.** Der Kritiker
von Konzept 27 hat Abschnitt 4 dieses Papiers (Stand `b3296ee6`) erstmals gelesen: **2 Major,
7 Minor** (K27 18.2). Fassung 4 arbeitet sie ein und gleicht das Papier an K27 an (K27 18.1,
20 Punkte).

| Nr. | Stufe | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Major | Wer die Treffpunkt-Kennung kennt, konnte dem Fernseher ein eigenes Paket unterschieben oder den Treffpunkt verbrauchen; der Schlüsselnachweis war nicht beschrieben | 4.3 (Abholgeheimnis, verschlüsseltes Token), K-1, K-3, K-9, bekannte Grenze in 4.6 |
| 2 | Major | Die Erlaubnisliste zählte die `scoped`-Operationen nicht auf | 4.5, Anhang B (alle 116 Operationen) |
| 3 | Minor | Abmelden liess die Passwortänderung aus; die Aufräumung war falsch beschrieben | 4.2 |
| 4 | Minor | «Cookie und Bearer» meinte das Sitzungs-Cookie; HTTP mit Token ohne Ablauf fehlte als Grenze | 4.2, 4.6 |
| 5 | Minor | «Radio-Start» ist eine JSON-Operation, die Streaming-Routen des Radios sind andere | 4.2 |
| 6 | Minor | Der Zähler für Fehlversuche ist ein Topf für mehrere Wege und zählt je Adresse | 4.4 |
| 7 | Minor | Die Meldung nennt einen Gerätenamen, den der Angreifer wählt | K-7 |
| 8 | Minor | «In beiden Apps änderbar» vertrug sich nicht damit, dass die Handy-App einen fremden Dienst ablehnt | 4.7 |
| 9 | Minor | Das Format des QR-Codes war nicht beschrieben | 4.3 |

Die Einarbeitung und Anhang B hat kein Kritiker gegengelesen. In Anhang B sind die Gruppen
«Empfehlen», «Benachrichtigungen», «Ankündigungen» und «Vorlieben der Wiedergabe» Vorschlag der
Redaktion; der Kritiker hatte zwei davon ausdrücklich als offen genannt.

**Vierter Nachtrag (Fassung 5): dritte Lesung dieses Papiers.** Derselbe Kritiker wie in Runde 1
und 2 hat Fassung 4 gegen den Vertrag und gegen Konzept 27 gelesen: **REVISE, 0 Blocker, 1 Major,
8 Minor.** Bestätigt hat er: Anhang B nennt alle 24 `account`- und 92 `scoped`-Operationen genau
einmal und mit der richtigen Sichtbarkeit (93 ja, 23 nein), die Streaming-Routen stimmen mit
`binaryRoutes` überein, und alle neun Befunde aus K27 18.2 sind in 4.2 bis 4.7 behoben.

| Nr. | Stufe | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Major | Niemand trug `tv` an den 93 Operationen ein, bevor es die Gerätesitzung gibt: K27 Stufe 1 schafft sie und testet sie, dieses Papier hatte die Angabe aber bei der TV-App | Abschnitt 7, F-18; **muss in K27 Stufe 1** |
| 2 | Minor | Das Einlösen trug keinen Gerätenamen | 4.3 Schritt 6; **muss in K27 7.2** |
| 3 | Minor | Der Fehler des Dienstes bei einem schon liegenden Paket wird nicht als Warnung genutzt | 4.6; **muss in K27 Abschnitt 10** |
| 4 | Minor | Anhang B.3 führte öffentliche Operationen unter `x-clients`; «Server weist sich aus» fehlte | Anhang B.3 |
| 5 | Minor | Stufe 3: Paketname und Testspur stehen erst nach K27 Stufe 5 | Abschnitt 10 |
| 6 | Minor | Reste des alten Entwurfs: Texteingabe, Liste des Mockups, R-9, Übergangszeit in Abschnitt 11, Fälligkeit von F-19 | 3.4, Abschnitte 11 bis 13 |
| 7 | Minor | Die Profilseite am Fernseher lässt Sehbericht und Offline-PIN weg, ohne es zu sagen | 3.3 |
| 8 | Minor | Zuschreibung: fester Name und änderbare Adresse des Dienstes sind in K27 offene Fragen; F-3 «beantwortet durch E-16» war ein Schluss, kein Entscheid | 4.7, F-3 |

**Was Konzept 27 davon übernehmen muss** (dessen Zweig wird hier nicht angefasst): die Angabe
`tv` nach Anhang B und Frage F-18 in seiner Stufe 1; Gerätename und Systemfassung im Einlösen
(K27 7.2); die Warnung der Handy-App, wenn am Treffpunkt schon ein Paket liegt (K27 Abschnitt 10);
F-19 in seiner Stufe 4. Ausserdem nennt K27 dieses Papier noch mit dem Stand `b3296ee6`; sein
Abschnitt 18 ist mit Fassung 4 und 5 erledigt.

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

## Anhang B. Erlaubnisliste des Fernsehers je Operation

Stand: Vertrag 1.41.0 am Anker, 24 Operationen mit `account` und 92 mit `scoped`. «ja» heisst:
Die Operation bekommt `tv` in `x-clients` (K27 4.4). Der Fernseher darf danach 93 Operationen
aufrufen und 23 nicht. Wo «Vorschlag» steht, hat Marco nicht entschieden.

### B.1 Kontoebene (`account`)

| Gruppe | Operationen | Fernseher | Grund |
|---|---|---|---|
| Sitzung und Profilwahl | `logout`, `getSession`, `listMyProfiles`, `selectProfile` | ja | nötig |
| Profile verwalten | `listProfiles`, `createProfile`, `updateProfile`, `deleteProfile`, `setProfilePin`, `removeProfilePin`, `listAccountLibraries` | ja | E-8, E-9; nur mit aktivem Hauptprofil |
| Offline-PIN | `setOfflinePin`, `removeOfflinePin` | nein | E-8 |
| Offline-Ereignisse | `uploadOfflineEvents` | nein | E-8 |
| Kontodaten | `updateAccount` | nein | Vorschlag (F-18) |
| Sehbericht | `getWatchReport` | nein | Vorschlag (F-18) |
| Meldekanäle | `getMyChannels`, `updateMyChannel`, `testMyChannel`, `createTelegramLink`, `deleteTelegramLink`, `getPushPublicKey`, `createPushSubscription`, `deletePushSubscription` | nein | Vorschlag (F-18); schützt die Meldung aus K-7 vor dem Umleiten |

### B.2 Mit Profil (`scoped`)

| Gruppe | Operationen | Fernseher | Grund |
|---|---|---|---|
| Stöbern | `listVisibleLibraries`, `listMyMedia`, `listLibraryItems`, `listAudioGroups`, `getAudioSeries`, `getHome`, `listHomeRow`, `listPicks`, `getItem`, `getItemTrailers`, `getPerson`, `listPodcastEpisodes` | ja | E-5 |
| Bilder und Vorschau | `getItemImage`, `getChapterImage`, `getTrickplay`, `getTrickplaySheet`, `getPersonImage`, `getCollectionImage` | ja | E-5 |
| Abspielen | `startPlayback`, `getItemTracks`, `getItemStream`, `reportProgress`, `stopPlayback`, `getPlaybackSessionState`, `getPlaybackMaster`, `getPlaybackMedia`, `getPlaybackInit`, `getPlaybackSegment`, `getPlaybackSubtitle` | ja | E-5 |
| Suchen und Sammlungen | `search`, `listGenres`, `listVisibleCollections`, `getCollection` | ja | E-5 |
| Merkliste und «Gesehen» | `listWatchlist`, `addToWatchlist`, `removeFromWatchlist`, `listWatched`, `setItemWatched`, `setCollectionWatched` | ja | E-5 |
| Empfehlen | `recommendItem`, `unrecommendItem` | ja | Vorschlag: Nutzerbereich (E-7). Ein fremdes Gerät könnte allen Profilen Titel empfehlen; das ist sichtbar und rücknehmbar |
| Benachrichtigungen | `listNotifications`, `markNotificationRead`, `markAllNotificationsRead` | ja | Vorschlag. Ein fremdes Gerät könnte Meldungen in der App als gelesen markieren; die Meldung aus K-7 kommt darum ausserhalb der App |
| Ankündigungen | `listMyAnnouncements`, `ackAnnouncement` | ja | Vorschlag |
| Vorlieben der Wiedergabe | `getMyPlaybackPrefs`, `putMyPlaybackPrefs`, `resetMySeriesTracks` | ja | Vorschlag; Seite «Einstellungen der Wiedergabe» |
| Wünsche | `getWishDiscover`, `listWishDiscoverRow`, `listWishTitles`, `listWishGenres`, `getWishGenreBackdrop`, `searchWishTitles`, `searchExternalTitles`, `getExternalTitle`, `getExternalTitleImage`, `getWishSeasons`, `listWishes`, `createWish`, `listAccountWishes`, `getWishQuota`, `withdrawWish` | ja | E-5, Antwort auf F-16 |
| Webradio | `listRadioStations`, `getRadioFacets`, `getRadioStation`, `listRadioFavorites`, `setRadioFavoriteOrder`, `addRadioFavorite`, `removeRadioFavorite`, `startRadioListen`, `radioListenHeartbeat`, `endRadioListen`, `getRadioNow`, `getRadioPreferences`, `setRadioPreferences`, `getRadioLivePlaylist`, `getRadioLiveInit`, `getRadioLiveSegment`, `getRadioImage`, `getRadioCover` | ja | E-5 |
| Downloads und Offline | `getDownloadOptions`, `createDownloads`, `deleteDownload`, `getDownloadManifest`, `getDownloadFile`, `getDownloadSubtitle`, `getDownloadTrickplaySheet`, `syncOffline`, `getDownloadRules`, `setDownloadRules` | nein | E-8 |

### B.3 Neue Operationen aus Konzept 27

| Operation | Sichtbarkeit | Fernseher | Grund |
|---|---|---|---|
| «App anmelden» mit Art `tv` | `public` | ruft sie auf | Einrichten von Hand (4.4) |
| Gutschein einlösen | `public` | ruft sie auf | Einrichten per Scan (4.3) |
| «Server weist sich aus» | `public` | ruft sie auf | nur falls K27 F-38 mit Ja beantwortet wird (4.6) |
| Kopplungsgutschein ausstellen | `account` | nein | E-16: nur die App-Sitzung des Handys |
| Meine Geräte: Liste und Abmelden | `account` | nein | 4.5 |

Öffentliche Operationen tragen kein `x-clients`; die Angabe gibt es nur für `account` und `scoped`
(K27 4.4).

Jede Operation, die später dazukommt, muss die Frage neu beantworten; der Schema-Test verlangt
die Angabe (K27 4.4).
