# Messungen Konzept 13 «Abspielseite»

## 1. 2160p auf CT 190 (Stufe 2, Schritt 12, 4. November 2026)

**Auftrag:** 11.5 und ADR 0021 verlangen vor dem Merge von Stufe 2 eine Messung von Start und
gleichzeitigen Strömen in 2160p auf CT 190, um die Kosten 5 von 8 zu begründen. Marcos Wort «Ja,
so messen» (4. November 2026).

### 1.1 Aufbau

- **CT 190:** Intel Core i5-10500T (UHD 630, PCI `8086:9BC8`), 6 Kerne, 4 GB RAM, Backend `qsv`,
  Tonemapping auf der CPU. Der laufende Server (Release 1.10.0) und seine Datenbank blieben
  unberührt.
- **Image:** `ghcr.io/slouchcat/slouchcat:stufe2-8428d7f`, auf dem CT gebaut aus `8428d7f1`
  (Zweig `claude/abspielseite-stufe2`), nur für Einmal-Container von `hlsbench`.
- **Quellen:** Die Bibliothek auf CT 190 hat keine 4K-Datei (2392 Dateien, höchstens 1924×1104).
  Darum synthetische Clips (`testsrc2` mit zeitlichem Rauschen) in `/root/uhd-test`, erzeugt mit
  [`13-abspielseite-uhd-clips.sh`](13-abspielseite-uhd-clips.sh):
  - `sdr-2160p24.mkv`: H.264, 3840×2160, 24 fps, 90 s;
  - `sdr-scope24.mkv`: H.264, 3840×1600 (Breitbild), 24 fps, 90 s;
  - `sdr-2160p60.mkv`: H.264, 3840×2160, 60 fps, 60 s;
  - `hdr10-2160p24.mkv`: HEVC Main 10, BT.2020/PQ, 3840×2160, 24 fps, 60 s.
- **Messung:** [`13-abspielseite-uhd-measure.sh`](13-abspielseite-uhd-measure.sh), je Fall
  `slouchcat hlsbench --hw qsv --json` in einem Einmal-Container mit `/dev/dri` und der GID aus
  `RENDER_GID` der `.env`.
- **Grenze:** Synthetische Bilder kodieren leichter als ein echter Film, die Zahlen sind eher zu
  günstig. Die Quellen sind H.264, echte 4K-Dateien meist HEVC. Spulen in 2160p ist nicht gemessen
  (das geht nur in der App mit einer echten 4K-Datei).

### 1.2 Ergebnisse

| Fall | Ausgabe | Kosten | Start (`initMs`) | Film / Wanduhr | Zu spät | CPU |
|---|---|---|---|---|---|---|
| 1 Strom 2160p24 | 3840×2160 | 5 | 1258 ms | 45 s / 11,9 s (3,8×) | nein | 3 s |
| 1 Strom Breitbild 24 | 3840×1600 | 5 | 905 ms | 45 s / 8,8 s (5,1×) | nein | 3 s |
| 1 Strom 2160p60 | 3840×2160 | 5 | 2767 ms | 30 s / 20,9 s (1,4×) | nein | 3 s |
| 2 Ströme 2160p24 | 3840×2160 | 5 je | 2263 / 2263 ms | 45 s / 23,6 s (je 1,9×) | nein | 4 + 3 s |
| 2160p24 neben 1080p | 3840×2160 und 1920×1080 | 5 und 3 | 1811 / 1801 ms | 45 s / 17,9 s und 17,6 s | nein | 3 + 5 s |
| Budget 8, 2 Ströme 2160p | 3840×2160 | 5 je | 1259 / **9411 ms** | 30 s / 16,2 s | nein | 2 + 2 s |
| Gegenprobe 4K-HDR10 | **1920×1080** | 4 | 2553 ms | 30 s / 18,0 s (1,7×) | nein | 63 s |

- **Grenze je Backend hält:** Auf QSV mit Tonemapping auf der CPU bleibt 4K-HDR bei 1080p (Kosten
  4), 4K-SDR geht auf 2160p (Kosten 5).
- **Breitbild:** Eine 3840×1600-Quelle wird als 3840×1600 ausgegeben, nicht hochskaliert (Marcos
  Entscheid, 11.15).
- **Zulassung:** Bei Budget 8 wartet der zweite 2160p-Strom (5 + 5 > 8), bis der erste das Budget
  freigibt, und startet dann nach 9,4 s. `hlsbench` misst nur die Zulassung. Im Server senkt
  `lowered` einen zweiten 2160p-Strom bei vollem Budget auf 1080p (5 + 3 = 8, Schritt 11); erst
  wenn auch das nicht passt, antwortet er 503 `transcode_busy` mit `Retry-After`.
- **Lauf ohne GPU:** Ein erster Lauf mit der Host-Gruppe `render` (992) statt der GID des Geräts
  (993) fiel still auf Software zurück (`No VA display found`) und ist verworfen. Der Server selbst
  ist nicht betroffen: compose setzt `RENDER_GID` aus der `.env`.

### 1.3 Schluss für die Kosten

- Ein 2160p24-Strom läuft mit 3,8-facher Echtzeit, zwei gleichzeitig noch mit je 1,9-facher. Die
  GPU trüge also knapp drei 2160p24-Ströme.
- 2160p60 läuft allein nur mit 1,4-facher Echtzeit; zwei davon wären unter Echtzeit.
- Echte Filme kodieren schwerer als das Testbild.
- **Die Kosten 5 von 8 bleiben:** ein 2160p-Strom plus höchstens ein 1080p-Transcode aus SDR
  (3) gleichzeitig. Das lässt Reserve für 60 fps und echte Inhalte. ADR 0021 bleibt ohne
  Korrektur.
