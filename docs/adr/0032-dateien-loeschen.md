# ADR 0032: Dateien eines Titels löschen

- Status: angenommen (12. November 2026); **hebt Spec B-1 («read-only») in einem Punkt auf**: Slouchcat darf
  in einem Medienordner Dateien löschen, wenn der Betreiber diesen Ordner selbst und bewusst schreibbar
  eingebunden hat, und nur auf ausdrücklichen Befehl des Server-Administrators. **Schreibt ADR 0029 fort**
  (Konzept 19 F-4 «Slouchcat löscht nie eine Datei im Podcast-Ordner» gilt weiter für alles Automatische,
  nicht mehr für das Löschen von Hand).
- Quelle: Konzept `docs/planning/20-bedienung-und-gesehen.md` Abschnitte 9 bis 11, 16, 18 und 19;
  **Frage 27** in `docs/planning/04-entscheidungen.md` (F-3, F-13, O-4 bis O-6, O-11, O-13, O-14); Messung M-1
  `docs/perf/20-bedienung.md` Abschnitt 5
- Vertrag 1.31.0 (additiv: `deleteLibrary`, `getItemDeletion`, `deleteItem`, `listRemovedItems`,
  `restoreRemovedItem`, `LibraryRoot.readOnly`, `Problem.playingProfiles`), Migration 00055
- Betroffene Spec-IDs: B-1, B-9, H-2, A-3
- Harte Regeln: 2 (alles in `library`), 3 (kein `os/exec`), 4 (gelöscht wird nur über `paths`), 5 (Migration
  nach Vorlage), 7 (`x-visibility: admin`), 10 (keine Pfade im Aktivitätsprotokoll), 12 (Entscheid im Server)

## Kontext

Bis 1.30 konnte Slouchcat keine Mediendatei löschen, und das war Absicht: `deploy/compose.yaml` bindet die
Medien mit `:ro` ein, die Spec verlangt es (B-1), ein Abnahmetest prüft, dass Schreiben nach `/media`
scheitert, und das Protokoll bekräftigt es. Einzige Ausnahme ist seit ADR 0029 der Podcast-Ordner, in den
Slouchcat neue Folgen schreibt; gelöscht hat es dort nur seine eigenen Hilfsdateien.

Marco will einen Titel aus Slouchcat heraus löschen können und dabei jedes Mal wählen, ob nur der Eintrag in
Slouchcat verschwindet oder auch die Dateien auf der Platte (F-3). Zu entscheiden war:

1. Wie wird ein Ordner löschbar, ohne dass ein Fehler in Slouchcat jemals eine Datei in einem nur lesbaren
   Ordner trifft?
2. Woran erkennt der Server, ob er löschen darf, ohne dafür etwas zu schreiben?
3. Was genau wird gelöscht, und was nie?
4. Was geschieht in der Datenbank, wenn alle oder nur einige Dateien weg sind?
5. Wie wird verhindert, dass später Code an `paths` vorbei löscht?

Verworfen: (a) eine Einstellung «Dateien löschen erlauben» in der Verwaltung (eine Schutzschicht, die ein Klick
aufhebt, schützt nicht; der `:ro`-Schalter liegt beim Betreiber und gilt auch bei einem Fehler im Code);
(b) eine Schreibprobe (Datei anlegen und wieder löschen) als Prüfung: sie prüft den Bibliotheksordner statt des
Ordners der Datei und weckt die Ordner-Überwachung von Slouchcat, Radarr und Sonarr; (c) ein Papierkorb-Ordner
(O-6: bräuchte Platz, eine Frist und eine eigene Ansicht); (d) rekursives Löschen des Titelordners (nimmt
mit, was Slouchcat nicht kennt); (e) weiches Löschen des Titels, wenn alle Dateien weg sind (gegen O-14).

## Entscheid

1. **Vorgabe bleibt nur lesbar.** Das mitgelieferte `compose.yaml` bindet Medien weiter mit `:ro` ein. Wer aus
   Slouchcat Dateien löschen will, legt einen Ordner selbst und bewusst schreibbar darüber
   (`docs/INSTALLATION.md`, «Dateien aus Slouchcat löschen»). Der `:ro`-Schalter bleibt die harte
   Sicherung: Ohne ihn umzulegen, kann kein Fehler in Slouchcat eine Datei löschen.
2. **Prüfung ohne Schreiben, statt einer Einstellung.** Je Ordner der Bibliothek fragt der Server mit `fstatfs`,
   ob das Dateisystem nur lesbar eingehängt ist (`ST_RDONLY`); der Dialog «Ordner von …» zeigt «schreibbar»
   oder «nur lesbar» (`LibraryRoot.readOnly`, mit einer Frist von zwei Sekunden gegen hängende Freigaben). Je
   Datei der Vorschau prüft er mit `faccessat(W_OK|X_OK, AT_EACCESS)`, ob er im **Elternordner dieser Datei**
   löschen darf (Ordner von Radarr und Sonarr haben oft eigene Besitzer und Rechte). Die Vorschau meldet je
   Datei `deletable` und `reason` (`read_only`, `no_permission`, `shared`, `not_regular`, `unreachable`,
   `missing`). Ist auch nur eine Datei nicht löschbar, ist «Auch die Dateien löschen» grau mit dem Grund, und
   der Server lehnt den Auftrag ab (`files_not_deletable`), ohne etwas zu löschen.
3. **Genau diese Liste, in dieser Reihenfolge:**
   1. die Mediendateien des Titels, aller Titel darunter und seiner Extras;
   2. die Bild- und Metadatendateien, die Slouchcat für diese Titel gelesen hat: die verzeichneten Bilder
      (`item_local_images`), das Ordnercover eines Hörbuchs oder Podcasts (`items.cover_path`) und die Dateien
      mit festem Namen (`movie.nfo`, `tvshow.nfo`, `season.nfo`, `metadata.json`) eines Ordners, sobald dort
      und darunter keine Mediendatei irgendeines Titels mehr liegt;
   3. Begleitdateien neben einer gelöschten Mediendatei, deren Name mit dem **ganzen Dateistamm** beginnt,
      gefolgt von `.` oder `-` (`Film (2024).de.srt`, `Film (2024)-poster.jpg`);
   4. danach der Ordner, wenn er leer ist, und leere Elternordner bis unter den Bibliotheksordner.
4. **Nie:** eine Datei, die in `media_files` einem Titel gehört, oder ein verzeichnetes Bild eines Titels
   ausserhalb, auch wenn ihr Name nach Regel 3 passte (flacher Filmordner: `Alien.mkv` und
   `Alien-Resurrection.mkv`); als Begleitdatei nie eine Datei mit der Endung einer Video- oder Audiodatei
   (auch wenn noch kein Scan sie kennt) und nie eine Datei, deren Gerät und Inode die einer Mediendatei
   irgendeines Titels sind; eine Mediendatei, die als harter Link zugleich die Datei eines anderen Titels
   ist (sie bleibt, das Ergebnis nennt sie); ein Symlink (wird nie verfolgt, bleibt liegen); der
   Bibliotheksordner selbst; ein Ordner mit Inhalt (kein rekursives Löschen; das Ergebnis zählt, was liegen
   bleibt). Eine schon fehlende Datei gilt als gelöscht.
5. **Reihenfolge und Datenbank.** Spielt ein Profil einen der Titel ab, lehnt der Server ab (409 `playing`,
   O-13) und beendet keine Wiedergabe. Sonst unter der Scan-Sperre der Bibliothek (ohne zu warten:
   `scan_running`): erst die Dateien, dann die Datenbank. Die Zeilen der gelöschten Dateien fallen sofort weg,
   damit kein Player eine gelöschte Version anbietet. Bleibt vom ganzen Titel keine Datei, wird er **endgültig**
   gelöscht, samt Fortschritt, Haken und Merkliste (O-14); er bekommt weder `deleted_at` noch `removed_at`.
   Scheitert eine Datei, bleibt der Titel mit den verbliebenen Dateien, Titel darunter ohne Datei und leer
   gewordene Staffeln gehen endgültig, und das Ergebnis nennt die gebliebenen Dateien. Eine Podcast-Folge mit
   Enclosure bleibt als Verweis aus dem Feed. Jedes Löschen schreibt ein Aktivitätsereignis `item_deleted`
   (wer, Titel, Wahl, Zahl der Dateien, Bytes; kein Pfad).
6. **Kein automatisches Löschen.** Kein Aufräumjob, keine Regel «nach dem Sehen löschen», keine Frist, kein
   Löschen bei Platzmangel; das Löschen einer Bibliothek entfernt nie eine Datei. Löschen ist nur der
   Server-Administrator (`x-visibility: admin`, in der Web-App nur das Hauptprofil des Administrators).
7. **Lint-Regel.** `Remove` und `RemoveAll` auf `*os.Root` sind ausserhalb von `internal/paths` verboten
   (`forbidigo`, Muster `^os\.Root\.Remove(All)?$`). Gelöscht wird unter einer Wurzel nur mit
   `paths.Remove` (nur gewöhnliche Dateien, `lstat` ohne Folgen von Symlinks, `..` und Symlinks nach
   aussen abgelehnt; eine Mediendatei nur, wenn Gerät und Inode noch die aus `media_files` sind, sonst
   bleibt sie mit dem Grund `changed`) und `paths.RemoveEmptyDir` (nur leere Ordner, nie die Wurzel). Die bestehenden Löschungen
   im Cache (Bilder, Trickplay, Kapitelbilder, Offline-Pakete, Radiologos, Anime-Listen) und der eigenen
   Hilfsdateien eines Downloads (`internal/fetch`, Podcast-Downloads nach ADR 0029) tragen an jeder Stelle ein
   `//nolint:forbidigo` mit Begründung; `os.Remove` auf `/config` und `/cache` bleibt erlaubt.
8. **Radarr und Sonarr wissen von nichts.** Slouchcat steuert sie nicht (Nicht-Ziel der Spec); der Dialog
   weist darauf hin. Lädt Radarr denselben Film später neu, erscheint er als neuer Titel ohne Benutzerdaten.

## Folgen

- B-1 gilt für die Vorgabe weiter. Der Abnahmetest «Schreiben nach `/media` scheitert» bleibt gültig und hat ein
  Gegenstück für den schreibbaren Fall: `web/e2e/admin-item-delete.spec.ts` löscht im schreibbaren Medienordner
  des Testlaufs einen Film samt Datei, `server/internal/library/delete_files_test.go` prüft die Liste, die
  Grenzen (flacher Ordner, harter Link, Symlink nach aussen, Teilfehler) und die graue Wahl bei einem nur
  lesbaren Ordner, `server/internal/paths/remove_linux_test.go` die beiden Funktionen. Für die Abnahme auf
  CT 190 wird ein eigener Testordner schreibbar eingebunden, die echten Medien bleiben nur lesbar.
- Der Schaden eines Fehlers ist auf schreibbar eingebundene Ordner begrenzt und dort auf die Dateien eines
  Titels, den der Administrator mit Namen bestätigt hat (bei mehr als einer Datei getippt).
- Die Prüfung sieht ACLs und die Regeln einer Netzfreigabe nur so weit, wie der Kernel sie sieht; scheitert
  das Löschen trotzdem, bleibt der Titel mit der Datei, und das Ergebnis nennt sie.
- Ein Podcast-Ordner, der für ADR 0029 schreibbar eingebunden ist, ist damit auch für das Löschen von Hand
  schreibbar. Das ist gewollt (Abschnitt 16 des Konzepts).
