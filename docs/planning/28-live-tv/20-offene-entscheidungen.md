# Konzept 28 · Offene Entscheidungen

Stand: 29. November 2026 · Fassung 2 · Status: Entwurf nach Gesamtreview, nicht freigegeben

> **Kurz gesagt.** Diese Liste sammelt alles, was du für Live-TV entscheiden sollst, aus den Papieren 02 bis 19,
> den sechs Reviews und der Nachprüfung der Quellen. Es sind **69 Fragen** (`E-28-01` bis `E-28-69`):
> **38 Weichen** und **31 Feinschliff**. Vor der Annahme des Konzepts musst du nur die zwölf Weichen auf der
> Seite «Das Wichtigste zuerst» kennen; alle Feinschliff-Fragen können bis zu ihrer Stufe warten. Dazu kommen
> 68 offene technische Punkte für die Entwicklung (Teil B), die Messungen vor dem Bau (Teil C), die Risiken
> (Teil D) und die Unstimmigkeiten, die zwischen den Papieren noch bestehen (Teil E). Nichts davon ist gebaut.

## So liest du diese Liste

1. **Was ohne deine Antwort gilt:** die Zeile **«Vorgabe»**. Sie ist **wortlauttreu**, also das, was dein Auftrag
   vom 28. November 2026 wörtlich verlangt; bei Sicherheitsfragen ist sie die **sichere** Variante. Wo selbst die
   Vorgabe eine Anforderung erst später erfüllt, steht dort «weicht in der Zeit ab» mit der Stufe, ab der sie gilt.
2. **Die Empfehlung gilt erst mit deiner Antwort.** Sagst du «wie empfohlen», gilt die Empfehlung; sagst du nichts,
   gilt die Vorgabe. Wo beide gleich sind, steht es dabei.
3. **Zwei Schritte:** «Konzept freigegeben» heisst, du hast das Konzept angenommen; eine Freigabe ohne
   Einzelantworten setzt die **Vorgaben** in Kraft, nicht die Empfehlungen. «Baufreigabe einer Stufe» heisst, du
   beauftragst ausdrücklich den Bau dieser einen Stufe (Regel 11); vorher wird nichts gebaut.
4. **Fälligkeit:** Teil A ist nach der Stufe geordnet, vor der die Antwort da sein muss (Stufen nach `18`: V, 0, 1a,
   1b, 2, 3, H, 4, 5, 6, L, W, E, Apps). Vor Stufe 0 sind 13 Fragen fällig, vor Stufe 1a weitere 19.
5. **Weiche** ändert Umfang, Sicherheit oder sichtbares Verhalten; **Feinschliff** ist ein Detail innerhalb eines
   festen Umfangs, die Vorgabe ist ein sicherer Weg. Der Vermerk **«Wortlaut»** im Titel heisst: Der Entwurf
   erfüllt eine verbindliche Anforderung nicht wörtlich oder nicht in jeder Stufe, oder die Empfehlung weicht vom
   Wortlaut ab.

### Was du schon entschieden hast (nur Verweis)

Entschieden hast du die **21 Grundsatzentscheide** deines Auftrags (LTV-001 bis LTV-021) und diese vier Antworten.
Alles andere, auch die Leitplanken und Auslegungen des Koordinators, ist hier als Frage geführt (E-28-59 bis
E-28-62).

| Nr. | Thema | Deine Antwort (Kurzform) | Stelle |
|---|---|---|---|
| E-1 | Zielprodukt | aktuelles Go-Produkt; Abschnitt 5 des Auftrags nur im Sinn | `02` 3 |
| E-2 | Umfang der Mockups | Web, Android TV und Handy-App | `02` 3 |
| E-3 | Aufnahmen von Profilen mit Altersgrenze | Hauptprofil darf sie sehen, löschen und abspielen; andere bleiben ganz privat | `02` 3 |
| E-4 | Kontingent voll | neue Aufnahme abgelehnt und gemeldet; automatisch gelöscht nur nach eigener Frist und «behalte N» | `02` 3 |

## Das Wichtigste zuerst

Diese zwölf Weichen solltest du kennen, bevor du das Konzept annimmst. Je zwei Zeilen: worum es geht; was ohne
deine Antwort gilt und was empfohlen ist.

1. **E-28-02 · Quellen mit wenigen Verbindungen (LTV-017).** Soll es je Quelle einen Schalter oder eine Zahl für
   Zugänge mit nur einer Verbindung geben? **Vorgabe:** nein, nichts je Quelle; **Empfehlung:** ein Schalter je Quelle.
2. **E-28-30 · Dauerempfang gegen neuen Zuschauer (LTV-002).** Weicht ein Dauersender, den gerade niemand schaut,
   einem neuen Zuschauer? **Vorgabe:** nein, die neue Anfrage wird abgelehnt; **Empfehlung:** der Zuschauer geht vor.
3. **E-28-04 · Umwandlung ohne Sperre.** Darf FFmpeg Live-TV umwandeln, wenn der Kernel keine Sperre erlaubt?
   **Vorgabe:** nein, dann nur Sender ohne Umwandlung; **Empfehlung:** dieselbe.
4. **E-28-59 · Auslegungen des Koordinators bestätigen.** Rechte-Zuständigkeit, Zählung zu den Grenzen,
   Heimnetz, «Kinderprofil», kein eigener Internetdienst. **Vorgabe:** so wie in `02` 3.1; **Empfehlung:** bestätigen.
5. **E-28-60 · Fire TV.** Dein Auftrag nennt es unter «Berücksichtige», das Repo führt es als Nicht-Ziel.
   **Vorgabe:** berücksichtigt (geprüft wird, ob die TV-App läuft), keine Zusage; **Empfehlung:** dieselbe.
6. **E-28-12 · Safari, iPhone, iPad.** Sie gehören zum Umfang, aber es gibt kein Gerät zum Prüfen. **Vorgabe:**
   vorgesehen, ungeprüft, nichts entschieden; **Empfehlung:** auf die Zusage verzichten, bis ein Gerät da ist.
7. **E-28-56 · Untertitel.** Im Browser gibt es bis Stufe E keine wählbaren Untertitel, Teletext in keinem Client.
   **Vorgabe:** sie gehören zum Umfang, bis Stufe E nicht erfüllt; **Empfehlung:** so, Teletext vorziehen, falls nötig.
8. **E-28-10 · Speicherort des Startpuffers (LTV-019).** Wählbar wie verlangt, oder nur im Arbeitsspeicher?
   **Vorgabe:** wählbar; **Empfehlung:** nur Arbeitsspeicher (Abweichung, nur auf dein Wort).
9. **E-28-07 · Aufnahmen und Altersgrenze.** Was sieht das Hauptprofil, wenn eine Grenze später gesetzt oder
   entfernt wird? **Vorgabe:** nur Aufnahmen mit Grenze, solange sie noch gilt; **Empfehlung:** dieselbe.
10. **E-28-09 · Werkseinstellung der Rechte.** Wer sieht Live-TV nach dem Update? **Vorgabe:** nur
    Administrator-Konten, alle anderen aus; **Empfehlung:** dieselbe.
11. **E-28-03 · Härtung der Compose-Datei.** Drei Schutz-Einstellungen für jede Installation, auch ohne Live-TV?
    **Vorgabe:** für alle, nach einem Probelauf; **Empfehlung:** dieselbe.
12. **E-28-01 · Clients je Stufe.** Kommt Live-TV gleich auch auf den Fernseher? **Vorgabe:** Web und TV-App je
    Stufe, Handy-App gesammelt danach; **Empfehlung:** dieselbe.

**Alle 31 Feinschliff-Fragen können warten**, bis ihre Stufe drankommt; bis dahin gilt ihre Vorgabe.

## Kurzübersicht

Geordnet nach Kennung; Teil A ordnet nach Fälligkeit.

| Kennung | Titel | Vorgabe (ohne Antwort) | Empfehlung | Fällig vor | Gewicht |
|---|---|---|---|---|---|
| E-28-01 | Welche Clients in welcher Stufe | (b) Web und TV-App je Stufe | (b) | 0 | Weiche |
| E-28-02 | Quellen mit wenigen Verbindungen (Wortlaut) | (a) nichts je Quelle | (b) Schalter je Quelle | 0 | Weiche |
| E-28-03 | Härtung der Standard-Compose-Datei für alle | (a) für alle, nach Probelauf | (a) | 0 | Weiche |
| E-28-04 | Umwandlung, wenn der Kernel keine Sperre erlaubt | (b) nur mit Sperre | (b) | 0 | Weiche |
| E-28-05 | Weiterleitung von `https` nach `http` | (a) immer abbrechen | (a) | 0 | Weiche |
| E-28-06 | Zertifikatsprüfung abschaltbar | (a) nie | (a) | 0 | Weiche |
| E-28-07 | Aufnahmen und Altersgrenze: Zeitpunkt und Definition | A (b), B (a) | A (b), B (a) | 0 | Weiche |
| E-28-08 | Eigenes Release für Stufe 0 | (a) nein | (a) | 0 | Feinschliff |
| E-28-09 | Werkseinstellung der Rechte | (a) mit (i) | (a) mit (i) | 1a | Weiche |
| E-28-10 | Speicherort des Startpuffers (Wortlaut) | (a) wählbar | (b) nur Arbeitsspeicher | 1a | Weiche |
| E-28-11 | Zugangsdaten in der «Sicherung ohne Geheimnisse» | (a) leeren | (a) | 1a | Weiche |
| E-28-12 | Safari, iPhone, iPad: Gerät beschaffen oder auf Zusage verzichten (Wortlaut) | vorgesehen, ungeprüft | (b) verzichten, bis ein Gerät da ist | 1a | Weiche |
| E-28-13 | Schwellen des Benchmarks; Zielwerte hängen an der Quelle (Wortlaut) | (a) Vorschlag | (a) | 1a | Weiche |
| E-28-14 | Live-TV gegen Filme | (a) kein Borgen | (a) | 1a | Weiche |
| E-28-15 | Verwaltungsbereich | (C) | (C) | 1a | Feinschliff |
| E-28-16 | Eintrag in der Web-Navigation | (A) | (A) | 1a | Feinschliff |
| E-28-17 | Sortierung der Sender | (A) | (A) | 1a | Feinschliff |
| E-28-18 | Kategorien und Genres | (a) | (a) | 1a | Feinschliff |
| E-28-19 | CPU-, RAM- und GPU-Werte in der Verwaltung | (A) anzeigen | (A) | 1a | Feinschliff |
| E-28-20 | Geräteprüfungen und Kamera-Gegenprobe | (a) | (a) | 1a | Feinschliff |
| E-28-21 | Blender-Filme im Benchmark | (a) nein | (a) | 1a | Feinschliff |
| E-28-22 | Kanäle für Erinnerungen und Aufnahme-Meldungen | (a) | (a) | 2 | Weiche |
| E-28-23 | Programmsuche in der allgemeinen Suche | (a) nur im Live-TV-Bereich | (a) | 2 | Weiche |
| E-28-24 | Programmdaten in eigener Datei | (a) | (a) | 2 | Weiche |
| E-28-25 | EPG-Adressen im Heimnetz | (b) nein | (a) feste Ziele, Stufe H | 2 | Weiche |
| E-28-26 | Namensgleiche Sender zuordnen | (a) nur Vorschlag | (a) | 2 | Feinschliff |
| E-28-27 | Sendungsbilder im Raster | (A) keine | (A) | 2 | Feinschliff |
| E-28-28 | Altersangaben der Programmdaten | (A) nie anzeigen | (A) | 2 | Feinschliff |
| E-28-29 | Werkszustand des globalen Vorladens | (c) an, vorsichtige Obergrenzen | (c); bei E-28-02 (b) dann (b) | 3 | Weiche |
| E-28-30 | Dauerempfang gegen neuen Zuschauer (Wortlaut) | (b) Dauersender bleibt | (a) Zuschauer geht vor | 3 | Weiche |
| E-28-31 | Erkennung eingefrorener Bilder (Wortlaut) | (a) zwei Schritte | (a) | 3 | Weiche |
| E-28-32 | Welche Ausfälle der Administrator gemeldet bekommt | (A) | (A) | 3 | Feinschliff |
| E-28-33 | Angabe der Anschlussbandbreite | (a) keine | (a) | 3 | Feinschliff |
| E-28-34 | Multicast: Netzmodus «host» oder eigenes Netz (Wortlaut) | (b), sonst (a) | dieselbe | H | Weiche |
| E-28-35 | Rückspulgrenze | (a) am Tastendruck | (a) | 4 | Weiche |
| E-28-36 | Speicherform des Timeshift-Puffers | (a) eigene Kopie | (a) | 4 | Weiche |
| E-28-37 | Sofortaufnahme im Zeitversatz | (a) ab jetzt | (a) | 4 | Feinschliff |
| E-28-38 | Rückfrage beim Senderwechsel aus dem Zeitversatz | (B) | (B) | 4 | Feinschliff |
| E-28-39 | Vorgabe des Aufnahme-Kontingents | (a) 100 GB | (a) | 5 | Weiche |
| E-28-40 | Beginnende Aufnahme gegen Wiedergaben | (a) reservieren | (a) | 5 | Weiche |
| E-28-41 | Entzug des Rechts: Planungen und laufende Aufnahmen | (a) | (a) | 5 | Weiche |
| E-28-42 | Hauptprofil löscht, wenn das Konto kein Live-TV mehr hat | (b) nein | (a) ja (A-12) | 5 | Weiche |
| E-28-43 | Was das Hauptprofil ausser Aufnahmen sieht | (A) nur Aufnahmen | (B) | 5 | Weiche |
| E-28-44 | Spuren ausserhalb des TS (Wortlaut) | (b) Begleitdatei | (a) ehrlich anzeigen | 5 | Weiche |
| E-28-45 | Notfall-Löschen durch den Administrator | (A) nie | (A) | 5 | Weiche |
| E-28-46 | Laufende Aufnahme wächst über das Kontingent (Wortlaut) | (b) stoppt am Kontingent | (a) läuft zu Ende | 5 | Feinschliff |
| E-28-47 | Standard-Aufbewahrungsfrist des Administrators | (a) nur Vorschlag | (a) | 5 | Feinschliff |
| E-28-48 | Update, während eine Aufnahme läuft | (a) warnen | (a) | 5 | Feinschliff |
| E-28-49 | Höchstzahl gleichzeitiger Aufnahmen | (A) keine Grenze | (B) einstellbar | 5 | Feinschliff |
| E-28-50 | Gleiche Sendung, zwei Profile | D0 | D0 | 5 | Feinschliff |
| E-28-51 | Bestehende Aufnahmen beim Wechsel des Speicherorts | (A) | (A) | 5 | Feinschliff |
| E-28-52 | Serienaufnahme ohne Folgenkennung | (a) | (a) | 6 | Weiche |
| E-28-53 | Zusätzliche Felder der Serienregel | (A) ohne | (B) | 6 | Feinschliff |
| E-28-54 | RTSP und RTMP | (a) Go-Bibliotheken | (a) | L | Weiche |
| E-28-55 | DASH und HLS mit fMP4, falls Weg W nicht dicht ist (Wortlaut) | (a), sonst (b) | (a) | W | Weiche |
| E-28-56 | Untertitel bis Stufe E (Wortlaut) | (a) Stufung, bis E nicht erfüllt | (a), dazu (b) bei Teletext | 1a | Weiche |
| E-28-57 | Bedienung des TV-Players mit dem Steuerkreuz | (A) | (A) | Apps | Feinschliff |
| E-28-58 | Abnahme der TV-App-Ausgaben | (a) volle Prüfliste | (a) | Apps | Feinschliff |
| E-28-59 | Auslegungen des Koordinators bestätigen | wie `02` 3.1 | bestätigen | 0 | Weiche |
| E-28-60 | Fire TV (Wortlaut) | (a) berücksichtigt, keine Zusage | (a) | 0 | Weiche |
| E-28-61 | Plattform-Streams (YouTube, Twitch) | (a) nicht eigens unterstützt | (a) | 0 | Feinschliff |
| E-28-62 | Download und Offline von Aufnahmen | (a) nicht verlangt, nicht geplant | (a) | 0 | Feinschliff |
| E-28-63 | Hauptprofil kann jede PIN ersetzen | (a) hinnehmen | (b) Meldung ans Profil | 5 | Weiche |
| E-28-64 | Strengerer Adressfilter auch für Radio und Podcasts | (a) für alle, mit Hinweis | (a) | 1a | Feinschliff |
| E-28-65 | Live-TV beendet Radio und Hörbuch | (a) wie heute beim Wechsel | (a) | 1a | Feinschliff |
| E-28-66 | Rechte, die der Auftrag «bestehend» nennt, die es nicht gibt | (a) keine neuen Rechte | (a) | 1a | Weiche |
| E-28-67 | Einstellungen, die der Auftrag nicht verlangt | (a) einstellbar wie im Entwurf | (b) feste Werte | 1a | Feinschliff |
| E-28-68 | Übrige FFmpeg-Aufrufe der Bibliothek schützen | (a) Bestand unverändert | (b) kleine Umgebung für alle | 0 | Feinschliff |
| E-28-69 | Verhalten des Anbieters über einen Neustart merken | (a) nicht merken | (a) | 1a | Feinschliff |

---

## Teil A · Fragen an dich

### A.1 Vor Stufe 0 (vor der ersten Baufreigabe)

#### E-28-01 · Welche Clients in welcher Stufe

- **Herkunft:** `03` F-6; `18` Abschnitt 5, D-10, O-6 · **Bezug:** LTV-014, LTV-021; Regel 8 (Tastentest)
- **Worum es geht:** Jede Stufe kann zuerst nur in der Web-App kommen, gleich auch auf dem Fernseher oder auf allen
  drei Geräten. Davon hängt ab, wie gross jede Stufe wird, wann der Fernseher Live-TV bekommt und ob vor Stufe 1a
  schon auf dem Pixel und dem TV-Gerät gemessen werden muss (V-5).
- **Varianten:**
  - (a) Web zuerst, die Apps gesammelt danach: jede Stufe ist am kleinsten, der Fernseher wartet lange.
  - (b) Web und TV-App in jeder Stufe, die Handy-App gesammelt nach Stufe 5 oder 6: Fernsehen kommt früh auf den Fernseher; jede Stufe wird grösser, V-5 und der TV-Tastentest werden Pflicht.
  - (c) Alle drei in jeder Stufe: volle Abdeckung, aber jede Stufe am grössten.
- **Empfehlung:** (b), wobei die TV-App-Ausgabe nach dem Server-Release folgen darf, weil Live-TV meist auf dem
  Fernseher geschaut wird.
- **Vorgabe bis zur Antwort:** (b); der Auftrag legt keine Reihenfolge fest.
- **Fällig vor:** Stufe 0 · **Gewicht:** Weiche

#### E-28-02 · Quellen mit wenigen Verbindungen · Wortlaut

- **Herkunft:** `04` F-3 und O-32; `03` DR-29; `13` P-3; `02` AU-10; früher E-28-29 (Schalter je Quelle);
  `review-4` RG-03 und SC-1; `review-3` RM-01; `review-1-architektur` RA-01 und W-10 · **Bezug:** LTV-017, LTV-002,
  LTV-010; T-17, T-18
- **Worum es geht:** Viele IPTV-Zugänge erlauben nur eine oder wenige gleichzeitige Verbindungen. Dein Auftrag
  schliesst jede Verwaltung von Verbindungen je Anbieter aus; der Administrator verantwortet die Verbindungszahl
  selbst. Slouchcat merkt darum erst im Betrieb, dass eine Quelle eine weitere Verbindung ablehnt (Muster A) oder
  die ältere trennt (Muster B); dann ruht der vorsorgliche Empfang dieser Quelle von selbst (30 Minuten, 6 Stunden,
  24 Stunden), und es bleibt bei einem Strom (Ein-Strom-Betrieb). **Zweck der Frage:** Ein Zugang mit nur einer
  Verbindung soll das Vorladen nicht stören. **Folgen:** Ohne Einstellung je Quelle stockt bei Muster B ein Bild
  einmal kurz, bevor der Server es gelernt hat, und nach jedem Neustart wieder (E-28-69). Wer das vermeiden will,
  schaltet das Vorladen **für alle** Quellen aus. Schauen mehr Personen verschiedene Sender, als der Anbieter
  erlaubt, bekommt die letzte «Quelle lehnt eine weitere Verbindung ab»; eine geplante Aufnahme kann daran scheitern.
- **Varianten:**
  - (a) Nichts je Quelle: Vorladen, «automatisch» und Prüfungen nur global (an/aus, Obergrenzen) und je Sender; die Verwaltung empfiehlt bei einem Zugang mit einer Verbindung, das Vorladen global auszuschalten.
  - (b) Zusätzlich ein Schalter je Quelle «Vorladen für Sender dieser Quelle: an/aus»: Eine knappe Quelle wird vom vorsorglichen Empfang ausgenommen, die anderen behalten ihn. In der Sache unterscheidet er Zugänge mit einer von Zugängen mit mehreren Verbindungen; Review 4 wertet ihn als Verbindungskontingent je Anbieter. Er **berührt LTV-017** und braucht eine Spalte, ein Feld der API und eine Zeile in der Verwaltung.
  - (c) Zusätzlich eine Verbindungszahl je Quelle «höchstens N gleichzeitige Empfänge»: Der Server lehnt sauber ab, bevor etwas stört, und Aufnahmen können Plätze reservieren. Das **ändert LTV-017**.
- **Empfehlung:** (b), weil ein Haushalt mit einem knappen und einem grosszügigen Zugang sonst für beide auf das
  Vorladen verzichten muss; (c) nur, wenn dir Reservierungen für Aufnahmen wichtig sind. Beides nur auf dein Wort.
- **Vorgabe bis zur Antwort:** (a), wortlauttreu zu LTV-017. Den Werkszustand des **globalen** Vorladens fragt
  E-28-29.
- **Fällig vor:** Stufe 0 (gehört in ADR 0046) · **Gewicht:** Weiche

#### E-28-03 · Härtung der Standard-Compose-Datei für alle

- **Herkunft:** `04` F-12; `review-1-sicherheit` Frage 4 (RS-03); `18` Schritt 0.6 und D-9; `review-4` SC-8 ·
  **Bezug:** T-27; `AK-L-281`
- **Worum es geht:** Drei Einstellungen in der Docker-Datei (keine neuen Rechte im laufenden Prozess, alle
  Sonderrechte entzogen, Höchstzahl an Prozessen) machen FFmpeg schwerer angreifbar. Sie ändern keine Funktion,
  aber jede bestehende Installation.
- **Varianten:**
  - (a) In `compose.yaml` für alle: jede Installation ist gehärtet, auch für Filme.
  - (b) Nur in der Zusatzdatei für Live-TV: nur wer Live-TV einschaltet, bekommt sie.
- **Empfehlung:** (a), weil es nichts kostet und auch die heutige Filmumwandlung schützt.
- **Vorgabe bis zur Antwort:** (a), sobald M-25 (V-4) und ein Probelauf der ganzen Prüfreihe grün sind.
- **Fällig vor:** Stufe 0 · **Gewicht:** Weiche

#### E-28-04 · Darf FFmpeg umwandeln, wenn der Kernel keine Sperre erlaubt?

- **Herkunft:** `12` F-1 und R-1; `review-6` RT-01; `04` T-27, 10.8, F-13 (Schlussabsatz); `18` U-3, RP-4; `02` A-13 ·
  **Bezug:** LTV-004; T-12, T-27
- **Worum es geht:** Die Umwandlung je Zuschauer liest fremde Daten mit FFmpeg. Die Sperre (Landlock: der Kernel legt
  fest, welche Dateien und Netzziele ein Prozess erreichen darf; dazu Seccomp, ein Filter für Systemaufrufe)
  verhindert, dass Schadcode in FFmpeg `secret.key` liest **und die Datenbank überschreibt**; ohne Sperre wäre so
  die Übernahme deines Administratorkontos möglich. Ob CT 190 die Sperre kann, misst M-25 vor der Baufreigabe von
  Stufe 0 und 1a.
- **Varianten:**
  - (a) Ja, mit sichtbarem Zustand «Eindämmung nicht verfügbar»: Umwandlung auf jedem Host, das Risiko bleibt dort hoch. Nicht mehr empfohlen.
  - (b) Nur mit Sperre: Ohne Sperre gibt es nur den Direktweg ohne FFmpeg, die Verwaltung sagt es mit der Folge; viele Sender (Zeilensprung, MP2, AC-3, HEVC) zeigen dann im Browser kein Bild.
  - (c) Wie (b), dazu «trotzdem erlauben» als ausdrückliche Wahl des Betreibers (ab Werk aus, mit Bestätigung, Protokolleintrag und dauerndem Hinweis); das wäre die benannte Ausnahme A-13.
- **Empfehlung:** (b); (c) nur, wenn M-25 zeigt, dass CT 190 die Sperre nicht kann, und du das Risiko bewusst trägst.
- **Vorgabe bis zur Antwort:** (b), die sichere Variante. Seccomp ist in jeder Variante Pflicht.
- **Fällig vor:** Stufe 0 (ADR 0046) · **Gewicht:** Weiche

#### E-28-05 · Weiterleitung von `https` nach `http`

- **Herkunft:** `12` F-2; `04` T-28; `05` 2.4 · **Bezug:** LTV-003; T-28
- **Worum es geht:** Leitet ein Anbieter eine verschlüsselte Adresse auf eine unverschlüsselte um, kann jeder
  dazwischen Bild und Ton austauschen, auch für Profile mit Altersgrenze.
- **Varianten:**
  - (a) Immer abbrechen, mit der Meldung «Quelle leitet auf unverschlüsseltes HTTP um»: ein Anbieter, der so umleitet, funktioniert nicht.
  - (b) Nur bei Quellen mit Zugangsdaten abbrechen: mehr Quellen laufen, Quellen ohne Zugangsdaten bleiben verfälschbar.
- **Empfehlung:** (a); eingetragene `http`-Quellen bleiben erlaubt, verboten ist nur der stille Wechsel.
- **Vorgabe bis zur Antwort:** (a), die sichere Variante.
- **Fällig vor:** Stufe 0 · **Gewicht:** Weiche

#### E-28-06 · Darf der Administrator die Zertifikatsprüfung abschalten?

- **Herkunft:** `12` F-3 · **Bezug:** LTV-003 (HTTPS); T-28
- **Worum es geht:** Manche Geräte im Heimnetz und manche Anbieter haben ein selbst ausgestelltes Zertifikat (der
  Ausweis, mit dem ein Server seine Echtheit zeigt). Ohne Prüfung kann sich jemand dazwischen schalten.
- **Varianten:**
  - (a) Nie: solche Quellen gehen nur über `http` oder mit gültigem Zertifikat.
  - (b) Je Quelle, nur für feste Ziele im Heimnetz, mit Warnung und Protokolleintrag (neue benannte Ausnahme).
  - (c) Je Quelle für alle Quellen, mit Warnung: Zugangsdaten zu öffentlichen Anbietern werden abhörbar.
- **Empfehlung:** (a); (b) nur, wenn V-1 zeigt, dass du so eine Quelle wirklich hast.
- **Vorgabe bis zur Antwort:** (a), die sichere Variante.
- **Fällig vor:** Stufe 0 · **Gewicht:** Weiche

#### E-28-07 · Aufnahmen und Altersgrenze: Zeitpunkt und Definition

- **Herkunft:** `04` F-10; `07` F-1; `07` F-3; `review-1-sicherheit` Frage 1 (RS-09); `review-2` RD-05 und RD-18;
  `review-6` RT-07; `02` AU-4, A-6 · **Bezug:** LTV-009, LTV-015; E-3
- **Worum es geht:** Nach E-3 sieht das Hauptprofil die Aufnahmen seiner Profile mit Altersgrenze. Offen sind zwei
  Dinge: was gilt, wenn das Hauptprofil eine Grenze erst später setzt oder wieder entfernt (etwa bei Volljährigkeit),
  und ob die Altersgrenze des **Kontos** mitzählt, die der Administrator setzt. Die PIN-Frage steht in E-28-63.
- **Varianten Teil A (Zeitpunkt):**
  - (a) Nur die Grenze von heute zählt: Setzt das Hauptprofil nachträglich eine Grenze, sieht es auch alle älteren Aufnahmen (benannte Ausnahme).
  - (b) Beides verlangen: Die Aufnahme entstand mit Grenze **und** das Profil hat heute noch eine; wer die Grenze verliert, wird für alle Aufnahmen privat.
  - (c) Nur der Zustand beim Anlegen: Was mit Grenze aufgenommen wurde, bleibt sichtbar, auch nach der Volljährigkeit.
- **Varianten Teil B (Definition «Profil mit Altersgrenze»):**
  - (a) Die wirksame Grenze aus Profil und Konto zählt (kleinerer Wert unter 18), wie heute beim Wunschrecht: In einem Konto mit Altersgrenze gelten alle Profile als begrenzt, auch die erwachsenen; das erweitert A-6 durch den Administrator.
  - (b) Nur die eigene Grenze des Profils zählt.
- **Empfehlung:** Teil A (b) und Teil B (a), weil so «Profile ohne Altersgrenze bleiben ganz privat» ohne Lücke gilt
  und eine einzige Definition für Werkseinstellung, Rechte und Aufnahmen reicht.
- **Vorgabe bis zur Antwort:** Teil A (b), Teil B (a) (`07` 4.3, 4.4; `02` A-6).
- **Fällig vor:** Stufe 0 (Wortlaut der Ausnahme A-6 in ADR 0046) · **Gewicht:** Weiche

#### E-28-08 · Eigenes Release für Stufe 0

- **Herkunft:** `18` O-4 und 3.4 · **Bezug:** LTV-021
- **Worum es geht:** Stufe 0 bringt Schwärzung in den Protokollen, Lint-Regeln und die Compose-Härtung, aber keine
  sichtbare Funktion.
- **Varianten:** (a) Kein eigenes Release, Stufe 0 reist mit Stufe 1a; (b) ein Patch-Release, damit die Härtung
  früher bei allen ankommt.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 0 · **Gewicht:** Feinschliff

#### E-28-59 · Auslegungen des Koordinators bestätigen

- **Herkunft:** `02` 3.1 (AU-1 bis AU-10); `review-4` RG-08, SC-11; `18` 2.1 · **Bezug:** LTV-012, LTV-014, LTV-015,
  LTV-017; Regel 11
- **Worum es geht:** Einige Festlegungen stammen nicht von dir, sondern aus den Leitplanken des Koordinators oder dem
  Bestand des Repos. Sie gelten bis zu deiner Antwort in der Form der Spalte «Vorgabe». Die vier mit eigener
  Tragweite stehen als eigene Fragen; die übrigen kannst du hier gesammelt bestätigen oder ändern.

  | Nr. | Auslegung | Vorgabe bis zur Antwort | Andere Wahl |
  |---|---|---|---|
  | AU-1 | Der Administrator setzt die drei Rechte je Konto als Deckel, das Hauptprofil die seiner Profile | so; «Hauptnutzer» heisst Hauptprofil des Kontos | nur der Administrator; nur der Besitzer |
  | AU-2 | Live-TV zählt zu den bestehenden Grenzen (gleichzeitige Ströme, Bandbreite, Auflösung, Schauzeit) | so; Schauzeit nach dem, was der Server ausliefert | wie Radio nur zur Schauzeit |
  | AU-3 | Quellen im Heimnetz nur vom Administrator, an festen Zielen je Quelle | so; ab Werk je Eintrag eine Adresse mit einem Port | Netze freigebbar; auch Konten tragen Quellen ein |
  | AU-4 | «Kinderprofil» heisst jedes Profil mit wirksamer Altersgrenze unter 18 | so; ob die Grenze des Kontos zählt, ist E-28-07 | eigener Profiltyp |
  | AU-5 | Fire TV | eigene Frage E-28-60 | – |
  | AU-6 | Plattform-Streams | eigene Frage E-28-61 | – |
  | AU-7 | Kein Download von Aufnahmen | eigene Frage E-28-62 | – |
  | AU-8 | Safari, iPhone, iPad | eigene Frage E-28-12 | – |
  | AU-9 | Kein eigener Dienst von Slouchcat im Internet für Live-TV | so; der Auftrag verlangt keinen | – |
  | AU-10 | Kein Verbindungslimit und kein Schalter je Quelle | eigene Frage E-28-02 | – |

- **Varianten:** (a) Alle bestätigen; (b) einzelne ändern (nenne die Nummer und die andere Wahl).
- **Empfehlung:** (a), weil jede Vorgabe dem Wortlaut deines Auftrags folgt.
- **Vorgabe bis zur Antwort:** die Spalte «Vorgabe».
- **Fällig vor:** Stufe 0 (Wortlaut von Regel 11, Spec-Nachtrag und ADR 0046) · **Gewicht:** Weiche

#### E-28-60 · Fire TV · Wortlaut

- **Herkunft:** `02` AU-5; `04` O-29; `03` LTV-014.82; `review-4` RG-08 · **Bezug:** LTV-014 («Berücksichtige: … Fire
  TV»); Regel 11; Konzept 26 E-1
- **Worum es geht:** Dein Auftrag nennt Fire TV unter «Berücksichtige». Das Repo führt Fire TV als Nicht-Ziel (du
  hast in Konzept 26 «Fire TV nicht» gewählt), und der Auftrag lässt bestehende Ausschlüsse stehen, «sofern sie
  nicht ausdrücklich geändert werden». Ob LTV-014 das ändert, kannst nur du sagen.
- **Varianten:**
  - (a) Berücksichtigt: Das Konzept prüft, ob die Android-TV-App auf Fire OS läuft, und zeigt, was dafür nötig wäre; kein eigener Client, keine Zusage.
  - (b) Nicht-Ziel bleibt: Fire TV wird nicht betrachtet.
  - (c) Fire TV wird Zielgerät der TV-App: eigene Prüfung, Gerät nötig, Eintrag im Amazon-Store.
- **Empfehlung:** (a), weil es den Auftrag wörtlich nimmt, ohne einen neuen Client zu bauen.
- **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 0 (Regel 11) · **Gewicht:** Weiche

#### E-28-61 · Plattform-Streams (YouTube, Twitch)

- **Herkunft:** `02` AU-6, A-1; `review-4` RG-08, SC-11 · **Bezug:** LTV-003; Regel 11 («YouTube-Downloads»)
- **Worum es geht:** Der Spec-Nachtrag schliesst Plattform-Streams als Quelle aus; dein Auftrag kennt diesen
  Ausschluss nicht. Technisch sind sie keine IPTV-Quellen im Sinn von LTV-003. Eine **Aufnahme** eines solchen
  Stroms wäre ein Download nach Regel 11.
- **Varianten:**
  - (a) Nicht eigens unterstützt (kein Leser für Plattformseiten), Aufnahmen davon bleiben verboten; nicht als Ausschluss durch dich geführt.
  - (b) Ausdrücklich ausschliessen (so heute im Spec-Nachtrag).
  - (c) Schauen unterstützen, ohne Aufnahme: eigener Leser je Plattform, Pflege bei jeder Änderung der Plattform.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 0 (Spec-Nachtrag, ADR 0046) · **Gewicht:** Feinschliff

#### E-28-62 · Download und Offline von Aufnahmen

- **Herkunft:** `02` AU-7; `18` 10.3; `review-4` RG-08 · **Bezug:** LTV-008, LTV-009
- **Worum es geht:** Aufnahmen auf ein Gerät laden und offline schauen verlangt dein Auftrag nicht; die Leitplanken
  haben es als «nicht in Konzept 28» geführt.
- **Varianten:** (a) Nicht verlangt, nicht geplant (kein Ausschluss durch dich); (b) als spätere eigene Stufe vormerken.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 0 (Wortlaut von ADR 0046) · **Gewicht:** Feinschliff

#### E-28-68 · Übrige FFmpeg-Aufrufe der Bibliothek schützen

- **Herkunft:** `12` O-13, R-14, SM-55; `18` O-8; `review-6` RT-18 · **Bezug:** Auftrag 10.3; T-27
- **Worum es geht:** Die FFmpeg-Aufrufe der Dateiwiedergabe (Filme, Untertitel, Bilder) laufen heute ohne Hülle und
  können `secret.key` lesen, der künftig auch deine Anbieter-Zugänge schützt. Das ist nicht Teil von Konzept 28; die
  Hülle aus Stufe 0 wäre aber wiederverwendbar.
- **Varianten:**
  - (a) Bestand unverändert: nur Live-TV bekommt die Hülle.
  - (b) Jeder FFmpeg-Aufruf bekommt in Stufe 0 eine kleine Umgebung ohne Geheimnisse und keine Speicherabbilder (`RLIMIT_CORE 0`), ohne das Verhalten der Dateiwiedergabe zu ändern.
  - (c) Wie (b), dazu die Hülle mit Dateisperre auch für die Bibliothek, als eigener Schritt nach M-25.
- **Empfehlung:** (b), weil es wenig kostet und das Risiko R-14 senkt.
- **Vorgabe bis zur Antwort:** (a); eine Änderung am Bestand braucht deine Zustimmung.
- **Fällig vor:** Stufe 0 · **Gewicht:** Feinschliff

### A.2 Vor Stufe 1a

#### E-28-09 · Werkseinstellung der Rechte

- **Herkunft:** `03` F-1; `07` F-2; `14` F-1; `15` LA-22; `review-2` RD-05 · **Bezug:** LTV-012; `03` DR-01
- **Worum es geht:** Profile mit Altersgrenze haben Live-TV ab Werk aus (DR-01). Offen ist, was nach der Installation
  oder beim Update für alle anderen gilt und welche Konten bei der Einführung Live-TV sofort sehen.
- **Varianten:**
  - (a) Konten ab Werk aus, Profile ohne Altersgrenze an (sie wirken, sobald das Konto an ist); bei der Einführung sind nur Administrator-Konten an.
  - (b) Alles an ausser Profilen mit Altersgrenze: jedes Konto sieht Live-TV sofort und belastet sofort Leitung und Anbieter.
  - (c) Alles aus, auch Profile ohne Altersgrenze: jedes Profil braucht eine Freigabe.
  - Zusatz «Administrator-Konto»: (i) alle Konten mit der Rolle Administrator; (ii) nur der Besitzer.
- **Empfehlung:** (a) mit (i), für «Timeshift» und «Aufnahmen» dieselbe Werkseinstellung, weil du für die
  Verbindungen deiner Anbieter verantwortlich bist (LTV-017).
- **Vorgabe bis zur Antwort:** (a) mit (i); der Auftrag nennt keine Werkseinstellung.
- **Fällig vor:** Stufe 1a (die Migration setzt die Werte für den Bestand) · **Gewicht:** Weiche

#### E-28-10 · Speicherort des Startpuffers · Wortlaut

- **Herkunft:** `04` F-2; `18` D-3; `03` LTV-019.01 und .04 · **Bezug:** LTV-019 Punkt 1; T-7
- **Worum es geht:** Dein Auftrag verlangt einen eigenen, einstellbaren Speicherort auch für den kurzen technischen
  Startpuffer (die letzten Sekunden eines empfangenen Senders, damit das Bild beim Umschalten sofort steht). Auf
  einer Platte bringt das keinen Nutzen und viel Schreiblast: bei 20 Dauersendern rund 1,7 TB je Tag auf einer SSD.
- **Varianten:**
  - (a) Wählbar, wie im Auftrag: Arbeitsspeicher (ab Werk), RAM-Disk oder Ordner; zwei Bauformen des Rings, die Verwaltung warnt vor Platten; die Ortswahl kommt in Stufe 4.
  - (b) Nur Arbeitsspeicher, einstellbar ist nur die Grösse: eine Bauform, kein Fehlgriff, aber eine **Abweichung** von LTV-019.
- **Empfehlung:** (b), weil der Puffer im Arbeitsspeicher am schnellsten ist und die Ortswahl bei Timeshift und
  Aufnahmen bleibt, wo sie etwas bewirkt.
- **Vorgabe bis zur Antwort:** (a), wortlauttreu.
- **Fällig vor:** Stufe 1a · **Gewicht:** Weiche

#### E-28-11 · Zugangsdaten in der «Sicherung ohne Geheimnisse»

- **Herkunft:** `12` F-4; `review-2` RD-08; `04` 10.5; `07` 6.3 · **Bezug:** LTV-017 (Zugangsdaten sicher); T-28
- **Worum es geht:** Die bestehende Sicherung ohne Geheimnisse löscht verschlüsselte Einstellungen. Offen ist, ob das
  auch für die Zugänge deiner Fernsehquellen gilt.
- **Varianten:**
  - (a) Leeren (Platzhalter `v0:` statt Wert): keine Zugangsdaten in der Sicherung; nach einer Wiederherstellung gibst du die Zugänge neu ein.
  - (b) Verschlüsselt behalten: auf demselben Server laufen die Quellen sofort weiter; wer Sicherung und später den Schlüssel bekommt, liest sie.
- **Empfehlung:** (a), weil Sicherungen oft an weniger geschützten Orten liegen.
- **Vorgabe bis zur Antwort:** (a), die sichere Variante; `04` 10.5 und `07` 6.3 setzen sie schon um.
- **Fällig vor:** Stufe 1a · **Gewicht:** Weiche

#### E-28-12 · Safari, iPhone, iPad: Gerät beschaffen oder auf Zusage verzichten · Wortlaut

- **Herkunft:** `02` AU-8; `17` O-11, O-13, G-4; `04` T-11, 12.1; `05` M-5 und O-28; `18` V-3, D-7, RP-14; `16`
  Abschnitt 10 Nr. 6, 9, 17, 20; `review-4` RG-04 · **Bezug:** LTV-014 (Smartphone, Tablet), LTV-010 («Verhalten
  unterschiedlicher Browser und nativer Clients»); Spec v1 (PWA auf iPhone und iPad); Entscheid 2
- **Worum es geht:** Safari, iPhone und iPad gehören als Web-Clients zum Umfang. Im Haushalt gibt es kein
  Apple-Gerät zum Prüfen (Entscheid 2 sagt «ungeprüft», nicht «nicht zugesagt»). Apples Vorgaben verlangen längere
  Wiedergabelisten, als Slouchcat für schnelles Umschalten liefert; ob es dort gut läuft, weiss niemand.
  **Entschieden ist nichts.**
- **Varianten:**
  - (a) Du besorgst oder leihst vor Stufe 1a ein iPhone oder einen Mac: V-3 läuft, danach folgen vielleicht Anpassungen (längerer Anlauf, bis zu sechs Segmente, `05` O-28), und die Abnahme schliesst Apple ein.
  - (b) Du verzichtest auf die Zusage: Die Clients sind vorgesehen und werden gebaut wie die übrigen Browser, aber nicht geprüft und nicht zugesagt; Hinweis im Release-Text; ein später beschafftes Gerät holt die Prüfung nach.
- **Empfehlung:** (b), bis Apple-Geräte im Haushalt Live-TV nutzen sollen; dann (a).
- **Vorgabe bis zur Antwort:** vorgesehen wie die übrigen Browser, ungeprüft, solange kein Gerät da ist; keine
  Zusage, bis du (a) oder (b) wählst. Weicht von der Spec v1 ab, solange nicht geprüft ist.
- **Fällig vor:** Stufe 1a (Abnahme) · **Gewicht:** Weiche

#### E-28-13 · Schwellen des Benchmarks; Zielwerte hängen an der Quelle · Wortlaut

- **Herkunft:** `17` O-8, G-8 und H-7; `05` 8.5; `04` 14.11; `13` 9; `18` U-10 · **Bezug:** LTV-010 (Zielwerte);
  `AK-L-082` bis `-085`
- **Worum es geht:** Dein Auftrag nennt Umschaltzeiten (unter 1, 3 und 5 Sekunden) als zu prüfende Ziele, keine
  Garantien. Ob sie erreichbar sind, hängt oft an der Quelle: Sendet ein Anbieter nur alle vier Sekunden ein
  vollständiges Bild, ist «noch nicht empfangener Sender unter 3 Sekunden» ohne Vorladen nicht erreichbar. Für die
  Abnahme braucht es feste Schwellen.
- **Varianten:**
  - (a) Median unter dem Zielwert, p95 (der Wert, unter dem 95 von 100 Messungen liegen) höchstens doppelt so hoch, Fehlerquote höchstens 2 %; wird ein Ziel verfehlt, nennt das Protokoll den Grund, und du entscheidest über die Abnahme.
  - (b) Strenger: auch p95 unter dem Zielwert.
  - (c) Nur messen und berichten, keine Schwelle.
- **Empfehlung:** (a), weil sie Ausreisser zulässt, aber eine schlechte Mitte nicht.
- **Vorgabe bis zur Antwort:** (a) als Vorschlag; die Zielwerte bleiben Messgrössen, nie Zusagen. Ein verfehltes Ziel
  ändert keinen Werkszustand von selbst (`17` AK-L-083).
- **Fällig vor:** Stufe 1a (Abnahme) · **Gewicht:** Weiche

#### E-28-14 · Live-TV gegen Filme

- **Herkunft:** `13` P-7 und P-8; `17` O-3; `04` 7.6, 15.2, T-20; `18` U-9, RP-9; `13` M-6 · **Bezug:** LTV-004; Auftrag
  11.3; `AK-L-033`
- **Worum es geht:** Live-Umwandlungen und Filmwiedergabe teilen sich Grafikchip und Prozessor. Offen ist, ob Live
  ungenutzte Einheiten der Filme borgen darf, ob Empfang und Aufnahme ohne Grafikchip vor einer Filmumwandlung
  Vorrang haben und ab wann ein verzögerter Filmstart «unzulässig» ist.
- **Varianten:**
  - (a) Kein Borgen; Empfang und Aufnahme laufen ohne Zurückstufung; Grenze: Filmstart höchstens doppelt so lang wie ohne Live, Filmumwandlung mindestens in Echtzeit.
  - (b) Borgen erlaubt, solange Filme nichts brauchen: mehr Live-Zuschauer, ein Filmstart kann kurz warten.
  - (c) Filme gehen immer vor: neue Live-Umwandlungen werden abgelehnt, solange ein Film umwandelt.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a), wortlauttreu zu «bestehende Wiedergaben möglichst schützen».
- **Fällig vor:** Stufe 1a · **Gewicht:** Weiche

#### E-28-56 · Untertitel bis Stufe E · Wortlaut

- **Herkunft:** `04` F-15, 7.4, O-5, O-22; `18` O-7, 4.12, D-6, D-14; `16` Matrix E und Abschnitt 10 Nr. 16; `17` G-14,
  `AK-L-332`, `-353`; `review-4` RG-05; `review-5` RB-06 · **Bezug:** LTV-004 («individuelle Untertitelauswahl»),
  LTV-014 (Player: Untertitelauswahl), LTV-004.17, LTV-014.52
- **Worum es geht:** Beide Anforderungen gelten ohne Stufe. Nach dem Entwurf gibt es Untertitel in Stufe 1a nur in den
  Apps und nur für Formate, die Media3 liest (DVB-Untertitel, Annahme bis M-24). **Im Browser gibt es bis Stufe E
  keine wählbaren Untertitel**, weil der schlanke Bau von hls.js Untertitel-Spuren weglässt, und **Teletext-Untertitel
  zeigt bis Stufe E kein Client**. Ob deine Sender Teletext oder DVB-Untertitel liefern, misst V-1.
- **Varianten:**
  - (a) Stufung wie beschrieben: Teletext-Untertitel als WebVTT in Stufe E für alle Clients; bis dahin fehlen Untertitel im Browser ganz und Teletext überall; DVB-Untertitel im Browser auch danach nicht.
  - (b) Stufe E vorziehen: Teletext nach WebVTT vor dem Release der Stufe 1a; 1a wird grösser und später fertig.
  - (c) Einbrennen im Browser als Rückfall: Wer im Browser Untertitel wählt, bekommt eine Bild-Umwandlung mit eingebrannten Untertiteln (2 Einheiten); auf der Referenz höchstens zwei solche Zuschauer gleichzeitig; deckt als einzige auch DVB-Untertitel im Browser.
  - (d) Voller Bau von hls.js für Live-TV: bringt im Browser nur Untertitel im Bildstrom und fertige Textspuren, weder DVB noch Teletext; der Player wird grösser.
  - Zusatz für Stufe E: Wie zeigt der Browser WebVTT, ohne den vollen Bau? (i) eigene Textspur im Player neben hls.js (Annahme: möglich); (ii) voller Bau nur für Sitzungen mit Untertiteln.
- **Empfehlung:** (a) mit (i), dazu (b), falls V-1 zeigt, dass deine Sender Untertitel vor allem als Teletext führen.
- **Vorgabe bis zur Antwort:** Untertitel gehören für alle Clients zum Umfang und werden gebaut; Zeitpunkt (a) mit
  (i). **Weicht in der Zeit ab:** LTV-004 und LTV-014 sind im Browser und für Teletext erst ab Stufe E erfüllt.
- **Fällig vor:** Stufe 1a (der Zusatz vor Stufe E) · **Gewicht:** Weiche

#### E-28-66 · Rechte, die der Auftrag «bestehend» nennt, die es nicht gibt

- **Herkunft:** `02` AB-9, AB-10; `review-4` RG-15 · **Bezug:** LTV-012 («bestehende Transcoding-Berechtigungen»,
  «bestehende lokale und externe Zugriffsbeschränkungen»)
- **Worum es geht:** Dein Auftrag nennt zwei Rechte als bestehend, die es im Repo nicht gibt: ein Recht auf Umwandlung
  je Konto und eine Sperre «nur im Heimnetz / auch von aussen». Live-TV kann sie also nicht übernehmen. Du könntest
  annehmen, du könntest Live-TV von aussen sperren oder Umwandlung je Konto verbieten.
- **Varianten:**
  - (a) Keine neuen Rechte: Live-TV nutzt nur die vorhandenen Grenzen (Ströme, Bandbreite, Auflösung, Schauzeit).
  - (b) Neues Recht «Umwandlung» je Konto und Profil (Live und Filme).
  - (c) Neues Recht «nur im Heimnetz» je Konto (Live und Filme).
- **Empfehlung:** (a); (b) und (c) wären neue Funktionen für das ganze Produkt, eigenes Konzept.
- **Vorgabe bis zur Antwort:** (a), wortlauttreu zu «bestehende … übernehmen» (nichts übernehmen, was es nicht gibt).
- **Fällig vor:** Stufe 1a · **Gewicht:** Weiche

#### E-28-15 · Verwaltungsbereich

- **Herkunft:** `15` F-13 · **Bezug:** Auftrag Abschnitt 9
- **Varianten:** (A) eigener Bereich «Live-TV» mit acht Reitern (so das Mockup); (B) eine Seite unter «Medien» mit
  Reitern wie beim Radio; (C) eigener Bereich, erst sichtbar, wenn eine Quelle besteht; vorher nur «Live-TV
  einrichten» in der Übersicht.
- **Empfehlung:** (C), Server ohne Live-TV zeigen keinen leeren Bereich. **Vorgabe bis zur Antwort:** (C).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-16 · Eintrag in der Web-Navigation

- **Herkunft:** `14` F-12 · **Bezug:** LTV-014; `03` DR-23
- **Varianten:** (A) ein Eintrag im Band «Meine Medien» wie Webradio (so das Mockup); (B) zusätzlich oben neben Start
  und Suche.
- **Empfehlung:** (A). **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-17 · Sortierung der Sender

- **Herkunft:** `15` F-15 · **Bezug:** LTV-006
- **Varianten:** (A) nach Kanalnummer, Sender ohne Nummer alphabetisch danach; (B) zusätzlich eine eigene Reihenfolge
  je Gruppe.
- **Empfehlung:** (A), eine Wahrheit für Zifferneingabe und Zappen. **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-18 · Kategorien und Genres

- **Herkunft:** `15` F-16; `09` O-8 · **Bezug:** LTV-005 (Kategorien, Genres), LTV-006
- **Worum es geht:** Auslegung: «Kategorien» sind Kategorien der Sender, «Genres» kommen aus den Programmdaten so, wie
  der Anbieter sie liefert; eine eigene Genre-Liste gibt es nicht.
- **Varianten:** (a) feste, übersetzte Liste von Senderkategorien, Genres wie geliefert; (b) Senderkategorien frei vom
  Administrator anlegbar.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-19 · CPU-, RAM- und GPU-Werte in der Verwaltung

- **Herkunft:** `15` F-22; `17` G-10 · **Bezug:** Auftrag 9.1
- **Varianten:** (A) anzeigen, wo das System sie hergibt, sonst «–»; (B) weglassen und auf «Server» verweisen (eine
  Abweichung von Auftrag 9.1).
- **Empfehlung:** (A). **Vorgabe bis zur Antwort:** (A), wortlauttreu.
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-20 · Geräteprüfungen und Kamera-Gegenprobe

- **Herkunft:** `17` O-10 und O-13 · **Bezug:** LTV-010 (Benchmark); `AK-L-296`, `-297`
- **Varianten:** (a) du führst die Prüfungen am Pixel und TV-Gerät nach einer Vorlage mit Protokoll durch; einmal je
  Client filmt ein Handy in Zeitlupe (mindestens 120 Bilder je Sekunde) als Gegenprobe der Messhaken; (b) nur die
  Messhaken im Client, ohne Gegenprobe.
- **Empfehlung:** (a), einmal zur Abnahme von Stufe 1a. **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 1a (Abnahme) · **Gewicht:** Feinschliff

#### E-28-21 · Blender-Filme im Benchmark

- **Herkunft:** `17` O-2 und H-9 · **Bezug:** Auftrag 15.2 («selbst erzeugt», «rechtmässig verwendbar»)
- **Varianten:** (a) nein, Testbild reicht; (b) ja, nur lokal und nach Lizenzprüfung.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-64 · Strengerer Adressfilter auch für Radio und Podcasts

- **Herkunft:** `04` 10.9 (Adressfilter); `12` 4.x; `18` 2.1 und 9; `review-4` RG-09, SC-6 · **Bezug:** Auftrag
  Z. 1509 (bestehende Funktionen); Auftrag 10.1
- **Worum es geht:** Der Netzwächter sperrt künftig weitere Adressbereiche, die nie öffentlich sind. Das wirkt auch für
  Webradio und Podcasts und kann einen bisher spielenden Sender, der über so eine Adresse läuft, verstummen lassen.
- **Varianten:** (a) für alle, mit Hinweis im Release-Text; (b) nur für Live-TV, Radio und Podcasts bleiben unverändert.
- **Empfehlung:** (a), sicherheitlich richtig und selten spürbar.
- **Vorgabe bis zur Antwort:** (a), die sichere Variante.
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-65 · Live-TV beendet Radio und Hörbuch

- **Herkunft:** `14` LS-25; `17` `AK-L-217`, `-284`; `18` 2.1; `review-4` RG-09, SC-7 · **Bezug:** Auftrag Z. 1509
- **Worum es geht:** Schaltest du Live-TV ein, während Webradio oder ein Hörbuch läuft, endet der Ton (ein Ton zur
  selben Zeit).
- **Varianten:** (a) so, wie heute beim Wechsel zwischen Radio und Hörbuch; (b) Radio und Hörbuch laufen weiter, Live-TV
  startet stumm.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-67 · Einstellungen, die der Auftrag nicht verlangt

- **Herkunft:** `review-4` RG-14, SC-4, SC-10; `07` 3.1; `04` 14 · **Bezug:** Auftrag Abschnitt 5 im Sinn (kein
  Overengineering)
- **Worum es geht:** Rund ein Dutzend Werte (etwa Karenz, Länge des Rings, Byte-Budget, Anteil der Umwandlung,
  Höchstzahl der Sitzungen, Obergrenzen des Vorladens, längste Sofortaufnahme) sind im Entwurf einstellbar, obwohl der
  Auftrag sie nicht als Einstellung verlangt. Jede Einstellung kostet Oberfläche, Tests und Verträglichkeit.
- **Varianten:** (a) einstellbar wie im Entwurf (`07` 3.1); (b) feste Werte aus `04` Abschnitt 14, einstellbar nur, was
  der Auftrag verlangt (Strategie, Zeitpläne, Timeshift-Dauer, Vor- und Nachlauf, Fristen, Speicherorte, Limits,
  Reserve, Intervalle, EPG-Zeitraum).
- **Empfehlung:** (b), weil es die Verwaltung schlank hält; ein Wert lässt sich später einstellbar machen.
- **Vorgabe bis zur Antwort:** (a), wie der Entwurf es beschreibt (keine verbindliche Anforderung berührt).
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

#### E-28-69 · Verhalten des Anbieters über einen Neustart merken

- **Herkunft:** `04` O-32, 5.3 · **Bezug:** LTV-017
- **Worum es geht:** Ob eine Quelle eine zweite Verbindung ablehnt oder die ältere trennt (Muster A und B), lernt der
  Server im Betrieb und vergisst es bei jedem Neustart; danach stockt bei Muster B ein Bild einmal kurz.
- **Varianten:** (a) nicht merken, nur im Arbeitsspeicher; (b) je Quelle speichern: kein Stocken nach Neustart, aber
  nahe an einem verwalteten Limit je Anbieter.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a), wortlauttreu zu LTV-017.
- **Fällig vor:** Stufe 1a · **Gewicht:** Feinschliff

### A.3 Vor Stufe 2

#### E-28-22 · Kanäle für Erinnerungen und Aufnahme-Meldungen

- **Herkunft:** `03` F-3; `09` F-3; `14` F-6; `07` W-21; `03` DR-05 · **Bezug:** LTV-005 (Erinnerungen), LTV-009; `02` AB-15
- **Worum es geht:** Benachrichtigungskanäle (E-Mail, Web-Push, Telegram, Push an die Handy-App) sind je Konto
  eingestellt, nicht je Profil. Geht die Erinnerung eines Profils auch dorthin, sehen andere Profile mit derselben
  E-Mail, dass eine kam.
- **Varianten:**
  - (a) Glocke des Profils und die Kanäle des Kontos mit dem Profilnamen davor; je Ereignisart ein Schalter; externe Meldungen ohne Titel, ausser das Profil wählt es.
  - (b) Nur die Glocke: ganz privat, aber niemand wird erreicht, der die App nicht offen hat.
  - (c) Eigene Kanaleinstellungen je Profil: ändert das Benachrichtigungsmodell für alle Meldungen.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a), nutzt das bestehende Benachrichtigungssystem (Auftrag Z. 794).
- **Fällig vor:** Stufe 2 · **Gewicht:** Weiche

#### E-28-23 · Programmsuche in der allgemeinen Suche

- **Herkunft:** `03` F-5; `14` F-3; `02` AB-14, A-2 · **Bezug:** LTV-005.15, LTV-015
- **Worum es geht:** Die Programmsuche kommt sicher in den Live-TV-Bereich. Die allgemeine Suche ist für Titel
  altersgefiltert; Sendungen sind es nicht.
- **Varianten:** (a) nur im Live-TV-Bereich; (b) zusätzlich ein Abschnitt «Im Fernsehen» in der allgemeinen Suche, nur
  bei wirksamem Recht.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a); die senderübergreifende Programmsuche ist damit erfüllt.
- **Fällig vor:** Stufe 2 · **Gewicht:** Weiche

#### E-28-24 · Programmdaten in eigener Datei, nicht in der Sicherung

- **Herkunft:** `09` F-1; `04` O-8; `09` O-12; `18` RP-8, U-13; `review-4` SC-5 · **Bezug:** LTV-005; Auftrag 11.2
- **Worum es geht:** Programmdaten können ein Vielfaches der heutigen Datenbank ausmachen (5'000 Sender × 7 Tage ≈
  600 MB). Heute wird die eine Datei vor jedem Update gesichert und in jede Sicherung mitgenommen.
- **Varianten:** (a) eigene Datei, nicht gesichert: kleine Sicherungen und Updates, nach einer Wiederherstellung fehlen
  die Programmdaten einige Minuten; (b) alles in einer Datei: einfacher, aber grosse Sicherungen und ein Import, der
  sich die Schreibverbindung mit der Wiedergabe teilt.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 2 · **Gewicht:** Weiche

#### E-28-25 · EPG-Adressen im Heimnetz

- **Herkunft:** `09` F-4; `12` O-9; `review-2` RD-30 (S-12) · **Bezug:** LTV-005; Ausnahme A-3
- **Worum es geht:** Wer Programmdaten selbst erzeugt (etwa auf dem NAS), stellt sie oft im Heimnetz bereit. Die
  Ausnahme für das Heimnetz gilt bisher nur für Senderquellen.
- **Varianten:** (a) ja, mit festen Zielen je EPG-Quelle wie bei Senderquellen (Einzeladresse mit Port, nie Loopback
  oder der Server selbst), gebaut in Stufe H; (b) nein, selbst erzeugte Programmdaten nur per Upload oder über eine
  öffentliche Adresse.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (b), die sichere Variante; der Auftrag verlangt kein EPG im Heimnetz.
- **Fällig vor:** Stufe 2 (Schema der EPG-Quellen); gebaut in Stufe H · **Gewicht:** Weiche

#### E-28-26 · Namensgleiche Sender automatisch zuordnen

- **Herkunft:** `09` F-2 · **Bezug:** LTV-005 (automatische Senderzuordnung)
- **Worum es geht:** Die automatische Zuordnung über die Kennung `tvg-id` gibt es in jedem Fall. Fehlt sie, bleibt der
  Name, und ein eindeutiger Name kann trotzdem falsch sein.
- **Varianten:** (a) nur Vorschlag mit Sammelbestätigung; (b) automatisch, markiert «automatisch (Name)».
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 2 · **Gewicht:** Feinschliff

#### E-28-27 · Sendungsbilder im Raster

- **Herkunft:** `14` F-11 · **Bezug:** LTV-014 (Programmzeitschrift)
- **Varianten:** (A) im Raster nie Bilder, nur Text; (B) kleine Bilder in breiten Zellen.
- **Empfehlung:** (A). **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 2 · **Gewicht:** Feinschliff

#### E-28-28 · Altersangaben der Programmdaten

- **Herkunft:** `14` F-8 · **Bezug:** LTV-015
- **Varianten:** (A) nie anzeigen, weil die Anzeige Schutz vortäuschen könnte; (B) als reine Information auf der
  Detailseite mit dem Satz «Angabe des Senders, Slouchcat prüft sie nicht».
- **Empfehlung:** (A). **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 2 · **Gewicht:** Feinschliff

### A.4 Vor Stufe 3

#### E-28-29 · Werkszustand des globalen Vorladens

- **Herkunft:** `04` F-9; `03` DR-29; `13` P-5; `06` Abschnitt 6; `17` AK-L-083; `review-4` RG-03, RG-11 · **Bezug:**
  LTV-002, LTV-010
- **Worum es geht:** Vorladen (Nachbarsender vorsorglich empfangen) und «automatisch» (häufig genutzte Sender
  bereithalten) sind verbindlich und werden gebaut. Der Auftrag sagt nicht, was nach der Installation an ist; jeder
  vorsorgliche Empfang belegt eine Verbindung zum Anbieter. Ob es dazu einen Schalter je Quelle gibt, fragt E-28-02.
- **Varianten:**
  - (a) Alles aus, bis der Administrator es einschaltet: sicher bei knappen Zugängen; schnelles Umschalten erst nach bewusstem Einschalten.
  - (b) An, aber nur für Quellen, die der Administrator dafür vorsieht: setzt E-28-02 (b) voraus.
  - (c) Vorladen an, mit vorsichtigen Obergrenzen (1 Sender davor und 1 danach, höchstens 4 insgesamt; «automatisch» mit 3 Sendern für Sender mit dieser Strategie); die Verwaltung rät bei einem Zugang mit einer Verbindung, das Vorladen global auszuschalten.
- **Empfehlung:** (c), solange E-28-02 bei (a) bleibt; wählst du E-28-02 (b), dann (b).
- **Vorgabe bis zur Antwort:** (c); LTV-010 verlangt das Vorladen, und nichts im Auftrag schaltet es ab Werk ab. Ein
  Messergebnis ändert den Werkszustand nicht von selbst, sondern legt dir diese Frage erneut vor.
- **Fällig vor:** Stufe 3 · **Gewicht:** Weiche

#### E-28-30 · Dauerempfang gegen neuen Zuschauer · Wortlaut

- **Herkunft:** `04` F-4, T-17, 8.2; `13` P-1; `06` 2.1, 3.14; `review-4` RG-02 · **Bezug:** LTV-002 (permanenter Empfang
  «unabhängig davon, ob jemand zuschaut»), Auftrag 11.3
- **Worum es geht:** Die Obergrenze der Eingangsströme ist erreicht, weil Dauersender laufen, die gerade niemand
  schaut. Jemand will einen anderen Sender sehen, oder eine Aufnahme auf einem anderen Sender beginnt.
- **Varianten:**
  - (a) Der Abnehmer geht vor: Ein Dauersender ohne Abnehmer wird «zurückgestellt» und kommt wieder, sobald Platz ist; Zuschauer und Aufnahmen bekommen fast immer Platz, aber der Dauerempfang ist nicht mehr lückenlos. Eine **Abweichung** von LTV-002.
  - (b) Dauerhaft bleibt dauerhaft: Ein laufender Dauersender weicht niemandem; der Zuschauer bekommt «Der Server ist ausgelastet», die Aufnahme scheitert mit diesem Grund. Zuerst weichen Vorgeladenes, «automatisch» gehaltene Ströme und Ströme in Karenz. Wer viele Dauersender einstellt, setzt die Obergrenze entsprechend; die Verwaltung warnt.
  - In beiden gleich: «Zurückgestellt» gibt es für einen Dauersender, der gar nicht **starten** kann (etwa beim Hochfahren), mit Meldung an den Administrator. An einer Quelle mit einer Verbindung gilt «wer hat, behält».
- **Empfehlung:** (a), weil ein Mensch vor dem Bildschirm wichtiger ist als ein Puffer, den niemand nutzt.
- **Vorgabe bis zur Antwort:** (b), wortlauttreu zu LTV-002; so steht es jetzt in `04`, `05`, `06` und `13`.
- **Fällig vor:** Stufe 3 · **Gewicht:** Weiche

#### E-28-31 · Erkennung eingefrorener Bilder · Wortlaut

- **Herkunft:** `04` F-14; `18` D-5, U-14; `17` G-1, H-14 · **Bezug:** LTV-011 (eingefrorene Streams), LTV-011.04
- **Worum es geht:** Steht der Transport, erkennt der Server das am Strom. Sendet die Quelle weiter gültige Daten mit
  immer demselben Bild (oder Schwarz, oder Stille), ist das ohne Dekodieren nicht sicher erkennbar; dafür braucht es
  eine Stichprobe mit FFmpeg.
- **Varianten:**
  - (a) Zwei Schritte: Stufe 3 erkennt den Stillstand des Transports und zeigt beim Standbild nur einen Verdacht; die Stichprobe kommt in Stufe E nach M-20; bis dahin wechselt ein Sender mit Standbild nicht von selbst auf die Ersatzquelle.
  - (b) Stichprobe schon in Stufe 3: LTV-011 ist mit Stufe 3 erfüllt, die Stufe wird grösser.
  - (c) Nur der Stillstand des Transports, dauerhaft: ein Abstrich an LTV-011.
- **Empfehlung:** (a), weil Filter und Kosten zuerst gemessen werden sollen.
- **Vorgabe bis zur Antwort:** (a). **Weicht in der Zeit ab:** LTV-011 ist in diesem Punkt erst ab Stufe E erfüllt;
  wortlauttreu ab Stufe 3 wäre (b).
- **Fällig vor:** Stufe 3 · **Gewicht:** Weiche

#### E-28-32 · Welche Ausfälle der Administrator gemeldet bekommt

- **Herkunft:** `15` F-19; `04` 10.2 · **Bezug:** LTV-011 (Benachrichtigung bei längeren Ausfällen, keine Meldungsflut)
- **Varianten:** (A) nur Dauer- und Zeitplansender und Sender einer laufenden oder in 30 Minuten beginnenden Aufnahme,
  nach 10 Minuten Störung; (B) zusätzlich Sender, die gerade jemand schaut; (C) jeder aktive Sender, der ausfällt.
- **Empfehlung:** (A). **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 3 · **Gewicht:** Feinschliff

#### E-28-33 · Angabe der Anschlussbandbreite

- **Herkunft:** `13` O-4 · **Bezug:** Auftrag 11.3 (Bandbreitenmangel)
- **Varianten:** (a) keine Angabe, Schutz nur über die Obergrenze der Eingangsströme; (b) neue Einstellung «Anschluss hat
  X Mbit/s»: Vorladen und «automatisch» werden vor der Sättigung abgebaut.
- **Empfehlung:** (a) für jetzt; nach V-1 und M-17 neu prüfen. **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 3 · **Gewicht:** Feinschliff

### A.5 Vor Stufe H

#### E-28-34 · Multicast: Netzmodus «host» oder eigenes Netz · Wortlaut

- **Herkunft:** `04` F-11; `review-1-sicherheit` Frage 3 (RS-13); `02` A-9; `18` U-8, V-6; `12` R-5 · **Bezug:** LTV-003
  (UDP, RTP, Multicast)
- **Worum es geht:** Multicast (ein Strom an viele Empfänger im Heimnetz) kommt im normalen Netz des Containers
  vermutlich nicht an (V-6 misst es). Es gibt zwei Auswege, und vielleicht geht auf deinem Aufbau keiner.
- **Varianten:**
  - (a) Netzmodus «host» als benannte Ausnahme: einfach, aber jeder Dienst auf dem Host liegt hinter jeder Lücke der Abwehr.
  - (b) Eigenes Netz des Containers (`macvlan` oder `ipvlan`): kleinerer Schaden, mehr Einrichtung.
  - Geht keins: LTV-003.16 ist auf deinem Aufbau nicht erfüllbar; dann entscheidest du, ob Stufe H ohne Multicast kommt.
- **Empfehlung:** (b), wenn V-6 es bestätigt, sonst (a). **Vorgabe bis zur Antwort:** dieselbe; ohne deine Wahl gibt
  es keine Zusatzdatei.
- **Fällig vor:** Stufe H · **Gewicht:** Weiche

### A.6 Vor Stufe 4

#### E-28-35 · Rückspulgrenze

- **Herkunft:** `04` F-7; `06` 3.2, 3.6; `review-1-architektur` RA-03 · **Bezug:** LTV-007 («nicht vor dem Einschalten»); T-7
- **Worum es geht:** Damit das Bild sofort steht, beginnt jede Wiedergabe mit einem kurzen Anlauf aus dem Startpuffer,
  also mit Bildern von bis zu zwei Bildgruppen vor dem Tastendruck (meist bis 4 Sekunden; begrenzt durch die Länge
  des Rings, 15 Sekunden, bei Sendern mit langen Bildgruppen höchstens 30).
- **Varianten:** (a) Grenze am Tastendruck: Der Anlauf wird einmal gezeigt und ist danach nicht mehr abrufbar;
  (b) Grenze am ersten gezeigten Bild: einfacher, aber bis zu einer Ringlänge vor dem Tastendruck ist rückspulbar.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a), wortlauttreu.
- **Fällig vor:** Stufe 4 · **Gewicht:** Weiche

#### E-28-36 · Speicherform des Timeshift-Puffers

- **Herkunft:** `04` F-8; `13` P-4; `06` 3.3; `review-1-architektur` RA-09; `18` U-5; `24` N-19 · **Bezug:** LTV-007,
  Ausschluss «gemeinsame Timeshift-Puffer»; T-10
- **Worum es geht:** Schauen 20 Personen mit Timeshift denselben Sender, schreibt die eigene Kopie je Sitzung
  zwanzigmal dasselbe (72 GB je Stunde).
- **Varianten:** (a) eigene Kopie je Sitzung; (b) einmal geschriebene Stücke je Sender, je Sitzung eine Liste der
  eigenen Stücke (in der Speicherform ein gemeinsamer Vorrat; ob der Ausschluss das meint, kannst nur du sagen);
  (c) eigene Kopie und eine Obergrenze gleichzeitiger Sitzungen mit Timeshift.
- **Empfehlung:** (a); (c) als Bremse, falls V-7 es verlangt. **Vorgabe bis zur Antwort:** (a), wortlauttreu.
- **Fällig vor:** Stufe 4 · **Gewicht:** Weiche

#### E-28-37 · Sofortaufnahme im Zeitversatz

- **Herkunft:** `10` F-1 · **Bezug:** LTV-008.01; T-16
- **Varianten:** (a) ab jetzt (live), Knopf «Ab jetzt aufnehmen (live)»; (b) ab der eigenen Stelle aus dem eigenen
  Puffer; (c) mit dem ganzen eigenen Puffer ab dem Einschalten.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 4 · **Gewicht:** Feinschliff

#### E-28-38 · Rückfrage beim Senderwechsel aus dem Zeitversatz

- **Herkunft:** `14` F-9 · **Bezug:** LTV-007 (Senderwechsel löscht den Puffer)
- **Varianten:** (A) immer fragen; (B) erst ab 60 Sekunden Abstand zu Live oder bei Pause, mit «Nicht mehr fragen»;
  (C) nie fragen, nur danach «Zeitversatz verworfen».
- **Empfehlung:** (B). **Vorgabe bis zur Antwort:** (B).
- **Fällig vor:** Stufe 4 · **Gewicht:** Feinschliff

### A.7 Vor Stufe 5

#### E-28-39 · Vorgabe des Aufnahme-Kontingents

- **Herkunft:** `03` F-2; `15` F-20; `13` H-6 · **Bezug:** LTV-008.12, LTV-019; E-4
- **Worum es geht:** Jedes Konto bekommt ab Werk Speicherplatz für Aufnahmen; der Administrator ändert ihn je Konto.
  Eine Stunde HD braucht bei 8 Mbit/s rund 3,6 GB.
- **Varianten:** (a) 100 GB je Konto (rund 28 Stunden HD); (b) 0 GB bis zur Einstellung; (c) unbegrenzt, nur die
  Speicherreserve begrenzt.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a); der Auftrag verlangt ein Limit je Konto, nennt aber keine Zahl.
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-40 · Beginnende Aufnahme gegen laufende Wiedergaben

- **Herkunft:** `03` F-4; `04` F-5; `13` P-2; `06` 3.8, 3.14, N-29 · **Bezug:** LTV-002 (Schutz aktiver Wiedergaben und
  laufender Aufnahmen), Auftrag 11.3
- **Varianten:**
  - (a) Die Aufnahme reserviert ab 2 Minuten vor Beginn einen Platz; eine laufende Wiedergabe wird nie beendet; reicht es nicht, scheitert die Aufnahme mit Meldung. Ein laufender Dauersender weicht nach E-28-30 (b) ebenfalls nicht.
  - (b) Die Aufnahme verdrängt im Notfall die jüngste Wiedergabe, mit Hinweis.
  - (c) Wer zuerst kommt, mahlt zuerst.
- **Empfehlung:** (a), mit Warnung im Planer. **Vorgabe bis zur Antwort:** (a), wortlauttreu.
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-41 · Entzug des Rechts: Planungen und laufende Aufnahmen

- **Herkunft:** `03` F-7; `04` O-9; `14` F-10; `06` 3.15; `17` G-2 · **Bezug:** LTV-012, LTV-018.13
- **Varianten:** (a) Planungen ruhen und wirken wieder, wenn das Recht zurückkommt; eine laufende Aufnahme endet sofort
  und bleibt als «abgebrochen (Recht entzogen)»; (b) Planungen werden gelöscht; (c) wie (a), aber die laufende
  Aufnahme läuft zu Ende.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a): Entzug wird sofort durchgesetzt, Planungen bleiben erhalten.
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-42 · Hauptprofil löscht, wenn das Konto kein Live-TV mehr hat

- **Herkunft:** `08` F-1; `07` O-7; `08` O-6; `14` F-10; `02` A-12; `03` DR-03 · **Bezug:** LTV-012 (ohne Recht
  unsichtbar), E-3; `12` SM-38
- **Worum es geht:** Schaltet der Administrator Live-TV für ein Konto aus, ist der Bereich für alle Profile
  unsichtbar; die Aufnahmen bleiben und belegen Platz. Der Administrator sieht keine Titel und löscht nichts (E-28-45).
- **Varianten:** (a) ja, als benannte Ausnahme A-12: Das Hauptprofil erreicht weiter nur Liste, Belegung und Löschen,
  nicht das Abspielen; (b) nein: ohne Live-TV ist alles unsichtbar, Platz wird erst frei, wenn das Recht zurückkommt.
- **Empfehlung:** (a), sonst bleiben Aufnahmen, die niemand mehr löschen kann.
- **Vorgabe bis zur Antwort:** (b), wortlauttreu zu «ohne Recht unsichtbar» (`02` A-12, `03` DR-03).
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-43 · Was das Hauptprofil ausser den Aufnahmen sieht

- **Herkunft:** `14` F-4; `review-2` RD-18; `review-4` RG-04; `19` Abschnitt 6 · **Bezug:** E-3; LTV-009 («eigene
  Aufnahmeplanungen»)
- **Worum es geht:** Fest steht «sehen, löschen, abspielen» der Aufnahmen von Profilen mit Altersgrenze. Offen sind
  deren geplante Aufnahmen und ob das Profil erfährt, dass das Hauptprofil etwas gelöscht hat.
- **Varianten:** (A) nur die Aufnahmen, nichts von den Planungen; (B) dazu geplante Termine und Regeln, nur lesend;
  das Profil bekommt eine Meldung, wenn das Hauptprofil eine seiner Aufnahmen löscht; (C) geplante auch abbrechen
  und ändern.
- **Empfehlung:** (B), mit Meldung an das Profil.
- **Vorgabe bis zur Antwort:** (A), wortlauttreu zu E-3 (nennt nur Aufnahmen); auch keine Zahl der geplanten Termine.
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-44 · Spuren, die eine Quelle ausserhalb des TS liefert · Wortlaut

- **Herkunft:** `11` F-1 · **Bezug:** LTV-020 («alle empfangenen verfügbaren Untertitelspuren»; der Auftrag verlangt
  eine Lösung zur Entscheidung), LTV-020.21
- **Worum es geht:** Manche HLS-Quellen liefern Untertitel oder eine Tonspur als eigene Liste neben dem Hauptstrom.
  Der Empfänger liest nur den TS-Strom (das Format der Aufnahmen); diese Spuren fehlen dann in Aufnahme und Live-Bild.
- **Varianten:** (a) nicht aufnehmen, ehrlich anzeigen (Abstrich an LTV-020); (b) als Begleitdatei aufnehmen (WebVTT,
  zweite Tonspur je Teil), verlustfrei, aber ein zweites Format; (c) später mit Weg W, soweit verlustfrei möglich.
- **Empfehlung:** (a) für Stufe 5, und (b), sobald V-1 zeigt, dass deine Quellen solche Spuren haben.
- **Vorgabe bis zur Antwort:** (b), wortlauttreu.
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-45 · Notfall-Löschen durch den Administrator

- **Herkunft:** `15` F-18 · **Bezug:** LTV-009, ADR 0032, E-4
- **Varianten:** (A) der Administrator löscht nie fremde Aufnahmen, er senkt nur Kontingente und benachrichtigt;
  (B) er kann je Konto «älteste Aufnahmen bis X GB löschen», ohne Titel zu sehen.
- **Empfehlung:** (A). **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-63 · Das Hauptprofil kann jede PIN ersetzen

- **Herkunft:** `12` F-5, R-12; `02` A-6; `04` F-10 (Grenze der Frage); `review-6` RT-07 · **Bezug:** LTV-009 («Andere
  Profile desselben Accounts dürfen diese Aufnahmen nicht sehen oder abspielen»)
- **Worum es geht:** Über die API sind Aufnahmen anderer Profile unsichtbar. Das Hauptprofil kann aber heute schon die
  PIN jedes Profils setzen oder entfernen, ohne die alte zu kennen, und danach in das Profil wechseln; das Profil merkt
  es erst beim nächsten Wechsel. Profile ohne PIN öffnet jeder mit Zugang zum Konto. Das ist bestehendes Verhalten.
- **Varianten:**
  - (a) Hinnehmen: A-6 sagt im Wortlaut «Privat gegenüber anderen Profilen des Kontos sind Aufnahmen nur mit PIN; das Hauptprofil kann eine PIN ersetzen» (Restrisiko R-12).
  - (b) Der PIN-Ersatz meldet sich beim Profil: Ersetzt oder entfernt das Hauptprofil die PIN eines Profils ohne Altersgrenze, beendet der Server dessen Sitzungen und legt in dessen Glocke «Deine PIN wurde vom Hauptprofil geändert»; kleine Änderung an der Anmeldung, wirkt für alle Profildaten.
  - (c) Ausserhalb von Konzept 28 ändern, etwa PIN-Ersatz nur mit Bestätigung des Profils.
- **Empfehlung:** (b), die kleinste Änderung, die das Profil nicht im Unklaren lässt.
- **Vorgabe bis zur Antwort:** (a); Konzept 28 ändert das bestehende Verhalten der Anmeldung nicht ohne dein Wort.
  Unabhängig davon nennen externe Meldungen keinen Titel (SM-56).
- **Fällig vor:** Stufe 5 · **Gewicht:** Weiche

#### E-28-46 · Laufende Aufnahme wächst über das Kontingent · Wortlaut

- **Herkunft:** `03` F-9; `17` G-3 · **Bezug:** LTV-008 («Speicherplatzlimits pro Account»), Auftrag 15.1
  («Account-Speicherlimits werden eingehalten»), LTV-019 (Schutz laufender Aufnahmen), E-4
- **Varianten:** (a) die Aufnahme läuft zu Ende; das Konto liegt danach über dem Kontingent, neue werden abgelehnt;
  (b) die Aufnahme stoppt beim Erreichen des Kontingents und gilt als unvollständig.
- **Empfehlung:** (a); das Kontingent wird um höchstens eine Aufnahme überschritten.
- **Vorgabe bis zur Antwort:** (b), wortlauttreu zu «Account-Speicherlimits werden eingehalten».
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-47 · Standard-Aufbewahrungsfrist des Administrators

- **Herkunft:** `03` F-10 · **Bezug:** LTV-019 (automatische Aufbewahrungsfristen), Auftrag 9.6, E-4
- **Varianten:** (a) die Frist des Administrators ist nur ein Vorschlag, den der Planer vorfüllt; sie gilt erst, wenn
  das Profil sie speichert; (b) sie gilt für alle Aufnahmen ohne eigene Frist: widerspricht E-4.
- **Empfehlung:** (a), ab Werk «unbegrenzt». **Vorgabe bis zur Antwort:** (a), wortlauttreu zu E-4.
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-48 · Update, während eine Aufnahme läuft

- **Herkunft:** `04` F-6; `15` LA-27 · **Bezug:** LTV-002 (Schutz laufender Aufnahmen)
- **Varianten:** (a) warnen und bestätigen lassen (laufend oder in den nächsten 15 Minuten); (b) Update sperren, bis keine
  Aufnahme läuft; (c) nichts tun wie heute.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-49 · Höchstzahl gleichzeitiger Aufnahmen auf dem Server

- **Herkunft:** `15` F-21; `13` P-6; `04` O-12; `11` K-3; `review-4` RG-04, SC-2 · **Bezug:** LTV-008 («gleichzeitige
  Aufnahmen»); LTV-017
- **Varianten:** (A) keine Grenze (Aufnahmen kopieren nur); (B) einstellbare Grenze, ab Werk «keine».
- **Empfehlung:** (B), als Schutz der Hardware.
- **Vorgabe bis zur Antwort:** (A), wortlauttreu: keine Einstellung, die der Auftrag nicht verlangt.
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-50 · Gleiche Sendung, zwei Profile

- **Herkunft:** `11` F-2 und Abschnitt 9; `11` M-6 · **Bezug:** LTV-009.12 (Deduplizierung untersuchen)
- **Varianten:** D0 getrennte Dateien; D1 harte Verknüpfungen; D2 Zählung in der Datenbank; D3 Kopie ohne Platzbedarf
  (nur auf manchen Dateisystemen).
- **Empfehlung:** D0; nach M-6 neu fragen. **Vorgabe bis zur Antwort:** D0 (der Auftrag verlangt keine Deduplizierung).
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

#### E-28-51 · Bestehende Aufnahmen beim Wechsel des Speicherorts

- **Herkunft:** `15` F-17; `11` 10.5 · **Bezug:** LTV-019 (kein automatisches Verschieben)
- **Varianten:** (A) alter Ort bleibt lesend eingetragen, bis er leer ist; (B) Wechsel nur, wenn der alte Ort leer ist;
  (C) einmaliges Verschieben auf ausdrücklichen Knopfdruck.
- **Empfehlung:** (A). **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** Stufe 5 · **Gewicht:** Feinschliff

### A.8 Vor Stufe 6

#### E-28-52 · Serienaufnahme ohne zuverlässige Folgenkennung

- **Herkunft:** `03` F-8; `09` 7.2; `18` V-9 · **Bezug:** LTV-008 (bereits aufgenommene Folgen «sofern zuverlässig»)
- **Varianten:** (a) alle passenden Ausstrahlungen aufnehmen, mit Hinweis «Folgen nicht erkennbar»; (b) höchstens eine
  je Tag; (c) Serienaufnahme für solche Sendungen nicht anbieten.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a): der Auftrag verlangt keine Begrenzung je Tag.
- **Fällig vor:** Stufe 6 · **Gewicht:** Weiche

#### E-28-53 · Zusätzliche Felder der Serienregel

- **Herkunft:** `14` F-23; `11` O-6 und 14.1a; `07` O-3, W-10, W-11; `review-4` SC-3 · **Bezug:** LTV-008
- **Varianten:** (A) ohne zusätzliche Felder; (B) mit Zeitfenster und Wochentagen (und «nur Erstausstrahlungen», wo die
  Programmdaten es angeben); (C) wie (B), dazu ein Ende der wiederkehrenden Aufnahme.
- **Empfehlung:** (B), das wirksamste Mittel gegen Wiederholungen. **Vorgabe bis zur Antwort:** (A); die Felder werden
  nicht gebaut.
- **Fällig vor:** Stufe 6 · **Gewicht:** Feinschliff

### A.9 Vor den Stufen L, W und den App-Ausgaben

#### E-28-54 · RTSP und RTMP

- **Herkunft:** `04` F-1; `18` U-7, V-11 · **Bezug:** LTV-003 (RTSP, RTMP); Auftrag 10.3; ADR 0025
- **Varianten:** (a) neue Go-Bibliotheken (`gortsplib`, `gortmplib`, `mediacommon`, MIT), vorher belegt, dass sie nur
  über den geprüften Verbindungsweg verbinden (V-11); (b) FFmpeg öffnet RTSP und RTMP selbst, als zweite Ausnahme von
  ADR 0025; (c) später, bis dahin fehlt dieser Teil von LTV-003.
- **Empfehlung:** (a) in Stufe L. **Vorgabe bis zur Antwort:** (a); erfüllt LTV-003 ohne neue Ausnahme.
- **Fällig vor:** Stufe L · **Gewicht:** Weiche

#### E-28-55 · DASH und HLS mit fMP4, falls Weg W nicht dicht ist · Wortlaut

- **Herkunft:** `04` F-13; `review-1-sicherheit` Frage 2; `18` U-3, U-6, V-10; `12` R-3; `02` A-10 · **Bezug:** LTV-003
  (MPEG-DASH ohne DRM, HLS); ADR 0025; T-3, T-4
- **Worum es geht:** DASH und HLS mit fMP4 liest FFmpeg selbst (Weg W). Das wird zuletzt gebaut und nur, wenn die Sperre
  verfügbar ist und M-10 zeigt, dass FFmpeg keine andere Verbindung öffnet. Darum kommen diese Quellarten spät und nur
  nach Messung.
- **Varianten:** (a) nur mit Sperre; (b) Go liest selbst (eigener Leser, FFmpeg nur zum Umpacken); (c) auch ohne Sperre,
  mit Restrisiko; (d) gar nicht (Abstrich an LTV-003).
- **Empfehlung:** (a), und (b) erst, wenn V-1 zeigt, dass du solche Quellen wirklich hast.
- **Vorgabe bis zur Antwort:** (a); fehlt die Sperre oder scheitert V-10, gilt (b), weil (d) dem Wortlaut widerspricht
  und (c) unsicher ist.
- **Fällig vor:** Stufe W · **Gewicht:** Weiche

#### E-28-57 · Bedienung des TV-Players mit dem Steuerkreuz

- **Herkunft:** `14` F-7; `19` Abschnitt 6 · **Bezug:** LTV-010, LTV-014
- **Varianten:** (A) OK = Bedienleiste, ← = Senderliste, → = Info, ↑/↓ = Sender wechseln (so das TV-Mockup); (B) wie der
  Videoplayer: OK = Pause, ←/→ spulen, Senderwechsel nur über Liste, Kanaltasten und Ziffern.
- **Empfehlung:** (A). **Vorgabe bis zur Antwort:** (A).
- **Fällig vor:** der ersten TV-App-Ausgabe (bei E-28-01 (b) also Stufe 1a) · **Gewicht:** Feinschliff

#### E-28-58 · Abnahme der TV-App-Ausgaben

- **Herkunft:** `18` O-6 und Abschnitt 5 · **Bezug:** LTV-021
- **Varianten:** (a) dieselbe Prüfliste wie die Stufe, am Gerät, dazu Fernbedienung und Zurück; (b) eine verkürzte Prüfliste.
- **Empfehlung:** (a). **Vorgabe bis zur Antwort:** (a).
- **Fällig vor:** der ersten App-Ausgabe · **Gewicht:** Feinschliff

---

## Teil B · Offene technische Entscheidungen

Für die Entwicklung, nicht für dich. Erledigte Punkte stehen nur im Anhang.

| Kennung | Titel | Herkunft | Worum es geht | Hängt ab von | Fällig vor |
|---|---|---|---|---|---|
| OT-28-01 | Form der Fähigkeitsangaben der Geräte | `04` O-1 | additive Felder für `mpeg2video`, `mp2`, `aac_latm`, Zeilensprung, Untertitelformate, «in TS abspielbar» | M-7 | 1a |
| OT-28-02 | «Strom normalisieren» | `04` O-2 | FFmpeg nur über Pipes für Quellen ohne Keyframe-Kennzeichen | M-2 (V-1) | 1a, sonst 3 |
| OT-28-03 | Zweites Segmentformat fMP4 | `04` O-3; `16` Nr. 2, 8; `review-5` 9 | Umpacken je Sitzung für HEVC auf Apple und im Browser; Kosten je Prozess in `13` nachtragen | M-2, M-5, V-2 | nach 1a |
| OT-28-04 | Zeilensprung: doppelte Bildrate, «im Browser durchreichen» | `04` O-4, O-23 | Option und Einstellung nach Bewertung | M-6 | 1a |
| OT-28-05 | Teletext nach WebVTT; Untertitel in Aufnahmen | `04` O-5; `11` O-3 | Umsetzung nach E-28-56 | M-24, E-28-56 | E (bei E-28-56 (b) 1a) |
| OT-28-06 | Voller Bau von hls.js, CEA-608 im schlanken Bau | `04` O-22; `16` Nr. 3; Nachprüfung Nr. 12, 13 | Grösse des Zusatzteils, AC-3/HEVC, Untertitel im Bildstrom | M-2, M-3 | nach V-2 |
| OT-28-07 | Schnittstelle des Umwandlungs-Managers für Live | `04` O-7 | reservieren, umbuchen, freigeben ohne Änderung der Dateiwiedergabe | bestehende Tests | 1a |
| OT-28-08 | Netzmodus «host»: Zusatzdatei und Erkennung | `04` O-10; `12` O-4 | Port, Proxys, HTTPS; was gilt ohne Zusatzdatei | M-16, E-28-34 | H |
| OT-28-09 | Mehrere Programme in einem Multicast-Strom | `04` O-11 | je Sender ein Socket auf derselben Gruppe | M-16 | H |
| OT-28-10 | Zuschnitt der ADR und Zusatz zu Regel 3 | `04` O-13, O-26; `12` O-1; `18` O-3; `02` A-11 | ADR 0046 ff.; Hülle in `internal/transcode` darf sich durch FFmpeg ersetzen | Freigabe | 0 |
| OT-28-11 | Codec-Helfer von `mediacommon` | `04` O-21 | ersetzen sie die eigene Erkennung der Bildparameter? | E-28-54 | L |
| OT-28-12 | Folgeänderungen in den Papieren nachführen | `04` O-24, 15.3; `18` O-1 | Punkte aus Teil E | Teil E | 0 |
| OT-28-13 | Herzschlag im Hintergrund-Tab und bei Bildübertragung | `04` O-25; `16` Nr. 18; `17` G-15; `12` R-9; `review-5` RB-11 | bis zur Messung gilt die 60-Sekunden-Regel ohne Ausnahme | M-3, M-5 | 1a |
| OT-28-14 | Landlock und Seccomp im Go-Code | `04` O-27; `12` O-2, O-3; Nachprüfung Nr. 34, 35; `review-6` RT-02, RT-03 | `x/sys/unix` oder `go-landlock` ohne `BestEffort`; Bindung an den Betriebssystem-Faden; Seccomp als Erlaubnisliste ohne `io_uring` | M-25 (V-4), E-28-04 | 0, 1a |
| OT-28-15 | Auswahl der Variante bei HLS-Quellen mit mehreren Qualitäten | `05` O-14 | Vorgabe höchste Qualität | – | 1a |
| OT-28-16 | RTP mit Sitzungsbeschreibung, Fehlerkorrektur, 204-Byte-TS | `05` O-15 | in Stufe H nicht unterstützt; Bedarf | M-2, M-16 | H |
| OT-28-17 | DRM-Grenze bei `SAMPLE-AES` | `05` O-16; `12` O-6, R-10; Nachprüfung N-13 | Grenze Transportverschlüsselung/DRM belegen | Lesen der Quelle | 1a |
| OT-28-18 | Erste Wiedergabeliste in der Antwort auf «einstellen» | `05` O-17; `08` O-4 (Rest) | spart eine Anfrage | M-1 | 1a |
| OT-28-19 | Segmentlängen in der Startphase, längere Bildgruppen im Betrieb | `05` O-19; `review-5` RB-13 | Zieldauer der Liste bei veränderlichen Segmenten | M-1, M-4, M-21 | 1a |
| OT-28-20 | Ersatz des FFmpeg-Empfängers in Weg W | `05` O-20 | wie oft, wie lange die Lücke | M-12 | W |
| OT-28-21 | Bis zu sechs Segmente für Apple-Player ohne Recht «Timeshift» | `05` O-28; `16` Nr. 6 | erweitert das Rückspulen (LTV-007); nur mit Gerät und dann als Frage an dich | M-5, E-28-12 | nur mit Gerät |
| OT-28-22 | Umschlüsseln bestehender `v1`-Werte nach `v2` | `07` O-1 | Format in `12` 5.1 | – | 1a |
| OT-28-23 | Satzformat des Index je Teil | `07` O-2; `11` O-2 | feste Länge, Version, Prüfsumme | – | 5 |
| OT-28-24 | Grösse von `live_stream_events` | `07` O-4 | reichen 50'000 Zeilen für 7 Tage? | Betrieb | nach 3 |
| OT-28-25 | `live_channel_streams.params` als JSON | `07` O-5 | eigene Spalte, falls nach Codec gefiltert wird | – | 3 |
| OT-28-26 | Dauer der Import-Transaktion | `07` O-8 | 11'000 Senderquellen; über 2 s Zwischentabelle | `07` M-1 | 1a |
| OT-28-27 | Schematest für Indizes der Fremdschlüssel | `07` O-9 | jede Kindspalte mit führendem Index | – | 1a |
| OT-28-28 | Unbekannte Aufzählungswerte in den Apps | `08` O-1 | Ausweichwert in den erzeugten Kotlin-Clients | – | 1a |
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
| OT-28-39 | Bedeutung von `tvg-shift` | `09` O-9 | Stunden, Vorzeichen | `09` M-12 | 2 |
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
| OT-28-60 | Ort der Wegwerf-Werkzeuge der Stufe V | `18` O-5; `review-4` RG-16 | eigener, nie gemergter Zweig; Regel 11 braucht dafür einen Halbsatz (nur auf dein Wort) | – | V |
| OT-28-61 | Umgebung, Wiederverbindung und Zeitlimits von FFmpeg | Nachprüfung Nr. 25, 26 | `http_proxy` leeren; `reconnect` und `rw_timeout` ausdrücklich setzen | – | 1a |
| OT-28-62 | CI-Runner mit Landlock | `17` O-14; `12` O-14 | Trägt der GitHub-Runner Landlock und Seccomp? Sonst eigener Runner oder die Sperre bleibt ein Messlauf auf CT 190 | Versuch | 0 |
| OT-28-63 | Reservierung vor Aufnahmen im Ein-Strom-Betrieb | `04` 5.3; `06` N-9; `review-5` RB-03 | Hält ein Zuschauer die einzige Verbindung einer Quelle, scheitert eine beginnende Aufnahme («wer hat, behält»); wie die Reservierung 2 Minuten vorher dort wirkt | E-28-02, E-28-40, V-13 | 5 |
| OT-28-64 | Untertitel im Browser ohne vollen Bau von hls.js | `18` O-7, 4.12; `16` Nr. 16 | eigene Textspur im Player neben hls.js: belegen, dass es geht | M-3, E-28-56 | E |
| OT-28-65 | `AF_UNIX` und Namensauflösung unter Seccomp | `12` O-15 | kommt FFmpeg ohne `socket()` aus? sonst einzeln benannte Erlaubnis | M-10, M-25 | 1a |
| OT-28-66 | Token im Hostnamen | `12` O-16 | `site` auf die registrierbare Domain kürzen; Liste der öffentlichen Endungen | – | 1a |
| OT-28-67 | Anmeldung an einer Xtream-Schnittstelle prüfen | `04` O-30; `review-5` RB-09 | «Zugang falsch» von «Verbindung abgelehnt» trennen; Verbindungszahlen der Antwort nie lesen | – | 1a |
| OT-28-68 | Speichergrenze der Go-Laufzeit | `04` O-31 | `GOMEMLIMIT`: Wert, Zusammenspiel mit dem Radio und einer Grenze des Betreibers | `13` | 1a |

---

## Teil C · Messungen und Versuche vor dem Bau

Die Versuche der Stufe V sind klein, wegwerfbar und laufen ausserhalb des Produkts (`18` 2.2); Code dafür braucht dein
Wort (OT-28-60). Ohne ihr Ergebnis wird die genannte Stufe nicht freigegeben.

### C.1 Versuche der Stufe V

| Kennung | Was gemessen wird | Trägt Entscheid oder Frage | Muss vorliegen vor | Gerät oder Quellen von dir |
|---|---|---|---|---|
| V-1 (`05` M-2) | deine echten Quellen: Codec, Zeilensprung, Ton, **Verteilung der Bildgruppen-Längen** und Segmentdauern, Bitrate, Schub beim Verbinden, Untertitelart | T-7, T-11, T-12, T-15; E-28-06, -13, -44, -55, -56; OT-28-02, -03, -06 | 1a | **ja: deine IPTV-Zugänge**, je Quelle wenige Minuten, bis 50 Sender, mit deiner Zustimmung |
| V-2 (`05` M-3) | hls.js im schlanken Bau mit TS-Segmenten, Diskontinuitäten, Startpunkt, drei Segmente aus dem Ring bei 2, 6 und 10 s, leere Liste, Untertitel im Bildstrom, Hintergrund-Tab | T-11; E-28-56; OT-28-06, -13, -44, -64 | 1a | nein (Chrome, Firefox, selbst erzeugte Testströme) |
| V-3 (`05` M-5, bedingt) | Safari, iPhone, iPad: Zeit bis Bild bei Listen mit 1, 3, 6 Segmenten | E-28-12; OT-28-03, -21 | keine Baufreigabe; nur bei E-28-12 (a) | **ja: ein Apple-Gerät**; ohne Gerät läuft der Versuch nicht |
| V-4 (`05` M-25) | Sperre für FFmpeg auf CT 190: Landlock-Fassung, Seccomp gegen UDP und rohe Sockets, Umgehungen (`io_uring`, 32-Bit), Abbruch der Hülle, **jeder** Prozess unter Last trägt die Sperre, `no-new-privileges`, Prozessgrenze, QSV mit Adressraumgrenze | T-27; E-28-03, -04, -55; OT-28-14, -62, -65 | **Baufreigabe 0 und 1a**, W | **ja: Zeit auf CT 190** |
| V-5 (`05` M-4) | Media3 mit HLS-TS: Startpuffer 1000/500 ms, Stockungen, Decoder beim Umschalten | E-28-01; Player-Einstellungen | Stufe, in der die Apps Live-TV bekommen (bei E-28-01 (b): 1a) | **ja: Pixel und Android-TV-Gerät** |
| V-6 (`05` M-16) | UDP und Multicast im Container: Bridge, «host», `macvlan`/`ipvlan`, Paketverlust | E-28-34; OT-28-08, -09, -16 | H | **ja: CT 190 und dein Heimnetz** |
| V-7 (`05` M-14, `10` M-1, `11` M-1, M-2) | Schreiblast je Zuschauer und Aufnahme auf SSD, HDD, RAM-Disk, NAS; NAS 30 s weg; `fdatasync` | T-8, T-25; E-28-36 (c); NAS als Speicherort «Aufnahmen» (`18` U-5) | 4 und 5 | **ja: CT 190 und dein NAS** (wird kurz getrennt) |
| V-8 (`13` M-1) | 72 Stunden Dauerempfang mit 20 Testsendern, gemessen am Empfänger der Stufe 1a | Freigabe der Stufe 3 (`18` U-4) | 3 | **ja: 72 h Zeitfenster auf CT 190** |
| V-9 (`09` M-10 bis M-12) | deine echten Programmdaten: Schwankung, fehlende Endzeiten, Folgenkennung, Zeitversatz | E-28-52, -53; Schrumpf-Schwelle; OT-28-35, -39, -40 | 2 (läuft nebenher) | **ja: deine XMLTV-Dateien**, zwei Wochen |
| V-10 (`05` M-10, M-11, M-12 30 h) | Weg W dicht; FFmpeg-Leser für DASH und HLS-fMP4 30 Stunden | T-4; E-28-55; OT-28-20 | W | ja: 30 h auf CT 190 |
| V-11 (`05` M-23) | RTSP- und RTMP-Bibliotheken verbinden nur über den geprüften Weg | E-28-54 | L | nein |
| V-12 (`11` M-7) | Zonendaten im Image, wiederkehrende Aufnahmen über eine Zeitumstellung | RP-19 | 6 | nein |
| **V-13** (`05` M-26) | **Verhalten deiner Anbieter bei einer zweiten Verbindung** auf demselben Zugang: lehnen sie ab (Muster A, mit welchem Code), trennen sie die ältere (Muster B), oder geben sie einen Platz erst verzögert frei? Dazu am Testserver die drei Fälle gegen die Regeln aus `04` 5.3 | T-17; E-28-02, -29, -69; OT-28-63 | **Baufreigabe 1a** | **ja: dein Einverständnis für eine zweite Verbindung je Zugang** |

### C.2 Messungen während des Baus, die eine Entscheidung tragen

| Kennung | Was gemessen wird | Trägt | Muss vorliegen vor | Von dir |
|---|---|---|---|---|
| `05` M-1 | Umschaltzeit je Fall, Client und Netz (Benchmark) | E-28-13; Abnahme | Abnahme 1a, 2, 3 | Abnahme am Gerät (E-28-20) |
| `05` M-6, M-7 | Zeilensprung je Client; Decoder je Gerät | T-15; OT-28-01, -04 | während 1a | Pixel, TV-Gerät |
| `05` M-8, `13` M-5 | Live-Umwandlung: Startzeit, Kosten, Ton-Umwandlung, Speicher je Prozess | Budgetwerte `04` 7.6; E-28-14 | während 1a | Zeit auf CT 190 |
| `13` M-6 | Wirkung auf die Dateiwiedergabe | E-28-14 (Grenze Filmstart) | Abnahme 1a, 3, 5 | Zeit auf CT 190 |
| `05` M-18 | Verhalten hinter deinem Reverse-Proxy | Abnahme | Abnahme 1a | **ja: dein Aufbau** |
| `05` M-22 | Zeitüberlauf nach 26,5 h und 53 h | Regel «FFmpeg-Lauf höchstens 24 h» | während 1a | nein |
| `05` M-13, M-15, M-17 | Ring je Sender; Schwellen der Überwachung; Gewinn durch Vorladen | Vorgaben `04` 6.3; E-28-29, -33 | während 3 | nein |
| `05` M-12 (7 Tage) | dauerhafter Empfang über 7 Tage | Abnahme LTV-002.01 | Abnahme 3 | ja: 7 Tage auf CT 190 |
| `05` M-20 | Filter für Standbild, Schwarzbild, Stille, Kosten | E-28-31 | E (bei E-28-31 (b): 3) | nein |
| `05` M-9, M-19; `10` M-2, M-3 | nahtloses Spulen mit Umwandlung; lange Listen am TV; RAM-Disk im Container | Rückfall `05` 4.5; Freigabe RAM-Disk | während 4 | TV-Gerät |
| `09` M-1 bis M-9 | Parser, Schreibtransaktionen, Wirkung des Imports, Raster, Push-Verzögerung | E-28-24 (`09` M-3); OT-28-29, -37, -41 | während 2 | Geräte für `09` M-8 |
| `11` M-3 bis M-6 | Pünktlichkeit des Planers, Wiedergabe von Aufnahmen, Neuaufbau des Index, doppelte Aufnahmen | E-28-50 (`11` M-6); Abnahme 5 | während und nach 5 | nein |
| `05` M-24 | DVB-Untertitel und Teletext in Media3, Umsetzung nach WebVTT | E-28-56; OT-28-05 | Apps, E (bei E-28-56 (b): 1a) | Pixel, TV-Gerät |
| `07` M-1 | Dauer der einen Import-Transaktion | OT-28-26 | während 1a | nein |

### C.3 Was du beisteuern musst

- **Deine echten IPTV-Quellen** für V-1 (Bildgruppen, Codecs, Bitrate, Untertitelart): ohne sie gibt es keine
  Baufreigabe der Stufe 1a. Ins Repo kommen nur Kennzahlen, nie Adressen, Senderlisten oder Zugangsdaten.
- **Dein Einverständnis für eine zweite Verbindung je Zugang** in V-13: Der Versuch öffnet kurz eine zweite
  Verbindung, um zu sehen, wie dein Anbieter reagiert. Ohne V-13 ist die Regel für Zugänge mit einer Verbindung
  nicht belegt, und Stufe 1a wird nicht freigegeben.
- **Deine echten XMLTV-Dateien** über zwei Wochen für V-9, früh beginnen.
- **Ein Apple-Gerät** (iPhone, iPad oder Mac), nur wenn du bei E-28-12 die Zusage willst.
- **Zeit auf CT 190** in angekündigten Fenstern: V-4 (vor der Baufreigabe von Stufe 0 und 1a), V-6, V-7 (mit kurzem
  Trennen des NAS), V-8 (72 h), `05` M-12 (7 Tage), V-10 (30 h). Updates auf CT 190 vorher abstimmen (`17` R-4). Für
  Netz- und Speicherausfälle braucht das Testskript Rechte für `tc netem`, `umount`, `kill -STOP` (`17` R-11).
- **Pixel und Android-TV-Gerät** für V-5, M-6, M-7, M-19, M-24 und die Geräteprüfungen (E-28-20).
- **Deinen Reverse-Proxy** für M-18 und dein **Heimnetz** für V-6.

---

## Teil D · Technische Risiken

Zusammengeführt aus `04` 12.1, `12` 13.1, `13`, `17` 10.1 und `18` 8.1.

| Risiko in Alltagssprache | Wirkung | Gegenmassnahme | Woran man es früh merkt |
|---|---|---|---|
| Deine Sender passen selten direkt in den Browser (Zeilensprung, MP2, MPEG-2, AC-3, HEVC) | wenige gleichzeitige Zuschauer im Browser, viel Umwandlung | Ton-Umwandlung als billiger Weg; Kapazität neu rechnen (`18` U-1) | V-1 |
| Web-Player oder Safari kommen mit TS-Segmenten schlecht zurecht | langsames Umschalten, Abbrüche | Umstellung auf fMP4 an einer Stelle (T-11, `18` U-2); dann ein Prozess je Browser-Zuschauer (OT-28-03) | V-2, V-3 |
| Apple-Geräte sind nie geprüft | Live-TV auf iPhone und Mac kann schlecht laufen, ohne dass es jemand merkt | Frage E-28-12; Hinweis im Release | erst mit Gerät (V-3) |
| Der Anbieter sendet selten ein vollständiges Bild (lange Bildgruppen) | ein kalter Sender braucht über 3 s; der Ring trägt das Fenster knapp | getrennte Startfristen und längerer Ring (T-7); Länge je Sender zeigen | V-1, M-1 |
| **Anbieter mit nur einer Verbindung** | eine Ablehnung oder, bei Muster B, ein kurzes Stocken des Bildes, bevor der Server es gelernt hat, und nach jedem Neustart wieder; eine Aufnahme kann scheitern, wenn ein Zuschauer die Verbindung hält | erst freigeben, dann verbinden; Ruhen von selbst (30 min, 6 h, 24 h); Ein-Strom-Betrieb; Hinweis, das Vorladen global auszuschalten; E-28-02, E-28-69, OT-28-63 | V-13, Betrieb |
| **Hosts ohne Landlock** | nach der Vorgabe von E-28-04 keine Live-Umwandlung: viele Sender zeigen im Browser kein Bild; Weg W entfällt | Zustand «Eindämmung nicht verfügbar» mit Folge; Variante (c) nur auf dein Wort | V-4 |
| Schadcode in FFmpeg ohne Sperre (nur bei E-28-04 (a) oder (c)) | liest `secret.key`, überschreibt die Datenbank, Übernahme des Administratorkontos (`12` R-1) | Vorgabe: ohne Sperre keine Umwandlung; Seccomp immer | V-4 |
| FFmpeg der Bibliothek ohne Hülle | liest `secret.key` und damit künftig auch die Anbieter-Zugänge (`12` R-14) | E-28-68 | – |
| Weg W lässt sich nicht als dicht belegen | DASH und HLS mit fMP4 fehlen oder brauchen eigenen Code | Stufe W zuletzt; E-28-55 | V-10 |
| Speicher, Goroutinen oder offene Dateien wachsen über Tage | Absturz nach Tagen | 72-h- und 7-Tage-Lauf, 1'000 Umschaltungen | V-8, `05` M-12, `13` M-2 |
| Viele Dauersender brauchen viel Arbeitsspeicher | neue Anfragen werden abgelehnt (Dauersender weichen nach E-28-30 nicht) | Byte-Budget aus dem Arbeitsspeicher; Warnung bei vielen Dauersendern | `05` M-13 |
| Netzspeicher stockt oder fällt weg; viele Timeshift-Puffer schreiben | Lücken in Aufnahmen, im schlimmsten Fall Schreiben auf die falsche Platte | kein stilles Ausweichen (T-25), Rückstau je Schreiber, E-28-36 (c) | V-7 |
| Der EPG-Import blockiert die eine Schreibverbindung | stockende Wiedergabe und Verwaltung | eigene Datei (E-28-24), Stapel | `09` M-2, M-3 |
| Live-TV verdrängt Filme | langsamer Filmstart | Live-Anteil am Budget, E-28-14 | `13` M-6 |
| Zugangsdaten gelangen in Protokolle, Antworten oder Sicherungen | Zugänge offen | Schwärzung in Stufe 0, ganze Adresse verschlüsselt, E-28-11 | Sicherheitstests SM-14 bis SM-20 |
| Ein Gerät im Heimnetz fälscht einen UDP- oder Multicast-Sender | falsches Bild, auch für Kinder und in Aufnahmen | Bindung an eine Schnittstelle, klarer Text, Empfehlung eigenes Netzsegment | bleibt Restrisiko (`12` R-2) |
| Netzmodus «host» öffnet den Server nach innen | Dienste auf dem Host hinter jeder Lücke | eigene Stufe H, Zusatzdatei, E-28-34 | V-6 |
| **Ein deaktivierter Sender läuft für laufende Zuschauer weiter** (`12` R-11) | ein aus Sicherheitsgründen gesperrter Inhalt bleibt bis zu 12 h sichtbar | Befehl «sofort beenden»; Quelle entfernen oder ändern beendet ihre Empfänger innert 5 s (T-29) | Verwaltung |
| **Das Hauptprofil kann jede PIN ersetzen** (`12` R-12) | Aufnahmen erwachsener Profile sind gegenüber dem Hauptprofil nur so privat wie die PIN; eine Konto-Altersgrenze macht alle Profile «begrenzt» | E-28-63, E-28-07; externe Meldungen ohne Titel (SM-56) | – |
| Stufe 1 ist sehr gross | Prüfung und Review leiden | Schnitt 1a und 1b, kleine Planschritte | Bau 1a (`18` U-12) |
| Lange Messläufe blockieren CT 190 | deine eigenen Tests und Updates warten | angekündigte Fenster, Zwischenstand je 24 h | Planung |
| Der Herzschlag fällt im Hintergrund-Tab oder bei Bildübertragung aus | Sitzung endet nach 60 s | messen, dann Ausnahme (OT-28-13) | V-2, M-5 |
| Ältere Apps vertragen neue Werte nicht | leere Einträge in der App | Auslieferung nur an Clients mit Live-TV-Fassung; Schema-Test (OT-28-28) | Bau 1a |
| Ein Bauteil rutscht Richtung Ausschlussliste (Archiv, gemeinsamer Puffer, Anbieterlimit) | Verstoss gegen den Auftrag | Negativtests «gibt es nicht» in jeder Stufe; Schalter je Quelle nur auf dein Wort (E-28-02) | Review je Stufe |
| Die Zeitumstellung verschiebt wiederkehrende Aufnahmen | Aufnahme eine Stunde daneben | Tests mit gestellter Uhr | V-12 |
| Schema-Fehler aus Review 2 (Verklemmung beim Löschen, vertauschte Sender beim Import, Lecks der Privatsphäre) | Hänger, falsche Zuordnung, Titel sichtbar | Arbeitspunkte je Stufe (`18` 4.14) | Tests unter `-race` |
| Der Simulator verhält sich anders als echte Anbieter | grüne Tests, Fehler im Betrieb | V-1, V-13; neue Eigenheit zuerst im Simulator nachstellen | V-1, Betrieb |
| Zeilensprung-Entfernung ist zu teuer | wenige Zuschauer bei solchen Sendern | Filter der GPU, Kosten nach Messung | M-6, M-8 |
| Spulen mit Umwandlung über mehrere FFmpeg-Läufe springt | sichtbare Sprünge | Rückfall «spiele ab Uhrzeit» (`05` 4.5) | M-9 |
| Ein Token einer Wiedergabe-Adresse ist nach dem Abfangen 60 s nutzbar | fremde Person schaut kurz mit | kurze Laufzeit, Bindung an Sitzung | bleibt Restrisiko (`12` R-9) |
| Der Betreiber verletzt die Bedingungen seiner Anbieter | rechtlich, deine Sache | Hinweis in Verwaltung und Anleitung | – |

---

## Teil E · Bekannte Unstimmigkeiten zwischen den Dokumenten

Stand 29. November 2026, in den Dateien geprüft (`07`, `08` und `13` Fassung 3; `09` und `19` Fassung 2). Massgeblich
für Stufen ist `18`; für Vorgaben die Grundregel «wortlauttreu, bei Sicherheit sicher»; wo `12` strenger ist als `04`,
gilt `12`.

### E.1 Offen

| Nr. | Stelle A | Stelle B | Massgeblich | Stand nach der letzten Angleichung |
|---|---|---|---|---|
| U-14 | `14` LD-05 im Mockup: Vorlauf der Erinnerung 2 und 10 Minuten | `03` DR-08: 0, 5, 15, 30 Minuten | `03`; Mockup angleichen | bleibt offen, weil nur das Mockup abweicht; `19` und die Mockups ändert ein anderer Agent |
| U-16 | `18` 4.12 und `16` Nr. 16: WebVTT im Browser in Stufe E | `04` 7.4: kein voller Bau von hls.js | offen, E-28-56 und OT-28-64 | bleibt offen, weil es deine Frage ist (E-28-56) |
| U-19 | `12` SM-39: Sitzung ohne Segmentabruf endet nach 15 Minuten, Stufe **3** | `04` 8.5 und 14 (Wert), `17` `AK-L-268`, `-351`: Stufe **1a** | Stufe 1a (früher ist strenger); `12` nachziehen | erledigt in `12` (SM-39, Abschnitt 9.2, Stufenliste in 11: Sitzung ohne Segmentabruf in 1a) |
| U-20 | `08` F-1: Vorgabe (a) in der Form von A-12 | `02` A-12, `03` DR-03, `11` 7: Vorgabe (b), A-12 gilt nur nach Marcos Wahl | (b), Grundregel; `08` nachziehen | erledigt in `08` (F-1, 3.1, 3.5, O-6), `07` (4.3, O-7) und `14` (Grundsatz 1, 3.14, Abschnitt 13): Vorgabe (b), A-12 nur als Variante (a) |
| U-21 | `11` (Kontingent, Verweis auf `03` F-9): «Vorgabe bis zur Antwort (a): sie läuft zu Ende» | `03` F-9: Vorgabe (b), die Aufnahme stoppt am Kontingent | `03` (b); `11` nachziehen | erledigt in `11` (Kontingent während der Aufnahme: Vorgabe (b)) |
| U-22 | `15` F-21: Vorgabe (B) einstellbare Grenze | `11` 3.4 und K-3, `13` 8.3, `review-4` RG-04: (A) keine Grenze | (A); `15` nachziehen | erledigt in `15` (F-21 Vorgabe (A); die Einstellung in LA-14 nur bei (B)) |
| U-23 | `14` F-4 und `03` DR-02: Vorgabe «geplante Termine nur als Zahl» | `review-4` RG-04, E-3: wortlauttreu ist (A) | (A); `14`, `03` und das Mockup nachziehen | erledigt in `14` (F-4, 3.14, Abschnitt 11 und 13), `03` (DR-02) und `08` (2.4, 3.5: keine Zahl, `scheduled` nur bei (B)); das Mockup bleibt offen, weil es ein anderer Agent nachzieht |
| U-24 | `09` F-4 (a): feste Ziele «höchstens /24 bzw. /64», «höchstens 64 Ports» | `04` 14.9, `12` 4.5, `15` LA-03: Einzelziel ab Werk, Netz höchstens /28 bzw. /124, 16 Ports | `12`; `09` nachziehen | erledigt in `09` (F-4 (a): Einzelziel, Netz höchstens /28 bzw. /124, 16 Ports) |
| U-25 | `19` Abschnitt 6 und das Mockup: Einstellung je Quelle «Vorladen für Sender dieser Quelle», «3sat zurückgestellt» | `04` Fassung 4 (RG-02, RG-03): kein Schalter je Quelle; Dauersender weicht nicht | `04`; `19` und Mockup nachziehen | bleibt offen, weil `19` und die Mockups ein anderer Agent nachzieht |
| U-26 | `18` 2.1: `15` F-14 als offene Frage vor Stufe 1; Tabelle nach «`03` F-6», «`04` F-12» … | Anhang A: `15` F-14 erledigt durch `03` DR-16; Nummern `E-28-nn` | `20`; `18` 2.1 auf `E-28-nn` umstellen (RG-22) | erledigt in `18` (2.1 mit `E-28-nn`; `15` F-14 als erledigt durch `03` DR-16) |
| U-27 | Verweise auf das alte Paar: `18` 4.5 «Vorlade-Schalter je Quelle nur bei Variante (b) von E-28-29»; `03` DR-29, `13` P-3, `18` D-13 und 4.5 «E-28-02/E-28-29» als eine Frage | `20` Fassung 2: der Schalter je Quelle ist E-28-02 (b), der Werkszustand des globalen Vorladens E-28-29 | `20`; Verweise nachziehen | erledigt in `03` (DR-29), `13` (8, P-3), `15` (Grundsatz 3) und `18` (2.1, D-13, 4.5) |
| U-28 | `12` F-5, `02` 3.1, `18` 2.1, `04` O-29, O-32, `12` O-13, `18` O-8: «Nummer vergibt `20`» oder ohne Nummer | `20` Fassung 2: E-28-63, E-28-59 bis -62, E-28-64, -65, E-28-60, E-28-69, E-28-68 | `20`; Verweise eintragen | erledigt in `12` (F-5, O-13, Abschnitt 12.3), `02` (3.1), `18` (2.1, O-8) und `04` (O-29, O-32) |

### E.2 Erledigt seit Fassung 1

U-1 (Stufe H in `04` T-3), U-2 (Härtung in Stufe 0), U-3 (M-4 an die Stufe der Apps), U-4 («Eindämmung nicht
verfügbar»), U-5 (nur die Startregel ist belegt), U-6 (Entzug eines festen Ziels beendet Verbindungen), U-7 (Seccomp
immer), U-8 (Ersatzpfade), U-9 (Gegenrichtung in `04` F-10), U-11 (`live_ref` in `09`), U-12 (Stichprobe in Stufe E),
U-13 (LTV-007 statt LTV-005), U-15 (Sicherung leeren: `04` 10.5 und `07` 6.3 setzen die Vorgabe um), U-18 (keine
Deutung «vermutlich Verbindungslimit»). **Überholt durch RG-03:** U-10 und U-17 (Name und Spalte des Schalters je
Quelle): Der Schalter entfällt in der Vorgabe; `07` 3.2 und `08` 3.7 führen `preload_enabled` und `preloadEnabled`
nur noch als Folge von E-28-02 (b). **Erledigt durch RG-02:** «zurückgestellt» weicht nicht mehr Zuschauern in `04`,
`05`, `06`, `13`. Ebenso: Timeshift 60 Minuten und Deckel 20 % / 50 GB, Update-Warnfenster 15 Minuten, Upload 200 MiB,
Reserve «Aufnahmen», 72 h gegen 7 Tage, Stufen für RTSP, RTMP und DASH, LTV-018 zwischen 1a und 1b, «höchstens eine je
Tag» in `09`, Weg W im Live-Anteil, `04` 8.3, feste Ziele /28 in `04` 14.9 (`12` O-17), Apple «vorgesehen, ungeprüft»
in `04`, `16`, `18`, Untertitel «nicht erfüllt bis E» in `04` 7.4 und `16` (D-14), AK-L-083 ändert keinen Werkszustand
(RG-11), die Merkzeilen in `13` (RG-18).

**Schlussprüfung (Review 7):** Die 14 Reststellen sind behoben (Abnahmekriterien in `17`, Untertitel in `18`, Grundregel in `04-entscheidungen.md`, ADR 0046 und `09`, Ring nach T-7, Zählung 588, Nachprüfung Nr. 32, Vorgabe «5 s» in `12`); keine ist offen.

---

## Anhang · Zuordnung

### A. Fragen der Papiere zu `E-28-nn`

| Quelle | Zu |
|---|---|
| `02` E-1, E-2, E-3, E-4 | entschieden (Abschnitt «Was du schon entschieden hast») |
| `02` AU-1, AU-2, AU-3, AU-4, AU-9 | E-28-59 |
| `02` AU-5 | E-28-60 |
| `02` AU-6 | E-28-61 |
| `02` AU-7 | E-28-62 |
| `02` AU-8 | E-28-12 |
| `02` AU-10 | E-28-02 |
| `02` AB-9, AB-10 | E-28-66 |
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
| `04` F-15 | E-28-56 |
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
| `12` F-5 | E-28-63 |
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
| `13` P-7, P-8 | E-28-14 |
| `13` P-9 | erledigt durch `04` 8.1 |
| `review-1-sicherheit` Frage 1, 2, 3, 4 | E-28-07, E-28-55, E-28-34, E-28-03 |
| `review-1-architektur` RA-03, RA-09, RA-18, W-10 | E-28-35, E-28-36, E-28-29, E-28-02 |
| `review-2` RD-05, RD-18 | E-28-07 |
| `review-3` RM-01 | E-28-02 |
| `review-4` RG-02, RG-03, RG-05, RG-08, RG-09, RG-14, RG-15 | E-28-30; E-28-02, -29; E-28-56; E-28-59 bis -62, -12; E-28-64, -65; E-28-67; E-28-66 |
| `review-6` RT-01, RT-07, RT-18 | E-28-04; E-28-63; E-28-68 |
| `17` O-2, O-3, O-8, O-10, O-11, O-13 | E-28-21, E-28-14, E-28-13, E-28-20, E-28-12, E-28-20 |
| `18` O-4, O-6, O-7, O-8 | E-28-08, E-28-58, E-28-56 (Zusatz), E-28-68 |
| `09` O-8 | E-28-18 |
| `13` O-4 | E-28-33 |
| `04` O-29, O-32 | E-28-60, E-28-69 |
| `12` O-13 | E-28-68 |
| `16` Abschnitt 10 Nr. 16, 20 | E-28-56, E-28-12 |
| Nachprüfung der Quellen | keine Fragen an dich; Folgen in OT-28-06, -14, -17, -33, -61 |

`05`, `06`, `13`, `16`, `17`, `18` und `19` haben keine eigenen `F-n`; `06` und `19` verweisen nur auf `04` und `14`.

### B. Offene Punkte der Papiere zu `OT-28-nn`

| Quelle | Zu |
|---|---|
| `04` O-1, O-2, O-3, O-4 | OT-28-01, -02, -03, -04 |
| `04` O-5 | OT-28-05 |
| `04` O-6 | entfällt (ersetzt durch `04` F-8, E-28-36) |
| `04` O-7 | OT-28-07 |
| `04` O-8, O-9, O-12 | E-28-24, E-28-41, E-28-49 |
| `04` O-10, O-11 | OT-28-08, -09 |
| `04` O-13 | OT-28-10 |
| `04` O-21, O-22, O-23 | OT-28-11, -06, -04 |
| `04` O-24, O-25, O-26, O-27 | OT-28-12, -13, -10, -14 |
| `04` O-29 | E-28-60 |
| `04` O-30, O-31 | OT-28-67, -68 |
| `04` O-32 | E-28-69 |
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
| `12` O-13 | E-28-68 |
| `12` O-14, O-15, O-16 | OT-28-62, -65, -66 |
| `12` O-17 | erledigt durch `04` 14.9 Fassung 4 (Rest in `09`: Teil E U-24) |
| `13` O-1, O-2, O-3, O-7 | erledigt durch `04` 8.1, 7.6 und `07` |
| `13` O-4 | E-28-33 |
| `13` O-5, O-6 | OT-28-53, -54 |
| `16` Abschnitt 10 Nr. 3, 6, 18 | OT-28-06, -21, -13 |
| `16` Abschnitt 10 Nr. 16, 20 | E-28-56 (mit OT-28-64), E-28-12 |
| `16` Abschnitt 10 Nr. 19 | Annahme, Messung `05` M-4 (V-5) |
| `16` Abschnitt 10 übrige Nummern | entschieden in `05` |
| `17` O-1 | OT-28-49 |
| `17` O-2, O-3 | E-28-21, E-28-14 |
| `17` O-4, O-5, O-6, O-7 | OT-28-55, -56, -57, -58 |
| `17` O-8 | E-28-13 |
| `17` O-9 | OT-28-59 |
| `17` O-10, O-11, O-13 | E-28-20, E-28-12, E-28-20 |
| `17` O-12 | OT-28-52 |
| `17` O-14 | OT-28-62 |
| `18` O-1 | OT-28-12 |
| `18` O-2 | erledigt durch `17` Fassung 2 |
| `18` O-3 | OT-28-10 |
| `18` O-4 | E-28-08 |
| `18` O-5 | OT-28-60 |
| `18` O-6, O-7, O-8 | E-28-58, E-28-56 (mit OT-28-64), E-28-68 |
| Nachprüfung Nr. 12, 13 / 25, 26 / 32, 40 / 34, 35 / N-13 | OT-28-06 / -61 / -33 / -14 / -17 |
| `review-5` RB-03, RB-09, RB-11, RB-13 | OT-28-63, -67, -13, -19 |
| `review-6` RT-02, RT-03 | OT-28-14 |

---

## Änderungen gegenüber der vorigen Fassung

Fassung 1 → 2, Grundlage: `review-4-gesamt.md` (RG-01 bis RG-22), `review-5-architektur-zweitpruefung.md`,
`review-6-sicherheit-zweitpruefung.md`, die Entscheide des Koordinators nach Review 4 und 6, `04`/`05` Fassung 4,
`12` Fassung 3 und die übrigen Papiere in ihrer jetzigen Fassung.

- **Lesart (RG-01, RG-06):** Ohne Antwort gilt die wortlauttreue, bei Sicherheit die sichere Vorgabe; die Empfehlung
  erst mit deiner Antwort; «Konzept freigegeben» und «Baufreigabe einer Stufe» getrennt. Kurzübersicht mit eigener
  Spalte «Vorgabe».
- **Entschieden ist nur, was du entschieden hast (RG-08):** E-1 bis E-4 und die 21 Grundsatzentscheide; die Leitplanken
  stehen als Fragen.
- **Neu: «Das Wichtigste zuerst»** mit zwölf Weichen (Review 4 Punkt 9, RG-17).
- **Zusammengeführt (RG-03):** E-28-02 ist jetzt die eine Frage «Quellen mit wenigen Verbindungen» (Vorgabe (a)
  nichts je Quelle; (b) Schalter; (c) Zahl), mit Zweck und Folgen; E-28-29 fragt nur noch den Werkszustand des
  globalen Vorladens (`04` F-9).
- **Vorgaben richtiggestellt:** E-28-04 (b) nur mit Sperre (RT-01); E-28-12 neu gefasst «Gerät beschaffen oder auf
  Zusage verzichten», nichts entschieden (RG-04, RG-08); E-28-29 (c) statt (b); E-28-30 ohne den Hinweis auf andere
  Abläufe (RG-02 erledigt); E-28-31 und E-28-56 «weicht in der Zeit ab»; E-28-43 (A) statt «Zahl»; E-28-46 (b) statt
  (a); E-28-49 (A) statt (B); E-28-42 (b) bestätigt gegen `08`. Alle anderen Vorgaben gegen die Quellstellen
  einzeln geprüft und gleich gelassen.
- **E-28-56 erweitert (RG-05, RB-06, `04` F-15):** Browser ohne wählbare Untertitel bis Stufe E, Teletext in keinem
  Client; vier Varianten, Zusatz zur Anzeige im Browser; fällig jetzt vor Stufe 1a.
- **Neue Fragen E-28-59 bis E-28-69:** Auslegungen bestätigen (AU-1 bis AU-10), Fire TV, Plattform-Streams, kein
  Download von Aufnahmen, PIN-Ersatz (`12` F-5), strengerer Adressfilter und Ton-Vorrang (RG-09), Rechte, die es nicht
  gibt (RG-15), nicht verlangte Einstellungen (RG-14), FFmpeg der Bibliothek (RT-18), Muster A/B merken (`04` O-32).
- **E-28-11:** Satz «nach der Antwort angleichen» gestrichen (RG-20).
- **Teil B:** OT-28-62 bis OT-28-68 neu (Runner mit Landlock, Reservierung im Ein-Strom-Betrieb, Untertitel im Browser
  ohne vollen Bau, `AF_UNIX`, Token im Hostnamen, Xtream-Anmeldung, `GOMEMLIMIT`); OT-28-05 auf Teletext und Aufnahmen
  beschränkt; OT-28-14 um Fadenbindung und Seccomp-Erlaubnisliste ergänzt.
- **Teil C:** V-13 (`05` M-26) als Baufreigabe 1a; V-4 (M-25) als Baufreigabe 0 und 1a; V-1 mit Verteilung der
  Bildgruppen; C.3 mit dem Einverständnis für eine zweite Verbindung je Zugang.
- **Teil D:** Anbieter mit einer Verbindung, Hosts ohne Landlock, `12` R-11, R-12, R-14 neu; Text «vermutlich Limit»
  entfernt (RG-10).
- **Teil E:** in «offen» und «erledigt» geteilt; neu U-19 bis U-28.
- **Zählung:** 64 `F-n` in den Papieren (Fassung 1: 62; neu `04` F-15, `12` F-5), jede mit genau einer E-28-Nummer oder
  als erledigt geführt.

- Letzte Angleichung: Teil E.1 um die Spalte «Stand nach der letzten Angleichung» ergänzt (U-19 bis U-28 erledigt;
  U-14, U-16, U-25 offen).
- Nach der Schlussprüfung (Review 7): Teil E um den Vermerk «14 Reststellen behoben» ergänzt.

*Ende von `20-offene-entscheidungen.md`. Nichts in diesem Dokument ist gebaut; jede Vorgabe gilt, bis du anders
entscheidest.*
