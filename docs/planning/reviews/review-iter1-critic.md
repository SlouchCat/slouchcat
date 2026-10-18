# Critic-Review, Iteration 1 – VERDICT: REVISE

## 1. Prinzipien-Options-Konsistenz: teilweise erfüllt
Driver 3 (gut bekannte Werkzeuge für Claude-Code-Agenten) widerspricht der Empfehlung und wird in 2.3 ignoriert; die Empfehlung stützt sich nur auf Driver 1 und 2. Das Argument „npm-Churn widerspricht sehr stabil" zählt einseitig nur gegen A, obwohl B im Web-Teil (React 19, TanStack, Vite, Storybook, Style Dictionary, pnpm) denselben Churn hat.

## 2. Alternativenprüfung: knapp genügend, mit Strohmann-Anteilen
- Contra `sharp`/Multi-Arch gegen A ist falsch (sharp liefert linux-arm64-Binaries). Der Plan sagt nicht, womit Go Bilder skaliert; govips/CGO würde dem Entscheid „modernc.org/sqlite ohne CGO" widersprechen.
- Image-Vergleich gegen C (~200 MB plus ffmpeg) ist irreführend: auch bei B macht ffmpeg samt Treibern den Grossteil der 600 MB aus (P-2).
- „ein blockierender Handler stoppt den Event-Loop": ffmpeg läuft ohnehin als eigener Prozess.
- Nicht geprüft: Go-Server mit Bildverarbeitung über ffmpeg statt Bibliothek.

## 3. Vollständigkeit gegen die Spec
- Alle 68 IDs sind einem Meilenstein zugeordnet; keine ID verwaist.
- **Kein Prüfschritt in DoD oder P-Kriterium (22 IDs):** K-2, K-6, B-1 (read-only), B-5, B-7, H-1, W-6, W-7, W-8, W-10, E-2, E-5, E-6, E-8, O-2, A-1, A-3, A-5, A-7, A-8, I-3, K-1 (nur indirekt).
- **Nur teilweise geprüft (12 IDs):** K-5 (Verlaufsbericht fehlt), B-3 (Metadatensprache je Profil), B-8 (Trailer-Sprachwahl, manueller Ersatz), H-3 (Geschwindigkeit), W-9 (Auto-Next, Kapitel), E-4 (Personen, Sammlungen, Filter; nur Latenz geprüft), O-3 (Regel „nur WLAN"), O-6, O-7 (Speicheranzeige), C-1 (Kaltstart offline), C-3 (Vollständigkeit Englisch), I-1.
- P-22 verschiebt die Zuordnung aller 68 Kriterien zu Tests auf M10; sie gehört in den Plan.
- Non-Goals: keine Verletzung. Grenzwertig 5.1 „oder Slouchcat erzeugt lokale Seerr-Benutzer" (Schreibzugriff auf Seerr-Verwaltung) – entscheiden statt „oder".
- Constraints: (a) „Durchstich innerhalb der ersten Wochen" ist ohne Zeitschätzung nicht prüfbar; M0 ist mit Storybook, Multi-Arch-CI, Lint-Negativtests, Medien-Generator überladen. (b) VAAPI/AMD hat weder Referenzhardware noch Test.

## 4. Testbarkeit: rund 50 % vollständig prüfbar, Ziel 90 % verfehlt
Selbstbewertung „22 von 22 messbar" stimmt nicht; etwa 17 von 22 P-Kriterien sind messbar. Gegen die 68 Spec-IDs: 34 voll, 12 teilweise, 22 gar nicht. Vage: P-6 „SDR-Bild ohne Farbverfälschung" (Grenzwert nötig, z. B. SSIM/ΔE gegen Referenzbild), P-10 „bleibt ohne Ablaufdatum" (keine Prüfdauer), P-15 „Alle Hauptseiten" (keine Liste), P-16 „alle Zuschauerfunktionen" (keine Liste), M1 „Go/No-Go" (keine Schwelle), 9.1 Schritt 5 „Manuelle Gerätesichtung" (keine Checkliste).

## 5. Risiken
Abzulehnen: „Solo-Kapazität / Scope" (feste Reihenfolge verringert den Umfang nicht; es fehlen Schnittlinie, Zeitbudget je Meilenstein, Abbruchkriterium) und „Agenten erzeugen uneinheitlichen Code" (generisch). Fehlende Risiken: Verfügbarkeit der Testhardware (Mac, iPhone, iPad, N100-NAS, Pi 5); Lizenz/Kosten TVDB v4 und wer YouTube-/Trakt-Schlüssel stellt; Seerr-API-Änderungen und Unterschiede Overseerr/Jellyseerr; GPL-Quellcode-Angebot für das jellyfin-ffmpeg-Image; Aufwand des mtime-Abgleichs alle 45 s über den ganzen Baum auf SMB/NFS.

## 6. Verifikation
Kinderprofil-Negativtest ist als Pflicht verankert (gut), aber zu eng: „Jede GET-Operation mit Liste" erfasst keine verschachtelten Listen (Startseite mit Reihen, Serie mit Folgen, Person mit Filmografie, Sammlung mit Titeln), keine Suche per POST, keinen Seerr-Proxy, keine ID-Endpunkte für Stream-Start, HLS-Playlist/-Segmente, Direct-Play-Range, Download-Vorbereitung, Bilder, Trailer (Lecks im Sinne von K-3/K-8). Nur ein Kinderprofil (FSK 6), keine Positivprüfung; Off-by-one an Stufengrenzen bleibt unentdeckt. C-4 erst in M10 geprüft → Nachrüstrisiko.

## 7. Klartext
Spalte „Klartext" in Abschnitt 4 ist gut; die eigentliche Begründung „warum Go" (2.3, ADR) ist nur Fachsprache („blockierender Handler stoppt den Event-Loop", „Fehlerrate am Borrow-Checker"). „Er braucht keine Zusatzsoftware" ist irreführend (Container enthält ffmpeg und Treiber).

## 8. Annahmen
AN-1…AN-6 sind markiert, aber ohne Bestätigungsfrist, und es wird vorher damit gebaut (AN-6 vor M0, AN-1 vor M2, AN-4 in M10-DoD). Stillschweigend eingebaut und nicht in der AN-Liste: Abbildung US→FSK (PG→6, R→16), Einladungen „Ablauf 7 Tage, einmalig", PIN-Sperrkurve, O-5 „neuester Zeitstempel gewinnt", Seerr-Benutzer erzeugen oder zuordnen, die Entscheide 5.1–5.3 selbst (5.1 weicht von der Spec-Empfehlung „Einbettung als Übergang erlaubt" ab).

## 9. Befunde
1. **blockierend, 6 (DoDs) und 7.** 22 Spec-IDs ohne Prüfschritt, 12 nur teilweise; Zuordnung auf M10 verschoben. Korrektur: Abnahmematrix mit Spec-ID, Meilenstein und konkretem Test/Mass für alle 68 IDs; DoDs ergänzen (M3: B-5 Versionswahl-E2E, B-7 Extras-Fixture; M4: W-6/W-7/W-8; M7: E-2/E-5/E-6/E-8; M8: A-1/A-3/A-5/A-7/A-8).
2. **blockierend, 9.2.** Kinderprofil-Test erfasst verschachtelte Listen, Nicht-GET-Suchen und Medien-/Stream-/Download-/Bild-Endpunkte über ID nicht. Korrektur: jede Operation, deren Antwort direkt oder verschachtelt eine Titel-Referenz enthält oder einen Titel/eine Datei über ID ausliefert, trägt `x-visibility` (Schema-Test); FSK-18-ID gegen alle Medien-Endpunkte → 404; Profile je Stufe (0/6/12/16) mit Positiv- und Negativprüfung.
3. **wichtig, 2.3 und 11.** Rückfallkriterium „M0 dauert mehr als doppelt so lange wie geschätzt" ohne Schätzung. Korrektur: Aufwand je Meilenstein in Tagen; Rückfall in absoluten Tagen.
4. **wichtig, 2.2 und 2.3.** Driver 3 ignoriert; npm-Churn einseitig. Korrektur: Driver 3 gegen B abwägen; Bildverarbeitung in Go festlegen (pure Go oder ffmpeg, kein CGO).
5. **wichtig, 4.** Versionen veraltet: Go 1.25 (per 18.10.2026 vermutlich nicht mehr unterstützt), Storybook 8, Style Dictionary 4, Node 22 (lokal läuft Node 24.22). Korrektur: aktuelle Versionen und Update-Regel („immer eine der zwei neuesten Go-Versionen").
6. **wichtig, 3.3 und 9.1.** Hardware-Annahmen nicht deklariert (Mac, iPhone, iPad, N100, Pi 5). Korrektur: AN-7 mit Ersatzweg.
7. **wichtig, 8.** Risiken Scope und Agenten-Code vage. Korrektur: Schnittlinie (zuerst verschiebbar: Intro-Erkennung, Dolby Vision, Statistiken), Zeitbudget, Abbruchkriterium; fehlende Risiken aus Punkt 5 ergänzen.
8. **wichtig, 5.4.** Annahmen ohne Frist; weitere Entscheide stillschweigend. Korrektur: Spalte „spätestens bestätigt vor Mx"; US→FSK, Einladungsablauf, Sync-Regel, Seerr-Benutzer als AN-7 ff.; 5.1–5.3 bestätigungspflichtig.
9. **wichtig, M1.** iOS-Spike ohne Go/No-Go-Schwelle. Korrektur: z. B. No-Go, wenn Datei nach 8 Tagen fehlt, `persist()` false liefert oder Quote unter 5 GB.
10. **wichtig, M10 (C-4).** Tastatur-Navigation erst am Ende geprüft. Korrektur: Playwright-Tastaturtest als DoD jeder neuen Seite ab M1.
11. **klein, 7.** P-6, P-10, P-15, P-16 ohne Mass/Aufzählung.
12. **klein, 2.3/11/4.** Stack-Begründung nicht laientauglich; drei Sätze Klartext „warum Go statt TypeScript/.NET".
13. **klein, 3.3.** VAAPI/AMD: Testfall oder „nicht unterstützt" dokumentieren.
14. **klein, 3.2 Schritt 1.** Aufwand des 45-s-mtime-Abgleichs auf SMB beziffern; Last-Messung in M1/M3 mit Grenzwert.

## 10. Verdict: REVISE
Vor Zustimmung behoben: Befunde 1 und 2 (blockierend) sowie 3, 4, 5, 8, 9. Befunde 6, 7, 10 sollten in Iteration 2 miterledigt werden.

## Offene Fragen (nicht bewertet)
- Wie autorisieren Fremd-Clients HLS-Segmente, wenn native Player keinen Bearer-Header senden? Signierte URLs? Zusammenspiel mit Redaction?
- E-8/A-6: Kanäle je Konto, Favoriten je Profil. Wohin geht die Benachrichtigung für ein Kinderprofil?
- Registriert Slouchcat die Radarr/Sonarr-Webhooks selbst? Das wäre Steuerung, also ein Non-Goal.
