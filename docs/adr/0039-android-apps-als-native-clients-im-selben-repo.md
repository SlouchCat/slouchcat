# ADR 0039: Android-Apps als native Clients im selben Repo

- Status: gebaut auf dem Zweig `claude/k27-stufe3-a` (21. November 2026, Stufe 3 Teil 1 von
  Konzept 27), **angenommen mit dem Merge**. Die Zahl ist in Konzept 26 reserviert.
- Quelle: Konzept `docs/planning/27-android-handy-app.md`, Abschnitte 3.2, 5, 6, 11.1, 11.5, 12, 14;
  Konzept `docs/planning/26-android-tv.md`, Abschnitte 3.2 und 3.5; Fragen F-1, F-13, F-33 mit ihren
  Empfehlungen; Marcos Entscheide E-2, E-13, E-18, E-20; Bericht der Messung
  `docs/perf/K27-android-handy.md` (Stufe 0); Logbuch des Baus («Android-Architektur»)
- Kein Vertrag des Servers, keine Migration. Releases der Apps mit eigenen Tags `app-vX.Y.Z` (Handy)
  und `tv-vX.Y.Z` (Fernseher)
- Harte Regeln: 8 (Oberflächentest je Seite der Handy-App), 9 (Texte und Farben auch in `android/`),
  10 (kein Token, Gutschein und keine Gerätekennung im Log der App), 11 (benannte Ausnahme seit
  PR #192), 12 (Entscheid im Server)

## Kontext

Bis November 2026 waren native Apps ein Nicht-Ziel; die PWA lief auch auf Handy und Fernseher. Marco
hat zwei native Android-Apps bestellt: eine für das Handy (Konzept 27) und eine für den Fernseher
(Konzept 26). Offen war, wo sie liegen (F-1), wie sie Vertrag, Texte und Tokens bekommen, ohne die
Regeln 1 und 9 zu umgehen, und wie Bau und CI sie prüfen, ohne dass wer nur am Server arbeitet ein
JDK braucht.

## Entscheid

1. **Ein Ordner `android/` im selben Repo** (F-1, Empfehlung): ein Gradle-Projekt mit Kotlin-DSL,
   Wrapper im Repo (Gradle 9.9.2, Prüfsumme gepinnt), Versionskatalog
   `android/gradle/libs.versions.toml`, eingebundene Bauten `build-logic` (Konventionen) und `tools`
   (Generatoren). Module: `app-mobile`, später `app-tv`, und gemeinsam `core/api`, `core/session`,
   `core/design`, `core/strings`, `lint`; `core/player` und `core/pairing` legt die Stufe an, die sie
   füllt. Ein Pull Request kann Vertrag, Server und App zusammen ändern.
2. **Erzeugtes wird nie eingecheckt.** Beim Bau entstehen im Bauordner:
   - der **Kotlin-Client** aus `api/openapi.yaml` (openapi-generator 7.27.0, `kotlin`, OkHttp,
     kotlinx.serialization; Stufe 0 Punkt 3). Ein Vorlauf (`TrimOpenApi`) schneidet den Vertrag auf
     die Operationen der App zu: mit `x-clients` nach dieser Liste, ohne nach `x-visibility`
     (`public`, `account`, `scoped`). Unbekannte Felder und Aufzählungswerte brechen den Client nicht
     (`UNKNOWN_DEFAULT_OPEN_API`, Test);
   - die **Texte** aus `web/src/locales/*.json` für die Namensräume in
     `android/strings-namespaces.txt` (`R.string.<namensraum>_<schlüssel>`), alle 22 Sprachen und
     `de-DE`. Texte nur für die Apps stehen in den Namensräumen `app` und `tv` derselben Dateien; der
     Übersetzungsweg aus ADR 0033 bleibt einer. `_zero` wird ein eigener Text, weil Android die Form
     für null in Deutsch und Englisch nie wählt; was der Generator nicht kennt (`$t()`, Auszeichnung,
     andere Formate als `number`), bricht den Bau mit dem Schlüssel ab;
   - die **Farben und Masse** aus `packages/design-tokens` über ein drittes Ausgabeformat
     (`build-kotlin.mjs`, `SlouchcatTokens.kt`). `task gen`, `task gen:check` und `dist/` bleiben, wie
     sie sind; Regel 1 bekommt keine neuen Verzeichnisse.
3. **Wächter im Lint-Lauf** (Stufe 0 Punkt 9: Android Lint meldet festen Text in Compose nicht): drei
   eigene Regeln als Fehler in jedem Modul, `SlouchcatHardcodedText` (Text an einem Text-Parameter
   eines Composables), `SlouchcatHardcodedColor` (`Color(0x…)`, `Color.Red` …) und
   `SlouchcatDirectLog` (`android.util.Log`, `println`, `printStackTrace` am Logger vorbei). Geloggt
   wird nur über `com.slouchcat.core.api.log.Log`, das Token, Gutschein, Gerätekennung und
   Stream-Token entfernt; nur ein `LogSink` schreibt nach logcat. Erzeugter Code ist ausgenommen.
4. **Ein HTTP-Client** (`SlouchcatHttp` in `core/api`) für API, Bilder und Player: `X-Slouchcat-Client`
   (`mobile/<Version>`) an jeder Anfrage, `X-Requested-With: Slouchcat` an jeder schreibenden,
   Bearer und `X-Slouchcat-Device` nur an den eigenen Server, je Netzsprung geprüft: eine
   Weiterleitung an einen anderen Host trägt sie nie, Weiterleitungen von https auf http folgt der
   Client nicht. Unverschlüsseltes HTTP ist für alles freigegeben (E-18,
   Netzwerk-Sicherheitskonfiguration); die Seite «Serveradresse» warnt vor HTTP ausserhalb des
   Heimnetzes, entschieden an der aufgelösten Adresse: die Bereiche aus K27 2.7 (10/8, 172.16/12,
   192.168/16, 169.254/16, 100.64/10), dazu Loopback, `fe80::/10` und `fc00::/7`. Globale
   IPv6-Adressen gelten als fremd, auch wenn sie im eigenen Netz liegen; die Warnung erscheint dann
   auch zu Hause (sicher, aber lästig). `100.64/10` zählt mit, weil VPNs wie Tailscale es nutzen;
   die Kehrseite: eine Adresse aus dem CGNAT eines Anbieters warnt nicht.
   **Zertifikate:** Die App vertraut den Zertifizierungsstellen des Systems und denen, die der
   Nutzer auf dem Gerät installiert hat. Ein Haushalt mit eigener Zertifizierungsstelle erreicht
   seinen Server so wie im Browser. Die Angaben zur Verbindung sagen dann nicht einfach
   «Verbindung verschlüsselt», sondern «Verschlüsselt mit einem selbst installierten Zertifikat»
   (die App prüft, ob die Kette in einer vom Nutzer installierten Stelle endet).
   **Fehler eines Aufrufs** kommen als `ApiResult` (`Ok`, `NotFound`, `NotAllowed`, `SignedOut`,
   `AppTooOld`, `Offline`, `Failed`) aus `ApiCaller`; 401 und 426 meldet er zusätzlich an
   `SessionEvents`, auf die die App an einer Stelle reagiert (Anmeldung, «Bitte die App
   aktualisieren»). Seiten halten, was sie laden, als `Loadable` (laden, leer, fehlgeschlagen,
   bereit). Der Logger entfernt Werte nach denselben Schlüsselendungen wie der Server.
5. **Verträglichkeit** (K27 11.5): die App braucht mindestens Vertrag 1.42.0 und liest ihn aus
   `getHealth.contractVersion`; fehlt das Feld, gilt der Server als zu alt.
6. **Oberfläche:** eine Activity, Compose mit Material 3, ein dunkles Thema aus den Tokens, keine
   Farben aus dem Hintergrundbild; Navigation Compose mit typisierten Routen; je Seite `XScreen`
   (zustandslos) und `XViewModel` (StateFlow); Abhängigkeiten von Hand in `AppGraph`, kein Hilt. Das
   Mockup `docs/planning/mockups/27-android-handy/` ist die Vorlage; jede Seite hat zwei Breiten
   (412 dp, ab 700 dp breit).
7. **Bau und Prüfung:** JDK 21 (Robolectric braucht es für SDK 36), `compileSdk` 37.2 (die heutigen
   Bibliotheken verlangen es, Stufe 0 Punkt 11), `targetSdk` 36 am Handy und 34 am Fernseher,
   `minSdk` 28. Bauarten `play` und `foss` (F-33). Task-Ziele `android:check`, `android:test`,
   `android:contract`, `android:apk`; `task ci` ruft `ci:android`, die CI hat den Job `android` im
   Gate. Je Seite ein Oberflächentest mit Robolectric (Regel 8).
8. **Lizenz:** Die Apps stehen wie das Repo unter AGPL-3.0. Die Bibliotheken der ersten Ausgabe
   stehen unter Apache-2.0 (Tests: Robolectric MIT, JUnit EPL-1.0). Googles Bibliotheken für Push und
   Scanner kommen nur in die Bauart `play`.

## Folgen

- Wer nur Server oder Web-App ändert, braucht für `task check:fast` kein JDK: Es prüft die Apps nur
  bei Änderungen an `android/`, am Vertrag, an den Texten oder an den Tokens, bei reinen
  Textänderungen nur mit dem Textgenerator, und überlässt es ohne JDK der CI. `task ci` dagegen
  baut die Apps immer (`ci:android`) und braucht JDK und Android-SDK.
- **Bekannte Grenze der Zertifikate:** Wer auf dem Gerät eine Zertifizierungsstelle installieren
  kann (Firmenverwaltung des Geräts, Prüfung des Datenverkehrs im Firmennetz), kann die Verbindung
  der App zu einem HTTPS-Server mitlesen: Passwort, Token und die Medienadressen mit `dt`, `st`
  und `rt`. Die App nennt diesen Fall in den Angaben zur Verbindung, verhindert ihn aber nicht.
  «Server weist sich aus» (F-38) schützt nur Server über HTTP.
- Die öffentlichen Operationen des Einrichtungsassistenten (`getSetupStatus`, `runSetup`) bleiben
  im Client, weil `public`-Operationen keine `x-clients`-Liste tragen; die App ruft sie nicht, der
  Vertragstest richtet damit seinen Server ein.
- Sicherung und Gerätewechsel nehmen nur die Serveradresse mit (`data_extraction_rules.xml`);
  Token, Datenbanken und Zwischenspeicher bleiben auf dem Gerät.
- Release-Bauten laufen mit R8; `task android:check` baut sie unsigniert mit.
- Eine Vertragsänderung, die die App bricht, zeigt der Job `android`; ab Stufe 10 gilt dazu die
  Übergangszeit aus K27 11.5.
- Jede neue Seite der Apps braucht ihre Texte in `de` und `en` (Namensraum `app` oder einen der
  Zuschauerseiten in `strings-namespaces.txt`) und einen Oberflächentest in beiden Breiten.
- Ein Lauf der CI dauert länger: der Job `android` kommt neben die bestehenden Jobs.

## Nachtrag 21. November 2026: das zweite App-Modul (Konzept 26 Stufe 2a)

- **`:app-tv`** ist das zweite App-Modul neben `:app-mobile`: Paket `com.slouchcat.app.tv`, App-Kennung
  `com.slouchcat.app` wie die Handy-App (E-20), Entwicklungsbau `.dev`, `targetSdk` 34, `minSdk` 28, keine
  Bauarten, eigene Versionsdatei (`app-tv/version.properties`, Tags `tv-v…`), R8 für `release`. Im
  Store ist sie das zweite Artefakt desselben Eintrags; erkannt wird sie am `LEANBACK_LAUNCHER` und am Banner.
- **Geteilt** werden `:core:api` (Client, Netz, «Server weist sich aus»), `:core:session` (Server,
  Token im Keystore, Sitzung), `:core:design` (Tokens, Farben; die TV-Masse kommen als
  `SlouchcatTokens.Tv` aus `packages/design-tokens/tokens-tv`), `:core:strings` (Texte, für den Fernseher
  im Namensraum `tv`) und neu `:core:pairing` (Schlüssel, QR-Code, Vermittlung, HPKE, Einlösen; der
  Fernseher empfängt, die Handy-App versiegelt in Stufe 9). **Nicht geteilt** wird die Oberfläche:
  Thema, Bausteine, Hülle und Seiten des Fernsehers liegen in `:app-tv` und nutzen «Compose for TV»
  (`androidx.tv:tv-material`). Der QR-Code entsteht mit dem Encoder von ZXing core (Apache-2.0).
- Die Hülle des Fernsehers ist ein Register wie die des Handys (`shell/Features.kt`, `merge=union`):
  Einträge der linken Leiste, Medien aus den Bibliotheken, Zeilen der Einstellungen.
- Steuerkreuz-Tests laufen als JVM-Tests mit Robolectric (TV-Qualifier, K26 Stufe 0 Punkt 7) in
  `task android:test`; der Ablauf im TV-Emulator samt Einrichten per Scan gegen eine lokale
  Vermittlung ist `task android:e2e:tv` und läuft nur lokal (`docs/perf/K27-android-ci.md`).

## Nachtrag 22. November 2026: gemeinsame Module der zwei Apps

Was beide Apps ohne Oberfläche brauchen, liegt in eigenen `:core`-Modulen; in `:app-mobile` und `:app-tv`
bleiben Seiten, Features, Routen, Navigator und Bausteine.

- **`:core:media`**: Kartenmodell (`MediaItem`), Bildadressen, Haken und grosser Knopf (`ItemModel`),
  Nachlader (`PageLoader`), `Loadable`, Zähler der Hülle (`LiveCount`), Bibliotheken des Profils und alle
  Repositories der Stöberseiten über den erzeugten Client (`MediaData`, je Graph einmal, Testdaten über
  den Konstruktor des Graphen).
- **`:core:browse`**: die ViewModels der Stöberseiten (Start, Bibliothek, Listen, Detail, Person, Suche,
  Merkliste und Gesehen, Sammlungen, Profile verwalten) samt Zuständen, Meldungen und Ereignissen. Wo
  sich die Apps unterscheiden, entscheidet ein Parameter (z. B. `offlinePin = false` für den Fernseher,
  E-8). Die Testhilfen beider Apps liegen in **`:core:browse-testing`** (nur `testImplementation`).
- **`:core:player`**: Fähigkeiten des Geräts, Wiedergabesitzung, Media3 über den gemeinsamen Client, Ablauf
  der Wiedergabe; jede App zeichnet ihren eigenen Player darauf.
- **`:core:offline`**: Downloads und Wiedergabe ohne Netz der Handy-App (Konzept 27 Stufe 6).
- **`:core:pairing`**: Kopplung des Fernsehers über die Vermittlung (siehe Nachtrag vom 21. November).

Ein Modul unter `:core` hängt nie an einer App und nie an Compose-Material; Lint mit den eigenen Regeln
und die Ziele `ciCheck`, `ciTest` und `fastCheck` erfassen jedes Modul.

## Nachtrag 23. November 2026: Ausgabe, Versionen und Release-Weg der Apps

- **Versionen:** Jede App hat ihre Versionsdatei (`app-mobile/version.properties`, `app-tv/version.properties`)
  und ihren Tag (`app-vX.Y.Z`, `tv-vX.Y.Z`); der Tag muss dem `versionName` entsprechen. Weil beide Apps ein
  Store-Eintrag mit derselben Kennung sind und jedes hochgeladene Bundle einen eigenen `versionCode`
  braucht, zählt die Handy-App ab 1 und bleibt unter 1000000, die TV-App zählt ab 1000001; je Ausgabe +1.
  Die Bauskripte brechen ausserhalb des Bereichs ab.
- **Signatur:** `release` wird nur signiert, wenn die vier Werte `slouchcat.signing.*` (oder
  `SLOUCHCAT_SIGNING_*`) gesetzt sind (`build-logic`, `Signing.kt`); sonst bleibt der Bau unsigniert. Der
  Upload-Schlüssel kommt nie ins Repo.
- **Release-Weg:** Der Workflow `release-android.yml` läuft nur bei `app-v*` und `tv-v*`, prüft Tag und
  Version, fährt `task android:check` und `task android:test` ohne Secrets und hängt die Dateien von
  `task android:dist` an ein GitHub-Release, das **nie «latest»** wird (`--latest=false`). Grund: Server und
  Updater lesen `releases/latest/download/manifest.json`; ein App-Release ohne Manifest darf das
  Server-Release dort nicht verdrängen. Anders als `release-relay.yml` legt dieser Workflow ein Release an,
  weil die APKs und Bundles einen Ort zum Herunterladen brauchen.
- Anleitung für den Betrieb: `docs/dev/ANDROID-INBETRIEBNAHME.md`.
