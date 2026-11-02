# Messwerte: Übersicht der Verwaltung (Konzept 11, ADR 0019)

Konzept `docs/planning/11-verwaltung-uebersicht.md`, Abschnitt 11.8 Schritt 6 und 11.9
(«Pflegezähler auf grossen Beständen», «Protokoll wird trotz Regeln laut»). Latten aus dem Papier:
`GET /api/v1/admin/dashboard` mit `counts` beim ersten Aufruf nach Ablauf des 60-s-Speichers
**unter 200 ms**, sonst liefert der Server beim ersten Mal `counts` ohne Pflegezahl und rechnet im
Hintergrund; `activity_events` nach einem Beobachtungstag **ohne sechs gleichartige Zeilen** einer
Art, sonst wird die Regel dieser Art enger.

## 1. Umstellung auf CT 190

2. November 2026, 08:01 Uhr (Europe/Zurich). Image `ghcr.io/slouchcat/slouchcat:verwaltung-955709b`
(`v1.6.0-58-g955709b`, Kopf nach Schritt 5), gebaut auf dem CT aus `git archive 955709b`, davor
`1.6.0`; Sicherung der Umgebung `.env.bak-vor-verwaltung`. Aus dem Log des Containers:

```
database backed up before migration path=/config/backups/pre-migration-v1.6.0-58-g955709b-20261102T070130.545Z.db
migration applied version=31 file=00031_activity_events.sql duration_ms=235
transcode ready backend=qsv budget=8 hw_tonemap=false tonemapper=cpu
listening addr=[::]:8096 startup_ms=2372
```

`healthy` rund 4 s nach `docker compose up -d`. Sicherung und Migration zusammen 1.4 s.

## 2. `GET /api/v1/admin/dashboard` (p95 kalt und warm)

Gemessen auf dem CT selbst (kein Netzweg) mit `docs/perf/measure-home.sh`, angemeldet als
Administratorkonto mit Hauptprofil (das Passwort gibt Marco selbst ein):

```bash
bash /root/measure-home.sh /api/v1/admin/dashboard                                   # warm: 60 Anfragen nach 5 Aufwärmläufen
SLOUCHCAT_RUNS=20 SLOUCHCAT_PAUSE=61 bash /root/measure-home.sh /api/v1/admin/dashboard  # kalt: 61 s Pause vor jeder Anfrage
```

Mit `SLOUCHCAT_PAUSE=61` ist der 60-s-Speicher von `counts` vor jeder gemessenen Anfrage
abgelaufen, jede Anfrage rechnet also Konten, offene Pflege und Titel neu.

**Stand 2. November 2026:** noch nicht gemessen; der Lauf braucht Marcos Anmeldung. Die
Ergebniszeilen kommen hierher.

## 3. `activity_events` nach einem Tag

Gelesen schreibgeschützt auf dem CT (`sqlite3` über Python, `mode=ro`).

**Ausgangsstand** 2. November 2026, 09:10 Uhr, rund 70 min nach der Umstellung: **7 Zeilen**,
alle aus dem Start um 08:01 (`server_started` 1, `scan_started` 3, `scan_done` 3 für die drei
Bibliotheken). Noch keine Wiedergabe, keine Sicherung, kein Neustart über die Übersicht.

**Nach einem Tag** (3. November 2026): Zeilen je Art und die häufigste Art je Stunde kommen
hierher.
