# Weg zur Veröffentlichung im Play Store

Schritt für Schritt vom leeren Play-Konto bis zur öffentlichen App, zuerst die Handy-App (Konzept 27
Stufe 10), dann die TV-Geräteart (Konzept 26 Stufe 6). Je Schritt: was er tut, wo, woran du siehst,
dass es geklappt hat. Die Texte für die Formulare stehen in den anderen Dateien dieses Ordners:

| Datei | Inhalt |
|---|---|
| `eintrag-de.md`, `listing-en.md` | Store-Eintrag Deutsch und Englisch |
| `grafiken.md` | verlangte Bilder mit Massen |
| `datenschutz.md`, `privacy.md`, `datenschutz.html` | Datenschutzerklärung (Entwurf), Seite zum Veröffentlichen |
| `datensicherheit.md` | Datensicherheit, Vordergrunddienste, Alterseinstufung, Zielgruppe, Zugang für Prüfer |
| `deploy/demo/README.md` | Vorführserver für die Prüfer |
| `docs/dev/VERTRAEGLICHKEIT.md` | Verträglichkeitsregel, ab der öffentlichen Stufe verbindlich |

Die Namen der Menüs in der Play Console können sich ändern; gemeint ist jeweils die Stelle mit
dieser Aufgabe. Was die Papiere «unbelegt» nennen, steht am Ende jedes Schritts.

## 0. Vorher entscheiden

- **Öffentlicher Name (K27 R-15).** Bei einem privaten Entwicklerkonto zeigt der Store den
  bürgerlichen Namen und das Land. Ob auch die E-Mail-Adresse: Zwei Seiten von Google widersprechen
  sich (unbelegt). Wer das nicht will, braucht ein Konto als Organisation, und das vor dem ersten
  Hochladen.
- **12 Tester (F-35).** Empfehlung: Familie und Freunde aus der internen Testspur, früh anfragen.
  Jeder braucht ein Google-Konto und ein Android-Gerät.
- **Vorführserver (F-10)** läuft unter einem öffentlichen Namen mit HTTPS (`deploy/demo/README.md`).
- **Datenschutzerklärung (F-36)** geprüft und unter HTTPS veröffentlicht.

## 1. App anlegen

**Was:** legt den Eintrag `com.slouchcat.app` im Konto an. **Wo:** Play Console › «App erstellen».

1. App-Name `Slouchcat`, Standardsprache «Deutsch – de-DE», Typ «App», «Kostenlos».
2. Die Erklärungen zu Programmrichtlinien und US-Exportgesetzen bestätigen.

**Geklappt, wenn:** die App im Dashboard steht. Der **Paketname** steht erst mit dem ersten
hochgeladenen Bundle fest und ist danach unveränderlich («unique and permanent», K27 2.7). Darum
nie ein Bundle mit `.dev`-Anhängsel oder der Wegwerf-App `com.slouchcat.spike` hochladen.

## 2. Upload-Schlüssel erzeugen

**Was:** erzeugt den Schlüssel, mit dem du jedes Bundle vor dem Hochladen signierst. Google hält
den eigentlichen Signaturschlüssel (Play App Signing, K27 11.2). **Wo:** auf deinem Rechner, nie im
Repo.

```bash
mkdir -p ~/.config/slouchcat && chmod 700 ~/.config/slouchcat
keytool -genkeypair -v \
  -keystore ~/.config/slouchcat/upload.jks -storetype PKCS12 \
  -alias upload -keyalg RSA -keysize 4096 -validity 10000 \
  -dname "CN=<Name des Entwicklers>"
chmod 600 ~/.config/slouchcat/upload.jks
```

`keytool` fragt zweimal nach einem Passwort; es gehört in den Passwortmanager. `-validity 10000`
sind gut 27 Jahre; Google empfiehlt mindestens 25 und verlangt ein Ende nach dem 22. Oktober 2033
(Android-Doku «Sign your app»). Eine Kopie der Datei an einen zweiten, sicheren Ort legen
(`<Ort der Sicherung des Upload-Schlüssels>`).

Das Zertifikat (nur der öffentliche Teil, für einen späteren Wechsel des Schlüssels):

```bash
keytool -export -rfc -keystore ~/.config/slouchcat/upload.jks -alias upload \
  -file ~/.config/slouchcat/upload-certificate.pem
```

**Geklappt, wenn:** `keytool -list -v -keystore ~/.config/slouchcat/upload.jks` einen Eintrag
`upload` mit «Gültig bis» nach 2033 zeigt. **Verlust:** Bei Play App Signing lässt sich der
Upload-Schlüssel in der Play Console zurücksetzen (App-Integrität › App-Signatur); die App bleibt
dieselbe.

## 3. App-Bundle bauen und signieren

**Was:** baut das Bundle der Bauart `play` (FCM, Google code scanner) und signiert es. **Wo:** im
Repo, mit JDK 21 und Android-SDK aus `scripts/install-android-tools.sh` (unter `~/.local/share`,
`docs/dev/SETUP.md`, Abschnitt «Die Android-Apps: `android/`»).

1. `versionCode` und `versionName` in `android/app-mobile/version.properties` hochzählen
   (`versionCode` steigt bei jedem Hochladen um 1; Regel und Bereiche von Handy und TV:
   `docs/dev/ANDROID-INBETRIEBNAHME.md`, Abschnitt 7).
2. Die vier Signatur-Werte setzen (`docs/dev/ANDROID-INBETRIEBNAHME.md`, Abschnitt 6.2; der Schlüssel
   bleibt ausserhalb des Repos) und die Firebase-Werte (ebenda, Abschnitt 5). Ohne Firebase-Werte
   baut `play` ohne Push; für den Store müssen sie beim Bau eingespielt sein.
3. Bauen:

   ```bash
   task android:dist APPS=app
   ```

   Das Bundle liegt dann als `android/dist/slouchcat-app-<versionName>-play-release.aab` da, mit
   dem Upload-Schlüssel signiert. Heisst es `…-play-release-unsigned.aab`, waren die Signatur-Werte
   nicht gesetzt: Ohne die Werte baut Gradle das Bundle unsigniert (geprüft beim Bau, kein
   Debug-Schlüssel darin). Es lässt sich dann von Hand signieren (`docs/dev/ANDROID-INBETRIEBNAHME.md`,
   Abschnitt 6.3). `task android:bundle` baut nur die zwei Bundles, ohne Umbenennen.
4. Prüfen:

   ```bash
   jarsigner -verify android/dist/slouchcat-app-<versionName>-play-release.aab
   ```

**Geklappt, wenn:** `jarsigner -verify` mit «jar verified.» endet. Die Bauart `foss` geht nie in
den Store (F-33). Den Tag `app-v<versionName>` setzen: `docs/dev/ANDROID-INBETRIEBNAHME.md`,
Abschnitt 7; der Workflow baut dieselben Dateien und hängt sie an ein GitHub-Release.

## 4. Interne Testspur

**Was:** erste Verteilung an bis zu 100 eingetragene Google-Konten über einen Link (K27 11.3,
Schritt 2). Legt den Paketnamen fest. **Wo:** Play Console › Testen › Interner Test.

1. «Tester»: eine E-Mail-Liste anlegen (`<Liste der internen Tester>`), speichern.
2. «Neuen Release erstellen»: Bei der ersten Ausgabe fragt die Console nach der App-Signatur;
   «Von Google generierten Schlüssel verwenden» lassen. Das signierte Bundle hochladen, Release-Notizen
   aus `eintrag-de.md` («Was ist neu»), speichern, «Einführen».
3. Den Link «Tester einladen» kopieren und an die Tester schicken. Sie öffnen ihn auf dem Handy,
   nehmen teil und installieren aus dem Play Store.

**Geklappt, wenn:** der Release «Verfügbar für interne Tester» zeigt und ein Tester die App aus dem
Store installiert. Danach steht unter App-Integrität › App-Signatur der SHA-256 des
Signaturschlüssels von Google; die Bauten per ADB (`foss` oder eigene Signatur) lassen sich nicht
über eine Store-Installation spielen (K27 11.2). **Unbelegt:** ob das Hochladen schon geht, solange
die Identitätsprüfung des Kontos läuft (K27 2.7).

## 5. App-Inhalte und Store-Eintrag ausfüllen

**Was:** alle Angaben, die Google vor einem geschlossenen Test und vor der Produktion verlangt.
**Wo:** Play Console › Richtlinien › App-Inhalte, und Wachstum › Store-Präsenz › Haupteintrag.

| Formular | Quelle |
|---|---|
| Datenschutzerklärung: die HTTPS-Adresse `<Adresse der Datenschutzerklärung>` | `datenschutz.html` |
| App-Zugriff | `datensicherheit.md` Abschnitt 5 |
| Werbung, Zielgruppe, Inhaltseinstufung, Datensicherheit, weitere Erklärungen | `datensicherheit.md` Abschnitte 1, 3, 4, 6 |
| Vordergrunddienste | `datensicherheit.md` Abschnitt 2, mit den zwei Videos |
| Haupteintrag (Name, Beschreibungen, Grafiken, Kategorie, Kontakt) | `eintrag-de.md`, `listing-en.md` (als Übersetzung «Englisch – en-US» hinzufügen), `grafiken.md` |

**Die Datenschutzerklärung veröffentlichen:** `datenschutz.html` mit gefüllten Platzhaltern als
statische Seite unter HTTPS ablegen, zum Beispiel auf der Website des Projekts
(`<Adresse der Datenschutzerklärung>`). Die Seite braucht keine Skripte und lädt nichts nach. Die
Adresse kommt an drei Stellen hin: App-Inhalte › Datenschutzerklärung, Store-Eintrag (Kontakt) und,
falls die App darauf verweist, in die App.

**Geklappt, wenn:** das Dashboard unter «App einrichten» keinen offenen Punkt mehr zeigt.
**Unbelegt:** die Einordnung in «Datensicherheit», die Kategorie der Alterseinstufung und die
Wertung der Kinderprofile (Liste am Ende von `datensicherheit.md`).

## 6. Geschlossener Test: 12 Tester, 14 Tage am Stück

**Was:** die Testpflicht für private Konten, die nach dem 13. November 2023 angelegt wurden:
mindestens 12 Tester, die seit 14 Tagen ohne Unterbruch teilnehmen. Wer austritt und wieder
eintritt, beginnt neu (Hilfeseite «App testing requirements for new personal developer
accounts», <https://support.google.com/googleplay/android-developer/answer/14151465>, gelesen am
21. November 2026). **Wo:** Play Console › Testen › Geschlossener Test.

1. Einen Track anlegen («Geschlossener Test – Alpha»), Länder wählen (alle, in denen Tester
   wohnen).
2. Tester: eine E-Mail-Liste (`<Liste der geschlossenen Tester>`, mindestens 12, besser 15 als
   Reserve) oder eine Google-Gruppe.
3. Einen Release mit demselben Bundle wie in der internen Spur anlegen («Release übernehmen»),
   einführen. Google prüft den ersten Release eines geschlossenen Tests (kann Tage dauern).
4. Den Teilnahme-Link an die Tester schicken. Jeder muss auf «Tester werden» tippen **und** die App
   installieren und während der 14 Tage drin bleiben.
5. In den 14 Tagen: Rückmeldungen sammeln (wie, notieren: Google fragt danach), mindestens ein
   Update mit Verbesserungen hochladen (gibt Stoff für die Fragen in Schritt 7).

**Geklappt, wenn:** das Dashboard zeigt «12 Tester, 14 Tage» als erfüllt und bietet «Zugriff auf
Produktion beantragen» an. Die Frist beginnt neu, wenn die Zahl unter 12 fällt. **Unbelegt:** ob der
Test für die TV-Geräteart erneut anfällt (K26 2.6).

## 7. Antrag auf Produktion

**Was:** Google prüft, ob die App bereit ist; «in der Regel 7 Tage oder weniger». **Wo:** Dashboard ›
«Zugriff auf Produktion beantragen». Die Fragen in drei Teilen (Hilfeseite, sinngemäss) mit
Vorschlägen; die Einzelheiten aus dem eigenen Test einsetzen:

**Teil 1, zum geschlossenen Test**

| Frage | Vorschlag |
|---|---|
| Wie leicht war es, Tester zu finden? | ehrlich wählen; Familie und Freunde aus der internen Spur |
| Haben die Tester alle Funktionen genutzt? | Ja/teilweise: Stöbern, Abspielen, Downloads offline, Hörbücher; Fernseher per Scan und Push nur, soweit ihr Server sie hat. `<welche Funktionen nicht>` |
| Entsprach die Nutzung der erwarteten Nutzung? | Ja: Haushalte, die einen eigenen Slouchcat-Server betreiben oder zu einem eingeladen sind. Unterschied: `<Unterschiede>` |
| Rückmeldungen und wie gesammelt | `<Zusammenfassung>`, gesammelt über `<Weg, zum Beispiel E-Mail und GitHub-Issues>` |

**Teil 2, zur App**

| Frage | Vorschlag |
|---|---|
| Zielgruppe | Erwachsene, die einen eigenen Slouchcat-Medienserver betreiben, und Mitglieder ihres Haushalts mit einem Konto darauf |
| Welchen Wert hat die App? | Spielt die eigene Mediensammlung vom eigenen Server ab, auch offline, ohne Konto bei einem Anbieter, ohne Werbung und ohne Datenweitergabe; freie Software |
| Erwartete Installationen im ersten Jahr | `<Spanne>`; Vorschlag: die kleinste Stufe, die Console anbietet |

**Teil 3, zur Produktionsreife**

| Frage | Vorschlag |
|---|---|
| Was hast du nach dem Test geändert? | `<Änderungen aus den Rückmeldungen, mit Fassungen>` |
| Woran hast du erkannt, dass die App bereit ist? | Abnahme je Stufe auf echten Geräten (K27 Abschnitt 13), automatische Tests je Seite, keine offenen Absturzmeldungen aus dem Test, Rückmeldungen eingearbeitet |

**Geklappt, wenn:** Google den Zugriff gewährt (Mitteilung in der Console und per E-Mail).

## 8. Produktion

**Was:** öffentliche Ausgabe. **Wo:** Play Console › Produktion.

1. Länder wählen (Vorschlag: alle).
2. Release aus dem geschlossenen Test übernehmen, «Was ist neu» Deutsch und Englisch, einführen.
   Vorschlag: gestaffelt mit 20 %, nach einigen Tagen ohne Probleme 100 %.
3. **Ab jetzt gilt die Verträglichkeitsregel verbindlich** (`docs/dev/VERTRAEGLICHKEIT.md`): Tag
   und Fassung dieser App-Version notieren (`<Datum und versionName der ersten öffentlichen
   Ausgabe>`), weil die 30-Tage-Frist daran hängt.
4. Vorführserver laufen lassen: Jedes Update geht wieder durch die Prüfung.

**Geklappt, wenn:** der Eintrag unter `https://play.google.com/store/apps/details?id=com.slouchcat.app`
ohne Anmeldung als Tester erscheint.

## 9. TV-Geräteart (Konzept 26 Stufe 6)

**Was:** dieselbe App bekommt die Geräteart Android TV mit eigener Veröffentlichungsspur (E-20).
**Unumkehrbar:** Die Wahl der eigenen Spur lässt sich nach dem Speichern nicht mehr ändern (K27 2.7).
**Wo:** Play Console › Test und Release › Erweiterte Einstellungen › Gerätetypen (Form factors) ›
«Android TV hinzufügen».

1. Voraussetzung: Handy-App mindestens im geschlossenen Test, Vorführserver läuft, TV-App gebaut
   (`app-tv`, Ziel-API 34, `LEANBACK_LAUNCHER`, Banner).
2. Android TV hinzufügen, «Eigene Spur für Android TV» wählen, speichern.
3. Store-Eintrag: TV-Banner 1280 × 720 und mindestens ein TV-Bildschirmfoto (`grafiken.md`); den
   Absatz «Zusatz für die TV-Geräteart» aus `eintrag-de.md` und `listing-en.md` einsetzen.
4. Das TV-Bundle bauen und signieren wie in Schritt 3, mit `android/app-tv/version.properties` und
   `task android:dist APPS=tv` (Datei `slouchcat-tv-<versionName>-release.aab`, derselbe
   Upload-Schlüssel). Der `versionCode` der TV-App zählt ab 1000001, der der Handy-App bleibt unter
   1000000, damit sich die zwei nicht in die Quere kommen (`docs/dev/ANDROID-INBETRIEBNAHME.md`,
   Abschnitt 7). Zuerst in die interne TV-Spur, dann in die Produktion der TV-Spur.
   **Das TV-Bundle nur in die TV-Spur, das Handy-Bundle nie dorthin.** Google Play liefert je Gerät das
   passende Artefakt mit dem höchsten `versionCode`. Die TV-App verlangt darum
   `android.software.leanback` (`required="true"`: «Google Play will only make your app available on
   Android TV OS», developer.android.com, «Create and run a TV app», Abschnitt «Declare TV device
   support», gelesen am 23. November 2026); `scripts/android-dist.sh` prüft es in jedem Bau.
   **Ziel-API 34 reicht für die TV-App:** «Android TV and Android XR apps, which must target Android
   14 (API level 34) or higher» für neue Apps und Updates ab 31. August 2026
   (<https://developer.android.com/google/play/requirements/target-sdk>, gelesen am 23. November
   2026). Bestehende TV-Apps mit Ziel-API 33 oder tiefer sind auf neueren Geräten nicht mehr
   erhältlich; 34 liegt darüber.
5. Der Anleitung für die Prüfer (App-Zugriff) den TV-Absatz beilassen: Einrichten von Hand mit dem
   Prüfkonto (`datensicherheit.md` Abschnitt 5).
6. Die TV-Prüfung abwarten. Google prüft gegen die TV-Qualitätsliste; Ablehnungen kommen mit
   Hinweisen auf die Punkte.

**Prüfliste vor dem Einreichen** (TV-Qualitätsliste nach K26 2.6; die Punkte TV-PS und TV-G6 hat
der Rechercheur gelesen, nicht nachgeprüft):

- [ ] Eintrag für den TV-Startbildschirm (`LEANBACK_LAUNCHER` im Manifest)
- [ ] Banner 320 × 180 dp in der App mit dem App-Namen, und TV-Banner im Store
- [ ] alles mit der Fernbedienung bedienbar, Fokus immer sichtbar, keine Fokusfalle
- [ ] Zurück führt von jeder Seite bis zum Startbildschirm
- [ ] Taste Wiedergabe/Pause der Fernbedienung wirkt im Player
- [ ] kein Bildschirmschoner während der Wiedergabe
- [ ] keine Funktion, die Touch, Kamera oder GPS voraussetzt, als Pflicht im Manifest
- [ ] App-Bundle (AAB), Ziel-API mindestens 34, `minSdk` höchstens 31
- [ ] `android.software.leanback` verlangt (nur Fernseher), Touchscreen nicht verlangt (prüft `task android:dist`)
- [ ] 32 und 64 Bit sowie 16-KB-Speicherseiten unterstützt
- [ ] Test-Zugangsdaten für Googles Prüfer eingetragen (Vorführserver)

**Geklappt, wenn:** der Eintrag auf einem Android-TV-Gerät im Play Store erscheint. **Unbelegt:** ob
die Installation aus der internen Spur auf einem Fernseher klappt (K26 2.6); ob der Test mit 12
Testern für die TV-Geräteart erneut anfällt.

## Noch einzutragen

| Platzhalter | Schritt |
|---|---|
| `<Name des Entwicklers>` | 2 (Zertifikat) |
| `<Ort der Sicherung des Upload-Schlüssels>` | 2 |
| `<Liste der internen Tester>` | 4 |
| `<Adresse der Datenschutzerklärung>` | 5 |
| `<Liste der geschlossenen Tester>` | 6 |
| `<welche Funktionen nicht>`, `<Unterschiede>`, `<Zusammenfassung>`, `<Weg, …>`, `<Spanne>`, `<Änderungen aus den Rückmeldungen, mit Fassungen>` | 7 |
| `<Datum und versionName der ersten öffentlichen Ausgabe>` | 8 |

Dazu die Platzhalter der anderen Dateien (je am Ende der Datei).

## Unbelegt (Sammlung)

- ob die interne Testspur schon während der Identitätsprüfung geht (4);
- ob der Bau ohne `signingConfig` unsigniert oder debug-signiert ist (3);
- ob die E-Mail-Adresse eines privaten Entwicklers öffentlich ist (0);
- Einordnung der Daten an den eigenen Server in «Datensicherheit», Kategorie der Alterseinstufung,
  Wertung der Kinderprofile (5);
- Erklärung für den Übertragungsauftrag ab Android 14 und für Auto-Downloads (5);
- ob der geschlossene Test für die TV-Geräteart erneut anfällt und ob die interne Spur auf dem
  Fernseher installiert (6, 9);
- Grenze von 500 Zeichen für «Was ist neu» (aus Fremdquellen; im Formular sichtbar);
- Googles Wortlaut zu Apps mit eigenem Server des Nutzers (Zugang für Prüfer, 5).
