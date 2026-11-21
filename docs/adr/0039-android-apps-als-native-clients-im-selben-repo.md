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
   Bearer und `X-Slouchcat-Device` nur an den eigenen Server. Unverschlüsseltes HTTP ist für alles
   freigegeben (E-18, Netzwerk-Sicherheitskonfiguration); die Seite «Serveradresse» warnt vor HTTP
   ausserhalb des Heimnetzes, entschieden an der aufgelösten Adresse (Bereiche aus K27 2.7, dazu
   Loopback und die IPv6-Gegenstücke).
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

- Wer nur Server oder Web-App ändert, braucht kein JDK; `task check:fast` prüft die Apps nur bei
  Änderungen an `android/`, am Vertrag, an den Texten oder an den Tokens, und überlässt es ohne JDK
  der CI.
- Eine Vertragsänderung, die die App bricht, zeigt der Job `android`; ab Stufe 10 gilt dazu die
  Übergangszeit aus K27 11.5.
- Jede neue Seite der Apps braucht ihre Texte in `de` und `en` (Namensraum `app` oder einen der
  Zuschauerseiten in `strings-namespaces.txt`) und einen Oberflächentest in beiden Breiten.
- Ein Lauf der CI dauert länger: der Job `android` kommt neben die bestehenden Jobs.
