# Konzept 25 · Abdeckungsmatrix: Seiten, Mockups, Umsetzung

> **Stand:** Fassung 2 der Papiere, Mockup vom 17. November 2026.

> **Kurz gesagt.** Diese Matrix führt jede Seite auf, die Konzept 25 berührt: 13 Seiten der Zuschauer-App
> (M-01 bis M-12 und der Dialog M-13), 11 Seiten der Verwaltung (M-20 bis M-30) und die Mockup-Steuerung M-00.
> Je Seite steht, was sie heute kann, was sich ändert, wer sie nutzt, welche Einstellungen und Operationen
> dazukommen, was man im Mockup wirklich anklicken kann und wann und wo der Bau sie umsetzt. Danach folgen
> Gegenproben für jede neue Einstellung, jede neue Berechtigung und jeden Zustand. Grundlage ist der Code des
> Mockups (`js/*.js`), nicht der Plan. Abschnitt 9 nennt die Stellen, die das Mockup nicht bedienbar macht oder
> nicht zeigt. Abschnitt 10 nennt die bewussten und die verbleibenden echten Unterschiede zwischen Mockup und
> Papieren.

Grundlage sind die Papiere `01-…` bis `11-…` in Fassung 2. Operationen nach `08-datenmodell-api.md` Abschnitt 4,
Einstellungen nach Abschnitt 2.12, Stufen und Schritte nach `09-umsetzungsplan.md`, Verwaltung nach
`07-administration.md`, Abnahme nach `10-abnahmekriterien.md`. Verweise auf Anhang F meinen
[Anhang F](anhang/F-entscheid-blatt.md). Anhang F bleibt als Zwischenstand stehen; wo die Papiere davon abweichen,
gilt das Papier (`11-entscheide-und-adr.md`, Abschnitt 7.3). Wichtige Festlegungen von Fassung 2:
`listAccountWishes` ist `scoped` und nur mit Hauptprofil; fehlende Titel sieht ein Profil nur, wenn es sie
anfordern kann, ausser bei eigenen Wünschen (F-9); der Reiter «Wünsche» steht nach «Pflege»; die Reihe «Deine
Wünsche» steht ab Werk nach «Weiterschauen»; die Zuordnung wird direkt in der Liste geändert; Einstellungen der
Wünsche gelten sofort; die Benachrichtigung «verfügbar» kommt in Stufe 2, Staffel-Meldungen, `wish_status` und
`wish_review` in Stufe 4; acht neue Jobarten mit `wishes.withdraw` und `wishes.import`; der Sammelbenutzer heisst
im Wert `fallback`; Teilfragen T-1 und T-2.

## 1. So öffnest du das Mockup

- **Einstieg:** `docs/planning/mockups/25-medienwuensche/index.html` im Browser öffnen. Ohne Hash springt das
  Mockup auf die Route `#/mockup`: die Übersicht aller Mockup-Seiten mit Kennung, Name und Link.
- **Steuerung:** die gelbe Schaltfläche «Mockup» unten rechts auf jeder Seite (M-00). Sie öffnet die
  «Mockup-Steuerung» mit sechs Teilen: «Ansicht» (Desktop, Tablet, Handy), «Wer schaut?» (alle sechs Profile mit
  Altersstufe, darunter Rolle und Recht des aktiven Profils), «Jellyseerr ist …» (Verbunden, Nicht erreichbar,
  Schlüssel ungültig, CSRF-Schutz an, Nicht eingerichtet) mit «Abgleich jetzt ausführen», «Offene Wünsche
  vorspulen» (je offener Wunsch ein Knopf «Übermitteln», «Freigeben», «Datei trifft ein» oder «Nächste Staffel
  trifft ein», dazu «Ablehnen» bei `pending`, «Scheitern» bei `processing` und `partial`, «24 h vergehen» bei
  `queued`), «Zuletzt zugestellt» (die letzten vier Zustellungen an Kanäle) und «Alle Mockup-Seiten» sowie «Alles
  zurücksetzen».
- **Zustand:** Alles bleibt im Browser (`localStorage`, Schlüssel `slouchcat-mockup-25-v4`), bis «Alles
  zurücksetzen». Ist Jellyseerr nicht verbunden, trägt die Schaltfläche den Zusatz «· Jellyseerr: {Zustand}».
- **Vorführwerte:** Das Mockup startet mit «Jellyseerr verwenden», «Wünsche aktiv» und «Fehlende Titel zeigen»
  eingeschaltet. Ab Werk sind alle drei aus (Abschnitt 10.1).
- **Klickfolgen** in diesem Papier beginnen nach «Alles zurücksetzen» mit dem Profil Marco, wenn nichts anderes
  steht. «Mockup › Noah» heisst: Steuerung öffnen, unter «Wer schaut?» Noah wählen.
- **Beispieldaten** (Anhang F 11, `04-benutzer-profilmapping.md` Abschnitt 11): Konto `marco@example.ch` mit Marco
  (Hauptprofil, 18), Sandra (18, PIN), Noah (12), Mia (6, darf nicht anfordern); `grosseltern@example.ch` mit
  «Oma & Opa» (16, Konto darf, keine Zuordnung); `gast@example.ch` mit «Gast» (16, Konto darf nicht).
  Jellyseerr-Benutzer 1 marco, 2 sandra, 3 kinder (Freigabe nötig, Kontingent 5 je 7 Tage), 4 grosseltern (keine
  Serien), 5 wohnzimmer-tv (gehört keinem Konto).

## 2. Matrix je Seite

### M-00 · Mockup-Steuerung und Seitenübersicht (nur Mockup)

| Zeile | Inhalt |
|---|---|
| Seite | Keine App-Seite. Route im Mockup `#/mockup` und die Schaltfläche «Mockup» auf jeder Seite |
| Heute | gibt es nicht |
| Geplante Änderung | keine; ersetzt im Mockup Jellyseerr, Radarr, Sonarr und die Zeit |
| Rolle | Marco als Prüfer |
| Neue oder veränderte Einstellungen | keine der App; Mockup-Zustand `S.view`, `S.seerr.condition` (`ok`, `unreachable`, `badkey`, `csrf`), `S.seerr.configured`, `S.session` |
| API-Operationen | keine |
| Mockup | M-00, `#/mockup`, Datei `js/mock.js` |
| Abgedeckte interaktive Zustände | Ansicht Desktop, Tablet, Handy; Profilwechsel (wechselt auch das Konto; von einer Verwaltungsseite aus springt ein Nicht-Administrator auf `#/`); fünf Zustände von Jellyseerr, darunter «CSRF-Schutz an» (Protokolleintrag «HTTP 403: CSRF-Schutz blockiert schreibende Aufrufe»); «Abgleich jetzt ausführen» übermittelt wartende Wünsche oder meldet «Abgleich nicht möglich»; Vorspulen je Wunsch; «24 h vergehen» bei `queued`: der Wunsch wird `failed` mit Grund `timeout` («Jellyseerr war 24 Stunden lang nicht erreichbar.»), das Profil und seine Mitwünscher bekommen «Fehlgeschlagen: …», die Administratorkonten eine Zustellung «Wunsch nicht übermittelt: …» über Kanäle mit dem Ereignis «Fehler»; «Zuletzt zugestellt»; Zurücksetzen; Übersicht mit Anleitung «So probierst du es aus» und Links zu M-01 bis M-30 |
| Spätere technische Umsetzung | wird nicht gebaut |

### M-01 · Startseite mit Spotlight (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/` |
| Heute | Reihen nach Verwaltung «Startseite»: Spotlight-Bühne mit Posterreihe (nur Titel der Bibliothek, erste Folie vorgemalt), Weiterschauen, Neu hinzugefügt, Kürzlich veröffentlicht, User-Empfehlungen, Merkliste, ausblendbar Sammlungen, Genres, Meine Medien; Posterkarten mit «Gesehen» und «Merken» |
| Geplante Änderung | Bühne und Posterreihe zeigen auch fehlende Titel (Chip «Noch nicht in der Bibliothek», Hauptknopf «Film anfordern» oder «Serie anfordern», danach Zustandsknopf); fremd angeforderter Titel mit «Benachrichtige mich» oder, bei «Nur Zustand zeigen», nur mit dem Chip «Bereits angefordert»; Posterkarte eines fehlenden Titels mit rundem Plus-Knopf statt «Gesehen» und «Merken»; Chip «Dein Wunsch ist da» 14 Tage nach der Erfüllung; neue Reihe «Deine Wünsche» ab Werk nach «Weiterschauen» |
| Rolle | Zuschauer; fehlende Titel nur für Profile, die sie anfordern können (F-9) |
| Neue oder veränderte Einstellungen | wirken hier: «Fehlende Titel zeigen», «Höchstens fehlende Titel», «Reihenfolge» (M-27); «Wünsche aktiv», «Anfordern aus dem Spotlight», «Filme erlauben», «Serien erlauben», «Bereits angeforderte Titel» (M-25); Platz und Ausblenden von «Deine Wünsche» (M-28) |
| API-Operationen | `getHome`, `listHomeRow` (neu `HomeRow.external[]` mit `ExternalTitleRef`), `getExternalTitleImage`, `createWish` über M-13, Sitzung `wishes` |
| Mockup | M-01, `#/`, `js/home.js` (`route('/')`, `heroHtml`, `posterCard`, `mainActions`, `metaLine`, `homeRowItems`); Mischregel `spotlightFor` in `js/store.js` |
| Abgedeckte interaktive Zustände | Bühne blättern mit «Zurückblättern»/«Weiterblättern», Wahl in der Posterreihe (hält an), «Pause»/«Weiter», Wechsel alle 6 s (steht bei Fokus in der Bühne, offenem Dialog, offener Mockup-Steuerung); erste Folie immer vorhanden («Heart of the Beast»); fehlender Titel «Odyssey» mit Chip, «Film anfordern», «Details», «Trailer»; eigener Wunsch «Resident Evil» mit Zustandsknopf «In Beschaffung» nach `#/wishes`; fremder Wunsch «Verity» mit Chip «Bereits angefordert» und «Benachrichtige mich»; vorhandener Titel mit «Abspielen» (Dialog); «Godzilla Minus One» mit Chip «Dein Wunsch ist da» (14 Tage); teilweise vorhandene Serie «Tulsa King» mit Chip «2 von 3 Staffeln vorhanden» (nur für Profile, die anfordern können); Notiz aus M-27 unter dem Titel; Posterreihe: Plus-Knopf (fehlt), Glocke (fremder Wunsch), Zustands-Chip unten links (eigener Wunsch); Reihe «Deine Wünsche» (Marco: Resident Evil, Godzilla Minus One). «Nur Zustand zeigen» (M-25): «Verity» zeigt auf der Bühne nur «Details» und «Trailer», die Posterkarte den Chip «Bereits angefordert» statt der Glocke. «Aus dem Spotlight» aus: keine fehlenden Titel auf der Bühne. Profile: Mockup › Noah zeigt «Toy Story 5» mit «Freigabe ausstehend»; Mockup › Gast oder «Oma & Opa» zeigt keine fehlenden Titel; Mockup › Mia zeigt gar kein Spotlight (nur 2 Treffer, «Ausblenden unter» ist 5). Mockup › «Nicht eingerichtet» nimmt alle fehlenden Titel weg |
| Spätere technische Umsetzung | Stufe 3: Schritt 3.2 (Mischregel in `server/internal/discover/`, `visibility/discover.go`, `queries/scoped/spotlight.sql`), 3.3 (Vertrag `HomeRow.external`, `HomeRowKey` + `wishes`), 3.4 (`web/src/ui/HeroCarousel.tsx`, `HomePage.tsx`); Tabellen `spotlight_entries` (+ `tvdb_id`, `imdb_id`), `spotlight_config`, `home_rows`, `external_titles`; Regeln `05-spotlight-integration.md` Abschnitte 2 bis 4 und 12; Messung nach `10-abnahmekriterien.md` Abschnitt 7 |

### M-02 · Spotlight-Liste (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/lists/spotlight` |
| Heute | ganze Trakt-Liste als Posterraster, nur Titel der Bibliothek |
| Geplante Änderung | auch fehlende Titel; Filter «Alle», «In der Bibliothek», «Noch nicht da»; Unterzeile «… davon n noch nicht in der Bibliothek» |
| Rolle | Zuschauer, die anfordern können (Filter und fehlende Titel), sonst wie heute |
| Neue oder veränderte Einstellungen | wirken hier: «Fehlende Titel zeigen» (M-27), «Wünsche aktiv», «Anfordern aus dem Spotlight» (M-25); ausgeblendete Einträge und Medienart (M-27, bestehend) |
| API-Operationen | `listHomeRow` für die Reihe `spotlight` (erweitert um `external[]`) |
| Mockup | M-02, `#/lists/spotlight`, `js/viewer-titles.js` (`route('/lists/spotlight')`) |
| Abgedeckte interaktive Zustände | Filterpillen mit Zählern (nur wenn fehlende Titel gezeigt werden); leere Auswahl mit eigenem Text; Spotlight aus («Das Spotlight ist ausgeschaltet.»); Posterkarten wie M-01 mit Plus-Knopf, Glocke, Chip «Bereits angefordert» oder Zustand; «Aus dem Spotlight» aus (M-25): nur vorhandene Titel, keine Filter; Mockup › Gast zeigt nur vorhandene Titel ohne Filter; «Zurück» |
| Spätere technische Umsetzung | Stufe 3, Schritt 3.4 (`HomeListPage.tsx`); `05-spotlight-integration.md` Abschnitt 13 |

### M-03 · Vorschauseite Film (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/titles/movie/{tmdbId}` |
| Heute | gibt es nicht; fehlende Titel haben keine Seite |
| Geplante Änderung | Aufbau wie die Detailseite (Hintergrund, Poster, Titel, Metazeile mit Freigabe, Genres, Tagline, Beschreibung, Besetzung); Hauptknopf «Film anfordern» oder der Zustand; Block «Dein Wunsch» mit Verlauf; letzter geschlossener Wunsch mit Datum und Grund; vorhandener Titel leitet auf `/items/{id}` um; nicht sichtbar heisst 404 ohne Hinweis auf die Freigabe |
| Rolle | Zuschauer, die den Titel anfordern können; sonst nur, wenn ein eigener Wunsch besteht (F-9, `02-funktionskonzept.md` Abschnitt 4.5) |
| Neue oder veränderte Einstellungen | wirken hier: «Wünsche aktiv», «Filme erlauben», «Zurückziehen erlauben», «Bereits angeforderte Titel» (M-25). «Anfordern auf Detailseiten» wirkt hier nicht; die Seite bleibt erreichbar, solange eine der drei Stellen an ist (`07-administration.md` Abschnitt 7) |
| API-Operationen | `getExternalTitle` (mit `lastWish?`; 404 ohne Recht ausser bei eigenem Wunsch; 409 `already_available` mit `item`), `getExternalTitleImage`, `createWish`, `withdrawWish` |
| Mockup | M-03, `#/titles/movie/1368337` («Odyssey»), `js/viewer-titles.js` (`route('/titles/:media/:tmdbId')`, `vtPreviewVisible`, `vtStatusBlock`, `vtWishCard`) |
| Abgedeckte interaktive Zustände | ohne Wunsch: Hinweis «Noch nicht in der Bibliothek» und «Film anfordern» (öffnet M-13); eigener Wunsch: Block «Dein Wunsch» mit Chip, Verlauf (erledigt, laufend, ausstehend), «Zu meinen Wünschen», «Zurückziehen» bei `queued` und `pending`; Zusatz bei `queued` (etwa «Jellyseerr ist nicht erreichbar. Slouchcat versucht es weiter.»); Mitwunsch: «du wünschst mit» und «Nicht mehr mitwünschen» (Dialog); fremder Wunsch: Hinweis «Bereits angefordert» und «Benachrichtige mich», bei «Nur Zustand zeigen» ohne Knopf; letzter eigener Wunsch geschlossen: Hinweis «Dein letzter Wunsch: Abgelehnt/Fehlgeschlagen/Zurückgezogen» mit Datum und Grund (Mockup › Noah, `#/titles/movie/1153576`); ohne Recht mit eigenem Wunsch: Hinweis «Anfordern ist für dieses Profil nicht möglich», für Administratoren bei fehlender Zuordnung mit «Zuordnung öffnen»; ohne Recht und ohne eigenen Wunsch sowie über der Altersstufe: «Titel nicht gefunden» (Mockup › Noah, `#/titles/movie/1423191`); vorhanden: Umleitung ohne Verlaufseintrag (`#/titles/movie/940721` nach `#/items/godzilla-minus-one`); «Trailer» als Dialog; Besetzung blättern; «Zurück» |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.2 (`server/internal/metadata/`, Jobart `titles.refresh`, Bild-Handler), 2.3 (`visibility`: `MissingTitle`, `TitleByExternalIDs`, Regel F-9), 2.8 (`web/src/features/titles/`); Tabellen `external_titles`, `external_title_texts`, `external_title_images`; `02-funktionskonzept.md` Abschnitt 5.1, ADR 0038 |

### M-04 · Vorschauseite Serie (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/titles/show/{tmdbId}` |
| Heute | gibt es nicht |
| Geplante Änderung | wie M-03, dazu Staffelwand mit gedämpften Karten je Staffel und Zustand; Hinweis «Läuft noch · nächste Folge am …»; Hauptknopf «Serie anfordern»; Serie ohne TVDB-ID ohne Knopf mit Hinweis (AK-39) |
| Rolle | wie M-03 |
| Neue oder veränderte Einstellungen | zusätzlich «Serien erlauben» (M-25); der Jellyseerr-Benutzer muss Serien anfordern dürfen |
| API-Operationen | `getExternalTitle` (`seasons[] {number, episodeCount, airDate, state}`), `createWish` mit `seasons` (403 `tvdb_id_missing`), `withdrawWish` |
| Mockup | M-04, `#/titles/show/291507` («The Body»); weitere: `#/titles/show/225385` («Black Doves», Staffel 1 von Sandra angefordert), `#/titles/show/73223` («Black Clover», zwei Staffeln); `js/viewer-titles.js` (`vtGhost`, `vtSeasonState`) |
| Abgedeckte interaktive Zustände | Staffelkarten mit Band «Fehlt» (klickbar, öffnet M-13 mit genau dieser Staffel), eigener Zustand (führt zu `#/wishes`), «Bereits angefordert» (fremder Wunsch, auf der Karte nicht klickbar, im Dialog wählbar), «Angekündigt»; Hinweis «Läuft noch · nächste Folge am 19. November 2026»; als Marco bei «Black Doves»: Staffel 1 «Bereits angefordert», Staffel 2 «Fehlt»; alle Zustände von M-03 |
| Spätere technische Umsetzung | Stufe 2, Schritte 2.2, 2.3, 2.8 (Staffeln aus `external_titles.seasons`); `02-funktionskonzept.md` Abschnitt 6; TVDB-ID beim Senden Pflicht (`02-funktionskonzept.md` Abschnitt 6, AK-39) |

### M-05 · Detailseite Film (kleine Änderung)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/items/{id}` |
| Heute | Hintergrund, Poster mit Merken, «Abspielen», runde Knöpfe «Trailer», «Von vorne», «Merken», «Gesehen», «Empfehlen» (Besitzerkonto), «Löschen» (Administrator), Besetzung |
| Geplante Änderung | im Mockup: Die Metazeile zeigt 14 Tage lang den Chip «Dein Wunsch ist da», wenn ein eigener Wunsch erfüllt ist. Die Papiere führen die Seite noch als unverändert (Abschnitt 10.2) |
| Rolle | Zuschauer |
| Neue oder veränderte Einstellungen | keine |
| API-Operationen | `getItem` (für Filme unverändert; der Chip braucht den Wunsch des Profils, etwa über `listWishes`) |
| Mockup | M-05, `#/items/masterplan`; mit erfülltem Wunsch `#/items/godzilla-minus-one`; `js/viewer-titles.js` (`route('/items/:key')`), Chip in `js/home.js` (`fulfilledRecently`, `metaLine`) |
| Abgedeckte interaktive Zustände | «Abspielen» und «Von vorne» öffnen den Platzhalter des Players; «Trailer» als Dialog; «Merken» und «Gesehen» schalten um; «Empfehlen» mit Meldung «Bleibt 30 Tage»; «Löschen» öffnet den unveränderten Dialog und löscht nichts; Chip «Dein Wunsch ist da» bei Marco auf «Godzilla Minus One»; Titel über der Altersstufe gibt «Titel nicht gefunden» |
| Spätere technische Umsetzung | offen, solange die Papiere die Seite als unverändert führen; der Chip auf der Startseite ist AK-131 (Stufe 3, Schritt 3.4) |

### M-06 · Detailseite Serie mit fehlenden Staffeln (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/items/{id}` |
| Heute | Staffelwand der vorhandenen Staffeln, Folgenliste der gewählten Staffel mit Gesehen je Folge, runde Knöpfe wie M-05 |
| Geplante Änderung | fehlende Staffeln als gedämpfte Karten; runder Knopf «Staffeln anfordern»; Chip «2 von 3 Staffeln vorhanden»; Zeile mit fehlenden und angeforderten Staffeln und Link «Meine Wünsche»; Hinweis «Läuft noch, nächste Folge am …» oder «Staffel 4 ist angekündigt»; alles nur für Profile, die anfordern können |
| Rolle | Zuschauer; gedämpfte Karten, Zeile und Chip nur für Profile, die anfordern können, sonst wie heute |
| Neue oder veränderte Einstellungen | wirken hier: «Anfordern auf Detailseiten», «Serien erlauben», «Wünsche aktiv», «Bereits angeforderte Titel» (M-25) |
| API-Operationen | `getItem` (neu `catalog`, `canRequestSeasons`), `createWish` mit `seasons` |
| Mockup | M-06, `#/items/tulsa-king`; weitere `#/items/rings-of-power`, `#/items/dexter-resurrection`, `#/items/black-clover` (nach Anfordern und «Datei trifft ein»); `js/viewer-titles.js` (`route('/items/:key')`, `vtMissingLine`, `vtGhost`, `vtEpisodes`) |
| Abgedeckte interaktive Zustände | Staffel 1 und 2 vorhanden und wählbar (Folgenliste, Gesehen je Folge, Folge abspielen); Staffel 3 «Fehlt» (Klick öffnet M-13 mit Staffel 3), Staffel 4 «Angekündigt»; Zeile «Staffel 3 fehlt.»; nach dem Anfordern «Staffel 3 ist angefordert: In Beschaffung.» mit Link; Knopf «Staffeln anfordern» verschwindet, wenn keine Staffel frei ist oder «Auf Detailseiten» aus ist (dann sind die Karten auch nicht klickbar); Mockup › Gast oder «Oma & Opa» (ohne Zuordnung): nur vorhandene Staffeln, keine gedämpften Karten, keine Zeile, kein Chip; «Oma & Opa» nach «Übernehmen» in M-24 (grosseltern darf keine Serien): keine Karten, keine Zeile, aber der Chip bleibt (Abschnitt 9, Nr. 15); Mockup › Mia: «Titel nicht gefunden» (ab 16) |
| Spätere technische Umsetzung | Stufe 4: Schritte 4.1 (`server/internal/metadata/`, Tabelle `series_catalog`, Jobart `series.catalog`), 4.2 (`web/src/features/catalog/ItemPage.tsx`, `SeasonShelf.tsx`, Staffelwahl mit Mitwunsch); `02-funktionskonzept.md` Abschnitt 6 |

### M-07 · Suche (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/search` |
| Heute | Suchfeld (ab 2 Zeichen), Filter Genre, Jahr von/bis, Altersfreigabe höchstens, Medientyp, Verfügbarkeit «Auf dem Server»/«Auf diesem Gerät», Ergebnisse «Titel», «Personen», «Sammlungen» |
| Geplante Änderung | Abschnitt «Noch nicht in der Bibliothek» aus der TMDB-Suche; Verfügbarkeitsfilter «Noch nicht da»; kein Hinweis für Profile, die nicht anfordern können (auch nicht bei fehlender Zuordnung) |
| Rolle | Zuschauer; der neue Abschnitt nur für Profile, die anfordern können |
| Neue oder veränderte Einstellungen | wirken hier: «Anfordern aus der Suche», «Wünsche aktiv», «Filme erlauben», «Serien erlauben» (M-25) |
| API-Operationen | `search` (unverändert), `searchExternalTitles` (leer ohne Recht oder mit «Anfordern aus der Suche» aus), `getExternalTitleImage` |
| Mockup | M-07, `#/search?q=the`, `js/viewer-pages.js` (`route('/search')`) |
| Abgedeckte interaktive Zustände | Eingabe mit Live-Ergebnis; ein Zeichen: «Gib mindestens zwei Zeichen ein.»; Abschnitt «Titel» und darunter «Noch nicht in der Bibliothek» mit Plus-Knöpfen; Filter aufklappbar mit Zusammenfassung der aktiven Filter und «Filter zurücksetzen»; Pille «Noch nicht da» zeigt fehlende Titel auch ohne Suchbegriff; Personen und Sammlungen führen zur Hinweisseite «Unverändert»; «Nichts gefunden»; Mockup › Mia, Gast oder «Oma & Opa» (ohne Zuordnung): weder Abschnitt noch Pille noch Hinweis; «Oma & Opa» nach «Übernehmen» in M-24: nur fehlende Filme; «Aus der Suche» aus (M-25): Abschnitt und Pille fehlen |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.3 (Suche über TMDB gefiltert in `visibility`, Detailabruf der ersten 20 Treffer für Profile unter 18), 2.8 (`features/discover/SearchPage.tsx`); `02-funktionskonzept.md` Abschnitt 5.2 |

### M-08 · Meine Wünsche (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/wishes`; Eintrag «Wünsche» in der Seitenleiste nach «Gesehen» mit Zähler offener Wünsche, nur wenn das Profil anfordern kann oder eigene Wünsche hat |
| Heute | gibt es nicht |
| Geplante Änderung | Liste der eigenen Wünsche und Mitwünsche mit Zustand, Filter, Verlauf, Zurückziehen, Nicht mehr benachrichtigen, Erneut anfordern; für das Hauptprofil umschaltbar auf alle Profile des Kontos |
| Rolle | jedes Profil mit Wünschen; Hauptprofil zusätzlich alle Profile des Kontos |
| Neue oder veränderte Einstellungen | wirken hier: «Zurückziehen erlauben», «Bereits angeforderte Titel» (M-25) |
| API-Operationen | `listWishes`, `listAccountWishes` (`scoped`, zusätzlich Hauptprofil, sonst 403 `primary_profile_required`), `withdrawWish`, `createWish` (über «Erneut anfordern») |
| Mockup | M-08, `#/wishes`, Sprung aus M-09 mit `#/wishes?focus={id}`; `js/viewer-pages.js` (`route('/wishes')`, `vpWishRow`, `vpSteps`); Seitenleiste `js/shell.js` |
| Abgedeckte interaktive Zustände | Filter «Alle», «Offen», «Verfügbar», «Abgeschlossen» mit Zählern; Schalter «Wünsche aller Profile meines Kontos» (nur Hauptprofil, zeigt den Profilnamen); je Zeile Chip, «Du wünschst mit», «In Jellyseerr angelegt», Klartext bei `queued`, `failed` (auch «Jellyseerr war 24 Stunden lang nicht erreichbar.»), `pending`, Staffel-Chips bei Serien; «Abspielen» und «Zum Titel» bei `available` und `partial`; «Zurückziehen» mit Rückfrage (Besitzer, `queued` und `pending`); «Nicht mehr benachrichtigen» (Mitwunsch); «Erneut anfordern» bei `declined`, `failed`, `withdrawn` (öffnet M-13); «Verlauf» aufklappbar; `?focus=` klappt den Verlauf auf und scrollt hin; leer mit Recht: «Noch keine Wünsche» mit «Zum Spotlight» und «Zur Suche»; leer ohne Recht: Hinweis mit Grund (Mockup › Mia); mit Wünschen, aber ohne Recht: Hinweis «Neue Wünsche sind nicht möglich»; Rückgabe an Mitwünscher beim Zurückziehen («… übernimmt») |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.4 (`server/internal/wishes/`: `createWish`, `withdrawWish` mit Job `wishes.withdraw`, Ausgangsfach), 2.9 (`web/src/features/wishes/`, `AppLayout.tsx`); Tabellen `wishes`, `wish_seasons`, `wish_requesters`, `wish_events`; `02-funktionskonzept.md` Abschnitte 4.8 und 5.3 |

### M-09 · Benachrichtigungen (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/notifications`; Glocke im Kopf mit Zähler |
| Heute | Eingang des Profils mit «Neue Folge» je Serie der Merkliste, gelesen/ungelesen, «Alle als gelesen markieren», «Kanäle einstellen» |
| Geplante Änderung | neue Eingangsart `wish_available` (Stufe 2); `wish_season_available`, `wish_approved`, `wish_declined`, `wish_failed` (Stufe 4); Link zum Titel oder zu `/wishes?focus={id}` |
| Rolle | jedes Profil; nur das anfordernde Profil und Mitwünscher, je erst wenn der Titel für dieses Profil sichtbar ist |
| Neue oder veränderte Einstellungen | keine; der Eingang ist immer an |
| API-Operationen | `listNotifications` (brechend: `item` wird optional, neu `wish`), bestehende Operationen zum Lesen |
| Mockup | M-09, `#/notifications`, `js/viewer-pages.js` (`route('/notifications')`), Texte `N_TEXT` in `js/store.js` |
| Abgedeckte interaktive Zustände | ungelesen hervorgehoben und im Zähler der Glocke; Klick markiert gelesen und springt zum Titel bzw. zu M-08 mit `focus`; «Als gelesen markieren»/«Als ungelesen markieren»; «Alle als gelesen markieren»; leer: «Keine Benachrichtigungen» (Mockup › Mia); neue Meldungen entstehen beim Vorspulen (Freigeben, Ablehnen, Scheitern, 24 h vergehen, Datei trifft ein) mit Meldung am Bildschirm, wer über welche Kanäle benachrichtigt wurde. Alle Arten erscheinen ohne Stufenhinweis (Abschnitt 9) |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.6 (`server/internal/notify/`, Schlüssel `wish:<id>:available`, `notified_at`), 2.9 (`NotificationsPage.tsx`); Stufe 4: Schritt 4.3; Tabelle `notifications` (Neubau mit `wish_id`, `dedupe_key`); `06-benachrichtigungen.md` Abschnitte 2, 3, 7 und 10 |

### M-10 · Benachrichtigungskanäle (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/settings/notifications` |
| Heute | je Kanal (E-Mail, Telegram, Web-Push) ein Band mit Zustand, Schalter «Aktiv», Ereignissen «Neue Inhalte», «Fehler» (nur Administratoren), «Einladungen», «Ankündigungen», Adresse bzw. Verknüpfung bzw. Geräte, «Speichern», «Testnachricht»; ändern nur im Hauptprofil |
| Geplante Änderung | drei neue Häkchen je Kanal: «Wünsche: verfügbar» (Stufe 2), «Wünsche: freigegeben, abgelehnt, fehlgeschlagen» (Stufe 4, ab Werk aus), nur Administratoren «Wünsche: Freigabe nötig» (Stufe 4); Hinweis «Wünsche und Profile» zur PIN-Regel; Kopf zählt die Ereignisse («5 Ereignisse», `07-administration.md` Abschnitt 11) |
| Rolle | Hauptprofil (ändern), weitere Profile (lesen) |
| Neue oder veränderte Einstellungen | Ereignisse `wish_available`, `wish_status`, `wish_review` je Kanal |
| API-Operationen | bestehende Kanal-Operationen; Ereignisliste erweitert (`EventType`, brechend), `wish_review` für andere Konten abgelehnt (`ErrEventNotAllowed`) |
| Mockup | M-10, `#/settings/notifications`, `js/viewer-pages.js` (`channelSettingsHtml`, `VP_EVENTS`) |
| Abgedeckte interaktive Zustände | Bänder zugeklappt mit Zusammenfassung (Adresse, Geräte oder Verknüpfung und Zustand, ohne Zahl der Ereignisse); Häkchen wirken sofort auf spätere Zustellungen; Mockup › «Oma & Opa»: ohne «Fehler» und «Wünsche: Freigabe nötig»; Mockup › Sandra: alles gesperrt mit Hinweis «Nur im Hauptprofil» und «Profil wechseln»; «Testnachricht» mit Warnung, wenn der Kanal aus, nicht verknüpft oder ohne Gerät ist; Telegram «Verknüpfen» mit Code, «Verknüpfung prüfen», «Code erneuern», «Code verwerfen», «Verknüpfung lösen»; Web-Push «Auf diesem Gerät aktivieren» und «Entfernen» |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.6 (Rückfüllung `wish_available`, F-14), 2.9; Stufe 4: Schritt 4.3 (`wish_status` ab Werk aus, `wish_review`, Datenmigration für bestehende Administratorkonten); Tabelle `account_channels` (Feld `events`), `deliveries` (Neubau); `06-benachrichtigungen.md` Abschnitte 5 und 6 |

### M-11 · Profile (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/settings/profiles` |
| Heute | Karten je Profil mit Chips, «Bearbeiten», «Schauzeit anzeigen», Menü «PIN setzen/ändern/entfernen», «Löschen»; Profildialog mit Name, Altersfreigabe, Sprache, Sprache der Beschreibungen, Farbe, Bibliotheken, Wiedergabe, Wiedergabe-Limits, PIN beim Anlegen; nur Hauptprofil |
| Geplante Änderung | Profildialog: Abschnitt «Wünsche» mit Schalter «Darf Titel anfordern», gesperrt mit Hinweis, wenn das Konto nicht darf; Karte: Chip «Darf anfordern»/«Darf nicht anfordern» und Zeile «Sendet als {Benutzer} (…)» |
| Rolle | Hauptprofil des Kontos |
| Neue oder veränderte Einstellungen | Wunschrecht des Profils (`profiles.can_request`) |
| API-Operationen | `createProfile`, `updateProfile`, `listProfiles` (Feld `canRequest`) |
| Mockup | M-11, `#/settings/profiles`, `js/viewer-pages.js` (`vpProfCard`, `DIALOGS.vpProf`) |
| Abgedeckte interaktive Zustände | Schalter an/aus und speichern; neues Profil startet mit «aus»; Mockup › Gast: Schalter gesperrt mit «Das Konto darf nicht anfordern. Das stellt der Administrator ein.»; Mockup › Noah: Hinweis «Nur im Hauptprofil»; Zeile «Sendet als» zeigt eigener Benutzer, Benutzer des Kontos, Sammelbenutzer oder «Keine Zuordnung»; alle heutigen Teile bedienbar (Pflichtfeld Name, PIN 4 bis 8 Ziffern, Schauzeit-Diagramm, Löschen mit Rückfrage) |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.2 (Migration `wishes_rights`), 1.6 (`server/internal/auth/`, Deckelregel), 1.9 (`web/src/features/profiles/`); `04-benutzer-profilmapping.md` Abschnitt 9 |

### M-12 · Profilwahl (unverändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/profiles` |
| Heute | «Wer schaut?» mit Profilkacheln, PIN-Abfrage, «Profile verwalten» |
| Geplante Änderung | keine |
| Rolle | Konto ohne gewähltes Profil |
| Neue oder veränderte Einstellungen | keine |
| API-Operationen | Sitzung (erweitert um `wishes` als `WishRights {enabled, canRequest, reason?, movies, shows}` beim Wählen) |
| Mockup | M-12, `#/profiles`, `js/viewer-pages.js` (`route('/profiles')`) |
| Abgedeckte interaktive Zustände | Profil wählen; PIN-Dialog (im Mockup öffnet jede vierstellige PIN); Abschnitt «Andere Konten (nur im Mockup)» zum direkten Kontowechsel |
| Spätere technische Umsetzung | Stufe 1, Schritt 1.6 (Sitzung `wishes`) |

### M-13 · Dialog «Anfordern» (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, Dialog auf M-01, M-02, M-03, M-04, M-06, M-07, M-08 und M-27 |
| Heute | gibt es nicht |
| Geplante Änderung | ein Dialog: Poster, Titel, «Geht als {Benutzer} an Jellyseerr», «Wird sofort beschafft» oder «Braucht eine Freigabe», Kontingent, Kanäle der Meldung, bei Serien die Staffelwahl mit Mitwunsch je Staffel; «Anfordern»/«Abbrechen» |
| Rolle | Profil, das anfordern kann; in M-27 der Administrator mit seinem Hauptprofil |
| Neue oder veränderte Einstellungen | wirken hier: «Bereits angeforderte Titel», «Konten ohne Zuordnung» mit Sammelbenutzer (M-25); Zuordnung (M-24); Kanal-Ereignis «Wünsche: verfügbar» (M-10) |
| API-Operationen | `createWish` (201 neu, auch mit `joined` bei Serien; 200 bestehend oder Mitwunsch; 403 `wish_not_allowed` mit `reason`, etwa `status_only`, `tvdb_id_missing`; 404; 409 `already_available`) |
| Mockup | M-13, kein eigener Hash; `js/home.js` (`ACT.askWish`, `DIALOGS.wish`, `ACT.confirmWish`), `ACT.vtAskSeason` in `js/viewer-titles.js`, `requestWish` in `js/store.js` |
| Abgedeckte interaktive Zustände | Film neu: Fakten und «Anfordern»; Film schon angefordert: «Schon angefordert» mit «Benachrichtige mich» (bei «Nur Zustand zeigen» gibt es keinen Knopf, der dorthin führt); Serie: «Alle fehlenden Staffeln (n)» und einzelne Staffeln, Zeilen «Vorhanden», «Angekündigt», «Bereits angefordert» (fremd, wählbar) oder eigener Zustand; Knopf «n Staffeln anfordern», «Benachrichtige mich» (nur fremd angeforderte gewählt) oder gesperrt «Staffeln wählen»; beim Bestätigen hängt sich das Profil an die fremden Wünsche und bestellt nur den Rest neu (Marco bei «Black Doves»: Staffel 1 Mitwunsch, Staffel 2 neu); «Nur Zustand zeigen»: fremd angeforderte Staffeln gesperrt mit ihrem Zustand; «Nichts mehr anzufordern»; Jellyseerr nicht erreichbar: Hinweis «Dein Wunsch wird vorgemerkt» (Mockup › «Nicht erreichbar»); Kontingent: Mockup › Noah zeigt «5 Titel in 7 Tagen, 2 verbraucht»; der vierte neue Wunsch von Noah endet als «Fehlgeschlagen» mit Klartext; kein Recht: «Anfordern nicht möglich» mit Grund. Diesen Fall erreicht man nach Regel F-9 nur über einen eigenen Wunsch (Recht entziehen, dann «Erneut anfordern» in M-08, Mockup › Noah, «Street Fighter»); so steht es auch in `02-funktionskonzept.md` Abschnitt 4.5. Rückmeldung am Bildschirm je Ergebnis mit Link «Meine Wünsche» |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.4 (`createWish`, Mitwunsch auch je Staffel, Sofortversuch 3 s, `wishes.submit`), 2.8 (`web/src/ui/`); Stufe 4: Schritt 4.2 (Staffelwahl auf der Serienseite); `02-funktionskonzept.md` Abschnitte 4.1 bis 4.5; `03-architektur.md` Abschnitte 6.1 bis 6.5; Storybook für Dialog, Plus-Knopf, Zustandschip |

### M-20 · Verwaltung › Übersicht (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin` |
| Heute | «Alles im Blick», «Aktualisieren», Kennzahlen «Aktive Streams», «Laufende Jobs», «Titel zur Pflege», «Speicherplatz»; «Was benötigt Aufmerksamkeit?»; «Gerade aktiv»; «Direkt zu deinen Aufgaben»; «Server & letzte Aktivitäten» |
| Geplante Änderung | fünfte Kennzahl «Offene Wünsche» (nur wenn Jellyseerr eingerichtet ist); Hinweise zu Wünschen und Jellyseerr, darunter «Andere Jellyseerr-Instanz erkannt»; Schnellzugriff «Wünsche»; Zeile «Jellyseerr» im Serverzustand; Wunsch-Ereignisse in «Letzte Aktivität» |
| Rolle | Administrator mit Hauptprofil |
| Neue oder veränderte Einstellungen | keine |
| API-Operationen | `getDashboard` (Kennzahl, Hinweise, Integrationsstatus `seerr`), `getSeerrStatus` |
| Mockup | M-20, `#/admin`, `js/admin-a.js` (`route('/admin')`, `aaHints`) |
| Abgedeckte interaktive Zustände | Kennzahl mit «n warten auf Freigabe», «–», wenn Jellyseerr nicht verbunden und kein Wunsch offen ist; Hinweise «Jellyseerr nicht erreichbar», «Jellyseerr lehnt den Schlüssel ab», «Jellyseerr blockiert schreibende Aufrufe» (CSRF), «n Konten ohne Zuordnung», «Benutzer in Jellyseerr gelöscht», «n Wünsche warten auf Freigabe», «n Wünsche fehlgeschlagen» (auch nach «24 h vergehen»), «n Wünsche warten auf Übermittlung», je «Ansehen» mit vorgewähltem Filter in M-23; «Alles in Ordnung.»; Mockup › «Nicht eingerichtet» blendet Kennzahl und Schnellzugriff aus; Mockup › Sandra zeigt «Nur für Administratoren» |
| Spätere technische Umsetzung | Stufe 1 und 2: Schritt 2.10 (`web/src/features/admin/`); `07-administration.md` Abschnitt 2 |

### M-21 · Verwaltung › Integrationen (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/integrations` |
| Heute | «n von m eingerichtet» und sechs Kacheln: Schlüssel der Anbieter, Webhooks von Radarr und Sonarr, E-Mail-Server, Telegram-Bot, Web-Push, Server |
| Geplante Änderung | siebte Kachel «Jellyseerr» nach «Webhooks von Radarr und Sonarr» mit Meta «Zuordnung: n von m Konten · n offene Wünsche» und Zustand; die Zeile oben zählt neun Statusschlüssel (TMDB, TVDB, YouTube, Trakt, E-Mail, Telegram, Web-Push, Update-Prüfung, Jellyseerr) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | keine auf der Kachel (alle in M-22) |
| API-Operationen | `getSeerrStatus` neben den bestehenden Operationen |
| Mockup | M-21, `#/admin/integrations`, `js/admin-a.js` (`aaIntegrationsPage`, `aaTile`, `aaIntCounts`); Zähler im Bereichsreiter `js/shell.js` (`areaTabs`) |
| Abgedeckte interaktive Zustände | Kachel mit Punkt und «Verbunden», «Nicht erreichbar» (mit «zuletzt erreicht vor …»), «Schlüssel ungültig», «CSRF-Schutz blockiert», «Nicht eingerichtet», «Ausgeschaltet»; Zeile oben «n von 9 eingerichtet»; der Bereichsreiter «Integrationen» trägt denselben Wert «n / 9», beide aus `aaIntCounts`; Chip «ungespeichert» bei offener Eingabe in M-22; alle sechs bestehenden Ansichten bedienbar (`#/admin/integrations/keys`, `/webhooks`, `/email`, `/telegram`, `/push`, `/server`) |
| Spätere technische Umsetzung | Stufe 1, Schritt 1.7 (`web/src/features/admin/integrations/`); `07-administration.md` Abschnitt 3 |

### M-22 · Integrationen › Jellyseerr (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/admin/integrations?item=jellyseerr` (im Mockup `#/admin/integrations/jellyseerr`) |
| Heute | gibt es nicht |
| Geplante Änderung | vier Bänder: «Verbindung» (Schalter, Adresse, API-Schlüssel, Test mit Prüfliste einschliesslich Instanz, Speichern, Verwerfen, Löschen), «Abgleich» (letzter, nächster, Takt, Zähler, «Jetzt abgleichen»), «Sofortmeldung» (Stufe 4), «Protokoll» (20 Einträge); Verweise auf Zuordnung, Einstellungen, Anfragen |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | «Jellyseerr verwenden» (ab Werk aus), «Adresse», «API-Schlüssel»; Adresse der Sofortmeldung |
| API-Operationen | `getSeerrSettings`, `updateSeerrSettings`, `testSeerrConnection` (neuer Host verlangt den Schlüssel), `getSeerrStatus`, `syncWishesNow`, `confirmSeerrInstance`; Stufe 4 `createIntegration`, `receiveWebhook` mit Art `jellyseerr` |
| Mockup | M-22, `#/admin/integrations/jellyseerr`, `js/admin-a.js` (`aaSeerrView`, `aaSeerrRun`) |
| Abgedeckte interaktive Zustände | Band «Verbindung» klappt bei Fehler und bei CSRF von selbst auf; «Verbindung testen» mit Ladeanzeige und Prüfliste (sieben Punkte: Erreichbar, Schlüssel gültig, Version, Handeln im Namen von Benutzern, CSRF-Schutz, Radarr, Sonarr); «falsch» in der Adresse ergibt «Nicht erreichbar», im Schlüssel «Schlüssel ungültig»; Mockup › «CSRF-Schutz an»: Prüfpunkt CSRF als Warnung, Ergebnis «Verbunden, mit einer Warnung.»; Pflicht- und Formfehler beim Speichern («Gib die Adresse von Jellyseerr ein.», «Ohne /api/v1 …»); ohne Einrichtung die Schrittanleitung und gesperrter Schalter; Schalter aus ergibt «Ausgeschaltet»; «Verbindung löschen …» mit Rückfrage; Abgleich mit Fehlerzeile «Letzter Versuch …»; Zähler als Links mit Filter in M-23; «Zeitplan ändern» springt zu M-30 auf `wishes.sync`; Sofortmeldung «Adresse erzeugen» (einmal gezeigt, kopieren, JSON-Vorlage), «Token erneuern», «Entfernen», «Meldung simulieren (Mockup)»; Protokoll mit Fehlern |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.1 (`server/internal/seerr/`), 1.3 (`settings` `seerr.*`, Redaction `seerr.api_key`), 1.5 (Verbindungstest, Fingerabdruck der Instanz), 1.7; Stufe 2: Schritte 2.5, 2.10 (Band «Abgleich»); Stufe 4: Schritt 4.5 (`webhook_integrations`); Tabelle `seerr_log`; `07-administration.md` Abschnitt 4, `03-architektur.md` Abschnitte 2.2, 7.4 und 15.1 |

### M-23 · Medien › Wünsche › Anfragen (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/admin/wishes`; Bereichsreiter «Wünsche» im Bereich Medien direkt nach «Pflege» mit Zähler «wartet auf Freigabe» |
| Heute | gibt es nicht |
| Geplante Änderung | Kennzahlen als Filter, Filter nach Zustand, «Nicht zuordenbar» und Medienart, Tabelle (Titel, Art, Gewünscht von, Jellyseerr-Benutzer, Zustand, Seit), Aufklappzeile mit Verlauf und Diagnose, Aktionen «Freigeben», «Ablehnen» (Stufe 4), «Erneut übermitteln», «In Jellyseerr öffnen», «Zum Titel», «Jetzt abgleichen» |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | keine (Reiter «Einstellungen» ist M-25) |
| API-Operationen | `listAdminWishes` (`AdminWish` ohne `ItemRef`), `getAdminExternalTitleImage`, `retryWish`, `syncWishesNow`; Stufe 4 `approveWish`, `declineWish` |
| Mockup | M-23, `#/admin/wishes`, `js/admin-b.js` (`route('/admin/wishes')`, `abWishRow`, `wishTabs`); Bereichsreiter `js/shell.js` (`AREAS`, `areaTabs`) |
| Abgedeckte interaktive Zustände | Bereichsreiter «Wünsche» nach «Pflege» mit Zahl der wartenden Freigaben (sonst «NEU»); Reiter «Anfragen», «Zuordnung», «Einstellungen»; fünf Kennzahlen setzen den Filter; Filterpillen für alle acht Zustände (leere ausgeblendet) und «Nicht zuordenbar»; leere Auswahl mit «Alle Wünsche zeigen»; Aufklappzeile mit Verlauf und Diagnose (Request-ID, Übermittelt als mit `X-API-User`, TMDB-ID, Herkunft, Zustand, Fehlergrund, Zusatz, Mitwünscher); «Clayface» als «Nicht zuordenbar» mit Hinweis und «Zuordnung öffnen»; «Freigeben» und «Ablehnen» (Rückfrage nennt die Profile) bei «Toy Story 5»; Jellyseerr nicht verbunden oder CSRF: Band oben und Meldung «Geht nicht: Jellyseerr ist …»; «Erneut übermitteln» bei `queued` und `failed`; «In Jellyseerr öffnen» als Dialog mit der Nummer; Wünsche aus: Band mit Link zu M-25 |
| Spätere technische Umsetzung | Stufe 1: Schritt 1.8 (Reiter, «Anfragen» zuerst nur als Hinweis); Stufe 2: Schritt 2.10 (`web/src/features/admin/`), Index `wishes_admin`; Stufe 4: Schritt 4.4 (`approveWish`, `declineWish`); `07-administration.md` Abschnitte 1.2 und 5 |

### M-24 · Medien › Wünsche › Zuordnung (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/admin/wishes/mapping` |
| Heute | gibt es nicht |
| Geplante Änderung | je Konto ein Auswahlfeld des Jellyseerr-Benutzers direkt in der Liste, sofort wirksam, Vorschlag bei gleicher E-Mail mit «Übernehmen», Fakten und Warnungen des Benutzers, aufklappbar die Profile mit «Wie Konto» oder eigenem Benutzer; Kopfzeile mit «Benutzerliste neu laden», «Alle Vorschläge übernehmen» und ab Stufe 4 «Anfragen übernehmen»; Liste der Jellyseerr-Benutzer ohne Konto; Meldung nennt Folgen einer Änderung |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | Zuordnung je Konto und je Profil |
| API-Operationen | `listSeerrUsers`, `refreshSeerrUsers`, `getWishMappings`, `setAccountWishMapping`, `setProfileWishMapping` (409 `seerr_user_taken`), `applyWishMappingSuggestions` (Vorschlag); Stufe 4 `importSeerrRequests`, `countSeerrRequests` |
| Mockup | M-24, `#/admin/wishes/mapping`, `js/admin-b.js` (`abMapCard`, `abProfMap`, `abUserFacts`, `abUserWarn`) |
| Abgedeckte interaktive Zustände | Auswahl ändern wirkt sofort mit Meldung «Zuordnung geändert: …» und Chip «Zuordnung geändert»; belegte Benutzer gesperrt mit «(belegt von …)»; gelöschte gesperrt mit «(in Jellyseerr gelöscht)»; Chip «Zuordnung fehlt»; Vorschlag «grosseltern» mit «Übernehmen»; «Alle Vorschläge übernehmen» mit Rückfrage und Liste der Paare; Profilzeilen «Profil zugeordnet: …», «Profil nicht zugeordnet · geht als …», «… kein Benutzer»; Wunschrecht des Profils nur lesend mit «Im Profil ändern»; Warnungen «darf keine Serien anfordern» (grosseltern), «Benutzer … nicht gefunden»; Hinweis für Konten ohne Recht mit «Zu den Konten»; «Benutzerliste neu laden» (scheitert ohne Verbindung mit «Gezeigt wird der letzte Stand»); Mockup-Hilfe «Benutzer „sandra“ in Jellyseerr löschen (Mockup)» und zurück |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.2 (Migration `wishes_mapping`), 1.5 (`server/internal/wishes/` `Mapping`, Jobart `seerr.users`), 1.8 (`web/src/features/admin/wishes/`); Stufe 4: Schritt 4.6 (`wishes.import`); Tabellen `seerr_users`, `wish_account_map`, `wish_profile_map`; `04-benutzer-profilmapping.md` Abschnitte 6 bis 8, `07-administration.md` Abschnitt 6 |

### M-25 · Medien › Wünsche › Einstellungen (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/admin/wishes/settings` |
| Heute | gibt es nicht |
| Geplante Änderung | Karten «Grundsatz», «Woher angefordert werden darf», «Regeln», «Verweise»; jede Änderung gilt sofort, ohne «Speichern» und «Verwerfen» |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | alle `wishes.*` aus `08-datenmodell-api.md` Abschnitt 2.12 (Abschnitt 4 unten); «Wünsche aktiv» ab Werk aus, einschaltbar nur nach gelungenem Test (**Vorschlag** T-1) |
| API-Operationen | `getWishSettings`, `setWishSettings` (je Änderung) |
| Mockup | M-25, `#/admin/wishes/settings`, `js/admin-b.js` (`route('/admin/wishes/settings')`, `ACT.abSaved`) |
| Abgedeckte interaktive Zustände | jeder Schalter mit Meldung «… ist eingeschaltet. Gilt sofort.»; «Wünsche aktiv» aus sperrt alle übrigen Zeilen mit Hinweis; «Bereits angeforderte Titel» mit Hilfetext je Wahl («Mitwünschen» / «Nur Zustand zeigen»); «Konten ohne Zuordnung» auf «Sammelbenutzer» zeigt die Auswahl, ohne Wahl Chip «Ohne Benutzer gilt «Nicht erlauben»»; Verweise mit Zustand: Spotlight («An · höchstens 5» oder «Aus»), Verbindung, Zuordnung «n von m Konten». «Wünsche aktiv» lässt sich ohne gelungenen Test einschalten (Abschnitt 9) |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.3 (`server/internal/admin/settings.go`), 1.8; Tabelle `settings` (`wishes.*`); `07-administration.md` Abschnitte 4.6 und 7 |

### M-26 · Nutzer & Geräte › Konten (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/accounts` |
| Heute | Zusammenfassung, Reiter «Konten» und «Einladungen», Tabelle mit E-Mail, Rolle, Altersstufe, Bibliotheken, Profile, Zuletzt gesehen, Kanäle, Aktionen; eigenes Konto «Konto ändern» und «Meine Profile»; anderes Konto Formular «Zugriff»; «Konto einladen» |
| Geplante Änderung | Spalte «Wünsche» (Chip); im Formular «Zugriff» Schalter «Darf Titel anfordern», Zeile «Jellyseerr-Benutzer: … · Zuordnung ändern», Chips der Profile; Administratorkonten mit eigenem Block «Wünsche» (Entscheid vom 17. November 2026); Dialog «Konto einladen» mit Schalter; Spalte «Wünsche» bei Einladungen; Satz «Wünsche bleiben in Jellyseerr bestehen» beim Entfernen |
| Rolle | Administrator; Besitzer zusätzlich Rollenwechsel |
| Neue oder veränderte Einstellungen | Wunschrecht des Kontos (`accounts.can_request`), Wunschrecht der Einladung (`invitations.can_request`) |
| API-Operationen | `listAccounts`, `updateAccountAccess` (`canRequest`, `seerrUser`, `wishState`), `createInvitation`, `listInvitations` |
| Mockup | M-26, `#/admin/accounts`, `js/admin-b.js` (`abAccRow`, `abAccDetail`, `abAccessForm`, `abWishBlock`, `DIALOGS.abInvite`) |
| Abgedeckte interaktive Zustände | Chips «Erlaubt · marco», «Erlaubt · {Benutzer} (Sammelbenutzer)», «Zuordnung fehlt», «Benutzer gelöscht», «Nicht erlaubt»; Formular «Zugriff» bei grosseltern und gast mit Schalter, «Ungespeicherte Änderungen», «Verwerfen», «Zugriff speichern»; eigenes Administratorkonto mit Block «Wünsche» (gilt sofort) und «Meine Profile» mit Chips; Einladen mit Vorgabe aus «Neue Konten dürfen anfordern», danach Einmal-Link; Reiter «Einladungen» mit Spalte «Wünsche», «Widerrufen»; Menü «Zum Administrator ernennen», «Rolle entziehen», «Konto entfernen» (macht Wünsche nicht zuordenbar) |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.2, 1.6, 1.9 (`web/src/features/admin/accounts/`); `07-administration.md` Abschnitt 8; Wünsche eines entfernten Kontos werden geschlossen (`04-benutzer-profilmapping.md` Abschnitt 10) |

### M-27 · Startseite › Spotlight (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/spotlight` |
| Heute | Band «Quelle» mit elf Feldern («Auf der Startseite zeigen», «Trakt-Liste», «Adresse der Liste», «Medienart», «Höchstens», «Ausblenden unter», «Abruf alle … Stunden», «Spotlight Wechsel Takt», Abrufzustand, «Jetzt abrufen», «Speichern»); Band «Einträge der Liste» mit «Anheften», «Ausblenden», «Notiz»; Band «Verwaiste Anpassungen» |
| Geplante Änderung | Hinweis im Kopf; «Ausblenden unter» mit «Fehlende Titel zählen nicht.»; neues Band «Fehlende Titel» (gespeichert zusammen mit «Quelle»); Einträge: fehlende mit Poster, Zustand und «Anfordern» |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | «Fehlende Titel zeigen» (ab Werk aus, F-8), «Höchstens fehlende Titel» (0 bis 10, Vorgabe 5), «Reihenfolge» («Wie bei Trakt»/«Nach den vorhandenen») |
| API-Operationen | `getSpotlightConfig`, `setSpotlightConfig` (`showMissing`, `maxMissing`, `missingOrder`, Zähler), `getSpotlightEntries` (Poster über `getAdminExternalTitleImage`), `setSpotlightEntryOverride`, `createWish` des Hauptprofils |
| Mockup | M-27, `#/admin/spotlight`, `js/admin-b.js` (`abSourceBand`, `abMissingBand`, `abEntriesBand`, `ACT.abSpotSave`) |
| Abgedeckte interaktive Zustände | Band «Fehlende Titel» mit Schalter, Zahl und Reihenfolge (gesperrt, wenn aus), Zähler «n Einträge der Liste fehlen …, n davon sind angefordert. Marco sähe zurzeit n fehlende Titel» mit Grund; Warnungen bei «Wünsche aktiv» aus, «Aus dem Spotlight» aus («… zeigt das Spotlight keine fehlenden Titel»), Jellyseerr nicht eingerichtet; «Ungespeicherte Änderungen»; ein «Speichern» sichert beide Bänder; Prüfung beim Speichern («Erlaubt sind 0 bis 10.»); Einträge mit Status «Sichtbar», «Ausgeblendet», «Noch nicht in der Bibliothek», «Angefordert · In Beschaffung»; «Anheften» mit Platzwahl und Belegungsprüfung, «Notiz» (erscheint auf der Bühne), «Ausblenden»/«Einblenden», «Anfordern» (öffnet M-13 als aktives Profil); «Jetzt abrufen» mit Ladeanzeige; «Entfernen» bei verwaisten Anpassungen |
| Spätere technische Umsetzung | Stufe 3: Schritte 3.1 (Trakt-IDs, Migration `spotlight_missing`), 3.5 (`web/src/features/admin/SpotlightPage.tsx`); Tabellen `spotlight_config`, `spotlight_entries`; `05-spotlight-integration.md` Abschnitt 8, `07-administration.md` Abschnitt 9 |

### M-28 · Startseite › Startseite (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/home-rows` |
| Heute | Band «Reihen» mit Platz, Reihe, Zustand, Ausblenden, Ausklappzeile; Grenze 15 sichtbare Reihen; Band «Vorschau»; Speicherleiste |
| Geplante Änderung | neue Reihe «Deine Wünsche» mit Platz und Ausblenden, ab Werk direkt nach «Weiterschauen» (bei schon 15 sichtbaren Reihen ausgeblendet, **Vorschlag**); Ausklappzeile mit Link zu M-23 |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | Platz und Ausblenden der Reihe `wishes` |
| API-Operationen | `getHomeRows`, `setHomeRows` (`HomeRowKey` + `wishes`, brechend) |
| Mockup | M-28, `#/admin/home-rows`, `js/admin-b.js` (`route('/admin/home-rows')`, `AB_ROW_DESC.wishes`); Vorgabe in `seed()` von `js/store.js` |
| Abgedeckte interaktive Zustände | Reihe «Deine Wünsche» auf Platz 3 nach «Weiterschauen»; Platz per Auswahl ändern (Meldung «Speichere, damit es gilt»); «Ausblenden»; Grenze 15 mit Warnung; Vorschau in ungespeicherter Reihenfolge; Speicherleiste mit «Verwerfen» (Rückfrage) und «Speichern»; Wirkung auf M-01 nach dem Speichern |
| Spätere technische Umsetzung | Stufe 3: Schritte 3.3, 3.5 (`HomeRowsPage.tsx`); Tabelle `home_rows` (Neubau, CHECK + `wishes`); `07-administration.md` Abschnitt 10 |

### M-29 · Automatisierung › Benachrichtigungen (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route der Kanalseite in der Verwaltung |
| Heute | Kanalseite des eigenen Kontos in der Verwaltungsleiste, Link zu den Kanälen des Servers |
| Geplante Änderung | drei neue Ereignisse je Kanal wie M-10; Kopf zählt die Ereignisse; Karte «Was neu ist»; im Mockup zusätzlich Tabelle «Zuletzt zugestellt» (geht über `07-administration.md` Abschnitt 11 hinaus) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | wie M-10 (`wish_available`, `wish_status`, `wish_review`) |
| API-Operationen | bestehende Kanal-Operationen; Zustellungen aus `deliveries` |
| Mockup | M-29, `#/admin/notifications`, `js/admin-a.js` (`route('/admin/notifications')`), Kanalteil aus `channelSettingsHtml({ admin: true })` |
| Abgedeckte interaktive Zustände | Karte «Kanäle des Servers» mit Zustand und «Ansehen»; alle Zustände von M-10; Tabelle «Zuletzt zugestellt» (Zeit, Kanal, Empfänger mit Profil, Betreff, Zustand) füllt sich beim Vorspulen, auch mit «Wunsch nicht übermittelt: …» (Ereignis «Fehler») nach «24 h vergehen»; Sandra mit PIN: Betreff ohne Titel («[Sandra] Ein Wunsch ist jetzt verfügbar.») |
| Spätere technische Umsetzung | Stufe 2: Schritt 2.6; Stufe 4: Schritt 4.3; Tabellen `account_channels`, `deliveries`; `07-administration.md` Abschnitt 11, `06-benachrichtigungen.md` Abschnitt 8 |

### M-30 · Automatisierung › Hintergrundjobs (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/jobs` |
| Heute | Zusammenfassung, Arbeiter je Pool, Banner pausierter Jobs, «Läuft gerade», Reiter je Gruppe mit Filter, je Jobart Zustand, Letzter und Nächster Lauf, «Jetzt ausführen», «Pausieren», «Zeitplan», «Läufe», «Wartende abbrechen»; Reiter «Fehlschläge» |
| Geplante Änderung | acht neue Jobarten: `wishes.submit`, `wishes.withdraw`, `wishes.sync` (mit Erstabgleich als Lauf mit Fortschritt), `wishes.import` (Stufe 4), `wishes.reconcile`, `seerr.users`, `titles.refresh`, `series.catalog`; Fehlschläge von Wünschen im Reiter «Fehlschläge» |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | Zeitpläne der neuen Jobs mit Takt (einziger Ort des Abgleichtakts: `wishes.sync`); `wishes.submit`, `wishes.withdraw`, `wishes.import` laufen nach Ereignis ohne Zeitplan |
| API-Operationen | bestehende Job-Operationen; neue Typen über `RegisterTypes` |
| Mockup | M-30, `#/admin/jobs`, Sprung `#/admin/jobs?group=system&job=wishes.sync`; `js/admin-a.js` (`AA_JOBS`, `aaJobsPage`, `aaJobRow`, `DIALOGS.aaSched`, `DIALOGS.aaRuns`) |
| Abgedeckte interaktive Zustände | acht neue Jobarten mit «NEU» und Gruppen-Markierung: «Wunsch übermitteln» (Zustellung), «Fehlende Titel auffrischen» und «Staffelliste der Serien» (Metadaten), «Wünsche abgleichen», «Verfügbare Wünsche prüfen», «Jellyseerr-Benutzer laden», «Zurückgezogene Wünsche in Jellyseerr löschen» (`wishes.withdraw`) und «Bestehende Anfragen übernehmen» (`wishes.import`) (System); die beiden letzten mit «Ausgelöst durch: automatisch, durch Ereignisse im Server», nur «Pausieren» und «Läufe», kein Lauf im Mockup; «Jetzt ausführen» für `wishes.sync` (gleicht ab oder meldet den Zustand), `seerr.users`, `wishes.reconcile` («nichts zu tun»); «Pausieren» von `wishes.submit` mit Banner «Wünsche bleiben «Wird übermittelt» …»; «Nächster Lauf» «wartet bis Jellyseerr eingerichtet ist»; Zeitplan-Dialog mit Takt, Zeitfenster, einmaligem Termin und Hinweis «einziger Ort des Abgleichtakts»; Läufe-Dialog von `wishes.submit` mit wartenden Wünschen («Jetzt übermitteln») und fehlgeschlagenen («Erneut»); Reiter «Fehlschläge» mit Zeilen «Wunsch übermitteln» und «Zum Wunsch», auch nach «24 h vergehen» |
| Spätere technische Umsetzung | Stufe 1: Schritt 1.5 (`seerr.users`); Stufe 2: Schritte 2.2 (`titles.refresh`), 2.4 (`wishes.submit`, `wishes.withdraw`), 2.5 (`wishes.sync` mit Erstabgleich, `wishes.reconcile`), 2.10 (Jobtexte); Stufe 4: Schritte 4.1 (`series.catalog`), 4.6 (`wishes.import`); Tabelle `job_schedules`; `03-architektur.md` Abschnitt 13, `07-administration.md` Abschnitt 12 |

## 3. Zuordnung zur Mindestprüfliste des Auftraggebers

| Oberfläche der Liste | Mockup | Bemerkung |
|---|---|---|
| Startseite | M-01 | Reihen, Bühne, Reihe «Deine Wünsche» |
| Spotlight | M-01 (Bühne und Posterreihe), M-02 (Liste), M-27 (Verwaltung) | |
| Film-Detailseite | M-05 (vorhanden, Chip «Dein Wunsch ist da»), M-03 (fehlend, neu) | |
| Serien-Detailseite | M-06 (vorhanden mit fehlenden Staffeln), M-04 (fehlend, neu) | |
| Globale Suche | M-07 | Abschnitt «Noch nicht in der Bibliothek» |
| Entdecken | M-07, M-02, M-03, M-04 | Diese Seite gibt es in Slouchcat nicht, und es entsteht keine (Anhang F 2.14, `02-funktionskonzept.md` Abschnitt 8): Suche, Spotlight-Liste und Vorschauseite decken das Finden fehlender Titel ab, ohne eine Liste zu bauen, die wie eine algorithmische Empfehlung wirkt (Non-Goal). |
| Meine Wünsche | M-08 | mit Sicht des Hauptprofils auf alle Profile |
| Benachrichtigungen | M-09 (Eingang), M-10 (Kanäle) | |
| Profileinstellungen | M-11 (Profile), M-10 (Kanäle), M-12 (Profilwahl) | die Seite «Einstellungen» des Profils (Tonsprachen, Untertitel) ändert sich nicht: Hinweisseite `#/unchanged/einstellungen` |
| Admin Integrationen | M-21, M-22 | |
| Admin Benutzer | M-26 (Konten, Einladungen), M-24 (Zuordnung) | |
| Admin Profile | M-11, M-24, M-26 | Profile verwaltet jedes Konto selbst (M-11); die Verwaltung zeigt sie nur (Chips in M-26) und begrenzt sie über den Deckel des Kontos (M-26) und die Zuordnung je Profil (M-24). |
| Admin Spotlight | M-27, M-28 | |
| Admin Benachrichtigungen | M-29 | |
| Admin Hintergrundjobs | M-30 | |
| Admin Anfragen | M-23, dazu M-20 (Hinweise) und M-25 (Regeln) | |

## 4. Gegenprobe Einstellungen

Spalte «Gespeichert» nach `08-datenmodell-api.md` Abschnitt 2.12 und Abschnitt 1.1.

| Einstellung | Sichtbar und bedienbar im Mockup | Gespeichert | Beobachtbare Wirkung im Mockup |
|---|---|---|---|
| Jellyseerr verwenden | M-22 `#/admin/integrations/jellyseerr`, Band «Verbindung», Schalter «Jellyseerr verwenden» | `settings` `seerr.enabled` (ab Werk 0) | Kachel M-21 und Chip zeigen «Ausgeschaltet»; auf M-01 verschwinden fehlende Titel; neue Wünsche nicht möglich |
| Adresse | M-22, Feld «Adresse» | `seerr.url` | mit «falsch» nach «Speichern»: «Nicht erreichbar», Hinweis auf M-20 |
| API-Schlüssel | M-22, Feld «API-Schlüssel» (Platzhalter «Gesetzt, endet auf …a1b2») | `seerr.api_key` (verschlüsselt) | mit «falsch»: «Schlüssel ungültig»; leer lassen behält den Schlüssel |
| Sofortmeldung | M-22, Band «Sofortmeldung», «Adresse erzeugen» | `webhook_integrations` Art `jellyseerr` (Stufe 4) | einmal gezeigte Adresse; «Letzte Meldung» nach «Meldung simulieren (Mockup)» |
| Wünsche aktiv | M-25 `#/admin/wishes/settings`, «Wünsche aktiv» | `wishes.enabled` (ab Werk 0) | fehlende Titel verschwinden aus M-01, M-02, M-07; Band in M-23 und M-24; übrige Zeilen in M-25 gesperrt |
| Filme erlauben | M-25, «Filme erlauben» | `wishes.allow_movies` | fehlende Filme verschwinden auf M-01, M-02 und M-07; Vorschauseite eines Films ohne eigenen Wunsch gibt 404 |
| Serien erlauben | M-25, «Serien erlauben» | `wishes.allow_shows` | fehlende Serien verschwinden; auf M-06 keine gedämpften Staffeln und keine Zeile; der Chip «2 von 3 Staffeln vorhanden» bleibt (Abschnitt 9, Nr. 15) |
| Aus dem Spotlight | M-25, «Aus dem Spotlight» | `wishes.from_spotlight` | Bühne M-01 und Spotlight-Liste M-02 zeigen keine fehlenden Titel; M-27 warnt «Anfordern aus dem Spotlight ist aus» |
| Aus der Suche | M-25, «Aus der Suche» | `wishes.from_search` | M-07 ohne Abschnitt und ohne Pille «Noch nicht da» |
| Auf Detailseiten | M-25, «Auf Detailseiten (fehlende Staffeln)» | `wishes.from_detail` | M-06 ohne «Staffeln anfordern», Staffelkarten nicht klickbar; gedämpfte Karten, Zeile und Chip bleiben (Abschnitt 10.2, Nr. 1); M-03/M-04 bleiben erreichbar |
| Bereits angeforderte Titel | M-25, Segment «Mitwünschen»/«Nur Zustand zeigen» | `wishes.already_requested` (`join`/`status`) | «Nur Zustand zeigen»: Bühne und Vorschauseite ohne «Benachrichtige mich», Posterkarte mit Chip «Bereits angefordert» statt Glocke; in M-13 sind fremd angeforderte Staffeln gesperrt und zeigen ihren Zustand |
| Konten ohne Zuordnung | M-25, Segment «Nicht erlauben»/«Sammelbenutzer» | `wishes.unmapped` (`deny`/`fallback`) | «Sammelbenutzer» mit Wahl: M-26 zeigt «Erlaubt · … (Sammelbenutzer)», «Oma & Opa» darf anfordern und sieht fehlende Titel |
| Sammelbenutzer | M-25, Auswahl unter «Sammelbenutzer» | `wishes.shared_seerr_user` | M-13 «(Sammelbenutzer)»; M-11 «Sendet als … (Sammelbenutzer)» |
| Neue Konten dürfen anfordern | M-25, Schalter | `wishes.new_accounts_can_request` | Vorgabe des Schalters im Dialog «Konto einladen» (M-26) |
| Zurückziehen erlauben | M-25, Schalter | `wishes.allow_withdraw` | «Zurückziehen» verschwindet in M-08 und M-03 |
| Externe Nachrichten bei PIN-Profilen ohne Titel | M-25, Schalter | `wishes.pin_external_hide_title` | Betreff für Sandra in M-29 «Zuletzt zugestellt» und in M-00 mit oder ohne Titel |
| Zuordnung Konto | M-24 `#/admin/wishes/mapping`, Auswahl «Jellyseerr-Benutzer» je Konto | `wish_account_map` | Chip in M-26, Zeile «Sendet als» in M-11, Fakten im Dialog M-13, fehlende Titel für die Profile des Kontos |
| Zuordnung Profil | M-24, Band «Profile» je Konto, Auswahl «Wie Konto»/Benutzer | `wish_profile_map` | M-13 «(eigener Benutzer des Profils)»; Auto-Freigabe und Kontingent wechseln |
| Wunschrecht Konto | M-26 `#/admin/accounts`, Zeile aufklappen, Formular «Zugriff», «Darf Titel anfordern» (Administratorkonten: Block «Wünsche») | `accounts.can_request` | Spalte «Wünsche»; Profilschalter in M-11 wird bedienbar oder gesperrt |
| Wunschrecht Profil | M-11 `#/settings/profiles`, «Bearbeiten», Abschnitt «Wünsche», «Darf Titel anfordern» | `profiles.can_request` | Chips in M-11, M-24, M-26; fehlende Titel für das Profil sichtbar oder nicht |
| Einladung | M-26, «Konto einladen», «Darf Titel anfordern» | `invitations.can_request` | Spalte «Wünsche» im Reiter «Einladungen» |
| Fehlende Titel zeigen | M-27 `#/admin/spotlight`, Band «Fehlende Titel», «Fehlende Titel zeigen», «Speichern» | `spotlight_config.show_missing` (ab Werk aus, F-8) | Bühne M-01 mit oder ohne fehlende Titel; Filter in M-02; Verweis in M-25 |
| Höchstens fehlende Titel | M-27, Zahl 0 bis 10 | `spotlight_config.max_missing` | mit 10 erscheinen auf M-01 für Marco auch «Toy Story 5» und weitere |
| Reihenfolge | M-27, Segment «Wie bei Trakt»/«Nach den vorhandenen» | `spotlight_config.missing_order` | fehlende Titel an ihrem Trakt-Platz oder am Ende der Bühne |
| Reihe «Deine Wünsche» | M-28 `#/admin/home-rows`, Zeile «Deine Wünsche», Platz und «Ausblenden», «Speichern» | `home_rows` (Schlüssel `wishes`) | Reihe auf M-01 wandert oder fehlt |
| Ereignisse je Kanal | M-10 `#/settings/notifications` und M-29 `#/admin/notifications`, Häkchen «Wünsche: verfügbar», «Wünsche: freigegeben, abgelehnt, fehlgeschlagen», «Wünsche: Freigabe nötig» | `account_channels.events` | beim Vorspulen nennt die Meldung am Bildschirm nur noch «App» oder die verbleibenden Kanäle; «Zuletzt zugestellt» bekommt keinen Eintrag |
| Zeitplan `wishes.sync` | M-30 `#/admin/jobs`, Gruppe «System», «Zeitplan» | `job_schedules` | M-22 Band «Abgleich» zeigt den neuen Takt |
| Zeitplan `wishes.reconcile`, `seerr.users` | M-30, Gruppe «System», «Zeitplan» | `job_schedules` | «Nächster Lauf» in M-30 |
| Zeitplan `titles.refresh`, `series.catalog` | M-30, Gruppe «Metadaten», «Zeitplan» | `job_schedules` | «Nächster Lauf» in M-30 |
| `wishes.submit` | M-30, Gruppe «Zustellung»: kein Zeitplan (Ereignis), nur «Pausieren» und «Läufe» | Pause wie heute | Banner und Hinweis «nach 24 Stunden … fehlgeschlagen» |
| `wishes.withdraw`, `wishes.import` | M-30, Gruppe «System»: kein Zeitplan (Ereignis), nur «Pausieren» und «Läufe» | Pause wie heute | nur Anzeige; die beiden Jobs laufen im Mockup nie (Abschnitt 9) |
| Ansicht «Wünsche aller Profile meines Kontos» | M-08, Schalter (nur Hauptprofil) | keine Einstellung, nur Ansicht | Liste mit Profilnamen |

## 5. Gegenprobe Berechtigungen

| Berechtigung | Wo einstellbar oder sichtbar | Prüfung im Mockup |
|---|---|---|
| Wunschrecht Konto (Deckel) | einstellbar M-26 (Formular «Zugriff» oder Block «Wünsche»), sichtbar M-24, M-26, M-11 | Mockup › Gast: keine fehlenden Titel, kein Eintrag «Wünsche», Profilschalter in M-11 gesperrt. Dann Marco: M-26, gast aufklappen, «Darf Titel anfordern» an, «Zugriff speichern» (Chip «Zuordnung fehlt»); M-24 gast auf «wohnzimmer-tv»; Mockup › Gast, M-11 «Bearbeiten», Schalter an, «Speichern»: fehlende Titel erscheinen |
| Wunschrecht Profil | einstellbar M-11 (Hauptprofil), sichtbar M-24, M-26 | Mockup › Mia: kein Abschnitt in M-07, «Keine Wünsche» in M-08. Marco: M-11, Mia «Bearbeiten», «Darf Titel anfordern», «Speichern»; Mockup › Mia: M-07 zeigt fehlende Titel bis «ab 6» |
| Zuordnung als Bedingung (F-9) | M-24, M-26, M-20 | Mockup › «Oma & Opa» (Konto darf, keine Zuordnung): keine fehlenden Titel, kein Hinweis in M-07. Marco: M-24 bei grosseltern «Übernehmen»; Mockup › «Oma & Opa»: fehlende Filme erscheinen |
| Jellyseerr-Recht Filme/Serien | sichtbar M-24 (Chips «Filme», «Serien», Warnung), nicht einstellbar (gehört Jellyseerr) | nach «Übernehmen» bei grosseltern: Mockup › «Oma & Opa» sieht fehlende Filme, aber keine fehlenden Serien und auf M-06 keine gedämpften Staffeln (`canRequest` Grund `seerr_kind`); M-24 warnt «darf keine Serien anfordern» |
| Jellyseerr-Benutzer gelöscht | sichtbar M-24, M-20, M-26 | M-24 «Benutzer „sandra“ in Jellyseerr löschen (Mockup)»; Mockup › Sandra: fehlende Titel verschwinden (`seerr_user_gone`), eigene Wünsche bleiben in M-08 |
| Auto-Freigabe | sichtbar M-24 (Chip «Auto-Freigabe»/«Braucht Freigabe»), M-13 | Marco fordert «Odyssey» an: «In Beschaffung». Mockup › Noah fordert «Moana» an: M-13 sagt «Braucht eine Freigabe», danach «Freigabe ausstehend» und Zustellung `wish_review` an Marco |
| Kontingent | sichtbar M-24 (Chip «Kontingent 5 Titel je 7 Tage»), M-13 (Zählung) | Mockup › Noah: drei Filme anfordern gelingt, der vierte endet als «Fehlgeschlagen» mit «Das Kontingent des Jellyseerr-Benutzers ist erschöpft.» |
| Freigeben und Ablehnen in Slouchcat (Stufe 4) | M-23, nur Administrator | Marco: M-23 «Toy Story 5» «Freigeben» oder «Ablehnen» (Rückfrage); Mockup › «Nicht erreichbar»: «Geht nicht …»; Mockup › Sandra: `#/admin/wishes` zeigt «Nur für Administratoren» |
| Hauptprofil sieht Wünsche aller Profile | M-08, Schalter | Marco: Schalter an zeigt Wünsche von Sandra und Noah mit Namen; Mockup › Noah: kein Schalter |
| Altersstufe | M-11 und M-26 (bestehend), wirkt auf fehlende Titel | Mockup › Noah (12): «Resident Evil» (16) fehlt überall, `#/titles/movie/1423191` zeigt «Titel nicht gefunden»; Mockup › Mia: «Tulsa King» gibt 404 |
| Ändern der Kanäle | M-10, nur Hauptprofil | Mockup › Sandra: Häkchen gesperrt mit Hinweis |
| Ereignis «Freigabe nötig» | M-10, M-29, nur Administratorkonten | Mockup › «Oma & Opa»: Häkchen fehlt |
| Zugang zur Verwaltung | Administrator mit Hauptprofil | Mockup › Sandra auf `#/admin`: «Nur für Administratoren» |

## 6. Gegenprobe Zustände eines Wunsches

Kürzel der Orte: B = Bühne (M-01), P = Posterreihe (M-01, M-02, M-07), V = Vorschauseite (M-03, M-04) bzw.
Serienseite (M-06), W = Meine Wünsche (M-08), A = Verwaltung Anfragen (M-23), S = Einträge in M-27.

| Zustand | Sichtbar auf | Herstellen im Mockup |
|---|---|---|
| `queued` «Wird übermittelt» | B (Zustandsknopf), P (Chip), V (Block mit Zusatzzeile), W (Klartext), A (Filter), M-30 «Fehlschläge» und Läufe von `wishes.submit` | Mockup › «Nicht erreichbar» (Zusatz «Jellyseerr ist nicht erreichbar»), «Schlüssel ungültig» oder «CSRF-Schutz an» (Zusatz «Jellyseerr blockiert schreibende Aufrufe (CSRF-Schutz)»); auf M-01 bei «Odyssey» «Film anfordern», «Anfordern». Weiter mit Mockup › «Verbunden» und «Übermitteln» oder «Abgleich jetzt ausführen» |
| `pending` «Freigabe ausstehend» | B, P, V, W («Wartet auf die Freigabe in Jellyseerr.»), A (mit «Freigeben», «Ablehnen»), Bereichsreiter-Zähler, M-20 | vorhanden: «Toy Story 5» (Mockup › Noah). Neu: Mockup › Noah, «Moana» anfordern |
| `processing` «In Beschaffung» | B, P, V, W, A, S «Angefordert · In Beschaffung» | vorhanden: «Resident Evil» (Marco). Neu: Marco fordert «Primetime» an |
| `partial` «Teilweise verfügbar» | W (Staffel-Chips), A, Reihe «Deine Wünsche» («1 Staffel fehlt»), Serienseite `#/items/black-clover` | Marco: `#/titles/show/73223`, «Serie anfordern», «2 Staffeln anfordern»; Mockup «Datei trifft ein» |
| `available` «Verfügbar» | W (mit «Abspielen»), A, B und M-05 Chip «Dein Wunsch ist da», Reihe «Deine Wünsche» 14 Tage, M-09 | vorhanden: «Godzilla Minus One». Neu: Mockup «Datei trifft ein» bei «Resident Evil»; bei Serien «Nächste Staffel trifft ein» |
| `declined` «Abgelehnt» | W (mit «Erneut anfordern»), A, V (Hinweis «Dein letzter Wunsch: Abgelehnt»), M-09 | vorhanden: «Street Fighter» (Mockup › Noah). Neu: Mockup «Ablehnen» bei einem `pending` oder M-23 «Ablehnen» |
| `failed` «Fehlgeschlagen» | W (Grund), A (Diagnose «Fehlergrund»), V (Hinweis), M-20, M-30 «Fehlschläge», M-09 | vorhanden: «Coyote vs. Acme» (Mockup › Sandra). Neu: Mockup «Scheitern» bei `processing`, Kontingent (Abschnitt 5) oder «24 h vergehen» bei `queued` (Grund «Jellyseerr war 24 Stunden lang nicht erreichbar.», Zustellung «Fehler» an die Administratoren) |
| `withdrawn` «Zurückgezogen» | W (Filter «Abgeschlossen»), A (Filter), V (Hinweis «Du hast ihn zurückgezogen.») | Mockup › Noah, M-08, «Toy Story 5» «Zurückziehen», bestätigen |
| fehlt «Noch nicht in der Bibliothek» | B (Chip), P (Plus-Knopf, Unterzeile «Noch nicht da»), V (Hinweis), M-07 (Abschnitt), S (Chip) | vorhanden: «Odyssey», «Primetime» für Marco |
| bereits angefordert (Mitwunsch) | B und P («Bereits angefordert», Glocke), V (Hinweis), M-13 «Schon angefordert»; nach dem Mitwunsch W («Du wünschst mit», «Nicht mehr benachrichtigen»), V («Nicht mehr mitwünschen»), A («+ Sandra») | Mockup › Sandra; auf M-01 «Resident Evil» «Benachrichtige mich», im Dialog «Benachrichtige mich». Für Staffeln: Marco bei «Black Doves», «Serie anfordern», Staffel 1 («Bereits angefordert») und Staffel 2 wählen, «2 Staffeln anfordern» |
| bereits angefordert (nur Zustand) | B (nur «Details», «Trailer»), P (Chip «Bereits angefordert»), V (Hinweis ohne Knopf), M-13 (Staffel gesperrt mit Zustand) | M-25 «Nur Zustand zeigen», dann wie oben |
| teilweise vorhanden | B (Chip «2 von 3 Staffeln vorhanden» bei «Tulsa King»), M-06 (gedämpfte Karten, Zeile), M-13 («Vorhanden») | vorhanden: `#/items/tulsa-king` |
| angekündigt | M-06 und M-04 (Band «Angekündigt»), M-13 (Chip, nicht wählbar), Hinweis «Staffel 4 ist angekündigt» | vorhanden: Tulsa King Staffel 4, «Die Ringe der Macht» Staffel 3, Dexter Staffel 2 |
| In Jellyseerr angelegt | W (Chip), A (Chip, Herkunft), M-24 (Zähler) | vorhanden: «Verity» (Sandra), «Clayface» (nicht zuordenbar) |
| nicht zuordenbar | A (Filter, Hinweis), M-22 (Zähler), M-24 («davon 1 nicht zuordenbar») | vorhanden: «Clayface» von wohnzimmer-tv |

## 7. Gegenprobe Verbindung, Zuordnung und Benachrichtigungen

### 7.1 Verbindung

| Zustand | Mockup-Seite | Klickfolge |
|---|---|---|
| Nicht eingerichtet | M-21, M-22, M-20, M-23, M-25, M-27 | Mockup › «Nicht eingerichtet», oder M-22 «Verbindung löschen …» bestätigen. M-22 zeigt die Schrittanleitung; M-01 ohne fehlende Titel |
| Verbunden | alle | Mockup › «Verbunden», oder M-22 «Speichern» ohne «falsch» |
| Schlüssel ungültig | M-22 (Band offen, Prüfliste mit 403), M-21, M-20 | Mockup › «Schlüssel ungültig», oder M-22 Schlüssel «falsch123», «Speichern» |
| Nicht erreichbar | M-22, M-21, M-20, M-23 (Band), M-13 (Hinweis) | Mockup › «Nicht erreichbar», oder Adresse mit «falsch», «Speichern» |
| CSRF-Schutz an | M-22 (Band offen, Prüfpunkt als Warnung), M-21 (gelber Punkt «CSRF-Schutz blockiert»), M-20 (Hinweis «Jellyseerr blockiert schreibende Aufrufe»), M-23 (gelbes Band), M-13 (Hinweis), Schaltfläche «Mockup · Jellyseerr: CSRF-Schutz blockiert» | Mockup › «CSRF-Schutz an»; M-22 «Verbindung testen». Neue Wünsche bleiben `queued`; «Abgleich jetzt ausführen» meldet «Abgleich nicht möglich» |
| Ausgeschaltet | M-22, M-21 | M-22 Schalter «Jellyseerr verwenden» aus |
| Andere Instanz erkannt | nicht herstellbar (Abschnitt 9) | |

### 7.2 Zuordnung

| Zustand | Mockup-Seite | Klickfolge |
|---|---|---|
| Profil zugeordnet | M-24 (Band «Profile» bei marco), M-11 | vorhanden: Sandra → sandra, Noah → kinder |
| Profil nicht zugeordnet (wie Konto) | M-24, M-11 «Benutzer des Kontos» | vorhanden: Marco, Mia |
| Konto nicht zugeordnet | M-24 Chip «Zuordnung fehlt», M-26, M-20 Hinweis; Profile sehen keine fehlenden Titel | vorhanden: grosseltern (mit Vorschlag), gast |
| Benutzer nicht gefunden | M-24 (rote Warnung, Band offen), M-20, M-26 | M-24 «Benutzer „sandra“ in Jellyseerr löschen (Mockup)»; Mockup › Sandra: fehlende Titel verschwinden |
| Zuordnung geändert | M-24 Chip «Zuordnung geändert», Meldung | eine Auswahl in M-24 ändern oder «Übernehmen» |
| Belegter Benutzer | M-24 Auswahl gesperrt «(belegt von …)» | Auswahl bei grosseltern öffnen |
| Sammelbenutzer | M-25, M-26, M-13 | M-25 «Sammelbenutzer», Benutzer wählen |

### 7.3 Benachrichtigungen

| Zustand | Mockup-Seite | Klickfolge |
|---|---|---|
| neu (ungelesen) | M-09, Glocke mit Zähler | vorhanden: Marco «Dein Wunsch ist da: Godzilla Minus One»; neu durch Vorspulen |
| gelesen | M-09 | Meldung anklicken oder «Als gelesen markieren», «Alle als gelesen markieren» |
| Ereignis ausgeschaltet | M-10, M-29, M-00 «Zuletzt zugestellt» | M-10 bei E-Mail «Wünsche: verfügbar» abwählen; Mockup «Datei trifft ein» bei «Resident Evil»: Meldung am Bildschirm nennt nur «App, Web-Push» |
| E-Mail und Web-Push | M-29 «Zuletzt zugestellt» | vorhanden für marco (beide aktiv); grosseltern nur E-Mail |
| Telegram | M-10 | «Verknüpfen», «Verknüpfung prüfen», Schalter «Aktiv» an |
| Freigabe nötig an Administratoren (Stufe 4) | M-29 «Zuletzt zugestellt» | Mockup › Noah fordert «Moana» an: Zustellung «Wunsch wartet auf Freigabe» an marco |
| Zustandsmeldung (Stufe 4) | M-09 | Mockup «Freigeben»: Eingang «Freigegeben: …», extern nur mit Häkchen `wish_status` (ab Werk aus) |
| Übermittlung nach 24 h gescheitert | M-09, M-29, M-00 | Mockup › «Nicht erreichbar», Titel anfordern, Mockup «24 h vergehen»: Eingang «Fehlgeschlagen: …» beim Profil (in den Papieren erst Stufe 4), Zustellung «Wunsch nicht übermittelt: …» per E-Mail an marco (Ereignis «Fehler», Stufe 2) |
| PIN-Profil | M-29, M-00 | Mockup › Sandra fordert einen Film an, Mockup «Datei trifft ein»: Betreff ohne Titel |
| nicht sichtbar, keine Meldung | M-09 | Verfügbar-Meldungen über einen Titel über der Altersstufe des Profils entstehen nicht (Funktion `notify` in `js/store.js`) |

## 8. Bestehende Funktionen bleiben

| Seite | Bestehendes im Mockup weiter bedienbar | Bewusst vereinfacht |
|---|---|---|
| M-01 Startseite | Bühne blättern, Pause, Wechsel, Posterreihe, Gesehen, Merken, alle Reihen nach Verwaltung | Reihen mit festen Beispieltiteln; «Abspielen» und «Trailer» als Dialog; Sammlungen, Genres, Meine Medien nur als ausgeblendete Zeilen |
| M-02 Spotlight-Liste | Raster, «Zurück» | keine Sortierung |
| M-05/M-06 Detailseiten | Abspielen, Von vorne, Trailer, Merken, Gesehen, Empfehlen, Löschen-Dialog, Staffelwahl, Folgenliste, Gesehen je Folge, Besetzung | Player und Trailer als Dialog; Löschen löscht nichts; höchstens 30 Folgen je Staffel |
| M-07 Suche | Suchfeld, alle Filter, Personen, Sammlungen | Personen und Sammlungen führen auf die Hinweisseite «Unverändert»; «Auf diesem Gerät» mit drei festen Titeln |
| M-09 Benachrichtigungen | «Neue Folge», gelesen/ungelesen, «Alle als gelesen markieren», «Kanäle einstellen» | |
| M-10/M-29 Kanäle | Aktiv, Ereignisse, Adresse, Telegram-Verknüpfung, Geräte, Speichern, Testnachricht | «In Telegram öffnen» nur als Meldung |
| M-11 Profile | ganzer Profildialog, PIN, Schauzeit, Löschen | Schauzeit als erfundenes Diagramm |
| M-12 Profilwahl | Kacheln, PIN | jede vierstellige PIN öffnet; Kontowechsel nur im Mockup |
| M-20 Übersicht | Kennzahlen, Hinweise, Schnellzugriffe, Serverzustand, Aktivität | «Gerade aktiv» und «Jetzt läuft» fest; Streams, Pflege, Speicher als feste Zahlen |
| M-21 Integrationen | alle sechs Ansichten mit Speichern, Prüfen, Löschen, Webhooks samt Pfad-Umsetzung | externe Seiten nur als Meldung; Prüfungen simuliert |
| M-26 Konten | Tabelle, Zugriff, Konto ändern, Einladungen, Rollen, Entfernen | Passwortprüfung simuliert |
| M-27 Spotlight | alle elf Felder des Bands «Quelle», Einträge, verwaiste Anpassungen | Abruf ändert die Liste nicht |
| M-28 Startseite | Platz, Ausblenden, Grenze 15, Vorschau, Speicherleiste | Vorschau als Schema |
| M-30 Hintergrundjobs | alle Jobarten, Pools, Läufe, Zeitpläne, Fehlschläge | Läufe und Restzeiten erfunden |
| unveränderte Seiten | Merkliste, Gesehen, Einstellungen, Bibliotheken, Person, Sammlungen (`#/unchanged/…`); 15 Verwaltungsseiten (`#/admin/x/…`, etwa Pflege, Geräte, Streams) | nur Hinweisseite «Unverändert» mit «Zurück» und «Alle Mockup-Seiten» |

**Was das Mockup grundsätzlich nicht zeigt:** Lade- und Fehlerzustände der App ausser dem Verbindungstest (etwa
`tmdb_unreachable` 503, 429 der Suche); Offline-Verhalten der PWA; Texte in den 23 Sprachen; Tastaturtests nach
Regel 8 (das Mockup ist bedienbar, wird aber nicht geprüft); echte Bilder, wenn `img/` fehlt; Zeitabläufe ausser
der 24-Stunden-Grenze (Backoff, Zeitplan, Frist 14 Tage); den echten Player und Trailer.

## 9. Lücken

Stellen, die das Mockup nicht bedienbar macht oder nicht zeigt, obwohl die Papiere sie vorsehen. Nicht repariert.
Die Regel F-9 (fehlende Titel nur bei Anfordern-Können) und die Folge, dass «kein Recht» und «Konto ohne
Zuordnung» im Dialog M-13 nur über einen eigenen Wunsch erreichbar sind, sind keine Lücke mehr: So steht es in
`02-funktionskonzept.md` Abschnitt 4.5.

| Nr. | Lücke | Datei und Stelle | Warum das nicht stört oder wo es steht |
|---|---|---|---|
| 1 | Der Verbindungstest kann «Handeln im Namen von Benutzern wirkt nicht», «Version zu alt» und «Kein Sonarr in Jellyseerr» nicht als Fehler oder Warnung zeigen; die Varianten von «Nicht erreichbar» (abgelehnt, Weiterleitung), die Warnung bei `http` ausserhalb des Heimnetzes und die Pflicht, bei neuem Host den Schlüssel neu einzugeben, fehlen | `js/admin-a.js` Zeilen 347 bis 369 (`aaSeerrRun`) und 440 bis 459 (`ACT.aaSeerrSave`) | Die Prüfliste zeigt alle sieben Punkte und ihre Form; die Texte der Fehlerfälle stehen in `07-administration.md` Abschnitt 4.1 und `08-datenmodell-api.md` Abschnitt 4.5 (`testSeerrConnection`) |
| 2 | «Wünsche aktiv» lässt sich einschalten, ohne dass der Test zuletzt gelang | `js/admin-b.js` Zeile 311 | Teilfrage T-1 ist offen (`11-entscheide-und-adr.md`, Teilfragen T-1 und T-2); `07-administration.md` Abschnitte 4.6 und 7 |
| 3 | Instanzwechsel fehlt ganz: kein Prüfpunkt «Instanz», kein Zustand «Andere Instanz erkannt» auf M-21 und M-22, kein Hinweis «Andere Jellyseerr-Instanz erkannt» auf M-20, kein Band mit «Zuordnung bestätigen» und «Zuordnung löschen» auf M-24 | kein Vorkommen von «Instanz» in `js/` | Fingerabdruck ist **Vorschlag**; Ablauf in `03-architektur.md` Abschnitt 7.4, `04-benutzer-profilmapping.md` Abschnitt 8.4, `07-administration.md` Abschnitte 2, 4.1 und 6, AK-107 |
| 4 | Warnung «Auto-Freigabe für ein Kinderprofil» fehlt, etwa wenn Mia «wie Konto» als marco mit Auto-Freigabe sendet | `js/admin-b.js` Zeilen 173 bis 180 (`abUserWarn`) | **Vorschlag** in `07-administration.md` Abschnitt 6; die übrigen Warnungen zeigen die Form |
| 5 | Diagnose in M-23 ohne Versuche, nächsten Versuch, Media-ID, Rohstatus und Sichtbarkeit je Anforderer («für Noah nicht sichtbar»); kein Filter nach Medienart und keine eigene Spalte «Art»; Kennzahl «Verfügbar» zählt alle verfügbaren statt der letzten 30 Tage; bei fehlender Verbindung sind «Freigeben» und «Ablehnen» nicht gesperrt, sondern melden «Geht nicht …» | `js/admin-b.js` Zeilen 43 bis 54, 57 bis 94 und 100; `js/store.js` Zeile 207 | Aufbau, Verlauf und die vorhandene Diagnose genügen zur Beurteilung; vollständig in `07-administration.md` Abschnitte 5 und 13 |
| 6 | Zusatz «wird eingelesen» bei `processing` fehlt, ebenso der Vorschlag «5 von 10 Folgen» für eine teilweise eingelesene Staffel | kein Vorkommen in `js/` | `02-funktionskonzept.md` Abschnitte 2 und 6, `03-architektur.md` Abschnitte 8.1 und 8.3; der Zustandschip zeigt die Form |
| 7 | Der Fall «Serie ohne TVDB-ID» (kein Knopf, Hinweis) ist nicht dargestellt | kein Vorkommen in `js/` | AK-39 und `10-abnahmekriterien.md` Abschnitt 8 (M-03, M-04); Hinweisform wie «Anfordern ist für dieses Profil nicht möglich» in `js/viewer-titles.js` Zeile 107 |
| 8 | Stufen sind nur bei «Sofortmeldung» gekennzeichnet. Ohne Hinweis erscheinen: Freigeben/Ablehnen (M-23), Eingangsmeldungen «Freigegeben», «Abgelehnt», «Fehlgeschlagen» und Staffel-Meldungen (M-09, auch nach «24 h vergehen»), `wish_status` und `wish_review` (M-10, M-29), fehlende Staffeln (M-06) und `wishes.import`, `series.catalog` (M-30), alle Stufe 4 | `js/admin-a.js` Zeile 415; `js/viewer-pages.js` Zeile 237 (`VP_EVENTS`) | Die Stufen stehen in `06-benachrichtigungen.md` Abschnitt 2 und `09-umsetzungsplan.md` Abschnitte 3 bis 6; das Mockup zeigt das Endbild |
| 9 | Ungespeicherte Änderungen (M-22, M-26, M-27, M-28) zeigen den Chip «ungespeichert» bzw. «Ungespeicherte Änderungen», aber beim Verlassen der Seite kommt keine Rückfrage | keine Wegnavigations-Sperre in `js/core.js` (`go`, `render`) | Die Rückfrage ist heutiges Verhalten und bleibt (`07-administration.md` Abschnitte 3, 8 und 9) |
| 10 | M-24 hat keinen Knopf «Anfragen übernehmen» (Stufe 4); die Meldung nach einer Zuordnung nennt keine Folgen («2 offene Wünsche bleiben bei kinder», «3 Anfragen von grosseltern werden übernommen»); `wishes.import` und `wishes.withdraw` stehen in M-30, laufen aber nie (Zurückziehen setzt sofort `withdrawn`, ohne Job und ohne den Fall «war schon freigegeben») | `js/admin-b.js` Zeilen 224 und 240 bis 251; `js/admin-a.js` Zeilen 534 und 535; `js/store.js` Zeilen 393 bis 401 (`withdrawWish`) | Übernahme als Beispieldaten («Verity», «Clayface») sichtbar; Regeln in `03-architektur.md` Abschnitte 6.6 und 7.3, `04-benutzer-profilmapping.md` Abschnitt 8.2, `07-administration.md` Abschnitte 5 und 6 |
| 11 | Der Kopf der Kanalbänder zählt die Ereignisse nicht («5 Ereignisse») | `js/viewer-pages.js` Zeile 253 | `07-administration.md` Abschnitt 11; die Häkchen selbst sind bedienbar |
| 12 | Erstabgleich von `wishes.sync` als eigener Lauf mit Fortschritt («Seite 12 von 60») fehlt | kein Vorkommen in `js/` | `10-abnahmekriterien.md` Abschnitt 8 (M-30), `03-architektur.md` Abschnitt 7.2, `07-administration.md` Abschnitt 12 |
| 13 | Bei «CSRF-Schutz an» stimmen drei Texte nicht: Das Band «Abgleich» sagt «solange Jellyseerr den Schlüssel ablehnt»; der Dialog M-13 sagt «Jellyseerr ist gerade nicht erreichbar»; auch der lesende Abgleich wird verweigert, obwohl nur schreibende Aufrufe scheitern | `js/admin-a.js` Zeile 393; `js/home.js` Zeile 150; `js/store.js` Zeile 412 (`syncNow`) | Der Zustand selbst ist herstellbar und wirkt richtig auf neue Wünsche (`queued` mit Zusatz); Texte nach `07-administration.md` Abschnitt 4.1 |

## 10. Unterschiede zwischen Mockup und Papieren

### 10.1 Bewusste Unterschiede

| Stelle | Mockup | Papiere |
|---|---|---|
| «Jellyseerr verwenden» | startet eingeschaltet (`js/store.js` Zeile 37, `seerr.enabled: true`) | ab Werk aus (`07-administration.md` Abschnitt 4.1, `08-datenmodell-api.md` Abschnitt 2.12) |
| «Wünsche aktiv» | startet eingeschaltet (`js/store.js` Zeile 47) | ab Werk aus, Redaktionsentscheid (`07-administration.md` Abschnitt 7) |
| «Fehlende Titel zeigen» | startet eingeschaltet (`js/store.js` Zeile 48, `showMissing: true`) | ab Werk aus (F-8, `07-administration.md` Abschnitt 9) |
| Beispieldaten, Zeit, Jellyseerr | gespielt über M-00 | gehört nicht zur App |

Die drei Schalter stehen an, damit man ohne Einrichtung sofort fehlende Titel sieht.

### 10.2 Verbleibende echte Widersprüche

Keine bekannt. Die sieben Widersprüche, die diese Matrix in ihrer vorigen Fassung führte, sind am 17. November 2026
behoben und im Browser nachgeprüft:

| Nr. | Widerspruch | Behoben durch |
|---|---|---|
| 1 | «Auf Detailseiten» aus zeigte weiter gedämpfte Karten, Zeile und Chip | Mockup: `js/viewer-titles.js` (`ghosts` verlangt `fromDetail`), `js/home.js` (`metaLine`) |
| 2 | Hilfetexte in M-25 beschrieben die alte Regel | Mockup: `js/admin-b.js` |
| 3 | Alle drei Stellen aus: Vorschauseite blieb erreichbar | Mockup: `missingVisible` in `js/store.js` verlangt mindestens eine Stelle; eigene Wünsche bleiben sichtbar |
| 4 | Detailseite Film galt als unverändert, zeigt aber den Chip «Dein Wunsch ist da» | Papiere: `02-funktionskonzept.md` Abschnitt 5, `10-abnahmekriterien.md` Abschnitt 8 |
| 5 | Namen und Gruppe von `wishes.withdraw` und `wishes.import` in M-30 | Mockup: `js/admin-a.js` (Namen wie `07-administration.md` Abschnitt 12, `wishes.withdraw` unter Zustellung, `wishes.import` mit «Stufe 4») |
| 6 | CSRF-Schutz im Test nur als Warnung | Mockup: `js/admin-a.js` (Prüfpunkt gilt als nicht bestanden) |
| 7 | Hinweis «n Konten ohne Zuordnung» zählte trotz Sammelbenutzer | Mockup: `js/admin-a.js` |

Ebenfalls behoben: Der Hinweis «Bereits angefordert» auf der Vorschauseite passt sich an «Nur Zustand zeigen» an, und der
Chip «2 von 3 Staffeln vorhanden» prüft jetzt das Recht für den Titel (Art, Rechte des Jellyseerr-Benutzers).
