# O-6-Langzeittest – Protokoll (Start M5, Abschluss M7)

Stand: 20. Oktober 2026 (M5 Schritt 0). Grundlage: Spec O-6 («Downloads haben kein Ablaufdatum»), Plan
`docs/planning/03-plan-v1.md` Abnahmematrix O-6 und P-9 (Prüfdauer 30 Tage auf Android und Desktop, Start in
M5, Abschluss in M7), Entscheid 2 (kein iPhone, kein iPad, kein Mac) und Entscheid 8.

## Ziel

Ein mit Slouchcat geladener Film bleibt 30 Kalendertage im Browser-Speicher (OPFS) und spielt danach im
Flugmodus ab, ohne dass die App in der Zwischenzeit geöffnet wurde. Ausser Löschen durch das Profil oder
Entzug des Zugriffs entfernt nichts den Download.

| Gerät | Start | Tag 1 | Tag 3 | Tag 8 | Tag 30 | Ergebnis |
|---|---|---|---|---|---|---|
| Android Chrome (installierte PWA, Marco) | ausstehend | | | | | |
| Desktop Chromium (Entwicklungsrechner, eigenes Browser-Profil) | ausstehend | | | | | |
| Desktop Firefox (Entwicklungsrechner, eigenes Browser-Profil) | ausstehend | | | | | |
| iOS Safari (installierte PWA) | ungeprüft, keine Hardware (Entscheid 2, AN-7); der 8-Tage-Test nach P-9 entfällt mangels Gerät | | | | | ungeprüft |

## Ablauf je Gerät

1. Slouchcat-Stack mit M5 starten (Entwicklungsrechner, Port 8096) und auf dem Gerät als PWA öffnen.
   Auf Android über `adb reverse tcp:8096 tcp:8096` und `http://localhost:8096` (sicherer Kontext, siehe
   `docs/perf/M1-ios-offline.md`), dann «App installieren».
2. Profil wählen, einen Film in Qualität «Mittel» laden (Detailseite, «Herunterladen»), warten, bis der
   Download in «Downloads» als fertig erscheint. Startdatum, Gerät, Browserversion, Titel, Grösse und die
   Speicheranzeige der Seite «Downloads» in die Tabelle eintragen.
3. Die App schliessen und nicht öffnen. Kein Löschen von Website-Daten, keine Bereinigungs-Apps.
4. An Tag 1, 3, 8 und 30: Gerät in den Flugmodus (Desktop: Netzwerk trennen), App starten, Download
   abspielen, auf 50 % springen, Untertitel einschalten. Bestanden heisst: die App startet offline, der
   Download ist da, Wiedergabe und Sprung funktionieren. Danach die App wieder schliessen.
5. Am Tag 30 Ergebnis eintragen; in M7 wird die Matrixzeile O-6 damit abgehakt.

## Einträge

_(je Prüftag: Datum, Gerät, Ergebnis, Auffälligkeiten)_
