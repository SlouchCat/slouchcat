# Testdaten Webradio

Ton der Test-Station `server/internal/radio/radiotest` (Konzept 16, Bauplan S4). Alle Dateien sind
Sinustöne aus der ffmpeg-Quelle `sine`, keine Musik und kein geschütztes Material. Sie sind klein und
eingecheckt; `generate.sh` erzeugt sie neu (nur nötig, wenn sie sich ändern sollen):

```bash
testdata/radio/generate.sh
FFMPEG=/usr/lib/jellyfin-ffmpeg/ffmpeg testdata/radio/generate.sh
```

| Datei | Inhalt | Zweck |
|---|---|---|
| `tone-mp3-128.mp3` | MP3 128 kbit/s, 44,1 kHz Stereo, 16 s, mit ID3-Kopf | Icecast/Shoutcast, Transcode-Weg, Framer |
| `tone-aac-64.aac` | AAC-LC in ADTS, 64 kbit/s, 16 s | Copy-Weg, Framer, HLS-Packed-Audio |
| `tone-ogg.ogg` | Ogg Vorbis, 16 s, Kommentare `TITLE`/`ARTIST` | Ogg-Titel |
| `tone-opus.ogg` | Ogg Opus 48 kbit/s, 8 s, `OpusTags` | Opus-Titel |
| `tone-ogg-chained.ogg` | zwei verkettete Ogg-Vorbis-Ströme (je 6 s, eigene Titel) | Titelwechsel wie bei Icecast-Ogg |
| `hls/seg0.ts` … `seg5.ts` | sechs MPEG-TS-Segmente zu 4 s, AAC 64 kbit/s | HLS-Quelle |

Voraussetzung: ffmpeg mit `libmp3lame`, `aac`, `libvorbis` und `libopus`.
