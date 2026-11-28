# Konzept 28 · Anforderungsmatrix

Stand: 29. November 2026 · Fassung 2 · Status: Entwurf nach Gesamtreview, nicht freigegeben

> **Kurz gesagt.** Diese Datei ist die Rückverfolgung von Marcos 21 Grundsatzentscheiden (LTV-001 bis LTV-021) und der
> Querschnittsabschnitte des Auftrags über die einzelnen Anforderungen bis zu Architektur, Datenmodell, API, Sicherheit,
> Oberfläche, Umsetzungsplan, Mockups und Abnahmekriterien. Sie ist **maschinell erzeugt** aus den Konzeptdokumenten
> (Fassung 2 von `03` und `17`, aktueller Stand der übrigen); was sich dort nicht finden liess, steht als «nicht zugeordnet»
> in Abschnitt 7 und nicht als geratene Zelle in den Matrizen. **Jede Anforderung hat in der Konzeptphase den Status
> «spezifiziert»; keine ist «implementiert».**

Dieses Papier ändert kein anderes Dokument. Unstimmigkeiten zwischen den Dokumenten sind nur gemeldet (Abschnitt 7).

---

## 1. Lesehilfe

**Kennungen.** `LTV-nnn` Grundsatzentscheid des Auftrags (Abschnitt 6); `LTV-nnn.mm` Anforderung, `LTV-nnn.X-mm`
Ausschluss darunter; `LTV-Q-nn.mm` Querschnittsanforderung (nn = Abschnitt des Auftrags); `LTV-X-nn` Zeile der
Ausschlussliste (Auftrag Abschnitt 16). Alle aus `03-fachkonzept.md`. `AK-L-nnn` Abnahmekriterium aus
`17-test-und-abnahme.md`. `T-n` Architekturentscheid (`04`), `SM-nn` Sicherheitsmassnahme (`12`), `LS-nn` Seite der
Zuschauer (`14`), `LA-nn` Seite der Verwaltung (`15`), `LD-nn` Dialog (`14`), `E-28-nn` offene Entscheidung (`20`; Frage an Marco;
hier «E-28-nn» aus `20-offene-entscheidungen.md` Fassung 2).

**Status.** Alle Zeilen: **spezifiziert** (Konzept liegt vor, nichts gebaut). **Implementiert** gibt es nicht. Eine
Anforderung gilt erst als **erfüllt**, wenn alle Teilanforderungen umgesetzt und durch Test oder Messung belegt sind
(Auftrag 15.2, `17` Grundsatz 1). Ein Entscheid mit Sammelanforderungen (LTV-001) ist erst erfüllt, wenn alle
Anforderungen seiner Detailentscheide erfüllt sind.

**Abnahme** (Spalte «Abn.»): **T** automatischer Test, **M** Messung, **S** Sichtprüfung am Dokument, Mockup oder Bau
(`03` Lesart). **Stufe**: V, 0, 1a, 1b, 2, 3, H, 4, 5, 6, L, W, E, Apps, K (Konzeptphase) nach `18` 4.13; die Stufenangabe
steht hier so, wie `03` sie schreibt («1a; E»: beginnt in 1a, ganz erfüllt in E).

**Pflichtfälle.** `17` Abschnitt 4: `LTV-Q-15.nn` ist `AK-L-(nn−1)` (`LTV-Q-15.02` = `AK-L-001`). Diese Zuordnung ist hier eingesetzt, obwohl
die Spalte «Bezug» der Pflichtfälle sie nicht immer ausschreibt.

**Woher die Zellen kommen (und was abgeleitet ist).**

| Angabe | Quelle | Art |
|---|---|---|
| Anforderung, Kurztext, Stufe, Dokument, Abnahme | `03` Abschnitte 3, 4, 7.1 (Tabellenzeilen) | gelesen |
| Zugeordnete `AK-L` | `17`: Spalte «Bezug» jeder `AK-L`-Zeile; Bereiche («.01–.04», «bis .15») sind aufgelöst | gelesen, Bereiche aufgelöst |
| `T-n` je Entscheid | `04` Tabelle der Entscheide: `LTV-…` im Text der Zeile | gelesen (Treffer im Text, kein Pflichtfeld) |
| Tabellen (`07`) je Entscheid | Absatz der Tabelle nennt den Entscheid (`LTV-…`); mit «\*» zusätzlich über Seiten (`07` 12.2, Seite → Tabelle) | gelesen bzw. über Seiten abgeleitet |
| Operationen (`08`) je Entscheid | Operation nennt eine Seite (Spalte «Seite»), die Seite nennt den Entscheid (`14` 2, `15` 2, Spalte «LTV»); dazu direkte `LTV-…` in der Zeile | **abgeleitet über Seiten**; zählt eine Operation bei jedem Entscheid der Seite mit, Summen über Entscheide sind daher keine Gesamtzahl |
| Seiten und Mockup | `14` 2 und `15` 2 (Spalte «LTV»); «Mockup» heisst: die Seite kommt in `19-mockups.md` vor | gelesen |
| Dialoge `LD-nn` | `14` 2 nennt für Dialoge keinen Entscheid; sie erscheinen nur über Operationen | nicht zugeordnet zu Entscheiden |
| Offene Fragen | Fragen `E-28-nn` aus `20` (Absatz der Frage nennt den Entscheid `LTV-…`, meist in «Bezug») | gelesen (Treffer im Text der Frage) |
| Vertiefende Dokumente | Spalte «Dok» von `03` (Zahl der Anforderungen je Dokument); Abschnitte nennt `03` nicht | gelesen |

---

## 2. Matrix A · je Grundsatzentscheid

Spalten: **Entscheid** nach Register des Auftrags und `02`; **Anf./Ausschl.** Zahl der Anforderungen und der
Ausschlüsse (`…X…`) in `03`; **Dok** vertiefende Dokumente (Zahl der Anforderungen); **T-n** Entscheide aus `04`;
**Tabellen** (`07`); **Operationen** (`08`, abgeleitet, Gruppe: Zahl); **Seiten** (Mockup: ✓ in `19`, ✗ ohne);
**AK-L** Zahl und Bereich; **Stufe** nach `03` (Zahl der Anforderungen); **Fragen**; **Status**.

| LTV | Thema | Entscheid | Anf. / Ausschl. | Dok | T-n | Tabellen (07) | Operationen (08) | Seiten (Mockup) | AK-L (Zahl; Bereich) | Stufe(n) | Fragen | Status |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| LTV-001 | Funktionsumfang | C – Vollwertige Live-TV-Plattform gemäss Detailentscheidungen: Slouchcat bekommt einen eigenständigen Live-TV-Bereich mit dem Umfang der Detailentscheide | 16 / 0 | 02 (1), 04 (1), 05 (4), 09 (2), 10 (1), 11 (2), 14 (3), 15 (2) | – | `live_favorites`\* | 4: Sender und Favoriten 4 | LS-03✓ | 3; 082, 188, 312 | K (1), 1a (8), 1b (1), 2 (3), 3 (1), 4 (1), 5 (1), 6 (1) | – | spezifiziert |
| LTV-002 | Empfangsstrategie | C – Hybrid: Jeder Sender wird nach einer von vier Strategien empfangen: permanent, automatisch, bei Bedarf oder … | 13 / 0 | 05 (9), 06 (1), 11 (1), 15 (2) | T-17, T-18, T-29, T-16\*, T-19\* | `live_channel_streams`\*, `live_channels`\*, `live_reception_schedules`\*, `live_reception_windows`\* | 17: Sender und Favoriten 1, Verwaltung: Sender, Gruppen, Duplikate 9, Verwaltung: Empfang, Einstellungen, EPG, Speicher 7 | LA-06✓, LA-09✓ | 24; 004–008, 013, 061–063, 065, 083, 089, 147, 149, 209, 280, 301, 305–306, 313–316, 334 | K (1), 1a (3), 3 (7), 5 (2) | E-28-02, E-28-29, E-28-30, E-28-40, E-28-48 | spezifiziert |
| LTV-003 | Empfangsquellen | B – Erweitertes IPTV: Slouchcat nimmt M3U/M3U8 als Datei und URL, einzelne Stream-URLs und Xtream-Codes-Live-APIs an und empfängt … | 25 / 11 | 05 (12), 07 (1), 09 (2), 12 (3), 15 (8), 16 (1) | T-2\*, T-5\* | `live_channel_streams`, `live_source_headers`, `live_sources`\* | 7: Verwaltung: Quellen und Importe 6, Verwaltung: Empfang, Einstellungen, EPG, Speicher 1 | LA-03✓ | 21; 024, 034–045, 051, 102, 175–177, 203, 273, 355 | K (2), 1a (16), 3 (1), H (3), L (2), W (1) | E-28-05, E-28-06, E-28-61, E-28-34, E-28-54, E-28-55 | spezifiziert |
| LTV-004 | Transcoding | B – Individuelles Transcoding: Ein Sender wird möglichst nur einmal empfangen und intern an alle Zuschauer und Aufnahmen … | 38 / 1 | 05 (33), 13 (1), 15 (1), 17 (4) | T-2, T-8, T-12, T-6\*, T-7\*, T-13\*, T-14\*, T-15\*, T-16\*, T-17\*, T-19\*, T-20\*, T-26\* | – | 0 | – | 31; 001–004, 013, 024, 026, 030, 033, 040, 064, 067–076, 181, 300–303, 308, 320, 330, 332, 353 | 1a (36), 3 (1), 5 (1), E (1) | E-28-04, E-28-14, E-28-56 | spezifiziert |
| LTV-005 | EPG | B – Erweitert: Slouchcat liest Programmdaten aus XMLTV (Datei, URL, komprimiert, mehrere Quellen), ordnet sie Sendern zu … | 28 / 3 | 09 (21), 11 (2), 14 (5), 15 (1) | – | – | 29: Sender und Favoriten 4, Programm 5, Erinnerungen (Stufe 2) 3, Verwaltung: Quellen und Importe 1, Verwaltung: Empfang, Einstellungen, EPG, Speicher 16 | LA-10✓, LA-11✓, LS-04✓, LS-05✓, LS-06✓, LS-13✓, LS-15✓ | 23; 023, 104–116, 119, 138, 178, 183, 189–190, 198, 200, 304 | 2 (26), 5 (2) | E-28-18, E-28-22, E-28-23, E-28-24, E-28-25, E-28-26 | spezifiziert |
| LTV-006 | Senderverwaltung | B – Erweitert: Der Administrator pflegt Sender in einer leistungsfähigen Verwaltung; jeder Sender hat eine stabile Identität … | 20 / 1 | 07 (3), 09 (2), 14 (1), 15 (14) | T-2, T-29\* | `live_channel_categories`, `live_channels`, `live_group_channels`, `live_groups`, `live_channel_streams`\*, `live_duplicate_dismissals`\* | 26: Sender und Favoriten 5, Verwaltung: Sender, Gruppen, Duplikate 19, Verwaltung: Empfang, Einstellungen, EPG, Speicher 2 | LA-05✓, LA-06✓, LA-07✓, LA-08✓, LS-02✓ | 16; 040, 048, 052–059, 184, 205–208, 313 | 1a (11), 1b (9) | E-28-17, E-28-18 | spezifiziert |
| LTV-007 | Timeshift | A – Persönlicher Puffer: Jeder Zuschauer hat ab dem Einschalten seinen eigenen Zeitversatz-Puffer zum Pausieren und Spulen, der … | 14 / 1 | 05 (2), 10 (12), 15 (1) | T-7, T-10 | – | 15: Sender und Favoriten 3, Live-Sitzung und Wiedergabe 9, Aufnahmen (Stufe 5) 2, Verwaltung: Empfang, Einstellungen, EPG, Speicher 1 | LA-15✓, LS-07✓ | 13; 009–011, 087, 120–124, 154, 171–172, 215 | 3 (2), 4 (12) | E-28-35, E-28-36, E-28-38 | spezifiziert |
| LTV-008 | DVR | B – Erweiterte Aufnahmefunktion: Slouchcat nimmt Sendungen sofort, geplant, manuell, als Serie und wiederkehrend auf, auch ohne angemeldeten … | 26 / 5 | 05 (2), 06 (1), 09 (2), 11 (18), 14 (4) | T-16 | `live_recording_gaps`, `live_recording_rules`, `live_recordings`\* | 22: Programm 2, Live-Sitzung und Wiedergabe 8, Aufnahmen (Stufe 5) 8, Regeln (Serie, wiederkehrend; Stufe 6) 3, Verwaltung: Empfang, Einstellungen, EPG, Speicher 1 | LA-14✓, LS-06✓, LS-08✓, LS-09✓, LS-10✓, LS-11✓, LS-12✓ | 29; 012–013, 017, 027, 130–143, 145, 148, 178, 183, 193–195, 214, 303, 330, 356 | 5 (23), 6 (3) | E-28-62, E-28-37, E-28-39, E-28-46, E-28-49, E-28-52, E-28-53 | spezifiziert |
| LTV-009 | Aufnahmeeigentum | A – Privat pro Profil: Aufnahmen, Planungen, Serienregeln, Fortschritt und Löschen gehören allein dem Profil; das Kontingent gilt je … | 12 / 0 | 07 (5), 11 (3), 12 (3), 15 (1) | – | – | 8: Programm 1, Aufnahmen (Stufe 5) 7 | LS-08✓, LS-09✓, LS-14✓ | 15; 016–017, 133, 140–141, 148, 153, 163–165, 173, 199, 212, 327–328 | K (1), 5 (10), 6 (1) | E-28-07, E-28-62, E-28-22, E-28-43, E-28-45, E-28-63, E-28-50 | spezifiziert |
| LTV-010 | Fast Zapping | B – Erweitert: Senderwechsel sollen sehr schnell sein: bestehende Ströme sofort nutzen, kleine Startpuffer, Vorladen benachbarter, häufiger … | 43 / 1 | 05 (24), 13 (15), 16 (5) | T-7\*, T-17\*, T-18\* | `live_favorites`\* | 17: Sender und Favoriten 6, Live-Sitzung und Wiedergabe 9, Aufnahmen (Stufe 5) 2 | LS-03✓, LS-07✓ | 14; 008, 082–090, 181, 302, 316–317 | K (29), 1a (3), 3 (13) | E-28-02, E-28-12, E-28-13, E-28-20, E-28-29, E-28-57 | spezifiziert |
| LTV-011 | Streamüberwachung | B – Erweitert: Slouchcat prüft Ströme, verbindet neu, erkennt Ausfälle, Standbild und fehlende Spuren, wechselt auf Ersatzquellen … | 21 / 1 | 05 (16), 12 (1), 14 (1), 15 (3) | T-17\* | `live_channels`, `live_stream_events`, `live_channel_streams`\* | 14: Sender und Favoriten 1, Verwaltung: Übersicht, Ströme, Diagnose 2, Verwaltung: Sender, Gruppen, Duplikate 8, Verwaltung: Empfang, Einstellungen, EPG, Speicher 3 | LA-06✓, LA-13✓ | 17; 024, 092–103, 213, 302, 331, 335 | 0 (1), 1a (6), 3 (15), E (1) | E-28-31, E-28-32 | spezifiziert |
| LTV-012 | Benutzerrechte | A – Einfach: Live-TV, Aufnahmen und Timeshift werden je Konto und je Profil erlaubt oder verboten, ein … | 13 / 1 | 05 (2), 07 (2), 12 (3), 13 (2), 14 (3), 15 (1) | T-10\*, T-13\*, T-19\*, T-20\*, T-21\*, T-26\* | – | 0 | – | 18; 018, 028–029, 033, 071, 076–077, 125, 160, 166–167, 169, 184, 218, 223, 267, 318, 326 | 1a (11), 4 (1), 5 (1) | E-28-59, E-28-09, E-28-66, E-28-41, E-28-42 | spezifiziert |
| LTV-013 | Catch-up TV | Ausgeschlossen: Slouchcat bietet kein Catch-up TV; permanenter Empfang ist keine Aufzeichnung | 2 / 6 | 02 (1), 05 (1), 10 (1), 11 (1) | T-7 | – | 0 | – | 3; 005, 170–171 | K (1), 3 (1) | – | spezifiziert |
| LTV-014 | Benutzeroberfläche | B – Moderne Streaming-Oberfläche: Live-TV bekommt einen eigenen, ruhigen Bereich im bestehenden Design mit Startseite, Senderübersicht, Programmzeitschrift, Sendungsdetails, … | 82 / 0 | 14 (74), 16 (7), 19 (1) | T-11\* | `live_favorites`\*, `live_recent_channels`\*, `live_recordings`\* | 36: Sender und Favoriten 11, Programm 5, Live-Sitzung und Wiedergabe 9, Aufnahmen (Stufe 5) 7, Regeln (Serie, wiederkehrend; Stufe 6) 3, Erinnerungen (Stufe 2) 1 | LS-01✓, LS-02✓, LS-03✓, LS-04✓, LS-06✓, LS-07✓, LS-08✓, LS-11✓ | 28; 110, 130–131, 133, 135, 137, 139, 141–142, 186–197, 200, 217, 294–297, 353 | K (2), 1a (22), 2 (27), 4 (5), 5 (20), 6 (4), E (1), Apps (4) | E-28-01, E-28-59, E-28-60, E-28-12, E-28-56, E-28-16, E-28-27, E-28-57 | spezifiziert |
| LTV-015 | Jugendschutz | Freigabe durch Hauptnutzer, keine Altersprüfung: Für Live-TV gibt es keine automatische Altersprüfung; das Hauptprofil schaltet Live-TV für ein Profil … | 14 / 0 | 09 (3), 11 (1), 12 (5), 14 (2), 17 (1), 20 (1), 21 (1) | – | `live_channels` | 4: Aufnahmen (Stufe 5) 4 | LS-14✓ | 8; 018–020, 161, 163, 179, 285, 326 | K (1), 0 (1), 1a (8), 2 (3), 5 (1) | E-28-07, E-28-59, E-28-23, E-28-28 | spezifiziert |
| LTV-016 | Multi-View und PiP | A – Ausgeschlossen: Ein Player zeigt genau einen Sender | 2 / 3 | 13 (1), 14 (1) | – | – | 14: Sender und Favoriten 3, Live-Sitzung und Wiedergabe 9, Aufnahmen (Stufe 5) 2 | LS-07✓, LS-10✓ | 4; 080, 174, 192, 330 | 1a (2) | – | spezifiziert |
| LTV-017 | Anbieterverwaltung | A – Keine separate Anbieterlimitverwaltung: Slouchcat verwaltet keine Verbindungslimits je IPTV-Anbieter; die allgemeinen Ressourcenlimits gelten, Zugangsdaten werden sicher gespeichert | 3 / 6 | 12 (1), 13 (1), 15 (1) | T-17, T-18\*, T-26\* | `live_sources` | 7: Verwaltung: Quellen und Importe 6, Verwaltung: Empfang, Einstellungen, EPG, Speicher 1 | LA-03✓ | 6; 039, 166, 180, 203, 329, 336 | 1a (3) | E-28-02, E-28-59, E-28-09, E-28-11, E-28-69, E-28-49 | spezifiziert |
| LTV-018 | Senderaktualisierung | B – Automatisch: Senderlisten werden automatisch und von Hand aktualisiert, ohne dass Pflege, Favoriten, Zuordnungen und Planungen … | 23 / 1 | 06 (1), 09 (20), 15 (2) | – | `live_channel_locks`, `live_channels`, `live_group_channels`, `live_source_import_items`\*, `live_source_imports`\*, `live_sources`\* | 16: Verwaltung: Quellen und Importe 13, Verwaltung: Sender, Gruppen, Duplikate 2, Verwaltung: Empfang, Einstellungen, EPG, Speicher 1 | LA-02✓, LA-03✓, LA-04✓ | 14; 021–023, 046–051, 059, 204, 323–325 | 1a (18), 1b (3), 2 (1), 5 (1) | E-28-41 | spezifiziert |
| LTV-019 | Speicherverwaltung | B – Erweitert: Startpuffer, Timeshift und Aufnahmen haben getrennt einstellbare Speicherorte mit Limits, Reserve, Bereinigung und Warnungen, … | 22 / 4 | 11 (6), 13 (7), 15 (9) | T-25 | `live_config`\*, `live_recordings`\*, `live_storage`\* | 10: Aufnahmen (Stufe 5) 2, Verwaltung: Empfang, Einstellungen, EPG, Speicher 8 | LA-15✓, LA-16✓, LS-12✓ | 18; 017, 025, 032, 143, 150–159, 182, 216, 307, 356 | 4 (17), 5 (6) | E-28-10, E-28-39, E-28-46, E-28-47, E-28-51 | spezifiziert |
| LTV-020 | Aufnahmequalität | A – Originalqualität: Aufnahmen werden ohne Neucodierung mit allen empfangenen Ton- und Untertitelspuren gespeichert; verlustfreies Umpacken ist … | 22 / 0 | 05 (1), 11 (20), 14 (1) | T-6, T-16\* | – | 8: Live-Sitzung und Wiedergabe 8 | LS-10✓ | 8; 014–015, 027, 140, 144–145, 182, 332 | K (2), 5 (20) | E-28-44 | spezifiziert |
| LTV-021 | Entwicklung | C – Konzept und Mockups vor Implementierung: Live-TV wird zuerst vollständig konzipiert und mit klickbaren Mockups gezeigt; gebaut wird erst nach … | 15 / 0 | 04 (1), 07 (1), 08 (1), 12 (1), 13 (2), 17 (2), 18 (1), 19 (1), 21 (1) | – | – | 0 | – | 1; 311 | K (15) | E-28-01, E-28-08, E-28-58 | spezifiziert |

Anmerkungen: «\*» an einer Tabelle: nur über eine Seite abgeleitet; «\*» an einem T-n: nur über ein `AK-L` des Entscheids abgeleitet (das `AK-L` nennt T-n und Entscheid). Bei LTV-001 (Sammelentscheid) und LTV-014 (Oberfläche) sind Seiten-,
Operations- und Tabellenspalten wegen der vielen Seiten breit; die Zeilen der Detailentscheide sind genauer. Die Zahl der `AK-L`
zählt Kriterien, die den Entscheid oder eine seiner Anforderungen nennen (Auftrag 15 und Ausschlüsse eingeschlossen).

---

## 3. Matrix B · je Anforderung

Eine Zeile je `LTV-nnn.mm`, nach Entscheid gruppiert. «Abn.» T = Test, M = Messung, S = Sichtprüfung. «AK-L» sind
die Kriterien aus `17`, die die Anforderung in der Spalte «Bezug» nennen («–»: keines, siehe Abschnitt 7). Kurztext: erste 12 Wörter von `03`.
Alle Zeilen: Status **spezifiziert**.

### LTV-001 Funktionsumfang (16 Anforderungen, 0 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-001.01 | Slouchcat hat einen eigenständigen Bereich «Live-TV», getrennt von Bibliothek, Webradio und Podcasts. | 1a | 14 | S | – | spezifiziert |
| LTV-001.02 | Slouchcat zeigt Internet-TV aus den Quellen nach LTV-003. | 1a | 04 | T | 312 | spezifiziert |
| LTV-001.03 | Slouchcat hat eine Senderverwaltung nach LTV-006. | 1a, 1b | 15 | S | – | spezifiziert |
| LTV-001.04 | Slouchcat liest Programmdaten (EPG) nach LTV-005. | 2 | 09 | T | 312 | spezifiziert |
| LTV-001.05 | Slouchcat zeigt eine elektronische Programmzeitschrift nach LTV-005 und LTV-014. | 2 | 14 | S | – | spezifiziert |
| LTV-001.06 | Slouchcat schaltet schnell um (Fast Zapping) nach LTV-010. | 3 | 05 | M | 082 | spezifiziert |
| LTV-001.07 | Slouchcat empfängt jeden benötigten Sender zentral nach LTV-004. | 1a | 05 | T | 312 | spezifiziert |
| LTV-001.08 | Slouchcat verteilt einen Eingangsstrom an mehrere Zuschauer und Aufnahmen (Stream-Sharing) nach LTV-004. | 1a | 05 | T | 312 | spezifiziert |
| LTV-001.09 | Slouchcat bietet persönliches Timeshift nach LTV-007. | 4 | 10 | T | 312 | spezifiziert |
| LTV-001.10 | Slouchcat nimmt auf (DVR) nach LTV-008. | 5 | 11 | T | 312 | spezifiziert |
| LTV-001.11 | Slouchcat bietet Serienaufnahmen nach LTV-008. | 6 | 11 | T | 312 | spezifiziert |
| LTV-001.12 | Jedes Profil hat eigene Senderfavoriten (DR-14). | 1a | 14 | T | 188, 312 | spezifiziert |
| LTV-001.13 | Slouchcat erinnert ein Profil an gewählte Sendungen (DR-08). | 2 | 09 | T | 312 | spezifiziert |
| LTV-001.14 | Slouchcat überwacht empfangene Ströme nach LTV-011. | 1a | 05 | T | 312 | spezifiziert |
| LTV-001.15 | Slouchcat hat eine umfangreiche Live-TV-Verwaltung nach Auftrag Abschnitt 9 (LTV-Q-09). | 1a | 15 | S | – | spezifiziert |
| LTV-001.16 | Bei Widerspruch zwischen LTV-001 und einem Detailentscheid gilt der Detailentscheid. | K | 02 | S | 312 | spezifiziert |

### LTV-002 Hybride Empfangsstrategie (13 Anforderungen, 0 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-002.01 | Ein Sender mit Strategie «permanent» wird rund um die Uhr empfangen, auch … | 3 | 05 | T | 005, 301, 305–306, 314, 334 | spezifiziert |
| LTV-002.02 | Mit Strategie «automatisch» hält Slouchcat häufig genutzte Sender nach Nutzung und freien … | 3 | 05 | T | 062 | spezifiziert |
| LTV-002.03 | Mit Strategie «bei Bedarf» startet Slouchcat den Empfang erst, wenn ein Profil … | 1a | 05 | T | 063 | spezifiziert |
| LTV-002.04 | Mit Strategie «zeitgesteuert» empfängt Slouchcat den Sender nach einem Zeitplan, den der … | 3 | 05 | T | 007, 314, 334 | spezifiziert |
| LTV-002.05 | Der Administrator legt eine globale Standardstrategie fest, die für jeden Sender ohne … | 3 | 15 | T | 061, 209 | spezifiziert |
| LTV-002.06 | Der Administrator kann je Sender eine eigene Strategie wählen, die der globalen … | 3 | 15 | T | 061, 209 | spezifiziert |
| LTV-002.07 | Slouchcat lädt benachbarte Sender (nach Kanalnummer) vor, wenn die Mittel reichen (LTV-010). … | 3 | 05 | T | 008, 083, 089, 316 | spezifiziert |
| LTV-002.08 | Slouchcat startet den Empfang für eine geplante Aufnahme selbst und rechtzeitig vor … | 5 | 11 | T | 013, 065, 149 | spezifiziert |
| LTV-002.09 | Slouchcat beendet einen Eingangsstrom, den kein Zuschauer, keine Aufnahme, keine Strategie und … | 1a | 05 | T | 006, 063, 315 | spezifiziert |
| LTV-002.10 | Slouchcat beendet nie einen Eingangsstrom, solange eine Wiedergabe ihn nutzt, auch nicht … | 1a | 05 | T | 004, 063, 313, 315, 334 | spezifiziert |
| LTV-002.11 | Slouchcat beendet nie einen Eingangsstrom, solange eine laufende Aufnahme ihn nutzt. | 5 | 05 | T | 004, 147, 313 | spezifiziert |
| LTV-002.12 | Nach einem Neustart nimmt Slouchcat den Empfang aller Sender mit Strategie «permanent» … | 3 | 06 | T | 005, 065, 280 | spezifiziert |
| LTV-002.13 | Das Konzept beschreibt eine ressourcenschonende Regel, wann Slouchcat Eingangsströme automatisch startet und … | K | 05 | S | 062 | spezifiziert |

### LTV-003 Unterstützte Empfangsquellen (25 Anforderungen, 11 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-003.01 | Der Administrator kann eine M3U-Datei hochladen und als Quelle importieren. | 1a | 15 | T | 034–035, 051, 203 | spezifiziert |
| LTV-003.02 | Der Administrator kann eine M3U8-Datei (Senderliste in UTF-8) hochladen und als Quelle … | 1a | 15 | T | 034–035, 051, 203 | spezifiziert |
| LTV-003.03 | Der Administrator kann eine M3U-Adresse als Quelle eintragen, die Slouchcat abruft. | 1a | 15 | T | 034, 051, 203 | spezifiziert |
| LTV-003.04 | Der Administrator kann eine M3U8-Adresse als Quelle eintragen, die Slouchcat abruft. | 1a | 15 | T | 034, 051, 203 | spezifiziert |
| LTV-003.05 | Der Administrator kann einen Sender mit einer von Hand erfassten Stream-Adresse anlegen. | 1a | 15 | T | 036, 203 | spezifiziert |
| LTV-003.06 | Slouchcat empfängt Ströme über HTTP. | 1a | 05 | T | 038 | spezifiziert |
| LTV-003.07 | Slouchcat empfängt Ströme über HTTPS. | 1a | 05 | T | 038 | spezifiziert |
| LTV-003.08 | Slouchcat empfängt HLS-Ströme. | 1a | 05 | T | 038 | spezifiziert |
| LTV-003.09 | Slouchcat empfängt MPEG-TS-Ströme. | 1a | 05 | T | 038 | spezifiziert |
| LTV-003.10 | Slouchcat empfängt MPEG-DASH-Ströme ohne DRM; DASH mit Kopierschutz lehnt es mit verständlicher … | W | 05 | T | 041 | spezifiziert |
| LTV-003.11 | Slouchcat bezieht Live-Sender (nur Live) über eine Xtream-Codes-kompatible Schnittstelle. | 1a | 09 | T | 037 | spezifiziert |
| LTV-003.12 | Slouchcat empfängt RTSP-Ströme. | L | 05 | T | 042 | spezifiziert |
| LTV-003.13 | Slouchcat empfängt RTMP-Ströme. | L | 05 | T | 042 | spezifiziert |
| LTV-003.14 | Slouchcat empfängt UDP-Ströme an festen Zielen der Quelle (A-3: ab Werk eine … | H | 05 | T | 043, 045 | spezifiziert |
| LTV-003.15 | Slouchcat empfängt RTP-Ströme an festen Zielen der Quelle (A-3, A-9). | H | 05 | T | 043, 045 | spezifiziert |
| LTV-003.16 | Slouchcat empfängt Multicast-Ströme an festen Zielen der Quelle (A-3; Netzmodus «host» oder … | H | 05 | T | 044–045 | spezifiziert |
| LTV-003.17 | Slouchcat empfängt Ströme, die eine Anmeldung verlangen (Benutzer und Passwort oder Token), … | 1a | 12 | T | 039 | spezifiziert |
| LTV-003.18 | Der Administrator kann je Quelle eigene HTTP-Header festlegen, die Slouchcat beim Abruf … | 1a | 15 | T | 039 | spezifiziert |
| LTV-003.19 | Der Administrator kann je Quelle den User-Agent festlegen. | 1a | 15 | T | 039 | spezifiziert |
| LTV-003.20 | Ein Sender kann mehrere alternative Quellen haben. | 1a | 07 | T | 024, 040 | spezifiziert |
| LTV-003.21 | Der Administrator legt die Rangfolge der Quellen eines Senders fest. | 1a | 15 | T | 024, 040 | spezifiziert |
| LTV-003.22 | Bei einer Störung wechselt Slouchcat selbst auf die nächste Ersatzquelle nach Rang. | 3 | 05 | T | 024, 102 | spezifiziert |
| LTV-003.23 | Slouchcat erkennt beim Import, ob eine M3U-Datei eine Senderliste oder ein HLS-Manifest … | 1a | 09 | T | 035 | spezifiziert |
| LTV-003.24 | Das Konzept beschreibt je Protokoll die besonderen technischen Anforderungen (Netz, Wiederverbindung, Risiken). | K | 05 | S | – | spezifiziert |
| LTV-003.25 | Das Konzept enthält eine Kompatibilitätsmatrix aus Eingangsprotokoll, Container, Codec und Client. | K | 16 | S | – | spezifiziert |

**Ausschlüsse zu LTV-003** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-003.X-01 | Nicht dabei: physische TV-Tuner. | – | S | 175 | spezifiziert |
| LTV-003.X-02 | Nicht dabei: HDHomeRun. | – | S | 175 | spezifiziert |
| LTV-003.X-03 | Nicht dabei: SAT>IP. | – | S | 175 | spezifiziert |
| LTV-003.X-04 | Nicht dabei: TVHeadend-Integration. | – | S | 175 | spezifiziert |
| LTV-003.X-05 | Nicht dabei: DVB-C. | – | S | 175 | spezifiziert |
| LTV-003.X-06 | Nicht dabei: DVB-S/S2. | – | S | 175 | spezifiziert |
| LTV-003.X-07 | Nicht dabei: DVB-T/T2. | – | S | 175 | spezifiziert |
| LTV-003.X-08 | Nicht dabei: DRM-geschützte Streams; Slouchcat erkennt sie und lehnt sie ab. | 12 | T | 041, 176, 273 | spezifiziert |
| LTV-003.X-09 | Nicht dabei: Umgehung von Kopierschutzmechanismen. | 12 | S | 176 | spezifiziert |
| LTV-003.X-10 | Nicht dabei: Xtream-VOD-Bibliotheken; Slouchcat ruft Xtream-VOD weder ab noch zeigt es sie. | – | T | 037, 177 | spezifiziert |
| LTV-003.X-11 | Nicht dabei: Xtream-Serienbibliotheken. | – | T | 037, 177 | spezifiziert |

### LTV-004 Zentraler Streamempfang und individuelles Transcoding (38 Anforderungen, 1 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-004.01 | Slouchcat empfängt einen benötigten Sender möglichst nur einmal, von der aktiven, kompatiblen … | 1a | 05 | T | 001 | spezifiziert |
| LTV-004.02 | Ein Eingangsstrom steht zugleich mehreren Zuschauern und Aufnahmen zur Verfügung. | 1a | 05 | T | 001 | spezifiziert |
| LTV-004.03 | Prüffall: 20 Zuschauer schauen denselben Sender. | 1a | 17 | T | 001, 300 | spezifiziert |
| LTV-004.04 | Im Prüffall LTV-004.03 empfängt Slouchcat den Sender genau einmal. | 1a | 17 | T | 001, 300 | spezifiziert |
| LTV-004.05 | Im Prüffall LTV-004.03 verteilt Slouchcat den Strom intern an alle 20 Zuschauer. | 1a | 17 | T | 001, 300 | spezifiziert |
| LTV-004.06 | Eine zusätzliche Aufnahme desselben Senders öffnet möglichst keine weitere Eingangsverbindung. | 5 | 17 | T | 013, 303 | spezifiziert |
| LTV-004.07 | Jeder Zuschauer hat eine eigene, unabhängige Wiedergabesitzung. | 1a | 05 | T | 002, 330 | spezifiziert |
| LTV-004.08 | Passen Originalqualität und Format zum Gerät, rechnet Slouchcat Bild und Ton nicht … | 1a | 05 | T | 002, 068 | spezifiziert |
| LTV-004.09 | Braucht ein Zuschauer eine Umwandlung, bekommt er einen eigenen Umwandlungsprozess, aber nur … | 1a | 05 | T | 002 | spezifiziert |
| LTV-004.10 | Slouchcat gibt den Strom in Originalform weiter, wenn das Gerät ihn kann. | 1a | 05 | T | 068 | spezifiziert |
| LTV-004.11 | Slouchcat packt den Strom verlustfrei um (Remux), wenn nur der Container nicht … | 1a | 05 | T | 069 | spezifiziert |
| LTV-004.12 | Slouchcat wandelt das Bild je Zuschauer um, wenn Gerät oder Grenze es … | 1a | 05 | T | 002, 070, 308, 320 | spezifiziert |
| LTV-004.13 | Slouchcat wandelt den Ton je Zuschauer um, wenn Gerät oder Grenze es … | 1a | 05 | T | 002, 069, 308 | spezifiziert |
| LTV-004.14 | Jeder Zuschauer kann innerhalb seiner Grenzen eine eigene Auflösung bekommen. | 1a | 05 | T | 002, 071 | spezifiziert |
| LTV-004.15 | Jeder Zuschauer kann innerhalb seiner Grenzen eine eigene Bitrate bekommen. | 1a | 05 | T | 002, 071 | spezifiziert |
| LTV-004.16 | Jeder Zuschauer wählt seine Tonspur selbst, ohne andere Zuschauer zu beeinflussen. | 1a | 05 | T | 002, 072 | spezifiziert |
| LTV-004.17 | Jeder Zuschauer wählt seine Untertitelspur selbst, ohne andere Zuschauer zu beeinflussen. Stand … | 1a; E | 05 | T | 002, 072, 332, 353 | spezifiziert |
| LTV-004.18 | Slouchcat nutzt für Umwandlungen die bestehende Hardwarebeschleunigung. | 1a | 05 | T | 070, 308 | spezifiziert |
| LTV-004.19 | Ohne verfügbare Hardware fällt Slouchcat wie heute auf Software zurück, gemäss der … | 1a | 05 | T | 070 | spezifiziert |
| LTV-004.20 | Live-TV hält die bestehenden Konto- und Ressourcenlimits ein (Ströme, Bandbreite, Auflösung, Budget … | 1a | 13 | T | 033, 067, 076, 301 | spezifiziert |
| LTV-004.21 | Slouchcat hat eine zentrale Streamverwaltung mit klarem Lebenszyklus je Eingangsstrom (Start, Betrieb, … | 1a | 05 | S | – | spezifiziert |
| LTV-004.22 | Slouchcat öffnet für denselben Sender nie zwei Eingangsströme, wenn einer genügt. | 1a | 05 | T | 001, 064 | spezifiziert |
| LTV-004.23 | Gleichzeitige Anfragen für denselben Sender führen zu genau einem Eingangsstrom. | 1a | 05 | T | 001, 064 | spezifiziert |
| LTV-004.24 | Gleichzeitiges Starten, Beitreten und Verlassen desselben Senders führt nie zu einem verlorenen … | 1a | 05 | T | 064 | spezifiziert |
| LTV-004.25 | Slouchcat weiss jederzeit, wer einen Eingangsstrom nutzt (Zuschauer, Aufnahme, Strategie, Vorladen). | 1a | 05 | T | 075 | spezifiziert |
| LTV-004.26 | Slouchcat verwaltet die Abnehmer eines Eingangsstroms (Hinzufügen, Entfernen, Zählen). | 1a | 05 | T | 075 | spezifiziert |
| LTV-004.27 | Bricht die Verbindung eines Zuschauers ab, entfernt Slouchcat ihn als Abnehmer, ohne … | 1a | 05 | T | 004 | spezifiziert |
| LTV-004.28 | Ein langsamer Zuschauer verlangsamt weder den Eingangsstrom noch andere Zuschauer. | 1a | 05 | T | 073, 300 | spezifiziert |
| LTV-004.29 | Slouchcat behandelt Rückstau (Backpressure) je Abnehmer, ohne den Eingangsstrom anzuhalten. | 1a | 05 | T | 073 | spezifiziert |
| LTV-004.30 | Alle Puffer der Verteilung haben eine feste Obergrenze. | 1a | 05 | T | 074 | spezifiziert |
| LTV-004.31 | Slouchcat hinterlässt keine verwaisten Prozesse und räumt sie nach einem Absturz auf. | 1a | 05 | T | 026, 030, 302 | spezifiziert |
| LTV-004.32 | Fällt die Quelle aus, kann der Eingangsstrom auf eine Ersatzquelle wechseln (LTV-003.22). | 3 | 05 | T | 024 | spezifiziert |
| LTV-004.33 | Slouchcat gibt Prozesse, Verbindungen, Puffer und Dateien frei, sobald sie niemand mehr … | 1a | 05 | T | 030, 302 | spezifiziert |
| LTV-004.34 | Jeder Zuschauer bleibt von den anderen unabhängig (Start, Pause, Ende, Qualität). | 1a | 05 | T | 003 | spezifiziert |
| LTV-004.35 | Slouchcat legt fest, wann zwei Abrufe derselbe Strom sind, auch bei unterschiedlichen … | 1a | 05 | T | 040 | spezifiziert |
| LTV-004.36 | Ein langsamer oder fehlerhafter Client beeinträchtigt andere Zuschauer nicht. | 1a | 05 | T | 003, 073, 300 | spezifiziert |
| LTV-004.37 | Beendet ein Zuschauer seine Wiedergabe, laufen Empfang für andere Zuschauer und laufende … | 1a | 05 | T | 004 | spezifiziert |
| LTV-004.38 | Die Verwaltungsseite der Quelle weist darauf hin, dass der Administrator für die … | 1a | 15 | S | – | spezifiziert |

**Ausschlüsse zu LTV-004** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-004.X-01 | Nicht dabei: gemeinsam genutzte Transcoding-Ausgaben für mehrere Zuschauer. | 05 | T | 002, 181 | spezifiziert |

### LTV-005 EPG und Programmzeitschrift (28 Anforderungen, 3 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-005.01 | Der Administrator kann eine XMLTV-Datei hochladen. | 2 | 09 | T | 104 | spezifiziert |
| LTV-005.02 | Der Administrator kann eine XMLTV-Adresse eintragen, die Slouchcat abruft. | 2 | 09 | T | 104 | spezifiziert |
| LTV-005.03 | Slouchcat liest komprimierte XMLTV-Dateien. | 2 | 09 | T | 104 | spezifiziert |
| LTV-005.04 | Slouchcat nutzt mehrere EPG-Quellen zugleich; jeder Sender hat genau eine EPG-Zuordnung (DR-19). | 2 | 09 | T | 105 | spezifiziert |
| LTV-005.05 | Der Administrator kann einen Sender von Hand einem EPG-Eintrag zuordnen. | 2 | 09 | T | 106 | spezifiziert |
| LTV-005.06 | Slouchcat schlägt Zuordnungen selbst vor bzw. ordnet eindeutig passende Sender selbst zu. | 2 | 09 | T | 106 | spezifiziert |
| LTV-005.07 | Slouchcat aktualisiert die Programmdaten automatisch nach dem Intervall der Quelle. | 2 | 09 | T | 023, 116, 304 | spezifiziert |
| LTV-005.08 | Slouchcat zeigt je Sender die laufende Sendung. | 2 | 14 | T | 110, 189 | spezifiziert |
| LTV-005.09 | Slouchcat zeigt je Sender kommende Sendungen. | 2 | 14 | T | 110, 189, 200 | spezifiziert |
| LTV-005.10 | Slouchcat zeigt den Sendungstitel. | 2 | 14 | T | 110 | spezifiziert |
| LTV-005.11 | Slouchcat zeigt die Sendungsbeschreibung, wenn die Quelle sie liefert. | 2 | 14 | T | 110 | spezifiziert |
| LTV-005.12 | Slouchcat zeigt Sendungsbilder, wenn die Quelle sie liefert (über den Server, 12-sicherheit.md). | 2 | 09 | T | 110, 119 | spezifiziert |
| LTV-005.13 | Slouchcat zeigt Kategorien, wenn die Quelle sie liefert. | 2 | 09 | T | 110 | spezifiziert |
| LTV-005.14 | Slouchcat zeigt Genres, wenn die Quelle sie liefert, und kann danach filtern. | 2 | 09 | T | 110 | spezifiziert |
| LTV-005.15 | Ein Profil kann über alle sichtbaren Sender nach Sendungen suchen (F-5). | 2 | 09 | T | 111, 190 | spezifiziert |
| LTV-005.16 | Der Administrator legt fest, wie viele Tage voraus Slouchcat Programmdaten führt und … | 2 | 15 | T | 112 | spezifiziert |
| LTV-005.17 | Slouchcat zeigt eine elektronische Programmzeitschrift (Ausgestaltung LTV-014.20 ff.). | 2 | 14 | S | – | spezifiziert |
| LTV-005.18 | Ein Profil kann aus der Programmzeitschrift eine Aufnahme planen. | 5 | 11 | T | 115 | spezifiziert |
| LTV-005.19 | Ein Profil kann aus der Programmzeitschrift eine Erinnerung setzen (DR-08). | 2 | 09 | T | 114, 198 | spezifiziert |
| LTV-005.20 | Slouchcat rechnet Zeitangaben mit Zeitzone richtig um und zeigt sie in der … | 2 | 09 | T | 107 | spezifiziert |
| LTV-005.21 | Slouchcat zeigt Sendungen über die Umstellung von Sommer- auf Winterzeit und zurück … | 2 | 09 | T | 108 | spezifiziert |
| LTV-005.22 | Slouchcat verwirft Sendungen mit unmöglichen Zeitangaben (Ende vor Beginn, überlappend) nachvollziehbar, ohne … | 2 | 09 | T | 023, 109 | spezifiziert |
| LTV-005.23 | Fehlende Angaben bleiben leer; Slouchcat zeigt eine Lücke als Lücke. | 2 | 09 | T | 023, 109 | spezifiziert |
| LTV-005.24 | Ein Sender ohne EPG-Zuordnung bleibt nutzbar und zeigt «Keine Programmdaten» (DR-09); die … | 2 | 09 | T | 023, 113 | spezifiziert |
| LTV-005.25 | Ändert eine Aktualisierung die Zeit einer Sendung, zeigt Slouchcat die neue Zeit. | 2 | 09 | T | 023 | spezifiziert |
| LTV-005.26 | Ändert eine Aktualisierung die Zeit einer geplanten Sendung, passt Slouchcat die Aufnahmeplanung … | 5 | 11 | T | 023, 115, 138 | spezifiziert |
| LTV-005.27 | Slouchcat ergänzt unvollständige Programmdaten nie durch erfundene Angaben. | 2 | 09 | T | 023, 109 | spezifiziert |
| LTV-005.28 | Fällt eine EPG-Quelle aus, spielt Slouchcat die Sender weiter ab. | 2 | 09 | T | 023, 113, 304 | spezifiziert |

**Ausschlüsse zu LTV-005** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-005.X-01 | Nicht dabei: intelligente Zusammenführung konkurrierender EPG-Quellen je Sender. | 09 | S | 105 | spezifiziert |
| LTV-005.X-02 | Nicht dabei: KI-Empfehlungen. | – | S | 178 | spezifiziert |
| LTV-005.X-03 | Nicht dabei: automatische Verknüpfung mit den Film- und Serienbibliotheken. | – | S | 183 | spezifiziert |

### LTV-006 Erweiterte Senderverwaltung (20 Anforderungen, 1 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-006.01 | Der Administrator hat eine übersichtliche Senderverwaltung, die auch mit 10'000 Sendern bedienbar … | 1a | 15 | M | 052, 205 | spezifiziert |
| LTV-006.02 | Slouchcat übernimmt die Gruppen einer M3U-Liste als Sendergruppen. | 1a | 09 | T | 053, 208 | spezifiziert |
| LTV-006.03 | Der Administrator kann eigene Sendergruppen anlegen, umbenennen und löschen. | 1b | 15 | T | 053, 208 | spezifiziert |
| LTV-006.04 | Ein Sender kann zu mehreren Sendergruppen gehören. | 1b | 07 | T | 053, 208 | spezifiziert |
| LTV-006.05 | Der Administrator kann einen Sender aktivieren und deaktivieren; ein deaktivierter Sender ist … | 1a | 15 | T | 054, 313 | spezifiziert |
| LTV-006.06 | Der Administrator kann einen Sender umbenennen. | 1a | 15 | T | 055 | spezifiziert |
| LTV-006.07 | Slouchcat zeigt Senderlogos aus der Quelle (über den Server, 12-sicherheit.md). | 1a | 14 | T | 056 | spezifiziert |
| LTV-006.08 | Der Administrator kann das Logo eines Senders von Hand ersetzen. | 1b | 15 | T | 055 | spezifiziert |
| LTV-006.09 | Der Administrator kann die Reihenfolge der Sender von Hand festlegen. | 1b | 15 | T | 055 | spezifiziert |
| LTV-006.10 | Der Administrator kann jedem Sender eine eigene Kanalnummer geben. | 1b | 15 | T | 055 | spezifiziert |
| LTV-006.11 | Der Administrator kann gleiche Sender aus verschiedenen Quellen zu einem Sender mit … | 1b | 15 | T | 048, 057 | spezifiziert |
| LTV-006.12 | Der Administrator kann die Rangfolge der Senderquellen eines Senders festlegen (wie LTV-003.21). | 1a | 15 | T | 040, 206 | spezifiziert |
| LTV-006.13 | Der Administrator kann einem Sender ein Land zuordnen. | 1a | 15 | T | 058 | spezifiziert |
| LTV-006.14 | Der Administrator kann einem Sender eine Sprache zuordnen. | 1a | 15 | T | 058 | spezifiziert |
| LTV-006.15 | Der Administrator kann einem Sender Kategorien zuordnen. | 1a | 15 | T | 058 | spezifiziert |
| LTV-006.16 | Der Administrator kann mehrere Sender auf einmal bearbeiten (aktivieren, Gruppe, Land, Sprache, … | 1b | 15 | T | 058, 205 | spezifiziert |
| LTV-006.17 | Der Administrator kann Sender von Hand als Duplikat markieren und zusammenführen. | 1b | 15 | T | 057, 207 | spezifiziert |
| LTV-006.18 | Slouchcat schlägt wahrscheinliche Duplikate selbst vor; zusammengeführt wird erst nach Bestätigung des … | 1b | 09 | T | 057, 207 | spezifiziert |
| LTV-006.19 | Jeder Sender hat eine stabile interne Kennung, unabhängig von seiner aktuellen Stream-Adresse. | 1a | 07 | T | 048, 059 | spezifiziert |
| LTV-006.20 | Sendergruppen sind reine Inhaltskategorien; sie vergeben keine Rechte und sind keine Benutzergruppen. | 1a | 07 | S | – | spezifiziert |

**Ausschlüsse zu LTV-006** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-006.X-01 | Nicht dabei: ein zusätzliches Benutzergruppenmodell. | – | S | 184 | spezifiziert |

### LTV-007 Persönliches Timeshift (14 Anforderungen, 1 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-007.01 | Timeshift ist je Zuschauer (Wiedergabesitzung) getrennt. | 4 | 10 | T | 009 | spezifiziert |
| LTV-007.02 | Der persönliche Puffer beginnt beim Einschalten eines Senders. | 4 | 10 | T | 009, 123 | spezifiziert |
| LTV-007.03 | Jeder Zuschauer hat seinen eigenen Timeshift-Puffer. | 4 | 10 | T | 009, 011 | spezifiziert |
| LTV-007.04 | Ein Zuschauer kann Live-TV pausieren. | 4 | 10 | T | 120 | spezifiziert |
| LTV-007.05 | Ein Zuschauer kann innerhalb seines Puffers zurückspulen. | 4 | 10 | T | 121 | spezifiziert |
| LTV-007.06 | Ein Zuschauer kann bis zum Live-Zeitpunkt vorspulen, nicht darüber hinaus. | 4 | 10 | T | 122 | spezifiziert |
| LTV-007.07 | Ein Zuschauer kann direkt zum Live-Zeitpunkt springen. | 4 | 10 | T | 122 | spezifiziert |
| LTV-007.08 | Beim Senderwechsel löscht Slouchcat den bisherigen Puffer des Zuschauers. | 4 | 10 | T | 010 | spezifiziert |
| LTV-007.09 | Beim Beenden der Wiedergabe löscht Slouchcat die temporären Dateien des Puffers. | 4 | 10 | T | 010 | spezifiziert |
| LTV-007.10 | Der Administrator stellt die maximale Timeshift-Dauer ein (ab Werk 60 Minuten, wählbar … | 4 | 15 | T | 124, 154, 215 | spezifiziert |
| LTV-007.11 | Ein Zuschauer kann nie auf Inhalte zurückspulen, die vor seinem Einschalten gesendet … | 4 | 10 | T | 123 | spezifiziert |
| LTV-007.12 | Der technische Startpuffer ist vom persönlichen Puffer getrennt (eigener Zweck, eigene Grenze, … | 3 | 05 | S | – | spezifiziert |
| LTV-007.13 | Der Startpuffer dient nur dem schnelleren Start; kein Zuschauer kann über ihn … | 3 | 05 | T | 087, 123 | spezifiziert |
| LTV-007.14 | Timeshift verändert den gemeinsamen Eingangsstrom nicht (Pause eines Zuschauers hält ihn nicht … | 4 | 10 | T | 120 | spezifiziert |

**Ausschlüsse zu LTV-007** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-007.X-01 | Nicht dabei: ein gemeinsames, serverweites Timeshift-Archiv. | 10 | T | 171–172 | spezifiziert |

### LTV-008 Erweiterte DVR-Funktion (26 Anforderungen, 5 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-008.01 | Ein Profil kann eine Sofortaufnahme des laufenden Senders starten (im Player und … | 5 | 11 | T | 130, 356 | spezifiziert |
| LTV-008.02 | Ein Profil kann eine einzelne künftige Sendung zur Aufnahme planen. | 5 | 11 | T | 131, 330 | spezifiziert |
| LTV-008.03 | Ein Profil kann eine Aufnahme direkt aus der Programmzeitschrift planen (wie LTV-005.18). | 5 | 11 | T | 131, 330 | spezifiziert |
| LTV-008.04 | Ein Profil kann eine manuelle Aufnahme mit Sender, Start- und Endzeit planen, … | 5 | 11 | T | 132 | spezifiziert |
| LTV-008.05 | Ein Profil kann eine Serienaufnahme anlegen, die passende Folgen selbst plant. | 6 | 11 | T | 133 | spezifiziert |
| LTV-008.06 | Ein Profil kann eine wiederkehrende Aufnahme nach Wochentag und Uhrzeit anlegen. | 6 | 11 | T | 134 | spezifiziert |
| LTV-008.07 | Ein Profil kann je Planung einen Vorlauf einstellen; ohne Angabe gilt der … | 5 | 11 | T | 135, 214 | spezifiziert |
| LTV-008.08 | Ein Profil kann je Planung einen Nachlauf einstellen; ohne Angabe gilt der … | 5 | 11 | T | 135, 214 | spezifiziert |
| LTV-008.09 | Slouchcat erkennt bereits aufgenommene Folgen, wenn die Programmdaten sie zuverlässig kennzeichnen, und … | 6 | 09 | T | 133 | spezifiziert |
| LTV-008.10 | Slouchcat nimmt verschiedene Sender gleichzeitig auf. | 5 | 11 | T | 136, 303 | spezifiziert |
| LTV-008.11 | Slouchcat löscht Aufnahmen nach der Aufbewahrungsfrist, die das Profil gewählt hat (A-4, … | 5 | 11 | T | 143 | spezifiziert |
| LTV-008.12 | Slouchcat hält ein Speicherkontingent je Konto für alle Aufnahmen seiner Profile ein … | 5 | 11 | T | 017 | spezifiziert |
| LTV-008.13 | Slouchcat erkennt Konflikte einer Planung vor ihrer Ausführung und zeigt sie an … | 5 | 11 | T | 137 | spezifiziert |
| LTV-008.14 | Slouchcat passt geplante Aufnahmen an geänderte EPG-Zeiten an, wenn die Sendung eindeutig … | 5 | 09 | T | 138 | spezifiziert |
| LTV-008.15 | Aufnahmen laufen unabhängig davon, ob jemand angemeldet ist. | 5 | 11 | T | 012 | spezifiziert |
| LTV-008.16 | Ein Profil sieht eine Übersicht seiner laufenden Aufnahmen. | 5 | 14 | T | 139 | spezifiziert |
| LTV-008.17 | Ein Profil sieht eine Übersicht seiner geplanten Aufnahmen. | 5 | 14 | T | 139 | spezifiziert |
| LTV-008.18 | Ein Profil sieht eine Übersicht seiner abgeschlossenen Aufnahmen. | 5 | 14 | T | 139, 193 | spezifiziert |
| LTV-008.19 | Ein Profil sieht eine Übersicht seiner fehlgeschlagenen Aufnahmen mit Grund. | 5 | 14 | T | 139, 193 | spezifiziert |
| LTV-008.20 | Ein Profil kann abgeschlossene Aufnahmen abspielen. | 5 | 11 | T | 140, 193, 195 | spezifiziert |
| LTV-008.21 | Ein Profil kann eigene Aufnahmen löschen. | 5 | 11 | T | 141, 193 | spezifiziert |
| LTV-008.22 | Slouchcat zeigt bei einer unvollständigen Aufnahme verständlich, welche Teile fehlen und warum. | 5 | 11 | T | 027, 142, 194 | spezifiziert |
| LTV-008.23 | Aufnahmen nutzen die zentrale Streamverwaltung (LTV-004). | 5 | 05 | T | 013 | spezifiziert |
| LTV-008.24 | Wird ein Sender schon empfangen, nimmt Slouchcat aus dem bestehenden Eingangsstrom auf. | 5 | 05 | T | 013 | spezifiziert |
| LTV-008.25 | Eine Aufnahme hängt nicht davon ab, ob das aufnehmende Profil gerade schaut. | 5 | 11 | T | 012 | spezifiziert |
| LTV-008.26 | Slouchcat behandelt Serverneustart, Stromausfall, Netzunterbruch und fehlenden Speicher bei Aufnahmen so, dass … | 5 | 06 | T | 027, 145 | spezifiziert |

**Ausschlüsse zu LTV-008** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-008.X-01 | Nicht dabei: automatische Werbeerkennung. | – | S | 178 | spezifiziert |
| LTV-008.X-02 | Nicht dabei: KI-Funktionen. | – | S | 178 | spezifiziert |
| LTV-008.X-03 | Nicht dabei: suchwortbasierte automatische Aufnahmen. | – | S | 178 | spezifiziert |
| LTV-008.X-04 | Nicht dabei: automatische Aufnahmen bestimmter Schauspieler oder Sportmannschaften. | – | S | 178 | spezifiziert |
| LTV-008.X-05 | Nicht dabei: automatische Übernahme von Aufnahmen in die Film- und Serienbibliotheken. | 11 | T | 148, 183 | spezifiziert |

### LTV-009 Aufnahmeeigentum · Private Aufnahmen pro Profil (12 Anforderungen, 0 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-009.01 | Jede Aufnahme gehört genau einem Profil, dem Profil, das sie geplant oder … | 5 | 07 | T | 016, 164 | spezifiziert |
| LTV-009.02 | Jedes Profil hat eigene Aufnahmen. | 5 | 07 | T | 016, 164 | spezifiziert |
| LTV-009.03 | Jedes Profil hat eigene Aufnahmeplanungen. | 5 | 07 | T | 016, 164 | spezifiziert |
| LTV-009.04 | Jedes Profil hat eigene Serienaufnahmen. | 6 | 07 | T | 016, 133 | spezifiziert |
| LTV-009.05 | Jedes Profil hat einen eigenen Wiedergabefortschritt je Aufnahme (DR-21). | 5 | 07 | T | 016, 140 | spezifiziert |
| LTV-009.06 | Jedes Profil kann seine Aufnahmen und Planungen selbst löschen. | 5 | 11 | T | 016, 141 | spezifiziert |
| LTV-009.07 | Andere Profile desselben Kontos sehen und spielen die Aufnahmen eines Profils nicht; … | 5 | 12 | T | 016, 163, 173, 199, 328 | spezifiziert |
| LTV-009.08 | Andere Konten haben keinen Zugriff auf die Aufnahmen; jeder Zugriff ist 404. | 5 | 12 | T | 016, 164 | spezifiziert |
| LTV-009.09 | Das Speicherkontingent gilt für das Konto als Ganzes (Summe der Aufnahmen aller … | 5 | 11 | T | 016–017 | spezifiziert |
| LTV-009.10 | Die Privatsphäre des bestehenden Profilmodells bleibt erhalten (Aufnahmen, Planungen, Favoriten, Erinnerungen, Fortschritt … | 5 | 12 | T | 016, 148, 165 | spezifiziert |
| LTV-009.11 | Der Administrator sieht Speicherbelegung je Konto und technische Fehler, aber keine Titel, … | 5 | 15 | T | 153, 165, 212, 327 | spezifiziert |
| LTV-009.12 | Das Konzept untersucht, ob gleiche Aufnahmen mehrerer Profile physisch einmal gespeichert werden … | K | 11 | S | – | spezifiziert |

### LTV-010 Erweitertes Fast Zapping (43 Anforderungen, 1 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-010.01 | Schnelles Umschalten hat bei Abwägungen in Konzept und Bau hohe Priorität. | K | 05 | S | – | spezifiziert |
| LTV-010.02 | Wird ein Sender schon empfangen, nutzt Slouchcat beim Umschalten diesen Eingangsstrom sofort. | 1a | 05 | T | 090, 302, 317 | spezifiziert |
| LTV-010.03 | Die Player starten Live-Ströme mit optimierten Einstellungen für einen schnellen Start. | 1a; 3 | 05 | M | 088 | spezifiziert |
| LTV-010.04 | Slouchcat hält je empfangenem Sender einen kleinen technischen Startpuffer. | 3 | 05 | T | 087 | spezifiziert |
| LTV-010.05 | Slouchcat lädt benachbarte Sender vor (wie LTV-002.07). | 3 | 05 | T | 008, 089, 316 | spezifiziert |
| LTV-010.06 | Slouchcat lädt häufig genutzte Sender vor (global und je Sender gesteuert, DR-29). | 3 | 05 | T | 008, 089 | spezifiziert |
| LTV-010.07 | Slouchcat berücksichtigt beim Vorladen die Senderfavoriten der aktiven Zuschauer. | 3 | 05 | T | 008, 089 | spezifiziert |
| LTV-010.08 | Slouchcat gibt vorgeladene Ströme frei, wenn sie nicht mehr gebraucht werden (wie … | 3 | 05 | T | 008, 089 | spezifiziert |
| LTV-010.09 | Die Grösse der Startpuffer ist auf schnellen Start bei geringem Speicher abgestimmt. | 3 | 05 | M | 087 | spezifiziert |
| LTV-010.10 | Slouchcat bereitet die Ausgabe je Gerätetyp so vor, dass jedes Gerät ohne … | 1a; 3 | 16 | M | 088 | spezifiziert |
| LTV-010.11 | Das Konzept untersucht Keyframe-Abstände. | K | 05 | S | – | spezifiziert |
| LTV-010.12 | Das Konzept untersucht GOP-Strukturen. | K | 05 | S | – | spezifiziert |
| LTV-010.13 | Das Konzept untersucht HLS-Segmentlängen. | K | 05 | S | – | spezifiziert |
| LTV-010.14 | Das Konzept untersucht Low-Latency HLS. | K | 05 | S | – | spezifiziert |
| LTV-010.15 | Das Konzept untersucht MPEG-TS. | K | 05 | S | – | spezifiziert |
| LTV-010.16 | Das Konzept untersucht Anlaufzeiten der Ströme. | K | 05 | S | – | spezifiziert |
| LTV-010.17 | Das Konzept untersucht die Pufferung in den Clients. | K | 16 | S | – | spezifiziert |
| LTV-010.18 | Das Konzept untersucht die Wechselzeiten der Player. | K | 16 | S | – | spezifiziert |
| LTV-010.19 | Das Konzept untersucht die Wiederverwendung bestehender Empfangsverbindungen. | K | 05 | S | – | spezifiziert |
| LTV-010.20 | Das Konzept untersucht, wie am schnellsten ein decodierbares Bild bereitsteht. | K | 05 | S | – | spezifiziert |
| LTV-010.21 | Das Konzept untersucht die Synchronität von Bild und Ton. | K | 05 | S | – | spezifiziert |
| LTV-010.22 | Das Konzept untersucht die Wirkung der Eingangsprotokolle. | K | 05 | S | – | spezifiziert |
| LTV-010.23 | Das Konzept untersucht die Wirkung der Video-Codecs. | K | 05 | S | – | spezifiziert |
| LTV-010.24 | Das Konzept untersucht das Verhalten der Browser und der nativen Clients. | K | 16 | S | – | spezifiziert |
| LTV-010.25 | Das Konzept bestimmt je Client das am besten geeignete Auslieferungsprotokoll. | K | 16 | S | – | spezifiziert |
| LTV-010.26 | Das Konzept legt sich nicht vorschnell auf ein Streaming-Protokoll fest, sondern begründet … | K | 05 | S | – | spezifiziert |
| LTV-010.27 | Das Konzept vergleicht Varianten nach Kompatibilität, Geschwindigkeit, Stabilität, Sicherheit und Ressourcenbedarf. | K | 05 | S | – | spezifiziert |
| LTV-010.28 | Zielwert: Umschalten auf einen schon empfangenen, kompatiblen Sender unter 1 Sekunde. | 3 | 13 | M | 082 | spezifiziert |
| LTV-010.29 | Zielwert: Umschalten auf einen vorgeladenen Sender unter 1 Sekunde. | 3 | 13 | M | 083 | spezifiziert |
| LTV-010.30 | Zielwert: Umschalten auf einen noch nicht empfangenen Sender unter 3 Sekunden. | 3 | 13 | M | 084 | spezifiziert |
| LTV-010.31 | Zielwert: Umschalten mit neuer Umwandlung unter 5 Sekunden. | 3 | 13 | M | 085 | spezifiziert |
| LTV-010.32 | Die Zielwerte sind zu prüfende Grössen, keine Zusagen; Konzept und Bau behaupten … | K | 13 | S | – | spezifiziert |
| LTV-010.33 | Der Umschalt-Benchmark nennt die Referenzhardware. | K | 13 | S | – | spezifiziert |
| LTV-010.34 | Der Benchmark nennt die Testquellen (selbst erzeugt, rechtlich unbedenklich). | K | 13 | S | – | spezifiziert |
| LTV-010.35 | Der Benchmark nennt die Eingangsprotokolle. | K | 13 | S | – | spezifiziert |
| LTV-010.36 | Der Benchmark nennt die Client-Geräte. | K | 13 | S | – | spezifiziert |
| LTV-010.37 | Der Benchmark nennt die Netzbedingungen. | K | 13 | S | – | spezifiziert |
| LTV-010.38 | Der Benchmark beschreibt die Messmethode. | K | 13 | S | – | spezifiziert |
| LTV-010.39 | Der Benchmark nennt die Zahl der Wiederholungen. | K | 13 | S | – | spezifiziert |
| LTV-010.40 | Der Benchmark wertet Median und geeignete Perzentile aus. | K | 13 | S | – | spezifiziert |
| LTV-010.41 | Der Benchmark weist die Fehlerquote aus. | K | 13 | S | – | spezifiziert |
| LTV-010.42 | Gemessen wird vom Befehl des Benutzers bis zum sichtbaren Bild. | 3 | 13 | M | 082, 086 | spezifiziert |
| LTV-010.43 | Das Konzept bewertet die Zielkonflikte zwischen Umschaltzeit, Bildqualität, Ressourcen und Verzögerung zum … | K | 05 | S | – | spezifiziert |

**Ausschlüsse zu LTV-010** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-010.X-01 | Nicht dabei: vorsorglich gestartete individuelle Umwandlungen für Sender, die niemand schaut. | 05 | T | 008, 181 | spezifiziert |

### LTV-011 Erweiterte Streamüberwachung (21 Anforderungen, 1 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-011.01 | Slouchcat prüft die Verfügbarkeit der Sender regelmässig, in einem massvollen Takt (LTV-011.18). | 3 | 05 | T | 092 | spezifiziert |
| LTV-011.02 | Slouchcat verbindet sich nach einem Abbruch selbst neu. | 1a | 05 | T | 093 | spezifiziert |
| LTV-011.03 | Slouchcat erkennt einen unterbrochenen Strom (keine Daten mehr). | 1a | 05 | T | 093 | spezifiziert |
| LTV-011.04 | Slouchcat erkennt einen eingefrorenen Strom: Stillstand der Zeit in Stufe 3, stehendes … | 3; E | 05 | T | 094, 103, 331 | spezifiziert |
| LTV-011.05 | Slouchcat erkennt fehlende Ton- oder Bilddaten. | 3 | 05 | T | 095, 103 | spezifiziert |
| LTV-011.06 | Slouchcat überwacht die Bitrate eines Stroms. | 3 | 05 | T | 096, 103 | spezifiziert |
| LTV-011.07 | Slouchcat überwacht die Auflösung eines Stroms. | 3 | 05 | T | 096, 103 | spezifiziert |
| LTV-011.08 | Slouchcat überwacht die Codecs eines Stroms und erkennt Wechsel. | 3 | 05 | T | 096, 103 | spezifiziert |
| LTV-011.09 | Slouchcat wechselt bei Störung selbst auf eine Ersatzquelle (wie LTV-003.22). | 3 | 05 | T | 024, 102 | spezifiziert |
| LTV-011.10 | Slouchcat stellt einen Strom möglichst unterbrechungsarm wieder her (Zuschauer bleiben verbunden). | 3 | 05 | M | 102 | spezifiziert |
| LTV-011.11 | Zuschauer und Administrator bekommen verständliche Fehlermeldungen ohne Zugangsdaten. | 1a | 14 | T | 097 | spezifiziert |
| LTV-011.12 | Slouchcat protokolliert technische Fehler über das bestehende Logging (Regel 10). | 0; 1a | 12 | T | 098 | spezifiziert |
| LTV-011.13 | Slouchcat meldet dem Administrator längere Ausfälle eines Senders oder einer Quelle (Meldeschwelle … | 3 | 15 | T | 099, 213 | spezifiziert |
| LTV-011.14 | Die Überwachung unterscheidet Stromarten (Video mit Ton, ohne Ton, verschiedene Protokolle). | 3 | 05 | T | 095 | spezifiziert |
| LTV-011.15 | Ein Sender, der regulär keine Tonspur hat, gilt nicht als gestört. | 3 | 05 | T | 095 | spezifiziert |
| LTV-011.16 | Die Wiederherstellung endet nie in einer Endlosschleife. | 1a | 05 | T | 093, 100 | spezifiziert |
| LTV-011.17 | Wiederverbindungen sind begrenzt und werden mit wachsendem Abstand versucht. | 1a | 05 | T | 093, 100 | spezifiziert |
| LTV-011.18 | Verfügbarkeitsprüfungen sind in Zahl und Takt begrenzt. | 3 | 05 | T | 092 | spezifiziert |
| LTV-011.19 | Slouchcat öffnet keine unnötigen parallelen Prüfverbindungen (ein laufender Strom wird an sich … | 3 | 05 | T | 092, 101, 302, 335 | spezifiziert |
| LTV-011.20 | Meldungen an den Administrator sind entprellt (keine Flut für denselben Ausfall). | 3 | 15 | T | 099 | spezifiziert |
| LTV-011.21 | Meldungen laufen über das bestehende Benachrichtigungssystem. | 3 | 15 | T | 099 | spezifiziert |

**Ausschlüsse zu LTV-011** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-011.X-01 | Nicht dabei: ein umfangreiches historisches IPTV-Qualitätsbewertungssystem. | – | S | – | spezifiziert |

### LTV-012 Benutzerrechte (13 Anforderungen, 1 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-012.01 | Die Rechte bleiben bewusst einfach: drei Schalter je Konto und Profil (Abschnitt … | 1a | 07 | S | – | spezifiziert |
| LTV-012.02 | Der Administrator schaltet Live-TV je Konto ein oder aus. | 1a | 15 | T | 160, 223 | spezifiziert |
| LTV-012.03 | Das Hauptprofil schaltet Live-TV je Profil ein oder aus. | 1a | 14 | T | 160, 218 | spezifiziert |
| LTV-012.04 | Administrator (Konto) und Hauptprofil (Profil) erlauben oder verbieten Aufnahmen. | 5 | 14 | T | 160, 223 | spezifiziert |
| LTV-012.05 | Administrator (Konto) und Hauptprofil (Profil) erlauben oder verbieten Timeshift. | 4 | 14 | T | 125, 160, 223 | spezifiziert |
| LTV-012.06 | Live-TV-Wiedergaben zählen zu den bestehenden Grenzen gleichzeitiger Ströme je Konto und Profil … | 1a | 13 | T | 033, 076, 166, 318 | spezifiziert |
| LTV-012.07 | Für Live-TV gelten die bestehenden Zugriffsregeln innen und aussen unverändert; eine neue … | 1a | 12 | T | 033, 167 | spezifiziert |
| LTV-012.08 | Für Umwandlungen gelten die bestehenden Regeln (Budget der Umwandlungen); ein eigenes Umwandlungsrecht … | 1a | 13 | T | 033, 077, 167 | spezifiziert |
| LTV-012.09 | Live-TV hält die bestehende Bandbreitengrenze je Konto und Profil ein. | 1a | 05 | T | 033, 071, 076, 166 | spezifiziert |
| LTV-012.10 | Live-TV hält die bestehende Auflösungsgrenze je Konto und Profil ein. | 1a | 05 | T | 033, 071, 076, 166 | spezifiziert |
| LTV-012.11 | Ein Profil hat nie mehr Rechte als sein Konto; wirksam ist ein … | 1a | 07 | T | 029, 160, 326 | spezifiziert |
| LTV-012.12 | Der Server setzt alle Rechte durch; der Client entscheidet nichts. | 1a | 12 | T | 018, 029, 169 | spezifiziert |
| LTV-012.13 | Nicht freigegebene Inhalte und Funktionen sind auch über direkte API-Aufrufe, Segment- und … | 1a | 12 | T | 018, 028–029, 169, 267 | spezifiziert |

**Ausschlüsse zu LTV-012** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-012.X-01 | Nicht dabei: Rechte je Sender oder je Sendergruppe. | – | S | 184 | spezifiziert |

### LTV-013 Kein Catch-up TV (2 Anforderungen, 6 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-013.01 | Persönliches Timeshift (LTV-007) und gezielte Aufnahmen (LTV-008) bleiben vom Ausschluss unberührt. | K | 02 | S | – | spezifiziert |
| LTV-013.02 | Permanenter Empfang schreibt nichts dauerhaft auf; ausser dem Startpuffer entsteht ohne Zuschauer … | 3 | 05 | T | 005, 171 | spezifiziert |

**Ausschlüsse zu LTV-013** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-013.X-01 | Nicht dabei: Catch-up TV. | – | S | 170 | spezifiziert |
| LTV-013.X-02 | Nicht dabei: Einbindung von TV-Archiven der IPTV-Anbieter. | – | S | 170 | spezifiziert |
| LTV-013.X-03 | Nicht dabei: Wiedergabe verpasster Sendungen aus Anbieterarchiven. | – | S | 170 | spezifiziert |
| LTV-013.X-04 | Nicht dabei: Erzeugen von Catch-up-Adressen. | – | S | 170 | spezifiziert |
| LTV-013.X-05 | Nicht dabei: serverweite rollierende TV-Archive. | 10 | T | 171 | spezifiziert |
| LTV-013.X-06 | Nicht dabei: Daueraufzeichnung aller empfangenen Sender. | 11 | T | 171 | spezifiziert |

### LTV-014 Moderne Benutzeroberfläche (82 Anforderungen, 0 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-014.01 | Live-TV ist ein eigener Bereich im bestehenden Slouchcat-Design, mit den Bauteilen aus … | 1a | 14 | S | 217 | spezifiziert |
| LTV-014.02 | Die Live-TV-Startseite zeigt «Aktuell im Fernsehen». | 2 | 14 | T | 186 | spezifiziert |
| LTV-014.03 | Die Live-TV-Startseite zeigt «Demnächst im Fernsehen». | 2 | 14 | T | 186 | spezifiziert |
| LTV-014.04 | Die Live-TV-Startseite zeigt die zuletzt angesehenen Sender des Profils (DR-15). | 2 | 14 | T | 186 | spezifiziert |
| LTV-014.05 | Die Live-TV-Startseite zeigt die Senderfavoriten des Profils. | 2 | 14 | T | 186 | spezifiziert |
| LTV-014.06 | Die Live-TV-Startseite zeigt Sendungen nach Genre. | 2 | 14 | T | 186, 190 | spezifiziert |
| LTV-014.07 | Die Live-TV-Startseite führt übersichtlich in die Programmzeitschrift. | 2 | 14 | S | – | spezifiziert |
| LTV-014.08 | Die Live-TV-Startseite führt zu «Meine Aufnahmen» (nur mit Recht «Aufnahmen»). | 5 | 14 | T | 186 | spezifiziert |
| LTV-014.09 | Die Live-TV-Startseite führt zu «Geplante Aufnahmen» (nur mit Recht «Aufnahmen»). | 5 | 14 | T | 186 | spezifiziert |
| LTV-014.10 | Die Senderübersicht zeigt Senderlogos (Platzhalter, wenn keines da ist). | 1a | 14 | T | 187 | spezifiziert |
| LTV-014.11 | Die Senderübersicht zeigt Kanalnummern. | 1a | 14 | T | 187 | spezifiziert |
| LTV-014.12 | Die Senderübersicht zeigt Sendernamen. | 1a | 14 | T | 187 | spezifiziert |
| LTV-014.13 | Die Senderübersicht zeigt die laufende Sendung, wenn Programmdaten da sind. | 2 | 14 | T | 110, 187 | spezifiziert |
| LTV-014.14 | Die Senderübersicht zeigt die nächste Sendung, wenn Programmdaten da sind. | 2 | 14 | T | 110, 187 | spezifiziert |
| LTV-014.15 | Die Senderübersicht zeigt den Fortschritt der laufenden Sendung. | 2 | 14 | T | 110, 187 | spezifiziert |
| LTV-014.16 | Die Senderübersicht lässt nach Sendergruppen ordnen und wählen. | 1a | 14 | T | 187 | spezifiziert |
| LTV-014.17 | Die Senderübersicht hat Filter (Gruppe, Land, Sprache, Kategorie, Favoriten). | 1a | 14 | T | 187 | spezifiziert |
| LTV-014.18 | Die Senderübersicht hat eine Suche nach Sendernamen. | 1a | 14 | T | 187 | spezifiziert |
| LTV-014.19 | In der Senderübersicht kann ein Profil Favoriten setzen und entfernen. | 1a | 14 | T | 188 | spezifiziert |
| LTV-014.20 | Die Programmzeitschrift ist ein klassisches Zeitraster. | 2 | 14 | S | – | spezifiziert |
| LTV-014.21 | Sender stehen untereinander. | 2 | 14 | S | – | spezifiziert |
| LTV-014.22 | Die Zeit läuft von links nach rechts. | 2 | 14 | S | – | spezifiziert |
| LTV-014.23 | Eine Linie markiert die aktuelle Zeit. | 2 | 14 | T | 189 | spezifiziert |
| LTV-014.24 | Man kann zwischen Tagen und Uhrzeiten springen. | 2 | 14 | T | 189 | spezifiziert |
| LTV-014.25 | Die Programmzeitschrift zeigt laufende und künftige Sendungen. | 2 | 14 | T | 189, 200 | spezifiziert |
| LTV-014.26 | Eine Sendung öffnet ihre Details. | 2 | 14 | T | 189 | spezifiziert |
| LTV-014.27 | Aus der Programmzeitschrift lässt sich eine Aufnahme planen. | 5 | 14 | T | 131, 189 | spezifiziert |
| LTV-014.28 | Aus der Programmzeitschrift lässt sich eine Serienaufnahme planen. | 6 | 14 | T | 133, 189 | spezifiziert |
| LTV-014.29 | Aus der Programmzeitschrift lässt sich eine Erinnerung setzen. | 2 | 14 | T | 189 | spezifiziert |
| LTV-014.30 | Die Sendungsdetailseite zeigt den Titel. | 2 | 14 | T | 191 | spezifiziert |
| LTV-014.31 | Sie zeigt die Beschreibung, wenn vorhanden. | 2 | 14 | T | 191 | spezifiziert |
| LTV-014.32 | Sie zeigt den Sender. | 2 | 14 | T | 191 | spezifiziert |
| LTV-014.33 | Sie zeigt verfügbare Bilder. | 2 | 14 | T | 191 | spezifiziert |
| LTV-014.34 | Sie zeigt Start- und Endzeit. | 2 | 14 | T | 191 | spezifiziert |
| LTV-014.35 | Sie zeigt die Dauer. | 2 | 14 | T | 191 | spezifiziert |
| LTV-014.36 | Sie zeigt das Genre, wenn vorhanden. | 2 | 14 | T | 191 | spezifiziert |
| LTV-014.37 | Läuft die Sendung gerade, startet sie von hier die Wiedergabe des Senders. | 2 | 14 | T | 191 | spezifiziert |
| LTV-014.38 | Von hier lässt sich eine Aufnahme planen. | 5 | 14 | T | 131, 191 | spezifiziert |
| LTV-014.39 | Von hier lässt sich eine Serienaufnahme planen. | 6 | 14 | T | 133, 191 | spezifiziert |
| LTV-014.40 | Von hier lässt sich eine Erinnerung erstellen. | 2 | 14 | T | 191 | spezifiziert |
| LTV-014.41 | Der Player gibt den Sender wieder. | 1a | 14 | T | 192 | spezifiziert |
| LTV-014.42 | Der Player kann pausieren (mit Recht «Timeshift»; ohne Recht hält der Server … | 4 | 14 | T | 192 | spezifiziert |
| LTV-014.43 | Der Player bedient das persönliche Timeshift. | 4 | 14 | T | 192 | spezifiziert |
| LTV-014.44 | Der Player spult zurück. | 4 | 14 | T | 192 | spezifiziert |
| LTV-014.45 | Der Player spult vor bis Live. | 4 | 14 | T | 192 | spezifiziert |
| LTV-014.46 | Der Player zeigt an, ob er live ist oder wie weit zurück. | 1a | 14 | T | 192 | spezifiziert |
| LTV-014.47 | Der Player springt zurück zum Live-Zeitpunkt. | 4 | 14 | T | 192 | spezifiziert |
| LTV-014.48 | Im Player lässt sich der Sender wechseln (Liste, Nummer, nächster/vorheriger). | 1a | 14 | T | 192 | spezifiziert |
| LTV-014.49 | Der Player zeigt aktuelle und kommende Sendung. | 2 | 14 | T | 192 | spezifiziert |
| LTV-014.50 | Im Player lässt sich eine Aufnahme starten (mit Recht «Aufnahmen»). | 5 | 14 | T | 130, 192 | spezifiziert |
| LTV-014.51 | Im Player lässt sich die Tonspur wählen. | 1a | 14 | T | 192 | spezifiziert |
| LTV-014.52 | Im Player lässt sich die Untertitelspur wählen; Stand je Client und Stufe … | 1a; E | 14 | T | 192, 353 | spezifiziert |
| LTV-014.53 | Im Player lässt sich die Qualität wählen, innerhalb der Grenzen. | 1a | 14 | T | 192 | spezifiziert |
| LTV-014.54 | Der Player hat einen Vollbildmodus. | 1a | 14 | T | 192 | spezifiziert |
| LTV-014.55 | Der Player zeigt verständliche Ladezustände. | 1a | 14 | S | – | spezifiziert |
| LTV-014.56 | Der Player zeigt verständliche Fehlerzustände (Sender gestört, kein Recht mehr, Grenze erreicht). | 1a | 14 | S | – | spezifiziert |
| LTV-014.57 | «Meine Aufnahmen» zeigt die private Aufnahmebibliothek des Profils. | 5 | 14 | T | 139, 193 | spezifiziert |
| LTV-014.58 | Sie zeigt laufende Aufnahmen. | 5 | 14 | T | 139, 193 | spezifiziert |
| LTV-014.59 | Sie zeigt abgeschlossene Aufnahmen. | 5 | 14 | T | 139, 193 | spezifiziert |
| LTV-014.60 | Sie zeigt fehlgeschlagene Aufnahmen. | 5 | 14 | T | 139, 193 | spezifiziert |
| LTV-014.61 | Sie zeigt den Wiedergabefortschritt je Aufnahme. | 5 | 14 | T | 193 | spezifiziert |
| LTV-014.62 | Von hier lässt sich eine Aufnahme abspielen. | 5 | 14 | T | 193, 195 | spezifiziert |
| LTV-014.63 | Von hier lässt sich eine Aufnahme löschen (mit Rückfrage). | 5 | 14 | T | 141, 193 | spezifiziert |
| LTV-014.64 | Sie zeigt je Aufnahme, wie lange sie aufbewahrt wird und warum (Frist, … | 5 | 14 | T | 193 | spezifiziert |
| LTV-014.65 | «Geplante Aufnahmen» zeigt Einzelaufnahmen. | 5 | 14 | T | 139, 196 | spezifiziert |
| LTV-014.66 | Sie zeigt Serienaufnahmen. | 6 | 14 | T | 139, 196 | spezifiziert |
| LTV-014.67 | Sie zeigt wiederkehrende Aufnahmen. | 6 | 14 | T | 139, 196 | spezifiziert |
| LTV-014.68 | Eine Planung lässt sich bearbeiten. | 5 | 14 | T | 196 | spezifiziert |
| LTV-014.69 | Eine Planung lässt sich abbrechen. | 5 | 14 | T | 196 | spezifiziert |
| LTV-014.70 | Konflikte sind sichtbar und erklärt (DR-11). | 5 | 14 | T | 137, 197 | spezifiziert |
| LTV-014.71 | Vor- und Nachlauf sind sichtbar und änderbar. | 5 | 14 | T | 135, 196 | spezifiziert |
| LTV-014.72 | Der Aufnahmezeitpunkt ist sichtbar. | 5 | 14 | T | 196 | spezifiziert |
| LTV-014.73 | Der Zustand jeder Planung ist sichtbar (geplant, läuft, Konflikt, nicht eindeutig, ruht). | 5 | 14 | T | 142, 194, 196 | spezifiziert |
| LTV-014.74 | Die Oberfläche ist modern, ruhig, übersichtlich und konsistent mit dem bestehenden Design. | 1a | 19 | S | – | spezifiziert |
| LTV-014.75 | Live-TV läuft in der Web-App am Desktop. | 1a | 16 | T | 294 | spezifiziert |
| LTV-014.76 | Live-TV läuft auf dem Smartphone (Web-App und Handy-App, F-6). Safari auf dem … | 1a; Apps | 16 | T | 295–296 | spezifiziert |
| LTV-014.77 | Live-TV läuft auf dem Tablet (Web-App und Handy-App, F-6). Safari auf dem … | 1a; Apps | 16 | T | 295–296 | spezifiziert |
| LTV-014.78 | Live-TV läuft in der Android-TV-App (F-6). | Apps | 16 | T | 297 | spezifiziert |
| LTV-014.79 | Live-TV läuft auf Google-TV-Geräten über die Android-TV-App (F-6). | Apps | 16 | T | 297 | spezifiziert |
| LTV-014.80 | Weitere bestehende Clients sind berücksichtigt; geplante weitere Clients gibt es nicht (Apple … | K | 16 | S | – | spezifiziert |
| LTV-014.82 | Slouchcat berücksichtigt Fire TV: Das Konzept prüft, ob die Android-TV-App auf Fire … | K | 16 | S | – | spezifiziert |
| LTV-014.81 | Live-TV erfindet kein neues Designsystem. | 1a | 14 | S | – | spezifiziert |

### LTV-015 Kinderprofile und Jugendschutz (14 Anforderungen, 0 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-015.01 | Slouchcat prüft bei Live-TV kein Alter. | 1a | 12 | T | 019, 179 | spezifiziert |
| LTV-015.02 | Das Hauptprofil entscheidet ausdrücklich, ob ein Profil mit Altersgrenze Live-TV nutzen darf; … | 1a | 14 | T | 018–019, 161, 326 | spezifiziert |
| LTV-015.03 | Das Hauptprofil kann Live-TV für ein Profil mit Altersgrenze freigeben und wieder … | 1a | 14 | T | 019, 161 | spezifiziert |
| LTV-015.04 | Slouchcat prüft keinen Sender auf ein Alter. | 1a | 12 | T | 019, 179 | spezifiziert |
| LTV-015.05 | Slouchcat prüft keine Sendung auf ein Alter, auch wenn die Programmdaten eine … | 2 | 09 | T | 019, 179 | spezifiziert |
| LTV-015.06 | Slouchcat filtert die Programmzeitschrift nicht nach Alter. | 2 | 09 | T | 019, 179 | spezifiziert |
| LTV-015.07 | Slouchcat sperrt nichts dynamisch aufgrund von Programminformationen. | 2 | 09 | T | 019, 179 | spezifiziert |
| LTV-015.08 | Mit Freigabe sieht das Profil alle Sender, die für sein Konto verfügbar … | 1a | 12 | T | 019 | spezifiziert |
| LTV-015.09 | Ohne Freigabe ist der ganze Live-TV-Bereich für das Profil unsichtbar (404). | 1a | 12 | T | 018, 326 | spezifiziert |
| LTV-015.10 | Der Deckel des Administrators je Konto bleibt verbindlich; das Hauptprofil kann nicht … | 1a | 12 | T | 161 | spezifiziert |
| LTV-015.11 | Die Ausnahme vom altersabhängigen Sichtbarkeitsmodell ist als beschlossene Ausnahme dokumentiert (A-2). | 0 | 21 | S | – | spezifiziert |
| LTV-015.12 | Die Ausnahme gilt nicht für Filme, Serien und andere altersgefilterte Bibliotheken; deren … | 1a | 17 | T | 020, 285 | spezifiziert |
| LTV-015.13 | Die Behandlung der Aufnahmen von Profilen mit Altersgrenze, auch in Suche und … | 5 | 11 | T | 163 | spezifiziert |
| LTV-015.14 | Nicht eindeutige Detailregeln zu Aufnahmen von Profilen mit Altersgrenze sind Marco vorgelegt … | K | 20 | S | – | spezifiziert |

### LTV-016 Multi-View und PiP · Klassische Wiedergabe (2 Anforderungen, 3 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-016.01 | Ein Player gibt genau einen Sender wieder. | 1a | 14 | T | 080, 174, 192 | spezifiziert |
| LTV-016.02 | Ein Profil kann auf verschiedenen Geräten zugleich schauen, im Rahmen der Grenze … | 1a | 13 | T | 080, 330 | spezifiziert |

**Ausschlüsse zu LTV-016** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-016.X-01 | Nicht dabei: Multi-View. | – | S | 174 | spezifiziert |
| LTV-016.X-02 | Nicht dabei: Bild-in-Bild. | – | S | 174 | spezifiziert |
| LTV-016.X-03 | Nicht dabei: mehrere Sender gleichzeitig im selben Player. | – | S | 174 | spezifiziert |

### LTV-017 Anbieterverwaltung · Keine separate Anbieterlimitverwaltung (3 Anforderungen, 6 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-017.01 | Die allgemeinen Ressourcenlimits von Slouchcat gelten für Live-TV weiter (DR-11, DR-12). | 1a | 13 | T | 166, 180 | spezifiziert |
| LTV-017.02 | Zugangsdaten authentifizierter Quellen sind verschlüsselt gespeichert und nie in Logs, Antworten, Fehlermeldungen … | 1a | 12 | T | 039, 203, 329 | spezifiziert |
| LTV-017.03 | Die Verwaltung der Quelle weist darauf hin, dass der Administrator für Nutzungsbedingungen … | 1a | 15 | S | – | spezifiziert |

**Ausschlüsse zu LTV-017** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-017.X-01 | Nicht dabei: eine Verwaltung von Verbindungslimits je IPTV-Anbieter, auch kein vom Administrator … | – | S | 180 | spezifiziert |
| LTV-017.X-02 | Nicht dabei: anbieterbezogene Verbindungskontingente. | – | S | 180 | spezifiziert |
| LTV-017.X-03 | Nicht dabei: Abonnementverwaltung. | – | S | 180 | spezifiziert |
| LTV-017.X-04 | Nicht dabei: Abonnementlaufzeiten. | – | S | 180 | spezifiziert |
| LTV-017.X-05 | Nicht dabei: automatische Verteilung auf mehrere Anbieterzugänge. | – | S | 180 | spezifiziert |
| LTV-017.X-06 | Nicht dabei: anbieterbezogene Verbrauchsabrechnung. | – | S | 180 | spezifiziert |

### LTV-018 Automatische Senderaktualisierung (23 Anforderungen, 1 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-018.01 | Slouchcat aktualisiert externe Senderlisten automatisch. | 1b | 09 | T | 046, 051 | spezifiziert |
| LTV-018.02 | Der Administrator stellt das Aktualisierungsintervall je Quelle ein. | 1b | 15 | T | 046 | spezifiziert |
| LTV-018.03 | Der Administrator kann eine Quelle von Hand aktualisieren. | 1a | 15 | T | 046 | spezifiziert |
| LTV-018.04 | Neue Sender einer Quelle übernimmt Slouchcat automatisch (aktiv nach Vorgabe der Quelle, … | 1a | 09 | T | 022, 047 | spezifiziert |
| LTV-018.05 | Sender, die in der Liste fehlen, kennzeichnet Slouchcat als «nicht mehr vorhanden», … | 1a | 09 | T | 022, 047, 325 | spezifiziert |
| LTV-018.06 | Geänderte Stream-Adressen übernimmt Slouchcat für denselben Sender. | 1a | 09 | T | 022, 050, 323 | spezifiziert |
| LTV-018.07 | Von Hand gepflegte Sendernamen bleiben erhalten. | 1a | 09 | T | 022 | spezifiziert |
| LTV-018.08 | Von Hand gepflegte Logos bleiben erhalten. | 1a | 09 | T | 022 | spezifiziert |
| LTV-018.09 | Sendergruppen und Zuordnungen zu Gruppen bleiben erhalten. | 1a | 09 | T | 022 | spezifiziert |
| LTV-018.10 | Kanalnummern bleiben erhalten. | 1a | 09 | T | 022 | spezifiziert |
| LTV-018.11 | Favoriten der Profile bleiben erhalten. | 1a | 09 | T | 022, 323 | spezifiziert |
| LTV-018.12 | EPG-Zuordnungen bleiben erhalten. | 2 | 09 | T | 022–023 | spezifiziert |
| LTV-018.13 | Aufnahmeplanungen bleiben erhalten. | 5 | 09 | T | 022, 323 | spezifiziert |
| LTV-018.14 | Ein fehlerhafter Import zerstört keine funktionierende Senderliste; Slouchcat behält dann den letzten … | 1a | 09 | T | 021, 049, 324 | spezifiziert |
| LTV-018.15 | Der Import nutzt stabile interne Kennungen und schreibt alles oder nichts (transaktional). | 1a | 09 | T | 021, 048–049, 324 | spezifiziert |
| LTV-018.16 | Geänderte Sendernamen in der Quelle führen nicht zu einem neuen Sender, wenn … | 1a | 09 | T | 022, 048, 059, 323 | spezifiziert |
| LTV-018.17 | Geänderte Adressen werden wie LTV-018.06 behandelt; ein aktiver Strom wechselt erst an … | 1a | 06 | T | 022, 048, 050 | spezifiziert |
| LTV-018.18 | Fehlende Sender werden wie LTV-018.05 behandelt. | 1a | 09 | T | 022, 047 | spezifiziert |
| LTV-018.19 | Doppelte Sender innerhalb einer Liste werden erkannt und nicht doppelt angelegt. | 1a | 09 | T | 022, 047 | spezifiziert |
| LTV-018.20 | Ist eine Liste kurz nicht erreichbar, bleibt der bisherige Stand unverändert. | 1a | 09 | T | 021 | spezifiziert |
| LTV-018.21 | Eine fehlerhafte Liste wird verworfen, mit Fehleranzeige in der Verwaltung. | 1a | 09 | T | 021, 204 | spezifiziert |
| LTV-018.22 | Eine auffällig unvollständige Liste (Schwelle in 09-epg.md) entfernt nicht massenhaft Sender, sondern … | 1b | 09 | T | 021 | spezifiziert |
| LTV-018.23 | Von Hand bearbeitete Sender behalten alle Änderungen des Administrators. | 1a | 09 | T | 022, 324 | spezifiziert |

**Ausschlüsse zu LTV-018** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-018.X-01 | Nicht dabei: ein komplexes Freigabe- oder Versionierungssystem für Senderlisten. | – | S | – | spezifiziert |

### LTV-019 Erweiterte Speicherverwaltung (22 Anforderungen, 4 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-019.01 | Der Administrator stellt den Speicherort der technischen Startpuffer ein. | 4 | 15 | T | 150, 216 | spezifiziert |
| LTV-019.02 | Der Administrator stellt den Speicherort der persönlichen Timeshift-Puffer ein. | 4 | 15 | T | 150, 216 | spezifiziert |
| LTV-019.03 | Der Administrator stellt den Speicherort der Aufnahmen ein. | 5 | 15 | T | 150, 216 | spezifiziert |
| LTV-019.04 | Ein Speicherort kann eine RAM-Disk sein. | 4 | 13 | T | 151 | spezifiziert |
| LTV-019.05 | Ein Speicherort kann eine SSD sein. | 4 | 13 | T | 151 | spezifiziert |
| LTV-019.06 | Ein Speicherort kann eine NVMe sein. | 4 | 13 | T | 151 | spezifiziert |
| LTV-019.07 | Ein Speicherort kann eine HDD sein. | 4 | 13 | T | 151 | spezifiziert |
| LTV-019.08 | Ein Speicherort kann ein eingebundener Netzspeicher sein. | 4; 5 | 13 | T | 151 | spezifiziert |
| LTV-019.09 | Ein Speicherort kann jedes bestehende Linux-Dateisystem sein, das der Container sieht. | 4 | 13 | T | 151 | spezifiziert |
| LTV-019.10 | Der Administrator setzt globale Speicherlimits je Speicherort. | 4 | 15 | T | 025, 152, 216 | spezifiziert |
| LTV-019.11 | Der Administrator setzt ein Kontingent für Aufnahmen je Konto. | 5 | 15 | T | 017, 153, 216, 356 | spezifiziert |
| LTV-019.12 | Der Administrator setzt die maximale Timeshift-Dauer (wie LTV-007.10). | 4 | 15 | T | 154 | spezifiziert |
| LTV-019.13 | Der Administrator setzt je Speicherort eine Mindestreserve an freiem Platz, die Slouchcat … | 4 | 15 | T | 025, 152, 216 | spezifiziert |
| LTV-019.14 | Slouchcat wendet Aufbewahrungsfristen automatisch an, so wie das Profil sie gewählt hat … | 5 | 11 | T | 143 | spezifiziert |
| LTV-019.15 | Slouchcat bereinigt automatisch, was flüchtig ist (Puffer) oder nach der Wahl des … | 4 | 11 | T | 143, 156 | spezifiziert |
| LTV-019.16 | Slouchcat warnt den Administrator bei knappem Speicherplatz. | 4 | 15 | T | 025, 155 | spezifiziert |
| LTV-019.17 | Die Verwaltung zeigt eine Speicherübersicht (je Speicherort und je Konto, ohne Titel). | 4 | 15 | S | – | spezifiziert |
| LTV-019.18 | Keine automatische Bereinigung erfasst eine laufende Aufnahme. | 5 | 11 | T | 143 | spezifiziert |
| LTV-019.19 | Nach einem Absturz bereinigt Slouchcat verwaiste temporäre Dateien, aber nie Aufnahmen. | 4 | 11 | T | 156 | spezifiziert |
| LTV-019.20 | Ein voller Datenträger bringt Slouchcat nicht zum Absturz; andere Funktionen laufen weiter. | 4 | 13 | T | 025, 157, 307 | spezifiziert |
| LTV-019.21 | Fällt ein eingebundener Netzspeicher aus, schreibt Slouchcat nicht unbemerkt auf das lokale … | 5 | 11 | T | 032 | spezifiziert |
| LTV-019.22 | Slouchcat prüft Platz, Schreibrecht und Verfügbarkeit jedes Speicherorts beim Einrichten und laufend. | 4 | 11 | T | 032, 158 | spezifiziert |

**Ausschlüsse zu LTV-019** (Negativtest aus `17` 5.12, falls vorhanden):

| ID | Kurztext | Dok | Abn. | Negativtest / AK-L | Status |
|---|---|---|---|---|---|
| LTV-019.X-01 | Nicht dabei: automatische Verteilung auf mehrere Aufnahme-Datenträger. | – | S | 159, 182 | spezifiziert |
| LTV-019.X-02 | Nicht dabei: automatische Archivierung. | – | S | 182 | spezifiziert |
| LTV-019.X-03 | Nicht dabei: automatisches Verschieben älterer Aufnahmen. | – | S | 159, 182 | spezifiziert |
| LTV-019.X-04 | Nicht dabei: nachträgliche Komprimierung. | – | S | 182 | spezifiziert |

### LTV-020 Aufnahmequalität · Aufnahmen in Originalqualität (22 Anforderungen, 0 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-020.01 | Slouchcat speichert Aufnahmen ausschliesslich in Originalqualität. | 5 | 11 | T | 014 | spezifiziert |
| LTV-020.02 | Während der Aufnahme rechnet Slouchcat nichts neu. | 5 | 11 | T | 014, 182 | spezifiziert |
| LTV-020.03 | Die Bildqualität der Quelle bleibt erhalten. | 5 | 11 | T | 014 | spezifiziert |
| LTV-020.04 | Die Tonqualität der Quelle bleibt erhalten. | 5 | 11 | T | 014 | spezifiziert |
| LTV-020.05 | Alle empfangenen Tonspuren bleiben erhalten. | 5 | 11 | T | 015 | spezifiziert |
| LTV-020.06 | Alle empfangenen Untertitelspuren bleiben erhalten. | 5 | 11 | T | 015 | spezifiziert |
| LTV-020.07 | Slouchcat komprimiert Aufnahmen nachträglich nicht. | 5 | 11 | T | 014, 182 | spezifiziert |
| LTV-020.08 | Es gibt keine Auswahl einer reduzierten Aufnahmequalität. | 5 | 14 | S | – | spezifiziert |
| LTV-020.09 | Slouchcat darf verlustfrei umpacken, wenn es technisch nötig ist. | 5 | 11 | T | 014 | spezifiziert |
| LTV-020.10 | Das Konzept vergleicht Containerformate und begründet die Wahl. | K | 11 | S | – | spezifiziert |
| LTV-020.11 | Beschädigte Eingangsströme führen zu einer nutzbaren Aufnahme mit markierten Fehlstellen. | 5 | 11 | T | 027 | spezifiziert |
| LTV-020.12 | Unterbrochene Eingangsströme führen zu einer Aufnahme mit markierter Lücke. | 5 | 11 | T | 027, 145 | spezifiziert |
| LTV-020.13 | Wechselnde Stromparameter (Auflösung, Codec, Spuren) brechen die Aufnahme nicht ab. | 5 | 11 | T | 027 | spezifiziert |
| LTV-020.14 | Ein Wechsel auf eine Ersatzquelle während der Aufnahme bricht sie nicht ab. | 5 | 11 | T | 027 | spezifiziert |
| LTV-020.15 | Unterschiedliche Tonformate werden erhalten. | 5 | 11 | T | 015 | spezifiziert |
| LTV-020.16 | Unterschiedliche Untertitelformate werden erhalten oder ihr Verlust nach LTV-020.21 behandelt. | 5 | 11 | T | 015, 332 | spezifiziert |
| LTV-020.17 | Mehrere Tonspuren werden erhalten. | 5 | 11 | T | 015 | spezifiziert |
| LTV-020.18 | Zeitstempel bleiben stimmig; die Aufnahme ist durchgehend spulbar. | 5 | 11 | T | 144 | spezifiziert |
| LTV-020.19 | Bild und Ton bleiben synchron. | 5 | 11 | T | 144 | spezifiziert |
| LTV-020.20 | Eine unvollständige Aufnahme (Neustart, Absturz) bleibt abspielbar, soweit Daten da sind. | 5 | 11 | T | 027, 144–145 | spezifiziert |
| LTV-020.21 | Kann ein Container eine Quellspur nicht erhalten, dokumentiert das Konzept den Fall … | K | 11 | S | – | spezifiziert |
| LTV-020.22 | Beim Abspielen einer Aufnahme ist eine Umwandlung je Zuschauer weiterhin möglich. | 5 | 05 | T | 140 | spezifiziert |

### LTV-021 Entwicklungsstrategie (15 Anforderungen, 0 Ausschlüsse)

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-021.01 | Live-TV ist vollständig konzipiert, bevor gebaut wird. | K | – | S | 311 | spezifiziert |
| LTV-021.02 | Konzept 28 enthält eine Ist-Analyse (01-bestandsanalyse.md). | K | – | S | 311 | spezifiziert |
| LTV-021.03 | Konzept 28 enthält ein Soll-Konzept (diese Datei und 02-grundsatzentscheide.md). | K | – | S | 311 | spezifiziert |
| LTV-021.04 | Konzept 28 enthält ein Architekturkonzept. | K | 04 | S | 311 | spezifiziert |
| LTV-021.05 | Konzept 28 enthält ein Datenmodell. | K | 07 | S | 311 | spezifiziert |
| LTV-021.06 | Konzept 28 enthält ein API-Konzept. | K | 08 | S | 311 | spezifiziert |
| LTV-021.07 | Konzept 28 enthält ein Sicherheitskonzept. | K | 12 | S | 311 | spezifiziert |
| LTV-021.08 | Konzept 28 enthält ein Performance-Konzept. | K | 13 | S | 311 | spezifiziert |
| LTV-021.09 | Konzept 28 enthält eine Speicher- und Ressourcenplanung. | K | 13 | S | 311 | spezifiziert |
| LTV-021.10 | Konzept 28 enthält ein Testkonzept. | K | 17 | S | 311 | spezifiziert |
| LTV-021.11 | Konzept 28 enthält klickbare Mockups. | K | 19 | S | 311 | spezifiziert |
| LTV-021.12 | Konzept 28 enthält die Änderungen an bestehenden Spezifikationen. | K | 21 | S | 311 | spezifiziert |
| LTV-021.13 | Konzept 28 enthält eine Entwicklungsroadmap. | K | 18 | S | 311 | spezifiziert |
| LTV-021.14 | Konzept 28 enthält Abnahmekriterien. | K | 17 | S | 311 | spezifiziert |
| LTV-021.15 | Vor Marcos ausdrücklicher Baufreigabe einer Stufe entsteht für diese Stufe kein Produktivcode. | K | – | S | 311 | spezifiziert |

---

## 4. Matrix C · Querschnittsanforderungen `LTV-Q-…`

Abschnitte 8 bis 15 und 17 des Auftrags. Je Abschnitt eine Summenzeile (Zahl, Abnahmeart, Stufen, Zahl der mit `AK-L`
belegten), darunter die Zeilen. Abnahme S heisst: Der Nachweis ist das Konzept selbst; die Prüfung liegt bei dieser
Matrix (`17` 9.3, `AK-L-310`).

| Abschnitt | Titel | Anforderungen | davon T / M / S | mit AK-L | ohne AK-L (T oder M) | Status |
|---|---|---|---|---|---|---|
| LTV-Q-08 | Zentrale Streaming-Engine (Auftrag Abschnitt 8) | 47 | 2 / 0 / 45 | 3 | 0 | spezifiziert |
| LTV-Q-09 | Administration (Auftrag Abschnitt 9) | 83 | 75 / 5 / 3 | 80 | 0 | spezifiziert |
| LTV-Q-10 | Sicherheit (Auftrag Abschnitt 10) | 65 | 59 / 0 / 6 | 63 | 0 | spezifiziert |
| LTV-Q-11 | Performance und Ressourcen (Auftrag Abschnitt 11) | 46 | 3 / 0 / 43 | 3 | 0 | spezifiziert |
| LTV-Q-12 | Integration in bestehende Funktionen (Auftrag Abschnitt 12) | 30 | 21 / 0 / 9 | 21 | 0 | spezifiziert |
| LTV-Q-13 | Klickbare Mockups (Auftrag Abschnitt 13) | 71 | 0 / 0 / 71 | 0 | 0 | spezifiziert |
| LTV-Q-14 | Datenmodell und API (Auftrag Abschnitt 14) | 47 | 1 / 0 / 46 | 1 | 0 | spezifiziert |
| LTV-Q-15 | Tests und Abnahmekriterien (Auftrag Abschnitt 15) | 50 | 41 / 5 / 4 | 50 | 0 | spezifiziert |
| LTV-Q-17 | Technische Recherche (Auftrag Abschnitt 17) | 36 | 0 / 0 / 36 | 0 | 0 | spezifiziert |

### LTV-Q-08 Zentrale Streaming-Engine (Auftrag Abschnitt 8)

**Summe:** 47 Anforderungen; mit `AK-L`: 3; Stufen: K (45), 1 (1), 5 (2).

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-Q-08.01 | Das Konzept enthält eine besonders detaillierte Architektur des zentralen Empfangs. | K | 05 | S | – | spezifiziert |
| LTV-Q-08.02 | Es untersucht die Verantwortung «Source Manager» (Quellen). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.03 | Es untersucht «Channel Registry» (Sender und ihre Kennungen). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.04 | Es untersucht «Stream Manager» (Eingangsströme). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.05 | Es untersucht «Stream Lifecycle Manager» (Lebenszyklus). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.06 | Es untersucht «Stream Distributor» (Verteilung). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.07 | Es untersucht «Input Buffer Manager» (Startpuffer). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.08 | Es untersucht «Preload Manager» (Vorladen). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.09 | Es untersucht «Playback Session Manager» (Wiedergabesitzungen). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.10 | Es untersucht «Timeshift Manager». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.11 | Es untersucht «Recording Manager» (Aufnahmen). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.12 | Es untersucht «EPG Manager». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.13 | Es untersucht «Stream Health Monitor» (Überwachung). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.14 | Es untersucht «Resource Manager» (Ressourcen). | K | 04 | S | – | spezifiziert |
| LTV-Q-08.15 | Die Bezeichnungen führen nicht zu eigenen Prozessen, Microservices oder Pflichtmodulen; alles läuft … | K | 04 | S | 185 | spezifiziert |
| LTV-Q-08.16 | Das Konzept prüft je Verantwortung, welche bestehende Komponente sie abdeckt (Relay, Transcoding, … | K | 04 | S | – | spezifiziert |
| LTV-Q-08.17 | Die Architektur trennt die Ebene «externe Eingangsquelle». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.18 | … die Ebene «zentraler Empfang». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.19 | … die Ebene «kurzfristiger technischer Startpuffer». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.20 | … die Ebene «interne Verteilung». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.21 | … die Ebene «individuelle Wiedergabesitzung». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.22 | … die Ebene «individuelle Umwandlung». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.23 | … die Ebene «persönliches Timeshift». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.24 | … die Ebene «Aufnahme». | K | 04 | S | – | spezifiziert |
| LTV-Q-08.25 | Der zentrale Empfang läuft unabhängig von den einzelnen Wiedergabesitzungen. | 1a; 5 | 05 | T | 081 | spezifiziert |
| LTV-Q-08.26 | Eine Aufnahme hängt nie an einem umgewandelten Zuschauerstrom, sondern am Eingangsstrom. | 5 | 05 | T | 081 | spezifiziert |
| LTV-Q-08.27 | Das Konzept begründet die Lage des Timeshift-Puffers in der Kette (Spulen, Umwandlung, … | K | 10 | S | – | spezifiziert |
| LTV-Q-08.28 | Ablaufdiagramm: Start eines noch nicht empfangenen Senders. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.29 | Ablaufdiagramm: Start eines schon empfangenen Senders. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.30 | Ablaufdiagramm: mehrere Zuschauer desselben Senders. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.31 | Ablaufdiagramm: ein Zuschauer braucht eine Umwandlung. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.32 | Ablaufdiagramm: Senderwechsel. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.33 | Ablaufdiagramm: Pause und Timeshift. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.34 | Ablaufdiagramm: Aufnahme bei schon empfangenem Sender. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.35 | Ablaufdiagramm: Aufnahme ohne angemeldeten Benutzer. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.36 | Ablaufdiagramm: Ausfall einer Quelle. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.37 | Ablaufdiagramm: Wechsel auf eine Ersatzquelle. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.38 | Ablaufdiagramm: der letzte Zuschauer beendet. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.39 | Ablaufdiagramm: Neustart von Slouchcat. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.40 | Ablaufdiagramm: Ausfall eines Datenträgers. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.41 | Ablaufdiagramm: erschöpfte Serverressourcen. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.42 | Ablaufdiagramm: Entzug eines Rechts während der Wiedergabe. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.43 | Ablaufdiagramm: M3U-Aktualisierung ändert die Adresse eines aktiven Senders. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.44 | Ablaufdiagramm: ein langsamer Client gefährdet die Verteilung. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.45 | Ablaufdiagramm: mehrere Aufnahmen und Zuschauer am selben Sender. | K | 06 | S | – | spezifiziert |
| LTV-Q-08.46 | Das Konzept beschreibt den vollständigen Lebenszyklus eines Eingangsstroms. | K | 05 | S | – | spezifiziert |
| LTV-Q-08.47 | Der Lebenszyklus unterscheidet permanenten Empfang, Vorladen, aktive Zuschauer, persönliche Puffer und aktive … | K | 05 | S | – | spezifiziert |

### LTV-Q-09 Administration (Auftrag Abschnitt 9)

**Summe:** 83 Anforderungen; mit `AK-L`: 80; Stufen: 1 (52), 2 (8), 3 (8), 4 (7), 5 (8).

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-Q-09.01 | Live-TV ist in die bestehende Verwaltung und ihre Navigation eingefügt (Seitenleiste als … | 1a | 15 | S | – | spezifiziert |
| LTV-Q-09.02 | Das Live-TV-Dashboard zeigt den Status der Live-TV-Funktion. | 1a | 15 | T | 201, 221 | spezifiziert |
| LTV-Q-09.03 | … die Zahl konfigurierter Sender. | 1a | 15 | T | 201 | spezifiziert |
| LTV-Q-09.04 | … die Zahl verfügbarer Sender. | 1a | 15 | T | 201 | spezifiziert |
| LTV-Q-09.05 | … die Zahl nicht verfügbarer Sender. | 1a | 15 | T | 201 | spezifiziert |
| LTV-Q-09.06 | … die aktiven Eingangsströme. | 1a | 15 | T | 201 | spezifiziert |
| LTV-Q-09.07 | … die permanent empfangenen Sender. | 3 | 15 | T | 201 | spezifiziert |
| LTV-Q-09.08 | … die vorgeladenen Sender. | 3 | 15 | T | 201 | spezifiziert |
| LTV-Q-09.09 | … die aktiven Zuschauer (Anzahl). | 1a | 15 | T | 201 | spezifiziert |
| LTV-Q-09.10 | … die aktiven Umwandlungen. | 1a | 15 | T | 201 | spezifiziert |
| LTV-Q-09.11 | … die laufenden Aufnahmen (Anzahl, ohne Titel). | 5 | 15 | T | 201 | spezifiziert |
| LTV-Q-09.12 | … die geplanten Aufnahmen (Anzahl, ohne Titel). | 5 | 15 | T | 201 | spezifiziert |
| LTV-Q-09.13 | … die eingehende Bandbreite. | 1a | 15 | M | 201 | spezifiziert |
| LTV-Q-09.14 | … die ausgehende Bandbreite. | 1a | 15 | M | 201 | spezifiziert |
| LTV-Q-09.15 | … die CPU-Auslastung (neu zu messen, AB-13). | 1a | 13 | M | 201 | spezifiziert |
| LTV-Q-09.16 | … die GPU-Auslastung, soweit die Hardware sie liefert (AB-13). | 1a | 13 | M | 201 | spezifiziert |
| LTV-Q-09.17 | … den RAM-Verbrauch (AB-13). | 1a | 13 | M | 201 | spezifiziert |
| LTV-Q-09.18 | … den Speicherverbrauch der Live-TV-Speicherorte. | 4 | 15 | T | 201 | spezifiziert |
| LTV-Q-09.19 | … aktuelle Fehler und Warnungen. | 1a | 15 | T | 201 | spezifiziert |
| LTV-Q-09.20 | Der Administrator kann Quellen hinzufügen. | 1a | 15 | T | 202 | spezifiziert |
| LTV-Q-09.21 | … Quellen bearbeiten. | 1a | 15 | T | 202 | spezifiziert |
| LTV-Q-09.22 | … Quellen entfernen (mit Rückfrage, welche Sender betroffen sind). | 1a | 15 | T | 060, 202 | spezifiziert |
| LTV-Q-09.23 | … M3U-Dateien importieren. | 1a | 15 | T | 203 | spezifiziert |
| LTV-Q-09.24 | … M3U-Adressen einrichten. | 1a | 15 | T | 203 | spezifiziert |
| LTV-Q-09.25 | … authentifizierte Quellen einrichten. | 1a | 15 | T | 203 | spezifiziert |
| LTV-Q-09.26 | … Xtream-Codes-Quellen einrichten. | 1a | 15 | T | 203 | spezifiziert |
| LTV-Q-09.27 | … einzelne Ströme erfassen. | 1a | 15 | T | 036, 203 | spezifiziert |
| LTV-Q-09.28 | … eine Quelle testen, ohne sie zu speichern. | 1a | 15 | T | 036, 202–203 | spezifiziert |
| LTV-Q-09.29 | … die automatische Aktualisierung einstellen. | 1b | 15 | T | 202–203 | spezifiziert |
| LTV-Q-09.30 | Die Verwaltung zeigt Importfehler verständlich. | 1a | 15 | T | 202, 204 | spezifiziert |
| LTV-Q-09.31 | Die Senderverwaltung lässt Sender bearbeiten. | 1a | 15 | T | 205 | spezifiziert |
| LTV-Q-09.32 | … Sendergruppen verwalten. | 1b | 15 | T | 208 | spezifiziert |
| LTV-Q-09.33 | … Kanalnummern pflegen. | 1b | 15 | T | 205 | spezifiziert |
| LTV-Q-09.34 | … Senderlogos pflegen. | 1b | 15 | T | 205 | spezifiziert |
| LTV-Q-09.35 | … Ersatzquellen pflegen. | 1a | 15 | T | 206 | spezifiziert |
| LTV-Q-09.36 | … die Empfangsstrategie wählen. | 3 | 15 | T | 061, 206, 209 | spezifiziert |
| LTV-Q-09.37 | … Empfangszeitpläne pflegen. | 3 | 15 | T | 209 | spezifiziert |
| LTV-Q-09.38 | … mehrere Sender auf einmal bearbeiten. | 1b | 15 | T | 205 | spezifiziert |
| LTV-Q-09.39 | … Duplikate erkennen und zusammenführen. | 1b | 15 | T | 207 | spezifiziert |
| LTV-Q-09.40 | … die Verfügbarkeit je Sender zeigen. | 1a | 15 | T | 205 | spezifiziert |
| LTV-Q-09.41 | … technische Parameter je Sender zeigen (Codec, Auflösung, Bitrate, Spuren). | 3 | 15 | T | 206 | spezifiziert |
| LTV-Q-09.42 | Die EPG-Verwaltung pflegt EPG-Quellen. | 2 | 15 | T | 104, 210 | spezifiziert |
| LTV-Q-09.43 | … Aktualisierungsintervalle. | 2 | 15 | T | 116, 210 | spezifiziert |
| LTV-Q-09.44 | … Senderzuordnungen. | 2 | 15 | T | 106, 211 | spezifiziert |
| LTV-Q-09.45 | … die automatische Zuordnung. | 2 | 15 | T | 106, 211 | spezifiziert |
| LTV-Q-09.46 | … zeigt fehlende Zuordnungen. | 2 | 15 | T | 106, 211 | spezifiziert |
| LTV-Q-09.47 | … zeigt den Importstatus. | 2 | 15 | T | 210 | spezifiziert |
| LTV-Q-09.48 | … zeigt Fehler. | 2 | 15 | T | 210 | spezifiziert |
| LTV-Q-09.49 | … stellt den Vorschauzeitraum ein. | 2 | 15 | T | 112, 210 | spezifiziert |
| LTV-Q-09.50 | «Aktive Streams» zeigt je Eingangsstrom den Sender. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.51 | … die aktive Quelle (ohne Zugangsdaten). | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.52 | … den Empfangsstatus. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.53 | … die Laufzeit. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.54 | … die Zahl der Zuschauer. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.55 | … die Zahl der Aufnahmeprozesse, ohne Konto, Profil und Titel (DR-17). | 5 | 15 | T | 212, 327 | spezifiziert |
| LTV-Q-09.56 | … die Empfangsbitrate. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.57 | … die Video-Codecs. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.58 | … die Audio-Codecs. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.59 | … die Auflösung. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.60 | … technische Fehler. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.61 | … den Umwandlungszustand. | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.62 | … den Speicherverbrauch (Puffer). | 3 | 15 | T | 212 | spezifiziert |
| LTV-Q-09.63 | Profilbezogene Angaben folgen der bestehenden Regel (W-6) für Wiedergaben; Aufnahmen erscheinen nur … | 1a | 15 | T | 212 | spezifiziert |
| LTV-Q-09.64 | «DVR und Speicher» enthält globale Aufnahme-Einstellungen. | 5 | 15 | T | 214 | spezifiziert |
| LTV-Q-09.65 | … den Standard-Vorlauf. | 5 | 15 | T | 135, 214 | spezifiziert |
| LTV-Q-09.66 | … den Standard-Nachlauf. | 5 | 15 | T | 135, 214 | spezifiziert |
| LTV-Q-09.67 | … Aufbewahrungsfristen (als Vorgabe, F-10). | 5 | 15 | T | 143, 214 | spezifiziert |
| LTV-Q-09.68 | … Speicherpfade. | 4 | 15 | T | 159, 216 | spezifiziert |
| LTV-Q-09.69 | … Speicherlimits. | 4 | 15 | T | 159, 216 | spezifiziert |
| LTV-Q-09.70 | … Kontingente je Konto. | 5 | 15 | T | 153, 216 | spezifiziert |
| LTV-Q-09.71 | … Timeshift-Einstellungen. | 4 | 15 | T | 154, 215 | spezifiziert |
| LTV-Q-09.72 | … verfügbaren Speicherplatz. | 4 | 15 | T | 155, 216 | spezifiziert |
| LTV-Q-09.73 | … Speicherverbrauch. | 4 | 15 | T | 155, 216 | spezifiziert |
| LTV-Q-09.74 | … Warnmeldungen. | 4 | 15 | T | 155, 216 | spezifiziert |
| LTV-Q-09.75 | «Überwachung und Diagnose» zeigt nicht erreichbare Sender. | 1a | 15 | T | 213 | spezifiziert |
| LTV-Q-09.76 | … fehlerhafte Ströme. | 1a | 15 | T | 213 | spezifiziert |
| LTV-Q-09.77 | … Wiederverbindungen. | 1a | 15 | T | 213 | spezifiziert |
| LTV-Q-09.78 | … Wechsel auf Ersatzquellen. | 3 | 15 | T | 213 | spezifiziert |
| LTV-Q-09.79 | … technische Protokolle (gefiltert, ohne Zugangsdaten). | 1a | 15 | T | 213 | spezifiziert |
| LTV-Q-09.80 | … Benachrichtigungen. | 3 | 15 | T | 213 | spezifiziert |
| LTV-Q-09.81 | … Möglichkeiten zur Fehlerdiagnose (zum Beispiel Quelle testen, letzter Fehler je Quelle). | 1a | 15 | T | 213 | spezifiziert |
| LTV-Q-09.82 | Die Live-TV-Verwaltung nutzt die bestehenden Verwaltungsbauteile und ihr Design. | 1a | 15 | S | – | spezifiziert |
| LTV-Q-09.83 | Bestehende Verwaltungsseiten werden nur erweitert, wo es nötig ist (Konten, Profile, Dashboard, … | 1a | 15 | S | – | spezifiziert |

### LTV-Q-10 Sicherheit (Auftrag Abschnitt 10)

**Summe:** 65 Anforderungen; mit `AK-L`: 63; Stufen: K (3), 0 (3), 1 (43), 2 (6), H (3), 4 (6), 5 (2), W (1).

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-Q-10.01 | Das Konzept enthält ein vollständiges Sicherheitskonzept mit Bedrohungsmodell. | K | 12 | S | – | spezifiziert |
| LTV-Q-10.02 | Stream-Adressen können keine serverseitige Anfragefälschung (SSRF, Abruf interner Ziele über den Server) … | 1a | 12 | T | 230 | spezifiziert |
| LTV-Q-10.03 | EPG-Adressen können kein SSRF auslösen. | 2 | 12 | T | 230 | spezifiziert |
| LTV-Q-10.04 | M3U-Adressen können kein SSRF auslösen. | 1a | 12 | T | 230 | spezifiziert |
| LTV-Q-10.05 | Weiterleitungen werden bei jedem Sprung neu geprüft. | 1a | 12 | T | 233 | spezifiziert |
| LTV-Q-10.06 | DNS-Rebinding umgeht die Prüfung nicht. | 1a | 12 | T | 234 | spezifiziert |
| LTV-Q-10.07 | Die Prüfung gilt für IPv4. | 1a | 12 | T | 232 | spezifiziert |
| LTV-Q-10.08 | Die Prüfung gilt für IPv6. | 1a | 12 | T | 232 | spezifiziert |
| LTV-Q-10.09 | Interne Netze sind nur über eine freigegebene Quelle erreichbar (A-3). | H | 12 | T | 045, 230, 235 | spezifiziert |
| LTV-Q-10.10 | Lokale Quellen brauchen eine ausdrückliche Freigabe des Administrators je Quelle als feste … | H | 12 | T | 230 | spezifiziert |
| LTV-Q-10.11 | Risiken von Multicast und UDP sind behandelt (Netzmodus des Containers, fremde Gruppen). | H | 12 | S | 239 | spezifiziert |
| LTV-Q-10.12 | Verschachtelte HLS- und DASH-Manifeste werden in jeder Stufe geprüft. | 1a | 12 | T | 236 | spezifiziert |
| LTV-Q-10.13 | Segment-Adressen werden geprüft. | 1a | 12 | T | 236 | spezifiziert |
| LTV-Q-10.14 | Schlüssel-Adressen werden geprüft. | 1a | 12 | T | 236 | spezifiziert |
| LTV-Q-10.15 | Playlist-Adressen werden geprüft. | 1a | 12 | T | 236 | spezifiziert |
| LTV-Q-10.16 | Externe Senderlogos lädt nur der Server, mit Prüfung. | 1a | 12 | T | 242 | spezifiziert |
| LTV-Q-10.17 | Externe EPG-Bilder lädt nur der Server, mit Prüfung. | 2 | 12 | T | 242 | spezifiziert |
| LTV-Q-10.18 | Zugangsdaten sind verschlüsselt gespeichert. | 1a | 12 | T | 243 | spezifiziert |
| LTV-Q-10.19 | Zugangsdaten in Adressen (Benutzer, Passwort, Pfad, Query) werden erkannt und geschützt. | 0 | 12 | T | 243, 246 | spezifiziert |
| LTV-Q-10.20 | Eigene HTTP-Header gelten als geheim und werden geschützt. | 1a | 12 | T | 244 | spezifiziert |
| LTV-Q-10.21 | Die Anmeldung an Quellen geschieht nur vom Server aus. | 1a | 12 | T | 244 | spezifiziert |
| LTV-Q-10.22 | Zugangsdaten erscheinen nie in Logs (Regel 10 erweitert). | 0 | 12 | T | 246 | spezifiziert |
| LTV-Q-10.23 | Zugangsdaten erscheinen nie in Antworten an Clients. | 1a | 12 | T | 245 | spezifiziert |
| LTV-Q-10.24 | Zugangsdaten erscheinen nie in Fehlermeldungen. | 1a | 12 | T | 245 | spezifiziert |
| LTV-Q-10.25 | Zugangsdaten sind in Sicherungen geschützt. | 1a | 12 | T | 247 | spezifiziert |
| LTV-Q-10.26 | Zugangsdaten gehen nie an andere Ziele als die eingetragene Quelle. | 1a | 12 | T | 244, 249 | spezifiziert |
| LTV-Q-10.27 | Kein Eingabewert kann Befehle einschleusen (Argumentliste, Regel 3). | 1a | 12 | T | 250 | spezifiziert |
| LTV-Q-10.28 | FFmpeg darf nur die nötigen Protokolle verwenden. | 1a | 12 | T | 250 | spezifiziert |
| LTV-Q-10.29 | Bösartige Medienströme gefährden den Server nicht (Grenzen, Abbruch). | 1a | 12 | T | 251, 254 | spezifiziert |
| LTV-Q-10.30 | Fehlerhafte Mediendaten führen zu einem kontrollierten Fehler. | 1a | 12 | T | 254 | spezifiziert |
| LTV-Q-10.31 | Die Zahl externer Prozesse ist begrenzt. | 1a | 13 | T | 253 | spezifiziert |
| LTV-Q-10.32 | Jeder externe Prozess hat Zeitgrenzen. | 1a | 12 | T | 253 | spezifiziert |
| LTV-Q-10.33 | Prozesse werden sicher beendet. | 1a | 12 | T | 253 | spezifiziert |
| LTV-Q-10.34 | Verwaiste Prozesse entstehen nicht oder werden aufgeräumt. | 1a | 12 | T | 253 | spezifiziert |
| LTV-Q-10.35 | Prozesse können die Ressourcen des Servers nicht erschöpfen. | 1a | 13 | T | 251, 253 | spezifiziert |
| LTV-Q-10.36 | Adressen, die FFmpeg oder eine Medienbibliothek selbst abruft, umgehen den SSRF-Schutz nicht. | 1a; W | 12 | T | 237 | spezifiziert |
| LTV-Q-10.37 | XML External Entities werden nicht aufgelöst. | 2 | 12 | T | 257 | spezifiziert |
| LTV-Q-10.38 | XML-Bomben werden abgewehrt. | 2 | 12 | T | 257 | spezifiziert |
| LTV-Q-10.39 | Komprimierte XMLTV-Dateien werden mit Grenzen entpackt. | 2 | 12 | T | 258 | spezifiziert |
| LTV-Q-10.40 | Dekompressionsbomben werden abgewehrt. | 2 | 12 | T | 258 | spezifiziert |
| LTV-Q-10.41 | Fehlerhafte M3U-Dateien führen zu einem kontrollierten Fehler. | 1a | 12 | T | 256 | spezifiziert |
| LTV-Q-10.42 | Uploads haben eine Grössengrenze. | 1a | 12 | T | 256 | spezifiziert |
| LTV-Q-10.43 | Abgerufene Dateien haben eine Grössengrenze. | 1a | 12 | T | 256 | spezifiziert |
| LTV-Q-10.44 | Parser haben Zeitgrenzen. | 1a | 12 | T | 256 | spezifiziert |
| LTV-Q-10.45 | Importe können die Ressourcen des Servers nicht erschöpfen. | 1a | 13 | T | 256 | spezifiziert |
| LTV-Q-10.46 | Kein Puffer ist unbegrenzt. | 1a; 4 | 12 | T | 260 | spezifiziert |
| LTV-Q-10.47 | Pfade können nicht aus den Speicherorten ausbrechen (Regel 4). | 4 | 12 | T | 261 | spezifiziert |
| LTV-Q-10.48 | Dateirechte der Speicherorte sind eng gesetzt. | 4 | 12 | S | 261 | spezifiziert |
| LTV-Q-10.49 | Timeshift-Dateien sind nur für ihre Sitzung abrufbar. | 4 | 12 | T | 263 | spezifiziert |
| LTV-Q-10.50 | Aufnahmen sind nur für Berechtigte abrufbar (Profil, Hauptprofil nach E-3). | 5 | 12 | T | 263 | spezifiziert |
| LTV-Q-10.51 | Datenträgerausfälle sind sicher behandelt (LTV-019.20–.21). | 4 | 12 | T | 262 | spezifiziert |
| LTV-Q-10.52 | Temporäre Dateien werden sicher bereinigt, nur in eigenen Ordnern. | 4 | 12 | T | 261 | spezifiziert |
| LTV-Q-10.53 | Stream-Endpunkte verlangen eine gültige Berechtigung. | 1a | 12 | T | 264 | spezifiziert |
| LTV-Q-10.54 | Segment-Endpunkte verlangen eine gültige Berechtigung. | 1a | 12 | T | 264 | spezifiziert |
| LTV-Q-10.55 | Die Berechtigung wird bei jedem relevanten Zugriff geprüft. | 1a | 12 | T | 265 | spezifiziert |
| LTV-Q-10.56 | Laufende Wiedergabeberechtigungen lassen sich widerrufen (Abschnitt 2.4). | 1a | 12 | T | 265 | spezifiziert |
| LTV-Q-10.57 | Private Profildaten sind geschützt (Favoriten, Verlauf, Erinnerungen). | 1a | 12 | T | 266 | spezifiziert |
| LTV-Q-10.58 | Private Aufnahmebibliotheken sind geschützt. | 5 | 12 | T | 266 | spezifiziert |
| LTV-Q-10.59 | Live-TV nutzt die bestehende Sitzungs- und Token-Architektur, keine zweite Anmeldung. | 1a | 12 | T | 264, 269 | spezifiziert |
| LTV-Q-10.60 | Die bestehenden Kontogrenzen gelten. | 1a | 12 | T | 268 | spezifiziert |
| LTV-Q-10.61 | Alle Freigaben werden im Server durchgesetzt. | 1a | 12 | T | 267 | spezifiziert |
| LTV-Q-10.62 | Unsichtbare Inhalte liefern 404. | 1a | 12 | T | 267 | spezifiziert |
| LTV-Q-10.63 | Alle bestehenden Sicherheitsanforderungen gelten weiter, ausser den benannten Ausnahmen A-1 bis A-13. | 0 | 12 | S | 272 | spezifiziert |
| LTV-Q-10.64 | Kein Sicherheitsmechanismus wird stillschweigend gelockert. | K | 12 | S | 272 | spezifiziert |
| LTV-Q-10.65 | Das Sicherheitskonzept wird unabhängig geprüft. | K | 23 | S | – | spezifiziert |

### LTV-Q-11 Performance und Ressourcen (Auftrag Abschnitt 11)

**Summe:** 46 Anforderungen; mit `AK-L`: 3; Stufen: K (43), 3 (3).

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-Q-11.01 | Das Konzept enthält eine nachvollziehbare Ressourcenplanung für verschiedene Grössen. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.02 | Szenario: 5 permanent empfangene Sender. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.03 | Szenario: 20 permanent empfangene Sender. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.04 | Szenario: 50 permanent empfangene Sender. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.05 | Szenario: 100 permanent empfangene Sender. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.06 | Szenario: 500 importierte Sender. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.07 | Szenario: 5'000 importierte Sender. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.08 | Szenario: 10'000 importierte Sender. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.09 | Szenario: 1 gleichzeitiger Zuschauer. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.10 | Szenario: 5 gleichzeitige Zuschauer. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.11 | Szenario: 20 gleichzeitige Zuschauer. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.12 | Szenario: 50 gleichzeitige Zuschauer. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.13 | Szenario: mehrere Zuschauer desselben Senders. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.14 | Szenario: mehrere Zuschauer verschiedener Sender. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.15 | Szenario: gleichzeitige Aufnahmen. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.16 | Szenario: gleichzeitige individuelle Umwandlungen. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.17 | Die Szenarien sind Analysen, keine zugesicherten Kapazitäten. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.18 | Untersucht wird die Eingangsbandbreite. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.19 | … die Ausgangsbandbreite. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.20 | … die CPU. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.21 | … die GPU. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.22 | … der RAM. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.23 | … der temporäre Speicher. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.24 | … der Timeshift-Speicher. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.25 | … der Aufnahmespeicher. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.26 | … die Zahl der Dateideskriptoren. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.27 | … die Zahl externer Prozesse. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.28 | … die Datenbanklast (SQLite, eine Schreibverbindung). | K | 13 | S | – | spezifiziert |
| LTV-Q-11.29 | … die «Redis-Last»: entfällt, es gibt kein Redis (AB-1); an ihre Stelle … | K | 13 | S | – | spezifiziert |
| LTV-Q-11.30 | … die EPG-Datenmenge. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.31 | … die Dateisystemlast. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.32 | … die Wirkung auf bestehende Medienwiedergaben. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.33 | Das Konzept rechnet Beispiele und empfiehlt Ausstattungen für kleine, mittlere und grössere … | K | 13 | S | – | spezifiziert |
| LTV-Q-11.34 | Die Rechnung trennt empfangene Sender, Zuschauer, Umwandlungen und Aufnahmen. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.35 | Slouchcat hat ein festgelegtes Verhalten bei Überlastung. | 3 | 13 | T | 066, 301, 307 | spezifiziert |
| LTV-Q-11.36 | Bei Überlastung bleiben laufende Wiedergaben und Aufnahmen möglichst geschützt. | 3 | 13 | T | 066, 301, 303, 307 | spezifiziert |
| LTV-Q-11.37 | Bei Mangel reduziert Slouchcat zuerst Vorladen und andere entbehrliche Hintergrundarbeit. | 3 | 13 | T | 066, 301, 307 | spezifiziert |
| LTV-Q-11.38 | Untersucht wird ein voller Datenträger. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.39 | … CPU-Überlast. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.40 | … GPU-Überlast. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.41 | … RAM-Mangel. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.42 | … Bandbreitenmangel. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.43 | … zu viele offene Verbindungen. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.44 | … zu viele FFmpeg-Prozesse. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.45 | … fehlerhafte oder nicht erreichbare Quellen. | K | 13 | S | – | spezifiziert |
| LTV-Q-11.46 | Offene Vorrangkonflikte sind ausdrücklich aufgeschrieben (F-4, 20-offene-entscheidungen.md). | K | 20 | S | – | spezifiziert |

### LTV-Q-12 Integration in bestehende Funktionen (Auftrag Abschnitt 12)

**Summe:** 30 Anforderungen; mit `AK-L`: 21; Stufen: K (1), 0 (1), 1 (21), 2 (2), 5 (3), Apps (2).

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-Q-12.01 | Die Benutzerverwaltung (Konten, Einladungen) trägt die drei Schalter je Konto. | 1a | 15 | T | 160, 223 | spezifiziert |
| LTV-Q-12.02 | Die Profilverwaltung trägt die drei Schalter je Profil. | 1a | 14 | T | 160, 218 | spezifiziert |
| LTV-Q-12.03 | Profile mit Altersgrenze folgen LTV-015 und DR-01 bis DR-05. | 1a; 5 | 14 | T | 146 | spezifiziert |
| LTV-Q-12.04 | Die Verwaltung des Hauptprofils zeigt Freigaben und die Aufnahmen der Profile mit … | 5 | 14 | T | 163 | spezifiziert |
| LTV-Q-12.05 | Der Live-TV-Player baut auf dem bestehenden Player auf. | 1a | 14 | S | – | spezifiziert |
| LTV-Q-12.06 | Umwandlungen laufen über die bestehende Transcoding-Kette, kein zweites System. | 1a | 04 | S | – | spezifiziert |
| LTV-Q-12.07 | Die bestehende Hardwarebeschleunigung wird genutzt. | 1a | 05 | T | 070 | spezifiziert |
| LTV-Q-12.08 | Live-Wiedergaben fügen sich in die bestehenden Wiedergabesitzungen ein (Zählung, Stopp, Widerruf). | 1a | 04 | T | 168, 283 | spezifiziert |
| LTV-Q-12.09 | Die Geräteverwaltung zeigt und beendet auch Live-Wiedergaben eines Geräts. | 1a | 15 | T | 222, 289 | spezifiziert |
| LTV-Q-12.10 | Senderfavoriten folgen dem Muster der Radio-Favoriten (AB-12). | 1a | 07 | T | 284 | spezifiziert |
| LTV-Q-12.11 | Die Programmsuche fügt sich in die Suche nach F-5 ein. | 2 | 09 | T | 111 | spezifiziert |
| LTV-Q-12.12 | Meldungen nutzen das bestehende Benachrichtigungssystem mit neuen Ereignisarten (AB-15, F-3). | 2 | 08 | T | 099, 114, 219, 224, 290 | spezifiziert |
| LTV-Q-12.13 | Import, Abgleich, Prüfung und Aufräumen laufen im bestehenden Jobsystem (AB-16). | 1a | 04 | T | 046, 224 | spezifiziert |
| LTV-Q-12.14 | Die Systemüberwachung zeigt Live-TV-Werte (AB-13). | 1a | 15 | S | – | spezifiziert |
| LTV-Q-12.15 | Live-TV protokolliert über das bestehende Logging mit Redaktion (Regel 10). | 0 | 12 | T | 098 | spezifiziert |
| LTV-Q-12.16 | Die Statistiken zählen Live-TV-Schauzeit getrennt von Titeln (DR-12). | 1a | 15 | T | 168 | spezifiziert |
| LTV-Q-12.17 | Sicherung und Wiederherstellung umfassen die Live-TV-Daten in der Datenbank, nicht die Aufnahmedateien; … | 5 | 07 | T | 148, 227, 276 | spezifiziert |
| LTV-Q-12.18 | Der «Installer» (Compose-Dateien und Anleitung) beschreibt Speicherorte und Netz für Live-TV. | 1a | 21 | S | – | spezifiziert |
| LTV-Q-12.19 | Der Einrichtungsassistent bleibt unverändert; Live-TV wird in der Verwaltung eingerichtet (Annahme). | 1a | 15 | S | – | spezifiziert |
| LTV-Q-12.20 | Live-TV läuft im bestehenden Docker-Container, ohne zweiten Container. | 1a | 04 | T | 185, 281 | spezifiziert |
| LTV-Q-12.X-01 | Nicht dabei: native Linux-Installation (.deb), Nicht-Ziel des Repos (AB-2). | – | 02 | S | – | spezifiziert |
| LTV-Q-12.21 | Alle neuen Operationen stehen im Vertrag api/openapi.yaml mit x-visibility und x-clients. | 1a | 08 | T | 267 | spezifiziert |
| LTV-Q-12.22 | Alle neuen Texte stehen in den Locales (Namensraum live, de, en, de-DE). | 1a | 14 | T | 228 | spezifiziert |
| LTV-Q-12.23 | Die Web-App erhält den Live-TV-Bereich. | 1a | 14 | T | 294 | spezifiziert |
| LTV-Q-12.24 | Die Android-Handy-App erhält den Live-TV-Bereich (F-6). | Apps | 16 | T | 296 | spezifiziert |
| LTV-Q-12.25 | Die Android-TV-App erhält den Live-TV-Bereich (F-6). | Apps | 16 | T | 297 | spezifiziert |
| LTV-Q-12.26 | Weitere geplante Clients: keine (AB-3). | K | 16 | S | – | spezifiziert |
| LTV-Q-12.27 | Keine bestehende Funktion, Einstellung oder Option geht durch Live-TV verloren. | 1a | 17 | T | 282, 287, 291–292 | spezifiziert |
| LTV-Q-12.28 | Live-TV schafft keine zweite Anmeldung, keine zweite Benutzerverwaltung und kein zweites Transcoding-System. | 1a | 04 | S | – | spezifiziert |
| LTV-Q-12.29 | Live-TV folgt der bestehenden Versionierung (Vertrag = Tag) und der Stufen-Systematik. | 1a | 18 | S | – | spezifiziert |

### LTV-Q-13 Klickbare Mockups (Auftrag Abschnitt 13)

**Summe:** 71 Anforderungen; mit `AK-L`: 0; Stufen: K (71).

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-Q-13.01 | Alle neuen und geänderten Seiten sind vor dem Bau als Mockup anschaubar … | K | 19 | S | – | spezifiziert |
| LTV-Q-13.02 | Die Mockups sind voll klickbar und haben realistische Beispieldaten. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.03 | Die Mockups nutzen das aktuelle Design und die bestehenden Bauteile. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.04 | Mockup: Live-TV-Startseite. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.05 | Mockup: Senderübersicht. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.06 | Mockup: Sendergruppen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.07 | Mockup: Favoriten. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.08 | Mockup: Programmzeitschrift. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.09 | Mockup: Sendungsdetails. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.10 | Mockup: Live-TV-Player. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.11 | Mockup: Senderwechsel im Player. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.12 | Mockup: Timeshift-Bedienung. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.13 | Mockup: Sofortaufnahme. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.14 | Mockup: geplante Aufnahme. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.15 | Mockup: Serienaufnahme. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.16 | Mockup: Meine Aufnahmen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.17 | Mockup: Geplante Aufnahmen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.18 | Mockup: Aufnahmekonflikte. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.19 | Mockup: Sendungserinnerungen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.20 | Mockup: leere Zustände. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.21 | Mockup: Ladezustände. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.22 | Mockup: Fehlerzustände. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.23 | Mockup Verwaltung: Live-TV-Dashboard. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.24 | … Quellenverwaltung. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.25 | … M3U-Import. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.26 | … Xtream-Quellenkonfiguration. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.27 | … manuelle Streamkonfiguration. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.28 | … Senderverwaltung. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.29 | … Sendergruppen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.30 | … Senderbearbeitung. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.31 | … Ersatzquellen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.32 | … Empfangsstrategien. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.33 | … Empfangszeitpläne. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.34 | … EPG-Verwaltung. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.35 | … EPG-Zuordnungen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.36 | … aktive Streams. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.37 | … Streamüberwachung. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.38 | … DVR-Einstellungen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.39 | … Timeshift-Einstellungen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.40 | … Speicherverwaltung. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.41 | … Fehlerdiagnose. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.42 | Mockup bestehender Seite: Hauptnavigation. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.43 | … Benutzerberechtigungen (Konten). | K | 19 | S | – | spezifiziert |
| LTV-Q-13.44 | … Profilverwaltung. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.45 | … Einstellungen von Profilen mit Altersgrenze (in der Profilverwaltung, AB-18). | K | 19 | S | – | spezifiziert |
| LTV-Q-13.46 | … Serverübersicht (Dashboard). | K | 19 | S | – | spezifiziert |
| LTV-Q-13.47 | … aktive Wiedergaben. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.48 | … weitere tatsächlich betroffene Seiten (zum Beispiel «Meine Medien», Benachrichtigungseinstellungen). | K | 19 | S | – | spezifiziert |
| LTV-Q-13.49 | Die Mockups sind realistisch, ganz navigierbar und tatsächlich bedienbar. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.50 | Im Mockup funktionieren Knöpfe. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.51 | … Dialoge. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.52 | … Menüs. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.53 | … Untermenüs. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.54 | … Filter. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.55 | … Reiter. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.56 | … Suchfelder. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.57 | … Eingabefelder. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.58 | … Auswahlfelder. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.59 | … Kontextmenüs, wo nötig. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.60 | … die wesentlichen Bedienabläufe. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.61 | Senderlogos und Bilder: rechtmässig verwendbar; im Repo nur neutrale Platzhalter, echte Logos … | K | 19 | S | – | spezifiziert |
| LTV-Q-13.62 | Keine rein statischen Screenshots. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.63 | Keine unvollständigen Wireframes. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.64 | Keine Einzelseiten ohne zusammenhängende Navigation. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.65 | Keine produktiven Zugangsdaten und keine vertraulichen Stream-Adressen. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.66 | Die Mockups brauchen keine echte Streaming-Engine. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.67 | Simulierte Wiedergabe und Beispieldaten sind als solche erkennbar. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.68 | Die Mockups liegen getrennt von der App (docs/planning/mockups/28-live-tv/). | K | 19 | S | – | spezifiziert |
| LTV-Q-13.69 | Die Mockups lassen sich ohne Anpassung öffnen und ausprobieren. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.70 | Aufruf und Start sind dokumentiert. | K | 19 | S | – | spezifiziert |
| LTV-Q-13.71 | Die Mockups werden unabhängig gegen die Anforderungen geprüft. | K | 23 | S | – | spezifiziert |

### LTV-Q-14 Datenmodell und API (Auftrag Abschnitt 14)

**Summe:** 47 Anforderungen; mit `AK-L`: 1; Stufen: K (45), 1 (2).

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-Q-14.01 | Das Konzept enthält ein vollständiges Datenmodell aller Entitäten. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.02 | Entität: Streamquellen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.03 | Entität: Sender. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.04 | Entität: alternative Senderquellen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.05 | Entität: Sendergruppen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.06 | Entität: Senderzuordnungen (Gruppen, EPG). | K | 07 | S | – | spezifiziert |
| LTV-Q-14.07 | Entität: EPG-Quellen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.08 | Entität: EPG-Programme (Sendungen). | K | 07 | S | – | spezifiziert |
| LTV-Q-14.09 | Entität: Empfangsstrategien. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.10 | Entität: Empfangszeitpläne. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.11 | Entität: aktive Eingangsströme. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.12 | Entität: Wiedergabesitzungen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.13 | Entität: Timeshift-Sitzungen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.14 | Entität: Aufnahmen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.15 | Entität: Aufnahmeplanungen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.16 | Entität: Serienaufnahme-Regeln. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.17 | Entität: Speicherlimits. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.18 | Entität: Favoriten. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.19 | Entität: Sendungserinnerungen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.20 | Entität: technische Streamzustände. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.21 | Entität: Berechtigungen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.22 | Das Datenmodell trennt Dauerhaftes (in SQLite, AB-1) von flüchtigem Laufzeitzustand. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.23 | Es berücksichtigt stabile Kennungen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.24 | … referenzielle Integrität. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.25 | … Indizes. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.26 | … Transaktionen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.27 | … Nebenläufigkeit (eine Schreibverbindung). | K | 07 | S | – | spezifiziert |
| LTV-Q-14.28 | … Datenbereinigung. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.29 | … Aufbewahrungsfristen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.30 | … Löschvorgänge. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.31 | … Zuordnung zu Profil und Konto. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.32 | Das Konzept enthält ein Entity-Relationship-Diagramm. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.33 | In der Konzeptphase entstehen keine Migrationen. | K | 07 | S | – | spezifiziert |
| LTV-Q-14.34 | Das Konzept enthält ein vollständiges API-Konzept. | K | 08 | S | – | spezifiziert |
| LTV-Q-14.35 | … mit Endpunkten. | K | 08 | S | – | spezifiziert |
| LTV-Q-14.36 | … Request-Modellen. | K | 08 | S | – | spezifiziert |
| LTV-Q-14.37 | … Response-Modellen. | K | 08 | S | – | spezifiziert |
| LTV-Q-14.38 | … Berechtigungen (x-visibility, x-clients). | K | 08 | S | – | spezifiziert |
| LTV-Q-14.39 | … Fehlerzuständen (RFC 9457, 404 statt 403). | K | 08 | S | – | spezifiziert |
| LTV-Q-14.40 | … seitenweiser Abfrage (Pagination). | K | 08 | S | – | spezifiziert |
| LTV-Q-14.41 | … Filtern. | K | 08 | S | – | spezifiziert |
| LTV-Q-14.42 | … Streaming-Endpunkten (http.Handler, Regel 6). | K | 08 | S | – | spezifiziert |
| LTV-Q-14.43 | … Statusaktualisierungen. | K | 08 | S | – | spezifiziert |
| LTV-Q-14.44 | … Client-Kompatibilität (VERTRAEGLICHKEIT.md). | K | 08 | S | – | spezifiziert |
| LTV-Q-14.45 | … API-Versionierung (Vertrag = Tag). | K | 08 | S | – | spezifiziert |
| LTV-Q-14.46 | Die API folgt den bestehenden API- und OpenAPI-Konventionen. | 1a | 08 | T | 267 | spezifiziert |
| LTV-Q-14.47 | Es gibt keine zweite API-Architektur. | 1a | 08 | S | – | spezifiziert |

### LTV-Q-15 Tests und Abnahmekriterien (Auftrag Abschnitt 15)

**Summe:** 50 Anforderungen; mit `AK-L`: 50; Stufen: K (2), 1 (27), 2 (1), 3 (7), 4 (4), 5 (9).

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-Q-15.01 | Das Konzept enthält einen vollständigen Test- und Abnahmeplan. | K | 17 | S | 310–311 | spezifiziert |
| LTV-Q-15.02 | Test: Mehrere Zuschauer desselben Senders nutzen nachweislich denselben Eingangsstrom. | 1a | 17 | T | 001 | spezifiziert |
| LTV-Q-15.03 | Test: Verschiedene Zuschauer bekommen bei Bedarf eigene Umwandlungen. | 1a | 17 | T | 002 | spezifiziert |
| LTV-Q-15.04 | Test: Ein Senderwechsel stört andere Zuschauer nicht. | 1a | 17 | T | 003 | spezifiziert |
| LTV-Q-15.05 | Test: Beendet ein Zuschauer, laufen die anderen weiter. | 1a | 17 | T | 004 | spezifiziert |
| LTV-Q-15.06 | Test: Permanente Sender bleiben ohne Zuschauer aktiv. | 3 | 17 | T | 005 | spezifiziert |
| LTV-Q-15.07 | Test: Nicht mehr gebrauchte dynamische Ströme enden zuverlässig. | 1a | 17 | T | 006 | spezifiziert |
| LTV-Q-15.08 | Test: Zeitgesteuerter Empfang funktioniert. | 3 | 17 | T | 007 | spezifiziert |
| LTV-Q-15.09 | Test: Vorladen bleibt innerhalb seiner Grenzen. | 3 | 17 | T | 008 | spezifiziert |
| LTV-Q-15.10 | Test: Timeshift-Puffer bleiben strikt getrennt. | 4 | 17 | T | 009 | spezifiziert |
| LTV-Q-15.11 | Test: Timeshift-Puffer werden beim Senderwechsel gelöscht. | 4 | 17 | T | 010 | spezifiziert |
| LTV-Q-15.12 | Test: Niemand greift auf einen fremden Timeshift-Puffer zu. | 4 | 17 | T | 011 | spezifiziert |
| LTV-Q-15.13 | Test: Geplante Aufnahmen laufen ohne angemeldeten Benutzer. | 5 | 17 | T | 012 | spezifiziert |
| LTV-Q-15.14 | Test: Aufnahmen nutzen möglichst bestehende Eingangsströme. | 5 | 17 | T | 013 | spezifiziert |
| LTV-Q-15.15 | Test: Aufnahmen sind in Originalqualität gespeichert. | 5 | 17 | T | 014 | spezifiziert |
| LTV-Q-15.16 | Test: Ton- und Untertitelspuren bleiben erhalten. | 5 | 17 | T | 015 | spezifiziert |
| LTV-Q-15.17 | Test: Private Aufnahmen sind für andere Profile unsichtbar (ausser Hauptprofil nach E-3). | 5 | 17 | T | 016 | spezifiziert |
| LTV-Q-15.18 | Test: Kontingente je Konto werden eingehalten. | 5 | 17 | T | 017 | spezifiziert |
| LTV-Q-15.19 | Test: Profile mit Altersgrenze ohne Freigabe bekommen keine Live-TV-Inhalte. | 1a | 17 | T | 018 | spezifiziert |
| LTV-Q-15.20 | Test: Freigaben für Profile mit Altersgrenze wirken ohne Altersfilter. | 1a | 17 | T | 019 | spezifiziert |
| LTV-Q-15.21 | Test: Der Jugendschutz für andere Medienarten bleibt unverändert. | 1a | 17 | T | 020 | spezifiziert |
| LTV-Q-15.22 | Test: Fehlerhafte M3U-Importe beschädigen keine vorhandenen Sender. | 1a | 17 | T | 021 | spezifiziert |
| LTV-Q-15.23 | Test: Senderaktualisierungen erhalten Favoriten und Aufnahmeplanungen. | 5 | 17 | T | 022 | spezifiziert |
| LTV-Q-15.24 | Test: EPG-Aktualisierungen funktionieren zuverlässig. | 2 | 17 | T | 023 | spezifiziert |
| LTV-Q-15.25 | Test: Ersatzquellen greifen bei Ausfällen. | 3 | 17 | T | 024 | spezifiziert |
| LTV-Q-15.26 | Test: Speicherlimits werden eingehalten. | 4 | 17 | T | 025 | spezifiziert |
| LTV-Q-15.27 | Test: Neustarts hinterlassen keine unkontrollierten Prozesse. | 1a | 17 | T | 026 | spezifiziert |
| LTV-Q-15.28 | Test: Aufnahmeunterbrechungen werden richtig erkannt. | 5 | 17 | T | 027 | spezifiziert |
| LTV-Q-15.29 | Test: Unberechtigte Stromzugriffe werden verhindert. | 1a | 17 | T | 028 | spezifiziert |
| LTV-Q-15.30 | Test: Entzogene Rechte setzt der Server durch. | 1a | 17 | T | 029 | spezifiziert |
| LTV-Q-15.31 | Test: FFmpeg-Prozesse enden zuverlässig. | 1a | 17 | T | 030 | spezifiziert |
| LTV-Q-15.32 | Test: Verschachtelte Stream-Adressen umgehen die Sicherheit nicht. | 1a | 17 | T | 031 | spezifiziert |
| LTV-Q-15.33 | Test: Ein ausgefallener Netzspeicher führt nicht zu lokalen Schreibvorgängen. | 5 | 17 | T | 032 | spezifiziert |
| LTV-Q-15.34 | Test: Live-TV beeinträchtigt bestehende Medienfunktionen nicht unzulässig. | 1a | 17 | M | 033 | spezifiziert |
| LTV-Q-15.35 | Testart: Unit-Tests. | 1a | 17 | T | 309 | spezifiziert |
| LTV-Q-15.36 | Testart: Integrationstests. | 1a | 17 | T | 309 | spezifiziert |
| LTV-Q-15.37 | Testart: End-to-End-Tests (Playwright-Tastaturtest je Web-Seite, XScreenTest je Handy-Seite, Regel 8). | 1a | 17 | T | 309 | spezifiziert |
| LTV-Q-15.38 | Testart: API-Vertragstests. | 1a | 17 | T | 309 | spezifiziert |
| LTV-Q-15.39 | Testart: Lasttests. | 1a | 17 | M | 309 | spezifiziert |
| LTV-Q-15.40 | Testart: Sicherheitstests. | 1a | 17 | T | 309 | spezifiziert |
| LTV-Q-15.41 | Testart: Langzeittests. | 3 | 17 | M | 305–306, 309 | spezifiziert |
| LTV-Q-15.42 | Testart: Neustarttests. | 1a | 17 | T | 309 | spezifiziert |
| LTV-Q-15.43 | Testart: Ausfalltests. | 1a | 17 | T | 309 | spezifiziert |
| LTV-Q-15.44 | Testart: Performance-Benchmarks. | 3 | 17 | M | 309 | spezifiziert |
| LTV-Q-15.45 | Testart: Client-Kompatibilitätstests. | 1a | 17 | T | 309 | spezifiziert |
| LTV-Q-15.46 | Testart: Regressionstests. | 1a | 17 | T | 309 | spezifiziert |
| LTV-Q-15.47 | Tests nutzen kontrollierte, reproduzierbare, rechtmässig verwendbare Teststreams (selbst erzeugt). | 1a | 17 | S | 309 | spezifiziert |
| LTV-Q-15.48 | Ein Langzeittest zeigt, dass permanenter Empfang mehrere Tage stabil läuft, ohne Ressourcenlecks. | 3 | 17 | M | 305–306 | spezifiziert |
| LTV-Q-15.49 | Eine Anforderung gilt erst als erfüllt, wenn alle Teile gebaut und durch … | 1a | 22 | S | 310 | spezifiziert |
| LTV-Q-15.50 | In der Konzeptphase heisst jede Anforderung «spezifiziert», nicht «implementiert». | K | 22 | S | 310 | spezifiziert |

### LTV-Q-17 Technische Recherche (Auftrag Abschnitt 17)

**Summe:** 36 Anforderungen; mit `AK-L`: 0; Stufen: K (36).

| ID | Kurztext | Stufe | Dok | Abn. | AK-L | Status |
|---|---|---|---|---|---|---|
| LTV-Q-17.01 | Die Recherche nutzt Research- und Architect-Agents. | K | 23 | S | – | spezifiziert |
| LTV-Q-17.02 | Recherchethema: zentraler Empfang. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.03 | … interne Verteilung. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.04 | … FFmpeg-Prozessführung. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.05 | … Fast Zapping. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.06 | … HLS. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.07 | … Low-Latency HLS. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.08 | … MPEG-TS. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.09 | … MPEG-DASH. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.10 | … RTSP. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.11 | … RTMP. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.12 | … UDP/RTP. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.13 | … Multicast. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.14 | … Browser-Kompatibilität. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.15 | … Android-Kompatibilität. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.16 | … TV-Client-Kompatibilität. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.17 | … persönliches Timeshift. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.18 | … Aufnahmen. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.19 | … EPG-Import. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.20 | … Speicherung grosser Programmdatenmengen. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.21 | … Streamüberwachung. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.22 | … automatische Wiederherstellung. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.23 | Plex, Emby, Jellyfin, TVHeadend und andere dürfen als Vergleich dienen. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.24 | Keine Architektur wird unkritisch übernommen; Abweichungen sind begründet. | K | 04 | S | – | spezifiziert |
| LTV-Q-17.25 | Je vorgeschlagener Bibliothek: technische Eignung. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.26 | … Wartungszustand. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.27 | … Sicherheitsrisiken. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.28 | … Lizenzbedingungen. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.29 | … Verträglichkeit mit der Slouchcat-Lizenz (AGPL-3.0). | K | 24 | S | – | spezifiziert |
| LTV-Q-17.30 | … Wirkung auf das Deployment. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.31 | … Ressourcenbedarf. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.32 | … Plattformverträglichkeit. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.33 | Quellen sind vorzugsweise offizielle Dokumentationen und Primärquellen. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.34 | Quellen und Erkenntnisse sind dokumentiert. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.35 | Belegte Fakten, Annahmen, Empfehlungen und Hypothesen sind getrennt gekennzeichnet. | K | 24 | S | – | spezifiziert |
| LTV-Q-17.36 | Kein Performance-Ziel gilt als erreicht ohne Messung. | K | 13 | S | – | spezifiziert |

---

## 5. Matrix D · Ausschlussliste (Auftrag Abschnitt 16)

Spalte «Festgehalten in 03»: Zeile der Ausschlussliste (`03` 7.1) und die Ausschlüsse im Entscheid (Spalte «Auch in»).
«Negativtest»: `AK-L` aus `17` 5.12 (und weitere), die die Zeile `LTV-X-nn` nennen, sonst die `AK-L` der Ausschlüsse in «Auch in».
«Mockup-Review»: ob `anhang/reviews/review-3-mockups.md` Abschnitt 4 den Punkt ausdrücklich prüft. Diese Zuordnung zu den elf
Zeilen der Review-Tabelle hat das Skript über Stichwörter vergeben; `03` und `19` halten sie nicht fest.

| ID | Ausgeschlossen | Festgehalten in 03 | Negativtest (AK-L) | Mockup-Review (review-3 Abschn. 4) | Status |
|---|---|---|---|---|---|
| LTV-X-01 | Catch-up TV | 03 7.1 Z. 1761; auch in LTV-013.X-01 | 170 | ja (1 Catch-up, Archiv) | spezifiziert |
| LTV-X-02 | Serverweite TV-Archive | 03 7.1 Z. 1762; auch in LTV-013.X-05 | 171 | ja (1 Catch-up, Archiv) | spezifiziert |
| LTV-X-03 | Gemeinsame Timeshift-Puffer | 03 7.1 Z. 1763; auch in LTV-007.X-01 | 172 | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-04 | Gemeinsam sichtbare Aufnahmebibliotheken (die Einsicht des Hauptprofils nach E-3 ist keine gemeinsame Bibliothek) | 03 7.1 Z. 1764; auch in LTV-009.07 | 163, 173 | ja (4 Geteilte Aufnahmen) | spezifiziert |
| LTV-X-05 | Multi-View | 03 7.1 Z. 1765; auch in LTV-016.X-01 | 174 | ja (2 Bild-im-Bild, Mehrfachbild) | spezifiziert |
| LTV-X-06 | Bild-in-Bild | 03 7.1 Z. 1766; auch in LTV-016.X-02 | 174 | ja (2 Bild-im-Bild, Mehrfachbild) | spezifiziert |
| LTV-X-07 | Physische TV-Tuner | 03 7.1 Z. 1767; auch in LTV-003.X-01 | 175 | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-08 | SAT>IP | 03 7.1 Z. 1768; auch in LTV-003.X-03 | (nur über «Auch in»: 175) | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-09 | DVB-Empfang | 03 7.1 Z. 1769; auch in LTV-003.X-05 bis X-07 | (nur über «Auch in»: 175) | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-10 | HDHomeRun | 03 7.1 Z. 1770; auch in LTV-003.X-02 | (nur über «Auch in»: 175) | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-11 | TVHeadend | 03 7.1 Z. 1771; auch in LTV-003.X-04 | (nur über «Auch in»: 175) | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-12 | DRM-Umgehung | 03 7.1 Z. 1772; auch in LTV-003.X-09 | 176 | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-13 | Xtream-VOD-Bibliotheken | 03 7.1 Z. 1773; auch in LTV-003.X-10 | 177 | ja (7 VOD und Serien aus Xtream) | spezifiziert |
| LTV-X-14 | Xtream-Serienbibliotheken | 03 7.1 Z. 1774; auch in LTV-003.X-11 | 177 | ja (7 VOD und Serien aus Xtream) | spezifiziert |
| LTV-X-15 | Automatische Werbeerkennung | 03 7.1 Z. 1775; auch in LTV-008.X-01 | 178 | ja (8 Werbeerkennung, KI, Suchwort) | spezifiziert |
| LTV-X-16 | KI-Funktionen | 03 7.1 Z. 1776; auch in LTV-005.X-02, LTV-008.X-02 | 178 | ja (8 Werbeerkennung, KI, Suchwort) | spezifiziert |
| LTV-X-17 | Automatische Altersprüfung von Live-TV-Sendungen | 03 7.1 Z. 1777; auch in LTV-015.05 | 179 | ja (3 Altersfilter) | spezifiziert |
| LTV-X-18 | Anbieterbezogene Abonnementverwaltung | 03 7.1 Z. 1778; auch in LTV-017.X-03 | 180 | ja (6 Verbindungslimits, Abo-Verwaltung) | spezifiziert |
| LTV-X-19 | Separate IPTV-Anbieterlimits | 03 7.1 Z. 1779; auch in LTV-017.X-01 | 180 | ja (6 Verbindungslimits, Abo-Verwaltung) | spezifiziert |
| LTV-X-20 | Gemeinsam genutzte Transcoding-Ausgaben mehrerer Zuschauer | 03 7.1 Z. 1780; auch in LTV-004.X-01 | 181 | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-21 | Vorsorglich gestartete individuelle Transcoding-Prozesse | 03 7.1 Z. 1781; auch in LTV-010.X-01 | 181 | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-22 | Automatische Komprimierung von Aufnahmen | 03 7.1 Z. 1782; auch in LTV-019.X-04, LTV-020.07 | 182 | ja (5 Aufnahmequalität wählbar) | spezifiziert |
| LTV-X-23 | Automatische Aufnahme-Konvertierung | 03 7.1 Z. 1783; auch in LTV-020.02 | 182 | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-24 | Automatische Übernahme in Film- und Serienbibliotheken | 03 7.1 Z. 1784; auch in LTV-008.X-05 | 183 | ja (10 Übernahme in die Bibliothek) | spezifiziert |
| LTV-X-25 | Suchwortbasierte automatische Aufnahmen | 03 7.1 Z. 1785; auch in LTV-008.X-03 | 178 | ja (8 Werbeerkennung, KI, Suchwort) | spezifiziert |
| LTV-X-26 | Neue Benutzergruppen | 03 7.1 Z. 1786; auch in LTV-006.X-01 | 184 | ja (9 Benutzergruppen) | spezifiziert |
| LTV-X-27 | Externe Transcoding-Nodes | 03 7.1 Z. 1787; auch in AB-1 | 185 | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-28 | Separate Live-TV-Microservices | 03 7.1 Z. 1788; auch in LTV-Q-08.15 | (nur über «Auch in»: 185) | nein (nicht ausdrücklich) | spezifiziert |
| LTV-X-29 | Verpflichtende externe IPTV-Middleware | 03 7.1 Z. 1789; auch in AB-1 | 333 | nein (nicht ausdrücklich) | spezifiziert |

---

## 6. Matrix E · rückwärts

### 6.1 T-n → LTV

`LTV-…` aus dem Text der Zeile in `04`; «AK-L»: Kriterien aus `17`, deren «Bezug» das T-n nennt. Ein T-n ohne Nennung eines Entscheids im Text steht mit «–».

| T-n | Titel | LTV-Entscheide (04-Text) | LTV-Q | LTV über AK-L (abgeleitet) | AK-L (Zahl; Bereich) |
|---|---|---|---|---|---|
| T-1 | Ein Dienst im Prozess. | – | – | – | 0; – |
| T-2 | Identität eines Eingangsstroms ist der Sender. | LTV-004, LTV-006 | – | 003, 004, 006, Q-15 | 2; 001, 040 |
| T-3 | Wer die Quelle holt: drei Wege. | – | – | – | 0; – |
| T-4 | Die Regel «FFmpeg sieht keine Adresse» (ADR 0025) gilt für … | – | – | – | 1; 321 |
| T-5 | Netzschutz je Quelle mit festen Zielen. | – | – | 003, Q-10 | 3; 045, 230–231 |
| T-6 | Internes Verteilformat ist unveränderter MPEG-TS. | LTV-020 | – | 004, Q-15 | 2; 001, 079 |
| T-7 | Der technische Startpuffer ist dieser Ring und nichts sonst. | LTV-007, LTV-013 | – | 004, 007, 010 | 4; 074, 087, 123, 338 |
| T-8 | Niemand kann den Empfang aufhalten. | LTV-004 | – | 004, Q-08, Q-15 | 3; 003, 073, 081 |
| T-9 | Eigener Sitzungstyp «Live-Sitzung». | – | – | – | 0; – |
| T-10 | Der persönliche Zeitversatz-Puffer liegt vor der Umwandlung und gehört einer … | LTV-007 | – | 007, 012, Q-15 | 3; 009–010, 125 |
| T-11 | Auslieferung als HLS je Sitzung mit TS-Segmenten; die Wiedergabeliste baut … | – | – | 014, Q-12 | 1; 294 |
| T-12 | Direktweg ohne FFmpeg, wo der Strom zum Gerät passt. | LTV-004 | – | 004 | 1; 068 |
| T-13 | Umwandlung je Sitzung durch genau einen FFmpeg, TS hinein, TS … | – | – | 004, 012, Q-12, Q-15 | 4; 002, 070, 077, 282 |
| T-14 | Der Server entscheidet je Sitzung zwischen Original, Ton-Umwandlung und Bild-Umwandlung | – | – | 004, Q-15 | 1; 002 |
| T-15 | Zeilensprung-Entfernung (Deinterlacing) ist ein neuer, abschaltbarer Baustein der Videokette | – | – | 004, Q-12 | 2; 070, 078 |
| T-16 | Die Aufnahme zweigt am Verteiler ab, | LTV-008 | – | 002, 004, 008, 020, Q-08, Q-15 | 3; 013–014, 081 |
| T-17 | Lebenszyklus über benannte Haltegründe. | LTV-002, LTV-017 | – | 002, 004, 010, 011, 017, Q-15 | 8; 003, 315–316, 319, 334–336, 354 |
| T-18 | Empfangsstrategie: vier Arten und Vorladen, alle verbindlich und vollständig beschrieben | LTV-002 | – | 002, 010, 017 | 2; 316, 336 |
| T-19 | Ein Zulassungsort, vier Grössen. | – | – | 002, 004, 012, Q-15 | 4; 033, 067, 074, 334 |
| T-20 | Empfang und Aufnahme laufen im Serverprozess, nicht in FFmpeg, | – | – | 004, 012, Q-15 | 1; 033 |
| T-21 | Stream-Endpunkte tragen ein Token lt, | – | – | 012, Q-10, Q-15 | 3; 028, 264, 318 |
| T-22 | Flüchtiges lebt im Arbeitsspeicher, Dauerhaftes in SQLite. | – | – | – | 0; – |
| T-23 | Statusänderungen kommen über Abfragen, nicht über einen neuen Push-Kanal. | – | – | – | 0; – |
| T-24 | Paketzuschnitt | – | – | – | 0; – |
| T-25 | Drei Speicherorte, jeder vom Administrator gewählt. | LTV-019 | – | 019, Q-15 | 2; 032, 150 |
| T-26 | Live-TV zählt zu den bestehenden Grenzen: | – | – | 004, 012, 017, Q-12 | 3; 076, 166, 283 |
| T-27 | FFmpeg wird eingedämmt, und kein Paket der Quelle kann den … | – | – | – | 2; 321, 341 |
| T-28 | Jede Adresse einer Quelle und jede Stream-Adresse eines Senders ist … | – | – | Q-10 | 2; 243, 348 |
| T-29 | Deaktivieren oder Verschwinden eines Senders beendet nichts Laufendes von selbst; … | LTV-002 | – | 002, 006 | 3; 147, 313, 340 |

### 6.2 SM-nn → LTV / Auftrag Abschnitt 10

Spalte «Bezug» aus `12` Abschnitt 11. `LTV-Q-10.nn` entspricht dem Auftrag Abschnitt 10 (Sicherheit). «Pflicht vor»: Stufe, vor der Massnahme und Testfall vorliegen müssen.

| SM | Massnahme | Bezug (LTV) | Auftrag 10 | Pflicht vor | LTV über AK-L (abgeleitet) | AK-L |
|---|---|---|---|---|---|---|
| SM-01 | Dialer je Quelle mit Prüfung jeder Verbindung an der angewählten … | LTV-003 | 10: .02, .03, .04, .09, .10 | 1a (Regel mit leerer Liste: nur öffentlich), H (feste Ziele) | 003 | 031, 045, 230, 352 |
| SM-02 | ein HTTP-Transport je Quelle und Regelstand, geschlossen bei Änderung; laufende … | – | – | 1a (H für den Fall mit Freigabe) | – | 231, 340 |
| SM-03 | Präfixe 64:ff9b:1::/48, 2001::/32, ::ffff:0:0:0/96, 192.0.0.0/24, 240.0.0.0/4 in netguard; eigene Adressen … | – | 10: .07, .08 | 1a | – | 232 |
| SM-04 | eigene CheckRedirect: höchstens 5 Sprünge mit Unterlisten, nur http/https, kein … | – | 10: .05 | 1a | – | 031, 233 |
| SM-05 | DNS-Rebinding: keine Entscheidung vor der Verbindung; feste Ziele als Adressen, … | – | 10: .06 | 1a | – | 234 |
| SM-06 | Import ohne Probe; Sender ausserhalb der Ziele markiert, nie verbunden; … | – | 10: .09 | 1a (Markierung ohne Probe), H (Sammelhinweis, Freigabe) | 003 | 045, 235, 352 |
| SM-07 | Grenzen für HLS: Takt, Zieldauer, Varianten, Tiefe, Kreise, Grössen | – | 10: .12, .13, .14, .15 | 1a | – | 236 |
| SM-08 | Weg W: Ersatzpfade auf 127.0.0.1 für jede Adresse, keine Proxy-Einstellung, … | – | 10: .36 | W | 003 | 031, 041, 237 |
| SM-09 | Sperre des FFmpeg-Empfängers, hart gesetzt: Landlock (Dateien; TCP nur Port … | – | – | W (Pflicht) | – | 238, 321, 341 |
| SM-10 | UDP, RTP, Multicast: nur Administrator, Bindung an eine Schnittstelle, Port … | – | 10: .11 | H | 003 | 043–044, 239 |
| SM-11 | Netzmodus «host» als A-9; Weg W gesperrt; Adressen und IPv6-Präfixe … | – | – | H | 003 | 044, 240 |
| SM-12 | RTSP/RTMP nur über den übergebenen Dialer, RTSP nur TCP, keine … | – | – | L | 003 | 042, 241 |
| SM-13 | Logos und EPG-Bilder nur über den Server, nur öffentlich; Go … | – | 10: .16, .17 | 1a (Logos), 2 (EPG-Bilder) | 005, 006 | 056, 119, 242 |
| SM-14 | jede Adresse und jeder Kopfzeilen-Wert verschlüsselt, nur site im Klartext; … | – | 10: .18, .19 | 1a | 003, 017 | 039, 243 |
| SM-15 | Geheimnisse nur an feste Ursprünge; aus Listen nur User-Agent und … | – | 10: .20, .21, .26 | 1a | – | 244 |
| SM-16 | API und Oberfläche: Geheimnisse nur schreibbar, nur site und Namen, … | – | 10: .23, .24 | 1a | 003, 011, 017 | 039, 097, 245 |
| SM-17 | Schwärzung nach 5.4; nur netguard.Site im Protokoll (Lint); redactAny für … | – | 10: .19, .22 | 0 | 011 | 098, 246 |
| SM-18 | Sicherung ohne Geheimnisse überschreibt jeden verschlüsselten Live-Wert mit secure_delete durch … | – | 10: .25 | 1a | 017 | 247, 276, 329 |
| SM-19 | keine Zugangsdaten in Argumenten, Umgebung, ffmpeg.log; Fehlerausgabe von FFmpeg nie … | – | – | 1a | 011 | 097, 248 |
| SM-20 | Xtream: Adressen nur aus dem eingetragenen Host, server_info nie verwendet, … | – | 10: .26 | 1a | 003 | 037, 249 |
| SM-21 | Argumente nur Zahlen und Listenwerte; festes Eingangsformat; -protocol_whitelist pipe auch … | – | 10: .27, .28 | 1a | 020 | 015, 250 |
| SM-22 | Hülle für jeden Live-FFmpeg: Umgebung (ohne SLOUCHCAT_, http_proxy, no_proxy), rlimits … | – | 10: .29, .35 | 1a (mit M-25 auf CT 190 vor der Baufreigabe von 0/1a) | – | 251, 321, 341 |
| SM-23 | Compose: no-new-privileges, cap_drop: [ALL], pids_limit | – | – | 0 (nach 04 F-12 (a) und V-4, 18 Planschritt 0.6), sonst 1a | – | 252 |
| SM-24 | Prozesslimits, Zeitgrenzen, Beendigung, Waisen, Neustartdrossel | – | 10: .31, .32, .33, .34, .35 | 1a | 004 | 026, 030, 253 |
| SM-25 | recover an der Wurzel jeder Live-Go-Routine; keine Allokation aus Längenfeldern; … | – | 10: .29, .30 | 1a (TS, HLS, M3U, Xtream), 2 (XMLTV), H (RTP), W (MPD) | – | 254 |
| SM-26 | govulncheck in der CI; Abgleich der FFmpeg-Fassung bei jedem Release; … | – | – | 0 (Prüfung auf Test-Tags, 18 Planschritt 0.5), 1a (govulncheck, FFmpeg-Abgleich) | – | 255 |
| SM-27 | Grenzen für M3U, Xtream und Upload; streamendes Lesen; Stapel | – | 10: .41, .42, .43, .44, .45 | 1a | 018 | 021, 049, 256 |
| SM-28 | XMLTV: eigene Tiefen- und Mengengrenze in der Token-Schleife (Go begrenzt … | – | 10: .37, .38 | 2 | – | 257 |
| SM-29 | Kompression: Magic Bytes, identity, eine Schicht, Zähler nach dem Entpacken, … | – | 10: .39, .40 | 2 | 005 | 104, 258 |
| SM-30 | FTS5-Eingabe in Wörter zerlegt, zitiert, begrenzt | – | – | 2 | 005 | 111, 259 |
| SM-31 | Deckel, Reserve, Höchstdauer, Rückstau, kein stilles Ausweichen | – | 10: .46 | 4 | 019 | 025, 128, 152, 260 |
| SM-32 | Namen nur aus Kennungen, Zugriff nur über paths, 0700/0600, Löschen … | – | 10: .47, .48, .52 | 4 (Aufnahmen 5) | 007 | 009, 261 |
| SM-33 | Marker und Dateisystemtyp je Speicherort, kein Schreiben in leeren Einhängepunkt | – | 10: .51 | 4 (Aufnahmen 5) | 019 | 032, 158, 262 |
| SM-34 | Timeshift- und Aufnahmedateien nur über Server-Endpunkte der eigenen Sitzung; keine … | – | 10: .49, .50 | 4 (Aufnahmen 5) | 007 | 009, 011, 263 |
| SM-35 | Token lt nach 08 4.2: HMAC zuerst, Bindung an Sitzung … | – | 10: .53, .54, .59 | 1a | 012 | 028, 264 |
| SM-36 | Widerruf über die Haken in auth; Sicherung je Listenanfrage und … | – | 10: .55, .56 | 1a | 012 | 028–029, 265, 351 |
| SM-37 | eine Zugriffsabfrage je Objektart mit Profil und Konto; Aufnahmeregel nach … | LTV-009 | 10: .57, .58 | 1a (Favoriten, Verlauf, Sitzungen), 2 (Erinnerungen), 5 (Aufnahmen, Planungen), 6 (Serienregeln) | 007, 009, 012 | 011, 016, 028, 164, 266 |
| SM-38 | Vertragstest: jede Live-Operation und Route ohne wirksames Recht 404; x-visibility, … | LTV-012 | 10: .61, .62 | 1a | 007, 012, 015 | 011, 018, 267 |
| SM-39 | Drosselung nach 9.5; Serverdeckel; Karenz weicht zuerst; Sitzung ohne Segmentabruf … | – | 10: .60 | 1a (Drosselung, Deckel, Sitzung ohne Segmentabruf wie 04 8.5 und 17), 3 (deviceId) | 010 | 090, 268, 351 |
| SM-40 | bestehender CSRF-Schutz unverändert für jede schreibende Live-Operation (403 csrf_header_missing, gebaut); … | – | 10: .59 | 1a | 012 | 028, 269 |
| SM-41 | Protokoll der Verwaltung für Quellen, Ziele, Ursprünge, Speicherorte, Schalter | – | – | 1a (Quellen, Ursprünge, Schalter), H (feste Ziele) | – | 270 |
| SM-42 | Administratorsicht ohne private Titel; Meldungen bei PIN-Profilen ohne Titel | LTV-009 | – | 1a (Wiedergaben), 5 (Aufnahmen) | 009 | 016, 165, 271, 327 |
| SM-43 | benannte Ausnahmen A-1 bis A-13 (A-10 Weg W, A-11 Zusatz … | – | 10: .63, .64 | 0 | – | 272 |
| SM-44 | DRM erkennen und ablehnen (05 2.5; Einordnung nach der HLS-Spezifikation … | LTV-003 | – | 1a (HLS), W (DASH) | 003 | 031, 041, 176, 273 |
| SM-45 | Verwaltung zeigt laufende Aufnahmen je Eingang nur als Zahl ab … | LTV-009 | – | 5 | 009 | 327 |
| SM-46 | Meldung «Kontingent voll» an das Hauptprofil in eigener Art ohne … | LTV-009 | – | 5 | 009 | 328 |
| SM-47 | Entfernen einer Quelle und Ändern ihrer Adresse, Zugangsdaten, Kopfzeilen, Ursprünge … | – | – | 1a (Quellen, Text beim Deaktivieren), H (Ziele) | – | 340 |
| SM-48 | Bau der Hülle: Unterbefehl als erste Anweisung in main ohne … | – | – | 1a (Lint 0) | – | 342 |
| SM-49 | Filter der Systemaufrufe als Erlaubnisliste mit SECCOMP_RET_KILL_PROCESS, Prüfung der Architektur, … | – | – | 1a (Umwandlung, Probe), W (Empfänger) | – | 343 |
| SM-50 | Schauzeit nach dem, was der Server beobachtet: jedes ausgelieferte, verschiedene … | LTV-012 | – | 1a | – | 344 |
| SM-51 | Weiterleiter beantwortet nur selbst umgeschriebene Verweise (genaue Adressen, Vorlagen nur … | – | – | W | – | 345 |
| SM-52 | Feste Reihenfolge der Prüfungen: Anmeldung, CSRF, Recht, Objekt, erst dann … | – | 10: .62 | 1a | – | 346 |
| SM-53 | Mengengrenze der Programmdaten (500 Sendungen je Sender und Tag, Gesamtzahl … | – | 10: .45 | 2 | – | 347 |
| SM-54 | Ursprung = Schema, Host und Port; Geheimnisse einer https-Quelle nie … | – | 10: .26 | 1a | – | 348 |
| SM-55 | Empfehlung an den Bestand (kein Teil von Konzept 28, braucht … | – | – | 0 (Empfehlung) | – | 349 |
| SM-56 | Meldungen über E-Mail, Push und Telegram zu Aufnahmen und Erinnerungen … | LTV-009 | – | 2 (Erinnerungen), 5 (Aufnahmen) | – | 350 |

### 6.3 Seiten LS-/LA-/LD- → LTV

Spalte «LTV» wörtlich aus `14` 2 und `15` 2 (Auftragsabschnitte wie «9.3» bleiben dort stehen, sind hier weggelassen). Die geänderten bestehenden Seiten (LS-20 bis LS-26, LA-20 bis LA-28) haben dort keine LTV-Spalte; die Tabelle zeigt nur `LTV-…`, das in ihrer Zeile steht. Dialoge `LD-nn` haben keinen Entscheid. Operationen: Zahl der Operationen in `08`, die die Seite nennen.

| Seite | Name | LTV | Stufe | Mockup | Operationen (08) | Doku |
|---|---|---|---|---|---|---|
| LS-01 | Live-TV-Start | LTV-014 | 1 (Zuletzt, Meine Sender, Einstiege); 2 (Jetzt, Gleich, Genre); 5 (Einstiege Aufnahmen) | ja | 8 | 14 |
| LS-02 | Senderübersicht | LTV-006, LTV-014 | 1 (Jetzt/Danach ab 2; eigene Gruppen ab 1b) | ja | 4 | 14 |
| LS-03 | Meine Sender | LTV-001, LTV-010, LTV-014 | 1 | ja | 4 | 14 |
| LS-04 | Programmzeitschrift | LTV-005, LTV-014 | 2 (Aufnahme-Aktionen 5) | ja | 4 | 14 |
| LS-05 | Programmsuche | LTV-005 | 2 | ja | 1 | 14 |
| LS-06 | Sendungsdetails | LTV-005, LTV-008, LTV-014 | 2 (Aufnehmen 5, Serie 6) | ja | 2 | 14 |
| LS-07 | Live-Player | LTV-007, LTV-010, LTV-014, LTV-016 | 1 (Schauen, Zappen); 2 (Banner, Programm-Überlagerung); 3 (Hinweis «Ersatzquelle»); 4 (Timeshift, LD-10); 5 (Aufnehmen) | ja | 14 | 14 |
| LS-08 | Meine Aufnahmen | LTV-008, LTV-009, LTV-014 | 5 (Serienstapel 6) | ja | 5 | 14 |
| LS-09 | Aufnahme-Detail | LTV-008, LTV-009 | 5 | ja | 2 | 14 |
| LS-10 | Aufnahme-Player | LTV-008, LTV-016, LTV-020 | 5 | ja | 8 | 14 |
| LS-11 | Geplante Aufnahmen | LTV-008, LTV-014 | 5 (Termine, Manuell); 6 (Serien, Wiederkehrend) | ja | 5 | 14 |
| LS-12 | Konflikte und Warnungen | LTV-008, LTV-019 | 5 | ja | 2 | 14 |
| LS-13 | Erinnerungen | LTV-005 | 2 | ja | 2 | 14 |
| LS-14 | Aufnahmen meiner Profile | LTV-009, LTV-015 | 5 | ja | 4 | 14 |
| LS-15 | Senderprogramm | LTV-005 | 2 («Zeit aufnehmen» 5) | ja | 4 | 14 |
| LA-01 | Live-TV-Übersicht | – | 1a (Kosten des vorsorglichen Empfangs 3, Zahl der festen Ziele H) | ja | 5 | 15 |
| LA-02 | Quellen | LTV-018 | 1a (Abzeichen Heimnetz H) | ja | 5 | 15 |
| LA-03 | Quelle anlegen und ändern | LTV-003, LTV-017, LTV-018 | 1a (HTTP(S), HLS-TS, MPEG-TS, M3U, Xtream); H (feste Ziele, UDP, RTP, Multicast); L (RTSP, … | ja | 7 | 15 |
| LA-04 | Import-Vorschau und -Ergebnis | LTV-018 | 1a («zurückgehaltene Liste» 1b) | ja | 5 | 15 |
| LA-05 | Sender | LTV-006 | 1a (Massenbearbeitung, Nummern 1b) | ja | 5 | 15 |
| LA-06 | Sender bearbeiten | LTV-002, LTV-006, LTV-011 | 1a (Logo-Upload 1b, Programmdaten 2, Empfang 3) | ja | 11 | 15 |
| LA-07 | Duplikate | LTV-006 | 1b | ja | 5 | 15 |
| LA-08 | Sendergruppen | LTV-006 | 1b (Gruppen aus Quellen 1a) | ja | 5 | 15 |
| LA-09 | Empfang | LTV-002 | 3 | ja | 6 | 15 |
| LA-10 | EPG-Quellen | LTV-005 | 2 | ja | 11 | 15 |
| LA-11 | EPG-Zuordnungen | LTV-005 | 2 | ja | 6 | 15 |
| LA-12 | Aktive Streams | – | 1a (Quellwechsel, «zurückgestellt» 3) | ja | 5 | 15 |
| LA-13 | Überwachung und Diagnose | LTV-011 | 1a (Grundteil), 3 (erweitert), E (stehendes Bild) | ja | 4 | 15 |
| LA-14 | Aufnahmen (DVR) | LTV-008 | 5 | ja | 1 | 15 |
| LA-15 | Timeshift | LTV-007, LTV-019 | 4 | ja | 1 | 15 |
| LA-16 | Speicher | LTV-019 | 4 (Timeshift), 5 (Aufnahmen, Belegung je Konto) | ja | 8 | 15 |
| LD-01 | Aufnahme planen | – | 5 | ja | 3 | 14 |
| LD-02 | Serienaufnahme | – | 6 | ja | 3 | 14 |
| LD-03 | Manuelle oder wiederkehrende Aufnahme | – | 5; 6 | ja | 6 | 14 |
| LD-04 | Sofortaufnahme | – | 5 | ja | 2 | 14 |
| LD-05 | Erinnerung | – | 2 | ja | 3 | 14 |
| LD-06 | Löschen und Abbrechen | – | 5 | ja | 3 | 14 |
| LD-07 | Konflikt oder Warnung | – | 5 | ja | 1 | 14 |
| LD-08 | Tag und Uhrzeit wählen | – | 2 | ja | 0 | 14 |
| LD-09 | Filterblatt, Handy | – | 1 | ja | 1 | 14 |
| LD-10 | Senderwechsel bei Zeitversatz | – | 4 | ja | 1 | 14 |
| LD-11 | Aufbewahrung ändern | – | 5 | ja | 1 | 14 |
| LS-20 | Hauptnavigation | – | 1 | ja | 0 | 14 |
| LS-21 | Profile verwalten: Profilformular (/settings/profiles, web/src/features/profiles/ProfileForm.tsx; Handy-App Einstellungen › … | – | 1 (Schalter «Live-TV»), 4 («Timeshift»), 5 («Aufnahmen», Link, Löschdialog) | ja | 1 | 14 |
| LS-22 | Benachrichtigungen einstellen (/settings/notifications, features/notifications/ChannelSettingsPage.tsx) | – | 2 (Erinnerung), 5 (Aufnahmen) | ja | 0 | 14 |
| LS-23 | Benachrichtigungen (Glocke, features/discover/NotificationsPage.tsx, ui/InboxEntry.tsx) | – | 2, 5 | ja | 0 | 14 |
| LS-24 | Suche (/search, features/discover/SearchPage.tsx) | – | 2 (nur nach `03` F-5) | ja | 0 | 14 |
| LS-25 | Ton-Vorrang (web/src/app/radio/priority.ts) und Mini-Player-Dock | – | 1 | ja | 0 | 14 |
| LS-26 | Startseite | – | – | ja | 0 | 14 |
| LA-20 | Verwaltungsübersicht /admin (AdminHomePage.tsx) | – | 1a | ja | 1 | 15 |
| LA-21 | Aktive Wiedergaben /admin/streams (StreamsPage.tsx) | – | 1a; «Private Aufnahme» 5 | ja | 0 | 15 |
| LA-22 | Konten /admin/accounts (AccountsPage.tsx, AccountDetail.tsx, accounts/AccessForm.tsx) und Einladungen | – | 1a (Live-TV), 4 (Timeshift), 5 (Aufnahmen, Kontingent) | ja | 0 | 15 |
| LA-23 | Hintergrundjobs /admin/jobs (JobsPage.tsx) | – | 1a, je Stufe ergänzt | ja | 0 | 15 |
| LA-24 | Benachrichtigungen der Verwaltung /admin/notifications | – | 1a (Import), 2 (EPG), 3 (Ausfall), 4/5 (Speicher) | ja | 0 | 15 |
| LA-25 | Seitenleiste und Suchpalette | – | 1a | ja | 2 | 15 |
| LA-26 | Meine Medien /admin/my-media (features/admin/mymedia) | – | 1a | ja | 0 | 15 |
| LA-27 | Server › Update (ServerPage.tsx, Update-Dialog) | – | 5 | ja | 1 | 15 |
| LA-28 | Sicherung /admin/backup | – | 1a; Satz zu Programmdaten 2, zu Aufnahmen 5 | ja | 0 | 15 |

Seiten, die `08` nennt, aber `14`/`15` Abschnitt 2 nicht führt: keine.

### 6.4 Stufe → LTV-Entscheide

Zahl der Anforderungen und Ausschlüsse je Entscheid, deren Stufenangabe in `03` die Stufe nennt (eine Anforderung mit zwei Stufen zählt in beiden; 1a und 1b sind unter «1» zusammengefasst). «18 4.13»: Spalte «LTV-Entscheide (Schwerpunkt)» des Umsetzungsplans.

| Stufe | Entscheide nach 03 (Zahl der Anforderungen) | Entscheide nach 18 4.13 |
|---|---|---|
| K | 001 (1), 002 (1), 003 (2), 009 (1), 010 (29), 013 (1), 014 (2), 015 (1), 020 (2), 021 (15), Q-08 (45), Q-10 (3), Q-11 (43), Q-12 (1), Q-13 (71), Q-14 (45), Q-15 (2), Q-17 (36) | – |
| 0 | 011 (1), 015 (1), Q-10 (3), Q-12 (1) | 015 |
| 1 | 001 (8), 002 (3), 003 (16), 004 (36), 006 (20), 010 (3), 011 (6), 012 (11), 014 (22), 015 (8), 016 (2), 017 (3), 018 (21), Q-08 (1), Q-09 (52), Q-10 (43), Q-12 (21), Q-14 (2), Q-15 (27) | 001, 002, 003, 004, 006, 010, 011, 012, 015, 016, 017, 018 |
| 2 | 001 (3), 005 (26), 014 (27), 015 (3), 018 (1), Q-09 (8), Q-10 (6), Q-12 (2), Q-15 (1) | 005, 014, 018 |
| 3 | 001 (1), 002 (7), 003 (1), 004 (1), 007 (2), 010 (13), 011 (15), 013 (1), Q-09 (8), Q-11 (3), Q-15 (7) | 002, 003, 004, 007, 010, 011, 013, 019 |
| H | 003 (3), Q-10 (3) | 003 |
| 4 | 001 (1), 007 (12), 012 (1), 014 (5), 019 (17), Q-09 (7), Q-10 (6), Q-15 (4) | 007, 012, 014, 019 |
| 5 | 001 (1), 002 (2), 004 (1), 005 (2), 008 (23), 009 (10), 012 (1), 014 (20), 015 (1), 018 (1), 019 (6), 020 (20), Q-08 (2), Q-09 (8), Q-10 (2), Q-12 (3), Q-15 (9) | 002, 004, 005, 008, 009, 012, 014, 015, 018, 019, 020 |
| 6 | 001 (1), 008 (3), 009 (1), 014 (4) | 008, 009, 014 |
| L | 003 (2) | 003 |
| W | 003 (1), Q-10 (1) | 003 |
| E | 004 (1), 011 (1), 014 (1) | 004, 011 |
| Apps | 014 (4), Q-12 (2) | 014 |

---

## 7. Prüfergebnis (aus dem Erzeugungsskript)

Das Skript (nicht im Repo) hat `03`, `17`, `04`, `07`, `08`, `12`, `14`, `15`, `18`, `19` gelesen. Die Zahlen sind gezählte Zeilen, keine Schätzung.

### 7.1 Zählung

| Grösse | Wert |
|---|---|
| Zeilen in `03` Abschnitte 3 und 4 (ohne `LTV-X-nn` aus 7.1) | **974**: Entscheide 454 Anforderungen + 45 Ausschlüsse `…X…`; Querschnitt 474 Anforderungen + 1 Ausschluss `LTV-Q-12.X-01` |
| davon Anforderungen (ohne Ausschlüsse) | **928** (= 454 + 474) |
| davon Ausschlüsse (`…X-…`) | 46 |
| als «entfällt» geführte Zeilen (nicht mitgezählt) | 1: LTV-014.X-01 |
| Zeilen der Ausschlussliste `LTV-X-nn` (03 7.1) | 29 |
| `AK-L`-Zeilen in `17` | 357 (davon ohne erkennbaren LTV-/LTV-Q-/LTV-X-Bezug: 55) |
| Architekturentscheide `T-n` in `04` | 29 |
| Sicherheitsmassnahmen `SM-nn` in `12` | 56 |
| Seiten und Dialoge `LS-`/`LA-`/`LD-` in `14`/`15` | 58 (davon Mockup: 58) |
| Operationen in `08` (Tabellen 2.1–2.10 mit Seitenspalte) | 115 (`08` 2.13 nennt «Neu insgesamt» 115) |
| Tabellen in `07` mit eigenem Absatz | 31 |
| Offene Entscheidungen `E-28-nn` in `20` | 69 |

**Abgleich mit der Zählung in `03` (Anhang «Zählung»):**

- Zeilen «Summe» im Anhang von `03`: 454 Anforderungen / 45 Ausschlüsse. Die Einzelzellen desselben Anhangs ergeben 454 / 45; das Skript zählt 454 / 45. 
- Querschnitt: Anhang nennt 474 Anforderungen und 1 Ausschluss; das Skript zählt 474 Anforderungen und 1 Ausschluss.
- Keine Abweichung je Entscheid und je Querschnittsabschnitt.
- `LTV-Q-12.X-…`-Zeilen (Ausschluss im Querschnitt): 1; sie zählen oben bei «Querschnitt».

### 7.2 Anforderungen in `03`: Stufe, Dokument, Abnahme, Kriterium

| Prüfung | Zahl | von |
|---|---|---|
| Anforderungen mit Stufe (nicht leer, nicht «–») | 928 | 928 |
| mit Dokumentverweis (Spalte «Dok») | 924 | 928 |
| Anforderungen mit Abnahme T oder M | 588 | 928 |
| … davon mit mindestens einem `AK-L` | 588 | 588 |
| … davon **ohne** `AK-L` | 0 | 588 |
| Abnahme S (Sichtprüfung) | 340 | 928 |
| … davon mit `AK-L` (freiwillig) | 27 | |
| Anforderungen mit irgendeinem `AK-L` | 615 | 928 |
| Anforderungen ohne `AK-L` (alle Arten) | 313 | 928 |
| Ausschlüsse (`…X-…`) mit mindestens einem `AK-L` | 43 | 46 |
| Ausschlüsse mit Abnahme T oder M | 9 | 46 |

Verteilung der Abnahmeart in `03`: «M»: 21, «S»: 377, «T»: 576.

**Ohne Stufe (0):** keine. Die 46 Ausschlüsse tragen in `03` bewusst «–» als Stufe (sie werden nicht gebaut) und sind hier nicht mitgezählt.

**Ohne Dokumentverweis (4):** LTV-021.01, LTV-021.02, LTV-021.03, LTV-021.15. Von den Ausschlüssen tragen {sum(1 for r in excl if r['dok'].strip() in ('', '–', '-'))} von {len(excl)} «–» als Dokument; sie sind hier nicht mitgezählt (Negativtest siehe Matrix D).

**Abnahme T oder M, aber ohne `AK-L` («nicht zugeordnet», 0):** keine.

**Abnahme S ohne `AK-L` (313):** erwartungsgemäss; `17` 9.3 weist sie der Sichtprüfung dieser Matrix zu (`AK-L-310`). Kennungen: LTV-001.01, LTV-001.03, LTV-001.05, LTV-001.15, LTV-003.24, LTV-003.25, LTV-004.21, LTV-004.38, LTV-005.17, LTV-006.20, LTV-007.12, LTV-009.12, LTV-010.01, LTV-010.11, LTV-010.12, LTV-010.13, LTV-010.14, LTV-010.15, LTV-010.16, LTV-010.17, LTV-010.18, LTV-010.19, LTV-010.20, LTV-010.21, LTV-010.22, LTV-010.23, LTV-010.24, LTV-010.25, LTV-010.26, LTV-010.27, LTV-010.32, LTV-010.33, LTV-010.34, LTV-010.35, LTV-010.36, LTV-010.37, LTV-010.38, LTV-010.39, LTV-010.40, LTV-010.41, LTV-010.43, LTV-012.01, LTV-013.01, LTV-014.07, LTV-014.20, LTV-014.21, LTV-014.22, LTV-014.55, LTV-014.56, LTV-014.74, LTV-014.80, LTV-014.82, LTV-014.81, LTV-015.11, LTV-015.14, LTV-017.03, LTV-019.17, LTV-020.08, LTV-020.10, LTV-020.21, LTV-Q-08.01, LTV-Q-08.02, LTV-Q-08.03, LTV-Q-08.04, LTV-Q-08.05, LTV-Q-08.06, LTV-Q-08.07, LTV-Q-08.08, LTV-Q-08.09, LTV-Q-08.10, LTV-Q-08.11, LTV-Q-08.12, LTV-Q-08.13, LTV-Q-08.14, LTV-Q-08.16, LTV-Q-08.17, LTV-Q-08.18, LTV-Q-08.19, LTV-Q-08.20, LTV-Q-08.21, LTV-Q-08.22, LTV-Q-08.23, LTV-Q-08.24, LTV-Q-08.27, LTV-Q-08.28, LTV-Q-08.29, LTV-Q-08.30, LTV-Q-08.31, LTV-Q-08.32, LTV-Q-08.33, LTV-Q-08.34, LTV-Q-08.35, LTV-Q-08.36, LTV-Q-08.37, LTV-Q-08.38, LTV-Q-08.39, LTV-Q-08.40, LTV-Q-08.41, LTV-Q-08.42, LTV-Q-08.43, LTV-Q-08.44, LTV-Q-08.45, LTV-Q-08.46, LTV-Q-08.47, LTV-Q-09.01, LTV-Q-09.82, LTV-Q-09.83, LTV-Q-10.01, LTV-Q-10.65, LTV-Q-11.01, LTV-Q-11.02, LTV-Q-11.03, LTV-Q-11.04, LTV-Q-11.05, LTV-Q-11.06, LTV-Q-11.07, LTV-Q-11.08, LTV-Q-11.09, LTV-Q-11.10, LTV-Q-11.11, LTV-Q-11.12, LTV-Q-11.13, LTV-Q-11.14, LTV-Q-11.15, LTV-Q-11.16, LTV-Q-11.17, LTV-Q-11.18, LTV-Q-11.19, LTV-Q-11.20, LTV-Q-11.21, LTV-Q-11.22, LTV-Q-11.23, LTV-Q-11.24, LTV-Q-11.25, LTV-Q-11.26, LTV-Q-11.27, LTV-Q-11.28, LTV-Q-11.29, LTV-Q-11.30, LTV-Q-11.31, LTV-Q-11.32, LTV-Q-11.33, LTV-Q-11.34, LTV-Q-11.38, LTV-Q-11.39, LTV-Q-11.40, LTV-Q-11.41, LTV-Q-11.42, LTV-Q-11.43, LTV-Q-11.44, LTV-Q-11.45, LTV-Q-11.46, LTV-Q-12.05, LTV-Q-12.06, LTV-Q-12.14, LTV-Q-12.18, LTV-Q-12.19, LTV-Q-12.26, LTV-Q-12.28, LTV-Q-12.29, LTV-Q-13.01, LTV-Q-13.02, LTV-Q-13.03, LTV-Q-13.04, LTV-Q-13.05, LTV-Q-13.06, LTV-Q-13.07, LTV-Q-13.08, LTV-Q-13.09, LTV-Q-13.10, LTV-Q-13.11, LTV-Q-13.12, LTV-Q-13.13, LTV-Q-13.14, LTV-Q-13.15, LTV-Q-13.16, LTV-Q-13.17, LTV-Q-13.18, LTV-Q-13.19, LTV-Q-13.20, LTV-Q-13.21, LTV-Q-13.22, LTV-Q-13.23, LTV-Q-13.24, LTV-Q-13.25, LTV-Q-13.26, LTV-Q-13.27, LTV-Q-13.28, LTV-Q-13.29, LTV-Q-13.30, LTV-Q-13.31, LTV-Q-13.32, LTV-Q-13.33, LTV-Q-13.34, LTV-Q-13.35, LTV-Q-13.36, LTV-Q-13.37, LTV-Q-13.38, LTV-Q-13.39, LTV-Q-13.40, LTV-Q-13.41, LTV-Q-13.42, LTV-Q-13.43, LTV-Q-13.44, LTV-Q-13.45, LTV-Q-13.46, LTV-Q-13.47, LTV-Q-13.48, LTV-Q-13.49, LTV-Q-13.50, LTV-Q-13.51, LTV-Q-13.52, LTV-Q-13.53, LTV-Q-13.54, LTV-Q-13.55, LTV-Q-13.56, LTV-Q-13.57, LTV-Q-13.58, LTV-Q-13.59, LTV-Q-13.60, LTV-Q-13.61, LTV-Q-13.62, LTV-Q-13.63, LTV-Q-13.64, LTV-Q-13.65, LTV-Q-13.66, LTV-Q-13.67, LTV-Q-13.68, LTV-Q-13.69, LTV-Q-13.70, LTV-Q-13.71, LTV-Q-14.01, LTV-Q-14.02, LTV-Q-14.03, LTV-Q-14.04, LTV-Q-14.05, LTV-Q-14.06, LTV-Q-14.07, LTV-Q-14.08, LTV-Q-14.09, LTV-Q-14.10, LTV-Q-14.11, LTV-Q-14.12, LTV-Q-14.13, LTV-Q-14.14, LTV-Q-14.15, LTV-Q-14.16, LTV-Q-14.17, LTV-Q-14.18, LTV-Q-14.19, LTV-Q-14.20, LTV-Q-14.21, LTV-Q-14.22, LTV-Q-14.23, LTV-Q-14.24, LTV-Q-14.25, LTV-Q-14.26, LTV-Q-14.27, LTV-Q-14.28, LTV-Q-14.29, LTV-Q-14.30, LTV-Q-14.31, LTV-Q-14.32, LTV-Q-14.33, LTV-Q-14.34, LTV-Q-14.35, LTV-Q-14.36, LTV-Q-14.37, LTV-Q-14.38, LTV-Q-14.39, LTV-Q-14.40, LTV-Q-14.41, LTV-Q-14.42, LTV-Q-14.43, LTV-Q-14.44, LTV-Q-14.45, LTV-Q-14.47, LTV-Q-17.01, LTV-Q-17.02, LTV-Q-17.03, LTV-Q-17.04, LTV-Q-17.05, LTV-Q-17.06, LTV-Q-17.07, LTV-Q-17.08, LTV-Q-17.09, LTV-Q-17.10, LTV-Q-17.11, LTV-Q-17.12, LTV-Q-17.13, LTV-Q-17.14, LTV-Q-17.15, LTV-Q-17.16, LTV-Q-17.17, LTV-Q-17.18, LTV-Q-17.19, LTV-Q-17.20, LTV-Q-17.21, LTV-Q-17.22, LTV-Q-17.23, LTV-Q-17.24, LTV-Q-17.25, LTV-Q-17.26, LTV-Q-17.27, LTV-Q-17.28, LTV-Q-17.29, LTV-Q-17.30, LTV-Q-17.31, LTV-Q-17.32, LTV-Q-17.33, LTV-Q-17.34, LTV-Q-17.35, LTV-Q-17.36.

### 7.3 Kennungen in `17`, die in `03` nicht vorkommen, und umgekehrt

**Im «Bezug» einer `AK-L`-Zeile oder in der Prosa von `17` genannt, aber in `03` nicht definiert (1):** LTV-014.X-01.

Nur im Bezug (mit den `AK-L`): LTV-014.X-01: AK-L-185, AK-L-355.

In `03` steht LTV-014.X-01 nur noch als «entfällt»; `17` nennt die Kennung weiterhin (siehe oben), das ist ein Nachzug in `17`.

**In `03` definiert, in `17` nirgends genannt (weder im Bezug noch in der Prosa; 315), davon mit Abnahme T oder M: 0** (keine). Die übrigen haben Abnahme S (Sichtprüfung, kein `AK-L` vorgesehen, `17` 9.3):

LTV-001.01, LTV-001.03, LTV-001.05, LTV-001.15, LTV-003.24, LTV-003.25, LTV-004.21, LTV-005.17, LTV-006.20, LTV-007.12, LTV-009.12, LTV-010.01, LTV-010.11, LTV-010.12, LTV-010.13, LTV-010.14, LTV-010.15, LTV-010.16, LTV-010.17, LTV-010.18, LTV-010.19, LTV-010.20, LTV-010.21, LTV-010.22, LTV-010.23, LTV-010.24, LTV-010.25, LTV-010.26, LTV-010.27, LTV-010.32, LTV-010.33, LTV-010.34, LTV-010.35, LTV-010.36, LTV-010.37, LTV-010.38, LTV-010.39, LTV-010.40, LTV-010.41, LTV-010.43, LTV-011.X-01, LTV-012.01, LTV-013.01, LTV-014.07, LTV-014.20, LTV-014.21, LTV-014.22, LTV-014.55, LTV-014.56, LTV-014.74, LTV-014.80, LTV-014.81, LTV-014.82, LTV-015.11, LTV-015.14, LTV-017.03, LTV-018.X-01, LTV-019.17, LTV-020.08, LTV-020.10, LTV-020.21, LTV-Q-08.01, LTV-Q-08.02, LTV-Q-08.03, LTV-Q-08.04, LTV-Q-08.05, LTV-Q-08.06, LTV-Q-08.07, LTV-Q-08.08, LTV-Q-08.09, LTV-Q-08.10, LTV-Q-08.11, LTV-Q-08.12, LTV-Q-08.13, LTV-Q-08.14, LTV-Q-08.16, LTV-Q-08.17, LTV-Q-08.18, LTV-Q-08.19, LTV-Q-08.20, LTV-Q-08.21, LTV-Q-08.22, LTV-Q-08.23, LTV-Q-08.24, LTV-Q-08.27, LTV-Q-08.28, LTV-Q-08.29, LTV-Q-08.30, LTV-Q-08.31, LTV-Q-08.32, LTV-Q-08.33, LTV-Q-08.34, LTV-Q-08.35, LTV-Q-08.36, LTV-Q-08.37, LTV-Q-08.38, LTV-Q-08.39, LTV-Q-08.40, LTV-Q-08.41, LTV-Q-08.42, LTV-Q-08.43, LTV-Q-08.44, LTV-Q-08.45, LTV-Q-08.46, LTV-Q-08.47, LTV-Q-09.01, LTV-Q-09.82, LTV-Q-09.83, LTV-Q-10.01, LTV-Q-10.65, LTV-Q-11.01, LTV-Q-11.02, LTV-Q-11.03, LTV-Q-11.04, LTV-Q-11.05, LTV-Q-11.06, LTV-Q-11.07, LTV-Q-11.08, LTV-Q-11.09, LTV-Q-11.10, LTV-Q-11.11, LTV-Q-11.12, LTV-Q-11.13, LTV-Q-11.14, LTV-Q-11.15, LTV-Q-11.16, LTV-Q-11.17, LTV-Q-11.18, LTV-Q-11.19, LTV-Q-11.20, LTV-Q-11.21, LTV-Q-11.22, LTV-Q-11.23, LTV-Q-11.24, LTV-Q-11.25, LTV-Q-11.26, LTV-Q-11.27, LTV-Q-11.28, LTV-Q-11.29, LTV-Q-11.30, LTV-Q-11.31, LTV-Q-11.32, LTV-Q-11.33, LTV-Q-11.34, LTV-Q-11.38, LTV-Q-11.39, LTV-Q-11.40, LTV-Q-11.41, LTV-Q-11.42, LTV-Q-11.43, LTV-Q-11.44, LTV-Q-11.45, LTV-Q-11.46, LTV-Q-12.05, LTV-Q-12.06, LTV-Q-12.14, LTV-Q-12.18, LTV-Q-12.19, LTV-Q-12.26, LTV-Q-12.28, LTV-Q-12.29, LTV-Q-12.X-01, LTV-Q-13.01, LTV-Q-13.02, LTV-Q-13.03, LTV-Q-13.04, LTV-Q-13.05, LTV-Q-13.06, LTV-Q-13.07, LTV-Q-13.08, LTV-Q-13.09, LTV-Q-13.10, LTV-Q-13.11, LTV-Q-13.12, LTV-Q-13.13, LTV-Q-13.14, LTV-Q-13.15, LTV-Q-13.16, LTV-Q-13.17, LTV-Q-13.18, LTV-Q-13.19, LTV-Q-13.20, LTV-Q-13.21, LTV-Q-13.22, LTV-Q-13.23, LTV-Q-13.24, LTV-Q-13.25, LTV-Q-13.26, LTV-Q-13.27, LTV-Q-13.28, LTV-Q-13.29, LTV-Q-13.30, LTV-Q-13.31, LTV-Q-13.32, LTV-Q-13.33, LTV-Q-13.34, LTV-Q-13.35, LTV-Q-13.36, LTV-Q-13.37, LTV-Q-13.38, LTV-Q-13.39, LTV-Q-13.40, LTV-Q-13.41, LTV-Q-13.42, LTV-Q-13.43, LTV-Q-13.44, LTV-Q-13.45, LTV-Q-13.46, LTV-Q-13.47, LTV-Q-13.48, LTV-Q-13.49, LTV-Q-13.50, LTV-Q-13.51, LTV-Q-13.52, LTV-Q-13.53, LTV-Q-13.54, LTV-Q-13.55, LTV-Q-13.56, LTV-Q-13.57, LTV-Q-13.58, LTV-Q-13.59, LTV-Q-13.60, LTV-Q-13.61, LTV-Q-13.62, LTV-Q-13.63, LTV-Q-13.64, LTV-Q-13.65, LTV-Q-13.66, LTV-Q-13.67, LTV-Q-13.68, LTV-Q-13.69, LTV-Q-13.70, LTV-Q-13.71, LTV-Q-14.01, LTV-Q-14.02, LTV-Q-14.03, LTV-Q-14.04, LTV-Q-14.05, LTV-Q-14.06, LTV-Q-14.07, LTV-Q-14.08, LTV-Q-14.09, LTV-Q-14.10, LTV-Q-14.11, LTV-Q-14.12, LTV-Q-14.13, LTV-Q-14.14, LTV-Q-14.15, LTV-Q-14.16, LTV-Q-14.17, LTV-Q-14.18, LTV-Q-14.19, LTV-Q-14.20, LTV-Q-14.21, LTV-Q-14.22, LTV-Q-14.23, LTV-Q-14.24, LTV-Q-14.25, LTV-Q-14.26, LTV-Q-14.27, LTV-Q-14.28, LTV-Q-14.29, LTV-Q-14.30, LTV-Q-14.31, LTV-Q-14.32, LTV-Q-14.33, LTV-Q-14.34, LTV-Q-14.35, LTV-Q-14.36, LTV-Q-14.37, LTV-Q-14.38, LTV-Q-14.39, LTV-Q-14.40, LTV-Q-14.41, LTV-Q-14.42, LTV-Q-14.43, LTV-Q-14.44, LTV-Q-14.45, LTV-Q-14.47, LTV-Q-17.01, LTV-Q-17.02, LTV-Q-17.03, LTV-Q-17.04, LTV-Q-17.05, LTV-Q-17.06, LTV-Q-17.07, LTV-Q-17.08, LTV-Q-17.09, LTV-Q-17.10, LTV-Q-17.11, LTV-Q-17.12, LTV-Q-17.13, LTV-Q-17.14, LTV-Q-17.15, LTV-Q-17.16, LTV-Q-17.17, LTV-Q-17.18, LTV-Q-17.19, LTV-Q-17.20, LTV-Q-17.21, LTV-Q-17.22, LTV-Q-17.23, LTV-Q-17.24, LTV-Q-17.25, LTV-Q-17.26, LTV-Q-17.27, LTV-Q-17.28, LTV-Q-17.29, LTV-Q-17.30, LTV-Q-17.31, LTV-Q-17.32, LTV-Q-17.33, LTV-Q-17.34, LTV-Q-17.35, LTV-Q-17.36.

**`AK-L`-Zeilen ohne erkennbaren LTV-Bezug (55):** AK-L-078, AK-L-079, AK-L-091, AK-L-117, AK-L-118, AK-L-126, AK-L-127, AK-L-128, AK-L-129, AK-L-162, AK-L-220, AK-L-225, AK-L-226, AK-L-229, AK-L-231, AK-L-238, AK-L-240, AK-L-241, AK-L-248, AK-L-252, AK-L-255, AK-L-259, AK-L-270, AK-L-271, AK-L-274, AK-L-275, AK-L-277, AK-L-278, AK-L-279, AK-L-286, AK-L-288, AK-L-293, AK-L-298, AK-L-299, AK-L-319, AK-L-321, AK-L-322, AK-L-337, AK-L-338, AK-L-339, AK-L-340, AK-L-341, AK-L-342, AK-L-343, AK-L-344, AK-L-345, AK-L-346, AK-L-347, AK-L-348, AK-L-349, AK-L-350, AK-L-351, AK-L-352, AK-L-354, AK-L-357 (Bezug nennt dort nur `T-n`, `SM-nn`, `M-n`, `DR-nn`, Regeln oder Dokumente).

### 7.4 Stufenangaben, die zwischen `03`, `17` und `18` abweichen

Mechanischer Vergleich: Stufenmengen werden aus der Zelle gelesen (1a und 1b als «1»; K ausgenommen). Ein Treffer heisst nur: keine gemeinsame Stufe; ob die Abweichung ein Fehler ist oder ein beabsichtigter Teilschnitt («1a; E» gegen «1a»), entscheidet die Durchsicht.

**`03` gegen `17` (Anforderung: Stufen von `03` und `AK-L` haben keine gemeinsame Stufe; 10):**

| Anforderung | Stufe in 03 | AK-L | Stufe in 17 |
|---|---|---|---|
| LTV-Q-15.49 | 1a | 310 | K |
| LTV-001.02 | 1a | 312 | K |
| LTV-001.04 | 2 | 312 | K |
| LTV-001.07 | 1a | 312 | K |
| LTV-001.08 | 1a | 312 | K |
| LTV-001.09 | 4 | 312 | K |
| LTV-001.10 | 5 | 312 | K |
| LTV-001.11 | 6 | 312 | K |
| LTV-001.13 | 2 | 312 | K |
| LTV-001.14 | 1a | 312 | K |

**`03` gegen `18` je Anforderung (`18` 4.3 bis 4.12 nennt die Anforderung in der Umfangszeile einer Stufe, die `03` nicht angibt; 0):**

keine.

(Nur ausdrücklich genannte Kennungen der Umfangszeilen sind erfasst; Angaben wie «LTV-004 ausser .06 und .32» sind nicht aufgelöst.)

**`03` gegen `18` 10.1 je Entscheid (`03` nennt eine Stufe, die `18` 10.1 für den Entscheid nicht aufführt; 0):**

keine.

**`17` gegen `18` 10.1 je Entscheid (Stufe eines `AK-L` des Entscheids steht nicht in `18` 10.1; 4):**

| Entscheid | Stufen der AK-L, die 18 10.1 nicht nennt |
|---|---|
| LTV-004 | 4, 6, Apps |
| LTV-008 | 1 |
| LTV-009 | 1, 3 |
| LTV-017 | 3 |

### 7.5 Doppelte Kennungen

| Art | Doppelt |
|---|---|
| Anforderungen und `LTV-X-nn` in `03` | keine |
| `AK-L` in `17` | keine |
| `T-n` in `04` | keine |
| `SM-nn` in `12` | keine |
| Seiten in `14`/`15` | keine |
| Operationen (`operationId`) in `08` | keine |

### 7.6 Weitere Befunde der Zuordnung

- Thema eines Entscheids in `03` (Überschrift) weicht vom Register des Auftrags ab (0): keine Abweichung. Matrix A nennt das Thema des Auftrags.
- Dokumentverweise in `03` (Spalte «Dok») auf Dokumente, die im Ordner nicht vorhanden sind: keine.
- Entscheide ohne `T-n` im Text von `04`: LTV-001, LTV-003, LTV-005, LTV-009, LTV-010, LTV-011, LTV-012, LTV-014, LTV-015, LTV-016, LTV-018, LTV-021.
- Entscheide, die in `14`/`15` Abschnitt 2 keiner Seite zugeordnet sind: LTV-004, LTV-012, LTV-013, LTV-021.
- Entscheide, die kein Absatz in `07` nennt (Tabellen dann nur über Seiten, falls vorhanden): LTV-001, LTV-002, LTV-004, LTV-005, LTV-007, LTV-009, LTV-010, LTV-012, LTV-013, LTV-014, LTV-016, LTV-019, LTV-020, LTV-021.
- Entscheide ohne irgendein `AK-L`: keine.
- Entscheide ohne offene Entscheidung in `20` (keine nennt sie): LTV-001, LTV-013, LTV-016.
- Seiten ohne Entscheid in der Spalte «LTV» (29): LA-01, LA-12, LA-20, LA-21, LA-22, LA-23, LA-24, LA-25, LA-26, LA-27, LA-28, LD-01, LD-02, LD-03, LD-04, LD-05, LD-06, LD-07, LD-08, LD-09, LD-10, LD-11, LS-20, LS-21, LS-22, LS-23, LS-24, LS-25, LS-26.
- Seiten ohne Erwähnung in `19-mockups.md` (0): keine.
- Operationen ohne Seite in der Spalte «Seite» (1): updateLiveConfig.
- `T-n` ohne `LTV-…` im Text (18): T-1, T-3, T-4, T-5, T-9, T-11, T-13, T-14, T-15, T-19, T-20, T-21, T-22, T-23, T-24, T-26, T-27, T-28.
- `SM-nn` ohne `LTV-…` in der Spalte «Bezug» (14): SM-02, SM-09, SM-11, SM-12, SM-19, SM-23, SM-26, SM-30, SM-41, SM-47, SM-48, SM-49, SM-51, SM-55.
- Ausschlüsse `LTV-X-nn` ohne Negativtest (0): keine.
- Alle Dokumente `01`–`24` bis auf `22` (diese Datei) liegen im Ordner; `22` verweist `03` Fassung 3 auf diese Datei.

**Grenzen dieser Matrix.** Die Zuordnung Tabelle, Operation und Seite zu einem Entscheid beruht auf Textnennungen und Seitenverweisen
und ist keine Aussage des Konzepts; was keinen Treffer hat, bleibt leer, auch wenn es fachlich dazugehört. Die Abnahmekriterien
sind nur geprüft, soweit ihre Spalte «Bezug» eine Kennung nennt. Ob ein Test die Anforderung wirklich beweist, prüft dieses
Papier nicht.

