# ADR 0024: Updater-Begleiter

- Status: angenommen (7. November 2026); **erweitert Spec A-5** (Hinweis und Installieren),
  **ändert `CLAUDE.md` Regel 3** (benannte Ausnahme für `server/cmd/updater`) und den Kommentar in
  `deploy/compose.yaml` («ein Update ist ein bewusster Schritt»)
- Quelle: Konzept `docs/planning/15-feinschliff.md` Abschnitt 9 (P-8, Variante A mit Rückfall C)
  mit dem Nachtrag zu Stufe 4 in 9.3; **Frage 22** in `docs/planning/04-entscheidungen.md` mit
  Marcos Antworten F-13 bis F-15 vom 5. November 2026; Entscheide des Koordinators zum Bau von
  Stufe 4 vom 7. November 2026 (hier unter «Entscheid» Punkte 3 bis 8)
- Betroffene Spec-IDs: A-5 (Update), A-9 (keine Geheimnisse in Protokollen), A-4 (Sicherung vor der
  Migration)
- Harte Regeln: 3 (`os/exec`, neue Ausnahme), 6 (alle JSON-Endpunkte über den strict server),
  7 (`x-visibility: admin`, Vertragstest), 10 (Token nie im Log), 11 (kein automatisches Update),
  12 (Entscheid im Server)

## Kontext

Die Update-Prüfung (`admin/update.go`, Spec A-5) liest das Manifest der neuesten Version und zeigt
einen Hinweis. Sie installiert nichts. `deploy/compose.yaml` pinnt das Image auf
`SLOUCHCAT_VERSION`; ein Update heisst bisher: `.env` ändern, `docker compose pull`,
`docker compose up -d`. Der Server kann das nicht selbst tun, denn ein neues Image holt nur, wer
Docker auf dem Gastgeber steuert. Wer den Docker-Socket hat, hat Root auf dem Gastgeber.

Marco will einen Knopf «Update installieren» (F-13), eine Rücknahme eines gescheiterten Starts ohne
Migration (F-14) und den Begleiter auf CT 190 (F-15). Der Stack läuft oft mit mehreren
Compose-Dateien: CT 190 mit `compose.yaml`, `compose.ct.yaml` und `compose.intel.yaml`. Ein
schlichtes `docker compose up -d slouchcat` liesse die Zusatzdateien weg; Port 8096 stünde wieder
im Netz, obwohl Caddy davor sein soll, und die GPU fehlte.

## Entscheid

1. **Eigenes Programm mit eigenem Image.** Der Updater ist `server/cmd/updater`, Image
   `ghcr.io/slouchcat/slouchcat-updater`, gebaut und attestiert im selben Release-Lauf wie der
   Server. Nur er bekommt `/var/run/docker.sock`. Er hat keine Webseite, keine Benutzer, keine
   Medien und sieht `/config` nicht (weder Datenbank noch `secret.key` noch Sicherungen). Der
   Server sieht den Docker-Socket nie.
2. **Benannte Ausnahme von Regel 3.** Der Updater ruft `docker compose` über
   `exec.CommandContext` mit Argumentliste auf, nie über eine Shell. `server/.golangci.yml` lässt
   `os/exec` (depguard) und `exec.CommandContext` (forbidigo) nur unter `server/cmd/updater` zu;
   `exec.Command` und `exec.LookPath` bleiben auch dort verboten. Die Docker-Engine-API direkt zu
   sprechen ginge ohne Ausnahme, müsste aber Compose nachbauen (mehrere Dateien, Profile, Netze,
   Variablen).
3. **Eingeschaltet über eine Override-Datei, nicht über ein Profil.** Der Begleiter kommt mit
   `deploy/compose.updater.yaml`, nach dem Muster von `compose.https-only.yaml` und
   `compose.intel.yaml`:
   `docker compose -f compose.yaml [-f …] -f compose.updater.yaml up -d`. Ein Profil in
   `compose.yaml` genügt nicht, denn auch der Dienst `slouchcat` braucht dann einen Mount und eine
   Umgebungsvariable, und ein Profil kann einem anderen Dienst nichts bedingt hinzufügen. Ab Werk
   ist der Begleiter nicht dabei.
4. **Ein geteilter Ordner `deploy/updater/`.** Darin liegen `token`, `state.json` und
   `server.json`. Der Server hängt ihn **lesend und schreibend** unter `/run/slouchcat-updater`
   ein, weil er `server.json` schreibt (Konzept 9.3 sagte «lesend»; das ist hiermit berichtigt).
   Jede Datei hat genau einen Schreiber und wird atomar ersetzt (Temporärdatei im selben Ordner,
   dann Umbenennen): `token` und `state.json` schreibt nur der Updater, `server.json` nur der
   Server.
5. **`deploy/` unter demselben absoluten Pfad.** Der Updater hängt den Ordner `deploy/` unter dem
   Pfad ein, den er auf dem Gastgeber hat. Den Pfad gibt `SLOUCHCAT_DEPLOY_DIR` in `.env` vor;
   `compose.updater.yaml` verlangt die Variable (`${SLOUCHCAT_DEPLOY_DIR:?…}`). So stimmen
   `./config`, `./cache` und alle relativen Pfade der Compose-Dateien im Updater wie auf dem
   Gastgeber. Liegt eine Compose- oder Env-Datei aus den Labels des laufenden Containers ausserhalb
   dieses Ordners, lehnt der Updater ab (`outside_deploy_dir`).
6. **Genau die Compose-Zusammenstellung des laufenden Containers.** Der Updater liest am Container
   `slouchcat` die Labels `com.docker.compose.project`, `com.docker.compose.project.config_files`,
   `com.docker.compose.project.working_dir` und `com.docker.compose.project.environment_file` und
   ruft Compose mit genau diesen Werten auf:

   ```
   docker compose --project-name <project> --project-directory <working_dir>
     --env-file <environment_file> -f <config_file 1> -f <config_file 2> …
     pull slouchcat
   docker compose … (dieselben Argumente)
     up -d --no-deps --no-build slouchcat
   ```

   `--no-deps` lässt Caddy und den Updater selbst stehen. **Ohne `--wait`** (berichtigt beim Bau,
   Schritt 1): Der Healthcheck des Images (5 s Abstand, 3 Versuche) meldet den Container nach rund
   16 s `unhealthy`, während eine lange Migration noch läuft, und `--wait` gäbe dann auf. Der Updater
   wartet selbst bis zu zehn Minuten auf `healthy` und auf `server.json` mit der neuen Version. Fehlen die Labels (Start ohne Compose), lehnt der Updater ab
   (`no_compose_labels`), und der Knopf zeigt die Anleitung.
7. **Verbindung nur im internen Docker-Netz, mit Token.** Der Server ruft den Updater unter
   `http://slouchcat-updater:8099` auf; der Updater veröffentlicht keinen Port. Jede Anfrage trägt
   `Authorization: Bearer <token>`. Der Updater legt `deploy/updater/token` beim ersten Start an
   (256 Bit Zufall, Rechte `0600`, Eigentümer `PUID:PGID`, damit der Server es lesen kann). Das
   Token erscheint nie im Log (Regel 10); der Server führt es nur im Kopf der Anfrage.
8. **Zustand ohne Updater lesbar.** `GET /admin/update/status` liest `state.json` direkt aus dem
   eingehängten Ordner; so zeigt die Karte das Ergebnis auch dann, wenn der Updater gerade nicht
   läuft. Nur `POST /admin/update/install` ruft den Updater auf. Das einmalige Ereignis «Fehler»
   nach einem gescheiterten oder zurückgenommenen Update merkt sich der Server in einer eigenen
   Einstellung (welches Update er schon gemeldet hat), nicht in `state.json`, die nur der Updater
   schreibt.
9. **Die Version bestimmt der Updater.** Der Server sagt nur «installiere die neueste». Der
   Updater holt das Manifest selbst und prüft die Version streng: `^\d+\.\d+\.\d+$` und neuer als
   die laufende. Eine übernommene Web-App kann so weder eine ältere Version erzwingen noch über
   einen Zeilenumbruch andere Werte in `.env` schreiben. Läuft eine Version, die sich nicht lesen
   lässt (`dev`), installiert er nichts.
10. **Image-Digest.** Das Manifest bekommt das Feld `image`
    (`ghcr.io/slouchcat/slouchcat@sha256:…`), geschrieben von `.github/workflows/release.yml`.
    Fehlt es, lehnt der Updater ab (`no_digest`); weicht der gezogene Digest ab, bricht er vor
    dem Neustart ab (`digest_mismatch`).
11. **`.env` nur an einer Zeile.** Der Updater ändert nur `SLOUCHCAT_VERSION=` (fügt die Zeile an,
    wenn sie fehlt) und lässt den Rest Byte für Byte stehen. Vorher sichert er die Datei als
    `.env.bak-vor-<Version>` mit Rechten `0600`, weil sie Geheimnisse enthält; die letzten fünf
    Sicherungen bleiben.
12. **Migration und Rücknahme (F-14).** Der Server schreibt bei jedem Start vor und nach den
    Migrationen `server.json` (Version, Schema-Version vor und nach, Name der Sicherung vor der
    Migration). Daran sieht der Updater auch dann, ob eine Migration lief, wenn der neue Server nie
    gesund wird. Ohne Migration setzt er bei einem gescheiterten Start die alte Zeile zurück,
    startet die alte Version und meldet `rolled_back`. Mit Migration nimmt er nichts zurück und
    meldet `migrated_no_rollback` mit dem Namen der Sicherung `pre-migration-…`.
13. **Der Updater aktualisiert sich nicht selbst.** Sein Tag steht als
    `SLOUCHCAT_UPDATER_VERSION` in `.env`; ihn hebt man von Hand, wenn die Release-Notizen es
    verlangen.
14. **Gemeinsames Protokoll.** Adressen, Dateinamen, Phasen, Fehlercodes, die Formen von
    `state.json` und `server.json`, das atomare Lesen und Schreiben und die Versionsregel stehen im
    Paket `server/internal/updater`. Es startet keinen Prozess und kennt Docker nicht; Server und
    Updater importieren es beide.

## Folgen

- Vertrag **1.16.0**, additiv: `UpdateInfo` bekommt `updaterAvailable` (der Server erreicht den
  Updater mit dem Token) und `install` (Schema `UpdateInstallStatus`, aus `state.json`); neu sind
  `POST /admin/update/install` und `GET /admin/update/status`, beide `x-visibility: admin`.
  **Keine Migration**; die Einstellung für das gemeldete Ereignis ist eine Zeile in `settings`.
- Ohne Begleiter bleibt alles wie bisher, und der Knopf zeigt die Befehle für das Update von Hand
  (Rückfall C). `docs/INSTALLATION.md` und `deploy/README.md` beschreiben den Begleiter als
  freiwillig.
- Wer `compose.updater.yaml` einsetzt, gibt einem Container Root auf dem Gastgeber. Das Risiko
  bleibt auf ein Programm ohne offenen Port und ohne Benutzer begrenzt, das nur einen Auftrag
  annimmt und die Version selbst bestimmt.
- Phasen in `state.json`: `idle`, `checking`, `backing_up`, `pulling`, `verifying`, `restarting`,
  `waiting`, dann `done`, `rolled_back` oder `failed`. Gründe: `busy`, `not_newer`,
  `manifest_unreachable`, `manifest_invalid`, `no_digest`, `no_compose_labels`,
  `outside_deploy_dir`, `pull_failed`, `digest_mismatch`, `unhealthy`, `rollback_failed`,
  `migrated_no_rollback`.
- Abnahme auf CT 190 (F-15) mit seinen drei Compose-Dateien und `compose.updater.yaml`; danach
  stehen Port und GPU wie vorher.

## Verworfen

- **Docker-Socket im Slouchcat-Container (Variante B):** Wer die Web-App übernimmt, hätte Root auf
  dem Gastgeber.
- **Watchtower (Variante D):** aktualisiert von selbst, widerspricht dem gepinnten Tag und dem
  bewussten Schritt; kein Knopf, keine Rücknahme.
- **Compose-Profil `updater` in `compose.yaml`:** kann dem Dienst `slouchcat` den geteilten Ordner
  und die Adresse des Updaters nicht bedingt geben.
- **Docker-Engine-API statt `docker compose`:** keine Ausnahme von Regel 3, aber ein Nachbau von
  Compose mit eigenen Fehlern.
- **Der Server schreibt den gemeldeten Fehler in `state.json`:** zwei Schreiber einer Datei; der
  Updater könnte die Markierung beim nächsten Auftrag überschreiben.
