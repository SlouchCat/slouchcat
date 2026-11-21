# ADR 0040: Sitzungsarten für native Apps und Gerätekopplung

- Status: angenommen (21. November 2026, Stufe 1 von Konzept 27, Teil 1; die Abschnitte «Kopplung» und
  «Server weist sich aus» mit Teil 3 derselben Stufe). **Nachtrag zu ADR 0005** (drei Sitzungsarten, Ablauf, Aufräumung
  der Geräte).
- Quelle: Konzept `docs/planning/27-android-handy-app.md` (K27), Abschnitte 2.3, 4, 11.5 und 15.1;
  Konzept `docs/planning/26-android-tv.md` (K26), Abschnitte 4.2, 4.5 und Anhang B; Marcos Entscheide
  E-7, E-8, E-14, E-15, E-16 und F-4; Frage 34 und Frage 35 in `docs/planning/04-entscheidungen.md`
- Vertrag: 1.42.0 (zusätzlich); Migrationen `00082_session_kind.sql`, `00083_pairing_vouchers.sql`,
  `00084_device_linked_deliveries.sql`
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

## Meine Geräte und Aufräumen

Teil 2 von Stufe 1 (K27 4.5 und 15.1, F-15, F-24, F-40; K26 E-9 und F-19). Vertrag 1.42.0, ohne Migration.

| Punkt | Entscheid |
|---|---|
| Operationen | `listMyDevices` (`GET /account/devices`) und `signOutMyDevice` (`DELETE /account/devices/{deviceId}`), beide `account` mit `x-clients: [app]`. Eine Gerätesitzung (TV) bekommt 403 `client_not_allowed`, sonst könnte ein gekoppelter Fernseher die echten Geräte abmelden (K26 4.5) |
| Welche Geräte | Jedes Gerät mit einer lebenden Sitzung **oder** einem Download des Kontos (`ListMyDevices`). Massgeblich ist das Konto der Sitzung oder des Downloads, nie `devices.account_id` (Besitzer des letzten Abspielstarts). Ein Browser mit abgelaufener Sitzung bleibt so in der Liste, solange er Downloads des Kontos hält; dann fehlen `sessionKind` und `signedInAt` |
| Felder | `MyDevice`: `id`, `name`, `type`, `sessionKind` und `signedInAt` der lebenden Sitzung des Kontos mit dem jüngsten Kontakt, `lastSeenAt` des Geräts, `downloads` (Anzahl des Kontos), `current` (Gerät der aufrufenden Sitzung) |
| Wer darf | Lesen jedes Profil. Abmelden nur eine Sitzung ohne aktives Profil oder mit dem Hauptprofil (403 `primary_profile_required`, die Regel der Meldekanäle). Ein Gerät ohne Sitzung und ohne Download des Kontos, ein fremdes Gerät und die reservierte Kennung `ffff…` sind 404 |
| Abmelden | Räumt in einer Transaktion alles weg, was **dieses Konto** auf dem Gerät hat, und nichts eines anderen Kontos: die Sitzungen (ihre Streams und Radio-Sitzungen enden vorher wie bei `logout`), übrige Wiedergaben, die Downloads (der Auslöser `downloads_tombstone` schreibt die Widerrufsliste; die Dateien räumt die Aufräumung von `offline` mit, wie nach `deleteDevice`), die Push-Abos des Kontos auf dem Gerät und die Regeln für Auto-Downloads der Profile des Kontos. Die Gerätezeile bleibt; ganz vergessen bleibt `deleteDevice` der Verwaltung, für alle Konten |
| Eigenes Gerät | Erlaubt; es beendet die eigene Sitzung. Ein Browser bekommt dabei das Sitzungs-Cookie gelöscht wie bei `logout`, die Web-App meldet sich danach auch hier ab |
| Push bei `logout` | `logout` einer App- oder Gerätesitzung löscht die Push-Abos des Kontos auf ihrem Gerät (`DeleteSessionPushSubscriptions`); die App trägt ihr Abo bei der nächsten Anmeldung neu ein. Ein Browser behält sein Abo (F-40) |
| Push bei `deleteDevice` | löscht die Abos des Geräts aller Konten **vor** der Gerätezeile (`DeleteDevicePushSubscriptions`); danach wäre die Kennung im Abo leer (`ON DELETE SET NULL`) und das Abo nicht mehr zu finden |
| Passwortänderung | `updateAccount` mit neuem Passwort entzieht wie bisher jede andere Sitzung, App- und Gerätesitzungen eingeschlossen (F-24), und löscht dazu die Push-Abos des Kontos auf den Geräten dieser App- und Gerätesitzungen (`DeleteOtherAppDevicePushSubscriptions`), nicht aber auf dem Gerät der aufrufenden Sitzung. Abos von Browsern bleiben, weil ein Browser sich nicht von selbst neu einträgt (F-40) |
| Bekannte Grenze | Abos ohne Gerätekennung (aus der Zeit vor 1.42.0, oder ihr Gerät hat die Aufräumung nach 400 Tagen gelöscht) räumt niemand auf. Sie fallen wie bisher nur bei 404/410 der Gegenstelle, mit dem Konto, über `deletePushSubscription` oder mit einem neuen VAPID-Schlüssel |
| Web-App | Seite «Meine Geräte» unter `/settings/devices`, in der Seitenleiste im Band «Konto» für jedes Profil; ein anderes als das Hauptprofil sieht die Liste ohne «Abmelden» und mit Erklärung. Texte unter `settings.devices.*` |
| PIN-Empfehlung | «Profile verwalten» zeigt einen Hinweis mit «PIN setzen», wenn das Hauptprofil keine PIN hat und ein anderes Profil des Kontos eine Altersgrenze (`maxRating` unter 18). Nur Empfehlung, keine Pflicht; die Angaben liefert `listProfiles` schon. Texte unter `profiles.pinAdvice.*`, damit die Apps denselben Hinweis an derselben Stelle zeigen (F-19) |

## Kopplung

Teil 3 von Stufe 1 (K27 Abschnitte 7.2 und 10, K26 4.3 und 4.6). Formate und `info`-Werte stehen im
Logbuch unter «Protokolle»; die Testvektoren in `testdata/crypto/` halten sie fest.

| Punkt | Entscheid |
|---|---|
| Gutschein ausstellen | `createPairingVoucher` (`POST /account/pairing-vouchers`, `account`, `x-clients: [app]`). Nur eine App-Sitzung: eine Browser-Sitzung bekommt 403 `app_session_required`, eine Gerätesitzung schon in der Middleware 403 `client_not_allowed`. Nur ohne aktives Profil oder mit dem Hauptprofil (F-23, `primary_profile_required`). Eingabe: der X25519-Schlüssel des Fernsehers aus dem QR-Code (`tvKey`, 32 Byte base64url, ein gültiger Punkt) |
| Gutschein | 256 Bit Zufall in base64url, gültig 2 Minuten, einmal verwendbar, an `tvKey` gebunden, trägt die Art `tv`. Der Server speichert nur den SHA-256 (Tabelle `pairing_vouchers`, Migration 00083), höchstens einen offenen je Konto: ein neuer ersetzt den alten. Höchstens 6 Gutscheine je Minute und Konto. Die Aufräumung der Sitzungen löscht abgelaufene |
| Einlösen | `redeemPairingVoucher` (`POST /auth/pairing/redeem`, `public`): Gutschein, `tvKey`, Gerätename, Systemfassung. Der Server liest und löscht den Gutschein in einer Anweisung (`DELETE … RETURNING`); von gleichzeitigen Anfragen bekommt ihn nur eine. Danach prüft er Ablauf und Schlüssel. Ein Gutschein mit fremdem Schlüssel ist damit auch verbraucht: wer ihn kennt, aber den Schlüssel nicht hat, ist schon ein Zeichen eines Angriffs |
| Antwort | Gerätesitzung über `StartAppSession` mit der Art `tv` (Gerätezeile sofort mit Name und Art, die Systemfassung als `user_agent`, neue Gerätekennung). Profil wie bei `appLogin`: ein einziges Profil ohne PIN ist gleich aktiv, sonst zeigt der Fernseher die Profilwahl. `sealedToken` = HPKE (RFC 9180, Base-Modus, DHKEM(X25519, HKDF-SHA256), HKDF-SHA256, AES-256-GCM, `info` `slouchcat-redeem-v1`, `aad` leer) an `tvKey`, Klartext `{"token","deviceId"}`, aus der Standardbibliothek `crypto/hpke`. Dazu `accountEmail` für die Anzeige am Fernseher (K-9). `Cache-Control: no-store`, nie ein Cookie (auch kein Gerätecookie) |
| Fehler | Unbekannt, abgelaufen, verbraucht und fremder Schlüssel ergeben dieselbe Antwort 410 `voucher_invalid`; der Fernseher zeigt in jedem Fall einen neuen Code. Nur das Log nennt den Grund (ohne den Gutschein). Jeder Versuch zählt in einem eigenen Topf je Adresse des Aufrufers (10 je Minute), nicht im Topf von `login` |
| Meldung | Nach jeder Kopplung geht `device_linked` über **alle eingeschalteten Kanäle des Kontos** (E-Mail, Telegram, Push), unabhängig von den gewählten Ereignissen: sie meldet einen neuen Zugang zum Konto und ist darum nichts, was man abwählt. Sie nennt, was der Server selbst weiss (Zeitpunkt, Art des Geräts), und gibt Name und Systemfassung als Angabe des Geräts wieder (K-7). Texte des Servers in `de`, `en` und `de-DE`; die übrigen Sprachen fallen bis zur Übersetzungsrunde auf Englisch zurück |
| Kleinster Weg für die Meldung | Die Liste in der App (`notifications`) hängt an einem Profil, und jede Art dort braucht einen Titel oder einen Wunsch (Migration 00070). Die Meldung gehört aber zum Konto. Darum bekommt nur `deliveries` das Ereignis `device_linked` (Migration 00084, Umbau nach Vorlage); die Liste in der App und die Ereignisse der Kanäle (`EventType`, `account_channels.events`) bleiben, wie sie sind. «Meine Geräte» zeigt das neue Gerät ohnehin |
| Logs | Die Redaktionsliste kennt `voucher`, `pickup`, `pickupsecret`, `pickuphash`, `pairid`, `meetingpoint`, `meetingpointid`, `packet`, `sealedtoken` und `fcmtoken` (K-8) |

Bekannte Grenzen (K27 7.2, K26 4.6), unverändert:

- **Untergeschobener QR-Code:** Wer einem Kontoinhaber das Bild eines eigenen Codes schickt und ihn zum
  Scannen bringt, koppelt seinen Fernseher. Die Warnung in der Handy-App (K-5), die Erlaubnisliste (K-6)
  und die Meldung (K-7) machen es sichtbar und begrenzen den Schaden; verhindern lässt es sich nicht.
- **Untergeschobenes Paket:** Wer die Treffpunkt-Kennung kennt, kann dem Fernseher ein Paket für einen
  fremden Server ablegen. Lesen kann er nichts; der Fernseher zeigt Server und Konto (K-9).
- **HTTP:** Läuft der Server ohne TLS, schützt die Versiegelung nur diese eine Antwort. Jede spätere
  Anfrage trägt das Gerätetoken offen, und es gilt, bis jemand den Fernseher abmeldet oder das Passwort
  ändert; die Meldung schlägt dabei nicht an.
- Der Gutschein selbst reist nur im versiegelten Paket. Die Bindung an den Schlüssel schützt vor einem
  Gutschein, der doch abgefangen wird (K-3): einlösen kann ihn nur, wer den privaten Schlüssel hat.

## Server weist sich aus

F-38 nach der Empfehlung (K27 Abschnitt 6).

| Punkt | Entscheid |
|---|---|
| Schlüssel | Ed25519, abgeleitet aus dem Geheimnis des Servers: `secrets.Keyring.Derive("server-identity")` (HKDF-SHA256 über `secret.key`) ist der Seed. Kein zweites Schlüsselfile: derselbe `secret.key` ergibt nach jedem Neustart denselben Schlüssel, ein neuer `secret.key` (Neuinstallation, Wiederherstellung ohne Geheimnisse) einen neuen |
| Vertrag | `getHealth.serverKey` (öffentlicher Schlüssel, base64url) und `proveServer` (`POST /server/prove`, `public`): Anfrage `{"nonce"}` mit 32 Byte, Antwort `{"signature","serverKey"}`; unterschrieben wird UTF-8 `slouchcat-server-proof-v1:` gefolgt von den Bytes der Zufallszahl. 30 Beweise je Minute und Adresse des Aufrufers, sonst 429. Nie ein Cookie |
| Kopplung | Das Paket der Handy-App trägt `serverKey`; der Fernseher prüft den Server so schon vor dem Einlösen |

Was es leistet: Eine App, die sich den Schlüssel beim Einrichten gemerkt hat, erkennt in einem fremden
Netz einen anderen Rechner unter derselben Adresse und sendet ihm weder Token noch Passwort.

Was es nicht leistet: Es schützt nicht vor jemandem, der im richtigen Netz mitliest (E-18). Ist der echte
Server auch aus dem Internet erreichbar, kann ein fremdes Gerät die Prüfung an ihn weiterreichen und danach
mitlesen; dagegen hilft nur HTTPS. Wer eine Sicherung mit `secret.key` erbeutet, kann sich als der Server
ausgeben.

Schlüsselwechsel: Ein neuer `secret.key` wechselt den Schlüssel ohne Übergang. Die App meldet dann «Der
Server hat einen neuen Schlüssel» und lässt ihn erst nach einer neuen Anmeldung mit dem Passwort bestätigen
(K27 Abschnitt 6). Ein Wechsel nur des Ed25519-Schlüssels ohne neues Geheimnis ist nicht vorgesehen.

## Folgen

- Vertrag 1.42.0 ist zusätzlich. Neu sind `appLogin`, `AppLoginRequest`, `AppLogin`, `SessionKind`,
  `Health.contractVersion`, `AdminDevice.sessionKind` und `x-clients` an 116 Operationen; mit Teil 3
  `createPairingVoucher`, `redeemPairingVoucher`, `proveServer`, `Health.serverKey` und die Fehlercodes
  `app_session_required` und `voucher_invalid`.
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
