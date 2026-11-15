# Bestandsabgleich: bestehende Routen vs. neuer Navigationsort

## Verlässliche Basis

`web/src/features/admin/sections.ts` definiert derzeit 19 Seiteneinträge. `web/src/app/paths.ts` enthält zusätzlich die Übersicht und Detailrouten. Die Übersicht ist über `web/src/features/admin/AdminHomePage.tsx` implementiert. Die neue Navigation darf bestehende Wege in der Anzeige gruppieren, aber bestehende Links nicht zerstören.

| Schlüssel | Bisherige Route | Neuer Navigationsort |
|---|---|---|
| `libraries` | `/admin/libraries` | Medien → Bibliotheken |
| `podcasts` | `/admin/podcasts` | Medien → Podcasts |
| `radio` | `/admin/radio` | Medien → Webradio |
| `maintenance` | `/admin/maintenance` | Medien → Pflege |
| `metadata` | `/admin/metadata` | Medien → Metadaten |
| `accounts` | `/admin/accounts` | Nutzer & Geräte → Konten |
| `devices` | `/admin/devices` | Nutzer & Geräte → Geräte |
| `streams` | `/admin/streams` | Nutzer & Geräte → Streams |
| `spotlight` | `/admin/spotlight` | Startseite → Spotlight |
| `picks` | `/admin/picks` | Startseite → Cat’s Empfehlungen |
| `collections` | `/admin/collections` | Startseite → Sammlungen |
| `homeRows` | `/admin/home-rows` | Startseite → Startseite |
| `myMedia` | `/admin/my-media` | Startseite → Meine Medien |
| `announcements` | `/admin/announcements` | Startseite → Mitteilungen |
| `jobs` | `/admin/jobs` | Automatisierung → Hintergrundjobs |
| `integrations` | `/admin/integrations` | Automatisierung → Integrationen |
| `notifications` | `/admin/notifications` | Automatisierung → Benachrichtigungen |
| `stats` | `/admin/stats` | System → Statistiken |
| `backup` | `/admin/backup` | System → Sicherungen |

## Ergänzende Unterseiten und Querverbindungen

- `web/src/features/admin/AudiobookSourcesPage.tsx` (Pfad `/admin/metadata/audiobooks`) → **Medien / Hörbuch-Quellen**. In bestehendem Routing unter Metadaten; der neue Zugriff kann zusätzlich sein, ohne alte Links abzuschneiden.
- `web/src/features/admin/PodcastPage.tsx` (`/admin/podcasts/:id`) → **Medien / Podcasts / Podcastdetails**, Episoden und Downloadstatus.
- `web/src/features/admin/AdminItemPage.tsx` (`/admin/items/:id`) und `web/src/features/admin/MaintenancePage.tsx` → **Medien / Pflege / Titeldetails**; bestehende Metadaten-Bearbeitungen und Medienvarianten prüfen.
- `web/src/features/notifications/ChannelSettingsPage.tsx` → Admin-Navigation bei `/admin/notifications` und kontospezifische `/settings/notifications` weiter korrekt abbilden. Keine Rollenverwechslung.
- `/settings/profiles` und `/settings/playback` bleiben ausserhalb der Verwaltung als Einstellungen für die dafür berechtigten Konten/Profile verfügbar.
- Serverseitiger Status/Neustart/Updates (zurzeit `AdminHomePage.tsx`) muss in Übersicht und/oder System weiterhin erreichbar bleiben, auch wenn **Server** keine eigenständige bestehende Admin-Route in `sections.ts` ist.

## Die 69 Mockup-Detailansichten

**Nicht als abschliessende Funktionsliste verwenden.** Sie dokumentieren die zuvor freigegebene Konzeption; einzelne Optionen können rein illustrativ sein und die tatsächliche Anwendung hat mehr Sonderfälle. Die maschinenlesbare Liste steht in `data/69_mockup_detailansichten.csv`.

| Mockup-Unterseite | Anzahl | Unterkonzepte |
|---|---:|---|
| Bibliotheken | 6 | Neue Bibliothek anlegen, Bibliothek bearbeiten, Ordner und Scans, Entwürfe & Löschen, Hörbuch-Quellen, Metadaten pro Bibliothek |
| Metadaten | 6 | Textsprachen, Poster, Hintergründe und Logos, Bildqualität, Quellen und Inhalte, Altersfreigaben, Metadaten anwenden |
| Pflege | 6 | Fehlende oder falsche Zuordnung, Unvollständige Metadaten, Duplikate zusammenführen, Verwaiste Daten neu zuordnen, Entfernte Titel wiederherstellen, Altersfreigabe in Auswahl ändern |
| Podcasts | 5 | Feed abonnieren, Podcast-Einstellungen, Episoden verwalten, Feed- und Downloadqueue, Podcast entfernen |
| Webradio | 5 | Katalog & Angebot, Sender verwalten, Eigene Sender, Puffer & Relays, Cover & externe Daten |
| Konten | 4 | Einladungen erstellen, Rollen & Berechtigungen, Profile je Konto, Zustellung je Konto |
| Geräte | 2 | Registrierte Geräte, Gerätefähigkeiten |
| Streams | 2 | Aktive Streams, Transcoding & Auslastung |
| Startseite | 2 | Reihenfolge & Sichtbarkeit, Bibliotheksreihe |
| Spotlight | 3 | Spotlight-Quelle, Titel anheften, Verwaiste Einträge |
| Cat’s Empfehlungen | 2 | Aufbewahrungsdauer, Benutzerempfehlungen |
| Sammlungen | 3 | Sammlungen durchsuchen, Manuelle Sammlung bearbeiten, Sammlung löschen |
| Meine Medien | 1 | Sortierung & Sichtbarkeit |
| Mitteilungen | 2 | Mitteilung verfassen, Veröffentlichte Mitteilungen |
| Hintergrundjobs | 4 | Jobtypen und Warteschlange, Automatischer Zeitplan, Läufe & Fehler, Ereignis-Zeitfenster |
| Integrationen | 6 | Anbieter-Schlüssel, Radarr / Sonarr Webhooks, E-Mail / SMTP, Telegram, Web Push, Server & Updates |
| Benachrichtigungen | 2 | Zustellkanäle, Benachrichtigungsarten |
| Statistiken | 2 | Wiedergabestatistik, Auswertungen |
| Sicherungen | 3 | Vorhandene Sicherungen, Neue Sicherung, Aus Datei wiederherstellen |
| Server | 3 | Serverzustand, Updates & Neustart, Speicher & Logs |

## Vollständigkeitsprüfung vor Umbau

Für **jede** vorhandene UI-Aktion (einschliesslich Row-Expand, verstecktem Menü, Dialog, Formularfeld und Shortcuts) muss Claude die tatsächliche React-Komponente und API-Verbindung prüfen und in eine Sollposition abbilden. Auch Funktionen, die im V2-HTML nicht vorkommen, müssen integriert werden. Fehlende Beschreibung im Mockup ist **kein** Löschauftrag.

Mindestens gesondert zu überprüfen:

- Bibliotheks-Entwürfe, Ordnerverwaltung, Scans, Scan-Abbruch, Datenverlust-Akzeptanz, Lösch-/Bereinigungsabläufe.
- Metadaten: Serverstandard vs. Bibliotheksabweichung, Bildarten und Sprachreihenfolgen, Mindestgrösse, Altersfreigaben, Apply-Jobs.
- Podcasts: Abonnement, Unterseiten/Details, Downloadmodi und Episodenaktionen, Queue-Zustände.
- Webradio: Angebot, Sender, eigene Sender, Relay-/Budget-/Nachlauf-Einstellungen und externe Coverquellen.
- Konten: Einladungen, Rollenänderung, Profile, Zugriff auf Bibliotheken, Benachrichtigungskanäle, Löschschutz.
- Geräte: Erkannt/vergessen, Fähigkeitssicht und Unterkategorien; Streams: Typ, Transcoding, Load und Stoppen.
- Startseitenreihen, Spotlight-Pins/Notizen, Empfehlungen, manuelle und TMDB-Sammlungen, Meine-Medien-Sichtbarkeit, Entwürfe/Veröffentlichung von Mitteilungen.
- Jobs: Typen-Gruppen, alle Run-States, Fortschritt, Retry, Cancel, Pause/Resume, Intervall/Einmal-Termin, Zeitfenster, alle Fehlerdetails.
- Integrationen: sechs Provider, gespeicherte vs. Environment-Schlüssel, Secret-Masking, Webhook-Pfadzuordnung, Mail-Test, Telegram, Push, Server-URL.
- Backups: ohne/mit Geheimnissen, Passphrase, Download, Upload/Restore, Lösch- und Neustartablauf; Statistiken: Zeiträume, Nutzungsarten und Profile/Geräte.
- Übersicht: Fehlerlog, Aktivität, Speicher `/config` und `/cache`, Webhooks, Updates/Neustart, HW-Transcoding, inotify und Hinweise.
