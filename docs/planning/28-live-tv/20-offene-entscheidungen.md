# Konzept 28 · Offene Entscheidungen

Stand: 28. November 2026 · Fassung 1 · Status: Entwurf, nicht freigegeben

> **Kurz gesagt.** Diese Liste sammelt alles, was du vor dem Bau von Live-TV entscheiden sollst, aus den Papieren
> 02 bis 19, den vier Reviews und der Nachprüfung der Quellen. Aus 62 Fragen der Papiere, neun Vorrangkonflikten
> und weiteren Punkten für dich sind **58 Fragen** geworden (`E-28-01` bis `E-28-58`): **34 Weichen** und
> **24 Feinschliff**. Dazu kommen 61 offene technische Punkte für die Entwicklung (Teil B), die Messungen vor dem
> Bau (Teil C), die Risiken (Teil D) und die Unstimmigkeiten, die zwischen den Papieren noch bestehen (Teil E).
> Nichts davon ist gebaut; alles ist spezifiziert, empfohlen oder zu messen.

## So liest du diese Liste

1. **Wortlauttreue Vorgabe:** Bis zu deiner Antwort gilt, was dein Auftrag vom 28. November 2026 wörtlich sagt. Weicht meine Empfehlung davon ab, steht in der Zeile «Vorgabe» die Variante nach dem Wortlaut, und die Frage trägt den Vermerk **«Wortlaut»**.
2. **Wenn du nichts entscheidest:** Mit der Freigabe des Konzepts gilt für jede offene Frage die Zeile «Vorgabe» (`18` 1.1). Eine spätere andere Antwort kostet Umbau, umso mehr, je später sie kommt.
3. **Fälligkeit:** Teil A ist nach der Stufe geordnet, vor der die Antwort da sein muss (Stufen nach `18-umsetzungsplan.md`: V, 0, 1a, 1b, 2, 3, H, 4, 5, 6, L, W, E, Apps). Für die Baufreigabe von Stufe 0 brauchst du nur die Gruppe A.1 (8 Fragen), für Stufe 1 zusätzlich A.2 (13 Fragen).
4. **Weiche** heisst: Die Antwort ändert Umfang, Sicherheit oder sichtbares Verhalten. **Feinschliff** heisst: Detail innerhalb eines festen Umfangs; die Empfehlung ist ein sicherer Weg.
5. **Schon entschieden und hier nicht mehr gefragt:** deine vier Antworten E-1 bis E-4 (`02` Abschnitt 3), alles aus den Leitplanken (Rechte-Modell, keine Kopplung an die Altersgrenze, Zählung zu den Grenzen, kein Download, Aufnahmen nie in der Bibliothek) und die 21 Grundsatzentscheide.

### Bereits beantwortet (nur Verweis)

| Nr. | Thema | Deine Antwort (Kurzform) | Stelle |
|---|---|---|---|
| E-1 | Zielprodukt | aktuelles Go-Produkt; Abschnitt 5 des Auftrags nur im Sinn | `02` 3 |
| E-2 | Umfang der Mockups | Web, Android TV und Handy-App | `02` 3 |
| E-3 | Aufnahmen von Profilen mit Altersgrenze | Hauptprofil darf sie sehen, löschen und abspielen; andere bleiben ganz privat | `02` 3 |
| E-4 | Kontingent voll | neue Aufnahme abgelehnt und gemeldet; automatisch gelöscht nur nach eigener Frist und «behalte N» | `02` 3 |

## Kurzübersicht

«Wortlaut» im Titel: Der Entwurf kann oder will eine verbindliche Anforderung des Auftrags nicht wörtlich oder
nicht in jeder Stufe erfüllen, oder die Empfehlung weicht vom Wortlaut ab.

| Kennung | Titel | Empfehlung | Fällig vor | Gewicht |
|---|---|---|---|---|
| E-28-01 | Welche Clients in welcher Stufe | (b) Web und TV-App je Stufe, Handy-App gesammelt | 0 | Weiche |
| E-28-02 | Verbindungslimit des Anbieters (Wortlaut) | (b) eine Zahl je Quelle, nur auf dein Wort; sonst (a) | 0 | Weiche |
| E-28-03 | Härtung der Standard-Compose-Datei für alle | (a) für alle, nach Probelauf | 0 | Weiche |
| E-28-04 | Umwandlung, wenn der Kernel keine Sperre erlaubt | (a) erlauben, mit sichtbarem Zustand | 0 | Weiche |
| E-28-05 | Weiterleitung von `https` nach `http` | (a) immer abbrechen | 0 | Weiche |
| E-28-06 | Zertifikatsprüfung abschaltbar | (a) nie | 0 | Weiche |
| E-28-07 | Aufnahmen und Altersgrenze: Zeitpunkt und Definition | A (b) beides verlangen; B (a) Grenze aus Profil und Konto | 0 | Weiche |
| E-28-08 | Eigenes Release für Stufe 0 | (a) nein, reist mit Stufe 1 | 0 | Feinschliff |
| E-28-09 | Werkseinstellung der Rechte | (a) Konten aus, Administratoren an | 1a | Weiche |
| E-28-10 | Speicherort des Startpuffers (Wortlaut) | (b) nur Arbeitsspeicher; Vorgabe (a) | 1a | Weiche |
| E-28-11 | Zugangsdaten in der «Sicherung ohne Geheimnisse» | (a) leeren | 1a | Weiche |
| E-28-12 | Safari, iPhone, iPad ohne Testgerät (Wortlaut) | (a) ungeprüft, nicht zugesagt | 1a | Weiche |
| E-28-13 | Schwellen des Benchmarks; Zielwerte hängen an der Quelle (Wortlaut) | (a) Median unter Ziel, p95 ≤ 2×, Fehler ≤ 2 % | 1a | Weiche |
| E-28-14 | Live-TV gegen Filme | (a) kein Borgen, Filmstart höchstens doppelt | 1a | Weiche |
| E-28-15 | Verwaltungsbereich | (C) eigener Bereich, erst nach «einrichten» | 1a | Feinschliff |
| E-28-16 | Eintrag in der Web-Navigation | (A) im Band «Meine Medien» | 1a | Feinschliff |
| E-28-17 | Sortierung der Sender | (A) nach Kanalnummer | 1a | Feinschliff |
| E-28-18 | Kategorien und Genres | (a) feste Liste, Genres wie geliefert | 1a | Feinschliff |
| E-28-19 | CPU-, RAM- und GPU-Werte in der Verwaltung | (A) anzeigen, wo lesbar | 1a | Feinschliff |
| E-28-20 | Geräteprüfungen und Kamera-Gegenprobe | (a) du nach Vorlage, einmal mit Zeitlupe | 1a | Feinschliff |
| E-28-21 | Blender-Filme im Benchmark | (a) nein, Testbild reicht | 1a | Feinschliff |
| E-28-22 | Kanäle für Erinnerungen und Aufnahme-Meldungen | (a) Glocke und Kanäle des Kontos | 2 | Weiche |
| E-28-23 | Programmsuche in der allgemeinen Suche | (a) nur im Live-TV-Bereich | 2 | Weiche |
| E-28-24 | Programmdaten in eigener Datei | (a) eigene Datei, nicht gesichert | 2 | Weiche |
| E-28-25 | EPG-Adressen im Heimnetz | (a) feste Ziele, in Stufe H | 2 | Weiche |
| E-28-26 | Namensgleiche Sender zuordnen | (a) nur Vorschlag | 2 | Feinschliff |
| E-28-27 | Sendungsbilder im Raster | (A) keine | 2 | Feinschliff |
| E-28-28 | Altersangaben der Programmdaten | (A) nie anzeigen | 2 | Feinschliff |
| E-28-29 | Vorladen ab Werk und Schalter je Quelle (Wortlaut) | (b) global an, je Quelle ab Werk aus | 3 | Weiche |
| E-28-30 | Neuer Zuschauer gegen Empfang ohne Abnehmer (Wortlaut) | (a) Zuschauer geht vor; Vorgabe (b) | 3 | Weiche |
| E-28-31 | Erkennung eingefrorener Bilder (Wortlaut) | (a) zwei Schritte, Stichprobe in Stufe E | 3 | Weiche |
| E-28-32 | Welche Ausfälle der Administrator gemeldet bekommt | (A) Dauer-, Zeitplan- und Aufnahmesender | 3 | Feinschliff |
| E-28-33 | Angabe der Anschlussbandbreite | (a) keine Angabe | 3 | Feinschliff |
| E-28-34 | Multicast: Netzmodus «host» oder eigenes Netz (Wortlaut) | (b) eigenes Netz, wenn V-6 es zeigt | H | Weiche |
| E-28-35 | Rückspulgrenze | (a) am Tastendruck | 4 | Weiche |
| E-28-36 | Speicherform des Timeshift-Puffers | (a) eigene Kopie je Sitzung | 4 | Weiche |
| E-28-37 | Sofortaufnahme im Zeitversatz | (a) ab jetzt (live) | 4 | Feinschliff |
| E-28-38 | Rückfrage beim Senderwechsel aus dem Zeitversatz | (B) ab 60 s Abstand | 4 | Feinschliff |
| E-28-39 | Vorgabe des Aufnahme-Kontingents | (a) 100 GB je Konto | 5 | Weiche |
| E-28-40 | Beginnende Aufnahme gegen Wiedergaben | (a) Platz 2 min vorher reservieren | 5 | Weiche |
| E-28-41 | Entzug des Rechts: Planungen und laufende Aufnahmen | (a) Planungen ruhen, laufende endet | 5 | Weiche |
| E-28-42 | Hauptprofil löscht, wenn das Konto kein Live-TV mehr hat | (a) ja, nur Liste und Löschen; Vorgabe (b) | 5 | Weiche |
| E-28-43 | Was das Hauptprofil ausser Aufnahmen sieht | (B) geplante Termine lesend, Meldung beim Löschen | 5 | Weiche |
| E-28-44 | Spuren ausserhalb des TS (Wortlaut) | (a) ehrlich anzeigen; Vorgabe (b) | 5 | Weiche |
| E-28-45 | Notfall-Löschen durch den Administrator | (A) nie | 5 | Weiche |
| E-28-46 | Laufende Aufnahme wächst über das Kontingent | (a) läuft zu Ende | 5 | Feinschliff |
| E-28-47 | Standard-Aufbewahrungsfrist des Administrators | (a) nur Vorschlag | 5 | Feinschliff |
| E-28-48 | Update, während eine Aufnahme läuft | (a) warnen und bestätigen | 5 | Feinschliff |
| E-28-49 | Höchstzahl gleichzeitiger Aufnahmen | (B) einstellbar, ab Werk keine | 5 | Feinschliff |
| E-28-50 | Gleiche Sendung, zwei Profile | D0 getrennte Dateien | 5 | Feinschliff |
| E-28-51 | Bestehende Aufnahmen beim Wechsel des Speicherorts | (A) alter Ort bleibt lesend | 5 | Feinschliff |
| E-28-52 | Serienaufnahme ohne Folgenkennung | (a) alles aufnehmen, mit Hinweis | 6 | Weiche |
| E-28-53 | Zusätzliche Felder der Serienregel | (B) Zeitfenster und Wochentage | 6 | Feinschliff |
| E-28-54 | RTSP und RTMP | (a) Go-Bibliotheken, späte Stufe | L | Weiche |
| E-28-55 | DASH und HLS mit fMP4, falls Weg W nicht dicht ist (Wortlaut) | (a) nur mit Sperre | W | Weiche |
| E-28-56 | Untertitel im Browser (Wortlaut) | (a) eigene Textspur im Player | E | Weiche |
| E-28-57 | Bedienung des TV-Players mit dem Steuerkreuz | (A) OK = Bedienleiste, ↑/↓ = Sender | Apps | Feinschliff |
| E-28-58 | Abnahme der TV-App-Ausgaben | (a) dieselbe Prüfliste am Gerät | Apps | Feinschliff |

---

## Teil A · Fragen an dich

### A.1 Vor Stufe 0 (vor der Baufreigabe)

#### E-28-01 · Welche Clients in welcher Stufe

- **Herkunft:** `03` F-6; `18` Abschnitt 5 und D-10 · **Bezug:** LTV-014, LTV-021; Regel 8 (Tastentest)
- **Worum es geht:** Jede Stufe kann zuerst nur in der Web-App kommen, gleich auch auf dem Fernseher oder auf
  allen drei Geräten. Davon hängt ab, wie gross jede Stufe wird, wann der Fernseher Live-TV bekommt und ob vor
  Stufe 1 schon auf dem Pixel und dem TV-Gerät gemessen werden muss (V-5).
- **Varianten:**
  - (a) Web zuerst, die Apps gesammelt danach: jede Stufe ist am kleinsten, der Fernseher wartet lange.
  - (b) Web und TV-App in jeder Stufe, die Handy-App gesammelt nach Stufe 5 oder 6: Fernsehen kommt früh auf den Fernseher; jede Stufe wird grösser, V-5 und der TV-Tastentest werden Pflicht.
  - (c) alle drei in jeder Stufe: volle Abdeckung, aber jede Stufe am grössten.
- **Empfehlung:** (b), mit der Lockerung, dass die TV-App-Ausgabe nach dem Server-Release folgen darf, weil Live-TV
  meist auf dem Fernseher geschaut wird und Fehler im Vertrag vor der ersten öffentlichen App noch billig sind.
- **Vorgabe bis zur Antwort:** (b) (der Auftrag legt keine Reihenfolge fest).
- **Fällig vor:** Stufe 0 · **Gewicht:** Weiche

#### E-28-02 · Verbindungslimit des Anbieters · Wortlaut

- **Herkunft:** `04` F-3; `13` P-3; `review-1-architektur` RA-01 und W-10 · **Bezug:** LTV-017; T-17
- **Worum es geht:** Dein Auftrag schliesst jede Verwaltung von Verbindungslimits je Anbieter aus. Slouchcat merkt
  darum erst an der Ablehnung, dass dein Anbieter keine weitere Verbindung erlaubt. Dann bekommt die letzte
  Person «Quelle lehnt weitere Verbindung ab», eine geplante Aufnahme kann scheitern, oder der Anbieter wirft eine
  laufende Verbindung hinaus.
- **Varianten:**
  - (a) Wie beschlossen, keine Zahl: Ablehnungen werden gedeutet, vorsorglicher Empfang ruht nach einer Störung; eine Aufnahme kann am Limit scheitern.
  - (b) Eine einzige Zahl je Quelle «höchstens N gleichzeitige Empfänge» (leer = unbegrenzt), ohne Abos und Laufzeiten: Der Server lehnt sauber ab, bevor etwas stört, und Aufnahmen können Plätze reservieren. Das **ändert LTV-017**.
- **Empfehlung:** `04` empfiehlt (b) in dieser kleinsten Form, weil sie Störungen laufender Wiedergaben verhindert;
  da sie einen Grundsatzentscheid ändert, gilt sie nur auf dein ausdrückliches Wort.
- **Vorgabe bis zur Antwort:** (a), wortlauttreu.
- **Fällig vor:** Stufe 0 (gehört in ADR 0046) · **Gewicht:** Weiche

#### E-28-03 · Härtung der Standard-Compose-Datei für alle

- **Herkunft:** `04` F-12; `review-1-sicherheit` Frage 4 (RS-03); `18` Schritt 0.6 und D-9 · **Bezug:** T-27; `AK-L-281`
- **Worum es geht:** Drei Einstellungen in der Docker-Datei (keine neuen Rechte im laufenden Prozess, alle
  Sonderrechte entzogen, Höchstzahl an Prozessen) machen FFmpeg schwerer angreifbar. Sie ändern keine Funktion,
  aber jede bestehende Installation.
- **Varianten:**
  - (a) In `compose.yaml` für alle: jede Installation ist gehärtet, auch für Filme; erst nach einem Probelauf (V-4).
  - (b) Nur in der Zusatzdatei für Live-TV: nur wer Live-TV einschaltet, bekommt sie.
- **Empfehlung:** (a), weil es nichts kostet und auch die heutige Filmumwandlung schützt.
- **Vorgabe bis zur Antwort:** (a), sobald V-4 grün ist.
- **Fällig vor:** Stufe 0 · **Gewicht:** Weiche

#### E-28-04 · Darf FFmpeg umwandeln, wenn der Kernel keine Sperre erlaubt?

- **Herkunft:** `12` F-1; `18` U-3 und RP-4; `12` R-1 · **Bezug:** LTV-004; T-12, T-27
- **Worum es geht:** Die Umwandlung je Zuschauer liest fremde Daten mit FFmpeg. Die Sperre (Landlock: der Kernel
  legt fest, welche Dateien und Netzziele ein Prozess erreichen darf) verhindert, dass Schadcode in FFmpeg deine
  Zugangsdaten liest. Manche Kernel bieten sie nicht.
- **Varianten:**
  - (a) Ja, mit sichtbarem Zustand «Eindämmung nicht verfügbar» in der Verwaltung und im Protokoll: Live-TV läuft überall, das Risiko bleibt auf solchen Hosts hoch.
  - (b) Nur mit Sperre; ohne Sperre gibt es nur den Direktweg ohne FFmpeg: sicherer, aber viele Sender zeigen im Browser kein Bild.
- **Empfehlung:** (a), und V-4 früh messen; kann CT 190 die Sperre, ist sie dort ohnehin an.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 0 (ADR 0046) · **Gewicht:** Weiche

#### E-28-05 · Weiterleitung von `https` nach `http`

- **Herkunft:** `12` F-2; `04` T-28; `05` 2.4; Rückmeldung von `12` an `04`/`05` Nr. 3 · **Bezug:** LTV-003; T-28
- **Worum es geht:** Leitet ein Anbieter eine verschlüsselte Adresse auf eine unverschlüsselte um, kann jeder
  dazwischen Bild und Ton austauschen, auch für Profile mit Altersgrenze.
- **Varianten:**
  - (a) Immer abbrechen, mit der Meldung «Quelle leitet auf unverschlüsseltes HTTP um»: ein Anbieter, der so umleitet, funktioniert nicht.
  - (b) Nur bei Quellen mit Zugangsdaten abbrechen: mehr Quellen laufen, Quellen ohne Zugangsdaten bleiben verfälschbar.
- **Empfehlung:** (a); eingetragene `http`-Quellen bleiben erlaubt, verboten ist nur der stille Wechsel.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 0 · **Gewicht:** Weiche

#### E-28-06 · Darf der Administrator die Zertifikatsprüfung abschalten?

- **Herkunft:** `12` F-3 · **Bezug:** LTV-003 (HTTPS); T-28
- **Worum es geht:** Manche Geräte im Heimnetz und manche Anbieter haben ein selbst ausgestelltes Zertifikat
  (der Ausweis, mit dem ein Server seine Echtheit zeigt). Ohne Prüfung kann sich jemand dazwischen schalten.
- **Varianten:**
  - (a) Nie: solche Quellen gehen nur über `http` oder mit gültigem Zertifikat.
  - (b) Je Quelle, nur für feste Ziele im Heimnetz, mit Warnung und Protokolleintrag (neue benannte Ausnahme).
  - (c) Je Quelle für alle Quellen, mit Warnung: Zugangsdaten zu öffentlichen Anbietern werden abhörbar.
- **Empfehlung:** (a); (b) nur, wenn V-1 zeigt, dass du so eine Quelle wirklich hast.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 0 · **Gewicht:** Weiche

#### E-28-07 · Aufnahmen und Altersgrenze: Zeitpunkt und Definition

- **Herkunft:** `04` F-10; `07` F-1; `07` F-3; `review-1-sicherheit` Frage 1 (RS-09); `review-2` RD-05 und RD-18;
  `18` Schritt 0.3 · **Bezug:** LTV-009, LTV-015; E-3; Ausnahme A-6
- **Worum es geht:** Nach deiner Antwort E-3 sieht das Hauptprofil die Aufnahmen seiner Profile mit Altersgrenze.
  Offen sind zwei Dinge: was gilt, wenn das Hauptprofil eine Grenze erst später setzt oder wieder entfernt (zum
  Beispiel, wenn ein Kind volljährig wird), und ob die Altersgrenze des **Kontos** mitzählt.
- **Varianten Teil A (Zeitpunkt):**
  - (a) Es zählt nur die Grenze von heute: Setzt das Hauptprofil nachträglich eine Grenze, sieht es auch alle älteren Aufnahmen dieses Profils (benannte Ausnahme).
  - (b) Beides verlangen: Die Aufnahme entstand mit Grenze **und** das Profil hat heute noch eine; wer die Grenze verliert, wird für alle Aufnahmen privat.
  - (c) Nur der Zustand beim Anlegen: Was mit Grenze aufgenommen wurde, bleibt für das Hauptprofil sichtbar, auch nach der Volljährigkeit.
- **Varianten Teil B (Definition «Profil mit Altersgrenze»):**
  - (a) Die wirksame Grenze aus Profil und Konto zählt (kleinerer Wert unter 18): In einem Konto mit Altersgrenze gelten alle Profile als begrenzt, wie heute beim Wunschrecht (ADR 0045).
  - (b) Nur die eigene Grenze des Profils zählt: Ein Konto mit Altersgrenze ändert nichts an der Privatsphäre seiner erwachsenen Profile.
- **Empfehlung:** Teil A (b) und Teil B (a), weil so «Profile ohne Altersgrenze bleiben ganz privat» ohne Lücke gilt
  und eine einzige Definition für Werkseinstellung, Rechte und Aufnahmen reicht.
- **Vorgabe bis zur Antwort:** Teil A (b), Teil B (a) (so in `07` 4.3 und 4.4).
- **Fällig vor:** Stufe 0 (Wortlaut der Ausnahme A-6 in ADR 0046) · **Gewicht:** Weiche

#### E-28-08 · Eigenes Release für Stufe 0

- **Herkunft:** `18` O-4 und 3.4 · **Bezug:** LTV-021
- **Worum es geht:** Stufe 0 bringt Schwärzung in den Protokollen, Lint-Regeln und die Compose-Härtung, aber keine
  sichtbare Funktion.
- **Varianten:**
  - (a) Kein eigenes Release: Stufe 0 reist mit dem Release der Stufe 1.
  - (b) Ein Patch-Release, damit die Härtung früher bei allen Installationen ankommt.
- **Empfehlung:** (a), weil ein Release ohne sichtbaren Nutzen nur Arbeit macht.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 0 · **Gewicht:** Feinschliff

### A.2 Vor Stufe 1 (1a)

#### E-28-09 · Werkseinstellung der Rechte

- **Herkunft:** `03` F-1; `07` F-2; `14` F-1; `15` LA-22 (`07` W-24); `review-2` RD-05 · **Bezug:** LTV-012; `03` DR-01
- **Worum es geht:** Profile mit Altersgrenze haben Live-TV ab Werk aus (DR-01). Offen ist, was nach der Installation
  oder beim Update für alle anderen gilt und welche Konten bei der Einführung Live-TV sofort sehen.
- **Varianten:**
  - (a) Konten ab Werk aus, Profile ohne Altersgrenze an (sie wirken, sobald das Konto an ist); bei der Einführung sind nur Administrator-Konten an: ein Klick je Konto, Gäste kommen nicht ungefragt an deine Anbieter.
  - (b) Alles an ausser Profilen mit Altersgrenze: jedes Konto sieht Live-TV sofort und belastet sofort Leitung und Anbieter.
  - (c) Alles aus, auch Profile ohne Altersgrenze: jedes Profil braucht eine Freigabe, Arbeit ohne Gewinn an Schutz.
  - Zusatz «Administrator-Konto»: (i) alle Konten mit der Rolle Administrator; (ii) nur der Besitzer.
- **Empfehlung:** (a) mit (i), für «Timeshift» und «Aufnahmen» dieselbe Werkseinstellung, weil du für die
  Verbindungen deiner Anbieter verantwortlich bist (LTV-017) und jeder Administrator Live-TV prüfen will.
- **Vorgabe bis zur Antwort:** (a) mit (i) (`07` 3.11).
- **Fällig vor:** Stufe 1a (die Migration setzt die Werte für den Bestand) · **Gewicht:** Weiche

#### E-28-10 · Speicherort des Startpuffers · Wortlaut

- **Herkunft:** `04` F-2; `18` D-3; `03` LTV-019.01 und .04; `review-1-architektur` Abschnitt «Abweichungen» A-2 ·
  **Bezug:** LTV-019 Punkt 1; T-7
- **Worum es geht:** Dein Auftrag verlangt einen eigenen, einstellbaren Speicherort auch für den kurzen technischen
  Startpuffer (die letzten Sekunden eines empfangenen Senders, damit das Bild beim Umschalten sofort steht). Auf
  einer Platte bringt das keinen Nutzen und viel Schreiblast: bei 20 dauerhaft empfangenen Sendern rund 1,7 TB je
  Tag auf einer SSD.
- **Varianten:**
  - (a) Wählbar, wie im Auftrag: Arbeitsspeicher (ab Werk), RAM-Disk oder Ordner; der Ring braucht zwei Bauformen, die Verwaltung warnt vor Platten; die Ortswahl kommt in Stufe 4.
  - (b) Nur Arbeitsspeicher, einstellbar ist nur die Grösse: eine Bauform, kein Fehlgriff möglich, aber eine **Abweichung** von LTV-019.
- **Empfehlung:** (b), weil der Puffer im Arbeitsspeicher am schnellsten ist und die Wahl des Orts bei Timeshift und
  Aufnahmen bleibt, wo sie etwas bewirkt.
- **Vorgabe bis zur Antwort:** (a), wortlauttreu.
- **Fällig vor:** Stufe 1a · **Gewicht:** Weiche

#### E-28-11 · Zugangsdaten in der «Sicherung ohne Geheimnisse»

- **Herkunft:** `12` F-4; `review-2` RD-08; `04` 10.5; `07` 6.3 · **Bezug:** LTV-017 (Zugangsdaten sicher speichern); T-28
- **Worum es geht:** Die bestehende Sicherung ohne Geheimnisse löscht verschlüsselte Einstellungen. Offen ist, ob
  das auch für die Zugänge deiner Fernsehquellen gilt.
- **Varianten:**
  - (a) Leeren (Platzhalter statt Wert): Die Sicherung enthält keine Zugangsdaten; nach einer Wiederherstellung gibst du die Zugänge neu ein.
  - (b) Verschlüsselt behalten: Auf demselben Server laufen die Quellen sofort weiter; wer Sicherung und später den Schlüssel bekommt, liest sie.
- **Empfehlung:** (a), weil Sicherungen oft an weniger geschützten Orten liegen als der Server selbst.
- **Vorgabe bis zur Antwort:** (a) (`04` 10.5 und `07` 6.3 sind danach anzugleichen, Teil E).
- **Fällig vor:** Stufe 1a · **Gewicht:** Weiche

#### E-28-12 · Safari, iPhone und iPad ohne Testgerät · Wortlaut

- **Herkunft:** `17` O-11; `04` T-11 und 12.1; `05` M-5 und O-28; `18` V-3, D-7, RP-14; `16` Abschnitt 10 Nr. 6, 9,
  17, 20; `17` G-4 · **Bezug:** LTV-010 («Verhalten unterschiedlicher Browser und nativer Clients»), LTV-014;
  Entscheid 2 (`docs/planning/04-entscheidungen.md`)
- **Worum es geht:** Im Haushalt gibt es kein Apple-Gerät zum Prüfen. Apples Vorgaben verlangen längere
  Wiedergabelisten, als Slouchcat für schnelles Umschalten liefert; ob Live-TV dort gut läuft, kann niemand sagen.
  Bis gemessen ist, gelten diese Geräte als ungeprüft und nicht zugesagt.
- **Varianten:**
  - (a) Bestätigen: ungeprüft und nicht zugesagt, Abnahme ohne sie, Hinweis im Release-Text.
  - (b) Du besorgst oder leihst vor Stufe 1a ein iPhone oder einen Mac: V-3 läuft, danach folgen vielleicht Anpassungen (längerer Anlauf, bis zu sechs Segmente, `05` O-28).
  - (c) Apple-Geräte ausdrücklich ausschliessen: klare Ansage, aber ein Abstrich an LTV-010.
- **Empfehlung:** (a); (b), sobald Apple-Geräte im Haushalt Live-TV nutzen sollen.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 1a (Abnahme) · **Gewicht:** Weiche

#### E-28-13 · Schwellen des Benchmarks; Zielwerte hängen an der Quelle · Wortlaut

- **Herkunft:** `17` O-8, G-8 und H-7; `05` 8.5; `04` 14.11; `13` 9; `18` U-10 · **Bezug:** LTV-010 (Zielwerte); `AK-L-082` bis `-085`
- **Worum es geht:** Dein Auftrag nennt Umschaltzeiten (unter 1, 3 und 5 Sekunden) als zu prüfende Ziele. Ob sie
  erreichbar sind, hängt oft an der Quelle: Sendet ein Anbieter nur alle vier Sekunden ein vollständiges Bild, ist
  «noch nicht empfangener Sender unter 3 Sekunden» ohne Vorladen nicht erreichbar (`05` 7). Für die Abnahme
  braucht es feste Schwellen.
- **Varianten:**
  - (a) Median unter dem Zielwert, p95 (der Wert, unter dem 95 von 100 Messungen liegen) höchstens doppelt so hoch, Fehlerquote höchstens 2 %; wird ein Ziel verfehlt, nennt das Protokoll den Grund, und du entscheidest über die Abnahme.
  - (b) Strenger: auch p95 unter dem Zielwert.
  - (c) Nur messen und berichten, keine Schwelle.
- **Empfehlung:** (a), weil sie Ausreisser zulässt, aber eine schlechte Mitte nicht.
- **Vorgabe bis zur Antwort:** (a) als Vorschlag; die Zielwerte bleiben Messgrössen, nie Zusagen.
- **Fällig vor:** Stufe 1a (Abnahme) · **Gewicht:** Weiche

#### E-28-14 · Live-TV gegen Filme

- **Herkunft:** `13` P-7 und P-8; `17` O-3; `04` 7.6, 15.2 und T-20; `18` U-9 und RP-9; `13` M-6 · **Bezug:** LTV-004;
  Auftrag 11.3; `AK-L-033`
- **Worum es geht:** Live-Umwandlungen und Filmwiedergabe teilen sich Grafikchip und Prozessor. Offen ist, ob Live
  ungenutzte Einheiten der Filme borgen darf, ob Empfang und Aufnahme auf Hardware ohne Grafikchip vor einer
  Filmumwandlung Vorrang haben und ab wann ein verzögerter Filmstart «unzulässig» ist.
- **Varianten:**
  - (a) Kein Borgen; Empfang und Aufnahme laufen ohne Zurückstufung; Grenze: Filmstart höchstens doppelt so lang wie ohne Live, Filmumwandlung mindestens in Echtzeit.
  - (b) Borgen erlaubt, solange Filme nichts brauchen: mehr Live-Zuschauer, ein Filmstart kann kurz warten.
  - (c) Filme gehen immer vor: neue Live-Umwandlungen werden abgelehnt, solange ein Film umwandelt.
- **Empfehlung:** (a), weil sie Filme schützt und Live-TV trotzdem planbar bleibt.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 1a · **Gewicht:** Weiche

#### E-28-15 · Verwaltungsbereich

- **Herkunft:** `15` F-13 · **Bezug:** Auftrag Abschnitt 9
- **Worum es geht:** Live-TV braucht in der Verwaltung viele Seiten (Quellen, Sender, Programm, Empfang, Speicher …).
- **Varianten:**
  - (A) Eigener Bereich «Live-TV» mit acht Reitern (so das Mockup).
  - (B) Eine Seite «Live-TV» unter «Medien» mit Reitern wie beim Radio: eng für so viele Seiten.
  - (C) Eigener Bereich, aber erst sichtbar, wenn eine Quelle besteht; vorher nur «Live-TV einrichten» in der Übersicht.
- **Empfehlung:** (C), weil Server ohne Live-TV keinen leeren Bereich zeigen.
- **Vorgabe bis zur Antwort:** (C).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-16 · Eintrag in der Web-Navigation

- **Herkunft:** `14` F-12 · **Bezug:** LTV-014; `03` DR-23
- **Worum es geht:** Live-TV bekommt einen Eintrag in der Seitenleiste; offen ist nur, wo.
- **Varianten:**
  - (A) Ein Eintrag im Band «Meine Medien» wie Webradio (so das Mockup).
  - (B) Zusätzlich ein Eintrag oben neben Start und Suche.
- **Empfehlung:** (A), eine Stelle, die der Administrator ordnet und versteckt.
- **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-17 · Sortierung der Sender

- **Herkunft:** `15` F-15 · **Bezug:** LTV-006
- **Worum es geht:** Reihenfolge der Sender in Listen, Zifferneingabe und beim Zappen.
- **Varianten:**
  - (A) Nach Kanalnummer, Sender ohne Nummer alphabetisch danach.
  - (B) Zusätzlich eine eigene Reihenfolge je Gruppe.
- **Empfehlung:** (A), eine einzige Wahrheit für Zifferneingabe und Zappen.
- **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-18 · Kategorien und Genres

- **Herkunft:** `15` F-16; `09` O-8 · **Bezug:** LTV-005 (Kategorien, Genres), LTV-006
- **Worum es geht:** Auslegung: «Kategorien» sind Kategorien der Sender, «Genres» kommen aus den Programmdaten
  (`category`) so, wie der Anbieter sie liefert; eine eigene Genre-Liste gibt es nicht.
- **Varianten:**
  - (a) Feste, übersetzte Liste von Senderkategorien (Vollprogramm, Nachrichten, Sport …), Genres wie geliefert.
  - (b) Senderkategorien frei vom Administrator anlegbar, Genres wie geliefert.
- **Empfehlung:** (a), übersetzbar und in allen Clients gleich; frei benennbar sind schon die Gruppen.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-19 · CPU-, RAM- und GPU-Werte in der Verwaltung

- **Herkunft:** `15` F-22; `17` G-10 · **Bezug:** Auftrag 9.1
- **Worum es geht:** Dein Auftrag verlangt diese Werte auf der Übersicht; der Server liest sie heute nicht.
- **Varianten:**
  - (A) Anzeigen, wo das System sie hergibt, sonst «–».
  - (B) Weglassen und auf «Server» verweisen: eine Abweichung von Auftrag 9.1.
- **Empfehlung:** (A), wortlauttreu; ob die GPU-Last überall lesbar ist, wird gemessen.
- **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-20 · Geräteprüfungen und Kamera-Gegenprobe

- **Herkunft:** `17` O-10 und O-13 · **Bezug:** LTV-010 (Benchmark), `AK-L-296`, `-297`
- **Worum es geht:** Prüfungen am Pixel und am TV-Gerät laufen nicht automatisch. Für die Umschaltzeit braucht es
  zudem einmal eine Gegenprobe mit einer Kamera (mindestens 120 Bilder je Sekunde), damit die Messhaken im Client
  stimmen.
- **Varianten:**
  - (a) Du führst die Geräteprüfungen nach einer Vorlage mit Protokoll durch; die Gegenprobe filmt einmal je Client ein Handy in Zeitlupe.
  - (b) Nur die Messhaken im Client, ohne Kamera-Gegenprobe: weniger Aufwand, aber die Messung ist nicht unabhängig belegt.
- **Empfehlung:** (a), einmal zur Abnahme von Stufe 1a; danach reichen die Messhaken.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 1a (Abnahme) · **Gewicht:** Feinschliff

#### E-28-21 · Blender-Filme im Benchmark

- **Herkunft:** `17` O-2 und H-9 · **Bezug:** Auftrag 15.2 («selbst erzeugt», «rechtmässig verwendbar»)
- **Worum es geht:** Eine Recherche schlug frei lizenzierte Filme als Testquelle vor; das Konzept nutzt selbst
  erzeugtes Testbild.
- **Varianten:**
  - (a) Nein, Testbild reicht.
  - (b) Ja, nur lokal und nach Lizenzprüfung, wenn die Streuung der Bitrate gemessen werden soll.
- **Empfehlung:** (a).
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

### A.3 Vor Stufe 2

#### E-28-22 · Kanäle für Erinnerungen und Aufnahme-Meldungen

- **Herkunft:** `03` F-3; `09` F-3; `14` F-6; `07` W-21; `03` DR-05 · **Bezug:** LTV-005 (Erinnerungen), LTV-009; `02` AB-15
- **Worum es geht:** Benachrichtigungskanäle (E-Mail, Web-Push, Telegram, Push an die Handy-App) sind heute je Konto
  eingestellt, nicht je Profil. Geht die Erinnerung eines Profils auch dorthin, sehen andere Profile mit derselben
  E-Mail, dass eine kam.
- **Varianten:**
  - (a) Glocke des Profils und die Kanäle des Kontos mit dem Profilnamen davor; je Ereignisart ein Schalter; PIN-Profile ohne Titel.
  - (b) Nur die Glocke des Profils: ganz privat, aber niemand wird erreicht, der die App nicht offen hat.
  - (c) Eigene Kanaleinstellungen je Profil: ändert das Benachrichtigungsmodell für alle Meldungen und geht über den Auftrag hinaus.
- **Empfehlung:** (a), weil sie das Bestehende nutzt; die Pünktlichkeit von Push wird gemessen (`09` M-8).
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 2 · **Gewicht:** Weiche

#### E-28-23 · Programmsuche in der allgemeinen Suche

- **Herkunft:** `03` F-5; `14` F-3; `02` AB-14 und A-2 · **Bezug:** LTV-005 (senderübergreifende Programmsuche), LTV-015
- **Worum es geht:** Die Programmsuche kommt sicher in den Live-TV-Bereich. Die allgemeine Suche ist für Titel
  altersgefiltert; Sendungen sind es nicht (LTV-015).
- **Varianten:**
  - (a) Nur im Live-TV-Bereich: der Jugendschutz der allgemeinen Suche bleibt unberührt.
  - (b) Zusätzlich ein Abschnitt «Im Fernsehen» in der allgemeinen Suche, nur bei wirksamem Recht: bequemer, mischt aber ungefilterte Sendungen in eine gefilterte Liste.
- **Empfehlung:** (a).
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 2 · **Gewicht:** Weiche

#### E-28-24 · Programmdaten in eigener Datei, nicht in der Sicherung

- **Herkunft:** `09` F-1; `04` O-8; `09` O-12; `18` RP-8 und U-13 · **Bezug:** LTV-005; Auftrag 11.2 (Datenbanklast)
- **Worum es geht:** Programmdaten können ein Vielfaches der heutigen Datenbank ausmachen (5'000 Sender × 7 Tage
  ≈ 600 MB). Heute wird die eine Datenbankdatei vor jedem Update gesichert und in jede Sicherung mitgenommen.
- **Varianten:**
  - (a) Eigene Datei, nicht gesichert: Sicherungen und Updates bleiben klein, der Import bremst den Rest nicht; nach einer Wiederherstellung fehlen die Programmdaten einige Minuten, bis die Quellen neu geladen sind.
  - (b) Alles in einer Datei: einfacher, aber grosse Sicherungen, langsamere Updates, und der Import teilt sich die Schreibverbindung mit der Wiedergabe.
- **Empfehlung:** (a). Quellen, Zuordnungen, Erinnerungen und Planungen sind in beiden Varianten gesichert.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 2 · **Gewicht:** Weiche

#### E-28-25 · EPG-Adressen im Heimnetz

- **Herkunft:** `09` F-4; `12` O-9; `review-2` RD-30 (S-12) · **Bezug:** LTV-005; Ausnahme A-3
- **Worum es geht:** Wer Programmdaten selbst erzeugt (zum Beispiel auf dem NAS), stellt sie oft im Heimnetz bereit.
  Die Ausnahme für das Heimnetz gilt bisher nur für Senderquellen.
- **Varianten:**
  - (a) Ja, mit festen Zielen je EPG-Quelle wie bei Senderquellen (Adresse und Port bestätigt, nie Loopback oder der Server selbst), gebaut in Stufe H.
  - (b) Nein: selbst erzeugte Programmdaten nur per Upload oder über eine öffentliche Adresse.
- **Empfehlung:** (a).
- **Vorgabe bis zur Antwort:** (b) (die Spalte dafür entsteht erst mit deiner Antwort, `review-2` S-12).
- **Fällig vor:** Stufe 2 (Schema der EPG-Quellen); gebaut in Stufe H · **Gewicht:** Weiche

#### E-28-26 · Namensgleiche Sender automatisch zuordnen

- **Herkunft:** `09` F-2 · **Bezug:** LTV-005 (automatische Senderzuordnung)
- **Worum es geht:** Die automatische Zuordnung über die Kennung `tvg-id` gibt es in jedem Fall. Fehlt sie, bleibt der
  Name; ein eindeutiger Name kann trotzdem falsch sein (Regionalfenster, gleicher Sender in zwei Ländern).
- **Varianten:**
  - (a) Nur Vorschlag mit Sammelbestätigung: keine stillen Fehltreffer, ein Klick für alle eindeutigen Vorschläge.
  - (b) Automatisch, markiert «automatisch (Name)»: weniger Arbeit, falsche Programme fallen erst beim Zuschauen auf.
- **Empfehlung:** (a).
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 2 · **Gewicht:** Feinschliff

#### E-28-27 · Sendungsbilder im Raster

- **Herkunft:** `14` F-11 · **Bezug:** LTV-014 (Programmzeitschrift)
- **Varianten:** (A) Im Raster nie Bilder, nur Text (ruhig, schnell); (B) kleine Bilder in breiten Zellen.
- **Empfehlung:** (A); Bilder in der Vorschau, auf Karten und auf der Detailseite.
- **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 2 · **Gewicht:** Feinschliff

#### E-28-28 · Altersangaben der Programmdaten

- **Herkunft:** `14` F-8 · **Bezug:** LTV-015
- **Worum es geht:** Manche Programmdaten enthalten Altersangaben. Slouchcat prüft und filtert sie nicht (LTV-015).
- **Varianten:** (A) Nie anzeigen, weil die Anzeige Schutz vortäuschen könnte; (B) als reine Information auf der
  Detailseite mit dem Satz «Angabe des Senders, Slouchcat prüft sie nicht».
- **Empfehlung:** (A).
- **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 2 · **Gewicht:** Feinschliff

### A.4 Vor Stufe 3

#### E-28-29 · Vorladen ab Werk und Schalter je Quelle · Wortlaut (Grenzfall LTV-017)

- **Herkunft:** `04` F-9; `06` Abschnitt 6; `13` P-5; `review-3` RM-01; `19` Abschnitt 6; `15` LA-03 Schritt 5,
  LA-09, Texte V-4 und V-7; Festlegung des Koordinators zu RM-01 · **Bezug:** LTV-002, LTV-010, LTV-017; T-18
- **Worum es geht:** Vorladen (Nachbarsender vorsorglich empfangen) und «automatisch» (häufig genutzte Sender
  bereithalten) sind verbindlich und werden gebaut. Offen ist, was nach der Installation eingeschaltet ist; jeder
  vorsorgliche Empfang belegt eine Verbindung zu deinem Anbieter. Der Entwurf hat dafür einen Schalter je Quelle.
  Review 3 nennt ihn einen Grenzfall zu LTV-017; der Koordinator hat festgelegt, dass er eine Einstellung des
  Vorladens ist, keine Verbindungsverwaltung, und «Vorladen für Sender dieser Quelle: an/aus» heisst.
- **Varianten:**
  - (a) Alles aus, bis der Administrator es einschaltet: sicher bei knappen Zugängen; schnelles Umschalten erst nach bewusstem Einschalten.
  - (b) Global an (1 Sender davor, 1 danach; «automatisch» mit 3 Sendern), wirksam nur für Quellen mit Schalter an; der Schalter ist ab Werk aus und wird beim Anlegen einer Quelle abgefragt.
  - (c) Alles an: schnellstes Umschalten ohne Zutun, bei Zugängen mit nur einer Verbindung aber Störungen.
  - (d) Kein Schalter je Quelle, nur global ein oder aus: nichts, was einer Anbieterverwaltung ähnelt; bei gemischten Quellen gilt alles oder nichts.
- **Empfehlung:** (b), eine Frage beim Anlegen der Quelle, danach arbeitet alles von selbst.
- **Vorgabe bis zur Antwort:** (b) mit der Bezeichnung des Koordinators.
- **Fällig vor:** Stufe 3 · **Gewicht:** Weiche

#### E-28-30 · Neuer Zuschauer gegen Empfang ohne Abnehmer · Wortlaut

- **Herkunft:** `04` F-4; `13` P-1; `06` 2.1 und 3.14 · **Bezug:** LTV-002 (permanenter Empfang «unabhängig davon, ob
  jemand zuschaut»), Auftrag 11.3
- **Worum es geht:** Die Obergrenze der Eingangsströme ist erreicht, weil dauerhaft oder nach Zeitplan empfangene
  Sender laufen, die gerade niemand schaut. Jemand will einen anderen Sender sehen.
- **Varianten:**
  - (a) Der Zuschauer geht vor: Ein Sender ohne Abnehmer wird «zurückgestellt» und läuft wieder an, sobald Platz ist. Das unterbricht den permanenten Empfang kurz.
  - (b) Dauerhaft bleibt dauerhaft: Der Zuschauer bekommt «Server ausgelastet».
- **Empfehlung:** (a), weil ein Mensch vor dem Bildschirm wichtiger ist als ein Puffer, den niemand nutzt.
- **Vorgabe bis zur Antwort:** (b), wortlauttreu zu LTV-002 (die Abläufe in `06` zeichnen schon (a)).
- **Fällig vor:** Stufe 3 · **Gewicht:** Weiche

#### E-28-31 · Erkennung eingefrorener Bilder · Wortlaut

- **Herkunft:** `04` F-14; `18` D-5 und U-14; `17` G-1 und H-14; Rückmeldung von `17` · **Bezug:** LTV-011 (eingefrorene
  Streams), LTV-011.04
- **Worum es geht:** Ein Strom kann auf zwei Arten einfrieren. Steht der Transport, erkennt der Server das am Strom.
  Sendet die Quelle aber weiter gültige Daten mit immer demselben Bild (oder Schwarz, oder Stille), ist das ohne
  Dekodieren nicht sicher erkennbar; dafür braucht es eine Stichprobe mit FFmpeg.
- **Varianten:**
  - (a) Zwei Schritte: Stufe 3 erkennt den Stillstand des Transports und zeigt beim Standbild nur einen Verdacht; die Stichprobe kommt in Stufe E nach Messung M-20. Bis dahin ist LTV-011 nur teilweise erfüllt.
  - (b) Stichprobe schon in Stufe 3: LTV-011 ist mit Stufe 3 erfüllt, die Stufe wird grösser, jede Stichprobe kostet Rechenzeit.
  - (c) Nur der Stillstand des Transports, dauerhaft: ein Abstrich an LTV-011.
- **Empfehlung:** (a), weil Filter und Kosten zuerst gemessen werden sollen.
- **Vorgabe bis zur Antwort:** (a) (die Anforderung fällt nicht weg, sie wird später fertig).
- **Fällig vor:** Stufe 3 · **Gewicht:** Weiche

#### E-28-32 · Welche Ausfälle der Administrator gemeldet bekommt

- **Herkunft:** `15` F-19; `04` 10.2 · **Bezug:** LTV-011 (Benachrichtigung bei längeren Ausfällen, keine Meldungsflut)
- **Varianten:**
  - (A) Nur dauerhaft oder nach Zeitplan empfangene Sender und Sender einer laufenden oder in 30 Minuten beginnenden Aufnahme, nach 10 Minuten Störung.
  - (B) Zusätzlich Sender, die gerade jemand schaut.
  - (C) Jeder aktive Sender, der bei einer Prüfung ausfällt.
- **Empfehlung:** (A), keine Flut bei grossen Listen; Zuschauer sehen den Ausfall ohnehin.
- **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 3 · **Gewicht:** Feinschliff

#### E-28-33 · Angabe der Anschlussbandbreite

- **Herkunft:** `13` O-4 · **Bezug:** Auftrag 11.3 (Bandbreitenmangel)
- **Worum es geht:** Slouchcat kennt deinen Internetanschluss nicht und merkt Sättigung nur indirekt.
- **Varianten:**
  - (a) Keine Angabe; Schutz nur über die Obergrenze der Eingangsströme.
  - (b) Neue Einstellung «Anschluss hat X Mbit/s»: Vorladen und «automatisch» werden vor der Sättigung abgebaut.
- **Empfehlung:** (a) für jetzt; nach V-1 und M-17 neu prüfen, ob (b) nötig ist.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 3 · **Gewicht:** Feinschliff

### A.5 Vor Stufe H

#### E-28-34 · Multicast: Netzmodus «host» oder eigenes Netz · Wortlaut

- **Herkunft:** `04` F-11; `review-1-sicherheit` Frage 3 (RS-13); `02` A-9; `18` U-8 und V-6; `12` R-5 ·
  **Bezug:** LTV-003 (UDP, RTP, Multicast)
- **Worum es geht:** Multicast (ein Strom an viele Empfänger im Heimnetz) kommt im normalen Netz des Containers
  vermutlich nicht an (V-6 misst es). Es gibt zwei Auswege, und vielleicht geht auf deinem Aufbau keiner.
- **Varianten:**
  - (a) Netzmodus «host» als benannte Ausnahme: einfach, aber jeder Dienst auf dem Host liegt hinter jeder Lücke der Abwehr.
  - (b) Eigenes Netz des Containers (`macvlan` oder `ipvlan`, eine eigene Adresse im Heimnetz): kleinerer Schaden, mehr Einrichtung; ob es im Proxmox-Container geht, misst V-6.
  - Geht keins: LTV-003.16 ist auf deinem Aufbau nicht erfüllbar, und du entscheidest dann, ob Stufe H ohne Multicast kommt (`18` U-8).
- **Empfehlung:** (b), wenn V-6 es bestätigt, sonst (a).
- **Vorgabe bis zur Antwort:** (b), sonst (a), nach V-6.
- **Fällig vor:** Stufe H · **Gewicht:** Weiche

### A.6 Vor Stufe 4

#### E-28-35 · Rückspulgrenze

- **Herkunft:** `04` F-7; `06` 3.2 und 3.6; `review-1-architektur` RA-03 · **Bezug:** LTV-007 («nicht vor dem
  Einschalten»); T-7
- **Worum es geht:** Damit das Bild sofort steht, beginnt jede Wiedergabe mit einem kurzen Anlauf aus dem
  Startpuffer, also mit Bildern von bis zu zwei Bildgruppen vor dem Tastendruck (meist bis 4 Sekunden, höchstens
  15 Sekunden).
- **Varianten:**
  - (a) Grenze am Tastendruck: Der Anlauf wird einmal gezeigt und ist danach nicht mehr abrufbar; die Zeitleiste beginnt beim Tastendruck.
  - (b) Grenze am ersten gezeigten Bild: einfacher, aber bis zu 15 Sekunden vor dem Tastendruck sind rückspulbar.
- **Empfehlung:** (a), wortlauttreu.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 4 · **Gewicht:** Weiche

#### E-28-36 · Speicherform des Timeshift-Puffers

- **Herkunft:** `04` F-8; `13` P-4; `06` 3.3; `review-1-architektur` RA-09; `18` U-5; `24` N-19 · **Bezug:** LTV-007,
  Ausschluss «gemeinsame Timeshift-Puffer»; T-10
- **Worum es geht:** Schauen 20 Personen mit Timeshift denselben Sender, schreibt die eigene Kopie je Sitzung
  zwanzigmal dasselbe (72 GB je Stunde).
- **Varianten:**
  - (a) Eigene Kopie je Sitzung: einfach und robust, für einen Haushalt mit bis zu fünf Zuschauern tragbar.
  - (b) Einmal geschriebene Stücke je Sender, je Sitzung eine Liste der eigenen Stücke: viel weniger Schreiblast, aber in der Speicherform ein gemeinsamer Vorrat; ob der Ausschluss das meint, kannst nur du sagen.
  - (c) Eigene Kopie und eine Obergrenze gleichzeitiger Sitzungen mit Timeshift: wer darüber liegt, schaut ohne Zeitversatz.
- **Empfehlung:** (a); (c) als Bremse, falls V-7 es verlangt; (b) nur, wenn du die Speicherform ausdrücklich zulässt.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 4 · **Gewicht:** Weiche

#### E-28-37 · Sofortaufnahme im Zeitversatz

- **Herkunft:** `10` F-1 · **Bezug:** LTV-008.01; T-16
- **Worum es geht:** Wer 20 Minuten zurückliegt und «Aufnehmen» drückt, sieht eine Stelle, die schon vorbei ist.
- **Varianten:**
  - (a) Ab jetzt (live): Der Knopf heisst «Ab jetzt aufnehmen (live)».
  - (b) Ab der eigenen Stelle: Slouchcat kopiert den eigenen Puffer ab der Abspielstelle in die Aufnahme; bequemer, aber ein zweiter Weg, eine Aufnahme zu füllen.
  - (c) Mit dem ganzen eigenen Puffer ab dem Einschalten.
- **Empfehlung:** (a), weil (b) und (c) eigenen Code für einen seltenen Fall brauchen.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 4 · **Gewicht:** Feinschliff

#### E-28-38 · Rückfrage beim Senderwechsel aus dem Zeitversatz

- **Herkunft:** `14` F-9 · **Bezug:** LTV-007 (Senderwechsel löscht den Puffer)
- **Varianten:** (A) Immer fragen; (B) erst ab 60 Sekunden Abstand zu Live oder bei Pause, mit «Nicht mehr fragen»;
  (C) nie fragen, nur danach die Meldung «Zeitversatz verworfen».
- **Empfehlung:** (B).
- **Vorgabe bis zur Antwort:** (B).
- **Fällig vor:** Stufe 4 · **Gewicht:** Feinschliff

### A.7 Vor Stufe 5

#### E-28-39 · Vorgabe des Aufnahme-Kontingents

- **Herkunft:** `03` F-2; `15` F-20; `13` H-6 · **Bezug:** LTV-008.12, LTV-019; E-4
- **Worum es geht:** Jedes Konto bekommt ab Werk einen Speicherplatz für Aufnahmen; der Administrator ändert ihn je
  Konto. Eine Stunde HD braucht bei 8 Mbit/s rund 3,6 GB (je nach Sender 2 bis 4 GB, zu messen).
- **Varianten:**
  - (a) 100 GB je Konto (wie das Download-Budget), also rund 28 Stunden HD.
  - (b) 0 GB: keine Aufnahmen, bis der Administrator ein Kontingent setzt; das Recht «Aufnahmen» wirkt bis dahin nicht.
  - (c) Unbegrenzt, nur die Speicherreserve begrenzt: ein Konto kann den ganzen Speicherort füllen.
- **Empfehlung:** (a); die Zahl nach der ersten Messung überprüfen.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-40 · Beginnende Aufnahme gegen laufende Wiedergaben

- **Herkunft:** `03` F-4; `04` F-5; `13` P-2; `06` 3.8, 3.14, N-29 · **Bezug:** LTV-002 (Schutz aktiver Wiedergaben und
  laufender Aufnahmen), Auftrag 11.3
- **Worum es geht:** Eine geplante Aufnahme beginnt, aber Zuschauer auf anderen Sendern belegen alle Plätze. Laufende
  Wiedergaben und Aufnahmen sollen geschützt sein; einer von beiden muss zurückstehen.
- **Varianten:**
  - (a) Die Aufnahme reserviert ab 2 Minuten vor Beginn einen Platz; wer dann einen weiteren Sender einstellt, wird abgelehnt; eine laufende Wiedergabe wird nie beendet; reicht es trotzdem nicht, scheitert die Aufnahme mit Meldung.
  - (b) Die Aufnahme verdrängt im Notfall die jüngste Wiedergabe auf einem anderen Sender, mit Hinweis an den Zuschauer.
  - (c) Wer zuerst kommt, mahlt zuerst, auch gegenüber Aufnahmen.
- **Empfehlung:** (a), mit Warnung im Planer, wenn eine Grenze zur geplanten Zeit knapp wird.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-41 · Entzug des Rechts: Planungen und laufende Aufnahmen

- **Herkunft:** `03` F-7; `04` O-9; `14` F-10; `06` 3.15; `17` G-2 · **Bezug:** LTV-012 (Entzug serverseitig), LTV-018.13
- **Worum es geht:** Wird «Aufnahmen» oder «Live-TV» entzogen, sind vorhandene Aufnahmen in jeder Variante
  unsichtbar und bleiben gespeichert. Offen ist, was mit Planungen und einer gerade laufenden Aufnahme geschieht.
- **Varianten:**
  - (a) Planungen ruhen und wirken wieder, wenn das Recht zurückkommt; eine laufende Aufnahme endet sofort und bleibt als «abgebrochen (Recht entzogen)» erhalten.
  - (b) Planungen werden gelöscht; laufende wie (a): endgültig und überraschend, wenn das Recht nur kurz weg war.
  - (c) Wie (a), aber eine laufende Aufnahme läuft bis zum geplanten Ende: widerspricht dem Entzug.
- **Empfehlung:** (a), weil nichts verloren geht, was das Profil eingerichtet hat.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-42 · Hauptprofil löscht, wenn das Konto kein Live-TV mehr hat

- **Herkunft:** `08` F-1; `07` O-7; `08` O-6; `14` F-10 · **Bezug:** LTV-012 (ohne Recht unsichtbar), E-3; `15` F-18
- **Worum es geht:** Schaltet der Administrator Live-TV für ein Konto aus, ist der ganze Bereich unsichtbar; die
  Aufnahmen bleiben und belegen Platz. Der Administrator sieht keine Titel und löscht nichts (E-28-45).
- **Varianten:**
  - (a) Ja: Das Hauptprofil erreicht weiter nur die Liste der Aufnahmen und das Löschen, nicht das Abspielen.
  - (b) Nein: Ohne Live-TV ist alles unsichtbar; Platz wird erst frei, wenn der Administrator das Recht zurückgibt.
- **Empfehlung:** (a), sonst bleiben Aufnahmen, die niemand mehr löschen kann.
- **Vorgabe bis zur Antwort:** (b), wortlauttreu zu «ohne Recht unsichtbar».
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-43 · Was das Hauptprofil ausser den Aufnahmen sieht

- **Herkunft:** `14` F-4; `review-2` RD-18; `19` Abschnitt 6 · **Bezug:** E-3, LTV-009
- **Worum es geht:** Fest steht «sehen, löschen, abspielen» der Aufnahmen von Profilen mit Altersgrenze. Offen sind
  deren geplante Aufnahmen und ob das Profil erfährt, dass das Hauptprofil etwas gelöscht hat.
- **Varianten:**
  - (A) Nur die Aufnahmen.
  - (B) Dazu geplante Termine und Regeln, nur lesend; das Profil bekommt eine Meldung, wenn das Hauptprofil eine seiner Aufnahmen löscht.
  - (C) Geplante Aufnahmen auch abbrechen und ändern.
- **Empfehlung:** (B), mit Meldung an das Profil.
- **Vorgabe bis zur Antwort:** geplante Termine nur als Zahl (wie `08` und das Mockup), sonst (A).
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-44 · Spuren, die eine Quelle ausserhalb des TS liefert · Wortlaut

- **Herkunft:** `11` F-1 · **Bezug:** LTV-020 («alle empfangenen verfügbaren Untertitelspuren»; der Auftrag verlangt
  ausdrücklich eine Lösung zur Entscheidung), LTV-020.21
- **Worum es geht:** Manche HLS-Quellen liefern Untertitel oder eine Tonspur als eigene Liste neben dem
  Hauptstrom, DASH liefert Untertitel oft als eigenes Format. Der Empfänger liest nur den TS-Strom (das
  Containerformat der Aufnahmen); diese Spuren fehlen dann in Aufnahme und Live-Bild.
- **Varianten:**
  - (a) Nicht aufnehmen, ehrlich anzeigen («liefert Untertitel ausserhalb des Stroms: nicht in Aufnahmen»): kein Mehraufwand, Abstrich an LTV-020.
  - (b) Als Begleitdatei aufnehmen (Untertitel als WebVTT-Datei, Tonspur als zweite Datei je Teil), die Wiedergabe führt sie zusammen: verlustfrei, aber ein zweites Format und eigener Code.
  - (c) Später mit Weg W, soweit FFmpeg die Spur verlustfrei nach TS packen kann (für WebVTT nicht).
- **Empfehlung:** (a) für Stufe 5, und (b), sobald V-1 zeigt, dass deine Quellen solche Spuren haben.
- **Vorgabe bis zur Antwort:** (b), wortlauttreu.
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-45 · Notfall-Löschen durch den Administrator

- **Herkunft:** `15` F-18 · **Bezug:** LTV-009 (privat), ADR 0032, E-4
- **Varianten:**
  - (A) Der Administrator löscht nie fremde Aufnahmen; er senkt nur Kontingente und benachrichtigt; der Server lehnt neue Aufnahmen ab.
  - (B) Er kann je Konto «älteste Aufnahmen bis X GB löschen», ohne Titel zu sehen, mit Meldung an das Konto.
- **Empfehlung:** (A), passt zu «privat» und zu «nichts löscht automatisch».
- **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-46 · Laufende Aufnahme wächst über das Kontingent

- **Herkunft:** `03` F-9; `17` G-3 · **Bezug:** LTV-019 (Schutz laufender Aufnahmen), E-4
- **Varianten:**
  - (a) Die Aufnahme läuft zu Ende; das Konto liegt danach über dem Kontingent, neue Aufnahmen werden abgelehnt (Meldung an Profil und Hauptprofil).
  - (b) Die Aufnahme stoppt beim Erreichen und gilt als unvollständig.
- **Empfehlung:** (a); das Kontingent wird um höchstens eine Aufnahme überschritten.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-47 · Standard-Aufbewahrungsfrist des Administrators

- **Herkunft:** `03` F-10 · **Bezug:** LTV-019 (automatische Aufbewahrungsfristen), Auftrag 9.6, E-4
- **Worum es geht:** Der Auftrag nennt Aufbewahrungsfristen in der Verwaltung; nach E-4 löscht Slouchcat
  automatisch nur, was das Profil selbst eingestellt hat.
- **Varianten:**
  - (a) Die Frist des Administrators ist nur ein Vorschlag, den der Planer vorfüllt; sie gilt erst, wenn das Profil sie speichert.
  - (b) Sie gilt für alle Aufnahmen ohne eigene Frist, auch bestehende: das widerspricht E-4 und wäre eine zweite Ausnahme von ADR 0032.
- **Empfehlung:** (a), ab Werk mit dem Vorschlag «unbegrenzt»; (b) nur, wenn du E-4 änderst.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-48 · Update, während eine Aufnahme läuft

- **Herkunft:** `04` F-6; `15` LA-27 · **Bezug:** LTV-002 (Schutz laufender Aufnahmen)
- **Varianten:** (a) Warnen und bestätigen lassen (Aufnahmen laufend oder in den nächsten 15 Minuten); (b) Update
  sperren, bis keine Aufnahme läuft; (c) nichts tun wie heute.
- **Empfehlung:** (a); (b) könnte Sicherheitsupdates bei Serienaufnahmen über Tage blockieren.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-49 · Höchstzahl gleichzeitiger Aufnahmen auf dem Server

- **Herkunft:** `15` F-21; `13` P-6; `04` O-12; `11` K-3 · **Bezug:** LTV-008.10; LTV-017 (kein Anbieterlimit)
- **Worum es geht:** Eine Grenze schützt die Hardware, nicht den Anbieter.
- **Varianten:** (A) Keine Grenze (Aufnahmen kopieren nur); (B) einstellbare Grenze, ab Werk «keine».
- **Empfehlung:** (B).
- **Vorgabe bis zur Antwort:** (B).
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-50 · Gleiche Sendung, zwei Profile

- **Herkunft:** `11` F-2 und Abschnitt 9; `11` M-6 · **Bezug:** LTV-009.12 (Deduplizierung untersuchen)
- **Varianten:** D0 getrennte Dateien; D1 harte Verknüpfungen im Dateisystem; D2 Zählung in der Datenbank;
  D3 Kopie ohne Platzbedarf (nur auf manchen Dateisystemen).
- **Empfehlung:** D0 ohne Vorbereitung für D1 bis D3; nach M-6 neu fragen, falls doppelte Aufnahmen häufig sind.
- **Vorgabe bis zur Antwort:** D0.
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-51 · Bestehende Aufnahmen beim Wechsel des Speicherorts

- **Herkunft:** `15` F-17; `11` 10.5 · **Bezug:** LTV-019 (kein automatisches Verschieben)
- **Varianten:** (A) Bestehende Aufnahmen bleiben am alten Ort abspielbar, der alte Ort bleibt lesend eingetragen, bis
  er leer ist; (B) Wechsel nur, wenn der alte Ort leer ist; (C) einmaliges Verschieben auf ausdrücklichen Knopfdruck.
- **Empfehlung:** (A).
- **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

### A.8 Vor Stufe 6

#### E-28-52 · Serienaufnahme ohne zuverlässige Folgenkennung

- **Herkunft:** `03` F-8; `09` 7.2 (Wahl «höchstens eine je Tag» gestrichen); `18` V-9 · **Bezug:** LTV-008 (bereits
  aufgenommene Folgen «sofern zuverlässig identifizierbar»)
- **Worum es geht:** Viele Programmdaten tragen keine Folgenkennung; dann erkennt Slouchcat Wiederholungen nicht.
- **Varianten:**
  - (a) Alle passenden Ausstrahlungen aufnehmen, die Planung zeigt «Folgen nicht erkennbar, Wiederholungen werden mit aufgenommen».
  - (b) Höchstens eine Ausstrahlung je Tag: spart Platz, verpasst Doppelfolgen; eine Zusatzregel, die der Auftrag nicht nennt.
  - (c) Serienaufnahme für solche Sendungen nicht anbieten, nur wiederkehrende Aufnahme nach Uhrzeit.
- **Empfehlung:** (a); «behalte N» mildert den Platzbedarf.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 6 · **Gewicht:** Weiche

#### E-28-53 · Zusätzliche Felder der Serienregel

- **Herkunft:** `14` F-23; `11` O-6 und 14.1a; `07` O-3, W-10, W-11 · **Bezug:** LTV-008
- **Worum es geht:** Mockups und `14` zeigen mehr Felder, als `11` und `07` vorsehen.
- **Varianten:**
  - (A) Ohne diese Felder: Eine Serienregel nimmt jede Ausstrahlung mit gleichem Titel auf; eine wiederkehrende Aufnahme läuft, bis das Profil sie beendet.
  - (B) Mit Zeitfenster und Wochentagen für Serien (und «nur Erstausstrahlungen», wo die Programmdaten es angeben): «Tatort, nur Sonntag 20:00 bis 22:30» schliesst Wiederholungen ohne Folgenkennung aus.
  - (C) Wie (B), dazu ein Ende der wiederkehrenden Aufnahme («am Datum» oder «nach N Aufnahmen»).
- **Empfehlung:** (B), das wirksamste Mittel gegen Wiederholungen.
- **Vorgabe bis zur Antwort:** (A); die Felder werden nicht gebaut.
- **Fällig vor:** Stufe 6 · **Gewicht:** Feinschliff

### A.9 Vor den Stufen L, W, E und den App-Ausgaben

#### E-28-54 · RTSP und RTMP

- **Herkunft:** `04` F-1; `18` U-7 und V-11 · **Bezug:** LTV-003 (RTSP, RTMP); Auftrag 10.3; ADR 0025
- **Worum es geht:** Go spricht diese beiden Protokolle nicht von Haus aus.
- **Varianten:**
  - (a) Neue Go-Bibliotheken (`gortsplib`, `gortmplib`, `mediacommon`, alle MIT): drei Abhängigkeiten mehr; vorher wird belegt, dass sie nur über den geprüften Verbindungsweg verbinden (V-11).
  - (b) FFmpeg öffnet RTSP und RTMP selbst, als zweite Ausnahme von ADR 0025: keine Abhängigkeit, aber Zugangsdaten in der Argumentliste und ungeprüfte Weiterleitungen.
  - (c) Später: bis dahin fehlt dieser Teil von LTV-003.
- **Empfehlung:** (a) in der späten Stufe L.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe L · **Gewicht:** Weiche

#### E-28-55 · DASH und HLS mit fMP4, falls Weg W nicht dicht ist · Wortlaut

- **Herkunft:** `04` F-13; `review-1-sicherheit` Frage 2 (RS-02, RS-03); `18` U-3, U-6, V-10; `12` R-3 · **Bezug:**
  LTV-003 (MPEG-DASH ohne DRM, HLS); ADR 0025; T-3, T-4
- **Worum es geht:** DASH und HLS mit fMP4 liest FFmpeg selbst (Weg W). Das wird erst ganz am Schluss gebaut und nur,
  wenn die Sperre verfügbar ist und eine Messung zeigt, dass FFmpeg keine andere Verbindung öffnet. Darum kommen
  diese Quellarten spät und nur nach Messung.
- **Varianten:**
  - (a) Nur mit Sperre: Auf Hosts ohne die nötige Kernel-Funktion gibt es DASH und HLS mit fMP4 nicht; die Verwaltung sagt es.
  - (b) Go liest selbst (eigener Leser, FFmpeg nur zum Umpacken über Pipes): keine Ausnahme von ADR 0025, aber viel eigener Code für seltene Quellen.
  - (c) Auch ohne Sperre, mit dem Restrisiko, dass FFmpeg den Netzschutz umgeht.
  - (d) Gar nicht: ein Abstrich an LTV-003.
- **Empfehlung:** (a), und (b) erst, wenn V-1 zeigt, dass du solche Quellen wirklich hast.
- **Vorgabe bis zur Antwort:** (a); fehlt die Sperre oder scheitert V-10, gilt (b), weil (d) dem Wortlaut widerspricht und (c) unsicher ist.
- **Fällig vor:** Stufe W · **Gewicht:** Weiche

#### E-28-56 · Untertitel im Browser · Wortlaut

- **Herkunft:** `18` O-7 und 4.12; `16` Abschnitt 10 Nr. 16; `17` G-14; `18` D-6; Rückmeldung «offene technische
  Punkte aus der Konsolidierung» · **Bezug:** LTV-004.17, LTV-014 (Player: Untertitel); `04` O-5
- **Worum es geht:** Der schlanke Bau von hls.js (der Web-Player) lässt Untertitel-Spuren weg, und ein voller Bau
  ist nicht vorgesehen. Bis Stufe E zeigt der Browser darum keine Untertitel; LTV-004.17 ist dort bis dahin nicht
  erfüllt (Fernseher und Handy-App zeigen DVB-Untertitel schon ab Stufe 1a).
- **Varianten:**
  - (a) Eigene Textspur im Player neben hls.js: Der Server liefert WebVTT, der Player zeigt es selbst; kein grösserer Player.
  - (b) Voller Bau von hls.js als eigener Teil, nur für Sitzungen mit Untertiteln: mehr Code im Browser.
  - (c) Untertitel im Browser nur eingebrannt in einer Bild-Umwandlung: kostet Rechenzeit je Zuschauer.
- **Empfehlung:** (a), unter Vorbehalt von M-3 und M-24, weil sie weder den Player vergrössert noch Umwandlung erzwingt.
- **Vorgabe bis zur Antwort:** (a) in Stufe E.
- **Fällig vor:** Stufe E · **Gewicht:** Weiche

#### E-28-57 · Bedienung des TV-Players mit dem Steuerkreuz

- **Herkunft:** `14` F-7; `19` Abschnitt 6 · **Bezug:** LTV-010, LTV-014
- **Varianten:** (A) OK = Bedienleiste, ← = Senderliste, → = Info, ↑/↓ = Sender wechseln (so das TV-Mockup);
  (B) wie der Videoplayer: OK = Pause, ←/→ spulen, Senderwechsel nur über Liste, Kanaltasten und Ziffern.
- **Empfehlung:** (A), schnelles Zappen hat Vorrang, und viele Fernbedienungen haben keine Kanaltasten.
- **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** der ersten TV-App-Ausgabe (bei E-28-01 (b) also Stufe 1) · **Gewicht:** Feinschliff

#### E-28-58 · Abnahme der TV-App-Ausgaben

- **Herkunft:** `18` O-6 und Abschnitt 5 · **Bezug:** LTV-021
- **Varianten:** (a) Dieselbe Prüfliste wie die Stufe, am Gerät, dazu Fernbedienung und Zurück; (b) eine verkürzte
  Prüfliste.
- **Empfehlung:** (a), wie in `18` Abschnitt 5 beschrieben.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** der ersten App-Ausgabe · **Gewicht:** Feinschliff

---

## Teil B · Offene technische Entscheidungen

Für die Entwicklung, nicht für dich. Erledigte Punkte stehen nur im Anhang.

| Kennung | Titel | Herkunft | Worum es geht | Hängt ab von | Fällig vor |
|---|---|---|---|---|---|
| OT-28-01 | Form der Fähigkeitsangaben der Geräte | `04` O-1 | additive Felder für `mpeg2video`, `mp2`, `aac_latm`, Zeilensprung, Untertitelformate, «in TS abspielbar» | M-7 | 1a |
| OT-28-02 | «Strom normalisieren» | `04` O-2 | FFmpeg nur über Pipes für Quellen ohne Keyframe-Kennzeichen | M-2 (V-1) | 1a, sonst 3 |
| OT-28-03 | Zweites Segmentformat fMP4 | `04` O-3; `16` Nr. 2, 8 | Umpacken je Sitzung für HEVC auf Apple und im Browser | M-2, M-5 | nach 1a |
| OT-28-04 | Zeilensprung: doppelte Bildrate, «im Browser durchreichen» | `04` O-4, O-23 | Option und Einstellung nach Bewertung | M-6 | 1a |
| OT-28-05 | Untertitel: Teletext nach WebVTT, Anzeige im Browser, Untertitel in Aufnahmen | `04` O-5; `11` O-3; `16` Nr. 16; `18` O-7 | technische Umsetzung nach E-28-56 | M-24, M-3, E-28-56 | E |
| OT-28-06 | Voller Bau von hls.js, CEA-608 im schlanken Bau | `04` O-22; `16` Nr. 3; Nachprüfung Nr. 12, 13 | Grösse des Zusatzteils, AC-3/HEVC, Untertitel im Bildstrom | M-2, M-3 | nach V-2 |
| OT-28-07 | Schnittstelle des Umwandlungs-Managers für Live | `04` O-7 | reservieren, umbuchen, freigeben ohne Änderung der Dateiwiedergabe | bestehende Tests | 1a |
| OT-28-08 | Netzmodus «host»: Zusatzdatei und Erkennung | `04` O-10; `12` O-4 | Zusammenspiel mit Port, Proxys, HTTPS; was gilt ohne Zusatzdatei | M-16, E-28-34 | H |
| OT-28-09 | Mehrere Programme in einem Multicast-Strom | `04` O-11 | je Sender ein Socket auf derselben Gruppe | M-16 | H |
| OT-28-10 | Zuschnitt der ADR und Zusatz zu Regel 3 | `04` O-13, O-26; `12` O-1; `18` O-3 | ADR 0046 ff.; Hülle in `internal/transcode` darf sich durch FFmpeg ersetzen | Freigabe | 0 |
| OT-28-11 | Codec-Helfer von `mediacommon` | `04` O-21 | ersetzen sie die eigene Erkennung der Bildparameter? | E-28-54 | L |
| OT-28-12 | Folgeänderungen in den Papieren nachführen | `04` O-24; `18` O-1 | Stufe H, D-3, D-7, D-9 und die Punkte aus Teil E | Teil E | 0 |
| OT-28-13 | Herzschlag im Hintergrund-Tab und bei Bildübertragung | `04` O-25; `16` Nr. 18; `17` G-15; `12` R-9 | bis zur Messung gilt die 60-Sekunden-Regel ohne Ausnahme | M-3, M-5 | 1a |
| OT-28-14 | Landlock und Seccomp im Go-Code | `04` O-27; `12` O-2, O-3; Nachprüfung Nr. 34, 35 | `x/sys/unix` oder `go-landlock` ohne `BestEffort`; Docker-Profil; ABI-Fassung; hart oder ausgewiesen | M-25 (V-4), E-28-04 | 0, 1a |
| OT-28-15 | Auswahl der Variante bei HLS-Quellen mit mehreren Qualitäten | `05` O-14 | Vorgabe höchste Qualität; Obergrenze je Quelle? | – | 1a |
| OT-28-16 | RTP mit Sitzungsbeschreibung, Fehlerkorrektur, 204-Byte-TS | `05` O-15 | in Stufe H nicht unterstützt; Bedarf | M-2, M-16 | H |
| OT-28-17 | DRM-Grenze bei `SAMPLE-AES` | `05` O-16; `12` O-6, R-10; Nachprüfung N-13 | Grenze Transportverschlüsselung/DRM nach RFC 8216 belegen | Lesen der Quelle | 1a |
| OT-28-18 | Erste Wiedergabeliste in der Antwort auf «einstellen» | `05` O-17; `08` O-4 (Rest) | spart eine Anfrage | M-1 | 1a |
| OT-28-19 | Segmentlängen in der Startphase, längere Bildgruppen im Betrieb | `05` O-19 | 2 s im Direktweg, nicht unter 1 s für Media3 | M-1, M-4, M-21 | 1a |
| OT-28-20 | Ersatz des FFmpeg-Empfängers in Weg W | `05` O-20 | wie oft, wie lange die Lücke | M-12 | W |
| OT-28-21 | Bis zu sechs Segmente für Apple-Player ohne Recht «Timeshift» | `05` O-28; `16` Nr. 6 | erweitert das Rückspulen auf bis 15 s; nur mit Gerät und dann als Frage an dich | M-5, E-28-12 | nur mit Gerät |
| OT-28-22 | Umschlüsseln bestehender `v1`-Werte nach `v2` | `07` O-1 | Format in `12` 5.1 | – | 1a |
| OT-28-23 | Satzformat des Index je Teil | `07` O-2; `11` O-2 | feste Länge, Version, Prüfsumme | – | 5 |
| OT-28-24 | Grösse von `live_stream_events` | `07` O-4 | reichen 50'000 Zeilen für 7 Tage? | Betrieb | nach 3 |
| OT-28-25 | `live_channel_streams.params` als JSON | `07` O-5 | eigene Spalte, falls nach Codec gefiltert wird | – | 3 |
| OT-28-26 | Dauer der Import-Transaktion | `07` O-8 | 11'000 Senderquellen; über 2 s Zwischentabelle | `07` M-1 | 1a |
| OT-28-27 | Schematest für Indizes der Fremdschlüssel | `07` O-9 | jede Kindspalte mit führendem Index | – | 1a |
| OT-28-28 | Unbekannte Aufzählungswerte in den Apps | `08` O-1 | Ausweichwert in den erzeugten Kotlin-Clients per Schema-Test | – | 1a |
| OT-28-29 | ETag für Programmraster und Senderliste | `08` O-2 | nur falls die Messungen es verlangen | `09` M-4, M-5 | nach 2 |
| OT-28-30 | Kopfzeilen und User-Agent für EPG-Quellen | `08` O-3, W-27 | heute keine | – | 2 |
| OT-28-31 | Form der `programmeRef` bei gleichen Sendungen | `08` O-5 | zweite Unterscheidung nötig? | – | 2 |
| OT-28-32 | xz und zstd für EPG | `09` O-1 | Fremdbibliothek, Lizenz, Pflege; nur bei Bedarf | echte Quelle | nach 2 |
| OT-28-33 | Zeit- und XML-Feinheiten | `09` O-2; Nachprüfung Nr. 32, 40 | Sommerzeit-Lücke, `xmltv_ns`, Tiefe und Menge in der `Token`-Schleife | – | 2 |
| OT-28-34 | SQLite-Fassung in `modernc.org/sqlite` | `09` O-3 | nur FTS5 mit externem Inhalt | – | 2 |
| OT-28-35 | Xtream ohne brauchbares `xmltv.php` | `09` O-4 | Einzelabfragen nachrüsten? | `09` M-12 | nach 2 |
| OT-28-36 | XMLTV aus einem Ordner | `09` O-5 | nur über `paths` (Regel 4) | – | nach 2 |
| OT-28-37 | Push-Dringlichkeit «high» für Erinnerungen | `09` O-6 | heute immer normal | `09` M-8, E-28-22 | nach 2 |
| OT-28-38 | HD- und SD-Sender auf demselben EPG-Kanal | `09` O-7 | Serienregel, Doppelaufnahmen | – | 6 |
| OT-28-39 | Bedeutung von `tvg-shift` | `09` O-9 | Stunden, Vorzeichen bei echten Listen | `09` M-12 | 2 |
| OT-28-40 | Titelzusätze wie «(Wh.)» | `09` O-10 | bewusst nicht wegnormalisiert | `09` M-12 | 6 |
| OT-28-41 | Beschreibung in der Programmsuche | `09` O-11; `08` W-26 | Grösse, Nutzen | `09` M-6 | nach 2 |
| OT-28-42 | ADR «Programmdaten in eigener Datei» | `09` O-12 | nur mit E-28-24 (a) | E-28-24 | 2 |
| OT-28-43 | Anzeige-Zeitzone für E-Mail und Telegram | `09` O-13 | Server- oder Profilzeitzone | – | 2 |
| OT-28-44 | Segment unter der Abspielstelle gelöscht | `10` O-3 | hls.js springt ab Werk nicht; Client setzt fort | `05` M-3 | 4 |
| OT-28-45 | Schwelle «gesehen» für Aufnahmen | `11` O-4 | aus `playback` übernehmen | – | 5 |
| OT-28-46 | Höchstzahl 50 Termine je Regel, Horizont 14 Tage | `11` O-5 | bestätigen | – | 6 |
| OT-28-47 | Wartefrist beim Löschen eines Profils mit laufender Aufnahme | `11` O-7 | Vorschlag 10 s | – | 5 |
| OT-28-48 | Abgleich laufender Aufnahmen mit geänderten Programmdaten | `11` O-1 | `09` 6.3 muss es nachtragen | – | 5 |
| OT-28-49 | Testdialer und Namen der Testwerkzeuge | `12` O-7; `17` O-1 | Build-Tag nur Loopback; CI prüft das Release-Binary | – | 0 |
| OT-28-50 | Meldung mit Adresse im Heimnetz | `12` O-10 | nur für die selbst eingetippte Adresse | – | H |
| OT-28-51 | WebP-Decoder für Logos | `12` O-11 | neue Abhängigkeit `golang.org/x/image` oder kein WebP | – | 1a |
| OT-28-52 | CI-Zeit für Live und Fuzz-Laufzeit | `12` O-12; `17` O-12 | Dauer des Gates `ci`, Zeilen in `testgroups.txt` | Messung bei 1a | 1a |
| OT-28-53 | Stockungen beim Client melden | `13` O-5 | keine Erkennung der Ausgangsbandbreite im Server | – | 3 |
| OT-28-54 | Grenze offener Dateien in der Compose-Datei | `13` O-6 | rund 500 nötig, 65'536 als Reserve | E-28-03 | 0 |
| OT-28-55 | Test-Zertifizierungsstelle für HTTPS-Quellen | `17` O-4 | nur im Test-Binary | – | 1a |
| OT-28-56 | Lint-Riegel gegen die Altersprüfung der Bibliothek im Paket `live` | `17` O-5 | Beleg für `AK-L-179` | – | 1a |
| OT-28-57 | Multicast in der CI | `17` O-6 | Netzwerk-Namensraum mit virtuellem Paar | Versuch | H |
| OT-28-58 | Freiplatz-Schnittstelle | `17` O-7 | volle und fehlende Datenträger ohne `mount` nachstellen | – | 4 |
| OT-28-59 | Messhaken im Client | `17` O-9 | nur im Entwicklungsmodus | E-28-20 | 1a |
| OT-28-60 | Ort des Wegwerf-Werkzeugs von V-1 | `18` O-5 | eigener, nie gemergter Zweig oder Arbeitsordner | – | V |
| OT-28-61 | Umgebung, Wiederverbindung und Zeitlimits von FFmpeg | Nachprüfung Nr. 25, 26 | `http_proxy` leeren; `reconnect` und `rw_timeout` ausdrücklich setzen | – | 1a |

---

## Teil C · Messungen und Versuche vor dem Bau

Die Versuche der Stufe V sind klein, wegwerfbar und laufen ausserhalb des Produkts (`18` 2.2). Ohne ihr Ergebnis wird
die genannte Stufe nicht freigegeben.

### C.1 Versuche der Stufe V

| Kennung | Was gemessen wird | Trägt Entscheid oder Frage | Muss vorliegen vor | Gerät oder Quellen von dir |
|---|---|---|---|---|
| V-1 (`05` M-2) | deine echten Quellen: Codec, Zeilensprung, Ton, Bildgruppen-Länge, Bitrate, Schub beim Verbinden, zweite Verbindung | T-7, T-11, T-12, T-15; E-28-06, E-28-13, E-28-44, E-28-55; OT-28-02, -03, -06 | 1a | **ja: deine IPTV-Zugänge**, je Quelle wenige Minuten, bis 50 Sender, mit deiner Zustimmung |
| V-2 (`05` M-3) | hls.js im schlanken Bau mit TS-Segmenten, Diskontinuitäten, Startpunkt, Untertitel im Bildstrom, Hintergrund-Tab | T-11; E-28-56; OT-28-06, -13, -44 | 1a | nein (Chrome, Firefox, selbst erzeugte Testströme) |
| V-3 (`05` M-5) | Safari, iPhone, iPad: Zeit bis Bild bei Listen mit 1, 3, 6 Segmenten | E-28-12; OT-28-03, -21 | keine Baufreigabe; nur für eine Zusage | **ja: ein Apple-Gerät**; ohne Gerät läuft der Versuch nicht |
| V-4 (`05` M-25) | Sperre für FFmpeg auf CT 190: Landlock, Seccomp, Docker-Profil, `no-new-privileges`, Prozessgrenze, QSV mit Adressraumgrenze | T-27; E-28-03, E-28-04, E-28-55; OT-28-14 | 0 (Härtung), 1a, W | **ja: Zeit auf CT 190** |
| V-5 (`05` M-4) | Media3 mit HLS-TS: Startpuffer 1000/500 ms, Stockungen, Decoder beim Umschalten | E-28-01; Player-Einstellungen | Stufe, in der die Apps Live-TV bekommen (bei E-28-01 (b): 1a) | **ja: Pixel und Android-TV-Gerät** |
| V-6 (`05` M-16) | UDP und Multicast im Container: Bridge, «host», `macvlan`/`ipvlan`, Paketverlust | E-28-34; OT-28-08, -09, -16 | H | **ja: CT 190 und dein Heimnetz** |
| V-7 (`05` M-14, `10` M-1, `11` M-1, M-2) | Schreiblast je Zuschauer und Aufnahme auf SSD, HDD, RAM-Disk, NAS; NAS 30 s weg; `fdatasync` | T-8, T-25; E-28-36 (c); NAS als Speicherort «Aufnahmen» (`18` U-5) | 4 und 5 | **ja: CT 190 und dein NAS** (wird kurz getrennt) |
| V-8 (`13` M-1) | 72 Stunden Dauerempfang mit 20 Testsendern | Freigabe der Stufe 3 (`18` U-4) | 3 | **ja: 72 h Zeitfenster auf CT 190** |
| V-9 (`09` M-10 bis M-12) | deine echten Programmdaten: Schwankung, fehlende Endzeiten, Folgenkennung, Zeitversatz | E-28-52, E-28-53; Schrumpf-Schwelle; OT-28-35, -39, -40 | 2 (läuft nebenher) | **ja: deine XMLTV-Dateien**, zwei Wochen |
| V-10 (`05` M-10, M-11, M-12 30 h) | Weg W dicht; FFmpeg-Leser für DASH und HLS-fMP4 30 Stunden | T-4; E-28-55; OT-28-20 | W | ja: 30 h auf CT 190 |
| V-11 (`05` M-23) | RTSP- und RTMP-Bibliotheken verbinden nur über den geprüften Weg | E-28-54 | L | nein |
| V-12 (`11` M-7) | Zonendaten im Image, wiederkehrende Aufnahmen über eine Zeitumstellung | RP-19 | 6 | nein |

### C.2 Messungen während des Baus, die eine Entscheidung tragen

| Kennung | Was gemessen wird | Trägt | Muss vorliegen vor | Von dir |
|---|---|---|---|---|
| `05` M-1 | Umschaltzeit je Fall, Client und Netz (Benchmark) | E-28-13; Abnahme | Abnahme 1a, 2, 3 | Abnahme am Gerät (E-28-20) |
| `05` M-6, M-7 | Zeilensprung je Client; Decoder je Gerät | T-15; OT-28-01, -04 | während 1a | Pixel, TV-Gerät |
| `05` M-8, `13` M-5 | Live-Umwandlung: Startzeit, Kosten, Ton-Umwandlung, Speicher je Prozess | Budgetwerte `04` 7.6; E-28-14 | während 1a | Zeit auf CT 190 |
| `13` M-6 | Wirkung auf die Dateiwiedergabe | E-28-14 (Grenze Filmstart) | Abnahme 1a, 3, 5 | Zeit auf CT 190 |
| `05` M-18 | Verhalten hinter deinem Reverse-Proxy | Abnahme | Abnahme 1a | **ja: dein Aufbau** |
| `05` M-22 | Zeitüberlauf nach 26,5 h und 53 h | Regel «FFmpeg-Lauf höchstens 24 h» | während 1a | nein |
| `05` M-13, M-15, M-17 | Ring je Sender; Schwellen der Überwachung; Gewinn durch Vorladen | Vorgaben `04` 6.3; E-28-29, E-28-33 | während 3 | nein |
| `05` M-12 (7 Tage) | dauerhafter Empfang über 7 Tage | Abnahme LTV-002.01 | Abnahme 3 | ja: 7 Tage auf CT 190 |
| `05` M-20 | Filter für Standbild, Schwarzbild, Stille, Kosten | E-28-31 | E (bei E-28-31 (b): 3) | nein |
| `05` M-9, M-19; `10` M-2, M-3 | nahtloses Spulen mit Umwandlung; lange Listen am TV; RAM-Disk im Container | Rückfall `05` 4.5; Freigabe RAM-Disk | während 4 | TV-Gerät |
| `09` M-1 bis M-9 | Parser, Schreibtransaktionen, Wirkung des Imports, Raster, Push-Verzögerung | E-28-24 (`09` M-3); OT-28-29, -37, -41 | während 2 | Geräte für `09` M-8 |
| `11` M-3 bis M-6 | Pünktlichkeit des Planers, Wiedergabe von Aufnahmen, Neuaufbau des Index, doppelte Aufnahmen | E-28-50 (`11` M-6); Abnahme 5 | während und nach 5 | nein |
| `05` M-24 | DVB-Untertitel und Teletext in Media3, Umsetzung nach WebVTT | E-28-56; OT-28-05 | Apps, E | Pixel, TV-Gerät |
| `07` M-1 | Dauer der einen Import-Transaktion | OT-28-26 | während 1a | nein |

### C.3 Was du beisteuern musst

- **Deine echten IPTV-Quellen** für V-1 (Bildgruppen, Codecs, Bitrate): ohne sie gibt es keine Baufreigabe der Stufe 1.
  Ins Repo kommen nur Kennzahlen, nie Adressen, Senderlisten oder Zugangsdaten.
- **Deine echten XMLTV-Dateien** über zwei Wochen für V-9, früh beginnen.
- **Ein Apple-Gerät** (iPhone, iPad oder Mac), nur falls Safari, iPhone und iPad zugesagt werden sollen (E-28-12).
- **Zeit auf CT 190** in angekündigten Fenstern: V-4, V-6, V-7 (mit kurzem Trennen des NAS), V-8 (72 h),
  `05` M-12 (7 Tage), V-10 (30 h). Updates auf CT 190 vorher abstimmen (`17` R-4). Für Netz- und Speicherausfälle
  braucht das Testskript Rechte für `tc netem`, `umount`, `kill -STOP` (`17` R-11).
- **Pixel und Android-TV-Gerät** für V-5, M-6, M-7, M-19, M-24 und die Geräteprüfungen (E-28-20).
- **Deinen Reverse-Proxy** für M-18 und dein **Heimnetz** für V-6.

---

## Teil D · Technische Risiken

Zusammengeführt aus `04` 12.1, `12` 13.1, `13`, `17` 10.1 und `18` 8.1.

| Risiko in Alltagssprache | Wirkung | Gegenmassnahme | Woran man es früh merkt |
|---|---|---|---|
| Deine Sender passen selten direkt in den Browser (Zeilensprung, MP2, MPEG-2, AC-3, HEVC) | wenige gleichzeitige Zuschauer im Browser, viel Umwandlung | Ton-Umwandlung als billiger Weg; Kapazität neu rechnen (`18` U-1) | V-1 |
| Web-Player oder Safari kommen mit TS-Segmenten schlecht zurecht | langsames Umschalten, Abbrüche | Umstellung auf fMP4 an einer Stelle (T-11, `18` U-2) | V-2, V-3 |
| Apple-Geräte sind nie geprüft | Live-TV auf iPhone und Mac kann schlecht laufen, ohne dass es jemand merkt | ungeprüft und nicht zugesagt, Hinweis im Release (E-28-12) | erst mit Gerät (V-3) |
| Der Anbieter sendet selten ein vollständiges Bild | ein kalter Sender braucht über 3 s | Länge je Sender messen und zeigen; Vorladen | V-1, M-1 |
| Der Anbieter erlaubt nur wenige Verbindungen | Störungen beim Vorladen, gescheiterte Aufnahmen | erst freigeben, dann verbinden; Ablehnung als «vermutlich Limit»; E-28-02, E-28-29 | M-17, Betrieb |
| Der Kernel erlaubt keine Sperre für FFmpeg | Schadcode in FFmpeg läse alle Zugangsdaten | Sperre hart oder sichtbar fehlend; Weg W nur mit Sperre (E-28-04) | V-4 |
| Weg W lässt sich nicht als dicht belegen | DASH und HLS mit fMP4 fehlen oder brauchen eigenen Code | Stufe W zuletzt; E-28-55 | V-10 |
| Speicher, Goroutinen oder offene Dateien wachsen über Tage | Absturz nach Tagen | 72-h- und 7-Tage-Lauf, 1'000 Umschaltungen | V-8, `05` M-12, `13` M-2 |
| Viele Dauersender brauchen viel Arbeitsspeicher | Ablehnung neuer Sender | Byte-Budget aus dem Arbeitsspeicher, Ablehnung statt Wachstum | `05` M-13 |
| Netzspeicher stockt oder fällt weg; viele Timeshift-Puffer schreiben | Lücken in Aufnahmen, im schlimmsten Fall Schreiben auf die falsche Platte | kein stilles Ausweichen (T-25), Rückstau je Schreiber, E-28-36 (c) | V-7 |
| Der EPG-Import blockiert die eine Schreibverbindung | stockende Wiedergabe und Verwaltung | eigene Datei (E-28-24), Stapel | `09` M-2, M-3 |
| Live-TV verdrängt Filme | langsamer Filmstart | Live-Anteil am Budget, E-28-14 | `13` M-6 |
| Zugangsdaten gelangen in Protokolle, Antworten oder Sicherungen | Zugänge offen | Schwärzung in Stufe 0, ganze Adresse verschlüsselt, E-28-11 | Sicherheitstests SM-14 bis SM-20 |
| Ein Gerät im Heimnetz fälscht einen UDP- oder Multicast-Sender | falsches Bild, auch für Kinder und in Aufnahmen | Bindung an eine Schnittstelle, klarer Text, Empfehlung eigenes Netzsegment | bleibt Restrisiko (`12` R-2) |
| Netzmodus «host» öffnet den Server nach innen | Dienste auf dem Host hinter jeder Lücke | eigene Stufe H, Zusatzdatei, E-28-34 | V-6 |
| Stufe 1 ist sehr gross | Prüfung und Review leiden | Schnitt 1a und 1b, kleine Planschritte | Bau 1a (`18` U-12) |
| Lange Messläufe blockieren CT 190 | deine eigenen Tests und Updates warten | angekündigte Fenster, Zwischenstand je 24 h | Planung |
| Der Herzschlag fällt im Hintergrund-Tab oder bei Bildübertragung aus | Sitzung endet nach 60 s | messen, dann Ausnahme (OT-28-13) | V-2, M-5 |
| Ältere Apps vertragen neue Werte nicht | leere Einträge in der App | Auslieferung nur an Clients mit Live-TV-Fassung; Schema-Test (OT-28-28) | Bau 1a |
| Ein Bauteil rutscht Richtung Ausschlussliste (Archiv, gemeinsamer Puffer) | Verstoss gegen den Auftrag | Negativtests «gibt es nicht» in jeder Stufe | Review je Stufe |
| Die Zeitumstellung verschiebt wiederkehrende Aufnahmen | Aufnahme eine Stunde daneben | Tests mit gestellter Uhr | V-12 |
| Schema-Fehler aus Review 2 (Verklemmung beim Löschen, vertauschte Sender beim Import, Lecks der Privatsphäre) | Hänger, falsche Zuordnung, Titel sichtbar | Arbeitspunkte je Stufe (`18` 4.14) | Tests unter `-race` |
| Der Simulator verhält sich anders als echte Anbieter | grüne Tests, Fehler im Betrieb | V-1; neue Eigenheit zuerst im Simulator nachstellen | V-1, Betrieb |
| Zeilensprung-Entfernung ist zu teuer | wenige Zuschauer bei solchen Sendern | Filter der GPU, Kosten nach Messung | M-6, M-8 |
| Spulen mit Umwandlung über mehrere FFmpeg-Läufe springt | sichtbare Sprünge | Rückfall «spiele ab Uhrzeit» (`05` 4.5) | M-9 |
| Ein deaktivierter Sender läuft für laufende Zuschauer weiter | gesperrter Inhalt noch kurz sichtbar | Befehl «sofort beenden» (T-29) | Verwaltung |
| Ein Token einer Wiedergabe-Adresse ist nach dem Abfangen 60 s nutzbar | fremde Person schaut kurz mit | kurze Laufzeit, Bindung an Sitzung | bleibt Restrisiko (`12` R-9) |
| Der Betreiber verletzt die Bedingungen seiner Anbieter | rechtlich, deine Sache | Hinweis in Verwaltung und Anleitung | – |

---

## Teil E · Bekannte Unstimmigkeiten zwischen den Dokumenten

Stand nach Konsolidierung 1, am 28. November 2026 in den Dateien geprüft. Massgeblich für alle Stufenangaben ist
`18` (dort so festgelegt).

| Nr. | Stelle A | Stelle B | Massgeblich | Stand nach der Restebereinigung |
|---|---|---|---|---|
| U-1 | `04` T-3: «UDP, RTP, Multicast (Stufe 3)» | `18` D-11, `07` 11.2, `08` 2.8, `12` SM-41: Stufe H | `18`: Stufe H | erledigt in `04` (T-3: Stufe H; 10.9 Stufe H; 10.6 Zusatzdateien Stufe H) und `05` (2.1, 2.2, O-15) |
| U-2 | `04` 10.6 und Tabelle 10.8: Compose-Härtung «1, vorbehaltlich F-12» | `18` D-9, Schritt 0.6; `17` `AK-L-281`: Stufe 0 | `18`: Stufe 0 | erledigt in `04` (10.6 und Tabelle 10.8: Stufe 0, `18` Schritt 0.6) |
| U-3 | `04` T-11: M-2, M-3 **und M-4** ohne Bedingung vor Stufe 1 | `05` M-4, `17` 8.2, `18` D-7: M-4 vor der Stufe, in der die Apps Live-TV bekommen | `05`/`18` (bei E-28-01 (b) fällt es auf 1a) | erledigt in `04` (T-11: M-4 an die Stufe der Apps, M-5 bedingt, kein Apple-Gerät) |
| U-4 | `04` 10.8 und `18` RP-4: Anzeige «FFmpeg läuft ohne Sandbox» | `12` F-1 und R-1: «Eindämmung nicht verfügbar» | `12` (ein Text in der Verwaltung) | erledigt in `04` (10.8) und `18` (RP-4); Hinweis in `12` 4.8 gestrichen |
| U-5 | `04` 15.1: W-11 «belegt (RFC 8216 gelesen)» | `12` R-10, O-6; `05` O-16; Nachprüfung Abschnitt 5: DRM-Grenze nicht nachgelesen | belegt ist nur die Startregel; die DRM-Grenze bleibt Annahme (OT-28-17) | erledigt in `04` (15.1, Zeile `24` W-11: nur die Startregel belegt) |
| U-6 | `04` 10.9 schweigt zu laufenden Verbindungen, wenn ein festes Ziel entzogen wird | `12` SM-47: sie werden geschlossen | `12` | erledigt in `04` (10.9, neue Zeile «Entzug eines festen Ziels») |
| U-7 | `04` (Antwort auf `12` Zeile 430): Filter für Systemaufrufe nur für ältere Kerne | `12`: Seccomp immer, zusätzlich zu Landlock | `12` (strenger), bis M-25 anderes zeigt | erledigt in `04` (T-4, 10.8, 15.1: Seccomp immer) |
| U-8 | `18` Schritt 0.4: Schwärzung für «Ersatznamen» | `05` 2.6 Fassung 3: Weg W nutzt Ersatzpfade | `05`: Ersatzpfade | erledigt in `18` (Schritt 0.4: Ersatzpfad, kein Schwärzungsmuster) |
| U-9 | `04` F-10 fragt nur «Grenze nachträglich gesetzt» | `07` F-3, `review-2` RD-18: auch «Grenze entfernt» | zusammengeführt in E-28-07 | erledigt in `04` (F-10, Absatz «Gegenrichtung» mit Verweis auf E-28-07) |
| U-10 | `04` T-18, 5.3; `15` LA-03 Schritt 5, LA-09, V-7: Schalter «weitere gleichzeitige Verbindungen erlaubt»; `15` 3.9 (LA-09) hängt ihn an `04` F-3; Mockup: «vorsorglich zusätzliche Verbindungen öffnen» | Festlegung des Koordinators zu RM-01: «Vorladen für Sender dieser Quelle: an/aus», Einstellung des Vorladens, keine Verbindungsrechnung, hängt nur an `04` F-9 | Koordinator; Frage E-28-29 | erledigt in `04` (T-17, T-18, 5.3, 6.1, F-9, 15.1), `05` (6, 9), `06`, `15` (Grundsatz 3, LA-03 Schritt 5, LA-09, V-7, Abschnitt 8) und `17` (`AK-L-101`, `-316`); Frage bleibt E-28-29 |
| U-11 | `09` 10.3: neue Spalte `live_reminder_id` in `notifications` | `07` 3.11 und W-31: `live_ref` ohne Fremdschlüssel | `07` | erledigt in `09` (10.3: `live_ref`) |
| U-12 | `13` 10.2 M-20: Stichprobe «während des Baus Stufe 3 (letzter Schritt), sonst Stufe E» | `04` F-14 (a), `18` 4.12 und D-5, `17` H-14: Stufe E | `18`: Stufe E (bei E-28-31 (b) Stufe 3) | erledigt in `13` (10.2 M-20: Stufe E, bei E-28-31 (b) Stufe 3) |
| U-13 | `05` O-28 und `16` 3.2: sechs Segmente «berühren LTV-005» | gemeint ist das Rückspulen nach LTV-007 | LTV-007 | erledigt in `05` (O-28) und `16` (3.2, Abschnitt 9 Nr. 6) |
| U-14 | `14` LD-05 im Mockup: Vorlauf der Erinnerung 2 und 10 Minuten | `03` DR-08: 0, 5, 15, 30 Minuten (`07` W-2) | `03`; Mockup angleichen | bleibt offen, weil nur das Mockup abweicht (`14` 11 nennt es); Mockups gehören nicht zu dieser Bereinigung |
| U-15 | `04` 10.5 und `07` 6.3 beschreiben «Chiffrat bleibt in der Sicherung» als Folge | `12` F-4 Vorgabe: leeren | `12`; nach E-28-11 angleichen | erledigt: `04` 10.5 und `07` 6.3 setzen die Vorgabe «leeren» schon um (Platzhalter `v0:`); die Alternative hängt an E-28-11 |
| U-16 | `16` 10 Nr. 16 und `18` 4.12: Stufe E «WebVTT im Browser» | `04` 7.4, `05` 4.3: kein voller Bau von hls.js | offen, Frage E-28-56 | bleibt offen, weil es eine Frage an dich ist (E-28-56) |
| U-17 (neu) | `07` 3.2 (live_sources) und 11.2: Spalte `live_sources.allow_more_conns`; `08` 3.7: Feld `allowMoreConnections` («mehr Verbindungen erlauben») | Festlegung des Koordinators zu RM-01 (`04` T-18, `15`): «Vorladen für Sender dieser Quelle: an/aus», keine Verbindungsverwaltung | Koordinator (Entscheid nach der Restebereinigung) | erledigt nach Entscheid des Koordinators: `07` 3.2 und 11.2 `live_sources.preload_enabled`, `08` 3.7 `preloadEnabled`, `18` 4.5 |
| U-18 (neu) | `04` T-17 und 5.3 (massgeblich): Deutung «vermutlich Verbindungslimit (des Anbieters)»; ebenso `06` (Tabelle der Ablehnungen), `15` LA-13 (Protokolltext), `17` 5.13 (LA-13) | Festlegung des Koordinators: das Wort «Verbindungslimit» nur im Verweis auf LTV-017 und die offene Frage | Koordinator | erledigt nach Entscheid des Koordinators: «Quelle lehnt eine weitere Verbindung ab» in `04` T-17, 5.3, `05` 9.6, `06` N-8, `11` 4.1, `15` LA-13, `17` `AK-L-213`; Regel unverändert |
Erledigt und in den Dateien nachgezogen (nicht mehr offen): Timeshift-Höchstdauer 60 Minuten und Deckel aus 20 %
und 50 GB (`15` zeigt 2 Stunden nur als Wert der Beispielinstanz), Update-Warnfenster 15 Minuten, Upload-Grösse
200 MiB, Reserve «Aufnahmen», 72 h gegen 7 Tage, Stufen für RTSP, RTMP und DASH (D-1, D-2), LTV-018 zwischen 1a
und 1b (D-12), «höchstens eine je Tag» in `09` 7.2, Zählungen in `18` 4.3 (158 `AK-L` in 1a, mit `AK-L-333`), Weg W im
Live-Anteil (`13` H-5), Ring- und Rückstau-Reservierung in `04` 8.3 (`13` H-9). Aus `22` Abschnitt 7 nach der Restebereinigung: Summe der Ausschlüsse in `03` (46),
Überschriften von LTV-009, -016, -017, -020, Negativtest für LTV-X-29 (`AK-L-333`), 17 der 27 Stufenabweichungen
zwischen `03` und `17` (nach `18` in `03` oder `17` ergänzt). Ohne Änderung: die 10 Abweichungen mit Stufe K
(`AK-L-310`, `-312` prüfen in der Konzeptphase die Dokumente; die Anforderung wird in der Stufe aus `03` gebaut) und
die Teilstufen gemischter `AK-L` bei LTV-004 (4, 6), LTV-008 (1), LTV-009 (1), die `18` 10.1 nicht als eigene
Stufe des Entscheids führt. `23-agent-einsatz-und-reviews.md` ist geplant, aber noch nicht angelegt.

---

## Anhang · Zuordnung

### A. Fragen der Papiere zu `E-28-nn`

| Quelle | Zu |
|---|---|
| `02` E-1, E-2, E-3, E-4 | beantwortet (Abschnitt «Bereits beantwortet») |
| `03` F-1 | E-28-09 |
| `03` F-2 | E-28-39 |
| `03` F-3 | E-28-22 |
| `03` F-4 | E-28-40 |
| `03` F-5 | E-28-23 |
| `03` F-6 | E-28-01 |
| `03` F-7 | E-28-41 |
| `03` F-8 | E-28-52 |
| `03` F-9 | E-28-46 |
| `03` F-10 | E-28-47 |
| `04` F-1 | E-28-54 |
| `04` F-2 | E-28-10 |
| `04` F-3 | E-28-02 |
| `04` F-4 | E-28-30 |
| `04` F-5 | E-28-40 |
| `04` F-6 | E-28-48 |
| `04` F-7 | E-28-35 |
| `04` F-8 | E-28-36 |
| `04` F-9 | E-28-29 |
| `04` F-10 | E-28-07 |
| `04` F-11 | E-28-34 |
| `04` F-12 | E-28-03 |
| `04` F-13 | E-28-55 |
| `04` F-14 | E-28-31 |
| `07` F-1 | E-28-07 |
| `07` F-2 | E-28-09 |
| `07` F-3 | E-28-07 |
| `08` F-1 | E-28-42 |
| `09` F-1 | E-28-24 |
| `09` F-2 | E-28-26 |
| `09` F-3 | E-28-22 |
| `09` F-4 | E-28-25 |
| `10` F-1 | E-28-37 |
| `11` F-1 | E-28-44 |
| `11` F-2 | E-28-50 |
| `12` F-1 | E-28-04 |
| `12` F-2 | E-28-05 |
| `12` F-3 | E-28-06 |
| `12` F-4 | E-28-11 |
| `14` F-1 | E-28-09 |
| `14` F-2 | erledigt durch `03` DR-23 (keine Live-TV-Reihe auf der Startseite) |
| `14` F-3 | E-28-23 |
| `14` F-4 | E-28-43 |
| `14` F-5 | erledigt durch `03` DR-21 (kein Fortschritt) |
| `14` F-6 | E-28-22 |
| `14` F-7 | E-28-57 |
| `14` F-8 | E-28-28 |
| `14` F-9 | E-28-38 |
| `14` F-10 | E-28-41 |
| `14` F-11 | E-28-27 |
| `14` F-12 | E-28-16 |
| `14` F-23 | E-28-53 |
| `15` F-13 | E-28-15 |
| `15` F-14 | erledigt durch `03` DR-16 (neue Sender ab Werk aktiv, je Quelle abschaltbar) |
| `15` F-15 | E-28-17 |
| `15` F-16 | E-28-18 |
| `15` F-17 | E-28-51 |
| `15` F-18 | E-28-45 |
| `15` F-19 | E-28-32 |
| `15` F-20 | E-28-39 |
| `15` F-21 | E-28-49 |
| `15` F-22 | E-28-19 |
| `13` P-1 | E-28-30 |
| `13` P-2 | E-28-40 |
| `13` P-3 | E-28-02 |
| `13` P-4 | E-28-36 |
| `13` P-5 | E-28-29 |
| `13` P-6 | E-28-49 |
| `13` P-7 | E-28-14 |
| `13` P-8 | E-28-14 |
| `13` P-9 | erledigt durch `04` 8.1 |
| `review-1-sicherheit` Frage 1 | E-28-07 |
| `review-1-sicherheit` Frage 2 | E-28-55 |
| `review-1-sicherheit` Frage 3 | E-28-34 |
| `review-1-sicherheit` Frage 4 | E-28-03 |
| `review-1-architektur` RA-03, RA-09, RA-18, W-10 | E-28-35, E-28-36, E-28-29, E-28-02 |
| `review-2` RD-05, RD-18 | E-28-07 |
| `review-3` RM-01 | E-28-29 |
| `17` O-2, O-3, O-8, O-10, O-11, O-13 | E-28-21, E-28-14, E-28-13, E-28-20, E-28-12, E-28-20 |
| `18` O-4, O-6, O-7 | E-28-08, E-28-58, E-28-56 |
| `09` O-8 | E-28-18 |
| `13` O-4 | E-28-33 |
| `16` Abschnitt 10 Nr. 16 | E-28-56 |
| Nachprüfung der Quellen | keine Fragen an dich; Folgen in OT-28-06, -14, -17, -33, -61 |

`05`, `06`, `13`, `16`, `17`, `18` und `19` haben keine eigenen `F-n`; `06` und `19` verweisen nur auf `04` F-4, F-5,
F-7, F-8, F-9 und `14` F-4, F-7, F-9.

### B. Offene Punkte der Papiere zu `OT-28-nn`

| Quelle | Zu |
|---|---|
| `04` O-1, O-2, O-3, O-4 | OT-28-01, -02, -03, -04 |
| `04` O-5 | OT-28-05 |
| `04` O-6 | entfällt (ersetzt durch `04` F-8, E-28-36) |
| `04` O-7 | OT-28-07 |
| `04` O-8 | E-28-24 |
| `04` O-9 | E-28-41 |
| `04` O-10, O-11 | OT-28-08, -09 |
| `04` O-12 | E-28-49 |
| `04` O-13 | OT-28-10 |
| `04` O-21, O-22, O-23 | OT-28-11, -06, -04 |
| `04` O-24, O-25, O-26, O-27 | OT-28-12, -13, -10, -14 |
| `05` O-14, O-15, O-16, O-17 | OT-28-15, -16, -17, -18 |
| `05` O-18 | entfällt |
| `05` O-19, O-20, O-28 | OT-28-19, -20, -21 |
| `06` O-1 bis O-12 | erledigt durch `04` Fassung 3 (13.4) |
| `07` O-1, O-2 | OT-28-22, -23 |
| `07` O-3 | E-28-53 |
| `07` O-4, O-5 | OT-28-24, -25 |
| `07` O-6 | erledigt |
| `07` O-7 | E-28-42 |
| `07` O-8, O-9 | OT-28-26, -27 |
| `08` O-1, O-2, O-3 | OT-28-28, -29, -30 |
| `08` O-4 | erledigt durch `04` 14.4 (Rest OT-28-18) |
| `08` O-5 | OT-28-31 |
| `08` O-6 | E-28-42 |
| `08` O-7 | erledigt |
| `09` O-1 bis O-7 | OT-28-32, -33, -34, -35, -36, -37, -38 |
| `09` O-8 | E-28-18 |
| `09` O-9 bis O-13 | OT-28-39, -40, -41, -42, -43 |
| `10` O-1, O-2 | erledigt durch `04` 14.6 |
| `10` O-3 | OT-28-44 |
| `11` O-1 | OT-28-48 |
| `11` O-2, O-3, O-4, O-5 | OT-28-23, -05, -45, -46 |
| `11` O-6 | E-28-53 |
| `11` O-7 | OT-28-47 |
| `12` O-1 | erledigt durch `04` 10.8, O-26 (OT-28-10) |
| `12` O-2, O-3, O-4 | OT-28-14, -14, -08 |
| `12` O-5 | erledigt durch `05` 2.6 |
| `12` O-6, O-7 | OT-28-17, -49 |
| `12` O-8 | erledigt durch `04` 14.9 |
| `12` O-9 | E-28-25 |
| `12` O-10, O-11, O-12 | OT-28-50, -51, -52 |
| `13` O-1, O-2, O-3, O-7 | erledigt durch `04` 8.1, 7.6 und `07` |
| `13` O-4 | E-28-33 |
| `13` O-5, O-6 | OT-28-53, -54 |
| `16` Abschnitt 10 Nr. 3, 6, 18 | OT-28-06, -21, -13 |
| `16` Abschnitt 10 Nr. 16, 20 | E-28-56, E-28-12 |
| `16` Abschnitt 10 Nr. 19 | Annahme, Messung `05` M-4 (V-5) |
| `16` Abschnitt 10 übrige Nummern | entschieden in `05` Fassung 3 |
| `17` O-1 | OT-28-49 |
| `17` O-2, O-3 | E-28-21, E-28-14 |
| `17` O-4, O-5, O-6, O-7 | OT-28-55, -56, -57, -58 |
| `17` O-8 | E-28-13 |
| `17` O-9 | OT-28-59 |
| `17` O-10, O-11 | E-28-20, E-28-12 |
| `17` O-12 | OT-28-52 |
| `17` O-13 | E-28-20 |
| `18` O-1 | OT-28-12 |
| `18` O-2 | erledigt durch `17` Fassung 2 |
| `18` O-3 | OT-28-10 |
| `18` O-4 | E-28-08 |
| `18` O-5 | OT-28-60 |
| `18` O-6, O-7 | E-28-58, E-28-56 |
| Nachprüfung Nr. 12, 13 / 25, 26 / 32, 40 / 34, 35 / N-13 | OT-28-06 / -61 / -33 / -14 / -17 |

*Ende von `20-offene-entscheidungen.md`. Nichts in diesem Dokument ist gebaut; jede Vorgabe gilt, bis du anders
entscheidest.*
