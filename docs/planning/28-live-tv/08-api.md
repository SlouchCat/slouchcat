# Konzept 28 · API

Stand: 29. November 2026 · Fassung 3 · Status: Entwurf nach Gesamtreview, nicht freigegeben

Fassung 2 arbeitete Review 2 (`anhang/reviews/review-2-datenmodell-api.md`), `04` Fassung 3 und die Stufen aus
`18-umsetzungsplan.md` ein (Abschnitt 10). Fassung 3 folgt `04` Fassung 4 (Abschnitt 14, Folgeänderungen 15.3),
`12-sicherheit.md` Fassung 3 (wo strenger) und den Schlussreviews 4, 5 und 6 (Abschnitt 11). **Ohne deine Antwort
gilt die wortlauttreue Vorgabe**, bei Sicherheitsfragen die sichere; ein Feld, das erst eine andere Variante
bräuchte, steht nur als Hinweis «käme mit …», nie im Schema.

Dieses Dokument ist die **einzige Quelle** für die Schnittstellen von Live-TV: welche Operationen es gibt, wer sie
aufrufen darf, was sie annehmen und liefern, welche Fehler sie melden und wie die Ströme ausgeliefert werden. Die
Tabellen dahinter stehen in `07-datenmodell.md`. Es entsteht keine Zeile in `api/openapi.yaml`; die YAML-Blöcke
sind ausdrücklich **Entwurf** im Stil des Vertrags. Die Vertragsfassung (ab 1.45.0) vergibt erst der Bau.

Alles hier ist **spezifiziert**, nichts ist gebaut. «Gebaut» mit Datei und Zeile bezieht sich auf den Stand des
Arbeitsbaums (Vertrag 1.44.0, `api/openapi.yaml:4`). Kennungen: `O-n` offene technische Punkte, `F-n` Fragen an
dich (zählen eigen), `W-n` Widersprüche (Nummern aus `07` 12.3, neue ab W-25).

Begriffe, einmal erklärt: Eine **Operation** ist eine Anfrageart des Servers (Methode und Pfad, zum Beispiel
`GET /live/channels`), mit einer festen Kennung (`operationId`). Der **Vertrag** ist die Datei
`api/openapi.yaml`, aus der Server und Web-Client erzeugt werden. **Seitenweise Abfrage** heisst, dass eine lange
Liste in Stücken (`offset`, `limit`) geholt wird. **Polling** heisst, dass der Client in festen Abständen
nachfragt, statt dass der Server von sich aus meldet.

---

## 1. Grundsätze

### 1.1 Kein zweiter Stil

| Punkt | Festlegung | Beleg (gebaut) |
|---|---|---|
| Pfade | unter `/api/v1` (im Vertrag ohne Präfix); Zuschauer unter `/live/…`, Verwaltung unter `/admin/live/…`; Kennungen als Pfadparameter | `server/internal/api/api.go:24`; Muster `/radio/…`, `/admin/radio/…` (`api/openapi.yaml:11543-12838`) |
| JSON | über den erzeugten strict server; Felder in `camelCase`; Zeiten als Unix-Millisekunden (`integer`, `int64`) wie `RadioNow.since` | Regel 6; `api/openapi.yaml:11922-11929` |
| Streams, Bilder, Uploads | `http.Handler` ausserhalb des strict servers, im Vertrag beschrieben, `x-visibility`/`x-clients` in `binaryRoutes` wiederholt und vom Vertragstest abgeglichen | Regel 6; `server/internal/api/api.go:26-110` |
| Sichtbarkeit | jede Operation mit `x-visibility`; `scoped` und `account` mit `x-clients` | Regel 7, ADR 0040; Schema-Test |
| Fehler | RFC 9457 mit stabilem `code` (`type` bleibt `about:blank`); Feldfehler `validation_failed` (422) | `api/openapi.yaml:13428-13460`; `server/internal/api/session.go:611`, `:913` |
| Seiten | `offset`/`limit`, Antwort `{items, total, offset}` | `listRadioStations`, `RadioStationPage` (`api/openapi.yaml:11577-11591`, `:24487-24501`) |
| Filter | je Parameter ein Wert oder mehrere (ODER), verschiedene Parameter UND | `listRadioStations` (`api/openapi.yaml:11547-11553`) |
| Apps zu alt | jede Operation mit `x-clients` nennt `426` `AppTooOld` | `api/openapi.yaml:12863-12872` |
| Schreibende Anfragen | Kopf `X-Requested-With: Slouchcat` (CSRF) | `api/openapi.yaml`, Kopfbeschreibung |
| Tags | `live` (Zuschauer), `live-admin` (Verwaltung) | Muster `radio` |

### 1.2 Sichtbarkeitsstufen von Live-TV

| Wer | `x-visibility` | `x-clients` | Fachrecht |
|---|---|---|---|
| Zuschauer (Profil gewählt) | `scoped` | `[app, tv]` (beide Apps zeigen alle Zuschauerseiten, `14` 2) | im Dienst `live`: «Live-TV», «Aufnahmen», «Timeshift», Hauptprofil (`07` 4) |
| Profilverwaltung des Hauptprofils | `account` (bestehende Operationen) | `[app, tv]` (wie `updateProfile`, gebaut) | `requirePrimary` (gebaut: `server/internal/auth/profiles.go:40-45`) |
| Administrator | `admin` | – | – |

Es gibt **keine neue Stufe**: Die drei Schalter setzt der Dienst durch, nicht die Middleware (gebaut:
`server/internal/api/session.go:384` kennt nur `public, account, scoped, admin, owner`; Fachrecht im Dienst wie
`server/internal/wishes/rights.go:14-20`). Ohne wirksames Recht antwortet jede Live-Operation **404**
`not_found` (Abschnitt 3.1).

### 1.3 Vertragsfassung und Verträglichkeit

- Jede Stufe bringt eine Vertragsfassung mit Tag (Regel «Vertrag = Tag»); die erste ist 1.45.0, die Nummer vergibt
  der Bau.
- Alle Änderungen an bestehenden Operationen sind **additiv** im Sinn von `docs/dev/VERTRAEGLICHKEIT.md`: neue
  Operationen, neue freiwillige Felder in Anfragen, neue Felder in Antworten, neue Aufzählungswerte
  (`VERTRAEGLICHKEIT.md` «Nicht brechend»). Kein bestehendes Pflichtfeld wird freiwillig, kein Wert verschwindet.
- **Neue Aufzählungswerte (RI-6, A1):** Neue Werte in bestehenden Antworten (`MyMediaKey: livetv`,
  `NotificationKind: live_*`, Kanal-Ereignisse `live_*`, `AdminStream.kind`) sind nach
  `docs/dev/VERTRAEGLICHKEIT.md` «nicht brechend, die Apps vertragen Unbekanntes». Der Server liefert `livetv` in
  `listMyMedia` und Live-Einträge in `listNotifications` nur, wenn das Profil Live-TV wirksam hat (Recht, keine
  App-Fassung). Einen Filter nach App-Fassung gibt es **nicht** (RD-27): Verträgt ein erzeugter Kotlin-Client
  einen unbekannten Wert nicht, ist das ein Fehler der App gegen Konzept 27 11.5 und wird dort behoben (O-1). Der
  Test `TestXClientsCompatibility` (gebaut: `server/internal/api/contract_compat_test.go:16-35`) prüft die sieben
  Bruchregeln.
- **Vertragstest:** Jede neue Operation bekommt einen Eintrag in der Registry ihrer Stufe
  (`TestContractCoversEveryOperation`, gebaut: `server/internal/api/contract_access_test.go:562-600`), jede
  Binärroute einen Eintrag in `binaryRoutes`.
- Der Vertrag beschreibt Live-TV-Werte erst mit ihrem Erzeuger (ADR 0009), auch wenn die Datenbank sie schon kennt
  (`07` 11.2).

### 1.4 Antworten ohne Geheimnisse

Keine Antwort enthält eine Adresse einer Quelle, einer Senderquelle, eines Logos oder einer EPG-Quelle, keinen
Benutzer, kein Passwort, keinen Wert einer Kopfzeile (T-28). Felder dafür sind in Anfragen **nur schreibbar**;
«leer» oder «fehlt» heisst «unverändert». Antworten nennen `site` (Schema, Host, Port), die **Namen** der
Kopfzeilen und `hasCredentials: true`. Fehlertexte (`detail`) laufen durch die Schwärzung (`04` 10.4) und nennen
von einer Adresse nur Schema und Host (gebaut: `server/internal/netguard/netguard.go:132-138`).

---

## 2. Ressourcenübersicht

Spalten: **Recht** = zusätzlich nötiges Fachrecht (L = «Live-TV» wirksam, A = «Aufnahmen» wirksam, T = «Timeshift»
wirksam, H = Hauptprofil, Adm = Administrator mit Hauptprofil). **Seite** = Kennung aus `14` (LS-, LD-) und `15`
(LA-). **St.** = Stufe nach `18` 4.13. **Bin** = `http.Handler`. Alle Zuschauer-Operationen sind `scoped` mit
`x-clients: [app, tv]`; alle Verwaltungs-Operationen `admin` ohne `x-clients`. In welcher Stufe die Apps die Seiten
bekommen, regelt `18` Abschnitt 5 (Stufe «Apps»); der Vertrag ist ihnen voraus (`18` D-10).

### 2.1 Sender und Favoriten

| Methode | Pfad | `operationId` | Zweck | Recht | Seite | St. |
|---|---|---|---|---|---|---|
| GET | `/live/home` | `getLiveHome` | Startseite in einer Anfrage: jetzt, gleich, zuletzt, Favoriten, Genres, Zähler | L | LS-01 | 1a (Programmteile 2, Aufnahmezähler 5) |
| GET | `/live/channels` | `listLiveChannels` | Senderliste mit Filtern, Suche, Seiten; mit `now=true` jetzt/danach, mit `ids` für Zapliste und Banner | L | LS-02, LS-03, LS-04 (Senderfenster, Handy), LS-07 Zapliste und Banner, LD-03 Senderwahl | 1a (`now` 2) |
| GET | `/live/channels/facets` | `getLiveChannelFacets` | Zahlen je Gruppe, Land, Sprache, Kategorie | L | LS-02, LD-09 | 1a (Gruppen 1b) |
| GET | `/live/channels/{channelId}` | `getLiveChannel` | ein Sender (Kopf, Spuren-Kurzinfo, Zustand) | L | LS-07, LS-15 | 1a |
| GET | `/live/favorites` | `listLiveFavorites` | Favoriten in Reihenfolge | L | LS-03, LS-01 | 1a |
| PUT | `/live/favorites` | `setLiveFavoriteOrder` | ganze Reihenfolge setzen | L | LS-03 | 1a |
| PUT | `/live/favorites/{channelId}` | `addLiveFavorite` | Favorit setzen (idempotent) | L | LS-02, LS-15, LS-07 | 1a |
| DELETE | `/live/favorites/{channelId}` | `removeLiveFavorite` | Favorit entfernen (idempotent) | L | LS-02, LS-03 | 1a |
| GET | `/live/recent` | `listLiveRecentChannels` | zuletzt gesehene Sender (höchstens 20) | L | LS-01 | 1a |
| DELETE | `/live/recent` | `clearLiveRecentChannels` | Liste leeren (`03` DR-15) | L | LS-01 | 1a |
| GET (Bin) | `/live/images/channel/{channelId}/{size}` | `getLiveChannelLogo` | Senderlogo über den Server; für Administratoren auch inaktive Sender (S-4) | L oder Adm | LS-01 bis LS-15, LA-05, LA-06 | 1a |

### 2.2 Programm

| Methode | Pfad | `operationId` | Zweck | Recht | Seite | St. |
|---|---|---|---|---|---|---|
| GET | `/live/guide` | `getLiveGuide` | Raster: Zeitfenster × Senderseite | L | LS-04, Überlagerung im Player (`14` 4.9) | 2 |
| GET | `/live/channels/{channelId}/programmes` | `listLiveChannelProgrammes` | Tagesprogramm eines Senders | L | LS-15, LS-04 Handy | 2 |
| GET | `/live/programmes/{programmeRef}` | `getLiveProgramme` | Sendungsdetails samt eigenem Zustand und weiteren Terminen | L | LS-06 | 2 |
| GET | `/live/programmes` | `searchLiveProgrammes` | Programmsuche über alle Sender, «Nach Genre» | L | LS-05, LS-01 Genre | 2 |
| GET (Bin) | `/live/images/programme/{imageKey}/{size}` | `getLiveProgrammeImage` | Sendungsbild über den Server | L | LS-06, LS-01, LS-08 | 2 |

### 2.3 Live-Sitzung und Wiedergabe

| Methode | Pfad | `operationId` | Zweck | Recht | Seite | St. |
|---|---|---|---|---|---|---|
| POST | `/live/sessions` | `createLiveSession` | Sitzung anlegen und einen Sender einstellen oder eine Aufnahme abspielen | L (Aufnahme: L + A oder H) | LS-07, LS-10 | 1a (Aufnahme 5) |
| POST | `/live/sessions/{sessionId}/tune` | `tuneLiveSession` | umschalten in derselben Sitzung | L | LS-07, LD-10 | 1a |
| POST | `/live/sessions/{sessionId}/heartbeat` | `heartbeatLiveSession` | am Leben halten, Pause zur Anzeige melden, Zustand holen | L | LS-07, LS-10 | 1a |
| PUT | `/live/sessions/{sessionId}/output` | `setLiveSessionOutput` | Qualität, Tonspur, Untertitel | L | LS-07, LS-10 | 1a |
| POST | `/live/sessions/{sessionId}/seek` | `seekLiveSession` | «spiele ab Uhrzeit» (Rückfall aus `05` 4.5) | L + T (Aufnahme: A oder H) | LS-07, LS-10 | 4 |
| DELETE | `/live/sessions/{sessionId}` | `endLiveSession` | Sitzung beenden (idempotent) | – (eigene Sitzung) | LS-07, LS-10 | 1a |
| GET (Bin) | `/live/play/{sessionId}/{tune}/index.m3u8` | `getLivePlayMaster` | übergeordnete Liste mit Codecs | Token `lt` | LS-07, LS-10 | 1a |
| GET (Bin) | `/live/play/{sessionId}/{tune}/media.m3u8` | `getLivePlayMedia` | Wiedergabeliste der Sitzung | Token `lt` | LS-07, LS-10 | 1a |
| GET (Bin) | `/live/play/{sessionId}/{tune}/seg/{gen}/{n}` | `getLivePlaySegment` | TS-Segment | Token `lt` | LS-07, LS-10 | 1a |

### 2.4 Aufnahmen (Stufe 5)

| Methode | Pfad | `operationId` | Zweck | Recht | Seite | St. |
|---|---|---|---|---|---|---|
| GET | `/live/recordings` | `listLiveRecordings` | eigene Aufnahmen (`view=library`) oder kommende Termine (`view=upcoming`), mit `profileId` die eines Profils mit Altersgrenze (LS-14) | L + A (mit `profileId`: H) | LS-08, LS-11 «Termine», «Manuell», LS-14 | 5 |
| GET | `/live/recordings/usage` | `getLiveRecordingUsage` | Speicher des Kontos: eigenes Profil, übrige Profile, frei | L + A (oder H) | LS-08, LS-14 | 5 |
| POST | `/live/recordings/preview` | `previewLiveRecording` | Zeiten, erwartete Grösse, Warnungen vor dem Speichern | L + A | LD-01, LD-03, LD-04, LD-07 | 5 |
| POST | `/live/recordings` | `createLiveRecording` | Sofort-, Einzel- oder manuelle Aufnahme anlegen (idempotent) | L + A | LD-01, LD-03 (einmal), LD-04 | 5 |
| GET | `/live/recordings/{recordingId}` | `getLiveRecording` | Detail mit Lücken, Spuren, Herkunft, Aufbewahrung, Aktionen | L + A (oder H) | LS-09 | 5 |
| PATCH | `/live/recordings/{recordingId}` | `updateLiveRecording` | Vor-/Nachlauf, Zeiten und Sender (manuell), Aufbewahrung, behalten, verlängern, Warnung ausblenden, Kandidat wählen | L + A | LD-01 Ändern, LD-11, LS-07, LS-12 | 5 |
| POST | `/live/recordings/{recordingId}/stop` | `stopLiveRecording` | laufende Aufnahme stoppen, behalten | L + A | LS-07, LS-09, LD-06 | 5 |
| DELETE | `/live/recordings/{recordingId}` | `deleteLiveRecording` | löschen oder Termin absagen; `?skip=true` lässt einen Regeltermin aus | L + A (H: Profile mit Altersgrenze) | LD-06, LS-14 | 5 |
| POST | `/live/recordings/delete` | `deleteLiveRecordings` | mehrere löschen | L + A | LS-08 Auswahl | 5 |
| GET | `/live/warnings` | `listLiveRecordingWarnings` | Konflikte und Warnungen der eigenen Termine | L + A | LS-12, LS-01 Band, LS-11 | 5 |
| GET | `/live/profile-recordings` | `listProfileLiveRecordings` | Übersicht je Profil mit Altersgrenze (Name, Zahl, Grösse; geplant als Zahl nur bei `20` E-28-43 (B)); die Aufnahmen kommen über `listLiveRecordings?profileId=` (RD-28) | H (3.5) | LS-14, LS-08 Karte | 5 |

### 2.5 Regeln (Serie, wiederkehrend; Stufe 6)

| Methode | Pfad | `operationId` | Zweck | Recht | Seite | St. |
|---|---|---|---|---|---|---|
| GET | `/live/rules` | `listLiveRecordingRules` | Regeln mit Zählern und nächstem Termin | L + A | LS-11 «Serien», «Wiederkehrend» | 6 |
| POST | `/live/rules/preview` | `previewLiveRecordingRule` | kommende Termine, übersprungene mit Grund, Erkennbarkeit der Folgen | L + A | LD-02, LD-03 | 6 |
| POST | `/live/rules` | `createLiveRecordingRule` | Serie oder wiederkehrende Aufnahme anlegen (idempotent) | L + A | LD-02, LD-03 | 6 |
| GET | `/live/rules/{ruleId}` | `getLiveRecordingRule` | Regel mit Terminen und übersprungenen | L + A | LS-11 aufgeklappt | 6 |
| PATCH | `/live/rules/{ruleId}` | `updateLiveRecordingRule` | Regel ändern | L + A | LD-02, LD-03 | 6 |
| DELETE | `/live/rules/{ruleId}` | `deleteLiveRecordingRule` | beenden; `?recordings=keep|delete` | L + A | LD-06 | 6 |
| DELETE | `/live/rules/{ruleId}/episodes` | `clearLiveRuleEpisodes` | Folgengedächtnis leeren (`11` 2.4) | L + A | LS-11 | 6 |

### 2.6 Erinnerungen (Stufe 2)

| Methode | Pfad | `operationId` | Zweck | Recht | Seite | St. |
|---|---|---|---|---|---|---|
| GET | `/live/reminders` | `listLiveReminders` | offene Erinnerungen nach Tag | L | LS-13 | 2 |
| POST | `/live/reminders` | `createLiveReminder` | Erinnerung an eine künftige Sendung (idempotent für offene) | L | LD-05, LS-04 (Taste E) | 2 |
| PATCH | `/live/reminders/{reminderId}` | `updateLiveReminder` | Vorlauf ändern | L | LD-05 | 2 |
| DELETE | `/live/reminders/{reminderId}` | `deleteLiveReminder` | entfernen (Zeile weg, RD-20) | L | LD-05, LS-13 | 2 |

### 2.7 Verwaltung: Übersicht, Ströme, Diagnose

| Methode | Pfad | `operationId` | Zweck | Seite | St. |
|---|---|---|---|---|---|
| GET | `/admin/live/overview` | `getLiveOverview` | Zustand, Kennzahlen, «Braucht Aufmerksamkeit», Ressourcen, Programmdaten, Einrichtungsband, Warnung vor dem Update (S-7) | LA-01, LA-20, LA-25, LA-27 | 1a (Teile je Stufe) |
| GET | `/admin/live/ingests` | `listLiveIngests` | laufende Eingangsströme mit Zuschauern und Technik; Aufnahmen nur als Zahl | LA-12, LA-01 | 1a |
| POST | `/admin/live/ingests/{channelId}/reconnect` | `reconnectLiveIngest` | neu verbinden | LA-12 | 1a |
| POST | `/admin/live/ingests/{channelId}/next-source` | `switchLiveIngestSource` | zur nächsten Senderquelle wechseln | LA-12 | 3 |
| DELETE | `/admin/live/ingests/{channelId}` | `stopLiveIngest` | Eingang beenden | LA-12 | 1a |
| GET | `/admin/live/diagnostics` | `getLiveDiagnostics` | jetzt gestört, letzte 7 Tage, je Sender | LA-13 | 1a (Teile 3) |
| GET | `/admin/live/events` | `listLiveEvents` | Protokoll mit Filtern | LA-13 | 1a |

### 2.8 Verwaltung: Quellen und Importe

| Methode | Pfad | `operationId` | Zweck | Seite | St. |
|---|---|---|---|---|---|
| GET | `/admin/live/sources` | `listLiveSources` | alle Quellen mit Zustand und Zählern (auch «Sender ohne andere Quelle», S-6) | LA-02 | 1a |
| POST | `/admin/live/source-tests` | `testLiveSource` | neue oder bestehende (`sourceId`) Quelle testen; bei einer Senderliste ein Vorschau-Lauf (S-2, S-3) | LA-02, LA-03 Schritt 3–4 | 1a |
| POST | `/admin/live/sources` | `createLiveSource` | Quelle anlegen, optional aus einem Vorschau-Lauf | LA-03 Schritt 6 | 1a |
| GET | `/admin/live/sources/{sourceId}` | `getLiveSource` | eine Quelle (ohne Geheimnisse) | LA-03 Ändern | 1a |
| PATCH | `/admin/live/sources/{sourceId}` | `updateLiveSource` | ändern (Geheimnisse nur schreibbar) | LA-03 | 1a (Aktualisierung nach Zeitplan 1b, feste Ziele H) |
| DELETE | `/admin/live/sources/{sourceId}` | `deleteLiveSource` | entfernen, `?orphans=keep|delete` | LA-02 | 1a |
| POST | `/admin/live/sources/{sourceId}/refresh` | `refreshLiveSource` | Import jetzt (Job) | LA-02 | 1a |
| GET | `/admin/live/sources/{sourceId}/imports` | `listLiveSourceImports` | letzte Läufe | LA-02 | 1a |
| GET | `/admin/live/imports/{importId}` | `getLiveSourceImport` | Ergebnis oder Vorschau eines Laufs | LA-04 | 1a |
| GET | `/admin/live/imports/{importId}/items` | `listLiveSourceImportItems` | Einträge je Gruppe (Lauf oder Vorschau) | LA-04 | 1a |
| POST | `/admin/live/imports/{importId}/apply` | `applyHeldLiveSourceImport` | zurückgehaltene Liste trotzdem übernehmen | LA-04 | 1b |
| POST | `/admin/live/net-targets/resolve` | `resolveLiveNetTarget` | Hostname auflösen, Adressen zum Bestätigen zeigen | LA-03 Heimnetz | H |
| PUT (Bin) | `/admin/live/uploads/{kind}` | `uploadLiveFile` | M3U- oder XMLTV-Datei hochladen | LA-03, LA-10 | 1a (XMLTV 2) |

### 2.9 Verwaltung: Sender, Gruppen, Duplikate

| Methode | Pfad | `operationId` | Zweck | Seite | St. |
|---|---|---|---|---|---|
| GET | `/admin/live/channels` | `listAdminLiveChannels` | Tabelle für 10'000 Sender mit Filtern (schlanke Zeile, RD-28) | LA-05, LA-09 «Ausnahmen», LA-25 | 1a |
| GET | `/admin/live/channels/{channelId}` | `getAdminLiveChannel` | alle Angaben eines Senders samt Nutzung (S-5) und Laufendem | LA-06 | 1a |
| PATCH | `/admin/live/channels/{channelId}` | `updateLiveChannel` | Angaben, aktiv, Empfang, Logo-Art | LA-06, LA-05 Schalter | 1a (Empfang 3) |
| POST | `/admin/live/channels/{channelId}/end-now` | `endLiveChannelNow` | deaktivierten Sender «sofort beenden»: Wiedergaben und Aufnahmen enden (T-29) | LA-06, LA-12 | 1a (Aufnahmen 5) |
| DELETE | `/admin/live/channels/{channelId}` | `deleteLiveChannel` | löschen (Grabstein) | LA-06 | 1a |
| POST | `/admin/live/channels/bulk` | `bulkUpdateLiveChannels` | Massenbearbeitung, Nummern vergeben | LA-05 | 1b |
| PUT | `/admin/live/channels/{channelId}/streams` | `setLiveChannelStreams` | Rangfolge der Senderquellen | LA-06, LA-07 | 1a |
| POST | `/admin/live/channels/{channelId}/streams/{streamId}/split` | `splitLiveChannelStream` | als eigenen Sender abtrennen | LA-06 | 1b |
| POST | `/admin/live/streams/{streamId}/test` | `testLiveChannelStream` | eine Senderquelle prüfen | LA-06, LA-13 «Jetzt prüfen» | 1a |
| POST | `/admin/live/channels/merge/preview` | `previewLiveChannelMerge` | Zusammenfassung vor dem Zusammenführen, samt wegfallender Doppel | LA-07 | 1b |
| POST | `/admin/live/channels/merge` | `mergeLiveChannels` | zusammenführen | LA-07, LA-04, LA-05 | 1b |
| GET | `/admin/live/duplicates` | `listLiveDuplicates` | Vorschläge | LA-07 | 1b |
| POST | `/admin/live/duplicates/dismiss` | `dismissLiveDuplicate` | «Kein Duplikat» | LA-07, LA-04 | 1b |
| PUT (Bin) | `/admin/live/channels/{channelId}/logo` | `uploadLiveChannelLogo` | Logo hochladen | LA-06 | 1b |
| GET | `/admin/live/groups` | `listLiveGroups` | Gruppen | LA-08 | 1b |
| POST | `/admin/live/groups` | `createLiveGroup` | eigene Gruppe | LA-08 | 1b |
| PATCH | `/admin/live/groups/{groupId}` | `updateLiveGroup` | Name, sichtbar, Mitglieder | LA-08 | 1b |
| DELETE | `/admin/live/groups/{groupId}` | `deleteLiveGroup` | eigene Gruppe löschen | LA-08 | 1b |
| PUT | `/admin/live/groups/order` | `setLiveGroupOrder` | Reihenfolge | LA-08 | 1b |

### 2.10 Verwaltung: Empfang, Einstellungen, EPG, Speicher

| Methode | Pfad | `operationId` | Zweck | Seite | St. |
|---|---|---|---|---|---|
| GET | `/admin/live/config` | `getLiveConfig` | alle Einstellungen von Live-TV | LA-09, LA-10, LA-13, LA-14, LA-15, LA-16 | 1a (Abschnitte je Stufe) |
| PATCH | `/admin/live/config` | `updateLiveConfig` | einen oder mehrere Abschnitte speichern | wie oben | 1a |
| GET | `/admin/live/schedules` | `listLiveSchedules` | Zeitpläne mit Zahl der Sender | LA-09 | 3 |
| POST | `/admin/live/schedules` | `createLiveSchedule` | Zeitplan anlegen | LA-09 | 3 |
| PATCH | `/admin/live/schedules/{scheduleId}` | `updateLiveSchedule` | ändern | LA-09 | 3 |
| DELETE | `/admin/live/schedules/{scheduleId}` | `deleteLiveSchedule` | löschen, Sender fallen auf Standard | LA-09 | 3 |
| GET | `/admin/live/epg/sources` | `listLiveEpgSources` | EPG-Quellen mit Stand | LA-10, LA-01 | 2 |
| POST | `/admin/live/epg/source-tests` | `testLiveEpgSource` | Quelle testen (neu oder bestehend) | LA-10 | 2 |
| POST | `/admin/live/epg/sources` | `createLiveEpgSource` | anlegen | LA-10, LA-03 Angebot | 2 |
| PATCH | `/admin/live/epg/sources/{epgSourceId}` | `updateLiveEpgSource` | ändern | LA-10 | 2 |
| DELETE | `/admin/live/epg/sources/{epgSourceId}` | `deleteLiveEpgSource` | entfernen (samt ihrer Kanäle in `epg.db`) | LA-10 | 2 |
| POST | `/admin/live/epg/sources/{epgSourceId}/refresh` | `refreshLiveEpgSource` | jetzt laden (Job) | LA-10 | 2 |
| POST | `/admin/live/epg/sources/{epgSourceId}/accept-held` | `acceptHeldLiveEpgImport` | zurückgehaltene Datei trotzdem übernehmen | LA-10 | 2 |
| PUT | `/admin/live/epg/sources/order` | `setLiveEpgSourceOrder` | Reihenfolge für die Zuordnung | LA-10 | 2 |
| GET | `/admin/live/epg/sources/{epgSourceId}/imports` | `listLiveEpgImports` | Importverlauf | LA-10 | 2 |
| GET | `/admin/live/epg/mappings` | `listLiveEpgMappings` | Zuordnungen mit Zählern und Vorschau | LA-11, LA-01 | 2 |
| PUT | `/admin/live/epg/mappings/{channelId}` | `setLiveEpgMapping` | von Hand zuordnen oder «bewusst ohne» | LA-11, LA-06 | 2 |
| DELETE | `/admin/live/epg/mappings/{channelId}` | `removeLiveEpgMapping` | Zuordnung entfernen | LA-11 | 2 |
| POST | `/admin/live/epg/mappings/accept-suggestions` | `acceptLiveEpgSuggestions` | Vorschläge bestätigen (einzeln oder alle Rang 90) | LA-11 | 2 |
| POST | `/admin/live/epg/mappings/auto` | `runLiveEpgAutoMapping` | «Automatisch zuordnen» | LA-11 | 2 |
| GET | `/admin/live/epg/channels` | `searchLiveEpgChannels` | EPG-Kanäle suchen, mit zwei nächsten Sendungen | LA-11, LA-06 | 2 |
| GET | `/admin/live/storage` | `getLiveStorage` | Speicherorte, Belegung je Konto, verwaiste Dateien | LA-16, LA-01 | 4 (Aufnahmen 5) |
| GET | `/admin/live/storage/candidates` | `listLiveStorageCandidates` | schreibbar eingebundene Ordner | LA-16 Wechsel | 4 |
| POST | `/admin/live/storage/{role}/validate` | `validateLiveStorageLocation` | Prüfungen für einen Ordner | LA-16 Wechsel | 4 |
| PUT | `/admin/live/storage/{role}` | `setLiveStorageLocation` | Speicherort setzen oder wechseln | LA-16 Wechsel | 4 |
| POST | `/admin/live/storage/{role}/check` | `checkLiveStorageLocation` | jetzt prüfen | LA-16 | 4 |
| POST | `/admin/live/storage/cleanup` | `cleanupLiveStorage` | verwaiste Puffer jetzt aufräumen | LA-16 | 4 |
| DELETE | `/admin/live/storage/orphans` | `deleteLiveOrphanRecordingFiles` | verwaiste Aufnahmedateien löschen (Rückfrage) | LA-16 | 5 |

Die Rolle `startbuffer` gibt es in den Speicher-Operationen nur bei `04` F-2 (a) (`18` D-3).

### 2.11 Neue Operation in der Profilverwaltung

| Methode | Pfad | `operationId` | `x-visibility` | `x-clients` | Zweck | Seite | St. |
|---|---|---|---|---|---|---|---|
| GET | `/profiles/{id}/live-usage` | `getProfileLiveUsage` | `account` | `[app, tv]` | Zahl und Grösse der Aufnahmen eines Profils für den Löschdialog (`03` DR-04); nur das Hauptprofil, keine Titel | LS-21, Löschdialog | 5 |

### 2.12 Erweiterungen bestehender Operationen (additiv, keine neue Operation)

| Operation (gebaut) | Erweiterung | Seite | St. |
|---|---|---|---|
| `getSession` (`/auth/session`) | `SessionInfo.live`: wirksame Rechte, Grund, ob LS-14 erreichbar ist (3.10) | alle | 1a (Felder je Stufe) |
| `listProfiles`, `createProfile`, `updateProfile` | `Profile.canLiveTv` (1a), `canLiveTimeshift` (4), `canLiveRecord` (5) und `liveCapped` | LS-21 | 1a, 4, 5 |
| `listAccounts`, `updateAccountAccess` | `Account.live`: Schalter je Stufe, `quotaBytes`, `usedBytes`, `recordingsCount`, `overQuota` (5) | LA-22 | 1a, 4, 5 |
| `listInvitations`, `createInvitation` | dieselben Schalter und `liveQuotaBytes` | LA-22 | 1a, 4, 5 |
| `listMyMedia`, `getMyMedia`, `setMyMedia` | `MyMediaKey` bekommt `livetv`; `listMyMedia` liefert den Eintrag nur mit Recht | LS-20, LA-26 | 1a |
| `listNotifications` | neue `NotificationKind`-Werte `live_*` und ein Block `live` (Ziel, Titel, Sender, Zeiten; beim Hauptprofil für erwachsene Profile ohne Titel) | LS-23 | 2 (Aufnahmearten 5) |
| `getMyChannels`, `updateMyChannel` | neue Ereignisse `live_reminder`, `live_recording_done`, `live_recording_failed` (W-21 in `07`); Feld `liveTitles` des aktiven Profils (SM-56, 3.10) | LS-22 | 2, 5 |
| `listStreams`, `stopStream` | `AdminStream.kind` `live` und `live_recording`; für Aufnahmen ohne Titel und Sender und nur mit grober Technik (ohne Auflösung, Bildrate und Bitrate der Quelle; `12` SM-45, RT-19) | LA-21 | 1a (Aufnahme 5) |
| `getDashboard` | Block `live` (Zustand, Zuschauer, laufende Aufnahmen, gestörte Sender) | LA-20 | 1a |
| `listJobRuns` | neue Jobarten `livetv.*` in einer neuen Gruppe | LA-23 | 1b, 2, 5, 6 |
| `getWatchReport` | zählt Live-TV-Schauzeit mit, ohne Sender | – | 1a |
| `installUpdate` | neuer Parameter `confirmLiveRecordings`; ohne ihn bei laufenden oder in 15 Minuten beginnenden Aufnahmen 409 `live_recordings_running` (`04` F-6 (a), 14.8) | LA-27 | 5 |
| `PlaybackCapabilities` (Schema) | Videocodec `mpeg2video`, Toncodecs `mp2`, `aac_latm`, Feld `interlacedOk`, Untertitel `dvb_subtitle`, `dvb_teletext` (`04` 7.1) | LS-07 | 1a |

### 2.13 Zählung

| Gruppe | Operationen | davon Binär |
|---|---|---|
| Sender und Favoriten | 11 | 1 |
| Programm | 5 | 1 |
| Sitzung und Wiedergabe | 9 | 3 |
| Aufnahmen | 11 | 0 |
| Regeln | 7 | 0 |
| Erinnerungen | 4 | 0 |
| Verwaltung Übersicht, Ströme, Diagnose | 7 | 0 |
| Verwaltung Quellen und Importe | 13 | 1 |
| Verwaltung Sender, Gruppen, Duplikate | 19 | 1 |
| Verwaltung Empfang, Einstellungen, EPG, Speicher | 28 | 0 |
| Profilverwaltung | 1 | 0 |
| **Neu insgesamt** | **115** (Fassung 1: 122) | **7** |
| Erweiterte bestehende Operationen | 20 (dazu das Schema `PlaybackCapabilities`) | – |

Je Stufe (neue Operationen): 1a 45, 1b 13, 2 24, 3 5, H 1, 4 7, 5 13, 6 7. `18` 4.3 bis 4.9 nennt dieselben
Zahlen (Abschnitt 9.3, W-33 erledigt).

---

## 3. Modelle, Fehler, Seiten und Filter je Gruppe

Die YAML-Blöcke sind **Entwurf** im Stil von `api/openapi.yaml` (Komponenten unter `components/schemas`). Gezeigt
sind die tragenden Modelle; Antworten ohne eigenen Körper sind `204`.

### 3.1 Gemeinsame Fehlerregeln

| Status | `code` | Wann | neu |
|---|---|---|---|
| 401 | `unauthenticated` | keine gültige Anmeldung bei JSON-Operationen | gebaut |
| 403 | `profile_required` | `scoped` ohne gewähltes Profil | gebaut |
| 403 | `client_not_allowed` | Bearer einer Art, die `x-clients` nicht nennt | gebaut |
| **404** | `not_found` | **kein wirksames Recht**, Sender unsichtbar, Aufnahme, Regel, Erinnerung, Sitzung oder Import eines anderen Profils oder unbekannt, Token falsch, fremd, abgelaufen oder widerrufen. **Nie 403 für Unsichtbares** (`03` 2.3). Gleicher Körper in allen Fällen. | Regel gebaut, Anwendung neu |
| 409 | `stream_limit_reached` | Grenze gleichzeitiger Ströme von Profil oder Konto (mit `extensions.maxStreams`, `activeStreams`) | gebaut (`api/openapi.yaml:2233-2240`), jetzt auch für Live |
| 403 | `watch_limit_reached` | tägliche Schauzeit aufgebraucht (`extensions.limitMinutes`, `usedMinutes`) | gebaut (Radio, `api/openapi.yaml:11801-11806`) |
| 409 | `live_conflict` | Zustand passt nicht (laufende Aufnahme löschen, Sitzung einer anderen Art umschalten, Zeitplan ist Vorgabe) mit `extensions.reason` | neu |
| 409 | `live_superseded` | ein neueres Einstellen derselben Sitzung hat diese wartende Antwort überholt; Haltegrund und Reservierung sind zurückgegeben, der Client verwirft die Antwort (`04` 9, RB-15) | neu |
| 409 | `live_import_running` | Import der Quelle läuft schon | neu |
| 409 | `live_recordings_running` | Update ohne Bestätigung bei laufenden oder in 15 Minuten beginnenden Aufnahmen | neu |
| 200 statt 201 | – | gleiches Anlegen noch einmal (Aufnahme, Regel, Erinnerung): Antwort ist die bestehende Zeile (RD-10, `07` 3.9) | neu |
| 410 | `live_session_gone` | **eigene** Sitzung endete (Neustart, Entzug, Ablauf, Sender oder Quelle weg, 15 Minuten ohne Segmentabruf): der Client legt eine neue an. Eine fremde oder unbekannte Sitzung ist 404 (RT-12) | neu (Muster Radio 410, `api/openapi.yaml:11873-11878`) |
| 422 | `validation_failed` | Feldfehler mit `errors[]` (Feld und Code) | gebaut |
| 422 | `live_programme_past` | Sendung ist vorbei (Erinnerung, Einzelaufnahme) | neu |
| 422 | `live_no_epg` | Aktion braucht Programmdaten (Einzel-, Serienaufnahme, Erinnerung bei Sender ohne EPG, `03` DR-09) | neu |
| 429 | `live_too_fast` | Drosselung (Abschnitt 8) mit Kopf `Retry-After` | neu (Muster `too_many_attempts`, `api/openapi.yaml:271-276`) |
| – | `live_source_unreachable`, `live_source_no_picture`, `live_start_timeout`, `live_source_rejected` | Startversuch gescheitert: kein Fehlerstatus der Anfrage, sondern `state: start_failed` mit `startError.code` in Antwort oder Herzschlag; die Sitzung bleibt (3.4, `04` 9) | neu |
| 503 | `transcode_busy` | Umwandlungsbudget nach 10 s erschöpft | gebaut (`api/openapi.yaml:2252-2258`) |
| 503 | `live_capacity` | Serverdeckel: Eingangsströme, Sitzungen oder Byte-Budget voll (`04` 8.5), mit `extensions.limit` | neu (W-25) |
| 503 | `live_storage_unavailable` | Speicherort fehlt oder nicht beschreibbar | neu |
| 503 | `live_epg_rebuilding` | `epg.db` wird nach einem Fassungswechsel neu gebaut (Programm-Operationen; Fernsehen läuft) | neu |
| 507 | `live_quota_full` | Sofortaufnahme bei vollem Kontingent (`14` LD-04) | neu |
| 507 | `live_storage_full` | Speicherort unter Mindestreserve (Sofortaufnahme) | neu |

`type` bleibt `about:blank` wie im Bestand; der stabile Bezeichner ist `code` (gebaut: `api/openapi.yaml:13433-13457`).
Neue Codes sind für bestehende Operationen kein Bruch, weil sie nur an neuen Operationen hängen.

**Reihenfolge der Prüfungen (RT-12, `04` 10.3, `12` 9.3, SM-52):** Jede Live-Operation und jede Live-Route prüft
fest in dieser Reihenfolge und antwortet mit dem ersten Fehler: (1) Anmeldung (401 wie heute; an `lt`-Routen 404),
(2) CSRF-Kopf (403 `csrf_header_missing` wie heute), (3) wirksames Recht (404), (4) Objekt sichtbar und eigenes
(404), **erst dann** (5) Drosselung, Serverdeckel und Zustände (429, 503, 409, 403 `watch_limit_reached`, 410). Ein
Profil ohne Recht bekommt so immer 404, auch wenn gedrosselt wird, der Deckel voll ist oder `epg.db` neu gebaut
wird. Sammelantworten (`deleteLiveRecordings.skipped`) nennen fremde und unbekannte Kennungen mit demselben Grund
`not_found`. Vorgabe (`20` E-28-42 (b)): keine Ausnahme von (3). Nur wenn Marco Variante (a) wählt, gilt A-12 (3.5):
Das Hauptprofil sieht und löscht Aufnahmen seiner Profile mit Altersgrenze auch ohne eigenes wirksames «Live-TV».

### 3.2 Sender und Favoriten

```yaml
# Entwurf
LiveChannel:
  type: object
  required: [id, name, state, favorite, logo]
  properties:
    id: {type: integer, format: int64, minimum: 1}
    number: {type: integer, minimum: 1, maximum: 9999}
    name: {type: string}
    country: {type: string, minLength: 2, maxLength: 2}
    language: {type: string}
    categories: {type: array, items: {$ref: "#/components/schemas/LiveCategory"}}
    groupIds: {type: array, items: {type: integer, format: int64}}
    state:
      type: string
      enum: [ok, disturbed, unknown]
      description: |
        Server-side health of the channel (any stream failing for longer than the threshold = disturbed). A
        disturbed channel stays visible and may be tuned (14 LS-02).
    noAudio: {type: boolean}
    noVideo: {type: boolean}
    favorite: {type: boolean}
    logo:
      $ref: "#/components/schemas/LiveImageRef"      # url with content hash; absent hash: initials
    hasEpg: {type: boolean}
    now: {$ref: "#/components/schemas/LiveProgrammeBrief"}    # only with now=true
    next: {$ref: "#/components/schemas/LiveProgrammeBrief"}
    badges:
      type: array
      items: {type: string, enum: [recording, recording_planned, series_planned, reminder]}
      description: Only the active profile's own planning; never other profiles' (14 4.10).
LiveCategory:
  type: string
  enum: [general, news, sports, movies_series, documentary, kids, music, culture, regional, entertainment, other]
LiveChannelPage:
  type: object
  required: [items, total, offset]
  properties:
    items: {type: array, items: {$ref: "#/components/schemas/LiveChannel"}}
    total: {type: integer, minimum: 0}
    offset: {type: integer, minimum: 0}
LiveFavoriteOrder:
  type: object
  required: [channelIds]
  properties:
    channelIds: {type: array, items: {type: integer, format: int64}, maxItems: 2000}
```

**`listLiveChannels` · Parameter:** `q` (Name oder Nummer, gefaltet), `group` (mehrere; `favorites` als
Sondergruppe), `country`, `language`, `category` (je mehrere), `favorites` (bool), `ids` (bis 200, für Zapliste
und Banner; ersetzt die frühere Operation `getLiveNowNext`, S-1), `now` (bool: jetzt/danach mitliefern, Stufe 2),
`sort` (`number` Vorgabe, `name`), `offset` (ab 0),
`limit` (1 bis 200, Vorgabe 100). Reihenfolge `number`: nach Nummer, ohne Nummer nach Name (`15` F-15). Eine
Senderliste mit 10'000 Einträgen holt die virtuelle Liste seitenweise (`14` 4.1); `total` zeigt die volle Zahl
für die Bildlaufleiste. Antwortgrösse je Seite mit `now=true`: rund 100 × 0,5 KB (Annahme).

**`getLiveChannelFacets`:** dieselben Filter; je Facette Werte mit Zahl, die Zahlen einer Facette ignorieren ihre
eigene Auswahl (Muster `getRadioFacets`, gebaut: `api/openapi.yaml:11616-11620`). Gruppen nur `visible`.

**`getLiveHome`:** `{now: [LiveProgrammeCard], soon: [...], recent: [LiveChannel], favorites: [LiveChannel],
genres: [{name, count}], counts: {recordingsRunning, scheduled, reminders, warnings}}`; Reihen höchstens 24
Einträge; `soon` = Beginn in den nächsten 60 Minuten; `genres` = Genres mit Treffern in den nächsten 3 Stunden
(`14` 3.1). Ohne Programmdaten fehlen `now`, `soon`, `genres`, und `epgAvailable: false`. `counts` nur mit
Recht «Aufnahmen».

**Fehler:** 404 (kein Recht, Sender unsichtbar), 422 (`limit` ausserhalb). `setLiveFavoriteOrder`: 422, wenn die
Menge nicht genau den Favoriten entspricht (Muster gebaut: `api/openapi.yaml:11710-11734`); unsichtbare Favoriten
gehören zur Menge nicht und behalten ihren Platz am Ende.

### 3.3 Programm

```yaml
# Entwurf
LiveProgrammeRef:
  type: string
  pattern: "^[0-9]+-[0-9]+-[0-9a-f]{8}$"
  description: |
    channelId-startMs-titleHash (09 6.2): stable across imports while the programme keeps channel, start and
    title; resolved again on every request. A programme no longer found answers 404 (the client says «nicht
    mehr im Programm»).
LiveProgrammeBrief:
  type: object
  required: [ref, title, start]
  properties:
    ref: {$ref: "#/components/schemas/LiveProgrammeRef"}
    title: {type: string}
    subtitle: {type: string}
    start: {type: integer, format: int64}
    stop: {type: integer, format: int64, description: Absent when the end is unknown (09 4, end_unknown).}
    flags:
      type: array
      items: {type: string, enum: [end_derived, end_unknown, time_derived, time_ambiguous, shifted, overlap, new]}
LiveProgramme:
  allOf:
    - $ref: "#/components/schemas/LiveProgrammeBrief"
    - type: object
      required: [channel, actions]
      properties:
        channel: {$ref: "#/components/schemas/LiveChannel"}
        description: {type: string}
        genres: {type: array, items: {type: string}, maxItems: 10}
        episode:
          type: object
          properties:
            season: {type: integer, minimum: 1}     # xmltv_ns + 1, format conversion only (09 7.1)
            number: {type: integer, minimum: 1}
            onscreen: {type: string}
            reliable: {type: boolean, description: Stage A or B of 09 7.1.}
        year: {type: integer}
        image: {$ref: "#/components/schemas/LiveImageRef"}
        epgAsOf: {type: integer, format: int64, description: Last successful import of the source («Programmdaten vom»).}
        state: {type: string, enum: [past, live, upcoming]}
        actions:
          type: object
          description: The server decides (rule 12); the client shows a button only when true.
          required: [watch, record, recordSeries, remind]
          properties:
            watch: {type: boolean}
            record: {type: boolean}
            recordSeries: {type: boolean}
            remind: {type: boolean}
        mine:
          type: object
          properties:
            recordingId: {type: string}
            recordingState: {type: string}
            ruleId: {type: string}
            reminderId: {type: string}
        moreAirings: {type: array, items: {$ref: "#/components/schemas/LiveProgrammeBrief"}, maxItems: 20}
LiveGuide:
  type: object
  required: [from, to, rows, total, offset]
  properties:
    from: {type: integer, format: int64}
    to: {type: integer, format: int64}
    rows:
      type: array
      items:
        type: object
        required: [channel, programmes]
        properties:
          channel: {$ref: "#/components/schemas/LiveChannel"}
          programmes: {type: array, items: {$ref: "#/components/schemas/LiveProgrammeBrief"}}
          epgAsOf: {type: integer, format: int64}
    total: {type: integer, description: Number of channels of the filter (all pages).}
    offset: {type: integer}
```

**`getLiveGuide` · Parameter:** `from` (ms, Pflicht), `to` (ms, Pflicht, höchstens 6 Stunden nach `from`), Filter
wie `listLiveChannels` (`group`, `favorites`), `offset`, `limit` (Sender je Seite, 1 bis 100, Vorgabe 50, `09`
8.1). `from` darf nicht vor `jetzt − Rückschau` liegen (`live_config.epg_lookback_hours`) und nicht nach dem Ende
des Vorschauzeitraums; die Oberfläche beginnt bei heute 00:00 (`14` 4.1; Unterschied zu `09` 2.7 «zurückblättern
bis gestern Abend» ist nur Darstellung, W-34). Sendungen, die vor `from` beginnen und danach enden, sind enthalten
(Untergrenze `from − 24 h`, `09` 8.1).

Jetzt und danach für Zapliste und Banner liefert `listLiveChannels?ids=…&now=true` (S-1).

**`listLiveChannelProgrammes`:** `day` (Datum `YYYY-MM-DD` in der Zone `tz`), `tz` (IANA-Name des Geräts, Pflicht;
der Server schneidet den Tag in dieser Zone, Tagesgrenzen rechnet sonst der Client, `09` 2.4) → Sendungen des
Tages und Lücken als eigene Einträge `{gapFrom, gapTo}` (`14` LS-15).

**`searchLiveProgrammes`:** `q` (mindestens 2 Zeichen, Präfixsuche über Titel und Untertitel, `09` 8.4), `genre`
(mehrere), `when` (`now`, `today`, `tomorrow`, `week`; Vorgabe `week`), `tz`, `group`, `favorites`, `offset`,
`limit` (1 bis 100, Vorgabe 50; höchstens 100 Treffer insgesamt, `09` 8.4). Antwort `{items, total, offset,
genres: [{name, count}], missingEpgChannels: n}`. Ohne `q` und mit `genre` ist das der Einstieg «Nach Genre»
(LS-01). Keine Altersfilterung (LTV-015.06). Die Beschreibung ist in Stufe 2 nicht durchsucht (`09` O-11), obwohl
`14` LS-05 «Beschreibung» nennt (W-26).

**Fehler:** 404 (kein Recht, Sender unsichtbar, Sendung nicht mehr auffindbar), 422 (Fenster zu lang, `q` zu
kurz, `from` ausserhalb), 503 `live_epg_rebuilding` (neu: `epg.db` wird nach Fassungswechsel neu gebaut; der Client
zeigt «Keine Programmdaten», der Sender läuft, `09` 3.6).

### 3.4 Live-Sitzung

```yaml
# Entwurf
LiveSessionRequest:
  type: object
  description: Exactly one of channelId and recordingId.
  properties:
    channelId: {type: integer, format: int64}
    recordingId: {type: string, minLength: 26, maxLength: 26}
    # no deviceId: the server takes the device from the login (app bearer device, browser login cookie; RT-11)
    capabilities: {$ref: "#/components/schemas/PlaybackCapabilities"}   # additively extended (2.12)
    output: {$ref: "#/components/schemas/LiveOutputRequest"}
    zapList:
      $ref: "#/components/schemas/LiveZapList"      # list the client switches through (05 6, preload)
LiveZapList:
  type: object
  properties:
    kind: {type: string, enum: [all, group, favorites]}
    groupId: {type: integer, format: int64}
LiveTuneRequest:
  type: object
  required: [channelId]
  properties:
    channelId: {type: integer, format: int64}
    direction: {type: string, enum: [next, previous, direct], description: For preload direction (05 6).}
    zapList: {$ref: "#/components/schemas/LiveZapList"}
LiveOutputRequest:
  type: object
  properties:
    quality: {type: string, enum: [auto, original, 1080p, 720p, 480p, 360p]}
    audioTrack: {type: integer, minimum: 0, description: Index into LiveSession.tracks.audio.}
    subtitleTrack: {type: integer, minimum: -1, description: "-1 = off."}
LiveSession:
  type: object
  required: [id, kind, tune, playlistUrl, heartbeatSeconds, state, decision, timeshift, tracks]
  properties:
    id: {type: string}
    kind: {type: string, enum: [live, recording]}
    tune: {type: integer, minimum: 1, description: Tune sequence; part of the playlist path and of the token.}
    playlistUrl:
      type: string
      description: Master playlist, relative to /api/v1, carrying the token lt as query parameter (T-21).
    startPosition:
      type: number
      description: Seconds from the start of the playlist where the client starts (05 4.4, 5.3).
    heartbeatSeconds: {type: integer, enum: [10]}     # 04 14.4
    state:
      type: string
      enum: [starting, start_failed, live, shifted, paused, reconnecting, recording_live, recording_done]
      description: |
        Same list in LiveSession and LiveHeartbeat; a source switch is a notice, not a state (RD-26).
        reconnecting only for a running playback whose source breaks; a start that does not succeed ends in
        start_failed with startError, and the session stays (04 9).
    startingReason:
      type: string
      enum: [connecting, waiting_picture, waiting_transcode, first_segment, retrying]
      description: With state starting (04 5.3, three deadlines); retrying = one immediate attempt on a waiting or disturbed stream.
    startError: {$ref: "#/components/schemas/LiveStartError"}
    channel: {$ref: "#/components/schemas/LiveChannel"}
    recording: {$ref: "#/components/schemas/LiveRecordingBrief"}
    decision:
      type: object
      required: [path]
      properties:
        path: {type: string, enum: [original, audio_transcode, video_transcode]}
        reason: {type: string, description: Stable reason code like playback.Decide reasons (04 7.1).}
        hardware: {type: boolean}
        cappedTo: {type: string, description: Quality cap from the profile limits («Höchstens 720p», 14 4.12).}
    timeshift:
      type: object
      required: [enabled]
      properties:
        enabled: {type: boolean, description: False without the right, with full cap or missing storage (10 5).}
        reason: {type: string, enum: [no_right, disabled, storage_full, storage_missing]}
        maxMinutes: {type: integer}
        windowStart: {type: integer, format: int64, description: Program date-time of the rewind limit (tune time, 04 F-7).}
    tracks:
      type: object
      properties:
        audio: {type: array, items: {$ref: "#/components/schemas/LiveTrack"}}
        subtitles: {type: array, items: {$ref: "#/components/schemas/LiveTrack"}}
        qualities: {type: array, items: {type: string}}
    actions:
      type: object
      properties:
        pause: {type: boolean}
        record: {type: boolean}
    nowNext: {type: object, properties: {now: {$ref: "#/components/schemas/LiveProgrammeBrief"}, next: {$ref: "#/components/schemas/LiveProgrammeBrief"}}}
LiveStartError:
  type: object
  required: [code]
  description: The start attempt ended; hold and reservation are released, the session may tune again (04 9).
  properties:
    code:
      type: string
      enum: [live_source_unreachable, live_source_no_picture, live_start_timeout, live_source_rejected, transcode_busy, live_capacity]
      description: |
        live_source_unreachable: no first byte within deadline 1 (15 s) on every source. live_source_no_picture:
        no tables and keyframe within deadline 2. live_start_timeout: no first segment within deadline 3 or 90 s in
        total. live_source_rejected: «Quelle lehnt eine weitere Verbindung ab» (04 5.3, pattern A or single stream).
LiveHeartbeatRequest:
  type: object
  properties:
    playing:
      type: boolean
      description: |
        Reported pause, for display and for throttling a transcode only (04 10.3, 12 9.2, RT-04). Watch time
        counts every delivered, distinct segment regardless of this flag. Without a session buffer a reported
        pause ends the session after 60 s. Absent = playing.
    positionDateTime: {type: integer, format: int64, description: Program date-time of the playback position.}
    positionMs: {type: integer, format: int64, description: Recording sessions only: position for the progress.}
    zapUiOpen: {type: boolean, description: The channel list or zap overlay is open (05 6, preload restarts).}
LiveHeartbeat:
  type: object
  required: [state, expiresInSeconds]
  properties:
    state: {type: string, enum: [starting, start_failed, live, shifted, paused, reconnecting, recording_live, recording_done]}
    startingReason: {type: string, enum: [connecting, waiting_picture, waiting_transcode, first_segment, retrying]}
    startError: {$ref: "#/components/schemas/LiveStartError"}
    expiresInSeconds: {type: integer, description: 60 without a further heartbeat (T-21, 04 14.4).}
    liveEdge: {type: integer, format: int64}
    windowStart: {type: integer, format: int64, description: Moves when the buffer is capped or full (10 3.3).}
    notices:
      type: array
      items: {type: string, enum: [timeshift_shortened, timeshift_unavailable, transcode_too_slow, source_switched, retry_in]}
    retryInSeconds: {type: integer, description: With state reconnecting (14 4.12 «Nächster Versuch in 20 s»).}
    remainingWatchMinutes: {type: integer}
    playlistUrl: {type: string, description: New token after 11 hours (T-21, 12 hours per tune), only while channel and source are active (T-29); the client swaps it.}
    nowNext: {type: object, description: Present when the programme changed since the last heartbeat.}
```

**Ablauf:** `createLiveSession` legt die Sitzung an und stellt ein; `tuneLiveSession` stellt in derselben Sitzung
um (neue `tune`, neue `playlistUrl`, keine neue Sitzung, T-9); Reservierung bei den Stream-Grenzen und beim
Umwandlungsbudget bleibt beim Umschalten (T-13, T-26). **Die Antwort wartet bis zu 3 Sekunden auf das erste
Segment** (`04` 9, 14.1; `05` 4.4): liegt es vor, ist `state: live`, sonst `state: starting` mit `startingReason`
und derselben Listenadresse. Ein schon empfangener Sender antwortet sofort. Der Client lädt die Liste **nur** im
Zustand `live`, also wenn Antwort oder Herzschlag ihn melden (RB-15).

**Drei Startfristen** (`04` 5.3, 14.1): (1) bis zum ersten Byte je Verbindungsversuch 15 Sekunden; (2) vom ersten
Byte bis «läuft» (Tabellen und erster Keyframe) die gespeicherte Bildgruppe plus 5 Sekunden, mindestens 15,
höchstens 45, ohne gespeicherte Bildgruppe 30; (3) von «läuft» und Zulassung bis zum ersten Segment der Sitzung im
Direktweg die gespeicherte Segmentdauer plus 5, mindestens 10 Sekunden, mit Umwandlung die Wartezeit auf Einheiten
(bis 10) plus 10. Zusammen höchstens 90 Sekunden. Läuft eine Frist ab, meldet der Herzschlag `state:
start_failed` mit `startError.code`; Haltegrund und Reservierung sind zurückgegeben, die Sitzung bleibt und kann
neu einstellen (RB-07). `reconnecting` mit `retryInSeconds` gibt es nur für eine **laufende** Wiedergabe, deren
Quelle abbricht. Auf einem wartenden oder gestörten Strom löst ein neuer Zuschauer **einen** sofortigen Versuch
aus (höchstens einer je Minute und Sender; `startingReason: retrying`); scheitert er, folgt `start_failed`.

**Überholtes Einstellen (RB-15):** Kommt ein neues `tuneLiveSession` derselben Sitzung, während die Antwort auf das
vorige noch wartet, endet die ältere sofort mit `409 live_superseded`; es gibt je Sitzung nie zwei wartende
Einstellvorgänge. Ob die erste Liste in der Antwort mitkommt, ist `05` O-17 (Feld bis dahin nicht im Vertrag).
`setLiveSessionOutput` erzeugt eine neue Ausgabe-Generation (`04` 7.5); die Antwort ist die neue `LiveSession`.

**Ein Gerät, eine Sitzung (RD-10, RT-11):** Das Gerät nimmt der Server aus der Anmeldung (Gerät des App-Bearers,
im Browser eine Kennung im Cookie der Anmeldung), nie aus der Anfrage. Legt dasselbe Profil auf demselben Gerät
eine neue Sitzung an, endet eine offene ältere, bevor die Stream-Grenze zählt.

**Wann eine Sitzung endet** (`04` 10.3, 14.4): 60 Sekunden ohne Herzschlag; ohne Sitzungspuffer 60 Sekunden
nach gemeldeter Pause; **15 Minuten ohne Segmentabruf**, mit Sitzungspuffer nach der Höchstdauer des Zeitversatzes
plus 15 Minuten (RT-11); Handy- und TV-App beenden die Sitzung selbst 30 Sekunden nach dem Wechsel in den
Hintergrund. Die Angabe `playing` entscheidet nichts ausser der Frist «Pause ohne Sitzungspuffer» und der
Drosselung einer Umwandlung; die **Schauzeit** zählt jedes ausgelieferte, verschiedene Segment (RT-04, SM-50).

**Recht je Operation:** `createLiveSession` und `tuneLiveSession` prüfen «Live-TV» und die Sichtbarkeit des Senders;
eine Aufnahme braucht die Zugriffsregel aus `07` 4.3 und für das Hauptprofil eigenes «Live-TV» (`03` DR-03). Der
**Herzschlag** prüft höchstens 30 Sekunden alt Recht, Anmeldung und bei einer Aufnahme ihre Zugriffsregel, weil der
Player die abgeschlossene Liste einer fertigen Aufnahme nur einmal lädt (RT-10); scheitert die Prüfung, antwortet
er 410. Löschen einer Aufnahme beendet Sitzungen auf ihr sofort;
`seekLiveSession` in einer Live-Sitzung braucht «Timeshift». Ohne «Timeshift» liefert `timeshift.enabled: false`
mit `reason: no_right`, und `actions.pause` ist falsch; die Oberfläche blendet Pause und Spulen aus (Regel 12).

**Fehler:** 404 (Recht, Sender, Aufnahme, fremde Sitzung), 409 `stream_limit_reached`, 403 `watch_limit_reached`,
409 `live_conflict` (`tune` einer Aufnahme-Sitzung), 410 `live_session_gone`, 422 (`channelId` und `recordingId`
zugleich oder keines; Spur gehört nicht zur Sitzung), 409 `live_superseded`, 429 `live_too_fast`, 503
`transcode_busy`, 503 `live_capacity` (Deckel schon bei der Annahme voll). Was erst während des Starts scheitert,
kommt als `start_failed` mit `startError` (oben), nicht als Fehlerstatus. Ein deaktivierter Sender ist für das
Einstellen 404; eine laufende Wiedergabe auf ihm läuft weiter (T-29), bis der Zuschauer geht, der Administrator
«sofort beenden» wählt oder das Token abläuft (höchstens 12 Stunden, kein neues Token für einen deaktivierten
Sender); dann 410 beim nächsten Herzschlag. Wird die **Quelle** entfernt oder geändert, endet die Wiedergabe innert
5 Sekunden oder zieht auf eine andere Quelle um (`07` 10.3).

### 3.5 Aufnahmen

```yaml
# Entwurf
LiveRecordingState:
  type: string
  enum: [scheduled, suspended, waiting, recording, recording_gap, completed, incomplete, failed, cancelled]
  description: 07 3.9; «deleting» is never returned (404).
LiveRecordingActions:
  type: object
  description: |
    What the active profile may do with this row; the server decides (rule 12, RD-17). The primary profile on a
    recording of a profile with an age limit gets play (only with its own Live-TV), delete, never edit or stop
    (03 DR-03). A recording with filesMissing cannot be played; a running one cannot be deleted before stop.
  required: [play, resume, delete, stop, extend, edit, retention]
  properties:
    play: {type: boolean}
    resume: {type: boolean, description: A stored own position exists (never for another profile's recording, DR-21).}
    delete: {type: boolean}
    stop: {type: boolean}
    extend: {type: boolean, description: Running instant recording with EPG («Bis Sendungsende verlängern»).}
    edit: {type: boolean, description: Scheduled and pre-roll not yet started.}
    retention: {type: boolean}
LiveRecordingBrief:
  type: object
  required: [id, kind, state, title, channelName, start, end, createdAt, actions]
  properties:
    id: {type: string}
    kind: {type: string, enum: [instant, single, manual, rule]}
    createdAt: {type: integer, format: int64, description: «Einzeln geplant am 20.11.» (LS-09, RD-29).}
    actions: {$ref: "#/components/schemas/LiveRecordingActions"}
    state: {$ref: "#/components/schemas/LiveRecordingState"}
    title: {type: string}
    subtitle: {type: string}
    channelId: {type: integer, format: int64, description: Absent when the channel was deleted.}
    channelName: {type: string}
    start: {type: integer, format: int64, description: Core start (programme), not the pre-roll.}
    end: {type: integer, format: int64}
    preMinutes: {type: integer}
    postMinutes: {type: integer}
    untilStopped: {type: boolean}
    bytes: {type: integer, format: int64}
    expectedBytes: {type: integer, format: int64}
    progress: {type: object, properties: {positionMs: {type: integer, format: int64}, watched: {type: boolean}}}
    retention:
      type: object
      required: [mode]
      properties:
        mode: {type: string, enum: [unlimited, days, keep_last, kept]}
        deleteAt: {type: integer, format: int64}
        days: {type: integer}
        keepLast: {type: integer}
    failReason: {type: string, enum: [quota_full, server_busy, channel_unavailable, source_unreachable, source_encrypted, storage_full, storage_missing, server_not_running]}
    cancelReason: {type: string, enum: [profile, right_revoked, channel_deleted, skipped, merged]}
    warnings: {type: array, items: {$ref: "#/components/schemas/LiveWarningKind"}}
    matchState: {type: string, enum: [exact, moved_auto, ambiguous, missing]}
    adjustedFrom: {type: integer, format: int64, description: Core start before the last EPG adjustment (LS-11).}
    ruleId: {type: string}
    image: {$ref: "#/components/schemas/LiveImageRef"}
    filesMissing: {type: boolean}
LiveRecording:
  allOf:
    - $ref: "#/components/schemas/LiveRecordingBrief"
    - type: object
      properties:
        description: {type: string}
        gaps:
          type: array
          items:
            type: object
            required: [from, reason]
            properties:
              from: {type: integer, format: int64}
              to: {type: integer, format: int64}
              reason: {type: string, enum: [source_interrupted, source_switched, source_unreachable, source_encrypted, storage_full, storage_missing, disk_too_slow, server_not_running, stopped_by_admin, channel_disabled]}
        tracks: {type: object, properties: {audio: {type: array, items: {$ref: "#/components/schemas/LiveTrack"}}, subtitles: {type: array, items: {$ref: "#/components/schemas/LiveTrack"}}}}
        sourceSwitches: {type: array, items: {type: integer, format: int64}, description: Times only, never a source name.}
        candidates: {type: array, items: {$ref: "#/components/schemas/LiveProgrammeBrief"}, description: With matchState ambiguous.}
        ownedByOtherProfile:
          type: object
          description: Only for the primary profile looking at a profile with an age limit (LS-14).
          properties:
            profileId: {type: integer, format: int64}
            profileName: {type: string}
LiveRecordingCreate:
  type: object
  required: [kind]
  properties:
    kind: {type: string, enum: [instant, single, manual]}
    channelId: {type: integer, format: int64, description: instant and manual.}
    programmeRef: {$ref: "#/components/schemas/LiveProgrammeRef"}     # single
    until: {type: string, enum: [programme_end, minutes, until_stopped], description: instant.}
    minutes: {type: integer, enum: [30, 60]}
    start: {type: integer, format: int64, description: manual, UTC ms (the client converts the local form time).}
    end: {type: integer, format: int64}
    name: {type: string, maxLength: 200, description: manual.}
    preMinutes: {type: integer, minimum: 0, maximum: 30, description: Absent = administrator default at creation (11 1).}
    postMinutes: {type: integer, minimum: 0, maximum: 60}
    retentionDays: {type: integer, enum: [7, 14, 30, 90], description: Absent = «bis ich lösche» unless the suggestion is taken explicitly (03 F-10 a).}
LiveRecordingPreview:
  type: object
  required: [start, end, warnings]
  properties:
    start: {type: integer, format: int64, description: Including pre-roll.}
    end: {type: integer, format: int64}
    startsNow: {type: boolean, description: The programme runs already (14 LD-01).}
    expectedBytes: {type: integer, format: int64, description: Absent when the bitrate is unknown («Grösse unbekannt»).}
    warnings: {type: array, items: {$ref: "#/components/schemas/LiveWarning"}}
    defaults: {type: object, properties: {preMinutes: {type: integer}, postMinutes: {type: integer}, retentionDays: {type: integer}}}
LiveWarningKind:
  type: string
  enum: [quota, storage, server_busy, overlap, duplicate, channel_gone, programme_gone, time_changed, dst_gap, no_right]
LiveWarning:
  type: object
  required: [kind, severity]
  properties:
    kind: {$ref: "#/components/schemas/LiveWarningKind"}
    severity: {type: string, enum: [info, warning, danger]}
    recordingIds: {type: array, items: {type: string}}
    freeBytes: {type: integer, format: int64}
    neededBytes: {type: integer, format: int64}
    otherRecordingId: {type: string}
LiveRecordingUsage:
  type: object
  required: [quotaBytes, ownBytes, otherProfilesBytes, freeBytes]
  properties:
    quotaBytes: {type: integer, format: int64}
    ownBytes: {type: integer, format: int64}
    otherProfilesBytes: {type: integer, format: int64, description: Sum only, no names (14 3.8).}
    freeBytes: {type: integer, format: int64}
    scheduledBytes: {type: integer, format: int64, description: Forecast of the own scheduled recordings.}
LiveRecordingUpdate:
  type: object
  properties:
    preMinutes: {type: integer, minimum: 0, maximum: 30}
    postMinutes: {type: integer, minimum: 0, maximum: 60}
    start: {type: integer, format: int64, description: manual and scheduled only.}
    end: {type: integer, format: int64}
    channelId: {type: integer, format: int64, description: manual and scheduled only («Anderen Sender wählen», LS-12, RD-29).}
    rev: {type: integer, description: Optional; the row's rev the client saw. Another rev answers 409 live_conflict (reason changed).}
    retentionDays: {type: integer, enum: [7, 14, 30, 90], nullable: true}
    keep: {type: boolean}
    extendToProgrammeEnd: {type: boolean, description: Running instant recording («Bis Sendungsende verlängern», 14 4.10).}
    acknowledge: {type: array, items: {$ref: "#/components/schemas/LiveWarningKind"}, description: Hide info warnings (LS-12 «OK»).}
    chooseCandidate: {$ref: "#/components/schemas/LiveProgrammeRef"}
```

**`listLiveRecordings` · Parameter:** `view` (`library` Vorgabe: alles ausser `scheduled` und `suspended`;
`upcoming`: `scheduled`, `suspended`, `waiting`, `recording`, `recording_gap`, nach Beginn), `state` (mehrere,
Filter `ToggleToolbar` von LS-08), `kind` (mehrere: LS-11 Abschnitt «Manuell», RD-29), `ruleId` (Serienstapel
`?series=`), `profileId` (nur das Hauptprofil, nur ein Profil mit Altersgrenze, mit der Bedingung aus `07` 4.3;
sonst 404), `q` (Suche in Titel und Untertitel, `03` DR-13), `sort` (`newest` Vorgabe, `oldest`, `channel`,
`title`, `size`, `expiring`), `offset`, `limit` (1 bis 100, Vorgabe 50). Antwort `{items: [LiveRecordingBrief],
total, offset, counts: {all, recording, completed, incomplete, failed}, series: [{ruleId, title, count}]}`. Ohne
`profileId` sieht ein Hauptprofil hier nur die eigenen (`03` DR-02 «nicht vermischt»); mit `profileId` gibt es
keine Termine des Kinds (Vorgabe `20` E-28-43 (A), auch keine Zahl).

**`listProfileLiveRecordings` (LS-14):** nur wenn das aktive Profil das Hauptprofil ist und das Konto Profile mit
Altersgrenze (heute, `07` 4.4) hat; sonst 404. Antwort in der üblichen Form `{items: [{profileId, name, avatar,
count, bytes}], total, offset}` (RD-28; `scheduled` nur bei `20` E-28-43 (B)); die Aufnahmen eines Profils holt der Client mit
`listLiveRecordings?profileId=`. Die Operation verlangt **kein** eigenes Recht «Aufnahmen» des Hauptprofils, damit
es Platz frei machen kann (`03` DR-03); eigenes wirksames «Live-TV» verlangt sie nach der Vorgabe (`20` E-28-42
(b)): ohne Live-TV ist alles 404 (SM-38 ohne Ausnahme). **Nur bei Variante (a)** (benannte Ausnahme **A-12**,
`02`, `12` 1.1) antworten `listProfileLiveRecordings`, `listLiveRecordings?profileId=`, `getLiveRecording` und
`deleteLiveRecording` auf Aufnahmen eines Profils mit Altersgrenze auch ohne eigenes «Live-TV» wie mit Recht;
**Abspielen** (`createLiveSession` mit `recordingId`) bliebe 404.

**`createLiveRecording`:** 201 mit `LiveRecording` und `warnings`; dasselbe noch einmal (gleiche Sendung, gleiche
manuelle Zeit, laufende Sofortaufnahme auf demselben Sender) ergibt 200 mit der bestehenden Zeile (RD-10).
Geplante Aufnahmen werden trotz Warnung gespeichert (E-4: der Planer warnt, abgelehnt wird beim Start). Eine
**Sofortaufnahme** beginnt sofort; bei
vollem Kontingent 507 `live_quota_full` (die einzige Ablehnung beim Anlegen, weil Planen und Start zusammenfallen).
`until: programme_end` ohne Programmdaten: 422 `live_no_epg`. Die Sofortaufnahme bei zeitversetzter Wiedergabe
beginnt an der Live-Kante (`10` F-1 (a)).

**`deleteLiveRecording`:** laufende Aufnahme: 409 `live_conflict` mit `reason: running` (erst `stopLiveRecording`,
LD-06 «Stoppen und löschen» ist im Client zwei Aufrufe); geplanter Einzeltermin: Zeile weg; Regeltermin mit
`skip=true`: `cancelled/skipped` (`07` W-17); Hauptprofil auf Aufnahme eines Profils mit Altersgrenze: erlaubt
(`03` DR-03), Meldung an das Profil nach `14` F-4. Antwort 204; Dateien entfernt der Job (`07` 10.5).
`deleteLiveRecordings`: `{ids: [...]}` bis 200; Antwort `{deleted, skipped: [{id, reason}]}` (laufende werden
übersprungen).

**Fehler der Gruppe:** 404, 409 `live_conflict`, 422 (`validation_failed`, `live_programme_past`, `live_no_epg`),
503 `live_storage_unavailable` (Sofortaufnahme, Speicherort fehlt), 507 `live_quota_full`, `live_storage_full`.

### 3.6 Regeln und Erinnerungen

```yaml
# Entwurf
LiveRuleCreate:
  type: object
  required: [kind]
  properties:
    kind: {type: string, enum: [series, recurring]}
    programmeRef: {$ref: "#/components/schemas/LiveProgrammeRef"}   # series: the chosen programme (09 7.3)
    allChannels: {type: boolean, default: false}
    matchOnscreen: {type: boolean, default: false}
    matchSubtitle: {type: boolean, default: false}
    channelId: {type: integer, format: int64}                       # recurring
    weekdays: {type: array, items: {type: integer, minimum: 1, maximum: 7}, minItems: 1}
    startLocal: {type: string, pattern: "^([01][0-9]|2[0-3]):[0-5][0-9]$"}
    durationMinutes: {type: integer, minimum: 1, maximum: 720}
    name: {type: string, maxLength: 200}
    preMinutes: {type: integer, minimum: 0, maximum: 30}
    postMinutes: {type: integer, minimum: 0, maximum: 60}
    retentionDays: {type: integer, enum: [7, 14, 30, 90]}
    keepLast: {type: integer, minimum: 1, maximum: 50}
LiveRule:
  type: object
  required: [id, kind, name, state, counts, actions]
  properties:
    actions:
      type: object
      required: [edit, delete, clearEpisodes]
      properties:
        edit: {type: boolean}
        delete: {type: boolean}
        clearEpisodes: {type: boolean, description: Only series with a reliable episode mark (RD-17).}
    id: {type: string}
    kind: {type: string, enum: [series, recurring]}
    name: {type: string}
    state: {type: string, enum: [active, suspended, channel_gone]}
    episodesDetectable: {type: boolean, description: False: every matching airing is recorded (03 F-8 a).}
    counts: {type: object, properties: {scheduled: {type: integer}, recordings: {type: integer}}}
    next: {$ref: "#/components/schemas/LiveRecordingBrief"}
    upcoming: {type: array, items: {$ref: "#/components/schemas/LiveRecordingBrief"}}
    skipped:
      type: array
      items:
        type: object
        properties:
          programme: {$ref: "#/components/schemas/LiveProgrammeBrief"}
          reason: {type: string, enum: [already_recorded, already_scheduled, other_channel, rule_limit]}
LiveReminder:
  type: object
  required: [id, leadMinutes, remindAt, state, programme]
  properties:
    id: {type: string}
    leadMinutes: {type: integer, enum: [0, 5, 15, 30]}   # 03 DR-08 (W-2)
    remindAt: {type: integer, format: int64}
    state: {type: string, enum: [planned, sent, missed, cancelled, dropped]}
    matchState: {type: string, enum: [exact, moved_auto, ambiguous, missing]}
    programme: {$ref: "#/components/schemas/LiveProgrammeBrief"}
    channel: {$ref: "#/components/schemas/LiveChannel"}
```

- Die Zeitzone einer wiederkehrenden Regel setzt der Server (Zone des Servers beim Anlegen, `11` 3.5); kein Feld in
  der Anfrage.
- Serie aus einer Sendung ohne Programmdaten: 422 `live_no_epg`; aus einer Sendung, deren Sender keine Folgen
  kennzeichnet: 201 mit `episodesDetectable: false` (Hinweis aus `09` 7.2).
- Felder aus `14` LD-02 und LD-03, die `11` nicht kennt (Zeitfenster und Wochentage einer Serie, «nur
  Erstausstrahlungen», Schalter «Folgen nicht doppelt», Ende einer Wiederholung), gibt es nicht; sie sind eine
  Frage an dich über `11` O-6 (`07` O-3).
- `createLiveRecordingRule`: dieselbe Serie (gleicher normalisierter Titel und Sender) oder dieselbe wiederkehrende
  Zeit noch einmal: 200 mit der bestehenden Regel (RD-10).
- `createLiveReminder`: `{programmeRef, leadMinutes}`; vergangene Sendung 422 `live_programme_past`; Sender ohne
  EPG 422 `live_no_epg`; mehr als 200 offene 422 mit Feld `programmeRef`, Code `limit`; gleiche Sendung zweimal:
  200 mit der bestehenden **offenen** (Eindeutigkeit nur für offene, `07` 3.6). `deleteLiveReminder` löscht die
  Zeile; wer dieselbe Erinnerung neu setzt, bekommt eine neue (RD-20).
- `listLiveReminders`: `offset`, `limit` (1 bis 200); vergangene erscheinen nicht (LS-13).

### 3.7 Verwaltung: Quellen und Importe

```yaml
# Entwurf
LiveSourceKind: {type: string, enum: [m3u_url, m3u_file, xtream, single]}
LiveSource:
  type: object
  required: [id, kind, name, enabled, importState, accessState, hasCredentials, headerNames, origins]
  properties:
    id: {type: integer, format: int64}
    kind: {$ref: "#/components/schemas/LiveSourceKind"}
    name: {type: string}
    enabled: {type: boolean}
    site: {type: string, description: "Scheme, host and port only (T-28)."}
    hasCredentials: {type: boolean}
    headerNames: {type: array, items: {type: string}, description: Names only; values are never returned (W-5).}
    hasCustomUserAgent: {type: boolean, description: The value is encrypted and never returned (RD-19).}
    origins: {type: array, items: {type: string}}
    localTargets: {type: array, items: {$ref: "#/components/schemas/LiveNetTarget"}}   # stage H
    # no preload or connection setting per source (04 14.2, F-3; LTV-017): a switch would only come with F-3 (b)
    listenInterface: {type: string}                                                      # stage H
    refresh: {type: object, properties: {everyMinutes: {type: integer}, atMinute: {type: integer}, next: {type: integer, format: int64}}}
    newChannelsEnabled: {type: boolean}
    takeGroups: {type: boolean}
    takeNumbers: {type: boolean}
    guardIncomplete: {type: boolean}
    importState: {type: string, enum: [never, running, ok, unchanged, failed, held]}
    importError: {type: string, description: Stable code, translated by the client.}
    lastImportAt: {type: integer, format: int64}
    progress: {type: object, properties: {done: {type: integer}, total: {type: integer}}, description: While running (job progress).}
    accessState: {type: string, enum: [ok, check]}
    accessRetryAt: {type: integer, format: int64, description: «Zugang prüfen» is a quarantine; next attempt (04 5.3).}
    hostMismatch: {type: boolean}
    running:
      type: object
      description: |
        Numbers for the confirmation before removing or changing the source (rules, address, credentials,
        headers, origins, targets): all receivers of the source end within 5 s (04 5.3, 07 10.3, SM-47).
      properties: {viewers: {type: integer}, recordings: {type: integer}}
    runtime: {$ref: "#/components/schemas/LiveSourceRuntime"}
    counts:
      type: object
      description: orphans = channels without another source (numbers for the removal question of LA-02, S-6).
      properties: {channels: {type: integer}, active: {type: integer}, missing: {type: integer}, groups: {type: integer}, orphans: {type: integer}, orphanFavorites: {type: integer}, orphanScheduled: {type: integer}}
LiveSourceRuntime:
  type: object
  readOnly: true
  description: |
    Runtime state of the source from memory, not a setting and not stored (07 1.3, 04 5.3, 14.1); empty after a
    restart. Shown in the diagnosis of the source (LA-13) and in LA-02.
  properties:
    precautionResting:
      type: object
      description: «Vorsorglicher Empfang ruht»: no preload, no automatic, no checks beside a running stream.
      properties:
        since: {type: integer, format: int64}
        until: {type: integer, format: int64, description: 30 min, on repetition 6 h, then 24 h (04 14.1).}
        pattern: {type: string, enum: [rejection, displacement], description: A = rejection, B = displacement.}
    singleStream:
      type: object
      description: «Ein-Strom-Betrieb» after pattern B; the server opens no second connection to this source.
      properties: {since: {type: integer, format: int64}, until: {type: integer, format: int64}}
LiveSourceWrite:
  type: object
  description: |
    Create and update. Secret fields are write-only: absent = unchanged, empty string = remove. A URL with
    user:password@ is split into the credential fields and stored without them (04 10.5).
  properties:
    kind: {$ref: "#/components/schemas/LiveSourceKind"}
    name: {type: string, minLength: 1, maxLength: 200}
    url: {type: string, maxLength: 4096, writeOnly: true}
    username: {type: string, maxLength: 256, writeOnly: true}
    password: {type: string, maxLength: 256, writeOnly: true}
    headers: {type: array, maxItems: 16, items: {type: object, required: [name], properties: {name: {type: string}, value: {type: string, writeOnly: true}}}}
    userAgent: {type: string, maxLength: 256, writeOnly: true}
    uploadId: {type: string, description: m3u_file, from uploadLiveFile (200 MiB, 04 14.9).}
    previewImportId: {type: integer, format: int64, description: Apply this preview run (outcome preview) instead of fetching again (S-3).}
    excludeEntries: {type: array, items: {type: integer}, description: Preview rows «übernehmen» unchecked (LA-04).}
    single: {type: object, description: single, step 4 of LA-03, properties: {newChannel: {type: object, properties: {name: {type: string}, number: {type: integer}, groupId: {type: integer}}}, addToChannelId: {type: integer, format: int64}}}
    origins: {type: array, items: {type: string}}
    localTargets: {type: array, items: {$ref: "#/components/schemas/LiveNetTargetWrite"}}
    listenInterface: {type: string}
    refreshEveryMinutes: {type: integer, minimum: 60, maximum: 10080, nullable: true}
    refreshAtMinute: {type: integer, minimum: 0, maximum: 1439, nullable: true}
    newChannelsEnabled: {type: boolean}
    takeGroups: {type: boolean}
    takeNumbers: {type: boolean}
    guardIncomplete: {type: boolean}
    enabled: {type: boolean}
LiveNetTarget:
  type: object
  required: [id, address, prefixLength, portFrom, portTo]
  properties:
    id: {type: integer, format: int64}
    address: {type: string, description: Normalised network address.}
    prefixLength: {type: integer, description: "32 or 128 = one address; at least 28 (IPv4) or 124 (IPv6)."}
    portFrom: {type: integer}
    portTo: {type: integer, description: Equal to portFrom by default; at most 16 ports.}
    wide: {type: boolean, description: Network or port range, given with a second confirmation (RT-05).}
    hostname: {type: string}
LiveNetTargetWrite:
  type: object
  description: |
    Default: one address and one port (04 10.9, 12 4.5). A network (at most /28 or /124) or a port range (at
    most 16) needs wideConfirm with the counts shown; never with the default gateway of the home or container
    network. At most 16 entries per source. Any change raises the rule version of the source and ends its
    receivers (07 10.3).
  required: [address, portFrom, confirmToken]
  properties:
    address: {type: string}
    prefixLength: {type: integer, minimum: 28, maximum: 128, description: Absent = single address.}
    portFrom: {type: integer, minimum: 1, maximum: 65535}
    portTo: {type: integer, minimum: 1, maximum: 65535, description: Absent = portFrom.}
    confirmToken: {type: string, description: From resolveLiveNetTarget; binds the confirmed resolution (T-5).}
    wideConfirm: {type: string, description: "Second confirmation for a network or range: «<addresses>,<ports>» as shown."}
LiveSourceTestRequest:
  description: A new source (fields of LiveSourceWrite) or an existing one (sourceId; fields given override).
  allOf:
    - $ref: "#/components/schemas/LiveSourceWrite"
    - type: object
      properties:
        sourceId: {type: integer, format: int64}
LiveSourceTest:
  type: object
  required: [result]
  properties:
    result: {type: string, enum: [playlist, single_stream, xtream_ok, error]}
    error: {type: string, enum: [auth_rejected, unreachable, timeout, not_a_playlist, too_large, drm, local_not_allowed, hls_manifest_not_list, unsupported_scheme]}
    errorLine: {type: integer}
    playlist: {type: object, properties: {entries: {type: integer}, groups: {type: integer}, withLogo: {type: integer}, withEpgId: {type: integer}, epgUrlFound: {type: boolean}, epgSite: {type: string}}}
    xtream: {type: object, properties: {liveChannels: {type: integer}, categories: {type: integer}}}
    stream: {type: object, properties: {video: {type: string}, width: {type: integer}, height: {type: integer}, fps: {type: number}, interlaced: {type: boolean}, audio: {type: array, items: {type: string}}, subtitles: {type: array, items: {type: string}}, bitrateKbps: {type: integer}, firstFrameMs: {type: integer}}}
    previewImportId: {type: integer, format: int64, description: A run with outcome preview, kept 1 hour (07 3.2); read with getLiveSourceImport.}
LiveImport:
  type: object
  required: [id, outcome, startedAt, counts]
  properties:
    id: {type: integer, format: int64}
    trigger: {type: string, enum: [schedule, manual, create, upload, apply_held, test]}
    outcome: {type: string, enum: [running, ok, unchanged, failed, held, preview]}
    startedAt: {type: integer, format: int64}
    finishedAt: {type: integer, format: int64}
    counts: {type: object, properties: {new: {type: integer}, changed: {type: integer}, missing: {type: integer}, duplicates: {type: integer}, faulty: {type: integer}, unchanged: {type: integer}, blockedLocal: {type: integer}, drm: {type: integer}}}
    error: {type: string}
    errorLine: {type: integer}
    heldReason: {type: object, properties: {before: {type: integer}, after: {type: integer}}}
LiveImportItem:
  type: object
  required: [seq, kind]
  properties:
    seq: {type: integer}
    kind: {type: string, enum: [new, changed, missing, duplicate, faulty]}
    channelId: {type: integer, format: int64}
    entryName: {type: string}
    line: {type: integer}
    changes: {type: array, items: {type: string, enum: [name_changed, url_changed, logo_changed, group_changed, number_changed, identity_conflict]}}
    excluded: {type: boolean, description: Preview only.}
    kept: {type: array, items: {type: string, enum: [own_name, own_logo, own_number, own_groups]}}
    favorites: {type: integer}
    scheduled: {type: integer}
    duplicateOf: {type: integer, format: int64}
    reason: {type: string}
```

- **`testLiveSource`** (eine Operation für neue und bestehende Quellen, S-2) ruft einmal ab (Netzschutz T-5,
  Grenzen `04` 14.9) und ändert keinen Sender; eine Senderliste ergibt einen **Vorschau-Lauf** (`outcome =
  'preview'`, `previewImportId`, Einträge in `listLiveSourceImportItems`, S-3). `createLiveSource` mit
  `previewImportId` und dem Formular wendet genau diesen Stand an; `updateLiveSource` ebenso. Test einer lokalen
  Adresse ohne passendes festes Ziel: `result: error`, `error: local_not_allowed` (kein Abruf).
- **`uploadLiveFile`** (`PUT`, `kind` = `m3u` oder `xmltv`): Körper roh, höchstens 200 MiB für M3U (`04` 14.9; W-22)
  und 1 GiB für XMLTV (`09` 12), eigene Grenze in `largeBodies` (gebaut: `server/internal/api/limits.go:33-44`).
  Antwort `{uploadId, bytes, sha256}`; die Datei bleibt 1 Stunde, bis eine Quelle sie übernimmt (`07` 9).
- **`deleteLiveSource`:** `orphans=keep` (Vorgabe) oder `delete` (`07` 10.3); Antwort 204. 409 `live_import_running`
  während eines Imports. Die Zahlen für die Rückfrage stehen in `LiveSource.counts` (S-6) und `LiveSource.running`.
- **Entfernen oder Ändern beendet die Empfänger (RB-05, RT-06, SM-47):** `deleteLiveSource` und jedes
  `updateLiveSource`, das Adresse, Zugangsdaten, Kopfzeilen, Ursprünge oder feste Ziele ändert, verlangt
  `?confirm=<viewers>,<recordings>` aus `LiveSource.running`; fehlt es oder stimmt die Zahl nicht mehr, 409
  `live_conflict` (`reason: confirm_running`, mit den aktuellen Zahlen). Nach dem Speichern enden alle Empfänger
  der Quelle innert 5 Sekunden; Zuschauer ohne Ersatzquelle bekommen 410 beim nächsten Herzschlag, Aufnahmen eine
  Lücke `source_removed` (`07` 10.3). Name, Aktualisierung und Schalter für neue Sender ändern keinen Regelstand
  und brauchen keine Bestätigung. T-29 gilt hier nicht (`12` 4.2).
- **`refreshLiveSource`:** 202 `{jobId}` (Job-Fortschritt in `LiveSource.progress`); 409 `live_import_running`.
- **`applyHeldLiveSourceImport`:** nur für `outcome: held`, sonst 409 `live_conflict`.
- **`listLiveSources`:** ohne Seiten (wenige Quellen); `listLiveSourceImports`: die letzten 10.
  `listLiveSourceImportItems` (Lauf oder Vorschau): `kind`, `offset`, `limit` (1 bis 500); Einträge gibt es nur
  für den letzten Lauf einer Quelle (`07` 3.2, S-9), ältere Läufe nennen nur Zähler.
- **`resolveLiveNetTarget`:** `{host, port}` → `{addresses: [..], confirmToken, refused: [{address, reason:
  loopback|link_local|own_address|public}]}`; der Token bindet die gezeigte Auflösung, damit ein späterer Wechsel
  eine neue Rückfrage braucht (`04` 10.9).

**Fehler der Gruppe:** 404 (unbekannt), 409 (`live_import_running`, `live_conflict`), 413 `body_too_large`
(gebaut als Muster der Upload-Grenze), 422 (`validation_failed`: Schema nicht erlaubt, Ursprung ausserhalb von
`http(s)`, verbotene Kopfzeile, Netz grösser als /28 bzw. /124 oder mehr als 16 Ports, Netz mit Gateway, mehr als
16 Ziele, fehlender `confirmToken` oder `wideConfirm`), 429 `live_too_fast` (Test).

### 3.8 Verwaltung: Sender, Gruppen, Duplikate

```yaml
# Entwurf
AdminLiveChannelRow:
  type: object
  description: |
    One row of listAdminLiveChannels: only the columns of LA-05 (RD-28). Streams, technical data, availability
    and usage come from getAdminLiveChannel.
  required: [id, name, enabled, status, streamCount]
  properties:
    id: {type: integer, format: int64}
    number: {type: integer}
    name: {type: string}
    sourceName: {type: string}
    logo: {$ref: "#/components/schemas/LiveImageRef"}
    groupNames: {type: array, items: {type: string}}
    country: {type: string}
    language: {type: string}
    categories: {type: array, items: {$ref: "#/components/schemas/LiveCategory"}}
    streamCount: {type: integer}
    activeSourceName: {type: string}
    epg: {type: string, enum: [mapped, suggested, missing, none]}
    strategy: {type: string, enum: [default, on_demand, permanent, automatic, scheduled]}
    enabled: {type: boolean}
    status: {type: string, enum: [available, disturbed, missing, no_source, inactive, new]}
    statusSince: {type: integer, format: int64}
AdminLiveChannel:
  allOf:
    - $ref: "#/components/schemas/LiveChannel"
    - type: object
      required: [enabled, status, locks, streams]
      properties:
        running:
          type: object
          description: What «sofort beenden» would end (T-29); numbers only.
          properties: {viewers: {type: integer}, recordings: {type: integer}}
        enabled: {type: boolean}
        disabledAt: {type: integer, format: int64, description: «deaktiviert seit»; playback on it ends at the latest 12 h later (T-29).}
        playability:
          type: array
          items: {type: string, enum: [transcode_only, long_gop]}
          description: «nur mit Umwandlung abspielbar» (segments over 10 s without session buffer), «lange Bildgruppen» (04 10.8, 05 3.6).
        status: {type: string, enum: [available, disturbed, missing, no_source, inactive, new]}
        missingSince: {type: integer, format: int64}
        disturbedSince: {type: integer, format: int64}
        sourceName: {type: string, description: Name of the rank-1 stream in its source («Quelle: SRF1 HD CH»).}
        locks: {type: array, items: {type: string, enum: [name, logo, number, country, language, categories, groups, streams]}}
        strategy: {type: string, enum: [default, on_demand, permanent, automatic, scheduled]}
        scheduleId: {type: integer, format: int64}
        epg: {type: object, properties: {mode: {type: string, enum: [auto, manual, none, missing, suggested, orphaned]}, sourceName: {type: string}, xmltvId: {type: string}}}
        streams: {type: array, items: {$ref: "#/components/schemas/AdminLiveStream"}}
        technical: {type: object, description: Last stored stream parameters (LA-06 «Technik»).}
        availability: {type: object, properties: {outages7d: {type: integer}, reconnects7d: {type: integer}, switches7d: {type: integer}}}
        usage:
          type: object
          description: Numbers only, no profiles (LA-06 9); also the numbers of the delete question (S-5).
          properties: {favorites: {type: integer}, reminders: {type: integer}, scheduled: {type: integer}, rules: {type: integer}}
AdminLiveStream:
  type: object
  required: [id, rank, sourceId, sourceName, site, state]
  properties:
    id: {type: integer, format: int64}
    rank: {type: integer}
    sourceId: {type: integer, format: int64}
    sourceName: {type: string}
    site: {type: string}
    scheme: {type: string}
    format: {type: string, enum: [ts, hls, dash]}
    state: {type: string, enum: [unknown, ok, failing, drm, encrypted, local_not_allowed, unsupported]}
    stateSince: {type: integer, format: int64}
    lastError: {type: string}
    missingSince: {type: integer, format: int64}
LiveChannelUpdate:
  type: object
  properties:
    name: {type: string, minLength: 1, maxLength: 200}
    resetName: {type: boolean, description: Back to the name of the source (removes the lock).}
    number: {type: integer, minimum: 1, maximum: 9999, nullable: true}
    country: {type: string, nullable: true}
    language: {type: string, nullable: true}
    categories: {type: array, items: {$ref: "#/components/schemas/LiveCategory"}}
    groupIds: {type: array, items: {type: integer, format: int64}}
    enabled: {type: boolean}
    strategy: {type: string, enum: [default, on_demand, permanent, automatic, scheduled]}
    scheduleId: {type: integer, format: int64}
    noAudio: {type: boolean}
    noVideo: {type: boolean}
    logo: {type: object, properties: {mode: {type: string, enum: [source, url, upload]}, url: {type: string, writeOnly: true}}}
LiveBulkRequest:
  type: object
  required: [action]
  properties:
    ids: {type: array, items: {type: integer, format: int64}, maxItems: 10000}
    filter: {type: object, description: The filter of listAdminLiveChannels («Alle 412 Treffer auswählen»).}
    action: {type: string, enum: [enable, disable, add_group, remove_group, set_country, set_language, add_category, remove_category, set_strategy, assign_numbers]}
    groupId: {type: integer, format: int64}
    value: {type: string}
    sort: {type: string, enum: [number, name, status], description: Order for assign_numbers («in der aktuellen Sortierung», RD-29).}
    numbers: {type: object, properties: {start: {type: integer, minimum: 1}, step: {type: integer, minimum: 1, maximum: 100}, overwrite: {type: boolean}}}
LiveBulkResult:
  type: object
  required: [changed]
  properties:
    changed: {type: integer}
    errors: {type: array, items: {type: object, properties: {channelId: {type: integer, format: int64}, code: {type: string}, otherChannelId: {type: integer, format: int64}}}}
LiveMergeRequest:
  type: object
  required: [keepId, mergeIds]
  properties:
    keepId: {type: integer, format: int64}
    mergeIds: {type: array, items: {type: integer, format: int64}, minItems: 1, maxItems: 20}
    take: {type: object, properties: {name: {type: integer}, logo: {type: integer}, number: {type: integer}, epg: {type: integer}, countryLanguageCategories: {type: integer}}, description: Channel id per field.}
    unionGroups: {type: boolean, default: true, description: «Gruppen (Vereinigung als Vorgabe)», may be unchecked (15 LA-07, RD-29).}
    streamOrder: {type: array, items: {type: integer, format: int64}}
```

- **`listAdminLiveChannels` · Parameter:** `q` (Name, Nummer, Name in der Quelle), `status` (mehrere: `available`,
  `disturbed`, `missing`, `no_source`, `inactive`, `new`), `source`, `group`, `country`, `language`, `category`,
  `epg` (`mapped`, `suggested`, `missing`, `none`), `strategy` (`default`, `custom` für LA-09 «Ausnahmen», oder ein
  Wert), `sort` (`number` Vorgabe, `name`, `status`), `offset`, `limit` (1 bis 500, Vorgabe 100). Antwort
  `{items: [AdminLiveChannelRow], total, offset, counts: {active, total, byStatus}}`; eine Zeile kostet keine
  Teilabfrage auf Ereignisse oder Favoriten (RD-28). 10'000 Zeilen über die virtuelle Tabelle von LA-05.
- **Deaktivieren (`updateLiveChannel` mit `enabled: false`, T-29):** ist kein Sicherheitsmittel: Laufende
  Wiedergaben laufen bis zu 12 Stunden weiter (kein neues Token), Aufnahmen bis zu ihrem geplanten Ende. Die
  Antwort nennt `running`, damit die Verwaltung den Hinweis und «sofort beenden» anbietet (`15`).
- **`endLiveChannelNow` (T-29):** nur für einen deaktivierten oder verschwundenen Sender, sonst 409 `live_conflict`
  (`reason: active`). Verlangt `?confirm=<viewers>,<recordings>` aus `AdminLiveChannel.running`; stimmt die Zahl
  nicht mehr, 409 (`reason: changed`). Wiedergaben enden (`ended_by = 'channel'`, Zuschauer «Sender nicht
  verfügbar»), laufende Aufnahmen enden als `incomplete` mit Lücke `channel_disabled` (`07` 10.4). Antwort
  `{viewersEnded, recordingsEnded}`.
- **Logo «eigene Adresse»:** Eine Vorschau vor dem Speichern gibt es nicht, weil nur der Server den fremden Host
  abrufen darf (4.5); nach dem Speichern zeigt `logo_state` und das Bild über `getLiveChannelLogo` das Ergebnis
  (RD-29; `15` LA-06 nachziehen).
- **`bulkUpdateLiveChannels`:** in Stapeln zu 500 (`07` 8.1); doppelte Nummern: Zeile im Ergebnis mit `code:
  number_taken` und `otherChannelId`, die übrigen Änderungen gelten (Teilerfolg, 200). Ein Filter statt `ids`
  wirkt auf den Stand beim Absenden.
- **`deleteLiveChannel`:** nur ohne Quelle oder verschwunden, sonst 409 `live_conflict` (`reason: has_source`);
  Wirkung nach `07` 10.4. Die Zahlen der Rückfrage stehen in `AdminLiveChannel.usage` (S-5).
- **`mergeLiveChannels`:** Wirkung nach `07` 7.4 (Doppel von Erinnerungen und Terminen fallen weg, Serienregeln
  werden vereinigt); `previewLiveChannelMerge` nennt `{favoritesMoved, scheduledMoved, duplicatesDropped}`. 409
  `live_conflict`, wenn ein aufzugebender Sender gerade aufnimmt (`reason: recording`; Zusammenführen geht danach).
- **`setLiveChannelStreams`:** `{streamIds}` als ganze Rangfolge, sonst 422; setzt die Sperre `streams`.
- **`testLiveChannelStream`:** verbindet einmal, liest einige Sekunden, höchstens 15 Sekunden je Aufruf; Antwort
  `{reachable, responseMs, firstFrameMs, video, audio, subtitles, bitrateKbps, frozen, noAudio, message}`
  (LA-13 «Jetzt prüfen»: der Client prüft die Senderquellen nacheinander). Kein Text mit Adresse; «Technische
  Angaben kopieren» setzt der Client aus dieser Antwort zusammen.
- **Gruppen:** `updateLiveGroup` mit `{name, visible, channelIds}` (ganze Menge, setzt bei den betroffenen Sendern
  die Sperre `groups`); `deleteLiveGroup` einer Gruppe aus einer liefernden Quelle: 409 `live_conflict`
  (`reason: from_source`).

### 3.9 Verwaltung: Einstellungen, Empfang, EPG, Speicher, Ströme

```yaml
# Entwurf
LiveConfig:
  type: object
  description: |
    One object with a section per page; updateLiveConfig takes any subset (PATCH). Values outside the
    ranges of 07 3.1 answer 422. Sections of a later stage are absent before it. Not settings (constants of
    04 14): ring length (T-7), parallel checks; no maximum of concurrent recordings (would come with 20 E-28-49 (B)).
  properties:
    reception:
      type: object
      properties:
        defaultStrategy: {type: string, enum: [on_demand, permanent, automatic, scheduled]}
        defaultScheduleId: {type: integer, format: int64}
        preloadPerDirection: {type: integer, minimum: 0, maximum: 2}
        preloadMax: {type: integer, minimum: 0, maximum: 16}
        automaticKeep: {type: integer, minimum: 0, maximum: 10}
        automaticFavorites: {type: boolean}
        graceSeconds: {type: integer, minimum: 15, maximum: 300, description: Only after a viewer leaves (04 14.1).}
        maxIngests: {type: integer, minimum: 1, maximum: 200}
        ringBudgetMb: {type: integer, minimum: 64, maximum: 8192, nullable: true}
        maxSessions: {type: integer, minimum: 1, maximum: 200}
        transcodeSharePct: {type: integer, minimum: 0, maximum: 100}
        startbufferMode: {type: string, enum: [memory, folder], description: Stage 4 and only with 04 F-2 (a).}
    monitoring:
      type: object
      properties:
        idleCheckHours: {type: integer, enum: [6, 24], nullable: true}
        alertAfterMinutes: {type: integer, enum: [5, 10, 15, 30, 60]}
        alertUsedOnly: {type: boolean}
    epg: {type: object, properties: {days: {type: integer, minimum: 1, maximum: 14}, lookbackHours: {type: integer, minimum: 6, maximum: 72}, images: {type: boolean}}}
    timeshift: {type: object, properties: {maxMinutes: {type: integer, enum: [15, 30, 60, 120, 240]}, capBytes: {type: integer, format: int64, nullable: true}, effectiveCapBytes: {type: integer, format: int64, readOnly: true}}}
    recordings:
      type: object
      properties:
        preMinutes: {type: integer, minimum: 0, maximum: 30}
        postMinutes: {type: integer, minimum: 0, maximum: 60}
        retentionSuggestionDays: {type: integer, enum: [7, 14, 30, 90], nullable: true}
        instantMaxHours: {type: integer, minimum: 1, maximum: 12}
        followEpg: {type: boolean}
        warmupSeconds: {type: integer, minimum: 10, maximum: 120}
        defaultQuotaBytes: {type: integer, format: int64, minimum: 0}
        serverTimeZone: {type: string, readOnly: true}
LiveIngest:
  type: object
  required: [channelId, state, reasons, viewers, recordings, since]
  properties:
    channelId: {type: integer, format: int64}
    channelName: {type: string}
    number: {type: integer}
    source: {type: object, properties: {sourceName: {type: string}, rank: {type: integer}, site: {type: string}}}
    state: {type: string, enum: [connecting, running, reconnecting, disturbed, grace, deferred], description: deferred = «zurückgestellt», only a permanent or scheduled channel that cannot start (04 5.3, RG-02).}
    deferred:
      type: object
      description: With state deferred.
      properties:
        cause: {type: string, enum: [capacity, source_rejected, memory]}
        since: {type: integer, format: int64}
        attempts: {type: integer}
        nextAttempt: {type: integer, format: int64}
    attempt: {type: object, properties: {n: {type: integer}, of: {type: integer}}}
    reasons: {type: array, items: {type: string, enum: [viewer, recording, permanent, scheduled, automatic, preload, check]}}
    scheduleName: {type: string}
    since: {type: integer, format: int64}
    viewers: {type: integer}
    recordings: {type: integer, description: Present only from 2 on (SM-45); with one recording only recordingsActive.}
    recordingsActive: {type: boolean}
    bitrateKbps: {type: integer}
    video: {type: object}
    audio: {type: array, items: {type: object}}
    transcodes: {type: object, properties: {direct: {type: integer}, transcoded: {type: integer}, hardware: {type: string}}}
    memory: {type: object, properties: {ringBytes: {type: integer, format: int64}, timeshiftBytes: {type: integer, format: int64}, timeshiftBuffers: {type: integer}, recordingBytes: {type: integer, format: int64}}}
    errors: {type: string}
    viewersDetail:
      type: array
      description: |
        Viewers only, with account, profile, device, mode and live/shifted (W-6 rule, 03 DR-17). Recordings are
        never listed here, not even with their account: account plus channel at this time plus the public EPG
        would reveal the title (RD-06, Leitplanken 4). They are the number in `recordings`.
      items: {type: object}
LiveStorage:
  type: object
  properties:
    locations: {type: array, items: {type: object, properties: {role: {type: string, enum: [startbuffer, timeshift, recordings]}, status: {type: string, enum: [active, read_only]}, path: {type: string}, fsType: {type: string}, health: {type: string, enum: [ready, low, full, missing, unwritable, memory]}, freeBytes: {type: integer, format: int64}, usedBytes: {type: integer, format: int64}, reserveBytes: {type: integer, format: int64}, capBytes: {type: integer, format: int64}}}}
    accounts: {type: array, items: {type: object, properties: {accountId: {type: integer, format: int64}, email: {type: string}, quotaBytes: {type: integer, format: int64}, usedBytes: {type: integer, format: int64}, recordings: {type: integer}, forecast7dBytes: {type: integer, format: int64}, state: {type: string, enum: [ok, high, full]}}}, description: No titles, no profiles, no channels (07 5.1).}
    orphans: {type: object, properties: {timeshiftBytes: {type: integer, format: int64}, recordingFiles: {type: integer}, recordingBytes: {type: integer, format: int64}, lastCleanup: {type: integer, format: int64}}}
```

- **Empfang:** `deleteLiveSchedule` setzt die Sender auf «Standard» und antwortet `{channelsReset}`; ist der
  Zeitplan die Vorgabe, 409 `live_conflict` (`reason: default_schedule`). Zeitfenster als
  `{weekdays: [1..7], from: "HH:MM", to: "HH:MM"}`, `to` vor `from` heisst Folgetag.
- **EPG:** `listLiveEpgMappings` mit `mode` (`auto`, `suggested`, `manual`, `missing`, `none`, `orphaned`), `q`,
  `offset`, `limit` (1 bis 500) → `{items: [{channel, mode, reason, source, xmltvId, preview: [2 Sendungen],
  suggestions: [{epgChannelId, sourceName, xmltvId, displayName, rank}]}], total, offset, counts}`.
  `setLiveEpgMapping`: `{mode: manual, epgSourceId, xmltvId}` oder `{mode: none}`. `acceptLiveEpgSuggestions`:
  `{channelIds}` oder `{allRank: 90}` → `{accepted}`. `runLiveEpgAutoMapping` → `{mapped, remaining}` (nur
  Sender ohne Zuordnung, `15` LA-11). `searchLiveEpgChannels`: `q` (mindestens 2 Zeichen), `limit` bis 50.
  `testLiveEpgSource` liest höchstens die ersten 5 MiB und meldet Format, Kompression, Kanäle, Zeitraum und
  erkannte Zeitzone (`09` 2.4 Vorschau «laut dieser Quelle läuft jetzt …»). EPG-Quellen haben keine eigenen
  Kopfzeilen (W-27).
- **Speicher:** `validateLiveStorageLocation` `{path}` → `{checks: [{name, ok, detail}]}` (vorhanden und
  beschreibbar, Platz gegen Reserve, Art, eingehängt, nicht Medienordner, nicht derselbe Ort; `15` LA-16);
  `setLiveStorageLocation` nur nach bestandener Prüfung, sonst 422; für `recordings` bleibt der alte Ort
  `read_only` (`15` F-17 (A)). `path` muss in `listLiveStorageCandidates` stehen (Regel 4: kein freies Tippen).
  `deleteLiveOrphanRecordingFiles` verlangt `?confirm=<Zahl der Dateien aus getLiveStorage>`, sonst 409; der
  Administrator kann **nie** fremde Aufnahmen mit Zeile löschen (`15` F-18 (A)).
- **Ströme:** `listLiveIngests` mit `reason` (mehrere) und `state`; `stopLiveIngest` beendet den Eingang,
  Zuschauer bekommen 410 beim nächsten Herzschlag, Aufnahmen eine Lücke `stopped_by_admin` und verbinden neu;
  permanente Sender startet der Server danach neu (`15` LA-12). Laufendes endgültig beenden heisst Sender
  deaktivieren und `endLiveChannelNow` (T-29).
- **Diagnose:** `getLiveDiagnostics` → `{now: [{channelId, name, kind: unreachable|frozen|no_audio|reconnecting,
  since, nextAttempt}], deferred: [{channelId, name, cause, since, attempts, nextAttempt}], sources: [{sourceId,
  name, accessState, accessRetryAt, runtime: LiveSourceRuntime}], days7: {outages, reconnects, switches, heat:
  [[n × 24] × 7]}, perChannel: [...]}`. `sources` zeigt je Quelle «vorsorglicher Empfang ruht» und
  «Ein-Strom-Betrieb» mit seit und bis (`04` 5.3); nach einem Neustart ist der Block leer, die Ereignisse
  (`precaution_resting`, `single_stream`) bleiben in `listLiveEvents`. Dazu der Hinweis «Zugang mit nur einer
  Verbindung: Vorladen für alle Quellen ausschalten» als Code `hint: single_connection` (`15`);
  `listLiveEvents` mit `channelId`, `kind` (mehrere), `from`, `to`, `offset`, `limit` (1 bis 500); Texte ohne
  Zugangsdaten, Adressen nur als Host.
- **Übersicht:** `getLiveOverview` liefert alles von LA-01 in einer Anfrage (Muster gebaut:
  `server/internal/admin/dashboard.go:18-21`), auch `setup: {hasSource, hasChannels, hasEpg, accountsEnabled}`
  und `resources: {cpuPct, gpuPct, ramBytes}`; nicht lesbare Werte fehlen (Client zeigt «–», `15` F-22). Neu
  `containment: {state: available|unavailable, reason, consequence: original_only}` (RT-01, `04` 10.8, T-27): Ohne
  wirksame Sperre läuft keine Live-Umwandlung, nur der Direktweg; die Verwaltung zeigt «Eindämmung nicht
  verfügbar» mit der Folge «Live-TV nur im Original; Sender, die eine Umwandlung brauchen, sind nicht abspielbar».
  «Trotzdem erlauben» käme nur mit Variante (c) von `12` F-1 (dann A-13). Dazu `deferredPermanent: {count,
  overHalf}` (Warnung, wenn Dauersender mehr als die Hälfte der Eingangsströme belegen, `04` 8.2). Der Block
  `updateImpact: {running, startingWithin15Min}` (Stufe 5) ersetzt die frühere Operation `getLiveUpdateImpact`
  (S-7, W-28; Warnfenster 15 Minuten, `04` 14.8); LA-27 fragt ihn vor dem Update ab.

### 3.10 Erweiterungen bestehender Modelle

```yaml
# Entwurf: additive fields
SessionInfo:
  properties:
    live:
      $ref: "#/components/schemas/LiveRights"
      description: |
        What the active profile may do in Live-TV (rule 12). Absent without an active profile and on servers
        without Live-TV; then the client shows no Live-TV entry.
LiveRights:
  type: object
  required: [tv, recordings, timeshift]
  properties:
    tv: {type: boolean}
    recordings: {type: boolean}
    timeshift: {type: boolean}
    reason: {type: string, enum: [account_not_allowed, profile_not_allowed, no_channels]}
    profileRecordings: {type: boolean, description: The primary profile may open LS-14.}
Profile:
  properties:
    canLiveTv: {type: boolean}
    canLiveRecord: {type: boolean}
    canLiveTimeshift: {type: boolean}
    liveCapped:
      type: array
      items: {type: string, enum: [tv, recordings, timeshift]}
      description: Switches the account does not allow («Für dein Konto nicht freigeschaltet», 14 1.3).
Account:
  properties:
    live:
      type: object
      properties:
        tv: {type: boolean}
        recordings: {type: boolean}
        timeshift: {type: boolean}
        quotaBytes: {type: integer, format: int64, nullable: true, description: "null = default quota."}
        usedBytes: {type: integer, format: int64, readOnly: true}
        recordingsCount: {type: integer, readOnly: true}
        overQuota: {type: boolean, readOnly: true, description: Used bytes above the quota; new recordings are refused.}
Notification:
  properties:
    live:
      type: object
      description: |
        For kinds live_*; the target may be gone (then the client shows «nicht mehr vorhanden»). The kind
        live_quota_rejected_profile goes to the primary profile: it carries profileName only, and title,
        channelName and at only for a profile with an age limit (RD-07, 07 3.11).
      properties:
        ref: {type: string}
        title: {type: string}
        channelName: {type: string}
        at: {type: integer, format: int64}
        reason: {type: string}
        profileName: {type: string}
MyChannel:
  properties:
    liveTitles:
      type: boolean
      description: |
        Active profile only (stage 2, SM-56): external messages about recordings and reminders name title and
        channel. Default false; never effective for a profile with a PIN (03 DR-05). Not offered in the TV app.
AdminStream:
  properties:
    kind: {type: string, enum: [video, audiobook, podcast, radio, live, live_recording]}
    liveChannel: {type: object, description: kind live only; absent for live_recording (15 section 4).}
    shiftedMs: {type: integer, format: int64}
    # kind live_recording: no resolution, frame rate or source bitrate (SM-45, RT-19)
```

- `updateProfile` und `createProfile` nehmen die drei Felder an; das Hauptprofil darf setzen, was über den Deckel
  geht (wirkt nicht, `07` 4.2); für Profile mit Altersgrenze ab Werk alles aus (DR-01). Ein Profil ohne
  Hauptprofil-Recht: 403 wie heute (`requirePrimary`).
- `updateAccountAccess` und `createInvitation` nehmen `live` an; Senken des Kontingents unter die Belegung: 200,
  die Antwort trägt `live.overQuota: true`, und die Oberfläche zeigt den Hinweis aus `15` LA-22 (nichts wird
  gelöscht).
- `stopStream` beendet auch Live-Sitzungen (Kennung ULID wie bei Titeln); Antwort unverändert.
- **Meldungen nach aussen (SM-56):** E-Mail, Push und Telegram zu Aufnahmen und Erinnerungen nennen ab Werk keinen
  Titel und keinen Sender, nur Art und Profilname; erst `liveTitles` des Profils schaltet sie ein, für Profile mit
  PIN nie. Die Glocke (`Notification.live`) ist davon nicht betroffen.

---

## 4. Streaming-Endpunkte

### 4.1 Routen und Inhalte

| Route | Inhalt | Typ |
|---|---|---|
| `/live/play/{sessionId}/{tune}/index.m3u8?lt=` | übergeordnete Liste: eine Variante mit `CODECS`, `RESOLUTION`, `FRAME-RATE` aus den gespeicherten Parametern (`05` 4.4) | `application/vnd.apple.mpegurl` |
| `/live/play/{sessionId}/{tune}/media.m3u8?lt=` | Wiedergabeliste: ohne Timeshift drei neueste Segmente; mit Timeshift das Fenster ab der Rückspulgrenze; Aufnahme fertig mit `EXT-X-ENDLIST`, laufend wachsend (`05` 4.4, `11` 6.2) | `application/vnd.apple.mpegurl` |
| `/live/play/{sessionId}/{tune}/seg/{gen}/{n}?lt=` | TS-Segment; beginnt mit PAT, PMT und Keyframe; `gen` = Ausgabe-Generation | `video/mp2t` |
| `/live/images/channel/{channelId}/{size}?v=` | Senderlogo, quadratisch skaliert, WebP oder PNG | `image/webp`, `image/png` |
| `/live/images/programme/{imageKey}/{size}?v=` | Sendungsbild (16:9) | `image/webp`, `image/jpeg` |
| `PUT /admin/live/channels/{channelId}/logo` | Logo hochladen (roh, höchstens 10 MiB wie Radio) | – |
| `PUT /admin/live/uploads/{kind}` | M3U-Datei (höchstens 200 MiB, `04` 14.9) oder XMLTV-Datei (Grenzen nach `09`) | – |

Alle Zuschauer-Routen: `x-visibility: scoped`, `x-clients: [app, tv]` in `binaryRoutes` (Muster gebaut:
`server/internal/api/api.go:99-104`, Radio). Pfade mit `{n}` ohne Endung, weil der Router eine Endung im selben
Abschnitt nicht trennt (gebaut: Hinweis zu `subtitleRoute`, `api.go:37-39`); Typ steht im Kopf. Das Senderlogo
liefert dieselbe Route einem Administrator mit Hauptprofil auch für inaktive Sender (S-4: keine zweite
Admin-Route). Jede Live-Liste trägt `EXT-X-DISCONTINUITY-SEQUENCE`, auch ohne Bruch (Apple 8.17, Nachprüfung Nr. 4),
und `EXT-X-PROGRAM-DATE-TIME` an jedem Segment (`05` 4.4).

### 4.2 Token `lt` (T-21)

| Punkt | Festlegung |
|---|---|
| Inhalt | `base64url( sessionId (16 Byte ULID) ‖ tune (uint32) ‖ issuedAt (uint32 Sekunden) ‖ mac (16 Byte) )`, mac = HMAC-SHA256 mit `secrets.Keyring.Derive("live-token")` über alle vorderen Felder und das Kennzeichen `live-v1`, auf 128 Bit gekürzt (Muster Radio: `server/internal/radio/token.go:10-30`) |
| Bindung | an Live-Sitzung und Einstell-Nummer; Pfad und Token müssen übereinstimmen. Ein Token einer früheren Einstellung gilt für die neue nicht. |
| Prüfung je Anfrage | zuerst der HMAC (ohne Datenbank), dann Sitzung im Arbeitsspeicher; je **Listenanfrage** höchstens 30 Sekunden alt: Recht «Live-TV» (Aufnahme: Zugriffsregel), Gültigkeit der Anmeldung (Abmelden, Passwortwechsel, Rolle, Ablauf). Segmente prüfen Token und Sitzung, nicht die Anmeldung. |
| Laufzeit | höchstens 12 Stunden je Einstellen; ab 11 Stunden liefert der Herzschlag `playlistUrl` mit neuem Token, **nur solange Sender und Quelle aktiv sind** (`04` 14.4, T-29, `12` 9.1); auf einem deaktivierten Sender endet die Wiedergabe spätestens mit dem Ablauf |
| Prüfung je Herzschlag | höchstens 30 Sekunden alt: Recht, Anmeldung und bei einer Aufnahme ihre Zugriffsregel (RT-10); scheitert sie: 410 an den Herzschlag, 404 an Liste und Segmente |
| Was die Sitzung hält | nur der Herzschlag (alle 10 s); Segmentabrufe verlängern nichts; 60 s ohne Herzschlag = Ende; ohne Segmentabruf endet sie nach 15 Minuten, mit Sitzungspuffer nach der Höchstdauer des Zeitversatzes plus 15 Minuten (`04` 14.4, RT-11) |
| Fehler | ungültig, fremd, abgelaufen, widerrufen, Sitzung beendet: **404** mit demselben Körper wie «kein Recht» (bewusste Abweichung von den gebauten Token-Routen, die 401 antworten: `server/internal/api/stream_token.go:24-30`, Radio `api/openapi.yaml:12006-12007`; bestätigt `04` 13.4). Ein Segment ausserhalb von Ring oder Fenster ist ebenfalls **404** |
| Client-Regel (RD-16) | An `lt`-Routen wertet der Client einen Fehler **nie** selbst als «Sitzung weg»: Ein 404 an einem Segment heisst «zu spät, weiter mit dem nächsten» (hls.js und Media3 springen nach vorn, Wiederholungsgrenzen nach `16`), ein 404 an der Liste löst sofort einen Herzschlag aus; nur dessen **410** heisst «neu anlegen». So braucht keine HLS-Route einen JSON-Körper, den die Player nicht lesen |
| Protokoll | `lt` kommt in die Schwärzung der Abfrageparameter (gebaut: `server/internal/logging/logging.go:75`), nicht in die Schlüsselnamen (`04` 10.4 Punkt 2) |
| Kopf | `Referrer-Policy: no-referrer` wie bei `st` (V1 L-1) |

### 4.3 Cache-Kopfzeilen und Range

| Route | `Cache-Control` | Range |
|---|---|---|
| Listen | `no-store` | – |
| Segmente einer Live-Sitzung | `no-store` (ohne Timeshift liegen ältere Segmente im Server nicht mehr vor; der Client soll sie auch nicht halten) | einfacher Bereich unterstützt (`Accept-Ranges: bytes`), Antwort 206 |
| Segmente einer Aufnahme | `private, max-age=0, no-store` (Aufnahmen sind privat; gelöschte dürfen nicht im Gerät bleiben) | wie oben |
| Logos und Sendungsbilder | `private, max-age=31536000, immutable`, `v` ist der Inhalts-Hash, `Vary: Accept` (Muster gebaut: `api/openapi.yaml:12120-12125`) | – |

gzip für Listen, wenn `Accept-Encoding` es nennt (Muster Radio, `api/openapi.yaml:12001-12006`).

### 4.4 Was der Service Worker damit tun darf (V1 L-3)

- Alles unter `/api/` ist heute `NetworkOnly` (gebaut: `web/vite.config.ts:123-160`); Listen und Segmente von
  Live-TV bleiben das: kein Zwischenspeichern, Range bleibt unberührt, kein Offline.
- **Neu:** eine `CacheFirst`-Regel für `/api/v1/live/images/channel/…` und `/api/v1/live/images/programme/…` mit
  Ablauf (Vorschlag: 2'000 Einträge, 30 Tage), weil sonst jede Senderliste hunderte Logos neu lädt. Die Bilder sind
  öffentliche Angaben der Sender, aber nur mit Recht abrufbar; nach einem Entzug kann das Gerät gespeicherte Logos
  noch zeigen. Das ist hinnehmbar (kein privater Inhalt) und steht als Restrisiko in `12-sicherheit.md`.
- Aufnahmen, Sendungsdaten und Listen werden nie zwischengespeichert (keine Offline-Wiedergabe in Konzept 28).

### 4.5 Bilder: Authentisierung

Logos und Sendungsbilder laufen wie die Radio-Logos über die Sitzung (Cookie oder Bearer, Middleware
`authorize`, gebaut: `server/internal/api/api.go:205-209`), nicht über `lt`. Ohne Recht «Live-TV» oder für einen
unsichtbaren Sender: 404. Den Abruf beim fremden Host macht nur der Server (`09` 9, `05` 2.3), immer ohne
Heimnetz-Ausnahme.

---

## 5. Statusaktualisierungen (T-23)

Kein SSE, kein WebSocket (gebaut gibt es keinen Push-Kanal, A3 Abschnitt 6). Was wer wie oft abfragt:

| Was | Wer | Takt | Was die Antwort billig macht |
|---|---|---|---|
| Sitzungszustand, Fenster, Hinweise, Schauzeit | Player | alle 10 s (`heartbeatLiveSession`) | nur Arbeitsspeicher, keine Datenbank (`07` 8.2); `nowNext` nur bei Wechsel |
| Störung des Stroms | Player | sieht es an Liste und Segmenten selbst | – |
| Jetzt/danach in Listen | Senderliste, Zapliste | zum Ende der laufenden Sendung (der Client kennt `stop`), sonst alle 5 min (Annahme) | `listLiveChannels?ids=…&now=true` für bis zu 200 Sender |
| Aufnahmezustände | LS-08 mit laufender Aufnahme, LS-11 | alle 30 s, solange die Seite offen ist und eine Aufnahme läuft oder in 10 min beginnt (Annahme) | `listLiveRecordings` mit `state` gefiltert |
| Warnungen | LS-01, LS-11 | beim Öffnen | `listLiveRecordingWarnings` |
| Erinnerung «gleich» | Web und Apps offen | über die Glocke (bestehendes Abfragen der Benachrichtigungen) | – |
| Verwaltungsübersicht, aktive Streams, Diagnose «Jetzt» | LA-01, LA-12, LA-13 | alle 5 s, pausiert bei offenem Dialog (wie `StreamsPage`, `15` 2) | Zähler mit Zwischenspeicher wie das Dashboard (gebaut: `server/internal/admin/dashboard.go:26-36`) |
| Importfortschritt | LA-02 | alle 2 s, solange `importState: running` | `LiveSource.progress` aus dem Job |

**ETag und `If-None-Match`:** Der Vertrag kennt sie heute für JSON nicht (keine Fundstelle in `api/openapi.yaml`).
Live-TV führt sie in den ersten Stufen nicht ein; billig sind die Abfragen durch kleine Antworten und Zustand im
Arbeitsspeicher (O-2).

---

## 6. Entscheid im Server (Regel 12)

Der Client implementiert keine Rechte-, Alters- oder Kontingentregel. Er bekommt fertige Antworten:

| Frage | Feld |
|---|---|
| Sehe ich Live-TV, Aufnahmen, Timeshift, LS-14? | `SessionInfo.live` |
| Darf ich pausieren und spulen? Wie weit zurück? | `LiveSession.timeshift`, `actions.pause`, Herzschlag `windowStart` |
| Welcher Wiedergabeweg, welche Grenze? | `LiveSession.decision` (`path`, `reason`, `cappedTo`) |
| Welche Spuren und Qualitäten gibt es? | `LiveSession.tracks` |
| Darf ich hier aufnehmen, eine Serie anlegen, erinnern? | `LiveProgramme.actions`, `LiveSession.actions.record` |
| Darf ich diese Aufnahme ändern, stoppen, löschen, verlängern? | `LiveRecordingBrief.actions` (RD-19) |
| Darf ich diese Regel ändern, pausieren, löschen, Folgen zurücksetzen? | `LiveRule.actions` (RD-19) |
| Darf ich diese Erinnerung ändern oder löschen? | kein Feld: Erinnerungen gehören immer dem eigenen Profil, offene sind änderbar (`state`) |
| Reicht das Kontingent? Was steht im Weg? | `previewLiveRecording.warnings`, `listLiveRecordingWarnings` |
| Folgen erkennbar? | `LiveRule.episodesDetectable`, `previewLiveRecordingRule` |
| Wann wird gelöscht und warum? | `LiveRecordingBrief.retention` |
| Ist ein Sender gestört? | `LiveChannel.state` |
| Welche Schalter darf das Hauptprofil setzen? | `Profile.liveCapped` |

Der Client rechnet nur Darstellung: Tagesgrenzen in seiner Zeitzone, Breite der Zellen, Wanduhrzeit der Position.

---

## 7. Client-Kompatibilität

### 7.1 Wer was aufruft

| Gruppe | Web | Handy-App (`app`) | TV-App (`tv`) |
|---|---|---|---|
| Sender, Favoriten, Programm, Erinnerungen | ja | ja | ja |
| Sitzung, Wiedergabe, Streams | ja | ja | ja |
| Aufnahmen, Regeln, Warnungen, LS-14 | ja | ja | ja (LS-08 bis LS-14 gibt es am TV, `14` 2) |
| `getProfileLiveUsage`, Profilschalter | ja | ja | ja (TV «Profile verwalten», `14` 1.3) |
| Kanaleinstellungen (`getMyChannels`) | ja | ja | – (wie heute `[app]`) |
| Verwaltung `/admin/live/…` | ja | – | – (Bearer nie `admin`, gebaut: `server/internal/model/model.go:310-334`) |

In welcher Stufe die Apps folgen, ist `03` F-6; der Vertrag nennt `[app, tv]` ab der ersten Stufe, damit eine
spätere App-Fassung keine Vertragsänderung braucht (eine neue Art in `x-clients` wäre zwar nicht brechend, aber
ein Umweg).

### 7.2 Ältere App-Versionen

- Eine App ohne Live-TV ruft keine `/live/…`-Operation auf und sieht nichts.
- `SessionInfo.live` ist ein neues freiwilliges Feld: alte Apps übergehen es.
- `listMyMedia` liefert `livetv` nur mit Recht; eine alte App, die den Schlüssel nicht kennt, übergeht ihn.
  Fassung 1 wollte `livetv` und Glocke-Einträge `live_*` nach App-Fassung filtern; das entfällt (RD-27): Die
  erzeugten Kotlin-Clients der Handy- und TV-App lesen unbekannte Aufzählungswerte als «unbekannt»
  (Konzept 27 11.5, Bestand der Clients), ein Filter je Client-Kennung wäre eine zweite Verträglichkeitsregel
  neben `docs/dev/VERTRAEGLICHKEIT.md`. Der Schema-Test aus Stufe 1a prüft, dass jede neue Aufzählung in den
  App-Clients einen Ausweichwert hat (O-1).
- Glocke-Einträge `live_*` gehen an alle Clients; eine App ohne Live-TV zeigt sie mit dem allgemeinen Text ihrer
  Art «unbekannt» oder übergeht sie.

### 7.3 Offline

Live-TV und Aufnahmen sind nicht offline (Leitplanken, `03` 1.2): kein Download, kein OPFS-Speicher, keine
Prüfung `checkOfflineRevocations`. Ohne Verbindung zeigen die Apps den Bereich als «nicht verfügbar», wie andere
Online-Bereiche (Annahme, `16-client-kompatibilitaet.md`).

---

## 8. Rate-Limits

Paket `ratelimit` (gebaut, `04` 3.2 Abhängigkeit), Antwort 429 `live_too_fast` mit `Retry-After` in Sekunden.

| Was | Grenze (Annahme, `04` 8.5) | Schlüssel |
|---|---|---|
| `createLiveSession`, `tuneLiveSession` | 1 je Sekunde, Stoss 5 | je Sitzung **und** je Profil |
| `setLiveSessionOutput`, `seekLiveSession` | 2 in 10 Sekunden (Neustarts von FFmpeg) | je Sitzung |
| `heartbeatLiveSession` | 1 je 2 Sekunden, Stoss 3 | je Sitzung |
| `searchLiveProgrammes`, `listLiveChannels` mit `q` | 5 je Sekunde, Stoss 10 | je Profil |
| `testLiveSource` (auch Stichprobe), `testLiveEpgSource`, `testLiveChannelStream` | 6 je Minute | je Administrator |
| `refreshLiveSource`, `refreshLiveEpgSource` | einer je Quelle gleichzeitig (409), dazu 6 je Minute | je Quelle |
| `resolveLiveNetTarget` | 10 je Minute | je Administrator |
| `uploadLiveFile` | 2 gleichzeitig | je Administrator |
| `createLiveRecording`, `createLiveReminder`, `createLiveRecordingRule` | 30 je Minute | je Profil |

Die Serverdeckel (Eingänge, Sitzungen, Byte-Budget) sind keine Rate-Limits; sie antworten 503 `live_capacity`.

---

## 9. Abdeckung, offene Punkte, Fragen

### 9.1 Auftrag 14.2

| Zeile (Auftrag, Z.) | Abschnitt |
|---|---|
| Endpunkte (1674) | 2 |
| Request-Modelle (1675) | 3.2 bis 3.10 |
| Response-Modelle (1676) | 3.2 bis 3.10 |
| Berechtigungen (1677) | 1.2, 2 (Spalte Recht), 3.1, 6 |
| Fehlerzustände (1678) | 3.1 und je Gruppe |
| Pagination (1679) | 1.1, 3.2, 3.3, 3.5, 3.8, 3.9 |
| Filter (1680) | 3.2, 3.3, 3.5, 3.8, 3.9 |
| Streaming-Endpunkte (1681) | 4 |
| Statusaktualisierungen (1682) | 5 |
| Client-Kompatibilität (1683) | 7 |
| API-Versionierung (1684) | 1.3 |
| Bestehende Konventionen (1686) | 1.1 |
| Keine parallele API-Architektur (1688) | 1.1, 1.2 |

### 9.2 Seiten → Operationen

| Seite | Daten | Aktionen |
|---|---|---|
| LS-01 Start | `getLiveHome`, `listLiveRecordingWarnings` | `createLiveSession`, `clearLiveRecentChannels` |
| LS-02 Senderübersicht | `listLiveChannels` (`now=true`), `getLiveChannelFacets` | `createLiveSession`, `addLiveFavorite`, `removeLiveFavorite` |
| LS-03 Meine Sender | `listLiveFavorites` | `setLiveFavoriteOrder`, `removeLiveFavorite` |
| LS-04 Programmzeitschrift | `getLiveGuide`; Handy `listLiveChannels` (`now=true`) | `createLiveSession`, `createLiveRecording`, `createLiveRecordingRule`, `createLiveReminder`, `deleteLiveReminder` |
| LS-05 Programmsuche | `searchLiveProgrammes` | wie LS-04 |
| LS-06 Sendungsdetails | `getLiveProgramme`, `getLiveProgrammeImage` | `createLiveSession`, `createLiveRecording`, `updateLiveRecording`, `stopLiveRecording`, `createLiveRecordingRule`, `createLiveReminder`, `updateLiveReminder` |
| LS-07 Live-Player | `createLiveSession`, `getLivePlay*`, `heartbeatLiveSession`, `listLiveChannels` (`ids`, `now=true`) | `tuneLiveSession`, `setLiveSessionOutput`, `seekLiveSession`, `endLiveSession`, `createLiveRecording` (instant), `updateLiveRecording` (`extendToProgrammeEnd`), `stopLiveRecording` |
| LS-08 Meine Aufnahmen | `listLiveRecordings`, `getLiveRecordingUsage` | `deleteLiveRecording`, `deleteLiveRecordings` |
| LS-09 Aufnahme-Detail | `getLiveRecording` | `createLiveSession` (Aufnahme), `updateLiveRecording`, `stopLiveRecording`, `deleteLiveRecording` |
| LS-10 Aufnahme-Player | `createLiveSession`, `getLivePlay*`, `heartbeatLiveSession` | `setLiveSessionOutput`, `seekLiveSession`, `endLiveSession` |
| LS-11 Geplante Aufnahmen | `listLiveRecordings` (`view=upcoming`), `listLiveRecordingRules`, `getLiveRecordingRule` | `updateLiveRecording`, `deleteLiveRecording` (`skip`), `updateLiveRecordingRule`, `deleteLiveRecordingRule`, `clearLiveRuleEpisodes` |
| LS-12 Konflikte und Warnungen | `listLiveRecordingWarnings` | `updateLiveRecording` (`acknowledge`, `chooseCandidate`), `deleteLiveRecording` |
| LS-13 Erinnerungen | `listLiveReminders` | `updateLiveReminder`, `deleteLiveReminder`, `createLiveSession` |
| LS-14 Aufnahmen meiner Profile | `listProfileLiveRecordings` (Übersicht je Profil), `listLiveRecordings` (`profileId`), `getLiveRecordingUsage` | `createLiveSession` (Aufnahme), `deleteLiveRecording` |
| LS-15 Senderprogramm | `getLiveChannel`, `listLiveChannelProgrammes` | `createLiveSession`, `addLiveFavorite`, `createLiveRecording` (manual) |
| LS-20 Navigation | `getSession` (`live`), `listMyMedia` | – |
| LS-21 Profilformular | `listProfiles` | `updateProfile`, `createProfile`, `getProfileLiveUsage` (Löschdialog) |
| LS-22 Benachrichtigungen einstellen | `getMyChannels` | `updateMyChannel` |
| LS-23 Glocke | `listNotifications` | `markNotificationRead` |
| LS-24 Suche | `getSession` (`live`): Verweis nur mit Recht | – |
| LS-25 Ton-Vorrang | – (Client) | – |
| LD-01 bis LD-11 | `previewLiveRecording`, `previewLiveRecordingRule` | `createLiveRecording`, `createLiveRecordingRule`, `updateLiveRecording`, `updateLiveRecordingRule`, `deleteLiveRecording`, `deleteLiveRecordingRule`, `createLiveReminder`, `updateLiveReminder`, `deleteLiveReminder`, `tuneLiveSession` (LD-10) |
| LA-01 Übersicht | `getLiveOverview`, `listLiveIngests` | – (Links) |
| LA-02 Quellen | `listLiveSources` (`counts.orphans` und `running` für die Lösch- und Änderungsfrage, `runtime`), `listLiveSourceImports` | `refreshLiveSource`, `testLiveSource`, `deleteLiveSource` |
| LA-03 Quelle anlegen/ändern | `getLiveSource` | `uploadLiveFile`, `testLiveSource` (ohne und mit `sourceId`, auch Stichprobe), `resolveLiveNetTarget`, `createLiveSource`, `updateLiveSource`, `createLiveEpgSource` (Angebot) |
| LA-04 Import | `getLiveSourceImport`, `listLiveSourceImportItems` (auch Vorschau: Import mit `outcome=preview`) | `applyHeldLiveSourceImport`, `createLiveSource` (`previewImportId`, `excludeEntries`), `bulkUpdateLiveChannels` (Abschalten), `mergeLiveChannels`, `dismissLiveDuplicate` |
| LA-05 Sender | `listAdminLiveChannels`, Senderlogo über die Bildroute (4.1) | `updateLiveChannel` (aktiv), `bulkUpdateLiveChannels`, `mergeLiveChannels` |
| LA-06 Sender bearbeiten | `getAdminLiveChannel` (`running` für die Lösch- und Beenden-Frage), `searchLiveEpgChannels` | `updateLiveChannel`, `uploadLiveChannelLogo`, `setLiveChannelStreams`, `splitLiveChannelStream`, `testLiveChannelStream`, `setLiveEpgMapping`, `deleteLiveChannel`, `endLiveChannelNow` (T-29) |
| LA-07 Duplikate | `listLiveDuplicates`, `previewLiveChannelMerge` | `mergeLiveChannels`, `dismissLiveDuplicate` |
| LA-08 Gruppen | `listLiveGroups` | `createLiveGroup`, `updateLiveGroup`, `deleteLiveGroup`, `setLiveGroupOrder` |
| LA-09 Empfang | `getLiveConfig`, `listLiveSchedules`, `listAdminLiveChannels` (`strategy=custom`) | `updateLiveConfig`, `createLiveSchedule`, `updateLiveSchedule`, `deleteLiveSchedule`, `updateLiveChannel` («Auf Standard zurück») |
| LA-10 EPG-Quellen | `listLiveEpgSources`, `listLiveEpgImports`, `getLiveConfig` | `uploadLiveFile`, `testLiveEpgSource`, `createLiveEpgSource`, `updateLiveEpgSource`, `deleteLiveEpgSource`, `refreshLiveEpgSource`, `acceptHeldLiveEpgImport`, `setLiveEpgSourceOrder`, `updateLiveConfig` |
| LA-11 EPG-Zuordnungen | `listLiveEpgMappings`, `searchLiveEpgChannels` | `setLiveEpgMapping`, `removeLiveEpgMapping`, `acceptLiveEpgSuggestions`, `runLiveEpgAutoMapping` |
| LA-12 Aktive Streams | `listLiveIngests` | `reconnectLiveIngest`, `switchLiveIngestSource`, `stopLiveIngest` |
| LA-13 Diagnose | `getLiveDiagnostics` (auch `sources` mit «ruht» und «Ein-Strom-Betrieb», `deferred`), `listLiveEvents`, `getLiveConfig` | `testLiveChannelStream`, `updateLiveConfig` |
| LA-14 Aufnahmen (DVR) | `getLiveConfig` | `updateLiveConfig` |
| LA-15 Timeshift | `getLiveConfig` | `updateLiveConfig` |
| LA-16 Speicher | `getLiveStorage`, `listLiveStorageCandidates` | `validateLiveStorageLocation`, `setLiveStorageLocation`, `checkLiveStorageLocation`, `cleanupLiveStorage`, `deleteLiveOrphanRecordingFiles`, `updateLiveConfig`, `updateAccountAccess` (Kontingent) |
| LA-20 Verwaltungsübersicht | `getDashboard` (`live`) | – |
| LA-21 Aktive Wiedergaben | `listStreams` | `stopStream` |
| LA-22 Konten, Einladungen | `listAccounts`, `listInvitations` | `updateAccountAccess`, `createInvitation` |
| LA-23 Hintergrundjobs | `listJobRuns` | bestehende Job-Operationen |
| LA-24 Benachrichtigungen (Verwaltung) | bestehende Operationen (Ereignis «Fehler») | – |
| LA-25 Seitenleiste, Suchpalette | `getLiveOverview` (Pillen), `listAdminLiveChannels`, `listLiveSources` (Suche ab 3 Buchstaben) | – |
| LA-26 Meine Medien | `getMyMedia` | `setMyMedia` |
| LA-27 Update | `getLiveOverview` (`updateImpact`) | `installUpdate` (`confirmLiveRecordings`) |
| LA-28 Sicherung | – (Text) | – |

**Ohne Operation**, bewusst: LS-25 (nur Client), LA-28 (nur Text), LD-08 und LD-09 (Bedienung im Client).
**Nicht modelliert**, offen: Zeitfenster, Wochentage, «nur Erstausstrahlungen», Schalter «Folgen nicht doppelt»
einer Serie und Ende einer Wiederholung (`14` LD-02, LD-03; Frage an dich über `11` O-6, `07` W-10, W-11, O-3),
Vorladen je Sender (`07` W-23), Kopfzeilen einer EPG-Quelle (W-27).

### 9.3 Widersprüche (zusätzlich zu `07` 12.3)

| Nr. | Thema | Stellen | Gewählt |
|---|---|---|---|
| W-25 | Status bei Serverkapazität | Bestand Radio 409 `radio_capacity` (`api/openapi.yaml:11809-11816`), Dateiwiedergabe 503 `transcode_busy`; Auftrag für dieses Dokument «503/507 bei Auslastung» | 503 `live_capacity` (wie `transcode_busy`: vorübergehend, Wiederholung sinnvoll) |
| W-26 | Programmsuche in der Beschreibung | `14` LS-05 «Titel, Episodentitel, Beschreibung» gegen `09` 8.4 «Beschreibung nicht in Stufe 2» | `09` |
| W-27 | Kopfzeilen und User-Agent einer EPG-Quelle | `15` LA-10 «wie LA-03» gegen `09` 13.1 (keine), `03` LTV-003.18 (nur Quellen) | keine; O-3 |
| W-28 | Vorwarnzeit vor dem Update | `04` 9 und `11` 12 «15 Minuten» gegen `15` LA-27 «30 Minuten» | 15 Minuten |
| W-29 | Token-Fehler | gebaute Token-Routen 401 gegen `04` T-21 404 | 404 (`04` entscheidet bewusst, bestätigt `04` 13.4) |
| W-33 | Operationen je Stufe und Verträglichkeit | `18` 4.3 bis 4.9 zählt die Operationen aus Fassung 1 (122) und nennt in den Zeilen zur Verträglichkeit den Filter nach App-Fassung | 2.13 (115) und 7.2 ohne Filter (RD-27); `18` nachziehen |
| W-34 | Rückschau der Programmzeitschrift | `14` 4.1 «ab heute 00:00» gegen `09` 2.7 «bis gestern Abend» | Vertrag erlaubt beides (`epg_lookback_hours`); die Oberfläche folgt `14` |

### 9.4 Offene technische Punkte

| Nr. | Punkt | Gehört zu |
|---|---|---|
| O-1 | Unbekannte Aufzählungswerte in den Apps (`MyMediaKey`, `NotificationKind`, `AdminStream.kind`): Ausweichwert in den erzeugten Kotlin-Clients durch einen Schema-Test belegen (7.2); kein Filter nach App-Fassung (RD-27) | Bau Stufe 1a, `16` |
| O-2 | ETag und `If-None-Match` für `getLiveGuide` und `listLiveChannels`, falls die Messungen `09` M-4, M-5 es verlangen | nach Stufe 2 |
| O-3 | Kopfzeilen und User-Agent für EPG-Quellen (W-27) | `09` Fassung 2 |
| O-4 | **erledigt durch `04` 9 und 14.1:** `createLiveSession` und `tuneLiveSession` antworten nach höchstens 3 s (mit `state: starting`, wenn das erste Segment noch fehlt); danach lädt der Client die Liste erst bei `live`; drei Startfristen, zusammen höchstens 90 s, danach `start_failed` (3.4). Ob die erste Liste in der Antwort mitkommt, bleibt `05` O-17 | – |
| O-5 | Form der `programmeRef` bei Sendungen gleichen Beginns und Titels auf einem Sender (Überlappung mit gleicher Prüfsumme): zweite Unterscheidung nötig? | `09` |
| O-6 | hängt an F-1 (`20` E-28-42): Vorgabe (b) ohne Ausnahme; nur bei (a) A-12 (Liste, Belegung und Löschen ohne eigenes «Live-TV», Abspielen nicht, 3.5) | – |
| O-8 | Laufzeitzustand je Quelle über einen Neustart merken (`04` O-32): heute flüchtig, `LiveSourceRuntime` nach dem Neustart leer | `04`, `20` |
| O-9 | Wie hls.js, Media3 und Safari auf eine Live-Liste ohne Segment reagieren, obwohl der Client sie erst bei `live` lädt (RB-15) | M-3, M-4 |
| O-7 | **erledigt durch RD-27:** kein Filter je Client-Kennung (7.2) | – |

### 9.5 Fragen an dich (nur neue)

**F-1 · Darf das Hauptprofil Aufnahmen seiner Profile löschen, wenn das Konto kein Live-TV mehr hat?** → `20` E-28-42
Schaltet der Administrator Live-TV für ein Konto aus, ist der ganze Bereich für alle Profile 404 (`03` 2.4); die
Aufnahmen bleiben und belegen das Kontingent. `03` DR-03 erlaubt dem Hauptprofil das Löschen auch ohne Recht
«Aufnahmen», sagt aber nichts zu «Live-TV».

- **(a) Ja, als benannte Ausnahme A-12:** LS-14 und das Löschen der Aufnahmen seiner Profile mit Altersgrenze
  bleiben für das Hauptprofil erreichbar (nur Liste, Belegung und Löschen, kein Abspielen), damit der Platz frei
  werden kann. SM-38 («ohne Recht jede Live-Operation 404») gilt mit dieser einen Ausnahme.
- **(b) Nein:** Ohne Live-TV ist alles 404, SM-38 ohne Ausnahme; freigeben kann nur der Administrator, indem er
  das Recht wieder gibt.

Empfehlung: **(a)**, weil sonst Aufnahmen bleiben, die niemand mehr löschen kann (der Administrator sieht keine
Titel und löscht nicht, `15` F-18 (A)). **Vorgabe bis zu deiner Antwort:** (b), wortlauttreu zu «ohne Recht
unsichtbar» (`20` E-28-42, `02` A-12, `03` DR-03); A-12 gilt erst mit (a).

---

## 10. Änderungen gegenüber Fassung 1

Grundlage wie `07` Abschnitt 13. Hier steht, was die Operationen und Modelle betrifft; was nur das Schema
betrifft, steht dort.

### 10.1 Befunde aus Review 2

| Befund | Änderung im Vertrag |
|---|---|
| RD-01 Verklemmung beim Löschen | keine neue Operation; `deleteProfile` und `deleteAccount` antworten erst nach dem Stoppen (`07` 10.1, 10.2); `getProfileLiveUsage` bleibt für den Löschdialog |
| RD-02 Wiedererkennung | Import-Einträge mit `changes: identity_conflict`, nicht angewandt (3.7) |
| RD-03 Import atomar | keine Vertragsänderung; `getLiveSourceImport` zeigt `held` und die Schwelle (3.7) |
| RD-04 Kindspalten ohne Index | nur Schema (`07`) |
| RD-05 Werkseinstellung | `Profile.liveCapped` folgt der einen Definition (`07` 4.4); kein neues Feld |
| RD-06 Konto laufender Aufnahmen | `listLiveIngests.viewersDetail` ohne Aufnahmen und ohne Konto; Aufnahmen nur als Zahl `recordings` (3.9) |
| RD-07 Meldung an das Hauptprofil | neue Art `live_quota_rejected_profile`, `Notification.live.profileName`, Titel nur bei Profilen mit Altersgrenze (3.10) |
| RD-08 Sicherung | keine Vertragsänderung |
| RD-09 `rule_key` | keine Vertragsänderung; `LiveRule.skipped` bleibt nach Zusammenführen richtig |
| RD-10 Anlegen nicht idempotent | `createLiveRecording`, `createLiveRecordingRule`, `createLiveReminder` antworten bei Gleichem 200 mit dem Bestand (3.1, 3.5, 3.6); ein Gerät, eine Sitzung (3.4) |
| RD-11 Zusammenführen | `mergeLiveChannels` mit `unionGroups`; Termine und Regeln nach `07` 7.4; `previewLiveChannelMerge` nennt die wegfallenden Doppel (3.8) |
| RD-12 Sitzungszeilen | keine Vertragsänderung |
| RD-13 Löschhaken | keine Vertragsänderung |
| RD-14 Kontingent | `getLiveRecordingUsage`, `Account.live.usedBytes` und `previewLiveRecording.warnings` lesen dieselbe Abfrage `LiveAccountUsage`; 507 `live_quota_full` nur bei der Sofortaufnahme, geplante lehnt der Start ab und meldet es (3.5) |
| RD-15 Zeitgeber | `LiveRecordingUpdate.rev` (freiwillig): ein anderes `rev` antwortet 409 `live_conflict` (`reason: changed`); den Rest regelt der Server (`07` 8.3) (3.5) |
| RD-16 404 an `lt`-Routen | Client-Regel in 4.2: nur 410 vom Herzschlag heisst «neu anlegen» |
| RD-17 Aktionen fehlen | Pflichtfeld `actions` an `LiveRecordingBrief` und `LiveRule` (3.5, 3.6, 6) |
| RD-18 Zugriff des Hauptprofils | `listProfileLiveRecordings` als Übersicht `{items, total, offset}` je Profil, Einträge über `listLiveRecordings?profileId=`; dieselbe Bedingung für Liste und Einzelabruf (3.5) |
| RD-19 User-Agent | `hasCustomUserAgent` statt Wert (3.7) |
| RD-20 Erinnerungen | `deleteLiveReminder` löscht; neues Setzen gibt eine neue Kennung (3.6) |
| RD-21, RD-22, RD-23 | nur Schema |
| RD-24 Bild der Aufnahme | keine Vertragsänderung; `getLiveProgrammeImage` liefert das Bild, solange der Cache es hat. Teilweise übernommen, siehe `07` 13.1 |
| RD-25 Glocke | keine Vertragsänderung ausser RD-07 |
| RD-26 Widersprüche im Text | Status-Tabelle ohne `getLiveNowNext` (5); Token-Fehler mit `04` 13.4 belegt (4.2, W-29) |
| RD-27 Filter nach App-Fassung | gestrichen (7.2, O-1, O-7) |
| RD-28 Antwortmodelle | 200 statt 201 bei Bestehendem (3.1); `AdminLiveChannelRow` für die Liste (3.8); `counts.orphans` an `LiveSource` (3.7) |
| RD-29 Lücken der Stichprobe | `createdAt` (LS-09), Filter `kind` (LS-11), `channelId` in der Änderung (LS-12), `sort` für Nummern (LA-05), `unionGroups` (LA-07). Logo-Vorschau (LA-06) **nicht übernommen**, weil nur der Server den fremden Host abrufen darf: Vorschau nach dem Speichern (3.8). Grund «Wiederholung» (LS-11) offen mit W-10 |
| RD-30 Optionen im Schema | `live_recording_deleted` steht nicht im Vertrag, bis `14` F-4 beantwortet ist (3.10) |

### 10.2 Streichungen (Review 2 Abschnitt 2)

| Nr. | Fassung 1 | Fassung 2 |
|---|---|---|
| S-1 | `getLiveNowNext` | `listLiveChannels?ids=…&now=true` |
| S-2 | `testNewLiveSource` und `testLiveSource` | `testLiveSource` mit freiwilliger `sourceId`, auch als Stichprobe |
| S-3 | `getLiveImportPreview`, `listLiveImportPreviewItems` | Lauf mit `outcome = preview`, gelesen mit `getLiveSourceImport` und `listLiveSourceImportItems`; `previewImportId` in `createLiveSource` und `updateLiveSource` |
| S-4 | `getAdminLiveChannelLogo` | dieselbe Bildroute mit Ausnahme für Administratoren (4.1) |
| S-5 | `getLiveChannelRemovalImpact` | `AdminLiveChannel.usage` um `reminders`, `rules` ergänzt |
| S-6 | `getLiveSourceRemovalImpact` | `LiveSource.counts.orphans` |
| S-7 | `getLiveUpdateImpact` | `getLiveOverview.updateImpact` |
| S-11 | Filter nach App-Fassung | gestrichen (RD-27) |

S-8 bis S-10 und S-12 betreffen nur das Schema (`07` 13.2).

### 10.3 Aus `04` Fassung 3, `11` Fassung 2 und `18`

- Neue Operation `endLiveChannelNow` für «sofort beenden» (T-29); `AdminLiveChannel.running` nennt nur Zahlen.
- `LiveIngest.state` kennt `deferred` (in Fassung 3 nur noch für einen Dauersender, der nicht starten kann).
- Herzschlag mit Pausenangabe; Token-Erneuerung ab 11 Stunden, Ende nach 60 s ohne Herzschlag (4.2; in Fassung 3
  zählt die Schauzeit ohne Rücksicht auf die Pausenangabe).
- `createLiveSession` und `tuneLiveSession` antworten nach höchstens 3 s (3.4; in Fassung 3 drei Startfristen).
- Grenzen aus `04` 14 im Vertrag: Anteil der Umwandlung 0 bis 100 %, Upload M3U 200 MiB (3.9, 4.1; die Ringlänge
  ist seit Fassung 3 keine Einstellung).
- Stufen 1a, 1b, 2, 3, H, 4, 5, 6 je Operation (Abschnitt 2); `resolveLiveNetTarget` erst Stufe H (D-4).
- Felder aus `14` LD-02/LD-03 nicht im Vertrag, offen über `11` O-6 (3.6, 9.2).

### 10.4 Zählung

115 neue Operationen (Fassung 1: 122), davon 7 binär; 20 erweiterte bestehende Operationen. Gestrichen oder
zusammengelegt: 8 (S-1 bis S-7, `getLiveImportPreview` und `listLiveImportPreviewItems` als zwei); neu: 1
(`endLiveChannelNow`).

Restebereinigung nach Konsolidierung 1: Frage F-1 mit Verweis auf `20` E-28-42.

Restebereinigung nach Konsolidierung 1: Der Schalter je Quelle bekam einen neuen Namen; in Fassung 3 ist er gestrichen (Abschnitt 11).

---

## 11. Änderungen gegenüber Fassung 2

Grundlage wie `07` Abschnitt 14 (Schlussreviews 4, 5 und 6; `04` Fassung 4 Abschnitt 15.3; `12` Fassung 3;
Entscheide des Koordinators). Keine neue und keine gestrichene Operation: weiterhin 115 neue (7 binär) und 20
erweiterte bestehende; `getMyChannels` und `updateMyChannel` waren schon erweitert und bekommen nur das Feld
`liveTitles`.

| Anlass | Änderung im Vertrag |
|---|---|
| Grundregel (RG-01) | Kopf: ohne Antwort gilt die wortlauttreue Vorgabe; Felder anderer Varianten nur als Hinweis |
| RG-03, SC-1, `04` 15.3 | `preloadEnabled` aus `LiveSource` und `LiveSourceWrite` gestrichen; käme nur mit `04` F-3 (b) (3.7) |
| RG-03, `04` 5.3 | `LiveSourceRuntime` (vorsorglicher Empfang ruht, Ein-Strom-Betrieb, je mit seit und bis) in `LiveSource` und `getLiveDiagnostics.sources`; flüchtig (`07` 1.3); Hinweis `single_connection` (3.7, 3.9) |
| RG-02, `04` 5.3 | `deferred` nur für einen Dauersender, der nicht starten kann; `LiveIngest.deferred` mit Ursache, Zahl der Versuche und nächstem Versuch; `getLiveOverview.deferredPermanent` (3.9) |
| T-7, RB-01 | `ringSeconds` und `idleCheckParallel` aus `LiveConfig` gestrichen (Konstanten aus `04` 14) |
| SC-2 | `maxConcurrent` gestrichen; käme mit `20` E-28-49 (B) |
| RB-02, RB-07, `04` 9 | drei Startfristen; `state: start_failed` mit `LiveStartError`; Haltegrund und Reservierung zurück, Sitzung bleibt; `reconnecting` nur für eine laufende Wiedergabe; ein sofortiger Versuch auf wartendem oder gestörtem Strom (`startingReason: retrying`); der 502 beim Einstellen entfällt (3.1, 3.4) |
| RB-15 | `409 live_superseded`; Liste nur im Zustand `live` (3.1, 3.4); Verhalten der Player bei leerer Liste als O-9 |
| RT-04, SM-50 | `playing` nur Anzeige und Drosselung; Schauzeit nach ausgelieferten Segmenten; Feld nicht mehr Pflicht (3.4) |
| RT-11 | Sitzungsende nach 15 Minuten ohne Segmentabruf (mit Sitzungspuffer Höchstdauer plus 15 Minuten), ohne Puffer 60 Sekunden nach Pause; `deviceId` aus der Anfrage gestrichen, Gerät aus der Anmeldung (3.4, 4.2) |
| RT-10 | Herzschlag prüft Recht, Anmeldung und Zugriffsregel einer Aufnahme; Löschen beendet Sitzungen (3.4, 4.2) |
| RT-12, SM-52 | feste Reihenfolge der Prüfungen; 410 nur für eigene Sitzungen; `skipped` mit `not_found` (3.1) |
| T-29, RB-05 | Token-Erneuerung nur bei aktivem Sender und aktiver Quelle; `AdminLiveChannel.disabledAt`; Hinweis beim Deaktivieren (3.4, 3.8, 4.2) |
| RB-05, RT-06, SM-47 | Entfernen oder Ändern einer Quelle verlangt `confirm` mit `LiveSource.running` und beendet ihre Empfänger innert 5 Sekunden (3.7) |
| RT-05, `04` 10.9, `12` 4.5 | `LiveNetTarget` und `LiveNetTargetWrite`: Einzeladresse mit Port, Netz höchstens /28 bzw. /124 und 16 Ports nur mit `wideConfirm`, nie mit Gateway, höchstens 16 Ziele (3.7) |
| RT-01, `04` 10.8 | `getLiveOverview.containment` («Eindämmung nicht verfügbar», Folge «nur im Original»); `AdminLiveChannel.playability` (`transcode_only`, `long_gop`) (3.8, 3.9) |
| RT-09, A-12 | Hauptprofil sieht und löscht Aufnahmen seiner Profile mit Altersgrenze ohne eigenes wirksames «Live-TV»; F-1 mit Vorgabe (a) als A-12; O-6 erledigt (3.1, 3.5, 9.5) |
| SM-56, RT-07 | `MyChannel.liveTitles`: Meldungen nach aussen ab Werk ohne Titel und Sender (3.10) |
| SM-45, RT-19 | `LiveIngest.recordings` erst ab 2, sonst `recordingsActive`; `AdminStream` einer Aufnahme ohne Auflösung, Bildrate, Bitrate (2.12, 3.9, 3.10) |

Letzte Angleichung: F-1 mit Vorgabe (b) nach `20` E-28-42; 3.1, 3.5 und O-6 entsprechend, A-12 nur als Variante (a).
Letzte Angleichung: LS-14 ohne Zahl der geplanten Termine nach `20` E-28-43 (A) (2.4, 3.5).
