# Konzept: Feinschliff an Startseite, Verwaltung und Titelpflege

- **Status: Fassung 3 (6. November 2026), Kritiker-Review eingearbeitet (REVISE, 1 Blocker,
  10 Major, 6 Minor; Abschnitt 14).** Marco hat am 5. November 2026 alle Fragen **F-1 bis
  F-16** beantwortet (Abschnitt 11). Abweichend von der Empfehlung: **Spotlight C «Endlosband»**
  (F-6), **Editor als Dialog** wie Emby (F-8), **Zeitfenster für schwere automatische Jobs schon
  in diesem Konzept** (F-5), auf der Startseite **Kachelreihe «Meine Medien» und je Bibliothek
  eine Reihe «Neu in …»** (F-11, nach einem dritten Mockup) und **höchstens 15 Reihen** (F-12).
  Wo die Abschnitte 2 bis 9 noch die Empfehlung aus Fassung 1 beschreiben, sind sie auf die
  Antwort umgeschrieben. **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Auslöser:** Marco hat am 5. November 2026 nach der Abnahme von Release 1.13.0 auf CT 190 acht
  Wünsche geschickt, mit Bildschirmfotos (Abschnitt 1). Für Punkt 5 (Spotlight) und Punkt 6
  (Metadaten bearbeiten) wollte er je ein **klickbares Mockup**, für Punkt 7 (Bibliotheken) bei
  den Fragen ein drittes (Anhang A).
- **Verankert gegen:** `main` **`d06a40e3`** («Merge pull request #96», Release 1.13.1),
  Vertragsfassung **1.13.0** (`api/openapi.yaml:4`; 1.13.1 hat den Vertrag nicht geändert),
  letzte Migration **`00033_keyframe_source.sql`**, letzte ADR **`0023`**, letzte Frage im
  Entscheidungsprotokoll **21**.
- **Nummern:** Dieses Papier ist **Konzept 15**, reserviert **Frage 22** und **ADR 0024**
  (Updater, Abschnitt 9). Vorläufig gelten je Stufe (Abschnitt 12) **Vertrag 1.14.0, 1.15.0 und
  1.16.0** und **Migration 00034 und 00035**; sie werden beim Baubeginn in Bau-Reihenfolge
  endgültig vergeben (goose verlangt lückenlose
  Reihenfolge, die Regel «Vertrag = Tag» eine aufsteigende Version).
- `:Zeile`-Angaben beziehen sich auf `d06a40e3`; **nach Namen suchen, nicht nach Zeile.** Pfade
  ohne Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App), wo es eindeutig ist.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
  Wo dieses Papier einen früheren Entscheid ändert, steht das in Abschnitt 10 und als Frage in
  Abschnitt 11; erst Marcos Antwort ändert ihn.

> **Dieses Papier ändert keine Zeile Code.** Es beschreibt acht kleine bis mittlere Änderungen an
> verschiedenen Orten, sagt je Punkt, was der Code heute tut, und schlägt eine Lösung vor. Die
> Punkte hängen kaum zusammen; Abschnitt 12 ordnet sie in vier Stufen, die einzeln erscheinen
> können.

## 1. Was Marco will

| Nr. | Marcos Wunsch (5. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| P-1 | Bei «Weiterschauen» nicht die Folge zeigen, sondern die Serie; ein Klick führt zur letzten Folge. | 2 |
| P-2 | Bei den Hintergrundjobs ein Kommentar, der den Job beschreibt; es muss übersichtlich bleiben. | 3 |
| P-3 | Bei den Hintergrundjobs: Gibt es einen Grund, weshalb gewisse Jobs nicht einplanbar sind? | 4 |
| P-4 | Ein Klick auf «Benachrichtigungen» in der Verwaltung wirft einen aus der Verwaltung. | 5 |
| P-5 | Im Spotlight soll das aktive Bild ganz links bleiben; beim Weiterlaufen bleiben die Bilder links bündig. Vorher ein klickbares Mockup. | 6 |
| P-6 | Als Server-Administrator auf der Übersicht, der Film- und der Serienseite mit einem Stift die Metadaten (Bilder, Cover usw.) anpassen, nach dem Vorbild von Emby. Mit klickbarem Mockup. | 7 |
| P-7 | Auf der Startseite auch die einzelnen Bibliotheken auswählen und anzeigen können. | 8 |
| P-8 | Auf der Karte «Server» einen Knopf «Update installieren», der die neueste Version installiert. | 9 |

## 2. P-1 «Weiterschauen» je Serie

### 2.1 Heute

- Die Reihe kommt aus `ListContinueWatching` (`server/queries/scoped/items.sql:17-24`): jeder
  Titel mit `completed = 0` und `position_ms > 0`, ohne Extras, nach `progress.updated_at`
  absteigend, höchstens 20 (`MaxRowItems`, `visibility.go:32`). **Es gibt keine Gruppierung je
  Serie**: zwei angefangene Folgen derselben Serie ergeben zwei Karten.
- Eine Folge kommt als `ItemRef` mit ihrem eigenen Titel («Donuts im Stau»), `seriesTitle`,
  `seasonNumber`, `episodeNumber` und `parentId` = **Staffel**. Ein Feld `seriesId` und ein
  Serienposter gibt es im Vertrag nicht (`ItemRef`, `openapi.yaml:8173`).
- Die Karte (`HomePage.tsx`, `posterRow`, 208-287) zeigt `item.title` und nur
  `images.poster`. **Folgen haben kein Poster**, nur ein Folgenbild als `backdrop`
  (`metadata/images.go:146-154`). Darum zeigt jede Folge den grauen Platzhalter
  (`PosterFallback`); auf Marcos Foto sind alle sieben Karten mit Bild Filme.
- Ein Klick führt zur Detailseite der Folge (`paths.item`), dort steht «Fortsetzen».
- Ist eine Folge zu ≥ 90 % gesehen (`playback.go:280-284`), fällt sie aus der Reihe, und mit ihr
  die Serie, bis die nächste Folge angefangen ist. Eine Reihe «Als Nächstes» wie in Emby gibt es
  nicht. Die Regel für die nächste Folge gibt es schon (`GetNextVisibleEpisode`,
  `items.sql:131-142`), ebenso eine Gruppierung je Serie für die automatischen Downloads
  (`offline/sync.go:282-296`).
- Die Server-Vorabmalerei (`api/preload.go:147-153, 237-260`) malt die Poster der ersten Reihe,
  wenn kein Spotlight vorne steht, und bricht beim ersten Titel ohne Poster ab. Steht
  «Weiterschauen» vorne, malt sie heute nichts.

### 2.2 Vorschlag

1. **Eine Karte je Serie, eine Regel.** Der Server nimmt je Serie **den Fortschritt mit der
   jüngsten Aktivität**, gesehen oder nicht (`ROW_NUMBER() OVER (PARTITION BY series_id ORDER BY
   updated_at DESC)` über alle `progress`-Zeilen der Folgen; die Serie über `parent_id` der
   Staffel). Ist diese Folge **nicht fertig**, zeigt die Karte sie (Punkt 2 und 3). Ist sie
   **fertig**, zeigt die Karte die nächste Folge (Punkt 4) oder fällt weg. Eine ältere, halb
   gesehene Folge zählt nicht, wenn danach eine andere Folge zu Ende gesehen wurde. Sortiert
   wird über alle Karten nach dieser jüngsten Aktivität. Die Abfrage bleibt in `queries/scoped/` mit
   `visible_items` (Regel 2); die Serie muss für das Profil ebenfalls sichtbar sein, sonst fällt
   die Karte weg (nicht sichtbar heisst nicht da). Filme, Hörbücher und Podcastfolgen bleiben,
   wie sie sind.
2. **Die Karte zeigt die Serie:** Serienposter, Serientitel, darunter klein «S4 · F7 · Noch
   20 min» und der Fortschrittsbalken der Folge. Der Vertrag bekommt dafür additiv am `ItemRef`
   ein Feld **`series`** (`{ id, title, images }`, nur bei Folgen gesetzt); `seriesTitle` bleibt
   für alte Clients.
   *Gebaut (Stufe 2):* Das Feld heisst **`seriesRef`** (Schema `SeriesRef`), weil `ItemDetail.series`
   schon existiert und dort ein ganzes `ItemRef` ist.
3. **Klick** (F-1, entschieden): führt zur **Detailseite der Folge** mit «Fortsetzen» im Fokus,
   wie heute bei Folgen. Kein eigener Abspielknopf auf der Karte.
4. **Fertig gesehen → nächste Folge** (F-2, entschieden): Ist die zuletzt gesehene Folge fertig
   und gibt es eine nächste, bleibt die Serie in der Reihe mit der **nächsten Folge** («S4 · F8 ·
   Neu»), solange die letzte Aktivität höchstens **60 Tage** zurückliegt (gleiche Frist wie die
   automatischen Downloads). Sortiert wird nach der letzten Aktivität. Das ist die Regel
   `NextEpisode`, keine Empfehlung durch einen Algorithmus (Non-Goal bleibt unberührt). Ist die
   letzte Folge der Serie fertig, fällt die Serie aus der Reihe.
5. **Vorabmalerei:** `posterPrepaint` nimmt bei Folgen das Serienposter; damit malt auch eine
   «Weiterschauen»-Reihe an erster Stelle.

### 2.3 Was sich ändert

| Ort | Änderung |
|---|---|
| `queries/scoped/items.sql` | `ListContinueWatching` gruppiert je Serie; neue Abfrage oder Zweig für «nächste Folge» (F-2). |
| `visibility` | `ContinueWatching` liefert je Serie einen Eintrag; `summarize` füllt `Series` (Id, Titel, Bilder). |
| `api/openapi.yaml` | `ItemRef.series` (additiv), `SeriesRef`-Schema; Vertrag 1.14.0. |
| `HomePage.tsx` / `PosterCard` | Karte nimmt bei Folgen Serienposter und -titel, Unterzeile mit S/F. |
| `api/preload.go` | Serienposter für die Vorabmalerei. |
| Tests | `discover_test.go`, `contract_home_rows_test.go`, `HomePage.test.tsx`, `e2e/home.spec.ts`. |

Aufwand: rund **2 Tage**. Keine Migration.

## 3. P-2 Beschreibung je Hintergrundjob

### 3.1 Heute

Jeder Job hat in `locales/de.json` (`admin.jobs.types.<key>`, :3029-3126) nur `name` und
`trigger`. Der Text `trigger` («Zeitplan alle 6 h und Start des Servers, …») steht nur in der
aufgeklappten Zeile unter «Ausgelöst durch» (`JobsPage.tsx:1053-1060`). In der Tabelle sieht man
nur den Namen.

### 3.2 Vorschlag

- Neuer Text **`admin.jobs.types.<key>.description`**: **ein Satz, höchstens 70 Zeichen**, was
  der Job tut, in Marcos Worten. Er steht in der Tabelle als **zweite, gedämpfte Zeile unter dem
  Namen** (Grösse wie die Unterzeilen der Spalte «Wartend», einzeilig, bei Platzmangel mit «…»
  gekürzt). Die Zeile wird dadurch rund 14 px höher; die Tabelle bleibt eine Tabelle.
- In der aufgeklappten Zeile steht derselbe Satz ganz oben, ungekürzt, darunter wie heute
  «Ausgelöst durch».
- Am Handy (Tabelle als Liste) steht der Satz unter dem Namen, zweizeilig erlaubt.
- Kein Info-Symbol mit Tooltip: Tooltips gehen am Handy und mit der Fernbedienung schlecht.
- Nur Texte, kein Vertrag, keine Migration (Regel 9: Texte aus `locales/{de,en}.json`).
  Unbekannte Jobtypen zeigen keine Beschreibung.

> **Nachtrag (6. November 2026, Abnahme auf CT 190):** Marco wollte die Beschreibung in einer
> **eigenen Spalte** statt als zweite Zeile unter dem Namen (Wahl zwischen Spalte und Infobox).
> Gebaut: Spalte «Beschreibung» nach dem Namen, höchstens zwei Zeilen mit «…», voller Satz als
> `title` und oben in der aufgeklappten Zeile.

### 3.3 Entwurf der Texte

| Job | Beschreibung (Entwurf) |
|---|---|
| Bibliotheksscan | Sucht in allen Bibliotheken neue, geänderte und fehlende Dateien. |
| Ordnerscan | Liest einen einzelnen Ordner neu ein, sobald sich dort etwas ändert. |
| Datei prüfen | Liest Spuren, Länge und Format einer neuen oder geänderten Datei. |
| Keyframe-Index (ganze Datei lesen) | Liest eine Datei ganz, wenn ihr der Index fürs Spulen fehlt. |
| Aufräumen fehlender Titel | Entfernt Titel, deren Dateien länger als die Schonfrist fehlen. |
| Intro und Abspann erkennen | Findet Intro und Abspann von Folgen für «Intro überspringen». |
| Gelöschten Titel wiederverknüpfen | Hängt eine verschobene Datei an ihren alten Titel und Gesehen-Stand. |
| Kapitelbilder erzeugen | Erzeugt die Vorschaubilder der Kapitel einer Datei. |
| Kapitelbilder nachholen | Reiht alle Dateien ohne Kapitelbilder zum Erzeugen ein. |
| Keyframe-Index nachholen | Reiht alle Dateien ohne Keyframe-Index ein. |
| Titel zuordnen | Sucht für einen neuen Titel den passenden Eintrag bei TMDB oder TVDB. |
| Metadaten auffüllen | Holt Texte, Genres, Freigabe, Besetzung und Bilder eines Titels. |
| Wertungen nachholen | Holt fehlende Stimmenzahlen für die Auswahl der Bilder. |
| Originalsprachen nachholen | Ergänzt bei allen Titeln die Originalsprache für die Tonwahl. |
| Bilder holen | Lädt die gewählten Bilder eines Titels und rechnet die Grössen. |
| Metadaten anwenden | Wendet geänderte Einstellungen der Metadaten auf eine Bibliothek an. |
| Trailer suchen | Sucht einen Trailer bei TMDB, YouTube oder über die KI-Suche. |
| Podcast-Feed abrufen | Liest einen einzelnen Podcast-Feed und neue Folgen. |
| Podcast-Feeds prüfen | Prüft alle Podcast-Feeds auf neue Folgen. |
| Spotlight-Liste abrufen | Holt die Trakt-Liste, aus der das Spotlight seine Titel nimmt. |
| Download vorbereiten | Bereitet einen Download für die Offline-Wiedergabe vor. |
| Mitteilung zustellen | Verschickt eine Benachrichtigung per E-Mail, Telegram oder Web-Push. |
| Verlauf aufräumen | Löscht alte Läufe und Fehlschläge aus dem Verlauf. |
| Update-Prüfung | Schaut nach, ob es eine neuere Slouchcat-Version gibt. |

Aufwand: rund **0,5 Tage** (Texte de/en, Spalte, Storybook-Vergleichsbild, ein e2e-Schritt).

## 4. P-3 Warum manche Jobs keinen Zeitplan haben

### 4.1 Antwort auf Marcos Frage

Es gibt drei Arten von Jobs (`jobs/registry.go:49-56`), und nur eine davon hat einen Zeitplan:

| Art | Anzeige «Nächster Lauf» | Jobs | Warum kein Zeitplan |
|---|---|---|---|
| **Zeitplan** | «in 5 h 16 min · alle 6 h» oder «aus» | Bibliotheksscan, Aufräumen fehlender Titel, Keyframe-Index nachholen, Podcast-Feeds prüfen, Spotlight-Liste abrufen, Verlauf aufräumen, Update-Prüfung (7) | – |
| **Ereignis** | «bei Bedarf» | Ordnerscan, Datei prüfen, Keyframe-Index, Intro und Abspann, Wiederverknüpfen, Kapitelbilder erzeugen, Titel zuordnen, Metadaten auffüllen, Wertungen, Bilder holen, Metadaten anwenden, Trailer suchen, Podcast-Feed abrufen, Download vorbereiten, Mitteilung zustellen (15) | **Technisch.** Ein solcher Job gilt immer **einer** Datei, **einem** Titel oder **einer** Mitteilung und entsteht, wenn genau dafür etwas passiert (neue Datei, Wiedergabe, Einladung). Ohne diesen Gegenstand hat «jetzt ausführen» keinen Inhalt; der Server lehnt einen Zeitplan dafür ab (beim Registrieren `registry.go:359-362`, über die Schnittstelle 409 `job_type_not_scheduled`). Entschieden in Konzept 09, F-3. |
| **Von Hand** | «von Hand» | Kapitelbilder nachholen, Originalsprachen nachholen (2) | **Nur ein Entscheid.** Beide laufen über alle Dateien und brauchen keinen Gegenstand; sie wären einplanbar. Konzept 13 (Kapitelbilder) und der Nachtrag zu Konzept 10 (Originalsprachen) haben sie als Knopf gebaut, weil sie einmal den Bestand nachholen. |

«Keyframe-Index nachholen» zeigt «aus»: Es ist ein Zeitplan-Job, dessen Zeitplan ab Werk
ausgeschaltet ist (Konzept 14, O-6). Ein einmaliger Termin («Einmalig am») läuft trotzdem.

### 4.2 Vorschlag

1. **Die zwei Hand-Jobs werden einplanbar** (F-3, entschieden) wie «Keyframe-Index nachholen»: Jobart
   `schedule`, Zeitplan ab Werk **aus**, Takt 7 Tage, «Jetzt ausführen» bleibt. Damit hat das
   aufgeklappte Feld «Zeitplan» auch dort Takt, Zeitfenster und «Einmalig am». Keine Migration:
   `ensureSchedule` legt die Zeile beim nächsten Start an (`scheduler.go:175-223`). Ihre Texte
   `trigger` in `locales/{de,en}.json` sagen heute «von Hand» und werden angepasst.
2. **Ereignis-Jobs bekommen statt des Zeitplans einen Satz**, der im aufgeklappten Feld steht,
   wo sonst der Zeitplan steht: «Kein Zeitplan: läuft von selbst je Datei, sobald …» (aus dem
   bestehenden `trigger`-Text). Die Spalte zeigt statt «bei Bedarf» **«automatisch»** (F-4,
   entschieden).
3. **Zeitfenster für schwere automatische Jobs** (F-5, entschieden: ja, **je Job einstellbar**,
   Wiedergabe geht vor). Siehe 4.3.

Aufwand für 1 und 2: rund **0,5 Tage**, Vertrag unverändert (der Typ meldet schon heute
`trigger` und `schedule`).

### 4.3 Zeitfenster für schwere automatische Jobs

Marco will, dass die Jobs, die ganze Dateien lesen, nur nachts laufen können, damit das NAS
tagsüber ruhig bleibt.

- **Welche Jobs:** `probe.keyframes` (Keyframe-Index, ganze Datei lesen), `segments.detect`
  (Intro und Abspann) und `chapters.images` (Kapitelbilder erzeugen). Andere automatische Jobs
  lesen nur Kopfdaten oder kleine Dateien und bekommen kein Fenster. Die Zeitplan-Jobs haben ihr
  Fenster schon heute im Zeitplan. Welche Jobart ein Fenster haben darf, sagt ein neues Merkmal
  `Windowable` an `jobs.Type`; `validateType` lässt es nur bei Ereignis-Jobs zu.
- **Einstellung je Job:** Im aufgeklappten Feld dieser drei Jobs steht anstelle des Zeitplans ein
  Feld **«Nur in einem Zeitfenster»** mit Von und Bis (wie im Zeitplan, Zeitzone des Servers, über
  Mitternacht erlaubt), ab Werk **aus**.
- **Was das Fenster bewirkt:** Ausserhalb des Fensters nimmt kein Arbeiter einen neuen Auftrag
  dieser Jobart. Die Aufträge bleiben wartend; die Spalte «Nächster Lauf» zeigt «wartet bis
  01:00». Ein Auftrag, der beim Ende des Fensters läuft, läuft fertig (die Jobs sind je Datei
  kurz genug; Abbrechen würde Arbeit wegwerfen).
- **Wiedergabe geht vor** (entschieden): Ein Auftrag, den eine Wiedergabe auslöst, läuft
  **sofort, trotz Fenster**. Heute gibt es dafür nur das Merkmal `jobs.override_pause` aus
  Konzept 14 (F-3), und das setzt nur `QueueKeyframesForPlayback` (`library/jobs.go:126`); die
  Kapitelbilder nach dem Abspielen reiht `queueAfterPlayback` ohne Merkmal und mit Verzögerung ein
  (`library/chapter_images.go:235-236`). Darum ein **eigenes Merkmal `jobs.override_window`**:
  - Es hebt **nur das Fenster** auf, nicht die Pause, und zählt **nicht** gegen die Grenze von drei
    Pause-Ausnahmen (`MaxPauseOverrides`, `jobs/queue.go:72`). Sonst hielten verzögerte
    Kapitelbild-Aufträge Plätze, die der Keyframe-Index für die Wiedergabe braucht, und Konzept 14
    F-3 (ADR 0022) würde geändert.
  - `QueueKeyframesForPlayback` setzt beide Merkmale, `queueAfterPlayback` nur `override_window`.
    Die Kapitelbilder nach dem Abspielen bleiben damit bei pausierter Jobart pausiert, wie heute.
  - Die Seite zeigt solche Aufträge wie heute als «für Wiedergabe».

> **Nachtrag (6. November 2026, Review von Stufe 2, Befund M-1):** Marco hat entschieden: **Download
> geht vor.** Der Keyframe-Index für einen Offline-Download (`QueueKeyframesForDownload`) setzt
> ebenfalls `override_window` (ohne `override_pause`; die Pause hält ihn weiter an). Der Hinweis
> auf der Seite sagt «Wiedergabe und Downloads starten trotzdem sofort.» Entscheidungsprotokoll,
> Nachtrag zu Frage 22.
- **Speicher (Migration):** `job_types` bekommt `window_start` und `window_end` (Minuten des Tages,
  wie in `job_schedules`; `CHECK` beide leer oder beide gesetzt), `jobs` bekommt `override_window`
  (`INTEGER NOT NULL DEFAULT 0`). Beide Tabellen sind `STRICT`; `ALTER TABLE … ADD COLUMN` reicht,
  kein Neuaufbau. `override_pause` behält seinen Namen (Index `jobs_override_pause`).
- **Beanspruchen:** Der Server rechnet die Minute des Tages in seiner Zeitzone in Go aus und gibt
  sie `ClaimJob` mit; die Abfrage nimmt einen Auftrag einer Jobart mit Fenster nur innerhalb des
  Fensters oder mit `override_window = 1`.
- **Vertrag:** Die Jobart meldet ihr Fenster; eine Operation zum Setzen
  (`PUT /admin/job-types/{type}/window`, `x-visibility: admin`). Der Server lehnt sie für andere
  Jobarten mit 409 ab (wie `job_type_not_scheduled`, `admin/jobtypes.go:541-543`).
- **Regeln:** Pause geht vor Fenster (eine pausierte Jobart läuft auch im Fenster nicht, ausser
  `override_pause`). «Jetzt ausführen» gibt es für diese Jobarten nicht (sie haben keinen `Run`).
- **Tests:** Beanspruchen ausserhalb des Fensters (nicht genommen), über Mitternacht, Pause vor
  Fenster, `override_window` trotz Fenster, `override_pause` ohne `override_window` bleibt im
  Fenster gesperrt, Grenze der Pause-Ausnahmen unverändert.

Aufwand: rund **2 Tage**.

## 5. P-4 «Benachrichtigungen» bleibt in der Verwaltung

### 5.1 Heute

Der Eintrag der Verwaltungsleiste zeigt auf `/settings/notifications` (`sections.ts:61`,
`paths.ts:59`). Diese Route hängt nicht an `adminRoute` (`router.tsx:507-514`); `AppLayout`
erkennt die Verwaltung nur am Pfad `/admin/…` (`AppLayout.tsx:155-156`). Darum verschwindet die
Verwaltungsleiste, und die App-Seitenleiste kommt zurück. Das war so entschieden (Konzept 11,
O-3; Konzept 12, S-1): Die Seite gilt für jedes Konto, nicht nur für Administratoren. In der
Praxis wirkt es wie ein Fehler.

Die Seite selbst ist **pro Konto** (`/me/channels`, `x-visibility: account`): welche Kanäle
**dieses Konto** bekommt. Die Kanäle des Servers (E-Mail-Server, Telegram-Bot, Web-Push) richtet
man unter Integrationen ein.

### 5.2 Vorschlag (F-7, entschieden)

- Neue Route **`/admin/notifications`** unter `adminRoute`. Sie zeigt **dieselbe Seite**
  (`ChannelSettingsPage`, dieselben Bauteile) in der Verwaltung, mit Verwaltungsleiste und
  kompakter Dichte, dazu oben ein Verweis «Kanäle des Servers einrichten → Integrationen».
- Der Eintrag der Verwaltungsleiste zeigt auf `/admin/notifications`.
- `/settings/notifications` bleibt für alle Konten, wie heute, mit App-Seitenleiste.
- Kein Vertrag, keine Migration, kein neuer Server-Code.
- Tests: `sections.test.ts:38-40`, `AdminHomePage.test.tsx:749`, `e2e/admin-home.spec.ts:46,
  195-198`, `e2e/admin-phone.spec.ts:30` halten heute das Verlassen fest und werden umgestellt;
  die neue Seite bekommt ihren Tastaturtest (Regel 8).

Aufwand: rund **0,5 Tage**.

## 6. P-5 Spotlight: aktives Poster immer links

### 6.1 Heute

`HeroCarousel.tsx:271-292` verschiebt die Posterreihe nur, wenn das aktive Poster angeschnitten
oder ausserhalb liegt; dann stellt es die Reihe an die linke Kante. Solange es ganz sichtbar
ist, wandert nur der rote Rahmen. Das war **Marcos Nachtrag vom 30. Oktober 2026** (Konzept 08,
N-4; ADR 0016): Zuvor rückte die Reihe bei jedem Wechsel nach. Am Ende der Reihe sorgt ein
Leerraum dafür, dass auch die letzten Poster an die linke Kante können (`Row.tsx:29-33`).

### 6.2 Varianten (Mockup, Anhang A.1)

| Variante | Verhalten | Dafür | Dagegen |
|---|---|---|---|
| **A · heute** | Rahmen wandert, Reihe springt erst bei angeschnittenem Poster. | Reihe steht ruhig. | Das Auge muss dem Rahmen folgen; Marco will das nicht mehr. |
| **B · immer links** (Empfehlung in Fassung 1) | Bei **jedem** Wechsel rückt die Reihe, bis das aktive Poster bündig links unter dem Titel steht. Gezeigte Titel liegen links ausserhalb; nach links wischen zeigt sie. Nach dem letzten Titel gleitet die Reihe zurück an den Anfang. | Aktives Poster immer am selben Ort; Reihenfolge fest; einfach zu bauen (die Mechanik gibt es schon, nur die Bedingung fällt weg). | Die Reihe bewegt sich alle 5 s um eine Karte. |
| **C · Endlosband** (**Marcos Wahl**, F-6) | Links steht der aktive Titel, die gezeigten wandern ans Ende. | Kein Leerraum, kein Sprung zurück. | Reihenfolge wechselt ständig; nach links blättern gibt es nicht; für Bildschirmleser schwer. |

### 6.3 Bau (Variante C «Endlosband», F-6)

- **Reihenfolge:** Die Reihe zeigt die Spotlight-Titel **ab dem aktiven Titel im Kreis**: aktiv,
  dann die nächsten, am Ende die schon gezeigten. Die Liste der Titel und ihre Reihenfolge im
  Band bleiben, wie sie der Server liefert (`heroStart`, Takt); nur die **Anzeige** ist gedreht.
  Der rote Rahmen sitzt **immer auf dem ersten Poster**.
- **DOM bleibt in fester Reihenfolge:** Die Listeneinträge stehen im DOM immer in der Reihenfolge
  des Servers; gedreht wird nur sichtbar über CSS `order` je Eintrag (`(Stelle − aktiv + n) mod
  n`). So rückt beim Takt nichts unter dem Lesecursor eines Bildschirmlesers weg, und jeder
  Eintrag behält seine Stelle. Die räumliche Navigation (norigin) folgt der sichtbaren Lage und
  geht darum in der gedrehten Reihenfolge. `RowList` (`Row.tsx:58-60`) bekommt dafür eine Prop
  für `order` und `aria-posinset`/`aria-setsize` am `li`.
- **Wechsel nach vorn** (Takt, Pfeil rechts am Band, Klick auf ein Poster): Die Reihe gleitet um
  so viele Karten nach links, bis der gewählte Titel vorne steht (Transform auf der Spur), danach
  bekommen die Einträge ihr neues `order`, und die Spur steht wieder bei 0. Ein Klick auf das
  vierte Poster dreht also um drei Karten. Animation 0,45 s, bei «Bewegung reduzieren» ohne
  Animation.
- **Wechsel zurück** (Pfeil links am Band, «Abspielen» mit Pfeil links wie heute `pageBack`): Der
  letzte Titel der Reihe gleitet von links nach vorne.
- **Tastatur in der Reihe:** Pfeile bewegen den Fokus wie heute (F-5 aus Konzept 08), Enter wählt.
  Nach dem Wählen steht der gewählte Titel vorne, und **der Fokus bleibt auf ihm** (dem ersten
  Poster). Pfeil links auf dem ersten Poster verlässt die Reihe zur Seitenleiste, wie heute. Die
  **Endsperre** `holdEnds` (`HeroCarousel.tsx:341-344`) bleibt, bezogen auf die sichtbare
  Reihenfolge: Pfeil rechts auf dem letzten sichtbaren Poster hält den Fokus, er springt nicht
  zurück an den Anfang.
- **Halt:** Der Takt hält wie heute bei Tastaturfokus im Band (`:focus-visible`), Trailer, Knopf
  «Pause» und Tab im Hintergrund, bei «Bewegung reduzieren» läuft er nicht. Neu hält er auch,
  solange der Fokus irgendwo in der Posterreihe steht (nicht nur sichtbarer Fokus), damit ein
  Bildschirmleser die Reihe in Ruhe lesen kann. Der Mauszeiger hält weiterhin nicht (Konzept 08,
  F-3).
- **Bildschirmleser:** Jedes Poster trägt seine **feste** Stelle im Band («3 von 12»); die
  Positionsanzeige oben rechts im Band bleibt «3 / 12». Die Drehung wird nicht angesagt.
- **Wischen am Handy** scrollt die Reihe frei; nach dem nächsten Wechsel steht sie wieder bei 0.
- **Wegfallen:** der Leerraum am Ende der Spotlight-Reihe (`Row.tsx` `trailingSpace`) und die
  Bedingung «nur scrollen, wenn angeschnitten» (`HeroCarousel.tsx:286`).
- **Vorabmalerei:** unverändert; das erste Poster ist ohnehin der aktive Titel.
- Konzept 08 (N-4) und ADR 0016 bekommen einen Nachtrag (Abschnitt 10).
- Tests: `e2e/home.spec.ts` 191 und 541 (N-4) werden auf das Endlosband umgeschrieben (aktives
  Poster sichtbar zuerst, DOM-Reihenfolge fest, Fokus nach Enter auf dem ersten Poster,
  Endsperre); Storybook-Vergleichsbilder `ActivePoster` und `PosterStrip` neu.

Aufwand: rund **1,5 Tage**. Nur Web-App.

## 7. P-6 Metadaten bearbeiten mit dem Stift

### 7.1 Heute

- Die Verwaltung hat unter Pflege die Seite **Titel** (`/admin/items/$itemId`,
  `AdminItemPage.tsx`) mit «Zuordnen» (Suche bei TMDB/TVDB), «Aktualisieren», den Reitern
  Metadaten (Herkunft je Feld, Altersfreigabe, Originalsprache), **Bilder** (Poster, Hintergrund,
  Logo; Vorschläge der Anbieter mit «Mehr» und «Neu laden»; Wahl sperrt das Bild), Dateien und
  Trailer.
- Auf der Detailseite von Film und Serie steht für Administratoren mit Hauptprofil schon ein
  Knopf «Verwalten», der dorthin führt (`ItemPage.tsx:166-168, 223-227`). Auf Covern gibt es
  nichts.
- **Es fehlt:** Bild hochladen (kein Endpunkt; der Server kann ein gesperrtes Bild aber schon
  ohne Neuholen skalieren, `images.go:217-219`), freies Bearbeiten von Titel, Originaltitel,
  Sortiertitel, Jahr, Datum, Slogan, Beschreibung und Genres (die Spalten gibt es:
  `items.sort_title`, `original_title`, `release_date`, `genres`, `item_texts`), Schlösser je
  Feld (`items.locked_fields` gibt es; gesetzt wird es heute nur als Nebenwirkung), Bilder für
  **Folgen** (Folgenbild) im Editor.

### 7.2 Vorbild Emby

Emby hat am Titel ein Menü mit «Metadaten bearbeiten» (alle Felder, je Feld ein Schloss, eine
Sperre für den ganzen Titel), «Bilder bearbeiten» (je Bildart: Vorschläge der Anbieter mit
Sprachfilter, Hochladen, Löschen), «Identifizieren» und «Metadaten aktualisieren» (fehlende
ergänzen oder alles ersetzen, Bilder ersetzen ja/nein). Dieses Papier übernimmt den Aufbau, nicht
das Aussehen: dieselben vier Aufgaben, als vier Reiter in Slouchcat-Bauteilen.

### 7.3 Varianten (Mockup, Anhang A.2)

| Variante | Was der Stift öffnet | Dafür | Dagegen |
|---|---|---|---|
| **A · Dialog** (**Marcos Wahl**, F-8) | Fenster über der Seite, wie Emby. | Vertraut, viel Platz. | Seite dahinter verdeckt; die Wirkung eines neuen Posters sieht man erst nach dem Schliessen. |
| **B · Seitenblatt** (Empfehlung in Fassung 1) | Blatt von rechts, 520 px; die Seite bleibt links sichtbar und zeigt Änderungen sofort. Am Handy ganzseitig. | Sofortige Vorschau auf der echten Seite; aus der Bibliothek Titel für Titel durchgehen. | Schmaler; Hintergründe zwei statt vier nebeneinander. |
| **C · Verwaltungsseite** | Der Stift führt auf die bestehende Seite Pflege › Titel. | Am wenigsten Bau. | Führt aus der Zuschauerseite weg; keine Vorschau. |

### 7.4 Bau (Variante A «Dialog», F-8)

**Wo der Stift steht** (F-9, entschieden):

- Auf der **Detailseite** von Film, Serie, Staffel und Folge neben den Knöpfen (ersetzt «Verwalten»;
  der Weg zur Verwaltungsseite bleibt im Dialog oben als Verweis).
- Auf **Covern im Bibliotheksraster** beim Zeigen oder mit Tab (oben rechts). Die Karte ist ein
  Link; ein Knopf darin wäre ungültiges HTML. Der Stift steht darum **neben** dem Link im selben
  Kartenrahmen, absolut über das Cover gelegt, und ist in der Tab-Reihenfolge der zweite Halt der
  Karte (Link, dann Stift). Mit Pfeiltasten (räumliche Navigation) erreicht man ihn nicht; er ist
  ein Werkzeug für Maus und Tab. Am Handy steht er nur auf der Detailseite, damit das Raster
  ruhig bleibt.
- Nicht auf den Reihen der Startseite.
- **Nur für den Server-Administrator** (Marco ausdrücklich): Rolle `admin` mit Hauptprofil
  (`canManage`, wie heute). Andere Konten und Nebenprofile sehen keinen Stift. Das ist eine
  Abkürzung im Client, keine Regel: Der Server beantwortet jeden Bearbeitungsaufruf nur für diese
  Sitzung (`x-visibility: admin`, Regel 12).

**Der Dialog** liegt über der Seite (Breite bis 1100 px, Höhe bis 90 % des Fensters, eigener
Bildlauf im Inhalt), am Handy ganzseitig. Escape oder «Schliessen» schliesst ihn, der Fokus geht
zurück auf den Stift; im offenen Dialog bleibt der Fokus im Dialog (`Dialog` aus `web/src/ui`).
Nach dem Schliessen lädt die Seite den Titel neu, damit neue Bilder und Texte sofort stehen.
Er hat einen Kopf (Mini-Poster, Titel, Art, Bibliothek, Verweis «In der Verwaltung
öffnen») und vier Reiter:

1. **Bilder** (zuerst, weil Marco Bilder und Cover nennt): oben die Bildarten des Titels (Film
   und Serie: Poster, Hintergrund, Logo; Staffel: Poster; **Folge: Folgenbild**, neu) mit Zustand
   «automatisch · TMDB» oder «gewählt · gesperrt». Darunter die Vorschläge der Anbieter wie
   heute, mit **Filter nach Anbieter und Sprache** (neu im Client; die Daten liefert
   `image-candidates` schon). Ein Klick wählt, gilt sofort und sperrt. **«Eigenes Bild
   hochladen»** (neu) und **«Zurück auf automatisch»** (heute «Sperre aufheben»).
2. **Metadaten:** Titel, Originaltitel, Sortiertitel, Jahr, Erscheinungsdatum, Slogan (nur
   Film), Beschreibung, Genres (Chips mit Vorschlägen aus den vorhandenen Genres),
   Altersfreigabe, Originalsprache. Neben jedem Feld die Herkunft («aus TMDB») und ein
   **Schloss**. Wer ein Feld ändert, sperrt es; das Schloss hebt die Sperre auf. «Speichern»
   schreibt alle geänderten Felder auf einmal. Texte gelten für die **Metadaten-Sprache des
   Servers** (F-10, entschieden; heute Deutsch); die anderen Sprachen holt der Server weiter von
   den Anbietern.
3. **Zuordnen:** der bestehende Dialog (Suche TMDB/TVDB, Treffer wählen).
4. **Aktualisieren:** «Jetzt aktualisieren» lädt alle freien Felder und automatisch gewählten
   Bilder neu (das tut «Aktualisieren» schon heute); das Kästchen «Vorher alle Sperren dieses
   Titels aufheben» gibt vorher alles frei (7.5).

### 7.5 Server und Vertrag

**Sperren je Sprache.** Heute heisst die Sperre eines Textfelds wie das Feld und gilt für **alle**
Sprachen (`metadata/fill.go:169-181`; `match.go:1174-1185` behält Titel, Beschreibung und Slogan in
jeder Sprache, sobald sie gesperrt sind). Damit F-10 stimmt («die anderen Sprachen holt der Server
weiter»), bekommen die drei Textfelder **Sperren je Sprache**: `title:de`, `overview:de`,
`tagline:de`, `sort_title:de`. `match.go` und `fill.go` lesen sie je Sprache; eine alte Sperre
ohne Sprache (`overview`) gilt weiter für alle Sprachen, damit bestehende Sperren nichts ändern.
Die übrigen Felder (Originaltitel, Jahr, Datum, Genres, Freigabe, Originalsprache) haben keine
Sprache und behalten ihre heutigen Sperrnamen.

**Sortiertitel.** Die Bibliothek sortiert nach `item_texts.sort_key` der Profilsprache und nimmt
`items.sort_title` nur als letzten Rückfall (`visibility/library.go:166, 286-294`); jeder
Textschreiber rechnet `sort_key` aus dem Titel neu (`queries/system/metadata.sql:122`,
`match.go:1186`). Der bearbeitete Sortiertitel landet darum in `item_texts.sort_key` der
Server-Sprache und ist durch `sort_title:de` gesperrt; jeder Schreiber lässt einen gesperrten
`sort_key` stehen.

| Operation | Art | Inhalt |
|---|---|---|
| `PATCH /admin/items/{id}/metadata` (neu) | JSON, strict server, `x-visibility: admin` | Felder `title`, `originalTitle`, `sortTitle`, `year`, `releaseDate`, `tagline`, `overview`, `genres`, dazu `unlock: [Feld]`. Texte gelten immer für die Metadaten-Sprache des Servers (kein `lang` im Körper, F-10). Jedes gesetzte Feld bekommt seine Sperre. Antwort: `AdminItem`. |
| `PUT /admin/items/{id}/images/{kind}/upload` (neu) | `http.Handler` (Regel 6), multipart, höchstens **20 MB**, JPEG/PNG/WebP | Siehe «Hochladen» unten. |
| `POST /admin/items/{id}/refresh` (erweitert) | JSON | Neuer, optionaler Körper `{ unlockAll: bool }`; ohne Körper wie heute. |
| `GET /admin/genres` (neu) | JSON, `x-visibility: admin` | Alle Genres der Server-Sprache für die Vorschläge im Feld Genres. `Genres()` für Zuschauer ist `scoped` und auf 20 begrenzt und passt darum nicht. |

**Aktualisieren.** Heute reiht «Aktualisieren» `metadata.match` ein (`openapi.yaml:3367-3374`), und
das überschreibt **jedes freie Feld**, für das ein Anbieter einen Wert hat (`match.go:1058-1070`).
Das ist «freie Felder neu laden», nicht «Fehlendes ergänzen». Der Reiter bietet darum nur, was es
gibt: **«Jetzt aktualisieren»** (freie Felder und automatisch gewählte Bilder neu, gesperrte
bleiben) und das Kästchen **«Vorher alle Sperren dieses Titels aufheben»** (`unlockAll`). Die
dritte Wahl «Fehlendes ergänzen» aus dem Mockup A.2 entfällt.

**Hochladen.**
- Der Körper ist sonst auf 64 KiB begrenzt, und Ausnahmen gelten nur für genaue Pfade
  (`api/limits.go:11, 24-29, 38`). `largeBodies` bekommt darum Muster für Pfade mit Parametern;
  die Route kommt zu `binaryRoutes` (`api/api.go:53+`). Die Multipart-Verarbeitung folgt
  `restoreBackupUpload`.
- Vor jeder Verarbeitung prüft der Server das Format am Inhalt und die Pixelzahl mit
  `image.DecodeConfig` (höchstens 50 Megapixel); erst dann rechnet er die Grössen.
- Gespeichert wird auf dem bestehenden Weg «gesperrt ohne Wahl» (`metadata/images.go:216-219`):
  das Original als `<kind>-original.<ext>` über `paths` im Bild-Cache (Regel 4), Zeile in
  `images` mit Anbieter `manual` und `source_url` `upload:<sha256>`, die Bildart in
  `locked_fields`. **Keine** Zeile mit `chosen = 1` in `item_images`, sonst versucht
  `imageSource`, die Quelle bei einem Anbieter zu holen.
- Ausgeliefert wird weiter `scoped`.

**Folgenbild.** `ImageKind` kennt `backdrop` schon (`openapi.yaml:8086-8088`). Die Grenze für
Folgen steckt in Go: `choosableKinds` (`metadata/image_candidates.go:99`) und `imageKindsOf`
(`metadata/images_choice.go:204-207`, nur mit eingeschalteter Einstellung «Folgenbilder»). Beide
lassen `backdrop` an Folgen zu. Ist die Einstellung aus, zeigt der Reiter Bilder bei Folgen einen
Hinweis mit Verweis auf Verwaltung › Metadaten statt der Vorschläge.

Keine Migration: Spalten, `locked_fields`, `images` und der Anbieter `manual` gibt es. Alle neuen
Operationen bekommen einen Eintrag im Vertragstest (Regel 7).

Aufwand: rund **7,5 Tage** (Server 4: Sperren je Sprache, Sortiertitel, Hochladen; Dialog und
Reiter 2,5; Tests und Storybook 1).

## 8. P-7 Bibliotheken auf der Startseite

### 8.1 Heute

Die Startseite hat höchstens **acht feste Reihen** (Spec E-9; `home_rows` mit CHECK auf acht
Schlüssel, `00021_spotlight.sql:21-57`; `MaxRows = 8`). Reihenfolge und Ausblenden gelten für
den ganzen Server (Seite Startseitenreihen). Die Bibliotheken stehen in der **Seitenleiste**
unter «Meine Medien». Eine Reihe je Bibliothek oder die Kachelreihe «Meine Medien» wie in Emby
hat Marco am 29. Oktober 2026 **abgelehnt** (Konzept 08, Wunsch 2 und F-7; ADR 0016, Option e),
damit die Startseite übersichtlich bleibt.

### 8.2 Varianten (Mockup, Anhang A.3)

| Variante | Was es ist |
|---|---|
| A · Reihe je Bibliothek | In Startseitenreihen erscheint je Bibliothek eine Reihe «Neu in Filme», «Neu in Serien» usw., platzierbar und ausblendbar. |
| B · Kachelreihe «Meine Medien» | Eine Reihe mit einer Kachel je Bibliothek (Name, Zahl, Bildcollage), die zur Bibliotheksseite führt, wie in Emby. |
| C · Umschalter oben | Chips «Alle · Filme · Serien …» filtern alle Reihen auf eine Bibliothek. |

**Marcos Wahl (F-11): A und B zusammen.** «Meine Medien finde ich gut. Jedoch möchte ich auch Neu
in Filme, Neu in Serien etc. für jede Bibliothek. Die Verwaltung soll dieses auch anzeigen.» Und
zur Anzahl (F-12): «15 Plätze soll reichen.»

### 8.3 Bau

- **Neue Reihen:**
  - **«Meine Medien»** (Schlüssel `libraries`): eine Kachel je Bibliothek, die das Profil sehen
    darf (`ListVisibleLibraries`, eine Bibliothek zählt nur mit mindestens einem sichtbaren
    Titel), in der Reihenfolge der Seitenleiste. Die Kachel zeigt Name, Zahl der Titel und eine
    Collage aus vier Postern der neuesten Titel; ein Klick öffnet die Bibliotheksseite.
  - **«Neu in <Bibliothek>»** (Schlüssel `library`, mit `libraryId`): die neuesten sichtbaren Titel
    der Bibliothek, wie die Bibliotheksseite mit `sort=added`, höchstens 20; «Alle anzeigen» führt
    zur Bibliotheksseite, sortiert nach «Neu hinzugefügt». Eine Reihe je Bibliothek, auch für
    Hörbücher und Podcasts.
- **Verwaltung › Startseitenreihen** zeigt **alle** Reihen: die acht bisherigen, «Meine Medien»
  und je Bibliothek eine Zeile «Neu in …» mit dem Zusatz «Bibliothek». Platz und Ausblenden wie
  heute. Ab Werk: «Meine Medien» und alle Bibliotheksreihen **ausgeblendet**, am Ende der Liste;
  die Startseite sieht nach dem Update aus wie vorher.
- **Neue Bibliothek:** bekommt ihre Zeile «Neu in …» automatisch, ausgeblendet, am Ende. Eine
  Bibliothek löschen kann man heute nicht (kein `deleteLibrary` im Vertrag); der Fremdschlüssel
  mit `ON DELETE CASCADE` ist nur Vorsorge.
- **Höchstens 15 Plätze** (F-12): höchstens 15 Zeilen **sichtbar**; ausgeblendete zählen nicht.
  Wer bei 15 sichtbaren eine weitere einblendet, bekommt «Schon 15 Reihen sichtbar. Blende zuerst
  eine andere aus.» Leere Reihen bleiben wie heute verborgen (E-9); `page()` und `hasEntries`
  (`discover/discover.go:173`, `HomePage.tsx:55-56`) zählen bei «Meine Medien» die Kacheln, sonst
  fiele die Reihe als leer weg. Spec E-9 ändert sich von acht auf fünfzehn (Abschnitt 10).
- **Reihen bleiben für den ganzen Server gleich**; nur ihr Inhalt richtet sich nach dem Profil.
  Die Bibliotheksreihen zeigen nur sichtbare Titel (Regel 2; Abfrage in `queries/scoped/` mit
  `visible_items`). Eine Bibliothek, die ein Profil nicht sieht, fehlt dort ganz.
- **Eine Reihe bekommt eine Kennung.** Heute ist der Schlüssel überall die Kennung einer Reihe:
  `rowOrder` liefert `[]string` (`discover/discover.go:196-213`), `SetHomeRow … WHERE key = @key`
  (`queries/app/discover.sql:29`), `SetHomeRows` vergleicht mit `model.HomeRowKeys`
  (`admin/homerows.go:22-31`), `toRows`/`toSettings` behalten nur Schlüssel, Ausblenden und Platz
  (`features/admin/homerows/rows.ts:20-25`), `setHidden` sucht nach `entry.key`
  (`HomeRowsPage.tsx:138`), und `HomePage.tsx:52, 210, 247-250` nimmt `row.key` für React-Schlüssel,
  `focusKey`, `cardKey` und `rowPages`. Mehrere Reihen `library` würden sich überall gegenseitig
  treffen. Neu hat jede Reihe eine **Kennung `id`**: gleich dem Schlüssel für die festen Reihen
  (`picks`, `libraries`) und `library:<id>` für Bibliotheksreihen. Alle genannten Stellen gehen
  auf `id` über.
- **Speicher (Migration):** `home_rows` wird nach Vorlage neu aufgebaut (der CHECK auf acht
  Schlüssel fällt): `id TEXT PRIMARY KEY`, `key` (CHECK auf die zehn Schlüssel), `library_id`
  (Fremdschlüssel auf `libraries`, `ON DELETE CASCADE`), `position`, `hidden`, `updated_at` und
  `CHECK ((key = 'library') = (library_id IS NOT NULL))`. Die Migration übernimmt die acht Zeilen
  (`id = key`) und legt «Meine Medien» und eine Zeile je bestehender Bibliothek ausgeblendet an.
- **Vertrag:** `HomeRowKey` bekommt `libraries` und `library`; `HomeRow` und `HomeRowSetting`
  bekommen `id` und ein optionales `libraryId`; `HomeRow` bekommt für «Meine Medien» das Feld
  `libraries` (Kacheln); `HomePage.rows` `maxItems` 15. `setHomeRows` nimmt die Liste nach `id`;
  fehlt eine Zeile (zum Beispiel eine Bibliothek, die erst nach dem Öffnen der Seite dazukam),
  behält sie ihren gespeicherten Platz, statt dass der Server die ganze Liste mit 422 ablehnt.
  **Brechend:** Ein alter Client kennt die neuen Schlüssel nicht (`homeRowTitleKey` ist ein
  geschlossenes `Record<HomeRowKey, string>`, `catalog/labels.ts:18-27`, ohne Rückfall). Web-App
  und Server kommen im selben Image; fremde Clients gibt es nicht. Der Vertrag wird darum
  **1.15.0, brechend für `getHome` und `setHomeRows`**, so wie 1.2.0 (Benutzerverwaltung) brechend
  war; die Release-Notizen sagen es.
- **Vorabmalerei:** malt bei «Meine Medien» oder einer Bibliotheksreihe an erster Stelle deren
  Poster wie bei jeder Posterreihe.
- **Tests:** `contract_home_rows_test.go`, `discover_test.go`, `admin/homerows_test.go`,
  `HomeRowsPage` (Vitest, Storybook), `rows.ts`, `e2e/home.spec.ts`, neue Tastaturprüfung der
  Kachelreihe (Regel 8).

Aufwand: rund **3,5 Tage**.

## 9. P-8 «Update installieren»

### 9.1 Heute

- Die Update-Prüfung (`admin/update.go`) liest nur das Manifest der neuesten Version auf GitHub
  und zeigt einen Hinweis (Spec A-5). Sie installiert **nichts**. `deploy/compose.yaml` pinnt das
  Image auf `SLOUCHCAT_VERSION`; der Kommentar dort sagt: Ein Update ist ein bewusster Schritt
  (`.env` ändern, `docker compose pull`).
- «Neu starten» startet nur dasselbe Programm neu; ein neues Image kann der Container aus sich
  heraus nicht holen. Dafür braucht es Zugriff auf Docker auf dem Gastgeber.
- **Befund auf CT 190 (5. November 2026):** Die Karte zeigt «Letzte Prüfung fehlgeschlagen». Im
  Log steht um 16:33:57 `update check failed … release manifest unreachable … context deadline
  exceeded` (10 s Zeitlimit, eine Minute nach dem Start). Gespeichert ist noch 1.12.0 als neueste
  Version; die nächste Prüfung kommt erst nach 24 h. Release **1.13.1** (seit 22:26 auf GitHub,
  Manifest von hier aus erreichbar) erscheint darum nicht. Ein einzelner Aussetzer von GitHub
  wirkt also einen Tag lang.

### 9.2 Varianten

| Variante | Wie | Bewertung |
|---|---|---|
| **A · Updater-Begleiter** (**Marcos Wahl**, F-13) | Ein kleiner zweiter Container `slouchcat-updater` im Compose-Profil `updater`, **nur er** bekommt `/var/run/docker.sock` und den Ordner `deploy/`. Er hat keinen offenen Port, nur das interne Docker-Netz, und nimmt nur einen Auftrag an: «installiere Version X», wobei X die Version aus dem Manifest ist und neuer als die laufende. Ablauf: `.env` sichern (`.env.bak-vor-X`), `SLOUCHCAT_VERSION=X` setzen, `docker compose pull slouchcat`, Image-Digest gegen das Manifest prüfen, `docker compose up -d slouchcat`, auf `healthy` warten. Wird der Server nicht gesund, setzt er die alte Version zurück und meldet den Fehler. | Ein Klick für Marco; die gefährliche Berechtigung liegt in einem Container ohne Webseite, ohne Benutzer und ohne Medien. Ab Werk nicht dabei; wer ihn will, startet das Profil. |
| B · Docker-Socket im Slouchcat-Container | Der Server selbst steuert Docker. | **Abgelehnt.** Wer die Web-App übernimmt, hätte Root auf dem Gastgeber. |
| C · Nur Anleitung | Der Knopf zeigt die zwei Befehle zum Kopieren. | Ohne Risiko, aber kein Installieren. Gilt als **Rückfall**, wenn der Updater fehlt. |
| D · Watchtower | Fremdes Werkzeug, aktualisiert von selbst. | **Abgelehnt.** Widerspricht dem gepinnten Tag und dem bewussten Schritt; kein Knopf, keine Rücknahme. |

### 9.3 Bau (Variante A mit Rückfall C; F-13, F-14, F-15 entschieden)

**Wer ruft was auf.**
- Der Updater ist ein **eigenes Go-Programm** `server/cmd/updater` mit eigenem Image
  `ghcr.io/slouchcat/slouchcat-updater`, gebaut und attestiert im selben Release-Lauf. Er ruft
  `docker compose` über `exec.CommandContext` mit Argumentliste auf, nie über eine Shell. Das ist
  eine **benannte Ausnahme von Regel 3** (sonst `os/exec` nur in `transcode`): `forbidigo`
  erlaubt es nur unter `server/cmd/updater`, ADR 0024 hält das fest. Die Docker-Engine-API direkt
  zu sprechen wäre ohne Ausnahme möglich, müsste aber die Logik von Compose (mehrere Dateien,
  Profile, Netze) nachbauen.
- **Genau dieselbe Compose-Zusammenstellung wie beim Start.** Der Stack läuft oft mit mehreren
  Dateien (`deploy/compose.yaml:1-8`: `compose.https-only.yaml`, `compose.intel.yaml` usw.);
  CT 190 läuft mit `compose.yaml`, `compose.ct.yaml` und `compose.intel.yaml`. Ein schlichtes
  `docker compose up -d slouchcat` würde diese Dateien weglassen: Port 8096 stünde wieder im
  Netz, obwohl Caddy davor sein soll, und die GPU fehlte. Der Updater liest darum die Labels des
  laufenden Containers `com.docker.compose.project.config_files` und
  `com.docker.compose.project.working_dir` und ruft Compose mit **genau diesen `-f`-Dateien** und
  `--project-directory` auf. Der Ordner `deploy/` ist im Updater unter **demselben absoluten
  Pfad** eingehängt wie auf dem Gastgeber, damit `./config` und `./cache` stimmen. Fehlen die
  Labels (Start ohne Compose), lehnt der Updater ab, und der Knopf zeigt die Anleitung.
- **Die Version bestimmt der Updater selbst.** Der Server sagt nur «installiere die neueste»; der
  Updater holt das Manifest selbst, prüft die Version streng (`^\d+\.\d+\.\d+$`, neuer als die
  laufende) und schreibt erst dann. So kann eine übernommene Web-App weder eine ältere Version
  erzwingen noch über einen Zeilenumbruch andere Werte in `.env` schreiben.
- **Image-Digest:** Das Manifest bekommt ein Feld `image` mit dem Digest
  (`ghcr.io/slouchcat/slouchcat@sha256:…`), geschrieben von `.github/workflows/release.yml`.
  Heute hat es nur `version`, `url`, `notes`, `publishedAt` (`admin/update.go:56-61`). Der Updater
  vergleicht den gezogenen Digest damit und bricht bei Abweichung ab.
- **`.env`:** Der Updater ändert nur die Zeile `SLOUCHCAT_VERSION=` (fügt sie an, wenn sie fehlt)
  und lässt den Rest Byte für Byte stehen. Die Sicherung `.env.bak-vor-<Version>` enthält
  Geheimnisse (SMTP, Telegram) und bekommt Rechte `0600`; die letzten fünf bleiben.
- **Geheimnis:** Server und Updater teilen ein Token in einem eigenen kleinen Ordner
  `deploy/updater/` (Datei `token`, `0600`), im Server lesend unter `/run/slouchcat-updater`
  eingehängt. **Nicht** in `/config`: Der Updater sieht weder Datenbank noch `secret.key` noch
  Sicherungen. Der Updater hängt nur `deploy/` und `/var/run/docker.sock` ein, hat keinen offenen
  Port und hört nur im internen Docker-Netz. Das Token erscheint nie im Log (Regel 10).
- **Der Updater aktualisiert sich nicht selbst.** Sein Tag steht als `SLOUCHCAT_UPDATER_VERSION`
  in `.env`; ihn hebt man von Hand, wenn ein Release das verlangt (Release-Notizen).

**Ablauf und Rückmeldung.**
1. Server: `POST /admin/update/install` (`x-visibility: admin`) gibt dem Updater den Auftrag.
2. Updater: Manifest holen und prüfen, `.env` sichern, Zeile setzen, `docker compose pull
   slouchcat`, Digest prüfen, `docker compose up -d slouchcat`, auf `healthy` warten
   (Zeitlimit **10 Minuten**, damit lange Migrationen Platz haben).
3. **Zustand ohne Server:** Während des Neustarts gibt es keinen Server, der eine Karte zeigen
   könnte. Der Updater schreibt seinen Zustand in `deploy/updater/state.json` (Phase, von, nach,
   Ergebnis, Migration ja/nein, Meldung) und liefert ihn über `GET /status` im internen Netz.
   Der Server liest ihn über `GET /admin/update/status` (`x-visibility: admin`); die Web-App
   fragt während des Neustarts die Gesundheitsprüfung ab wie nach «Neu starten» und danach den
   Status.
4. **Lief eine Migration?** Der Server schreibt bei jedem Start vor den Migrationen und danach
   eine Zeile in `deploy/updater/server.json` (Version, Schema-Version vor und nach). Daran sieht
   der Updater auch dann, ob eine Migration lief, wenn der neue Server nie gesund wird.
5. **Rücknahme (F-14):** Ohne Migration setzt der Updater bei einem gescheiterten Start die alte
   Zeile zurück, startet die alte Version und hält das im Zustand fest. Mit Migration nimmt er
   nichts zurück und meldet «Migration gelaufen, zurück nur mit der Sicherung
   `pre-migration-…`». Der Server, der danach läuft (alt oder neu), liest den Zustand beim Start
   und schickt **einmal** das Ereignis «Fehler» an die Administratoren.

**Vertrag.** `UpdateInfo` bekommt `updaterAvailable` (der Server erreicht den Updater mit dem
Token) und `install` (Zustand aus `state.json`, Schema `UpdateInstallStatus`); neu sind
`POST /admin/update/install` und `GET /admin/update/status`. Ohne Updater zeigt der Knopf die
Anleitung (Rückfall C).

**Auf der Karte «Server».** Ist eine neuere Version da, steht **«Update auf 1.13.1
installieren»**, mit den Release-Notizen in einem Bestätigungsdialog («Der Server startet neu;
laufende Wiedergaben brechen ab. Vor einer Datenbankänderung sichert er die Datenbank.»). Danach
zeigt die Karte die Phasen (Herunterladen, Neustart, Gesundheitsprüfung, fertig oder
zurückgenommen) und lädt die Seite neu, wenn der neue Server antwortet.

**Nebenbefund beheben.** Nach einer fehlgeschlagenen Prüfung versucht es der Server nach
**15 min, 1 h und 6 h** erneut, das Zeitlimit steigt auf **20 s**. Eigener kleiner Schritt ohne
Vertrag (Stufe 1).

**Einrichtung.** CT 190 bekommt das Profil `updater` für die Abnahme (F-15), mit seinen drei
Compose-Dateien; die Abnahme prüft danach, dass Port und GPU wie vorher stehen.
`docs/INSTALLATION.md` und `deploy/README.md` beschreiben das Profil als freiwillig. Neue
**ADR 0024** «Updater-Begleiter»; Spec A-5 wird erweitert (Hinweis **und** Installieren).

Aufwand: rund **6 Tage** (Updater 3, Release-Lauf und Manifest 0,5, Server und Karte 1, Doku
und Abnahme auf CT 190 1,5).

## 10. Was sich gegenüber früheren Entscheiden ändert

| Früherer Entscheid | Was dieses Konzept entscheidet | Frage |
|---|---|---|
| Konzept 08, N-4 (Marcos Nachtrag 30. Oktober 2026); ADR 0016 | Spotlight-Reihe als Endlosband: aktives Poster immer vorne, gezeigte Titel hinten an, kein Leerraum und keine Endsperre. | F-6 |
| Konzept 08, Wunsch 2 und F-7; ADR 0016 Option e; Entscheidungsprotokoll «Nicht gebaut» | Kachelreihe «Meine Medien» und je Bibliothek eine Reihe «Neu in …», ab Werk ausgeblendet. | F-11 |
| Spec E-9 (höchstens acht Reihen) | Höchstens **fünfzehn** sichtbare Reihen; leere bleiben verborgen. | F-12 |
| Konzept 11, O-3; Konzept 12, S-1 | «Benachrichtigungen» öffnet in der Verwaltung; `/settings/notifications` bleibt für alle. | F-7 |
| Konzept 13 (Kapitelbilder nachholen), Konzept 10 (Originalsprachen nachholen) | Beide werden Zeitplan-Jobs, Zeitplan ab Werk aus. | F-3 |
| Konzept 09 (Pause ist das einzige Halten einer automatischen Jobart, ADR 0017) | Zeitfenster je Jobart für drei schwere automatische Jobs, mit eigenem Merkmal `override_window` für Wiedergaben. Konzept 14 F-3 und ADR 0022 (`override_pause`, höchstens drei) bleiben unverändert. | F-5 |
| `CLAUDE.md` Regel 3 (`os/exec` nur in `transcode`) | Benannte Ausnahme für `server/cmd/updater` (ADR 0024). | F-13 |
| `deploy/compose.yaml`, Kommentar «Update ist ein bewusster Schritt»; Spec A-5 | Update per Knopf, wenn der Updater-Begleiter eingerichtet ist; sonst Anleitung. | F-13 bis F-15 |

Mit dem Baubeginn trägt Schritt 0 **Frage 22** ins Entscheidungsprotokoll ein (alle Antworten aus
Abschnitt 11) und ergänzt Spec E-9 und A-5, Konzept 08 (N-4, F-7), ADR 0016 und ADR 0017 je mit
einem Hinweis auf dieses Konzept.

## 11. Fragen an Marco und seine Antworten (5. November 2026)

| Nr. | Frage | Empfehlung | **Antwort** |
|---|---|---|---|
| F-1 | Weiterschauen: Wohin führt der Klick auf eine Serienkarte? | Detailseite der Folge | **Detailseite der Folge** |
| F-2 | Weiterschauen: Serie bleibt nach fertiger Folge mit der nächsten Folge? | Ja, 60 Tage | **Ja, bis 60 Tage** |
| F-3 | «Kapitelbilder nachholen» und «Originalsprachen nachholen» einplanbar? | Ja | **Ja** |
| F-4 | «automatisch» statt «bei Bedarf»? | Ja | **Ja** |
| F-5 | Zeitfenster für schwere automatische Jobs? | Nicht jetzt | **Ja, in Konzept 15; je Job einstellbar; Wiedergabe sofort, trotz Fenster** |
| F-6 | Spotlight: A, B oder C? | B | **C · Endlosband** |
| F-7 | Benachrichtigungen als Seite in der Verwaltung? | Ja | **Ja** |
| F-8 | Metadaten-Editor: A, B oder C? | B · Seitenblatt | **A · Dialog wie Emby** |
| F-9 | Wo steht der Stift? | Detailseite und Raster | **Detailseite und Bibliotheksraster, nur für den Server-Administrator** |
| F-10 | Sprache der bearbeiteten Texte? | Sprache des Servers | **Sprache des Servers** |
| F-11 | Bibliotheken auf der Startseite: A, B oder C? | A | **«Meine Medien» (B) und je Bibliothek «Neu in …» (A); die Verwaltung zeigt alle** |
| F-12 | Wie viele Reihen? | höchstens 8 sichtbare | **15 Plätze** |
| F-13 | Updater-Begleiter mit Anleitung als Rückfall? | Ja | **Ja** |
| F-14 | Updater nimmt einen gescheiterten Start ohne Migration selbst zurück? | Ja | **Ja; mit Migration nur melden** |
| F-15 | Updater auf CT 190 einrichten? | Ja | **Ja** |
| F-16 | Vier Stufen, je ein Release? | Ja | **Ja** |

## 12. Bauplan in Stufen

| Stufe | Inhalt | Vertrag / Migration | Aufwand |
|---|---|---|---|
| **1 · Web-App und kleine Server-Fixes** | P-2 Beschreibungen, P-3 Anzeige «automatisch» und zwei Jobs einplanbar, P-4 Benachrichtigungen in der Verwaltung, P-5 Spotlight als Endlosband, Wiederholung der Update-Prüfung (9.3) | unverändert, Release 1.13.2 / keine | ≈ 3,5 Tage |
| **2 · Weiterschauen und Zeitfenster** | P-1 mit nächster Folge, P-3 Zeitfenster je Jobart (4.3) | 1.14.0 (additiv) / 00034 (`job_types`, `jobs`) | ≈ 4 Tage |
| **3 · Titelpflege und Startseite** | P-6 Dialog mit vier Reitern, Hochladen, Sperren je Sprache; P-7 «Meine Medien» und «Neu in …», 15 Plätze | 1.15.0 (brechend für `getHome`, `setHomeRows`) / 00035 (`home_rows`) | ≈ 11 Tage |
| **4 · Update installieren** | P-8 Updater-Begleiter, Manifest mit Digest, Knopf, ADR 0024, Doku, Einrichtung und Abnahme auf CT 190 | 1.16.0 (additiv) / keine | ≈ 6 Tage |

- Jeder Planschritt ist ein eigener Commit mit DoD-Belegen der Stufe 1 der Teststufen
  (`task check:fast` und die betroffenen Tests); vor jedem Pull Request `task ci` grün
  (`CLAUDE.md`, «Arbeitsweise»).
- Jede neue oder umgebaute Seite hat ihren Playwright-Tastaturtest (Regel 8): Metadaten-Dialog
  (Tab-Reihenfolge, Escape schliesst, Fokus zurück auf den Stift, keine Fokusfalle ausserhalb des
  offenen Dialogs), Stift im Bibliotheksraster (Link, dann Stift), Admin-Seite
  Benachrichtigungen, Spotlight, Kachelreihe «Meine Medien».
- Abnahme je Stufe auf CT 190; jede Stufe erscheint als eigenes Release (F-16).

## 13. Was nicht dazugehört

- Startseitenreihen **je Profil** (bleiben für den ganzen Server).
- Bearbeiten von **Besetzung**, Studios und Schlagworten; eine Sperre «ganzer Titel» wie in Emby
  (die Schlösser je Feld reichen).
- Texte in anderen Sprachen als der Server-Sprache bearbeiten (F-10).
- Hochladen von Bildern **von einer Adresse** (nur Datei vom Gerät).
- Automatische Updates ohne Knopf, Updates von Vorabversionen, Rückkehr über eine Migration hinweg.
- Zeitfenster für andere als die drei schweren automatischen Jobs.
- Alles aus den Non-Goals der Spec (Regel 11).

## 14. Kritiker-Review von Fassung 2 (6. November 2026)

Der OMC-Kritiker hat Fassung 2 gegen den Code geprüft: **REVISE**, 1 Blocker, 10 Major, 6 Minor.
Die meisten Code-Angaben stimmten. Marcos Entscheide F-1 bis F-16 bleiben; die Befunde betreffen
nur die Umsetzung. Fassung 3 arbeitet alle ein:

| Nr. | Stufe | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Blocker | Updater mit schlichtem `docker compose` verliert Override-Dateien (Port 8096 wieder offen, GPU weg) und relative Pfade. | 9.3: Compose-Dateien aus den Labels des Containers, `deploy/` unter gleichem Pfad, Abnahme prüft Port und GPU. |
| 2 | Major | Regel 3, Digest fehlt im Manifest, Version vom Server vertraut, Geheimnis in `/config`, Veröffentlichung des Updater-Images. | 9.3: benannte Ausnahme, Feld `image` im Manifest, Updater prüft selbst, Token in `deploy/updater/`, eigenes Image. |
| 3 | Major | Kein Weg für Fortschritt und Fehler, wenn der Server unten ist; Migration nicht erkennbar; Vertrag fehlt. | 9.3: `state.json`, `server.json`, `GET /admin/update/status`, `updaterAvailable`. |
| 4 | Major | Kapitelbilder nach Wiedergabe nutzen `override_pause` nicht; die Grenze von drei würde blockieren. | 4.3: eigenes Merkmal `override_window`. |
| 5 | Major | Der Schlüssel ist überall die Kennung einer Reihe; mehrere Bibliotheksreihen würden kollidieren; `labels.ts` hat keinen Rückfall. | 8.3: Kennung `id`, alle Stellen genannt, Vertrag 1.15.0 brechend. |
| 6 | Major | `UNIQUE (key, library_id)` greift in SQLite bei NULL nicht; Löschen von Bibliotheken gibt es nicht. | 8.3: `id` als Primärschlüssel, CHECK, Hinweis. |
| 7 | Major | Sperren gelten heute für alle Sprachen, F-10 stimmte so nicht. | 7.5: Sperren je Sprache für Textfelder. |
| 8 | Major | Sortiertitel ändert die Sortierung nicht (`sort_key`). | 7.5: Sortiertitel schreibt `sort_key` der Server-Sprache. |
| 9 | Major | «Aktualisieren» lädt heute freie Felder neu, nicht nur Fehlendes. | 7.4, 7.5: nur «Jetzt aktualisieren» plus «Sperren aufheben». |
| 10 | Major | Körpergrenze 64 KiB, Ausnahmen nur für feste Pfade. | 7.5: Muster in `largeBodies`, `binaryRoutes`, Pixelgrenze. |
| 11 | Major | Endlosband: DOM-Umordnung stört Bildschirmleser; `holdEnds`. | 6.3: DOM fest, Drehung über CSS `order`, Halt bei Fokus, Endsperre bleibt. |
| 12 | Minor | Umsetzung des Fensters (`STRICT`, CHECK, `Windowable`, Minute in Go). | 4.3. |
| 13 | Minor | Hochladen nicht über `chosen = 1`. | 7.5. |
| 14 | Minor | `ImageKind` kennt `backdrop` schon; Grenze in Go; Einstellung Folgenbilder. | 7.5. |
| 15 | Minor | Stift im Link ungültig; Tastaturtest fürs Raster fehlte. | 7.4, 12. |
| 16 | Minor | Regel für Weiterschauen bei fertiger und halb gesehener Folge offen; leere Kachelreihe. | 2.2, 8.3. |
| 17 | Minor | Nummern im Kopf, Herkunft der Hand-Jobs, drei zu lange Texte, Fundstelle 409, `trigger`-Texte. | Kopf, 4.1, 4.2, 3.3. |

## Anhang A. Mockups

Alle drei sind privat; Marco kann sie teilen. Die Kopien im Repository enthalten keine Filmbilder,
nur Platzhalter-Flächen (das Repository ist öffentlich).

| Nr. | Inhalt | Artefakt | Kopie im Repository |
|---|---|---|---|
| A.1 | Spotlight-Reihe, Varianten A/B/C, Desktop und Handy, Takt 5 s, Pause, Tastatur; gewählt **C** | <https://claude.ai/artifact/MXs2iAKkryJTKLSNZzZMXd> | `docs/planning/mockups/15-spotlight-prototyp.html` |
| A.2 | Metadaten bearbeiten, Varianten A/B/C, Detailseite und Bibliothek, Film und Folge, vier Reiter, echtes Hochladen im Browser; gewählt **A**. Abweichend vom Mockup: kein «Fehlendes ergänzen» im Reiter Aktualisieren (7.5), Stift im Raster neben dem Link (7.4) | <https://claude.ai/artifact/FiyTRoDydz7rJqhEB5eVcP> | `docs/planning/mockups/15-metadaten-prototyp.html` |
| A.3 | Bibliotheken auf der Startseite, Varianten A/B/C mit der Verwaltungsseite Startseitenreihen; gewählt **A und B** | <https://claude.ai/artifact/TFBzRhLp2nHkd5xgUTZQ1J> | `docs/planning/mockups/15-bibliotheken-prototyp.html` |
