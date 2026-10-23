# DoD bestätigt mit Vorbehalten

> **Nachtrag vom 23. Oktober 2026, nach diesem Durchgang.** Alle sieben Befunde sind behoben:
> B-6 (die vier Formulare mit dem formularweiten `useEdited`-Riegel — `useEdited` ist gelöscht, die
> Formulare hängen jetzt wirklich an `web/src/ui/Form`), B-1 (die Farb-Abweichung hat die Nummer
> **AW-6** und steht in der Abweichungstabelle und in der M10-DoD), die fehlenden
> Protokolleinträge für VAAPI/AMD und für die weggefallene Lighthouse-Prüfung, die Operationszahl
> (151, nicht 152), die Seitenliste von P-16 im Plan, der Beleg zu W-9 und der Kommentar zum
> `change`-Horcher in `Form.tsx`. Ausserdem tragen die Abweichungen jetzt die Nummern **AW-1 bis
> AW-9**, damit sie nicht mit den gleichnamigen Spec-IDs verwechselt werden. Der Bericht unten
> bleibt unverändert, so wie er geschrieben wurde.

Unabhängige Gegenprobe zu Punkt 6 der gemeinsamen DoD (`docs/planning/03-plan-v1.md` 6.2), Branch
`claude/m10-hardening-release`, 23. Oktober 2026. Geprüft wurde nur lesend; der Arbeitsbaum ist
unverändert (`git status --short --branch` → nur die Branch-Zeile).

Die Substanz hält: alle fünf M10-Schritte sind belegt, jeder stichprobenartig nachgeschlagene
Beleg existiert und deckt seine Matrixzeile, die beiden nachgelaufenen Belege sind grün, und im
Diff steht kein Platzhalter. Die Vorbehalte betreffen vier Stellen im Papier und eine im Code.

---

## 1. Stichprobe über die Abnahmezeilen

Geprüft: **29 der 64 Abnahmezeilen** und **8 der 16 Plan-Kriterien**, quer durch alle Gruppen.
Für jede wurde der genannte Beleg im Repository aufgeschlagen und mit der Matrixzeile aus Plan
Abschnitt 7 bzw. 8 verglichen.

| Gruppe | Geprüfte Zeilen | Befund |
|---|---|---|
| K | K-1, K-2, K-3, K-5, K-7, K-8 | Belege vorhanden; Zahlfehler in K-3 (siehe B-2) |
| B | B-2, B-6, B-7, B-9 | deckend |
| H | H-1, H-3, H-4 | deckend |
| W | W-2, W-3, W-9 | W-9 Beleg unvollständig (siehe B-5) |
| E | E-4, E-7, E-8 | deckend |
| O | O-5, O-6, O-7 | deckend |
| C | C-1, C-2, C-5 | C-1 mit undeklariertem Grund (siehe B-3) |
| A | A-1, A-2, A-4, A-5, A-8, A-9 | deckend |
| I | I-5, I-7, I-8 | deckend |
| P | P-1, P-2, P-6, P-10, P-11, P-13, P-15, P-16 | P-6 und P-16 siehe B-1 und B-4 |

Was im Einzelnen nachgeschlagen wurde:

- **Alle 51 nachgeschlagenen Go-Testnamen aufgelöst.** `grep -rn "func Test…"` über `server/` findet
  jeden, inklusive der als Regex-Präfix zitierten (`TestLogin` → `TestLoginFailures`,
  `TestLoginTooManyAttempts`, `TestLoginProfileChoice`; `TestReportOffline` → sechs Tests;
  `TestAdminMaintenance` → fünf Tests; `TestStats` → `TestStatsMatchRawData` u. a.). Kein
  toter Testname.
- **Alle 44 genannten E2E-Spezifikationen existieren** unter `web/e2e/` (63 Dateien gesamt),
  ebenso die Helfer `web/e2e/keyboard.ts` und `web/e2e/log-scan.ts`.
- **H-1 gegen die Matrixzeile gelesen** (`server/internal/library/audiobook_test.go:166`): der Test
  prüft 11 Dateien in zwei Hörbüchern (M4B und MP3-Ordner), genau 12 Kapitel, Startzeiten mit
  `d < -1000 || d > 1000`, also ±1 s, dazu Autor, Sprecher und eingebettetes Cover. Plan verlangt
  genau das. **Deckt.**
- **B-2** (`server/internal/library/names_test.go:233`): der Test bricht selbst ab, wenn der Korpus
  unter 150 Pfaden liegt. Plan verlangt ≥ 150 plus 20 NFO-Fälle. **Deckt.**
- **A-2** (Storybook): 57 Komponenten in `web/src/ui/*.tsx`, 57 `*.stories.tsx`, keine ohne Story.
  Plan verlangt eine Story je verwendeter `web/src/ui`-Komponente. **Deckt.**
- **P-15** (Tastatursuite): alle 37 im Bericht aufgezählten Seiten haben eine Spezifikation, und
  jede davon lädt den Helfer `keyboard`. 60 der 63 Spezifikationen nutzen ihn. **Deckt.**
- **O-6** (`docs/perf/M5-o6-langzeittest.md`): Tag 0, 1 und 3 auf Chromium 156 und Firefox 157 mit
  Messwerten; Tag 8 und 30 ausdrücklich «entfällt». Deckt die erklärte Abweichung B-1, nicht mehr.
- **P-2 / P-10 / P-11 / P-1** gegen `docs/perf/M10.md` gelesen: 83.0 MB Leerlauf-RSS bei 50'570
  Titeln (§ 1b), Image 507 MB (§ 1c), `healthy` 1.200–1.221 s auf CT 190 (§ 1a), LCP 1.802 s nach
  der Korrektur gegenüber 2.702 s davor (§ 8), Antwortzeiten (§ 3). Die Zahlen im Bericht stimmen
  mit der Quelle überein.

**Keine der 37 geprüften Zeilen hat einen Beleg, der die Anforderung nicht deckt.** Zwei sind
unvollständig oder zahlenmässig falsch zitiert (B-2, B-5 unten), keiner ist falsch.

## 2. Die Abweichungen

### Stehen alle acht im Entscheidungsprotokoll mit Datum?

**Sieben ja, eine nein.**

| Nr. | Eintrag in `docs/planning/04-entscheidungen.md` |
|---|---|
| A-1 | Zeile 11 (Frage 2) und Zeile 27, 2026-10-18 ✓ |
| **A-2** | **kein Eintrag.** `grep -in "vaapi\|amd"` findet nur Zeile 13 — und dort geht es um die Durchreichung der UHD 630 für QSV/VAAPI in CT 190, nicht um AMD als ungetestet ✗ |
| A-3 | Zeile 13 (Frage 4) und Zeile 31, 2026-10-19 ✓ |
| A-4 | Zeile 12 (Frage 3) und Zeile 28, 2026-10-18 ✓ |
| A-5 | Zeile 17 (Frage 8) und Zeile 38, 2026-10-20 ✓ |
| B-1 | Zeile 51, 2026-10-23 ✓ |
| C-1 | Zeile 52, 2026-10-23 ✓ |
| C-2 | Zeile 54, 2026-10-23 ✓ |

### Ist jede in der DoD von M10 genannt?

**Ja, alle acht.** `docs/planning/03-plan-v1.md:521` nennt AN-4 (= A-5), AN-12 (= A-4), AN-13 und
AN-7 (= A-1), AN-17 (= A-3), O-6 verkürzt (= B-1), VAAPI/AMD «best effort, ungetestet» (= A-2),
arm64 nur emuliert (= C-2) und die sieben Gerätetests (= C-1).

Auch die Rückverweise stimmen: jede Zeile, die eine Abweichungsnummer trägt, steht in der Spalte
«Betrifft» derselben Abweichung, und umgekehrt. Ich habe alle acht Listen gegengeprüft.

### Eine «grün» geführte Zeile, die in Wahrheit eine Abweichung trägt?

**Ja — eine, und sie ist nicht stillschweigend, aber sie fehlt in der Liste.** Siehe Befund B-1.

## 3. Die fünf Plan-Schritte von M10

| Schritt | Beleg | Stand |
|---|---|---|
| 1. Fremd-Client-Test (P-16) | `server/cmd/apicheck`, depguard-Regel `apicheck` in `server/.golangci.yml:42-44`; Lauf siehe Abschnitt 4 | **belegt** |
| 2. Leistung auf Referenz-NAS und arm64 | `docs/perf/M10.md` §§ 1a–1c, 2, 3, 4 (NAS) und 5, 6 (arm64, nur QEMU — Abweichung C-2) | **belegt** |
| 3. Migrationstest aller DB-Stände M1a → 1.0 | `TestFrozenStatesMigrate`, Lauf siehe Abschnitt 4; `testdata/db/M10.sql` im Diff; `TestFrozenM10StateCoversM10Data` in `server/internal/store/frozen_m10_test.go:16` | **belegt** |
| 4. Sicherheitsreview und Installationsanleitung de-CH | `docs/dev/SICHERHEITSREVIEW-1.0.md` (55'655 Byte), `docs/INSTALLATION.md` (deutsch, Schweizer Schreibweise, Laienton), Entscheid 2026-10-23 Zeile 56 | **belegt** |
| 5. Tag `v1.0.0` | ausdrücklich nicht Gegenstand dieser Prüfung | — |

## 4. Nachgelaufene Belege

Beide selbst ausgeführt, nicht übernommen. `task ci` und die vollen Pakettests wurden **nicht**
gestartet.

```
cd server && go test -race -count=1 -run TestFrozenStatesMigrate ./internal/store/ -v
--- PASS: TestFrozenStatesMigrate (135.30s)
    M1a 8.16s · M1b 8.01s · M2 7.81s · M3 8.26s · M4 11.91s · M5 13.34s
    M6 13.77s · M7 18.83s · M8 14.90s · M9 14.37s · M10 15.93s
ok  github.com/slouchcat/slouchcat/server/internal/store  136.318s   (exit 0)
```

**Elf Stände, elf bestanden.** Deckt DoD 6.2 Punkt 4 und P-13.

```
task apicheck
Lauf: 2026-10-23T13:02:20+02:00, Dauer 4s, 85 Aufrufe
Zusammenfassung: 18 von 18 Zuschauerfunktionen ok, 0 fehlgeschlagen.
EXIT=0
```

**18 von 18, Exit-Code 0.** Deckt P-16 und C-5, mit dem Vorbehalt aus Befund B-4.

Zusätzlich, weil schnell:

```
cd server && go test -count=1 -run 'TestContractCoversEveryOperation$' ./internal/api/ -v
--- PASS: TestContractCoversEveryOperation (0.01s)   (exit 0)
```

```
cd web && npx vitest run src/ui/Form.test.tsx
Test Files  1 passed (1)   Tests  5 passed (5)
```

**Nicht geprüft:** DoD 6.2 Punkt 2 (`task ci` auf diesem Branch) — läuft parallel in der
ausführenden Sitzung und wird hier nicht vorweggenommen. Der Bericht führt ihn selbst als Nachtrag.

## 5. Platzhalter im Diff

`git diff main...HEAD` (167 Dateien, +7'604/−732) gegen TODO, FIXME, HACK, XXX, «Platzhalter»,
`t.Skip`, `test.skip`, `it.only`, `describe.only`, `.fixme`, `xit(`, `xdescribe(`.

**Kein Blocker.** Alle Treffer sind harmlos und benannt:

- Zehn Treffer auf «Platzhalter» stehen in Dokumentation und beschreiben die Regel selbst
  (`docs/dev/SICHERHEITSREVIEW-1.0.md`, `docs/perf/M10.md` über den Platzhalterbaum
  `/srv/media/placeholder-tree`).
- `server/internal/api/…:3512` — `var placeholder = regexp.MustCompile(...)` ist ein Bezeichner
  für Pfadparameter im Vertrag, kein TODO.
- Zwei Treffer auf `os.Exit(2)` / `os.Exit(1)` in `cmd/apicheck` sind reguläre Beendigungen.
- **Kein einziges `t.Skip`, `test.skip`, `.only` oder `.fixme` im Diff.**

## 6. `web/src/ui/Form.tsx` und seine Tests

### Nageln die Tests das Verhalten fest?

**Weitgehend ja.** Die fünf Fälle in `web/src/ui/Form.test.tsx` sind nicht nur die glücklichen:

| Test | Prüft |
|---|---|
| `drops the server message of a field while it is corrected` | der eigentliche Zweck |
| `keeps the message of a field nobody touched` | **die wichtigste Abgrenzung** — pro Feld, nicht pro Formular |
| `shows an answer that arrives after the correction` | Antwort ohne Submit (Assistent) |
| `shows the message again when the next answer repeats it` | **der schwierige Fall**: wortgleiche Wiederholung |
| `clears the message when the next answer carries none` | Rücknahme |

Der Rundenzähler in `useAnswer` löst den wortgleichen Fall korrekt, **solange** zwischen zwei
identischen Antworten eine leere Runde liegt. Das ist bei allen 21 Aufrufstellen gegeben, weil sie
`mutation.error ? formErrors(t, mutation.error) : null` rechnen und `@tanstack/react-query` 5.104.2
den Fehler beim `mutate()` auf `null` setzt. `SetupPage` löst es von Hand: `setKeyCheck(null)` vor
`tmdbCheck.run(value)` (`web/src/features/setup/SetupPage.tsx:164-168`), und `pathCheck.data`
verschwindet während des Laufs. Auch die Flags `blank.path` / `blank.tmdb` werden beim Tippen
zurückgesetzt (Zeilen 262 und 317), also kein hängendes «Pflichtfeld».

### Verschwindet irgendwo eine Server-Meldung, obwohl sie noch gilt?

**Ja — aber nicht in `Form.tsx`, sondern in drei Seiten, die daran vorbeigreifen.** Siehe Befund
B-6. Die Komponente selbst habe ich auf diese Frage hin durchgespielt und keinen solchen Zustand
gefunden: `marked` gehört immer der Runde, in der es gesetzt wurde, eine veraltete Markierung wird
in Zeile 82 und 94 verworfen, und `useHeld` hält nur die Identität stabil, nie den Inhalt.

Zwei Lücken in den Tests bleiben (Befund B-7), beide in der ungefährlichen Richtung.

---

## Befunde

### B-1 — Die ΔE2000-Überschreitung ist eine neunte Abweichung ohne Nummer

**Fundstelle:** `docs/dev/ABNAHME-1.0.md:87` (W-2) und `:164` (P-6); `docs/planning/03-plan-v1.md:521`.

P-6 (b) verlangt «mittleres ΔE2000 ≤ 5 … auf Graustufenkeil und Hautton-Feldern». Gemessen sind für
`tonemap_cuda` **5.87 auf Hauttönen** — über der Grenze. Die Annahme ist im Entscheidungsprotokoll
sauber mit Datum und Begründung eingetragen (Zeile 37, 2026-10-20), und der Bericht nennt die Zahl
im Klartext. Sie fehlt aber in der Abweichungstabelle A-1…C-2, obwohl deren Einleitung sagt: «Sie
stehen hier zusammen, damit keine davon in einer Tabellenzeile untergeht.» W-2 und P-6 tragen als
Abw. nur A-1, A-2 und A-3 — keine davon ist diese. Und die M10-DoD zählt die erlaubten Abweichungen
abschliessend auf und schliesst mit «P-1–P-16 erfüllt», ohne Ausnahme für P-6.

**Zu tun:** Die Überschreitung als eigene Nummer (z. B. **D-1**, damit sie nicht mit der Abnahmezeile A-6 verwechselt wird) in die Abweichungstabelle des Berichts aufnehmen
(Betrifft: W-2, P-6; beschlossen 20.10.2026) und in `03-plan-v1.md:521` in die Aufzählung der
erlaubten Abweichungen ergänzen. Risiko: **mittel** — ohne das steht in der DoD eine Zusage, die
die Messung nicht hält.

### B-2 — Die Operationszahl stimmt dreimal nicht

**Fundstelle:** `docs/dev/ABNAHME-1.0.md:199` («alle 152 Operationen»), `:65` (K-3: «zuletzt 145
Operationen plus `receiveWebhook`»), `docs/dev/SICHERHEITSREVIEW-1.0.md:480` («alle **152**»).

`api/openapi.yaml` trägt **151** Operationen: 151 `operationId:`, 151 `x-visibility:`, und 151
Pfad/Methode-Paare über 125 Pfade (eigene Auszählung). Die Verteilung ist 78 `admin`, 39 `scoped`,
24 `account`, 10 `public`. `TestContractCoversEveryOperation` läuft grün, die Sache selbst hält
also; nur die Zahlen im Bericht stimmen nicht — 152, 146 und 151 stehen nebeneinander.

**Zu tun:** Beide Stellen auf 151 korrigieren und K-3 auf den aktuellen Stand nachführen.
Risiko: **niedrig** — Zahlendreher, keine Lücke im Nachweis.

### B-3 — C-1 «installierbar»: der Grund ist nicht beschlossen

**Fundstelle:** `docs/dev/ABNAHME-1.0.md:127`.

Die Matrixzeile C-1 verlangt ausdrücklich «Lighthouse ‹installierbar›». Der Bericht führt zwei
Gründe an, warum das fehlt: die nicht durchgeführte Installation am Handy (Abweichung C-1, gedeckt)
**und** «Lighthouse 12 kennt diese Prüfung nicht mehr». Der zweite Grund steht nirgends im
Entscheidungsprotokoll — `grep -in "lighthouse"` findet dort nur Zeile 57 zur LCP-Regression. Damit
gibt es für das Wegfallen eines Prüfmittels keinen datierten Entscheid.

**Zu tun:** Entweder einen Entscheid nachtragen («Lighthouse-Installierbarkeitsprüfung entfällt
mit Lighthouse 12; Ersatz: Manifest- und Service-Worker-Prüfung in
`web/e2e/offline-cold-start.spec.ts`») oder ein Ersatzmass nennen. Risiko: **niedrig** — die Zeile
ist als «grün mit Abweichung» geführt, wird also nicht stillschweigend gezählt.

### B-4 — P-16 im Plan nennt «Wünschen» noch, P-15 nicht mehr

**Fundstelle:** `docs/planning/03-plan-v1.md`, Abschnitt 8, Zeile P-16; Bericht `:174`.

Für P-15 wurde die Seite «Wünschen» am 23.10.2026 aus der Plan-Liste gestrichen und das im Plantext
vermerkt. Für P-16 steht «Wünschen» **immer noch** in der Aufzählung der Zuschauerfunktionen,
obwohl Entscheid 10 (22.10.2026) sie ersatzlos gestrichen hat. Der Bericht führt P-16 als «grün»
und erklärt den Wegfall im Belegtext — sachlich richtig, aber das Kriterium, wie es im Plan steht,
ist nicht erfüllt.

**Zu tun:** «Wünschen» in P-16 genauso streichen wie in P-15, mit demselben Datumshinweis.
Risiko: **niedrig**.

### B-5 — W-9: der Beleg nennt nur ein Drittel der Matrixzeile

**Fundstelle:** `docs/dev/ABNAHME-1.0.md:94`.

Die Matrixzeile W-9 verlangt dreierlei: Intro-Erkennung ±2 s bei ≥ 80 %, **Auto-Next nach
10-s-Countdown** und **Kapitelsprung ±1 s**. Der Beleg nennt nur `docs/perf/M4.md` 3 (89 %) und
`go test ./internal/segments`. Die anderen beiden sind tatsächlich abgedeckt —
`server/internal/segments/segments_test.go:193` `TestDetectChapters`, `:239` `TestChapterKind` und
`web/e2e/chapters.spec.ts` —, aber nicht zitiert.

**Zu tun:** `web/e2e/chapters.spec.ts` und die beiden Kapiteltests in die Belegspalte aufnehmen.
Risiko: **niedrig** — Belegtext zu knapp, nicht falsch.

### B-6 — Drei Seiten greifen an `Form.tsx` vorbei und löschen fremde Meldungen mit

**Fundstelle:**
- `web/src/features/admin/AccountsPage.tsx:49-50`
- `web/src/features/admin/AnnouncementsPage.tsx:61-62`
- `web/src/features/admin/BackupPage.tsx:72-73` und `:177-178`

Alle vier rechnen:

```tsx
const { edited, submitted, onInput } = useEdited();
const errors = save.error && !edited ? formErrors(t, save.error) : null;
```

`useEdited` (`web/src/lib/useEdited.ts:11-16`) ist **ein einziges Boolean für das ganze Formular**.
Der erste Tastendruck in **irgendeinem** Feld setzt es, `errors` wird `null`, und damit wird
`validationErrors` `undefined` — bevor `Form.tsx` überhaupt entscheiden kann, welches Feld
korrigiert wurde.

Konkret in `AnnouncementsPage`: der Server beanstandet `title` und `body`; der Leser kürzt den
Titel; **die Meldung zu `body` verschwindet, obwohl sie noch gilt**. Sie kommt erst beim nächsten
Submit zurück. Dasselbe in `BackupPage`, wo schon das Umschalten der RadioGroup `includeSecrets`
ein `input`-Ereignis auslöst und die Passphrasen-Meldung mitnimmt. Zusätzlich fällt in allen vier
Fällen `errors?.form` weg — der Formularhinweis für alles, was keinem Feld gehört.

Das ist genau der Zustand, den `Form.test.tsx`’ zweiter Test (`keeps the message of a field nobody
touched`) für die Komponente ausschliesst; diese vier Formulare erreichen ihn nie. Es ist **keine
Regression dieses Branches** — `useEdited` ist älter, der Branch hat in diesen Dateien nur die
Import-Zeile geändert (`git diff main...HEAD -- web/src/features/admin/BackupPage.tsx` zeigt genau
`-import { Form } from 'react-aria-components'` / `+  Form,`). Aber die Aussage, `Form.tsx` sitze
unter allen Formularen der App, trifft für diese drei Seiten nicht zu.

**Zu tun:** In den vier Stellen `useEdited` entfernen und `const errors = save.error ?
formErrors(t, save.error) : null` schreiben; `onInput` und `submitted()` fallen mit weg.
`web/src/lib/useEdited.ts` wird damit unbenutzt und kann gelöscht werden. Danach einen Test wie
`keeps the message of a field nobody touched` auf `AnnouncementsPage` ziehen, damit es nicht
zurückfällt. Risiko: **mittel** — sichtbarer Fehlbedienungspfad in vier Admin-Dialogen, aber kein
Datenverlust und keine Matrixzeile, die es zusagt.

### B-7 — Zwei untestete Pfade in `Form.tsx`

**Fundstelle:** `web/src/ui/Form.tsx:86-87`, `web/src/ui/Form.test.tsx`.

1. Der `change`-Horcher (Zeile 87) ist nie geprüft. Alle fünf Tests tippen in ein `TextField` und
   lösen nur `input` aus. Formulare mit `RadioGroup`, `CheckboxGroup` oder einem nativen `select`
   (z. B. `BackupPage`, `AnnouncementsPage`) hängen an diesem Zweig.
2. Die stille Voraussetzung, dass zwischen zwei wortgleichen Antworten eine leere Runde liegt,
   steht nur im Kommentar (Zeile 16-20). Sie ist bei allen heutigen Aufrufstellen erfüllt, aber
   nichts erzwingt sie für die nächste.

Beide Lücken wirken in die ungefährliche Richtung: eine nicht erkannte Korrektur lässt die Meldung
**stehen**, sie löscht sie nicht.

**Zu tun:** Einen sechsten Test mit `userEvent.selectOptions` oder einer `RadioGroup` ergänzen.
Risiko: **niedrig**.

---

## Was ohne Vorbehalt gilt

- Die elf eingefrorenen DB-Stände migrieren, selbst nachgelaufen (DoD 6.2 Punkt 4, P-13).
- Der Fremd-Client meldet 18 von 18, Exit-Code 0, selbst nachgelaufen (P-16, C-5).
- Alle fünf M10-Schritte ausser dem Tag sind mit einem Artefakt belegt.
- Alle acht erklärten Abweichungen stehen namentlich in der M10-DoD; sieben zusätzlich mit Datum
  im Entscheidungsprotokoll.
- Die Querverweise zwischen Abweichungstabelle und Matrixzeilen stimmen in beide Richtungen.
- Kein Platzhalter, kein übersprungener und kein Stub-Test im Diff.
- Jeder der 51 nachgeschlagenen Go-Testnamen und jede der 44 nachgeschlagenen E2E-Spezifikationen existiert.
- `web/src/ui/Form.tsx` selbst hat keinen Zustand, in dem eine noch geltende Meldung verschwindet.
