# ADR 0040: Sitzungsarten für native Apps und Gerätekopplung

- Status: angenommen (21. November 2026, Stufe 1 von Konzept 27, Teil 1; die Abschnitte «Kopplung» und
  «Server weist sich aus» mit Teil 3 derselben Stufe). **Nachtrag zu ADR 0005** (drei Sitzungsarten, Ablauf, Aufräumung
  der Geräte).
- Quelle: Konzept `docs/planning/27-android-handy-app.md` (K27), Abschnitte 2.3, 4, 11.5 und 15.1;
  Konzept `docs/planning/26-android-tv.md` (K26), Abschnitte 4.2, 4.5 und Anhang B; Marcos Entscheide
  E-7, E-8, E-14, E-15, E-16 und F-4; Frage 34 und Frage 35 in `docs/planning/04-entscheidungen.md`
- Vertrag: 1.42.0 (zusätzlich); Migrationen `00082_session_kind.sql`, `00083_pairing_vouchers.sql`,
  `00084_device_linked_deliveries.sql`, `00085_pairing_issuer_push_progress.sql`
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
| Träger | Browser-Sitzung im Cookie, App- und Gerätesitzung in `Authorization: Bearer`. Ein Browser-Token als Bearer, ein App- oder Geräte-Token im Cookie zählen wie kein Token. Cookie und Bearer zusammen beantwortet der Server auf jeder Route mit 401 `unauthenticated`. Nur das Schema `Bearer` (gross oder klein, ein oder mehr Leerzeichen) betritt den Bearer-Weg; jedes andere Schema, etwa `Basic` eines Reverse-Proxys vor der Web-App, zählt wie kein Kopf. Ein Token auf dem falschen Träger berührt seine Sitzung nicht (kein Verlängern, kein letzter Kontakt) |
| Ablauf | Browser 30 Tage gleitend wie bisher. App und Gerät bekommen den Ablauf `9999-12-31` (`auth.NeverExpires`), kein leerer Wert; `TouchSession` verschiebt ihn bei diesen Arten nicht und hält nur den letzten Kontakt fest |
| Kein Cookie | Eine Anfrage mit `Authorization: Bearer`, mit `X-Slouchcat-Device` oder an `appLogin`, `redeemPairingVoucher` und `proveServer` bekommt nie ein `Set-Cookie` (weder Sitzung noch Gerät), ein 401 löscht kein Cookie. `logout` einer App- oder Gerätesitzung entzieht die Sitzung ohne Cookie und ohne `Clear-Site-Data` |
| CSRF | Der Kopf `X-Requested-With: Slouchcat` bleibt Pflicht für jede schreibende Anfrage, auch mit Bearer |
| Erlaubnisliste | Jede `account`- und `scoped`-Operation nennt in `x-clients` die Bearer-Arten, die sie aufrufen dürfen (`app`, `tv`). Fehlt die Art, antwortet der Server 403 `client_not_allowed`; `admin`- und `owner`-Operationen tragen keine Liste und sind darum für jede Bearer-Art gesperrt. Die Middleware liest die Liste zur Laufzeit aus dem eingebetteten Vertrag (`clientsOf` neben `visibilityOf`); für die Streaming-Routen steht sie in `binaryRoutes`. Der Schema-Test verlangt die Liste für `account` und `scoped` und verbietet sie sonst |
| Stand 1.42.0 | `app` an allen 116 Operationen des Nutzerbereichs (24 `account`, 92 `scoped`), `tv` an den 93 aus K26 Anhang B. Was dort Vorschlag ist (Empfehlen, Benachrichtigungen, Ankündigungen, Vorlieben der Wiedergabe ja; Kontodaten, Sehbericht, Meldekanäle nein, K26 F-18), gilt nach der Regel der Papiere als Empfehlung |
| Medien-Adressen | Die Routen mit eigenem Token (`st`, `dt`, `rt`) lesen den Bearer nicht. Ihr Token gibt nur eine Operation aus, die die Liste prüft (`startPlayback`, `getDownloadManifest`, `startRadioListen`) |
| App anmelden | `appLogin` (`POST /auth/app-login`, `public`): E-Mail, Passwort, Gerätename, Art (`phone`, `tablet`, `tv`), optional die Gerätekennung der App. `phone` und `tablet` ergeben eine App-Sitzung, `tv` eine Gerätesitzung. Antwort: Token (256 Bit, nur als Hash gespeichert), Gerätekennung und `SessionInfo`, mit `Cache-Control: no-store`. Fehlversuche zählen im Topf von `login`, das Passwort geht wie bei `login` an Seerr |
| Gerät | Die Gerätezeile entsteht beim Anmelden mit Name und Art. Ein Abspielstart einer App- oder Gerätesitzung schreibt Fähigkeiten und Besitzer, aber nicht Name und Art (`recordDevice`). Gerätenamen verlieren an allen drei Eingängen (`appLogin`, Einlösen, Fähigkeitsprofil) Steuer- und Formatzeichen (Bidi, Nullbreite, `model.CleanDeviceName`) |
| Gerätekennung | vergibt der Server. Eine mitgebrachte Kennung gilt nur, wenn sie die Form hat, nicht die reservierte `ffff…` ist und kein anderes Konto auf dem Gerät eine lebende Sitzung oder einen Download hat (`DeviceHeldByOtherAccount`); sonst vergibt er eine neue. Prüfung, Gerätezeile und Sitzung laufen in einer Schreibtransaktion. Eine neue App- oder Gerätesitzung eines Kontos entzieht die älteren App- und Gerätesitzungen desselben Kontos auf demselben Gerät samt Wiedergabe. Mit Bearer kommt das Gerät aus der Sitzung, auch für `checkOfflineRevocations`; ohne Sitzung nennt die App es im Kopf `X-Slouchcat-Device` (`checkOfflineRevocations`; bei `appLogin` gilt der Kopf, wenn der Körper keine Kennung trägt) |
| Aufräumung | `DeleteStaleDevices` verschont Geräte mit einer lebenden App- oder Gerätesitzung, die Aufräumung der Widerrufsliste (`DeleteOldTombstones`, 400 Tage) deren Einträge. Ist das Gerät einer App- oder Gerätesitzung trotzdem weg (in der Verwaltung vergessen), entzieht der Server die Sitzung und antwortet 401; eine neue Kennung gibt es dafür nicht (`loadSession`) |
| Verträglichkeit | `getHealth` nennt `contractVersion` (`info.version`). Die Apps nennen sich im Kopf `X-Slouchcat-Client` (`mobile/1.2.3`, `tv/1.2.3`). Ist die Version kleiner als die Mindestfassung der App (`api.DefaultMinClientVersions`, heute `0.0.0` für beide), antwortet der Server mit 426 `app_too_old` (gemeinsame Antwort `AppTooOld` an jeder Operation mit `x-clients`), ausser auf `getHealth`, `checkOfflineRevocations` und `logout`: eine zu alte App erfährt Widerrufe und kann sich abmelden. Kein `Upgrade`-Kopf, weil das Mittel eine neue App ist, kein anderes Protokoll. Die Routen mit eigenem Token (`st`, `dt`, `rt`) prüfen die Fassung nicht; ihr Token gibt nur eine Operation aus, die sie prüft. Ein fehlender oder unlesbarer Kopf und ein unbekannter App-Name zählen nicht |
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
| Gutschein | 256 Bit Zufall in base64url, gültig 2 Minuten, einmal verwendbar, an `tvKey` gebunden, trägt die Art `tv` und hängt an der App-Sitzung, die ihn ausgestellt hat (Migration 00085). Der Server speichert nur den SHA-256 (Tabelle `pairing_vouchers`, Migrationen 00083 und 00085), höchstens einen offenen je Konto: ein neuer ersetzt den alten. Höchstens 6 Gutscheine je Minute und Konto. Die Aufräumung der Sitzungen löscht abgelaufene |
| Einlösen | `redeemPairingVoucher` (`POST /auth/pairing/redeem`, `public`): Gutschein, `tvKey`, Gerätename, Systemfassung. Der Server liest und löscht den Gutschein in einer Anweisung (`DELETE … RETURNING`); von gleichzeitigen Anfragen bekommt ihn nur eine. Danach prüft er Ablauf, Schlüssel und dass die ausstellende Sitzung noch lebt: `logout`, «Meine Geräte» und ein neues Passwort beenden einen offenen Gutschein mit dem Handy (das neue Passwort löscht ihn zusätzlich). Den Schlüssel prüft er beim Ausstellen und beim Einlösen streng: genau 43 Zeichen kanonisches base64url, 32 Byte, kein Punkt niedriger Ordnung (Probe-ECDH). Ein Gutschein mit fremdem Schlüssel ist damit auch verbraucht: wer ihn kennt, aber den Schlüssel nicht hat, ist schon ein Zeichen eines Angriffs |
| Antwort | Gerätesitzung über `StartAppSession` mit der Art `tv` (Gerätezeile sofort mit Name und Art, die Systemfassung als `user_agent`, neue Gerätekennung). Profil wie bei `appLogin`: ein einziges Profil ohne PIN ist gleich aktiv, sonst zeigt der Fernseher die Profilwahl. `sealedToken` = HPKE (RFC 9180, Base-Modus, DHKEM(X25519, HKDF-SHA256), HKDF-SHA256, AES-256-GCM, `info` `slouchcat-redeem-v1`, `aad` leer) an `tvKey`, Klartext `{"token","deviceId","accountEmail"}` (die E-Mail für die Anzeige am Fernseher, K-9, liest nur er), aus der Standardbibliothek `crypto/hpke`. Dazu `signature`: Ed25519 des Servers über UTF-8 `slouchcat-redeem-signature-v1:` und die Bytes von `sealedToken`; der Fernseher prüft sie mit dem `serverKey` des Pakets, so kann niemand auf dem Weg ein eigenes, für den öffentlichen Schlüssel versiegeltes Token unterschieben. `Cache-Control: no-store`, nie ein Cookie (auch kein Gerätecookie) |
| Fehler | Unbekannt, abgelaufen, verbraucht und fremder Schlüssel ergeben dieselbe Antwort 410 `voucher_invalid`; der Fernseher zeigt in jedem Fall einen neuen Code. Nur das Log nennt den Grund (ohne den Gutschein). Jeder Versuch zählt in einem eigenen Topf je Adresse des Aufrufers (10 je Minute), nicht im Topf von `login` |
| Meldung | Nach jeder Kopplung geht `device_linked` als **Sicherheitsmeldung** hinaus, unabhängig von den gewählten Ereignissen und vom Schalter der Kanäle: per E-Mail an die Adresse des Kontos (nicht an eine im Kanal eingetragene, die eine gestohlene Sitzung geändert haben könnte), sobald der Server SMTP hat, dazu an jeden eingerichteten Kanal (verknüpftes Telegram, jedes Push-Abo), auch wenn er ausgeschaltet ist. Die Zustellung hängt nicht an der Anfrage des Fernsehers (`context.WithoutCancel`) und nutzt die fünf Versuche der Zustellung; Push wird je Abo wiederholt. Link `/settings/devices`. Sie nennt, was der Server selbst weiss (Zeitpunkt, Art des Geräts), und gibt Name und Systemfassung als Angabe des Geräts wieder (K-7). Texte des Servers in `de`, `en` und `de-DE`; die übrigen Sprachen fallen bis zur Übersetzungsrunde auf Englisch zurück |
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
- Der Gutschein selbst reist nur im versiegelten Paket. Die Bindung an den Schlüssel schützt nicht vor dem
  Einlösen, nur vor dem Nutzen: Wer Gutschein und öffentlichen Schlüssel kennt (der steht im QR-Code), kann
  einlösen, ohne den privaten Schlüssel zu haben. Er bekommt dann ein Token, das er nicht lesen kann, und die
  Konto-Adresse steht nur im versiegelten Klartext. Folge: eine Gerätesitzung, die niemand benutzen kann und
  die unter «Meine Geräte» steht, die Meldung `device_linked` geht hinaus, und der echte Fernseher bekommt 410
  und zeigt einen neuen Code. Ein Besitznachweis des privaten Schlüssels (zweite Anfrage mit Unterschrift)
  hätte das verhindert; er kostet einen Durchgang mehr, und der Gutschein reist ohnehin nur versiegelt.
  Darum bleibt es bei der Bindung (Sicherheits-Review L-2).
- **Keine Meldung ohne Kanal:** Hat der Server weder SMTP noch einen eingerichteten Kanal des Kontos (Telegram
  verknüpft, ein Push-Abo), geht die Meldung nirgends hin; der Server schreibt das als Fehler ins Log, und nur
  «Meine Geräte» zeigt das neue Gerät.
- **Hinter einem Reverse-Proxy** ohne `SLOUCHCAT_TRUSTED_PROXIES` sieht der Server jede Anfrage von der
  Adresse des Proxys. Dann teilen sich alle Clients die Töpfe von `proveServer` (30 je Minute) und
  `redeemPairingVoucher` (10 je Minute): wer sie leert, lässt die Apps in einem neuen Netz «Server nicht
  erreichbar» melden (ein Ausfall, kein Einbruch; der Topf von `login` hat dasselbe Problem). Darum gehört
  `SLOUCHCAT_TRUSTED_PROXIES` zu jedem Betrieb hinter einem Proxy (`docs/INSTALLATION.md`). Ein höherer oder
  zweiter, globaler Topf schützt nicht: er läuft genauso leer.

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

### Nachtrag K27 Stufe 3: die Handy-App (Entscheid des Baus, weicht vom Wortlaut von K27 Abschnitt 6 ab)

Das Sicherheits-Review der App (H-1) hat gezeigt: Ein Blatt «neuer Schlüssel», das sich von selbst öffnet
und zur neuen Anmeldung führt, gibt dem fremden Gerät im fremden Netz mit einem Tipp das Passwort, also mehr
als ohne Prüfung. Der Koordinator des Baus hat darum entschieden:

| Punkt | Entscheid |
|---|---|
| Anderer Schlüssel | Unterschreibt die Gegenstelle gültig mit einem anderen Schlüssel, sendet die App nichts und zeigt still «Server nicht bestätigt» dort, wo sonst Daten stünden. Kein Dialog, keine Aufforderung zur Anmeldung |
| Bestätigen | Nur ausdrücklich unter «Mehr › Server» und, abgemeldet, auf der Anmeldeseite über denselben Baustein: Fingerabdruck des gemerkten und des neuen Schlüssels (die ersten 16 Byte von SHA-256 des Schlüssels), die zwei Fälle (Server neu installiert oder aus einer Sicherung ohne Geheimnis wiederhergestellt gegen fremdes Netz mit derselben Adresse), der Hinweis, dass E-Mail und Passwort danach an das Gerät mit dem neuen Schlüssel gehen, und der Schalter «Ich bin in meinem eigenen Netz». Erst dann die neue Anmeldung; währenddessen gehen weder Token noch Gerätekennung mit. Der Schlüssel gilt, sobald der Server das Passwort angenommen hat |
| Neu einrichten | Ein anderer Schlüssel unter derselben Adresse zählt wie ein anderer Server (H-2): Token, Gerätekennung und jede Prüfung des alten Schlüssels gehen weg. Die Einrichtung fragt dafür mit demselben Baustein nach |
| Sperre je Sprung | Zusätzlich zur Sperre vor der Anfrage prüft ein Netzwerk-Interceptor jeden Sprung zum Server (Wiederholung, Weiterleitung) gegen das aktuelle Netz (M-2) |
| Netzwechsel | Neues Standardnetz, Verlust, geänderte Adressen oder Routen, und jedes Netz ausser VPN, das kommt oder geht. Dazu prüft die App spätestens nach 10 min und nach jedem Verbindungsabbruch neu (M-1) |

Bekannte Grenze: Ein Always-on-VPN, das auch das lokale Netz trägt und darunter nichts Sichtbares
ändert, verdeckt einen Wechsel des WLAN. Die Prüfung nach Zeit und nach Verbindungsabbruch mildert das;
ganz schliessen kann es nur HTTPS.

Nicht gebaut, der stärkere Weg: ein zweiter Beweis des Servers über den Hash eines gültigen Tokens
(der Server unterschreibt die Zufallszahl mit `HMAC(sha256(token), nonce)`). Ein Server, der aus einer
Sicherung ohne Geheimnis kommt, kennt die Hashes noch; ein fremdes Gerät nicht. Damit liesse sich ein neuer
Schlüssel ohne Passwort bestätigen. Es braucht eine neue Server-Operation und gehört in eine spätere Stufe
des Servers.

### Nachtrag 23. November 2026, erster Teil: was als Verbindungsabbruch zählt

Die Zeile «Netzwechsel» sagt, die App prüfe «nach jedem Verbindungsabbruch neu» (M-1). Gebaut war: nach
jeder `IOException` einer Anfrage. Dazu zählte auch, was die App selbst beendet: ein abgebrochener Aufruf
(das Bild einer verlassenen Seite, ein Segment beim Spulen) und ein Aufruf, dessen Thread unterbrochen wurde
(eine abgebrochene Coroutine). Der verworfene Beweis liess Anfragen scheitern, die schon unterwegs waren
(«the network changed before the request left»), in einem Netz, das sich nicht geändert hatte. Gemessen in
`docs/perf/K27-android-ci.md`, Abschnitt «Rote Fälle im Emulator-Job: Ursachen», Unterabschnitt «Behoben: die
App verwarf den Serverbeweis nach einem abgebrochenen Aufruf».

| Punkt | Entscheid |
|---|---|
| Abbruch durch die App | Zählt nicht als Verbindungsabbruch, der Beweis bleibt. Erkannt wird er am abgebrochenen Aufruf (`Call.isCanceled`, sofern der Aufruf kein eigenes Zeitlimit hat) oder an einer `InterruptedIOException`, die keine `SocketTimeoutException` ist, auf einem unterbrochenen Thread |
| Zeitlimit und Bruch | Unverändert: Ein Zeitlimit von OkHttp (`SocketTimeoutException`, auch das Zeitlimit eines ganzen Aufrufs, das OkHttp als Abbruch meldet), eine abgelehnte, zurückgesetzte oder geschlossene Verbindung verwerfen den Beweis. Zeitlimit und Reset auch auf einem unterbrochenen Thread; die eine Ausnahme dort ist eine ordentlich geschlossene Verbindung (siehe unten) |
| Netz und Alter | Unverändert: Jeder Sprung prüft das aktuelle Netz und das Alter des Beweises (10 min), bevor Token und Gerätekennung dazukommen |
| Zeitlimit um einen Aufruf | Ein Zeitlimit, das als Verbindungsabbruch zählen soll, gehört in OkHttp (`readTimeout`, `callTimeout`). Ein Zeitlimit der Coroutine um den Aufruf (`withTimeout`) unterbricht den Thread wie ein Abbruch und lässt den Beweis stehen. Heute betrifft das eine Stelle: das Löschen der Downloads beim Abmelden (`OfflineEngine.beforeSignOut`, 5 s) |

**Was die Änderung für die Sicherheit kostet.** Sie wirkt nur dort, wo sich das Netz ändert, ohne dass die
App es sieht, also in der bekannten Grenze oben (ein Always-on-VPN verdeckt den WLAN-Wechsel). Dort war ein
Abbruch durch die App mehr als eine Nebenwirkung des Blätterns: Hängt eine Anfrage nach dem verdeckten
Wechsel und verlässt der Nutzer die Seite, bevor das Zeitlimit von OkHttp greift (10 s beim Verbinden, 30 s
beim Lesen), hat bisher dieser Abbruch den Beweis verworfen, und die nächste Anfrage hat neu bewiesen. Jetzt
bleibt der Beweis, und die nächste Anfrage geht mit Token an das Gerät unter der Adresse. Dasselbe gilt, wenn
die App einen hängenden Aufruf über ein Zeitlimit der Coroutine aufgibt, und wenn ein Abbruch mit einem
echten Fehler zusammenfällt: Ein Aufruf gilt beim Fang schon als abgebrochen, und eine Verbindung, die die
Gegenstelle ordentlich schliesst, meldet Okio auf einem unterbrochenen Thread als Unterbrechung (ein Reset
bleibt ein Bruch). Der Beweis gilt in diesen Fällen bis zum nächsten Verbindungsabbruch, den das Gate sieht,
höchstens 10 min. Der zweite Teil dieses Nachtrags (unten) begrenzt das auf 1 min: Ein Abbruch setzt den
Beweis in Zweifel.

### Nachtrag 23. November 2026, zweiter Teil: jeder Sprung merkt seinen Bruch, Zweifel nach einem Abbruch

Der erste Teil liess eine Lücke offen und nannte Kosten. Dieser Teil schliesst beides und zwei weitere Stellen
derselben Art. Geändert ist nur `ServerIdentityGate` (`android/core/api`).

**Was nicht stimmte.** Jede Stelle ist mit einem Test belegt, der vor der Änderung rot war.

1. «Nach jedem Verbindungsabbruch neu» (M-1) galt nur für Brüche, die bis zur Anfrage durchdrangen. Bricht
   eine Verbindung aus dem Vorrat, wiederholt OkHttp die Anfrage selbst auf einer neuen Verbindung, unterhalb
   der Stelle, an der das Gate den Bruch sah. Die Wiederholung trug Token und Gerätekennung mit dem alten
   Beweis. Das ist der Fall, den M-1 meint: Das fremde Gerät unter der Adresse beantwortet die alte Verbindung
   mit einem Reset und nimmt die neue an. Selten ist der Weg nicht: Der Server schliesst unbenutzte
   Verbindungen nach 2 min, OkHttp behält sie 5 min, also trifft nach einer Pause von 2 bis 5 min die erste
   Anfrage eine tote Verbindung.
2. Bricht die Verbindung, während die Antwort gelesen wird (ein Segment im Player, eine Datei beim
   Herunterladen), endete keine Anfrage, die das Gate sah. Wer liest, fragt mit einer neuen Anfrage nach, und
   die fand den alten Beweis. Beim Bau dieser Änderung gefunden.
3. Wurde die Anfrage abgebrochen, die gerade den Beweis holte, galt der Beweis als gescheitert: 5 s lang
   scheiterte jede Anfrage sofort, ohne dass der Server gefragt wurde.
4. Die Kosten des ersten Teils: Ein Abbruch durch die App löste keinen neuen Beweis mehr aus, der alte galt
   bis zu 10 min.

| Punkt | Entscheid |
|---|---|
| Bruch je Sprung | Der Netzwerk-Interceptor merkt sich selbst, wie sein Sprung endet, auch während die Antwort gelesen wird. Ein Bruch verwirft den Beweis dort, also bevor OkHttp wiederholt. Was als Bruch zählt und was als Abbruch durch die App, regelt weiter der erste Teil |
| Verbindungen im Vorrat | Mit dem Beweis schliesst das Gate die unbenutzten Verbindungen der App (`ConnectionPool.evictAll`), wie die App es beim sichtbaren Netzwechsel tut. Sie stammen alle aus der Zeit vor dem Bruch; OkHttp nähme sonst die nächste und bräche wieder. Das trifft die unbenutzten Verbindungen zu jedem Host des gemeinsamen Clients, enger kann der Vorrat es nicht; es kostet dort einen neuen Verbindungsaufbau |
| Beweis unterwegs | Ein Sprung ohne gültigen Beweis holt einen neuen und geht dann weiter, wenn das Netz noch das ist, in dem die Anfrage losging. Die Grenze ist der zweite eigene Bruch: Bricht auch die Verbindung nach dem neuen Beweis, endet die Anfrage. Ein Beweis, den der Bruch einer anderen Anfrage gekostet hat, zählt dabei nicht mit (brechen mehrere alte Verbindungen nacheinander, verliert eine Anfrage den Beweis zuerst an den fremden und dann an den eigenen Bruch). Scheitert der Beweis, endet sie als «nicht erreichbar», bevor Token und Gerätekennung dazukommen. So bleibt eine tote Verbindung des echten Servers unsichtbar, und an ein fremdes Gerät geht nach dem Bruch keine Anfrage mehr |
| Wiederholung | Bleibt bei OkHttp. Das Gate sendet keine Anfrage selbst ein zweites Mal; es lässt die Wiederholung von OkHttp erst nach dem neuen Beweis hinaus |
| Netzwechsel | Unverändert: Hat sich das Netz geändert, seit die Anfrage losging, scheitert der Sprung sofort und ohne neuen Beweis. Was aus der Anfrage wird, entscheidet die App |
| Bruch aus einem alten Netz | Wie eine Anfrage endet, die vor einem Netzwechsel losging, sagt in den 30 s danach (dem Lesezeitlimit) nichts über das Netz von jetzt: kein Verwerfen, kein Zweifel. Nach einem Netzwechsel brechen die alten Verbindungen oft mehrere zugleich; das neue Netz hat seinen eigenen Beweis. Später zählt ihr Bruch wieder: Nicht jeder Wechsel nimmt die Verbindungen mit (mobile Daten kommen dazu, das WLAN bleibt), und eine Verbindung, die ihn überlebt hat, bricht aus einem eigenen Grund |
| Beweis von vor dem Bruch | Ein Beweis, der begonnen hat, bevor eine Verbindung brach, zählt nicht, auch wenn er danach gültig endet: Der Server hat vor dem Bruch unterschrieben. Die Anfrage fragt dann noch einmal, so lange, wie sie auch auf den Beweis einer anderen wartet (15 s); drei Runden gelten nur noch für ein Netz, das sich immer wieder ändert, und für Beweise, die immer wieder abgebrochen werden |
| Abgebrochener Beweis | Unterbricht die App den Thread der Anfrage, die den Beweis holt, ist das kein Urteil über den Server: kein Fehlschlag, keine Sperre, und ein Beweis, der erneuert werden sollte, bleibt, wie er war. Wartende beginnen eine neue Runde. Ein Zeitlimit oder ein Verbindungsfehler des Beweises bleibt ein Fehlschlag, auch auf einem unterbrochenen Thread |
| Zweifel | Ein Abbruch durch die App lässt den Beweis stehen und setzt ihn in Zweifel. Ist ein Beweis im Zweifel 1 min alt, beweist die nächste neue Anfrage zuerst neu. Anfragen, die schon unterwegs sind, laufen mit dem alten weiter. Scheitert die Erneuerung, ist der Beweis weg, auch für sie |
| Gescheiterte Erneuerung | Jeder Fehlschlag nimmt den Beweis weg, auch ein Verbindungsfehler oder eine Antwort 429 oder 500: Ein fremdes Gerät antwortet genau so wie ein Server, dem es kurz schlecht geht. Das kostet im eigenen Netz im seltenen Fall 5 s «nicht erreichbar». Nur der Beweis desselben Schlüssels geht weg: Scheitert ein neuer Schlüssel, den der Nutzer gerade bestätigt, bleibt der Beweis des gemerkten |
| «Server wechseln» | `reset` zählt wie ein Bruch: Ein Beweis, der davor begonnen hat, zählt nicht |

**Wie viele Beweise das kostet.** Der Server erlaubt 30 Beweise je Minute und Adresse. Ein Abbruch durch die
App löst nie selbst einen Beweis aus; ein Beweis im Zweifel wird höchstens einmal je Minute erneuert, wie oft
die App auch abbricht. Ein Bruch kostet einen Beweis, nach einer Pause von 2 bis 5 min also einen je
Wiederaufnahme. Brechen mehrere Verbindungen kurz nacheinander (eine Seite lädt nach der Pause fünf Dinge
zugleich, oder der Server kam von einem Update zurück), zählt jeder Bruch für sich, und es werden höchstens
so viele Beweise, wie Verbindungen brachen; nach einer Pause sind das höchstens die fünf, die OkHttp
unbenutzt behält. Keine Anfrage scheitert daran. Eine Gegenstelle, die unterschreibt und dann jede Verbindung abbricht, kostet je Anfrage
zwei Beweise statt einen; bei 30 je Minute antwortet der Server mit 429, das gilt als Fehlschlag, und das
Gate antwortet 5 s lang selbst.

**Was es für die Sicherheit heisst**, in der bekannten Grenze (ein Always-on-VPN verdeckt den WLAN-Wechsel):

- Die erste Anfrage nach dem verdeckten Wechsel geht auf einer alten Verbindung hinaus, mit Token. Wer die
  Adresse des Servers hält, kann sie lesen, auch wenn sein Gerät die Verbindung nicht kennt und mit einem
  Reset antwortet; wer im fremden Netz mitliest, ebenso. **Das Gate nimmt das nicht zurück.** Der Bruch
  schützt alles, was danach käme, und damit vor einem fremden Gerät, das nicht gezielt mitliest. Gegen eines,
  das es tut, ist das Token mit dieser einen Anfrage weg. Das war immer so.
- Antwortet das fremde Gerät auf eine alte Verbindung mit einem Reset oder schliesst es sie, verwirft der
  Bruch den Beweis, bevor eine Anfrage eine neue Verbindung zu ihm benutzt. Bisher ging auch die Wiederholung
  mit Token an das fremde Gerät, auf einer Verbindung, die es angenommen hatte.
- Schweigt das fremde Gerät zu alten Verbindungen (eine Firewall verwirft Pakete, die zu keiner Verbindung
  gehören) und nimmt neue an, hängt die Anfrage. Wartet die App das Zeitlimit von OkHttp ab (30 s beim Lesen),
  ist es ein Bruch. Gibt der Nutzer vorher auf, ist der Beweis im Zweifel: War er 1 min alt, beweist die
  nächste Anfrage neu und scheitert; war er jünger, gilt er, bis er 1 min alt ist. Vor dem ersten Teil hat
  hier jeder Abbruch den Beweis verworfen, mit dem ersten Teil galt er bis zu 10 min.
- Hält die App beim verdeckten Wechsel keine Verbindung (nach 5 min ohne Verkehr), bricht nichts. Dann gilt
  der Beweis, bis er 10 min alt ist, und die nächste Anfrage geht mit Token auf einer neuen Verbindung an das
  fremde Gerät. Das war immer so; die Prüfung nach Zeit ist die einzige, die hier greift.
- Zwischen der Prüfung im Sprung und dem Senden liegen wenige Befehle. Ein Bruch, den eine andere Anfrage
  genau dort bemerkt, kommt für diesen Sprung zu spät. Das war immer so.

Was bleibt, ist die bekannte Grenze selbst mit drei Fenstern: die erste Anfrage auf einer alten Verbindung,
bis zu 10 min ohne jeden Bruch, wenn die App beim Wechsel keine Verbindung hielt, und bis zu 1 min nach einem
Abbruch, wenn das fremde Gerät schweigt. Ganz schliessen kann sie nur HTTPS.

Nicht gebaut:

- Eine eigene Wiederholung des Gates, die eine Anfrage nach dem neuen Beweis selbst noch einmal sendet. Das
  Gate müsste entscheiden, was sich wiederholen lässt; OkHttp weiss das schon.
- Ein neuer Beweis für eine Anfrage, die über einen sichtbaren Netzwechsel hinweg unterwegs ist: Sie
  scheitert wie bisher.

Den «Verfall nach Ruhe», den das Sicherheits-Review dieser Änderung vorschlug, regelt der dritte Teil.

### Nachtrag 23. November 2026, dritter Teil: Verfall nach Ruhe

Marco am 23. November 2026: bauen. Der Vorschlag kam aus dem Sicherheits-Review des zweiten Teils (Befund
M-1): Was dort blieb, war vor allem die Pause. Ein verdeckter Netzwechsel fällt meist in eine Zeit, in der
die App nichts sendet (der Nutzer geht in ein anderes Netz). Danach ging die erste Anfrage mit Token hinaus,
auf einer alten Verbindung oder, nach 5 min, auf einer neuen, und der Beweis galt bis zu 10 min.

| Punkt | Entscheid |
|---|---|
| Verfall nach Ruhe | Ein Beweis zählt nicht mehr, wenn 60 s lang kein Sprung zum Server hinausging (`SlouchcatHttp.PROOF_IDLE`). Die erste Anfrage nach einer solchen Pause beweist zuerst neu; scheitert der Beweis, geht nichts hinaus, auch nicht das erste Paket |
| Was als Benutzung zählt | Ein Sprung, der mit gültigem Beweis hinausgeht. Nicht das Warten auf eine Antwort, nicht das Lesen eines langen Antwortkörpers, nicht die Frage, ob der Beweis gilt. Wer nur eine lange Antwort liest (ein Radiostrom), beweist mit der nächsten Anfrage neu |
| Dauer im Vorrat | Der gemeinsame Client behält eine unbenutzte Verbindung 90 s (`SlouchcatHttp.KEEP_ALIVE`) statt der 5 min von OkHttp. Kürzer als die 2 min des Servers: Eine Pause endet nicht mehr an einer Verbindung, die der Server geschlossen hat, sie kostet genau den einen Beweis und keinen Bruch. Länger als die 60 s des Verfalls: Solange ein Beweis ohne neuen gilt, hält die App ihre alten Verbindungen noch, und an deren Bruch fällt ein verdeckter Wechsel auf. Ein Test hält beide Abstände fest |
| Übrige Regeln | Unverändert: höchstens 10 min, je Netz, Bruch, Zweifel nach 1 min, Beweis unterwegs. Ein Sprung, dessen Beweis inzwischen verfallen ist, holt ihn wie jeder Sprung ohne Beweis |

**Was es kostet.** Einen Beweis nach jeder Pause über 60 s, also eine Antwortzeit des Servers mehr vor der
ersten Anfrage; alle Anfragen, die in dem Moment losgehen, teilen ihn. Mehr als einer je Minute kann daraus
nicht werden. Die Kosten des zweiten Teils nach einer Pause von 2 bis 5 min (ein Beweis je gebrochene
Verbindung, höchstens fünf) fallen weg.

**Was es für die Sicherheit heisst**, in der bekannten Grenze:

- Nach einer Pause über 60 s geht nichts mehr ohne neuen Beweis hinaus. Das Fenster «erste Anfrage auf einer
  alten Verbindung» gibt es dann nicht mehr, und das Fenster «bis zu 10 min ohne jeden Bruch» ist auf 60 s
  seit der letzten Anfrage geschrumpft.
- Es bleibt der Wechsel mitten in der Benutzung, also weniger als 60 s nach der letzten Anfrage. Die nächste
  Anfrage geht mit Token hinaus, in der Regel auf einer alten Verbindung, und wer die Adresse des Servers
  hält, kann sie lesen. Bricht das fremde Gerät die alte Verbindung oder schweigt es, greifen Bruch und
  Zweifel wie im zweiten Teil.
- Antwortet das fremde Gerät statt dessen, bricht nichts. Das kann es, wenn die App in dem Moment keine alte
  Verbindung hält (der Server hat sie mit `Connection: close` beendet, oder die fünf Plätze im Vorrat waren
  anders belegt) und es die neue annimmt und irgendetwas antwortet, oder wenn es sich gezielt in die alte
  Verbindung einschaltet. Dann hält jede weitere Anfrage den Beweis in Benutzung, er gilt, bis er 10 min alt
  ist, und jede Anfrage trägt das Token. Die 60 s begrenzen nur, wann dieses Fenster beginnen kann, nicht wie
  lange es dauert; hier greift allein die Prüfung nach Zeit.

Nicht gebaut: eine kürzere Höchstdauer des Beweises, solange die App den Server benutzt (heute 10 min). Sie
kürzte das letzte Fenster und kostete laufend Beweise, auch im eigenen Netz. Das wäre ein eigener Entscheid.

Ganz schliessen kann die Grenze weiter nur HTTPS.

### Nachtrag 24. November 2026: was die App nach einem Netzwechsel unterwegs tut

Die Regel «Netzwechsel» lässt offen, was aus einer Anfrage wird, die ein Netzwechsel unterwegs beendet. Gebaut
war nichts: Die Anfrage endete als «nicht erreichbar» (`ApiResult.Offline`), und jede Seite zeigte die Meldung
mit «Erneut versuchen», bis der Nutzer tippte. Keine Seite fragt von selbst neu, wenn das Netz wieder da ist.

**Der Fall.** CI-Lauf 45253038403 (PR #221), `PlayerFlowTest.filmAndEpisodes`: Direkt nach der Anmeldung
meldete der Emulator dreimal einen Netzwechsel (neues Standardnetz, zweimal neue Adressen, zusammen 136 ms).
`listMyProfiles` ging mit dem Beweis aus dem zweiten Netz los, der dritte Wechsel kam vor dem Sprung, und das
Gate hielt die Anfrage an («the network changed before the request left»). Danach kam kein Wechsel mehr, das
Netz erreichte den Server. Die Profilwahl zeigte «nicht erreichbar», der Test wartete 20 s auf die Profile und
tippte nie auf «Erneut versuchen». Auf einem echten Handy geschieht dasselbe bei jedem Wechsel, den der Nutzer
nicht sieht (WLAN wird bestätigt, eine IPv6-Adresse kommt oder geht): eine Seite meldet «nicht erreichbar»,
obwohl der Server erreichbar ist. Das ist ein Fehler der App, nicht nur des Tests.

| Punkt | Entscheid |
|---|---|
| Lesende Anfragen | Eine Anfrage mit GET, HEAD oder OPTIONS, die scheitert, nachdem sich das Netz seit ihrem Start geändert hat, geht einmal neu hinaus (`SlouchcatHttp.NetworkChangeRetry`). Das gilt für den Halt durch das Gate wie für eine Verbindung, die mit dem alten Netz bricht. Für Seiten, Bilder, Player und Downloads gleich |
| Wo | Im gemeinsamen Client vor dem Gate. Der zweite Versuch geht wie jede neue Anfrage durch das Gate und wartet auf den Beweis des neuen Netzes. Das Gate bleibt, wie es ist: Es wiederholt nichts selbst, und ein Sprung über einen Netzwechsel hinweg scheitert weiter |
| Schreibende Anfragen | Nie durch diese Wiederholung. Der Sprung kann den Server erreicht haben, bevor die Verbindung brach; ob die Seite ihn noch einmal senden darf, weiss nur sie. Sie zeigt weiter ihre Meldung (die Profilwahl «nicht erreichbar», das Profil bleibt antippbar). Unverändert bleibt die eigene Wiederholung von OkHttp nach einem Verbindungsfehler (`retryOnConnectionFailure`), auch für Schreibendes: über HTTP nur im selben Netz und nach einem neuen Beweis am Sprung, über HTTPS ohne Gate |
| Nicht wiederholt | Ein Abbruch durch die App (abgebrochener Aufruf, unterbrochener Thread), ein anderer Schlüssel unter der Adresse des Servers, und jedes Scheitern im selben Netz: Das ist eine Aussage über den Server |
| Wie oft | Einmal. Ändert sich das Netz auch im zweiten Versuch, endet die Anfrage als «nicht erreichbar» |
| Start | Das Netz beim Aufruf, vor dem Warten auf den Beweis. Ändert es sich schon während dieses Wartens und scheitert die Anfrage dann im neuen Netz aus eigenem Grund (Verbindung abgelehnt, Zeitlimit), geht sie trotzdem ein zweites Mal; das kostet höchstens ein Zeitlimit mehr. In welchem Netz der Sprung hinausging, weiss nur das Gate |
| Brüche | Der zweite Versuch ist derselbe OkHttp-Aufruf und erbt im Gate dessen Zahl eigener Brüche. Er bekommt deshalb unter Umständen einen Beweis unterwegs weniger als eine neue Anfrage; das ist die sichere Seite |

**Was es kostet.** Höchstens eine Anfrage mehr je lesende Anfrage, die ein Netzwechsel trifft; die Anfragen
eines Moments teilen sich den Beweis des neuen Netzes. Im selben Netz nichts.

**Was es für die Sicherheit heisst:** nichts Neues. Der zweite Versuch ist eine neue Anfrage im neuen Netz und
braucht dort einen eigenen Beweis, bevor Token und Gerätekennung hinausgehen. Er ist dasselbe, was der Tipp
auf «Erneut versuchen» schon tat.

Nicht gebaut: ein neues Laden der Seiten, wenn das Netz nach einer Zeit ohne Netz zurückkommt. Wo eine Seite
«nicht erreichbar» zeigt, weil es wirklich kein Netz gab, bleibt «Erneut versuchen».

## Folgen

- Vertrag 1.42.0 ist zusätzlich. Neu sind `appLogin`, `AppLoginRequest`, `AppLogin`, `SessionKind`,
  `Health.contractVersion`, `AdminDevice.sessionKind` und `x-clients`; mit Teil 2 `listMyDevices`,
  `signOutMyDevice` und `MyDevice`, mit Teil 3 `createPairingVoucher`, `redeemPairingVoucher`, `proveServer`,
  `Health.serverKey`, die gemeinsame Antwort `AppTooOld` und die Fehlercodes `app_session_required` und
  `voucher_invalid`. `x-clients` steht damit an 119 Operationen: den 116 des Nutzerbereichs von Teil 1 und
  den drei neuen `account`-Operationen (`listMyDevices`, `signOutMyDevice`, `createPairingVoucher`, je `[app]`).
- **Neue Regel neben «Vertrag = Tag» (K27 11.5, Frage F-9):** Operationen mit `x-clients` brechen nicht mehr
  von einem Release aufs nächste; eine brechende Änderung braucht eine Übergangszeit. Frei ändern lässt
  sich nur noch die Verwaltung.
- Der Vertragstest ruft jede Operation mit einem App-Token und einem TV-Token des Besitzerkontos mit
  aktivem Hauptprofil auf (`TestBearerClientsCoverEveryOperation`); fehlt die Art, erwartet er 403
  `client_not_allowed`. Jede falsche Paarung von Token und Träger hat einen Test (`TestBearerCarrierPairings`).
- Bekannte Grenzen aus den Reviews: Ein Download-Token `dt` hängt nicht an der Sitzung (Altbestand, Sicherheits-Review L-5); ein über HTTP mitgelesenes `dt` lädt die vorbereiteten Titel, bis die Server-Kopie weg ist (24 h nach Abruf, 7 Tage ohne). Ein Forward-Auth-Proxy, der selbst `Authorization: Bearer` einsetzt, sperrt Browser aus, weil Cookie und Bearer zusammen abgelehnt werden.
- Ehrlich gesagt (K27 4.2): Welche Art ein Client nennt, entscheidet der Client. Wer das Passwort kennt,
  meldet sich über `login` auch voll an. Die Sitzungsarten begrenzen den Schaden eines verlorenen Geräts oder
  Tokens, nicht den eines bekannten Passworts.
- Ein verlorenes Gerät bleibt angemeldet, bis jemand es abmeldet («Meine Geräte», Teil 2), die Verwaltung es
  vergisst oder das Passwort sich ändert (`updateAccount` entzieht jede andere Sitzung, auch App und Gerät).
