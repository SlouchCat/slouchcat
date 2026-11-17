# Konzept 25 · Abdeckungsmatrix: Seiten, Mockups, Umsetzung

> **Stand:** Fassung 3 der Papiere mit beiden Rückmeldungen von Marco vom 17. November 2026 (E-1 bis E-7,
> [Anhang H](anhang/H-nachtrag-17-november.md)) und den Redaktionsentscheiden zur Kritik (Anhang H Abschnitt 9,
> K-1 bis K-23); Mockup vom 17. November 2026, abgeglichen mit den Papieren.

> **Kurz gesagt.** Diese Matrix führt jede Seite auf, die Konzept 25 berührt: 18 Seiten der Zuschauer-App
> (M-01 bis M-12, der Dialog M-13 und neu M-31 bis M-35), 11 Seiten der Verwaltung (M-20 bis M-30) und die
> Mockup-Steuerung M-00, zusammen 30 Blöcke. Je Seite steht, was sie heute kann, was sich ändert, wer sie nutzt,
> welche Einstellungen und Operationen dazukommen, was man im Mockup wirklich anklicken kann und wann und wo der
> Bau sie umsetzt. Danach folgen Gegenproben für jede Einstellung, jede Berechtigung, jeden Zustand eines Wunsches
> samt Download-Anzeige, die Verbindung, die Jellyseerr-Benutzer mit Vorschlag und Passwort, das Limit und die Tags.
> Grundlage ist der Code des Mockups (`js/*.js`), nicht der Plan. Abschnitt 9 nennt, was das Mockup nicht zeigt.
> Abschnitt 10 nennt die Unterschiede zwischen Mockup und Papieren.

Grundlage sind die Papiere `01-…` bis `11-…` in Fassung 3 und Anhang H. Operationen nach `08-datenmodell-api.md`
Abschnitte 4.5 und 4.6, Einstellungen nach Abschnitt 2.12, Tabellen nach Abschnitt 2, Stufen und Schritte nach
`09-umsetzungsplan.md`, Verwaltung nach `07-administration.md`, Zuschauerseiten nach `02-funktionskonzept.md`
Abschnitt 5, Abnahme nach `10-abnahmekriterien.md` Abschnitt 8. Regeln N-1 bis N-88 und N-2a nach Anhang H. Die
Einschränkungen aus `01-bestandsanalyse.md` Abschnitt 20 heissen TE-1 bis TE-…; E-1 bis E-7 sind nur Marcos
Antworten.

**Was Marco entschieden hat** (nur seine zwei Nachrichten und die Antworten E-1 bis E-7, Anhang H Abschnitte 1,
2, 8.1 und 8.2): Konto auch in Jellyseerr mit gleichem Login (E-1), Profil am Wunsch und als Tag in Radarr und
Sonarr (E-2), Limit je Konto, Jellyseerr zählt alles (E-3), Verknüpfen über die gleiche E-Mail (E-4), Bereich
«Wünsche» mit Discover, Movies, Series, Search, TVDB für Serien «konsistent», keine fehlenden Titel im Spotlight und
nirgends in der Zuschauer-App (E-5), Webhooks wie heute (E-6), Download-Fortschritt bei Wünschen (E-7). **Alles
andere** in dieser Matrix, etwa Namen, Vorgaben, Reihen, Anzeigen, Grenzen und die Entscheide zur Kritik K-1 bis
K-23 (etwa der Vorschlag einer Verknüpfung, N-2a), ist **Empfehlung der Redaktion**. Offen sind die Fragen F-1
bis F-34 ohne F-8 (durch E-5 entschieden) und F-20 (durch F-34 ersetzt), also 32, dazu T-1 und T-2.

## 1. So öffnest du das Mockup

- **Einstieg:** `docs/planning/mockups/25-medienwuensche/index.html` im Browser öffnen. Ohne Hash springt das
  Mockup auf `#/mockup`: Anleitung «So probierst du es aus» (sieben Schritte) und die Übersicht aller Seiten
  (`MOCK_PAGES` in `js/mock.js`, 18 Seiten der Zuschauer-App und 11 der Verwaltung) mit Kennung, Name und Link.
- **Steuerung:** die gelbe Schaltfläche «Mockup» unten rechts (M-00). Sie öffnet die «Mockup-Steuerung» mit den
  Teilen «Ansicht», «Wer schaut?», «Jellyseerr ist …» mit «Abgleich jetzt ausführen», «Limit», «Radarr und Sonarr
  (Profil-Tags, Fortschritt)», «Jellyseerr liest Staffeln aus …», «Konten», «Offene Wünsche vorspulen», «Zuletzt
  zugestellt», «Alle Mockup-Seiten» und «Alles zurücksetzen» (Block M-00).
- **Zustand:** Alles bleibt im Browser (`localStorage`, Schlüssel `slouchcat-mockup-25-v5`), bis «Alles
  zurücksetzen». Ist Jellyseerr nicht verbunden, trägt die Schaltfläche den Zusatz «· Jellyseerr: {Zustand}».
- **Vorführwerte:** Das Mockup startet mit «Jellyseerr verwenden» und «Wünsche aktiv» eingeschaltet; ab Werk sind
  beide aus (Abschnitt 10.1). Zwei Konten haben schon einen Jellyseerr-Benutzer (marco verknüpft, grosseltern
  eröffnet).
- **Klickfolgen** beginnen nach «Alles zurücksetzen» mit dem Profil Marco, wenn nichts anderes steht. «Mockup ›
  Noah» heisst: Steuerung öffnen, unter «Wer schaut?» Noah wählen.
- **Beispieldaten** (`04-benutzer-profilmapping.md` Abschnitt 11, `js/store.js` `seed()`):
  - Konto `marco@example.ch` (Besitzer, Administrator) mit Marco (Hauptprofil, 18), Sandra (18, PIN, Ausnahme
    → sandra), Noah (12, Ausnahme → kinder), Mia (6, darf nicht anfordern); Jellyseerr-Benutzer 1 marco,
    «Verknüpft über gleiche E-Mail», Passwort «Bleibt in Jellyseerr (Administrator)», kein Limit.
  - Konto `grosseltern@example.ch` mit «Oma & Opa» (16): Benutzer 4 grosseltern, «Von Slouchcat eröffnet»,
    Passwort «Gleich wie in Slouchcat», Limit «Standard» 10 Filme und 5 Staffeln in 7 Tagen; direkt in Jellyseerr
    gestellt sind zwei Filme und zwei Staffeln, also «8 von 10 Filmen frei» und «3 von 5 Staffeln frei».
  - Konto `gast@example.ch` mit «Gast» (16), Konto darf nicht anfordern, «Nicht eröffnet»; in Jellyseerr gibt es
    6 gast mit gleicher E-Mail.
  - Jellyseerr-Benutzer 2 sandra (Auto-Freigabe), 3 kinder (keine Auto-Freigabe, Limit in Jellyseerr 5 Filme
    und 2 Staffeln je 7 Tage), 5 wohnzimmer-tv (`tv@example.ch`, Anmeldung über Jellyfin, gehört keinem Konto).
  - Offene Einladung «Für Tante Ruth» (ab 16, darf anfordern, Limit «Standard»).
  - 82 Titel: 34 aus Fassung 2 (`js/data.js`, davon 20 in der Bibliothek) und 48 für die Seiten «Entdecken»,
    «Filme», «Serien» (`js/data-discover.js`, keiner in der Bibliothek); 23 Titel ohne bekannte Freigabe zählen
    als ab 18. Dazu ein erfundener, nicht zuordenbarer Spotlight-Eintrag «Hafen im Nordlicht» (nur M-27).
  - Wünsche: «Resident Evil» (Marco, In Beschaffung, Download 45 %), «Toy Story 5» (Noah, Freigabe ausstehend),
    «Godzilla Minus One» (Marco, verfügbar, Noah wünscht mit), «Street Fighter» (Noah, abgelehnt), «Black Doves»
    Staffel 1 (Sandra, In Beschaffung, Download mit «6 von 8 Folgen da»), «Coyote vs. Acme» (Sandra,
    fehlgeschlagen), «Clayface» (wohnzimmer-tv, nicht zuordenbar), «Verity» (in Jellyseerr angelegt, Sandra
    zugeordnet, «Download verzögert», ohne Profil-Tag).

## 2. Matrix je Seite

### M-00 · Mockup-Steuerung und Seitenübersicht (nur Mockup)

| Zeile | Inhalt |
|---|---|
| Seite | Keine App-Seite. Route im Mockup `#/mockup` und die Schaltfläche «Mockup» auf jeder Seite |
| Heute | gibt es nicht |
| Geplante Änderung | keine; ersetzt im Mockup Jellyseerr, Radarr, Sonarr und die Zeit |
| Rolle | Marco als Prüfer |
| Neue oder veränderte Einstellungen | keine der App; Mockup-Zustand `S.view`, `S.seerr.condition` (`ok`, `unreachable`, `badkey`, `csrf`), `S.seerr.configured`, `S.seerr.seasonsFrom` (`tmdb`, `tvdb`), `S.arr.radarr.condition`, `S.arr.sonarr.condition`, `seerrUsers[].direct` (direkt in Jellyseerr gestellte Wünsche), `wishes[].dl` (Download-Schritt, `stalled`), `S.session` |
| API-Operationen | keine |
| Mockup | M-00, `#/mockup`, Datei `js/mock.js` |
| Abgedeckte interaktive Zustände | **Ansicht** Desktop, Tablet, Handy. **Wer schaut?** alle Profile mit Altersstufe, darunter Rolle und «darf anfordern als «…»» oder «darf nicht anfordern»; Profilwechsel wechselt das Konto; von einer Verwaltungsseite springt ein Nicht-Administrator auf `#/`. **Jellyseerr ist …** «Verbunden», «Nicht erreichbar», «Schlüssel ungültig», «CSRF-Schutz an», «Nicht eingerichtet» (mit Protokolleintrag); «Abgleich jetzt ausführen» übermittelt wartende Wünsche, holt Tags nach, eröffnet wartende Konten (ein Konto aus einer Einladung erhält dabei bei gleicher E-Mail nur einen Vorschlag) oder meldet «Abgleich nicht möglich». **Limit** zeigt das Limit des Benutzers des aktiven Profils (««grosseltern»: 8 von 10 Filmen frei, 3 von 5 Staffeln frei»; bei Marco «hat kein Limit (Administrator). Wechsle zu «Oma & Opa» …»); «Filme ausschöpfen» und «Staffeln ausschöpfen» legen den Rest als direkt in Jellyseerr gestellte Wünsche an; «Direkte Wünsche leeren». **Radarr und Sonarr** «Radarr erreichbar», «Radarr weg», «Sonarr erreichbar», «Sonarr weg»; «erreichbar» holt offene Tags nach. **Jellyseerr liest Staffeln aus …** «TMDB (ab Werk)», «TVDB» (wirkt auf den Verbindungstest, die Kachel und M-27). **Konten** «Einladung annehmen (Für Tante Ruth)» öffnet M-35; je Konto mit Passwortstand «folgt bei der nächsten Anmeldung» ein Knopf «{Konto} meldet sich an» (übernimmt das Passwort, sonst Meldung «Jellyseerr ist nicht erreichbar …»). **Offene Wünsche vorspulen** je offener Wunsch mit Zustand, Download-Anzeige und Profil: «Übermitteln» (`queued`), «Freigeben» und «Ablehnen» (`pending`), «Datei trifft ein» bzw. «Nächste Staffel trifft ein», «Download weiter», «Download hängt»/«Download läuft wieder», «Scheitern» (`processing`, `partial`), «24 h vergehen» (`queued`: Wunsch `failed` mit Grund «Jellyseerr war 24 Stunden lang nicht erreichbar.», Eingang an Profil und Mitwünscher, Zustellung «Wunsch nicht übermittelt: …» an Administratoren über Kanäle mit «Fehler»). **Zuletzt zugestellt** die letzten vier Zustellungen. «Alle Mockup-Seiten», «Alles zurücksetzen» |
| Spätere technische Umsetzung | wird nicht gebaut |

### M-01 · Startseite mit Spotlight (geändert: nur neue Reihe)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/` |
| Heute | Reihen nach Verwaltung «Startseite»: Spotlight-Bühne mit Posterreihe (nur Titel der Bibliothek, erste Folie vorgemalt), Weiterschauen, Neu hinzugefügt, Kürzlich veröffentlicht, User-Empfehlungen, Merkliste, ausblendbar Sammlungen, Genres, Meine Medien; Posterkarten mit «Gesehen» und «Merken»; Abgleich mit der Trakt-Liste nur über die TMDB-ID |
| Geplante Änderung | Spotlight wie heute: Bühne und Posterreihe nur mit Titeln der Bibliothek, ohne Wunsch-Chip und ohne Staffelhinweis (Marco, E-5; N-70; K-3). Neu erkennt das Spotlight vorhandene Serien auch über TVDB- und IMDb-ID (N-73). Neu die Reihe «Erfüllte Wünsche»: Wünsche des Profils, die in den letzten 14 Tagen erfüllt wurden, ab Werk nach «Weiterschauen» (Empfehlung N-74, F-34). Offene Wünsche stehen nicht auf der Startseite, ihre Zahl steht am Eintrag «Wünsche» der Seitenleiste |
| Rolle | Zuschauer; jedes Profil sieht nur, was seine Altersstufe erlaubt |
| Neue oder veränderte Einstellungen | Platz und Ausblenden der Reihe «Erfüllte Wünsche» (M-28); sonst nur die bestehenden des Spotlights (M-27). Keine Einstellung für fehlende Titel (N-72) |
| API-Operationen | `getHome`, `listHomeRow` (Reihe `wishes` nur `ItemRef`, nie `ExternalTitleRef`; Spotlight neu mit Abgleich über TVDB und IMDb). Zähler der Seitenleiste aus den Wünschen des Profils (`listWishes`; `08` nennt dafür keine eigene Operation) |
| Mockup | M-01, `#/`, `js/home.js` (`route('/')`, `heroHtml` mit `metaLine(t, { stage: true })`, `posterCard`, `mainActions`, `homeRowItems`); Spotlight `spotlightFor` in `js/store.js` (nur `isPresent`); Seitenleiste `js/shell.js` (`viewerNav`) |
| Abgedeckte interaktive Zustände | Bühne mit 20 Titeln der Bibliothek (Marco): blättern, Wahl in der Posterreihe (hält an), «Pause»/«Weiter», Wechsel alle 6 s (steht bei Fokus in der Bühne, offenem Dialog, offener Steuerung); Knöpfe «Abspielen» (Dialog), «Details», «Trailer»; Notiz aus M-27 unter dem Titel. Auf der Bühne erscheint kein fehlender Titel und kein Chip, auch nicht bei «Godzilla Minus One» (erfüllt) und «Tulsa King» (teilweise vorhanden). Reihe «Erfüllte Wünsche» (Marco und Noah: «Godzilla Minus One»; Sandra: keine Reihe), nach «Datei trifft ein» erscheint der Titel dort, auch eine teilweise erfüllte Serie, ohne Staffelhinweis. Mockup › Mia: kein Spotlight (2 Treffer, «Ausblenden unter» 5). Eintrag «Wünsche» der Seitenleiste mit Zähler offener Wünsche; er führt zu M-31 oder, ohne Recht, zu M-08; er fehlt bei Profilen ohne Recht und ohne Wünsche (Mockup › Gast) |
| Spätere technische Umsetzung | Stufe 3: Schritte 3.1 (Trakt-IDs, Migration `spotlight_ids`), 3.2 (`visibility`, `queries/scoped/spotlight.sql`: Filme TMDB, IMDb; Serien TVDB, TMDB, IMDb), 3.3 (`HomeRowKey` + `wishes`, brechend), 3.4 (Reihe `wishes`; das Paket der Startseite bekommt die Titel-IDs vom Dienst `wishes` und prüft über `visibility`, K-16); `05-spotlight-integration.md` Abschnitte 2 und 3; `02-funktionskonzept.md` Abschnitt 5; AK-69, AK-75, AK-78, AK-191, AK-195; Seitenleiste Stufe 2, Schritt 2.9 |

### M-02 · Spotlight-Liste (unverändert gegenüber heute)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/lists/spotlight` |
| Heute | ganze Trakt-Liste als Posterraster, nur Titel der Bibliothek |
| Geplante Änderung | keine im Aufbau; nur Titel der Bibliothek, kein Filter «Noch nicht da» (N-70, Anhang H 8.8). Mehr Treffer durch den Abgleich über TVDB und IMDb (N-73) |
| Rolle | Zuschauer |
| Neue oder veränderte Einstellungen | keine; die bestehenden aus M-27 (ausgeblendete Einträge, Medienart, Spotlight an/aus) |
| API-Operationen | `listHomeRow` für die Reihe `spotlight`, nur `ItemRef` |
| Mockup | M-02, `#/lists/spotlight`, `js/viewer-titles.js` (`route('/lists/spotlight')`) |
| Abgedeckte interaktive Zustände | Raster nur mit Titeln der Bibliothek, die das Profil sehen darf; Unterzeile «Trending bei Trakt · n Titel»; Spotlight aus: «Das Spotlight ist ausgeschaltet.»; nichts in der Bibliothek: «Von dieser Liste ist noch nichts in der Bibliothek.»; Posterkarten mit «Gesehen» und «Merken»; «Zurück». Kein Filter, kein Plus-Knopf |
| Spätere technische Umsetzung | Stufe 3, Schritt 3.2 (gleiche Abfrage wie M-01); `05-spotlight-integration.md` Abschnitt 2; AK-191 |

### M-03 · Vorschauseite Film (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/titles/movie/{tmdbId}` |
| Heute | gibt es nicht; fehlende Titel haben keine Seite |
| Geplante Änderung | Aufbau wie die Detailseite (Hintergrund, Poster, Titel, Metazeile mit Chip «Noch nicht in der Bibliothek» und Freigabe, Genres, Tagline, Beschreibung, Besetzung); Hauptknopf «Film anfordern» oder der Zustand; **Wunschkarte** mit Zustand, Datum, Verlauf und, solange der Wunsch «In Beschaffung» ist, dem Download-Fortschritt (N-82); letzter geschlossener Wunsch mit Datum und Grund; vorhandener Titel leitet auf `/items/{id}` um; nicht sichtbar heisst 404 |
| Rolle | Zuschauer, die den Titel anfordern können; sonst nur mit eigenem Wunsch (F-9, `02-funktionskonzept.md` Abschnitt 5.1) |
| Neue oder veränderte Einstellungen | wirken hier: «Wünsche aktiv», «Filme erlauben», «Zurückziehen erlauben», «Bereits angeforderte Titel», «Download-Fortschritt zeigen» (M-25); die Seite antwortet 404, wenn «Anfordern aus der Suche», «Anfordern auf Detailseiten» und «Seiten Entdecken, Filme, Serien und Suche» alle aus sind (ausser bei eigenem Wunsch) |
| API-Operationen | `getExternalTitle` (mit `lastWish?`, `wish.progress?`; 404 ohne Recht ausser bei eigenem Wunsch; 409 `already_available`), `getExternalTitleImage`, `createWish`, `withdrawWish`, `getWishQuota` (für M-13) |
| Mockup | M-03, `#/titles/movie/1368337` («Odyssey»); mit Download `#/titles/movie/1423191` («Resident Evil», Marco), mit «Download verzögert» `#/titles/movie/1283515` («Verity», Mockup › Sandra); `js/viewer-titles.js` (`route('/titles/:media/:tmdbId')`, `vtPreviewVisible`, `vtStatusBlock`, `vtWishCard`), Fortschritt `dlBlock` in `js/model2.js` |
| Abgedeckte interaktive Zustände | ohne Wunsch: Hinweis «Noch nicht in der Bibliothek» und «Film anfordern» (öffnet M-13); eigener Wunsch: Wunschkarte «Dein Wunsch» mit Chip, Verlauf (erledigt, laufend, ausstehend), «Zu meinen Wünschen», «Zurückziehen» bei `queued` und `pending`; Zusatz bei `queued` (etwa «Jellyseerr ist nicht erreichbar. Slouchcat versucht es weiter.»); **Download** auf der Karte: «Wird gesucht», «Wird heruntergeladen · 45 % · noch 12 min» mit Balken, «Wird eingelesen», «Download verzögert», je mit «Stand aus Radarr, vor weniger als einer Minute gelesen.»; bei «Radarr weg» oder Schalter aus kein Zusatz (der Chip bleibt «In Beschaffung»); Mitwunsch: «du wünschst mit» und «Nicht mehr mitwünschen» (Dialog); fremder Wunsch: Hinweis «Bereits angefordert» und «Benachrichtige mich», bei «Nur Zustand zeigen» ohne Knopf; letzter eigener Wunsch geschlossen: «Dein letzter Wunsch: Abgelehnt/Fehlgeschlagen/Zurückgezogen» mit Datum und Grund (Mockup › Noah, `#/titles/movie/1153576`); ohne Recht mit eigenem Wunsch: «Anfordern ist für dieses Profil nicht möglich» mit Grund, für Administratoren ohne Jellyseerr-Benutzer «Jellyseerr-Benutzer öffnen»; ohne Recht und ohne eigenen Wunsch sowie über der Altersstufe: «Titel nicht gefunden» (Mockup › Noah, `#/titles/movie/1423191`); vorhanden: Umleitung ohne Verlaufseintrag (`#/titles/movie/940721` nach `#/items/godzilla-minus-one`); «Trailer» als Dialog; Besetzung blättern; «Zurück» |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.2 (`titles.refresh`, Bild-Handler), 2.3 (`visibility`: `MissingTitle`, `TitleByExternalIDs`, Regel F-9), 2.8 (`web/src/features/titles/`); Stufe 4: Schritte 4.6d (`wish_progress`, `wishes.progress`; liest Warteschlange und Dateistand, N-81 nach K-5), 4.6e (Fortschritt auf der Wunschkarte); `02-funktionskonzept.md` Abschnitte 5.1 und 5.3; AK-197 bis AK-200 |

### M-04 · Vorschauseite Serie (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/titles/show/{tmdbId}` |
| Heute | gibt es nicht |
| Geplante Änderung | wie M-03, dazu Staffelwand mit gedämpften Karten je Staffel und Zustand; «Läuft noch · nächste Folge am …»; Hauptknopf «Serie anfordern»; Staffeln im Dialog aus Jellyseerr (N-63); Serie ohne TVDB-ID ohne Knopf mit Hinweis (AK-39); bei Serien im Fortschritt «x von y Folgen da» (N-83) |
| Rolle | wie M-03 |
| Neue oder veränderte Einstellungen | zusätzlich «Serien erlauben» (M-25); der Jellyseerr-Benutzer muss Serien anfordern dürfen |
| API-Operationen | `getExternalTitle` (`seasons[]`, `wish.progress?`), `getWishSeasons` (Staffeln aus Jellyseerr), `createWish` mit `seasons`, `withdrawWish`, `getWishQuota` |
| Mockup | M-04, `#/titles/show/291507` («The Body»); weitere: `#/titles/show/225385` («Black Doves», Staffel 1 von Sandra angefordert, mit «6 von 8 Folgen da»), `#/titles/show/73223` («Black Clover», zwei Staffeln), für das Limit `#/titles/show/1100` («How I Met Your Mother», neun Staffeln); `js/viewer-titles.js` (`vtGhost`, `vtSeasonState`) |
| Abgedeckte interaktive Zustände | Staffelkarten mit Band «Fehlt» (klickbar, öffnet M-13 mit genau dieser Staffel), eigener Zustand (führt zu `#/wishes`), «Bereits angefordert» (fremd, auf der Karte nicht klickbar, im Dialog wählbar), «Angekündigt»; «Läuft noch · nächste Folge am 19. November 2026»; Marco bei «Black Doves»: Staffel 1 «Bereits angefordert», Staffel 2 «Fehlt»; Mockup › Sandra bei «Black Doves»: Wunschkarte mit «Wird heruntergeladen · 12 % · 6 von 8 Folgen da · noch 38 min»; alle Zustände von M-03 |
| Spätere technische Umsetzung | Stufe 2, Schritte 2.2, 2.3, 2.4 (`getWishSeasons`), 2.8; Stufe 4, Schritte 4.6d, 4.6e; `02-funktionskonzept.md` Abschnitte 5.1 und 6; AK-39, AK-201 |

### M-05 · Detailseite Film (fast unverändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/items/{id}` |
| Heute | Hintergrund, Poster mit Merken, «Abspielen», runde Knöpfe «Trailer», «Von vorne», «Merken», «Gesehen», «Empfehlen» (Besitzerkonto), «Löschen» (Administrator), Besetzung |
| Geplante Änderung | nur der Chip «Dein Wunsch ist da» 14 Tage nach der Erfüllung; er steht nur hier, nie auf der Bühne (`02-funktionskonzept.md` Abschnitt 5, K-3). Kein Download-Fortschritt an Titeln der Bibliothek (N-82) |
| Rolle | Zuschauer |
| Neue oder veränderte Einstellungen | keine |
| API-Operationen | `getItem` (unverändert); der Chip braucht den erfüllten Wunsch des Profils (`listWishes`) |
| Mockup | M-05, `#/items/godzilla-minus-one` (mit erfülltem Wunsch); ohne Wunsch `#/items/masterplan`; `js/viewer-titles.js` (`route('/items/:key')`), Chip in `js/home.js` (`fulfilledRecently`, `metaLine`) |
| Abgedeckte interaktive Zustände | «Abspielen» und «Von vorne» öffnen den Platzhalter des Players; «Trailer» als Dialog; «Merken» und «Gesehen» schalten um; «Empfehlen» mit Meldung «Bleibt 30 Tage»; «Löschen» öffnet den unveränderten Dialog und löscht nichts; Chip «Dein Wunsch ist da» bei Marco und Noah auf «Godzilla Minus One»; Titel über der Altersstufe gibt «Titel nicht gefunden» |
| Spätere technische Umsetzung | Stufe 3, Schritt 3.4 zusammen mit der Reihe; AK-131 |

### M-06 · Detailseite Serie mit fehlenden Staffeln (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/items/{id}` |
| Heute | Staffelwand der vorhandenen Staffeln, Folgenliste der gewählten Staffel mit Gesehen je Folge, runde Knöpfe wie M-05 |
| Geplante Änderung | nur für Profile, die anfordern können: fehlende Staffeln als gedämpfte Karten, runder Knopf «Staffeln anfordern», Chip «2 von 3 Staffeln vorhanden», Zeile mit fehlenden und angeforderten Staffeln und Link «Meine Wünsche»; «Läuft noch, nächste Folge am …» oder «Staffel 4 ist angekündigt». Kein Download-Fortschritt (N-82) |
| Rolle | Zuschauer; ohne Recht wie heute |
| Neue oder veränderte Einstellungen | wirken hier: «Anfordern auf Detailseiten», «Serien erlauben», «Wünsche aktiv», «Bereits angeforderte Titel» (M-25) |
| API-Operationen | `getItem` (neu `catalog`, `canRequestSeasons`), `getWishSeasons` (auch für eine vorhandene Serie, K-4), `createWish` mit `seasons`, `getWishQuota` |
| Mockup | M-06, `#/items/tulsa-king`; weitere `#/items/rings-of-power`, `#/items/dexter-resurrection`, `#/items/black-clover` (nach Anfordern und «Datei trifft ein»); `js/viewer-titles.js` (`route('/items/:key')`, `vtMissingLine`, `vtGhost`, `vtEpisodes`) |
| Abgedeckte interaktive Zustände | Staffel 1 und 2 vorhanden und wählbar (Folgenliste, Gesehen je Folge, Folge abspielen); Staffel 3 «Fehlt» (Klick öffnet M-13 mit Staffel 3), Staffel 4 «Angekündigt»; Chip «2 von 3 Staffeln vorhanden» in der Metazeile; Zeile «Staffel 3 fehlt.»; nach dem Anfordern «Staffel 3 ist angefordert: In Beschaffung.» mit Link; «Staffeln anfordern» verschwindet, wenn keine Staffel frei ist; «Anfordern auf Detailseiten» aus: keine gedämpften Karten, keine Zeile, kein Chip, kein Knopf; Mockup › Gast: nur vorhandene Staffeln; Mockup › Mia: «Titel nicht gefunden» (ab 16) |
| Spätere technische Umsetzung | Stufe 4: Schritte 4.1 (`series_catalog`, `series.catalog`), 4.2 (`ItemPage.tsx`, `SeasonShelf.tsx`, Staffelwahl mit Mitwunsch); `02-funktionskonzept.md` Abschnitt 6; AK-80, AK-132 |

### M-07 · Suche (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/search` |
| Heute | Suchfeld (ab 2 Zeichen), Filter Genre, Jahr von/bis, Altersfreigabe höchstens, Medientyp, Verfügbarkeit «Auf dem Server»/«Auf diesem Gerät», Ergebnisse «Titel», «Personen», «Sammlungen» |
| Geplante Änderung | Abschnitt «Noch nicht in der Bibliothek» aus der TMDB-Suche, Verfügbarkeitsfilter «Noch nicht da»; neu ein Verweis auf die Wunsch-Suche M-34 mit demselben Suchbegriff (N-57). Kein Download-Fortschritt (N-82) |
| Rolle | Zuschauer; Abschnitt, Filter und Verweis nur für Profile, die anfordern können |
| Neue oder veränderte Einstellungen | wirken hier: «Anfordern aus der Suche», «Seiten Entdecken, Filme, Serien und Suche» (Verweis), «Wünsche aktiv», «Filme erlauben», «Serien erlauben» (M-25) |
| API-Operationen | `search` (unverändert), `searchExternalTitles`, `getExternalTitleImage` |
| Mockup | M-07, `#/search?q=the`, `js/viewer-pages.js` (`route('/search')`) |
| Abgedeckte interaktive Zustände | Eingabe mit Live-Ergebnis; ein Zeichen: «Gib mindestens zwei Zeichen ein.»; «Titel» und darunter «Noch nicht in der Bibliothek» mit Plus-Knöpfen und dem Link «Alle Treffer unter Wünsche suchen» (nur mit Suchbegriff, wenn es fehlende Treffer gibt und die Seiten an sind); Filter aufklappbar mit Zusammenfassung und «Filter zurücksetzen»; Pille «Noch nicht da» zeigt fehlende Titel auch ohne Suchbegriff; Personen und Sammlungen führen zur Hinweisseite «Unverändert»; «Nichts gefunden»; Mockup › Mia oder Gast: weder Abschnitt noch Pille noch Verweis; «Anfordern aus der Suche» aus: Abschnitt, Pille und Verweis fehlen; «Seiten Entdecken …» aus: nur der Verweis fehlt |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.3 (Suche über TMDB gefiltert in `visibility`), 2.8 (`SearchPage.tsx`); Verweis Stufe 5, Schritt 5.5; `02-funktionskonzept.md` Abschnitt 5.2; AK-187 |

### M-08 · Wünsche · Meine Wünsche (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/wishes`; fünfter Reiter des Bereichs «Wünsche» (N-50); Eintrag «Wünsche» der Seitenleiste nach «Gesehen» mit Zähler offener Wünsche |
| Heute | gibt es nicht |
| Geplante Änderung | Reiter «Entdecken», «Filme», «Serien», «Suche», «Meine Wünsche» (ohne Recht nur «Meine Wünsche»); oben das Limit des Kontos (02 Abschnitt 4.10); Liste der eigenen Wünsche und Mitwünsche mit Zustand, Filter, Verlauf, Zurückziehen, Nicht mehr benachrichtigen, Erneut anfordern; Download-Fortschritt bei Wünschen in Beschaffung, ohne Meldung von Radarr oder Sonarr (N-82, N-83); für das Hauptprofil umschaltbar auf alle Profile des Kontos |
| Rolle | jedes Profil mit Wünschen oder Recht; Hauptprofil zusätzlich alle Profile des Kontos |
| Neue oder veränderte Einstellungen | wirken hier: «Zurückziehen erlauben», «Bereits angeforderte Titel», «Download-Fortschritt zeigen», «Seiten Entdecken, Filme, Serien und Suche» (Reiter) (M-25); Limit des Kontos (M-26) bzw. des Ausnahme-Benutzers |
| API-Operationen | `listWishes`, `listAccountWishes` (`scoped`, nur Hauptprofil, sonst 403 `primary_profile_required`), beide mit `progress?`; `withdrawWish`, `createWish` (über «Erneut anfordern»), `getWishQuota` |
| Mockup | M-08, `#/wishes`, Sprung aus M-09 mit `#/wishes?focus={id}`; `js/viewer-pages.js` (`route('/wishes')`, `vpWishRow`, `vpSteps`); Reiter und Limit `wishNav`, `quotaPill` in `js/model2.js`; Fortschritt `dlChip` |
| Abgedeckte interaktive Zustände | Reiter mit Zähler an «Meine Wünsche»; Limit oben: Marco «Limit keines (Administrator)», Mockup › «Oma & Opa» je ein Balken «Filme 2 / 10 in 7 Tagen» und «Staffeln 2 / 5 in 7 Tagen», nach «Filme ausschöpfen» voll; Filter «Alle», «Offen», «Verfügbar», «Abgeschlossen» mit Zählern; Schalter «Wünsche aller Profile meines Kontos» (nur Hauptprofil, mit Profilname); je Zeile Chip, **Download-Zeile** («Wird gesucht», «Wird heruntergeladen · 45 % · noch 12 min», «Wird eingelesen», «Download verzögert», bei «Black Doves» mit «6 von 8 Folgen da»; bei «Radarr weg» oder Schalter aus nichts), «Du wünschst mit», «In Jellyseerr angelegt», Klartext bei `queued`, `failed`, `pending`; Staffel-Chips bei Serien; «Abspielen» und «Zum Titel» bei `available` und `partial`; «Zurückziehen» mit Rückfrage; «Nicht mehr benachrichtigen»; «Erneut anfordern» bei `declined`, `failed`, `withdrawn` (öffnet M-13); «Verlauf» aufklappbar; `?focus=` klappt auf und scrollt hin; leer mit Recht: «Noch keine Wünsche» mit «Unter «Entdecken», «Filme», «Serien» und in der Suche findest du Titel …» und den Knöpfen «Zu «Entdecken»» und «Zur Suche»; leer ohne Recht: «Keine Wünsche» mit Grund (Mockup › Mia); mit Wünschen ohne Recht: «Neue Wünsche sind nicht möglich»; Übergabe an Mitwünscher beim Zurückziehen |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.4 (`createWish`, `withdrawWish`, `getWishQuota`), 2.9 (`web/src/features/wishes/`, `AppLayout.tsx`); Stufe 4: Schritt 4.6e (Fortschritt); Stufe 5: Schritt 5.5 (Reiter); Tabellen `wishes`, `wish_seasons`, `wish_requesters`, `wish_events`, `wish_progress`; `02-funktionskonzept.md` Abschnitte 4.8, 4.10 und 5.3 |

### M-09 · Benachrichtigungen (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/notifications`; Glocke im Kopf mit Zähler |
| Heute | Eingang des Profils mit «Neue Folge» je Serie der Merkliste, gelesen/ungelesen, «Alle als gelesen markieren», «Kanäle einstellen» |
| Geplante Änderung | neue Eingangsart `wish_available` (Stufe 2); `wish_season_available`, `wish_approved`, `wish_declined`, `wish_failed` (Stufe 4); Link zum Titel oder zu `/wishes?focus={id}`. Keine Meldung zu Limit oder Download-Fortschritt (`06-benachrichtigungen.md` Abschnitt 4) |
| Rolle | jedes Profil; nur Anforderer und Mitwünscher, je erst wenn der Titel für dieses Profil sichtbar ist |
| Neue oder veränderte Einstellungen | keine; der Eingang ist immer an |
| API-Operationen | `listNotifications` (brechend: `item` optional, neu `wish`), bestehende Operationen zum Lesen |
| Mockup | M-09, `#/notifications`, `js/viewer-pages.js` (`route('/notifications')`), Texte `N_TEXT` in `js/store.js` |
| Abgedeckte interaktive Zustände | ungelesen hervorgehoben und im Zähler der Glocke; Klick markiert gelesen und springt zum Titel bzw. zu M-08 mit `focus`; «Als gelesen markieren»/«Als ungelesen markieren»; «Alle als gelesen markieren»; leer: «Keine Benachrichtigungen» (Mockup › Mia); neue Meldungen beim Vorspulen (Freigeben, Ablehnen, Scheitern, 24 h vergehen, Datei trifft ein) mit Meldung am Bildschirm, über welche Kanäle zugestellt wurde. «Download weiter» und «Download hängt» erzeugen keine Meldung. Alle Arten erscheinen ohne Stufenhinweis (Abschnitt 9) |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.6, 2.9; Stufe 4: Schritt 4.3; Tabelle `notifications`; `06-benachrichtigungen.md` Abschnitte 2, 3, 7 und 10 |

### M-10 · Benachrichtigungskanäle (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/settings/notifications` |
| Heute | je Kanal (E-Mail, Telegram, Web-Push) ein Band mit Zustand, «Aktiv», Ereignissen «Neue Inhalte», «Fehler» (nur Administratoren), «Einladungen», «Ankündigungen», Adresse bzw. Verknüpfung bzw. Geräte, «Speichern», «Testnachricht»; ändern nur im Hauptprofil |
| Geplante Änderung | drei neue Häkchen je Kanal: «Wünsche: verfügbar» (Stufe 2), «Wünsche: freigegeben, abgelehnt, fehlgeschlagen» (Stufe 4, ab Werk aus), nur Administratoren «Wünsche: Freigabe nötig» (Stufe 4); Hinweis «Wünsche und Profile»; Kopf zählt die Ereignisse (`07-administration.md` Abschnitt 11) |
| Rolle | Hauptprofil (ändern), weitere Profile (lesen) |
| Neue oder veränderte Einstellungen | Ereignisse `wish_available`, `wish_status`, `wish_review` je Kanal |
| API-Operationen | bestehende Kanal-Operationen; `EventType` erweitert (brechend) |
| Mockup | M-10, `#/settings/notifications`, `js/viewer-pages.js` (`channelSettingsHtml`, `VP_EVENTS`) |
| Abgedeckte interaktive Zustände | Bänder zugeklappt mit Zusammenfassung (ohne Zahl der Ereignisse); Häkchen wirken sofort auf spätere Zustellungen; Mockup › «Oma & Opa»: ohne «Fehler» und «Wünsche: Freigabe nötig»; Mockup › Sandra: alles gesperrt mit «Nur im Hauptprofil» und «Profil wechseln»; «Testnachricht» mit Warnung bei aus, nicht verknüpft oder ohne Gerät; Telegram «Verknüpfen», «Verknüpfung prüfen», «Code erneuern», «Code verwerfen», «Verknüpfung lösen»; Web-Push «Auf diesem Gerät aktivieren», «Entfernen» |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.6, 2.9; Stufe 4: Schritt 4.3; Tabellen `account_channels`, `deliveries`; `06-benachrichtigungen.md` Abschnitte 5 und 6 |

### M-11 · Profile (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/settings/profiles` |
| Heute | Karten je Profil mit Chips, «Bearbeiten», «Schauzeit anzeigen», Menü «PIN setzen/ändern/entfernen», «Löschen»; Profildialog mit Name, Altersfreigabe, Sprache, Sprache der Beschreibungen, Farbe, Bibliotheken, Wiedergabe, Wiedergabe-Limits, PIN beim Anlegen; nur Hauptprofil |
| Geplante Änderung | Profildialog: Abschnitt «Wünsche» mit «Darf Titel anfordern», gesperrt mit Hinweis, wenn das Konto nicht darf; Karte: Chip «Darf anfordern»/«Darf nicht anfordern» und Zeile «Sendet als {Benutzer} (…)»; ein gelöschtes Profil gibt seine Wünsche an das Hauptprofil ab (`04-benutzer-profilmapping.md` Abschnitt 10). Der Profil-Tag und die Ausnahme werden in M-24 gesetzt, nicht hier |
| Rolle | Hauptprofil des Kontos |
| Neue oder veränderte Einstellungen | Wunschrecht des Profils (`profiles.can_request`) |
| API-Operationen | `createProfile`, `updateProfile`, `listProfiles` (Feld `canRequest`) |
| Mockup | M-11, `#/settings/profiles`, `js/viewer-pages.js` (`vpProfCard`, `DIALOGS.vpProf`, `ACT.vpProfDelOk`) |
| Abgedeckte interaktive Zustände | Schalter an/aus und speichern; neues Profil startet mit «aus»; Mockup › Gast: Schalter gesperrt mit «Das Konto darf nicht anfordern. Das stellt der Administrator ein.»; Mockup › Noah: «Nur im Hauptprofil»; Zeile «Sendet als» mit «eigener Jellyseerr-Benutzer» (Ausnahme), «Benutzer des Kontos», «Sammelbenutzer» oder «Kein Jellyseerr-Benutzer»; Profil löschen (etwa Noah): Meldung «… 1 Wunsch gehört jetzt Marco.», im Verlauf «Von gelöschtem Profil Noah übernommen: Marco», Mitwünsche des Profils fallen weg; ein neues Profil mit dem Namen eines Profils in einem anderen Konto bekommt den Tag mit Kontonamen (`sc-noah-grosseltern`, N-21 nach K-18); alle heutigen Teile bedienbar (Pflichtfeld Name, PIN 4 bis 8 Ziffern, Schauzeit, Löschen mit Rückfrage) |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.2 (`wishes_rights`), 1.6 (Deckelregel), 1.9 (`web/src/features/profiles/`); `04-benutzer-profilmapping.md` Abschnitte 9 und 10 |

### M-12 · Profilwahl (unverändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/profiles` |
| Heute | «Wer schaut?» mit Profilkacheln, PIN-Abfrage, «Profile verwalten» |
| Geplante Änderung | keine |
| Rolle | Konto ohne gewähltes Profil |
| Neue oder veränderte Einstellungen | keine |
| API-Operationen | Sitzung (`getSession`: `wishes` als `WishRights`, Stufe 5 `wishes.discover`) |
| Mockup | M-12, `#/profiles`, `js/viewer-pages.js` (`route('/profiles')`) |
| Abgedeckte interaktive Zustände | Profil wählen; PIN-Dialog (jede vierstellige PIN öffnet); Abschnitt «Andere Konten (nur im Mockup)» zum Kontowechsel, auch zu Konten aus M-35 |
| Spätere technische Umsetzung | Stufe 1, Schritt 1.6; Stufe 5, Schritt 5.4 |

### M-13 · Dialog «Anfordern» (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, Dialog auf M-03, M-04, M-06, M-07, M-08 («Erneut anfordern»), M-27 («Anfordern» des Administrators) und M-31 bis M-34 (`02-funktionskonzept.md` Abschnitt 5) |
| Heute | gibt es nicht |
| Geplante Änderung | ein Dialog: Poster, Titel, «Wird als {Benutzer} an Jellyseerr übermittelt», «Wird sofort beschafft» oder «Braucht eine Freigabe», **Limit** (frei, erreicht mit Zeitpunkt des nächsten Platzes, zu viele Staffeln; N-36), Kanäle der Meldung, bei Serien Staffelwahl (aus Jellyseerr, N-63) mit Mitwunsch je Staffel; «Anfordern»/«Abbrechen» (02 Abschnitt 5.5) |
| Rolle | Profil, das anfordern kann; in M-27 der Administrator mit seinem Hauptprofil |
| Neue oder veränderte Einstellungen | wirken hier: «Bereits angeforderte Titel», «Konten ohne Jellyseerr-Benutzer» mit Sammelbenutzer (M-25), Limit ab Werk (M-25) und je Konto (M-26), Ausnahme je Profil (M-24), «Wünsche: verfügbar» (M-10) |
| API-Operationen | `createWish` (201 neu, 200 bestehend oder Mitwunsch; 403 `wish_not_allowed` mit `reason`; 404; 409 `already_available`; 409 `quota_exceeded`), `getWishQuota`, `getWishSeasons` |
| Mockup | M-13, kein eigener Hash; `js/home.js` (`ACT.askWish` mit Kappung der Vorwahl, `DIALOGS.wish`, `ACT.confirmWish`), `ACT.vtAskSeason` in `js/viewer-titles.js`, `requestWish` in `js/store.js`, `limitCheck`, `quotaFor` in `js/model2.js` |
| Abgedeckte interaktive Zustände | Film neu: Fakten «Wird als marco an Jellyseerr übermittelt (Benutzer des Kontos).» und «Anfordern»; Fakt Limit: «Limit: 8 von 10 Filmen frei (10 in 7 Tagen, für alle Profile dieses Kontos gemeinsam)» (Mockup › «Oma & Opa»), «Kein Limit: Administratoren fordern unbegrenzt an.» (Marco), «Kein Limit.» (Sandra), Limit des Ausnahme-Benutzers «4 von 5 Filmen frei» (Mockup › Noah); **Limit erreicht** («Filme ausschöpfen»): Band «Limit erreicht» mit «Das Limit ist erreicht: 10 Filme in 7 Tagen. Der nächste Platz wird am {Tag} um {Zeit} frei.», «Anfordern» gesperrt; Film schon angefordert: «Schon angefordert» mit «Benachrichtige mich», auch bei erreichtem Limit; Serie: «Alle fehlenden Staffeln (n)» und einzelne Staffeln mit «Vorhanden», «Angekündigt», «Bereits angefordert» oder eigenem Zustand; **gekappte Vorwahl**: reicht das Limit nicht, sind nur so viele neue Staffeln vorgewählt, wie frei sind, mit Hinweis «Das Limit reicht nicht für alle Staffeln … Vorgewählt sind die ersten 3 fehlenden Staffeln» (Mockup › «Oma & Opa», «How I Met Your Mother»; Mockup › Noah «Modern Family» mit 2); **zu viele Staffeln**: «Zu viele Staffeln für das Limit» mit «Noch 3 von 5 Staffeln frei. Wähle höchstens 3 Staffeln.», Knopf gesperrt; Staffeln, die schon jemand angefordert hat, kosten nichts; Knopf «n Staffeln anfordern», «Benachrichtige mich» (nur fremd angeforderte gewählt) oder gesperrt «Staffeln wählen»; «Nur Zustand zeigen»: fremd angeforderte Staffeln gesperrt; «Nichts mehr anzufordern»; Jellyseerr nicht erreichbar: «Dein Wunsch wird vorgemerkt»; kein Recht: «Anfordern nicht möglich» mit Grund (nur über einen eigenen Wunsch erreichbar, F-9); Rückmeldung je Ergebnis mit Link «Meine Wünsche» |
| Spätere technische Umsetzung | Stufe 2: Schritte 2.4 (`createWish` mit Vorprüfung des Limits und `quota_exceeded`, `getWishQuota`, Staffeln aus Jellyseerr), 2.8 (`web/src/ui/`, Dialog mit Limit); Stufe 4: Schritt 4.2; `02-funktionskonzept.md` Abschnitte 4.1 bis 4.5, 4.10 und 5.5; `04-benutzer-profilmapping.md` Abschnitt 17; AK in `10-abnahmekriterien.md` Abschnitt 3.8 |

### M-31 · Wünsche · Entdecken (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/discover` (K-13); erster Reiter des Bereichs «Wünsche», Ziel des Eintrags «Wünsche» der Seitenleiste |
| Heute | gibt es nicht |
| Geplante Änderung | Reihen wie die Startseite von Jellyseerr (Marco, Nachricht): Im Trend, Beliebte Filme, Film-Genres, Demnächst im Kino, Beliebte Serien, Serien-Genres, Demnächst als Serie (Auswahl Empfehlung N-51, F-28); Suchfeld; Bibliothekstitel mit Marke und Link zur Detailseite (N-55); keine Reihe mit fehlenden Einträgen des Spotlights (E-5) |
| Rolle | nur Profile, die anfordern können (F-9, N-56); alle anderen sehen höchstens «Meine Wünsche» |
| Neue oder veränderte Einstellungen | «Seiten Entdecken, Filme, Serien und Suche» (M-25); wirken auch «Filme erlauben», «Serien erlauben», «Wünsche aktiv»; Zeitplan `wishes.discover` (M-30) |
| API-Operationen | `getWishDiscover`, `listWishDiscoverRow`, `getExternalTitleImage`, `createWish`, `getWishQuota`; Sitzung `wishes.discover` |
| Mockup | M-31, `#/discover`, `js/viewer-discover.js` (`route('/discover')`, `dcGenreRow`, `dcGate`); Reihen `DISC_ROWS`, `discoverRow` in `js/model2.js`, Daten `DISCOVER_ROWS` in `js/data-discover.js`; Marke über `posterCard` mit `libMark` in `js/home.js` |
| Abgedeckte interaktive Zustände | Reiter und Limit oben; Suchfeld «Film oder Serie suchen» mit «Suchen» oder Enter → M-34; Reihen in der Reihenfolge «Im Trend», «Beliebte Filme» (Kopf führt zu M-32), «Film-Genres» als Kacheln mit Zahl (führt zu M-32 mit Genre), «Demnächst im Kino» (M-32 «Neueste zuerst»), «Beliebte Serien», «Serien-Genres», «Demnächst als Serie»; je Reihe blättern; Poster: fehlend mit Plus und «Noch nicht da», fremd angefordert mit Glocke, eigener Wunsch mit Zustands-Chip, vorhanden mit Unterzeile «Jahr · In der Bibliothek» sowie «Gesehen» und «Merken»; Jellyseerr nicht verbunden: Band «Jellyseerr ist nicht erreichbar. Du siehst den Stand vom letzten Abruf.»; Mockup › Noah: nur Titel bis 12 mit bekannter Freigabe; Profil ohne Recht ohne Wünsche (Mockup › Mia): «Titel nicht gefunden»; ohne Recht mit Wünschen oder bei ausgeschalteten Seiten: Seite «Entdecken gibt es für dieses Profil nicht» mit «Zu «Meine Wünsche»» |
| Spätere technische Umsetzung | Stufe 5: Schritte 5.1 (`wish_discover_pages`), 5.2 (Discover-Routen von Jellyseerr), 5.3 (`wishes.discover`, Nachprüfung je Treffer), 5.4 (Vertrag), 5.5 (`web/src/features/wishes/`); `02-funktionskonzept.md` Abschnitte 4.11 und 5.4; `05-spotlight-integration.md` Abschnitt 3.3 (Abgleich); AK-179 bis AK-183, AK-186, AK-188, AK-192 |

### M-32 · Wünsche · Filme (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/discover/movies` |
| Heute | gibt es nicht |
| Geplante Änderung | Posterraster aller Filme, die TMDB kennt, mit Sortierung (Beliebtheit, Neueste zuerst, Titel), Genre-Filter und «Vorhandene ausblenden»; der Filter ist zugeklappt und zeigt seine Wahl im Kopf; «Mehr laden» (N-52) |
| Rolle | wie M-31 |
| Neue oder veränderte Einstellungen | wie M-31; Sortierung und Filter sind Ansicht, keine Einstellung |
| API-Operationen | `listWishTitles` (`media=movie`, `sort`, `genre`, `hideAvailable`, `page`), `getExternalTitleImage`, `createWish`, `getWishQuota` |
| Mockup | M-32, `#/discover/movies`, mit Vorgabe `#/discover/movies?genre=Drama&sort=new`; `js/viewer-discover.js` (`dcGridPage('movie')`), `discoverList` in `js/model2.js` |
| Abgedeckte interaktive Zustände | Band «Filter und Sortierung» zugeklappt mit Kopf «Beliebtheit · Alle Genres · Vorhandene sichtbar»; Auswahl «Sortierung» (Beliebtheit, Neueste zuerst, Titel A–Z), «Genre», Schalter «Vorhandene ausblenden» mit Hilfe; «Filter zurücksetzen»; Vorgaben aus dem Link als Pillen «Genre: …», «Sortierung: …» mit Entfernen; Zählung «n Filme, davon k in der Bibliothek»; 24 je Seite, «Mehr laden» mit «x von y angezeigt»; leer: «Keine Filme für diese Auswahl» mit «Filter zurücksetzen»; Poster wie M-31, vorhandene mit «In der Bibliothek»; Jellyseerr nicht verbunden: Band mit letztem Stand; Zugang wie M-31 |
| Spätere technische Umsetzung | Stufe 5: Schritte 5.2, 5.3, 5.4, 5.5; `02-funktionskonzept.md` Abschnitt 5.4; AK-185, AK-188 |

### M-33 · Wünsche · Serien (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/discover/series` |
| Heute | gibt es nicht |
| Geplante Änderung | wie M-32 für Serien; Bibliotheksserien über TVDB zuerst erkannt (N-61) |
| Rolle | wie M-31 |
| Neue oder veränderte Einstellungen | wie M-32 |
| API-Operationen | `listWishTitles` (`media=show`), `getExternalTitleImage`, `createWish`, `getWishSeasons`, `getWishQuota` |
| Mockup | M-33, `#/discover/series`, `js/viewer-discover.js` (`dcGridPage('series')`) |
| Abgedeckte interaktive Zustände | alle Zustände von M-32 mit «Serien»; vorhandene Serien («MobLand», «Tulsa King», «Die Ringe der Macht», «Dexter: Wiedererwachen») mit «In der Bibliothek», «Gesehen» und «Merken»; Plus öffnet M-13 mit Staffelwahl |
| Spätere technische Umsetzung | Stufe 5: Schritte 5.2 bis 5.5; AK-183, AK-185, AK-188 |

### M-34 · Wünsche · Suche (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/discover/search` |
| Heute | gibt es nicht |
| Geplante Änderung | Suche in allem, was TMDB kennt, auch nach Kennung `tmdb:`, `tvdb:`, `imdb:`; keine Personen (N-53) |
| Rolle | wie M-31 |
| Neue oder veränderte Einstellungen | wie M-31 |
| API-Operationen | `searchWishTitles` (`q` 2–100 Zeichen), `getExternalTitleImage`, `createWish`, `getWishQuota` |
| Mockup | M-34, `#/discover/search?q=the`, `js/viewer-discover.js` (`route('/discover/search')`, `dcHit`, `dcChip`, `dcExamples`), `discoverSearch` in `js/model2.js` |
| Abgedeckte interaktive Zustände | Suchfeld mit Fokus beim Öffnen und Live-Ergebnis; ein Zeichen: «Gib mindestens zwei Zeichen ein.»; Kennungen als klickbare Beispiele «tmdb:1368337 · tvdb:413215 · imdb:…» füllen das Feld; ohne Begriff «Gib einen Titel ein …» und «Gerade beliebt» (6 Poster); Treffer getrennt «Filme (n)» und «Serien (n)», je Zeile Poster, Titel, «Jahr · Film · ab 14» oder «Freigabe unbekannt, zählt als ab 18», Originaltitel, Chip («In der Bibliothek», «2 von 3 Staffeln vorhanden», eigener Zustand, «Bereits angefordert», «Noch nicht in der Bibliothek») und Knöpfe («Film anfordern», «Benachrichtige mich», Zustand oder «Abspielen», dazu «Trailer»); 0 Treffer: «Nichts gefunden für «…»» mit «nicht nach Personen»; Jellyseerr nicht verbunden: Feld gesperrt, Band «Die Suche fragt Jellyseerr direkt …» mit «Zu «Entdecken»»; Zugang wie M-31 |
| Spätere technische Umsetzung | Stufe 5: Schritte 5.2 (Präfixe), 5.3, 5.4, 5.5; AK-184, AK-188 |

### M-35 · Einladung annehmen (heute vorhanden, im Mockup neu)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/invite/$token` (`web/src/app/router.tsx:237`); im Mockup `#/invite/{id}` |
| Heute | Seite der Einladung; legt mit `acceptInvitation` Konto und Hauptprofil an, unter anderem mit E-Mail und Passwort (Anhang H 3.6, `02-funktionskonzept.md` Abschnitt 4.9) |
| Geplante Änderung | über dem Formular die Fakten der Einladung (Notiz, Altersstufe, Wunschrecht mit Limit, gültig bis); Hinweis «Dein Login gilt auch für Jellyseerr»; beim Absenden eröffnet Slouchcat im selben Zug den Jellyseerr-Benutzer mit gleicher E-Mail und gleichem Passwort (Marco, E-1; N-1). Gibt es die E-Mail in Jellyseerr schon, entsteht nur ein Vorschlag, den ein Administrator in M-24 bestätigt (N-2a nach K-1); die Seite sagt nie, ob Jellyseerr die Adresse kennt. Ist Jellyseerr nicht erreichbar, folgt der Benutzer beim nächsten Abgleich, das Passwort bei der nächsten Anmeldung (N-4). Fehlt Jellyseerr oder ist «Neue Konten in Jellyseerr eröffnen» aus, entfällt der Hinweis (Vorschlag, 02 Abschnitt 5.6) |
| Rolle | eingeladene Person ohne Konto |
| Neue oder veränderte Einstellungen | wirken hier: «Neue Konten in Jellyseerr eröffnen», «Passwort abgleichen» (M-25); Wunschrecht und Limit der Einladung (M-26) |
| API-Operationen | `acceptInvitation` (Antwort in allen Fällen gleich; Seiteneffekt Eröffnung oder Vorschlag, Passwort nur an einen neu eröffneten Benutzer); danach `login` mit Seiteneffekt bei `next_login` |
| Mockup | M-35, `#/invite/i1` (auch aus M-00 «Konten» und aus M-26 «Einladungslink öffnen»); `js/viewer-account.js` (`route('/invite/:id')`, `ACT.vaAccept`), `acceptInvitation`, `provisionAccount` (mit `by: 'self'`) in `js/model2.js` |
| Abgedeckte interaktive Zustände | Kopf mit Notiz «Für Tante Ruth», «Altersstufe ab 16», «Darf Titel anfordern · Limit: 10 Filme in 7 Tagen, 5 Staffeln in 7 Tagen», «Gültig bis …»; Felder «E-Mail», «Name des Profils», «Passwort» («Mindestens 10 Zeichen.»), «Passwort wiederholen»; Fehler «Gib eine gültige E-Mail-Adresse ein.», «Mit dieser E-Mail-Adresse gibt es schon ein Konto.», «Gib einen Namen für dein Profil ein.», «Mindestens 10 Zeichen.», «Die beiden Passwörter sind nicht gleich.»; Band «Dein Login gilt auch für Jellyseerr» (NEU) mit Text je Zustand (verbunden bzw. «Jellyseerr ist gerade nicht erreichbar …»); «Konto anlegen» meldet an, wechselt ins neue Profil und meldet bei neuer Adresse «In Jellyseerr hast du jetzt ein Konto mit derselben E-Mail und demselben Passwort.», bei einer Adresse, die Jellyseerr schon kennt (etwa `tv@example.ch`), und bei nicht erreichbarem Jellyseerr gleich neutral «Dein Konto in Jellyseerr ist noch nicht bereit. Bis dahin kannst du noch keine Titel anfordern.»; das Konto erscheint in M-24 als «Von Slouchcat eröffnet» (Passwort «Gleich wie in Slouchcat»), «Verknüpfung vorgeschlagen, wartet auf den Administrator» oder «Wird eröffnet, sobald Jellyseerr erreichbar ist», und in M-26; benutzte oder widerrufene Einladung: «Diese Einladung ist nicht mehr gültig»; ohne Jellyseerr oder mit Schalter aus kein Band, kein Benutzer |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.5a (`seerr.provision` mit Vorschlag), 1.5b (Haken `WithPasswordSeen` in `AcceptInvitation`, `Login`), 1.5c (Limit), 1.9 (Seite mit Hinweis); `04-benutzer-profilmapping.md` Abschnitte 12 und 13; `02-funktionskonzept.md` Abschnitte 4.9 und 5.6; AK in `10-abnahmekriterien.md` Abschnitt 2.5 (AK-213) |

### M-20 · Verwaltung › Übersicht (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin` |
| Heute | «Alles im Blick», «Aktualisieren», Kennzahlen «Aktive Streams», «Laufende Jobs», «Titel zur Pflege», «Speicherplatz»; «Was benötigt Aufmerksamkeit?»; «Gerade aktiv»; «Direkt zu deinen Aufgaben»; «Server & letzte Aktivitäten» |
| Geplante Änderung | fünfte Kennzahl «Offene Wünsche»; Hinweise zu Wünschen und Jellyseerr, neu in Fassung 3 «n Konten ohne Jellyseerr-Benutzer» (samt Vorschlägen), «Limit erreicht: {Konto}», «n Wünsche ohne Profil-Tag», «n Downloads verzögert» (`07-administration.md` Abschnitt 2); Schnellzugriff «Wünsche»; Zeile «Jellyseerr» im Serverzustand; Wunsch-Ereignisse in «Letzte Aktivität» |
| Rolle | Administrator mit Hauptprofil |
| Neue oder veränderte Einstellungen | keine |
| API-Operationen | `getDashboard` (Kennzahl, Hinweise, Integrationsstatus `seerr`), `getSeerrStatus` |
| Mockup | M-20, `#/admin`, `js/admin-a.js` (`route('/admin')`, `aaHints`, `aaNoUserAccounts`, `aaTagLate`) |
| Abgedeckte interaktive Zustände | Kennzahl mit «n warten auf Freigabe», «–», wenn Jellyseerr nicht verbunden und kein Wunsch offen ist; Hinweise «Jellyseerr nicht erreichbar», «Jellyseerr lehnt den Schlüssel ab», «Jellyseerr blockiert schreibende Aufrufe», «n Konten ohne Jellyseerr-Benutzer» (nur Konten mit Wunschrecht; Text nennt «davon n Verknüpfung vorgeschlagen, wartet auf deine Bestätigung» und, bei nicht erreichbarem Jellyseerr, dass die Konten warten), «Benutzer in Jellyseerr gelöscht», «Limit erreicht: grosseltern@example.ch» mit «Filme: 10 von 10 in 7 Tagen, der nächste Platz wird am … frei» und «Zum Konto» (öffnet das Konto in M-26), «n Wünsche ohne Profil-Tag» (nur Tags, die nicht gesetzt werden konnten; nach «Radarr weg» und einem neuen Filmwunsch) mit Link auf M-23, Filter «Tag nicht gesetzt», ein Sammelhinweis «1 Download verzögert» mit ««Verity» (Radarr): Download verzögert», ohne Meldung von Radarr, Link auf M-23, Filter «Download verzögert», «n Wünsche warten auf Freigabe», «n Wünsche fehlgeschlagen», «n Wünsche warten auf Übermittlung»; je «Ansehen»; «Alles in Ordnung.»; Mockup › «Nicht eingerichtet» blendet Kennzahl, Jellyseerr-Hinweise und Schnellzugriff aus; Mockup › Sandra: «Nur für Administratoren» |
| Spätere technische Umsetzung | Stufe 1 und 2: Schritt 2.10 (Kennzahl, «Limit erreicht», «Konten ohne Jellyseerr-Benutzer»); Stufe 4: Schritte 4.6c, 4.6e (Tag- und Download-Hinweise); `07-administration.md` Abschnitt 2 |

### M-21 · Verwaltung › Integrationen (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/integrations` |
| Heute | «n von m eingerichtet» und sechs Kacheln: Schlüssel der Anbieter, Webhooks von Radarr und Sonarr, E-Mail-Server, Telegram-Bot, Web-Push, Server |
| Geplante Änderung | siebte Kachel «Jellyseerr» nach «Webhooks von Radarr und Sonarr» mit Meta und Zustand, bei Hinweisen gelber Punkt und der erste Hinweis; neun Statusschlüssel. Die Ansicht «Webhooks von Radarr und Sonarr» bleibt wörtlich unverändert (E-6, N-80, K-7) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | keine auf der Kachel |
| API-Operationen | `getSeerrStatus`, Stufe 4 `getArrStatus`, neben den bestehenden |
| Mockup | M-21, `#/admin/integrations`, `js/admin-a.js` (`aaIntegrationsPage`, `aaTile`, `aaIntCounts`, `aaSeerrHintList`); Zähler im Bereichsreiter `js/shell.js` (`areaTabs`) |
| Abgedeckte interaktive Zustände | Kachel mit Beschreibung «Wünsche der Zuschauer an Jellyseerr weitergeben und ihren Zustand abgleichen. Konten erhalten dort einen Benutzer mit demselben Login.», Meta «Jellyseerr-Benutzer: 2 von 3 Konten · n offene Wünsche» und Zustand «Jellyseerr: Verbunden · Staffeln aus TMDB · Seerr 3.5.2» mit gelbem Punkt (erster Hinweis; sonst etwa «Radarr für Profil-Tags und Fortschritt nicht erreichbar», «Lokale Anmeldung in Jellyseerr aus» oder «n Konten ohne Jellyseerr-Benutzer»); «Nicht erreichbar» (mit «zuletzt erreicht vor …»), «Schlüssel ungültig», «CSRF-Schutz blockiert», «Nicht eingerichtet», «Ausgeschaltet»; Zeile oben «n von 9 eingerichtet», Bereichsreiter «n / 9»; Chip «ungespeichert»; alle sechs bestehenden Ansichten bedienbar (`#/admin/integrations/keys`, `/webhooks`, `/email`, `/telegram`, `/push`, `/server`) |
| Spätere technische Umsetzung | Stufe 1, Schritt 1.7; Stufe 4, Schritt 4.6c; `07-administration.md` Abschnitt 3 |

### M-22 · Integrationen › Jellyseerr (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/admin/integrations?item=jellyseerr` (im Mockup `#/admin/integrations/jellyseerr`) |
| Heute | gibt es nicht |
| Geplante Änderung | sechs Bänder in dieser Reihenfolge (`07-administration.md` Abschnitt 4): «Verbindung» (Schalter, Adresse, Schlüssel, Test mit Prüfliste, neu «Benutzer anlegen und ändern», «Lokale Anmeldung», «Staffeln aus TVDB», «Radarr/Sonarr für Profil-Tags und Fortschritt», «Passwort verschlüsselt»), «Abgleich» (mit Fakt «Download-Fortschritt»), «Benutzer und Limit» (Stufe 1, nur Anzeige), «Radarr und Sonarr (Profil-Tags und Fortschritt)» (Stufe 4: Adresse aus Jellyseerr, eigene Adresse mit «Speichern», «Testen», «Tag Requests», Zähler der Tags, Fakt «Download-Fortschritt», Link auf die unveränderten Webhooks), «Sofortmeldung» (Stufe 4), «Protokoll» |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | «Jellyseerr verwenden» (ab Werk aus), «Adresse», «API-Schlüssel», Adresse der Sofortmeldung; neu «Eigene Adresse» für Radarr und Sonarr |
| API-Operationen | `getSeerrSettings`, `updateSeerrSettings`, `testSeerrConnection` (neue Prüfpunkte `manage_users`, `local_login`, `tvdb_seasons`, `password_transport`, Stufe 4 `radarr_tags`, `sonarr_tags`), `getSeerrStatus`, `syncWishesNow`, `confirmSeerrInstance`; Stufe 4 `getArrStatus`, `updateArrSettings`, `testArrConnection`, `createIntegration`, `receiveWebhook` mit Art `jellyseerr` |
| Mockup | M-22, `#/admin/integrations/jellyseerr`, `js/admin-a.js` (`aaSeerrView`, `aaSeerrRun`, `aaSeerrRows`, `aaUsersBand`, `aaArrBand`, `aaTagCounts`, `aaProgFact`, `ACT.aaArrTest`, `ACT.aaArrSave`) |
| Abgedeckte interaktive Zustände | **Verbindung** klappt bei Fehler auf; «Verbindung testen» mit Ladeanzeige und Prüfliste aus zwölf Punkten (Erreichbar, Schlüssel gültig, «Version» mit «Seerr 3.5.2», Handeln im Namen von Benutzern, CSRF-Schutz, Radarr eingerichtet, Sonarr eingerichtet, neu «Benutzer anlegen und ändern», «Lokale Anmeldung in Jellyseerr», «Staffeln aus TVDB», «Radarr für Profil-Tags und Fortschritt» mit «Radarr erreichbar unter http://radarr:7878 · Warteschlange lesbar · Tag Requests: an», ebenso Sonarr); gelbe Hinweise lassen den Test gelingen («Verbunden · 1 Hinweis»): «Staffeln aus TVDB» bei Mockup › Staffeln «TMDB», Radarr oder Sonarr «… nicht erreichbar: Profil-Tags werden nachgeholt …; der Download-Fortschritt fehlt so lange. Eigene Adresse eintragen?»; «falsch» in Adresse oder Schlüssel ergibt «Nicht erreichbar» bzw. «Schlüssel ungültig»; CSRF: zwei Punkte rot, «Der Test ist gescheitert: …»; Pflicht- und Formfehler beim Speichern; ohne Einrichtung Schrittanleitung und gesperrter Schalter; «Verbindung löschen …». **Abgleich** mit Fehlerzeile, «Zeitplan ändern», Zählern als Links, Fakt «Download-Fortschritt» («Fortschritt aus Radarr und Sonarr: jede Minute, solange Wünsche in Beschaffung sind · zuletzt vor 30 s», bei Ausfall «Fortschritt nicht lesbar: Radarr nicht erreichbar») mit «Zum Band Radarr und Sonarr», «Jetzt abgleichen». **Benutzer und Limit** (nur eingerichtet): Kopf «2 von 3 Konten eröffnet · Limit 10 Filme / 5 Staffeln je 7 Tage», Einleitung «Jedes Konto hat in Jellyseerr einen eigenen Benutzer mit demselben Login …», Werte «Konten in Jellyseerr eröffnen», «Passwort abgleichen», «Passwort» (wann es weitergegeben wird), «Lokale Anmeldung», «Limit ab Werk» mit «Administratoren in Jellyseerr haben kein Limit.», Zähler je Zustand (auch «Verknüpfung vorgeschlagen …»), «Jellyseerr-Benutzer verwalten», letzte drei Eröffnungen und Passwortabgleiche. **Radarr und Sonarr** (nur eingerichtet, klappt bei Fehler auf): Kopf «Radarr erreichbar · Sonarr erreichbar · n Tags offen · n Downloads»; je Dienst Zustand («Erreichbar» bzw. «Nicht erreichbar · Eigene Adresse eintragen»), «Adresse aus Jellyseerr», «Tag Requests in Jellyseerr» (Radarr an, Sonarr «aus · Empfohlen: dann trägt jeder Titel auch den Tag des Kontos»), Feld «Eigene Adresse» (Entwurf mit Chip «ungespeichert»), «Testen» mit Ladeanzeige («erreichbar unter … · Schlüssel gültig · Warteschlange lesbar»; Adresse mit «falsch» nicht erreichbar); «Speichern» des Bands; Zähler «Tags gesetzt (30 Tage)», «offen», «fehlgeschlagen» als Links (die beiden letzten auf M-23, Filter «Tag nicht gesetzt»); Fakt «Download-Fortschritt» («an · n Wünsche mit Fortschritt · zuletzt gelesen vor 30 s» bzw. «aus · Einstellungen»); Verweise auf Präfix, Namen je Profil und die unveränderten Webhooks. **Sofortmeldung** (Chip «Stufe 4») «Adresse erzeugen», «Token erneuern», «Entfernen», «Meldung simulieren (Mockup)». **Protokoll** mit Eröffnungen, Vorschlägen, Passwortabgleichen, Tags und Fehlern |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.1, 1.3, 1.5 (Test mit neuen Prüfpunkten), 1.7; Stufe 2: Schritte 2.5, 2.10; Stufe 4: Schritte 4.5, 4.6a (Schlüssel 10 Minuten im Speicher, K-23; Warteschlange seitenweise, K-8), 4.6c; Tabelle `seerr_log`, Einstellungen `arr.radarr_url`, `arr.sonarr_url`; `07-administration.md` Abschnitt 4; AK-171, AK-172 |

### M-23 · Medien › Wünsche › Anfragen (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/admin/wishes`, mit Filter aus dem Link `?f=notag`, `?f=stalled`, `?f=unassigned`, `?state=…`; Bereichsreiter «Wünsche» im Bereich Medien direkt nach «Pflege» |
| Heute | gibt es nicht |
| Geplante Änderung | Kennzahlen als Filter, Filter nach Zustand, «Nicht zuordenbar», Medienart, ab Stufe 4 «Tag nicht gesetzt» und «Download verzögert»; Tabelle mit Profil-Tag je Profil und Download-Fortschritt im Zustand; Aufklappzeile mit Verlauf, Diagnose und der Meldung von Radarr oder Sonarr; Band «Limit je Konto»; Aktionen «Freigeben», «Ablehnen» (Stufe 4), «Erneut übermitteln», «In Jellyseerr öffnen», «Zum Titel», «Jetzt abgleichen»; nicht zuordenbare Anfragen nur mit «In Jellyseerr öffnen» und «Benutzer zuordnen» (`07-administration.md` Abschnitt 5) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | keine |
| API-Operationen | `listAdminWishes` (`AdminWish` ohne `ItemRef`, mit `progress?`, Tags, Filter `progress=stalled`), `getAdminExternalTitleImage`, `retryWish`, `syncWishesNow`; Stufe 4 `approveWish`, `declineWish` |
| Mockup | M-23, `#/admin/wishes`, `js/admin-b.js` (`route('/admin/wishes')`, `abWishRow`, `abQuotaBand`, `abWishTabs`, `AB_WF`, `abTakeQuery`); Tags `tagChips` und Fortschritt `dlChip`, `dlBlock` in `js/model2.js` |
| Abgedeckte interaktive Zustände | Reiter «Anfragen» (Zahl offen), «Jellyseerr-Benutzer» (mit «n ohne»), «Einstellungen»; fünf Kennzahlen setzen den Filter; Band «Limit je Konto» zugeklappt (Kopf «0 Konten am Limit · 2 Konten mit Jellyseerr-Benutzer», nach «Filme ausschöpfen» «1 Konto am Limit»), Tabelle «Konto», «Filme» («2 von 10 in 7 Tagen», am Limit mit «nächster Platz am …»), «Staffeln», «Art» («Standard», «Eigenes», «Unbegrenzt», «Kein Limit (Administrator)») und «Limit ändern» (öffnet das Konto in M-26); Filterpillen für die Zustände, «Nicht zuordenbar», **«Tag nicht gesetzt»** (nur bei eingeschalteten Tags) und **«Download verzögert»** (nur bei eingeschaltetem Fortschritt) mit eigenem Leertext; Tabelle mit «Gewünscht von» samt Mitwünschern und **Tag-Chips** je Profil («sc-marco», «sc-noah · noch nicht gesetzt», «sc-marco · fehlgeschlagen» mit Grund im Titel), «Jellyseerr-Benutzer», «Zustand» mit **Download-Zeile** einschliesslich «Fortschritt nicht lesbar» (Mockup › «Radarr weg»); Aufklappzeile mit Verlauf und Diagnose (Request-ID, Übermittelt als, TMDB-ID, Herkunft, Zustand, Fehlergrund, Zusatz, Mitwünscher, «Profil-Tags» mit Ereignissen, bei «Verity» ohne Tag, «Download» mit Meldung von Radarr, etwa «Radarr meldet: Der Download-Client antwortet nicht.»); «Clayface» «Nicht zuordenbar» nur mit «In Jellyseerr öffnen» und «Benutzer zuordnen» (öffnet M-24 mit diesem Benutzer vorgewählt); Wunsch eines gelöschten Kontos mit Chip «Konto gelöscht» und Hinweis; «Freigeben», «Ablehnen» (Rückfrage nennt die Profile) bei «Toy Story 5»; Jellyseerr nicht verbunden oder CSRF: Band oben und «Geht nicht: …»; «Erneut übermitteln» bei `queued` und `failed`; «In Jellyseerr öffnen» als Dialog; Wünsche aus: Band mit Link zu M-25 |
| Spätere technische Umsetzung | Stufe 1: Schritt 1.8; Stufe 2: Schritt 2.10 (mit Band «Limit je Konto»); Stufe 4: Schritte 4.4, 4.6c (Tags mit Filter), 4.6e (Fortschritt mit Filter und Meldung); `07-administration.md` Abschnitte 1.2, 5 und 13; AK-89, AK-173 bis AK-175, AK-198, AK-200 |

### M-24 · Medien › Wünsche › Jellyseerr-Benutzer (neu, vorher «Zuordnung»)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/admin/wishes/mapping` (mit `?user={id}` aus M-23) |
| Heute | gibt es nicht |
| Geplante Änderung | je Konto ein zugeklapptes Band: Jellyseerr-Benutzer, Zustand (eröffnet, verknüpft, von Hand, wartet, Verknüpfung vorgeschlagen, nicht eröffnet), Passwort, Rechte, Limit inklusive direkt in Jellyseerr gestellter Wünsche; Aktionen «In Jellyseerr eröffnen», «Alle eröffnen» (mit Rückfrage, lässt Vorschläge aus), «Verknüpfung bestätigen» (N-2a), «Anderen Benutzer wählen», «Lösen», Passwortmodus; je Profil Tag-Name und zugeklappt die Ausnahme «Profil sendet als anderer Jellyseerr-Benutzer» (N-41); Band «Jellyseerr-Benutzer ohne Konto» mit «Einem Konto zuordnen»; alles sofort wirksam (`07-administration.md` Abschnitt 6) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | Jellyseerr-Benutzer je Konto, Passwortmodus je Konto, Tag-Name je Profil, Ausnahme je Profil |
| API-Operationen | `listSeerrUsers`, `refreshSeerrUsers`, `getWishMappings`, `provisionSeerrAccount`, `provisionSeerrAccounts` (409 `provision_disabled`), `confirmSeerrAccountLink`, `setAccountWishMapping` (wählen, lösen, zuordnen; 409 `seerr_user_taken`), `setSeerrPasswordMode`, `setProfileWishMapping`, Stufe 4 `setProfileTagName`, `importSeerrRequests`, `countSeerrRequests`. `applyWishMappingSuggestions` entfällt (Vorschlag in `08` Abschnitt 4.5) |
| Mockup | M-24, `#/admin/wishes/mapping`, `js/admin-b.js` (`route('/admin/wishes/mapping')`, `abSeerrBand`, `abProfRow`, `abQuotaBlock`, `abUserFacts`, `abUserWarn`, `DIALOGS.abProvisionAll`, `ACT.abConfirmLink`, `ACT.abAssignOk`); Zustände `SEERR_ACC`, `PW_STATE`, `provisionAccount`, `confirmLink`, `linkAccount`, `unlinkAccount`, `pwSetMode`, `seerrFreeFor` in `js/model2.js` |
| Abgedeckte interaktive Zustände | Kennzahlen «Konten», «In Jellyseerr», «Ohne Benutzer» (mit «wartet»), «Passwort noch offen»; Warnung «1 Konto ohne Jellyseerr-Benutzer» mit «Alle eröffnen» und «Zu den Einstellungen»; «Alle eröffnen» fragt nach und listet «Über gleiche E-Mail-Adresse verknüpfen», «In Jellyseerr neu anlegen», «Vorgeschlagene Verknüpfung bleibt offen …»; Zeile «Neue Konten: werden in Jellyseerr eröffnet · Passwort: wird abgeglichen · Ändern»; «Benutzerliste neu laden» (ohne Verbindung «Gezeigt wird der letzte Stand»); je Konto ein zugeklapptes Band mit Rolle, Zustand, Benutzer mit ID, Zahl der Wünsche und Verbrauch im Kopf; darin «Jellyseerr-Benutzer» (Name, ID, E-Mail, Art, «von Slouchcat eröffnet» oder «bestand schon in Jellyseerr»; bei einem Vorschlag «… · vorgeschlagen»), «Wünsche», «Zustand» («Von Slouchcat eröffnet», «Verknüpft über gleiche E-Mail», «Von Hand zugeordnet», «Wird eröffnet, sobald Jellyseerr erreichbar ist», «Verknüpfung vorgeschlagen, wartet auf den Administrator», «Nicht eröffnet», «In Jellyseerr gelöscht»), «Passwort» («Gleich wie in Slouchcat» mit Datum, «Wird bei der nächsten Anmeldung in Slouchcat übernommen», «Wird nicht abgeglichen», «Bleibt in Jellyseerr (Administrator)», «Anmeldung über Jellyfin oder Plex», ohne Benutzer «Folgt mit dem Benutzer»), «Rechte in Jellyseerr», «Limit» mit Balken, «davon n direkt in Jellyseerr gestellt», «Nächster Platz frei …», «Limit ändern»; **Vorschlag** (Konto aus M-35 mit `tv@example.ch`): Band offen, gelber Hinweis «Verknüpfung vorgeschlagen: «wohnzimmer-tv»» mit «Bestätige nur, wenn Konto und Benutzer dieselbe Person sind …» und «Verknüpfung bestätigen» (danach «Verknüpft über gleiche E-Mail», Passwort nach Art des Benutzers); Hinweis «In Jellyseerr gibt es schon «gast»» bei gast; «In Jellyseerr eröffnen» (verknüpft gast, bei «Nicht erreichbar» «wartet»), bei ausgeschaltetem Eröffnen gesperrt mit «Gesperrt: «Neue Konten in Jellyseerr eröffnen» ist … ausgeschaltet.» und «Zur Einstellung» (ebenso «Alle eröffnen»); «Anderen Benutzer wählen» mit Auswahl (Benutzer anderer Konten und solche, unter denen ein Profil eines anderen Kontos anfordert, gesperrt mit «gehört zu …») und «Übernehmen»; «Verknüpfung lösen» ohne Rückfrage mit Meldung; «Passwort bei der nächsten Anmeldung neu übernehmen» (gesperrt bei ausgeschaltetem Abgleich) und «Passwort nicht abgleichen», Erklärung bei Administrator und Jellyfin; Hinweis für Konten ohne Recht; je Profil Alter, PIN, Wunschrecht nur lesend («Im Profil ändern»), Feld «Eigener Name (Profil-Tag in Radarr und Sonarr)» mit Vorgabe als Platzhalter, wirksamem Tag als Chip, Meldung «Noah: Tag heisst jetzt …» und Fehler «Erlaubt sind nur Kleinbuchstaben a–z, Ziffern 0–9 und der Bindestrich.»; Band «Ausnahme» zugeklappt (Sandra: «Sendet als sandra», Noah: «Sendet als kinder») mit Auswahl (Benutzer mit ID), Fakten, Warnungen (Rechte nur bei Benutzern, die es schon gab) und Limit des Ausnahme-Benutzers; Band «Jellyseerr-Benutzer ohne Konto» (wohnzimmer-tv, gast) mit Anfragen, «davon n nicht zuordenbar», «Gleiche E-Mail wie …» und «Einem Konto zuordnen» (Auswahl der Konten ohne Benutzer, Meldung «n Anfragen übernimmt der nächste Abgleich»); Mockup-Hilfe «Benutzer «sandra» in Jellyseerr löschen (Mockup)» und zurück |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.2 (`wish_account_map` mit Herkunft, Zustand `ready`/`waiting`/`suggested`, `suggested_seerr_user_id`, Passwortstand), 1.5 (`seerr.users`), 1.5a (`seerr.provision`), 1.5b, 1.5c, 1.8 (`web/src/features/admin/wishes/`); Stufe 4: Schritte 4.6 (`wishes.import`), 4.6b, 4.6c (Tag-Name, `wish_profile_tags`); `04-benutzer-profilmapping.md` Abschnitte 12 bis 18; `07-administration.md` Abschnitt 6; AK in `10-abnahmekriterien.md` Abschnitte 2.2 und 2.5 (AK-213) |

### M-25 · Medien › Wünsche › Einstellungen (neu)

| Zeile | Inhalt |
|---|---|
| Seite | neu, App-Route `/admin/wishes/settings` |
| Heute | gibt es nicht |
| Geplante Änderung | Karten «Grundsatz», «Woher angefordert werden darf» (neu der Schalter für die Seiten Entdecken, Filme, Serien und Suche), «Regeln», «Verweise»; Bänder «Limit ab Werk», «Konten in Jellyseerr» (mit dem Hinweis auf das geteilte Login, K-19), «Radarr und Sonarr: Profil-Tag und Fortschritt» (mit «Download-Fortschritt zeigen»); jede Änderung gilt sofort; kein Schalter «Aus dem Spotlight» mehr (N-72) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | alle `wishes.*` aus `08-datenmodell-api.md` Abschnitt 2.12 (Abschnitt 4 unten); «Wünsche aktiv» ab Werk aus, einschaltbar nur nach gelungenem Test (**Vorschlag** T-1) |
| API-Operationen | `getWishSettings`, `setWishSettings` (je Änderung; mit `showProgress`) |
| Mockup | M-25, `#/admin/wishes/settings`, `js/admin-b.js` (`route('/admin/wishes/settings')`, `abLimitDefBand`, `abProvBand`, `abTagBand`, `ACT.abSaved`, `ACT.abPrefixSaved`) |
| Abgedeckte interaktive Zustände | jeder Schalter mit Meldung «… ist eingeschaltet. Gilt sofort.»; «Wünsche aktiv» aus sperrt die übrigen Zeilen mit Hinweis; «Anfordern aus der Suche», «Anfordern auf Detailseiten», «Seiten Entdecken, Filme, Serien und Suche»; «Bereits angeforderte Titel» mit Hilfetext je Wahl; «Konten ohne Jellyseerr-Benutzer» auf «Sammelbenutzer» zeigt die Auswahl, ohne Wahl Chip «Ohne Benutzer gilt «Nicht erlauben»»; **Limit ab Werk**: «Filme [10] in [7] Tagen», «Staffeln [5] in [7] Tagen», «0 heisst ohne Limit.», Fehler «Anzahl 0 bis 100, Tage 1 bis 90.», Meldung «Slouchcat schreibt es in 2 Jellyseerr-Benutzer der Konten mit «Standard».», Zeile «0 mit eigenem Limit · 0 unbegrenzt» mit Link zu M-26; **Konten in Jellyseerr**: «Neue Konten in Jellyseerr eröffnen» (Hilfe: bei gleicher Adresse nur ein Vorschlag, den du bestätigst), «Passwort abgleichen» mit Hilfe «Wer sich in Jellyseerr anmeldet, sieht dort die Wünsche aller Profile des Kontos, auch die eines Profils mit PIN.» und Chip «Lokale Anmeldung in Jellyseerr: an», Link «Zu den Jellyseerr-Benutzern»; **Radarr und Sonarr**: «Profil-Tag in Radarr und Sonarr setzen», «Download-Fortschritt zeigen», «Präfix» (`sc-`, 1 bis 10 Zeichen, nur a–z, 0–9, Bindestrich; ungültig: Fehler und Chip «Präfix ungültig», nicht gespeichert; gesperrt bei Tag aus), Beispiele «Marco → sc-marco», «Noah → sc-noah», «Oma & Opa → sc-oma-und-opa», Zustand von Radarr und Sonarr; Verweise mit Zustand der Verbindung und «2 von 3 Konten». «Wünsche aktiv» lässt sich ohne gelungenen Test einschalten (Abschnitt 9) |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.3, 1.8; Stufe 4: Schritt 4.6c; Stufe 5: Schritt 5.6; Tabelle `settings` (`wishes.*`); `07-administration.md` Abschnitte 4.6 und 7 |

### M-26 · Nutzer & Geräte › Konten (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/accounts` (mit `?open={konto}` aus M-20 und M-23) |
| Heute | Zusammenfassung, Reiter «Konten» und «Einladungen», Tabelle mit E-Mail, Rolle, Altersstufe, Bibliotheken, Profile, Zuletzt gesehen, Kanäle, Aktionen; eigenes Konto «Konto ändern» und «Meine Profile»; anderes Konto Formular «Zugriff»; «Konto einladen» |
| Geplante Änderung | Spalte «Wünsche»; im Formular «Zugriff» bzw. im Block «Wünsche» der Administratorkonten Schalter «Darf Titel anfordern», **Limit je Konto** («Standard», «Eigenes», «Unbegrenzt»; einziger Ort, N-31, gespeichert mit dem Formular) und Zeile «Jellyseerr-Benutzer»; Dialog «Konto einladen» mit Schalter, Limit und Hinweis auf dasselbe Login; Spalte «Wünsche» bei Einladungen; Passwortänderung mit Hinweis auf Jellyseerr; beim Entfernen Rechte und Zufallspasswort nur bei eröffneten Benutzern (N-10 nach K-2), offene Wünsche geschlossen (`07-administration.md` Abschnitt 8) |
| Rolle | Administrator; Besitzer zusätzlich Rollenwechsel |
| Neue oder veränderte Einstellungen | `accounts.can_request`, `invitations.can_request`, Limit je Konto (`wish_account_limits`), Limit der Einladung (**Vorschlag** Spalten an `invitations`) |
| API-Operationen | `listAccounts`, `updateAccountAccess` (`canRequest`, `seerrUser` mit `state` auch `suggested`, `wishState`, `wishLimit`), `setAccountWishLimit`, `createInvitation`, `listInvitations` (`canRequest`, `wishLimit`), `updateAccount` (Seiteneffekt Passwort und E-Mail), `deleteAccount` (Seiteneffekt) |
| Mockup | M-26, `#/admin/accounts`, `js/admin-b.js` (`abAccRow`, `abAccWish`, `abAccDetail`, `abAccessForm`, `abWishForm`, `abWishBlock`, `abSeerrLine`, `abLimitField`, `abOwnForm`, `DIALOGS.abInvite`, `DIALOGS.abRemove`, `abRemovePlan`) |
| Abgedeckte interaktive Zustände | Spalte «Wünsche» mit «Erlaubt · marco», «Erlaubt · … (Sammelbenutzer)», «Wird in Jellyseerr eröffnet», «Kein Jellyseerr-Benutzer» (auch bei einem Vorschlag), «Benutzer gelöscht», «Nicht erlaubt»; grosseltern und gast: Formular «Zugriff» mit Fieldset «Wünsche» (Schalter, Zeile «Jellyseerr-Benutzer: grosseltern · eröffnet · Passwort abgeglichen · Ändern» bzw. «Verknüpfung vorgeschlagen: … · Bestätigen», Chips der Profile, Feld «Limit» mit «Standard»/«Eigenes»/«Unbegrenzt», bei «Eigenes» der Editor, Verbrauch in Jellyseerr), «Ungespeicherte Änderungen», «Verwerfen», «Zugriff speichern» mit «Die Rechte von «grosseltern» in Jellyseerr werden angepasst.» und «Limit: … In Jellyseerr bei «grosseltern» gesetzt.»; Administratorkonten: Block «Wünsche» mit denselben Teilen und «Speichern», Feld «Limit» gesperrt «Kein Limit (Administrator)»; eigenes Konto Marco: «Konto ändern» (E-Mail, neues Passwort mit Hinweis «Das neue Passwort gilt auch in Jellyseerr.», wenn das Passwort abgeglichen wird; aktuelles Passwort; Meldung nach dem Speichern je Stand, bei Marco «Dein Passwort in Jellyseerr bleibt, wie es ist: Du bist dort Administrator.»), Block «Wünsche», «Meine Profile»; «Konto einladen» mit Vorgabe aus «Neue Konten dürfen anfordern», Feld «Limit» und Hinweis «Beim Annehmen eröffnet Slouchcat das Konto auch in Jellyseerr …», danach Einmal-Link; Reiter «Einladungen» mit Spalte «Wünsche» («Darf anfordern», «Limit: Standard»), «Einladungslink öffnen» (führt zu M-35), «Widerrufen»; Menü «Zum Administrator ernennen», «Rolle entziehen», «Konto entfernen» mit Rückfrage: bei eröffnetem Benutzer «… verliert seine Rechte und bekommt ein neues Zufallspasswort.», bei verknüpftem «… bleibt unverändert.», bei Vorschlag «Die vorgeschlagene Verknüpfung … fällt weg»; dazu die Liste der Wünsche: offene werden geschlossen («Das Konto, das den Wunsch gestellt hat, gibt es nicht mehr.»), Wünsche mit Mitwünschern anderer Konten gehen an den ältesten |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.2, 1.5b (Passwort bei `UpdateAccount`), 1.5c (`setAccountWishLimit`), 1.6, 1.9 (`web/src/features/admin/accounts/`); `07-administration.md` Abschnitt 8; `04-benutzer-profilmapping.md` Abschnitte 10, 13, 15 und 17 |

### M-27 · Startseite › Spotlight (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/spotlight` |
| Heute | Band «Quelle» mit elf Feldern; Band «Einträge der Liste» mit «Anheften», «Ausblenden», «Notiz»; fehlende Einträge ohne Poster und ohne Aktion; Band «Verwaiste Anpassungen» |
| Geplante Änderung | Hinweis im Kopf; neues Band «Fehlende Einträge» ohne Schalter mit Liste, Zustand und «Anfordern» (N-72); in «Einträge der Liste» fehlende Einträge mit Poster, Zustand und «Anfordern», je Eintrag die IDs TMDB, TVDB, IMDb, bei Serien der Erkennungsweg, nicht zuordenbare Einträge ohne «Anfordern» (N-62); neues Band «Abgleich über TMDB, TVDB und IMDb» mit Zählung, Staffelquelle und Liste der nicht zuordenbaren Einträge; Band «Quelle» unverändert (N-71; `05-spotlight-integration.md` Abschnitt 4) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | keine (die Schalter für fehlende Titel aus der ersten Fassung 3 entfallen, N-72) |
| API-Operationen | `getSpotlightConfig`, `setSpotlightConfig` (unverändert), `getSpotlightEntries` (`tvdbId?`, `imdbId?`, `unmatched?`, `wish?`, Poster über `getAdminExternalTitleImage`), `setSpotlightEntryOverride`, `createWish` des Hauptprofils |
| Mockup | M-27, `#/admin/spotlight`, `js/admin-b.js` (`abSourceBand`, `abMissingBand`, `abEntriesBand`, `abIdsBand`, `abOrphansBand`, `abSpotNoRequest`, `AB_UNMATCHED`, `ACT.abSpotSave`); IDs `idLine`, `ID_MATCH` in `js/model2.js` |
| Abgedeckte interaktive Zustände | Hinweis «Die Startseite zeigt weiterhin nur Vorhandenes …»; Band «Quelle» mit allen elf Feldern, Prüfung beim Speichern, «Jetzt abrufen» mit Ladeanzeige, Gründe bei zu wenigen Treffern, «Ungespeicherte Änderungen»; Band «Fehlende Einträge» zugeklappt, Kopf «14 fehlen · 5 angefordert», zwei Sätze zur Regel und Zählung, darunter die Liste der 14 Einträge mit Poster, Zustand («Noch nicht in der Bibliothek» oder «Angefordert · In Beschaffung» usw.) und «Anfordern» (ohne offenen Wunsch); ohne Jellyseerr, mit «Jellyseerr verwenden» aus oder «Wünsche aktiv» aus: Chip «Anfordern nicht möglich», Hinweis «Anfordern geht erst, wenn Wünsche eingerichtet sind.» mit Grund und Links, kein «Anfordern» (auch nicht in «Einträge der Liste»); «Einträge der Liste» mit 35 Einträgen, je Rang, Poster, «Film · Jahr», IDs «TMDB … · IMDb …» bzw. bei Serien «TMDB … · TVDB … · IMDb …» (unbekannte als «–»), bei Serien Chip «In der Bibliothek über die TVDB-ID erkannt», «… über die TMDB-ID erkannt», «TMDB- und TVDB-ID von Trakt» oder «Trakt nennt nur die TVDB-ID, TMDB-ID nachgeschlagen»; Status «Sichtbar», «Ausgeblendet», «Noch nicht in der Bibliothek», «Angefordert · …» und beim erfundenen Eintrag «Hafen im Nordlicht» «Nicht zuordenbar: nur TVDB-ID bekannt, bei TMDB nicht gefunden» mit «Keine Aktion: ohne TMDB-ID nicht anforderbar»; «Anheften» mit Platzwahl und Belegungsprüfung, «Lösen», «Ausblenden»/«Einblenden», «Notiz» (erscheint auf der Bühne), «Anfordern» (öffnet M-13 als aktives Profil); Band «Abgleich über TMDB, TVDB und IMDb» mit Kopf «20 in der Bibliothek · 14 fehlen · 1 nicht zuordenbar», Text zu Filmen, Serien und Jellyseerr, Zählung «Erkannt nach Weg», «Staffeln in Jellyseerr» (TMDB mit Empfehlung, auf TVDB umzustellen; nach Mockup › TVDB grün) und Liste «Nicht zuordenbar»; «Entfernen» bei verwaisten Anpassungen; Fussnote zu M-28 |
| Spätere technische Umsetzung | Stufe 3: Schritte 3.1 (`spotlight_ids`), 3.3 (`SpotlightEntry` erweitert), 3.5 (`SpotlightPage.tsx`); Stufe 2: Schritt 2.2 (`titles.refresh` auch für fehlende Einträge); `05-spotlight-integration.md` Abschnitte 4, 5 und 6; `07-administration.md` Abschnitt 9; AK-79, AK-169, AK-170, AK-193, AK-194 |

### M-28 · Startseite › Startseite (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/home-rows` |
| Heute | Band «Reihen» mit Platz, Reihe, Zustand, Ausblenden, Ausklappzeile; Grenze 15 sichtbare Reihen; Band «Vorschau»; Speicherleiste |
| Geplante Änderung | neue Reihe «Erfüllte Wünsche» mit Platz und Ausblenden, ab Werk direkt nach «Weiterschauen» (bei schon 15 sichtbaren Reihen ausgeblendet, **Vorschlag**); Ausklappzeile mit der Regel «in den letzten 14 Tagen erfüllt» und Link «Zur Seite «Wünsche»» (`07-administration.md` Abschnitt 10; F-34) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | Platz und Ausblenden der Reihe `wishes` |
| API-Operationen | `getHomeRows`, `setHomeRows` (`HomeRowKey` + `wishes`, brechend) |
| Mockup | M-28, `#/admin/home-rows`, `js/admin-b.js` (`route('/admin/home-rows')`, `AB_ROW_DESC.wishes`); Vorgabe in `seed()` von `js/store.js` |
| Abgedeckte interaktive Zustände | Reihe «Erfüllte Wünsche» (NEU) auf Platz 3 nach «Weiterschauen»; Ausklappzeile «Wünsche des Profils, die in den letzten 14 Tagen erfüllt wurden … Offene Wünsche stehen unter «Wünsche», nicht auf der Startseite.» mit Link; Platz per Auswahl ändern («Speichere, damit es gilt»); «Ausblenden»; Grenze 15 mit Warnung; Vorschau in ungespeicherter Reihenfolge mit Hinweis, dass leere Reihen entfallen; «Verwerfen» (Rückfrage) und «Speichern»; Wirkung auf M-01 |
| Spätere technische Umsetzung | Stufe 3: Schritte 3.3, 3.4, 3.5 (`HomeRowsPage.tsx`); Tabelle `home_rows` (CHECK + `wishes`); AK-78 |

### M-29 · Automatisierung › Benachrichtigungen (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route der Kanalseite in der Verwaltung |
| Heute | Kanalseite des eigenen Kontos in der Verwaltungsleiste, Link zu den Kanälen des Servers |
| Geplante Änderung | drei neue Ereignisse je Kanal wie M-10; Kopf zählt die Ereignisse; im Mockup zusätzlich Karte «Was neu ist» und Tabelle «Zuletzt zugestellt» (geht über `07-administration.md` Abschnitt 11 hinaus) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | wie M-10 |
| API-Operationen | bestehende Kanal-Operationen; Zustellungen aus `deliveries` |
| Mockup | M-29, `#/admin/notifications`, `js/admin-a.js` (`route('/admin/notifications')`), Kanalteil `channelSettingsHtml({ admin: true })` |
| Abgedeckte interaktive Zustände | Karte «Kanäle des Servers» mit «Ansehen»; Karte «Was neu ist»; alle Zustände von M-10; Tabelle «Zuletzt zugestellt» (Zeit, Kanal, Empfänger mit Profil, Betreff, Zustand) füllt sich beim Vorspulen, auch mit «Wunsch nicht übermittelt: …» nach «24 h vergehen» und «Wunsch wartet auf Freigabe: …»; Sandra mit PIN: Betreff ohne Titel |
| Spätere technische Umsetzung | Stufe 2: Schritt 2.6; Stufe 4: Schritt 4.3; `07-administration.md` Abschnitt 11, `06-benachrichtigungen.md` Abschnitt 8 |

### M-30 · Automatisierung › Hintergrundjobs (geändert)

| Zeile | Inhalt |
|---|---|
| Seite | bestehend, App-Route `/admin/jobs` |
| Heute | Zusammenfassung, Arbeiter je Pool, Banner pausierter Jobs, «Läuft gerade», Reiter je Gruppe mit Filter, je Jobart Zustand, Letzter und Nächster Lauf, «Jetzt ausführen», «Pausieren», «Zeitplan», «Läufe», «Wartende abbrechen»; Reiter «Fehlschläge» |
| Geplante Änderung | zwölf neue Jobarten (acht aus Fassung 2, vier aus Fassung 3: `seerr.provision`, `wishes.tag`, `wishes.progress`, `wishes.discover`); Fehlschläge von Wünschen im Reiter «Fehlschläge»; Erstabgleich von `wishes.sync` als Lauf mit Fortschritt (`07-administration.md` Abschnitt 12) |
| Rolle | Administrator |
| Neue oder veränderte Einstellungen | Zeitpläne der neuen Jobs mit Takt (einziger Ort des Abgleichtakts: `wishes.sync`) |
| API-Operationen | bestehende Job-Operationen; neue Typen über `RegisterTypes` |
| Mockup | M-30, `#/admin/jobs`, Sprünge `#/admin/jobs?group=system&job=wishes.sync`, `…&job=wishes.progress`; `js/admin-a.js` (`AA_JOBS`, `aaJobsPage`, `aaJobRow`, `aaLastNew`, `aaNext`, `ACT.aaJobRun`, `DIALOGS.aaSched`, `DIALOGS.aaRuns`) |
| Abgedeckte interaktive Zustände | zwölf Jobarten mit «NEU»: Metadaten «Fehlende Titel auffrischen», «Staffelliste der Serien», «Wünsche-Seiten vorladen» (1 h; ruht, wenn die Seiten aus sind: «ruht · Seiten Entdecken, Filme, Serien und Suche ausgeschaltet»); Zustellung «Wunsch übermitteln», «Wunsch in Jellyseerr zurückziehen»; System «Wünsche abgleichen», «Verfügbare Wünsche prüfen», «Jellyseerr-Benutzer laden», «Anfragen aus Jellyseerr übernehmen (Stufe 4)», «Konten in Jellyseerr eröffnen», «Profil-Tag setzen» (ohne eigenen Zeitplan, «Ausgelöst durch: nach jedem Abgleich, Wunsch «In Beschaffung», neuer Mitwunsch», «Nächster Lauf: mit dem Abgleich · …»), «Download-Fortschritt lesen» (1 min); «Jetzt ausführen» für `wishes.sync`, `seerr.users`, `wishes.reconcile`, `seerr.provision` (eröffnet oder verknüpft alle Konten ohne Benutzer und ohne Vorschlag; bei Schalter aus «Nichts eröffnet»), `wishes.tag` («n Tags gesetzt» oder «… nicht gesetzt: Radarr nicht erreichbar»), `wishes.progress`, `wishes.discover` («Listen aufgefrischt: 5 Reihen mit 83 Titeln»); «Nächster Lauf» mit «aus · Profil-Tags ausgeschaltet», «aus · Download-Fortschritt ausgeschaltet», «bei Bedarf · 1 Konto wartet …», «wartet bis Jellyseerr eingerichtet ist»; Pausieren von `wishes.submit` mit Banner; Zeitplan-Dialog; Läufe-Dialog; Reiter «Fehlschläge» mit «Wunsch übermitteln» und «Zum Wunsch» |
| Spätere technische Umsetzung | Stufe 1: Schritte 1.5 (`seerr.users`), 1.5a (`seerr.provision`); Stufe 2: Schritte 2.2, 2.4, 2.5, 2.10; Stufe 4: Schritte 4.1, 4.6, 4.6b (`wishes.tag`), 4.6d (`wishes.progress`); Stufe 5: Schritte 5.3, 5.6 (`wishes.discover`); Tabelle `job_schedules`; `03-architektur.md` Abschnitt 13, `07-administration.md` Abschnitt 12 |

## 3. Zuordnung zur Mindestprüfliste des Auftraggebers

| Oberfläche der Liste | Mockup | Bemerkung |
|---|---|---|
| Startseite | M-01 | Spotlight nur mit Titeln der Bibliothek (E-5), Reihe «Erfüllte Wünsche» |
| Spotlight | M-01 (Bühne und Posterreihe), M-02 (Liste), M-27 (Verwaltung) | fehlende Einträge nur in M-27 |
| Film-Detailseite | M-05 (vorhanden, Chip «Dein Wunsch ist da»), M-03 (fehlend, neu, mit Wunschkarte und Download-Fortschritt) | |
| Serien-Detailseite | M-06 (vorhanden mit fehlenden Staffeln), M-04 (fehlend, neu) | |
| Globale Suche | M-07 | Abschnitt «Noch nicht in der Bibliothek», Verweis auf M-34 |
| Entdecken | M-31 (Entdecken), M-32 (Filme), M-33 (Serien), M-34 (Suche) | neu in Fassung 3 nach Marcos Nachricht («Discover, Movies, Series und Search») |
| Meine Wünsche | M-08 | fünfter Reiter, Limit, Download-Fortschritt, Sicht des Hauptprofils |
| Benachrichtigungen | M-09 (Eingang), M-10 (Kanäle) | |
| Profileinstellungen | M-11 (Profile), M-10 (Kanäle), M-12 (Profilwahl) | die Seite «Einstellungen» des Profils ändert sich nicht: `#/unchanged/einstellungen` |
| Admin Integrationen | M-21, M-22 | mit Band «Radarr und Sonarr (Profil-Tags und Fortschritt)»; Webhooks unverändert |
| Admin Benutzer | M-26 (Konten, Einladungen, Limit), M-24 (Jellyseerr-Benutzer, Vorschläge), M-35 (Einladung annehmen) | |
| Admin Profile | M-11, M-24, M-26 | Profile verwaltet jedes Konto selbst (M-11); die Verwaltung zeigt sie (M-26) und setzt je Profil Tag und Ausnahme (M-24) |
| Admin Spotlight | M-27, M-28 | |
| Admin Benachrichtigungen | M-29 | |
| Admin Hintergrundjobs | M-30 | |
| Admin Anfragen | M-23, dazu M-20 (Hinweise) und M-25 (Regeln) | |

## 4. Gegenprobe Einstellungen

Spalte «Gespeichert» nach `08-datenmodell-api.md` Abschnitte 2.2, 2.3, 2.12 bis 2.14.

| Einstellung | Sichtbar und bedienbar im Mockup | Gespeichert | Beobachtbare Wirkung im Mockup |
|---|---|---|---|
| Jellyseerr verwenden | M-22, Band «Verbindung» | `seerr.enabled` (ab Werk 0) | M-21 «Ausgeschaltet»; keine fehlenden Titel; M-31 bis M-34 nur noch über eigene Wünsche; neue Wünsche nicht möglich; M-27 ohne «Anfordern» |
| Adresse | M-22, «Adresse» | `seerr.url` | mit «falsch» nach «Speichern»: «Nicht erreichbar», Hinweis auf M-20 |
| API-Schlüssel | M-22, «API-Schlüssel» | `seerr.api_key` (verschlüsselt) | mit «falsch»: «Schlüssel ungültig»; leer behält ihn |
| Sofortmeldung | M-22, «Adresse erzeugen» | `webhook_integrations` Art `jellyseerr` (Stufe 4) | einmal gezeigte Adresse; «Letzte Meldung» nach «Meldung simulieren (Mockup)» |
| Eigene Adresse Radarr, Sonarr | M-22, Band «Radarr und Sonarr», «Eigene Adresse», «Testen», «Speichern» | `arr.radarr_url`, `arr.sonarr_url` (bestätigter Host `arr.*_host` nicht im Mockup) | Entwurf mit «ungespeichert»; mit «falsch» und «Testen»: «nicht erreichbar unter …»; gespeichert mit «falsch»: Kachel und Band mit Fehler, Tags «fehlgeschlagen», Fortschritt «nicht lesbar»; korrigiert: wieder erreichbar, Tags nachgeholt |
| Wünsche aktiv | M-25, «Wünsche aktiv» | `wishes.enabled` (ab Werk 0) | keine fehlenden Titel; Seiten M-31 bis M-34 weg; Band in M-23 und M-24; übrige Zeilen in M-25 gesperrt; M-27 ohne «Anfordern» |
| Filme erlauben, Serien erlauben | M-25 | `wishes.allow_movies`, `wishes.allow_shows` | fehlende Filme bzw. Serien verschwinden aus M-07, M-31 bis M-34; Vorschauseite ohne eigenen Wunsch 404; bei Serien keine gedämpften Staffeln in M-06 |
| Anfordern aus der Suche | M-25 | `wishes.from_search` | M-07 ohne Abschnitt, Pille und Verweis |
| Anfordern auf Detailseiten | M-25 | `wishes.from_detail` | M-06 ohne gedämpfte Karten, Zeile, Chip und Knopf |
| Seiten Entdecken, Filme, Serien und Suche | M-25 | `wishes.discover_pages` | Seitenleiste führt zu M-08, Reiter nur «Meine Wünsche», M-31 bis M-34 zeigen «Entdecken gibt es für dieses Profil nicht» (bzw. 404 ohne eigene Wünsche); M-07 ohne Verweis; `wishes.discover` ruht; sind auch Suche und Detailseiten aus, antworten Vorschauseiten 404 |
| Bereits angeforderte Titel | M-25, «Mitwünschen»/«Nur Zustand zeigen» | `wishes.already_requested` | «Nur Zustand zeigen»: Vorschauseite ohne «Benachrichtige mich», Posterkarte mit Chip «Bereits angefordert» statt Glocke, in M-13 fremd angeforderte Staffeln gesperrt |
| Konten ohne Jellyseerr-Benutzer, Sammelbenutzer | M-25, Segment und Auswahl | `wishes.unmapped`, `wishes.shared_seerr_user` | M-26 «Erlaubt · … (Sammelbenutzer)»; ein Konto ohne Benutzer darf anfordern; M-13 «(Sammelbenutzer)»; M-20 und M-21 ohne Hinweis «ohne Jellyseerr-Benutzer» |
| Neue Konten dürfen anfordern | M-25 | `wishes.new_accounts_can_request` | Vorgabe im Dialog «Konto einladen» |
| Zurückziehen erlauben | M-25 | `wishes.allow_withdraw` | «Zurückziehen» fehlt in M-08 und M-03 |
| Externe Nachrichten bei PIN-Profilen ohne Titel | M-25 | `wishes.pin_external_hide_title` | Betreff für Sandra in M-29 und M-00 mit oder ohne Titel |
| Limit ab Werk (Filme, Staffeln je Anzahl und Tage) | M-25, Band «Limit ab Werk» | `wishes.limit_movie_count`, `…_movie_days`, `…_tv_count`, `…_tv_days` | gilt sofort für Konten mit «Standard»: M-08 (Balken), M-13, M-23 (Band), M-24, M-26 («Standard: …»), M-35 (Kopf der Einladung); Prüfung «Anzahl 0 bis 100, Tage 1 bis 90.» |
| Neue Konten in Jellyseerr eröffnen | M-25, Band «Konten in Jellyseerr» | `wishes.provision_accounts` | aus: M-35 ohne Hinweis und ohne Eröffnung, M-26 Einladen ohne Hinweis, M-24 «In Jellyseerr eröffnen» und «Alle eröffnen» gesperrt mit Grund und Link, M-30 «Konten in Jellyseerr eröffnen» «Nichts eröffnet», M-22 «aus» |
| Passwort abgleichen | M-25, Band «Konten in Jellyseerr» | `wishes.sync_password` | aus: neu eröffnete oder verknüpfte Konten mit «Wird nicht abgeglichen»; in M-24 ist «Passwort bei der nächsten Anmeldung neu übernehmen» gesperrt mit Link; in M-26 kein Hinweis unter «Neues Passwort» |
| Profil-Tag in Radarr und Sonarr setzen | M-25, Band «Radarr und Sonarr» | `wishes.profile_tags` | aus: keine Tag-Chips in M-23, Filter «Tag nicht gesetzt» weg, Diagnose «Ausgeschaltet», M-30 «aus · Profil-Tags ausgeschaltet», M-22 «Profil-Tag aus», M-24 Hinweis am Tag-Feld |
| Präfix | M-25, Band «Radarr und Sonarr» | `wishes.tag_prefix` (`^[a-z0-9-]{1,10}$`) | Beispiele, Tag-Chips in M-23 und Platzhalter in M-24 ändern sich; ungültig: Fehler, «Präfix ungültig», nicht gespeichert |
| Download-Fortschritt zeigen | M-25, Band «Radarr und Sonarr» | `wishes.show_progress` | aus: keine Download-Zeile in M-03, M-04, M-08, M-23 und M-00, Filter «Download verzögert» weg; M-30 «aus · Download-Fortschritt ausgeschaltet»; M-20 ohne «Downloads verzögert»; M-22 «aus»; Tags laufen weiter |
| Jellyseerr-Benutzer des Kontos | M-24, je Konto «In Jellyseerr eröffnen», «Alle eröffnen», «Verknüpfung bestätigen», «Anderen Benutzer wählen», «Verknüpfung lösen»; Band «ohne Konto» «Einem Konto zuordnen» | `wish_account_map` (`seerr_user_id`, `origin`, `state` mit `suggested`, `suggested_seerr_user_id`) | Zustand in M-24, M-26, M-22; Konto darf anfordern oder nicht; Limit des Kontos geht an den Benutzer |
| Passwortmodus je Konto | M-24, «Passwort bei der nächsten Anmeldung neu übernehmen», «Passwort nicht abgleichen» | `wish_account_map.password_state` | Zustand in M-24 und M-26; Knopf «{Konto} meldet sich an» erscheint in M-00 |
| Ausnahme je Profil | M-24, Band «Ausnahme» je Profil | `wish_profile_map` | M-13 «(eigener Benutzer des Profils)», Freigabe und Limit des Ausnahme-Benutzers; M-11 «Sendet als … (eigener Jellyseerr-Benutzer)» |
| Tag-Name je Profil | M-24, Feld «Eigener Name» | `wish_profile_tags` | Chip daneben, Meldung, Tag-Chips in M-23 |
| Wunschrecht Konto | M-26, Formular «Zugriff» bzw. Block «Wünsche», gespeichert mit dem Formular | `accounts.can_request` | Spalte «Wünsche»; Profilschalter in M-11 bedienbar oder gesperrt; Rechte eines eröffneten Benutzers in M-24 folgen |
| Wunschrecht Profil | M-11, «Bearbeiten», «Darf Titel anfordern» | `profiles.can_request` | Chips in M-11, M-24, M-26; Bereich «Wünsche» und fehlende Titel für das Profil |
| Limit je Konto | M-26, Feld «Limit» im Formular «Zugriff» bzw. im Block «Wünsche», gespeichert mit «Zugriff speichern» bzw. «Speichern»; bei Administratoren gesperrt | `wish_account_limits` (`setAccountWishLimit`) | M-08, M-13, M-23 (Spalte «Art»), M-24 |
| Einladung: Wunschrecht und Limit | M-26, «Konto einladen» | `invitations.can_request`, **Vorschlag** `invitations.wish_limit_*` | Spalte «Wünsche» bei Einladungen; Kopf von M-35; Konto aus M-35 erhält beides |
| Reihe «Erfüllte Wünsche» | M-28 | `home_rows` (Schlüssel `wishes`) | Reihe auf M-01 wandert oder fehlt |
| Ereignisse je Kanal | M-10, M-29 | `account_channels.events` | Meldung am Bildschirm nennt nur die verbleibenden Kanäle; «Zuletzt zugestellt» ohne Eintrag |
| Zeitpläne `wishes.sync`, `wishes.reconcile`, `seerr.users`, `wishes.progress` | M-30, Gruppe «System», «Zeitplan» | `job_schedules` | «Nächster Lauf» in M-30; `wishes.sync` auch im Band «Abgleich» von M-22; `wishes.tag` läuft mit dem Abgleich |
| Zeitpläne `titles.refresh`, `series.catalog`, `wishes.discover` | M-30, Gruppe «Metadaten», «Zeitplan» | `job_schedules` | «Nächster Lauf» in M-30 |
| `wishes.submit`, `wishes.withdraw`, `wishes.import`, `seerr.provision`, `wishes.tag` | M-30: kein Zeitplan (Ereignis), «Pausieren» und «Läufe»; `seerr.provision` und `wishes.tag` dazu «Jetzt ausführen» | Pause wie heute | Banner bei `wishes.submit`; `wishes.withdraw` und `wishes.import` laufen nie (Abschnitt 9) |
| Ansicht «Wünsche aller Profile meines Kontos» | M-08 (nur Hauptprofil) | keine Einstellung | Liste mit Profilnamen |
| Sortierung, Genre, «Vorhandene ausblenden» | M-32, M-33 | keine Einstellung (Ansicht) | Raster, Zählung, Pillen |
| nur im Mockup: Staffelquelle, Lokale Anmeldung | M-00 «Jellyseerr liest Staffeln aus …»; Lokale Anmeldung fest an | in Jellyseerr, nicht in Slouchcat | Prüfpunkt «Staffeln aus TVDB», erster Hinweis an der Kachel, Band «Abgleich über …» in M-27; «Lokale Anmeldung» bleibt grün (Abschnitt 9) |

## 5. Gegenprobe Berechtigungen

| Berechtigung oder Grund | Wo einstellbar oder sichtbar | Prüfung im Mockup |
|---|---|---|
| Wunschrecht Konto (Deckel), Grund «Dieses Konto darf keine Titel anfordern.» | einstellbar M-26; sichtbar M-24, M-26, M-11 | Mockup › Gast: kein Eintrag «Wünsche», keine fehlenden Titel, M-11 gesperrt. Marco: M-26 gast «Darf Titel anfordern» an, «Zugriff speichern»; M-24 gast «In Jellyseerr eröffnen» (verknüpft 6 gast); Mockup › Gast: Bereich «Wünsche» erscheint |
| Wunschrecht Profil, Grund «Dieses Profil darf keine Titel anfordern …» | einstellbar M-11 (Hauptprofil); sichtbar M-24, M-26 | Mockup › Mia: kein Eintrag «Wünsche», kein Abschnitt in M-07. Marco: M-11 Mia «Darf Titel anfordern», «Speichern»; Mockup › Mia: M-31 mit Titeln bis 6 |
| Jellyseerr-Benutzer als Bedingung (F-9), Grund «Diesem Konto ist noch kein Jellyseerr-Benutzer zugeordnet …» | M-24, M-26, M-20 | Marco: M-24 grosseltern «Verknüpfung lösen»; Mockup › «Oma & Opa»: kein Bereich «Wünsche», keine fehlenden Titel; M-20 «1 Konto ohne Jellyseerr-Benutzer» |
| Vorschlag ohne Bestätigung (N-2a) | M-24, M-20, M-21, M-26 | M-35 mit `tv@example.ch` annehmen: das neue Konto kann nicht anfordern, M-20 nennt «davon 1 Verknüpfung vorgeschlagen»; nach «Verknüpfung bestätigen» in M-24 darf es anfordern |
| Jellyseerr-Benutzer gelöscht, Grund «… gibt es nicht mehr …» | M-24, M-20, M-26 | M-24 «Benutzer «sandra» in Jellyseerr löschen (Mockup)»; Mockup › Sandra: keine fehlenden Titel, M-08 «Neue Wünsche sind nicht möglich» mit Grund, eigene Wünsche bleiben |
| Recht des Jellyseerr-Benutzers je Art, Grund «… darf diese Art von Titel nicht anfordern.» | M-24 (Chips «Filme», «Serien», Warnung nur bei Benutzern, die es schon gab) | mit den Beispieldaten nicht herstellbar (Abschnitt 9) |
| Rechte eines eröffneten Benutzers folgen dem Wunschrecht (N-9) | M-26, M-24 | Marco: M-26 grosseltern Recht aus, «Zugriff speichern»: «Die Rechte von «grosseltern» in Jellyseerr werden angepasst.»; M-24 zeigt Filme und Serien gesperrt. Bei einem verknüpften Benutzer «… behält in Jellyseerr seine Rechte, weil Slouchcat ihn nicht eröffnet hat.» |
| Konto löschen: Rechte nur bei eröffneten Benutzern (N-10 nach K-2) | M-26, M-24 | grosseltern entfernen: Rückfrage «… verliert seine Rechte und bekommt ein neues Zufallspasswort.»; danach steht grosseltern unter «ohne Konto» mit gesperrten Rechten. Ein verknüpftes Konto (gast nach «In Jellyseerr eröffnen») entfernen: «… bleibt unverändert.» |
| Kein Limit für Administratoren (N-34) | M-13, M-08, M-23, M-24, M-26 | Marco: überall «kein Limit (Administrator)»; M-26 Feld «Limit» gesperrt «Kein Limit (Administrator)» |
| Limit des Kontos und des Ausnahme-Benutzers | M-13, M-08, M-23, M-24 | Mockup › «Oma & Opa»: «8 von 10 Filmen frei»; Mockup › Noah: Limit von kinder (5 Filme, 2 Staffeln) |
| Auto-Freigabe | M-24 (Chip «Automatische Freigabe: ja/nein»), M-13 | Marco fordert «Primetime» an: «In Beschaffung». Mockup › Noah fordert «Moana» an: «Braucht eine Freigabe», danach «Freigabe ausstehend» und Zustellung an Marco |
| Freigeben und Ablehnen in Slouchcat (Stufe 4) | M-23, nur Administrator | Marco: «Toy Story 5» «Freigeben» oder «Ablehnen»; Mockup › «Nicht erreichbar»: «Geht nicht …»; Mockup › Sandra: `#/admin/wishes` zeigt «Nur für Administratoren» |
| Wünsche ausgeschaltet, Grund «Wünsche sind ausgeschaltet.» | M-25 | M-25 «Wünsche aktiv» aus; Marco: M-08 «Neue Wünsche sind nicht möglich» mit diesem Grund |
| Jellyseerr nicht eingerichtet, Grund «Jellyseerr ist nicht eingerichtet.» | M-22, M-00 | Mockup › «Nicht eingerichtet»: wie oben mit diesem Grund |
| Art ausgeschaltet, Grund «Filme/Serien können zurzeit nicht angefordert werden.» | M-25 | M-25 «Filme erlauben» aus: kein fehlender Film; Vorschauseite eines eigenen Wunschs zeigt den Grund |
| Altersstufe (404, nie 403) | M-11, M-26 (bestehend) | Mockup › Noah: «Resident Evil» (16) fehlt in M-31 bis M-34, `#/titles/movie/1423191` «Titel nicht gefunden»; Titel ohne Freigabe fehlen Noah ganz; Mockup › Mia: `#/items/tulsa-king` 404 |
| Bereich «Wünsche» nur mit Recht (F-9, N-56) | Seitenleiste, M-31 bis M-34 | Mockup › Mia: Eintrag fehlt, `#/discover` «Titel nicht gefunden»; Mockup › Noah nach Entzug des Rechts: Eintrag führt zu M-08, `#/discover` zeigt «Entdecken gibt es für dieses Profil nicht» |
| Hauptprofil sieht Wünsche aller Profile | M-08 | Marco: Schalter an zeigt Wünsche von Sandra und Noah; Mockup › Noah: kein Schalter |
| Ändern der Kanäle | M-10, nur Hauptprofil | Mockup › Sandra: gesperrt mit Hinweis |
| Ereignis «Freigabe nötig» | M-10, M-29, nur Administratorkonten | Mockup › «Oma & Opa»: Häkchen fehlt |
| Passwort: kein Abgleich bei Administratoren und Anmeldung über Jellyfin (N-6) | M-24, M-26 | marco: «Bleibt in Jellyseerr (Administrator)», kein Knopf; M-24 gast «Anderen Benutzer wählen» → wohnzimmer-tv: «Anmeldung über Jellyfin oder Plex» |
| Zugang zur Verwaltung | Administrator mit Hauptprofil | Mockup › Sandra auf `#/admin`: «Nur für Administratoren» |

## 6. Gegenprobe Zustände eines Wunsches und Download-Anzeigen

Kürzel der Orte: P = Posterkarten (M-07, M-31 bis M-33), L = Trefferliste M-34, V = Vorschauseite (M-03, M-04)
bzw. Staffelkarten (M-06), W = Meine Wünsche (M-08), A = Verwaltung Anfragen (M-23), S = Einträge und fehlende
Einträge in M-27, R = Reihe «Erfüllte Wünsche» (M-01). Die Bühne zeigt keine Wunschzustände (N-70, K-3).

| Zustand | Sichtbar auf | Herstellen im Mockup |
|---|---|---|
| `queued` «Wird übermittelt» | P (Chip), L, V (Karte mit Zusatzzeile), W (Klartext), A (Filter), M-30 «Fehlschläge» und Läufe von `wishes.submit`, M-20 | Mockup › «Nicht erreichbar», «Schlüssel ungültig» oder «CSRF-Schutz an»; M-31 bei «Odyssey» Plus, «Anfordern». Weiter mit Mockup › «Verbunden» und «Übermitteln» oder «Abgleich jetzt ausführen» |
| `pending` «Freigabe ausstehend» | P, L, V, W («Wartet auf die Freigabe in Jellyseerr.»), A (mit «Freigeben», «Ablehnen»), Bereichsreiter, M-20 | vorhanden: «Toy Story 5» (Mockup › Noah). Neu: Mockup › Noah, «Moana» anfordern |
| `processing` «In Beschaffung» | P, L, V, W, A, S «Angefordert · In Beschaffung» | vorhanden: «Resident Evil». Neu: Marco fordert «Primetime» an |
| ↳ «Wird gesucht» | V, W, A, M-00 | Mockup «Freigeben» bei «Toy Story 5» oder ein neuer Wunsch von Marco |
| ↳ «Wird heruntergeladen · n % · noch m min» | V, W, A (mit Balken), M-00 | «Resident Evil» (45 %, 12 min); «Download weiter» schaltet 12 %, 45 %, 92 %; «Black Doves» mit «6 von 8 Folgen da» |
| ↳ «Wird eingelesen» | V, W, A | «Resident Evil» zweimal «Download weiter» |
| ↳ «Download verzögert» | V, W ohne Meldung; A (Filter «Download verzögert», Meldung von Radarr in der Aufklappzeile); M-20 Sammelhinweis ohne Meldung | vorhanden: «Verity» (Mockup › Sandra); sonst «Download hängt»; zurück mit «Download läuft wieder» |
| ↳ Fortschritt nicht lesbar | A («Fortschritt nicht lesbar»), M-21, M-22; V und W ohne Zusatz | Mockup «Radarr weg» bzw. «Sonarr weg» |
| ↳ Fortschritt aus | nirgends | M-25 «Download-Fortschritt zeigen» aus |
| `partial` «Teilweise verfügbar» | W (Staffel-Chips), A, R, M-06 `#/items/black-clover` | Marco: `#/titles/show/73223`, «Serie anfordern», «2 Staffeln anfordern»; Mockup «Datei trifft ein» |
| `available` «Verfügbar» | W (mit «Abspielen»), A, R (14 Tage), M-05 Chip «Dein Wunsch ist da», M-09 | vorhanden: «Godzilla Minus One». Neu: «Datei trifft ein» bei «Resident Evil»; bei Serien «Nächste Staffel trifft ein» |
| `declined` «Abgelehnt» | W («Erneut anfordern»), A, V («Dein letzter Wunsch: Abgelehnt»), M-09 | vorhanden: «Street Fighter» (Mockup › Noah). Neu: Mockup «Ablehnen» oder M-23 «Ablehnen» |
| `failed` «Fehlgeschlagen» | W (Grund), A («Fehlergrund»), V, M-20, M-30, M-09 | vorhanden: «Coyote vs. Acme» (Grund Radarr/Sonarr). Neu: «Scheitern»; «24 h vergehen» bei `queued` (Grund «Jellyseerr war 24 Stunden lang nicht erreichbar.»); Limit: Mockup › «Oma & Opa», «Nicht erreichbar», Film anfordern, «Filme ausschöpfen», «Verbunden», «Abgleich jetzt ausführen» (Grund «Limit erreicht: Jellyseerr hat den Wunsch nicht angenommen.»); Benutzer gelöscht: Mockup › Sandra bei «Nicht erreichbar» anfordern, sandra löschen, abgleichen; Konto gelöscht: grosseltern mit offenem Wunsch entfernen (Grund «Das Konto, das den Wunsch gestellt hat, gibt es nicht mehr.», in A mit «Konto gelöscht») |
| `withdrawn` «Zurückgezogen» | W (Filter «Abgeschlossen»), A, V («Du hast ihn zurückgezogen.») | Mockup › Noah, M-08, «Toy Story 5» «Zurückziehen» |
| fehlt «Noch nicht in der Bibliothek» | P (Plus, «Noch nicht da»), L (Chip), V, M-07 (Abschnitt), S | «Odyssey», «Primetime» und die Titel aus `js/data-discover.js` |
| vorhanden «In der Bibliothek» | P unter «Wünsche» (Unterzeile), L (Chip) | etwa «Dune: Part Two» in M-31, «Tulsa King» in M-33 |
| bereits angefordert (Mitwunsch) | P (Glocke), L («Bereits angefordert»), V, M-13 «Schon angefordert»; danach W («Du wünschst mit», «Nicht mehr benachrichtigen»), V («Nicht mehr mitwünschen»), A («+ Name», zusätzlicher Tag-Chip) | Mockup › Sandra; M-34 «Resident Evil» «Benachrichtige mich». Staffeln: Marco bei «Black Doves» Staffeln 1 und 2 wählen |
| bereits angefordert (nur Zustand) | P (Chip statt Glocke), V (ohne Knopf), M-13 (Staffel gesperrt) | M-25 «Nur Zustand zeigen» |
| teilweise vorhanden | M-06 (gedämpfte Karten, Zeile, Chip), L («2 von 3 Staffeln vorhanden»), M-13 («Vorhanden»); nicht auf der Bühne | `#/items/tulsa-king` |
| angekündigt | M-06 und M-04 («Angekündigt»), M-13 (nicht wählbar) | Tulsa King Staffel 4, «Die Ringe der Macht» Staffel 3, Dexter Staffel 2 |
| in Jellyseerr angelegt | W (Chip), A (Chip, Herkunft, kein Profil-Tag) | «Verity», «Clayface» |
| nicht zuordenbar | A (Filter, Hinweis, «Benutzer zuordnen»), M-22 (Zähler), M-24 («davon 1 nicht zuordenbar») | «Clayface» von wohnzimmer-tv |
| Konto gelöscht | A (Chip «Konto gelöscht», Hinweis «Slouchcat hat den Wunsch geschlossen …») | M-26 grosseltern entfernen, nachdem «Oma & Opa» angefordert hat |

## 7. Gegenprobe Verbindung, Jellyseerr-Benutzer, Limit, Tags und Benachrichtigungen

### 7.1 Verbindung zu Jellyseerr, Radarr, Sonarr und Staffelquelle

| Zustand | Mockup-Seite | Klickfolge |
|---|---|---|
| Nicht eingerichtet | M-21, M-22, M-20, M-23, M-24, M-25, M-27 | Mockup › «Nicht eingerichtet» oder M-22 «Verbindung löschen …»; M-22 zeigt die Schrittanleitung, ohne Bänder «Benutzer und Limit» und «Radarr und Sonarr»; M-27 ohne «Anfordern» |
| Verbunden | alle | Mockup › «Verbunden» |
| Verbunden mit Hinweis | M-21 (gelber Punkt, erster Hinweis «Staffeln aus TMDB»), M-22 (Test «Verbunden · 1 Hinweis») | ab Start: Staffeln aus TMDB |
| Schlüssel ungültig | M-22, M-21, M-20 | Mockup › «Schlüssel ungültig» oder Schlüssel «falsch123» speichern |
| Nicht erreichbar | M-22, M-21, M-20, M-23, M-24, M-13, M-31 bis M-35 | Mockup › «Nicht erreichbar» oder Adresse mit «falsch»; M-34 gesperrt, M-31 bis M-33 «Stand vom letzten Abruf» |
| CSRF-Schutz an | M-22 (zwei Prüfpunkte rot), M-21 (gelber Punkt), M-20, M-23, M-13 | Mockup › «CSRF-Schutz an»; neue Wünsche bleiben `queued`, Abgleich «nicht möglich» |
| Ausgeschaltet | M-22, M-21 | M-22 Schalter aus |
| Radarr bzw. Sonarr nicht erreichbar | M-21 (Hinweis, sofern es der erste ist), M-22 (Band offen, Chip «Fehler», Prüfpunkt gelb, Fakt «Fortschritt nicht lesbar»), M-25 (Chip), M-23 («Fortschritt nicht lesbar», Tag «fehlgeschlagen»), M-20 («Wünsche ohne Profil-Tag»), M-30 (`wishes.tag`, `wishes.progress` mit Fehler) | Mockup «Radarr weg» oder M-22 eigene Adresse mit «falsch», «Speichern» |
| Radarr bzw. Sonarr wieder erreichbar | wie oben, Tags nachgeholt | Mockup «Radarr erreichbar» oder Adresse korrigieren, «Testen», «Speichern» |
| Staffeln aus TMDB / aus TVDB | M-22 Prüfpunkt «Staffeln aus TVDB» gelb bzw. grün, M-21 Hinweis, M-27 «Staffeln in Jellyseerr» | Mockup › «Jellyseerr liest Staffeln aus …» |
| Lokale Anmeldung aus | nicht herstellbar (Abschnitt 9) | |
| Andere Instanz erkannt | nicht herstellbar (Abschnitt 9) | |

### 7.2 Jellyseerr-Benutzer eines Kontos und Passwort

| Zustand | Mockup-Seite | Klickfolge |
|---|---|---|
| «Von Slouchcat eröffnet» | M-24, M-26, M-22 | vorhanden: grosseltern; neu: M-35 mit einer E-Mail, die es in Jellyseerr nicht gibt |
| «Verknüpft über gleiche E-Mail» | M-24, M-26 | vorhanden: marco; neu: M-24 gast «In Jellyseerr eröffnen» oder «Alle eröffnen», oder einen Vorschlag bestätigen |
| «Verknüpfung vorgeschlagen, wartet auf den Administrator» | M-24 (Band offen, «Verknüpfung bestätigen»), M-26 («Verknüpfung vorgeschlagen: … · Bestätigen», Spalte «Kein Jellyseerr-Benutzer»), M-20, M-22 (Zähler) | M-35 mit `tv@example.ch` (wohnzimmer-tv) annehmen; «Alle eröffnen» lässt den Vorschlag stehen; ebenso nach einer Eröffnung, die bei nicht erreichbarem Jellyseerr wartete |
| «Von Hand zugeordnet» | M-24, M-26 | M-24 «Anderen Benutzer wählen», Benutzer, «Übernehmen», oder «Einem Konto zuordnen» im Band «ohne Konto» |
| «Wird eröffnet, sobald Jellyseerr erreichbar ist» | M-24 (Hinweis «Wartet auf Jellyseerr»), M-26 («Wird in Jellyseerr eröffnet»), M-20, M-30 | Mockup › «Nicht erreichbar», dann M-24 «In Jellyseerr eröffnen» oder M-35 absenden; eröffnet wird bei «Abgleich jetzt ausführen» mit «Verbunden» |
| «Nicht eröffnet» | M-24, M-26, M-20 | vorhanden: gast; neu: «Verknüpfung lösen» (ohne Rückfrage; der Benutzer bleibt in Jellyseerr) |
| «In Jellyseerr gelöscht» | M-24 (rote Warnung), M-26 («Benutzer gelöscht»), M-20 | Mockup › Sandra-Ausnahme: «Benutzer «sandra» in Jellyseerr löschen (Mockup)»; für ein Konto: «Ausnahme» von Sandra entfernen, gast → sandra wählen, dann löschen |
| Passwort «Gleich wie in Slouchcat» | M-24 mit Datum, M-26 «Passwort abgeglichen» | grosseltern; Konto aus M-35; nach «{Konto} meldet sich an» |
| Passwort «Wird bei der nächsten Anmeldung in Slouchcat übernommen» | M-24, M-26, M-00 (Knopf «… meldet sich an») | gast verknüpfen; Konto aus M-35 bei «Nicht erreichbar» nach dem Abgleich; «Passwort bei der nächsten Anmeldung neu übernehmen» |
| Passwort «Wird nicht abgeglichen» | M-24, M-26 | «Passwort nicht abgleichen» oder M-25 «Passwort abgleichen» aus und dann eröffnen |
| Passwort «Bleibt in Jellyseerr (Administrator)» | M-24, M-26 (Meldung bei Passwortänderung) | marco |
| Passwort «Anmeldung über Jellyfin oder Plex» | M-24, M-26 | gast «Anderen Benutzer wählen» → wohnzimmer-tv, oder den Vorschlag von `tv@example.ch` bestätigen |
| Anmeldung übernimmt das Passwort | M-00, M-24, M-22 (Protokoll) | M-00 «gast meldet sich an»; bei «Nicht erreichbar»: Meldung, Stand bleibt |
| Ausnahme gesetzt / entfernt | M-24, M-11, M-13 | M-24 Sandra «Sendet als» auf «Benutzer des Kontos (marco)»: Meldung «Ausnahme entfernt …» |

### 7.3 Limit

| Zustand | Mockup-Seite | Klickfolge |
|---|---|---|
| frei | M-08, M-13, M-23, M-24, M-26, M-00 | Mockup › «Oma & Opa»: «8 von 10 Filmen frei», «3 von 5 Staffeln frei», davon direkt in Jellyseerr 2 und 2 |
| kein Limit (Administrator) | überall, M-26 gesperrt | Marco |
| kein Limit (Benutzer ohne Limit) | M-13, M-08 | Mockup › Sandra (Ausnahme sandra) |
| Limit des Ausnahme-Benutzers | M-13, M-08, M-24 (Ausnahme) | Mockup › Noah (kinder, 5 Filme und 2 Staffeln, «Limit aus Jellyseerr: Dieser Benutzer gehört zu keinem Konto.») |
| Filme erreicht | M-13 (gesperrt mit Datum), M-08 (Balken voll), M-20 («Zum Konto»), M-23 («1 Konto am Limit», «nächster Platz am …»), M-24 («Nächster Platz frei …») | Mockup › «Oma & Opa», «Filme ausschöpfen»; zurück mit «Direkte Wünsche leeren» |
| Staffeln erreicht | wie oben | «Staffeln ausschöpfen» |
| gekappte Vorwahl | M-13 | Mockup › «Oma & Opa», `#/titles/show/1100`, «Serie anfordern» |
| zu viele Staffeln | M-13 | danach eine vierte Staffel oder «Alle fehlenden Staffeln» wählen |
| eigenes Limit, unbegrenzt | M-26 Feld «Limit» im Formular «Zugriff», M-24, M-23 (Spalte «Art») | M-26 grosseltern «Eigenes» bzw. «Unbegrenzt», «Zugriff speichern» |
| Vorgabe geändert | M-25, alle Orte oben | M-25 Filme auf 3 |
| wartende Wünsche zählen mit | M-13 | Mockup › «Oma & Opa» bei «Nicht erreichbar» drei Filme anfordern: Zähler sinkt |
| abgelehnte, zurückgezogene Wünsche und Mitwünsche zählen nicht | M-13 | Mockup › Noah: «Street Fighter» (abgelehnt) und «Godzilla Minus One» (Mitwunsch) zählen nicht, «4 von 5 Filmen frei» |

### 7.4 Profil-Tag

| Zustand | Mockup-Seite | Klickfolge |
|---|---|---|
| gesetzt | M-23 (Chip, Ereignis «Tag «sc-marco» in Radarr gesetzt»), M-22 (Zähler «Tags gesetzt (30 Tage)») | «Resident Evil», «Godzilla Minus One» (sc-marco, sc-noah), «Black Doves» (sc-sandra) |
| noch nicht gesetzt | M-23 (Chip «… · noch nicht gesetzt», Diagnose «Folgt nach dem Übermitteln.»), M-22 (Zähler «offen») | «Toy Story 5» (`pending`); ein `queued`-Wunsch |
| fehlgeschlagen | M-23 (Chip «… · fehlgeschlagen», Ereignis, Filter «Tag nicht gesetzt»), M-20 («n Wünsche ohne Profil-Tag»), M-22 (Zähler), M-30 (`wishes.tag` mit Fehler) | Mockup «Radarr weg», Marco fordert «Primetime» an |
| nachgeholt | M-23 | Mockup «Radarr erreichbar» oder «Abgleich jetzt ausführen» oder M-30 `wishes.tag` «Jetzt ausführen» |
| Mitwunsch bekommt seinen Tag | M-23 | Mockup › Sandra, «Resident Evil» «Benachrichtige mich»: zweiter Chip «sc-sandra» |
| kein Tag bei direkt in Jellyseerr gestellten Wünschen | M-23 | «Verity», «Clayface» (N-27) |
| eigener Name, gleicher Name | M-24, M-23, M-25 | M-24 Noah «Eigener Name» «sc-noah-k»; als «Oma & Opa» in M-11 ein Profil «Noah» anlegen: es bekommt «sc-noah-grosseltern», Noah behält «sc-noah» (N-21 nach K-18) |
| ausgeschaltet | M-23 («Ausgeschaltet»), M-30, M-22 | M-25 «Profil-Tag in Radarr und Sonarr setzen» aus |

### 7.5 Benachrichtigungen

| Zustand | Mockup-Seite | Klickfolge |
|---|---|---|
| neu (ungelesen) | M-09, Glocke | vorhanden: Marco «Dein Wunsch ist da: Godzilla Minus One»; neu durch Vorspulen |
| gelesen | M-09 | anklicken, «Als gelesen markieren», «Alle als gelesen markieren» |
| Ereignis ausgeschaltet | M-10, M-29, M-00 | M-10 E-Mail «Wünsche: verfügbar» ab; «Datei trifft ein» bei «Resident Evil»: Meldung nennt «App, Web-Push» |
| Freigabe nötig an Administratoren (Stufe 4) | M-29, M-00 | Mockup › Noah fordert «Moana» an |
| Zustandsmeldung (Stufe 4) | M-09 | Mockup «Freigeben»: «Freigegeben: …» |
| Übermittlung nach 24 h gescheitert | M-09, M-29, M-00 | «Nicht erreichbar», anfordern, «24 h vergehen» |
| PIN-Profil | M-29, M-00 | Mockup › Sandra fordert an, «Datei trifft ein»: Betreff ohne Titel |
| nicht sichtbar, keine Meldung | M-09 | «verfügbar» über einen Titel über der Altersstufe entsteht nicht (`notify` in `js/store.js`) |
| keine Meldung zu Limit oder Download | M-09 | «Download hängt», «Filme ausschöpfen»: kein Eingang |

## 8. Bestehende Funktionen bleiben

| Seite | Bestehendes im Mockup weiter bedienbar | Bewusst vereinfacht |
|---|---|---|
| M-01 Startseite | Bühne blättern, Pause, Wechsel, Posterreihe, Gesehen, Merken, alle Reihen nach Verwaltung | Reihen mit festen Beispieltiteln; «Abspielen» und «Trailer» als Dialog; Sammlungen, Genres, Meine Medien nur als ausgeblendete Zeilen |
| M-02 Spotlight-Liste | Raster, «Zurück» | keine Sortierung |
| M-05/M-06 Detailseiten | Abspielen, Von vorne, Trailer, Merken, Gesehen, Empfehlen, Löschen-Dialog, Staffelwahl, Folgenliste, Gesehen je Folge, Besetzung | Player und Trailer als Dialog; Löschen löscht nichts; höchstens 30 Folgen je Staffel |
| M-07 Suche | Suchfeld, alle Filter, Personen, Sammlungen | Personen und Sammlungen führen auf «Unverändert»; «Auf diesem Gerät» mit drei festen Titeln |
| M-09 Benachrichtigungen | «Neue Folge», gelesen/ungelesen, «Alle als gelesen markieren», «Kanäle einstellen» | |
| M-10/M-29 Kanäle | Aktiv, Ereignisse, Adresse, Telegram-Verknüpfung, Geräte, Speichern, Testnachricht | «In Telegram öffnen» nur als Meldung |
| M-11 Profile | ganzer Profildialog, PIN, Schauzeit, Löschen | Schauzeit als erfundenes Diagramm |
| M-12 Profilwahl | Kacheln, PIN | jede vierstellige PIN öffnet; Kontowechsel nur im Mockup |
| M-20 Übersicht | Kennzahlen, Hinweise, Schnellzugriffe, Serverzustand, Aktivität | «Gerade aktiv» und «Jetzt läuft» fest; Streams, Pflege, Speicher als feste Zahlen |
| M-21 Integrationen | alle sechs Ansichten mit Speichern, Prüfen, Löschen, Webhooks samt Pfad-Umsetzung | externe Seiten nur als Meldung; Prüfungen simuliert |
| M-26 Konten | Tabelle, Zugriff, Konto ändern, Einladungen, Rollen, Entfernen | jedes nicht leere aktuelle Passwort gilt |
| M-27 Spotlight | alle elf Felder des Bands «Quelle», Einträge, verwaiste Anpassungen | Abruf ändert die Liste nicht |
| M-28 Startseite | Platz, Ausblenden, Grenze 15, Vorschau, Speicherleiste | Vorschau als Schema |
| M-30 Hintergrundjobs | alle Jobarten, Pools, Läufe, Zeitpläne, Fehlschläge | Läufe und Restzeiten erfunden |
| M-35 Einladung annehmen | Felder und Prüfungen | nur die Einladung «Für Tante Ruth» und neu ausgestellte; das Passwort lebt nur im Entwurf und wird nicht gespeichert |
| unveränderte Seiten | Merkliste, Gesehen, Einstellungen, Bibliotheken, Person, Sammlungen (`#/unchanged/…`); 15 Verwaltungsseiten (`#/admin/x/…`) | nur Hinweisseite «Unverändert» |

**Was das Mockup grundsätzlich nicht zeigt:** Lade- und Fehlerzustände der App ausser den Verbindungstests (etwa
`tmdb_unreachable` 503, 429 der Suche); Offline-Verhalten der PWA (auch nicht «Nur online verfügbar» in M-08);
Texte in den 23 Sprachen; Tastaturtests nach Regel 8 (das Mockup ist bedienbar, wird aber nicht geprüft); echte
Bilder, wenn `img/` fehlt; Zeitabläufe ausser der 24-Stunden-Grenze und dem rollierenden Limit (Backoff,
Zwischenspeicher von 1 Stunde, Zeitplan, Frist 14 Tage); echte Listen von TMDB und Jellyseerr; Jellyseerr selbst
(etwa die Anmeldung dort oder seine eigenen E-Mails, K-19); den echten Player und Trailer.

## 9. Verbleibende Lücken

Stellen, die das Mockup nicht bedienbar macht oder nicht zeigt, obwohl Papiere oder Regeln N-1 bis N-88 und N-2a
sie vorsehen. Nicht repariert. Geprüft wurden alle Regeln und alle Seitenabschnitte von `07` gegen den heutigen
Code.

| Nr. | Lücke | Datei und Stelle | Wo es steht |
|---|---|---|---|
| 1 | Verbindungstest: «Handeln im Namen von Benutzern wirkt nicht», «Version nicht geprüft» (die Version ist fest «Seerr 3.5.2»), «Kein Sonarr in Jellyseerr», die Varianten von «Nicht erreichbar» und die Pflicht, bei neuem Host den Schlüssel neu einzugeben, sind nicht herstellbar; ebenso «Benutzer anlegen und ändern» als Fehler. Der Prüfpunkt «Passwort verschlüsselt» (Warnung bei `http` ausserhalb des eigenen Netzes, N-12) fehlt ganz. «Speichern» löst keinen Test aus | `js/admin-a.js` `aaSeerrRun`, `aaSeerrRows`, `ACT.aaSeerrSave` | `07-administration.md` Abschnitt 4.1; `08-datenmodell-api.md` Abschnitt 4.6 (`password_transport`); N-11, N-12 |
| 2 | «Lokale Anmeldung in Jellyseerr» ist im Mockup fest an; die Warnungen im Test, an der Kachel, in M-25 und in M-24 lassen sich nicht zeigen | `js/store.js` `seed()` (`localLogin: true`), keine Steuerung in `js/mock.js` | N-11; `07-administration.md` Abschnitte 4.1, 4.7, 6 und 7 |
| 3 | «Wünsche aktiv» lässt sich einschalten, ohne dass der Test zuletzt gelang | `js/admin-b.js` `route('/admin/wishes/settings')` | **Vorschlag** T-1; `07-administration.md` Abschnitte 4.6 und 7 |
| 4 | Instanzwechsel fehlt: kein Prüfpunkt «Instanz», kein Zustand «Andere Instanz erkannt», kein Hinweis in M-20, keine Knöpfe «Zuordnung bestätigen» und «Zuordnung löschen» | kein Vorkommen in `js/` | `07-administration.md` Abschnitte 2, 3, 4.1, 6 und 13; `04-benutzer-profilmapping.md` Abschnitt 8.4; `confirmSeerrInstance` |
| 5 | Warnungen in M-24 «Auto-Freigabe für ein Kinderprofil» und «Lokale Anmeldung in Jellyseerr aus» fehlen; «darf keine Serien anfordern» ist mit den Beispieldaten nicht herstellbar (alle Benutzer, die es schon gab, dürfen beides), ebenso der Grund `seerr_kind` und der Fehlschlag «darf diese Art von Titel nicht anfordern» | `js/admin-b.js` `abUserWarn`; `js/store.js` `seed()` | `07-administration.md` Abschnitt 6; `04-benutzer-profilmapping.md` Abschnitt 10 |
| 6 | M-23: Diagnose ohne Media-ID, Rohstatus, Versuche, nächsten Versuch, TVDB- und IMDb-ID, Sichtbarkeit je Anforderer, Zeit des letzten Lesens des Fortschritts und Download-Programm; «Fortschritt unvollständig» (AK-208) fehlt; keine Spalte «Art», kein Filter nach Medienart, keine Suche nach Titel; Kennzahl «Verfügbar (letzte 30 Tage)» zählt alle verfügbaren; ohne Verbindung sind «Freigeben» und «Ablehnen» nicht gesperrt, sondern melden «Geht nicht …»; 409 «Schon entschieden» fehlt | `js/admin-b.js` `abWishRow`, `AB_WF`, `ACT.abApprove`; `js/store.js` `wishCounts` | `07-administration.md` Abschnitte 5 und 13 |
| 7 | Folgenstand: «x von y Folgen da» gibt es nur als fester Wert beim Beispiel «Black Doves» und nur während «Wird heruntergeladen»; ein neuer Serienwunsch bekommt keinen Folgenstand, auch nicht je Staffel; die Staffelkarte «5 von 10 Folgen» einer teilweise eingelesenen Staffel fehlt | `js/model2.js` `dlInfo` (`d.eps`); `js/store.js` `seed()` (`w105`), `arriveWish` | N-83; AK-201; `02-funktionskonzept.md` Abschnitt 6 |
| 8 | Der Fall «Serie ohne TVDB-ID» (kein Knopf, Hinweis) ist nicht dargestellt | kein Vorkommen in `js/` | AK-39; `10-abnahmekriterien.md` Abschnitt 8 (M-03, M-04) |
| 9 | Die Staffelquelle von Jellyseerr wirkt nur auf Test, Kachel und M-27. Der Dialog M-13 zeigt die Staffeln immer aus den Beispieldaten; ein Beispiel, in dem TMDB und TVDB eine Serie verschieden teilen, gibt es nicht | `js/home.js` `DIALOGS.wish` (`t.seasons`); `js/mock.js` `ACT.mockSeasons` | N-63, N-64; `05-spotlight-integration.md` Abschnitt 3.3 |
| 10 | Spotlight-Abgleich: Der Erkennungsweg je Serie ist fest hinterlegt, der nicht zuordenbare Eintrag «Hafen im Nordlicht» ist ein fester Vorführeintrag; dass eine TVDB-zuerst-Serie heute fehlte und jetzt erscheint, lässt sich nicht vorher/nachher zeigen | `js/model2.js` `ID_MATCH`; `js/admin-b.js` `AB_UNMATCHED` | N-62, N-73; AK-75, AK-169 |
| 11 | Stufen sind nur bei «Sofortmeldung» und `wishes.import` gekennzeichnet. Ohne Hinweis erscheinen: Freigeben/Ablehnen, Status- und Staffel-Meldungen, `wish_status`, `wish_review`, fehlende Staffeln, `series.catalog` (Stufe 4), Profil-Tag und Download-Fortschritt (Stufe 4), die Seiten M-31 bis M-34 und `wishes.discover` (Stufe 5), Reihe «Erfüllte Wünsche» und Spotlight-Abgleich (Stufe 3) | `js/admin-a.js` (`aaSeerrView`, `AA_JOBS`); `js/viewer-pages.js` `VP_EVENTS` | Anhang H 4.7 und 8.7; `09-umsetzungsplan.md` |
| 12 | Ungespeicherte Änderungen (M-22, M-26, M-27, M-28) zeigen einen Chip, aber beim Verlassen kommt keine Rückfrage | `js/core.js` (`go`, `render`) | heutiges Verhalten (`07-administration.md` Abschnitte 3, 8, 9) |
| 13 | M-24 hat kein «Anfragen übernehmen» (Stufe 4); nach «Anderen Benutzer wählen» nennt die Meldung keine Folgen («2 offene Wünsche bleiben bei kinder»), nur «Einem Konto zuordnen» nennt die Zahl der Anfragen; `wishes.import` und `wishes.withdraw` laufen nie; Zurückziehen setzt sofort `withdrawn`, ohne den Fall «war schon freigegeben» | `js/admin-b.js` `ACT.abPickOk`, `ACT.abAssignOk`; `js/store.js` `withdrawWish` | `07-administration.md` Abschnitte 5 und 6; `04-benutzer-profilmapping.md` Abschnitt 8.2 |
| 14 | Der Kopf der Kanalbänder zählt die Ereignisse nicht | `js/viewer-pages.js` `channelSettingsHtml` | `07-administration.md` Abschnitt 11 |
| 15 | Erstabgleich von `wishes.sync` als eigener Lauf mit Fortschritt fehlt | kein Vorkommen in `js/` | `07-administration.md` Abschnitt 12; `10-abnahmekriterien.md` Abschnitt 8 (M-30) |
| 16 | Texte bei «CSRF-Schutz an»: Band «Abgleich» sagt «solange Jellyseerr den Schlüssel ablehnt», M-13 «Jellyseerr ist gerade nicht erreichbar», M-31 bis M-35 ebenfalls «nicht erreichbar»; auch der lesende Abgleich wird verweigert | `js/admin-a.js` `aaSeerrView`; `js/home.js` `DIALOGS.wish`; `js/viewer-discover.js` `dcStale`; `js/store.js` `syncNow` | `07-administration.md` Abschnitt 4.1 |
| 17 | Kein Ablauf «Passwort zurücksetzen» (weder für das eigene Konto noch durch den Administrator); die Weitergabe an Jellyseerr in diesem Moment ist darum nicht zu sehen | kein Vorkommen in `js/` | N-3, N-5; `04-benutzer-profilmapping.md` Abschnitt 13.1 |
| 18 | Eine geänderte E-Mail geht nicht an Jellyseerr, und ein Konto ohne Benutzer bekommt danach keinen Vorschlag (N-2a); «Konto ändern» ändert die Adresse nur in Slouchcat. Mit den Beispieldaten führt eine Passwortänderung immer zu «bleibt in Jellyseerr (Administrator)»; «Das neue Passwort gilt auch in Jellyseerr» erreicht man nur über ein Konto aus M-35, das Marco danach zum Administrator ernennt | `js/admin-b.js` `ACT.abOwnSave`; `js/model2.js` `pwChanged` | N-5, N-2a; `07-administration.md` Abschnitte 6 und 8; `08` `updateAccount` |
| 19 | Konto entfernen: Das Zufallspasswort für einen eröffneten Benutzer steht nur im Dialogtext, in der Meldung und im Protokoll; der Benutzer erscheint danach unter «Jellyseerr-Benutzer ohne Konto» mit gesperrten Rechten | `js/admin-b.js` `DIALOGS.abRemove`, `ACT.abRemoveOk` | N-10 nach K-2, F-31; `04-benutzer-profilmapping.md` Abschnitt 15 |
| 20 | «Tag Requests» steht nur als Fakt (Radarr an, Sonarr aus); der Tag des Kontos (`4-grosseltern`) erscheint nirgends, und der Wert lässt sich nicht umschalten | `js/store.js` `seed()` (`tagRequests`); `js/admin-a.js` `aaArrBand` | N-28; `07-administration.md` Abschnitt 4.8 |
| 21 | Die Reihen von «Entdecken» sind Beispieldaten: «Demnächst als Serie» ist mit laufenden Serien gefüllt (etwa «Die Simpsons» seit 1989, «Grey's Anatomy», «Navy CIS»), nicht mit angekündigten; «Im Trend» hat kein Zeitfenster; «Mehr laden» deckt nur die vorhandene Beispielliste weiter auf | `js/data-discover.js` `DISCOVER_ROWS`; `js/model2.js` `DISC_ORDER` | N-51, N-54; Anhang H 3.3 |
| 22 | Die Wunsch-Suche findet nur die 82 Beispieltitel (Titel und Originaltitel), nicht alles, was TMDB kennt; Kennungen nur, soweit `js/data-discover.js` sie führt; Personen sind nicht suchbar (gewollt, N-53), es gibt aber auch kein Beispiel, das einen Personentreffer ausblendet | `js/model2.js` `discoverSearch` | N-53; AK-184 |
| 23 | Vorschlag F-33 (Sofort-Import über Pfad und Kennung aus der Webhook-Meldung) ist nirgends gezeigt, auch nicht die zwei zusätzlichen Spalten im Meldungsprotokoll der Webhooks | kein Vorkommen in `js/` | N-88; `07-administration.md` Abschnitt 14; `09-umsetzungsplan.md` Abschnitt 6a |
| 24 | Die Eröffnung meldet nach 24 Stunden «wartet» kein Fehlerereignis an die Administratoren | `js/model2.js` `provisionAccount` | `04-benutzer-profilmapping.md` Abschnitt 10 |
| 25 | Jellyseerr-Adresse von Radarr oder Sonarr geändert («Radarr-Adresse in Jellyseerr geändert. Testen, um sie zu übernehmen.») und der bestätigte Host fehlen | `js/admin-a.js` `aaArrBand`, `ACT.aaArrTest` | `07-administration.md` Abschnitt 4.8; AK-171 |

Gegenüber dem ersten Stand von Fassung 3 geschlossen: Ein nicht zuordenbarer Spotlight-Eintrag ist als
Vorführeintrag zu sehen (Lücke 10 nur noch eingeschränkt); «Wird eingelesen» gibt es seit dem Download-Fortschritt;
die Regel F-9 und die Folge, dass «kein Recht» im Dialog M-13 nur über einen eigenen Wunsch erreichbar ist, stehen
so in `02-funktionskonzept.md` Abschnitt 4.5.

## 10. Widersprüche zwischen Mockup und Papieren

### 10.1 Bewusste Unterschiede

| Stelle | Mockup | Papiere |
|---|---|---|
| «Jellyseerr verwenden» | startet eingeschaltet (`js/store.js` `seed()`, `seerr.enabled: true`) | ab Werk aus (`07-administration.md` Abschnitt 4.1, `08-datenmodell-api.md` Abschnitt 2.12) |
| «Wünsche aktiv» | startet eingeschaltet (`settings.wishes.enabled: true`) | ab Werk aus (`07-administration.md` Abschnitt 7) |
| Konten in Jellyseerr | marco und grosseltern haben schon einen Benutzer | entstehen erst nach dem Einrichten; bestehende Konten eröffnet der Administrator (K-15; `04-benutzer-profilmapping.md` Abschnitt 11 beschreibt denselben Stand als Beispiel) |
| nicht zuordenbarer Spotlight-Eintrag | «Hafen im Nordlicht» ist erfunden (`AB_UNMATCHED`) | kommt in der App aus der Trakt-Liste |
| Beispieldaten, Zeit, Jellyseerr, Radarr, Sonarr | gespielt über M-00 | gehört nicht zur App |

Die Schalter stehen an, damit man ohne Einrichtung sofort anfordern kann.

### 10.2 Verbleibende Widersprüche

Keine bekannt. Der letzte Fall (M-24: Kennzahlen der Kopfzeile, Platz des Bands mit Vorschlag, Name des Knopfs
«Verknüpfung lösen») ist in `07-administration.md` Abschnitt 6 nachgezogen: Das Papier beschreibt jetzt, was das
Mockup zeigt.

**Wie die 30 Widersprüche des ersten Stands gelöst wurden.** Im **Mockup** geändert wurden die Nummern 1 bis 13,
15 bis 26, 28 und 30: Filter «Tag nicht gesetzt» und «Download verzögert» mit Link `?f=` (1), Sammelhinweis ohne
Meldung von Radarr (2), Hinweis nur für Konten mit Wunschrecht (3), Kachel mit erstem Hinweis und Beschreibung (4),
Reihenfolge und Fakten der Bänder (5), Prüftexte und Version (6), Einleitung und Hinweis des Bands «Benutzer und
Limit» (7), Band «Radarr und Sonarr» mit Zählern und «Speichern» (8), Tag-Chips mit Zustand, Tabelle «Limit je
Konto» mit «Art» und nächstem Platz, nur zwei Aktionen bei nicht zuordenbaren Anfragen (9), kein Tag für direkt in
Jellyseerr gestellte Wünsche (10), Rückfrage bei «Alle eröffnen», «Verknüpfung lösen» ohne Dialog (11),
`seerrFreeFor` (12), gesperrte Knöpfe bei ausgeschaltetem Eröffnen (13), Bezeichnungen und «Präfix» 1 bis 10 (15),
Limit im Formular «Zugriff» und gesperrt bei Administratoren (16), Zeile «Jellyseerr-Benutzer» mit Passwortstand und
Hinweis unter «Neues Passwort» (17), Konto entfernen mit `account_deleted` und Unterscheidung eröffnet/verknüpft
(18), Liste, Hinweis und «Anfordern» im Band «Fehlende Einträge» (19), Band «Abgleich» mit Zählung, Staffelquelle
und nicht zuordenbarem Eintrag (20), keine Chips auf der Bühne und kein Staffelhinweis in «Erfüllte Wünsche» (21),
Leerzustand von M-08 mit «Entdecken» (22), Reihenfolge der Genre-Reihen (23), Marke «In der Bibliothek» (24),
Suche ab zwei Zeichen (25), Namen und Gruppen der Jobs (26), Klartext beim abgelehnten Limit (28), Wünsche eines
gelöschten Profils gehen an das Hauptprofil (30). In den **Papieren** geändert wurden die Nummern 14 (M-24 als
Bänder in `07` Abschnitt 6), 15 (Name «Konten ohne Jellyseerr-Benutzer»), 27 (Dialog auch aus M-08 und M-27), 28
(Text «Limit erreicht: Jellyseerr hat den Wunsch nicht angenommen.»), 29 (M-35 zeigt die Fakten der Einladung)
und bei Nummer 2 die 24-Stunden-Regel (Hinweis und Filter zählen nur Tags, die nicht gesetzt werden konnten). Die
sieben Widersprüche der Fassung 2 bleiben behoben.
