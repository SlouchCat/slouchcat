# Konzept 25 · Entscheide, Regeländerungen und ADR-Entwürfe

> **Kurz gesagt.** Konzept 25 braucht Marcos Antwort auf 22 Fragen (F-1 bis F-22) und zwei Teilfragen (T-1, T-2);
> zu jeder steht hier, worum
> es geht, welche Möglichkeiten es gibt, was empfohlen ist und was eine andere Wahl bedeutet. Wünschen ist heute
> ein Nicht-Ziel; Abschnitt 2 listet jede Stelle in Spec, Plan, Protokoll, ADRs und `CLAUDE.md`, die sich mit
> einer Freigabe ändert, und was ausdrücklich Nicht-Ziel bleibt. Abschnitt 3 ist der Entwurf für Frage 33 im
> Entscheidungsprotokoll, Abschnitt 4 und 5 sind die Entwürfe von ADR 0037 und ADR 0038, Abschnitt 6 der Vorschlag
> für Regel 11, Abschnitt 7 die Einarbeitung der ersten Kritik samt Abweichungen von Anhang F. **Dieses Papier
> ändert keine Regel und keine bestehende Datei.**

> **Hinweis.** Alle Änderungen in Abschnitt 2 und 6 gelten erst, wenn Marco Konzept 25 freigibt und die
> Antworten als **Frage 33** in `docs/planning/04-entscheidungen.md` stehen. Bis dahin gilt der heutige Wortlaut,
> und kein Agent baut etwas davon (`docs/ideas/README.md:11-12`: «der Non-Goal fällt erst mit einem Eintrag im
> Entscheidungsprotokoll»). Die ADR-Dateien entstehen in Stufe 0 unter `docs/adr/`, nicht vorher.

Grundlage ist [Anhang F](anhang/F-entscheid-blatt.md) (Abschnitte 2, 8, 9, 10). Wortlaute von heute sind im Worktree am Anker
`ba6efe31` nachgesehen. Die übrigen Papiere: `01-bestandsanalyse.md`, `02-funktionskonzept.md`,
`03-architektur.md`, `04-benutzer-profilmapping.md`, `05-spotlight-integration.md`, `06-benachrichtigungen.md`,
`07-administration.md`, `08-datenmodell-api.md`, `09-umsetzungsplan.md`, `10-abnahmekriterien.md`. Die
Analyseberichte und die Kritik liegen in `anhang/`, [Anhang A](anhang/A-spotlight-startseite.md) bis
[Anhang G](anhang/G-kritik-fassung-1.md). Abschnitte mit **Vorschlag** gehen über Anhang F hinaus.

## 1. Offene Entscheide F-1 bis F-22

### F-1 Begriffe «Wünsche» und «anfordern»

- **Worum:** Wie heisst die Funktion in der Oberfläche? Ein Wort je Sache, überall gleich.
- **Möglichkeiten:** (a) «Wünsche» für die Funktion und Seiten, «anfordern» auf Knöpfen; (b) «Anfragen» und
  «anfragen» wie in Jellyseerr («Request»); (c) «Bestellen».
- **Empfehlung: (a).** «Wünschen» ist das alte Wort in Slouchcats Papieren (Frage 10), klingt nach Haushalt statt
  nach Behörde, und «Film anfordern» sagt auf dem Knopf genau, was passiert. «Bestellen» klingt nach Kauf.
- **Andere Wahl:** Nur Texte und Mockups ändern sich; technisch nichts.

### F-2 Zuordnung: A je Konto, B je Profil, C hybrid

- **Worum:** Mit welchem Jellyseerr-Benutzer sendet Slouchcat den Wunsch eines Profils?
- **Möglichkeiten:** (a) ein Jellyseerr-Benutzer je Konto; (b) je Profil; (c) je Konto als Standard, einzelne
  Profile mit eigenem Benutzer.
- **Empfehlung: (c).** Ein Kinderprofil kann einen eigenen Benutzer ohne Auto-Freigabe bekommen; alle anderen
  Profile brauchen keine Arbeit in Jellyseerr. Vergleich in `04-benutzer-profilmapping.md`, Abschnitt 4.
- **Andere Wahl:** (a) Kinder erben die Auto-Freigabe der Eltern. (b) je Profil ein Benutzer in Jellyseerr mit
  künstlicher E-Mail; ein gelöschtes Profil nimmt seine Anfragen mit.

### F-3 Wunsch als Benutzer über `X-API-User` oder über `userId`

- **Worum:** Jellyseerr hat einen Schlüssel für alles. Wie sagt Slouchcat, für wen ein Wunsch ist?
- **Möglichkeiten:** (a) Header `X-API-User` (undokumentiert, steht seit Overseerr im Code); (b) Feld `userId` im
  Body (dokumentiert).
- **Empfehlung: (a), mit Prüfung bei jedem Verbindungstest.** Nur so gelten Recht, Kontingent und Auto-Freigabe des
  Benutzers. Mit `userId` prüft Jellyseerr die Auto-Freigabe am Aufrufer, also am Administrator: Jeder Wunsch
  wäre sofort freigegeben ([Anhang D](anhang/D-jellyseerr-api.md) 3.3).
- **Andere Wahl:** (b) schafft die Freigabe in Jellyseerr faktisch ab; Kinderwünsche gingen ungeprüft an Radarr
  und Sonarr. Entfernt eine künftige Jellyseerr-Version den Header, meldet der Test es; ob «Wünsche aktiv» dann
  sperrt, ist Teilfrage T-1. Danach ist neu zu entscheiden.

### F-4 Konten ohne Zuordnung

- **Worum:** Was passiert mit einem Konto, dem der Administrator keinen Jellyseerr-Benutzer gegeben hat?
- **Möglichkeiten:** (a) es kann nicht anfordern; (b) es sendet als gemeinsamer Sammelbenutzer.
- **Empfehlung: (a) als Vorgabe, (b) als Wahl.** Ein Sammelbenutzer teilt Kontingent und Auto-Freigabe zwischen
  Gästen, und in Jellyseerr sieht man nicht mehr, wer gewünscht hat.
- **Andere Wahl:** (b) als Vorgabe: Gäste können sofort anfordern, aber unter einem Namen und mit gemeinsamem
  Kontingent.

### F-5 Wunschrecht je Konto und je Profil

- **Worum:** Wer darf überhaupt Titel anfordern, und wer stellt das ein?
- **Möglichkeiten:** (a) Schalter je Konto (Administrator) als Deckel und je Profil (Hauptprofil); (b) nur je Konto;
  (c) feste Regel wie in Frage 10 vorgeschlagen («Profile des Hauptkontos ab FSK 12, Gäste nicht»).
- **Empfehlung: (a)** mit den Vorgaben Besitzerkonto an, neue Konten aus, Hauptprofil an, weitere Profile aus. Das
  folgt dem Muster der Altersgrenze (Konto als Deckel, Profil fein) und dem Grundsatz «Niemand hebt seine eigenen
  Grenzen an» (ADR 0014).
- **Andere Wahl:** (b) Eltern können ein einzelnes Kind nicht ausnehmen. (c) passt nicht zu jedem Haushalt und ist
  nicht einstellbar.

### F-6 Kinderprofile und Freigabe

- **Worum:** Dürfen Kinderprofile anfordern, und wer gibt frei?
- **Möglichkeiten:** (a) mit Recht ja, die Freigabe entscheidet Jellyseerr über die Auto-Freigabe des zugeordneten
  Benutzers; (b) Kinder nie; (c) eine eigene Freigabe in Slouchcat («Kind fragt Eltern»).
- **Empfehlung: (a).** Jellyseerr hat die Freigabe schon; ein zweiter Ablauf in Slouchcat wäre doppelt und bleibt
  Nicht-Ziel (Spec Zeile 65).
- **Andere Wahl:** (b) nimmt Kindern die Funktion ganz. (c) ist ein neuer, grosser Ablauf und hebt ein weiteres
  Nicht-Ziel auf.

### F-7 Schon angeforderter Titel

- **Worum:** Ein zweites Profil will einen Titel, den schon jemand gewünscht hat. Jellyseerr sperrt Doppelte.
- **Möglichkeiten:** (a) Mitwünschen: das Profil hängt sich an und wird ebenfalls benachrichtigt; (b) nur den
  Zustand zeigen.
- **Empfehlung: (a).** Das zweite Profil will vor allem wissen, wann es schauen kann. Slouchcat ruft Jellyseerr
  dafür nicht auf.
- **Andere Wahl:** (b) das zweite Profil erfährt nichts, ausser es schaut selbst nach.

### F-8 Spotlight mit fehlenden Titeln

- **Worum:** Zeigt die Bühne der Startseite auch Titel, die noch fehlen, damit man sie anfordern kann?
- **Möglichkeiten:** (a) Schalter ab Werk aus, höchstens 5 fehlende von 20, Trakt-Reihenfolge, erste Folie immer
  vorhanden; (b) ab Werk an; (c) andere Höchstzahl.
- **Empfehlung: (a).** Nach dem Update sieht die Startseite gleich aus, bis Marco einschaltet. Die erste Folie
  bleibt vorhanden, weil Vormalen und Vorladen ein Bild aus der Bibliothek brauchen.
- **Andere Wahl:** (b) ändert die Startseite nach dem Update ungefragt für alle Profile, die anfordern können.

### F-9 Wer fehlende Titel sieht

- **Worum:** Sieht ein Profil, das nicht anfordern kann, fehlende Titel trotzdem (ohne Knopf)?
- **Möglichkeiten:** (a) nein: fehlende Titel sieht nur, wer sie auch anfordern kann; (b) ja, ohne Knopf, mit Hinweis.
- **Empfehlung: (a), eine Regel überall** (Redaktionsentscheid vom 17. November 2026). Fehlende Titel sieht ein
  Profil nur, wenn alles zutrifft: Wünsche aktiv, Jellyseerr eingerichtet und eingeschaltet, Konto und Profil haben
  das Recht, eine Zuordnung ist vorhanden (eigene, des Kontos oder Sammelbenutzer), die Art ist erlaubt, die
  Altersstufe passt. Sonst existieren sie für das Profil nicht (404), auch nicht mit «Hinweis statt Knopf». Einzige
  Ausnahme: Titel, zu denen das Profil selbst einen Wunsch hat (jeder Zustand), bleiben auf der Vorschauseite und
  unter «Meine Wünsche» sichtbar. Ein Konto mit Recht, aber ohne Zuordnung bemerkt der Administrator in der
  Verwaltung (Übersicht, Zuordnung, Konten), nicht der Zuschauer. Ist Jellyseerr nur vorübergehend nicht
  erreichbar oder lehnt den Schlüssel ab, bleiben fehlende Titel sichtbar, und Wünsche gehen ins Ausgangsfach.
  Ein Kind ohne Recht sähe sonst laufend Titel, die es nicht bekommen kann (`02-funktionskonzept.md`, Abschnitt 3;
  `04-benutzer-profilmapping.md`, Abschnitt 9).
- **Andere Wahl:** (b) mehr Unruhe auf Startseite und Suche, und ein Knopf ohne Wirkung wäre nur Werbung; der
  Jugendschutz gilt trotzdem.

### F-10 Bestätigungsdialog

- **Worum:** Geht der Wunsch beim ersten Druck weg, oder erst nach einem Dialog?
- **Möglichkeiten:** (a) ein Dialog mit Poster, Jellyseerr-Benutzer, Hinweis zur Freigabe, Staffelwahl; (b) sofort
  mit «Rückgängig».
- **Empfehlung: (a).** Der Dialog sagt vorher, ob der Wunsch sofort beschafft wird oder eine Freigabe braucht, und
  trägt die Staffelwahl.
- **Andere Wahl:** (b) schneller, aber ohne Staffelwahl; ein versehentlicher Wunsch liegt schon in Jellyseerr.

### F-11 Suche nach fehlenden Titeln

- **Worum:** Woher kommen die Treffer «Noch nicht in der Bibliothek» in der Suche?
- **Möglichkeiten:** (a) TMDB-Suche über Slouchcats eigenen Schlüssel; (b) Suche von Jellyseerr.
- **Empfehlung: (a).** Funktioniert auch, wenn Jellyseerr nicht erreichbar ist, nutzt dieselben Daten wie die
  Vorschauseite und dieselbe Jugendschutzregel. Jellyseerr fragt ohnehin TMDB (Anhang D 6.2).
- **Andere Wahl:** (b) die Suche hängt an Jellyseerr, und jede Eingabe belastet es.

### F-12 Serien und Staffeln

- **Worum:** Wie fordert man Serien an?
- **Möglichkeiten:** (a) «Alle fehlenden Staffeln» oder einzelne Staffeln, keine einzelnen Folgen, fehlende
  Staffeln auf der Serienseite; (b) nur ganze Serien.
- **Empfehlung: (a).** So arbeitet Jellyseerr (staffelweise), und wer eine neue Staffel will, findet den Knopf auf
  der Seite der Serie.
- **Andere Wahl:** (b) Wünsche werden unnötig gross; fehlende Staffeln vorhandener Serien bleiben unsichtbar.

### F-13 Was «verfügbar» heisst

- **Worum:** Wann bekommt das Profil «Dein Wunsch ist da»?
- **Möglichkeiten:** (a) wenn Slouchcat den Titel für dieses Profil abspielen kann; (b) wenn Jellyseerr «verfügbar»
  meldet.
- **Empfehlung: (a).** Jellyseerr meldet «verfügbar» nach seinem Radarr- oder Sonarr-Scan (Vorgabe täglich
  04:00), oft Stunden nach dem Import; das sagt nichts darüber, ob Slouchcat die Datei schon eingelesen hat
  (Anhang D 11.1). Ein Kind ohne passende Freigabe sähe den Titel noch nicht. Bei Serien gilt eine Staffel erst als
  verfügbar, wenn sie vollständig abspielbar ist (`06-benachrichtigungen.md`, Abschnitt 3.3).
- **Andere Wahl:** (b) die Meldung kommt zu früh, der Link führt ins Leere.

### F-14 Kanal-Ereignisse und Vorgaben

- **Worum:** Welche Häkchen gibt es auf der Kanalseite, und was ist ab Werk an?
- **Möglichkeiten:** (a) drei Ereignisse: «verfügbar» (an), «freigegeben, abgelehnt, fehlgeschlagen» (aus),
  «Freigabe nötig» (an, nur Administratoren); (b) `new_content` wiederverwenden; (c) ein Ereignis für alles.
- **Empfehlung: (a).** Eigene Häkchen lassen sich getrennt abwählen. Ein gewöhnliches Konto hat `new_content` ab
  Werk auf keinem Kanal (`server/queries/system/notify_channels.sql:7-18`); mit (b) käme die Meldung nur im Eingang an.
- **Andere Wahl:** (b) kein eigener Schalter; (c) wer nur «verfügbar» will, bekommt auch Ablehnungen.

### F-15 PIN-Profile in externen Nachrichten

- **Worum:** E-Mail, Telegram und Push gehen an das Konto, Push an alle Geräte. Soll ein PIN-geschütztes Profil dort
  seinen Titel zeigen?
- **Möglichkeiten:** (a) ohne Titel («[Sandra] Ein Wunsch ist jetzt verfügbar.»); (b) mit Titel wie heute bei
  neuen Folgen.
- **Empfehlung: (a)**, abschaltbar in den Einstellungen der Wünsche. Grund: Externe Kanäle gehören dem Konto und
  erreichen jedes Gerät des Kontos, auch das eines Kindes (Push geht an alle Geräte). Die PIN-Regel betrifft nur
  diese Kanäle. Innerhalb des Kontos sieht das Hauptprofil die Wünsche aller Profile (`listAccountWishes`), auch
  die von PIN-Profilen. Das ist gewollt: Das Hauptprofil verwaltet das Konto, setzt und entfernt PINs und sieht
  Schauzeit und Einstellungen aller Profile (`06-benachrichtigungen.md`, Abschnitt 8).
- **Andere Wahl:** (b) der Titel steht auf jedem Gerät des Kontos, auch auf dem Tablet der Kinder.

### F-16 Freigeben und Ablehnen in Slouchcat

- **Worum:** Muss der Administrator für jede Freigabe in Jellyseerr wechseln?
- **Möglichkeiten:** (a) Knöpfe «Freigeben» und «Ablehnen» in der Verwaltung, die an Jellyseerr durchreichen, in
  Stufe 4; (b) nur in Jellyseerr.
- **Empfehlung: (a).** Spart einen Wechsel; die Entscheidung bleibt in Jellyseerr gespeichert. Es ist kein eigener
  Freigabe-Ablauf, nur ein zweiter Knopf für dieselbe Aktion.
- **Andere Wahl:** (b) etwas weniger Bau; der Administrator arbeitet in zwei Oberflächen.

### F-17 Zurückziehen

- **Worum:** Darf ein Profil einen Wunsch zurücknehmen?
- **Möglichkeiten:** (a) ja, solange er nicht freigegeben ist; (b) nein.
- **Empfehlung: (a).** Jellyseerr erlaubt das Löschen eigener offener Anfragen (Anhang D 8); danach beschafft
  Radarr schon.
- **Andere Wahl:** (b) versehentliche Wünsche bleiben liegen, bis ein Administrator sie ablehnt.

### F-18 Jellyseerr-Webhook

- **Worum:** Slouchcat fragt alle 5 Minuten nach. Soll Jellyseerr zusätzlich sofort Bescheid geben?
- **Möglichkeiten:** (a) Webhook als Anstoss in Stufe 4, Grundlage bleibt der Abgleich; (b) kein Webhook; (c)
  Webhook als Grundlage.
- **Empfehlung: (a).** Der Webhook von Jellyseerr hat ein einziges Ziel, keine Signatur, keine Wiederholung und
  keine Benutzer-ID (Anhang D 9). Als Anstoss schadet ein Verlust nicht.
- **Andere Wahl:** (b) bis zu 5 Minuten Verzug. (c) jede verlorene Meldung wäre ein verlorener Zustand.

### F-19 Ort in der Verwaltung

- **Worum:** Wo stehen Verbindung, Anfragen und Zuordnung?
- **Möglichkeiten:** (a) Verbindung unter Integrationen, Seite «Wünsche» unter Medien; (b) alles unter
  Integrationen; (c) ein eigener Bereich.
- **Empfehlung: (a).** Einrichten bei den anderen Diensten, tägliche Arbeit bei den Medien (`07-administration.md`,
  Abschnitt 1.2).
- **Andere Wahl:** (b) mischt Einrichten und Liste; (c) widerspricht den sechs Bereichen aus ADR 0035.

### F-20 Startreihe «Deine Wünsche»

- **Worum:** Bekommt die Startseite eine Reihe mit den eigenen Wünschen?
- **Möglichkeiten:** (a) ja, ausblendbar wie jede Reihe; (b) nein, nur «Meine Wünsche» in der Seitenleiste.
- **Empfehlung: (a).** Man sieht beim Öffnen, was angekommen ist. Leer erscheint sie nicht.
- **Andere Wahl:** (b) Wünsche nur über die Seitenleiste und die Glocke.

### F-21 Bestehende Anfragen übernehmen

- **Worum:** In Jellyseerr liegen oft schon Anfragen. Zeigt Slouchcat sie dem passenden Profil?
- **Möglichkeiten:** (a) ja, für zugeordnete Benutzer: dem Profil mit eigener Zuordnung, sonst dem Hauptprofil,
  gekennzeichnet «In Jellyseerr angelegt»; (b) nein, nur Wünsche aus Slouchcat.
- **Empfehlung: (a)**, in Stufe 4. Wer in Jellyseerr gewünscht hat, bekommt die Meldung «verfügbar» auch über
  Slouchcat. Anfragen nicht zugeordneter Benutzer bleiben «nicht zuordenbar». Die Übernahme läuft als eigener Job
  `wishes.import`, ausgelöst beim Setzen oder Ändern einer Zuordnung und von Hand
  (`GET /request?requestedBy=<Benutzer>` als Benutzer 1, `03-architektur.md`, Abschnitt 7.3).
- **Andere Wahl:** (b) Wünsche aus Jellyseerr bleiben in Slouchcat unsichtbar; doppelte Wünsche fängt der
  Mitwunsch trotzdem ab.

### F-22 Stufen

- **Worum:** In welcher Reihenfolge wird gebaut, mit je einem Release?
- **Möglichkeiten:** (a) vier Stufen: Fundament, Anfordern, Spotlight, Serien und Komfort (Anhang F 10);
  (b) alles in einem Release.
- **Empfehlung: (a).** Stufe 1 ändert für Zuschauer nichts und lässt Marco die Zuordnung in Ruhe einrichten;
  jede Stufe ist für sich nützlich und prüfbar.
- **Andere Wahl:** (b) ein grosser Release mit langer Prüfung; Fehler in der Zuordnung zeigten sich erst am Ende.

### Teilfragen T-1 und T-2 (aus der ersten Kritik)

Zwei Festlegungen gehen über Anhang F hinaus und waren in Fassung 1 nicht als Frage gekennzeichnet (Kritik,
Befunde 30 und 40). Sie gehören zu F-3 bzw. F-17.

**T-1 «Wünsche aktiv» nur mit wirksamem `X-API-User` (zu F-3).**

- **Worum:** Darf der Administrator «Wünsche aktiv» einschalten, wenn der letzte Verbindungstest nicht gelang oder
  «Handeln im Namen von Benutzern» nicht wirkt?
- **Möglichkeiten:** (a) nein, der Schalter ist gesperrt, bis der Test gelingt; (b) ja, der Test warnt nur.
- **Empfehlung: (a).** Ohne wirksamen `X-API-User` liefen Wünsche als Benutzer 1 und wären sofort freigegeben.
- **Andere Wahl:** (b) Anhang F verlangt nur die Meldung; der Administrator trüge das Risiko selbst.

Die Vorgabe «Wünsche aktiv = aus» ist ein Redaktionsentscheid vom 17. November 2026 und steht mit den übrigen Empfehlungen zur Freigabe.

**T-2 Frist der Übermittlung, solange «Jellyseerr verwenden» aus ist (zu F-17 und zum Ausgangsfach).**

- **Worum:** Wünsche `queued` scheitern nach 24 h. Zählt die Frist weiter, wenn der Administrator Jellyseerr
  bewusst ausschaltet?
- **Möglichkeiten:** (a) die Frist ruht, solange «Jellyseerr verwenden» aus ist; (b) beim Ausschalten werden alle
  `queued`-Wünsche sofort `failed` mit Grund `disabled`.
- **Empfehlung: (a).** Ausschalten ist eine Pause, kein Fehler; die Wünsche gehen nach dem Einschalten weiter.
- **Andere Wahl:** (b) klarer Abschluss, aber die Profile müssten neu anfordern.

## 2. Frühere Entscheide und Regeln, die sich ändern

**Dieses Papier ändert keine Regel und keine bestehende Datei.** Die Spec wird nach `docs/spec/README.md` nicht
umgeschrieben; der neue Wortlaut gilt über Frage 33 und ADR 0037 (Vorgehen wie ADR 0015 und ADR 0018, Abschnitt
«Spec-Wortlaut, der hier abgelöst wird»). Die übrigen Dateien ändert Stufe 0 nach der Freigabe.

### 2.1 Spec `docs/spec/01-anforderungen-v1.md`

| Stelle | Wortlaut heute | Neu (Vorschlag) |
|---|---|---|
| `:35` Statuszeile Integrationen | «Statusanzeige, Wünschen über Seerr, SABnzbd und die Steuerung der Arr-Dienste zurückgestellt (Runden 12 und 20)» | Zusatz: «Ausnahme seit {{Datum}}: Wünsche über Jellyseerr (Frage 33, Konzept 25, ADR 0037)» |
| `:46` Constraint | «Wünschen von Inhalten erfolgt ausserhalb von Slouchcat direkt in Seerr (Overseerr/Jellyseerr); Slouchcat baut weder eine eigene Wunschverwaltung noch einen Seerr-Bereich (Runde 20).» | «Wünsche verwaltet Jellyseerr. Slouchcat legt sie dort im Namen des zugeordneten Benutzers an, zeigt ihren Zustand und meldet die Verfügbarkeit (Frage 33); eine eigene Wunschverwaltung baut es nicht.» |
| `:65` Non-Goal | «Freigabe-Anfragen von Kinderprofilen an den Hauptbenutzer.» | **bleibt.** Klarstellung in ADR 0037: Die Freigabe eines Wunsches entscheidet Jellyseerr; das Durchreichen von «Freigeben/Ablehnen» für Administratoren (F-16) ist kein Ablauf «Kind fragt Eltern». |
| `:66` Non-Goal | «Eigene Wunschverwaltung und ein Bereich „Wünschen“ über die Seerr-API in Slouchcat; Wünsche entstehen direkt in Seerr (Runde 20).» | Zusatz: «Ausnahme seit {{Datum}}: Wünsche über Jellyseerr (Frage 33, ADR 0037). Jellyseerr bleibt die Wunschverwaltung; Slouchcat legt an, zeigt den Zustand, zieht zurück und reicht für Administratoren Freigeben und Ablehnen durch.» |
| `:67` Non-Goal | «Beschaffungsstatus aus Radarr/Sonarr am Titel und in der Suche („gewünscht“, „in Beschaffung“, „verfügbar“) sowie die Benachrichtigung bei Verfügbarkeit (Runde 20, Kandidaten für die Ideensammlung).» | Zusatz: «Ausnahme seit {{Datum}}: der Zustand **eigener Wünsche** aus Jellyseerr und die Benachrichtigung, wenn ein gewünschter Titel in Slouchcat abspielbar ist (Frage 33). Ein Beschaffungsstatus aus Radarr/Sonarr für andere Titel und der Suchfilter «in Beschaffung» bleiben Nicht-Ziele.» |
| `:68` Non-Goal | «Download-Fortschritt aus SABnzbd am Titel (Runde 20).» | **bleibt** |
| `:69` Non-Goal | «Steuerung von Radarr, Sonarr, SABnzbd aus Slouchcat heraus (Qualitätsprofile, Warteschlangen).» | **bleibt**; ADR 0037 hält fest: Slouchcat spricht nur Jellyseerr an. |
| `:159` Hinweis | «I-1 bis I-4 und I-6 … sind in Runde 20 entfallen … Die Nummern bleiben unbesetzt» | bleibt; **Vorschlag:** neue Kriterien I-9 (Wunsch anlegen und Zustand), I-10 (Benachrichtigung bei Verfügbarkeit), I-11 (fehlende Titel im Spotlight und in der Suche) in ADR 0037 |
| `:176`, `:182`, `:185` | Annahmen und offene Punkte «Wünschen via Seerr in Runde 20 gestrichen», «Seerr-Einbettung: entfallen» | bleiben als Geschichte; ADR 0037 verweist darauf. Einbettung bleibt entfallen. |
| `:120` E-4, `:149` A-6 | E-4 «… Verfügbarkeit (lokal, Download, in Beschaffung)»; A-6 «… für neue Inhalte, Beschaffungsstatus, Fehler und Einladungen» | veraltete Reste aus Frage 10; **Vorschlag:** A-6 gilt mit den Ereignissen der Wünsche als erfüllt; «in Beschaffung» in E-4 bleibt gestrichen |

### 2.2 Plan, Protokoll, ADRs, `CLAUDE.md`

| Stelle | Wortlaut heute | Neu (Vorschlag) |
|---|---|---|
| `CLAUDE.md:57-59` Regel 11 | «… algorithmische Empfehlungen, Trakt-Login, eigene Wunschverwaltung, Steuerung von Arr/Seerr, Mandanten …» | Abschnitt 6 |
| `docs/planning/03-plan-v1.md:30` | «Nicht eingeplant …: … eigene Wunschverwaltung und Seerr-Bereich zum Wünschen, Beschaffungsstatus am Titel, …, Steuerung von Radarr/Sonarr/SABnzbd/Seerr …» | Zusatz «(Ausnahme seit {{Datum}}: Wünsche über Jellyseerr, Frage 33, ADR 0037)»; «Steuerung … Seerr» wird «Verwaltung von Jellyseerr (Benutzer, Rechte, Einstellungen)» |
| `docs/planning/03-plan-v1.md:286-292` 5.1 | «entfallen … Slouchcat bindet Seerr gar nicht an, weder über die API noch als Einbettung … AN-5 (Wunschrecht) und AN-16 (Kontozuordnung)» | Nachtrag: «Mit Frage 33 neu gefasst: Anbindung über die API ohne Einbettung; AN-5 lebt als Wunschrecht je Konto und Profil, AN-16 als Zuordnung C (ADR 0037).» |
| `docs/planning/03-plan-v1.md:308-310` 5.5 | «Ein Ereignis für ein Profil geht an die Kanäle des Kontos mit Profilname als Präfix … Vor dem Versand prüft `notify` über `visibility`, ob das Profil das Item sehen darf» | bleibt; Zusatz: «Bei Wünschen nennt die externe Nachricht eines PIN-Profils keinen Titel (F-15).» |
| `docs/planning/03-plan-v1.md:312-316` 5.6 | «Der Webhook ist die einzige Verbindung zu den Arr-Diensten und rein eingehend.» | bleibt für Radarr/Sonarr; Zusatz: «Zu Jellyseerr baut Slouchcat eine ausgehende Verbindung (ADR 0037); Radarr und Sonarr spricht es weiter nie an.» |
| `docs/planning/03-plan-v1.md:164` 3.4 Pkt. 5 | «Ereignis `item.available` an `notify`» (im Code nicht vorhanden) | **Vorschlag:** Hinweis, dass der Haken nach dem Match (`WithMatched`) diese Rolle für Wünsche übernimmt |
| `docs/planning/04-entscheidungen.md:19` Frage 10 | Vorschlag damals «Profile des Hauptkontos ab FSK 12 dürfen wünschen, Gäste nicht; jedem Konto wird ein bestehender Seerr-Benutzer zugeordnet»; Antwort «… Statusanzeige und Wünschen als Kandidaten für die Ideensammlung» | bleibt als Geschichte; Frage 33 sagt, dass Wünschen damit aus der Ideensammlung übernommen ist. Webhooks: zur Art Radarr/Sonarr kommt in Stufe 4 die Art Jellyseerr. |
| `docs/planning/04-entscheidungen.md:37`, `:55` | Frage 5: «Wünschen über eigene Seite via Seerr-API ohne Einbettung»; M9: «I-1, I-2, I-3, I-4, I-6 … ersatzlos gestrichen» | bleiben als Geschichte |
| S-4: `docs/planning/04-entscheidungen.md:20`, `docs/planning/05-plan-spotlight.md:84-85, 291, 1622`, `docs/adr/0011-spotlight.md:99` | «Titel ohne Bibliothekstreffer erscheinen **nie** auf der Startseite (Non-Goal «eigene Wunschverwaltung») …, in der **Verwaltung** aber schon»; ADR 0011 verwirft den Schalter «auch Titel ohne Bibliothekstreffer zeigen» | **abgelöst durch ADR 0038:** fehlende Titel erscheinen nur mit eingeschaltetem Schalter und nur für Profile mit Wunschrecht; die erste Folie ist immer vorhanden |
| `docs/adr/0009-administration-benachrichtigungen.md:36` und Nachtrag `:62-67` | Kanal-Ereignisse `new_content`, `errors`, `invitations`, `announcements`; `acquisition` entfiel, «ohne Erzeuger wäre der Kanal eine Schaltfläche ohne Wirkung» | Nachtrag: drei neue Ereignisse `wish_available`, `wish_status`, `wish_review` mit echtem Erzeuger (`wishes`); fünf neue Eingangsarten |
| `docs/adr/0010-webhook-empfang.md:11-13`, `:64-65` | «M9 ist auf den eingehenden Webhook zusammengeschnitten (Frage 10)»; «Die CSRF-Ausnahme ist die erste ihrer Art …» | Nachtrag in Stufe 4: Art `jellyseerr` neben `radarr`/`sonarr` (CHECK in `webhook_integrations`), nur als Anstoss des Abgleichs. Dieselbe Route `/webhooks/{token}`, darum **keine** zweite CSRF-Ausnahme. |
| `docs/adr/0008-entdecken-suche-trakt.md:82` | «Offen: Filter «in Beschaffung» (M9) …» | veraltet; keine Änderung nötig |
| `docs/ideas/README.md:31-37` (Index) | kein Eintrag zu Wünschen | **Vorschlag:** neue Zeile «06 Wünsche über Jellyseerr · Übernommen (Konzept 25, Frage 33, ADR 0037/0038)» |
| `server/internal/integrations/integrations.go:5-6`, Text `webhooks.intro` | «Slouchcat never calls Radarr or Sonarr …»; «… fragt die Dienste nie von sich aus ab.» | stimmen weiter (Jellyseerr ist nicht Radarr/Sonarr); nur prüfen |

### 2.3 Was ausdrücklich Nicht-Ziel bleibt

| Bleibt Nicht-Ziel | Stelle heute |
|---|---|
| Steuerung von Radarr, Sonarr, SABnzbd (Qualitätsprofile, Warteschlangen, Zielordner, Tags, 4K) | Spec `:69` |
| Download-Fortschritt aus SABnzbd | Spec `:68` |
| Eigene Beschaffung; Beschaffungsstatus für Titel, die niemand gewünscht hat; Suchfilter «in Beschaffung» | Spec `:67`, `:120` |
| Freigabe-Anfragen von Kinderprofilen an den Hauptbenutzer als eigener Slouchcat-Ablauf | Spec `:65` |
| Benutzer, Rechte, Kontingente oder Einstellungen in Jellyseerr verwalten (auch `POST /user`) | neu in ADR 0037 |
| Einbettung von Jellyseerr (iFrame) | Spec `:185` |
| Trakt-Login, Scrobbling, Wünsche aus Trakt-Merklisten | Spec Non-Goals, Regel 11 |
| Algorithmische Empfehlungen: fehlende Titel kommen nur aus der Trakt-Liste des Spotlights und aus der Suche | Spec Non-Goals |
| Interaktion zwischen Profilen: kein «wer hat noch gewünscht» | Spec Non-Goals, ADR 0028 |
| Seite «Entdecken» | Anhang F 2.14 |
| Wünsche offline | Anhang F 2.20 |

## 3. Entwurf: Eintrag Frage 33 im Entscheidungsprotokoll

Zeile für die Tabelle «Weitere Entscheide während der Umsetzung» in `docs/planning/04-entscheidungen.md`
(Spalten Datum, Entscheid, Begründung, Betrifft), im Stil der Zeile zu Frage 32. Der Entwurf nimmt an, dass Marco
allen Empfehlungen folgt; jede abweichende Antwort ersetzt ihren Satz.

```text
| {{Datum}} | **Konzept 25 «Wünsche über Jellyseerr» freigegeben: Zuschauer fordern fehlende Filme und Serien an, Jellyseerr bleibt die Wunschverwaltung, Slouchcat meldet die Verfügbarkeit** (Frage 33, `docs/planning/25-medienwuensche/`, Fassung {{n}}; PR #{{n}}; ADR 0037 und ADR 0038 reserviert, sie entstehen mit Stufe 0). **F-1:** Funktion und Seiten heissen «Wünsche», Knöpfe «anfordern». **F-2:** Zuordnung C: jedes Konto ein Jellyseerr-Benutzer als Standard, einzelne Profile mit eigenem Benutzer; ein Jellyseerr-Benutzer gehört höchstens einem Konto; Slouchcat schlägt bei gleicher E-Mail vor und ordnet nie selbst zu. **F-3:** Gesendet wird mit `X-API-User` und ohne `userId`; der Verbindungstest prüft es. **F-4:** Konten ohne Zuordnung können nicht anfordern; Sammelbenutzer als Wahl, ab Werk aus. **F-5:** Wunschrecht je Konto (Administrator) als Deckel und je Profil (Hauptprofil); Besitzerkonto und Hauptprofile an, neue Konten und weitere Profile aus. **F-6:** Kinderprofile mit Recht dürfen anfordern, die Freigabe entscheidet Jellyseerr. **F-7:** Ein zweites Profil wird Mitwünscher. **F-8:** Fehlende Titel im Spotlight nur mit Schalter (ab Werk aus), höchstens 5 von 20, Trakt-Reihenfolge, erste Folie immer vorhanden. **F-9:** Fehlende Titel sieht ein Profil nur, wenn es sie auch anfordern kann (Recht, Zuordnung, Art, Alter); einzige Ausnahme sind Titel mit eigenem Wunsch. **T-1:** «Wünsche aktiv» lässt sich nur mit gelungenem Test einschalten; ab Werk aus. **T-2:** Die Frist der Übermittlung ruht, solange «Jellyseerr verwenden» aus ist. **F-10:** Anfordern über einen Dialog. **F-11:** Suche nach fehlenden Titeln über den eigenen TMDB-Schlüssel. **F-12:** Serien staffelweise, fehlende Staffeln auf der Serienseite. **F-13:** «Verfügbar» heisst: Slouchcat kann den Titel für das Profil abspielen. **F-14:** Kanal-Ereignisse `wish_available` (an), `wish_status` (aus), `wish_review` (an, nur Administratoren). **F-15:** Externe Nachrichten von PIN-Profilen ohne Titel. **F-16:** Freigeben und Ablehnen in der Verwaltung als Durchreichen, Stufe 4. **F-17:** Zurückziehen, solange nicht freigegeben. **F-18:** Jellyseerr-Webhook nur als Anstoss, Stufe 4; Abgleich alle 5 min bleibt die Grundlage. **F-19:** Verbindung unter Integrationen, Seite «Wünsche» unter Medien. **F-20:** Startreihe «Deine Wünsche». **F-21:** Anfragen zugeordneter Benutzer aus Jellyseerr werden übernommen («In Jellyseerr angelegt»). **F-22:** vier Stufen, je ein Release. Damit fallen die Nicht-Ziele Spec Zeile 66 und Zeile 67 in den genannten Teilen; Zeile 65, 68 und 69 bleiben. S-4 aus Frage 11 ist abgelöst. CLAUDE.md Regel 11 nennt die Ausnahme. | Auftrag von Marco vom 16. November 2026 (Wünsche über Jellyseerr anbinden), Analyseberichte und Kritik in `docs/planning/25-medienwuensche/anhang/` (A bis G), Antworten auf F-1 bis F-22 und T-1, T-2, Freigabe von Konzept und Mockups | Frage 33, ADR 0037, ADR 0038; hebt Spec Non-Goals Zeilen 66 und 67 in Teilen auf, ändert Spec Zeilen 35 und 46; löst S-4 (Frage 11, ADR 0011, `05-plan-spotlight.md`) ab; Nachträge zu Frage 10, ADR 0009, ADR 0010, Plan 5.1, 5.5, 5.6; Regeln 2, 5, 6, 7, 8, 9, 10, 11, 12 |
```

## 4. Entwurf ADR 0037 «Wünsche über Jellyseerr»

```markdown
# ADR 0037: Wünsche über Jellyseerr

- Status: Entwurf; angenommen mit Stufe 0 nach Freigabe (Frage 33). **Hebt die Spec-Nicht-Ziele Zeile 66
  und 67 in Teilen auf**; Zeile 65, 68 und 69 bleiben.
- Quelle: Konzept `docs/planning/25-medienwuensche/` (Papiere 01 bis 11), **Frage 33** in
  `docs/planning/04-entscheidungen.md` mit den Antworten F-1 bis F-22
- Vertrag und Migrationen: vergibt der Bau; Stufe 1 nicht brechend, Stufen 2 bis 4 brechend
  (`08-datenmodell-api.md`, Abschnitt 4.6)
- Betroffene Spec-IDs: A-6, I-8; neu I-9, I-10, I-11
- Harte Regeln: 2 (Titelabfragen nur über `visibility`), 5 (Migrationen nach Vorlage), 7 (`x-visibility`,
  Vertragstest), 8 (Tastaturtest), 9 (Texte), 10 (Schlüssel redigiert), 11 (Nicht-Ziel mit Ausnahme),
  12 (Entscheid im Server)

## Kontext

Bis 1.40 bindet Slouchcat Jellyseerr nicht an (Frage 10, Plan 5.1): Wer etwas sehen will, das fehlt, wünscht es
in Jellyseerr. Marco will, dass Zuschauer fehlende Filme und Serien in Slouchcat anfordern, den Zustand sehen
und über Slouchcats Kanäle erfahren, wann der Titel da ist. Jellyseerr soll die Wunschverwaltung bleiben:
Freigaben, Kontingente und die Übergabe an Radarr und Sonarr liegen dort.

Zu entscheiden war: wie Slouchcat im Namen einer Person wünscht, obwohl Jellyseerr einen einzigen Schlüssel und
keine Profile kennt; wie ein Wunsch einem Profil gehört; wann ein Titel als verfügbar gilt; wie ein Wunsch
übersteht, dass Jellyseerr nicht erreichbar ist; und wie ein Client im Heimnetz zu den Regeln für ausgehende
Aufrufe passt.

## Entscheid

1. **Ausnahme vom Nicht-Ziel nur für Wünsche über Jellyseerr.** Slouchcat legt Wünsche in Jellyseerr an,
   liest ihren Zustand, zeigt ihn, zieht zurück und meldet die Verfügbarkeit. Keine eigene Beschaffung, keine
   Qualitätsprofile, Zielordner oder Tags, kein eigener Freigabe-Ablauf. Radarr, Sonarr und SABnzbd spricht
   Slouchcat nie an. Benutzer, Rechte und Einstellungen in Jellyseerr verwaltet es nicht.
2. **Client `internal/seerr`** ohne Datenbank (Vorbild `internal/trakt`): `X-Api-Key` aus den verschlüsselten
   Einstellungen (AES-256-GCM, nie wieder angezeigt, letzte vier Zeichen), Zeitlimit 10 s, höchstens 5 Anfragen/s,
   **keine Weiterleitungen**, nur `http`/`https`. **Ausdrückliche LAN-Erlaubnis:** Der Client läuft nicht über
   `netguard`, weil der Administrator die Adresse setzt (Muster Podcast-Feed des Administrators und SMTP); es
   wird nur diese Adresse angesprochen, nie ein Wert aus einer Antwort.
3. **Im Namen des Benutzers:** `POST /api/v1/request` mit `X-API-User: <Jellyseerr-Benutzer>` und ohne `userId`.
   So gelten Recht, Kontingent und Auto-Freigabe dieses Benutzers. Der Verbindungstest prüft den Header mit
   `GET /auth/me`; wirkt er nicht, lässt sich die Funktion nicht einschalten (T-1). Beim ersten erfolgreichen
   Test speichert Slouchcat einen Fingerabdruck der Instanz; weicht er später ab, halten Übermittlung und Abgleich
   an, bis der Administrator die Zuordnung bestätigt oder löscht.
4. **Zuordnung C:** Jedes Konto hat höchstens einen Jellyseerr-Benutzer als Standard (`wish_account_map`), Profile
   können einen eigenen haben (`wish_profile_map`). Ein Jellyseerr-Benutzer gehört höchstens einem Konto. Der
   Administrator ordnet zu; Slouchcat schlägt bei gleicher E-Mail vor und ordnet nie selbst zu. Je Wunsch
   speichert Slouchcat Konto, Profil, Jellyseerr-Benutzer (eingefroren) und Request-ID, bei Serien je Staffel.
5. **Wunschrecht** je Konto (`accounts.can_request`, Administrator) als Deckel und je Profil
   (`profiles.can_request`, Hauptprofil). Der Server liefert `canRequest` und einen Grund; der Client entscheidet
   nichts. Konten ohne Zuordnung können nicht anfordern, ausser ein Sammelbenutzer ist gewählt. Fehlende Titel
   sieht ein Profil nur, wenn es sie anfordern kann (F-9).
6. **Zustände** `queued`, `pending`, `processing`, `partial`, `available`, `declined`, `failed`, `withdrawn`,
   abgeleitet aus Jellyseerr und der eigenen Bibliothek.
7. **Verfügbar ist, was Slouchcat für das Profil abspielen kann.** Auslöser ist ein Haken nach dem
   Metadaten-Match (IDs geschrieben), dazu stündlich `wishes.reconcile`. Abgleich über TMDB-ID, bei Serien auch
   TVDB- und IMDb-ID. «AVAILABLE» in Jellyseerr löst allein nichts aus. Ein Film ist verfügbar mit der ersten
   abspielbaren Datei; eine Staffel erst, wenn sie vollständig ist (abspielbare Folgen = Folgenzahl laut TMDB,
   bei laufenden Staffeln die bisher ausgestrahlten). Ob es den Titel gibt und wer ihn sehen darf, beantwortet
   `visibility`; `wishes` fragt `items` nicht ab (Regel 2).
8. **Ausgangsfach:** `createWish` speichert lokal (`queued`), versucht sofort (3 s, ohne Vorprüfung) und sonst
   über `wishes.submit` mit eigener Wiederholungsleiter; nach 24 h `failed` und Fehlerereignis an
   Administratoren. Ab dem zweiten Versuch prüft der Job vor dem Senden, ob der Wunsch in Jellyseerr schon
   besteht; 409 «Request for this media already exists.» und 202 zählen als Erfolg. **Zurückziehen** setzt lokal
   `withdrawn`; der Job `wishes.withdraw` sucht einen eigenen Request des Benutzers und löscht ihn, solange er
   `PENDING` ist. Ist er schon freigegeben, bleibt er stehen und der Wunsch wird wieder geöffnet.
9. **Abgleich** `wishes.sync` alle 5 min: `GET /request?sort=modified` absteigend bis zum letzten Abgleich; der
   Zeiger rückt je Seite vor, ein abgebrochener Lauf setzt dort fort. Der erste Abgleich und der nach einem
   Instanzwechsel laufen als Erstabgleich ohne Seitengrenze im Hintergrund. Dazu `GET /request/{id}` für offene
   Wünsche ohne Änderung seit 24 h, höchstens einmal je 24 h je Wunsch. Die Übernahme bestehender Anfragen
   (F-21) läuft als eigener Job `wishes.import`. Der Jellyseerr-Webhook ist nur ein Anstoss (Stufe 4).
10. **Mitwunsch:** In Slouchcat höchstens ein offener Wunsch je Film bzw. Serienstaffel. Ein weiteres Profil wird
    Mitwünscher, ohne Aufruf an Jellyseerr, und wird ebenfalls benachrichtigt. Zuschauer sehen nie, wer sonst
    gewünscht hat.
11. **Benachrichtigungen** nur über Slouchcat: neue Eingangsarten `wish_available`, `wish_season_available`,
    `wish_approved`, `wish_declined`, `wish_failed`; neue Kanal-Ereignisse `wish_available`, `wish_status`,
    `wish_review` (nur Administratoren). Stufe 2 bringt «verfügbar» für Filme und ganze Serien, Stufe 4 die
    Staffel- und Zustandsmeldungen und `wish_review`. Doppelschutz über `wish_requesters.notified_at` und einen
    eindeutigen Schlüssel je Profil. Eingangseinträge ohne Titel in der Bibliothek hängen am Wunsch und sind für
    Anforderer und Mitwünscher sichtbar. Externe Nachrichten von PIN-Profilen nennen keinen Titel.
12. **Freigeben und Ablehnen** für Administratoren als Durchreichen an `POST /request/{id}/approve|decline`
    (Stufe 4). Das ist kein Ablauf «Kind fragt Eltern» (Spec Zeile 65 bleibt).
13. **Verwaltung:** Integrationen › Jellyseerr (Verbindung, Test, Abgleich, Protokoll) und Medien › Wünsche
    (Anfragen, Zuordnung, Einstellungen); jede Einstellung genau ein Ort. Verwaltungsantworten tragen keine
    `ItemRef`; `listAccountWishes` ist `scoped` und verlangt das Hauptprofil (Regel 7).
14. **Nur online.** `/api` bleibt NetworkOnly; Wünsche haben keinen Offline-Zustand.

## Verworfen

| Alternative | Grund |
|---|---|
| eigene Wunschverwaltung oder eigener Freigabe-Ablauf in Slouchcat | doppelt zu Jellyseerr; Spec Zeile 65 |
| Radarr/Sonarr direkt ansprechen | Spec Zeile 69; Jellyseerr kann es schon |
| `userId` im Body statt `X-API-User` | jeder Wunsch wäre sofort freigegeben, weil Jellyseerr die Auto-Freigabe am Aufrufer prüft |
| ein Jellyseerr-Benutzer je Profil (B) | künstliche E-Mail-Adressen, Arbeit in Jellyseerr je Profil, Anfragen gehen mit dem Profil verloren |
| ein Jellyseerr-Benutzer je Konto ohne Ausnahme (A) | Kinder erben die Auto-Freigabe der Eltern |
| Jellyseerr als Iframe einbetten | entfiel schon in Runde 20; fremde Oberfläche, eigene Anmeldung |
| Benutzer in Jellyseerr anlegen (`POST /user`) | Passwort oder SMTP in Jellyseerr nötig, Rechte separat; Verwaltung von Jellyseerr bleibt Nicht-Ziel |
| Webhook als Grundlage | ein Ziel, keine Signatur, keine Wiederholung, keine Benutzer-ID |
| «verfügbar» nach dem Zustand in Jellyseerr | Meldung vor der Datei in Slouchcat; Kinderprofile sähen den Titel noch nicht |
| Wünsche über `netguard` | sperrt das Heimnetz, in dem Jellyseerr steht |

## Folgen

- Neue Pakete `internal/seerr` und `internal/wishes`; Regel 2 bleibt unberührt (neue Funktionen in `visibility`,
  Haken aus `metadata`).
- Neue Tabellen nach Vorlage; Neubau von `notifications`, `deliveries`, `home_rows` und für `can_request` von
  `accounts` und `profiles`.
- Neuer Schlüsselname `seerr.api_key` in Redaction-Liste und Test (Regel 10).
- Acht neue Jobarten; Abgleich und Prüfung belasten Jellyseerr alle 5 min mit wenigen Aufrufen.
- `X-API-User` ist undokumentiert. Fällt es in einer künftigen Version weg, meldet der Test es, und die Funktion
  bleibt aus, bis neu entschieden ist.
- Jellyseerr verschickt weiter eigene Meldungen, wenn sie dort eingerichtet sind.

## Spec-Wortlaut, der hier abgelöst wird (ohne die Spec umzuschreiben)

- **Constraint Zeile 46** und **Non-Goal Zeile 66**: Wünsche entstehen auch aus Slouchcat, in Jellyseerr.
- **Non-Goal Zeile 67**: Zustand eigener Wünsche und Benachrichtigung bei Verfügbarkeit sind erlaubt; ein
  Beschaffungsstatus für andere Titel und der Suchfilter «in Beschaffung» nicht.
- **A-6**: Die Kanäle tragen die Ereignisse der Wünsche statt des alten «Beschaffungsstatus».
- **Neu I-9 bis I-11** (Nummern I-1 bis I-4 bleiben unbesetzt).
```

## 5. Entwurf ADR 0038 «Titel ausserhalb der Bibliothek»

```markdown
# ADR 0038: Titel ausserhalb der Bibliothek

- Status: Entwurf; angenommen mit Stufe 0 nach Freigabe (Frage 33). **Löst S-4 ab** (Frage 11,
  ADR 0011: «Titel ohne Bibliothekstreffer erscheinen nie auf der Startseite»). **Ergänzt ADR 0034**
  (Freigabeländer) um Titel ohne Eintrag in `items`.
- Quelle: Konzept `docs/planning/25-medienwuensche/` (vor allem `05-spotlight-integration.md`),
  **Frage 33** (F-8, F-9, F-11, F-12)
- Vertrag und Migrationen: vergibt der Bau
- Betroffene Spec-IDs: E-1, E-4, E-7
- Harte Regeln: 2, 5, 6 (Bilder als `http.Handler`), 7, 12

## Kontext

Bis 1.40 kennt Slouchcat nur Titel der eigenen Bibliothek. Alles hängt an `items`: Freigabe
(`effective_rating`, `item_certifications`), Bilder, Merkliste, Empfehlungen, Eingang. Das Spotlight speichert für
Trakt-Einträge ohne Treffer nur TMDB-ID, Titel und Jahr und zeigt sie nur in der Verwaltung (S-4). Wer einen Titel
anfordern soll, muss ihn aber sehen: mit Poster, Beschreibung, Freigabe und Trailer, im Spotlight, in der Suche
und auf einer eigenen Seite. Dabei darf kein Profil einen Titel sehen, den es nach seiner Altersstufe nicht sehen
dürfte, und der Browser soll keine Bilder direkt bei TMDB holen.

## Entscheid

1. **Eigene Daten für fehlende Titel:** `external_titles` (TMDB: Titel je Sprache in `external_title_texts`, Jahr,
   Beschreibung, Laufzeit, Genres, Freigabe, Poster, Hintergrund, Logo, Trailer; bei Serien Staffeln mit Folgenzahl
   und Datum, Status, nächste Folge). Aufgefrischt von `titles.refresh` (24 h, nach `trakt.refresh`). Fehlende
   Titel kommen nie in `items`.
2. **Jugendschutz im Server:** Die TMDB-Freigabe wird in der Länderreihenfolge nach ADR 0034 gelesen
   (gewähltes Freigabeland, dann DE, AT, CH, GB, US; das erste Land mit bekanntem Wert gilt, dieselbe Funktion
   wie für Bibliothekstitel; innerhalb eines Lands der strengste Wert über die Anbieter) und gegen
   min(Profil, Konto) geprüft. **Unbekannt zählt als 18.** Was ein Profil nicht sehen dürfte oder nicht anfordern
   kann, existiert für es nicht: 404 auf der Vorschauseite und bei Bildern, kein Treffer in Suche und Spotlight
   (F-9; Ausnahme: Titel mit eigenem Wunsch). Die Regel wohnt in `visibility`. Weil die TMDB-Suche keine
   Freigaben liefert, holt der Server für Profile unter 18 die Details der ersten 20 Treffer.
3. **Bildweg:** Bilder fehlender Titel laufen über Slouchcats eigenen Bildspeicher und die Route
   `GET /titles/{media}/{tmdbId}/images/{kind}/{size}` (`http.Handler`, `scoped`, im Vertrag beschrieben), nie
   direkt von TMDB im Browser. Die Verwaltung nutzt die eigene Route `GET /admin/titles/{media}/{tmdbId}/images/…`
   (`admin`), weil Verwaltungsantworten keine scoped Adressen tragen.
4. **Vertrag:** fehlende Titel als eigenes Schema `ExternalTitleRef` neben `ItemRef` (nicht brechend);
   `getExternalTitle` und `searchExternalTitles` sind `scoped`; die Vorschauseite liegt unter
   `/titles/{movie|show}/{tmdbId}`.
5. **Spotlight (löst S-4 ab):** fehlende Titel nur mit eingeschaltetem Schalter (ab Werk aus), höchstens N von
   20 (0 bis 10, Vorgabe 5), in Trakt-Reihenfolge; die erste Folie ist immer ein vorhandener Titel (Vormalen und
   Vorladen bleiben); fehlende zählen nicht für «Ausblenden unter»; Profile, die nicht anfordern können, sehen
   keine; mit «Anfordern aus dem Spotlight» aus erscheinen keine.
6. **Abgleich über mehrere IDs:** Trakt-Einträge speichern neu TVDB- und IMDb-ID; vorhandene Serien, die nur eine
   TVDB-ID haben, gelten so nicht fälschlich als fehlend.
7. **Suche:** Unter der Bibliothekssuche ein Abschnitt «Noch nicht in der Bibliothek» aus der TMDB-Suche über den
   eigenen Schlüssel, gefiltert nach Bibliothek, Freigabe und Wunschrecht. Eine Seite «Entdecken» entsteht nicht.
8. **Serien in der Bibliothek:** `series_catalog` hält die Staffelliste laut TMDB, Status und nächste Folge; damit
   zeigt die Serienseite fehlende Staffeln und «2 von 4 Staffeln vorhanden».
9. **Keine Merkliste, kein Gesehen, kein Empfehlen** für fehlende Titel; diese hängen an Bibliothekstiteln.

## Verworfen

| Alternative | Grund |
|---|---|
| fehlende Titel als Zeilen in `items` | bräche `visible_items`, Scans, Zählungen und alle Fremdschlüssel auf `items` |
| Titeldaten über Jellyseerr (`GET /movie/{id}`) | jeder Aufruf geht ohnehin an TMDB; die Anzeige hinge an der Erreichbarkeit von Jellyseerr |
| Bilder direkt von `image.tmdb.org` im Browser | verrät dem Anbieter die Zuschauer, umgeht den Bild-Cache der App |
| fehlende Titel ohne Freigabe zeigen | widerspricht AN-1 (unbekannt = gesperrt unter 18) |
| fehlender Titel als erste Folie | Vormalen und Vorladen brauchen ein Bild aus der Bibliothek |
| Schalter je Profil für fehlende Titel | das Wunschrecht entscheidet schon (F-9) |

## Folgen

- Neue Tabellen `external_titles`, `external_title_texts`, `external_title_images`, `series_catalog`; `spotlight_entries` und
  `spotlight_config` bekommen Spalten.
- Mehr TMDB-Aufrufe über den bestehenden Limiter; der Bildspeicher wächst um die fehlenden Titel des Spotlights.
- **Vorschlag:** Einträge in `external_titles` ohne Bezug (nicht mehr im Spotlight, kein Wunsch, 30 Tage nicht
  gesucht) räumt `titles.refresh` weg.
- Die Spotlight-Verwaltung zeigt für fehlende Einträge Poster und Aktionen.
```

## 6. Vorschlag für Regel 11 in `CLAUDE.md`

Heute `CLAUDE.md:57-59`. Die Datei wird erst in Stufe 0 nach der Freigabe geändert.

```diff
 11. **Keine Nicht-Ziele bauen** (Spec "Non-Goals": Musik (ausser Live-Webradio, Frage 23), Fotos, Live-TV, TV-Apps, native Apps, Plugins,
-    algorithmische Empfehlungen, Trakt-Login, eigene Wunschverwaltung, Steuerung von Arr/Seerr, Mandanten,
-    Cloud, YouTube-Downloads). Nichts aus einem späteren Meilenstein vorziehen.
+    algorithmische Empfehlungen, Trakt-Login, eigene Wunschverwaltung (ausser Wünsche über Jellyseerr, Frage 33,
+    ADR 0037: anlegen, Zustand zeigen, zurückziehen, Verfügbarkeit melden; Freigeben/Ablehnen nur als Durchreichen
+    für Administratoren), Steuerung von Radarr/Sonarr/SABnzbd, Verwaltung von Jellyseerr (Benutzer, Rechte,
+    Einstellungen), Mandanten, Cloud, YouTube-Downloads). Nichts aus einem späteren Meilenstein vorziehen.
```

Die Regel bleibt eine Zeile in der Liste der harten Regeln. Die Zeile zu Regel 2 braucht keine Änderung, solange
`wishes` keine Titeltabellen abfragt; käme ein neues Paket in die Liste der erlaubten Pakete, wäre das ein eigener
Entscheid in ADR 0037.

## 7. Kritik und Einarbeitung

### 7.1 Urteil der ersten Kritik

Die erste Kritik (Anhang G, 17. November 2026) prüfte Fassung 1 gegen Anhang F,
die Anhänge A bis D, den Code im Worktree und `CLAUDE.md`. **Urteil: REVISE**, 40 Befunde: 1 Blocker, 13 Major,
26 Minor. Fassung 2 arbeitet alle 40 Befunde ein; wo die Redaktionsentscheide vom 17. November 2026 etwas anderes
sagen, gilt der Redaktionsentscheid. Nur Befund 12 ist nicht übernommen.

**Wer hat entschieden?** Die Redaktionsentscheide hat die Hauptsession beim Einarbeiten der Kritik getroffen, damit
die Papiere und das Mockup dasselbe sagen. **Marco hat dazu noch nichts entschieden.** Sie sind Empfehlungen wie alles
andere in diesem Konzept und stehen zusammen mit F-1 bis F-22 und T-1, T-2 zur Freigabe.

### 7.2 Befunde und wo sie eingearbeitet sind

| Nr. | Stufe | Befund (kurz) | Einarbeitung |
|---|---|---|---|
| 1 | Blocker | `ItemRef` und scoped Bildadressen in Antworten, die nicht `scoped` sind (Regel 7) | eingearbeitet in `08-datenmodell-api.md` 4.2 (`AdminWishTitle`, `AdminWish` ohne `ItemRef`), 4.5 (`listAccountWishes` `scoped` mit Hauptprofil, `getAdminExternalTitleImage`), 4.6, 4.7; `03-architektur.md` 15.2, 16; `02-funktionskonzept.md` 5.3; `05-spotlight-integration.md` 8, 9; `10-abnahmekriterien.md` AK-135, AK-136 |
| 2 | Major | Sieht ein Profil mit Recht, aber ohne Zuordnung, fehlende Titel? Drei Fassungen | eingearbeitet **anders als vorgeschlagen** (Redaktionsentscheid): eine Regel überall, fehlende Titel nur bei Anfordern-Können, ohne Zuordnung nicht sichtbar; in `02-funktionskonzept.md` 3, 4.5, 5.1; `04-benutzer-profilmapping.md` 9, 10, 11; `05-spotlight-integration.md` 4 (S-6), 10; `03-architektur.md` 3; `07-administration.md` 2; F-8, F-9 in Abschnitt 1; AK-113, AK-114 |
| 3 | Major | «Anfordern aus dem Spotlight» aus: keine fehlenden Titel oder nur «Details»? | eingearbeitet in `05-spotlight-integration.md` 4 (S-8), 8, 10; `07-administration.md` 7 (auch «aus der Suche» und «auf Detailseiten»); `02-funktionskonzept.md` 4.5, 5.1; AK-129 |
| 4 | Major | Wann ist eine Staffel «verfügbar»? Zwei Regeln | eingearbeitet nach Redaktionsentscheid (Staffel vollständig, Jellyseerr keine Bedingung) in `06-benachrichtigungen.md` 3.3; `03-architektur.md` 3, 8.1, 9.3; `02-funktionskonzept.md` 2, 4.7, 6; `08-datenmodell-api.md` 2.5, 2.9, 2.10; ADR 0037 Entscheid 7; AK-82, AK-125 |
| 5 | Major | «strengster Wert» widerspricht ADR 0034 und dem Code | eingearbeitet in `01-bestandsanalyse.md` 8; `02-funktionskonzept.md` 7; `03-architektur.md` 3, 8.3; `05-spotlight-integration.md` 4 (S-7); `08-datenmodell-api.md` 2.9; ADR 0038 Entscheid 2; AK-28 |
| 6 | Major | Wunschmeldungen ohne `item_id` verschwinden aus dem Eingang | eingearbeitet nach Redaktionsentscheid (sichtbar für Anforderer und Mitwünscher des Wunsches) in `06-benachrichtigungen.md` 8, 11; `08-datenmodell-api.md` 2.11; `03-architektur.md` 2.1, 12; `09-umsetzungsplan.md` 2.6; AK-133 |
| 7 | Major | Abgleich kann sich festfahren (40-Seiten-Grenze ohne Fortschritt) | eingearbeitet **anders als vorgeschlagen** (Redaktionsentscheid: Zeiger je Seite, eigener Erstabgleich ohne Seitengrenze) in `03-architektur.md` 7.1, 7.2, 13; `08-datenmodell-api.md` 2.12; `09-umsetzungsplan.md` 2.5, 9; AK-52, AK-123 |
| 8 | Major | Übernahme bestehender Anfragen (F-21) geht mit dem beschriebenen Abgleich nicht | eingearbeitet als Job `wishes.import` (Redaktionsentscheid) in `03-architektur.md` 2.3, 7.3, 13; `04-benutzer-profilmapping.md` 8.2; `07-administration.md` 6, 12; `08-datenmodell-api.md` 4.5, 5; `09-umsetzungsplan.md` 4.6; F-21; AK-88 |
| 9 | Major | Serien: ein Wunsch kann mehrere Jellyseerr-Requests haben | eingearbeitet nach Redaktionsentscheid (`wish_seasons.seerr_request_id`, nicht eindeutig) in `08-datenmodell-api.md` 2.4, 2.5; `03-architektur.md` 6.2, 6.3, 6.4; `04-benutzer-profilmapping.md` 1; AK-122. Das vorgeschlagene UNIQUE (`seerr_request_id`, `season_number`) entfällt; es gibt nur einen Index |
| 10 | Major | Zurückziehen erzeugt Geisterbestellungen | eingearbeitet nach Redaktionsentscheid (Job `wishes.withdraw`, Sperre je Wunsch, Wiederöffnen) in `03-architektur.md` 2.3, 6.3, 6.6, 8.1, 8.2, 13, 14; `02-funktionskonzept.md` 4.8; `08-datenmodell-api.md` 2.4, 5; `07-administration.md` 5; `09-umsetzungsplan.md` 2.4; AK-116, AK-117 |
| 11 | Major | Wechsel der Jellyseerr-Instanz weist Wünsche falschen Personen zu | eingearbeitet nach Redaktionsentscheid (Fingerabdruck als **Vorschlag**) in `04-benutzer-profilmapping.md` 8.4; `03-architektur.md` 7.4, 14, 15.1; `07-administration.md` 2, 4.1, 6, 13; `08-datenmodell-api.md` 2.1, 2.12, 4.2, 4.5; `09-umsetzungsplan.md` 1.5; AK-107 |
| 12 | Major | Datenschutz: das Hauptprofil liest die Wünsche von PIN-Profilen | **nicht übernommen**, weil das Hauptprofil das Konto verwaltet (PINs setzen und entfernen, Schauzeit und Einstellungen aller Profile) und darum bewusst die Wünsche aller Profile sieht; F-15 schützt externe Kanäle, die jedes Gerät erreichen. Begründung klargestellt in `06-benachrichtigungen.md` 8, `02-funktionskonzept.md` 5.3 und F-15 |
| 13 | Major | Suche nach fehlenden Titeln: Altersprüfung braucht je Treffer einen Detailabruf | eingearbeitet in `03-architektur.md` 16; `02-funktionskonzept.md` 5.2; `08-datenmodell-api.md` 4.5; `01-bestandsanalyse.md` 20 (E-23); ADR 0038 Entscheid 2; AK-115, AK-137 |
| 14 | Minor | 403 «unbekannter Benutzer» und «falscher Schlüssel» nicht zu unterscheiden | eingearbeitet in `03-architektur.md` 6.4, 14; `04-benutzer-profilmapping.md` 8.1 |
| 15 | Minor | Rückfüllung von `wish_review`: drei Zeitpunkte | eingearbeitet in `06-benachrichtigungen.md` 6; `08-datenmodell-api.md` 2.11, 3.2; `09-umsetzungsplan.md` 4.3; AK-63, AK-134 |
| 16 | Minor | ADR 0037 «je Stufe additiv» gegen `08` 4.6 «brechend» | eingearbeitet in ADR 0037 (Kopf), `06-benachrichtigungen.md` 11, `09-umsetzungsplan.md` 9, `README.md` |
| 17 | Minor | Fehlercode bei zu vielen Wünschen | eingearbeitet in `03-architektur.md` 15.4 (`too_many_attempts`) |
| 18 | Minor | Als welches Profil fordert der Administrator in der Spotlight-Verwaltung an? | eingearbeitet in `05-spotlight-integration.md` 6 |
| 19 | Minor | Freigeben als Benutzer 1 oder als zugeordneter Administrator? | eingearbeitet in `04-benutzer-profilmapping.md` 2.2 |
| 20 | Minor | Stufe der Eingangsmeldung «Abgelehnt» | eingearbeitet in `02-funktionskonzept.md` 4.4, 4.6; `06-benachrichtigungen.md` 2 (Stufen nach Redaktionsentscheid) |
| 21 | Minor | Zwei Zustandsdiagramme, verschieden | eingearbeitet: `02-funktionskonzept.md` 2 verweist auf `03-architektur.md` 8.2, das einzige Diagramm |
| 22 | Minor | «aufheben» gegen «Ausnahme» bei Spec-Zeile 46 | eingearbeitet in `09-umsetzungsplan.md` 2 (Schritt 0.1), AK-1 |
| 23 | Minor | «Erneut anfordern» nach `arr_failed` führt in eine Schleife | eingearbeitet in `03-architektur.md` 8.1; `02-funktionskonzept.md` 2; `05-spotlight-integration.md` 2; `07-administration.md` 5 |
| 24 | Minor | Einzelprüfung trifft jeden Lauf dieselben Wünsche | eingearbeitet in `03-architektur.md` 7.1; `08-datenmodell-api.md` 2.4 (`checked_at`); AK-124 |
| 25 | Minor | Sofortversuch mit zwei Jellyseerr-Aufrufen in 3 s | eingearbeitet in `03-architektur.md` 6.1, 6.3, 6.5, 9.1; `02-funktionskonzept.md` 4.4; ADR 0037 Entscheid 8 |
| 26 | Minor | Prioritäten gegen die Regel der Queue | eingearbeitet in `03-architektur.md` 13; `09-umsetzungsplan.md` 2.2, 4.1 |
| 27 | Minor | Ort des Hakens `WithMatched` | eingearbeitet in `03-architektur.md` 11; `06-benachrichtigungen.md` 3.1; `09-umsetzungsplan.md` 2.5 |
| 28 | Minor | Doppelschutz hängt an Zeilen, die nach 90 Tagen gelöscht werden | eingearbeitet in `06-benachrichtigungen.md` 3.2, 7, 11; `08-datenmodell-api.md` 2.5, 2.6; `03-architektur.md` 12; AK-126 |
| 29 | Minor | Der «stündliche Test» hat keinen Job | eingearbeitet in `03-architektur.md` 7.1, 13, 14; AK-4 |
| 30 | Minor | «Jellyseerr verwenden» aus lässt Wünsche nach 24 h scheitern | eingearbeitet als **Vorschlag** «Frist ruht» mit Teilfrage T-2 (Abschnitt 1) in `03-architektur.md` 6.3, 14; `07-administration.md` 4.6; `02-funktionskonzept.md` 4.4; AK-111 |
| 31 | Minor | Gespeicherter Schlüssel geht an eine neu eingetippte Adresse | eingearbeitet in `03-architektur.md` 15.3; `07-administration.md` 4.1; `08-datenmodell-api.md` 4.5; AK-105 |
| 32 | Minor | Mindestversion «Jellyseerr ≥ 2.0», geprüft ist nur 2.7.3 | eingearbeitet in `03-architektur.md` 15.1; `07-administration.md` 4.1; `08-datenmodell-api.md` 4.2; AK-106 |
| 33 | Minor | Unbelegte API-Details (`filter=all`, `externalIds.tvdbId`, `media.seasons[]`) | eingearbeitet in `03-architektur.md` 7.1 (`filter` weggelassen, «nicht verifiziert»); `08-datenmodell-api.md` 5 |
| 34 | Minor | Gelöschtes Konto: «geschlossen» oder «nicht zuordenbar»? | eingearbeitet **anders als vorgeschlagen** (Redaktionsentscheid: Wünsche werden geschlossen): offene `failed` mit `account_deleted`, Request-IDs in den Verlauf, damit der Teilindex frei wird; in `04-benutzer-profilmapping.md` 10; `03-architektur.md` 8.1; `08-datenmodell-api.md` 2.4, 2.6; `07-administration.md` 8; AK-128 |
| 35 | Minor | Gleichzeitiges Anfordern zweier Profile | eingearbeitet in `03-architektur.md` 6.1, 6.5; `09-umsetzungsplan.md` 2.4; AK-121 |
| 36 | Minor | Tote oder ungenaue Belege | eingearbeitet in `01-bestandsanalyse.md` 1, 7, 8, 9; `03-architektur.md` 2.4, 3, 7.2, 10, 13; `05-spotlight-integration.md` 1; `07-administration.md` 4.4; `08-datenmodell-api.md` 2.7, 2.8, 2.12; F-13 |
| 37 | Minor | Verweise ohne Dateinamen und tote Verweise | eingearbeitet in `02-funktionskonzept.md` (Einleitung, 3, 4.7), `05-spotlight-integration.md` 9, `01-bestandsanalyse.md` 1, `README.md` (Jugendschutz «–»). Der Link `mockup-abdeckung.md` bleibt; die Datei entsteht gesondert. Den Ordner `.omc/` im Konzeptordner hat diese Fassung nicht angefasst; er gehört vor dem Commit gelöscht oder in `.gitignore` |
| 38 | Minor | Grundlage im Repo nicht auffindbar | eingearbeitet: Berichte als `anhang/A` bis `anhang/F` im Repo, alle Verweise auf «Bericht», «Bestandsanalyse», «Entscheid-Blatt» und Scratchpad-Pfade umgestellt; `README.md` erklärt den Anhang |
| 39 | Minor | Lücken in den Abnahmekriterien | eingearbeitet in `10-abnahmekriterien.md` AK-105 bis AK-137 sowie AK-4, AK-7, AK-63 |
| 40 | Minor | Unmarkierte Abweichungen von Anhang F | eingearbeitet in `04-benutzer-profilmapping.md` 2.3, `07-administration.md` 4.6, 7, `08-datenmodell-api.md` 2.12 und Teilfrage T-1; «Wünsche aktiv» ab Werk aus ist jetzt Redaktionsentscheid |

### 7.3 Abweichungen von Anhang F

Anhang F bleibt als Zwischenstand stehen und wird nicht nachgeführt. Wo die Papiere davon abweichen, gilt das
Papier.

| Stelle in Anhang F | Dort | Jetzt (massgeblich) |
|---|---|---|
| 2.7 Jugendschutz | «Freigabeland-Reihenfolge nach ADR 0034, strengster Wert» | Länderreihenfolge nach ADR 0034, das erste Land mit bekanntem Wert gilt; strengster Wert nur innerhalb eines Lands über die Anbieter; unbekannt = 18 (`03-architektur.md`, Abschnitt 3). Der Wortlaut in Anhang F ist falsch |
| 2.13, 3 Fehlende Titel sehen | «Profile ohne Wunschrecht sehen keine fehlenden Titel»; «Konto ohne Zuordnung» als Fall am Knopf | Fehlende Titel sieht nur, wer sie anfordern kann (auch Zuordnung, Art, Alter); kein Hinweis statt Knopf; Ausnahme eigene Wünsche (F-9) |
| 6 `listAccountWishes` | `account` | `scoped`, zusätzlich Hauptprofil; Verwaltungsantworten ohne `ItemRef`, eigene Admin-Bildroute |
| 4 Version | Jellyseerr ≥ 2.0 | Jellyseerr ≥ 2.7 bzw. Seerr ≥ 3.0; ältere «nicht geprüft» |
| 4 Hintergrundjobs | sechs Jobarten | acht: dazu `wishes.withdraw` und `wishes.import`; Erstabgleich als Lauf von `wishes.sync` |
| 2.9 Abgleich | Abbruch beim Zeiger; Einzelprüfung offener Wünsche nach 24 h | Zeiger je Seite, Erstabgleich ohne Seitengrenze, Einzelprüfung höchstens einmal je 24 h je Wunsch |
| 5 Datenmodell | `wishes.seerr_request_id` je Wunsch; keine Spalten für Doppelschutz und Instanz | Request-ID bei Serien je Staffel; neu `wish_seasons.tmdb_id`, `seerr_request_id`, `notified_at`; `wish_requesters.notified_at`; `wishes.withdraw_pending`, `checked_at`; `seerr_users.created_at`; Tabellen `seerr_log`, `external_title_images` |
| 9 F-21 | Übernahme ohne eigenen Weg | eigener Job `wishes.import` beim Setzen einer Zuordnung und von Hand |
| 10 Stufen der Meldungen | Stufe 4 «Staffel-Meldungen» und «Ereignis Zustand»; `wish_review` ohne Stufe | Stufe 2 «verfügbar» (Film und ganze Serie); Stufe 4 Staffel, `wish_status`, `wish_review` |
| 4 Einstellungen und Zuordnung | ohne Angabe zum Speichern | Einstellungen gelten sofort; Zuordnung direkt in der Liste; Reiter «Wünsche» nach «Pflege»; Reihe «Deine Wünsche» nach «Weiterschauen» (wie im Mockup) |
| neu | – | Fingerabdruck der Instanz (**Vorschlag**); Zurückziehen über einen Job ohne Geisterbestellung; geschlossene Wünsche machen den Titel wieder anforderbar; «Anfordern aus dem Spotlight» aus heisst keine fehlenden Titel im Spotlight; Konto gelöscht schliesst seine Wünsche |
