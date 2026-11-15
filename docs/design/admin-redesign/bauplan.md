# Verwaltungs-Redesign: Bauplan

Stand: 15. November 2026. Zweig `claude/admin-redesign` auf `main` `55b72c9a` (Release 1.38.0).
Auftrag und Übergabepaket: `handoff/` (README, `docs/01`–`05`, `data/`). Gestaltungsvorlage ist das
klickbare Mockup `Slouchcat_Verwaltung_V2.html` (ausserhalb des Repos, siehe `BRIEF-INVENTAR.md`).
Bestand: `inventory/*.csv` und `inventory/*.md` (Phase 0). Abdeckung: `funktionsabdeckung.md`.

Oberste Bedingung: **keine bestehende Funktion, Option oder Einstellmöglichkeit geht verloren.**
Das Redesign ändert nur die Web-Oberfläche. Kein neuer Endpunkt, keine Migration, keine neue
Berechtigung, keine geänderte API-Semantik. Bestehende Adressen (`/admin/...`, `?tab=`, `?item=`,
`?days=` usw.) bleiben gültig.

## 1. Navigationsmodell

Sechs Einträge in der Seitenleiste statt vier Gruppen mit 19 Einträgen. Die Seiten eines Bereichs
stehen als **Bereichsreiter** über jeder Seite des Bereichs.

| Bereich (`area`) | Seiten in dieser Reihenfolge | Symbol |
|---|---|---|
| Übersicht | `/admin` | `grid` |
| `media` Medien | libraries, metadata, maintenance, podcasts, radio, audiobookSources | `folder` |
| `people` Nutzer & Geräte | accounts, devices, streams | `users` |
| `home` Startseite | homeRows, spotlight, picks, collections, myMedia, announcements | `star` |
| `automation` Automatisierung | jobs, integrations, notifications | `refresh` |
| `system` System | stats, backup, server | `server` |

- `web/src/features/admin/sections.ts` bleibt die eine Quelle: `adminAreas`, `adminSections` mit
  `area` statt `group`. Neu sind zwei Einträge: `audiobookSources` (`/admin/metadata/audiobooks`,
  bisher nur über «Metadaten» erreichbar; der Verweis dort bleibt) und `server` (`/admin/server`,
  neue Seite, siehe 4).
- Welche Seite «aktuell» ist, entscheidet der **längste passende Pfad**: `/admin/metadata/audiobooks`
  gehört zu `audiobookSources`, nicht zu `metadata`; `/admin/items/…` weiter zu `maintenance`,
  `/admin/podcasts/:id` zu `podcasts`.
- Der Eintrag eines Bereichs führt auf seine erste Seite. Er ist «aktuell», solange eine Seite des
  Bereichs offen ist.
- Titel der Seiten bleiben, wie sie in `admin.sections.*.title` stehen (z. B. «User-Empfehlungen»,
  «Ankündigungen», «Sicherung»): Das Mockup nennt sie teils anders, die Namen im Produkt sind
  entschieden und stehen auch auf der Zuschauerseite.

### Zahlen und Hinweise (heute an 6 von 19 Einträgen)

Keine Zahl geht verloren; sie wandern vom Seitenleisten-Eintrag an den Bereichsreiter der Seite. Der
Bereichseintrag der Seitenleiste zeigt nur, was einen Blick braucht:

| Zahl heute | Bereichsreiter | Bereichseintrag |
|---|---|---|
| Bibliotheken: Titelzahl (gedämpft) | libraries | – |
| Pflege: offene Fälle (Warnpille) | maintenance | Medien: dieselbe Warnpille |
| Konten: Zahl (gedämpft) | accounts | – |
| Streams: laufende (Akzentpille) | streams | Nutzer & Geräte: dieselbe Pille |
| Jobs: fehlgeschlagen (Gefahrpille), sonst laufend (gedämpft) | jobs | Automatisierung: Gefahrpille, wenn fehlgeschlagen |
| Integrationen: stille Webhooks (Warnpille), sonst «3 / 8» | integrations | Automatisierung: Warnpille, wenn kein Job fehlschlug |
| Übersicht: Abzeichen «Update» | server | Übersicht **und** System: Abzeichen «Update» |

Die gesprochenen Namen (`admin.nav.counts.*`) bleiben und gelten für Reiter und Bereichseintrag.

## 2. Rahmen

### 2.1 Kopfzeile

Die Kopfzeile der App (`AppHeader`) bleibt auch in der Verwaltung stehen: Logo (zur Startseite),
Glocke, Profil. So bleibt alles erreichbar, was heute von jeder Verwaltungsseite aus geht. In der
Verwaltung (`data-shell="admin"`) kommt dazu:

- **Zusatz am Logo**: kleine Zeile «Control Center» (Text aus den Sprachdateien).
- **Brotkrumen** ab 768 px in der Kopfzeile: «Verwaltung › Medien / Bibliotheken». «Verwaltung»
  führt zur Übersicht, der Bereich auf seine erste Seite, die Seite ist `aria-current="page"`. Unter
  768 px stehen die Brotkrumen als erste Zeile im Inhalt (wie im Mockup).
- **«Bereich suchen»** (Knopf mit Kürzel-Hinweis) statt des Verweises «Suche»: öffnet die
  Suchpalette (2.4). Die Mediensuche bleibt über die Taste `/` und als Eintrag der Palette erreichbar.

Der Demo-Punkt («Demo-Modus») und der Stern-Knopf des Mockups haben keine Entsprechung im Produkt
und entfallen.

### 2.2 Seitenleiste (ab 768 px)

`AdminSideNav` (in `web/src/ui`) im Aussehen des Mockups: Überschrift «Verwaltung» in kleinen
Grossbuchstaben, darunter die sechs Einträge mit Symbol; der aktuelle Eintrag mit Akzentfläche,
Akzentstrich links und Pfeil rechts; Pille oder Abzeichen am Ende des Eintrags. Zuoberst bleibt
**«Zurück zur App»** (fehlt im Mockup, ist bestehende Funktion). Am Fuss steht das aktive Profil mit
Rolle («Administrator») als Anzeige.

Bedienung unverändert: ein Tab-Stopp, Pfeiltasten zwischen den Einträgen, Pos1/Ende,
`focusCarry` über Seitenwechsel, Pfeiltasten zwischen Kopf, Leiste und Inhalt (`shellFocus`).

### 2.3 Handy (unter 768 px)

Der Knopf «Verwaltung · Seite» über jeder Seite bleibt und öffnet das Blatt: «Zurück zur App»,
«Übersicht» und die fünf Bereiche mit allen ihren Seiten (wie heute die vier Gruppen), damit jede
Seite ein Tippen entfernt bleibt. Die Bereichsreiter stehen auch auf dem Handy über der Seite und
brechen um.

### 2.4 Suchpalette

Dialog (`role="dialog"`, Fokusfalle, Esc schliesst, Fokus zurück auf den Knopf). Suchfeld, darunter
Liste: Übersicht, jeder Bereich, jede Seite (mit Bereich als Zusatz), dazu «Zurück zur App» und
«Mediensuche». Tippen filtert (Titel und Bereich, ohne Gross-/Kleinschreibung), Pfeiltasten wählen,
Enter öffnet. Kürzel: `Strg+K` / `⌘K` auf jeder Verwaltungsseite, nicht in Eingabefeldern eines
offenen Dialogs. Die Palette lädt mit dem Verwaltungs-Bündel, nicht mit dem Startbündel (P-10).

### 2.5 Bereichsreiter

Navigation (`<nav aria-label="…">`, Verweise, `aria-current="page"`), im Aussehen der Reiterleiste
des Mockups (abgerundeter Behälter, aktiver Reiter hervorgehoben), mit Zahl oder Pille nach 1. Sie
steht über dem Seitenkopf jeder Verwaltungsseite, auch der Unterseiten (Podcast, Titel). Die
seiteneigenen Reiter (`?tab=`) bleiben darunter in ihrem heutigen Aussehen (Unterstreichung), damit
beide Ebenen unterscheidbar sind. Die Übersicht hat keine Bereichsreiter.

### 2.6 Seitenkopf (`AdminPage`)

- Zeile «MEDIEN / BIBLIOTHEKEN» in Akzentfarbe über dem Titel (aus dem Navigationsmodell).
- Titel gross (Mockup: 25 bis 33 px), darunter die Einleitung der Seite **sichtbar** (bisher hinter
  dem Hilfe-Knopf), rechts die Hauptaktion(en). Der Hilfe-Knopf entfällt, sein Text steht offen.
- «Live, alle n Sekunden» bleibt neben dem Titel.
- `AdminPage` bekommt Bereichsreiter, Brotkrumen (Handy) und die Zeile über dem Titel aus einem
  Kontext, den `AdminLayout` setzt. `/settings/*` nutzt `AdminPage` ohne diesen Kontext und bleibt,
  wie es ist.

### 2.7 Aussehen

Nur vorhandene Tokens (`packages/design-tokens`): `bg.base`, `bg.elevated`, `surface.1–3`,
`text.*`, `border.subtle`, `state.*`, `--sc-accent` (Profilakzent, nicht fest Ember). Neue
Komponenten-Tokens nur, wo ein Mass fehlt. Das neue Aussehen gilt **nur unter
`data-shell="admin"`** (wie die kompakte Dichte, ADR 0020); Zuschauerseiten und `/settings/*`
bleiben unverändert. Muster des Mockups:

- Karte: Fläche `bg.elevated`, Rand `border.subtle`, Radius `radius.lg`, Kopf mit Titel und Zusatz.
- Kennzahl-Kachel: Beschriftung klein, Zahl gross, Unterzeile gedämpft; als Verweis fokussierbar.
- Tabelle: Kopf in kleinen Grossbuchstaben, Zeilen mit feiner Trennlinie, Aktion am Zeilenende.
- Abzeichen: Farbe des Zustands auf 16 % Tönung (`color-mix`, wie die Pillen heute).
- Hinweisband: Warnfarbe getönt, Symbol, Text, Aktion.

## 3. Übersicht (`/admin`)

Aufbau nach Mockup, mit echten Daten des Dashboards (`dashboardQuery`, Abfrage alle 5 s wie heute):

1. Kopf «Übersicht» mit «Aktualisieren».
2. **Hinweisband**, wenn etwas einen Blick braucht: Zahl der Punkte, Knopf springt zur Liste.
3. **Vier Kennzahlen** (Verweise): laufende Streams, laufende Jobs (wartend, pausiert), Titel zur
   Pflege, Speicherplatz.
4. **«Was benötigt Aufmerksamkeit?»**: alle Hinweise des Dashboards (fehlgeschlagene Jobs, stille
   Webhooks, nicht eingerichtete oder fehlerhafte Integrationen, Update verfügbar, Speicher knapp,
   inotify, Hardware-Transcoding, weitere Hinweise wie heute), je mit Verweis auf die Seite.
   Leerzustand: «Alles in Ordnung».
5. **«Gerade aktiv»**: laufende Jobs mit Fortschritt, laufende Streams (heutige Live-Kacheln),
   nächster geplanter Job.
6. **«Direkt zu deinen Aufgaben»**: Schnellzugriffe auf die Bereiche.
7. **«Server & letzte Aktivitäten»**: Zustand in Kurzform mit Verweis «Serverdetails», Aktivität
   (heutiger `ActivityFeed`), Fehlerprotokoll in Kurzform mit Verweis.

Was die heutige Übersicht darüber hinaus zeigt, steht ungekürzt auf **System → Server**; die
Zuordnung jedes Elements steht in `inventory/automatisierung-shell.md`.

## 4. System → Server (`/admin/server`, neue Seite)

Nimmt die Serverfunktionen der heutigen Übersicht auf, ohne Kürzung: Version und Laufzeit,
Update-Hinweis mit Update-Ablauf, Neustart (`RestartPanel`), Hardware-Transcoding, Dateiwächter
(inotify), Speicher `/config` und `/cache`, Fehlerprotokoll, Aktivität in voller Länge. Verweise
auf «Integrationen» für öffentliche Adresse und Update-Prüfung (bleiben dort). Die Seite ist eine
reine Web-Route; sie braucht einen Playwright-Tastaturtest (CLAUDE.md Regel 8).

## 5. Seiten der Bereiche

Jede Seite behält ihre Funktionen, Dialoge, Bestätigungen, Suchparameter, Abfragen und Tests. Der
Umbau je Seite folgt dem Mockup, soweit der Bestand es trägt:

- **Listen** (Bibliotheken, Podcasts, Konten, Geräte, Sammlungen, Mitteilungen, Sicherungen, Jobs):
  Suchleiste und Filter über der Liste, Zustand als Abzeichen, sprechende Aktion am Zeilenende,
  Einzelheiten beim Öffnen der Zeile.
- **Einstellungen** (Metadaten, Hörbuch-Quellen, Radio-Einstellungen, Integrationen,
  Benachrichtigungen): ein Abschnitt (Karte) je Thema mit Titel und Zusatz; Speichern bleibt
  ausdrücklich (kein stilles Speichern, wo heute gespeichert wird); Abfrage bei ungespeicherten
  Änderungen bleibt.
- **Integrationen**: Kachelübersicht der Dienste mit Zustand; jeder Dienst öffnet seine eigene
  Ansicht (Deeplink `?item=` bleibt).
- **Hintergrundjobs**: vier Kennzahlen (läuft, wartend, Fehler, pausiert), Filter, Liste der
  Jobtypen mit Zustand; Zeitplan, Läufe und Aktionen in der Einzelansicht wie heute.
- **Startseite**: Editor der Reihen links, Vorschau rechts (wenn der Bestand eine Vorschau hat;
  sonst keine erfinden).
- **Riskante Aktionen** behalten Warnung und Bestätigung unverändert.

Die Karte «Optionen & Arbeitsabläufe» des Mockups ist ein Gerüst des Prototyps (Liste der
Detailkonzepte) und wird nicht gebaut; die Funktionen stehen auf den echten Seiten.

## 6. Schritte

Jeder Schritt ist ein eigener Commit mit `task check:fast` und den gezielten Tests des Schritts.

| Nr. | Schritt | Inhalt |
|---|---|---|
| 0 | Bestand | `inventory/` (vier Teile), dieser Bauplan |
| 1 | Rahmen | Navigationsmodell, Seitenleiste, Kopfzeile, Brotkrumen, Suchpalette, Bereichsreiter, Seitenkopf, Aussehen unter `data-shell="admin"`; Tests der Navigation |
| 2 | Übersicht und Server | neue Übersicht, neue Seite `/admin/server` |
| 3a | Medien | Bibliotheken, Metadaten, Hörbuch-Quellen, Pflege, Titelseite, Podcasts, Webradio |
| 3b | Nutzer & Geräte, Startseite | Konten, Geräte, Streams, Startseite, Spotlight, Empfehlungen, Sammlungen, Meine Medien, Ankündigungen |
| 3c | Automatisierung, System | Hintergrundjobs, Integrationen, Benachrichtigungen, Statistiken, Sicherung |
| 4 | Prüfung | unabhängige Funktionsprüfung gegen `inventory/`, Korrekturen |
| 5 | Abschluss | Übersetzungen, Bildschirmfotos, `funktionsabdeckung.md`, Abweichungen, Status, `task ci`, PR |

3a bis 3c laufen nebeneinander (höchstens drei Executor), jeder nur in den Dateien seiner Seiten.
Gemeinsame Bauteile ändert nur Schritt 1; braucht ein Seitenschritt ein neues gemeinsames Bauteil,
meldet er es dem Koordinator.

## 7. Abweichungen vom Mockup

Die vollständige Liste nach dem Bau steht in `abweichungen.md`; hier die Grundsätze.

| Punkt | Mockup | Umsetzung | Grund |
|---|---|---|---|
| Kopfzeile | eigene Leiste ohne App-Kopf | Kopfzeile der App mit Brotkrumen und «Bereich suchen» | Glocke, Profil und Logo bleiben erreichbar |
| «Zurück zur App» | fehlt | bleibt zuoberst in der Seitenleiste | bestehende Funktion |
| «Optionen & Arbeitsabläufe» | Karte auf jeder Seite | entfällt | Gerüst des Prototyps |
| Demo-Punkt, Stern-Knopf | in der Kopfzeile | entfallen | keine Entsprechung im Produkt |
| Seitentitel | teils andere Namen | Namen des Produkts | entschieden, auch auf Zuschauerseite |
| Beispielwerte, Beispielaktionen | Formulare im Mockup | nur, was der Bestand kann | Übergabepaket: nicht als Anforderung lesen |

### 7.1 Widersprüche zwischen Mockup und Bestand (aus dem Inventar): es gilt der Bestand

| Mockup | Bestand, der bleibt |
|---|---|
| D002 «Keyframe-Vorschau aktivieren» als Schalter; Freigabestufe für jede Bibliothek | «Vorschau beim Spulen» mit vier Modi je Filme und Serien, «Speichern», «Umfang»; Freigabestufe nur bei Hörbüchern und Podcasts |
| D004 «Dateien auf Datenträger ebenfalls löschen» beim Löschen einer Bibliothek | Bibliothek löschen entfernt nie Dateien; nur das Löschen eines Titels kennt «Auch die Dateien löschen» |
| D005 Hörbuch-Quellen unter Bibliotheken | eigene Seite `/admin/metadata/audiobooks`, als Bereichsreiter unter Medien, Verweis aus «Metadaten» bleibt |
| D023 Schalter «Heruntergeladene Dateien löschen» | Dateien entfernt der nächste Abgleich; kein Schalter |
| D029 Einladung mit E-Mail, Rolle und wählbarer Ablaufzeit | Einladung erzeugt ein Konto ohne Verwaltung, gilt sieben Tage, Link wird einmal gezeigt |
| D030 Rolle als Feld | Rolle als Menüpunkt mit Bestätigung, nur für den Eigentümer |
| D031 Profile fremder Konten bearbeiten | nur die Profile des eigenen Kontos («Meine Profile») |
| D039 Spotlight-Quelle «Automatisch/Manuell» | Quelle ist immer eine Trakt-Liste (sieben Arten) |
| D049 veröffentlichte Mitteilung bearbeiten | veröffentlichte Ankündigung ist nicht mehr änderbar |
| D065 «Mit Passwort verschlüsseln» als Schalter | Sicherung mit Schlüsseln ist immer verschlüsselt, ohne Schlüssel nie |
| D067 «Dateiwächter Grenze» als Eingabe, «Diagnose kopieren» | Anzeige und sysctl-Befehl zum Kopieren; das Limit setzt der Host |
| Speicher als eine Zahl («nicht doppelt zählen») | immer beide Pfade, Konfiguration und Cache (Entscheid Konzept 11, F-4) |
| D069 «Aktivität ansehen» als längere Liste | die API liefert höchstens sechs Einträge; die werden gezeigt |

Beispiele des Mockups ohne Basis im Code (je Seite im Inventar, Abschnitt «Abgleich mit dem
Mockup») werden nicht gebaut. Ausnahmen, weil sie nur Verweise oder ein erneutes Laden sind und
keine Funktion erfinden: die Suchpalette, die Schnellzugriffe der Übersicht, «Aktualisieren» im
Kopf der Übersicht.

### 7.2 Festlegungen zu offenen Punkten des Inventars

- **Grafikchip-Wahl, öffentliche Adresse, Update-Prüfung** bleiben auf «Integrationen» (Band
  «Server», im Mockup die Kachel «Server & Updates»). Die Seite «Server» verweist dorthin. Texte,
  die «Übersicht › Neu starten» nennen, nennen neu «System › Server».
- **Zustand der Integrationen und Webhooks** (acht Schlüssel mit vier Zuständen und Grund, «x von
  y», Meldungen der letzten 24 Stunden, stille Webhooks): Fehler und stille Webhooks stehen auf der
  Übersicht unter «Was benötigt Aufmerksamkeit?»; die ganze Liste steht neu auf «Integrationen» an
  den Kacheln der Dienste (gleiche Abfrage `getDashboard`, keine neue Datenquelle).
- **Fehlerprotokoll** und seine Ansage «N neue Fehler» stehen auf «Server» (Abfrage alle 5 s wie
  heute, A-3); die Übersicht nennt die Zahl mit Verweis.
- **Laufende Update-Installation**: Ablauf und Anschluss auf «Server»; die Übersicht zeigt sie im
  Hinweisband mit Verweis.
- **Titel der Übersicht**: «Alles im Blick» (Mockup), Zeile darüber «Übersicht». Alle anderen Seiten
  behalten ihren Titel.

## 8. Rückweg

Das Redesign ist ein eigener Zweig und ein eigenes Image. Es enthält keine Migration; der Wechsel
zurück auf das bisherige Erscheinungsbild ist auf CT 190 das Zurücksetzen des Image-Tags
(`.env`-Sicherung daneben, Skripte `design-alt.sh` und `design-neu.sh`) und kostet keine Daten.
