# Konzept: Android-TV-App – native App für den Fernseher mit eigenem Player, Kopplung per Code und Mehrkanalton

- **Status: Entwurf, Fassung 1 (19. November 2026). Nicht freigegeben, kein Kritiker-Review.**
  Gebaut wird erst nach Marcos Freigabe und auf seinen eigenen Auftrag.
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

> **Dieses Papier ändert keine Zeile Code.** Was als «Entscheid von Marco» bezeichnet ist, steht in
> Abschnitt 1 (E-1 bis E-6). Alles andere ist Vorschlag der Redaktion. Aussagen über den Code
> stammen vom Anker, Aussagen über Android und Google Play aus einer Recherche vom
> 19. November 2026 (Anhang A). Was dort nicht belegt werden konnte, heisst im Text «unbelegt» und
> wird in Stufe 0 auf echten Geräten gemessen.

## 1. Was Marco will

Marco hat am 19. November 2026 sechs Weichen über Rückfragen gestellt. Die Tabelle nennt die
gewählte Antwort wörtlich.

| Nr. | Frage | Marcos Wahl | Empfehlung war |
|---|---|---|---|
| E-1 | Auf welchen Geräten soll die App laufen und getestet werden? | «Chromecast / Google TV Streamer», «Nvidia Shield TV», «Fernseher mit Google TV». Fire TV nicht gewählt. | keine |
| E-2 | Wer soll die App bekommen? | «Erst privat, später öffentlich» | keine |
| E-3 | Wie soll die App gebaut sein? | **«Ganz nativ»** | Mischform (Web-Oberfläche, nativer Player) |
| E-4 | Wie meldet man sich am Fernseher an? | «Code und Passwort» | Code und Passwort |
| E-5 | Was kann die erste Ausgabe ausser Filmen und Serien? | «Hörbücher und Podcasts», «Webradio», «Wünsche über Seerr». Die Verwaltung bleibt im Browser (in der Frage so vorgegeben). | keine |
| E-6 | Mehrkanalton unverändert an eine Tonanlage weitergeben? | «Ja, von Anfang an» | Ja, von Anfang an |

Dazu gehört der Stand der Konten vom selben Tag: Für das Projekt gibt es ein eigenes Google-Konto
und darauf ein Entwicklerkonto in der Google Play Console (privates Konto). Google prüft die
Identität noch; bis dahin lässt sich dort keine App anlegen. Ein Entwicklerkonto bei Amazon gibt es
nicht.

## 2. Heute

### 2.1 Regeln

TV-Apps und native Apps sind heute ein Nicht-Ziel:

- Spec, Abschnitt «Non-Goals»: «TV-Apps (Android TV, Apple TV, webOS, Tizen) und native iOS-/Android-Apps
  (spätere Versionen).» (`docs/spec/01-anforderungen-v1.md`)
- `CLAUDE.md`, Regel 11, zählt «TV-Apps, native Apps» unter den Nicht-Zielen auf.
- Plan 2.1 führt TV-Apps und native Apps ebenfalls als Nicht-Ziel.

Die Spec hat den Weg aber offen gehalten. Kriterium C-5 verlangt einen Vertrag, der «so vollständig
[ist], dass ein Fremd-Client alle Zuschauerfunktionen umsetzen kann», und Plan P-16 prüft das mit
einem Fremd-Client, der nur `openapi.yaml` kennt.

### 2.2 Anmeldung und Sitzung

- Anmelden: `POST /auth/login` mit E-Mail und Passwort. Die Sitzung ist ein Cookie
  (`slouchcat_session`, `HttpOnly`, `SameSite=Lax`, `Secure` nur über TLS), 30 Tage gleitend
  (`SessionLifetime`, `server/internal/auth/auth.go`).
- Ein zweites Cookie `slouchcat_device` (400 Tage) ist die Gerätekennung für PIN-Sperre und
  Geräteliste.
- Nach dem Anmelden hat die Sitzung noch kein Profil (ADR 0005). Profilwahl:
  `GET /auth/profiles`, `POST /auth/profiles/{id}/select` mit PIN. Die PIN hat 4 bis 8 Ziffern, mit
  Sperre je Profil und Gerät.
- Jede schreibende Anfrage braucht den Kopf `X-Requested-With: Slouchcat`, sonst 403
  `csrf_header_missing`. Eine Prüfung von `Origin` oder `Referer` gibt es nicht.
- Bearer-Token sind im Vertrag als Schema `bearerToken` angelegt, aber keine Operation nutzt sie und
  der Server liest den Kopf `Authorization` nicht. ADR 0005 und Plan 5.3 Punkt 7 sehen sie für
  Fremd-Clients «später» vor.
- Eine Kopplung per Code oder QR gibt es nicht.
- Die Verwaltung führt eine Geräteliste (`listDevices`) mit Name, Typ, letztem Kontakt und
  Fähigkeitsprofil. `deleteDevice` entzieht alle Sitzungen eines Geräts.

Ein Client ohne Browser kann sich also schon heute anmelden: Der Vertragstest
(`server/cmd/apicheck/client.go`) tut genau das mit einem Cookie-Speicher und dem CSRF-Kopf.

### 2.3 Wiedergabe

- `POST /items/{id}/play` (`startPlayback`) liefert `PlaybackInfo`: `mode` ist `direct` (Datei mit
  Range) oder `hls` (fMP4, als `remux` oder `transcode`).
- **Der Server entscheidet** (`server/internal/playback/decide.go`). Der Client meldet nur, was er
  kann: `PlaybackCapabilities` mit `containers`, `video`, `audio`, `hls` (`native`, `mse`, `none`),
  `maxWidth`, `maxHeight`, `hdr`, `name`, `type`. Der Typ `tv` ist im Vertrag schon vorgesehen.
- Medien-Adressen tragen ein Stream-Token `st` und brauchen kein Cookie. Plan 5.4 hat das Token
  ausdrücklich für native Player entworfen. Bilder und Trickplay-Bögen brauchen das Cookie.
- **Ton:** Der Server kopiert die Tonspur, wenn das Gerät den Codec meldet
  (`audioOK` in `decide.go`), unabhängig von der Kanalzahl. Muss er den Ton neu kodieren, entsteht
  neben einem Videobild immer Stereo-AAC (`outputChannels`, `server/internal/transcode/pipeline.go`).
  Eine Kanalzahl kann der Client nicht melden.
- **Tonspur wechseln:** Eine andere als die Standardspur führt heute zu einem Remux
  (Grund `audio_track`), weil ein Browser in einer Datei keine Spur wählen kann.
- **Untertitel:** Textspuren liefert der Server als WebVTT, die ASS-Gestaltung fällt weg. Bildspuren
  (PGS, DVD) werden eingebrannt, das heisst immer Umrechnen.
- **HDR:** geht nur mit `hdr: true` unverändert durch. Dolby Vision Profil 5 und 7 nie.
- **`hls: native` meint heute Safari.** `directVideo` und `hdrRemux` in `decide.go` behandeln
  einen Client mit `native` gesondert (HEVC nur über einen Remux mit `hvc1`-Kennung). Eine
  Android-App darf diesen Wert darum nicht einfach übernehmen (Abschnitt 5.2).
- Fortschritt: `POST /items/{id}/progress` alle 10 Sekunden, bei Pause und am Ende. Der Player
  fragt alle 4 Sekunden `/playback/sessions/{id}/state` ab und beendet mit
  `DELETE /playback/sessions/{id}`.
- `PlaybackInfo` trägt Kapitel, Marken für Vorspann und Abspann (`segments`), nächste und vorige
  Folge und den Verweis auf Trickplay.

### 2.4 Fernbedienung in der Web-App

Die Web-App ist mit Pfeiltasten bedienbar (`web/src/ui/spatial.tsx`, Bibliothek
`@noriginmedia/norigin-spatial-navigation`), Escape und Backspace gelten als Zurück-Taste, und rund
100 Playwright-Dateien prüfen das (Regel 8, C-4). Eine Erkennung «das ist ein Fernseher» gibt es
nicht; `deviceType()` meldet nie `tv`.

Für dieses Konzept ist das nur Vorbild: Mit E-3 baut die TV-App ihre Oberfläche neu. Was die
Web-App an Fokusregeln gelernt hat (Zurück-Verhalten, Fokus nach dem Schliessen eines Menüs,
Reihen mit Erinnerung an die letzte Kachel), gilt als Vorgabe für die TV-Seiten.

### 2.5 Vertrag und Versionen

- Vertrag 1.41.0 mit rund 307 Operationen: 179 `admin`, 92 `scoped`, 24 `account`, 8 `public`,
  4 `owner`. Die TV-App braucht nur `public`, `account` und `scoped`, also 124 Operationen.
- Regel «Vertrag = Tag»: Jede Erhöhung von `info.version` bekommt ein Release.
- **Es gibt keine Verträglichkeitsregel zwischen Client und Server.** Die Web-App reist im selben
  Container wie der Server, darum darf ein Release den Vertrag brechen (zuletzt 1.40.0). Der einzige
  Versionshinweis für einen Client ist `GET /healthz` mit `version`.

### 2.6 Vorgaben von Google (Recherche, Anhang A)

- **Privates Entwicklerkonto:** Vor der öffentlichen Freigabe verlangt Google einen geschlossenen
  Test mit mindestens 12 Testern, die 14 Tage am Stück angemeldet bleiben. Ob das für reine
  TV-Apps gilt, ist unbelegt; das Papier rechnet damit.
- **Interne Testspur:** ohne Vorbedingung, bis 100 Tester je App über eine Liste von E-Mail-Adressen,
  kein Ablaufdatum genannt. Ob die Installation aus dieser Spur auf einem Fernseher klappt, ist
  unbelegt.
- **TV-Prüfung:** Eintrag für den TV-Startbildschirm (`LEANBACK_LAUNCHER`), Banner 320 × 180 mit
  App-Namen, alles mit der Fernbedienung bedienbar, Zurück führt bis zum Startbildschirm, Taste
  Wiedergabe/Pause wirkt, kein Bildschirmschoner während der Wiedergabe, **Test-Zugangsdaten für
  Googles Prüfer**.
- **Technik:** App-Bundle (AAB) ist Pflicht; Ziel-API mindestens 34; `minSdk` höchstens 31;
  seit 1. August 2026 müssen TV-Apps 32 und 64 Bit sowie 16-KB-Speicherseiten unterstützen.
- **Bausteine:** «Compose for TV» (`androidx.tv:tv-material` 1.1.0) ist stabil, das ältere Leanback
  ist abgekündigt. Der Player-Baukasten Media3/ExoPlayer steht bei 1.11.1.

## 3. Die App

### 3.1 Bauweise (E-3)

Eine eigene Android-App in Kotlin:

| Teil | Baustein | Grund |
|---|---|---|
| Oberfläche | Jetpack Compose mit «Compose for TV» | von Google empfohlen, Leanback ist abgekündigt |
| Video und Ton | AndroidX Media3 (ExoPlayer) | spielt MKV, MP4, TS, HLS; nutzt die Decoder des Geräts |
| Netz | OkHttp mit Cookie-Speicher | dieselbe Sitzung wie der Browser, kein neuer Anmeldeweg im Server |
| API | aus `api/openapi.yaml` erzeugter Kotlin-Client | Regel 1: der Vertrag bleibt die einzige Quelle |
| Bilder | Coil über denselben OkHttp-Client | Bilder brauchen das Sitzungs-Cookie |

Die App nutzt die bestehende Cookie-Sitzung samt CSRF-Kopf. Bearer-Token bleiben, wie in ADR 0005
vorgesehen, eine Sache für Fremd-Clients und werden hier nicht gebaut. Das Cookie liegt
verschlüsselt im App-Speicher.

**Was «ganz nativ» kostet,** damit es beim Freigeben klar ist: Jede Zuschauerseite gibt es danach
zweimal. Jedes künftige Konzept, das eine Zuschauerseite ändert, braucht einen TV-Teil mit eigenem
Bau, eigenen Tests und eigener Abnahme. Regel 9 («Admin und Zuschauer nutzen dieselben Komponenten
aus `web/src/ui`») kann für die TV-App nicht gelten; an ihre Stelle treten eigene Bauteile mit
denselben Tokens und Texten (Abschnitt 3.5).

### 3.2 Platz im Repo

Vorschlag (Frage F-1): ein Ordner `android/` im bestehenden Repo.

- Der Vertrag, die Texte und die Design-Tokens liegen schon hier; die App erzeugt daraus ihren Code
  mit `task gen`. In einem zweiten Repo müssten diese drei Quellen bei jeder Änderung hinüberkopiert
  werden.
- Ein Pull Request kann Server und App gemeinsam ändern und wird gemeinsam geprüft.
- Neue erzeugte Verzeichnisse (Regel 1): der Kotlin-Client, die Farb- und Massangaben aus den
  Tokens, die Textdateien `strings.xml`.
- Werkzeuge: JDK und Android-SDK ohne sudo unter `~/.local`, Gradle über den Wrapper im Repo,
  Versionen gepinnt wie bei Go und Node. `task android:check` und `task android:test` als
  Einstieg; `task ci` ruft beide auf.
- CI: ein eigener Job `android`, der nur läuft, wenn `android/`, der Vertrag, die Tokens oder die
  Texte berührt sind. Er hängt am Gate `ci`.

Welcher Generator den Kotlin-Client erzeugt, entscheidet Stufe 0 nach einem Versuch mit dem echten
Vertrag (er ist mit rund 15'000 Zeilen gross und nutzt `oneOf` und `nullable`).

### 3.3 Seiten

| TV-Seite | Vorbild in der Web-App | Stufe |
|---|---|---|
| Serveradresse | neu | 2 |
| Anmelden (Code, QR, E-Mail und Passwort) | `/login` | 2 |
| Profilwahl mit PIN | `/profiles` | 2 |
| Start (Spotlight, Reihen) | `/` | 2 |
| Bibliothek (Raster, Filter, Sortierung) | `/libraries/$libraryId` | 2 |
| Film, Serie, Staffel, Folge | `/items/$itemId` | 2 |
| Person | `/people/$id` | 2 |
| Suche | `/search` | 2 |
| Merkliste, Gesehen, Sammlungen, Listen, Empfehlungen | `/watchlist`, `/watched`, `/collections`, `/lists/$key`, `/picks` | 2 |
| Einstellungen der Wiedergabe | `/settings/playback` | 3 |
| Videoplayer | `/play/$itemId` | 3 |
| Hörbuch, Hörbuch-Reihe, Podcast, Tonplayer | `/libraries/$id/series/$sid`, Tonplayer | 4 |
| Webradio | `/radio` | 4 |
| Wünsche, Entdecken, Titel ausserhalb der Bibliothek | `/wishes`, `/discover…`, `/titles/$media/$tmdbId` | 5 |
| Benachrichtigungen (nur die Liste) | `/notifications` | 5 |

**Nicht in der App:** die Verwaltung (`/admin/*`), Ersteinrichtung und Einladungen, Profile anlegen
und ändern, Downloads und Offline, Web-Push. Wer das braucht, nimmt den Browser.

### 3.4 Bedienung und Gestaltung

- Alles mit Steuerkreuz, OK und Zurück. Zurück schliesst zuerst Menüs, dann die Seite, auf der
  Startseite die App (Vorgabe von Google).
- Der Fokus ist immer sichtbar. Jede Reihe merkt sich ihre letzte Kachel.
- Texteingabe nur an drei Stellen: Serveradresse, Anmeldung mit Passwort, Suche. Die Suche bekommt
  zusätzlich die Spracheingabe des Geräts, wo sie vorhanden ist.
- Marcos Gestaltungsregeln vom 16. November 2026 gelten auch hier: Klappbares ist zu, Kontrast
  hoch (Rahmen mindestens 3:1, gedämpfter Text mindestens 8:1).
- Schrift und Abstände für Sofa-Abstand: eigene Tokens für den Fernseher, keine Zahl im Code.

**Mockup vor dem Bau (Frage F-2):** Vorschlag ist ein klickbares Mockup in 1920 × 1080, mit den
Pfeiltasten bedienbar, für Anmeldung, Profilwahl, Start, Bibliothek, Detailseite und
Player-Bedienung. Marco gibt es frei, bevor Stufe 2 beginnt.

### 3.5 Texte und Farben (Regel 9)

- **Texte:** Quelle bleiben `web/src/locales/*.json` mit ihren 23 Sprachen. Ein Generator schreibt
  daraus die Android-Dateien `strings.xml`. Texte, die nur der Fernseher braucht, kommen unter einem
  eigenen Namensraum in dieselben Dateien, damit der Übersetzungsweg aus ADR 0033 einer bleibt.
  Zu prüfen in Stufe 2: Mehrzahlformen und Platzhalter der heutigen Texte müssen sich verlustfrei
  übertragen lassen.
- **Farben und Masse:** `packages/design-tokens` baut schon mit Style Dictionary. Ein weiteres
  Ausgabeformat erzeugt Kotlin-Konstanten für Compose.
- **Wächter:** Android Lint meldet festen Text als Fehler; eine Lint-Regel verbietet Farbwerte
  ausserhalb des erzeugten Codes.
- **Regel 12 gilt unverändert:** Die App entscheidet nichts über Sichtbarkeit oder Alter. Sie zeigt,
  was der Server liefert; nicht sichtbar heisst 404.

## 4. Anmeldung (E-4)

### 4.1 Serveradresse

Beim ersten Start fragt die App nach der Adresse des Servers, prüft sie mit `GET /healthz` und
zeigt Name und Version. Die Adresse bleibt gespeichert; wechseln geht in den Einstellungen.

- Eine automatische Suche im Heimnetz ist nicht Teil der ersten Ausgabe (Frage F-5): Der Server
  läuft in Docker, dort kommen die üblichen Suchverfahren ohne Zusatzeinrichtung nicht aus dem
  Container heraus.
- Android blockiert unverschlüsseltes HTTP ab Werk. Viele Heimserver laufen ohne TLS im LAN. Die
  App muss HTTP darum ausdrücklich erlauben (Frage F-6).

### 4.2 Kopplung per Code

Ablauf, angelehnt an RFC 8628 («Device Authorization Grant»):

1. Der Fernseher bittet den Server um eine Kopplung und nennt seinen Gerätenamen.
2. Der Server antwortet mit einem **Gerätecode** (lang, geheim, bleibt im Fernseher) und einem
   **Benutzercode** (kurz, steht am Bildschirm), gültig 10 Minuten.
3. Der Fernseher zeigt den Benutzercode gross an, dazu einen QR-Code auf die neue Web-Seite
   `/link` des Servers.
4. Marco öffnet die Seite am Handy. Ist er dort nicht angemeldet, meldet er sich zuerst an. Die
   Seite zeigt Gerätename, Zeitpunkt und Code und fragt: «Diesen Fernseher anmelden?»
5. Der Fernseher fragt alle 5 Sekunden nach. Nach der Bestätigung bekommt er eine Sitzung ohne
   Profil und zeigt die Profilwahl.

Neue Operationen (Namen legt der Bau fest):

| Operation | `x-visibility` | Zweck |
|---|---|---|
| Kopplung beginnen | `public` | Gerätecode und Benutzercode ausgeben |
| Kopplung abholen | `public` | mit dem Gerätecode nachfragen; setzt bei Erfolg das Sitzungs-Cookie |
| Kopplung ansehen | `account` | Gerätename und Alter zum Benutzercode zeigen |
| Kopplung bestätigen oder ablehnen | `account` | die Sitzung freigeben |

Die Web-Seite `/link` ist eine neue Seite und bekommt ihren Tastaturtest (Regel 8).

### 4.3 E-Mail und Passwort

Der zweite Weg nutzt `POST /auth/login` wie der Browser, ohne Änderung am Server. Googles Prüfer
brauchen ihn später (Abschnitt 8.2).

### 4.4 Profilwahl und PIN

Wie im Browser: Liste der Profile, PIN über ein Ziffernfeld für die Fernbedienung. Sperren und
Fehlermeldungen kommen vom Server (`pin_required`, `pin_invalid`, `429 pin_locked`). Profil wechseln
geht über das Menü der Startseite.

### 4.5 Gerätesitzung

Vorschlag: Eine Sitzung, die über die Kopplung oder über die Anmeldung in der TV-App entsteht,
ist eine **Gerätesitzung**:

- Der Server verweigert ihr alle Operationen mit `admin` und `owner` und die Kontopflege (Passwort
  ändern, Einladungen). Die TV-App braucht nichts davon (E-5).
- Sie erscheint in der Geräteliste der Verwaltung mit Typ `tv` und dem gemeldeten Namen.
  «Gerät vergessen» (`deleteDevice`) meldet den Fernseher ab.
- Wie lange sie ohne Benutzung gültig bleibt, ist Frage F-4.

### 4.6 Sicherheit der Kopplung

Die Kopplung ist ein neuer Weg zu einer Sitzung und damit der heikelste Teil dieses Konzepts.

| Nr. | Regel | Schützt gegen |
|---|---|---|
| K-1 | Benutzercode: 8 Zeichen aus 20 Buchstaben ohne Vokale und ohne verwechselbare Zeichen (Muster aus RFC 8628), Anzeige `XXXX-XXXX` | Raten, Tippfehler, anstössige Wörter |
| K-2 | Gerätecode: 256 Bit Zufall, im Server nur gehasht gespeichert (Plan 5.3 Punkt 2) | Auslesen der Datenbank |
| K-3 | Gültig 10 Minuten, einmal verwendbar, nach der Bestätigung sofort verbraucht | Wiederverwendung |
| K-4 | Ratenbegrenzung: offene Kopplungen je Adresse und gesamt; falsche Benutzercodes je Konto zählen wie falsche PINs | Raten, Überfluten |
| K-5 | Die Seite `/link` bestätigt nie von selbst. Sie zeigt Gerätename und Alter der Anfrage und den Satz «Bestätige nur, wenn dieser Code jetzt auf deinem eigenen Fernseher steht.» | untergeschobene Links |
| K-6 | Gerätesitzung ohne Verwaltung und ohne Kontopflege (4.5) | Schaden, falls doch ein fremdes Gerät gekoppelt wird |
| K-7 | Eine bestätigte Kopplung erscheint als Benachrichtigung beim Konto und in der Geräteliste | unbemerkte Kopplung |
| K-8 | `devicecode` und `usercode` kommen in die Redaktionsliste der Logs (Regel 10) | Codes in Logs |

**Bekannte Grenze:** Ein Angreifer kann eine Kopplung auf seinem eigenen Gerät beginnen und einem
angemeldeten Kontoinhaber den Link samt Code schicken. Bestätigt dieser trotz K-5, schaut der
Angreifer mit. K-6 und K-7 begrenzen den Schaden und machen ihn sichtbar, sie verhindern ihn nicht.
Dieses Muster ist bei jeder Kopplung per Code so. Ob der QR-Code den Benutzercode schon mitbringen
darf oder ob man ihn immer abtippen muss, ist Frage F-3.

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
Anzeige, Tonausgang), mit `type: tv` und einem Namen wie «Chromecast · Android TV 14».

Der Vertrag braucht dafür vier Ergänzungen. Alle sind zusätzlich; ein Client, der sie nicht sendet,
bekommt das heutige Verhalten.

| Ergänzung | Bedeutung | Wirkung im Server |
|---|---|---|
| Kennzeichen für einen nativen Player | «spielt HLS selbst, ist aber nicht Safari» | Die Safari-Sonderfälle in `directVideo` und `hdrRemux` hängen heute an `hls: native`; sie müssen an Safari hängen, nicht an der App |
| höchste Kanalzahl des Tonausgangs | 2, 6 oder 8 | beim Neu-Kodieren Mehrkanal statt Stereo (5.3) |
| «wählt Spuren selbst» | der Player wählt Ton- und Untertitelspur in der Originaldatei | eine andere Tonspur erzwingt keinen Remux mehr |
| Untertitelformate, die der Player selbst zeichnet | zum Beispiel `ass`, `pgs` | diese Spuren werden nicht mehr eingebrannt (5.4) |

### 5.3 Mehrkanalton (E-6)

Zwei Fälle:

1. **Die Datei läuft unverändert** (Direktwiedergabe oder Remux mit kopiertem Ton). Dann gibt die
   App die Tonspur so weiter, wie sie ist. Kann die Tonanlage das Format (Dolby Digital, Dolby
   Digital Plus, DTS, TrueHD), reicht das Gerät es durch; sonst dekodiert das Gerät, falls es das
   Format kennt. Die App meldet dem Server nur Formate, für die eines von beidem zutrifft.
   Dafür muss im Server nichts geändert werden: Er kopiert den Ton schon heute, wenn der Codec
   gemeldet ist.
2. **Der Server muss den Ton neu kodieren** (das Gerät kann das Format nicht, oder das Video wird
   ohnehin umgerechnet). Heute entsteht Stereo-AAC. Neu: Meldet das Gerät 6 Kanäle oder mehr und
   hat die Quelle so viele, kodiert der Server in 5.1. Vorschlag für das Format: Dolby Digital
   (AC-3), wenn das Gerät es meldet, weil es Soundbars und Receiver auch über den Rückkanal des
   Fernsehers (ARC) erreicht; sonst AAC 5.1. Stufe 0 misst, welches der beiden auf den drei Geräten
   sauber läuft.

Welche Formate ein Gerät durchreicht, hängt am Gerät und an der Tonanlage; eine verlässliche
Aussage von Google dazu war nicht zu finden (unbelegt). Die App bekommt darum in den Einstellungen
der Wiedergabe einen Schalter «Ton»: automatisch, immer Stereo, Formate von Hand. So kommt man
weiter, wenn die Erkennung am Gerät falsch liegt.

### 5.4 Untertitel

- Text (SRT, WebVTT, ASS): ExoPlayer zeichnet sie selbst. Die ASS-Gestaltung bleibt nach Googles
  Liste nur zum Teil erhalten.
- Bild (PGS, DVD): In Googles Formatliste stehen sie nicht; ein anderes Projekt gibt an, ExoPlayer
  könne PGS (unbelegt). Stufe 0 misst das. Wenn es trägt, meldet die App `pgs`, und der Server
  brennt diese Spuren nicht mehr ein. Wenn nicht, bleibt es beim Einbrennen wie im Browser.

### 5.5 HDR

Die App meldet `hdr: true`, wenn Gerät und Anzeige HDR können. Dolby Vision Profil 5 und 7 bleiben
wie heute ausgenommen. Das Umschalten der Bildfrequenz des Fernsehers auf die des Films ist nicht
Teil der ersten Ausgabe (Frage F-8).

### 5.6 Tonplayer (Stufe 4)

Hörbücher, Podcasts und Webradio nutzen dieselben Endpunkte wie die Web-App und einen eigenen
Tonbildschirm mit Titelbild, Kapiteln oder Titelanzeige. Der Ton läuft weiter, solange die App im
Vordergrund ist; der Bildschirmschoner des Geräts darf hier kommen. Weiterlaufen im Hintergrund
ist nicht Teil der ersten Ausgabe.

## 6. Wünsche über Seerr (Stufe 5)

Die App zeigt den Bereich «Wünsche» wie die Web-App (Konzept 25): Entdecken, Filme, Serien, Suche,
die Seite eines Titels ausserhalb der Bibliothek, Anfordern, Zurückziehen, Stand und Fortschritt.
Der Server ändert sich dafür nicht. Freigeben und Ablehnen für Administratoren bleibt im Browser,
weil eine Gerätesitzung keine Verwaltungsrechte hat (4.5).

## 7. Änderungen am Server im Überblick

| Was | Wo | Art |
|---|---|---|
| Kopplung: vier Operationen, Tabelle für offene Kopplungen | Vertrag, `auth`, Migration | neu, zusätzlich |
| Gerätesitzung: Kennzeichen an der Sitzung, Sperre für `admin`, `owner` und Kontopflege | Sitzungs-Middleware, Migration | neu |
| Web-Seite `/link` mit Tastaturtest, Texte in allen Sprachen | `web/src` | neu |
| Benachrichtigung «Neues Gerät gekoppelt» | `notify` | neu |
| `PlaybackCapabilities`: vier Ergänzungen (5.2) | Vertrag, `playback/decide.go` | zusätzlich |
| Mehrkanal beim Neu-Kodieren | `transcode/pipeline.go` (`outputChannels`) | Verhalten nur für Geräte, die Kanäle melden |
| Geräteliste zeigt Typ `tv` | Verwaltung | klein |
| Redaktionsliste der Logs um `devicecode`, `usercode` | `internal/logging` | Regel 10 |

Alles zusammen ist eine zusätzliche Vertragsänderung (keine brechende). Sie wird mit Stufe 1
gebaut, bevor die App sie braucht.

## 8. Verteilung (E-2)

### 8.1 Privat

1. **Während des Baus:** Installation über ADB im Heimnetz. Marco schaltet dafür auf jedem Gerät
   einmal die Entwickleroptionen ein.
2. **Sobald Google die Identität bestätigt hat:** interne Testspur im Play Store. Marco trägt die
   Google-Adressen von Familie und Freunden ein (bis 100), sie melden sich über einen Link an und
   finden die App danach im Play Store des Fernsehers. Ob der letzte Schritt auf einem Fernseher so
   klappt, misst Stufe 0 (unbelegt).

Für die Testspur braucht es einen festen Paketnamen (Frage F-7), ein App-Bundle und einen
Signaturschlüssel. Der Paketname lässt sich nach dem ersten Hochladen nie mehr ändern.

### 8.2 Öffentlich (spätere Stufe)

- Geschlossener Test mit 12 Testern über 14 Tage (privates Konto).
- TV-Prüfung nach Googles Liste (2.6).
- Store-Eintrag: Banner, Bildschirmfotos, Beschreibung, Datenschutzerklärung, Angaben zur
  Datensicherheit, Alterseinstufung.
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

- Die App hat eine eigene Versionsnummer und kennt die kleinste Vertragsfassung, die sie braucht.
  Beim Verbinden liest sie `GET /healthz`. Ist der Server zu alt, zeigt sie «Server zu alt, bitte
  aktualisieren» statt halb zu funktionieren.
- Unbekannte Felder ignoriert die App.
- **Neue Regel für den Vertrag:** Operationen, die die TV-App nutzt, dürfen nicht mehr von einem
  Release aufs nächste brechen. Eine brechende Änderung braucht eine Übergangszeit, in der der
  Server alt und neu liefert. Wie lang diese Zeit ist, ist Frage F-9.

Solange die App nur privat verteilt wird, hat Marco beide Seiten in der Hand; zwingend wird die
Regel mit der öffentlichen Stufe. Sie von Anfang an einzuhalten, kostet aber am wenigsten.

## 9. Prüfung

| Ebene | Was | Entspricht |
|---|---|---|
| Unit | Zustände der Seiten, Fähigkeitsprofil, Fehlerbilder | Vitest der Web-App |
| Oberfläche | je Seite ein Test mit Tastenereignissen des Steuerkreuzes: jeder Knopf erreichbar, Fokus sichtbar, Zurück führt hinaus, keine Fokusfalle | Regel 8 |
| Vertrag | die App gegen einen echten Server mit Testdaten, wie die e2e-Läufe | `apicheck` |
| Server | Kopplung und Gerätesitzung im Vertragstest (`TestContractCoversEveryOperation`), Entscheid-Tests für die neuen Fähigkeiten | wie heute |
| Geräte | Abnahme durch Marco auf Chromecast, Shield und Fernseher, je Stufe eine Liste | Abnahme auf CT 190 |

Die Teststufen aus `CLAUDE.md` gelten weiter. Ob die Oberflächentests in der CI auf einem
Emulator laufen können und wie lange das dauert, misst Stufe 2 und schreibt es nach
`docs/perf/`.

## 10. Stufen

Jede Stufe ist ein eigener Auftrag von Marco und endet mit etwas, das er selbst prüfen kann.

| Stufe | Inhalt | Ergebnis für Marco |
|---|---|---|
| 0 Messung | Wegwerf-App auf den drei Geräten, kein Eingriff ins Produkt. Misst: Direktwiedergabe (MKV, HEVC, HDR), Tonformate und Durchreichen je Gerät, AC-3 gegen AAC 5.1 aus dem Server, PGS und ASS, Tempo von Compose auf dem schwächsten Gerät, Generator für den Kotlin-Client, Installation aus der internen Testspur. | Bericht `docs/perf/K26-android-tv.md` mit Entscheid je Punkt |
| 1 Server | Kopplung, Gerätesitzung, Seite `/link`, Ergänzungen am Fähigkeitsprofil, Mehrkanal beim Neu-Kodieren | Kopplung im Browser durchspielbar; ein Release |
| 2 Grundgerüst | Projekt `android/`, Werkzeuge, CI, Tokens, Texte, Serveradresse, Anmeldung, Profilwahl, Start, Bibliothek, Detailseiten, Suche, Listen | App auf dem Fernseher, blättern ohne Abspielen |
| 3 Videoplayer | Player vollständig, Mehrkanalton, HDR, Einstellungen der Wiedergabe | Filme und Serien schauen; erste Ausgabe für Familie und Freunde |
| 4 Ton | Hörbücher, Podcasts, Webradio | |
| 5 Wünsche | Bereich «Wünsche», Benachrichtigungen | erste Ausgabe ist vollständig nach E-5 |
| 6 Öffentlich | Geschlossener Test, TV-Prüfung, Store-Eintrag, Vorführserver, Verträglichkeitsregel verbindlich | App im Play Store |

Das Mockup (Frage F-2) liegt zwischen Stufe 1 und Stufe 2. Stufe 0 und Stufe 1 hängen nicht
voneinander ab.

## 11. Änderungen an früheren Entscheiden und Regeln

Alle erst mit dem Bau und auf Marcos Wort, nicht mit diesem Papier.

| Stelle | Heute | Neu |
|---|---|---|
| Spec «Non-Goals» | TV-Apps (Android TV, Apple TV, webOS, Tizen) und native Apps sind Nicht-Ziel | Android TV ist Ziel (Frage 34). Apple TV, webOS, Tizen, Fire TV und native Apps für Handy und Tablet bleiben Nicht-Ziele |
| `CLAUDE.md` Regel 11 | «TV-Apps, native Apps» in der Liste der Nicht-Ziele | benannte Ausnahme: die Android-TV-App nach Frage 34 und ADR 0039 |
| `CLAUDE.md` Regel 1 | vier erzeugte Verzeichnisse | dazu die erzeugten Verzeichnisse unter `android/` |
| `CLAUDE.md` Regel 8 | Playwright-Tastaturtest je neuer Seite | dazu: je TV-Seite ein Steuerkreuz-Test |
| `CLAUDE.md` Regel 9 | Farben aus Tokens, Texte aus `web/src/locales`, gemeinsame Bauteile | gilt für `android/` mit erzeugten Farben und Texten; der Satz zu `web/src/ui` gilt nur für die Web-App |
| ADR 0005 | eine Sitzungsart | Nachtrag: Gerätesitzung ohne Verwaltung (ADR 0040) |
| Regel «Vertrag = Tag» | Vertrag darf je Release brechen | Übergangszeit für Operationen der TV-App (8.4, ADR 0039) |

## 12. Risiken

| Nr. | Risiko | Folge | Gegenmittel |
|---|---|---|---|
| R-1 | Zweite Oberfläche | jede Zuschauer-Funktion doppelt, dauerhaft | TV-Teil als fester Abschnitt in jedem künftigen Konzept; kleiner Umfang der TV-Seiten |
| R-2 | Schwache Sticks | ruckelnde Oberfläche | Stufe 0 misst auf dem schwächsten Gerät; Bildgrössen vom Server passend anfordern |
| R-3 | Tonformate je Gerät verschieden | kein Ton oder Stereo trotz Tonanlage | Schalter «Ton» (5.3), Messung je Gerät |
| R-4 | Kopplung als neuer Weg zur Sitzung | Mitschauen durch untergeschobenen Code | K-1 bis K-8, Gerätesitzung ohne Verwaltung, Kritiker-Review vor der Freigabe |
| R-5 | Vertrag bricht eine installierte App | App auf dem Fernseher geht nicht mehr | Verträglichkeitsregel (8.4) |
| R-6 | Google lehnt die App ab oder verlangt mehr | öffentliche Stufe verzögert sich | private Verteilung hängt nicht davon ab |
| R-7 | Identitätsprüfung bei Google dauert | interne Testspur später | bis dahin ADB |
| R-8 | 12 Tester über 14 Tage | öffentliche Stufe braucht Mitmacher | Familie und Freunde aus der privaten Stufe |
| R-9 | Zweite Werkzeugkette (JDK, Android-SDK, Gradle) | längere CI, mehr Pflege | eigener CI-Job nur bei Änderungen an der App |
| R-10 | Texte in 23 Sprachen auch für die TV-App | mehr Übersetzungsarbeit je Funktion | eine Quelle, ein Übersetzungsweg (3.5) |

## 13. Fragen

Bis zur Antwort gilt die Empfehlung. Die Bau-Session legt offene Fragen vor ihrer Stufe vor.

| Nr. | Frage | Empfehlung | Fällig vor |
|---|---|---|---|
| F-1 | App im selben Repo (`android/`) oder in einem eigenen? | selbes Repo | Stufe 2 |
| F-2 | Klickbares Mockup der TV-Oberfläche vor dem Bau? | ja | Stufe 2 |
| F-3 | Darf der QR-Code den Benutzercode mitbringen, oder muss man ihn am Handy abtippen? | mitbringen, mit K-5 bis K-7 | Stufe 1 |
| F-4 | Wie lange bleibt ein gekoppelter Fernseher ohne Benutzung angemeldet? Heute 30 Tage gleitend. | 400 Tage gleitend für Gerätesitzungen, wie das Geräte-Cookie | Stufe 1 |
| F-5 | Serveradresse nur von Hand oder auch Suche im Heimnetz? | erste Ausgabe von Hand | Stufe 2 |
| F-6 | Unverschlüsseltes HTTP zum Server erlauben? | ja, mit Hinweis, wenn die Adresse nicht im Heimnetz liegt | Stufe 2 |
| F-7 | Paketname der App (unveränderlich nach dem ersten Hochladen) | `com.slouchcat.app`, neutral, falls später eine Handy-App denselben Store-Eintrag nutzen soll | interne Testspur |
| F-8 | Dolby Vision Profil 5 und 7 und Umschalten der Bildfrequenz in der ersten Ausgabe? | nein, spätere Stufe | Stufe 3 |
| F-9 | Übergangszeit für brechende Vertragsänderungen an Operationen der TV-App | zwei Minor-Releases des Servers | Stufe 6, empfohlen ab Stufe 2 |
| F-10 | Vorführserver für Googles Prüfer: wo und mit welchem Inhalt? | kleiner Server im Internet mit frei lizenzierten Testfilmen, nur für die Prüfung | Stufe 6 |
| F-11 | Hochladen in den Play Store von Hand oder aus der CI (braucht einen Dienstschlüssel von Google als Secret)? | von Hand, solange privat | Stufe 3 |
| F-12 | Kleinste Android-Version (`minSdk`) | nach Stufe 0 aus den drei Geräten ableiten; Google erlaubt höchstens 31 | Stufe 2 |
| F-13 | Reihe «Weiterschauen» auf dem Startbildschirm von Google TV? Sie zeigt Titel ausserhalb der App, für alle im Raum sichtbar. | nicht in der ersten Ausgabe | nach Stufe 5 |
| F-14 | Fire TV | eigenes Konzept nach Stufe 6. Neue Fire-TV-Geräte mit Vega OS führen keine Android-Apps aus | – |

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

Unbelegt geblieben und darum in Stufe 0 zu messen: Verhalten der internen Testspur auf einem
Fernseher, Durchreichen von Tonformaten je Gerät, PGS in ExoPlayer, Tempo von Compose auf schwachen
Sticks, ob die Testpflicht für reine TV-Apps gilt.
