# ADR 0019: Verwaltungsleiste, Aktivitätsprotokoll und Neustart

- Status: angenommen (1. November 2026); **ersetzt teilweise Entscheid 14 N-2** für die Seiten unter
  `/admin/*` (die Verwaltungsleiste trägt Symbole, die App-Seitenleiste bleibt ohne) und **ergänzt
  ADR 0009** um ein Aktivitätsprotokoll neben der Liste der letzten Server-Fehler; **grenzt sich gegen
  ADR 0010 ab** (das Ereignisprotokoll der Webhooks bleibt getrennt)
- Quelle: Konzept `docs/planning/11-verwaltung-uebersicht.md` (Fassung 3, 1. November 2026,
  Abschnitte 10.1, 10.2 und 11); **Frage 17** in `docs/planning/04-entscheidungen.md` mit Marcos
  Antworten F-1 bis F-9 und O-1; Konzept 08 (`docs/planning/08-startseite.md`, N-2), Konzept 09
  (`docs/planning/09-hintergrundjobs.md`, Abschnitt 2 Punkt 4)
- Betroffene Spec-IDs: A-2, A-3, A-5, A-8
- Harte Regeln: 1 (generierter Code nur durch Codegen), 2 (keine neue Abfrage auf `items` ausserhalb
  der erlaubten Pakete; kein neuer Import von `store/systemq`), 5 (Migration 00031 nach Vorlage),
  7 (`x-visibility: admin` für `restartServer`, Eintrag im Vertragstest), 8 (Tastaturtest der Leiste
  und der Übersicht), 9 (Band-Farbe ohne neue Farb-Tokens, Sätze des Protokolls aus `de.json`/`en.json`),
  10 (keine Geheimnisse, Pfade, IP-Adressen oder freien Fehlertexte im Protokoll), 11 (keine
  Nicht-Ziele), 12 (der Server entscheidet über Hinweise und Zähler)

## Kontext

Marco hat am 31. Oktober 2026 an der heutigen Übersicht `/admin` gesagt: «Es soll einfach,
übersichtlich, symmetrisch und klar sein.» Die Übersicht zeigte 18 Bereichskarten, eine Tabelle der
letzten Fehlschläge und Kacheln untereinander; die Verwaltungsseiten waren durch eine waagrechte
Reiterleiste (`ui/AdminNav`) verbunden. Marco hat Variante B «Seitenleiste wie Emby» gewählt
(F-1), die Gruppentitel als getöntes Band (F-9), wenige Zahlen in der Leiste (F-7), und «Letzte
Aktivität» sowie «Neu starten» ausdrücklich gewünscht (F-6): «Neu starten soll unterstützt werden.
Letzte Aktivität soll ebenfalls erstellt werden.»

Drei Dinge waren zu entscheiden, die über eine einzelne Seite hinausgehen:

1. Wie die Verwaltung neben der App-Seitenleiste aus Konzept 08 steht, die nach N-2 ohne Symbole
   und ausblendbar ist.
2. Woher «Letzte Aktivität» ihre Einträge nimmt.
3. Wie ein Neustart ausgelöst wird, ohne eine laufende Sicherung oder Wiederherstellung zu stören.

## Entscheid

**Verwaltungsleiste.** Auf jeder Seite unter `/admin/*` steht für ein Administratorkonto mit
Hauptprofil links die Verwaltungsleiste **statt** der App-Seitenleiste, am selben Platz und in
derselben Breite (`--sc-size-sidenav-width`), ab 768 px immer, auch wenn das Gerät die App-Leiste
ausgeblendet hat. Das Menü-Symbol öffnet dort die Schublade der App. Die Leiste hat 20 Einträge
(Zurück zur App, Übersicht, 18 Seiten in vier Gruppen), **mit Symbolen** (Abweichung von N-2 nur hier).
Die Gruppentitel sind ein Band in der Profilfarbe zu 16 % (`--sc-band-tint` am Band, keine neuen
Farb-Tokens), Schrift in der Profilfarbe; fällt der Kontrast einer Profilfarbe unter 4,5:1, nimmt
das Band für diese Farbe `text-primary`. Tastatur: ein Tab-Halt, Pfeil hoch/runter, Pos1/Ende.
Unter 768 px öffnet ein Knopf «Verwaltung · *Seite*» die Leiste als Blatt. `index.html` setzt vor
dem ersten Bild `data-shell="admin"`, damit beim Laden nichts springt.

**Aktivitätsprotokoll.** Eine eigene Tabelle `activity_events` (Migration 00031, `STRICT`, Zeit als
Unix-ms) mit `kind` als String **ohne Enum und ohne CHECK** (eine neue Art bricht keinen Client),
`subject` für den Namen von Titel oder Bibliothek zum Zeitpunkt des Ereignisses und `detail` als JSON
mit wenigen Zahlen und festen Codes. Bekannte Arten: `playback_started`, `playback_completed`,
`playback_stopped`, `scan_started`, `scan_done`, `scan_failed`, `scan_aborted`, `job_failed`,
`backup_created`, `backup_restored`, `server_started`, `server_restart`. Regeln gegen Flut je Art
(Start höchstens einmal je 30 min, Profil und Titel; Pfad-Scans schreiben nichts; `job_failed` eine
Zeile je Jobart und Stunde mit Zähler). Geschrieben wird über `(*admin.Service).RecordActivity`, als
Haken in `playback`, `library`, `jobs` und `admin` verdrahtet; ein Schreibfehler wird geloggt und
bricht die Handlung nie ab. Aufbewahrung 30 Tage und höchstens 2'000 Zeilen, aufgeräumt über einen
Haken in `jobs.prune`. Gelesen wird nur über `Dashboard.activity` (die sechs neuesten). Der Server
schickt keine Sätze; die Oberfläche baut sie aus `kind` und Feldern, mit einem allgemeinen Satz für
Unbekanntes.

**Neustart.** `POST /admin/server/restart` (`restartServer`, `x-visibility: admin`) antwortet 202
mit `restartAt` und plant den Neustart wie die Wiederherstellung (`time.AfterFunc`, geordnet
herunterfahren, `syscall.Exec`; scheitert das, Exit 0 und Docker startet neu). Ein Merker
`restartPending` und `TryLock` auf die Sperre der Sicherung verhindern Überschneidungen: 409
`backup_busy`, solange eine Sicherung geschrieben wird, 409 `restart_pending`, solange ein Neustart
oder eine Wiederherstellung aussteht; die Wiederherstellung prüft denselben Merker. Die Warteanzeige
`RestartPanel` wird allgemein und von Neustart und Wiederherstellung benutzt.

## Warum eine eigene Tabelle (geprüfte Alternative)

Die Liste liesse sich aus vorhandenen Tabellen zusammensetzen (`playback_sessions`, Jobverlauf,
`webhook_events`). Das scheitert an drei Stellen: Serverstart, Neustart und Sicherungen stehen
nirgends; `playback_sessions` hat eine Zeile je Spurwechsel und keine Zeile «fertig geschaut»; die
Quellen haben verschiedene Aufbewahrung und brauchten eine UNION mit Regeln je Quelle. Eine kleine
Tabelle mit einer Aufbewahrung ist einfacher.

## Datenschutz

Das Protokoll hält fest, welches Profil wann was zu schauen begann oder fertig sah. Darum:

- Konto und Profil stehen nur als Id in der Zeile; E-Mail und Profilname liest der Server beim Lesen
  dazu. Beim Löschen eines Kontos oder Profils löscht er dessen Einträge mit.
- Keine Geheimnisse, keine Pfade, keine IP-Adressen und kein freier Fehlertext (Regel 10);
  `detail.reason` nimmt nur `root_missing`, `root_empty`, `loss_over_threshold`, `cancelled`, `error`.
- Lesen dürfen nur Administratorkonten mit Hauptprofil (`x-visibility: admin`), wie das Dashboard.
- 30 Tage Aufbewahrung.

## Was von ADR 0009 und ADR 0010 stehen bleibt

- Die Liste der letzten Server-Fehler (ADR 0009) bleibt, wie sie ist; auf der Übersicht steht sie
  zugeklappt als «Server-Protokoll» ganz unten (F-3), die Abnahme A-3 gilt unverändert.
- Das Ereignisprotokoll der Webhooks (ADR 0010, `webhook_events`) bleibt getrennt und auf
  «Integrationen»; eingehende Meldungen stehen nicht im Aktivitätsprotokoll.

## Restrisiken

- **Flut:** die Regeln je Art sind eine Schätzung. Zeigt der Beobachtungstag auf CT 190 sechs
  gleichartige Zeilen, wird die Regel der Art enger; die Obergrenze 2'000 hält die Tabelle klein.
- **Neustart während Streams oder Downloads:** der Dialog nennt beides; der Zuschauer startet den
  Titel neu, die Position kommt aus dem gespeicherten Fortschritt.
- **Pflegezähler** in `Dashboard.counts` auf grossen Beständen: mit 60-s-Zwischenspeicher selten,
  aber der erste Aufruf nach dem Start ist kalt (gemessen in Schritt 6).

## Folgen

- Vertrag 1.7.0 (additiv): `JobCounts.active`, `JobCounts.nextRun`, `JobCounts.failedByType`,
  `AdminStream.posterUrl`, `Dashboard.attention`, `Dashboard.counts`, `Dashboard.activity`,
  `restartServer` mit `RestartAccepted`.
- Neue Bausteine `ui/AdminSideNav`, `ui/AdminSideNavMenu`, `DashCard`, `TaskRow`, `StreamRow`,
  `ActivityFeed`; die waagrechte `ui/AdminNav` und die 18 Bereichskarten der Übersicht fallen weg.
- Die Tabelle der letzten Fehlschläge mit Erneut/Abbrechen steht nicht mehr auf der Übersicht
  (Abweichung von Konzept 09 Abschnitt 2 Punkt 4); sie bleibt auf «Hintergrundjobs».
