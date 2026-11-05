# Messung Konzept 14 «Schneller Wiedergabestart»

Konzept `docs/planning/14-wiedergabestart.md`, Abschnitt 8. Testserver CT 190 (`192.168.1.43`,
Intel UHD 630, jellyfin-ffmpeg 7.1, Medien über NFS), Image `wiedergabestart-68cefaf`
(`v1.12.0-34-g68cefaf9`) seit 5. November 2026, 13:28. Alle Werte stammen aus Server-Log und
Datenbank (nur lesend ausgelesen), Abnahme durch Marco am selben Tag.

## Teil 1: Abnahme auf CT 190 (5. November 2026)

### Umstellung

| Schritt | Wert |
|---|---|
| Migration 00033 | 1786 ms, DB-Sicherung `pre-migration-v1.12.0-34-g68cefaf9-20261105T122847.149Z.db` |
| Index fehlte vorher | Filme 440, Serien 287, test 53 (indexwürdige Videodateien) |
| `probe.keyframes` | pausiert seit 31. Oktober 2026, 664 wartende Altjobs |

### Start ohne Index: «Silo» S01E03 in Firefox (MKV, HEVC 1080p, EAC3 Atmos)

| Messpunkt | Vorher (1. November 2026, 1.6.0) | Nachher |
|---|---|---|
| `POST /items/{id}/play` | 20 016 ms | **16,6 ms** |
| bis zum ersten Segment | 20 016 ms + 350 ms | **rund 0,87 s** (`init.mp4` 648 ms, erstes Segment 810 ms) |
| Modus | Transcode nach Abbruch der Volllesung | sofort Transcode, `reason=keyframes_pending`, QSV 1080p |
| Index im Hintergrund | Job blieb pausiert | wartender Altjob angehoben (Priorität 9, `override_pause=1`), im Arbeiter `index-1` trotz Pause in **193 ms** aus den Cues |
| zweiter Start | wieder bis 20 s | **Remux** (`reason=audio_codec`, nur der Ton wird umgewandelt) |

### Altbestand und Nachhol-Lauf

| Lauf | Ergebnis |
|---|---|
| `probe.keyframes` fortgesetzt (662 Altjobs) | alle in rund 3 min fertig, 0 Fehlschläge, je 0,16 bis 0,42 s (Index aus dem Container vor der Volllesung) |
| davon verworfen | 2 Filme mit Endlücke 4:16 und 1:59 min (Regel ≤ 60 s), danach ganz gelesen (`scan`) |
| `keyframes.backfill` (Serien) | 257 Dateien in **88 s**, 257 aus den Cues gespeichert, 0 Volllesungen |
| neue Dateien während der Abnahme | 4 Radarr-Upgrades, `probe.file` je **107 bis 299 ms** inklusive Cues und Stichprobe |

Abdeckung danach (indexwürdige Videodateien):

| Bibliothek | ausstehend | Cues | ganz gelesen | nicht möglich |
|---|---|---|---|---|
| Filme | **0** | 441 | 1 356 | 0 |
| Serien | **0** | 287 | 4 | 0 |
| test | **0** | 53 | 250 | 0 |

### Einmaliger Termin

`scan.library`, Termin 5. November 2026 13:57 (Europe/Zurich), gespeichert als `12:57Z`. Ausgelöst
um 13:57:53 beim nächsten Takt (`one_off=true`, 3 Scans), `once_at` danach leer, `next_run_at`
unverändert (Takt 6 h). Zwei Speicherversuche davor antworteten 422 (vermutlich Termin weniger als
1 min in der Zukunft; das Feld steht nicht im Log).

### QSV-Bild auf dem Handy

«Project Hail Mary» (1920 × 1040, Quelle 6,1 Mbit/s), im Player 720p gewählt: `mode=transcode`,
`backend=qsv`, `reason=quality`. Über 34 Segmente im Mittel **2,84 Mbit/s** (Ziel 3, Deckel 4),
höchstes 4-s-Segment **5,39 Mbit/s** mit Ton. Das liegt über R, aber unter 1,5 × R, was der Puffer
von 2 × R über 4 s rechnerisch erlaubt (I-5: Ziel, keine Garantie). Marco: «Bild wirkt scharf».

## Teil 2: noch offen

- QSV bei 480p (R = 2 Mbit/s) und 360p (R = 1 Mbit/s, Ziel 999 kbit/s) mit `-loglevel verbose`
  (`RateControlMethod: VBR`), SSIM gegen Anhang A.3 (O-13).
- VAAPI (`h264_vaapi`, iHD) mit derselben Regel (O-2); AMD bleibt ungemessen.
- NVENC hält die Leiter nur im Mittel (O-19), nicht Gegenstand dieses Konzepts.
