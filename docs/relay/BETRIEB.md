# Betrieb der zwei Dienste: Vermittlung und Push-Weiterleitung

Dieses Papier beschreibt, wie die zwei Dienste aus Konzept 27 Abschnitt 7 laufen, wie man sie
einrichtet, überwacht und umzieht. Entscheide: ADR 0042 (Betrieb und Umzug) und ADR 0044 (Push).
Code: `relay/`, Images und Compose-Dateien: `deploy/relay/pair/` und `deploy/relay/push/`.

| Dienst | Fester Name | Programm, Image | Hält an Geheimnissen |
|---|---|---|---|
| Vermittlung | `pair.slouchcat.com` | `relay/cmd/pair`, `ghcr.io/slouchcat/relay-pair` | nichts |
| Push-Weiterleitung | `push.slouchcat.com` | `relay/cmd/push`, `ghcr.io/slouchcat/relay-push` | Firebase-Schlüssel, Schlüsseldatei zum Versiegeln |

Die zwei Dienste laufen getrennt, je auf einer eigenen Maschine (E-21), und ziehen unabhängig
voneinander um. Beide haben keine Datenbank und keinen Zustand auf der Platte (D-1): Die
Vermittlung hält Treffpunkte höchstens fünf Minuten im Arbeitsspeicher, die Push-Weiterleitung
reicht Meldungen durch. Ausser den zwei Geheimnissen der Push-Weiterleitung gibt es nichts zu
sichern.

## 1. Die zwei Orte

Dieselbe Compose-Datei und dasselbe Image gelten an beiden Orten. Der Unterschied steht nur in der
Umgebungsdatei `.env` neben der Compose-Datei; sie schaltet auch das Profil `tls` ein
(`COMPOSE_PROFILES=tls`). Vorlagen: `env.home.example` (eigene Hardware) und `env.rented.example`
(Mietserver), je Dienst.

| | Eigene Hardware (vorläufig, E-22) | Mietserver (später) |
|---|---|---|
| Vorlage | `env.home.example` | `env.rented.example` |
| Profil | keines | `tls` |
| TLS | der vorhandene Reverse-Proxy | der TLS-Abschluss im selben Stack, im Netz des Hosts, eigenes Zertifikat |
| Port des Dienstes (`RELAY_BIND`) | LAN-Adresse der Maschine, nur für den Reverse-Proxy (Firewall, 1.1) | `127.0.0.1`, von aussen nicht erreichbar |
| Vertrauter Proxy (`RELAY_TRUSTED_PROXIES`) | die eine Adresse des Reverse-Proxys | das Gateway des Stack-Netzes (`RELAY_GATEWAY`) |
| Von aussen | Port 443 am eigenen Anschluss | Ports 80 und 443 am Mietserver |

Auf dem Mietserver ist der Port des Dienstes nur an die Loopback-Adresse gebunden. Von aussen
erreicht man ihn nicht. Eine Compose-Datei kann einen Port nicht je Umgebungsdatei ganz weglassen,
darum diese Lösung.

**Warum der TLS-Abschluss im Netz des Hosts läuft:** Hinter der Portweiterleitung von Docker käme
ein Aufrufer über IPv6 beim TLS-Abschluss mit der IPv4-Adresse des Bridge-Gateways an. Alle
IPv6-Nutzer zählten dann als ein einziger Aufrufer, und die Grenzen je /64 (D-2) wären wirkungslos.
Im Netz des Hosts sieht der TLS-Abschluss die echte Adresse, IPv4 wie IPv6. Er erreicht den Dienst
über `127.0.0.1:RELAY_PORT`; Docker reicht diese Verbindung vom Gateway des Stack-Netzes weiter,
darum ist das Gateway (`RELAY_GATEWAY`, fest `.1` im Netz `RELAY_SUBNET`) der eine vertraute Proxy.
Belegt in der lokalen Probe (Abschnitt 8). Der TLS-Abschluss läuft ohne Admin-Schnittstelle,
schreibgeschützt, ohne Fähigkeiten ausser dem Binden an Ports unter 1024 und mit
`no-new-privileges`.

### 1.1 Auf eigener Hardware einrichten

1. Einen Container oder eine VM vom übrigen Netz getrennt bereitstellen (R-3), mit Docker und dem
   Compose-Plugin.
2. Den Ordner `deploy/relay/pair/` (oder `push/`) aus dem Repo auf die Maschine kopieren.
3. `cp env.home.example .env`, dann `RELAY_BIND` (LAN-Adresse der Maschine), `RELAY_PORT` und
   `RELAY_TRUSTED_PROXIES` (die eine Adresse des Reverse-Proxys, so wie der Container sie sieht)
   setzen.
4. Nur Push-Weiterleitung: die zwei Geheimnisse nach `secrets/` legen (Abschnitt 3).
5. `docker compose up -d`, dann `curl http://<RELAY_BIND>:<RELAY_PORT>/v1/healthz`.
6. Im vorhandenen Reverse-Proxy den Namen auf `http://<RELAY_BIND>:<RELAY_PORT>` weiterleiten.
7. Den Port nur für den Reverse-Proxy öffnen. An die LAN-Adresse gebunden, erreichte ihn sonst jeder
   Rechner im Netz ohne TLS. Entweder leistet das die Trennung des Netzes aus Schritt 1, oder eine
   Regel in der Kette `DOCKER-USER` (ersetzt durch die Werte aus `.env`):

   ```bash
   iptables -I DOCKER-USER -p tcp --dport 8080 -m conntrack --ctorigdstport <RELAY_PORT> ! -s <Adresse des Reverse-Proxys> -j DROP
   ```

   Docker leitet den veröffentlichten Port auf den Port 8080 im Container um; die Regel vergleicht
   darum den ursprünglichen Zielport.

Was der Reverse-Proxy für die zwei Namen tun muss:

- TLS mit gültigem Zertifikat abnehmen (D-7).
- `X-Forwarded-For` setzen oder hinten anfügen. Der Dienst liest den Kopf von hinten und glaubt
  ihn nur, wenn die Anfrage von `RELAY_TRUSTED_PROXIES` kommt (D-9). Ohne diesen Eintrag zählen
  alle Grenzen auf den Proxy, und der Dienst sperrt nach wenigen Anfragen alle.
- Für diese Namen keine Adressen von Aufrufern in Logs schreiben (D-5).
- Den `Host`-Kopf unverändert weiterreichen: Die Push-Weiterleitung wählt daran den Namen, unter
  dem sie eine neue Push-Adresse ausgibt.
- **Für den Namen der Push-Weiterleitung nie 404 antworten,** auch nicht, wenn das Ziel fehlt oder
  nicht läuft; dann 502 oder 503. Ein Slouchcat-Server löscht ein Abo bei 404 (Sicherheits-Review
  M-3). Viele Proxys antworten für einen unbekannten Namen mit einem Standard-404: Für
  `push.slouchcat.com` braucht es darum immer einen Eintrag, auch nach einem Umzug (Abschnitt 5,
  Schritt 6).
- Ist `RELAY_TRUSTED_PROXIES` falsch, schreibt der Dienst einmal je Stunde «X-Forwarded-For from a
  peer that is not a trusted proxy» ins Log.

### 1.2 Auf einem Mietserver einrichten

1. Den Mietserver vorbereiten: Docker mit Compose-Plugin, Ports 80 und 443 offen.
2. Den Ordner des Dienstes kopieren, `cp env.rented.example .env`, `RELAY_NAMES` und `RELAY_TLS`
   setzen (Abschnitt 2). Bei der Push-Weiterleitung die zwei Geheimnisse mitkopieren.
3. `docker compose up -d` startet den Dienst und den TLS-Abschluss.

Der TLS-Abschluss holt die Zertifikate für alle Namen in `RELAY_NAMES` selbst über ACME, sobald der
DNS-Eintrag des Namens auf den Mietserver zeigt. Er legt sie im Volume `tls_data` ab; sie lassen
sich jederzeit neu ausstellen und ziehen nicht um. Er schreibt keine Zugriffslogs und lässt die
Logs weg, die Adressen von Aufrufern enthalten (D-5).

## 2. Umgebungsvariablen

Gemeinsam:

| Variable | Vorgabe | Bedeutung |
|---|---|---|
| `RELAY_VERSION` | `1.0.0` | Fassung des Images (Tag `relay-v1.0.0` ergibt `1.0.0`) |
| `COMPOSE_PROFILES` | leer | `tls` auf dem Mietserver |
| `RELAY_BIND`, `RELAY_PORT` | `127.0.0.1`, `8180` / `8181` | Adresse und Port am Host |
| `RELAY_TRUSTED_PROXIES` | leer | Adressen oder Netze, deren `X-Forwarded-For` gilt, durch Komma getrennt |
| `RELAY_LOG_FORMAT`, `RELAY_LOG_LEVEL` | `json`, `info` | Logs; `debug` schreibt eine Zeile je Anfrage, ohne Pfad und Adresse |
| `RELAY_RATE`, `RELAY_BURST` | Vermittlung `2`/`30`, Push `20`/`200` | Anfragen je Sekunde und Vorrat je Aufrufer; IPv6 zählt je /64 (D-2) |
| `RELAY_NET_RATE`, `RELAY_NET_BURST` | Vermittlung `20`/`300`, Push `200`/`2000` | dasselbe je grösserem Netz: IPv6 je /48, IPv4 je /24. Ein /48 enthält 65 536 Netze /64 und ist billig zu haben; ohne diese Grenze reichte er, um alle Plätze zu belegen (Sicherheits-Review M-1, M-2) |
| `RELAY_NAMES` | fester Name | Namen des TLS-Abschlusses, durch Komma getrennt; beim Umzug dazu der Probename |
| `RELAY_TLS` | `internal` | E-Mail-Adresse für das ACME-Konto, oder `internal` für die lokale Prüf-CA des TLS-Abschlusses |
| `RELAY_TLS_HTTP_PORT`, `RELAY_TLS_HTTPS_PORT` | `80`, `443` | Ports des TLS-Abschlusses am Host |
| `RELAY_SUBNET`, `RELAY_GATEWAY` | `172.30.89.0/29`, `172.30.89.1` (Push: `172.30.90.…`) | Netz des Stacks und sein Gateway, der vertraute Proxy auf dem Mietserver |

Eine volle Ratentabelle (100 000 Schlüssel) weist niemanden ab: Ein beliebiger Eintrag weicht dem
neuen. Fremde können die Tabelle also nicht so füllen, dass neue Handys 429 bekommen.

Nur Vermittlung:

| Variable | Vorgabe | Bedeutung |
|---|---|---|
| `RELAY_PAIR_MAX_OPEN` | `10000` | offene Treffpunkte gesamt; darüber 503 |
| `RELAY_PAIR_MAX_OPEN_PER_CALLER` | `10` | offene Treffpunkte je Aufrufer; darüber 429 |
| `RELAY_PAIR_MAX_OPEN_PER_NET` | `50` | offene Treffpunkte je /48 oder /24; darüber 429 |
| `RELAY_PAIR_TTL` | `5m` | Lebensdauer eines Treffpunkts (K-1); höchstens `5m`, kürzer nur für Tests |

Nur Push-Weiterleitung:

| Variable | Vorgabe | Bedeutung |
|---|---|---|
| `RELAY_PUSH_ORIGINS` | `https://push.slouchcat.com` | eigene Ursprünge; `aud` jeder VAPID-Unterschrift muss einer davon sein (D-4). Der erste ist der feste Name. Beim Umzug kommt der Probename dazu, mit Port, falls er nicht 443 ist |
| `RELAY_SECRETS_DIR` | `./secrets` | Ordner mit `firebase.json` und `sealing-keys.json`, schreibgeschützt eingebunden |
| `RELAY_USER` | `65532:65532` | Benutzer im Container; ihm gehören die Geheimnisse |
| `RELAY_PUSH_MAX_BODY` | `2700` | grösster Web-Push-Körper in Byte; darüber 413. Ein Server sendet an die Push-Weiterleitung mit der Satzgrösse 2700 (Stufe 0); Base64 macht 3602 Byte daraus, Google nimmt 4096 |
| `RELAY_PUSH_ADDRESS_RATE`, `…_BURST` | `0.5`, `20` | Meldungen je Sekunde und Vorrat je Push-Adresse; belastet erst nach gültiger VAPID-Unterschrift |
| `RELAY_PUSH_REGISTER_RATE`, `…_BURST` | `0.2`, `10` | Anmeldungen je Sekunde und Vorrat je Aufrufer |
| `RELAY_PUSH_REGISTER_NET_RATE`, `…_BURST` | `2`, `100` | Anmeldungen je /48 oder /24 |
| `RELAY_PUSH_GLOBAL_RATE`, `…_BURST` | `2000`, `4000` | Meldungen an Google je Sekunde für den ganzen Dienst; leer gibt es 503 mit `Retry-After`. Schützt das Kontingent des Firebase-Projekts vor einer Flut |
| `RELAY_FCM_BASE_URL`, `RELAY_FCM_TOKEN_URL` | leer (Google) | nur für den Nachbau `fcmfake` in Proben |

## 3. Geheimnisse der Push-Weiterleitung

Beide liegen als Dateien in `RELAY_SECRETS_DIR`, nie im Image und nie im Repo (D-6). Sie gehören
dem Benutzer des Containers:

```bash
chown 65532:65532 secrets/firebase.json secrets/sealing-keys.json
chmod 0400 secrets/firebase.json secrets/sealing-keys.json
```

Fehlt eine Datei oder ist sie falsch, startet der Dienst trotzdem, schreibt einen Fehler ins Log,
meldet `/v1/healthz` mit 503 und beantwortet jede Meldung mit 503 (D-10). Die Server versuchen es
dann erneut, statt Abos zu löschen.

**Firebase-Schlüssel** (`firebase.json`): die JSON-Datei des Dienstkontos des Firebase-Projekts der
App, so wie die Firebase-Konsole sie unter «Dienstkonten» ausgibt. Gebraucht werden
`project_id`, `client_email`, `private_key` und `token_uri`. Wer sie hat, kann Nachrichten an alle
Handys mit der App schicken (R-4): nur auf der Maschine der Push-Weiterleitung ablegen.

**Schlüsseldatei zum Versiegeln** (`sealing-keys.json`): Sie versiegelt Google-Token und
VAPID-Schlüssel in die Push-Adressen. Jede Fassung hat einen eigenen Schlüssel; die aktuelle
versiegelt neue Adressen, alle eingetragenen öffnen alte.

```json
{"current": "2", "keys": {"1": "<32 Byte base64url>", "2": "<32 Byte base64url>"}}
```

Eine neue Fassung anlegen (legt die Datei an, wenn sie fehlt, und macht die Fassung zur aktuellen;
die alten bleiben):

```bash
docker run --rm -v "$PWD/secrets:/run/secrets" --user 65532:65532 \
  ghcr.io/slouchcat/relay-push:1.0.0 -add-key 2 -keyfile /run/secrets/sealing-keys.json
docker compose up -d --force-recreate push
```

Eine alte Fassung erst entfernen, wenn keine Adresse mit ihr mehr in Gebrauch ist: Jede App meldet
sich beim Start neu an und bekommt eine Adresse der aktuellen Fassung. Wer die Schlüsseldatei
verliert, macht alle Push-Adressen unbrauchbar, bis sich jede App neu angemeldet hat. Darum:
**die Schlüsseldatei sichern** und beim Umzug mitnehmen.

## 4. Schnittstellen und Antworten

Die verbindlichen Formate stehen in Konzept 27 und im Logbuch des Baus («Protokolle»); kurz:

**Vermittlung:** `POST /v1/pair` `{"pickupHash"}` → 201 `{"id","expiresAt"}`;
`PUT /v1/pair/{id}` `{"packet"}` → 204, 409 (liegt schon eines), 410 (schon abgeholt), 404
(unbekannt oder abgelaufen), 413 (über 4 KB); `GET /v1/pair/{id}` mit
`Authorization: Pickup <Abholgeheimnis>` → 200 `{"packet"}`, 204 (noch keines), 403 (falsches
Geheimnis), 410, 404; `DELETE /v1/pair/{id}` → 204; `GET /tv` Hilfeseite; `GET /v1/healthz` → 200.
Der Fernseher fragt alle 2 Sekunden nach.

**Push-Weiterleitung:** `POST /v1/register` `{"fcmToken","vapidKey"}` → 200 `{"endpoint"}`;
`POST /v1/push/<Fassung>.<Blob>` (Web-Push) → 201, 400 (`TTL` fehlt), 403 (VAPID), 413, 415
(nicht `aes128gcm`), 429, **410 nur, wenn Google das Token nicht mehr kennt**, 503 bei jedem
eigenen Fehler, auch wenn die Unterschrift stimmt, ihr `aud` aber in `RELAY_PUSH_ORIGINS` fehlt.
404 kommt auf diesem Weg nie vor, weil ein Server auch bei 404 das Abo löscht.

Fehlerkörper beider Dienste: `{"error":"<code>"}`.

## 5. Umzug eines Dienstes

Der Umzug besteht nur aus Starten und Umschalten (E-21). Am Beispiel der Push-Weiterleitung; die
Vermittlung zieht gleich um, ohne Geheimnisse.

1. **Den Mietserver vorbereiten** (Abschnitt 1.2, Punkt 1).
2. **Kopieren und starten:** den Ordner des Dienstes, `.env` aus `env.rented.example`, und bei der
   Push-Weiterleitung `secrets/firebase.json` und `secrets/sealing-keys.json` vom alten Ort. In
   `.env` den Probenamen ergänzen, zum Beispiel `push-probe.slouchcat.com`:
   `RELAY_NAMES=push.slouchcat.com, push-probe.slouchcat.com` und
   `RELAY_PUSH_ORIGINS=https://push.slouchcat.com,https://push-probe.slouchcat.com`. Dann
   `docker compose up -d`.
3. **Unter dem Probenamen prüfen.** Ein DNS-Eintrag für den Probenamen zeigt auf den Mietserver;
   der TLS-Abschluss holt dafür ein Zertifikat, was zugleich zeigt, dass das Ausstellen auf dieser
   Maschine klappt. Dann von einem beliebigen Rechner:

   ```bash
   task relay:build
   relay/bin/paircheck -url https://pair-probe.slouchcat.com
   relay/bin/pushcheck -register-url https://push.slouchcat.com -url https://push-probe.slouchcat.com \
     -token "$PUSHCHECK_TOKEN"
   ```

   Das Token eines Testgeräts kommt aus der Variable `PUSHCHECK_TOKEN` (ohne `-token` liest
   `pushcheck` sie selbst); so landet es nicht in der Shell-Historie.

   `pushcheck` lässt die Adresse am alten Ort versiegeln und schickt die Meldung an den neuen; das
   gelingt nur mit dem mitkopierten Schlüssel. Gegen das echte Google braucht es mit `-token` das
   Token eines echten Testgeräts: Ein erfundenes
   Token weist Google ab, und die Probe-Meldung endet dann mit 503. Das Gerät bekommt eine Meldung,
   die die App verwirft, weil sie sie nicht entschlüsseln kann. Ob sie ankommt, prüft `pushcheck`
   nur mit `-fcmfake`. Die echte App und ein echter Server taugen für die Probe nicht: Die App kennt
   nur den festen Namen, und ein Server sendet nur an ihn mit der kleinen Satzgrösse.
4. **Den DNS-Eintrag des festen Namens** auf den Mietserver umstellen. Die kurze Lebensdauer im DNS
   ist von Anfang an gesetzt (F-37).
5. **Warten, bis am alten Ort nichts mehr ankommt.** Die Logs zeigen keine Adressen; mit
   `RELAY_LOG_LEVEL=debug` schreibt der alte Dienst eine Zeile je Anfrage.
6. **Aufräumen:** den Container am alten Ort entfernen, den Probenamen aus `.env` und aus dem DNS
   nehmen, `docker compose up -d`. **Den Eintrag des Namens im alten Reverse-Proxy nicht löschen,**
   sondern einige Tage lang mit 503 und `Retry-After` antworten lassen. Ein Server, der den alten
   DNS-Eintrag noch zwischengespeichert hat, landet sonst beim Standard-404 des Proxys und löscht das
   Abo (Sicherheits-Review M-3). Mit 503 wiederholt er und erreicht nach dem Ablauf seines DNS den
   neuen Ort. Der Serverbau löscht für Adressen der Push-Weiterleitung nur noch bei 410 (ADR 0044);
   der Eintrag mit 503 bleibt trotzdem, weil ältere Server bei 404 löschen.

**Das Zertifikat beim Umschalten (F-41, Empfehlung: die Lücke hinnehmen).** Für den festen Namen
bekommt der Mietserver erst ein Zertifikat, wenn der DNS-Eintrag auf ihn zeigt, also nach Schritt 4.
Bis der TLS-Abschluss es geholt hat, meist Sekunden bis wenige Minuten, scheitern Verbindungen,
die schon beim Mietserver ankommen. Eine Kopplung braucht dann einen neuen Code, und der Server
wiederholt eine Meldung von sich aus. Wer die Lücke schliessen will, braucht einen DNS-Nachweis und
dafür ein weiteres Geheimnis auf dem Mietserver; das ist nicht eingerichtet.

**Was beim Umschalten geschieht**, solange alter und neuer Ort nebeneinander gelten:

- **Vermittlung:** Handy und Fernseher können bei verschiedenen Instanzen landen. Die Kopplung
  scheitert dann einmal (404 beim Ablegen) und klappt mit einem neuen Code.
- **Push-Weiterleitung:** Beide Instanzen arbeiten nebeneinander, wenn die Schlüsseldatei mit
  umgezogen ist. Fehlt sie oder ist sie eine andere, antwortet die neue Instanz mit 503 und löscht
  nichts; `/v1/healthz` meldet 503, und die Überwachung schlägt an. Die Meldungen bleiben aus, bis
  die richtige Datei am Ort ist; die Server wiederholen sie bis zu fünfmal.

## 6. Überwachung

`relay/cmd/watch` fragt `/v1/healthz` beider Dienste ab und schickt bei einem Ausfall und bei der
Erholung je eine Meldung: ein `POST` mit Text im Körper und dem Kopf `Title` an eine einstellbare
Adresse. Es läuft **ausserhalb** des Netzes der Dienste, sonst fällt es mit ihnen aus.

```bash
relay/bin/watch -target pair=https://pair.slouchcat.com -target push=https://push.slouchcat.com \
  -notify https://notify.example.test/relay -interval 1m -failures 2
```

`-failures 2` meldet einen Ausfall erst nach zwei Fehlversuchen in Folge. `-once` prüft einmal und
endet mit 1, wenn ein Dienst nicht antwortet (für einen Zeitplan des Systems; meldet dann aber
keine Erholung). Die Push-Weiterleitung meldet 503, solange ein Geheimnis fehlt; das gilt hier als
Ausfall.

## 7. Was ein Dienst sieht (Konzept 27, 7.6)

| Wer | Sieht | Sieht nicht |
|---|---|---|
| Vermittlung | Internet-Adressen von Fernseher und Handy, Zeitpunkt, Geheimtext | Serveradresse, Gutschein, Konto |
| Push-Weiterleitung | Internet-Adresse des Slouchcat-Servers, Zeitpunkt und Häufigkeit der Meldungen, Google-Token des Handys, Geheimtext; beim Anmelden einer App (`/v1/register`) auch die Internet-Adresse des Handys | Titel und Text der Meldung, Konto |
| Der Proxy vor einem Dienst | dasselbe wie der Dienst dahinter | dasselbe wie der Dienst dahinter |
| Google (Push) | Google-Token, Zeitpunkt, Geheimtext | Titel und Text, Serveradresse |

Gespeichert wird davon nichts (D-1). Adressen hält ein Dienst nur für die Ratenbegrenzung im
Arbeitsspeicher (D-5). Die Logs maskieren Adressen, Treffpunkt-Kennungen, Push-Adressen, Token und
Schlüssel; der Redaktions-Handler steht in `relay/internal/logging`.

## 8. Lokale Probe

Beide Dienste lassen sich mit ihren Compose-Dateien lokal starten, die Push-Weiterleitung gegen den
Nachbau von Google (`fcmfake`, nie ausgeliefert). Der Nachbau läuft als zusätzlicher Dienst aus
einer zweiten Compose-Datei, die nur für die Probe entsteht; er braucht `RELAY_FCM_BASE_URL` und
eine von ihm geschriebene `firebase.json`. Eine zweite Instanz mit dem Profil `tls`, einem
Probenamen, `RELAY_TLS=internal` und anderen Ports stellt den Umzug im Kleinen nach; die
Prüfprogramme erreichen sie mit `-connect 127.0.0.1:<Port>` und `-ca` (Wurzel der lokalen CA aus
`/data/caddy/pki/authorities/local/root.crt` im TLS-Abschluss).

Nur nachgestellt ist dabei: der DNS-Eintrag (ersetzt durch `-connect`), das öffentliche Zertifikat
(lokale CA statt ACME), die zweite Maschine (ein zweiter Stack auf demselben Rechner), Google
(der Nachbau) und Port 443 (andere Ports).

Am 21. November 2026 belegt, mit dem TLS-Abschluss im Netz des Hosts:

- Fünf Anfragen von einer globalen IPv6-Adresse des Rechners durch den TLS-Abschluss bekommen 200,
  die sechste von einer zweiten Adresse im selben /64 bekommt 429 (Vorrat 5): Der Dienst zählt die
  echte IPv6-Adresse je /64. Eine Anfrage über IPv4 und eine über `::1` bekommen danach 200, also
  zählen sie getrennt und nicht unter dem Gateway.
- Keine Warnung «untrusted peer»: Das Gateway als vertrauter Proxy stimmt.
- Nach einem fehlerhaften TLS-Handshake, einer Anfrage mit unbekanntem Namen und einer Anfrage bei
  gestopptem Dienst (502, nie 404) steht im Log des TLS-Abschlusses keine IP-Adresse.

## 9. Abnahme nach dem Einrichten

Diese Belege aus Konzept 27 Abschnitt 13 (Zeile «2 Dienste») brauchen Marcos Infrastruktur und
stehen nach dem Bau noch aus. Jeder Schritt nennt den Befehl und was man sehen muss.

1. **Beide Container hinter dem vorhandenen Reverse-Proxy** (Abschnitt 1.1), dann von einem Rechner
   ausserhalb des eigenen Netzes:

   ```bash
   curl -s https://pair.slouchcat.com/v1/healthz     # {"status":"ok"}
   curl -s https://push.slouchcat.com/v1/healthz     # {"status":"ok"}
   curl -sI https://pair.slouchcat.com/tv | grep -i content-security-policy   # default-src 'none'; …
   ```

   Zu sehen: zweimal `ok`; die Hilfeseite mit der strengen CSP. Steht im Log eines Dienstes «peer
   that is not a trusted proxy», ist `RELAY_TRUSTED_PROXIES` falsch.
2. **Zwei Prüfprogramme koppeln über die laufende Vermittlung:**

   ```bash
   relay/bin/paircheck -url https://pair.slouchcat.com
   ```

   Zu sehen: `paircheck: 21 checks passed against https://pair.slouchcat.com`.
3. **Eine Push-Probe kommt an,** mit echtem Firebase-Projekt und einem Testgerät, auf dem eine
   Entwicklungsfassung der App angemeldet ist:

   ```bash
   PUSHCHECK_TOKEN=<Google-Token des Testgeräts> relay/bin/pushcheck -url https://push.slouchcat.com
   ```

   Zu sehen: `pushcheck: 7 checks passed` (ohne `-fcmfake` fallen die Prüfungen am Nachbau weg),
   201 für die Probe-Meldung. Das Testgerät bekommt eine Datennachricht, die die App verwirft (sie
   ist für einen Schlüssel von `pushcheck` verschlüsselt). Dass eine echte Meldung ankommt und
   angezeigt wird, belegt Stufe 8 mit einem Server.
4. **Ein abgeschalteter Dienst löst die Meldung an Marco aus:** `watch` auf einem Rechner ausserhalb
   des Netzes der Dienste starten (Abschnitt 6) mit der echten Meldeadresse, dann am Dienst
   `docker compose stop pair`. Zu sehen: nach zwei Intervallen «Slouchcat relay pair is DOWN»;
   nach `docker compose start pair` «… is back up».
5. **Umzugsprobe auf einer zweiten, von aussen erreichbaren Maschine:** die Schritte 1 bis 3 aus
   Abschnitt 5 mit einem Probenamen, dann:

   ```bash
   relay/bin/paircheck -url https://pair-probe.slouchcat.com
   PUSHCHECK_TOKEN=<…> relay/bin/pushcheck -register-url https://push.slouchcat.com \
     -url https://push-probe.slouchcat.com
   ```

   Zu sehen: wie in 2 und 3; der TLS-Abschluss hat für den Probenamen selbst ein Zertifikat geholt.
   Danach den Schlüssel `sealing-keys.json` am neuen Ort kurz wegnehmen und
   `docker compose up -d --force-recreate push`: `pushcheck … -expect-unavailable` zeigt 503, und
   `/v1/healthz` meldet `no_sealing_key`. Den Schlüssel zurücklegen, neu starten, dann die Schritte
   4 bis 6 aus Abschnitt 5.
