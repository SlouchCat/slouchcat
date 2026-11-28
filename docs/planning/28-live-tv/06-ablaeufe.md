# Konzept 28 · Abläufe und Sequenzdiagramme

Stand: 29. November 2026 · Fassung 3 · Status: Entwurf nach Gesamtreview, nicht freigegeben

Dieses Dokument zeichnet, was `04-architektur.md` und `05-streaming-und-fast-zapping.md` in **Fassung 4**
festlegen: den Lebenszyklus eines Eingangsstroms, die 18 Situationen aus Auftrag Abschnitt 8.2, vier weitere
Abläufe und die Stellen, an denen zwei Dinge gleichzeitig geschehen. Es fügt **keine** Regel und keinen
Übergang hinzu. Jeder Schritt stützt sich auf einen Entscheid `T-n` (T-1 bis T-29) oder einen Abschnitt in
`04` oder `05`; Zahlen stammen aus `04` Abschnitt 14 «Massgebliche Werte». Wo `12-sicherheit.md` Fassung 3
strenger ist (feste Ziele, Schauzeit, Grenze von T-29), gilt `12`. **Solange Marco eine Frage nicht
beantwortet hat, zeichnen die Abläufe die wortlauttreue Vorgabe**, bei Sicherheitsfragen die sichere;
Empfehlungen und Varianten stehen nur als Variante dabei (Abschnitt 6). Alles hier ist **spezifiziert**,
nichts ist gebaut. Stufen nach `18-umsetzungsplan.md` (V, 0, 1a, 1b, 2, 3, H, 4 Timeshift, 5 Aufnahmen, 6, L,
W, E).

Kennungen wie in `04`: `T-n` tragende Entscheide, `F-n` Fragen an dich aus `04` (fremde Dateien mit
Dateinummer, etwa «`03` F-7»), `O-n` offene Punkte aus `04` und `05`, `M-n` Messpunkte, `DR-n` Detailregeln
aus `03`, `LTV-nnn.mm` Anforderungen aus `03`. Zahlen (Sekunden, Grössen) sind die Annahmen aus `04` und `05`,
keine Zusagen.

Begriffe, einmal erklärt: Ein **Eingangsstrom** ist der eine Empfang eines Senders von seiner Quelle. Ein
**Haltegrund** ist ein Eintrag «wer braucht diesen Strom gerade» (Art und Besitzer); solange einer da ist,
läuft der Strom. **Karenz** ist eine kurze Nachlaufzeit ohne Haltegrund, damit ein Wiedereinstieg sofort Bild
hat. Der **Ring** ist der technische Startpuffer im Verteiler (15 Sekunden; bei langen Segmenten das Vierfache der Segmentdauer, höchstens 30 Sekunden). Der
**Sitzungspuffer** ist der persönliche Zeitversatz-Puffer einer Sitzung (nur mit Recht «Timeshift»).

## 1. Teilnehmer

### 1.1 Namen in allen Diagrammen

Die Teilnehmer folgen den Paketen und Bausteinen aus `04`, Abschnitt 3. Ein Diagramm zeigt nur die
Teilnehmer, die darin vorkommen, aber immer mit demselben Kürzel und Namen.

| Kürzel | Name im Diagramm | Paket (`04` 3.2) | Aufgabe in den Abläufen |
|---|---|---|---|
| `CL` | Client | Web-App, Handy-App, TV-App | stellt ein, holt Liste und Segmente, schickt alle 10 s den Herzschlag. `CL1`, `CL2` sind zwei Clients; `CLV` ist der Client einer Person in der Verwaltung (Administrator oder Hauptprofil). |
| `API` | API und Handler | `internal/api` | JSON-Operationen über den strict server; Stream-Handler (Liste, Segment) als `http.Handler`. |
| `SES` | Sitzung | `internal/live` | Live-Sitzung (T-9): drei Schalter, Token `lt`, Einstell-Nummer, Herzschlag, Entscheid Original, Ton- oder Bild-Umwandlung (T-14). |
| `ZUL` | Zulassung | `internal/live` (Teil der Stromverwaltung) | vier Grössen (Eingangsströme, Byte-Budget, Platz, Umwandlungseinheiten), Serverdeckel, Drosselung, Rangfolge bei Mangel (T-19, `04` 8). |
| `STR` | Stromverwaltung | `internal/live` | Verzeichnis der Eingangsströme, Haltegründe, Zustandsautomat, Empfangsstrategie, Vorladen (T-17, T-18). |
| `EMP` | Empfang | `internal/live/ingest` | Empfänger je Protokoll, Wiederverbindung, eigener Dialer und Transport je Quelle (T-3, T-5). Im späten Weg W liest FFmpeg nur über **Ersatzpfade** `http://127.0.0.1:<Port>/<Zufall>/…` vom Go-Weiterleiter, ohne Proxy-Einstellung (`05` 2.6); kein Diagramm hier zeigt Weg W. |
| `VT` | Verteiler | `internal/live/hub` | Ring, GOP-Index, Tabellen, Leser, Messwerte (T-6 bis T-8). |
| `SP` | Sitzungspuffer | `internal/live/spool` | persönlicher Zeitversatz-Puffer einer Sitzung auf dem Speicherort «Timeshift» (T-10). |
| `SEG` | Segmentierer | `internal/live/spool` | schneidet Segmente, baut die Wiedergabeliste der Sitzung (T-11, T-12). |
| `UW` | Umwandlung | `internal/transcode` (erweitert), gesteuert von `live` | FFmpeg je Sitzung über Pipes, Probe, Budget mit reservieren, umbuchen, freigeben (T-13, T-27). |
| `AUF` | Aufnahme-Schreiber | `internal/live/dvr` mit `live/spool` | eigener Leser am Verteiler, schreibt Teilstücke auf den Speicherort «Aufnahmen» (T-16). |
| `PL` | Planer | `internal/live/dvr` | sekundengenauer Aufnahme-Planer im Prozess (T-1). |
| `UEB` | Überwachung | Messwerte in `live/hub`, Bewertung in `internal/live` | erkennt Stillstand, Abbruch, Parameterwechsel (`05` 9). |
| `PB` | playback-Grenzen | `internal/playback` (zwei Rückrufe) | Stream-Grenzen und Schauzeit je Konto und Profil, Startsperre (T-26). |
| `AU` | auth | `internal/auth` | Haken bei Änderung der Schalter, Abmelden, Löschen, Tageslimit (`04` 9). |
| `CAT` | catalog | `internal/live/catalog` | Quellen, Sender, Rangfolge der Ersatzquellen, entschlüsselte Adressen nur im Speicher (T-28). |
| `JOB` | Jobs | `internal/jobs` | Stapelarbeit: Listen-Abgleich, Prüfjobs (`04` 10.1). |
| `NTF` | notify | `internal/notify` | Meldungen an Administrator und Profile, entprellt (`04` 10.2). |
| `Q` | Quelle | ausserhalb | Anbieter oder Gerät im Heimnetz. `Q1`, `Q2` sind Haupt- und Ersatzquelle eines Senders. |
| `DB` | SQLite | `internal/store/liveq` | Dauerhaftes (T-22). |
| `DSK` | Datenträger | Speicherorte «Timeshift» und «Aufnahmen» (T-25) | Stückdateien, Teilstücke, Markerdatei. |

### 1.2 Farben: wer den Strom gerade braucht

Jedes Diagramm färbt die Abschnitte nach dem Haltegrund, um den es gerade geht, und nennt ihn zusätzlich in
einer Notiz (für die Ansicht ohne Farbe):

| Farbe | Bedeutung | Haltegrund (`04` 5.2) |
|---|---|---|
| Grau | Dauersender und vorsorglicher Empfang ohne Abnehmer | dauerhaft, Zeitplan; automatisch, Prüfung |
| Gelb | Vorladen eines Nachbarsenders | Vorladen |
| Blau | aktiver Zuschauer mit Live-Sitzung | Zuschauer |
| Grün | persönlicher Sitzungspuffer (Timeshift) | **kein eigener Haltegrund:** hängt am Haltegrund Zuschauer seiner Sitzung (`04` 5.2: «Ein pausierter Zuschauer hält den Strom weiter») |
| Rot | aktive Aufnahme | Aufnahme |

## 2. Lebenszyklus eines Eingangsstroms

Begriffe wie in `04` 5: **Dauersender** heisst ein Sender mit Strategie «dauerhaft» und ein Sender mit Zeitplan,
solange sein Fenster läuft. **Abnehmer** sind Zuschauer und Aufnahmen. **Vorsorglich** heissen Vorladen,
«automatisch» und Prüfungen.

### 2.1 Zustandsdiagramm

Das Diagramm ist aus `04` 5.1 (Fassung 4) abgeleitet, der vollständigen Quelle der Zustände und Übergänge.
Dieses Dokument fügt keinen Übergang hinzu. Neu gegenüber Fassung 3 ist der Zustand «Endet» (höchstens
1 Sekunde: Verbindung schliessen, Ring freigeben, Byte-Budget zurückbuchen). «Zurückgestellt» entsteht nach der
Vorgabe nur, wenn ein Dauersender **nicht starten kann**; ein laufender Dauersender weicht niemandem (F-4,
Vorgabe (b)).

```mermaid
stateDiagram-v2
  state "Ruht" as Ruht
  state "Verbindet" as Verbindet
  state "Läuft" as Laeuft
  state "Wartet auf Wiederverbindung" as Wartet
  state "Gestört" as Gestoert
  state "Karenz" as Karenz
  state "Zurückgestellt" as Zurueck
  state "Endet" as Endet

  [*] --> Ruht
  Ruht --> Verbindet: erster Haltegrund, Zulassung erteilt
  Ruht --> Zurueck: Dauersender kann nicht starten
  Verbindet --> Laeuft: Tabellen und erster Keyframe da
  Verbindet --> Wartet: Verbindung scheitert oder Frist um
  Verbindet --> Verbindet: Quelle lehnt ab, Rangniedrigeres freigegeben, neuer Versuch
  Verbindet --> Zurueck: Dauersender, Quelle lehnt weiter ab
  Laeuft --> Wartet: Abbruch oder Stillstand
  Laeuft --> Verbindet: Verdrängung durch die Quelle, dieser Strom behält die Verbindung
  Wartet --> Verbindet: Wartezeit um, oder neuer Abnehmer
  Wartet --> Gestoert: Leiter erschöpft
  Gestoert --> Verbindet: langsamer Wiederversuch, oder neuer Abnehmer
  Gestoert --> Endet: kein dauerhafter Haltegrund
  Laeuft --> Karenz: letzter Haltegrund weg, weil ein Zuschauer von sich aus geht
  Laeuft --> Endet: letzter Haltegrund weg aus jedem anderen Anlass
  Karenz --> Laeuft: neuer Haltegrund
  Karenz --> Endet: Karenzzeit um, Platz gebraucht, Quelle bricht ab oder Sender deaktiviert
  Zurueck --> Verbindet: Kriterium des Wiederanlaufs erfüllt, oder ein Abnehmer kommt
  Zurueck --> Zurueck: Wiederanlauf scheitert
  Zurueck --> Ruht: Haltegrund entfällt
  Verbindet --> Endet: kein Haltegrund mehr
  Wartet --> Ruht: kein Haltegrund mehr
  Endet --> Ruht: Verbindung geschlossen, Ring freigegeben
  Endet --> Verbindet: ein Beitritt hat gewartet

  note right of Laeuft
    Haltegründe am Strom
    dauerhaft, Zeitplan, automatisch
    Vorladen, Prüfung
    Zuschauer mit oder ohne Sitzungspuffer
    Aufnahme
  end note
  note right of Karenz
    kein Haltegrund, nur nach Sitzungsende durch den Zuschauer
    zählt bei der Zulassung nicht
    weicht als Erstes
  end note
  note left of Zurueck
    nur Dauersender, die nicht starten können
    Haltegrund bleibt, keine Verbindung, kein Ring
    jeder Eintritt wird dem Administrator gemeldet
  end note
```

Ereignisse, die in **jedem** Zustand gelten und deshalb im Bild fehlen (`04` 5.1):

| Ereignis | Folge |
|---|---|
| Quelle entfernt; Adresse, Zugangsdaten, Ursprünge oder feste Ziele der Quelle geändert oder entzogen | Der Empfänger endet innert 5 Sekunden. Hat der Strom noch einen Haltegrund, verbindet er danach mit dem neuen Regelstand oder der nächsten Quelle der Rangfolge; ohne brauchbare Quelle endet er mit Fehlercode, ein Dauersender wird «zurückgestellt» (Ablauf 4.4). |
| Befehl «sofort beenden» der Verwaltung | alle Haltegründe gelöst, «Endet» |
| Herunterfahren des Servers | eigener Abschluss (Ablauf 4.3) |

### 2.2 Haltegründe im Lebenszyklus

| Art | Besitzer | Entsteht | Endet | Danach ohne weiteren Haltegrund | Leiter bei Störung | Bei Mangel |
|---|---|---|---|---|---|---|
| dauerhaft | Sender | Strategie «dauerhaft», beim Serverstart | Strategie geändert, Sender deaktiviert (T-29) | sofort Ende | verlängert (dauerhafter Haltegrund) | **geschützt**, sobald er läuft (Vorgabe F-4 (b)); kann er nicht starten: zurückgestellt |
| Zeitplan | Zeitplan-Eintrag | Beginn des Fensters, beim Serverstart im Fenster | Ende des Fensters, Sender deaktiviert | sofort Ende | verlängert | wie dauerhaft |
| automatisch | Regel `04` 6.2 | Sender mit dieser Strategie unter den ersten N, und jemand schaut Live-TV | 10 min nach der letzten Live-Sitzung im Haushalt, Sender fällt aus der Liste, Sender deaktiviert, vorsorglicher Empfang der Quelle ruht | sofort Ende | nur auf Abruf: endet nach der Leiter | Rangfolge 2: endet |
| Vorladen | Live-Sitzung, die umschaltet | 2 s Verweilen, Nachbar nach `05` 6; Vorladen global an (Vorgabe, F-9) | 3 min nach letztem Umschalten, Sitzungsende, kein Nachbar mehr, Mangel, vorsorglicher Empfang der Quelle ruht, Sender deaktiviert | sofort Ende | keine: ein Versuch | Rangfolge 1: endet |
| Prüfung | Prüfjob | Verfügbarkeitsprüfung eines ruhenden Senders | sobald der Strom «läuft», spätestens mit Frist 2 | sofort Ende | keine: ein Versuch | Rangfolge 3: endet |
| Zuschauer | Live-Sitzung | Einstellen | Umschalten, Stopp, Entzug, Ablauf, Startfrist um, Stopp durch die Verwaltung | Ende durch den Client, 60 s ohne Herzschlag, 60 s gemeldete Pause ohne Sitzungspuffer, 15 min ohne Segmentabruf: **Karenz**. Umschalten und jedes Ende durch den Server: sofort Ende | nur auf Abruf: endet nach der Leiter mit Fehlercode | **geschützt** |
| Sitzungspuffer | – (am Haltegrund Zuschauer) | mit dem Einstellen, nur mit Recht «Timeshift»; nach Rückkehr des Speicherorts neu (3.13) | Umschalten, Sitzungsende, Entzug von «Timeshift», Ausfall des Speicherorts, Serverstart | – | – | wird gekürzt, nie der Strom beendet (`04` 8.3) |
| Aufnahme | Aufnahme | Aufwärmzeit 30 s (10 bis 120) vor dem Beginn des Vorlaufs; bei langer gespeicherter Bildgruppe Bildgruppe plus 10 s, höchstens 120 | Ende mit Nachlauf, Absage, Kontingent, «sofort beenden» | sofort Ende | verlängert | **geschützt** |

### 2.3 Zustand × Ereignis

| Zustand | Ereignis | Folgezustand | Regel |
|---|---|---|---|
| Ruht | erster Haltegrund, Zulassung erteilt | Verbindet; vorher enden Ströme derselben Quelle **ohne Haltegrund** (höchstens 1 s) | `04` 5.3 «Start», «Vor einem Verbindungsaufbau» |
| Ruht | erster Haltegrund eines Abnehmers, Zulassung verweigert | Ruht, Anfrage abgelehnt («ausgelastet») | `04` 8.2 Stufe 5 |
| Ruht | Dauersender will starten, Zulassung verweigert oder Speicher für den Ring fehlt | Zurückgestellt, Meldung an den Administrator | `04` 5.3 «Zurückgestellt» |
| Verbindet | Programmtabellen und erster Keyframe im Ring | Läuft | `04` 5.3 |
| Verbindet | Frist 1 (15 s bis zum ersten Byte) oder Frist 2 (Bildgruppe plus 5 s, 15 bis 45 s, ohne gespeicherte Bildgruppe 30 s) um | Wartet, Fehlversuch | `04` 5.3 «Fristen des Starts» |
| Verbindet | Frist 3 einer Sitzung um (Direktweg: Segmentdauer plus 5 s, mindestens 10 s; mit Umwandlung: Warten auf Einheiten plus 10 s) oder Gesamtfrist 90 s | diese Sitzung bekommt einen Fehlercode, ihr Haltegrund und ihre Reservierung gehen zurück; hält sonst niemand: Endet | `04` 5.1, 5.3 |
| Verbindet | Versuch für einen Abnehmer oder Dauersender wird abgelehnt, Muster A | Verbindet: bei dieser Quelle freigeben, was im Rang darunter steht (ohne Haltegrund, Vorgeladenes, Prüfungen, automatisch), neue Versuche nach 1 s und 2 s (höchstens drei in 5 s); vorsorglicher Empfang der Quelle ruht | `04` 5.3, Tabelle der Ablehnungen |
| Verbindet | alle drei Versuche scheitern | Zuschauer: Fehlercode «Quelle lehnt eine weitere Verbindung ab»; Aufnahme: scheitert nach F-5; Dauersender: Zurückgestellt | `04` 5.3 |
| Verbindet | vorsorglicher Versuch wird abgelehnt | Endet, Versuch entfällt still; bei Muster A ruht vorsorglicher Empfang der Quelle; nie «Zugang prüfen» | `04` 5.3 |
| Verbindet | 401 oder 403 ohne andere Verbindung der Quelle, zweimal im Abstand von mindestens 10 s | Wartet; Quelle «Zugang prüfen», Quarantäne mit Wiederversuch nach 5 min, 30 min, dann stündlich | `04` 5.3 |
| Verbindet | Verbindungsfehler; 429 ohne andere Verbindung | Wartet | `04` 5.3 |
| Verbindet | DRM oder verschlüsselter Strom erkannt | Wartet, Quelle mit Zustand «DRM» übersprungen | `04` 5.3, `05` 2.5 |
| Verbindet | letzter Haltegrund weg | Endet | `04` 5.1 |
| Verbindet, Wartet, Gestört, Läuft | Mangel, nur vorsorgliche Haltegründe | Haltegründe enden, Endet | `04` 5.1, 8.2 Stufen 1 bis 3 |
| Verbindet, Wartet, Gestört, Läuft | Mangel, ein Dauersender hält | nichts: Ein Dauersender weicht nicht (Vorgabe F-4 (b)). Nur bei Variante F-4 (a): Zurückgestellt | `04` 5.1 |
| Läuft | Abbruch, 10 s keine Daten, Uhr steht 10 s (HLS: der grössere Wert aus 10 s und anderthalb Zieldauern), Liste rückt nicht vor | Wartet | `04` 5.3, 9 |
| Läuft | Programmfehler beim Lesen der Quelle | Wartet, Quelle in Quarantäne | T-27, `04` 9 |
| Läuft | Muster B: dieser gehaltene Strom bricht innert 10 s ab, nachdem der Server eine andere Verbindung zur selben Quelle aufgebaut hat | erfüllt er einen Auftrag: Verbindet sofort, der Server schliesst die neue Verbindung; danach Ein-Strom-Betrieb und Ruhen des vorsorglichen Empfangs | `04` 5.3 |
| Läuft | Ersatzquelle aktiv, kein Abnehmer, 10 min um | Verbindet zur Hauptquelle | `04` 5.1, 5.3 |
| Läuft | Parameter oder Quelle wechseln | Läuft, neuer Zeitabschnitt; reicht das Byte-Budget für den neuen Ring nicht: Rangfolge, sonst bisheriger Ring («Startpuffer verkürzt») | T-6, `04` 5.3 |
| Läuft | letzter Haltegrund weg: Zuschauer geht von sich aus (Stopp, App zu, App länger als 30 s im Hintergrund, 60 s ohne Herzschlag, 60 s gemeldete Pause ohne Sitzungspuffer, 15 min ohne Segmentabruf) | Karenz | `04` 5.3 «Freigabe», 10.3 |
| Läuft | letzter Haltegrund weg aus jedem anderen Anlass (Umschalten, Ende durch den Server, Ende einer Aufnahme, eines Zeitfensters, einer Strategie, von «automatisch», Vorladen, Prüfung) | Endet, keine Karenz | `04` 5.3 «Freigabe» |
| Läuft | Sender deaktiviert | läuft weiter, solange ein Abnehmer hält, höchstens 12 h (Token), Sofortaufnahme höchstens 4 h; vorsorgliche Haltegründe und Dauersender-Haltegründe dieses Senders enden sofort | T-29, `04` 5.3 |
| Wartet | Wartezeit der Leiter um (1, 2, 5, 10, 20 s) | Verbindet, nächste Quelle ohne Quarantäne | `04` 5.3 |
| Wartet, Gestört | neuer Haltegrund Zuschauer oder Aufnahme | ein Versuch sofort, höchstens einer je Minute und Sender; danach weiter auf der Leiter | `04` 5.1, 5.3 |
| Wartet | Leiter erschöpft | Gestört | `04` 5.3 |
| Wartet | kein Haltegrund mehr | Ruht | `04` 5.1 |
| Gestört | verlängerte Leiter fällig (60 s, 5 min, ab 1 h 15 min) | Verbindet | `04` 5.3 |
| Gestört | 10 min Ausfall bei dauerhaftem Haltegrund | Gestört, Meldung an den Administrator | `04` 5.3, 10.2 |
| Gestört | kein dauerhafter Haltegrund, nur Haltegründe auf Abruf | Endet, Zuschauer bekommen einen Fehlercode | `04` 5.1, 5.3 |
| Karenz | neuer Haltegrund | Läuft | `04` 5.1 |
| Karenz | Karenzzeit um (30 s, 15 s bis 5 min), Platz oder Verbindung derselben Quelle gebraucht, Quelle bricht ab, Sender deaktiviert | Endet; bei Abbruch keine Leiter, keine Quarantäne | `04` 5.1, 5.3 |
| Karenz | Neustart des Servers | nichts, alles flüchtig | `04` 5.1 |
| Zurückgestellt | Mangel beim Start: Zulassung gelänge seit 60 s ununterbrochen; Quelle gab die Verbindung nicht: seit 60 s keine andere Verbindung zur Quelle | Verbindet als **vorsorglicher** Versuch (gibt nichts frei, wiederholt nicht sofort, keine zweite Verbindung im Ein-Strom-Betrieb); der am längsten Zurückgestellte zuerst, höchstens 2 Starts je Sekunde | `04` 5.3 |
| Zurückgestellt | ein Zuschauer oder eine Aufnahme braucht genau diesen Sender | Verbindet als gewöhnlicher Start für den Abnehmer | `04` 5.3 |
| Zurückgestellt | Wiederanlauf scheitert | Zurückgestellt, nächster Versuch nach 60 s, 5 min, dann alle 15 min | `04` 5.1, 5.3 |
| Zurückgestellt | Rangfolge der Quellen geändert, Quelle entfernt | Haltegrund bleibt, Wiederanlauf nimmt die dann ranghöchste Quelle; ohne Quelle Ruht mit Fehlercode | `04` 5.1 |
| Zurückgestellt | Zeitfenster endet, Strategie ändert sich, Sender deaktiviert | Ruht | `04` 5.1, 5.3 |
| Endet | Beitritt eines Abnehmers während des Schliessens | der Beitritt wartet höchstens 1 s, dann Verbindet | `04` 5.1, 5.3 |
| Endet | Verbindung geschlossen, Ring freigegeben | Ruht | `04` 5.1 |

### 2.4 Quellen, die nur eine Verbindung zulassen

Es gibt **keine Einstellung und keine Zahl je Quelle** (LTV-017; Vorgabe, F-3). Vorladen, «automatisch» und
Prüfungen haben nur globale Einstellungen und folgen der Strategie des Senders. Der Server beobachtet zwei
Muster (`04` 5.3); das ist Laufzeitverhalten im Arbeitsspeicher, ein Neustart vergisst es:

| Muster | Woran erkannt | Folge |
|---|---|---|
| A · Ablehnung | Verbindungsversuch abgelehnt (401, 403, 429 oder Abbruch vor dem ersten Byte), während eine andere Verbindung derselben Quelle offen ist oder vor höchstens 10 s geschlossen wurde | vorsorglicher Empfang der Quelle **ruht**: kein Vorladen, kein «automatisch», keine Prüfung; laufende vorsorgliche Ströme der Quelle enden sofort |
| B · Verdrängung | ein gehaltener Strom bricht innert 10 s ab, nachdem der Server irgendeine andere Verbindung zur selben Quelle aufgebaut hat | dasselbe und **Ein-Strom-Betrieb**: keine zweite Verbindung zu dieser Quelle; wer einen Auftrag erfüllt und die Verbindung hat, behält sie; unter Gleichzeitigen gilt Aufnahme, Zuschauer, Dauersender, «automatisch», Prüfung, Vorladen |

Die Frist des Ruhens ist 30 Minuten, beim nächsten Auftreten 6 Stunden, dann 24 Stunden; sie fällt auf
30 Minuten zurück nach 7 Tagen ohne Muster oder wenn der Administrator die Quelle ändert oder testet. Wer
einen Zugang mit einer Verbindung hat, schaltet das Vorladen **global** aus; die Verwaltung gibt den Hinweis,
sobald ein Muster erkannt ist.

## 3. Die 18 Situationen des Auftrags

### 3.1 Benutzer startet einen noch nicht empfangenen Sender

**Ausgangslage.** Sender K ruht, niemand hält ihn. Das Profil hat das Recht «Live-TV» (wirksam auf Konto und
Profil); Strategie «bei Bedarf». **Der Benutzer sieht:** nach dem Tippen ein kurzes Warten, dann Bild; der
Zielwert «unter 3 Sekunden» (Fall 3, zu prüfen, M-1) hängt vor allem an der Bildgruppen-Länge der Quelle
(`05` 5.2, 7).

```mermaid
sequenceDiagram
  participant CL as Client
  participant API as API und Handler
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant ZUL as Zulassung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
    participant SP as Sitzungspuffer
    participant SEG as Segmentierer
    participant UW as Umwandlung
  end
  participant PB as playback-Grenzen
  participant CAT as catalog
  participant Q as Quelle
  participant DB as SQLite
  participant DSK as Datenträger

  Note over STR,VT: Sender K ruht, kein Haltegrund
  rect rgba(66,135,245,0.14)
    Note over CL,SES: Zuschauer, Haltegrund Zuschauer
    CL->>API: POST einstellen, Sender K
    API->>SES: einstellen mit Anmeldung, Profil, Gerät
    SES->>SES: drei Schalter prüfen, Drosselung je Sitzung und Profil
    break kein wirksames Recht oder Sender deaktiviert
      SES-->>API: dieselbe Antwort wie unbekannt
      API-->>CL: 404
    end
    SES->>PB: Platz reservieren unter der Startsperre
    PB-->>SES: Platz innerhalb der Stream-Grenzen
    SES->>ZUL: Serverdeckel Live-Sitzungen
    SES->>STR: Haltegrund Zuschauer, Besitzer Sitzung S
    STR->>STR: suchen oder anlegen unter Sperre
    STR->>ZUL: Eingangsstrom und Ring zulassen
    break kein Platz nach Rangfolge 0 bis 4
      ZUL-->>SES: abgelehnt
      SES->>PB: Platz freigeben
      SES-->>CL: Server ist ausgelastet
    end
    STR->>STR: Ströme derselben Quelle ohne Haltegrund beenden, höchstens 1 s
    STR->>CAT: ranghöchste Quelle ohne Quarantäne
    CAT-->>STR: Adresse und Zugang, nur im Speicher
    STR->>EMP: Quelle öffnen
    EMP->>Q: Verbindung über den Dialer der Quelle
    Q-->>EMP: TS-Daten
    EMP->>VT: Pakete eines Programms
    VT->>VT: PAT und PMT, erster Keyframe in den GOP-Index
    VT-->>STR: Strom läuft
    opt keine gespeicherten Streamparameter
      STR->>UW: Probe mit ffprobe über Pipe aus dem Ring, nur mit wirksamer Sperre
      UW-->>STR: Codecs, Bildgrösse, Zeilensprung
      STR->>DB: Streamparameter speichern
    end
    SES->>SES: Entscheid Original, Ton- oder Bild-Umwandlung
    SES->>VT: Leser ab letzter vollständiger Bildgruppe
    SES->>DB: Sitzungszeile anlegen
  end
  rect rgba(46,160,67,0.16)
    opt Recht Timeshift wirksam, ab Stufe 4
      Note over SP,DSK: Sitzungspuffer, gehört Sitzung S
      SES->>SP: Sitzungspuffer anlegen ab dem Einstellen
      SP->>DSK: Stückdateien schreiben
    end
  end
  rect rgba(66,135,245,0.14)
    SES->>SEG: bis 3 s auf das erste Segment warten
    alt erstes Segment innert 3 s da
      SES-->>API: Sitzung, Token lt, Startstelle, Zustand läuft
      API-->>CL: 200 mit Startstelle, Zustand läuft
    else noch kein Segment
      SES-->>API: Sitzung, Token lt, Zustand startet mit Grund
      API-->>CL: 200, Zustand startet
      loop Herzschlag alle 10 s bis läuft
        CL->>API: Herzschlag
        API-->>CL: startet, Grund verbindet, wartet auf Keyframe oder wartet auf Umwandlung
      end
    end
    break Frist 1, 2 oder 3 um, oder 90 s seit dem Einstellen
      SES-->>CL: Fehlercode, Haltegrund und Reservierung zurück, die Sitzung bleibt
    end
    opt Zuschauer stellt erneut ein, bevor die Antwort kam
      SES-->>CL: ältere Antwort endet mit 409 live_superseded
    end
    CL->>API: GET Liste mit lt, erst bei läuft
    API->>SES: Token, Einstell-Nummer, Recht höchstens 30 s alt
    SES->>SEG: Liste der Sitzung bauen
    SEG-->>CL: Liste mit Anlauf
    CL->>API: GET Segment mit lt
    API->>SEG: Segment lesen
    SEG-->>CL: Segment mit PAT, PMT und Keyframe
    Note over CL: erstes Bild
  end
```

**Sichergestellt.** Der Sender wird genau einmal empfangen; das Recht prüft der Server vor allem anderen, und
ohne Recht gibt es nur 404. Die Stream-Grenze des Kontos ist reserviert, bevor eine Verbindung aufgeht. Passt
der Strom zum Gerät, läuft kein FFmpeg (Weg «Original»); sonst geht es weiter wie in 3.4. Mit Recht
«Timeshift» beginnt der Sitzungspuffer beim Einstellen. Die Antwort auf «einstellen» wartet bis 3 Sekunden
auf das erste Segment, sonst kommt sie mit Zustand «startet» und Grund; der Client lädt die Liste erst bei
«läuft». Es gelten drei getrennte Fristen: bis zum ersten Byte 15 Sekunden je Versuch, bis «läuft» die
gespeicherte Bildgruppe plus 5 Sekunden (15 bis 45, ohne gespeicherte Bildgruppe 30), bis zum ersten Segment
der Sitzung im Direktweg die Segmentdauer plus 5 Sekunden (mindestens 10), mit Umwandlung die Wartezeit auf
Einheiten plus 10 Sekunden; zusammen höchstens 90 Sekunden. Läuft eine ab, bekommt die Sitzung einen
Fehlercode und bleibt bestehen. Ein überholtes Einstellen endet mit `409 live_superseded` (`04` 5.3, 9;
`05` 4.4). Ohne wirksame Sperre gibt es keine Probe über FFmpeg (T-27). **Gestützt auf:** T-2, T-5, T-7, T-9, T-10, T-12, T-14,
T-17, T-19, T-21, T-26; LTV-002.03, LTV-004.01, .07, .08, .10, LTV-012.06, .13. **Fehlerzweige:** kein Recht
(404); Grenze des Kontos erreicht (bestehende Meldung von `playback`); kein Platz (3.14); Verbindung scheitert
oder Quelle lehnt ab (Leiter und Deutung, 3.9 und 2.3); DRM (Quelle übersprungen); keine vollständige
Bildgruppe im Ring (bei Bildgruppen über 15 bis 30 s Start am neuesten Keyframe, ohne schnellen Start; bei
Segmenten über 10 s braucht ein Zuschauer ohne Sitzungspuffer eine Umwandlung, `05` 3.6); eine Frist läuft
ab (Fehlercode, Sitzung bleibt).

### 3.2 Benutzer startet einen bereits empfangenen Sender

**Ausgangslage.** Sender K läuft schon: dauerhaft, vorgeladen, in Karenz oder weil ein anderes Profil schaut.
Der Ring hält 15 Sekunden (bei langen Segmenten das Vierfache der Segmentdauer, höchstens 30 Sekunden) mit mindestens einer vollständigen Bildgruppe; die Streamparameter sind
gespeichert. **Der Benutzer sieht:** fast sofort Bild (Fall 1 und 2, Zielwert unter 1 Sekunde, zu prüfen (M-1); hängt am
Startpuffer des Players und am Netz, `05` 5.2).

```mermaid
sequenceDiagram
  participant CL as Client
  participant API as API und Handler
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
    participant SEG as Segmentierer
  end
  participant PB as playback-Grenzen
  participant Q as Quelle

  rect rgba(128,128,128,0.16)
    Note over STR,Q: Empfang ohne Abnehmer, Haltegrund dauerhaft, automatisch oder Vorladen
    Q-->>EMP: TS-Daten laufend
    EMP->>VT: Pakete in den Ring, 15 s; bei langen Segmenten das Vierfache der Segmentdauer, höchstens 30 s
  end
  rect rgba(66,135,245,0.14)
    Note over CL,SES: Zuschauer, Haltegrund Zuschauer
    CL->>API: POST einstellen, Sender K
    API->>SES: einstellen
    SES->>SES: drei Schalter, Drosselung
    SES->>PB: Platz reservieren unter der Startsperre
    SES->>STR: Haltegrund Zuschauer, Besitzer Sitzung S
    STR->>STR: suchen oder anlegen, Strom gefunden
    alt Strom in Karenz
      STR->>STR: Karenz endet, Strom läuft weiter
    else Strom läuft mit anderem Haltegrund
      STR->>STR: Haltegrund hinzufügen
    end
    STR-->>SES: Strom läuft, Parameter gespeichert
    SES->>SES: Entscheid ohne Probe
    SES->>VT: Leser ab letzter vollständiger Bildgruppe
    VT-->>SEG: Anlauf, bis zu zwei Bildgruppen vor dem Tastendruck
    SES-->>CL: Token lt, Startstelle
    CL->>API: GET Liste mit lt
    API->>SEG: Liste bauen
    SEG-->>CL: Liste mit Anlauf
    CL->>API: GET Segment
    SEG-->>CL: Segment aus dem Ring
    Note over CL: erstes Bild
    Note over SEG: Anlauf fällt aus der Liste, sobald Segmente nach dem Tastendruck da sind, höchstens nach 15 s
  end
```

**Sichergestellt.** Keine neue Verbindung, keine Probe, kein Warten auf einen Keyframe der Quelle. Der
Anlauf aus dem Ring ist einmal sichtbar, danach antwortet der Server für seine Segmente mit 404; die
Rückspulgrenze ist der Tastendruck (Vorgabe F-7 (a)). Ein Strom, der seit einer Stunde dauerhaft läuft, gibt
einem neuen Zuschauer höchstens den Anlauf. **Gestützt auf:** T-6, T-7, T-17, T-22; LTV-004.02, LTV-007.11, .13,
LTV-013; Zielwerte Fall 1 und 2 (LTV-010, zu prüfen, M-1). **Fehlerzweige:** wie 3.1 bis zum Haltegrund; Strom endet gerade
(Beitritt wartet auf das Ende und startet neu, 4.1); Bildgruppen über 15 bis 30 s (Start am neuesten Keyframe, kein
schneller Start; `05` 3.6).

### 3.3 Mehrere Benutzer schauen denselben Sender

**Ausgangslage.** Zwei Profile, ein Profil mit Recht «Timeshift» und eines ohne, stellen denselben Sender
ein. **Sie sehen:** beide dasselbe Programm, jedes mit eigener Wiedergabe, eigener Qualität und eigener
Tonspur; nur wer «Timeshift» hat, kann pausieren.

```mermaid
sequenceDiagram
  participant CL1 as Client 1
  participant CL2 as Client 2
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
    participant SP as Sitzungspuffer
    participant SEG as Segmentierer
  end
  participant Q as Quelle
  participant DSK as Datenträger

  rect rgba(66,135,245,0.14)
    Note over CL1,SES: Zuschauer 1, Sitzung S1, mit Recht Timeshift
    CL1->>SES: einstellen, Sender K
    SES->>STR: Haltegrund Zuschauer S1
    STR->>EMP: Strom anlegen wie Ablauf 3.1
    EMP->>Q: eine Verbindung
    Q-->>EMP: TS-Daten
    EMP->>VT: Pakete in den Ring
    SES->>VT: Leser für S1
  end
  rect rgba(46,160,67,0.16)
    Note over SP,DSK: Sitzungspuffer nur von S1
    VT->>SP: Kopie ab dem Einstellen von S1
    SP->>DSK: Stückdateien S1
  end
  rect rgba(66,135,245,0.14)
    Note over CL2,SES: Zuschauer 2, Sitzung S2, ohne Recht Timeshift
    CL2->>SES: einstellen, Sender K
    SES->>STR: Haltegrund Zuschauer S2
    STR-->>SES: Strom läuft, kein zweiter Empfang
    SES->>VT: Leser für S2
    par Segmente für S1
      CL1->>SEG: GET Segment mit lt von S1
      SEG->>SP: aus dem Sitzungspuffer S1
      SEG-->>CL1: Segment
    and Segmente für S2
      CL2->>SEG: GET Segment mit lt von S2
      SEG->>VT: Kopie aus dem Ring, nur drei neueste Segmente
      SEG-->>CL2: Segment
    end
    Note over STR: ein Eingangsstrom, zwei Haltegründe, zwei Leser
  end
  rect rgba(66,135,245,0.14)
    CL2->>SES: beenden
    SES->>STR: Haltegrund S2 lösen
    STR->>STR: Haltegrund S1 bleibt, Strom läuft weiter
  end
```

**Sichergestellt.** Ein Eingangsstrom, beliebig viele Haltegründe und Leser; zwanzig Zuschauer ergeben einen
Empfang. Jede Sitzung hat eigene Liste, eigenes Token, eigenen Entscheid. Zwei Zuschauer teilen nur den Ring,
nie den Zeitversatz: Jeder Sitzungspuffer ist eine eigene Kopie (Vorgabe F-8 (a)) und wird nie aus einem
anderen gefüllt. Das Ende eines Zuschauers löst nur seinen Haltegrund. **Gestützt auf:** T-2, T-6, T-8, T-9, T-10;
LTV-004.02 bis .05, .07, .34, .37, LTV-007.01, .03, DR-24. **Fehlerzweige:** Grenze gleichzeitiger Ströme des
Kontos erreicht (Meldung beim zweiten Profil); Deckel aller Sitzungspuffer erreicht (neue Sitzung läuft ohne
Zeitversatz, `04` 8.3); Umwandlung nötig (3.4, je Sitzung eigener Prozess).

### 3.4 Ein Benutzer benötigt individuelles Transcoding

**Ausgangslage.** Der Sender sendet zum Beispiel H.264 mit Zeilensprung und MP2-Ton; der Browser kann beides
nicht sauber (`04` 7.6). **Der Benutzer sieht:** Bild nach Fall 4 (Zielwert unter 5 Sekunden, zu prüfen, M-1), sonst wie
immer. Andere Zuschauer desselben Senders merken nichts.

```mermaid
sequenceDiagram
  participant CL as Client
  participant API as API und Handler
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant VT as Verteiler
    participant SP as Sitzungspuffer
    participant SEG as Segmentierer
    participant UW as Umwandlung
  end

  rect rgba(66,135,245,0.14)
    Note over CL,SES: Zuschauer, Haltegrund Zuschauer, Strom läuft
    CL->>API: POST einstellen mit Gerätefähigkeiten
    API->>SES: einstellen
    SES->>SES: Entscheid aus Parametern, Fähigkeiten, Grenzen ergibt Bild-Umwandlung
    break Eindämmung nicht verfügbar, Sperre auf dem Host fehlt
      SES-->>CL: auf diesem Server nicht abspielbar, Eindämmung nicht verfügbar
      Note over SES,UW: kein Live-FFmpeg ohne wirksame Sperre, nur der Direktweg bleibt
    end
    SES->>UW: Einheiten reservieren für Sitzung S
    UW->>UW: Anteil Live höchstens die Hälfte, Warten in Ankunftsreihenfolge bis 10 s
    break nach 10 s kein Platz
      UW-->>SES: ausgelastet
      SES-->>CL: Server ist ausgelastet
    end
    UW->>UW: Offline-Downloads verdrängen, falls nötig
    UW-->>SES: Reservierung gehört Sitzung S
    SES->>UW: FFmpeg starten, eigene Umgebung, nur Pipes, feste Argumente
    alt mit Recht Timeshift
      rect rgba(46,160,67,0.16)
        Note over SP,UW: Eingang aus dem Sitzungspuffer
        SP->>UW: Quell-TS ab dem Einstellen
      end
    else ohne Recht Timeshift
      VT->>UW: Zuführer am Ring, Rückstau bis 8 MiB
    end
    UW->>SEG: TS-Ausgabe, Keyframe im Raster von 2 s
    SES-->>CL: Token lt, Startstelle
    loop solange der Client Segmente holt
      CL->>API: GET Segment mit lt
      API->>SEG: Segment der Ausgabe
      SEG-->>CL: Segment
    end
    Note over UW: 60 s ohne Anfrage, FFmpeg endet, Reservierung bleibt bei der Sitzung
    opt Qualität oder Tonspur ändern
      CL->>API: POST Qualität oder Spur
      API->>SES: neuer Entscheid
      SES->>UW: Reservierung umbuchen, nur eine Erhöhung wartet
      UW->>UW: neue Ausgabe-Generation, wartet nicht auf das Ende der alten
      SEG-->>CL: Liste mit Diskontinuität, Startstelle bleibt
    end
  end
```

**Sichergestellt.** Genau ein FFmpeg je Sitzung, nie gemeinsam genutzt, nie vorsorglich, nur über Pipes und
**nur mit wirksamer Sperre** (Landlock hart, Filter für Systemaufrufe): Fehlt sie auf dem Host, bleibt nur der
Direktweg, und die Verwaltung zeigt «Eindämmung nicht verfügbar» mit der Folge; «trotzdem erlauben» wäre die
ausdrückliche Wahl des Betreibers und ist eine Variante der Frage `12` F-1 (T-27, `04` 10.8); die
Reservierung gehört der Sitzung und übersteht Umschalten, Pause und Neustart von FFmpeg. Die Dateiwiedergabe
behält mindestens die Hälfte des Budgets. Mit Zeitversatz liegt der Puffer **vor** der Umwandlung, sodass ein
Qualitätswechsel auf die ganze Vergangenheit wirkt. **Gestützt auf:** T-4, T-10, T-13, T-14, T-15, T-19, T-27;
LTV-004.09, .12 bis .19, LTV-004.X-01, LTV-012.08. **Fehlerzweige:** Budget nach 10 s nicht frei
(«ausgelastet»); FFmpeg langsamer als Echtzeit (mit Zeitversatz fällt er zurück, ohne überspringt er, nach
30 s «zu langsam», `04` 8.3); Parameterwechsel (neuer Zeitabschnitt, FFmpeg startet neu); Hardware fehlt
(Software-Rückfall wie heute, kein neuer Rückfall zur Laufzeit); Sperre fehlt (Sender nur im Original
abspielbar, sonst «nicht abspielbar»).

### 3.5 Ein Benutzer wechselt den Sender

**Ausgangslage.** Sitzung S schaut Sender A (Einstell-Nummer n) und schaltet auf Sender B. A und B kommen
aus derselben Quelle, deren Zugang vielleicht nur eine Verbindung erlaubt. **Der Benutzer sieht:** ein
Umschalten ohne neuen Player; bei vorgeladenem oder empfangenem B Fall 1 und 2, sonst Fall 3.

```mermaid
sequenceDiagram
  participant CL as Client
  participant API as API und Handler
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
    participant SP as Sitzungspuffer
    participant UW as Umwandlung
  end
  participant Q as Quelle
  participant DSK as Datenträger

  rect rgba(66,135,245,0.14)
    Note over CL,SES: Zuschauer, Haltegrund Zuschauer wandert von A nach B
    CL->>API: POST einstellen, Sender B, Liste und Richtung
    API->>SES: einstellen in Sitzung S
    SES->>SES: Drosselung höchstens 1 je Sekunde, Stoss 5
    break zu schnell
      SES-->>CL: zu schnell, Client wiederholt
    end
    SES->>SES: Einstell-Nummer n plus 1, neues Token, altes Token ergibt 404
    SES->>STR: Haltegrund Zuschauer von A lösen
    alt A hat einen anderen Haltegrund
      STR->>STR: A läuft weiter
    else A hat keinen Haltegrund mehr
      STR->>EMP: A sofort beenden, keine Karenz
      EMP->>Q: Verbindung A schliessen
    end
  end
  rect rgba(46,160,67,0.16)
    opt Sitzungspuffer vorhanden
      SES->>SP: Sitzungspuffer von A löschen
      SP->>DSK: Stückdateien A Datei für Datei löschen
    end
  end
  rect rgba(66,135,245,0.14)
    SES->>STR: Haltegrund Zuschauer für B
    alt B läuft oder ist vorgeladen
      STR-->>SES: Strom läuft, wie Ablauf 3.2
    else B ruht
      STR->>STR: Ströme derselben Quelle ohne Haltegrund beenden, Ende von A abwarten, höchstens 1 s
      STR->>EMP: B öffnen, wie Ablauf 3.1
      EMP->>Q: Verbindung B
      Q-->>EMP: TS-Daten B
      EMP->>VT: Pakete B
    end
    SES->>SES: Platz bei den Stream-Grenzen bleibt
    opt Umwandlung nötig
      SES->>UW: Reservierung umbuchen, neuer FFmpeg wartet nicht auf den alten
    end
    SES->>VT: Leser B ab letzter vollständiger Bildgruppe
    SES-->>CL: neues Token, Startstelle
  end
  rect rgba(46,160,67,0.16)
    opt Recht Timeshift
      SES->>SP: neuer Sitzungspuffer ab dem Einstellen von B
    end
  end
  rect rgba(255,193,7,0.18)
    Note over SES,STR: nach 2 s auf B, Vorladen der Nachbarn, Ablauf 4.2
  end
```

**Sichergestellt.** Umschalten ist ein Vorgang **in** der Sitzung, keine neue Sitzung; Stream-Platz und
Umwandlungsreservierung bleiben. Beim Umschalten gibt es keine Karenz, und vor der neuen Verbindung schliesst
die alte derselben Quelle: Der Server bleibt mit Zugängen verträglich, die nur eine Verbindung erlauben, ohne
ein Limit zu kennen. Vorgeladenes derselben Quelle bleibt dabei stehen; es weicht erst, wenn die Quelle sich wehrt (Muster A
oder B, 2.4). Eine Einstellung je Quelle gibt es nicht. Der Puffer von A ist gelöscht. **Gestützt auf:** T-9, T-10, T-13, T-17, T-21; LTV-007.08,
LTV-002.09, LTV-004.33, LTV-017 (kein Limit verwaltet), Abnahmeprüfung `04` 8.5 (20 Sender in 10 s, höchstens
zwei Eingangsströme danach). **Fehlerzweige:** zu schnell (Client wiederholt); B lehnt ab, während A noch
schliesst (Muster A: Rangniedrigeres der Quelle freigeben, bis drei Versuche in 5 s, keine Sperre; scheitern sie:
«Quelle lehnt eine weitere Verbindung ab»); im Ein-Strom-Betrieb hält ein anderer Auftrag die Verbindung
(Fehlercode, niemand wird verdrängt); kein Platz für B (Ablehnung, A ist dann schon
beendet: Folge der Reihenfolge in `04` 5.3).

### 3.6 Ein Benutzer pausiert und verwendet Timeshift

**Ausgangslage.** Sitzung S mit wirksamem Recht «Timeshift» (Stufe 4) schaut Sender K; der Sitzungspuffer
läuft seit dem Einstellen. **Der Benutzer sieht:** Pause, Zurückspulen bis zum Zeitpunkt des Einschaltens,
Vorspulen bis Live, Sprung zu Live.

```mermaid
sequenceDiagram
  participant CL as Client
  participant API as API und Handler
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant VT as Verteiler
    participant SP as Sitzungspuffer
    participant SEG as Segmentierer
    participant UW as Umwandlung
  end
  participant DSK as Datenträger

  rect rgba(46,160,67,0.16)
    Note over VT,DSK: Sitzungspuffer von S, läuft auch in der Pause weiter
    loop dauernd
      VT->>SP: Pakete, Rückstau bis 16 MiB
      SP->>DSK: Stückdateien mit Verzeichnis
    end
  end
  rect rgba(66,135,245,0.14)
    Note over CL,STR: Haltegrund Zuschauer bleibt, der pausierte Zuschauer hält den Strom
    CL->>CL: Pause im Player
    loop alle 10 s
      CL->>API: Herzschlag mit Zustand Pause und Abspielstelle
      API->>SES: Sitzung am Leben, Anzeige pausiert
    end
    Note over SES: Schauzeit zählt, was der Server ausliefert, die Angabe Pause schaltet nichts ab
    opt mit Umwandlung
      SES->>UW: nur so viel Eingang, wie der Client abholt, plus drei Segmente
      Note over UW: 60 s ohne Anfrage, FFmpeg endet, Reservierung bleibt
    end
    CL->>API: GET Segment an der Pausenstelle
    alt Direktweg
      API->>SEG: Segment aus dem Sitzungspuffer
      SEG->>SP: lesen
      SEG-->>CL: Segment
    else Umwandlung, Segment nicht erzeugt
      SEG->>UW: FFmpeg an dieser Rastergrenze neu starten, höchstens 2 in 10 s
      UW->>SP: Eingang ab dem Keyframe davor
      UW-->>SEG: Segmente
      SEG-->>CL: Segment
    end
    CL->>API: GET Segment vor dem Einschalten
    API-->>CL: 404, Rückspulgrenze ist der Tastendruck
    opt Puffer erreicht die Höchstdauer, Vorgabe 60 min
      SP->>DSK: ältestes Stück verwerfen
      SEG-->>CL: Liste ohne dieses Stück, Wiedergabe setzt am Fensteranfang fort
    end
    CL->>CL: Sprung zu Live an das neueste Segment
  end
```

**Sichergestellt.** Die Pause hält den Eingangsstrom nicht an; der Puffer gehört nur dieser Sitzung und
beginnt beim Einstellen; die Liste reicht höchstens von der Rückspulgrenze bis zum neuesten Segment, jede
Anfrage ausserhalb ist 404. Ohne Recht «Timeshift» gibt es keinen Sitzungspuffer: Die Liste nennt die drei
neuesten Segmente aus dem Ring, hält der Player an, sind sie danach 404 und er springt an die Live-Kante
(T-10). Der Herzschlag meldet «spielt» oder «Pause»; das dient nur der Anzeige und dem Vorlauf einer Umwandlung. Die
Schauzeit zählt jedes ausgelieferte, verschiedene Segment, also was der Server beobachtet; ein pausierter
Player holt keine Segmente, deshalb zählt die Pause von selbst nicht, aber die Angabe «Pause» schaltet die
Zählung nie ab (`04` 10.3, `12`, `03` DR-12). Holt die Sitzung keine Segmente mehr, endet sie nach der
Höchstdauer des Zeitversatzes plus 15 Minuten; ohne Sitzungspuffer endet eine gemeldete Pause nach
60 Sekunden, und ohne Segmentabruf endet die Sitzung nach 15 Minuten. **Gestützt auf:** T-7, T-8, T-10, T-13, T-21;
LTV-007.01 bis .07, .10, .11, .14, DR-07, DR-12, DR-22. **Fehlerzweige:**
nahtloses Spulen mit Umwandlung nicht belegt (M-9), Rückfall `05` 4.5: jeder Neustart ist eine neue
Ausgabe-Generation, «spiele ab Uhrzeit» als Operation; Deckel oder Speicherort voll (grösster Puffer wird von
hinten gekürzt); Speicherort fällt aus (3.13); Herzschlag fehlt 60 s (Sitzungsende, 3.11).

### 3.7 Eine Aufnahme startet bei einem bereits empfangenen Sender

**Ausgangslage.** Ein Profil hat eine Aufnahme auf Sender K geplant; K läuft, weil jemand schaut. **Der
Benutzer sieht:** in «Meine Aufnahmen» die laufende Aufnahme; der Zuschauer merkt nichts.

```mermaid
sequenceDiagram
  participant CL as Client
  box rgba(128,128,128,0.08) Dienst live
    participant PL as Planer
    participant ZUL as Zulassung
    participant STR as Stromverwaltung
    participant VT as Verteiler
    participant AUF as Aufnahme-Schreiber
    participant SEG as Segmentierer
  end
  participant NTF as notify
  participant DB as SQLite
  participant DSK as Datenträger

  rect rgba(66,135,245,0.14)
    Note over CL,VT: Zuschauer, Haltegrund Zuschauer, Strom läuft
    loop laufende Wiedergabe
      CL->>SEG: Segmente
    end
  end
  rect rgba(220,53,69,0.14)
    Note over PL,DSK: Aufnahme, Haltegrund Aufnahme
    PL->>PL: Aufwärmzeit, 30 s vor dem Beginn des Vorlaufs
    PL->>ZUL: Recht Aufnahmen, Sender aktiv, Kontingent, Platz über der Reserve
    break abgelehnt
      PL->>DB: Aufnahme fehlgeschlagen mit Grund
      PL->>NTF: Meldung an Profil und Hauptprofil
    end
    PL->>STR: Haltegrund Aufnahme, Besitzer Aufnahme R
    STR-->>PL: Strom läuft, keine neue Verbindung
    PL->>AUF: Schreiber starten
    AUF->>VT: Leser ab letzter vollständiger Bildgruppe
    AUF->>DSK: Speicherort prüfen, Marker und Dateisystem
    loop bis Ende mit Nachlauf
      VT->>AUF: Quell-TS unverändert, Rückstau bis 64 MiB oder 60 s
      AUF->>DSK: Teilstück schreiben über paths
      AUF->>DB: Fortschritt alle 10 s
    end
    AUF->>DSK: Teil schliessen
    AUF->>DB: Aufnahme fertig
    PL->>NTF: Aufnahme fertig, Kanäle nach 03 F-3
    PL->>STR: Haltegrund Aufnahme lösen
    alt Haltegrund Zuschauer bleibt
      STR->>STR: Strom läuft weiter
    else kein Haltegrund mehr
      STR->>STR: sofort Ruht, keine Karenz
    end
  end
```

**Sichergestellt.** Die Aufnahme hängt am Eingangsstrom, nicht an der Sitzung des Zuschauers, und öffnet
keine zweite Verbindung. Sie schreibt den Quell-TS unverändert mit allen Spuren; kein FFmpeg, kein
Umwandlungsbudget. **Gestützt auf:** T-1, T-6, T-8, T-16, T-22, T-25; LTV-008.23, .24, .25, LTV-004.06, LTV-020.01,
.02, .05, LTV-Q-08.26. **Fehlerzweige:** Kontingent voll oder Speicherort unter der Reserve (abgelehnt und
gemeldet, `03` DR-06); Recht «Aufnahmen» entzogen (`03` F-7); Speicherort fällt aus (3.13); Quelle fällt aus
(3.9); Sender wird deaktiviert (die Aufnahme läuft bis zu ihrem Ende, T-29; nur «sofort beenden» bricht sie
ab, dann «unvollständig, Sender deaktiviert»). Nach dem Ende ohne weiteren Haltegrund ruht der Strom sofort;
folgt eine weitere Aufnahme auf demselben Sender, hält deren Haltegrund ihn schon (`04` 5.3).

### 3.8 Eine Aufnahme startet ohne angemeldeten Benutzer

**Ausgangslage.** Nachts, niemand angemeldet, keine Live-Sitzung. Sender K ruht. Eine geplante Aufnahme
wird fällig. **Der Benutzer sieht:** am Morgen die fertige Aufnahme, oder eine Meldung mit Grund.

```mermaid
sequenceDiagram
  box rgba(128,128,128,0.08) Dienst live
    participant PL as Planer
    participant ZUL as Zulassung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
    participant AUF as Aufnahme-Schreiber
  end
  participant CAT as catalog
  participant NTF as notify
  participant Q as Quelle
  participant DB as SQLite
  participant DSK as Datenträger

  Note over PL,DSK: keine Anmeldung, keine Sitzung, Planer läuft im Serverprozess
  rect rgba(220,53,69,0.14)
    Note over PL,AUF: Aufnahme, Haltegrund Aufnahme
    opt nur bei F-5 Variante a
      PL->>ZUL: ab 2 min vor Beginn einen Platz freihalten
    end
    PL->>PL: Aufwärmzeit, 30 s vor dem Beginn des Vorlaufs
    PL->>ZUL: Recht Aufnahmen, Kontingent, Platz, Eingangsströme
    break keine Zulassung
      PL->>DB: fehlgeschlagen mit Grund
      PL->>NTF: Meldung an Profil und Hauptprofil
    end
    PL->>STR: Haltegrund Aufnahme
    STR->>STR: suchen oder anlegen, Ströme der Quelle ohne Haltegrund beenden
    STR->>CAT: ranghöchste Quelle
    STR->>EMP: Quelle öffnen
    EMP->>Q: Verbindung
    alt Verbindung gelingt
      Q-->>EMP: TS-Daten
      EMP->>VT: Pakete
      VT-->>STR: Strom läuft
      PL->>AUF: Schreiber starten
      AUF->>VT: Leser
      loop bis Ende mit Nachlauf
        AUF->>DSK: Teilstück schreiben
        AUF->>DB: Fortschritt alle 10 s
      end
    else Verbindung scheitert
      STR->>STR: Leiter, verlängert, weil die Aufnahme ein dauerhafter Haltegrund ist
      AUF->>DB: Lücke mit Grund
      Note over STR,NTF: nach 10 min Ausfall Meldung an den Administrator
    end
    PL->>STR: Haltegrund lösen
    STR->>EMP: kein weiterer Haltegrund, sofort Ruht, keine Karenz
  end
```

**Sichergestellt.** Aufnahmen brauchen weder Anmeldung noch Sitzung noch Client; sie zählen nicht zu den
Stream-Grenzen des Kontos (`03` DR-12) und laufen unabhängig vom Jobsystem. Der Empfang startet rechtzeitig
vor dem Beginn. **Gestützt auf:** T-1, T-16, T-17, T-19; LTV-008.15, .25, LTV-002.08, .11, DR-11, DR-12.
**Fehlerzweige:** Zulassung verweigert (Vorrang offen: F-5 und `03` F-4; Vorgabe nach `04` 8.2 Stufe 5:
abgelehnt mit Grund, niemand wird verdrängt); Quelle nicht erreichbar (verlängerte Leiter, Lücke); Sender deaktiviert oder fehlt
(`03` DR-16, «Sender nicht verfügbar»); Quelle lehnt ab, während eine andere Verbindung derselben Quelle
offen ist (Muster A: Rangniedrigeres der Quelle wird freigegeben, nie ein Zuschauer und kein laufender
Dauersender, bis drei Versuche in 5 s; scheitern sie, scheitert die Aufnahme nach F-5, `04` 5.3). Die Aufwärmzeit von 30 Sekunden (10 bis 120) zählt vor
dem Beginn des Vorlaufs (`04` 5.2, 14.1; wie `11` 3.3); bei langer gespeicherter Bildgruppe Bildgruppe plus
10 Sekunden, höchstens 120.

### 3.9 Eine Streamquelle fällt aus

**Ausgangslage.** Sender K läuft mit einem Zuschauer und einer Aufnahme; die Quelle bricht ab oder liefert
10 Sekunden nichts. Es gibt keine Ersatzquelle (sonst 3.10). **Der Benutzer sieht:** das Bild steht kurz,
der Hinweis «Verbindung wird wiederhergestellt», danach Bild an der Live-Kante oder «Sender nicht
verfügbar».

```mermaid
sequenceDiagram
  participant CL as Client
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
    participant AUF as Aufnahme-Schreiber
    participant SEG as Segmentierer
    participant UEB as Überwachung
  end
  participant NTF as notify
  participant Q as Quelle
  participant DB as SQLite

  Q--xEMP: Abbruch oder 10 s keine Daten
  UEB->>STR: Stillstand erkannt
  STR->>STR: Läuft zu Wartet
  rect rgba(220,53,69,0.14)
    Note over AUF,DB: Aufnahme, Haltegrund Aufnahme
    AUF->>DB: Lücke beginnt, Grund Quelle weg
  end
  rect rgba(66,135,245,0.14)
    Note over CL,SES: Zuschauer, Haltegrund Zuschauer
    CL->>SES: Herzschlag
    SES-->>CL: Zustand Verbindung wird wiederhergestellt
  end
  opt neuer Zuschauer oder neue Aufnahme auf dem wartenden Strom
    STR->>EMP: ein Versuch sofort, höchstens einer je Minute und Sender
  end
  loop Leiter 1, 2, 5, 10, 20 s
    STR->>EMP: nächster Versuch, Quelle ohne Quarantäne
    EMP->>Q: Verbindung
    alt liefert Daten bis zu einem Keyframe
      Q-->>EMP: TS-Daten
      EMP->>VT: Pakete, Diskontinuität
      STR->>STR: Läuft, Leiter erst nach 60 s ohne Abbruch zurückgesetzt
      VT->>SEG: Diskontinuität in der Liste
      AUF->>DB: Lücke endet, derselbe Teil
    else verbindet ohne Brauchbares oder scheitert
      STR->>STR: Fehlversuch zählen
    end
  end
  alt Leiter erschöpft, nur Haltegründe auf Abruf
    STR->>SES: Strom endet mit Fehlercode
    SES-->>CL: Sender nicht verfügbar, neu einstellen möglich
  else Leiter erschöpft, dauerhafter Haltegrund wie hier die Aufnahme
    STR->>STR: Gestört, Versuche nach 60 s, dann alle 5 min, ab 1 h alle 15 min
    Note over STR,NTF: nach 10 min Meldung an den Administrator, einmal je Quelle und Stunde
    STR->>NTF: Fehler, Absender Quelle
  end
```

**Sichergestellt.** Die Wiederverbindung ist begrenzt und wird mit wachsendem Abstand versucht, nie als
Schleife. Eine Wiederverbindung zur selben Quelle ist eine Diskontinuität, kein neuer Zeitabschnitt: Die
Aufnahme bleibt im selben Teil und vermerkt die Lücke. Andere Sender sind nicht betroffen. **Gestützt auf:** T-6,
T-17, T-23; LTV-011.02, .03, .13, .16, .17, .20, LTV-020.12, LTV-008.26. **Fehlerzweige:** 401 oder 403
(Deutung nach Lage, 2.3); DRM erscheint beim Nachladen einer HLS-Liste (Quelle gesperrt); Programmfehler durch
ungültige Daten (Quarantäne, Server läuft weiter, T-27); Verbindung liefert kurz und reisst wieder ab (die
Leiter wächst weiter, weil sie erst nach 60 s gesundem Lauf zurücksetzt); ein Zuschauer stellt während der
Störung neu ein (ein Versuch sofort; läuft eine Startfrist ab, bekommt er einen Fehlercode, nicht «wird
wiederhergestellt»); Aufnahme endet während der Störung (Haltegrund weg,
Strom endet, wenn sonst nichts hält).

### 3.10 Ein Ersatzstream wird verwendet

**Ausgangslage.** Sender K hat eine Hauptquelle Q1 und eine Ersatzquelle Q2 in der Rangfolge. Q1 bricht ab,
während jemand schaut und eine Aufnahme läuft. **Der Benutzer sieht:** einen kurzen Bildsprung und den
Hinweis «Ersatzquelle».

```mermaid
sequenceDiagram
  participant CL as Client
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant ZUL as Zulassung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
    participant AUF as Aufnahme-Schreiber
    participant SEG as Segmentierer
    participant UW as Umwandlung
  end
  participant CAT as catalog
  participant Q1 as Quelle 1
  participant Q2 as Quelle 2

  Q1--xEMP: Abbruch
  STR->>STR: Q1 in Quarantäne, 30 s, dann 5 min, dann 30 min
  STR->>CAT: nächste Quelle der Rangfolge
  CAT-->>STR: Q2
  STR->>STR: Ströme der Quelle von Q2 ohne Haltegrund beenden, im Ein-Strom-Betrieb keine zweite Verbindung
  STR->>EMP: Wiederverbinden im selben Eingangsstrom mit Q2
  EMP->>Q2: Verbindung, keine zweite neben Q1
  Q2-->>EMP: TS-Daten
  EMP->>VT: neuer Zeitabschnitt
  VT->>ZUL: Ring neu reservieren
  alt Byte-Budget reicht, gegebenenfalls nach Rangfolge 0 bis 4
    ZUL-->>VT: neuer Ring
  else reicht auch dann nicht
    ZUL-->>VT: bisheriger Ring bleibt, weniger als 15 s
    Note over VT,ZUL: Verwaltung zeigt Startpuffer verkürzt, der Strom endet nie deswegen
  end
  VT->>UW: neue Probe über Pipe
  rect rgba(220,53,69,0.14)
    Note over AUF: Aufnahme, Haltegrund Aufnahme
    AUF->>AUF: neuer Teil, Aufnahme läuft weiter
  end
  rect rgba(66,135,245,0.14)
    Note over CL,SES: Zuschauer, Haltegrund Zuschauer
    SES->>SES: Entscheid neu berechnen
    opt Umwandlung läuft
      SES->>UW: FFmpeg neu starten, Reservierung umbuchen
    end
    VT->>SEG: Diskontinuität
    CL->>SES: Herzschlag
    SES-->>CL: Zustand Ersatzquelle
  end
  alt Strom ohne Zuschauer und ohne Aufnahme
    Note over STR: frühestens nach 10 min zurück zu Q1 durch Neuverbinden
  else mit Zuschauer oder Aufnahme
    Note over STR: nie von selbst zurück, nur auf Befehl des Administrators
  end
  Note over STR,CAT: beim nächsten Start des Senders immer zuerst Q1
```

**Sichergestellt.** Der Wechsel ist ein Wiederverbinden mit anderer Quelle **im selben** Eingangsstrom, also
ohne zweite Verbindung; Identität ist der Sender, nicht die Adresse. Eine Probeverbindung zur Hauptquelle
neben dem laufenden Strom gibt es nicht. Die Aufnahme bricht nicht ab, sie beginnt einen neuen Teil.
**Gestützt auf:** T-2, T-6, T-16, T-17; LTV-003.22, LTV-004.32, LTV-011.09, .10, LTV-020.13, .14. **Fehlerzweige:**
neuer Ring passt nicht ins Byte-Budget (Rangfolge, sonst bisheriger, kürzerer Ring, `04` 5.3); alle Quellen
in Quarantäne (Leiter wie 3.9); Q2 auf einem anderen Zugang mit Limit (Deutung der Ablehnung);
Q2 hat andere Spuren (Entscheid und Tonspur neu, `05` 3.4).

### 3.11 Der letzte Zuschauer beendet die Wiedergabe

**Ausgangslage.** Sitzung S ist die einzige, die Sender K hält. Der Benutzer tippt «Beenden», schliesst die
App oder verliert die Verbindung. **Der Benutzer sieht:** Die Wiedergabe endet; kehrt er in der Karenzzeit
zurück, steht das Bild sofort.

```mermaid
sequenceDiagram
  participant CL as Client
  participant API as API und Handler
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant ZUL as Zulassung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
    participant SP as Sitzungspuffer
    participant UW as Umwandlung
  end
  participant PB as playback-Grenzen
  participant Q as Quelle
  participant DB as SQLite
  participant DSK as Datenträger

  rect rgba(66,135,245,0.14)
    Note over CL,SES: Zuschauer, letzter Haltegrund Zuschauer
    alt Benutzer beendet
      CL->>API: POST beenden mit CSRF-Kopf
      API->>SES: Sitzung beenden
    else App im Hintergrund oder Bildschirm aus
      CL->>API: Player hält sofort an, nach 30 s beendet der Client die Sitzung
    else gemeldete Pause ohne Sitzungspuffer 60 s, oder 15 min ohne Segmentabruf
      SES->>SES: Sitzung endet wie beim Gehen des Zuschauers
    else App geschlossen oder Verbindung weg
      SES->>SES: 60 s ohne Herzschlag
    end
    SES->>SES: Token ungültig, jede weitere Anfrage 404
    opt Umwandlung
      SES->>UW: FFmpeg beenden, Reservierung freigeben
    end
    SES->>PB: Platz freigeben, Schauzeit melden
    SES->>DB: Sitzungszeile, Ende und Grund
  end
  rect rgba(46,160,67,0.16)
    opt Sitzungspuffer
      SES->>SP: löschen
      SP->>DSK: Stückdateien Datei für Datei über paths
    end
  end
  SES->>STR: Haltegrund Zuschauer lösen
  alt anderer Haltegrund vorhanden
    rect rgba(220,53,69,0.14)
      Note over STR: Aufnahme, dauerhaft oder Zeitplan hält, Strom läuft weiter
    end
  else kein Haltegrund mehr
    STR->>STR: Karenz, Vorgabe 30 s, einstellbar 15 s bis 5 min
    Note over STR: nur weil der Zuschauer selbst ging, zählt nicht als belegt, weicht als Erstes
    alt neuer Haltegrund in der Karenz
      STR->>STR: Läuft, Wiedereinstieg mit sofortigem Bild
    else Quelle bricht in der Karenz ab oder steht still
      STR->>STR: sofort Ruht, keine Leiter, keine Quarantäne, Abbruch protokolliert
    else Karenz um oder Platz oder Verbindung derselben Quelle gebraucht
      STR->>EMP: Verbindung schliessen
      EMP->>Q: schliessen
      STR->>VT: Ring freigeben
      STR->>ZUL: Byte-Budget zurückbuchen
    end
  end
  rect rgba(128,128,128,0.16)
    Note over STR: Haltegrund automatisch endet 10 min nach der letzten Live-Sitzung im Haushalt
  end
```

**Sichergestellt.** Das Ende eines Zuschauers räumt alles ab, was ihm gehört (Token, Puffer, FFmpeg,
Reservierung, Stream-Platz); andere Haltegründe bleiben unberührt. Ein Strom ohne Haltegrund belegt keinen
Platz und wird spätestens nach der Karenz beendet. **Gestützt auf:** T-10, T-13, T-17, T-21, T-26; LTV-002.09,
LTV-004.27, .33, .37, LTV-007.09. Karenz gibt es nur in diesem Fall, wenn der Zuschauer selbst geht; endet
die Sitzung durch den Server (Entzug, Abmelden, Tageslimit, Stopp durch die Verwaltung), ruht der Strom
sofort (3.15). **Fehlerzweige:** Herzschlag fällt im Browser-Tab im Hintergrund oder bei Bildübertragung aus
(Sitzung endet nach 60 s; offen als `04` O-25, M-3, M-5); Quelle bricht in der Karenz ab (Strom endet
sofort, keine Wiederverbindung).

### 3.12 Slouchcat startet neu

**Ausgangslage.** Der Server ist beendet worden (Herunterfahren nach 4.3, Absturz oder Stromausfall) und
startet wieder. Es gab Sitzungen, Sitzungspuffer, laufende Aufnahmen und dauerhaft empfangene Sender.
**Der Benutzer sieht:** die Wiedergabe bricht ab; der Client stellt neu ein; laufende Aufnahmen zeigen eine
Lücke «Server lief nicht».

```mermaid
sequenceDiagram
  participant CL as Client
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant SP as Sitzungspuffer
    participant AUF as Aufnahme-Schreiber
    participant PL as Planer
  end
  participant Q as Quelle
  participant DB as SQLite
  participant DSK as Datenträger

  rect rgba(46,160,67,0.16)
    Note over SP,DSK: Schritt 1, Sitzungspuffer gibt es nach dem Start nicht mehr
    SP->>DSK: Marker des Speicherorts Timeshift prüfen
    SP->>DSK: nur Dateien mit eigenem Namensmuster löschen, Datei für Datei
    SES->>DB: offene Live-Sitzungen beenden, Grund Neustart
  end
  rect rgba(220,53,69,0.14)
    Note over PL,DB: Schritt 2, Aufnahmen im Zustand läuft
    PL->>DB: laufende Aufnahmen lesen
    alt geplantes Ende in der Zukunft
      PL->>STR: Haltegrund Aufnahme
      PL->>AUF: weiter aufnehmen mit neuem Teil
      AUF->>DB: Lücke Server lief nicht
    else Ende vorbei
      PL->>DB: als unvollständig abschliessen
    end
    Note over STR,Q: Schritt 3, Sender mit weiterzuführender Aufnahme sofort, ohne Staffelung
    par verschiedene Quellen gleichzeitig
      STR->>EMP: Sender starten
      EMP->>Q: Verbindung
    and mehrere Sender derselben Quelle
      STR->>EMP: nacheinander ohne Pause
    end
  end
  rect rgba(128,128,128,0.16)
    Note over STR,Q: Schritt 4, danach Dauersender nach Kanalnummer
    loop gestaffelt, höchstens 2 Starts je Sekunde, nie zwei gleichzeitig bei derselben Quelle
      STR->>EMP: Sender starten
      EMP->>Q: Verbindung
    end
    Note over STR: reichen die Grenzen nicht, sind die übrigen zurückgestellt, Meldung mit ihrer Zahl
    Note over STR: Schritt 5, automatisch und Vorladen aus der Nutzung, Muster A und B vergessen
  end
  rect rgba(66,135,245,0.14)
    Note over CL,SES: Zuschauer, alte Sitzung gibt es nicht mehr
    CL->>SES: Herzschlag oder Liste mit altem Token
    SES-->>CL: 404
    CL->>SES: neu einstellen, Ablauf 3.1 oder 3.2
  end
```

**Sichergestellt.** Nach einem Neustart gibt es keinen alten Sitzungspuffer und keinen Startpuffer mehr;
gelöscht wird nur unter dem eigenen Marker und nur, was dem eigenen Namensmuster entspricht. Aufnahmen werden
nie gelöscht: Erhaltene Teile bleiben abspielbar, die Lücke ist benannt. Dauerhafte und zeitgesteuerte
Sender laufen von selbst wieder an. **Gestützt auf:** T-7, T-17, T-22, T-25; LTV-002.12, LTV-008.26, LTV-019.19,
LTV-020.20. **Fehlerzweige:** Speicherort fehlt beim Start (keine Löschung, kein Ausweichen, 3.13); Quelle
nicht erreichbar (Leiter, für Aufnahmen verlängert). Die Staffelung gilt nur für dauerhafte Sender und
Zeitfenster; Sender mit weiterzuführenden Aufnahmen verbinden sofort, weil jede Sekunde eine Lücke ist
(`04` 5.3).

### 3.13 Ein Datenträger fällt aus

**Ausgangslage.** Der Speicherort «Timeshift» oder «Aufnahmen» verschwindet: Netzspeicher getrennt, Platte
nicht eingehängt, Schreibfehler. **Der Benutzer sieht:** beim Zeitversatz den Hinweis «Zeitversatz gerade
nicht möglich»; bei Aufnahmen eine Lücke «Speicher fehlt». Live läuft weiter.

```mermaid
sequenceDiagram
  participant CL as Client
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant VT as Verteiler
    participant SP as Sitzungspuffer
    participant SEG as Segmentierer
    participant AUF as Aufnahme-Schreiber
  end
  participant NTF as notify
  participant DB as SQLite
  participant DSK as Datenträger

  alt Speicherort Timeshift
    rect rgba(46,160,67,0.16)
      Note over SP,DSK: Sitzungspuffer
      SP->>DSK: Marker prüfen vor dem nächsten Stück
      DSK--xSP: Marker fehlt, anderes Dateisystem oder Schreibfehler
      SP->>SP: Rückstau bis 16 MiB fängt kurzes Stocken auf
      SP->>SES: Speicherort fehlt
      SES->>SP: betroffene Sitzungspuffer enden, Stücke nicht mehr ausliefern
      SP->>DSK: Stücke löschen, soweit der Ort erreichbar ist
    end
    rect rgba(66,135,245,0.14)
      Note over CL,SES: Zuschauer läuft weiter
      SES->>SEG: ohne Zeitversatz, drei neueste Segmente aus dem Ring
      VT->>SEG: Segmente
      SEG-->>CL: neue Ausgabe-Generation, zeitversetzte oder pausierte Wiedergabe springt an die Live-Kante
      SES-->>CL: Herzschlag, Zeitversatz gerade nicht möglich
    end
    SES->>NTF: Meldung an den Administrator
    rect rgba(46,160,67,0.16)
      opt Speicherort 60 s lang wieder in Ordnung
        SES->>SP: neuer Sitzungspuffer für jede laufende Sitzung mit Recht
        Note over SP: Rückspulgrenze ist dieser Zeitpunkt
      end
    end
  else Speicherort Aufnahmen
    rect rgba(220,53,69,0.14)
      Note over AUF,DSK: Aufnahme
      AUF->>DSK: Marker prüfen, schreiben
      DSK--xAUF: Schreibfehler oder Marker fehlt
      AUF->>AUF: Rückstau bis 64 MiB oder 60 s
      alt Speicherort kommt rechtzeitig zurück
        AUF->>DSK: Rückstau schreiben, keine Lücke
      else Rückstau voll
        AUF->>DSK: Teil schliessen, soweit möglich
        AUF->>DB: Lücke Speicher fehlt
        AUF->>NTF: Meldung an den Administrator
        loop bis Marker wieder stimmt
          AUF->>DSK: Speicherort prüfen, nie in den leeren Einhängepunkt schreiben
        end
        AUF->>DSK: Wiederaufnahme mit neuem Teil
      end
    end
  end
```

**Sichergestellt.** Ein fehlender Speicherort ist ein Zustand, kein Programmfehler; der Server weicht nie
auf einen anderen Ordner aus und schreibt nie in einen leeren Einhängepunkt. Beim Speicherort «Timeshift»
enden die Puffer, die Wiedergabe springt an die Live-Kante, und 60 Sekunden nach der Rückkehr des Orts
beginnt je Sitzung mit Recht ein neuer Puffer mit neuer Rückspulgrenze (`04` 9, 14.6). Der Empfang wartet nicht auf den
Datenträger (T-8). Ein voller Speicherort verhält sich ähnlich: Timeshift kürzt zuerst den grössten Puffer,
Aufnahmen schliessen den Teil mit Lücke «Speicher voll», neue Aufnahmen starten nicht (`04` 8.3). Liegt der
Startpuffer in einem Ordner, der stockt, hat der Sender nur keinen schnellen Start. **Gestützt auf:** T-8, T-16,
T-25; LTV-019.20, .21, .22, LTV-020.12, LTV-008.26. **Fehlerzweige:** Datenträger nur langsam statt weg
(Rückstau, dann Lücke oder Diskontinuität, 3.17); Speicherort fehlt schon beim Serverstart (3.12).

### 3.14 Die Serverressourcen sind erschöpft

**Ausgangslage.** Die Obergrenze der Eingangsströme oder das Byte-Budget ist erreicht; ein Profil stellt
einen weiteren, nicht empfangenen Sender ein. Ein Teil der Plätze belegen Dauersender ohne Zuschauer. **Der
Benutzer sieht:** entweder Bild wie in 3.1 oder «Der Server ist ausgelastet». Niemand, der schon schaut oder
aufnimmt, wird unterbrochen, und ein laufender Dauersender weicht nicht.

```mermaid
sequenceDiagram
  participant CL as Client
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant ZUL as Zulassung
    participant STR as Stromverwaltung
    participant UW as Umwandlung
  end

  rect rgba(66,135,245,0.14)
    Note over CL,SES: neuer Zuschauer
    CL->>SES: einstellen, Sender X
    SES->>STR: Haltegrund Zuschauer
    STR->>ZUL: Eingangsstrom und Ring zulassen
  end
  ZUL->>ZUL: Ströme, Byte-Budget, Platz, Einheiten prüfen
  opt eine Grösse fehlt
    ZUL->>STR: Stufe 0, Ströme ohne Haltegrund, Karenz zuerst, ältester zuerst
    rect rgba(255,193,7,0.18)
      ZUL->>STR: Stufe 1, vorgeladene Sender, jüngster zuerst
    end
    rect rgba(128,128,128,0.16)
      ZUL->>STR: Stufe 2, automatisch gehaltene Sender, hinterster der Rangliste zuerst
      ZUL->>STR: Stufe 3, Prüfungen ruhender Sender
    end
    ZUL->>UW: Stufe 4, Offline-Downloads und zurückstellbare Jobs
  end
  alt Platz reicht
    ZUL-->>STR: zugelassen
    STR-->>SES: weiter wie Ablauf 3.1
  else reicht nicht, Stufe 5
    ZUL-->>SES: neue Anforderung abgelehnt
    SES-->>CL: Der Server ist ausgelastet
  end
  rect rgba(128,128,128,0.16)
    Note over ZUL,STR: laufende Dauersender ohne Zuschauer weichen nicht, Vorgabe F-4 b
  end
  rect rgba(220,53,69,0.14)
    Note over ZUL: nie freigegeben, Ströme mit Zuschauer oder Aufnahme, Umwandlungen aktiver Sitzungen, laufende Aufnahmen
  end
  rect rgba(128,128,128,0.16)
    opt ein Dauersender will starten, Beginn eines Zeitfensters oder Hochfahren
      STR->>ZUL: Zulassung für den Dauersender
      alt zugelassen
        ZUL-->>STR: Start wie Ablauf 3.1
      else verweigert
        ZUL-->>STR: zurückgestellt, Meldung an den Administrator
        loop Wiederanlauf als vorsorglicher Versuch
          STR->>ZUL: erst wenn die Zulassung seit 60 s ununterbrochen gelänge
          alt gelingt
            ZUL-->>STR: Verbindet
          else scheitert
            ZUL-->>STR: bleibt zurückgestellt, nächster Versuch nach 60 s, 5 min, dann alle 15 min
          end
        end
      end
    end
  end
```

**Sichergestellt.** Es gibt genau einen Ort, der zulässt und freigibt, mit fester Rangfolge: Vorsorgliches
weicht zuerst, Laufendes nie, auch kein laufender Dauersender (LTV-002: Empfang «unabhängig davon, ob jemand
zuschaut»). «Zurückgestellt» wird ein Dauersender nur, wenn er nicht starten kann; der Wiederanlauf zählt wie
ein vorsorglicher Versuch und stört deshalb keinen Zuschauer und keine Aufnahme (`04` 5.3). Der Rückstau der
Schreiber zählt erst bei Gebrauch; keine Aufnahme und keine Sitzung wird wegen des Rückstaus abgelehnt
(`04` 8.1). Folge der Vorgabe: Belegen Dauersender alle Eingangsströme (ab Werk 8), kann niemand einen
weiteren Sender schauen, bis der Administrator die Obergrenze anhebt; die Verwaltung warnt, wenn Dauersender
mehr als die Hälfte belegen (`04` 8.2). Unabhängig von den Grenzen der Konten gelten Serverdeckel
(Live-Sitzungen, Drosselung, Neustarts von FFmpeg). Eine fällige Aufnahme bei Mangel scheitert nach F-5; bis
zur Antwort verdrängt sie niemanden. **Gestützt auf:** T-13, T-17, T-19; LTV-002.01, .02, .10, .11,
LTV-004.20, LTV-017.01, DR-11. **Fehlerzweige:** Umwandlungseinheiten fehlen (warten bis 10 s, dann
Ablehnung, 3.4); CPU ausgelastet bei laufenden Umwandlungen («zu langsam» nach 30 s); Arbeitsspeicher knapp
(Ringe wachsen nicht, das Betriebssystem beendet zuerst FFmpeg-Umwandlungen). *Nur als Variante F-4 (a), nicht
in der Vorgabe:* Zwischen Stufe 4 und 5 würden Dauersender ohne Abnehmer zurückgestellt und kämen nach den
Regeln des Wiederanlaufs zurück.

### 3.15 Eine Benutzerberechtigung wird während der Wiedergabe entzogen

**Ausgangslage.** Ein Profil schaut; der Administrator schaltet «Live-TV» am Konto aus, oder das Hauptprofil
am Profil, oder die Anmeldung endet (Abmelden, Passwortwechsel, Tageslimit). **Der Benutzer sieht:** Die
Wiedergabe endet mit einer verständlichen Meldung, Ziel innert 10 Sekunden; danach ist der Bereich
unsichtbar.

```mermaid
sequenceDiagram
  participant CLV as Client Verwaltung
  participant CL as Client
  participant API as API und Handler
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant SP as Sitzungspuffer
    participant SEG as Segmentierer
    participant UW as Umwandlung
  end
  participant AU as auth
  participant PB as playback-Grenzen
  participant DSK as Datenträger

  CLV->>API: Schalter ändern
  API->>AU: Recht entziehen
  AU->>SES: Haken, Sitzungen des Kontos oder Profils
  alt Live-TV entzogen oder Anmeldung ungültig
    rect rgba(66,135,245,0.14)
      Note over SES,STR: Zuschauer endet
      SES->>SES: Token ungültig
      opt Umwandlung
        SES->>UW: FFmpeg beenden, Reservierung freigeben
      end
      SES->>PB: Platz freigeben
      SES->>STR: Haltegrund lösen
      Note over STR: Ende durch den Server, ohne anderen Haltegrund sofort Ruht, keine Karenz
    end
    rect rgba(46,160,67,0.16)
      SES->>SP: Sitzungspuffer löschen
      SP->>DSK: Stückdateien löschen
    end
    par Abruf läuft gerade
      CL->>SEG: GET Segment mit altem Token
      SEG-->>CL: 404 wie kein Recht
    and Sicherung
      CL->>API: GET Liste
      API->>SES: Recht und Anmeldung höchstens 30 s alt
      SES-->>CL: 404
    end
    Note over CL: Player zeigt Meldung, Bereich unsichtbar
  else nur Timeshift entzogen
    rect rgba(46,160,67,0.16)
      SES->>SP: Sitzungspuffer löschen
      SP->>DSK: Stückdateien löschen
    end
    rect rgba(66,135,245,0.14)
      SES->>SEG: ohne Zeitversatz, drei neueste Segmente aus dem Ring
      SES-->>CL: zeitversetzte Wiedergabe springt an die Live-Kante
    end
  else Aufnahmen entzogen
    rect rgba(220,53,69,0.14)
      Note over SES,STR: laufende Aufnahmen und Planungen nach 03 F-7, offen
    end
  end
```

**Sichergestellt.** Der Server setzt den Entzug durch, nicht der Client. Der Haken beendet sofort; als
Sicherung prüft jede Listenanfrage Recht und Anmeldung höchstens 30 Sekunden alt, und ein ungültiges Token
ergibt bei allen Live-Routen dieselbe 404-Antwort wie «kein Recht». Andere Zuschauer desselben Senders laufen
weiter. **Gestützt auf:** T-10, T-21, T-26; LTV-012.11, .12, .13, `03` 2.4. **Fehlerzweige:** Segmentanfrage
zwischen Entzug und Haken (spätestens die nächste Listenanfrage ist 404); Hauptprofil spielt Aufnahme eines
Profils mit Altersgrenze und verliert selbst «Live-TV» (`03` DR-03); Verlust des Rechts «Aufnahmen» bei
laufender Aufnahme (`03` F-7, O-9).

### 3.16 Eine M3U-Aktualisierung ändert die URL eines aktiven Senders

**Ausgangslage.** Sender K läuft (Zuschauer, eventuell Aufnahme). Der Listen-Abgleich der Quelle liefert für
K eine neue Stream-Adresse. Die **Quelle selbst** (ihre Adresse, Zugangsdaten, Ursprünge, festen Ziele) ändert
sich dabei nicht; das wäre Ablauf 4.4. **Der Benutzer sieht:** nichts, solange die alte Verbindung hält.

```mermaid
sequenceDiagram
  box rgba(128,128,128,0.08) Dienst live
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
  end
  participant JOB as Jobs
  participant CAT as catalog
  participant DB as SQLite
  participant Q1 as Quelle 1
  participant Q2 as Quelle 2

  rect rgba(66,135,245,0.14)
    Note over STR,Q1: Zuschauer oder Aufnahme, Strom läuft über die alte Adresse
    Q1-->>EMP: TS-Daten
    EMP->>VT: Pakete
  end
  JOB->>CAT: Abgleich-Job der Quelle
  CAT->>CAT: Liste lesen, Grössengrenzen, Sender wiedererkannt
  CAT->>DB: Quellzeile mit neuer Adresse, verschlüsselt, in Stapeln
  Note over STR,EMP: laufende Verbindung bleibt, solange sie liefert
  alt Strom bricht später ab oder startet neu
    STR->>CAT: Quelle für den nächsten Versuch
    CAT-->>STR: neue Adresse
    alt neue Adresse in den festen Zielen und Ursprüngen
      STR->>EMP: verbinden
      EMP->>Q2: Verbindung zur neuen Adresse
    else ausserhalb
      CAT->>DB: Ziel im Heimnetz nicht freigegeben, nie verbunden
      Note over EMP: T-5 und T-28 gelten neu, Zugangsdaten nur an feste Ursprünge
    end
  else Quellzeile verschwindet ganz, Sender fehlt in der Liste
    Note over STR: wie Deaktivieren, T-29, Laufendes mit Abnehmer läuft weiter
  else Administrator deaktiviert den Sender
    CAT->>DB: Sender deaktiviert, für alles Neue sofort 404, auch Umschalten dorthin
    STR->>STR: Vorladen, automatisch, dauerhaft, Zeitplan dieses Senders enden sofort
    alt Wiedergabe läuft
      Note over STR: bis der Zuschauer geht, höchstens bis das Token abläuft, 12 h, keine Erneuerung
    else geplante Aufnahme läuft
      Note over STR: bis zum geplanten Ende mit Nachlauf
    else Sofortaufnahme läuft
      Note over STR: höchstens bis zu ihrer Höchstdauer von 4 h
    end
    opt ausdrücklicher Befehl sofort beenden, mit Zahl der Betroffenen und Bestätigung
      STR->>EMP: Strom endet
      Note over STR: Zuschauer Sender nicht verfügbar, Aufnahme unvollständig, Sender deaktiviert
    end
  end
```

**Sichergestellt.** Ein aktiver Strom wechselt die Adresse eines Senders erst an einem sicheren Punkt, nämlich
bei der nächsten Verbindung; die Identität bleibt der Sender (Favoriten, Planungen bleiben). Netzschutz und
feste Ursprünge gelten für die neue Adresse neu, der Import probiert keine Adresse aus. Deaktivieren oder
Verschwinden eines Senders beendet nichts mit Abnehmer von selbst, aber begrenzt: Wiedergaben höchstens bis
zum Ablauf ihres Tokens (12 Stunden), Sofortaufnahmen höchstens 4 Stunden. Deaktivieren ist **kein
Sicherheitsmittel**; dafür gibt es «sofort beenden» und das Entfernen oder Ändern der Quelle (4.4).
**Gestützt auf:** T-2, T-5, T-17, T-28, T-29; LTV-002.10, .11, LTV-018.05, .06, .13, .14, .17.
**Fehlerzweige:** fehlerhafte oder auffällig kurze Liste (alter Stand bleibt, LTV-018.14, .22); Liste nicht
erreichbar (nichts ändert sich); neue Adresse mit DRM-Hinweis (beim Import markiert).

### 3.17 Ein langsamer Client gefährdet die interne Streamverteilung

**Ausgangslage.** Sender K hat drei Abnehmer: einen schnellen Client, einen Client im schwachen WLAN und eine
Aufnahme auf einem stockenden Netzspeicher. **Die Benutzer sehen:** Der schnelle Client merkt nichts; der
langsame springt nach vorn; die Aufnahme bekommt erst nach 64 MiB oder 60 Sekunden eine Lücke.

```mermaid
sequenceDiagram
  participant CL1 as Client 1
  participant CL2 as Client 2
  box rgba(128,128,128,0.08) Dienst live
    participant EMP as Empfang
    participant VT as Verteiler
    participant SP as Sitzungspuffer
    participant SEG as Segmentierer
    participant UW as Umwandlung
    participant AUF as Aufnahme-Schreiber
  end
  participant Q as Quelle
  participant DSK as Datenträger

  loop dauernd
    Q-->>EMP: TS-Daten
    EMP->>VT: schreiben, der Schreiber wartet nie
  end
  par schneller Client
    rect rgba(66,135,245,0.14)
      CL1->>SEG: GET Segment
      SEG->>VT: Bereich kopieren, danach prüfen
      SEG-->>CL1: Segment
    end
  and langsamer Client
    rect rgba(66,135,245,0.14)
      CL2->>SEG: GET Segment, verspätet
      SEG->>VT: Bereich kopieren, danach prüfen
      VT-->>SEG: inzwischen überschrieben
      SEG-->>CL2: 404
      CL2->>SEG: nächstes Segment, Player springt nach vorn
    end
  and langsamer Sitzungspuffer
    rect rgba(46,160,67,0.16)
      VT->>SP: Pakete, Rückstau bis 16 MiB
      SP->>SP: Rückstau voll, Leser überrundet
      SP->>SP: Diskontinuität, weiter am nächsten Keyframe
    end
  and langsame Umwandlung ohne Zeitversatz
    rect rgba(66,135,245,0.14)
      VT->>UW: Zuführer, Rückstau bis 8 MiB
      UW->>UW: Sprung nach vorn, nach 30 s Meldung zu langsam
    end
  and stockender Datenträger der Aufnahme
    rect rgba(220,53,69,0.14)
      VT->>AUF: Pakete, Rückstau bis 64 MiB oder 60 s
      AUF->>DSK: schreiben stockt
      AUF->>AUF: Rückstau voll, Lücke Datenträger zu langsam
      AUF->>VT: neuer Startpunkt am nächsten Keyframe
    end
  end
```

**Sichergestellt.** Niemand kann den Empfang aufhalten: Der Schreiber des Rings wartet nie und nimmt keine
Sperre, die ein Leser halten könnte; jeder Leser kopiert in Blöcken von 256 KiB und prüft danach. Jeder
Rückstau gehört genau einem Abnehmer und hat eine feste Grenze; er zählt erst bei Gebrauch gegen das
Byte-Budget, und stocken mehrere zugleich, teilen sie sich den Rest des Budgets (mindestens ein Viertel) zu
gleichen Teilen (`04` 8.1). Der Abwurf trifft nur den langsamen
Abnehmer. **Gestützt auf:** T-8, T-19; LTV-004.28, .29, .30, .36. **Fehlerzweige:** langsamer Client mit Zeitversatz
(liest aus seinem eigenen Sitzungspuffer und fällt höchstens aus seinem eigenen Fenster); Zähler der
Überrundungen in der Verwaltung.

### 3.18 Mehrere Aufnahmeprozesse und Zuschauer benötigen denselben Sender

**Ausgangslage.** Zwei Profile verschiedener Konten nehmen dieselbe Sendung auf Sender K auf, zwei weitere
schauen. **Die Benutzer sehen:** jede Aufnahme in den eigenen «Meine Aufnahmen», jeder Zuschauer seine
Wiedergabe.

```mermaid
sequenceDiagram
  participant CL1 as Client 1
  participant CL2 as Client 2
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
    participant AUF as Aufnahme-Schreiber
    participant PL as Planer
  end
  participant Q as Quelle
  participant DSK as Datenträger

  rect rgba(220,53,69,0.14)
    Note over PL,AUF: Aufnahme R1 von Profil A
    PL->>STR: Haltegrund Aufnahme R1
    STR->>EMP: Strom anlegen, falls er ruht
    EMP->>Q: eine Verbindung
    PL->>AUF: Schreiber R1, eigener Leser
  end
  rect rgba(220,53,69,0.14)
    Note over PL,AUF: Aufnahme R2 von Profil B
    PL->>STR: Haltegrund Aufnahme R2
    STR-->>PL: Strom läuft
    PL->>AUF: Schreiber R2, eigener Leser
  end
  rect rgba(66,135,245,0.14)
    Note over CL1,SES: Zuschauer S1 und S2
    CL1->>SES: einstellen
    SES->>STR: Haltegrund Zuschauer S1
    CL2->>SES: einstellen
    SES->>STR: Haltegrund Zuschauer S2
  end
  Note over STR: ein Eingangsstrom, vier Haltegründe, vier Leser
  par Aufnahme R1
    VT->>AUF: Pakete für R1
    AUF->>DSK: Teilstücke R1, Kontingent Konto A
  and Aufnahme R2
    VT->>AUF: Pakete für R2
    AUF->>DSK: Teilstücke R2, Kontingent Konto B
  and Zuschauer
    VT->>SES: Leser S1 und S2, Segmente wie Ablauf 3.3
  end
  PL->>STR: R1 endet mit Nachlauf, Haltegrund R1 lösen
  STR->>STR: R2, S1, S2 halten weiter
```

**Sichergestellt.** Beliebig viele Aufnahmen und Zuschauer teilen einen Eingangsstrom; jede Aufnahme hat
eigenen Leser, eigenen Rückstau und eigene, private Teilstücke im Kontingent ihres Kontos. Zwei Aufnahmen
desselben oder verschiedener Sender zur selben Zeit sind kein Konflikt. Ein Quellwechsel gibt allen
Aufnahmen zugleich einen neuen Teil. Bei Mangel wird keiner dieser Abnehmer verdrängt. **Gestützt auf:** T-2, T-8,
T-16, T-17, T-19; LTV-004.02, .06, .26, LTV-008.10, LTV-002.10, .11, DR-11. **Fehlerzweige:** Kontingent eines
Kontos voll (nur diese Aufnahme abgelehnt); eine Aufnahme hat einen stockenden Datenträger (nur sie bekommt
eine Lücke, 3.17).

## 4. Zusätzliche Abläufe

### 4.1 Gleichzeitige Erstanfragen zweier Benutzer auf denselben Sender

**Ausgangslage.** Sender K ruht; zwei Clients stellen ihn im selben Augenblick ein. Ein dritter kommt, während
der Strom gerade endet. **Sichergestellt ist:** genau ein Eingangsstrom, kein verlorener und kein doppelter.

```mermaid
sequenceDiagram
  participant CL1 as Client 1
  participant CL2 as Client 2
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant EMP as Empfang
  end
  participant Q as Quelle

  rect rgba(66,135,245,0.14)
    Note over CL1,STR: Zuschauer S1 und S2
    par Anfrage S1
      CL1->>SES: einstellen, Sender K
      SES->>STR: suchen oder anlegen für S1
    and Anfrage S2
      CL2->>SES: einstellen, Sender K
      SES->>STR: suchen oder anlegen für S2
    end
    STR->>STR: Sperre des Verzeichnisses, S1 kommt zuerst
    STR->>STR: kein Strom, zweite Prüfung nach dem Lesen der Quellen, anlegen
    STR->>STR: Beitrittszähler plus 1, Haltegrund S1
    STR->>EMP: verbinden
    STR->>STR: Sperre für S2, Strom gefunden im Zustand Verbindet
    STR->>STR: Beitrittszähler plus 1, Haltegrund S2
    EMP->>Q: eine Verbindung
    Q-->>EMP: TS-Daten
    EMP-->>STR: läuft
    STR-->>SES: beide Sitzungen bekommen Leser
  end
  opt dritter Beitritt während der Strom endet
    STR->>STR: Beitritt wartet auf das Ende
    STR->>EMP: Strom neu starten
  end
```

**Sichergestellt.** «Suchen oder anlegen» ist ein Schritt unter einer Sperre mit zweiter Prüfung; der
Beitrittszähler schützt den Strom vor der Aufräumschleife. Anders als beim Radio wird ein Beitritt zu einem
endenden Strom nicht abgelehnt, sondern wartet und startet neu. Zwanzig gleichzeitige Anfragen ergeben einen
Strom und zwanzig Haltegründe. **Gestützt auf:** T-2, T-17; LTV-004.22 bis .26. **Fehlerzweige:** Zulassung
verweigert (beide bekommen «ausgelastet»); Verbindung scheitert (beide Haltegründe sehen dieselbe Leiter).

### 4.2 Vorladen eines Nachbarsenders, sein Abbau und das Ruhen bei einer Quelle mit einer Verbindung

**Ausgangslage.** Sitzung S schaltet in der Liste «nach Kanalnummer» von Sender 4 auf 5 und bleibt dort.
Vorladen ist global eingeschaltet (Vorgabe bis zur Antwort auf F-9: an, mit vorsichtigen Obergrenzen). Eine
Einstellung je Quelle gibt es nicht. **Der Benutzer sieht:** Beim nächsten Umschalten auf 6 steht das Bild
sofort (Fall 2). Hat der Zugang nur eine Verbindung, stockt sein Bild beim ersten Vorfall höchstens einmal
kurz (Muster B).

```mermaid
sequenceDiagram
  participant CL as Client
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant ZUL as Zulassung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant VT as Verteiler
  end
  participant Q as Quelle

  rect rgba(66,135,245,0.14)
    Note over CL,SES: Zuschauer auf Sender 5
    CL->>SES: einstellen, Sender 5, Liste Kanalnummer, Richtung weiter
    SES->>SES: 2 s Verweilen abwarten
  end
  rect rgba(255,193,7,0.18)
    Note over STR,VT: Vorladen, Haltegrund Vorladen, Besitzer Sitzung S
    SES->>STR: Nachbarn 6 in Richtung weiter, dazu 4 als zuvor gesehen
    STR->>STR: Vorladen global an, vorsorglicher Empfang der Quelle ruht nicht, kein Ein-Strom-Betrieb
    STR->>ZUL: höchstens je Richtung und insgesamt 4, innerhalb der Obergrenze
    alt kein Platz
      ZUL-->>STR: Vorladen entfällt still
    else zugelassen
      STR->>STR: nur Ströme derselben Quelle ohne Haltegrund beenden
      Note over STR: Vorgeladenes derselben Quelle, etwa Sender 4, läuft nebeneinander weiter
      STR->>EMP: Sender 6 öffnen, ein Versuch, keine Leiter
      EMP->>Q: Verbindung
      Q-->>EMP: TS-Daten
      EMP->>VT: Ring ohne Sitzungspuffer und ohne Umwandlung
    end
  end
  alt Zuschauer schaltet auf 6
    rect rgba(66,135,245,0.14)
      CL->>SES: einstellen, Sender 6
      SES->>STR: Haltegrund Zuschauer, Strom läuft, wie Ablauf 3.2
    end
  else Abbau
    rect rgba(255,193,7,0.18)
      Note over STR: sofort, wenn 6 kein Nachbar mehr ist, 3 min nach dem letzten Umschalten oder beim Sitzungsende
      Note over STR: bei Mangel, Rangfolge 1, oder Sender deaktiviert
      STR->>EMP: Strom beenden, keine Karenz
      EMP->>Q: schliessen
    end
  else Muster A, das Vorladen wird abgelehnt, während Sender 5 dieselbe Quelle hält
    rect rgba(255,193,7,0.18)
      STR->>STR: Versuch entfällt still, nie Zugang prüfen
      STR->>STR: vorsorglicher Empfang der Quelle ruht, 30 min, dann 6 h, dann 24 h
    end
  else Muster B, die Quelle trennt die ältere Verbindung von Sender 5
    rect rgba(66,135,245,0.14)
      Q--xEMP: Verbindung von Sender 5 abgebrochen innert 10 s
      STR->>EMP: neue Verbindung von Sender 6 schliessen, der Zuschauer erfüllt einen Auftrag
      STR->>EMP: Sender 5 verbindet einmal sofort ohne Wartezeit
      EMP->>Q: Verbindung Sender 5
    end
    rect rgba(255,193,7,0.18)
      STR->>STR: Ein-Strom-Betrieb und Ruhen des vorsorglichen Empfangs für die Frist
      Note over STR: Verwaltung zeigt den Hinweis, Vorladen global ausschalten
    end
  end
```

**Sichergestellt.** Vorladen ist nur Empfang in den Ring: kein Sitzungspuffer, keine Umwandlung, keine
Leiter, keine Karenz; wer schnell durchschaltet, löst kein Vorladen aus; der Client meldet nur Liste und
Richtung, der Server entscheidet. Es gibt keine Einstellung und keine Zahl je Quelle (LTV-017); Vorladen,
«automatisch» und Prüfungen sind nur global und je Sender gesteuert. Mehrere vorgeladene Sender derselben
Quelle laufen nebeneinander; ein Verbindungsaufbau beendet vorher nur Ströme ohne Haltegrund. Wehrt sich die
Quelle (Muster A oder B), ruht vorsorglicher Empfang von selbst (30 Minuten, 6 Stunden, 24 Stunden), bei
Muster B zusätzlich mit Ein-Strom-Betrieb, und es gibt höchstens einen Wechsel (2.4). **Gestützt auf:** T-17,
T-18, T-19; LTV-002.07, LTV-010.05, .08, LTV-010.X-01, LTV-017. **Fehlerzweige:** Vorladen global aus (kein
Vorladen); Mangel (Vorladen weicht nach den Strömen ohne Haltegrund als Erstes). Ob es zusätzlich eine
Einstellung je Quelle geben soll, ist F-3.

### 4.3 Herunterfahren in 10 Sekunden

**Ausgangslage.** Docker sendet das Stoppsignal; heute beendet Docker nach 10 Sekunden hart, weil die
Standard-Compose-Datei keine Stoppfrist setzt. Es laufen Wiedergaben und eine Aufnahme. **Der Benutzer sieht:**
die Wiedergabe endet; die Aufnahme ist bis zum Abschluss sauber geschrieben.

```mermaid
sequenceDiagram
  participant CL as Client
  participant API as API und Handler
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant AUF as Aufnahme-Schreiber
    participant UW as Umwandlung
  end
  participant DB as SQLite
  participant DSK as Datenträger

  Note over API,DSK: Stoppsignal
  par HTTP wartet bis 10 s auf offene Anfragen
    API->>API: keine neuen Verbindungen, offene Anfragen abwarten
  and eigener Abschluss von live beginnt gleichzeitig
    rect rgba(66,135,245,0.14)
      SES->>SES: neue Sitzungen ablehnen
      CL->>API: einstellen
      API-->>CL: abgelehnt
    end
    rect rgba(220,53,69,0.14)
      AUF->>DSK: Aufnahmeteile schliessen
      AUF->>DB: Stand schreiben, Ziel unter 3 s
    end
    SES->>UW: FFmpeg beenden
    STR->>STR: Empfang beenden
  end
  Note over UW,DB: erst danach schliessen Umwandlungs-Manager und Datenbank
  Note over API,DSK: nach 10 s beendet Docker hart, ab Stufe 5 stop_grace_period 30 s
```

**Sichergestellt.** Der Abschluss von `live` wartet nicht, bis die HTTP-Anfragen fertig sind, sondern
beginnt gleichzeitig; Aufnahmeteile sind vor den 10 Sekunden geschlossen, und Datenbank und
Umwandlungs-Manager schliessen erst danach. Was der harte Abbruch trotzdem trifft, behandelt der Neustart
(3.12) als Lücke. **Gestützt auf:** `04` 5.3 «Herunterfahren», 10.6; LTV-008.26, LTV-020.20. **Fehlerzweige:**
Abschluss braucht länger als 10 s (bis Stufe 5 harter Abbruch, danach 30 s Frist); Absturz ohne Signal (nur
3.12).

### 4.4 Eine Quelle wird entfernt oder geändert

**Ausgangslage.** Der Administrator entfernt eine Quelle, ändert ihre Adresse, Zugangsdaten, Ursprünge oder
festen Ziele oder entzieht ein festes Ziel, oft weil ein Zugang offengelegt oder eine Liste gekapert ist. Auf
Sendern dieser Quelle laufen ein Zuschauer, eine Aufnahme und ein Dauersender. **Der Benutzer sieht:** eine
kurze Unterbrechung, wenn es eine Ersatzquelle gibt; sonst «Sender nicht verfügbar».

```mermaid
sequenceDiagram
  participant CLV as Client Verwaltung
  participant CL as Client
  participant API as API und Handler
  box rgba(128,128,128,0.08) Dienst live
    participant SES as Sitzung
    participant STR as Stromverwaltung
    participant EMP as Empfang
    participant AUF as Aufnahme-Schreiber
  end
  participant CAT as catalog
  participant DB as SQLite
  participant Q1 as Quelle 1
  participant Q2 as Quelle 2

  CLV->>API: Quelle entfernen oder ändern
  API->>CAT: Zahl der betroffenen Zuschauer und Aufnahmen
  CAT-->>CLV: Rückfrage mit Zahlen
  CLV->>API: bestätigt
  CAT->>DB: neuer Regelstand, Versionsnummer der Quelle steigt
  CAT->>STR: Empfänger dieser Quelle beenden
  STR->>EMP: alle Empfänger der Quelle enden innert 5 s
  EMP->>Q1: Verbindung schliessen, entschlüsselte Werte und Verbindungsvorrat freigeben
  Note over EMP: jeder Aufbau prüft die Versionsnummer, mit einem alten Stand verbindet er nie
  alt Quelle geändert
    rect rgba(66,135,245,0.14)
      STR->>EMP: gehaltene Ströme verbinden sofort mit dem neuen Stand
      EMP->>Q1: Verbindung mit neuem Regelstand
    end
  else Quelle entfernt, Ersatzquelle vorhanden
    STR->>CAT: nächste Quelle der Rangfolge
    STR->>EMP: Wiederverbinden mit Q2, neuer Zeitabschnitt wie Ablauf 3.10
    EMP->>Q2: Verbindung
  else keine Quelle mehr
    rect rgba(66,135,245,0.14)
      SES-->>CL: Sender nicht verfügbar
    end
    rect rgba(220,53,69,0.14)
      AUF->>DB: Lücke mit Grund, ohne Quelle bis zum Ende unvollständig abgeschlossen
    end
    rect rgba(128,128,128,0.16)
      STR->>STR: Dauersender zurückgestellt, Wiederanlauf mit der dann ranghöchsten Quelle
    end
  end
```

**Sichergestellt.** Anders als das Deaktivieren eines Senders (T-29, 3.16) ist das Entfernen oder Ändern einer
Quelle ein Sicherheitsmittel: Nach 5 Sekunden hält kein Empfänger mehr eine Verbindung mit dem alten Stand,
und kein Empfänger verbindet je mit einem Stand, der nicht mehr in der Datenbank steht. **Gestützt auf:** T-5,
T-28, T-29; `04` 5.1, 5.3 «Quelle entfernt oder geändert»; `12` (strenger als 04 gilt 12). **Fehlerzweige:**
HLS-Quelle mitten im Abruf einer Liste oder eines Segments (die Prüfung der Versionsnummer gilt je Abruf);
geänderte Quelle lehnt ab (Leiter, Deutung der Ablehnung, 2.3).

## 5. Nebenläufigkeit

Stellen, an denen zwei Dinge gleichzeitig geschehen können, und die Regel aus `04` oder `05` (Fassung 4), die
sie ordnet.

| Nr. | Was gleichzeitig geschieht | Regel | Stelle |
|---|---|---|---|
| N-1 | zwei Erstanfragen auf denselben Sender | «suchen oder anlegen» unter der Sperre des Verzeichnisses, zweite Prüfung nach dem Lesen der Quellen | `04` 5.3, Ablauf 4.1 |
| N-2 | Abbau eines Stroms gegen neuen Abnehmer | Beitrittszähler schützt vor der Aufräumschleife; Beitritt zu einem Strom in «Endet» wartet höchstens 1 s und startet ihn neu | `04` 5.1, 5.3 |
| N-3 | Karenz gegen neuen Haltegrund | neuer Haltegrund beendet die Karenz, Strom läuft weiter | `04` 5.1 |
| N-4 | Karenz gegen Abbruch der Quelle oder Deaktivieren | Strom endet sofort, keine Leiter, keine Quarantäne | `04` 5.1, 5.3 |
| N-5 | neue Verbindung gegen Ströme derselben Quelle | vorher enden nur Ströme **ohne Haltegrund** (Ende abwarten höchstens 1 s); Vorgeladenes und automatisch Gehaltenes bleibt | `04` 5.3, `05` 6 |
| N-6 | zwei Vorladungen derselben Quelle | laufen nebeneinander; es gibt keine Einstellung je Quelle, nur das globale Vorladen | `04` 5.3, 6.1, `05` 6 |
| N-7 | Ablehnung für einen Abnehmer oder Dauersender (Muster A) gegen vorsorglichen Empfang derselben Quelle | bei dieser Quelle freigeben, was im Rang darunter steht (ohne Haltegrund, Vorgeladenes, Prüfungen, automatisch); neue Versuche nach 1 s und 2 s, höchstens drei in 5 s; nie ein Strom mit Zuschauer oder Aufnahme und nach der Vorgabe kein laufender Dauersender; vorsorglicher Empfang der Quelle ruht | `04` 5.3 |
| N-8 | gehaltener Strom bricht kurz nach einer anderen Verbindung zur selben Quelle ab (Muster B) | Erfüllte er einen Auftrag, schliesst der Server die neue Verbindung und er verbindet einmal sofort; war er vorsorglich, bleibt die neue; danach Ein-Strom-Betrieb und Ruhen; höchstens ein Wechsel | `04` 5.3 |
| N-9 | mehrere Wartende im Ein-Strom-Betrieb (nach Neustart, nach Ende des bisherigen Stroms) | Wer die Verbindung hat und einen Auftrag erfüllt, behält sie; unter Gleichzeitigen Rangfolge Aufnahme, Zuschauer, Dauersender, «automatisch», Prüfung, Vorladen; nach dem Ende verbindet der nächste Wartende | `04` 5.3 |
| N-10 | beginnende Aufnahme gegen laufenden Zuschauer oder laufenden Dauersender derselben Quelle im Ein-Strom-Betrieb | bis zur Antwort auf F-5 und F-4: kein Verdrängen; die Aufnahme scheitert mit «Quelle lehnt eine weitere Verbindung ab» | `04` 5.3, F-4, F-5 |
| N-11 | Ruhen des vorsorglichen Empfangs gegen neues Auftreten eines Musters | Frist 30 min, dann 6 h, dann 24 h; zurück auf 30 min nach 7 Tagen ohne Muster oder wenn der Administrator die Quelle ändert oder testet | `04` 5.3 |
| N-12 | neuer Abnehmer gegen wartenden oder gestörten Strom | ein Versuch sofort, höchstens einer je Minute und Sender; danach weiter auf der Leiter | `04` 5.3 |
| N-13 | kurz liefernde, wieder abreissende Verbindung gegen die Leiter | die Leiter setzt erst nach 60 s Lauf ohne Abbruch zurück | `04` 5.3 |
| N-14 | Schreiber des Rings gegen Leser | Schreiber wartet nie; Leser kopiert, prüft danach, verwirft bei Überrundung und setzt am nächsten Keyframe fort; GOP-Index unter kurzer Lesesperre; «neue Daten» als ersetzter Kanal | T-8, `05` 3.5 |
| N-15 | mehrere stockende Schreiber gegen das Byte-Budget | Rückstau zählt bei Gebrauch; der Rest des Budgets wird zu gleichen Teilen geteilt; wer seinen Teil ausschöpft, bekommt eine Lücke; niemand wird abgelehnt | `04` 8.1 |
| N-16 | Quellwechsel gegen Aufnahme | neuer Zeitabschnitt, Aufnahme beginnt einen neuen Teil, bricht nicht ab | T-6, T-16, Ablauf 3.10 |
| N-17 | neuer Zeitabschnitt oder höhere Bitrate gegen volles Byte-Budget | Rangfolge 8.2, sonst behält der Strom seinen bisherigen Ring («Startpuffer verkürzt»); ein gehaltener Strom endet nie deswegen | `04` 5.3 |
| N-18 | Quell- oder Parameterwechsel gegen laufende Umwandlung | Sitzung berechnet den Entscheid neu, FFmpeg startet neu | `05` 3.4 |
| N-19 | Rechteentzug gegen Segmentabruf | Haken beendet die Sitzung, Token ungültig, jede Anfrage 404; Sicherung: Listenanfrage prüft Recht höchstens 30 s alt; Strom ohne anderen Haltegrund endet sofort | T-21, `04` 9, 10.3 |
| N-20 | Entzug von «Timeshift» oder Ausfall des Speicherorts gegen zeitversetzte Wiedergabe | Sitzungspuffer endet, Sitzung läuft aus dem Ring, Sprung an die Live-Kante (neue Ausgabe-Generation); nach Rückkehr des Orts nach 60 s neuer Puffer | `04` 9 |
| N-21 | Qualitäts- oder Spurwechsel gegen laufenden FFmpeg | neue Ausgabe-Generation; Reservierung umgebucht; neuer Prozess wartet nicht auf das Ende des alten | T-13, `04` 7.5 |
| N-22 | Spulen mit Umwandlung, mehrere Anfragen ausserhalb des Erzeugten | höchstens 2 Neustarts von FFmpeg je Sitzung in 10 s, weitere Anfragen warten | `04` 8.5 |
| N-23 | schnelles Umschalten gegen Vorladen und Verbindungen | Drosselung 1 je Sekunde, Stoss 5; Vorladen erst nach 2 s Verweilen; keine Karenz beim Umschalten | `04` 8.5, `05` 6 |
| N-24 | Einstellen gegen ein neueres Einstellen derselben Sitzung | die wartende ältere Antwort endet mit `409 live_superseded`; Haltegrund und Reservierung gehen zurück; nie zwei wartende Einstellvorgänge je Sitzung | `04` 9 |
| N-25 | Einstellen gegen langsamen Start | Antwort wartet bis 3 s, sonst Zustand «startet» mit Grund; der Client lädt die Liste erst bei «läuft»; drei Fristen, zusammen höchstens 90 s | `04` 5.3, 9 |
| N-26 | neue Umwandlungen gegeneinander | Reservieren wartet bis 10 s in Ankunftsreihenfolge, dann «ausgelastet»; ohne wirksame Sperre gibt es keine Live-Umwandlung | `04` 8.1, T-27 |
| N-27 | Start der Stream-Grenze gegen Dateiwiedergabe | `live` reserviert seinen Platz unter derselben Startsperre wie `playback` | T-26 |
| N-28 | Listen-Abgleich gegen aktiven Strom | laufende Verbindung bleibt; neue Adresse eines Senders gilt ab der nächsten Verbindung | `04` 9 |
| N-29 | Deaktivieren eines Senders gegen laufende Wiedergabe und Aufnahme | Laufendes mit Abnehmer läuft weiter, Wiedergabe höchstens bis zum Ablauf des Tokens (12 h), Sofortaufnahme höchstens 4 h; Vorsorgliches und Dauersender dieses Senders enden sofort; «sofort beenden» ist ein eigener, bestätigter Befehl | T-29, `04` 5.3 |
| N-30 | Entfernen oder Ändern einer Quelle gegen laufende Empfänger | alle Empfänger der Quelle enden innert 5 s; jeder Aufbau prüft die Versionsnummer des Regelstands | `04` 5.1, 5.3, Ablauf 4.4 |
| N-31 | zurückgestellter Dauersender gegen neuen Abnehmer oder freien Platz | Abnehmer startet ihn sofort; sonst Wiederanlauf als vorsorglicher Versuch erst nach 60 s erfüllter Bedingung, der am längsten Zurückgestellte zuerst | `04` 5.3 |
| N-32 | Mangel gegen laufenden Dauersender | Dauersender weicht nicht (Vorgabe F-4 (b)); die neue Anfrage wird abgelehnt | `04` 8.2 |
| N-33 | Haltegrund gegen verschwundenen Besitzer | Abgleich einmal je Minute, verwaiste Einträge entfernt und protokolliert | `04` 5.2 |
| N-34 | lange Importe gegen Sitzungs- und Aufnahmezeilen | ein Schreiber in SQLite; Stapel bis 500 Zeilen; Zustände nur bei Wechsel | T-22, `04` 4.2 |
| N-35 | Herunterfahren gegen offene Anfragen | Abschluss von `live` beginnt gleichzeitig mit dem Warten; neue Sitzungen abgelehnt | `04` 5.3, Ablauf 4.3 |
| N-36 | Serverstart: weiterzuführende Aufnahmen gegen Dauersender | Aufnahmen zuerst, sofort, ohne Staffelung (verschiedene Quellen gleichzeitig, dieselbe Quelle nacheinander); danach Dauersender nach Kanalnummer gestaffelt, höchstens 2 Starts je Sekunde, nie zwei gleichzeitig bei derselben Quelle; übrige zurückgestellt | `04` 5.3 |

## 6. Was nach `04` Fassung 4 offen bleibt

Dieses Dokument trifft keine Entscheide. Wo Marco eine Frage nicht beantwortet hat, zeichnen die Abläufe die
**wortlauttreue Vorgabe**, bei Sicherheitsfragen die sichere; Varianten stehen nur als Variante dabei.

| Frage oder Punkt | Gezeichnet ist die Vorgabe | Wo es in den Abläufen auffällt | Stelle |
|---|---|---|---|
| F-3 Einstellung oder Zahl je Quelle | keine Einstellung je Quelle; Muster A und B, Ruhen, Ein-Strom-Betrieb | 2.4, 3.5, 4.2 | `04` 12.3 (→ `20`) |
| F-4 Vorrang Zuschauer gegen laufenden Dauersender | (b): ein laufender Dauersender weicht niemandem; (a) nur als Variante | 2.1, 3.14, N-32 | `04` 12.3 (→ `20` E-28-30) |
| F-5 Vorrang beginnende Aufnahme gegen Wiedergaben | kein Verdrängen; die Aufnahme scheitert mit Grund | 3.8, 3.14, N-10 | `04` 12.3, `03` F-4 |
| F-7 Rückspulgrenze am Tastendruck | (a) | 3.2, 3.6 | `04` 12.3 |
| F-8 eigene Kopie des Zeitversatzes je Sitzung | (a) | 3.3 | `04` 12.3 |
| F-9 Werkszustand des globalen Vorladens und von «automatisch» | Vorladen an mit vorsichtigen Obergrenzen | 4.2 | `04` 12.3 |
| Umwandlung ohne wirksame Sperre | kein Live-FFmpeg ohne Sperre; nur Direktweg, «Eindämmung nicht verfügbar» | 3.4 | `12` F-1 (→ `20` E-28-04) |
| Laufende Aufnahmen und Planungen beim Entzug des Rechts «Aufnahmen» | – | 3.15 | `03` F-7, `04` O-9 |
| Herzschlag im Browser-Tab im Hintergrund und bei Bildübertragung | Regel der 60 s ohne Ausnahme | 3.11 | `04` O-25, M-3, M-5 |
| Bis zu sechs Segmente für native Apple-Player ohne Recht «Timeshift» | drei neueste Segmente | 3.6 | `05` O-28, M-5 |
| Nahtloses Spulen mit Umwandlung über mehrere FFmpeg-Läufe | Rückfall «spiele ab Uhrzeit» | 3.6 | M-9, `05` 4.5 |
| Zielwerte der Umschaltzeit (Fälle 1 bis 4) | keine Zusage | 3.1, 3.2, 3.4 | zu prüfen (M-1), `05` 5 |

## 7. Änderungen gegenüber der vorigen Fassung

### 7.1 Fassung 2 → 3 (Schlussbereinigung, 29. November 2026)

Grundlage: `04`/`05` Fassung 4 (5.1, 5.3, 8.2, 9, 10.3, 14, 15.3), `12` Fassung 3,
`.omc/research/live-tv/ENTSCHEIDE-KOORDINATOR-NACH-REVIEW-4.md`, Reviews 4 bis 6.

- Kopf und Einleitung: Fassung 4 als Grundlage; Grundregel «ohne Marcos Antwort gilt die wortlauttreue
  Vorgabe» (RG-01); «Belegt» heisst jetzt «Gestützt auf», Zielwerte «zu prüfen (M-1)» (RB-21).
- 2.1 bis 2.3 neu aus `04` 5.1 abgeleitet: Zustand «Endet»; «zurückgestellt» nur, wenn ein Dauersender nicht
  starten kann; ein laufender Dauersender weicht niemandem; Ereignisse in jedem Zustand; Mangel, neuer
  Abnehmer auf «Wartet» oder «Gestört», Fehlschlag des Wiederanlaufs, Quelle entfernt, Rückkehr zur
  Hauptquelle; keine eigenen Übergänge mehr (RG-02, `04` 15.3).
- 2.4 neu: kein Schalter und keine Zahl je Quelle; Muster A und B, Ruhen 30 min, 6 h, 24 h, Ein-Strom-Betrieb
  (RG-03, RB-03, RB-04).
- 3.1: drei Startfristen und Gesamtfrist 90 s statt 15 s; Liste erst bei «läuft»; `409 live_superseded`;
  Probe nur mit Sperre (RB-02, RB-15, RT-01).
- 3.2: lange Bildgruppen nach `05` 3.6 (Ring bis 30 s).
- 3.4: Zweig «Eindämmung nicht verfügbar», kein Live-FFmpeg ohne Sperre (RT-01).
- 3.5, 3.8, 3.10, 4.2: Schalter je Quelle entfernt; Muster A mit drei Versuchen in 5 s, Muster B mit einem
  Wechsel; kein Verdrängen laufender Dauersender (RG-02, RG-03).
- 3.6, 3.11: Schauzeit nach dem, was der Server ausliefert; Sitzungsende nach 15 min ohne Segmentabruf und nach
  60 s gemeldeter Pause ohne Sitzungspuffer; App 30 s im Hintergrund (RT-04, RT-11, RB-11).
- 3.9: Leiter setzt erst nach 60 s gesundem Lauf zurück; neuer Abnehmer löst einen Versuch sofort aus.
- 3.12: Dauersender nach Kanalnummer, übrige zurückgestellt; Muster nach Neustart vergessen.
- 3.14 neu: Rangfolge 0 bis 4, dann Ablehnung; Dauersender weichen nicht; Wiederanlauf eines nicht gestarteten
  Dauersenders als vorsorglicher Versuch; F-4 (a) nur als Variante (RG-02).
- 3.16 neu: T-29 mit Grenzen (Token 12 h, Sofortaufnahme 4 h), «sofort beenden» (RB-05).
- 4.4 neu: Quelle entfernt oder geändert beendet ihre Empfänger innert 5 s (RB-05, RT-06).
- 5: Nebenläufigkeit N-1 bis N-36 neu gefasst.
- 6: offene Punkte mit der gezeichneten Vorgabe je Frage.

### 7.2 Fassung 1 → 2

Fassung 1 → 2, Grundlage `04` und `05` Fassung 3 (Abschnitt 13.4, 14, 15.1) und
`.omc/research/live-tv/KONSOLIDIERUNG.md`.

- Kopf, Einleitung: Bezug auf `04`/`05` Fassung 3, T-1 bis T-29, Werte aus `04` 14, Stufen nach `18`.
- Teilnehmer: Weg W mit Ersatzpfad beim Go-Weiterleiter statt Ersatzname, keine Proxy-Einstellung
  (`04` 15.1, `05` 2.6).
- 2.1 Zustandsdiagramm neu: Zustand «zurückgestellt» mit Wiederanlauf; Karenz nur, wenn der Zuschauer selbst
  geht; Quellabbruch in der Karenz beendet sofort; Ablehnung gibt bei der Quelle Vorsorgliches frei
  (`06` O-1, O-2, O-3, O-4, O-5).
- 2.2 und 2.3 neu: Haltegründe mit Freigabe je Anlass, T-29, Aufwärmzeit vor dem Vorlauf, neuer
  Zeitabschnitt bei knappem Budget, «einstellen» mit 3 s und 15 s (`06` O-6, O-7, O-9, O-10).
- 3.1: Warten bis 3 s auf das erste Segment, Zustand «startet», Abbruch nach 15 s (`06` O-9).
- 3.5, 3.8, 3.10, 4.2: vor einem Aufbau enden nur Ströme ohne Haltegrund; Vorgeladenes derselben Quelle läuft
  nebeneinander und weicht bei Ablehnung (`06` O-1).
- 3.6: Herzschlag meldet «spielt» oder «Pause»; Schauzeit zählt ausgelieferte Segmente (`06` O-11).
- 3.7, 3.8: Aufwärmzeit 30 s vor dem Beginn des Vorlaufs; nach dem Ende sofort «ruht» (`06` O-2, O-10).
- 3.10: neuer Ring nach Rangfolge, sonst bisheriger Ring «Startpuffer verkürzt» (`06` O-6).
- 3.11: Hintergrund und «Bildschirm aus» beenden die Sitzung (Apps); Quellabbruch in der Karenz (`06` O-5,
  `04` 10.3).
- 3.12: Aufnahmen verbinden sofort ohne Staffelung, Staffelung nur für dauerhaft und Zeitfenster (`06` O-12).
- 3.13: Puffer enden, Sprung an die Live-Kante, neuer Puffer 60 s nach Rückkehr des Orts (`06` O-8).
- 3.14: Stufe 5 stellt zurück; Wiederanlauf; Rückstau zählt bei Gebrauch (`06` O-4, `13` H-1).
- 3.15: Ende durch den Server ohne Karenz (`06` O-3).
- 3.16: T-29, Deaktivieren beendet nichts Laufendes, «sofort beenden» als eigener Befehl (`06` O-7,
  `07` 12.3).
- 3.17: Lesen in Blöcken von 256 KiB, Teilung des Rückstaus (`04` 8.1).
- 5: Nebenläufigkeit neu gefasst (N-1 bis N-29).
- 6: Liste «offen in 04» ersetzt durch die Weichen, die `04`, `05` und `03` selbst offen führen.
- Restebereinigung nach Konsolidierung 1: Schalter je Quelle heisst «Vorladen für Sender dieser Quelle: an/aus»
  (Ablauf 3 und Ablauf zum Vorladen; Koordinator zu RM-01).
- Restebereinigung nach Konsolidierung 1, Nachtrag: Deutung «Quelle lehnt eine weitere Verbindung ab» (N-8; `20` U-18).
- Nach der Schlussprüfung (Review 7): Ring nach `04` T-7 (15 s, bei langen Segmenten das Vierfache der Segmentdauer, höchstens 30 s) an vier Stellen.
