# Auftrag Bestandsaufnahme (Phase 0 des Verwaltungs-Redesigns)

Stand: 15. November 2026, Zweig `claude/admin-redesign` auf `main` `55b72c9a` (nach Release 1.38.0).

## Worum es geht

Die ganze Verwaltung (`/admin/...`) bekommt ein neues Erscheinungsbild und eine neue Navigation
(sechs Bereiche: Übersicht, Medien, Nutzer & Geräte, Startseite, Automatisierung, System). Oberste
Bedingung des Auftraggebers: **keine bestehende Funktion, Option oder Einstellmöglichkeit darf
verloren gehen.** Dafür braucht es zuerst eine vollständige, am Code geprüfte Liste von allem, was
die Verwaltung heute kann. Diese Liste ist später die Abnahmegrundlage: jede Zeile bekommt eine neue
Stelle und einen Testbeleg.

Übergabepaket des Auftraggebers (lesen): `docs/design/admin-redesign/handoff/` (README,
`docs/01`–`05`, `data/19_hauptrouten_mapping.csv`, `data/69_mockup_detailansichten.csv`). Das
klickbare Mockup liegt ausserhalb des Repos:
`/home/marco/Downloads/Slouchcat_Admin_Redesign_Claude_Handoff/mockup/Slouchcat_Verwaltung_V2.html`
(nur lesen; die Mockup-Daten sind Beispiele, keine Anforderungen).

## Was du tust

Du nimmst **nur die dir zugeteilten Seiten** auf (siehe dein Auftrag) und schreibst zwei Dateien nach
`docs/design/admin-redesign/inventory/`:

1. `<teil>.csv`: eine Zeile je Funktion, Trennzeichen `;`, UTF-8, Kopfzeile genau so:

   `ID;Bereich neu;Seite;Route und Query;Komponente;Funktion;Typ;Bedingung;API;Zustaende;Tests heute;Mockup;Bemerkung`

   - **ID**: Kürzel der Seite plus laufende Nummer, z. B. `LIB-001` (Kürzel stehen in deinem Auftrag).
   - **Bereich neu**: Zielort nach `handoff/data/19_hauptrouten_mapping.csv`, z. B. `Medien → Bibliotheken`.
   - **Seite**: Schlüssel aus `web/src/features/admin/sections.ts` oder Name der Unterseite.
   - **Route und Query**: z. B. `/admin/libraries?tab=…&item=…`; jeden Suchparameter nennen, den die
     Funktion liest oder setzt (`web/src/app/router.tsx`, `adminSearch`).
   - **Komponente**: `pfad/Datei.tsx:Zeile` der Stelle, an der das Bedienelement entsteht.
   - **Funktion**: was der Administrator dort tun oder sehen kann, in einem Satz, mit dem sichtbaren
     deutschen Text des Bedienelements (aus `web/src/locales/de.json`) in «Guillemets».
   - **Typ**: genau einer von `Aktion`, `Feld`, `Schalter`, `Auswahl`, `Filter`, `Suche`,
     `Sortierung`, `Reiter`, `Aufklappen`, `Dialog`, `Bestaetigung`, `Massenaktion`, `Link`,
     `Anzeige`, `Status`, `Leerzustand`, `Ladezustand`, `Fehlerzustand`, `Polling`, `Tastatur`,
     `Fokus`, `Schutz` (z. B. Abfrage bei ungespeicherten Änderungen), `Berechtigung`.
   - **Bedingung**: wann das Element sichtbar oder aktiv ist (Rolle, Eigentümer, Hauptprofil,
     Bibliothekstyp, Zustand, Bildschirmbreite). Leer, wenn immer.
   - **API**: `operationId` aus `api/openapi.yaml` (oder Methode und Pfad) jeder Abfrage und Mutation
     hinter der Funktion; bei Mutationen auch, welche Abfragen danach neu geladen werden, wenn
     ersichtlich.
   - **Zustaende**: Lade-, Leer-, Fehler-, Erfolgs- und Zwischenzustände, die der Code für diese
     Funktion unterscheidet (auch Ansagen für Screenreader, Fortschritt, Sperren während des Laufs).
   - **Tests heute**: Datei und Testname (Vitest `*.test.tsx`, Playwright `web/e2e/*.spec.ts`,
     Storybook-Story), die die Funktion heute belegen; `kein Test`, wenn keiner.
   - **Mockup**: Kennung `D0xx` aus `handoff/data/69_mockup_detailansichten.csv`, wenn das Mockup die
     Funktion zeigt; sonst `fehlt im Mockup`.
   - **Bemerkung**: Fallen für den Umbau (Fokusführung, Abhängigkeit von anderer Seite, Text nur in
     bestimmten Sprachen usw.).

   Felder mit `;` oder Zeilenumbruch in `"…"` setzen, `"` darin verdoppeln.

2. `<teil>.md`: je Seite ein Abschnitt mit
   - Aufbau heute (Kopf, Statusleiste, Reiter, Abschnitte, Tabellen, Dialoge, Unterseiten) in
     Stichworten, mit Datei- und Zeilenangaben;
   - Suchparameter und Deeplinks, Verweise von und zu anderen Seiten;
   - Abfragen mit Poll-Intervallen, Mutationen mit Bestätigungen;
   - Fallen für den Umbau;
   - **Abgleich mit dem Mockup**: jede `D0xx`-Zeile deiner Seiten eingestuft als
     `bestehende Funktion`, `visuelle Umgruppierung`, `Beispiel ohne Basis` (im Code gibt es das
     nicht; nicht bauen) oder `offene Frage`, mit einem Satz Begründung; dazu die Liste
     «im Mockup fehlt» (bestehende Funktionen, die das Mockup nicht zeigt und die trotzdem bleiben
     müssen).

## Regeln

- Massgeblich ist der Code im Arbeitsbaum
  `/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/.claude/worktrees/admin-redesign`. Arbeite nur
  dort. **Lies jede zugeteilte Datei ganz**, auch die Unterordner, Dialoge, Hilfsdateien und
  CSS-Module (Breitenumbrüche). Nichts überfliegen: eine vergessene Zeile ist später eine verlorene
  Funktion.
- Frühere Aufnahme als Starthilfe (Stand 12./13. November, **vor** den Releases 1.32 bis 1.38, also
  teils veraltet): `/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/.claude/worktrees/konzept-verwaltung-ux/docs/planning/23-verwaltung-ux/02-inventar-*.md`
  und `01-audit-*.md`. Jede übernommene Zeile am heutigen Code prüfen; was seither dazukam
  (Konzepte 21, 22, 23), ergänzen.
- Auch aufnehmen: Verhalten ohne eigenes Bedienelement (Polling, Fokus nach Löschen, Abfrage bei
  ungespeicherten Änderungen, Ansagen, Tastaturbedienung, Verhalten auf dem Handy unter 768 px,
  Anzeige für Nicht-Administratoren).
- Quelltext, Tests und Übersetzungen **nicht ändern**. Keine Tests, kein Build, kein Commit. Du
  schreibst nur deine zwei Dateien.
- Sprache der Dateien: Deutsch, Schweizer Schreibweise (ss statt ß). Bezeichner wie im Code.
- Nichts erfinden. Was du nicht am Code belegen kannst, als `unklar` markieren und in der Rückmeldung
  nennen.

## Rückmeldung an den Koordinator (kurz)

Zahl der Zeilen je Seite; was im Mockup fehlt und bleiben muss (die wichtigsten Punkte); Beispiele
des Mockups ohne Basis im Code; echte Widersprüche zwischen Mockup und Bestand; Unklares.
