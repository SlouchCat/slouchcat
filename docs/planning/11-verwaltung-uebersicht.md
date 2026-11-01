# Konzept: Übersicht der Verwaltung (Seite `/admin`)

- **Status: Fassung 3 (1. November 2026), Bauplan für Variante B (Abschnitt 11).** Fassungen 1
  und 2 legten vier Varianten zur Wahl vor; ihr Text bleibt unten stehen, damit die Wahl
  nachvollziehbar ist. **Wo Abschnitt 11 etwas anderes sagt als die Abschnitte 4 bis 8, gilt
  Abschnitt 11.** Vorgeschichte: Marco hat am 31. Oktober 2026 am Bildschirmfoto
  der heutigen Übersicht gesagt: «Ich bin mit der Übersicht der Verwaltungsseite überhaupt nicht
  einverstanden. Es soll einfach, übersichtlich, symmetrisch und klar sein.» Die Struktur darf sich
  ändern, das Aussehen muss zum Rest der App passen; Emby dient als Vorbild. Dieses Papier legt
  **vier Varianten** vor (Abschnitt 5) und einen klickbaren Prototyp
  (`docs/planning/mockups/11-verwaltung-prototyp.html`, Abschnitt 9). Der OMC-Kritiker hat
  Fassung 1 geprüft (1 Blocker, 8 Major, 8 Minor; Abschnitt 11); Fassung 2 arbeitet alle ein.
  **Marco hat am 31. Oktober 2026 Variante B «Seitenleiste wie Emby» gewählt**, mit dem Wunsch,
  die Gruppentitel deutlich stärker und farbig zu markieren (Abschnitt 10.1). **Alle Fragen F-1
  bis F-9 sind beantwortet** (Abschnitt 10.2). Fassung 3 legt die Umsetzungsschritte für B fest
  (Abschnitt 11). **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Datum: 31. Oktober 2026; Fassung 2 vom selben Tag; Fassung 3 vom 1. November 2026**
- **Wogegen dieses Konzept verankert ist:** Fassung 3 auf `main` **`6d562b8`** («Merge pull
  request #59»), Vertragsfassung **1.6.0**. Konzept 10 (Metadaten) ist umgesetzt und gemergt
  (PR #58, `da5d854`): Vertrag 1.6.0, ADR 0018, Frage 16, Migrationen 00028–00030 und die
  18. Verwaltungsseite «Metadaten» sind in `main`. Damit sind die nächsten freien Nummern sicher:
  **Frage 17, ADR 0019, Migration 00031, Vertrag 1.7.0** (additiv). (Fassung 2 war gegen
  `d3d73bc` und Vertrag 1.5.0 verankert; die Zeilenangaben in den Abschnitten 1 bis 8 beziehen
  sich auf diesen Stand.)
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).

> **Dieses Papier ändert keine Zeile Code.** Es betrifft die Oberfläche der Übersicht, die
> Navigation aller Verwaltungsseiten und wenige additive Felder im Vertrag (Abschnitt 6).

## 1. Ausgangslage und Problem

### 1.1 Was Marco am Bild stört

Am Bildschirmfoto vom 31. Oktober 2026 (Server mit 0 Streams und 265 fehlgeschlagenen
Keyframe-Jobs, App-Seitenleiste ausgeblendet) sind diese Mängel zu sehen:

| Nr. | Mangel | Ursache im Code |
|---|---|---|
| P-1 | Die vier Karten der ersten Reihe sind **ungleich hoch**. | Die Rasterzellen sind gleich hoch, aber das `Panel` im umhüllenden `div` füllt seine Zelle nicht (`AdminHomePage.tsx:69-70`); dazu verschieden lange Inhalte. |
| P-2 | Die Karte **Webhooks steht allein** in der zweiten Reihe. | Fünf Kacheln in einem Raster `auto-fill minmax(18rem, 1fr)` (`AdminHomePage.module.css:14-18`), das bei den meisten Breiten vier Spalten hat. |
| P-3 | **Speicher zeigt zweimal dieselben Zahlen** (`/config` und `/cache`, beide 19,4 GB von 34,4 GB belegt). | `statfs` je Verzeichnis (`storage_linux.go:9-16`); der Vertrag sagt nicht, ob beide auf demselben Datenträger liegen. |
| P-4 | Die Tabelle **«Fehlgeschlagene Jobs»** mit langen ffprobe-Pfaden dominiert die Seite. | Volle Fehlertexte in Monoschrift mit `overflow-wrap: anywhere`. Darunter folgt die Fehlerliste aus dem Server-Log; sie enthält **auch** diese Jobfehler (ADR 0009:40), dazu andere Fehler (Trakt, Scan, HTTP). |
| P-5 | Die **Navigation** bricht unruhig auf zwei Zeilen um; Gruppennamen und Links stehen gemischt. | `ui/AdminNav` mit `flex-wrap` (`AdminNav.module.css:9-41`). |
| P-6 | Ein **grosser Titel mit Erklärsatz** nimmt den oberen Bildschirm ein. | Statischer Kopf (`AdminHomePage.tsx:451`). |
| P-7 | Unten folgen 17 Verwaltungskarten, nachdem man durch Fehler gescrollt hat; die Seite hat **keine klare Ordnung** (Zustand, Fehler, Server, Fehler, Karten). | Reihenfolge der Blöcke in `AdminHomePage.tsx`. |

### 1.2 Was die Seite heute zeigt (in dieser Reihenfolge)

1. Titel «Verwaltung» mit Erklärsatz.
2. Hinweise: neue Version, Entwicklungsstand, verlorene Ordnerplätze (nur wenn zutreffend).
3. Raster mit fünf Kacheln: Streams (bis 5 Zeilen), Hintergrundjobs (wartend, laufend,
   fehlgeschlagen, pausiert), Speicher (zwei Balken), Integrationen (acht Punkte), Webhooks.
4. «Letzte fehlgeschlagene Jobs», bis 10 Zeilen mit **Erneut** und **Abbrechen**.
5. Server (Version, Startzeit, Grafikchip, Dateiwächter) mit Warnungen bei fehlendem Grafikchip
   und vollem Dateiwächter, je mit Befehl (`SystemState.tsx:75-97`).
6. Fehlerliste aus dem Server-Log, bis 50 Einträge; neue Fehler werden für Bildschirmleser als
   Zahl angesagt (`role="status"`, `AdminHomePage.tsx:267`).
7. 17 Karten zu allen Verwaltungsseiten in vier Gruppen.

Daten: `GET /admin/dashboard` alle 5 Sekunden (`queries.ts:320-327`), `GET /admin/system` einmal.

### 1.3 Die Seitenleiste der App

Ab 768 px zeigt die App links ihre Seitenleiste (`AppLayout.tsx`, `ui/SideNav.tsx`, ADR 0016);
jedes Gerät kann sie über das Menü-Symbol ausblenden. Auf Marcos Bildschirmfoto ist sie
ausgeblendet. Mit Seitenleiste ist der Inhalt rund 15 rem schmaler. Jede Variante muss in
**beiden** Zuständen aufgehen; der Prototyp hat dafür einen Schalter.

## 2. Was gilt (Spec, Plan, Entscheide)

| Quelle | Inhalt | Folge für die neue Übersicht |
|---|---|---|
| Spec A-2 (`01-anforderungen-v1.md:142`) | Alles «an einem Ort», mit derselben Sorgfalt wie die Zuschauerseite. | Alle Verwaltungsseiten bleiben von der Übersicht aus erreichbar. |
| Spec A-3, Plan Abnahme A-3 (`03-plan-v1.md:585`) | Dashboard zeigt Streams, Jobs (wartend/laufend/fehlgeschlagen), **freien Speicher `/config` und `/cache`**, Integrationsstatus, **letzte 50 Fehler**; provozierter Fehler erscheint **< 10 s**. Der Test provoziert einen **Trakt**-Fehler (`web/e2e/admin-home.spec.ts:166-181`). | Variante D erfüllt A-3 weiter, wenn die Fehlerliste zugeklappt auf der Übersicht bleibt (F-3). Speicher zusammenlegen braucht eine neue Lesart der Abnahme (F-4). Beides kommt in **Frage 17**. |
| Spec A-5 | Hinweis bei neuer Version. | Bleibt, im Hinweisband. |
| Entscheid 10 (e) (`04-entscheidungen.md:19`) | Webhooks-Kachel auf dem Dashboard. | D hat eine eigene Karte «Webhooks». |
| Entscheid 15, Konzept 09 F-6 und `09-hintergrundjobs.md:226` | Fehlgeschlagene Jobs (letzte 10 mit Erneut/Abbrechen) bleiben auf dem Dashboard; Warnwert «Pausiert». | D zeigt nur eine Zusammenfassung je Jobart mit Link; das weicht ab (F-2, Frage 17). «Pausiert» bleibt in der Karte Aufgaben. |
| Entscheid 3 | Der erste Scan zeigt seinen Fortschritt auf dem Dashboard. | Laufende Aufgaben mit Fortschritt (B, D) passen dazu. |
| Plan A2-N12 (`03-plan-v1.md:815`) | Abgebrochener Scan wird auf dem Dashboard gemeldet, «Verlust akzeptieren» dort. | Heute steht «Verlust akzeptieren» schon auf der Seite Bibliotheken (`LibraryCard.tsx:103-108`), nicht auf dem Dashboard; der Code weicht also bereits ab. F-8 klärt, ob die Übersicht einen Hinweis mit Link bekommt. |
| Plan P-15 und Regel 8 (`CLAUDE.md`) | Tastaturtest für jede Seite. | `admin-home.spec.ts` und die Navigationstests werden angepasst, nicht gelöscht (Abschnitt 8). |
| Regel 9 (`CLAUDE.md`) | Farben nur aus Tokens, Texte nur aus `locales/{de,en}.json`, Komponenten aus `web/src/ui`. | Neue Bausteine kommen nach `web/src/ui` mit Story und Referenzbild; neue Texte in Deutsch und Englisch. |
| Regel 11 (`CLAUDE.md`) | Keine Nicht-Ziele, nichts vorziehen. | Aktivitätsprotokoll und Neustart-Knopf aus Variante B wären neue Funktionen (Abschnitt 6.3). |

## 3. Ziel

Die Übersicht beantwortet in **fünf Sekunden** drei Fragen und führt dann weiter:

1. **Muss ich etwas tun?** Nur wenn ja, ein Hinweisband oben mit Knopf. Sonst nichts.
2. **Was läuft gerade?** Streams und Aufgaben, kurz, mit Fortschritt.
3. **Wohin muss ich?** Jede Verwaltungsseite ist einen Klick entfernt.

Lange Listen (alle Jobs, alle Streams) wohnen auf ihren eigenen Seiten. Die Übersicht fasst
zusammen und verlinkt.

## 4. Gestaltungsregeln (gelten für jede Variante)

- **G-1 Strenges Raster:** gemessen an der **echten Inhaltsbreite** (mit oder ohne
  App-Seitenleiste), nicht an der Fensterbreite. Karten 2 Spalten am Desktop, 1 am Handy;
  Bereichskarten 4 Spalten ab rund 1040 px Inhaltsbreite, sonst 2, am Handy 1.
- **G-2 Gleiche Höhe je Reihe:** Karten einer Reihe sind gleich hoch und füllen ihre Zelle; keine
  Karte steht allein.
- **G-3 Eine Hierarchie:** Seitentitel, darunter Karten mit gleichem Kopf (Symbol, Titel, Link
  unten). Kein Erklärsatz unter dem Titel; «Live, alle 5 Sekunden» steht klein rechts.
- **G-4 Nichts Langes auf der Übersicht:** Fehlertexte höchstens eine Zeile, gekürzt, mit Link.
  Höchstens drei Zeilen je Liste.
- **G-5 Ein Hinweisband statt verstreuter Warnungen,** nur wenn etwas zu tun ist: neue Version,
  Entwicklungsstand, fehlgeschlagene Jobs, pausierte Jobs, stumme Webhooks, Integrationsfehler,
  verlorene Ordnerplätze, fehlender Grafikchip, voller Dateiwächter, Speicher über 90 %, je nach
  F-8 abgebrochener Scan. Das Band ist eine Live-Region (`role="status"`); Bildschirmleser hören
  neue Hinweise wie heute neue Fehler.
- **G-6 Speicher einmal**, wenn `/config` und `/cache` auf demselben Datenträger liegen; sonst zwei
  Balken nebeneinander.
- **G-7 Aussehen wie die App:** dieselben Tokens, Flächen, Radien, Fokusrahmen und Komponenten
  (`Panel`, `StatTile`, `StorageMeter`, `LinkCard`, `ProgressBar`, `PosterCard`). Profilfarbe als
  Akzent.
- **G-8 Jede Angabe nur einmal** auf der Seite.
- **G-9 Navigation ruhig:** keine gemischt umbrechenden Zeilen (Abschnitt 7).

## 5. Die vier Varianten

Alle vier liegen im Prototyp, je in drei Zuständen: **«Ruhig»** (Stand des Bildschirmfotos),
**«Alles gut»** (keine Probleme, kein Hinweisband) und **«Betrieb»** (2 Streams, 3 laufende
Aufgaben, 1 stummer Webhook); am Desktop mit und ohne App-Seitenleiste und im Handy-Rahmen.

### Variante A «Ruhiges Raster»

Navigation in zwei Zeilen (Gruppen als Reiter, darunter die Seiten der Gruppe). Kompakter Titel,
Hinweisband nur bei Bedarf, **vier gleiche Kennzahlen** (Streams, Jobs, Speicher %, Integrationen
5/8), darunter **2 × 2 Karten**: Laufende Streams, Hintergrundjobs (Zähler und die drei neuesten
Fehlschläge), Speicher, Integrationen und Webhooks. Unten eine schmale Serverzeile.

- **Für:** am ruhigsten, nah an heute, braucht kaum neue Serverdaten.
- **Gegen:** Kennzahlen und Karten wiederholen sich teilweise (G-8); ohne Bereichskarten führt nur
  die Navigation weiter; die Streams-Karte ist meist leer.

### Variante B «Seitenleiste wie Emby»

In der Verwaltung **ersetzt** eine Verwaltungsleiste die Seitenleiste der App, am selben Platz und
im selben Stil: oben «Zurück zur App», dann Übersicht und die vier Gruppen mit Zählern (265, 2, 1).
Sie steht immer da, auch wenn die App-Seitenleiste sonst ausgeblendet ist; am Handy ist sie ein
Knopf «Verwaltung · Übersicht», der ein Blatt öffnet. Rechts **sechs gleiche Karten in zwei
Spalten**: Server (mit «Nach Updates suchen» und «Neu starten»), Jetzt läuft, Aufgaben, Letzte
Aktivität, Speicher, Hinweise.

- **Für:** am nächsten an Emby; das Menü ist am Desktop immer sichtbar; `ui/SideNav` gibt es schon
  als Vorlage.
- **Gegen:** alle Verwaltungsseiten bekommen ein anderes Gerüst (Navigation, Referenzbilder,
  Tastaturtests); «Letzte Aktivität» und «Neu starten» gibt es im Server nicht (6.3); am Handy ist
  das Menü versteckt.

### Variante C «Status und Kacheln»

Eine **Zustandskarte** («Alles in Ordnung» oder «1 Sache braucht Aufmerksamkeit») mit Chips und
vier Zahlen. Darunter **«Alle Bereiche»** als vier Gruppenkarten; jede Seite mit Symbol, Titel und
einer Live-Zeile. Navigation als eine Zeile mit vier Aufklappmenüs.

- **Für:** am einfachsten; die Übersicht ist eine Startrampe; stark am Handy.
- **Gegen:** keine Live-Details (Streams, Fortschritt, Speicherbalken) ohne Klick; Integrationen
  nur als «5 / 8» (A-3 verlangt den Status je Integration); viele Live-Zeilen brauchen neue Zahlen;
  die Gruppenkarten haben 4, 4, 5 und 5 Seiten und enden innen ungleich.

### Variante D «Cockpit» (Empfehlung)

Navigation aus A. Hinweisband nur bei Bedarf. Dann **sechs gleiche Live-Karten in drei Reihen**:

| Links | Rechts |
|---|---|
| **Jetzt läuft:** Poster, Titel, Profil, Gerät, Modus, Fortschritt; leer «Gerade läuft nichts.» | **Aufgaben:** Wartend, Laufend, Fehlgeschlagen (Pausiert, wenn > 0); bis drei laufende Aufgaben mit Fortschritt und Restzeit, sonst der nächste geplante Lauf; Fehlschläge je Jobart in einer Zeile mit dem neuesten Fehler gekürzt. |
| **Speicher:** ein Balken, wenn gemeinsamer Datenträger, sonst zwei. | **Server:** Version, Laufzeit, Transcoding, Dateiwächter, Update-Zustand, «Nach Updates suchen». |
| **Integrationen:** acht Punkte mit Namen, «5 / 8». | **Webhooks:** eingerichtet, Meldungen in 24 h, still seit 7 Tagen. |

Darunter **«Alle Bereiche»**: vier Gruppenkarten mit allen 18 Seiten; eine Live-Zeile nur, wo die
Zahl billig ist (F-7), sonst ein kurzer Zweck («Zeitpläne, Pause, Abbruch»). Darunter, zugeklappt,
das **Server-Protokoll** mit den letzten 50 Fehlern (F-3).

- **Für:** beantwortet die drei Fragen aus Abschnitt 3 auf einer Seite; jede Angabe steht einmal;
  Kartenreihen sind 2 gleiche Felder; Live-Karten wie Emby; die Navigation bleibt oben wie auf den
  anderen Verwaltungsseiten; erfüllt A-3 und Entscheid 10 (e) weiter.
- **Gegen:** die längste Seite; die Bereichskarten enden innen ungleich wie bei C; Aufgaben und
  Speicher brauchen kleine Vertragserweiterungen (Abschnitt 6.2).
- **Kürzere Form:** ohne «Alle Bereiche» (dann führt nur die Navigation weiter).

### 5.1 Vergleich

| | A | B | C | D |
|---|---|---|---|---|
| Einfach und klar | ●●● | ●● | ●●● | ●● |
| Symmetrisch | ●●● | ●●● | ●● | ●● |
| Live-Einblick | ● | ●●● | ● | ●●● |
| Nah an Emby | ● | ●●● | ● | ●● |
| Passt zu den anderen Verwaltungsseiten ohne Umbau | ●● | ● | ●● | ●● |
| Neue Serverarbeit | klein | gross | mittel | klein |

(A, C und D ändern die Navigation aller Verwaltungsseiten gleich: zwei Zeilen statt einer
umbrechenden. B tauscht das ganze Gerüst.)

## 6. Welche Daten fehlen

### 6.1 Schon vorhanden (keine Vertragsänderung)

- **Laufzeit:** im Client aus `SystemInfo.startedAt`.
- **Stream-Fortschritt, Modus, Gerät, Qualität:** `AdminStream.positionMs/durationMs/mode/
  hardware/quality/device`.
- **Update-Zustand und Knopf:** `Dashboard.update`, `POST /admin/update/check`.
- **Integrationen und Webhooks:** `Dashboard.integrations`, `Dashboard.webhooks`.
- **Server-Warnungen:** `update.development`, `GPUInfo`, `InotifyInfo.warn`, `browse.lostSlots`.

### 6.2 Klein und additiv (Vertrag 1.7.0, nicht brechend)

| Feld | Gebraucht von | Herkunft |
|---|---|---|
| `Dashboard.jobs.active: JobRun[]` (höchstens 3, mit `progress`, `estimatedSecondsLeft`, `subject`) | B, D | Gibt es in `GET /admin/job-types` (`JobTypeInfo.running`); der Server reicht es durch. **Nicht** `jobs.running`: das ist heute schon eine Zahl (`JobCounts`), ein Typwechsel wäre brechend. Alternative ohne Vertrag: zweite Abfrage alle 5 s. |
| `Dashboard.jobs.failedByType: {type, count, lastError, lastFailedAt}[]` | D | Zahl aus `JobTypeInfo.counts.failed`; der neueste Fehler je Jobart braucht eine **neue Abfrage** (jüngster fehlgeschlagener Lauf je Art). |
| `Dashboard.jobs.nextRun: {type, at}` | B, D | `job_schedules` aus Konzept 09. |
| `Dashboard.storage.shared: boolean` | alle (G-6) | Entscheid im Server: `os.Stat(dir).Sys().(*syscall.Stat_t).Dev` beider Ordner vergleichen (als `uint64`, der Typ hängt von der Architektur ab) in `storage_linux.go`; `storage_other.go` liefert `false`. Kein roher Gerätewert im Vertrag, keine Regel im Client. **Grenze:** btrfs-Subvolumes und ZFS-Datasets auf demselben Pool haben verschiedene `Dev`-Werte und zeigen weiter zwei Balken. |
| `AdminStream.posterUrl` | B, D | Der Client kann die URL nicht selbst bauen: Die Bildroute antwortet `immutable, max-age=31536000` (`item_handlers.go:345`), eine URL ohne `?v=` bliebe ewig alt, und Folgen haben kein eigenes Poster (`admin/images.go:23-34`). Der Server liefert für Folgen das Poster der Serie, mit Hash. |
| `Dashboard.attention.abortedScans` | nur mit F-8 «ja» | Abgebrochene Scans mit Bibliothek; Link auf die Seite Bibliotheken. |

### 6.3 Gross oder neu (nicht empfohlen in diesem Schritt)

- **Letzte Aktivität** (B): es gibt kein Protokoll. Das wäre eine neue Tabelle mit Ereignissen
  (Wiedergabe, Scan, Sicherung, Webhook) – eine eigene Funktion, als Idee in `docs/ideas/`.
- **Neustart-Knopf** (B): es gibt keinen Endpunkt; ein Neustart läuft heute nur nach einer
  Wiederherstellung. Ebenfalls Idee.
- **Live-Zeilen aller 18 Bereiche** (C, D): je Seite eine eigene Abfrage oder ein neues Objekt
  `Dashboard.counts`. Empfehlung (F-7): Live-Zeile nur, wo die Zahl billig ist (Bibliotheken und
  Titel, Konten und Profile, offene Pflegefälle, letzte Sicherung), sonst ein kurzer Zweck.

## 7. Navigation der Verwaltung (A, C, D)

Heute bricht `ui/AdminNav` Gruppen und Links gemischt um (P-5) und ist **ein einziger Tab-Halt**
für alle Links (`AdminNav.tsx:37-42`). Vorschlag für **alle** Verwaltungsseiten gleich:

- **Zeile 1:** «Übersicht» und die vier Gruppen als Reiter, aktive Gruppe mit Akzentlinie. Ein
  Gruppenreiter **öffnet** seine Seiten in Zeile 2; er navigiert nicht.
- **Zeile 2:** die Seiten der offenen Gruppe als Pillen. Auf der Übersicht ist Zeile 2 zu, bis man
  eine Gruppe öffnet; auf einer Unterseite steht ihre Gruppe offen.
- **Handy:** beide Zeilen wischen seitwärts; ein Verlauf am rechten Rand zeigt, dass mehr kommt.
- **Tastatur:** **zwei** Tab-Halte (Reiter, Seiten), Pfeiltasten innerhalb, Pos1/Ende wie heute.
  Das ändert das Tastaturmodell von `AdminNav` und braucht neue Tests.
- **Profile und Benachrichtigungen** liegen ausserhalb von `/admin` und erscheinen wie heute nur
  als Bereichskarten, nicht in Zeile 2.

Variante B ersetzt das durch die Verwaltungsleiste (5, B); Variante C durch Aufklappmenüs.

## 8. Grober Ablauf nach der Wahl (Variante D)

> **Überholt durch Abschnitt 11** (Marco hat B gewählt). Bleibt als Vergleich stehen.

Die genauen Schritte schreibt Fassung 3, wenn Marco gewählt hat. Grob:

0. Frage 17 ins Entscheidungsprotokoll (Lesart A-3 für Speicher und Fehlerliste, Abweichung von
   Konzept 09 Abschnitt 2 Punkt 4, A2-N12), Reihenfolge zu Konzept 10, Idee «Aktivitätsprotokoll
   und Neustart» nach `docs/ideas/`.
1. Vertrag 1.7.0: Felder aus 6.2, `x-visibility: admin`, `task gen`, Vertrags- und Schematests,
   neue Abfrage für den neuesten Fehler je Jobart, `storage_linux.go` und `storage_other.go`.
2. `ui`: Karte, die ihre Zelle füllt; Aufgabenzeile; Bereichskarte; zweizeilige `AdminNav` mit
   neuem Tastaturmodell; je mit Story. Die drei Referenzbilder `ui-adminnav--*` und neue Bilder
   entstehen auf CT 190 (Weg ohne Docker, siehe Bibliotheken-Stand).
3. `AdminHomePage` neu aufbauen; neue Texte in `de.json` und `en.json`.
4. Tests: `AdminHomePage.test.tsx`; `web/e2e/admin-home.spec.ts` (Regionen, Fehler < 10 s,
   Tab-Reihenfolge); alle Specs, die die Navigation «Verwaltung» benutzen: `admin-integrations`,
   `admin-webhooks`, `admin-podcasts`, `admin-collections`, `admin-home-rows`, `admin-picks`,
   `admin-spotlight`, `admin-jobs`, `invitations`, `profiles`, `sidenav`.

Schätzung: **6 bis 8 Tage** für D (mit Navigation auf allen Verwaltungsseiten), **3 bis 4 Tage**
für A, **10 bis 14 Tage** für B (Gerüst aller Seiten, ohne Aktivitätsprotokoll).

## 9. Klickbarer Prototyp

`docs/planning/mockups/11-verwaltung-prototyp.html`, eine Datei ohne Fremdquellen. Oben ein
Umschalter: Variante A/B/C/D; Zustand «Ruhig», «Alles gut», «Betrieb»; «Desktop»/«Handy» (390 px);
App-Seitenleiste aus/ein; «Seltene Hinweise zeigen» (Entwicklungsstand, kein Grafikchip, voller
Dateiwächter, blockierter Ordnerbrowser); «Neue Serverdaten markieren» (gestrichelter Rahmen um
alles, was der heutige Vertrag nicht liefert). Der Prototyp ist kein Code aus `web/src`; Farben
und Abstände sind von den Tokens abgeschrieben. Das Server-Protokoll (F-3) zeigt er noch nicht.

## 10. Fragen an Marco

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-1 | Welche Variante: A, B, C oder D? | **D «Cockpit»** |
| F-2 | Die Tabelle «Fehlgeschlagene Jobs» mit Erneut/Abbrechen zieht auf die Seite «Hintergrundjobs» (dort gibt es sie schon); die Übersicht zeigt je Jobart eine Zeile mit Link. Einverstanden? | **Ja.** |
| F-3 | Wohin mit der Liste der letzten 50 Server-Fehler? (a) zugeklappt ganz unten auf der Übersicht, (b) eigene Seite «Protokoll» unter System. | **(a)**: bleibt auf einen Blick erreichbar, die Abnahme A-3 gilt unverändert. |
| F-4 | Speicher nur einmal zeigen, wenn Konfiguration und Cache auf derselben Platte liegen? | **Ja.** |
| F-5 | Laufende Aufgaben mit Fortschritt und Restzeit auf der Übersicht? | **Ja.** |
| F-6 | «Letzte Aktivität» und «Neu starten» wie in Emby? | **Nein, jetzt nicht**; als Idee festhalten. |
| F-7 | Zahlen in «Alle Bereiche» nur, wo sie billig sind, sonst ein kurzer Zweck? | **Ja.** |
| F-8 | Ein abgebrochener Bibliotheksscan als Hinweis auf der Übersicht, mit Link zur Seite Bibliotheken (dort bleibt «Verlust akzeptieren»)? | **Ja.** |

### 10.1 Marcos Wahl (31. Oktober 2026)

- **F-1: Variante B «Seitenleiste wie Emby»**, nicht die Empfehlung D. Wörtlich: «Mir gefällt die
  Seitenleiste (Emby) am besten. Jedoch müssen die Titel stärker markiert werden (Bibliothek,
  Konten, Kuratierung etc.), am besten farblich markiert.»
- Folge für die anderen Fragen: Die Navigation aller Verwaltungsseiten wird die Verwaltungsleiste
  (Abschnitt 7 gilt dann nicht). F-6 («Letzte Aktivität», «Neu starten») ist bei B die Karte
  rechts unten und der zweite Knopf im Server; bleibt die Empfehlung «nein», füllt eine andere
  Karte den Platz (zum Beispiel Integrationen und Webhooks), damit das Raster symmetrisch bleibt.
- Der Prototyp zeigt für B drei **Titelstile** der Gruppen (Schalter «Gruppentitel»), alle
  0,8125 rem, fett, Grossbuchstaben, mit Gruppensymbol (Bibliothek Film, Konten Personen,
  Kuratierung Stern, System Server):

| Nr. | Stil | Farben (Kontrast auf `bg-elevated`) |
|---|---|---|
| 1 | **Akzent:** Titel und Symbol in der Profilfarbe, feine Linie über jeder Gruppe | Profilfarbe, 6,31:1 bei Ember |
| 2 | **Je Gruppe eine Farbe:** Titel, Gruppensymbol und Markierung der aktuellen Seite in der Gruppenfarbe | Bibliothek Teal 8,35:1, Konten Himmelblau 7,67:1, Kuratierung Rosa 6,72:1, System Violett 7,11:1 – aus den bestehenden Profilfarben |
| 3 | **Band:** Titel als farbig getöntes Band über die ganze Breite, Einträge eingerückt | Profilfarbe auf 16 % Tönung, 5,04:1 |

  Alle liegen über 4,5:1 (WCAG AA). Stil 2 braucht vier neue Tokens in
  `packages/design-tokens/tokens/color.json` (`group.library` usw.); Stil 1 und 3 kommen ohne neue
  Farbe aus. Bei Stil 2 liegt Rosa nahe am Fehlerrot; dafür trägt Kuratierung nie einen roten Zähler.
- **Abweichung:** Die Verwaltungsleiste zeigt Symbole vor jeder Seite (wie Emby, für Stil 2 nötig);
  die Seitenleiste der App hat nach N-2 keine. Das gehört in Frage 17.

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-9 | Welcher Titelstil der Gruppen: 1 Akzent, 2 je Gruppe eine Farbe, 3 Band? | Offen, Marcos Geschmack. |

### 10.2 Marcos Antworten (31. Oktober 2026, Interview)

| Nr. | Antwort | Folge |
|---|---|---|
| F-1 | **B «Seitenleiste wie Emby»** | 10.1 |
| F-2 | **Ja, wie empfohlen.** | Tabelle mit Erneut/Abbrechen nur auf «Hintergrundjobs»; Übersicht je Jobart eine Zeile mit Link. Abweichung von Konzept 09 Abschnitt 2 Punkt 4, in Frage 17. |
| F-3 | **Zugeklappt unten, wie empfohlen.** | Abschnitt «Server-Protokoll» ganz unten, zugeklappt mit Anzahl; Abnahme A-3 unverändert. |
| F-4 | **Nein, immer zwei Balken** (abweichend von der Empfehlung). | `/config` und `/cache` bleiben getrennt, auch bei gleichen Zahlen. Das Feld `storage.shared` (6.2) und G-6 entfallen; A-3 bleibt wörtlich erfüllt. |
| F-5 | **Ja, wie empfohlen.** | `jobs.active`, `jobs.nextRun` (6.2). |
| F-6 | **Beides bauen** (abweichend von der Empfehlung), nach Erklärung: «Neu starten soll unterstützt werden. Letzte Aktivität soll ebenfalls erstellt werden.» | Neues Ereignisprotokoll im Server (Tabelle, Einträge bei Wiedergabe, Scan, Sicherung, Webhook, Serverstart; Aufräumen alter Einträge) und Neustart-Endpunkt mit Rückfrage, weil laufende Streams abbrechen. Beides ist neue Funktion: braucht eine Migration (00031), Vertrag 1.7.0 und vermutlich ADR 0019; in Frage 17. Rund 3 bis 4 Tage zusätzlich. |
| F-7 | **Wenige, wie empfohlen** (nach Beispiel im Prototyp, Schalter «Zahlen»). | Zahlen in der Verwaltungsleiste nur bei Bibliotheken (Titel), Pflege (offene Fälle, Warnfarbe), Konten, Integrationen (eingerichtet / gesamt), Streams und Hintergrundjobs (laufend); Alarme als farbige Pille (Fehlschläge, stummer Webhook). Braucht wenige billige Zähler im Dashboard, kein Zählerobjekt für alle 18 Seiten. |
| F-8 | **Ja, wie empfohlen.** | Hinweis «Scan abgebrochen» in der Karte Hinweise mit Link zu Bibliotheken; `attention.abortedScans` (6.2). |
| F-9 | **3 «Band».** | Keine neuen Farb-Tokens; Titel als getöntes Band in der Profilfarbe, Einträge eingerückt. |

## 11. Umsetzung von Variante B (Fassung 3)

Grundlage: Marcos Wahl und Antworten (10.1, 10.2) und der Prototyp mit den Schaltern
«Gruppentitel 3 Band», «Zahlen wenige», «Aktivität & Neustart mit». Zeilenangaben in diesem
Abschnitt beziehen sich auf `main` `6d562b8`. Der OMC-Kritiker hat diesen Abschnitt geprüft
(Abschnitt 12, Fassung 2 → 3); die Befunde sind eingearbeitet.

### 11.1 Zielbild

**Gerüst.** Auf jeder Seite unter `/admin/*` steht für ein Administratorkonto mit Hauptprofil
links die **Verwaltungsleiste** statt der Seitenleiste der App, am selben Platz und in derselben
Breite (`--sc-size-sidenav-width`). Sie steht ab 768 px **immer** da, auch wenn das Gerät die
App-Seitenleiste ausgeblendet hat (`slouchcat.sidenav = hidden`). Das Menü-Symbol oben links
öffnet auf `/admin/*` die Schublade der App (`SideNavMenu`); so bleibt die App-Navigation einen
Klick entfernt, ohne zweite Leiste. Ausserhalb von `/admin/*` ändert sich nichts.

**Verwaltungsleiste** (von oben nach unten):

1. «Zurück zur App» (Symbol `back`, Ziel `/`).
2. «Übersicht» (`/admin`). Steht eine neue Version bereit, trägt dieser Eintrag das Abzeichen, das
   heute am Eintrag «Verwaltung» der App-Leiste hängt (A-5).
3. Vier Gruppen in der Reihenfolge von `adminGroups` (`sections.ts:8`): Bibliothek, Konten,
   Kuratierung, System. Der **Gruppentitel ist ein Band** (F-9, Stil 3): über die ganze Breite,
   0,8125 rem, fett, Grossbuchstaben, mit Gruppensymbol, Hintergrund die Profilfarbe zu 16 %,
   Schrift in der Profilfarbe. Die Einträge darunter sind eingerückt.
4. Je Gruppe ihre Seiten aus `adminSections` mit Symbol (Abweichung von N-2, 11.2), **alle
   18 Seiten**, also auch «Profile» und «Benachrichtigungen» (beide unter `/settings/*`; ein Klick
   verlässt die Verwaltung, dort steht wieder die App-Seitenleiste). Die Leiste hat damit
   **20 Einträge** (Zurück, Übersicht, 18 Seiten).
5. **Zahlen nach F-7** rechts am Eintrag, gedämpft: Bibliotheken (Titel gesamt), Konten (Anzahl),
   Integrationen (eingerichtet / gesamt), Hintergrundjobs (laufend, nur wenn > 0). **Alarm-Pillen**
   in Farbe: Pflege (offene Fälle, Warnfarbe), Streams (laufend, Akzent), Hintergrundjobs
   (fehlgeschlagen, Fehlerfarbe; ersetzt dann die Zahl «laufend»), Integrationen (stumme Webhooks,
   Warnfarbe; ersetzt dann «x / y»). Jede Zahl hat einen ausgeschriebenen Namen für
   Bildschirmleser («12 Titel brauchen Pflege»).

**Handy (unter 768 px):** keine Leiste. Oben auf jeder Verwaltungsseite steht statt der heutigen
`AdminNav`-Zeile ein Knopf «Verwaltung · *Seitenname*», der die Verwaltungsleiste als Blatt öffnet
(gleiche Bauart wie `SideNavDrawer`: Fokusfalle nur solange offen, Esc schliesst, Fokus zurück auf
den Knopf).

**Übersicht `/admin`.** Kopf: Titel «Verwaltung» (h1 wie heute; viele Specs prüfen ihn), rechts
klein «Live, alle 5 Sekunden». Kein Erklärsatz (P-6). Darunter **acht gleiche Karten in zwei
Spalten** (vier Reihen), je Reihe gleich hoch, jede Karte füllt ihre Zelle (P-1). Die Spaltenzahl richtet sich
nach der Inhaltsbreite (Container-Abfrage, G-1): zwei Spalten ab rund 44 rem, sonst eine.

| Links | Rechts |
|---|---|
| **Server:** Version, Laufzeit, Transcoding (Grafikchip), Dateiwächter, Update-Zustand; Knöpfe «Nach Updates suchen» und «Neu starten» | **Jetzt läuft:** bis drei Streams mit Poster, Titel, Profil, Gerät, Modus, Fortschritt; leer «Gerade läuft nichts.»; Link «Alle Streams» |
| **Aufgaben:** Wartend, Laufend, Fehlgeschlagen, Pausiert (nur > 0); bis drei laufende Aufgaben mit Fortschritt und Restzeit, sonst der nächste geplante Lauf; Fehlschläge je Jobart (höchstens drei Zeilen, neuester Fehler einzeilig gekürzt); Link «Hintergrundjobs» | **Letzte Aktivität:** die sechs neuesten Einträge aus dem Aktivitätsprotokoll (11.4) mit Symbol, Satz und relativer Zeit |
| **Integrationen:** acht Punkte mit Name und Zustand («Trakt: Fehler»), im Kopf «x / y eingerichtet»; Link «Integrationen einrichten» (heute `IntegrationsTile`) | **Webhooks:** eingerichtet, Meldungen in 24 h, still seit 7 Tagen (heute `WebhooksTile`, Entscheid 10 (e)); Link «Webhooks verwalten» |
| **Speicher:** zwei Balken, `/config` und `/cache` (F-4, immer zwei) | **Hinweise:** alles, was zu tun ist (Liste unten), sonst «Keine Hinweise.»; im Kopf die **Zahl der Hinweise** |

Marco hat am 1. November 2026 entschieden, dass Integrationen und Webhooks **eigene Karten
behalten** (O-1, Antwort «Nein, behalten»). Mit «Letzte Aktivität» sind es darum acht Karten; das
Raster bleibt symmetrisch (2 × 4).

Ganz unten **«Server-Protokoll»** als zugeklapptes `<details>` mit der Anzahl im Titel
(«Server-Protokoll · 3 Fehler»); offen die heutige Fehlerliste (bis 50, `ErrorsTile`). Neue Fehler
werden wie heute angesagt (`role="status"`), auch wenn zugeklappt.

**Hinweise** (Reihenfolge = Dringlichkeit; jeder Hinweis eine Zeile mit Link oder Knopf):
fehlgeschlagene Jobs («265 fehlgeschlagen», Link Hintergrundjobs), pausierte Jobarten, abgebrochener
Scan je Bibliothek (F-8, Link Bibliotheken; «Verlust akzeptieren» bleibt dort), verlorene
Ordnerplätze, fehlender Grafikchip und voller Dateiwächter (je mit Befehl wie heute in
`SystemState`), Speicher über 90 %, neue Version (mit Link wie heute), Entwicklungsstand.
Integrationsfehler und stumme Webhooks stehen **nicht** in den Hinweisen, sondern in ihren eigenen
Karten (G-8). Damit
ein neuer Fehler nicht doppelt angesagt wird, sagt die Karte Hinweise nur ihre **Zahl** an
(«3 Hinweise»); den Fehlertext sagt das Server-Protokoll an.

**Neustart auf der Übersicht:** Nach «Neu starten» und Bestätigung ersetzt die Warteanzeige
(`RestartPanel`, 11.5) das Kartenraster; Titel, Leiste und Kopf bleiben stehen.

**Was wegfällt:** die Tabelle «Letzte fehlgeschlagene Jobs» mit Erneut/Abbrechen (F-2; sie ist auf
«Hintergrundjobs»), die 18 Bereichskarten `ManagementCards` (die Leiste ersetzt sie), die horizontale `AdminNav`.
Die Kacheln «Integrationen» und «Webhooks» bleiben als Karten (O-1).

### 11.2 Abweichungen (gehen in Frage 17)

1. **N-2 (Entscheid 14):** Die Verwaltungsleiste trägt Symbole; die App-Seitenleiste bleibt ohne.
2. **Konzept 09 Abschnitt 2 Punkt 4 (`09-hintergrundjobs.md:226`) und Entscheid 15:** Die zehn
   letzten Fehlschläge mit Erneut/Abbrechen stehen nicht mehr auf der Übersicht, sondern nur auf
   «Hintergrundjobs» (F-2). Die Warnung «Pausiert» bleibt (Karte Aufgaben und Hinweis).
3. **Keine Abweichung bei A-3 und Entscheid 10 (e)** (O-1, Marcos Antwort vom 1. November
   2026): Integrationen und Webhooks behalten ihre Karten. Die Abnahme A-3 und ihr Test bleiben,
   wie sie sind (`admin-home.spec.ts`: Trakt-Fehler in der Region «Integrationen» < 10 s), dazu
   neu im aufgeklappten Server-Protokoll. Der Prototyp zeigte bei «Aktivität & Neustart: mit» nur
   sechs Karten; die Seite bekommt acht.
4. **A2-N12:** Der abgebrochene Scan erscheint als Hinweis mit Link; «Verlust akzeptieren» bleibt auf
   der Seite Bibliotheken (F-8).
5. **Neue Funktionen nach F-6:** Aktivitätsprotokoll und Neustart-Knopf. Kein Nicht-Ziel (Spec
   «Non-Goals» nennt beides nicht); sie kommen aus Marcos Antwort, nicht aus der Spec. ADR 0019
   hält Tabelle, Ereignisarten, Aufbewahrung, Datenschutz und den Neustartweg fest.
6. **Prototyp-Zeilen, die das Protokoll nicht so zeigt:** Der Prototyp zeigt für jede Wiedergabe
   den Start; gebaut wird Start (höchstens einmal je 30 min und Titel) und «fertig geschaut»
   (11.4), aber kein Ende bei jedem Stopp, weil jeder Ton-, Untertitel- oder Qualitätswechsel eine
   neue Sitzung ist (`PlayerPage.tsx:171-214`). Eingehende Webhooks stehen nicht im Protokoll; sie
   haben ihr eigenes auf «Integrationen» (`webhook_events`, ADR 0010).

### 11.3 Vertrag 1.7.0 (additiv)

Nichts wird entfernt oder im Typ geändert. **Pflicht** (`required`) in der Antwort sind die
neuen Felder `JobCounts.active`, `JobCounts.failedByType`, `Dashboard.attention`,
`Dashboard.counts`, `Dashboard.activity` (je leere Liste statt fehlend); **optional** sind
`JobCounts.nextRun` (fehlt ohne aktiven Zeitplan) und `AdminStream.posterUrl` (fehlt ohne Poster).
`JobCounts.recentFailed` bleibt im Vertrag und wird weiter gefüllt (fremde Clients); die eigene
Oberfläche liest es nicht mehr. Die Beschreibung von `getDashboard` (`api/openapi.yaml:5062-5065`,
«last ten failed jobs») wird nachgeführt.

| Ort | Neu | Herkunft im Server |
|---|---|---|
| `JobCounts.active` | `JobRun[]`, höchstens 3, neueste zuerst | Neue Abfrage `ListRunningJobs` (alle Arten, `LIMIT 3`); Abbildung über `admin.jobRun` (`server/internal/admin/jobtypes.go:258`) mit derselben Schätzung wie auf «Hintergrundjobs». |
| `JobCounts.nextRun` | `{type, at}`, optional | Frühestes `NextRunAt` über `s.jobctl.sched.Schedules` (`server/internal/jobs/scheduler.go:316`); `admin` hält den Scheduler schon über `WithJobControl` (`main.go:332`, `jobtypes.go:43-50`). Ohne ausgeschaltete Zeitpläne und ohne pausierte Jobarten. |
| `JobCounts.failedByType` | `{type, count, lastError, lastFailedAt}[]`, nach `lastFailedAt` absteigend | Neue Abfrage `ListFailedJobsNewestPerType` (Fensterfunktion `ROW_NUMBER() OVER (PARTITION BY type ORDER BY updated_at DESC)` plus Anzahl je Art) in `server/queries/system/admin_dashboard.sql`. |
| `AdminStream.posterUrl` | Bild-URL mit Hash, optional | Für Folgen das Poster der Serie; dafür liefert `AdminStreamItems` (`server/queries/system/admin.sql:283-295`) zusätzlich die Id der Serie. Hash wie bei `posterHashes` (`server/internal/admin/images.go:74`). |
| `Dashboard.attention` | `{abortedScans: {libraryId, name, reason}[]}` | Bibliotheken mit `last_scan_state = 'aborted'`, `last_scan_error <> 'cancelled'` und ohne `loss_accepted_at`. Das ist **strenger** als die heutige Bedingung des Knopfs auf «Bibliotheken» (`LibrariesPage.tsx:338` prüft `lossAcceptedAt` nicht); der Knopf wird in Schritt 5 an dieselbe Bedingung angeglichen. Der Server entscheidet (Regel 12). |
| `Dashboard.counts` | `{titles, accounts, maintenanceOpen}` | `titles` = Summe der Titelzahlen aller Bibliotheken (wie `LibraryScan.itemCount`); `accounts` = `CountAccounts` (`server/queries/app/auth.sql:4`); `maintenanceOpen` = Summe der **fünf billigen Pflegezähler** (`AdminCountUnmatched`, `…NoImages`, `…Unrated`, `…Orphaned`, `…Unplayable`, `server/queries/system/admin.sql:30-118`). Die sechste Pflegeart `rating_mismatch` prüft alle Titel gegen den Anbieter und zählt **nicht** mit; die Pille heisst darum «Titel brauchen Pflege», nicht «alle Pflegefälle». Der Server rechnet die drei Zahlen höchstens alle **60 s** neu (Zwischenspeicher in `admin.Service`), weil die Leiste auf jeder Verwaltungsseite pollt; jede Änderung über die Pflege-, Bibliotheks- und Kontoseiten leert den Speicher. |
| `Dashboard.activity` | `ActivityEntry[]`, höchstens 6 | Aktivitätsprotokoll (11.4). |
| `POST /admin/server/restart` | `restartServer`, `x-visibility: admin`, **202** mit neuem Schema `RestartAccepted {restartAt}`; **409** mit Problem-Code `restart_pending` (Neustart oder Wiederherstellung steht aus) oder `backup_busy` (eine Sicherung wird gerade geschrieben) | 11.5 |

`ActivityEntry`: `{id, at, kind, profileName?, accountEmail?, itemTitle?, libraryName?, detail}`
mit `detail` als Objekt `{newTitles?, sizeBytes?, version?, trigger?, reason?, jobType?,
count?, by?}`. **`kind` ist ein String ohne Enum** (wie `JobTypeName`, «No enum, so a new type
breaks no client»); die bekannten Werte stehen in der Beschreibung. **Der Server schickt keine
fertigen Sätze**: die Oberfläche baut den Satz aus `kind` und den Feldern über
`de.json`/`en.json` (Regel 9); für eine unbekannte Art zeigt sie einen allgemeinen Satz
(«Ereignis auf dem Server»).

Integrationen «x / y» und Streams zählt die Oberfläche aus `Dashboard.integrations` und
`Dashboard.streams`; dafür braucht es kein Feld.

**Nebenbefund (Fehler in `main`):** Der Handler bildet `JobCounts.paused` nicht ab
(`server/internal/api/system_handlers.go:75` setzt nur `Queued`, `Running`, `Failed`), obwohl das
Modell den Wert füllt (`server/internal/admin/dashboard.go:94`). Die Kachel «Pausiert»
(`AdminHomePage.tsx:547`) erscheint darum nie. Schritt 1 behebt das mit einem eigenen Commit und
einem Test.

### 11.4 Aktivitätsprotokoll «Letzte Aktivität»

(Name «Aktivitätsprotokoll», damit es nicht mit dem Ereignisprotokoll der Webhooks aus
Entscheid 10 verwechselt wird.)

**Warum eine eigene Tabelle (Alternative geprüft):** Man könnte die Liste aus vorhandenen Tabellen
zusammensetzen (`playback_sessions`, Jobverlauf, `webhook_events`). Das scheitert an drei Stellen:
Serverstart, Neustart und Sicherungen stehen nirgends; `playback_sessions` hat eine Zeile je
Spurwechsel und keine Zeile «fertig geschaut»; die Quellen haben verschiedene Aufbewahrung
(Jobverlauf 30 Tage, Sitzungen anders) und brauchten eine UNION mit Regeln je Quelle. Eine kleine
Tabelle mit einer Aufbewahrung ist einfacher. Das steht so in ADR 0019.

**Tabelle** (Migration **00031** nach Vorlage, `STRICT` wie 00027 und 00030, Zeit als INTEGER
Unix-Millisekunden wie überall):

```
activity_events(
  id INTEGER PRIMARY KEY,
  at INTEGER NOT NULL,             -- Unix ms, UTC
  kind TEXT NOT NULL,              -- bekannte Werte siehe unten, geprüft in Go, kein CHECK
  account_id INTEGER, profile_id INTEGER,
  item_id TEXT, library_id TEXT,
  subject TEXT,                    -- Titel bzw. Bibliotheksname zum Zeitpunkt des Ereignisses
  detail TEXT NOT NULL DEFAULT '{}' -- JSON, nur die Felder aus ActivityEntry.detail
) STRICT
INDEX activity_events_at ON activity_events(at DESC)
INDEX activity_events_account ON activity_events(account_id)
```

Keine Fremdschlüssel auf Titel und Bibliotheken: ein Eintrag überlebt das Löschen eines Titels
oder einer Bibliothek, `subject` hält den Namen fest. **Konto und Profil dagegen werden nicht
festgehalten:** E-Mail und Profilname liest der Server beim Lesen über `account_id`/`profile_id`
dazu, und beim Löschen eines Kontos oder Profils (`auth.DeleteAccount`, `accounts.go:110`;
`auth.DeleteProfile`, `profiles.go:460`) löscht er dessen Einträge mit. So bleibt nach dem
Löschen eines Kontos nichts von seinem Schauen im Protokoll. Kein Geheimnis, kein Pfad, keine
IP-Adresse und kein freier Fehlertext im Protokoll (Regel 10): `detail.reason` nimmt nur feste
Codes (`root_missing`, `root_empty`, `loss_over_threshold`, `cancelled`, `error`).

**Ereignisarten** (`kind`) und wo sie entstehen:

| `kind` | Quelle | Regel gegen Flut |
|---|---|---|
| `playback_started` | Ende von `playback.Start` (`server/internal/playback/sessions.go:104`), nach der Prüfung auf Abmeldung (`:224-229`), ohne auf den Schreibvorgang zu warten (`context.WithoutCancel`, eigene Goroutine), damit die Zeit bis zum ersten Bild nicht wächst | nur, wenn dasselbe Profil denselben Titel in den letzten **30 min** nicht schon gestartet hat (Spur-, Qualitäts- und Versionswechsel sind neue Sitzungen) |
| `playback_completed` | Fortschritt (`playback.go:300`), wenn ein Titel **neu** als gesehen gilt (90-%-Regel, vorher nicht gesehen) | einmal je Übergang |
| `playback_stopped` | nur Stopp durch die Verwaltung und Widerruf (`StopStream`, `EndDevice`, `devices.go:121-140`; `stopped_by` `admin`/`revoked`); diese beenden mehrere Sitzungen in einem `UPDATE`, darum mit `RETURNING` je Sitzung eine Zeile | Stopp durch den Zuschauer und Abmelden schreiben nichts |
| `scan_started` | `library/scanner.go`, vollständiger Scan einer Bibliothek; `detail.trigger` (`manual`, `schedule`, `watcher`) | Pfad-Scans (Dateiwächter, Webhook, `pathscan.go:99`) schreiben nichts |
| `scan_done` | `finish` (`scanner.go:395`), vollständiger Scan; `detail.newTitles` | wie oben |
| `scan_failed`, `scan_aborted` | `endScan` (`scanner.go:319`); Abbruch von Hand in `runCancellable` (`scanner.go:221-243`) mit `reason = cancelled` | auch für Pfad-Scans, aber höchstens einer je Bibliothek und Stunde |
| `job_failed` | zweiter Haken an `worker.OnFinalFailure` (`server/internal/jobs/worker.go:63`, heute für `notifySvc.JobFailed`); `detail.jobType`, `detail.count` | zusammengefasst: **eine** Zeile je Jobart und Stunde, deren `count` hochzählt |
| `backup_created`, `backup_restored` | `admin.CreateBackup` (`backup.go:213`, mit `detail.sizeBytes`), `admin.restore` (`backup.go:548`) | – |
| `server_started` | `main.go`, nach «listening» (`:403`), mit `detail.version` | – |
| `server_restart` | `restartServer` (11.5), mit Konto | – |

**Schreiben:** Der Schreiber ist `(*admin.Service).RecordActivity(ctx, model.ActivityEvent)`.
`playback`, `library` und `jobs` bekommen ihn wie die heutigen Haken als `WithActivity(func)` bzw.
`OnFinalFailure` in `main.go` (Muster `WithScanAborted`, `main.go:282`); kein neues Paket
importiert `store/systemq` (Regel 2 bleibt unberührt). Ein Fehler beim Schreiben wird als WARN
geloggt und **bricht die eigentliche Handlung nie ab**.

**Aufräumen:** `jobs.prune` (`server/internal/jobs/history.go`) bekommt einen Haken für weitere
Aufräumarbeiten; `admin` meldet dort das Löschen der Einträge an, die älter als **30 Tage** sind,
und hält höchstens **2'000** Zeilen, in Stapeln wie `PruneJobs`. So kennt `jobs` die neue Tabelle
nicht, und es entsteht keine neue Jobart und keine neue Karte auf «Hintergrundjobs».

**Lesen:** nur über `Dashboard.activity` (die sechs neuesten). Wer das Dashboard sieht, sieht das
Protokoll: jedes Administratorkonto mit Hauptprofil (`x-visibility: admin`). Eine eigene Seite mit
dem ganzen Protokoll baut dieses Konzept **nicht** (nicht verlangt; spätere Idee).

### 11.5 Neustart

- **Server:** Die Neustart-Funktion, die heute nur die Wiederherstellung bekommt (`WithRestart`,
  `backup.go:109`, verdrahtet als `requestRestart`, `main.go:224-228, 288`), wird eine Option von
  `admin.Service` für beide Wege. Neu ist ein Merker `restartPending` (`atomic.Bool`) im
  Sicherungszustand (`backupState`, `backup.go:75-81`), den Wiederherstellung und Neustart setzen.
  `restartServer` versucht `mu.TryLock()`: hält eine laufende Sicherung die Sperre, antwortet er
  **409 `backup_busy`** (sonst würde der Neustart die Sicherung abschneiden); ist der Merker schon
  gesetzt, **409 `restart_pending`**. Die Wiederherstellung antwortet ebenso mit 409
  `restart_pending`, solange der Merker steht. Sonst schreibt `restartServer` `server_restart` ins
  Protokoll, plant den Neustart wie die Wiederherstellung mit `time.AfterFunc(1s, …)` und antwortet
  sofort 202 mit `restartAt`. Der Ablauf danach ist der heutige: geordnet herunterfahren,
  `syscall.Exec` auf das eigene Programm (gleiche PID); nur wenn das scheitert, Exit 0 und Docker
  startet neu (`restart: unless-stopped`, `deploy/compose.yaml:21`).
- **Oberfläche:** Knopf «Neu starten» in der Karte Server, `ConfirmDialog` mit Satz je Lage
  («2 laufende Streams brechen ab. Laufende Downloads für unterwegs werden unterbrochen.» bzw.
  «Gerade läuft nichts.»). Danach ersetzt die Warteanzeige das Kartenraster (11.1). Die Anzeige
  `RestartPanel` zieht aus `BackupPage.tsx:260-334` in eine eigene Datei
  `features/admin/RestartPanel.tsx` und wird allgemein: Eingaben `{acceptedAt, title, body, notice?,
  onBack}`. Die Wiederherstellung übergibt wie heute ihre Texte (Version, Geheimnisse) und
  `reloadToLogin()`, weil die zurückgeholte Datenbank eigene Sitzungen hat. **Der Neustart
  übergibt `location.reload()`**: die Sitzung bleibt gültig, Marco landet wieder auf der
  Übersicht. `restart.ts` bleibt. Laufende Wiedergaben brechen ab; der Zuschauer startet den Titel
  neu, die Position kommt aus dem gespeicherten Fortschritt.
- **Test:** Der Vertrags- und der Handlertest laufen mit einer Attrappe für die Neustart-Funktion;
  auch der Eintrag `restartServer` in `adminRegistry` (`contract_access_test.go:187` ff.)
  verdrahtet die Attrappe, damit der Zugriffstest keinen Neustart auslöst. Kein Test startet den
  Prozess wirklich neu.

### 11.6 Oberfläche im Detail

- **Neuer Baustein `ui/AdminSideNav`** (senkrechte Leiste) und **`ui/AdminSideNavMenu`** (Blatt am
  Handy). Die heutige horizontale `ui/AdminNav` bleibt bis Schritt 5 unverändert in Gebrauch und
  wird dort mit ihren Stories und den drei Referenzbildern `ui-adminnav--*` gelöscht. So bleibt
  `task ci` auch nach Schritt 4 grün.
- **`AdminSideNav`:** `nav` mit `aria-label` «Verwaltung», Gruppentitel als `span` im Band mit
  `id`, die Liste darunter mit `aria-labelledby` (wie heute `AdminNav.tsx:105-110`; keine
  Überschriften, damit vor dem h1 der Seite keine vier Überschriften stehen). Einträge mit Symbol,
  Zahl oder Pille, `aria-current="page"` am aktuellen. **Tastatur wie heute ein einziger
  Tab-Halt** (roving tabindex über alle 20 Einträge), aber senkrecht: **Pfeil hoch/runter**,
  Pos1/Ende, Enter folgt dem Link; Tab verlässt die Leiste zum Inhalt. Der Baustein ist rein
  darstellend: er bekommt Einträge, Zahlen und Pillen als Eingaben und liest keine Daten.
- **Container `features/admin/AdminShellNav.tsx`:** liest `dashboardQuery` (gleicher Schlüssel
  `['admin','dashboard']`, `queries.ts:329-333`), baut aus `adminSections` und den Zahlen die
  Eingaben für `AdminSideNav` bzw. `AdminSideNavMenu` und übernimmt `focusCarry` aus
  `AdminLayout`. Auf der Übersicht pollt die Seite alle 5 s wie heute; auf allen anderen
  Verwaltungsseiten pollt der Container alle **30 s**. Ohne Daten (laden, Fehler) zeigt die Leiste
  keine Zahlen, aber alle Einträge.
- **Gerüst (`AppLayout.tsx`):** `adminShell = isAdmin && pathname beginnt mit /admin`. Ist es wahr
  und breit, rendert das Layout `AdminShellNav` statt `SideNav` (`AppLayout.tsx:284`) und zählt die
  App-Leiste als «nicht gezeigt», damit das Menü-Symbol die Schublade öffnet
  (`AppLayout.tsx:141-145`). In dieser Schublade fehlt auf `/admin/*` der Knopf «Seitenleiste
  einblenden» (`show` nur ausserhalb von `/admin`, `AppLayout.tsx:297-301`), weil er dort nichts
  bewirkt.
- **Kein Springen beim Laden:** `index.html:11-12` setzt vor dem ersten Bild zusätzlich
  `data-shell="admin"`, wenn der Pfad mit `/admin` beginnt; `global.css` gibt dann ab 768 px die
  Leistenbreite frei, auch bei `data-sidenav="hidden"`. `AppLayout` hält das Attribut beim
  Seitenwechsel aktuell.
- **`AdminLayout.tsx`:** liefert im `PageNav` am Handy nur den Knopf «Verwaltung · *Seite*» mit dem
  Blatt; breit nichts (die Leiste steht im Gerüst).
- **Band-Farbe ohne neue Farb-Tokens (F-9):** Der Band-Selektor selbst definiert
  `--sc-band-tint: color-mix(in srgb, var(--sc-accent) 16%, transparent)` und benutzt
  `background-color: var(--sc-band-tint)` und `color: var(--sc-accent)`. Weil die Variable am Band
  und nicht an `:root` steht, färbt eine Story mit anderer Profilfarbe am Rahmen das Band richtig
  um. stylelint (`declaration-strict-value`, nur `var(--sc-…)` in Farbeigenschaften) bleibt
  zufrieden, weil die `color-mix`-Zeile eine eigene Variable ist. **Prüfung:** ein Unit-Test
  rechnet für **alle acht Profilfarben** den Kontrast der Schrift auf dem Band über `bg-elevated`
  aus und verlangt ≥ 4,5:1. Fällt eine Farbe darunter, nimmt das Band für diese Farbe
  `text-primary` als Schrift (Liste im Test, keine neue Farbe).
- **Symbole:** neu in `ui/Icon.tsx`: `grid` (Übersicht), `film` (Gruppe Bibliothek), `server`
  (Gruppe System, Karte Server), `power` (Neu starten), `disk` (Speicher), `activity` (Letzte
  Aktivität). Konten nimmt `users`, Kuratierung `star`, Hinweise `warning`. Pfade aus dem Prototyp
  (`11-verwaltung-prototyp.html`), gleiche Strichstärke wie die vorhandenen.
- **Weitere `ui`-Bausteine mit Story:** `ActivityFeed` (Zeilen mit Symbol, Satz, Zeit), `TaskRow`
  (Jobart, Gegenstand, `ProgressBar`, Restzeit), `StreamRow` (Poster, Titel, Profil · Gerät,
  Modus, Fortschritt), `DashCard` (Panel, das seine Zelle füllt: Kopf mit Symbol, Titel, optional
  Zahl; Fuss mit Link oder Knöpfen). Vorhandene `Panel`, `StorageMeter`, `StatTile`,
  `ProgressBar`, `StatusDot`, `ConfirmDialog` werden benutzt, nicht kopiert. Die heutigen
  `IntegrationsTile` und `WebhooksTile` (`AdminHomePage.tsx:157-216`) ziehen mit ihrem Inhalt in
  `DashCard` um.
- **Texte:** neue Schlüssel unter `admin.nav.*` (Zurück zur App, Blatt-Knopf, Zahlennamen),
  `admin.home.*` (Karten, Hinweise, Server-Protokoll, Neustart-Dialog), `admin.activity.*` (ein Satz
  je `kind` und ein allgemeiner) und `admin.restart.*`, je Deutsch und Englisch. Nicht mehr
  gebrauchte Schlüssel (`admin.home.intro`, Tabelle Fehlschläge, Bereichskarten)
  werden entfernt, nicht liegen gelassen.

### 11.7 Tests, die sich ändern

- **Server:** Vertragstest (`server/internal/api/contract_test.go`, `TestContractOpenAPI`),
  `TestContractCoversEveryOperation` mit `restartServer` (`contract_access_test.go:187` ff., Zugriff
  nur `admin`, Attrappe), Handlertests für alle neuen Felder, Test `paused` (Nebenbefund),
  Abfragetests für `ListRunningJobs` und `ListFailedJobsNewestPerType`, Migrationstest 00031 gegen
  den eingefrorenen Stand `testdata/db/metadaten.sql`, **neuer eingefrorener Stand
  `testdata/db/verwaltung.sql`** mit mindestens einer Zeile je bekannter Art und
  `frozen_verwaltung_test.go` (nach `testdata/db/README.md`, wie bei Konzept 09 und 10), Tests
  der Flut-Regeln (zweiter Start binnen 30 min schreibt nichts; Spurwechsel schreibt nichts;
  Pfad-Scan schreibt nichts; `job_failed` zählt hoch), Löschtest (Konto gelöscht → seine Einträge
  weg), Test «Schreibfehler im Protokoll bricht `playback.Start` nicht ab», Prune-Test (30 Tage,
  2'000), Neustart-Tests (202, 409 `restart_pending`, 409 `backup_busy`, Attrappe aufgerufen,
  Protokolleintrag), Zähler-Speicher (60 s, geleert nach Pflegeänderung).
- **Unit (web):** `AdminHomePage.test.tsx` umgeschrieben (acht Karten, Hinweise, Server-Protokoll
  zugeklappt, Neustart-Dialog, Warteanzeige statt Raster); der Block «admin navigation (A-2)»
  (`:410-509`) prüft die Leiste mit **20 Einträgen**, senkrechte Pfeile und den Wegfall der
  «Hauptnavigation» auf `/admin/*` (bisher `:481-491`); der Abzeichentest A-5 (`:514-528`, heute
  am Eintrag «Verwaltung» der App-Leiste auf `/admin/devices`) prüft das Abzeichen am Eintrag
  «Übersicht». Die Seitentests, die `navigation 'Verwaltung'` abfragen (`HomeRowsPage`,
  `PodcastsPage`, `SpotlightPage`, `CollectionsPage`, `PicksPage`, `DevicesStreamsPages`), behalten
  ihre Abfrage; sie rendern die Leiste jetzt aus dem Gerüst statt aus `AdminLayout` und brauchen
  dafür eine Testhilfe, die beides montiert, und Dashboard-Attrappen mit den neuen Feldern.
  `BackupPage.test.tsx` und `restart.test.ts` laufen gegen das allgemeine `RestartPanel`; Kontrasttest
  der Profilfarben; Test der Satzbildung je `kind` mit unbekannter Art.
- **e2e:** `admin-home.spec.ts` umgeschrieben (der A-3-Test steht heute bei `:167-199`): Leiste
  (Reihenfolge `NAV_PAGES` mit Profile und Benachrichtigungen, ein Tab-Halt, Pfeil hoch/runter,
  Pos1/Ende, Enter, Zurück), Karten, A-3 mit Trakt-Fehler in der Karte «Integrationen» (wie heute) und im
  aufgeklappten Server-Protokoll < 10 s, Neustart-Dialog bis «Abbrechen» (kein echter Neustart), Handy-Blatt,
  sichtbarer Fokus, keine Fokusfalle. **Anpassen** (Pfeil rechts → Pfeil runter,
  Tab-Reihenfolge): `admin-spotlight.spec.ts:48-80`, `admin-jobs.spec.ts:49-86`,
  `admin-libraries.spec.ts:28-29`, `admin-item.spec.ts:111-112`. **Prüfen, sollten unverändert
  laufen** (`aria-current` in `nav 'Verwaltung'`): `admin-integrations`, `admin-home-rows`,
  `admin-collections`, `admin-podcasts`, `admin-picks`, `admin-metadata`; h1 «Verwaltung»:
  `admin-webhooks`, `admin-spotlight`, `admin-jobs`; Kopf-Eintrag «Verwaltung»: `profiles`,
  `invitations`, `sidenav`. `sidenav.spec.ts` bekommt einen Fall «`/admin` bei ausgeblendeter
  App-Leiste: Verwaltungsleiste da, kein Springen» (Vorlage `:306`).
- **Storybook-Referenzbilder** (`web/e2e-storybook/__screenshots__/`, auf CT 190 erzeugt, Weg ohne
  Docker): neu `ui-adminsidenav--*` (`Default`, `Counts`, `ArrowKeys` mit `press:Tab+ArrowDown`,
  `Accents` mit zwei Profilfarben), `ui-adminsidenavmenu--phone`, `ui-activityfeed--*`,
  `ui-taskrow--*`, `ui-streamrow--*`, `ui-dashcard--*`; geändert `ui-icon--all`; gelöscht in
  Schritt 5 `ui-adminnav--default`, `--arrow-keys`, `--narrow`.

### 11.8 Schnitt in Schritte

Jeder Schritt ein Commit auf einem Zweig ab `main` `6d562b8` (oder dem dann aktuellen `main`),
`task ci` grün (inklusive e2e), DoD-Belege in `docs/dev/CURRENT_STATUS.md`.

```
0 Papiere → 1 Vertrag 1.7.0 und Dashboard-Felder (+ Fix paused)
  → 2 Aktivitätsprotokoll (00031, eingefrorener Stand) → 3 Neustart
  → 4 ui-Bausteine mit Stories (noch nicht eingebaut)
  → 5 Gerüst, Leiste, Übersicht, alle Tests, alte AdminNav weg
  → 6 Abnahme auf CT 190, Messung, Doku
```

**Schritt 0 — Papiere.** Frage 17 ins Entscheidungsprotokoll (F-1 bis F-9, O-1, die Abweichungen 1, 2, 4,
5 und 6 aus 11.2; A-3 und Entscheid 10 (e) bleiben unberührt); ADR 0019 «Verwaltungsleiste, Aktivitätsprotokoll, Neustart» (Teilnachfolger von
Entscheid 14 N-2 für `/admin/*`; ergänzt ADR 0009 um das Aktivitätsprotokoll neben dem Fehlerring
und grenzt es gegen das Webhook-Ereignisprotokoll aus ADR 0010 ab; geprüfte Alternative und
Datenschutz aus 11.4); dieses Papier auf den dann aktuellen `main` verankern. DoD: Doku-Commit,
`task ci` grün. **0,5 Tage.**

**Schritt 1 — Vertrag 1.7.0 und Dashboard-Felder.** Zuerst eigener Commit «fix(api): map paused
job count on the dashboard» mit Test. Dann OpenAPI (11.3 ohne `activity` und `restartServer`),
`task gen`, Abfragen `ListRunningJobs` und `ListFailedJobsNewestPerType`, `AdminStreamItems` mit
Serien-Id, `jobs.active`, `jobs.nextRun`, `jobs.failedByType`, `posterUrl`,
`attention.abortedScans`, `counts` mit 60-s-Speicher und Leeren, Handler, Tests; Dashboard-Attrappen
in den Web-Tests um die Pflichtfelder ergänzen. Die Oberfläche ändert sich noch nicht. DoD:
Vertrags- und Handlertests grün, Antwort von `GET /admin/dashboard` als Beleg. **2,5 Tage.**

**Schritt 2 — Aktivitätsprotokoll.** Migration 00031 mit Test, eingefrorener Stand
`verwaltung.sql` mit Test, `RecordActivity` mit den Flut-Regeln, Haken in `playback`, `library`,
`jobs` (`OnFinalFailure`), `admin` (Sicherung) und `main.go`, Löschen mit Konto und Profil,
Aufräum-Haken in `jobs.prune`, `Dashboard.activity` im Vertrag, Tests (11.7). DoD: nach einem
lokalen Scan und einer Wiedergabe stehen die Einträge im Dashboard (Beleg: Antwort). **3 Tage.**

**Schritt 3 — Neustart.** `restartServer` im Vertrag, Neustart-Funktion als Option von
`admin.Service`, `restartPending` und `TryLock`, 409-Codes auch für die Wiederherstellung,
Protokolleintrag, `RestartPanel` allgemein und ausgelagert (Sicherung benutzt es weiter), Tests.
DoD: lokaler Neustart über `curl` mit Beleg (202, zweiter Aufruf 409, Server wieder da, Einträge
`server_restart` und `server_started`). **1,5 Tage.**

**Schritt 4 — ui-Bausteine.** Symbole, `AdminSideNav` mit Band, Zahlen, Pillen und senkrechtem
Tastaturmodell, `AdminSideNavMenu`, `DashCard`, `TaskRow`, `StreamRow`, `ActivityFeed`, je Story;
Kontrasttest der acht Profilfarben. Noch nicht im Gerüst eingebaut; die alte `AdminNav` läuft
weiter. DoD: `task ci` grün, Referenzbilder auf CT 190 erzeugt. **3 Tage.**

**Schritt 5 — Gerüst, Leiste, Übersicht.** `AdminShellNav`, `AppLayout`, `index.html`,
`global.css`, `AdminLayout`, `AdminHomePage` neu nach 11.1, Neustart-Dialog und Warteanzeige,
Bedingung «Verlust akzeptieren» angleichen, Texte de/en, alte `AdminNav` mit Stories und
Referenzbildern löschen, alle Tests aus 11.7. DoD: `task ci` grün in Chromium und Firefox,
Tastaturtest der Übersicht und der Leiste grün, Regel 9 (Lint) grün. **3,5 Tage.**

**Schritt 6 — Abnahme und Doku.** Image bauen und auf CT 190 bringen (Weg aus dem CT-190-Stand),
Abnahme durch Marco am Desktop (mit und ohne ausgeblendete App-Leiste) und am Handy; Messung:
Dauer von `GET /admin/dashboard` mit `counts` (p95 kalt und warm) und Zeilen in
`activity_events` nach einem Tag in `docs/perf/verwaltung.md`; Prototyp gegen die gebaute Seite
abgleichen; `CURRENT_STATUS.md`, Release-Notes 1.7.0. **1 Tag.**

**Aufwand (Arbeitstage, Schätzung):** 0: 0,5 · 1: 2,5 · 2: 3 · 3: 1,5 · 4: 3 · 5: 3,5 · 6: 1 ·
**Summe 15** (Spanne 14 bis 18). Fassung 2 schätzte B auf 10 bis 14 Tage plus 3 bis 4 für
Protokoll und Neustart.

### 11.9 Offene Punkte

| Nr. | Punkt | Vorschlag |
|---|---|---|
| O-1 | Ohne Kachel «Integrationen und Webhooks» hätten A-3 eine neue Lesart gebraucht und die Webhooks-Kachel aus Entscheid 10 (e) wäre entfallen. | **Entschieden 1. November 2026: Marco «Nein, behalten».** Integrationen und Webhooks bleiben eigene Karten, die Übersicht hat acht Karten (2 × 4, 11.1). |
| O-2 | Doppelte Nennung: Fehlschläge stehen in der Karte Aufgaben **und** als Hinweis. | Gewollt: die Karte zeigt **was** fehlschlägt (je Jobart), der Hinweis sagt **dass** etwas zu tun ist. G-8 gilt hier nicht streng. Der Bau klärt es am Bild. |
| O-3 | Profile und Benachrichtigungen liegen unter `/settings/*`; ein Klick in der Leiste verlässt die Verwaltung. | So lassen; die Seiten gelten für jedes Konto, nicht nur die Verwaltung. |

### 11.10 Risiken

- **Grösster Umbau des Gerüsts seit Konzept 08:** jede Verwaltungsseite ändert ihre Navigation; die
  Tests in 11.7 sind die Liste, die vollständig grün werden muss. Schritt 4 baut die Leiste darum
  zuerst allein mit Stories.
- **Pflegezähler auf grossen Beständen:** fünf Zählabfragen über `items`; mit 60-s-Speicher selten,
  aber der erste Aufruf nach dem Start ist kalt. Schritt 6 misst; ist er über 200 ms, liefert der
  Server beim ersten Mal `counts` ohne Pflegezahl und rechnet im Hintergrund.
- **Protokoll wird trotz Regeln laut:** die Flut-Regeln in 11.4 sind eine Schätzung. Zeigt der
  Beobachtungstag auf CT 190 sechs gleichartige Zeilen, wird die Regel der Art enger; die
  Obergrenze 2'000 hält die Tabelle in jedem Fall klein.
- **Neustart während eines Streams oder Downloads:** der Dialog nennt beides; der Zuschauer startet
  den Titel neu, die Position bleibt.

## 12. Review-Protokoll

### Fassung 1 → 2 (OMC-Kritiker, 31. Oktober 2026)

Befund: 1 Blocker, 8 Major, 8 Minor. Alle Zeilenangaben der Fassung 1 stimmten.

| Befund | Was Fassung 2 tut |
|---|---|
| **B-1** Prototyp ohne die Seitenleiste der App; B hätte zwei Leisten | Abschnitt 1.3, G-1 an der Inhaltsbreite; Prototyp mit Schalter; B ersetzt die App-Seitenleiste wie Emby. |
| M-1 `stat.Dev` gibt es beim `statfs` nicht | 6.2: `os.Stat(…).Sys().(*syscall.Stat_t).Dev`, Entscheid im Server als `storage.shared`, `storage_other.go`, Grenze btrfs/ZFS. |
| M-2 `jobs.running` ist schon eine Zahl | 6.2: `jobs.active`; neue Abfrage für den neuesten Fehler je Jobart. |
| M-3 D liess Webhooks, Integrationsstatus, Wartend, Server-Warnungen, Entwicklungsstand und die Ansage weg | D mit Karten Integrationen und Webhooks, Wartend immer; G-5 erweitert und als Live-Region. |
| M-4 Abgebrochener Scan ohne Daten | 6.2 `attention.abortedScans`, Abweichung des Codes von A2-N12 in Abschnitt 2, F-8. |
| M-5 Fehlerliste ist das Server-Log, nicht nur Jobs | P-4 berichtigt; F-3 mit Empfehlung «zugeklappt auf der Übersicht», A-3 bleibt. |
| M-6 Poster nicht «schon vorhanden» | 6.2 `AdminStream.posterUrl`. |
| M-7 D wiederholte Angaben, Symmetrie zu hoch bewertet | Kennzahlenreihe in D gestrichen, G-8, Bewertung ●●. |
| M-8 Konzept 10 und Navigationstests fehlten | Kopf und 7: 18 Seiten, Reihenfolge 10 vor 11; Tastaturmodell, Referenzbilder und alle Specs in 8. |
| Minor 1–8 | Zustand «Alles gut»; tote Links entfernt; Verlauf am Rand der Reiter; Hilfstext entfernt; P-1 Ursache berichtigt; D-Beschreibung = Prototyp, neuester Fehler je Jobart; 19,4 GB; Texte und Seiten ausserhalb `/admin` in 7 und 8. Fragen F-2/F-3 getrennt, F-4 ohne Technik. |

### Fassung 2 → 3 (OMC-Kritiker, 1. November 2026)

Geprüft wurde Abschnitt 11. Befund: kein Blocker, 9 Major, 24 Minor. Die Zeilenangaben stimmten
bis auf wenige; die Nummern Frage 17, ADR 0019, Migration 00031 und Vertrag 1.7.0 sind frei.

| Befund | Was Fassung 3 tut |
|---|---|
| M-1 Schritt 4 hätte `AdminNav` umgebaut, während `AdminLayout` sie noch horizontal braucht (CI rot) | Neuer Baustein `AdminSideNav`; die alte `AdminNav` fällt erst in Schritt 5 (11.6, 11.8). |
| M-2 Ereignisarten passten nicht zum Prototyp | `playback_completed`, `scan_started`, `job_failed` (zusammengefasst), `detail` als Objekt mit Zahlen; verbleibende Abweichung als Punkt 6 in 11.2. |
| M-3 Flut durch Spurwechsel und Pfad-Scans | Flut-Regeln je Art (11.4), Webhooks nicht im Protokoll. |
| M-4 `RestartPanel` war auf die Wiederherstellung zugeschnitten | Allgemeine Eingaben, Neustart lädt die Seite neu statt zur Anmeldung; Ort auf der Übersicht (11.1, 11.5). |
| M-5 409 ohne Zustand | `restartPending`, `TryLock`, Codes `restart_pending` und `backup_busy` (11.5). |
| M-6 kein eingefrorener DB-Stand | `testdata/db/verwaltung.sql` mit Test (11.7, Schritt 2). |
| M-7 Alternative nicht geprüft, Namenskollision | Abschnitt «Warum eine eigene Tabelle»; Name «Aktivitätsprotokoll»; ADR 0019 grenzt gegen ADR 0010 ab. |
| M-8 geschlossenes Enum | `kind` als String ohne Enum und ohne CHECK, allgemeiner Satz für Unbekanntes. |
| M-9 Datenabfrage in `ui/` | Container `features/admin/AdminShellNav.tsx`, `ui` bleibt darstellend. |
| Minor 1–24 | Scheduler über `WithJobControl`; 20 Einträge; 18 Karten; Abschnitt 2 statt 4 in Konzept 09; «umschreiben» statt «neu»; Schema `RestartAccepted`; Pflicht- und optionale Felder getrennt; fünf billige Pflegezähler ohne `rating_mismatch`; Speicher leeren; Serien-Id in `AdminStreamItems`; strengere Bedingung und Angleichung; Abbruch in `runCancellable`; nur feste Fehlercodes; `RETURNING` für Mehrfach-Stopps; Start-Eintrag am Ende von `Start`, ohne Warten; Aufräum-Haken statt Kopplung; INTEGER-Zeit und `STRICT`; Löschen mit Konto und Profil, Namen per Join; Abzeichentest A-5; Knopf «Seitenleiste einblenden» auf `/admin` weg; Aussage zur räumlichen Navigation gestrichen; `--sc-band-tint` am Band; `span` statt `h2`; Beschreibung von `getDashboard`, eine Ansage je Fehler, ADR 0010, Aufwand 15 Tage. |
