# ADR 0046: Live-TV als eigener Bereich

- Status: **vorgeschlagen** (28. November 2026, Konzept 28, noch nicht freigegeben). **Hebt mit der Freigabe für
  Live-TV das Nicht-Ziel «Live-TV und DVR» auf** (Spec «Non-Goals», `CLAUDE.md` Regel 11) und nennt die
  Ausnahmen von ADR 0025, ADR 0032 und ADR 0045, die Live-TV braucht. Angenommen wird es erst mit Stufe 0 des
  Baus, nach Marcos ausdrücklicher Baufreigabe; bis dahin entsteht kein Live-TV-Code.
- Quelle: Konzept `docs/planning/28-live-tv/` (vor allem `02-grundsatzentscheide.md` Abschnitte 1, 3 und 4,
  `03-fachkonzept.md` Abschnitt 1.2, `04-architektur.md` T-1 bis T-29 und Abschnitt 11, `12-sicherheit.md`,
  `18-umsetzungsplan.md` Stufe 0); **Frage 37** in `docs/planning/04-entscheidungen.md` mit Marcos Antworten
  E-1 bis E-4; Wortlaut des Auftrags `docs/planning/28-live-tv/anhang/auftrag-2026-11-28.md` (21
  Grundsatzentscheide LTV-001 bis LTV-021, Ausschlussliste Abschnitt 16)
- Kein Vertrag, keine Migration. Vertragsfassung und Migrationsnummern vergibt der Bau.
- Betroffene Spec-IDs: K-3 (Altersstufe), K-5 (Schauzeit), W-8 (Stream-Grenzen), B-1 (Medien nur lesend),
  A-6 (Benachrichtigungen), A-9 (keine Geheimnisse in Protokollen); neuer Baustein L (Live-TV), der mit Stufe 0
  in die Spec kommt
- Harte Regeln: 2 (Sender, Sendungen und Aufnahmen sind keine Titel), 3 (FFmpeg nur in `transcode`),
  4 (Schreiben nur über `paths`), 6 (Live-HLS als `http.Handler`), 7 (`x-visibility`, `x-clients`),
  10 (Zugangsdaten von Quellen nie im Log), 11 (benannte Ausnahme «Live-TV»), 12 (Entscheid im Server)

## Kontext

Live-TV, EPG und DVR waren seit der Spec Version 1 Nicht-Ziele («Musik, Fotos, Live-TV und DVR»), bestätigt in
Frage 23 («Musikbibliothek, Fotos, Live-TV und DVR bleiben Nicht-Ziele»). Am 28. November 2026 hat Marco
entschieden, dass Live-TV einen eigenständigen Funktionsbereich bildet: Internetfernsehen aus IPTV-Quellen mit
Programmzeitschrift, persönlichem Timeshift und Aufnahmen, festgelegt in 21 verbindlichen Grundsatzentscheiden.
Der Auftrag wurde für ein altes Repo geschrieben; Marco hat am selben Tag entschieden, dass er für das aktuelle
Go-Produkt gilt (E-1).

Das nächste Vorbild ist das Webradio (ADR 0025): Live-Ströme ohne Titel, über den Server geleitet, mit
Zeitversatz. Drei seiner Regeln binden aber nur das Radio und widersprechen dem Auftrag, wenn man sie übernähme:
der Puffer nur im Arbeitsspeicher, «keine Sender im lokalen Netz» und «alle Profile, ohne Schalter». Dazu
kollidiert der Auftrag mit dem Altersmodell (K-3), mit «nichts löscht automatisch» (ADR 0032), mit «ein Recht
folgt der Altersgrenze» (ADR 0045) und mit «Medien nur lesend» (B-1).

## Entscheid

1. **Live-TV ist ein eigener Funktionsbereich, kein Medientyp der Bibliothek.** Sender, Sendungen und Aufnahmen
   sind keine Titel (`items`); sie erscheinen nie auf der Startseite, in Top Picks, Sammlungen, Merkliste, Suche
   der Bibliothek oder Downloads. Aufnahmen stehen nur in «Meine Aufnahmen» des Profils. Umfang: LTV-001 bis
   LTV-021 mit Marcos Antworten E-1 bis E-4.
2. **Ein Dienst im Serverprozess** (T-1). Empfang, Verteilung, Sitzungen, Aufnahmen und der Planer laufen als
   Dienst `live` im bestehenden Prozess mit eigener Laufschleife, wie das Radio. Das bestehende Jobsystem
   bekommt nur Stapelarbeit (Listen- und EPG-Import, Logos, Prüfungen, Aufräumen). Kein zweiter Container, kein
   Microservice, keine externe Middleware, kein zweites Benutzer-, Rechte-, Job- oder Transcoding-System.
3. **Rechte:** drei Schalter «Live-TV», «Aufnahmen», «Timeshift» je Konto (Administrator setzt den Deckel) und je
   Profil (Hauptprofil des Kontos); wirksam nur, wenn beide an sind. Ohne Recht ist der Bereich unsichtbar: 404,
   nie 403. Live-TV zählt zu den bestehenden Stream-, Bandbreiten- und Auflösungsgrenzen und zur Schauzeit.
4. **Verhältnis zu bestehenden ADR:**

   | ADR | Was fortgilt | Was als benannte Ausnahme abweicht |
   |---|---|---|
   | 0001 Stack | Go, SQLite mit einer Schreibverbindung, eigene Job-Queue, ein Container | nichts; Abschnitt 5 des Auftrags (PostgreSQL, Redis, BullMQ, `.deb`) gilt nur im Sinn (E-1) |
   | 0003 HLS-Segmenter | gilt unverändert für Dateien (absolute Zeitachse, Suchmodell) | Live-HLS ist ein neuer Fall mit eigenem Segmentierer im Paket `live`; 0003 wird nicht geändert |
   | 0025 Webradio | Regel «FFmpeg sieht keine Anbieter-Adresse» gilt für Live-TV fort (T-4); Radio unverändert, ohne Aufnahmen, Puffer nur im RAM, keine lokalen Sender | A-3, A-9: Quellen im Heimnetz und Multicast nur für Live-TV; Weg W als einzige, noch nicht gebaute Ausnahme von «nur Pipes» (T-4) |
   | 0032 Dateien löschen | gilt für alle Bibliotheksdateien | A-4: Aufnahmen löscht Slouchcat nur nach Wahl des Profils; Puffer sind flüchtig |
   | 0041 Ton- und Spurfähigkeiten | Geräteprofil in der Entscheidung, additiv erweitert (T-12, T-14) | nichts |
   | 0045 Wunschrecht | gilt für das Wunschrecht unverändert | A-5: die drei Live-TV-Rechte folgen nicht der Altersgrenze |

5. **Benannte Ausnahmen** (je nur für Live-TV; Wortlaut und volle Grenzen `02-grundsatzentscheide.md` Abschnitt 4):

   | Nr. | Ausnahme | Grenze |
   |---|---|---|
   | A-1 | Nicht-Ziel «Live-TV und DVR» aufgehoben: Live-TV, EPG, Timeshift und Aufnahmen von **Fernsehsendern** | Webradio ohne Aufnahmen; keine Plattform-Streams, kein DRM und keine Umgehung; keine weiteren TV-Apps (Fire TV, Apple TV, webOS, Tizen); kein eigener Dienst im Internet; Ausschlussliste Auftrag Abschnitt 16 bleibt |
   | A-2 | Altersmodell (K-3, AN-1, Plan 10.2) gilt für Live-TV nicht; das Hauptprofil gibt Live-TV für ein Profil mit Altersgrenze frei, dann alle Sender des Kontos | nicht für Filme, Serien, Hörbücher, Podcasts und andere Bibliotheken; Administrator-Deckel bleibt |
   | A-3 | Quellen im Heimnetz, UDP, RTP und Multicast | nur vom Administrator, nur an festen Zielen je Quelle (Adresse oder Netz bis /24 bzw. /64, Port oder bis 64 Ports), Prüfung für jede Verbindung; Loopback, unspezifizierte Adressen, Link-Local und eigene Adressen nie; Logos und EPG-Bilder nur öffentlich |
   | A-4 | automatisches Löschen von Aufnahmen | nur nach Aufbewahrungsfrist oder «behalte die letzten N Folgen» des Profils; nie bei vollem Kontingent (E-4), nie eine laufende Aufnahme |
   | A-5 | Live-TV-Rechte ohne Kopplung an die Altersgrenze | nur «Live-TV», «Aufnahmen», «Timeshift»; Wunschrecht bleibt unter ADR 0045 |
   | A-6 | Hauptprofil sieht, löscht und spielt Aufnahmen seiner Profile mit Altersgrenze ab (E-3) | nur dasselbe Konto, nur Profile mit Altersgrenze; Profile ohne Altersgrenze ganz privat; Administrator sieht keine Titel fremder Aufnahmen |
   | A-7 | Slouchcat schreibt Aufnahmen und Puffer (B-1) | nur in eingerichtete Live-TV-Speicherorte, nur über `paths`, nie in Bibliotheksordner |
   | A-8 | Kinderprofil-Vertragstest zählt Live-TV nicht | Live-TV-Antworten tragen keine `ItemRef`; für Titel bleibt der Test unverändert |
   | A-9 | Netzmodus «host» und veröffentlichte UDP-Ports | nur als freiwillige Zusatzdatei, nie in der Standard-Compose-Datei, nur für Stufe H, mit vier benannten Folgen |

6. **Technische Einzelentscheide** T-1 bis T-29 stehen in `04-architektur.md`. Sie werden **nicht** hier
   festgehalten, sondern als eigene ADR mit der Baustufe, die sie baut.

## Verworfen

| Alternative | Grund |
|---|---|
| Live-TV bleibt Nicht-Ziel oder wird nur als Idee in `docs/ideas/` geführt | Marcos Entscheid vom 28. November 2026 |
| Sender als `items`, Sitzungen in `playback_sessions` | zöge Sichtbarkeit, Altersregeln, Suche und Startseite der Bibliothek auf Sender (`04` Abschnitt 11) |
| eigener Live-TV-Dienst oder zweiter Container, externe Middleware (TVHeadend, Dispatcharr, Threadfin), MediaMTX oder go2rtc als Prozess | Auftrag Abschnitt 16 und Abschnitt 5 im Sinn; zweiter Serverprozess |
| Redis oder ein anderer Zwischenspeicher | zweiter Dienst; der Ring im Prozess leistet dasselbe |
| FFmpeg als Dauerempfänger je Sender | Adresse und Zugangsdaten in der Argumentliste, kein Netzschutz, Wiederverbindung als Blackbox |
| Regeln des Radios unverändert übernehmen (Puffer nur im RAM, keine lokalen Quellen, alle Profile ohne Schalter) | trägt für Video und für den Auftrag nicht (LTV-003, LTV-007, LTV-012) |
| Freigabe lokaler Ziele als Ja-Nein-Schalter | öffnet das ganze Heimnetz für jede Adresse einer Liste |

## Folgen

- Mit der Freigabe ändern sich Regel 11, Spec, Plan, Vision und Handover so, wie
  `docs/planning/28-live-tv/21-aenderungen-an-dokumenten.md` es auflistet; diese Stellen sind schon in der
  Konzeptphase nachgeführt und sagen ausdrücklich «konzipiert, nicht gebaut».
- Stufe 0 des Baus bringt: neue Schlüssel und Muster in der Redaction-Liste (Regel 10, `12-sicherheit.md`
  Abschnitt 5.4), Regel 8 für die TV-App, falls sie Live-TV bekommt, Regel 3 falls eine Hülle für FFmpeg nötig
  wird, Riegel für `store/liveq`, die Härtung der Compose-Datei und die Nachträge zu ADR 0025, 0029, 0032 und 0045.
- Neue Operationen tragen `x-visibility` und `x-clients` und fallen unter `docs/dev/VERTRAEGLICHKEIT.md`.
- Aufnahmen sind nie Teil der Sicherung (ADR 0009); ihre Zeilen in der Datenbank schon.
- Bekannte Grenze: Mit A-9 (Netzmodus «host») wird die Lücke «weltweite IPv6-Adresse im Heimnetz nicht
  erkennbar» wirksam; die Verwaltung und die Installationsanleitung nennen das.
- Offene Fragen an Marco: `docs/planning/28-live-tv/20-offene-entscheidungen.md`; ihre Antworten kommen als
  Nachtrag zu Frage 37 und zu diesem ADR.

## Spec-Wortlaut, der hier abgelöst wird (ohne die Spec umzuschreiben)

- **Topologie** «Musik, Fotos, Live-TV zurückgestellt», **Constraint** «Architektur hält … Live-TV offen, baut
  sie aber nicht» und **Non-Goal** «Musik, Fotos, Live-TV und DVR»: Live-TV ist beschlossen, konzipiert, nicht
  gebaut (Nachträge vom 28. November 2026 in der Spec).
- **K-3** und **B-1**: benannte Ausnahmen A-2 und A-7.
