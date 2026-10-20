# iOS-Offline-Spike – Protokoll M1a (Entscheid in M1b)

Stand: 20. Oktober 2026 (M1a Schritt 1, Auswertung M1b Schritt 6, Android-Werte und endgültiger Formatentscheid vor M5). Grundlage: Plan `docs/planning/03-plan-v1.md` Abschnitt 10.4,
Annahmen AN-20 (Download-Format) und AN-7 (Testhardware), Entscheid 2 in
`docs/planning/04-entscheidungen.md` (kein iPhone, kein iPad, kein Mac).

Der Spike ist ein Wegwerf-Werkzeug: Testseite `web/public/spike/`, Endpunkt `GET /api/v1/spike/{variant}`
und die Testdateien unter `testdata/media/spike/` werden nach dem Entscheid in M1b entfernt.

## Ziel und Ablauf

**Ziel.** Prüfen, ob eine installierte PWA einen Film von 1.5 GB zuverlässig offline speichern, nach Tagen
noch abspielen und darin spulen kann (Spec O-1, O-4, O-6, O-7). Die Schwellen stehen in der Tabelle unten.
Mangels Apple-Geräten (Entscheid 2) laufen die Prüfungen auf Android (Handy oder Tablet) und auf dem
Desktop (Chrome, Firefox); die iOS-Spalte bleibt "ungeprüft".

**Testdateien.** `testdata/generate-spike-media.sh` erzeugt zwei Varianten derselben Aufnahme (1080p,
H.264 + AAC, rund 52 min, Keyframe alle 2 s): `spike-fmp4.mp4` (fragmentiertes MP4 mit globalem `sidx`,
1'613'262'329 Byte) und `spike-faststart.mp4` (`faststart`, 1'614'609'911 Byte).

**Server starten** (Entwicklungsrechner, Port 8096, im lokalen Netz erreichbar unter
`http://192.168.1.143:8096`):

```
task web:build
cd server && go build -o bin/slouchcat ./cmd/slouchcat
SLOUCHCAT_SPIKE_DIR=$(cd ../testdata/media/spike && pwd) ./bin/slouchcat serve --dev --listen :8096
```

Die Testseite liegt unter **`/spike/`** (mit Schrägstrich am Ende).

**Sicherer Kontext ist Pflicht.** OPFS, Service Worker und `persist()` gibt es nur unter HTTPS oder
`localhost`. Unter `http://192.168.1.143:8096/spike/` zeigt die Seite eine Warnung und kann nichts
speichern. Für das Android-Gerät deshalb:

1. USB-Debugging am Gerät einschalten, Gerät per USB anschliessen.
2. Auf dem Rechner `adb reverse tcp:8096 tcp:8096`.
3. Am Gerät in Chrome `http://localhost:8096/spike/` öffnen.

Ohne Kabel geht es mit `chrome://flags/#unsafely-treat-insecure-origin-as-secure` und dem Eintrag
`http://192.168.1.143:8096` (danach Chrome neu starten). Das Feld "Server base URL" der Seite bleibt leer,
solange Seite und Dateien vom selben Server kommen.

**Ablauf je Gerät und Browser.**

1. Seite öffnen. Auf Android über das Chrome-Menü "App installieren" bzw. "Zum Startbildschirm hinzufügen"
   als PWA installieren und ab dann nur noch die installierte App verwenden ("Installed PWA" muss `true`
   zeigen). Firefox auf dem Desktop kennt keine Installation; dort gilt die Prüfung im Browser-Tab.
2. Abschnitt 1: `persist()` anfragen (geschieht beim Laden automatisch, zusätzlich per Knopf). Quote und
   Schreibwege ablesen.
3. Abschnitt 2: je Variante (`fmp4`, `faststart`) einen Download starten, mindestens einmal mit dem
   Schreibweg `sync` (`createSyncAccessHandle` im Worker) und einmal mit `writable` (`createWritable`).
   Während des Downloads den Bildschirm sperren und die App wechseln, danach zurückkehren. Die Seite setzt
   nach einem Neuladen selbständig beim letzten vollständigen Block fort und zählt Wiederaufnahmen
   ("resumes after reload") und Neuversuche nach Netzfehlern ("retries after error"). Beim Weg `writable`
   ist ein Block erst nach dem nächsten Commit (Standard: alle 16 Blöcke à 8 MiB) dauerhaft gespeichert.
4. Abschnitt 3: für jede Datei "Blob URL: start + seek" und "Service worker: start + seek". Gemessen wird
   die Zeit vom Setzen von `currentTime` auf 25, 50 und 75 % bis zur weiterlaufenden Wiedergabe. Ein Lauf
   gilt nur, wenn die Dauer während des Laufs gleich bleibt und jeder Sprung auf der Zielposition landet.
5. Abschnitt 3, Soak: 30 Minuten Wiedergabe (Bildschirm bleibt dank Wake Lock an). Die Seite schreibt alle
   10 s einen Herzschlag; bricht die Seite ab, steht beim nächsten Öffnen "interrupted" im Protokoll.
6. Abschnitt 4 (Bestand): die App danach **nicht öffnen**, erst nach 1, 3 und 8 Kalendertagen wieder. Bei
   jedem Laden schreibt die Seite einen Eintrag `[persistence]` mit Dateien, Grösse und Alter; pro Datei
   zusätzlich "Play check" drücken.
7. Oben "Copy summary" drücken und die Gerätezusammenfassung unten einfügen; das ganze Protokoll mit
   "Export JSON" sichern (Datei `slouchcat-spike-<Zeitpunkt>.json`).

## Prüfungen und Schwellen (Plan 10.4)

| Prüfung | Schwelle | Android Chrome | Desktop Chrome | Desktop Firefox | iOS Safari (PWA) |
|---|---|---|---|---|---|
| `navigator.storage.persist()` | true (in installierter PWA) | bestanden: `true`, `persisted()` ja (Chrome 155, Android 10, im Chrome-Tab über `unsafely-treat-insecure-origin-as-secure`, nicht als installierte PWA; im Tab ohne sicheren Kontext vorher `false`) | n/a: Desktop-Chrome gibt `persist()` ohne Installation nicht frei; Funktionsprüfung lief im Tab | n/a (keine Installation; Firefox gewährt `persist()` per Nachfrage) | ungeprüft, keine Hardware (Entscheid 2, AN-7) |
| `navigator.storage.estimate().quota` | ≥ 5 GB | bestanden: 13.97 GB (belegt 3.23 GB) | bestanden (Desktop-Quote weit über 5 GB, Funktionsprüfung) | bestanden (Funktionsprüfung) | ungeprüft, keine Hardware (Entscheid 2, AN-7) |
| Schreibweg | `createSyncAccessHandle` im Worker oder `createWritable` funktioniert für 1.5 GB; gewählter Weg je Version dokumentiert | bestanden: beide Wege (Probe ok), beide Downloads mit `createSyncAccessHandle` | bestanden: beide Wege (Funktionsprüfung, Chrome 152) | bestanden: beide Wege (Funktionsprüfung, Firefox 158) | ungeprüft, keine Hardware (Entscheid 2, AN-7) |
| Download 1.5 GB mit Bildschirmsperre und App-Wechsel | abgeschlossen mit Wiederaufnahme ab letztem Block, ≤ 3 Wiederaufnahmen | bestanden: beide Dateien vollständig und grössengleich trotz 39 Wechseln in den Hintergrund und 10 eingefrorenen Phasen; fMP4 2 Wiederaufnahmen (15 Neuversuche nach Netzfehlern, 17 min aktiv, 12 Mbit/s), `faststart` 3 Wiederaufnahmen (22 Neuversuche, 19 min aktiv, 11 Mbit/s) | teilweise: Wiederaufnahme nach Neuladen ab letztem Commit geprüft; Bildschirmsperre am Desktop nicht prüfbar | teilweise: Abbrechen und Fortsetzen ab Commit geprüft | ungeprüft, keine Hardware (Entscheid 2, AN-7) |
| Wiedergabe aus OPFS per Blob-URL und per Range-Antwort des Service Workers | Spulen auf 3 Positionen je < 2 s mit mindestens einem der beiden Wege | bestanden: `faststart` per Blob-URL 82–95 ms und per Service Worker 102–108 ms; fMP4 siehe nächste Zeile | bestanden per Blob-URL (< 60 ms); Service-Worker-Weg im eingebetteten Chrome nicht prüfbar | bestanden per Blob-URL (< 100 ms); Service-Worker-Weg mit `faststart` bestanden | ungeprüft, keine Hardware (Entscheid 2, AN-7) |
| Download-Format (AN-20) | dieselbe Spulprüfung mit fMP4 (mit `sidx`) und mit `faststart`-MP4; bestanden mit fMP4 → AN-20 gilt, sonst Fallback `faststart` | fMP4 knapp bestanden: per Service Worker 1774/1795/1782 ms, per Blob-URL am 19.10. 1750 ms je Sprung, am 20.10. 1697/**12634**/1642 ms (nicht bestanden); `faststart` über beide Wege rund 0.1 s | fMP4 per Blob-URL bestanden | fMP4 per Blob-URL bestanden; **fMP4 über Service-Worker-Range nicht bestanden** (Dauer instabil, Sprung landet am Ende des gelesenen Bereichs); `faststart` über beide Wege bestanden | ungeprüft, keine Hardware (Entscheid 2, AN-7) |
| Speicher während 30 min Wiedergabe | kein Neuladen oder Absturz der PWA | ungültig: Lauf am 19.10. nach 14 s beendet, weil die Seite verlassen wurde (`pagehide` ohne Absturz, Heap 10 MB); Wiederholung durch Marco offen | nicht gemessen (nur 1-min-Soak in der Funktionsprüfung) | 1-min-Soak «survived» (Funktionsprüfung); 30 min nicht gemessen | ungeprüft, keine Hardware (Entscheid 2, AN-7) |
| Bestand nach 1, 3, 8 Kalendertagen ohne Öffnen | Datei vorhanden und abspielbar | Tag 1 (20.10.) bestanden: beide Dateien vorhanden (Alter 0.84 und 0.88 Tage), fMP4 spielt; Tag 3 und 8 ausstehend (bis 27.10.) | nicht gemessen | nicht gemessen | ungeprüft, keine Hardware (Entscheid 2, AN-7) |

Gerätebezeichnung, Betriebssystem- und Browserversion stehen in der jeweiligen Gerätezusammenfassung.

## Was Marco auf einem iPhone später selbst prüfen müsste

Sobald ein iPhone (aktuelles iOS, wenn möglich zusätzlich die Vorversion) mit ≥ 20 GB freiem Speicher
verfügbar ist, gilt derselbe Ablauf in Safari. Voraussetzung ist HTTPS mit einem Zertifikat, dem das iPhone
vertraut (z. B. Caddy mit eigener Domain im lokalen Netz), weil iOS keinen `adb reverse`-Ersatz und keinen
Flag für unsichere Ursprünge hat.

1. **`persist()`:** Seite in Safari öffnen, über "Teilen → Zum Home-Bildschirm" installieren, die App vom
   Home-Bildschirm starten ("Installed PWA" = `true`), "Request persist()" drücken. Erwartet: `true`.
   Zum Vergleich einmal im normalen Safari-Tab notieren.
2. **Quote:** in der installierten App "Quota" ablesen; erwartet ≥ 5 GB.
3. **Schreibweg:** Zeilen "createSyncAccessHandle in worker" und "createWritable" ablesen, danach je einen
   vollständigen Download mit `sync` und mit `writable` versuchen; notieren, welcher Weg auf welcher
   iOS-Version 1.5 GB schafft.
4. **Download mit Sperre und App-Wechsel:** Download starten, Bildschirm sperren, nach einigen Minuten
   entsperren, in eine andere App wechseln und zurück; wiederholen bis zum Abschluss. Zahl der
   Wiederaufnahmen und Neuversuche ablesen (Schwelle ≤ 3). Bricht iOS den Download bei Sperre jedes Mal ab,
   ist das laut 10.4 eine dokumentierte Abweichung ("Download nur bei geöffneter App"), kein No-Go.
5. **Spulen:** je Datei beide Wege ("Blob URL" und "Service worker") laufen lassen; jede der drei
   Positionen muss mit mindestens einem Weg unter 2 s liegen und die Dauer muss stabil bleiben.
6. **Download-Format:** Punkt 5 für `spike-fmp4-*.mp4` und für `spike-faststart-*.mp4` getrennt
   auswerten. Besteht fMP4, gilt AN-20; sonst Fallback `faststart`.
7. **30 min Wiedergabe:** Soak mit 30 Minuten starten, Gerät nicht anfassen. Nach Ablauf muss "survived"
   stehen; "interrupted" bedeutet Neuladen oder Absturz (No-Go-Kriterium für diesen Punkt).
8. **Bestand:** App schliessen und nicht öffnen; nach 1, 3 und 8 Kalendertagen öffnen, Abschnitt 4
   ablesen und "Play check" je Datei drücken. Fehlt die Datei nach 8 Tagen → No-Go.
9. Gerätezusammenfassung kopieren und hier einfügen, Protokoll als JSON exportieren.

## Gerätezusammenfassungen

Hier die mit "Copy summary" kopierten Blöcke einfügen, je Gerät, Browser und Prüftag (Tag 0, 1, 3, 8).

### Android Chrome

Lauf von Marco am 19. und 20. Oktober 2026, Export `slouchcat-spike-2026-10-20T11-48-30-317Z.json` (nicht im
Repository). Gerät mit Android 10, Chrome 155, Seite im Browser-Tab (nicht installiert) unter
`http://192.168.1.143:8096/spike/` mit dem Flag für unsichere Ursprünge.

```
Device summary 2026-10-20T11:49:11.679Z
- User agent: Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Mobile Safari/537.36
- Secure context: yes; installed PWA (standalone): no
- persist(): true; persisted(): yes
- Quota: 13.97 GB; usage: 3.23 GB
- createSyncAccessHandle (worker): yes (probe write ok); createWritable: yes (probe write ok)
- Service worker controls page: yes
Downloads (variant/path):
- faststart/sync: complete, size ok, 1.61 GB, attempts 4, resumes after reload 3, retries after error 22, active 19m 02s, avg 1.4 MB/s (11 Mbit/s)
- fmp4/sync: complete, size ok, 1.61 GB, attempts 3, resumes after reload 2, retries after error 15, active 17m 15s, avg 1.6 MB/s (12 Mbit/s)
Seek tests (latest per file and way, 25/50/75 %, threshold < 2000 ms):
- spike-fmp4-sync.mp4 via blob: start 320 ms, duration 3120 s; 25 %: 1697 ms, 50 %: 12634 ms, 75 %: 1642 ms -> fail
- spike-fmp4-sync.mp4 via sw: start 169 ms, duration 3120 s; 25 %: 1774 ms, 50 %: 1795 ms, 75 %: 1782 ms -> pass
- spike-faststart-sync.mp4 via blob: start 182 ms, duration 3120 s; 25 %: 95 ms, 50 %: 91 ms, 75 %: 82 ms -> pass
- spike-faststart-sync.mp4 via sw: start 193 ms, duration 3120 s; 25 %: 108 ms, 50 %: 102 ms, 75 %: 107 ms -> pass
Soak (30 min playback):
- 2026-10-19T15:39:32.742Z spike-faststart-sync.mp4 via blob: interrupted (page reloaded or crashed; last heartbeat 2026-10-19T15:39:46.329Z), played 13.6s, waiting events 6, JS heap 10 -> 10 MB (max 10)
Persistence (OPFS files now):
- spike-faststart-sync.mp4: 1.61 GB (1614609911 B), downloaded 2026-10-19T15:38:52.967Z, age 0.8 days, last play check ok at 2026-10-19T15:39:38.151Z
- spike-fmp4-sync.mp4: 1.61 GB (1613262329 B), downloaded 2026-10-19T14:42:09.711Z, age 0.9 days, last play check ok at 2026-10-19T15:39:39.218Z
Interruptions logged: 123
```

Auswertung des Protokolls (284 Einträge):

- Der Lauf vor 14:12 Uhr ohne sicheren Kontext zeigt «OPFS nicht verfügbar» und `persist() -> false`; ab dem
  Flag ist der Kontext sicher und `persist()` liefert `true`.
- Downloads: 39 Wechsel in den Hintergrund (Bildschirmsperre, App-Wechsel) und 10 eingefrorene Phasen (`freeze`,
  bis 6.5 min); nach jedem `resume` lief der Download weiter, nach Neuladen ab dem letzten Block.
- Die 123 Unterbrechungen sind fehlgeschlagene Blockabrufe («Failed to fetch») während der eingefrorenen
  Phasen; jeder Block wurde danach neu geholt.
- Spulen fMP4: Chrome auf Android braucht für jeden Sprung in der fragmentierten Datei rund 1.7 s (Blob-URL wie
  Service Worker), einmal 12.6 s. Die Datei hat ein Fragment je 2 s (rund 1560 Fragmente); Chrome liest offenbar
  die Fragmentköpfe bis zur Zielposition. In `faststart` stehen alle Positionen im Kopf, ein Sprung kostet 0.1 s.
- Soak: gestartet 15:39:32, `pagehide` um 15:39:56 (Seite verlassen), erst um 19:16 wieder geöffnet. Kein
  Absturz, aber keine gültige 30-min-Messung.

### Desktop Chrome

_(noch keine)_

### Desktop Firefox

_(noch keine)_

## Funktionsprüfung der Testseite (kein Ergebnis im Sinne von 10.4)

Vor der Übergabe wurde die Testseite auf dem Entwicklungsrechner gegen den lokalen Server (Port 8098)
durchgespielt, um Fehler der Seite selbst auszuschliessen. Die Werte stammen von `localhost` ohne
Bildschirmsperre und ohne installierte PWA und ersetzen keine Zelle der Tabelle oben.

- Headless Firefox 158 (WebDriver BiDi): beide Schreibwege funktionieren, beide Varianten vollständig
  geladen und in Grösse und `ftyp` geprüft, Abbrechen und Fortsetzen ab Commit funktionieren, Soak mit
  1 min "survived", Neuladen während des Soaks wird als "interrupted" erkannt.
- **Auffälligkeit für AN-20:** In Firefox meldet das fMP4 über den Service-Worker-Weg zuerst die Dauer
  `Infinity` und danach eine wachsende Dauer (nach 1.5 s rund 1012 s statt 3120 s); ein Sprung auf 2340 s
  landet am Ende des bisher gelesenen Bereichs. Per Blob-URL und für `faststart` über den Service Worker
  stimmt die Dauer (3120 s) und alle Sprünge liegen unter 100 ms. Die Seite wertet einen solchen Lauf als
  "fail" (Dauer instabil). Ob sich das auf echten Geräten und in Chrome gleich verhält, zeigt der Spike.
- Chrome 152 (eingebetteter Browser der Claude-App): Download beider Wege, Wiederaufnahme nach Neuladen
  (Fortsetzung genau ab Block 48 = letzter Commit), Abbrechen und Blob-Spulen (alle drei Sprünge unter
  60 ms) funktionieren. Der Service-Worker-Weg liess sich dort nicht prüfen, weil dieser eingebettete
  Browser keine Service Worker registriert; das muss im echten Chrome geschehen.
- Beim Weg `writable` steigt die Zeit für das erneute Öffnen mit `keepExistingData` mit der Dateigrösse
  (Chrome: 64 ms nach 8 Blöcken, rund 1 s nach 152 Blöcken), weil der Browser die Datei für jeden
  Schreibstrom kopiert. Die Commit-Zeiten stehen im Protokoll als Einträge `[commit]`.

## Go/No-Go (Entscheid in M1b)

_Stand 20. Oktober 2026 (M5, Vorabklärung): die Android-Werte von Marco (Tag 0 am 19. Oktober, Tag 1 am
20. Oktober) sind eingetragen. Format und Schreibweg sind damit endgültig entschieden. Offen bleiben nur der
Bestand an Tag 3 und Tag 8 (bis 27. Oktober) und eine gültige 30-min-Dauerwiedergabe auf Android; bis dahin
bleiben Spike-Seite und `SLOUCHCAT_SPIKE_DIR` im Server._

- **Go:** alle Schwellen erfüllt.
- **No-Go:** Datei fehlt nach 8 Tagen, `persist()` liefert auch in der installierten PWA `false`, Quote
  < 5 GB oder Spulen aus OPFS scheitert mit beiden Wegen → native iOS-App als erste Ergänzung nach V1,
  V1 liefert iOS-Offline mit Hinweis.
- Scheitert nur der Download bei Sperre: dokumentierte Abweichung (Download nur bei geöffneter App).
- Für iOS gilt bis zur Verfügbarkeit eines Geräts: ungeprüft (Entscheid 2, AN-7); das iOS-Risiko bleibt
  offen.

| Punkt | Entscheid | Begründung | Datum |
|---|---|---|---|
| Spike gesamt (Android/Desktop) | **Go unter Vorbehalt Tag 8** | Kein No-Go-Kriterium ist getroffen: `persist()` liefert auf Android `true`, die Quote liegt bei 13.97 GB, beide Downloads überstehen Bildschirmsperre und App-Wechsel mit höchstens 3 Wiederaufnahmen, und Spulen aus OPFS besteht mit mindestens einem Weg. Der Bestand nach 8 Tagen (27. Oktober) ist das letzte No-Go-Kriterium; die 30-min-Dauerwiedergabe auf Android ist zu wiederholen (der erste Lauf endete durch Verlassen der Seite). | 2026-10-20 |
| Download-Format AN-20 (fMP4 oder Fallback `faststart`) | **endgültig: fMP4 bleibt; Wiedergabe über Media Source Extensions mit eigenem Fragmentindex, nicht über Blob-URL oder Service-Worker-Range** | Formal besteht fMP4 die Schwelle von 10.4 (Android per Service Worker unter 1.8 s, Desktop per Blob-URL unter 0.1 s). Beide Wege sind für fMP4 aber unzuverlässig: Chrome auf Android braucht je Sprung rund 1.7 s und einmal 12.6 s, Firefox meldet über den Service-Worker-Weg eine falsche Dauer. Ursache ist, dass der Browser die Fragmente selbst suchen muss. M5 spielt Downloads deshalb mit hls.js aus OPFS: der Server liefert zum Download eine Liste der Fragmente mit Zeit und Byte-Bereich, ein eigener Loader liest genau diesen Bereich aus der Datei. So kennt der Player jede Position im Voraus, wie bei HLS-Streaming. Die fMP4-Vorteile aus AN-20 bleiben: der Server schreibt segmentweise und kann pausieren, das Gerät lädt bereichsweise und setzt nach Abbruch fort. **Fallback `faststart`** (der Server fügt die Segmente am Ende zu einer Datei mit Kopf zusammen, kurzzeitig doppelter Platz auf dem Server, Wiedergabe per Blob-URL wie im Spike mit 0.1 s) gilt nur, wenn die Messung in M5 Sprünge über 2 s zeigt. Die M5-Messung steht in `docs/perf/M5.md`. | 2026-10-20 |
| Schreibweg für M5 | **endgültig: `createSyncAccessHandle` im Worker, `createWritable` als Fallback** | Beide Wege funktionieren in Chrome 152/155 (Desktop, Android) und Firefox 158; beide Android-Downloads liefen über `createSyncAccessHandle`. `createWritable` kopiert die Datei je Schreibstrom und bleibt nur für Browser ohne Sync-Handle im Worker. | 2026-10-20 |
| iOS | ungeprüft, keine Hardware | Entscheid 2, AN-7; das iOS-Risiko bleibt offen, die Anleitung oben beschreibt die spätere Prüfung | 2026-10-19 |
