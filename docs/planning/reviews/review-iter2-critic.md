# Critic-Review, Iteration 2 – VERDICT: APPROVED

## Befunde 1–14 aus Iteration 1
| # | Status | Beleg / was fehlt |
|---|---|---|
| 1 | behoben | Abschnitt 7: 68 Zeilen; DoDs in 6.2 und M0–M10 verweisen auf die Matrixzeilen. |
| 2 | behoben | 10.2: Antwort-Walk, Markierung `ItemRef` → `scoped`, ID-Endpunkte, Seerr, SSE, Profile P0–P18 mit Positiv- und Negativprüfung. |
| 3 | behoben | 6.1: Arbeitstage je Meilenstein; 2.5: Rückfall bei > 40 Arbeitstagen bis Ende M1b. |
| 4 | behoben | 2.2/2.3: Driver gewichtet, Driver 3 gegen B abgewogen, einseitige Contras korrigiert; Bilder über ffmpeg ohne CGO (3.1). |
| 5 | teilweise | 4: Update-Regel vorhanden. 3.7 nennt weiterhin `debian:bookworm-slim` (Oktober 2026 nur noch oldstable). |
| 6 | behoben | 5.7: AN-7 mit Ersatzweg je Gerät. |
| 7 | behoben | 9: Schnittlinie, Warnschwelle 1.3×, Neuplanung bei > 129 Tagen nach M4, neue Risiken. |
| 8 | behoben | 5.7: Spalte „Spätestens bestätigt vor", AN-8 bis AN-17, 5.1–5.3 bestätigungspflichtig. |
| 9 | behoben | 10.4: Schwellen und Go/No-Go; Abwandlung beim persist()-Kriterium sachlich richtig. |
| 10 | behoben | 6.2 Punkt 3: Tastaturtest je neue Seite ab M1a. |
| 11 | behoben | 8: P-6 mit SSIM/ΔE, P-9 mit Prüfdauer, P-15/P-16 mit Listen, Gerätecheckliste 10.1. |
| 12 | behoben | 2.4: Klartext-Absatz. |
| 13 | teilweise | 3.5/AN-7 „best effort, ungetestet"; M10-DoD lässt Abweichungen nur nach AN-4/12/13/17 zu, VAAPI und „ungeprüft" aus AN-7 fehlen dort. |
| 14 | behoben | 3.4: adaptives Intervall, Grenzwert < 15 s für 10 000 Dateien, AN-12. |

Offene Fragen aus Iteration 1 beantwortet: HLS-Autorisierung 5.4, Kinderprofil-Benachrichtigungen 5.5, Webhooks 5.6.

## Abnahmematrix, Stichprobe 16 IDs
Konkret und messbar (14): K-3, K-7, B-4, W-3, W-7, E-8, C-5, A-4, I-4, K-6, H-1, E-2, A-3, W-8. O-6: messbar, aber 30-Tage-Test passt zeitlich nicht in M5 (N4). A-2: „Storybook-Checks" nicht definiert. K-1: Test falsch formuliert (N3). Anteil prüfbar: rund 95 % (65 von 68).

## Kinderprofil-Vertragstest (10.2)
Abgedeckt: verschachtelte Listen inkl. Zähler, POST-Suche, Seerr-Proxy, ID-Endpunkte (Stream-Start, HLS-Master/-Varianten, Segmente, Range, Untertitel, Download vorbereiten/abrufen, Bilder, Trailer-Info, Merkliste, Favorit, Top-Pick-Detail), Benachrichtigungen über Dispatcher-Mock, SSE, Offline-Liste. Je Stufe Positiv- und Negativprüfung P0/P6/P12/P16/P18 plus Profil ohne Bibliotheksrecht. Neue Operationen automatisch erfasst (Test liest OpenAPI). Lücke: N5.

## Aufwandsschätzung
Summen stimmen (104 bzw. 163), serielle Kumulierung korrekt für Solo. M4 (15–25 Tage) und M0 (4–6 Tage) ambitioniert; Spannen und Schnittlinie fangen das auf. Rückfallkriterium 40 Tage messbar, sofern Stunden protokolliert; 30-%-Kriterium „Go-spezifischer Fehler" subjektiv (klein).

## Annahmen-Tabelle (5.7)
Fristen vorhanden, widerspruchsfrei. Sichtbar: US-Kino/US-TV, BBFC, Vererbung (AN-9), Einladungen (AN-14), PIN-Sperrkurve (AN-11), O-5 (AN-10), Seerr-Zuordnung (AN-16), 5.1–5.3. Lücke: N1.

## Neue Befunde
- **N1 (wichtig, AN-8):** Reihenfolge DE → AT → CH → GB → US nennt AT und CH, aber ohne Abbildung ihrer Stufen (AT 10/14, CH 8/10/14). Korrektur: Abbildung mit Aufrunden auf die nächsthöhere FSK-Stufe (AT/CH 14 → 16, 10 → 12, 8 → 12) oder AT/CH streichen. Betrifft Kindersicherheit.
- **N2 (klein, 3.7):** `debian:bookworm-slim` widerspricht der Update-Regel → `debian:trixie-slim`, sofern jellyfin-ffmpeg Pakete liefert.
- **N3 (klein, 7, K-1):** Schema-Test „Profil hat nur name und optional image" würde scheitern (Stufe, PIN, Sprache, Akzentfarbe). Korrektur: „Profil hat kein E-Mail- und kein Passwortfeld".
- **N4 (klein, 7, O-6/P-9 vs. M5):** 30-Tage-Lauf passt nicht in M5 (12–18 Tage). Zu Beginn M5 starten, in M6/M7 abhaken.
- **N5 (klein, 3.1/10.2):** Bilder inhaltsadressiert und immutable: Werden Hash-URLs ohne Scope-Prüfung ausgeliefert, greift die 404-Prüfung per Item-ID nicht. Festlegen: Auslieferung über Item-ID mit Scope, oder Hash-URLs ausdrücklich in 10.2.
- **N6 (klein, M10-DoD):** Zulässige Abweichungen decken „ungeprüft" aus AN-7 und VAAPI best effort nicht ab.
- **N7 (klein, 3.5/M5):** Vorbereitete Downloads (3 Qualitätsstufen) ohne Speicherort, Obergrenze, Aufräumregel. Vorschlag: `/cache/downloads`, Löschen 24 h nach erfolgreichem Abruf, Budget im Admin.

## Verdict: APPROVED
Alle blockierenden und wichtigen Befunde aus Iteration 1 substanziell behoben. Neue Befunde klein oder bei Bestätigung von AN-8 vor M2 behebbar (N1); als Folgeaufgaben in den Changelog, keine weitere Iteration nötig.

Ralplan-Zusammenfassung: Prinzip-Options-Konsistenz Pass; Alternativenprüfung Pass; Risiko-/Verifikationsstrenge Pass; Deliberate-Zusätze nicht erforderlich.
