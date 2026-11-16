# Auftrag Schritt 4: unabhängige Funktionsprüfung

Du prüfst, ob das Verwaltungs-Redesign von Slouchcat eine bestehende Funktion verloren oder
verschlechtert hat. Du hast am Umbau nicht mitgearbeitet und glaubst den Berichten der Executor
nichts: massgeblich ist der Code.

## Lage

- Arbeitsbaum: `/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/.claude/worktrees/admin-redesign`,
  Zweig `claude/admin-redesign`. Stand vor dem Umbau: Commit `55b72c9a` (`git show 55b72c9a:<pfad>`,
  `git diff 55b72c9a..HEAD -- <pfad>`).
- Oberste Bedingung des Auftraggebers: **kein** Bedienelement, keine Einstellung, kein Dialog, kein
  Filter, kein Zustand, keine Fehlermeldung, keine Tastaturbedienung und keine Berechtigungsprüfung
  darf fehlen oder unzugänglich geworden sein.
- `docs/design/admin-redesign/bauplan.md`: was absichtlich anders ist (Navigation in sechs
  Bereichen, Bereichsreiter, Einleitung sichtbar statt Hilfe-Knopf, Übersicht neu, Serverfunktionen
  auf `/admin/server`, Abschnitt 7 Abweichungen). `rahmen-ergebnis.md`: was der Rahmen gebaut hat.
- `docs/design/admin-redesign/functional-inventory.csv`: 876 Funktionen des Bestands (Spalten siehe
  `BRIEF-INVENTAR.md`), mit `inventory/<teil>.md` als Erläuterung.
- `docs/design/admin-redesign/abdeckung/vorpruefung.csv`: maschinelle Vorprüfung je Zeile. «unverändert»
  heisst: die genannten Dateien sind seit `55b72c9a` gleich und die genannten Tests stehen noch da.
  «prüfen: …» nennt den Grund: `Code` (eine genannte Datei wurde geändert oder entfernt), `Tests`
  (ein genannter Test fehlt unter diesem Namen), `Rahmen` (die Zeile betrifft Navigation, Hilfe oder
  Einleitung, die der Rahmen geändert hat).

## Was du tust

Für **jede Zeile deiner Kürzel** (siehe dein Auftrag) schreibst du eine Zeile nach
`docs/design/admin-redesign/abdeckung/<teil>.csv`, Trennzeichen `;`, UTF-8, Kopfzeile genau:

`ID;Neue Stelle;Aenderung;Beleg;Status;Bemerkung`

- **Neue Stelle**: Bereich → Seite und `pfad/Datei.tsx:Zeile` im heutigen Code, an der die Funktion
  steht.
- **Aenderung**: `keine`, `Aussehen` (gleiche Stelle, gleiche Bedienung, anderes Aussehen),
  `verschoben` (andere Stelle oder Seite, gleiche Funktion), `umgebaut` (Bedienung geändert) oder
  `entfällt` (nur, wenn der Bauplan es ausdrücklich nennt, z. B. Hilfe-Knopf).
- **Beleg**: Testdatei und Testname im heutigen Stand, der die Funktion an der neuen Stelle prüft
  (Vitest, Playwright, Story). Prüfe, dass der Test die Funktion wirklich berührt, nicht nur so
  heisst. `kein Test`, wenn keiner.
- **Status**: genau einer von
  - `ok`: Funktion vorhanden, Test belegt sie.
  - `ok, ohne Test`: Funktion am Code nachgewiesen, kein Test; in der Bemerkung steht, ob sie schon
    vor dem Umbau keinen Test hatte.
  - `LÜCKE`: Funktion fehlt, ist unzugänglich oder schlechter als vorher (auch: nur noch über einen
    Umweg, Fokus geht verloren, Zustand wird nicht mehr gezeigt, Text nennt einen falschen Ort).
  - `LÜCKE Test`: Funktion vorhanden, aber sie wurde verschoben oder umgebaut und hat an der neuen
    Stelle keinen Test, oder ein Test, der sie vorher belegte, wurde gestrichen oder ausgehöhlt.
  - `unklar`: am Code nicht zu entscheiden; Bemerkung sagt, was fehlt.
- **Bemerkung**: bei `LÜCKE` genau, was vorher war (Datei und Zeile im Stand `55b72c9a`), was jetzt
  ist und was zu tun wäre.

Vorgehen:

1. Zeilen mit Vorprüfung «unverändert»: übernehmen als `Aenderung` `keine`, Beleg aus der Spalte
   «Tests heute» des Inventars, Status `ok` bzw. `ok, ohne Test`. Stichprobe: bei jeder zehnten
   solchen Zeile die Stelle im Code wirklich nachsehen.
2. Zeilen mit «prüfen»: jede einzeln am Code nachvollziehen. Lies dazu den Unterschied der Datei
   (`git diff 55b72c9a..HEAD -- <datei>`) und die heutige Datei. Ist ein genannter Test nur umbenannt
   oder verschoben, finde ihn; ist er weg, prüfe, ob ein anderer Test dieselbe Aussage macht.
3. Zusätzlich je Seite deiner Kürzel: den **ganzen** Unterschied der Seite lesen
   (`git diff 55b72c9a..HEAD --stat -- web/src/features/admin web/src/features/notifications web/e2e`
   als Einstieg) und nach Verlusten suchen, die keine Inventarzeile trifft: gestrichene Prüfungen in
   Tests (`expect`, `test(`), gelockerte Erwartungen, verlängerte Timeouts, `.skip`/`.only`,
   entfernte Texte in `web/src/locales/de.json`, entfernte `aria-`-Angaben, entfernte Ansagen.
4. Gezielt selbst laufen lassen darfst du nur einzelne Vitest-Dateien bei einem Zweifel
   (`cd web && pnpm exec vitest run <datei>`). Kein e2e, kein Storybook, kein `task ci`.

## Regeln

- Quelltext, Tests und Übersetzungen **nicht ändern**. Du schreibst nur deine `abdeckung/<teil>.csv`
  und `abdeckung/<teil>.md`. Kein Commit.
- `abdeckung/<teil>.md`: Liste aller `LÜCKE`, `LÜCKE Test` und `unklar` mit Begründung; Funde aus
  Schritt 3 (Verluste ohne Inventarzeile); Zählung je Kürzel und Status.
- Sprache: Deutsch, Schweizer Schreibweise (ss statt ß). Bezeichner wie im Code.
- Was der Bauplan als Abweichung nennt (Abschnitte 2.1, 2.6, 7, 7.1, 7.2), ist kein Verlust, solange
  die Funktion erreichbar bleibt. Im Zweifel `LÜCKE` oder `unklar` setzen, nicht `ok`.

## Rückmeldung an den Koordinator (kurz)

Zählung je Kürzel und Status; jede `LÜCKE` und `LÜCKE Test` in einem Satz mit ID und Stelle; Funde
ohne Inventarzeile; was unklar blieb.
