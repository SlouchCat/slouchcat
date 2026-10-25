# Plan: Spotlight und Kuratierung (Meilenstein M11, nach 1.0)

- **Status: Entwurf. Die elf Entscheide sind beantwortet (Marco, 25. Oktober 2026, Abschnitt 4.2);
  der Plan als Ganzes wartet noch auf Marcos Freigabe zur Umsetzung.**
- **Datum: 23. Oktober 2026, Entscheide eingearbeitet am 25. Oktober 2026**
- Verfahren: RALPLAN-DR, Modus **DELIBERATE** (Pre-mortem und erweiterter Testplan sind Pflicht,
  Abschnitte 8 und 9). Zweite Fassung nach Critic (ITERATE) und Architect (tragfähig mit Änderungen)
- Grundlage: Mockups A bis K (Design-Canvas, extern), gewählt sind **F/J** (Startseite), **H**
  (Admin Spotlight), **I** (Admin Cat's Empfehlungen) und **K** (Admin Startseite). Mockup I kam mit
  Marcos Antwort auf S-11 dazu und ist als **Schritt 5b** umgesetzt
- Ausgangslage: Version 1.0.0 ist veröffentlicht; `main` steht auf `2ca2282`, letzte Migration ist
  `00019_progress_days.sql`, letzter ADR ist `0010-webhook-empfang.md`, Vertragsfassung ist `1.0.0`
  (`api/openapi.yaml` Zeile 4). (`docs/dev/CURRENT_STATUS.md` nennt noch `6f78a5c`; der Stand dort ist
  überholt und wird in Schritt 9 nachgezogen)
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: Entscheidungsprotokoll > Plan > Spec (CLAUDE.md)
- **Jeder Schritt ist ein eigener Commit, und vor dem Abschluss jedes Schritts ist `task ci` grün**
  (CLAUDE.md «Arbeitsweise»). DoD-Belege sind Befehl **und** Ausgabe

> **Korrektur gegenüber dem ersten Entwurf.** `docs/spec/README.md` hält fest: «Die Spec wird nicht
> nachträglich umgeschrieben; Abweichungen stehen im Entscheidungsprotokoll.» Dieser Plan ändert
> darum **keine Zeile** in `docs/spec/01-anforderungen-v1.md`. Die Abweichungen zu E-1, E-7 und E-9
> werden als Frage 11 im Entscheidungsprotokoll und als ADR 0011 festgehalten (Schritt 0).

---

## 1. Ziel

Die Startseite bekommt eine neue oberste Bühne und verliert zwei feste Reihen:

1. **Spotlight** — eine in der Verwaltung wählbare öffentliche Trakt-Liste, auf die eigene
   Bibliothek gefiltert, dargestellt als gerahmtes Band mit Backdrop und darunter einer Posterleiste
   als Auswahl (ersetzt die Punkte des heutigen Karussells).
2. **Cat's Empfehlungen** — inhaltlich unverändert nach E-2 (`docs/spec/01-anforderungen-v1.md`
   Zeile 115: von Hand gepflegt, Reihenfolge, Notiz, Empfänger, Zeitraum), neu als Posterzeile mit
   Notiz unter dem Spotlight statt als Hero-Karussell.
3. **Kürzlich veröffentlicht** — neue Reihe nach `items.release_date`
   (`server/migrations/00003_libraries_items.sql` Zeile 56).
4. **Trending bei Trakt** und **Beliebt bei Trakt** entfallen; Spotlight deckt sie ab.
5. **Einzelne Einträge sind in der Verwaltung abänderbar**: ausblenden, verschieben, eigene Notiz.
6. **Knopf «Trailer»** neben «Abspielen» und «Details» auf der Startseite (W-10, Spec Zeile 111,
   gilt heute nur für die Detailseite).
7. **Neue Admin-Seite «Spotlight»** in der Gruppe `curation`; Reihenfolge und Ausblenden der Reihen
   bleiben auf der bestehenden Seite «Startseite» (`web/src/features/admin/HomeRowsPage.tsx`).
8. **Admin-Seite «Cat's Empfehlungen» nach Mockup I** (S-11): Spalte «Nr.», Posterbild in der
   Titelspalte und eine Hinweiszeile, die für Reihenfolge und Ausblenden auf die Seite «Startseite»
   verweist. Inhaltlich ändert sich an E-2 nichts.

## 2. Abgrenzung (CLAUDE.md Regel 11)

Nicht Teil dieses Plans, weil Non-Goal (`docs/spec/01-anforderungen-v1.md` Zeilen 55 bis 70):

- Trakt-Konten je Profil, Scrobbling, **Import von Trakt-Watchlists** (Zeile 61; bestätigt in
  Entscheid 9, `docs/planning/04-entscheidungen.md` Zeile 18). Das betrifft S-2 unmittelbar: die
  Eingabe «öffentliche Liste per Adresse» darf keine Watchlist-Adresse annehmen (Abschnitt 5.6).
- Algorithmische Empfehlungen (Zeile 59). Spotlight ordnet nach der Reihenfolge einer fremden
  öffentlichen Liste; es berechnet nichts aus dem Schauverhalten.
- Eigene Wunschverwaltung (Zeile 63). Titel, die **nicht** in der Bibliothek stehen, erscheinen auf
  der Startseite nicht. Der Schalter «Nur vorhandene Titel zeigen» aus Mockup H entfällt (S-4).
- Trailer-Reel und automatische Themenlisten (Zeile 60). Der Trailer-Knopf öffnet den bestehenden
  `TrailerPanel` (`web/src/features/catalog/TrailerPanel.tsx` Zeilen 30 bis 62).

Ebenfalls nicht Teil: eine neunte Reihe (R-2) und eine Anhebung von `visibility.MaxRowItems` (3.4).

**Nicht mehr ausgeschlossen:** die Admin-Seite «Cat's Empfehlungen» nach Mockup I. Sie stand bis zum
25. Oktober 2026 in dieser Liste; Marcos Antwort auf S-11 holt sie als Schritt 5b herein. Sie ändert
**nichts** an E-2 — dieselben Felder, dieselben Regeln, nur Darstellung und ein Querverweis.

---

## 3. RALPLAN-DR

### 3.1 Prinzipien

| Nr. | Prinzip | Woher |
|---|---|---|
| P1 | **Der Server entscheidet, der Client zeigt nur.** Sichtbarkeit, Altersstufe, Mindesttreffer, Overrides, Reihenfolge **und welche Reihe die Bühne ist** werden serverseitig bestimmt und stehen im Vertrag. | CLAUDE.md Regel 12, Plan 2.1 Prinzip 1, C-5 (Spec Zeile 138) |
| P2 | **Titeldaten nur durch die Sichtbarkeitsschicht.** Kein neues Paket, keine Änderung der Allowlists. | CLAUDE.md Regel 2, `server/tools/itemsaccess/itemsaccess.go` Zeile 28, `server/.golangci.yml` Zeilen 35 bis 41 |
| P3 | **Ein Ausfall von Trakt kostet nur die Reihe.** | I-7 (Spec Zeile 153), ADR 0008 Zeile 42 |
| P4 | **Kuratierung überlebt den nächtlichen Abruf.** Eine Notiz hängt an der Titelidentität und an der Liste, nie an einer Position. | `server/migrations/00015_discover.sql` Zeile 322 (`PRIMARY KEY (list, media, position)`) |
| P5 | **Jeder Commit ist grün.** Der Schnitt folgt der Baubarkeit, nicht dem Thema: `task ci` läuft an jeder Commit-Grenze durch. | CLAUDE.md «Arbeitsweise»; Abschnitt 6.1 |
| P6 | **Nur vorwärts, mit Sicherung davor.** Keine Down-Migration; die Rückkehr läuft über die automatische `pre-migration`-Kopie und A-4. | CLAUDE.md Regel 5, `server/migrations/TEMPLATE.sql.txt` Zeilen 6 bis 11, ADR 0009 Zeilen 41 bis 42 |

### 3.2 Decision Drivers (gewichtet)

| Rang | Driver | Gewicht | Was er entscheidet |
|---|---|---|---|
| 1 | **Die Startseite behält immer einen Kopf und bleibt unter den Messwerten** (P-10 LCP < 2.5 s, P-11 Startseiten-API < 300 ms p95, `docs/planning/03-plan-v1.md` Zeilen 613 und 614) | 0.40 | Entwurfsfrage **C** (wer steht oben, was bei leerer Bühne), die Entscheidung gegen eine Anhebung von `MaxRowItems` (3.4) und den Index auf `release_date` |
| 2 | **Kuratierung bleibt verlässlich** (eine Notiz steht morgen noch am selben Titel) | 0.35 | Entwurfsfrage **B**: Overrides als eigene Tabelle mit Listenidentität im Schlüssel |
| 3 | **Der Vertrag bleibt ehrlich und prüfbar** (C-5; jede Operation mit `x-visibility`, jede im Vertragstest; eine Regel hat genau eine Implementierung) | 0.25 | Entwurfsfrage **A**, das Vertragsfeld `HomePage.heroRow` (3.5) und die Anhebung auf 1.1.0 |

### 3.3 Entwurfsfrage A — Reihenschlüssel

Heute führt `home_rows` genau acht Schlüssel (`server/migrations/00015_discover.sql` Zeilen 76 bis
86), der Vertrag erzwingt `minItems`/`maxItems` 8 auf `PUT /admin/home-rows` (`api/openapi.yaml`
Zeilen 3887 und 3888) und `maxItems: 8` auf `HomePage.rows` (Zeile 6429); der Code hält
`MaxRows = 8` (`server/internal/discover/discover.go` Zeile 48).

| Option | Pro | Contra |
|---|---|---|
| **A1 (gewählt) — neue Schlüssel `spotlight` und `released`, `trakt_trending` und `trakt_popular` entfallen** | Der Schlüssel sagt, was die Reihe ist; jede spätere Lesart stimmt. Zwei raus, zwei rein geht auf `MaxRows = 8` genau auf. Eine Abfrage auf eine Tabelle statt zwei auf zwei Listen | Der `CHECK` auf `home_rows.key` lässt sich in SQLite nur durch einen Tabellen-Rebuild ändern. Das Enum `HomeRowKey` (Zeile 6399) verliert zwei Werte: für einen Fremd-Client (C-5) brechend, darum Vertragsfassung 1.1.0 |
| **A2 (verworfen) — beide Trakt-Schlüssel umwidmen** (`trakt_trending` = Spotlight, `trakt_popular` = Kürzlich veröffentlicht) | Kein Rebuild, Enum unverändert, keine Vertragsanhebung | `trakt_popular` für eine Reihe nach `release_date` ist eine dauerhafte Falschaussage in Datenbank, Vertrag, Code, Übersetzungen und — seit 3.5 — im Bühnenfeld. Spart einmalig vier Anweisungen und kostet dauerhaft |
| **A3 (verworfen) — Schlüssel unverändert lassen, nur den Inhalt konfigurierbar machen**: `list_key` in `spotlight_config`, `trakt_lists.list`-CHECK erweitert, Overrides wie B1 | Erfüllt «wählbare Liste, ausblendbar, verschiebbar, mit Notiz» **ohne** Enum-Bruch, ohne Rebuild, ohne Bühnenregel; der Vertrag bliebe bei 1.0.x. Der mit Abstand kleinste Eingriff | Die zwei Schlüssel bleiben zwei Reihen, das Ziel ist **eine**. Einer der beiden wird damit zur Karteileiche in einem von acht Plätzen — oder er wird für «Kürzlich veröffentlicht» umgewidmet, und dann **ist A3 für diesen Schlüssel genau A2**. Zweitens müsste die Bühnenregel «Band ist `trakt_trending`» lauten; mit dem Vertragsfeld aus 3.5 wandert die Falschaussage in die öffentliche Antwort |

| **A4 (verworfen) — A3 plus einen neunten Schlüssel `released`**: Enum **erweitert** statt getauscht, `minItems`/`maxItems` auf 9, `MaxRows = 9` | Für einen Fremd-Client ist ein hinzugefügter Enum-Wert deutlich milder als zwei entfernte: alter Code läuft weiter, er kennt die neue Reihe bloss nicht. Umgeht damit R-10 fast vollständig | Neun Reihen widersprechen **E-9** («höchstens acht Reihen», Spec Zeile 122) und **Driver 1**: eine Reihe mehr heisst mehr über der Faltkante und eine grössere Antwort in jedem Startseitenabruf. Der Schlüsselname bleibt zudem falsch (A3-Erbe) |

**Entwertung von A2, A3 und A4.** A3 ist die ernsthafteste Alternative und scheitert an einer Kette:
«Kürzlich veröffentlicht» braucht einen Schlüssel → ein **neunter** Schlüssel ist durch E-9 und
Driver 1 versperrt (das ist A4) → der Schlüssel muss aus den zwei Trakt-Zeilen kommen → damit trägt
A3 den Kern von A2 in sich. Der Vorteil «kein Enum-Bruch» verschwindet in dem Moment, in dem man
einen Schlüsselnamen nicht mehr glauben darf.

**Die Kette hängt an E-9, nicht an einer technischen Grenze.** Dass es genau acht sind, steht in
`minItems: 8` (`api/openapi.yaml` Zeile 3887) und ist eine Entscheidung aus M7; die Spec sagt nur
«höchstens acht» (Zeile 122). Wer den Plan später aufmacht, muss also gegen **E-9 und Driver 1**
argumentieren, nicht gegen ein Schema — und E-9 ist Marcos Entscheid, nicht unserer.

**Wann A3 gewinnen würde, ausdrücklich:** antwortet Marco auf S-1 mit «nein» (Karussell bleibt oben)
**und** streicht er die Reihe «Kürzlich veröffentlicht» (S-5), dann bleibt genau ein Umbau übrig —
die wählbare Liste. Dann ist A3 der richtige Plan, und dieser hier schrumpft auf die Schritte 0, 2,
4 und 5. Das ist die Bedingung, unter der der Plan neu geschnitten wird, nicht nachgebessert.

### 3.4 Anzahl und Mindesttreffer — wo die Zahlen wirklich stehen

Die feste 20 steht **nicht** in `discover`, sondern in `server/internal/visibility/visibility.go`
Zeilen 30 und 31 (`MaxRowItems = 20`) und wird in `server/internal/visibility/discover.go` über
`clamp` (`visibility.go` Zeile 389) hart geklemmt: `WatchlistRow` Zeile 62, `Genres` Zeile 114,
`Collections` Zeile 132, `TraktRow` Zeile 271. Ein `item_count = 30` käme still als 20 zurück.
Der Mindesttreffer steht in `server/internal/discover/discover.go` Zeile 45
(`TraktMinHits = 5`), angewandt in den Zeilen 146 bis 148.

| Option | Pro | Contra |
|---|---|---|
| **(gewählt) `item_count` auf 5 bis 20 begrenzen** | `MaxRowItems` und `clamp` bleiben unberührt; keine Wirkung auf die sechs anderen Reihen. 20 ist die Zahl, die E-7 und AN-3 nennen; die Posterleiste aus Mockup J zeigt ohnehin keine 30 Titel. `item_count` ist damit ein reiner **Senk**-Regler | Mehr als 20 geht nicht, auch nicht auf Wunsch |

**Was `item_count` zählt — ausdrücklich, weil der Begriff zwei Lesarten hat.** `item_count`
begrenzt die **sichtbaren Bibliothekstreffer je Profil**, nicht die abgerufenen Trakt-Einträge.
Genau so ist es heute: `queries/scoped/discover.sql` Zeilen 125 bis 133 setzt das `LIMIT` **nach**
dem Join auf `visible_items`, `visibility/discover.go` Zeilen 266 bis 280 fragt `2 × n` an,
dedupliziert und schneidet auf `n`, und der Vertrag sagt «the first 20 library titles»
(`api/openapi.yaml` Zeile 943). **Folge, die benannt sein muss:** blendet Marco fünf der zwanzig
aus, rücken die nächsten Treffer nach — die Reihe zeigt wieder zwanzig, nicht fünfzehn. Die Zahl der
**abgerufenen** Einträge ist davon unabhängig (heute Top 100 je Liste,
`server/internal/metadata/trakt.go` Zeilen 20 bis 23) und in 5.5 getrennt geführt.
| (verworfen) `MaxRowItems` anheben | Spotlight könnte 30 zeigen | Eine gemeinsame Klemme für sechs weitere Reihen wegen einer einzigen anheben: mehr Poster über der Faltkante (P-10) und grössere Antworten in jeder Reihe (P-11), für einen Nutzen, den niemand verlangt hat. Verstösst gegen Driver 1 |

Will Marco später mehr als 20, ist das ein eigener Plan mit eigener Messung über alle sieben Reihen.

### 3.5 Entwurfsfrage C — wer steht oben, und wer entscheidet das

E-1 (Spec Zeile 114) schreibt das Karussell «Cat's Empfehlungen» zuoberst fest; ADR 0008 Zeile 40
hält fest, dass es ohne Picks kein Karussell gibt. Der Server malt das erste Pick-Bild in die Shell
vor (`server/internal/api/preload.go`, `heroPicker` Zeilen 19 bis 22, `heroPrepaint` Zeile 142), die
Startseite sucht heute fest nach `picks` (`web/src/features/catalog/HomePage.tsx` Zeilen 147 bis
149).

| Option | Pro | Contra |
|---|---|---|
| **C1 (gewählt) — Spotlight ist die oberste Bühne, Cat's Empfehlungen wird Posterzeile mit Notiz** | Entspricht Mockup F/J. Eine grosse Bildfläche, darunter ruhige Zeilen — Design-Richtung (Spec Zeile 50) und E-9 (Zeile 122). E-2 bleibt vollständig: Notiz, Empfänger, Zeitraum reisen weiter, die Notiz über das bestehende `ItemRef.note` (`api/openapi.yaml` Zeilen 6185 bis 6187) | Ändert E-1, braucht also S-1. Prepaint muss mit, sonst Blitzer (P-10). Ist Spotlight leer, wäre die Seite ohne Gegenmittel kopflos |
| **C2 (verworfen als Dauerzustand) — Karussell bleibt oben, Spotlight darunter** | E-1 unverändert, kein Risiko einer kopflosen Seite | Zwei grosse Bühnen übereinander, gegen E-9; zweiter grosser Bilddownload über der Faltkante |

**C2 bleibt trotzdem erreichbar** — und zwar ohne Code: welche Reihe die Bühne ist, folgt aus der
gespeicherten Reihenfolge. Marco schiebt auf `/admin/home-rows` `picks` nach oben, und die Bühne
wechselt. C1 und C2 sind dieselbe Software in zwei Einstellungen.

#### Die Bühnenregel gehört in den Vertrag, nicht in den Client

Ein `FirstHero`, das nur ein Bild liefert, reicht **nicht**: die Entscheidung «welche Reihe wird als
Band gezeigt, welche als Posterzeile» bliebe im Client (`HomePage.tsx` Zeilen 147 bis 149), stünde
nirgends im Vertrag, und ein Fremd-Client könnte die Startseite nicht nachbauen (C-5). Zwei
Implementierungen derselben Regel sind genau die Divergenz, die R-3 abwehren soll.

**Entscheid:** `HomePage` bekommt das Feld **`heroRow`** (`HomeRowKey`, optional; fehlt es, gibt es
keine Bühne). Gesetzt wird es dort, wo `Home()` die Reihen ohnehin baut
(`server/internal/discover/discover.go` Zeilen 82 bis 102): **die erste nicht leere bühnenfähige
Reihe in der gespeicherten Reihenfolge**; bühnenfähig sind `spotlight` und `picks`.
`FirstHero` ist danach keine zweite Implementierung, sondern eine Ableitung aus demselben Ergebnis:
der erste Titel der von `heroRow` benannten Reihe. `preload.go` folgt **derselben Regel** — nicht
derselben Antwort: auf `GET /` gibt es keine `/home`-Antwort, `startPagePreload` (Zeile 54) ruft den
Katalogdienst direkt. Eine Regel, eine Stelle, im Vertrag sichtbar — und kein zusätzlicher Bruch,
weil die Antwort ohnehin auf 1.1.0 geht.

Ist keine bühnenfähige Reihe gefüllt, fehlt `heroRow`, und die Seite beginnt mit der ersten
Posterzeile. Das ist der heutige Zustand und bereits abgedeckt (`HomePage.tsx` Zeile 151).

### 3.6 Entwurfsfrage D — wie der Trailer-Knopf weiss, ob es einen Trailer gibt

Trailer liegen in `trailers` (`server/migrations/00008_library_complete.sql` Zeilen 45 bis 56);
`ItemDetail.trailerCount` gibt es (`api/openapi.yaml` Zeile 6282), `ItemRef` nicht.

| Option | Pro | Contra |
|---|---|---|
| **D1 (verworfen) — `ItemRef.hasTrailer` überall füllen** | Ein Feld, eine Regel, kein Sonderfall | `ItemRef` ist das meistgelesene Schema: Startseite, gepagte Suche, Sammlungen, Merkliste, Downloads. Überall ein `EXISTS` auf `trailers`, auch ohne Knopf. P-11 ist der engste Messwert |
| **D2 (verworfen) — kein Feld, Knopf immer zeigen** | Null Vertrags- und Abfrageänderung | Ein Knopf, der in `trailer.none` läuft (`TrailerPanel.tsx` Zeile 41), an der sichtbarsten Stelle der App |
| **D3 (gewählt) — `ItemRef.hasTrailer` optional, gefüllt nur in den Reihen `spotlight` und `picks`** | Ein `EXISTS` auf höchstens 40 Zeilen je Startseitenabruf; nichts in Suche, Sammlungen, Downloads. Vorlage `CountVisibleTrailers` (`server/queries/scoped/items.sql` Zeilen 152 bis 155) | Ein Feld, das nicht überall gesetzt ist. Die Beschreibung muss das eng und ausdrücklich sagen, sonst liest ein Fremd-Client «kein Trailer» statt «nicht bekannt» |

**Ausstiegsklausel:** zeigt die Messung in Schritt 8, dass der `EXISTS` auch in der Suche unter 5 ms
bleibt, wird D3 in einem eigenen Folge-Commit auf D1 gehoben. Vorher nicht.

---

## 4. Entscheidungsstand

### 4.1 Bereits entschieden — gilt, ist hier nicht zur Diskussion

| Thema | Entscheid | Quelle |
|---|---|---|
| Trakt-Umfang | Nur öffentliche Listen über einen Server-Schlüssel; kein Login je Profil, kein Scrobbling, kein Watchlist-Import, nichts wird gesendet | Spec Zeilen 48 und 61, I-7 Zeile 153, Entscheid 9 (`04-entscheidungen.md` Zeilen 18 und 42) |
| Trakt-Schlüssel | `SLOUCHCAT_TRAKT_CLIENT_ID` in `deploy/.env`, sonst verschlüsselt in `settings`; Umgebung gewinnt und sperrt die Oberfläche (`422 env_wins`) | Entscheid 9, ADR 0009 Zeile 45, `server/internal/admin/trakt.go` Zeile 12 |
| Ausfallverhalten | Ohne Schlüssel oder bei Ausfall fehlt nur die Reihe; der Fehlschlag wird als `Error` protokolliert und erscheint im Dashboard | I-7, ADR 0008 Zeile 42, ADR 0009 Zeile 40 |
| Sichtbarkeit | Reihe je Profil durch `visible_items`; der Client hält keine Regel | CLAUDE.md Regeln 2 und 12 |
| Reihenzahl | Höchstens acht Reihen, keine leeren, Reihenfolge vom Hauptbenutzer anpassbar | E-9 (Spec Zeile 122) |
| Spec-Pflege | Die Spec wird nicht umgeschrieben; Abweichungen ins Entscheidungsprotokoll | `docs/spec/README.md` |
| Migrationen | Nur vorwärts, nach Vorlage, mit `VACUUM INTO`-Sicherung davor | CLAUDE.md Regel 5, `TEMPLATE.sql.txt` |
| Gerätetests | Marco prüft vor einer Fassung nicht an echten Geräten; Abweichungen werden erklärt statt belegt | `04-entscheidungen.md` Zeile 52 |

### 4.2 Beantwortet — Marco, 25. Oktober 2026

Alle elf Fragen sind beantwortet. **Zehn davon wie vorgeschlagen; eine weicht ab (S-11).** Die
Vorschlagsspalte bleibt stehen, damit nachvollziehbar ist, worauf geantwortet wurde.

> **Die Neuschnitt-Bedingung aus 3.3 greift nicht.** Sie verlangte S-1 «nein» **und** S-5 «keine
> neue Reihe». Marco hat S-1 mit **ja** und S-5 mit **12 Monate fest** beantwortet, also beide in
> die andere Richtung. **A1 bleibt** — neue Schlüssel `spotlight` und `released`, Enum getauscht,
> Vertragsfassung 1.1.0. A3 und A4 sind damit endgültig vom Tisch und stehen nur noch als
> Alternativen im ADR.

| Nr. | Frage | Vorschlag | **Antwort (25.10.2026)** |
|---|---|---|---|
| **S-1** | Verdrängt Spotlight das Hero-Karussell (ändert E-1)? | **Ja.** Die Migration setzt `spotlight` auf Position 0. Umstellen ist danach ein Klick auf `/admin/home-rows`, kein Umbau (3.5) | **Ja, wie vorgeschlagen.** Spotlight verdrängt das Karussell, Cat's Empfehlungen wird Posterzeile. Betrifft Schritt 7b |
| **S-2** | Welche Listen sind wählbar? | Trending, Beliebt, Meistgesehen (Woche), Meistgesammelt (Monat), Am meisten erwartet, Kinocharts, öffentliche Liste per Adresse — je Filme, Serien oder beide. **Watchlist-Adressen werden abgelehnt** (Non-Goal, 5.6) | **Ja, wie vorgeschlagen** — die sieben Listenarten, Watchlist-Adressen abgelehnt (`422`). Betrifft Schritt 2 |
| **S-3** | Anzahl und Mindesttreffer einstellbar? | **Ja**, Anzahl **5 bis 20** (nicht 30, siehe 3.4), Mindesttreffer 0 bis 10. Ändert AN-3 aus Entscheid 9 und weicht von E-7 ab | **Ja, wie vorgeschlagen** — Anzahl 5 bis 20, Mindesttreffer 0 bis 10. AN-3 aus Entscheid 9 ist damit geändert. Betrifft Schritte 1, 2, 4 |
| **S-4** | Titel ohne Bibliothekstreffer auf der Startseite zeigen? | **Nein**, Non-Goal «eigene Wunschverwaltung». In der **Verwaltung** stehen sie trotzdem, damit Marco sieht, was Trakt geliefert hat | **Nein, wie vorgeschlagen** — nie auf der Startseite, in der Verwaltung sichtbar. Betrifft Schritte 2 und 5 |
| **S-5** | Zeitfenster «Kürzlich veröffentlicht» | **12 Monate, fest.** Nur Titel mit `release_date` | **12 Monate fest, wie vorgeschlagen**, nur Titel mit `release_date`. Betrifft Schritt 3 |
| **S-6** | Serien darin | **Datum der neuesten sichtbaren Folge**, nicht das Serienjahr | **Wie vorgeschlagen** — Datum der neuesten sichtbaren Folge. Betrifft Schritt 3 |
| **S-7** | Trailer-Knopf | **Ja**, nach D3: `hasTrailer` nur in den Reihen `spotlight` und `picks` | **Ja, wie vorgeschlagen** — `hasTrailer` nur in `spotlight` und `picks`. Betrifft Schritte 1 und 8 |
| **S-8** | Abrufintervall, «Jetzt abrufen», Veraltung | Intervall einstellbar **6 bis 48 Stunden** (Standard 24 wie heute); «Jetzt abrufen» höchstens einmal je Minute (`429`); die Veraltungsgrenze ist **nicht mehr die Konstante `TraktMaxAge = 48h`** (`visibility/discover.go` Zeilen 258 bis 261), sondern **2 × `interval_hours`** | **Wie vorgeschlagen** — 6 bis 48 h, Standard 24; «Jetzt abrufen» höchstens 1/min (`429`); Veraltung `2 × interval_hours` statt `TraktMaxAge`. Betrifft Schritte 1, 2, 4 |
| **S-9** | Was wird aus `HeroCarousel`? | **Bleibt in `web/src/ui`** und wird in Schritt 7b zum Band weiterentwickelt. Kein zweites Bühnen-Bauteil | **Wie vorgeschlagen** — `HeroCarousel` bleibt und wird in 7b zum Band. Betrifft Schritt 7b |
| **S-10** | Überlappung «Kürzlich veröffentlicht» / «Neu hinzugefügt» | **In Kauf nehmen.** Die Ausschluss-Variante ist in R-7 bewertet und verworfen | **Wie vorgeschlagen** — Überschneidung in Kauf genommen. Betrifft Schritt 3 |
| **S-11** | Mockup I (Admin Cat's Empfehlungen) jetzt umsetzen? | **Nein.** E-2 ändert sich inhaltlich nicht; `web/src/features/admin/PicksPage.tsx` bleibt unberührt. Mockup I gehört in einen eigenen kleinen Plan, sonst wächst dieser um eine Seite, die niemand braucht, damit die Startseite funktioniert | **Abweichung vom Vorschlag: ja, Mockup I wird mitgebaut.** Marco will die Admin-Seite im selben Zug umbauen. Der angekündigte **Schritt 5b** wird damit fällig |

Diese elf Zeilen gehören nach der Antwort als **Frage 11** ins Entscheidungsprotokoll, zusammen mit
den Abweichungen zu E-1, E-7, E-9, zu ADR 0008 und zu P-10 (Schritt 0).

---

## 5. Architektur

### 5.1 Reihen

Neue Standardreihenfolge, weiterhin genau acht Schlüssel:

```
spotlight, picks, continue, recent, released, watchlist, collections, genres
```

`MaxRows = 8` (`discover.go` Zeile 48), `minItems`/`maxItems` 8 (`api/openapi.yaml` Zeilen 3887 und
3888), `HomeRowSetting.position maximum: 7` (Zeile 8274) und `HomePage.rows maxItems: 8`
(Zeile 6429) bleiben unverändert. Mitzuführen sind `model.HomeRowKeys`
(`server/internal/model/model.go` Zeile 657), die Konstanten (Zeilen 646 bis 653), der Zweig in
`discover.row` (Zeilen 140 bis 149) **und der Paketkommentar** `discover.go` Zeilen 5 bis 8, der die
acht Reihen namentlich aufzählt.

### 5.2 Paketgrenzen (keine Änderung an den Lint-Listen)

| Aufgabe | Paket | Warum dort |
|---|---|---|
| Abruf, Schreiben von `spotlight_entries` | `metadata` | Schreibt heute `trakt_lists` (`server/internal/metadata/trakt.go`); in beiden Allowlists |
| Reiner HTTP-Zugriff auf Trakt | `trakt` | Hat keine Datenbank (ADR 0008 «Pakete») |
| Reihe lesen (`visible_items`) | `visibility` über `server/queries/scoped/spotlight.sql` | `ListTraktRow` (`queries/scoped/discover.sql` Zeilen 125 bis 133) ist die Vorlage |
| Reihen ordnen, leere weglassen, Mindesttreffer, `heroRow` setzen | `discover` | Hält kein Titel-SQL |
| Konfiguration und Overrides schreiben, Abruf auslösen | `admin` | Schreibt heute Picks, Sammlungen, Reihenfolge; hält `Trakt` als Schnittstelle |

`itemsaccess.go` Zeile 28 und `.golangci.yml` Zeilen 35 bis 41 bleiben **unverändert**.

#### Welche Abfrage in welches generierte Paket gehört

Keine Formalie: das entscheidet, wer die Konfiguration überhaupt lesen **darf**. `discover`
importiert heute **ausschliesslich `appq`** (`server/internal/discover/discover.go` Zeilen 14 bis
29) — folgerichtig, denn `home_rows` liegt in `server/queries/app/discover.sql` Zeile 26.
`systemq` darf es **nicht** importieren: `.golangci.yml` Zeilen 36 bis 41 nennt nur `visibility`,
`admin`, `library`, `metadata`, `jobs`, `notify`, `offline`. Und P2 verspricht dreimal, die
Allowlists nicht anzufassen — die naheliegende «Reparatur» ist damit ausgeschlossen.

| Tabelle | Abfragen in | Liest / schreibt | Warum dort |
|---|---|---|---|
| `spotlight_config` | **`server/queries/app/spotlight.sql`** → `appq` | `discover` und `visibility` lesen, `admin` schreibt | `discover` braucht `min_hits` und `item_count` für die Reihe und für `heroRow`, `visibility` braucht `interval_hours` für die Veraltung. `appq` ist frei importierbar (`server/sqlc.yaml`), und die Tabelle nennt **keine** Titel — CLAUDE.md Regel 2 ist gar nicht berührt |
| `spotlight_entries` | `server/queries/system/spotlight.sql` → `systemq` zum Schreiben, `server/queries/scoped/spotlight.sql` → `scopedq` zum Lesen | `metadata` schreibt, `visibility` liest | Die Tabelle trägt `tmdb_id` und wird gegen `items` gejoint; sie gehört hinter die Sichtbarkeitsschicht |
| `spotlight_overrides` | wie `spotlight_entries` | `admin` schreibt, `visibility` liest über den Join | wie oben |

**Wer liest die Veraltungsgrenze?** Heute ist sie die Konstante `TraktMaxAge`
(`server/internal/visibility/discover.go` Zeile 261) und wird in `TraktRow` als `min_fetched`
gesetzt (Zeilen 266 bis 280). Dort bleibt sie: **`visibility` berechnet sie** aus
`spotlight_config.interval_hours`, gelesen über `appq`. `discover` rechnet nichts daran — es kennt
nur `min_hits` und `item_count`.

### 5.3 Datenmodell und Migrationsweg

Migration **`server/migrations/00020_spotlight.sql`**, nach `server/migrations/TEMPLATE.sql.txt`:
**`-- +goose Up`** (Zeile 14 der Vorlage, ohne sie gibt es keinen Up-Block), `-- +goose NO
TRANSACTION`, `PRAGMA foreign_keys=OFF`, `BEGIN`, Anweisungen, `COMMIT`, `PRAGMA foreign_key_check`,
`PRAGMA foreign_keys=ON`. Keine Down-Migration. **Jede neue Tabelle ist `STRICT`**, wie jede Tabelle
im Repo (`00003_libraries_items.sql` Zeile 76, `00006_images_formats.sql` Zeile 39,
`00015_discover.sql` Zeilen 82 und 323, Beispiel in der Vorlage Zeile 23) — sonst nähme
`item_count` die Zeichenkette `'20'` an, `sqlc` erzeugte einen anderen Go-Typ als für die
Nachbartabellen, und die Grenzprüfung aus Schritt 4 verglichen Text statt Zahl. Der Server sichert vor
jedem Lauf mit `VACUUM INTO` und behält die neuesten fünf (TEMPLATE Zeilen 8 und 9).

1. **`home_rows` neu bauen** (der `CHECK` lässt sich in SQLite nicht ändern): `home_rows_new` mit
   `CHECK (key IN ('spotlight','picks','continue','recent','released','watchlist','collections','genres'))`,
   sonst identisch zu `00015_discover.sql` Zeilen 76 bis 82.
2. Bestehende Zeilen ohne die zwei Trakt-Zeilen übernehmen.
3. **Die zwei neuen Zeilen anlegen** — ohne diesen `INSERT` liefe die Migration durch, `home_rows`
   hätte **sechs** Zeilen, die Startseite keine Bühne, und das Ziel «genau 0 bis 7» wäre
   nachweislich falsch:
   `INSERT INTO home_rows_new (key, position, hidden, updated_at) VALUES ('spotlight', …, 0, <now>), ('released', …, 0, <now>);`
   Beide starten **sichtbar** (`hidden = 0`); die Positionen setzt Punkt 4.
4. **Explizite Zielpositionen vergeben** — **nicht** über `row_number()` mit Tiebreak `key`:
   `home_rows` hat **kein** `UNIQUE` auf `position` (Zeilen 76 bis 82), und bei Gleichstand ordnet
   ein alphabetischer Tiebreak `picks` **vor** `spotlight`, also genau falsch herum. Verfahren
   stattdessen: Marcos vorhandene Reihenfolge der sechs bleibenden Schlüssel wird über
   `ORDER BY position, key` zu einer Rangliste; `spotlight` wird fest auf **0** gesetzt, die sechs
   folgen in ihrer Rangfolge ab 1, und `released` wird unmittelbar **hinter** `recent` eingefügt.
   Danach tragen die acht Zeilen genau 0 bis 7.
5. `DROP TABLE home_rows; ALTER TABLE home_rows_new RENAME TO home_rows;`
6. **Die drei neuen Tabellen anlegen** (unten) — **vor** dem Löschen von `trakt_lists`, sonst ist
   der Übergang in Punkt 7 unmöglich.
7. **Die Singleton-Zeile von `spotlight_config` einfügen**, nach dem Vorbild der `home_rows`-
   Startwerte (`00015_discover.sql` Zeilen 84 bis 86). Ohne sie hat der Server keine Liste, keine
   Anzahl und kein Intervall — und `2 × interval_hours` (S-8) wäre undefiniert:

   ```sql
   INSERT INTO spotlight_config (id, enabled, list_key, media, item_count, min_hits, interval_hours, updated_at)
   VALUES (1, 1, 'trending', 'both', 20, 5, 24, <now>);
   ```

   Das sind genau die Werte, die E-7 und AN-3 heute als Konstanten halten: 20 Titel, ausgeblendet
   unter 5, täglicher Abruf. Die Migration ändert damit kein Verhalten, sie macht es nur schreibbar.
8. **Den eingefrorenen Stand übernehmen**, damit der M7-Beleg nicht verschwindet (6.1):

   ```sql
   INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, title, year, fetched_at)
   SELECT 'trending', media, position, tmdb_id, title, year, fetched_at
   FROM trakt_lists WHERE list = 'trending';
   ```

   **Nur `trending`, und zwar weil eine Singleton-Konfiguration nur einen `list_key` trägt.**
   Nicht etwa, weil das Schema es verböte: `list_key` steht **im** Primärschlüssel, `'trending'`
   und `'popular'` kollidieren dort gerade **nicht**. Die Kollision entstünde allein dadurch, beide
   Quelllisten fest unter **einen** Schlüssel zu kopieren — in `testdata/db/M7.sql` stehen
   `('popular','movie',1,7001)` und `('trending','movie',1,7001)` nebeneinander. Dieser Unterschied
   ist wichtig für Follow-up 3 (mehrere Listen): das Schema kann zwei Listen nebeneinander halten,
   die **Konfiguration** kann es nicht. Siehe den benannten Verlust unten.
9. `DROP TABLE trakt_lists;` (der Index `trakt_lists_tmdb`, Zeile 324, fällt mit). **Erst zulässig,
   wenn im selben Commit die letzten Leser verschwunden sind** — siehe 6.1.
10. **Index für die neue Reihe:** `items` hat Indizes auf `added_at` (`00003_libraries_items.sql`
   Zeile 80), `tmdb_id`, `parent_id` — aber **keinen** auf `release_date` (Zeile 56). Neu:
   `CREATE INDEX items_released ON items (release_date DESC) WHERE release_date IS NOT NULL AND deleted_at IS NULL;`
11. Die Settings `trakt_last_success`, `trakt_last_attempt`, `trakt_last_error` bleiben stehen: der
    Abruf holt weiterhin bei Trakt.

**Zwei benannte Verluste**, beide im Abnahmetext und in der DoD von Schritt 1:

1. War `trakt_trending` oder `trakt_popular` ausgeblendet, geht dieses `hidden` mit den Zeilen
   verloren; die neuen Reihen starten sichtbar.
2. Die Einträge der Liste **«Beliebt»** werden nicht übernommen (Punkt 8). Das ist ein Cache-Inhalt,
   kein Datenverlust: beim nächsten Lauf füllt die konfigurierte Liste die Tabelle. Zwischen
   Migration und erstem Lauf zeigt Spotlight die übernommenen Trending-Titel — solange der
   eingefrorene Stand genug Treffer hat **und sein `fetched_at` jünger ist als `2 ×
   interval_hours`** (S-8). Bei einem Server, der länger gestanden hat, ist die Reihe nach der
   Migration leer, bis der erste Lauf durch ist; das ist genau das Verhalten, das I-7 vorsieht, und
   kein Sonderfall.

| Tabelle | Spalten | Schlüssel und Hinweis |
|---|---|---|
| `spotlight_config` | `id` (immer 1), `enabled`, `list_key`, `list_url`, `media`, `item_count`, `min_hits`, `interval_hours`, `last_manual_refresh`, `updated_at` | `PRIMARY KEY (id) CHECK (id = 1)`. `item_count` 5 bis 20 (3.4), `min_hits` 0 bis 10, `interval_hours` 6 bis 48. `last_manual_refresh` ist der Zeitstempel für die Minutensperre von `refreshSpotlight` — in der Datenbank, nicht prozesslokal, damit sie einen Neustart überlebt und prüfbar ist |
| `spotlight_entries` | `list_key`, `media`, `position`, `tmdb_id`, `title`, `year`, `fetched_at` | `PRIMARY KEY (list_key, media, position)`, `WITHOUT ROWID`, Index auf `(media, tmdb_id)`. **Reiner Cache**, je Lauf in einer Transaktion ersetzt |
| `spotlight_overrides` | `list_key`, `media`, `tmdb_id`, `hidden`, `pinned_rank`, `note`, `updated_at` | `PRIMARY KEY (list_key, media, tmdb_id)`, `WITHOUT ROWID`. **Nie** an `position` hängen (P4); `list_key` gehört in den Schlüssel, damit ein Listenwechsel die Notizen nicht auf fremde Titel kippt. `note` mit `CHECK (length(note) <= 280)`, gleich lang wie die Pick-Notiz (`api/openapi.yaml` Zeilen 8148 bis 8150 und 8161 bis 8164). Kein Fremdschlüssel auf `items`: ein Eintrag ohne Bibliothekstreffer muss ausblendbar sein |

**Ordnungsregel: `pinned_rank` statt `sort_offset`.** Die erste Fassung dieses Plans ordnete nach
`(position + sort_offset)` mit dem Tiebreak «kleinere Ursprungsposition». Das ist **falsch**, und
zwar auf eine Art, die kein Test der ersten Fassung gefunden hätte: zieht Marco Eintrag 5 auf
Platz 1, schreibt die Verwaltung `sort_offset = −4`, Summe 1 — Eintrag 1 hat ebenfalls Summe 1 und
gewinnt den Tiebreak. Der Eintrag landet auf **Platz 2 und kommt nie weiter**. Die DoD prüfte
«zwei Einträge gleicher Summe, stabile Ordnung», also **Determinismus statt Zielgenauigkeit** — sie
wäre grün gewesen, während die Funktion den Auftrag verfehlt.

Die zwei naheliegenden Reparaturen reichen nicht: den Tiebreak umzudrehen («gesetzter Offset
gewinnt») richtet das Hochziehen, aber nicht das Hinunterschieben; Bruchzahlen zwischen den
Nachbarn brechen, sobald der nächtliche Lauf alle Positionen neu vergibt — und genau das ist der
Normalfall (P4).

Darum trägt `spotlight_overrides` statt `sort_offset` ein **`pinned_rank`** (`INTEGER`, 1-basiert,
`NULL` = nicht angeheftet):

```
ORDER BY (o.pinned_rank IS NULL) ASC, o.pinned_rank ASC,
         e.position ASC, e.media ASC, i.id ASC
```

Angeheftete Einträge stehen in ihrer Rangfolge zuoberst, die übrigen füllen in natürlicher
Reihenfolge auf. «Nach Platz 1 verschieben» heisst dann `pinned_rank = 1` und bedeutet **Platz 1**,
nicht «ungefähr vorne». Der Rang überlebt den nächtlichen Lauf, weil er an der Titelidentität hängt
und nicht an einer Position, die sich verschiebt — dieselbe Logik wie bei `hidden` und `note` (P4).
Testfälle in 8.1, darunter ausdrücklich «Eintrag 5 nach Platz 1 verschoben steht danach auf
Platz 1».

### 5.4 Vertragsänderungen

`api/openapi.yaml` ist die Quelle; danach **`task gen`**. Generierter Code unter
`server/internal/api/gen/`, `server/internal/store/*q/`, `packages/api-client/src/generated/` wird
nie von Hand geändert (CLAUDE.md Regel 1).

| Stelle | Änderung |
|---|---|
| `info.version` (Zeile 4) | `1.0.0` → `1.1.0` (das Enum verliert Werte: brechend für Fremd-Clients) |
| `HomeRowKey` (Zeilen 6397 bis 6399) | `[spotlight, picks, continue, recent, released, watchlist, collections, genres]`. Entfernte Werte werden nie wiederverwendet |
| `HomePage` (Zeilen 6423 bis 6429) | **Neues Feld `heroRow`** (`$ref HomeRowKey`, optional): die Reihe, die als Band gezeigt wird; fehlt es, gibt es keine Bühne (3.5) |
| `ItemRef.note` (Zeilen 6185 bis 6187) | Beschreibung «only in the row `picks`» wird falsch, sobald die Notiz auch in `spotlight` reist — mitändern. Dort steht **kein** `maxLength`; neu `maxLength: 280`, gleich wie `PickRequest.note` (8148 bis 8150) und `PickUpdate.note` (8161 bis 8164) |
| `ItemRef` (ab Zeile 6114) | Neues Feld `hasTrailer: boolean`, optional, Beschreibung eng: «nur in den Reihen `spotlight` und `picks` von `GET /home` gesetzt; fehlt es, ist unbekannt, ob ein Trailer vorliegt» (D3) |
| `TraktStatus` (Zeilen 8278 bis 8302) | `trending` und `popular` entfallen; neu `listKey` und `entries`. Mitzuziehen: `server/internal/model/model.go` Zeilen 803 und 804, `server/internal/api/picks_handlers.go` Zeilen 189 bis 191, `server/internal/metadata/trakt.go` Zeilen 219 bis 224. **`server/internal/admin/dashboard.go` Zeilen 156 und 157 braucht keine Änderung**: dort werden nur `LastSuccess`, `LastError` und `LastAttempt` gelesen |
| `POST /admin/trakt/refresh` (Zeile 3933) | **Entfällt** zugunsten von `refreshSpotlight`. Die interne Methode `Trakt.RefreshTrakt` bleibt: `server/internal/admin/settings.go` Zeile 307 ruft sie beim Schlüsselwechsel |
| `GET /admin/trakt` (Zeile 3911) | **Bleibt**, `x-visibility: admin`; nur noch von der Spotlight-Seite gelesen |
| `/admin/home-rows` (Zeile 3856) | Unverändert; nur die Beschreibung nennt die neuen Schlüssel |
| **`getHome`, Beschreibung (Zeilen 942 bis 947)** | Zählt heute die acht Reihen namentlich auf und sagt «`trakt_trending` and `trakt_popular` (the first 20 library titles of the public Trakt lists, hidden under 5 hits …, E-7, I-7, AN-3)». Nach dem Umbau ist **jeder Teil davon falsch**: die Namen, die 20 und die 5 (beide stehen dann in `spotlight_config`). Neu zu schreiben, inklusive `heroRow` |
| **`getTraktStatus`, Beschreibung (Zeilen 3915 bis 3918)** | «how many entries each list holds» — es gibt nur noch **eine** Liste (`listKey`, `entries`) |
| **Abschnittskommentar (Zeile 3485)** | «Cat's picks, collections, row order, Trakt» — nennt Spotlight nicht |

**Warum die drei Beschreibungen eine eigene Zeile wert sind.** Beschreibungen sind Prosa, und
**kein Test fängt sie**: `TestContractCoversEveryOperation` zählt Operationen,
`TestContractNoTraktPerProfile` erlaubt das Wort «trakt» unter `/home` ausdrücklich. Eine falsche
Beschreibung fährt still in die Fassung 1.1.0 und trifft C-5 — ein Fremd-Client liest, er bekomme
die ersten 20 Titel zweier Trakt-Listen, und bekommt etwas anderes. Es ist derselbe Defekt wie bei
`ItemRef.note` («only in the row `picks`»), nur an drei weiteren Stellen.

**Neue Operationen**, alle `x-visibility: admin`, jede mit Eintrag im Vertragstest
`TestContractCoversEveryOperation` (`server/internal/api/contract_access_test.go` Zeile 313):

| Operation | Pfad | Zweck |
|---|---|---|
| `getSpotlightConfig` | `GET /admin/spotlight` | Liste, Medienart, Anzahl, Mindesttreffer, Intervall, aktiv |
| `setSpotlightConfig` | `PUT /admin/spotlight` | Dieselben Werte; `422 validation_failed` ausserhalb der Grenzen (S-3, S-8) und bei einer Watchlist-Adresse (5.6) |
| `getSpotlightEntries` | `GET /admin/spotlight/entries` | Der zuletzt abgerufene Stand mit Position, Titel, Jahr, Bibliothekstreffer, Override; **verwaiste Overrides ausdrücklich als solche** |
| `setSpotlightEntryOverride` | `PATCH /admin/spotlight/entries/{media}/{tmdbId}` | Ausblenden, Verschieben, Notiz; leerer Rumpf löscht den Override |
| `refreshSpotlight` | `POST /admin/spotlight/refresh` | `202`; `422 provider_not_configured` ohne Schlüssel; `429` bei mehr als einem Abruf je Minute (`last_manual_refresh`) |

**Dazu in Schritt 5b** (Teilentscheidung 5b-a), ebenfalls `x-visibility: admin`:

| Operation | Pfad | Zweck |
|---|---|---|
| `getAdminItemImage` | `GET /admin/items/{id}/images/{kind}/{size}` | Poster oder Backdrop eines Titels **systemweit, ohne Profilprüfung**, für Admin-Tabellen. Nach **CLAUDE.md Regel 6** ein `http.Handler` mit Beschreibung im Vertrag, weil sie ein Bild liefert und kein JSON — wie ihr `scoped` Gegenstück `getItemImage` (Zeile 992), das als Vorlage dient. Trägt `?v=<Inhalts-Hash>` zum Cache-Busting, **kein** `p`. Fügt sich in die bestehende Familie `/admin/items/{id}/…` ein (Zeilen 2581, 2625, 2654, 2697, 2726) |
| — | `AdminPick.posterUrl` | Neues optionales Feld (ab `api/openapi.yaml` Zeile 8097), zeigt auf die Route oben. **Additiv, also nicht brechend**; die Fassung bleibt 1.1.0 |

Beide tragen einen Eintrag in `TestContractCoversEveryOperation` beziehungsweise im Schema-Test.
`getAdminItemImage` gehört ausserdem in die Liste der **binären** Routen, die
`TestBinaryRoutesMatchContract` (`server/internal/api/contract_test.go` Zeile 214) prüft — sie
antwortet nicht über den generierten strict server.

Admin-Antworten nennen Titel mit `id` und `title` und sind **keine** `ItemRef` — der Kommentar dazu
steht in `api/openapi.yaml` Zeilen 3957 und 3958.

#### Der Schema-Test verbietet die neuen Namen zunächst

`TestContractNoTraktPerProfile` (`server/internal/api/contract_test.go` Zeile 269) erlaubt das Wort
«trakt» in einem Schema nur bei Präfix `trakt`, `homerow`, `adminsettings` oder bei
`integrationstatus` (Zeilen 286 und 287) und in einem Pfad nur unter `/admin/trakt*` oder `/home`
(Zeile 297). Jedes Spotlight-Schema nennt Trakt (Listenarten, `list_url`), jeder Pfad
`/admin/spotlight*` ebenso — der Test schlägt fehl, **bevor** irgendetwas falsch ist. In Schritt 1
mitzuändern:

- Allowlist der Schemas (Zeilen 286 und 287) um Präfix `spotlight` erweitern.
- Pfadprüfung (Zeile 297) um `/admin/spotlight` erweitern.
- Kopfkommentar zu I-7 (Zeilen 265 bis 268) nachführen: er zählt heute «die Startseiten-Reihen, die
  die Listen zeigen» auf und muss Spotlight nennen.

Die Absicht des Tests — kein Trakt in Profil-, Gast-, Einladungs-, Sitzungs- und Kontoschemas
(Zeilen 282 bis 285) — bleibt unangetastet. Genau das ist der Teil, der I-7 schützt.

### 5.5 Sichtbarkeit und Zahlen

Die Reihe wird je Profil durch `visible_items` gefiltert (Regeln 2 und 12). Der Mindesttreffer wird
**nach** der Verschneidung je Profil geprüft, wie heute (`discover.go` Zeilen 146 bis 148).

**Drei Zahlen, die nicht verwechselt werden dürfen** — die Oberfläche muss sie getrennt benennen
(`web/src/locales/{de,en}.json`):

| Zahl | Was sie ist | Wo sie gilt |
|---|---|---|
| **Abgerufene Einträge** | Was Trakt geliefert hat, vor jeder Verschneidung. Heute die Top 100 je Liste (`metadata/trakt.go` Zeilen 20 bis 23) | **systemweit**, nur in der Verwaltung |
| **Treffer in der Bibliothek** | Davon die, die es als Titel gibt — ebenfalls ohne Profilfilter | **systemweit**, nur in der Verwaltung |
| **`item_count`** | Wie viele **sichtbare** Treffer die Reihe höchstens zeigt (3.4) | **je Profil**, auf der Startseite |

«20 Einträge, 11 davon in der Bibliothek» ist damit eine Aussage über den Server, nicht über ein
Profil: ein Kinderprofil kann weniger sehen und die Reihe wegen des Mindesttreffers gar nicht
bekommen, obwohl die Verwaltung grün meldet.

**Offen und bewusst nicht hier entschieden:** wie viele der abgerufenen Einträge
`getSpotlightEntries` zurückgibt. Der Job holt heute 100 je Liste; eine Admin-Tabelle mit 100 oder
200 Zeilen ohne Paginierung ist keine Tabelle mehr. Schritt 4 legt darum eine **Obergrenze und
Sortierung** der Admin-Liste fest; 100 Einträge sind die Obergrenze des Abrufs und zugleich die
grösstmögliche Länge der Liste, und mehr als 100 wird nie gespeichert.

### 5.6 Watchlist-Adressen ablehnen (Regel 11)

S-2 erlaubt eine öffentliche Liste per Adresse. Trakt-Watchlists sind davon ausgenommen: ihr Import
ist Non-Goal (Spec Zeile 61, Entscheid 9). `setSpotlightConfig` prüft die Adresse serverseitig und
antwortet `422 validation_failed` mit `listUrl: invalid_value`, wenn der Pfad auf eine Watchlist
zeigt (`/users/<name>/watchlist…`). Die Prüfung steht im Server, nicht im Client (P1); Testfall in
8.1.

---

## 6. Schritte

### 6.1 Warum der Schnitt der Baubarkeit folgt, nicht dem Thema

Der Codegen hat **zwei Hälften**, und beide hängen an Schritt 1. `task gen` ruft `server:gen` **und**
`web:gen` (`Taskfile.yml` Zeilen 28 bis 32); `task gen:check` prüft den Diff über
`server/internal/api/gen`, `server/internal/store` **und `packages/api-client/src/generated`**
(Zeilen 80 bis 83); `task ci` fährt danach `lint` (darin `web:lint` mit `tsc --noEmit`), `test`
(darin `vitest`), `build` und `e2e` (Zeilen 64 bis 73). Ein Commit, der nur die Go-Seite nachzieht,
ist also genauso rot wie einer, der nur das Schema ändert.

#### Hälfte 1 — Go: `sqlc`, Konstanten, Felder, Fixtures

`sqlc` liest das Schema aus `migrations` (`server/sqlc.yaml`). `DROP TABLE trakt_lists` in einem
Commit, der die Leser stehen lässt, macht `task gen` und damit `task ci` **per Konstruktion rot**.
Die Leser sind:

| Stelle | Was |
|---|---|
| `server/queries/system/trakt.sql` Zeilen 6, 9, 15 | `TraktDeleteLists`, `TraktInsertEntry`, `TraktCountLists` |
| `server/queries/scoped/discover.sql` Zeilen 125 bis 133 | `ListTraktRow` |
| `server/internal/metadata/trakt.go` Zeile 173 | `dropTraktLists` |
| `server/internal/metadata/trakt.go` Zeilen 219 bis 224 | Füllen von `TraktStatus.Trending`/`Popular` |
| `server/internal/discover/discover.go` Zeilen 140 bis 149 | Zweig `RowTraktTrending`/`RowTraktPopular` |
| `server/internal/visibility/discover.go` Zeilen 264 bis 273 | `TraktRow`, `TraktMaxAge` |
| `server/internal/api/picks_handlers.go` Zeilen 189 bis 191 | `GetTraktStatus200JSONResponse` mit `Trending`/`Popular` |
| `server/internal/store/migration00015_test.go` Zeile 64 | erwartet `picks` auf Position 0 |
| `server/internal/store/migration00015_test.go` Zeile 67 | erwartet die Tabelle `trakt_lists` nach dem vollen Migrationslauf |
| `server/internal/store/frozen_m7_test.go` Zeilen 39 bis 41 | drei Zusicherungen auf `trakt_lists`-Inhalt |
| `server/internal/discover/discover_test.go` Zeilen 195, 197, 233 | `INSERT INTO trakt_lists` als Fixture |
| `server/internal/discover/discover_test.go` Zeilen 206, 210, 222 | `RowTraktTrending`, `RowTraktPopular` als Konstanten — **Compile-Fehler**, dieselbe Klasse wie `curation_test.go` Zeile 302 |
| `server/internal/visibility/discover_test.go` Zeilen 268, 270, 338 | `INSERT INTO trakt_lists` als Fixture |
| `server/internal/metadata/trakt_test.go` Zeile 106 | `SELECT … FROM trakt_lists` |
| `server/internal/metadata/trakt_test.go` Zeilen 150, 165, 192, 312, 317, 321, 325, 340 | `st.Trending` / `st.Popular` — **acht Compile-Fehler** |
| `server/internal/metadata/trakt_test.go` Zeilen 299 bis 303 | prüft `visibility.TraktMaxAge != 2*TraktInterval` — **die Zusicherung, die S-8 ablöst** |
| `server/internal/admin/trakt_test.go` Zeile 50 | `model.TraktStatus{Trending: 7, Popular: 3}` — **Compile-Fehler**, sobald die Felder entfallen |
| `server/internal/admin/curation_test.go` Zeile 302 | `model.RowTraktPopular`, `model.RowTraktTrending` — **Compile-Fehler** |
| `server/internal/api/contract_curation_test.go` Zeile 293 | `rows[0].Key != "picks" \|\| rows[7].Key != "trakt_popular"` — nach der Migration steht `spotlight` auf 0 und `rows[7]` ist `genres` |
| `server/internal/api/contract_curation_test.go` Zeile 331 | `order := []string{…, "trakt_trending", "trakt_popular"}` per `PUT /admin/home-rows` — unbekannter Schlüssel, Vertrag und `CHECK` weisen ab |
| `server/internal/api/contract_curation_test.go` Zeile 344 | Fixture «twice picks» mit beiden Trakt-Schlüsseln |
| `server/internal/api/contract_access_test.go` Zeilen 239 bis 241 | `setHomeRows`-Rumpf mit allen acht alten Schlüsseln |
| `server/internal/api/contract_access_test.go` Zeile 244 | Eintrag `"refreshTrakt": {contract: "POST /admin/trakt/refresh"}` — **diese Karte speist `TestContractCoversEveryOperation`** (Zeile 313): der Eintrag muss raus, die fünf neuen Operationen müssen hinein |

Nur die ersten vier Zeilen der Tabelle sind Schemafehler. Alles darunter bricht beim **Übersetzen**
oder an einer **Zusicherung** — `sqlc` sieht davon nichts, `task test` schon. Die beiden
API-Vertragstests tragen die Schlüssel zudem als **String-Literale**: sie fallen durch jede Suche
nach Go-Konstanten und durch jede Suche nach `trakt_lists`. Genau deshalb standen sie bis zur
dritten Fassung dieses Plans nicht in der Liste.

#### Hälfte 2 — Web: `tsc`, vitest, Playwright

`packages/api-client/src/generated/schema.d.ts` Zeile 3358 trägt heute `HomeRowKey` mit den acht
alten Werten. Sobald `web:gen` darüberläuft, bricht:

**Wie weit `tsc` reicht.** Das Skript `lint` in `web/package.json` Zeile 11 fährt `tsc --noEmit`
über `tsconfig.json` **und** `tsconfig.node.json`, und dessen `include` nennt `e2e` **und**
`e2e-storybook`. Die e2e-Dateien sind damit keine reine Playwright-Angelegenheit: sie werden
übersetzt, und ein Tippfehler dort macht `task web:lint` rot, ohne dass ein Browser startet.

| Stelle | Was | Wer meldet es |
|---|---|---|
| `web/src/features/admin/HomeRowsPage.tsx` Zeile 26 | `} as const satisfies Record<HomeRowKey, string>` über der Karte in Zeilen 18 bis 25 | `tsc --noEmit` |
| `web/src/features/admin/HomeRowsPage.tsx` Zeilen 82 und 83 | `String(status.data.trending)`, `String(status.data.popular)` — `TraktStatus` verliert beide Felder | `tsc` |
| `web/src/features/admin/HomeRowsPage.test.tsx` Zeilen 22 bis 28 | `const configured: TraktStatus = { …, trending: 40, popular: 38 }` — eine **getypte** Konstante mit zwei Feldern zu viel; `src` liegt im Haupt-`tsconfig.json` | `tsc`, nicht erst vitest |
| `web/src/features/admin/queries.ts` Zeile 254 | `api.POST('/admin/trakt/refresh')` — der Pfad fällt aus dem generierten `paths`-Typ | `tsc` |
| `web/src/features/catalog/HomePage.tsx` Zeile 181 mit der Karte in Zeilen 36 bis 45 | `t(rowTitleKey[row.key])` | `tsc` |
| `web/e2e/home.spec.ts` Zeilen 73 bis 75 | `ROW_TITLES.traktTrending`, `ROW_TITLES.traktPopular` und `const both = ['trakt_trending', 'trakt_popular']` — sobald Schritt 1 `discover.ts` Zeilen 22 und 23 umstellt, gibt es diese Eigenschaften nicht mehr. Mechanisch zu `ROW_TITLES.spotlight` und `['spotlight']` | `tsc` über `tsconfig.node.json` |
| `web/src/features/admin/HomeRowsPage.test.tsx` Zeilen 14 und 15, 89 und 90, 46, 104 bis 127 | Schlüsselpaare und die zwei Trakt-Pfade | vitest |
| `web/src/features/catalog/HomePage.test.tsx` Zeilen 299 und 300 | beide Trakt-Reihen in den Testdaten | vitest |
| `web/e2e/curation.ts` Zeilen 41 und 42, `web/e2e/discover.ts` Zeilen 22, 23, 131, 132, `web/e2e/admin-home.spec.ts`, `web/e2e/admin-home-rows.spec.ts` | Schlüssellisten, Titelkarten, Helfer `refreshTrakt` | `tsc` und Playwright |
| `web/src/locales/de.json` und `en.json` Zeilen 253 und 254 | `home.rows.traktTrending` / `.traktPopular`; dazu fehlen `home.rows.spotlight` und `home.rows.released` | erst der e2e-Titelvergleich (`home.spec.ts` Zeile 37) — `locales.test.ts` prüft die de/en-Symmetrie und dass `de.json` die Schweizer Schreibweise einhält, also kein scharfes S trägt (Zeilen 17 bis 19), **aber nicht, ob ein Schlüssel existiert** — ein in beiden Sprachen fehlender fällt dort nicht auf |

Zwei dieser Zeilen findet die `grep`-Zusicherung aus der DoD von Schritt 1 **nicht**:
`HomeRowsPage.tsx` Zeilen 82 und 83 und `HomeRowsPage.test.tsx` Zeilen 22 bis 28 schreiben
`trending` und `popular` ohne Präfix. Dafür ist `tsc` zuständig, und nur deshalb steht
`task web:lint` als eigene DoD-Zeile dort — die Suche nach `trakt_*` ist das gröbere Netz.

#### Hälfte 3 — Storybook: der dritte Gate

`task ci` fährt als letztes `storybook:test` (`Taskfile.yml` Zeile 73).
`web/e2e-storybook/components.spec.ts` Zeile 61 vergleicht **je Story ein committetes Bild**
(`toHaveScreenshot`), Zeile 45 erzwingt zusätzlich, dass **jede** Komponente in `web/src/ui` eine
Story-Datei hat. Dieser Gate betrifft Schritt 1 nicht — keine Story nennt die Reihenschlüssel —,
aber zwei spätere Schritte, und er stand bis zur vierten Fassung dieses Plans **nirgends** darin:

| Schritt | Was anschlägt |
|---|---|
| 5 (neues Symbol) | `web/src/ui/Icon.stories.tsx` Zeile 5 führt die gezeigten Namen in einem **von Hand gepflegten** `names: IconName[]`-Array — ein neues Symbol erscheint dort **nicht** automatisch. Baseline: `web/e2e-storybook/__screenshots__/ui-icon--all.png` |
| 7b (Band) | Vier Baselines weichen ab: `ui-herocarousel--default.png`, `--focused.png`, `--poster-only.png`, `--with-note.png` (Stories `Default`, `Focused`, `PosterOnly`, `WithNote` in `HeroCarousel.stories.tsx` Zeilen 44 bis 53). Die neue Posterleiste braucht ausserdem eine **eigene Story**, sonst deckt der Gate sie nicht ab |
| 8 (Trailer-Knopf) | Der dritte Knopf sitzt an der Bühne, und die Bühne **ist** `HeroCarousel`: die zwei vorhandenen Knöpfe stehen in `web/src/ui/HeroCarousel.tsx` Zeilen 206 bis 225, gespeist aus `HeroSlide.playHref`/`detailsHref` (Zeilen 26 und 27) und `HeroCarouselProps.playLabel`/`detailsLabel` (Zeilen 34 und 35). Ein dritter Knopf heisst: ein Feld mehr in `HeroSlide`, ein Label mehr in `HeroCarouselProps`, also eine Änderung in `web/src/ui` — und damit weichen **alle** `ui-herocarousel--*.png` ein **zweites** Mal ab, dann **fünf** statt vier, weil 7b die Posterleisten-Story dazugestellt hat |

Schritt 8 ist der Grund, warum dieser Abschnitt existiert. `task ci` fängt die Abweichung
zuverlässig — aber an einer Stelle, die niemand erwartet, der «Knopf an der Startseite» liest und
nicht «Änderung an einer `ui`-Komponente mit fünf Baselines». Genau diese Sorte Halt sollte ein
Plan vorwegnehmen, nicht die CI.

**Entscheid:** Schritt 1 nimmt Migration, Abfragen, Go-Code, Vertrag, `task gen`, **alle neun
Go-Testdateien und die mechanische Web-Anpassung** in einen Commit. «Mechanisch» heisst auf der
**Web**-Seite: Schlüssel umbenennen, zwei Statusfelder ersetzen, einen Pfadnamen ersetzen, Fixtures
und Titelkarten nachziehen. Der inhaltliche Web-Umbau bleibt, wo er hingehört: Trakt-Block kürzen
und Status umziehen in Schritt 6, `heroRow` lesen in 7a, das Band in 7b.

**Auf der Server-Seite ist Schritt 1 nicht rein mechanisch, und das muss gesagt sein:** er schrumpft
`traktLists` von vier Paaren auf eines (siehe unten) und macht damit aus zwei Reihen eine. Das ist
eine inhaltliche Änderung, die sich nicht verschieben lässt — ohne sie kollidiert der nächtliche
Lauf. Wer den Commit liest, soll sie im Body finden, nicht suchen müssen.

#### Warum der Commit nicht kleiner geht — der Beweis

«Jeder kleinere Schnitt ist an mindestens einer Grenze rot» ist eine Behauptung; hier ist sie
ausgerechnet. Es gibt genau drei Arten, die Schlüsseländerung zu teilen:

1. **Additiv zuerst** (`spotlight` und `released` dazu, die zwei Trakt-Schlüssel später weg): der
   Zwischenstand führt **zehn** Schlüssel. Das bricht gleichzeitig `minItems`/`maxItems: 8`
   (`api/openapi.yaml` Zeilen 3887 und 3888), `position maximum: 7` (Zeile 8274), `MaxRows = 8`
   (`discover.go` Zeile 48) und den `CHECK` der Migration — und widerspricht für die Dauer des
   Zwischenstands E-9.
2. **Subtraktiv zuerst** (die zwei Trakt-Schlüssel weg, die neuen später): der Zwischenstand führt
   **sechs oder sieben**. Das bricht `minItems: 8` beim ersten `PUT /admin/home-rows` — also bei der
   ersten Benutzung der Seite «Startseite».
3. **Kardinalität zwischendurch lockern** (`minItems`/`maxItems` für zwei Commits aufmachen): ein
   **bewusst instabiler Vertrag** über zwei Commits, und genau in dem Fenster ist nichts davon
   geprüft. Das ist kein kleinerer Schnitt, das ist ein abgeschalteter Wächter.

Es gibt aber eine **vierte Tür, und die ist grün: Tausch je Schlüssel.**

4. **Commit A** tauscht `trakt_trending` gegen `spotlight` (Migration, `spotlight_entries`,
   `spotlight_config`); `trakt_popular` bleibt vorerst auf `trakt_lists`. **Commit B** tauscht
   `trakt_popular` gegen `released` und lässt `trakt_lists` fallen. An **jeder** Grenze stehen genau
   acht Schlüssel: `minItems`/`maxItems`, `position maximum`, `MaxRows` und der `CHECK` halten,
   und `frozen_m7_test.go` Zeile 38 («eight home rows») ebenfalls.

Die Grösse des Commits ist also **nicht erzwungen, sondern gewählt** — die drei oberen Türen sind
zu, die vierte ist offen. Ihr Preis: **zwei Migrationen statt einer**, eine Übergangszeit, in der
`trakt_lists` und `spotlight_entries` nebeneinander bestehen und zwei Abrufwege gepflegt werden,
und zweimal `task gen` über beide Hälften. Dafür wären die Commits je etwa halb so gross.

**Die Wahl fällt auf einen Commit**, weil der Übergangszustand — zwei Cache-Tabellen, zwei
Abrufwege, ein Schlüssel, der schon `spotlight` heisst, neben einem, der noch `trakt_popular`
heisst — teurer zu verstehen ist als ein grosser, im Body sauber geteilter Diff. Wer anders
entscheidet, hat damit kein Argument gegen sich, sondern einen bekannten Preis: diesen Absatz.

#### Wie die Startseite zwischen Schritt 1 und 7a aussieht

Nach Schritt 1 steht `spotlight` auf Position 0, aber `HomePage.tsx` Zeilen 147 bis 149 sucht noch
fest nach `picks`, und `lists = rows.filter(row => row !== picks)` lässt Spotlight schlicht als
**erste Posterzeile** erscheinen, während das Karussell Bühne bleibt. Das ist genau **C2** aus 3.5
— der Zustand, den der Plan ohnehin als dauerhaft erreichbar führt. Der Zwischenstand ist damit
nicht nur lauffähig, sondern **vorführbar**, und zwar in einer Gestalt, die als Endzustand zulässig
wäre. Das ist keine glückliche Fügung, sondern eine Eigenschaft, die aus der Bühnenregel folgt, und
sie gilt für jeden Stand zwischen Schritt 1 und 7a.

Nachzuführen sind:

- **`migration00015_test.go`**: `spotlight` auf Position 0 statt `picks` (Zeile 64); die
  Tabellenliste prüft `spotlight_config`, `spotlight_entries`, `spotlight_overrides` statt
  `trakt_lists` (Zeile 67).
- **`frozen_m7_test.go`**: die drei `trakt_lists`-Zusicherungen weichen Zusicherungen auf
  `spotlight_entries` — beide Medienarten vorhanden, mindestens ein Eintrag mit Bibliothekstreffer.
  Der Übergang aus 5.3 Punkt 8 füllt die Tabelle, **also prüft der Test echte Zeilen und keine
  leere Tabelle**.
- **`discover_test.go`**: zwei verschiedene Arbeiten, nicht eine. Die Fixtures (Zeilen 195, 197,
  233) wechseln auf `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id,
  fetched_at)`; die **Konstanten** `RowTraktTrending`/`RowTraktPopular` (Zeilen 206, 210, 222)
  werden zu `RowSpotlight`. Die Fälle «unter dem Mindesttreffer» und «veraltet» bleiben inhaltlich.
  Zeile 210 (`rowOf(rows, RowTraktPopular).Key != ""`) hat nach dem Umbau kein Gegenstück mehr —
  es gibt nur noch eine Liste — und entfällt.
- **`visibility/discover_test.go`**: Fixtures Zeilen 268, 270, 338 auf `spotlight_entries`.
- **`metadata/trakt_test.go`** — **umschreiben, nicht nachführen.** Acht Stellen (150, 165, 192,
  312, 317, 321, 325, 340) lesen `st.Trending`/`st.Popular` und werden zu `st.Entries`; Zeile 106
  liest `spotlight_entries` ohne `list`-Sortierung. Der Kern ist aber Zeile 302: der Test prüft
  heute `visibility.TraktMaxAge != 2*TraktInterval` und bindet damit **Veraltung und
  Abrufintervall** aneinander (Kommentar Zeilen 297 bis 299: «the status counts no entries either,
  like the start page»). Genau diese Beziehung ersetzt S-8 durch `2 × interval_hours`. Würde der
  Test nur nachgeführt, verschwände die **einzige Zusicherung**, die die beiden Zahlen koppelt — und
  ein späterer Umbau könnte sie auseinanderlaufen lassen, ohne dass etwas rot wird. Er prüft künftig
  gegen `spotlight_config.interval_hours`, und zwar für zwei Intervalle (6 h und 48 h), damit die
  Kopplung und nicht ein Zahlenwert belegt ist.
- **`admin/trakt_test.go`**: `fakeTrakt` liefert `model.TraktStatus{Configured: true, ListKey:
  "trending", Entries: 10}`.
- **`admin/curation_test.go`**: die erwartete Schlüsselliste (Zeile 302) auf die neuen acht.
- **`api/contract_curation_test.go`**: Zeile 293 erwartet `spotlight` auf 0 und `genres` auf 7;
  die Reihenfolge in Zeile 331 und die Fixture «twice picks» in Zeile 344 bekommen die neuen
  Schlüssel.
- **`api/contract_access_test.go`**: der `setHomeRows`-Rumpf (Zeilen 239 bis 241) auf die neuen
  acht; der Eintrag `"refreshTrakt"` (Zeile 244) entfällt, und die **fünf neuen Operationen** aus
  5.4 kommen in dieselbe Karte — ohne sie meldet `TestContractCoversEveryOperation` (Zeile 313)
  jede als fehlend.
- **Web, mechanisch:** die vier `tsc`-Stellen, zwei vitest-Dateien, fünf Playwright-Dateien und die
  beiden Sprachdateien aus der Tabelle oben. In `de.json` und `en.json` ersetzen
  `home.rows.spotlight` und `home.rows.released` die Zeilen 253 und 254 — in **beiden** Sprachen,
  weil `locales.test.ts` Symmetrie und Schweizer Schreibweise prüft (Zeilen 14 bis 19), aber keine
  Existenz — ein beidseitig fehlender Schlüssel fällt erst im e2e-Titelvergleich auf.
- **Kommentare, die die acht Schlüssel oder `trakt_lists` nennen:**
  `server/internal/model/model.go` Zeile 638, `server/internal/discover/discover.go` Zeilen 5 bis 8,
  `server/internal/metadata/metadata.go` Zeile 46 («replaces `trakt_lists`») und
  `server/internal/metadata/trakt.go` Zeile 22 («the two start page rows»). Kommentare sind keine
  Kosmetik: sie sind das, was die nächste Sitzung liest.

#### Was Schritt 1 nicht enthält, und was er stattdessen tut

Nicht enthalten, damit der Commit reviewbar bleibt: die wählbare Liste und die Overrides (Schritt 2).
Der Zwischenstand ist trotzdem lauffähig, und zwar konkret so:

- **Eine Liste, nicht zwei — und der Job ist dafür zu ändern.** `traktLists`
  (`server/internal/metadata/trakt.go` Zeilen 40 bis 44) hält heute **vier Paare**
  (trending/popular × movie/show), der Kopfkommentar (Zeilen 20 bis 23) sagt es ebenso. Schritt 1
  **schrumpft `traktLists` auf das gewählte Paar** und schreibt nur Trending (Filme und Serien)
  unter `list_key = 'trending'`. Bliebe der Job wie er ist, schriebe er beide Listen unter einen
  Schlüssel, der `INSERT` kollidierte, die Transaktion liesse die Tabelle leer — und die Reihe
  verschwände **jede Nacht**. Das wäre Szenario 7.1, ausgelöst vom Plan selbst.
  Der Grund für genau eine Liste ist einfach: **eine Singleton-Konfiguration trägt nur einen
  `list_key`.**
- **Welche Liste die Reihe zeigt**, steht ab der Migration in `spotlight_config` (5.3 Punkt 7) und
  nicht mehr im Code — mit den heutigen Werten, nur eben schreibbar.
- **«Beliebt» fällt weg**, ab Schritt 1 und sichtbar. Das ist der zweite benannte Verlust aus 5.3
  und kein Nebeneffekt: Spotlight **ist** eine Liste, und Marco wählt ab Schritt 2, welche.

### Schritt 0 — Entscheide und Papiere
`docs(planning,adr): record the spotlight decisions and ADR 0011`

Antworten auf 4.2 als **Frage 11** in `docs/planning/04-entscheidungen.md`, dazu die Abweichungen:

- **E-1** (Karussell zuoberst weicht dem Spotlight), **E-7** (zwei feste Reihen weichen einer
  wählbaren Liste; Anzahl und Mindesttreffer einstellbar, ändert AN-3), **E-9** (neue
  Standardreihenfolge; Zahl bleibt acht).
- **W-10** (Spec Zeile 111: «Trailer lassen sich von der Detailseite aus abspielen»): Schritt 8
  bringt den Knopf zusätzlich auf die Startseite. Eine Erweiterung, keine Einschränkung — aber eine
  Abweichung vom Wortlaut, und ADR 0011 nennt W-10 bereits, also muss das Protokoll es auch.
- **ADR 0008 teilweise abgelöst:** Zeile 40 («ohne Picks gibt es kein Karussell» — ersetzt durch die
  Bühnenregel mit `heroRow`) und Zeile 42 (vier feste Listen, «höchstens 20, ausgeblendet unter 5» —
  ersetzt durch `spotlight_config`). Beides im ADR 0008 als Hinweis vermerken, nicht löschen.
- **P-10** lautet in `docs/planning/03-plan-v1.md` Zeile 613 «Startseite: Karussell + ≤ 8 Reihen,
  keine leere, LCP < 2.5 s». Das Wort «Karussell» ist überholt; die Abweichung gehört ins Protokoll,
  die Abnahmezeile in Schritt 9.
- **Der Meilenstein heisst M11.** `03-plan-v1.md` endet bei M10; die Nummer ist nirgends vergeben
  und wird hier gesetzt, damit `docs/perf/M11.md` der Regel «Messwerte nach `docs/perf/<Mx>.md`»
  folgt. Eine Zeile in Frage 11 — sonst ist der Dateiname eine stille Erfindung.

Neuer ADR `docs/adr/0011-spotlight.md` nach Abschnitt 10.

**DoD:** `git diff --stat docs/spec/` ist leer (Beleg im Commit-Body). Frage 11 trägt Datum. ADR 0011
liegt vor und nennt E-1, E-2, E-7, E-9, I-7, W-10, A-2, C-5, K-3. ADR 0008 trägt den
Ablösungshinweis. `task ci` grün (reiner Doku-Commit, der Lauf belegt, dass nichts anderes im Baum
liegt).

### Schritt 1 — Schema, Vertrag und Ablösung von `trakt_lists`
`feat(spotlight): replace the trakt rows with spotlight and released`

Der grosse Commit aus 6.1. Der Bereich ist **`spotlight`**, nicht `server`: CLAUDE.md verlangt einen
Bereich, nicht genau ein Verzeichnis, und dieser Commit umfasst beide Hälften des Codegen. Die
Alternative `feat(server,web)` wäre ebenfalls regelkonform, benennt aber den Ort statt die Sache.

- **Schema und Abfragen:** Migration 00020 nach 5.3; `queries/system/spotlight.sql` ersetzt
  `queries/system/trakt.sql`; `queries/scoped/spotlight.sql` ersetzt `ListTraktRow`.
- **Go:** `metadata`, `discover`, `visibility`, `picks_handlers`, `model` nachziehen; `TraktMaxAge`
  aus `spotlight_config.interval_hours` (S-8); die vier Kommentare aus 6.1.
- **Vertrag:** alles aus 5.4 — `heroRow`, `hasTrailer`, `ItemRef.note`, `TraktStatus`, die drei
  Beschreibungen, die Schema-Test-Allowlist. Danach `task gen` (Go **und** Web).
- **Neun Go-Testdateien** nach der Liste in 6.1, `metadata/trakt_test.go` davon umgeschrieben.
- **Web, nur mechanisch:** vier `tsc`-Stellen, zwei vitest-Dateien, fünf Playwright-Dateien, beide
  Sprachdateien. Kein inhaltlicher Umbau — der steht in 6, 7a und 7b.

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` — **das deckt den generierten Code bereits ab**: `ci` führt `gen` und `gen:check` aus und bricht bei Drift ab (`Taskfile.yml` Zeilen 64 bis 73). Ein separates `task gen && git diff --exit-code` wäre redundant |
| Migration auf allen eingefrorenen Ständen | neuer Test nach Muster `server/internal/store/frozen_m10_test.go` über `testdata/db/M1a.sql` … `M10.sql`; `PRAGMA foreign_key_check` leer |
| Reihenfolge stimmt | Test «`spotlight` hat Position 0» und «die acht Positionen sind genau 0 bis 7» in `migration00015_test.go`; zusätzlich ein Fall mit von Hand umsortierter und einer ausgeblendeten Reihe (vgl. `frozen_m7_test.go` Zeile 37) |
| Übergang und Singleton | `go test ./internal/store -run FrozenM7` grün: `spotlight_entries` trägt nach der Migration Zeilen beider Medienarten und mindestens einen Bibliothekstreffer; `spotlight_config` hat genau eine Zeile mit `list_key = 'trending'`, `item_count = 20`, `min_hits = 5`, `interval_hours = 24` |
| Vertrag vollständig | `go test ./internal/api -run 'TestContractCoversEveryOperation\|TestContractNoTraktPerProfile\|TestContractRejectsViolations'` plus `go test ./internal/api -run TestEveryRouteHasVisibility`. (`TestContractVisibility` aus der ersten Fassung **existiert nicht**; `-run` meldet einen Fehlgriff nicht, der Befehl wäre grün durchgelaufen und hätte zwei statt drei Dinge geprüft. Die Wächter heissen `TestContractRejectsViolations`, `contract_test.go` Zeile 137, und `TestEveryRouteHasVisibility`, `session_test.go` Zeile 141) |
| Fassung angehoben | `grep -n '^  version:' api/openapi.yaml` zeigt `1.1.0` |
| Drei Orte, eine Wahrheit | Test vergleicht `HomeRowKey`-Enum, `model.HomeRowKeys` und die `CHECK`-Liste der Migration |
| Web-Hälfte grün | `task web:lint` (`tsc --noEmit`), `task web:test` (vitest), `task web:e2e` — alle drei laufen in `task ci` ohnehin, hier einzeln als Beleg, weil sie in der ersten Fassung dieses Plans gar nicht vorkamen |
| Keine alten Schlüssel mehr | `git grep -n "trakt_trending\|trakt_popular\|trakt_lists" -- server web packages api ':!server/migrations' ':!*/dist/*'` ist leer. **Beide Ausschlüsse sind nötig**, und die erste Planfassung hatte keinen davon: `00015_discover.sql` trägt die Zeichenketten in den Zeilen 19, 78, 86, 314 und 324, und eine committete Migration darf nach Regel 5 (`TEMPLATE.sql.txt` Zeile 5) **nicht** angefasst werden; dazu sähe `grep -r` die ignorierten Bauartefakte unter `web/dist/` und `server/internal/webassets/dist/` (`.gitignore` Zeilen 10 und 14). `git grep` lässt ignorierte Dateien von sich aus weg |
| Beide Sprachen | `home.rows.spotlight` und `home.rows.released` stehen in `de.json` **und** `en.json`; `pnpm -C web test locales` prüft Symmetrie und Schweizer Schreibweise (`locales.test.ts` Zeilen 14 bis 19), **nicht** die Existenz; der Nachweis dafür ist der e2e-Titelvergleich (`home.spec.ts` Zeile 37) |
| Commit-Body trennt Mechanik von Inhalt | Der Body nennt die **zwei Hälften** (Go, Web) und trägt die Dateiliste. Bei rund 33 handgeschriebenen Dateien kann ein Reviewer sonst nicht erkennen, was mechanische Umbenennung ist und was Entscheidung — und genau diese Trennung ist die Begründung dafür, dass der Commit so gross sein darf |

### Schritt 2 — Wählbare Liste, Overrides, Mindesttreffer
`feat(server): make the spotlight list configurable and apply its overrides`

`trakt` um die Listenarten aus S-2 erweitern (inklusive Adresse, mit der Watchlist-Ablehnung aus
5.6); `metadata` holt gemäss `spotlight_config` über `ratelimit` (1/s) und ersetzt
`spotlight_entries` in einer Transaktion; die scoped-Abfrage wendet Overrides nach der Ordnungsregel
aus 5.3 an und gibt die Notiz als `ItemRef.note` mit; `TraktMinHits` und die Klemmung auf 20 weichen
`min_hits` und `item_count` (Grenzen nach 3.4). Ein Fehlschlag leert `spotlight_entries` und
protokolliert einen `Error` (I-7, ADR 0009 Zeile 40).

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` |
| Kuratierung überlebt | Test `TestSpotlightOverrideSurvivesReorder`: Liste vollständig umsortiert abgerufen, jeder Override hängt am selben Titel (P4) |
| Listenwechsel | Test: nach Wechsel des `list_key` gilt kein Override der alten Liste |
| Ordnung trifft das Ziel | **«Eintrag 5 nach Platz 1 verschoben steht danach auf Platz 1»** — der Fall, den die erste Planfassung verfehlt hätte (5.3). Dazu: zwei angeheftete Einträge in ihrer Rangfolge; ein angehefteter und ein freier; `pinned_rank` überlebt einen Lauf mit vollständig neuen Positionen |
| Mindesttreffer | Tests bei genau `min_hits`, einem weniger, und `min_hits = 0` |
| Watchlist abgelehnt | Test: Watchlist-Adresse ergibt `422` mit `listUrl: invalid_value` |
| Ausfall kostet nur die Reihe | Test `TestSpotlightOutageLeavesEverythingElse`: nach Fehlschlag fehlt die Reihe; **derselbe Test vergleicht die übrigen sieben Reihen der Antwort Feld für Feld mit dem Lauf davor** (das ist der prüfbare Ersatz für «und nichts anderes ändert sich») |
| Paketgrenzen | `task lint` grün ohne Änderung an `itemsaccess.go` Zeile 28 oder `.golangci.yml` Zeilen 35 bis 41 (`git diff --exit-code` auf beide Dateien) |

### Schritt 3 — Reihe «Kürzlich veröffentlicht»
`feat(server): add the recently released home row`

Abfrage in `server/queries/scoped/items.sql` über `items.release_date`, Fenster 12 Monate (S-5), für
Serien das Datum der neuesten **sichtbaren** Folge (S-6). Der Index `items_released` aus 5.3 Punkt 10 trägt das
Zeitfenster; die Serien-Aggregation läuft je Profil über `visible_items` und ist der teure Teil.

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` |
| `visible_items` ist Pflicht | `go test ./internal/store -run Scoped` (`scoped_test.go` Zeile 35 prüft jede Abfrage in `queries/scoped/`) |
| Analyzer | `task lint` (itemsaccess, depguard) |
| Fälle | Titel ohne `release_date`; genau an der 12-Monats-Grenze; Serie mit unsichtbarer neuester Folge; Serie ohne sichtbare Folge |
| Messung P-11 | **Nach der Methode von `docs/perf/M10.md` Abschnitt 3**, nicht als Go-Benchmark: in `server/internal/visibility` gibt es **keine einzige** `func Benchmark…` (Benchmarks nur in `auth`, `transcode`, `jobs`, `admin`), und `go test -bench` ohne Treffer endet mit Exit 0 — ein Beleg, der grün aussieht und nichts misst. Also: `GET /home` vom Container CT 190 aus (ohne Netzweg), **60 Anfragen nach fünf Aufwärmläufen**, Sitzung mit gewähltem Hauptprofil, Bibliothek vom NAS (~2'500 Filme, ~40'000 Folgen, Entscheid 3); zusätzlich ein Lauf mit einem Kinderprofil. Ziel < 300 ms p95 (M10 mass 105.4 ms). Zweiter Teil von P-11: dieselbe Messung während eines laufenden Erst-Scans. Ergebnis nach `docs/perf/M11.md` |
| Rückfall benannt | Reisst die Messung, fällt die Serien-Aggregation weg: Serien erscheinen dann mit ihrem eigenen `release_date`. Das ist eine Abweichung von S-6 und gehört in diesem Fall ins Protokoll, nicht in einen stillen Umbau |

### Schritt 4 — Admin-Endpunkte und `heroRow`
`feat(server): admin endpoints for spotlight and the hero row of the start page`

Die fünf Operationen aus 5.4 in `admin`, mit Grenzprüfung (Anzahl 5 bis 20, Mindesttreffer 0 bis 10,
Intervall 6 bis 48 Stunden) und `problem+json`; `429` über `spotlight_config.last_manual_refresh`.
`heroRow` in `Home()` setzen (`discover.go` Zeilen 82 bis 102); `FirstPick` (Zeile 156) wird zur
Ableitung daraus; `preload.go` (`heroPicker`, Zeilen 19 bis 22) folgt. `hasTrailer` in den Reihen
`spotlight` und `picks` füllen (D3).

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` |
| Vertrag | `go test ./internal/api -run 'TestContractCoversEveryOperation\|TestContractNoTraktPerProfile'`; jede neue Operation trägt `x-visibility: admin`; keine `admin`-Antwort enthält ein `ItemRef` |
| Fehlende Titel sind sichtbar | `getSpotlightEntries` gibt auch Einträge **ohne** Bibliothekstreffer zurück, als solche markiert — das ist der Beleg für Marcos Antwort auf S-4 («nie auf der Startseite, in der Verwaltung sichtbar»). Dazu: die Liste ist auf 100 Einträge begrenzt und in Abrufreihenfolge sortiert (5.5) |
| Grenzen | Tests: jeder Wert ausserhalb ergibt `422 validation_failed` mit Feldnamen; zweiter Abruf binnen einer Minute ergibt `429`, nach einem Neustart ebenfalls (der Zeitstempel steht in der Datenbank) |
| Bühnenregel | Tests für: Spotlight gefüllt; Spotlight leer und Picks gefüllt; beide leer (`heroRow` fehlt); Spotlight ausgeblendet; Reihenfolge von Marco vertauscht. In jedem Fall zeigt `FirstHero` auf den ersten Titel **der von `heroRow` benannten Reihe** |
| LCP-Pfad | `startPagePreload` (`preload.go` Zeile 54) läuft synchron bei **jedem** `GET /`. Messung von `GET /` (nicht nur `GET /home`) vor und nach dem Umbau nach `docs/perf/M11.md` |

### Schritt 5 — Web: Admin-Seite Spotlight
`feat(web): add the spotlight administration page`

Seite nach Mockup H unter `/admin/spotlight`. Drei Einträge, nicht zwei:

1. `web/src/app/paths.ts` (neben `adminHomeRows`, Zeile 38) — liefert nur die Zeichenkette.
2. **`web/src/app/router.tsx`**: jede Admin-Seite wird dort als
   `createRoute({ getParentRoute: () => adminRoute, … lazyRouteComponent(…) })` angelegt und in
   `adminRoute.addChildren([…])` (Zeile 485) eingehängt. **Ohne diesen Eintrag läuft
   `/admin/spotlight` ins Leere**, und der nach Regel 8 verlangte Tastaturtest kann gar nicht
   starten.
3. `web/src/features/admin/sections.ts` (Gruppe `curation`, neben Zeilen 52 bis 55) — sonst fehlt
   die Karte auf `/admin` (A-2).

Neues Symbol in `web/src/ui/Icon.tsx` (die `paths`-Karte; `IconName` leitet sich daraus ab,
Zeile 154) — alle bestehenden Namen sind vergeben. Dazu der Eintrag im **von Hand gepflegten**
`names: IconName[]`-Array in `web/src/ui/Icon.stories.tsx` Zeile 5 und die neu erzeugte Baseline
`web/e2e-storybook/__screenshots__/ui-icon--all.png` (6.1, Hälfte 3). Status und «Jetzt abrufen»
leben hier. Der Trakt-Schlüssel wird **nie** angezeigt; die
Seite verlinkt nach `/admin/integrations` (`paths.adminIntegrations`, Zeile 42), wo er gepflegt wird
(`AdminSettingsKeys.trakt`, `api/openapi.yaml` Zeile 8694). Gegenrichtung mitnehmen:
`web/src/features/admin/IntegrationsPage.tsx` Zeile 219 verlinkt heute auf `adminHomeRows` und muss
auf die neue Seite zeigen.

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` |
| Tastatur (Regel 8) | neue `web/e2e/admin-spotlight.spec.ts` nach Muster `admin-home-rows.spec.ts`: Pfeiltasten, Tab, Enter, Zurück, sichtbarer Fokus, `expectNoFocusTrap` |
| Texte und Farben | `pnpm -C web lint` (eslint-plugin-i18next, stylelint); `pnpm -C web test locales` (`web/src/locales/locales.test.ts` prüft de/en) |
| Kein Geheimnis | `git diff --name-only -z \| xargs -0r grep -nEi 'client.?id\|api.?key\|bearer\|secret'` ohne Treffer. Die erste Fassung liess `xargs` weg — dann durchsucht `grep` die **Pfadnamen** statt der Dateien und ist immer grün; und das Muster `token` traf jeden Pfad unter `packages/design-tokens` |
| Storybook-Gate | `ui-icon--all.png` im selben Commit neu erzeugt, dazu der Eintrag im von Hand gepflegten `names`-Array (`web/src/ui/Icon.stories.tsx` Zeile 5). Einzeln aufgeführt, obwohl `task ci` ihn umfasst — aus demselben Grund wie `task web:lint` in Schritt 1: ein Gate, der nur im Fliesstext steht, ist nicht abgenommen |

### Schritt 5b — Web: Admin-Seite Cat's Empfehlungen nach Mockup I
`feat(web): add positions, posters and a cross-reference to the picks page`

**Muss nach Schritt 5 laufen, hängt sonst an nichts.** Der Grund ist der Nav-Eintrag: Mockup I zeigt
«Spotlight» neben «Cat's Empfehlungen» in der Admin-Navigation — und die wird aus `adminSections`
**erzeugt** (`web/src/features/admin/AdminLayout.tsx` Zeilen 29 bis 31). Der Eintrag entsteht also
bereits in Schritt 5 und ist hier **keine Arbeit**. Gegenüber den Schritten 6, 7a, 7b und 8 ist 5b
unabhängig: er berührt weder `home_rows` noch `heroRow` noch `HeroCarousel`, und er kann vor oder
nach ihnen liegen.

**Was die Seite heute schon kann** (`web/src/features/admin/PicksPage.tsx`): Tabelle mit Titel
(plus Metazeile), Für, Notiz, Zeitraum und Aktionen (hoch, runter, bearbeiten, entfernen,
Zeilen 392 bis 440), Knopf «Hinzufügen» (Zeilen 503 bis 510), Dialoge zum Anlegen und Ändern,
Live-Bereich für Meldungen. Der Kasten mit der Startseiten-Reihenfolge, den Mockup I nicht mehr
zeigt, **existiert auf der Seite gar nicht** — es gibt nichts zu entfernen.

**Die drei echten Änderungen:**

1. **Spalte «Nr.»** — die Position, die heute nur implizit in der Zeilenreihenfolge steckt.
   `AdminPick.position` liegt bereits vor (`api/openapi.yaml` Zeilen 8120 bis 8122).
2. **Posterbild in der Titelspalte** — siehe die Teilentscheidung unten.
3. **Hinweiszeile unter der Tabelle:** «Wo die Reihe auf der Startseite steht und ob sie erscheint,
   stellst du unter Startseite ein», mit Verweis auf `paths.adminHomeRows`. Das ist die Gegenrichtung
   zu Schritt 6 und hält die Regel aus Abschnitt 1 Punkt 7 aufrecht: Reihenfolge und Ausblenden
   leben **nur** auf der Seite «Startseite».

Der Knopf «Hinzufügen» steht heute links über der Tabelle, in Mockup I oben rechts. Das ist **kein
neues Bauteil, aber auch keine Datei für sich**: `PicksPage.module.css` gibt es nicht. Die Seite
bindet `AdminTablePage.module.css` und `Curation.module.css` ein (`PicksPage.tsx` Zeilen 39 und 40),
und **beide sind geteilt** — unter anderem mit `HomeRowsPage`, die in Schritt 6 ohnehin umgebaut
wird. Entweder entsteht also eine eigene `PicksPage.module.css` für diese eine Regel, oder die
Änderung liegt im geteilten Stylesheet und muss belegen, dass die übrigen Admin-Tabellenseiten
unverändert aussehen. **Vorschlag: eigene Datei**, weil eine Platzierungsregel für genau eine Seite
nichts in einem geteilten Stylesheet verloren hat; die DoD deckt beide Wege ab.

#### Teilentscheidung 5b-a — woher kommt das Posterbild? **(entschieden)**

`AdminPick` ist im Vertrag ausdrücklich «not an `ItemRef`» (`api/openapi.yaml` Zeile 8099) und trägt
**kein** Bild; keine Admin-Seite zeigt heute ein Bibliotheksposter. Der einzige Bildweg
`/items/{id}/images/{kind}/{size}` (Zeile 992) ist `scoped` **und profilgebunden**: `getItemImage`
prüft den Scope (`server/internal/api/item_handlers.go` Zeile 293), und `imageURL` baut die Adresse
mit der Profil-Id (Zeile 376), festgehalten in `TestImageURLsCarryProfileID`
(`server/internal/api/profiles_api_test.go` Zeile 431).

| Option | Pro | Contra |
|---|---|---|
| **(gewählt) eigene Admin-Route, `AdminPick.posterUrl` zeigt darauf** | Die Admin-Tabelle ist **systemweit** (5.5), und das Bild dann auch: jede Zeile hat ihr Poster, unabhängig vom aktiven Profil. Die Admin-Seite macht **keinen** `scoped`-Aufruf, die Trennung aus P1 bleibt scharf | Eine Operation und ein Feld mehr im Vertrag. Beides additiv, die Fassung bleibt 1.1.0 |
| (verworfen) den bestehenden `scoped`-Bildweg rufen | Null Vertragsänderung | Ein Pick für ein Kinderprofil, den das Hauptprofil nicht sehen darf, bekäme **404 und damit kein Poster** — eine Lücke, die wie ein Ladefehler aussieht. Zudem trägt jede solche Adresse die Profil-Id, was auf einer systemweiten Seite schlicht falsch ist |
| (verworfen) gar kein Poster | Nichts zu bauen | Mockup I zeigt eines, und das Poster ist der Grund, warum die Tabelle auf einen Blick lesbar ist |

**Marcos Entscheid (25. Oktober 2026): eigene Admin-Route, Poster immer sichtbar.** Ihm wurden die
drei Wege vorgelegt; er hat die Route gewählt.

**Die Route heisst `GET /admin/items/{id}/images/{kind}/{size}`**, Operation `getAdminItemImage`,
`x-visibility: admin`, systemweit und **ohne** Profilprüfung. Sie hängt am **Titel, nicht am Pick** —
aus zwei Gründen: es gibt bereits eine Familie `/admin/items/{id}/…` (`rating` Zeile 2581, `{id}`
2625, `match` 2654, `refresh` 2697, `trailers` 2726), in die sie sich einfügt; und die nächste
Admin-Seite, die ein Poster zeigen will — die Spotlight-Seite aus Schritt 5, die Pflegewerkzeuge mit
`no_images`, die Titelseite — kann sie ohne Umbau mitbenutzen. Eine Route `/admin/picks/{id}/image`
wäre an die eine Tabelle gefesselt, die sie zuerst braucht.

Nach **CLAUDE.md Regel 6** ist sie ein `http.Handler` mit Beschreibung in `api/openapi.yaml`, weil
sie ein Bild liefert und kein JSON — genau wie ihr `scoped` Gegenstück, dessen Handler die Vorlage
ist. Sie trägt denselben `v`-Parameter (Inhalts-Hash, nur Cache-Busting, Vertrag Zeile 992) und
dieselben Cache-Köpfe, aber **kein** `p`: es gibt kein Profil, an dem der Cache hinge.

`posterUrl` bleibt ein **URL-Feld**, kein `ItemRef`; die Regel aus `api/openapi.yaml` Zeilen 3957
und 3958 («Administration views name titles by id and title; they are never ItemRefs») bleibt
unangetastet.

#### 5b-b — der Weg auf der Serverseite

| Frage | Antwort |
|---|---|
| Welches Paket füllt das Feld? | `admin`. Es darf `systemq` importieren und SQL auf `items` halten — beide Allowlists decken es ab (`itemsaccess.go` Zeile 28, `.golangci.yml` Zeilen 35 bis 41). **Keine Änderung an den Lint-Listen**, wie im ganzen Plan (P2) |
| Woher kommt die Zeile? | Tabelle `images` (`server/migrations/00006_images_formats.sql` Zeilen 22 bis 38: `item_id`, `kind`, `size`, `format`, `rel_path`, `content_hash`, `media_type`, `UNIQUE (item_id, kind, size, format)`) |
| Welche Abfrage? | Für die **Liste** eine neue Batch-Abfrage in `server/queries/system/`, die je Titel genau eine Posterzeile liefert (Grösse `w200`, wie die Posterleiste sie braucht) — die bestehende `ListImagesByItem` (`server/queries/system/metadata.sql` Zeilen 121 bis 123) liefert alle Zeilen eines Titels und ist für eine Tabelle mit vielen Zeilen das falsche Werkzeug. Für den **Handler** eine Einzelabfrage auf `(item_id, kind, size)`, die `rel_path`, `media_type` und `content_hash` zurückgibt |
| Wie wird die Datei geöffnet? | Über `paths`, relativ zur Cache-Wurzel — wie `discover.Image` es heute tut (`server/internal/discover/discover.go` Zeilen 208 bis 241). CLAUDE.md Regel 4 |
| Was, wenn kein Poster da ist? | `posterUrl` fehlt (optionales Feld), die Oberfläche zeigt den Platzhalter. Der Handler antwortet 404 |
| Was trägt die URL? | `?v=<content_hash>` zum Cache-Busting, **kein** `p`. Gebaut von einem `adminImageURL` neben `imageURL` (`server/internal/api/item_handlers.go` Zeile 376) |
| Wo wird sie aufgehängt? | In `binaryRoutes` (`server/internal/api/api.go` Zeilen 47 bis 60), neben dem Eintrag `backupDownloadPath: "admin"` (Zeile 59) — **eine Admin-Binärroute gibt es also schon**, diese hier ist die zweite und folgt einem bestehenden Muster statt Neuland zu betreten. Dazu eine Pfadkonstante nach dem Vorbild von `binaryImagePath` (Zeile 30) beziehungsweise `backupDownloadPath` (`backup_handlers.go` Zeilen 21 und 22). Eingehängt wird sie wie alle anderen über die Schleife in `api.go` Zeile 99, die `binaryRoutes` in die Sichtbarkeitskarte mischt |
| Wird sie offline zwischengespeichert? | **Nein, bewusst nicht.** `web/vite.config.ts` Zeile 144 bindet das `CacheFirst`-Bild-Caching an `^/api/v1/items/[^/]+/images/`; `/api/v1/admin/items/…` passt nicht darauf und fällt unter die `NetworkOnly`-Regel (Zeile 154). Das ist richtig so: die Verwaltung ist kein Offline-Fall (C-1 nennt die Zuschauerseite), und ein Poster im Service-Worker-Cache, das nur der Admin sieht, wäre verschenkter Platz im Quota, das O-7 dem Profil ausweist. **Folge, die benannt sein muss:** `immutable` wirkt für diese Route nur über den HTTP-Cache des Browsers, nicht über den Service Worker — nach einem harten Neuladen wird jedes Poster der Tabelle neu geholt. Bei höchstens einer Handvoll Zeilen ist das belanglos; wird die Admin-Tabelle je lang, ist das die Stelle, an der man nachsieht |

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` |
| Vertrag | `go test ./internal/api -run 'TestContractCoversEveryOperation\|TestContractRejectsViolations\|TestBinaryRoutesMatchContract'`; `posterUrl` ist optional, `getPicks` bleibt `x-visibility: admin` und enthält weiterhin keinen `ItemRef` |
| Eintrag in der Registry | `getAdminItemImage` kommt in `adminRegistry` (`server/internal/api/contract_access_test.go` Zeile 169; die Spotlight-Einträge aus Schritt 1 liegen in denselben Zeilen 239 bis 244). **Form eines Binäreintrags:** `getItemImage` in `contract_profiles_test.go` Zeile 360 ist die Vorlage — `perItem: true`, `okStatus: http.StatusOK` |
| Eintrag in `binaryRoutes` | `server/internal/api/api.go` Zeilen 47 bis 60, mit `"admin"` wie `backupDownloadPath` (Zeile 59), plus Pfadkonstante nach dem Muster `binaryImagePath` (Zeile 30) |
| Route ist systemweit | Test: `GET /admin/items/{id}/images/poster/w200` liefert das Bild auch für einen Titel, den das **aktive** Profil nicht sehen darf (das ist der ganze Zweck); ohne Sitzung 401, mit Nicht-Hauptprofil 403 |
| Keine Profil-Id in der Adresse | `posterUrl` trägt `?v=` und **kein** `&p=`; Gegenstück zu `TestImageURLsCarryProfileID` (`profiles_api_test.go` Zeile 431), das für den scoped Weg das Gegenteil festhält |
| Tastatur (Regel 8) | `web/e2e/admin-picks.spec.ts` deckt die Seite bereits ab (Hinzufügen, Notiz, Verschieben, Zeitfenster, Entfernen, `expectNoFocusTrap`). **Zu ergänzen:** die neue Hinweiszeile ist ein Link und damit eine neue Tab-Station — der Test prüft, dass sie erreichbar ist, `/admin/home-rows` öffnet und den Fokus nicht einfängt. Die Spalte «Nr.» darf die Pfeiltasten-Navigation zwischen den Zeilen (Zeilen 110 bis 139) nicht verschieben |
| Poster steuert keinen Text bei | `admin-picks.spec.ts` Zeile 136 prüft `table.getByRole('rowheader').first()` gegen `^<Titel>`. Die Titelspalte **ist** der Zeilenkopf, das Poster sitzt darin — es trägt deshalb `alt=""` und bleibt für die Rolle unsichtbar, sonst bricht die vorhandene Zusicherung |
| Poster ohne Bild | Test: Pick auf einen Titel ohne Poster zeigt den Platzhalter, nicht ein gebrochenes Bild; der Handler antwortet dort 404 |
| Texte | alle neuen Texte aus `web/src/locales/{de,en}.json` in **beiden** Sprachen (`admin.picks.homeRowsHint`); `pnpm -C web lint` (eslint-plugin-i18next), `pnpm -C web test locales` |
| Keine Farbe hart im Code | `pnpm -C web lint` (stylelint) |
| Geteiltes Stylesheet | Entsteht die Platzierungsregel in `AdminTablePage.module.css` oder `Curation.module.css` statt in einer eigenen Datei, belegt ein Test, dass die übrigen Seiten mit diesen Stylesheets (vor allem `HomeRowsPage`) unverändert aussehen. Mit einer eigenen `PicksPage.module.css` entfällt diese Zeile — dann ist `git diff --name-only -- web/src/features/admin/AdminTablePage.module.css web/src/features/admin/Curation.module.css` leer |
| Kein Storybook-Gate | `web/src/ui` wird **nicht** berührt: die Seite nutzt `DataTable`, `AriaButton` und `TextLink`, wie sie sind, und das Poster bleibt in der Seite. Beleg: `git diff --name-only -- web/src/ui` ist leer. **Entsteht dafür doch ein Bauteil in `web/src/ui`**, verlangt `web/e2e-storybook/components.spec.ts` Zeile 45 sofort eine Story samt Baseline — dann gilt hier dieselbe Zeile wie in den Schritten 5, 7b und 8 |
| Reihenfolge bleibt an einem Ort | Die Seite bietet **keine** Möglichkeit, Reihen zu ordnen oder auszublenden; sie verweist nur. `grep -n "adminHomeRows" web/src/features/admin/PicksPage.tsx` zeigt genau den Verweis |

### Schritt 6 — Web: Admin-Seite Startseite aufräumen
`refactor(web): leave the trakt status to the spotlight page`

**Nur noch der inhaltliche Umbau** — die Schlüsselnamen, Pfade und Fixtures sind seit Schritt 1
mechanisch nachgezogen (6.1, Hälfte 2). Hier geht es um die Seite selbst: `TraktSection`
(`HomeRowsPage.tsx` Zeilen 36 bis 95) schrumpft auf eine Zeile mit Verweis auf die Spotlight-Seite,
die Einbindung (Zeile 250) wird ersetzt, und `traktStatusQuery`
(`web/src/features/admin/queries.ts` Zeile 189) zieht dorthin um. Reihenfolge und Ausblenden bleiben
hier der einzige Ort. `web/e2e/trakt-server.mjs` bekommt die Listen aus S-2 (der Mock liefert heute
nur `trending` und `popular`).

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` |
| Kein toter Pfad | `grep -rn "admin/trakt/refresh\|refreshTrakt" web/src web/e2e` ist leer (seit Schritt 1; hier erneut als Beleg, dass der Umzug nichts wiederbelebt hat) |
| Genau eine Statusfläche | `grep -rln "traktStatusQuery" web/src` nennt genau `web/src/features/admin/queries.ts` und die neue Spotlight-Seite; dazu ein Test in `web/src/features/admin/HomeRowsPage.test.tsx`, der prüft, dass die Seite «Startseite» keine Statustabelle mehr rendert |
| Übersetzungen | `admin.trakt.*` umgezogen oder gelöscht; `pnpm -C web test locales` grün |

### Schritt 7a — Web: Startseite auf die neue Struktur
`feat(web): read the hero row from the server and show spotlight`

Die Startseite liest `heroRow` statt fest nach `picks` zu suchen (`HomePage.tsx` Zeilen 147 bis 149).
Die Bühne bleibt **optisch das heutige Karussell**, egal welche Reihe sie ist; Cat's Empfehlungen
wird, wenn sie nicht Bühne ist, Posterzeile mit Notiz. `rowTitleKey` (Zeilen 36 bis 45) trägt seit
Schritt 1 bereits die neuen Schlüssel — hier kommt nur die **Logik** dazu. Inhaltlich mitzuziehen:
`web/e2e/home.spec.ts` Zeilen 4 und **76 bis 104** (die eigentliche Startseiten-Spec; `discover.ts`
ist nur ihr Helfer) und die Bühnenfälle in `web/src/features/catalog/HomePage.test.tsx`. **Nicht
mehr 73 bis 75**: diese drei Zeilen (`ROW_TITLES.traktTrending`, `.traktPopular`,
`const both = [...]`) sind reine Umbenennung, brechen `tsc` und liegen deshalb in Schritt 1
(6.1, Hälfte 2). Ab 76 stehen die Zusicherungen, die erst mit `heroRow` Sinn ergeben.

Nach diesem Commit sieht Marco die neue Struktur mit echten Daten. Gefällt ihm die Reihenfolge
nicht, schiebt er `picks` auf `/admin/home-rows` nach oben — ohne Code und ohne Revert.

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` |
| Bühne kommt vom Server | Test: Antwort mit `heroRow: 'picks'` zeigt Picks als Bühne, Spotlight als Zeile; Antwort mit `heroRow: 'spotlight'` umgekehrt; Antwort ohne `heroRow` zeigt keine Bühne |
| Ohne Trakt-Schlüssel | `web/e2e/home.spec.ts`: Startseite ohne `SLOUCHCAT_TRAKT_CLIENT_ID` hat eine Bühne (Picks) und keine leere Spotlight-Zeile |
| Prepaint | Test: ist ein Backdrop vorhanden, trägt `#hero-prepaint` dieselbe URL wie das erste Bühnenbild. **Ohne Backdrop liefert `heroPrepaint` `""`** (`preload.go` Zeilen 153 bis 155) und das Karussell fällt auf den Poster zurück (`HeroCarousel.tsx` Zeilen 47 bis 58) — der Test fordert dann nur, dass **kein** Prepaint im Dokument steht |

### Schritt 7b — Web: das Band
`feat(web): show the hero row as a framed band with a poster strip`

Mockup J im bestehenden `HeroCarousel` (S-9): gerahmter Backdrop, Posterleiste statt Punkte,
Pfeilknöpfe mindestens 44 px. `HeroSlide` (Zeilen 14 bis 28) bekommt die Leiste, `HERO_PLAY_KEY`
(Zeile 41) bleibt der Standardfokus. Rein darstellend — **dieser Commit ist einzeln revertierbar**,
und damit ist die umstrittenste Entscheidung des Plans rückbaubar statt nur abschaltbar.

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` |
| Tastatur | `web/e2e/home.spec.ts` erweitert: Pfeiltasten in der Posterleiste, Enter wählt, kein Fokusverlust beim Wechsel (Regel 8, C-4) |
| Bewegung | `prefers-reduced-motion` stoppt den Wechsel (vorhandene Prüfung `HeroCarousel.tsx` Zeilen 44 bis 46, ausgewertet in Zeile 94) |
| LCP (P-10) | **Nach der Methode von `docs/perf/M10.md` Abschnitt 8**, nicht über ein pnpm-Skript: `web/package.json` kennt **kein** `lighthouse`-Skript. Also Lighthouse **12.8.2 als CLI**, Mobile-Voreinstellung, simuliert mit 1.6 Mbit/s, 150 ms RTT und 4× CPU, angemeldet, Server auf `127.0.0.1`, **Median aus drei Läufen**; Ziel < 2.5 s. Hintergrund: der Befund vom 23. Oktober 2026 (`04-entscheidungen.md` Zeile 57) — die letzte Regression kam aus `web/vite.config.ts`, nicht aus der Seite |
| LCP im zweiten Zustand | Dieselben drei Läufe **ohne `SLOUCHCAT_TRAKT_CLIENT_ID`**. Szenario 7.1 beschreibt genau diesen Fall (die Bühne ist dann Picks, nicht Spotlight), und ohne diese Messung bleibt er ungemessen. Der e2e-Fall dafür existiert bereits aus Schritt 7a. Beide Werte nach `docs/perf/M11.md` |
| Storybook-Gate | Die vier Baselines `ui-herocarousel--default.png`, `--focused.png`, `--poster-only.png`, `--with-note.png` sind **im selben Commit** neu erzeugt (`task storybook:test` mit `--update-snapshots`, dann ohne). Dazu eine **neue Story** für die Posterleiste in `web/src/ui/HeroCarousel.stories.tsx` (nach dem Muster der Zeilen 44 bis 53) samt ihrer Baseline — ohne Story deckt der Gate die neue Fläche nicht ab (`components.spec.ts` Zeilen 45 und 61) |
| Revertierbar | `git revert --no-commit <7b> && task ci` grün, danach `git revert --abort` (Beleg im Commit-Body). **Die Zusage gilt nur, weil die Baselines Teil desselben Commits sind** — lägen sie daneben, nähme der Revert den Code zurück und liesse die Bilder stehen, und `task ci` wäre rot |

### Schritt 8 — Trailer
`feat(web): add a trailer button next to play and details`

Knopf «Trailer» neben «Abspielen» und «Details» an der Bühne und an der Cat's-Zeile, nur wenn
`hasTrailer` gesetzt **und** wahr ist. Öffnet `TrailerPanel` (Zeilen 30 bis 62) mit `itemId` und
`title`; der Hinweis `trailer.privacy` (Zeile 57) bleibt.

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` |
| Drei Fälle | `hasTrailer: true` (Knopf da), `hasTrailer: false` (kein Knopf), Feld fehlt (kein Knopf) |
| Tastatur | Abspielen → Details → Trailer → Dialog → Escape zurück auf den Knopf. **Die Pfeiltasten hängen an den äusseren Knöpfen**: `onArrowPress={pageBack}` auf Abspielen, `pageForward` auf Details (`HeroCarousel.tsx` Zeilen 206 bis 225). Ein dritter Knopf verschiebt diese Verdrahtung — der Test deckt ab, dass Pfeil links und rechts weiterhin durch die Bühne blättern (C-4, Regel 8) |
| Storybook-Gate | Alle **fünf** `ui-herocarousel--*.png` (die vier bestehenden plus die Posterleisten-Story aus 7b) im selben Commit neu erzeugt. Schritt 8 ändert `web/src/ui`, nicht nur die Startseite: `HeroSlide` bekommt ein Feld, `HeroCarouselProps` ein Label (6.1, Hälfte 3) |
| Messung | Wie in Schritt 3, nach `docs/perf/M10.md` Abschnitt 3 (60 Anfragen nach fünf Aufwärmläufen, aus dem Container): `GET /home` **und** `POST /search` «Se», je einmal mit und einmal ohne das `EXISTS` auf `trailers`. Vergleichswerte aus M10: 105.4 ms und 54.2 ms p95. Differenz nach `docs/perf/M11.md`. Unter 5 ms in der Suche wird die Ausstiegsklausel aus 3.6 als Folge-Commit notiert, **nicht** hier mitgemacht |

### Schritt 9 — Abnahme und Doku
`docs(dev): record the spotlight acceptance evidence`

`docs/dev/CURRENT_STATUS.md` nachführen (auch den überholten Commit-Stand `6f78a5c`); die
Abnahmezeilen zu E-1, E-2, E-7, E-9, I-7, W-10, A-2 und C-5 sowie das Plan-Kriterium **P-10** (neue
Formulierung ohne «Karussell», `03-plan-v1.md` Zeile 613) mit Belegen oder erklärten Abweichungen
versehen. Messwerte nach `docs/perf/M11.md`.

**DoD**

| Prüfung | Befehl / Beleg |
|---|---|
| Alles grün | `task ci` auf dem Branch |
| Belege vollständig | je DoD-Zeile der Schritte 1 bis 8 **einschliesslich 5b** ein Befehl mit Ausgabe |
| Mockup I abgenommen | Die Seite «Cat's Empfehlungen» zeigt Nr., Poster und die Hinweiszeile; Marco vergleicht sie am laufenden Container mit Mockup I (S-11) |
| Keine Platzhalter | `git diff main... --name-only --diff-filter=d -z -- server web packages api \| xargs -0r grep -nE 'TODO\|FIXME\|test\.skip\|\.only\(\|t\.Skip'` ohne Treffer (CLAUDE.md «Arbeitsweise»). Drei Zusätze, jeder gegen einen Fehlalarm: **`--diff-filter=d`**, weil Schritt 1 `server/queries/system/trakt.sql` löscht und `grep` über eine gelöschte Datei abbricht; **`-- server web packages api`**, weil die Muster als Zeichenketten in **dieser Planzeile** stehen und `docs/` sonst einen Treffer in der eigenen Datei meldet, sobald Schritt 0 sie in den Branch legt; **`-z | xargs -0r`**, weil `xargs grep` bei leerer Dateiliste sonst auf stdin blockiert |
| Abnahme | Marco nimmt am laufenden Container ab; nicht pushen, bevor er es sagt |

---

## 7. Pre-mortem (DELIBERATE, genau drei Szenarien)

### 7.1 Szenario 1 — «Die Startseite ist kopflos»

**Was schiefgegangen ist.** Spotlight steht oben, ist aber leer: der Trakt-Schlüssel fehlt nach einem
Umzug der `deploy/.env`, oder die Liste überschneidet sich mit der Bibliothek unter dem
Mindesttreffer. Cat's Empfehlungen ist nur noch eine Posterzeile. Die Seite beginnt ohne Bild und
wirkt schlechter als vorher. Der LCP misst gegen eine Posterzeile und springt je nach Zustand.

**Frühwarnzeichen.** Eine Installation ohne `SLOUCHCAT_TRAKT_CLIENT_ID` zeigt keine Bühne.
Lighthouse findet kein Hero-Element. Ein Review-Befund lautet «sieht bei mir anders aus als im
Mockup». Im Dashboard steht ein Trakt-Fehler, den niemand mit der Startseite verbindet.

**Gegenmittel.** Die Bühnenregel ist serverseitig und steht als `heroRow` im Vertrag (3.5): ist
Spotlight leer, rückt Picks nach und wird Band. Der Fall «ohne Trakt-Schlüssel» ist Pflichttest in
Schritt 4 (Server) und Schritt 7a (e2e). Die Spotlight-Seite nennt sichtbar den Grund, warum die
Reihe gerade fehlt.

### 7.2 Szenario 2 — «Die Notiz steht am falschen Film»

**Was schiefgegangen ist.** Marco hat sechs Einträge mit Notizen versehen, dann die Liste von
«Trending» auf «Kinocharts» umgestellt. Notizen tauchen an fremden Titeln auf oder verschwinden.
Oder: eine Liste liefert denselben TMDB-Titel als Film und als Serie, und das Ausblenden trifft
beide. Das Vertrauen in die Kuratierung ist weg — und Kuratierung ist das Alleinstellungsmerkmal.

**Frühwarnzeichen.** Ein Test, der nur eine Liste kennt, bleibt grün, während die Verwaltung nach
einem Wechsel verwaiste Zeilen zeigt. Marco meldet «die Notiz war vorher woanders». In
`spotlight_overrides` stehen mehr Zeilen, als die Liste Einträge hat.

**Gegenmittel.** `list_key` **und** `media` gehören in den Primärschlüssel (5.3); das ist die Lehre
aus `trakt_lists.PRIMARY KEY (list, media, position)`. In Schritt 2 sind zwei Tests Pflicht:
vollständig umsortierte Liste und Listenwechsel. `getSpotlightEntries` weist verwaiste Overrides
ausdrücklich aus und macht sie löschbar.

### 7.3 Szenario 3 — «Der Vertrag bricht still für alles ausserhalb der Web-App»

**Was schiefgegangen ist.** `HomeRowKey` hat zwei Werte verloren, `TraktStatus` zwei Felder, und
`HomePage` ein Feld gewonnen. Ein Fremd-Client — oder der eigene Prüf-Client `cmd/apicheck`, der
Slouchcat nur über `api/openapi.yaml` kennt (`.golangci.yml` Zeilen 42 bis 47) — kennt
`trakt_trending` noch, bekommt es nie mehr und zeigt eine leere Seite statt eines Fehlers. C-5 ist
formal erfüllt und praktisch verletzt.

**Frühwarnzeichen.** `task gen` erzeugt einen grossen Diff im generierten Client, ohne dass jemand
die Fassungsnummer anfasst. `TestContractCoversEveryOperation` ist grün, weil es Operationen zählt,
nicht Enum-Werte. Die Release-Notes nennen die Änderung nicht — derselbe Fehlertyp wie beim
1.0-Release (`CURRENT_STATUS.md`, zweiter Fehler im Release-Weg, PR #32).

**Gegenmittel.** `info.version` geht in **Schritt 1** auf `1.1.0`, im selben Commit wie das Enum.
Entfernte Werte werden nie wiederverwendet (Satz im ADR 0011). Der Gleichstandstest aus Schritt 1
vergleicht Vertrag, `model.HomeRowKeys` und die `CHECK`-Liste. Schritt 9 trägt die brechende
Änderung in die Release-Notes des Tags ein — genau die Stelle, die beim 1.0-Release falsch gelesen
wurde.

---

## 8. Erweiterter Testplan (DELIBERATE)

### 8.1 Unit

| Gegenstand | Fälle |
|---|---|
| Abbildung Trakt → Bibliothek | Treffer über `tmdb_id`; Eintrag ohne `tmdb_id`; Eintrag ohne Bibliothekstreffer; derselbe TMDB-Titel als Film und als Serie |
| Overrides | `hidden` filtert; `pinned_rank` ordnet nach der Regel aus 5.3, **und Platz 1 heisst Platz 1** (Eintrag 5 nach vorn gezogen); zwei angeheftete Einträge in Rangfolge; angeheftet neben frei; `note` reist als `ItemRef.note`; Override nach vollständig umsortierter Liste; Override nach Listenwechsel; verwaister Override |
| Mindesttreffer | genau `min_hits`; einer weniger; `min_hits = 0` |
| Anzahl | `item_count` grösser als die Trefferzahl; an den Grenzen 5 und 20; 21 ergibt `422` (3.4) |
| Veraltung | Eintrag älter als `2 × interval_hours` fällt weg; Intervall 6 h und 48 h je ein Fall (S-8, ersetzt die Konstante `TraktMaxAge`) |
| Watchlist-Ablehnung | `/users/<name>/watchlist` ergibt `422 listUrl: invalid_value` (5.6, Regel 11) |
| Minutensperre | zweiter `refreshSpotlight` binnen einer Minute ergibt `429`, auch nach einem Neustart (`last_manual_refresh`) |
| `heroRow` | Spotlight gefüllt; Spotlight leer und Picks gefüllt; beide leer (Feld fehlt); Spotlight ausgeblendet; Reihenfolge vertauscht. `FirstHero` stimmt in jedem Fall mit `heroRow` überein |
| «Kürzlich veröffentlicht» | ohne `release_date`; genau an der 12-Monats-Grenze; Serie mit unsichtbarer neuester Folge; Serie ohne sichtbare Folge |
| Migration 00020 | Standardreihenfolge; von Hand umsortierte `home_rows`; ausgeblendete Trakt-Reihen (Verlust ist erwartet); `spotlight` auf Position 0; Positionen genau 0 bis 7 |
| Enum-Gleichstand | Vertrag, `model.HomeRowKeys`, `CHECK`-Liste stimmen überein (7.3) |

### 8.2 Integration

- Migration über `testdata/db/M1a.sql` bis `M10.sql`, danach `PRAGMA foreign_key_check` leer
  (Muster `server/internal/store/frozen_m10_test.go`).
- **`server/internal/store/migration00015_test.go`**: Zeile 64 (`picks` auf Position 0) wird zu
  `spotlight` auf Position 0; Zeile 67 (Tabellenliste mit `trakt_lists`) wird zu
  `spotlight_config`, `spotlight_entries`, `spotlight_overrides`.
- **`server/internal/store/frozen_m7_test.go`**: die drei Zusicherungen auf `trakt_lists`
  (Zeilen 39 bis 41) werden zu Zusicherungen auf `spotlight_entries` — beide Medienarten vorhanden,
  mindestens ein Eintrag mit Bibliothekstreffer. Der M7-Beleg bleibt erhalten, **weil 5.3 Punkt 8
  die Trending-Zeilen übernimmt**; ohne diesen Übergang liefe der Test gegen eine leere Tabelle.
- **Fixtures und Kompilat:** `server/internal/discover/discover_test.go` Zeilen 195, 197, 233 und
  `server/internal/visibility/discover_test.go` Zeilen 268, 270, 338 schreiben heute
  `INSERT INTO trakt_lists` und wechseln auf `spotlight_entries`;
  `server/internal/metadata/trakt_test.go` Zeile 106 liest daraus;
  `server/internal/admin/trakt_test.go` Zeile 50 (`TraktStatus{Trending, Popular}`) und
  `server/internal/admin/curation_test.go` Zeile 302 (`model.RowTrakt…`) brechen beim **Übersetzen**
  und fallen deshalb erst `task test` auf, nicht `sqlc`.
- `server/internal/store/scoped_test.go` Zeile 35 prüft, dass jede neue scoped-Abfrage
  `visible_items` joint — gilt für `spotlight.sql` und die neue Abfrage in `items.sql`.
- Abruf gegen `web/e2e/trakt-server.mjs` beziehungsweise den Go-Mock: Erfolg, HTTP 500,
  Zeitüberschreitung, leere Liste, fehlender Schlüssel. Nach jedem Fehlschlag ist
  `spotlight_entries` leer, die Reihe fehlt, die übrigen sieben Reihen sind Feld für Feld
  unverändert.
- `ratelimit` greift: zwei Abrufe im selben Lauf liegen mindestens eine Sekunde auseinander.
- Vertragstests: `TestContractCoversEveryOperation`
  (`server/internal/api/contract_access_test.go` Zeile 313) — die Karte in den Zeilen 239 bis 244
  verliert `refreshTrakt` und bekommt die fünf neuen Operationen; `TestContractNoTraktPerProfile`
  (`contract_test.go` Zeile 269) mit der erweiterten Allowlist; `TestContractRejectsViolations`
  (`contract_test.go` Zeile 137); `TestEveryRouteHasVisibility` (`session_test.go` Zeile 141);
  `contract_curation_test.go` Zeilen 293, 331 und 344 auf die neuen Schlüssel.
- Sichtbarkeit: Kinderprofil unter der halben Spotlight-Liste — Reihe fehlt für dieses Profil,
  erscheint für das Hauptprofil, Admin-Zahl unverändert (K-3, 5.5).
- Startseiten-API gegen P-11 mit und ohne Spotlight, auch während eines Scans; zusätzlich `GET /`
  wegen `startPagePreload` (`preload.go` Zeile 54).

### 8.3 e2e / Playwright und der Storybook-Gate

**Zwei Playwright-Läufe, nicht einer.** `task ci` fährt `e2e` (`web/e2e`) **und** `storybook:test`
(`web/e2e-storybook`, `Taskfile.yml` Zeile 73). Der zweite vergleicht je Story ein committetes Bild
und verlangt zu jeder Komponente in `web/src/ui` eine Story (`components.spec.ts` Zeilen 45 und 61).
Betroffen sind Schritt 5 (`ui-icon--all.png` plus Eintrag im `names`-Array) und Schritt 7b (vier
`ui-herocarousel--*.png` plus eine neue Story für die Posterleiste); beides steht in der jeweiligen
DoD. Schritt 1 berührt ihn nicht — keine Story nennt die Reihenschlüssel.

| Spec | Inhalt |
|---|---|
| `web/e2e/admin-spotlight.spec.ts` (neu) | Tastaturtest nach Regel 8. Liste wählen, speichern, «Jetzt abrufen», Eintrag ausblenden, verschieben, Notiz setzen und löschen, verwaisten Override sehen und löschen |
| `web/e2e/admin-home-rows.spec.ts` | Neue acht Reihen; Trakt-Block nur noch ein Verweis; Zeilen 119 und 133 auf die neuen Pfade |
| `web/e2e/admin-home.spec.ts` | Karte «Spotlight» in der Gruppe `curation` (A-2); Zeilen 2, 174, 187, 191 auf `refreshSpotlight` |
| `web/e2e/admin-picks.spec.ts` (Schritt 5b) | Die Spec deckt die Seite heute ab (Hinzufügen, Notiz, Verschieben, Zeitfenster, Entfernen, `expectNoFocusTrap`). Zu ergänzen: die neue Hinweiszeile als Tab-Station, die `/admin/home-rows` öffnet; die Spalte «Nr.» darf die Pfeiltasten-Navigation zwischen den Zeilen (Zeilen 110 bis 139) nicht verschieben |
| `web/e2e/home.spec.ts` | **Die Startseiten-Spec.** Zeilen 73 bis 75 sind Umbenennung und liegen in Schritt 1; Zeilen 4 und **76 bis 104** werden in 7a und 7b umgeschrieben: Bühne nach `heroRow`, Posterleiste mit Pfeiltasten, Cat's Empfehlungen als Zeile mit Notiz, Trailer-Knopf vorhanden und fehlend, Startseite **ohne** Trakt-Schlüssel (7.1), `prefers-reduced-motion` |
| `web/e2e/discover.ts` | Helfer: `ROW_TITLES` (Zeilen 22 und 23), `refreshTrakt` → `refreshSpotlight` (Zeilen 131 und 132) |
| `web/e2e/curation.ts` | Schlüsselliste Zeilen 41 und 42 |
| `web/e2e/trakt-server.mjs` | Mock um die Listen aus S-2 erweitern |
| `web/src/features/admin/HomeRowsPage.test.tsx` | Zeilen 14 **und 15**, 89 **und 90** (die Schlüssel stehen paarweise) sowie 46, 104 bis 127 (Pfade) |
| `web/src/features/catalog/HomePage.test.tsx` | Zeilen 299 **und 300** (beide Trakt-Reihen in den Testdaten) |
| `web/src/locales/de.json`, `web/src/locales/en.json` | Zeilen 253 und 254 raus, `home.rows.spotlight` und `home.rows.released` rein — in beiden Sprachen |

### 8.4 Beobachtbarkeit und Protokolle

- Ein fehlgeschlagener Abruf erzeugt genau einen `Error` über den Redaction-Handler aus
  `internal/logging`, landet im `ErrorRing` und damit im Dashboard (ADR 0009 Zeile 40).
  Erfolgreiche Läufe auf `Info` mit `list_key`, Anzahl und Dauer.
- **Kein Geheimnis im Protokoll** (Regel 10, A-9): der Trakt-Client-Schlüssel nie in Meldung oder
  URL. **`list_url` wird nicht protokolliert** — sie kann einen Benutzernamen enthalten; in die
  Meldung geht nur `list_key`, bei `list_key = 'url'` zusätzlich die ersten acht Zeichen eines
  SHA-256 der Adresse als Unterscheidungsmerkmal. `listurl` kommt in die Redaction-Liste und in
  deren Test.
- Der E2E-Lauf scannt das Server-Log auf jedes Fake-Secret des Laufs, wie bisher.
- Messwerte in `docs/perf/M11.md`: LCP der Startseite (P-10), `GET /home` **und** `GET /` p95
  (P-11), Dauer eines Abruflaufs, Mehraufwand des `EXISTS` auf `trailers` in `GET /home` und in der
  Suche. Der Name folgt der Regel «Messwerte nach `docs/perf/<Mx>.md`»; dieser Plan ist
  Meilenstein **M11** nach M0 bis M10.

---

## 9. Risiken und Rückfallweg

### 9.1 Risikotabelle

| Nr. | Risiko | Wirkung | Gegenmittel |
|---|---|---|---|
| R-1 | Overrides hängen an der Position oder an der falschen Liste | Notiz und Ausblenden wandern auf fremde Titel | Schlüssel `(list_key, media, tmdb_id)`; zwei Pflichttests (7.2) |
| R-2 | Acht-Reihen-Grenze | Eine neunte Reihe bricht `CHECK`, `minItems`/`maxItems`, `position maximum` und `MaxRows` gleichzeitig | Genau zwei raus, zwei rein; Gleichstandstest. Eine neunte Reihe ist ein eigener Plan |
| R-3 | Zwei Implementierungen der Bühnenregel | Server malt ein Bild vor, das der Client nicht als Bühne zeigt; Blitzer, LCP-Regression | `heroRow` im Vertrag; `FirstHero` ist Ableitung, keine zweite Regel (3.5) |
| R-4 | Startseite ohne Bühne | Die sichtbarste Fläche wirkt leer | Bühnenregel plus Pflichtfall «ohne Trakt-Schlüssel» in zwei Testebenen (7.1) |
| R-5 | **Bestehende Tests und Typen brechen durch das Schema** — auf **beiden** Seiten des Codegen | `task ci` rot an einer Commit-Grenze; schlimmer: jemand schneidet den Commit kleiner und der Branch ist mittendrin unbaubar | Schnitt nach Baubarkeit über die ganze Kette (6.1, beide Hälften): neun Go-Testdateien und die mechanische Web-Anpassung liegen **im selben Commit**. Der M7-Beleg wandert mit, statt zu verschwinden. Die gefährlichste Klasse sind die Stellen, die **keine** Suche nach `trakt_lists` oder nach Go-Konstanten findet: String-Literale in den API-Vertragstests und der `satisfies Record<HomeRowKey, …>` in `HomeRowsPage.tsx` Zeile 26 |
| R-14 | **Spotlight und Cat's Empfehlungen zeigen denselben Titel** | Derselbe Film steht als erster Slide des Bands und direkt darunter als erste Karte der Cat's-Zeile. Das ist wahrscheinlich, nicht theoretisch: Marco empfiehlt, was gerade trendet | Heute dedupliziert `ListTraktRow` nur **innerhalb** einer Reihe (`visibility/discover.go` Zeile 279); über Reihen hinweg gibt es keine Entdopplung, und eine einzuführen wäre ein Eingriff in `discover`, der jede Reihe beträfe. Der Plan nimmt die Überschneidung darum **in Kauf** und benennt sie — wie bei R-7. Gegenmittel, falls sie stört: die Bühne lässt ihren eigenen Titel aus der Zeile darunter weg; das ist eine Zeile in `HomePage.tsx` und gehört dann in einen eigenen Commit, nicht in 7a |
| R-13 | **Vertragsbeschreibungen bleiben im alten Zustand** | Falsche Prosa fährt still in Fassung 1.1.0; ein Fremd-Client liest «die ersten 20 Titel zweier Trakt-Listen» und bekommt etwas anderes (C-5) | Kein Test fängt Prosa (5.4). Die drei Stellen (`openapi.yaml` 942-947, 3915-3918, 3485) stehen darum namentlich in der Vertragstabelle und in der DoD von Schritt 1 |
| R-6 | Schema-Test `TestContractNoTraktPerProfile` verbietet die neuen Namen | Schritt 1 ist rot, obwohl nichts falsch ist | Allowlist (Zeilen 286, 287, 297) und Kopfkommentar (265 bis 268) im selben Commit erweitern; die I-7-Absicht (Zeilen 282 bis 285) bleibt unangetastet (5.4) |
| R-7 | «Kürzlich veröffentlicht» doppelt zu «Neu hinzugefügt» | Zwei Reihen mit denselben Titeln | Überlappung ist entschieden (S-10). Die Ausschluss-Variante ist **verworfen**: beim Erst-Scan ist alles neu hinzugefügt, `released` wäre dann leer — ausgerechnet beim ersten Eindruck. Rückfall, falls es stört: `recent` auf 30 Tage `added_at` begrenzen; das trennt ohne Abhängigkeit zwischen den Reihen |
| R-8 | Messung von Schritt 3 reisst (P-11) | Startseite über 300 ms p95 | Index `items_released` (5.3); reicht er nicht, fällt die Serien-Aggregation weg und Serien erscheinen mit eigenem `release_date` — als erklärte Abweichung von S-6 im Protokoll, nicht still |
| R-9 | Ratelimit bei kurzem Intervall oder häufigem «Jetzt abrufen» | Abruf schlägt fehl, Reihe verschwindet | Intervall mindestens 6 Stunden, Minutensperre über `last_manual_refresh`, Abruf über `ratelimit` (1/s), Fehler im Status sichtbar |
| R-10 | Vertragsbruch für Fremd-Clients | C-5 praktisch verletzt | Fassung 1.1.0 im selben Commit wie das Enum; entfernte Werte nie wiederverwenden; Release-Notes (7.3) |
| R-11 | `hidden` der zwei entfallenden Reihen geht verloren | Marco sieht eine Reihe, die er ausgeblendet hatte | Benannt in der DoD von Schritt 1 und im Abnahmetext; zwei Klicks zum Wiederausblenden |
| R-12 | Der Trakt-Schlüssel landet doch in der Oberfläche oder im Log | Regel 10 und A-9 verletzt | Seite verlinkt nur nach `/admin/integrations`; `grep`-Beleg (Schritt 5); `list_url` nicht protokollieren (8.4) |

### 9.2 Rückfallweg ohne Down-Migration

Keine Down-Migration (Regel 5). Vier Stufen, von der billigsten zur teuersten:

1. **Ein Klick — die Bühne tauschen.** Auf `/admin/home-rows` `picks` nach oben schieben. Die
   Startseite sieht aus wie vor dem Umbau (C2). Deckt jeden Fall ab, in dem Spotlight inhaltlich
   enttäuscht.
2. **Ein Klick — die Reihe abschalten.** `spotlight_config.enabled = 0` oder die Reihe ausblenden.
3. **Ein Revert — das Band zurücknehmen.** Schritt 7b ist rein darstellend und einzeln
   revertierbar; die Bühne fällt auf das heutige Karussell zurück, alles andere bleibt. Das ist der
   Grund für den Schnitt 7a/7b.

   **7a ist dagegen kein Revert-Kandidat.** Schritt 4 stellt `preload.go` auf `FirstHero` aus
   `heroRow` um; ein `git revert` von 7a allein liesse den Server das Spotlight-Bild vormalen,
   während der Client wieder fest nach `picks` sucht — **genau R-3, herbeigeführt durch den
   Rückbau**. Der Rückfall für ein fehlerhaftes `heroRow` ist darum eine Vorwärtskorrektur
   (`00021_…` beziehungsweise ein Fix in `discover`), niemals ein Revert von 7a. Wer 7a
   zurücknimmt, nimmt Schritt 4 mit.
4. **Vorwärts korrigieren oder zurück auf 1.0.** Ein Schemafehler wird mit `00021_…` behoben. Ist
   die Migration selbst schädlich: (a) Container auf `ghcr.io/slouchcat/slouchcat:1.0.0`
   zurückstellen, **nicht** starten, solange die Datenbank auf Schema 00020 steht; (b) die
   automatische Kopie `pre-migration-*` aus `<config>/backups/` über die Wiederherstellung aus A-4
   einspielen (ADR 0009 Zeile 42: prüfen, `slouchcat.db.restore` bereitlegen, geordnet herunter,
   mit `exec` ersetzen, vorherige Datenbank als `pre-restore-*`); (c) 1.0.0 starten. Verloren sind
   Fortschritte und Schauzeiten seit der Migration — darum gehört der Rückbau in dieselbe Sitzung
   wie das Update.

---

## 10. ADR 0011 — Spotlight als wählbare Trakt-Liste auf der Startseite

> Vorlage für `docs/adr/0011-spotlight.md` (Schritt 0). Status dort beim Anlegen: «vorgeschlagen»;
> auf «angenommen» mit Marcos Freigabe.

**Betroffene Spec-IDs:** E-1, E-2, E-7, E-9, I-7, W-10, A-2, C-5, K-3.
**Löst teilweise ab:** ADR 0008, Zeile 40 (Karussell nur mit Picks) und Zeile 42 (vier feste Listen,
höchstens 20, ausgeblendet unter 5).

### Decision

Die Startseite bekommt den Schlüssel `spotlight`: eine in der Verwaltung wählbare öffentliche
Trakt-Liste, auf die Bibliothek gefiltert, als Band mit Backdrop und Posterleiste. `trakt_trending`
und `trakt_popular` entfallen; `released` («Kürzlich veröffentlicht» nach `items.release_date`,
mit eigenem Index) kommt dazu. Es bleiben genau acht Schlüssel. Der Cache der abgerufenen Liste
liegt in `spotlight_entries` und wird je Lauf ersetzt; die Kuratierung einzelner Einträge liegt
getrennt davon in `spotlight_overrides` mit dem Schlüssel `(list_key, media, tmdb_id)` und einer
ausgeschriebenen Ordnungsregel. **Welche Reihe die Bühne ist, entscheidet der Server und sagt es im
Vertrag: `HomePage.heroRow` ist die erste nicht leere bühnenfähige Reihe (`spotlight`, `picks`) in
der gespeicherten Reihenfolge**; `FirstHero` und das Prepaint sind Ableitungen daraus, keine zweite
Implementierung. Cat's Empfehlungen bleibt inhaltlich unverändert (E-2) und wird Posterzeile mit
Notiz, sobald sie nicht Bühne ist. Anzahl und Mindesttreffer werden einstellbar, die Anzahl jedoch
nur **bis 20**: `visibility.MaxRowItems` bleibt unangetastet. Der Trailer reist als optionales
`ItemRef.hasTrailer`, gesetzt nur in den Reihen `spotlight` und `picks`. Die Vertragsfassung steigt
auf 1.1.0. Es entsteht kein neues Go-Paket.

### Drivers

1. Die Startseite behält immer einen Kopf und bleibt unter P-10 und P-11. Gewicht 0.40.
2. Kuratierung bleibt verlässlich: eine Notiz steht nach dem Abruf und nach einem Listenwechsel noch
   am selben Titel. Gewicht 0.35.
3. Der Vertrag bleibt ehrlich und prüfbar; eine Regel hat genau eine Implementierung. Gewicht 0.25.

### Alternatives considered

| Alternative | Warum nicht |
|---|---|
| Beide Trakt-Schlüssel umwidmen (A2) | `trakt_popular` für eine Reihe nach `release_date` ist eine dauerhafte Falschaussage in Schema, Vertrag, Code und — seit `heroRow` — in der öffentlichen Antwort |
| Schlüssel lassen, nur den Inhalt konfigurierbar machen (A3) | Die ernsthafteste Alternative: kein Enum-Bruch, kein Rebuild, keine Bühnenregel. Sie scheitert an einer Kette: «Kürzlich veröffentlicht» braucht einen Schlüssel, ein neunter ist durch E-9 versperrt (A4), der Schlüssel muss aus den zwei Trakt-Zeilen kommen — damit trägt A3 den Kern von A2 in sich. **Sie gewinnt**, wenn S-1 «nein» **und** S-5 «keine neue Reihe» lautet; dann wird der Plan neu geschnitten, nicht nachgebessert |
| A3 plus einen neunten Schlüssel, Enum erweitert statt getauscht (A4) | Wäre für Fremd-Clients deutlich milder: ein hinzugefügter Wert bricht niemanden, zwei entfernte schon. Verworfen **nicht** aus technischer Not — dass es genau acht sind, steht in `minItems: 8` (`api/openapi.yaml` Zeile 3887) und ist eine M7-Entscheidung, die Spec sagt «höchstens acht» — sondern wegen **E-9** (Spec Zeile 122) und **Driver 1**: eine neunte Reihe heisst mehr über der Faltkante und eine grössere Antwort bei jedem Startseitenabruf. Dazu bliebe der Schlüsselname falsch (A3-Erbe) |
| `hidden`, `pinned_rank`, `note` als Spalten im Cache | Der Cache wird je Lauf ersetzt; die Kuratierung müsste jedes Mal hinübergerettet werden — die Mechanik, die bei `trakt_lists.PRIMARY KEY (list, media, position)` zum Wandern führt |
| Overrides am Bibliothekstitel `item_id` | Ein Eintrag ohne Bibliothekstreffer liesse sich nicht ausblenden; genau das verlangt Mockup H |
| Karussell bleibt dauerhaft oben (C2) | Zwei grosse Bühnen übereinander, gegen E-9. **Bleibt trotzdem erreichbar**: die Bühne folgt der gespeicherten Reihenfolge, also einem Klick |
| `FirstHero` liefert nur das Bild, der Client entscheidet die Bühne | Die Regel stünde nirgends im Vertrag, ein Fremd-Client könnte die Startseite nicht nachbauen (C-5), und zwei Implementierungen derselben Regel sind genau R-3 |
| `visibility.MaxRowItems` anheben, damit Spotlight 30 zeigt | Eine gemeinsame Klemme für sechs weitere Reihen wegen einer einzigen anheben: mehr Poster über der Faltkante und grössere Antworten überall, für einen Nutzen, den niemand verlangt hat |
| `ItemRef.hasTrailer` überall füllen (D1) | Ein `EXISTS` auf `trailers` in jeder Liste und in der gepagten Suche. Bleibt als Ausstiegsklausel nach einer Messung |
| Trailer-Knopf ohne Flag (D2) | Ein toter Knopf an der sichtbarsten Stelle der App |
| `released` schliesst aus, was in `recent` steht | Beim Erst-Scan ist alles neu hinzugefügt; die Reihe wäre ausgerechnet beim ersten Eindruck leer |
| Ein eigenes Paket `spotlight` mit `systemq` | Die Allowlists müssten wachsen; Regel 2 bliebe nicht unberührt. ADR 0008 hat dieselbe Alternative schon verworfen |
| Trakt-Konto je Profil, Scrobbling, Watchlist-Import | Non-Goal (Spec Zeile 61, Entscheid 9); die Watchlist-Adresse wird serverseitig abgelehnt |
| Schalter «auch Titel ohne Bibliothekstreffer zeigen» | Non-Goal «eigene Wunschverwaltung» (Spec Zeile 63) |
| Mockup I in einen eigenen Plan verschieben | War der Vorschlag zu S-11 und ist **von Marco verworfen** (25. Oktober 2026): die Admin-Seite wird im selben Zug umgebaut, als Schritt 5b. Das Argument dagegen hielt nur, solange 5b gross wäre — er ist es nicht, weil der Nav-Eintrag aus Schritt 5 fällt und E-2 unverändert bleibt |

### Why chosen

Die Kombination hält alle drei Driver gleichzeitig. `heroRow` im Vertrag macht die Startseite gegen
einen leeren Spotlight immun **und** beseitigt die zweite Implementierung, die R-3 fürchtet
(Driver 1 und 3 in einem Zug). Der getrennte Override-Schlüssel mit Listenidentität ist die einzige
Variante, bei der Kuratierung den nächtlichen Abruf und einen Listenwechsel überlebt (Driver 2). Der
neue Reihenschlüssel kostet eine Vertragsanhebung, hält dafür Schema, Vertrag und Code an einer
gemeinsamen Wahrheit, die ein Test prüft (Driver 3). Die Begrenzung auf 20 statt 30 Titeln kauft für
einen verzichtbaren Regler keine Wirkung auf sechs fremde Reihen ein (Driver 1). Der Zuschnitt auf
bestehende Pakete hält Regel 2 unberührt.

### Consequences

- Vertrag 1.1.0: **sechs** neue Admin-Operationen (fünf für Spotlight, dazu die binäre Route
  `getAdminItemImage` aus Schritt 5b), `POST /admin/trakt/refresh` entfällt, `TraktStatus` bekommt
  `listKey` und `entries` statt `trending` und `popular`, `HomePage` bekommt `heroRow`, `ItemRef`
  bekommt `hasTrailer` und ein `maxLength` auf `note`, `AdminPick` bekommt `posterUrl` (additiv),
  `HomeRowKey` tauscht zwei Werte. Für Fremd-Clients brechend; steht in den Release-Notes.
- Admin-Oberflächen können ab 5b Bibliotheksposter zeigen, **ohne** `scoped` zu lesen:
  `GET /admin/items/{id}/images/{kind}/{size}` ist systemweit und profilunabhängig, nach Regel 6 ein
  `http.Handler`. Die Spotlight-Seite, die Pflegewerkzeuge (`no_images`) und die Titelseite können
  sie ohne Umbau mitbenutzen — sie hängt am Titel, nicht am Pick.
- Migration 00020 baut `home_rows` neu, löscht `trakt_lists`, legt drei Tabellen und einen Index auf
  `items.release_date` an. Ein `hidden`-Flag der zwei entfallenden Reihen geht verloren.
- Spec E-1 und E-7 sind überholt; die Spec bleibt unverändert, die Abweichung steht als Frage 11 im
  Entscheidungsprotokoll. AN-3 aus Entscheid 9 ist geändert. P-10 verliert das Wort «Karussell».
- `TraktMinHits` entfällt; `TraktMaxAge` wird aus `interval_hours` berechnet; `MaxRowItems` bleibt.
- Der Schema-Test `TestContractNoTraktPerProfile` bekommt `spotlight` in die Allowlist; seine
  I-7-Absicht bleibt unverändert.
- Die Verwaltung bekommt eine Seite in der Gruppe `curation`, ein neues Symbol und Texte in beiden
  Sprachen. Der Trakt-Status zieht von der Seite «Startseite» dorthin; der Schlüssel bleibt bei den
  Integrationen, und `IntegrationsPage.tsx` Zeile 219 verweist neu dorthin.
- `migration00015_test.go` und `frozen_m7_test.go` ändern ihre Zusicherungen; der M7-Beleg wandert
  nach `spotlight_entries`, statt zu verschwinden.

### Follow-ups

1. **Ausstiegsklausel D3 → D1** nach der Messung in Schritt 8.
2. **Neuer eingefrorener Datenbankstand** `testdata/db/M11.sql` nach der Freigabe.
3. **Mehrere Spotlight-Listen** (zwei Bänder, Wechsel je Wochentag) — ausdrücklich **nicht** Teil
   dieses Plans (R-2). Ideensammlung.
4. *(erledigt, kein Follow-up mehr)* Mockup I ist seit Marcos Antwort auf S-11 Teil dieses Plans,
   als Schritt 5b.
5. **Trennung von «Neu hinzugefügt» und «Kürzlich veröffentlicht»** über ein Zeitfenster auf
   `added_at`, falls die Überlappung stört (R-7).
6. **Mehr als 20 Titel je Reihe**: eigener Plan mit Messung über alle sieben Reihen (`clamp` wird an vier Stellen gerufen: `visibility/discover.go` Zeilen 62, 114, 132, 271), falls gewünscht
   (3.4).
7. **Abnahmebericht 1.1** nach dem Muster von `docs/dev/ABNAHME-1.0.md`.
