# K27: Laufzeiten der Android-Prüfungen und Pfadfilter

Stand 21. November 2026, Konzept 27 Stufe 3 (Teil 2). Konzept 27 Abschnitt 5.7 verlangt, die Laufzeit
der Android-Jobs zu messen und danach über Pfadfilter für `android/` und `relay/` zu entscheiden.

## Gemessen

Lokal: Entwicklungsmaschine (31 GB, eine geteilte Maschine), Emulatoren `sc_phone` (API 36, Pixel 9)
und `sc_tablet` (API 36, Pixel Tablet), Start ohne Fenster mit Schnellstart aus dem gespeicherten Zustand.

| Prüfung                           | Wo                                      | Zeit       | Bemerkung                                                                                                                                   |
| --------------------------------- | --------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| Job `android` (`task ci:android`) | GitHub, erster Lauf (PR #197, Teil 1)   | 7 min 50 s | SDK-Pakete, Gradle ohne Cache, beide Bauarten, Lint, 162 JVM-Tests                                                                          |
| `task android:check`              | lokal, Gradle warm                      | 57 s       | beide Bauarten debug und release (R8), Lint                                                                                                 |
| `task android:test`               | lokal, Gradle warm, Tests aus dem Cache | 22 s       | 330 Tests; nach Änderungen an `app-mobile` rund 2 min für dessen 107 Oberflächen- und ViewModel-Tests (Robolectric, eine Test-JVM zugleich) |
| `task android:contract`           | lokal, Gradle warm                      | 6 s        | Server bauen und starten, 4 Tests gegen den echten Server                                                                                   |
| `task android:e2e` (Handy)        | lokal                                   | 34 s       | Emulator bereit nach 18 s, Gradle und Installation 8 s, Ablauf 8 s                                                                          |
| `task android:e2e AVD=sc_tablet`  | lokal                                   | 39 s       | Emulator bereit nach 22 s                                                                                                                   |
| `task android:e2e SHOTS=…`        | lokal                                   | 33 s       | derselbe Ablauf über `am instrument`, 16 Fotos                                                                                              |

## Schätzung für GitHub

Die Schätzung galt dem Job `android-e2e` (Vertragstest und Ablauf im Emulator in einem Job). Seit dem 22. November 2026 sind es zwei Jobs, siehe «Emulator auf dem Runner» unten.

Geschätzt für beide zusammen, neben `android`:

| Schritt                                                 | geschätzt        |
| ------------------------------------------------------- | ---------------- |
| SDK-Pakete, Systemabbild API 36 (rund 1,5 GB), AVD      | 2 bis 4 min      |
| Go-Server bauen (Go-Cache)                              | unter 1 min      |
| Gradle: Client, App und Test-APK (Gradle-Cache)         | 3 bis 5 min      |
| Vertragstest                                            | unter 1 min      |
| Emulator kalt starten (KVM, ohne gespeicherten Zustand) | 1 bis 2 min      |
| Ablauf                                                  | unter 1 min      |
| **Summe**                                               | **8 bis 13 min** |

Der Job läuft parallel zu `android` (rund 8 min) und zu den Server-Jobs (rund 19 min). Er verlängert
den Gesamtlauf eines Pull Requests darum nicht. Die tatsächliche Zeit zeigt der erste PR-Lauf; sie
gehört hier nachgetragen.

## Entscheid: vorerst kein Pfadfilter

Empfehlung: **keine Pfadfilter für `android/` und `relay/` bauen**, solange die Android-Jobs neben den
Server-Jobs laufen und den Lauf nicht verlängern.

Gründe:

- Die Android-Jobs hängen nicht nur an `android/`: Der Client entsteht aus `api/openapi.yaml`, die
  Texte aus `web/src/locales`, Farben und Masse aus `packages/design-tokens`, der Vertragstest und der
  Ablauf brauchen den Server aus `server/`. Ein Filter müsste diese Pfade mitführen; eine vergessene
  Abhängigkeit liesse eine brechende Änderung grün durch.
- Umgekehrt ist der längste Weg der Server (e2e rund 19 min). Ein Pull Request nur an `android/`
  würde mit Filter 19 min Server-Prüfung sparen, aber die Jobs laufen ohnehin parallel; die Wartezeit
  bestimmt der längste Job, nicht die Summe.
- Die Regeln des Gates (ADR 0013) bleiben einfach: Jeder Job steht in `needs`, «übersprungen» ist nur
  grün bei reiner Dokumentation oder bei einem schon geprüften Stand auf `main`. Ein Pfadfilter
  bräuchte eine dritte Ausnahme mit eigener Ausgabe im Job `changes` und je Job eine eigene Regel.

Wann neu entscheiden: Wenn die Runner-Minuten knapp werden oder ein Android-Job länger als der
Server-Weg dauert. Dann eine Ausgabe `android_relevant` im Job `changes` (Pfade: `android/`,
`api/openapi.yaml`, `web/src/locales/`, `packages/design-tokens/`, `server/`, die Workflows) und im
Gate «übersprungen» für `android` und `android-contract` nur bei `android_relevant == false` gelten lassen.
Für `relay/` gilt dasselbe mit den Pfaden `relay/`, `testdata/crypto/` und den Workflows.

## Stufe 4: Testmedien im Emulator-Job (21.11.2026)

Der Emulator-Ablauf stöbert seit Stufe 4 durch eine echte Bibliothek. Dafür braucht der Server die
synthetischen Testmedien (`task testdata:media`, ffmpeg) und `apicheck -devices`, das ihn einrichtet.

| Schritt                              | lokal gemessen                    | GitHub, Schätzung                                                                              |
| ------------------------------------ | --------------------------------- | ---------------------------------------------------------------------------------------------- |
| `task testdata:media` frisch         | 53 s (6 min CPU auf allen Kernen) | 2,9 min bei einem Fehltreffer (Lauf 39789916437, Job `server-test`)                            |
| Testmedien aus dem Cache             | –                                 | wenige Sekunden: gleicher Schlüssel wie `server-test` und `e2e` (`.github/actions/test-media`) |
| ffmpeg über apt aus dem `.deb`-Cache | –                                 | unter 1 min (Cache je Gruppe, `.github/actions/system-packages`)                               |
| `apicheck -devices` samt Scan        | 10 s                              | ähnlich                                                                                        |

**Entscheid:** eingebaut. Der Cache der Testmedien teilt seinen Schlüssel mit den Server-Tests, die
ihn auf `main` füllen; bei einem Treffer kostet der Job unter einer Minute mehr, nur bei einem
Fehltreffer (neue ffmpeg-Fassung, geänderter Generator) knapp drei Minuten. Der Ablauf mit
eingespielten Daten (`BrowseShowcaseTest`, ohne Server) läuft in jedem Emulator-Lauf zusätzlich mit
und deckt Bühne, «Weiterschauen» und Bilder ab, die der Testserver ohne TMDB nicht hat.

## TV-App (K26 Stufe 2a): Emulator in der CI?

Gemessen am 21.11.2026 lokal mit `task android:e2e:tv` (Emulator `sc_tv`, Systemabbild Google TV API 34
x86_64, KVM, Server und Vermittlung schon gebaut, Gradle warm): Emulator bereit nach rund 14 s, ganzer
Lauf beider Abläufe (von Hand bis Abmelden, per Scan über die lokale Vermittlung) 29 s, davon 15 s am
Gerät.

In der CI kämen dazu: ein zweites Systemabbild (Google TV API 34, rund 1,5 GB, ohne Cache 2 bis 4 min),
der Kaltstart ohne gespeicherten Zustand (1 bis 2 min), Test-APK der TV-App (1 bis 2 min) und die
Vermittlung (unter 1 min). Zusammen 5 bis 9 min zusätzlich zu den geschätzten 8 bis 13 min von
`android-emulator`, also fast die doppelte Laufzeit des Jobs.

**Entscheid: kein TV-Emulator in der CI.** Die Vorgabe lautet «nur, wenn er die Laufzeit von
`android-emulator` nicht verdoppelt»; das ist nicht gesichert. Die CI prüft die TV-App stattdessen so:

- Job `android`: `ciCheck`/`ciTest` enthalten `:app-tv` und `:core:pairing` (Lint, R8-Release-Bau,
  JVM-Tests mit dem Steuerkreuz unter Robolectric für jede Seite, Kopplung gegen
  `testdata/crypto/`), das TV-APK liegt als Artefakt bei.
- Job `relay`: der Dienst, gegen den die Kopplung läuft.

Der Emulatorlauf bleibt lokal vor jedem PR, der die TV-App anfasst (`docs/dev/SETUP.md`). Neu
entscheiden, wenn `android-emulator` auf dem Runner hält, gemessen unter 8 min liegt oder ein Fehler nur im Emulator auffiel.

## Emulator auf dem Runner (22.11.2026): gemessen, nicht im Gate

Konzept 27 Abschnitt 5.7 lässt offen, ob die Oberflächentests im Emulator auf den Runnern von GitHub
laufen: «misst Stufe 3 und schreibt es nach `docs/perf/`». Gemessen am Pull Request der Stufen 3 bis 5:

| Lauf        | Stand                                  | Ergebnis                                                                                                     |
| ----------- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| 45032477885 | erster Lauf                            | AVD am falschen Ort angelegt (`XDG_CONFIG_HOME`), Job bricht vor dem Start ab. Behoben (`ANDROID_AVD_HOME`). |
| 45032862694 | AVD an festem Ort                      | Emulator bereit nach 89 s, **Segmentation fault** rund 50 s im Ablauf. Aufräumen hing an `adb`. Behoben.     |
| 45036212674 | `adb` der Aufräumroutine mit Zeitlimit | bereit nach 68 s, **Segmentation fault** nach rund 60 s, Logs als Artefakt.                                  |
| 45041798200 | Gast ohne Vulkan (`-feature -Vulkan`)  | bereit nach 70 s, **Segmentation fault** nach rund 80 s.                                                     |

Was die Logs zeigen:

- Der Emulator ist dieselbe Fassung wie auf der Entwicklungsmaschine (37.3.10.0, Systemabbild API 36
  `google_apis` x86_64, `-gpu swiftshader_indirect`, KVM). Dort läuft derselbe Ablauf durch
  (7 Tests, 0 Fehler, auch ohne Vulkan, Server mit Testmedien wie in der CI).
- Der Absturz kommt in allen drei Läufen an derselben Stelle des Ablaufs: Detailseite eines Titels,
  Tipp auf «Merken». Der Gast läuft bis dahin normal; im Log der App steht kein Fehler.
- Unmittelbar davor meldet der Gast auf dem Runner bei jedem Ton einen Schreibfehler seines
  Tongeräts (`pcm_writei failed with 'cannot read/write stream data: I/O error'`); lokal steht diese
  Zeile nie im Log. Vulkan scheidet als Ursache aus (vierter Lauf).
- `swiftshader_indirect` gilt laut Googles Fehlerdatenbank als veraltet und nicht mehr gepflegt;
  Abstürze mit Emulatoren ab 35 auf Ubuntu 24 sind dort gemeldet.

**Entscheid:** Der Ablauf im Emulator gehört nicht ins Gate, solange der Emulator auf dem Runner nicht
hält. Der frühere Job ist geteilt:

- `android-contract` (im Gate): der Vertragstest des Kotlin-Clients gegen einen echten Server. Er
  braucht keinen Emulator und lief in jedem der vier Läufe grün.
- `android-emulator` (nicht im Gate, darf rot sein): der Ablauf wie bisher, mit Testmedien. Er zeigt,
  wann der Emulator auf dem Runner hält; die Logs bleiben als Artefakt.

Der Ablauf läuft dafür auf der Entwicklungsmaschine vor jedem Pull Request, der die Apps anfasst
(`task android:e2e`, bei Änderungen an der TV-App auch `task android:e2e:tv`; `docs/dev/SETUP.md`).

Was als Nächstes zu versuchen ist, ohne dass ein Pull Request daran hängt: Töne im Gast abstellen
(seit diesem Stand schaltet das Skript die Tastentöne aus), eine andere Darstellung (`-gpu swangle`
oder `lavapipe`), ein Systemabbild ohne Google-Apps (`aosp_atd`) oder eine ältere API-Stufe. Hält der
Job über mehrere Pull Requests, kommt er zurück ins Gate.

## Absturz des Emulators (23.11.2026): Darstellung `swangle_indirect` statt `swiftshader_indirect`

**Befund:** Auf der Entwicklungsmaschine stürzte der Emulator im Handy-Ablauf auch bei ruhiger
Maschine ab, rund 26 s in `AudioFlowTest`. Das Skript meldete «Speicherzugriffsfehler», danach
`adb: device offline`. Der Logcat des Gasts endet jedes Mal damit, dass SystemUI die Medienkarte der
Mitteilung aufbaut (`ConstraintSet: id unknown media_action_barrier_…`, `HWUI: Image decoding`).

Die Speicherabzüge (`coredumpctl`, vier geprüfte Abstürze am 23.11.2026 zwischen 04:04 und 04:45) halten alle
an derselben Instruktion an (`movdqu 0x184f0(%rax,%rcx,1),%xmm5`, gleicher Versatz in der Seite). Die
Adresse liegt in Code, der zur Laufzeit erzeugt wurde, ausserhalb jeder Bibliothek des Emulators;
auf dem Stack stehen Gleitkommawerte (0,5; 1,5). Das passt zu Shader-Code, den der Renderer
SwiftShader zur Laufzeit erzeugt. Der Tonpfad (`-no-audio`) ist damit nicht als Ursache belegt.

**Versuche** (`CLASS=com.slouchcat.app.mobile.AudioFlowTest`, `sc_phone`, API 36, je ein Lauf mit
frischem Emulator, `PORT=9497`, `EMU_PORT=5574`):

| Einstellung                                         | Abstürze |
| --------------------------------------------------- | -------- |
| unverändert (`-gpu swiftshader_indirect`)           | 3 von 5  |
| `-gpu swangle_indirect` (ANGLE über SwiftShader-Vulkan) | 0 von 5  |

Die Einstellungen der SystemUI zur Medienkarte haben wir nicht verändert. Der Gast kennt unter
`settings secure` nur `media_button_receiver`, also keinen Schalter, der die Karte verbirgt. Der
Wechsel der Darstellung hielt schon im ersten Versuch; weitere Versuche (Karte aus, Ton-Backend)
waren nicht mehr nötig.

**Lösung:** `scripts/android-e2e.sh` startet den Emulator mit `-gpu swangle_indirect`. Mit `EMU_GPU`
lässt sich das überschreiben. Die Darstellung bleibt reine Software, wie bisher. Der CI-Job
`android-emulator` ruft dasselbe Skript auf und bekommt die Einstellung mit. Ob der Job auf dem
Runner damit hält, zeigt der nächste Pull Request: Die Abstürze dort (Detailseite, «Merken») hatten
dasselbe Bild. Die Medienbenachrichtigung bleibt an. `AudioFlowTest` prüft weiterhin, dass die
Mediensitzung im Hintergrund spielt; mit `SHOTS` fotografiert er die Mitteilung. Danach lief der ganze
Handy-Ablauf ohne `CLASS` einmal durch (Ergebnis im Bericht der Stufe 7).

`scripts/android-tv-e2e.sh` (TV-Emulator) nutzt weiter `swiftshader_indirect`; dort ist kein Absturz
gemeldet.

**Ergebnis auf dem Runner (PR #206, 23.11.2026):** Der Job `android-emulator` (AVD `ci_phone`,
`-gpu swangle_indirect`) lief ohne Absturz des Emulators. 13 von 16 Fällen waren grün, 3 rot:

| Fall                        | Befund                                                         |
| --------------------------- | -------------------------------------------------------------- |
| `AudioFlowTest`             | ein Warten von 30 s läuft ab                                   |
| `MessagesFlowTest.kt:349`   | die Mitteilung steht nicht unter `activeNotifications`         |
| `PlayerFlowTest.kt:242`     | ein Warten von 10 s läuft ab                                   |

Auf der Entwicklungsmaschine sind dieselben 16 Fälle grün. Der Job bleibt informativ und steht nicht im
Gate (Entscheid oben). Die drei Fälle auf dem Runner sind offen.

## Download-Limit und Rennen in JVM-Tests (23. November 2026)

**429 von Maven Central.** An den Pull Requests #208, #209 und #210 scheiterte der Job `android` je einmal oder
mehrmals an «Received status code 429 from server: Too Many Requests», immer in
`:app-mobile:generateFossDebugUnitTestLintModel`. Der Job fand keinen Gradle-Zwischenspeicher («Cache not found
for input keys: gradle-Linux-…»): Ein Pull Request speichert ihn nur für seinen eigenen Ref, und main speicherte
keinen, weil die CI auf main geprüften Inhalt überspringt (ADR 0013, Nachtrag). Jeder neue Zweig lud darum alle
Abhängigkeiten frisch. Seit dem vollen Lauf auf main vom 23. November 2026 (08:03 UTC, 2,17 GiB unter
`refs/heads/main`) treffen Pull Requests den Zwischenspeicher; #211 lief ohne 429. Ändert ein Zweig die
Gradle-Dateien, wechselt der Schlüssel und er lädt wieder frisch, bis main einmal voll gelaufen ist.

**Rennen in Tests.** Mit dem Zwischenspeicher laufen die JVM-Tests auf den Runnern (vier Kerne) anders
getaktet. In zwei Läufen hintereinander wurden zwei Tests rot, die lokal immer grün waren; beide Male lag das
Rennen im Test, nicht in der App:

| Test | Ursache | Behebung |
|---|---|---|
| `DownloadsViewModelTest.deleteAsksThenDeletesOnTheServerAndTheDevice` | `OfflineEngine.delete` markiert den Download zuerst (die Zeile verschwindet) und fragt danach den Server; der Test las die Aufrufe der Attrappe direkt nach der leeren Liste | wartet begrenzt auch auf den Server-Aufruf |
| `AudioServiceTest.headphonesPulledOutPauseTheSound` | Media3 meldet den Empfänger für `ACTION_AUDIO_BECOMING_NOISY` erst über seinen Hintergrund-Thread an; kam die Meldung vorher, empfing sie niemand | wartet vor dem Senden begrenzt, bis der Empfänger angemeldet ist |

Beleg: beide Klassen je zehnmal auf vier Kernen (`taskset -c 0-3`), alle grün. Rund 35 weitere Testklassen
wurden nach demselben Muster durchgesehen, ohne weiteren Beleg. Verdachtsfälle ohne Beleg, nicht geändert:
`OfflineEngineTest` (die Liste `syncs` ist nicht threadsicher), `OfflineWorkerTest.tearDown` (bricht den Scope
nicht ab), `FileFetcherTest` (Abbruch gegen eine gedrosselte Übertragung).

## Laufzeit des Emulator-Jobs (23. November 2026)

Marco am 23. November 2026: Der Job `android-emulator` dauert viel zu lange. Gemessen an drei Pull-Request-Läufen
desselben Tages (45132700824, 45158493082, 45163735384): 15,2 bis 15,6 min je Lauf.

**Wohin die Zeit ging** (Lauf 45163735384, Job 137695228452):

| Abschnitt                                              | Zeit  | Bemerkung                                                              |
| ------------------------------------------------------ | ----- | ---------------------------------------------------------------------- |
| Einrichten (Checkout, Go, Node, apt, SDK, AVD, Caches) | 110 s | davon 54 s SDK-Pakete mit dem Systemabbild                             |
| Server und Vermittlung bauen                           | 11 s  |                                                                        |
| Server einrichten, Helfer starten                      | 47 s  |                                                                        |
| Emulator kalt starten                                  | 43 s  | bereit 91 s nach dem Start des Skripts                                 |
| Gradle: App und Test-APK                               | 237 s | 197 von 251 Aufgaben ausgeführt, 54 aus dem Cache                      |
| Ablauf am Gerät                                        | 480 s | 16 Fälle, Summe 469 s                                                  |

Zwei Ursachen ausserhalb der Tests:

- **Der Gradle-Cache war fast leer.** Der Job teilte seinen Schlüssel (`gradle-e2e-…`) mit `android-contract`.
  Der Vertragstest ist nach vier Minuten fertig und speichert zuerst, und er baut nur `core:api`. Der Emulator-Job
  selbst speicherte nie: `actions/cache` speichert nur nach einem grünen Job, und dieser war in jedem Lauf rot.
  Die App entstand darum jedes Mal fast von Grund auf.
- **Der Gast nutzte den halben Runner.** `avdmanager` legt ein Gerät mit zwei Kernen an, der Runner hat vier.
  Der Bildschirm (1080 × 2424) wird in Software gezeichnet (`swangle_indirect`). Im Logcat des Laufs oben stehen
  80 Meldungen «Skipped … frames» mit zusammen 4468 ausgelassenen Bildern.

**Gemessen** auf dem Runner, je Variante ein eigener Job desselben Laufs (Wegwerf-Workflow auf einem eigenen
Zweig, nicht gemergt). Alle Varianten mit dem geänderten Skript (APK-Bau im Hintergrund, siehe unten).
«Stets grün» ist die Summe der 12 Fälle, die in allen sechs Varianten des ersten Laufs grün waren; sie ist das
Mass für die Geschwindigkeit des Gasts, weil ein roter Fall an seinem Zeitlimit hängt.

Lauf 45182593516:

| Variante           | Kerne | Bildschirm  | Gradle-Cache    | Emulator bereit | APKs bereit | Fälle | stets grün | rot | Job      |
| ------------------ | ----- | ----------- | --------------- | --------------- | ----------- | ----- | ---------- | --- | -------- |
| wie bisher         | 2     | 1080 × 2424 | `gradle-e2e`    | 141 s           | 256 s       | 444 s | 260 s      | 2   | 14,2 min |
| vier Kerne         | 4     | 1080 × 2424 | `gradle-e2e`    | 220 s           | 268 s       | 464 s | 287 s      | 2   | 15,5 min |
| kleiner Bildschirm | 2     | 720 × 1616  | `gradle-e2e`    | 258 s           | 350 s       | 438 s | 257 s      | 2   | 16,9 min |
| beides             | 4     | 720 × 1616  | `gradle-e2e`    | 175 s           | 303 s       | 384 s | 194 s      | 3   | 14,2 min |
| drei Kerne, klein  | 3     | 720 × 1616  | `gradle-e2e`    | 215 s           | 361 s       | 415 s | 232 s      | 3   | 15,5 min |
| beides, Cache      | 4     | 720 × 1616  | der von android | 116 s           | 121 s       | 323 s | 176 s      | 1   | 11,7 min |

Lauf 45185868418, alle mit vier Kernen und dem Cache des Jobs `android`:

| Variante | Bildschirm          | Emulator bereit | APKs bereit | Gradle-Bau | Fälle | rot | Job      |
| -------- | ------------------- | --------------- | ----------- | ---------- | ----- | --- | -------- |
| final a  | 720 × 1616, 280 dpi | 121 s           | 127 s       | 87 s       | 366 s | 3   | 11,9 min |
| final b  | 720 × 1616, 280 dpi | 117 s           | 122 s       | 81 s       | 329 s | 1   | 11,9 min |
| winzig a | 540 × 1212, 210 dpi | 117 s           | 123 s       | 82 s       | 280 s | 1   | 10,7 min |
| winzig b | 540 × 1212, 210 dpi | 159 s           | 167 s       | 119 s      | 334 s | 1   | 11,6 min |

Was die Zahlen zeigen:

- **Der Cache des Jobs `android` bringt am meisten.** Der Bau der beiden APKs dauert damit 81 bis 119 s statt
  4 bis 6 min (121 von 250 Aufgaben ausgeführt, 129 aus dem Cache). Das Zurückspielen der 2,2 GiB kostet 20 bis
  40 s statt 5 s.
- **Vier Kerne oder ein kleinerer Bildschirm allein bringen nichts** (287 s und 257 s gegen 260 s). Beides
  zusammen macht die stets grünen Fälle um ein Viertel bis ein Drittel schneller (194 s und 176 s). Mit vier
  Kernen und 720 × 1616 stehen im Logcat noch 11 Meldungen «Skipped … frames».
- **Noch weniger Pixel lohnen nicht belegbar.** 540 × 1212 war in einem Lauf schneller und im anderen gleich
  schnell wie 720 × 1616. 280 dpi ist zudem eine Dichte, die Android kennt; 210 dpi ist keine.
- **Der Bau neben dem Emulatorstart** hilft erst mit dem Cache. Mit kaltem Gradle teilen sich Bau und Start die
  vier Kerne: Der Emulator war 141 bis 258 s nach dem Start des Skripts bereit statt nach 91 s, und der Ablauf
  begann nicht früher als zuvor. Mit dem Cache sind APKs und Emulator nach rund zwei Minuten zugleich bereit;
  nacheinander wären es rund drei.
- Die Runner streuen stark: Dieselben SDK-Pakete brauchten 36 bis 101 s, zwei gleiche Varianten 329 und 366 s.

**Umgesetzt:**

- `ci.yml`, Job `android-emulator`: liest den Gradle-Cache des Jobs `android` (gleiche Pfadliste und gleicher
  Schlüssel, nur lesend über `actions/cache/restore`). Ein eigener Cache käme zu den 2,2 GiB dazu, und das
  Repository steht mit 9,99 von 10 GiB an der Grenze. `android-contract` behält `gradle-e2e-…`.
- `ci.yml`: Der AVD `ci_phone` hat vier Kerne (`hw.cpu.ncore=4`) und 720 × 1616 bei 280 dpi. Das sind dieselben
  412 × 923 dp wie `sc_phone`. Fotos macht der Ablauf in der CI keine.
- `scripts/android-e2e.sh`: Der Bau der beiden APKs läuft von der ersten Zeile an im Hintergrund, neben dem
  Einrichten des Servers und dem Start des Emulators. Der Ablauf wartet auf ihn. Das gilt auch lokal und für
  `SHOTS`. Bricht das Skript vorher ab, beendet es die Prozessgruppe des Baus.

**Ergebnis:** 11,7 bis 11,9 min (die drei Jobs mit vier Kernen, 720 × 1616 und dem Cache von `android`) statt
15,2 bis 15,6 min, also rund 3,5 Minuten oder knapp ein Viertel weniger. Der Rest verteilt sich so (Variante «final b»): Einrichten rund 3,5 min, bis zum ersten Fall 2,7 min, Ablauf 5,6 min.

**Nicht gemacht, mit Grund:**

- **Den Ablauf auf zwei Jobs verteilen.** Das brächte rund 2,5 min, für die doppelte Rechenzeit. Der Job steht
  nicht im Gate und bestimmt die Wartezeit eines Pull Requests nicht: Am längsten laufen `server tests (2)`
  (22 min) und `android` (16 bis 18 min).
- **Die Tests kürzen.** `AudioFlowTest`, `OfflineFlowTest`, `PlayerFlowTest` und `TvLinkFlowTest` machen zwei
  Drittel des Ablaufs aus. Sie spielen Ton und Bild in Echtzeit und warten auf Herzschläge des Servers.
  `TvLinkFlowTest` dauerte in allen zehn Jobs 31 bis 36 s, unabhängig von der Variante.
- **SDK-Pakete im Hintergrund laden** (rund 45 s): ein Prozess über mehrere Schritte des Workflows hinweg, für
  einen Job ausserhalb des Gates zu viel Mechanik.

**Rote Fälle in den zehn Jobs der Messung** (der Job bleibt rot und ausserhalb des Gates):

| Fall                                | rot in    | Stelle                                                               |
| ----------------------------------- | --------- | -------------------------------------------------------------------- |
| `AudioFlowTest`                     | 10 von 10 | `AudioFlowTest.kt:341`: das Radio spielt nicht innert 30 s           |
| `OfflineFlowTest`                   | 4 von 10  | `OfflineFlowTest.kt:252` und `:253`                                  |
| `MessagesFlowTest`                  | 2 von 10  | `FlowWaits.kt:45`: ein Text erscheint nicht innert 20 s              |
| `MessagesShowcaseTest`              | 2 von 10  | «Cannot run onActivity since Activity has been destroyed already»    |
| `SignInFlowTest`                    | 1 von 10  | `SignInFlowTest.kt:368`: «the phone is still signed in»              |
| `PlayerFlowTest`                    | 0 von 10  | in den drei Läufen davor jedes Mal rot (`PlayerFlowTest.kt:242`)     |

Mit dem Cache des Jobs `android` war in vier von fünf Jobs nur `AudioFlowTest` rot, mit `gradle-e2e` hatte
jeder der fünf Jobs mindestens zwei rote Fälle. Fünf Läufe je Seite belegen das nicht; die Ursache der roten
Fälle ist nicht untersucht.

`OfflineFlowTest.kt:253` wackelt auch auf der Entwicklungsmaschine: Im Lauf mit dem geänderten Skript
(`task android:e2e`, `sc_phone`, 8 Kerne, 12 GB) war der Fall rot und die übrigen 15 grün, die Wiederholung
allein (`CLASS=…OfflineFlowTest`) war grün. Der Fall spult im Offline-Player auf die Hälfte und wartet 30 s
darauf, dass wieder «Pause» zu sehen ist.
