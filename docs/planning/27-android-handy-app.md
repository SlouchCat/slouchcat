# Konzept: Native Android-Handy-App – stöbern, abspielen, Downloads, Fernseher verbinden

- **Status: freigegeben am 21. November 2026 (Frage 35), Fassung 6 (20. November 2026).** Marco hat an diesem Tag den Bau beider Konzepte bestellt («Starte mit der Umsetzung von Konzept 26 + 27. Mach alles 100% selbständig»); für offene Fragen gilt die Empfehlung. Fassung 2 hat das
  Kritiker-Review der Fassung 1 eingearbeitet (REVISE, 0 Blocker, 8 Major, 17 Minor), Fassung 3
  die zweite Lesung (REVISE nahe an der Annahme, 0 Blocker, 1 Major, 13 Minor); beides in
  Abschnitt 19. Der Kritiker hat dabei erstmals Abschnitt 4 von Konzept 26 gelesen (2 Major,
  7 Minor; Abschnitt 18). **Fassung 4** trägt vier Punkte nach, die die dritte Lesung von
  Konzept 26 diesem Papier zugewiesen hat (19.2). **Fassung 5** arbeitet Marcos Entscheide E-21
  und E-22 vom 20. November 2026 ein: zwei getrennt betriebene Dienste, die sich ohne Mühe
  umziehen lassen (1.3, Abschnitt 7, 19.3). **Fassung 6** arbeitet die dritte Lesung ein, die
  Fassung 3 bis 5 geprüft hat (Annahme mit Vorbehalt, 0 Blocker, 0 Major, 13 Minor; 19.4). Die
  Einarbeitung dieser dritten Lesung hat der Kritiker nicht mehr gegengelesen. Gebaut wird erst
  nach Marcos Freigabe und auf seinen eigenen Auftrag je Stufe.
- **Auslöser:** Marco am 19. November 2026 im Konzept 26 «Android-TV-App»: «Eigentlich möchte
  ich es so: User lädt sich die Android App herunter und konfiguriert alles. Danach kann er mit
  dieser App den QR Code auf dem TV scannen und auf dem TV wird alles eingerichtet (sofern mit
  Internet verbunden).» Auf die Rückfragen hat er «Neue native Android-App» (E-10), «Über einen
  Dienst im Internet» (E-11) und «Volle Handy-App zuerst» (E-12) gewählt.
- **Verankert gegen:** `main` **`d70c1c79`** («Merge pull request #188», Release 1.41.2),
  Vertragsfassung **1.41.0** (`api/openapi.yaml:4`), letzte Migration
  **`00081_wishes_request_open.sql`**, letzte ADR **`0038`**, letzte Frage im
  Entscheidungsprotokoll **33**. Der Zweig dieses Papiers sitzt auf `879d1a7f` («Merge pull
  request #189», Seitenleiste der Verwaltung); gegenüber dem Anker ändert das weder Vertrag noch
  Server, CI, letzte ADR oder letzte Frage. Konzept 26 liegt auf dem Zweig
  `claude/konzept-android-tv` (Fassung 5, Stand `82c21a63`), nicht in `main`; Verweise «K26 4.3»
  meinen dessen Abschnitte.
- **Nummern:** Dieses Papier ist **Konzept 27** und reserviert **Frage 35** sowie **ADR 0043**
  («Native Android-Handy-App: Offline und Abruf im Hintergrund») und **ADR 0044** («Push an die
  Handy-App über einen Weiterleitungsdienst»). Konzept 26 hat Frage 34 und ADR 0039 bis 0042
  reserviert. Drei davon beschreiben das gemeinsame Fundament und entstehen darum mit dem Bau
  dieses Konzepts: ADR 0039 (native Clients im selben Repo), ADR 0040 (Sitzungsarten, Kopplung),
  ADR 0042 (Vermittlung im Internet; Betrieb und Umzug der zwei Dienste). ADR 0041 (Ton- und
  Spurfähigkeiten) bleibt bei der TV-App.
  Migrations- und Versionsnummern vergibt der Bau.
- Pfade ohne Präfix liegen unter `web/src/` oder `server/`. **Nach Namen suchen, nicht nach
  Zeile**; Zeilen beziehen sich auf den Anker.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
  Dieses Papier schlägt Änderungen an früheren Entscheiden und an `CLAUDE.md` vor (Abschnitt 14);
  es ändert selbst keine Regel.

> **Dieses Papier ändert keine Zeile Code.** Marcos Entscheide stehen in Abschnitt 1 und nur dort.
> Alles andere ist Vorschlag der Redaktion, auch wo es nicht eigens dabeisteht. Aussagen über den
> Code stammen vom Anker, Aussagen über Android und Google Play aus einer Recherche vom
> 19. November 2026 (Anhang A). Was dort nicht belegt werden konnte, heisst im Text «unbelegt» und
> wird in Stufe 0 gemessen oder beim ersten Hochladen geprüft.

## 1. Was Marco will

### 1.1 Aus Konzept 26 übernommen

Diese Entscheide hat Marco am 19. November 2026 für die TV-App gefällt; sie legen die Handy-App
mit fest. Wortlaut und Empfehlung stehen in K26 Abschnitt 1.

| Nr. | Entscheid | Folge für die Handy-App |
|---|---|---|
| E-2 | «Erst privat, später öffentlich» | gilt für die Handy-App gleich (Abschnitt 11) |
| E-3 | «Ganz nativ» | Kotlin, Compose, Media3; keine Web-Oberfläche in der App |
| E-8 | «Downloads ist nur wichtig für die Handy Apps.» | Downloads und Offline sind Kern der Handy-App (Abschnitt 9) |
| E-10 | «Neue native Android-App» scannt den QR-Code am Fernseher | «Fernseher verbinden» in der Handy-App (Abschnitt 10) |
| E-11 | «Über einen Dienst im Internet» | Vermittlungsdienst `relay/` (Abschnitt 7) |
| E-12 | «Volle Handy-App zuerst» | dieses Konzept wird vor der TV-App gebaut |
| F-4 | Gerätetoken eines Fernsehers: «Unbegrenzt» | Gerätesitzungen laufen nicht ab (Abschnitt 4) |
| F-15 | «Meine Geräte» für jedes Konto: «Ja» | Seite und Operationen entstehen hier (Abschnitt 4.5) |

### 1.2 Für dieses Konzept entschieden

Marco hat am 19. November 2026 acht Weichen über Rückfragen gestellt. Die Tabelle nennt die
gewählte Antwort wörtlich, ausser bei E-19: Dort steht sie sinngemäss, weil das Repo öffentlich
ist und die Antwort den Ort eines Dienstes nennt, der vom Internet aus erreichbar ist.

| Nr. | Frage | Marcos Wahl | Empfehlung war |
|---|---|---|---|
| E-13 | Was soll die erste Ausgabe ausser Filmen, Serien, Downloads, Profilen und «Fernseher verbinden» können? | «Hörbücher und Podcasts», «Webradio», «Wünsche über Seerr», «Benachrichtigungen» (alle vier) | keine |
| E-14 | Soll die Handy-App auch den Verwaltungsbereich bekommen? | «Nein, nur Nutzerbereich» | Nein, nur Nutzerbereich |
| E-15 | Wie lange bleibt die Handy-App angemeldet, wenn sie nicht benutzt wird? | **«Unbegrenzt, wie der Fernseher»** | 400 Tage ohne Benutzung |
| E-16 | Wer darf einen Fernseher per QR-Scan mit einem Konto verbinden? | «Nur die Handy-App» | Nur die Handy-App |
| E-17 | Wie kommen Meldungen aufs Handy, wenn die App zu ist? | **«Push über eigenen Dienst»** | Push über das Web-Push, das der Server schon hat |
| E-18 | Darf sich die Handy-App über unverschlüsseltes HTTP mit einem Server verbinden? | **«HTTP überall mit Warnung»** | HTTP nur im Heimnetz |
| E-19 | Wo läuft der Vermittlungsdienst? | **eigene Antwort, sinngemäss: vorläufig auf eigener Hardware, später auf einem Mietserver** | eigener kleiner Mietserver |
| E-20 | Eine App für Handy und Fernseher im Play Store oder zwei? | «Eine App: com.slouchcat.app» | Eine App: `com.slouchcat.app` |

Die Folgen standen in den Fragen:

- **Zu E-14:** «Der Server gibt der App-Anmeldung keine Verwaltungsrechte; ein verlorenes Handy
  kann also nichts verstellen.»
- **Zu E-15:** «Ein verlorenes Handy behält den Zugang, bis es jemand unter ‹Meine Geräte›
  abmeldet.»
- **Zu E-16:** «Wer bloss ein Browser-Cookie erbeutet, kann kein dauerhaftes Gerät anhängen.
  iPhone-Nutzer richten den Fernseher von Hand ein.» Die Frage war, wer per QR-Scan verbinden
  darf. Dass es daneben keinen dritten Weg über einen Code im Browser gibt (K26 F-3), ist daraus
  abgeleitet und nicht eigens entschieden.
- **Zu E-17:** «Du betreibst dauerhaft einen weiteren Dienst, über den jede Meldung aller Nutzer
  läuft (verschlüsselt).» Die abgelehnte Empfehlung hätte keinen eigenen Dienst gebraucht, war
  aber «für native Apps wenig verbreitet». Die Option sprach von einem «zweiten Dienst»; mit
  E-21 (1.3) hat Marco das bestätigt: Es sind zwei getrennt betriebene Dienste.
- **Zu E-18:** «In fremden Netzen kann jemand Passwort und Anmeldung mitlesen und den Zugang
  übernehmen.» Zusammen mit E-15 heisst das: Ein mitgelesenes Anmeldetoken gilt, bis das Gerät
  abgemeldet wird. **Nicht in der Frage stand,** was der Kritiker gefunden hat: Auch eine Adresse
  im Heimnetz ist betroffen, sobald das Handy in einem fremden Netz mit denselben Adressen ist.
  Das steht in Abschnitt 4.6 und ist Frage F-38.
- **Zu E-19:** Marco hat keine der drei Optionen gewählt, sondern selbst geantwortet. E-21 und
  E-22 (1.3) führen den Entscheid fort; Abschnitt 7.5 nennt, was der vorläufige Ort verlangt.
- **Zu E-20:** Der Fragetext nannte: «Der Paketname der App im Play Store lässt sich nach dem
  ersten Hochladen nie mehr ändern.» Die Option nannte: «Der Test mit 12 Testern fällt
  voraussichtlich nur einmal an (von Google nicht ausdrücklich bestätigt).»

### 1.3 Nachtrag vom 20. November 2026: die zwei Dienste

Marco hat am 20. November 2026 in der Sitzung zu Konzept 26 zum Dienst im Internet entschieden;
diese Sitzung hat seine Worte und die gewählten Optionen wörtlich übermittelt. Wie bei E-19 steht
hier der Sinn und nicht der Wortlaut, weil das Repo öffentlich ist: Die Antworten nennen
Maschinen, einen Anbieter und ein Produkt in Marcos eigenem Netz.

| Nr. | Frage | Marcos Wahl (sinngemäss) | Empfehlung war |
|---|---|---|---|
| E-21 | (von Marco selbst) | **Zwei Dienste. Vorläufig läuft jeder auf einem eigenen Container auf eigener Hardware; sie sollen sich ohne Mühe auf zwei Mietserver verschieben lassen.** Auf die Rückfrage, was jetzt zu tun ist: «Ins Konzept schreiben»; die Container entstehen erst mit dem Bau der Dienste | – |
| E-22 | Wie sind die zwei Dienste aus dem Internet erreichbar, solange sie auf eigener Hardware laufen? | **Über den Reverse-Proxy, der dort schon steht** | ein Tunnel eines Dritten |

Die Folgen standen in den Optionen zu E-22. Zur gewählten: Port 443 am eigenen Anschluss, der
Name der Dienste zeigt auf diesen Anschluss, der Umzug hat «zwei Schritte mehr», dafür steht
«kein Dritter dazwischen». Zur abgelehnten: kein offener Port und kein Name auf dem eigenen
Anschluss, dafür steht der Anbieter des Tunnels «dauerhaft dazwischen und sieht Adressen und
Zeitpunkte».

**Was E-21 entscheidet und was nicht:** Entschieden ist der getrennte Betrieb auf zwei Maschinen
und dass der Umzug leicht sein muss. Nicht entschieden ist, ob es zwei Programme sind oder ein
Programm, das je Maschine nur einen Teil einschaltet (Frage F-39), und wie der Aufbau im
Einzelnen aussieht; Abschnitt 7.5 ist Vorschlag der Redaktion.

Was Marco nicht entschieden hat, steht als Frage mit Empfehlung in Abschnitt 17.

## 2. Heute

### 2.1 Regeln

Native Apps sind heute ein Nicht-Ziel (Spec «Non-Goals», `CLAUDE.md` Regel 11, Plan Abschnitt 1),
«Cloud» ebenso. Die Spec hat den Weg offen gehalten: Kriterium C-5 verlangt einen Vertrag, mit dem
«ein Fremd-Client alle Zuschauerfunktionen umsetzen kann», Plan P-16 prüft das mit
`server/cmd/apicheck`, und Plan 3.8 Punkt 7 sieht Bearer-Token für solche Clients vor.

Regel 12 und Plan-Prinzip 1 («Entscheid im Server, offline nur Nachschlagen») kennen zwei erklärte
Ausnahmen (Plan 3.3): die Offline-PIN (AN-19) und die Schauzeit offline (AN-18). Beide gelten für
die Web-App und unverändert für die native App; eine dritte kommt nicht dazu.

### 2.2 Die Web-App auf dem Handy

Die Web-App läuft schon heute als PWA auf Android, samt Downloads. Sie speichert die Datei im
Browser (OPFS) und spielt sie über Media Source Extensions (`features/offline/`). Grenzen, die
eine native App aufhebt:

- Der Abruf läuft nur, solange die App offen ist (`engine.ts`: Abgleich beim Start, bei
  Rückkehr des Netzes, beim Fokus und alle 60 Sekunden mit aktivem Profil).
- Der Browser kann den Speicher von sich aus räumen; eine Zusage über den Platz gibt es nicht.
- Kein Ton bei ausgeschaltetem Bildschirm mit Bedienung am Sperrbildschirm, wie Hörbuch-Hörer es
  erwarten.
- Die Gerätecheckliste M5 führt «Android Chrome» für Download, Flugmodus und «Nur WLAN» als
  «ausstehend» (`docs/perf/M5.md`, Abschnitt 7).

Die PWA bleibt. Die native App ersetzt sie nicht; iPhone-Nutzer haben weiterhin nur die PWA.

### 2.3 Sitzung und Geräte

- `login` (`POST /auth/login`, `public`) legt eine Sitzung an und setzt das Cookie
  `slouchcat_session`. Die Sitzung läuft 30 Tage gleitend (`SessionLifetime`,
  `server/internal/auth/auth.go`); `sessions.expires_at` ist `NOT NULL`, und `TouchSession`
  schiebt es bei jeder Verlängerung auf 30 Tage ab jetzt. Rund ein Dutzend Abfragen prüfen
  `expires_at > @now` (Wiedergabe, Radio, Löschen, Zugriff).
- Der Zähler für Fehlversuche von `login` hängt an der Adresse des Aufrufers (`AllowAttempt`,
  `server/internal/auth/limits.go`). Anmeldung, Ersteinrichtung, PIN und Einladungen teilen sich
  einen Topf.
- Nach einer geglückten Anmeldung gleicht der Server das Passwort mit Seerr ab (`seen`,
  Konzept 25).
- Die Middleware (`authorize`, `server/internal/api/session.go`) liest nur das Cookie
  (`resolveSession`). Den Kopf `Authorization` liest der Server nicht. Das Schema `bearerToken`
  steht im Vertrag («Bearer token for third-party clients. Added in M2.»), keine Operation nutzt es.
- Jede schreibende Anfrage braucht den Kopf `X-Requested-With: Slouchcat` (`csrfOK`).
- Das **Gerät** ist heute das Cookie `slouchcat_device` (32 Hex-Zeichen, 400 Tage). Die Sitzung
  merkt sich das Gerät (`sessions.device_id`); `ensureDeviceCookie` gibt dem Gerät der Sitzung
  Vorrang vor dem Cookie.
- Am Gerät hängen die PIN-Zähler (`pin_attempts`) und die Downloads (`downloads.device_id`),
  beide mit `ON DELETE CASCADE`. Die Widerrufsliste führt die Kennung des Geräts ohne
  Fremdschlüssel. `checkOfflineRevocations` ist `public` und erkennt das Gerät **nur am
  Cookie** (`CheckOfflineRevocations`, `offline_handlers.go`).
- Name und Typ eines Geräts (`desktop`, `phone`, `tablet`, `tv`, `other`) schreibt der Server
  bei jedem Abspielstart aus `PlaybackCapabilities` (`RecordDeviceProfile`); ein gemeldeter Name
  ersetzt den alten. `devices.account_id` nennt nur das Konto des letzten Abspielstarts.
- **Aufräumung:** `DeleteExpiredSessions` löscht bei jeder Anmeldung auch Geräte, deren letzter
  Kontakt länger als 400 Tage zurückliegt (`DeleteStaleDevices`: `DELETE FROM devices WHERE
  last_seen_at < @before`). Was dann geschieht:
  - Die Downloads des Geräts fallen mit (`ON DELETE CASCADE`).
  - Eine Sitzung des Geräts lebt weiter, verliert aber ihr Gerät (`sessions.device_id ... ON
    DELETE SET NULL`) und bekommt beim nächsten Kontakt ein **neues** (`loadSession`,
    `assignDevice`).
  - Die Einträge der Widerrufsliste (`download_tombstones`, ohne Fremdschlüssel) bleiben unter
    der alten Kennung und verfallen ihrerseits nach 400 Tagen (`tombstoneKeep`).
- `listDevices` und `deleteDevice` sind `admin`. `deleteDevice` entzieht die Sitzungen des Geräts,
  beendet seine Streams, löscht seine PIN-Zähler und die Zeile.
- `updateAccount` verlangt das aktuelle Passwort; ein neues Passwort entzieht schon heute jede
  andere Sitzung des Kontos (`RevokeOtherAccountSessions`).
- Die Kennung `ffff…` ist für die PIN-Obergrenze je Profil reserviert (`ReservedDeviceID`).
- 307 Operationen: 179 `admin`, 92 `scoped`, 24 `account`, 8 `public`, 4 `owner`.

### 2.4 Downloads und Offline im Vertrag

Der Server bereitet jeden Download selbst vor; das Gerät holt eine fertige Datei.

| Schritt | Operation | Was geschieht |
|---|---|---|
| Auswahl | `getDownloadOptions` | drei Stufen `high` (bis 1080p), `medium` (bis 720p), `low` (bis 480p) mit geschätzter Grösse, Tonspuren, Untertitel |
| Bestellen | `createDownloads` | ein Download je Titel **auf dem Gerät der Sitzung** (`device_required` ohne Gerät); eine Staffel in einer Aktion |
| Vorbereiten | Job `download.prepare` | H.264 und AAC in fMP4 mit fester Bitrate, als Kopie oder Transcode; wartet, wenn das Budget voll ist, und weicht der Live-Wiedergabe |
| Abholen | `getDownloadManifest`, dann `getDownloadFile` | Manifest mit Grösse, Fragmentindex, Kapiteln, Untertiteln und Vorschaubildern; die Datei mit Range und Download-Token `dt` |
| Abgleich | `syncOffline` | Gerät meldet, was es hält; Server meldet Zustand, `revoked`, `allowedProfileIds`, Offline-Profile, Regel für Auto-Downloads |
| Widerruf ohne Sitzung | `checkOfflineRevocations` | Gerät nennt seine Download-IDs, Server nennt die zu löschenden |
| Fortschritt | `uploadOfflineEvents` | Roh-Ereignisse der Offline-Wiedergabe für jedes Profil des Kontos |
| Löschen | `deleteDownload` | Download und vorbereitete Kopie weg; das Gerät erfährt es beim Abgleich |
| Auto-Downloads | `getDownloadRules`, `setDownloadRules` | nächste Folgen und neue Podcast-Folgen je Profil und Gerät; `wifiOnly` prüft der Client |
| Offline-PIN | `setOfflinePin`, `removeOfflinePin` | eigene, längere PIN für die Offline-Bibliothek |

Einzelheiten, die für eine native App zählen:

- **Das Download-Token `dt` läuft nicht ab.** Es ist ein HMAC über Download, Titel und Profil
  (`SignToken`, `server/internal/offline/manifest.go`); jede Anfrage prüft, ob das Profil den
  Titel noch sieht. Die Datei lässt sich also ohne Sitzung und im Hintergrund holen.
- Der Server hält die vorbereitete Kopie 24 Stunden nach dem vollständigen Abruf und 7 Tage ohne
  Abruf (`fetchedKeep`, `unfetchedKeep`); danach `410 download_evicted`, der nächste Abgleich
  bereitet neu vor.
- **Das Budget von 100 GB ist das des Servers** für vorbereitete Kopien
  (`SLOUCHCAT_DOWNLOAD_BUDGET_GB`, Entscheid 8, AN-20). Für den Speicher auf dem Gerät kennt der
  Vertrag keine Grenze.
- Hörbücher und Podcast-Folgen sind Downloads mit `audio/mp4` (AAC 128, 96 oder 64 kbit/s,
  `server/internal/offline/audio.go`). Ein Hörbuch ist ein Download über alle Teile.
- Bildspuren-Untertitel gibt es offline nicht («bitmap subtitles are not available offline»).
- Die Offline-PIN prüft das Gerät gegen einen PBKDF2-SHA256-Verifier (`OfflineProfile`,
  `features/offline/pin.ts`); die Schauzeit zieht das Gerät nach eigener Uhr vom Tagesrest ab
  (`OfflineWatchTime`).

### 2.5 Meldungen

- Die Liste in der App gehört zu einem Profil (`listNotifications`, `scoped`), dazu die
  Ankündigungen (`listMyAnnouncements`).
- Kanäle nach draussen gehören zum **Konto**: E-Mail, Telegram und Web-Push (`getMyChannels`,
  `createPushSubscription`). Ändern darf sie nur eine Sitzung ohne aktives Profil oder mit dem
  Hauptprofil (`primary_profile_required`).
- Web-Push: Der Server verschlüsselt selbst (`server/internal/notify/push`, mit VAPID) und
  schickt an jede Adresse, die mit `https` beginnt (`CheckEndpoint`) und nicht auf eine private
  oder lokale Adresse zeigt (`netguard.RefuseLocal`). Antwortet die Gegenstelle mit 404 oder
  410, löscht der Server das Abo.
- Die Nutzlast ist `{title, body, url}`. `url` ist eine vollständige Adresse der Web-App, wenn
  der Server seine öffentliche Adresse kennt, sonst nur ein Pfad (`absolute`,
  `server/internal/notify/texts.go`). Jede Meldung geht an **alle** Abos des Kontos
  (`sendPush`), mit dem Profilnamen vorangestellt (`Emit`).
- Ein Kanal sendet nur, wenn er eingeschaltet ist; einschalten darf ihn nur das Hauptprofil
  (`updateMyChannel`).
- **Jeder verschlüsselte Körper ist genau 4096 Byte gross.** Die Bibliothek füllt auf die
  Satzgrösse auf, und der Server setzt sie nicht (`webpush.Options` ohne `RecordSize`).
- **Ein Abo überlebt jede Abmeldung.** `push_subscriptions.device_id` ist `ON DELETE SET NULL`;
  weder `logout` noch `deleteDevice` noch eine Passwortänderung löscht ein Abo. Ein Abo fällt
  nur auf vier Wegen: die Gegenstelle antwortet 404 oder 410, das Konto wird gelöscht,
  `deletePushSubscription` (Hauptprofil) oder ein neuer VAPID-Schlüssel (`RotateVAPID` löscht
  alle). Ein Browser trägt sich danach nicht von selbst neu ein; die Web-App bietet dafür einen
  Knopf (`PushDevices.tsx`).
- Administratoren bekommen über ihre Kanäle auch Fehlermeldungen und Wünsche zur Freigabe
  (`ChannelTargetsAdmins`), mit Zielen unter `/admin`.

### 2.6 Bau und CI

- `task` ist der einzige Einstieg. Die Werkzeuge stehen in `.tool-versions` (Go 1.27.3, Node
  24.22.0, pnpm 12.14.0, task 3.54.0). `server/go.mod` ist das einzige Go-Modul; es gibt kein
  `go.work`.
- Die CI (`.github/workflows/ci.yml`) hat die Jobs `changes`, `server-lint`, `server-groups`,
  `server-test`, `web`, `storybook`, `build`, `e2e`, `image` und das Gate `ci`, das alle in
  `needs` aufzählt. Ein Kommentar verlangt, einen neuen Job dort einzutragen.
- Der einzige Pfadfilter heisst «nur Dokumente» (`.github/scripts/docs-only.sh`): `docs/*`,
  `*.md` und Release-Notizen, ausser unter `web/`, `packages/` und `deploy/`. Alles andere läuft
  durch alle Jobs. Auf `main` überspringen die schweren Jobs ausserdem Inhalt, den ein grüner
  Lauf des Pull Requests schon geprüft hat (`already-tested.sh`).
- Das Release-Workflow reagiert nur auf Tags der Form `v*.*.*` und baut die Images des Servers
  und des Updaters.
- `packages/design-tokens` baut mit Style Dictionary 5.7 zwei Ausgaben, CSS und TypeScript. Es
  gibt **ein** Thema, dunkel. Masse stehen in `rem`, `px`, `ms` und `em`, Farben mit Deckkraft
  als achtstelliges Hex. Die Schrift ist eine Systemschrift-Liste; es gibt keine eigene
  Schriftdatei.
- `web/src/locales/` hält 23 Dateien: 22 Sprachen und die Ergänzung `de-DE` (287 Texte).
  `de.json` hat rund 5'700 Texte in 38 Namensräumen. Mehrzahlformen heissen `_one`, `_other`,
  `_zero` (Russisch und andere auch `_few`, `_many`), Platzhalter `{{name}}`.
  `scripts/i18n/check.ts` prüft die Dateien gegeneinander.

### 2.7 Vorgaben von Android und Google (Recherche, Anhang A)

**Google Play**

- **Privates Entwicklerkonto:** Vor der öffentlichen Freigabe verlangt Google einen geschlossenen
  Test mit mindestens 12 Testern, die 14 Tage am Stück angemeldet sind; wer austritt und wieder
  eintritt, beginnt neu. Danach folgt ein Antrag mit Fragen zum Test, zur App und zur
  Produktionsreife; die Prüfung dauert «in der Regel bis 7 Tage». Ob der Test für eine zweite
  Geräteart derselben App oder für eine zweite App desselben Kontos erneut anfällt: unbelegt.
- **Interne Testspur:** bis 100 Tester je App über eine Liste von E-Mail-Adressen, Installation
  über einen Link. Ob dafür die Identitätsprüfung abgeschlossen sein muss: unbelegt.
- **Eine App für Handy und Fernseher:** Google empfiehlt denselben Paketnamen ausdrücklich
  («strongly recommended»). Für den Fernseher gibt es eine eigene Veröffentlichungsspur; die Wahl
  lässt sich nach dem Speichern nicht mehr ändern.
- **Ziel-API:** für neue Handy-Apps und Updates API 36 (Android 16) seit 31. August 2026, für
  TV-Apps API 34. Die nächste Frist ist nicht angekündigt (unbelegt).
- **Signatur:** Neue Apps nehmen an «Play App Signing» teil; Google hält den Signaturschlüssel,
  der Entwickler einen Upload-Schlüssel, der sich zurücksetzen lässt. Der Paketname ist
  «unique and permanent».
- **Zugang für Googles Prüfer:** Sie müssen sich anmelden und alles benutzen können; verlangt wird
  ein Testzugang, der nicht abläuft. Zu Apps mit eigenem Server des Nutzers sagt die Seite nichts
  (unbelegt). Praktisch heisst das: ein erreichbarer Vorführserver.
- **Angaben im Store:** Datenschutzerklärung unter einer HTTPS-Adresse und das Formular
  «Datensicherheit». Als «erhoben» gilt, was das Gerät verlässt; Ende-zu-Ende-Verschlüsseltes ist
  ausgenommen. Wie Daten an den eigenen Server des Nutzers und wie ein Dienst des Entwicklers
  zählen, der nur Adressen sieht: unbelegt.
- **Kontolöschung:** Die Pflicht gilt nur, wenn sich in der App ein Konto anlegen lässt.
- **Öffentlich sichtbar bei privaten Konten:** der bürgerliche Name und das Land des Entwicklers.
  Ob auch die E-Mail-Adresse: Zwei Seiten von Google widersprechen sich.
- **Vordergrunddienste:** Für Ziel-API 34 und höher muss jede Art erklärt werden, je mit
  Beschreibung und einem Video (`mediaPlayback`: vom Nutzer gestartete Wiedergabe; `dataSync`:
  vom Nutzer gestartete Übertragung).
- **Entwicklerprüfung für Installationen ausserhalb des Stores:** Installation über ADB bleibt
  frei; für Entwickler mit Play-Konto werden Paketnamen von selbst registriert.

**Android**

- **Unverschlüsseltes HTTP** ist seit Android 9 ab Werk gesperrt. Die Freigabe geht je benanntem
  Domainnamen oder für alles; Adressbereiche kennt die Konfiguration nicht (aus der
  Dokumentation geschlossen, nicht ausdrücklich belegt).
- **Zugriff aufs lokale Netz:** Für Apps mit Ziel-API 37 (Android 17) ist jede Verbindung zu
  Adressen im Heimnetz gesperrt, bis der Nutzer die Laufzeit-Berechtigung `ACCESS_LOCAL_NETWORK`
  erteilt. Als lokal gelten 10/8, 172.16/12, 192.168/16, 169.254/16 und 100.64/10. Ohne die
  Berechtigung laufen Verbindungen in ein Timeout. Mit Ziel-API 36 gilt das noch nicht.
- **Abruf im Hintergrund:** Ein Vordergrunddienst der Art `dataSync` darf ab Ziel-API 35 höchstens
  6 Stunden in 24 Stunden laufen. Für vom Nutzer gestartete Übertragungen gibt es seit Android 14
  eigene Aufträge («user-initiated data transfer»), die diese Grenze nicht haben; eine
  Jetpack-Bibliothek dafür gibt es nicht. Der Download-Dienst von Media3 nutzt `dataSync`.
- **Ton im Hintergrund:** `MediaSessionService` von Media3 mit Vordergrunddienst `mediaPlayback`.
- **Grosse Bildschirme:** Mit Ziel-API 36 ignoriert Android auf Anzeigen ab 600 dp Breite die
  Sperre von Ausrichtung und Fenstergrösse; mit Ziel-API 37 entfällt die Ausnahme.
- **Rand-zu-Rand und Zurück-Geste** sind mit Ziel-API 36 Pflicht.
- **Speicher der App:** Der eigene Ordner der App braucht keine Berechtigung, kann auf einer
  SD-Karte liegen und wird beim Deinstallieren gelöscht. Die automatische Sicherung nimmt
  Einstellungen und Dateien ab Werk mit; Ausnahmen regeln die `dataExtractionRules`.
- **Schlüssel und Token:** `androidx.security:security-crypto` ist abgekündigt zugunsten des
  Android Keystore.
- **QR-Scanner:** Der «Google code scanner» braucht keine Kamera-Berechtigung und läuft über die
  Google-Play-Dienste.
- **Google-Push (FCM):** Senden verlangt ein Zugangstoken aus dem Dienstkonto des
  Firebase-Projekts der App. Ein selbst gehosteter Server kann darum nicht direkt senden. Die
  Nutzlast ist auf 4096 Byte begrenzt.
- **Verschlüsselung an einen öffentlichen Schlüssel:** HPKE (RFC 9180) steht in Go seit 1.26 in
  der Standardbibliothek (`crypto/hpke`). Android bringt es erst ab API 37 mit; für ältere Geräte
  gibt es die Bibliothek Tink (Einzelheiten unbelegt, Stufe 0 prüft).
- **Bausteine:** Media3 1.11.1 (stabil), Compose BOM 2026.11.01 mit Material 3 1.5.0, «Material 3
  Adaptive» 1.3.0, Android Gradle Plugin 9.5 mit JDK 17 als Minimum, Kotlin 2.4.

## 3. Umfang der ersten Ausgabe (E-13, E-14)

### 3.1 Seiten

Die App bildet den Nutzerbereich der Web-App ab. Die Spalte «Stufe» verweist auf Abschnitt 13.

| Seite der App | Vorbild in der Web-App | Stufe |
|---|---|---|
| Serveradresse eingeben, Anmelden | `/login` | 3 |
| Profilwahl mit PIN | `/profiles` | 3 |
| Meine Geräte | neu (auch in der Web-App, Stufe 1) | 3 |
| Start (Spotlight, Reihen) | `/` | 4 |
| Bibliothek (Raster, Filter, Sortierung) | `/libraries/$libraryId` | 4 |
| Film, Serie, Staffel, Folge | `/items/$itemId` | 4 |
| Person | `/people/$personId` | 4 |
| Suche | `/search` | 4 |
| Merkliste, Gesehen, Sammlungen, Listen, Empfehlungen | `/watchlist`, `/watched`, `/collections`, `/lists/$key`, `/picks` | 4 |
| Profile verwalten, PIN, Offline-PIN | `/settings/profiles` | 4 |
| Einstellungen der Wiedergabe | `/settings/playback` | 5 |
| Videoplayer | `/play/$itemId` | 5 |
| Downloads, Download-Dialog, Regeln für Auto-Downloads, Speicher | `/downloads` | 6 |
| Offline-Bibliothek und Offline-Player | `/offline`, `/offline/play/$downloadId` | 6 |
| Hörbuch, Hörbuch-Reihe, Podcast, Tonplayer | `/libraries/$libraryId/series/$seriesId`, Tonplayer | 7 |
| Webradio | `/radio` | 7 |
| Wünsche, Entdecken, Titel ausserhalb der Bibliothek | `/wishes`, `/discover…`, `/titles/$media/$tmdbId` | 8 |
| Benachrichtigungen, Ankündigungen, Meldekanäle | `/notifications`, `/settings/notifications` | 8 |
| Fernseher verbinden | neu | 9 |

**Nicht in der App:**

- nach Marcos Entscheid: die Verwaltung (`/admin/*`, E-14).
- als Folgerung der Redaktion daraus: Auch das Freigeben und Ablehnen von Wünschen bleibt im
  Browser, weil es Operationen der Verwaltung sind.
- als Vorschlag: Ersteinrichtung (`/setup/*`), Einladung annehmen (`/invite/$token`),
  Sehbericht der Profile. Wer das braucht, nimmt den Browser. Kontodaten (E-Mail, Passwort)
  lassen sich in der App ändern, weil die Operation das aktuelle Passwort ohnehin verlangt.

### 3.2 Bedienung und Gestaltung

- Navigation unten mit fünf Zielen: Start, Suche, Downloads, Wünsche, Mehr. Auf breiten Anzeigen
  wandert sie an die Seite («Material 3 Adaptive»).
- **Tablets und Querformat** müssen von Anfang an gehen: Mit Ziel-API 36 lässt Android die
  Sperre auf Hochformat auf grossen Anzeigen nicht mehr zu (2.7). Die erste Ausgabe bekommt
  darum zwei Breiten je Seite, keine eigenen Tablet-Seiten.
- Ein Thema, dunkel, aus den Tokens (2.6). Keine Farben aus dem Hintergrundbild des Geräts.
- Marcos Gestaltungsregeln vom 16. November 2026 gelten als Vorschlag auch hier: Klappbares ist
  zu, Kontrast hoch (Rahmen mindestens 3:1, gedämpfter Text mindestens 8:1).
- Bedienhilfen: jedes Bedienelement mit Beschriftung für den Vorleser, Schriftgrösse des Systems
  wird beachtet, Tippflächen mindestens 48 dp.
- **Mockup vor dem Bau (Frage F-22):** Vorschlag ist ein klickbares Mockup in Handygrösse für
  Anmeldung, Profilwahl, Start, Detailseite, Player, Downloads und «Fernseher verbinden». Marco
  gibt es frei, bevor Stufe 4 beginnt.

### 3.3 Was die erste Ausgabe bewusst nicht hat (Vorschlag)

| Funktion | Grund | Frage |
|---|---|---|
| Bild-in-Bild | eigener Prüfaufwand je Gerät | F-28 |
| Auf einen Fernseher übertragen (Google Cast) | braucht eine Empfänger-App; die TV-App kommt ohnehin | F-29 |
| Android Auto | eigene Prüfung durch Google | F-29 |
| Trailer in der App | der Vertrag erlaubt nur den eingebetteten YouTube-Player; die App öffnet YouTube | F-30 |
| Mehrere Server gleichzeitig | eine Adresse, ein Konto | F-31 |
| «Original» als Download-Stufe | braucht eine neue Art von Download im Server | F-27 |
| Sperre der App mit Fingerabdruck | die PINs der Profile bleiben der Schutz | F-32 |

## 4. Sitzungsarten (E-14, E-15, E-16)

### 4.1 Drei Arten

Heute gibt es eine Sitzungsart. Mit den Apps werden es drei. Die TV-Spalte wiederholt K26 4.2 und
4.5; sie steht hier, weil der Serverteil für beide zusammen gebaut wird.

| | Browser-Sitzung | **App-Sitzung** (Handy) | Gerätesitzung (Fernseher) |
|---|---|---|---|
| Träger | Cookie | `Authorization: Bearer` | `Authorization: Bearer` |
| Entsteht durch | `login` | «App anmelden» mit E-Mail und Passwort | Gutschein einlösen oder «App anmelden» mit Art `tv` |
| Läuft ab | 30 Tage ohne Benutzung | **nie** (E-15) | nie (F-4) |
| `admin`, `owner` | nach Rolle | **nie** (E-14) | nie (E-7) |
| `account`, `scoped` | alle | alle | nur die Erlaubnisliste aus K26 4.5 |
| Kopplungsgutschein ausstellen | **nein** (E-16) | **ja** (E-16) | nein |
| Meine Geräte | ja | ja | nein |
| Downloads und Offline | ja | ja | nein (E-8) |

Die App-Sitzung ist damit die Sitzung eines Zuschauers ohne Verwaltung. Ein Administrator sieht
in der App seinen Nutzerbereich; verwalten kann er dort auch dann nicht, wenn jemand die
Anfragen von Hand baut, weil der Server die Art der Sitzung prüft und nicht die App.

**Die Art steht in der Sitzung, und der Träger muss dazu passen.** Der Server leitet die Art nie
aus dem Träger ab:

- Ein Token einer Browser-Sitzung im Kopf `Authorization` wird abgelehnt. Sonst würde aus einem
  erbeuteten Cookie eine App-Sitzung, die Gutscheine ausstellt; das wäre der Weg um E-16 herum.
- Ein Token einer App- oder Gerätesitzung im Cookie wird abgelehnt. Sonst bekäme es über
  `TouchSession` einen Ablauf von 30 Tagen und ein `Set-Cookie`.
- Für jede falsche Paarung gibt es einen Test.

### 4.2 Anmelden

Die App ruft nicht `login` auf, sondern eine neue Operation **«App anmelden»** (`public`; den
Namen legt der Bau fest). K26 4.4 hat dieselbe Operation als «Gerät mit Passwort anmelden»
vorgesehen; es ist eine Operation mit einer Angabe zur Art.

- **Hinein:** E-Mail, Passwort, Name des Geräts (zum Beispiel «Pixel 9»), Art (`phone`,
  `tablet` oder `tv`) und, wenn das Gerät schon eine hat, seine Gerätekennung.
- **Heraus:** das Token, die Gerätekennung und die Sitzung (`SessionInfo`).
- Die Art bestimmt die Sitzungsart: `phone` und `tablet` ergeben eine App-Sitzung, `tv` eine
  Gerätesitzung.
- Der Server legt die Gerätezeile **sofort** mit Name und Art an, nicht erst beim ersten
  Abspielstart. Das Gerät steht damit gleich unter «Meine Geräte». Der Name aus der Anmeldung
  bleibt; der Abspielstart überschreibt ihn bei App- und Gerätesitzungen nicht mehr (2.3).
- **Die Gerätekennung vergibt der Server.** Eine mitgebrachte Kennung nimmt er nur an, wenn
  kein **anderes** Konto auf dem Gerät eine lebende Sitzung oder Downloads hat (dieselbe Prüfung
  wie in 4.5) und sie nicht die reservierte ist (2.3); sonst vergibt er eine neue. Wer eine
  fremde Kennung kennt, könnte sich sonst an das fremde Gerät hängen, es umbenennen und dessen
  Downloads blockieren (`downloads` ist je Gerät und Titel eindeutig).
- Die Operation teilt sich den Topf für Fehlversuche mit `login` (2.3). Er zählt je Adresse des
  Aufrufers; Handys hinter demselben Mobilfunk-Ausgang teilen ihn sich.
- Der Abgleich des Passworts mit Seerr läuft wie bei `login` (2.3).

Das Token: 256 Bit Zufall, im Server nur als Hash gespeichert, wie heute das Sitzungstoken
(`hashToken`). Die App legt es verschlüsselt mit einem Schlüssel aus dem Android Keystore ab und
nimmt es von der Sicherung und der Übertragung auf ein neues Gerät aus (`dataExtractionRules`).
Ein neues Handy meldet sich neu an.

**Ehrlich gesagt** (wie K26 4.4): Welche Art ein Client nennt, entscheidet der Client. Wer das
Passwort kennt, kann sich mit jedem Programm über `login` auch voll anmelden. Die Sitzungsarten
begrenzen den Schaden eines **verlorenen Geräts oder Tokens**, nicht den eines bekannten
Passworts.

### 4.3 Der Bearer-Weg im Server

K26 4.2 nennt die Regeln; sie gelten für beide Bearer-Arten:

- Eine Anfrage mit Bearer bekommt nie ein `Set-Cookie`, weder für die Sitzung noch für das
  Gerät, und ein 401 löscht kein Cookie.
- Kommen Sitzungs-Cookie und Bearer zusammen, lehnt der Server ab.
- Der Kopf `X-Requested-With: Slouchcat` bleibt Pflicht für jede schreibende Anfrage.
- Auch die Streaming-Routen aus `binaryRoutes` (Bilder, Vorschaubilder, Radio-Bilder) nehmen das
  Token an. Medien-Adressen brauchen es nicht; sie tragen `st`, `dt` oder `rt`.

Dazu kommt, was Abschnitt 2.3 am Code zeigt und K26 noch nicht nennt:

- **Gerätekennung ohne Cookie.** Mit Bearer nimmt der Server das Gerät aus der Sitzung. Für
  Anfragen ohne Sitzung (`checkOfflineRevocations`, «App anmelden») sendet die App die Kennung in
  einem Kopf. Die Kennung ist wie das Cookie ein Geheimnis mit 128 Bit.
- **Sitzungen ohne Ablauf.** Die Tabelle bekommt eine Spalte für die Art. «Läuft nicht ab»
  heisst: ein Ablaufdatum weit in der Zukunft, kein leerer Wert. So gelten die Abfragen weiter,
  die `expires_at > @now` prüfen (2.3). `TouchSession` verschiebt den Ablauf bei diesen Arten
  nicht.
- **Abmelden mit Bearer** entzieht die Sitzung, setzt aber weder ein Cookie noch
  `Clear-Site-Data`; heute tut `logout` beides.
- **Die Aufräumung nach 400 Tagen nimmt Geräte mit einer lebenden Sitzung ohne Ablauf aus,**
  ebenso deren Einträge in der Widerrufsliste. Sonst geschähe, was 2.3 zeigt: Ein Handy ohne
  Kontakt seit 400 Tagen verlöre seine Downloads im Server und bekäme eine neue Kennung. Der
  nächste Abgleich fände unter der neuen Kennung keinen Widerruf, und die Dateien blieben auf
  dem Handy liegen, ohne dass der Server sie noch kennt. Für App- und Gerätesitzungen ist ein
  verschwundenes Gerät darum ein Fehler und kein Anlass für eine neue Kennung: Der Server
  entzieht die Sitzung und antwortet mit 401; das Gerät meldet sich neu an.

### 4.4 Die Erlaubnisliste im Vertrag

K26 4.5 schlägt je Operation eine zweite Angabe neben `x-visibility` vor, die sagt, ob eine
Gerätesitzung sie aufrufen darf. Mit zwei Bearer-Arten wird daraus eine Liste, Vorschlag
`x-clients` (den Namen legt der Bau fest):

- `x-clients: [app, tv]` nennt die Bearer-Arten, die die Operation aufrufen dürfen. Fehlt die
  Angabe, darf es keine.
- Der Schema-Test verlangt die Angabe für jede `account`- und `scoped`-Operation und verbietet
  sie für `admin` und `owner`. Jede neue Operation muss die Frage also beantworten.
- Die Middleware liest die Angabe zur Laufzeit aus dem Vertrag, wie heute `x-visibility`
  (`visibilityOf`). Für die Streaming-Routen steht sie in `binaryRoutes`.
- `app` steht an allen 24 `account`- und 92 `scoped`-Operationen von heute. `tv` steht an 93
  davon; K26 Anhang B zählt alle 116 je Operation auf (93 ja, 23 nein; von der Redaktion gegen
  den Vertrag nachgezählt).
- **Eingetragen wird `tv` in Stufe 1 dieses Konzepts,** nicht erst mit der TV-App. Stufe 1
  schafft die Gerätesitzung und den Test unten. Ohne die Angabe bekäme ein Fernseher auf alles
  403, auch auf `selectProfile`, und das Prüfprogramm, das einen Fernseher spielt, käme nicht
  bis zur Profilwahl.
- Was in Anhang B nicht Marcos Entscheid ist, sondern Vorschlag: die Gruppen Empfehlen,
  Benachrichtigungen, Ankündigungen und Vorlieben der Wiedergabe (ja) sowie Kontodaten,
  Sehbericht und Meldekanäle (nein, K26 F-18). F-18 wird damit vor Stufe 1 fällig.
- **Der Test:** Er ruft jede Operation mit einem App-Token und einem TV-Token des
  **Besitzerkontos mit aktivem Hauptprofil** auf. Wo die Art fehlt, erwartet er 403 mit einem
  eigenen Fehlercode (Vorschlag `client_not_allowed`). Mit einem gewöhnlichen Konto bewiese der
  Test nichts: Dort lehnt schon die Rollenprüfung ab.

Dieselbe Angabe sagt, welche Operationen unter die Verträglichkeitsregel fallen (11.5), und
welche der Kotlin-Client braucht (5.3).

### 4.5 Meine Geräte (F-15)

Zwei neue Operationen auf Kontoebene, dazu eine Seite in den Einstellungen der Web-App und der
Handy-App:

- **Liste:** alle Geräte, die eine lebende Sitzung **oder Downloads** des Kontos halten, mit
  Name, Art, Zeitpunkt der Anmeldung, letztem Kontakt und der Angabe «dieses Gerät». Massgeblich
  ist das Konto der Sitzung oder des Downloads, nicht `devices.account_id` (2.3). Ein Browser,
  dessen Sitzung abgelaufen ist, steht so weiter in der Liste, solange er Downloads hält.
- **Abmelden** räumt alles weg, was dieses Konto auf dem Gerät hat:
  - die Sitzungen und deren Streams
  - die Downloads; das Gerät erfährt es beim nächsten Kontakt über die Widerrufsliste und löscht
    die Dateien (AN-4)
  - **die Push-Abos des Geräts.** Heute überlebt ein Abo jede Abmeldung (2.5); ein verlorenes
    Handy bekäme sonst weiter jede Meldung des Kontos, auch «Neues Gerät verbunden».
  - die Regeln für Auto-Downloads auf diesem Gerät
- Dasselbe Aufräumen der Push-Abos gilt für `logout` einer App-Sitzung, für `deleteDevice` der
  Verwaltung und für eine Passwortänderung. Drei Einzelheiten dazu:
  - `deleteDevice` löscht die Abos **vor** der Gerätezeile. Danach ist die Kennung im Abo leer
    (2.5) und das Abo nicht mehr zu finden.
  - Das Aufräumen bei der Passwortänderung trifft nur Handys und Fernseher; die App trägt ihr
    Abo bei der nächsten Anmeldung von selbst neu ein. Für Browser bleibt es beim heutigen
    Verhalten, weil ein Browser sich nicht von selbst neu einträgt (Frage F-40).
  - Abos ohne Gerätekennung, die es heute schon gibt, räumt niemand auf. Das bleibt eine
    bekannte Grenze.
- Lesen darf jedes Profil, abmelden nur eine Sitzung ohne aktives Profil oder mit dem
  Hauptprofil. Das ist die Regel der Meldekanäle (2.5).
- Einer Gerätesitzung sind beide Operationen verwehrt (K26 4.5).

Der Unterschied zu `deleteDevice` der Verwaltung: Dort wird das Gerät als Ganzes vergessen, für
alle Konten.

### 4.6 Bekannte Grenzen

- **E-15 und E-18 zusammen:** Wer die App über unverschlüsseltes HTTP benutzt, sendet Passwort
  und Token offen. Wer auf dem Weg mitliest, hat ein Token, das gilt, bis jemand das Gerät unter
  «Meine Geräte» abmeldet oder das Passwort ändert (Frage F-24). Die Meldung «Neues Gerät» aus
  K-7 schlägt dabei nicht an, weil kein neues Gerät entsteht. Abschnitt 6 nennt, was die App
  dagegen tun kann: warnen und anzeigen.
- **Fremdes Netz mit denselben Adressen (Befund des Kritikers, Frage F-38):** Ein Server unter
  `http://192.168.0.10` gilt als Heimnetz und löst keine Warnung aus. Ist das Handy in einem
  fremden WLAN mit demselben Adressbereich, schickt die App ihr Token samt jeder Bildanfrage an
  das Gerät, das dort diese Adresse hat. Dafür muss niemand mitlesen und der Nutzer nichts
  antippen. Dasselbe gilt für die Medien-Adressen mit `dt`, `st` und `rt`, die der Abruf im
  Hintergrund gerade nach einem Netzwechsel anfragt. Diese Folge stand nicht in der Frage zu
  E-18. Abschnitt 6 schlägt vor, dass der Server sich ausweist, bevor die App irgendetwas
  Geheimes sendet.
- Die Gerätekennung für die Widerrufsliste (4.3) reist über HTTP ebenfalls offen. Mit der Regel
  aus 4.2 gibt sie allein keinen Zugang.
- **Verlorenes Handy:** Es behält den Zugang zum Nutzerbereich aller Profile ohne PIN. Schutz
  bieten die Bildschirmsperre des Handys, die PINs der Profile und «Meine Geräte».
- **E-9 gilt auch hier:** Hat das Hauptprofil keine PIN, kann jeder mit dem entsperrten Handy
  Profile ändern und PINs entfernen. Im Browser ist das heute genauso. Ob die Handy-App und die
  Web-App dabei dieselbe PIN-Empfehlung zeigen wie der Fernseher, ist Frage F-19 aus K26.

## 5. Das gemeinsame Fundament `android/`

### 5.1 Platz im Repo (Frage F-1)

Vorschlag wie K26 3.2: ein Ordner `android/` im bestehenden Repo. Vertrag, Texte und Tokens
liegen schon hier, und ein Pull Request kann Server und App gemeinsam ändern.

```
android/
  app-mobile/        die Handy-App (dieses Konzept)
  app-tv/            die TV-App (Konzept 26, später)
  core/api/          erzeugter Kotlin-Client, Anmeldung, Netz
  core/session/      Serveradresse, Token, Profilwahl, Verträglichkeit
  core/player/       Media3: Fähigkeiten, Abspielstart, Fortschritt, Spuren
  core/design/       erzeugte Farben und Masse, gemeinsame Bauteile
  core/strings/      erzeugte Texte
  core/pairing/      Paket für die Kopplung, Verschlüsselung
  tools/             Generatoren für Texte und Tokens
relay/               die zwei Dienste im Internet (Go, eigenes Modul, zwei Programme)
```

Beide Apps tragen denselben Paketnamen (E-20) und sind zwei Artefakte mit eigenen
Versionsnummern. Oberflächen teilen sie nicht: Die TV-App baut ihre Seiten mit «Compose for TV»
(K26 3.1), die Handy-App mit Material 3.

### 5.2 Bausteine

| Teil | Baustein | Grund |
|---|---|---|
| Oberfläche | Jetpack Compose, Material 3, «Material 3 Adaptive» | von Google empfohlen, deckt Handy und Tablet |
| Video und Ton | AndroidX Media3 (ExoPlayer, `MediaSessionService`) | spielt MKV, MP4, HLS und lokale fMP4-Dateien; Ton im Hintergrund |
| Netz | OkHttp, ein Client für API, Bilder und Player | ein Ort für Token, CSRF-Kopf und die Regel aus E-18 |
| API | aus `api/openapi.yaml` erzeugter Kotlin-Client | der Vertrag bleibt die einzige Quelle |
| Bilder | Coil über denselben OkHttp-Client | Bilder brauchen die Sitzung |
| Ablage | Room (Downloads, Ereignisse, Profile offline), DataStore (Einstellungen) | entspricht der Offline-Datenbank der PWA |
| Hintergrund | Aufträge des Systems für Abruf, WorkManager für den Abgleich | Abschnitt 9.3 |
| QR-Scanner | «Google code scanner» | keine Kamera-Berechtigung nötig |
| Push | Firebase Cloud Messaging | E-17 |

Kein FFmpeg in der App (Frage F-12, Empfehlung aus K26): Was das Gerät nicht selbst abspielt,
rechnet der Server um. Die App bleibt damit ohne eigenen C-Code; das erspart die Pflicht zu
16-KB-Speicherseiten.

**Zwei Bauarten (Frage F-33).** Push und QR-Scanner hängen an Bibliotheken von Google. Vorschlag:
eine Bauart `play` mit diesen Bibliotheken und eine Bauart `foss` ohne sie (kein Push, Scanner
mit Kamera-Berechtigung). Veröffentlicht wird nur `play`. Die Bauart `foss` hält die App baubar
für jeden, der das Repo klont, und lässt die Frage offen, ob eine AGPL-App Googles Bibliotheken
einbinden darf, ohne dass der Bau daran hängt. Eine Regel von Google Play gegen Copyleft-Apps war
nicht zu finden (unbelegt).

### 5.3 Der Kotlin-Client

- Der Vertrag hat 24'629 Zeilen in OpenAPI 3.1 mit `oneOf` und `nullable`. Welcher Generator ihn
  trägt, entscheidet Stufe 0 nach einem Versuch. Bekannt ist: Der verbreitete Generator
  unterstützt `oneOf` nur mit Diskriminator.
- Ein kleiner Vorlauf schneidet den Vertrag auf die Operationen mit `x-clients` zu (4.4), bevor
  der Generator läuft. Die App braucht die 116 `account`- und `scoped`-Operationen von heute,
  einige öffentliche (`getHealth`, `checkOfflineRevocations`) und die neuen; die 183 Operationen
  der Verwaltung fallen weg.
- **Erzeugter Code wird nicht eingecheckt** (K26 3.2). Client, Farben und Texte entstehen beim
  Bau im Bauordner. Regel 1 bekommt keine neuen Verzeichnisse, `task gen` und `gen:check` bleiben,
  wie sie sind, und wer nur Server oder Web-App ändert, braucht kein JDK. Bricht eine
  Vertragsänderung die App, zeigt es der Bau der App in der CI.

### 5.4 Texte (Regel 9)

- Quelle bleiben `web/src/locales/*.json`. Ein Generator in `android/tools/` schreibt daraus beim
  Bau `strings.xml` je Sprache.
- Er nimmt nur die Namensräume, die eine Liste in `android/` nennt. Alle 5'700 Texte in 22
  Sprachen wären rund 125'000 Einträge, die meisten davon für die Verwaltung.
- Texte, die nur die Apps brauchen, kommen unter einem eigenen Namensraum in dieselben Dateien.
  Der Übersetzungsweg (ADR 0033, `docs/i18n/README.md`) bleibt einer.
- Der Generator überträgt Mehrzahlformen nach `<plurals>`, Platzhalter `{{name}}` nach `%1$s`,
  `de-DE` nach `values-de-rDE` und Sprachen mit Region nach `values-pt-rBR`. Trifft er auf etwas,
  das er nicht kennt (verschachtelte Verweise `$t()`, Auszeichnung in Texten), bricht er ab und
  nennt den Schlüssel.
- Die Form `_zero` braucht einen eigenen Text: Android wertet die Mehrzahlform für null in
  Deutsch und Englisch nicht aus (Kenntnis des Kritikers, unbelegt; Stufe 0 prüft es am
  Generator).
- Die Texte des Servers für Meldungen (`server/internal/notify/locales/`) braucht die App nicht;
  Push-Meldungen kommen fertig formuliert.

### 5.5 Farben und Masse (Regel 9)

`packages/design-tokens` bekommt ein drittes Ausgabeformat für Kotlin. Es rechnet `rem` in `dp`
und `sp` um (1 rem = 16) und `px` in `dp` (1 zu 1), dreht Farben mit Deckkraft von `#RRGGBBAA`
in die Reihenfolge von Android und lässt weg, was nur das Web kennt (`em`). Die Kotlin-Ausgabe
entsteht beim Bau der App, nicht bei `task gen`.

### 5.6 Wächter

- Ein Lint-Lauf meldet festen Text und Farbwerte ausserhalb des erzeugten Codes als Fehler. Ob
  die mitgelieferte Prüfung von Android das für Compose leistet, ist unbelegt; sonst braucht es
  eine eigene Lint-Regel.
- Regel 12: Die App entscheidet nichts über Sichtbarkeit oder Alter. Sie zeigt, was der Server
  liefert; nicht sichtbar heisst 404. Für Offline gilt Abschnitt 9.5.
- Regel 10 gilt sinngemäss: Token, Gutschein und Gerätekennung erscheinen in keinem Log der App.

### 5.7 Werkzeuge und CI

- JDK 17 und Android-SDK ohne sudo unter `~/.local`, Gradle über den Wrapper im Repo, Versionen
  gepinnt wie bei Go und Node (`.tool-versions`, Versionskatalog von Gradle).
- `task android:check` (Bau, Lint, Generatoren) und `task android:test`; `task ci` ruft beide
  auf. Gradle-Läufe warten wie die anderen schweren Läufe auf einen Platz
  (`scripts/heavy-slot.sh`).
- `relay/` ist ein zweites Go-Modul mit `task relay:check` und `task relay:test`. Die Regeln für
  Go gelten dort gleich (golangci-lint, `slog` mit Redaktion). Das Paket
  `server/internal/logging` kann ein zweites Modul nicht einbinden (Go sperrt `internal` nach
  aussen); die Dienste bekommen eine eigene kleine Fassung des Redaktions-Handlers.
- CI: zwei neue Jobs `android` und `relay`, beide in der `needs`-Liste des Gates `ci` und in der
  Regel, nach der `main` schon Geprüftes überspringt (2.6).
  GitHub-Runner können einen Android-Emulator mit Hardware-Beschleunigung starten; ob die
  Oberflächentests dort laufen und wie lange, misst Stufe 3 und schreibt es nach `docs/perf/`.
  Danach entscheidet sich, ob `android/` und `relay/` eigene Pfadfilter bekommen. Heute liefe
  jeder Pull Request an der App durch alle Jobs des Servers und umgekehrt.
- **Releases:** Tags `app-v1.0.0` für die Handy-App, `tv-v1.0.0` für die TV-App (K26),
  `relay-v1.0.0` für die zwei Dienste. Keiner trifft das Release-Workflow des Servers
  (`v*.*.*`), und keiner berührt die Regel «Vertrag = Tag». Die zwei Images der Dienste baut
  ein eigenes Release-Workflow; heute gibt es nur das des Servers.
- **Lizenz:** Das Repo steht unter AGPL-3.0, die App und die Dienste damit auch. Der
  Store-Eintrag verweist auf den Quellcode.

## 6. Serveradresse und unverschlüsseltes HTTP (E-18)

- Die App fragt beim ersten Start nach der Adresse, prüft sie mit `getHealth` und zeigt Adresse,
  Version und ob die Verbindung verschlüsselt ist.
- **HTTP geht überall (E-18).** Android kann unverschlüsseltes HTTP nur für alles oder für
  benannte Namen freigeben (2.7); die App gibt es darum für alles frei. Die Regel sitzt im
  gemeinsamen OkHttp-Client, den auch Bilder und Player benutzen.
- **Die Warnung** erscheint beim Eintragen einer `http://`-Adresse, die nicht im Heimnetz liegt,
  und verlangt eine Bestätigung. Vorschlag für den Text: «Diese Verbindung ist nicht
  verschlüsselt. In fremden Netzen kann jemand dein Passwort mitlesen und deinen Zugang
  übernehmen.»
- Vorschlag darüber hinaus: Solange die Verbindung unverschlüsselt ist, zeigt die Seite «Mehr»
  das dauerhaft an, auch im Heimnetz. Ob eine Adresse im Heimnetz liegt, entscheidet die App an
  der aufgelösten Adresse (die Bereiche aus 2.7).
- **Der Server weist sich aus (Vorschlag, Frage F-38).** Gegen das fremde Netz mit denselben
  Adressen (4.6) hilft keine Warnung. Vorschlag für Server über HTTP:
  - Der Server bekommt ein eigenes Schlüsselpaar und nennt den öffentlichen Schlüssel in
    `getHealth`. Die App merkt ihn sich beim Einrichten (Vertrauen beim ersten Kontakt).
  - In jedem neuen Netz schickt die App zuerst eine Zufallszahl, und der Server unterschreibt
    sie. Passt die Unterschrift nicht zum gemerkten Schlüssel, sendet die App nichts und meldet
    «Server nicht erreichbar».
  - **Die Sperre sitzt im gemeinsamen OkHttp-Client und gilt je Netz.** Sie hält jede Anfrage
    zurück, bis das Netz geprüft ist: API, Bilder, Player und Abruf im Hintergrund. Damit
    schützt sie alles, was geheim ist, nicht nur das Token: die Medien-Adressen mit `dt`, `st`
    und `rt`, die Gerätekennung für die Widerrufsliste und das Passwort bei einer neuen
    Anmeldung.
  - Der Schlüssel kann im Paket der Kopplung mitreisen (7.2); der Fernseher prüft dann von
    Anfang an.
  - Im Server kostet das eine öffentliche Operation mit Ratenbegrenzung.
- **Wechselt der Schlüssel des Servers,** erkennt die App ihren Server nicht mehr. Das geschieht
  nach einer Neuinstallation und nach einer Wiederherstellung aus einer Sicherung ohne
  Geheimnisse: Der Schlüssel liegt beim Geheimnis des Servers (`secret.key`), und das nimmt eine
  Sicherung nur auf Wunsch mit. Die App meldet dann nicht «Server nicht erreichbar», sondern «Der
  Server hat einen neuen Schlüssel». Nach einer neuen Anmeldung mit dem Passwort bestätigt der
  Nutzer den neuen Schlüssel; die App sagt dazu, dass er das nur im eigenen Netz tun soll.
- **Was das nicht leistet:**
  - Es schützt nicht vor jemandem, der im richtigen Netz auf dem Weg mitliest. Diese Folge von
    E-18 bleibt.
  - Ist der echte Server auch aus dem Internet erreichbar, kann ein fremdes Gerät die Prüfung an
    ihn weiterreichen, bestehen und danach mitlesen. Dagegen hilft nur HTTPS.
  - Wer eine Sicherung des Servers mit dem privaten Schlüssel erbeutet, kann sich als der Server
    ausgeben.
- Eine kleinere Fassung ohne Schlüsselpaar stand in Fassung 2: Der Server beweist, dass er den
  gespeicherten Hash des Tokens kennt. Sie deckt weder die neue Anmeldung noch die
  Widerrufsliste und ist darum nicht mehr der Vorschlag.
- Verworfen: das WLAN wiederzuerkennen. Den Namen des WLAN und den Zugangspunkt gibt Android nur
  mit der Standort-Berechtigung her, die Hardware-Adresse der Gegenstelle seit Android 10 gar
  nicht (Kenntnis der Redaktion und des Kritikers, unbelegt).
- **Heimnetz und Android 17:** Sobald die App auf API 37 zielt, braucht sie die Berechtigung
  `ACCESS_LOCAL_NETWORK`, um einen Server im Heimnetz zu erreichen. Die App erklärt sie vor der
  Frage des Systems und erkennt den Fall «verweigert» (Timeout bei einer lokalen Adresse), statt
  «Server nicht erreichbar» zu melden. Die erste Ausgabe zielt auf API 36, wie Google es
  verlangt; die Berechtigung wird mit dem Wechsel auf 37 fällig.
- Die zwei Dienste aus Abschnitt 7 sind davon getrennt: Beide sind immer über HTTPS erreichbar.

## 7. Die zwei Dienste im Internet: `relay/` (E-11, E-17, E-19, E-21, E-22)

Marco betreibt für alle Slouchcat-Nutzer **zwei Dienste**, getrennt voneinander und je auf einer
eigenen Maschine (E-21):

| Dienst | Wofür | Hält an Geheimnissen | Entscheid |
|---|---|---|---|
| **Vermittlung** | Handy und Fernseher finden beim Einrichten per Scan zueinander | nichts | E-11 |
| **Push-Weiterleitung** | Meldungen der Slouchcat-Server gehen an Googles Push-Dienst | Firebase-Schlüssel, Schlüssel zum Versiegeln (7.3) | E-17 |

Entschieden ist der getrennte Betrieb. **Nicht entschieden** ist, ob das zwei Programme sind
oder ein Programm, das je Maschine nur einen Teil einschaltet (Frage F-39). Vorschlag: **ein
Go-Modul `relay/` im selben Repo mit zwei Programmen und zwei Images,** offen wie der Rest.

- Gemeinsam ist, was beide brauchen: die Grenzen je Aufrufer, die Adresse des Aufrufers hinter
  einem Proxy, Logs mit Redaktion, die Erreichbarkeitsprüfung.
- Getrennt ist alles andere. Das Image der Vermittlung enthält keinen Code, der mit dem
  Firebase-Schlüssel umgeht, und lässt sich nicht versehentlich mit Push starten.
- Bis Fassung 4 stand hier ein Programm mit zwei schaltbaren Teilen, begründet mit «eine
  Installation, ein Update». Mit zwei Maschinen fällt dieser Grund weg.

Jeder Dienst hat einen eigenen festen Namen (7.5). Wer nur die Vermittlung selbst betreiben
will, braucht die Push-Weiterleitung nicht. In 7.1 und 7.2 heisst «der Dienst» die Vermittlung,
in 7.3 die Push-Weiterleitung.

### 7.1 Vermittlung: das Protokoll

Der Ablauf für den Nutzer steht in K26 4.3. Dies ist die Schnittstelle des Dienstes (Vorschlag,
Pfade unter `/v1`):

| Schritt | Wer | Anfrage | Antwort |
|---|---|---|---|
| Treffpunkt anlegen | Fernseher | `POST /v1/pair` mit dem Hash eines Abholgeheimnisses | Kennung, Ablaufzeit |
| Paket ablegen | Handy | `PUT /v1/pair/{Kennung}` mit dem verschlüsselten Paket | 204; 409, wenn schon eines liegt; 410, wenn schon eines abgeholt wurde; 404, wenn die Kennung unbekannt oder abgelaufen ist |
| Paket abholen | Fernseher | `GET /v1/pair/{Kennung}` mit dem Abholgeheimnis | 200 mit dem Paket, danach ist der Treffpunkt verbraucht; 204, solange keines liegt |
| Abbrechen | Fernseher | `DELETE /v1/pair/{Kennung}` mit dem Abholgeheimnis | 204 |
| Erreichbarkeit | Überwachung | `GET /v1/healthz` | 200 |

- Die Kennung hat 128 Bit Zufall und gilt 5 Minuten (K-1). Sie steht im QR-Code.
- **Neu gegenüber K26:** Abholen kann nur, wer das **Abholgeheimnis** kennt. Es verlässt den
  Fernseher nie; im QR-Code steht es nicht. Wer den QR-Code fotografiert, kann das Paket darum
  weder abholen noch den Treffpunkt vor dem Fernseher verbrauchen.
- Das Paket ist höchstens 4 KB gross. Der Dienst hält es nur im Arbeitsspeicher.
- Nach dem Abholen merkt sich der Dienst bis zum Ablauf nur noch, dass der Treffpunkt verbraucht
  ist. So kann er dem Handy «schon abgeholt» antworten statt «unbekannt»; die Handy-App braucht
  den Unterschied für ihre Warnung (Abschnitt 10).
- Grenzen: Obergrenze für offene Treffpunkte gesamt und je Aufrufer-Adresse, Ratenbegrenzung je
  Adresse (K-4).
- Der Fernseher fragt alle 2 Sekunden nach oder hält die Anfrage offen; das legt der Bau fest.

### 7.2 Vermittlung: die Verschlüsselung

- Der Fernseher erzeugt beim Anzeigen des QR-Codes ein Schlüsselpaar, das nur für diese eine
  Kopplung lebt.
- **Der QR-Code** enthält: die Adresse des Dienstes, die Kennung, den öffentlichen Schlüssel des
  Fernsehers und eine Fassungsnummer. Er ist eine `https`-Adresse auf eine Hilfeseite, die Angaben
  stehen hinter dem `#`. Wer ihn mit der Kamera-App scannt, landet auf «Öffne die Slouchcat-App
  und wähle Fernseher verbinden»; der Teil hinter dem `#` erreicht dabei keinen Server. Die
  Hilfeseite lädt keine fremden Skripte und trägt eine strenge Inhaltsrichtlinie (CSP), weil ein
  Skript auf der Seite den Teil hinter dem `#` lesen könnte.
- **Die Handy-App nimmt den Code nur aus ihrem eigenen Scanner an,** nie als Link von aussen.
  Sonst würde aus «Bild scannen» ein Antippen (K26 4.6, bekannte Grenze).
- **Das Paket** enthält die Serveradresse, den Kopplungsgutschein und die Fassungsnummer, dazu
  den öffentlichen Schlüssel des Servers, falls F-38 mit Ja beantwortet wird (Abschnitt 6). Die
  Handy-App verschlüsselt es mit HPKE (RFC 9180) für den Schlüssel des Fernsehers; die Kennung
  des Treffpunkts geht als gebundene Zusatzangabe ein, damit ein Paket nicht an einen anderen
  Treffpunkt passt.
- **Einlösen:** Der Fernseher schickt Gutschein, öffentlichen Schlüssel, seinen Gerätenamen und
  die Fassung seines Systems direkt an den Server des Nutzers; die Art `tv` steckt im Gutschein.
  Ohne Namen hätte die Gerätezeile nichts für «Meine Geräte» und für die Meldung aus K-7. Der
  Server prüft, dass der Gutschein an genau diesen Schlüssel gebunden ist (K-3),
  und antwortet mit dem Gerätetoken, **verschlüsselt für diesen Schlüssel**. Lesen kann es nur,
  wer den privaten Schlüssel hat. Das ersetzt den «Schlüsselnachweis» aus K26 4.3 Schritt 6 ohne
  zweite Anfrage und ohne zweites Schlüsselpaar zum Unterschreiben. Läuft der Server ohne TLS,
  schützt es das Token in dieser einen Antwort; jede spätere Anfrage trägt es offen (4.6).
- Dieselbe Verschlüsselung braucht es an drei Orten: Handy-App (verschlüsseln), TV-App
  (entschlüsseln), Server (verschlüsseln). In Go steht HPKE in der Standardbibliothek; für
  Android prüft Stufe 0 die Bibliothek. **Beleg:** ein gemeinsamer Testvektor, den Go und Kotlin
  in der CI beide lesen.
- **Untergeschobenes Paket (bekannte Grenze).** Ablegen kann jeder, der die Kennung kennt: wer
  den QR-Code fotografiert, und der Dienst selbst. Er kann dem Fernseher ein eigenes Paket
  unterschieben, bevor das echte Handy seines ablegt, und ihn so mit einem **fremden** Server
  und Konto verbinden. Lesen kann er dabei nichts. Der Fernseher zeigt darum nach dem Einlösen
  bei der Profilwahl die Serveradresse und das Konto; verhindern lässt es sich nicht, nur
  sichtbar machen.

Der Dienst sieht vom Paket nur Geheimtext. Er kennt weder die Serveradresse noch den Gutschein
und spricht nie mit dem Server des Nutzers.

### 7.3 Push-Weiterleitung (E-17)

Googles Push-Dienst nimmt Nachrichten nur von einem Absender an, der den Schlüssel des
Firebase-Projekts der App hat (2.7). Dieser Schlüssel kann nicht in jedem selbst gehosteten
Server liegen. Darum steht ein Dienst dazwischen.

**Vorschlag: Der Dienst spricht zum Server hin Web-Push.** Der Server hat Web-Push schon (2.5)
und braucht dann keinen neuen Sendeweg, wohl aber vier Anpassungen (unten).

1. Die App holt beim Start ihr Google-Token und erzeugt ein Schlüsselpaar für Web-Push.
2. Die App meldet sich beim Dienst an: Google-Token und der öffentliche VAPID-Schlüssel ihres
   Servers (`getPushPublicKey`) hinein, eine **Push-Adresse** heraus. Die Adresse enthält das
   Google-Token und den VAPID-Schlüssel, vom Dienst versiegelt, und nennt die Fassung des
   Schlüssels, mit dem versiegelt wurde. Der Dienst braucht dafür keine Datenbank.
3. Die App trägt die Push-Adresse mit ihrem Web-Push-Schlüssel beim Server ein
   (`createPushSubscription`).
4. Der Server schickt eine Meldung wie an einen Browser: verschlüsselt für den Schlüssel der App
   (RFC 8291), mit VAPID unterschrieben.
5. Der Dienst öffnet die Adresse und prüft die VAPID-Unterschrift: gegen den versiegelten
   Schlüssel, auf den Empfänger (`aud` ist die eigene Adresse des Dienstes) und auf den Ablauf.
   Dann reicht er den Geheimtext als Datennachricht an Google weiter und übersetzt dabei
   Haltedauer und Dringlichkeit.
6. Die App entschlüsselt und zeigt die Meldung an. Ein Tippen öffnet das Ziel: Die App nimmt
   aus `url` nur den Pfad und übersetzt ihn in ihre eigene Seite; einen fremden Rechnernamen
   öffnet sie nie. Sieht das aktive Profil den Titel nicht, antwortet der Server mit 404 wie
   sonst. Ziele der Verwaltung (`/admin…`, 2.5) öffnet die App im Browser, unter der Adresse
   ihres eigenen Servers.

**Was der Server dafür ändern muss:**

- **Satzgrösse.** Jeder Körper ist heute genau 4096 Byte gross (2.5). Google erlaubt 4096 Byte
  je Nachricht, und der Geheimtext wächst beim Umkodieren um ein Drittel. Jede Meldung wäre also
  zu gross, nicht nur eine lange. Der Server setzt für Adressen des Dienstes eine kleinere
  Satzgrösse und kürzt den Text entsprechend (`maxBody` ist heute 4000 Zeichen). Den Wert misst
  Stufe 0.
- **Der Server erkennt Adressen des Dienstes** am Ursprung: Die Adresse des Dienstes ist im
  Server eingebaut und einstellbar. Nur für sie gilt die kleinere Satzgrösse.
- **Eine App-Sitzung verwaltet ihr eigenes Abo immer:** anlegen, erneuern, löschen. Heute
  verlangt schon das erste Eintragen das Hauptprofil oder kein Profil (2.5).
- **Ein neues Google-Token ersetzt das alte Abo des Geräts,** bei App-Sitzungen. Sonst bleibt
  die alte Zeile stehen, weil die Adresse eine andere ist.
- **Abmelden löscht das Abo** (4.5).

**Was gleich bleibt:** Ob das Konto überhaupt Push bekommt, entscheidet weiter das Hauptprofil
über den Kanal (2.5). Die App trägt ihr Abo immer ein; ist der Kanal aus, zeigt sie das an und
sagt, dass das Hauptprofil ihn einschalten muss. Ein Handy, auf dem nur ein Kinderprofil benutzt
wird, bekommt sonst ein Abo und trotzdem keine Meldung.

Die App prüft beim Start und beim Abgleich im Hintergrund (9.6), ob ihr Abo noch besteht und der
VAPID-Schlüssel des Servers noch derselbe ist; ein neuer VAPID-Schlüssel löscht im Server alle
Abos (2.5), und die App trägt sich dann neu ein. Nur beim Start zu prüfen reicht nicht: Android
startet eine App unter Umständen tagelang nicht neu.

Was das bringt:

- Der Dienst und Google sehen **nur Geheimtext**. Titel und Text einer Meldung liest allein die
  App.
- Nur der Server, dessen VAPID-Schlüssel in der Adresse steckt, kann an dieses Handy senden.
- Meldet Google «Token unbekannt», antwortet der Dienst mit 410, und der Server löscht das Abo
  von selbst (2.5). **Nur dann.** Kann der Dienst eine Adresse nicht öffnen (fehlender oder
  falscher Schlüssel, unbekannte Fassung), antwortet er mit 503. Der Server versucht es dann
  erneut (`deliverAttempts`, fünf Versuche), statt das Abo zu löschen. Sonst löschte eine einzige
  falsche Schlüsseldatei beim Dienst binnen Minuten die Abos aller Server, die gerade senden
  (D-10).
- Die Fassung in der Adresse erlaubt später, den Schlüssel zum Versiegeln zu wechseln, ohne
  dass alle Adressen auf einmal ungültig werden.
- Die bestehende Regel bleibt: Push gehört zum Konto. Eine Meldung geht an alle Geräte des Kontos,
  mit dem Profilnamen vorangestellt, auch wenn auf dem Handy gerade ein anderes Profil aktiv ist.
  Das ist heute im Browser genauso und steht hier als bekannte Grenze.

Was offen ist und Stufe 0 misst:

- **Die Satzgrösse,** mit der eine Meldung sicher durch Googles Grenze passt.
- **Zustellung im Stromsparmodus.** Der Server sendet mit normaler Dringlichkeit; Google kann
  solche Nachrichten verzögern, bis das Handy aufwacht. Für «neue Folge» reicht das; ob und wann
  der Dienst hohe Dringlichkeit setzt, legt der Bau fest.
- **Das Verhalten von Googles Bibliothek** bei Nachrichten, die nicht von der App verarbeitet
  werden (7.4).

Was Marco dafür anlegen muss: ein Firebase-Projekt auf dem Google-Konto des Projekts. Der
Dienstschlüssel dieses Projekts liegt nur beim Dienst, nie im Repo. Die Konfigurationsdatei der
App für Firebase gilt nach Googles Angaben nicht als geheim; Vorschlag ist trotzdem, sie beim Bau
einzuspielen statt einzuchecken.

### 7.4 Regeln für die Dienste

| Nr. | Regel | Schützt gegen |
|---|---|---|
| D-1 | Keine Konten, keine Datenbank, kein Zustand auf der Platte. Treffpunkte leben im Arbeitsspeicher, höchstens 5 Minuten; Push-Nachrichten werden durchgereicht und nicht gespeichert | Datenabfluss bei einem Einbruch; macht den Umzug leicht (7.5) |
| D-2 | Grössengrenze je Paket und je Push, Ratenbegrenzung je Aufrufer-Adresse und je Push-Adresse, Obergrenze für offene Treffpunkte. Aufrufer mit IPv6 zählen je Netz (/64), nicht je Adresse | Missbrauch als Speicher oder Versandweg, Überfluten |
| D-3 | Abholen nur mit Abholgeheimnis (7.1) | Abgreifen oder Verbrauchen eines Treffpunkts |
| D-4 | Push nur mit gültiger VAPID-Unterschrift des Servers, der in der Adresse steht, ausgestellt für diesen Dienst und nicht abgelaufen | fremde Meldungen an ein Handy, Wiederverwendung einer Unterschrift |
| D-5 | Die Dienste schreiben keine Adressen von Aufrufern in Logs; für die Ratenbegrenzung halten sie sie nur im Arbeitsspeicher. Dasselbe gilt für den Proxy davor (7.5) | Bewegungsprofile |
| D-6 | Der Firebase-Schlüssel und der Schlüssel zum Versiegeln liegen nur bei der Push-Weiterleitung, als Dateien ausserhalb des Images und in der Redaktionsliste der Logs. Die Vermittlung hat kein Geheimnis | Geheimnisse in Logs und Images |
| D-7 | Nach aussen nur HTTPS mit gültigem Zertifikat. TLS nimmt der Proxy vor dem Dienst ab (7.5) | Mitlesen und Verändern auf dem Weg |
| D-8 | Eigene Fassungsnummer in den Pfaden (`/v1`); alte Apps bleiben bedient, bis sie niemand mehr benutzt | Bruch installierter Apps |
| D-9 | Die Adresse des Aufrufers nimmt ein Dienst aus dem Weiterleitungskopf, aber nur, wenn die Anfrage von einer eingetragenen Proxy-Adresse kommt (7.5) | Grenzen, die alle Anfragen auf den Proxy zählen; Aufrufer, die sich eine Adresse aussuchen |
| D-10 | Die Push-Weiterleitung antwortet nur dann mit 410, wenn Google das Token nicht mehr kennt. Jeder eigene Fehler ist 503 (7.3) | Massenlöschung von Abos durch einen Fehler beim Dienst |

**Was ein Einbruch anrichten kann,** je Dienst:

- **Vermittlung:** Der Angreifer kann Treffpunkte stören und einem Fernseher beim Einrichten ein
  eigenes Paket unterschieben (7.2). An Push kommt er nicht: Der Firebase-Schlüssel liegt auf
  der anderen Maschine. Das ist der Gewinn aus E-21.
- **Push-Weiterleitung:** Der Angreifer kann Meldungen unterdrücken und mit dem
  Firebase-Schlüssel über Googles Push-Dienst **Nachrichten an alle Handys mit der App**
  schicken.

- **Der Proxy vor den Diensten (7.5):** Er nimmt TLS für beide Namen ab. Wer ihn übernimmt, kann
  dasselbe wie bei der Vermittlung, dazu Meldungen unterdrücken und Google-Token und Adressen
  von Servern mitlesen. An den Firebase-Schlüssel kommt er nicht, und eine versiegelte
  Push-Adresse kann er nicht fälschen. Auf eigener Hardware teilt dieser Proxy sein Schicksal
  mit allem, was sonst hinter ihm steht.

In keinem Fall kann er eine Meldung lesen, einen Slouchcat-Server erreichen oder ein Konto
übernehmen. Bei den Nachrichten sind zwei Fälle zu unterscheiden:

- Eine Datennachricht verarbeitet die App selbst. Sie zeigt nur an, was sich mit ihrem
  Web-Push-Schlüssel entschlüsseln lässt; das dafür nötige Geheimnis kennt nur ihr eigener
  Server, nicht der Dienst. Eine gefälschte Datennachricht verwirft sie.
- Googles Bibliothek kann eine «Benachrichtigungs-Nachricht» selbst anzeigen, ohne die App zu
  fragen, solange die App im Hintergrund ist (Kenntnis der Redaktion, unbelegt; Stufe 0 prüft
  es). So käme ein irreführender Text auf den Bildschirm. Die App folgt darum nie einem Ziel aus
  einer Meldung, die sie nicht selbst entschlüsselt hat; ein Tippen öffnet dann nur die App.

### 7.5 Betrieb und Umzug (E-19, E-21, E-22)

Marco betreibt die zwei Dienste vorläufig je auf einem eigenen Container auf eigener Hardware
und später je auf einem eigenen Mietserver (E-19, E-21). Solange sie auf eigener Hardware
laufen, sind sie über den Reverse-Proxy erreichbar, der dort schon steht (E-22). Die Container
entstehen mit dem Bau der Dienste in Stufe 2, nicht vorher. Alles Weitere in diesem Abschnitt
ist Vorschlag der Redaktion.

**Anforderung aus E-21: Der Umzug besteht nur aus Starten und Umschalten.** Dafür gilt je
Dienst:

- **Dasselbe Image und dieselbe Compose-Datei an beiden Orten.** Was sich unterscheidet, steht
  in einer Umgebungsdatei und einem Compose-Profil, nirgends sonst. Das Muster gibt es im Repo
  schon: Der Server hat das Profil `https` mit eigenem TLS-Abschluss und die Einstellung
  `SLOUCHCAT_TRUSTED_PROXIES` (`deploy/compose.yaml`). Anders als beim Server steht auch in der
  Umgebungsdatei, an welche Adresse der Dienst seinen Port bindet; der Server braucht dafür
  heute eine zweite Compose-Datei (`compose.https-only.yaml`).
- **Kein Zustand, der umziehen muss (D-1).** Die Vermittlung hält alles im Arbeitsspeicher. Die
  Push-Weiterleitung hat nur ihre zwei Geheimnisse als Dateien. Ausser diesen Dateien gibt es
  nichts zu sichern und nichts zu übertragen. Auf dem Mietserver legt der TLS-Abschluss seine
  Zertifikate auf die Platte; sie lassen sich neu ausstellen und ziehen nicht um.
- **Ein fester Name je Dienst** unter `slouchcat.com`, von Anfang an mit kurzer Lebensdauer im
  DNS (Frage F-37). Apps und Server tragen die Namen eingebaut. Beim Umzug ändert sich nur der
  DNS-Eintrag.
- **Die zwei Dienste ziehen unabhängig voneinander um.**

**Die zwei Orte:**

| | Auf eigener Hardware (vorläufig) | Auf dem Mietserver (später) |
|---|---|---|
| TLS | nimmt der vorhandene Reverse-Proxy ab und reicht an den Container weiter (E-22) | ein Compose-Profil startet einen eigenen TLS-Abschluss im selben Stack, mit eigenem Zertifikat |
| Adresse des Aufrufers (D-9) | aus dem Weiterleitungskopf, wenn die Anfrage von der eingetragenen Adresse des Reverse-Proxys kommt; eingetragen ist diese eine Adresse, kein ganzer Bereich | ebenso; eingetragen ist das Netz des TLS-Abschlusses im selben Stack |
| Port des Dienstes | nur für den Reverse-Proxy erreichbar | nicht am Host veröffentlicht; nur der TLS-Abschluss erreicht ihn |
| Von aussen erreichbar | Port 443 am eigenen Anschluss; der Name zeigt auf diesen Anschluss | Port 443 am Mietserver |
| Dazwischen | kein Dritter | kein Dritter |

Zu D-9: Hinter einem Proxy kommt jede Anfrage von dessen Adresse. Ohne die Regel zählten die
Grenzen je Aufrufer (D-2) alle Anfragen auf den Proxy, und der Dienst wäre nach wenigen
Kopplungen für alle gesperrt. Glaubte der Dienst den Kopf von jedem, könnte sich jeder Aufrufer
eine Adresse aussuchen. Der Server löst dasselbe mit `clientIP` und den vertrauten Proxys
(`server/internal/api/session.go`); die Dienste übernehmen die Regel: Der Kopf ist
`X-Forwarded-For`, gelesen von hinten, und es zählt der erste Eintrag, der kein eingetragener
Proxy ist. Was ein Aufrufer selbst in den Kopf schreibt, steht weiter vorn und wird nicht
geglaubt.

**Bekannte Folgen von E-22,** solange ein Dienst auf eigener Hardware läuft:

- Der Name des Dienstes zeigt auf den eigenen Anschluss, und dort ist Port 443 offen. Die Frage
  nannte das.
- Die zwei Container sind vom Internet aus erreichbar und gehören darum vom übrigen Netz
  getrennt. Mit E-22 wiegt das schwerer als zuvor.
- Fällt der Anschluss aus, fallen beide Dienste für alle Nutzer aus.
- Ein Slouchcat-Server im selben Netz muss die Push-Weiterleitung über deren **öffentliche**
  Adresse erreichen. Löst der Name dort auf eine private Adresse auf, weigert sich der Server zu
  senden (2.5). Leitet der Router Anfragen von innen an die eigene öffentliche Adresse nicht
  zurück, erreicht dieser Server die Push-Weiterleitung gar nicht.
- Der Reverse-Proxy sieht denselben Verkehr wie die Dienste (7.6). Für die zwei Namen schreibt
  er keine Adressen von Aufrufern in seine Logs (D-5).
- Zwischen Reverse-Proxy und Container läuft der Verkehr ohne TLS im eigenen Netz. Pakete und
  Meldungen sind dabei weiter Ende-zu-Ende verschlüsselt.

**Der Umzug eines Dienstes:**

1. Den Mietserver vorbereiten.
2. Compose-Datei, Umgebungsdatei und, bei der Push-Weiterleitung, die Geheimnisse kopieren; mit
   dem TLS-Profil starten.
3. Unter einem Probenamen prüfen: Erreichbarkeit, dazu eine Probe-Kopplung oder ein Probe-Push
   mit den Prüfprogrammen aus Stufe 2. Die echte App und ein echter Server taugen dafür nicht:
   Die App kennt nur den festen Namen der Push-Weiterleitung, und ein Server sendet nur an ihn
   mit der kleinen Satzgrösse (7.3). Der Dienst kennt seine Namen aus der Umgebungsdatei; für
   die Probe steht dort zusätzlich der Probename, weil die Push-Weiterleitung die Unterschrift
   gegen den eigenen Namen prüft (D-4).
4. Den DNS-Eintrag des festen Namens umstellen.
5. Warten, bis am alten Ort nichts mehr ankommt.
6. Den Eintrag im Reverse-Proxy und den Container am alten Ort entfernen.

**Das Zertifikat beim Umschalten (Frage F-41).** Für den festen Namen bekommt der Mietserver
auf dem üblichen Weg erst ein Zertifikat, wenn der DNS-Eintrag auf ihn zeigt, also nach
Schritt 4. Schritt 3 beweist nur den Probenamen. Zwei Wege:

- **Die kurze Lücke hinnehmen (Vorschlag).** Nach dem Umstellen holt der TLS-Abschluss das
  Zertifikat selbst; bis dahin scheitern Verbindungen. Eine Kopplung braucht dann einen neuen
  Code, und der Server wiederholt eine Meldung von sich aus. Dass das Ausstellen auf dieser
  Maschine klappt, zeigt Schritt 3.
- **Das Zertifikat vorab ausstellen,** über einen Nachweis im DNS. Das schliesst die Lücke,
  verlangt aber ein weiteres Geheimnis auf dem Mietserver: einen Zugang zum DNS, beschränkt auf
  diesen einen Namen.

Solange alte und neue Adresse nebeneinander gelten:

- **Vermittlung:** Handy und Fernseher können bei verschiedenen Instanzen landen. Eine Kopplung
  scheitert dann einmal und klappt mit einem neuen Code.
- **Push-Weiterleitung:** Beide Instanzen arbeiten nebeneinander, **wenn der Schlüssel zum
  Versiegeln mit umgezogen ist.** Ohne ihn kann die neue Instanz die Push-Adressen der alten
  nicht öffnen. Sie antwortet dann mit 503 und löscht nichts (D-10); die Meldungen bleiben aus,
  bis der Schlüssel am Ort ist.

**Auslieferung:** je Dienst ein Container-Image aus dem Repo; ein Tag `relay-v*` baut beide.

**Überwachung:** je Dienst eine Erreichbarkeitsprüfung auf `/v1/healthz` mit Meldung an Marco.
Vorschlag: von ausserhalb des Netzes, in dem der Dienst läuft, weil sie sonst mit ihm ausfällt.

**Adresse in den Apps änderbar (Frage F-21):** Für die Vermittlung ja, in den erweiterten
Einstellungen von Handy- und TV-App; beide Geräte müssen dann dieselbe Vermittlung eingestellt
haben, weil die Handy-App einen QR-Code mit einer anderen ablehnt (Abschnitt 10). Für Push hilft
ein eigener Dienst nicht: Er bräuchte den Firebase-Schlüssel der veröffentlichten App.

### 7.6 Datenschutz

| Wer | Sieht | Sieht nicht |
|---|---|---|
| Vermittlung | Internet-Adressen von Fernseher und Handy, Zeitpunkt, Geheimtext | Serveradresse, Gutschein, Konto |
| Push-Weiterleitung | Internet-Adresse des **Slouchcat-Servers**, Zeitpunkt und Häufigkeit der Meldungen, Google-Token des Handys, Geheimtext | Titel und Text der Meldung, Konto |
| Der Proxy vor einem Dienst (7.5) | dasselbe wie der Dienst dahinter | dasselbe wie der Dienst dahinter |
| Google (Push) | Google-Token, Zeitpunkt, Geheimtext | Titel und Text, Serveradresse |

Die Push-Weiterleitung sieht mehr als die Vermittlung: Jede Meldung zeigt ihr die Adresse eines
Heimservers, und das wiederholt. Gespeichert wird davon nichts (D-1, D-5). Der Proxy vor einem
Dienst ist an beiden Orten Marcos eigener; ein Dritter steht nicht dazwischen (E-22). Das gehört
in die Datenschutzerklärung, die der Store-Eintrag verlangt, und in die Angaben zur
Datensicherheit (11.4).

### 7.7 Ausfall

| Fällt aus | Folge | Bleibt |
|---|---|---|
| Vermittlung | niemand kann einen Fernseher per Scan einrichten | Einrichten von Hand am Fernseher (K26 4.4); eingerichtete Geräte merken nichts |
| Push-Weiterleitung | keine Meldungen bei geschlossener App; der Server versucht es wie bei jedem Kanal erneut | die Liste in der App, E-Mail und Telegram |
| Der eigene Anschluss, solange beide Dienste dort laufen | beides zugleich | wie oben |
| Googles Push-Dienst | wie bei der Push-Weiterleitung | wie dort |

Die zwei Dienste fallen sonst unabhängig voneinander aus. Die App selbst braucht sie für nichts
anderes: Stöbern, Abspielen und Downloads laufen ohne sie.

## 8. Wiedergabe

### 8.1 Videoplayer

Der Player der App kann dasselbe wie der Web-Player (Konzepte 13, 18 und 23):

- Wiedergabe, Pause, Springen um 10 Sekunden, Spulen mit Vorschaubildern (Trickplay), Kapitel
- Menüs für Tonspur, Untertitel und Qualität
- Vorspann und Abspann überspringen, Countdown zur nächsten Folge, vorige und nächste Folge
- Fortschritt alle 10 Sekunden, bei Pause und am Ende; Abbruch durch den Server erkennen;
  Wiedergabesitzung beim Verlassen beenden
- eine Info-Anzeige mit Codec, Bitrate und Abspielart

Dazu kommt, was ein Handy verlangt: Querformat für Video, Bildschirm bleibt an, Pause beim
Abziehen des Kopfhörers, Bedienung über Kopfhörer- und Bluetooth-Tasten (`MediaSession`).

### 8.2 Was die App dem Server meldet

Der Server entscheidet weiter allein zwischen Direktwiedergabe, Remux und Transcode
(`server/internal/playback/decide.go`). Die App füllt `PlaybackCapabilities`:

| Feld | Wert |
|---|---|
| `type` | `phone` oder `tablet` |
| `name` | zum Beispiel «Pixel 9 · Android 16» |
| `hls` | `mse`, wie K26 5.2 es begründet: Der Wert trifft, was ein ExoPlayer braucht; `native` bleibt Safari auf dem iPhone |
| `containers` | die Namen, die der Server kennt (`mp4`, `mkv`, `webm`, `ts`, …); ohne die Liste spielt der Server nur MP4 direkt |
| `video`, `audio` | was die Decoder des Geräts können |
| `maxWidth`, `maxHeight`, `hdr` | aus der Anzeige des Geräts |

Neue Felder im Vertrag braucht die Handy-App dafür nicht. Die drei Ergänzungen aus K26 5.2
(Kanalzahl, «wählt Spuren selbst», Untertitelformate) gehören zur TV-App und zu ADR 0041; die
Handy-App nutzt sie, sobald es sie gibt.

### 8.3 Ton (E-13)

- **Hörbücher und Podcasts** nutzen `startPlayback` wie die Web-App. Ein Hörbuch aus mehreren
  Dateien kommt als eine HLS-Zeitachse; eine Podcast-Folge ohne Datei streamt der Server aus der
  Quelle des Feeds.
- **Webradio** nutzt `startRadioListen`: eine HLS-Liste mit dem Token `rt`, ein Herzschlag im
  genannten Abstand, `endRadioListen` am Ende. Radio zählt zur Schauzeit des Profils; das
  entscheidet der Server.
- **Der Ton läuft weiter,** wenn der Bildschirm aus ist oder eine andere App vorn steht
  (`MediaSessionService`, Vordergrunddienst `mediaPlayback`). Bedienung am Sperrbildschirm, in
  der Mitteilungsleiste und über Kopfhörer. Fortschritt und Herzschlag laufen dabei weiter.
- Ein Tonplayer mit Titelbild, Kapiteln oder Titelanzeige, Tempo und Sprungtasten, wie in der
  Web-App.

Die TV-App hat das Weiterlaufen im Hintergrund ausgenommen (K26 5.6); für das Handy ist es der
Hauptfall.

### 8.4 Profilwechsel und Abmelden

Der Browser leert beim Profilwechsel seinen Bildspeicher auf Geheiss des Servers
(`Clear-Site-Data`, ADR 0005). Die App tut es selbst: Sie stoppt die Wiedergabe, leert den
Bildspeicher und alle gemerkten Listen beim Profilwechsel und beim Abmelden.

## 9. Downloads und Offline (E-8)

### 9.1 Grundsatz

Die App nutzt die Offline-Endpunkte, wie sie sind (2.4). Der Server bereitet vor, das Gerät holt
ab, der Abgleich hält beide gleich. Neue Offline-Operationen braucht die App nicht. Am Server
ändert sich für Offline dreierlei: die Gerätekennung im Kopf und die Aufräumung (4.3) sowie das
Aufräumen von Downloads und Regeln beim Abmelden unter «Meine Geräte» (4.5).

Die PWA und die native App auf demselben Handy sind **zwei Geräte** mit getrennten Downloads.

| Aufgabe | PWA heute | Native App |
|---|---|---|
| Datei | OPFS im Browser | Ordner der App: Mediendatei, Untertitel, Vorschaubilder, Titelbild |
| Datenbank | IndexedDB (`store.ts`) | Room |
| Abspielen | Abspielliste aus dem Fragmentindex, über Media Source Extensions (`loader.ts`) | Abspielliste aus dem Fragmentindex, gespielt von Media3 aus der lokalen Datei |
| Abruf | Blöcke von 8 MiB, solange die App offen ist (`downloader.ts`) | Auftrag des Systems im Hintergrund (9.3) |
| Abgleich | Start, Netz zurück, Fokus, alle 60 s (`engine.ts`) | dasselbe, dazu regelmässig im Hintergrund (9.6) |
| Verbindungsart | `navigator.connection`, auf dem iPhone nicht verfügbar | das System kennt WLAN und Mobilfunk verlässlich |
| Offline-PIN | PBKDF2 über WebCrypto (`pin.ts`), Bremse nach Fehlversuchen auf dem Gerät | PBKDF2 der Plattform, dieselben Werte, dieselbe Bremse |
| Uhr | Abstand zur Serverzeit aus dem Abgleich (`serverTime`), `sentAt` beim Hochladen | dasselbe |
| Start ohne Netz | Service Worker | die App startet von selbst in die Offline-Bibliothek |

Zum Abspielen: Die Download-Datei ist fragmentiertes MP4. Ob ExoPlayer darin ohne Hilfe spulen
kann, ist unbelegt. Der Fragmentindex des Manifests löst das wie in der PWA; Stufe 0 misst
beide Wege.

### 9.2 Ablauf

1. **Bestellen:** Download-Dialog mit den drei Stufen, Grösse, Tonspur und Untertiteln
   (`getDownloadOptions`), dann `createDownloads`. Eine Staffel in einer Aktion.
2. **Warten:** Die Seite «Downloads» zeigt den Zustand des Servers (wartet, wird vorbereitet,
   pausiert, bereit, fehlgeschlagen) samt Grund, zum Beispiel «Budget des Servers voll».
3. **Holen:** Sobald der Download bereit ist, holt die App das Manifest, dann Datei, Untertitel,
   Vorschaubilder und Titelbild. Ein unterbrochener Abruf setzt an der Stelle fort; der starke
   `ETag` der Datei verhindert, dass zwei Vorbereitungen gemischt werden.
4. **Melden:** Der nächste Abgleich meldet den Download als vollständig. Der Server räumt seine
   Kopie 24 Stunden später.
5. **Spielen:** aus der Offline-Bibliothek, auch ohne Netz und ohne gültige Sitzung.

### 9.3 Abruf im Hintergrund

- **Vom Nutzer gestartete Downloads** laufen als Übertragungsauftrag des Systems (Android 14 und
  neuer). Er läuft weiter, wenn die App geschlossen wird, zeigt eine Mitteilung mit Fortschritt
  und unterliegt nicht der 6-Stunden-Grenze. Das System verlangt, dass er startet, während die
  App sichtbar ist; das trifft zu, weil der Nutzer gerade auf «Herunterladen» getippt hat.
- **Auto-Downloads** startet niemand von Hand, und die App ist dabei meist zu. Android lässt eine
  App aus dem Hintergrund keinen Vordergrunddienst starten (seit Android 12, Kenntnis des
  Kritikers, unbelegt). Ein langer Abruf lässt sich so nicht einfach anstossen. Vorschlag: Der
  Abgleich im Hintergrund holt die Datei in kurzen Läufen von WorkManager stückweise und setzt
  jedes Mal an der Stelle fort. Trägt das nicht, holt die App Auto-Downloads beim nächsten
  Öffnen, wie die PWA. Stufe 0 misst es.
- **Ältere Geräte** (vor Android 14) nutzen für vom Nutzer gestartete Downloads den
  Vordergrunddienst mit der 6-Stunden-Grenze. Ob es sie gibt, hängt an der kleinsten
  Android-Version (Frage F-13).
- Das Download-Token läuft nicht ab (2.4). Datei, Untertitel und Vorschaubilder lassen sich
  darum auch nach einem Neustart des Handys und ohne gültige Sitzung weiter holen; der Server
  prüft bei jeder Anfrage, ob das Profil den Titel noch sieht. **Das Titelbild braucht die
  Sitzung** (`getItemImage`); die App holt es darum zuerst.
- Unbelegt und in Stufe 0 gemessen: ob ein Auftrag das Wegwischen der App und einen Neustart
  übersteht, und wie sich ein Abruf von 20 GB über Nacht verhält.

### 9.4 Speicher

- Die Dateien liegen im eigenen Ordner der App. Das braucht keine Berechtigung; beim
  Deinstallieren verschwinden sie. Von der Sicherung des Geräts sind sie ausgenommen.
- Die Seite «Downloads» zeigt belegten und freien Platz. Vor dem Bestellen vergleicht die App
  die geschätzte Grösse mit dem freien Platz und warnt.
- Eine eigene Obergrenze auf dem Gerät und die SD-Karte als Ort sind Frage F-26.
- Das Budget des Servers (100 GB für vorbereitete Kopien) bleibt, wie es ist. Ist es voll,
  zeigt die App den Zustand; der Download beginnt, sobald Platz ist.
- Die Dateien liegen unverschlüsselt im Ordner der App, wie in der PWA im Speicher des Browsers.
  Ein Kopierschutz ist kein Ziel.

### 9.5 Regel 12 offline

- Die App speichert `allowedProfileIds` je Download, wie der Server sie liefert, und zeigt einen
  Download nur den Profilen darin. Sie rechnet nichts selbst aus.
- Die zwei erklärten Ausnahmen von Plan-Prinzip 1 gelten wie in der PWA: Die App prüft die
  Offline-PIN gegen einen Verifier und zieht die Schauzeit nach Geräteuhr vom Tagesrest ab.
  Beides schützt vor neugierigen Mitbenutzern, nicht vor einem manipulierten Gerät.
- Der Verifier kommt auf zwei Wegen, wie in der PWA (Plan 3.3, `pin.ts`): Für die eigene
  Offline-PIN liefert ihn der Server. Für die normale PIN kennt der Server nur seinen eigenen
  Hash; die App leitet den Verifier nach der nächsten geglückten PIN-Eingabe mit Netz selbst ab,
  mit dem Salz aus dem Abgleich, und nur, wenn das Gerät einen Download des Profils hält.
- **Widerruf:** Was der Abgleich unter `revoked` nennt, löscht die App sofort. Ohne gültige
  Sitzung fragt sie `checkOfflineRevocations` mit der Gerätekennung (4.3). Der Entzug wirkt wie
  heute beim nächsten Kontakt (AN-4); ein Handy, das nie mehr Kontakt hat, behält die Dateien.
- **Fortschritt:** Die App schreibt Roh-Ereignisse der Offline-Wiedergabe in eine Warteschlange
  und lädt sie beim nächsten Kontakt hoch (`uploadOfflineEvents`). Zusammenführen und Buchen
  macht der Server.

### 9.6 Auto-Downloads

Die Regel je Profil und Gerät bleibt (`setDownloadRules`): nächste Folgen, neue Podcast-Folgen,
«nur WLAN», Höchstzahl, Qualität. Der Server legt Auto-Downloads beim Abgleich an. Damit das
auch geschieht, wenn die App zu ist, gleicht die App regelmässig im Hintergrund ab (Vorschlag:
etwa alle 6 Stunden, Frage F-34).

«Nur WLAN» prüft der Client. Auf Android weiss er die Verbindungsart sicher; der Punkt «Nur WLAN
für Auto-Downloads» der Gerätecheckliste M5 wird damit prüfbar.

### 9.7 Abmelden

Vorschlag (Frage F-25): Wer sich in der App abmeldet, wird gefragt, ob die Downloads auf dem
Gerät gelöscht werden sollen; ab Werk ja. Die App löscht sie dann auch im Server
(`deleteDownload`), bevor sie die Sitzung beendet; sonst blieben die Zeilen stehen und kämen
bei der nächsten Anmeldung zurück. Die PWA behält Downloads heute, weil dort mehrere Konten
denselben Browser teilen können; die App kennt ein Konto je Gerät (Frage F-31).

## 10. Fernseher verbinden (E-10, E-16)

Der Ablauf aus Sicht des Fernsehers steht in K26 4.3, die Vermittlung in Abschnitt 7. Dies ist
die Seite der Handy-App:

1. «Mehr › Fernseher verbinden» öffnet den Scanner.
2. Die App liest den QR-Code und prüft Fassung und Adresse der Vermittlung. Nennt der Code eine
   andere als die eingestellte, lehnt sie ab.
3. Die App zeigt Konto und Server und fragt: «Diesen Fernseher mit deinem Konto verbinden? Tu
   das nur, wenn der Code auf deinem eigenen Fernseher steht.» (K-5)
4. Nach dem Ja holt die App beim Server den **Kopplungsgutschein** für den Schlüssel aus dem
   QR-Code.
5. Die App verschlüsselt Serveradresse und Gutschein und legt das Paket bei der Vermittlung ab
   (7.2). **Antwortet sie mit 409 (es liegt schon ein Paket) oder 410 (es wurde schon eines
   abgeholt), war jemand schneller.** Das ist der Fall «untergeschobenes Paket» (7.2); 410 ist
   dabei der häufigere, weil der Fernseher alle paar Sekunden nachfragt. Die App warnt dann:
   Der Fernseher ist womöglich schon mit einem fremden Server verbunden; am Fernseher stehen
   Server und Konto, mit denen er verbunden ist. Den eigenen Gutschein lässt sie verfallen.
6. Sonst wartet die App, bis das neue Gerät unter «Meine Geräte» erscheint, und zeigt
   «Verbunden» mit dem Namen des Fernsehers. Nach 2 Minuten ohne Einlösen meldet sie, dass es
   nicht geklappt hat.

**Der Gutschein** (K-3): 256 Bit Zufall, im Server nur als Hash, gültig 2 Minuten, einmal
verwendbar, an den Schlüssel des Fernsehers gebunden. Er trägt die Art des Geräts (`tv`).

- **Ausstellen darf ihn nur eine App-Sitzung** (E-16). Eine Browser-Sitzung und eine
  Gerätesitzung bekommen 403.
- Vorschlag (Frage F-23): nur ohne aktives Profil oder mit dem Hauptprofil, wie bei «Meine
  Geräte» und den Meldekanälen. Ein Kind im Kinderprofil am Handy der Eltern hängt dann keinen
  Fernseher an.
- Höchstens ein offener Gutschein je Konto und eine Ratenbegrenzung je Konto.
- Jede Kopplung geht als Meldung über die Kanäle des Kontos hinaus (K-7). Mit E-17 kommt sie
  auch als Push auf das Handy. Die Meldung nennt, was der Server selbst weiss (Zeitpunkt, Art
  des Geräts), und kennzeichnet den Namen als Angabe des Geräts; den Namen wählt, wer koppelt.

**Die Adresse, die das Handy mitgibt,** ist die, unter der es selbst den Server erreicht. Ist
sie für den Fernseher nicht erreichbar (zum Beispiel eine VPN-Adresse des Handys), scheitert das
Einlösen. Der Fernseher zeigt dann die Adresse und bietet das Einrichten von Hand an.

**Solange es die TV-App nicht gibt,** prüft ein Prüfprogramm, das einen Fernseher spielt: Es
zeigt den QR-Code an, holt das Paket und löst den Gutschein ein.

## 11. Verteilung (E-2, E-20)

### 11.1 Eine App, ein Paketname (E-20)

- Paketname **`com.slouchcat.app`**, für Handy und Fernseher. Er ist nach dem ersten Hochladen
  unveränderlich.
- Die Handy-App wird zuerst hochgeladen. Die TV-App kommt später als zweite Geräteart mit eigener
  Veröffentlichungsspur in denselben Eintrag (K26).
- Wegwerf- und Entwicklungsbauten tragen einen anderen Namen (Anhängsel) und werden nie
  hochgeladen.

### 11.2 Signatur

- Neue Apps nehmen an «Play App Signing» teil: Google hält den Schlüssel, mit dem die App
  ausgeliefert wird. Marco hält den Upload-Schlüssel; er liegt nie im Repo und lässt sich bei
  Verlust zurücksetzen.
- Folge: Eine App, die nicht aus dem Play Store kommt (ADB, Bauart `foss`), trägt eine andere
  Signatur und lässt sich nicht über eine Installation aus dem Store spielen oder umgekehrt.
- Ob das Hochladen von Hand oder aus der CI geschieht, ist Frage F-11.

### 11.3 Der Weg zur Veröffentlichung

| Schritt | Wer bekommt die App | Vorbedingung |
|---|---|---|
| 1 Bau | Marco, per ADB auf sein Handy | Entwickleroptionen am Handy |
| 2 Interne Testspur | bis 100 eingetragene Google-Adressen, über einen Link | Entwicklerkonto nutzbar (Identitätsprüfung läuft); erstes Hochladen legt den Paketnamen fest |
| 3 Geschlossener Test | mindestens 12 Tester, 14 Tage am Stück | wie 2 |
| 4 Antrag auf Produktion | – | Schritt 3 erfüllt; Fragen von Google zum Test und zur App |
| 5 Öffentlich | alle | Antrag bewilligt, Store-Eintrag vollständig (11.4) |

Der geschlossene Test muss nicht auf die fertige App warten. Er kann beginnen, sobald eine
brauchbare Ausgabe steht (nach Stufe 6), und läuft dann neben den späteren Stufen. Wer die 12
Tester sind, ist Frage F-35.

### 11.4 Was der Store-Eintrag verlangt

- **Datenschutzerklärung** unter einer HTTPS-Adresse (Frage F-36). Sie nennt die zwei Dienste
  aus Abschnitt 7 und was sie sehen (7.6).
- **Angaben zur Datensicherheit.** Vorschlag, vorbehaltlich Googles Formular: Die App erhebt
  selbst nichts; Inhalte und Konto gehen nur an den Server des Nutzers; die zwei Dienste sehen
  Internet-Adressen, die Push-Weiterleitung auch das Google-Token, und speichern nichts davon
  (7.6). Wie Google die ersten beiden
  Punkte einordnet, ist unbelegt.
- **Zugang für die Prüfer:** ein erreichbarer Vorführserver mit Testzugang und frei lizenzierten
  Testfilmen (K26 F-10; wird mit der Handy-App zuerst fällig).
- **Erklärung der Vordergrunddienste** `mediaPlayback` und `dataSync`, je mit einem Video.
- **Alterseinstufung und Zielgruppe.** Slouchcat kennt Kinderprofile, die App richtet sich aber
  an den Kontoinhaber. Was das für Googles Familienrichtlinie heisst, ist unbelegt und vor dem
  Antrag zu klären.
- **Öffentlich sichtbar** sind bei einem privaten Entwicklerkonto der bürgerliche Name und das
  Land (2.7).
- Der Text des Eintrags beschreibt die App als Abspieler für die eigene Mediensammlung. Googles
  Richtlinie verbietet Apps, die zu Urheberrechtsverletzungen anleiten; persönliche
  Medienabspieler nennt sie nicht.

### 11.5 Versionen und Verträglichkeit

Heute reist die Web-App im selben Container wie der Server. Die App im Play Store aktualisiert
sich von selbst, der Server wird von Hand gehoben. Beide Richtungen müssen halten.

- **Server zu alt:** `getHealth` bekommt ein Feld mit der Vertragsfassung. Die App kennt die
  kleinste Fassung, die sie braucht, und zeigt sonst «Server zu alt, bitte aktualisieren» mit
  beiden Fassungen.
- **App zu alt:** Die App nennt ihre Version in jeder Anfrage. Der Server kann eine zu alte App
  mit einem eigenen Fehler abweisen; die App zeigt dann «Bitte die App aktualisieren».
- **Unbekanntes verträgt die App:** unbekannte Felder und unbekannte Werte einer Aufzählung
  dürfen den Client nicht zum Absturz bringen. Das ist eine Anforderung an den Generator
  (Stufe 0).
- **Neue Regel neben «Vertrag = Tag» (Frage F-9):** Operationen mit `x-clients` brechen nicht
  mehr von einem Release aufs nächste. Eine brechende Änderung braucht eine Übergangszeit, in
  der der Server alt und neu liefert. **Das trifft den ganzen Nutzerbereich:** `app` steht an
  allen 116 Operationen ausserhalb der Verwaltung (4.4). Frei ändern lässt sich danach nur noch
  die Verwaltung.
- **Zur Dauer:** K26 empfiehlt «zwei Minor-Releases des Servers». Am 15. November 2026 sind
  sechs Minor-Releases erschienen (Tags `v1.33.0` bis `v1.38.0`); zwei Releases können ein Tag
  sein. Vorschlag darum nach Zeit: mindestens 90 Tage, und mindestens 30 Tage, nachdem eine
  App-Version im Store ist, die das Neue kann.

Solange die App nur privat verteilt wird, hat Marco beide Seiten in der Hand. Zwingend wird die
Regel mit der öffentlichen Stufe; sie von Anfang an einzuhalten, kostet am wenigsten.

## 12. Prüfung

| Ebene | Was | Entspricht |
|---|---|---|
| Unit | Zustände der Seiten; Offline-Logik (Abgleich anwenden, PIN, Schauzeit, Ereignisse) mit den Fällen aus `sync.test.ts`, `pin.test.ts` und `engine.test.ts` | Vitest der Web-App |
| Oberfläche | je Seite ein Compose-Test: jedes Bedienelement erreichbar und beschriftet, Zurück führt hinaus, zwei Breiten | Regel 8 |
| Generatoren | Texte: alle 22 Sprachen und die Ergänzung `de-DE` ohne Abbruch; Farben und Masse: Stichproben gegen die CSS-Ausgabe | `gen:check` |
| Vertrag | der Client der App gegen einen echten Server mit Testdaten: anmelden, Profil wählen, Abspielstart, Download bestellen und holen, Abgleich | `apicheck` |
| Server | Vertragstest mit jeder Sitzungsart, die neuen öffentlichen Operationen im Register; der Test aus 4.4 (Besitzerkonto, jede Bearer-Art, eigener Fehlercode); falscher Träger je Art wird abgelehnt; Sitzungen ohne Ablauf und ihre Geräte überleben die Aufräumung; Widerrufsliste über den Kopf; fremde und reservierte Gerätekennung; Gutschein abgelaufen, doppelt, falscher Schlüssel; Abmelden löscht Push-Abos; «Server weist sich aus»: richtige und falsche Unterschrift, Ratenbegrenzung, neuer Schlüssel verlangt die Bestätigung | `TestContractCoversEveryOperation` |
| Dienste | Ablauf, Grössen, Raten, Abholgeheimnis, die drei Antworten beim Ablegen (409, 410, 404), VAPID-Prüfung; 503 statt 410 bei falschem Schlüssel (D-10); Adresse des Aufrufers nur aus dem Kopf eines eingetragenen Proxys (D-9); gemeinsamer Testvektor für die Verschlüsselung in Go und Kotlin | wie Server |
| Gerät | Abnahme durch Marco auf seinem Handy, je Stufe eine Liste; für Offline die Zeile «Android» der Gerätecheckliste M5 | Abnahme auf CT 190 |

Die Teststufen aus `CLAUDE.md` gelten weiter.

## 13. Stufen

Jede Stufe ist ein eigener Auftrag von Marco. Stufe 1 ist ein Release des Servers, Stufe 2 eines
der zwei Dienste; ab Stufe 3 ist jede Stufe eine Ausgabe der App.

| Stufe | Inhalt | Beleg | Braucht |
|---|---|---|---|
| **0 Messung** | Wegwerf-App mit eigenem Paketnamen, kein Eingriff ins Produkt (Anmeldung mit Cookie wie `apicheck`). Misst: Generator für den Kotlin-Client am echten Vertrag; Direktwiedergabe und HLS in ExoPlayer; Download-Datei lokal abspielen und spulen; Abruf von 20 GB im Hintergrund samt Wegwischen und Neustart; Abruf ohne offene App in kurzen Läufen; Verschlüsselung zwischen Go und Android; Push durch einen Probe-Dienst (Satzgrösse, Verzögerung, Verhalten von Googles Bibliothek); Generatoren für Texte und Farben; Laufzeit eines CI-Jobs | Bericht `docs/perf/K27-android-handy.md` mit Entscheid je Punkt | Frage 35 im Entscheidungsprotokoll und die Ausnahme in Regel 11 (Abschnitt 14); Firebase-Projekt für die Push-Probe |
| **1 Server** | alles aus Abschnitt 15.1: Bearer-Weg, drei Sitzungsarten mit `x-clients`, `tv` an den 93 Operationen aus K26 Anhang B, «App anmelden», Gerätekennung im Kopf, Sitzungen ohne Ablauf samt Aufräumung, «Meine Geräte» (Operationen und Seite der Web-App), Gutschein ausstellen und einlösen, Meldung «Neues Gerät», Push-Anpassungen, Vertragsfassung in `getHealth`, Redaktionsliste | Vertragstest samt dem Test aus 4.4; ein Prüfprogramm nach Art von `apicheck`, das ein Handy und einen Fernseher spielt; Tastaturtest für «Meine Geräte»; Abnahme auf CT 190 | **Stufe 0** (Bibliothek für die Verschlüsselung, Satzgrösse für Push); F-18, F-23, F-24, F-38 |
| **2 Dienste** | `relay/` mit den zwei Diensten Vermittlung und Push-Weiterleitung (E-21): zwei Images, je Dienst eine Compose-Datei mit Umgebungsdatei und TLS-Profil, Betriebsanleitung samt Umzug, Release-Workflow für `relay-v*`, Überwachung. Die zwei Container entstehen hier; beide laufen hinter dem vorhandenen Reverse-Proxy (E-22) | Tests der Dienste; zwei Prüfprogramme koppeln über die laufende Vermittlung; eine Push-Probe kommt an; ein abgeschalteter Dienst löst die Meldung an Marco aus; **Umzugsprobe:** ein Dienst startet unter einem Probenamen auf einer zweiten Maschine mit dem TLS-Profil und übernimmt nach dem Umschalten | Stufe 0; F-37, F-39, F-41; Firebase-Projekt; für die Umzugsprobe eine zweite, von aussen erreichbare Maschine (Port 443 der eigenen Hardware gehört dem vorhandenen Reverse-Proxy) |
| **3 Gerüst** | Projekt `android/`, Werkzeuge, CI-Jobs, Generatoren und Wächter; Serveradresse mit Warnung (E-18), Anmelden, Profilwahl mit PIN, Meine Geräte, Abmelden | App per ADB auf dem Handy: anmelden, Profil wählen, Gerät erscheint unter «Meine Geräte»; gemessene Laufzeit der CI in `docs/perf/` | Stufe 1; F-1, F-13, F-33 |
| **4 Stöbern** | Start, Bibliothek, Detailseiten, Person, Suche, Listen, Profile verwalten | Oberflächentest je Seite; Abnahme: blättern ohne Abspielen | freigegebenes Mockup (F-22); F-19 |
| **5 Videoplayer** | Player vollständig, Einstellungen der Wiedergabe | Abnahmeliste auf dem Handy; **erste Ausgabe über die interne Testspur** | nutzbares Entwicklerkonto; F-11 |
| **6 Downloads** | Download-Dialog, Seite «Downloads», Abruf im Hintergrund, Offline-Bibliothek, Offline-Player, Offline-PIN, Schauzeit, Abgleich, Widerruf, Auto-Downloads | Gerätecheckliste M5 für Android im Flugmodus; Abruf von 20 GB über Nacht; Entzug eines Kontos löscht die Dateien beim nächsten Kontakt | F-25, F-26, F-34 |
| **7 Ton** | Hörbücher, Podcasts, Webradio, Wiedergabe im Hintergrund, Downloads von Hörbüchern und Folgen | Abnahme: eine Stunde Hörbuch bei ausgeschaltetem Bildschirm, Fortschritt stimmt | |
| **8 Wünsche und Meldungen** | Bereich «Wünsche», Benachrichtigungen, Ankündigungen, Meldekanäle, Push | Abnahme mit echtem Seerr; eine Meldung kommt bei geschlossener App an | Stufe 2 |
| **9 Fernseher verbinden** | Scanner und Kopplung in der App | Kopplung vom Handy über die Vermittlung gegen CT 190 mit dem Prüfprogramm als Fernseher; **erste Ausgabe nach E-13 vollständig** | Stufe 1 und 2 |
| **10 Öffentlich** | Geschlossener Test, Antrag, Store-Eintrag, Datenschutzerklärung, Vorführserver, Verträglichkeitsregel verbindlich | App im Play Store | F-9, F-10, F-35, F-36 |

Zur Reihenfolge:

- Stufe 0 kommt zuerst, weil sie über den Generator, den Abspielweg für Downloads, die
  Verschlüsselung und den Push-Weg entscheidet. Sie braucht keinen Code im Produkt, ist aber die
  erste Arbeit an einer nativen App; Frage 35 und die Ausnahme in Regel 11 müssen darum vorher
  stehen, wie bei Konzept 25 die Regeländerung mit Stufe 0 kam.
- Stufe 1 hängt an Stufe 0: Sie baut das Einlösen mit verschlüsseltem Token und die
  Push-Anpassungen, über deren Einzelheiten Stufe 0 entscheidet.
- Stufe 1 baut die Kopplung im Server schon mit, obwohl die App sie erst in Stufe 9 nutzt: Die
  Sitzungsarten werden so nur einmal angefasst, und die TV-App findet den Serverteil fertig vor.
- Stufe 2 kann neben Stufe 3 bis 7 laufen. Gebraucht wird die Push-Weiterleitung in Stufe 8,
  die Vermittlung in Stufe 9.
- Das Mockup entsteht neben Stufe 1 bis 3.
- Die TV-App (K26) kann nach Stufe 3 beginnen: Dann stehen Serverteil, Projekt, Client, Texte
  und Farben. Für das Einrichten per Scan braucht sie Stufe 2 und 9.

## 14. Änderungen an früheren Entscheiden und Regeln

Alle erst mit dem Bau und auf Marcos Wort, nicht mit diesem Papier. Frage 35 kommt mit der
Freigabe ins Entscheidungsprotokoll; die Ausnahme in Regel 11 und die Zeile der Spec kommen
vor Stufe 0, die übrigen Regeln mit der Stufe, die sie braucht. Wird dieses Papier vor
Konzept 26 freigegeben, steht Frage 35 vor Frage 34 im Protokoll; die Nummern bleiben. K26
Abschnitt 11 nennt dieselben Stellen aus Sicht der TV-App; wo beide Papiere eine Stelle ändern,
gilt die Fassung hier, weil dieses Konzept zuerst gebaut wird.

| Stelle | Heute | Neu |
|---|---|---|
| Spec «Non-Goals» und Übersichtstabelle | native iOS- und Android-Apps sind Nicht-Ziel | die native Android-Handy-App ist Ziel (Frage 35). Native iOS-Apps bleiben Nicht-Ziel |
| Plan Abschnitt 1 («Nicht eingeplant») | native Apps | Android-Handy-App herausgenommen |
| `CLAUDE.md` Regel 11, «native Apps» | Nicht-Ziel | benannte Ausnahme: die Android-Handy-App nach Frage 35 und die Android-TV-App nach Frage 34 |
| `CLAUDE.md` Regel 11, «Cloud» | Nicht-Ziel | zwei benannte Ausnahmen: die Vermittlung für das Einrichten per Scan (E-11) und die Push-Weiterleitung (E-17), zwei getrennt betriebene Dienste aus dem Modul `relay/` (E-21). Sie halten keine Konten und keine Inhalte und sehen Pakete und Meldungen nur verschlüsselt |
| `CLAUDE.md` Regel 7 | jede Operation trägt `x-visibility` | dazu: jede `account`- und `scoped`-Operation nennt in `x-clients`, welche Bearer-Arten sie aufrufen dürfen |
| `CLAUDE.md` Regel 8 | Playwright-Tastaturtest je neuer Seite | dazu: je Seite der Handy-App ein Oberflächentest (Abschnitt 12) |
| `CLAUDE.md` Regel 9 | Farben aus Tokens, Texte aus `web/src/locales`, gemeinsame Bauteile aus `web/src/ui` | gilt für `android/` mit erzeugten Farben und Texten; der Satz zu `web/src/ui` gilt nur für die Web-App |
| `CLAUDE.md` Regel 10 | Redaktionsliste der Logs | dazu die Schlüsselnamen für Gerätetoken, Gutschein, Gerätekennung und Google-Token, im Server und in den zwei Diensten |
| `CLAUDE.md` Regel 12 | Entscheid im Server | unverändert; die zwei erklärten Ausnahmen gelten für die native App wie für die PWA |
| `CLAUDE.md` Werkzeuge | Go, Node, pnpm, `task` | dazu JDK, Android-SDK, Gradle und ein zweites Go-Modul `relay/` |
| Plan 3.8 Punkt 7 | «Fremd-Clients per Bearer» (nie gebaut) | gebaut als App-Sitzung und Gerätesitzung |
| ADR 0005 | eine Sitzungsart, 30 Tage gleitend | Nachtrag: drei Sitzungsarten; App- und Gerätesitzungen laufen nicht ab (E-15, F-4) |
| Aufräumung der Geräte (ADR 0005, «400 Tage unbenutzt») | löscht jedes Gerät ohne Kontakt | nimmt Geräte mit lebender Sitzung ohne Ablauf aus |
| `checkOfflineRevocations` (AN-4, K-7) | erkennt das Gerät am Cookie | dazu am Kopf mit der Gerätekennung |
| Geräteverwaltung (W-7) | nur `admin` | dazu «Meine Geräte» je Konto (F-15) |
| Web-Push (Plan 5.5) | Abos von Browsern; ein Abo überlebt jede Abmeldung; ändern nur mit Hauptprofil | dazu Abos der Handy-App über die Push-Weiterleitung mit kleinerer Satzgrösse; Abmelden, «Gerät vergessen» und Passwortänderung löschen die Abos des Geräts; eine App-Sitzung verwaltet ihr eigenes Abo |
| Passwortänderung (`updateAccount`, A-2) | entzieht jede andere Sitzung | bleibt und gilt auch für Handys und Fernseher (Frage F-24) |
| Regel «Vertrag = Tag» | ein Tag je Vertragsfassung | bleibt; daneben die Übergangszeit für Operationen mit `x-clients` (11.5) |
| Gerätecheckliste M5 | «Android Chrome» ausstehend | die native App bekommt eine eigene Zeile |

## 15. Änderungen am Code im Überblick

### 15.1 Server (Stufe 1)

| Was | Wo | Art |
|---|---|---|
| Bearer: der Server liest `Authorization`, auch auf den Streaming-Routen; kein `Set-Cookie` auf diesem Weg; die Art steht in der Sitzung, der Träger muss passen (4.1) | `server/internal/api/session.go`, `api.go` | neu |
| Stellen ausserhalb der Middleware, die das Cookie setzen: `logout` (`loggedOut` löscht das Cookie und setzt `Clear-Site-Data`) antwortet einer Bearer-Sitzung ohne beides. Das Vorladen der Web-Hülle (`preload.go`) bleibt beim Cookie | `server/internal/api` | Verhalten ändert sich für Bearer |
| Sitzungsart je Sitzung; «läuft nicht ab» als Ablauf weit in der Zukunft; `TouchSession` verschiebt ihn bei diesen Arten nicht | `server/internal/auth`, `server/queries/app`, Migration | neu |
| Erlaubnisliste `x-clients` im Vertrag: `app` an allen 116 Operationen des Nutzerbereichs, `tv` an den 93 aus K26 Anhang B; samt Schema-Test und dem Test aus 4.4 mit eigenem Fehlercode | `api/openapi.yaml`, Middleware, `binaryRoutes` | neu |
| «App anmelden»: Token, Gerätezeile mit Name und Art sofort; Gerätekennung vom Server, fremde und reservierte Kennung abgewiesen; Abgleich des Passworts mit Seerr wie bei `login`; Name bleibt beim Abspielstart | Vertrag, `auth`, `playback` | neu |
| Gerätekennung im Kopf für Anfragen ohne Sitzung | `session.go`, `offline_handlers.go` | neu |
| Aufräumung verschont Geräte mit lebender Sitzung ohne Ablauf und deren Einträge der Widerrufsliste; ein verschwundenes Gerät ist für diese Arten ein Fehler (Sitzung entzogen, 401) | `auth` (`DeleteStaleDevices`, `loadSession`), `offline` (`tombstoneKeep`) | Verhalten ändert sich |
| Meine Geräte: Liste (Sitzungen oder Downloads des Kontos) und Abmelden; Seite in der Web-App mit Tastaturtest; Texte in allen Sprachen | Vertrag, `web/src` | neu |
| Abmelden räumt auf: Push-Abos, Downloads und Regeln für Auto-Downloads des Kontos auf dem Gerät; dasselbe für Push-Abos bei `logout` einer App-Sitzung, bei `deleteDevice` (vor der Gerätezeile) und bei einer Passwortänderung (nur Handys und Fernseher) | `auth`, `admin`, `notify`, `offline` | Verhalten ändert sich |
| Kopplung: Gutschein ausstellen (nur App-Sitzung, trägt die Art `tv`) und einlösen (mit Gerätename und Systemfassung; Token verschlüsselt für den Schlüssel des Geräts) | Vertrag, `auth`, Migration | neu |
| Meldung «Neues Gerät verbunden» über die Kanäle des Kontos: neue Art von Ereignis, Texte des Servers in allen Sprachen, wo nötig eine Migration der Arten | `server/internal/notify`, `notify/locales`, Migration | neu |
| Push: Adresse der Push-Weiterleitung im Server eingebaut und einstellbar; kleinere Satzgrösse und gekürzter Text für Adressen mit diesem Ursprung; eine App-Sitzung verwaltet ihr eigenes Abo; ein neues Abo des Geräts ersetzt bei App-Sitzungen das alte | `notify`, `notify/push`, `config` | Verhalten ändert sich |
| «Server weist sich aus» für Verbindungen über HTTP (Abschnitt 6, F-38): Schlüsselpaar des Servers beim Geheimnis des Servers, öffentlicher Schlüssel in `getHealth`, öffentliche Operation mit Ratenbegrenzung und Eintrag im Register des Vertragstests | Vertrag, `auth` | neu, falls F-38 ja |
| `getHealth`: Feld mit der Vertragsfassung; Fehler «App zu alt» | Vertrag | zusätzlich |
| Geräteliste der Verwaltung zeigt die Sitzungsart | Vertrag, `web/src/features/admin` | zusätzlich |
| Register des Vertragstests (`publicRegistry` und die anderen) und `apicheck` für die neuen Operationen | `server/internal/api/contract_access_test.go`, `server/cmd/apicheck` | Regel 7 |
| Redaktionsliste der Logs | `server/internal/logging` | Regel 10 |

Alles zusammen ist eine zusätzliche Vertragsänderung, keine brechende. Die Ton- und
Spur-Ergänzungen aus K26 5.2 und 5.3 gehören nicht dazu; sie bleiben bei der TV-App.

### 15.2 Neu im Repo

| Was | Wo | Stufe |
|---|---|---|
| Zwei Dienste, Vermittlung und Push-Weiterleitung: ein Go-Modul, zwei Programme, zwei Images, je Dienst eine Compose-Datei mit Umgebungsdatei und TLS-Profil, Betriebsanleitung samt Umzug | `relay/` | 2 |
| Android-Projekt mit gemeinsamen Bausteinen und der Handy-App | `android/` | 3 bis 9 |
| drittes Ausgabeformat der Design-Tokens | `packages/design-tokens` | 3 |
| Namensraum für App-Texte in allen Sprachen | `web/src/locales` | 3 bis 9 |
| CI-Jobs `android` und `relay`, Release-Workflow für `relay-v*` (zwei Images), Tasks, Werkzeuge | `.github/workflows/`, `Taskfile.yml`, `docs/dev/SETUP.md` | 2 und 3 |
| Hilfeseite für den QR-Code und Datenschutzerklärung | Website des Projekts | 9 und 10 |

## 16. Risiken

| Nr. | Risiko | Folge | Gegenmittel |
|---|---|---|---|
| R-1 | Dritte Oberfläche neben Web und TV | jede Zuschauer-Funktion dreifach, dauerhaft | App-Teil als fester Abschnitt in jedem künftigen Konzept; gemeinsame Bausteine in `android/core` |
| R-2 | Token ohne Ablauf über offenes HTTP (E-15, E-18) | mitgelesenes Token gilt, bis das Gerät abgemeldet wird; in einem fremden Netz mit denselben Adressen geht es ohne Zutun an ein fremdes Gerät (4.6) | Warnung, dauerhafte Anzeige «unverschlüsselt», «Meine Geräte», Passwort ändern (F-24); der Server weist sich aus (F-38) |
| R-3 | Zwei Dienste als dauernde Betriebspflicht, vorläufig auf eigener Hardware hinter dem vorhandenen Reverse-Proxy (E-17, E-19, E-21, E-22) | Ausfall trifft alle Nutzer; am eigenen Anschluss ist ein Port offen, und die Namen der Dienste zeigen auf ihn (bekannte Folge von E-22) | Trennung der zwei Container vom übrigen Netz, feste Namen, Überwachung von aussen, Umzug auf Mietserver ohne Änderung an Apps und Server (7.5) |
| R-4 | Firebase-Schlüssel bei der Push-Weiterleitung | wer ihn erbeutet, schickt Nachrichten an alle Handys mit der App | Schlüssel nur als Datei und nur auf der Maschine der Push-Weiterleitung, nicht bei der Vermittlung (E-21); die App folgt nur selbst entschlüsselten Meldungen (7.4) |
| R-5 | Abhängigkeit von Googles Push-Dienst und Bibliotheken | kein Push ohne Google-Dienste; Lizenzfrage offen | Bauart `foss` (F-33); Liste in der App, E-Mail und Telegram bleiben |
| R-6 | Vertrag bricht eine installierte App | App geht nach einem Update des Servers nicht mehr | Verträglichkeitsregel (11.5), Feld in `getHealth`, Test über `x-clients` |
| R-7 | Bearer im Server ist ein neuer Anmeldeweg | ein Fehler dort trifft alle Operationen | Erlaubnisliste im Vertrag, Test über alle Operationen je Sitzungsart, Rollen `admin` und `owner` für Bearer gesperrt |
| R-8 | Abruf grosser Dateien im Hintergrund | Android beendet den Abruf, Downloads bleiben hängen | Stufe 0 misst; Fortsetzen an der Stelle; Token ohne Ablauf |
| R-9 | Zugriff aufs lokale Netz ab Android 17 | Heimserver nicht erreichbar ohne Berechtigung | Erklärung vor der Frage, Erkennen der Verweigerung (Abschnitt 6) |
| R-10 | Google lehnt die App ab oder verlangt mehr | öffentliche Stufe verzögert sich | private Verteilung hängt nicht davon ab; neutraler Store-Text; Vorführserver |
| R-11 | 12 Tester über 14 Tage; Identitätsprüfung dauert | öffentliche Stufe und interne Testspur später | bis dahin ADB; Test früh beginnen (11.3) |
| R-12 | Zweite Werkzeugkette und zwei neue CI-Jobs | längere CI, mehr Pflege | Laufzeit in Stufe 3 messen, danach Pfadfilter |
| R-13 | Texte in 22 Sprachen auch für die App | mehr Übersetzungsarbeit je Funktion | eine Quelle, ein Übersetzungsweg; nur benötigte Namensräume |
| R-14 | Offline zweimal gebaut (PWA und App) | Abweichungen im Verhalten | dieselben Testfälle (Abschnitt 12), ein Vertrag |
| R-15 | Öffentlicher Entwicklername | Marcos bürgerlicher Name und Land stehen im Store | vor Stufe 10 bewusst entscheiden; ein Firmenkonto wäre der andere Weg |
| R-16 | Die TV-App wartet (E-12) | späterer Start der TV-App | TV-App kann nach Stufe 3 beginnen (Abschnitt 13) |
| R-17 | Die Verträglichkeitsregel deckt den ganzen Nutzerbereich (11.5) | jede brechende Änderung an einer Zuschauer-Operation braucht eine Übergangszeit; der Server wird langsamer änderbar | Regel erst mit der öffentlichen Stufe verbindlich; bis dahin üben |
| R-18 | Auto-Downloads ohne offene App (9.3) | neue Folgen kommen erst beim Öffnen der App | Stufe 0 misst den Abruf in kurzen Läufen; sonst Abruf beim Öffnen wie in der PWA |
| R-19 | Untergeschobenes Paket beim Einrichten per Scan (7.2) | ein Fernseher wird mit einem fremden Server verbunden | Abholgeheimnis, Anzeige von Server und Konto nach dem Einlösen, Warnung der Handy-App, wenn am Treffpunkt schon ein Paket liegt oder schon eines abgeholt wurde (Abschnitt 10) |
| R-20 | Umschalten beim Umzug (7.5) | eine Kopplung scheitert einmal; bis das Zertifikat steht, scheitern Verbindungen kurz; ohne den Schlüssel zum Versiegeln bleiben Meldungen aus | kurze Lebensdauer im DNS von Anfang an; der Schlüssel zieht mit um; 503 statt 410 bei falschem Schlüssel (D-10); Umzugsprobe in Stufe 2 |

## 17. Fragen

Bis zur Antwort gilt die Empfehlung. Die Bau-Session legt offene Fragen vor ihrer Stufe vor.

**Aus Konzept 26 übernommen** (dort F-1 bis F-21):

| Nr. | Frage | Empfehlung | Fällig vor |
|---|---|---|---|
| F-1 | App im selben Repo (`android/`) oder in einem eigenen? | selbes Repo | Stufe 3 |
| F-3 | Bestätigung im Browser per Code als dritter Weg neben Scan und Eintippen? | **Abgeleitet, nicht entschieden: nein** (K26 F-3). E-16 fragte, wer per QR-Scan verbinden darf, nicht nach einem Code im Browser | Marcos Freigabe |
| F-6 | Unverschlüsseltes HTTP zum Server erlauben? | **Für die Handy-App beantwortet: «HTTP überall mit Warnung» (E-18).** Für die TV-App stellt K26 die Frage | – |
| F-7 | Paketname der App | **Beantwortet: `com.slouchcat.app`, eine App (E-20)** | – |
| F-9 | Übergangszeit für brechende Änderungen an Operationen mit `x-clients` | 90 Tage, und 30 Tage nach einer App-Version, die das Neue kann (11.5); K26 empfahl zwei Minor-Releases | Stufe 10, empfohlen ab Stufe 3 |
| F-10 | Vorführserver für Googles Prüfer: wo und mit welchem Inhalt? | kleiner Server im Internet mit frei lizenzierten Testfilmen, nur für die Prüfung | Stufe 10 |
| F-11 | Hochladen in den Play Store von Hand oder aus der CI? | von Hand, solange privat | Stufe 5 |
| F-12 | FFmpeg in der App? | nein; der Server rechnet um | Stufe 5 |
| F-13 | Kleinste Android-Version der Handy-App | Android 9 (API 28). Android 14 und neuer bekommen den Abruf ohne Zeitgrenze, ältere den Vordergrunddienst. Wie viele Geräte das ausschliesst, ist unbelegt | Stufe 3 |
| F-18 | Meldekanäle, Sehbericht und Kontodaten auch am Fernseher? | nein (K26); die Antwort bestimmt, an welchen Operationen `tv` steht (4.4) | Stufe 1 |
| F-19 | PIN-Empfehlung aus E-9 auch in der Web-App und der Handy-App zeigen? | ja, gleicher Hinweis an gleicher Stelle; betrifft «Profile verwalten» | Stufe 4 |
| F-20 | Wo läuft der Dienst, und wer merkt einen Ausfall? | **Ort und Zugang beantwortet (E-19, E-21, E-22).** Überwachung: je Dienst eine Prüfung von aussen mit Meldung an Marco (7.5) | Stufe 2 |
| F-21 | Adresse der Vermittlung in den Apps änderbar? | für die Vermittlung ja, in den erweiterten Einstellungen; für Push nein (7.5) | Stufe 9 |

**Neu in diesem Konzept:**

| Nr. | Frage | Empfehlung | Fällig vor |
|---|---|---|---|
| F-22 | Klickbares Mockup der Handy-App vor dem Bau? | ja | Stufe 4 |
| F-23 | Darf nur das Hauptprofil (oder eine Sitzung ohne Profil) einen Fernseher verbinden? | ja | Stufe 1 |
| F-24 | Eine Passwortänderung entzieht heute jede andere Sitzung. Soll das auch für Handys und Fernseher gelten, obwohl sie «nie» ablaufen? | ja; es ist der einzige Weg, alles auf einmal abzumelden. Folge: Nach einer Passwortänderung wird jeder Fernseher neu verbunden | Stufe 1 |
| F-25 | Löscht das Abmelden in der App die Downloads auf dem Gerät? | ja, nach Rückfrage | Stufe 6 |
| F-26 | Downloads auf die SD-Karte? Eigene Obergrenze auf dem Gerät? | beides nicht in der ersten Ausgabe; die App zeigt belegten und freien Platz | Stufe 6 |
| F-27 | «Original» als vierte Download-Stufe (Datei unverändert, auch HEVC und HDR)? | nicht in der ersten Ausgabe; braucht eine neue Art von Download im Server | nach Stufe 9 |
| F-28 | Bild-in-Bild für Video? | nicht in der ersten Ausgabe | nach Stufe 9 |
| F-29 | Übertragen auf einen Fernseher (Google Cast) und Android Auto? | nicht in der ersten Ausgabe | nach Stufe 9 |
| F-30 | Trailer: YouTube öffnen oder in der App einbetten? | YouTube öffnen | Stufe 4 |
| F-31 | Mehrere Server in einer App? | nein, eine Adresse | Stufe 3 |
| F-32 | Sperre der App mit Fingerabdruck oder Geräte-PIN, weil die Anmeldung nie abläuft? | nicht in der ersten Ausgabe; die PINs der Profile bleiben | nach Stufe 9 |
| F-33 | Zweite Bauart `foss` ohne Bibliotheken von Google? | ja, als Bauart im Projekt; veröffentlicht wird nur `play` | Stufe 3 |
| F-34 | Wie oft gleicht die App im Hintergrund ab, damit Auto-Downloads auch bei geschlossener App kommen? | etwa alle 6 Stunden, nur mit Netz | Stufe 6 |
| F-35 | Wer sind die 12 Tester für den geschlossenen Test? | Familie und Freunde aus der internen Testspur; früh anfragen | Stufe 10 |
| F-36 | Datenschutzerklärung: wer schreibt sie, wo steht sie? | Entwurf durch die Bau-Session, Prüfung durch Marco, Seite auf der Website des Projekts. Das Papier ersetzt keine Rechtsberatung | Stufe 10 |
| F-37 | Namen der zwei Dienste | je eine eigene Subdomain von `slouchcat.com`, fest in Apps und Server, von Anfang an mit kurzer Lebensdauer im DNS (7.5) | Stufe 2 |
| F-38 | Soll die App bei einem Server über HTTP in jedem neuen Netz zuerst prüfen, dass es der richtige ist, bevor sie Token, Medien-Adressen, Gerätekennung oder Passwort sendet (Abschnitt 6)? Sonst gehen sie in einem fremden Netz mit denselben Adressen ohne Zutun an ein fremdes Gerät (4.6). Die Frage zu E-18 nannte diese Folge nicht. **Die Prüfung schützt nicht** vor Mitlesen im richtigen Netz und nicht, wenn der echte Server auch aus dem Internet erreichbar ist; dagegen hilft nur HTTPS. Wechselt der Schlüssel des Servers (Neuinstallation, Wiederherstellung ohne Geheimnisse), muss der Nutzer ihn in der App neu bestätigen. | ja | Stufe 1 |
| F-39 | Der getrennte Betrieb auf zwei Maschinen ist entschieden (E-21). Offen: zwei Programme, oder ein Programm, das je Maschine nur einen Teil einschaltet? | zwei Programme aus einem Go-Modul, zwei Images (Abschnitt 7); bis Fassung 4 war ein Programm empfohlen | Stufe 2 |
| F-40 | Sollen beim Abmelden und bei einer Passwortänderung auch die Push-Abos von **Browsern** fallen? Heute überleben sie; ein Browser trägt sich nicht von selbst neu ein. | ja, zusammen mit einem Neu-Eintragen der Web-App nach der Anmeldung; nicht in Stufe 1 | nach Stufe 1 |
| F-41 | Zertifikat beim Umzug eines Dienstes: die kurze Lücke nach dem Umstellen hinnehmen, oder das Zertifikat vorab über einen Nachweis im DNS ausstellen (ein weiteres Geheimnis auf dem Mietserver)? | die Lücke hinnehmen (7.5) | Stufe 2 |

## 18. Was sich in Konzept 26 ändern muss

**Erledigt.** Konzept 26 hat mit seiner Fassung 4 alle 20 Punkte aus 18.1 und alle neun Befunde
aus 18.2 eingearbeitet; seine Fassung 5 (Stand `88c85bfb`) arbeitet eine dritte Lesung ein
(REVISE, 0 Blocker, 1 Major, 8 Minor). Was diese Lesung diesem Papier zugewiesen hat, steht in
19.2. Mit dem Stand `82c21a63` hat Konzept 26 auch die zwei Dienste aus 18.3 übernommen. Die
Listen bleiben als Nachweis stehen; 18.1 und 18.2 beschreiben Konzept 26 im Stand `b3296ee6`.
Offen ist dort nur noch: Der Kopf von Konzept 26 nennt dieses Papier mit einem älteren Stand,
und es führt die Entscheide vom 20. November noch ohne die Nummern E-21 und E-22.

Konzept 26 liegt auf seinem eigenen Zweig und wird hier nicht angefasst. Der Kritiker hat
Abschnitt 4 von Konzept 26 mit diesem Papier erstmals gelesen; seine Befunde stehen in 18.2.

### 18.1 Folgen aus diesem Konzept

1. **Kopf und Titel:** Verweis auf Konzept 27; «Kopplung per Code» im Titel ist seit E-10
   überholt. ADR 0039, 0040 und 0042 entstehen mit dem Bau von Konzept 27 und bekommen weitere
   Titel: «Android-Apps als native Clients im selben Repo», «Sitzungsarten für native Apps und
   Gerätekopplung», «Vermittlung im Internet; Betrieb und Umzug der zwei Dienste». Die
   Push-Weiterleitung beschreibt ADR 0044.
2. **2.6 und 8.1, 8.2:** Ziel-API für Handy 36, für TV 34. Der Test mit 12 Testern fällt mit der
   Handy-App an; ob er für die TV-Geräteart erneut anfällt, ist unbelegt. Die TV-Geräteart
   bekommt eine eigene Veröffentlichungsspur; die Wahl ist unumkehrbar. 2.6 nennt die interne
   Testspur «ohne Vorbedingung», 8.1 «Sobald Google die Identität bestätigt hat»; das
   widerspricht sich und ist unbelegt.
3. **3.2 und 9:** Die TV-App ist das Modul `app-tv` im Projekt `android/` aus Konzept 27 und
   nutzt dessen Bausteine. Paketname `com.slouchcat.app` (E-20). Das Messen des Emulators in der
   CI und der Entscheid über einen Pfadfilter liegen bei Stufe 3 von Konzept 27.
4. **3.5:** «Android Lint meldet festen Text als Fehler» steht dort als Tatsache; für Compose
   ist das unbelegt (hier 5.6).
5. **4.2 und 4.5:** drei Sitzungsarten statt zwei (hier 4.1), die Art an den Träger gebunden;
   die Angabe im Vertrag ist eine Liste `x-clients` statt eines Ja oder Nein (hier 4.4). Der
   letzte Punkt von 4.2 («eine Spalte oder eine eigene Regel je Sitzungsart») ist entschieden:
   ein Ablauf weit in der Zukunft, den `TouchSession` nicht verschiebt (hier 4.3).
6. **4.2, Aufräumung:** Ein Fernseher ohne Kontakt seit 400 Tagen wird nicht abgemeldet, aber
   seine Gerätezeile wird gelöscht und die Sitzung bekommt eine neue Kennung (hier 2.3). Das
   behebt Stufe 1 von Konzept 27 (hier 4.3).
7. **4.2, Abmelden:** «bleibt angemeldet, bis ihn jemand unter ‹Meine Geräte› oder in der
   Verwaltung abmeldet» lässt die Passwortänderung aus; sie entzieht heute jede andere Sitzung
   (hier F-24).
8. **4.3:** «Welche Sitzungen einen Gutschein ausstellen dürfen» ist beantwortet: nur die
   App-Sitzung (E-16). Schritt 6 und die Zeile «Gutschein einlösen» der Tabelle: Statt eines
   Schlüsselnachweises verschlüsselt der Server das Gerätetoken für den Schlüssel des Fernsehers
   (hier 7.2). Der Fernseher zeigt nach dem Einlösen Server und Konto.
9. **4.4:** «Gerät mit Passwort anmelden» und «App anmelden» sind eine Operation (hier 4.2).
10. **4.6:** K-1 bekommt das Abholgeheimnis (hier 7.1); K-7 nennt Push als weiteren Kanal.
11. **4.7:** Der Dienst hat zwei Aufgaben (E-17). «Was er sieht» und «Fällt er aus» verweisen
    auf 7.6 und 7.7 hier. F-20 ist beantwortet (E-19). «In den Einstellungen änderbar» gilt nur
    für die Vermittlung, und beide Geräte müssen denselben Dienst eingestellt haben (hier 7.5).
12. **7:** Die Zeilen Bearer, Gerätetoken, Kopplung, «Gerät mit Passwort anmelden», «Meine
    Geräte», Meldung, Sitzungen, Vermittlungsdienst, `getHealth` und Redaktionsliste wandern
    nach Konzept 27 (hier 15.1). Bei der TV-App bleiben `PlaybackCapabilities` und Ton.
13. **8.4:** Die Übergangszeit nach Zeit statt nach Releases (hier 11.5).
14. **10, Stufentabelle:** neu schneiden.
    - Stufe 0 bleibt.
    - Stufe 1 schrumpft auf Ton und Fähigkeiten (5.2, 5.3).
    - Stufe 2a schrumpft auf das Modul `app-tv` mit Einrichten per Scan (QR-Code zeigen, Paket
      holen, einlösen), Einrichten von Hand und Profilwahl. Sie braucht die Stufen 1 bis 3 von
      Konzept 27, für den Scan auch 2 und 9.
    - Stufe 3: Die interne Testspur und der Paketname stehen dann schon.
    - Stufe 6: Der geschlossene Test und der Vorführserver sind mit der Handy-App schon erledigt
      oder laufen.
15. **11:** Regel 11 nennt beide Apps; die Ausnahme «Cloud» umfasst auch die
    Push-Weiterleitung. Die Zeilen zu Plan 3.8 Punkt 7 («nur als Gerätetoken»), zum Nachtrag von
    ADR 0005 und zu Regel 7 nennen die App-Sitzung und `x-clients`.
16. **12:** R-11 («Bearer nur für Gerätetoken») nennt beide Bearer-Arten. R-13 nennt, dass die
    TV-App nach Stufe 3 von Konzept 27 beginnen kann.
17. **13:** F-7 und F-20 sind beantwortet; F-6 ist für die Handy-App beantwortet und für die
    TV-App offen; F-1, F-9, F-11, F-12, F-13 und F-21 stehen in Konzept 27; F-10 wird mit der
    Handy-App zuerst fällig.
18. **Neu für die TV-App:** Mit Ziel-API 37 braucht auch sie die Berechtigung für das lokale
    Netz (hier 2.7).
19. **5.2:** Der Name aus `PlaybackCapabilities` («Chromecast · Android TV 14») überschreibt den
    Gerätenamen nicht mehr (hier 4.2). Es bleibt der Name, den der Fernseher beim Einrichten
    nennt.
20. **4.6:** Falls F-38 mit Ja beantwortet wird, prüft auch die TV-App den Server, bevor sie
    etwas Geheimes sendet; der Schlüssel reist im Paket der Kopplung mit (hier Abschnitt 6).

### 18.2 Befunde des Kritikers zu Abschnitt 4 von Konzept 26

Erste Lesung dieses Abschnitts, am 19. November 2026 zusammen mit diesem Papier: **2 Major,
7 Minor.** Die Spalte «Stand» sagt, was dieses Papier schon löst; der Rest ist in Konzept 26
nachzutragen.

| Nr. | Stufe | Befund | Stand |
|---|---|---|---|
| 1 | Major | 4.3, 4.6: Nichts hindert jemanden, der die Treffpunkt-Kennung kennt (Foto des QR-Codes, der Dienst selbst), dem Fernseher ein eigenes Paket unterzuschieben und ihn mit einem fremden Server zu verbinden. Der «Schlüsselnachweis» ist nicht beschrieben; «nach dem Abholen verbraucht» erlaubt einem Fotografen, den Treffpunkt zu verbrauchen. K-1 bis K-8 nennen kein Gegenmittel | hier 7.1 (Abholgeheimnis), 7.2 (verschlüsseltes Token, Anzeige von Server und Konto, bekannte Grenze); K26 4.6 muss beides übernehmen |
| 2 | Major | 4.5: Die Erlaubnisliste der Gerätesitzung zählt die `scoped`-Operationen nicht auf («zum Schauen, Hören, Suchen, Merken und Wünschen»). Offen sind zum Beispiel Meldungen als gelesen markieren, Empfehlungen an andere Profile und der Offline-Abgleich | offen; `x-clients` (hier 4.4) verlangt je Operation eine Antwort für `tv`, K26 braucht die Tabelle |
| 3 | Minor | 4.2: Abmelden lässt die Passwortänderung aus; die Aufräumung ist falsch beschrieben | hier 2.3, 4.3, F-24 |
| 4 | Minor | 4.2: «Kommen Cookie und Bearer zusammen» meint das Sitzungs-Cookie, nicht das Geräte-Cookie. HTTP (F-6) mit einem Token ohne Ablauf (F-4) gehört als bekannte Grenze nach 4.6 | hier 4.3, 4.6; für die TV-App nachzutragen |
| 5 | Minor | 4.2: «der Radio-Start» ist eine JSON-Operation; die Streaming-Routen des Radios sind andere (`binaryRoutes`) | offen |
| 6 | Minor | 4.4: Der Zähler für Fehlversuche ist ein Topf für Anmeldung, Ersteinrichtung, PIN und Einladungen und zählt je Adresse | hier 2.3, 4.2 |
| 7 | Minor | 4.6, K-7: Die Meldung nennt einen Gerätenamen, den der Angreifer wählt | hier Abschnitt 10 |
| 8 | Minor | 4.7: «in beiden Apps änderbar» verträgt sich nicht damit, dass die Handy-App einen fremden Dienst im QR-Code ablehnt, und nicht mit Push | hier 7.5 |
| 9 | Minor | 4.3: Das Format des QR-Codes ist nicht beschrieben | hier 7.2 |

### 18.3 Nachtrag vom 20. November 2026: was Konzept 26 zu den zwei Diensten nachzieht

Aus E-21 und E-22 (1.3) folgt für K26 4.7 (dort mit dem Stand `82c21a63` nachgezogen):

1. Es sind zwei getrennt betriebene Dienste. Der Fernseher spricht nur mit der Vermittlung; die
   Push-Weiterleitung betrifft ihn nicht.
2. Der QR-Code nennt die Adresse der Vermittlung. «Adresse änderbar» (K26 F-21, hier 7.5) meint
   nur sie.
3. Betrieb, Umzug und die bekannten Folgen von E-22 stehen hier in 7.5; K26 verweist darauf.
4. Beim Umschalten eines Umzugs kann eine Kopplung einmal scheitern (hier 7.5). K26 4.3 deckt
   das schon: Der Fernseher zeigt dann einen neuen Code.

## 19. Kritiker-Review (Fassung 1)

Ein getrennter Kritiker hat Fassung 1 am 19. November 2026 gegen den Code am Anker gelesen,
zusammen mit Abschnitt 4 von Konzept 26: **REVISE, 0 Blocker, 8 Major, 17 Minor.** Sein Urteil
zum Kern: Drei Sitzungsarten, Erlaubnisliste, Gutschein am Schlüssel des Fernsehers,
Abholgeheimnis und Web-Push zum Dienst tragen; die Fehler lagen in den Einzelheiten.

Die Redaktion hat die tragenden Befunde am Code nachgeprüft, bevor sie eingearbeitet wurden:
die Satzgrösse der Push-Bibliothek, `push_subscriptions.device_id`, `TouchSession`,
`RecordDeviceProfile`, `netguard.RefuseLocal`, `RotateVAPID`, `RevokeOtherAccountSessions`,
`loadSession` mit `assignDevice`, `download_tombstones`, `ChannelTargetsAdmins`, den gemeinsamen
Topf der Fehlversuche und die Tags vom 15. November. Alle haben gestimmt.

| Nr. | Stufe | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Major | Push scheitert an der Grösse bei **jeder** Meldung: Die Bibliothek füllt jeden Körper auf 4096 Byte auf; den Text zu kürzen ändert nichts | 2.5, 7.3 (Satzgrösse), 15.1, Stufe 0 |
| 2 | Major | Ein abgemeldetes oder verlorenes Handy bekommt weiter jede Meldung des Kontos: Push-Abos überleben `logout`, `deleteDevice` und Passwortänderung | 2.5, 4.5, 7.3, 15.1 |
| 3 | Major | Der Träger war nicht an die Sitzungsart gebunden: Ein Browser-Token als Bearer wäre eine App-Sitzung geworden (Weg um E-16), ein App-Token im Cookie hätte einen Ablauf bekommen | 4.1 |
| 4 | Major | E-18 und private Adressen: In einem fremden Netz mit demselben Adressbereich geht das Token ohne Zutun an ein fremdes Gerät; weder 4.6 noch die Frage zu E-18 nannten das | 1.2, 4.6, 6 (der Server weist sich aus), F-38, R-2 |
| 5 | Major | Der 403-Test bewies nichts über E-14: Ein gewöhnliches Konto scheitert schon an der Rollenprüfung | 4.4, 12 |
| 6 | Major | Stufe 1 hängt an Stufe 0 (Verschlüsselung, Push-Grösse), die Tabelle sagte es nicht | 13 |
| 7 | Major | Auto-Downloads im Hintergrund starten womöglich nicht: kein Vordergrunddienst aus dem Hintergrund | 9.3, R-18, Stufe 0 |
| 8 | Major | 15.1 unvollständig: `logout` mit Bearer, Abfragen auf `expires_at`, neue Art von Meldung samt Texten, Abgleich des Passworts mit Seerr, reservierte Gerätekennung, Aufräumen beim Abmelden, Sitzungsart in der Verwaltung, Register des Vertragstests und `apicheck` | 2.3, 4.2, 4.3, 15.1 |
| 9 | Minor | Zuschreibung: Satz zum Paketnamen (stand im Fragetext, nicht in der Option); «zweiter Dienst» gegen ein Programm mit zwei Teilen | 1.2, 7, F-39 |
| 10 | Minor | Aufräumung nach 400 Tagen falsch beschrieben: Das Gerät wird nicht abgemeldet, es bekommt eine neue Kennung; Widerrufe bleiben unter der alten | 2.3, 4.3, 18.1 |
| 11 | Minor | Ungenau in 2.3: PIN-Zähler bei `deleteDevice`, Widerrufsliste ohne Fremdschlüssel, Name bei jedem Abspielstart, `devices.account_id`, Passwortänderung entzieht schon heute | 2.3, 4.2, 4.5, F-24 |
| 12 | Minor | Push: Der Server sendet nicht an private Adressen; der Profilname kommt aus `Emit`; `url` ist eine Web-Adresse | 2.5, 7.3, 7.5 |
| 13 | Minor | Lebenszyklus der Abos: erstes Eintragen, neues Google-Token, neuer VAPID-Schlüssel, Prüfungen des Dienstes | 7.3, 7.4 |
| 14 | Minor | Administratoren bekommen Meldungen mit Zielen unter `/admin` | 2.5, 7.3 |
| 15 | Minor | Gerätekennung vom Client: Anhängen an ein fremdes Gerät | 4.2 |
| 16 | Minor | «schützt das Token ohne TLS» galt nur für die eine Antwort | 7.2 |
| 17 | Minor | Untergeschobenes Paket fehlte in der Analyse; Hilfeseite ohne fremde Skripte | 7.2, 7.4, R-19 |
| 18 | Minor | Offline: Verifier der normalen PIN, Uhrenkorrektur, Titelbild braucht die Sitzung, Abmelden muss im Server löschen | 9.1, 9.3, 9.5, 9.7 |
| 19 | Minor | Stufe 0 ist schon Arbeit an einer nativen App; Frage 35 kann vor Frage 34 stehen | 13, 14 |
| 20 | Minor | 22 Sprachen und eine Ergänzung; `_zero`; `px` | 2.6, 5.4, 5.5 |
| 21 | Minor | CI: `main` überspringt Geprüftes; Ausnahmen des Dokumentfilters | 2.6, 5.7 |
| 22 | Minor | Die Verträglichkeitsregel deckt den ganzen Nutzerbereich; sechs Releases am 15. November | 11.5, R-17 |
| 23 | Minor | «Meine Geräte» zeigte kein Gerät, das nur noch Downloads hält | 4.5 |
| 24 | Minor | `relay/` kann `server/internal/logging` nicht einbinden | 5.7 |
| 25 | Minor | Öffentliches Repo: Der Wortlaut von E-19 nennt den Ort eines Dienstes, der vom Internet aus erreichbar ist | 1.2, 7.5 (sinngemäss statt wörtlich) |

**Nicht eingearbeitet, sondern als Frage gestellt:** Befund 4 schlägt zwei Wege vor (das Netz
wiedererkennen oder das Token nie über HTTP senden). Das Wiedererkennen ist verworfen
(Abschnitt 6). «Der Server weist sich aus» ist ein dritter, kleinerer Weg und steht als
Vorschlag mit Frage F-38; er schützt nur vor dem fremden Gerät, nicht vor dem Mitlesen.

**Vom Kritiker geprüft und bestätigt:** die Zahl der Operationen (307 = 179, 92, 24, 8, 4) und
die Vertragsfassung; `bearerToken` ohne Nutzer; Sitzung mit 30 Tagen und `expires_at NOT NULL`;
die Middleware liest nur das Cookie; Geräte-Cookie und Vorrang des Geräts der Sitzung;
`checkOfflineRevocations` öffentlich und nur am Cookie; `DeleteStaleDevices` bei jeder
Anmeldung; `dt` ohne Ablauf mit Prüfung je Anfrage; 24 Stunden und 7 Tage; Budget 100 GB; die
Bitraten für Ton; keine Bildspuren offline; Web-Push an alle Abos des Kontos, Nutzlast,
`maxBody`, 404 und 410; Jobs und Gate der CI; Release nur auf `v*.*.*`; Werkzeugversionen;
Tokens und Texte; die Auslöser des Abgleichs in der PWA.

**Offen geblieben (ohne Wertung des Kritikers):** ob Google Datenschutzerklärung und
«Datensicherheit» schon für die interne Testspur verlangt; ob ein Übertragungsauftrag
Wegwischen und Neustart übersteht (Stufe 0). Zur kleinsten Android-Version: Handy- und TV-App
sind zwei Artefakte und können je eine eigene haben.

### 19.1 Zweite Lesung (Fassung 2)

Derselbe Kritiker hat Fassung 2 am 19. November 2026 gegengelesen: **REVISE, nahe an der
Annahme mit Vorbehalt; 0 Blocker, 1 Major, 13 Minor.** Von den 25 Befunden der ersten Lesung
galten 19 als erledigt und sechs als nur teilweise (Nr. 4, 11, 12, 13, 15, 20). Nummerierung,
Querverweise, die neuen Aussagen über den Code und die Wiedergabe seiner Befunde zu Konzept 26
(18.2) hat er bestätigt. Die Redaktion hat die drei neuen Fundstellen am Code nachgeprüft
(`absolute` in `notify/texts.go`, die Prüfung «Kanal eingeschaltet» in `deliver.go`, der Knopf
in `PushDevices.tsx`).

| Nr. | Stufe | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Major | «Der Server weist sich aus» deckte nur das Bearer-Token: nicht die Medien-Adressen mit `dt`, `st` und `rt`, nicht die Gerätekennung, nicht das Passwort bei neuer Anmeldung; die Prüfung «nach dem Start oder Netzwechsel» war zu grob; ein fremdes Gerät kann die Prüfung an den echten Server weiterreichen; die neue Operation braucht Ratenbegrenzung und Test; eine erbeutete Sicherung erlaubt, sich als Server auszugeben | 4.6, 6 (Schlüsselpaar des Servers, Sperre im gemeinsamen Client je Netz, was es nicht leistet), 12, 15.1, F-38 neu gefasst |
| 2 | Minor | Unbelegte Aussage zu Android (Standort-Berechtigung) ohne Kennzeichen und ungenau | 6 |
| 3 | Minor | 9.1 widersprach 4.5, 9.7 und 15.1: Auch das Abmelden ändert den Server für Offline | 9.1 |
| 4 | Minor | 2.5 nannte zwei Wege, auf denen ein Abo fällt; es sind vier | 2.5 |
| 5 | Minor | Aufräumen der Push-Abos: Reihenfolge bei `deleteDevice`, Abos ohne Gerätekennung, Folge für Browser bei der Passwortänderung, Ersetzen nur bei App-Sitzungen | 4.5, 7.3, 15.1, F-40 |
| 6 | Minor | 15.1: woran der Server eine Adresse des Dienstes erkennt; Ratenbegrenzung und Register für F-38; was ein verschwundenes Gerät für die Sitzung heisst | 4.3, 7.3, 15.1 |
| 7 | Minor | 7.3: Die App nimmt aus `url` nur den Pfad und öffnet nie einen fremden Rechnernamen | 7.3 |
| 8 | Minor | 18.1 fehlten zwei Punkte: Ablauf weit in der Zukunft (K26 4.2), Gerätename gegen Fähigkeitsname (K26 5.2) | 18.1 Punkt 5 und 19 |
| 9 | Minor | 3.1: Die Folgerung zu Wünschen stand unter Marcos Entscheid | 3.1 |
| zu 11 | Minor | 2.3 sagte noch, die Widerrufsliste hänge am Gerät | 2.3 |
| zu 12 | Minor | `url` ist nur mit öffentlicher Adresse des Servers vollständig | 2.5, 7.3 |
| zu 13 | Minor | Den Kanal Push einzuschalten bleibt dem Hauptprofil vorbehalten; ein Abo allein bringt keine Meldung | 2.5, 7.3 |
| zu 15 | Minor | Mitgebrachte Gerätekennung: Prüfung auf Sitzungen **oder Downloads** eines anderen Kontos | 4.2 |
| zu 20 | Minor | «23 Sprachen» stand noch an zwei Stellen | 12, R-13 |

Zu Befund 1: Der Kritiker hat als stärkere Fassung das Schlüsselpaar des Servers vorgeschlagen
(Vertrauen beim ersten Kontakt). Sie ist jetzt der Vorschlag in Abschnitt 6. Die Grenzen, die
bleiben, stehen dort und im Text von F-38; gegen das Mitlesen hilft nur HTTPS.

Die Einarbeitung der zweiten Lesung hat der Kritiker nicht mehr gegengelesen.

### 19.2 Nachtrag aus der dritten Lesung von Konzept 26 (Fassung 4)

Ein Kritiker hat Konzept 26 in dessen Fassung 4 gegen den Vertrag und gegen dieses Papier
gelesen (REVISE, 0 Blocker, 1 Major, 8 Minor; K26 Abschnitt 14, vierter Nachtrag). Vier Befunde
betreffen dieses Papier. Marco hat die Weitergabe freigegeben («Ja, weitergeben»); ein neuer
Entscheid ist keiner davon. Die Redaktion hat Anhang B von Konzept 26 gegen den Vertrag
nachgezählt: 116 Operationen, jede genau einmal, 93 mit `tv`, 23 ohne.

| Nr. | Stufe dort | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Major | Niemand trug `tv` an den 93 Operationen ein, bevor es die Gerätesitzung gibt. Stufe 1 hier schafft und testet sie; ohne die Angabe bekäme ein Fernseher auf alles 403. F-18 aus K26 wird damit vor Stufe 1 fällig | 4.4, 13 (Stufe 1), 15.1, F-18 |
| 2 | Minor | Das Einlösen trug keinen Gerätenamen; die Gerätezeile hätte keinen Namen für «Meine Geräte» und die Meldung aus K-7 | 7.2, Abschnitt 10, 15.1 |
| 3 | Minor | Antwortet der Dienst beim Ablegen mit 409, ist das der Fall «untergeschobenes Paket»; die Handy-App meldete nur nach zwei Minuten «nicht geklappt» | Abschnitt 10, R-19 |
| 4 | Minor | F-19 aus K26 (PIN-Empfehlung auch in Web-App und Handy-App) betrifft die Seite «Profile verwalten» und wird mit Stufe 4 fällig | 4.6, 13 (Stufe 4), F-19 |

Dazu zwei Kleinigkeiten: Kopf und Abschnitt 18 nennen jetzt den Stand von Konzept 26
(Fassung 5), und F-3 steht hier wie dort als «abgeleitet, nicht entschieden» (1.2, Abschnitt 17).

Diesen Nachtrag hat kein Kritiker gelesen.

### 19.3 Nachtrag vom 20. November 2026: zwei Dienste (Fassung 5)

Marco hat am 20. November 2026 E-21 und E-22 entschieden (1.3). Eingearbeitet:

| Was | Wo |
|---|---|
| Zwei getrennt betriebene Dienste statt eines Programms mit zwei Teilen; die Empfehlung zu F-39 hat sich umgekehrt | Abschnitt 7, 5.1, 5.7, 15.2, F-39 |
| Umzug als Anforderung an den Aufbau: dasselbe Image und dieselbe Compose-Datei an beiden Orten, kein Zustand auf der Platte, ein fester Name je Dienst, Liste der Schritte, Folgen beim Umschalten | 7.5, D-1, R-20 |
| Betrieb hinter dem vorhandenen Reverse-Proxy: Adresse des Aufrufers nur aus dem Kopf eines eingetragenen Proxys; der Name zeigt auf den eigenen Anschluss (bekannte Folge von E-22) | 7.4 (D-5, D-7, D-9), 7.5, 7.6, R-3 |
| Einbruch je Dienst getrennt betrachtet; der Firebase-Schlüssel liegt nur bei der Push-Weiterleitung | 7.4, D-6, R-4 |
| Stufe 2 baut beide Dienste und die zwei Container; Umzugsprobe als Beleg | 13 |
| Was Konzept 26 nachzieht | 18.3 |

Abweichung von der Vorlage der Hauptsession, von der Redaktion so gefasst: Auf dem Mietserver ist
die Liste der vertrauten Proxys nicht leer. Dort steht der TLS-Abschluss im selben Stack vor dem
Dienst; er ist der eine eingetragene Proxy.

Der Satz aus Fassung 4, der DNS-Eintrag solle nicht auf einen privaten Anschluss zeigen, ist
gestrichen: Mit E-22 zeigt er vorläufig dorthin.

### 19.4 Dritte Lesung (Fassung 3 bis 5)

Auf Marcos Wort («Lass die Fassung 5 durch einen Kritiker lesen») hat derselbe Kritiker am
20. November 2026 Fassung 5 gelesen und dabei auch die Einarbeitungen aus Fassung 3 und 4
geprüft, die bis dahin niemand gegengelesen hatte: **Annahme mit Vorbehalt; 0 Blocker,
0 Major, 13 Minor.** Sein Urteil: Alle drei Überarbeitungen halten gegen den Code, kein Befund
öffnet eine Frage des Entwurfs neu, die Aufteilung in zwei Dienste trägt. Die Schwächen lagen im
Betrieb.

Bestätigt hat er:

- 13 der 14 Befunde der zweiten Lesung sind erledigt; F-38 war bis auf den Schlüsselwechsel
  vollständig.
- Die Wiedergabe von E-21 und E-22 in 1.3 ist treu; die zitierten Folgen stehen in den Optionen;
  ausserhalb von Abschnitt 1 liest sich nichts als Marcos Entscheid.
- Auf dem Mietserver ist die Liste der vertrauten Proxys nicht leer (19.3).
- Anhang B von Konzept 26: 116 Operationen, 93 mit `tv`, 23 ohne, keine fehlt oder steht doppelt.
- Im Papier steht kein Container, kein Anbieter, kein Name eines Proxy-Produkts und keine
  Adresse. «Der Name zeigt auf den eigenen Anschluss» hält er für vertretbar, weil sich der Name
  ohnehin auflösen lässt, sobald er lebt.

Die Redaktion hat seine Fundstellen am Code nachgeprüft (`compose.https-only.yaml`, den Ordner
für Zertifikate in `deploy/compose.yaml`, `secret.key` in der Sicherung, `deliverAttempts`).

| Nr. | Befund | Eingearbeitet in |
|---|---|---|
| 1 | «Kein Zustand auf der Platte» stimmte auf dem Mietserver nicht (Zertifikate); für den festen Namen gibt es erst nach dem Umstellen ein Zertifikat | 7.5 (Zertifikat beim Umschalten), F-41, R-20 |
| 2 | Eine falsche Schlüsseldatei bei der Push-Weiterleitung hätte über 410 die Abos aller sendenden Server gelöscht; «heilt sich selbst» war wahr, aber langsam | 7.3 (Fassung des Schlüssels, 503 statt 410, Prüfung auch beim Abgleich), D-10, 7.5, R-20 |
| 3 | D-9: Name des Kopfes und Leserichtung; der Port des Dienstes darf auf dem Mietserver nicht am Host liegen; eine Adresse statt eines Bereichs; IPv6 je Netz | 7.4 (D-2), 7.5 |
| 4 | Leitet der Router Anfragen von innen an die eigene öffentliche Adresse nicht zurück, erreicht der eigene Server die Push-Weiterleitung nicht | 7.5 |
| 5 | Die Probe unter einem Probenamen kann kein echter Push von Ende zu Ende sein | 7.5 Schritt 3 |
| 6 | Die Warnung bei 409 verfehlte den häufigsten Ablauf: Das untergeschobene Paket ist meist schon abgeholt, das Handy bekam 404 | 7.1 (410 «schon abgeholt»), Abschnitt 10, 12, R-19 |
| 7 | Ein Einbruch in den Proxy vor den Diensten war nicht betrachtet | 7.4 |
| 8 | Die Umzugsprobe braucht eine zweite, von aussen erreichbare Maschine | 13 (Stufe 2) |
| 9 | F-38: Nach einem Schlüsselwechsel des Servers wäre jede App für immer ausgesperrt | 6, 12, 15.1, F-38 |
| 10 | Ein Tag `relay-v*` braucht ein eigenes Release-Workflow | 5.7, 13, 15.2 |
| 11 | Veraltete Verweise auf Konzept 26 | Kopf, Abschnitt 18 |
| 12 | Reste von «der Dienst» in der Einzahl | Kopf, 6, 14, 18.1 |
| 13 | R-20 stand vor R-19 | 16 |

Die Einarbeitung der dritten Lesung hat der Kritiker nicht mehr gegengelesen.

## Anhang A. Quellen der Recherche (19. November 2026)

Google Play:

- Testpflicht für private Konten: <https://support.google.com/googleplay/android-developer/answer/14151465>
- Testspuren: <https://support.google.com/googleplay/android-developer/answer/9845334>
- Handy und Fernseher in einer App: <https://developer.android.com/training/tv/publishing/distribute>
- Eigene Spur je Geräteart: <https://support.google.com/googleplay/android-developer/answer/13295490>
- Ziel-API: <https://support.google.com/googleplay/android-developer/answer/11926878>
- 16-KB-Speicherseiten: <https://developer.android.com/guide/practices/page-sizes>
- Play App Signing: <https://support.google.com/googleplay/android-developer/answer/9842756>
- Paketname: <https://support.google.com/googleplay/android-developer/answer/113469>
- Zugang für die Prüfung: <https://support.google.com/googleplay/android-developer/answer/15748846>
- Datensicherheit: <https://support.google.com/googleplay/android-developer/answer/10787469>
- Kontolöschung: <https://support.google.com/googleplay/android-developer/answer/13327111>
- Öffentliche Angaben zum Entwickler: <https://support.google.com/googleplay/android-developer/answer/13628312>
  und <https://support.google.com/googleplay/android-developer/answer/13634081>
- Vordergrunddienste erklären: <https://support.google.com/googleplay/android-developer/answer/13392821>
- Entwicklerprüfung: <https://developer.android.com/developer-verification/guides/faq>
- Urheberrecht: <https://support.google.com/googleplay/android-developer/answer/9888072>
- Hochladen über die Schnittstelle: <https://developers.google.com/android-publisher/getting_started>

Android:

- Netzwerk-Sicherheitskonfiguration: <https://developer.android.com/privacy-and-security/security-config>
- Zugriff aufs lokale Netz: <https://developer.android.com/privacy-and-security/local-network-permission>
- Verhaltensänderungen Android 16: <https://developer.android.com/about/versions/16/behavior-changes-16>
- Verhaltensänderungen Android 17: <https://developer.android.com/about/versions/17/behavior-changes-17>
- Zeitgrenze für Vordergrunddienste: <https://developer.android.com/develop/background-work/services/fgs/timeout>
- Vom Nutzer gestartete Übertragungen: <https://developer.android.com/develop/background-work/background-tasks/uidt>
- Media3, Downloads: <https://developer.android.com/media/media3/exoplayer/downloading-media>
- Media3, Wiedergabe im Hintergrund: <https://developer.android.com/media/media3/session/background-playback>
- Media3, Versionen: <https://developer.android.com/jetpack/androidx/releases/media3>
- Compose, Versionen: <https://developer.android.com/develop/ui/compose/bom/bom-mapping>
- Android Gradle Plugin: <https://developer.android.com/build/releases/gradle-plugin>
- Speicher der App: <https://developer.android.com/training/data-storage/app-specific>
- Automatische Sicherung: <https://developer.android.com/identity/data/autobackup>
- `security-crypto`: <https://developer.android.com/jetpack/androidx/releases/security>
- QR-Scanner: <https://developers.google.com/ml-kit/vision/barcode-scanning/code-scanner>
- HPKE in Go: <https://pkg.go.dev/crypto/hpke>
- HPKE in Android: <https://developer.android.com/reference/android/crypto/hpke/Hpke>
- Push senden (FCM): <https://firebase.google.com/docs/cloud-messaging/send/v1-api>
- Push, Arten und Grösse: <https://firebase.google.com/docs/cloud-messaging/customize-messages/set-message-type>
- Schlüssel in der Firebase-Konfiguration: <https://firebase.google.com/docs/projects/api-keys>
- Web-Push über Googles Push-Dienst (abgelehnte Empfehlung zu E-17): <https://unifiedpush.org/kdoc/embedded_fcm_distributor/>
- Emulator auf GitHub-Runnern: <https://github.blog/changelog/2024-04-02-github-actions-hardware-accelerated-android-virtualization-now-available/>
- Generator für Kotlin: <https://openapi-generator.tech/docs/generators/kotlin/>

Unbelegt geblieben: ob der Test mit 12 Testern für eine zweite Geräteart oder eine zweite App
erneut anfällt; ob die interne Testspur die abgeschlossene Identitätsprüfung braucht; ob sich
der Signaturschlüssel aus Play exportieren lässt; die nächste Frist für die Ziel-API; Googles
Wortlaut zu Apps mit eigenem Server des Nutzers; die Einordnung in «Datensicherheit»;
Alterseinstufung und Familienrichtlinie; ob die E-Mail-Adresse eines privaten Entwicklers
öffentlich ist; eine Regel zu Copyleft-Apps; Adressbereiche in der Netzwerk-Konfiguration; ob ein
Übertragungsauftrag Wegwischen und Neustart übersteht; ob eine App aus dem Hintergrund einen
langen Abruf starten kann; die Mehrzahlform für null auf Android; Spulen in fragmentiertem MP4 mit
ExoPlayer; HPKE auf Android vor API 37; Zustellung von Push im Stromsparmodus; das Verhalten von
Googles Bibliothek bei «Benachrichtigungs-Nachrichten»; die Lint-Prüfung für festen Text in
Compose; die Verbreitung der Android-Versionen. Was davon den Bau betrifft, misst Stufe 0.
