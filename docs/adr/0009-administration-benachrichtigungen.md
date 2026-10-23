# ADR 0009: Administration – Kanäle, Zustellung, Sicherung, Neustart und Statistiken

- Status: angenommen (21. Oktober 2026)
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitte 3.1 (Module `admin`, `notify`, `secrets`), 3.6, 3.7, 3.8 Pkt. 3
  und 4, 5.2, 5.5 und M8; Entscheide vom 21. Oktober 2026 (Kanäle mit echten Zugangsdaten, kein NAS-Ziel) in
  `docs/planning/04-entscheidungen.md`; Spec A-1 bis A-9, E-8; Entwurf in den Sitzungsnotizen `m8-design.md`;
  ADR 0005 (Profilsperre) und ADR 0008 (Dispatcher aus M7)
- Betroffene Spec-IDs: A-1 bis A-9, E-8, K-3, K-8

## Kontext

M8 macht die Verwaltung vollständig: Assistent mit Prüfungen, Dashboard, Einstellungen, Sicherung und
Wiederherstellung, Update-Hinweis, Benachrichtigungskanäle (E-Mail, Web-Push, Telegram), Ankündigungen und
Statistiken. Sechs Fragen waren zu entscheiden:

1. Wem gehören die **Kanäle**, und wer darf sie ändern, wenn Ereignisse Titel nennen, die nicht jedes Profil sehen darf?
2. Wie wird der **Versand** vom Erzeuger des Ereignisses entkoppelt (ein Scan darf nicht auf einen Mailserver warten)?
3. Wie sieht eine **Sicherung** aus, die nie eine Mediendatei enthält und Secrets nur passwortgeschützt mitnimmt?
4. Wie wird eine **Wiederherstellung** auf eine offene SQLite-Datenbank angewendet, ohne dass ein zweiter Prozess oder
   ein Zusatzdienst nötig ist?
5. Wie bleiben **Statistiken** über 365 Tage richtig, wenn Wiedergabesitzungen bisher nach einem Tag gelöscht wurden?
6. Woher kommen **Fehler** für das Dashboard und die Kanäle, ohne eine zweite Protokollierung einzubauen?

Verworfen: (a) Kanäle je Profil – ein Kind müsste eigene Zugangsdaten verwalten, und die Regel 5.5 (Präfix, Prüfung
über `visibility`) wäre je Profil zu wiederholen. (b) Versand direkt im Hook – ein Scan-Batch würde auf SMTP warten,
und ein Ausfall des Mailservers würde den Scanner bremsen. (c) Ein eigener Outbox-Dienst – Zusatzcontainer
(Prinzip 2). (d) Wiederherstellung durch Kopieren über die offene Datenbank – die Verbindungspools halten alte
Dateideskriptoren, und ein Schemawechsel müsste live migriert werden. (e) Wiederherstellung nur über Docker-Neustart –
ein nackter Prozess (Tests, Entwicklung) bliebe ohne Weg. (f) Eine zweite Fehlertabelle – die Logs sind schon
redigiert; ein Ring im Prozess reicht für «letzte 50 Fehler».

## Entscheid

| Punkt | Entscheid |
|---|---|
| Kanäle | Kanäle gehören dem **Konto** (`account_channels` je Kanal: `email`, `push`, `telegram`, mit Ereignisliste `new_content`, `errors`, `invitations`, `announcements`; `acquisition` ist am 22. Oktober 2026 mit M9 entfallen, siehe Nachtrag). Standard nach 5.2: In-App immer, E-Mail für Gäste an (nur Ankündigungen), übrige aus; die Zeilen entstehen mit dem Konto (Migration rückfüllt). Ändern dürfen nur Sitzungen **ohne Profil oder mit dem Hauptprofil** (ADR 0005, 403 `primary_profile_required`), Lesen jedes Profil. Push-Abos hängen am Konto und am Gerät (`push_subscriptions`, VAPID-Paar einmal erzeugt, verschlüsselt in `settings`), Telegram über einen Link-Code (`/start CODE`, 15 Minuten) und die Chat-ID im Kanal |
| 5.5 | Ein Profil-Ereignis geht an die Kanäle des Kontos mit `[Profilname] ` als Präfix und in den In-App-Eingang nur dieses Profils; `notify` prüft vorher `visibility.Visible`. `errors` erreicht nur Hauptkonten und wird je Quelle 60 Minuten dedupliziert |
| Zustellung | `notify.Emit` schreibt je Konto und Kanal eine Zeile `deliveries` und einen Job `notify.deliver` (Priorität 3, fünf Versuche mit Backoff der Queue, Idempotenz je Zustellung). Der Job rendert und sendet über die Sender `notify/email` (net/smtp, TLS/STARTTLS/keine, Zugangsdaten nur verschlüsselt oder an localhost), `notify/push` (RFC 8291/8292 über `webpush-go`, nur https ausser `--dev`, `netguard` gegen interne Ziele, 404/410 löscht das Abo) und `notify/telegram` (Bot-API, Token nur im Pfad, der im Log maskiert wird). Ein Fehlschlag von `notify.deliver` erzeugt kein `errors`-Ereignis (keine Schleife) |
| Hooks | Erzeuger rufen `notify`: neue Folgen (M7), `InvitationAccepted` (auth), `JobFailed` (jobs, beim letzten Versuch), `ScanAborted` (library, Verlustschutz). Ankündigungen (`announcements`, `announcement_acks` je Profil) laufen über denselben Weg |
| Fehler | `logging.ErrorRing` hält hinter der Redaction die letzten 50 `Error`-Einträge im Prozess; das Dashboard zeigt sie mit den endgültig fehlgeschlagenen Jobs der Queue. Ein fehlgeschlagener Trakt-Abruf wird als Error protokolliert |
| Sicherung | tar.gz mit genau `manifest.json`, `slouchcat.db` (VACUUM INTO über eine eigene Leseverbindung) und – nur mit Schlüssel – `secret.key`; nie eine Mediendatei. Ohne Schlüssel entfallen in der Kopie die `v1:`-Settings, der VAPID-Public-Key, Push-Abos, Telegram-Links, PIN-Zähler und Sitzungen (`secure_delete`); private Feed-URLs bleiben (Manifest `includesFeedCredentials`). Mit Schlüssel ist das Archiv age/scrypt-verschlüsselt (Passphrase ≥ 12 Zeichen, Arbeitsfaktor beim Lesen auf 2^20 begrenzt). Automatische Kopien (`pre-migration-*`, `pre-restore-*`) liegen daneben und sind aus der Liste wiederherstellbar |
| Wiederherstellung | Archiv wird entpackt und geprüft (Einträge, Grössen, Manifest-Format, Version und Schema nicht neuer als der Server, `integrity_check`), dann als `slouchcat.db.restore` (+ `secret.key.restore`) bereitgelegt. Nach der Antwort 202 fährt der Server geordnet herunter und ersetzt sich mit `syscall.Exec` durch dieselbe Binärdatei (gleiche PID; schlägt das fehl, Exit 0 für die Neustartregel des Containers). `store.Open` wendet den Stand vor den Migrationen an: aktuelle Datenbank als `pre-restore-*` sichern (bei Fehler verschieben statt kopieren), Dateien tauschen, dann normale Migrationen |
| Update-Hinweis | Job `update.check` liest täglich das Release-Manifest (JSON, ≤ 64 KiB, 10 s), vergleicht semver (`git describe`-Suffixe gelten als Build-Metadaten, `dev` als älter), hält den Stand in `settings`; abschaltbar; der Hinweis reist in `SessionInfo.updateAvailable` für Hauptkonten |
| Statistiken | Schauzeit und meistgesehene Titel aus `progress_events` (seit M8 mit `device_id` des Sitzungsgeräts), Wiedergabeart aus `playback_sessions`; beendete Sitzungen bleiben **400 Tage** (vorher ein Tag), damit 365-Tage-Zeiträume stimmen; abgelaufene Kontositzungen bleiben, solange eine Wiedergabezeile auf sie zeigt (sie autorisieren nichts mehr) |
| Einstellungen | Schlüssel, SMTP-Passwort, Bot-Token und VAPID-Privatschlüssel liegen verschlüsselt in `settings`; Werte aus `deploy/.env` gewinnen und sind in der Oberfläche gesperrt (422 `env_wins`) |

## Folgen

- Vertrag 0.9.0 (145 Operationen): 36 neue Operationen für Assistent-Prüfungen, System, Dashboard, Jobs,
  Einstellungen, Sicherung, Update, Konten, Kanäle, Push, Telegram, Ankündigungen und Statistiken.
- Migration 00016 legt `account_channels`, `telegram_links`, `push_subscriptions`, `deliveries`, `announcements` und
  `announcement_acks` an und füllt die Standardkanäle bestehender Konten.
- Die Redaction-Liste kennt die neuen Schlüsselnamen (`smtppassword`, `bottoken`, `passphrase`, `vapidprivatekey`,
  `p256dh`, `authsecret`, `endpoint`) und maskiert `/bot<token>/` in URLs; der E2E-Lauf scannt das Server-Log auf
  jedes Fake-Secret des Laufs (A-9).
- Der Client enthält weiterhin keine Sichtbarkeitsregel (Prinzip 1): Kanäle, Ankündigungen und Statistiken kommen
  gefiltert vom Server; die Hauptprofil-Regel setzt der Server durch, die Oberfläche blendet nur die Bedienelemente aus.
- Offen: Mehrfachzustellung bei fehlgeschlagener Einreihung nach dem Einfügen (M10), Umwandlung alter
  `progress_events` zu Tageswerten (Plan 3.2, nach 1.0).

## Nachtrag vom 22. Oktober 2026 (M9)

Der Kanal-Ereignistyp `acquisition` ist ersatzlos entfallen. Sein Erzeuger war I-1, der Radarr/Sonarr-Client
mit dem Beschaffungsstatus eines Titels; mit dem Zuschnitt von M9 auf den reinen Webhook-Empfang (Frage 10 im
Entscheidungsprotokoll) gibt es ihn nicht mehr, und ohne Erzeuger wäre der Kanal eine Schaltfläche ohne
Wirkung. Entfernt sind `EventAcquisition` und `AcquisitionChanged` in `internal/model` und `internal/notify`,
die Texte, der Eintrag im Vertrag (`EventType`) und die Oberfläche; die Vorwärts-Migration 00018 baut
`deliveries` ohne den Namen neu auf und putzt ihn aus den gespeicherten Ereignislisten. Die Einstellungsseite
zeigt seither vier Ereignistypen statt fünf. Alles andere in diesem ADR bleibt gültig.
