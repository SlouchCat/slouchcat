# ADR 0037: Wünsche über Seerr

- Status: angenommen (17. November 2026, Stufe 0 von Konzept 25, mit Frage 33 und ihrem Nachtrag); **hebt die
  Spec-Nicht-Ziele Zeile 66
  in Teilen und Zeile 67 nur beim Wunsch auf und fasst Zeile 69 mit einer Ausnahme**; Zeile 65 und 68 bleiben.
  **Nachtrag zu ADR 0010** (zwei Ausnahmen: Profil-Tags setzen, Warteschlange und Dateistand der gewünschten Titel
  lesen; der Webhook-Empfang bleibt).
- Quelle: Konzept `docs/planning/25-medienwuensche/` (Papiere 01 bis 11, Fassung 3, nachgeführt auf die Antworten
  vom 17. November 2026), die Prüfung an Marcos echten Instanzen mit seinen Antworten I-F1 bis I-F3
  (`docs/planning/25-medienwuensche/anhang/I-pruefung-echte-instanzen.md`, «Anhang I»), Marcos Entscheide E-1 bis E-11 und seine Antworten auf die Fragen F-1 bis F-34, T-1, T-2
  und B-1 bis B-4 vom 17. November 2026 (`docs/planning/25-medienwuensche/anhang/H-nachtrag-17-november.md`,
  Abschnitte 1, 2, 8, 10 und 11; die Anhänge in `docs/planning/25-medienwuensche/anhang/` heissen hier «Anhang D», «Anhang H» usw.), **Frage 33** mit Nachtrag in
  `docs/planning/04-entscheidungen.md`
- Vertrag und Migrationen: vergibt der Bau; ein Release für alle Stufen (F-22, B-1), der Vertrag steigt einmal,
  brechend (`docs/planning/25-medienwuensche/08-datenmodell-api.md`, Abschnitt 4.6)
- Betroffene Spec-IDs: Spec A-6, Spec I-8; neu Spec I-9, Spec I-10, Spec I-11
- Harte Regeln: 2 (Titelabfragen nur über `visibility`), 3 (kein `os/exec` in `seerr` und `arr`), 5 (Migrationen
  nach Vorlage), 7 (`x-visibility`, Vertragstest), 8 (Tastaturtest), 9 (Texte), 10 (Schlüssel und Passwörter
  redigiert), 11 (Nicht-Ziel mit Ausnahmen), 12 (Entscheid im Server)

## Kontext

Bis 1.40 bindet Slouchcat Seerr nicht an (Frage 10, Plan 5.1): Wer etwas sehen will, das fehlt, wünscht es
in Seerr. Marco will, dass Zuschauer fehlende Filme und Serien in Slouchcat anfordern, den Zustand sehen
und über Slouchcats Kanäle erfahren, wann der Titel da ist. Seerr soll die Wunschverwaltung bleiben:
Freigaben, Limits und die Übergabe an Radarr und Sonarr liegen dort. Nach der Durchsicht der zweiten Fassung
verlangte Marco am 17. November 2026: Eingeladene Konten bekommen in Seerr einen Benutzer mit denselben
Login-Daten; Profile brauchen kein Login, nur einen Tag; ein Limit je Konto; eine Seite für Wünsche wie die
Startseite von Seerr; Serien konsistent über TVDB. Nach der Durchsicht des Mockups fragte er, ob Radarr und Sonarr
direkt angebunden werden sollen, damit ein neuer Titel «rasendschnell» in Slouchcat ankommt, und ob die Webhooks
dafür wegfallen könnten. Radarr und Sonarr melden von sich aus nur per Webhook; ihre API ist rein abfragend.
Mit dem Bauauftrag vom 17. November 2026 hat Marco alle offenen Fragen beantwortet (Anhang H 11.2), bei vier
gegen die Empfehlung der Papiere: keine Ausnahme je Profil (F-27), Sammelbenutzer ab Werk (F-4), Passwort nur für
eingeladene und von Slouchcat eröffnete Benutzer (F-24) und ein Release am Schluss (F-22, B-1).

Zu entscheiden war: wie Slouchcat im Namen einer Person wünscht, obwohl Seerr einen einzigen Schlüssel und
keine Profile kennt; wie ein Wunsch einem Profil gehört; wann ein Titel als verfügbar gilt; wie ein Wunsch
übersteht, dass Seerr nicht erreichbar ist; wie ein Client im Heimnetz zu den Regeln für ausgehende
Aufrufe passt; wie ein Passwort nach Seerr kommt, ohne dass Slouchcat es speichert; wer das Limit zählt; und
wie weit Slouchcat Radarr und Sonarr anfassen darf und wie es von ihnen erfährt.

## Entscheid

1. **Ausnahme vom Nicht-Ziel nur für Wünsche über Seerr, mit zwei eng gefassten Ausnahmen von «Steuerung
   von Arr/Seerr».** Slouchcat legt Wünsche in Seerr an, liest ihren Zustand, zeigt ihn, zieht zurück und
   meldet die Verfügbarkeit. Es legt die Benutzer der Konten in Seerr an und pflegt sie (E-Mail, Passwort,
   Rechte, Limit), und es setzt in Radarr und Sonarr Profil-Tags (anlegen, anhängen). Warteschlange und Dateistand
   der gewünschten Titel (`queue`, `movie?tmdbId`, `series?tvdbId`) liest es nur, für den Download-Fortschritt bei
   Wünschen (Entscheid 19). Der Import in die Bibliothek kommt weiter über den Webhook (Entscheid 20). Keine eigene Beschaffung,
   keine Qualitätsprofile oder Zielordner, keine Änderung der Warteschlange, kein Suchen, Hinzufügen oder Löschen in Radarr und
   Sonarr, kein Entfernen von Tags, kein eigener Freigabe-Ablauf. SABnzbd spricht Slouchcat nie an. Übrige
   Einstellungen von Seerr ändert es nicht; Benutzer löscht es nicht.
2. **Client `internal/seerr`** ohne Datenbank (Vorbild `internal/trakt`): `X-Api-Key` aus den verschlüsselten
   Einstellungen (AES-256-GCM, nie wieder angezeigt, letzte vier Zeichen), Zeitlimit 10 s, höchstens 5 Anfragen/s,
   **keine Weiterleitungen**, nur `http`/`https`. **Ausdrückliche LAN-Erlaubnis:** Der Client läuft nicht über
   `netguard`, weil der Administrator die Adresse setzt (Muster Podcast-Feed des Administrators und SMTP); es
   wird nur diese Adresse angesprochen, nie ein Wert aus einer Antwort.
3. **Im Namen des Benutzers:** `POST /api/v1/request` mit `X-API-User: <Seerr-Benutzer>` und ohne `userId`.
   So gelten Recht, Limit und Auto-Freigabe dieses Benutzers. Der Verbindungstest prüft den Header mit
   `GET /auth/me`; wirkt er nicht, lässt sich die Funktion nicht einschalten (T-1). Beim ersten erfolgreichen
   Test speichert Slouchcat einen Fingerabdruck der Instanz; weicht er später ab, halten Übermittlung und Abgleich
   an, bis der Administrator die Zuordnung bestätigt oder löscht.
4. **Ein Seerr-Benutzer je Konto (E-1, E-4):** Nimmt jemand eine Einladung an, eröffnet Slouchcat im selben
   Zug den Benutzer in Seerr mit gleicher E-Mail und gleichem Passwort (`wish_account_map` mit Herkunft
   eröffnet, verknüpft oder von Hand). Für bestehende Konten gibt es «In Seerr eröffnen» und «Alle
   eröffnen»; das Einrichten von Seerr eröffnet sie nicht von selbst. **Über die E-Mail wird sofort verknüpft
   (E-4, E-8):** Gibt es in Seerr einen Benutzer mit derselben Adresse, verknüpft Slouchcat ihn, gleich ob ein
   Administrator «In Seerr eröffnen» oder «Alle eröffnen» wählt oder das Konto die Adresse beim Annehmen einer
   Einladung oder beim Ändern seiner E-Mail selbst eintippt. Das Passwort geht an ihn nur nach einer Einladung
   oder E-Mail-Änderung (E-8); verknüpft ein Administrator, behält der Benutzer sein Passwort (F-24, N-103);
   nie an Benutzer 1, andere Administratoren von Seerr und Benutzer mit Anmeldung über Jellyfin, Emby oder Plex
   (N-91). Den Auslöser hält `wish_account_map.linked_by` fest; eine Verknüpfung nach Einladung oder
   E-Mail-Änderung zeigt die Übersicht sieben Tage lang als «Neu verknüpft» (Empfehlung, N-93). Das Konto erfährt
   nie, ob Seerr die Adresse kannte (N-94). Profile haben kein Login; sie senden
   als Benutzer des Kontos. Eine Ausnahme je Profil gibt es nicht (F-27, N-100). Konten ohne eigenen Benutzer
   senden über den Sammelbenutzer «slouchcat-gaeste», den Slouchcat anlegt (F-4, N-101, N-102). Ein
   Seerr-Benutzer gehört höchstens einem Konto. Je Wunsch speichert Slouchcat Konto, Profil,
   Seerr-Benutzer (eingefroren) und Request-ID, bei Serien je Staffel. Die Rechte eines eröffneten Benutzers
   folgen dem Wunschrecht: mit Recht Seerrs Vorgabe für neue Benutzer (`defaultPermissions`), ohne Recht keine
   (Marco, I-F1; N-110); die eines verknüpften fasst Slouchcat nicht an. Wird ein Konto
   gelöscht, bleibt sein Seerr-Benutzer bestehen; nur einem eröffneten nimmt Slouchcat die Rechte und setzt
   ein Zufallspasswort, verknüpfte und von Hand gewählte bleiben unverändert (N-10, F-31).
5. **Wunschrecht** je Konto (`accounts.can_request`, Administrator) als Deckel und je Profil
   (`profiles.can_request`, Hauptprofil). Der Server liefert `canRequest` und einen Grund; der Client entscheidet
   nichts. Konten ohne Seerr-Benutzer senden ab Werk über den Sammelbenutzer; mit «Nicht erlauben» können sie
   nicht anfordern (F-4).
   Fehlende Titel und die Wünsche-Seiten sieht ein Profil nur, wenn es anfordern kann (F-9).
6. **Zustände** `queued`, `pending`, `processing`, `partial`, `available`, `declined`, `failed`, `withdrawn`,
   abgeleitet aus Seerr und der eigenen Bibliothek.
7. **Verfügbar ist, was Slouchcat für das Profil abspielen kann.** Auslöser ist ein Haken nach dem
   Metadaten-Match (IDs geschrieben), dazu stündlich `wishes.reconcile`. Abgleich bei Filmen über TMDB-, dann
   IMDb-ID, bei Serien wie die Bibliothek über TVDB-, dann TMDB-, dann IMDb-ID. «AVAILABLE» in Seerr löst
   allein nichts aus. Ein Film ist verfügbar mit der ersten
   abspielbaren Datei; eine Staffel erst, wenn sie vollständig ist (abspielbare Folgen = Folgenzahl laut TMDB,
   bei laufenden Staffeln die bisher ausgestrahlten). Ob es den Titel gibt und wer ihn sehen darf, beantwortet
   `visibility`; `wishes` fragt `items` nicht ab (Regel 2).
8. **Ausgangsfach:** `createWish` speichert lokal (`queued`), versucht sofort (3 s, ohne Vorprüfung) und sonst
   über `wishes.submit` mit eigener Wiederholungsleiter; nach 24 h `failed` und Fehlerereignis an
   Administratoren. Ab dem zweiten Versuch prüft der Job vor dem Senden, ob der Wunsch in Seerr schon
   besteht; 409 «Request for this media already exists.» und 202 zählen als Erfolg. **Zurückziehen** setzt lokal
   `withdrawn`; der Job `wishes.withdraw` sucht einen eigenen Request des Benutzers und löscht ihn, solange er
   `PENDING` ist. Ist er schon freigegeben, bleibt er stehen und der Wunsch wird wieder geöffnet.
9. **Abgleich** `wishes.sync` alle 5 min: `GET /request?sort=modified` absteigend bis zum letzten Abgleich; der
   Zeiger rückt je Seite vor, ein abgebrochener Lauf setzt dort fort. Der erste Abgleich und der nach einem
   Instanzwechsel laufen als Erstabgleich ohne Seitengrenze im Hintergrund. Dazu `GET /request/{id}` für offene
   Wünsche ohne Änderung seit 24 h, höchstens einmal je 24 h je Wunsch. Die Übernahme bestehender Anfragen
   (F-21) läuft als eigener Job `wishes.import`. Der Seerr-Webhook ist nur ein Anstoss (Stufe 4).
10. **Mitwunsch:** In Slouchcat höchstens ein offener Wunsch je Film bzw. Serienstaffel. Ein weiteres Profil wird
    Mitwünscher, ohne Aufruf an Seerr, und wird ebenfalls benachrichtigt. Zuschauer sehen nie, wer sonst
    gewünscht hat.
11. **Benachrichtigungen** nur über Slouchcat: neue Eingangsarten `wish_available`, `wish_season_available`,
    `wish_approved`, `wish_declined`, `wish_failed`; neue Kanal-Ereignisse `wish_available`, `wish_status`,
    `wish_review` (nur Administratoren). Stufe 2 bringt «verfügbar» für Filme und ganze Serien, Stufe 4 die
    Staffel- und Zustandsmeldungen und `wish_review`. Doppelschutz über `wish_requesters.notified_at` und einen
    eindeutigen Schlüssel je Profil. Eingangseinträge ohne Titel in der Bibliothek hängen am Wunsch und sind für
    Anforderer und Mitwünscher sichtbar. Externe Nachrichten von PIN-Profilen nennen keinen Titel.
12. **Freigeben und Ablehnen** für Administratoren als Durchreichen an `POST /request/{id}/approve|decline`
    (Stufe 4). Das ist kein Ablauf «Kind fragt Eltern» (Spec Zeile 65 bleibt).
13. **Verwaltung:** Integrationen › Seerr (Verbindung, Test, Abgleich, Protokoll) und Medien › Wünsche
    (Anfragen, Seerr-Benutzer, Einstellungen); jede Einstellung genau ein Ort. Verwaltungsantworten tragen keine
    `ItemRef`; `listAccountWishes` ist `scoped` und verlangt das Hauptprofil (Regel 7).
14. **Nur online.** `/api` bleibt NetworkOnly; Wünsche haben keinen Offline-Zustand.
15. **Passwort nur in drei Momenten (E-1).** Slouchcat gibt das Passwort an Seerr weiter, wenn es der Server
    ohnehin sieht: beim Annehmen der Einladung, beim Ändern des Passworts und bei der Anmeldung (dann nur, wenn
    es noch aussteht), und nur für Benutzer, die Slouchcat eröffnet oder nach Einladung oder E-Mail-Änderung
    verknüpft hat (F-24, N-103). Es geht nur im Speicher weiter, nie über eine Job-Nutzlast, und Slouchcat speichert und
    protokolliert es nie. Geht es nicht sofort, bekommt der Benutzer ein Zufallspasswort, und das echte folgt bei
    der nächsten Anmeldung. Benutzer 1, Administratoren von Seerr und Benutzer mit Anmeldung über Jellyfin,
    Emby oder Plex gleicht Slouchcat nicht ab. Ein Passwort, das jemand direkt in Seerr ändert, bemerkt
    Slouchcat nicht. Der
    Verbindungstest warnt bei ausgeschalteter «Lokaler Anmeldung» und bei `http` ausserhalb des eigenen Netzes.
16. **Limit je Konto (E-3).** Je Konto ein Limit für Filme und eines für Staffeln, je Anzahl und Tage, rollierend,
    für alle Profile gemeinsam. Eingestellt wird in Slouchcat (Vorgabe, je Konto «Standard», «Eigenes»,
    «Unbegrenzt»); Slouchcat schreibt es in den Seerr-Benutzer, Seerr zählt und setzt durch, auch für
    Wünsche, die direkt dort gestellt werden. Abgelehnte Wünsche und Mitwünsche zählen nicht. Benutzer mit
    `MANAGE_USERS` haben kein Limit. Beim Verknüpfen übernimmt Slouchcat ein bestehendes Limit des Benutzers als
    eigenes Limit des Kontos, wenn das Konto noch keines hat; ohne Limit in Seerr gilt «Standard» (Marco, I-F3;
    Einzelheiten N-112). Slouchcat prüft vor dem Anfordern mit dem zuletzt gelesenen Stand und sagt,
    wann der nächste Platz frei wird.
17. **Profil-Tag in Radarr und Sonarr (E-2).** Ist ein Wunsch «In Beschaffung», legt Slouchcat in Radarr bzw.
    Sonarr den Tag des Profils an (`sc-<profilname>`, gebildet nach N-113, weil Radarr nur `a–z`, `0–9`, `-`
    annimmt) und hängt ihn an den Film bzw. die Serie, nachträglich auch
    für Mitwünscher. Adresse und Schlüssel liest es aus Seerr und hält die Schlüssel nur im Speicher, je
    10 Minuten; neu gelesen wird nach Ablauf oder nach einer Antwort 401. Ein
    eigenes Paket `internal/arr` kennt nur diese Aufrufe. Fehler ändern nichts am Wunsch; Tags werden nie
    entfernt.
18. **Wünsche-Seiten.** Unter «Wünsche» zeigen «Entdecken», «Filme», «Serien» und «Suche» Titel zum Anfordern. Die
    Listen holt der Server über Seerr im Namen des Benutzers, mit Zwischenspeicher; Altersstufe und
    Bibliotheksabgleich rechnet Slouchcat (ADR 0038).
19. **Download-Fortschritt nur beim Wunsch (E-7).** Solange ein Wunsch «In Beschaffung» ist, liest der Job
    `wishes.progress` jede Minute die Warteschlange je Dienst (`GET /api/v3/queue`, alle Seiten) und den Dateistand
    der gewünschten Titel (`movie?tmdbId=` bzw. `series?tvdbId=`) und ordnet die Einträge über die ID aus diesen
    Aufrufen zu. Mehr liest es nicht. Vier Anzeigen: «Wird gesucht», «Wird heruntergeladen · n % · noch m
    min», «Wird eingelesen», «Download verzögert»; bei Serien «x von y Folgen da». Gezeigt unter «Meine Wünsche», auf
    der Wunschkarte und in der Verwaltung, nie am Titel der Bibliothek, in Reihen oder in der Suche. Die Meldung von
    Radarr oder Sonarr sieht nur die Verwaltung. Der Fortschritt liegt in einem Zwischenspeicher; kein Seitenaufbau
    ruft Radarr oder Sonarr auf. Er ändert keinen Zustand; «verfügbar» bleibt, was Slouchcat abspielen kann.
    Schalter «Download-Fortschritt zeigen», ab Werk an.
20. **Webhooks wie heute (E-6).** Neue Dateien meldet weiter der Webhook von Radarr und Sonarr, den der
    Administrator von Hand einrichtet. Slouchcat trägt dort keinen Webhook ein. Der Import in die Bibliothek kommt
    weiter über den Webhook; in Radarr und Sonarr liest Slouchcat nur für die Anzeige am Wunsch (Entscheid 19). Der eingehende Teil von ADR 0010 bleibt unverändert. Ob Slouchcat aus derselben Meldung Dateipfad und
    Kennung liest, um schneller einzulesen, entscheidet E-11: als eigener Schritt nach einer Messung, der
    entfällt, wenn die Messung keinen Gewinn zeigt; die Webhooks selbst ändern sich dadurch nicht.

21. **Sammelbenutzer ab Werk (F-4).** Sobald Seerr eingerichtet und «Wünsche aktiv» an ist, legt Slouchcat in
    Seerr den lokalen Benutzer «slouchcat-gaeste» an (`slouchcat-gaeste@slouchcat.invalid`, Zufallspasswort, Rechte
    aus Seerrs Vorgabe für neue Benutzer, auf Marcos Instanz also mit automatischer Freigabe: I-F2, N-111) und schreibt ihm das Standard-Limit; es gilt gemeinsam für alle Konten,
    die über ihn senden. Über ihn senden Konten mit Wunschrecht ohne eigenen Seerr-Benutzer («nicht eröffnet»,
    «wartet»). «Konten ohne Seerr-Benutzer» steht ab Werk auf «Sammelbenutzer», die andere Wahl ist «Nicht
    erlauben»; einen anderen Benutzer als Sammelbenutzer wählt man nicht. Wünsche über ihn merkt sich Slouchcat mit
    Konto und Profil; bekommt das Konto später einen eigenen Benutzer, bleiben die alten Wünsche beim
    Sammelbenutzer, und seine Anfragen übernimmt `wishes.import` nicht (N-101, N-102; E-Mail, Zeitpunkt und N-102
    sind Redaktion).
22. **Passwort nur für eröffnete und eingeladene Benutzer (F-24).** Slouchcat setzt das Passwort in Seerr nur bei
    Benutzern, die es selbst eröffnet hat (Einladung, «In Seerr eröffnen», «Alle eröffnen»), und bei einem
    bestehenden Benutzer, der beim Annehmen einer Einladung oder nach einer E-Mail-Änderung des Kontos verknüpft
    wurde (E-8). Verknüpft ein Administrator ein bestehendes Konto über die E-Mail oder von Hand mit einem
    bestehenden Seerr-Benutzer, steht der Passwortstand auf «Wird nicht abgeglichen» (`password_state = off`); der
    Administrator kann den Abgleich je Konto einschalten (`setSeerrPasswordMode`; Schalter Redaktion, N-103).
23. **Ein Release (F-22, B-1).** Die Stufen 0 bis 5 sind die Reihenfolge des Baus; es gibt einen Zweig, einen Pull
    Request, eine Version und einen eingefrorenen Datenbankstand am Schluss (N-104). Gemergt und getaggt wird erst auf
    Marcos Wort (B-2).

## Verworfen

| Alternative | Grund |
|---|---|
| eigene Wunschverwaltung oder eigener Freigabe-Ablauf in Slouchcat | doppelt zu Seerr; Spec Zeile 65 |
| Radarr/Sonarr für mehr als Tags und das Lesen von Warteschlange und Dateistand der gewünschten Titel ansprechen | Spec Zeile 69; Seerr kann es schon |
| «API und Webhook»: Slouchcat trägt den Webhook in Radarr und Sonarr selbst ein und holt Verpasstes über `GET /api/v3/history/since` nach | Marco hat «Webhooks wie heute» gewählt (E-6). Slouchcat schriebe in die Einstellungen von Radarr und Sonarr, was «registers nothing there» aus ADR 0010 bricht |
| «Nur API, ohne Webhook»: Radarr und Sonarr im Takt nach Importen abfragen | Marco hat «Webhooks wie heute» gewählt (E-6). Die API kann nicht von sich aus melden («Push»); Abfragen im Takt sind langsamer als die Sofortmeldung des Webhooks und erzeugen dauernde Grundlast (Anhang H 8.3, 8.4) |
| Download-Fortschritt am Titel der Bibliothek, in Reihen und in der Suche | Marco: «bei Wünschen» (E-7); Spec Zeile 67 bleibt dort Nicht-Ziel |
| Fortschritt im Seitenaufbau direkt bei Radarr und Sonarr lesen | jede Seite hinge an der Erreichbarkeit von Radarr und Sonarr; der Zwischenspeicher genügt |
| Profil-Tag über `tags` im Body von `POST /request` | braucht `REQUEST_ADVANCED` für jeden Benutzer, ersetzt die Standard-Tags, kann einem Mitwunsch nichts nachreichen, und Seerr legt keine Tags an (Anhang H 4.2) |
| Passwort verschlüsselt in Slouchcat speichern, um es jederzeit nachzureichen | ein zweiter Ort für das Passwort; die drei Momente reichen |
| Limit in Slouchcat zählen | Wünsche direkt in Seerr zählten nicht mit (E-3) |
| `userId` im Body statt `X-API-User` | jeder Wunsch wäre sofort freigegeben, weil Seerr die Auto-Freigabe am Aufrufer prüft |
| Ausnahme je Profil «Profil sendet als anderer Seerr-Benutzer» (`wish_profile_map`, Auswahl «Sendet als») | von der Redaktion empfohlen; Marco hat abgelehnt (F-27, N-100). Folge, die er kennt: Profile im Konto eines Seerr-Administrators fordern ohne Freigabe und ohne Limit an; begrenzen lässt sich das nur über das Wunschrecht des Profils |
| Konten ohne Seerr-Benutzer ab Werk «Nicht erlauben», Sammelbenutzer nur als Wahl unter bestehenden Benutzern | von der Redaktion empfohlen; Marco hat «Sammelbenutzer ab Werk» gewählt (F-4). Den Sammelbenutzer legt Slouchcat selbst an |
| Passwort auch bei Konten übernehmen, die ein Administrator nachträglich mit einem bestehenden Seerr-Benutzer verknüpft | von der Redaktion empfohlen (K-11); Marco hat abgelehnt (F-24): ein solcher Benutzer behält sein Passwort |
| je Stufe ein Release | von der Redaktion empfohlen; Marco hat ein Release am Schluss gewählt (F-22, B-1) |
| Seerr als Iframe einbetten | entfiel schon in Runde 20; fremde Oberfläche, eigene Anmeldung |
| Seerr-Benutzer je Profil mit eigenem Login | Marco: Profile brauchen kein Login (Anhang H 1); künstliche E-Mail-Adressen; ein gelöschtes Profil nähme seine Anfragen mit |
| Benutzer eines gelöschten Kontos in Seerr löschen | Seerr löschte seine Requests mit (Anhang D 7.2) |
| «Verknüpfung erst nach Bestätigung»: nach einer Einladung oder E-Mail-Änderung nur ein Vorschlag, den ein Administrator bestätigt (N-2a) | von der Redaktion nach dem Befund K-1 der Kritik empfohlen; Marco hat «Sofort verknüpfen» gewählt (E-8). Das Risiko steht unter «Folgen» als bekannte Grenze |
| Webhook als Grundlage | ein Ziel, keine Signatur, keine Wiederholung, keine Benutzer-ID |
| «verfügbar» nach dem Zustand in Seerr | Meldung vor der Datei in Slouchcat; Kinderprofile sähen den Titel noch nicht |
| Wünsche über `netguard` | sperrt das Heimnetz, in dem Seerr steht |

## Folgen

- Neue Pakete `internal/seerr` und `internal/wishes`; Regel 2 bleibt unberührt (neue Funktionen in `visibility`,
  Haken aus `metadata`).
- Neue Tabellen nach Vorlage; Neubau von `notifications`, `deliveries`, `home_rows` und für `can_request` von
  `accounts` und `profiles`.
- Neue Schlüsselnamen `seerr.api_key`, `newPassword` und die Schlüssel von Radarr und Sonarr in Redaction-Liste
  und Test (Regel 10).
- Neues Paket `internal/arr` für Tags und das Lesen von Warteschlange und Dateistand der gewünschten Titel; ADR 0010
  bekommt den Nachtrag zu beiden.
  Die Seite «Webhooks von Radarr und Sonarr» bleibt, wie sie ist.
- Zwölf neue Jobarten; Abgleich und Prüfung belasten Seerr alle 5 min mit wenigen Aufrufen; `wishes.progress`
  ruft Radarr und Sonarr jede Minute einmal je Dienst auf, nur solange Wünsche in Beschaffung sind.
- Die Felder der Sonarr-Warteschlange (`seriesId`, `episodeId`, `seasonNumber`) sind nicht gegen eine Instanz
  geprüft; fehlen sie, zeigt Slouchcat weniger, nie Falsches.
- **Befund der Prüfung (Anhang I, I-10):** Auf Marcos Instanz ist der Medienserver Emby, und alle bestehenden
  Seerr-Benutzer melden sich über Emby an. Sie werden verknüpft, bekommen aber nie ein Passwort (N-91, N-6);
  dasselbe Login gibt es dort nur für Benutzer, die Slouchcat neu eröffnet.
- Das Passwort verlässt den Server im Klartext Richtung Seerr; ändert jemand es direkt dort, laufen die beiden
  auseinander (Marco kannte die Folge, E-1).
- Benutzerpflege, Passwort-Route und `settings/main` von Seerr sind im Code von Seerr 3.5.2 belegt, aber nicht
  dokumentiert; der Bau prüft sie gegen eine Instanz.
- `X-API-User` ist undokumentiert. Fällt es in einer künftigen Version weg, meldet der Test es, und die Funktion
  bleibt aus, bis neu entschieden ist.
- Seerr verschickt weiter eigene Meldungen, wenn sie dort eingerichtet sind.
- **Bekannte Grenze (Datenschutz, K-19):** Alle Profile eines Kontos teilen das Seerr-Login des Kontos. Wer
  sich damit in Seerr anmeldet, sieht dort die Wünsche aller Profile, auch die eines Profils mit PIN. Die
  PIN-Regel F-15 gilt nur für Slouchcats eigene Nachrichten. Seerr verschickt zudem eigene E-Mails mit dem
  Titel. Empfehlung: in Seerr die eigenen Benachrichtigungen für eröffnete Benutzer ausgeschaltet lassen;
  Wünsche › Einstellungen zeigt dazu einen Hinweis.
- **Bekannte Grenze (Übernahme, E-8, N-92):** Die E-Mail beim Annehmen einer Einladung und beim Ändern des
  eigenen Kontos prüft niemand. Wer eine Einladung hat und eine fremde Adresse eintippt, bekommt den
  Seerr-Benutzer dieser Person samt ihren Anfragen. Marco kennt das Risiko und hat es so gewählt. Es begrenzen:
  Einladungen stellt nur ein Administrator aus; Administratoren von Seerr und Benutzer mit Anmeldung über
  Jellyfin, Emby oder Plex bekommen kein Passwort; die Übersicht zeigt die Verknüpfung sieben Tage lang, das
  Protokoll hält sie fest, und sie lässt sich unter «Seerr-Benutzer» lösen (Empfehlung, N-93).
- Den Sofort-Import gibt es nur, wenn die Messung vorher einen Gewinn zeigt (E-11); sonst bleibt der Empfang
  der Webhooks, wie er ist.
- **Bekannte Grenze (Sammelbenutzer, F-4):** Alle Konten ohne eigenen Seerr-Benutzer teilen Limit und Freigabe des
  Sammelbenutzers; in Seerr erscheinen ihre Anfragen unter «slouchcat-gaeste». Wer gewünscht hat, weiss nur
  Slouchcat.
- **Zwei Passwörter (F-24):** Wer schon in Seerr war und von einem Administrator verknüpft wird, meldet sich in
  Seerr weiter mit seinem alten Passwort an, bis der Abgleich für das Konto eingeschaltet wird.
- Das Mockup des Konzepts zeigt an drei Stellen den Stand vor den Antworten
  (`docs/planning/25-medienwuensche/mockup-abdeckung.md`, Abschnitt 11); massgeblich ist dieses ADR.

## Spec-Wortlaut, der hier abgelöst wird (ohne die Spec umzuschreiben)

- **Constraint Zeile 46** und **Non-Goal Zeile 66**: Wünsche entstehen auch aus Slouchcat, in Seerr.
- **Non-Goal Zeile 69**: Ausnahme für Profil-Tags in Radarr und Sonarr.
- **Non-Goal Zeile 67**: nur beim Wunsch erlaubt: Zustand eigener Wünsche, Download-Fortschritt und Benachrichtigung
  bei Verfügbarkeit; ein Beschaffungsstatus am Titel der Bibliothek, in Reihen und in der Suche und der Suchfilter
  «in Beschaffung» nicht.
- **Spec A-6**: Die Kanäle tragen die Ereignisse der Wünsche statt des alten «Beschaffungsstatus».
- **Neu Spec I-9 bis I-11** (Nummern I-1 bis I-4 bleiben unbesetzt).
