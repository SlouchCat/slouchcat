# Konzept 28 · Review 1 Sicherheit

Stand: 28. November 2026 · Reviewer: oh-my-claudecode:security-reviewer

Unabhängiges Sicherheits-Review des Architekturentwurfs Live-TV (Fassung 1) mit Bedrohungsmodell. Der
Bericht ist zugleich die Grundlage für `12-sicherheit.md`. Der Reviewer hat die Dokumente nicht geschrieben.

## 0. Kurzfassung

- **Risikostufe des Entwurfs, so wie er geschrieben ist: HOCH.** Nach den Korrekturen dieses Berichts:
  MITTEL (Restrisiko vor allem FFmpeg als C-Code auf feindlichen Daten und das Heimnetz bei UDP-Quellen).
- **24 Befunde:** 0 kritisch, 6 hoch, 12 mittel, 6 niedrig.
- Der Entwurf ist in der Haltung richtig: Er übernimmt die Regel «FFmpeg sieht keine Adresse» (ADR 0025),
  prüft Adressen im Dialer nach der Namensauflösung, will Zugangsdaten verschlüsseln und kennt die Lücken
  der Ist-Analyse (Schwärzung, `netguard.Clients.For`). Er bleibt aber an sechs Stellen hinter seinem eigenen
  Anspruch zurück. Die Freigabe lokaler Ziele ist nicht eng gefasst. Weg W ist nicht dicht. FFmpeg läuft mit
  Zugriff auf `secret.key`. Go-Parser können den Server zum Absturz bringen. Adressen mit eingebetteten
  Zugangsdaten bleiben im Klartext. Geheime Kopfzeilen können an Hosts gehen, die die Liste bestimmt.
- Fünf **stillschweigende Lockerungen** (Abschnitt 5). Die wichtigsten: Die lokale Freigabe gilt laut
  `05` für alle Anfragen einer Quelle, laut `02` A-3 nur für die eingetragene Quelle. Der Netzmodus «host»
  ist nicht als Ausnahme benannt. Kopfzeilen werden nur maskiert, wenn ihr Name «geheim klingt».

## 1. Umfang, Methode, Einordnung

**Gelesen:** `02-grundsatzentscheide.md` (A-1 bis A-8), `03-fachkonzept.md` (Abschnitte 2, LTV-003, LTV-Q-10,
5), `04-architektur.md` (ganz), `05-streaming-und-fast-zapping.md` (Abschnitte 2, 3), `09-epg.md` (Abschnitte
2, 3, 9, 12, 14.3), `14-oberflaeche.md` und `15-administration.md` (sicherheitsrelevante Stellen),
`anhang/auftrag-2026-11-28.md` Abschnitt 10 ganz sowie LTV-009, LTV-012, LTV-015, LTV-017, ADR 0025, ADR 0040,
`CLAUDE.md`; Rohberichte `LEITPLANKEN.md`, `V1`, `R3`, `R4`, `M1`. Am Code geprüft: `server/internal/netguard`,
`fetch`, `secrets`, `logging`, `radio` (Token, Leser, Logos, Hörsitzung), `transcode` (`radio.go`,
`session.go` Prozessstart), `paths`, `auth/profiles.go`, `api` (`stream_token.go`, `radio_binary.go`,
`session.go` CSRF, `limits.go`), `notify` (Escaping), `deploy/compose.yaml`, `deploy/Dockerfile`; die
Go-Standardbibliothek `encoding/xml` im Quelltext der Toolchain 1.27.3.

**Nicht getan** (Auftrag: nur lesen): keine Builds, keine Tests, kein `govulncheck`, keine Läufe gegen fremde
Systeme. Das Verhalten von FFmpeg ist darum nur aus Dokumentation und Kenntnis des Quelltexts beurteilt.

**Einordnung jeder Aussage:**

- **[belegt Code]** am Quelltext geprüft, mit Datei:Zeile (Stand Arbeitsbaum `claude/konzept-live-tv`).
- **[belegt Doku]** an einem Dokument dieses Konzepts geprüft, mit Datei:Zeile.
- **[Annahme]** nicht nachgeprüft (meist Verhalten von FFmpeg 7.1.4, libxml2, gortsplib); im Bau zu belegen.

**Secrets-Scan:** Die Konzeptdateien und `docs/planning/mockups/28-live-tv/` enthalten keine Zugangsdaten
und keine Anbieter-Adressen (Suche nach `password=`, `token=`, `api_key=` und nach Adressen ausserhalb
`example.*`; Fundstellen nur Recherche-Quellen in `24-recherche-und-quellen.md`). [belegt]

**Abhängigkeiten:** nicht geprüft (kein `govulncheck`). Neu kämen bei F-1 (a) `gortsplib`, `gortmplib`,
`mediacommon` und `pion/*` dazu; FFmpeg ist `jellyfin-ffmpeg7` 7.1.4-3 (`deploy/Dockerfile:73-75`). Beides
gehört in den Massnahmenkatalog (MS-24).

## 2. Befunde

Schwere nach Schaden × Ausnutzbarkeit × Reichweite: **kritisch** (ausnutzbar, schwerer Schaden, ohne besondere
Voraussetzung), **hoch** (schwerer Schaden unter einer realistischen Voraussetzung), **mittel** (begrenzter
Schaden oder schwierige Ausnutzung), **niedrig** (gute Praxis, kleiner Schaden).

### RS-01 · Freigabe lokaler Ziele ist nicht eng gefasst · **hoch**

- **Stelle:** `04` T-5 (Zeile 48); `05` Abschnitt 2.3 «Regel je Quelle» (Zeilen 79-98); `02` A-3 (Zeile 123);
  `15` LA-03 Schalter «Ziele im Heimnetz erlauben» (Zeile 292), Text V-3 (Zeile 802).
- **Szenario:** Der Administrator schaltet für eine Quelle «Heimnetz erlauben» ein, etwa für eine
  M3U-Liste seines TVHeadend auf dem NAS oder für eine Anbieterliste, die auch `udp://@239…`-Einträge hat
  («UDP, RTP und Multicast sind immer lokale Quellen und brauchen diese Freigabe», `04:48`). Nach `05:91`
  bekommt `netguard` «eine Prüffunktion mit Regel». Die Regel ist ein Ja/Nein je Quelle. Dann darf **jede**
  Anfrage dieser Quelle jedes lokale Ziel erreichen: jede Sender-Adresse, die die Liste nennt, jede
  Weiterleitung, jede Unterliste, jedes Segment, jeder Schlüssel. Wer die Liste oder das Gerät kontrolliert
  (ein gekaperter Listen-Host, ein gekapertes Gerät im Heimnetz), trägt «Sender» mit internen Adressen ein:
  `http://127.0.0.1:8096/…` (der Server selbst), `http://127.0.0.1:<Port des Weiterleiters>`, `http://172.17.0.1:2375/`
  (Docker-API des Hosts, falls offen), `http://192.168.1.1/…` (Router), `http://169.254.169.254/` (Metadaten).
  Die Verfügbarkeitsprüfung ruhender Sender (Haltegrund «Prüfung», `04:299`), Vorladen, «automatisch» und
  geplante Aufnahmen lösen die Anfragen **ohne Zutun eines Menschen** aus. Das ist blinde SSRF (GET mit
  Kopfzeilen der Quelle) ins ganze Heimnetz. Die Zustände «HTML-Fehlerseite», «Verbindung abgelehnt»,
  «Zeitüberschreitung» (`05:96`) machen sie für den Administrator zum Portscanner. Der Angreifer selbst sieht
  sie nicht, aber blinde GETs genügen für viele Geräte im Heimnetz (Neustart-, Schalt- und Konfigurationsadressen).
- **Beleg:** [belegt Code] Das einzige gebaute Muster einer Heimnetz-Ausnahme gibt einen Client ganz ohne
  `Control`-Funktion zurück (`server/internal/netguard/netguard.go:93-111`, `:118-128`). Die Entscheidung
  fällt mit einer eigenen Namensauflösung (`:58-68`), getrennt von der späteren Verbindung. Ein Name mit einer
  öffentlichen **und** einer privaten Adresse gilt schon als lokal (`slices.ContainsFunc(addrs, Local)`, `:67`).
  [belegt Doku] `02` A-3 sagt das Gegenteil von `05`: «Die Freigabe gilt nur für die eingetragene Quelle, nicht
  für Adressen, auf die sie weiterleitet oder verweist (Playlist-, Segment-, Schlüssel-…)». Wörtlich genommen
  bricht A-3 dagegen jede lokale HLS-Quelle (Segmente auf demselben Host) und jede lokale M3U-Liste (die
  Sender-Adressen sind «verwiesen»). Die Regel ist also widersprüchlich und in keiner Fassung baubar.
- **Korrektur (Pflicht vor Stufe 1, weil T-5 und der Dialer in Stufe 1 entstehen):**
  1. Die Freigabe ist eine **Liste fester Ziele**, keine Ja/Nein-Einstellung: je Eintrag eine IP-Adresse oder ein
     kleines Netz (höchstens /24 bei IPv4, /64 bei IPv6), dazu ein Port oder ein kleiner Portbereich. Der
     Administrator bestätigt sie beim Anlegen. Ein Hostname wird **beim Anlegen** aufgelöst, und die Adressen
     werden mit angezeigt und gespeichert. Ändert sich die Auflösung, gilt das als neue Freigabe mit Rückfrage.
  2. Der Dialer jeder Live-Quelle prüft **jede** Verbindung (Liste, Sender, Weiterleitung, Unterliste, Segment,
     Schlüssel): Eine öffentliche Adresse geht; eine lokale nur, wenn sie in den festen Zielen **dieser** Quelle
     liegt. Logos und EPG-Bilder sind immer nur öffentlich (`09` Abschnitt 9 schreibt das schon so).
  3. **Nie freigebbar**, auch nicht mit Schalter: Loopback (`127.0.0.0/8`, `::1`), `0.0.0.0/8`, unspezifiziert,
     Link-Local (`169.254.0.0/16`, `fe80::/10`, darin die Metadaten-Adressen), die eigenen Adressen des
     Containers und der Port des Weiterleiters (Weg W). Wer TVHeadend auf dem Docker-Host betreibt, gibt die
     Adresse des Hosts im Heimnetz frei, nicht Loopback.
  4. Sender aus einer Liste erben die Freigabe nur, wenn ihr Ziel in den festen Zielen liegt. Sonst importiert
     der Import den Eintrag, markiert ihn «Ziel im Heimnetz nicht freigegeben» und verbindet nie. Er probiert
     beim Import auch keine Adresse aus.
  5. `netguard.Clients.For` wird für Live-TV **nicht** verwendet. Kein Live-Client ohne `Control`.
  6. Jede Änderung einer Freigabe schreibt einen Eintrag ins Protokoll der Verwaltung (wer, wann, welche Ziele).
- **Sicherer Code (Skizze):**

```go
// BAD: one switch per source, the local client has no Control at all (netguard.go:93-111, :118-128).
client := clients.For(ctx, src.URL, origin)

// GOOD: every dial of every request of the source is checked against its pinned targets.
type Pinned struct{ Nets []netip.Prefix; Ports []uint16 }

func (p Pinned) Control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: unparsable address", netguard.ErrRefused)
	}
	ip := ap.Addr().Unmap()
	if netguard.NeverLocal(ip) { // loopback, 0/8, unspecified, link-local, own addresses
		return fmt.Errorf("%w: %s", netguard.ErrRefused, ip)
	}
	if !netguard.Local(ip) {
		return nil
	}
	if slices.ContainsFunc(p.Nets, func(n netip.Prefix) bool { return n.Contains(ip) }) &&
		slices.Contains(p.Ports, ap.Port()) {
		return nil
	}
	return fmt.Errorf("%w: %s not pinned for this source", netguard.ErrRefused, ip)
}
```

### RS-02 · Weg W ist nicht dicht · **hoch**

- **Stelle:** `04` T-4 (Zeile 47); `05` Abschnitt 2.6 (Zeilen 161-195); `04` Abschnitt 12.1 Risiko «FFmpeg
  umgeht den Weiterleiter» (Zeile 699).
- **Szenario 1 (Tunnel ohne Einsicht):** Für absolute `https`-Adressen im Manifest baut FFmpeg nach `05:171-172`
  einen `CONNECT`-Tunnel durch den Weiterleiter. Der Weiterleiter sieht dann nur `Host:Port`. Inhalt,
  Weiterleitungen, Kopfzeilen und Grössen im Tunnel sieht er nicht. Die Zusage «Der Weiterleiter liest jede
  durchgereichte Liste und jedes Manifest mit (DRM-Regel 2.5, Grössengrenzen)» (`05:178`) gilt für alles, was
  über HTTPS kommt, nicht. Das ist bei IPTV der Regelfall. Die DRM-Erkennung (LTV-003.X-08) und die
  Grössengrenzen sind damit für Unterlisten, Segmente und Init-Teile in Weg W wirkungslos. Ausserdem prüft
  FFmpeg TLS-Zertifikate ab Werk nicht (Option `tls_verify` ist 0) [Annahme].
- **Szenario 2 (Ausfall offen statt zu):** Die Erlaubnisliste `http,https,tcp,tls,httpproxy,crypto`
  (`05:174`) enthält `tcp` und `tls`. Der Ersatzname `*.invalid` schützt nur relative Adressen. Eine absolute
  Adresse `http://192.168.1.1/x.m4s` in einem DASH-Manifest geht nur dann über den Weiterleiter, wenn FFmpeg
  auf diesem Codepfad die Proxy-Angabe beachtet. Tut es das auf einem Pfad nicht (Unteranfragen des DASH-Lesers,
  Wiederverbindung, ein Pfad, den die Recherche nicht kennt; `05:186-191` nennt das selbst unbelegt), verbindet
  FFmpeg direkt, ohne jede Prüfung in Go. Das Netz des Containers erlaubt es. Der Fehler fällt also nach «offen»
  aus, nicht nach «zu».
- **Szenario 3 (Weiterleitung durchgereicht):** Reicht der Weiterleiter eine Antwort 3xx der Quelle an FFmpeg
  durch, folgt FFmpeg der neuen Adresse selbst. Beachtet es die Proxy-Angabe dort nicht, gilt Szenario 2.
- **Szenario 4 (Einmalkennwort):** FFmpeg kann ein Kennwort beim Proxy nur über die Proxy-Adresse
  (`http://k:pw@127.0.0.1:port`) mitschicken, also in Argumentliste oder Umgebung [Annahme]. Im Bridge-Netz
  erreicht nur der Container `127.0.0.1`. Im Netzmodus «host» (`04:656`) ist `127.0.0.1` das Loopback des
  Docker-Hosts: Jeder Benutzer auf dem Host liest das Kennwort mit `ps` (Argumentliste) und benutzt den
  Weiterleiter. Der setzt dann die Kopfzeilen und Zugangsdaten der Quelle ein (siehe RS-13).
- **Beleg:** [belegt Doku] `05:161-195`. [belegt] Protokolle im Image laut `M1` (`file`, `concat`,
  `subfile`, `tcp`, `udp`, `unix`, `gopher`, `ftp`, `httpproxy` …). [Annahme] Dass HLS- und DASH-Leser von
  FFmpeg 7.1 in Unteranfragen nur `http*`, `file`, `crypto`, `data` annehmen und `http_proxy` als Option an
  Unteranfragen weiterreichen.
- **Korrektur (Pflicht vor Stufe 3, Bedingung für jeden Bau von Weg W):**
  1. **Kein `CONNECT`.** Der Weiterleiter lehnt `CONNECT` mit 403 ab. Er **schreibt jede absolute Adresse** in
     durchgereichten Listen und Manifesten um (HLS-Zeilen und `URI=`-Attribute; DASH `BaseURL`, `Location`,
     `SegmentTemplate@media`/`@initialization`, `SegmentURL`). Jeder fremde Host bekommt einen eigenen
     Ersatznamen. TLS endet immer in Go, und FFmpeg spricht nur noch Klartext-HTTP mit `127.0.0.1`.
  2. Erlaubnisliste nur `http,tcp,crypto`; kein `https`, `tls`, `httpproxy`, `data`, `file`. Eingang ausdrücklich
     `-f hls` oder `-f dash`.
  3. Der Weiterleiter folgt Weiterleitungen selbst (geprüft wie in RS-01) und gibt nie 3xx an FFmpeg weiter.
  4. Go liest jedes MPD selbst mit `encoding/xml` (DRM-Regel) und verwirft Dokumente mit `<!DOCTYPE` oder
     `<!ENTITY`, bevor libxml2 in FFmpeg sie sieht.
  5. **Egress-Sperre für den FFmpeg-Empfänger:** Mit Landlock (Linux ab 6.7, ABI 4) darf der Prozess nur zum
     Port des Weiterleiters TCP-Verbindungen öffnen und `/config` nicht lesen (Hülle als Unterbefehl des
     Servers, der die Regeln setzt und dann FFmpeg startet). Fehlt Landlock (alter Kernel, Seccomp-Profil sperrt
     die Aufrufe), bleibt Weg W aus. Ohne diese Sperre ist «FFmpeg öffnet keine Verbindung, die nicht Go gewählt
     hat» nicht beweisbar, nur testbar.
  6. Weg W ist im Netzmodus «host» gesperrt.
  7. M-10 als feindliche Testmenge: absolute `http://`-, `https://`-, `tcp://`-, `file:`-, `data:`-Adressen und
     IP-Literale, Weiterleitungen auf LAN-Adressen, Unterlisten in drei Ebenen, `EXT-X-KEY` und `EXT-X-MAP` auf
     fremden Hosts. Beleg mit einer Mitschrift aller `connect()`-Aufrufe des FFmpeg-Prozesses (zum Beispiel
     `strace -f -e trace=connect`): einziges Ziel ist der Weiterleiter.

### RS-03 · FFmpeg verarbeitet feindliche Netzdaten mit vollem Zugriff auf die Geheimnisse · **hoch**

- **Stelle:** `04` T-13, T-20, Abschnitt 8.4; `05` Abschnitt 2.6; `04` Abschnitt 10.6 (Compose unverändert).
- **Szenario:** Live-TV gibt FFmpeg erstmals Video aus beliebigen Fremdquellen: MPEG-2, H.264, HEVC,
  DVB-Untertitel, Teletext (`libzvbi`), im Weg W dazu die Leser für HLS und DASH (libxml2). Dazu kommen Logos und
  EPG-Bilder von beliebigen Hosts (Muster `radio/logo.go`, Dekodieren über FFmpeg, siehe RS-21). Eine
  Speicherlücke in einem Demuxer oder Decoder (bei FFmpeg regelmässig) gibt dem Angreifer Code im Prozess
  FFmpeg. Der läuft mit derselben UID wie der Server. Er liest `/config/secret.key`, `slouchcat.db`, die
  Sicherungen und `/proc/1/environ` (die API-Schlüssel aus `compose.yaml:47-71`). Damit entschlüsselt er alle
  Zugangsdaten der Anbieter und übernimmt Sitzungen. Er erreicht das Heimnetz ohne `netguard` und, mit dem
  Netzmodus «host», auch die Dienste des Hosts. Angreifer ist jeder Anbieter, jeder Listen-Host, jeder Host
  eines Segments und bei UDP jedes Gerät im Heimnetz (RS-12).
- **Beleg:** [belegt Code] `server/internal/transcode/session.go:662-683`: Prozessstart ohne `cmd.Env`
  (FFmpeg erbt die ganze Umgebung), ohne `SysProcAttr`, ohne Ressourcengrenzen (rlimit); nur `nice` und
  `oom_score_adj` (`:704-712`). `deploy/compose.yaml:29` setzt nur den Benutzer; kein `no-new-privileges`, kein
  `cap_drop`, kein `pids_limit`, kein `read_only`. `deploy/Dockerfile:136` `USER 1000:1000`, der Server ist
  PID 1 (`:143`). Das Radio verarbeitet schon heute fremde Tonströme über Pipes; die Angriffsfläche dort (Ton)
  ist aber viel kleiner als bei Video, Untertiteln und Manifesten.
- **Korrektur:**
  - **Pflicht vor Stufe 1:** Jeder Live-FFmpeg und `ffprobe` bekommt eine **ausdrückliche, kleine Umgebung**
    (`PATH`, `LANG`, die Variablen für Hardware wie `LIBVA_DRIVER_NAME`; keine `SLOUCHCAT_*`). Eingang immer mit
    `-f mpegts` und `-protocol_whitelist pipe` (auch `ffprobe`). Ressourcengrenzen per `prlimit` gleich nach dem
    Start: `RLIMIT_AS` (Richtwert 2 GiB für Umwandlung, nach M-8 festzulegen), `RLIMIT_NOFILE` 256,
    `RLIMIT_CORE` 0. Compose: `security_opt: ["no-new-privileges:true"]`, `cap_drop: [ALL]`,
    `pids_limit` (Richtwert 512). Diese drei Zeilen härten nur und lockern nichts, sie dürfen auch in die
    Standard-Compose-Datei.
  - **Pflicht vor Stufe 3 (Weg W) und empfohlen ab Stufe 1:** Landlock-Hülle wie in RS-02 Punkt 5 (kein
    Lesezugriff auf `/config`, Schreiben nur in den Ordner der Sitzung, TCP-Verbindungen nur zum Weiterleiter
    oder gar keine). Wo der Kernel Landlock nicht hat, zeigt die Verwaltung «FFmpeg läuft ohne Sandbox».
  - **Betrieb:** FFmpeg-Fassung wird bei jedem Release gegen bekannte Lücken geprüft (MS-24).
- **Restrisiko** nach der Korrektur: Code in FFmpeg kann weiter rechnen und die Pipe vergiften, aber keine
  Geheimnisse lesen und das Netz nicht erreichen. Ohne Landlock bleibt das Risiko hoch.

### RS-04 · Ein kaputtes Paket kann den ganzen Server beenden · **hoch**

- **Stelle:** `04` Abschnitt 3.2 (`live/mpegts`, `live/hub`, `live/ingest`, `live/catalog`, `live/guide`);
  `05` Abschnitt 2.7, 3.
- **Szenario:** Empfang, Verteilung, Sitzungspuffer und Aufnahme laufen als Goroutinen im Serverprozess
  (T-1, T-20). Sie verarbeiten Fremddaten: TS-Pakete (Adaptionsfeld-Länge, `section_length`, Deskriptor-Schleifen,
  PES-Köpfe, 33-Bit-Zeiten), RTP-Köpfe mit Erweiterungen, HLS-Listen, MPD, M3U, Xtream-JSON. Ein Indexfehler
  in Go löst `panic` aus. In einer Goroutine ausserhalb des HTTP-Servers beendet `panic` den **ganzen** Prozess:
  alle Wiedergaben (auch Filme), alle laufenden Aufnahmen, alle Jobs. Ein Anbieter, ein Listen-Host oder ein
  Gerät, das UDP einschleust, kann den Server mit einem Paket wiederholt neu starten lassen. Der Container
  startet neu (`restart: unless-stopped`), das Paket kommt wieder.
- **Beleg:** [belegt Code] `recover()` steht heute nur in `api/problem.go:83` (HTTP), `jobs/worker.go:189`
  (Jobs) und `discover/discover.go:290`; die Relay-Goroutinen des Radios haben keines. Fuzz-Tests gibt es nur
  in `mediaindex` (`mp4_test.go`, `matroska_test.go`). [belegt Doku] `04:121-122` verlangt «mit feindlichen
  Eingaben prüfbar» (Vorbild `radio/hostile_test.go`), aber keinen Schutz gegen `panic` und kein Fuzzing.
- **Korrektur (Pflicht vor Stufe 1):**
  1. Jede Goroutine von `live/ingest`, `live/hub`, `live/spool` und `live/dvr` hat an ihrer Wurzel `recover`. Er
     macht aus `panic` einen Fehler der Quelle («Quelle liefert ungültige Daten», Quarantäne) und meldet ihn mit
     Stack ohne Inhalt.
  2. Go-Fuzz-Ziele (`func FuzzX(f *testing.F)`) für `mpegts` (Paket, PSI, PES), RTP, HLS-Liste, MPD (DRM-Leser),
     M3U, Xtream-JSON und XMLTV-Element; Ausgangsmenge aus der feindlichen Testmenge; die CI lässt sie eine
     feste Zeit laufen.
  3. Keine Speicheranforderung aus einem Längenfeld der Quelle ohne Obergrenze.

```go
// GOOD: a panic in a receiver ends this source, not the server.
func (r *receiver) run(ctx context.Context) (err error) {
	defer func() {
		if p := recover(); p != nil {
			r.log.ErrorContext(ctx, "live receiver panicked", "channel_id", r.channelID,
				"site", netguard.Site(r.siteURL), "stack", string(debug.Stack()))
			err = errInvalidSourceData // quarantines the source
		}
	}()
	return r.loop(ctx)
}
```

### RS-05 · Zugangsdaten in Adressen bleiben im Klartext; Maskierung nach Muster versagt · **hoch**

- **Stelle:** `05` Abschnitt 2.4 «Speicherung» (Zeilen 103-105); `04` Abschnitt 10.5; `15` Grundsatz 5
  (Zeilen 40-41) und Beispiel `…playlist.m3u8?token=••••` (Zeile 54).
- **Szenario:** Verschlüsselt werden laut Entwurf die «Adresse mit Zugangsdaten», Benutzer und Passwort einer
  Xtream-Quelle und die Kopfzeilen. Offen bleibt, woran der Server eine Adresse mit Zugangsdaten erkennt.
  Offen bleibt auch, was mit den **Stream-Adressen der Sender** geschieht, die ein Import in die Datenbank
  schreibt. Bei Xtream-M3U-Listen (`get.php`) hat jede Sender-Adresse die Form `/live/<Benutzer>/<Passwort>/<id>.ts`;
  andere Listen tragen `?token=`, `?u=…&p=…` oder Signaturen. Tausende solcher Zeilen im Klartext landen in jeder
  Sicherung **ohne** Geheimnisse (die sonst gerade keine Geheimnisse enthält, `admin/backup.go:33-40` nach `01:315`),
  in den fünf `VACUUM INTO`-Kopien vor jeder Migration (`CLAUDE.md` Regel 5) und in jeder Antwort der Verwaltung,
  die eine Adresse zeigt. Eine Maskierung nach bekannten Namen (`token=••••`) übersieht unbekannte Namen und
  Pfade. Dazu kommt Xtream: Die Antwort von `player_api.php` nennt in `server_info` einen Host und Port [Annahme,
  R3 Abschnitt 3]. Baut der Server die Stream-Adressen daraus, schickt er Benutzer und Passwort im Pfad an einen
  Host, den die Antwort bestimmt.
- **Beleg:** [belegt Doku] `05:103-105`, `15:40-41`, `15:54`. [belegt Doku] Vorbild mit genau dieser Lücke:
  Podcast-Adressen im Klartext in `items.feed_url` (`01-bestandsanalyse.md:304-305`).
- **Korrektur (Pflicht vor Stufe 1):**
  1. **Jede** Adresse einer Quelle und jede Stream-Adresse eines Senders gilt als geheim und wird ganz
     verschlüsselt gespeichert (`secrets.Keyring`). Daneben steht nur `site` (Schema, Host, Port) im Klartext,
     für Anzeige, Suche und Protokoll.
  2. Die Verwaltung bekommt aus der API nur `site` und den Hinweis «Pfad und Abfrage verborgen». Felder für
     Adresse, Passwort und Kopfzeilen sind nur schreibbar. «Ändern» füllt nichts vor; leer heisst unverändert.
  3. Xtream: Stream- und EPG-Adressen (`/live/…`, `xmltv.php`) baut der Server im Speicher nur aus dem Host, den
     der Administrator eingetragen hat. `server_info.url` und `server_info.port` werden nicht verwendet.
     Weicht der Host ab, zeigt die Verwaltung das als Hinweis.
  4. Der Import speichert Sender-Adressen einer Xtream-M3U nicht, sondern nur `stream_id` und baut die Adresse
     beim Verbinden.
  5. Testfall «Sicherung ohne Geheimnisse»: Die entpackte Datenbank enthält keinen der Testwerte von Benutzer,
     Passwort, Token und Kopfzeilen (Suche nach Bytes).

### RS-06 · Geheime Kopfzeilen und Zugangsdaten können an Hosts gehen, die die Liste bestimmt · **hoch**

- **Stelle:** `05` Abschnitt 2.4 «Weitergabe nur an den Ursprung» (Zeilen 109-113); `15` LA-03 Kopfzeilen
  (Zeile 290); M3U-Zusatzzeilen laut R3 (Zeile 22).
- **Szenario A:** Für eine M3U- oder Xtream-Quelle ist «Ursprung der Quelle» nicht festgelegt. Ist es der Host
  der Liste, dann bekommt kein Sender auf einem CDN-Host die Kopfzeilen (dann funktioniert nichts). Gilt die
  Kopfzeile dagegen für jeden Sender der Quelle, schickt der Server `Authorization`, `Cookie` und frei
  eingetragene Kopfzeilen an jeden Host, den die Liste nennt. Ein gekaperter Listen-Host trägt einen Sender
  `https://sammler.example/…` ein und erntet die Zugangsdaten. Vorladen, «automatisch» und die Prüfung lösen
  den Abruf selbst aus.
- **Szenario B:** M3U-Listen tragen eigene Kopfzeilen: `#EXTVLCOPT:http-user-agent=…`, `http-referrer=…`,
  `#EXTHTTP:{"Cookie":…}`, Anhang `|User-Agent=…&Authorization=…` (R3 Abschnitt 1, [Annahme]). Der Entwurf
  sagt nicht, ob der Import sie übernimmt. Übernimmt er sie ungeprüft, setzt der Listen-Host Kopfzeilen wie
  `X-Forwarded-For`, `Authorization` oder `Cookie` für Anfragen ins freigegebene Heimnetz (RS-01).
- **Szenario C:** Eine `https`-Quelle leitet auf `http` desselben Hosts weiter. Go behält Kopfzeilen beim
  gleichen Host, also gehen Zugangsdaten im Klartext übers Netz.
- **Beleg:** [belegt Doku] `05:109-113` sagt richtig, dass Go bei Weiterleitungen nur `Authorization` und
  `Cookie` entfernt und frei eingetragene Kopfzeilen weiterschickt. [Annahme] Go entfernt auch
  `Authorization` nicht, wenn das neue Ziel eine Subdomain des alten ist.
- **Korrektur (Pflicht vor Stufe 1):**
  1. Geheimnisse einer Quelle (Kopfzeilen, Benutzer, Passwort) hängen an einer **festen Menge von Ursprüngen**
     (Schema, Host, Port). Ab Werk ist das nur der Host der eingetragenen Adresse. Der Administrator kann
     weitere Hosts ausdrücklich hinzufügen (die Verwaltung schlägt die Hosts der importierten Sender vor). Jede
     Anfrage an einen anderen Ursprung bekommt nur User-Agent und `Accept`.
  2. Eigene `CheckRedirect`-Funktion: Bei jedem Wechsel des Ursprungs fällt jede Kopfzeile ausser
     User-Agent, `Accept` und `Range` weg; bei `https`→`http` bricht der Abruf einer Quelle mit Geheimnissen ab.
  3. Aus M3U übernimmt der Import nur `User-Agent` und `Referer`, jeweils auf 256 Zeichen gekürzt. `Authorization`,
     `Cookie`, `Host`, `X-Forwarded-*`, `Proxy-*` und alle anderen werden verworfen und gezählt. Werte mit
     Steuerzeichen werden verworfen.
  4. Adressen mit `user:pass@` werden beim Anlegen in Benutzer und Passwort zerlegt und ohne Benutzerteil
     gespeichert. Go setzt sonst daraus selbst einen `Authorization`-Kopf.

```go
// GOOD: secrets only for the pinned origins of the source; nothing else crosses an origin change.
client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
	if len(via) >= maxHops || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
		return errTooManyHops
	}
	if via[0].URL.Scheme == "https" && req.URL.Scheme == "http" && src.HasSecrets() {
		return errDowngrade
	}
	if !src.Origins.Contains(req.URL) {
		ua := req.Header.Get("User-Agent")
		req.Header = http.Header{"User-Agent": {ua}, "Accept": {"*/*"}}
	}
	return nil
}
```

### RS-07 · Schwärzung: die vorgesehene Ergänzung reicht nicht · **mittel**

- **Stelle:** `04` Abschnitt 10.4 (Zeilen 614-630); `09` Abschnitt 12 «Zugangsdaten in Adressen».
- **Szenario:** Der Entwurf nennt die richtigen Lücken (`username`, `password`, `user`, `pass`, `lt`,
  `user:pass@`, Xtream-Pfade). Weitere bleiben offen: (a) `sensitiveQuery` erkennt nur genau diese Namen. `pwd`,
  `passwd`, `auth_token`, `api_token`, `x-auth-token` gehen durch, weil die Regel `(?:…|token|…)=` direkt nach
  `?`, `&` oder `;` beginnen muss. (b) Attributwerte der Art `KindAny` werden nur für `error` und `*url.URL`
  geschwärzt. Ein `slog.Any("args", args)` (Argumentliste), ein `http.Header`, eine `url.Values` oder eine
  Struktur mit Adresse gehen roh hinaus. (c) Es fehlt der Schlüssel `cookie`. (d) Xtream kennt weitere Pfade:
  `/movie/u/p/…`, `/series/u/p/…`, `/timeshift/u/p/…`, die Kurzform `/u/p/<id>` sowie `get.php`, `player_api.php`
  und `xmltv.php` mit `username=`/`password=`. (e) Was die Schwärzung übersieht, steht auch im Fehlerring der
  Verwaltung (`logging/errors.go`) und in «Text kopieren» der Diagnose (`15:609`). (f) Ein Fallstrick: Der Name `lt`
  gehört nur in `sensitiveQuery`, **nicht** in `sensitiveKeys`. Dort wird nach Endung verglichen, und «lt» passt
  auf `default`, `result`, `alt`.
- **Beleg:** [belegt Code] `server/internal/logging/logging.go:63-70` (Schlüssel), `:75` (Abfrage), `:142-161`
  (Pfade), `:185-198` (`redactAny` nur `error` und `*url.URL`), `:200-207` (Vergleich nach Endung).
- **Korrektur (Pflicht vor Stufe 0, denn LTV-Q-10.19 und .22 liegen dort):** Liste in Abschnitt 7. Dazu
  zwei Regeln: Erstens protokolliert Live-TV von einer Adresse **nur** `netguard.Site(u)`; ein Lint (`forbidigo`
  auf `slog.String("url"…)` und `slog.Any` mit Adressen im Paket `live/**`) setzt das durch. Zweitens schwärzt
  `redactAny` `http.Header`, `url.Values` und `[]string` Element für Element. Dazu gehört ein Schwärzungstest
  mit jedem Muster.

### RS-08 · Token `lt`: Lebensdauer, Bindung, Widerruf · **mittel**

- **Stelle:** `04` T-21 (Zeile 64), Ebene 5 (Zeile 233, «60 s ohne Anfrage und ohne Herzschlag»), Abschnitt 9
  «Entzug» (Zeile 568).
- **Szenario:** Das Token steht in jeder Listen- und Segmentadresse. Damit landet es im Verlauf des Browsers, in
  den Protokollen eines Reverse-Proxys (NPM) und in Bildschirmfotos des Netzwerk-Tabs (V1 L-2). Nach dem Muster
  des Radios verlängert **jede** Anfrage mit dem Token die Sitzung (`radio/listen.go:569`). Wer ein Token
  erbeutet, hält die Sitzung also unbegrenzt am Leben, auch wenn das echte Gerät längst aus ist. Er belegt dabei
  einen Platz der Stream-Grenze und einen Eingangsstrom. Das Radio prüft einmal je Minute, ob die
  Anmeldesitzung noch gilt (`radio/listen.go:578-587`). Der Live-Entwurf verlangt nur die Prüfung «des Rechts»
  alle 30 Sekunden, nicht die der Anmeldesitzung. Das Token ist an kein Gerät gebunden. Jedes Profil, das
  es kennt, kann es benutzen.
- **Beleg:** [belegt Code] `radio/token.go:11-41`, `radio/listen.go:561-595`, `playback/sessions.go:803-855`;
  `api/stream_token.go:54-61` setzt `Referrer-Policy: no-referrer` (das schützt gegen Referer, nicht gegen die
  anderen Lecks).
- **Korrektur (Pflicht vor Stufe 1):**
  1. Am Leben hält eine Live-Sitzung **nur der Herzschlag**, also eine angemeldete JSON-Operation derselben
     Anmeldesitzung (Cookie oder Bearer). Segmentabrufe verlängern nicht. 60 Sekunden ohne Herzschlag heissen
     Ende, auch wenn noch Segmente geholt werden.
  2. Jede Listenanfrage prüft höchstens 30 Sekunden alt das Recht **und** die Gültigkeit der Anmeldesitzung
     (Abmelden, Passwortwechsel, Rolle, Ablauf).
  3. Höchstdauer je Einstellen (Richtwert 12 Stunden), danach neues Token über den Herzschlag.
  4. «Einstellen», «Beenden» und Spurwechsel sind `POST` mit dem CSRF-Kopf (`api/session.go:278-286`).
  5. Ein ungültiges, fremdes, abgelaufenes oder widerrufenes Token ergibt bei Live-Routen dieselbe Antwort wie
     «kein Recht». Siehe RS-19 zur Wahl zwischen 401 und 404.

### RS-09 · Das Hauptprofil kann die Privatsphäre erwachsener Profile über die Altersgrenze aufheben · **mittel**

- **Stelle:** `03` Abschnitt 2.4, letzte Zeile («Profil bekommt eine Altersgrenze … Das Hauptprofil sieht ab
  jetzt die Aufnahmen dieses Profils»), DR-02; `02` A-6 («Profile ohne Altersgrenze bleiben ganz privat»).
- **Szenario:** Das Hauptprofil setzt einem erwachsenen Profil desselben Kontos kurz eine Altersgrenze. Damit
  sieht und spielt es dessen Aufnahmen und kann sie löschen. Danach nimmt es die Grenze wieder weg. Das widerspricht
  A-6 und LTV-009 («Die Privatsphäre des bestehenden Profilmodells muss erhalten bleiben»).
- **Beleg:** [belegt Code] `server/internal/auth/profiles.go:204-206` (`UpdateProfile` verlangt nur das
  Hauptprofil) und `:216-225` (die Altersgrenze `MaxRating` gehört zu den änderbaren Feldern).
- **Korrektur (Frage an Marco, vor Stufe 5):** Varianten: (a) Eine Aufnahme merkt sich beim Anlegen, ob ihr
  Profil eine Altersgrenze hatte. Nur solche Aufnahmen sieht das Hauptprofil (Empfehlung). (b) Beim Setzen einer
  Altersgrenze bekommt das Profil eine Meldung, und Aufnahmen von vorher bleiben privat. (c) Wie bisher
  beschrieben, als benannte Ausnahme mit diesem Wortlaut. Unabhängig davon ist die Zugriffsregel **eine** Abfrage
  in `liveq` mit allen Bedingungen (eigenes Profil, oder: Hauptprofil desselben Kontos und Aufnahme mit
  Altersgrenze-Merkmal und Live-TV beim Hauptprofil wirksam). Der Test ist eine Matrix über Profilarten.

### RS-10 · Umschalten als Hebel gegen Server und Anbieterzugang · **mittel**

- **Stelle:** `04` Abschnitt 5.3 «Freigabe» (30 Sekunden Karenz), Abschnitt 8.2 (Rangfolge bei Mangel),
  Abschnitt 6.3 (8 Eingangsströme ab Werk), Abschnitt 7.5 «Spulen mit Umwandlung».
- **Szenario:** Jedes Einstellen kann einen neuen Eingangsstrom öffnen. Der alte Strom bleibt 30 Sekunden in
  der Karenz. Wer in 30 Sekunden durch acht Sender schaltet, belegt alle acht Eingangsströme. Alle anderen
  Haushaltsmitglieder bekommen «Der Server ist ausgelastet». Beim Anbieter sind acht Verbindungen offen, und
  das Konto wird gesperrt (LTV-017: Slouchcat kennt das Limit nicht). Die Rangfolge in 8.2 nennt Ströme in der
  Karenz nicht als verdrängbar. Die Stream-Grenze je Konto ist ab Werk leer, also unbegrenzt
  (`max_streams` NULL). Ein Konto kann damit beliebig viele Live-Sitzungen mit je eigenem Sitzungspuffer
  öffnen. Ein Segmentabruf ausserhalb des Erzeugten startet FFmpeg neu (`04:482-484`). Wer das Token hat, startet
  so mit `GET` beliebig oft Prozesse.
- **Beleg:** [belegt Code] `server/migrations/00011_playback_complete.sql:48`, `:53` (`max_streams` NULL
  erlaubt), `00063_minimum_age_accounts.sql:41`. [belegt Doku] `04:350-353`, `04:509-524`.
- **Korrektur (Pflicht vor Stufe 1):** Drosselung des Einstellens je Sitzung und je Profil (Token-Bucket,
  Richtwert 1 je Sekunde, Stoss 5; über `internal/ratelimit`). Ströme in der Karenz sind die **ersten**, die
  bei Mangel weichen, vor dem Vorladen. Höchstens ein Strom in Karenz je Sitzung. Feste Serverdeckel für
  Live-Sitzungen und Sitzungspuffer-Bytes, unabhängig von `max_streams`. Höchstens 2 Neustarts von FFmpeg je
  Sitzung und 10 Sekunden.

### RS-11 · Wiederverwendete Verbindungen umgehen die Prüfung je Quelle · **mittel**

- **Stelle:** `05` Abschnitt 2.3 «Regel je Quelle» (Zeile 91).
- **Szenario:** Die Prüfung sitzt in `Dialer.Control` und läuft nur beim **Aufbau** einer Verbindung. Teilen
  sich Quellen mit verschiedenen Regeln ein `http.Transport`, holt eine Anfrage einer Quelle «nur öffentlich»
  eine offene Verbindung aus dem Vorrat, die für eine freigegebene Quelle zum selben Host:Port aufgebaut
  wurde. Ohne `Control`. Ein Listen-Host muss dafür nur die Adresse eines freigegebenen Geräts kennen (oft
  bekannte Standardadressen wie `192.168.1.1`).
- **Beleg:** [belegt Code] `netguard.go:93-111` baut aus diesem Grund zwei getrennte Transports (öffentlich und
  lokal). Das Muster muss für «Regel je Quelle» erhalten bleiben. Der Entwurf sagt das nicht.
- **Korrektur (Pflicht vor Stufe 1):** Ein `http.Transport` je Quelle (und je Regelstand). Ändert sich die
  Regel, wird er geschlossen (`CloseIdleConnections`). Kein gemeinsamer Transport zwischen Quellen. Testfall
  in Abschnitt 6.

### RS-12 · UDP, RTP und Multicast: eingeschleuste Pakete aus dem Heimnetz · **mittel**

- **Stelle:** `05` Abschnitt 2.2 (UDP, RTP, Multicast), 2.3 (Zeile 98); `04` Abschnitt 10.6
  (`compose.live-udp.yaml`, `compose.live-hostnet.yaml`), Risiko «Heimnetz-Quellen über UDP» (Zeile 705).
- **Szenario:** Die Absenderadresse eines UDP-Pakets lässt sich im selben Netz beliebig fälschen. «Pakete von
  anderen Absendern werden verworfen» schützt deshalb nicht gegen ein Gerät im Heimnetz (Gast-WLAN im selben
  Netz, gekapertes Smart-Gerät). Es kann (a) beliebiges Bild und beliebigen Ton in einen Sender einspeisen, den
  auch Kinderprofile sehen (LTV-015) und der aufgenommen wird; (b) gezielt bösartige Mediendaten an FFmpeg und
  den Go-Parser schicken (RS-03, RS-04); (c) den Port mit Datenmüll fluten. Im Netzmodus «host» lauscht der
  Socket auf allen Schnittstellen des Hosts, mit einer offenen Firewall auch am Internet. Sendet der Empfänger
  RTCP-Berichte an eine gefälschte Adresse, wird er zum Verstärker [Annahme, der Entwurf sagt nichts zu RTCP].
- **Beleg:** [belegt Doku] `05:66-68`, `05:98`; `04:655-656`, `04:705`.
- **Korrektur (Pflicht vor Stufe 3):** Socket an die Adresse einer gewählten Schnittstelle binden, nie an
  `0.0.0.0`. Für Multicast Quellen-spezifischer Beitritt (Source-Specific Multicast, IGMPv3), wo die Quelle das
  zulässt. Der Empfänger sendet nichts, kein RTCP. Obergrenze für Pakete je Sekunde und Bytes je Sekunde
  (das Doppelte der gemessenen Bitrate); ein Überschuss wird verworfen und gezählt. Die Verwaltung sagt
  wörtlich, dass UDP-Quellen jedem Gerät im Heimnetz vertrauen (Text V-3 ergänzen). Empfehlung: Multicast über
  ein eigenes VLAN.

### RS-13 · Netzmodus «host» ist eine nicht benannte Lockerung · **mittel**

- **Stelle:** `04` Abschnitt 10.6 (Zeilen 656-660), O-10 (Zeile 720).
- **Szenario:** Mit `network_mode: host` teilt der Container das Netz des Hosts. Damit (a) ist `127.0.0.1` das
  Loopback des Hosts. Alles, was dort lauscht (Docker-API auf TCP, Datenbanken, Proxmox-Dienste, der
  Weiterleiter aus Weg W), erreicht dann jeder Fehler der SSRF-Abwehr und jeder kompromittierte FFmpeg. (b) Der
  Weiterleiter und sein Kennwort (in `ps` des Hosts sichtbar) stehen allen Benutzern des Hosts offen (RS-02
  Szenario 4). (c) Der Server bekommt die IPv6-Adressen des Hosts. Damit greift die bekannte Lücke
  «weltweite IPv6-Adresse im Heimnetz wird nicht erkannt» (`netguard.go:44-47`). Im Bridge-Netz ohne IPv6 ist
  sie heute praktisch geschlossen. (d) Die Annahmen über vertrauenswürdige Proxys ändern sich (O-10). `04`
  beschreibt die Zusatzdatei als freiwillige Betriebsoption, nicht als Ausnahme von Sicherheitsregeln.
- **Beleg:** [belegt Doku] `04:656-660`; [belegt Code] `deploy/compose.yaml:3-7`, `:29-40`.
- **Korrektur (Pflicht vor Stufe 3):** Als benannte Ausnahme A-9 in `02` aufnehmen, mit diesen vier Folgen im
  Wortlaut. Weg W ist im Netzmodus «host» gesperrt. `netguard` verweigert dort zusätzlich die eigenen Adressen
  des Hosts (IPv4 und IPv6, beim Start aus den Schnittstellen gelesen). Als Alternative mit kleinerem Schaden
  ist `macvlan` oder `ipvlan` nur für Multicast zu prüfen (O-10).

### RS-14 · Grenzen für M3U, Xtream und HLS sind nicht festgelegt · **mittel**

- **Stelle:** `04` Abschnitt 3.2 (`live/catalog`); `05` Abschnitt 2.3 (nur HLS-Listengrösse und Segment).
- **Szenario:** Eine Liste mit 10 GB, einer einzigen Zeile ohne Zeilenumbruch, einer Million Einträgen,
  ungültigem UTF-8 oder einer Xtream-Antwort `get_live_streams` mit tief verschachteltem JSON hält den Import
  an, füllt den Speicher und hält den einzigen SQLite-Schreiber auf. Eine HLS-Liste mit `#EXT-X-TARGETDURATION:0`
  lässt Go die Liste in einer engen Schleife neu laden. Eine Hauptliste mit tausenden Varianten oder eine Kette
  von Unterlisten (A→B→A) vervielfacht die Anfragen. Die Zahlen stehen nur im Rohbericht R3
  (Zeilen 29-31: 64 KiB je Zeile, 50 bis 100 MB), nicht im Konzept.
- **Beleg:** [belegt Doku] `R3-eingangsquellen-protokolle.md:29-31`, `:108`; keine Fundstelle in `04`, `05`, `15`.
- **Korrektur (Pflicht vor Stufe 1), Richtwerte als Annahme bis zur Messung:** M3U höchstens 200 MiB
  (Abruf und Upload, eigener Eintrag in `largeBodies`), Zeile höchstens 64 KiB, höchstens 200'000 Einträge,
  Attribut höchstens 4 KiB, ungültiges UTF-8 ersetzt statt Abbruch, Import höchstens 10 Minuten. Xtream-JSON
  streamend über `json.Decoder.Token`, höchstens 200 MiB, Tiefe höchstens 32. HLS: Neuladen frühestens nach
  max(1 s, halbe Zieldauer), Zieldauer höchstens 60 s, höchstens 32 Varianten, Kette höchstens 3 Ebenen und
  zusammen mit Weiterleitungen höchstens 5 Sprünge (Muster `radio/reader.go:33-34`). Schreiben in Stapeln von
  500 Zeilen (`04:252-253`).

### RS-15 · XMLTV: kein XXE, aber ein einzelnes Token ist unbegrenzt · **mittel**

- **Stelle:** `09` Abschnitt 3.3, Abschnitt 12.
- **Prüfung am Verhalten der Standardbibliothek** [belegt Code, Go 1.27.3,
  `src/encoding/xml/xml.go`]: Eine Direktive `<!DOCTYPE …>` oder `<!ENTITY …>` wird nur als Text gesammelt
  und als `Directive` zurückgegeben, nie ausgewertet (`:718-760`). Ein Entitätsverweis `&name;` wird nur aus den
  fünf vordefinierten Namen und aus `Decoder.Entity` ersetzt (`:1036-1112`). Der Ersatztext wird wörtlich
  eingefügt und nicht erneut gelesen. Unbekannte Namen ergeben im strikten Modus den Fehler «invalid character
  entity» (`:1117-1126`). Externe Entitäten werden nie geladen, es gibt keine Rekursion und damit keine
  «Billion Laughs». `Unmarshal` und `DecodeElement` begrenzen die Tiefe auf 10'000 (`read.go:319-327`).
  Die Annahme in `09:839-840` ist damit **belegt**. Mit `d.Entity = xml.HTMLEntity` ist die Ersetzung
  ebenfalls einstufig.
- **Was trotzdem fehlt:** Der Decoder puffert ein einzelnes Token (Text, Attribut, Kommentar, Direktive)
  vollständig in `d.buf`, ohne Grenze. Eine Datei mit einem einzigen `<desc>` von 2 GB oder einer endlosen
  Direktive bringt den Server an die Speichergrenze, **bevor** die Grenze «ein `<programme>` höchstens 256 KiB»
  (`09:841`) prüfen kann. `DecodeElement` liest das Element erst ganz, gemessen wird danach. Der Zeichensatz-Weg
  des Podcast-Parsers liest alles in den Speicher (`09:280` nennt das richtig). Bei zip vertraut der Entwurf der
  Grössenangabe nicht, das ist richtig. Offen bleiben die Kompressionsart, abweichende lokale und zentrale
  Einträge und zip64.
- **Korrektur (Pflicht vor Stufe 2):** Ein zählender Leser zwischen Entpacker und Decoder bricht ab, sobald
  `d.InputOffset()` seit dem letzten Start eines Elements der obersten Ebene (`<channel>`, `<programme>`) mehr als
  256 KiB beträgt. Damit ist auch jedes einzelne Token begrenzt. Zeichensatz streamend über
  `golang.org/x/text/encoding` (schon Abhängigkeit, `server/go.mod:23`). zip: nur `Store` und `Deflate`, genau
  ein Eintrag im zentralen Verzeichnis, Eintrag über `io.LimitReader` gezählt. Testdateien: 2 GB Text in einem
  Element, `<!DOCTYPE` mit internen Entitäten, `&xxe;`, Tiefe 100'000, gzip-Bombe 1:1000, zip mit zwei
  Einträgen, zip mit Methode bzip2.

```go
// GOOD: no single element (and so no single token) may exceed the budget.
type elementBudget struct {
	r     io.Reader
	dec   *xml.Decoder
	start int64 // InputOffset at the start of the current top-level element
	limit int64
}

func (b *elementBudget) Read(p []byte) (int, error) {
	if b.dec.InputOffset()-b.start > b.limit {
		return 0, errElementTooLarge
	}
	return b.r.Read(p)
}
```

### RS-16 · Argumente und Filtergraph aus Werten der Quelle · **mittel**

- **Stelle:** `04` T-13 (neue Datei `transcode/live.go`), T-15 (Zeilensprung), Abschnitt 7.4 (Einbrennen von
  DVB-Untertiteln über `overlayGraph`).
- **Szenario:** Die Argumentliste schützt gegen Shell-Injektion (Regel 3). Sie schützt nicht gegen Werte, die
  FFmpeg selbst auswertet. Gelangt ein Wert aus der Quelle (Sprachkennung aus der PMT, Dienstname, Codecname aus
  `ffprobe`, Titel aus dem EPG) in einen Filtergraphen, kann ein Wert wie `x,movie=/config/secret.key` oder
  `amovie=http://…` eine weitere Eingabe öffnen [Annahme: dass die Filter `movie`/`amovie` die
  `-protocol_whitelist` des Eingangs nicht erben]. Ohne `-f mpegts` errät FFmpeg das Format einer Pipe am Inhalt.
  Ein präparierter Strom kann dann als `hls`- oder `concat`-Liste erkannt werden. Die Erlaubnisliste `pipe`
  hält das nach heutigem Wissen auf, ein Grund mehr für ein festes Format.
- **Beleg:** [belegt Code] Das Radio setzt Format und Erlaubnisliste (`transcode/radio.go:75-82`); die übrigen
  FFmpeg-Aufrufe ausser Bild und Radio haben keine Erlaubnisliste (V1 L-5).
- **Korrektur (Pflicht vor Stufe 1):** In die Argumentliste eines Live-FFmpeg gelangen nur Ganzzahlen (PIDs,
  Indizes, Bitraten, Grössen) und Werte aus festen Listen (Codec, Filter, Backend). Kein Text aus Quelle, EPG oder
  Profil, auch kein `-metadata`. Jeder Eingang mit `-f mpegts` (Weg W: `-f hls` oder `-f dash`). Ein Test prüft
  jede erzeugte Argumentliste gegen eine Erlaubnisliste von Mustern.

### RS-17 · Kopfzeilen werden nur maskiert, wenn ihr Name geheim klingt · **mittel**

- **Stelle:** `15` LA-03 (Zeile 290): «Werte gelten als geheim und werden maskiert, wenn ihr Name nach
  Schlüssel klingt (`Authorization`, `Cookie`, `X-Api-Key`)».
- **Szenario:** Viele Anbieter nutzen eigene Namen (`X-Token`, `X-Customer`, `Referer` mit Signatur). Diese
  Werte erscheinen in der Verwaltung, in Bildschirmfotos und in «Text kopieren» der Diagnose. Das widerspricht
  `05:109-110` («alle frei eingetragenen» gelten als geheim) und LTV-Q-10.20.
- **Beleg:** [belegt Doku] `15:290` gegen `05:109-110` und `03` LTV-Q-10.20.
- **Korrektur (Pflicht vor Stufe 1):** Alle Kopfzeilen-Werte sind nur schreibbar. Die API liefert nur die Namen.
  Die Maskierung geschieht im Server, nicht in der Oberfläche.

### RS-18 · Lücken im Adressfilter, die Live-TV vergrössert · **niedrig**

- **Stelle:** `05` Abschnitt 2.3 «IPv6» (Zeile 97); `netguard.Local`.
- **Szenario:** Nicht als lokal erkannt werden: das lokale NAT64-Präfix `64:ff9b:1::/48` (RFC 8215, bettet eine
  IPv4-Adresse ein), Teredo `2001::/32`, `192.0.0.0/24`, `240.0.0.0/4`, die IPv4-übersetzte Form
  `::ffff:0:0:0/96`, die öffentliche Adresse des eigenen Anschlusses (viele Router zeigen ihre Verwaltung auch auf
  der WAN-Adresse) und weltweite IPv6-Adressen im Heimnetz (bekannt). Live-TV vergrössert die Wirkung, weil
  Listen tausende fremd bestimmte Adressen bringen und Prüfungen sie selbsttätig abrufen.
- **Beleg:** [belegt Code] `netguard.go:34-54`.
- **Korrektur (empfohlen, vor Stufe 1):** Präfixe ergänzen (wirkt auch für Podcasts und Radio, das ist eine
  Verschärfung, keine Lockerung). Im Netzmodus «host» die eigenen Präfixe sperren (RS-13). Die WAN-Adresse als
  bekannte Restlücke im Sicherheitskonzept benennen.

### RS-19 · 401 oder 404, Zeitverhalten, aufzählbare Kennungen · **niedrig**

- **Stelle:** `03` Abschnitt 2.3 («jede Anfrage … eine Segment- oder Bildadresse … 404, nie 403»); `04`
  Abschnitt 10.3.
- **Szenario:** Die gebauten Token-Routen antworten bei falschem Token mit 401 (`api/stream_token.go:24-43`,
  `api/radio_binary.go:42-56`). Der Entwurf verlangt 404 für alle Live-Routen. Bleibt es bei der Mischung,
  unterscheidet ein Angreifer «Token falsch» von «Recht fehlt». Das ist harmlos, solange das Token die einzige
  Berechtigung ist, aber es widerspricht der Regel. `playback.Authorize` liest erst die Datenbank und prüft dann
  das HMAC (`playback/sessions.go:803-822`). Die Antwortzeit verrät so, ob eine Sitzungskennung existiert. Bei
  zufälligen Kennungen (ULID) ist das ohne Nutzen. Fortlaufende Kennungen von Aufnahmen und Planungen würden
  dagegen deren Zahl verraten.
- **Korrektur (vor Stufe 1):** Für Live-Routen festlegen: Token ungültig, fremd oder abgelaufen gibt
  **dieselbe** 404-Antwort (gleicher Körper) wie «kein Recht». HMAC zuerst prüfen, dann lesen. Aufnahmen,
  Planungen, Erinnerungen und Sitzungen bekommen zufällige Kennungen (ULID) in der API.

### RS-20 · Verschlüsselte Werte sind nicht an ihre Zeile gebunden · **niedrig**

- **Stelle:** `04` Abschnitt 10.5; `server/internal/secrets`.
- **Szenario:** Die zusätzlichen Daten (AAD) von AES-GCM sind nur die Versionskennung (`secrets.go:121`, `:136`).
  Wer die Datenbank schreiben kann, ohne den Schlüssel zu kennen (eine manipulierte Sicherung, die
  wiederhergestellt wird), kopiert das verschlüsselte Passwort einer Quelle in eine zweite Quelle, deren Adresse
  auf seinen Host zeigt. Der Server schickt ihm dann das Passwort im Klartext.
- **Korrektur (empfohlen, vor Stufe 1, weil Live-TV neue Spalten bekommt):** AAD für neue Werte = Tabelle,
  Spalte und Zeilenkennung (`v2:`); bestehende `v1:`-Werte bleiben lesbar.

### RS-21 · Logos und Sendungsbilder: viele fremde Bilder durch FFmpeg · **niedrig**

- **Stelle:** `05` Abschnitt 2.3 «Logos und EPG-Bilder»; `09` Abschnitt 9.
- **Szenario:** Eine Liste mit 5'000 Sendern bringt 5'000 `tvg-logo`-Adressen auf beliebigen Hosts. Der
  Netzschutz ist richtig vorgesehen (immer öffentlich, 10 MiB, Pixelgrenze `transcode.MaxImagePixels`,
  `radio/logo.go:472`, `:525`). Skaliert wird aber mit FFmpeg (`transcode/images.go:108`, `:174`), also mit
  C-Decodern auf fremden Bildern (RS-03). Jeder Abruf verrät dem Bild-Host die öffentliche Adresse des
  Servers und den Zeitpunkt des Imports.
- **Korrektur (empfohlen, vor Stufe 1):** Bilder zuerst mit den Go-Decodern (`image/png`, `image/jpeg`,
  `image/gif`, `golang.org/x/image/webp`) vollständig dekodieren. Nur was das besteht, geht an FFmpeg zum
  Skalieren, oder Go skaliert selbst. Abruf gedrosselt (Muster `radio.logo.fetch`) und erst, wenn ein Sender aktiv
  ist.

### RS-22 · Speicherorte: Vorgaben für Deckel, Ausweichordner, Bereinigung, Rechte · **mittel**

- **Stelle:** `04` T-25, Abschnitt 8.3, Abschnitt 9 «Datenträger ausgefallen» (Zeile 565), «Verwaiste
  Dateien» (Zeile 563); `03` DR-07.
- **Szenario:** (a) Der Deckel über alle Sitzungspuffer hat keinen Wert ab Werk. Sitzungspuffer liegen ab Werk im
  Cache-Volume, auf dem auch die Umwandlung der Filme schreibt. 20 Sitzungen × 60 Minuten × 8 Mbit/s sind rund
  72 GB, was eine kleine SSD füllt. Dann scheitert auch die Wiedergabe von Filmen («Live-TV darf die bestehenden
  Medienfunktionen nicht beeinträchtigen», Auftrag Abschnitt 5). (b) Fehlt der Speicherort «Timeshift» (zum
  Beispiel die RAM-Disk), schreibt der Server in «den Ausweichordner des Cache». Das ist ein Ort, den der
  Administrator nicht gewählt hat (er wollte die SSD schonen). (c) Die Bereinigung beim Start und im Betrieb
  löscht ganze Ordner. Das gebaute `paths` löscht bewusst nur Dateien und nie rekursiv (`paths/remove.go:9-13`).
  Der Entwurf sagt nicht, womit er rekursiv löscht. (d) Dateinamen und Rechte sind offen.
- **Korrektur (Pflicht vor Stufe 4):** Deckel ab Werk = kleinster Wert aus 20 % des freien Platzes beim
  Einrichten und 50 GB; Mindestreserve des Volumes 5 GB oder 5 %; der Deckel gilt für den Ausweichordner mit.
  Ausweichen nur mit dem Mindestfenster (60 s) und nur, wenn der Administrator das eingeschaltet hat. Rekursives
  Löschen nur über `os.Root.RemoveAll` auf einer Wurzel, die beim Öffnen die eigene Markerdatei trägt; nie über
  Symlinks; nie die Wurzel selbst. Datei- und Ordnernamen ausschliesslich aus Kennungen (ULID, Zahlen), nie aus
  Sender- oder Sendungstiteln. Rechte 0700 für Ordner und 0600 für Dateien.

### RS-23 · Programmsuche: FTS5-Syntax aus Benutzereingabe · **niedrig**

- **Stelle:** `09` Abschnitt 8.4.
- **Szenario:** Eine ungeprüfte Eingabe in `MATCH` ergibt Syntaxfehler (Antwort 500) oder sehr teure Anfragen
  (viele Präfix-Ausdrücke mit `OR`).
- **Korrektur (vor Stufe 2):** Die Eingabe wird wie in der Bibliothekssuche in Wörter zerlegt, jedes Wort in
  Anführungszeichen gesetzt, höchstens 8 Wörter und 100 Zeichen; Drosselung je Profil. Geprüft und ohne Befund:
  E-Mail-Texte werden escaped (`notify/texts.go:85-92`), der Betreff ist einzeilig (`notify/email/email.go:258`,
  `:302`); Sendungstitel in Meldungen sind damit unkritisch.

### RS-24 · Neue Abhängigkeiten, FFmpeg-Fassung, Test-Dialer · **niedrig**

- **Stelle:** `04` F-1 (a), `05` Abschnitt 2.2 (RTSP über Bibliothek), V1 L-5 (Test-Variante des Dialers).
- **Szenario:** (a) `gortsplib`, `gortmplib`, `mediacommon` und `pion/*` bringen eigenen Netzcode. Der Entwurf
  setzt voraus, dass sie über einen eigenen Dialer verbinden und Weiterleitungen über ihn führen [Annahme]. Bei
  RTSP über UDP kann der Server im `Transport`-Kopf eine andere Adresse (`source=`, `destination=`) nennen, an die
  der Client Pakete schickt [Annahme]. (b) FFmpeg ist auf 7.1.4-3 festgelegt; Lücken darin erreichen jetzt
  Fremddaten (RS-03). (c) Eine Build-Variante `radiotest` erlaubt Loopback für e2e. Ein Live-Gegenstück darf nie
  in ein Release-Image gelangen.
- **Korrektur:** Vor Stufe 3: Nachweis, dass jede Bibliothek nur über den übergebenen Dialer verbindet (Test
  mit Dialer, der alles ablehnt); RTSP nur `Transport: TCP`, kein Rückfall auf UDP. `govulncheck` in der CI für
  `server/` (MS-24). Bei jedem Release ein Abgleich der FFmpeg-Fassung mit den bekannten Lücken. Ein CI-Schritt
  prüft, dass das Release-Binary ohne Test-Tags gebaut ist.

## 3. Bedrohungsmodell

### 3.1 Akteure

| Nr. | Akteur | Fähigkeit | Ziel |
|---|---|---|---|
| AK-1 | Böswilliger oder gekaperter **IPTV-Anbieter / Listen-Host** (M3U, Xtream-API) | bestimmt Sender-Adressen, Kopfzeilen in der Liste, Antworten von `player_api.php`, Weiterleitungen | Heimnetz erreichen, Zugangsdaten ernten, Server stören, Schadcode in FFmpeg |
| AK-2 | **Stream- oder Segment-Host, CDN** (auch der Host einer absoluten Adresse im Manifest) | bestimmt Mediendaten, Manifeste, Unterlisten, Schlüssel-Adressen, Weiterleitungen | wie AK-1, dazu DRM-Erkennung aushebeln |
| AK-3 | **EPG-Host** | bestimmt XMLTV (auch gepackt), `<icon>`-Adressen | Speicher und CPU erschöpfen, SSRF über Bilder, Inhalte einschleusen |
| AK-4 | **Logo- oder Bild-Host** | bestimmt Bilddaten | Schadcode in Bild-Decoder, Nachverfolgung des Servers |
| AK-5 | **Konto ohne Live-TV-Recht** | gültige Sitzung, direkte API-Aufrufe | Live-TV, EPG, Logos oder Aufnahmen trotzdem erreichen |
| AK-6 | **Profil eines anderen Kontos** | eigene Sitzung mit Live-TV-Recht | fremde Aufnahmen, Planungen, Erinnerungen, Puffer (IDOR) |
| AK-7 | **Profil im selben Konto**, auch das **Hauptprofil** | Profilwechsel, PIN-Lücken, Rechte des Hauptprofils | Aufnahmen anderer Profile sehen (RS-09) |
| AK-8 | **Kinderprofil** (Profil mit Altersgrenze) | eigene Sitzung | Live-TV ohne Freigabe nutzen; Altersfilter der Bibliothek über Live-TV umgehen |
| AK-9 | **Gast oder Gerät im Heimnetz** | erreicht Port 8096, UDP-Ports, Multicast; kann Absender fälschen | Pakete in UDP-Sender einschleusen, Fluten, Multicast mitlesen |
| AK-10 | **Administrator mit Fehlbedienung** | trägt Quellen ein, gibt Heimnetz frei, wählt Netzmodus «host», Speicherorte | ungewollte SSRF-Öffnung, volle Platte, Lecks in Bildschirmfotos |
| AK-11 | **Angreifer im Internet mit einem erbeuteten Token** (`lt`) oder einer Sitzung | Bearer-Adresse aus Verlauf, Proxy-Protokoll, Bildschirmfoto | fremd fernsehen, Platz belegen, Prozesse auslösen |
| AK-12 | **Benutzer auf dem Docker-Host** | `ps`, `/proc`, Loopback des Hosts | Kennwort des Weiterleiters, Argumentlisten (RS-02, RS-13) |

### 3.2 Schutzgüter

| Nr. | Schutzgut | Warum wichtig |
|---|---|---|
| SG-1 | **Heimnetz** (Router, NAS, Smart-Geräte, Docker-Host, Loopback-Dienste, Metadaten-Adressen) | der Server steht darin und wird sonst zum Sprungbrett |
| SG-2 | **Zugangsdaten der Anbieter** (Benutzer, Passwort, Token in Adressen, Kopfzeilen) | Missbrauch, Sperre des Zugangs, Kosten |
| SG-3 | **Geheimnisse des Servers** (`secret.key`, Datenbank, Sitzungen, API-Schlüssel der Umgebung) | mit ihnen sind alle anderen Schutzgüter offen |
| SG-4 | **Private Aufnahmen und Profildaten** (Aufnahmen, Planungen, Erinnerungen, Favoriten, Verlauf, Puffer) | LTV-009, A-6 |
| SG-5 | **Verfügbarkeit des Servers** (Wiedergabe von Filmen, laufende Aufnahmen, Jobs) | Auftrag Abschnitt 5: Live-TV darf Bestehendes nicht beeinträchtigen |
| SG-6 | **Bestehende Bibliothek** und ihre Sicherheitsregeln (Altersmodell, `items` nur über `visibility`, Medien nur lesend) | A-2, A-7, Regeln 2 und 4 |
| SG-7 | **Integrität des Bildes** (was Kinder sehen, was aufgenommen wird) | LTV-015, LTV-020 |

### 3.3 Vertrauensgrenzen

| Nr. | Grenze | Was sie überquert |
|---|---|---|
| VG-1 | Internet ↔ Serverprozess | Listen, Manifeste, Segmente, Schlüssel, XMLTV, Bilder, Xtream-JSON (über `netguard`) |
| VG-2 | Heimnetz ↔ Serverprozess | freigegebene Quellen, UDP/RTP/Multicast, Clients im Heimnetz |
| VG-3 | Serverprozess ↔ FFmpeg | Pipes (Regelfall), im Weg W HTTP über den Weiterleiter auf `127.0.0.1`; Umgebung, Argumente |
| VG-4 | Client ↔ API | Cookie, Bearer (`app`, `tv`), Token `lt` in Adressen; CSRF-Kopf |
| VG-5 | Profil ↔ Profil, Konto ↔ Konto | Abfragen in `liveq` mit Profil- und Kontokennung |
| VG-6 | Container ↔ Host | Netzmodus Bridge oder «host», Volumes (`/config`, `/cache`, Aufnahmen, `tmpfs`), Prozessliste des Hosts |
| VG-7 | Verwaltung ↔ übrige Rollen | Quellen, Freigaben, Geheimnisse nur schreibbar; technische Angaben ohne private Titel |

### 3.4 Angriffsflächen (neu durch Live-TV)

1. Abruf von M3U, Xtream-API, HLS (Liste, Unterliste, Segment, Schlüssel), HTTP-TS, DASH (Weg W), RTSP und RTMP
   (Weg L), XMLTV, Logos und EPG-Bilder.
2. Lauschende UDP-Sockets und Multicast-Beitritt.
3. Weiterleiter für Weg W auf `127.0.0.1`.
4. Neue Parser im Serverprozess: TS, RTP, HLS, MPD, M3U, Xtream-JSON, XMLTV mit Entpackern.
5. FFmpeg mit Video, Untertiteln und Manifesten aus Fremdquellen; `ffprobe` auf dem Ring.
6. Neue API: Einstellen, Herzschlag, Liste und Segment mit `lt`, Aufnahmen, Planungen, Erinnerungen, Suche,
   Logos; Verwaltung mit Quellen, Freigaben, Test, Diagnose, Upload (M3U, XMLTV bis 1 GiB).
7. Neue Speicherorte (Timeshift, Aufnahmen), Bereinigung beim Start und im Betrieb.
8. Zusatzdateien für Compose (UDP-Ports, Netzmodus «host», schreibbares Volume).

### 3.5 Bedrohungstabelle (STRIDE)

Risiko je Zeile in drei Ständen: **ohne Massnahmen** (die Funktion naiv gebaut), **mit dem Entwurf** (so wie
`04`, `05`, `09`, `15` ihn heute beschreiben) und **nach Korrektur** (mit den Massnahmen aus Abschnitt 6).
STRIDE: S Vortäuschen, T Verfälschen, R Abstreiten, I Informationsabfluss, D Dienstverweigerung, E Rechteausweitung.

| Nr. | Bedrohung | STRIDE | Akteur → Schutzgut | Angriffsfläche | ohne | Entwurf | nach Korrektur | Befund / Massnahme |
|---|---|---|---|---|---|---|---|---|
| B-01 | SSRF über Sender-, Listen-, Segment-, Schlüssel-Adresse einer öffentlichen Quelle | I, E | AK-1, AK-2 → SG-1 | 3.4/1 | hoch | niedrig | niedrig | Dialer-Prüfung gebaut; RS-11 (Vorrat), RS-18 |
| B-02 | SSRF über eine Quelle mit Heimnetz-Freigabe, Sender und Verweise aus der Liste | I, E | AK-1, AK-9, AK-10 → SG-1 | 3.4/1 | hoch | **hoch** | niedrig | RS-01, MS-01 |
| B-03 | DNS-Rebinding, Weiterleitung, IPv4-gemappte und eingebettete Adressen | I, E | AK-1, AK-2 → SG-1 | 3.4/1 | hoch | niedrig | niedrig | `netguard.go:70-82`; RS-18 |
| B-04 | FFmpeg verbindet in Weg W an Go vorbei; `CONNECT` ohne Einsicht | I, E | AK-2 → SG-1, SG-2 | 3.4/3, 3.4/5 | hoch | **hoch** | niedrig (mit Landlock), mittel (ohne) | RS-02, MS-02 |
| B-05 | SSRF über EPG-Adresse oder `url-tvg` | I | AK-3 → SG-1 | 3.4/1 | hoch | niedrig | niedrig | `09` 2.1, 3.2 gut; F-4 an RS-01 binden |
| B-06 | SSRF über Logo- und Sendungsbild-Adressen | I | AK-1, AK-3, AK-4 → SG-1 | 3.4/1 | mittel | niedrig | niedrig | `09` 9: immer öffentlich |
| B-07 | Zugangsdaten an einen Host, den die Liste bestimmt (Kopfzeilen, Xtream `server_info`) | I | AK-1 → SG-2 | 3.4/1 | hoch | **hoch** | niedrig | RS-05, RS-06 |
| B-08 | Zugangsdaten in Protokoll, Fehlerring, Diagnose | I | AK-10, Dritte mit Protokollzugang → SG-2 | Protokoll | hoch | mittel | niedrig | RS-07, MS-07 |
| B-09 | Zugangsdaten im Klartext in DB, Sicherung, `VACUUM INTO`, API der Verwaltung | I | Dritte mit Kopie, AK-10 → SG-2 | DB, Sicherung | hoch | **hoch** | niedrig | RS-05, RS-17, RS-20 |
| B-10 | Zugangsdaten in `/proc/<pid>/cmdline`, Umgebung, `ffmpeg.log` | I | AK-12 → SG-2 | 3.4/5 | hoch | niedrig | niedrig | T-4 gut; RS-02 Szenario 4, RS-03 (Umgebung) |
| B-11 | Speicherlücke in FFmpeg durch bösartigen Strom oder Bild → Code im Container | E, I | AK-1, AK-2, AK-4, AK-9 → SG-3, SG-1 | 3.4/5 | hoch | **hoch** | mittel | RS-03, RS-21, MS-03 |
| B-12 | Argument- oder Filtergraph-Injektion, Formatraten auf der Pipe | E, I | AK-1, AK-2, AK-3 → SG-3 | 3.4/5 | mittel | mittel | niedrig | RS-16 |
| B-13 | Panic in einem Go-Parser beendet den Server | D | AK-1, AK-2, AK-9 → SG-5 | 3.4/4 | hoch | **hoch** | niedrig | RS-04, MS-04 |
| B-14 | XXE und Entity-Expansion in XMLTV | I, D | AK-3 → SG-3, SG-5 | 3.4/4 | mittel | niedrig | niedrig | Go `encoding/xml` belegt sicher (RS-15) |
| B-15 | Dekompressionsbombe, riesiges Token, riesige Datei (XMLTV, M3U, JSON) | D | AK-1, AK-3 → SG-5 | 3.4/4 | hoch | mittel | niedrig | RS-14, RS-15 |
| B-16 | Endlose oder langsame Antworten, Neulade-Sturm bei HLS | D | AK-1, AK-2 → SG-5 | 3.4/1 | mittel | mittel | niedrig | Fristen gebaut als Muster; RS-14 |
| B-17 | UDP-Einspeisung: falsches Bild an Kinder, Schadmedien, Flut | T, D, E | AK-9 → SG-7, SG-5, SG-3 | 3.4/2 | hoch | mittel | mittel | RS-12; Restrisiko bleibt (UDP vertraut dem Netz) |
| B-18 | Netzmodus «host» öffnet Loopback und IPv6 des Hosts | I, E | AK-1, AK-2, AK-12 → SG-1, SG-3 | 3.4/8 | hoch | **hoch** | mittel | RS-13 |
| B-19 | Live-TV ohne Recht (direkte API, Segment, Logo, Suche) | E, I | AK-5, AK-8 → SG-4, SG-6 | 3.4/6 | hoch | niedrig | niedrig | 404 im Dienst `live`; Vertragstest je Operation (MS-19) |
| B-20 | Fremde Aufnahmen, Planungen, Erinnerungen, Puffer (IDOR) | I, T | AK-6, AK-7 → SG-4 | 3.4/6 | hoch | mittel | niedrig | Abfrage mit Profilkennung (`04` 10.3); MS-20 Matrix |
| B-21 | Hauptprofil hebt Privatsphäre über die Altersgrenze auf | I | AK-7 → SG-4 | 3.4/6 | mittel | mittel | niedrig | RS-09 (Frage an Marco) |
| B-22 | Erbeutetes `lt` hält die Sitzung unbegrenzt | S, I, D | AK-11 → SG-4, SG-5 | 3.4/6 | hoch | mittel | niedrig | RS-08 |
| B-23 | Widerruf wirkt nicht (Abmelden, Rechteentzug, Profil gelöscht) | E | AK-5, AK-11 → SG-4 | 3.4/6 | hoch | niedrig | niedrig | Haken `EndSession`/`EndProfile` (`04` 9); Anmeldesitzung prüfen (RS-08) |
| B-24 | Umschalten als Hebel: Eingangsströme, Anbieterlimit, Prozessstarts | D | AK-5 (mit Recht), AK-11 → SG-5, SG-2 | 3.4/6 | hoch | mittel | niedrig | RS-10 |
| B-25 | Sitzungspuffer füllen Platte und Cache der Filme | D | AK-5 (mit Recht), AK-10 → SG-5 | 3.4/7 | hoch | mittel | niedrig | RS-22 |
| B-26 | Bereinigung löscht ausserhalb der eigenen Ordner; Pfade aus Titeln | T, D | AK-3 (Titel), AK-10 → SG-6 | 3.4/7 | mittel | niedrig | niedrig | Marker (`04` 5.3); RS-22 |
| B-27 | Schreiben in leeren Einhängepunkt (nicht eingehängtes Volume) | D, T | AK-10 → SG-5, SG-6 | 3.4/7 | mittel | niedrig | niedrig | Marker und Dateisystemtyp (`04` 9) |
| B-28 | Live-TV verändert das Altersmodell der Bibliothek | E | AK-8 → SG-6 | – | mittel | niedrig | niedrig | A-2, A-8; keine `ItemRef` |
| B-29 | CSRF auf schreibende Live-Operationen | T | Fremde Webseite → SG-4 | 3.4/6 | mittel | niedrig | niedrig | CSRF-Kopf gebaut (`api/session.go:278-314`); Einstellen als `POST` (RS-08) |
| B-30 | Freigaben und Quellen ohne Nachweis geändert | R | AK-10 → SG-1 | Verwaltung | mittel | mittel | niedrig | Protokolleintrag je Änderung von Freigabe und Quelle (RS-01 Punkt 6) |
| B-31 | Inhalte aus EPG als HTML oder in Meldungen | T | AK-3 → Clients | 3.4/6 | mittel | niedrig | niedrig | Text statt HTML (`09` 12); Escaping in E-Mail belegt (RS-23) |
| B-32 | Verwaiste FFmpeg-Prozesse, Prozesse ohne Grenze | D | AK-1, AK-2 → SG-5 | 3.4/5 | mittel | niedrig | niedrig | Kontext, Abgleich je Minute (`04` 9); rlimits (RS-03) |

## 4. Abdeckung «Auftrag Abschnitt 10, Zeile → abgedeckt in / Lücke»

Zeilennummern des Auftrags in `anhang/auftrag-2026-11-28.md`. Stand: **abgedeckt** (Entwurf reicht),
**teilweise** (Ansatz richtig, Lücke benannt), **Lücke** (fehlt oder widerspricht).

### 10.1 Netzwerksicherheit

| Zeile | Punkt | Abgedeckt in | Stand | Lücke / Befund |
|---|---|---|---|---|
| 1324 | SSRF bei Stream-URLs | `05` 2.2, 2.3; `netguard.go:73-82` | teilweise | RS-01, RS-02, RS-11 |
| 1325 | SSRF bei EPG-URLs | `09` 2.1, 3.2, 12 | abgedeckt | F-4 an die festen Ziele aus RS-01 binden |
| 1326 | SSRF bei M3U-URLs | `05` 2.3 | teilweise | Sender aus der Liste erben die Freigabe (RS-01 Punkt 4) |
| 1327 | Weiterleitungen | `05` 2.3 (5 Sprünge, nur http/https) | teilweise | Weg W reicht 3xx durch (RS-02 Szenario 3); Kopfzeilen bei Ursprungswechsel (RS-06) |
| 1328 | DNS-Rebinding | `05` 2.3; `netguard.go:70-82` | abgedeckt | Vorrat der Verbindungen (RS-11); `LocalHost` mit eigener Auflösung nicht verwenden (RS-01) |
| 1329 | IPv4 | `netguard.go:34-54` | abgedeckt | kleine Präfix-Lücken (RS-18) |
| 1330 | IPv6 | `netguard.go:34-54`; `05:97` | teilweise | globale IPv6 im Heimnetz, NAT64 `64:ff9b:1::/48`; im Netzmodus «host» relevant (RS-13, RS-18) |
| 1331 | Zugriff auf interne Netzwerke | `02` A-3; `05` 2.3 | **Lücke** | Widerspruch A-3 / «Regel je Quelle», Loopback freigebbar (RS-01) |
| 1332 | Explizite Freigaben lokaler Quellen | `02` A-3; `15` LA-03 | **Lücke** | Ja/Nein statt fester Ziele; kein Protokolleintrag (RS-01) |
| 1333 | Multicast- und UDP-Risiken | `05` 2.2, 2.3; `04` 10.6 | teilweise | gefälschte Absender, Binden, RTCP, «host» (RS-12, RS-13) |
| 1334 | Verschachtelte HLS- und DASH-Manifeste | `05` 2.3 (Weg G), 2.6 (Weg W) | teilweise | Weg G gut; Weg W über `CONNECT` ohne Einsicht (RS-02); Kettenlänge (RS-14) |
| 1335 | Segment-URLs | `05` 2.3 | teilweise | wie 1334; bei Freigabe RS-01 |
| 1336 | Schlüssel-URLs | `05` 2.3, 2.5 (64 Byte) | teilweise | Weg W (RS-02) |
| 1337 | Playlist-URLs | `05` 2.3 (1 MiB) | teilweise | Neuladen-Takt, Varianten (RS-14) |
| 1338 | Externe Senderlogos | `05` 2.3; Muster `radio/logo.go` | teilweise | immer öffentlich festschreiben; Decoder (RS-21) |
| 1339 | Externe EPG-Bilder | `09` 9 | abgedeckt | Decoder (RS-21) |

### 10.2 Zugangsdaten

| Zeile | Punkt | Abgedeckt in | Stand | Lücke / Befund |
|---|---|---|---|---|
| 1343 | Sichere Speicherung | `05` 2.4; `04` 10.5; `secrets.go` | teilweise | Stream-Adressen der Sender, AAD (RS-05, RS-20) |
| 1344 | Zugangsdaten in URLs | `05` 2.4; `15` Grundsatz 5 | **Lücke** | Erkennung nach Muster; `user:pass@`; Xtream `server_info` (RS-05, RS-06) |
| 1345 | Konfigurierbare HTTP-Header | `05` 2.4; `15` LA-03 | **Lücke** | Maskierung nur bei «geheim klingenden» Namen (RS-17); M3U-Kopfzeilen (RS-06) |
| 1346 | Authentifizierung gegenüber Quellen | `05` 2.2, 2.4 | teilweise | Ursprungsmenge fehlt (RS-06) |
| 1347 | Keine Zugangsdaten in Logs | `04` 10.4 | teilweise | RS-07 |
| 1348 | Keine Zugangsdaten in Client-Antworten | `05` 2.4 «Antworten an Clients» | teilweise | Verwaltung bekommt Adressen (RS-05 Punkt 2) |
| 1349 | Keine Zugangsdaten in Fehlermeldungen | `15` LA-03, LA-13; `netguard.Clean` | abgedeckt | Fehlerring und «Text kopieren» an die Schwärzung binden (RS-07) |
| 1350 | Schutz in Backups | `04` 10.5 | teilweise | Klartext-Adressen in Sicherung ohne Geheimnisse (RS-05) |
| 1351 | Keine Weitergabe an nicht autorisierte Ziele | `05` 2.4 | teilweise | RS-06 |

### 10.3 FFmpeg und externe Prozesse

| Zeile | Punkt | Abgedeckt in | Stand | Lücke / Befund |
|---|---|---|---|---|
| 1355 | Command Injection | `04` T-24 (Regel 3) | teilweise | Filtergraph, Formatraten (RS-16) |
| 1356 | Protokollbeschränkungen | `05` 2.6 (`pipe`; Weg W Liste) | teilweise | Liste im Weg W zu weit (RS-02) |
| 1357 | Bösartige Medienstreams | – | **Lücke** | keine Eindämmung von FFmpeg (RS-03); Go-Parser (RS-04) |
| 1358 | Fehlerhafte Mediendaten | `05` 2.7, 3.2 | teilweise | `panic` (RS-04) |
| 1359 | Prozesslimits | `04` 8.3 (Einheiten, Eingangsströme) | teilweise | rlimits, `pids_limit` (RS-03); Neustarts beim Spulen (RS-10) |
| 1360 | Timeouts | `04` 7.5 (60 s Leerlauf), `05` 2.2 (10 s Stillstand) | abgedeckt | – |
| 1361 | Sichere Prozessbeendigung | `04` 5.3 (SIGTERM, 2 s, SIGKILL) | abgedeckt | gebaut `session.go:662-666` |
| 1362 | Verwaiste Prozesse | `04` 9 | abgedeckt | – |
| 1363 | Ressourcenerschöpfung | `04` 8 | teilweise | `RLIMIT_AS`, Speichergrenze des Containers (RS-03) |
| 1365 | Indirekt abgerufene URLs umgehen den SSRF-Schutz nicht | `04` T-4; `05` 2.6 | **Lücke** | nur testbar, nicht erzwungen; `CONNECT` (RS-02); RTSP/RTMP-Bibliotheken (RS-24) |

### 10.4 Import- und Parser-Sicherheit

| Zeile | Punkt | Abgedeckt in | Stand | Lücke / Befund |
|---|---|---|---|---|
| 1369 | XML External Entities | `09` 12 | abgedeckt | am Quelltext belegt (RS-15); für DASH in FFmpeg RS-02 Punkt 4 |
| 1370 | XML-Bomben | `09` 12 | abgedeckt | belegt (RS-15) |
| 1371 | Komprimierte XMLTV-Dateien | `09` 2.2 | teilweise | zip-Einzelheiten (RS-15) |
| 1372 | Dekompressionsbomben | `09` 12 | abgedeckt | Zähler nach dem Entpacken, Verhältnis 100 |
| 1373 | Fehlerhafte M3U-Dateien | – | **Lücke** | Grenzen fehlen im Konzept (RS-14) |
| 1374 | Uploadlimits | `09` 2.1, 12 (XMLTV 1 GiB) | teilweise | M3U-Upload ohne Grenze (RS-14) |
| 1375 | Dateigrössenlimits | `05` 2.3; `09` 12 | teilweise | M3U, Xtream-JSON (RS-14); einzelnes XML-Token (RS-15) |
| 1376 | Parser-Zeitlimits | `09` 3.2 (30 min) | teilweise | M3U und Xtream (RS-14) |
| 1377 | Ressourcenerschöpfung | `09` 12; `04` 4.2 (Stapel 500) | teilweise | RS-14, RS-15 |

### 10.5 Dateisystem und Speicherung

| Zeile | Punkt | Abgedeckt in | Stand | Lücke / Befund |
|---|---|---|---|---|
| 1381 | Unbegrenzte Puffer | `04` T-7 (15 s, 64 MiB), DR-07 | teilweise | Deckel ohne Wert; unbegrenzte Sitzungen (RS-10, RS-22) |
| 1382 | Path Traversal | `04` T-24, T-25 (`paths`) | teilweise | Namen nur aus Kennungen festschreiben (RS-22) |
| 1383 | Dateisystemrechte | – | **Lücke** | 0700/0600 (RS-22) |
| 1384 | Zugriffsschutz Timeshift | `04` T-10, T-21 | abgedeckt | nur über `lt` der eigenen Sitzung; RS-08 |
| 1385 | Zugriffsschutz Aufnahmen | `04` 10.3; `03` DR-02 | teilweise | Abfrage und Matrix; RS-09 |
| 1386 | Datenträgerausfälle | `04` 8.3, 9 | abgedeckt | Ausweichordner (RS-22) |
| 1387 | Sichere Bereinigung temporärer Dateien | `04` 5.3, 9 (Marker) | teilweise | rekursives Löschen nicht festgelegt (RS-22) |

### 10.6 Authentifizierung und Autorisierung

| Zeile | Punkt | Abgedeckt in | Stand | Lücke / Befund |
|---|---|---|---|---|
| 1391 | Authentifizierung Stream-Endpunkte | `04` T-21 | abgedeckt | RS-08, RS-19 |
| 1392 | Authentifizierung Segment-Endpunkte | `04` T-21 | abgedeckt | wie oben |
| 1393 | Prüfung bei jedem relevanten Zugriff | `04` 9 «Entzug» (30 s) | teilweise | Anmeldesitzung mitprüfen (RS-08) |
| 1394 | Widerruf laufender Berechtigungen | `04` 9; `03` 2.4 | abgedeckt | Haltegrund durch Segmentabruf (RS-08) |
| 1395 | Schutz privater Profildaten | `04` 10.3; `03` 5.9 | abgedeckt | Verwaltung zeigt Sender je Profil (DR-17, wie heute Titel) |
| 1396 | Schutz privater Aufnahmebibliotheken | `02` A-6; `03` DR-02 | teilweise | RS-09 |
| 1397 | Bestehende Session- und Token-Architektur | `04` T-21; ADR 0040 | abgedeckt | `x-clients` für alle `scoped`-Operationen; Token gibt nur eine geprüfte Operation aus |
| 1398 | Bestehende Account-Limits | `04` T-26 | teilweise | `max_streams` ab Werk unbegrenzt: Serverdeckel (RS-10) |
| 1399 | Serverseitige Durchsetzung | `04` 10.3; Regel 12 | abgedeckt | Vertragstest je Operation (MS-19) |
| 1400 | Unsichtbares liefert 404 | `03` 2.3; `04` 10.3 | teilweise | 401 der Token-Routen (RS-19) |
| 1402 | Bestehende Anforderungen bleiben gültig | `02` A-1 bis A-8 | teilweise | Abschnitt 5 dieses Berichts |
| 1404 | Keine stillschweigende Lockerung | `03` LTV-Q-10.64 | **Lücke** | fünf Lockerungen (Abschnitt 5) |
| 1406 | Unabhängige Security Reviewer | dieser Bericht | abgedeckt | ein zweites Review nach `12-sicherheit.md` empfohlen |

## 5. Stillschweigende Lockerungen

| Nr. | Lockerung | Wo | Was gelockert wird | Gegenmittel |
|---|---|---|---|---|
| SL-1 | Die Heimnetz-Freigabe gilt für alle Anfragen einer Quelle, auch für Verweise und Weiterleitungen; Loopback und Link-Local sind nicht ausgenommen. | `05:91-93` gegen `02` A-3 | ADR 0025 F-15 («nichts im lokalen Netz»), weiter als A-3 erlaubt | RS-01 |
| SL-2 | Weg W darf `https`, `tls`, `tcp`, `httpproxy` und `CONNECT`-Tunnel nutzen. Die benannte Ausnahme spricht nur von «HTTP zum Go-Weiterleiter». | `05:170-177` | ADR 0025 Punkt 2 und Auftrag 10.3 («indirekt abgerufene Adressen») weiter als benannt | RS-02 |
| SL-3 | Kopfzeilen-Werte werden nur bei bestimmten Namen maskiert. | `15:290` | `05:109-110` und LTV-Q-10.20 | RS-17 |
| SL-4 | Netzmodus «host» und veröffentlichte UDP-Ports als «freiwillige Zusatzdateien», ohne Nennung der Sicherheitsfolgen. | `04` 10.6 | Isolation des Bridge-Netzes; Annahmen der vertrauenswürdigen Proxys | RS-12, RS-13 |
| SL-5 | Das Hauptprofil sieht Aufnahmen eines Profils, sobald es ihm eine Altersgrenze setzt. | `03` 2.4 | A-6 («Profile ohne Altersgrenze bleiben ganz privat») | RS-09 |

Kandidaten, die **keine** Lockerung sind: Die neue Prozessklasse «Empfang» mit `oom_score_adj 100` statt 500
(T-20) ändert nur die Reihenfolge bei Speichermangel. Die eigene Datei `epg.db` ohne Sicherung (`09` F-1)
verliert Daten, gibt aber nichts preis. Dass FFmpeg-Fehlerausgaben bei Live-TV nie roh gespeichert werden
(`04` 10.4), ist eine **Verschärfung**.

## 6. Massnahmenkatalog

**Pflicht vor Stufe n** heisst: Die Stufe wird nicht veröffentlicht, solange die Massnahme und ihr Testfall
fehlen. Stufen nach `03` (0 Grundlagen, 1 Empfang HTTP/HLS/Xtream/M3U und Wiedergabe, 2 EPG, 3 DASH, RTSP,
RTMP, UDP, Multicast, Ersatzquellen, 4 Timeshift-Speicher, 5 Aufnahmen).

| Nr. | Bedrohung | Massnahme | Verbindlichkeit | Sicherheitstestfall (ein Satz) |
|---|---|---|---|---|
| MS-01 | B-02, B-05 | Heimnetz-Freigabe als feste Ziele (IP/Netz, Port), Prüfung im Dialer für jede Verbindung, Loopback, Link-Local und eigene Adressen nie freigebbar, Sender aus Listen erben nur feste Ziele, Protokolleintrag je Änderung (RS-01). | Pflicht vor Stufe 1 (Regel), Stufe 3 (UDP, Multicast) | Eine freigegebene Quelle mit Ziel `192.168.1.20:9981` erreicht `192.168.1.20:22`, `192.168.1.1:80`, `127.0.0.1:8096` und `169.254.169.254:80` nicht, weder als Sender noch über Weiterleitung, Unterliste oder Segment. |
| MS-02 | B-04 | Weg W: Umschreiben aller absoluten Adressen, kein `CONNECT`, Liste `http,tcp,crypto`, `-f hls`/`-f dash`, keine durchgereichten 3xx, MPD ohne DOCTYPE, Landlock-Sperre, gesperrt im Netzmodus «host» (RS-02). | Pflicht vor Stufe 3 | Mit einer feindlichen Testmenge (absolute `http`/`https`/`tcp`/`file`/`data`-Adressen, IP-Literale im Heimnetz, Weiterleitungen) zeigt die Mitschrift aller `connect()`-Aufrufe des FFmpeg-Prozesses nur den Port des Weiterleiters. |
| MS-03 | B-11, B-32 | FFmpeg: kleine ausdrückliche Umgebung, `-f mpegts -protocol_whitelist pipe` auch für `ffprobe`, `prlimit` (AS, NOFILE, CORE), Compose `no-new-privileges`, `cap_drop: [ALL]`, `pids_limit`; empfohlen Landlock ohne Lesezugriff auf `/config` (RS-03). | Pflicht vor Stufe 1 (ohne Landlock), Landlock Pflicht vor Stufe 3 | Ein Live-FFmpeg hat in `/proc/<pid>/environ` keine `SLOUCHCAT_*`-Variable, in `/proc/<pid>/limits` die gesetzten Grenzen und kann (mit Landlock) `/config/secret.key` nicht öffnen. |
| MS-04 | B-13 | `recover` an der Wurzel jeder Live-Goroutine; Fuzz-Ziele für TS, PSI, PES, RTP, HLS, MPD, M3U, Xtream-JSON, XMLTV-Element in der CI (RS-04). | Pflicht vor Stufe 1 (Stufe 2 für XMLTV, Stufe 3 für RTP und MPD) | Ein Paket, das im Parser einen Indexfehler auslöst, beendet nur den Empfang dieses Senders mit Quarantäne; der Server und eine laufende Filmwiedergabe laufen weiter. |
| MS-05 | B-09 | Jede Quell- und Stream-Adresse ganz verschlüsselt, daneben nur `site`; API der Verwaltung liefert nur `site`; Felder nur schreibbar; Xtream-Adressen nur aus dem eingetragenen Host (RS-05). | Pflicht vor Stufe 1 | Eine Sicherung ohne Geheimnisse und eine `VACUUM INTO`-Kopie enthalten keinen der Testwerte von Benutzer, Passwort, Token und Kopfzeilen, und keine API-Antwort enthält sie. |
| MS-06 | B-07 | Geheimnisse an eine feste Menge von Ursprüngen gebunden; eigenes `CheckRedirect`; kein `https`→`http` mit Geheimnissen; aus M3U nur `User-Agent` und `Referer`; `user:pass@` beim Anlegen zerlegen (RS-06). | Pflicht vor Stufe 1 | Ein Sender einer Liste auf einem fremden Host und eine Weiterleitung auf einen fremden Host empfangen weder `Authorization` noch `Cookie` noch eine eigene Kopfzeile der Quelle. |
| MS-07 | B-08 | Schwärzung nach Abschnitt 7, nur `netguard.Site` im Protokoll (Lint), `redactAny` für `http.Header`, `url.Values`, `[]string` (RS-07). | Pflicht vor Stufe 0 | Der Schwärzungstest enthält jedes Muster aus Abschnitt 7, und ein Protokoll mit allen Testwerten (Abfrage, Pfad, Benutzerteil, Kopfzeile, Argumentliste) enthält keinen davon. |
| MS-08 | B-22, B-23 | Nur der Herzschlag der eigenen Anmeldesitzung hält die Live-Sitzung; Prüfung von Recht und Anmeldesitzung bei Listenanfragen (höchstens 30 s alt); Höchstdauer je Einstellen; schreibende Operationen als `POST` mit CSRF-Kopf (RS-08). | Pflicht vor Stufe 1 | Ein Client, der nur Segmente holt und keinen Herzschlag schickt, verliert die Sitzung nach 60 Sekunden; nach dem Abmelden antwortet jede Adresse mit dem alten Token innert 30 Sekunden mit 404. |
| MS-09 | B-21 | Entscheid zu RS-09, Zugriffsregel als eine Abfrage in `liveq` (RS-09). | Pflicht vor Stufe 5 | Setzt das Hauptprofil einem Profil ohne Altersgrenze eine Grenze, sieht es dessen vorher entstandene Aufnahmen weiterhin nicht (bei Variante a). |
| MS-10 | B-24 | Drosselung des Einstellens je Sitzung und Profil; Karenz-Ströme weichen zuerst; höchstens ein Karenz-Strom je Sitzung; Serverdeckel für Live-Sitzungen und Puffer-Bytes; Neustart-Drosselung für FFmpeg (RS-10). | Pflicht vor Stufe 1 | Wer in 10 Sekunden durch 20 Sender schaltet, hält danach höchstens zwei Eingangsströme, und ein zweites Profil kann sofort einen anderen Sender einstellen. |
| MS-11 | B-01 | Ein `http.Transport` je Quelle und Regelstand, geschlossen bei Regeländerung (RS-11). | Pflicht vor Stufe 1 | Nach einem Abruf einer freigegebenen Quelle von `192.168.1.20` scheitert derselbe Abruf über eine Quelle «nur öffentlich» mit «Ziel im Heimnetz», auch innerhalb der Leerlaufzeit der Verbindung. |
| MS-12 | B-17 | UDP: Binden an eine Schnittstelle, SSM wo möglich, kein Senden, Paket- und Bytegrenzen, klarer Text in der Verwaltung (RS-12). | Pflicht vor Stufe 3 | Pakete mit gefälschtem Absender an den Port werden gezählt und verworfen, sobald sie die Bytegrenze überschreiten, und der Socket lauscht nicht auf `0.0.0.0`. |
| MS-13 | B-18 | Netzmodus «host» als benannte Ausnahme A-9; Weg W dort aus; eigene Adressen des Hosts gesperrt; Alternative `macvlan` prüfen (RS-13). | Pflicht vor Stufe 3 | Im Netzmodus «host» erreicht keine Quelle `127.0.0.1:<Port>` eines Testdienstes auf dem Host, und eine DASH-Quelle meldet «im Netzmodus host nicht verfügbar». |
| MS-14 | B-15, B-16 | Grenzen für M3U, Xtream-JSON und HLS mit Zahlen (RS-14). | Pflicht vor Stufe 1 | Eine M3U mit einer 10-MB-Zeile, eine mit 300'000 Einträgen und eine HLS-Liste mit Zieldauer 0 brechen mit einem stabilen Fehlercode ab, ohne dass der Speicher des Servers über die Grenze wächst. |
| MS-15 | B-14, B-15 | XMLTV: Budget je Element über `InputOffset`, Zeichensatz streamend, zip nur Store/Deflate und ein Eintrag (RS-15). | Pflicht vor Stufe 2 | Eine XMLTV mit einem 2-GB-`<desc>` bricht nach 256 KiB des Elements ab, und eine Datei mit `<!DOCTYPE>`, internen Entitäten und `&xxe;` lädt nichts nach und expandiert nichts. |
| MS-16 | B-12 | Nur Ganzzahlen und Werte aus festen Listen in Argumente; kein Quelltext im Filtergraph; festes Eingangsformat (RS-16). | Pflicht vor Stufe 1 | Ein Strom mit Sprachkennung `deu,movie=/etc/passwd` und Dienstname mit `;` und `[` erzeugt eine Argumentliste, die den Test der erlaubten Muster besteht. |
| MS-17 | B-09 | Alle Kopfzeilen-Werte nur schreibbar, Maskierung im Server (RS-17). | Pflicht vor Stufe 1 | Die API der Verwaltung liefert für eine Quelle mit `X-Token: abc` nur den Namen `X-Token`, nie den Wert. |
| MS-18 | B-01, B-03 | Präfixe `64:ff9b:1::/48`, `2001::/32`, `192.0.0.0/24`, `240.0.0.0/4`, `::ffff:0:0:0/96` in `netguard`; WAN-Adresse als Restlücke benennen (RS-18). | empfohlen, vor Stufe 1 | Eine Quelle mit Adresse `http://[64:ff9b:1::c0a8:101]/` wird abgewiesen. |
| MS-19 | B-19, B-28 | Vertragstest: Jede Live-Operation und jede Live-Route (Liste, Segment, Logo, Suche, Aufnahmen) antwortet ohne wirksames Recht mit 404; Kinderprofil ohne Freigabe sieht in der allgemeinen Suche keine Programmdaten (F-5). | Pflicht vor Stufe 1 | Für jede Operation mit Live-Bezug aus `api/openapi.yaml` liefert ein Profil ohne Recht 404 mit identischem Körper. |
| MS-20 | B-20 | IDOR-Matrix: eigenes Profil, anderes Profil desselben Kontos, Hauptprofil, Profil mit Altersgrenze, anderes Konto, Administrator, je für Aufnahme, Planung, Serienregel, Erinnerung, Favorit, Sitzung, Puffer. | Pflicht vor Stufe 1 (Favoriten, Sitzungen), Stufe 2 (Erinnerungen), Stufe 5 (Aufnahmen) | Jede Zelle der Matrix, die nicht erlaubt ist, antwortet 404, und jede erlaubte liefert genau die eigenen Daten. |
| MS-21 | B-11, B-06 | Bilder zuerst mit Go-Decodern prüfen; Abruf gedrosselt und erst bei aktivem Sender (RS-21). | empfohlen, vor Stufe 1 | Ein Bild mit gültigem Kopf und kaputtem Inhalt erreicht FFmpeg nicht. |
| MS-22 | B-25, B-26 | Deckel und Reserve mit Werten ab Werk, Ausweichen nur mit Schalter, rekursives Löschen nur über `os.Root` mit Marker, Namen nur aus Kennungen, 0700/0600 (RS-22). | Pflicht vor Stufe 4 (Aufnahmen: Stufe 5) | Ein Symlink im Timeshift-Ordner, der auf `/media` zeigt, bleibt beim Aufräumen unberührt, und das Ziel wird nicht gelöscht. |
| MS-23 | B-31 | FTS5-Eingabe in Wörter zerlegen und quotieren, Länge begrenzen (RS-23). | Pflicht vor Stufe 2 | Die Suche nach `"a* OR b* NEAR(` liefert eine leere Trefferliste, keinen Fehler 500. |
| MS-24 | B-11, B-04 | `govulncheck` in der CI; Release-Abgleich der FFmpeg-Fassung; Nachweis eigener Dialer der RTSP- und RTMP-Bibliotheken; RTSP nur TCP; Release ohne Test-Tags (RS-24). | Pflicht vor Stufe 1 (CI), Stufe 3 (Bibliotheken) | Eine RTSP-Quelle, deren Server per `Transport`-Kopf UDP an `192.168.1.1` verlangt, wird abgelehnt. |
| MS-25 | B-27 | Marker und Dateisystemtyp je Speicherort, wie in `04` 9 vorgesehen. | Pflicht vor Stufe 4 | Mit ausgehängtem Aufnahme-Volume entsteht keine Datei im leeren Einhängepunkt. |
| MS-26 | B-30 | Protokolleintrag (Verwaltung) für Anlegen, Ändern, Löschen von Quellen und Freigaben, ohne Geheimnisse. | Pflicht vor Stufe 1 | Nach dem Einschalten einer Freigabe steht ein Eintrag mit Administrator, Zeit und festen Zielen im Protokoll, aber kein Passwort. |
| MS-27 | B-19 | Fehlerantworten mit `lt` gleich wie «kein Recht» (404), HMAC vor dem Lesen, zufällige Kennungen (RS-19). | Pflicht vor Stufe 1 | Ein gefälschtes, ein abgelaufenes und ein fremdes Token ergeben dieselbe 404-Antwort. |
| MS-28 | B-09 | AAD `v2:` mit Tabelle, Spalte und Zeile für neue Geheimnisse (RS-20). | empfohlen, vor Stufe 1 | Ein in eine andere Zeile kopierter verschlüsselter Wert lässt sich nicht entschlüsseln. |

## 7. Neue Schlüssel und Muster für die Schwärzung

Alles hier gehört nach `server/internal/logging/logging.go` **und** in `logging_test.go` (Regel 10), dazu für
`android/` in `com.slouchcat.core.api.log.Log`, soweit die App Live-Adressen kennt.

**Abfrageparameter** (`sensitiveQuery`): `lt`, `username`, `user`, `password`, `pass`, `passwd`, `pwd`,
`auth_token`, `api_token`, `access_key`, `hash`, `jwt`. Empfehlung: statt einer festen Liste jeden Namen
schwärzen, der auf `token`, `key`, `pass`, `pwd`, `secret`, `sig` oder `auth` **endet**.

**Pfadmuster** (neue Ausdrücke wie `botPath`):

- Benutzerteil vor dem Host: `(?i)([a-z][a-z0-9+.-]*://)[^/@\s]+@` → `${1}[redacted]@`.
- Xtream: `(?i)/(live|movie|series|timeshift)/[^/\s]+/[^/\s]+/` → `/${1}/[redacted]/[redacted]/`.
- Xtream-Kurzform: `(?i)(://[^/\s]+)/[^/\s]+/[^/\s]+/(\d+)(\.(ts|m3u8))?\b` → `${1}/[redacted]/[redacted]/${2}${3}`
  (nur auf Werte anwenden, die als Adresse erkannt sind, sonst zu breit).
- Ersatznamen des Weiterleiters: `(?i)[a-z0-9]{16,}\.src\.slouchcat\.invalid` → `[redacted].src.slouchcat.invalid`.

**Attributschlüssel** (`sensitiveKeys`, Vergleich nach Endung): `cookie`, `setcookie`, `username`,
`sourceurl`, `streamurl`, `epgurl`, `playlisturl`, `manifesturl`, `segmenturl`, `keyurl`, `sourceheaders`,
`headers`, `livetoken`, `relaysecret`, `proxysecret`, `xtreampassword` (schon über `password`). **Nicht**
aufnehmen: `lt` als Endung (passt auf `default`, `result`, `alt`) und `url` als Endung (zu breit; hier gilt
stattdessen die Regel «nur `netguard.Site`»).

**Werte der Art `KindAny`:** `http.Header`, `url.Values`, `[]string` und `map[string]string` Element für Element
durch `RedactString` und die Schlüsselregel.

**Nie protokolliert,** auch nicht geschwärzt: Fehlerausgabe von FFmpeg im Weg W (nur gekürzt auf Schema und
Host, `04` 10.4), Antwortkörper von Quellen, Inhalt von M3U-Zeilen.

## 8. Fragen an Marco aus diesem Review

Für `20-offene-entscheidungen.md`; Nummern vergibt der Koordinator.

1. **Aufnahmen und Altersgrenze (RS-09).** Soll das Hauptprofil nur Aufnahmen sehen, die entstanden sind, als
   das Profil eine Altersgrenze hatte (Empfehlung (a)), oder alle Aufnahmen eines Profils, solange es eine
   Altersgrenze hat (heutiger Text, dann als benannte Ausnahme)?
2. **Weg W ohne Landlock (RS-02, RS-03).** Soll DASH und HLS mit fMP4 nur auf Hosts mit Landlock laufen
   (Empfehlung), oder auch ohne, mit dem Restrisiko, dass ein unbekannter Codepfad von FFmpeg den Netzschutz
   umgeht?
3. **Netzmodus «host» (RS-13).** Soll Multicast nur über `macvlan`/`ipvlan` angeboten werden (kleinerer Schaden,
   mehr Einrichtung), oder über «host» als benannte Ausnahme A-9 mit den vier Folgen aus RS-13?
4. **Härtung der Standard-Compose-Datei (RS-03).** `no-new-privileges`, `cap_drop: [ALL]` und `pids_limit` auch
   in `compose.yaml` für alle (Empfehlung: ja, es ändert nichts an den Funktionen), oder nur in
   `compose.live.yaml`?

## 9. Gesamturteil

**Risikostufe: HOCH** für den Entwurf in Fassung 1, **MITTEL** nach den Pflichtmassnahmen.

Der Entwurf ist als Grundlage geeignet. Er wählt die richtigen Muster (Prüfung im Dialer nach der Auflösung,
FFmpeg nur über Pipes, Token je Sitzung und Sender, 404 im Dienst `live`, verschlüsselte Geheimnisse,
streamender XML-Parser) und benennt viele Risiken selbst. Er ist aber **nicht freigabefähig**, bevor sechs
Punkte geschlossen sind:

1. die Heimnetz-Freigabe als feste Ziele statt Ja/Nein (RS-01),
2. ein Weg W ohne `CONNECT` und mit erzwungener Sperre des Netzes für FFmpeg, oder kein Weg W (RS-02),
3. Eindämmung von FFmpeg (Umgebung, Grenzen, Compose-Härtung; Landlock für Weg W) (RS-03),
4. Schutz gegen `panic` und Fuzzing aller neuen Parser im Serverprozess (RS-04),
5. jede Adresse als Geheimnis, keine Adressen in der API der Verwaltung, Xtream nur gegen den eingetragenen
   Host (RS-05),
6. Geheimnisse nur an feste Ursprünge (RS-06).

Die zwölf mittleren Befunde gehören als Regeln in `12-sicherheit.md` und sind je Stufe Pflicht (Abschnitt 6).
Die fünf stillschweigenden Lockerungen (Abschnitt 5) sind entweder zu schliessen oder als benannte Ausnahmen in
`02` aufzunehmen, mit Entscheid von Marco. Nach `12-sicherheit.md` ist ein zweites, unabhängiges
Sicherheits-Review empfohlen. Es soll vor allem M-10 (Weg W) und die feindlichen Testmengen prüfen.
