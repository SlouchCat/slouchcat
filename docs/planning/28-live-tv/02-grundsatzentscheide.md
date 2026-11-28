# Konzept 28 · Grundsatzentscheide

Stand: 29. November 2026 · Fassung 3 · Status: Entwurf nach Gesamtreview, nicht freigegeben

> **Kurz gesagt.** Marcos Auftrag vom 28. November 2026 legt 21 Grundsatzentscheide fest (LTV-001 bis LTV-021).
> Diese Datei fasst sie in je einem Satz zusammen, zeigt, wo der Auftrag von diesem Repo abweicht (er wurde für
> das alte Repo geschrieben), hält Marcos vier Antworten vom 28. November 2026 fest, trennt davon die Auslegungen
> des Koordinators, die Marco noch bestätigen soll, nennt die Ausnahmen von
> bestehenden Regeln, die Live-TV braucht, und verweist auf Frage 37 im Entscheidungsprotokoll.
> Die einzelnen Anforderungen stehen in `03-fachkonzept.md`.

Dieses Papier ändert keine Zeile Code und keine Regel. Massgeblich bleiben Spec, Entscheidungsprotokoll und ADR;
bei Widerspruch gilt Entscheidungsprotokoll > Plan > Spec (`CLAUDE.md`). Alles in Konzept 28 ist
**spezifiziert**, nichts ist gebaut.

Wortlaut des Auftrags: `anhang/auftrag-2026-11-28.md` (Zeilenangaben «Z.» beziehen sich auf diese Datei).

## Lesart

1. **Der speziellere Entscheid geht dem allgemeineren vor** (Auftrag Z. 362). LTV-001 zählt den Umfang auf; die
   Detailentscheide LTV-002 bis LTV-020 schränken ihn ein (Z. 387). Beispiel: LTV-001 nennt «Persönliches
   Timeshift», LTV-007 legt fest, dass es kein gemeinsames Archiv gibt; es gilt LTV-007.
2. **Marcos Antworten vom 28. November 2026** (Abschnitt 3) sind spezieller als der Auftrag und gehen ihm vor, wo
   sie eine Einzelfrage entscheiden. Sie ändern keinen der 21 Entscheide, sondern legen eine Detailregel fest, die
   der Auftrag Marco zur Entscheidung vorlegen liess (LTV-015, Z. 971).
3. **Abschnitt 5 des Auftrags** (Technik des alten Repos) gilt nur in seinem Sinn, nicht wörtlich (Abschnitt 2).
4. **Die Marken [MVP], [1.1], [1.x], [Later]** gibt es in diesem Repo nicht. Umfang wird Stufen zugeordnet
   (V, 0, 1a, 1b, 2, 3, H, 4, 5, 6, L, W, E, Apps; jede Stufe ein Release), nach `18-umsetzungsplan.md`. Die
   Zuordnung ist ein Vorschlag an Marco.
5. **Zahlen** (Fristen, Grenzen, Vorgaben) gelten so, wie `04-architektur.md` Abschnitt 14 sie nennt; wo
   `12-sicherheit.md` strenger ist (feste Ziele, Schauzeit, Grenze von T-29), gilt `12`.
6. **Was ohne Marcos Antwort gilt:** die wortlauttreue Vorgabe, bei Sicherheitsfragen die sichere. Eine
   Empfehlung gilt erst mit seiner Antwort; eine Freigabe «ohne Einzelantworten» setzt die Vorgaben in Kraft.
7. **Wer was entschieden hat:** Marcos Entscheide sind nur die 21 Grundsatzentscheide und seine Antworten E-1 bis
   E-4 (Abschnitt 3). Alles Weitere, auch die Leitplanken des Koordinators, ist Auslegung oder Empfehlung; die
   Auslegungen, die Marco bestätigen soll, stehen in Abschnitt 3.1.
8. **Zwei Arten Freigabe:** «Konzept freigegeben» heisst, Marco hat das Konzept angenommen. «Baufreigabe einer
   Stufe» heisst, Marco hat den Bau dieser Stufe beauftragt. Massgeblich ist Regel 11 in `CLAUDE.md`: Live-TV ist
   kein Nicht-Ziel mehr und konzipiert; gebaut wird je Stufe erst nach Marcos ausdrücklicher Baufreigabe.

---

## 1. Die 21 Grundsatzentscheide

Spalte «Entscheid» gibt den Auftrag in einem Satz wieder; «schliesst ein» und «schliesst aus» nennen nur, was der
Auftrag selbst sagt (mit Zeile). Die Anforderungen dazu: `03-fachkonzept.md`, Abschnitt 3.

| Kennung | Thema | Entscheid in einem Satz | Schliesst ein | Schliesst aus |
|---|---|---|---|---|
| LTV-001 | Funktionsumfang (C, vollwertig) | Slouchcat bekommt einen eigenständigen Live-TV-Bereich mit dem Umfang der Detailentscheide (Z. 368, 387). | Internet-TV, Senderverwaltung, EPG und Programmzeitschrift, Fast Zapping, zentraler Empfang, Stream-Sharing, persönliches Timeshift, DVR, Serienaufnahmen, Senderfavoriten, Erinnerungen, Streamüberwachung, Administration (Z. 372–385) | alles, was ein Detailentscheid ausschliesst (Z. 387) |
| LTV-002 | Empfangsstrategie (C, hybrid) | Jeder Sender wird nach einer von vier Strategien empfangen: permanent, automatisch, bei Bedarf oder zeitgesteuert, mit globaler Vorgabe und Einstellung je Sender (Z. 393–412). | Vorladen benachbarter Sender, Start für geplante Aufnahmen, Freigabe nicht benötigter Ströme, Schutz laufender Wiedergaben und Aufnahmen, Wiederherstellung nach Neustart (Z. 413–418) | – (keine ausdrücklichen Ausschlüsse) |
| LTV-003 | Empfangsquellen (B, erweitertes IPTV) | Slouchcat nimmt M3U/M3U8 als Datei und URL, einzelne Stream-URLs und Xtream-Codes-Live-APIs an und empfängt HTTP(S), HLS, MPEG-TS, DASH ohne DRM, RTSP, RTMP, UDP, RTP und Multicast (Z. 426–441). | Zugangsdaten, eigene Header und User-Agents, mehrere Quellen je Sender mit Rangfolge und automatischem Wechsel (Z. 442–447); Kompatibilitätsmatrix (Z. 453) | physische Tuner, HDHomeRun, SAT>IP, TVHeadend, DVB-C/S/S2/T/T2, DRM-Streams, Umgehung von Kopierschutz, Xtream-VOD und -Serien (Z. 457–467) |
| LTV-004 | Transcoding (B, individuell) | Ein Sender wird möglichst nur einmal empfangen und intern an alle Zuschauer und Aufnahmen verteilt; muss umgewandelt werden, bekommt jeder Zuschauer seinen eigenen Umwandlungsprozess (Z. 473–488). | Original, Remux, eigenes Video-/Audio-Transcoding, eigene Auflösung, Bitrate, Ton- und Untertitelwahl, Hardwarebeschleunigung, Software-Rückfall, bestehende Limits (Z. 492–502); sauberer Lebenszyklus (Z. 506–523) | gemeinsam genutzte Transcoding-Ausgaben (Z. 504) |
| LTV-005 | EPG (B, erweitert) | Slouchcat liest Programmdaten aus XMLTV (Datei, URL, komprimiert, mehrere Quellen), ordnet sie Sendern zu und zeigt sie in einer Programmzeitschrift mit Suche, Aufnahmeplanung und Erinnerungen (Z. 535–553). | Zeitzonen, Sommerzeit, fehlerhafte und unvollständige Daten, geänderte Zeiten, Anpassung geplanter Aufnahmen (Z. 557–563); nichts erfinden (Z. 565); Wiedergabe ohne EPG (Z. 567) | intelligente Zusammenführung konkurrierender Quellen je Sender, KI-Empfehlungen, Verknüpfung mit den Bibliotheken (Z. 571–573) |
| LTV-006 | Senderverwaltung (B, erweitert) | Der Administrator pflegt Sender in einer leistungsfähigen Verwaltung; jeder Sender hat eine stabile Identität unabhängig von seiner Stream-URL (Z. 577, 599). | M3U-Gruppen, eigene Gruppen, Mehrfachzuordnung, (de)aktivieren, umbenennen, Logos, Sortierung, Kanalnummern, Zusammenführen, Rangfolge der Quellen, Land, Sprache, Kategorien, Massenbearbeitung, Duplikaterkennung (Z. 581–597) | Sendergruppen als Benutzergruppen, ein neues Benutzergruppenmodell (Z. 601–603) |
| LTV-007 | Timeshift (A, persönlicher Puffer) | Jeder Zuschauer hat ab dem Einschalten seinen eigenen Zeitversatz-Puffer zum Pausieren und Spulen, der beim Senderwechsel und Beenden gelöscht wird (Z. 607–616). | Pause, Zurück- und Vorspulen, Sprung zu Live, Höchstdauer durch den Administrator (Z. 611–617) | gemeinsames Archiv, Zurückspulen vor das eigene Einschalten, Startpuffer als Archiv (Z. 619–625) |
| LTV-008 | DVR (B, erweitert) | Slouchcat nimmt Sendungen sofort, geplant, manuell, als Serie und wiederkehrend auf, auch ohne angemeldeten Benutzer, und nutzt dafür möglichst den bestehenden Eingangsstrom (Z. 633–660). | Vor-/Nachlauf, Folgenerkennung (sofern zuverlässig), parallele Aufnahmen, Fristen, Kontingente je Konto, Konflikte, EPG-Anpassung, Übersichten, Wiedergabe, Löschen, unvollständige Aufnahmen; Neustart, Stromausfall, Netz, Speicher (Z. 633–662) | Werbeerkennung, KI, suchwortbasierte Aufnahmen, Aufnahmen nach Personen oder Mannschaften, Übernahme in die Bibliotheken (Z. 666–670) |
| LTV-009 | Aufnahmeeigentum (A, privat je Profil) | Aufnahmen, Planungen, Serienregeln, Fortschritt und Löschen gehören allein dem Profil; das Kontingent gilt je Konto (Z. 674–688). | Schutz vor anderen Profilen und Konten (Z. 684–686), Trennung von Technik und Inhalt für den Administrator (Z. 692); Deduplizierung darf untersucht werden (Z. 694) | gemeinsam sichtbare Aufnahmebibliotheken (Abschnitt 16) |
| LTV-010 | Fast Zapping (B, erweitert) | Senderwechsel sollen sehr schnell sein: bestehende Ströme sofort nutzen, kleine Startpuffer, Vorladen benachbarter, häufiger und favorisierter Sender (Z. 698–710). | Untersuchung von GOP, Segmenten, LL-HLS, TS, Clients (Z. 716–735); Zielwerte als zu prüfende Grössen (Z. 741–746); reproduzierbarer Benchmark (Z. 748–762) | vorsorglich gestartete individuelle Umwandlungen (Z. 712) |
| LTV-011 | Streamüberwachung (B, erweitert) | Slouchcat prüft Ströme, verbindet neu, erkennt Ausfälle, Standbild und fehlende Spuren, wechselt auf Ersatzquellen und meldet längere Ausfälle dem Administrator (Z. 768–780). | Unterscheidung legitimer Sonderfälle (Z. 782–784), Schutz vor Endlosschleifen und Prüfflut (Z. 788–792), bestehendes Benachrichtigungssystem (Z. 794) | ein umfangreiches historisches Qualitätsbewertungssystem (Z. 796) |
| LTV-012 | Benutzerrechte (A, einfach) | Live-TV, Aufnahmen und Timeshift werden je Konto und je Profil erlaubt oder verboten, ein Profil hat nie mehr Rechte als sein Konto, und der Server setzt alles durch (Z. 804–820). | bestehende Grenzen für Ströme, Bandbreite, Auflösung und Zugriff (Z. 808–812) | Rechte je Sender oder Sendergruppe (Z. 816) |
| LTV-013 | Catch-up TV (ausgeschlossen) | Slouchcat bietet kein Catch-up TV; permanenter Empfang ist keine Aufzeichnung (Z. 824, 836). | persönliches Timeshift und gezielte Aufnahmen bleiben (Z. 834) | Anbieterarchive, verpasste Sendungen, Catch-up-URLs, rollierende Serverarchive, Daueraufzeichnung (Z. 828–832) |
| LTV-014 | Oberfläche (B, modern) | Live-TV bekommt einen eigenen, ruhigen Bereich im bestehenden Design mit Startseite, Senderübersicht, Programmzeitschrift, Sendungsdetails, Player, «Meine Aufnahmen» und «Geplante Aufnahmen» (Z. 840–935). | Desktop, Smartphone, Tablet, Android TV, Google TV, weitere Clients der Roadmap (Z. 939–945) | ein neues Designsystem (Z. 947); Fire TV ist in diesem Repo Nicht-Ziel (Abschnitt 2) |
| LTV-015 | Jugendschutz (Freigabe, keine Altersprüfung) | Für Live-TV gibt es keine automatische Altersprüfung; das Hauptprofil schaltet Live-TV für ein Profil mit Altersgrenze ausdrücklich frei oder sperrt es (Z. 951–961). | Zugriff auf alle Sender des Kontos bei Freigabe, volle Unsichtbarkeit ohne Freigabe (Z. 960–961); Administrator-Deckel bleibt (Z. 963); dokumentierte Ausnahme (Z. 965) | Altersprüfung je Sender oder Sendung, altersabhängige EPG-Filterung, dynamische Sperren; Ausweitung auf Filme, Serien, andere Bibliotheken (Z. 956–959, 967) |
| LTV-016 | Multi-View und PiP (A, ausgeschlossen) | Ein Player zeigt genau einen Sender (Z. 975). | gleichzeitige Wiedergabe auf verschiedenen Geräten im Rahmen der Limits (Z. 983) | Multi-View, Bild-in-Bild, mehrere Sender in einem Player (Z. 979–981) |
| LTV-017 | Anbieterverwaltung (A, keine Limits) | Slouchcat verwaltet keine Verbindungslimits je IPTV-Anbieter; die allgemeinen Ressourcenlimits gelten, Zugangsdaten werden sicher gespeichert (Z. 987, 997–999). | Verantwortung des Administrators für die Bedingungen seiner Anbieter (Z. 1001) | Verbindungskontingente, Abonnements, Laufzeiten, Verteilung auf mehrere Zugänge, Verbrauchsabrechnung (Z. 991–995) |
| LTV-018 | Senderaktualisierung (B, automatisch) | Senderlisten werden automatisch und von Hand aktualisiert, ohne dass Pflege, Favoriten, Zuordnungen und Planungen verloren gehen oder ein fehlerhafter Import eine funktionierende Liste zerstört (Z. 1007–1021). | stabile IDs, transaktionale Aktualisierung, geänderte Namen und URLs, fehlende, doppelte, unvollständige Listen (Z. 1023–1034) | ein komplexes Freigabe- oder Versionierungssystem (Z. 1036) |
| LTV-019 | Speicherverwaltung (B, erweitert) | Startpuffer, Timeshift und Aufnahmen haben getrennt einstellbare Speicherorte mit Limits, Reserve, Bereinigung und Warnungen, und ein voller oder fehlender Datenträger bringt Slouchcat nicht zum Absturz (Z. 1040–1072). | RAM-Disk, SSD, NVMe, HDD, Netzspeicher, Linux-Dateisysteme (Z. 1048–1053); Schutz laufender Aufnahmen, Aufräumen nach Abstürzen (Z. 1065–1066) | Verteilung auf mehrere Datenträger, Archivierung, Verschieben, nachträgliches Komprimieren (Z. 1076–1079) |
| LTV-020 | Aufnahmequalität (A, Original) | Aufnahmen werden ohne Neucodierung mit allen empfangenen Ton- und Untertitelspuren gespeichert; verlustfreies Umpacken ist erlaubt (Z. 1083–1093). | Begründung des Containers, robuste Behandlung gestörter Ströme (Z. 1095–1108); Vorlage an Marco, falls Spuren verloren gingen (Z. 1110); Transcoding bei der Wiedergabe (Z. 1112) | Neucodierung, nachträgliche Komprimierung, reduzierte Aufnahmequalität (Z. 1085, 1090–1091) |
| LTV-021 | Entwicklung (C, Konzept vor Bau) | Live-TV wird zuerst vollständig konzipiert und mit klickbaren Mockups gezeigt; gebaut wird erst nach Marcos ausdrücklicher Freigabe (Z. 1116, 1134). | Ist-Analyse, Soll-, Architektur-, Daten-, API-, Sicherheits-, Performance-, Speicher-, Testkonzept, Mockups, Spezifikationen, Roadmap, Abnahmekriterien (Z. 1120–1132) | produktive Implementierung vor der Freigabe (Z. 1134) |

---

## 2. Abweichungen zwischen Auftrag und Repo

Der Auftrag nennt das Repo `SlouchCat/server` (Z. 228) mit NestJS, PostgreSQL, Redis, BullMQ, `.deb` und DEC-001.
Gemeint ist das aktuelle Go-Produkt `SlouchCat/slouchcat` (Marco, 28. November 2026). Belege aus der Ist-Analyse
(Rohbericht A1 Abschnitte 4 und 6.1, A3), Stand `1220769b`: Server 1.44.1, Vertrag 1.44.0, letzte Migration 00086,
letzte ADR 0045, letzte Frage 36.

| Nr. | Aussage des Auftrags | Wirklichkeit in diesem Repo | Umgang in Konzept 28 |
|---|---|---|---|
| AB-1 | Stack: Node.js, TypeScript, NestJS, Fastify, PostgreSQL, Drizzle, Redis, BullMQ (Z. 289–299) | Go-Server, SQLite mit einer Schreibverbindung (`docs/adr/0001-stack.md:32`), eigene Job-Queue in SQLite (`docs/adr/0001-stack.md:35`, ADR 0002, ADR 0017), ein Container (`docs/planning/03-plan-v1.md:39`); Web-App React, TanStack, hls.js | Es gelten `CLAUDE.md` und die ADR dieses Repos. Der Sinn von Abschnitt 5 bleibt: ein Serverprozess, kein Microservice, kein zweiter Container, keine externe Middleware, kein zweites Benutzer-, Rechte-, Job- oder Transcoding-System, Homelab-Grösse. «PostgreSQL» (Z. 1652) heisst hier SQLite; «Redis-Last» (Z. 1447) entfällt, an ihre Stelle tritt die Last auf der einen Schreibverbindung (siehe `13-ressourcen-und-performance.md`). |
| AB-2 | Native Linux-Installation als `.deb` (Z. 306, 1501) | Nicht-Ziel: «Kein natives Paket» (`docs/spec/01-anforderungen-v1.md:41`, `:86`); nur Docker (`deploy/Dockerfile`) | bleibt Nicht-Ziel; als «nicht dabei» geführt (`03-fachkonzept.md` Abschnitt 7). Der hiesige «Installer» sind Compose-Dateien und Overrides. |
| AB-3 | Fire TV als zu berücksichtigender Client (Z. 944) | Nicht-Ziel: «TV-Apps (Apple TV, webOS, Tizen, Fire TV)» (`docs/spec/01-anforderungen-v1.md:58-60`, `CLAUDE.md` Regel 11); Konzept 26 E-1 «Fire TV nicht» (`docs/planning/04-entscheidungen.md:139`) | Der Auftrag verlangt, Fire TV zu **berücksichtigen**; er lässt bestehende Ausschlüsse stehen, «sofern sie nicht ausdrücklich geändert werden» (Z. 281). Ob Z. 944 den Ausschluss ändert, kann nur Marco sagen. Vorgabe bis zur Antwort (Auslegung des Koordinators, Abschnitt 3.1): Fire TV wird berücksichtigt (das Konzept prüft, ob die Android-TV-App auf Fire OS läuft, und zeigt, was dafür nötig wäre), ohne eigenen Client und ohne Zusage (`03` LTV-014.82). Clients sind Web-App (Desktop, Smartphone, Tablet), Android-Handy-App und Android-TV-App (deckt Google TV ab). |
| AB-4 | DEC-001 schliesst Live-TV, IPTV, EPG und DVR aus und wird geändert (Z. 273–275); `docs/spec/00_Glossar_und_Entscheidungen.md` (Z. 236) | Eine DEC-Nummerierung und diese Datei gibt es nicht. Das Nicht-Ziel steht in `CLAUDE.md` Regel 11 («Live-TV», `CLAUDE.md:75`), in der Spec «Non-Goals» (`docs/spec/01-anforderungen-v1.md:56-57`, auch `:29`, `:44`), im Plan (`docs/planning/03-plan-v1.md:30`) und in Frage 23 («Musikbibliothek, Fotos, Live-TV und DVR bleiben Nicht-Ziele», `docs/planning/04-entscheidungen.md:90`) | Die Aufhebung geschieht wie bei Webradio und den Android-Apps: Frage 37 im Entscheidungsprotokoll (eingetragen mit «Konzept liegt vor, nicht freigegeben», Abschnitt 5), benannte Ausnahme in Regel 11 und Nachträge in Spec, Plan, Vision und Handover sind schon auf dem Konzeptzweig nachgeführt, mit dem Vermerk «konzipiert, Bau erst nach Freigabe je Stufe» (Abschnitt 4); Stufe 0 behält nur, was `21` Abschnitt 3 nennt. Die Liste aller Stellen steht in `21-aenderungen-an-dokumenten.md`. |
| AB-5 | Marken [MVP], [1.1], [1.x], [Later] (Z. 1881–1886) | gibt es nicht; Umfang je Konzept in Stufen, jede Stufe ein Release oder von Marco gebündelt (`docs/planning/16-webradio.md:680`, `docs/planning/26-android-tv.md:764`) | Jede Anforderung trägt eine Stufe (Vorschlag) nach dem Schnitt von `18-umsetzungsplan.md`: V, 0, 1a, 1b, 2, 3, H (Heimnetz, UDP, RTP, Multicast), 4, 5, 6, L (RTSP, RTMP), W (DASH, HLS mit fMP4), E (Ergänzungen nach Messung), Apps. Die bisherige Entwicklungspriorität ändert das nicht. |
| AB-6 | «Bestehende gleichzeitige Streamlimits» (Z. 808) | gebaut je Konto und je Profil (`server/migrations/00063_minimum_age_accounts.sql:41`, `server/migrations/00061_profiles_open_language.sql:49`, Durchsetzung `server/internal/playback/limits.go:159-197`); gezählt werden nur Wiedergabesitzungen von Titeln (`playback_sessions.item_id NOT NULL`, `server/migrations/00032_playback_prefs.sql:58-63`); Webradio zählt bewusst nicht (K16 F-19) | Live-TV zählt zu den bestehenden Stream-, Bandbreiten- und Auflösungsgrenzen und zur Schauzeit (festgelegte Auslegung). Wie gezählt wird, ist neu (`04-architektur.md`). |
| AB-7 | «Bestehende Bandbreitenbeschränkungen» (Z. 811) | gebaut (`00063…:39` `max_bitrate_kbps`, `server/internal/playback/decide.go:392`) | übernommen |
| AB-8 | «Bestehende Auflösungsbeschränkungen» (Z. 812) | gebaut (`00063…:40` `max_height`, `decide.go:391`); eine Grenze unter der Quellauflösung erzwingt heute eine Umwandlung (`decide.go:391-426`) | übernommen; bei Live-TV ist das je Zuschauer ein eigener Umwandlungsprozess (LTV-004, siehe `13-ressourcen-und-performance.md`) |
| AB-9 | «Bestehende Transcoding-Berechtigungen» (Z. 810) | **fehlt**: es gibt kein Recht «darf umwandeln» (keine Spalte, kein Recht; nur das gemeinsame Budget der Umwandlungen, `server/internal/transcode/download.go:207-223`) | Kein neues Recht; es gelten das Budget der Umwandlungen und die Grenzen AB-6 bis AB-8. Das ist die wortlauttreue Vorgabe («nichts übernehmen, was es nicht gibt»); weil Marco annehmen könnte, er könne Umwandlung je Konto verbieten, ist es eine Weiche in `20` (Gesamtreview RG-15). |
| AB-10 | «Bestehende lokale und externe Zugriffsbeschränkungen» (Z. 809) | **fehlt als Recht**: kein Recht «nur im Heimnetz»; von aussen gelten dieselben Konten und Rechte (Spec K-8, `docs/spec/01-anforderungen-v1.md:102`); `SLOUCHCAT_TRUSTED_PROXIES` dient nur der Client-IP (`server/internal/config/config.go:241`); Handy-App E-18 «HTTP überall mit Warnung» | Live-TV gilt innen und aussen gleich (K-8). Keine neue Heimnetz-Regel; wortlauttreue Vorgabe, als Weiche in `20` (RG-15). |
| AB-11 | «Kinderprofile» und «Hauptnutzer» (Z. 949–953) | Einen Profiltyp «Kinderprofil» gibt es nicht; «Kind» heisst Altersgrenze unter 18 (`server/migrations/00061_profiles_open_language.sql:39` `max_rating`, `server/internal/model/ratings.go:26`, ADR 0045 `docs/adr/0045-wunschrecht-folgt-der-altersgrenze.md:28-29`). Der Hauptnutzer ist das Hauptprofil eines Kontos (`is_default`, `00061…:43`; `requirePrimary`, `server/internal/auth/profiles.go:16-17`) | «Kinderprofil» = jedes Profil mit Altersgrenze unter 18; «Hauptnutzer» = Hauptprofil des Kontos (festgelegte Auslegung). |
| AB-12 | «Bestehende Favoriten» (Z. 1490) | Titel-Favoriten gibt es nicht mehr; sie gingen in der Merkliste auf (Frage 27; `server/migrations/00054_watchlist_follow.sql:46`). Radio hat eigene Senderfavoriten mit Reihenfolge (`server/migrations/00038_radio.sql:123-129`) | Senderfavoriten je Profil nach dem Vorbild der Radio-Favoriten; die Merkliste bleibt für Titel. |
| AB-13 | Systemüberwachung mit CPU-, GPU-, RAM-Werten (Z. 1225–1227, 1494) | **teilweise**: Version, erkannte Hardware, Dateisysteme, freier Platz (`api/openapi.yaml:7251` `getSystemInfo`, `server/internal/admin/dashboard.go:385`); **keine** Messung von CPU-, GPU- oder RAM-Last (`server/internal/admin/system.go:44-50`) | Die Anzeige verlangt der Auftrag; sie ist neu und wird spezifiziert (`15-administration.md`, `13-ressourcen-und-performance.md`), nicht als vorhanden vorausgesetzt. |
| AB-14 | «Bestehende Suche» (Z. 1491) | gebaut nur für Titel, Personen, Sammlungen (`api/openapi.yaml:5951`, `server/migrations/00015_discover.sql:142`) | Programmsuche ist neu; ob sie auch in der allgemeinen Suche erscheint, ist F-5 (`03-fachkonzept.md`). |
| AB-15 | «Bestehendes Benachrichtigungssystem» (Z. 794) | gebaut, Kanäle E-Mail, Web-Push, Telegram, Push an die Handy-App über den Weiterleitungsdienst; Einstellungen **je Konto**, nicht je Profil (`server/migrations/00016_admin.sql:27-35`); Push immer mit normaler Dringlichkeit (`server/internal/notify/push/push.go:174`) | wird genutzt; neue Ereignisarten sind nötig. Erinnerungen je Profil über Kanäle je Konto: F-3. |
| AB-16 | «Bestehende Hintergrundjobs» (Z. 1493) | gebaut, aber ein Zeitplan je Jobart, Takt 60 s, Lease 5 min; ungeeignet für sekundengenaue und stundenlange Aufnahmen (Rohbericht A3 Abschnitt 4) | Import, Abgleich, Prüfung, Aufräumen laufen im bestehenden Jobsystem; Aufnahme und Dauerempfang brauchen einen eigenen Dienst im selben Prozess (wie das Radio-Relay), kein zweites Jobsystem (`04-architektur.md`). |
| AB-17 | «Explizite Freigaben lokaler Quellen gemäss bestehender Sicherheitspolitik» (Z. 1332) | Eine solche Freigabe gibt es nicht. Der Netzwächter sperrt Heimnetz, Loopback, Link-local und Multicast (`server/internal/netguard/netguard.go:48-50`); Webradio F-15 «keine Sender im lokalen Netz» (ADR 0025) | neue, eng gefasste Ausnahme nur für Live-TV mit festen Zielen je Quelle (Abschnitt 4, A-3); dazu A-9 für den Netzmodus |
| AB-18 | «Kinderprofil-Einstellungen» als bestehende Seite (Z. 1578) | keine eigene Seite; Altersgrenze, PIN, Grenzen stehen in der Profilverwaltung des Hauptprofils | Die Live-TV-Schalter erscheinen in der bestehenden Profilverwaltung (`14-oberflaeche.md`). |
| AB-19 | Mockups mit «echten Senderlogos» (Z. 1601) | öffentliches Repo; keine Senderlogos, Sendungsbilder, echten Senderlisten ins Repo (Leitplanken; Vorbild Konzept 25: Bilder nur lokal) | Im Repo nur neutrale, selbst erzeugte Platzhalter; echte Logos höchstens lokal und ausserhalb von git (`19-mockups.md`). |
| AB-20 | Commits, Push und Aktualisierung von `CURRENT_STATUS.md` in der Konzeptphase (Z. 2021–2025) | In Konzeptphasen bleibt `docs/dev/CURRENT_STATUS.md` unverändert (Rohbericht A1 7.4; PR #63, #97, #159, #174, #180); er wird mit dem Bau nachgeführt. Für Konzept 28 verlangt der Auftrag (Abschnitt 20) den Eintrag schon jetzt | Der Koordinator committet; `CURRENT_STATUS.md` ist schon auf dem Konzeptzweig nachgeführt («Entwurf, nichts gebaut, nichts freigegeben», `21` D-15); den Stand des Baus trägt erst der Bau nach. |

---

## 3. Was Marco entschieden hat

Marco hat entschieden: die **21 Grundsatzentscheide** des Auftrags (Abschnitt 1) und die **vier Antworten E-1 bis
E-4** vom 28. November 2026 (diese Tabelle). Sonst nichts; was darüber hinausgeht, ist Auslegung (Abschnitt 3.1)
oder eine offene Frage (`20`).

Wiedergegeben nach den Leitplanken des Koordinators (`LEITPLANKEN.md` Abschnitte 2 bis 4); der genaue Fragetext
stand im Gespräch mit Marco. Vermerk «gegen die Empfehlung» nur, wo die Leitplanken ihn festhalten oder die
Empfehlung der Ist-Analyse (A1) nachweislich anders lautete.

| Nr. | Frage | Marcos Wahl | Gegen die Empfehlung? |
|---|---|---|---|
| E-1 | Zielprodukt: altes Repo `SlouchCat/server` oder aktuelles Go-Produkt? | **Das Konzept gilt für das aktuelle Go-Produkt.** Abschnitt 5 des Auftrags gilt nur im Sinn (Abschnitt 2, AB-1). | nein |
| E-2 | Umfang der Mockups | **Mockups für Web, Android TV und Handy-App.** | **ja.** Empfohlen war «Web + TV» (die Handy-App folgt der Web-Ansicht in Handy-Breite). Marco wählte «Web + TV + Handy-App»: jeder Client bekommt sein eigenes Mockup im Geräterahmen. |
| E-3 | Aufnahmen von Profilen mit Altersgrenze («Kinderprofile») | **Das Hauptprofil des Kontos darf die Aufnahmen seiner Profile mit Altersgrenze sehen, löschen und abspielen.** Aufnahmen von Profilen ohne Altersgrenze bleiben ganz privat (nur das Profil selbst). Der Administrator sieht nie Titel fremder Aufnahmen, nur Belegung je Konto und technische Zustände. | **ja.** Empfohlen war (A1 Q-7 c): Hauptprofil sieht Liste und Grösse und darf löschen, spielt aber nicht ab. Marco erlaubt zusätzlich das Abspielen. |
| E-4 | Kontingent des Kontos ist voll | **Die neue Aufnahme wird abgelehnt und gemeldet** (an das Profil und an das Hauptprofil); der Planer warnt vorher. **Automatisch gelöscht wird nur, was das Profil selbst eingestellt hat:** Aufbewahrungsfrist und «behalte die letzten N Folgen» einer Serienaufnahme. | nein (entspricht A1 Q-11 «(a) plus (c)») |

### 3.1 Auslegungen des Koordinators, von Marco zu bestätigen

Diese Punkte stammen aus den Leitplanken des Koordinators oder aus dem Bestand des Repos, nicht aus Marcos Wort.
Sie gelten bis zu seiner Antwort als **Vorgabe**, und zwar in der wortlauttreuen Form der Spalte «Vorgabe». Die
Frage dazu führt `20` als «Bestätige die Auslegungen des Koordinators» (→ `20` E-28-59 bis E-28-62).

| Nr. | Auslegung | Woher | Vorgabe bis zur Antwort | Varianten |
|---|---|---|---|---|
| AU-1 | **Rechte-Zuständigkeit:** Der Administrator setzt die drei Schalter je Konto als Deckel, das Hauptprofil die Schalter seiner Profile. | Leitplanken; Auftrag LTV-012 («Account», «Profil»), LTV-015 («Hauptnutzer»), ADR 0014 | so; «Hauptnutzer» heisst Hauptprofil des Kontos (AB-11) | nur der Administrator; nur der Besitzer |
| AU-2 | **Zählung:** Live-TV zählt zu den bestehenden Grenzen gleichzeitiger Ströme, Bandbreite, Auflösung und zur Schauzeit. | Leitplanken; Auftrag Z. 808, 811, 812 | so (wortlauttreu zu «bestehende … Limits»); Schauzeit nach dem, was der Server ausliefert (`03` DR-12, `12` 10.3) | wie Radio nur zur Schauzeit |
| AU-3 | **Quellen im Heimnetz** nur vom Administrator, an festen Zielen je Quelle (A-3). | Leitplanken; Auftrag Z. 439–441, 1332 | so; feste Ziele ab Werk als Einzeladresse mit Port (sichere Vorgabe, `12` 4.5) | Netze freigebbar; Freigabe auch durch Konten |
| AU-4 | **«Kinderprofil»** heisst jedes Profil mit wirksamer Altersgrenze unter 18. | Leitplanken; Bestand (kein Profiltyp) | so (`03` DR-25); ob die Grenze des Kontos zählt, ist `20` E-28-07 | eigener Profiltyp |
| AU-5 | **Fire TV:** Das Repo führt Fire TV als Nicht-Ziel; der Auftrag nennt es unter «Berücksichtige» (Z. 944). | Spec `:58-60`, Regel 11, K26 E-1 | **berücksichtigt**: Das Konzept prüft, ob die Android-TV-App auf Fire OS läuft, und zeigt, was nötig wäre; kein eigener Client, keine Zusage (`03` LTV-014.82) | Nicht-Ziel bleibt; Fire TV wird Zielgerät der TV-App |
| AU-6 | **Plattform-Streams** (YouTube, Twitch) werden nicht eigens unterstützt. | Auslegung aus Regel 11 («YouTube-Downloads») | Der Auftrag nennt sie nicht; technisch sind sie keine IPTV-Quellen im Sinn von LTV-003 und werden nicht eigens gebaut. Eine **Aufnahme** eines YouTube-Stroms wäre ein Download nach Regel 11 und bleibt verboten. | unterstützen; ausdrücklich ausschliessen |
| AU-7 | **Kein Download und kein Offline-Abspielen von Aufnahmen.** | Leitplanken | vom Auftrag **nicht verlangt**, in Konzept 28 **nicht geplant**; kein Ausschluss durch Marco | als spätere Stufe vormerken |
| AU-8 | **Safari, iPhone, iPad** | Auftrag LTV-014 (Smartphone, Tablet); kein Apple-Gerät im Haushalt | **vorgesehen** wie die übrigen Browser, aber **ungeprüft, solange kein Gerät da ist** (`20` E-28-12) | Gerät beschaffen; auf die Zusage verzichten |
| AU-9 | **Kein eigener Dienst von Slouchcat im Internet für Live-TV.** | Leitplanken; Regel 11 (Cloud) | so; der Auftrag verlangt keinen | – |
| AU-10 | **Kein Verbindungslimit und kein Schalter je Quelle.** | Auftrag LTV-017 (Z. 987–995) | so, wortlauttreu (`03` DR-29); Frage `20` E-28-02 | Schalter je Quelle; Zahl je Quelle |

**Folge von E-3 für LTV-009:** Der Auftrag sagt «Andere Profile desselben Accounts dürfen diese Aufnahmen nicht
sehen oder abspielen» (Z. 684). E-3 ist die speziellere, spätere Entscheidung zu genau der Detailfrage, die
LTV-015 Marco vorlegen liess (Z. 969–971); sie gilt nur für das Hauptprofil und nur für Profile mit
Altersgrenze. Für alle anderen Profile bleibt Z. 684 wörtlich.

---

## 4. Benannte Ausnahmen von bestehenden Regeln

Jede Ausnahme gilt nur für Live-TV und nur so weit, wie die Spalte «Grenze» sagt. A-9 ist mit Fassung 2 dazugekommen, A-10 bis A-13 mit Fassung 3; A-12 und A-13 gelten nur nach der genannten Antwort Marcos. Die Ausnahmen sind schon auf dem
Konzeptzweig in Regel 11, Spec, Frage 37 und ADR 0046 (vorgeschlagen) eingetragen, mit dem Vermerk «konzipiert, Bau erst nach Freigabe je Stufe» (`21` Abschnitt 1); was erst der Bau ändert, nennt `21` Abschnitt 3.

| Nr. | Bestehende Regel | Ausnahme für Live-TV | Grenze der Ausnahme | Beleg im Auftrag |
|---|---|---|---|---|
| A-1 | **Nicht-Ziel Live-TV und DVR** (`CLAUDE.md` Regel 11; Spec «Non-Goals» `docs/spec/01-anforderungen-v1.md:56-57`; Frage 23) | Live-TV mit Programmzeitschrift, persönlichem Timeshift und Aufnahmen (DVR) von **Fernsehsendern** ist erlaubt. | Webradio bleibt unverändert und ohne Aufnahmen (K16 F-1). Kein DRM und keine Umgehung (LTV-003). Keine weiteren TV-Apps (Apple TV, webOS, Tizen bleiben Nicht-Ziele). Alles aus der Ausschlussliste (Abschnitt 16) bleibt verboten. Als **Auslegung des Koordinators**, bis Marco bestätigt (3.1): Fire TV nur berücksichtigt, kein eigener Client (AU-5); Plattform-Streams nicht eigens unterstützt (AU-6); kein eigener Dienst im Internet (AU-9). Die Ausnahme ist mit Frage 37 wirksam; gebaut wird je Stufe erst nach Marcos Baufreigabe (Regel 11). | Z. 273–281, LTV-001 |
| A-2 | **Altersmodell**: ein Profil sieht Inhalte über seiner Altersstufe nirgends (Spec K-3 `:97`), unbekannte Freigabe gilt unter 18 als gesperrt (AN-1, Frage 6), Kinderprofil-Vertragstest (Plan 10.2, `docs/planning/03-plan-v1.md:671-683`) | Live-TV kennt keine Altersregel. Ein Profil mit Altersgrenze sieht Live-TV, wenn das Hauptprofil es freigibt, dann alle Sender des Kontos ohne Prüfung von Sender, Sendung oder Programmdaten. Das gilt auch für die eigenen Aufnahmen dieses Profils. | Gilt **nicht** für Filme, Serien, Hörbücher, Podcasts und andere bisher altersgefilterte Bibliotheken; deren Jugendschutz bleibt unverändert. Live-TV-Sender, -Sendungen und -Aufnahmen erscheinen nie auf der Startseite, in Top Picks, in Sammlungen oder in Downloads der Bibliothek. Der Administrator-Deckel je Konto bleibt verbindlich. | LTV-015, Z. 951–967 |
| A-3 | **«Nichts im lokalen Netz»** für fremd bestimmte Adressen (ADR 0025 F-15; Netzwächter sperrt privat, Loopback, Link-local, Multicast, `server/internal/netguard/netguard.go:48-50`) | Quellen im Heimnetz sowie UDP, RTP und Multicast sind für Live-TV erlaubt, aber nur an **festen Zielen je Quelle**, die der Administrator beim Freigeben sieht und bestätigt: ab Werk je Eintrag **eine** Adresse (/32 bzw. /128) mit **einem** Port. Ein kleines Netz (höchstens /28 bzw. /124, also 16 Adressen) oder ein Portbereich (höchstens 16 Ports) nur mit zweiter Bestätigung, die zählt, was sie umfasst, und nie, wenn das Netz das Standard-Gateway des Heimnetzes oder des Containers enthält (`12` 4.5, strenger als `04` 14.9; es gilt `12`). Ein Hostname wird beim Freigeben aufgelöst; die Adressen werden angezeigt und gespeichert, eine geänderte Auflösung ist eine neue Freigabe. | Nur Quellen, die **der Administrator** einträgt; Konten und Profile tragen keine Quellen ein. Die Prüfung gilt für **jede** Verbindung der Quelle (Liste, Sender, Weiterleitung, Unterliste, Segment, Schlüssel): öffentlich geht, lokal nur innerhalb der festen Ziele dieser Quelle; ein Verweis ausserhalb ist gesperrt. **Nie freigebbar:** Loopback, `0.0.0.0/8`, unspezifizierte Adressen, Link-Local samt Metadaten-Adressen, die eigenen Adressen des Containers und, im Netzmodus «host», des Hosts, der Port des Weiterleiters, die Netze der eigenen Container-Schnittstellen als Ganzes. Wird ein Ziel entzogen oder die Quelle entfernt oder geändert, enden ihre Verbindungen innert 5 Sekunden (T-29). Sender einer Liste mit lokalem Ziel ausserhalb der festen Ziele werden importiert, aber als «Ziel im Heimnetz nicht freigegeben» nie verbunden. Logos, EPG-Bilder und `url-tvg` aus Listen nie über eine Freigabe, immer nur öffentlich. Bis Stufe H gibt es keine Freigabe (nur öffentliche Ziele). Radio, Podcasts und Bilder bleiben unverändert. Ein ungeprüfter Client wird für Live-TV nie verwendet (`04` T-5, 10.9). | LTV-003 Z. 439–441; Z. 1331–1333 |
| A-4 | **ADR 0032 «Dateien löschen nur auf Befehl des Administrators, nichts löscht automatisch»** (auch Spec B-1 Nachtrag `:105`) | Slouchcat löscht Aufnahmen automatisch, wenn das **Profil selbst** eine Aufbewahrungsfrist oder «behalte die letzten N Folgen» einer Serienaufnahme gewählt hat. Timeshift- und Startpuffer-Dateien sind flüchtig und werden beim Senderwechsel, beim Beenden und nach Abstürzen bereinigt. | Keine andere automatische Löschung von Aufnahmen: nicht bei vollem Kontingent (E-4), nicht durch eine Vorgabe des Administrators, die das Profil nicht übernommen hat (F-10 in `03-fachkonzept.md`), nie eine laufende Aufnahme. Bibliotheksdateien bleiben unter ADR 0032. | LTV-008 Z. 643, LTV-019 Z. 1061–1062, E-4 |
| A-5 | **ADR 0045 «Ein Recht folgt der Altersgrenze»** (Trigger setzen das Wunschrecht unter 18 aus und weisen das Einschalten ab, `server/migrations/00086_wishes_rights_follow_rating.sql:86-120`) | Die drei Live-TV-Rechte sind **nicht** an die Altersgrenze gekoppelt. Das Hauptprofil kann sie für ein Profil mit Altersgrenze einschalten; eine neue oder geänderte Altersgrenze schaltet sie nicht von selbst um. | Gilt nur für «Live-TV», «Aufnahmen», «Timeshift». Das Wunschrecht bleibt unter ADR 0045. Welche Werte ein neues Profil mit Altersgrenze ab Werk hat, regelt `03-fachkonzept.md` (DR-01, F-1). | LTV-015 Z. 953–955, Z. 963 |
| A-6 | **Privatsphäre des Profilmodells** (Aufnahmen privat je Profil, Z. 684–690) | Das Hauptprofil darf die Aufnahmen seiner Profile mit Altersgrenze sehen, löschen und abspielen. | Nur das Hauptprofil desselben Kontos, nur Profile mit Altersgrenze (eine Definition: `03` DR-25; ob die Grenze des Kontos mitzählt, ist `07` F-1). Welche Aufnahmen dazugehören, regelt bis zur Antwort die Vorgabe von `20` E-28-07: Teil A (b) mit Gegenrichtung nach `07` 4.3, also nur Aufnahmen, die entstanden, als das Profil eine Altersgrenze hatte, und nur, solange es heute noch eine hat; Teil B (a), also zählt auch die Altersgrenze des Kontos, die der Administrator setzt (`07` 4.4). Abspielen nur mit beim Hauptprofil wirksamem «Live-TV» (`03` DR-03). Profile ohne Altersgrenze sind gegenüber anderen Profilen über die API privat (404). **Restrisiko:** Gegen das Hauptprofil schützt sie nur eine PIN, und das Hauptprofil kann jede PIN ersetzen (gebaut, `server/internal/auth/profiles.go:464-497`; `12` R-12, Frage `12` F-5; wird in Konzept 28 nicht geändert). Andere Konten und der Administrator sehen keine Titel fremder Aufnahmen, nur Belegung je Konto und technische Zustände. | E-3; LTV-015 Z. 969–971 |
| A-7 | **Medien nur lesend** (Spec B-1, Medien `:ro` eingebunden, `deploy/compose.yaml:36`; Ausnahme bisher nur ADR 0029 für Podcasts) *(von der Redaktion erkannt)* | Slouchcat schreibt Aufnahmen, Timeshift- und Startpuffer-Dateien in eigene, schreibbare Speicherorte. | Nur in ausdrücklich eingerichtete Live-TV-Speicherorte, nur über `paths` (Regel 4), nie in Bibliotheksordner. | LTV-019 Z. 1040–1044 |
| A-8 | **Kinderprofil-Vertragstest** (Plan 10.2: keine Kennung ohne Freigabe in `scoped`-Antworten) *(von der Redaktion erkannt)* | Sender, Sendungen und Aufnahmen sind keine Titel mit Altersfreigabe; der Test zählt sie nicht als Verstoss. | Der Test bleibt für alle Titel unverändert; Live-TV-Antworten tragen keine `ItemRef` (`08-api.md`). | LTV-015 Z. 965 |
| A-9 | **Standard-Netz des Containers** (Bridge-Netz, nur Port 8096 veröffentlicht, `deploy/compose.yaml:29-40`) *(aus `04` 10.6 und review-1-sicherheit)* | Für Multicast darf der Container im Netzmodus «host» laufen (Zusatzdatei, Arbeitsname `compose.live-hostnet.yaml`); für UDP- oder RTP-Unicast an den Server dürfen UDP-Ports veröffentlicht werden (`compose.live-udp.yaml`). Ob statt «host» ein eigenes Netz des Containers (`macvlan`, `ipvlan`) gewählt wird, ist `04` F-11. | Nur als freiwillige Zusatzdatei, nie in der Standard-Compose-Datei; nur für Stufe H. Benannte Folgen im Wortlaut: (1) `127.0.0.1` ist dann das Loopback des Docker-Hosts; alles, was dort lauscht, liegt hinter jeder Lücke der Abwehr. (2) Der Server bekommt die IPv6-Adressen des Hosts; die bekannte Lücke «weltweite IPv6-Adresse im Heimnetz ist nicht erkennbar» (`netguard.go:44-47`) wird wirksam. (3) Die Annahmen über vertrauenswürdige Proxys und die HTTPS-Variante ändern sich (`compose.yaml:3-7`). (4) Ein lauschender UDP-Socket ist für jedes Gerät im Netz erreichbar. Die Verwaltung und die Installationsanleitung nennen diese Folgen. | LTV-003 Z. 439–441; Z. 1333 |
| A-10 | **ADR 0025 «FFmpeg sieht nie das Netz, nur Pipes»** (Webradio-Relay, ADR 0025 §2) *(aus `12` 1.1, RT-09)* | **Weg W:** Für DASH und HLS mit fMP4 oder getrennter Tonspur darf FFmpeg selbst über HTTP lesen, aber nur vom Go-Weiterleiter. | Nur DASH und HLS mit fMP4 oder getrennter Tonspur; nur unverschlüsseltes HTTP an `127.0.0.1`; jede Adresse auf einen Ersatzpfad des Weiterleiters umgeschrieben, und der Weiterleiter beantwortet nur diese (`05` 2.6, `12` SM-51); kein Tunnel, keine Proxy-Einstellung; nur mit der hart gesetzten Sperre des Prozesses; nur nach dem Beleg M-10; **nie** im Netzmodus «host». Ohne Sperre und ohne M-10 wird Weg W nicht gebaut (`04` F-13). Nur Stufe W. | LTV-003 Z. 435; Z. 1365 |
| A-11 | **Regel 3 «`os/exec` nur in `transcode`»** *(aus `04` 10.8, `12` 1.1, RT-09)* | Die Hülle, die FFmpeg eindämmt, darf sich in `internal/transcode` per `execve` durch FFmpeg ersetzen. | Nur in `internal/transcode`, nur für die Hülle, mit Argumentliste, nie Shell; die Lint-Muster dafür kommen mit Stufe 0 (`04` O-26). | Z. 1355, 1362 |
| A-12 | **«Ohne Recht unsichtbar» für jede Live-Operation** (`12` SM-38) *(aus `12` 1.1, RT-09)* | Das Hauptprofil darf die Aufnahmen seiner Profile mit Altersgrenze auch **ohne eigenes wirksames Live-TV** sehen und löschen, nicht abspielen. | **Gilt nur, wenn Marco bei `20` E-28-42 die Variante (a) wählt.** Bis dahin gilt die wortlauttreue Vorgabe (b): ohne Recht ist auch für das Hauptprofil alles unsichtbar, und SM-38 gilt ohne Ausnahme (`03` DR-03). | Z. 961, 820 |
| A-13 | **Live-Umwandlung nur mit wirksamer Sperre** (`04` T-27, `12` 6.3) | Der Betreiber darf die Umwandlung für Live-TV auch ohne Sperre erlauben. | **Gilt nur, wenn Marco bei `12` F-1 die Variante (c) wählt.** Ab Werk aus, mit Bestätigung und dauernder Anzeige des Restrisikos. Bis dahin gilt die sichere Vorgabe: ohne Sperre nur Direktweg (`03` DR-30). | Z. 1357, 1363 |

### 4.1 Festlegung der Architektur, die Grundsätze berührt

| Nr. | Festlegung | Berührt | Wirkung |
|---|---|---|---|
| T-29 (`04`) | **Das Deaktivieren oder Verschwinden eines Senders beendet nichts Laufendes von selbst; das Entfernen oder Ändern einer Quelle beendet ihre Empfänger.** Für alles Neue ist der Sender sofort unsichtbar (404). Laufende Wiedergaben laufen, bis der Zuschauer geht, höchstens 12 Stunden (das Token wird nicht erneuert); laufende Aufnahmen bis zu ihrem Ende, eine Sofortaufnahme höchstens bis zu ihrer Höchstdauer. Wird eine Quelle entfernt oder werden Adresse, Zugangsdaten, Ursprünge oder feste Ziele geändert, enden alle ihre Empfänger innert 5 Sekunden; Deaktivieren ist kein Sicherheitsmittel. Nur der ausdrückliche Befehl «sofort beenden» des Administrators, mit Zahl der Betroffenen und Bestätigung, beendet beides; eine so beendete Aufnahme heisst «unvollständig, Sender deaktiviert». | LTV-002.10, LTV-002.11 (Schutz laufender Wiedergaben und Aufnahmen), LTV-006.05, LTV-Q-10.56 (Widerruf) | Kein Widerspruch zu einem Grundsatz; er schützt sie. Bewusste Abweichung vom Webradio, das beim Ausblenden laufende Hörer beendet. Fachregel: `03` DR-16. |

---

## 5. Frage 37 im Entscheidungsprotokoll

Frage 37 ist eingetragen: `docs/planning/04-entscheidungen.md`, Zeile vom 28. November 2026 (Tabelle «Weitere
Entscheide während der Umsetzung»), mit dem Status «Konzept liegt vor, nicht freigegeben. Gebaut wird je Stufe erst
nach Marcos ausdrücklicher Baufreigabe». Zwei Freigaben sind zu unterscheiden (Lesart 8): «Konzept freigegeben»
und «Baufreigabe einer Stufe». Ohne Antwort auf eine offene Frage gilt deren wortlauttreue Vorgabe (Lesart 6). Massgeblich ist der Wortlaut dort; er enthält E-1 bis E-4, A-1 bis A-13,
T-29 und den Verweis auf `20-offene-entscheidungen.md`. Die Freigabe und die Antworten auf die offenen Fragen
kommen als Nachtrag zu Frage 37 (`18` Schritt 0.1). Die Liste der nachgeführten Dokumente steht in
`21-aenderungen-an-dokumenten.md`.

---

## Änderungen gegenüber der vorigen Fassung

Fassung 2 (Konsolidierung 1, 28. November 2026). Marcos Antworten E-1 bis E-4 sind unverändert.

- **A-3 präzisiert:** feste Ziele je Quelle (Adresse oder kleines Netz und Port, bestätigt), Prüfung für jede
  Verbindung, Verweise innerhalb der Ziele erlaubt, Loopback, Link-Local und eigene Adressen nie. Fassung 1 hätte
  mit «nicht für Verweise» jede lokale HLS-Quelle gebrochen. Quelle: `04` T-5, 10.9, 15.1; review-1-sicherheit.
- **A-9 neu:** Netzmodus «host» und veröffentlichte UDP-Ports als benannte Ausnahme mit vier Folgen im Wortlaut.
  Quelle: `04` 10.6; review-1-sicherheit (Pflicht vor Stufe 3, jetzt Stufe H).
- **A-6:** Verweis auf die eine Definition «Profil mit Altersgrenze» (`03` DR-25, `07` F-1) und auf `04` F-10
  (Aufnahmen vor einer nachträglichen Altersgrenze). Quelle: review-2 RD-05, review-1-sicherheit RS-09.
- **Abschnitt 4.1 neu:** T-29 (Deaktivieren beendet nichts Laufendes). Quelle: `04` Fassung 3.
- **Lesart und AB-5:** Stufenschnitt aus `18-umsetzungsplan.md`; Zahlen nach `04` Abschnitt 14.
- **Entwurf Frage 37** nachgeführt: feste Ziele, A-9, T-29, Stufen, Verweis auf die Sammelliste der Fragen.
- Restebereinigung nach Konsolidierung 1: Abschnitt 4, AB-4 und AB-20 auf den Stand gebracht, dass Regel 11, Spec,
  Plan, Vision, Handover, Frage 37, ADR 0046 (vorgeschlagen) und `CURRENT_STATUS.md` schon auf dem Konzeptzweig
  nachgeführt sind (`21`); Abschnitt 5 verweist statt des Entwurfs auf den eingetragenen Wortlaut von Frage 37.
  Quelle: spec-autor, `21` Abschnitt 5.

### Fassung 3 (Schlussbereinigung nach den Schlussreviews 4, 5 und 6, 29. November 2026)

- **Lesart 5 bis 8 neu:** `12` geht `04` vor, wo es strenger ist; ohne Antwort gilt die wortlauttreue Vorgabe
  (RG-01); wer was entschieden hat (RG-08); zwei Arten Freigabe nach Regel 11 (RG-06).
- **Abschnitt 3:** «Marco hat entschieden» nur für die 21 Grundsatzentscheide und E-1 bis E-4. **Neu 3.1:**
  Auslegungen des Koordinators AU-1 bis AU-10 mit wortlauttreuer Vorgabe (Rechte-Zuständigkeit, Zählung,
  Heimnetz, «Kinderprofil», Fire TV, Plattform-Streams, kein Download von Aufnahmen, Safari/iPhone/iPad, kein
  Internetdienst, kein Limit je Quelle). Quelle: review-4 RG-08, Entscheide des Koordinators.
- **AB-3:** Fire TV «berücksichtigt» statt Nicht-Ziel, als Auslegung zum Bestätigen. **AB-9, AB-10:** als Weiche
  in `20` (RG-15).
- **A-1:** Fire TV, Plattform-Streams und Internetdienst als Auslegung markiert; Wirkung von Frage 37 und
  Baufreigabe nach Regel 11 (RG-06).
- **A-3:** feste Ziele ab Werk als Einzeladresse mit Port, Netz höchstens /28 mit zweiter Bestätigung, nie mit
  Gateway; Entzug oder Änderung beendet Verbindungen innert 5 Sekunden. Quelle: `12` 4.5, RT-05, `04` T-5.
- **A-6:** Vorgabe von `20` E-28-07 (Teil A (b) mit Gegenrichtung, Teil B (a)); Restrisiko PIN-Ersatz. Quelle:
  RT-07, RT-20.
- **A-10 bis A-13 neu:** Weg W, Zusatz zu Regel 3, Hauptprofil ohne eigenes Live-TV (nur mit `20` E-28-42 (a)),
  Umwandlung ohne Sperre (nur mit `12` F-1 (c)). Quelle: RT-09, `12` 1.1.
- **T-29** nach `04` Fassung 4: 12 Stunden, Sofortaufnahme bis zur Höchstdauer, Quelle entfernen oder ändern
  beendet innert 5 Sekunden. Quelle: RB-05, RT-06.
- **Abschnitt 5:** zwei Freigaben, Grundregel, A-10 bis A-13 in Frage 37 nachzutragen.
- Letzte Angleichung: 3.1 verweist auf `20` E-28-59 bis E-28-62.
- Nach der Schlussprüfung (Review 7): Satz «A-10 bis A-13 nachzutragen» gestrichen (stehen in Frage 37 und ADR 0046).
