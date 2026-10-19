# ADR 0004: hls.js als HLS-Engine des Web-Players

- Status: angenommen (19. Oktober 2026)
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitte 3.5, 5.4, 10.3 und M1b; ADR 0001 (Zeile «Player»);
  ADR 0003 (Segmentierung auf dem Server)
- Betroffene Spec-IDs: W-1, W-2, W-3, C-1, P-10

## Kontext

Ab M1b liefert der Server Titel, die der Browser nicht direkt abspielen kann, als fMP4-HLS aus: Remux
(Video kopiert, MKV → fMP4, HEVC mit `hvc1`) oder Transcode (H.264/AAC). Die Playlist ist von Anfang an
vollständig (VOD), damit der Player überallhin spulen kann; Segmente, die noch fehlen, erzeugt der Server
beim Abruf (Seeking-Modell 3.5). Alle Playlist- und Segment-URLs tragen das Stream-Token `st` (5.4).

Im Browser gibt es zwei Wege, HLS abzuspielen:

- **Nativ:** Das `<video>`-Element lädt die Master-Playlist selbst. Safari kann das seit jeher, das iPhone
  ausschliesslich so oder über `ManagedMediaSource`. Chromium meldet auf dem Entwicklungsrechner
  (Playwright-Chromium 156) inzwischen ebenfalls `canPlayType('application/vnd.apple.mpegurl') = "maybe"`.
  Firefox kann kein natives HLS.
- **Media Source Extensions (MSE):** Eine JavaScript-Bibliothek lädt Playlists und Segmente und hängt sie
  an einen `SourceBuffer`. Das geht in Chromium, Firefox und Safari auf dem Desktop.

Firefox braucht also zwingend eine MSE-Bibliothek. Die Startseite darf dadurch nicht wachsen (P-10: LCP
< 2.5 s, M1a 1.80 s).

## Entscheid

| Punkt | Entscheid |
|---|---|
| Bibliothek | **hls.js 1.7.3**, exakt gepinnt in `web/package.json` |
| Build | `hls.js/light`: ohne Untertitel-Rendering, EME/DRM und alternative Tonspuren, die M1b nicht braucht; die Typen kommen aus dem vollen Paket (`web/src/features/player/hls-light.d.ts`) |
| Laden | dynamischer Import im Player (`VideoPlayer.tsx`), nur wenn die Antwort von `startPlayback` `mode: hls` ist und der Browser über MSE abspielt; eigener Chunk `hls.light-*.js` mit 359 kB (114 kB gzip). Die Startseite lädt ihn nie: `index-*.js` bleibt bei 436 kB (140 kB gzip), wie in M1a |
| Lizenz | Apache-2.0, verträglich mit AGPL-3.0-only |
| Engine-Wahl | `web/src/features/player/capabilities.ts`: **hls.js, sobald die klassische `MediaSource` vorhanden ist** (Chromium, Firefox, Safari auf dem Desktop); **nativ**, wenn der Browser HLS selbst spielt und nur `ManagedMediaSource` kennt (iPhone-Safari); sonst hls.js über `ManagedMediaSource`; ohne beides zeigt der Player «Format nicht abspielbar» |
| Konfiguration | `startPosition` = Fortsetzungsposition (ein Transcode beginnt so gleich an der richtigen Stelle, statt zuerst den Anfang zu kodieren); `fragLoadPolicy` mit 20 s bis zum ersten Byte, weil ein Sprung weit nach vorne ffmpeg neu startet; ein fataler Fehler zeigt die bestehende Fehlermeldung des Players |
| Fähigkeiten | Der Client meldet mit `startPlayback` `capabilities` (`video`, `audio`, `hls: native/mse/none`) aus `canPlayType` und, auf dem MSE-Weg, `MediaSource.isTypeSupported`; HEVC wird als Main 10 geprüft. Der Server entscheidet daraus Direct Play, Remux oder Transcode (Plan 10.3; Prinzip 1: Entscheid im Server) |
| Sitzungsende | Beim Verlassen des Players `DELETE /playback/sessions/{id}` mit `keepalive` (auch bei `pagehide`), damit das Token sofort ungültig wird und der Server ffmpeg beendet |

Die Abweichung vom ursprünglichen Plan («natives HLS, wenn `canPlayType` es meldet») ist bewusst:
Chromium meldet natives HLS inzwischen selbst, und mit hls.js laufen Chromium, Firefox und Safari auf dem
Desktop über denselben, getesteten Weg mit derselben Startposition, demselben Spulen und derselben
Fehlerbehandlung. Native Wiedergabe bleibt für das iPhone, wo sie Akku, AirPlay und Vollbild am besten
unterstützt.

## Betrachtete Alternativen

- **Nur natives HLS:** scheidet aus, Firefox spielt kein HLS.
- **Shaka Player:** kann HLS und DASH, ist aber deutlich grösser und auf DASH/DRM ausgerichtet; Slouchcat
  liefert nur HLS ohne DRM.
- **video.js mit VHS:** bringt eine ganze Player-Oberfläche mit, die Slouchcat mit eigenen Komponenten aus
  `web/src/ui` bereits hat (C-2); mehr Code für dieselbe Aufgabe.
- **Eigene MSE-Anbindung:** fMP4 anhängen wäre machbar, aber Pufferverwaltung, Spulen, Fehlerbehandlung
  und Browser-Eigenheiten nachzubauen lohnt sich nicht, solange hls.js gepflegt wird.
- **Voller hls.js-Build:** 593 kB statt 371 kB (minifiziert); die zusätzlichen Teile (Untertitel, EME,
  alternative Tonspuren) braucht M1b nicht.

## Konsequenzen

- Untertitel (W-6, M4): Text-Untertitel kommen als WebVTT-Spuren; ob dafür der volle Build nötig ist oder
  `<track>`-Elemente reichen, entscheidet M4. Der Wechsel ist eine Zeile (`hls.js` statt `hls.js/light`).
- Safari auf dem Desktop meldet `hls: mse`. Der Server tonemappt HDR10 dort deshalb wie in Chromium, statt
  HDR durchzureichen; HDR-Durchleitung gibt es nur auf dem nativen Weg (iPhone). Das ist für M1b
  ausreichend (W-2 verlangt den SDR-Fall) und wird mit den Geräteprofilen in M4 verfeinert.
- Firefox 157 meldet auf dem Entwicklungsrechner HEVC Main 10 als abspielbar (Dekodierung über das
  System-ffmpeg) und erhält damit Remux statt Transcode; die Wiedergabe wurde geprüft (90-s-Testclip,
  Start 87 ms, Spulen < 40 ms). Der Playwright-Test für den Transcode-Fall meldet deshalb nur H.264, damit
  er den in Plan 10.3 vorgesehenen Firefox-Transcode misst.
- hls.js wird vom Service Worker vorab gespeichert (Precache des App-Shells), aber erst geladen, wenn eine
  HLS-Wiedergabe startet.
- Aktualisierungen von hls.js laufen wie alle Abhängigkeiten über eine exakte Version und den Playwright-Lauf
  (`web/e2e/hls.spec.ts`, Chromium und Firefox).
