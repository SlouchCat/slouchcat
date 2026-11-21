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

Der neue Job `android-e2e` (Vertragstest und Ablauf im Emulator) läuft neben `android`:

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
Gate «übersprungen» für `android` und `android-e2e` nur bei `android_relevant == false` gelten lassen.
Für `relay/` gilt dasselbe mit den Pfaden `relay/`, `testdata/crypto/` und den Workflows.

## Stufe 4: Testmedien im Job `android-e2e` (21.11.2026)

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
