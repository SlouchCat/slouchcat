# Verwaltungs-Redesign

Umbau der ganzen Verwaltung (`/admin/...`) nach dem Mockup, das Marco am 15. November 2026
freigegeben hat (Frage 31 im Entscheidungsprotokoll, ADR 0035). Zweig `claude/admin-redesign`.
Nur Web-Oberfläche: kein neuer Endpunkt, keine Migration, Vertrag unverändert.

## Was neu ist

- **Sechs Einträge in der Seitenleiste:** Übersicht, Medien, Nutzer & Geräte, Startseite,
  Automatisierung, System. Die Seiten eines Bereichs stehen als Reiter über jeder Seite.
- **Kopfzeile** mit Brotkrumen und «Bereich suchen» (`Strg K`): Suchpalette über alle Seiten.
- **Übersicht** «Alles im Blick»: Hinweisband, vier Kennzahlen, «Was benötigt Aufmerksamkeit?»,
  «Gerade aktiv», Schnellzugriffe, Kurzstatus.
- **System › Server** (`/admin/server`): Version, Updates, Neustart, Speicher, Fehlerprotokoll.
- **Integrationen** als Kacheln je Dienst mit eigener Ansicht.
- Alle bisherigen Adressen samt `?tab=`, `?item=`, `?days=` gelten weiter.

## Lesereihenfolge

| Datei | Inhalt |
|---|---|
| `handoff/` | Übergabepaket des Auftraggebers: Auftrag, Design, Checkliste, Mockup, Bildschirmfotos |
| `bauplan.md` | Navigationsmodell, Rahmen, Übersicht, Server, Schritte, Widersprüche Mockup gegen Bestand |
| `rahmen-ergebnis.md` | was der Rahmen gebaut hat: Bauteile, Token-Stellen, Testhelfer |
| `funktionsabdeckung.md` | Nachweis des Funktionserhalts: 876 Funktionen, Ergebnis der unabhängigen Prüfung |
| `funktionsabdeckung.csv` | die Matrix: je Funktion alte Stelle, neue Stelle, Testbeleg, Status |
| `abweichungen.md` | Abweichungen vom Mockup mit Grund |
| `abnahme.md` | ausgefüllte Abnahme-Checkliste des Übergabepakets mit Belegen |
| `screenshots/` | die umgesetzten Seiten in Desktop, Mobil und Tablet |
| `inventory/`, `abdeckung/` | Aufnahme des Bestands und Befunde der Prüfer im Einzelnen |
| `BRIEF-*.md` | die Aufträge an die Agenten (Aufnahme, Rahmen, Seiten, Prüfung) |

## Rückweg

Der Umbau enthält keine Migration. Ein Image vor dem Umbau (Release 1.38.0) läuft mit derselben
Datenbank; auf dem Testserver CT 190 ist der Wechsel zurück ein Befehl
(`/opt/slouchcat/deploy/design-alt.sh`, wieder nach vorn `design-neu.sh`).
