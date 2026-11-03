# Idee 04: Benutzerdaten beim Wechsel eines Bibliotheksordners behalten

- Status: **Entwurf** (Analyse und Vorschlag vom 3. November 2026, Fragen F-1 bis F-6 an Marco offen). Issue [#73](https://github.com/SlouchCat/slouchcat/issues/73)
- Frühestens: sofort möglich, weil es eine Fehlerbehebung ist. Plan 3.2 verspricht das Verhalten
  schon (Abschnitt 3). Gebaut wird erst auf Marcos Wort.
- Auslöser: Abnahme von Konzept 12 (K12-9, PR #72) am 3. November 2026. In der Statistik hatte
  eine Folge kein Bild. Ihre ganze Serie war seit dem 28. Oktober 2026 soft-gelöscht, und eine
  zweite Kopie lief unter neuen IDs. Für die Bilder gibt es seit `e4e323a` einen Rückfall auf die
  Live-Kopie mit gleicher Anbieter-ID. Verlauf und Fortschritt hängen weiter an den gelöschten IDs.
- Grundlage: Code auf `main` `9f2927f`, lesende Messung auf CT 190 (SQLite `mode=ro`,
  3. November 2026, etwa 10 Uhr). Pfade ohne Präfix liegen unter `server/`.

## 1. Problem und Ziel

Ein Administrator ersetzt den Ordner einer Bibliothek. Er fügt den neuen Ordner hinzu und entfernt
den alten. Danach hängen Fortschritt, Wiedergabeverlauf und Statistik eines Zuschauers an
soft-gelöschten Titeln. Derselbe Titel steht unter einer neuen ID wieder in der Bibliothek, aber
ohne «weiterschauen», ohne Gesehen-Haken und mit einer leeren Verlaufszeile.

**Ziel:** Kehrt ein Titel nach einem Ordnerwechsel zurück, behält er seine Benutzerdaten. Plan 3.2
sagt dasselbe: «Benutzerdaten bleiben 30 Tage und werden bei gleicher Provider-ID wieder
verknüpft.» Für Daten, die schon getrennt sind, gibt es einen Reparaturweg.

### 1.1 Was auf CT 190 geschah (gemessen)

| Zeit (UTC) | Ereignis | Beleg |
|---|---|---|
| 28.10. 15:45:30 | Ordner `/media/nas/TestSerien` zur Bibliothek «Serien» (id 2) hinzugefügt, `library_roots` id 3. Ein `scan.library` wird eingereiht. | `library_roots.created_at`, Job 102473 |
| 28.10. 15:45:50 | Der alte Ordner der Bibliothek wird entfernt (`RemoveRoot`), **bevor der Scan läuft**. 29 106 Folgen und 27 Extras werden soft-gelöscht, danach 2 010 leere Staffeln und 498 leere Serien, zusammen **31 641 Titel**. | `items.deleted_at` aller 31 641 Zeilen = 15:45:50 |
| 28.10. 15:47:56 | Der Scan liest `TestSerien`. Dort liegt zu diesem Zeitpunkt nur eine Datei von «Two and a Half Men». Er legt eine **neue** Serie an (`01M61K4N…`). | `items.created_at` |
| 31.10. 09:02 | Weitere Dateien in `TestSerien`: der Rest von «Two and a Half Men» und «Silo» (wieder eine neue Serie `01M68K5B…`). | `items.created_at` |

**Der Auftrag ging von einem Neuimport des ganzen Baums aus. Das trifft nicht zu.** `TestSerien` ist
eine kleine Testauswahl. Nur **2 der 498 Serien** (15 Staffeln, 291 Folgen) sind als Live-Kopie
zurück. Die anderen 496 Serien liegen in keinem registrierten Ordner mehr und sind zu Recht
gelöscht. Beide Live-Serien haben nach `metadata.match` dieselben Anbieter-IDs wie ihr gelöschter
Zwilling, zum Beispiel «Two and a Half Men», TMDB 2691, TVDB 72227, `sort_title`
«two and a half men», Jahr 2003, auf beiden Seiten.

Der alte Ordner war mit grosser Wahrscheinlichkeit `/media/nas/Serien`. Er liegt noch auf dem NAS,
hat 503 Einträge und enthält ebenfalls «Two and a Half Men» und «Silo». Den Pfad selbst nennt die
Datenbank nicht mehr, weil die Zeile in `library_roots` gelöscht ist, und das Container-Log reicht
nicht so weit zurück. Die Dateien in `TestSerien` sind **Kopien**, keine harten Links (Linkzahl 1).
Die Serienordner tragen **weder Jahr noch Anbieter-Marke**, und es liegt **keine NFO** daneben.

### 1.2 Betroffene Benutzerdaten auf CT 190

Gezählt sind alle Zeilen, deren `item_id` auf einen gelöschten Titel der Bibliothek 2 zeigt. Die
Spalte «mit Live-Kopie» zeigt, wie viele sich über Anbieter-ID, Staffel- und Folgennummer einem
Live-Titel zuordnen lassen.

| Tabelle | Zeilen an gelöschten IDs | mit Live-Kopie | Bestand auf dem Server |
|---|---:|---:|---:|
| `progress` | 3 (Profil 1) | 3 | 16 |
| `progress_events` | 140 (3 Folgen, 26. und 27.10.) | 140 | 244 |
| `progress_days` | 0 | – | 0 |
| `playback_sessions` | 5 | 5 | – |
| `watchlist`, `favorites`, `downloads`, `notifications`, `collection_items`, `top_picks`, `activity_events` | 0 | – | – |

Die drei Fortschritte betreffen «Two and a Half Men»: S01E18 bei 29 s, S06E05 bei 31 s und S06E18
fertig geschaut. Ihre Live-Kopien haben noch keinen eigenen Fortschritt, beim Umhängen gibt es also
keinen Konflikt. Gemessen am Bestand ist der Anteil gross: **3 von 16 Fortschritten und 140 von 244
Wiedergabe-Ereignissen** des Servers hängen an gelöschten IDs.

Die Zahl «29 106 Folgen betroffen» aus dem Auftrag zählt die gelöschten Folgen. Benutzerdaten haben
davon nur 3.

## 2. Ursache im Code

Es wirken drei Lücken zusammen. Alle liegen im Paket `library`.

**(a) Gelöschte Titel findet der Scanner nur über Anbieter-IDs aus der Datei.**
`itemIndex.find` (`internal/library/index.go`) sucht einen Titel in dieser Reihenfolge: TVDB, TMDB,
IMDb, Ordner der Serie, dann (`sort_title`, Jahr). Soft-gelöschte Titel aus `relinkWindow`
(30 Tage, `internal/library/scanner.go`) trägt `addDeleted` **nur in die drei ID-Tabellen** ein,
nicht in `byTitle`. Der Ordnerschlüssel enthält die `root_id` und schliesst gelöschte Besitzer aus.
Die Datei in `TestSerien` bringt keine Anbieter-ID mit, weder eine NFO noch eine Marke wie
`[tvdb-72227]` im Ordnernamen. Darum findet `find` nichts, und `titleItem`
(`internal/library/write.go`) legt eine neue Serie an. `metadata.match` gibt ihr danach dieselben IDs
wie dem gelöschten Zwilling, aber kein Schritt verknüpft die beiden nachträglich. Das Versprechen aus
Plan 3.2 gilt damit nur für Dateien, die ihre ID selbst mitbringen.

Hätte `find` die gelöschte Serie gefunden, wäre alles erhalten geblieben. `restore` holt sie zurück,
und die Tabellen `seasons` und `episodes` des Index enthalten auch gelöschte Staffeln und Folgen.
Der alte Baum lebt dann mit seinen IDs und allen Benutzerdaten wieder auf.

**(b) Ein Titel ohne Jahr ist an seinen Ordner in *dieser* Wurzel gebunden.** Trägt der Ordnername
kein Jahr, nimmt `find` einen Treffer über den Namen nur, wenn der Titel schon eine Datei im selben
Ordner hat. Der Ordnerschlüssel (`titleFolderKey`, `seriesFolderKey`) enthält die `root_id`. Der
alte Titel hat dagegen nach `metadata.match` das Jahr 2003 und steht unter dem Schlüssel
«two and a half men|2003». Ein Ordner «Two and a Half Men» in einer anderen Wurzel findet ihn darum
nie, auch nicht, solange er noch lebt.

**(c) Dateien erkennt der Scanner nur innerhalb derselben Wurzel wieder, und ein Ordner bekommt
keinen neuen Pfad.** `planRoot` (`internal/library/scanner.go`) vergleicht der Reihe nach
`rel_path`, dann (dev, inode, size), dann `partial_hash`, und zwar nur gegen die bekannten Dateien
dieser Wurzel. `UpdateRoot` ändert nur Inhalt und Anbieterliste (`LibraryRootUpdate` in
`api/openapi.yaml`). Wer einen Ordner ersetzt, muss also hinzufügen und entfernen.
`RemoveRoot` löscht dabei die `media_files` des alten Ordners mit (Kaskade). Danach ist auch der Weg
über `rel_path` und `partial_hash` verloren.

Die Reihenfolge war auf CT 190 unglücklich: Der alte Ordner wurde entfernt, bevor der neue gelesen
war. Wegen (b) hätte aber auch die umgekehrte Reihenfolge den Fall nicht verhindert. Der Scan hätte
neben der lebenden alten Serie eine zweite angelegt, und das Entfernen hätte die alte danach
gelöscht. Mit Jahr im Ordnernamen wäre die neue Datei dagegen als zweite Fassung an die bestehende
Folge gegangen, und `SoftDeleteItemsOfRootWithoutOtherFiles` hätte die Serie verschont.

**Kein Datenverlust, nur verwaiste Daten.** Soft-gelöschte Titel werden nie endgültig gelöscht. Das
tut nur `AdminDeleteItem` beim Zusammenführen. Die Benutzerdaten bleiben also erhalten und lassen sich
reparieren. Der Scanner verknüpft über `relinkWindow` noch bis zum **27. November 2026** neu, und auch
das nur über die IDs der Datei.

## 3. Abgrenzung zur Spec v1

- Berührt Plan 3.2 (Soft-Delete, 24 h, Wiederverknüpfung nach 30 Tagen) und Idee 02 / Konzept 07
  (Ordner je Bibliothek, `RemoveRoot`, E12).
- Kein Non-Goal betroffen. Es entsteht keine neue Funktion für Zuschauer. Der Administrator bekommt
  höchstens eine Warnung oder eine Reparaturaktion (Optionen C und D).

## 4. Optionen

### A – Scanner findet gelöschte Titel auch über den Namen

`addDeleted` trägt gelöschte Titel aus `relinkWindow` zusätzlich in `byTitle` ein, markiert als
gelöscht. `find` bevorzugt einen lebenden Treffer. Erst wenn es keinen gibt, nimmt es den neuesten
gelöschten, und auch nur, wenn `fits` gilt, die Anbieter-IDs sich also nicht widersprechen.

- **A1 – mit Jahr:** nur (`sort_title`, Jahr). Das ist sicher, hilft auf CT 190 aber **nicht**,
  weil die Ordnernamen kein Jahr tragen (2 b).
- **A2 – ohne Jahr:** Trägt die Datei kein Jahr, genügt ein gleicher `sort_title`. Bedingung: Es
  gibt genau einen gelöschten Kandidaten und keinen lebenden Titel dieses Namens. Das hätte den Fall
  auf CT 190 gefangen.
- **Wirkung:** Die alte Serie kehrt samt Staffeln, Folgen und Fortschritt zurück, sofort beim Scan
  und ohne Wartezeit auf den Match-Job.
- **Risiko:** Eine *andere* Serie gleichen Namens erbt innerhalb von 30 Tagen die Daten der
  gelöschten, etwa ein Remake. Bei A2 ist das wahrscheinlicher als bei A1. `metadata.match` deckt
  es nicht auf, denn `titleItem` reiht für einen wiederhergestellten Titel keinen Match-Job ein.
- **Grenzen:** A repariert keine bestehenden Daten, denn auf CT 190 gewinnt die schon vorhandene
  Live-Kopie.
- Aufwand: klein, etwa ein halber Tag mit Tests im Paket `library`.

### B – Späte Wiederverknüpfung nach `metadata.match`

Bekommt ein *neu angelegter* Titel durch `metadata.match` eine TVDB-, TMDB- oder IMDb-ID, die ein
gelöschter Titel derselben Art und Bibliothek aus `relinkWindow` schon trägt, werden beide
zusammengeführt. Das Paket `metadata` darf auf `items` zugreifen (Regel 2). Der Vergleich stützt
sich auf die Anbieter-ID und nicht auf den Namen, darum ist er so sicher wie die Zuordnung selbst.
Auf CT 190 hätte er beide Serien gefangen. Zwei Richtungen sind möglich:

- **B1 – die alte ID bleibt:** Der gelöschte Titel wird wiederhergestellt. Dateien, Staffeln und
  Folgen des neuen Titels wandern zu ihm, Staffeln und Folgen werden nach Nummer zusammengeführt,
  und der neue Titel verschwindet. Verlauf, Statistik, Offline-Pakete und Verweise in alten
  Benachrichtigungen bleiben gültig. Dafür braucht es ein Zusammenführen über den ganzen Baum.
  `MergeItems` hängt heute nur die Kinder um und führt Staffeln nicht nach Nummer zusammen.
- **B2 – die neue ID bleibt:** Die Benutzerdaten wandern vom alten zum neuen Titel, mit den
  vorhandenen Schritten aus `mergeSource` (`internal/admin/maintenance.go`: `AdminMergeProgress`,
  `AdminMoveProgressEvents`, `AdminMergeProgressDays`, Merkliste, Favoriten, Downloads …), je Folge
  über Staffel- und Folgennummer. Das ist einfacher. Allerdings sind Titel-IDs, die schon zu Geräten
  oder in Links gelangt sind, danach ungültig.

Unschön ist das Zeitfenster: Bis der Match-Job läuft, sieht der Zuschauer den Titel ohne
Fortschritt. Bei einem Stau der Warteschlange können das Stunden sein. Folgen, die nach dem
Serien-Match hinzukommen, laufen über `episodeMatchJob` und brauchen keinen eigenen Abgleich, weil
sie in die schon verknüpfte Serie fallen. Aufwand: B2 etwa 1 Tag, B1 etwa 2 Tage.

### C – Den Ordnerwechsel im Ablauf absichern

- **C1 – «Pfad ändern» als eigene Operation:** `updateLibraryRoot` nimmt zusätzlich `path` an. Der
  Server prüft den neuen Pfad wie beim Hinzufügen (lesbar, unter einer browsbaren Wurzel, keine
  Überschneidung) und schreibt ihn in `library_roots.path`. `media_files` bleiben erhalten, der
  Watcher startet neu, und ein Scan folgt. Der Scan erkennt die Dateien über `rel_path`, inode oder
  `partial_hash` wieder. Auf CT 190 hätten die Dateien von «Two and a Half Men» und «Silo» unter
  gleichem `rel_path` ihre IDs behalten. Der Rest wäre «fehlend» geworden, und die bestehende
  Verlustschwelle von 20 % (`lossThresholdPercent`, «Verlust akzeptieren») hätte erst nachgefragt.
  Das ist der sauberste Weg für «gleicher Baum, neuer Ort», aber eine Vertragsänderung (additiv,
  `x-visibility: admin`, Vertragstest) mit Formular und Tastaturtest.
- **C2 – Schutz beim Entfernen:** `RemoveRoot` antwortet mit 409, solange ein anderer Ordner
  derselben Bibliothek noch keinen abgeschlossenen Scan hat. Das ist klein, hätte den Fall auf
  CT 190 wegen 2 (b) aber **nicht** verhindert. Es hilft nur zusammen mit A oder B. **Nicht
  empfohlen.**

### D – Reparatur der bestehenden Daten

Für Daten, die schon getrennt sind. Die Zuordnung gelöscht → lebend läuft über die Anbieter-ID der
Serie oder des Films, dann über Staffel- und Folgennummer. So hat die Messung in 1.2 gezählt.

- **D1 – Pflegeaktion** unter «Pflege», wo das Zusammenführen schon wohnt (B-9). Sie heisst etwa
  «Verwaiste Benutzerdaten umhängen» und zeigt vorher die Zahlen. Ein Lauf verwendet dieselbe
  Zusammenführung wie B, in der Richtung, die F-2 festlegt. Sie fängt auch künftige Fälle, die A, B
  und C nicht abdecken.
- **D2 – einmalige Migration** nach Vorlage (`TEMPLATE.sql.txt`). Sie läuft auf jedem Server
  einmal mit derselben Zuordnung in SQL und ist auf frischen Installationen ohne Wirkung. Sie ist
  unsichtbar und nicht wiederholbar, und eine falsche Zuordnung lässt sich nur aus der Sicherung
  zurückholen.
- **D3 – nichts tun:** CT 190 ist ein Testserver mit 3 Fortschritten und 140 Ereignissen eines
  Profils. Der Rückfall für Statistik-Bilder (`e4e323a`) bleibt.

`playback_sessions` hängt `mergeSource` heute nicht um. Die Zeilen gehen beim Löschen der Quelle per
`ON DELETE CASCADE` mit, und die Statistik liest sie (`admin_stats.sql`). Eine Zusammenführung nach
B oder D muss sie mitnehmen.

### Empfehlung

**B1 + D1, dazu C1, wenn ein Ordnerwechsel öfter vorkommt.**

- **B1** schliesst die Lücke dort, wo sie sicher zu schliessen ist: über die Anbieter-ID nach dem
  Match. Das ist unabhängig von Jahr, Ordnername und Reihenfolge, und die alte ID bleibt.
- **D1** baut auf derselben Zusammenführung auf und repariert CT 190 sowie künftige Ausreisser
  sichtbar und wiederholbar.
- **C1** ist der richtige Weg für einen Umzug desselben Baums, braucht aber Vertrag und Oberfläche.
  Er ist nicht dringend, solange B die Daten auffängt.
- **A1** kann als kleine Ergänzung mitkommen. Er schliesst das Zeitfenster bis zum Match, wenn der
  Ordner ein Jahr trägt. **A2** wird wegen des Verwechslungsrisikos nicht empfohlen.

## 5. Datenmodell und API-Skizze

- A und B: keine Migration, kein Vertrag. Betroffen sind `internal/library/index.go` bzw.
  `internal/metadata/match.go` sowie eine gemeinsame Zusammenführung über den ganzen Baum, etwa
  neben `MergeItems` in `internal/admin` oder als eigenes Hilfspaket. Wo sie wohnt, ist beim Bau zu
  klären. `metadata` darf `admin` nicht einfach importieren, wenn daraus ein Zyklus entsteht.
- D1: neue Operation, etwa `POST /admin/maintenance/relink-orphans` mit `dryRun`, `x-visibility:
  admin` und Eintrag im Vertragstest. Antwort: Zahl der Paare und Zeilen je Tabelle.
- C1: `path` in `LibraryRootUpdate`, mit `confirmPath` wie bei der Inhaltsänderung.

## 6. Berührungspunkte mit den harten Regeln

- **Regel 2:** Alles bleibt in `library`, `admin` und `metadata`, die `items` lesen und schreiben
  dürfen. Es entsteht keine neue Abfrage in `queries/scoped/`.
- **Regel 5:** Nur D2 bräuchte eine Migration nach Vorlage, nur vorwärts.
- **Regel 7:** Neue Operationen (C1, D1) tragen `x-visibility: admin` und einen Vertragstest.
- **Regel 8 und 9:** Ein neuer oder geänderter Dialog (C1, D1) braucht Texte aus den Locales und
  einen Tastaturtest.
- **Regel 12:** Nicht berührt. Die Zuordnung entscheidet allein der Server.

## 7. Offene Fragen an Marco

- **F-1:** Soll ein neu zugeordneter Titel automatisch mit einem gelöschten Titel gleicher
  Anbieter-ID zusammengeführt werden (Option B)? Empfehlung: ja.
- **F-2:** Welche ID soll überleben, wenn zwei Kopien zusammenkommen (B und D)? Empfehlung: die
  **alte (B1)**, damit Verlauf, Statistik und Offline-Pakete gültig bleiben, auch wenn der Bau
  grösser wird.
- **F-3:** Soll der Scanner zusätzlich über Name und Jahr wiederverknüpfen (A1)? Oder, riskanter,
  auch über den Namen allein, wenn der Ordner kein Jahr trägt (A2)? Empfehlung: A1 ja, A2 nein.
- **F-4:** Wie sollen die bestehenden Daten repariert werden? Zur Wahl stehen eine Pflegeaktion
  (D1, Empfehlung), eine einmalige Migration (D2) oder keine Reparatur, weil CT 190 ein Testserver
  ist (D3).
- **F-5:** Braucht es «Pfad ändern» als eigene Operation (C1)? Oder kommt ein Umzug desselben Baums
  so selten vor, dass B genügt?
- **F-6:** Soll das gebaut werden, und wann? B1 + D1 wäre ein eigener PR nach Konzept 12 (1.8.0).
  Ohne neue Operation ginge es als Patch, mit D1 (additiver Vertrag) als nächste Minor-Version.
  Daneben: Soll der Bildrückfall aus `e4e323a` danach bleiben? Er schadet nicht und fängt Fälle
  ohne Reparatur.

## 8. Grobe Aufwandsschätzung und Abhängigkeiten

| Teil | Aufwand | Migration | Vertrag | hätte CT 190 verhindert |
|---|---|---|---|---|
| A1 / A2 | ½ Tag | nein | nein | nein / ja |
| B2 / B1 | 1 / 2 Tage | nein | nein | ja (nach dem Match) |
| C1 | 1–2 Tage | nein | additiv | ja |
| C2 | ½–1 Tag | eventuell | additiv (Fehlercode) | nein |
| D1 | 1 Tag (auf B aufbauend) | nein | additiv | – (Reparatur) |

Abhängigkeiten: keine zu offenen Konzepten. B und D1 teilen sich die Zusammenführung über den ganzen
Baum und ergänzen `playback_sessions` (Abschnitt 4 D). Die automatische Wiederverknüpfung über
`relinkWindow` endet für CT 190 am **27. November 2026**. Wird B danach gebaut, verknüpft es die
Fälle vom 28. Oktober nicht mehr. Dann hilft nur D.
