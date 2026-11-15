# Funktionsprüfung `startseite-system` (HOM, SPT, PCK, COL, MYM, ANN, STA, BKP)

Geprüfter Stand: `850a31b6` gegen `55b72c9a`. Zeilen: 170, Ergebnis in `startseite-system.csv`.
Quelle ist der Code; Berichte der Executor sind nicht eingeflossen.

## Vorgehen

- Alle 170 Zeilen gegen den heutigen Code; für die 77 Zeilen «prüfen» einzeln am Diff.
- Jeder in «Tests heute» genannte Testname wurde im heutigen Stand gesucht (298 Nennungen, mit
  verschachtelten «…» richtig zerlegt). Es fehlen nur die neun Nennungen der Startseite, deren Tests
  umbenannt sind (siehe HOM-002, HOM-005, HOM-012, HOM-014, HOM-017, HOM-020). Die übrigen Meldungen
  «Tests» der Vorprüfung (COL, MYM, BKP-019) kommen daher, dass Testnamen mit «…» falsch zerlegt
  wurden; die Tests stehen da. Die Zeilenangaben in der Spalte «Beleg» sind auf den heutigen Stand
  nachgeführt.
- Seit `55b72c9a` **unverändert** sind: `SpotlightPage.tsx`, `spotlight/`, `picks/`,
  `collections/*.tsx` (ausser Test), `MyMediaPage.tsx`, `mymedia/`, `AnnouncementsPage.tsx`,
  `announcements/`, `StatsPage.tsx`, `stats/`, `backup/`, `CurationParts.tsx`, `RestartPanel.tsx`,
  `restart.ts`, `PicksPage.test.tsx`, `BackupPage.test.tsx`, `AnnouncementsPage.test.tsx`,
  `e2e/admin-picks.spec.ts`, `e2e/admin-stats.spec.ts`. Geändert sind `HomeRowsPage.tsx` und
  `homerows/HomeRows.module.css` (Reiter durch zwei Karten ersetzt), `BackupPage.tsx` (nur der
  Kopf), dazu der Rahmen `ui/AdminPage.tsx` und `ui/DataTable.module.css`.
- Stichprobe der «unverändert»-Zeilen: SPT-005, SPT-021, PCK-008, COL-018, MYM-017, ANN-010,
  STA-007, STA-012, BKP-008, BKP-019 an der genannten Stelle nachgesehen; alle stimmen.
- Selbst laufen lassen (Zweifel an der umgebauten Startseite und am neuen Kopf der Sicherung):
  `pnpm exec vitest run src/features/admin/HomeRowsPage.test.tsx src/features/admin/BackupPage.test.tsx`
  → 2 Dateien, 35 Tests bestanden.

## LÜCKE, LÜCKE Test, unklar

| ID | Status | Begründung |
|---|---|---|
| HOM-021 | LÜCKE Test | Die Mindestbreite der Tabelle «Reihen der Startseite» ist umgebaut: vorher immer 48rem ab 768 px (`55b72c9a` `ui/DataTable.module.css:36`), jetzt 36rem, sobald Editor und Vorschau nebeneinander stehen (Behälter ≥ 58rem, `homerows/HomeRows.module.css:176-188`, `--sc-table-fixed-min-width`). Spaltenbreiten 20 %/36 %/18 % sind gleich. K12-9 (`e2e/admin-home-rows.spec.ts:255`) und der Handy-Test stehen noch, aber kein Test prüft die neue Grenze oder das Stapeln unter 58rem. Zu tun: Test für Mindestbreite neben der Vorschau und für die gestapelte Lage. |

Keine Zeile mit `LÜCKE` oder `unklar`.

## Startseite: Reiter durch zwei Karten ersetzt

- Vorher (`55b72c9a` `HomeRowsPage.tsx:243-307`): ein Band «Reihen» mit den Reitern «Reihenfolge»
  («{n} Reihen») und «Vorschau» («{n} sichtbar»), Auswahl in `?tab=`.
- Jetzt (`HomeRowsPage.tsx:276-311`): Karte «Reihen» mit dem Zusatz «{n} Reihen» und Karte
  «Vorschau» mit dem Zusatz «{n} sichtbar». Ab 58rem stehen sie nebeneinander, darunter
  untereinander. Die Vorschau bleibt beim Scrollen stehen (sticky). Beide Zahlen bleiben erhalten.
- `?tab=preview` gilt weiter: Die Seite scrollt einmal zur Vorschau, sobald die Reihen da sind
  (`:107-117`). Geprüft wird das in `HomeRowsPage.test.tsx:308`. `?tab=order` gilt weiter,
  unbekanntes `?tab=` wird ersetzt (replace, gleicher Test).
- Der Entwurf, die offenen Zeilen, die Speicherleiste (direktes Kind der Seitenspalte), «Verwerfen»
  mit Rückfrage, die Rückfrage beim Verlassen, die 15er-Grenze und der Spotlight-Satz sind im Code
  unverändert. Ihre Tests bestehen.
- Gegenstandslos: «←/→ wechselt die Reiter» (HOM-020) und «Entwurf und offene Zeilen bleiben über
  den Reiterwechsel» (HOM-005, HOM-011). Beides ist im Bauplan 5 gedeckt, weil beide Hälften
  gleichzeitig sichtbar sind.

## Sicherung: neue Hauptaktion «Neue Sicherung»

`BackupPage.tsx:357-374`: Ein Knopf im Kopf ruft dasselbe `start()` und `selectTab('create')` auf
wie der Reiterwechsel. Er ist bei `restoring` gesperrt, wie die Reiter. Auf der Neustart-Tafel
fehlt er (`!restart`). Reiter, Klappgruppen, Zeilenmenüs, Rückfragen, Passphrase-Dialog, Sperren
während Wiederherstellung und Löschen sowie die Neustart-Tafel sind im Code unverändert; nur die
Zeilennummern ab :354 sind um 21 verschoben. `BackupPage.test.tsx` ist unverändert und besteht.

## Funde ohne Inventarzeile

1. **Neue Hauptaktion der Sicherung ohne Test.** Weder Vitest noch e2e prüfen den Knopf
   «Neue Sicherung» im Kopf, also weder das Öffnen von `?tab=create`, noch die Sperre während
   einer Wiederherstellung (`BackupPage.test.tsx:578` prüft nur Reiter und Menüs), noch das
   Fehlen auf der Neustart-Tafel. Die Funktion ist neu, also kein Verlust; aber sie hat keinen Beleg.
   Ausserdem tragen jetzt Knopf und Reiter denselben Namen «Neue Sicherung». Die Tests suchen nach
   der Rolle `tab` und bleiben eindeutig; für Screenreader sind es zwei Bedienelemente gleichen
   Namens mit unterschiedlicher Rolle.
2. **Verwaiste Texte.** `admin.homeRows.tabs.label` («Ansicht der Reihen») und
   `admin.homeRows.tabs.order` («Reihenfolge») nutzt kein Code mehr (`tabs.preview` dient jetzt als
   Kartentitel). Das ist kein Verlust; die Texte können in Schritt 5 aufgeräumt werden.
3. **Test-Änderungen geprüft, nichts ausgehöhlt.** Gestrichene Erwartungen betreffen nur die
   entfallenen Reiter der Startseite, den Hilfe-Knopf (jetzt «Einleitung sichtbar, kein Knopf»)
   und die Seitenleisten-Einträge der Seiten (jetzt Bereichsreiter bzw. Bereichseintrag, Bauplan
   2.5). Es gibt kein `.skip` oder `.only` in den Dateien dieses Teils und keine längeren Timeouts.
   Die Grenze in `navTo` (`e2e/admin-spotlight.spec.ts`) ist von 30 auf 10 Pfeiltasten enger
   geworden. In `e2e/admin-density.spec.ts` ist die h1-Höhe von ≤ 32 auf ≤ 34 px gelockert, weil
   der Titel im Rahmen 30 px statt 24 px hat (Bauplan 2.6). Das ist begründet.
4. Keine entfernten `aria-`-Angaben, Ansagen oder Texte in `de.json` für diese Seiten.
   `Live, alle n Sekunden` (Spotlight) steht im Rahmen neben dem Titel; die Radiogruppe «Zeitraum»
   (Statistiken) und die Hauptknöpfe «Sammlung anlegen» und «Neue Ankündigung» stehen als
   `actions` im Kopf.
5. Der Handy-Test `e2e/admin-phone.spec.ts` deckt neu auch `/admin/my-media` ab. Das ist ein
   Gewinn für MYM-020, das bisher keinen Test hatte; die Zeile ist trotzdem als `ok, ohne Test`
   aus dem Inventar übernommen.

## Hilfetext (Zeilen …-002)

Für HOM, SPT, PCK, COL, MYM, ANN, STA und BKP gilt Bauplan 2.6: Der Hilfe-Knopf entfällt, die
Einleitung steht offen unter dem Titel (`ui/AdminPage.tsx:143`). Seitentests belegen das für HOM,
SPT, COL und STA. Für PCK, MYM, ANN und BKP belegt es nur der Bauteiltest
`ui/AdminPage.test.tsx:69`; diese Seiten hatten schon vor dem Umbau keinen Test des Hilfetexts.

## Zählung

| Kürzel | Zeilen | ok | ok, ohne Test | LÜCKE | LÜCKE Test | unklar |
|---|---|---|---|---|---|---|
| HOM | 21 | 18 | 2 | 0 | 1 | 0 |
| SPT | 33 | 32 | 1 | 0 | 0 | 0 |
| PCK | 13 | 7 | 6 | 0 | 0 | 0 |
| COL | 23 | 23 | 0 | 0 | 0 | 0 |
| MYM | 20 | 16 | 4 | 0 | 0 | 0 |
| ANN | 24 | 24 | 0 | 0 | 0 | 0 |
| STA | 14 | 13 | 1 | 0 | 0 | 0 |
| BKP | 22 | 21 | 1 | 0 | 0 | 0 |
| **Summe** | **170** | **154** | **15** | **0** | **1** | **0** |

Alle `ok, ohne Test` hatten schon vor dem Umbau keinen Test.
