# Konzept 28 · Agent-Einsatz und unabhängige Reviews

Stand: 29. November 2026 · Fassung 1 · Status: Entwurf, nicht freigegeben

Dieses Dokument hält fest, wie das Konzept entstanden ist: welche Agents mit welchem Modell welche Aufgabe
hatten, was unabhängig geprüft wurde, was die Prüfer fanden und wie es behoben wurde. Geschrieben hat es der
Koordinator (die Hauptsitzung), nicht ein Agent.

## 1. Umgebung

| Punkt | Befund |
|---|---|
| OMC (oh-my-claudecode) | Plugin `oh-my-claudecode@omc` in Version **5.8.4** (im Cache liegt daneben noch 5.7.5) |
| OMC-Agents | 19 Typen vorhanden: `analyst`, `architect`, `code-reviewer`, `code-simplifier`, `critic`, `debugger`, `designer`, `document-specialist`, `executor`, `explore`, `git-master`, `planner`, `qa-tester`, `scientist`, `security-reviewer`, `test-engineer`, `tracer`, `verifier`, `writer` |
| Modelle laut Agent-Definition | Opus: analyst, architect, code-reviewer, code-simplifier, critic, planner, security-reviewer · Sonnet: debugger, designer, document-specialist, executor, git-master, qa-tester, scientist, test-engineer, tracer, verifier · Haiku: explore, writer |
| Modelle, die sich wählen liessen | `fable`, `opus`, `sonnet`, `haiku` je Aufruf |
| Aufwand (Reasoning) | je Aufruf einstellbar; für Architektur, Sicherheit und die Reviews auf «hoch» gesetzt |
| Parallelität | Agents laufen im Hintergrund nebeneinander und melden sich beim Ende |
| Projekteigene Agents | bei Beginn keine. Am 28. November 2026 kamen auf `main` sieben Design-Agents dazu (`.claude/agents/`); sie waren in dieser Sitzung noch nicht geladen und wurden nicht eingesetzt |
| Koordinator | Hauptsitzung, Claude Fable 5.2 |

Die OMC-Abläufe (`ralplan`, `autopilot`, `team`) wurden nicht benutzt; der Koordinator hat die Agents direkt
gestartet, damit zwischen den Phasen entschieden werden konnte.

## 2. Verantwortungsbereiche und eingesetzte Agents

«Modell» nennt, was beim Start gewählt oder gemeldet wurde. Wo kein Modell angegeben wurde, meldete die
Umgebung «inherit»: Ob dann das Modell der Agent-Definition oder das der Hauptsitzung lief, **liess sich nicht
feststellen**; die Spalte nennt für diese Fälle das Modell der Definition mit dem Zusatz «(Definition)».

### 2.1 Analyse und Recherche

| Verantwortung | Agent (OMC-Typ) | Modell | Aufgabe | Ergebnis |
|---|---|---|---|---|
| Analyst | `analyst` | Opus (Definition) | Spezifikationen, Regeln, Nicht-Ziele, Kennungen, Annahmen des Auftrags gegen das Repo | Rohbericht A1, eingeflossen in `01`, `02` |
| Architect (Ist) | `architect`, Aufwand hoch | Opus (Definition) | Medienkette des Servers: Wiedergabe, Transcoding, Radio-Relay, Netzschutz, Betrieb | Rohbericht A2 → `01` |
| Architect (Ist) | `architect` | Opus (Definition) | Plattform: Konten, Rechte, Sichtbarkeit, Datenbank, Jobs, Benachrichtigungen, API | Rohbericht A3 → `01` |
| UI/UX (Ist) | `designer` | Opus | Web, Handy-App, TV-App, Bauteile, Mockup-Baukästen | Rohbericht A4 → `01`, `14` |
| Researcher | `document-specialist` | Opus | Stream-Sharing, Fast Zapping, FFmpeg im Dauerlauf | R1 → `05`, `24` |
| Researcher | `document-specialist` | Sonnet (Definition) | Client-Kompatibilität der Auslieferung | R2 → `16`, `24` |
| Researcher | `document-specialist` | Sonnet (Definition) | Eingangsquellen, M3U, Xtream, Protokolle | R3 → `05`, `24` |
| Researcher | `document-specialist` | Sonnet (Definition) | EPG und XMLTV | R4 → `09`, `24` |
| Researcher | `document-specialist` | Opus | Timeshift und DVR | R5 → `10`, `11`, `24` |
| Researcher (Nachprüfung) | `document-specialist` | Sonnet (Definition) | tragende Aussagen an Primärquellen nachprüfen | `anhang/nachpruefung-quellen.md` |

Der Koordinator hat selbst gemessen, was das FFmpeg im Image kann (Version 7.1.4-Jellyfin, Protokolle, Demuxer,
Deinterlacer): das ist die einzige Messung der Konzeptphase.

### 2.2 Konzeptdokumente

| Verantwortung | Agent (OMC-Typ) | Modell | Dokumente |
|---|---|---|---|
| Documentation | `writer` | Sonnet | `01-bestandsanalyse.md`, `24-recherche-und-quellen.md`, `16-client-kompatibilitaet.md` |
| Fachkonzept | `executor` | Opus | `02-grundsatzentscheide.md`, `03-fachkonzept.md` |
| Architect, Streaming Specialist | `executor`, Aufwand hoch | **Fable** | `04-architektur.md`, `05-streaming-und-fast-zapping.md` (vier Fassungen) |
| Architect (Abläufe) | `executor` | Opus | `06-ablaeufe.md` (22 Diagramme) |
| Database und API Specialist | `executor`, Aufwand hoch | Opus | `07-datenmodell.md`, `08-api.md` |
| EPG | `executor` | Opus | `09-epg.md` |
| DVR Specialist | `executor` | Opus | `10-timeshift.md`, `11-dvr.md` |
| Security (Autor) | `executor`, Aufwand hoch | Opus | `12-sicherheit.md` |
| Performance Specialist | `executor` | Opus | `13-ressourcen-und-performance.md` |
| UI/UX Designer | `designer` | Opus | `14-oberflaeche.md`, `15-administration.md` |
| QA Specialist | `test-engineer` | Sonnet (Definition) | `17-test-und-abnahme.md` |
| Planner | `planner` | Opus (Definition) | `18-umsetzungsplan.md` |
| Documentation | `executor` | Opus | `20-offene-entscheidungen.md`, `21-aenderungen-an-dokumenten.md` samt den Änderungen an Regel 11, Spec, Plan, Vision, Frage 37, ADR 0046 |
| Documentation | `executor` | Sonnet | `22-anforderungsmatrix.md` (per Skript aus `03` und `17` erzeugt) |
| Bereinigung | `executor` | Opus | Reste zwischen den Dokumenten nach der ersten Konsolidierung |
| Orchestrator | Hauptsitzung | Fable 5.2 | `README.md`, dieses Dokument, Leitplanken und Entscheide zwischen den Phasen, alle Commits |

### 2.3 Mockups

| Verantwortung | Agent (OMC-Typ) | Modell | Aufgabe |
|---|---|---|---|
| Mockup Developer (Gerüst) | `designer` | Opus | Gerüst für drei Clients, gemeinsame Beispieldaten, Zustandsleiste, vier Musterseiten; später Zusammenführung, Korrekturrunden, `19-mockups.md` |
| Mockup Developer | `designer` | Sonnet | Web Zuschauer A |
| Mockup Developer | `designer` | Opus | Web Zuschauer B (Programmzeitschrift, Live-Player, Aufnahmen, Dialoge) |
| Mockup Developer | `designer` | Sonnet | Web Verwaltung A und B (zwei Läufe) |
| Mockup Developer | `designer` | Sonnet | Android TV |
| Mockup Developer | `designer` | Sonnet | Android-Handy-App |
| Hilfsaufgabe | `general-purpose` (kein OMC-Typ) | Sonnet | 16 Senderlogos von Wikimedia Commons lokal ablegen |

### 2.4 Unabhängige Reviews

Kein Prüfer hat an dem Dokument mitgeschrieben, das er prüfte.

| Nr. | Gegenstand | Agent (OMC-Typ) | Modell | Bericht |
|---|---|---|---|---|
| V1 | Ist-Analyse A1 bis A4 gegen den Code | `verifier` | Sonnet (Definition) | Rohbericht V1, Vermerk in `01` |
| 1 | Architektur und Streaming, Fassung 1 | `architect`, Aufwand hoch | Opus (Definition) | `anhang/reviews/review-1-architektur.md` |
| 1 | Sicherheit des Entwurfs, Bedrohungsanalyse | `security-reviewer`, Aufwand hoch | Opus (Definition) | `anhang/reviews/review-1-sicherheit.md` |
| 2 | Datenmodell und API | `code-reviewer`, Aufwand hoch | Opus (Definition) | `anhang/reviews/review-2-datenmodell-api.md` |
| 3 | Mockups gegen Auftrag Abschnitt 13 und `14`/`15` | `verifier` | Sonnet | `anhang/reviews/review-3-mockups.md` |
| 4 | Gesamtreview aller Dokumente | `critic`, Aufwand hoch | Opus (Definition) | `anhang/reviews/review-4-gesamt.md` |
| 5 | Architektur, Zweitprüfung nach den Korrekturen | `architect`, Aufwand hoch (anderer Lauf) | Opus (Definition) | `anhang/reviews/review-5-architektur-zweitpruefung.md` |
| 6 | Sicherheit, Zweitprüfung nach den Korrekturen | `security-reviewer`, Aufwand hoch (anderer Lauf) | Opus (Definition) | `anhang/reviews/review-6-sicherheit-zweitpruefung.md` |
| Q | Rechercheaussagen an Primärquellen | `document-specialist` (anderer Lauf) | Sonnet (Definition) | `anhang/nachpruefung-quellen.md` |

Der Koordinator hat zusätzlich die Mockups selbst im Browser angesehen (Web, Verwaltung, TV, Handy) und den
Wortlaut von Regel 11 gegengelesen.

## 3. Was die Reviews fanden und wie es behoben wurde

| Review | Urteil | Befunde | Die wichtigsten | Behoben in |
|---|---|---|---|---|
| V1 Ist-Analyse | taugt als Grundlage | 100 Aussagen: 95 bestätigt, 5 ungenau, 0 falsch; 5 Lücken nachgetragen | Segment-Token hängt an Titeln; Tageslimit hängt an Titeln; Shutdown in 10 s | `01` (Prüfvermerk) |
| 1 Architektur | Fassung 1 nicht freigabereif | 26: 1 kritisch, 9 hoch | Umschalten konnte eine Quelle sperren, wenn der Anbieter nur eine Verbindung erlaubt; LTV-007 war umgedeutet; Direktweg im Browser ist die Ausnahme; Timeshift-Recht nicht im Server durchgesetzt | `04`/`05` Fassung 2 (Protokoll in `04` Abschnitt 13) |
| 1 Sicherheit | Risiko hoch | 24: 0 kritisch, 6 hoch; 5 stillschweigende Lockerungen | Freigabe lokaler Ziele zu weit; FFmpeg-Weg für DASH nicht dicht; FFmpeg mit vollem Zugriff auf Geheimnisse; ein kaputtes Paket konnte den Server beenden; Zugangsdaten im Klartext | `04`/`05` Fassung 2, `12` |
| Q Quellen | – | 45 Aussagen: 26 bestätigt, 13 teilweise, 2 widerlegt, 3 nicht prüfbar | Apple: HEVC nur in fMP4, mindestens 6 Segmente; hls.js-Light-Bau kann HEVC und AC-3 in TS nicht; Media3-Startpuffer 1000 ms; Landlock-Bibliothek fällt still zurück | `04`/`05` Fassung 3, `16`, `24` Fassung 2 |
| 2 Datenmodell/API | überarbeiten | 30: 1 kritisch, 7 hoch | Profil löschen bei laufender Aufnahme konnte alle Schreibvorgänge blockieren; Wiedererkennung beim Import ohne zweites Merkmal; Import nicht transaktional; Verwaltung zeigte Konto und Sender laufender Aufnahmen | `07`/`08` Fassung 2 (Tabelle je Befund in `07` Abschnitt 13) |
| 3 Mockups | bereit mit Korrekturen | 16: 2 hoch, 3 mittel | Schalter je Quelle berührte LTV-017; Profil ohne Live-TV sah Hinweistexte | Korrekturrunde der Mockups, `19` Fassung 2 |
| 4 Gesamtreview | übergabereif nach Korrekturen | 22: 3 kritisch, 5 hoch | «ohne Antwort gilt die Empfehlung» stand in zwei Dokumenten; Dauersender wich neuen Zuschauern (LTV-002); Schalter je Quelle war ein Verbindungskontingent (LTV-017); Auslegungen des Koordinators standen als Entscheide des Auftraggebers | Entscheide des Koordinators, `04`/`05` Fassung 4, Schlussbereinigung aller Dokumente und Mockups |
| 5 Architektur, Zweitprüfung | zur Vorlage geeignet nach Korrekturen | 21: 0 kritisch, 6 hoch; RA-01 bis RA-10: 7 behoben, 3 teilweise | Ring zu knapp für lange Bildgruppen; Startfrist widersprach sich; Anbieter, die die ältere Verbindung trennen; deaktivierter Sender lief unbegrenzt weiter | `04`/`05` Fassung 4 |
| 6 Sicherheit, Zweitprüfung | Risiko mittel (vorher hoch) | 21: 0 kritisch, 1 hoch; RS-01 bis RS-24: 20 geschlossen, 4 teilweise | ohne Landlock hätte FFmpeg Live-Daten ungeschützt umgewandelt; Schauzeit hing an der Angabe des Clients | `12` Fassung 3, `04`/`05` Fassung 4 |

Dazu fanden die Autoren beim Ausarbeiten selbst Lücken in der Architektur (12 beim Zeichnen der Abläufe, 6 in
der Ressourcenrechnung, 8 im Sicherheitskonzept, 14 Widersprüche beim Datenmodell). Sie sind in der Architektur
Fassung 3 und in zwei Konsolidierungsrunden bereinigt worden.

**Nach den Schlussreviews wurde nicht ein drittes Mal geprüft.** Die Fassung 4 der Architektur, die Fassung 3
des Sicherheitskonzepts und die Schlussbereinigung der übrigen Dokumente sind von ihren Autoren gegen die
Befunde abgearbeitet und vom Koordinator stichprobenweise gegengelesen, aber von keinem unabhängigen Agent
erneut beurteilt. Was die Reviews als «vor einer Baufreigabe zusätzlich nötig» nennen, steht in
`20-offene-entscheidungen.md` und `18-umsetzungsplan.md`.

## 4. Entscheide des Koordinators zwischen den Phasen

Der Koordinator hat nur dort entschieden, wo der Auftrag oder die Regeln des Repos die Antwort geben; alles
andere steht als Frage an den Auftraggeber in `20`.

| Anlass | Entscheid |
|---|---|
| Auftrag nennt das alte Repo und dessen Stack | Rückfrage an Marco: Konzept gilt für das Go-Produkt (seine Antwort E-1) |
| Umfang der Mockups | Rückfrage: Web, Android TV und Handy-App (E-2) |
| Aufnahmen von Kinderprofilen, volles Kontingent | Rückfrage: zwei Antworten (E-3, E-4) |
| Review 1 | Wo der Entwurf vom Wortlaut eines Grundsatzentscheids abwich, wurde daraus eine Frage; bis zur Antwort gilt der Wortlaut |
| Review 3, Review 4 | Kein Schalter und keine Zahl je Quelle (LTV-017); Vorladen nur global und je Sender |
| Review 4 | Ohne Antwort gilt überall die wortlauttreue Vorgabe; laufender Dauersender weicht nicht (LTV-002); Auslegungen (Fire TV, Plattform-Streams, kein Download von Aufnahmen, Safari/iPhone) werden Fragen |
| Review 6 | Live-Umwandlung nur mit wirksamer Sperre als Vorgabe; Schauzeit zählt, was der Server ausliefert |

## 5. Einschränkungen und Abweichungen

- **Modell bei «inherit» nicht feststellbar** (Abschnitt 2).
- **Agents ohne Schreibwerkzeug** (`analyst`, `architect`, `critic`, `code-reviewer`, `security-reviewer`,
  `document-specialist`, `verifier`) haben ihre Berichte über die Shell in Dateien geschrieben; sonst haben sie
  nichts verändert.
- **Recherche:** Die ersten fünf Rechercheberichte riefen nur einen Teil ihrer Quellen wirklich ab (69
  abgerufen, 36 nicht) und stützten sich sonst auf Vorwissen. Darum die Nachprüfung Q. Was auch sie nicht prüfen
  konnte, steht in `24` als Annahme.
- **Gemessen wurde fast nichts.** Alle Zielwerte, Kapazitäten und Kompatibilitäten sind Rechnungen oder
  Annahmen; die Messungen stehen als Versuche der Stufe V in `18` und `20`.
- **Keine Geräteprüfung:** Die Mockups wurden in Chromium geprüft (auch über `file://`), nicht in Firefox oder
  Safari, nicht auf echten Geräten und nicht mit Bildschirmlesern. Ein Apple-Gerät fehlt.
- **Diagramme:** Die Mermaid-Diagramme wurden mit einem Prüfskript auf Syntax geprüft, nicht gerendert.
- **Sitzungsabbruch:** Am 28. November lief die Anmeldung ab; zwei Agents wurden mitten in der Arbeit
  unterbrochen und danach auf ihrem Zwischenstand fortgesetzt. Verloren ging nichts.
- **Nicht eingesetzte OMC-Agents:** `explore`, `scientist`, `debugger`, `tracer`, `qa-tester`, `git-master`,
  `code-simplifier` (für eine Konzeptphase ohne Code nicht nötig).
- **Grenze je Lauf:** Mehr als acht Agents liefen nie gleichzeitig; Bau- und Testläufe gab es keine.

## 6. Implementierungsverbot

Kein Agent hat Produktivcode, Migrationen, den API-Vertrag, Übersetzungen oder die Betriebsdateien verändert.
Geändert wurden nur Dokumente unter `docs/`, `CLAUDE.md` (Regel 11) und die Mockups unter
`docs/planning/mockups/28-live-tv/`. Die Liste steht in `21-aenderungen-an-dokumenten.md`.
