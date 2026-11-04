# ADR 0021: Transcode bis 2160p auf Hardware-Backends

- Status: angenommen (4. November 2026, Entscheid vom 1. November 2026); **ändert die Umsetzung
  von AN-17 / Frage 4** (die Obergrenze 1080p gilt nur noch für Software-Transcoding), gebaut in
  Konzept 13, Stufe 2, Schritt 12
- Quelle: Konzept `docs/planning/13-abspielseite.md` (Fassung 3, Abschnitte 11.5, 11.9, 11.12 und
  11.14), **Frage 19** in `docs/planning/04-entscheidungen.md` (F-8, F-14), Plan
  `docs/planning/03-plan-v1.md` Abschnitt 3.5 (Admission Control)
- Betroffene Spec-IDs: W-2, W-3, W-8
- Harte Regeln: 3 (`os/exec` nur in `transcode`), 5 (Migration nach Vorlage für den erweiterten
  `CHECK` von `playback_sessions.quality`), 12 (der Server entscheidet die Stufe)

## Kontext

AN-17 (Frage 4) erlaubt beim **Software**-Transcoding (libx264 `veryfast`) höchstens 1080p-SDR,
auch bei 4K-HDR-Quellen. Der Code deckelt aber **jeden** Transcode auf 1080 Zeilen, auch auf
Hardware-Backends:

- `MaxTranscodeHeight = 1080` in `server/internal/playback/decide.go`.
- `heightLimit` in `server/internal/transcode/plan.go` klemmt jede Höhe über 1080 auf 1080.
- `-level:v 4.1` in `pipeline.go` und der HLS-Codec-String `avc1.640029` in `plan.go` reichen nur
  bis 1080p.
- `playback_sessions.quality` hat einen `CHECK` bis `1080p` (Migration 00011).
- Die Qualitätsleiter steht doppelt: `transcode` (`ladder`, `QualityFor`, `Rungs`) und
  `playback/quality.go`.

Marco will im Qualitäts-Panel die Stufen «Original» und **«2160p» (25 Mbit/s)** (F-8). Er weiss,
dass CT 190 HDR nur auf der CPU tonemappt und 2160p dort fast nie wählbar ist, und will die Stufe
trotzdem (F-14).

Die Kosten der Admission Control (Plan 3.5, `transcode.Cost`) folgen heute der Quelle und der
Pipeline: 4K-HDR mit Tonemapping 4, anderes 4K 3, bis 1080p 2, Ausgabe bis 720p 1,5, Remux 0,5,
nur Ton 0,25. Ein Budget je Backend: NVENC 12, QSV/VAAPI 8, Software Kerne/2. Eine 4K-Quelle mit
1080p-Ausgabe kostet 3. Ein 2160p-Ausgang kodiert aber viermal so viele Bildpunkte wie 1080p.

## Entscheid

1. **Grenze je Backend.** Die höchste Ausgabehöhe eines Transcodes ist 1080 beim
   Software-Backend und 2160 bei Hardware-Backends (NVENC, QSV, VAAPI). `MaxTranscodeHeight` wird
   eine Funktion des Backends. `heightLimit` klemmt nicht mehr fest auf 1080.
2. **Wann 2160p wählbar ist.** Alle Bedingungen müssen gelten:
   - Die Quelle hat mindestens 2160 Zeilen.
   - Ein Hardware-Backend läuft.
   - Das Profil erlaubt `max_height` 2160.
   - Das Gerät meldet mindestens 1440 Zeilen.
   - Die Quelle ist **SDR**, oder das Backend tonemappt auf der GPU.
3. **Tonemapping bleibt bei höchstens 1080p auf der CPU.** Die Regel «Tonemap immer mit
   MaxHeight ≤ 1080» (`plan.go`) gilt weiter für CPU-Tonemapping. 2160p mit Tonemapping gibt es
   nur auf GPU-Backends (zum Beispiel `tonemap_cuda`). Auf CT 190 (QSV, Tonemapping auf der CPU) ist
   2160p damit nur bei 4K-SDR-Quellen wählbar.
4. **`auto` wählt nie selbst 2160p.** 2160p gibt es nur auf ausdrückliche Wahl im Panel.
   V-17 merkt eine Stufe unter 2160p als `qualityCap`.
5. **Kosten 5.** Ein Transcode mit 2160p-Ausgang kostet **5** Einheiten, unabhängig vom Backend.
   Er dekodiert und kodiert in 4K. Auf QSV mit 8 Einheiten läuft damit ein 2160p-Strom neben
   einem 1080p-Strom (5 + 2), nicht zwei 2160p-Ströme. Der Wert wird in Schritt 12 auf CT 190
   gemessen. Weicht die Messung ab, wird der Wert in einem Nachtrag zu diesem ADR korrigiert.
6. **Level und Codec-String aus der Ausgabehöhe.** Bis 1080p bleibt es bei Level 4.1 und
   `avc1.640029`. 2160p bekommt Level 5.1 und `avc1.640033`. Beides wird aus der Ausgabehöhe
   abgeleitet und steht nicht mehr fest im Code.
7. **Eine Leiter.** `ladder`, `QualityFor` und `Rungs` in `transcode` bekommen 2160p (25 Mbit/s).
   Die Kopie in `playback/quality.go` entfällt, `playback` liest `transcode.Rungs`.
8. **Datenbank.** `playback_sessions` wird nach der Vorlage neu aufgebaut (Regel 5, neue Tabelle,
   kopieren, umbenennen, `foreign_key_check`). Der `CHECK` erlaubt dann `original` und `2160p`.
   Der Migrationstest fügt beide Werte ein.

## Folgen

- Auf Rechnern mit GPU-Tonemapping (NVENC mit `tonemap_cuda`) ist 4K-HDR als 2160p-SDR wählbar.
  Auf CT 190 bleibt es bei 4K-SDR-Quellen. Das ist selten, und Marco weiss es (F-14).
- Ein 2160p-Strom belegt mehr als die Hälfte des QSV-Budgets. Weitere Starts warten bis 10 s und
  werden dann abgewiesen (Plan 3.5). Das ist gewollt: 2160p ist eine ausdrückliche Wahl.
- Die Software-Grenze aus AN-17 bleibt unverändert. Ohne GPU erscheint 2160p nicht im Panel.
- Vor dem Merge von Stufe 2 misst Schritt 12 auf CT 190 für 2160p (SDR) Start, Spulen und
  gleichzeitige Ströme (`docs/perf/13-abspielseite.md`).

## Umsetzung (Stufe 2, Schritt 12, Teil Code, 4. November 2026)

- `transcode.MaxOutputHeight(backend, cpuTonemap)` ist die Grenze aus Punkt 1 und 3: 2160 auf
  NVENC, QSV und VAAPI, 1080 auf Software, bei unbekanntem Backend und bei Tonemapping auf der CPU.
  `playback.MaxTranscodeHeight(backend)` liest sie.
- `transcode.Target` trägt dafür `Backend` und `HWTonemap` (aus `Capabilities`). Ohne Backend
  bleibt jeder Transcode bei 1080 Zeilen. `MaxHeight` 0 heisst weiterhin 1080 (Punkt 4): über
  1080 geht es nur mit `MaxHeight` 2160.
- `transcode.Rung2160Allowed(caps, stream, profileMaxHeight, deviceHeight)` prüft die Bedingungen
  aus Punkt 2. Die Gerätehöhe ist die kurze Bildschirmseite in Gerätepixeln wie bei der
  Gerätegrenze (`deviceCap`); 0 (unbekannt) lässt 2160p nicht zu.
- Die Leiter in `transcode` beginnt mit `2160p` (25 Mbit/s). `QualityFor` gibt über 1080 Zeilen
  `2160p`, bei unbekannter Höhe weiter `1080p`.
- Level und Codec-String folgen der Ausgabehöhe (`h264Level`): bis 1080 Zeilen 4.1
  (`avc1.640029`), darüber 5.1 (`avc1.640033`).
- **Kosten:** `cost2160p = 5` in `transcode/plan.go` (Punkt 5), für jede Ausgabe über 1080 Zeilen.
  Der Wert ist der Startwert dieses ADR. Die Messung auf CT 190 steht noch aus; sie folgt vor dem
  Merge von Stufe 2 in `docs/perf/13-abspielseite.md`.

## Nachtrag: Breitbild und Level (Marcos Entscheid vom 4. November 2026)

- **Eine Quelle gilt für 2160p als 4K**, wenn sie mindestens 2160 Zeilen **oder** mindestens
  3840 Pixel Breite hat (`transcode.Is4KSource`). Das ersetzt in Punkt 2 «Die Quelle hat
  mindestens 2160 Zeilen». Ein Scope-Film in 3840×1600 bekommt 2160p angeboten, 2560×1440 und
  3836×1600 nicht.
- Die 2160p-Stufe skaliert nie hoch: 3840×1600 bleibt 3840×1600 (`heightLimit` ist eine
  Obergrenze, die Ausgabe ist höchstens so hoch wie die Quelle).
- **Level aus Bildgrösse und Bildrate.** Das H.264-Level folgt der Zahl der Makroblöcke
  (16×16 Pixel) je Bild und je Sekunde (MaxFS und MaxMBPS aus Tabelle A-1 der Norm), nicht nur
  der Höhe. Das ersetzt Punkt 6 und die Zeile zu `h264Level` in der Umsetzung:
  - bis 8192 Makroblöcke je Bild und 245 760 je Sekunde (1920×1080 mit 30 Bildern/s) Level 4.1,
    `avc1.640029`;
  - bis 36 864 und 983 040 (2592×1080 als 1080p-Ausgabe eines Scope-Films, 3840×1600, 3840×2160
    und 4096×2160 mit 30 Bildern/s, 1080p mit 60 Bildern/s) Level 5.1, `avc1.640033`;
  - sonst Level 5.2, `avc1.640034` (zum Beispiel 2160p mit 60 Bildern/s).
- Die Bildrate ist die mittlere Bildrate der Quelle (`avg_frame_rate` von ffprobe, neu
  `Stream.FrameRate`). Probes, die vor diesem Schritt gespeichert wurden, haben sie nicht; dann
  entscheidet die Bildgrösse allein, bis die Datei neu geprüft wird.

## Verworfene Alternativen

- **2160p auch auf dem Software-Backend.** libx264 schafft 4K nicht in Echtzeit auf den
  Zielrechnern. Das widerspricht AN-17.
- **Kosten 3 wie eine 4K-Quelle mit 1080p-Ausgang.** Dann würde das Budget zwei oder mehr
  2160p-Kodierungen auf einer iGPU zulassen, und das überlastet sie (Risiko in 11.14).
- **`auto` wählt 2160p bei 4K-Geräten.** Das verbraucht das halbe Budget, ohne dass jemand danach
  gefragt hat. Direct Play bleibt für 4K-taugliche Geräte der Normalfall.
