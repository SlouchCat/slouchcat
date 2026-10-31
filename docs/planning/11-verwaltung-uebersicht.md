# Konzept: Übersicht der Verwaltung (Seite `/admin`)

- **Status: Fassung 2, zur Auswahl durch Marco.** Marco hat am 31. Oktober 2026 am Bildschirmfoto
  der heutigen Übersicht gesagt: «Ich bin mit der Übersicht der Verwaltungsseite überhaupt nicht
  einverstanden. Es soll einfach, übersichtlich, symmetrisch und klar sein.» Die Struktur darf sich
  ändern, das Aussehen muss zum Rest der App passen; Emby dient als Vorbild. Dieses Papier legt
  **vier Varianten** vor (Abschnitt 5) und einen klickbaren Prototyp
  (`docs/planning/mockups/11-verwaltung-prototyp.html`, Abschnitt 9). Der OMC-Kritiker hat
  Fassung 1 geprüft (1 Blocker, 8 Major, 8 Minor; Abschnitt 11); Fassung 2 arbeitet alle ein.
  **Marco hat am 31. Oktober 2026 Variante B «Seitenleiste wie Emby» gewählt**, mit dem Wunsch,
  die Gruppentitel deutlich stärker und farbig zu markieren (Abschnitt 10.1). **Alle Fragen F-1
  bis F-9 sind beantwortet** (Abschnitt 10.2). Die genauen Umsetzungsschritte für B folgen in
  Fassung 3. **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Datum: 31. Oktober 2026, Fassung 2 vom selben Tag**
- **Wogegen dieses Konzept verankert ist:** `main`, Stand **`d3d73bc`** («Merge pull request #57»),
  Vertragsfassung **1.5.0**. Konzept 10 (Metadaten) ist gemergt, aber nicht umgesetzt; es hat
  **Vertrag 1.6.0, ADR 0018, Frage 16, Migrationen 00028–00030** reserviert und bringt eine
  18. Verwaltungsseite «Metadaten» in der Gruppe Bibliothek (`10-metadaten-einstellungen.md:627-630`).
  Dieses Papier setzt **10 vor 11** voraus. Die nächsten freien Nummern sind darum **Frage 17,
  ADR 0019 (nur falls nötig), Vertrag 1.7.0**. Wird 11 vor 10 gebaut, tauschen die beiden ihre
  Vertragsfassungen; das legt Schritt 0 fest. Eine Migration braucht keine Variante.
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

Die genauen Schritte schreibt Fassung 3, wenn Marco gewählt hat. Grob:

0. Frage 17 ins Entscheidungsprotokoll (Lesart A-3 für Speicher und Fehlerliste, Abweichung von
   Konzept 09 Abschnitt 4 Punkt 4, A2-N12), Reihenfolge zu Konzept 10, Idee «Aktivitätsprotokoll
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
| F-2 | **Ja, wie empfohlen.** | Tabelle mit Erneut/Abbrechen nur auf «Hintergrundjobs»; Übersicht je Jobart eine Zeile mit Link. Abweichung von Konzept 09 Abschnitt 4 Punkt 4, in Frage 17. |
| F-3 | **Zugeklappt unten, wie empfohlen.** | Abschnitt «Server-Protokoll» ganz unten, zugeklappt mit Anzahl; Abnahme A-3 unverändert. |
| F-4 | **Nein, immer zwei Balken** (abweichend von der Empfehlung). | `/config` und `/cache` bleiben getrennt, auch bei gleichen Zahlen. Das Feld `storage.shared` (6.2) und G-6 entfallen; A-3 bleibt wörtlich erfüllt. |
| F-5 | **Ja, wie empfohlen.** | `jobs.active`, `jobs.nextRun` (6.2). |
| F-6 | **Beides bauen** (abweichend von der Empfehlung), nach Erklärung: «Neu starten soll unterstützt werden. Letzte Aktivität soll ebenfalls erstellt werden.» | Neues Ereignisprotokoll im Server (Tabelle, Einträge bei Wiedergabe, Scan, Sicherung, Webhook, Serverstart; Aufräumen alter Einträge) und Neustart-Endpunkt mit Rückfrage, weil laufende Streams abbrechen. Beides ist neue Funktion: braucht eine Migration (00031), Vertrag 1.7.0 und vermutlich ADR 0019; in Frage 17. Rund 3 bis 4 Tage zusätzlich. |
| F-7 | **Wenige, wie empfohlen** (nach Beispiel im Prototyp, Schalter «Zahlen»). | Zahlen in der Verwaltungsleiste nur bei Bibliotheken (Titel), Pflege (offene Fälle, Warnfarbe), Konten, Integrationen (eingerichtet / gesamt), Streams und Hintergrundjobs (laufend); Alarme als farbige Pille (Fehlschläge, stummer Webhook). Braucht wenige billige Zähler im Dashboard, kein Zählerobjekt für alle 18 Seiten. |
| F-8 | **Ja, wie empfohlen.** | Hinweis «Scan abgebrochen» in der Karte Hinweise mit Link zu Bibliotheken; `attention.abortedScans` (6.2). |
| F-9 | **3 «Band».** | Keine neuen Farb-Tokens; Titel als getöntes Band in der Profilfarbe, Einträge eingerückt. |

## 11. Review-Protokoll

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
