# Aufgezeichnete Webhook-Nutzlasten von Radarr und Sonarr

Grundlage: Spec I-8, Plan Entscheid 5.6 und die Abnahmematrix-Zeile I-8 («Integrationstest mit
aufgezeichneten Radarr- und Sonarr-Nutzlasten»). Die Dateien hier sind **Aufzeichnungen aus
laufenden Diensten**, keine von Hand geschriebenen Beispiele: nur so merkt der Test, wenn eine
neue Version ein Feld umbenennt.

## Benennung

```
<dienst>-<version>-<ereignis>.json
```

zum Beispiel `radarr-5.14.0.9383-download.json`. Der Dienst ist `radarr` oder `sonarr`, die
Version kommt aus dem `User-Agent` der Meldung (Radarr und Sonarr senden dort
«Radarr/5.14.0.9383 (linux x86_64)»), das Ereignis ist `eventType` in Kleinschrift
(`download`, `rename`, `moviefiledelete`, `episodefiledelete`, `moviedelete`, `seriesdelete`,
`test`, `grab`). Der Test in `server/internal/api` (`webhook_handlers_test.go`) liest jede Datei dieses
Musters und
leitet Dienst und erwartete Wirkung aus dem Namen ab — neue Aufzeichnungen brauchen keine
Codeänderung.

## Aufzeichnen

1. `python3 record.py` auf einer Maschine starten, die Radarr und Sonarr erreichen kann
   (Port 9099). Dienst und Version holt der Empfänger aus dem `User-Agent`; nachzuschlagen ist
   nichts. Nur falls ein Dienst keinen brauchbaren `User-Agent` sendet, helfen `--service` und
   `--version` nach — der Empfänger sagt es dann in der Konsole.
2. In Radarr unter «Einstellungen → Verbindungen → + → Webhook» die Adresse
   `http://<maschine>:9099/` eintragen, Methode POST, und die Ereignisse ankreuzen: On Grab,
   On Import, On Upgrade, On Rename, On Movie Delete, On Movie File Delete, On Health Issue.
3. «Test» drücken → `radarr-<version>-test.json` entsteht.
4. Je einen echten Vorgang auslösen (Import, Umbenennung, Löschung einer Datei, Löschung eines
   Films). Jede Meldung schreibt ihre Datei.
5. Dasselbe in Sonarr; derselbe laufende Empfänger nimmt beide Dienste entgegen. Dort heissen die
   Ereignisse On Import, On Upgrade, On Rename, On Series Delete, On Episode File Delete.
6. Die Verbindung in Radarr/Sonarr danach wieder löschen oder auf die echte Slouchcat-Adresse
   umstellen.

Die Aufzeichnungen enthalten die Pfade und Titel der eigenen Bibliothek. Wo das stört, dürfen
Pfade und Titel ersetzt werden — aber nur deren Werte, nie die Struktur: kein Feld entfernen,
keines hinzufügen, keine Schreibweise ändern.

## Was der Test prüft

- Aus `movie.folderPath` (Radarr) bzw. `series.path` (Sonarr) entsteht genau ein `scan.path`-Auftrag.
- `test` wird mit 200 quittiert und protokolliert.
- `grab` und jedes andere Ereignis werden mit 200 angenommen und bleiben wirkungslos.
