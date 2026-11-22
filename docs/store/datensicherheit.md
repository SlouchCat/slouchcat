# Angaben im Abschnitt «App-Inhalte» der Play Console

Vorgeschlagene Antworten für die Formulare, die Google vor der Veröffentlichung verlangt: Datensicherheit,
Vordergrunddienste, Alterseinstufung, Zielgruppe, Zugang für die Prüfer und die kleineren Erklärungen.
Grundlage: Konzept 27 Abschnitte 7.6 und 11.4, Konzept 26 Abschnitt 8.2, `docs/relay/BETRIEB.md`,
`datenschutz.md`. **Vorschläge, keine Rechtsberatung.** Wo die Papiere «unbelegt» sagen, steht es
hier auch; du entscheidest im Formular.

Die Begriffe von Google stehen auf der Hilfeseite «Datensicherheit»
(<https://support.google.com/googleplay/android-developer/answer/10787469>, gelesen am
21. November 2026):

- **Erheben** heisst: Daten verlassen das Gerät, auch über Bibliotheken. Nicht erhoben ist, was nur
  auf dem Gerät bleibt oder Ende-zu-Ende verschlüsselt ist, «not readable by any intermediary
  entity, including the developer». Daten, die nur im Arbeitsspeicher und nur für eine Anfrage
  gehalten werden («ephemeral»), gelten als erhoben, erscheinen aber nicht im Eintrag.
- **Teilen** heisst: erhobene Daten an einen Dritten geben. Nicht geteilt ist, was an einen
  Dienstleister geht, der nur im Auftrag arbeitet, und was der Nutzer selbst erwartbar überträgt.
- **Unbelegt:** wie Daten zählen, die an den **eigenen Server des Nutzers** gehen. Die Hilfeseite
  kennt den Fall nicht. Der Vorschlag unten zählt sie vorsichtig als erhoben und nicht geteilt.

## 1. Datensicherheit

### 1.1 Die Grundfragen

| Frage | Vorschlag | Begründung |
|---|---|---|
| Erhebt oder teilt die App Daten der verlangten Arten? | **Ja** | Sicher erhoben: Die zwei Dienste sehen das Push-Token von Google (Geräte-ID) und Internet-Adressen (K27 7.6). Vorsichtig erhoben: alles, was an den Server des Nutzers geht (unbelegt, siehe oben) |
| Werden alle erhobenen Daten bei der Übertragung verschlüsselt? | **Nein** | Die Dienste und Google sprechen nur HTTPS. Der Server des Nutzers darf über HTTP angesprochen werden (E-18, «HTTP überall mit Warnung»). Ein «Ja» wäre falsch, sobald man die Daten an den eigenen Server als erhoben zählt. Der Eintrag zeigt dann «Daten werden nicht verschlüsselt» |
| Können Nutzer die Löschung ihrer Daten verlangen? | **Ja** | Der Entwickler hält keine Daten auf Dauer (D-1). Auf dem Gerät löscht Abmelden oder Deinstallieren alles; auf dem Server meldet «Meine Geräte» ein Gerät ab und räumt dessen Downloads und Push-Abos ab (K27 4.5); Anfragen an den Entwickler über `<E-Mail-Adresse für Datenschutz>` |
| Konto in der App anlegen möglich? (Pflicht zur Kontolöschung) | **Nein** | Die App legt kein Konto an: Ersteinrichtung und Einladung bleiben im Browser (K27 3.1). Die Pflicht gilt nur, wenn sich in der App ein Konto anlegen lässt (K27 2.7). Eine Adresse für die Kontolöschung braucht es darum nicht |
| Unabhängige Sicherheitsprüfung (MASA) | Nein | keine gemacht |

**Wenn du die Daten an den eigenen Server nicht als erhoben zählst** (Gegenposition: Der Entwickler
bekommt sie nie, der Nutzer bestimmt den Empfänger), bleiben nur die Zeilen «Geräte- oder andere
IDs» (Push-Token, nur kurz bei den Diensten) aus der Tabelle unten. Die Verschlüsselung wäre dann
«Ja», weil Dienste und Google nur HTTPS sprechen. Das ist die günstigere Anzeige, aber die
Einordnung ist unbelegt; eine Ablehnung bei der Prüfung kostet eine Runde.

### 1.2 Datenarten (Vorschlag mit eigenem Server als «erhoben»)

| Kategorie › Art | Erhoben | Geteilt | Nur kurz (ephemeral) | Pflicht oder freiwillig | Zweck | Was genau |
|---|---|---|---|---|---|---|
| Persönliche Angaben › E-Mail-Adresse | ja | nein | nein | Pflicht | App-Funktionen, Kontoverwaltung | Anmeldung am eigenen Server |
| Persönliche Angaben › Nutzer-IDs | ja | nein | nein | Pflicht | App-Funktionen | Kennungen von Konto und Profil, die der Server vergibt und zurückbekommt |
| App-Aktivitäten › Interaktionen mit der App | ja | nein | nein | Pflicht | App-Funktionen | Wiedergabe-Fortschritt, Merkliste, Gesehen, Downloads, an den eigenen Server |
| App-Aktivitäten › Suchverlauf in der App | ja | nein | nein | Pflicht | App-Funktionen | Suchbegriffe gehen an den eigenen Server; gespeichert werden sie dort nicht (unbelegt, am Server prüfen) |
| Geräte- oder andere IDs | ja | nein | nein | Pflicht | App-Funktionen | Gerätekennung des Servers; Push-Token von Google (bei den Diensten nur kurz, beim eigenen Server in der Push-Adresse versiegelt); Installationskennung von Firebase (an Google, für das Push-Token) |
| App-Aktivitäten › Sonstige nutzergenerierte Inhalte | ja | nein | nein | Freiwillig | App-Funktionen | Wünsche (angeforderte Titel und Staffeln) an den eigenen Server, der sie an das Seerr des Haushalts gibt (Stufe 8) |

**Nicht angekreuzt**, mit Grund:

- Standort: Die Dienste nutzen Internet-Adressen nur für die Ratenbegrenzung, nicht für einen
  Standort.
- Fotos, Videos, Audio, Dateien: Medien kommen vom eigenen Server aufs Gerät, nicht vom Gerät weg.
- Name, Adresse, Telefon, Finanzen, Gesundheit, Nachrichten, Kontakte, Kalender, Webverlauf,
  installierte Apps: erhebt die App nicht.
- Absturzberichte, Diagnose: keine Bibliothek dafür (am Bau prüfen, `datenschutz.md` Abschnitt 3).
  Die Angaben zum Gerät (Modell, Android-Fassung, Decoder) gehen an den eigenen Server, damit er die
  Wiedergabe wählt. Ob Google sie unter «App-Info und -Leistung › Sonstiges» sieht, ist unbelegt;
  vorsichtig wäre, sie dort mit Zweck «App-Funktionen» anzukreuzen.
- Geteilt: nichts. Google (Push) und die zwei Dienste sind Dienstleister im Auftrag; der eigene
  Server ist eine Übertragung, die der Nutzer selbst anstösst.
- Inhalt der Push-Meldungen: Titel und Text verschlüsselt der eigene Server für den Schlüssel des
  Geräts (Web-Push, RFC 8291); Push-Weiterleitung und Google sehen nur Geheimtext. Das ist
  Ende-zu-Ende verschlüsselt und zählt nach Google nicht als erhoben (Stufe 8 gebaut: Entschlüsselung
  auf dem Gerät, privater Schlüssel nur auf dem Gerät, nicht in der Sicherung).

## 2. Vordergrunddienste

Für Ziel-API 34 und höher verlangt Google je Art des Vordergrunddienstes eine Beschreibung und ein
Video (K27 2.7, Anhang A). Die App nutzt zwei Arten. Was die App tatsächlich im Manifest erklärt,
am fertigen Bau prüfen (`android/app-mobile/src/main/AndroidManifest.xml`); die TV-App nimmt
`mediaPlayback` nur für die Wiedergabe im Vordergrund und keinen Ton im Hintergrund (K26 5.6).

### 2.1 `mediaPlayback`

**Beschreibung (Deutsch):**

> Die App spielt Hörbücher, Podcasts und Webradio vom eigenen Slouchcat-Server des Nutzers ab. Hat
> der Nutzer die Wiedergabe gestartet, läuft sie weiter, wenn der Bildschirm aus ist oder eine
> andere App vorn steht, mit Bedienung in der Mitteilung, am Sperrbildschirm und über Kopfhörer.
> Der Dienst läuft nur, solange der Nutzer etwas abspielt, und endet mit Stopp oder Pause.

**Description (English):**

> The app plays audiobooks, podcasts and web radio from the user's own Slouchcat server. Once the
> user has started playback, it continues while the screen is off or another app is in front, with
> controls in the notification, on the lock screen and on headphones. The service runs only while
> the user is playing something and ends on stop or pause.

**Folge, wenn der Dienst fehlt oder aufgeschoben wird:** Ein Hörbuch bricht ab, sobald der
Bildschirm ausgeht. / *Impact if deferred: an audiobook stops as soon as the screen turns off.*

### 2.2 `dataSync`

**Beschreibung (Deutsch):**

> Der Nutzer tippt bei einem Film, einer Folge oder einem Hörbuch auf «Herunterladen». Die App
> holt dann eine Kopie vom eigenen Slouchcat-Server des Nutzers aufs Gerät, damit er sie ohne Netz
> abspielen kann. Der Abruf zeigt seinen Fortschritt in einer Mitteilung, lässt sich dort anhalten
> und endet, wenn die Datei da ist. Auf Android 14 und neuer läuft derselbe Abruf als vom Nutzer
> gestartete Übertragung; den Vordergrunddienst nutzen nur ältere Geräte.

**Description (English):**

> The user taps «Download» on a film, episode or audiobook. The app then fetches a copy from the
> user's own Slouchcat server to the device, so it can be played without a network. The transfer
> shows its progress in a notification, can be paused there and ends once the file is complete. On
> Android 14 and later the same transfer runs as a user-initiated data transfer job; only older
> devices use the foreground service.

**Folge, wenn der Dienst fehlt oder aufgeschoben wird:** Ein grosser Download bricht ab, sobald die
App in den Hintergrund geht, und der Nutzer hat unterwegs den Film nicht. / *Impact if deferred: a
large download stops when the app goes to the background, and the user does not have the film
when offline.*

**So gebaut (Stufe 6, ADR 0043):** Der Vordergrunddienst `dataSync` läuft nur auf Android 9 bis 13
und startet nur, während die App sichtbar ist (nach «Herunterladen», «Fortsetzen» oder beim Öffnen
der App mit offenen Downloads); er holt dann alle offenen Downloads und endet, wenn sie da sind.
Bei geschlossener App holt WorkManager Auto-Downloads (ohne Tippen des Nutzers, K27 9.3) in kurzen
Läufen ohne Vordergrunddienst. Das Manifest erklärt dazu
`POST_NOTIFICATIONS`, `RUN_USER_INITIATED_JOBS`, `FOREGROUND_SERVICE`,
`FOREGROUND_SERVICE_DATA_SYNC` und `RECEIVE_BOOT_COMPLETED` (der Abruf überlebt einen Neustart).

**Unbelegt:** ob Google für den Übertragungsauftrag (Android 14 und neuer) eine eigene Erklärung
verlangt.

### 2.3 Drehbuch für die zwei Videos

Je ein Bildschirmvideo aus dem Emulator `sc_phone` gegen den Vorführserver, 30 bis 60 Sekunden, ohne
Ton, als nicht gelistetes YouTube-Video oder an der Stelle, die die Play Console nennt
(`<Adresse des Videos mediaPlayback>`, `<Adresse des Videos dataSync>`). Aufnahme:
`adb shell screenrecord /sdcard/fgs.mp4`, danach `adb pull`.

**Video `mediaPlayback`:**

1. App offen, angemeldet mit dem Prüfkonto (0–5 s).
2. Ein Hörbuch öffnen und auf Abspielen tippen (5–15 s). Der Vorführserver hat ab Werk kein
   Hörbuch (`deploy/demo/README.md`, «Nicht dabei»): Vor der Aufnahme ein gemeinfreies Hörbuch,
   zum Beispiel von LibriVox, nach `media/Audiobooks/` legen und in der Verwaltung als Bibliothek
   «Hörbücher» anlegen.
3. Auf den Startbildschirm wechseln: Die Mitteilung mit Bedienung erscheint, der Ton läuft (15–25 s).
4. Bildschirm aus, wieder an: Bedienung am Sperrbildschirm, Ton läuft (25–40 s).
5. In der Mitteilung Pause, dann Stopp: Die Mitteilung verschwindet (40–50 s).

**Video `dataSync`:**

1. App offen, Detailseite «Tears of Steel» (0–5 s).
2. Auf «Herunterladen» tippen, Stufe wählen, bestätigen (5–15 s).
3. Auf den Startbildschirm wechseln, Mitteilungsleiste öffnen: Fortschritt des Downloads (15–30 s).
4. Zurück in die App, Seite «Downloads»: der Film ist fertig (30–45 s).
5. Flugmodus an, den Film aus der Offline-Bibliothek abspielen (45–60 s).

Für das zweite Video ein Gerät unter Android 14 nehmen (zum Beispiel `sc_phone28`), weil nur dort der
Vordergrunddienst läuft.

## 3. Alterseinstufung (Fragebogen der IARC)

Der Fragebogen fragt nach dem, was die App selbst zeigt. Slouchcat zeigt nur die Sammlung des
Nutzers von dessen eigenem Server; die App bringt keine Inhalte mit. **Unbelegt:** wie die IARC
eine App einordnet, deren Inhalte vollständig vom Nutzer kommen, und ob sie sie als
«uneingeschränkten Zugang» zählt. Die genauen Fragen zeigt erst das Formular.

| Frage (sinngemäss) | Vorschlag | Begründung |
|---|---|---|
| Kategorie | «Sonstiges» (Utility, Productivity, Communication or Other) | Abspieler ohne eigene Inhalte; «Consumer store or commercial streaming service» passt nicht, die App verkauft und vertreibt nichts |
| Gewalt, Sexualität, Sprache, Drogen, Glücksspiel in der App | Nein | die App enthält keine Inhalte |
| Können Nutzer miteinander kommunizieren oder Inhalte austauschen? | Nein | keine Nachrichten, kein Austausch zwischen Konten; Empfehlungen bleiben im Haushalt des Servers |
| Teilt die App den Standort mit anderen Nutzern? | Nein | |
| Digitale Käufe | Nein | |
| Zugang zu Inhalten aus dem Internet ohne Filter | **Ja, mit Hinweis** | Die App spielt ab, was der Server bietet, den der Nutzer einträgt; Profile mit Altersgrenze filtert der Server. Ehrlicher als «Nein». Wenn die Frage im Formular so nicht vorkommt, entfällt die Zeile |

Ergebnis vermutlich eine niedrige Einstufung; sie gilt für die App, nicht für die Filme des Nutzers.

## 4. Zielgruppe und Familienrichtlinie

**Vorschlag: Zielgruppe «18 Jahre und älter»**, und auf die Frage, ob die App Kinder anspricht:
**Nein**.

- Die App richtet sich an den Inhaber eines Kontos auf einem Slouchcat-Server (K27 11.4). Kinder
  nutzen sie nur über ein Kinderprofil, das der Inhaber einrichtet und das der Server begrenzt.
- **Folge:** Wählst du eine Altersgruppe unter 13, gilt die Familienrichtlinie von Google mit
  eigenen Pflichten (unter anderem zu Bibliotheken und Datenerhebung). Mit «18 und älter» gilt sie
  nicht.
- **Folge für die Grafiken:** Der Eintrag darf nicht wirken, als richte er sich an Kinder. Darum
  auf Bildschirmfotos und Feature-Grafik nicht «Big Buck Bunny» in den Vordergrund stellen
  (Trickfilm mit Tieren), sondern «Tears of Steel» und «Sintel» (`grafiken.md`).
- **Offen und unbelegt:** ob Google Kinderprofile in einer App für Erwachsene als «für Kinder
  geeignet» wertet und dann Fragen stellt; ob «16 bis 17» zusätzlich sinnvoll ist. Vor dem Antrag
  klären (K27 11.4).

## 5. Zugang für Googles Prüfer

Play Console › App-Inhalte › «App-Zugriff»: **«Alle oder einige Funktionen sind eingeschränkt»**,
dann eine Anleitung anlegen. Den Server liefert `deploy/demo/` (F-10). Felder und Text:

| Feld | Inhalt |
|---|---|
| Name der Anleitung | `Slouchcat demo server` |
| Nutzername / E-Mail | `<E-Mail-Adresse des Prüfkontos>` |
| Passwort | `<Passwort des Prüfkontos>` |
| Weitere Angaben | Text unten |

```text
Slouchcat is a player for the user's own self-hosted Slouchcat server; the app contains no content. To review it, use our demo server with open movies of the Blender Foundation (CC BY):

1. Start the app. On the first screen enter the server address: https://<Name des Vorführservers>
2. Sign in with the e-mail and password above. The profile "Demo" opens; it has no PIN.
3. The home screen shows four films. Tap one to see details, play it, or tap "Download" to save it for offline playback.
4. Audiobooks, podcasts and web radio are empty on this server. Wishes are not set up.
5. TV (Android TV form factor): on the TV choose "Set up by hand", enter the same address and sign in with the same account. Pairing by QR code needs the phone app signed in to the same server.

The account does not expire and has no administration rights. The server runs only for the review.
```

Die Angaben dürfen sich nicht ändern, solange Google prüft; nach einer Passwortänderung hier
nachtragen (`deploy/demo/README.md`, «Was das Einrichten tut»).

## 6. Weitere Erklärungen

| Formular | Vorschlag |
|---|---|
| Werbung | Die App enthält keine Werbung |
| Nachrichten-App | Nein |
| Gesundheits-, Finanz-, Behörden-App | Nein |
| COVID-19 | Nicht zutreffend |
| Datenschutzerklärung | `<Adresse der Datenschutzerklärung>` |
| Berechtigungen mit eigener Erklärung | voraussichtlich keine: Benachrichtigungen, Internet und Vordergrunddienste brauchen keine eigene Erklärung ausser den Vordergrunddiensten (Abschnitt 2); der QR-Scanner braucht keine Kamera-Berechtigung (K27 2.7). Am Manifest des fertigen Baus prüfen |
| Ziel-API | Handy 36, TV 34 (Festlegungen des Baus) |

## Noch einzutragen

| Platzhalter | Was |
|---|---|
| `<E-Mail-Adresse für Datenschutz>` | wie in `datenschutz.md` |
| `<Adresse der Datenschutzerklärung>` | HTTPS-Adresse der Seite |
| `<E-Mail-Adresse des Prüfkontos>`, `<Passwort des Prüfkontos>` | aus `deploy/demo/.env` |
| `<Name des Vorführservers>` | öffentlicher Name des Vorführservers |
| `<Adresse des Videos mediaPlayback>`, `<Adresse des Videos dataSync>` | Videos nach Abschnitt 2.3 |

**Unbelegt, im Formular zu entscheiden:** Einordnung der Daten an den eigenen Server (1.1);
Diagnosedaten der Geräteangaben (1.2); Erklärung für den Übertragungsauftrag und für
Auto-Downloads (2.2); Kategorie und «uneingeschränkter Zugang» der IARC (3); Wertung der
Kinderprofile (4).
