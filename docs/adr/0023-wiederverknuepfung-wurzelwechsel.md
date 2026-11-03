# ADR 0023: Wiederverknüpfung gelöschter Titel und «Pfad ändern»

- Status: angenommen (3. November 2026); **ändert Plan 3.2** (Löschschutz: Frist der
  Wiederverknüpfung) und **ergänzt Idee 02 / ADR 0015** (Ordner je Bibliothek: neuer Pfad für einen
  bestehenden Ordner)
- Quelle: Idee `docs/ideas/04-wurzelwechsel-benutzerdaten.md` (Analyse und Messung auf CT 190,
  Abschnitte 1 bis 4; Antworten in Abschnitt 7); **Frage 21** in `docs/planning/04-entscheidungen.md`
  mit Marcos Antworten aus dem Interview vom 3. November 2026; Issue #73
- Betroffene Spec-IDs: A-3 (Scan, Löschschutz), B-9 (Pflege)
- Harte Regeln: 2 (Titelabfragen nur in `library`, `admin`, `metadata`), 7 (`x-visibility: admin`,
  Vertragstest), 8 (Tastaturtests), 9 (Texte aus den Locales), 12 (Entscheid im Server)

## Kontext

Am 28. Oktober 2026 wurde auf CT 190 der Bibliothek «Serien» ein neuer Ordner hinzugefügt und der
alte entfernt. Der Scanner legte die zurückkehrenden Serien mit **neuen IDs** an. Fortschritt,
Wiedergabe-Ereignisse und Sitzungen hingen danach an soft-gelöschten Titeln: 3 von 16 Fortschritten
und 140 von 244 Ereignissen des Servers.

Ursache (Idee 04, Abschnitt 2):

- Der Scanner findet gelöschte Titel nur über Anbieter-IDs, die die Datei selbst mitbringt (NFO,
  `[tvdb-…]`), und nur 30 Tage lang (`relinkWindow`).
- Ein Titel ohne Jahr im Ordnernamen ist an den Ordner derselben Wurzel gebunden.
- Nach `metadata.match` tragen neue Kopie und gelöschter Titel dieselbe ID, aber nichts verknüpft
  sie.
- Ein Ordner kann keinen neuen Pfad bekommen. Wer umzieht, muss hinzufügen und entfernen, und das
  Entfernen löscht die `media_files` des alten Ordners mit.

## Entscheid

1. **Späte Wiederverknüpfung nach dem Match, die alte ID bleibt (B1).** Gibt `metadata.match`
   einem lebenden Film oder einer lebenden Serie eine TMDB-, TVDB- oder IMDb-ID, die ein
   soft-gelöschter Titel trägt, führt das Paket `library` die beiden zusammen. Bedingungen: gleiche
   Bibliothek, gleiche Art (`movie`, `series`), keine widersprüchliche ID. Bei mehreren Kandidaten
   gewinnt der zuletzt gelöschte. Der gelöschte Titel lebt wieder auf und behält seine ID. Die
   Kopie geht in ihm auf:
   - Dateien wandern zum alten Titel.
   - Staffeln werden nach Staffelnummer zusammengeführt, Folgen nach Folgennummer. Ein Kind ohne
     Gegenstück wird umgehängt.
   - Benutzerdaten wandern mit den Regeln von `MergeItems` (B-9): Beim Fortschritt gewinnt
     «fertig», dann die grössere Position. Ereignisse, Tage, Merkliste, Favoriten, Sammlungen,
     Benachrichtigungen, Top-Picks und Downloads wandern ebenfalls, dazu `playback_sessions` und
     `activity_events`.
   - Zuletzt wird die Kopie endgültig gelöscht. Ein gelöschtes Kind ohne Gegenstück bleibt
     gelöscht.
   - Der wiederbelebte Titel bekommt einen `metadata.match`, der die zugezogenen Folgen beschreibt.

   Das Paket `metadata` ruft dafür einen Haken auf (`metadata.WithRelink`), den `cmd/slouchcat`
   mit `library` verdrahtet. `library` importiert `metadata` schon, deshalb geht es nicht
   umgekehrt.
2. **Fristen der Wiederverknüpfung.** Über **Anbieter-ID** (aus der Datei im Scanner und nach dem
   Match) gilt **keine Frist**. Über **Name und Jahr** (A1, neu im Scanner) gilt eine Frist von
   **30 Tagen**: Der Scanner findet einen gelöschten Titel auch über (`sort_title`, Jahr), wenn
   kein lebender Titel passt und `fits` gilt. Titel ohne Jahr bleiben an ihren Ordner gebunden. Ein
   Abgleich nur über den Namen (A2) entfällt.
3. **Pflegeaktion «Verwaiste Benutzerdaten umhängen» (D1).** Sie steht unter «Pflege». Zuerst
   zeigt sie die Paare (gelöschter Titel, lebende Kopie) mit den Zahlen je Tabelle. Erst auf
   Bestätigung führt sie die Paare mit derselben Zusammenführung wie in Punkt 1 zusammen. Sie kennt
   keine Frist und lässt sich wiederholen.
4. **«Pfad ändern» für einen Ordner (C1).** `updateLibraryRoot` nimmt einen neuen `path` an. Er
   wird geprüft wie beim Hinzufügen: absolut, lesbar, unter einer browsbaren Wurzel, ohne
   Überschneidung mit anderen Ordnern ausser sich selbst. Ein neuer Pfad ist nicht mit einem
   Inhaltswechsel kombinierbar. Der Server übernimmt den neuen Pfad unter der Scan-Sperre, behält
   die `media_files`, startet den Watcher neu und reiht einen Scan ein. Dieser findet die Dateien
   über `rel_path`, inode oder `partial_hash` wieder. Bestätigt wird mit einem **einfachen Dialog**
   mit altem und neuem Pfad, ohne Abtippen. Die Verlustschwelle von 20 % sichert zusätzlich ab.
5. **Der Bildrückfall der Statistik (`e4e323a`) bleibt.**

## Folgen

- Vertrag **1.9.0**, additiv: zwei Operationen der Pflegeaktion und `path` an
  `LibraryRootUpdate`. Keine Migration.
- Ohne Frist lädt der Scanner alle gelöschten Titel einer Bibliothek in seinen Index, auf CT 190
  31 641 Zeilen. Die Scanzeit wird vor und nach dem Bau gemessen (`docs/perf/wurzelwechsel.md`).
- Gelöschte Titel werden weiterhin nie endgültig gelöscht, ausser durch `MergeItems` oder die
  neue Zusammenführung.
- Nicht gebaut: eine Sperre beim Entfernen (C2), der Abgleich über den Namen ohne Jahr (A2) und
  ein Abgleich von Dateien über Wurzeln hinweg.

## Nachträge aus dem Bau (3. November 2026)

Die Reviews der Schritte W-1 bis W-5 und zwei weitere Antworten von Marco haben den Entscheid
genauer gefasst:

- **Nur frische Kopien, automatisch mit 7 Tagen Spielraum.** Die späte Wiederverknüpfung
  (Punkt 1) gilt nur für eine Kopie, die frühestens **7 Tage vor** dem Löschen des Zwillings
  entstanden ist (`relinkGrace`). Ohne diese Regel würde ein seit Jahren lebender Titel beim
  nächsten Match in ein altes gelöschtes Duplikat gezogen. Der Spielraum deckt die Reihenfolge
  «neuen Ordner hinzufügen, scannen, alten entfernen» ab (Marco, 3. November 2026). Die
  **Pflegeaktion kennt diese Regel nicht**, weil der Administrator dort beide Titel sieht.
- **Auch nach dem Löschen wird verknüpft.** Wird ein Film oder eine Serie soft-gelöscht (Ordner
  entfernen, Inhaltswechsel, Sweep, Bereinigung in der Pflege, ignorierte Dateien), reiht dieselbe
  Transaktion `library.relink` für frische Kopien mit gleicher Anbieter-ID ein. Erst damit greift
  der Spielraum von 7 Tagen auch für «neuen Ordner hinzufügen, scannen, alten entfernen».
- **Dauerhafter Job statt Speicher.** Der Haken nach dem Match prüft nur, ob es einen Zwilling
  gibt. Gibt es einen, endet der Match der Kopie nach dem Titel, und der Job `library.relink`
  (Schlüssel `relink:<id>`) wird eingereiht. Ist die Scan-Sperre belegt, reiht er sich mit
  `jobs.RetryLater` nach einer Minute neu ein, ohne einen Versuch zu verbrauchen (neu im Paket
  `jobs`: `Queue.Defer`). Ein Neustart verliert nichts. Scheitert der Job endgültig oder bricht ihn der Administrator ab, wird
  der Match der Kopie mit `noRelink` neu eingereiht, damit Staffeln und Bilder nicht fehlen. Nach
  einer Zusammenführung prüft der Server offene Wiedergaben nach (`library.WithVisibilityRecheck`).
- **Ältere Zwillinge werden mitgefaltet.** Gibt es mehrere gelöschte Zwillinge, überlebt der
  zuletzt gelöschte. Die älteren gehen in derselben Transaktion in ihm auf. Bei gleichem
  Löschzeitpunkt entscheidet die höhere ID.
- **Gepflegte Werte der Kopie bleiben:**
  - Die Freigabe wird **nie milder als die der lebenden Kopie**, ausser der Administrator hat sie
    an der Kopie selbst gesperrt. Dann gilt die gesperrte Freigabe der Kopie. Sonst gilt die
    strengere der beiden, auch gegen eine gesperrte Freigabe des Zwillings. Dessen Sperre bleibt
    und hält dann den strengeren Wert. Eine Kopie ohne Freigabe lässt die des Zwillings stehen.
  - Sperren, die dem Zwilling fehlen, gehen mit ihrem Wert über.
  - Bildwahl und eigene Bilder gehen über, wo der Zwilling keine hat.
  - Kapitel und erkannte Intro- und Abspann-Marken wandern mit der Datei.
  - Fehlt einer Folge die Erkennung, wird ihre Staffel neu geprüft.
- **Downloads:** Ein unbrauchbarer Download des Zwillings (fehlgeschlagen, verdrängt, Datei weg)
  weicht dem funktionierenden der Kopie. Das Gerät behält sein Paket.
- **Pflegeaktion:**
  - Alle Paare sind vorausgewählt. Abgewählte bleiben abgewählt (Marco, 3. November 2026).
  - Die Tabelle zeigt den gelöschten Titel und die Kopie.
  - Paare, die es nicht mehr gibt, zählen als übersprungen.
  - Läuft mitten im Lauf ein Scan, meldet die Antwort den Teilerfolg (`stoppedBy: scan_running`).
- **«Pfad ändern»:**
  - Ein neuer Pfad lässt sich nicht mit einem Inhaltswechsel verbinden (neuer Fehlercode
    `path_with_content`), wohl aber mit einer neuen Anbieterliste. Pfad und Anbieterliste werden
    in einer Transaktion unter der Scan-Sperre geschrieben.
  - Kennt der Ordner schon Dateien, muss mindestens eine davon unter dem neuen Pfad liegen. Sonst
    lehnt der Server ab (neuer Fehlercode `root_files_missing`). Ein Tippfehler im Pfad löscht so
    keine Titel.
  - Gleicher Pfad ändert nichts.
- **Scanner:** Der Index lädt gelöschte Titel mit einer schmalen Abfrage (11 Spalten, nur Filme,
  Serien, Hörbücher, Podcasts, Staffeln, Folgen). Die Suche über Name und Jahr gilt nur für
  Filme und Serien.

## Verworfen

- **B2, die neue ID bleibt:** einfacher, macht aber Titel-IDs ungültig, die schon auf Geräten, in
  Links und in Offline-Paketen stehen.
- **D2, eine einmalige Migration:** unsichtbar und nicht wiederholbar. Ein Fehler liesse sich nur
  aus der Sicherung zurückholen.
- **30 Tage auch für die Anbieter-ID:** Marco will, dass ein Titel mit derselben ID immer
  zurückfindet. Das Verwechslungsrisiko liegt dort nur beim Match selbst.
