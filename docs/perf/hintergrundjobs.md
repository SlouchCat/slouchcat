# Messwerte: Hintergrundjobs (Konzept 09, ADR 0017)

Konzept `docs/planning/09-hintergrundjobs.md`, Schritt 5 (Abschnitt 10) mit den Risiken 12.3
(Schätzung gegen Wirklichkeit) und 12.4 (`jobs` ist gross, bevor `prune` das erste Mal lief).
Latte des Papiers: `listJobTypes` < 50 ms p95 auf CT 190 bei 50'000 Zeilen in `jobs`, vor dem
ersten `prune` und danach; dazu der Vergleich der Restzeit mit der Wirklichkeit an einem NAS-Scan.

Entwicklungsrechner (Linux Mint 22.3, i7-14700K, NVMe, wie `docs/perf/M3.md`), 30. Oktober 2026,
23:28 bis 23:39 Uhr, Binärdatei aus `task build` vom Stand `9ee3b72` (Servercode identisch mit
`b356da8`). Eigener Server (`serve`, `127.0.0.1:8099`, leerer Konfigurations- und Cache-Ordner,
ohne TMDB-Schlüssel, 4 Worker, Datenbank auf der lokalen Platte); die Medien liegen auf dem NAS
(`192.168.1.50:/mnt/nerdnas/medien`, NFS 4.2). Skripte: `docs/perf/measure-home.sh` mit dem Pfad
als Argument (5 Aufwärmläufe, dann 60 Anfragen per curl vom selben Rechner, p50/p95/max, Zahl der
Antworten ungleich 200; Sitzung mit gewähltem Hauptprofil) und `docs/perf/seed-jobs.py` (50'000
erledigte Läufe in der Mischung von CT 190 nach dem Import, siehe unten). Die Abfrage der
Übersicht alle 2 s während des Scans und die Auswertung liegen in den Sitzungsnotizen.

Die schweren Folgejobs des Scans (`probe.keyframes`, `segments.detect`, `trailer.find`,
`image.fetch`, `metadata.match`) waren über die neue API **pausiert**, damit die Messung das NAS
nicht mit Keyframe-Läufen teilt. Die Pause nahm vier laufende `probe.keyframes` sofort in die
Warteschlange zurück (12 wartend nach dem ersten Scan, 1'803 nach dem zweiten).

## 1. `listJobTypes` vor und nach dem ersten `prune` (12.4)

`docs/perf/seed-jobs.py` fügt 50'000 beendete Läufe ein: `probe.keyframes` 25'000, `image.fetch`
10'000, `segments.detect` 8'000, `metadata.match` 4'000, `metadata.fill` 2'000, `trailer.find`
400, `scan.library` 300, `library.sweep` 300 (das Verhältnis von CT 190 nach dem Import, 12.6);
`finished_at` gleichverteilt über die letzten 90 Tage, 2 % `failed`, 1 % `cancelled`, 5 % der Scans
und Sweeps `skipped`. Dazu kamen 5'423 echte Zeilen des Laufs (1'832 erledigt, 3'591 wartend und
pausiert): **55'423 Zeilen vor dem `prune`**.

`POST /admin/job-types/jobs.prune/run` löschte in **einem Lauf von 525 ms** 32'760 Zeilen
(erledigt und abgebrochen, älter als 30 Tage, die neuesten 50 je Jobart ausgenommen; `failed`
bleibt): **22'663 Zeilen nach dem `prune`** (17'867 erledigt, 169 abgebrochen, 1'036
fehlgeschlagen, 3'591 wartend).

| Aufruf | vorher p50 | vorher **p95** | vorher max | nachher p50 | nachher **p95** | nachher max | nicht 200 |
|---|---|---|---|---|---|---|---|
| `GET /admin/job-types` | 14.9 ms | **23.6 ms** | 33.8 ms | 9.0 ms | **23.6 ms** | 26.2 ms | 0 |
| `GET /admin/jobs?type=probe.keyframes&state=done&limit=50` | 13.9 ms | 20.2 ms | 27.3 ms | 2.1 ms | 8.6 ms | 8.9 ms | 0 |
| `GET /admin/jobs?type=scan.library&state=planned&limit=50` | 3.0 ms | 5.1 ms | 11.0 ms | 1.0 ms | 4.7 ms | 4.8 ms | 0 |
| `GET /admin/dashboard` | 8.2 ms | 10.1 ms | 11.3 ms | 6.8 ms | 8.9 ms | 12.6 ms | 0 |

Dieselben Zahlen aus dem Serverlog (`duration_ms` des Handlers, also ohne curl): vor dem `prune`
p50 14.2 ms, p95 23.4 ms, max 33.6 ms (n = 69); danach p50 9.4 ms, p95 23.2 ms, max 26.1 ms
(n = 65). Zum Vergleich die Übersicht alle 2 s **während des NAS-Scans** mit 4 Arbeitern und
5'423 Zeilen: p50 5.4 ms, p95 10.8 ms, max 25.7 ms (n = 91); ruhig mit 5'423 Zeilen: p50 8.6 ms,
p95 15.1 ms (n = 101).

Abfragepläne auf dem Stand mit 55'423 Zeilen (`EXPLAIN QUERY PLAN`, roh in SQLite, 20 Läufe):
`JobCountsByType` 1.5 ms über `jobs_claim (state)` mit temporärem B-Baum für `GROUP BY`;
`GetLastFinishedJobByType` 0.01 ms und `ListJobDurations` 0.02 ms über den deckenden Index
`jobs_history (type, state, finished_at)`; die Zahl der erledigten Zeilen einer Jobart 0.5 ms.

**Lesart.** Der Index aus Migration 00027 tut, was 12.4 verlangt: die Zeilenzahl bewegt den p50
(14.9 → 9.0 ms), den p95 nicht (23.6 ms in beiden Zuständen). Die Kosten der Übersicht sind die
18 Jobarten mit je fünf Abfragen (Zähler, letzter Lauf, Dauern für den Median, laufende Läufe,
Gegenstand) und die Antwort, nicht die Tabelle. Mit dem Faktor 2 bis 3 aus `docs/perf/M3.md` 1c
sind auf CT 190 (2 vCPU) **47 bis 71 ms p95** zu erwarten, also an oder über der Latte von 50 ms.
Die Messung auf CT 190 steht aus (Marco stellt den CT um, dann `measure-home.sh` gegen
`http://192.168.1.43:8096`); dort liegen echte Zeilen des Imports, der Fall «vor dem ersten
`prune`» ist aber nur kurz zu sehen, weil `jobs.prune` beim ersten Start feuert. Hält die Latte
auf CT 190 nicht, ist der Hebel eine Abfrage für die letzten Läufe aller Jobarten statt 18
Einzelabfragen, nicht ein weiterer Index.

## 2. Schätzung gegen Wirklichkeit an einem NAS-Scan (12.3)

Bibliothek «Filme» mit der Wurzel `/mnt/nerdnas/medien/Filme` (1'789 Filmordner, 1'791 Dateien,
Inhalt `movie`, Quelle TMDB ohne Schlüssel), angelegt über die API, erster Scan als Lauf 41,
Übersicht alle 2 s abgefragt. **Dauer 186 s.** Die drei Phasen nach den Abfragen und dem Code
(`scanner.go`):

| Zeit | Phase | Was die Übersicht zeigte |
|---|---|---|
| 0 bis 2 s | Durchsuchen (`walkRoot`, 1'795 Ordner) | `1 von ?` (erste Meldung), dann **bis 61 s unverändert**; `1'795 von ?` erschien erst mit dem Heartbeat nach 60 s |
| 2 bis 152 s | `planRoot`: `PartialHash` jeder neuen Datei über NFS (1'791 Dateien) | `1'795 von ?`, «wird berechnet»: **keine Meldung** in der längsten Phase |
| 152 bis 186 s | Schreiben in Batches zu 125 Operationen | `done von 1'791` alle 2 s, Schätzung «gemessen» |

Schätzung «gemessen» gegen die wirkliche Restzeit (Ende = erste Abfrage ohne den Lauf):

| t | Fortschritt | Schätzung | wirklich | Fehler |
|---|---|---|---|---|
| 152 s | 125 von 1'791 | 2'036 s | 34 s | +5'900 % |
| 158 s | 500 von 1'791 | 410 s | 28 s | +1'350 % |
| 164 s | 750 von 1'791 | 228 s | 22 s | +930 % |
| 170 s | 1'125 von 1'791 | 101 s | 16 s | +530 % |
| 177 s | 1'375 von 1'791 | 53 s | 10 s | +430 % |
| 183 s | 1'625 von 1'791 | 18 s | 4 s | +350 % |
| 185 s | 1'750 von 1'791 | 4 s | 2 s | +100 % |

Danach sechs Läufe «Jetzt ausführen» auf der unveränderten Bibliothek (Läufe 5413 bis 5423, je
zwei Bibliotheken): **jeder 0 s** (Durchsuchen aus dem Cache, nichts zu hashen, nichts zu
schreiben), `skipped = 0`; der Median von `scan.library` stand nach acht Läufen bei **0 s**.

**Befunde** (am 31. Oktober 2026 behoben, Abschnitt 2a; Papier 13.8):

- **B-1 Meldung bleibt liegen.** Der Reporter (`jobs/progress.go`) schreibt eine Meldung nur,
  wenn seit der letzten 2 s vergangen sind; sonst bleibt sie «pending» bis zur nächsten Meldung
  oder zum Heartbeat (60 s). Eine Phase, die in unter 2 s endet, ist bis zu 60 s unsichtbar: der
  Endstand des Durchsuchens (1'795 Ordner) kam 58 s zu spät. Hebel: ein Timer, der eine liegende
  Meldung nach `ProgressInterval` schreibt.
- **B-2 Die längste Phase meldet nichts.** `PartialHash` jeder neuen oder geänderten Datei in
  `planRoot` kostete hier 150 von 186 s (über NFS; auf CT 190 mit dem NAS mehr) und ruft
  `jobs.Progress` nie; die Seite zeigt so lange den Zähler des Durchsuchens mit unbekanntem
  Umfang. Hebel: `Progress(i, Anzahl neue Dateien, Ordner)` je Datei in `planRoot`.
- **B-3 «Gemessen» rechnet mit der falschen Zeit.** Die Rate ist `done / (jetzt − started_at)`
  (`admin/jobtypes.go`), also mit den Phasen ohne Fortschritt im Nenner: 2'036 s statt 34 s beim
  ersten Wert, noch +350 % drei Sekunden vor dem Ende. Hebel: die Rate ab dem Beginn der
  Fortschrittsreihe (erste Meldung mit `total > 0`), im Speicher der Laufliste gehalten (kein
  Schema), oder B-2, das die Phase ohne Fortschritt beseitigt — am besten beides.
- **B-4 Der Median kippt auf 0.** Ein Scan ohne Änderung dauert 0 s und zählt (`skipped = 0`);
  nach zwei davon ist der Median der Jobart 0 s, und jeder echte Scan zeigt ab der ersten Sekunde
  «länger als üblich». Das ist der Fall aus 12.3 («um mehr als Faktor 2 daneben»). Hebel: `Skip`
  im Scanner, wenn nichts geschrieben wurde (`len(ops) == 0` und nichts verloren), damit der Median
  nur Scans mit Arbeit zählt; die Alternative des Papiers (der Median-Fall verschwindet) nähme auch
  den anderen Jobarten die Schätzung.
- **B-5, in Ordnung:** Pause und Fortsetzen über die API wirkten sofort (vier laufende
  `probe.keyframes` zurück in die Warteschlange), `jobs.prune` räumte 32'760 Zeilen in einem Lauf
  von 525 ms, die Zähler der Seite blieben während des Scans unter 11 ms p95.

### 2a. Nachmessung mit den vier Hebeln (31. Oktober 2026, 00:18 bis 00:23 Uhr)

Gebaut auf Marcos Wort: B-1 ein Timer im Reporter schreibt eine zu frühe Meldung nach
`ProgressInterval`; B-2 `planRoot` meldet die Hash-Phase als Dateien von Dateien mit dem Ordner;
B-3 die Rate zählt die Arbeit seit Beginn der Fortschrittsreihe (erste Meldung mit Umfang, neu bei
anderem Umfang oder fallendem `done`; aus dem Speicher der Laufliste, `Registry.ProgressSince`),
dazu mindestens 10 s Reihe; B-4 ein Scan ohne neue, geänderte oder fehlende Datei ruft `Skip`.
Gleicher Aufbau wie oben, frische Datenbank, `go build` des Standes mit den Hebeln.

Erster Scan «Filme» (1'791 Dateien, Cache des NAS warm): **9 s**, die Hash-Phase meldete 386 und
1'141 von 1'791 nach 2 und 4 s, die Schreibphase 375 und 1'000 von 1'791; kein 60-s-Loch mehr
(B-1, B-2). Sechs Läufe «Jetzt ausführen» danach: 0 s und **`skipped = true`**, der Median bleibt
«unbekannt» statt 0 s (B-4). Erster Scan «KinderSerien» (3'749 Folgen, 304 Serienordner, Cache
kalt): **126 s**, Hash-Phase bis 119 s, Schreibphase 7 s (zu kurz für eine Schätzung, «wird
berechnet»). Schätzung «gemessen» gegen die wirkliche Restzeit bis zum Ende des Laufs (B-3):

| t | Fortschritt | Schätzung | wirklich (Lauf) | Fehler | wirklich (Phase) |
|---|---|---|---|---|---|
| 30 s | 950 von 3'749 | 90 s | 96 s | −6 % | 89 s |
| 45 s | 1'219 von 3'749 | 93 s | 81 s | +15 % | 74 s |
| 59 s | 2'442 von 3'749 | 31 s | 67 s | −54 % | 60 s |
| 75 s | 2'766 von 3'749 | 26 s | 51 s | −49 % | 44 s |
| 89 s | 3'025 von 3'749 | 21 s | 37 s | −43 % | 30 s |
| 103 s | 3'164 von 3'749 | 18 s | 23 s | −22 % | 16 s |
| 113 s | 3'421 von 3'749 | 10 s | 13 s | −23 % | 6 s |

**Lesart.** Statt Faktor 60 liegt die Schätzung innerhalb von Faktor 2, meist unter 25 %; die
Abweichung in der Mitte kommt vom NAS (ein Schub von 1'497 auf 2'442 Dateien in 6 s, danach
wieder langsam), nicht von der Rechnung. Die Schreibphase einer grossen Bibliothek bekommt ihre
eigene Reihe und nach 10 s ihre eigene Schätzung. Offen bleibt die Messung auf CT 190.

## 3. CT 190

Ausstehend: Marco stellt CT 190 auf das Image des Zweigs (Weg wie in 10.1), danach dieselben vier
Aufrufe mit `measure-home.sh` gegen `http://192.168.1.43:8096` (ein Tag Beobachtung der Zeitpläne,
12.1) und ein Scan der NAS-Bibliothek mit dem Blick auf die Restzeit; die Werte kommen hierher.
