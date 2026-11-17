# Konzept 25 · Wünsche über Jellyseerr

- **Status: Entwurf zur Freigabe, Fassung 2 (nach Kritik), 17. November 2026. Nichts ist gebaut.** Fassung 2
  arbeitet die 40 Befunde der ersten Kritik (Anhang G) und die Redaktionsentscheide vom 17. November 2026 ein
  (`11-entscheide-und-adr.md`, Abschnitt 7). Marco prüft Konzept und Mockups, beantwortet die Fragen F-1 bis F-22
  und die Teilfragen T-1 und T-2 (`11-entscheide-und-adr.md`) und gibt frei. Erst danach folgt ein eigener
  Auftrag für den Bau.
- **Auslöser:** Marcos Auftrag vom 16. November 2026: fehlende Filme und Serien direkt in Slouchcat anfordern,
  vor allem aus dem Spotlight. Jellyseerr bleibt bestehen und verwaltet die Wünsche. Slouchcat zeigt den
  Zustand und benachrichtigt über die eigenen Kanäle, sobald der Titel abspielbar ist.
- **Verankert gegen:** `main` `ba6efe31` (Release 1.40.1), Vertrag 1.40.0, letzte Migration 00065, letzte
  ADR 0036, letzte Frage im Entscheidungsprotokoll 32.
- **Nummern:** Dieses Papier ist Konzept 25 und reserviert Frage 33, ADR 0037 («Wünsche über Jellyseerr») und
  ADR 0038 («Titel ausserhalb der Bibliothek»). Migrations- und Versionsnummern vergibt der Bau.
- **Dieses Konzept ändert keine Zeile Code und keine Regel.** Wünschen ist heute ein Nicht-Ziel (Spec,
  Frage 10, S-4, `CLAUDE.md` Regel 11). Welche Stellen sich mit einer Freigabe ändern, steht in
  `11-entscheide-und-adr.md` Abschnitt 2.

## In einem Absatz

Ein Profil, das anfordern kann, sieht fehlende Titel im Spotlight, in der Suche und auf einer Vorschauseite und
fordert sie mit einem Dialog an. Wer nicht anfordern kann, sieht sie nicht. Slouchcat reicht den Wunsch im Namen des zugeordneten Jellyseerr-Benutzers
weiter (Header `X-API-User`), speichert selbst, welches Konto und welches Profil gewünscht hat, gleicht den
Zustand alle fünf Minuten ab und meldet «verfügbar» erst, wenn die eigene Bibliothek den Titel für das
Profil abspielen kann. Slouchcat beschafft nichts selbst und spricht weder Radarr noch Sonarr an.

## Die Papiere

| Datei | Inhalt |
|---|---|
| [01-bestandsanalyse.md](01-bestandsanalyse.md) | Was es heute gibt, was die Jellyseerr-API wirklich kann, welche Einschränkungen das Konzept bestimmen |
| [02-funktionskonzept.md](02-funktionskonzept.md) | Begriffe, Zustände eines Wunsches, Abläufe, Seiten der Zuschauer-App, Serien und Staffeln |
| [03-architektur.md](03-architektur.md) | Pakete, Anbindung, Ausgangsfach, Abgleich, Verfügbarkeit, Jobs, Fehler, Sicherheit, Diagramme |
| [04-benutzer-profilmapping.md](04-benutzer-profilmapping.md) | Varianten A, B, C im Vergleich, Empfehlung, Regeln der Zuordnung, Sonderfälle, Wunschrecht |
| [05-spotlight-integration.md](05-spotlight-integration.md) | Fehlende Titel auf der Bühne: Regeln, Kuratierung, Daten, Performance |
| [06-benachrichtigungen.md](06-benachrichtigungen.md) | Ereignisse, Empfänger, Datenschutz, Doppelschutz, Standardwerte |
| [07-administration.md](07-administration.md) | Jede betroffene Verwaltungsseite: heutiger Inhalt und was dazukommt, Status und Diagnose |
| [08-datenmodell-api.md](08-datenmodell-api.md) | Tabellen, Migrationen, Operationen, verwendete Jellyseerr-Endpunkte |
| [09-umsetzungsplan.md](09-umsetzungsplan.md) | Stufen 0 bis 4, Schritte, Tests, Risiken |
| [10-abnahmekriterien.md](10-abnahmekriterien.md) | Prüfbare Kriterien je Stufe, Abnahme der Mockups |
| [11-entscheide-und-adr.md](11-entscheide-und-adr.md) | Offene Entscheide F-1 bis F-22 und T-1, T-2 mit Empfehlung, geänderte frühere Entscheide, Entwürfe Frage 33, ADR 0037, ADR 0038, Regel 11, Einarbeitung der Kritik und Abweichungen von Anhang F |
| [anhang/](anhang/) | Grundlagen: [A](anhang/A-spotlight-startseite.md) Bestand Spotlight und Startseite; [B](anhang/B-konten-benachrichtigungen.md) Bestand Konten, Profile und Benachrichtigungen; [C](anhang/C-integrationen-jobs-regeln.md) Bestand Integrationen, Jobs und harte Regeln; [D](anhang/D-jellyseerr-api.md) was die Jellyseerr-API wirklich kann; [E](anhang/E-design-sheet.md) Gestaltungsvorgaben für das Mockup; [F](anhang/F-entscheid-blatt.md) Entscheid-Blatt als Zwischenstand (wo die Papiere abweichen, gilt das Papier); [G](anhang/G-kritik-fassung-1.md) erste Kritik mit 40 Befunden. In den Papieren heissen sie kurz «Anhang A» bis «Anhang G» |
| [mockup-abdeckung.md](mockup-abdeckung.md) | Abdeckungsmatrix: Seite, heutige Funktion, Änderung, Rolle, Einstellungen, Operationen, Mockup, Zustände, Umsetzung |

Lesereihenfolge für Marco: dieses Blatt, dann das Mockup, dann `02`, `05`, `04`, `06`, `07` und zum Schluss
`11` mit den Fragen. `01`, `03`, `08`, `09` und `10` sind für den Bau.

## Das Mockup

Klickbar, ohne Server, ohne Jellyseerr, ohne echte Anfrage. Es liegt getrennt vom Produkt in
[`../mockups/25-medienwuensche/`](../mockups/25-medienwuensche/) und teilt keinen Code mit der App.

- **Einstieg:** `docs/planning/mockups/25-medienwuensche/index.html` im Browser öffnen. Die erste Seite
  listet alle Mockup-Seiten (M-01 bis M-30).
- **Steuerung:** Die gelbe Schaltfläche «Mockup» unten rechts ersetzt Jellyseerr, Radarr und die Zeit:
  Profil wechseln, Jellyseerr ausfallen lassen, Wünsche vorspulen (übermitteln, freigeben, Datei trifft ein,
  scheitern), Ansicht Desktop, Tablet oder Handy, alles zurücksetzen.
- **Bilder:** Poster und Szenenbilder stammen von TMDB und liegen nur lokal im Ordner `img/`. Das Repo ist
  öffentlich, darum stehen sie nicht im Repo (`.gitignore`). Ohne Bilder zeigt das Mockup Platzhalter mit
  dem Titel.
- **Zustand:** Was du anklickst, bleibt im Browser gespeichert (`localStorage`), bis du zurücksetzt.

## Die wichtigsten Entscheide in Kürze

| Thema | Empfehlung | Frage |
|---|---|---|
| Wer verwaltet Wünsche | Jellyseerr; Slouchcat legt an, liest, zeigt und meldet | – |
| Im Namen von wem | Header `X-API-User` mit dem zugeordneten Benutzer: seine Rechte, sein Kontingent, seine Auto-Freigabe gelten | F-3 |
| Zuordnung | hybrid: ein Jellyseerr-Benutzer je Konto, einzelne Profile mit eigenem | F-2 |
| Wunschrecht | Schalter je Konto (Administrator) und je Profil (Hauptprofil) | F-5 |
| Jugendschutz | TMDB-Freigabe in der Länderreihenfolge nach ADR 0034 (erstes Land mit bekanntem Wert) gegen die Altersstufe, unbekannt zählt als 18, Entscheid im Server | – |
| Wer fehlende Titel sieht | nur wer sie anfordern kann (Recht, Zuordnung, Art, Alter); Ausnahme eigene Wünsche | F-9 |
| Verfügbar | wenn Slouchcat den Titel für das Profil abspielen kann | F-13 |
| Schon angefordert | Mitwunsch statt Doppelbestellung | F-7 |
| Spotlight | Schalter, höchstens 5 fehlende von 20, erste Folie immer vorhanden | F-8 |
| Verwaltung | Kachel Jellyseerr nach «Webhooks von Radarr und Sonarr»; Reiter «Wünsche» unter Medien nach «Pflege»; Reihe «Deine Wünsche» nach «Weiterschauen»; «Wünsche aktiv» ab Werk aus | F-19, F-20 |
| Benachrichtigung | Eingang des Profils und Kanäle des Kontos, bei PIN-Profilen ohne Titel | F-14, F-15 |
| Bau | vier Stufen nach der Freigabe, je Stufe ein Release; Stufe 1 im Vertrag nicht brechend, Stufen 2 bis 4 brechend (`08-datenmodell-api.md`, Abschnitt 4.6) | F-22 |
