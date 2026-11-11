# Messwerte: Konzept 19 Teil B «Radio-Angebot», Stufe B0

Konzept `docs/planning/19-podcasts-und-radio.md` B7.2, B7.5, B8 und B12 (Stufe B0), ADR 0030. Gemessen auf
CT 190 (Proxmox-CT, 4 GB RAM, Docker; Image `quellen-1cd8f01`, Stand von Release 1.26.0) mit dem Katalog,
den der Server dort am 11. November 2026 hielt.

## 1. Zahlen aus dem echten Index (11. November 2026)

Abfrage lesend auf der Datenbank von CT 190 (`?mode=ro`):

| Grösse | Wert |
|---|---|
| Sender nach dem Zusammenführen | 55'953 (alle aus dem Katalog, 0 eigene) |
| Länder mit Sendern | 228 |
| Sender ohne Land | 1'099 |
| nicht erreichbar (`reachable = 0` oder `broken_at`) | 806 |
| mit Logo im Cache | 2'700 (1'402 aus dem Katalog, 1'298 von der Webseite) |
| Logo-Kette gescheitert | 1'365 |
| ausgeblendet | 9 |
| Favoriten | 8 (8 Sender) |

Je Land (Katalogsender; «Logo-Adresse» = `favicon` im Katalog):

| Land | Sender | nicht erreichbar | Logo im Cache | mit Webseite | mit Logo-Adresse |
|---|---|---|---|---|---|
| US | 7'325 | 140 | 261 | 7'163 | 4'539 |
| DE | 5'226 | 38 | 312 | 4'942 | 2'887 |
| RU | 2'772 | 10 | 117 | 2'611 | 1'783 |
| MX | 2'760 | 33 | 77 | 2'755 | 2'219 |
| FR | 2'750 | 21 | 141 | 2'692 | 1'613 |
| GR | 2'052 | 20 | 80 | 2'035 | 1'541 |
| GB | 1'981 | 43 | 95 | 1'949 | 1'344 |
| CN | 1'979 | 7 | 27 | 1'962 | 1'249 |
| IT | 1'589 | 18 | 92 | 1'559 | 841 |
| CH | 502 | 3 | 129 | 480 | 241 |
| AT | 298 | 10 | 14 | 291 | 187 |
| LI | 3 | 0 | 0 | 3 | 1 |

## 2. Speicher je Logo (Bestand auf CT 190)

Was gespeichert wird: Das heruntergeladene Original bleibt **nicht** liegen. Die Logo-Kette rechnet es in sechs
Dateien um (Quadrat mit 96, 256 und 512 Pixeln, je WebP und PNG) unter `cache/images/radio/<id>/`; in der Datenbank
stehen je Sender nur `logo_state` und `logo_hash` (64 Zeichen). Über die 2'700 Logos, die CT 190 am
11. November 2026 schon hatte (`find -printf %s`), in Byte:

| Datei | Mittel | Median | 90 % |
|---|---|---|---|
| `logo-w96.webp` | 2'227 | 1'876 | 3'960 |
| `logo-w96.png` | 8'956 | 6'770 | 18'537 |
| `logo-w256.webp` | 7'157 | 5'418 | 14'166 |
| `logo-w256.png` | 44'291 | 29'016 | 103'055 |
| `logo-w512.webp` | 15'826 | 11'272 | 33'136 |
| `logo-w512.png` | 148'772 | 92'803 | 366'846 |
| **alle sechs Dateien** | **227'229** | | |

Zusammen 599 MB Dateigrösse (658 MB auf der Platte) für 2'700 Logos. Das gespeicherte 96-Pixel-Logo selbst ist
klein (WebP im Median 1,9 KB, PNG 6,8 KB); zwei Drittel des Platzes belegt das PNG mit 512 Pixeln. Das Papier
schätzte «wenige KB je Sender» (B8); das gilt nur für die 96- und 256-Pixel-WebP-Dateien.

## 3. Die Logo-Kette auf CT 190

Messweg: `server/internal/radio/logo_measure_test.go` (Build-Tag `radiomeasure`, läuft nie in einem normalen
Testlauf). Das Testprogramm läuft in einem eigenen Container aus dem Image, das auf CT 190 läuft (gleiches ffmpeg,
gleiches Netz), auf einer frischen Kopie der Datenbank je Lauf, mit leerem Cache. Es führt die echte Logo-Kette
(`handleLogo`: Logo-Adresse des Katalogs, Symbole der Webseite, mindestens 96 Pixel, Umrechnen in sechs Dateien)
für alle Katalogsender eines Landes aus, nach Beliebtheit, mit vier Workern wie die Warteschlange, der gemeinsamen
Drossel und der Sperre je Host; ein Auftrag, dessen Host belegt ist, kommt wie in der Warteschlange nach der
Wartezeit wieder dran. Gezählt werden Abrufe (jede Weiterleitung einzeln), Byte, Antworten je Status, Dauer je
Auftrag, Wartefälle wegen der Sperre und die Dateien im Cache.

```
cd server && CGO_ENABLED=0 go test -c -tags radiomeasure -o radiomeasure.test ./internal/radio
# auf CT 190: /root/radiomeasure/radiomeasure.sh (Kopie der DB, ein Lauf je Land und Drossel)
docker run --rm -v /root/radiomeasure:/m -e MEASURE_CONFIG=… -e MEASURE_CACHE=… -e MEASURE_COUNTRY=CH \
  -e MEASURE_RATE=4 -e MEASURE_WORKERS=4 -e MEASURE_FFMPEG=/usr/local/bin/ffmpeg --entrypoint /m/radiomeasure.test \
  ghcr.io/slouchcat/slouchcat:quellen-1cd8f01 -test.run TestMeasureLogoChain -test.v
```

### 3.1 Erste Läufe: zwei Befunde

**Befund 1, Abruf ohne Ende.** Der erste Lauf (Schweiz, 2 je Sekunde) blieb nach 501 von 502 Sendern stehen: Der
Sender «OnFlow Hot» nennt als Webseite `https://api.zeno.fm/mounts/metadata/subscribe/…`, einen Ereignis-Stream,
der nie endet. Der Abruf las ohne Zeitgrenze weiter, hielt den Worker und die Sperre des Hosts `api.zeno.fm`. Das
gab es schon vor B1 (es hielt einen Worker fest); mit der Sperre je Host hätte es zusätzlich alle Aufträge dieses
Hosts aufgehalten. Behoben: jeder Abruf der Logo-Kette hat eine Zeitgrenze von 20 Sekunden samt Körper
(`logoFetchTimeout`, Test `TestLogoChainEndlessHomepage`).

**Befund 2, Sperre je Host ohne Streuung.** Mit der Zeitgrenze, aber noch mit fester Wartezeit von 15 s:

| Lauf | Dauer | Abrufe | Byte | Auftrag p50 / p90 / max | Wartefälle | Logos |
|---|---|---|---|---|---|---|
| CH, 2 je Sekunde | 14 min 9 s | 1'470 | 74,0 MB | 2,2 s / 6,5 s / 30,4 s | 661 | 378 von 502 (75 %) |
| CH, 4 je Sekunde | 14 min 1 s | 1'466 | 73,6 MB | 1,3 s / 3,2 s / 20,9 s | 1'121 | 377 von 502 (75 %) |

Mit 4 je Sekunde waren 90 % der Sender nach 4 Minuten fertig (mit 2 je Sekunde nach 7), der Rest dauerte in beiden
Läufen bis 14 Minuten. Ursache: 85 Schweizer Sender von 1.FM teilen den Host `www.1.fm` (danach `www.srf.ch` 37,
`www.traxx.fm` 21, `www.my105.ch` 19). Alle wartenden Aufträge eines Hosts wachten nach denselben 15 s auf, einer
nahm die Sperre, die anderen warteten wieder: der Host lief einen Auftrag je 15 s. Behoben: die Wartezeit ist 15 s
plus bis zu 15 s Zufall (`retryDelay`).

Antworten im Lauf mit 4 je Sekunde: 931 × 200, 447 Weiterleitungen (301, 302, 308), 62 × 404, 6 × 400, 1 × 403,
19 Verbindungsfehler; **keine Antwort 429**, kein Sender hat sich über die Drossel beschwert.

### 3.2 Mit gestreuter Wartezeit (`e2a99e5e`)

| Lauf | Dauer | Abrufe | Byte | Auftrag p50 / p90 / max | Wartefälle | Logos |
|---|---|---|---|---|---|---|
| CH, 4 je Sekunde | **5 min 50 s** (vorher 14 min 1 s) | 1'466 | 73,4 MB | 1,3 s / 3,2 s / 20,8 s | 233 (vorher 1'121) | 378 von 502 (75 %) |

Antworten: 933 × 200, 445 Weiterleitungen, 62 × 404, 6 × 400, 1 × 403, 19 Verbindungsfehler, keine 429.

**Deutschland, 4 je Sekunde** (abgebrochen nach 85 Minuten, weil CT 190 um 11:32 für die Abnahme von Teil A neu
startete): 5'012 von 5'226 Sendern fertig, 12'161 Abrufe, **38'369 Wartefälle**. Nach rund 40 Minuten waren alle
Sender ausser denen von `laut.fm` fertig: 509 deutsche Sender haben dort Logo-Adresse oder Webseite (dazu 91 auf
`assets.laut.fm`). Die Sperre je Host lässt sie nacheinander laufen; ein Auftrag dauert dort rund 15 s, so kam der
Rest mit rund 4 Sendern je Minute voran.

**Befund 3, zu viele Rückstellungen.** Die Streuung von 15 bis 30 s kannte die Zahl der Wartenden nicht: die rund
300 wartenden Aufträge von `laut.fm` kamen etwa siebenmal je Sekunde zurück. Im Server ist jede Rückstellung ein
Schreiben der Warteschlange und eine Zeile im Log. Behoben (`91096131`): die Wartezeit ist 15 s plus ein Zufall bis
15 s und 2 s je Auftrag, der auf denselben Host wartet; bei 300 Wartenden verteilen sich die Rückkehrer über rund
10 Minuten, statt alle 22 s wiederzukommen. Ein Lauf für Deutschland mit dieser Streuung folgt nach der Abnahme von
Teil A auf CT 190.

## 4. Speicher nach Marcos Entscheid: PNG nur in 96 Pixeln (11. November 2026)

Befund aus Abschnitt 2 und 3: Das gespeicherte 96-Pixel-Logo ist klein, zwei Drittel des Platzes belegt das PNG
mit 512 Pixeln. Marco hat entschieden, das in B1 zu ändern: gespeichert werden WebP in 96, 256 und 512 Pixeln und
PNG nur in 96 Pixeln; ein PNG in 256 oder 512 Pixeln entsteht bei der Anfrage aus dem WebP derselben Breite
(`pngFromWebP`), der Start räumt die alten PNG weg (`removeStoredLargePNGs`). Vorher und nachher, gerechnet aus den
Dateien des Messlaufs Schweiz (378 Logos) und des Bestands auf CT 190 (2'700 Logos, Abschnitt 2):

| | Dateien je Logo | Schweiz, Summe | je Logo (Mittel) | Bestand CT 190 |
|---|---|---|---|---|
| vorher | 6 (WebP und PNG in 96, 256, 512) | 59,9 MB | 158,5 KB | 613 MB |
| nachher | 4 (WebP in 96, 256, 512; PNG in 96) | 9,6 MB | 25,5 KB | 92 MB |
| Ersparnis | | 84 % | | 85 % |

Summen der Schweiz nachher, in Byte: `logo-w96.webp` 696'970, `logo-w96.png` 2'608'486, `logo-w256.webp` 2'021'254,
`logo-w512.webp` 4'324'870. Hochgerechnet auf Deutschland (5'226 Sender, rund drei Viertel mit Logo) rund 100 MB
statt 600 MB. Ein PNG aus dem WebP kostet einen Aufruf von ffmpeg je Anfrage; das trifft nur Clients ohne WebP, und
die Adresse trägt den Logo-Hash, so dass der Client es zwischenspeichert. Höchstens zwei Umrechnungen laufen
gleichzeitig. Das PNG in 256 und 512 Pixeln entsteht aus dem WebP (Qualität 82), nicht mehr aus dem Original; es
trägt damit die Spuren der WebP-Kompression.

## 5. Wikidata (Stufe B2, 11. November 2026)

Gemessen auf dem Arbeitsrechner gegen `query.wikidata.org`, abgeglichen mit dem echten Katalog von CT 190
(55'953 Sender, nur gelesen über `?mode=ro`, nichts auf CT 190 geschrieben).

**Abfrage.** Radiosender (`Q14350` und Unterklassen) mit Webseite (P856), Eigentümer (P127) und Gründung (P571)
samt Genauigkeit, nur Aussagen des besten Rangs:

| Form | Dauer | Grösse | Zeilen |
|---|---|---|---|
| CSV, ohne Bezeichnungen (gewählt) | 2,5 s | 2,0 MB | 20'072 Zeilen, 19'229 Sender |
| JSON, ohne Bezeichnungen | 1,7 s | 7,3 MB | gleich |
| CSV mit Bezeichnungsdienst für die Eigentümer | 17,2 s | 2,2 MB | gleich |
| JSON mit `rdfs:label` de/en für die Eigentümer | 12,3 s | 8,8 MB | gleich |

Die Bezeichnungen der Eigentümer holt darum eine zweite, kleine Abfrage nur für die Eigentümer der Treffer im
Angebot (`VALUES`, je 500, als POST). Die Genauigkeit der Gründung: 11'453 Jahr, 94 Monat, 1'561 Tag.

**Treffer** nach der gebauten Regel (Adresse ohne `www.`, Pfad ohne Schrägstrich am Ende; sonst Host, ausser einem
Host mit mehr als 100 Senderadressen im Katalog; zwei Einträge für Adresse oder Host zählen nicht):

| Land | Sender | mit Webseite | Treffer | davon ein Eigentümer | mehrere Eigentümer | mit Gründung |
|---|---|---|---|---|---|---|
| Schweiz | 502 | 480 | 106 (21 %) | 47 | 0 | 78 |
| Deutschland | 5'226 | 4'942 | 1'647 (32 %) | 374 | 88 | 939 |
| Österreich | 298 | 291 | 123 (41 %) | 43 | 0 | 79 |
| USA | 7'325 | 7'163 | 2'043 (28 %) | 888 | 10 | 1'572 |
| ganzer Katalog | 55'953 | 53'391 | 9'879 (18 %) | 4'015 | 183 | 6'543 |

Nur über den Host allein wären es in der Schweiz 78 Treffer, nur über die ganze Adresse 89; beides zusammen 106.
Ein Wikidata-Eintrag trifft oft mehrere Katalogsender: «Radio SRF 1» elf, «Radio Swiss Classic» 13 (Varianten
und Sprachen desselben Senders), richtig zugeordnet. **Plattformen:** `zeno.fm` hat einen eigenen Eintrag und
514 verschiedene Senderadressen im Katalog; ohne Sperre hätten alle 576 Sender von `zeno.fm` dessen Eigentümer und
Gründung bekommen. Über 100 Adressen haben elf Hosts (`laut.fm` 538, `zeno.fm` 514, `facebook.com` 357, …); der
grösste echte Sender darunter ist `somafm.com` mit 54. Für diese elf gilt nur die ganze Adresse.

**Last im Betrieb:** zwei bis drei Abrufe je Lauf (ein Abruf der Sender, ein Abruf der Bezeichnungen je 500
Eigentümer), höchstens einer je Sekunde, wöchentlich und nach «Übernehmen» mit neuen Sendern höchstens einmal am
Tag. Antwort höchstens 20 MB, Zeitgrenze 2 Minuten je Abruf.
