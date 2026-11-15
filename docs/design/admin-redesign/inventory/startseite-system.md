# Bestandsaufnahme Teil «startseite-system»

Stand: Arbeitsbaum `claude/admin-redesign` auf `main` `55b72c9a` (Release 1.38.0), gelesen am
15. November 2026. Nur gelesen, nichts ausgeführt, nichts geändert.

Seiten: Startseite (HOM), Spotlight (SPT), User-Empfehlungen (PCK), Sammlungen (COL), Meine Medien
(MYM), Ankündigungen (ANN), Statistiken (STA), Sicherung (BKP). Zeilen in
`startseite-system.csv`: HOM 21, SPT 33, PCK 13, COL 23, MYM 20, ANN 24, STA 14, BKP 22, zusammen 170.

**Lesehilfe**

- Pfade sind relativ zu `web/src/` (Code) bzw. `web/` (`e2e/…`). `ui/…` = `web/src/ui/`.
- Texte stehen wörtlich aus `web/src/locales/de.json` in «…».
- Für alle acht Seiten gleich (in der CSV je Seite einmal erfasst): Zugriff nur mit Rolle `admin`
  **und** Hauptprofil, sonst h1 plus «Nur für Administratoren» und keine Anfrage; Kopf aus
  `ui/AdminPage.tsx` (h1 mit `tabIndex=-1` als Fokusziel, Knopf «Hilfe zu dieser Seite» für den
  Einleitungssatz, optional «Live, alle n Sekunden», Hauptaktionen rechts); Meldungen in
  `StatusRegion` (jede Meldung ein neuer Knoten); Fehler einer Aktion als roter `Notice`; Ladefehler
  mit «Erneut versuchen»; die Abfrage «Seite mit ungespeicherten Änderungen verlassen?» kommt aus
  `ui/useUnsavedChanges.ts` (TanStack `useBlocker`, dazu `beforeunload`), ein Wechsel nur von `?tab=`
  oder `?item=` gilt nicht als Verlassen.
- Alle Operationen haben `x-visibility: admin` (`api/openapi.yaml:6202-8414`), ausser `getHealth`
  (`public`, `:177`).
- Tabellen (`ui/DataTable.tsx`): ↑/↓ springen zwischen Zeilen, Bedienelemente in der Tab-Folge;
  `layout="fixed"` gibt ab 768 px feste Spaltenbreiten und mind. 48rem, darunter Spalten nach
  Inhalt und seitliches Scrollen im Rahmen (`ui/DataTable.module.css:35`).
- Neue oder geänderte Texte mit «» oder ss-Wörtern brauchen eine Fassung in `de-DE.json`; betroffen
  sind heute u. a. `admin.picks.empty`, `admin.collections.rename.done`, `admin.myMedia.*`
  (intro, list, saved, preview.home/empty, homeRows/-Link), `admin.homeRows.rows.libraries/library`,
  `pageLink`, `spotlight`, `spotlightLink`, `admin.spotlight.enabledHint`, `entries.quoted`,
  `rowSettings`, `rowSettingsLink`, `admin.backup.columns.size`, `create.withoutHint`,
  `admin.announcements.publish.body/reach`. Alle Schlüssel dieses Teils gibt es auch in `en.json`.

---

## 1. Startseite `/admin/home-rows` (HOM) → Startseite → Startseite

**Aufbau heute**

- Kopf: h1 «Startseite», Hilfe (`HomeRowsPage.tsx:279-283`). Keine Hauptaktion im Kopf.
- Laden/Fehler (`:284-295`); ohne Reihen kein weiterer Inhalt (`:296`).
- Band «Reihen» mit Reitern (`:298-307`):
  - «Reihenfolge» (Zahl «{n} Reihen»): Hinweis zur Platzwahl, Tabelle «Reihen der Startseite»
    mit Spalten «Platz» (RankSelect), «Reihe» (Zeilenkopf, Knopf zum Aufklappen; Chip «Bibliothek»
    bei Bibliotheksreihen), «Zustand», «Ausblenden» (Kontrollkästchen) (`:169-226,252-264`);
    Klappbereich mit Beschreibung und Link zur zuständigen Seite (`:228-240`, `homerows/rows.ts:16-25`).
  - «Vorschau» (Zahl «{n} sichtbar»): Skizze mit Platzhaltern (`homerows/HomeRowsPreview.tsx`).
- Speicherleiste `SaveBar` als direktes Kind der Seitenspalte, haftet unten (`:308-315`); Fehler
  darunter (`:316`); Statusregion (`:317`); Satz mit Link zu Spotlight (`:318-321`).
- Dialoge: Rückfrage «Änderungen verwerfen?» (`:324-334`), Rückfrage beim Verlassen (`:335`).
- Reihen: Spotlight, User-Empfehlungen, Weiterschauen, Neu hinzugefügt, Kürzlich veröffentlicht,
  Merkliste, Sammlungen, Genres, Meine Medien, Meine Sender und je Bibliothek «Neu in {Bibliothek}»
  (`features/catalog/labels.ts:18-39`, Schema `HomeRowKey` `api/openapi.yaml:12311`).

**Suchparameter, Deeplinks, Verweise**

- `?tab=preview` (Vorgabe ohne Parameter = `order`); unbekannt → «Reihenfolge», Adresse ersetzt
  (`:46-49,105`; Route `app/router.tsx:496-503`).
- Ausgehend: Klappbereiche → `/admin/spotlight`, `/admin/picks`, `/admin/collections`,
  `/admin/my-media`; Satz unten → `/admin/spotlight`.
- Eingehend: Spotlight, User-Empfehlungen und Meine Medien verlinken hierher.

**Abfragen und Mutationen**

- `getHomeRows` ohne Takt; bei Fensterfokus/Wiederverbindung nur ohne Entwurf (`:98-102`).
- `setHomeRows` (alle Reihen mit `id`, `key`, `hidden`, `position`), danach Cache gesetzt und
  `['home']` neu (`:123-135`). Verwerfen nur nach Rückfrage.

**Fallen für den Umbau**

- Ein Entwurf für die ganze Liste (`useDrafts`, `DRAFT='rows'`); er überlebt Reiterwechsel. Offene
  Zeilen sind Seitenzustand und überleben den Reiterwechsel ebenfalls.
- Grenze 15 sichtbare Reihen steht im Client (`homerows/rows.ts:12-13`), Meldung im Ton `warning`.
- Fokus: nach Speichern bleibt er oder geht auf h1; nach Verwerfen auf h1, nach «Abbrechen» zurück
  auf «Verwerfen».
- Spotlight hat zwei Schalter: «Ausblenden» hier und «Auf der Startseite zeigen» auf Spotlight.
- Die Bibliotheksreihen sind Zeilen derselben Liste (ID `library:<id>`), keine eigene Einstellung.

**Abgleich mit dem Mockup**

| Mockup | Einstufung | Begründung |
|---|---|---|
| D037 Reihenfolge & Sichtbarkeit | visuelle Umgruppierung | Platz, Sichtbarkeit, Vorschau und Speichern gibt es; das Mockup zeigt sie als Einzelformular (Bereich wählen, Schalter «Bereich sichtbar», Zahl «Position»), heute ist es eine Tabelle mit Entwurf aller Reihen. |
| D038 Bibliotheksreihe | visuelle Umgruppierung | «Neu in {Bibliothek}» sind Zeilen derselben Tabelle mit Platz und Ausblenden; ein eigener Bereich wäre nur eine andere Anordnung. |

**Im Mockup fehlt (bleibt)**: Grenze von 15 sichtbaren Reihen mit Meldung; Klappbereich «was die
Reihe zeigt» mit Links zu Spotlight, User-Empfehlungen, Sammlungen, Meine Medien; «Verwerfen» mit
Rückfrage; Zustandsspalte; Rückfrage beim Verlassen; Pause der Abfragen bei Entwurf; Link zu
Spotlight; Tastaturbedienung der Tabelle und Reiter.

---

## 2. Spotlight `/admin/spotlight` (SPT) → Startseite → Spotlight

**Aufbau heute**

- Kopf: h1 «Spotlight», Hilfe, «Live, alle 2 Sekunden» während des Takts (`SpotlightPage.tsx:247-251`).
- Fehler/Meldung oben, Ladefehler der drei Abfragen getrennt (`:253-257`).
- Band «Quelle» (einklappbar; Chips «Ungespeicherte Änderungen» und «Auf der Startseite» /
  «Fehlt auf der Startseite») mit `SourceForm` (`spotlight/SourceForm.tsx`): Schalter «Auf der
  Startseite zeigen», Radiogruppe «Trakt-Liste» (sieben Arten vom Server), Feld «Adresse der Liste»
  (nur Art «Öffentliche Liste per Adresse»), Radiogruppe «Medienart», vier Zahlen («Höchstens»,
  «Ausblenden unter», «Abruf alle … Stunden», «Spotlight Wechsel Takt»), Fuss mit Abrufzustand
  (`spotlight/FetchState.tsx`), Knöpfe «Jetzt abrufen» und «Speichern».
- Band «Einträge der Liste» (einklappbar, `:284-305`): Tabelle «Rang», «Titel», «Status»,
  «Eigene Notiz» (mit «Notiz»), Aktionen «Anheften»/«Lösen», «Ausblenden»/«Einblenden»
  (`spotlight/columns.tsx:50-168`).
- Band «Verwaiste Anpassungen» nur bei Bedarf (`:307-322`), je Zeile «Entfernen».
- Satz mit Link «Zur Seite «Startseite»» (`:324-327`).
- Dialoge: «{Titel} anheften» (Platzraster), «Notiz zu {Titel}» (`spotlight/EntryDialogs.tsx`),
  Rückfrage beim Verlassen (`:357`).

**Suchparameter, Deeplinks, Verweise**

- Keine Suchparameter (Route ohne `validateSearch`, `app/router.tsx:514-519`).
- Ausgehend: «Schlüssel unter Integrationen», «Schlüssel gesetzt»/«Kein Schlüssel» →
  `/admin/integrations`; Satz unten → `/admin/home-rows`.
- Eingehend: Startseite (Klappbereich und Satz). Die Seite Hintergrundjobs zeigt bei
  `trakt.refresh` «Takt und Ein/Aus sind dieselbe Einstellung wie auf der Seite Spotlight.»
  (`features/admin/JobsPage.tsx:1541-1544`).

**Abfragen und Mutationen**

- `getSpotlightConfig`, `getTraktStatus`, `getSpotlightEntries`. Takt: nach «Jetzt abrufen» alle
  2 s für 30 s Zustand und Einträge (`:41-43,107-125`); angehalten bei offenem Dialog, offener
  Rückfrage oder ungespeicherter Quelle; Fensterfokus ebenso angehalten.
- `setSpotlightConfig` (ein Aufruf für Schalter und Quelle), danach Zustand, Einträge, `['home']`.
- `refreshSpotlight` (202, 422 `provider_not_configured`, 429 innerhalb einer Minute).
- `setSpotlightEntryOverride` für Anheften (`pinnedRank`), Lösen (`null`), Aus-/Einblenden
  (`hidden`), Notiz (`note`) und Entfernen einer verwaisten Anpassung (`{}`); Antwort ersetzt den
  Eintrag im Cache, danach Einträge und `['home']` neu. Keine Rückfrage bei Lösen, Ausblenden und
  Entfernen der verwaisten Anpassung (Entscheid).

**Fallen für den Umbau**

- Quelle ist ein Entwurf (`useDrafts`, `SOURCE_DRAFT`); Vergleich ohne Rand-Leerzeichen;
  Getipptes während des Speicherns bleibt Entwurf; Zuklappen des Bands behält ihn.
- Medienart: die erlaubten Medien je Liste kommen vom Server (`listKinds`), nicht aus dem Client.
- Plätze im Anheft-Dialog richten sich nach dem gespeicherten «Höchstens», nicht nach dem Entwurf.
- Fokus: Knopf «Anheften»/«Lösen» behält den Fokus und wechselt nur den Namen; nach einem Dialog
  zurück zum Knopf oder h1; nach Entfernen einer verwaisten Anpassung immer h1.
- Ein/Aus und Takt gibt es an zwei weiteren Orten (Startseite «Ausblenden», Hintergrundjobs
  `trakt.refresh`); beim Umbau nicht verdreifachen und nicht still entfernen.

**Abgleich mit dem Mockup**

| Mockup | Einstufung | Begründung |
|---|---|---|
| D039 Spotlight-Quelle | offene Frage | «Anzahl angezeigter Titel» = «Höchstens» besteht; «Quelle: Automatisch/Manuell» und «Vorschau» gibt es nicht (Quelle ist immer eine Trakt-Liste, sieben Arten). Manuelle Quelle wäre eine neue Funktion. |
| D040 Titel anheften | visuelle Umgruppierung | Anheften, Notiz und Lösen bestehen je Eintrag der Liste; das Feld «Titel suchen» hat keine Basis (angeheftet werden nur Einträge der Trakt-Liste). |
| D041 Verwaiste Einträge | visuelle Umgruppierung | Band «Verwaiste Anpassungen» mit «Entfernen» je Zeile besteht; Filter «Alle/Verwaist» und Mehrfachauswahl «Auswahl entfernen» gibt es nicht (Beispiel ohne Basis). |

**Im Mockup fehlt (bleibt)**: Schalter «Auf der Startseite zeigen»; Auswahl der Trakt-Liste mit
sieben Arten und Adresse; «Medienart»; «Ausblenden unter», «Abruf alle … Stunden», «Spotlight
Wechsel Takt»; «Jetzt abrufen» mit 2-s-Takt und Live-Anzeige; Abrufzustand mit Treffern und
Schlüssel-Link; Hinweis, warum die Reihe fehlt (fünf Gründe); Chip im Bandkopf; «Ausblenden» /
«Einblenden» je Eintrag; Spalten Rang/Status; Anzeige «Nicht in der Bibliothek»; Rückfrage beim
Verlassen; Link zur Startseite.

---

## 3. User-Empfehlungen `/admin/picks` (PCK) → Startseite → Cat’s Empfehlungen

**Aufbau heute**

- Kopf: h1 «User-Empfehlungen», Hilfe (`picks/PicksPage.tsx:126`). Keine Hauptaktion.
- Band «Ablauf» mit Formular «Empfehlungen bleiben … Tage in der Liste» und «Speichern»
  (`:130-154,223-275`).
- Band «Aktuelle Empfehlungen» («{n} Titel»): Tabelle «Titel», «Zuletzt empfohlen am»,
  «Läuft ab», «Anzahl», «Entfernen» (`:156-191`, `picks/columns.tsx`); Satz zur Anonymität.
- Info-Zeile mit Link «Startseite» (`:193-202`); Rückfrage zum Entfernen (`:203-214`).

**Suchparameter, Deeplinks, Verweise**

- Route nimmt `adminSearch` an (`app/router.tsx:480-485`), die Seite liest aber weder `tab` noch
  `item`. Ausgehend → `/admin/home-rows`. Die Empfehlungen selbst entstehen auf der Titelseite (Stern).

**Abfragen und Mutationen**

- `getPicksSettings`, `listAdminUserPicks` (ohne Takt; Fensterfokus angehalten, solange die
  Rückfrage offen ist, `:71`).
- `updatePicksSettings` und `removeAdminUserPick`, danach `['admin','picks']`, `['home']`,
  `['picks']`, `['my-media']`, `['item']` neu (`:76-83`).

**Fallen für den Umbau**

- Name: Mockup «Cat’s Empfehlungen», Code und Startseite «User-Empfehlungen» (auch Meine Medien,
  Seitenleiste, Startseitenreihe, Rückfragetext «… aus den User-Empfehlungen entfernen?»). Ein
  neuer Name ist eine Textänderung in allen Sprachen, nicht nur hier.
- Keine Profilnamen (ADR 0028); Datum als Kalendertag in UTC gelesen (`picks/columns.tsx:12-16`).
- Fokus nach Entfernen: zurück zum Knopf, sonst h1; die Fokusarbeit bricht beim Verlassen ab.
- Kein Zugriffstest für diese Seite in den Vitest-Tests.

**Abgleich mit dem Mockup**

| Mockup | Einstufung | Begründung |
|---|---|---|
| D042 Aufbewahrungsdauer | bestehende Funktion | Zahlenfeld 7–365 Tage mit «Speichern» besteht (ab Werk 60, der Wert 90 ist Beispiel). |
| D043 Benutzerempfehlungen | visuelle Umgruppierung | Liste und «Entfernen» (mit Rückfrage) bestehen; Filter «Alle/Neueste/Älteste» und «Empfehlung ansehen» haben keine Basis (Reihenfolge vom Server, kein Link zur Titelseite). |

**Im Mockup fehlt (bleibt)**: Spalten «Zuletzt empfohlen am», «Läuft ab», «Anzahl»; Satz zur
Anonymität; Leertext; Rückfrage beim Entfernen; Link zur Startseite.

---

## 4. Sammlungen `/admin/collections` (COL) → Startseite → Sammlungen

**Aufbau heute**

- Kopf: h1 «Sammlungen», Hilfe, Hauptknopf «Sammlung anlegen» (`collections/CollectionsPage.tsx:340-354`).
- Band «Sammlungen» mit Reitern «Alle», «Manuell», «TMDB» (je Zahl) und Tabelle «Name», «Quelle»,
  «Titel», Zustand (`:369-394`, `collections/columns.tsx`).
- Klappbereich je Zeile (`collections/CollectionDetail.tsx`):
  - manuell: «Titel suchen» mit Treffern und «Hinzufügen», Liste «Titel der Sammlung» mit
    Platzwahl, Poster und «Entfernen»; unten «Ungespeicherte Änderungen», «Verwerfen», «Speichern»,
    «Umbenennen», «Löschen»;
  - TMDB: Schloss, Hinweis, Mitglieder als Kacheln nur zum Lesen.
- Dialoge: Name (`collections/NameDialog.tsx`, anlegen/umbenennen), Rückfrage Löschen, Rückfrage
  beim Verlassen.

**Suchparameter, Deeplinks, Verweise**

- `?tab=all|manual|tmdb` (Vorgabe `all` ohne Parameter, unbekannt → Alle, Adresse ersetzt).
  Offene Zeilen und Entwürfe stehen nicht in der Adresse.
- Eingehend: Startseite (Klappbereich der Reihe «Sammlungen»). Wirkung: Startseitenreihe
  «Sammlungen» und Sammlungsseiten der Zuschauer.

**Abfragen und Mutationen**

- `listCollections`, `getAdminCollection` (beim ersten Aufklappen), `searchAdminItems` (`limit`
  20, ab 2 Zeichen, 200 ms, `staleTime` 30 s). Kein Takt; Fensterfokus angehalten bei offenem
  Dialog oder Entwurf einer aufgeklappten Zeile.
- `createCollection`, `updateCollection`, `deleteCollection` (Rückfrage), `setCollectionItems`
  (eigener «Speichern» je Sammlung), danach Sammlungen, `['home']`, `['collection']`, `['item']`,
  `['search']` neu (`:195-203`).

**Fallen für den Umbau**

- Ein Entwurf je Sammlung (Reihenfolge der IDs); bleibt beim Zuklappen und Reiterwechsel; ein
  zugeklappter Entwurf pausiert die Abfragen nicht, hält aber die Rückfrage beim Verlassen.
- Die Suche erscheint erst nach dem Laden der gespeicherten Mitglieder, sonst würde «Speichern»
  Mitglieder verlieren.
- «Verwerfen» hier **ohne** Rückfrage, auf Startseite und Meine Medien **mit** Rückfrage.
- Fokus: nach Hinzufügen/Entfernen ins Suchfeld, nach Verwerfen auf «Speichern», nach Anlegen
  zurück auf «Sammlung anlegen», nach Löschen auf h1.

**Abgleich mit dem Mockup**

| Mockup | Einstufung | Begründung |
|---|---|---|
| D044 Sammlungen durchsuchen | visuelle Umgruppierung | «Typ» = Reiter Alle/Manuell/TMDB, «Öffnen» = Aufklappen bestehen; Freitext «Suchen» über die Sammlungen und «Aktualisieren» haben keine Basis. |
| D045 Manuelle Sammlung bearbeiten | bestehende Funktion | Anlegen, Umbenennen, Titel suchen/hinzufügen/entfernen und Reihenfolge bestehen. |
| D046 Sammlung löschen | visuelle Umgruppierung | Löschen mit Rückfrage besteht, aber in der Zeile, nicht über eine Auswahlliste. |

**Im Mockup fehlt (bleibt)**: Spalten Quelle/Zahl/Zustand («nicht änderbar», «geändert»);
Hinweis und Lesemodus für TMDB-Sammlungen; deutscher Name der TMDB-Sammlung; «Speichern» und
«Verwerfen» je Sammlung; Entwurf über Reiter und Zuklappen; Rückfrage beim Verlassen; Leertexte.

---

## 5. Meine Medien `/admin/my-media` (MYM) → Startseite → Meine Medien

**Aufbau heute**

- Kopf: h1 «Meine Medien», Hilfe (`MyMediaPage.tsx:272`).
- Band «Meine Medien» mit Reitern «Reihenfolge» und «Vorschau» (`:287-296`): Tabelle «Platz»,
  «Eintrag» (mit Chip der Art), «Inhalt», «Zustand», «Seitenleiste», «Startseite» (je
  Kontrollkästchen «Ausblenden») (`:182-240`); Vorschau beider Orte (`mymedia/MyMediaPreview.tsx`).
- `SaveBar`, Fehler, Statusregion, Satz mit Link zur Startseite (`:299-310`).
- Dialoge: «Änderungen verwerfen?», Rückfrage beim Verlassen.

**Suchparameter, Deeplinks, Verweise**

- `?tab=preview` (Vorgabe `order`, Route `app/router.tsx:505-511`). Ausgehend → `/admin/home-rows`;
  eingehend: Startseite (Klappbereich der Reihe «Meine Medien»).

**Abfragen und Mutationen**

- `getMyMedia` ohne Takt, Fensterfokus/Wiederverbindung nur ohne Entwurf.
- `setMyMedia` (alle Einträge mit `hideSidebar`, `hideHome`, `position`), danach `['home']` und
  `['my-media']` neu (Seitenleiste und Startseite).

**Fallen für den Umbau**

- Eine Reihenfolge für zwei Orte, Ausblenden je Ort getrennt (O-21); vier Zustandswörter.
- Keine Obergrenze wie bei der Startseite.
- Teilt das CSS-Modul der Startseite (`homerows/HomeRows.module.css`).
- `/admin/my-media` fehlt in `e2e/admin-phone.spec.ts`: keine Prüfung der Handybreite.
- Neue Bibliotheken kommen laut Kommentar am Ende sichtbar dazu (Server, `MyMediaPage.tsx:68-69`).

**Abgleich mit dem Mockup**

| Mockup | Einstufung | Begründung |
|---|---|---|
| D047 Sortierung & Sichtbarkeit | visuelle Umgruppierung | Reihenfolge, «In der Seitenleiste anzeigen», «Auf der Startseite anzeigen», Speichern und Vorschau bestehen; heute als Tabelle mit Entwurf und umgekehrter Logik («Ausblenden»). |

**Im Mockup fehlt (bleibt)**: Spalte «Inhalt» mit Zahlen; Zustandsspalte; «Verwerfen» mit
Rückfrage; Rückfrage beim Verlassen; Link zur Startseite; Hinweis, dass Sichtbarkeit je Profil
über die Konten geregelt ist.

---

## 6. Ankündigungen `/admin/announcements` (ANN) → Startseite → Mitteilungen

**Aufbau heute**

- Kopf: h1 «Ankündigungen», Hilfe, Hauptknopf «Neue Ankündigung» (`AnnouncementsPage.tsx:483-491`).
- `MasterDetail` (`:509-517`): links Liste «Ankündigungen» in Abschnitten «Entwürfe {n}» und
  «Veröffentlicht {n}»; rechts je nach Auswahl
  - neue: Band «Neue Ankündigung», Formular («Titel», «Text», «Sofort veröffentlichen»), Knöpfe
    «Als Entwurf speichern»/«Veröffentlichen» und «Abbrechen», Vorschau;
  - Entwurf: Band mit Chip «Entwurf», Fakten, Formular «Entwurf bearbeiten», Satz zum
    Veröffentlichen, «Als Entwurf speichern», «Verwerfen» (nur bei Änderung), «Veröffentlichen»,
    «Löschen», Vorschau;
  - veröffentlicht: Fakten mit Lesebalken, «Löschen», Vorschau (`announcements/AnnouncementDetail.tsx`).
- Ab 768 px Liste und Detail nebeneinander, darunter untereinander (`ui/MasterDetail.module.css:90`).
- Eine Rückfrage für die Seite (Veröffentlichen, Löschen, Verwerfen) und die Rückfrage beim Verlassen.

**Suchparameter, Deeplinks, Verweise**

- `?item=new` oder `?item=announcement-<id>`; ohne Angabe der erste gespeicherte; unbekannt →
  erster, Adresse bereinigt (replace) (`:166-186`). Keine Links zu anderen Admin-Seiten.
- Wirkung: Profildialog beim nächsten Start, Kanäle der Konten (Benachrichtigungen).

**Abfragen und Mutationen**

- `listAnnouncements` ohne Takt, Fensterfokus angehalten bei Rückfrage oder Eingabe.
- `createAnnouncement` (`publish` true/false), `updateAnnouncement`, `publishAnnouncement`
  (geänderter Text wird vorher mit `updateAnnouncement` gespeichert), `deleteAnnouncement`; nach
  jeder Mutation `listAnnouncements` neu. Rückfrage bei Veröffentlichen eines Entwurfs und Löschen,
  **keine** bei «Sofort veröffentlichen» einer neuen.

**Fallen für den Umbau**

- Name: Mockup «Mitteilungen», Code und Zuschauerseite «Ankündigungen».
- Drafts je Ankündigung (`useDrafts`), die neue als eigener Zustand; Getipptes während des
  Speicherns geht auf den gespeicherten Entwurf über (`:207-233`).
- 409 `announcement_published` (zweiter Administrator): Fehler oben, Änderung weg.
- «Veröffentlichen» prüft zuerst die Felder (`reportValidity`), Fokus aufs erste ungültige.
- Fokus nach Speichern der neuen auf «Als Entwurf speichern» des Entwurfs, nach Veröffentlichen und
  Löschen auf h1, nach Verwerfen ins Titelfeld.
- Die Vorschau nutzt `features/notifications/AnnouncementContent` der Zuschauerseite.

**Abgleich mit dem Mockup**

| Mockup | Einstufung | Begründung |
|---|---|---|
| D048 Mitteilung verfassen | bestehende Funktion | Titel, Text, Entwurf speichern und Veröffentlichen bestehen; der Schalter «Entwurf speichern» entspricht umgekehrt «Sofort veröffentlichen». |
| D049 Veröffentlichte Mitteilungen | offene Frage | Lesestatus und Löschen bestehen, der Filter ist heute eine Gliederung in zwei Abschnitte; «Bearbeiten» einer veröffentlichten widerspricht dem Bestand (danach nicht mehr änderbar). |

**Im Mockup fehlt (bleibt)**: Vorschau «So sehen es die Profile»; Fakten Erstellt/Veröffentlicht;
Lesebalken «{x} von {y} Profilen»; Zähler «{n} / 120» und Grenze 4000; «Verwerfen» eines
geänderten Entwurfs und «Abbrechen» der neuen mit Rückfrage; Rückfrage vor dem Veröffentlichen
eines Entwurfs; Hinweis zu Kanälen und Gästen; Auswahl in `?item=`; Rückfrage beim Verlassen;
Fall «inzwischen veröffentlicht».

---

## 7. Statistiken `/admin/stats` (STA) → System → Statistiken

**Aufbau heute**

- Kopf: h1 «Statistiken», Hilfe, Radiogruppe «Zeitraum» (7/30/90/365 Tage) als Hauptaktion
  (`StatsPage.tsx:91-109`).
- Inhalt (`:123-188`): Zustandsleiste «Wiedergabe» (vier Kacheln), Band «Meistgesehen» (Podium 1–3,
  Tabelle ab Platz 4), nebeneinander «Schauzeit je Profil» und «Geräte» (je Diagramm und Tabelle),
  Band «Wann geschaut wird» (zwei Diagramme, Zeitzone, Raster 7 × 24).
- Versteckte Statusmeldung «Zahlen für {Zeitraum} geladen.» (`:189-191`).

**Suchparameter, Deeplinks, Verweise**

- `?days=7|30|90|365`, Vorgabe 30, anderes gilt als keins (`app/router.tsx:569-582`,
  `statsDays.ts`); die Wahl ersetzt die Adresse.
- Ausgehend: jeder Titel → `/admin/items/{id}` (Pflege). Gerätetypen aus `admin.devices.types`.

**Abfragen und Mutationen**

- Nur `getStats` (`days`); kein Takt, kein Aktualisieren-Knopf; alte Zahlen bleiben beim Wechsel
  sichtbar (`keepPreviousData`, `aria-busy`, `data-switching`).

**Fallen für den Umbau**

- Fenster von «Wann geschaut wird» in ganzen Wochen (1, 4 oder 12, auch bei 365 Tagen 12).
- Diagramme tragen Tabellen für Screenreader; das Raster hat keinen Fokushalt.
- Profiltabelle zeigt die E-Mail-Adresse des Kontos.

**Abgleich mit dem Mockup**

| Mockup | Einstufung | Begründung |
|---|---|---|
| D062 Wiedergabestatistik | offene Frage | Zeitraum 7/30/90/365 besteht; «Wiedergabe» als Filter nach Modus und «Aktualisieren» haben keine Basis (heute zeigt die Leiste alle vier Werte, kein Takt). |
| D063 Auswertungen | visuelle Umgruppierung | Alle vier Auswertungen bestehen, heute alle zugleich auf einer Seite statt über eine Auswahl «Diagramm»; «Auswertung öffnen» entspricht nur dem Link eines Titels. |

**Im Mockup fehlt (bleibt)**: Podium mit Bildern und Link zur Titelseite; Tabellen zu jedem
Diagramm; Zeitzone des Servers; Raster «Woche und Stunde» mit Spitze; «Am meisten …»-Sätze;
Leertexte; Ansage nach dem Zeitraumwechsel; Zeitraum in der Adresse.

---

## 8. Sicherung `/admin/backup` (BKP) → System → Sicherungen

**Aufbau heute**

- Kopf: h1 «Sicherung», Hilfe (`BackupPage.tsx:354`); Meldung/Fehler oben.
- Band (Titel verborgen) mit Reitern (`:372-386`):
  - «Sicherungen» (Zahl): Klappgruppen «Manuell» und «Automatisch» (`ui` `DisclosureGroup`), je
    Tabelle «Datei», «Erstellt», «Grösse», «Schlüssel», «Verschlüsselt», «Herunterladen», Menü
    «Weitere Aktionen» mit «Wiederherstellen …» und «Löschen …» unter «Gefahr» (`backup/columns.tsx`).
  - «Neue Sicherung»: Umfang, Passphrasen, «Sicherung erstellen» (`backup/CreateTab.tsx`).
  - «Aus Datei wiederherstellen»: Warnung, Datei, Passphrase, «Wiederherstellen» (`backup/UploadTab.tsx`).
- Neustart-Tafel ersetzt das Band nach einer angenommenen Wiederherstellung (`:357-371`, `RestartPanel.tsx`).
- Dialoge: eine Rückfrage für die Seite (Wiederherstellen, Löschen), Passphrase-Dialog
  (`backup/PassphraseDialog.tsx`).

**Suchparameter, Deeplinks, Verweise**

- `?tab=list|create|upload` (Vorgabe `list`, unbekannt → list, Adresse ersetzt).
- Neustart-Tafel ohne Schlüssel verweist im Text auf Integrationen (kein Link). Nach dem Neustart
  volle Ladung von `/login` (`restart.ts:22-24`). Übersicht zeigt «Sicherung erstellt/wiederhergestellt»
  in der Aktivität (`AdminHomePage.tsx:104-150`).

**Abfragen und Mutationen**

- `listBackups` (ohne Takt; Fensterfokus angehalten bei Dialog; aus nach Wiederherstellung).
- `createBackup` (`includeSecrets`, `passphrase`), `downloadBackup` (Link), `deleteBackup`
  (Rückfrage), `restoreBackup` (Rückfrage, verschlüsselt vorher Passphrase-Dialog),
  `restoreBackupUpload` (multipart, Passphrase vor der Datei, Rückfrage). Danach `getHealth` alle
  2 s bis ein neuer Prozess antwortet; nach 90 s Abbruch mit «Erneut prüfen».

**Fallen für den Umbau**

- Passphrasen leben nur im Reiter oder Dialog und nie im Mutation-Cache (`useTransientMutation`);
  ein Reiterwechsel löscht sie, die gewählte Datei und der Umfang bleiben (Seitenzustand).
- Rückfrage «Alle Daten ersetzen?» mit Fokus auf «Abbrechen» (Schutz vor doppeltem Enter).
- Während einer Wiederherstellung sind Reiter und Menüs gesperrt.
- Kein Entwurf und keine Rückfrage beim Verlassen auf dieser Seite.
- `RestartPanel` wird auch von der Übersicht (Neustart) genutzt; beim Umbau gemeinsam halten.

**Abgleich mit dem Mockup**

| Mockup | Einstufung | Begründung |
|---|---|---|
| D064 Vorhandene Sicherungen | visuelle Umgruppierung | Herunterladen, Wiederherstellen und Löschen bestehen; «Art» ist heute die Gliederung in zwei Klappgruppen, kein Filter. |
| D065 Neue Sicherung | offene Frage | Umfang und Passphrase bestehen; ein eigener Schalter «Mit Passwort verschlüsseln» widerspricht dem Bestand: mit Schlüsseln ist immer verschlüsselt, ohne Schlüssel nie. |
| D066 Aus Datei wiederherstellen | visuelle Umgruppierung | Datei, Passphrase, Wiederherstellen mit Neustart bestehen; «Import prüfen» als eigener Schritt hat keine Basis (der Server prüft beim Hochladen, Knopf zeigt «Wird geprüft …»). |

**Im Mockup fehlt (bleibt)**: Spalten Schlüssel/Verschlüsselt/Grösse/Erstellt; Chip «neu»;
Kopfzeilen der Gruppen (verschlüsselt, neueste); Hinweis zu automatischen Sicherungen;
Passphrase-Dialog für verschlüsselte Sicherungen der Liste; Wiederholung der Passphrase und
Mindestlänge 12; Rückfrage «Alle Daten ersetzen?»; Neustart-Tafel mit 90-s-Grenze und «Erneut
prüfen»; Warnung ohne Zugangsdaten; Sperren während der Wiederherstellung.

---

## 9. Gemeinsame Teile

| Teil | Wo genutzt | Verhalten | Datei |
|---|---|---|---|
| `TitleSearch` | Sammlungen | Suchfeld, Treffer mit Knopf, Enter sendet nicht, ab 2 Zeichen, 200 ms | `features/admin/CurationParts.tsx:51-121` |
| `titleMeta` | User-Empfehlungen, Suche | «Film · 2024 · Serie X» | `CurationParts.tsx:21-26` |
| `toLocalInput` / `fromLocalInput` | nirgends mehr (grep in `web/src`) | Rest einer früheren Fassung | `CurationParts.tsx:123-140` |
| `PosterTitle` | Spotlight, User-Empfehlungen | Poster 40×60, Ersatzbild bei Fehler | `features/admin/PosterTitle.tsx` |
| `RestartPanel`, `restart.ts` | Sicherung, Übersicht | `/healthz` alle 2 s, 90 s Grenze | `features/admin/RestartPanel.tsx` |
| `HomeRows.module.css` | Startseite, Meine Medien | Tabelle, Kästchen, Vorschau | `features/admin/homerows/HomeRows.module.css` |

## 10. Unklar

1. **Doppelte Schalter Spotlight:** «Ausblenden» der Reihe auf der Startseite und «Auf der
   Startseite zeigen» auf Spotlight, dazu Ein/Aus und Takt bei `trakt.refresh` auf
   Hintergrundjobs. Welcher Schalter bei Widerspruch gilt, entscheidet der Server; im Client nicht
   belegt.
2. **Sofort veröffentlichen ohne Rückfrage** (ANN): durch Tests gewollt, Entscheid nicht gefunden.
3. **Neue Bibliothek in «Meine Medien»** am Ende sichtbar: nur im Code-Kommentar belegt
   (`MyMediaPage.tsx:68-69`), Verhalten liegt im Server.
4. **Handybreite von `/admin/my-media`:** nicht in `e2e/admin-phone.spec.ts`; ob die Seite
   breiter als der Bildschirm wird, ist ungeprüft.
5. **Zugriffsschutz `/admin/picks`:** kein Test für Gäste/Nebenprofile (Code vorhanden).
6. **Namen der Seiten** (Cat’s Empfehlungen / Mitteilungen / Sicherungen im Mockup gegenüber
   User-Empfehlungen / Ankündigungen / Sicherung im Code): ob umbenannt werden soll, ist offen.
