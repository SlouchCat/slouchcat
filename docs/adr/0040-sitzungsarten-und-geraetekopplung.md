# ADR 0040: Sitzungsarten für native Apps und Gerätekopplung

- Status: angenommen (21. November 2026, Stufe 1 von Konzept 27, Teil 1); der Abschnitt «Kopplung»
  folgt mit Teil 3 derselben Stufe. **Nachtrag zu ADR 0005** (drei Sitzungsarten, Ablauf, Aufräumung
  der Geräte).
- Quelle: Konzept `docs/planning/27-android-handy-app.md` (K27), Abschnitte 2.3, 4, 11.5 und 15.1;
  Konzept `docs/planning/26-android-tv.md` (K26), Abschnitte 4.2, 4.5 und Anhang B; Marcos Entscheide
  E-7, E-8, E-14, E-15, E-16 und F-4; Frage 34 und Frage 35 in `docs/planning/04-entscheidungen.md`
- Vertrag: 1.42.0 (zusätzlich); Migration `00082_session_kind.sql`
- Harte Regeln: 5 (Migration nach Vorlage), 7 (`x-visibility` und neu `x-clients`, Vertragstest),
  9 (Texte), 10 (Token und Gerätekennung redigiert), 12 (Entscheid im Server)

## Kontext

Bis 1.41 kennt Slouchcat eine Sitzungsart: die Browser-Sitzung im Cookie `slouchcat_session`, 30 Tage
gleitend. Das Schema `bearerToken` stand seit M2 im Vertrag, keine Operation nutzte es, und die
Middleware las den Kopf `Authorization` nicht. Mit der Handy-App (K27) und der TV-App (K26) kommen zwei
Clients ohne Cookie dazu. Marco hat entschieden: Die Handy-App bleibt angemeldet, bis jemand sie abmeldet
(E-15), sie verwaltet nie (E-14) und stellt als einzige Kopplungsgutscheine aus (E-16); der Fernseher
bleibt ebenso angemeldet (F-4), verwaltet nie (E-7), lädt nichts herunter (E-8) und darf nur, was auf einer
Liste steht (K26 4.5).

Zu entscheiden war: wo die Art einer Sitzung steht und wie der Server sie prüft; wie «läuft nie ab» zu den
Abfragen passt, die `expires_at > @now` prüfen; wie der Vertrag sagt, welche Operation welche Art aufrufen
darf; woran der Server ein Gerät ohne Cookie erkennt; was die Aufräumung nach 400 Tagen mit Geräten tut,
deren Sitzung nie abläuft; und wie eine zu alte App von einem neuen Server erfährt.

Verworfen: (a) Die Art aus dem Träger ableiten (Bearer = App) — dann würde aus einem erbeuteten Cookie eine
App-Sitzung, die Gutscheine ausstellt, und aus einem App-Token im Cookie eine Sitzung mit 30 Tagen Ablauf und
`Set-Cookie`. (b) `expires_at` leer für «nie» — rund ein Dutzend Abfragen der Wiedergabe, des Radios und der
Verwaltung prüfen `expires_at > @now` und hätten alle angepasst werden müssen. (c) Eine eigene Tabelle für
App-Token — Wiedergabe, Radio und Abmelden hängen an `sessions.id`; eine zweite Tabelle hätte jede dieser
Stellen verdoppelt. (d) Die Erlaubnisliste des Fernsehers im Code — dann beantwortet eine neue Operation
die Frage nie; im Vertrag erzwingt sie der Schema-Test.

## Entscheid

| Punkt | Entscheid |
|---|---|
| Drei Arten | Spalte `sessions.kind` mit `browser`, `app` (Handy, Tablet) und `device` (Fernseher), Vorgabe `browser` für jede bestehende Sitzung. Die Art steht in der Sitzung; der Server leitet sie nie aus dem Träger ab |
| Träger | Browser-Sitzung im Cookie, App- und Gerätesitzung in `Authorization: Bearer`. Ein Browser-Token als Bearer, ein App- oder Geräte-Token im Cookie zählen wie kein Token. Cookie und Bearer zusammen beantwortet der Server auf jeder Route mit 401 `unauthenticated` |
| Ablauf | Browser 30 Tage gleitend wie bisher. App und Gerät bekommen den Ablauf `9999-12-31` (`auth.NeverExpires`), kein leerer Wert; `TouchSession` verschiebt ihn bei diesen Arten nicht und hält nur den letzten Kontakt fest |
| Kein Cookie | Eine Anfrage mit `Authorization`, mit `X-Slouchcat-Device` oder an `appLogin` bekommt nie ein `Set-Cookie` (weder Sitzung noch Gerät), ein 401 löscht kein Cookie. `logout` einer App- oder Gerätesitzung entzieht die Sitzung ohne Cookie und ohne `Clear-Site-Data` |
| CSRF | Der Kopf `X-Requested-With: Slouchcat` bleibt Pflicht für jede schreibende Anfrage, auch mit Bearer |
| Erlaubnisliste | Jede `account`- und `scoped`-Operation nennt in `x-clients` die Bearer-Arten, die sie aufrufen dürfen (`app`, `tv`). Fehlt die Art, antwortet der Server 403 `client_not_allowed`; `admin`- und `owner`-Operationen tragen keine Liste und sind darum für jede Bearer-Art gesperrt. Die Middleware liest die Liste zur Laufzeit aus dem eingebetteten Vertrag (`clientsOf` neben `visibilityOf`); für die Streaming-Routen steht sie in `binaryRoutes`. Der Schema-Test verlangt die Liste für `account` und `scoped` und verbietet sie sonst |
| Stand 1.42.0 | `app` an allen 116 Operationen des Nutzerbereichs (24 `account`, 92 `scoped`), `tv` an den 93 aus K26 Anhang B. Was dort Vorschlag ist (Empfehlen, Benachrichtigungen, Ankündigungen, Vorlieben der Wiedergabe ja; Kontodaten, Sehbericht, Meldekanäle nein, K26 F-18), gilt nach der Regel der Papiere als Empfehlung |
| Medien-Adressen | Die Routen mit eigenem Token (`st`, `dt`, `rt`) lesen den Bearer nicht. Ihr Token gibt nur eine Operation aus, die die Liste prüft (`startPlayback`, `getDownloadManifest`, `startRadioListen`) |
| App anmelden | `appLogin` (`POST /auth/app-login`, `public`): E-Mail, Passwort, Gerätename, Art (`phone`, `tablet`, `tv`), optional die Gerätekennung der App. `phone` und `tablet` ergeben eine App-Sitzung, `tv` eine Gerätesitzung. Antwort: Token (256 Bit, nur als Hash gespeichert), Gerätekennung und `SessionInfo`, mit `Cache-Control: no-store`. Fehlversuche zählen im Topf von `login`, das Passwort geht wie bei `login` an Seerr |
| Gerät | Die Gerätezeile entsteht beim Anmelden mit Name und Art. Ein Abspielstart einer App- oder Gerätesitzung schreibt Fähigkeiten und Besitzer, aber nicht Name und Art (`recordDevice`) |
| Gerätekennung | vergibt der Server. Eine mitgebrachte Kennung gilt nur, wenn sie die Form hat, nicht die reservierte `ffff…` ist und kein anderes Konto auf dem Gerät eine lebende Sitzung oder einen Download hat (`DeviceHeldByOtherAccount`); sonst vergibt er eine neue. Mit Bearer kommt das Gerät aus der Sitzung; ohne Sitzung nennt die App es im Kopf `X-Slouchcat-Device` (`checkOfflineRevocations`; bei `appLogin` gilt der Kopf, wenn der Körper keine Kennung trägt) |
| Aufräumung | `DeleteStaleDevices` verschont Geräte mit einer lebenden App- oder Gerätesitzung, die Aufräumung der Widerrufsliste (`DeleteOldTombstones`, 400 Tage) deren Einträge. Ist das Gerät einer App- oder Gerätesitzung trotzdem weg (in der Verwaltung vergessen), entzieht der Server die Sitzung und antwortet 401; eine neue Kennung gibt es dafür nicht (`loadSession`) |
| Verträglichkeit | `getHealth` nennt `contractVersion` (`info.version`). Die Apps nennen sich im Kopf `X-Slouchcat-Client` (`mobile/1.2.3`, `tv/1.2.3`). Ist die Version kleiner als die Mindestfassung der App (`api.DefaultMinClientVersions`, heute `0.0.0` für beide), antwortet der Server auf jeder Route ausser `getHealth` mit 426 `app_too_old`. Ein fehlender oder unlesbarer Kopf und ein unbekannter App-Name zählen nicht |
| Verwaltung | `listDevices` nennt je Gerät `sessionKind`, die Art der lebenden Sitzung mit dem jüngsten Kontakt; die Geräteliste zeigt sie in der Spalte «Anmeldung» |
| Logs | Die Redaktionsliste kennt `apptoken`, `devicetoken` und `slouchcatdevice` (dazu wie bisher `token`, `authorization`, `deviceid`) |

## Kopplung

Folgt mit Teil 3 von Stufe 1: Kopplungsgutschein ausstellen und einlösen, Verschlüsselung des Pakets und
«Server weist sich aus».

## Folgen

- Vertrag 1.42.0 ist zusätzlich. Neu sind `appLogin`, `AppLoginRequest`, `AppLogin`, `SessionKind`,
  `Health.contractVersion`, `AdminDevice.sessionKind` und `x-clients` an 116 Operationen.
- **Neue Regel neben «Vertrag = Tag» (K27 11.5, Frage F-9):** Operationen mit `x-clients` brechen nicht mehr
  von einem Release aufs nächste; eine brechende Änderung braucht eine Übergangszeit. Frei ändern lässt
  sich nur noch die Verwaltung.
- Der Vertragstest ruft jede Operation mit einem App-Token und einem TV-Token des Besitzerkontos mit
  aktivem Hauptprofil auf (`TestBearerClientsCoverEveryOperation`); fehlt die Art, erwartet er 403
  `client_not_allowed`. Jede falsche Paarung von Token und Träger hat einen Test (`TestBearerCarrierPairings`).
- Ehrlich gesagt (K27 4.2): Welche Art ein Client nennt, entscheidet der Client. Wer das Passwort kennt,
  meldet sich über `login` auch voll an. Die Sitzungsarten begrenzen den Schaden eines verlorenen Geräts oder
  Tokens, nicht den eines bekannten Passworts.
- Ein verlorenes Gerät bleibt angemeldet, bis jemand es abmeldet («Meine Geräte», Teil 2), die Verwaltung es
  vergisst oder das Passwort sich ändert (`updateAccount` entzieht jede andere Sitzung, auch App und Gerät).
