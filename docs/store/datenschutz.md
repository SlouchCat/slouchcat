# Datenschutzerklärung der App Slouchcat (Entwurf)

> **Entwurf vom 21. November 2026, nicht veröffentlicht.** Geschrieben vom Bau (Konzept 27,
> Frage F-36) aus den Papieren des Projekts. **Dieser Entwurf ersetzt keine Rechtsberatung.** Der
> Auftraggeber prüft ihn, füllt die Platzhalter `<…>` und lässt ihn bei Bedarf rechtlich prüfen,
> bevor er ihn veröffentlicht. Die Fassung zum Veröffentlichen steht in `datenschutz.html`
> (Deutsch und Englisch, ohne Skripte); die englische Fassung dieses Texts in `privacy.md`.
> Die Kästen «Hinweis für den Auftraggeber» gehören nicht in die veröffentlichte Fassung.

Stand: `<Datum der Veröffentlichung>`

## 1. Worum es geht

Diese Erklärung gilt für die App «Slouchcat» für Android (Paket `com.slouchcat.app`) auf Handys,
Tablets und Fernsehern und für die zwei Hilfsdienste, die das Projekt für die App betreibt.

Slouchcat ist ein Medienserver, den jeder selbst betreibt. Die App ist der Abspieler dazu. Sie
enthält keine Inhalte und verbindet sich mit einem Slouchcat-Server, dessen Adresse du eingibst.
**Diesen Server betreibt nicht der Entwickler der App**, sondern du selbst oder die Person, die
dich eingeladen hat. Was dort mit deinen Daten geschieht, verantwortet der Betreiber des Servers.

## 2. Verantwortlich

Verantwortlich für die App und die zwei Hilfsdienste:

`<Name des Entwicklers>`
`<Postadresse>`
E-Mail: `<E-Mail-Adresse für Datenschutz>`

Verantwortlich für die Daten auf einem Slouchcat-Server ist dessen Betreiber.

## 3. Was der Entwickler nicht tut

- Kein Konto beim Entwickler. Die App legt nirgends ein Konto an ausser auf deinem Server.
- Keine Werbung, keine Werbe-Kennungen.
- Keine Analyse, keine Nutzungsstatistik, keine Absturzberichte an den Entwickler.
- Kein Verkauf und keine Weitergabe von Daten.
- Die zwei Hilfsdienste speichern nichts auf Dauer (Abschnitt 6).

> **Hinweis für den Auftraggeber:** «Keine Absturzberichte» und «keine Analyse» gelten nur, solange
> die App keine solche Bibliothek enthält. Vor dem Veröffentlichen am Bau prüfen
> (`android/gradle/libs.versions.toml`, Abhängigkeiten der Bauart `play`).

## 4. Was auf dem Gerät bleibt

Die App speichert auf deinem Gerät:

- die Adresse deines Servers und, bei unverschlüsselten Verbindungen, den öffentlichen Schlüssel,
  mit dem sich der Server ausweist;
- das Anmeldetoken deiner Sitzung und eine Gerätekennung, die dein Server vergibt;
- gespeicherte Filme, Folgen und Hörbücher samt Untertiteln und Bildern, die Liste der Downloads,
  Wiedergabe-Ereignisse ohne Netz, die noch zum Server müssen, und eine Prüfgrösse für die
  Offline-PIN eines Profils;
- für Push-Meldungen: einen eigenen Schlüssel, mit dem nur dieses Gerät die Meldungen entschlüsselt,
  das Push-Token von Google und die Push-Adresse; verschlüsselt mit einem Schlüssel des
  Android-Schlüsselspeichers, nie in einer Sicherung;
- deine Einstellungen.

In die Sicherung des Geräts und beim Umzug auf ein neues Gerät geht nur die Adresse deines
Servers; Anmeldung, Downloads und alles andere bleiben auf dem Gerät. Beim Deinstallieren löscht
Android alles, was die App gespeichert hat. Beim Abmelden fragt die App, ob sie die Downloads
löschen soll.

## 5. Was an deinen Server geht

Die App sendet an den Server, dessen Adresse du eingibst, und an keinen anderen Server des
Entwicklers:

- beim Anmelden E-Mail-Adresse und Passwort, danach bei jeder Anfrage das Anmeldetoken;
- die Gerätekennung, die Art und den Namen des Geräts (zum Beispiel Hersteller, Modell und
  Android-Fassung) und die Fassung der App;
- was das Gerät abspielen kann, damit der Server die passende Wiedergabe wählt;
- was du tust: gewählte Profile, Suchen, Merkliste, Wiedergabe-Fortschritt, Downloads und
  Einstellungen;
- deine Wünsche: welchen Film oder welche Staffel du anforderst oder zurückziehst. Dein Server gibt
  sie an das Seerr deines Haushalts weiter, nicht an den Entwickler;
- für Push-Meldungen die Push-Adresse der Weiterleitung und den öffentlichen Teil des Schlüssels
  dieses Geräts; damit verschlüsselt dein Server jede Meldung so, dass nur dieses Gerät sie lesen
  kann;
- die Internet-Adresse deines Geräts, wie bei jeder Verbindung.

Verbindet sich die App ohne HTTPS mit dem Server, gehen diese Angaben unverschlüsselt über das
Netz. Die App warnt davor. Wir empfehlen HTTPS.

## 6. Die zwei Hilfsdienste

Das Projekt betreibt zwei kleine Dienste, die nur Verschlüsseltes weiterreichen. Sie haben keine
Konten, keine Datenbank und schreiben keine Internet-Adressen in ihre Protokolle. Für die
Begrenzung der Anfragen je Aufrufer halten sie die Adresse kurz im Arbeitsspeicher. Sie laufen
`<Ort der zwei Dienste, zum Beispiel: auf eigener Hardware in der Schweiz>`.

**Vermittlung beim Einrichten eines Fernsehers** (`pair.slouchcat.com`). Wird ein Fernseher per
Scan eingerichtet, legt die Handy-App dort ein Paket ab, das nur der Fernseher entschlüsseln kann;
der Fernseher holt es ab. Der Dienst sieht die Internet-Adressen von Fernseher und Handy, den
Zeitpunkt und das verschlüsselte Paket. Er sieht nicht die Adresse deines Servers, nicht dein
Konto und nicht den Gutschein im Paket. Ein Paket lebt höchstens fünf Minuten im Arbeitsspeicher.
Richtest du den Fernseher von Hand ein, ist der Dienst nicht beteiligt.

**Weiterleitung von Benachrichtigungen** (`push.slouchcat.com`). Damit Meldungen deines Servers
auch bei geschlossener App ankommen, meldet sich die App mit ihrem Push-Token von Google bei diesem
Dienst an und gibt deinem Server eine Push-Adresse, in der das Token verschlüsselt steckt. Dein
Server schickt jede Meldung verschlüsselt an diese Adresse, der Dienst reicht sie an Googles
Push-Dienst weiter. Der Dienst sieht dabei:

- beim Anmelden die Internet-Adresse deines Geräts und das Push-Token;
- bei jeder Meldung die Internet-Adresse deines Servers, den Zeitpunkt, die Häufigkeit und die
  verschlüsselte Meldung.

Er sieht nicht Titel und Text einer Meldung und nicht dein Konto. Er speichert nichts davon.

Die App meldet sich bei jedem Start mit einer Sitzung, nach einem neuen Push-Token und nach einer
Passwortänderung erneut an; schaltest du Push-Meldungen auf diesem Gerät aus, löscht sie das Abo bei
deinem Server.

> **Am Bau bestätigt (Stufe 8):** Die App selbst ruft `POST /v1/register` auf; der Dienst sieht
> dabei die Internet-Adresse des Handys. Konzept 27 Abschnitt 7.6 nennt nur die des Servers.

## 7. Google

- **Push (Firebase Cloud Messaging):** Google stellt die Meldungen zu. Google sieht das Push-Token,
  den Zeitpunkt und die verschlüsselte Meldung, nicht deren Inhalt und nicht die Adresse deines
  Servers. Die Bibliothek von Google auf dem Gerät tauscht dafür Daten mit Google aus: Für das
  Push-Token meldet sie eine Installationskennung von Firebase und die Kennung der App bei Google
  an. Die App schaltet die Analyse von Firebase ab und bindet keine weitere Firebase-Bibliothek ein. Es gilt die
  Datenschutzerklärung von Google (<https://policies.google.com/privacy>) und die Angaben zu
  Firebase (<https://firebase.google.com/support/privacy>). Google kann Daten in den USA
  verarbeiten. Die Bauart ohne Google-Bibliotheken (`foss`, nicht im Play Store) nutzt diesen Weg
  nicht.
- **QR-Scanner:** Den QR-Code eines Fernsehers liest der «Google code scanner» der
  Google-Play-Dienste. Die App selbst bekommt kein Kamerabild, nur den gelesenen Code.
- **Play Store:** Installation, Updates und Kauf über Google Play verarbeitet Google nach eigener
  Erklärung. Der Entwickler sieht davon nur zusammengefasste Zahlen der Play Console.

> **Hinweis für den Auftraggeber:** Ob der Code-Scanner und die Push-Bibliothek von sich aus
> Diagnosedaten an Google senden, ist in den Papieren unbelegt. Vor dem Veröffentlichen in den
> Unterlagen von Google nachlesen und den Absatz anpassen.

## 8. Rechtsgrundlagen

Nach der EU-Datenschutz-Grundverordnung (DSGVO): Die Daten, die die App für ihre Aufgaben braucht
(Abschnitte 4 bis 7), verarbeiten wir, weil du die App nutzt und die Funktion anforderst
(Art. 6 Abs. 1 lit. b DSGVO). Die kurze Begrenzung der Anfragen je Internet-Adresse dient dem
Schutz der Dienste vor Missbrauch (berechtigtes Interesse, Art. 6 Abs. 1 lit. f DSGVO). Nach dem
Schweizer Datenschutzgesetz (revDSG) bearbeiten wir Personendaten nur für diese Zwecke und nur so
lange, wie es dafür nötig ist.

## 9. Deine Rechte

Du hast nach revDSG und DSGVO das Recht auf Auskunft, Berichtigung, Löschung, Einschränkung,
Herausgabe deiner Daten und Widerspruch. Weil die zwei Dienste nichts speichern, hat der
Entwickler in der Regel keine Daten über dich, die er herausgeben oder löschen könnte. Deine
Daten auf einem Slouchcat-Server verwaltet dessen Betreiber; dort kannst du auch Geräte abmelden
(«Meine Geräte»). Die Daten auf dem Gerät löschst du, indem du dich abmeldest oder die App
deinstallierst.

Anfragen an den Entwickler: `<E-Mail-Adresse für Datenschutz>`. Du kannst dich auch bei einer
Aufsichtsbehörde beschweren, in der Schweiz beim Eidgenössischen Datenschutz- und
Öffentlichkeitsbeauftragten (EDÖB), in der EU bei der Behörde deines Wohnsitzes.

## 10. Kinder

Die App richtet sich an Inhaberinnen und Inhaber eines Kontos auf einem Slouchcat-Server, nicht an
Kinder. Ein Konto kann Kinderprofile mit Altersgrenze haben; diese richtet der Inhaber des Kontos
ein, und die Grenzen setzt der Server durch. Die App erhebt über Kinder nichts anderes als über
andere Profile.

## 11. Änderungen

Ändert sich, was die App oder die Dienste verarbeiten, passen wir diese Erklärung an und ändern
das Datum oben. Die jeweils gültige Fassung steht unter `<Adresse der Datenschutzerklärung>`.

---

## Noch einzutragen

| Platzhalter | Was |
|---|---|
| `<Datum der Veröffentlichung>` | Stand der Erklärung |
| `<Name des Entwicklers>` | bürgerlicher Name oder Firma, wie im Play-Konto (öffentlich, K27 R-15) |
| `<Postadresse>` | ladungsfähige Adresse des Verantwortlichen |
| `<E-Mail-Adresse für Datenschutz>` | Adresse für Anfragen; kann die des Store-Eintrags sein |
| `<Ort der zwei Dienste, …>` | wo die Dienste laufen; beim Umzug auf Mietserver (ADR 0042) anpassen, samt Anbieter, falls dieser die Daten sieht |
| `<Adresse der Datenschutzerklärung>` | HTTPS-Adresse der veröffentlichten Seite |

**Unbelegt, vom Auftraggeber zu klären:** Diagnosedaten von Code-Scanner und Push-Bibliothek
(Abschnitt 7); ob die App eine Analyse- oder Absturzbibliothek enthält (Abschnitt 3); Adresse
ob ein Mietserver-Anbieter
nach dem Umzug als Auftragsverarbeiter zu nennen ist.
