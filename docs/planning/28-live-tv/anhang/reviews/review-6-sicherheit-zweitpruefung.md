# Konzept 28 · Review 6 Sicherheit, Zweitprüfung

Stand: 29. November 2026 · Reviewer: oh-my-claudecode:security-reviewer (anderer Lauf als Review 1)

Zweites, unabhängiges Sicherheits-Review nach den Korrekturen: `12-sicherheit.md` Fassung 2, `04-architektur.md`
und `05-streaming-und-fast-zapping.md` Fassung 3, `07-datenmodell.md` und `08-api.md` Fassung 2, dazu `09`, `11`,
`15`, `02`, `20`, Regel 11 in `CLAUDE.md` und ADR 0046. Der Reviewer hat keines dieser Dokumente geschrieben und
war an Review 1 nicht beteiligt. Geprüft wird, ob RS-01 bis RS-24 und SL-1 bis SL-5 wirklich geschlossen sind, ob
`12` den Auftrag Abschnitt 10 Zeile für Zeile deckt und ob die Korrekturen neue Lücken geöffnet haben.

## 0. Kurzfassung

- **Risiko des jetzigen Stands: MITTEL** (Review 1: hoch). Die sechs hohen Befunde sind in der Spezifikation
  geschlossen bis auf **RS-03**, das **teilweise** offen bleibt: Die Vorgabe bis zur Antwort auf `12` F-1 lässt
  Live-FFmpeg ohne Dateisperre laufen. Auf einem Host ohne Landlock ist das Risiko dort weiterhin **hoch**.
- **RS-01 bis RS-24:** 20 geschlossen, 4 teilweise (RS-01, RS-03, RS-09, RS-21). RS-02 ist in der Regel
  geschlossen, hat aber neue Lücken im Bau der Sperre und im Weiterleiter (RT-02, RT-03, RT-08).
  **SL-1 bis SL-5:** 4 geschlossen, SL-5 teilweise.
- **21 neue Befunde (RT-01 bis RT-21): 0 kritisch, 1 hoch, 8 mittel, 12 niedrig.** Der hohe: RT-01 (unsichere
  Vorgabe bei F-1, dazu ein Weg zur Kontoübernahme über die Datenbank, den `12` nicht nennt). Die wichtigsten
  mittleren: die Hülle kann die Sperre in Go still verfehlen (RT-02), der Seccomp-Filter ist nicht festgelegt
  (RT-03), die gemeldete Pause umgeht die Schauzeit (RT-04), eine Freigabe bis /24 öffnet wieder das ganze
  Heimnetz (RT-05), eine entfernte Quelle läuft weiter (RT-06), das Hauptprofil kann jede PIN ersetzen (RT-07).
- **Abschnitt 10 des Auftrags** ist Zeile für Zeile einer Massnahme und einem Testfall zugeordnet. Lücken in der
  Sache: «Bestehende Account-Limits» (RT-04), «Bösartige Medienstreams» ohne Sperre (RT-01), «Schutz privater
  Aufnahmebibliotheken» (RT-07), «Keine stillschweigende Lockerung» (RT-09). Für SM-47 fehlt der Testfall in `17`.
- **Vor einer Baufreigabe von Stufe 0/1a** sind sieben Punkte nötig (Abschnitt 9).

## 1. Umfang, Methode, Einordnung

**Gelesen:** `12` ganz; `04` T-4, T-5, T-9, T-21, T-25, T-27, T-28, T-29, 5.3, 6, 10.3 bis 10.10, 12.3 (F-9 bis F-14),
14; `05` 2.1 bis 2.7, 6; `07` 4, 5, 6, 10; `08` 1, 2, 3.1, 3.4, 3.5, 3.10, 4; `09` 3, 9, 12; `10` Pause;
`11` 6; `15` Schritt 2, 2a, 3, Abschnitt 4; `02` Abschnitt 4; ADR 0046; Regel 11 im Arbeitsbaum; Auftrag
Abschnitt 10 wörtlich; Review 1; Nachprüfung Nr. 24 bis 35; `17` AK-L-230 bis AK-L-273, AK-L-327, -328.

**Am Code geprüft** (Arbeitsbaum `claude/konzept-live-tv`): `server/internal/netguard/netguard.go`,
`logging/logging.go`, `radio/token.go`, `transcode/session.go`, `transcode/images.go`, `auth/profiles.go`,
`server/migrations/00002`, `00007`, `00022` (Spalte `max_rating`), `deploy/compose.yaml`, `deploy/Dockerfile`,
`server/go.mod`.

**Nicht getan** (Auftrag: nur lesen): keine Builds, keine Tests, kein `govulncheck`, keine Läufe gegen fremde
Systeme. Kein Werkzeug hat den Kernel, Landlock oder FFmpeg ausgeführt.

**Einordnung jeder Aussage:**

- **[belegt Code]** am Quelltext mit Datei:Zeile geprüft.
- **[belegt Doku]** an einem Dokument dieses Konzepts mit Datei:Zeile geprüft.
- **[belegt Quelle]** durch die Nachprüfung an Primärquellen belegt (`anhang/nachpruefung-quellen.md`, Nr.).
- **[Annahme]** Fachkenntnis des Reviewers (Linux, Go, FFmpeg), hier nicht nachgelesen; im Bau zu belegen.

**Secrets-Scan:** Die gelesenen Konzeptdateien und der Code-Ausschnitt enthalten keine Zugangsdaten; die
Beispiele nutzen `example.*`, `192.168.1.20` und Testwerte. **Abhängigkeiten:** nicht geprüft (kein
`govulncheck`, Auftrag). Neu kämen `go-landlock` (oder `x/sys`), bei Stufe L `gortsplib` und Verwandte; SM-26
verlangt die Prüfung in der CI.

## 2. Stand der Befunde aus Review 1

Spalte «Gegenversuch»: dasselbe Angriffsszenario wie in Review 1, gegen den neuen Text gespielt.

| Befund | Schwere R1 | Stand | Wo geschlossen | Gegenversuch und Ergebnis |
|---|---|---|---|---|
| RS-01 Freigabe lokaler Ziele | hoch | **teilweise** (in 1a geschlossen, in H weitgehend) | `04` T-5, 10.9; `12` 4.1, 4.5, SM-01, SM-06; `02` A-3 | Liste nennt `127.0.0.1:8096`, `169.254.169.254`, `192.168.1.1:80`: abgewiesen (nie freigebbar, ausserhalb der Ziele). **Aber** mit einem Ziel `192.168.1.0/24` und 64 Ports erreicht derselbe Listen-Host wieder jedes Gerät des typischen Heimnetzes auf diesen Ports, auch den Router: RT-05. |
| RS-02 Weg W nicht dicht | hoch | **geschlossen in der Regel**, neue Lücken im Bau (RT-02, RT-03, RT-08) | `04` T-4; `05` 2.6; `12` 4.8, SM-08, SM-09 | `file:`, `concat:`, `subfile:`, `data:` im Manifest: von `-protocol_whitelist http,tcp` abgewiesen [belegt Quelle Nr. 24, 27, für HLS; DASH: Annahme M-11]. Absolute `tcp://fremd:80`: Umschreiben lehnt ab, Landlock sperrt Port 80. `http://fremd:P/`: nur, wenn das Umschreiben eine Adresse übersieht; dann R-3. `CONNECT`, 3xx: abgewiesen. Rest: RT-08 (Weiterleiter als authentisierter Allzweck-Proxy je Host). |
| RS-03 FFmpeg mit Zugriff auf Geheimnisse | hoch | **teilweise** | `04` T-27, 10.8; `12` 6.3, SM-22, SM-23 | Umgebung, rlimits, `no_new_privs` vor `execve`: geschlossen. Dateisperre nur «empfohlen ab 1a»; Vorgabe F-1 (a) lässt FFmpeg ohne Sperre laufen: `secret.key` lesbar, `slouchcat.db` innerhalb der Dateigrösse überschreibbar (RT-01). |
| RS-04 Paket beendet den Server | hoch | **geschlossen** | `04` T-27, 10.8; `05` 2.7; `12` 6.4, SM-25 | Indexfehler im TS-Parser: `recover` an jeder Live-Go-Routine über eine gemeinsame Startfunktion (Lint). Rest: fatale Laufzeitfehler fängt `recover` nicht (RT-15). |
| RS-05 Adressen im Klartext | hoch | **geschlossen** | `04` T-28, 10.5; `07` 6.1; `12` 5.1, SM-14, SM-16, SM-18, SM-20 | Xtream-M3U mit `/live/u/p/1.ts`: nur `stream_id` gespeichert; jede Adresse `v2:`-verschlüsselt, Klartext nur `site`; Sicherung ohne Geheimnisse mit `v0:`; Bytesuche als Test. `server_info.url`: nie verwendet. |
| RS-06 Kopfzeilen an fremde Hosts | hoch | **geschlossen** (Wortlaut in `15`: RT-14) | `04` 10.5; `05` 2.4; `12` 5.2, 5.5, SM-04, SM-15 | Sender auf `sammler.example`: nur `User-Agent` und `Accept`. `#EXTHTTP:{"Cookie":…}`: verworfen. `https`→`http`: Abbruch immer. |
| RS-07 Schwärzung | mittel | geschlossen | `12` 5.4, SM-17 | Neue Abfrage-, Pfad- und Schlüsselmuster, `redactAny` für Sammelwerte, Lint «nur `netguard.Site`», Gegentest `default=`. [belegt Code: heutige Lücken `logging.go:75`, `:185-198`] |
| RS-08 Token `lt` | mittel | geschlossen (Rest RT-10) | `04` T-21, 10.3; `08` 4.2; `12` 9.1, SM-35 | Erbeutetes Token ohne Herzschlag: tot nach 60 s. HMAC vor Zustand; Bindung an Sitzung und Einstellen; 12 h. Rest: VOD-Liste einer Aufnahme wird nur einmal geladen (RT-10). |
| RS-09 Hauptprofil und Altersgrenze | mittel | **teilweise** | `07` 4.3 (`owner_age_limited` und «heute noch»); `04` F-10 offen | Grenze kurz setzen: alte Aufnahmen bleiben unsichtbar [belegt Doku `07:1552-1558`]. **Aber** das Hauptprofil kann die PIN jedes Profils ohne Kenntnis ersetzen und hineinwechseln (RT-07). |
| RS-10 Umschalten als Hebel | mittel | geschlossen | `04` 5.3, 8.5; `12` 9.5, SM-39 | 20 Sender in 10 s: höchstens ein Strom in Karenz, Karenz weicht zuerst, Deckel 8/20. |
| RS-11 Verbindungsvorrat | mittel | geschlossen | `04` 10.9; `12` 4.2, SM-02 | Transport je Quelle und Regelstand; laufende Verbindungen enden beim Entzug (SM-47; Test fehlt, RT-21). |
| RS-12 UDP, RTP, Multicast | mittel | geschlossen (Rest R-2 ehrlich) | `05` 2.3; `12` 4.10, SM-10 | Bindung an Schnittstelle, SSM, nichts senden, Flutgrenze, Text V-3 mit Zusatz. |
| RS-13 Netzmodus «host» | mittel | geschlossen | `02` A-9; `04` 10.6; `12` SM-11 | Vier Folgen im Wortlaut, Weg W gesperrt, Adressen des Hosts gesperrt; F-11 offen. |
| RS-14 Grenzen M3U, Xtream, HLS | mittel | geschlossen | `05` 2.3; `12` 7.3, 7.4, SM-07, SM-27 | Zieldauer 0, A→B→A, 10-MB-Zeile, Tiefe 1'000: stabile Fehler. |
| RS-15 XMLTV | mittel | geschlossen | `09` 3.3, 12; `12` 7.1, 7.2, SM-28, SM-29 | 2-GB-`<desc>`, Tiefe 100'000 mit `Token()`, zip mit zwei Einträgen: abgebrochen [belegt Quelle Nr. 31, 32]. |
| RS-16 Argumente, Filtergraph | mittel | geschlossen | `04` 10.8; `12` 6.1, SM-21 | `deu,movie=/etc/passwd` gelangt nicht in Argumente (nur Zahlen und Listenwerte). |
| RS-17 Kopfzeilen nach Namen maskiert | mittel | geschlossen | `15` Kopfzeilen-Zeile (Name und «gesetzt»); `12` SM-16 | `X-Token: abc`: API liefert nur den Namen. |
| RS-18 Adressfilter | niedrig | geschlossen (R-4 ehrlich) | `12` 4.4, SM-03 | NAT64 lokal, Teredo: abgewiesen. |
| RS-19 401/404, Zeit, Kennungen | niedrig | geschlossen (Rest RT-12) | `08` 3.1, 4.2; `12` 9.3 | Token falsch und Recht fehlt: gleiche 404. Rest: weitere Statuscodes vor der Rechteprüfung (RT-12). |
| RS-20 AAD ohne Zeile | niedrig | geschlossen | `07` 6.2; `12` 5.1, SM-14 | Kopierter `v2:`-Wert nicht entschlüsselbar. (`04` 10.5 nennt es noch «Empfehlung», `07` und `12` legen es fest.) |
| RS-21 Logos durch FFmpeg | niedrig | **teilweise** | `12` 4.9, SM-13 | Go-Decoder zuerst ist Testfall von SM-13 (Pflicht 1a). **Aber** `12` 6.2 nennt den Bildweg falsch («pipe, gebaut»), und das Bild-FFmpeg liegt ausserhalb der Hülle (RT-17). |
| RS-22 Speicherorte | mittel | geschlossen | `04` T-25, 10.10; `12` 8, SM-31 bis SM-34 | Symlink auf `/media`, `../../x` im Titel, leerer Einhängepunkt: abgewiesen. |
| RS-23 Programmsuche | niedrig | geschlossen | `12` 7.5, SM-30 | `"a* OR b* NEAR(`: leere Liste. |
| RS-24 Abhängigkeiten, Test-Dialer | niedrig | geschlossen | `12` 4.11, SM-12, SM-26 | Dialer, der alles ablehnt; Release ohne Test-Tag; `govulncheck`. |

| Lockerung | Stand | Stelle | Bemerkung |
|---|---|---|---|
| SL-1 Freigabe für alle Anfragen | geschlossen | `02` A-3, `12` 4.5 | Rest RT-05 (Netzgrösse) |
| SL-2 Weg W mit Tunnel | geschlossen | `05` 2.6, SM-08 | R-3 benannt; RT-08 ergänzt es |
| SL-3 Maskierung nach Namen | geschlossen | `15` Schritt 2, `12` SM-16 | – |
| SL-4 Netzmodus «host» | geschlossen | `02` A-9 | – |
| SL-5 Hauptprofil nach gesetzter Grenze | **teilweise** | `07` 4.3, `04` F-10 | Datenmodell setzt (b) um; der PIN-Weg (RT-07) und die Konto-Altersgrenze sind nicht benannt |

## 3. Gezielte Gegenversuche

### 3.1 Feste Ziele im Heimnetz (Stufe H)

| Versuch | Ergebnis | Beleg |
|---|---|---|
| Bindung an Adresse und Port | hält: Prüfung im `Dialer.Control` an der angewählten Adresse, Ziel ist eine Adresse, nie ein Name | [belegt Code] `netguard.go:73-82` (Kern); [belegt Doku] `12:311-312`, `04:1056-1057` |
| Hostname einer Liste, der auf das freigegebene Gerät zeigt | erlaubt (liegt im Ziel), aber ohne Zugangsdaten, weil der Name kein fester Ursprung ist | `12:537`, R-6 |
| Weiterleitung vom freigegebenen Gerät auf ein anderes lokales Ziel | abgewiesen; auf öffentlich: normaler öffentlicher Abruf | `12:391` |
| Unter-URLs aus Manifesten (Unterliste, Segment, `EXT-X-KEY`, `EXT-X-MAP`) | je eigene Anfrage über den Dialer derselben Quelle | `12:423`, `05:106` |
| Liste mit 500 internen Adressen | keine Verbindung, ein Zustand, Sammelzeile nach Netzen | `12:393`, SM-06, AK-L-235 |
| **Liste aus dem Internet, Quelle mit Ziel `192.168.1.0/24`, Ports 80–143** | **erlaubt**: der Listen-Host spricht jedes Gerät im /24 auf 64 Ports an, ohne Zutun eines Menschen (Prüfung ruhender Sender, Vorladen, Aufnahme) | `02:125`, `12:387`, `15:334`; RT-05 |
| Docker-Gateway (`172.17.0.1`, Gateway des Compose-Netzes) oder Nachbarcontainer im Bridge-Netz | **freigebbar** (privat, nicht «eigene Adresse des Containers»); dort lauschen Dienste des Hosts und anderer Container | `12:370`; RT-05 |
| Fehlermeldungen als Orakel | für nicht freigegebene Ziele ein Zustand ohne Verbindungsversuch; Zustände und Zeiten nur für öffentliche und freigegebene | `12:394`, `15:363-368` |
| Entzug eines Ziels bei laufendem Strom | Verbindungen enden nach Bestätigung innert 5 s (SM-47); **Testfall fehlt in `17`** (AK-L-231 prüft nur ruhende Verbindungen) | `12:323-330`, `17:638`; RT-21 |
| **Entfernen der ganzen Quelle bei laufendem Strom** | **nicht geregelt**: Ziele und Ursprünge gehen mit der Zeile weg, der Empfänger läuft nach T-29 weiter | `07:1908-1922`, `04:481-484`; RT-06 |

### 3.2 Weg W

**Whitelist:** `-protocol_whitelist http,tcp`, Eingang `-f hls` oder `-f dash`, Ausgabe `-c copy -f mpegts pipe:1`
[belegt Doku `05:202`, `12:439`]. `tcp` ist als untere Schicht nötig [belegt Quelle Nr. 24]. Der HLS-Leser nimmt
in Listen nur `http`, `https`, `file`, `data`, mit der Liste bleibt `http` [belegt Quelle Nr. 27].

| Kann ein Manifest FFmpeg führen zu … | Antwort |
|---|---|
| einem fremden Host | nur über eine Adresse, die das Umschreiben übersieht; dann nur auf Port `P` (Landlock), sonst nie. R-3. |
| `file:`, `concat:`, `subfile:`, `data:` | nein: nicht in der Liste [belegt Quelle Nr. 24]; der HLS-Leser lehnt `concat`/`subfile` schon vorher ab [belegt Quelle Nr. 27]. Für den DASH-Leser ist das Annahme (M-11). |
| `tcp://` direkt | Die Liste enthält `tcp`. Der HLS-Leser nimmt `tcp://` in Listen nicht an [belegt Quelle Nr. 27]; für DASH Annahme (M-11). Schutz: Umschreiben lehnt ab, Landlock nur Port `P`. |
| `https`, `tls`, `httpproxy`, `crypto` | nein, nicht in der Liste; `http_proxy`/`no_proxy` nie in der Umgebung [belegt Quelle Nr. 25]. |
| Weiterleitung | nein: der Weiterleiter folgt selbst, gibt nie 3xx weiter (`05:200`). |

**«Erst nach Messung»** ist durchgehend festgehalten: `04:80` (T-4), `05:65`, `05:187`, `12:63`, `13:752`,
`13:769`, `17:952`, `17:968`, `18:170`, `20:935`. **Nicht** im ADR: ADR 0046 Zeile 54 nennt Weg W als «einzige,
noch nicht gebaute Ausnahme», ohne die Bedingungen M-10 und Sperre (RT-09).

### 3.3 FFmpeg-Eindämmung

| Frage | Antwort | Beleg |
|---|---|---|
| Landlock hart? | Für Weg W ja (Start verweigert). Für Umwandlung und Probe **nein**: Vorgabe F-1 (a) läuft ohne Sperre weiter, ausgewiesen | `12:646`, `12:1195`; RT-01 |
| Seccomp? | «immer», aber nur als Satz: Art des Filters, Architekturprüfung, `io_uring`, Verhalten, wenn er nicht setzbar ist, sind offen | `12:455-459`, `12:645`; RT-03 |
| Eigene Umgebung, rlimits vor dem Start? | ja, über `execve` aus der Hülle; heute erbt FFmpeg alles | [belegt Code] `transcode/session.go:649`, `:663` (kein `cmd.Env`); `12:631-643` |
| Aussage zur Go-Bibliothek | richtig: `BestEffort()` fällt still zurück und ist verboten [belegt Quelle Nr. 35]. **Fehlt:** Vor ABI 8 (`TSYNC`, Nr. 34) wirkt `landlock_restrict_self` nur auf den aufrufenden Thread; die Hülle muss den Thread bis `execve` festhalten | RT-02 |
| Bestehende FFmpeg-Aufrufe der Dateiwiedergabe | bleiben ohne Hülle, erben weiter alle `SLOUCHCAT_*`-Schlüssel und können `secret.key` lesen, der jetzt auch die Anbieter-Zugänge schützt | [belegt Code] `session.go:649`, `backend.go:314`, `images.go:302`; RT-18 |
| Ältere Kernel, Container ohne Landlock | Weg W fällt aus (richtig). Umwandlung läuft **ungeschützt** weiter (F-1 a); Seccomp wäre trotzdem setzbar, aber das steht nicht als Pflicht da | RT-01, RT-03 |

### 3.4 Zugangsdaten

| Frage | Antwort | Beleg |
|---|---|---|
| Jede Adresse verschlüsselt? | ja, auch Logo-Adressen, User-Agent und `Referer` aus Listen | `07:1640-1654` |
| `url_fp` und `site` ohne Geheimnis? | `url_fp` ist HMAC mit eigenem Zweckschlüssel; `site` = Schema und Host mit Port, ohne Benutzerteil | `07:1664-1666`; [belegt Code] `netguard.go:132-138`. Rest: ein Token im Hostnamen (`<token>.cdn.example`) stünde in `site` (niedrig, RT-14) |
| Nie an Clients? | ja; nur `site`, Namen der Kopfzeilen, `hasCredentials` | `08:79-83`, SM-16 |
| Redaction-Muster | vollständig für die bekannten Formen; Gegentest gegen Überschwärzen | `12:564-579` |
| Sicherung mit Platzhalter | `v0:` statt `NULL`, CHECK-Regeln halten, Bytesuche als Test; sichere Vorgabe F-4 (a) | `07:1674-1679`, SM-18, AK-L-247 |
| Header nur an feste Ursprünge | ja in `04`, `05`, `12` (Schema, Host, Port); `15` spricht von «Hosts» (RT-14) | `15:324` |
| `https`→`http` | bricht immer ab (Weiterleitung). Ein Listeneintrag `http://` auf demselben Host ist keine Weiterleitung: geschützt nur, wenn der Ursprungsvergleich das Schema einschliesst (RT-14) | `04:948`, `12:407` |

## 4. Neue Befunde

Schwere nach Schaden × Ausnutzbarkeit × Reichweite, wie in Review 1: **kritisch**, **hoch**, **mittel**,
**niedrig**. «Pflicht vor» nennt die Stufe nach `18`.

### RT-01 · Vorgabe bei F-1 ist die unsichere Variante; der Schaden ist grösser als R-1 sagt · **hoch**

- **Stelle:** `12` 6.3 Zeile «Hart oder ausgewiesen» (`12:646`), F-1 mit Vorgabe (a) (`12:1190-1200`), R-1
  (`12:1154`); `04` T-27 (`04:103`), 10.8 (`04:1023`); `20` E-28-04.
- **Szenario:** Ein Anbieter, ein CDN- oder Segment-Host (AK-1, AK-2) liefert einen Strom, der eine Speicherlücke
  in einem Decoder von FFmpeg 7.1.4 auslöst (MPEG-2, HEVC, DVB-Untertitel, Teletext). Auf einem Host ohne
  Landlock (älterer Kernel, LXC- oder Docker-Profil ohne die Aufrufe; CT 190 ist nicht gemessen, M-25) läuft die
  Umwandlung nach F-1 (a) trotzdem. Der Code im FFmpeg-Prozess hat die UID des Servers und liest `secret.key`.
  `12` R-1 nennt nur das Lesen. **Dazu kommt das Schreiben:** `RLIMIT_FSIZE 0` verhindert nur das Verlängern einer
  Datei, nicht das Überschreiben innerhalb ihrer Grösse [Annahme, Semantik von `setrlimit`]. Der Angreifer
  überschreibt in `slouchcat.db` den Passwort-Hash des Eigentümers mit einem Hash gleicher Länge, den er kennt, und
  meldet sich über die öffentliche Adresse des Servers an (der Betrieb hat eine, Cloudflare-Proxy). Das ist
  Kontoübernahme mit Administratorrechten. Selbst ohne Netz (Seccomp sperrt Sockets, wenn er greift) bleibt so ein
  Weg nach aussen: über das Konto, nicht über den Prozess.
- **Beleg:** [belegt Doku] `12:644` «empfohlen ab 1a … ob Pflicht ab 1a: F-1», `12:646` «läuft FFmpeg nach der
  Vorgabe von F-1 (a) ohne Sperre weiter». [belegt Code] FFmpeg läuft heute mit der UID des Servers,
  `deploy/compose.yaml:29`, `deploy/Dockerfile:136`.
- **Korrektur (Pflicht vor 1a):**
  1. Vorgabe bis zur Antwort: **(b) nur mit Sperre**, also geschlossen ausfallen. Umwandlung ohne Dateisperre nur
     als dritte Variante **(c)**: ausdrücklicher Schalter des Betreibers (Umgebungsvariable oder Verwaltung mit
     Bestätigung und Protokolleintrag), ab Werk aus, mit dauerhaftem Hinweis. Das ist dann eine benannte Lockerung
     und gehört als A-10 nach `02`.
  2. R-1 um das Schreiben ergänzen (Datenbank, `secret.key`, Aufnahmen anderer Profile innerhalb ihrer Grösse).
  3. Seccomp bleibt **auch ohne Landlock Pflicht**; ist Seccomp nicht setzbar, startet kein Live-FFmpeg (RT-03).
  4. M-25 vor der Baufreigabe von 1a auf CT 190 messen, nicht nur «früh».

```go
// BAD (12 6.3 with F-1 (a)): the live transcoder runs on hostile data without a file sandbox.
if !ll.Complete() {
	status.Set(ContainmentUnavailable)
}
return unix.Exec(ffmpeg, args, env)

// GOOD: fail closed; running without the file sandbox is an explicit, logged opt-in of the operator.
if !ll.Complete() && !cfg.AllowUncontainedTranscode {
	return fmt.Errorf("%w: %s", ErrContainmentUnavailable, ll.Reason())
}
```

### RT-02 · Die Hülle kann die Sperre in Go still verfehlen (Thread-Bindung) · **mittel**

- **Stelle:** `12` 6.3 («Hülle … ersetzt sich dann mit `execve`», `12:631-637`), 4.8 Punkt 4 (`12:464-468`);
  `04` 10.8 (`04:1021`, `:1027-1032`); O-2.
- **Szenario:** Die Hülle ist ein Go-Programm mit mehreren OS-Threads. `prctl(PR_SET_NO_NEW_PRIVS)` gilt je
  Thread, `landlock_restrict_self` vor ABI 8 nur für den aufrufenden Thread (`TSYNC` gibt es erst ab ABI 8,
  [belegt Quelle Nr. 34]), ein Seccomp-Filter ohne `SECCOMP_FILTER_FLAG_TSYNC` ebenfalls nur je Thread
  [Annahme]. Der Go-Scheduler kann die Goroutine der Hülle zwischen dem Setzen der Sperre und `syscall.Exec` auf
  einen anderen Thread verschieben. `execve` aus diesem Thread übernimmt dessen Rechte: FFmpeg läuft **ohne**
  Sperre, obwohl die Hülle «vollständig gesetzt» geprüft hat. Der Fehler tritt zufällig auf; der Testfall von SM-22
  (`/proc/<pid>/status`, ein Öffnen von `secret.key`) fängt ihn nur, wenn er zufällig eintritt.
- **Beleg:** [belegt Quelle] Nr. 34 (ABI 8 `TSYNC`); [belegt Code] Server ohne cgo (`deploy/Dockerfile:60`), also
  wäre `syscall.AllThreadsSyscall` verfügbar; [Annahme] Verhalten des Go-Schedulers und der Kernel-Aufrufe.
- **Korrektur (Pflicht vor 1a):** Die Hülle hält den Thread mit `runtime.LockOSThread()` vom ersten Aufruf bis
  `execve` und gibt ihn nie frei; alternativ alle Threads über `AllThreadsSyscall` beziehungsweise `TSYNC`. Die
  Prüfung «vollständig gesetzt» liest danach `/proc/thread-self/status` (`NoNewPrivs: 1`, `Seccomp: 2`) **desselben**
  Threads. Testfall: 1'000 Starts unter Last, jeder FFmpeg zeigt in `/proc/<pid>/status` `NoNewPrivs: 1` und
  `Seccomp: 2`, und jeder scheitert an `/config/secret.key`.

```go
// GOOD: everything that restricts the process happens on the one thread that calls execve.
func sandboxExec(s Spec) error {
	runtime.LockOSThread() // never unlocked: this thread becomes ffmpeg
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	if err := applyRlimits(s); err != nil {
		return err
	}
	if err := applyLandlock(s); err != nil { // landlock_restrict_self on this thread
		return err
	}
	if err := applySeccomp(s); err != nil { // filter on this thread
		return err
	}
	return unix.Exec(s.FFmpeg, s.Args, s.Env) // same thread: the new image inherits both
}
```

### RT-03 · Der Seccomp-Filter ist nur ein Satz; seine Fallstricke sind nicht festgelegt · **mittel**

- **Stelle:** `12` 4.8 Punkt 2 (`12:455-460`), 6.3 «Sperre des Netzes» (`12:645`), SM-09, SM-22.
- **Szenario:** Vor ABI 10 ist Seccomp die **einzige** Sperre gegen UDP, und ohne Landlock die einzige gegen jedes
  Netz. Ein Filter, der nur `socket()` nach Familie prüft, lässt nach heutiger Kenntnis durch [Annahme]:
  (a) `io_uring` (`IORING_OP_SOCKET`, `IORING_OP_SENDMSG`) umgeht die Prüfung von `socket()` und `sendto()`;
  (b) Systemaufrufe der 32-Bit- und x32-Schnittstelle (`int 0x80`, `socketcall`) haben andere Nummern, wenn der
  Filter die Architektur nicht prüft; (c) `type` mit `SOCK_NONBLOCK|SOCK_CLOEXEC` fällt aus einem Vergleich auf
  Gleichheit; (d) `AF_UNIX` bleibt offen: Landlock regelt `connect()` auf Unix-Sockets mit Pfad erst ab ABI 9
  (`RESOLVE_UNIX`, [belegt Quelle Nr. 34]) und abstrakte Sockets erst mit den Bereichen ab ABI 6. Dazu: Weg W
  regelt in Landlock nur `CONNECT_TCP`; `BIND_TCP` ist nicht genannt. Und offen ist, was gilt, wenn Seccomp gar
  nicht setzbar ist (F-1 (a) liest sich wie «ohne Sperre weiter»).
- **Beleg:** [belegt Doku] `12:456-459` («`socket()` nur für `AF_INET`/`AF_INET6` mit `SOCK_STREAM` … sowie für
  `AF_UNIX`»), `12:645` («Umwandlung und Probe: … Seccomp: `socket()` nur `AF_UNIX`»).
- **Korrektur (Pflicht vor 1a für Umwandlung und Probe, vor W für den Empfänger):** Filter als **Erlaubnisliste**
  mit Vorgabe `SECCOMP_RET_KILL_PROCESS`; erste Regel prüft `AUDIT_ARCH_X86_64` (bzw. `AARCH64`) und tötet sonst;
  `io_uring_setup`, `io_uring_enter`, `io_uring_register`, `socketcall`, `ptrace`, `process_vm_readv`,
  `process_vm_writev`, `bpf`, `userfaultfd`, `perf_event_open` nicht erlaubt; `socket()` mit maskiertem `type`.
  Für Umwandlung und Probe **kein** `socket()`, auch nicht `AF_UNIX`, solange M-25 keinen Bedarf zeigt. Landlock
  ab ABI 6 mit `LANDLOCK_SCOPE_SIGNAL` und `LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET`; Weg W mit `BIND_TCP` ohne Regel.
  Ist Seccomp nicht setzbar, startet **kein** Live-FFmpeg, unabhängig von F-1. Testfälle: je ein Versuch über
  `io_uring`, über `int 0x80` und mit `SOCK_DGRAM|SOCK_NONBLOCK` scheitert.

### RT-04 · Die gemeldete Pause umgeht die tägliche Schauzeit (Regel 12, Account-Limits) · **mittel**

- **Stelle:** `04` 10.3 «Schauzeit und Pause» (`04:896`): «Segmente, die ein Player während einer **gemeldeten**
  Pause noch als Vorrat holt, zählen nicht.» `08` `LiveHeartbeatRequest.playing` (`08:602-606`); `10` «Lange
  Pause» (`10-timeshift.md:136`).
- **Szenario:** Ein Profil mit Tageslimit (meist ein Kind, AK-8) oder ein angepasster Client meldet im Herzschlag
  immer `playing: false` und holt dabei Segmente. Nichts zählt, die Schauzeit ist ausgehebelt. Das Feld kommt vom
  Client; die Regel macht damit eine Grenze des Kontos von einer Angabe des Clients abhängig. Das widerspricht
  Regel 12 («Entscheid im Server») und Auftrag 10.6 «Bestehende Account-Limits». Im Browser genügt das Ändern
  einer Zeile im Skript.
- **Beleg:** [belegt Doku] `04:896`, `08:605-606` («Watch time counts the duration of delivered segments while the
  last heartbeat said true»).
- **Korrektur (Pflicht vor 1a):** Gezählt wird jedes ausgelieferte, verschiedene Segment. Eine gemeldete Pause
  gewährt höchstens einen festen Vorrat (Vorschlag: so viele Segmente wie der Vorlauf des Players, `05` 5.3,
  höchstens 60 s je Pause-Meldung); was darüber geholt wird, zählt. Testfall: ein Client meldet dauerhaft Pause und
  holt 30 Minuten Segmente; die Schauzeit steigt um mindestens 29 Minuten.

```go
// BAD (04 10.3): the client's pause flag switches accounting off.
if hb.Playing {
	s.watch.Add(seg.Duration)
}

// GOOD: a reported pause only buys a bounded prefetch allowance.
if !s.playing && s.pausedPrefetch+seg.Duration <= maxPausedPrefetch {
	s.pausedPrefetch += seg.Duration
	return
}
s.watch.Add(seg.Duration)
```

### RT-05 · Eine Freigabe bis /24 und 64 Ports öffnet wieder das ganze Heimnetz · **mittel**

- **Stelle:** `02` A-3 (`02:125`), `12` 4.5 «Form» (`12:387`), `04` 10.9 (`04:1056`), `15` Schritt 2a
  (`15:334-335`); Liste «nie freigebbar» (`12:359-372`).
- **Szenario:** Die meisten Haushalte haben genau ein /24. Gibt der Administrator für eine Quelle «kleines Netz»
  `192.168.1.0/24` mit Ports 80 bis 143 frei, weil zwei Geräte darin stehen, kann der Listen-Host dieser Quelle
  (AK-1) wieder jedes Gerät auf diesen Ports ansprechen, auch den Router, und zwar selbsttätig über die Prüfung
  ruhender Sender und das Vorladen. Das ist die blinde SSRF aus RS-01, nur auf einen Portbereich begrenzt. Gleiches
  gilt, wenn die **Liste aus dem Internet** kommt und nur einzelne Sender auf ein lokales Gerät zeigen: Die Ziele
  stehen dann dem Internet-Host offen, nicht nur dem Gerät (R-6 nennt nur das Gerät). Dazu sind das Gateway des
  Docker-Netzes und das Bridge-Netz der Container (`172.16.0.0/12`) privat und damit freigebbar; dort lauschen
  Dienste des Hosts und anderer Container.
- **Beleg:** [belegt Doku] `12:387` («Adresse oder Netz (höchstens /24 bzw. /64) und Port oder Portbereich
  (höchstens 64 Ports)»), `12:370` (nie freigebbar nur «eigene Adressen des Containers»).
- **Korrektur (Pflicht vor H):** Ab Werk nur Einzeladresse (/32, /128) und Einzelport. Ein Netz oder Portbereich
  nur mit zweiter Bestätigung, die zählt, was es umfasst («254 Adressen, darunter das Standard-Gateway
  192.168.1.1»), und nie, wenn das Netz das Standard-Gateway des Heimnetzes oder des Containers enthält. Netze der
  eigenen Container-Schnittstellen als Ganzes nie freigebbar; das Gateway des Container-Netzes nur als Einzelziel
  mit Einzelport und eigener Warnung. Kommt die Liste der Quelle nicht selbst von einem festen Ziel, nennt V-3
  wörtlich, dass der Anbieter der Liste die Ziele ansprechen kann. SM-01 bekommt den Testfall «Netz mit Gateway
  wird abgewiesen».

### RT-06 · Eine entfernte Quelle läuft mit gelöschten Zielen und Zugangsdaten weiter · **mittel**

- **Stelle:** `07` 10.3 (`07:1908-1922`: Quelle löschen, «Kopfzeilen, Ursprünge, Ziele … gehen mit»), `07` 10.4
  (`07:1928`, `:1930`), `04` 5.3 (`04:481-484`), T-29 (`04:105`), SM-47 (`12:1009`).
- **Szenario:** Der Administrator entfernt eine Quelle, weil ihr Zugang kompromittiert ist, weil der Anbieter ihn
  verloren hat oder weil ein freigegebenes Gerät nicht mehr vertraut ist. Die Sender werden deaktiviert oder
  verschwinden; nach T-29 laufen Wiedergaben weiter, «bis der Zuschauer geht», Aufnahmen bis zu ihrem Ende. Der
  Empfänger hält dafür die entschlüsselte Adresse, Kopfzeilen und den Transport mit dem **alten** Regelstand im
  Speicher. Ob er nach einem Abbruch wieder verbindet, ist nirgends gesagt. SM-47 beendet Verbindungen nur beim
  Entzug **eines Ziels**, nicht beim Entfernen der Quelle; die Ziele verschwinden hier mit der Zeile ohne Entzug.
  Ein Zuschauer mit Herzschlag hält den Strom unbegrenzt (RT-11).
- **Beleg:** [belegt Doku] `07:1917`, `04:481-484`, `12:837` («Sender deaktiviert: kein Entzug»).
- **Korrektur (Pflicht vor 1a):** Entfernen einer Quelle, Ändern ihrer Adresse, ihrer Zugangsdaten, ihrer
  Ursprünge oder ihrer Ziele beendet alle Empfänger dieser Quelle **sofort** (Dialog mit Zahl der Betroffenen
  wie «sofort beenden»); Aufnahmen bekommen eine Lücke und wechseln, wenn vorhanden, auf eine Ersatzquelle. Der
  Speicher des Empfängers gibt die entschlüsselten Werte frei. Ein Empfänger verbindet nie mit einem Regelstand, der
  nicht mehr in der Datenbank steht (Versionsnummer der Quelle bei jeder Verbindung prüfen). Testfall: Quelle
  entfernen bei 1 Zuschauer und 1 Aufnahme; innert 5 s keine Verbindung mehr zur Quelle.

### RT-07 · Das Hauptprofil kann jede PIN ersetzen; «ganz privat» hält nicht · **mittel**

- **Stelle:** `02` A-6 (`02:128`: «Profile ohne Altersgrenze bleiben ganz privat»), `12` 9.3 IDOR-Matrix
  (`12:885`, `:887`), 9.4 «PIN-Profile» (`12:896`), `03` DR-05; R-Liste (`12:1150-1164`).
- **Szenario:** (1) Das Hauptprofil setzt oder entfernt die PIN eines erwachsenen Profils **ohne** die alte zu
  kennen, wechselt hinein und sieht, spielt und löscht dessen private Aufnahmen. Danach setzt es eine neue PIN,
  und das Profil merkt erst beim nächsten Wechsel, dass seine PIN nicht mehr gilt. Das ist RS-09 auf anderem Weg;
  F-10 (b) schützt davor nicht. (2) Profile **ohne** PIN öffnet jeder, der die Anmeldung des Kontos hat (Profilwahl
  ohne Geheimnis); die Zeile «anderes Profil desselben Kontos: 404» der Matrix schützt die API, nicht die Daten.
  (3) E-Mail-Meldungen gehen an die Adresse des Kontos (Profile haben keine eigene); DR-05 schwärzt Titel nur für
  PIN-Profile. (4) Eine **Konto-Altersgrenze** macht jedes Profil des Kontos zu einem «Profil mit Altersgrenze»
  (`07` 4.4); das Hauptprofil eines solchen Kontos sieht künftig alle Aufnahmen der anderen. Diese Grenze setzt der
  Administrator, nicht das Hauptprofil: ein zweiter Akteur erweitert A-6.
- **Beleg:** [belegt Code] `server/internal/auth/profiles.go:464-484` (`SetProfilePIN`: nur `requirePrimary`),
  `:488-497` (`RemoveProfilePIN`: dito, ohne alte PIN, ohne Meldung); `server/migrations/00002_accounts_sessions.sql:25`
  («Profiles carry no e-mail»). [belegt Doku] `07:1573`.
- **Korrektur (vor Stufe 5; Entscheid an Marco):** Ehrlich benennen als Restrisiko R-12 und im Wortlaut von A-6:
  «Privat gegenüber anderen Profilen des Kontos sind Aufnahmen nur mit PIN; das Hauptprofil kann eine PIN ersetzen.»
  Als Massnahme vorschlagen: Ersetzt oder entfernt das Hauptprofil die PIN eines Profils ohne Altersgrenze, beendet
  der Server dessen Sitzungen und legt einen Eintrag in dessen Glocke («Deine PIN wurde vom Hauptprofil geändert»);
  E-Mail-Meldungen zu Aufnahmen nennen ohne ausdrückliche Wahl des Profils keinen Titel. Die Konto-Altersgrenze in
  A-6 ausdrücklich nennen (Frage `07` F-1 mit dieser Folge stellen).

### RT-08 · Der Weiterleiter ist je Host ein authentisierter Allzweck-Proxy · **mittel** (Stufe W)

- **Stelle:** `05` 2.6 «Ersatzpfad» (`05:197`: «Je Host … einen eigenen zufälligen Pfadanfang»), «Zugang zum
  Weiterleiter» (`05:205`); `12` 4.8 «Ersatzpfade» (`12:440`); R-3 (`12:1156`).
- **Szenario:** Weil DASH-Vorlagen (`$Number$`, `$Time$`) Pfade erst in FFmpeg entstehen lassen, bildet der
  Weiterleiter einen Pfadanfang auf einen Host ab und hängt den Rest an, den FFmpeg schickt. Ein gekaperter
  FFmpeg-Empfänger (R-3) fragt dann beliebige Pfade auf jedem Host der Quelle ab, auch auf einem **festen
  Ursprung** mit `Authorization` oder Cookie: Der Weiterleiter setzt die Geheimnisse ein, FFmpeg liest die Antwort
  (Kontodaten, Verwaltungsseiten des Anbieters) und trägt sie über Port `P` hinaus. R-3 sagt dagegen «nur den Strom
  und die Ersatzpfade seiner eigenen Quelle». Zweitens: Baut der Weiterleiter die Zieladresse mit
  `url.Parse(base + rest)` oder `ResolveReference`, macht ein Rest `//fremd.example/x` daraus einen anderen Host
  (netzwerkpfad-relativer Verweis) [Annahme zur Bauweise].
- **Beleg:** [belegt Doku] `05:197`, `05:205`, `12:1156`.
- **Korrektur (Pflicht vor W):** Der Weiterleiter beantwortet nur Adressen, die er selbst umgeschrieben hat: genaue
  Adressen aus Listen und Manifesten und für Vorlagen nur Pfade, die dem Muster der Vorlage entsprechen (Platzhalter
  nur Ziffern). Zieladresse immer als `url.URL{Scheme, Host}` aus der Tabelle plus geprüftem Pfad, nie durch
  Zusammensetzen von Text. Antworten an FFmpeg nur mit dem Inhaltstyp, den der Verweis erwarten lässt (Liste,
  Segment, Kopfstück). R-3 ergänzen. M-10 bekommt den Fall «FFmpeg fragt einen nicht vergebenen Pfad unter einem
  gültigen Anfang ab: 404».

```go
// GOOD: only rewritten references are served; the target never comes from string concatenation.
ref, ok := w.refs.Lookup(prefix, rest) // exact URL, or a template match with digits only
if !ok {
	http.NotFound(rw, r)
	return
}
target := &url.URL{Scheme: ref.Scheme, Host: ref.Host, Path: ref.Path, RawQuery: ref.RawQuery}
```

### RT-09 · Drei Abweichungen stehen nicht in der Liste der benannten Ausnahmen · **niedrig**

- **Stelle:** `02` Abschnitt 4 (`02:116-131`, A-1 bis A-9); `12` 1.1 (`12:63`, Zeile «ADR 0025, Weg W») und SM-43
  (`12:1004`: «benannte Ausnahmen A-1 bis A-9 in Regeln, Spec und ADR»); ADR 0046 Zeile 54; `04` 10.8
  (`04:1027-1032`, Zusatz zu Regel 3); `08` 3.5 (`08:811-812`).
- **Szenario:** `12` sagt «Was dort nicht steht, ist nicht erlaubt» und führt Weg W als benannte Ausnahme. `02`,
  das die Grundlage für Regeln, Spec und ADR ist, führt Weg W nicht als A-Nummer; ADR 0046 nennt Weg W ohne die
  Bedingungen «nur mit Sperre, nur nach M-10». SM-43 prüft nur A-1 bis A-9, also nicht Weg W. Ebenso fehlen der
  benannte Zusatz zu Regel 3 (`execve` in der Hülle) und die Abweichung, dass `listProfileLiveRecordings` ohne
  eigenes Recht «Live-TV» des Hauptprofils antwortet, während SM-38 für jede Live-Operation ohne Recht 404
  verlangt. Ein späterer Bau kann sich auf die kürzere Fassung berufen.
- **Korrektur (Pflicht vor 0, mit SM-43):** In `02` drei Zeilen: A-10 «Weg W» mit Grenze wörtlich aus `05` 2.6
  (Sperre hart, M-10, nie «host»), A-11 «Regel 3, `execve` nur in `internal/transcode` für die Hülle», A-12
  «Hauptprofil verwaltet Aufnahmen seiner Profile auch ohne eigenes Live-TV» (oder `08` F-1 so entscheiden, dass
  SM-38 ohne Ausnahme gilt). ADR 0046 Zeile 54 um die Bedingungen ergänzen. Wird RT-01 (c) gewählt, kommt A-13 dazu.

### RT-10 · Aufnahmen spielen als VOD-Liste: die Prüfung «je Listenanfrage» greift nicht · **niedrig**

- **Stelle:** `08` 4.2 «Prüfung je Anfrage» (`08:1410`), `11` 6.2 (`11-dvr.md:500`: `EXT-X-ENDLIST`), `12` 9.2.
- **Szenario:** Eine fertige Aufnahme hat eine abgeschlossene Liste, die der Player einmal lädt; danach holt er nur
  Segmente, und Segmente prüfen «Token und Sitzung, nicht die Anmeldung». Ändert sich die Zugriffsregel ohne Haken
  in `auth` (das Hauptprofil schaut die Aufnahme eines Kinds, und die Altersgrenze des Kinds wird entfernt: nach
  `07` 4.3 erlischt der Zugriff), läuft die Wiedergabe bis zum Ende weiter. Dasselbe beim Löschen der Aufnahme
  während der Wiedergabe, wenn der Purge-Job die Datei noch nicht entfernt hat.
- **Korrektur (vor 5):** Der Herzschlag prüft höchstens 30 s alt Recht, Anmeldung **und** Zugriffsregel der
  Aufnahme; scheitert sie, endet die Sitzung (410). Löschen oder `deleting` beendet Sitzungen auf dieser Aufnahme
  sofort. Testfall in SM-36 ergänzen.

### RT-11 · Ein Herzschlag ohne Segmentabruf hält Sitzung und Eingang unbegrenzt · **niedrig**

- **Stelle:** `10-timeshift.md:136` («Eine Pause länger als die Höchstdauer … Die Sitzung endet deshalb nicht.»),
  `04` 10.3 (`04:895`), `08` 3.4 «Ein Gerät, eine Sitzung» (`08:635-636`).
- **Szenario:** Ein vergessener Tab oder ein Skript mit gültiger Anmeldung sendet alle 10 s einen Herzschlag mit
  Pause. Die Sitzung belegt einen der 8 Eingänge, einen Platz der Stream-Grenze und eine Verbindung beim Anbieter,
  ohne dass jemand schaut; ab 11 Stunden gibt es ein neues Token, also ohne Ende. Zweitens: `deviceId` beim Anlegen
  kommt vom Client; ein Gerät desselben Profils kann mit fremder `deviceId` die Sitzung eines anderen Geräts beenden
  (nur innerhalb des Profils, darum niedrig).
- **Korrektur (vor 3):** Höchstdauer einer Sitzung ohne Segmentabruf (Vorschlag: Fenster des Zeitversatzes plus
  15 Minuten, ohne Zeitversatz 15 Minuten), danach Ende; in der Rangfolge bei Mangel weicht eine pausierte Sitzung
  vor dem Vorladen. `deviceId` an die Anmeldung binden (Bearer der App trägt die Gerätekennung; im Browser eine
  Kennung im Cookie der Anmeldung).

### RT-12 · Andere Antworten als 404 können vor der Rechteprüfung stehen · **niedrig**

- **Stelle:** `08` 3.1 (`08:315-338`); `12` 9.3, SM-38 (`12:999`).
- **Szenario:** Für ein Profil ohne Recht muss jede Live-Operation 404 liefern. Prüft der Bau Drosselung,
  Serverdeckel oder Zustände vorher, verraten 429 `live_too_fast`, 503 `live_capacity`, 503 `live_epg_rebuilding`,
  409 `stream_limit_reached` oder 403 `watch_limit_reached`, dass es Live-TV gibt und wie es ausgelastet ist. 410
  `live_session_gone` für eine **fremde** beendete Sitzung unterscheidet «gab es» von «gibt es nicht».
  `deleteLiveRecordings` liefert `skipped: [{id, reason}]`; ein eigener Grund für fremde Kennungen wäre ein Orakel.
- **Korrektur (vor 1a):** Feste Reihenfolge im Dienst `live`: Anmeldung, CSRF (403 wie heute), Recht, Objekt, erst
  dann Grenzen und Zustände. 410 nur für eigene Sitzungen; fremde und unbekannte Kennungen in `skipped` mit
  demselben Grund `not_found`. SM-38 prüft jede Operation auch im Zustand «gedrosselt», «Deckel voll» und
  «EPG wird neu gebaut».

### RT-13 · Programmdaten haben keine Mengengrenze; `epg.db` liegt im Cache ohne Reserveprüfung · **niedrig**

- **Stelle:** `07:41` (`<cache>/live/epg.db`), `09` 8.6 (Mengengerüst nur als Annahme), `12` 7.4 Zeile XMLTV
  (4 GiB entpackt, 256 KiB je Element, aber keine Zahl je Sender und Tag).
- **Szenario:** Ein gekaperter EPG-Host (AK-3) liefert innerhalb der Grenzen 4 GiB gültiger Sendungen von je
  einer Sekunde für die zugeordneten Sender. `epg.db` mit Volltextindex und WAL wächst um mehrere GB auf dem
  Volume, auf dem auch die Umwandlung der Filme und ab Werk der Zeitversatz schreiben (SG-5).
- **Korrektur (vor 2):** Höchstens 500 Sendungen je Sender und Tag und eine Gesamtzahl je Import (Vorschlag:
  Sender × Tage × 100), Überschuss verworfen und gezählt; vor dem Import und je Stapel die Mindestreserve des
  Cache-Volumes prüfen, sonst abbrechen und den alten Stand behalten.

### RT-14 · Ursprünge heissen in `15` «Hosts»; ein `http`-Eintrag auf demselben Host umgeht den Abbruch · **niedrig**

- **Stelle:** `15` Schritt 2 «Ursprünge» (`15:324`: «Liste der Hosts … ab Werk nur der Host»); `04:947`, `12:537`
  (Schema, Host, Port); `12` 4.6 (`12:407`, nur Weiterleitungen).
- **Szenario:** Wird der Vergleich nach `15` als Host gebaut, gehen Kopfzeilen und `Authorization` einer
  `https`-Quelle auch an einen Listeneintrag `http://anbieter.example/…`: Das ist keine Weiterleitung, also greift
  der Abbruch `https`→`http` nicht, und die Zugangsdaten gehen im Klartext übers Netz. Zweitens schlägt die
  Verwaltung «die Hosts der importierten Sender» als Ursprünge vor (`12:537`); ein Listen-Host, der einen Sender auf
  `sammler.example` einträgt, steht damit in der Vorschlagsliste. Drittens steht ein Token im Hostnamen
  (`<token>.cdn.example`) in `site` im Klartext.
- **Korrektur (vor 1a):** `15` auf «Ursprung = Schema, Host, Port» angleichen; Geheimnisse einer `https`-Quelle nie
  an `http`, auch nicht an einen ausdrücklich eingetragenen Ursprung ohne eigene Bestätigung. Vorschläge ohne
  «alle übernehmen», mit Zahl der Sender je Host und dem Hinweis, dass die Liste sie nennt. `site` kürzt einen
  Hostnamen mit einem Teil über 32 Zeichen auf die registrierbare Domain (Annahme).

### RT-15 · `recover` fängt nicht jeden Absturz · **niedrig**

- **Stelle:** `12` 6.4 Punkt 1 (`12:671-676`), SM-25.
- **Szenario:** `recover` fängt `panic`, aber keine fatalen Laufzeitfehler [Annahme, Go-Laufzeit]: Überlauf des
  Stapels durch tiefe Rekursion, «out of memory», gleichzeitiges Schreiben in eine Map. Ein rekursiver Leser über
  eine Tiefe aus der Quelle (MPD, JSON) oder eine Allokation aus einem Längenfeld beendet so den ganzen Server.
  Dazu laufen Go-Routinen von Bibliotheken (Stufe L: `gortsplib`) ausserhalb der eigenen Startfunktion.
- **Korrektur (vor 1a):** In `12` 6.4 festhalten: keine Rekursion über eine Tiefe, die die Quelle bestimmt
  (Schleife mit eigenem Stapel und Grenze); `GOMEMLIMIT` des Servers gesetzt; Fuzz-Läufe auch mit `-race`; für
  Bibliotheken (Stufe L) ein Test, dass ein `panic` in ihren Rückrufen über die eigene Startfunktion läuft.

### RT-16 · `RLIMIT_NPROC` zählt alle Threads der UID, nicht die des Prozesses · **niedrig**

- **Stelle:** `12` 6.3 Zeile «Ressourcengrenzen» (`12:642`: «`RLIMIT_NPROC` klein … Annahme 64 wegen Threads»).
- **Szenario:** Die Grenze vergleicht beim Anlegen eines Threads die Zahl **aller** Threads der realen UID im
  Container, also auch die des Servers und aller anderen FFmpeg [Annahme, `setrlimit(2)`]. Mit 64 scheitert die
  Thread-Erzeugung von FFmpeg, sobald der Server unter Last viele Threads hat. Folge ist ein Ausfall, und der
  Druck, die Grenzen im Bau wegzulassen.
- **Korrektur (vor 1a):** `RLIMIT_NPROC` nicht als Grenze je Prozess verwenden; die Zahl der Threads von FFmpeg
  über `-threads` begrenzen, die Prozesszahl über `pids_limit` (SM-23). M-25 misst es.

### RT-17 · Der Bildweg ist falsch beschrieben und liegt ausserhalb der Hülle · **niedrig**

- **Stelle:** `12` 6.2 Zeile «Bild skalieren» (`12:617`: «`pipe` (gebaut für Bilder laut V1 §1)»), 4.9, SM-13.
- **Szenario:** Das gebaute Bild-FFmpeg liest eine Datei mit `-protocol_whitelist file`, nicht eine Pipe, und
  startet ohne eigene Umgebung. Logos und EPG-Bilder sind Fremddaten von beliebigen Hosts (AK-4). Ohne Hülle gilt
  für sie RS-03 in voller Höhe, sobald ein Bild die Go-Prüfung besteht und trotzdem einen FFmpeg-Decoder trifft
  (anderer Codepfad, Metadaten).
- **Beleg:** [belegt Code] `server/internal/transcode/images.go:108`, `:174` (`-protocol_whitelist file`), `:302`
  (`exec.CommandContext` ohne `Env`).
- **Korrektur (vor 1a):** Text richtigstellen. Go dekodiert und **kodiert neu** (PNG), FFmpeg bekommt nur diese
  Bytes, oder Go skaliert selbst; das Bild-FFmpeg für Live läuft über die Hülle.

### RT-18 · Die übrigen FFmpeg-Aufrufe bleiben ungehüllt und schützen jetzt mehr · **mittel**

- **Stelle:** `12` 6.2 («Kein Live-Aufruf erbt …», `12:620-622`), 6.3 (Hülle nur für Live), R-1.
- **Szenario:** Dateiwiedergabe, Untertitel, Bilder, Fingerabdruck und Probe der Bibliothek starten FFmpeg weiter
  ohne `cmd.Env` und ohne Grenzen. Medien in der Bibliothek kommen oft aus dem Usenet über Radarr und Sonarr, also
  von Fremden. Eine Lücke dort liest `secret.key` und die Umgebung mit allen `SLOUCHCAT_*`-Schlüsseln. Das Risiko
  besteht heute schon; mit Live-TV entschlüsselt derselbe Schlüssel zusätzlich alle Anbieter-Zugänge. Die Hülle
  schützt die Live-Seite, solange die Bibliotheks-Seite offen bleibt, nur zur Hälfte.
- **Beleg:** [belegt Code] `transcode/session.go:649`, `:663`; `backend.go:314`; `subtitles.go:163`, `:221`;
  `tags.go:84`; `fingerprint.go:33`; `probe.go:535`; `images.go:302`: kein `cmd.Env`, kein `SysProcAttr`.
- **Korrektur (empfohlen vor 1a, Code ohne Risiko):** Eigene kleine Umgebung und `RLIMIT_CORE 0` für **jeden**
  FFmpeg- und `ffprobe`-Aufruf (eine Funktion in `transcode`); die Hülle mit Dateisperre für die Bibliothek als
  eigener Schritt nach M-25. Als Verschärfung in `12` 1.2 eintragen.

### RT-19 · Die Verwaltung kann private Aufnahmen über Technik und Zeit zuordnen · **niedrig**

- **Stelle:** `15` Abschnitt 4 (`15:916-919`: LA-21 «Private Aufnahme, Konto, Profil, Gerät, Technik»; LA-12
  Aufnahmen als Zahl je Eingang).
- **Szenario:** Läuft auf Sender S genau eine Aufnahme («Aufnahmen: 1»), und schaut Konto X zur selben Zeit eine
  «Private Aufnahme» an der Spitze (`11` 6.2: fast live), mit derselben Auflösung, Bildrate und Bitrate wie der
  Eingang S, dann folgt «X nimmt S auf» und mit dem Programm der Titel. SM-45 verbietet die Kombination nur in
  einer Antwort, nicht über zwei Seiten.
- **Korrektur (vor 5):** Für `live_recording` in LA-21 nur grobe Technik (Direktweg oder Umwandlung, Zielqualität),
  keine Quellparameter; die Zahl laufender Aufnahmen je Eingang erst ab 2 oder nur als «Aufnahmen laufen».

### RT-20 · Redaktionelle Widersprüche mit Sicherheitsbezug · **niedrig**

- `12` Abschnitt 12.4 steht vor 12.3, R-11 vor R-10; in 7.1 gibt es zweimal Punkt «5.».
- `12` SL-5 (`12:1144`) sagt «bis zur Antwort gilt `02` A-6 wörtlich»; `02` A-6, `03` DR-02 und `07` 4.3 setzen
  Variante (b) mit «heute noch» um. Einheitlich «Vorgabe (b) mit Gegenrichtung nach `07` 4.3» schreiben.
- `04` 10.5 nennt die Bindung an die Zeile «Empfehlung», `07` 6.2 und `12` SM-14 legen sie fest (Pflicht 1a).
- `08` 4.4 (`08:1436`) verweist für Logos im Service Worker nach dem Entzug auf ein Restrisiko in `12`, das dort
  fehlt (R-Liste `12:1150-1164`).
- `12` 2.4 nennt als Beispiel eines freigegebenen Geräts «TVHeadend»; Regel 11 verbietet TVHeadend auch nach der
  Freigabe. Gemeint ist eine M3U über HTTP; das Beispiel neutral fassen, damit es nicht als Erlaubnis gelesen wird.

### RT-21 · Testfälle fehlen für neue Pflichten · **niedrig**

- SM-47 (laufende Verbindung endet innert 5 s nach Entzug eines Ziels) hat keinen Fall in `17`; AK-L-231 prüft nur
  ruhende Verbindungen (`17:638`).
- Für RT-02 (Thread-Bindung) und RT-03 (`io_uring`, `int 0x80`, `SOCK_NONBLOCK`) gibt es keine Fälle; AK-L-251
  (`17:658`) prüft die Sperre nur einmal und «CT (Landlock, nein)», also nicht in der CI.
- Für RT-04 (Pause und Schauzeit) und RT-06 (Quelle entfernt) fehlen Fälle.
- **Korrektur:** je ein AK-L-Fall, für die Sperre ein CI-Lauf auf einem Runner mit Landlock (die GitHub-Runner
  haben Kernel ab 6.x; Annahme, im Bau prüfen).

## 5. Abdeckung Auftrag Abschnitt 10, Zeile für Zeile

Testfall: AK-L-(229 + n) prüft SM-n (`17` Abschnitt Sicherheit, AK-L-230 bis -273); SM-45 und SM-46 prüfen
AK-L-327 und -328. Urteil: **gedeckt** (Massnahme und Testfall vorhanden, kein offener Befund), **gedeckt mit
Rest** (neuer Befund niedrig), **teilweise** (Befund mittel oder hoch oder Testfall fehlt).

| Z. | Zeile des Auftrags | Massnahme | Testfall | Urteil |
|---|---|---|---|---|
| 1324 | SSRF bei Stream-URLs | SM-01, -02, -03, -05 | AK-L-230 bis -234 | gedeckt |
| 1325 | SSRF bei EPG-URLs | SM-01 (EPG-Dialer), SM-04; `09` 3.2 | AK-L-230, -233 | gedeckt (Heimnetz nur nach `09` F-4) |
| 1326 | SSRF bei M3U-URLs | SM-01, SM-06 | AK-L-230, -235 | gedeckt |
| 1327 | Weiterleitungen | SM-04 | AK-L-233 | gedeckt |
| 1328 | DNS-Rebinding | SM-05 | AK-L-234 | gedeckt |
| 1329 | IPv4 | SM-03 | AK-L-232 | gedeckt mit Rest (WAN-Adresse, R-4) |
| 1330 | IPv6 | SM-03, SM-11 | AK-L-232, -240 | gedeckt mit Rest (R-4) |
| 1331 | Zugriff auf interne Netzwerke | SM-01, SM-06 | AK-L-230, -235 | **teilweise** (RT-05: Netz bis /24) |
| 1332 | Explizite Freigaben lokaler Quellen | A-3, SM-01, SM-06, SM-41 | AK-L-045, -230, -270 | **teilweise** (RT-05, RT-06) |
| 1333 | Multicast- und UDP-Risiken | SM-10, SM-11 | AK-L-239, -240 | gedeckt (R-2 ehrlich) |
| 1334 | Verschachtelte HLS- und DASH-Manifeste | SM-07, SM-08 | AK-L-236, -237 | gedeckt mit Rest (RT-08, nur W) |
| 1335 | Segment-URLs | SM-01, SM-07 | AK-L-230, -236 | gedeckt |
| 1336 | Schlüssel-URLs | SM-01, SM-07, SM-44 | AK-L-230, -236, -273 | gedeckt (DRM-Grenze Annahme, R-10) |
| 1337 | Playlist-URLs | SM-01, SM-07 | AK-L-230, -236 | gedeckt |
| 1338 | Externe Senderlogos | SM-13 | AK-L-242 | gedeckt mit Rest (RT-17) |
| 1339 | Externe EPG-Bilder | SM-13 | AK-L-242 | gedeckt mit Rest (RT-17) |
| 1343 | Sichere Speicherung von Zugangsdaten | SM-14 | AK-L-243 | gedeckt |
| 1344 | Zugangsdaten innerhalb von URLs | SM-14, -15, -17 | AK-L-243, -244, -246 | gedeckt |
| 1345 | Konfigurierbare HTTP-Header | SM-14, -15, -16 | AK-L-243 bis -245 | gedeckt |
| 1346 | Authentifizierung gegenüber Streamquellen | SM-15, SM-20 | AK-L-244, -249 | gedeckt mit Rest (RT-14) |
| 1347 | Keine Zugangsdaten in Logs | SM-17 | AK-L-246 | gedeckt |
| 1348 | Keine Zugangsdaten in Client-Antworten | SM-16 | AK-L-245 | gedeckt |
| 1349 | Keine Zugangsdaten in Fehlermeldungen | SM-16, SM-17 | AK-L-245, -246 | gedeckt |
| 1350 | Schutz in Backups | SM-14, SM-18 | AK-L-243, -247 | gedeckt (Vorgabe F-4 (a) sicher) |
| 1351 | Keine Weitergabe an nicht autorisierte Ziele | SM-04, -15, -20 | AK-L-233, -244, -249 | gedeckt mit Rest (RT-14; RT-08 in W) |
| 1355 | Command Injection | SM-21 | AK-L-250 | gedeckt |
| 1356 | FFmpeg-Protokollbeschränkungen | SM-21, SM-08 | AK-L-250, -237 | gedeckt |
| 1357 | Bösartige Medienstreams | SM-22, -23, -25, -26 | AK-L-251, -252, -254, -255 | **teilweise** (RT-01, RT-02, RT-03, RT-18) |
| 1358 | Fehlerhafte Mediendaten | SM-25 | AK-L-254 | gedeckt mit Rest (RT-15) |
| 1359 | Prozesslimits | SM-22, -23, -24 | AK-L-251 bis -253 | gedeckt mit Rest (RT-16) |
| 1360 | Timeouts | SM-24 | AK-L-253 | gedeckt |
| 1361 | Sichere Prozessbeendigung | SM-24 | AK-L-253 | gedeckt |
| 1362 | Verwaiste Prozesse | SM-22, SM-24 | AK-L-251, -253 | gedeckt |
| 1363 | Ressourcenerschöpfung (Prozesse) | SM-22, -24, -39 | AK-L-251, -253, -268 | gedeckt mit Rest (RT-11) |
| 1365 | Indirekt abgerufene URLs umgehen den SSRF-Schutz nicht | SM-08, -09, -12 | AK-L-237, -238, -241 | gedeckt für die Regel; **teilweise** für den Bau der Sperre (RT-02, RT-03) |
| 1369 | XML External Entities | SM-28 | AK-L-257 | gedeckt |
| 1370 | XML-Bomben | SM-28 | AK-L-257 | gedeckt |
| 1371 | Komprimierte XMLTV-Dateien | SM-29 | AK-L-258 | gedeckt |
| 1372 | Dekompressionsbomben | SM-29, 4.1 Punkt 4 | AK-L-258 | gedeckt |
| 1373 | Fehlerhafte M3U-Dateien | SM-27, SM-25 | AK-L-256, -254 | gedeckt |
| 1374 | Uploadlimits | SM-27 | AK-L-256 | gedeckt |
| 1375 | Dateigrössenlimits | SM-27, -28, -07 | AK-L-256, -257, -236 | gedeckt |
| 1376 | Parser-Zeitlimits | SM-27, SM-28 | AK-L-256, -257 | gedeckt |
| 1377 | Ressourcenerschöpfung (Parser) | SM-27 bis -29, SM-39 | AK-L-256 bis -258 | gedeckt mit Rest (RT-13) |
| 1381 | Unbegrenzte Puffer | SM-31 | AK-L-260 | gedeckt mit Rest (RT-13: `epg.db`) |
| 1382 | Path Traversal | SM-32 | AK-L-261 | gedeckt |
| 1383 | Dateisystemrechte | SM-32 | AK-L-261 | gedeckt |
| 1384 | Zugriffsschutz Timeshift-Dateien | SM-34, SM-35 | AK-L-263, -264 | gedeckt |
| 1385 | Zugriffsschutz Aufnahmen | SM-34, SM-37 | AK-L-263, -266 | gedeckt mit Rest (RT-10) |
| 1386 | Datenträgerausfälle | SM-33 | AK-L-262 | gedeckt |
| 1387 | Sichere Bereinigung temporärer Dateien | SM-32, SM-33 | AK-L-261, -262 | gedeckt |
| 1391 | Authentifizierung von Stream-Endpunkten | SM-35 | AK-L-264 | gedeckt |
| 1392 | Authentifizierung von Segment-Endpunkten | SM-35 | AK-L-264 | gedeckt |
| 1393 | Berechtigungsprüfung bei jedem relevanten Zugriff | SM-35, -36, -38 | AK-L-264, -265, -267 | gedeckt mit Rest (RT-10, RT-12) |
| 1394 | Widerruf laufender Wiedergabeberechtigungen | SM-36, SM-47 | AK-L-265; **für SM-47 keiner** | **teilweise** (RT-06, RT-21) |
| 1395 | Schutz privater Profildaten | SM-37 | AK-L-266 | **teilweise** (RT-07) |
| 1396 | Schutz privater Aufnahmebibliotheken | SM-37, -42, -45, -46 | AK-L-266, -271, -327, -328 | **teilweise** (RT-07, RT-19) |
| 1397 | Bestehende Session- und Token-Architektur | SM-35, SM-40 | AK-L-264, -269 | gedeckt |
| 1398 | Bestehende Account-Limits | SM-39; `04` T-26 | AK-L-268, AK-L-017 | **teilweise** (RT-04) |
| 1399 | Serverseitige Durchsetzung sämtlicher Freigaben | SM-37, SM-38 | AK-L-266, -267 | **teilweise** (RT-04 verletzt Regel 12) |
| 1400 | Unsichtbares liefert 404 | SM-35, SM-38 | AK-L-264, -267 | gedeckt mit Rest (RT-12) |
| 1402 | Bestehende Sicherheitsanforderungen bleiben gültig | SM-43 | AK-L-272 | gedeckt mit Rest (RT-18) |
| 1404 | Keine stillschweigende Lockerung | SM-43, `12` 1.1, 1.2 | AK-L-272 | **teilweise** (RT-09; bei RT-01 (c) eine neue) |
| 1406 | Unabhängige OMC Security Reviewer | Review 1, dieses Review | – | gedeckt |

## 6. Autorisierung im Datenmodell und in der API

| Prüfpunkt | Ergebnis |
|---|---|
| 404 ohne Orakel | Regel und Körper gleich für Recht, Objekt, Token (`08:320`, `07:1626-1632`). Zeit: HMAC vor Zustand, Zustand im Speicher (`08:1410`), fremde und fehlende Objekte in einer Abfrage (`07:1562`). Rest: Reihenfolge gegenüber 429/503/409/410 (RT-12). |
| IDOR Aufnahmen | eine Abfrage mit `profile_id` oder Hauptprofil-Fragment, kein Laden-und-dann-Prüfen (`07:1543-1559`). Gleiches Fragment für Liste und Einzelabruf. Gut. |
| IDOR Planungen, Erinnerungen, Regeln | `WHERE profile_id = @profile_id`, Kennung aus der Sitzung, nie aus der Anfrage (`07:1587`). Termine des Kinds für das Hauptprofil nur als Zahl. Gut. |
| IDOR Sitzungen, Timeshift-Segmente | Token mit `sessionId` und `tune`, Pfad muss passen; Segment nur im Fenster (`08:1408-1413`); `endLiveSession` nur eigene. Rest: `deviceId` (RT-11). |
| Token `lt` | Bindung an Sitzung und Einstellen, 12 h, nur Herzschlag verlängert, HMAC mit eigenem Zweckschlüssel und Kennzeichen `live-v1`. Widerruf: mit Haken sofort (Ziel 10 s), ohne Haken höchstens 30 s plus Nachladen der Liste; bei Aufnahmen als VOD-Liste erst über den Herzschlag (RT-10). R-9 ehrlich. |
| Hauptprofil und Altersgrenze | nur dasselbe Konto, nur `owner_age_limited = 1` **und** heute noch `min(Profil, Konto) < 18`; `max_rating` ist `NOT NULL` [belegt Code `00002:31`, `00007:23`], also kein NULL-Fehler in `min()`. Nachträglich gesetzte Grenze: alte Aufnahmen bleiben privat; entfernte: Zugriff erlischt. Konto-Altersgrenze: zählt mit, nicht in A-6 benannt (RT-07). |
| PIN-Profile | Meldungen ohne Titel (DR-05); Hauptprofil kann die PIN ersetzen (RT-07). |
| Meldungen ohne Titel | «Kontingent voll» an das Hauptprofil ohne Titel (SM-46, AK-L-328). E-Mail an die Adresse des Kontos für Profile ohne PIN mit Titel (RT-07). |
| Verwaltungssicht | Kombination Konto und Sender laufender Aufnahmen in keiner Antwort (SM-45, AK-L-327); `AdminStream` `live_recording` ohne Sender (`08:278`, `08:1366-1370`). Rest: Zuordnung über zwei Seiten (RT-19). |
| `x-clients` | alle Zuschauer-Operationen `[app, tv]`, Verwaltung ohne (`08:91-93`); gebautes Muster der Radio-Routen. Gut. |
| T-29 und Rechteentzug | Ein Profil, dem «Live-TV» entzogen wird, verliert die Wiedergabe sofort über den Haken (`12:831`); T-29 gilt nur für den Sender. **Aber** eine **entfernte Quelle** läuft für laufende Zuschauer und Aufnahmen weiter (RT-06). |

## 7. Benannte Ausnahmen und Regel 11

- **A-1 bis A-9** sind eng gefasst und haben je eine Grenze und einen Beleg im Auftrag (`02:123-131`). A-3 ist mit
  den festen Zielen jetzt widerspruchsfrei (Verweise innerhalb der Ziele). Zu eng gefasst ist nichts; zu weit ist
  die Netzgrösse in A-3 (RT-05).
- **Nicht geführt:** Weg W als A-Nummer, der Zusatz zu Regel 3 für `execve`, die Wirkung von
  `listProfileLiveRecordings` ohne eigenes Recht (RT-09). Mit RT-01 (c) käme eine weitere dazu.
- **Lockert ein Dokument etwas Ungeführtes?** Die Schauzeit-Regel in `04` 10.3 macht eine bestehende Grenze vom
  Client abhängig (RT-04). Das steht nirgends als Ausnahme und ist als Lockerung von Regel 12 zu werten. Sonst nichts.
- **Regel 11** (`CLAUDE.md:89-99`) ist nicht als Baufreigabe lesbar: «noch nicht zum Bau freigegeben … keinen Code,
  keine Migration, keinen Vertrag, nichts vorbereiten». ADR 0046 ist «vorgeschlagen» und wird erst mit Stufe 0
  angenommen. Gut. Einziger Stolperstein: «TVHeadend» bleibt nach Regel 11 verboten, `12` nennt es als Beispiel
  eines freigegebenen Geräts (RT-20).

## 8. Restrisiken und Fragen an den Auftraggeber

**R-1 bis R-11:**

| Nr. | Ehrlich? | Bemerkung |
|---|---|---|
| R-1 | **zu knapp** | nennt nur das Lesen; das Überschreiben der Datenbank innerhalb ihrer Grösse und die daraus mögliche Kontoübernahme fehlen (RT-01). Die Bibliotheks-Seite (RT-18) fehlt. |
| R-2 | ja | Wortlaut V-3 sagt es den Betreibern. |
| R-3 | **zu knapp** | «nur den Strom und die Ersatzpfade seiner eigenen Quelle» stimmt nicht, solange der Weiterleiter beliebige Pfade je Host mit Geheimnissen beantwortet (RT-08). |
| R-4, R-5, R-6 | ja | R-6 nennt nur das Gerät, nicht den Listen-Host einer Internet-Liste (RT-05). |
| R-7, R-8 | ja | – |
| R-9 | ja | – |
| R-10 | ja | Annahme offen benannt. |
| R-11 | ja | Text im Dialog vorgesehen; RT-06 gehört daneben. |
| fehlt | – | R-12 Privatsphäre zwischen Profilen nur mit PIN (RT-07); Logos im Service Worker nach Entzug (`08:1436`). |

**Fragen mit Sicherheitsfolgen:**

| Frage | Verständlich? | Vorgabe bis zur Antwort | Urteil |
|---|---|---|---|
| Netzmodus «host» (`04` F-11, `20` E-28-34) | ja, mit den vier Folgen | keine Zusatzdatei ohne Wahl; A-9 nur Stufe H | sicher |
| Umwandlung ohne Sperre (`12` F-1, `20` E-28-04) | ja | **(a) ohne Sperre weiter** | **nicht sicher**: Vorgabe (b) oder (c) nach RT-01 |
| Compose-Härtung für alle (`04` F-12, `20` E-28-03) | ja | (a) für alle nach Probelauf | sicher; die Hülle setzt `no_new_privs` ohnehin selbst |
| `https`→`http` (`12` F-2, `20` E-28-05) | ja | (a) immer abbrechen | sicher; RT-14 schliesst den Weg über Listeneinträge |
| Zertifikatsprüfung abschalten (`12` F-3) | ja | (a) nie | sicher |
| Sicherung (`12` F-4, `20` E-28-11) | ja | (a) leeren mit `v0:` | sicher; `07` 6.3 und `04` 10.5 nach dem Entscheid angleichen |
| Weg W ohne Sperre (`04` F-13, `20` E-28-55) | ja | (a) nur mit Sperre; ohne Sperre und M-10 kein Bau | sicher |
| Hauptprofil und nachträgliche Grenze (`04` F-10, `20` E-28-07) | ja | (b) mit «heute noch» | sicher; RT-07 als Teil der Frage stellen |
| Konto-Altersgrenze zählt mit (`07` F-1) | knapp | (a) zählt mit | Folge für A-6 nennen (RT-07) |

## 9. Risikourteil und Bedingungen für eine Baufreigabe von Stufe 0/1a

**Urteil für den jetzigen Stand: MITTEL.** Die Architektur ist richtig geschnitten: Go hält das Netz, FFmpeg
bekommt in 1a nur Pipes, jede Adresse ist geheim, die Autorisierung ist eine Abfrage je Objektart, und die
gefährlichen Teile (Heimnetz, Multicast, Weg W) liegen in späten Stufen mit Messpunkten. Hoch bleibt das Risiko
dort, wo die Vorgabe von F-1 auf einem Host ohne Landlock greift (RT-01). Für die Stufen H und W kommen RT-05,
RT-08 dazu; sie blockieren 0/1a nicht.

**Zusätzlich nötig vor der Baufreigabe von Stufe 0 und 1a** (über die Pflichten aus `12` 11 hinaus):

1. **F-1 sicher vorbelegen** (RT-01): ohne Dateisperre keine Umwandlung, ausser nach ausdrücklicher Wahl des
   Betreibers; R-1 ergänzen; M-25 auf CT 190 **vor** der Freigabe messen.
2. **Hülle genau festlegen** (RT-02, RT-03): Thread-Bindung bis `execve`, Seccomp als Erlaubnisliste mit
   Architekturprüfung und ohne `io_uring`, kein `socket()` für Umwandlung und Probe, Seccomp ohne Ausnahme Pflicht,
   Landlock-Bereiche ab ABI 6; Testfälle in `17` und ein CI-Lauf mit Landlock.
3. **Schauzeit unabhängig vom Client** (RT-04) in `04` 10.3 und `08` ändern.
4. **Entfernen oder Ändern einer Quelle beendet ihre Empfänger** (RT-06) in `04` 5.3, `07` 10.3, `12` 9.2.
5. **Reihenfolge der Prüfungen und Statuscodes** (RT-12) und die Korrektur der Ursprünge in `15` (RT-14).
6. **Kleine Umgebung und `RLIMIT_CORE 0` für jeden FFmpeg-Aufruf** (RT-18) und Bildweg richtigstellen (RT-17).
7. **Ausnahmen nachtragen** (RT-09) mit SM-43 in Stufe 0; RT-20 redigieren; Testfälle nach RT-21.

Vor Stufe H: RT-05. Vor Stufe 3: RT-11. Vor Stufe 2: RT-13. Vor Stufe 5: RT-07, RT-10, RT-19. Vor Stufe W: RT-08.

## 10. Sicherheits-Checkliste

- [x] Keine hart codierten Geheimnisse in Konzept und geprüftem Code-Ausschnitt
- [x] Eingaben begrenzt (Parser-Tabelle `12` 7.4) — Rest RT-13
- [x] Injektion verhindert (Argumente nur Zahlen und Listenwerte, FTS5 zitiert)
- [ ] Authentisierung und Autorisierung vollständig — offen RT-04, RT-07, RT-10, RT-12
- [ ] FFmpeg-Eindämmung sicher vorbelegt — offen RT-01, RT-02, RT-03
- [x] Zugangsdaten verschlüsselt, nie in Antworten, Logs, Argumenten — Rest RT-14
- [ ] Ausnahmen vollständig benannt — offen RT-09
- [ ] Abhängigkeiten geprüft — nicht ausgeführt (Auftrag); SM-26 verlangt `govulncheck` in der CI
