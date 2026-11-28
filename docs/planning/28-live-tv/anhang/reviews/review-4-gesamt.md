# Konzept 28 · Review 4 Gesamtreview

Stand: 29. November 2026 · Reviewer: oh-my-claudecode:critic

> **Urteil: übergabereif nach Korrekturen.** 22 Befunde: **3 kritisch**, **5 hoch**, **9 mittel**, **5 niedrig**.
> Die drei kritischen Befunde haben eine gemeinsame Wurzel: Was gilt, wenn Marco eine offene Frage nicht beantwortet,
> ist in den Papieren zweimal verschieden geregelt, und an zwei Stellen hat die massgebliche Architektur die
> Abweichung vom Wortlaut schon eingebaut, während die Frageliste den Wortlaut als Vorgabe nennt. Gebaut ist nichts;
> kein Dokument nennt etwas «implementiert»; im Diff gegen `origin/main` ist ausserhalb von `docs/` nichts geändert.

## 1. Vorgehen und Grenzen

- **Massstab:** Auftrag im Wortlaut (`anhang/auftrag-2026-11-28.md`, Abschnitte 4 und 6 bis 21), `LEITPLANKEN.md`
  (Koordinator, ausserhalb des Repos), `CLAUDE.md`, Bestand nur für Stichproben.
- **Vollständig gelesen:** `02`, `19`, `21`, `20` (Teil A bis E und Anhang), `18` Abschnitte 1 bis 4.3 und 9 bis 11,
  ADR 0046, der ganze Diff der Projektdokumente (`CLAUDE.md`, Spec, Plan, Vision, Frage 37, `HANDOVER.md`,
  `CURRENT_STATUS.md`), `04` Abschnitt 14 und die Zeilen T-17, T-18, 5.3, F-4, `22` Abschnitte 1, 3 (Auszüge) und 7,
  `review-3-mockups.md` Abschnitte 1, 2 und 7, `nachpruefung-quellen.md` Abschnitte 1, 2 und Zusammenfassung.
- **Gezielt gelesen (Suche und Abschnitt):** `03` (Anforderungszeilen, Abschnitt 6, DR-16/21/23), `05`, `06`, `07`
  (Tabellenliste, `live_config`, `live_sources`), `08` (2.13, Zählung), `09`, `11`, `13`, `14`, `15`, `16` (Matrix E),
  `17` (AK-L), `24`.
- **Stichprobe Anforderungen (Prüfauftrag 1):** alle Zeilen von LTV-004 (38 und 1 Ausschluss), LTV-007 (14 und 1),
  LTV-009 (12), LTV-013 (2 und 6), LTV-015 (14), LTV-017 (3 und 6); dazu LTV-001.15, LTV-002.02 bis .08,
  LTV-003.10 bis .16, .24, .25, LTV-005.17, LTV-006.20, LTV-010.01, .04 bis .07, .11 bis .13, .32 bis .35, LTV-011.04,
  LTV-014.40 bis .81, LTV-018.20 bis .23, LTV-019.01 bis .04 und .10 bis .22, LTV-020.04 bis .06, .20, .21:
  rund 190 Zeilen. Je Zeile geprüft: Anforderung in `03`, vertiefendes Dokument, `AK-L` in `17`, Stufe in `18`.
- **Stichprobe Abnahmekriterien (Prüfauftrag 10):** `AK-L-005` bis `-019`, `-082` bis `-087`, `-092`, `-120` bis
  `-124`, `-160` bis `-166`, `-170` bis `-174`, `-179` bis `-181`, `-285`, `-310` bis `-312`, `-326`, `-327`, `-330`
  bis `-333`: 52 Kriterien.
- **Nicht geprüft:** Mockup-Code (laut Auftrag in Korrekturrunde), Rohberichte A1 bis A4, R1 bis R5, der Bestand über
  einzelne Stellen hinaus, die Sicherheitsfeinheiten von `12` (Review 1 Sicherheit liegt vor).
- **Vorab erwartet (vor dem Lesen notiert):** (1) Abweichungen vom Wortlaut, die in Stufen oder «nach Messung»
  verschwinden; (2) eine LTV-017-Umgehung über einen Schalter je Quelle; (3) veraltete Querverweise nach zwei
  Bereinigungsrunden; (4) Stufen, die Marco als «entschieden» präsentiert werden; (5) Überkomplexität.
  **Gefunden:** alle fünf, dazu als Unerwartetes die widersprüchliche Regel für unbeantwortete Fragen (RG-01).

---

## 2. Befunde

### Kritisch

#### RG-01 · kritisch · Zwei verschiedene Regeln dafür, was ohne Marcos Antwort gilt

- **Stelle:** `20-offene-entscheidungen.md` Z. 14–15 («Bis zu deiner Antwort gilt, was dein Auftrag … wörtlich sagt»;
  «Mit der Freigabe des Konzepts gilt für jede offene Frage die Zeile «Vorgabe» (`18` 1.1)») gegen
  `18-umsetzungsplan.md` Z. 57–59 («Für jede Frage, die du dann noch nicht beantwortet hast, gilt **die Empfehlung des
  Papiers**») und `03-fachkonzept.md` Z. 1416 («Für jede Frage gilt die Empfehlung, solange Marco nichts anderes
  sagt»). `20` beruft sich auf `18` 1.1, das genau das Gegenteil sagt.
- **Problem:** Bei vier Weichen weicht die Empfehlung vom Wortlaut ab, die Vorgabe nicht: E-28-10 (Startpuffer nur
  im Arbeitsspeicher, Abweichung von LTV-019 Punkt 1), E-28-30 (dauerhafter Empfang weicht Zuschauern, Abweichung
  von LTV-002 «unabhängig davon, ob jemand zuschaut»), E-28-42 (Hauptprofil erreicht Aufnahmen trotz entzogenem
  Live-TV), E-28-44 (Spuren ausserhalb des TS werden nicht aufgenommen, Abstrich an LTV-020). Gibt Marco das Konzept
  frei oder sagt er, wie bei Konzept 26 und 27, «für jede offene Frage gilt die Empfehlung» (Frage 34/35,
  `04-entscheidungen.md` Z. 139–140), dann gelten nach `18` und `03` vier Abweichungen von verbindlichen Anforderungen,
  ohne dass er eine davon gewählt hat. Das ist genau die stillschweigende Änderung, die Auftrag Abschnitt 21 verbietet.
- **Beleg:** Auftrag Z. 2043 («keine beschlossenen Anforderungen eigenmächtig ändern»), Z. 2045 («zur Entscheidung
  bereitstellen»).
- **Korrektur:** Eine Regel, an einer Stelle: «Ohne Antwort gilt die Zeile Vorgabe; sie ist wortlauttreu oder trägt
  den Vermerk ‹weicht ab›.» `18` 1.1 (zweiter Punkt) und `03` Abschnitt 6 (Einleitung) darauf umschreiben; in
  Frage 37 und im Konzept-README denselben Satz. Zusätzlich RG-04 beheben, sonst ist die Vorgabe selbst nicht
  wortlauttreu. **Wer:** Koordinator in `18`, `03`, README; Frage 37.

#### RG-02 · kritisch · Die massgebliche Architektur hat E-28-30 schon gegen den Wortlaut entschieden

- **Stelle:** `20` E-28-30, Z. 498: «Vorgabe bis zur Antwort: (b), wortlauttreu zu LTV-002 (die Abläufe in `06`
  zeichnen schon (a))». Gegenstellen: `04` T-17 (Z. 93, massgeblich): «Ein dauerhaft oder zeitgesteuert empfangener
  Sender, der bei Mangel weicht, wird **zurückgestellt**»; `04` Z. 458–469 (Zustand «zurückgestellt»); `06` Z. 139–140
  («Rangfolge 5: zurückgestellt, F-4») und N-7 (Z. 1656: «für Zuschauer und Aufnahme auch dauerhaft und Zeitplan ohne
  Abnehmer»); `13` Z. 258: «Weicht ein Dauersender bei Mangel einem Zuschauer (**Vorgabe F-4 (a)**)»; `13` Z. 645
  (Überlastreihenfolge Stufe 5 «zurückgestellt»); `13` M-7 übt es.
- **Problem:** Marco liest in `20` «bis zu deiner Antwort bleibt dauerhaft dauerhaft». Architektur, Abläufe,
  Ressourcenplan und Überlastreihenfolge bauen aber (a): Der permanente Empfang wird bei Mangel unterbrochen. `13`
  nennt (a) sogar ausdrücklich «Vorgabe». Ein Executor folgt `04` (massgeblich nach `02` Lesart 5 und `07` Kopf),
  nicht `20`. Verbindliche Anforderung LTV-002 «Permanenter Empfang … unabhängig davon, ob jemand zuschaut» (Z. 395)
  ist damit stillschweigend geändert.
- **Korrektur:** In `04` T-17, 5.3, 8.2, `06` Zustands- und Rangfolgetabelle, N-7, `13` 8.2 und Z. 258 den Zustand
  «zurückgestellt» für dauerhaften und zeitgesteuerten Empfang als **bedingt auf E-28-30 (a)** kennzeichnen und die
  Vorgabe (b) als Regelfall beschreiben (Zuschauer bekommt «Server ausgelastet»); oder, falls der Koordinator (a)
  will, E-28-30 als «Vorgabe weicht ab» markieren. **Wer:** Autoren von `04`, `06`, `13`; Koordinator.

#### RG-03 · kritisch · Der Schalter «Vorladen für Sender dieser Quelle» ist ein Verbindungsschalter je Anbieter und bremst drei verbindliche Funktionen ab Werk

- **Stelle:** `04` 5.3 (Z. 374–385): «Bei einem Zugang mit einer einzigen erlaubten Verbindung steht der Schalter auf
  «aus»»; Tabelle: Schalter aus heisst «kein Vorladen, kein «automatisch», keine Prüfung neben einem laufenden
  Empfang». `07` Z. 381: `preload_enabled INTEGER NOT NULL DEFAULT 0`. `08` Z. 927: `preloadEnabled`. `20` E-28-29:
  Vorgabe (b) «je Quelle ab Werk aus», Titel mit Vermerk «Wortlaut». `20` Z. 477–478: «der Koordinator hat
  festgelegt, dass er eine Einstellung des Vorladens ist, keine Verbindungsverwaltung».
- **Problem:**
  1. **Funktion:** Nach dem eigenen Wortlaut von `04` 5.3 dient der Schalter dazu, Zugänge mit einer erlaubten
     Verbindung von Zugängen mit mehreren zu unterscheiden. Das ist ein Verbindungskontingent «1 oder mehr» je
     Quelle, also je Anbieterzugang. LTV-017 schliesst «Anbieterbezogene Verbindungskontingente» aus (Z. 991), die
     Ausschlussliste «Separate IPTV-Anbieterlimits» (Z. 1779), und die Leitplanken verlangen ausdrücklich: «auch kein
     «vom Administrator eingetragenes Limit» als Konfliktregel. Wer es für nötig hält, schreibt es als offene
     Entscheidung für Marco auf, nicht ins Konzept.» Der Schalter steht aber als Spalte, API-Feld, Verwaltungsseite
     (LA-03, LA-09) und Mockup im Konzept. Die Umbenennung ändert die Funktion nicht; die Deutung eines
     Grundsatzentscheids hat der Koordinator getroffen, nicht Marco.
  2. **Wirkung ab Werk:** Mit `DEFAULT 0` und Vorgabe (b) gibt es für jede neue Quelle kein Vorladen (LTV-002 «Vorladen
     benachbarter Sender», LTV-010.05 bis .07), keinen automatischen Empfang (LTV-002 «Automatischer Empfang»,
     LTV-010.06) und keine parallele Verfügbarkeitsprüfung, bis der Administrator den Schalter je Quelle umlegt.
  3. **Frage unvollständig:** E-28-29 nennt nur Vorladen und «automatisch», nicht die Prüfungen, und verschweigt den
     Zweck «Zugang mit einer Verbindung» aus `04` 5.3. Die Vorgabe ist die Empfehlung, nicht die wortlauttreue
     Variante, obwohl der Titel «Wortlaut» trägt (Verstoss gegen `20` Z. 14).
- **Korrektur:** E-28-29 neu schreiben: Zweck und alle drei Wirkungen offen nennen; als Grenzfall zu LTV-017 an
  Marco; wortlauttreue Vorgabe ist **(d) kein Schalter je Quelle** (nur globale Einstellungen des Vorladens), weil
  sie weder LTV-017 noch die Leitplanken berührt. Bis zur Antwort in `07` 3.2, `08` 3.7, `15` LA-03/LA-09 und `18`
  4.5 die Spalte als «nur bei E-28-29 (b)» kennzeichnen. «Festlegung des Koordinators» aus `20`, `04`, `07`, `08`
  streichen oder als Empfehlung umformulieren. **Wer:** Koordinator; Autoren `04`, `07`, `08`, `15`, `20`.

### Hoch

#### RG-04 · hoch · Mehrere «Vorgaben» sind nicht wortlauttreu, obwohl `20` das als Regel verspricht

- **Stelle und Beleg:** `20` Z. 14 verspricht: weicht die Empfehlung vom Wortlaut ab, ist die Vorgabe die
  wortlauttreue Variante. Gegenbeispiele:
  - **E-28-12** (Safari, iPhone, iPad, «Wortlaut»): Vorgabe (a) «ungeprüft und nicht zugesagt» ist die Abweichung.
    Die Spec v1 trägt die PWA ausdrücklich auch für iPhone und iPad («iOS-Eviction», `docs/spec/01-anforderungen-v1.md`
    Constraints); Entscheid 2 sagt nur «ungeprüft», nicht «nicht zugesagt». Zudem stellt `18` Kopf (Z. 15–16), 2.1
    (Z. 139), RP-14 und `16` Z. 23 es als **entschieden** dar.
  - **E-28-29**: siehe RG-03.
  - **E-28-31** (eingefrorene Ströme, «Wortlaut»): Vorgabe (a) heisst «LTV-011 nur teilweise erfüllt» bis Stufe E.
  - **E-28-56** (Untertitel im Browser, «Wortlaut»): Vorgabe ist Stufe E, also bis dahin LTV-004.17 und LTV-014.52 im
    Browser nicht erfüllt (dazu RG-05).
  - **E-28-43**: Vorgabe «geplante Termine nur als Zahl» ist keine der Varianten und geht über E-3 hinaus (E-3 nennt
    nur Aufnahmen; LTV-009 Z. 679 «Eigene Aufnahmeplanungen»). Wortlauttreu wäre (A).
  - **E-28-49**: Vorgabe (B) führt eine nicht verlangte Einstellung ein (Scope-Creep, Abschnitt 4).
- **Korrektur:** Je Frage entweder die Vorgabe auf die wortlauttreue Variante setzen oder in der Zeile «Vorgabe» den
  Vermerk **«weicht ab: …»** mit dem betroffenen LTV-nnn.mm ergänzen und die Frage in der Kurzübersicht kennzeichnen.
  Für Stufungen (E-28-31, E-28-56) reicht «wortlauttreu ab Stufe X, vorher nicht erfüllt». In `18` Kopf und 2.1 die
  Apple-Aussage als «Vorschlag, Frage E-28-12» formulieren. **Wer:** Koordinator in `20`, `18`, `16`.

#### RG-05 · hoch · Untertitel: zwei widersprüchliche Aussagen zum Browser, und Teletext-Untertitel fehlen in jedem Client ohne Frage an Marco

- **Stelle:** `16` Matrix E (Z. 213–216): «Im Browser und auf dem iPhone gibt es Untertitel in den ersten Stufen nur
  eingebrannt … «Individuelle Untertitelauswahl» (LTV-004) ist dort erfüllt, aber teuer.» Gegen `20` E-28-56
  (Z. 816–818): «Bis Stufe E zeigt der Browser darum keine Untertitel; LTV-004.17 ist dort bis dahin nicht erfüllt»,
  und `17` AK-L-332: «ohne diese Stufe fehlt die Wahl im Browser mit Hinweis». Ausserdem `16` Matrix E Zeile
  «Teletext-Untertitel»: Browser «nicht angeboten», Apps «**kein Client zeichnet sie**: Media3 hat keinen
  Teletext-Leser». `20` E-28-56 schreibt dagegen «Fernseher und Handy-App zeigen DVB-Untertitel schon ab Stufe 1a»
  und erwähnt Teletext nicht; die Umsetzung Teletext nach WebVTT steht nur als OT-28-05 (Teil B, «für die
  Entwicklung, nicht für dich»).
- **Problem:** LTV-004 verlangt «Individuelle Untertitelauswahl» (Z. 499), LTV-014 im Player «Untertitelauswahl»
  (Z. 906). Ob das in Stufe 1a im Browser erfüllt ist, sagt ein Papier «ja (eingebrannt)», zwei sagen «nein». Für
  Teletext-Untertitel ist die Anforderung bis Stufe E in **keinem** Client erfüllt, und Marco wird dazu nicht
  gefragt. Ob seine Quellen Teletext- oder DVB-Untertitel liefern, ist Annahme (V-1 misst es).
- **Korrektur:** Eine Aussage festlegen (Empfehlung: in 1a im Browser einbrennen als Rückfall, wie `16`, oder
  ausdrücklich «keine Untertitel bis E»), in `16`, `20`, `17` AK-L-332, `03` LTV-004.17 und LTV-014.52 gleich.
  E-28-56 auf «Untertitel im Browser **und Teletext-Untertitel in allen Clients**» erweitern, mit wortlauttreuer
  Vorgabe. **Wer:** Autoren `16`, `17`; Koordinator `20`.

#### RG-06 · hoch · Was «Freigabe» bedeutet und wann das Nicht-Ziel fällt, steht viermal verschieden da

- **Stelle:**
  - `CLAUDE.md` Regel 11 (Z. 89) und Spec Non-Goals: «**Live-TV ist kein Nicht-Ziel mehr**, aber noch nicht zum Bau
    freigegeben», seit 28. November 2026.
  - ADR 0046 Status: «vorgeschlagen … **Hebt mit der Freigabe** für Live-TV das Nicht-Ziel «Live-TV und DVR» auf …
    Angenommen wird es erst mit Stufe 0»; ADR «Folgen»: «Mit der Freigabe ändern sich Regel 11, Spec … ; diese Stellen
    sind schon in der Konzeptphase nachgeführt» (widerspricht sich im selben Satz).
  - `18` 1.1 (Z. 54–59): «Keine Migration, kein Code, **keine Änderung an Regeln, Spec oder
    `docs/dev/CURRENT_STATUS.md`** (Muster der Konzepte 16, 25, 26, 27)» und «Mit deiner Freigabe **kommt Frage 37 ins
    Entscheidungsprotokoll (Entwurf in `02` Abschnitt 5)**». Beides ist überholt: Frage 37 steht schon im Protokoll
    (`04-entscheidungen.md` Z. 143), Regel 11, Spec und `CURRENT_STATUS.md` sind geändert.
  - `21` Abschnitt 5 meldet, `02` Abschnitt 4 und `18` 0.2 beschrieben «noch den früheren Weg» und seien
    nachzuführen. `02` und `18` 0.2 sind nachgeführt, `18` 1.1 nicht; `21` sagt das Gegenteil des Stands.
  - Drei Arten «Freigabe» ohne Begriffsklärung: Freigabe des Konzepts (`18` 1.1: dann gelten Vorgaben oder
    Empfehlungen), Baufreigabe je Stufe (Regel 11, Frage 37), Annahme von ADR 0046 mit Stufe 0.
- **Problem:** Für Agenten ist der Bau überall verboten (kein Code vor Baufreigabe); das ist eindeutig. Für Marco
  (kein Entwickler) ist unklar, was er mit «freigegeben» auslöst, und ADR 0046 sagt etwas anderes als Regel 11.
  ADR 0046 Entscheid 1 legt zudem «nie in … Suche der Bibliothek» fest, obwohl E-28-23 (Programmsuche in der
  allgemeinen Suche) offen ist.
- **Korrektur:** ADR-Status auf «vorgeschlagen; das Nicht-Ziel ist mit Frage 37 schon aufgehoben, der Bau ist es
  nicht» ändern und «Folgen» bereinigen; `18` 1.1 auf den heutigen Stand; `21` Abschnitt 5 berichtigen; im README und
  in `20` die drei Freigaben mit je einem Satz erklären. ADR Entscheid 1 «Suche der Bibliothek» an E-28-23 binden.
  **Wer:** Koordinator (ADR 0046, `18`, `21`, README).

#### RG-07 · hoch · `24-recherche-und-quellen.md` führt widerlegte Aussagen als «belegt» und kennt die Nachprüfung nicht

- **Stelle:** `24` Z. 128: «FFmpeg-HLS-Demuxer: `max_reload` und `m3u8_hold_counters` je 1000 … | **belegt**».
  `nachpruefung-quellen.md` Nr. 27: «**widerlegt** (max_reload) … Standard **100**». `24` Z. 113 und N-7: «`hls.js/light`
  lässt vermutlich … CEA-608 weg» (Nachprüfung Nr. 12: «nicht belegt, wahrscheinlich falsch»). `24` W-5 (Z. 307) und
  N-6 (Z. 328) stehen noch offen, obwohl Nachprüfung Nr. 16 sie entschieden hat (R2-Teil **widerlegt**). Das Wort
  «Nachprüfung» kommt in `24` nicht vor; `24` ist Fassung 1 von 12:02, die Nachprüfung von 13:49.
- **Problem:** Auftrag Z. 1838–1840 verlangt Quellen und Erkenntnisse sowie die Trennung von Fakt und Annahme; die
  verlangte Übersicht (Auftrag Abschnitt 18 Punkt 24) widerspricht der eigenen Nachprüfung. `04` Z. 1733 und `16`
  Z. 379 haben die Korrektur, `24` nicht.
- **Korrektur:** `24` Fassung 2: je Zeile die Spalte «Nachprüfung» (Nr., Befund) ergänzen; Nr. 16, 27 berichtigen;
  Nr. 7, 10, 12, 13, 32, 40 («teilweise, abweichend») übernehmen; erledigte N- und W-Punkte schliessen.
  **Wer:** Autor `24` (quellen-autor) nach `nachpruefung-quellen.md`.

#### RG-08 · hoch · `20` erklärt Auslegungen des Koordinators zu Marcos Entscheiden und fragt sie nicht ab

- **Stelle:** `20` Z. 18: «Schon entschieden und hier nicht mehr gefragt: deine vier Antworten E-1 bis E-4, **alles
  aus den Leitplanken** (…) und die 21 Grundsatzentscheide.» Die Leitplanken sind vom Koordinator; Marcos Wort sind
  nur E-1 bis E-4. Darunter fallen:
  - **Fire TV:** Auftrag LTV-014 Z. 944 «Berücksichtige: … Fire TV». `03` macht daraus den Ausschluss
    LTV-014.X-01 «Nicht dabei: Fire TV»; `20` enthält keine Frage dazu. (Mildernd: Konzept 26 E-1 «Fire TV nicht»
    ist Marcos früherer Entscheid, und Auftrag Abschnitt 4 lässt bestehende Ausschlüsse stehen. Ob LTV-014 diesen
    Ausschluss «ausdrücklich» ändert, kann nur Marco sagen.)
  - **Plattform-Streams** (YouTube, Twitch) als Quelle ausgeschlossen: neu in Spec-Nachtrag («Ebenso: Plattform-Streams
    …»), `02` A-1, ADR 0046 A-1, `03` Z. 84. Der Auftrag kennt diesen Ausschluss nicht; der Bestand verbietet nur
    YouTube-**Downloads** (für Aufnahmen trägt das, für das Schauen nicht).
  - **Kein Download und kein Offline von Aufnahmen** (`18` 10.3): kein Auftragstext, nur Leitplanken.
  - **«Kein eigener Dienst von Slouchcat im Internet für Live-TV»** (A-1): vernünftig, aber nicht Marcos Wort.
- **Problem:** Marco kann nicht sehen, welche «Entscheide» seine sind. Der Fire-TV-Ausschluss ändert eine
  Aufzählungszeile eines verbindlichen Entscheids; der Plattform-Ausschluss steht schon in der Spec.
- **Korrektur:** In `20` Teil A.1 eine Frage **«Bestätige die Auslegungen des Koordinators»** mit Liste (Fire TV,
  Plattform-Streams, kein Download von Aufnahmen, kein eigener Internetdienst, «Kinderprofil» = Altersgrenze unter
  18) und wortlauttreuer Vorgabe je Punkt; Z. 18 auf «deine Antworten E-1 bis E-4 und die 21 Grundsatzentscheide»
  kürzen. **Wer:** Koordinator.

### Mittel

#### RG-09 · mittel · Änderungen an bestehenden Funktionen, die Marco nicht gefragt wird

- **Stelle:** `18` 9 Z. 679–680: «neue Erwartungen an bestehendes Verhalten gibt es nicht». Dieselbe Tabelle
  (Z. 685) und `04` Z. 1063, `12` Z. 76 und 374: `netguard` bekommt neue gesperrte Präfixe, «wirkt auch für Radio und
  Podcasts und ist eine Verschärfung»; `14` LS-25: «Einschalten von Live-TV **beendet Radio und Hörbuch**».
- **Problem:** Beides ändert Verhalten bestehender Funktionen (Auftrag Z. 1509, Z. 330). Die Verschärfung ist
  sicherheitlich richtig, kann aber einen bisher spielenden Radiosender stummschalten; der Ton-Vorrang ist eine
  sichtbare Verhaltensänderung des Radios. Keines steht in `20`. Der Satz in `18` 9 ist damit falsch.
- **Korrektur:** `18` 9 Satz berichtigen; beides in `20` als Feinschliff-Fragen (Vorgabe: Verschärfung mit Hinweis
  im Release-Text; Ton-Vorrang wie beim heutigen Wechsel Radio/Hörbuch). **Wer:** Koordinator.

#### RG-10 · mittel · «Erledigt» gemeldete Umbenennungen sind nicht überall nachgezogen

- **Stelle:** `20` Teil E U-10 und U-18 melden «erledigt». Noch vorhanden: `17` AK-L-008 (Z. 308) «Quelle mit «weitere
  Verbindungen erlaubt»; zweite Quelle im Modus «nur eine Verbindung»»; `20` Teil D Z. 982 «Ablehnung als «vermutlich
  Limit»»; `19` Abschnitt 6 Z. 265 «vorsorglich zusätzliche Verbindungen öffnen … hängt an … F-9»; `15` Z. 1106
  (Änderungsliste, «weitere gleichzeitige Verbindungen erlaubt»).
- **Korrektur:** Suchlauf nach den alten Namen und Ersetzen; Teil E erst danach auf «erledigt». Nach RG-03 ohnehin
  neu fassen. **Wer:** Koordinator (Restebereinigung).

#### RG-11 · mittel · Ein Abnahmekriterium legt den Werkszustand einer verbindlichen Funktion fest

- **Stelle:** `17` AK-L-083: «gewinnt Vorladen im Median nichts, **ist die Vorgabe «aus»** (`13` M-17)».
- **Problem:** Ein Messergebnis schaltet Vorladen (LTV-002, LTV-010.05) ab Werk ab, ohne Frage an Marco; widerspricht
  E-28-29 (Vorgabe «global an»).
- **Korrektur:** «… dann legt das Messprotokoll E-28-29 Marco erneut vor.» **Wer:** Autor `17`.

#### RG-12 · mittel · `22-anforderungsmatrix.md` ist auf einem älteren Stand und enthält Skriptreste

- **Stelle:** `22` Z. 21 («ein eigenes Dokument `20-offene-entscheidungen.md` gibt es noch nicht»), Z. 1710 («Fehlende
  Dokumente: `20` …, `21` … und `23`»), Z. 1595–1596 (unersetzte Platzhalter `{len(excl)}`, `{sum(1 for r in excl …)}`),
  Z. 1566 («332» `AK-L`; `17` hat 333), Z. 1707 («LTV-X-29 ohne Negativtest»; `AK-L-333` ist es), Z. 1570 («31»
  Tabellen; `07` Z. 2243 nennt 32), Spalte «Fragen» mit «dd F-n» statt `E-28-nn`.
- **Problem:** Die Matrix ist das Dokument, mit dem Marco Vollständigkeit prüfen soll (Auftrag Abschnitt 18 Punkt 21).
- **Korrektur:** Skript nach dem letzten Stand von `03`, `07`, `17`, `20` neu laufen lassen; Fragen auf `E-28-nn`.
  **Wer:** matrix-autor.

#### RG-13 · mittel · `19-mockups.md` Fassung 2: Platzhalter, alte Kennungen und eine gezeigte Abweichung als Normalfall

- **Stelle:** `19` Fassung 2 (Stand 15:14, während dieses Reviews nachgeführt; RM-01 bis RM-16 sind dort
  abgearbeitet). Verbleibend: Abschnitt 8 Zeile «**Zustände:** STATES_RESULT» (unersetzter Platzhalter statt
  Prüfergebnis); Abschnitt 6 verweist auf «F-4», «F-7», «F-23» statt `E-28-43`, `E-28-57`, `E-28-53`; Abschnitt 9
  «neue Beispieldaten: … **3sat zurückgestellt**» und Abschnitt 6 «zurückgestellt (3sat) in LA-01, LA-09, LA-12»:
  Das Mockup zeigt dem Auftraggeber die Abweichung aus RG-02 (dauerhafter Empfang weicht) als gewöhnlichen Zustand,
  ohne Hinweis, dass sie an E-28-30 (a) hängt und die Vorgabe (b) ist. LA-09/LA-03 zeigen den Schalter aus RG-03.
- **Korrektur:** Platzhalter durch das Ergebnis ersetzen oder die Zeile streichen; Kennungen auf `E-28-nn`; in
  Abschnitt 6 je eine Zeile «zeigt E-28-30 (a), Vorgabe ist (b)» und «zeigt E-28-29 (b), hängt an RG-03».
  **Wer:** mockup-geruest / Koordinator.

#### RG-14 · mittel · Komplexität über dem Homelab-Mass (wäre einfacher, ist nicht falsch)

- **Zahlen:** 14 Stufen (V, 0, 1a, 1b, 2, 3, H, 4, 5, 6, L, W, E, Apps), 39 Tabellen in zwei Datenbankdateien
  (32 und 7) plus 3 Neubauten, 115 Operationen, 927 Anforderungen, 333 Abnahmekriterien, 58 Fragen, 61 technische
  Punkte, 12 Versuche der Stufe V, rund 31 Einstellungen in `live_config`, drei Empfangswege (Go-Empfänger, Weg W mit
  FFmpeg, RTSP/RTMP-Bibliotheken).
- **Einschätzung:** Keine Tabelle und keine Operation dient einer ausgeschlossenen Funktion (Abschnitt 4). Die Grösse
  folgt grösstenteils aus dem verbindlichen Umfang (21 Entscheide, 18 Diagrammfälle, Abschnitt 9 mit sieben
  Verwaltungsbereichen). Vereinfachbar ohne Verlust einer verbindlichen Anforderung:
  1. **Einstellungen als Konstanten:** `grace_seconds`, `ring_seconds`, `ring_budget_mb`, `transcode_share_pct`,
     `max_sessions`, `preload_max`, `automatic_keep`, `idle_check_hours`, `idle_check_parallel`, `dvr_warmup_seconds`,
     `dvr_instant_max_hours`, `dvr_max_concurrent`, `startbuffer_mode` verlangt der Auftrag nicht als Einstellung
     (verlangt sind Strategie, Zeitpläne, Timeshift-Höchstdauer, Vor-/Nachlauf, Fristen, Speicherorte, Limits,
     Reserve, Intervalle, EPG-Zeitraum). Jede Einstellung kostet Oberfläche, Tests und Verträglichkeit.
  2. **Stufen bündeln:** 1a und 1b zu einem Release, L und W zu einer Stufe «weitere Protokolle», E in die Stufen
     auflösen, deren Anforderung sie fertig macht. Der Plan sieht Bündeln schon vor (`18` 1.3).
  3. **`epg.db`** nur, wenn `09` M-3 es zeigt (E-28-24 ist richtig als Frage, aber die Vorgabe (a) baut die zweite
     Datei vor der Messung).
- **Korrektur:** In `20` eine Feinschliff-Frage «Einstellungen, die der Auftrag nicht verlangt: als feste Werte?» mit
  Liste. **Wer:** Koordinator.

#### RG-15 · mittel · Zwei Rechte, die der Auftrag «bestehend» nennt, gibt es nicht; Marco erfährt es nur in `02`

- **Stelle:** `02` AB-9 («Bestehende Transcoding-Berechtigungen» **fehlt**), AB-10 («Bestehende lokale und externe
  Zugriffsbeschränkungen» **fehlt als Recht**), je «Annahme, Empfehlung». Auftrag LTV-012 Z. 809–810.
- **Problem:** Wortlautgetreu ist «nichts übernehmen, was es nicht gibt»; Marco könnte aber annehmen, er könne Live-TV
  von aussen sperren oder Umwandlung je Konto verbieten. Das ist eine Weiche (sichtbares Verhalten), keine Annahme.
- **Korrektur:** In `20` A.2 als Weiche mit Vorgabe «keine neuen Rechte» (wortlauttreu) aufnehmen. **Wer:**
  Koordinator.

#### RG-16 · mittel · Stufe V erlaubt Go-Code vor der Baufreigabe; Regel 11 kennt diese Ausnahme nicht

- **Stelle:** `18` 1.4 Z. 93–94 («Die Messversuche der Stufe V brauchen keinen Produktcode und können auch dann laufen,
  wenn der Bau noch nicht beauftragt ist»), 2.2 (V-1 «Wegwerf-Werkzeug in Go», V-4, V-5 «Wegwerf-App»), aber V-8:
  «**kein Wegwerf-Code**, sondern der Empfänger aus Stufe 1». `CLAUDE.md` Regel 11 neu: «bis dahin gilt … keinen Code».
- **Problem:** V-8 gehört nicht in Stufe V (er braucht Stufe 1). Für V-1 bis V-7 muss ein Agent Code schreiben; Regel 11
  verbietet «Code» ohne Ausnahme. Wer V-1 ohne Baufreigabe startet, verstösst gegen Regel 11 oder kann V nicht machen.
- **Korrektur:** V-8 nach Stufe 1 verschieben (Tor zu Stufe 3, wie `18` 2.2 selbst sagt); in Regel 11 oder Frage 37
  einen Halbsatz: «ausgenommen Wegwerf-Messversuche der Stufe V auf einem nie gemergten Zweig, nur auf Marcos Wort».
  **Wer:** Koordinator (`18`, Frage 37; Regel 11 nur auf Marcos Wort).

#### RG-17 · mittel · Übergabe: kein roter Faden, falsche Gewichtung vor Stufe 1a, fehlende Weichen

- **Stelle:** `README.md` und `23` fehlen noch (vom Koordinator angekündigt). `20` hat 1212 Zeilen; vor Stufe 1a sind
  21 Fragen fällig, davon 7 «Feinschliff» (E-28-15 bis -21), deren Vorgabe sicher ist. Zugleich fehlen in `20` die
  Weichen aus RG-01 (die Regel selbst), RG-03 (vollständig), RG-05 (Teletext), RG-08 (Auslegungen), RG-09 (Radio),
  RG-14 (Einstellungen), RG-15 (Rechte).
- **Korrektur:** README mit Lesereihenfolge (Abschnitt 6 dieses Berichts); in `20` eine erste Seite «Die 10 Fragen,
  die vor der Baufreigabe wirklich zählen» (E-28-02, -03, -04, -07, -09, -10, -29, -30, Auslegungen, Teletext);
  Feinschliff vor 1a in einen Sammelpunkt «Vorgaben bestätigen». **Wer:** Koordinator.

### Niedrig

#### RG-18 · niedrig · Veraltete Merkzeilen in `13`

`13` Z. 839–844 (H-5, H-6, H-7, H-10): «offen, bis `15` nachgeführt ist», «`18` ist noch nicht nachgezogen». `15`
Z. 755, 790, 996 und `18` 9 sind nachgeführt. Zeilen auf «erledigt» setzen. **Wer:** performance-autor.

#### RG-19 · niedrig · Werte ausserhalb der «einen Quelle» `04` Abschnitt 14

`02` Lesart 5 und `04` 14 erklären `04` 14 zur einzigen Quelle aller Zahlen. Nicht dort: längste Sofortaufnahme
4 Stunden (`11` Z. 342, `15` Z. 739, `07` `dvr_instant_max_hours`), Vorlauf der Erinnerung 0/5/15/30 (`03` DR-08),
Vorgabe-Kontingent 100 GB (E-28-39). In `04` 14 nachtragen. **Wer:** arch-autor.

#### RG-20 · niedrig · E-28-11 und Teil E sagen zu `04` 10.5 und `07` 6.3 Gegenteiliges

`20` Z. 255: «`04` 10.5 und `07` 6.3 sind danach anzugleichen (Teil E)»; Teil E U-15: «erledigt: setzen die Vorgabe
«leeren» schon um». Satz in E-28-11 streichen. **Wer:** Koordinator.

#### RG-21 · niedrig · Regel 11 nennt Live-TV im ersten Satz nicht mehr

`CLAUDE.md` Z. 75: «Live-TV» ist aus der Liste der Nicht-Ziele gestrichen, das Bauverbot steht erst neun Zeilen
später. Der Absatz ist eindeutig; wer nur den ersten Satz liest, übersieht ihn. Vorschlag: in der Liste «Live-TV (bis
zur Baufreigabe, siehe unten)» stehen lassen. Nicht zwingend. **Wer:** Koordinator, nur auf Marcos Wort.

#### RG-22 · niedrig · `18` 2.1 führt eine erledigte Frage als offen und zählt nach alten Kennungen

`18` 2.1 Z. 137 nennt `15` F-14 («neue Sender ab Werk aktiv») als Frage vor Stufe 1; `20` Anhang A: «`15` F-14 erledigt
durch `03` DR-16». Die ganze Tabelle `18` 2.1 arbeitet mit «`03` F-6», «`04` F-12» … statt `E-28-nn`; für Marco sind
das zwei Nummernsysteme für dieselben Fragen. Tabelle auf `E-28-nn` umstellen oder durch den Verweis auf `20` ersetzen.
**Wer:** plan-autor.

---

## 3. Die 21 Grundsatzentscheide

Urteil je Entscheid nach Prüfung von `03` (Anforderungen), vertiefendem Dokument, `17` (`AK-L`), `18` (Stufe) und
`20` (Frage). «Ausgewiesen» heisst: als Frage an Marco in `20` mit wortlauttreuer Vorgabe.

| Entscheid | Urteil | Begründung, Beleg |
|---|---|---|
| LTV-001 Funktionsumfang | vollständig | Sammelentscheid; jede Zeile verweist auf einen Detailentscheid (`AK-L-312`). |
| LTV-002 Empfangsstrategie | **abgeschwächt und nicht ausgewiesen** | Permanenter Empfang weicht Zuschauern in `04`, `06`, `13` und im Mockup, Vorgabe in `20` sagt das Gegenteil (RG-02); «automatisch» und Vorladen ab Werk je Quelle aus über den Schalter (RG-03). |
| LTV-003 Empfangsquellen | abgeschwächt und ausgewiesen | DASH und HLS-fMP4 erst Stufe W und nur mit Sperre oder eigenem Leser (E-28-55, Vorgabe wortlauttreu); RTSP/RTMP Stufe L (E-28-54); UDP/RTP/Multicast Stufe H, Multicast vielleicht unerfüllbar (E-28-34, U-8). Nicht ausgewiesen: Ausschluss der Plattform-Streams (RG-08). |
| LTV-004 Zentraler Empfang, Transcoding | **abgeschwächt, teils nicht ausgewiesen** | Alle 38 Zeilen haben Stufe und (ausser .21, .38) ein `AK-L`. Untertitelwahl im Browser: widersprüchlich (`16` gegen `20`, RG-05); Teletext-Untertitel in keinem Client bis Stufe E, nicht gefragt. |
| LTV-005 EPG | vollständig | gzip belegt; xz/zstd nur bei Bedarf (OT-28-32) ist innerhalb «komprimierte XMLTV-Dateien» vertretbar. |
| LTV-006 Senderverwaltung | vollständig | 1a/1b-Schnitt, alle Zeilen mit Stufe. |
| LTV-007 Timeshift | vollständig | Alle 14 Zeilen mit Stufe 4 bzw. 3 und `AK-L` (ausser .12, Sichtprüfung); Rückspulgrenze am Tastendruck (E-28-35, wortlauttreu); gemeinsame Speicherform nur auf Marcos Wort (E-28-36). |
| LTV-008 DVR | vollständig | Höchstdauer der Sofortaufnahme (4 h) ist eine nicht verlangte Grenze, einstellbar 1 bis 12 h (RG-19). |
| LTV-009 Aufnahmeeigentum | vollständig mit E-3 | Alle 12 Zeilen; Ausnahme E-3 sauber begrenzt (A-6, `AK-L-163`, `-173`). Vorgabe E-28-43 geht über E-3 hinaus (RG-04). |
| LTV-010 Fast Zapping | abgeschwächt und ausgewiesen | Zielwerte nur als Messgrössen (korrekt, Auftrag Z. 746); Apple-Clients «nicht zugesagt» mit nicht wortlauttreuer Vorgabe (RG-04); `AK-L-083` kann Vorladen ab Werk abschalten (RG-11). |
| LTV-011 Streamüberwachung | abgeschwächt und ausgewiesen | Standbild erst Stufe E (E-28-31); Vorgabe ist die Abweichung, nicht als solche bezeichnet (RG-04). |
| LTV-012 Benutzerrechte | vollständig | Zwei «bestehende» Rechte gibt es nicht (AB-9, AB-10); wortlauttreu nichts übernommen, Marco aber nicht gefragt (RG-15). |
| LTV-013 Kein Catch-up | vollständig | `AK-L-170`, `-171`; gemeinsamer Segmentvorrat verworfen (`04` Z. 1092). |
| LTV-014 Oberfläche | **abgeschwächt, teils nicht ausgewiesen** | Fire TV (Z. 944) als Ausschluss LTV-014.X-01 ohne Frage (RG-08); Safari/iPhone/iPad «nicht zugesagt», in `18` als entschieden (RG-04); Untertitelwahl im Browser (RG-05). |
| LTV-015 Jugendschutz | vollständig | Alle 14 Zeilen; eine Definition «Profil mit Altersgrenze» (`AK-L-326`); Detailregeln vorgelegt (E-28-07). |
| LTV-016 Multi-View, PiP | vollständig | `AK-L-174`. |
| LTV-017 Keine Anbieterlimits | **Grenzfall, irreführend ausgewiesen** | Schalter je Quelle mit dem Zweck «Zugang mit einer Verbindung» steht in Schema, API, Verwaltung und Mockup (RG-03); E-28-02 (Zahl je Quelle) ist korrekt als Frage mit wortlauttreuer Vorgabe. |
| LTV-018 Senderaktualisierung | vollständig | D-12 sauber; alle Zeilen mit `AK-L`. |
| LTV-019 Speicherverwaltung | abgeschwächt und ausgewiesen | Ort des Startpuffers: E-28-10 Vorgabe wortlauttreu (a), Empfehlung (b); nach `18`/`03` gilt aber die Empfehlung (RG-01). Ortswahl erst Stufe 4. |
| LTV-020 Originalqualität | abgeschwächt und ausgewiesen | Spuren ausserhalb des TS: E-28-44 Vorgabe (b) wortlauttreu, Empfehlung (a); RG-01 gilt. |
| LTV-021 Konzept vor Bau | vollständig | Kein Code ausserhalb `docs/` (`git diff --name-only origin/main...HEAD`); Stufe V braucht eine Klarstellung (RG-16). |

**Zusammengefasst:** 12 vollständig (009 mit einer kleinen Ausweitung über E-3, RG-04), 5 abgeschwächt und
ausgewiesen (003, 010, 011, 019, 020), 3 abgeschwächt und teils nicht ausgewiesen (002, 004, 014), 1 Grenzfall,
irreführend ausgewiesen (017). Fehlend ist keiner: Jeder Entscheid hat Anforderungen in `03`, ein vertiefendes
Dokument, mindestens ein `AK-L` und eine Stufe in `18` 10.1.

---

## 4. Scope-Creep-Kandidaten

Kein Bauteil dient einer Funktion der Ausschlussliste (Auftrag Abschnitt 16); geprüft an den 39 Tabellen in `07`, den
115 Operationen in `08` 2.13 und den 58 Seiten und Dialogen in `14`/`15`. Nicht verlangt oder über den Auftrag hinaus:

| Nr. | Kandidat | Wo | Berührt | Bewertung |
|---|---|---|---|---|
| SC-1 | Schalter «Vorladen für Sender dieser Quelle» | `07` `live_sources.preload_enabled`, `08` `preloadEnabled`, `15` LA-03/LA-09, Mockup | LTV-017, Ausschluss «Separate IPTV-Anbieterlimits» | **bereitet vor**, RG-03 |
| SC-2 | Höchstzahl gleichzeitiger Aufnahmen | `07` `live_config.dvr_max_concurrent`, E-28-49 Vorgabe (B) | LTV-008 «Gleichzeitige Aufnahmen» | nicht verlangt; ab Werk «keine», harmlos, aber Vorgabe sollte (A) sein |
| SC-3 | Zusätzliche Felder der Serienregel (Zeitfenster, Wochentage, «nur Erstausstrahlungen», Ende) | `14`, Mockup LD-02/LD-03 als «Vorschlag», E-28-53 | Grenze zu «suchwortbasierten Aufnahmen»: nein; nicht verlangt | Vorgabe (A) baut sie nicht; in Ordnung |
| SC-4 | Rund 13 Einstellungen in `live_config`, die der Auftrag nicht als einstellbar verlangt | `07` 3.1 | Overengineering | RG-14 |
| SC-5 | Zweite Datenbankdatei `epg.db` | `07` 3.7, E-28-24 | Overengineering, Sicherung | als Frage richtig, Vorgabe vor Messung |
| SC-6 | Verschärfung von `netguard` für Radio und Podcasts | `04` Z. 1063, `12` Z. 374 | bestehende Funktion | nicht verlangt, nicht gefragt (RG-09) |
| SC-7 | Ton-Vorrang: Live-TV beendet Radio und Hörbuch | `14` LS-25, `AK-L-217`, `-284` | bestehende Funktion | nicht gefragt (RG-09) |
| SC-8 | Härtung der Standard-Compose-Datei für alle Installationen | `18` 0.6, E-28-03 | bestehende Installationen | als Frage richtig ausgewiesen |
| SC-9 | Kamera-Gegenprobe, Messhaken im Client, Simulator `livesim` mit Build-Tag | `17`, `18` 0.7 | Testaufwand | verhältnismässig; kein Produktumfang |
| SC-10 | Längste Sofortaufnahme 4 h (Einstellung) | `07` `dvr_instant_max_hours`, `15` LA-14 | LTV-008 «Sofortaufnahmen» | nicht verlangte Grenze; als Schutz vertretbar |
| SC-11 | Neue Ausschlüsse ohne Auftrag: Plattform-Streams, kein Download von Aufnahmen, kein eigener Internetdienst | Spec-Nachtrag, `02` A-1, ADR 0046, `18` 10.3 | Umfang (Einschränkung, nicht Erweiterung) | RG-08 |

Gezählt: **0** Tabellen und **0** Operationen dienen einer ausgeschlossenen Funktion; **1** Spalte und **1** Feld
(SC-1) bereiten eine vor; **1** Spalte (SC-2) und rund **13** Einstellungen (SC-4) dienen keiner Anforderung.

---

## 5. Verbliebene Widersprüche

| Nr. | Sache | Stelle A | Stelle B | Befund |
|---|---|---|---|---|
| W-1 | Was ohne Antwort gilt | `20` Z. 14–15: Vorgabe | `18` Z. 57–59, `03` Z. 1416: Empfehlung | RG-01 |
| W-2 | Dauerhafter Empfang gegen neuen Zuschauer | `20` E-28-30 Vorgabe (b) | `04` T-17, `06` Z. 139–140, N-7, `13` Z. 258 «Vorgabe F-4 (a)», `19` «3sat zurückgestellt» | RG-02 |
| W-3 | Zweck des Schalters je Quelle | `20` E-28-29, `08` Z. 927: «keine Verbindungsverwaltung» | `04` 5.3: «Zugang mit einer einzigen erlaubten Verbindung … auf aus» | RG-03 |
| W-4 | Untertitel im Browser in Stufe 1a | `16` Matrix E: eingebrannt, «erfüllt» | `20` E-28-56, `17` AK-L-332: keine bis E | RG-05 |
| W-5 | Wann das Nicht-Ziel fällt | `CLAUDE.md` Regel 11, Spec: «kein Nicht-Ziel mehr» | ADR 0046 Status: «hebt mit der Freigabe auf» | RG-06 |
| W-6 | Stand der Projektdokumente | `18` 1.1: «keine Änderung an Regeln, Spec, `CURRENT_STATUS.md`; Frage 37 kommt mit der Freigabe» | Diff: alle geändert; Frage 37 eingetragen | RG-06 |
| W-7 | Stand von `02` und `18` 0.2 | `21` Abschnitt 5: «noch nachzuführen» | `02` Fassung 2, `18` 0.2: nachgeführt | RG-06 |
| W-8 | Safari, iPhone, iPad | `20` E-28-12: offene Weiche | `18` Kopf, 2.1, RP-14; `16` Z. 23: entschieden | RG-04 |
| W-9 | Werkszustand Vorladen | `20` E-28-29 Vorgabe «global an» | `17` AK-L-083: «gewinnt nichts → Vorgabe aus» | RG-11 |
| W-10 | Alte Namen des Schalters und der Ablehnung | `20` Teil E U-10, U-18: «erledigt» | `17` AK-L-008, `20` Teil D Z. 982, `15` Z. 1106 | RG-10 |
| W-11 | Recherche gegen Nachprüfung | `24` Z. 128, 113, W-5, N-6, N-7 | `nachpruefung-quellen.md` Nr. 12, 16, 27 | RG-07 |
| W-12 | Vorhandene Dokumente und Zählungen | `22` Z. 21, 1566, 1570, 1707, 1710 | Ordner, `17` (333), `07` (32), `AK-L-333` | RG-12 |
| W-13 | Merkzeilen | `13` H-5, H-6, H-7, H-10: offen | `15`, `18` 9: nachgeführt | RG-18 |
| W-14 | Abgleich der Geheimnisse in der Sicherung | `20` E-28-11: «anzugleichen» | `20` Teil E U-15: «erledigt» | RG-20 |
| W-15 | Suche der Bibliothek | ADR 0046 Entscheid 1: «nie in … Suche der Bibliothek» | `20` E-28-23 offen | RG-06 |
| W-16 | `15` F-14 | `18` 2.1 Z. 137: offen | `20` Anhang A: erledigt durch DR-16 | RG-22 |

Stichproben ohne Widerspruch: `04` 14 gegen `10`, `11`, `13`, `15`, `17` (Karenz 30 s, Herzschlag 10/60 s,
Timeshift 60 min, Deckel 20 %/50 GB, Eingänge 8, Sitzungen 20, Startpuffer 15 s, Warnfenster 15 min, Upload
200 MiB); `07`/`08` Vorlauf der Erinnerung {0, 5, 15, 30}; `08` 2.13 (115) gegen die Summe der Stufen in `18`
(58 + 24 + 5 + 1 + 7 + 13 + 7 = 115); `21` gegen den Diff (alle acht Dateien und Zeilenangaben stimmen).

---

## 6. Übergabereife und Lesereihenfolge

**Ist `20` für Marco verständlich?** Ja in der Form (Kurzübersicht, Varianten, Empfehlung, Vorgabe, Fälligkeit,
Alltagssprache). Nein in der Verlässlichkeit, solange RG-01 bis RG-04 offen sind: Marco kann sich nicht darauf
verlassen, dass «nichts sagen» den Wortlaut bedeutet.

**58 Fragen: zu viele?** Für eine Konzeptabnahme ja. Davon sind 24 Feinschliff mit sicherer Vorgabe. Richtig gewichtet
vor Stufe 0 sind E-28-02, -03, -04, -07; E-28-01, -05, -06 sind echte, aber weniger dringliche Weichen; E-28-08 ist
Feinschliff. Vor Stufe 1a fehlen die Weichen aus RG-03 (vollständig), RG-05 (Teletext), RG-08 (Auslegungen),
RG-15 (Rechte); E-28-15 bis -21 könnten gesammelt bestätigt werden.

**Roter Faden:** Ohne `README.md` (noch nicht angelegt) gibt es keine Lesereihenfolge durch 23 Dokumente mit rund
26'000 Zeilen. Vorschlag für das README: (1) `02` Grundsatzentscheide und Ausnahmen, (2) `20` Kurzübersicht und
Teil A.1/A.2, (3) `18` Kurzfassung und Abschnitt 4.13 (Stufenübersicht), (4) Mockups nach `19` Abschnitt 2 und 3,
(5) `03` Abschnitt 1 (Begriffe) und 7 (was nicht dabei ist), (6) `21` (was im Repo schon geändert ist); alles
Übrige nur auf Verweis.

**Die fünf Dinge, die Marco als Erstes lesen sollte** (nach den Korrekturen):

1. **`20` Lesart Punkt 1 und 2** mit der korrigierten Regel «ohne Antwort gilt der Wortlaut» (RG-01), weil sie bestimmt,
   was seine Freigabe auslöst.
2. **`20` E-28-02 und E-28-29** (Verbindungen zum Anbieter, LTV-017) in der neu geschriebenen Form (RG-03): die
   einzige Stelle, an der das Konzept einen Grundsatzentscheid berührt.
3. **`20` E-28-30** (dauerhafter Empfang gegen neuen Zuschauer, LTV-002) zusammen mit dem Hinweis, dass Architektur
   und Mockup heute (a) zeigen (RG-02).
4. **`02` Abschnitt 4** (Ausnahmen A-1 bis A-9 von bestehenden Regeln) und die neue Frage «Auslegungen des
   Koordinators bestätigen» (Fire TV, Plattform-Streams, kein Download; RG-08).
5. **`20` Teil C.3 «Was du beisteuern musst»** (eigene IPTV-Zugänge für V-1, XMLTV über zwei Wochen, Zeitfenster auf
   CT 190, Geräte): ohne diese Beiträge gibt es keine Baufreigabe der Stufe 1.

---

## 7. Geänderte Projektdokumente (Prüfauftrag 8)

- **Regel 11:** Kann nicht so gelesen werden, dass ein Agent Live-TV jetzt bauen darf: «bis dahin gilt für Live-TV
  dasselbe wie für ein Nicht-Ziel: keinen Code, keine Migration, keinen Vertrag, nichts vorbereiten» (Z. 92–93).
  Restpunkte: Stufe V (RG-16) und der erste Satz (RG-21).
- **Andere Nicht-Ziele:** unverändert, ausser Klarstellungen, die `21` als «Altlast» ausweist (Webradio im Plan,
  Android-Apps im Handover, Spec Constraints D-4, «Aufnahmen von Webradio» statt «DVR» in den Non-Goals). Die neuen
  Ausschlüsse «Plattform-Streams» und «eigener Internetdienst für Live-TV» stehen in Spec-Nachtrag und ADR, ohne
  Marcos Wort (RG-08).
- **Spec-Nachtrag, Frage 37, ADR 0046 gegen `02`:** inhaltlich gleich (E-1 bis E-4, A-1 bis A-9, T-29); abweichend nur
  der Status-Satz von ADR 0046 (RG-06).
- **Nicht in `21`:** keine Änderung gefunden, die `21` nicht nennt. `21` Abschnitt 5 selbst ist überholt (RG-06).

---

## 8. Auswirkungen auf bestehende Funktionen (Prüfauftrag 7)

`18` Abschnitt 9 benennt Wiedergabe und Umwandlung (Live-Anteil am Budget, kein Borgen, Filme behalten das ganze
Budget), Radio, Konten und Profile, Sichtbarkeit, Verwaltung, Benachrichtigungen (Neubau von `notifications` und
`deliveries`), Sicherung (`epg.db` und Aufnahmen nicht gesichert, Geheimnisse geleert), Updater (Warnung, Begleiter
unverändert), Compose (Härtung, `stop_grace_period: 30s` ab Stufe 5), Hintergrundjobs, Geräte, Offline, Protokolle,
Suche, Statistik, Service Worker und Apps, je mit Prüfung. Das ist vollständig bis auf:

- den falschen Satz «neue Erwartungen an bestehendes Verhalten gibt es nicht» (Verschärfung von `netguard`,
  Ton-Vorrang; RG-09);
- die Wirkung der Compose-Härtung und der längeren Stoppfrist auf **Updates bestehender Installationen** ist nur als
  Satz im Release-Text vorgesehen; E-28-03 fragt es, das genügt;
- die Neubauten `my_media`, `notifications`, `deliveries` (SQLite-Tabellenneubau) sind mit eingefrorenen DB-Ständen
  geprüft (`18` 9); ein Hinweis auf die Dauer der Migration bei grossen `notifications` fehlt (niedrig, nicht als
  Befund gezählt).

---

## 9. Gesamturteil

**Übergabereif nach Korrekturen.** Die Arbeit ist in Umfang, Rückverfolgung und Trennung von gebaut, spezifiziert,
Annahme und Messung sorgfältig; keine Kapazität und kein Zielwert wird als erreicht dargestellt, nichts als
implementiert, und kein Bauteil dient einer ausgeschlossenen Funktion. Vor der Übergabe müssen aber **RG-01 bis
RG-03** behoben sein, weil sie verbindliche Anforderungen (LTV-002, LTV-017, LTV-019, LTV-020) bei einer Freigabe ohne
Einzelantworten stillschweigend ändern. **RG-04 bis RG-08** sollten mitbehoben werden, weil sie Marcos Entscheidung
verzerren (Vorgaben, Untertitel, Freigabebegriff, widerlegte Belege, Auslegungen als Entscheide). Die mittleren und
niedrigen Befunde sind Nachzug und können in einer Restebereinigung laufen.

**Modus:** Begonnen gründlich; nach RG-01 (kritisch) auf adversarial umgestellt und die Nahtstellen um Vorgaben,
Schalter je Quelle und «erledigt»-Vermerke gezielt nachgeprüft (daraus RG-02, RG-10, RG-11).

**Nachprüfung der Schwere (Realist-Check):** RG-03 blieb kritisch, obwohl E-28-29 Variante (d) anbietet: Die
Vorgabe baut den Schalter, die Leitplanken verbieten ihn im Konzept, und die Frage verschweigt seinen Zweck. RG-08
von kritisch auf hoch gesenkt, gemildert durch Marcos früheren Entscheid «Fire TV nicht» (Konzept 26 E-1) und
Auftrag Abschnitt 4 (bestehende Ausschlüsse bleiben). E-28-12 (Apple) von hoch auf einen Teil von RG-04 gesenkt,
gemildert durch Entscheid 2 («iOS/macOS ungeprüft», `04-entscheidungen.md` Z. 32). RG-13 auf mittel belassen, weil
`19` während des Reviews auf Fassung 2 nachgeführt wurde und nur Reste bleiben.

## 10. Offene Fragen ohne Wertung

- Liefern Marcos Quellen Teletext- oder DVB-Untertitel? Davon hängt ab, wie schwer RG-05 wiegt (V-1).
- Hält der Koordinator (a) oder (b) bei E-28-30 für richtig? Je nachdem ist bei RG-02 `20` oder `04`/`06`/`13`
  zu ändern.
- Ist der Neubau von `notifications` auf einer Instanz mit vielen Meldungen in der Zeit des Updaters machbar? Nicht
  geprüft.
- `23-agent-einsatz-und-reviews.md` und `README.md` lagen nicht vor; ihre Stimmigkeit mit diesem Bericht ist nicht
  geprüft.
