# K27 Stufe 0: Messung für die Android-Handy-App

Stand 21. November 2026. Grundlage: Konzept 27 (`docs/planning/27-android-handy-app.md`),
Abschnitte 2.4 bis 2.7, 5, 7.2 bis 7.4, 8.2, 9, 11.5 und 13. Gemessen mit einer Wegwerf-App
(`com.slouchcat.spike`, nicht im Repo) gegen einen eigenen Testserver mit den Testmedien
(`task testdata:media`, eingerichtet mit `server/cmd/apicheck`, 18 von 18 Zuschauerfunktionen ok).
Kein Code im Produkt geändert.

**Umgebung:** Linux x86_64, 28 Kerne, 31 GB RAM, JDK 21.0.13, Gradle 9.9.2, AGP 9.5.2,
Kotlin 2.4.21, Go 1.27.2. Emulatoren: API 36 (Pixel 9, Google APIs, x86_64; `sc_phone` und eine
eigene Kopie mit 40-GB-Datenbereich) und API 28 (`sc_phone28`). Testserver auf
`127.0.0.1:8597`, aus dem Emulator `http://10.0.2.2:8597`; Anmeldung mit Cookie wie `apicheck`.

## Übersicht der Entscheide

| Nr. | Punkt | Entscheid | Für Stufe |
|---|---|---|---|
| 1 | Verschlüsselung | HPKE und Ed25519 mit Tink (`tink-android` 1.24.0); Web-Push mit eigenem JCA-Code; PBKDF2 der Plattform; Testvektoren im Format `slouchcat-crypto-vectors-v1` | 1, 2, 3, 8, 9 |
| 2 | Push-Satzgrösse | `RecordSize` 2700 für Adressen der Push-Weiterleitung, Nutzlast höchstens 2597 Byte, Kürzung nach Bytes, `maxBody` 350 Zeichen | 1, 2 |
| 3 | Kotlin-Client | openapi-generator 7.27.0, `kotlin`/`jvm-okhttp4`/`kotlinx_serialization`, mit Vorlauf `TrimOpenApi` als Gradle-Task | 3 |
| 4 | Wiedergabe | Media3 1.11.1 spielt Direkt (MP4, MKV), HLS-Remux und HLS-Transcode, spult, zeigt WebVTT; `hevc` nur melden, wenn der Decoder das Profil kann | 5 |
| 5 | Download abspielen | lokale HLS-Liste aus dem Fragmentindex des Manifests (wie `loader.ts`); die Datei allein ist nicht spulbar | 6 |
| 6 | Abruf im Hintergrund | Übertragungsauftrag (UIDT) ab API 34, Vordergrunddienst `dataSync` für API 28 bis 33; beide mit Range und `If-Range` | 6 |
| 7 | Abruf ohne offene App | WorkManager stückweise (setzt verlässlich fort, Zeitpunkt bestimmt das System), dazu UIDT beim Öffnen | 6 |
| 8 | Texte und Farben | Generatoren tragen; `{{x, number}}` als Argument; `_zero` in de/en nicht übernehmen; alle Zuschauer-Namensräume übertragbar | 3 |
| 9 | Wächter | Android Lint meldet nichts; eigene Lint-Regel (Kleinstbeispiel läuft) | 3 |
| 10 | Bau und Tests | Robolectric 4.18.1 auf SDK 36 unter JDK 21 läuft; CI-Job geschätzt 8 bis 12 min | 3 |
| 11 | Versionen | Tabelle unten; **`compileSdk` 37.2 statt 36** (Abweichung) | 3 |

## 1. Verschlüsselung zwischen Go und Android

**Gemessen.** Ein Go-Programm (`crypto/hpke` der Standardbibliothek) erzeugt Vektoren, ein
Instrumentierungstest liest sie auf dem Gerät und versiegelt zwei eigene Fälle, die Go wieder öffnet.

| Prüfung | API 36 | API 28 |
|---|---|---|
| HPKE Go → Kotlin, Tink (Paket, Token, leerer Klartext) | 3 von 3, 3–4 ms je Öffnen | 3 von 3, 1–2 ms |
| HPKE Go → Kotlin, BouncyCastle 1.87 (Gegenprobe) | 3 von 3 | 3 von 3 |
| HPKE Kotlin (Tink) → Go (`k27crypto verify`) | «2 of 2 cases ok» | «2 of 2 cases ok» |
| Ed25519 «Server weist sich aus» (gültig, ein Bit gekippt) | richtig erkannt | richtig erkannt |
| Web-Push RFC 8291 `aes128gcm`, Körper von webpush-go, Satz 4096 und 2700, eigener JCA-Code | 2 von 2, ≤ 1 ms | 2 von 2, ≤ 1 ms |
| dasselbe mit Tink `apps-webpush` 1.15.0 | 2 von 2 | 2 von 2 |
| PBKDF2-SHA256, 600'000 Iterationen, 32 Byte (`pin.ts`) | gleiche Bytes, 1,5 s | gleiche Bytes, 1,1 s |

Befehle: `k27crypto gen -out vectors`, dann `adb shell am instrument -w -e class
com.slouchcat.spike.CryptoVectorsTest …` (7 von 7 Tests grün auf beiden Geräten), dann
`k27crypto verify -in kotlin-hpke-api{36,28}.json`.

**Entscheid.**

- **HPKE: Tink `com.google.crypto.tink:tink-android:1.24.0`.** `HpkeParameters` mit
  `Variant.NO_PREFIX`, `DHKEM_X25519_HKDF_SHA256`, `HKDF_SHA256`, `AES_256_GCM`.
  `HybridEncrypt.encrypt(klartext, info)` liefert genau `enc ‖ Geheimtext` wie Go `hpke.Seal`,
  `aad` ist leer, `info` ist der Kontext. Läuft ab API 28 ohne XDH der Plattform. Die HPKE der
  Plattform (ab API 37) bleibt ungenutzt, solange `minSdk` 28 ist. BouncyCastle kann es auch, wird
  aber nicht gebraucht.
- **Ed25519: Tink `subtle.Ed25519Verify`.**
- **Web-Push entschlüsseln: eigener Code über JCA** (ECDH P-256, HMAC-SHA256, AES-GCM, rund
  50 Zeilen). Er liest die Satzgrösse aus dem Kopf und braucht keine zweite Bibliothek. Tink
  `apps-webpush` hängt an `tink` (Java) und muss neben `tink-android` ausgeschlossen werden; es
  verlangt die Satzgrösse im Voraus.
- **PBKDF2: `SecretKeyFactory("PBKDF2WithHmacSHA256")`** der Plattform, nie auf dem Hauptfaden
  (über eine Sekunde im Emulator).
- **Testvektoren:** Format `slouchcat-crypto-vectors-v1`, eine Datei je Art (`hpke.json`,
  `ed25519.json`, `webpush.json`, `pbkdf2.json`): `{"format","kind","generator","cases":[{"name",
  "input":{…},"expected":{…}}]}`. Jeder Bytewert ist base64url ohne Füllung; Klartext sind nur
  `name`, `kind`, `format`, `generator` und Zahlen. HPKE-Fall: `kem` 32, `kdf` 1, `aead` 2,
  `mode` 0, `recipientPrivateKey`, `recipientPublicKey`, `info`, `aad`, `sealed` (= `enc ‖ ct`)
  → `plaintext`.

**Für die Stufen.** Stufe 1 legt die Vektoren einmal erzeugt unter `testdata/crypto/` ab (fest,
weil HPKE zufällig verschlüsselt) und prüft sie in einem Go-Test; Stufe 2 nutzt sie für die
Dienste; Stufe 3 liest dieselben Dateien in einem Kotlin-Test.

## 2. Push-Satzgrösse

**Gemessen** mit der Bibliothek des Servers (webpush-go v1.4.0) in einem Go-Probeprogramm
(`pushsize`):

| `RecordSize` | Körper | base64 (Standard) | `wp` + Wert | Reserve zu 4096 | Nutzlast höchstens |
|---:|---:|---:|---:|---:|---:|
| 4096 (heute) | 4096 | 5464 | 5466 | −1370 | 3993 |
| 3072 | 3072 | 4096 | 4098 | −2 | 2969 |
| 3000 | 3000 | 4000 | 4002 | 94 | 2897 |
| **2700** | **2700** | **3600** | **3602** | **494** | **2597** |
| 2048 | 2048 | 2732 | 2734 | 1362 | 1945 |

Der Körper ist bei webpush-go immer genau `RecordSize` Byte (Kopf 86 Byte, aufgefüllter Klartext,
16 Byte Tag); Nutzlast höchstens `RecordSize − 103`. `json.Marshal` schreibt die leere Hülle
`{title, body, url}` mit 31 Byte; ein Zeichen kostet 1 bis 4 Byte, `<`, `>`, `&`, U+2028/2029 und
Steuerzeichen aber 6 Byte (`\uXXXX`).

**Entscheid.**

- Für Adressen der Push-Weiterleitung setzt der Server `RecordSize = 2700`; sonst bleibt 4096.
  Das Datenfeld `wp` misst damit 3602 Byte, Reserve 494 Byte (12 %).
- Nutzlast für diese Adressen höchstens 2597 Byte. Der Server kürzt nach Bytes: `body` (mit «…»
  wie `truncate`), bis das JSON passt, danach `title`; eine `url` über 256 Byte wird zum Pfad.
- Feste Obergrenze daneben `maxBody` = **350 Zeichen** (heute 4000). Rechnung bei 4 Byte je
  Zeichen: 2597 − 31 − 800 (`maxSubject` 200 × 4) − 256 (`url`) = 1510 Byte = 377 Zeichen, mit
  Reserve 350. Die Byte-Prüfung fängt die 6-Byte-Zeichen ab.
- Die Weiterleitung kodiert base64 mit Füllung und lehnt Körper über 2700 Byte mit 413 ab.

**Aus Googles Dokumentation (offen, braucht das Firebase-Projekt).**

- Grenze: «Maximum payload for both message types is 4096 bytes». Ob Schlüssel mitzählen, sagt
  die Seite nicht; die Reserve deckt beides. Reservierte Datenschlüssel: `from`, `message_type`,
  alles mit `google.` oder `gcm.`; `wp` ist frei.
- Benachrichtigungs-Nachrichten zeigt die Bibliothek im Hintergrund selbst an, Daten-Nachrichten
  verarbeitet immer die App. Die Weiterleitung sendet darum nur Daten-Nachrichten (7.4 bestätigt).
- Dringlichkeit: `normal` wird im Doze-Modus «delayed … until the device exits doze»; `high` weckt
  das Gerät, wird aber auf `normal` herabgestuft, wenn die App daraus keine sichtbare Meldung macht
  (etwa bei abgeschalteten Mitteilungen). Vorschlag für Stufe 2: `Urgency: high` des Servers auf
  `high` übersetzen, sonst `normal`; die App zeigt jede entschlüsselte Meldung an.
- Nicht messbar ohne Projekt: tatsächliche Zustellung, Verzögerung im Stromsparmodus, Zählweise der
  4096 Byte.

## 3. Generator für den Kotlin-Client

**Vorlauf.** Gradle-Task `TrimOpenApi` (in `buildSrc`, snakeyaml-engine 3.2): behält Operationen
mit `x-visibility` `public`, `account`, `scoped` (später nach `x-clients`), folgt allen `$ref`
und wirft den Rest weg. Ergebnis am Vertrag 1.41.0: **124 Operationen behalten, 183 verworfen,
274 Komponenten erreicht.** Er schreibt ausserdem zwei Formen um, an denen der Generator scheitert:
`oneOf: [X, {type: "null"}]` (6 Stellen) wird zu X mit `nullable: true`, und
`additionalProperties: true` neben `properties` (3 Webhook-Schemas) fällt weg.

**Weg A: openapi-generator 7.27.0** (Gradle-Plugin `org.openapi.generator`), Generator
`kotlin`, Bibliothek `jvm-okhttp4`, `serializationLibrary=kotlinx_serialization`,
`enumUnknownDefaultCase=true`, `enumPropertyNaming=UPPERCASE` (sonst kollidiert der Wert `name`
mit `Enum.name`), `dateLibrary=java8`. Läuft im Bauordner (`build/generated/openapi`) vor
`compileKotlin`, 257 Dateien, kompiliert ohne Handarbeit, auch mit Konfigurations-Cache.

- Unbekannte Felder: ignoriert (`ignoreUnknownKeys = true` im erzeugten `Serializer`).
- Unbekannter Aufzählungswert: wird `UNKNOWN_DEFAULT_OPEN_API`, auch bei `$ref`-Aufzählungen in
  Objekten (geprüft mit `Health.status` und `ItemRef.kind`); kein Absturz.
- `nullable`/`oneOf`-null: `PlaybackRequest.subtitleTrack` ist `Long?`; mit
  `explicitNulls = false` (eine Zeile Konfiguration) gehen fehlende Werte nicht als `null` hinaus.
- OkHttp 5.5.1 als Transport: läuft (der Generator schreibt für OkHttp 4, die API ist gleich).
- Live gegen den Testserver (JVM-Test, 168 ms für alles): `getHealth` → OK, `login` → Sitzung ohne
  Profil, `listMyProfiles` → 1 Profil mit PIN, `selectProfile` mit PIN, `getHome` → 4 Reihen,
  `startPlayback` → `DIRECT`. Dieselben Aufrufe laufen im Emulator in den Tests der Punkte 4 und 5.
- Kleinigkeiten: Rückgabetypen sind `T?`; 409 kommt als `ClientException(statusCode = 409)`.

**Weg B: Fabrikt 29.1.0** (`io.fabrikt:fabrikt`), `KOTLINX_SERIALIZATION`,
`FAULT_TOLERANT_ENUMS`: die Modelle sind gut (378 Dateien, `UNRECOGNIZED` als Rückfall), aber der
OkHttp-Client ist fest an Jackson (`ObjectMapper`) gebunden. Mit kotlinx-Modellen braucht es eine
eigene Aufrufschicht; einen kotlinx-Client mit OkHttp erzeugt Fabrikt nicht.

**Entscheid: Weg A** mit dem Vorlauf. Konfiguration und Task liegen bei den gesicherten Dateien
(`generator/`). Stufe 3 übernimmt sie als Modul `core/api`; Stufe 1 ergänzt `x-clients`, danach
filtert der Vorlauf danach und die Webhook-Schemas fallen von selbst weg.

## 4. Wiedergabe mit Media3/ExoPlayer

**Gemessen** mit `PlaybackTest` (ExoPlayer 1.11.1, OkHttp-Datenquelle mit Sitzungs-Cookie,
Bild auf eine unsichtbare Fläche), `PlaybackCapabilities` nach K27 8.2 (`hls: mse`,
`containers: mp4, mkv, webm, ts, mov`, `type: phone`). Zeiten: Start bis erstes Bild, Spulen bis
die Wiedergabe hinter dem Ziel weiterläuft.

| Fall | Modus des Servers | API 36: erstes Bild / Spulen | API 28: erstes Bild / Spulen |
|---|---|---|---|
| H.264/AAC in MP4, 1080p | `DIRECT` | 134 ms / 256 ms (auf 6 s) | 70 ms / 147 ms |
| H.264/AAC in MKV, 720p, 90 s | `DIRECT` | 736 ms / 776 ms (auf 60 s) | 170 ms / 145 ms |
| dasselbe, `containers: [mp4]` | `HLS`, `REMUX`, Grund `container` | 210 ms / 1033 ms | 136 ms / 145 ms |
| HEVC Main10, `video: [h264]` | `HLS`, `TRANSCODE`, Grund `video_codec` | 477 ms / 637 ms | 469 ms / 554 ms |
| MKV mit SRT, `subtitleDelivery: text` | `DIRECT` + `subtitleUrl` (.vtt) | 324 ms, WebVTT-Zeilen erscheinen | 222 ms, ebenso |

`startPlayback` selbst dauert 9 bis 38 ms. Alle 7 Tests grün auf beiden Geräten.

**Was der Emulator nicht kann:** HEVC Main10 direkt (der Server spielt direkt, weil `hevc` in
den Fähigkeiten steht; der Decoder `c2.goldfish.hevc.decoder` scheitert auf API 36 mit
`ERROR_CODE_DECODING_FAILED`, auf API 28 bleibt er hängen), HDR, Dolby-Ton (AC3/EAC3/DTS: kein
Decoder), echte Hardware-Decoder.

**Entscheid.** Media3 für Direkt und HLS wie vorgesehen. Die App meldet `hevc` nur, wenn
`MediaCodecList` das Profil der Datei kann; da `PlaybackCapabilities` heute keine Profile kennt,
meldet sie `hevc` nur bei einem Main10-fähigen Decoder (die meisten HEVC-Dateien sind 10 Bit).
Gleiches gilt für `ac3`/`eac3`/`dts` (nur mit Decoder melden). In der MKV sieht ExoPlayer auch die
eingebetteten Untertitel (5 Textspuren); die App nimmt trotzdem die Spuren des Servers.
**Für Stufe 5:** Fähigkeiten aus `MediaCodecList` ableiten, Messung auf einem echten Gerät.

## 5. Download-Datei lokal abspielen und spulen

**Gemessen** mit `DownloadPlayTest` (API 36): `getDownloadOptions` → `createDownloads` (`low`)
→ Manifest nach 2,1 s bereit (13'125'188 Byte, 23 Segmente, Init 1291 Byte,
`avc1.640029,mp4a.40.2`) → Datei mit `dt` in 319 ms geholt (Antwort 200, `ETag`,
`Accept-Ranges: bytes`), Grösse stimmt.

| Weg | erstes Bild | spulbar | Spulen auf 60 s |
|---|---|---|---|
| 1: die fMP4-Datei direkt | 220 ms | **nein** (`isCurrentMediaItemSeekable=false`, kein `sidx`/`mfra`) | ignoriert, Wiedergabe läuft von vorn weiter |
| 2: lokale HLS-Liste aus dem Fragmentindex (`EXT-X-MAP` + `EXT-X-BYTERANGE` auf dieselbe Datei) | 277 ms | ja, Dauer 90'021 ms | 232 ms, Position 60'303 ms |

**Entscheid: Weg 2.** Die App schreibt beim Abschluss eines Downloads eine `.m3u8` neben die Datei
(wie `loader.ts` die Abspielliste baut) und spielt sie mit `HlsMediaSource`. Keine Änderung am
Server, keine eigene `SeekMap`. **Für Stufe 6.**

## 6. Abruf im Hintergrund

**Gemessen** mit einem eigenen Range-Server auf dem Host (dünn besetzte Datei 20 GiB, starker
`ETag`, Bremse 50 MiB/s, Log je Anfrage) und einer Abruf-Klasse, die an der Dateilänge fortsetzt
(`Range` + `If-Range`, 200 statt 206 heisst neu anfangen).

Übertragungsauftrag (`JobInfo.setUserInitiated(true)`, `setPersisted(true)`, API 36, Gerät mit
40 GB Datenbereich), gestartet aus der sichtbaren App:

| Zeit | Ereignis | Ergebnis |
|---|---|---|
| 0 s | `schedule` → `onStartJob` nach 0,4 s | Abruf von 0 an, HTTP 200, 39–44 MiB/s |
| +11 s | App aus der Übersicht weggewischt | Prozess läuft weiter (gleiche PID), Abruf ungestört |
| +85 s, bei 3,7 GiB | `adb reboot` | kein `onStopJob`; nach dem Start ruft das System den Auftrag selbst wieder auf (13 s nach Boot), `Range: bytes=3920946711-` mit `If-Range` → 206 ab 3739 MiB |
| bei 17,1 GiB | Emulator hart beendet (Speichermangel des Hosts, wie Stromausfall) | nach Kaltstart wieder selbst gestartet, 206 ab 17'478 MiB (Dateilänge nach dem Absturz) |
| Ende | | «complete at 20480 MiB», 45 MiB/s; Auftrag danach aus dem Scheduler verschwunden |

Vordergrunddienst `dataSync` auf API 28: 45 s, 1805 MiB bei gebremsten 40 MiB/s; Wegwischen
aus der Übersicht beendet ihn nicht (`isForeground=true`, gleiche PID).

**Entscheid.** UIDT für vom Nutzer gestartete Downloads ab API 34, `dataSync`-Dienst für API 28
bis 33, beide über dieselbe Abruf-Klasse mit `Range`/`If-Range` und Fortsetzung an der Dateilänge.
**Falle:** Nach einem harten Absturz kann das Ende der Datei ungeschrieben sein, obwohl die Länge
stimmt. Die App schreibt darum mit `FileDescriptor.sync()` je 8 MiB und setzt nach einem
Neustart am letzten gesicherten Block an, oder prüft am Ende die Segmentgrenzen des Manifests.
**Für Stufe 6.**

## 7. Abruf ohne offene App in kurzen Läufen (WorkManager)

**Gemessen** auf API 36 mit `ChunkWorker` (`OneTimeWorkRequest`, Netz verbunden, Rückfall linear
10 s), App im Hintergrund, Range-Server auf 10 MiB/s gebremst, damit ein Lauf nicht fertig wird:

| Zeit | Zustand | Ergebnis |
|---|---|---|
| 0 bis 12:51 min | Bildschirm an, am Strom, Bucket `active` | ein Lauf ohne Unterbruch, 7,2 GiB; keine 10-Minuten-Grenze gesehen |
| 12:51 min | Akku (`battery unplug`), Bucket `rare`, Bildschirm aus, Doze erzwungen | Netz sofort weg, `stopped by the system, reason=7` (`STOP_REASON_DEVICE_STATE`) |
| bis 24:51 min | Doze aufgehoben, Bildschirm an, dann Bucket `working_set`, weiter auf Akku | kein neuer Lauf: Auftrag «Ready: true», aber vom System gebündelt aufgeschoben |
| 24:52 min | wieder am Strom | neuer Lauf nach unter 1 s, `Range` → 206 ab 7712 MiB |

Auf API 28 lief ein Worker vor dem Absturz der Emulatoren (Punkt 10) 9:29 min ohne Unterbruch
(3289 MiB bei 10 MiB/s).

**Entscheid: WorkManager stückweise** für Auto-Downloads, mit derselben Abruf-Klasse (Range,
`If-Range`), als Ergänzung zum Abruf beim Öffnen. Jeder Lauf setzt verlässlich fort; wann er läuft,
entscheidet das System: auf Akku und in Doze lange nicht, am Strom sofort. Für «über Nacht am
Ladegerät» trägt das. Vorschlag: Einschränkung «nicht getaktetes Netz» bei «nur WLAN», keine
Pflicht zum Laden (sonst nie unterwegs), und beim Öffnen der App läuft ein offener Auto-Download als
UIDT weiter (ab API 34, mit Mitteilung). **Für Stufe 6;** die echte Verteilung über einen Tag misst
erst das Handy.

## 8. Generatoren für Texte und Farben

**Texte** (`gen_strings.py`, Prototyp; Stufe 3 setzt ihn als Gradle-Task um). Je Namensraum, was
er nicht trägt (Zählung über `de.json`, 5689 Texte):

| Namensraum | Texte | Mehrzahlformen | `_zero` | Platzhalter | `$t()` | Auszeichnung | `{{x, number}}` | übertragbar |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| admin | 3744 | 385 | 4 | 1014 | 0 | 4 | 12 | 99 % |
| item | 106 | 6 | 0 | 43 | 0 | 0 | 1 | 99 % |
| audioSeries | 27 | 4 | 0 | 11 | 0 | 0 | 3 | 88 % |
| alle übrigen 35 Namensräume | 1812 | 150 | 0 | 381 | 0 | 0 | 0 | 100 % |
| **alle** | 5689 | 545 | 4 | 1449 | 0 | 4 | 16 | 99 % |

Lauf für die 35 Namensräume ohne `admin`, `setup`, `invite`: 23 Sprachdateien, 37'948 Ressourcen,
88 übersprungen (4 je Sprache, alle `{{x, number}}`); `aapt2` baut alle 23 Ordner
(`values`, `values-de`, `values-de-rDE`, `values-pt-rBR`, `values-zh-rTW` …) ohne Fehler.

Im Emulator (API 28 und 36), Testressource mit `zero`/`one`/`other`: Deutsch und Englisch nehmen
für 0 die Form **`other`**, `zero` wird nie ausgewertet (bestätigt die Annahme aus 5.4). Die
Kette `de-CH` → `values-de` und `fr-CH` → `values-fr` greift; `pt-BR` findet `values-pt-rBR`.

**Entscheid.**

- Alle Namensräume der Zuschauerseiten lassen sich übernehmen; `admin` braucht die App nicht.
- `{{x, number}}` überträgt der Generator als `%N$s`; die App formatiert die Zahl vorher mit
  `NumberFormat` der Sprache (Schweizer Format für `de`, wie die Web-App). Damit sind es 100 %.
- `_zero` in Sprachen ohne Null-Kategorie (de, en …) lässt der Generator weg und meldet es; wo die
  Seite «keine» zeigen soll, braucht sie einen eigenen Text (heute nur 4 Fälle in `admin`).
- Auszeichnung (`<path>…</path>`, 4 Fälle, alle in `admin`) und `$t()` (0 Fälle): Abbruch mit
  Schlüssel, wie 5.4 es verlangt.
- Platzhalter-Reihenfolge kommt aus `en`, damit alle Sprachen dieselben Positionen nutzen.
- Lint meldet an den erzeugten Texten `ImpliedQuantity`, `UnusedQuantity`, `MissingTranslation`
  und `Typos` (Schweizer «ss»); Stufe 3 schaltet diese Prüfungen für den erzeugten Ordner ab.

**Farben und Masse** (`build-kotlin.mjs`, drittes Format über Style Dictionary 5.7):
`SlouchcatTokens.kt` mit 174 Werten, kompiliert in Compose. `#RRGGBBAA` → `Color(0xAARRGGBB)`
(Stichprobe `color.bg.overlay` `#0b0c10d9` → `0xD90B0C10`, CSS `rgba(11,12,16,0.85)`), `rem` × 16
→ `dp` (Schriftgrössen `sp`), `px` → `dp`, `ms` → `Int`, `cubicBezier` → `CubicBezierEasing`.
Weggelassen: 6 Schriftlisten (die App nimmt die Systemschrift) und `9cqi` (Containereinheit).
**Abweichung von 5.5:** `em` (4 Werte, Buchstabenabstand) gibt es in Compose als `.em`; der
Prototyp übernimmt sie statt sie wegzulassen. Fallen: Gruppen heissen wie Compose-Typen
(`Color`), darum schreibt der Generator voll qualifizierte Namen.
**Für Stufe 3:** beide Generatoren als Gradle-Tasks in `android/tools/`, Ausgabe im Bauordner.

## 9. Wächter

**Gemessen:** `./gradlew :app:lintDebug` über ein Kleinstbeispiel mit `Text("Hallo Welt")`,
`Text(text = "Fester Text", modifier = Modifier.background(Color(0xFF112233)))` und
`color = Color.Red`: Android Lint 32.5.2 meldet **nichts** davon (`HardcodedText` gilt nur für
XML-Layouts).

**Entscheid: eigene Lint-Regel** (`Rule9Detector`, Modul `lint-rules`, `lintChecks(project(…))`,
rund 90 Zeilen UAST). Sie meldet Zeichenketten als `text` an `Text`/`BasicText` und
`Color(<Zahl>)` sowie `Color.Red`… als Fehler und nimmt den erzeugten Code aus. Ergebnis am
Beispiel: 4 von 4 Verstössen gemeldet, die Zeile mit `stringResource` und `SlouchcatTokens`
nicht. Detekt oder Konsist sind nicht nötig; die Regel läuft im ohnehin nötigen Lint-Lauf.
**Für Stufe 3.**

## 10. Bau und Tests

| Lauf | Dauer auf dieser Maschine |
|---|---|
| sauberer Bau `assembleDebug` (Daemon kalt, Abhängigkeiten im Cache, ohne Build-Cache; mit Client-Erzeugung, 257 Modelldateien, 23 Sprachen) | 52 s |
| JVM-Tests (`:app:testDebugUnitTest` Compose/Robolectric + `:api:test`), neu ausgeführt | 7,1 s gesamt, Compose-Test 4,4 s (erster Lauf 7,9 s) |
| Lint, erster Lauf mit eigener Regel | 35 s |
| Instrumentierungstests im Emulator, `PlaybackTest` (7 Tests) | 9,9 s (API 36), 35 s (API 28) |
| Emulator-Start kalt bis `boot_completed` | rund 25 s (Logbuch des Koordinators) |

Robolectric 4.18.1 läuft auf SDK 36 unter JDK 21 (`@Config(sdk = [36])`), samt Compose-Test mit
erzeugten Texten und `qualifiers = "de"`.

**Schätzung CI-Job** (GitHub-Runner, kalt): JDK und SDK einrichten 2–3 min, Gradle mit
Abhängigkeiten 3–4 min, Lint 1 min, JVM-Tests 1 min: **8 bis 12 min ohne Emulator**; mit einem
Emulator-Job (KVM) zusätzlich 5–8 min. Messung in der CI macht Stufe 3.

**Falle:** Zwei Emulatoren (2 und 3 GB) plus Gradle- und Kotlin-Daemon (3,7 GB) neben anderen
Agenten haben den Host in den Speichermangel getrieben; der OOM-Killer beendete beide Emulatoren.
Je Agent höchstens ein Emulator, Gradle-Daemons nach dem Lauf stoppen.

## 11. Versionen von heute (21.11.2026)

Gelesen aus `maven-metadata.xml` von Google Maven (G), Maven Central (C), dem Gradle-Plugin-Portal
(P) und `services.gradle.org` (S).

| Baustein | Version | Quelle | Lizenz |
|---|---|---|---|
| Gradle | 9.9.2 | S | Apache-2.0 |
| Android Gradle Plugin | 9.5.2 | G | Apache-2.0 |
| Kotlin (mit Compose- und Serialization-Plugin) | 2.4.21 | C | Apache-2.0 |
| KSP | 2.3.14 | C | Apache-2.0 |
| Compose BOM | 2026.11.01 (UI 1.13.0) | G | Apache-2.0 |
| Material 3 | 1.5.0 | G | Apache-2.0 |
| Material 3 Adaptive | 1.3.0 | G | Apache-2.0 |
| Navigation Compose / Navigation 3 | 2.10.2 / 1.2.2 | G | Apache-2.0 |
| Media3 | 1.11.1 | G | Apache-2.0 |
| `androidx.tv:tv-material` | 1.1.0 | G | Apache-2.0 |
| Room | 2.8.5 (Room 3: 3.0.3) | G | Apache-2.0 |
| DataStore | 1.3.0 | G | Apache-2.0 |
| WorkManager | 2.12.0 | G | Apache-2.0 |
| Activity Compose / Core KTX / Lifecycle | 1.13.0 / 1.19.1 / 2.11.1 | G | Apache-2.0 |
| OkHttp | 5.5.1 | C | Apache-2.0 |
| Coil | 3.6.3 (`io.coil-kt.coil3`) | C | Apache-2.0 |
| kotlinx.serialization / kotlinx.coroutines | 1.13.0 / 1.11.0 | C | Apache-2.0 |
| Robolectric | 4.18.1 | C | MIT |
| Firebase Messaging (BoM) | 26.0.2 (BoM 35.2.0), Plugin `google-services` 4.6.0 | G | Apache-2.0 (zieht Play-Dienste mit Android-SDK-Lizenz) |
| Google code scanner | `play-services-code-scanner` 16.1.0 | G | Android-SDK-Lizenz (proprietär) |
| Scanner für `foss` | ZXing `core` 3.5.4, `zxing-android-embedded` 4.3.0 | C | Apache-2.0 |
| Krypto | `tink-android` 1.24.0 | C | Apache-2.0 |
| openapi-generator | 7.27.0 (Plugin und CLI) | P, C | Apache-2.0 |
| Android Lint API | 32.5.2 | G | Apache-2.0 |
| Android-SDK | `compileSdk` 37.2, Build-Tools 37.0.0 | SDK | Android-SDK-Lizenz |

**Abweichung `compileSdk`:** Compose UI 1.13.0, Material 3 1.5.0, Core 1.19.1 und OkHttp 5.5.1
verlangen beim Bau **`compileSdk` 37.1 oder höher** (Fehler von AGP: «requires … to compile
against version 37.1 or later»). Festlegung des Baus ist 36. Entscheid: `compileSdk` 37.2
(`compileSdk { version = release(37) { minorApiLevel = 2 } }`), `targetSdk` bleibt 36 (Handy)
und 34 (TV); damit gilt die Sperre fürs lokale Netz aus API 37 nicht. Die Alternative wären ältere
Bibliotheken.

## Nur im Emulator geprüft

- Krypto auf API 28 und 36 (Tink, JCA, PBKDF2-Dauer).
- Wiedergabe: nur Software-Decoder des Emulators, kein HDR, kein HEVC Main10, kein Dolby-Ton.
- Download-Wiedergabe über die lokale HLS-Liste.
- UIDT: Wegwischen, `adb reboot` (Neustart des Gastsystems) und ein harter Absturz; gebremster
  Range-Server auf dem Host statt echtem Server, 40–47 MiB/s.
- `dataSync`-Dienst auf API 28, 45 s.
- WorkManager ohne Doze und ohne Akku-Optimierung des Herstellers.
- Mehrzahlformen und Sprachketten.

## Auf einem echten Gerät oder mit dem Firebase-Projekt noch zu messen

Anleitung an Marco, wenn die Stufen so weit sind:

1. **Firebase-Projekt** (Stufe 2): Zustellung einer Daten-Nachricht mit `wp` von 3602 Byte; ob
   Google sie annimmt (Zählweise der 4096 Byte); Verzögerung mit `normal` und `high` bei
   gesperrtem Handy über Nacht (Doze).
2. **Wiedergabe** (Stufe 5): eine HEVC-10-Bit-Datei und eine HDR-Datei direkt, Ton AC3/EAC3;
   welche Decoder das Handy meldet.
3. **Downloads** (Stufe 6): 20 GB über Nacht mit UIDT, Handy gesperrt, im WLAN; danach einmal das
   Handy neu starten und schauen, ob der Abruf fortsetzt.
4. **Auto-Downloads** (Stufe 6): WorkManager-Läufe über einen Tag mit Akku-Optimierung des
   Herstellers (Benachrichtigung im Abgleich-Log der App).
5. **Hersteller-Eigenheiten:** Wegwischen auf Geräten, die dabei den Prozess beenden.

## Stufe 5: Videoplayer (21. November 2026)

**Gemessen** mit `PlayerFlowTest` (`task android:e2e CLASS=…PlayerFlowTest`) im Emulator gegen
den Server dieses Zweigs mit den Clips von `scripts/android-test-video.sh` (H.264 in MKV, 1280 × 720,
zwei AAC-Spuren, SRT). Gemessen wird vom Abspielstart des Players (`startPlayback` geht hinaus,
wenige Millisekunden nach dem Tippen) bis zum ersten Bild auf der Fläche
(`Player.Listener.onRenderedFirstFrame`); die App schreibt den Wert ins Log
(`Slouchcat/player: first picture ms=…`).

| Start | Modus des Servers | `sc_phone` (API 36) | `sc_tablet` (API 36) |
|---|---|---|---|
| Film, erster Start nach dem App-Start | `direct` | 910 ms | 650 ms |
| derselbe Film auf 480p (Qualität im Blatt) | `hls`, `transcode`, Grund `quality` | 694 ms | 716 ms |
| Folge 1 | `direct` | 390 ms | 363 ms |
| Folge 2 (vom Countdown gestartet) | `direct` | 340 ms | 340 ms |

Der erste Start enthält das Anlegen von ExoPlayer, Mediensitzung und Decodern im frischen Prozess;
spätere Starts liegen bei 340 bis 390 ms. Der Transcode lief auf dem Entwicklungsrechner mit
Hardware-Encoder; ein Server ohne GPU braucht für das erste Segment länger.

**Was der Emulator nicht kann (bleibt auf dem echten Handy nachzumessen):**

1. **HEVC und HDR:** Der Emulator hat keinen Main-10-Decoder; die App meldet dort kein `hevc` und
   kein `hdr`. Auf dem Handy: eine HEVC-10-Bit-Datei und eine HDR10-Datei direkt abspielen, die
   Fähigkeiten in der Wiedergabe-Info ansehen (Gerät, Abspielart), Start bis erstes Bild messen.
2. **Dolby-Ton und Durchreichen:** AC-3/E-AC-3 über den Lautsprecher (Decoder vorhanden oder nicht)
   und über ein USB-C/HDMI-Kabel an einer Tonanlage; der Rückfall bei stummer Spur ist nur mit
   Fakes geprüft (`PlaybackControllerTest`).
3. **Stromverbrauch:** eine Stunde Film direkt und als Transcode, Akkustand vorher und nachher.
4. **Kopfhörer- und Bluetooth-Tasten:** Wiedergabe/Pause über die Taste eines Kopfhörers, Pause
   beim Abziehen des Kopfhörers, Audiofokus bei einem Anruf (die `MediaSession` ist da, im Emulator
   ohne Tasten nicht prüfbar).
5. **Hochformat und Drehen:** Der Player dreht das Handy ins Querformat; auf einem Gerät mit
   gesperrter Drehung prüfen, dass er quer bleibt und beim Verlassen zurückkehrt.

Der Emulator ist kein Gerät: Software-Decoder, kein HDR, keine Tonausgänge ausser dem Lautsprecher.
Die Zeiten oben zeigen die Grössenordnung und dass der Weg trägt, nicht, was ein Handy misst.
