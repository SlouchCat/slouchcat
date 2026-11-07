# ADR 0025: Webradio: Katalog, Weiterleitung mit Zeitversatz

- Status: angenommen (7. November 2026); **hebt für Live-Webradio das Nicht-Ziel «Musik» auf**
  (Spec «Non-Goals», `CLAUDE.md` Regel 11) und **ergänzt ADR 0004** (hls.js auch für Radio) und
  **ADR 0017** (Jobart mit Zeitfenster und einmaligem Termin)
- Quelle: Konzept `docs/planning/16-webradio.md` (Abschnitte 5 bis 7, 12 und 14); **Frage 23** in
  `docs/planning/04-entscheidungen.md` mit Marcos Antworten F-1 bis F-19; Bauplan Stufe 1 mit den
  Entscheiden des Koordinators vom 7. November 2026 (hier unter «Entscheid» Punkte 1, 2 und 6)
- Betroffene Spec-IDs: K-5 (tägliche Hörzeit), W-8 (Stream-Grenzen), A-3 (Hintergrundjobs),
  A-9 (keine Geheimnisse in Protokollen)
- Harte Regeln: 2 (kein SQL auf Titeltabellen), 3 (`os/exec` nur in `transcode`), 6 (Streaming als
  `http.Handler`), 7 (`x-visibility`, Vertragstest), 9 (Texte aus den Locales), 10 (`rt` nie im Log),
  11 (Ausnahme «Live-Webradio»), 12 (Entscheid im Server)

## Kontext

Marco will Webradio hören (Frage 23): den ganzen Senderkatalog, Pause und Zurückspulen bis 30
Minuten, Wiedergabe über den Server. Dafür fehlt dreierlei: eine Senderquelle, die sich täglich
abgleichen lässt, ein Weg, einen Live-Strom sicher durch den Server zu leiten, und eine Hör-Sitzung
ohne Titel. `playback_sessions` verlangt `item_id NOT NULL`, und `hlsAuth` löst auf eine
`PlaybackGrant` mit `ItemID` und `MediaFileID` auf; beides taugt für einen Sender nicht.
Dazu kommen zwei Eigenheiten der Quellen. Shoutcast v1 antwortet mit der Statuszeile `ICY 200 OK`,
die `net/http` ablehnt. Und ein Sender, der den Slouchcat-Server sieht, darf nie ein Weg ins
Heimnetz sein.

## Entscheid

1. **Quelle und Katalog: Radio Browser, täglich, mit stabiler Kennung.**
   - Quelle ist [Radio Browser](https://www.radio-browser.info) (`all.api.radio-browser.info`, bei
     Ausfall bis zu drei Spiegel `*.api.radio-browser.info` über `/json/servers`, sonst SRV
     `_api._tcp`). Die Daten sind **gemeinfrei**: Die Startseite von radio-browser.info nennt
     «Data license: public domain, software license: GPL, server software: open source» (nachgelesen
     am 7. November 2026). Die Server-Software steht laut ihrem Repository unter AGPL-3.0; Slouchcat
     nutzt nur die API und übernimmt keinen Code.
   - Die Jobart `radio.catalog.sync` läuft alle 24 Stunden im Fenster 04:30 bis 06:30. Der Seed trägt
     das Fenster, sodass auch der erste Lauf eines frischen Servers im Fenster liegt. Nur ein leerer
     Katalog bekommt beim Start einen einmaligen Termin 10 Minuten später. Kein Lesezugriff der App
     löst einen Abgleich aus. Ein Eintrag mit unverändertem `lastchangetime` wird nicht neu
     geschrieben.
   - **Stabile Kennung:** `radio_stations.id` ist `AUTOINCREMENT`. Die Katalogeinträge stehen in
     `radio_station_aliases` (`stationuuid` → Sender), und **ein Alias wandert nie**. Ein neuer
     Eintrag tritt dem Sender seines Gruppenschlüssels bei (gefalteter Name ohne Bitrate- und
     Formatwörter, Land, Host der Webseite) oder gründet einen. **Stufe 1 führt keine bestehenden
     Sender automatisch zusammen** und trennt nie; weder Zusammenführen noch «Stream abtrennen»
     noch eine Weiterleitungstabelle gibt es. Das war im Konzept (5.3) anders gedacht und folgt aus
     «ein Alias wandert nie».
   - **Einordnung** über die eingebettete Datei `server/internal/radio/rules/default.json`
     (Schlagwort → Genre, Sendeart, Stimmung) und feste Ableitungen für Land, Region, Sprache und
     Qualität. Von Hand gesetzte Felder tragen ein Schloss (`radio_station_locks`), das der Abgleich
     nicht überschreibt. Die Regeln des Admins (`radio_tag_rules`) kommen in Stufe 2.
   - Ein Sender, dessen Einträge 7 Tage fehlen, wird samt Favoriten gelöscht (F-18).
   - **Keine Rückmeldung an Radio Browser:** weder Klicks noch Stimmen (F-14). Der Abruf läuft über
     `ratelimit` (1 je Sekunde) mit dem User-Agent `Slouchcat/<Version>`.
2. **Weiterleitung: Go liest und schneidet, ffmpeg packt nur.**
   - **Ein Relay je Sender**, gleichgültig wie viele Profile zuhören. Der Go-Leser holt ICY-Ströme
     (Icecast und Shoutcast v1 mit `ICY 200 OK`, `Icy-MetaData: 1`) und HLS-Quellen über Verbindungen,
     deren Dialer `netguard.RefuseLocal` prüft, ohne Ausnahme auch für Weiterleitungen (höchstens 5,
     jede erneut geprüft). Der ICY-Kopf wird vor `http.ReadResponse` auf 16 KiB und 100 Zeilen
     begrenzt. Titelangaben (ICY, ID3 aus HLS, Ogg-Kommentare) trennt Go vom Ton.
   - **ffmpeg liest nur `pipe:0` und schreibt nur `pipe:1`** (fMP4, `-protocol_whitelist pipe`, nur im
     Paket `transcode`, Regel 3). Es sieht nie eine Netzadresse und kann keine in seine
     Fehlermeldungen schreiben. AAC wird umverpackt (`-c:a copy`) mit **`+delay_moov`**: mit
     `+empty_moov` allein steht `esds` ohne `AudioSpecificConfig`, und MSE lehnt das ab. Alles
     andere (MP3, Ogg, Opus, FLAC) wird zu AAC mit `clamp(Quelle, 96, 192)` kbit/s (128, wenn die
     Bitrate unbekannt ist).
   - **Go rahmt den Ton:** nur ganze MP3- und ADTS-Rahmen gehen an ffmpeg; nach einem Verbindungsabbruch
     wirft der Rahmer den halben Rahmen weg, entfernt ID3 und setzt erst nach zwei aufeinanderfolgenden
     gültigen Köpfen fort. Im Kopiermodus beginnt **jeder Bruch der Quelle eine neue Generation** (neues
     ffmpeg, neues Init, `EXT-X-DISCONTINUITY`, neuer `EXT-X-PROGRAM-DATE-TIME`), ebenso bei Ogg, FLAC
     und MPEG-TS, die Go nicht rahmt, und bei Wechsel von Codec, Bitrate oder Stream. Wartezeiten beim
     Wiederaufbau: 1, 2, 5, 10, 20 Sekunden, danach endet das Relay; drei Abbrüche in Folge markieren den
     Sender als gestört.
   - **Go schneidet die Stücke, schreibt sie und baut die Wiedergabeliste.** Es zerlegt ffmpegs Ausgabe
     auf Box-Ebene, legt Init und Stücke atomar in die `tmpfs` `/run/slouchcat/radio` und liefert
     Delta-Listen (`CAN-SKIP-UNTIL=36`, `_HLS_skip`, kein `CAN-BLOCK-RELOAD`) mit gzip. Das
     weicht vom Konzept (7.1: ffmpeg schreibt die HLS-Liste selbst) ab; der Grund ist Regel 3 mit
     «ffmpeg sieht nie einen Pfad ausser den Pipes» und die Kontrolle über Generationen und Budget.
   - **Stücke:** 2 Sekunden in den ersten 30 Sekunden nach dem Start des Relays, danach 4 Sekunden
     (zwei Fragmente). Das beschleunigt den ersten Ton (Ziel 6 s bei neuem, 2 s bei laufendem Relay);
     `listen` antwortet, sobald zwei Stücke bereitliegen.
   - **Ohne `tmpfs`** (ältere Compose-Datei) spielt Radio **nur live**, mit den letzten 8 Stücken im
     Arbeitsspeicher; **nie auf der Platte** (F-4). Die Verwaltung warnt (`bufferMode`).
3. **Grenzen.**
   - **Byte-Budget 512 MB** für alle Puffer, in Byte durchgesetzt: ein neues Relay reserviert
     Bitrate × Zeitversatz × 1,03 und startet nur, wenn die Summe ins Budget passt. **Höchstens
     10 Relays**, **2 Minuten Nachlauf** nach der letzten Sitzung, Zeitversatz 10, 30 oder 60
     Minuten. Alle vier sind einstellbar (`radio_config`) und gelten für neue Relays; laufende
     stoppt eine Senkung nie. Sonst `radio_capacity`.
   - **Pausierte Hörer zählen.** Eine offene Hör-Sitzung hält ihr Relay am Leben, ob sie spielt oder
     pausiert, denn der Puffer läuft weiter.
   - **Radio zählt nicht zu W-8** (F-19): keine Stream- und keine Bitratengrenze; geschützt wird der
     Server durch Budget und Höchstzahl.
   - Die `tmpfs` hat Budget + 10 % (564 MB), damit nie der Kernel zuerst abschneidet. CT 190 (Prüfung
     vom 7. November 2026, F-13): 4096 MB Arbeitsspeicher, 3919 MB verfügbar, Container rund 87 MB.
4. **Hör-Sitzungen und Hörzeit.**
   - Eigene Tabelle `radio_listen_sessions` und eigene Middleware `radioAuth`. Das Token ist **an den
     Sender gebunden** (HMAC über Sitzung und Sender) und steht im Abfrageparameter **`rt`**, den das
     Protokoll maskiert (`logging.sensitiveQuery`, Regel 10). Ohne gültige Sitzung gibt es weder Liste
     noch Stück.
   - **Gleitender Ablauf:** jeder Live-Abruf verlängert die Sitzung im Speicher um die Pufferlänge;
     geschrieben wird höchstens einmal je Minute (`touchInterval`). Der Heartbeat (alle 30 s) meldet
     Spielen oder Pause und verlängert ebenfalls; Zuhören hängt nicht an ihm, denn ein gesperrtes
     iPhone holt Stücke weiter, während seine Zeitgeber schlafen.
   - **Hörzeit:** gezählt werden einzeln ausgelieferte, verschiedene Stücke je Viertelstunde in
     `radio_listen_time`; Stücke bei gemeldeter Pause zählen nicht. `playback.Service.WatchTime` und
     `WatchReport` addieren diese Zeit, K-5 schliesst sie ein (F-16). Am Limit enden die Sitzungen des
     Profils, `listen` antwortet `watch_limit_reached`.
   - **Entzug** bei Abmelden (`logout`), Profilwechsel (`revoked`), Entzug des Kontos, Stopp durch die
     Verwaltung (`admin`), Tageslimit (`limit`) und Ende des Relays (`station`); Ablauf ist `expired`.
     Der Server beendet beim Start offen gebliebene Sitzungen (`restart`).
5. **Player auf App-Ebene in `authedRoute`, nicht in `activeRoute`.** Der React-Kontext
   `RadioProvider` hält `<audio>` und hls.js. `activeRoute` taugt nicht, denn `/profiles` liegt als
   Kind von `authedRoute` ausserhalb davon und risse den Player beim Profilwechsel mit. Gestoppt
   wird nach den Regeln in Konzept 10.3: Film, Folge, Hörbuch oder Podcast stoppt Radio; ein Trailer
   pausiert es; Profilwechsel und Abmelden stoppen es; Navigation, auch in die Verwaltung, nicht; ein
   zweiter Tab pausiert den ersten (`BroadcastChannel`). Kein Bedienfeld auf dem Sperrbildschirm (F-11).
6. **Standardland je Profil** (F-12): aus der **Zeitzone des Servers** (`TZ`, im Compose
   `Europe/Zurich` → CH, über die eingebettete Tabelle `rules/zones.json`), sonst
   `metadata.rating_country`, sonst `DE`. Profile haben keine Region. Das Profil ändert es auf
   `/radio`; gespeichert in `radio_profile_state`.
7. **Senderbild-Kette:** eigenes Bild des Admins, dann `favicon` des Katalogs, dann Symbole der
   Webseite (`apple-touch-icon`, Manifest, `og:image`), dann Initialen im Client aus Tokens, sodass
   jeder Sender ein Bild hat. Mindestens 96 × 96 Pixel, höchstens 10 MiB, kein SVG. Logos holt der
   Job `radio.logo.fetch` (Pool `general`, tiefste Priorität, gedrosselt) erst, wenn eine Liste oder
   die Favoriten einen Sender zeigen, und beim Abgleich für Favoriten und Empfohlene; ein
   Fehlschlag wird nach 30 Tagen wiederholt. Die App lädt nie ein Bild direkt vom Sender.
8. **Was Stufe 2 bringt** (Vertrag 1.18.0, brechend für `getHome`/`setHomeRows`, Migration 00039):
   Zeitversatz-Regler und Zustand «−12:30», Zeitleiste der Titel nach Ton-Position (Stufe 1 stempelt
   mit dem `EXT-X-PROGRAM-DATE-TIME` des Stücks, Fehler höchstens ein Stück), Cover-Kette
   MusicBrainz → iTunes, «Zuvor gespielt», Startseiten-Reihe «Meine Sender», Editor für Tag-Regeln
   (`radio_tag_rules`), eigene Sender und Radio auf der Seite «Streams».

## Folgen

- Migration 00038 (elf Tabellen, nur neue; im Konzept als 00036 vorgesehen), Vertrag **1.17.0**
  additiv: 25 Pfade unter `/radio` und `/admin/radio`, alle mit `x-visibility` (`scoped` oder
  `admin`); die HLS- und Bildrouten sind `http.Handler` (Regel 6). Ein ausgeblendeter Sender, ein
  Sender eines ausgeschlossenen Landes und ein unbekannter antworten 404, nie 403.
- Fehlercodes `radio_capacity`, `radio_unreachable`, `radio_not_audio`, `radio_catalog_loading`.
- `deploy/compose.yaml` bekommt die `tmpfs` (`size=564m,mode=0700,uid=${PUID},gid=${PGID}`); die
  Release-Notes nennen es. Ob Docker auf CT 190 `uid=` annimmt, zeigt die Abnahme; Rückfall ist
  `mode=1777`.
- Das Paket `radio` fragt keine Titeltabellen ab (Regel 2). `os/exec` bleibt in `transcode`. Eigener
  Testbuild mit Tag `radiotest` und Binärdatei `bin/slouchcat-e2e`: Dialer, der localhost erlaubt,
  und umstellbare Katalog-Adresse; die Produktion trägt den Tag nie.
- `CLAUDE.md` Regel 11 und die Spec «Non-Goals» bekommen den Zusatz «ausser Live-Webradio (Frage 23)».
- Der Abgleich nimmt rund 40 MB Datenbank und einen Abruf von rund 30 bis 40 MB am Tag in Kauf;
  Messungen stehen in `docs/perf/radio.md`.
- Offene Prüfungen der Abnahme: HE-AAC-Kopie in Firefox und Safari (sonst wie MP3 umrechnen), Delta-Listen
  auf dem iPhone, Ton bei gesperrtem Bildschirm, ffmpeg-Verhalten von `+delay_moov` im Container
  (`TestRadioPackagerInitHasAudioSpecificConfig`).
- Der Standort des Servers gilt für alle Profile. Ob ein Sender das zulässt, verantwortet der Betreiber;
  die Verwaltungsseite sagt das.
- `lastcheckok` von Radio Browser misst die Erreichbarkeit von einem Server in Deutschland aus (FAQ des
  Betreibers, «Why are some stations marked as broken»). Sender, die nur in der Schweiz zu hören sind,
  können darum dort als gestört gelten, obwohl Slouchcat sie aus der Schweiz erreicht. Die Einstellung
  «gestörte Sender ausblenden» (Vorgabe an) blendet sie in der App aus; bei der Abnahme an Schweizer
  Sendern prüfen, ob die Vorgabe so bleiben soll.

## Verworfen

- **ffmpeg liest die Sender-Adresse selbst:** ffmpeg könnte ins Heimnetz greifen und Adressen mit
  Schlüssel in Fehlermeldungen schreiben; ausserdem versteht es Shoutcast v1 und die Titelangaben
  nicht so, wie wir sie brauchen.
- **ffmpeg schreibt die HLS-Liste und die Stücke selbst** (Segment-Muxer wie ADR 0003): braucht
  Polling auf fertige Dateien, kennt weder Generationen noch Byte-Budget und schreibt Pfade.
- **Puffer auf der Platte:** widerspricht F-4. Ohne `tmpfs` gibt es nur Live.
- **Bestehende Sender zusammenführen oder trennen, mit Weiterleitung der alten Kennung:** bricht «ein
  Alias wandert nie». Bleibt Stufe 2, falls Marco es noch will.
- **Hörer nur über den Heartbeat messen:** iPhone-Zeitgeber schlafen bei gesperrtem Bildschirm, die
  Stück-Abrufe laufen weiter.
- **`hlsAuth` und `playback_sessions` mit einem Platzhaltertitel:** zwingt Radio in die Titelregeln
  (Regel 2) und verfälscht Wiedergabe-Berichte.
- **Player in `activeRoute`:** stirbt auf `/profiles`.
- **Standardland aus der Sprache des Profils:** Profile haben keine Region.
- **Klicks und Stimmen an Radio Browser melden:** verrät, was im Haushalt gehört wird (F-14).
