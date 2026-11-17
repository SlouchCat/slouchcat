# Entscheid-Blatt Konzept 25 «Wünsche über Jellyseerr» (verbindliche Grundlage für Papiere und Mockups)

Stand 16.11.2026. Anker: `main` `ba6efe31`, Vertrag 1.40.0, letzte Migration 00065, letzte ADR 0036, letzte Frage 32.
Dieses Konzept ist **Konzept 25**, reserviert **Frage 33**, **ADR 0037** («Wünsche über Jellyseerr») und **ADR 0038**
(«Titel ausserhalb der Bibliothek»). Migrations- und Versionsnummern vergibt der Bau. Nichts wird gebaut, bevor Marco freigibt.
Quellen: Berichte A–E im selben Ordner (`A-spotlight-startseite.md`, `B-konten-benachrichtigungen.md`,
`C-integrationen-jobs-regeln.md`, `D-jellyseerr-api.md`, `E-design-sheet.md`). Belege dort.

## 1. Begriffe (ein Wort je Sache, in Oberfläche und Papier)

- Funktion und Seiten: **«Wünsche»**; ein Eintrag ist ein **Wunsch**. Tätigkeit auf Knöpfen: **«anfordern»**
  («Film anfordern», «Serie anfordern», «Staffeln anfordern»). Zuschauerseite: **«Meine Wünsche»** (`/wishes`).
- Titel, der nicht in der Bibliothek steht: **«fehlender Titel»**, in der Oberfläche «Noch nicht in der Bibliothek».
- **Jellyseerr** steht in der Oberfläche; technisch heisst das Paket `seerr` (Nachfolger Seerr 3.x, gleiche API `/api/v1`).
- **Mitwunsch**: ein weiteres Profil hängt sich an einen offenen Wunsch.
- **Vorschauseite**: Detailseite eines fehlenden Titels (`/titles/{movie|show}/{tmdbId}`).

Zustände eines Wunsches (Slouchcat-eigen, abgeleitet aus Jellyseerr und der eigenen Bibliothek):

| Schlüssel | Anzeige | Bedeutung | Herkunft |
|---|---|---|---|
| `queued` | «Wird übermittelt» | lokal gespeichert, noch nicht bei Jellyseerr angekommen | Jellyseerr nicht erreichbar oder Übermittlung läuft |
| `pending` | «Freigabe ausstehend» | bei Jellyseerr, wartet auf Freigabe | Request-Status 1 PENDING |
| `processing` | «In Beschaffung» | freigegeben, Radarr/Sonarr beschaffen | Request 2 APPROVED (Media 3 PROCESSING); auch wenn Jellyseerr schon «verfügbar» meldet, Slouchcat die Datei aber noch nicht eingelesen hat (Zusatz «wird eingelesen») |
| `partial` | «Teilweise verfügbar» | Serie: ein Teil der gewünschten Staffeln ist in Slouchcat abspielbar | eigene Bibliothek |
| `available` | «Verfügbar» | in Slouchcat für das Profil abspielbar | eigene Bibliothek (nicht Jellyseerr) |
| `declined` | «Abgelehnt» | in Jellyseerr abgelehnt (ohne Grund: Jellyseerr kennt keinen Ablehnungsgrund) | Request 3 DECLINED |
| `failed` | «Fehlgeschlagen» | Übergabe an Radarr/Sonarr gescheitert oder Übermittlung endgültig abgewiesen (Kontingent, Sperrliste, kein Recht) | Request 4 FAILED oder 403 bei `POST /request` |
| `withdrawn` | «Zurückgezogen» | vom Profil zurückgezogen, solange nicht freigegeben | `DELETE /request/{id}` |

## 2. Kernentscheide (Empfehlungen; offen als F-1 bis F-22 in Abschnitt 9)

1. **Jellyseerr bleibt die Wunschverwaltung.** Slouchcat legt Wünsche dort an, liest ihren Zustand, zeigt ihn und meldet die
   Verfügbarkeit. Keine eigene Beschaffung, keine Qualitätsprofile, keine Zielordner, kein eigener Freigabe-Ablauf.
   Radarr/Sonarr spricht nur Jellyseerr an.
2. **Anbindung:** ausgehend, `X-Api-Key` (ein Admin-Schlüssel je Instanz), verschlüsselt gespeichert wie die anderen Schlüssel
   (AES-256-GCM, nie wieder angezeigt, letzte vier Zeichen). Eigener HTTP-Client `internal/seerr` **mit ausdrücklicher
   LAN-Erlaubnis** (Jellyseerr steht im Heimnetz; `netguard` sperrt LAN), **ohne Weiterleitungen**, Zeitlimit 10 s,
   höchstens 5 Anfragen/s. Nur die vom Administrator eingetragene Adresse wird je angesprochen.
3. **Wunsch im Namen des Benutzers:** `POST /api/v1/request` mit Header **`X-API-User: <Jellyseerr-Benutzer>`** und ohne
   `userId` im Body. Dann gelten Rechte, Kontingent und Auto-Freigabe **des zugeordneten Jellyseerr-Benutzers**, genau
   wie wenn er selbst geklickt hätte. (Mit `userId` im Body entschiede der Aufrufer = Benutzer 1 über die Freigabe: jeder
   Wunsch wäre sofort genehmigt.) `X-API-User` ist undokumentiert, steht aber unverändert im Code von Jellyseerr 2.7.3 und
   Seerr 3.5.2. Der Verbindungstest prüft es mit `GET /auth/me` + Header und meldet, wenn es nicht wirkt.
4. **Zuordnung = Variante C (hybrid):** Jedes Slouchcat-**Konto** bekommt einen Jellyseerr-Benutzer (Standard für alle seine
   Profile); einzelne **Profile** können einen eigenen bekommen («Eigener Benutzer» statt «Wie Konto»). Slouchcat speichert je
   Wunsch selbst: Konto, Profil, Jellyseerr-Benutzer, Jellyseerr-Request-ID. Jellyseerr kennt keine Slouchcat-Profile und hat
   kein Feld für fremde Kennungen (`profileId` dort = Qualitätsprofil von Radarr/Sonarr). Zuordnen macht der Administrator;
   Slouchcat **schlägt** bei gleicher E-Mail-Adresse vor, übernimmt aber nie von selbst. Ein Jellyseerr-Benutzer gehört
   höchstens zu **einem** Konto (Profile desselben Kontos dürfen ihn teilen).
5. **Konten ohne Zuordnung** können nicht anfordern (Vorgabe). Wahlweise ein Sammelbenutzer (aus, F-4).
6. **Wunschrecht:** Schalter je Konto (setzt der Administrator, Formular «Zugriff») als Deckel, Schalter je Profil (setzt das
   Hauptprofil im Profildialog). Vorgaben: Besitzerkonto an, neue Konten aus, Hauptprofil an, weitere Profile aus.
   Server liefert `canRequest` + Grund; der Client entscheidet nichts (Regel 12).
7. **Jugendschutz für fehlende Titel:** Server prüft die TMDB-Freigabe (Freigabeland-Reihenfolge nach ADR 0034, strengster
   Wert) gegen min(Profil, Konto); **unbekannt = 18**. Was ein Profil nicht sehen dürfte, existiert für es nicht (404),
   auch nicht in der Suche und nicht im Spotlight.
8. **Verfügbar ist, was Slouchcat abspielen kann.** Auslöser: Haken nach dem Metadaten-Match (IDs geschrieben), dazu
   stündlicher Nachlauf `wishes.reconcile`. Abgleich über TMDB-ID, bei Serien auch TVDB-ID und IMDb-ID. Der Zustand in
   Jellyseerr («AVAILABLE») löst allein keine Meldung aus.
9. **Abgleich:** Job `wishes.sync` alle 5 min (einstellbar): `GET /request?sort=modified` absteigend, bis `updatedAt` älter
   als der letzte Abgleich ist; kein Sammelabruf nach TMDB-IDs vorhanden. Dazu `GET /request/{id}` für offene Wünsche ohne
   Änderung seit 24 h. Jellyseerr-Webhook nur als **Anstoss** zum sofortigen Abgleich (Stufe 4, F-18): nur ein Webhook-Ziel
   je Jellyseerr, keine Signatur, keine Wiederholung, keine Benutzer-ID in der Nutzlast.
10. **Übermittlung mit Ausgangsfach:** `createWish` speichert lokal (`queued`), versucht sofort (Zeitlimit 3 s) und sonst
    per Job `wishes.submit` mit Backoff; nach 24 h ohne Erfolg `failed` + Fehlerereignis an Administratoren. Vor jedem
    erneuten Senden prüft der Job in Jellyseerr, ob der Wunsch schon existiert (`GET /movie|tv/{tmdbId}` → `mediaInfo.requests`).
    409 «Request for this media already exists.» zählt als Erfolg: Slouchcat hängt sich an den bestehenden Request.
11. **Doppelte Wünsche:** In Slouchcat höchstens ein offener Wunsch je Film bzw. je Serienstaffel. Ein zweites Profil wird
    **Mitwünscher** (lokal, kein Aufruf an Jellyseerr, dort sind Duplikate global gesperrt) und wird ebenfalls
    benachrichtigt. Zuschauer sehen nie, wer sonst gewünscht hat; die Verwaltung sieht alle.
12. **Fehlende Titel brauchen eigene Daten:** Tabelle `external_titles` (TMDB: Titel je Sprache, Jahr, Beschreibung, Laufzeit,
    Genres, Freigabe, Poster/Backdrop/Logo, Trailer, bei Serien Staffeln mit Folgenzahl und Datum, Status, nächste Folge),
    Bilder über Slouchcats eigenen Bildspeicher und eigene Route (nie direkt von TMDB im Browser). Für Serien **in** der
    Bibliothek hält `series_catalog` die Staffelliste laut TMDB, damit «teilweise vorhanden» erkennbar wird.
13. **Spotlight:** zeigt fehlende Titel nur, wenn der Administrator es einschaltet (ab Werk aus). Höchstens N fehlende
    (0–10, Vorgabe 5) unter den höchstens 20 Titeln, in Trakt-Reihenfolge; die **erste Folie ist immer ein vorhandener
    Titel** (Prepaint und Vorladen bleiben wie heute); fehlende zählen nicht für «Ausblenden unter». Profile ohne
    Wunschrecht sehen keine fehlenden Titel. Trakt-Einträge speichern neu auch TVDB- und IMDb-ID (heute nur TMDB: Serien,
    die TVDB zuerst zuordnet, gälten sonst fälschlich als fehlend).
14. **Suche:** Bibliothekssuche bleibt. Darunter ein Abschnitt «Noch nicht in der Bibliothek» aus der TMDB-Suche über den
    eigenen TMDB-Schlüssel (nicht über Jellyseerr), gefiltert nach Bibliothek, Freigabe und Recht. Eine Seite «Entdecken»
    gibt es heute nicht und es entsteht keine: Suche, Spotlight-Liste (`/lists/spotlight`) und die Vorschauseite decken es ab.
15. **Serien:** Im Anforderungsdialog wählt man «Alle fehlenden Staffeln» oder einzelne Staffeln; Staffel 0 und Staffeln
    ohne Folgen sind nicht wählbar; einzelne Folgen gehen nicht (Jellyseerr arbeitet staffelweise). Die Serienseite zeigt
    fehlende Staffeln als gedämpfte Karten in der Staffelwand und die Aktion «Staffeln anfordern». Anzeige «Läuft noch,
    nächste Folge am …» aus `series_catalog`. TVDB-ID wird mitgeschickt (ohne sie löscht Jellyseerr den Request beim Senden an Sonarr).
16. **Benachrichtigungen** nur über Slouchcat: Eingang des Profils (immer) und Kanäle des Kontos (E-Mail, Web-Push, Telegram).
    Neue Eingangsarten `wish_available`, `wish_season_available`, `wish_approved`, `wish_declined`, `wish_failed`.
    Neue Kanal-Ereignisse: `wish_available` «Wünsche: verfügbar», `wish_status` «Wünsche: freigegeben, abgelehnt,
    fehlgeschlagen», `wish_review` «Wünsche: Freigabe nötig» (nur Administratoren). «Wunsch eingereicht» ist keine
    Benachrichtigung, sondern die sofortige Rückmeldung am Bildschirm. Doppelschutz über eindeutigen Schlüssel
    (`wish:<id>:available`, `wish:<id>:season:<n>` …).
17. **Empfänger und Datenschutz:** Eingang: nur das anfordernde Profil und Mitwünscher, je erst wenn der Titel für **dieses**
    Profil sichtbar ist. Kanäle: die des Kontos, Betreff mit `[Profilname]` (heutige Regel 5.5). Neu: bei Profilen mit PIN
    nennt die externe Nachricht den Titel nicht («[Sandra] Ein Wunsch ist jetzt verfügbar.»). `wish_review` geht an Administratoren.
18. **Verwaltung:** neue Kachel/Ansicht **Integrationen › Jellyseerr** (Verbindung, Test, Zustand, Abgleich, Protokoll);
    neue Seite **Medien › Wünsche** mit den Reitern «Anfragen», «Zuordnung», «Einstellungen»; **Konten**: Feld «Darf Titel
    anfordern» und Anzeige der Zuordnung; **Spotlight**: Band «Fehlende Titel»; **Startseite**: neue Reihe «Deine Wünsche»;
    **Benachrichtigungen**: neue Ereignisse; **Hintergrundjobs**: neue Jobarten; **Übersicht**: Kennzahl und Hinweise.
    Jede Einstellung hat genau einen Ort; andere Seiten verlinken dorthin.
19. **Freigeben/Ablehnen in Slouchcat** (Durchreichen an `POST /request/{id}/approve|decline`) für Administratoren in Stufe 4 (F-16).
20. **Offline/PWA:** Wünsche nur online (`/api` ist NetworkOnly); kein Offline-Zustand.

## 3. Zuschauer-App: Seiten und Verhalten

| Seite | Route | neu/geändert | Kern |
|---|---|---|---|
| Startseite mit Spotlight | `/` | geändert | Bühne und Posterreihe zeigen auch fehlende Titel; Reihe «Deine Wünsche» |
| Spotlight-Liste | `/lists/spotlight` | geändert | ganze Liste, Filter «Alle / In der Bibliothek / Noch nicht da» |
| Detailseite Film | `/items/{id}` | unverändert (nur vorhanden) | – |
| Detailseite Serie | `/items/{id}` | geändert | fehlende Staffeln, «Staffeln anfordern», Zeile «2 von 4 Staffeln vorhanden» |
| Vorschauseite | `/titles/{movie\|show}/{tmdbId}` | neu | Aufbau wie Detailseite; Hauptknopf «Film anfordern»/«Serie anfordern» oder Zustand |
| Suche | `/search` | geändert | Abschnitt «Noch nicht in der Bibliothek» |
| Meine Wünsche | `/wishes` | neu | Liste, Filter, Verlauf je Wunsch, Zurückziehen, erneut anfordern |
| Benachrichtigungen | `/notifications` | geändert | neue Arten mit Link |
| Benachrichtigungskanäle | `/settings/notifications` | geändert | neue Ereignisse je Kanal |
| Profile | `/settings/profiles` | geändert | Schalter «Darf Titel anfordern» im Profildialog, Chip auf der Karte |
| Profilwahl | `/profiles` | unverändert | (im Mockup bedienbar, um Profile zu wechseln) |

Bühne bei fehlendem Titel: Metazeile mit Chip «Noch nicht in der Bibliothek»; Hauptknopf «Film anfordern» / «Serie
anfordern» (Plus-Symbol), daneben «Details» (Vorschauseite) und «Trailer». Nach dem Anfordern wird der Hauptknopf zur
Zustandsanzeige («Freigabe ausstehend» usw., führt zu «Meine Wünsche»). Posterreihe: fehlende Titel ohne Gesehen-Haken
und ohne Merken, dafür unten rechts ein runder Plus-Knopf (oder Zustandssymbol), Unterzeile «2026 · Noch nicht da» bzw. Zustand.
Anfordern öffnet **einen** Dialog: Poster, Titel, «Wird als {{Jellyseerr-Benutzer}} an Jellyseerr übermittelt»,
Hinweis «Wird sofort beschafft» oder «Braucht eine Freigabe», bei Serien die Staffelwahl, Knöpfe «Anfordern»/«Abbrechen».
Fälle im Dialog/Knopf: kein Recht (Knopf fehlt, Hinweis auf der Vorschauseite), Konto ohne Zuordnung, Jellyseerr nicht
erreichbar (Wunsch wird trotzdem angenommen: «Wird übermittelt, sobald Jellyseerr erreichbar ist»), schon angefordert
(«Auch benachrichtigen» = Mitwunsch), Kontingent erschöpft (fehlgeschlagen mit Klartext).
Seitenleiste: neuer Eintrag «Wünsche» nach «Gesehen» mit Zähler offener Wünsche; nur wenn das Profil anfordern darf oder Wünsche hat.

## 4. Verwaltung: Seiten und Einstellungen (jede Einstellung genau ein Ort)

**Automatisierung › Integrationen › Jellyseerr** (`/admin/integrations?item=jellyseerr`, neue siebte Kachel):
- Band «Verbindung»: Schalter «Jellyseerr verwenden»; «Adresse» (z. B. `http://192.168.1.50:5055`); «API-Schlüssel»
  (geheim, «Leer lassen, um den gespeicherten zu behalten», zeigt «…a1b2»); «Verbindung testen»; «Speichern», «Verwerfen».
  Prüfliste des Tests: erreichbar; Schlüssel gültig; Version (Jellyseerr ≥ 2.0 / Seerr ≥ 3.0); «Handeln im Namen von
  Benutzern» (`X-API-User`); CSRF-Schutz in Jellyseerr aus (sonst scheitern schreibende Aufrufe); Radarr und Sonarr in
  Jellyseerr eingerichtet. Zustände: Nicht eingerichtet, Verbunden, Schlüssel ungültig (403), Nicht erreichbar, Ausgeschaltet.
- Band «Abgleich»: letzter erfolgreicher Abgleich, nächster Lauf, «Jetzt abgleichen», Zähler (offene Wünsche, warten auf
  Übermittlung, nicht zuordenbar), Link zu den Jobs.
- Band «Sofortmeldung» (Stufe 4): Webhook-Adresse für Jellyseerr (einmal gezeigt), JSON-Vorlage zum Kopieren, Hinweis
  «Jellyseerr kennt nur ein Webhook-Ziel».
- Band «Protokoll»: letzte 20 Aufrufe und Fehler.
- Verweis «Zuordnung: n von m Konten» → Medien › Wünsche › Zuordnung.

**Medien › Wünsche** (`/admin/wishes`, neuer Bereichsreiter mit Zähler «wartet auf Freigabe»):
- Reiter «Anfragen»: Kennzahlen (Offen, Freigabe ausstehend, In Beschaffung, Verfügbar 30 Tage, Fehler); Filter nach
  Zustand und «Nicht zuordenbar»; Tabelle (Titel mit Poster, Art, Gewünscht von «Konto · Profil» + Mitwünscher,
  Jellyseerr-Benutzer, Zustand, Seit); Aufklappzeile mit Verlauf und Diagnose; Aktionen «Freigeben», «Ablehnen»
  (Stufe 4), «Erneut übermitteln», «In Jellyseerr öffnen», «Zum Titel».
- Reiter «Zuordnung»: je Konto ein Jellyseerr-Benutzer (Auswahl; belegte gesperrt), Vorschlag bei gleicher E-Mail mit
  «Übernehmen», aufklappbar die Profile («Wie Konto» / eigener Benutzer); je Jellyseerr-Benutzer die Fakten (darf
  Filme/Serien anfordern, Auto-Freigabe, Kontingent); Warnungen (Benutzer in Jellyseerr gelöscht, darf nicht anfordern);
  «Benutzerliste neu laden», «Alle Vorschläge übernehmen» (mit Bestätigung).
- Reiter «Einstellungen»: «Wünsche aktiv»; «Filme erlauben»; «Serien erlauben»; «Anfordern aus dem Spotlight», «… aus der
  Suche», «… auf Detailseiten»; «Bereits angeforderte Titel» (Mitwünschen / Nur Zustand zeigen); «Konten ohne Zuordnung»
  (Nicht erlauben / Sammelbenutzer + Auswahl); «Neue Konten dürfen anfordern»; «Zurückziehen erlauben»; «Externe Nachrichten
  bei PIN-Profilen ohne Titel»; Verweis auf Spotlight («Fehlende Titel im Spotlight: Aus · Zur Seite Spotlight»).

**Nutzer & Geräte › Konten:** neue Spalte «Wünsche» (Chip «Erlaubt»/«Nicht erlaubt»/«Zuordnung fehlt»); im Formular
«Zugriff» der Schalter «Darf Titel anfordern» und die Zeile «Jellyseerr-Benutzer: … · Zuordnung ändern» (Link);
Dialog «Konto einladen»: Schalter «Darf Titel anfordern»; eigenes Konto «Meine Profile»: Profildialog mit Schalter.

**Startseite › Spotlight:** neues Band «Fehlende Titel»: «Fehlende Titel zeigen» (Schalter), «Höchstens fehlende Titel»
(0–10), Reihenfolge («Wie bei Trakt» / «Nach den vorhandenen»), Zustand («7 fehlende Titel dürften erscheinen; 3 sind
angefordert»). Tabelle «Einträge der Liste»: fehlende Einträge bekommen Poster, Zustand («Noch nicht in der Bibliothek»,
«Angefordert · In Beschaffung») und die Aktionen «Anheften», «Ausblenden», «Notiz», «Anfordern». Alle elf heutigen Felder
des Bands «Quelle» bleiben.

**Startseite › Startseite:** neue Reihe «Deine Wünsche» in der Tabelle (Platz, Ausblenden), sonst unverändert.

**Automatisierung › Benachrichtigungen** (= Kanalseite des Kontos): neue Ereignisse je Kanal: «Wünsche: verfügbar»,
«Wünsche: freigegeben, abgelehnt, fehlgeschlagen», nur Administratoren «Wünsche: Freigabe nötig».

**Automatisierung › Hintergrundjobs:** neue Jobarten `wishes.submit` (Ereignis), `wishes.sync` (5 min), `wishes.reconcile`
(1 h), `seerr.users` (24 h), `titles.refresh` (24 h, nach `trakt.refresh`), `series.catalog` (Ereignis + 7 d).

**Übersicht:** Kennzahl «Offene Wünsche»; unter «Was benötigt Aufmerksamkeit?» «n Wünsche warten auf Freigabe»,
«Jellyseerr nicht erreichbar», «n Konten ohne Zuordnung».

## 5. Datenmodell (Entwurf; Namen englisch, Migrationen nach Vorlage, nur vorwärts)

- `seerr_users` (`seerr_id` PK, `display_name`, `email`, `user_type`, `permissions` Bitmaske, `movie_quota_*`, `tv_quota_*`, `fetched_at`, `gone_at`)
- `wish_account_map` (`account_id` PK → accounts CASCADE, `seerr_user_id` UNIQUE, `set_at`, `set_by`)
- `wish_profile_map` (`profile_id` PK → profiles CASCADE, `seerr_user_id`, `set_at`)
- `accounts.can_request`, `profiles.can_request` (0/1; Tabellenneubau nach Vorlage)
- `wishes` (`id`, `media` movie|show, `tmdb_id`, `tvdb_id`, `imdb_id`, `title`, `year`, `origin` slouchcat|jellyseerr,
  `seerr_request_id`, `seerr_media_id`, `seerr_user_id`, `state`, `state_detail`, `fail_reason`, `attempts`,
  `next_attempt_at`, `item_id` → items SET NULL, Zeitstempel `created_at`, `submitted_at`, `approved_at`, `available_at`,
  `closed_at`, `updated_at`, `seerr_updated_at`); eindeutig: ein offener Wunsch je (`media`, `tmdb_id`) bei Filmen
- `wish_seasons` (`wish_id`, `season_number`, `state`, `available_at`); eindeutig: eine offene Zeile je (`tmdb_id`, `season_number`)
- `wish_requesters` (`wish_id`, `profile_id`, `account_id`, `role` owner|joined, `created_at`) PK (`wish_id`, `profile_id`)
- `wish_events` (`wish_id`, `at`, `kind`, `detail`) Verlauf und Protokoll
- `external_titles` (+ `external_title_texts` je Sprache), `series_catalog` (`item_id`, Staffeln, Status, nächste Folge)
- `spotlight_entries` + `tvdb_id`, `imdb_id`; `spotlight_config` + `show_missing`, `max_missing`, `missing_order`
- `notifications`: Neubau (Art erweitert, `item_id` nullable, `wish_id`, `dedupe_key` eindeutig je Profil);
  `deliveries`: Neubau (Ereignis-CHECK erweitert); `home_rows`: CHECK + `wishes`
- `settings`: `seerr.*` (Adresse, Schlüssel verschlüsselt, an/aus, Zeitstempel), `wishes.*` (alle Schalter aus Abschnitt 4)

## 6. API (Entwurf; jede Operation mit `x-visibility` und Eintrag im Vertragstest)

| Operation | Methode, Pfad | Stufe |
|---|---|---|
| `listWishes` | GET `/wishes` | scoped |
| `createWish` | POST `/wishes` `{media, tmdbId, seasons?}` → 201 Wunsch; 200 bestehender Wunsch (idempotent, auch Mitwunsch); 409 `already_available` mit `ItemRef`; 403 `wish_not_allowed` (Grund); 404 nicht sichtbar | scoped |
| `withdrawWish` | DELETE `/wishes/{id}` | scoped |
| `listAccountWishes` | GET `/account/wishes` (alle Profile des Kontos, nur Hauptprofil) | account |
| `getExternalTitle` | GET `/titles/{media}/{tmdbId}` | scoped |
| `searchExternalTitles` | GET `/titles/search?q=` | scoped |
| Bilder fehlender Titel | GET `/titles/{media}/{tmdbId}/images/{kind}/{size}` (`http.Handler`) | scoped |
| `getHome`, `listHomeRow` | erweitert: fehlende Titel als eigenes Schema `ExternalTitleRef` neben `ItemRef` (nicht brechend) | scoped |
| `getItem` | erweitert: `catalog` (fehlende Staffeln, Status, nächste Folge), `canRequestSeasons` | scoped |
| Sitzung | erweitert: `wishes {enabled, canRequest, reason}` | account |
| `getSeerrSettings`, `updateSeerrSettings`, `testSeerrConnection`, `getSeerrStatus`, `syncWishesNow` | `/admin/seerr…` | admin |
| `listSeerrUsers`, `refreshSeerrUsers`, `getWishMappings`, `setAccountWishMapping`, `setProfileWishMapping` | `/admin/wishes/mappings…` | admin |
| `listAdminWishes`, `retryWish`, `approveWish`, `declineWish` | `/admin/wishes…` | admin |
| `getWishSettings`, `setWishSettings` | `/admin/wishes/settings` | admin |
| Spotlight, Konten, Profile, Einladungen, Startreihen | bestehende Operationen bekommen die neuen Felder | wie heute |
| `receiveWebhook` | POST `/webhooks/{token}` erweitert um die Art Jellyseerr (Stufe 4) | public |

## 7. Pakete und Regeln

- `internal/seerr`: reiner HTTP-Client ohne DB (Vorbild `internal/trakt`).
- `internal/wishes`: Dienst (anlegen, übermitteln, abgleichen, nachlaufen); SQL nur auf den Wunsch-Tabellen.
- Regel 2 bleibt: `wishes` fragt `items` nicht ab. «Gibt es den Titel, darf das Profil ihn sehen?» beantwortet
  `visibility` (neue Funktionen für externe IDs und für fehlende Titel); der Haken «Titel zugeordnet» kommt aus `metadata`
  (Stelle des heutigen `WithRelink`-Hakens nach dem Schreiben der IDs).
- Regel 10: `X-Api-Key`/`apikey` steht schon in der Redaction-Liste; neue Schlüsselnamen (`seerr.api_key`) ergänzen und testen.
- Regel 11 und Non-Goals: werden durch Frage 33 für genau diese Funktion geändert (Abschnitt 8).
- ADR 0037: Ausnahme vom Non-Goal, Jellyseerr als Wunschverwaltung, Client mit LAN-Erlaubnis, `X-API-User`, Zuordnung C,
  Zustände, Verfügbarkeit, Ausgangsfach, Mitwunsch, Benachrichtigungen. ADR 0038: Titel ausserhalb der Bibliothek
  (`external_titles`, Jugendschutzregel, Bildweg, Spotlight S-4 abgelöst, `series_catalog`).

## 8. Frühere Entscheide, die sich ändern (nur mit Marcos Freigabe, Frage 33)

- Spec Non-Goals Zeilen 65 (Freigabe-Anfragen von Kinderprofilen: bleibt Non-Goal, Slouchcat baut keine eigene Freigabe;
  die Freigabe liegt in Jellyseerr), 66 (Wunschverwaltung/Bereich Wünschen: Ausnahme), 67 (Beschaffungsstatus und
  Benachrichtigung bei Verfügbarkeit: Ausnahme), 69 (Steuerung von Radarr/Sonarr: bleibt Non-Goal; Slouchcat spricht nur
  Jellyseerr an), Zeile 46; gestrichene I-3/I-4, AN-5/AN-16 leben in neuer Form auf.
- Frage 10 (22.10.2026, M9-Zuschnitt): Wünschen war «Kandidat für die Ideensammlung»; Webhooks nur Radarr/Sonarr
  (Jellyseerr-Art kommt in Stufe 4 dazu).
- S-4 (Frage 11, ADR 0011): «Titel ohne Bibliothekstreffer erscheinen nie auf der Startseite» wird abgelöst: nur mit
  eingeschaltetem Schalter und nur für Profile mit Wunschrecht.
- CLAUDE.md Regel 11: «eigene Wunschverwaltung, Steuerung von Arr/Seerr» → Wortlaut anpassen (Vorschlag im Papier,
  ändern erst nach Freigabe).
- ADR 0009 (Benachrichtigungen: Ereignisliste), ADR 0010 (Webhook-Arten), Plan 5.1/5.6.

## 9. Offene Entscheide an Marco (mit Empfehlung)

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-1 | Begriffe «Wünsche» und «anfordern»? | Ja |
| F-2 | Zuordnung: A je Konto, B je Profil, C hybrid? | C |
| F-3 | Wunsch als Benutzer über `X-API-User` (undokumentiert, mit Prüfung) oder über `userId` (immer genehmigt)? | `X-API-User` |
| F-4 | Konten ohne Zuordnung: nicht erlauben oder Sammelbenutzer? | Nicht erlauben; Sammelbenutzer als Wahl |
| F-5 | Wunschrecht je Konto (Administrator) und je Profil (Hauptprofil); Vorgaben wie in 2.6? | Ja |
| F-6 | Kinderprofile dürfen mit Recht anfordern; die Freigabe entscheidet Jellyseerr (Auto-Freigabe des zugeordneten Benutzers), keine eigene Freigabe in Slouchcat? | Ja |
| F-7 | Schon angeforderter Titel: Mitwünschen oder nur Zustand zeigen? | Mitwünschen |
| F-8 | Spotlight: Schalter ab Werk aus, höchstens 5 fehlende von 20, Trakt-Reihenfolge, erste Folie immer vorhanden? | Ja |
| F-9 | Profile ohne Wunschrecht sehen keine fehlenden Titel? | Ja |
| F-10 | Anfordern mit einem Bestätigungsdialog (statt sofort mit «Rückgängig»)? | Dialog |
| F-11 | Suche nach fehlenden Titeln über den eigenen TMDB-Schlüssel (statt über Jellyseerr)? | TMDB |
| F-12 | Serien: Staffelwahl, keine einzelnen Folgen; fehlende Staffeln auf der Serienseite? | Ja |
| F-13 | «Verfügbar» = Slouchcat kann es für das Profil abspielen (nicht der Zustand in Jellyseerr)? | Ja |
| F-14 | Drei Kanal-Ereignisse; ab Werk «verfügbar» an, «Zustand» aus, «Freigabe nötig» an für Administratoren? | Ja |
| F-15 | Externe Nachrichten bei PIN-Profilen ohne Titel? | Ja |
| F-16 | Freigeben/Ablehnen in der Slouchcat-Verwaltung (Durchreichen) in Stufe 4? | Ja |
| F-17 | Wunsch zurückziehen, solange nicht freigegeben? | Ja |
| F-18 | Jellyseerr-Webhook als Zusatz (Sofortmeldung) in Stufe 4; Grundlage bleibt der Abgleich alle 5 min? | Ja |
| F-19 | Verwaltung: Seite «Wünsche» im Bereich Medien, Verbindung unter Integrationen? | Ja |
| F-20 | Startreihe «Deine Wünsche» (ausblendbar)? | Ja |
| F-21 | Bestehende Jellyseerr-Anfragen zugeordneter Benutzer übernehmen (dem Hauptprofil des Kontos bzw. dem Profil mit eigener Zuordnung, gekennzeichnet «In Jellyseerr angelegt»)? | Ja |
| F-22 | Vier Stufen in der Reihenfolge von Abschnitt 10? | Ja |

## 10. Stufen der Umsetzung (erst nach Freigabe, je Stufe ein Release)

| Stufe | Inhalt | grob |
|---|---|---|
| 0 | Frage 33, ADR 0037 und 0038, Regel 11 und Spec-Verweise anpassen | 0,5 Tage |
| 1 «Fundament» | Paket `seerr`, Verbindung und Test, Benutzerliste, Zuordnung, Wunschrecht, Einstellungen; Verwaltungsseiten. Für Zuschauer ändert sich nichts. | 4 Tage |
| 2 «Anfordern» | `external_titles`, Vorschauseite, Suche, `createWish` mit Ausgangsfach, Abgleich, «Meine Wünsche», Verwaltung «Anfragen», Verfügbarkeit erkennen, Benachrichtigung «verfügbar», Mitwunsch | 8 Tage |
| 3 «Spotlight» | fehlende Titel in Bühne, Posterreihe und Spotlight-Liste, Band «Fehlende Titel», Trakt-IDs, Reihe «Deine Wünsche» | 5 Tage |
| 4 «Serien und Komfort» | `series_catalog`, fehlende Staffeln auf der Serienseite, Staffel-Meldungen, Ereignis «Zustand», Freigeben/Ablehnen in der Verwaltung, Sofortmeldung per Webhook, bestehende Anfragen übernehmen | 7 Tage |

## 11. Beispieldaten der Mockups (erfunden; Titel und Bilder von TMDB)

Konten: `marco@example.ch` (Besitzer; Profile Marco [Hauptprofil, 18], Sandra [18, PIN], Noah [12], Mia [6]),
`grosseltern@example.ch` (Konto; Profil «Oma & Opa» [16]), `gast@example.ch` (Konto; Profil «Gast» [16]).
Jellyseerr-Benutzer: 1 marco (Administrator, Auto-Freigabe), 2 sandra (Auto-Freigabe), 3 kinder (keine Auto-Freigabe,
Kontingent 5 Filme je 7 Tage), 4 grosseltern (keine Auto-Freigabe; gleiche E-Mail wie das Konto → Vorschlag), 5 wohnzimmer-tv
(keinem Konto zugeordnet, hat eigene Anfragen → «nicht zuordenbar»).
Zuordnung zu Beginn: Konto marco → marco; Profil Sandra → sandra; Profil Noah → kinder; Mia wie Konto, darf nicht anfordern;
Konto grosseltern: Vorschlag offen; Konto gast: keine Zuordnung, kein Recht.

## 12. Mockup-Seiten (Kennungen für Papiere und Abdeckungsmatrix)

Ablage: `docs/planning/mockups/25-medienwuensche/` (Einstieg `index.html`, eine Seite mit Hash-Routen; Bilder liegen nur
lokal in `img/`, nicht im öffentlichen Repo; ohne Bilder zeigt das Mockup Platzhalter mit Titel).

| Kennung | Seite | Route im Mockup |
|---|---|---|
| M-00 | Einstieg: Seitenübersicht und Mockup-Steuerung (Profil, Jellyseerr-Zustand, Abläufe vorspulen, Ansicht Handy/Desktop, Zurücksetzen) | Leiste «Mockup» auf jeder Seite, `#/mockup` |
| M-01 | Startseite mit Spotlight und Reihe «Deine Wünsche» | `#/` |
| M-02 | Spotlight-Liste | `#/lists/spotlight` |
| M-03 | Vorschauseite Film (fehlender Titel) | `#/titles/movie/{tmdbId}` |
| M-04 | Vorschauseite Serie (fehlender Titel) | `#/titles/show/{tmdbId}` |
| M-05 | Detailseite Film (vorhanden) | `#/items/{key}` |
| M-06 | Detailseite Serie mit fehlenden Staffeln | `#/items/tulsa-king` |
| M-07 | Suche | `#/search` |
| M-08 | Meine Wünsche | `#/wishes` |
| M-09 | Benachrichtigungen | `#/notifications` |
| M-10 | Benachrichtigungskanäle | `#/settings/notifications` |
| M-11 | Profile (Profildialog mit Wunschrecht) | `#/settings/profiles` |
| M-12 | Profilwahl | `#/profiles` |
| M-13 | Dialog «Anfordern» (Film, Serie mit Staffelwahl, Mitwunsch, Sonderfälle) | von M-01 bis M-07 |
| M-20 | Verwaltung › Übersicht | `#/admin` |
| M-21 | Verwaltung › Automatisierung › Integrationen (alle Kacheln) | `#/admin/integrations` |
| M-22 | Integrationen › Jellyseerr | `#/admin/integrations/jellyseerr` |
| M-23 | Verwaltung › Medien › Wünsche › Anfragen | `#/admin/wishes` |
| M-24 | Wünsche › Zuordnung | `#/admin/wishes/mapping` |
| M-25 | Wünsche › Einstellungen | `#/admin/wishes/settings` |
| M-26 | Verwaltung › Nutzer & Geräte › Konten | `#/admin/accounts` |
| M-27 | Verwaltung › Startseite › Spotlight | `#/admin/spotlight` |
| M-28 | Verwaltung › Startseite › Startseite (Reihen) | `#/admin/home-rows` |
| M-29 | Verwaltung › Automatisierung › Benachrichtigungen | `#/admin/notifications` |
| M-30 | Verwaltung › Automatisierung › Hintergrundjobs | `#/admin/jobs` |
