# O-6-Langzeittest – Protokoll (Start M5, Abschluss M7)

Stand: 20. Oktober 2026 (M5). Grundlage: Spec O-6 («Downloads haben kein Ablaufdatum»), Plan
`docs/planning/03-plan-v1.md` Abnahmematrix O-6 und P-9 (Prüfdauer 30 Tage auf Android und Desktop, Start in
M5, Abschluss in M7), Entscheid 2 (kein iPhone, kein iPad, kein Mac) und Entscheid 8.

## Ziel

Ein mit Slouchcat geladener Film bleibt 30 Kalendertage im Browser-Speicher (OPFS) und spielt danach ohne
Server ab, ohne dass die App in der Zwischenzeit geöffnet wurde. Ausser Löschen durch das Profil oder Entzug des
Zugriffs entfernt nichts den Download.

| Gerät | Start | Tag 1 (21.10.) | Tag 3 (23.10.) | Tag 8 (28.10.) | Tag 30 (19.11.) | Ergebnis |
|---|---|---|---|---|---|---|
| Desktop Chromium 156 (Entwicklungsrechner, eigenes Browser-Profil) | 20.10.2026, «Slouchcat Test H264» in «Mittel», 2'658'803 Byte; Tag 0 ohne Server bestanden (Kaltstart 1334 ms, Sprung auf 50 % 41 ms) | | | | | läuft |
| Desktop Firefox 157 (Entwicklungsrechner, eigenes Browser-Profil) | 20.10.2026, derselbe Film, 2'658'803 Byte; Tag 0 ohne Server bestanden (Kaltstart 1446 ms, Sprung auf 50 % 19 ms) | | | | | läuft |
| Android Chrome (installierte PWA, Marco) | ausstehend: braucht einen Server mit M5, also das neu gebaute Image nach dem Merge (siehe unten) | | | | | |
| iOS Safari (installierte PWA) | ungeprüft, keine Hardware (Entscheid 2, AN-7); der 8-Tage-Test nach P-9 entfällt mangels Gerät | | | | | ungeprüft |

Abweichung vom Plan: Der Plan wollte den Test am ersten Tag von M5 starten. Ein Download gibt es aber erst mit
dem fertigen M5-Server; der Desktop-Teil startete deshalb am Ende des M5-Tages, der Android-Teil startet mit dem
Image nach dem Merge. Der Spike-Download von Marco (seit 19.10. auf Android, `docs/perf/M1-ios-offline.md`)
belegt bis zum 27.10. zusätzlich, dass OPFS-Dateien auf Android acht Tage überdauern.

## Desktop: Ablauf mit dem Skript

Das Skript `web/scripts/o6-longterm.mjs` hält seinen Zustand ausserhalb des Repositorys in
`~/.local/share/slouchcat-o6` (Serverdaten, die zwei Browser-Profile, das Konto mit Zufallspasswort und
`o6-log.json` mit einem Eintrag je Lauf). Voraussetzung: `task build` und `task testdata:media`.

```bash
node web/scripts/o6-longterm.mjs start
```

startet einen eigenen Server auf Port 8098 (Assistent mit der Filmbibliothek der Testmedien), meldet sich in
Chromium und in Firefox mit je einem dauerhaften Browser-Profil an, lädt den Film «Slouchcat Test H264» in
«Mittel» über die App (Service Worker, OPFS, IndexedDB) und beendet den Server wieder.

```bash
node web/scripts/o6-longterm.mjs check
```

verlangt, dass kein Server auf Port 8098 antwortet (die Browser sind damit wirklich offline, auch Firefox, dessen
Flugmodus in Playwright den Service Worker nicht erfasst), öffnet die App in beiden Profilen, erwartet die
Offline-Bibliothek, wählt das Profil, spielt den Download ab, springt auf 50 % und schreibt Kaltstartzeit,
Sprungzeit und Ergebnis ins Log. Bestanden heisst: Offline-Bibliothek in unter 3 s, Download vollständig mit allen
Bytes, Wiedergabe und Sprung auf 50 % in unter 2 s (das Skript bewertet alle drei).

`persist()` ist in diesen Playwright-Profilen nicht gewährt (`persisted: false`, Chrome gewährt die Dauerhaftigkeit
nach Nutzung oder Installation). Der Desktop-Test prüft damit den schwächeren Fall: Der Browser darf den Speicher
unter Speicherdruck räumen, auf dem Entwicklungsrechner mit rund 10 GB Quote und 5 MB Belegung tritt das nicht ein.

## Android: Ablauf (Marco)

1. Nach dem Merge von M5 und dem neuen Image läuft der Slouchcat-Stack mit M5 auf dem Entwicklungsrechner (Port
   8096). Am Gerät über `adb reverse tcp:8096 tcp:8096` und `http://localhost:8096` öffnen (sicherer Kontext,
   siehe `docs/perf/M1-ios-offline.md`), dann über das Chrome-Menü «App installieren».
2. Profil wählen, einen Film in «Mittel» herunterladen (Detailseite, «Herunterladen»), warten, bis er unter
   «Downloads» als fertig erscheint. Datum, Gerät, Chrome-Version, Titel, Grösse und die Speicheranzeige der Seite
   «Downloads» (Belegung, Quote, «dauerhaft» ja/nein) notieren.
3. Die App schliessen und nicht öffnen. Kein Löschen von Website-Daten, keine Bereinigungs-Apps.
4. An Tag 1, 3, 8 und 30: Flugmodus einschalten, App starten. Erwartet: «Offline-Bibliothek», Profil wählen,
   Download abspielen, auf 50 % springen, Untertitel einschalten. Danach die App wieder schliessen.
5. Werte hier eintragen; am Tag 30 wird die Matrixzeile O-6 in M7 abgehakt.

## Einträge

| Datum | Gerät | Lauf | Ergebnis | Auffälligkeiten |
|---|---|---|---|---|
| 20.10.2026 | Desktop Chromium 156 | `start` | Download vollständig (2'658'803 Byte), Belegung 5'084'930 Byte von 10'742'503'170 | `persisted: false` |
| 20.10.2026 | Desktop Firefox 157 | `start` | Download vollständig (2'658'803 Byte), Belegung 4'818'205 Byte von 10'737'418'240 | `persisted: false` |
| 20.10.2026 | Desktop Chromium 156 | `check` (Tag 0, kein Server) | bestanden: Kaltstart 1334 ms, Sprung 41 ms | – |
| 20.10.2026 | Desktop Firefox 157 | `check` (Tag 0, kein Server) | bestanden: Kaltstart 1446 ms, Sprung 19 ms | – |
| 20.10.2026 | Desktop Chromium 156 | `check` (Tag 0, zweiter Lauf mit der strengeren Bewertung) | bestanden: Kaltstart 1357 ms, Download vollständig, Sprung 42 ms | – |
| 20.10.2026 | Desktop Firefox 157 | `check` (Tag 0, zweiter Lauf mit der strengeren Bewertung) | bestanden: Kaltstart 1427 ms, Download vollständig, Sprung 18 ms | – |
