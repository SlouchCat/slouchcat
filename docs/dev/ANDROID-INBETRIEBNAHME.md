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
installieren; `task android:sign` signiert es nachträglich (Abschnitt 6.3). Das Skript
(`scripts/android-dist.sh`) prüft jede Debug-APK mit `apksigner verify` und `aapt2 dump badging`:
Paketname, Version, Symbol, bei der TV-App dazu Starter für den TV-Startbildschirm
(`leanback-launchable-activity`) und Banner. Bei den Release-APKs prüft es die Gerätefilter von
Google Play: Die TV-App verlangt `android.software.leanback` und keinen Touchscreen (sie erreicht nur
Fernseher), die Handy-App verlangt `leanback` nicht. Bricht eine Prüfung, gibt es keine `SHA256SUMS`.
Ein GitHub-Release (Abschnitt 7) enthält nur die Release-Dateien, keine Debug-APKs.

**Für echte Geräte das signierte Release-APK nehmen** (`…-release.apk`, Abschnitt 6; den
Upload-Schlüssel zu erzeugen dauert wenige Minuten). Es heisst `com.slouchcat.app` und verhält sich wie
die spätere App. Die App aus dem Play Store trägt aber Googles Schlüssel (Play App Signing, K27 11.2):
Über eine selbst eingespielte App installiert der Store nicht. Vor dem Wechsel auf die Store-Ausgabe
die selbst eingespielte deinstallieren; Anmeldung und Downloads sind dann weg.

**Der schnelle Weg mit dem Debug-APK** geht ohne Schlüssel, hat aber Folgen:

- **Debuggbar:** Wer das Gerät per USB oder Netz-Debugging erreicht, liest mit `adb shell run-as
  com.slouchcat.app.dev` die Daten der App, auch die gespeicherte Anmeldung. Nur auf eigenen Geräten,
  Debugging danach wieder aus (Abschnitte 2 und 3).
- **Anderer Paketname:** `com.slouchcat.app.dev` (`foss`: `com.slouchcat.app.foss.dev`). Debug-App und
  Store-App stehen nebeneinander, teilen keine Daten, und die eine wird nie zum Update der anderen.
- **Andere Signatur je Rechner:** Ein Debug-APK ist mit dem Debug-Schlüssel des Rechners signiert, der
  es gebaut hat. Über ein Debug-APK eines anderen Rechners spielt `adb install -r` nicht ein
  (`INSTALL_FAILED_UPDATE_INCOMPATIBLE`): erst `adb uninstall com.slouchcat.app.dev`.
- **Push** braucht im Debug-Bau die Firebase-App-ID von `com.slouchcat.app.dev` (Abschnitt 5).

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
3. **Gerät sehen:** `adb` liegt in den Platform-Tools des Android-SDK
   (`scripts/install-android-tools.sh` legt sie nach `~/.local/share/android-sdk/platform-tools`):

   ```bash
   export PATH="$HOME/.local/share/android-sdk/platform-tools:$PATH"
   adb devices
   ```

   Das Gerät steht mit `device` da, nicht `unauthorized`.
4. **Einspielen** (empfohlen das signierte Release-APK, Abschnitt 1.1):

   ```bash
   adb install -r android/dist/slouchcat-app-1.0.0-play-release.apk
   ```

   Ohne Schlüssel der schnelle Weg mit `slouchcat-app-1.0.0-play-debug.apk` (Folgen in 1.1). `-r`
   ersetzt eine frühere Fassung derselben Kennung und Signatur. Ohne Google-Play-Dienste auf dem Handy
   die `foss`-Datei nehmen.
5. **USB-Debugging wieder aus:** Einstellungen › System › Entwickleroptionen › «USB-Debugging» aus
   (oder die Entwickleroptionen ganz aus). Für das nächste Einspielen wieder ein.
6. **Erster Start:** Die App heisst «Slouchcat» (Debug-Bau ebenso). «Mit deinem Server verbinden»:
   Adresse eingeben, «Verbinden». Die App zeigt Adresse, Version und ob verschlüsselt; «Weiter».
7. **Anmelden** mit E-Mail und Passwort des Kontos. Das Handy erscheint danach unter «Meine Geräte»
   (Web-App und App).
8. **Profil wählen**, bei einem Profil mit PIN die PIN eingeben.

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

   `adb` wie in Abschnitt 2, Schritt 3, im `PATH`. Am Fernseher die Frage «USB-Debugging zulassen?»
   bestätigen; danach zeigt `adb devices` das Gerät mit `device`. **Vorbehalt:** Ob Port 5555 so offen
   ist, hängt vom Gerät ab (nicht geprüft). Neuere Geräte mit Google TV bieten statt Port 5555
   «Kabelloses Debugging» mit Kopplungscode: dort `adb pair <Adresse>:<Port der Kopplung>` mit dem
   angezeigten Code, dann `adb connect <Adresse>:<Port>` mit dem Port, den das Gerät nennt. Nur im
   eigenen Netz, nie über ein fremdes WLAN.
4. **Einspielen** (empfohlen das signierte Release-APK, Abschnitt 1.1; ohne Schlüssel
   `slouchcat-tv-1.0.0-debug.apk`):

   ```bash
   adb -s <Adresse des Geräts>:5555 install -r android/dist/slouchcat-tv-1.0.0-release.apk
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
   Umgebungsvariablen beim Bau setzen, in GitHub als Variablen des Repos gleichen Namens
   (Abschnitt 7.1). Alle vier oder keiner. Dann
   `task android:dist`.
4. **Push im Debug-Bau:** Die App-ID gehört zu einem Paketnamen. Ein Debug-Bau heisst
   `com.slouchcat.app.dev`; für Push dort im selben Projekt eine zweite Android-App
   `com.slouchcat.app.dev` anlegen und einen Debug-Bau mit deren App-ID bauen. `task android:dist`
   baut Debug und Release mit denselben vier Werten; mit der App-ID von `com.slouchcat.app` ist Push nur
   im Release-Bau zu erwarten (nicht geprüft).
5. Im Firebase-Projekt unter Projekteinstellungen › Dienstkonten einen privaten Schlüssel erzeugen.
   Diese Datei gehört nur zur Push-Weiterleitung: Auf deren Maschine als `secrets/firebase.json` in
   den Ordner `RELAY_SECRETS_DIR` legen (Vorgabe `./secrets` neben der Compose-Datei, Besitzer
   `65532:65532`, Rechte `0400`; `docs/relay/BETRIEB.md` Abschnitte 2 und 3). Die Compose-Datei bindet
   den Ordner schreibgeschützt als `/run/secrets` ein, der Dienst liest `/run/secrets/firebase.json`.
   Nie in die App und nie ins Repo.
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
gleich setzen. Sind Signaturwerte gesetzt, ruft `task android:dist` Gradle mit
`--no-configuration-cache` auf: So landet nichts vom Schlüssel im Konfigurations-Cache unter
`android/.gradle/`. Wer Gradle selbst aufruft, gibt die Option für signierte Bauten selbst mit.

### 6.3 Nachträglich signieren

Für unsignierte Dateien aus `android/dist/` oder aus einem GitHub-Release. Dasselbe Skript nutzt der
Job «sign» des Workflows (Abschnitt 7): ohne Gradle, nur `zipalign` und `apksigner` aus den
Build-Tools und `jarsigner` aus dem JDK. Die Passwörter kommen aus der Umgebung, nie als Argument
(sie stünden sonst in der Prozessliste):

```bash
export JAVA_HOME=~/.local/share/jdk/current ANDROID_HOME=~/.local/share/android-sdk
export SLOUCHCAT_SIGNING_STORE_FILE=~/.config/slouchcat/upload.jks SLOUCHCAT_SIGNING_KEY_ALIAS=upload
read -rs -p "Passwort: " SLOUCHCAT_SIGNING_STORE_PASSWORD; echo
export SLOUCHCAT_SIGNING_STORE_PASSWORD SLOUCHCAT_SIGNING_KEY_PASSWORD="$SLOUCHCAT_SIGNING_STORE_PASSWORD"
task android:sign                    # Ordner android/dist/
task android:sign DIR=/pfad/zum/ordner   # anderer Ordner (relativ zu android/)
```

Das Skript (`scripts/android-sign.sh`) macht je `*-release-unsigned.apk` daraus ein `*-release.apk`:
`zipalign -P 16 -f 4` (eingebettete Bibliotheken auf 16-KB-Seiten), `apksigner sign` mit
`--ks-pass env:…`, `apksigner verify` und `zipalign -c -P 16 4`. Je `*-release-unsigned.aab` signiert
es mit `jarsigner -storepass:env … -keypass:env …`, prüft mit `jarsigner -verify` und dem
Signaturblock unter `META-INF/`. Die unsignierte Datei geht weg, `SHA256SUMS` entsteht neu. Fehlt
einer der vier Werte, bricht es ab und nennt die fehlenden Namen. Geprüft mit einem
Wegwerf-Schlüssel. `zipalign` kommt vor `apksigner`, nie danach: Die Signatur deckt die Ausrichtung.
`jarsigner -verify` von Hand meldet «jar verified.» (deutsch: «JAR-Datei verifiziert.»); die
Warnungen danach (selbstsigniertes Zertifikat, kein Zeitstempel) sind bei einem Upload-Schlüssel
normal. Hochladen in die Play Console: `docs/store/VEROEFFENTLICHEN.md` ab Abschnitt 3.

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

**Das TV-Bundle gehört nur in die Spur «Android TV».** Die TV-App verlangt `android.software.leanback`
und erreicht darum nur Fernseher (`scripts/android-dist.sh` prüft das in jedem Bau). Trotzdem nie das
TV-Bundle in eine Spur des Handys laden und nie das Handy-Bundle in die TV-Spur: Google Play liefert je
Gerät das Artefakt mit dem höchsten `versionCode`, das zum Gerät passt.

### 7.1 Einmal in GitHub einrichten

Nicht geprüft (ohne Zugang zum Repo bei GitHub geschrieben); die Menünamen können sich ändern.

1. **Environment `android-release`:** Repo › Settings › Environments › «New environment», Name
   `android-release`. Darin:
   - «Required reviewers»: dich selbst eintragen. Jeder Signierlauf wartet dann auf deine Freigabe
     unter Actions › der Lauf › «Review deployments».
   - «Deployment branches and tags» › «Selected branches and tags» › «Add deployment branch or tag
     rule» › Typ «Tag», Muster `app-v*`; ein zweites Mal mit `tv-v*`.
2. **Secrets des Environments** (nur der Job «sign» sieht sie). Im Terminal, ohne den Keystore oder ein
   Passwort anzuzeigen (`gh secret set` liest den Wert aus der Pipe bzw. fragt verdeckt):

   ```bash
   base64 -w0 ~/.config/slouchcat/upload.jks | gh secret set SLOUCHCAT_SIGNING_KEYSTORE_BASE64 --env android-release
   gh secret set SLOUCHCAT_SIGNING_STORE_PASSWORD --env android-release
   gh secret set SLOUCHCAT_SIGNING_KEY_PASSWORD --env android-release
   printf upload | gh secret set SLOUCHCAT_SIGNING_KEY_ALIAS --env android-release
   ```

   Ohne diese vier baut der Workflow unsigniert und schreibt das in den Text des Releases; mit einem
   bis drei davon bricht der Job «sign» ab. Wer den Upload-Schlüssel nicht bei GitHub ablegen will,
   lässt sie weg und signiert die Dateien des Releases nach 6.3.
3. **Firebase-Werte als Variablen des Repos** (keine Geheimnisse, sie stehen in jeder APK; nur für die
   Handy-App): Settings › Secrets and variables › Actions › Reiter «Variables», oder
   `gh variable set SLOUCHCAT_FIREBASE_PROJECT_ID --body <Wert>`, ebenso `SLOUCHCAT_FIREBASE_APP_ID`,
   `SLOUCHCAT_FIREBASE_API_KEY`, `SLOUCHCAT_FIREBASE_SENDER_ID`. Alle vier oder keine: Bei einem bis
   drei bricht der Lauf ab.
4. **Tag-Ruleset:** Settings › Rules › Rulesets › «New ruleset» › «New tag ruleset». Name
   `App-Tags`, «Enforcement status» Active, «Target tags» › «Add target» › «Include by pattern»
   `app-v*` und `tv-v*`. Regeln «Restrict creations», «Restrict updates», «Restrict deletions»; unter
   «Bypass list» «Repository admin». Dann setzt nur du solche Tags.

### 7.2 Ablauf

Nicht geprüft, der Workflow lief noch nie.

1. `versionCode` und `versionName` in der Datei der App erhöhen, committen, über einen Pull Request nach
   `main`.
2. Auf dem Commit in `main` taggen und den Tag pushen:

   ```bash
   git tag -a app-v1.0.0 -m "Slouchcat phone app 1.0.0"
   git push origin app-v1.0.0
   ```

3. Der Workflow `.github/workflows/release-android.yml` läuft in drei Jobs:
   - **build** (nur Leserecht, ohne Secrets): prüft den Tag gegen die Datei
     (`scripts/android-tag-check.sh`) und dass der Commit in `main` liegt, fährt `task android:check`
     und `task android:test`, baut `task android:dist APPS=app` bzw. `APPS=tv` unsigniert.
   - **sign** (Environment `android-release`, wartet auf deine Freigabe): signiert mit
     `scripts/android-sign.sh`, ohne pnpm und ohne Gradle. Ohne Secrets reicht er unsigniert durch.
   - **publish** (Schreibrecht, ohne Secrets): hängt Release-APKs, Bundle und `SHA256SUMS` an ein
     GitHub-Release des Tags; die Debug-APKs nicht. Läuft derselbe Tag ein zweites Mal, löscht er
     zuerst alle Dateien des Releases und lädt dann die neuen hoch.
4. Das Release wird **nie «latest»**: Der Server und sein Updater lesen das neueste Release
   (`releases/latest/download/manifest.json`); ein App-Release darf das Server-Release dort nicht
   verdrängen. Die Workflows des Servers (`v*.*.*`) und der Dienste (`relay-v*.*.*`) laufen bei
   App-Tags nicht. Wird das «latest»-Release des Servers gelöscht oder zur Vorabversion, kann GitHub auf
   ein App-Release zurückfallen: dann `gh release edit vX.Y.Z --latest` mit dem neuesten Server-Release.

**Restrisiko:** Der Job «sign» signiert, was der Job «build» liefert, ohne es selbst bauen oder prüfen
zu können. Eine vergiftete Gradle-Abhängigkeit, ein vergiftetes Plugin oder ein vergifteter Cache im
Job «build» ergäbe eine Datei, die danach mit deinem Upload-Schlüssel signiert ist. Den Schlüssel
selbst sieht der Job «build» nie. Die Prüfsummen-Datei für die Gradle-Abhängigkeiten
(`android/gradle/verification-metadata.xml`), die das eingrenzen würde, ist noch offen: Sie braucht
einen eigenen Ablauf, damit Dependabot-Updates sie mitführen.

## 8. Nur im Emulator geprüft, nur auf echten Geräten möglich

**Im Emulator oder auf dem Rechner geprüft:**

- Beide Apps: Einrichten, Anmelden, Profil mit PIN, Abmelden (`task android:e2e`, `task android:e2e:tv`);
  Koppeln von Handy und Fernseher über eine lokale Vermittlung, je eine Seite mit der echten App
  (`docs/perf/K27-android-handy.md`, Stufe 9).
- Handy-App, alle Bereiche in einem Ablauf (`task android:e2e`, 16 Fälle auf `sc_phone` und `sc_tablet`):
  Stöbern, Detailseiten, Videoplayer mit echter Wiedergabe, Downloads samt Wiedergabe ohne Server, Hörbuch
  im Hintergrund, Podcast-Folge und Webradio gegen Teststationen, Wünsche gegen eine Attrappe von Seerr,
  Benachrichtigungen, «Fernseher verbinden».
- TV-App (`task android:e2e:tv`, 12 Fälle auf `sc_tv`): Start, Bibliotheken, Detailseiten, Suche,
  Videoplayer, Tonplayer und Webradio, Wünsche gegen die Attrappe von Seerr, Benachrichtigungen.
- Auf den Runnern von GitHub läuft derselbe Ablauf nur zur Information und ist dort nicht ganz grün
  (`docs/perf/K27-android-ci.md`); im Gate steht der Vertragstest `android-contract`.
- Krypto auf API 28 und 36, Download-Wiedergabe über die lokale HLS-Liste, Abruf im Hintergrund mit
  gebremstem Server auf dem Rechner (`docs/perf/K27-android-handy.md`, «Nur im Emulator geprüft»).
- Push nur mit der Attrappe `fcmfake` statt Google (`docs/perf/K27-android-handy.md`, Stufe 8).
- Ton der TV-App: Erkennung, Kanalzahl, Abwählen ohne Decoder; nur Software-Decoder, nur PCM Stereo
  (`docs/perf/K26-android-tv.md`, «Nur im Emulator geprüft»).
- Die Dateien aus `task android:dist`: Signatur der Debug-APKs (`apksigner verify`), `aapt2 dump
  badging` (Paket, Version, Symbol, TV-Starter, Banner, `leanback` verlangt nur bei der TV-App);
  Signieren mit `task android:sign` und einem Wegwerf-Schlüssel.
- Die Texte der Apps in 20 Sprachen: übersetzt und zweitgelesen, `task i18n:check` streng grün
  (`docs/i18n/glossar.md`, Abschnitt 9).

**Nur auf echten Geräten oder mit fremden Diensten:**

- Wünsche gegen ein echtes Seerr statt der Attrappe, an Handy und Fernseher.

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
| 3 | Am Handy Entwickleroptionen und USB-Debugging einschalten, Handy-App einspielen, Debugging wieder aus, einrichten | 2 |
| 4 | Gerätechecklisten der Handy-App abarbeiten | 2 |
| 5 | An jedem Fernseher Entwickleroptionen und Debugging einschalten, TV-App einspielen, von Hand und per Scan einrichten | 3 |
| 6 | Abnahmeliste der TV-App je Gerät und Tonanlage abarbeiten | 3 |
| 7 | Vermittlung und Push-Weiterleitung einrichten, `pair.slouchcat.com` und `push.slouchcat.com` darauf zeigen lassen | 4 |
| 8 | Firebase-Projekt anlegen, die vier Werte setzen, den Dienstkonto-Schlüssel der Push-Weiterleitung geben | 5 |
| 9 | Upload-Schlüssel erzeugen und sichern | 6.1 |
| 10 | Die vier Signatur-Werte setzen oder nachträglich signieren | 6.2, 6.3 |
| 11 | In GitHub: Environment `android-release` mit Pflicht-Reviewer und Tag-Regel, seine Signatur-Secrets (oder bewusst weglassen), Firebase-Variablen, Tag-Ruleset für `app-v*` und `tv-v*` | 7.1 |
| 12 | Version erhöhen, taggen, Signierlauf freigeben, Release prüfen | 7.2 |
| 13 | Bundle in die Play Console laden, das TV-Bundle nur in die Spur «Android TV» | 7, `docs/store/VEROEFFENTLICHEN.md` |
| 14 | Store-Grafiken erzeugen (`scripts/store-graphics.sh`) und Bildschirmfotos aufnehmen; beide liegen nicht im Repo | `docs/store/grafiken.md` |
| 15 | Wünsche mit dem echten Seerr an Handy und Fernseher abnehmen | 8 |
