# Architect-Review, Iteration 2 – Gesamturteil: tragfähig mit Änderungen (keine volle Iteration nötig)

## 1. Stand der Befunde aus Iteration 1
19 von 20 Befunden und alle vier Spannungen umgesetzt; Befund 14 teilweise.
- 2 Datenmodell: umgesetzt (3.2); Konto, Profil, Gerät, Wiedergabesitzung, Roh-Ereignisse fehlen in der Tabelle (folgen in M2).
- 11 Bilder ohne CGO: umgesetzt (3.1); Skalieren auf 1×1 px ergibt die Durchschnittsfarbe, nicht die dominante (N11).
- 12 Secrets: umgesetzt (3.8 Pkt. 3); eine Sicherung „mit Schlüssel" enthält `secret.key` neben den Secrets und ist faktisch Klartext → Archiv braucht Passwort oder Warnung.
- 14 Rückfallkriterium: teilweise (2.5), messbar, aber kein Signal für die Sprache (siehe 2).
- 15 M1 aufteilen: umgesetzt; M1a bleibt dicht (N9).
- S2: umgesetzt; K-5 offline nicht abgedeckt (N7).
- Alle übrigen (1, 3–10, 13, 16–20, S1, S3, S4): umgesetzt.

## 2. Gewichtete Stack-Abwägung und Rückfallkriterium
Rechnung stimmt (A 3.85, B 4.75, C 4.10). Punkte leicht zugunsten B gerundet: Bei Driver 1 wäre A = 4 ehrlicher (Unterschied nur 80–120 MB Image, P-2 erreicht A auch). Ergebnis robust: mit A = (4,4,5) 4.25, mit (4,5,5) 4.60; A gewinnt nur bei Gleichstand in Driver 1 und 2. Vorschlag: Sensitivitätszeile ausweisen.
Rückfallkriterium: 40 Arbeitstage = 1.29× obere Schätzung, passt zur Warnschwelle 1.3, misst aber den Gesamtaufwand inkl. Web, Design, iOS-Spike, Hardware, Gerätetests, die in Option A identisch wären. Das 30-%-Kriterium „Go-spezifische Fehler" ist das bessere Signal, aber wer einstuft, ist offen. 3-Tage-Prototyp nicht vergleichbar mit 7–11 Tagen M1a. Vorschlag: Aufgaben nach `server/` und `web/` taggen und nur Server-Aufwand gegen dessen Schätzanteil messen; Einstufung „Go-spezifisch" durch den Code-Review-Agenten nach fester Checkliste; Prototyp mit fixem kleinem Umfang (Scanner plus HLS-Transcode mit Seeking) gegen den Go-Aufwand für genau diesen Umfang.

## 3. Neue Befunde
- **N1 hoch – 3.2/3.3 – `effective_rating` denormalisiert.** Hebt der Admin eine Serie von FSK 12 auf 16 (B-9 oder manuell), behalten Staffeln/Folgen die 12; die View lässt sie für P12 durch (K-3). Gleiches für `allowed_profile_ids` bereits geladener Folgen. Vorschlag: Freigabe in der View per rekursiver CTE berechnen oder SQLite-Trigger aktualisieren Nachkommen in derselben Transaktion; danach `allowed_profile_ids` und Startseiten-Cache invalidieren; in 10.2 Fall „Freigabe der Serie ändern → Folgen sofort unsichtbar".
- **N2 mittel – 3.3 View – „mindestens eine vorhandene Datei" zu grob.** Serien/Staffeln haben keine Datei, gestreamte Podcast-Episoden (H-2) nur eine Feed-URL → unsichtbar. Vorschlag: Verfügbarkeit je `kind`: Blätter brauchen Datei oder Enclosure, Container ein sichtbares Kind.
- **N3 mittel – 5.4 – Stream-Token: Ablauf im HMAC und „Verlängerung" widersprechen sich.** Safari behält Playlist-URLs während der ganzen Wiedergabe; Pause über Nacht oder 4-h-Sitzung bricht ab. Widerruf und K-7 („401 innert 1 s") brauchen ohnehin die Sitzungstabelle. Vorschlag: Token signiert nur Sitzungs-ID und Item; Ablauf und Widerruf in `playback_sessions` (Lese-Pool je Segment); `Referrer-Policy: no-referrer`.
- **N4 mittel – 3.5 Admission Control – „nächstniedrigere Qualität" spart bei HDR-Quellen nichts.** Bei 4K-HDR kosten Decodieren und Tonemapping der Quelle, nicht die Ausgabeauflösung. Vorschlag: Kosten nach Quelle und Pipeline; bei vollem Budget warten oder ablehnen, niedrigere Qualität nur bei SDR; Budget aus Messung in M1b kalibrieren.
- **N5 mittel – 3.5 vs. M5 Schritt 1 – Pausieren passt nicht zu MP4 `faststart`.** Fortsetzung ab letztem Segment braucht segmentierte Ausgabe; `faststart` braucht zweiten Durchlauf. Vorschlag: fragmentiertes MP4 als Download-Format, oder Segmente zusammenfügen plus `faststart` (doppelter Platz in `/cache`); Format im Spike (10.4) mitprüfen.
- **N6 mittel – P-6 – SSIM/ΔE gegen Software-Referenz (bt.2390) für Hardware-Backends evtl. unerreichbar.** `vpp_qsv`/`tonemap_cuda` nutzen eigene Kurven. Vorschlag: Referenzbilder je Backend, oder ΔE nur auf Graustufen und Hauttöne plus Sichtprüfung.
- **N7 mittel – Prinzip 1 / O-7 / K-5 – Schauzeit-Limit offline ungeregelt.** Kinderprofil schaut Downloads offline ohne Limit; H-4 verlangt gleiche Regeln. Vorschlag: Server liefert beim Sync das Restbudget des Tages; Client setzt es offline mit Geräteuhr durch; Server korrigiert über Roh-Ereignisse nach. Als Ausnahme neben der Offline-PIN in Prinzip 1 und als neue AN.
- **N8 niedrig–mittel – 3.3 Offline-PIN.** PBKDF2-Verifier einer 4-stelligen PIN offline in Minuten knackbar, dieselbe PIN gilt online; bestehende PINs haben ab M5 keinen Verifier (Server kennt nur argon2id); Roh-Ereignisse ohne Aufbewahrungsregel. Vorschlag: Verifier nur an Geräte mit Downloads der betroffenen Profile; bei nächster erfolgreicher Online-Eingabe erzeugen; separate Offline-PIN anbieten; Roh-Ereignisse nach 90 Tagen zu Tageswerten verdichten.
- **N9 mittel – 6.1 Aufwand.** 104–163 Tage am unteren Rand optimistisch (Engpass: Verifikation, Gerätetests, Long Tail Safari/hls.js); M4 15–25 Tage knapp; M1a 7–11 Tage sehr dicht. Vorschlag: Stream-Token und Bildpipeline nach M1b (M1a mit Cookies in Direct Play); M4 nicht parallel zu M3.
- **N10 niedrig – 3.4 – FUSE pauschal als Netzfreigabe.** Unraid `shfs`, mergerfs bekämen dauerhaft Warnungen. Vorschlag: FUSE als „unbekannt", inotify plus adaptives Polling, Warnung erst bei Abweichung zwischen Watcher und Polling.
- **N11 niedrig – 3.1/10.2 – Bilder.** Durchschnittsfarbe trüb → `palettegen` mit gesättigtster Farbe; Bild-URLs als `/items/{id}/images/...` mit Scope-Prüfung und `Cache-Control: private, immutable`.
- **N12 niedrig – 3.2 Löschschutz – Abbruch bei > 20 % Verlust blockiert absichtliches Aufräumen.** Admin bestätigt im Dashboard („Verlust akzeptieren").

## 4. Bestehende Prinzipienverletzungen
- Prinzip 1 (mittel): K-5 offline ist eine zweite, nicht deklarierte Client-Durchsetzung (N7); mit N1 ist „Entscheid an einer Stelle" nur formal erfüllt (kopierte Spalte kann veralten).
- Prinzip 4 „rückholbar" (niedrig): Zurückholen = Sicherung wiederherstellen, Daten seit Migration verlieren; vertretbar, aber in der Klartext-Begründung so benennen.
- Prinzipien 2, 3, 5: keine Verletzung.

## 5. Gesamturteil
Tragfähig mit Änderungen: N1, N3, N4, N7 vor M1a bzw. vor ihrem Meilenstein im Plan präzisieren; keine weitere volle Iteration nötig.

Referenzen (Iteration 2): Z. 128 `effective_rating`; Z. 142 View-Bedingungen; Z. 146 `allowed_profile_ids`/PBKDF2; Z. 150 FUSE; Z. 178–181 Kapazitätsbudget; Z. 179/426 faststart; Z. 285 Stream-Token; Z. 62–71/81–88 Abwägung; Z. 337–353 Aufwand; Z. 554 P-6.
