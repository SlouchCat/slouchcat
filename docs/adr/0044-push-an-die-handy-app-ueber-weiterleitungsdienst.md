# ADR 0044: Push an die Handy-App über einen Weiterleitungsdienst

- Status: Dienst gebaut auf dem Zweig `claude/k27-stufe2-a` (21. November 2026, Stufe 2 von
  Konzept 27), **angenommen mit dem Merge**. Die Anpassungen des Servers (Satzgrösse, Abo je
  App-Sitzung) baut Stufe 1, die App Stufe 8.
- Quelle: Konzept `docs/planning/27-android-handy-app.md`, Abschnitte 2.5, 2.7, 7.3, 7.4 (D-4,
  D-6, D-10), 7.6, 7.7; Marcos Entscheid E-17; Logbuch des Baus («Protokolle»)
- Kein Vertrag des Servers, keine Migration
- Harte Regeln: 10 (Firebase-Schlüssel, Schlüssel zum Versiegeln, Google-Token und Push-Adresse in
  der Redaktionsliste), 11 (Ausnahme vom Nicht-Ziel «Cloud»)

## Kontext

Die Handy-App soll Meldungen auch bei geschlossener App bekommen (E-17). Auf Android geht das nur
über Googles Push-Dienst (FCM), und der nimmt Nachrichten nur von einem Absender mit dem
Dienstschlüssel des Firebase-Projekts der App an. Dieser Schlüssel kann nicht in jedem selbst
gehosteten Server liegen. Der Server kann heute Web-Push an Browser (RFC 8030, 8291, 8292) und
löscht ein Abo, wenn der Push-Dienst mit 404 oder 410 antwortet.

## Entscheid

1. **Ein Weiterleitungsdienst, der zum Server hin Web-Push spricht.** Der Server sendet an die App
   wie an einen Browser: verschlüsselt für den Schlüssel der App (`aes128gcm`), mit VAPID
   unterschrieben. Der Dienst reicht den unveränderten Körper als Datennachricht an FCM HTTP v1
   weiter (`data.wp` = Base64 mit Füllung, RFC 4648 Abschnitt 4). Dringlichkeit `high` wird zur
   Android-Priorität `HIGH`, sonst `NORMAL`; `TTL` wird zur Haltedauer, höchstens vier Wochen.
2. **Keine Datenbank: die Push-Adresse trägt alles.** `POST /v1/register` nimmt Google-Token und
   VAPID-Schlüssel des Servers und gibt `https://<Name>/v1/push/<Fassung>.<Blob>` zurück. Blob =
   base64url(Nonce 12 Byte ‖ AES-256-GCM über `{"t":…,"k":…}`), Zusatzangabe
   `slouchcat-push-v1:<Fassung>`. Der Schlüssel je Fassung steht in einer Schlüsseldatei
   ausserhalb des Images (D-6); neue Adressen versiegelt die aktuelle Fassung, alte öffnen alle
   eingetragenen. `push -add-key` legt eine neue Fassung an.
3. **VAPID-Prüfung (D-4):** Der Schlüssel `k` im Kopf muss der versiegelte sein, die
   ES256-Unterschrift muss mit ihm stimmen, `aud` muss einer der eigenen Ursprünge aus
   `RELAY_PUSH_ORIGINS` sein, `exp` muss in der Zukunft und höchstens 24 Stunden entfernt liegen.
   Sonst 403, mit einer Ausnahme: Stimmt die Unterschrift, fehlt ihr `aud` aber in
   `RELAY_PUSH_ORIGINS`, ist das ein Fehler der eigenen Einstellung und 503 (Code-Review, Minor 5).
4. **410 nur, wenn Google das Token nicht mehr kennt** (`errorCode` `UNREGISTERED`). **Jeder eigene
   Fehler ist 503** (D-10): fehlende, falsche oder unbekannte Fassung der Schlüsseldatei, fehlender
   Firebase-Schlüssel, Fehler von Google oder des Netzes, auch `INVALID_ARGUMENT`, weil es auch eine
   fehlerhafte Nachricht des Dienstes bedeuten kann. 404 kommt auf dem Push-Weg nie vor, auch
   nicht für eine unsinnige Adresse, weil der Server auch bei 404 löscht. Ohne Geheimnisse startet
   der Dienst trotzdem und meldet `/v1/healthz` mit 503.
5. **Grenzen (D-2):** Körper höchstens `RELAY_PUSH_MAX_BODY` Byte (Vorgabe 2700: die Satzgrösse,
   mit der ein Server an die Push-Weiterleitung sendet, Stufe 0; Base64 macht 3602 Byte daraus,
   Google nimmt 4096), sonst 413. `TTL` ist Pflicht (RFC 8030), sonst 400; über vier Wochen wird er
   gekürzt. Ratenbegrenzung je Aufrufer und je /48 oder /24, für Anmeldungen ebenso, und je
   Push-Adresse erst nach gültiger Unterschrift, sonst 429. Davor Google ein Vorrat für den ganzen
   Dienst (Vorgabe 2000 je Sekunde): Ist er leer, antwortet der Dienst 503. Er schützt das
   Kontingent des Firebase-Projekts vor einer Flut aus vielen Netzen (Sicherheits-Review M-2). Eine
   volle Ratentabelle weist niemanden ab, ein beliebiger Eintrag weicht.
6. **Zugang zu Google:** Der Dienst tauscht eine mit dem Dienstkonto (RS256) unterschriebene
   Zusicherung gegen ein Zugangstoken und hält es bis kurz vor dem Ablauf. Die Adressen von FCM
   und des Token-Endpunkts sind einstellbar, damit der Nachbau `relay/cmd/fcmfake` sie in Tests und
   Proben ersetzt. Der Nachbau wird nie ausgeliefert.
7. **Der Dienst entschlüsselt nie.** Das Paket für RFC 8291 (`relay/internal/webpush`) nutzen nur
   die Prüfprogramme und die Tests; depguard verbietet es der Push-Weiterleitung.

8. **Regel für den Server (Sicherheits-Review M-3):** Für Adressen mit dem Ursprung der
   Push-Weiterleitung gilt nur 410 als «Abo weg»; 404 ist dort ein Fehler mit Wiederholung wie 503.
   Ein Reverse-Proxy ohne passenden Eintrag oder ein Server mit veraltetem DNS bekommt sonst ein
   Standard-404 und löscht Abos massenhaft, genau was D-10 verhindern soll. Der Dienst selbst
   antwortet auf dem Push-Weg nie 404; der Proxy davor darf es für diesen Namen auch nicht
   (BETRIEB.md 1.1 und Umzug Schritt 6). Den Serverteil baut Stufe 1.
9. **Zugang zu Google ohne Stau:** Das Zugangstoken holt ein einziger Abruf ausserhalb der Sperre mit
   eigenem Zeitlimit (10 s); wartende Meldungen brechen mit ihrem eigenen Kontext ab, ein
   Fehlschlag bleibt 5 s gemerkt, jede Sendung hat 10 s (Code-Review, Major 1).

## Verworfen

- **Ein eigener Sendeweg des Servers an FCM:** verlangte den Firebase-Schlüssel in jedem Server.
- **Eine Datenbank der Abos beim Dienst:** brächte Zustand, der umziehen muss, und eine Liste von
  Servern und Geräten an einem Ort (D-1).
- **410 bei jeder Adresse, die der Dienst nicht öffnen kann:** Eine falsche Schlüsseldatei
  löschte binnen Minuten die Abos aller Server, die gerade senden (D-10).
- **Benachrichtigungs-Nachrichten statt Datennachrichten:** Googles Bibliothek zeigte sie ohne die
  App an; der Text käme vom Dienst und nicht vom Server des Nutzers (K27 7.4).

## Folgen

- Der Server (Stufe 1) erkennt Adressen dieses Ursprungs, setzt für sie die kleinere Satzgrösse
  und lässt eine App-Sitzung ihr Abo verwalten. Die App (Stufe 8) meldet sich mit Google-Token und
  `getPushPublicKey` an, trägt die Adresse mit ihrem Web-Push-Schlüssel ein, entschlüsselt
  `data.wp` und folgt nur Meldungen, die sie selbst entschlüsselt hat.
- Wer den Firebase-Schlüssel erbeutet, kann Nachrichten an alle Handys mit der App schicken (R-4);
  er liegt nur auf der Maschine der Push-Weiterleitung.
- Wer die Schlüsseldatei verliert, macht alle Push-Adressen unbrauchbar, bis sich jede App neu
  angemeldet hat. Sie gehört gesichert und zieht beim Umzug mit (ADR 0042, BETRIEB.md).
- Der Dienst sieht die Adresse jedes sendenden Servers, Zeitpunkt und Häufigkeit; gespeichert
  wird davon nichts (K27 7.6).
