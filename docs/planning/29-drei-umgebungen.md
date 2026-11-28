# Konzept: Drei Umgebungen – dev, test, produktiv; Kandidat und Freigabe

- **Status: Entwurf, Fassung 2 (28. November 2026).** Vier Weichen hat Marco am selben Tag über
  Rückfragen entschieden (Abschnitt 16.1). Elf kleinere Fragen sind offen, jede mit Empfehlung
  (Abschnitt 16.2). Fassung 2 arbeitet das Kritiker-Review ein (REVISE, 0 Blocker, 9 Major,
  16 Minor) und die Nachprüfung der Einarbeitung (ACCEPT, vier kleine Korrekturen; Abschnitt 19).
  Gebaut wird erst auf Marcos Wort.
- **Auslöser:** Marco am 28. November 2026: «Ich möchte drei CT's für slouchcat: 1. Slouchcat
  produktiv (CT190) 2. Slouchcat test (CT191) 3. Slouchcat dev (CT191). Die Idee ist, dass wir die
  änderungen ohne CI Lauf direkt im dev machen. Danach wenn ich es freigebe soll test die update
  mittels der Server Update funktion getestet werden. Erst wenn ich es da freigeben, soll der
  Release latest geändert werden und dann sieht es das produktiv system auch. Ist das der übliche
  Weg? Wenn nicht bitte korigiere mich.»
- **Verankert gegen:** `main` **`a7b14ea6`** («Merge pull request #240»), Server **1.44.1**,
  Vertragsfassung **1.44.0** (`api/openapi.yaml:4`), letzte Migration
  **`00086_wishes_rights_follow_rating.sql`**, letzte ADR **`0045`**, letzte Frage im
  Entscheidungsprotokoll **36**.
- **Nummern:** Dieses Papier ist **Konzept 29** und reserviert **Frage 38**, **ADR 0056** («Drei
  Umgebungen, Kandidat und Freigabe») und **ADR 0057** («Testbetrieb: keine Wirkung nach aussen»).
  Konzept 28 (Live-TV, Zweig `claude/konzept-live-tv`, noch nicht in `main`) belegt Frage 37 und
  ADR 0046 bis 0055. Migrations- und Versionsnummern vergibt der Bau.
- Pfade ohne Präfix liegen unter `server/`. **Nach Namen suchen, nicht nach Zeile**; Zeilen
  beziehen sich auf den Anker.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`). Dieses
  Papier ändert frühere Festlegungen (Abschnitt 13).

> **Dieses Papier ändert keine Zeile Code und keinen Container.** Es beschreibt, was heute gilt,
> und schlägt einen Weg in drei Baustufen vor. Die Angaben zu Proxmox und CT 190 sind am
> 28. November 2026 lesend abgefragt, alles andere stammt aus dem Code am Anker. Was nicht belegt
> ist, steht in Abschnitt 18.

## 1. Was Marco will

| Nr. | Wunsch (28. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| W-1 | Drei Container: produktiv (CT 190), test (CT 191), dev (CT 192). | 4, 12 |
| W-2 | Änderungen landen ohne CI-Lauf direkt auf dev. | 5, 11 |
| W-3 | Nach Marcos Freigabe spielt test das Update über die Update-Funktion des Servers ein. | 5, 6, 7 |
| W-4 | Erst nach Marcos zweiter Freigabe wird das Release «latest»; erst dann sieht es produktiv. | 5, 6 |
| W-5 | Ist das der übliche Weg? Wenn nicht: korrigieren. | 2 |

Nicht Teil dieses Papiers: ein Stufenweg für die Android-Apps (sie haben die Testspuren des Play
Store) und für die zwei Dienste aus `relay/`. Wo sie berührt sind, steht in Abschnitt 6.7.

## 2. Kurzfassung

**Ja, das ist der übliche Weg.** Drei Stufen mit einer Freigabe dazwischen sind Standard. Vier
Punkte kommen dazu, damit der Weg hält, was er verspricht:

1. **Die CI fällt nicht weg, sie wandert.** Dev läuft ohne CI. Vor test kommt aber der Pull
   Request mit grüner CI, der Merge und der Tag. Sonst prüft test einen ungeprüften Stand.
2. **Einmal bauen, dasselbe Paket befördern.** Test und produktiv bekommen dasselbe Image, Byte
   für Byte. Für test wird also nicht eine Vorabversion gebaut und für produktiv eine neue, sondern
   das Release wird einmal gebaut und ohne «latest» veröffentlicht (ein **Kandidat**). Die zweite
   Freigabe hängt nur «latest» um.
3. **Test läuft mit einer frischen Kopie von produktiv.** Vor jedem Kandidaten wird test auf den
   Stand von produktiv zurückgesetzt, gleiche Daten und gleiche Version. Test macht dann genau den
   Update-Schritt, den produktiv später macht, samt Migration am echten Bestand.
4. **Eine Kopie darf nichts nach aussen tun.** Eine Kopie von produktiv schickt Push und Mail an
   dieselben Handys und schreibt in dasselbe Seerr. Ein neuer Schalter «Testbetrieb» im Server
   sperrt das.

Der Weg einer Änderung:

```
Zweig ─► dev (CT 192) ─► Freigabe 1 ─► PR, CI, Merge, Tag ─► Kandidat
                                                                │
          produktiv (CT 190) ◄─ «latest» ◄─ Freigabe 2 ◄─ test (CT 191)
```

| | dev (CT 192) | test (CT 191) | produktiv (CT 190) |
|---|---|---|---|
| Wozu | Unfertiges ansehen, an echten Daten und echter Hardware | Den Kandidaten und den Update-Schritt prüfen | Die Familie schaut |
| Was läuft | Zweig-Image, auf dem Container gebaut | nur Releases | nur freigegebene Releases |
| Wie kommt Neues | Skript, ohne CI | Update-Funktion, Kanal «Kandidat» | Update-Funktion, Kanal «stabil» |
| Daten | Kopie von produktiv, vor jedem neuen Zweig frisch | Kopie von produktiv, vor jedem Kandidaten frisch | die echten |
| Wirkung nach aussen | gesperrt (Testbetrieb) | gesperrt (Testbetrieb) | normal |
| Erreichbar | nur im Heimnetz | nur im Heimnetz | wie heute |

**Was test beweist und was nicht:** Test beweist den Update-Schritt, die Migration am echten
Bestand und alles, was im Server selbst geschieht. Test beweist nicht, dass eine Push-Meldung am
Handy ankommt oder ein Wunsch in Seerr landet; genau das ist dort gesperrt (Abschnitt 8.5).

Heute fehlen dafür drei Dinge im Code und zwei Container (Abschnitt 3). Der Bau hat drei Stufen
(Abschnitt 14).

## 3. Heute

### 3.1 Ein Container für alles

CT 190 auf `pve2` ist heute Testserver, Abnahmeserver und der Server der Familie zugleich
(`docs/dev/TESTSERVER.md`). Vor einem Release läuft dort für die Abnahme ein Zweig-Image, danach
hebt Marco über die Update-Funktion auf das Release. Eine Migration eines unfertigen Zweigs trifft
damit den echten Bestand, und sie lässt sich nicht zurückdrehen (Migrationen laufen nur vorwärts,
Regel 5).

### 3.2 Der Release-Lauf setzt «latest» sofort

`.github/workflows/release.yml` läuft bei einem Tag `v*.*.*`. Der Schritt «Decide whether the tag
is a pre-release, a backport or the latest release» (Zeilen 79–97) macht einen Tag `vX.Y.Z`, der
die höchste Version des Repositorys ist, sofort zum «latest»-Release: Image-Tag `:latest` für
Server und Updater (Zeilen 124, 159) und `gh release create … --latest` (Zeilen 231–245). Der Lauf
startet nur bei einem Tag; einen Lauf von Hand gibt es nicht. Er hat keine `concurrency`-Gruppe,
zwei Läufe können also nebeneinander laufen.

### 3.3 Update-Prüfung und Updater lesen nur «latest»

- Der Server liest fest
  `https://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json`
  (`internal/config/config.go`, `DefaultUpdateManifestURL`). Eine andere Adresse gilt nur im
  Entwicklungsmodus (`Config.ManifestURL`).
- Der Updater-Begleiter liest sein eigenes Manifest. Ohne `SLOUCHCAT_UPDATE_MANIFEST_URL` ist es
  dieselbe Adresse (`cmd/updater/main.go`, `loadConfig`); `deploy/compose.updater.yaml` reicht
  diese Variable nicht durch. Der Server gibt dem Begleiter weder Adresse noch Version mit
  (`internal/admin/update_install.go`).
- Angeboten und installiert wird nur eine Version der Form `X.Y.Z`, die neuer ist als die laufende
  (`internal/updater/updater.go`, `ValidRelease`, `Newer`). `1.45.0-rc1` wird nie angeboten.
- Der Abruf kennt zwei Adressformen bei GitHub: `releases/latest/download/<name>` und
  `releases/download/<tag>/<name>`, beide mit Rückfall auf die API (`internal/updater/manifest.go`,
  `githubAsset`, `fetchViaAPI`). Die Compose-Dateien eines Releases holt der Updater bei einer
  GitHub-Adresse immer unter `releases/download/v<version>/<name>` (`ReleaseFileURL`). Nichts im
  Code vergleicht die Adresse des Manifests oder ihren Tag mit der Version.

Eine Testinstanz kann ein Release ohne «latest» heute also nicht sehen. Den Entwicklungsmodus als
Ausweg zu nehmen wäre falsch: Er schaltet rund zehn weitere Dinge um, unter anderem Push an
private Adressen und Fehlerdetails in den Antworten der API.

### 3.4 Eine Kopie ist nach aussen der Server selbst

Wer `config/` von produktiv kopiert, kopiert `secret.key`, die Schlüssel für Web-Push, alle
Push-Anmeldungen, die Sitzungen, die Zugangsdaten zu Seerr und die Warteschlange der Jobs. Eine
solche Kopie tut nach aussen dasselbe wie produktiv:

| Wirkung | Was passiert | Stelle |
|---|---|---|
| E-Mail, Web-Push, Push an die Handy-App, Telegram | dieselben Empfänger bekommen jede Meldung zweimal; wartende Zustellungen aus der kopierten Warteschlange gehen sofort raus | `internal/notify` (`deliver.go`) |
| Telegram-Kopplung | die Kopie holt Nachrichten des Bots ab, die für produktiv bestimmt sind | `internal/notify/telegram_link.go` |
| Seerr | legt Benutzer an, setzt Passwörter, Rechte und Limits, stellt Wünsche, zieht sie zurück, gibt frei und lehnt ab | `internal/seerr/calls.go`, aufgerufen aus `internal/wishes` (`provision.go`, `submit.go`, `withdraw.go`, `password.go`, `review.go`, `retry.go`) |
| Radarr, Sonarr | legt Tags an und hängt sie an | `internal/arr/calls.go` (`EnsureTag`, `AttachTag`) |
| Podcast-Ordner | lädt Folgen in denselben Ordner auf dem NAS und schreibt dort Probedateien | `internal/library/podcast_download.go`, `podcast_folder_check.go` |
| YouTube-Suche | verbraucht das Tageskontingent, das produktiv braucht | Job `trailer.find` |

Einen Schalter «nichts nach aussen» gibt es nicht. Es gibt nur Einzelschalter (Seerr aus,
Push-Schlüssel neu, Jobart pausieren), und die liegen in der Datenbank, also in der Kopie.

Zwei Dinge sind unkritisch, aber wissenswert:

- Die Vermittlung `pair.slouchcat.com` spricht nur mit Handy und Fernseher, nie mit dem Server.
  Eingehende Webhooks rufen die Adresse von produktiv.
- Die Kopie weist sich mit **demselben Serverschlüssel** aus wie produktiv (er stammt aus
  `secret.key`, `internal/auth/pairing.go`), und die kopierten Sitzungen von Browsern, Handy- und
  TV-Apps sind auf ihr gültig. Eine App geht aber nur an die Adresse, die sie sich gemerkt hat; von
  selbst landet sie nicht auf der Kopie.

### 3.5 Proxmox

| | Stand am 28. November 2026 |
|---|---|
| Host `pve2` | 12 Kerne, 62 GB RAM, davon 46 GB frei; `local-zfs` 620 GB frei |
| CT 190 | läuft, 10 Kerne, 4 GB RAM, Wurzelplatte 64 GB (24 GB belegt), Datenplatte `mp0` 64 GB (9 GB belegt), NAS als `mp1` nur lesbar, Podcast-Ordner als `mp2` schreibbar, Grafikeinheit durchgereicht (`dev0`, `dev1`) |
| CT 191, CT 192 | Nummern frei |
| Sicherung | Auftrag «alle Container auf pve2», täglich 21:00, auf den Proxmox Backup Server |
| Auf CT 190 | Server und Updater 1.44.1, alles unter `/opt/slouchcat/deploy` (20 GB, davon `cache/` der grösste Teil), `COMPOSE_FILE=compose.yaml:compose.ct.yaml:compose.intel.yaml:compose.updater.yaml:compose.podcasts.yaml`. `compose.ct.yaml` ist eine Datei nur dieses Containers; im Repository gibt es sie nicht. |

### 3.6 Versionsstellen und Anleitung

Vor jedem Release werden Versionsstellen von Hand nachgezogen (`docs/dev/HANDOVER.md`):
`deploy/compose.yaml`, `deploy/compose.updater.yaml`, `deploy/.env.example`, `deploy/README.md`,
`docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`. Die Installationsanleitung holt die Dateien unter
der festen Versionsnummer (`raw.githubusercontent.com/SlouchCat/slouchcat/v1.44.1/deploy/…`),
darunter `compose.yaml`, `.env.example` und `Caddyfile`, und nennt `SLOUCHCAT_VERSION=1.44.1`. Mit
einem Kandidaten zeigte sie damit auf ein Release, das noch nicht freigegeben ist (Abschnitt 6.6).
Am Release hängen heute nur `manifest.json`, `deploy/compose*.yaml` und der ffmpeg-Hinweis.

## 4. Die drei Umgebungen

### 4.1 Rollen

| | dev (CT 192) | test (CT 191) | produktiv (CT 190) |
|---|---|---|---|
| Zweck | Unfertiges ansehen und abnehmen, bevor es gemergt ist | Kandidat und Update-Schritt prüfen | Betrieb |
| Image | Zweig-Image, auf dem Container gebaut (wie bisher auf CT 190) | nur Release-Images von ghcr.io | nur Release-Images von ghcr.io |
| Wechsel der Version | Skript `dev-deploy` (Abschnitt 11) | Update-Funktion, sonst nur das Auffrischen (Abschnitt 10) | Update-Funktion |
| Update-Kanal | keiner | `candidate` | `stable` (Vorgabe) |
| Updater-Begleiter | nein | ja | ja |
| Testbetrieb | an | an | aus |
| Name im Kopf der Oberfläche | «Dev» | «Test» | keiner |
| Daten | Kopie von produktiv, vor jedem neuen Zweig frisch | Kopie von produktiv, vor jedem Kandidaten frisch | echt |
| Medien | NAS nur lesbar, auch der Podcast-Ordner | NAS nur lesbar, auch der Podcast-Ordner | wie heute |

### 4.2 Wer was darf

| Handlung | dev | test | produktiv |
|---|---|---|---|
| Zweig-Image einspielen | Claude, ohne Marcos Wort | nie | nie |
| Daten auffrischen | Claude, ohne Marcos Wort | Claude, als Teil von Freigabe 1 | ist die Quelle, wird nur gelesen |
| Update einspielen | – | Marco, über die Update-Funktion | Marco, über die Update-Funktion |
| `.env` oder Compose-Dateien ändern | Claude | Claude, mit Ansage | nur auf Marcos Wort |
| Daten ändern, löschen | frei | frei (geht beim Auffrischen verloren) | nur Marco |

«Ohne Marcos Wort» heisst: Es braucht keine Freigabe im Gespräch (Frage F-9). Schreibende Befehle
auf dem Proxmox-Host und in den Containern laufen wie bisher über das Terminal-Panel
(`docs/dev/HANDOVER.md`); eine Rückfrage der App dort bleibt möglich.

Zwei Sätze daraus sind neu und werden Regel (Abschnitt 13):

- **Auf produktiv läuft nie mehr ein Zweig-Image.** Die Abnahme vor dem Merge wandert auf dev.
- **Auf test läuft nie ein Zweig-Image.** Sonst prüft test nicht mehr den Update-Schritt.

## 5. Der Weg einer Änderung

### 5.1 Schritte

| Schritt | Wer | Was | Prüfung |
|---|---|---|---|
| 1 | Claude | Arbeit auf einem Zweig. Zum Ansehen: Zweig-Image auf dev. Beliebig oft. | Teststufe 1 (`task check:fast`, betroffene Tests), dazu die Wächter des Testbetriebs (Abschnitt 11); keine CI |
| 2 | Marco | Sieht es auf dev an. **Freigabe 1: «für test freigeben».** | |
| 3 | Claude | Pull Request, CI grün, Merge, Versionsstellen, Tag `vX.Y.Z`. Der Release-Lauf veröffentlicht den **Kandidaten**: gebaut, signiert, als Vorabversion und ohne «latest». | Teststufe 2 und 3 (`task ci`, GitHub CI) |
| 4 | Claude | Frischt test auf: Daten und Version von produktiv (Abschnitt 10). Vor **jedem** Kandidaten, auch vor einer Eilkorrektur. | Skript prüft Version und Testbetrieb |
| 5 | Marco | Auf test: Verwaltung › Server zeigt «Version X.Y.Z verfügbar». «Installieren». Prüft. | **Teststufe 4 (neu): Abnahme am Kandidaten** |
| 6 | Marco | **Freigabe 2: «für produktiv freigeben».** | |
| 7 | Claude | Startet den Freigabe-Lauf (Abschnitt 6.4): «latest» zeigt jetzt auf X.Y.Z. | Lauf prüft Version und Digest gegen den Zeiger |
| 8 | Marco | Auf produktiv: «Jetzt prüfen», «Installieren» (wie heute). | |

Schritt 2 und 3 fallen heute schon so an («Merge auf Marcos Wort»). Neu ist, dass ein Tag nicht
mehr gleich produktiv erreicht.

Tag und Freigabe-Lauf sind öffentliche Handlungen. Claude führt sie nur auf Marcos ausdrückliches
Wort für genau dieses Release aus (Freigabe 1, Freigabe 2), oder Marco führt sie selbst aus
(Frage F-1).

Mehrere Pull Requests lassen sich wie heute sammeln: Sie werden einzeln gemergt, und der Kandidat
entsteht erst mit dem Tag. Dev kann dafür auch `main` zeigen; `main` zählt dort als Zweig.

### 5.2 Fehler auf test

Der Kandidat X.Y.Z hat einen Fehler. Dann:

1. Korrektur auf einem Zweig, ansehen auf dev, Freigabe 1, Tag **X.Y.(Z+1)**. Ein Tag wird nie
   verschoben und nie neu gebaut.
2. Test wird wieder aufgefrischt (zurück auf den Stand von produktiv) und geht von dort direkt auf
   X.Y.(Z+1). Das ist der Schritt, den produktiv machen wird.
3. X.Y.Z wird nie «latest». Der Release-Lauf des Nachfolgers schreibt in den Titel des verdrängten
   Releases «nicht freigegeben, ersetzt durch X.Y.(Z+1)»; es bleibt Vorabversion (Frage F-6).

Folgen: Versionsnummern können Lücken haben (produktiv geht von 1.44.1 auf 1.45.1). Die
Release-Notizen eines Nachfolgers nennen darum **alles seit dem letzten freigegebenen Release**,
nicht nur die Korrektur. Produktiv zeigt nur die Notizen der Version, die es einspielt.

### 5.3 Ein Kandidat auf einmal; Eilkorrektur

**Es gibt immer nur einen Kandidaten. Ein neuer Tag ersetzt den liegenden.** Wer taggt, während
ein Kandidat in test liegt, verdrängt ihn; das ist die Bedeutung des Tags, und Freigabe 1 sagt es
dazu. Getaggt wird nie, solange ein Release-Lauf läuft.

Eilkorrektur: Produktiv hat einen Fehler, und in test liegt ein Kandidat, der noch nicht
freigegeben werden kann. Dann entsteht die Korrektur auf einem Zweig **ab dem Tag von «latest»**
und bekommt die nächste Patch-Nummer dieser Linie (produktiv 1.44.1, Kandidat 1.45.0,
Eilkorrektur 1.44.2). Sie geht denselben Weg: Kandidat, test auffrischen, test, Freigabe 2. Der
Tag 1.44.2 verdrängt den Kandidaten 1.45.0. Die Korrektur muss auch nach `main`; der nächste
Kandidat von dort (1.45.1) enthält sie.

Ein verdrängter Kandidat lässt sich nicht mehr freigeben: Der Freigabe-Lauf nimmt nur, was der
Zeiger nennt (Abschnitt 6.4). So kann 1.45.0 nach der Eilkorrektur nicht ohne sie auf produktiv
kommen.

### 5.4 Zurück nach einem Update auf produktiv

Unverändert (ADR 0024): Schlägt eine Installation fehl und lief keine Migration, dreht der Updater
selbst zurück. Lief eine Migration, bleibt die Sicherung `pre-migration-*.db` der Weg zurück. Dazu
kommt die tägliche Sicherung des ganzen Containers. Der Sinn von test ist, dass dieser Fall auf
produktiv nicht mehr eintritt, weil die Migration vorher an einer Kopie desselben Bestands lief.

## 6. Kandidat und Freigabe

### 6.1 Regel: einmal bauen

Das Image, das test geprüft hat, ist das Image, das produktiv bekommt: derselbe Digest. Darum gibt
es keine Tags mit Zusatz (`-rc1`) für diesen Weg. Sie bleiben, was sie heute sind: eine
Vorabversion, die niemandem angeboten wird.

### 6.2 `release.yml`: veröffentlichen ohne «latest»

Der Lauf bleibt, wie er ist (zwei Images für amd64 und arm64, Herkunftsnachweis, `manifest.json`,
Compose-Dateien als Anhang), mit diesen Änderungen:

1. Ein Tag `vX.Y.Z` wird **nie** beim Bauen «latest»: kein Image-Tag `:latest`, und das Release
   entsteht mit `--prerelease --latest=false`. Die Markierung als Vorabversion ist der Riegel:
   GitHub macht eine Vorabversion nie zu «latest», auch nicht, wenn es «latest» einmal neu
   bestimmt. Der Abruf in Server und Updater schaut auf diese Markierung nicht; test stört sie
   nicht.
2. **Zeiger:** Hat **dieser Lauf das Release angelegt** und ist die Version höher als die des
   Releases, das GitHub gerade als «latest» führt (nicht: als der höchste Tag), lädt der Lauf
   dasselbe `manifest.json` in das Zeiger-Release hoch (Abschnitt 6.3). Nennt der Zeiger vorher
   eine andere Version, die nicht «latest» ist, bekommt deren Release den Vermerk aus
   Abschnitt 5.2. Ein Lauf, der ein Release schon vorfindet (ein alter Lauf, von Hand neu
   gestartet), fasst den Zeiger nicht an; sonst zeigte er zurück auf einen verdrängten Kandidaten.
3. **Ein Lauf auf einmal:** `concurrency` mit einer festen Gruppe und ohne Abbruch laufender
   Läufe. Zwei Läufe können den Zeiger so nicht in falscher Reihenfolge schreiben. GitHub hält je
   Gruppe nur einen wartenden Lauf und bricht einen älteren wartenden ab; darum gilt die Regel aus
   Abschnitt 5.3, nie zu taggen, solange ein Release-Lauf läuft.
4. **Anhänge:** `deploy/.env.example` und `deploy/Caddyfile` hängen neu am Release (Abschnitt 6.6).
5. Die Zusammenfassung des Laufs sagt in einem Satz, was jetzt gilt: «Kandidat X.Y.Z
   veröffentlicht, «latest» bleibt A.B.C».

### 6.3 Der Zeiger des Kanals «Kandidat»

Der Kanal braucht eine feste Adresse, wie «latest» eine ist. Vorschlag: ein Release mit dem festen
Tag **`candidate`**, das nur eine Datei trägt, `manifest.json`. Die Adresse ist dann

`https://github.com/SlouchCat/slouchcat/releases/download/candidate/manifest.json`

- Das Zeiger-Release wird einmal von Hand angelegt, als Vorabversion und ohne «latest»; sein Titel
  sagt, was es ist («Zeiger auf den Kandidaten, kein Release»).
- `release.yml` ersetzt die Datei bei jedem Kandidaten (`gh release upload candidate manifest.json
  --clobber`).
- Der Abruf im Server und im Updater verarbeitet diese Adresse schon heute von Anfang bis Ende,
  samt Rückfall auf die API (`releases/tags/candidate`); das ist am Code geprüft. Die
  Compose-Dateien kommen weiter aus `releases/download/vX.Y.Z/`. Der Kanal «Kandidat» läuft damit
  durch denselben Code wie «stabil»; test prüft also auch den Abruf.
- Der Tag `candidate` stört nichts: `release.yml` hört nur auf `v*.*.*`, die Versionsermittlung
  (`git describe`) nur auf `v[0-9]*`, die Läufe der Apps auf `app-v*`, `tv-v*` und `relay-v*`.
- Wer den Zeiger schreiben darf, darf auch «latest» setzen (Schreibrecht auf das Repository). Der
  Zeiger ist so vertrauenswürdig wie «latest».

Verworfen wurde, die Liste aller Releases über die API zu lesen und das höchste zu nehmen
(Abschnitt 17).

### 6.4 Neuer Lauf `promote.yml`: Freigabe 2

Startet nur von Hand (`workflow_dispatch`), mit der Version als Eingabe und einem Probelauf als
Vorgabe (`dry_run: true` zeigt nur, was er täte).

Er prüft, in dieser Reihenfolge:

1. Die Eingabe hat die Form `X.Y.Z` (geprüft, bevor sie irgendwo eingesetzt wird).
2. Es läuft kein Release-Lauf. Sonst könnte der Zeiger sich während der Freigabe ändern.
3. **Der Zeiger nennt genau diese Version**, und `image` und `updaterImage` im Manifest des Zeigers
   sind dieselben Digests wie im Manifest des Releases `vX.Y.Z`. Freigegeben wird also nur der
   Kandidat, der gerade in test liegt.
4. Das Release trägt nicht den Vermerk «nicht freigegeben».
5. Die Version ist höher als die von «latest». Ein Rückschritt wird abgelehnt.
6. Beide Digests gibt es in der Registry, und ihr Herkunftsnachweis ist gültig
   (`gh attestation verify`).

Er tut:

1. `docker buildx imagetools create -t <image>:latest <image>@sha256:…` für Server und Updater.
   Das hängt nur einen Namen um; gebaut wird nichts.
2. `gh release edit vX.Y.Z --prerelease=false --latest`.
3. Schreibt in die Zusammenfassung: Version, beide Digests, wer gestartet hat.

Rechte: `contents: write`, `packages: write`, dazu was `gh attestation verify` zum Lesen braucht
(Abschnitt 18). Der Lauf hat eine eigene `concurrency`-Gruppe; teilte er die von `release.yml`,
könnte ein wartender Freigabe-Lauf einen wartenden Release-Lauf abbrechen.

Der Zeiger `candidate` bleibt nach der Freigabe stehen. Er zeigt dann auf dieselbe Version wie
«latest», und test meldet «aktuell».

### 6.5 Versionsnummern und Notizen

- Jeder Kandidat hat eine eigene, endgültige Nummer. Nummern nicht freigegebener Kandidaten bleiben
  verbraucht (Abschnitt 5.2).
- Die Regel «Vertrag = Tag» bleibt: Jede veröffentlichte Vertragsfassung hat ihren Tag, auch die
  eines Kandidaten.
- `docs/dev/VERTRAEGLICHKEIT.md` misst «von einem Release aufs nächste». Massgeblich wird das
  zuletzt **freigegebene** Release, denn nur das hat je eine App gesehen. Der eingefrorene Stand
  wird nach Freigabe 2 gezogen und immer mit dem freigegebenen Tag
  (`task server:freeze:x-clients REF=vX.Y.Z`); die Vorgabe «nächster Tag» träfe sonst einen
  Kandidaten.

### 6.6 Versionsstellen und Anleitung

Niemand, der neu installiert, darf einen Kandidaten erwischen.

- **Teil des Releases** und damit vor dem Tag nachgezogen wie heute: `deploy/compose.yaml`,
  `deploy/compose.updater.yaml`, `deploy/.env.example`. Sie hängen am Release.
- **Anleitung:** `docs/INSTALLATION.md` und `deploy/README.md` nennen keine Versionsnummer mehr,
  weder in Adressen noch im Text. Sie holen `compose.yaml`, `.env.example` und `Caddyfile` von
  «latest» (`…/releases/latest/download/<name>`). Die Anleitung folgt damit von selbst der
  Freigabe und muss je Release nicht mehr angefasst werden.
- Beides kommt schon mit Stufe 1 (Anhänge und Anleitung zusammen), damit die Adressen der
  Anleitung nie ins Leere zeigen.
- `docs/dev/HANDOVER.md` beschreibt den neuen Ablauf einmal und nennt keine laufende Version mehr.

### 6.7 Apps und Vermittlung

- `release-android.yml` und `release-relay.yml` bleiben unverändert. App-Releases werden nie
  «latest», die Vermittlung legt kein Release an.
- Die bekannte Falle bleibt und wird nicht grösser: Ist kein Server-Release «latest», kann GitHub
  auf ein App-Release zurückfallen (`docs/dev/ANDROID-INBETRIEBNAHME.md`, Abschnitt 7). Im neuen
  Weg bleibt immer das zuletzt freigegebene Server-Release «latest», und Kandidaten sind
  Vorabversionen, auf die GitHub nicht zurückfällt.
- **Neu als Regel:** Eine App-Version, die eine neue Vertragsfassung braucht, geht erst in den
  Store, wenn das Server-Release freigegeben ist (Freigabe 2) und produktiv es eingespielt hat.
- Handy- und TV-App gegen test prüfen geht im Heimnetz. Die App muss dafür auf die Adresse von
  test umgestellt werden und meldet sich dort neu an. Am einfachsten ist ein zweites Gerät oder
  der Emulator.

## 7. Update-Kanal im Server und im Updater

Neue Einstellung **`SLOUCHCAT_UPDATE_CHANNEL`**, gelesen von Server und Begleiter:

| Wert | Manifest | Für |
|---|---|---|
| leer oder `stable` | `releases/latest/download/manifest.json` | alle, wie heute |
| `candidate` | `releases/download/candidate/manifest.json` | Testinstanzen |

- Ein anderer Wert ist ein Konfigurationsfehler; der Server startet nicht, der Begleiter auch
  nicht.
- **Der Kanal ist eine Auswahl aus zwei festen Adressen, keine freie Adresse.** Das Manifest
  bestimmt, welches Image läuft (ADR 0024). Eine frei wählbare Adresse ausserhalb des
  Entwicklungsmodus bleibt im Server ausgeschlossen.
- `SLOUCHCAT_UPDATE_MANIFEST_URL` behält seine heutige Bedeutung (Server: nur Entwicklungsmodus;
  Begleiter: Spiegel und Tests). **Sind im Begleiter Kanal und Adresse zugleich gesetzt, startet
  er nicht.** Sonst läse der Server das eine Manifest und der Begleiter das andere.
- `deploy/compose.yaml` und `deploy/compose.updater.yaml` reichen die Variable durch, je mit
  leerer Vorgabe (`${SLOUCHCAT_UPDATE_CHANNEL:-}`). Ein Test im Repository belegt, dass beide
  Dateien sie durchreichen, und ebenso `SLOUCHCAT_SANDBOX` und `SLOUCHCAT_INSTANCE_NAME`. Die
  Compose-Datei ist eine Liste erlaubter Variablen; was dort fehlt, kommt im Container nicht an.
- Der Updater ändert in `.env` nur die Zeile `SLOUCHCAT_VERSION` (und beim Selbstupdate
  `SLOUCHCAT_UPDATER_VERSION`) und lässt fremde Compose-Dateien stehen. Der Kanal überlebt jedes
  Update.
- Sichtbar: Verwaltung › Server zeigt bei den Fakten «Update-Kanal: Kandidat», wenn er nicht
  «stabil» ist. Der Vertrag bekommt dafür additiv `UpdateState.channel` (`stable`, `candidate`).
  Die Operation ist `admin` und trägt kein `x-clients`.
- Der Kanal steht in `docs/INSTALLATION.md` als Einstellung für Testinstanzen. Wer ihn auf einem
  eigenen Server setzt, bekommt Releases vor der Freigabe; das ist dann seine Wahl.

Was sich **nicht** ändert: Es gibt weiter keine Wahl einer bestimmten Version und keinen
Rückschritt (ADR 0024, Punkt 9). `ValidRelease` und `Newer` bleiben.

## 8. Testbetrieb: keine Wirkung nach aussen

Neue Einstellung **`SLOUCHCAT_SANDBOX`**. In der Oberfläche heisst sie «Testbetrieb».

- **Werte:** `on`, `off` oder leer. Jeder andere Wert (`true`, `1`, `On`) ist ein
  Konfigurationsfehler, und der Server startet nicht. Ein Tippfehler darf nicht «aus» bedeuten.
- **Sie steht nur in der Umgebung, nie in der Datenbank und nie in der Verwaltung.** Eine kopierte
  Datenbank kann sie darum weder ein- noch ausschalten; sie gehört dem Container, nicht den Daten.
- **Marke der Kopie:** Liegt in `config/` die Datei `sandbox-required`, startet der Server nur mit
  `SLOUCHCAT_SANDBOX=on` und bricht sonst mit einer klaren Meldung ab, bevor er die Warteschlange
  anfasst. Das Auffrischen legt die Datei in jede Kopie (Abschnitt 10). Auf produktiv gibt es sie
  nie; eine Sicherung aus der Verwaltung enthält sie nicht.

### 8.1 Was gesperrt ist

Es gibt **eine** Fehlerklasse «Testbetrieb». Ein Job, der auf sie trifft, endet «übersprungen»:
kein Fehler, keine Wiederholung, keine Fehlermeldung an Administratoren. Ein Aufruf aus der
Oberfläche wird mit einer eigenen Problemart des Vertrags abgelehnt, und die Seite zeigt den Text
dazu.

| Stelle | Ausgelöst durch | Im Testbetrieb | Was man sieht |
|---|---|---|---|
| Zustellung in `internal/notify` (`deliver`, `sendOn`): E-Mail, Web-Push, Push an die Handy-App, Telegram; auch Einladungen und «Gerät verbunden» | Ereignisse; wartende Zustellungen der kopierten Warteschlange | übersprungen, Grund «Testbetrieb» | Verlauf der Benachrichtigungen nennt den Grund |
| Testnachrichten, darunter `SendTestEmail` (verschickt heute an `deliver` vorbei) | Knopf «Testnachricht» | abgelehnt | Meldung am Knopf |
| Telegram: Versand und Abruf beim Bot (`getUpdates`), Bestätigung der Kopplung | Kopplung, Meldungen | kein Abruf, kein Versand. Die reine Verbindungsprüfung (`getMe`) bleibt. | Seite sagt, dass Kopplung im Testbetrieb nicht geht |
| Seerr schreiben: `CreateUser`, `SetPassword`, `SetPermissions`, `SetUserSettings`, `CreateRequest`, `DeleteRequest`, `ApproveRequest`, `DeclineRequest`, `RetryRequest` | Jobs `seerr.users`, `seerr.provision`, `wishes.submit`, `wishes.withdraw`; Passwortabgleich; Verwaltung: Freigeben, Ablehnen, Erneut versuchen | Jobs: übersprungen. Aus der Oberfläche: abgelehnt. | Meldung |
| Wunsch anlegen | Zuschauer | abgelehnt, **bevor ein Wunsch gespeichert wird** (heute speichert der Server den Wunsch, versucht Seerr bis zu drei Sekunden lang und überlässt den Rest der Warteschlange) | «Im Testbetrieb werden keine Wünsche gestellt.» |
| Passwort eines Kontos ändern | Konto, Verwaltung | Das Passwort in Slouchcat ändert sich; der Abgleich mit Seerr wird übersprungen. | Hinweis |
| Radarr, Sonarr schreiben: `EnsureTag` (anlegen), `AttachTag` | Job `wishes.tag` | übersprungen | – |
| Podcast: Folgen laden (`podcast.download`), Schreibprobe des Ordners (`podcast_folder_check.go`) | Zeitplan, Verwaltung | übersprungen; der Ordner gilt als «nur lesbar» | Seite sagt es |
| Dateien einer Bibliothek löschen (`delete_files.go`) | Verwaltung | abgelehnt | Meldung |
| Quellen mit kleinem Tageskontingent (YouTube-Suche) | Job `trailer.find` | für diese Quelle übersprungen | – |

**Ausdrücklich erlaubt**, obwohl technisch ein POST: der Prüfaufruf `CSRFProtection` bei Seerr
(Verbindungstest), die Anmeldung bei TVDB, Abfragen bei AniList und Wikidata. Sie lesen nur.

Gesperrt wird an **benannten Stellen**, nicht pauschal nach HTTP-Methode. Seerr und Radarr/Sonarr
haben je eine Stelle, durch die alle Aufrufe gehen (`Session.exchange`, `Client.exchange`); dort
entscheidet eine Liste der schreibenden Aufrufe. Eine Sperre nach Methode träfe die lesenden POST
oben.

### 8.2 Was bleibt

Alles, was nur liest oder nur im Container wirkt: Update-Prüfung und Updater (dafür ist test da),
Metadaten und Bilder (TMDB, TVDB, Fanart, AniList, Wikidata, Radio-Browser, Google Books, Trakt),
Podcast-Feeds lesen, Seerr, Radarr und Sonarr lesen (Wunschstand, Suche, Warteschlange),
eingehende Webhooks, Abspielen, Umwandeln, Scannen, Downloads für offline.

Die kopierte Warteschlange läuft im Ziel normal an. Was davon gesperrt ist, endet nach
Abschnitt 8.1 «übersprungen»; der Rest läuft.

### 8.3 Sichtbar

- Verwaltung: ein Hinweisband auf jeder Seite, «Testbetrieb: Dieser Server ist eine Kopie. Er
  verschickt nichts und ändert nichts in Seerr, Radarr und Sonarr.»
- Verwaltung › Server: Fakt «Testbetrieb: an».
- Seiten, deren Wirkung gesperrt ist, sagen es an der Stelle (Spalte «Was man sieht»).
- Beim Start schreibt der Server eine `WARN`-Zeile `sandbox=on`.
- Vertrag, additiv: `Health.sandbox` (Wahrheitswert) und die Problemart «Testbetrieb».
  `getHealth` ist öffentlich und wird von den Apps gelesen; ein zusätzliches Feld ist nach
  `docs/dev/VERTRAEGLICHKEIT.md` erlaubt.

Texte kommen aus `web/src/locales` (`de`, `en`, `de-DE`; die übrigen Sprachen vor dem Release nach
`docs/i18n/README.md`) und aus `server/internal/notify/locales`, Farben aus Tokens, Bauteile aus
`web/src/ui` (Regel 9). Das Hinweisband und die Marke aus Abschnitt 9 sind neue Bauteile; sie
bekommen eine Storybook-Geschichte und laufen durch `task storybook:test`.

### 8.4 Wächter

Die Sperre ist eine Liste, und eine Liste veraltet. Vier Wächter halten sie aktuell:

1. **Je Zeile der Tabelle ein Test**, der mit einem falschen Gegenüber belegt: Im Testbetrieb
   erreicht kein Aufruf das Netz, der Job endet «übersprungen», die Oberfläche bekommt die
   Problemart.
2. **Ein Listentest** nach dem Muster von `TestContractCoversEveryOperation`. Er führt eine
   geprüfte Liste aller Stellen im Server, die nach aussen senden können: jeder HTTP-Aufruf mit
   anderer Methode als GET und HEAD, der Versand über SMTP, der Versand über Web-Push, jede
   Stelle, die in einen Medienordner schreibt oder dort löscht. Jede Stelle ist eingeordnet als
   «im Testbetrieb gesperrt» oder «liest nur». Findet der Test eine Stelle, die nicht in der Liste
   steht, wird er rot.
3. **Ein e2e-Test** im Testbetrieb: Hinweisband sichtbar, ein Wunsch wird mit Meldung abgelehnt,
   eine Testnachricht wird abgelehnt.
4. **Vor jedem Einspielen auf dev** laufen Wächter 1 und 2 für den Zweig (Abschnitt 11). Dev ist
   die eine Umgebung, auf der ungeprüfter Code auf eine Kopie echter Daten trifft.

### 8.5 Grenzen

- Push, Mail und Wünsche lassen sich auf test **nicht** bis zum Empfänger prüfen. Wer das braucht,
  prüft es vor dem Merge mit den vorhandenen Attrappen oder nach der Freigabe auf produktiv. Eine
  Ausnahme «nur an dieses eine Konto» ist bewusst nicht Teil dieses Papiers (Frage F-4).
- Der Testbetrieb schützt die Aussenwelt vor der Kopie. Er schützt nicht die Kopie: Auf test und
  dev liegen die echten Konten und Sitzungen. Darum sind beide nur im Heimnetz erreichbar.
- Die Sperre greift an benannten Stellen. Ein Riegel im Netzwerkweg aller Clients, der Unbekanntes
  von selbst sperrt, wäre dichter, verlangt aber den Umbau von rund zwanzig Stellen, an denen der
  Server heute einen HTTP-Client anlegt (Frage F-11).

## 9. Name der Instanz

Drei Instanzen mit denselben Daten sehen gleich aus. Die Gefahr ist, auf produktiv etwas zu
löschen, das für test gedacht war.

Neue Einstellung **`SLOUCHCAT_INSTANCE_NAME`** (freier Text, höchstens 20 Zeichen, leer auf
produktiv):

- Die Web-App zeigt den Namen als Marke im Kopf jeder Seite, bei Zuschauern und in der Verwaltung,
  und stellt ihn dem Seitentitel voran. Das Muster des Titels («[{name}] Slouchcat») steht in den
  Sprachdateien.
- Vertrag, additiv: `Health.instanceName`.
- Der Name selbst ist Daten, kein Text der Oberfläche (er kommt aus der Umgebung).
- Die Android-Apps zeigen ihn in dieser Stufe nicht (Frage F-10).

## 10. Test und dev auffrischen

Ein Skript auf dem Proxmox-Host setzt eine Umgebung auf den Stand von produktiv. Produktiv läuft
dabei weiter und wird nur gelesen.

### 10.1 Die Datei, die dem Ziel gehört

Was eine Umgebung von produktiv unterscheidet, steht in **einer Compose-Datei, die nur im Ziel
liegt und nie kopiert wird**, `compose.env.yaml`, mit festen Werten statt Verweisen auf `.env`:

| Einstellung | test | dev |
|---|---|---|
| `SLOUCHCAT_SANDBOX` (Server) | `on` | `on` |
| `SLOUCHCAT_INSTANCE_NAME` (Server) | `Test` | `Dev` |
| `SLOUCHCAT_UPDATE_CHANNEL` (Server und Begleiter) | `candidate` | – |
| `SLOUCHCAT_PUBLIC_URL` (Server) | Adresse von test im Heimnetz | Adresse von dev im Heimnetz |
| `SLOUCHCAT_WORKERS` (Server) | `1` | `1` |

Sie steht als letzte in `COMPOSE_FILE`. Der Schalter hängt damit weder an der kopierten `.env`
noch an den kopierten Compose-Dateien. Der Updater lässt die Datei stehen, weil ihr Name nicht im
Manifest vorkommt.

`COMPOSE_FILE` im Ziel ist die Liste von produktiv **ohne `compose.podcasts.yaml`** (bei dev auch
ohne `compose.updater.yaml`), dahinter `compose.env.yaml`. Ohne die Podcast-Datei bleibt der
Podcast-Ordner am selben Pfad sichtbar, aber nur lesbar, über die Medien-Einhängung.

`SLOUCHCAT_PUBLIC_URL` aus der Umgebung geht der gespeicherten Einstellung vor
(`internal/admin/settings.go`); so nennt keine Seite von test die Adresse von produktiv.
`SLOUCHCAT_WORKERS=1` hält die Last auf dem NAS klein (Risiko R-5).

### 10.2 Ablauf

1. **Vorprüfung an der Quelle:** Produktiv ist gesund (`healthz`), der Updater dort installiert
   gerade nichts, und es läuft keine Migration (Zustandsdateien in `updater/`). Sonst bricht das
   Skript ab. Ein Schnappschuss mitten in einer Migration wäre kein gültiger Stand.
2. **Vorprüfung am Ziel:** `compose.env.yaml` ist da und nennt `SLOUCHCAT_SANDBOX: "on"`; auf der
   Platte ist genug Platz; der Podcast-Ordner von produktiv (`PODCAST_DIR`, `PODCAST_TARGET`) liegt
   unter der Medien-Einhängung, sodass er im Ziel ohne `compose.podcasts.yaml` am selben Pfad
   erscheint (auf CT 190 ist das so: `/srv/media/nas/…` wird `/media/nas/…`). Sonst Abbruch,
   bevor etwas kopiert ist.
3. Stack im Ziel anhalten.
4. ZFS-Schnappschuss der Wurzelplatte von CT 190. Ein Schnappschuss ist ein Stand in einem
   Augenblick; die Datenbank darin ist so gültig wie nach einem Stromausfall, und SQLite ist dafür
   gebaut.
5. Aus dem Schnappschuss ins Ziel kopieren, mit `rsync --delete`, damit nichts vom letzten Mal
   stehen bleibt:
   - `config/` ganz, ohne `config/restore/` (eine dort bereitgelegte Wiederherstellung liefe sonst
     im Ziel an).
   - `cache/` ohne `transcode`, `trickplay-session` und `radio`. `cache/downloads` (vorbereitete
     Downloads) wird mitkopiert.
   - `.env` und, als feste Liste, die Compose-Dateien von produktiv. Die `.env` wird als Datei
     kopiert und nie ausgegeben (Regel 10).
6. Im Ziel: `COMPOSE_FILE` in der kopierten `.env` nach Abschnitt 10.1 setzen; die Datei
   `config/sandbox-required` anlegen.
7. Schnappschuss löschen.
8. **Prüfung vor dem Start:** `docker compose config` im Ziel muss für den Server
   `SLOUCHCAT_SANDBOX` gleich `on` ausweisen. Sonst startet das Skript nichts.
9. Stack starten.
10. **Nachprüfung:** `GET /api/v1/healthz` des Ziels meldet die Version von produktiv,
    `sandbox: true` und den Namen des Ziels. Stimmt eines nicht, hält das Skript den Stack an und
    meldet den Fehler.

`SLOUCHCAT_VERSION` und `SLOUCHCAT_UPDATER_VERSION` kommen mit der `.env` von produktiv. Nach
Schritt 10 steht test dort, wo produktiv steht. Die Update-Prüfung läuft 60 Sekunden nach dem
Start und zeigt den Kandidaten.

Drei Riegel stehen damit vor dem ersten Job einer Kopie: die Datei des Ziels (10.1), die Prüfung
vor dem Start (Schritt 8) und die Marke in `config/`, an der der Server selbst den Start ohne
Testbetrieb verweigert (Abschnitt 8).

### 10.3 Was nicht kopiert wird

Der gemeinsame Ordner des Updaters (`updater/`: Token, Zustand), die drei genannten Cache-Ordner
und `compose.env.yaml`. Der Updater im Ziel legt seinen Zustand selbst neu an.

### 10.4 Wo das Skript liegt

`scripts/env/` im Repository (`refresh.sh`, `dev-deploy.sh`), dazu die Anleitung
`docs/dev/UMGEBUNGEN.md`. Die Skripte laufen als `root` auf dem Proxmox-Host, fragen nichts ab,
was geheim ist, und schreiben keine Geheimnisse in ihre Ausgabe. Die bisherigen Einmalskripte je
Zweig (`.omc/ct190-*.sh`) fallen weg.

## 11. Dev

- **Einspielen:** `dev-deploy.sh <Zweig oder Commit>` baut das Image auf CT 192 aus
  `git archive` (der Weg, der heute auf CT 190 läuft), stellt `SLOUCHCAT_VERSION` um und startet.
  Kein Pull Request, keine CI.
- **Vor dem Bau** lässt das Skript für diesen Stand die Wächter 1 und 2 aus Abschnitt 8.4 laufen
  (ein gezielter `go test`-Aufruf) und baut bei Rot nicht. `task check:fast` allein reicht dafür
  nicht: Es prüft nur geänderte Pakete und könnte den Listentest auslassen.
- **Belegung:** Dev zeigt einen Zweig auf einmal. Das Skript schreibt beim Einspielen auf, welcher
  Zweig seit wann läuft, zeigt das vor dem nächsten Einspielen an und verlangt `--replace`, wenn
  es ein anderer Zweig ist. Mehrere Sessions nebeneinander überschreiben sich so nicht still.
- **Daten sind Wegwerfware.** Migrationen laufen nur vorwärts, und zwei Zweige können dieselbe
  Migrationsnummer verschieden füllen. Darum frischt das Skript dev **immer** auf, ausser es läuft
  derselbe Zweig weiter und der Aufruf nennt `--keep-data`.
- **Kein Updater, kein Kanal.** Die Update-Prüfung hat auf dev keine Bedeutung. Der Bau stempelt
  Zweig-Images so, dass die Verwaltung sie als Entwicklungsstand führt und kein «Installieren»
  anbietet; ein Stand nach `git describe` gälte heute als Release (`internal/admin/update.go`,
  `isRelease`).
- Der Stack auf dem Entwicklungsrechner (Port 8096) bleibt für die schnellste Schleife. Dev ist
  für das, was er nicht kann: echte Daten, NAS, Grafikeinheit, und Marco am Handy.

## 12. Proxmox

### 12.1 Container

| | produktiv CT 190 | test CT 191 | dev CT 192 |
|---|---|---|---|
| Stand | bleibt, wie er ist | neu | neu |
| Kerne, Gewicht (`cpuunits`) | 10, 512 | 10, 256 | 6, 128 |
| RAM | 4 GB | 4 GB | 8 GB (baut Images) |
| Wurzelplatte | 64 GB | 64 GB | 96 GB (Images und Bau-Cache) |
| Lokale Medien (`/srv/media`) | eigene Platte `mp0` | die Platte von CT 190, nur lesbar eingehängt | die Platte von CT 190, nur lesbar eingehängt |
| NAS | `mp1`, nur lesbar | `mp1`, nur lesbar | `mp1`, nur lesbar |
| Podcast-Ordner | `mp2`, schreibbar | kein `mp2`: nur lesbar über `mp1` | kein `mp2`: nur lesbar über `mp1` |
| Grafikeinheit | `dev0`, `dev1` | wie CT 190 | wie CT 190 |
| Start mit dem Host | ja | ja | ja |

Test ist so gross wie produktiv, damit Zeiten vergleichbar sind; das kleinere Gewicht gibt
produktiv bei Engpass den Vortritt. Summe RAM 16 GB von 46 GB frei, Platten 224 GB von 620 GB.

`scripts/proxmox/create-testserver.sh` kennt Nummer, Name, Kerne, RAM und die Grösse der
Datenplatte als Schalter, legt aber immer eine neue Datenplatte an und hat keine Wahl für die
Wurzelplatte. Es wird erweitert (Grösse der Wurzelplatte, Einhängen einer bestehenden Platte nur
lesbar statt einer neuen) und heisst danach nach dem, was es tut.

### 12.2 Namen und Adressen

- Feste Adressen im Heimnetz und feste Namen im DNS des Routers, zum Beispiel `slouchcat-test` und
  `slouchcat-dev`. Namen erst anlegen, dann abfragen (der DNS merkt sich «gibt es nicht»).
- Kein Eintrag bei Cloudflare, kein Eintrag im Nginx Proxy Manager. `server.slouchcat.com` bleibt
  bei CT 190.
- Die Web-App ist im Heimnetz über `http://<name>:8096` erreichbar. Handy- und TV-App erlauben
  das (`network_security_config.xml`).

### 12.3 Sicherung

CT 190 ist im täglichen Auftrag. CT 191 und CT 192 sind Kopien und werden davon ausgenommen
(Frage F-7).

### 12.4 Zweiter Gurt im Netz

Die Firewall von Proxmox sperrt CT 191 und CT 192 den Weg zum Mailserver (Host und Port) und zu
`push.slouchcat.com`. Fällt der Schalter «Testbetrieb» aus, kommen Mail und Push an die Handy-App
trotzdem nicht an (Frage F-3).

Gegen Seerr, Radarr und Sonarr hilft die Firewall nicht: Test soll dort lesen (Abschnitt 8.2), und
eine Firewall kann Lesen und Schreiben nicht trennen. Gegen Web-Push an Browser hilft sie auch
nicht; diese Meldungen gehen an viele Adressen im Internet, das test für Metadaten braucht.

### 12.5 Aufräumen von CT 190

Einmalig, mit dem Wechsel der Rolle: Zweig-Images, `/root/build-*`, `design-alt.sh`,
`design-neu.sh` und alte `.env.bak-*` entfernen. `SLOUCHCAT_BROWSE_ROOTS=/` bleibt (Marcos Wunsch
vom 25. Oktober 2026). Name und Adresse bleiben (Frage F-5).

## 13. Änderungen an Regeln und Dokumenten

| Wo | Was |
|---|---|
| `CLAUDE.md`, «Arbeitsweise» | Die drei Umgebungen in drei Sätzen; «auf produktiv und test nie ein Zweig-Image»; Teststufe 4 «Abnahme am Kandidaten»; «Ein Tag ist ein Kandidat und ersetzt den liegenden; «latest» setzt nur der Freigabe-Lauf auf Marcos Wort». |
| `CLAUDE.md`, Regel 10 | unverändert; die neuen Einstellungen sind keine Geheimnisse. |
| `docs/dev/SETUP.md`, «Teststufen» | Stufe 4 und die Zuordnung dev = Stufe 1 plus Wächter des Testbetriebs. |
| `docs/dev/HANDOVER.md`, Release-Abschnitt | Neuer Ablauf (Abschnitt 5), ohne laufende Versionsnummer. Der Satz «Den Tag pusht Marco selbst» wird zu «Tag und Freigabe-Lauf nur auf Marcos ausdrückliches Wort für dieses Release, oder durch Marco selbst». Der Satz «Server-Release bleibt «latest»» wird zu «das zuletzt freigegebene bleibt «latest»». |
| `docs/dev/TESTSERVER.md` | wird `docs/dev/UMGEBUNGEN.md`: drei Container, Auffrischen, Einspielen auf dev, Wer-darf-was. Der alte Zweck «schwaches NAS zum Messen» bleibt als Abschnitt; Messungen laufen künftig auf test (Frage F-8). |
| `docs/INSTALLATION.md`, `deploy/README.md`, `deploy/.env.example` | Dateien von «latest» holen, keine Versionsnummer; die drei neuen Einstellungen, mit dem Satz, dass sie für Testinstanzen sind. |
| `docs/dev/VERTRAEGLICHKEIT.md` | «Release» heisst «freigegebenes Release»; Einfrieren mit `REF` (Abschnitt 6.5). |
| `docs/dev/ANDROID-INBETRIEBNAHME.md`, Abschnitt 7 | Verweis auf den Freigabe-Lauf; Regel aus Abschnitt 6.7. |
| ADR 0024, Nachtrag | Punkt 6 «Nur echte Releases» neu: Tag = Kandidat als Vorabversion, «latest» durch den Freigabe-Lauf; der Kanal als Auswahl aus zwei festen Adressen. |
| ADR 0056 (neu) | Drei Umgebungen, Kandidat und Freigabe, Zeiger-Release; verworfene Wege aus Abschnitt 17. |
| ADR 0057 (neu) | Testbetrieb: was gesperrt ist, warum nur aus der Umgebung, Marke der Kopie, die Wächter. |
| `docs/planning/04-entscheidungen.md` | Frage 38 mit Marcos vier Entscheiden und den Antworten auf F-1 bis F-11. |
| `docs/dev/CURRENT_STATUS.md` | je Baustufe. |

Die Nicht-Ziele bleiben unberührt (Regel 11): Der Testbetrieb sperrt Bestehendes, er steuert
nichts Neues. Die Sicherheitseigenschaften von ADR 0024 bleiben: Kanal nur aus der Umgebung, keine
freie Adresse, keine Versionswahl, kein Rückschritt.

## 14. Stufen

Drei Stufen, jede ein Pull Request. Stufe 1 muss zuerst auf produktiv laufen, denn eine Kopie
läuft mit der Version von produktiv, und erst ab Stufe 1 kennt diese Version den Testbetrieb.

### Stufe 1 – Der Server kann Testbetrieb, Name und Kanal

- Inhalt: Abschnitte 7, 8, 9; ADR 0056 und 0057; die drei Einstellungen in `deploy/.env.example`
  und in beiden Compose-Dateien; Texte; Vertrag additiv; `.env.example` und `Caddyfile` als
  Anhänge des Releases und die Anleitung ohne Versionsnummer (Abschnitt 6.6).
- Veröffentlicht auf dem **alten** Weg (der Tag wird sofort «latest»). Es ist das letzte Release,
  das so entsteht. Abnahme davor wie bisher auf CT 190.
- Fertig, wenn:
  - die Wächter 1 bis 3 aus Abschnitt 8.4 grün sind, je Zeile der Tabelle 8.1 ein Test;
  - `SLOUCHCAT_SANDBOX=true` den Start verhindert, ebenso ein Start mit `config/sandbox-required`
    ohne `SLOUCHCAT_SANDBOX=on`;
  - der Server mit `SLOUCHCAT_UPDATE_CHANNEL=candidate` die Zeiger-Adresse liest und der Begleiter
    dieselbe (Test mit falschem Gegenüber), ein ungültiger Wert den Start verhindert und der
    Begleiter mit Kanal und Adresse zugleich nicht startet;
  - der Test belegt, dass beide Compose-Dateien die drei Variablen durchreichen;
  - die Bauteile Hinweisband und Marke eine Storybook-Geschichte haben und `task storybook:test`
    grün ist;
  - `task ci` grün ist; Vertragstest, Schema-Test und `TestXClientsCompatibility` eingeschlossen.

### Stufe 2 – Container und Skripte

- Voraussetzung: produktiv läuft auf dem Release aus Stufe 1.
- Inhalt: Abschnitte 10, 11, 12; `docs/dev/UMGEBUNGEN.md`.
- Was Marco dafür tut: die Befehle auf dem Proxmox-Host und am Router freigeben oder selbst
  ausführen (Container anlegen, Einhängepunkte, Namen, Firewall, Sicherungsauftrag).
- Fertig, wenn auf der ersten Kopie belegt ist:
  - `healthz` von test und dev: Version von produktiv, `sandbox: true`, richtiger Name;
  - eine ausgelöste Testnachricht wird abgelehnt, eine wartende Zustellung aus der kopierten
    Warteschlange endet «übersprungen», und am Handy kommt nichts an;
  - ein Wunsch auf test wird mit der Meldung des Testbetriebs abgelehnt, und die Liste der
    Wünsche in Seerr ist vorher und nachher gleich; die Seite der Wünsche zeigt weiter den Stand
    aus Seerr;
  - der Podcast-Ordner auf dem NAS ist vorher und nachher gleich, und die Podcast-Bibliothek von
    test zeigt dieselben Folgen wie produktiv;
  - drei Gegenproben: Das Skript bricht ab, wenn `compose.env.yaml` fehlt; es startet nichts, wenn
    `docker compose config` den Schalter nicht ausweist; der Server verweigert den Start, wenn die
    Marke liegt und der Schalter fehlt;
  - ein Zweig-Image läuft auf dev, ein zweiter Zweig verlangt `--replace`, und ein Zweig mit rotem
    Wächter wird nicht gebaut.

### Stufe 3 – Kandidat und Freigabe

- Inhalt: Abschnitt 6; Regeln und Dokumente aus Abschnitt 13; Zeiger-Release anlegen.
- Der erste Kandidat ist die Abnahme: ein kleines Release, das den ganzen Weg geht.
- Fertig, wenn belegt ist:
  - nach dem Tag: Release als Vorabversion ohne «latest», Zeiger zeigt auf die neue Version,
    produktiv meldet «aktuell», test meldet «verfügbar»;
  - Server und Begleiter auf test lesen den **echten** Zeiger (nicht nur die Attrappe aus
    Stufe 1), und der Rückfall über die API ist einmal gegen die echte Adresse belegt;
  - test installiert über die Update-Funktion; `healthz` meldet die neue Version; der Updater
    zieht sich selbst nach;
  - `promote.yml` im Probelauf zeigt die richtigen Digests; scharf gestartet setzt er «latest» und
    `:latest`, und der Digest von `:latest` ist der aus dem Manifest;
  - danach meldet produktiv «verfügbar» und installiert;
  - zwei Gegenproben: `promote.yml` lehnt eine Version ab, die der Zeiger nicht nennt, und eine,
    die nicht höher ist als «latest».
- Rückweg, falls der neue Lauf klemmt: `gh release edit vX.Y.Z --prerelease=false --latest` von
  Hand stellt das alte Verhalten für dieses eine Release her. Der Handgriff umgeht alle Prüfungen
  des Laufs; vorher darum von Hand prüfen, dass der Zeiger diese Version und dieselben Digests
  nennt.

Stufe 2 und 3 hängen nicht voneinander ab, ausser dass Stufe 3 für ihre Abnahme test braucht.

## 15. Risiken

| Nr. | Risiko | Gegenmittel |
|---|---|---|
| R-1 | Eine Kopie startet ohne Testbetrieb und schickt Push, Mail oder schreibt in Seerr. | Drei Riegel vor dem ersten Job (Abschnitt 10.2): Datei des Ziels, Prüfung vor dem Start, Marke in `config/`. Strenge Werte des Schalters (Abschnitt 8). Firewall gegen Mail und Push-Weiterleitung (12.4). |
| R-2 | Eine neue Funktion schreibt nach aussen und kennt den Schalter nicht. | Listentest in der CI und vor jedem Einspielen auf dev (Abschnitt 8.4). |
| R-3 | Der Zeiger ist eine Datei, die ersetzt wird; während des Ersetzens kann sie kurz fehlen. | Ein 404 beendet nur diese eine Prüfung; die nächste folgt nach 15 Minuten oder auf «Jetzt prüfen». |
| R-4 | Wer neu installiert, erwischt einen Kandidaten. | Anleitung holt von «latest» (Abschnitt 6.6); Kandidaten sind Vorabversionen. |
| R-5 | Drei Instanzen lesen dasselbe NAS; neue Dateien werden dreimal untersucht. | Ein Arbeiter auf test und dev; beim Bau messen; sonst Frage F-2. |
| R-6 | Verwechslung der Instanzen. | Name im Kopf und im Seitentitel (Abschnitt 9), Hinweisband (8.3). |
| R-7 | Produktiv steht versehentlich auf dem Kanal «Kandidat». | Fakt «Update-Kanal» in der Verwaltung; das Auffrischen warnt, wenn die `.env` von produktiv einen Kanal nennt. |
| R-8 | Lücken in den Versionsnummern verwirren. | Vermerk am nicht freigegebenen Release; Notizen nennen alles seit der letzten Freigabe (Abschnitt 5.2). |
| R-9 | Die Platte von pve2 läuft voll (Images auf dev, drei Bild-Caches). | Grössen aus Abschnitt 12.1; Platzprüfung vor dem Kopieren; `dev-deploy.sh` räumt alte Zweig-Images weg. |
| R-10 | Auf test und dev liegen echte Konten; die Kopie hat denselben Serverschlüssel, und kopierte Sitzungen von Browsern und Apps gelten dort. | Nur im Heimnetz; gleicher Host und gleicher Speicher wie produktiv; nicht in der Sicherung. |
| R-11 | Der alte und der neue Release-Weg vermischen sich während des Umbaus. | Stufe 1 ausdrücklich das letzte Release auf dem alten Weg; Rückweg in Stufe 3. |
| R-12 | Zwei Tags kurz nacheinander: Der Zeiger zeigt am Ende auf den falschen, oder ein verdrängter Kandidat wird freigegeben. | `concurrency` in beiden Läufen; Regel «nie taggen, solange ein Lauf läuft»; Freigabe nur für die Version des Zeigers (Abschnitt 6.4). |
| R-13 | GitHub bestimmt «latest» neu und nimmt einen Kandidaten. | Kandidaten sind Vorabversionen (Abschnitt 6.2). |

## 16. Entscheide und Fragen

### 16.1 Entschieden (Marco, 28. November 2026, über Rückfragen)

| Nr. | Frage | Entscheid |
|---|---|---|
| E-1 | Welche Nummern? | **190 produktiv, 191 test, 192 dev.** CT 190 bleibt, wie er ist; 191 und 192 entstehen neu auf pve2. |
| E-2 | Mit welchen Daten läuft test? | **Frische Kopie von produktiv vor jedem Release.** Was auf test angelegt wird, geht beim Auffrischen verloren. |
| E-3 | Wie wird die Kopie ungefährlich? | **Schalter im Server** («Testbetrieb»), nicht ein Skript, das nach jeder Kopie einzeln abschaltet. |
| E-4 | Von wo erreichbar? | **Test und dev nur im Heimnetz.** |

### 16.2 Offen, mit Empfehlung

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-1 | Wer taggt und wer startet den Freigabe-Lauf: Claude auf Marcos Wort im Gespräch, oder verlangt GitHub zusätzlich einen Klick von Marco (geschützte Umgebung mit Pflichtprüfer)? | Claude auf Marcos ausdrückliches Wort je Release; Marco kann beides auch selbst tun. Der Pflichtklick lässt sich später ohne Umbau einschalten. |
| F-2 | Laufen test und dev dauernd, oder werden sie zwischen den Einsätzen angehalten? | Dauernd, mit einem Arbeiter. Zeigt die Messung beim Bau spürbare Last auf dem NAS, wird dev angehalten, wenn kein Zweig darauf liegt. |
| F-3 | Firewall als zweiter Gurt gegen Mail und Push-Weiterleitung (Abschnitt 12.4)? | Ja. |
| F-4 | Soll der Testbetrieb eine Ausnahme kennen, etwa Push und Mail nur an ein Testkonto? | Nein, nicht jetzt. Erst wenn sich zeigt, dass es fehlt. |
| F-5 | CT 190 heisst `slouchcat-nas`. Umbenennen? | Nein. Name und Adresse bleiben, nichts hängt um. |
| F-6 | Was geschieht mit einem Kandidaten, der nicht freigegeben wird? | Release bleibt Vorabversion und bekommt im Titel «nicht freigegeben, ersetzt durch X.Y.Z». Löschen bräche die Regel «Vertrag = Tag». |
| F-7 | Test und dev aus der täglichen Sicherung nehmen? | Ja, es sind Kopien. |
| F-8 | Messungen für `docs/perf` künftig auf test statt auf CT 190? | Ja: gleiche Grösse, und niemand schaut dort gerade einen Film. |
| F-9 | Darf Claude auf dev ohne Marcos Wort einspielen und auffrischen (Abschnitt 4.2)? | Ja. Dafür ist dev da; test und produktiv bleiben an Marcos Wort gebunden. |
| F-10 | Sollen Handy- und TV-App den Namen der Instanz und den Testbetrieb zeigen? | Später, mit der nächsten App-Version, die ohnehin ansteht. Die Felder im Vertrag gibt es ab Stufe 1. |
| F-11 | Zusätzlich ein Riegel im Netzwerkweg aller Clients, der im Testbetrieb jeden unbekannten schreibenden Aufruf von selbst sperrt (Abschnitt 8.5)? | Nicht in Stufe 1. Die Stellen, die schreiben, sind wenige und benannt, und zwei Wächter melden eine neue. Als eigener Schritt nachholbar. |

## 17. Verworfen

| Weg | Warum nicht |
|---|---|
| Vorabversion `-rc1` für test, danach `X.Y.Z` neu bauen | Produktiv bekäme ein Image, das test nie gesehen hat. Dazu müssten `ValidRelease`, `Newer`, die Namen der Sicherungen und `ReleaseFileURL` Zusätze lernen. |
| Der Kanal liest die Liste aller Releases über die API und nimmt das höchste | Neuer Abrufweg nur für test (test prüfte nicht mehr den Abruf von produktiv), Grenze von 60 Abfragen je Stunde ohne Anmeldung, eine Listenantwort mit Notizen kann die heutige Grössengrenze sprengen. |
| Kandidat als gewöhnliches Release mit `--latest=false` | Bestimmt GitHub «latest» neu, könnte es den Kandidaten nehmen. Die Markierung als Vorabversion kostet nichts und schliesst das aus. |
| Freie Manifest-Adresse ausserhalb des Entwicklungsmodus | Das Manifest bestimmt das Image. Eine frei wählbare Adresse wäre ein neuer Angriffsweg auf jede Installation. |
| Test im Entwicklungsmodus betreiben | Schaltet rund zehn weitere Dinge um; test wäre nicht mehr, was produktiv ist. |
| Skript schaltet nach jeder Kopie einzeln ab | Von Marco verworfen (E-3): Die Liste läge ausserhalb des Codes und veraltet still. |
| Test mit eigenen Testdaten | Von Marco verworfen (E-2): Die Migration liefe nie am echten Bestand. |
| Schalter des Testbetriebs in der `.env` des Ziels | Hinge an der kopierten Compose-Datei, die ihn durchreichen muss. Feste Werte in einer Datei, die dem Ziel gehört, hängen an nichts Kopiertem. |
| Seerr, Radarr und Sonarr pauschal nach HTTP-Methode sperren | Träfe lesende POST (Verbindungstest von Seerr). |
| Firewall gegen Seerr, Radarr und Sonarr | Test soll dort lesen; eine Firewall trennt Lesen und Schreiben nicht. |
| Kopie über Sicherung und Wiederherstellung der Verwaltung | Braucht ein Passwort für die Geheimnisse und eine Anmeldung am Ziel; mehr bewegliche Teile als ein Schnappschuss. Bleibt der Weg für fremde Installationen. |
| Container klonen (`pct clone`) | Kopiert auch, was dem Ziel gehören muss; eingehängte Host-Ordner lassen sich so nicht klonen. |
| Produktiv aktualisiert sich nach der Freigabe selbst | Nicht bestellt. Marco klickt weiter selbst (Entscheid vom 9. November 2026). |

## 18. Prüfung beim Bau

Nicht belegt und vom Bau zuerst zu klären:

1. Ob `gh release upload --clobber` die Datei des Zeigers ohne Lücke ersetzt oder kurz 404 liefert
   (Risiko R-3).
2. Ob sich die Datenplatte von CT 190 nur lesbar in zwei weitere Container einhängen lässt und
   der NAS-Ordner darunter sichtbar bleibt (Reihenfolge der Einhängepunkte, Abschnitt 12.1).
3. Ob beide neuen Container dieselbe Zuordnung der Benutzernummern haben wie CT 190, sodass
   kopierte Dateien dem richtigen Benutzer gehören.
4. Was in `compose.ct.yaml` von CT 190 steht und ob es für test und dev unverändert gilt.
5. Welche Quellen ausser der YouTube-Suche ein Tageskontingent haben (Abschnitt 8.1).
6. Ob Telegram einem zweiten Abrufer mit einem Konflikt antwortet und ob schon `getMe` der Kopie
   produktiv stört. Wenn ja, sperrt der Testbetrieb auch die Verbindungsprüfung.
7. Wann Jobs, die im Augenblick des Schnappschusses als «laufend» galten, in der Kopie wieder
   anlaufen (sofort oder nach Ablauf einer Frist).
8. Wie lange das erste Auffrischen dauert (rund 20 GB) und wie lange die folgenden.
9. Ob `docker buildx imagetools create` den Herkunftsnachweis des Digests unberührt lässt, und
   welches Leserecht `gh attestation verify` im Lauf braucht.
10. Die Last auf dem NAS mit drei Instanzen (Risiko R-5).
11. Ob `Health` der richtige Ort für `sandbox` und `instanceName` ist oder eine Operation mit
    Sitzung besser passt; beides ist additiv.
12. Ob weitere Einstellungen aus `deploy/.env.example` in `deploy/compose.yaml` nicht
    durchgereicht werden (gesehen bei `SLOUCHCAT_WORKERS`); nicht Teil dieses Papiers, aber
    derselbe Fehler.
13. Was GitHub als «latest» ausliefert, wenn das «latest»-Release fehlt. Aus dem Repository nicht
    belegbar; darum sind Kandidaten Vorabversionen.

## 19. Kritiker-Review (Fassung 1)

Unabhängiger Durchgang gegen den Code am Anker, 28. November 2026. Urteil **REVISE**: 0 Blocker,
9 Major, 16 Minor. Der Kern ist bestätigt: Der feste Zeiger läuft ohne Änderung durch den
heutigen Abruf von Server und Begleiter. Alle Befunde sind in Fassung 2 eingearbeitet.

| Nr. | Befund | Eingearbeitet in |
|---|---|---|
| M-1 | Der Testbetrieb wurde erst nach dem Start der Kopie geprüft; die Compose-Datei reicht nur gelistete Variablen durch. | 8 (Marke), 10.1, 10.2 |
| M-2 | «`on`, sonst aus» machte aus einem Tippfehler «aus». | 8 |
| M-3 | «`COMPOSE_FILE` wie produktiv» hätte den Podcast-Ordner schreibbar eingehängt. | 10.1 |
| M-4 | Firewall gegen Seerr widersprach «Seerr lesen bleibt». | 12.4, F-3 |
| M-5 | Sperrliste unvollständig (Testmail, weitere Seerr-Jobs, Schreibprobe), Ausnahmen und Verhalten nicht festgelegt. | 8.1 |
| M-6 | Der Freigabe-Lauf hätte einen verdrängten Kandidaten ohne Eilkorrektur durchgelassen. | 5.3, 6.4 |
| M-7 | Zwei Release-Läufe konnten den Zeiger in falscher Reihenfolge schreiben. | 5.3, 6.2 |
| M-8 | Kandidaten als gewöhnliche Releases: GitHub könnte einen zu «latest» machen. | 6.2 |
| M-9 | Dev lässt ungeprüften Code auf echte Daten, ohne den Listentest. | 8.4, 11 |
| m-1 bis m-16 | Unter anderem: `SLOUCHCAT_WORKERS` wird nicht durchgereicht; dev zeigte nicht «Entwicklungsstand»; Anhänge `Caddyfile` und `.env.example`; Schnappschuss während einer Migration; `rsync --delete`; `cache/downloads`; Vorrang von Kanal und Adresse; derselbe Serverschlüssel; wer taggt; Einfrieren mit `REF`; Eingabeprüfung im Freigabe-Lauf; Erkennen fremder Migrationen; Listentest als Liste von Stellen; Storybook; Zeilennummern; Zugriffsweg auf den Host. | 3.2, 3.4, 4.2, 5.1, 6.4, 6.5, 6.6, 7, 8.3, 8.4, 9, 10, 11, 12.1, 13, 14, 15 |

Nur als Einschätzung, nicht am Code belegbar: M-2, M-4, M-6, M-8 und die Gestaltungsseite von M-9.

**Nachprüfung der Einarbeitung** (derselbe Kritiker, am selben Tag): **ACCEPT**. M-1, M-2, M-4,
M-5, M-6, M-8 und M-9 sind geschlossen; am Code bestätigt ist, dass der Updater eine Datei wie
`compose.env.yaml` stehen lässt und sie beim Neustart an ihrer Stelle in der Liste behält. Vier
kleine Punkte sind danach eingearbeitet:

| Punkt | Eingearbeitet in |
|---|---|
| Ein Wunsch wird heute erst gespeichert und dann bis zu drei Sekunden lang an Seerr versucht; der Satz dazu war falsch. | 8.1 |
| Eine `concurrency`-Gruppe hält nur einen wartenden Lauf; Freigabe- und Release-Lauf brauchen getrennte Gruppen. | 6.2, 6.4 |
| Ein alter Release-Lauf, von Hand neu gestartet, hätte den Zeiger zurückstellen können; der Freigabe-Lauf prüfte den Vermerk «nicht freigegeben» nicht. | 6.2, 6.4 |
| Der Rückweg von Hand umgeht alle Prüfungen. | 14, Stufe 3 |

Dazu aus M-3: Das Auffrischen prüft, dass der Podcast-Ordner unter der Medien-Einhängung liegt
(Abschnitt 10.2).
