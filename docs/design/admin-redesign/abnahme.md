# Abnahme-Checkliste des Übergabepakets, ausgefüllt

Vorlage: `handoff/docs/05_ABNAHME_CHECKLISTE.md`. Stand 16. November 2026, Zweig
`claude/admin-redesign`. `[x]` nachweislich geprüft, `[ ]` offen. Belege in Klammern.

## Bestands- und Funktionsvollständigkeit

- [x] Alle 19 Hauptabschnitte der bisherigen `sections.ts` sind im neuen Design vorhanden und direkt
  erreichbar (Bereichsreiter, Suchpalette, Handy-Blatt; `e2e/admin-shell.spec.ts` «the area tabs
  reach every page», «the sheet reaches every page»; dazu neu `audiobookSources` und `server`:
  21 Seiten).
- [x] Alle Detailrouten bleiben erreichbar: `/admin/podcasts/:id`, `/admin/metadata/audiobooks`,
  `/admin/items/:id` (`e2e/admin-shell.spec.ts` Deeplinks, `e2e/admin-phone.spec.ts` `title`,
  `podcast`).
- [x] Alle bestehenden Formulare mit Vorgabewerten, Prüfung, Pflichtfeldern, Fehlermeldungen,
  Speichern und Verwerfen sind erfasst (`functional-inventory.csv`, Typen `Feld`, `Schalter`,
  `Auswahl`, `Schutz`) und geprüft (`funktionsabdeckung.csv`).
- [x] Jedes Bedienelement und jede Mutation des Bestands steht in der Matrix mit Quellenzeile und
  Testbeleg (`funktionsabdeckung.csv`, 876 Zeilen). 117 Zeilen haben keinen Test, wie schon vor dem
  Umbau; sie sind am Code nachgewiesen.
- [x] Keine fehlende Bestandsfunktion (`abdeckung/nachpruefung.md`: 0 `LÜCKE`). Beispiele des Mockups
  ohne Basis wurden nicht gebaut (`abweichungen.md`, `inventory/*.md` «Abgleich mit dem Mockup»).
- [x] Polling, Live-Zustand, Fehlzustände, leere Listen, Bestätigungen, Rücknavigation und
  Suchparameter funktionieren (Matrix-Typen `Polling`, `Leerzustand`, `Fehlerzustand`,
  `Bestaetigung`; e2e der Seiten).

## Design und Bedienung

- [x] Navigation nach dem Sechs-Bereiche-Konzept (`screenshots/02-medien-bibliotheken-desktop.png`).
- [x] Die Übersicht stellt Handlungsbedarf voran und zeigt Kennzahlen nicht doppelt
  (`screenshots/01-uebersicht-desktop.png`).
- [x] Desktop, Tablet und Handy ohne waagrechtes Überlaufen (`e2e/admin-phone.spec.ts`: 30 Seiten
  bei 390 px; Bilder in 1440, 768 und 390 px). Bedienung per Tastatur: `e2e/admin-shell.spec.ts`,
  `e2e/admin-server.spec.ts` und die Tastaturtests der Seiten. Touch am echten Handy: **offen, durch
  Marco auf CT 190**.
- [x] Farb-Tokens, Profilakzent, Dichte und Bauteile des Bestands (`web/src/ui/admin-look.css` setzt
  nur Tokens; `AdminPage.test.tsx` «sets only tokens that a building block reads»; Kontrasttests
  der acht Akzente in `AdminSideNav.test.tsx` und `BandPanel.test.tsx`).
- [x] Keine Funktion nur für ein ruhigeres Bild versteckt (Prüfauftrag `BRIEF-PRUEFUNG.md`, Status
  `LÜCKE` auch für «nur noch über einen Umweg»; vier solche Befunde behoben).
- [x] Abweichungen vom Mockup dokumentiert und begründet (`abweichungen.md`).

## Sicherheit und Robustheit

- [x] Zugriff nur für Administratoren mit Hauptprofil, Eigentümer-Regeln unverändert (Zeilen vom
  Typ `Berechtigung` in der Matrix; keine Änderung an `server/` und `api/`).
- [x] Schlüssel und Geheimnisse bleiben maskiert (INT-Zeilen der Matrix; A-9-Logscan der e2e-Läufe
  ohne Fund).
- [x] Riskante Aktionen behalten Warnung und Bestätigung (Typ `Bestaetigung`: Löschen, Wiederherstellen,
  Neustart, Stream beenden, Rolle ändern).
- [x] Keine Änderung an Datenmodell, Migrationen oder API (`git diff 55b72c9a..HEAD -- server api`
  ist leer).

## Prüfungen und Übergabe

- [x] `task check:fast` und gezielte Tests je Schritt (Berichte der Schritte in den Commit-Texten
  und im Pull Request).
- [x] Neue und angepasste Bauteile mit Story und Tastaturnachweis (`AreaTabs`, `CommandPalette`,
  `AdminSideNav`, `AdminPage`, `StatTile` mit Verweis, `StatusStrip` mit Überschrift; Storybook
  551 bestanden).
- [x] Alle betroffenen e2e-Specs bestehen, auch Handy und Deeplinks (siehe `task ci`).
- [x] `task ci` vor dem Pull Request bestanden: vierter Lauf auf `39d1039e`, 18 min 05 s, 43
  Go-Pakete, 2160 Vitest-Tests, Storybook 551, e2e 723 bestanden (drei Shards, «all green»). Die
  drei Läufe davor waren rot und sind unten aufgeführt.
- [ ] GitHub CI am Pull Request: **offen** bis zum Lauf; Ergebnis steht im Pull Request.
- [x] Der Pull Request enthält Nachweise, Bilder, Restpunkte und die Matrix.
- [x] Kein Merge und kein produktives Deployment ohne ausdrückliche Freigabe. Auf dem Testserver
  CT 190 läuft der Zweig als eigenes Image; zurück mit `design-alt.sh`.

## Die Läufe von `task ci`

| Lauf | Stand | e2e | Rot | Ursache und Behebung |
|---|---|---|---|---|
| 1 | `a90b1860` | 719 von 723 | 4, alle Firefox | Die gezielten Läufe beim Bau liefen nur in Chromium. Zwei Tests hatten eine Obergrenze von 10 Tab-Schritten vom Seitenanfang zur Hauptaktion; der neue Rahmen hat 11 Stopps (vorher 9: zwei Brotkrumen und die Bereichsreiter dazu, Hilfe-Knopf weg). Der Helfer der Fokusfallen-Prüfung zählte `<summary>` nicht als Tab-Stopp. Ein Spec wartete nicht auf den Löschjob seiner Bibliothek. Behoben in `be5708b4`, `2abfdeea`, `9e702978`, nur Testcode. |
| 2 | `9e702978` | 722 von 723 | `offline-watch-limit` (Firefox) | Offline-Wiedergabe stoppte nach 91,2 s statt höchstens 90 s, unter der Last dreier Shards. Der Umbau berührt den Offline-Player nicht; einzeln in Firefox grün. Nichts geändert. |
| 3 | `9e702978` | 722 von 723 | `admin-my-media` (Firefox) | Eine gelöschte Bibliothek verlässt die Bibliotheksliste sofort, ihr Eintrag in «Meine Medien» erst mit dem Ende des Löschjobs; das Aufräumen von `admin-item-delete` wartete auf das Erste. Behoben in `39d1039e`, nur Testcode. |
| 4 | `39d1039e` | 723 von 723 | keiner | |

Server-Tests, Vitest, Storybook und Build waren in allen vier Läufen grün.

## Abnahme

Marco hat den Stand `95422263` am 16. November 2026 auf CT 190 angesehen und abgenommen («Sieht gut
aus. Machen wir so») und Merge und Release 1.39.0 freigegeben. GitHub CI des Pull Requests 172 war
grün (20 von 20 Prüfungen).

## Was nicht behauptet wird

«100 % funktionsgleich» steht hier nicht. Belegt ist: Jede der 876 aufgenommenen Funktionen ist im
neuen Code vorhanden; 759 davon prüft ein Test. Nicht belegt ist, was nur ein Mensch am Gerät
beurteilt (Wege, Lesbarkeit, Touch), und was schon die Aufnahme übersehen haben könnte.
