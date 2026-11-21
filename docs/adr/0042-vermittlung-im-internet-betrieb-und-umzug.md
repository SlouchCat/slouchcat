# ADR 0042: Vermittlung im Internet; Betrieb und Umzug der zwei Dienste

- Status: gebaut auf dem Zweig `claude/k27-stufe2-a` (21. November 2026, Stufe 2 von Konzept 27),
  **angenommen mit dem Merge**. Die Zahl ist in Konzept 26 reserviert; Konzept 27 hat den Inhalt
  um den Betrieb beider Dienste erweitert (K27 18.1).
- Quelle: Konzept `docs/planning/27-android-handy-app.md`, Abschnitte 1.3, 7.1, 7.2, 7.4 bis 7.7,
  Fragen F-21, F-37, F-39, F-41 mit ihren Empfehlungen; Marcos Entscheide E-11, E-19, E-21, E-22;
  Konzept 26 Abschnitt 4.3; Logbuch des Baus («Festlegungen», «Protokolle»)
- Kein Vertrag des Servers, keine Migration. Eigene Fassung in den Pfaden (`/v1`, D-8), eigene
  Release-Tags `relay-vX.Y.Z`
- Harte Regeln: 3 (kein `os/exec`), 10 (Redaktion), 11 (Ausnahme vom Nicht-Ziel «Cloud» seit
  PR #192)

## Kontext

Ein Fernseher soll sich einrichten lassen, indem das Handy seinen QR-Code scannt (E-10, E-11). Handy
und Fernseher stehen oft in verschiedenen Netzen, und der Server des Nutzers ist nicht immer von
beiden erreichbar. Es braucht einen Treffpunkt im Internet, den Marco für alle Nutzer betreibt.
Daneben braucht die Handy-App einen zweiten Dienst für Push (ADR 0044). Marco hat entschieden: zwei
Dienste, vorläufig je auf einem eigenen Container auf eigener Hardware, später leicht auf zwei
Mietserver verschiebbar (E-21), und solange sie auf eigener Hardware laufen, über den vorhandenen
Reverse-Proxy erreichbar (E-22). Offen war, ob das zwei Programme oder ein Programm mit zwei
schaltbaren Teilen sind (F-39), und wie Betrieb und Umzug im Einzelnen aussehen.

## Entscheid

1. **Ein Go-Modul `relay/` mit zwei Programmen und zwei Images** (F-39, Empfehlung). `relay/cmd/pair`
   ist die Vermittlung, `relay/cmd/push` die Push-Weiterleitung. Gemeinsam sind nur die Pakete für
   die Grenzen je Aufrufer, die Adresse hinter einem Proxy, die Logs mit Redaktion und den Rahmen
   des HTTP-Servers. depguard (`relay/.golangci.yml`) hält den Code für den Firebase-Schlüssel und
   den Schlüssel zum Versiegeln aus der Vermittlung heraus. Eigenes `go.mod`, dieselbe Go-Fassung
   wie `server/`, kein `go.work`, keine Abhängigkeit ausser der Standardbibliothek.
2. **Protokoll der Vermittlung** wie in K27 7.1 und im Logbuch («Protokolle»): Treffpunkt mit 128
   Bit Zufall, fünf Minuten, nur im Arbeitsspeicher; Paket bis 4096 Byte; Abholen und Abbrechen nur
   mit dem Abholgeheimnis im Kopf `Authorization: Pickup <…>`. Der Dienst kennt nur dessen SHA-256
   über die 32 Rohbytes und vergleicht in konstanter Zeit. Nach dem Abholen merkt er sich bis zum
   Ablauf nur «verbraucht»; Ablegen antwortet 204, 409, 410 oder 404. Ein falsches Geheimnis ist
   403. Obergrenzen: offene Treffpunkte gesamt (503) und je Aufrufer (429), Ratenbegrenzung je
   Aufrufer (429). Der Fernseher fragt alle 2 Sekunden nach; eine offen gehaltene Anfrage gibt es
   nicht, weil sie durch Proxys und Zeitlimits mehr Fehlerfälle bringt als sie spart.
3. **Die Hilfeseite des QR-Codes liefert die Vermittlung selbst** (`GET /tv`, Abweichung A-1 des
   Baus): statisch, ohne Skript, mit `Content-Security-Policy: default-src 'none'` und der einen
   Stilangabe über ihren Hash, `Referrer-Policy: no-referrer`, Text in Deutsch oder Englisch nach
   `Accept-Language`. Der Teil hinter `#` erreicht keinen Server und kein Skript.
4. **Adresse des Aufrufers** nur aus `X-Forwarded-For` eines eingetragenen Proxys, von hinten
   gelesen; der erste Eintrag, der kein eingetragener Proxy ist, zählt (D-9). IPv6 zählt je /64
   (D-2). Die Adresse lebt nur im Arbeitsspeicher; der Redaktions-Handler maskiert Adressen,
   Treffpunkt-Kennungen und Push-Adressen auch in Fehlermeldungen (D-5).
5. **Betrieb: dieselbe Compose-Datei an beiden Orten.** Je Dienst eine Compose-Datei unter
   `deploy/relay/<dienst>/` mit Vorlagen `env.home.example` und `env.rented.example`. Die
   Umgebungsdatei setzt die Bindeadresse des Ports und schaltet mit `COMPOSE_PROFILES=tls` den
   TLS-Abschluss im selben Stack ein. Er hat eine feste Adresse im Netz des Stacks; nur sie ist der
   vertraute Proxy. Auf dem Mietserver ist der Port des Dienstes an `127.0.0.1` gebunden.
6. **Umzug in sechs Schritten** (K27 7.5) mit den Prüfprogrammen `paircheck` und `pushcheck` unter
   einem Probenamen. Für das Zertifikat des festen Namens gilt die Empfehlung zu F-41: die kurze
   Lücke nach dem Umstellen des DNS hinnehmen.
7. **Überwachung** mit `relay/cmd/watch`, ausserhalb des Netzes der Dienste: Meldung bei Ausfall
   und bei Erholung an eine einstellbare Adresse.
8. **Auslieferung:** Tag `relay-vX.Y.Z`, Workflow `release-relay.yml`, Images
   `ghcr.io/slouchcat/relay-pair` und `relay-push` für amd64 und arm64. **Kein GitHub-Release**,
   weil die Update-Prüfung des Servers `releases/latest` liest.

## Verworfen

- **Ein Programm mit zwei schaltbaren Teilen** (bis Fassung 4 von K27): Mit zwei Maschinen fällt
  «eine Installation, ein Update» weg, und ein Image der Vermittlung trüge Code für den
  Firebase-Schlüssel.
- **Ein Tunnel eines Dritten** statt des vorhandenen Reverse-Proxys: von Marco abgelehnt (E-22).
- **Port des Dienstes auf dem Mietserver gar nicht veröffentlicht** (K27 7.5): Eine Compose-Datei
  kann einen Port nicht je Umgebungsdatei weglassen. Die Bindung an die Loopback-Adresse erreicht
  dasselbe Ziel, von aussen ist der Port nicht erreichbar.
- **Lange offene Abholanfrage:** siehe Entscheid 2.
- **Zertifikat vorab über einen DNS-Nachweis:** verlangte ein weiteres Geheimnis auf dem
  Mietserver (F-41).

## Folgen

- Die Vermittlung hat kein Geheimnis. Ein Einbruch dort kann Treffpunkte stören und einem
  Fernseher ein Paket unterschieben (R-19), aber nicht an Push.
- Der Fernseher und die Handy-App bauen gegen das Protokoll aus Entscheid 2; der gemeinsame
  Testvektor `testdata/crypto/pairing-v1.json` belegt die Verschlüsselung in Go und Kotlin.
- Die Dienste laufen als eigene Betriebspflicht (R-3); Anleitung in `docs/relay/BETRIEB.md`.
- CI: Job `relay` im Gate `ci`; `task ci` ruft `task ci:relay`.
- Die echte Umzugsprobe auf einer zweiten, von aussen erreichbaren Maschine steht aus; in Stufe 2
  ist sie nur lokal nachgestellt (BETRIEB.md, Abschnitt 8).
