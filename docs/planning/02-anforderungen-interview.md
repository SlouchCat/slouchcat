# Deep Interview Spec: Slouchcat – Vision und Anforderungen Version 1

## Metadata
- Interview ID: 1988ea7e-cb27-4242-876d-c7f32bfc0756
- Rounds: 19 (plus Runde 0 Topologie)
- Final Ambiguity Score: 10 %
- Type: greenfield
- Generated: 2026-10-18
- Threshold: 0.2
- Threshold Source: default
- Initial Context Summarized: no
- Status: PASSED
- Sprache: Deutsch (Schweizer Schreibweise)

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 0.90 | 0.40 | 0.360 |
| Constraint Clarity | 0.92 | 0.30 | 0.276 |
| Success Criteria | 0.85 | 0.30 | 0.255 |
| **Total Clarity** | | | **0.891** |
| **Ambiguity** | | | **0.109** |

## Topology
| Component | Status | Description | Coverage / Deferral Note |
|-----------|--------|-------------|--------------------------|
| Konten, Profile und Zugriff | active | Hauptbenutzer (E-Mail-Konto), Netflix-artige Profile, Altersstufen, PIN, Bibliotheksrechte, Schauzeit-Limits, Gäste per Einladung, Remote-Zugriff | Kriterien K-1 bis K-8 |
| Video-Bibliothek und Metadaten | active | Filme und Serien: Scannen, Erkennen, TMDB/TVDB, Versionen, Trailer, Extras, Pflegewerkzeuge | Kriterien B-1 bis B-9 |
| Hörbücher und Podcasts | active | Hörbücher mit Kapiteln, Podcast-Abos per Feed, Fortschritt je Profil | Kriterien H-1 bis H-4; **Musik, Fotos, Live-TV zurückgestellt** (Runde 4) |
| Wiedergabe und Transcoding | active | Direct Play, GPU-Transcoding zu HLS, Tonemapping, Qualitätsregeln, Komfortfunktionen, Geräte-Fähigkeiten | Kriterien W-1 bis W-10 |
| Entdecken und Kuratierung | active | Startseite mit Karussell „Cat's Empfehlungen“ und fünf Reihen, Suche mit Filtern, Trakt-Top-Listen aus der eigenen Bibliothek, Favoriten mit Benachrichtigung | Kriterien E-1 bis E-9; **algorithmische Empfehlungen, Profil-Interaktion sowie Assistent, Zufall, Trailer-Reel und Themenlisten zurückgestellt** (Runden 6 und 18) |
| Offline-Wiedergabe | active | Downloads einzelner Titel, Staffeln, Auto-Downloads, kein Ablauf, Profilregeln offline | Kriterien O-1 bis O-7 |
| Clients und Design | active | Web-App als PWA für Desktop, Handy, Tablet; eigenes Design-System; Admin-Oberfläche gleichwertig | Kriterien C-1 bis C-5; **TV-Apps und native Mobile-Apps zurückgestellt** (Runde 3) |
| Server-Administration | active | Einrichtungsassistent, zentrale Verwaltung, Dashboard, Sicherung, Update-Hinweis, Benachrichtigungen, Statistiken, Ankündigungen | Kriterien A-1 bis A-9 |
| Integrationen | active | Radarr/Sonarr lesend (Status), Seerr-Bereich zum Wünschen, Bazarr für Untertitel, SABnzbd optional, Trakt (öffentliche Listen), YouTube-Trailer | Kriterien I-1 bis I-7; **Steuerung der Arr-Dienste aus Slouchcat zurückgestellt** (Runde 12) |

## Goal
Slouchcat ist ein selbst gehosteter, vollständig eigenständiger Media-Server für einen Haushalt und dessen eingeladene Gäste. Nutzer erleben ihn wie einen persönlichen Streamingdienst: Netflix-artige Profile mit Altersfreigabe und PIN, eine kuratierte Startseite mit „Cat's Empfehlungen“ (vom Hauptbenutzer gepflegte Top Picks), verlässliche Wiedergabe mit Transcoding und echte Offline-Wiedergabe. Server und Produkt sind gleichwertig: Die Admin-Oberfläche erhält dieselbe Design-Sorgfalt wie die Zuschauerseite, und die API ist von Anfang an für Fremd-Clients dokumentiert. Version 1 deckt Filme, Serien, Hörbücher und Podcasts ab, läuft als Docker-Stack auf Linux und wird über eine Web-App (PWA) bedient. Alle wichtigen Funktionen von Jellyfin sind enthalten; darüber hinaus Kuratierung, Schauzeit-Limits, Geräte-Fähigkeitsprofile, Beschaffungsstatus aus Radarr/Sonarr, ein Wunsch-Bereich über Seerr, Trakt-Top-Listen aus der eigenen Bibliothek und Favoriten mit Benachrichtigung bei neuen Folgen.

## Constraints
- Betrieb ausschliesslich als Docker-Compose-Stack auf Linux (Heimserver oder NAS); GPU-Beschleunigung optional (NVENC, QuickSync, VAAPI). Kein natives Paket, keine Cloud-Variante in Version 1.
- Ein Server pro Haushalt. Hauptbenutzer-Konto mit E-Mail-Adresse; Profile darunter nur mit Namen; Gäste erhalten eigene Konten per Einladungslink.
- Genau ein Client in Version 1: Web-App als PWA für Desktop, Handy und Tablet. Offline-Speicher liegt im Browser; dessen Grenzen (Quota, iOS-Eviction) sind in der Architektur zu adressieren.
- Medientypen Version 1: Filme, Serien, Hörbücher, Podcasts. Architektur hält Musik, Fotos und Live-TV offen, baut sie aber nicht.
- Untertitel-Beschaffung erfolgt über Bazarr, nicht in Slouchcat. Anzeige und Umschalten vorhandener Untertitel sind Slouchcats Aufgabe.
- Wünschen von Inhalten erfolgt über Seerr (Overseerr/Jellyseerr) in einem eigenen Bereich von Slouchcat; Slouchcat baut keine eigene Wunschverwaltung.
- YouTube-Trailer werden eingebettet oder gestreamt, nicht heruntergeladen (Nutzungsbedingungen). Die Suche nach passenden Trailern darf über eine KI-API laufen, muss aber nicht.
- Trakt liefert nur öffentliche Listen über einen Server-API-Schlüssel; kein Trakt-Login für Profile, kein Hochladen des Schauverlaufs.
- Solo-Entwicklung mit Claude Code und oh-my-claudecode-Agenten, keine feste Deadline. Erstes Etappenziel: vertikaler Durchstich (Bibliothek scannen, Film abspielen, Startseite) innerhalb der ersten Wochen.
- Design-Richtung: dunkel und kinoartig (Nähe zu Apple TV+ und Netflix): grosse Bildflächen, ruhige Typografie, wenig Rahmen und Linien, weiche Bewegung bei Fokus und Scrollen, eine Akzentfarbe je Profil wählbar, Posterfarben färben den Hintergrund dezent. Sehr modern, schön und vor allem ordentlich und übersichtlich; Übersichtlichkeit hat Vorrang vor Effekten.
- Sprache, Framework und Datenbank sind bewusst offen; der Plan entscheidet nach drei Kriterien: sehr stabil, läuft überall gut (Docker auf Linux, auch auf schwachem NAS), performant. Die Begründung muss für Nicht-Entwickler verständlich sein.
- Offline auf iOS: Version 1 liefert Offline in der PWA auf allen Plattformen mit ehrlichem Hinweis auf die Apple-Grenzen (Speicherlimit, Löschung nach sieben Tagen Inaktivität). Zeigt der Durchstich, dass Downloads auf dem iPhone verschwinden, wird eine native iOS-App als erste Ergänzung nach Version 1 eingeplant, nicht Version 1 verzögert.
- Dokumentation auf Deutsch (Schweizer Schreibweise), Code und Bezeichner auf Englisch.

## Non-Goals
- Musik, Fotos, Live-TV und DVR (spätere Versionen).
- TV-Apps (Android TV, Apple TV, webOS, Tizen) und native iOS-/Android-Apps (spätere Versionen).
- Plugin-System oder Erweiterungs-Marktplatz.
- Automatische, algorithmische Empfehlungen („Weil du X gesehen hast") und Interaktion zwischen Profilen (Bewertungen, Empfehlungen untereinander).
- Entdecken-Hilfen „Was schauen wir heute?“-Assistent, Zufallsauswahl, Trailer-Reel und automatische Themenlisten (Kandidaten für Version 1.x, in Runde 18 nicht gewählt).
- Trakt-Konten je Profil, Scrobbling, Import von Trakt-Watchlists.
- Freigabe-Anfragen von Kinderprofilen an den Hauptbenutzer.
- Eigene Wunschverwaltung (Seerr übernimmt) und Steuerung von Radarr, Sonarr, SABnzbd aus Slouchcat heraus (Qualitätsprofile, Warteschlangen).
- Eigene Untertitel-Beschaffung (Bazarr übernimmt).
- Mandantenfähigkeit (mehrere gleichberechtigte Haushalte auf einem Server).
- Cloud-Betrieb, natives Installationspaket, Windows- oder macOS-Server.
- Download von YouTube-Inhalten auf den Server.

## Acceptance Criteria

### Konten, Profile und Zugriff (K)
- [ ] K-1 Der Hauptbenutzer meldet sich mit E-Mail-Adresse und Passwort an; Profile darunter tragen nur einen Namen und optional ein Bild.
- [ ] K-2 Der Hauptbenutzer legt Profile an, ändert und löscht sie und setzt je Profil eine Altersstufe (FSK 0, 6, 12, 16, 18), eine optionale PIN und Bibliotheksrechte.
- [ ] K-3 Ein Profil sieht Inhalte über seiner Altersstufe nirgends: nicht auf der Startseite, nicht in Top Picks, nicht in der Suche, nicht in Sammlungen, nicht in Downloads. Negativtest mit Kinderprofil ist Pflicht für jede Liste.
- [ ] K-4 Der Wechsel in ein PIN-geschütztes Profil verlangt die PIN; falsche Eingaben werden gebremst.
- [ ] K-5 Je Profil lässt sich ein tägliches Schauzeit-Limit setzen; bei Erreichen stoppt die Wiedergabe mit Hinweis, und der Hauptbenutzer sieht einen Verlaufsbericht je Profil.
- [ ] K-6 Der Hauptbenutzer erzeugt Einladungslinks; ein Gast erstellt damit ein eigenes Konto mit eigenen Profilen und eingeschränkten Rechten.
- [ ] K-7 Der Hauptbenutzer entzieht Gastzugriff jederzeit; danach sind Sitzungen des Gastes sofort ungültig und seine Downloads nicht mehr abspielbar.
- [ ] K-8 Zugriff von ausserhalb des Heimnetzes funktioniert über HTTPS mit denselben Konten; alle Rechte werden serverseitig durchgesetzt.

### Video-Bibliothek und Metadaten (B)
- [ ] B-1 Der Admin legt Bibliotheken auf Ordner an (Typ Filme oder Serien) und bindet Medien read-only ein.
- [ ] B-2 Dateien in üblicher Benennung (Plex/Jellyfin-Konvention) sowie NFO-Dateien werden korrekt als Film, Serie, Staffel, Folge erkannt.
- [ ] B-3 Metadaten und Bilder kommen von TMDB und TVDB in Deutsch und Englisch; Sprache ist je Profil wählbar.
- [ ] B-4 Änderungen im Ordner (neu, umbenannt, gelöscht) erscheinen ohne manuellen Scan innerhalb einer Minute.
- [ ] B-5 Mehrere Versionen desselben Films (4K, 1080p) erscheinen als ein Titel mit wählbarer Version.
- [ ] B-6 Falsch erkannte Titel kann der Admin manuell einem TMDB/TVDB-Eintrag zuordnen; die Zuordnung übersteht erneute Scans.
- [ ] B-7 Extras (Trailer, Making-of, gelöschte Szenen) aus Unterordnern werden erkannt und am Titel angezeigt.
- [ ] B-8 Zu jedem Film und jeder Serie wird automatisch ein YouTube-Trailer gefunden und eingebettet abgespielt; sind Trailer in mehreren Sprachen vorhanden, kann das Profil die Sprache wählen; der Admin kann den Trailer manuell ersetzen.
- [ ] B-9 Pflegewerkzeuge zeigen Duplikate, Titel ohne Metadaten oder Bilder, verwaiste Datenbankeinträge und nicht abspielbare Dateien und erlauben deren Bereinigung.

### Hörbücher und Podcasts (H)
- [ ] H-1 Hörbuch-Bibliotheken erkennen Bücher mit Kapiteln (aus Dateien oder Kapitelmarken) und zeigen Autor, Sprecher, Cover.
- [ ] H-2 Podcasts werden per Feed-URL abonniert; neue Episoden erscheinen automatisch.
- [ ] H-3 Fortschritt wird je Profil und Titel gespeichert, auch mitten in einem Kapitel; Wiedergabegeschwindigkeit ist einstellbar.
- [ ] H-4 Hörbücher und Podcasts unterliegen denselben Profil-, Altersstufen- und Offline-Regeln wie Video.

### Wiedergabe und Transcoding (W)
- [ ] W-1 Kann das Gerät Datei und Codec abspielen, erfolgt Direct Play ohne Umwandlung.
- [ ] W-2 Andernfalls wird automatisch zu HLS transcodiert, mit GPU wenn vorhanden, inklusive HDR-zu-SDR-Tonemapping.
- [ ] W-3 Ein 4K-HEVC-Film startet im Handy-Browser in unter fünf Sekunden; Spulen funktioniert auch im laufenden Transcode.
- [ ] W-4 Tonspur und Untertitel lassen sich während der Wiedergabe wechseln; Untertitel werden als Text oder eingebrannt geliefert, je nach Gerät.
- [ ] W-5 Fortschritt wird je Profil gespeichert und auf allen Geräten fortgesetzt.
- [ ] W-6 Der Admin sieht laufende Streams mit Profil, Gerät, Titel, Qualität und Transcoding-Status und kann Streams beenden.
- [ ] W-7 Der Admin sieht je Gerät Eigentümer, letzten Kontakt und ein Fähigkeitsprofil (unterstützte Container, Video- und Audio-Codecs, maximale Auflösung, HDR).
- [ ] W-8 Je Profil und je Gast lassen sich Bandbreite, maximale Qualität und Anzahl gleichzeitiger Streams begrenzen; im Player ist die Qualität manuell wählbar.
- [ ] W-9 Intro und Abspann lassen sich überspringen, die nächste Folge startet automatisch, Kapitelmarken sind anwählbar.
- [ ] W-10 Trailer lassen sich von der Detailseite aus abspielen.

### Entdecken und Kuratierung (E)
- [ ] E-1 Die Startseite beginnt mit einem Karussell „Cat's Empfehlungen“ (die Top Picks des Hauptbenutzers); darunter folgen die Reihen Weiterschauen, Neu hinzugefügt, Merkliste sowie Sammlungen und Genres.
- [ ] E-2 Der Hauptbenutzer pflegt Cat's Empfehlungen für alle Profile oder für einzelne Profile, mit Reihenfolge, optionaler persönlicher Notiz und optionalem Zeitraum.
- [ ] E-3 Cat's Empfehlungen respektieren die Altersstufe: Ein Profil sieht nur Picks, die es sehen darf.
- [ ] E-4 Die Suche findet Titel, Personen und Sammlungen ab dem zweiten Zeichen und filtert nach Genre, Jahr, Altersfreigabe, Medientyp und Verfügbarkeit (lokal, Download, in Beschaffung).
- [ ] E-5 Jedes Profil hat eine eigene Merkliste.
- [ ] E-6 Sammlungen entstehen automatisch aus TMDB-Reihen und manuell durch den Admin.
- [ ] E-7 Reihen „Trending bei Trakt“ und „Beliebt bei Trakt“ (Filme und Serien, Top 20) zeigen ausschliesslich Titel, die in der eigenen Bibliothek vorhanden sind, und aktualisieren sich täglich.
- [ ] E-8 Ein Profil markiert Serien als Favorit; erscheint eine neue Folge in der Bibliothek, wird das Profil benachrichtigt (In-App und über seine Kanäle aus A-6).
- [ ] E-9 Die Startseite bleibt übersichtlich: höchstens acht Reihen, keine leeren Reihen, klare Reihenfolge, die der Hauptbenutzer anpassen kann.

### Offline-Wiedergabe (O)
- [ ] O-1 Ein Profil lädt einen Film, eine Folge, ein Hörbuch oder eine Podcast-Episode in wählbarer Qualität auf das Gerät.
- [ ] O-2 Ganze Staffeln lassen sich mit einer Aktion laden.
- [ ] O-3 Auto-Downloads laden die nächste Folge einer begonnenen Serie und neue Episoden abonnierter Podcasts nach einstellbaren Regeln (nur im WLAN, maximale Anzahl).
- [ ] O-4 Geladene Inhalte spielen im Flugmodus ab, inklusive Untertitel und gewählter Tonspur.
- [ ] O-5 Fortschritt aus der Offline-Wiedergabe wird beim nächsten Kontakt mit dem Server zusammengeführt.
- [ ] O-6 Downloads haben kein Ablaufdatum; sie verschwinden nur, wenn das Profil sie löscht oder der Hauptbenutzer den Zugriff entzieht.
- [ ] O-7 Altersstufe und Bibliotheksrechte gelten offline wie online; der verfügbare Speicher und seine Grenzen werden dem Profil angezeigt.

### Clients und Design (C)
- [ ] C-1 Die Web-App läuft als PWA auf Desktop, Handy und Tablet, ist installierbar und startet offline mit den geladenen Inhalten.
- [ ] C-2 Ein eigenes Design-System (Tokens für Farbe, Typografie, Abstände, Bewegung) trägt Zuschauer- und Admin-Oberfläche gleichermassen; keine Farbe und kein Text steht hart im Code.
- [ ] C-3 Die Oberfläche ist in Deutsch und Englisch verfügbar, umschaltbar je Profil.
- [ ] C-4 Bedienung per Tastatur und Fernbedienungs-ähnlicher Navigation (Pfeiltasten, Enter, Zurück) funktioniert auf allen Hauptseiten.
- [ ] C-5 Die API ist als OpenAPI-Dokument veröffentlicht und so vollständig, dass ein Fremd-Client alle Zuschauerfunktionen umsetzen kann.

### Server-Administration (A)
- [ ] A-1 Beim ersten Start führt ein Einrichtungsassistent durch Hauptbenutzer-Konto, erste Bibliothek, GPU-Erkennung und Metadaten-Schlüssel.
- [ ] A-2 Bibliotheken, Konten, Profile, Gäste, Geräte, Integrationen und Top Picks werden an einem Ort verwaltet, mit derselben Design-Sorgfalt wie die Zuschauerseite.
- [ ] A-3 Ein Dashboard zeigt laufende Streams, Hintergrundjobs, Speicherplatz, Integrationsstatus und Fehler.
- [ ] A-4 Datenbank und Konfiguration lassen sich per Knopf sichern und aus einer Sicherung wiederherstellen; Medien sind nie Teil der Sicherung.
- [ ] A-5 Ein Hinweis erscheint, wenn eine neue Slouchcat-Version verfügbar ist.
- [ ] A-6 Benachrichtigungskanäle (E-Mail, Web-Push, Telegram) sind je Konto konfigurierbar für neue Inhalte, Beschaffungsstatus, Fehler und Einladungen.
- [ ] A-7 Der Hauptbenutzer veröffentlicht Ankündigungen (News), die Profile beim nächsten Start sehen und die optional per E-Mail an Gastkonten gehen.
- [ ] A-8 Statistiken zeigen meistgesehene Titel, Schauzeit je Profil, Transcoding-Anteil und Geräteverteilung.
- [ ] A-9 Protokolle enthalten keine Geheimnisse (Passwörter, Tokens, API-Schlüssel).

### Integrationen (I)
- [ ] I-1 Radarr und Sonarr werden per URL und API-Schlüssel angebunden; Slouchcat liest deren Status.
- [ ] I-2 Am Titel und in der Suche ist sichtbar, ob ein Inhalt gewünscht, in Beschaffung oder verfügbar ist; nach Verfügbarkeit erscheint er ohne manuellen Scan.
- [ ] I-3 Eine Benachrichtigung geht an die interessierten Profile, sobald ein beschaffter Inhalt verfügbar ist.
- [ ] I-4 Ein eigener Bereich „Wünschen" in Slouchcat bindet Seerr an, sodass Profile Filme und Serien wünschen können, ohne Slouchcat zu verlassen; Slouchcat verwaltet keine eigenen Wünsche.
- [ ] I-5 Von Bazarr beschaffte Untertitel werden nach Ablage erkannt und sofort im Player angeboten.
- [ ] I-6 SABnzbd kann optional angebunden werden, um den Download-Fortschritt am Titel zu zeigen; ohne Anbindung fehlt nur diese Anzeige.
- [ ] I-7 Trakt wird über einen Server-API-Schlüssel angebunden und liefert nur öffentliche Listen; fällt Trakt aus, verschwinden die Reihen aus E-7 ohne weitere Auswirkung.

## Assumptions Exposed & Resolved
| Assumption | Challenge | Resolution |
|------------|-----------|------------|
| Nur der eigene Haushalt nutzt den Server | Runde 1: Haushalt, Gäste oder Mandanten? | Haushalt plus eingeladene Gäste, kein Mandantenmodell |
| Server soll überall laufen | Runde 2: Docker, nativ, Cloud? | Nur Docker auf Linux, GPU optional |
| Offline braucht native Apps | Runde 3: Clients dieser Version? | Nur Web/PWA; Offline über Browser-Speicher, Risiko dokumentiert |
| „Alle Medientypen" | Runde 4 (Contrarian): nur Filme und Serien? | Filme, Serien, Hörbücher, Podcasts; Musik, Fotos, Live-TV später |
| Downloads laufen ab | Runde 5: Ablauf nach 30 Tagen? | Kein Ablauf; Entzug nur durch Profil oder Hauptbenutzer |
| Startseite braucht Algorithmus-Empfehlungen | Runde 6 (Simplifier): kleinste Startseite? | Top-Picks-Karussell plus fünf Reihen, keine Algorithmen |
| „Transcoding muss funktionieren" ist eine Anforderung | Runde 7: prüfbare Kriterien? | Zehn Kriterien inklusive Geräte-Fähigkeitsprofilen |
| Produkt vor Server | Runde 8 (Ontologist): was IST Slouchcat? | Server und Produkt gleichwertig; API dokumentiert; kein Plugin-System |
| Team und Deadline unbekannt | Runde 9 | Solo mit Agenten, keine Deadline, früher Durchstich |
| Profile = Netflix-Kopie | Runde 10 | Plus Schauzeit-Limits und Berichte; keine Freigabe-Anfragen |
| Bibliothek = Jellyfin-Parität | Runde 11 | Plus Trailer (YouTube), Extras, Pflegewerkzeuge; Untertitel via Bazarr; Arr-Dienste als neuer Baustein |
| „Voll integriert" = steuern | Runde 12 | Nur Status lesen und benachrichtigen |
| Design-Richtung ergibt sich später | Runde 14 | Dunkel und kinoartig, Akzentfarbe je Profil, Posterfarben im Hintergrund; ordentlich und übersichtlich |
| Stack braucht eine Vorgabe des Auftraggebers | Runde 15 | Keine Vorgabe; Kriterien stabil, überall lauffähig, performant; Plan entscheidet |
| Kein Ablaufdatum gilt auch auf iOS | Runde 16 | PWA zuerst mit Hinweis; native iOS-App nach Version 1 bei Bedarf |
| Entdecken = Startseite plus Suche | Runden 17–18 | Plus Trakt-Top-Listen aus der eigenen Bibliothek und Favoriten-Benachrichtigung; Top Picks heissen „Cat's Empfehlungen“; lokale Hilfen nicht gewählt |
| Trakt heisst Konto-Sync | Runde 19 | Nur öffentliche Listen, kein Login, kein Scrobbling |
| Admin-Oberfläche ist Nebensache | Runde 13 | Gleichwertig, mit Benachrichtigungen, Statistiken, Ankündigungen; Hauptbenutzer per E-Mail; Wünschen via Seerr |

## Offene Punkte für die Planungsphase (mit Empfehlung, im Plan zu bestätigen)
1. **Seerr-Einbettung:** Eigener Bereich über die Seerr-API mit Slouchcat-Oberfläche (Design konsistent, mehr Aufwand) oder Einbettung der Seerr-Oberfläche (schnell, Stilbruch). Empfehlung: API mit eigener Oberfläche, Einbettung als Übergangslösung im Durchstich erlaubt.
2. **Ankündigungen:** Nur In-App und E-Mail an Gastkonten, oder alle Kanäle aus A-6. Empfehlung: dieselben Kanäle wie A-6.
3. **Trailer-Beschaffung:** YouTube-Data-API mit eigener Heuristik oder KI-API als Vermittler. Beide erfüllen B-8; Empfehlung: YouTube-API zuerst, KI-API als austauschbarer Anbieter dahinter.
4. Offline auf iOS: entschieden in Runde 16 (siehe Constraints).

## Technical Context
Greenfield. Leeres Repository unter `/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat` (README, Vision, .gitignore). Betrieb als Docker-Compose-Stack auf Linux, GPU optional. Ein Web-Client als PWA. Externe Dienste: TMDB, TVDB, YouTube (eingebettet), Radarr, Sonarr, Seerr, Bazarr, optional SABnzbd, optional eine KI-API für die Trailer-Suche. Sprache, Framework, Datenbank und Transcoding-Werkzeug (voraussichtlich ffmpeg) werden in der Architekturphase entschieden. Entwicklungsumgebung: Linux Mint mit NVIDIA-GPU, Claude Code mit oh-my-claudecode.

## Ontology (Key Entities)
| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| Server | core domain | Version, Konfiguration, GPU | hat viele Bibliotheken, Konten, Geräte, Integrationen |
| Konto (Hauptbenutzer, Gastkonto) | core domain | E-Mail, Passwort, Rolle, Benachrichtigungskanäle | hat viele Profile; Hauptbenutzer erzeugt Einladungen |
| Profil | core domain | Name, Bild, Altersstufe, PIN, Sprache, Schauzeit-Limit, Bibliotheksrechte | gehört zu Konto; hat Fortschritte, Merkliste, Downloads |
| Einladung | supporting | Token, Ablauf, Rechte | erzeugt Gastkonto |
| Bibliothek | core domain | Typ, Pfade, Metadatenquellen | enthält Titel |
| Titel (Film, Serie, Staffel, Folge, Hörbuch, Podcast, Episode) | core domain | Metadaten, Bilder, Altersfreigabe, Versionen, Extras, Trailer | gehört zu Bibliothek; hat Medien-Dateien |
| Version / Medien-Datei | supporting | Pfad, Container, Codecs, Auflösung, HDR | gehört zu Titel |
| Trailer | supporting | Quelle (YouTube), Sprache, manuell gesetzt | gehört zu Titel |
| Sammlung | supporting | Name, automatisch/manuell | gruppiert Titel |
| Cat's Empfehlung (Top Pick) | core domain | Zielprofil oder alle, Reihenfolge, Notiz, Zeitraum | verweist auf Titel |
| Favorit | supporting | Serie, Benachrichtigung aktiv | Profil × Serie |
| Trakt-Liste | external system | Typ (Trending, Beliebt), Einträge, Stand | gefiltert auf Titel der Bibliothek |
| Merkliste | supporting | Einträge | gehört zu Profil |
| Fortschritt | supporting | Position, Zeitpunkt, Gerät | Profil × Titel |
| Gerät | core domain | Name, Eigentümer-Konto, Fähigkeitsprofil, letzter Kontakt | hat Sitzungen, Downloads |
| Fähigkeitsprofil | supporting | Container, Codecs, max. Auflösung, HDR | gehört zu Gerät |
| Stream | supporting | Profil, Gerät, Titel, Qualität, Transcoding-Status | läuft auf Server |
| Download | core domain | Titel/Version, Qualität, Grösse, Zustand | Profil × Gerät; Download-Regel steuert Auto-Downloads |
| Download-Regel | supporting | nur WLAN, max. Anzahl, nächste Folge, neue Episoden | gehört zu Profil |
| Altersstufe | supporting | FSK 0/6/12/16/18 | Profil und Titel |
| Ankündigung | supporting | Text, Zeitraum, Zielgruppe, Kanäle | vom Hauptbenutzer |
| Benachrichtigungskanal | supporting | E-Mail, Web-Push, Telegram, Ereignisse | gehört zu Konto |
| Integration (Arr-Dienst, Seerr, Bazarr, SABnzbd) | external system | URL, API-Schlüssel, Status | liefert Beschaffungsstatus |
| Beschaffungsstatus | supporting | gewünscht, in Beschaffung, verfügbar | Titel × Integration |
| Wunsch | external system | über Seerr | Profil × Titel |
| API-Client | supporting | OpenAPI-Dokument | nutzt Server |

## Ontology Convergence
| Round | Entity Count | New | Changed | Stable | Stability Ratio |
|-------|-------------|-----|---------|--------|----------------|
| 1 | 8 | 8 | - | - | - |
| 2 | 9 | 1 | 0 | 8 | 100 % |
| 3 | 10 | 1 | 0 | 9 | 100 % |
| 4 | 12 | 2 | 0 | 10 | 100 % |
| 5 | 14 | 2 | 0 | 12 | 100 % |
| 6 | 16 | 2 | 0 | 14 | 100 % |
| 7 | 19 | 3 | 0 | 16 | 100 % |
| 8 | 20 | 1 | 0 | 19 | 100 % |
| 9 | 20 | 0 | 0 | 20 | 100 % |
| 10 | 22 | 2 | 0 | 20 | 100 % |
| 11 | 25 | 3 | 0 | 22 | 100 % |
| 12 | 26 | 1 | 0 | 25 | 100 % |
| 13 | 29 | 3 | 0 | 26 | 100 % |

Keine Entität wurde umbenannt oder entfernt; das Modell wuchs nur durch bewusst neu aufgenommene Konzepte.

## Interview Transcript
<details>
<summary>Vollständige Fragen und Antworten (13 Runden)</summary>

### Runde 0 – Topologie
**F:** Sechs Bausteine vorgeschlagen (Konten/Profile, Bibliothek, Wiedergabe, Kuratierung, Offline, Clients/Design). Stimmt das?
**A:** Es fehlen Server-Administration, weitere Medientypen, Remote-Zugriff und Teilen, Suche und Entdecken. → Zusammengefasst zu acht Bausteinen; Integrationen kamen in Runde 11 als neunter dazu.

### Runde 1 – Konten/Zugriff, Ziel
**F:** Wer nutzt Slouchcat?
**A:** Ein Server pro Haushalt mit Hauptbenutzer und Familienprofilen plus eingeladene Gäste mit eigenen Konten.
**Mehrdeutigkeit:** 85.5 %

### Runde 2 – Server-Administration, Grenzen
**F:** Wo läuft Slouchcat?
**A:** Docker Compose auf Linux-Heimserver oder NAS, GPU optional. Nicht nativ, nicht Cloud.
**Mehrdeutigkeit:** 81 %

### Runde 3 – Clients/Design, Ziel
**F:** Welche Clients in dieser Version?
**A:** Nur Web-App als PWA. TV und native Apps später.
**Mehrdeutigkeit:** 74 %

### Runde 4 – Medientypen, Grenzen (Contrarian)
**F:** Was, wenn nur Filme und Serien?
**A:** Filme und Serien plus Hörbücher und Podcasts. Musik, Fotos, Live-TV später.
**Mehrdeutigkeit:** 67 %

### Runde 5 – Offline, Kriterien
**F:** Wann ist Offline erfüllt?
**A:** Einzeltitel, Staffeln, Auto-Downloads, Flugmodus-Wiedergabe, Fortschritt-Sync, Profilregeln offline, kein Ablaufdatum.
**Mehrdeutigkeit:** 59 %

### Runde 6 – Entdecken, Ziel (Simplifier)
**F:** Kleinste überzeugende Startseite?
**A:** Fünf Reihen plus Suche; Top Picks als Karussell. Keine Algorithmen, keine Profil-Interaktion.
**Mehrdeutigkeit:** 50.5 %

### Runde 7 – Wiedergabe, Kriterien
**F:** Wann ist Wiedergabe erfüllt?
**A:** Direct Play, GPU-Transcoding, Tonemapping, 5-Sekunden-Start, Spulen, Spurwechsel, Fortschritt, Stream-Übersicht; plus Qualitätsregeln; plus Komfortfunktionen; plus Geräteübersicht mit Codec-Fähigkeiten.
**Mehrdeutigkeit:** 41 %

### Runde 8 – Gesamtbild (Ontologist)
**F:** Was IST Slouchcat?
**A:** Server und Produkt gleichwertig; Admin-Oberfläche mit gleicher Design-Sorgfalt; API dokumentiert.
**Mehrdeutigkeit:** 35.8 %

### Runde 9 – Rahmen, Grenzen
**F:** Wer entwickelt, welcher Zeitrahmen?
**A:** Solo mit Agenten, keine Deadline, früher Durchstich, Stack offen.
**Mehrdeutigkeit:** 31.3 %

### Runde 10 – Konten/Profile, Kriterien
**F:** Wann ist Profilverwaltung erfüllt?
**A:** Altersstufen, PIN, Bibliotheksrechte, Kinderprofil sieht nichts Überstufiges, Einladungslinks, Entzug; plus Schauzeit-Limits und Berichte. Keine Freigabe-Anfragen.
**Mehrdeutigkeit:** 25.4 %

### Runde 11 – Bibliothek, Kriterien
**F:** Wann ist die Bibliothek erfüllt?
**A:** Wie vorgeschlagen plus Trailer und Extras; Trailer automatisch von YouTube mit wählbarer Sprache, ggf. via KI-API; Untertitel über Bazarr; Radarr und Sonarr voll integrieren, SABnzbd optional.
**Mehrdeutigkeit:** 25.2 %

### Runde 12 – Integrationen, Ziel
**F:** Was heisst „voll integriert"?
**A:** Nur Statusanzeige und Benachrichtigung; kein Wünschen durch Profile, keine Steuerung.
**Mehrdeutigkeit:** 20.2 %

### Runde 13 – Administration, Kriterien
**F:** Wann ist Administration erfüllt?
**A:** Wie vorgeschlagen plus Benachrichtigungskanäle und Statistiken; Bibliothekspflege; Seerr-Bereich zum Wünschen; Hauptbenutzer per E-Mail für Ankündigungen, Profile nur Namen.
**Mehrdeutigkeit:** 15.8 %

### Runde 14 – Clients/Design, Ziel
**F:** Welche Design-Richtung?
**A:** Dunkel und kinoartig, grosse Bildflächen, ruhige Typografie, weiche Bewegung, Akzentfarbe je Profil, Posterfarben im Hintergrund. Nachtrag: sehr modern, schön, ordentlich und übersichtlich.
**Mehrdeutigkeit:** 13 %

### Runde 15 – Rahmen, Grenzen
**F:** Vorgabe für Sprache und Stack?
**A:** Keine; Kriterien sehr stabil, läuft überall gut, performant; Plan entscheidet und erklärt verständlich.
**Mehrdeutigkeit:** 12.4 %

### Runde 16 – Offline, Grenzen
**F:** Umgang mit den Apple-Grenzen für Offline?
**A:** PWA zuerst mit Hinweis; native iOS-App als erste Ergänzung nach Version 1, falls nötig.
**Mehrdeutigkeit:** 11 %

### Runde 17 – Entdecken, Ziel
**F:** Welche Entdecken-Hilfen gehören in Version 1?
**A:** Zum Beispiel Top 20 von Trakt.tv (Filme und Serien), aber nur Titel aus der eigenen Bibliothek. Ausserdem Favoriten mit Benachrichtigung bei neuen Folgen.
**Mehrdeutigkeit:** 14 %

### Runde 18 – Entdecken, Ziel
**F:** Welche der vier lokalen Hilfen zusätzlich?
**A:** „Cat's Empfehlungen“ (die Administratoren-Empfehlungen). Interpretation: Top Picks heissen so; keine der vier Hilfen gewählt.
**Mehrdeutigkeit:** 12 %

### Runde 19 – Integrationen, Grenzen
**F:** Wie weit geht Trakt?
**A:** Nur öffentliche Listen über Server-Schlüssel; kein Login je Profil, kein Scrobbling.
**Mehrdeutigkeit:** 10 %

</details>
