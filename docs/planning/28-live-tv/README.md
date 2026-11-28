# Konzept 28 · Live-TV

Stand: 29. November 2026 · Fassung 1 · Status: **Entwurf, nicht freigegeben. Nichts ist gebaut.**

Slouchcat soll einen eigenen Bereich für Internetfernsehen bekommen: IPTV-Quellen, Sender, Programmzeitschrift,
schnelles Umschalten, persönliches Timeshift und Aufnahmen. Dieses Konzept ist die Antwort auf Marcos Auftrag vom
28. November 2026 mit 21 verbindlichen Grundsatzentscheiden (LTV-001 bis LTV-021). Es umfasst Analyse, Recherche,
Fach- und Technikkonzept, Sicherheits- und Testkonzept, Umsetzungsplan und klickbare Mockups für Web-App,
Android TV und Handy-App.

**Zwei Schritte stehen aus, beide brauchen Marcos Wort:** das Konzept annehmen («Konzept freigegeben») und danach
je Stufe den Bau erlauben («Baufreigabe»). Bis dahin entsteht kein Code, keine Migration und kein Vertrag
(`CLAUDE.md` Regel 11).

## Für Marco: in dieser Reihenfolge

1. **Mockups ausprobieren:** `docs/planning/mockups/28-live-tv/index.html` doppelklicken (kein Server nötig).
   Vier Einstiege: Web-App, Verwaltung, Fernseher mit Fernbedienung, Handy und Tablet. Oben in jedem Mockup
   schaltet «Zustände» Profil, Rechte und Störungen um. Rundgang und Grenzen: [`19-mockups.md`](19-mockups.md).
2. **Fragen beantworten:** [`20-offene-entscheidungen.md`](20-offene-entscheidungen.md). Die Kurzübersicht am
   Anfang zeigt alle Fragen mit Empfehlung. **Solange du eine Frage nicht beantwortest, gilt, was dein Auftrag
   wörtlich sagt**, nicht die Empfehlung. Zuerst wichtig sind die Fragen, die vor Stufe 0 und 1a fällig sind,
   und Teil C.3 «Was du beisteuern musst» (echte Quellen für Messungen, Zeit auf der Testinstanz).
3. **Was wo vom Auftrag abweicht:** [`02-grundsatzentscheide.md`](02-grundsatzentscheide.md): deine 21 Entscheide,
   deine vier Antworten vom 28. November, was am Auftrag nicht zum Repo passte (er war für das alte Repo
   geschrieben), und die benannten Ausnahmen von bestehenden Regeln.
4. **In welchen Schritten gebaut würde:** [`18-umsetzungsplan.md`](18-umsetzungsplan.md).
5. **Wie geprüft wurde und was die Prüfer fanden:**
   [`23-agent-einsatz-und-reviews.md`](23-agent-einsatz-und-reviews.md).

## Was du wissen solltest, bevor du liest

- **Nichts ist gemessen.** Die Zielwerte fürs Umschalten (unter 1, 1, 3 und 5 Sekunden) sind zu prüfende
  Grössen. Ob ein noch nicht empfangener Sender in 3 Sekunden da ist, hängt vor allem an der Quelle
  (Abstand der Schlüsselbilder), nicht am Server. Vor dem Bau stehen Messversuche mit deinen echten Quellen.
- **Im Browser läuft vieles nur mit Umwandlung.** Typische IPTV-Sender (Zeilensprung, Ton in MP2 oder AC-3,
  HEVC) kann der Browser nicht direkt abspielen. Auf kleiner Hardware begrenzt das die Zahl der
  Web-Zuschauer; die Android-Apps sind besser dran.
- **Einige Punkte des Auftrags kommen spät oder hängen an Messungen:** DASH und HLS mit fMP4, RTSP und RTMP,
  UDP/Multicast (braucht eine Netzänderung am Container), Untertitel im Browser, Erkennung eingefrorener
  Bilder. Jede dieser Stellen ist eine Frage in `20`.
- **Safari, iPhone und iPad sind ungeprüft**, weil kein Gerät da ist.
- **Sicherheit:** Fremde Live-Daten werden nur umgewandelt, wenn die Abschottung von FFmpeg auf dem Server
  wirkt. Ob sie auf der Testinstanz wirkt, ist vor dem Bau zu messen.

## Die Dokumente

| Datei | Inhalt |
|---|---|
| [`01-bestandsanalyse.md`](01-bestandsanalyse.md) | Was es heute gibt (gebaut, teilweise, nur spezifiziert, fehlt), am Code geprüft |
| [`02-grundsatzentscheide.md`](02-grundsatzentscheide.md) | Die 21 Entscheide, Abweichungen Auftrag ↔ Repo, Marcos Antworten, benannte Ausnahmen |
| [`03-fachkonzept.md`](03-fachkonzept.md) | Fachspezifikation: jede Zeile des Auftrags als Anforderung `LTV-nnn.mm`, Detailregeln |
| [`04-architektur.md`](04-architektur.md) | Architektur: tragende Entscheide T-1 bis T-29, Komponenten, Lebenszyklus, Überlast, massgebliche Werte |
| [`05-streaming-und-fast-zapping.md`](05-streaming-und-fast-zapping.md) | Empfang je Protokoll, Verteilung, Auslieferung je Client, Umschaltkette, Benchmark, Überwachung |
| [`06-ablaeufe.md`](06-ablaeufe.md) | Lebenszyklus und die 18 Situationen des Auftrags als Sequenzdiagramme |
| [`07-datenmodell.md`](07-datenmodell.md) | Tabellen, Beziehungen, Rechte, Import, Aufbewahrung (Entwurf, keine Migration) |
| [`08-api.md`](08-api.md) | Operationen, Modelle, Fehler, Streaming-Endpunkte, Verträglichkeit (Entwurf, kein Vertrag) |
| [`09-epg.md`](09-epg.md) | Programmdaten: Quellen, Import, Zuordnung, Suche, Erinnerungen |
| [`10-timeshift.md`](10-timeshift.md) | Persönliches Timeshift |
| [`11-dvr.md`](11-dvr.md) | Aufnahmen: Planung, Schreiben in Originalqualität, Kontingent, Privatsphäre |
| [`12-sicherheit.md`](12-sicherheit.md) | Bedrohungsmodell, Massnahmen mit Testfall, Restrisiken |
| [`13-ressourcen-und-performance.md`](13-ressourcen-und-performance.md) | Rechnungen für Bandbreite, Speicher, Umwandlungen; Überlast; Messplan |
| [`14-oberflaeche.md`](14-oberflaeche.md) | Zuschauerseiten für Web, Handy-App und TV |
| [`15-administration.md`](15-administration.md) | Verwaltungsseiten |
| [`16-client-kompatibilitaet.md`](16-client-kompatibilitaet.md) | Matrizen: Protokolle, Container, Codecs, Funktionen je Client |
| [`17-test-und-abnahme.md`](17-test-und-abnahme.md) | Testarten, Abnahmekriterien `AK-L-nnn`, Benchmark, Abnahme je Stufe |
| [`18-umsetzungsplan.md`](18-umsetzungsplan.md) | Stufen V, 0, 1a, 1b, 2, 3, H, 4, 5, 6, L, W, E und die Apps |
| [`19-mockups.md`](19-mockups.md) | Mockups öffnen, Rundgang, Abdeckung, Abweichungen, Grenzen |
| [`20-offene-entscheidungen.md`](20-offene-entscheidungen.md) | Fragen an Marco, offene technische Punkte, Messungen, Risiken |
| [`21-aenderungen-an-dokumenten.md`](21-aenderungen-an-dokumenten.md) | Was an Regel 11, Spec, Plan, Vision, Entscheidungsprotokoll und ADR geändert wurde |
| [`22-anforderungsmatrix.md`](22-anforderungsmatrix.md) | Rückverfolgung Entscheid → Anforderung → Dokument → Abnahmekriterium → Stufe |
| [`23-agent-einsatz-und-reviews.md`](23-agent-einsatz-und-reviews.md) | Welche Agents mit welchem Modell, was geprüft wurde, was gefunden und behoben |
| [`24-recherche-und-quellen.md`](24-recherche-und-quellen.md) | Recherche, Vergleichssysteme, Bibliotheken, Quellen und ihre Verlässlichkeit |
| `anhang/auftrag-2026-11-28.md` | Der Auftrag im Wortlaut |
| `anhang/reviews/` | Die sechs unabhängigen Reviews im Wortlaut |
| `anhang/nachpruefung-quellen.md` | Nachprüfung der Rechercheaussagen an Primärquellen |

Ausserhalb dieses Ordners: Frage 37 in `docs/planning/04-entscheidungen.md`, ADR 0046 (vorgeschlagen) in
`docs/adr/`, der Nachtrag in `docs/spec/01-anforderungen-v1.md`, Regel 11 in `CLAUDE.md`.

## Kennungen

`LTV-001` … `LTV-021` Grundsatzentscheide · `LTV-nnn.mm` Anforderungen · `DR-nn` Detailregeln ·
`T-n` tragende technische Entscheide · `SM-nn` Sicherheitsmassnahmen · `AK-L-nnn` Abnahmekriterien ·
`LS-`/`LA-`/`LD-` Seiten und Dialoge · `E-28-nn` Fragen an Marco · `OT-28-nn` offene technische Punkte ·
`M-n`, `V-n` Messungen und Versuche. Alles trägt den Status «spezifiziert»; nichts ist «implementiert».

## Bilder

Die Mockups zeigen echte Senderlogos und Bilder nur, wenn der lokale Bildsatz vorhanden ist
(`~/.local/share/slouchcat-design/media/live-tv-2026-11-28`, verlinkt als
`docs/planning/mockups/28-live-tv/shared/img`). Diese Bilder gehören den Rechteinhabern: nie in Git, nie in
öffentliche Artefakte oder Texte. Ohne Bildsatz zeigen die Mockups Initialen und Flächen.
