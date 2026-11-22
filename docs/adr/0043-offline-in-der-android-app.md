# ADR 0043: Offline in der Android-App

- Status: gebaut auf dem Zweig `claude/k27-stufe6-a` (22. November 2026, Stufe 6 von Konzept 27),
  **angenommen mit dem Merge**. Stufe 7 (Hörbücher und Podcast-Folgen) nutzt dasselbe Modul.
- Quelle: Konzept `docs/planning/27-android-handy-app.md`, Abschnitte 2.4, 8.4, 9.1 bis 9.7, 12,
  13 und die Fragen F-25, F-26, F-34 (es gilt die Empfehlung); Messbericht
  `docs/perf/K27-android-handy.md` Punkte 5 bis 7; ADR 0039 (native Apps im Repo), ADR 0040
  (Sitzungsarten und Gerätekennung)
- Kein Vertrag des Servers, keine Migration: Die App nutzt die Offline-Operationen der PWA, wie sie
  sind (`getDownloadOptions`, `createDownloads`, `syncOffline`, `getDownloadManifest`,
  `getDownloadFile`, `getDownloadSubtitle`, `getDownloadTrickplaySheet`, `deleteDownload`,
  `getDownloadRules`, `setDownloadRules`, `uploadOfflineEvents`, `checkOfflineRevocations`).
- Harte Regeln: 10 (PIN und Verifier nie in Logs), 12 (offline nur Nachschlagen)

## Kontext

Die PWA lädt Titel seit M5 herunter: der Server bereitet eine fMP4-Datei mit Fragmentindex vor,
das Gerät holt sie mit einem Download-Token, der Abgleich hält beide gleich. Die native App soll
dasselbe können, aber mit dem, was Android besser kann: Abruf im Hintergrund, verlässliche
Verbindungsart, Start ohne Netz. Stufe 0 hat gemessen, wie die Datei spielt und wie ein langer
Abruf im Hintergrund überlebt.

## Entscheid

1. **Ein Modul `:core:offline`** für beide Ton- und Bildwege. Es enthält die Datenbank, die Regeln
   des Abgleichs (dieselben wie `web/src/features/offline/sync.ts`), den Abruf, die Offline-PIN,
   die Schauzeit, die lokale Abspielliste und den Hintergrund. Keine Oberfläche.
2. **Abspielweg: lokale HLS-Liste aus dem Fragmentindex** (Messung Punkt 5). Ist eine Datei
   vollständig, schreibt die App `media.m3u8` neben `media.mp4` (`EXT-X-MAP` und
   `EXT-X-BYTERANGE` auf dieselbe Datei). Der Offline-Player ist der Player von Stufe 5 mit einer
   anderen `PlaybackServer`-Umsetzung (`OfflinePlaybackServer`): `start` antwortet aus dem
   Manifest mit `file://`-Adressen, Fortschritt geht in die Warteschlange der Offline-Ereignisse
   und in die Position auf dem Gerät, `state` meldet nur die aufgebrauchte Schauzeit. Media3 liest
   Dateien über `DefaultDataSource`, das Netz weiter über den einen OkHttp-Client.
3. **Abruf im Hintergrund** (Messung Punkte 6 und 7): Was der Nutzer startet, läuft ab Android 14
   als vom Nutzer gestartete Übertragung (`JobInfo.setUserInitiated`, dauerhaft über Neustarts),
   davor als Vordergrunddienst der Art `dataSync`; beide mit Mitteilung und «Anhalten». Was ein
   Auto-Download ist oder auf WLAN wartet, holt WorkManager in Läufen von höchstens 9 Minuten; ein
   periodischer Lauf gleicht etwa alle 6 Stunden ab, wenn Netz da ist (F-34). Jeder Lauf setzt mit
   `Range` und `If-Range` an der Länge fort, die zuletzt mit `sync()` gesichert wurde (alle 8 MiB);
   nach einem Absturz kürzt die App die Datei auf diese Länge. Eine 200-Antwort auf ein Fortsetzen
   heisst: neue Kopie im Server; die App holt das Manifest neu und beginnt die Datei von vorn. Der
   Abruf läuft als Kind des Auftrags, der ihn startet: Beendet das System einen Auftrag oder einen
   Lauf von WorkManager, endet der Abruf mit ihm, und erst nach dem Schliessen der Datei beginnt der
   nächste. Ein Lauf von WorkManager hält an seiner Frist auch mitten in einer Datei an. Wacht ein
   Auftrag in einem neuen Prozess auf (nach einem Neustart), wartet die Engine, bis die App die
   gespeicherte Sitzung gelesen hat.
4. **Speicher:** Datenbank (`offline.db`) und Dateien (`offline/<id>/`) liegen in
   `noBackupFilesDir`: keine Berechtigung, weg mit der App, nie in einer Sicherung oder
   Geräteübertragung, unabhängig von den Sicherungsregeln. Unverschlüsselt wie in der PWA. Keine
   eigene Obergrenze und keine SD-Karte (F-26); die Seite zeigt belegten und freien Platz, der
   Dialog warnt bei zu wenig Platz und lässt dann nicht starten.
5. **Regel 12 offline:** Die App speichert `allowedProfileIds` je Download und zeigt einen Download
   nur diesen Profilen. Die zwei erklärten Ausnahmen gelten wie in der PWA: Offline-PIN gegen einen
   Verifier, Schauzeit nach Geräteuhr.
6. **Offline-PIN:** PBKDF2-HMAC-SHA256 der Plattform mit den Werten von `testdata/crypto/pbkdf2.json`,
   Vergleich mit `MessageDigest.isEqual`. Der Verifier der eigenen Offline-PIN kommt vom Server; den
   der normalen PIN leitet die App nach der nächsten geglückten PIN-Eingabe mit Netz selbst ab, nur
   wenn ein Download des Profils auf dem Gerät liegt. Bremse: zwei freie Fehlversuche, dann 30 s,
   doppelt bis 15 Minuten. Die Pause läuft nur auf der Uhr seit dem Start des Handys; jede Prüfung
   speichert ihren Rest, und nach einem Neustart läuft dieser Rest auf der neuen Uhr von vorn. Weder
   ein vorgestelltes Datum noch ein Neustart beendet eine Pause; während der Pause sind Tastenfeld und
   «Öffnen» gesperrt. Zähler und Verifier liegen im selben Datensatz in `noBackupFilesDir`: Wer die
   App-Daten löscht, löscht auch den Verifier, eine Sicherung bringt die Bremse nicht zurück. Eine
   kurze PIN ist offline durchprobierbar, und wer den Verifier der normalen PIN kopiert, findet damit
   auch die PIN, die online gilt; die Seite sagt beides und verweist auf die eigene, längere
   Offline-PIN (Schutzziel «neugieriges Kind»). Die Ableitung nach der Profilwahl läuft im Hintergrund.
7. **Widerruf und Abgleich:** Beim Start, alle 60 s im Vordergrund, wenn das Netz zurückkommt, nach
   jeder Bestellung und im periodischen Hintergrundlauf. `revoked` löscht sofort Datensatz und
   Dateien; ohne Sitzung (401, Konto entzogen, Gerät unter «Meine Geräte» abgemeldet) fragt die App
   `checkOfflineRevocations` mit der Gerätekennung im Kopf und löscht, was der Server nennt.
   Löschungen ohne Antwort warten als `pendingDelete`. Ein Handy ohne Kontakt behält die Dateien
   (AN-4).
8. **Offline-Zustand:** Ohne Kontakt gilt die App erst nach zwei Fehlschlägen in Folge oder ohne Netz.
   Gilt das beim Start einer Sitzung (Start der App, Anmeldung, Profilwahl) und liegt ein Download des
   Profils vollständig auf dem Gerät, öffnet die App von selbst «Downloads» mit dem Hinweis «Kein
   Kontakt zum Server». Später springt sie nie mehr, sie zeigt dort nur den Hinweis. Antwortet der
   Server wieder, bleibt der Nutzer, wo er ist, und die App meldet «Wieder mit dem Server verbunden».
   Ein anderes Profil öffnet die Offline-Bibliothek nur mit seiner PIN; die Wahl gilt bis zum nächsten
   Profilwechsel, und der Offline-Player spielt nur für das so geöffnete Profil oder das der Sitzung.
9. **Besitzer der Downloads:** der Schlüssel des Servers und die Konto-ID (`getSession`), nicht Adresse
   und E-Mail: Ein anderer Server unter derselben Adresse oder ein anderes Konto mit derselben E-Mail
   öffnet behaltene Downloads nicht, eine geänderte E-Mail-Adresse sperrt sie nicht.
10. **Abmelden (F-25):** Das Blatt «Abmelden?» fragt, ob die Downloads gelöscht werden, ab Werk ja.
   Ja: die Dateien weg, `deleteDownload` je Download gleichzeitig und mit kurzer Frist, solange das
   Token gilt. Was der Server nicht bestätigt, merkt sich die App je Konto und sendet es bei der
   nächsten Anmeldung dieses Kontos vor dem ersten Abgleich; bis dahin kommen diese Downloads nicht
   zurück. Nein: Die Dateien bleiben gesperrt, bis sich dasselbe Konto wieder anmeldet (Regel der
   PWA). Der Hintergrund endet.

## Folgen

- Neue Bibliotheken: Room 2.8.5 (KSP 2.3.14) und WorkManager 2.12.0, beide Apache-2.0.
- Neue Berechtigungen der Handy-App: `POST_NOTIFICATIONS`, `RUN_USER_INITIATED_JOBS`,
  `FOREGROUND_SERVICE`, `FOREGROUND_SERVICE_DATA_SYNC`, `RECEIVE_BOOT_COMPLETED`. Die Erklärung des
  Vordergrunddienstes steht in `docs/store/datensicherheit.md` 2.2.
- Die Mitteilung des Abrufs ist privat: Auf dem Sperrbildschirm steht nur «Download läuft», der
  Titel nur, wenn das aktive Profil den Download sehen darf.
- Was der Vertrag nicht hergibt, zeigt die App nicht: das Budget des Servers für vorbereitete Kopien
  (im Mockup) und Regeln je Serie oder Podcast (der Vertrag kennt eine Regel je Profil und Gerät).
- Unbelegt bis zur Gerätecheckliste: 20 GB über Nacht auf einem echten Handy, WorkManager unter der
  Akku-Optimierung eines Herstellers.
