# Idee 04: Benutzerdaten beim Wechsel eines Bibliotheksordners behalten

- Status: **Übernommen** (Frage 21, ADR 0023, Bau auf Marcos Wort vom 3. November 2026 auf dem Zweig
  `claude/wurzelwechsel`, Vertrag 1.9.0). Analyse vom 3. November 2026; F-1 bis F-6 am selben Tag von Marco
  beantwortet, Abschnitt 7: B1 + A1 + C1 + D1, Wiederverknüpfung über Anbieter-ID ohne Frist). Issue [#73](https://github.com/SlouchCat/slouchcat/issues/73)
- Frühestens: sofort möglich, weil es eine Fehlerbehebung ist. Plan 3.2 verspricht das Verhalten
  schon (Abschnitt 3). Gebaut wird erst auf Marcos ausdrückliches Wort.
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

**Gewählt (Abschnitt 7):** B1 + A1 + C1 + D1. Die Empfehlung unten ist die Fassung vor dem
Interview.


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

## 7. Fragen an Marco und seine Antworten

Marco hat alle Fragen am 3. November 2026 im Interview beantwortet. Seine Antworten folgen der
Empfehlung, mit zwei Abweichungen: C1 wird gleich mitgebaut, und die Wiederverknüpfung über
Anbieter-IDs gilt ohne Frist.

- **F-1 / F-2, späte Wiederverknüpfung:** **Ja, die alte ID bleibt (B1).** Bekommt ein neuer
  Titel durch `metadata.match` eine TMDB-, TVDB- oder IMDb-ID, die ein gelöschter Titel derselben
  Art und Bibliothek trägt, lebt der gelöschte Titel wieder auf. Dateien, Staffeln und Folgen der
  Kopie wandern zu ihm. Staffeln werden nach Staffelnummer zusammengeführt, Folgen nach
  Folgennummer, und die Kopie verschwindet. `playback_sessions` wandern mit.
- **F-3, Scanner über den Namen:** **Nur Name und Jahr (A1).** Ein Abgleich über den Namen allein
  (A2) entfällt.
- **F-4, Reparatur:** **Pflegeaktion (D1).** Sie steht unter «Pflege», zeigt zuerst die Zahlen
  und läuft erst auf Klick. Sie ist wiederholbar und kennt keine Frist. Sie verwendet dieselbe
  Zusammenführung wie B1, und die alte ID bleibt.
- **F-5, «Pfad ändern»:** **Gleich mitbauen (C1)**, abweichend von der Empfehlung «später». Die
  Bestätigung ist ein **einfacher Dialog** «Pfad ändern?» mit altem und neuem Pfad und den Knöpfen
  «Ändern» und «Abbrechen». Der alte Pfad muss nicht abgetippt werden. Als zweite Sicherung wirkt
  die bestehende Verlustschwelle von 20 % («Verlust akzeptieren»).
- **F-6, Zeitpunkt:** **Eigenes Release zuerst.** Es kommt ein eigener PR als nächster Bau nach
  1.8.0, als **1.9.0** (Vertrag additiv, wegen D1 und C1). Konzept 13 und 14 rücken eine
  Version nach.
- **Frist der Wiederverknüpfung:** **Ohne Frist für die Wiederverknüpfung über Anbieter-IDs**,
  abweichend von der Empfehlung «30 Tage lassen». Das gilt für B1 und für IDs, die die Datei selbst
  mitbringt (heute `relinkWindow`). Für **A1 bleiben 30 Tage**. Damit wird `relinkWindow` in zwei
  Fristen geteilt: Anbieter-ID ohne Ende, Name und Jahr 30 Tage. Das weicht von Plan 3.2 ab
  («Benutzerdaten bleiben 30 Tage») und braucht beim Bau einen Eintrag im Entscheidungsprotokoll.
- **Bildrückfall `e4e323a`:** **Bleibt.**

### 7.1 Folgen der Antworten für den Bau

- **Der Index wird grösser.** Ohne Frist lädt `loadItemIndex` alle gelöschten Titel einer
  Bibliothek mit Anbieter-ID. Auf CT 190 sind das heute 497 gelöschte Serien. Staffeln und Folgen
  darunter lädt er über die Tabellen `seasons` und `episodes` des Index mit, zusammen 31 641
  gelöschte Zeilen. Das ist vertretbar, muss aber beim Bau gemessen werden (Scan der Bibliothek «Serien» vor
  und nach dem Bau, nach `docs/perf/`). Bei Bedarf werden nur gelöschte Titel geladen, und ihre
  Kinder erst dann, wenn ein Titel wiederauflebt.
- **Frist 27. November 2026 entfällt.** Ohne Frist verknüpft B1 auch die Fälle vom 28. Oktober,
  sobald ein Titel neu zugeordnet wird. Bei den zwei Serien auf CT 190 ist der Match schon
  gelaufen, darum repariert sie trotzdem erst D1.
- **C1 und gelöschte Titel:** Zeigt «Pfad ändern» auf einen Ordner mit weniger Dateien, werden
  die fehlenden nach 24 h soft-gelöscht. Kehren sie später zurück, verknüpft sie B1 oder die Datei-ID
  wieder, jetzt ohne Frist.
- **Nummern:** Frage, ADR, Migration (C1 und D1 brauchen voraussichtlich keine) und Vertragsfassung
  werden beim Baubeginn vergeben (Regel «Vertrag = Tag»).

## 8. Grobe Aufwandsschätzung und Abhängigkeiten

| Teil | Aufwand | Migration | Vertrag | hätte CT 190 verhindert |
|---|---|---|---|---|
| A1 / A2 | ½ Tag | nein | nein | nein / ja |
| B2 / B1 | 1 / 2 Tage | nein | nein | ja (nach dem Match) |
| C1 | 1–2 Tage | nein | additiv | ja |
| C2 | ½–1 Tag | eventuell | additiv (Fehlercode) | nein |
| D1 | 1 Tag (auf B aufbauend) | nein | additiv | – (Reparatur) |

Abhängigkeiten: keine zu offenen Konzepten. B1 und D1 teilen sich die Zusammenführung über den
ganzen Baum und nehmen `playback_sessions` mit (Abschnitt 4 D).

**Gewählter Umfang (Abschnitt 7):** A1 + B1 + C1 + D1, zusammen etwa **4½ bis 5½ Tage**, als
eigener PR und Release 1.9.0. Die Frist vom 27. November 2026 entfällt mit der
Wiederverknüpfung ohne Frist (7.1).
