# Entscheidungsprotokoll

Antworten von Marco auf die offenen Fragen aus `03-plan-v1.md`, Abschnitt 13. Dieses Protokoll ist für
alle Agenten verbindlich. Solange eine Antwort „offen" ist, gilt der Vorschlag des Plans. Jede Antwort
wird mit Datum eingetragen; die betroffenen Annahmen (AN-x) im Plan gelten damit als bestätigt oder
geändert.

| Nr. | Frist | Thema | Vorschlag des Plans | Antwort | Datum |
|-----|-------|-------|---------------------|---------|-------|
| 1 | vor M0 | Code privat, Paket öffentlich, Lizenz (AN-6) | Quellcode privat; Docker-Image und Versionsinfo öffentlich; Lizenz ohne Vorgabe AGPL-3.0 | **Alles öffentlich (Open Source):** Quellcode öffentlich auf GitHub, Docker-Image und Versionsinfo öffentlich, Lizenz AGPL-3.0. AN-6 damit geändert: kein privates Repository. | 2026-10-18 |
| 2 | vor M1a | Testgeräte und Muster-NAS (AN-7) | iPhone, iPad, Android, Mac aus dem Haushalt; Intel-N100-Mini-PC als Muster-NAS kaufen | **Muster-NAS:** kein N100; der Testserver ist ein Container (CT) in Marcos Proxmox im lokalen Netz, den Claude selbst anlegt und mit begrenzter CPU und begrenztem RAM als schwaches NAS dimensioniert (Zugang, GPU-Durchreichung und Limits vor M1b festlegen). **Endgeräte:** nur Android-Handy oder -Tablet vorhanden. Kein iPhone, kein iPad, kein Mac: Der iOS-Offline-Spike (M1a, AN-13) und Safari-Tests können nicht auf echter Hardware laufen; iOS und macOS gelten nach dem Ersatzweg von AN-7 als **ungeprüft**, bis Geräte verfügbar sind. Das iOS-Risiko aus Interview-Runde 16 bleibt offen. | 2026-10-18 |
| 3 | vor M1a | Netzlaufwerke dürfen länger brauchen (AN-12) | Ja; Eine-Minute-Garantie nur für lokale Platten und Radarr/Sonarr-Meldungen | **Medien liegen auf Netzlaufwerken** (SMB- und NFS-Freigaben vom NAS); Radarr und Sonarr schreiben dorthin, ihre Meldungen (Webhook) decken den Normalfall ab. Die Lockerung „mehrere Minuten für manuell abgelegte Dateien" hat Marco **nicht** gewählt: Ziel bleibt eine Minute auch auf SMB/NFS; der Watcher/Poller nach Plan 3.4 muss das in der Lastmessung (M1a/M3, Grenzwert < 15 s je Durchlauf bei 10 000 Dateien) nachweisen, sonst wird die Frage erneut gestellt. **Sammlungsgrösse (Marco, 2026-10-18):** rund 2'500 Filme und rund 40'000 Serienfolgen, also etwa 42'500 Mediendateien plus Untertitel, Extras und Bilder, geschätzt 6'000 bis 8'000 Ordner. Konsequenz: Die Lastmessung aus Plan 3.4 (bisher „10'000 Dateien, < 15 s je Durchlauf") wird auf mindestens 45'000 Dateien und 8'000 Ordner auf SMB und NFS skaliert; der Test-Medien-Generator (M0 Schritt 6) muss einen Baum dieser Grösse mit leeren Platzhalterdateien erzeugen können; Scanner und Metadaten-Abholung müssen für 40'000 Folgen in Batches arbeiten (TMDB/TVDB-Rate-Limits, Plan 3.1 `ratelimit`), und der Erst-Scan darf Stunden dauern, muss aber Fortschritt im Dashboard zeigen und unterbrechbar sein. Konsequenz für die Architektur: SMB/NFS sind der Hauptfall, nicht die Ausnahme; `/config` mit der SQLite-Datenbank darf trotzdem nie auf dem Netzlaufwerk liegen (Plan 3.7). | 2026-10-18 |
| 4 | vor M1b | Ohne Grafikchip langsamer und kleiner (AN-17) | Ja; ohne GPU bis 10 s Start und reduzierte Auflösung bei 4K-HDR | offen | |
| 5 | vor M2 | Drei Grundsatzentscheide (5.1–5.3) | (a) Wünschen über eigene Slouchcat-Seite via Seerr-API, keine Einbettung; (b) Ankündigungen über alle Kanäle, Gäste per E-Mail; (c) Trailer zuerst TMDB, dann YouTube-Suche, KI optional | offen | |
| 6 | vor M2 | Altersfreigaben (AN-1, AN-8, AN-11, AN-14) | Unbekannte Freigabe = gesperrt unter 18; ausländische Stufen auf nächsthöhere FSK aufrunden; PIN-Sperre 30 s bis 15 min nach drei Fehlversuchen; Einladungen einmalig, 7 Tage | offen | |
| 7 | vor M3 | Serien-Vererbung und Zugangsschlüssel (AN-9, AN-15) | Folgen erben die Freigabe der Serie, nur strenger möglich; eigene kostenlose Schlüssel für TMDB, Trakt, YouTube; TVDB nur mit Lizenz | offen | |
| 8 | vor M5 | Offline-Regeln (AN-4, AN-10, AN-13, AN-18, AN-19, AN-20) | Entzug wirkt beim nächsten Kontakt; iPhone-Auto-Downloads nur bei offener App; Schauzeit offline mit Tagesrest; Offline-PIN schützt nur vor Mitbenutzern, separate längere Offline-PIN möglich; neuerer Fortschritt gewinnt; Download-Cache 50 GB, Löschung 24 h nach Abruf | offen | |
| 9 | vor M6/M7 | Hörbücher, Podcasts, Trakt (AN-2, AN-3) | Altersstufe je Audio-Bibliothek, „explicit" = FSK 18; Trakt-Reihen mit den ersten 20 Treffern, ausgeblendet unter 5 | offen | |
| 10 | vor M9 | Wünschen (AN-5, AN-16) | Profile des Hauptkontos ab FSK 12 dürfen wünschen, Gäste nicht; jedem Konto wird ein bestehender Seerr-Benutzer zugeordnet | offen | |

## Weitere Entscheide während der Umsetzung

| Datum | Entscheid | Begründung | Betrifft |
|-------|-----------|------------|----------|
| 2026-10-18 | Plan `03-plan-v1.md` freigegeben (Konsens, Iteration 2 final) | Critic APPROVED, Architect tragfähig | alles |
| 2026-10-18 | Repository wird öffentlich (GitHub, AGPL-3.0); CI darf Image und Release öffentlich publizieren | Antwort auf Frage 1 | M0, A-5, AN-6 |
| 2026-10-18 | Testserver = Proxmox-CT mit Ressourcenlimits statt N100; iOS/macOS ungeprüft (keine Apple-Geräte) | Antwort auf Frage 2 | M1a, M1b, AN-7, AN-13 |
| 2026-10-18 | SMB/NFS sind der Hauptfall für Medien; Eine-Minute-Ziel bleibt, Nachweis in Lastmessung | Antwort auf Frage 3 | M1a, M3, AN-12 |
| 2026-10-18 | Referenzgrösse der Sammlung: ~2'500 Filme, ~40'000 Folgen; Lastmessung auf ≥ 45'000 Dateien / 8'000 Ordner skaliert; Erst-Scan in Batches, unterbrechbar, mit Fortschritt | Angabe von Marco | M0 (Generator), M1a, M3, B-4, P-4 |
