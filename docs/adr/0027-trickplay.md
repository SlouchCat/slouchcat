# ADR 0027: Trickplay: Erzeugen beim Start, Springen an Keyframes, Einstellungen je Bibliothek

- Status: angenommen (9. November 2026)
- Quelle: Konzept `docs/planning/18-trickplay.md` Teil A (Abschnitte 0 bis 13, Fassung 9), **Frage 25**
  in `docs/planning/04-entscheidungen.md` (F-1 bis F-16, O-1); Messungen in
  `docs/perf/18-trickplay.md`
- Vertrag 1.23.0 (Teil A additiv; die Fassung setzt Teil B, ADR 0028), Migration 00042
- Harte Regeln: 1 (Codegen), 2 (Sichtbarkeit nur über `visibility`, nicht sichtbar heisst 404), 3
  (`os/exec` nur in `transcode`), 4 (Dateien nur über `paths`), 5 (Migration nach Vorlage), 6 (Blätter
  als `http.Handler`), 7 (`x-visibility`, Vertragstest), 12 (Entscheid im Server)
- Ändert ADR 0022 **nicht**: Trickplay nimmt nicht an der Pausen-Ausnahme teil.

## Kontext

Die Zeitleiste der Abspielseite zeigt beim Spulen bis 1.22.0 nur Zeit, Kapitelname und Kapitelbild.
Marco wünscht Vorschaubilder für Filme und Folgen, erzeugt sobald ein Titel startet (Frage 19 F-7
«später» ist aufgehoben). Die ganze Datei zu lesen dauert auf CT 190 für einen Film von zwei Stunden
160 s und liest 6 GB vom NAS; QSV hilft nicht, weil die GPU jedes Bild dekodiert. Gesucht war ein Weg,
der beim Start in Sekunden die ersten Bilder liefert, das NAS schont, einen laufenden Stream nicht
bremst und je Bibliothek einstellbar ist.

## Entscheid

1. **Springen statt Lesen.** Je Bildpunkt (alle 10 s) ein eigener ffmpeg-Prozess:
   `-threads 1 -noaccurate_seek -ss <t> -i <datei> -map 0:v:<n> -frames:v 1 -fps_mode passthrough`,
   skaliert auf 320 px Breite (Höhe nach Anzeige-Seitenverhältnis, gerade), HDR und Dolby Vision mit
   der CPU-Tonemapping-Kette der Kapitelbilder. Ein Bild liest rund 1 MB und dekodiert 3 bis 4 Frames;
   eine Folge von 61 Minuten braucht 19 s, ein Film von 2 Stunden 42 s (zwei Prozesse). Der Bildpunkt
   ist das **nächste Keyframe** im 10-s-Abschnitt aus dem Keyframe-Index (ADR 0022), angesprungen auf
   Keyframe + 1 ms; ohne Index der Abschnittsanfang (`fallback` im Manifest). Liegt kein eigenes
   Keyframe im Abschnitt, kopiert der Job die vorige Kachel.
2. **Blätter.** 10 × 10 Kacheln je Blatt, WebP Qualität 75, das letzte Blatt nur so hoch wie nötig.
   Rund 1,3 bis 1,8 MB je Stunde Video. Ein Satz ist ein Ordner je Datei und Dateiversion
   `<mediaFileId>-<size>-<mtime>` mit `manifest.json` (zuerst), den Blättern (je unter Zwischennamen
   geschrieben, auf das Laufwerk geschrieben und umbenannt) und dem leeren Merker `complete`
   (zuletzt). Ein abgebrochener Lauf macht beim nächsten Mal nur die fehlenden Blätter. Die Version
   des Satzes (`Trickplay.version`, `v` der Blatt-Adressen, ETag) ist
   `<size>-<mtime>-<quelle><kachelhöhe>`: Wird ein vollständiger Satz neu gebaut (ein
   `fallback`-Satz, sobald die Datei einen Index hat; eine Kachelhöhe, die nicht mehr passt), entsteht
   er im Ordner `<name>.next` daneben, der alte dient bis zum Tausch, und die neuen Blätter bekommen
   neue Adressen. Die Kachelhöhe ist auf 1280 px begrenzt (ein Blatt bleibt unter den 16 383 px von
   libwebp); einem gespeicherten Probe ohne Seitenverhältnis der Bildpunkte (vor 1.23.0) liest der
   Job es einmal aus der Datei nach.
3. **Einstellung je Bibliothek, getrennt für Filme und Serien** (Migration 00042, fünf Spalten in
   `libraries`): Filme `ahead` (ab Werk), `on_start` mit `keep` oder `off`; Serien zusätzlich
   `series_start` mit Umfang `series` (ab Werk), `season_rest` oder `next_episode`. Gespeichert wird
   unter `cache/trickplay/` bei `ahead`, `series_start` und `on_start` mit `keep`, sonst unter
   `cache/trickplay-session/` nur so lange, wie eine Wiedergabe-Sitzung der Datei offen ist (plus
   10 Minuten). Hörbücher, Podcasts und Radio bekommen nie Trickplay.
4. **Jobarten.** `trickplay.generate` (Pool `trickplay` mit **einem** Arbeiter, O-1, `Windowable`,
   Schlüssel `trickplay:<mediaFileId>`, Payload `mediaFileId`, `startMs`, `origin`),
   `trickplay.series` (Pool `general`, Schlüssel `trickplay_series:<seriesId>`) und
   `trickplay.backfill` (Zeitplan alle 7 Tage, ab Werk aus). Die Herkunft `origin` (`start` vor
   `download` vor `series` vor `import` vor `backfill`) steht im Auftrag, die Einstellung nicht: der
   Job liest sie beim Start und nach jedem Blatt neu und endet, wenn sie seine Herkunft nicht mehr
   auslöst oder nicht speichert und keine Wiedergabe mehr offen ist.
5. **Vorrang ohne Abbruch.** Start-Aufträge laufen mit Priorität 9, Downloads 5, die nächsten zwei
   Folgen des Serienvorlaufs 3, übrige Folgen und Import 2, Nachholen 1. Jeder Auftrag ausser einem
   Start prüft nach jedem Blatt, ob ein Start-Auftrag wartet, und gibt dann ohne Versuch ab
   (`jobs.RetryLater`); seine Blätter bleiben. Zwei ffmpeg-Prozesse, nur einer solange ein Transcode
   die CPU belegt (`transcode.Manager.CPUHeavy`) und für gedrosselte Vorlauf-Aufträge, solange eine
   Wiedergabe läuft.
6. **Keine Pausen-Ausnahme.** Eingereiht wird nur über `Enqueue` und `EnqueueOverridingWindow`
   (Start, Vorlauf, Download gehen am Zeitfenster vorbei), nie über `EnqueueOverridingPause` oder
   `RaiseQueuedJobForPlayback`. Pausiert der Admin `trickplay.generate`, pausiert es ganz;
   `MaxPauseOverrides` zählt Trickplay nicht.
7. **Fehler.** Je Bildpunkt höchstens 15 s; ein kaputter Bildpunkt wird schwarz gefüllt, fünf in
   Folge oder eine unlesbare Datei mitten im Lauf beenden den Auftrag sofort als fehlgeschlagen
   (die Arbeiter brechen das Blatt ab, `jobs.Final`), fertige Blätter bleiben. Der Auftrag hat zwei
   Versuche; der zweite dient nur einem Lauf, den ein abgelaufener Lease abschnitt (Absturz), damit
   ein Import bei «Im Voraus» ohne Zeitplan nicht liegen bleibt. Endet der letzte Auftrag ohne
   vollständigen Satz (fehlgeschlagen, abgebrochen, unlesbare Datei, nicht mehr gewünscht), meldet
   der Zustand `failed`, und der Player fragt nicht weiter nach. Eine unlesbare Datei beim Beginn: Start, Vorlauf, Download und nicht speichernde
   Aufträge enden ohne Wiederholung, Import und Nachholen warten wie die Kapitelbilder (1 h, Aufgeben
   nach 7 Tagen). Ändert sich die Datei während des Laufs, endet er; der Probe reiht neu ein.
8. **Speicherplatz.** Unter 10 GB oder 10 % frei auf dem Cache-Laufwerk stoppen Import, Nachholen und
   Vorlauf; ein Start schreibt dann einen temporären Satz. Beides steht im Protokoll und als Notiz
   des Laufs auf «Hintergrundjobs».
9. **Pflege** ohne eigene Jobart in `library.sweep` (stündlich) und der Wiedergabe-Pflege (minütlich):
   Sätze anderer Versionen, verschwundener Dateien, gelöschter Titel, temporäre Sätze ohne offene
   Sitzung seit 10 Minuten und gespeicherte Sätze einer Titelart, die nicht mehr speichert; nie die
   Ordner einer Datei, deren Auftrag wartet oder läuft, Zwischennamen erst nach 6 Stunden.
10. **Vertrag.** `PlaybackInfo.trickplay` (Schema `Trickplay`) kommt bei jedem Start ausser «Aus» mit,
    beim ersten Mal mit `state: queued` und `sheets: []`; `getTrickplay` zum Nachfragen,
    `getTrickplaySheet` (`http.Handler`, `immutable` mit passendem `v`) und
    `getDownloadTrickplaySheet` (Download-Token). Alle `scoped`: unsichtbar, fremde Datei oder
    fehlendes Blatt heisst 404. `Library.trickplay` beim Anlegen, Ändern und Lesen;
    `LibraryTrickplayEstimate` nur in `getLibrary`. `DownloadManifest.trickplay` beschreibt die
    mitgenommenen Blätter; der Download wartet höchstens 10 Minuten auf einen fehlenden Satz.

## Folgen

- Beim ersten Start eines Titels ohne Satz erscheint das erste Blatt (gut 16 Minuten um die
  Startposition) nach 5 bis 10 s, der ganze Satz nach 20 bis 45 s (4K HDR rund das Zweieinhalbfache).
  Bis dahin zeigt die Zeitleiste das Kapitelbild.
- Starten zwei Zuschauer gleichzeitig zwei Titel ohne Satz, wartet der zweite auf den ersten Satz
  (O-1, hingenommen).
- Der ganze Bestand braucht im Voraus rund 55 GB Cache und bei zwei Prozessen 7 bis 8 Tage Nachholen;
  das Nachholen steht ab Werk ohne Zeitplan.
- Ein später eintreffender Keyframe-Index verbessert einen fertigen `fallback`-Satz nicht von selbst;
  das Nachholen baut ihn neu.
- Offen bis Bauschritt 8: Last von 4K über NFS, Trickplay neben CPU-Tonemapping und neben einem
  4K-Remux.

## Nachtrag vom 9. November 2026 (1.23.1)

Marco hat nach der Abnahme von 1.23.0 entschieden: **«Aus» gilt ab Werk** und steht in der Auswahl
zuoberst, für Filme und Serien. Migration **00043** stellt jede Bibliothek, die noch auf «Im Voraus»
steht, je Art auf «Aus»; «Beim Start», «Beim Start alle Folgen», «Aus», «Speichern» und der Umfang
bleiben. Der Spaltenvorgabewert `'ahead'` aus 00042 bleibt im Schema, wird aber nicht mehr gelesen:
`library.Service.Create` schreibt die Einstellung jeder neuen Bibliothek ausdrücklich
(`model.DefaultLibraryTrickplay`). Einen Vorgabewert ändert SQLite nur mit einem Neubau der Tabelle
`libraries`, auf die ein grosser Teil des Schemas verweist; das lohnt sich für einen ungenutzten Wert
nicht. Folgen: Wartende Aufträge dieser Bibliotheken enden ohne Arbeit (`trickplayWanted`), die nächste
Pflege entfernt ihre gespeicherten Sätze. Der Vertrag bleibt 1.23.0 (nur Beschreibungstexte).
