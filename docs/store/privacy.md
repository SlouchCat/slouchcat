# Privacy policy of the Slouchcat app (draft)

> **Draft of 21 November 2026, not published.** English counterpart of `datenschutz.md`; the
> German draft is the reference, and its notes for the commissioner (unconfirmed points) apply
> here too. **This draft is not legal advice.** The commissioner reviews it, fills in the
> placeholders `<…>` and has it checked where needed before publishing it. The page to publish is
> `datenschutz.html`.

Last updated: `<Datum der Veröffentlichung>`

## 1. Scope

This policy covers the app «Slouchcat» for Android (package `com.slouchcat.app`) on phones,
tablets and TVs, and the two helper services the project runs for the app.

Slouchcat is a media server that everyone runs themselves. The app is its player. It contains no
content and connects to the Slouchcat server whose address you enter. **That server is not run by
the developer of the app**, but by you or by the person who invited you. Whatever happens to your
data there is the responsibility of the server's operator.

## 2. Controller

For the app and the two helper services:

`<Name des Entwicklers>`
`<Postadresse>`
E-mail: `<E-Mail-Adresse für Datenschutz>`

The operator of a Slouchcat server is responsible for the data on that server.

## 3. What the developer does not do

- No account with the developer. The app creates no account anywhere except on your server.
- No advertising, no advertising IDs.
- No analytics, no usage statistics, no crash reports to the developer.
- No sale or transfer of data.
- The two helper services keep nothing permanently (section 6).

## 4. What stays on the device

The app stores on your device: the address of your server and, for unencrypted connections, the
public key your server identifies itself with; the sign-in token and a device ID issued by your
server; saved films, episodes and audiobooks with their subtitles and images, the list of
downloads, offline playback events still to be sent, and a check value for an offline PIN; your
settings.

Only the server address goes into the device backup and a transfer to a new device; sign-in,
downloads and everything else stay on the device. Uninstalling the app deletes everything it
stored. When you sign out, the app asks whether to delete the downloads.

## 5. What goes to your server

The app sends to the server whose address you enter, and to no server of the developer: your
e-mail address and password when you sign in, then the sign-in token with every request; the
device ID, the kind and name of the device (for example maker, model and Android version) and the
app version; what the device can play, so the server picks the right playback; what you do:
chosen profiles, searches, watchlist, playback progress, downloads and settings; and your
device's internet address, as with any connection.

If the app connects to the server without HTTPS, this information travels unencrypted. The app
warns about it. We recommend HTTPS.

## 6. The two helper services

The project runs two small services that only pass on encrypted data. They have no accounts and no
database and write no internet addresses into their logs. To limit requests per caller they keep
the address briefly in memory. They run `<Ort der zwei Dienste>`.

**Pairing relay for setting up a TV** (`pair.slouchcat.com`). When you set up a TV by scanning,
the phone app leaves a packet there that only the TV can decrypt, and the TV picks it up. The
service sees the internet addresses of TV and phone, the time and the encrypted packet. It does
not see your server's address, your account or the voucher inside the packet. A packet lives in
memory for five minutes at most. Setting up a TV by hand does not involve the service.

**Notification forwarder** (`push.slouchcat.com`). So that notifications of your server arrive
while the app is closed, the app registers its Google push token with this service and gives your
server a push address that holds the token in encrypted form. Your server sends every
notification encrypted to that address, and the service passes it on to Google's push service.
The service sees: when registering, your device's internet address and the push token; with every
notification, your server's internet address, the time, the frequency and the encrypted
notification. It does not see the title or text of a notification or your account. It stores none
of it.

## 7. Google

- **Push (Firebase Cloud Messaging):** Google delivers the notifications. Google sees the push
  token, the time and the encrypted notification, not its content and not your server's address.
  Google's library on the device exchanges data with Google for this. Google's privacy policy
  (<https://policies.google.com/privacy>) and the Firebase information
  (<https://firebase.google.com/support/privacy>) apply. Google may process data in the United
  States. The build without Google libraries (`foss`, not in the Play Store) does not use this.
- **QR scanner:** The «Google code scanner» of Google Play services reads the QR code of a TV. The
  app receives no camera image, only the code that was read.
- **Play Store:** Google processes installation, updates and purchases through Google Play under
  its own policy. The developer sees only aggregated figures in the Play Console.

## 8. Legal bases

Under the EU General Data Protection Regulation (GDPR): we process the data the app needs for its
functions (sections 4 to 7) because you use the app and request the function (Art. 6(1)(b) GDPR).
The brief rate limiting per internet address protects the services against abuse (legitimate
interest, Art. 6(1)(f) GDPR). Under the Swiss Federal Act on Data Protection (revFADP) we process
personal data only for these purposes and only as long as needed.

## 9. Your rights

Under the revFADP and the GDPR you have the right of access, rectification, erasure, restriction,
data portability and objection. Because the two services store nothing, the developer usually
holds no data about you to hand over or erase. Your data on a Slouchcat server is managed by its
operator; there you can also sign devices out («My devices»). You delete the data on the device by
signing out or uninstalling the app.

Requests to the developer: `<E-Mail-Adresse für Datenschutz>`. You may also complain to a
supervisory authority: in Switzerland the Federal Data Protection and Information Commissioner
(FDPIC), in the EU the authority of your place of residence.

## 10. Children

The app is meant for holders of an account on a Slouchcat server, not for children. An account
may have child profiles with an age limit; the account holder sets them up, and the server
enforces the limits. The app collects nothing about children beyond what it handles for any other
profile.

## 11. Changes

If what the app or the services process changes, we update this policy and the date above. The
current version is at `<Adresse der Datenschutzerklärung>`.

---

## Noch einzutragen

Dieselben Platzhalter wie in `datenschutz.md`: `<Datum der Veröffentlichung>`,
`<Name des Entwicklers>`, `<Postadresse>`, `<E-Mail-Adresse für Datenschutz>`,
`<Ort der zwei Dienste>` (englisch formuliert, zum Beispiel «on own hardware in Switzerland»),
`<Adresse der Datenschutzerklärung>`.
