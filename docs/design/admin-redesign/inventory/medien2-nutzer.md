# Bestandsaufnahme Teil «medien2-nutzer»: Podcasts, Webradio, Konten, Geräte, Streams

Stand: 15. November 2026, Arbeitsbaum `claude/admin-redesign` auf `main` `55b72c9a` (Release 1.38.0).
Nur gelesen, nichts ausgeführt. Zeilen je Funktion in `medien2-nutzer.csv` (224 Zeilen).

**Lesehilfe**

- Pfade in «Komponente» und hier sind relativ zu `web/src/features/admin/`, ausser sie beginnen mit
  `features/` (= `web/src/features/`), `app/` (= `web/src/app/`) oder `ui/` (= `web/src/ui/`).
- Tests: `X.test.tsx` liegt neben der Seite; `e2e/…` = `web/e2e/…`; Testnamen in «…».
- API: `operationId` aus `api/openapi.yaml`. Alle Admin-Operationen dieser Seiten tragen
  `x-visibility: admin`, ausser `setAccountRole` (`owner`), `updateAccount` und die Profil-Operationen
  (`account`).
- Gemeinsam für alle sechs Seiten (nicht je Zeile wiederholt): Zugang nur mit Rolle `admin` **und**
  Hauptprofil (`session.profile.isPrimary`), sonst Hinweis «Nur für Administratoren» / «Diesen Bereich
  verwaltet das Hauptprofil eines Administratorkontos.» ohne Abfrage; Kopf aus `ui/AdminPage.tsx`
  (h1 mit `titleRef`, Knopf «Hilfe zu dieser Seite» blendet den Einleitungssatz ein, Live-Anzeige
  «Live, alle N Sekunden», Hauptaktionen rechts); Meldungen in `StatusRegion` (gleicher Text wird erneut
  angesagt); Fehler einer Aktion als roter `Notice` mit `problemMessage`; Ladefehler mit «Erneut
  versuchen»; Tabellen aus `ui/DataTable` (↑/↓ zwischen Zeilen, unter 768 px Spalten nach Inhalt und
  seitliches Scrollen im Rahmen, `DataTable.module.css:35`); Bestätigungen sind die eigenen der Seite
  (eine verschwindende Zeile nimmt den Dialog nicht mit) und geben den Fokus an die h1, wenn der
  auslösende Knopf weg ist (`afterDialogsClosed` + `focusTitleIfLost`).
- `adminSearch` (`app/router.tsx:72`) lässt `?tab=` und `?item=` nur als Kennung durch
  (`^[A-Za-z0-9](?:[A-Za-z0-9_-]|\.(?!\.)){0,63}$`); ob die Kennung existiert, entscheidet die Seite
  (`useSearchSelection` mit `normalise`). Eine Zahl kommt als Text an und wird mit Anführungszeichen
  geschrieben (`?item=%221%22`).

---

## 1. Podcasts `/admin/podcasts` (POD)

**Zielort:** Medien → Podcasts. **Dateien:** `PodcastsPage.tsx`, `podcasts/columns.tsx`,
`podcasts/listView.ts`, `podcasts/PodcastDetail.tsx`, `podcasts/PodcastSetting.tsx`,
`podcasts/PodcastDownloads.tsx`, `podcasts/PodcastDownloadsBand.tsx`, `podcasts/downloads.ts`,
`podcasts/SubscribeDialog.tsx`, `podcasts/Podcasts.module.css`; Abfragen `queries.ts:197-252`.

### Aufbau heute

- Kopf (`PodcastsPage.tsx:385-404`): h1 «Podcasts», Hilfe (`admin.podcasts.intro`, nennt den Job
  «Podcast-Feeds prüfen»), Live-Anzeige nur während des 2-s-Pollings, Hauptaktion «Feed abonnieren»
  (fehlt ohne Podcast-Bibliothek).
- Statusleiste «Zusammenfassung» (`:334-374`): Podcasts · Neue Folgen (speichern / nur Link) · Belegt ·
  Downloads (laufen / warten / angehalten, Warnton bei Pause).
- Hinweis ohne Podcast-Bibliothek mit Link «Zu den Bibliotheken» (`:405-415`); Fehler/Status
  (`:416-440`).
- Bänder «Neue Folgen» je Podcast-Bibliothek (`:441-451`, `podcasts/PodcastDownloadsBand.tsx`):
  zugeklappt eine Zusammenfassung; offen Ordnerprüfung, «Neue Folgen» Speichern/Nur Link, «Umfang»,
  N, Bestätigung des einmaligen Ladens, «Ordner für neue Abos» (bei mehreren Ordnern),
  «Gleichzeitig laden» 1–5. Das ist eine **Einstellung der Bibliothek** (`updateLibrary`).
- Abschnitt «Downloads» (`:452-454`, `podcasts/PodcastDownloads.tsx`): Zusammenfassung,
  «Anhalten»/«Fortsetzen» (Jobtyp `podcast.download`), Tabelle Folge/Fortschritt/«Abbrechen»,
  «N von M gezeigt.» (nur die ersten 100). Sichtbar, wenn ein Band da ist oder etwas wartet.
- Band «Abonnierte Podcasts» mit Reitern «Alle» und je Bibliothek (`:456-477`); im Reiter
  Werkzeugleiste Filter («Alle/Speichert/Nur Link/Lädt/Fehler» mit Zahlen), Suche, Sortierung
  (`:256-286`); Tabelle mit Aufklappzeilen (`podcasts/columns.tsx`): Titel+Cover+Chips, Episoden mit
  Balken, Speicher, Neueste Folge, Zustand, Link «Folgen».
- Aufklappbereich (`podcasts/PodcastDetail.tsx`): Fakten, ganzer Fehler, «Jetzt abrufen», «Abo
  löschen» (nur ohne Ordner), Ordnerhinweis, eigene Einstellung des Podcasts (`PodcastSetting`).
- Dialoge: «Feed abonnieren» (`podcasts/SubscribeDialog.tsx`), Bestätigung «Abo … löschen?»
  (`:478-489`).
- CSS: alles `flex-wrap`, keine eigenen Breakpoints; `.errorCell`/`.clip` in `Podcasts.module.css:33-47`
  sind unbenutzt (Überbleibsel, keine Funktion).

### Suchparameter und Verweise

- `?tab=all|library-<id>` (Router `app/router.tsx:442`); unbekannt oder verschwundene Bibliothek →
  `all`, die Adresse wird erst korrigiert, wenn Podcasts und Bibliotheken geladen sind
  (`PodcastsPage.tsx:159-161`). Kein `?item=`: offene Zeilen, Filter, Suche, Sortierung sind nur
  Seitenzustand.
- Ausgehend: Link je Zeile auf `/admin/podcasts/:id` (PDP), «Zu den Bibliotheken», externe
  Installationsanleitung (`podcasts/downloads.ts:11`). Eingehend: Seitenleiste (`sections.ts:44`), die
  Unterseite gilt als Teil der Seite.

### Abfragen und Mutationen

| Abfrage | Schlüssel | Poll |
|---|---|---|
| `listPodcasts` | `[admin,podcasts]` | 2 s für 30 s nach «Jetzt abrufen», sonst keiner; ruht bei offenem Dialog; neu, wenn sich die Warteschlange ändert (`:129-138`) |
| `listPodcastDownloads` (limit 100) | `[admin,podcasts,downloads]` | 3 s, solange `total > 0` und kein Dialog offen |
| `listLibraries` | `[admin,libraries]` | 5 s nur während eines Scans (`queries.ts:80`) |
| `getLibrary` (Ordnerprüfung) | `[admin,library,id]` | keiner |

Mutationen: `createPodcast` (Dialog), `refreshPodcast` (ohne Bestätigung), `deletePodcast` (Bestätigung),
`updateLibrary` und `updatePodcast` (Bestätigung «Das lädt einmalig …», wenn die Schätzung
`estimatePodcastDownloads` mehr Folgen ergibt als die geltende Einstellung; sonst sofort),
`downloadPodcastEpisodes` («Fehlende nachladen», mit derselben Bestätigung), `pauseJobType` /
`resumeJobType`, `cancelPodcastDownload` (ohne Bestätigung).

### Fallen für den Umbau

- Die Download-Einstellung gibt es **zweimal**: je Bibliothek (Band «Neue Folgen») und je Podcast
  («Wie Bibliothek» erbt). Das Mockup zeigt nur eine Stelle.
- «Anhalten/Fortsetzen» schaltet den Jobtyp `podcast.download` – derselbe Schalter steht auf
  Hintergrundjobs. Beide Stellen müssen bleiben oder eine muss auf die andere zeigen.
- Polling ruht bei jedem offenen Dialog (`hold`); neue Dialoge müssen das übernehmen.
- Fokus: nach «Abo löschen» auf h1; nach «Abbrechen» in «Downloads» auf «Anhalten/Fortsetzen»; nach
  «Laden»/«Abbrechen» der Schätzung zurück auf das auslösende Element (`downloads.ts:65-68`).
- «Speichern» ist gesperrt, solange die Ordnerprüfung scheitert (Server sagt `savable` /
  `podcastFolder.writable`), ausser die Einstellung speichert schon.
- Ordner-Podcasts (`rootId`) haben kein «Abo löschen»; ohne Feed auch kein «Jetzt abrufen» und keine
  Einstellung.
- Der Reiter-Wert ist `library-<id>`, nicht die Zahl (sonst schreibt der Router `%22`).

### Abgleich mit dem Mockup

| Mockup | Einstufung | Begründung |
|---|---|---|
| D019 Feed abonnieren | bestehende Funktion | Feld «Feed-Adresse» und Bibliothek gibt es (Bibliothek als Radiogruppe); «Feed prüfen» als eigener Schritt ist **Beispiel ohne Basis**: der Server prüft beim «Abonnieren» und meldet Fehler am Feld. |
| D020 Podcast-Einstellungen | visuelle Umgruppierung | Modus/Umfang/N je Bibliothek und je Podcast, «Jetzt abrufen», «Fehlende nachladen» bestehen; «Speichermodus» mit drei Werten fasst Modus und Umfang zusammen. «Maximal gespeicherte Folgen» als Aufbewahrungsgrenze hat **keine Basis**: N heisst «Neue + letzte N» laden, nicht höchstens N behalten. Ein Speichern-Knopf fehlt im Code: jede Wahl gilt sofort. |
| D021 Episoden verwalten | bestehende Funktion | Ansichten Alle/Gespeichert/Ohne Datei, «Laden»/«Erneut»/«Abbrechen» je Folge bestehen (PDP). «Sortierung» (Älteste zuerst) und «Datei entfernen» sind **Beispiel ohne Basis**. |
| D022 Feed- und Downloadqueue | visuelle Umgruppierung | «Anhalten»/«Fortsetzen» und «Gleichzeitig laden» (je Bibliothek) bestehen; der Schalter «Downloads zugelassen» entspricht der Pause. «Fehler erneut versuchen» für alle Fehler gibt es hier nicht (nur «Erneut» je Folge): **offene Frage**. |
| D023 Podcast entfernen | bestehende Funktion | «Abo löschen» mit Bestätigung besteht. Der Schalter «Heruntergeladene Dateien löschen» ist **Beispiel ohne Basis**: die Dateien entfernt der nächste Abgleich immer (Text der Bestätigung). Ordner-Podcasts lassen sich gar nicht entfernen. |

**Im Mockup fehlt (bleibt):** Statusleiste mit vier Zahlen; Reiter je Podcast-Bibliothek in `?tab=`;
Filter, Suche, Sortierung der Liste; Spalten mit Speicheranteil und Zustand; Ordnerprüfung mit
Problemtexten und Anleitung; «Ordner für neue Abos»; die Bestätigung des einmaligen Ladens mit
Schätzung und freiem Platz; Frage «Abonnieren und laden» / «Nur abonnieren» im Abo-Dialog; Hinweis auf
unklare Folgen; Ordner-Podcasts («aus Ordner», Hinweis, kein Löschen); Hinweis ohne Podcast-Bibliothek;
«Explizit»-Chip.

---

## 2. Podcast-Seite `/admin/podcasts/:podcastId` (PDP)

**Zielort:** Medien → Podcasts (Unterseite). **Datei:** `PodcastPage.tsx`; nutzt `PodcastSetting`,
`feedError`, `downloadShare`; Route `app/router.tsx:452`.

### Aufbau heute

- 404: eigene Seite «Podcast nicht gefunden» mit Link «Alle Podcasts» (`:254-268`).
- Kopf (`:390-412`): h1 = Titel des Podcasts (auch Dokumenttitel), Link «Alle Podcasts», «Jetzt
  abrufen» (nur mit Feed); kein Hilfetext.
- Statusleiste «Zahlen dieses Podcasts» (`:354-386`): Folgen, Gespeichert («N ohne Datei»), Belegt,
  Neueste Folge.
- Kopfblock mit grossem Cover, Autor · Bibliothek «X» · aus Ordner, Chips, Feed-Fehler (`:430-463`).
- Zwei Spalten (`:464-567`, `flex-wrap`): links Band «Einstellung» (eigene Einstellung des Podcasts wie
  im Aufklappbereich, Feed-Adresse, Letzter Abruf); rechts Band «Folgen» mit Ansichten, Tabelle
  Datum/Folge/Zustand/Aktion, «N von M Folgen gezeigt.» und «Mehr zeigen» (Seiten zu 50).

### Suchparameter und Verweise

- `?tab=all|with_file|without_file` (Ansicht der Folgen, normalisiert). Zurück-Link auf die Liste;
  Browser-Zurück kehrt in die zuletzt gewählte Ansicht zurück.

### Abfragen und Mutationen

- `getPodcast` (`[admin,podcasts,id]`), `listLibraries`, `listPodcastDownloads` (nur für «angehalten»),
  `listAdminPodcastEpisodes` als unendliche Liste. Poll alle 3 s, solange ein Download des Podcasts
  wartet oder läuft, alle 2 s für 30 s nach «Jetzt abrufen»; dabei wird **nur die erste Folgenseite**
  ersetzt (`:186-204`).
- Mutationen ohne Bestätigung: `refreshPodcast`, `downloadPodcastEpisodes` (eine Folge),
  `cancelPodcastDownload`; dazu die Einstellung wie POD-048 ff. Alle laden danach `[admin,podcasts]`.

### Fallen für den Umbau

- Kein «Abo löschen» auf dieser Seite; nur in der Liste.
- Ein Knopf je Zeile wechselt die Beschriftung (Laden ↔ Abbrechen), damit der Fokus bleibt.
- Nach der letzten Seite fokussiert «Mehr zeigen» die Zählzeile (`shownRef`).
- Nicht in `e2e/admin-phone.spec.ts` geprüft.

### Abgleich mit dem Mockup

D020 und D021 zeigen Teile dieser Seite (Einstufung siehe Abschnitt 1). **Im Mockup fehlt (bleibt):**
eigene Seite je Podcast mit 404-Zustand, Statusleiste, Kopfblock mit Fehler, «Mehr zeigen», Fehlergründe
und nächster Versuch je Folge, Zustände «Entfernt» und «Unklar».

---

## 3. Webradio `/admin/radio` (RAD)

**Zielort:** Medien → Webradio (heute heisst die Seite «Radio»). **Dateien:** `RadioAdminPage.tsx`,
`radio/OverviewTab.tsx`, `radio/OfferTab.tsx`, `radio/StationsTab.tsx`, `radio/StationDetailsDialog.tsx`,
`radio/StationEditor.tsx`, `radio/OwnStationsTab.tsx`, `radio/CreateStationDialog.tsx`,
`radio/SettingsTab.tsx`, `radio/languages.ts`, `radio/Radio.module.css`; Abfragen `queries.ts:658-851`.

### Aufbau heute

- Kopf: h1 «Radio», Hilfe (Katalog, Weiterleitungen, Standort des Servers, Verantwortung), Live-Anzeige
  nur im Reiter «Übersicht». Keine Hauptaktion im Kopf.
- Statusleiste (`RadioAdminPage.tsx:108-155`): Im Katalog · Im Angebot (Warnton, wenn leer) ·
  Erreichbar · Live jetzt.
- Warnungen: «Kein RAM-Puffer» (`bufferMode = memory`), Ladefehler der Einstellungen, «Noch keine
  Sender im Angebot» mit Link «Zum Angebot» (ausser im Reiter Angebot).
- Reiter «Bereiche» (`:184-223`):
  - **Übersicht** (`radio/OverviewTab.tsx`): Band «Katalog» (Fakten, «Jetzt abgleichen», Fehler des
    letzten Abgleichs, Link «Zu den Hintergrundjobs»), Band «Angebot» (vier Zahlen, Link «Angebot
    bearbeiten»), Band «Live-Weiterleitungen» (Relays mit Hörern, «Stoppen» mit Bestätigung), Band
    «Serverlast» (Sender, Upload, Puffer).
  - **Angebot** (`radio/OfferTab.tsx`): Band «Länder» (Suche mit Treffertabelle, Kacheln «Nach
    Kontinent» mit Ländertabelle, «Alle wählen/abwählen»), Seitenspalte «Im Angebot» (Zahlen, Schalter
    «Nicht erreichbare weglassen»), Band «Gewählte Länder» (je Land «Sender wählen», «Land entfernen»;
    Senderliste mit Suche, «Alle (Gezeigte) markieren/demarkieren», Häkchen je Sender), klebende
    Leiste unten mit Differenztext, «Verwerfen», «Übernehmen». Ab 1024 px zweispaltig.
  - **Sender** (`radio/StationsTab.tsx`): Filter Suche, Land, Anbieter, Zustand, Sortierung, Schalter
    «Ganzen Katalog zeigen»; «Nicht erreichbare entfernen (N)» mit «Rückgängig»; virtuelle Tabelle
    (bis ~55 000 Sender) mit «Bearbeiten», «Angaben», «Aus dem/Ins Angebot nehmen».
  - **Eigene Sender** (`radio/OwnStationsTab.tsx`): Abschnitt mit «Eigenen Sender hinzufügen», virtuelle
    Tabelle, «Bearbeiten», «Löschen» mit Bestätigung.
  - **Einstellungen** (`radio/SettingsTab.tsx`): Zeilen Zeitversatz (10/30/60), Budget (MB),
    Höchstzahl Sender, Nachlauf (s), Cover-Suche MusicBrainz, Cover-Suche iTunes, Wikidata verwenden,
    Rechtshinweis, Speicherleiste.
- Editor-Dialog eines Senders (`radio/StationEditor.tsx`) über `?item=`, auf Seitenebene, nicht im
  Reiter.
- Dialoge: «Angaben zu …», «Eigenen Sender hinzufügen», Bestätigungen «Hören … stoppen?», «… löschen?»,
  Verlassensfrage.

### Suchparameter und Verweise

- `?tab=uebersicht|angebot|sender|eigene|einstellungen` (normalisiert, Verlauf ersetzt) und
  `?item=<stationId>` (Editor, Verlauf ersetzt; wirkt mit jedem Reiter). Filter des Reiters «Sender»
  und der Angebotsentwurf stehen **nicht** in der Adresse.
- Eingehend: `features/radio/RadioPage.tsx:631` (Zuschauerseite, nur für Admin) und
  `radio/OverviewTab.tsx:166`, `RadioAdminPage.tsx:176` verlinken `?tab=angebot`. Ausgehend: Link «Zu
  den Hintergrundjobs».

### Abfragen und Mutationen

| Abfrage | Schlüssel | Poll |
|---|---|---|
| `getRadioConfig`, `getRadioCatalog`, `getRadioOffer` | `[admin,radio,…]` | keiner |
| `listRadioRelays` | `[admin,radio,relays]` | 5 s nur im Reiter Übersicht, ruht bei offener Stopp-Bestätigung |
| `listAdminRadioStations` | `[admin,radio,stations,filter]` | keiner; Seiten zu 50 beim Scrollen |
| `previewRadioOffer` | `[admin,radio,offer,preview,steps]` / `…,list` | keiner; 300 ms nach jeder Änderung |
| `getAdminRadioStation` | `[admin,radio,station,id]` | keiner |
| `getRadioFacets` (Länder im Anlegedialog) | Zuschauerschlüssel | keiner |

Mutationen: `syncRadioCatalog` (ohne Bestätigung); `stopRadioListener` (Bestätigung);
`applyRadioOffer` aus dem Entwurf (Leiste, Konflikt 409 mit neu gerechneter Vorschau); aus «Sender»
`applyRadioOffer` sofort über `applyRadioOfferNow` (liest die Version, wiederholt bei 409, Schritte
nacheinander im Scope `admin-radio-offer-now`), ohne Bestätigung, mit «Rückgängig» für «Nicht
erreichbare entfernen»; `updateAdminRadioStation`, `probeRadioStation`, `uploadRadioStationLogo`,
`deleteRadioStationLogo` (ohne Bestätigung); `createRadioStation`; `deleteRadioStation` (Bestätigung,
lädt auch `[radio]` und `[home]`); `updateRadioConfig` (Speicherleiste). Nach Angebotsänderungen wird
auch die Zuschauer-Radioseite (`radioKeys.all`) neu geladen.

### Fallen für den Umbau

- Zwei Arten, das Angebot zu ändern: Entwurf mit «Übernehmen» im Reiter «Angebot», sofortige Schritte
  im Reiter «Sender». Der Client rechnet keine Regel (Regel 12); bis die Vorschau da ist, zeigen
  Häkchen die eigene Eingabe (Overrides).
- Der Entwurf lebt in der Seite (über Reiter hinweg) und ist durch `useUnsavedChanges` geschützt;
  `?tab=`/`?item=` lösen die Frage nicht aus. Die Einstellungen und der Editor haben **keinen**
  Verlassensschutz.
- Schrittgrenze 1000, Grenze 5000 IDs je Markierschritt (`OFFER_STEP_IDS_MAX`).
- Fokusführung: «Land entfernen» → gleicher Knopf der nächsten Zeile; Kontinent mit Escape zu → Fokus
  auf die Kachel; Senderliste mit Escape zu → «Sender wählen»; Zeile verschwindet aus «Sender» → h1;
  «Verwerfen» → h1.
- Reiter-IDs sind deutsche Slugs und stehen in fremden Links.
- Dieselben Radiohörer lassen sich auch auf Streams beenden (gleiche Abfrage `listRadioRelays`).
- Die Übersicht zeigt die E-Mail-Adressen der hörenden Konten (Datenschutz beim Umbau beachten).
- `/admin/radio` fehlt in `e2e/admin-phone.spec.ts`.

### Abgleich mit dem Mockup

| Mockup | Einstufung | Begründung |
|---|---|---|
| D024 Katalog & Angebot | visuelle Umgruppierung | Katalogfakten und «Jetzt abgleichen» (= «Katalog aktualisieren») stehen in Übersicht, das Angebot hat einen eigenen Reiter mit Ländersuche, Kontinenten und Vorschau («Angebot prüfen» entspricht der Vorschau). «Land» und «Verfügbarkeit» als einfache Selects decken das nicht ab. |
| D025 Sender verwalten | bestehende Funktion | Suche, Sortierung, «Stream prüfen» (im Editor) und «Bearbeiten» bestehen; Sortierung nach «Land» ist **Beispiel ohne Basis** (nur Beliebtheit und Name). |
| D026 Eigene Sender | bestehende Funktion | Name, Stream-Adresse, Hinzufügen, Löschen bestehen; «Logo-URL» ist **Beispiel ohne Basis** (Logo per Hochladen im Editor oder von der Webseite). |
| D027 Puffer & Relays | bestehende Funktion | Zeitversatz, Budget, Höchstzahl, Nachlauf, Speichern bestehen; Zeitversatz ist im Code eine Wahl 10/30/60, keine freie Zahl. «Relays anzeigen» ist eine visuelle Umgruppierung (Relays stehen immer in der Übersicht). |
| D028 Cover & externe Daten | bestehende Funktion | Die drei Schalter MusicBrainz, iTunes, Wikidata bestehen genau so. |

**Im Mockup fehlt (bleibt):** Statusleiste; Warnungen «Kein RAM-Puffer» und leeres Angebot; Hörer je
Relay mit «Stoppen»; Serverlast; Fehler des letzten Abgleichs; Kontinent-Kacheln, «Alle wählen»,
Senderliste je Land mit Markieren/Demarkieren; Schalter «Nicht erreichbare weglassen»; Konflikt 409;
Schrittgrenze; Filter Anbieter/Zustand und «Ganzen Katalog zeigen»; «Nicht erreichbare entfernen» mit
«Rückgängig»; «Aus dem/Ins Angebot nehmen» je Sender; Dialog «Angaben» mit Herkunft und Wikidata;
Editor mit Sperren je Feld, Region, Streamwahl, «Empfohlen», Logo-Zustand und -Rücksetzen; Felder
Webseite/Land/Region/Sprachen eigener Sender; Verlassensschutz des Entwurfs; Rechtshinweis.

---

## 4. Konten `/admin/accounts` (ACC)

**Zielort:** Nutzer & Geräte → Konten. **Dateien:** `AccountsPage.tsx`, `accounts/AccountDetail.tsx`,
`accounts/AccessForm.tsx`, `accounts/OwnAccountForm.tsx`, `accounts/Invitations.tsx`,
`accounts/columns.tsx`, `accounts/Accounts.module.css` (ohne Breakpoints); eingebunden
`features/profiles/ProfileList.tsx`, `ProfileForm.tsx`, `PinSetDialog.tsx`, `WatchReport.tsx`,
`PlaybackPrefsFields.tsx`, `features/shared/LibraryChoice.tsx`, `LimitFields.tsx`, `choices.ts`,
`useLibraryOptions.ts`.

### Aufbau heute

- Kopf: h1 «Konten», Hilfe, Hauptaktion «Konto einladen» (gesperrt bis die Bibliotheksliste da ist).
- Statusleiste: Konten · Administratoren (davon Besitzer) · Profile (eigene) · Offene Einladungen
  (nächste gültig bis).
- Reiter «Bereiche der Konten» (`:504-585`), je mit verborgener h2 und eigenem Fehler/Retry:
  - **Konten**: Tabelle E-Mail/Rolle/Altersstufe/Bibliotheken/Profile/Zuletzt gesehen/Kanäle/Menü,
    Zeilen klappen auf.
    - Eigenes Konto: «Konto ändern» (E-Mail, neues Passwort mit Wiederholung, aktuelles Passwort,
      Verwerfen/Speichern) und «Meine Profile» (`ProfileList`: Karten, «Profil hinzufügen»,
      «Bearbeiten», «Schauzeit anzeigen», Menü PIN setzen/ändern/entfernen, Löschen).
    - Fremdes Konto ohne Verwaltung: «Zugriff» (Höchste Altersfreigabe, Bibliotheken,
      Wiedergabe-Limits ohne Schauzeit, Verwerfen/«Zugriff speichern»), Profilnamen nur zum Lesen.
    - Fremder Administrator: Hinweis statt Zugriff, Profilnamen.
    - Menü «Weitere Aktionen» (nicht beim eigenen Konto, nicht beim Besitzer): Ernennen/Entziehen (nur
      der Besitzer sieht es), «Konto entfernen».
  - **Einladungen**: Tabelle Notiz/Zustand/Altersstufe/Bibliotheken/Gültig bis-angenommen/«Widerrufen».
- Dialoge: «Konto einladen» (Formular, danach einmal angezeigter Link mit «Link kopieren»), Profil
  neu/bearbeiten, PIN, Bestätigungen Ernennen, Entziehen, Konto entfernen, Widerrufen, PIN entfernen,
  Profil löschen, Verlassensfrage.

### Suchparameter und Verweise

- `?tab=accounts|invitations` (normalisiert). `?item=<Konto-Id>|me` klappt die Zeile einmal bei Ankunft
  auf; unbekannt → aus der Adresse (Hash bleibt). `#meine-profile` scrollt einmal je Ankunft zu «Meine
  Profile» (`accounts/AccountDetail.tsx:14,55-64`).
- Weiterleitung `/admin/access` → `/admin/accounts` (`app/router.tsx:470-476`, ADR 0014). Nach dem
  Erstellen einer Einladung wechselt die Seite auf `?tab=invitations`.
- Gleiche Profilverwaltung unter `/settings/profiles` (für alle Konten). Eigene Benachrichtigungskanäle
  ändert man unter `/admin/notifications` bzw. `/settings/notifications` (nicht Teil dieser Aufnahme);
  in Konten werden die Kanäle nur angezeigt.

### Abfragen und Mutationen

- `listAccounts`, `listInvitations`, `listAccountLibraries`, `listProfiles`, `getWatchReport`: kein
  Poll; kein Neuladen bei Fensterfokus, solange ein Dialog, ein Profil-Dialog oder die Verlassensfrage
  offen ist.
- Mutationen: `createInvitation` (Antwort nicht im Mutation-Cache), `revokeInvitation` (Bestätigung),
  `setAccountRole` (Bestätigung, nur Besitzer), `deleteAccount` (Bestätigung), `updateAccountAccess`
  (Formular), `updateAccount` (Formular, setzt die Sitzung), `createProfile`/`updateProfile` (Dialog),
  `setProfilePin` (Dialog), `removeProfilePin` und `deleteProfile` (Bestätigung).

### Fallen für den Umbau

- Entwürfe je Konto (`useDrafts`) überleben Zuklappen und Reiterwechsel; Passwörter nicht (beim
  Zuklappen und nach dem Speichern geleert, nie im Mutation-Cache). Was während eines Speicherns getippt
  wird, bleibt Entwurf (Entscheid 18).
- Ohne Bibliotheksliste weder Zugriffsformular noch Einladung (kein Recht wird erweitert).
- Löschschutz: eigenes Konto und Besitzer ohne Menü; Hauptprofil ohne «Löschen».
- Rollen ändern nur der Besitzer (`x-visibility: owner`); ein anderer Administrator sieht nur «Konto
  entfernen».
- Profile fremder Konten sind bewusst nicht bearbeitbar (`x-visibility: account`).
- Altersstufen kommen aus dem Freigabesystem des Servers (`useRatingSystem`, ADR 0034), nicht fest FSK.
- `data-perf="first-work-row"` an der Tabelle wird von der Leistungsmessung gelesen
  (`docs/perf/verwaltungsseiten.md`).

### Abgleich mit dem Mockup

| Mockup | Einstufung | Begründung |
|---|---|---|
| D029 Einladungen erstellen | offene Frage | Einladen und Widerrufen bestehen, aber mit anderen Feldern: Notiz, Höchste Altersfreigabe, Bibliotheken; Link wird angezeigt und kopiert. «E-Mail-Adresse», «Rolle», wählbare «Ablaufzeit» und «Einladung senden» (Versand) sind **Beispiel ohne Basis** (Einladung ergibt immer ein Konto ohne Verwaltung, gilt fest sieben Tage). |
| D030 Rollen & Berechtigungen | visuelle Umgruppierung | Rolle, Bibliotheken, Altersfreigabe, Speichern und «Konto entfernen» bestehen; Rolle ist im Code kein Feld, sondern Menüpunkt mit Bestätigung und nur für den Besitzer; Altersstufen folgen dem Freigabeland. Wiedergabe-Limits fehlen im Mockup. |
| D031 Profile je Konto | offene Frage | Profile bearbeiten gibt es nur für das **eigene** Konto («Meine Profile»); fremde Konten zeigen nur Namen. Der Schalter «Hauptprofil» ist **Beispiel ohne Basis**. Name und Akzentfarbe bestehen im Profildialog. |
| D032 Zustellung je Konto | Beispiel ohne Basis | Die Verwaltung kann Kanäle anderer Konten nicht schalten; die Tabelle zeigt nur die vom Konto eingeschalteten Kanäle (E-Mail, Web-Push, Telegram). |

**Im Mockup fehlt (bleibt):** Statusleiste; Reiter Konten/Einladungen in `?tab=`; Deeplink
`?item=me#meine-profile`; Spalten Zuletzt gesehen und Kanäle; Chip «Besitzer»; «Konto ändern» (E-Mail,
Passwort); einmal angezeigter Einladungslink mit Kopieren und Ausweichweg Strg+C; Zustände der
Einladungen; Wiedergabe-Limits je Konto; Profil-PIN, Schauzeitbericht, Sprache der Beschreibungen,
Wiedergabesprachen; Verlassensschutz; Hinweis für Administratoren; Weiterleitung `/admin/access`.

---

## 5. Geräte `/admin/devices` (DEV)

**Zielort:** Nutzer & Geräte → Geräte. **Dateien:** `DevicesPage.tsx`, `devices/columns.tsx`,
`devices/Devices.module.css`.

### Aufbau heute

- Kopf: h1 «Geräte», Hilfe; keine Statusleiste, keine Hauptaktion.
- Band (Titel verborgen) mit Reitern «Geräte» und «Fähigkeiten» (`:229-258`); je Reiter Hinweiszeile
  und klappbare Abschnitte je Gerätetyp (TV, Computer, Telefon, Tablet, Anderes, Unbekannt) mit Zahl und
  Streams; darin feste Tabellen (Spalten fluchten über die Abschnitte, K12-9).
- Geräte: Gerät, Eigentümer (letzter Wiedergabestart), Letzter Kontakt, Streams, «Vergessen».
- Fähigkeiten: Gerät, Container, Video, Audio, Max. Auflösung, HDR.
- Bestätigung «Gerät „…“ vergessen?».

### Suchparameter, Abfragen, Mutationen

- `?tab=devices|capabilities` (normalisiert, ersetzt). Klappzustand nur im Seitenzustand, gilt in beiden
  Reitern.
- `listDevices` ohne Poll; kein Neuladen bei Fensterfokus während der Bestätigung. `deleteDevice` mit
  Bestätigung, lädt danach auch `[admin,streams]`.

### Fallen für den Umbau

- «Vergessen» des eigenen Geräts meldet sofort ab (Text der Bestätigung).
- Unbekannte Typen eines neueren Servers fallen unter «Unbekannt»; nicht gemeldete Werte sind «–» mit
  gesprochenem «unbekannt».
- Fokus: nach «Vergessen» auf h1; Abbrechen lässt den Fokus auf dem Knopf.

### Abgleich mit dem Mockup

| Mockup | Einstufung | Begründung |
|---|---|---|
| D033 Registrierte Geräte | visuelle Umgruppierung | Liste nach Typ mit letztem Kontakt und «Vergessen» besteht; «Gerätetyp» ist im Code eine Gruppierung, kein Filter. «Details anzeigen» ist **Beispiel ohne Basis** (keine Detailansicht). |
| D034 Gerätefähigkeiten | visuelle Umgruppierung | Fähigkeiten bestehen als eigener Reiter mit allen Spalten zugleich; «Geräteprofil»/«Details» als Selects und «Erneut prüfen» sind **Beispiel ohne Basis** (das Profil meldet der Browser beim Wiedergabestart). |

**Im Mockup fehlt (bleibt):** Eigentümer (E-Mail · Profil), Streams je Gerät und je Typ, Reiter in
`?tab=`, klappbare Typ-Abschnitte, Bestätigung mit Abmeldewarnung, Leerzustand.

---

## 6. Streams `/admin/streams` (STR)

**Zielort:** Nutzer & Geräte → Streams. **Dateien:** `StreamsPage.tsx`, `streams/format.ts`,
`streams/Streams.module.css`; Kachel `ui/StreamTile` (Storybook «UI/StreamTile»).

### Aufbau heute

- Kopf: h1 «Streams», Hilfe, «Live, alle 5 Sekunden» (fehlt bei offener Bestätigung).
- Band «Serverlast» mit sechs Kacheln (Transcode HW, Transcode SW, Remux, Direct, Radio, Bandbreite
  gesamt).
- Abschnitt «Läuft gerade» mit Filter «Nach Art filtern» (Alle/Video/Hörbücher/Podcasts/Radio mit
  Zahlen), Teilausfall-Hinweisen, Kachelraster (min. 20rem, auf dem Handy eine Spalte): Kacheln für
  Wiedergaben und für Radiohörer, je «Beenden».
- Bestätigung «Stream/Radio von … beenden?».

### Suchparameter, Abfragen, Mutationen

- Keine Suchparameter (Route ohne `validateSearch`); Filter nur im Seitenzustand. Eingehend: Karte «Jetzt
  läuft» der Übersicht mit «Alle Streams» (`AdminHomePage.tsx:1097`).
- `listStreams` und `listRadioRelays` alle 5 s, angehalten während der Bestätigung. `stopStream` bzw.
  `stopRadioListener` mit Bestätigung.

### Fallen für den Umbau

- Teilausfall: eine fehlgeschlagene Hälfte zeigt «–» statt alter Daten; Leertext nur, wenn alle
  betroffenen Hälften bekannt sind.
- Fokus: nach «Beenden» auf h1; verschwindet die fokussierte Kachel mit dem Poll, ebenfalls h1.
- Pfeiltasten zwischen den Kacheln (nativer Listener, Spaltenzahl aus dem Grid).
- Radiohörer erscheinen hier **und** in Webradio → Übersicht.
- Der Server widerruft beim Beenden das Token; der Player stoppt innert Sekunden
  (`e2e/stream-token.spec.ts`).

### Abgleich mit dem Mockup

| Mockup | Einstufung | Begründung |
|---|---|---|
| D035 Aktive Streams | bestehende Funktion | Profil, Gerät, Quelle (bei Podcast), Modus und «Beenden» stehen auf jeder Kachel, Filter nach Art besteht (mit Hörbüchern und Podcasts statt «Audio»). Filter «Qualität» und «Details anzeigen» sind **Beispiel ohne Basis** (alle Angaben stehen auf der Kachel). |
| D036 Transcoding & Auslastung | visuelle Umgruppierung | «Serverlast» zeigt Transcoding-Last und Bandbreite zugleich; «Last» als Select, «Aktualisieren» (die Seite pollt alle 5 s) und «Session prüfen» sind **Beispiel ohne Basis**. |

**Im Mockup fehlt (bleibt):** Radiohörer als Kacheln mit eigenem Beenden, Teilausfall-Hinweise,
Fortschritt und Position, Grund für Nicht-Direktwiedergabe, Hörbuch-Kapitel und Teile, Live-Anzeige,
Pfeiltasten zwischen Kacheln, Fokusführung beim Verschwinden einer Kachel.

---

## Unklar

- Ob die Abfrage `listPodcastDownloads` mit `limit: 100` serverseitig mehr liefern könnte; die Seite
  zeigt nur «N von M gezeigt.» und lädt nicht nach (`podcasts/PodcastDownloads.tsx:172`).
- Ob die Escape-Taste im Editor-Dialog des Senders den Fokus zuverlässig auf den Zeilenknopf
  zurückgibt, ist am Code von `EditorDialog` nicht geprüft; belegt nur durch den Kommentar und
  `e2e/admin-radio.spec.ts` (Zeilen 7–11).
