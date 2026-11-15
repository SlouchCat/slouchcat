# Messwerte: Proben aller Grafikchips beim Start (Konzept 22, Stufe 2)

Konzept `docs/planning/22-ersteinrichtung-und-sprachen.md`, Abschnitt 3.3 Schritt 5 (F-10). Bis Vertrag
1.34 hörte `auto` beim ersten funktionierenden Backend auf. Seit 1.35.0 probiert der Server beim Start
**alle drei** Hardware-Backends einmal (NVENC, QSV, VA-API), damit Assistent und Verwaltung zeigen, welche
gehen. Gemessen wurde, was das an Startzeit kostet.

## Aufbau

- Entwicklungsrechner am 15. November 2026: i7-14700K, NVIDIA GeForce RTX 4070, `/dev/nvidia0` und ein
  Render-Knoten `/dev/dri/renderD128` der NVIDIA-Karte, kein Intel-Grafikchip. ffmpeg 6.1.1 aus Ubuntu
  (mit `h264_nvenc`, `h264_qsv` und `h264_vaapi`).
- Ein Wegwerf-Test im Paket `transcode` (nicht eingecheckt) rief je Lauf die drei Proben einzeln
  (`testEncodeArgs`, eine Ein-Bild-Kodierung mit `testEncodeTimeout`) und danach `Detect(auto)` als Ganzes
  auf; 5 Läufe.

## Ergebnis

| Probe | ms je Lauf (5 Läufe) | Ergebnis |
|---|---|---|
| NVENC | 319, 244, 264, 264, 252 | geht |
| QSV | 37, 38, 28, 31, 31 | scheitert (kein Intel-Gerät) |
| VA-API | 21, 22, 23, 29, 29 | scheitert (Treiber der NVIDIA-Karte ohne VA-API) |
| `Detect(auto)` gesamt, mit HDR-Tonemapper-Probe | 1363, 1350, 1350, 1347, 1404 | NVENC |

- **Zusätzliche Startzeit:** die Proben nach dem ersten funktionierenden Backend, hier QSV und VA-API:
  **51 bis 60 ms** (Mittel 58 ms) auf rund 1,35 s Erkennung. Ein Backend ohne Gerät scheitert sofort.
- Auf einem Rechner ohne funktionierendes Backend kostet die Änderung nichts: Dort liefen schon bisher alle
  drei Proben.
- Ungünstigster Fall: ein Gerät, das hängt. Jede Probe ist durch `testEncodeTimeout` (5 s) begrenzt,
  die Listen von ffmpeg durch `detectDeadline`; `TestDetectBoundsHangingTestEncode` hält das fest.
- Der Server schreibt die Dauer der Erkennung neu in die Zeile «transcode ready» (`detect_ms`).
- Mit `--hw software` oder `SLOUCHCAT_HW=software` probiert der Server keinen Grafikchip (0 ms).
