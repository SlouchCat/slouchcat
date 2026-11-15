# Messwerte: Folgennachbarn im Offline-Abgleich (Konzept 23, Stufe 2)

Konzept `docs/planning/23-player-bedienung.md`, Abschnitt 3.3. Seit Vertrag 1.38.0 nennt
`POST /offline/sync` zu jedem Download einer Folge (Serie oder Podcast) die nächste und die vorherige
sichtbare Folge für das aktive Profil (`DownloadState.nextEpisodeId`, `previousEpisodeId`). Der Server
ruft dafür je Download `visibility.NextEpisode` und `PreviousEpisode` auf, nacheinander. Gemessen wurde,
was das einen Abgleich kostet.

## Aufbau

- Entwicklungsrechner am 15. November 2026: i7-14700K, NVMe-SSD, Go 1.27.2, ohne `-race`.
- Testumgebung des Pakets `offline` (`newEnv`, Fixture-Datenbank aus `vistest` in einem temporären
  Ordner), drei fertig vorbereitete Downloads der Folgen E1 bis E3 der Testserie für das Profil P18.
- Ein Wegwerf-Test (nicht eingecheckt) rief `neighbours` 2000-mal und `Sync` 200-mal auf, je 3 Läufe;
  zum Vergleich derselbe Abgleich mit ausgeschalteter Nachbarsuche.

## Ergebnis

| Messung | Mittel je Aufruf (3 Läufe) |
|---|---|
| `neighbours` für einen Download (2 Abfragen) | 2,98 bis 3,02 ms |
| `Sync` mit 3 Folgen-Downloads, mit Nachbarn | 11,07 bis 11,28 ms |
| `Sync` mit 3 Folgen-Downloads, ohne Nachbarn | 2,19 bis 2,20 ms |

- **Kosten:** rund **3 ms je Folgen-Download** und Abgleich, linear in der Zahl der Downloads. Hochgerechnet
  kosten 50 Folgen-Downloads eines Geräts rund 150 ms je Abgleich; der Client gleicht alle 60 s ab
  (`SYNC_INTERVAL_MS`).
- **Eingrenzung:** Seit dem Review fragt der Server die Nachbarn nur noch für Downloads ab, die das Gerät
  abspielen kann (Zustand `ready` oder `evicted`, oder im Abgleich als vollständig gemeldet). Für
  Downloads in `queued`, `preparing`, `paused` und `failed` fallen keine Abfragen an; ihre Nachbarn kämen
  ohnehin nie zum Einsatz, und der Client behält bis zum nächsten Abgleich im Zustand `ready` die alten.
  Filme, Hörbücher und Downloads ohne Titeldaten für das aktive Profil kosten weiterhin nichts.
- Offen: Werden Geräte mit mehreren hundert Folgen-Downloads üblich, liessen sich die Nachbarn je Serie
  gesammelt bestimmen statt je Download. Bis dahin reicht die serielle Abfrage.
