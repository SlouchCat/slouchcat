# Konzept 28 · Änderungen an bestehenden Dokumenten

Stand: 29. November 2026 · Fassung 2 · Status: Entwurf, nicht freigegeben

> **Kurz gesagt.** Marcos Auftrag verlangt in Abschnitt 4, dass alle betroffenen Spezifikationen,
> Architekturentscheide, Pläne, Glossare und Ausschlusslisten widerspruchsfrei an «Live-TV wird ein eigener
> Funktionsbereich» angepasst werden. Diese Datei listet jede Änderung, die schon in der Konzeptphase gemacht ist,
> die Stellen, die bewusst unverändert bleiben, und das, was erst der Bau in Stufe 0 ändert. Überall steht
> derselbe Wortlaut: **Live-TV ist kein Nicht-Ziel mehr (Frage 37, Konzept 28). Stand: konzipiert. «Konzept
> freigegeben» und «Baufreigabe einer Stufe» sind zwei getrennte Schritte; beide stehen aus. Bis zur Baufreigabe
> gilt für Live-TV dasselbe wie für ein Nicht-Ziel.** Keine Stelle erlaubt einem Agenten, Live-TV jetzt zu bauen.

Diese Änderungen betreffen nur Markdown-Dokumente. Kein Code, keine Migration, kein Vertrag, nichts unter
`server/`, `web/`, `android/`, `relay/`, `api/`, `deploy/`, `packages/`.

## Lesart

- **Ein Wortlaut für den Stand** (RG-06; massgeblich ist `CLAUDE.md` Regel 11): «Live-TV ist kein Nicht-Ziel mehr
  (Frage 37, Konzept 28). Stand: konzipiert. «Konzept freigegeben» und «Baufreigabe einer Stufe» sind zwei
  getrennte Schritte; beide stehen aus. Bis zur Baufreigabe gilt für Live-TV dasselbe wie für ein Nicht-Ziel.»
  Der Satz steht wörtlich in Regel 11, ADR 0046, Frage 37, Spec (Non-Goals und Nachtrag), `HANDOVER.md` und
  `CURRENT_STATUS.md`; Plan und Vision geben ihn in ihren Satzbau eingepasst wieder, die kurzen Spec-Nachträge
  (Topologie, Assumptions) verweisen auf ihn.
  - **«Konzept freigegeben»:** Marco nimmt das Konzept an (Nachtrag zu Frage 37). Das erlaubt noch keinen Bau.
  - **«Baufreigabe einer Stufe»:** Marco gibt den Bau einer bestimmten Stufe ausdrücklich frei. Erst dann darf
    für diese Stufe Code entstehen; ADR 0046 wird mit Stufe 0 «angenommen».
- **Marcos Entscheide und Auslegungen getrennt** (RG-08): Marcos Wort sind nur die 21 Grundsatzentscheide und
  E-1 bis E-4. Fire TV «berücksichtigt», Plattform-Streams, kein Download von Aufnahmen, Safari/iPhone/iPad und
  «kein eigener Dienst im Internet» sind Auslegungen des Koordinators; Spec-Nachtrag, Frage 37 und ADR 0046
  führen sie in einem eigenen Absatz, als Fragen in `20-offene-entscheidungen.md`.
- **Alle anderen Nicht-Ziele bleiben wörtlich unverändert** (Musik ausser Webradio, Fotos, Apple TV, webOS,
  Tizen, Fire TV, iOS-Apps, Plugins, Empfehlungen, Trakt-Login, eigene Wunschverwaltung, Steuerung von Arr und
  Seerr, Mandanten, Cloud, YouTube-Downloads). Webradio bleibt ohne Aufnahmen.
- Zeilenangaben gelten für den Arbeitsstand auf dem Zweig `claude/konzept-live-tv`, verglichen mit dem
  Abzweigpunkt `1220769b` von `main`.

## 1. Die Änderungen

Abgeglichen mit `git diff 1220769b -- CLAUDE.md docs/spec docs/planning/01-vision.md docs/planning/03-plan-v1.md
docs/planning/04-entscheidungen.md docs/dev docs/adr` (acht Dateien, 14 Abschnitte des Diffs in 15 Zeilen; jede Änderung steht hier, nichts sonst).

| Nr. | Datei | Stelle | Vorher (kurz) | Nachher (kurz) | Grund |
|---|---|---|---|---|---|
| D-1 | `CLAUDE.md` | Regel 11, Z. 75 | Liste der Nicht-Ziele «… Fotos, Live-TV, TV-Apps …» | «Live-TV» aus der Liste genommen; Fire TV steht weiter in «Apple TV, webOS, Tizen, Fire TV … bleiben Nicht-Ziele» | Nicht-Ziel aufgehoben (A1 S-1, `02` A-1) |
| D-2 | `CLAUDE.md` | Regel 11, Z. 89–101 (neu, nach «Nichts aus einem späteren Meilenstein vorziehen.») | – | der Wortlaut für den Stand (fett); Umfang; «Die Freigabe des Konzepts erlaubt noch keinen Bau»; bis zur Baufreigabe kein Code, keine Migration, kein Vertrag, nichts vorbereiten; bleibende Ausschlüsse aus Auftrag Abschnitt 16; Webradio ohne Aufnahmen; Fire TV bleibt Nicht-Ziel des Repos, Konzept 28 führt die Frage an Marco; übrige Nicht-Ziele unverändert | Regel muss die Ausnahme nennen, ohne einen Bau zu erlauben (LTV-021, RG-06, RG-08) |
| D-3 | `docs/spec/01-anforderungen-v1.md` | Topologie, Z. 29 | «Musik, Fotos, Live-TV zurückgestellt (Runde 4)» | unverändert, dazu kursiv der Wortlaut für den Stand und Verweis auf den Nachtrag am Ende | Muster Nachtrag in Z. 35 (A1 S-4) |
| D-4 | `docs/spec/01-anforderungen-v1.md` | Constraints, Z. 43 | «Genau ein Client in Version 1: Web-App als PWA …» | unverändert, dazu kursiv Nachtrag: seit 21. November 2026 Android-Handy-App und Android-TV-App (Fragen 34, 35) | Altlast W-10 |
| D-5 | `docs/spec/01-anforderungen-v1.md` | Constraints, Z. 44 | «Architektur hält Musik, Fotos und Live-TV offen, baut sie aber nicht.» | unverändert, dazu kursiv der Wortlaut für den Stand; Live-TV ist ein eigener Bereich, kein Medientyp der Bibliothek; Musik und Fotos bleiben offen und ungebaut | A1 S-3, Muster Z. 46 |
| D-6 | `docs/spec/01-anforderungen-v1.md` | Non-Goals, Z. 56–62 | «… Ausnahme seit 6. November 2026: Live-Webradio …; Musikbibliothek und DVR bleiben Nicht-Ziele.» | erster Satz unverändert; «Musikbibliothek und **Aufnahmen von Webradio** bleiben Nicht-Ziele»; neu «Ausnahme seit 28. November 2026: Live-TV mit EPG, Timeshift und DVR für Fernsehsender aus IPTV-Quellen (ADR 0046 vorgeschlagen)», dann der Wortlaut für den Stand; Verweis auf den Nachtrag; «Fotos bleiben Nicht-Ziel» | A1 S-2; löst den inneren Widerspruch «DVR bleiben Nicht-Ziele» auf |
| D-7 | `docs/spec/01-anforderungen-v1.md` | K-3, Z. 102 | «Ein Profil sieht Inhalte über seiner Altersstufe nirgends …» | unverändert, dazu kursiv: vorgesehene benannte Ausnahme für Live-TV, Verweis auf den Nachtrag; für Titel gilt K-3 unverändert | sonst widerspräche K-3 allein gelesen `02` A-2 |
| D-8 | `docs/spec/01-anforderungen-v1.md` | Assumptions, Z. 193 | «… Musik, Fotos, Live-TV später» | unverändert, dazu «(Nachtrag 28. November 2026: Live-TV ist kein Nicht-Ziel mehr, Frage 37; konzipiert, Konzept- und Baufreigabe stehen aus; siehe Nachtrag am Ende)» | Muster der Zeile «Entdecken = Startseite plus Suche» («seit Konzept 20 Stufe 3 …») |
| D-9 | `docs/spec/01-anforderungen-v1.md` | neuer Abschnitt am Ende, Z. 388–457 | – | «Nachtrag vom 28. November 2026 – Live-TV als eigener Funktionsbereich»: Marcos Entscheid und der Wortlaut für den Stand; Umfang in fünf Zeilen; was ausgeschlossen bleibt (Auftrag Abschnitt 16, Webradio, übrige Non-Goals samt Fire TV); **Auslegungen des Koordinators** (Fire TV, Plattform-Streams, Download von Aufnahmen, Safari/iPhone/iPad, kein Internetdienst); benannte Ausnahmen A-1 bis A-13 in sechs Punkten (K-3/AN-1 mit Verweis auf E-28-23 zur Suche, private Aufnahmen samt A-12, ADR 0032 und B-1, Heimnetz mit festen Einzelzielen, ADR 0025 und Regel 3 mit A-10, A-11, A-13, ADR 0045); Verweise | Muster «Nachtrag vom 22. Oktober 2026 – Runde 20»; Kriterien L-n kommen mit Stufe 0 |
| D-10 | `docs/planning/03-plan-v1.md` | Abschnitt 1, Z. 30 | «Nicht eingeplant (Spec "Non-Goals"): Musik, Fotos, Live-TV/DVR, TV-Apps …» | «Musik (Ausnahme seit 6. November 2026: Live-Webradio ohne Aufnahmen, Frage 23, Konzept 16, ADR 0025)»; «Live-TV/DVR (seit 28. November 2026 kein Nicht-Ziel mehr … Stand: konzipiert, nicht eingeplant; «Konzept freigegeben» und «Baufreigabe einer Stufe» … beide stehen aus; bis zur Baufreigabe … wie für ein Nicht-Ziel; ausgeschlossen bleiben …)» | A1 S-5; Altlast W-10 (Webradio fehlte) |
| D-11 | `docs/planning/01-vision.md` | neuer Abschnitt am Ende, Z. 81–91 | – | «Nachtrag vom 28. November 2026»: Vision bleibt; seither gehören dazu Webradio (Frage 23), Android-Handy- und TV-App mit den zwei Diensten im Internet (Fragen 34, 35), Live-TV (Frage 37) mit dem Wortlaut für den Stand; geltende Fassung in Spec und Regel 11 | A1 H-2; Altlast W-10. Der Text der Vision (Z. 70) bleibt unverändert |
| D-12 | `docs/dev/HANDOVER.md` | Z. 155–159 | «Nicht-Ziele: Musik, Fotos, Live-TV, native Apps, Plugins, algorithmische Empfehlungen.» | «Musik (ausser Live-Webradio, Frage 23), Fotos, native Apps (ausser der Android-Handy-App und der Android-TV-App, Fragen 34 und 35), Plugins, algorithmische Empfehlungen.» und der Wortlaut für den Stand | A1 S-6; Altlast W-10 |
| D-13 | `docs/planning/04-entscheidungen.md` | neue Zeile 143 (Tabelle «Weitere Entscheide während der Umsetzung») | – | **Frage 37** im Stil der Fragen 33 bis 35: der Wortlaut für den Stand; Ausnahme vom Nicht-Ziel, löst den Teilsatz aus Frage 23 ab; **Von Marco entschieden:** 21 Grundsatzentscheide, E-1 bis E-4; **Auslegungen des Koordinators, keine Entscheide Marcos** (Fire TV, Plattform-Streams, Download von Aufnahmen, Safari/iPhone/iPad, kein Internetdienst); A-1 bis A-13 je in einem Satz (A-3 als feste Einzelziele); T-29 mit Grenze (entfernte oder geänderte Quelle beendet ihre Empfänger); offene Fragen als Verweis auf `20-offene-entscheidungen.md` | Auftrag Abschnitt 4; `02` Abschnitt 5; RG-06, RG-08, RT-09 |
| D-14 | `docs/adr/0046-live-tv-als-eigener-bereich.md` | neu | – | ADR «vorgeschlagen» mit dem Wortlaut für den Stand und dem Satz «Das Nicht-Ziel ist mit Frage 37 schon aufgehoben, der Bau ist es nicht»; Kontext; Entscheid (eigener Bereich, Suche der Bibliothek an E-28-23 gebunden, Marcos Entscheide gegen Auslegungen getrennt, ein Dienst im Serverprozess, Rechte, Verhältnis zu ADR 0001, 0003, 0025, 0032, 0041, 0045); A-1 bis A-13 mit Grenze; Verworfen; Folgen; abgelöster Spec-Wortlaut. T-1 bis T-29 nur als Verweis | Auftrag Abschnitt 4 «Architekturentscheidungen»; RG-06, RT-09 |
| D-15 | `docs/dev/CURRENT_STATUS.md` | neuer Abschnitt oben, Z. 3–21 | – | «Konzept 28 «Live-TV»: Entwurf, nichts gebaut, nichts freigegeben»: Auftrag, der Wortlaut für den Stand, Zweig, Ordner, Mockups, nachgeführte Dokumente, nächster Schritt (Prüfung, Entscheide, Freigabe des Konzepts, danach als eigener Schritt die Baufreigabe je Stufe) | Auftrag Abschnitt 20 («`CURRENT_STATUS.md` entsprechend den bestehenden Arbeitsregeln») |

Nicht geändert, weil dort Live-TV weder als ausgeschlossen noch als Idee steht (geprüft mit der Suche aus
Abschnitt 5): `docs/spec/README.md`, `docs/ideas/README.md`, `README.md` im Wurzelordner. Einen ADR-Index gibt
es in `docs/adr/` nicht; nichts nachzuführen. Ein Glossar gibt es nur als «Ontology» der Spec; Live-TV kommt dort
nicht vor, die Begriffe stehen in `03-fachkonzept.md` Abschnitt 1.3 und kommen mit Stufe 0 (Kriterien L-n) in
die Spec.

## 2. Geschichtlich, unverändert

Diese Stellen sind Belege eines früheren Standes (Interview, abgeschlossene Konzepte, angenommene ADR, frühere
Fragen). Das Repo schreibt sie nicht rückwirkend um, sondern löst sie mit einem neuen Eintrag ab (Vorbild: Frage 36
löst F-5 von Konzept 25 ab; ADR 0037 «Spec-Wortlaut, der hier abgelöst wird»). Abgelöst werden sie durch Frage 37,
ADR 0046 und die Nachträge der Spec.

| Datei:Zeile | Wortlaut (kurz) | Warum unverändert |
|---|---|---|
| `docs/spec/01-anforderungen-v1.md:291` | Runde 4: «Musik, Fotos, Live-TV später» | Interview-Protokoll; der Nachtrag am Ende und D-8 genügen (A1 H-1) |
| `docs/planning/01-vision.md:70` | «Musik, Fotos, Live-TV und DVR. TV-Apps und native Mobile-Apps.» | Vision von Version 1; abgelöst durch den Nachtrag D-11 (A1 H-2) |
| `docs/planning/04-entscheidungen.md:16` (Frage 7) | «Live-TV zurückgestellt … wird nach CLAUDE.md Regel 11 nicht gebaut» | Antwort vom Oktober 2026; abgelöst durch Frage 37 |
| `docs/planning/04-entscheidungen.md:90` (Frage 23) | «Musikbibliothek, Fotos, Live-TV und DVR bleiben Nicht-Ziele» | Frage 37 nennt ausdrücklich, dass sie diesen Teilsatz für Live-TV und Aufnahmen ablöst (A1 H-3) |
| `docs/planning/16-webradio.md:65`, `:693` | «Live-TV und DVR bleiben Nicht-Ziele»; «Aufnahmen … (kein DVR)» | Konzept 16 abgeschlossen; für Radio gilt F-1 «keine Aufnahmen» weiter (A1 H-4) |
| `docs/planning/13-abspielseite.md:63`, `:81`, `:205` | «Läuft gerade» und «TV Guide» gehören zu Live-TV, Non-Goal | Konzept 13 abgeschlossen; betrifft die Abspielseite der Bibliothek, nicht den Live-TV-Bereich (A1 H-5) |
| `docs/planning/25-medienwuensche/11-entscheide-und-adr.md:960` | Entwurf der alten Regel 11 mit «Live-TV» | Entwurf von damals (A1 H-6) |
| `docs/adr/0025-webradio-katalog-und-weiterleitung.md:3-4` | hebt nur das Nicht-Ziel «Musik» für Live-Webradio auf | angenommener ADR; ADR 0046 nennt ihn als Vorbild und, was für Live-TV fortgilt (A-10); Nachtrag kommt mit Stufe 0 (A1 H-7) |

## 3. Was erst der Bau ändert (Stufe 0)

Nach `18-umsetzungsplan.md` Abschnitt 3 und `12-sicherheit.md` Abschnitt 5.4. Diese Änderungen setzen Regeln für
Code, der noch nicht entstehen darf; sie kommen erst mit Marcos Baufreigabe von Stufe 0.

| Was | Wo | Inhalt | Quelle |
|---|---|---|---|
| Regel 8 für die TV-App | `CLAUDE.md` Regel 8 | Steuerkreuz-Test je TV-Seite, falls die TV-App Live-TV in den Stufen 1 bis 6 bekommt | `18` 0.2; `03` F-6; A1 RI-7 |
| Regel 10, Redaction-Liste | `CLAUDE.md` Regel 10, `server/internal/logging/`, Test, `com.slouchcat.core.api.log.Log` | neue Abfrageparameter (`lt`, `username`, `password`, `u`, `p` …), Endungsregel, Pfadmuster für Benutzerteil und Xtream-Pfade, Schlüsselnamen (`sourceurl`, `streamurl`, `epgurl` …), zusammengesetzte Werte | `12` 5.4, SM-17 |
| Regel 3, Hülle für FFmpeg (A-11) | `CLAUDE.md` Regel 3, `forbidigo` | `execve` (`syscall.Exec`/`unix.Exec`) für die Hülle nur im Paket `transcode` | `18` 0.2, 0.5; `04` 10.8; `12` O-1 |
| Regel 2, Riegel | `CLAUDE.md` Regel 2, `server/.golangci.yml` | `store/liveq` nur im Paket `live`; Paket `live` ohne `items`, `media_files`, `tracks` | `18` 0.2, 0.5 |
| Compose-Härtung | `deploy/compose.yaml`, `docs/INSTALLATION.md`, `deploy/README.md` | `no-new-privileges`, `cap_drop: ALL`, `pids_limit`, Grenze offener Dateien; später Zusatzdateien für Stufe H (A-9) | `18` 0.6; `04` F-12 |
| Installationsanleitung | `docs/INSTALLATION.md`, `deploy/README.md` | Speicherorte für Aufnahmen und Puffer, Zusatzdateien, Folgen von A-9, Hinweis «Eindämmung nicht verfügbar» | `18` 3.2, Stufe H; `12` F-1 |
| Übersetzungen | `web/src/locales/*.json`, Namensraum `live` | alle Texte des Bereichs in `de`, `en`, `de-DE` | Regel 9; Leitplanken |
| Kriterien L-n in der Spec | `docs/spec/01-anforderungen-v1.md` | Baustein L mit Rückverfolgung zu LTV-nnn | `18` 0.2; A1 3.2 |
| ADR «angenommen» und ADR zu den technischen Entscheiden | `docs/adr/0046-…` (Status), weitere ADR je Stufe; Nachträge zu ADR 0025, 0029, 0032, 0045 | T-1 bis T-29 aus `04-architektur.md` je mit der Stufe, die sie baut | `18` 0.3 |
| `x-clients` und Vertragsfassung | `api/openapi.yaml`, `docs/dev/VERTRAEGLICHKEIT.md` | neue Operationen mit `x-visibility` und `x-clients`; Vertrag ab 1.45.0, Migration ab 00087 (Nummern vergibt der Bau) | Regel 7; `08-api.md` |

## 4. Bereinigte Altlasten

Aus A1 Abschnitt 6.1, W-10: Diese Stellen waren schon vor Live-TV widersprüchlich und sind beim Nachführen mit
erledigt.

| Altlast | Stelle | Bereinigt durch |
|---|---|---|
| Plan nennt die Webradio-Ausnahme nicht | `docs/planning/03-plan-v1.md:30` | D-10 |
| Handover nennt «native Apps» als Nicht-Ziel, Webradio fehlt | `docs/dev/HANDOVER.md:155` | D-12 |
| Vision nennt alle alten Nicht-Ziele ohne die Ausnahmen | `docs/planning/01-vision.md:70` | D-11 (Nachtrag, Text unverändert) |
| Spec «Genau ein Client in Version 1» ohne die Android-Apps | `docs/spec/01-anforderungen-v1.md:43` | D-4 |
| Spec «Musikbibliothek und DVR bleiben Nicht-Ziele» | `docs/spec/01-anforderungen-v1.md:57` | D-6 |

## 5. Prüfung auf Widerspruchsfreiheit

**Abgleich mit dem Diff** (29. November 2026): `git diff 1220769b` über `CLAUDE.md`, `docs/spec`,
`docs/planning/01-vision.md`, `docs/planning/03-plan-v1.md`, `docs/planning/04-entscheidungen.md`, `docs/dev` und
`docs/adr` zeigt genau acht Dateien mit 14 Abschnitten; jede Änderung ist in einer der Zeilen D-1 bis D-15 erfasst, und keine Zeile nennt eine
Stelle, die nicht geändert ist. Andere Dateien in `docs/dev` und `docs/adr` sind auf dem Zweig nicht geändert
(Änderungen dort auf `main` nach dem Abzweig gehören nicht zu Konzept 28).

**Suche:** `grep -rniE 'live-?tv|iptv|dvr|epg' docs CLAUDE.md README.md`, ohne `docs/planning/28-live-tv/` und
ohne `mockups/`.

**Ergebnis:** Keine geltende Stelle nennt Live-TV, IPTV, EPG oder DVR mehr als ausgeschlossen, und alle Stellen
D-1 bis D-15 nennen denselben Stand («kein Nicht-Ziel mehr, konzipiert, Konzept- und Baufreigabe stehen aus, bis
zur Baufreigabe wie ein Nicht-Ziel»). Die übrigen Treffer sind die geschichtlichen Stellen aus Abschnitt 2 und
Fremdtreffer ohne Bezug: `depguard` (enthält «epg») in ADR, Plan und Prüfberichten; `DVRServer`, `DVRSettings` als
Namen der Seerr-Schnittstelle in Konzept 25; `timeShiftSeconds` des Webradios in `CURRENT_STATUS.md`.

**Inhaltliche Gegenprobe:**

- Keine der geänderten Stellen sieht einen **Schalter oder eine Zahl je Quelle** für Verbindungen oder Vorladen
  vor (LTV-017), und keine sagt, ein **Dauersender** weiche einem neuen Zuschauer (LTV-002).
- **Fire TV** steht in Regel 11 und in den Spec-Non-Goals weiter als Nicht-Ziel; Frage 37, ADR 0046 und der
  Spec-Nachtrag führen «berücksichtigt» als Auslegung mit offener Frage an Marco, nicht als Entscheid.
- **Feste Ziele im Heimnetz** stehen überall als Einzelziele (Adresse mit Port; kleines Netz nur mit zweiter
  Bestätigung), wie `12` Abschnitt 1.1.

**Noch offen, ausserhalb dieser Datei:** `02-grundsatzentscheide.md` Abschnitt 4 bekommt A-10 bis A-13 und die
Einzelziele für A-3 (RT-09; Frage 37, ADR 0046 und Spec-Nachtrag nennen sie schon). Bis dahin gelten für deren
Grenzen `12-sicherheit.md` Abschnitt 1.1 und `05` Abschnitt 2.6.
