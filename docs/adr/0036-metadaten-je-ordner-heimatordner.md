# ADR 0036: Metadaten je Ordner, Heimatordner eines Titels

- Status: gebaut auf dem Zweig `claude/k24-server` (16. November 2026, Server, Vertrag und die
  nötigste Anpassung der Web-App), **angenommen mit dem Merge**; **ersetzt in ADR 0018 den Entscheid E-8** («Keine Übersteuerung je Ordner»: Sprachen und
  Bilder je Bibliothek in `library_metadata`) und **ändert ADR 0015** in einem Punkt (die
  Quellenliste eines Ordners gilt für die Titel, deren Heimatordner er ist, statt für jeden Titel mit
  einer Datei darin samt Serie). ADR 0033 gilt für die Liste eines Ordners wie bisher für die Liste
  einer Bibliothek.
- Quelle: Konzept `docs/planning/24-metadaten-pro-ordner.md` (Fassung 3, 16. November 2026,
  Abschnitte 3 und 9); **Frage 32** in `docs/planning/04-entscheidungen.md` mit Marcos Antworten
  F-1 bis F-6
- Vertrag 1.40.0 (brechend), Migration 00065
- Betroffene Spec-IDs: B-3, B-6, K-2, K-3, C-5
- Harte Regeln: 1 (generierter Code nur durch Codegen), 2 (die neuen Abfragen auf `items` und
  `media_files` liegen in `library` und `metadata`), 5 (eine Migration nach Vorlage, neuer
  eingefrorener Stand `testdata/db/ordner-metadaten.sql`), 7 (`x-visibility: admin` und Eintrag im
  Vertragstest), 12 (der Client löst nichts auf)

## Kontext

Wie Slouchcat die Titel eines Ordners beschreibt, stand an drei Orten: Standard des Servers
(`settings`, `metadata.*`), Abweichung je Bibliothek (`library_metadata`) und Ordner (Inhalt und
Quellenliste in `library_roots`). Marco will am 16. November 2026 nur noch Einstellungen je Ordner
(«Ja, Standard behalten», F-1). Für Einstellungen je Ordner muss jeder Titel eindeutig einem Ordner
gehören. Das tat er nicht: Es gab nur `media_files.root_id`; die Quellenliste wurde beim Anlegen aus
dem gerade gelesenen Ordner kopiert, und eine Serie in zwei Ordnern trug gemischte Listen.

## Entscheid

| Punkt | Entscheid |
|---|---|
| **Zwei Ebenen** | Es gibt den Standard und den Ordner. Ein Video-Ordner (Inhalt Filme oder Serien) hat eigene Textsprachen und eigene Bilder (Arten und Sprachkette je Bildart) oder folgt dem Standard. Bildqualität, Quellen-Schalter und Freigabeland bleiben beim Standard. Hörbuch- und Podcast-Ordner haben keine eigenen Sprachen und Bilder; ein Ordner, der einen solchen Inhalt bekommt, verliert seine. |
| **Speicher** | `library_roots.text_langs` und `library_roots.images` (JSON mit `json_valid`-Prüfung, `NULL` = wie Standard). `images` ist `{"langs":{…},"kinds":[…]}` wie bisher in `library_metadata`: eine fehlende Kette folgt der des Standards, `kinds` ist vollständig, `[]` heisst «keine Logos, Staffelposter, Folgenbilder» und ist gültig. |
| **Heimatordner** | Neue Spalte `items.root_id INTEGER REFERENCES library_roots(id) ON DELETE SET NULL` mit Index `items_root`. Jeder Titel hat den Ordner seiner **ältesten vorhandenen Datei** (`media_files.created_at`, bei Gleichstand der Pfad). Eine Serie und ein Podcast zählen die Dateien ihrer Folgen; Extras zählen nie. **Staffeln, Folgen und Extras erben** den Heimatordner ihres Titels und nehmen nicht den Ordner, den der Scan gerade liest. Ein Podcast nur aus Feeds hat keinen; seine Feed-Folgen auch nicht. |
| **Gesetzt** | beim Anlegen eines Titels (`InsertItem`, `InsertAudiobook`, `PodcastInsertFromFolder`), für Kinder vom Elternteil samt dessen Quellenliste, für Feed-Folgen vom Podcast. |
| **Neu bestimmt** | sobald ein Titel in seinem Heimatordner keine vorhandene Datei mehr hat, aber anderswo eine: am Ende jedes Scan-Stapels für die Titel, deren Dateien der Stapel angelegt, verändert, wiedergefunden oder vermisst hat (Umzug, Rückkehr), beim Entfernen eines Ordners und bei einem anderen Inhalt für die überlebenden Titel, bei der späten Wiederverknüpfung (ADR 0023) für den zurückgekehrten Titel. In derselben Transaktion bekommt der Titel mit seinen Kindern die Quellenliste des neuen Heimatordners, und ein lebender Film oder eine Serie reiht `metadata.match` ein (das Auffüllen folgt), ein Hörbuch seine Buchzuordnung nach den Regeln von `queueBookMatch`. Ein Titel ohne vorhandene Datei behält, was er hat. Solange eine Datei im Heimatordner vorhanden ist, bleibt er, auch wenn anderswo eine ältere liegt. |
| **Pfad ändern** | lässt den Heimatordner stehen; die Kennung des Ordners bleibt (ADR 0023). |
| **Auflösung** | `metadata.Effective` fragt nach dem Titel: eigene Sprachen und Bilder seines Heimatordners, sonst der Standard; ein Titel ohne Heimatordner nimmt den Standard. Die Auflösung ist live, ein geänderter Ordner schreibt keine Titelzeile um. |
| **Quellenliste** | `UpdateItemsProvidersByRoot` schreibt die neue Liste eines Ordners auf die Titel, deren Heimatordner er ist (samt Kindern); die Aufträge nach einer Änderung gehen an dieselben Titel (`ListTitleIDsOfRoot`). Die Buchzuordnung liest die Quellen des Heimatordners (`bookSourcesOf`, `ListAudiobooksToMatch`, `ListAudiobooksInMatchState`). |
| **Sprachen der Profile** | `AvailableMetadataLanguages` = Liste des Standards, dann die eigenen Listen aller Video-Ordner. |
| **TMDB-Bildsprachen** | `include_image_language` = Vereinigung der Bildketten des Standards und aller Ordner mit eigenen Bildern. |
| **Hinweis der Quellen** | `ProviderLanguages` rechnet mit der Liste, die gerade bearbeitet wird; `listMetadataProviders` nimmt sie als `textLangs` (ohne = die Liste des Standards). |
| **Anwenden** | `ApplyMetadataRequest.rootIds` ersetzt `libraryIds`. Leer heisst alle Ordner und dazu die Titel ohne Heimatordner, also jeder zugeordnete Film und jede Serie wie bisher «alle Bibliotheken»; ein Ordner meint die Titel, deren Heimatordner er ist. `affectedTitles` zählt so, der Schlüssel gegen doppelte Läufe nennt die Ordner (`apply:<was>:roots:<Kennungen>`, getrennt von den Schlüsseln der Läufe vor dem Update). Ein wartender Lauf aus der Zeit vor dem Update mit `libraryIds` wird auf die Ordner dieser Bibliotheken umgerechnet; eine Bibliothek ohne Ordner ergibt nichts, nie «alle». |
| **Migration 00065** | Spalten und Index; jede Abweichung aus `library_metadata` auf jeden Video-Ordner ihrer Bibliothek; `items.root_id` nach der Regel (erst Titel mit eigenen Dateien, dann Serien und Podcasts über ihre Folgen, dann erben zwei Ebenen Kinder); `library_metadata` entfällt. |
| **Sichtbar** | Die Titelseite der Verwaltung nennt den Heimatordner (`AdminItem.homeRoot`); ein Ordner nennt die Zahl seiner Titel (`LibraryRoot.homeTitleCount`). |

## Verworfen

- **Sprachen und Bilder wie die Quellenliste auf jeden Titel kopieren:** zwei JSON-Spalten mehr je
  Titel, bei jeder Änderung tausende Zeilen neu schreiben.
- **Bei jedem Auftrag über die erste Datei nachschlagen wie bisher bei Hörbüchern:** eine Abfrage
  mehr je Auftrag, und eine Serie ohne eigene Datei hat keine Antwort.
- **Die Vereinigung aller Ordner eines Titels:** am teuersten, und die Sprache je Bildart lässt sich
  nicht vereinigen.
- **Jeden Scan den Heimatordner streng nach der Regel neu rechnen:** eine ältere Datei, die in einem
  zweiten Ordner wieder auftaucht, würde den Titel umziehen, obwohl er im ersten noch Dateien hat;
  jeder Umzug löst Zuordnen und Auffüllen aus.

## Folgen

- Ein Titel in zwei Ordnern trägt die Liste und die Einstellungen seines Heimatordners, nicht mehr
  gemischte Listen oder die des zuletzt geänderten Ordners (F-4: «Ordner der ältesten Datei»).
- Haben zwei Ordner einer Bibliothek verschiedene Listen, fällt ein Profil bei Titeln ohne seine
  Sprache auf die nächste zurück; zwischen zwei Bibliotheken war das schon so (Abschnitt 3.3 des
  Konzepts).
- Der Vertrag bricht (1.40.0): `getLibraryMetadata` und `updateLibraryMetadata` entfallen. Der
  einzige Client ist die eigene Web-App im selben Release.
- Die Zeit der Migration auf einer grossen Datenbank steht in `docs/perf/24-ordner-metadaten.md`.
