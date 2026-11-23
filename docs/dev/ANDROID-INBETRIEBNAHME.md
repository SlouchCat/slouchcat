# Android-Apps in Betrieb nehmen

Die eine Anleitung für die zwei Android-Apps: vom Bau der Dateien über das Einspielen auf Handy und
Fernseher bis zur signierten Ausgabe mit Tag. Sie wiederholt nicht, was anderswo steht, sondern
verweist darauf:

| Dokument | Inhalt |
|---|---|
| `docs/dev/SETUP.md`, Abschnitt «Die Android-Apps: `android/`» | JDK und Android-SDK ohne sudo, alle `task android:*` |
| `docs/relay/BETRIEB.md` | die zwei Dienste einrichten, überwachen, umziehen |
| `docs/store/VEROEFFENTLICHEN.md` | Play Console, Upload-Schlüssel, Testspuren, Produktion |
| `docs/perf/K27-android-handy.md` | Messungen und Gerätechecklisten der Handy-App |
| `docs/perf/K26-android-tv.md` | Messungen und Abnahmeliste der TV-App |

Befehle gelten im Repo (Wurzel), wenn nichts anderes dasteht. «Nicht geprüft» heisst: Der Schritt
braucht ein echtes Gerät, die Play Console, Firebase oder GitHub und lief beim Schreiben nicht.

## 1. Was vorliegt und was die Apps brauchen

### 1.1 Die Dateien

```bash
task android:dist                 # beide Apps
task android:dist APPS=app        # nur die Handy-App
task android:dist APPS=tv         # nur die TV-App
```

baut und legt nach `android/dist/` (nicht im Repo). `<v>` ist der `versionName` aus
`android/app-mobile/version.properties` (Handy) bzw. `android/app-tv/version.properties` (TV):

| Datei | Wofür |
|---|---|
| `slouchcat-app-<v>-play-debug.apk` | Handy, zum Einspielen mit `adb install` (Abschnitt 2); Bauart `play` mit Googles Code-Scanner und Push |
| `slouchcat-app-<v>-foss-debug.apk` | Handy ohne Google-Bibliotheken (Kamera über CameraX, kein Push); eigene Kennung, läuft neben `play` |
| `slouchcat-tv-<v>-debug.apk` | Fernseher, zum Einspielen mit `adb install` (Abschnitt 3) |
| `slouchcat-app-<v>-play-release[-unsigned].apk`, `…-foss-release[-unsigned].apk`, `slouchcat-tv-<v>-release[-unsigned].apk` | Release-Bau (R8); signiert nur mit den Werten aus Abschnitt 6 |
| `slouchcat-app-<v>-play-release[-unsigned].aab`, `slouchcat-tv-<v>-release[-unsigned].aab` | das Bundle für die Play Console; `foss` geht nie in den Store und hat keines |
| `SHA256SUMS` | Prüfsummen aller Dateien: `cd android/dist && sha256sum -c SHA256SUMS` |

`-unsigned` im Namen heisst: ohne Upload-Schlüssel gebaut. Ein unsigniertes APK lässt sich nicht
installieren. Das Skript (`scripts/android-dist.sh`) prüft jede Debug-APK mit `apksigner verify` und
`aapt2 dump badging`: Paketname, Version, Symbol, bei der TV-App dazu Starter für den
TV-Startbildschirm (`leanback-launchable-activity`) und Banner. Bricht eine Prüfung, gibt es keine
`SHA256SUMS`.

**Debug-Bauten tragen `.dev`** in der Kennung (`com.slouchcat.app.dev`, `foss`:
`com.slouchcat.app.foss.dev`) und sind mit dem Debug-Schlüssel des Rechners signiert, der sie gebaut
hat. Folgen:

- Eine Debug-App und die spätere App aus dem Store (`com.slouchcat.app`) sind zwei verschiedene Apps:
  Sie stehen nebeneinander, teilen keine Daten, und die eine wird nie zum Update der anderen. Die
  Debug-App löschst du von Hand, wenn du sie nicht mehr brauchst.
- Ein Debug-APK von einem anderen Rechner hat eine andere Signatur. Darüber spielt `adb install -r`
  nicht ein (`INSTALL_FAILED_UPDATE_INCOMPATIBLE`): erst deinstallieren
  (`adb uninstall com.slouchcat.app.dev`), dann einspielen. Anmeldung und Downloads sind dann weg.
- Ein Release-APK mit deinem Upload-Schlüssel heisst `com.slouchcat.app`. Die App aus dem Play Store
  trägt Googles Schlüssel (Play App Signing, K27 11.2): Darüber installiert der Store nicht. Vor dem
  Wechsel auf die Store-Ausgabe die selbst eingespielte deinstallieren.

Handy-APK (`play`, Debug) und TV-APK (Debug) haben dieselbe Kennung `com.slouchcat.app.dev`; sie
landen auf verschiedenen Geräten und stören sich nicht.

### 1.2 Was die Apps vom Server brauchen

- **Slouchcat 1.43.0 oder neuer.** Ab Vertrag 1.42.0 meldet die App sich an (App-Sitzungen, ADR 0040);
  ältere Server weist sie mit «Dieser Server … ist zu alt» ab (`Compatibility.MIN_CONTRACT` in
  `android/core/session/…/ServerProbe.kt`). Die TV-App meldet ab 1.43.0 Tonkanäle, Spurwahl und
  Untertitelformate (ADR 0041); ein Server 1.42.0 wählt Ton und Untertitel dann wie für einen
  Browser.
- **Erreichbar unter einer Adresse, die das Gerät kennt.** Dieselbe Adresse wie im Browser. HTTPS
  oder HTTP: Die Apps erlauben unverschlüsseltes HTTP überall (E-18), warnen aber ausserhalb des
  Heimnetzes vor dem Verbinden («Unverschlüsselte Verbindung»). Für das Einrichten des Fernsehers per
  Scan muss die Adresse, die das Handy benutzt, auch vom Fernseher aus erreichbar sein (K27
  Abschnitt 10); eine VPN-Adresse nur des Handys reicht nicht.
- **Ein Konto mit E-Mail und Passwort** am Server; Profile und PIN wie in der Web-App.
- Für den Scan und Push zusätzlich die zwei Dienste (Abschnitt 4) und für Push ein Firebase-Projekt
  (Abschnitt 5).

## 2. Handy-App aufs Handy

Nicht geprüft auf einem echten Handy; die Abläufe selbst laufen im Emulator (`task android:e2e`).

1. **Entwickleroptionen:** Einstellungen › Über das Telefon › siebenmal auf «Build-Nummer» tippen.
   Die Namen der Menüs unterscheiden sich je Hersteller.
2. **USB-Debugging:** Einstellungen › System › Entwickleroptionen › «USB-Debugging» an. Handy per USB
   an den Rechner, auf dem Handy «USB-Debugging zulassen» für diesen Rechner bestätigen.
3. **Gerät sehen:** `adb devices` (nach `source …/android-env.sh` oder mit `platform-tools` im
   `PATH`) zeigt das Gerät mit `device`, nicht `unauthorized`.
4. **Einspielen:**

   ```bash
   adb install -r android/dist/slouchcat-app-1.0.0-play-debug.apk
   ```

   `-r` ersetzt eine frühere Fassung derselben Kennung und Signatur. Ohne Google-Play-Dienste auf dem
   Handy die `foss`-Datei nehmen.
5. **Erster Start:** Die App heisst «Slouchcat» (Debug-Bau ebenso). «Mit deinem Server verbinden»:
   Adresse eingeben, «Verbinden». Die App zeigt Adresse, Version und ob verschlüsselt; «Weiter».
6. **Anmelden** mit E-Mail und Passwort des Kontos. Das Handy erscheint danach unter «Meine Geräte»
   (Web-App und App).
7. **Profil wählen**, bei einem Profil mit PIN die PIN eingeben.

**Was zu prüfen ist** (je Liste die Punkte abarbeiten, Ergebnis im Bericht ergänzen):

- `docs/perf/K27-android-handy.md`, Abschnitt «Auf einem echten Gerät oder mit dem Firebase-Projekt
  noch zu messen» (Wiedergabe, Downloads, Auto-Downloads, Hersteller-Eigenheiten).
- ebenda, «Stufe 6: Downloads und Offline», **Gerätecheckliste Stufe 6**.
- ebenda, «Stufe 7: Ton», **Gerätecheckliste Stufe 7**.
- ebenda, «Stufe 8: Wünsche und Meldungen», Unterabschnitt «Auf einem echten Gerät noch zu messen
  (braucht das Firebase-Projekt)».
- ebenda, «Stufe 9: Fernseher verbinden», **Gerätecheckliste Stufe 9** (mit Fernseher, Abschnitt 3).

## 3. TV-App auf Chromecast, Shield oder Fernseher

Nicht geprüft auf einem echten Fernseher; die Abläufe laufen im TV-Emulator (`task android:e2e:tv`,
`task android:e2e:pair`).

1. **Entwickleroptionen:** Einstellungen › System › Info › siebenmal auf «Build» bzw. «Android TV
   OS-Build» tippen. Bei der Nvidia Shield: Einstellungen › Geräteeinstellungen › Info › «Build».
2. **Debugging übers Netz:** Einstellungen › System › Entwickleroptionen › «USB-Debugging» an (bei
   Geräten ohne USB gilt es fürs Netz), wo es angeboten wird zusätzlich «Netzwerk-Debugging». Die
   Adresse des Geräts steht unter Einstellungen › Netzwerk.
3. **Verbinden:**

   ```bash
   adb connect <Adresse des Geräts>:5555
   adb devices
   ```

   Am Fernseher die Frage «USB-Debugging zulassen?» bestätigen; danach zeigt `adb devices` das Gerät
   mit `device`. Neuere Geräte mit Google TV bieten statt Port 5555 «Kabelloses Debugging» mit
   Kopplungscode: dort `adb pair <Adresse>:<Port der Kopplung>` mit dem angezeigten Code, dann
   `adb connect <Adresse>:<Port>` mit dem Port, den das Gerät nennt.
4. **Einspielen:**

   ```bash
   adb -s <Adresse des Geräts>:5555 install -r android/dist/slouchcat-tv-1.0.0-debug.apk
   ```

   Die App erscheint auf dem Startbildschirm mit dem Banner «Slouchcat» (bei Google TV unter «Apps»).
5. **Einrichten von Hand:** «Von Hand einrichten»: Serveradresse, E-Mail und Passwort mit der
   Fernbedienung, dann Profil wählen.
6. **Einrichten per Scan:** «Mit dem Handy einrichten». Am Handy (Hauptprofil) «Mehr › Fernseher
   verbinden», Code scannen, «Verbinden». Braucht die Vermittlung (Abschnitt 4): Handy und Fernseher
   müssen dieselbe Vermittlung eingestellt haben. Ablauf und Fehlerfälle: Gerätecheckliste Stufe 9 in
   `docs/perf/K27-android-handy.md`.
7. Debugging übers Netz nach dem Einspielen wieder ausschalten: Wer im selben Netz ist, kann sich
   sonst verbinden (bestätigen muss er am Fernseher trotzdem).

**Was zu prüfen ist:**

- `docs/perf/K26-android-tv.md`, Abschnitt «Stufe 3: Videoplayer der TV-App», **Abnahmeliste je Gerät
  und Tonanlage** (Nr. 1 bis 23).
- ebenda, «Auf den drei Geräten nachzumessen» (die Messungen der Wegwerf-App aus Stufe 0; nur, wo die
  Abnahmeliste eine Frage offenlässt).
- `docs/store/VEROEFFENTLICHEN.md`, Abschnitt 9, «Prüfliste vor dem Einreichen» (TV-Qualitätsliste).

## 4. Die zwei Dienste: Vermittlung und Push-Weiterleitung

Zwei kleine Dienste im Internet, getrennt voneinander auf je einer eigenen Maschine (E-21, ADR 0042).
Sie halten keine Konten und keine Inhalte und sehen Pakete und Meldungen nur verschlüsselt (Regel 11,
Ausnahme «Cloud»):

| Dienst | Tut | Ohne ihn |
|---|---|---|
| **Vermittlung** (`relay/cmd/pair`) | Treffpunkt für «Fernseher per Scan einrichten»: Der Fernseher zeigt einen Code, das Handy legt ein versiegeltes Paket ab, der Fernseher holt es (höchstens fünf Minuten im Arbeitsspeicher) | kein Einrichten per Scan; von Hand geht immer |
| **Push-Weiterleitung** (`relay/cmd/push`) | nimmt die verschlüsselte Web-Push-Meldung des Servers an und reicht sie über Firebase Cloud Messaging ans Handy | keine Push-Meldungen; Neues steht weiter unter «Benachrichtigungen», per E-Mail oder Telegram |

Einrichten, Umgebungsvariablen, Geheimnisse, Überwachung und Abnahme: `docs/relay/BETRIEB.md`
(Abschnitte 1 bis 9). Die Images kommen mit einem Tag `relay-vX.Y.Z` (Workflow `release-relay.yml`).

**Die Adressen, die die Apps eingebaut haben:**

| Dienst | Vorgabe | Wo festgelegt | Ändern |
|---|---|---|---|
| Vermittlung | `https://pair.slouchcat.com` | `RelaySettings.DEFAULT` (`android/core/session/…/RelaySettings.kt`), `PairingProtocol.DEFAULT_RELAY` (`android/core/pairing/…/Pairing.kt`) | im Gerät (F-21): am Handy «Mehr › Adresse der Vermittlung» (nur Hauptprofil), am Fernseher «Mit dem Handy einrichten › Vermittlung ändern»; Handy und Fernseher brauchen dieselbe; nur `https` (Debug-Bauten auch `http`) |
| Push-Weiterleitung | `https://push.slouchcat.com` | Gradle-Eigenschaft `slouchcat.pushRelay` oder Umgebungsvariable `SLOUCHCAT_PUSH_RELAY` beim Bau (`android/app-mobile/build.gradle.kts`, `BuildConfig.PUSH_RELAY`) | nur beim Bau; im Gerät nicht änderbar (F-21). Wer ein eigenes Firebase-Projekt hat, braucht seine eigene Weiterleitung |

Die zwei Namen müssen auf die laufenden Dienste zeigen, bevor Scan und Push gehen (DNS und TLS nach
`docs/relay/BETRIEB.md`). Nicht geprüft gegen die öffentlichen Namen; geprüft ist der Weg gegen lokale
Dienste (`task android:e2e:pair`, `task android:contract`).

## 5. Push mit Firebase

Nicht geprüft: Es gibt noch kein Firebase-Projekt. Ohne die vier Werte baut `play` ohne Push und die
App sagt unter «Mehr › Benachrichtigungen» bei «Push-Meldungen»: «Diese Ausgabe der App hat keine
Push-Meldungen».

1. In der Firebase-Konsole (Google-Konto des Projekts) ein Projekt anlegen, Google Analytics aus.
2. Darin eine Android-App mit dem Paketnamen `com.slouchcat.app` anlegen. Die angebotene
   `google-services.json` herunterladen, aber **nicht** ins Repo legen. Aus ihr kommen die vier
   Werte:

   | Wert | in `google-services.json` | Gradle-Eigenschaft | Umgebungsvariable |
   |---|---|---|---|
   | Projekt-ID | `project_info.project_id` | `slouchcat.firebase.projectId` | `SLOUCHCAT_FIREBASE_PROJECT_ID` |
   | App-ID | `client[].client_info.mobilesdk_app_id` | `slouchcat.firebase.appId` | `SLOUCHCAT_FIREBASE_APP_ID` |
   | API-Schlüssel | `client[].api_key[].current_key` | `slouchcat.firebase.apiKey` | `SLOUCHCAT_FIREBASE_API_KEY` |
   | Sender-ID | `project_info.project_number` | `slouchcat.firebase.senderId` | `SLOUCHCAT_FIREBASE_SENDER_ID` |

3. Die vier Werte in `~/.gradle/gradle.properties` eintragen (nicht ins Repo) oder als
   Umgebungsvariablen beim Bau setzen, in GitHub als Secrets gleichen Namens (Abschnitt 7). Dann
   `task android:dist`.
4. **Push im Debug-Bau:** Die App-ID gehört zu einem Paketnamen. Ein Debug-Bau heisst
   `com.slouchcat.app.dev`; für Push dort im selben Projekt eine zweite Android-App
   `com.slouchcat.app.dev` anlegen und einen Debug-Bau mit deren App-ID bauen. `task android:dist`
   baut Debug und Release mit denselben vier Werten; mit der App-ID von `com.slouchcat.app` ist Push nur
   im Release-Bau zu erwarten (nicht geprüft).
5. Im Firebase-Projekt unter Projekteinstellungen › Dienstkonten einen privaten Schlüssel erzeugen.
   Diese Datei gehört nur zur Push-Weiterleitung (`RELAY_FCM_CREDENTIALS`, `docs/relay/BETRIEB.md`
   Abschnitt 3), nie in die App und nie ins Repo.
6. **Prüfen:** App einspielen, anmelden, «Mehr › Benachrichtigungen»: «Push-Meldungen» an (Android
   fragt nach der Erlaubnis für Mitteilungen). Mit dem Hauptprofil den Kanal «Push» einschalten und
   im Blatt «Testmeldung senden». Dann die App wegwischen und eine Testmeldung aus der Web-App senden
   (Einstellungen › Benachrichtigungskanäle): Sie kommt auch bei geschlossener App. Doze:
   `adb shell dumpsys deviceidle force-idle`, Testmeldung, `adb shell dumpsys deviceidle unforce`.
   Kommt nichts: Log der Weiterleitung (`docker compose logs push`, 201 heisst an Google übergeben)
   und der Hinweis in der App unter «Push-Meldungen».
7. In der Firebase-Konsole ist für den Datenschutz nichts zu schalten. Das Manifest von `play`
   (`android/app-mobile/src/play/AndroidManifest.xml`) setzt
   `firebase_messaging_auto_init_enabled=false`, `firebase_analytics_collection_deactivated=true`
   und `delivery_metrics_exported_to_big_query_enabled=false`. Die App holt ein Token erst, wenn Push
   auf dem Gerät gewünscht ist, und löscht es beim Ausschalten.

## 6. Signieren

Der Upload-Schlüssel liegt nie im Repo (K27 11.2). Google hält den Schlüssel, mit dem der Store
ausliefert; du signierst nur, was du hochlädst.

### 6.1 Upload-Schlüssel erzeugen

Befehl und Ablage: `docs/store/VEROEFFENTLICHEN.md`, Abschnitt 2 (`keytool -genkeypair …` nach
`~/.config/slouchcat/upload.jks`, Alias `upload`). Ein Schlüssel für beide Apps.

### 6.2 Mit dem Bau signieren

Die Release-Bauten beider Apps werden signiert, sobald alle vier Werte gesetzt sind
(`android/build-logic/…/Signing.kt`); als Gradle-Eigenschaften in `~/.gradle/gradle.properties`
(Datei `chmod 600`):

```properties
slouchcat.signing.storeFile=/home/<Benutzer>/.config/slouchcat/upload.jks
slouchcat.signing.storePassword=<Passwort des Keystores>
slouchcat.signing.keyAlias=upload
slouchcat.signing.keyPassword=<Passwort des Schlüssels>
```

oder als Umgebungsvariablen `SLOUCHCAT_SIGNING_STORE_FILE`, `SLOUCHCAT_SIGNING_STORE_PASSWORD`,
`SLOUCHCAT_SIGNING_KEY_ALIAS`, `SLOUCHCAT_SIGNING_KEY_PASSWORD`. Einen relativen Pfad liest Gradle ab
`android/`; besser einen absoluten nehmen. Dann:

```bash
task android:dist
```

Die Release-Dateien heissen dann ohne `-unsigned`. Fehlt einer der vier Werte, bricht der Bau ab und
nennt die fehlenden Namen (nie die Werte); fehlen alle vier, bleibt `release` unsigniert. Im
PKCS12-Keystore aus `keytool` gilt ein Passwort für Keystore und Schlüssel: beide Passwort-Werte
gleich setzen.

### 6.3 Von Hand signieren

Für eine unsignierte Datei aus `android/dist/` oder aus einem GitHub-Release. Die Build-Tools liegen
unter `~/.local/share/android-sdk/build-tools/<Version>` (heute `37.0.0`):

```bash
BT=~/.local/share/android-sdk/build-tools/37.0.0
cd android/dist
"$BT/zipalign" -P 16 -f 4 slouchcat-app-1.0.0-play-release-unsigned.apk aligned.apk
"$BT/apksigner" sign --ks ~/.config/slouchcat/upload.jks --ks-key-alias upload \
  --out slouchcat-app-1.0.0-play-release.apk aligned.apk
"$BT/apksigner" verify --print-certs slouchcat-app-1.0.0-play-release.apk
rm aligned.apk
```

`zipalign` vor `apksigner`, nie danach (die Signatur deckt die Ausrichtung). `-P 16` richtet
eingebettete Bibliotheken auf 16-KB-Seiten aus. Das Bundle signiert `jarsigner` aus dem JDK:

```bash
jarsigner -keystore ~/.config/slouchcat/upload.jks \
  -signedjar slouchcat-app-1.0.0-play-release.aab \
  slouchcat-app-1.0.0-play-release-unsigned.aab upload
jarsigner -verify slouchcat-app-1.0.0-play-release.aab
```

`jarsigner -verify` meldet «jar verified.» (deutsch: «JAR-Datei verifiziert.»). Die Warnungen
danach (selbstsigniertes Zertifikat, kein Zeitstempel) sind bei einem Upload-Schlüssel normal.
Geprüft mit einem Wegwerf-Schlüssel: `apksigner verify` nennt den Signierer, `zipalign -c -P 16 4`
bestätigt die Ausrichtung. Hochladen in die Play Console:
`docs/store/VEROEFFENTLICHEN.md` ab Abschnitt 3.

## 7. Ausgabe taggen

Jede App hat ihre Versionsdatei und ihren Tag:

| App | Datei | Tag |
|---|---|---|
| Handy | `android/app-mobile/version.properties` | `app-vX.Y.Z` |
| Fernseher | `android/app-tv/version.properties` | `tv-vX.Y.Z` |

**Versionsregel:** `versionName` ist die Version des Tags. `versionCode` steigt mit jeder Ausgabe um 1
und sinkt nie: Die Handy-App zählt ab 1 und bleibt unter 1000000, die TV-App zählt ab 1000001. Beide
Apps sind ein Eintrag im Store mit derselben Kennung, und jedes hochgeladene Bundle braucht einen
eigenen `versionCode`. Die Bauskripte brechen ab, wenn ein Code den Bereich verlässt. Erste Ausgabe:
Handy `1.0.0`/`1`, Fernseher `1.0.0`/`1000001`.

Ablauf (nicht geprüft, der Workflow lief noch nie):

1. `versionCode` und `versionName` in der Datei der App erhöhen, committen, über einen Pull Request nach
   `main`.
2. Auf dem Commit in `main` taggen und den Tag pushen:

   ```bash
   git tag -a app-v1.0.0 -m "Slouchcat phone app 1.0.0"
   git push origin app-v1.0.0
   ```

3. Der Workflow `.github/workflows/release-android.yml` prüft den Tag gegen die Datei
   (`scripts/android-tag-check.sh`), fährt `task android:check` und `task android:test` ohne Secrets,
   baut dann mit `task android:dist APPS=app` bzw. `APPS=tv` und hängt APKs, Bundle und `SHA256SUMS`
   an ein GitHub-Release des Tags. Dieses Release wird **nie «latest»**: Der Server und sein Updater
   lesen das neueste Release (`releases/latest/download/manifest.json`); ein App-Release darf das
   Server-Release dort nicht verdrängen. Die Workflows des Servers (`v*.*.*`) und der Dienste
   (`relay-v*.*.*`) laufen bei App-Tags nicht.

**Secrets** (GitHub › Settings › Secrets and variables › Actions). Ohne sie baut der Workflow
unsigniert und ohne Push und schreibt das in den Text des Releases:

| Secret | Inhalt |
|---|---|
| `SLOUCHCAT_SIGNING_KEYSTORE_BASE64` | der Keystore als Base64: `base64 -w0 ~/.config/slouchcat/upload.jks` |
| `SLOUCHCAT_SIGNING_STORE_PASSWORD`, `SLOUCHCAT_SIGNING_KEY_ALIAS`, `SLOUCHCAT_SIGNING_KEY_PASSWORD` | wie in 6.2 |
| `SLOUCHCAT_FIREBASE_PROJECT_ID`, `SLOUCHCAT_FIREBASE_APP_ID`, `SLOUCHCAT_FIREBASE_API_KEY`, `SLOUCHCAT_FIREBASE_SENDER_ID` | wie in Abschnitt 5 (nur für die Handy-App) |

Wer den Upload-Schlüssel nicht bei GitHub ablegen will, lässt die vier Signatur-Secrets weg und
signiert die Dateien des Releases von Hand (6.3). Eine Tag-Schutzregel für `app-v*` und `tv-v*`
verhindert, dass andere den Workflow auslösen.

## 8. Nur im Emulator geprüft, nur auf echten Geräten möglich

Stand dieser Anleitung; der Koordinator ergänzt ihn am Schluss des Baus.

**Im Emulator oder auf dem Rechner geprüft:**

- Beide Apps: Einrichten, Anmelden, Profil mit PIN, Abmelden (`task android:e2e`, `task android:e2e:tv`);
  Koppeln von Handy und Fernseher über eine lokale Vermittlung, je eine Seite mit der echten App
  (`docs/perf/K27-android-handy.md`, Stufe 9).
- Krypto auf API 28 und 36, Download-Wiedergabe über die lokale HLS-Liste, Abruf im Hintergrund mit
  gebremstem Server auf dem Rechner (`docs/perf/K27-android-handy.md`, «Nur im Emulator geprüft»).
- Push nur mit der Attrappe `fcmfake` statt Google (`docs/perf/K27-android-handy.md`, Stufe 8).
- Ton der TV-App: Erkennung, Kanalzahl, Abwählen ohne Decoder; nur Software-Decoder, nur PCM Stereo
  (`docs/perf/K26-android-tv.md`, «Nur im Emulator geprüft»).
- Die Dateien aus `task android:dist`: Signatur der Debug-APKs (`apksigner verify`) und `aapt2 dump
  badging` (Paket, Version, Symbol, TV-Starter, Banner).

**Nur auf echten Geräten oder mit fremden Diensten:**

- Wiedergabe von HEVC 10 Bit, HDR, Dolby Vision, Dolby- und DTS-Ton, Durchreichen an eine Tonanlage
  (beide Berichte).
- Downloads über Nacht mit UIDT, Auto-Downloads mit Akku-Optimierung des Herstellers, Wegwischen bei
  Herstellern, die dabei den Prozess beenden (`docs/perf/K27-android-handy.md`).
- Push über Google: geschlossene App, Doze, Token-Wechsel (Firebase-Projekt, Abschnitt 5).
- Googles Code-Scanner (`play`) mit echter Kamera; Scan über die öffentliche Vermittlung.
- Tempo von «Compose for TV» auf dem schwächsten Gerät, Bildschirmschoner, Fernbedienung
  (`docs/perf/K26-android-tv.md`, Abnahmeliste).
- Installation aus der internen Testspur auf einem Fernseher (K26 8.1, unbelegt).
- Signieren mit dem echten Upload-Schlüssel, der Workflow `release-android.yml` auf GitHub.

## 9. Das muss Marco tun

| Nr. | Was | Abschnitt |
|---|---|---|
| 1 | Server auf 1.43.0 oder neuer heben | 1.2 |
| 2 | `task android:dist` laufen lassen (oder die Dateien eines GitHub-Releases nehmen) | 1.1, 7 |
| 3 | Am Handy Entwickleroptionen und USB-Debugging einschalten, Handy-App einspielen, einrichten | 2 |
| 4 | Gerätechecklisten der Handy-App abarbeiten | 2 |
| 5 | An jedem Fernseher Entwickleroptionen und Debugging einschalten, TV-App einspielen, von Hand und per Scan einrichten | 3 |
| 6 | Abnahmeliste der TV-App je Gerät und Tonanlage abarbeiten | 3 |
| 7 | Vermittlung und Push-Weiterleitung einrichten, `pair.slouchcat.com` und `push.slouchcat.com` darauf zeigen lassen | 4 |
| 8 | Firebase-Projekt anlegen, die vier Werte setzen, den Dienstkonto-Schlüssel der Push-Weiterleitung geben | 5 |
| 9 | Upload-Schlüssel erzeugen und sichern | 6.1 |
| 10 | Die vier Signatur-Werte setzen oder von Hand signieren | 6.2, 6.3 |
| 11 | Secrets bei GitHub eintragen (oder bewusst weglassen) und Tag-Schutz für `app-v*` und `tv-v*` | 7 |
| 12 | Version erhöhen, taggen, Release prüfen | 7 |
| 13 | Bundle in die Play Console laden | 6.3, `docs/store/VEROEFFENTLICHEN.md` |
