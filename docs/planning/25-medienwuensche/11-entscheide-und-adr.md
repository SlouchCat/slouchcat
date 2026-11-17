# Konzept 25 · Entscheide, Regeländerungen und ADR-Entwürfe

> **Kurz gesagt.** Stand Fassung 3 vom 17. November 2026; sie umfasst alle drei Rückmeldungen von Marco an diesem
> Tag. Marco hat elf Fragen entschieden (E-1 bis E-11, Abschnitt 0): gleiches Passwort in Seerr, Profil-Tag in
> Radarr und Sonarr, Seerr zählt das Limit, Verknüpfen über die E-Mail; fehlende Titel der Trakt-Liste
> nirgends in der Zuschauer-App, Webhooks von Radarr und Sonarr wie heute, Download-Fortschritt bei Wünschen; bei
> einer Einladung sofort verknüpfen, der Name «Seerr», die Startreihe «Erfüllte Wünsche», der Sofort-Import nach
> einer Messung. Es gibt die Fragen F-1 bis F-34; entschieden sind F-8 (E-5), F-32 (E-9), F-33 (E-11) und F-34
> (E-10), F-20 ist durch F-34 ersetzt, F-24 war im Teil «Verknüpfen» durch E-8 entschieden. Die übrigen
> 29 Fragen und die Teilfragen T-1, T-2 hat Marco am 17. November 2026 mit dem Bauauftrag beantwortet (Anhang H
> 11.2): meist nach Empfehlung, **gegen die Empfehlung** bei F-4 (Sammelbenutzer ab Werk), F-22 (ein Release),
> F-24 (Passwort nur für eingeladene und eröffnete Benutzer) und F-27 (keine Ausnahme je Profil). Jede Frage
> trägt die Antwort als ersten Punkt. Keine Frage ist mehr offen.
> Die Kritik an Fassung 3 (Anhang H 9, K-1 bis K-23) ist mit Redaktionsentscheiden eingearbeitet, nicht mit
> Entscheiden von Marco; den wichtigsten (K-1: Verknüpfen nur durch einen Administrator, N-2a) hat Marco mit E-8
> anders entschieden. Zu jeder steht hier, worum es geht, welche
> Möglichkeiten es gibt, was empfohlen ist und was eine andere Wahl bedeutet. Wünschen ist heute ein Nicht-Ziel;
> Abschnitt 2 listet jede Stelle in Spec, Plan, Protokoll, ADRs und `CLAUDE.md`, die sich mit einer Freigabe
> ändert, und was ausdrücklich Nicht-Ziel bleibt. Abschnitt 3 nennt den Eintrag Frage 33 im
> Entscheidungsprotokoll (eingetragen am 17. November 2026), Abschnitt 4 und 5 sind die Entwürfe von ADR 0037 und ADR 0038, Abschnitt 6 der Vorschlag
> für Regel 11, Abschnitt 7 die Einarbeitung der Kritik und von Marcos Rückmeldung samt Abweichungen. **Dieses
> Papier ändert keine Regel und keine bestehende Datei.**

> **Hinweis.** Marco hat Konzept 25 am 17. November 2026 freigegeben; **Frage 33** steht in
> `docs/planning/04-entscheidungen.md`. Damit gelten die Ausnahmen aus Abschnitt 2, soweit Frage 33 sie nennt
> (`docs/ideas/README.md:11-12`: «der Non-Goal fällt erst mit einem Eintrag im Entscheidungsprotokoll»). Der
> Wortlaut von `CLAUDE.md` Regel 11 (Abschnitt 6) und der Nachtrag zu ADR 0010 werden in Stufe 0 nachgeführt; die
> ADR-Dateien entstehen ebenfalls in Stufe 0 unter `docs/adr/`. Marco hat den Bau am 17. November 2026 bestellt
> und alle Fragen beantwortet (Nachtrag zu Frage 33, Anhang H 11). Die Entwürfe in den Abschnitten 4 und 5 sind
> auf diese Antworten nachgeführt; massgeblich sind die angenommenen ADR-Dateien.

Grundlage ist [Anhang F](anhang/F-entscheid-blatt.md) (Abschnitte 2, 8, 9, 10) und für Fassung 3
[Anhang H](anhang/H-nachtrag-17-november.md) (Regeln N-1 bis N-104; Abschnitt 11 geht allen früheren vor, Abschnitt 10 den Abschnitten 4 bis 9,
Abschnitt 8 den Abschnitten 4 bis 7). Wortlaute von heute sind im Worktree am Anker
`ba6efe31` nachgesehen. Die übrigen Papiere: `01-bestandsanalyse.md`, `02-funktionskonzept.md`,
`03-architektur.md`, `04-benutzer-profilmapping.md`, `05-spotlight-integration.md`, `06-benachrichtigungen.md`,
`07-administration.md`, `08-datenmodell-api.md`, `09-umsetzungsplan.md`, `10-abnahmekriterien.md`. Die
Analyseberichte, die Kritik und Marcos Rückmeldung liegen in `anhang/`, [Anhang A](anhang/A-spotlight-startseite.md)
bis [Anhang H](anhang/H-nachtrag-17-november.md). Abschnitte mit **Vorschlag** gehen über Anhang F hinaus.

## 0. Am 17. November 2026 von Marco entschieden (E-1 bis E-11)

Marco hat Fassung 2 durchgesehen, vier Ergänzungen verlangt und danach vier Rückfragen beantwortet (Anhang H,
Abschnitte 1 und 2). Danach hat er das Mockup der Fassung 3 angesehen, eine zweite Nachricht geschickt und drei
weitere Rückfragen beantwortet (Anhang H, Abschnitte 8.1 und 8.2). Auf seine Frage «Hast du noch offene Fragen oder
Unklarheiten?» hat er zuletzt vier Weichen entschieden (Anhang H, Abschnitt 10.1). Nur diese elf Antworten und
seine zwei Nachrichten sind Entscheide von Marco. Alles, was die Papiere daraus ableiten, ist Empfehlung der Redaktion und
stand als offene Frage F-23 bis F-31 oder als **Vorschlag** zur Freigabe. Die offenen Fragen hat Marco am selben
Tag beantwortet (Abschnitt 1, Anhang H 11.2).

| Nr. | Frage | Marcos Antwort | Was er dazu gesehen hat (Anhang H 2) | Folge in Fassung 3 (Ausgestaltung: Redaktion) |
|---|---|---|---|---|
| E-1 | Sollen sich eingeladene Benutzer mit E-Mail und Passwort von Slouchcat auch direkt bei Seerr anmelden können? | **«Ja, gleiches Passwort»** | Beim Annehmen der Einladung legt Slouchcat den Seerr-Benutzer mit gleicher E-Mail und gleichem Passwort an; bei jeder Passwortänderung gleicht es ab; das Passwort geht dabei einmal im Klartext an Seerr; ändert jemand es direkt in Seerr, laufen die beiden auseinander | Eröffnen beim Annehmen, Passwort in drei Momenten, nie gespeichert (`03-architektur.md`, Abschnitt 18); Stufe 1 |
| E-2 | Wo soll man sehen, welches Profil einen Titel angefordert hat? | **«Auch in Radarr/Sonarr»** | als Tag am Film oder an der Serie; Slouchcat müsste Radarr und Sonarr direkt ansprechen, das bricht die heutige Regel | Paket `internal/arr` für Tags und das Lesen des Fortschritts (E-7), Job `wishes.tag` (Redaktionsentscheid zu K-22); Ausnahme von ADR 0010 und vom Nicht-Ziel (Abschnitt 2); Stufe 4 |
| E-3 | Das Limit gilt je Konto für alle Profile gemeinsam. Was soll dazu zählen? | **«Alles, Seerr zählt»** | Limit in Slouchcat einstellen, Slouchcat schreibt es in den Seerr-Benutzer; Seerr zählt rollierend, auch direkt dort gestellte Wünsche; abgelehnte Wünsche und Mitwünsche zählen nicht | Limit je Konto mit Vorgabe, Vorprüfung im Dialog, Stand aus Seerr (`03-architektur.md`, Abschnitt 19); Stufe 1, Vorprüfung ab Stufe 2 |
| E-4 | Was soll mit Konten und Seerr-Benutzern geschehen, die es heute schon gibt? | **«Über gleiche E-Mail verknüpfen»** | Gibt es in Seerr einen Benutzer mit derselben E-Mail, verknüpft Slouchcat ihn, statt einen zweiten anzulegen; für Konten ohne Treffer gibt es «In Seerr eröffnen» | automatisches Verknüpfen beim Eröffnen; Knöpfe «In Seerr eröffnen» und «Alle eröffnen» (`07-administration.md`, Abschnitt 6). Die Einschränkung der Redaktion für selbst eingetippte Adressen (N-2a, K-1) hat Marco mit E-8 verworfen |
| E-5 | Fehlende Titel erscheinen nicht mehr im Spotlight der Startseite. Sollen die fehlenden Titel der Trakt-Liste stattdessen unter Wünsche › Entdecken als eigene Reihe stehen? | **«Nein, nirgends»** | Fehlende Titel der Trakt-Liste sieht nur der Administrator in der Verwaltung; anfordern geht über Entdecken, Filme, Serien und Suche | Startseite und Spotlight-Liste nur mit Bibliothekstiteln, S-4 ohne Ausnahme; Verwaltung › Spotlight mit Band «Fehlende Einträge» ohne Schalter; F-8 entschieden; vom Spotlight bleibt der Abgleich über TVDB und IMDb (`03-architektur.md`, Abschnitt 5.1); Stufe 3 |
| E-6 | Radarr und Sonarr melden von sich aus nur per Webhook; über die API muss Slouchcat nachfragen. Wie soll Slouchcat angebunden sein? | **«Webhooks wie heute»** (nicht «API und Webhook», nicht «Nur API, ohne Webhook») | Du richtest die Webhooks weiter von Hand ein; die API nutzt Slouchcat nur für die Profil-Tags | Webhook-Empfang und Seite «Webhooks von Radarr und Sonarr» unverändert; Slouchcat trägt in Radarr und Sonarr keinen Webhook ein (`03-architektur.md`, Abschnitt 10); «API und Webhook» und «Nur API» stehen in ADR 0037 unter «Verworfen» |
| E-7 | Soll Slouchcat bei Wünschen den Download-Fortschritt aus Radarr und Sonarr zeigen, zum Beispiel «Wird heruntergeladen, 45 %, noch 12 Minuten»? | **«Ja, bei Wünschen»** | Unter «Meine Wünsche» und in der Verwaltung; die Anzeige des Beschaffungsstands ist heute ein Nicht-Ziel, das wäre eine weitere Ausnahme | Job `wishes.progress` liest die Warteschlange und den Dateistand der gewünschten Titel, Zwischenspeicher `wish_progress`, vier Anzeigen nur beim Wunsch (`03-architektur.md`, Abschnitt 22); Ausnahme von Spec `:67` nur beim Wunsch und von ADR 0010; Stufe 4 |
| E-8 | Nimmt jemand eine Einladung an und tippt eine E-Mail ein, die es in Seerr schon gibt: Soll Slouchcat den bestehenden Seerr-Benutzer sofort verknüpfen oder erst nach deiner Bestätigung? | **«Sofort verknüpfen»** (nicht «Erst nach Bestätigung», das die Redaktion empfohlen hatte) | Wie in deiner früheren Antwort, auch bei Einladungen. Einfacher, aber wer eine fremde E-Mail eintippt, bekommt den Seerr-Benutzer dieser Person samt ihren Anfragen | N-2a ist zurückgezogen (N-90): kein Zustand «Verknüpfung vorgeschlagen», keine Aktion «Verknüpfung bestätigen», keine Operation dafür. Treffer werden sofort verknüpft, auch nach einer Einladung und nach einer E-Mail-Änderung (N-95); das Passwort geht an den Benutzer wie bei jedem anderen, nie an Benutzer 1, andere Administratoren von Seerr und Benutzer mit Anmeldung über Jellyfin, Emby oder Plex (N-91). Das Risiko ist bekannte Grenze (N-92). Empfehlung der Redaktion dazu: Hinweis «Neu verknüpft» sieben Tage lang in der Übersicht und Eintrag im Protokoll (N-93), gleiche Meldung an das Konto in beiden Fällen (N-94) (`03-architektur.md`, Abschnitte 15.4 und 18); Stufe 1 |
| E-9 | Wie soll der Dienst in der Oberfläche von Slouchcat heissen? | **«Seerr»** (nicht «Jellyseerr») | So heisst das Projekt seit Version 3, und so schreibst du es. Ich benenne Papiere und Mockup um | überall «Seerr»; «Jellyseerr» nur, wo eine Version 2.x gemeint ist, und in den eingefrorenen Anhängen A bis G; das Konzept heisst «Wünsche über Seerr» (N-96); F-32 entschieden |
| E-10 | Die Startseite zeigt nur Vorhandenes. Was soll mit der neuen Startreihe für Wünsche geschehen? | **«Erfüllte Wünsche»** (nicht «Keine Reihe», nicht «Auch offene Wünsche») | Die Reihe zeigt nur Wünsche, die in den letzten 14 Tagen wahr wurden, also Titel der Bibliothek. Offene Wünsche stehen unter «Wünsche» | Reihe `wishes` «Erfüllte Wünsche» (N-74, N-97); `HomeRowKey` + `wishes`, Stufe 3 bleibt im Vertrag brechend; F-34 entschieden |
| E-11 | Die Webhook-Meldung von Radarr und Sonarr nennt schon Dateipfad und Kennung. Soll Slouchcat das für einen Sofort-Import nutzen? | **«Ja, nach Messung»** (nicht «Ja, fest einplanen», nicht «Nein») | Kommt als eigener Schritt in den Bauplan. Zuerst wird gemessen, wo heute die Zeit bleibt. Die Webhooks selbst bleiben, wie sie sind | Schritt 6a im Bauplan (N-98): zuerst eine Messung in `docs/perf/`; zeigt sie keinen Gewinn, entfällt der Schritt; die Webhooks bleiben, wie sie sind (N-80); F-33 entschieden (`03-architektur.md`, Abschnitt 10.1; `09-umsetzungsplan.md`, Abschnitt 6a) |

E-6 und E-7 zusammen heissen: Die API von Radarr und Sonarr dient genau zwei Dingen, dem Profil-Tag und dem Lesen
des Download-Fortschritts (Anhang H 8.2, N-81). Gelesen werden dafür die Warteschlange und der Dateistand der
gewünschten Titel (`queue`, `movie?tmdbId`, `series?tvdbId`; Umfang Redaktion, K-5).

Aus Marcos Nachricht (Anhang H 1) stehen ausserdem fest: Profile brauchen in Seerr kein Login, nur einen Tag
oder Ähnliches; ein Limit je Konto, etwa 10 Filme und 5 Staffeln je Woche rollierend; eine Seite für Wünsche wie
die Startseite von Seerr mit Discover, Movies, Series und Search; Serien sollen über TVDB «konsistent» abgeglichen
werden; die Mockups sind zu ergänzen. Aus seiner zweiten Nachricht (Anhang H 8.1): Im Spotlight soll nicht
angezeigt werden, was fehlt; nur vorhandene Filme und Serien gehören auf die Startseite; die Verwaltung ist «gut
so»; er fragt, ob Radarr und Sonarr direkt angebunden werden sollen, damit der Import «rasendschnell» geht. Wie
das im Einzelnen aussieht (Name des Tags, Vorgabe des Limits, Reihen auf «Entdecken», Reihenfolge des Abgleichs,
Stufe 5, die vier Anzeigen des Fortschritts, der Zuschnitt des Sofort-Imports, der Hinweis «Neu verknüpft»), ist
Redaktion. Die Reihe «Erfüllte Wünsche» und den Sofort-Import nach Messung hat Marco mit E-10 und E-11 gewählt.

**Was E-1 bis E-11 an offenen Fragen klären:**

| Frage | in der Sache geklärt | bleibt offen |
|---|---|---|
| F-2 Zuordnung | Standard ist ein Seerr-Benutzer je Konto, mit demselben Login (E-1); Profile bekommen keinen eigenen Benutzer und kein Login (Nachricht); Möglichkeit (b) entfällt | beantwortet (F-27): keine Ausnahme |
| F-4 Konten ohne Seerr-Benutzer | jedes Konto bekommt seinen Benutzer über Eröffnen oder Verknüpfen (E-1, E-4); der Fall ist nur noch vorübergehend | beantwortet (F-4): Sammelbenutzer «slouchcat-gaeste» ab Werk |
| F-21 Übernahme | verknüpfte Benutzer haben oft schon Requests; die Übernahme läuft nach jedem Verknüpfen | beantwortet (F-21): ja, nach Empfehlung |
| F-8 Spotlight mit fehlenden Titeln | **entschieden durch E-5:** keine fehlenden Titel im Spotlight und nirgends in der Zuschauer-App | – |
| F-20 Startreihe | die Reihe darf nach Marcos zweiter Nachricht nur Vorhandenes zeigen | als F-34 neu gestellt, durch E-10 entschieden |
| F-24 Passwort und wer verknüpft | **im Teil «wer verknüpft» entschieden durch E-8:** sofort, auch bei der Einladung | beantwortet (F-24): Passwort nur für eingeladene und von Slouchcat eröffnete Benutzer; vom Administrator verknüpfte behalten ihres. Der Hinweis «Neu verknüpft» (N-93) bleibt Redaktion |
| F-32 Name | **entschieden durch E-9:** «Seerr» | – |
| F-33 Sofort-Import | **entschieden durch E-11:** ja, nach Messung | – |
| F-34 Startreihe | **entschieden durch E-10:** «Erfüllte Wünsche» | – |
| T-1 | – | beantwortet: nach Empfehlung (a) |

## 1. Fragen F-1 bis F-34 (alle beantwortet am 17. November 2026)

### F-1 Begriffe «Wünsche» und «anfordern»

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** «Wünsche» für die Funktion, «anfordern» auf Knöpfen.
- **Worum:** Wie heisst die Funktion in der Oberfläche? Ein Wort je Sache, überall gleich.
- **Möglichkeiten:** (a) «Wünsche» für die Funktion und Seiten, «anfordern» auf Knöpfen; (b) «Anfragen» und
  «anfragen» wie in Seerr («Request»); (c) «Bestellen».
- **Empfehlung: (a).** «Wünschen» ist das alte Wort in Slouchcats Papieren (Frage 10), klingt nach Haushalt statt
  nach Behörde, und «Film anfordern» sagt auf dem Knopf genau, was passiert. «Bestellen» klingt nach Kauf.
- **Andere Wahl:** Nur Texte und Mockups ändern sich; technisch nichts.

### F-2 Zuordnung: ein Seerr-Benutzer je Konto

- **Beantwortet am 17. November 2026:** in der Sache durch Marcos Nachricht und E-1 entschieden; mit F-27 (Marco, gegen die Empfehlung) gilt (a) ohne Ausnahme (N-100). Anhang H 11.2 führt F-2 nicht eigens auf.
- **Worum:** Mit welchem Seerr-Benutzer sendet Slouchcat den Wunsch eines Profils?
- **In der Sache geklärt (Marco, 17. November 2026):** Jedes Konto hat seinen eigenen Seerr-Benutzer mit
  demselben Login (E-1); die Profile darunter brauchen kein Login, sondern nur einen Tag (Nachricht, E-2). Damit ist
  der Normalfall Möglichkeit (a), und (b) «je Profil ein Benutzer» entfällt.
- **Möglichkeiten für den Rest:** (a) nur je Konto, ohne Ausnahme; (c) je Konto als Standard, einzelne Profile als
  zugeklappte Ausnahme mit einem anderen, schon bestehenden Seerr-Benutzer.
- **Empfehlung: (a) als Normalfall mit (c) als zugeklappter Ausnahme** (Redaktion, F-27). Nur so braucht der Wunsch
  eines Kinderprofils eine Freigabe, wenn das Konto selbst automatisch freigegeben wird. Vergleich in
  `04-benutzer-profilmapping.md`, Abschnitt 4.
- **Andere Wahl:** (a) ohne Ausnahme: Kinder im Konto des Administrators erben dessen Auto-Freigabe und sind ohne
  Limit (N-34).

### F-3 Wunsch als Benutzer über `X-API-User` oder über `userId`

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Slouchcat handelt als der Benutzer (`X-API-User`), der Verbindungstest prüft es.
- **Worum:** Seerr hat einen Schlüssel für alles. Wie sagt Slouchcat, für wen ein Wunsch ist?
- **Möglichkeiten:** (a) Header `X-API-User` (undokumentiert, steht seit Overseerr im Code); (b) Feld `userId` im
  Body (dokumentiert).
- **Empfehlung: (a), mit Prüfung bei jedem Verbindungstest.** Nur so gelten Recht, Limit und Auto-Freigabe des
  Benutzers. Mit `userId` prüft Seerr die Auto-Freigabe am Aufrufer, also am Administrator: Jeder Wunsch
  wäre sofort freigegeben ([Anhang D](anhang/D-jellyseerr-api.md) 3.3).
- **Andere Wahl:** (b) schafft die Freigabe in Seerr faktisch ab; Kinderwünsche gingen ungeprüft an Radarr
  und Sonarr. Entfernt eine künftige Seerr-Version den Header, meldet der Test es; ob «Wünsche aktiv» dann
  sperrt, ist Teilfrage T-1. Danach ist neu zu entscheiden.

### F-4 Konten ohne Seerr-Benutzer

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), gegen die Empfehlung:** **Sammelbenutzer ab Werk.** Slouchcat legt in Seerr den Benutzer «slouchcat-gaeste» an (ohne Anmeldung, ohne automatische Freigabe); über ihn senden Konten ohne eigenen Seerr-Benutzer. Für ihn gilt das Standard-Limit, gemeinsam für alle diese Konten.
- **Worum:** Was passiert mit einem Konto, das (noch) keinen Seerr-Benutzer hat?
- **In der Sache weitgehend geklärt:** Seit E-1 und E-4 bekommt jedes Konto einen Benutzer, durch Eröffnen beim
  Einladen, über die E-Mail oder über «In Seerr eröffnen». Ohne Benutzer bleibt ein Konto nur, solange es
  «wartet» (Seerr nicht erreichbar), solange «Neue Konten in Seerr eröffnen» aus ist, oder bis der
  Administrator ein bestehendes Konto eröffnet.
- **Möglichkeiten:** (a) es kann bis dahin nicht anfordern; (b) es sendet als gemeinsamer Sammelbenutzer.
- **Empfehlung: (a) als Vorgabe, (b) als Wahl.** Ein Sammelbenutzer teilt Limit und Auto-Freigabe zwischen
  Gästen, und in Seerr sieht man nicht mehr, wer gewünscht hat. **Redaktion:** Die Wahl (b) bleibt vorerst,
  weil sie den Übergang für bestehende Konten abdeckt; sie liesse sich streichen, wenn Marco es so will.
- **Andere Wahl:** (b) als Vorgabe: wartende Konten können sofort anfordern, aber unter einem Namen und mit
  gemeinsamem Limit.

### F-5 Wunschrecht je Konto und je Profil

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Wunschrecht je Konto (Administrator, Deckel) und je Profil (Hauptprofil); ab Werk Besitzerkonto und Hauptprofile an, neue Konten und weitere Profile aus.
- **Worum:** Wer darf überhaupt Titel anfordern, und wer stellt das ein?
- **Möglichkeiten:** (a) Schalter je Konto (Administrator) als Deckel und je Profil (Hauptprofil); (b) nur je Konto;
  (c) feste Regel wie in Frage 10 vorgeschlagen («Profile des Hauptkontos ab FSK 12, Gäste nicht»).
- **Empfehlung: (a)** mit den Vorgaben Besitzerkonto an, neue Konten aus, Hauptprofil an, weitere Profile aus. Das
  folgt dem Muster der Altersgrenze (Konto als Deckel, Profil fein) und dem Grundsatz «Niemand hebt seine eigenen
  Grenzen an» (ADR 0014).
- **Andere Wahl:** (b) Eltern können ein einzelnes Kind nicht ausnehmen. (c) passt nicht zu jedem Haushalt und ist
  nicht einstellbar.

### F-6 Kinderprofile und Freigabe

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Kinderprofile mit Recht dürfen anfordern; die Freigabe entscheidet Seerr am Benutzer des Kontos.
- **Worum:** Dürfen Kinderprofile anfordern, und wer gibt frei?
- **Möglichkeiten:** (a) mit Recht ja, die Freigabe entscheidet Seerr über die Auto-Freigabe des zugeordneten
  Benutzers; (b) Kinder nie; (c) eine eigene Freigabe in Slouchcat («Kind fragt Eltern»).
- **Empfehlung: (a).** Seerr hat die Freigabe schon; ein zweiter Ablauf in Slouchcat wäre doppelt und bleibt
  Nicht-Ziel (Spec Zeile 65).
- **Andere Wahl:** (b) nimmt Kindern die Funktion ganz. (c) ist ein neuer, grosser Ablauf und hebt ein weiteres
  Nicht-Ziel auf.

### F-7 Schon angeforderter Titel

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Mitwünschen.
- **Worum:** Ein zweites Profil will einen Titel, den schon jemand gewünscht hat. Seerr sperrt Doppelte.
- **Möglichkeiten:** (a) Mitwünschen: das Profil hängt sich an und wird ebenfalls benachrichtigt; (b) nur den
  Zustand zeigen.
- **Empfehlung: (a).** Das zweite Profil will vor allem wissen, wann es schauen kann. Slouchcat ruft Seerr
  dafür nicht auf.
- **Andere Wahl:** (b) das zweite Profil erfährt nichts, ausser es schaut selbst nach.

### F-8 Spotlight mit fehlenden Titeln

- **Entschieden durch E-5** (Marco, 17. November 2026, zweite Rückmeldung): Im Spotlight erscheinen keine fehlenden
  Titel. Startseite (Bühne und Posterreihe) und Spotlight-Liste zeigen nur Titel der Bibliothek, wie heute. Auch
  als Reihe unter «Entdecken» erscheinen die fehlenden Einträge der Trakt-Liste nicht («Nein, nirgends»). Nur die
  Verwaltung › Spotlight listet sie, mit Poster, Zustand, IDs und «Anfordern» (N-70, N-71).
- **Folge:** Die Schalter «Fehlende Titel zeigen», «Höchstens fehlende Titel», «Reihenfolge» und «Anfordern aus
  dem Spotlight» entfallen, ebenso die Mischregel der Bühne (N-72). S-4 bleibt ohne Ausnahme. Vom Spotlight bleibt
  als Bauarbeit, dass vorhandene Serien auch über die TVDB- und die IMDb-ID erkannt werden (N-73).

### F-9 Wer fehlende Titel sieht

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Wer nicht anfordern darf, sieht fehlende Titel und den Bereich «Wünsche» gar nicht; Ausnahme eigene Wünsche.
- **Worum:** Sieht ein Profil, das nicht anfordern kann, fehlende Titel trotzdem (ohne Knopf)?
- **Möglichkeiten:** (a) nein: fehlende Titel sieht nur, wer sie auch anfordern kann; (b) ja, ohne Knopf, mit Hinweis.
- **Empfehlung: (a), eine Regel überall** (Redaktionsentscheid vom 17. November 2026). Fehlende Titel sieht ein
  Profil nur, wenn alles zutrifft: Wünsche aktiv, Seerr eingerichtet und eingeschaltet, Konto und Profil haben
  das Recht, ein Seerr-Benutzer ist vorhanden (des Kontos, die Ausnahme des Profils oder Sammelbenutzer), die Art ist erlaubt, die
  Altersstufe passt. Sonst existieren sie für das Profil nicht (404), auch nicht mit «Hinweis statt Knopf». Einzige
  Ausnahme: Titel, zu denen das Profil selbst einen Wunsch hat (jeder Zustand), bleiben auf der Vorschauseite und
  unter «Meine Wünsche» sichtbar. Seit Fassung 3 gilt dieselbe Regel für die Seiten «Entdecken», «Filme», «Serien»
  und «Suche»: Ein Profil, das nicht anfordern kann, sieht im Bereich «Wünsche» höchstens «Meine Wünsche» (N-56). Ein Konto mit Recht, aber ohne Zuordnung bemerkt der Administrator in der
  Verwaltung (Übersicht, Zuordnung, Konten), nicht der Zuschauer. Ist Seerr nur vorübergehend nicht
  erreichbar oder lehnt den Schlüssel ab, bleiben fehlende Titel sichtbar, und Wünsche gehen ins Ausgangsfach.
  Ein Kind ohne Recht sähe sonst laufend Titel, die es nicht bekommen kann (`02-funktionskonzept.md`, Abschnitt 3;
  `04-benutzer-profilmapping.md`, Abschnitt 9).
- **Andere Wahl:** (b) mehr Unruhe in der Suche und auf den Wünsche-Seiten, und ein Knopf ohne Wirkung wäre nur
  Werbung; der Jugendschutz gilt trotzdem. Die Startseite betrifft F-9 seit E-5 nicht mehr.

### F-10 Bestätigungsdialog

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Anfordern über einen Dialog.
- **Worum:** Geht der Wunsch beim ersten Druck weg, oder erst nach einem Dialog?
- **Möglichkeiten:** (a) ein Dialog mit Poster, Seerr-Benutzer, Hinweis zur Freigabe, Staffelwahl und Stand des
  Limits; (b) sofort mit «Rückgängig».
- **Empfehlung: (a).** Der Dialog sagt vorher, ob der Wunsch sofort beschafft wird oder eine Freigabe braucht,
  trägt die Staffelwahl und zeigt seit Fassung 3, wie viel vom Limit frei ist; ist es erreicht, sperrt er den Knopf
  mit dem Zeitpunkt des nächsten freien Platzes (N-36).
- **Andere Wahl:** (b) schneller, aber ohne Staffelwahl; ein versehentlicher Wunsch liegt schon in Seerr.

### F-11 Suche nach fehlenden Titeln

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Bibliothekssuche über TMDB mit eigenem Schlüssel, Seite «Suche» über Seerr.
- **Worum:** Woher kommen die Treffer fehlender Titel? Seit Fassung 3 gibt es zwei Stellen: den Abschnitt «Noch
  nicht in der Bibliothek» in der Bibliothekssuche (M-07) und die neue Seite «Suche» im Bereich «Wünsche» (M-34),
  die Marco verlangt hat (Anhang H 1). Welche Quelle die neue Seite nutzt, hat Marco nicht entschieden.
- **Möglichkeiten:** (a) TMDB-Suche über Slouchcats eigenen Schlüssel; (b) Suche von Seerr (`GET /search`).
- **Empfehlung: (a) für die Bibliothekssuche wie in Fassung 2, (b) für die Seite «Suche»** (Redaktion, N-53, N-54).
  Die Bibliothekssuche funktioniert so auch, wenn Seerr nicht erreichbar ist. Die Seite «Suche» soll
  zeigen, was Seerr bestellen kann, und kennt die Präfixe `tmdb:`, `tvdb:`, `imdb:` (Anhang H 3.3); sie
  läuft über den Server mit Zwischenspeicher und derselben Jugendschutzregel.
- **Andere Wahl:** (a) überall: eine Quelle weniger, aber ohne Suche nach Kennung ausser über eigenen Bau.
  (b) überall: die Bibliothekssuche hinge an Seerr, und jede Eingabe belastete es.

### F-12 Serien und Staffeln

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Serien staffelweise.
- **Worum:** Wie fordert man Serien an?
- **Möglichkeiten:** (a) «Alle fehlenden Staffeln» oder einzelne Staffeln, keine einzelnen Folgen, fehlende
  Staffeln auf der Serienseite; (b) nur ganze Serien.
- **Empfehlung: (a).** So arbeitet Seerr (staffelweise), und wer eine neue Staffel will, findet den Knopf auf
  der Seite der Serie. Seit Fassung 3 kommen die Staffeln im Dialog aus Seerr (N-63), damit genau das
  angeboten wird, was Seerr bestellen kann; zur Staffelzählung siehe F-29.
- **Andere Wahl:** (b) Wünsche werden unnötig gross; fehlende Staffeln vorhandener Serien bleiben unsichtbar.

### F-13 Was «verfügbar» heisst

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** «Verfügbar» heisst: Slouchcat kann den Titel für das Profil abspielen.
- **Worum:** Wann bekommt das Profil «Dein Wunsch ist da»?
- **Möglichkeiten:** (a) wenn Slouchcat den Titel für dieses Profil abspielen kann; (b) wenn Seerr «verfügbar»
  meldet.
- **Empfehlung: (a).** Seerr meldet «verfügbar» nach seinem Radarr- oder Sonarr-Scan (Vorgabe täglich
  04:00), oft Stunden nach dem Import; das sagt nichts darüber, ob Slouchcat die Datei schon eingelesen hat
  (Anhang D 11.1). Ein Kind ohne passende Freigabe sähe den Titel noch nicht. Bei Serien gilt eine Staffel erst als
  verfügbar, wenn sie vollständig abspielbar ist (`06-benachrichtigungen.md`, Abschnitt 3.3).
- **Andere Wahl:** (b) die Meldung kommt zu früh, der Link führt ins Leere.

### F-14 Kanal-Ereignisse und Vorgaben

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Drei eigene Kanal-Ereignisse: «verfügbar» (an), «freigegeben, abgelehnt, fehlgeschlagen» (aus), «Freigabe nötig» (an, nur Administratoren).
- **Worum:** Welche Häkchen gibt es auf der Kanalseite, und was ist ab Werk an?
- **Möglichkeiten:** (a) drei Ereignisse: «verfügbar» (an), «freigegeben, abgelehnt, fehlgeschlagen» (aus),
  «Freigabe nötig» (an, nur Administratoren); (b) `new_content` wiederverwenden; (c) ein Ereignis für alles.
- **Empfehlung: (a).** Eigene Häkchen lassen sich getrennt abwählen. Ein gewöhnliches Konto hat `new_content` ab
  Werk auf keinem Kanal (`server/queries/system/notify_channels.sql:7-18`); mit (b) käme die Meldung nur im Eingang an.
- **Andere Wahl:** (b) kein eigener Schalter; (c) wer nur «verfügbar» will, bekommt auch Ablehnungen.

### F-15 PIN-Profile in externen Nachrichten

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Externe Nachrichten von PIN-Profilen ohne Titel, abschaltbar.
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

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** «Freigeben» und «Ablehnen» in der Verwaltung, als Durchreichen an Seerr.
- **Worum:** Muss der Administrator für jede Freigabe in Seerr wechseln?
- **Möglichkeiten:** (a) Knöpfe «Freigeben» und «Ablehnen» in der Verwaltung, die an Seerr durchreichen, in
  Stufe 4; (b) nur in Seerr.
- **Empfehlung: (a).** Spart einen Wechsel; die Entscheidung bleibt in Seerr gespeichert. Es ist kein eigener
  Freigabe-Ablauf, nur ein zweiter Knopf für dieselbe Aktion.
- **Andere Wahl:** (b) etwas weniger Bau; der Administrator arbeitet in zwei Oberflächen.

### F-17 Zurückziehen

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Zurückziehen, solange nicht freigegeben.
- **Worum:** Darf ein Profil einen Wunsch zurücknehmen?
- **Möglichkeiten:** (a) ja, solange er nicht freigegeben ist; (b) nein.
- **Empfehlung: (a).** Seerr erlaubt das Löschen eigener offener Anfragen (Anhang D 8); danach beschafft
  Radarr schon.
- **Andere Wahl:** (b) versehentliche Wünsche bleiben liegen, bis ein Administrator sie ablehnt.

### F-18 Seerr-Webhook

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Seerr-Webhook als Anstoss; der Abgleich alle 5 Minuten bleibt die Grundlage.
- **Worum:** Slouchcat fragt alle 5 Minuten nach. Soll Seerr zusätzlich sofort Bescheid geben?
- **Möglichkeiten:** (a) Webhook als Anstoss in Stufe 4, Grundlage bleibt der Abgleich; (b) kein Webhook; (c)
  Webhook als Grundlage.
- **Empfehlung: (a).** Der Webhook von Seerr hat ein einziges Ziel, keine Signatur, keine Wiederholung und
  keine Benutzer-ID (Anhang D 9). Als Anstoss schadet ein Verlust nicht.
- **Andere Wahl:** (b) bis zu 5 Minuten Verzug. (c) jede verlorene Meldung wäre ein verlorener Zustand.

### F-19 Ort in der Verwaltung

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Verbindung unter Integrationen, tägliche Arbeit unter Medien › Wünsche.
- **Worum:** Wo stehen Verbindung, Anfragen und Seerr-Benutzer (in Fassung 2 «Zuordnung»)?
- **Möglichkeiten:** (a) Verbindung unter Integrationen, Seite «Wünsche» unter Medien; (b) alles unter
  Integrationen; (c) ein eigener Bereich.
- **Empfehlung: (a).** Einrichten bei den anderen Diensten, tägliche Arbeit bei den Medien (`07-administration.md`,
  Abschnitt 1.2).
- **Andere Wahl:** (b) mischt Einrichten und Liste; (c) widerspricht den sechs Bereichen aus ADR 0035.

### F-20 Startreihe für Wünsche

- **Ersetzt durch F-34, entschieden durch E-10.** Fassung 3 vor der zweiten Rückmeldung empfahl eine Reihe mit offenen und kürzlich
  erfüllten Wünschen. Marcos zweite Nachricht («Nur vorhandene Filme und Serien sollen auf der Startseite angezeigt
  werden») passt dazu nur, wenn die Reihe ausschliesslich Titel der Bibliothek zeigt. Die Frage steht darum neu als
  F-34.

### F-21 Bestehende Anfragen übernehmen

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Bestehende Anfragen verknüpfter Seerr-Benutzer übernehmen («In Seerr angelegt»).
- **Worum:** In Seerr liegen oft schon Anfragen. Zeigt Slouchcat sie dem passenden Profil?
- **Möglichkeiten:** (a) ja, für zugeordnete Benutzer: dem Profil mit eigener Zuordnung, sonst dem Hauptprofil,
  gekennzeichnet «In Seerr angelegt»; (b) nein, nur Wünsche aus Slouchcat.
- **Empfehlung: (a)**, in Stufe 4. Wer in Seerr gewünscht hat, bekommt die Meldung «verfügbar» auch über
  Slouchcat. Anfragen nicht zugeordneter Benutzer bleiben «nicht zuordenbar». Die Übernahme läuft als eigener Job
  `wishes.import`, ausgelöst beim Setzen oder Ändern einer Zuordnung und von Hand
  (`GET /request?requestedBy=<Benutzer>` als Benutzer 1, `03-architektur.md`, Abschnitt 7.3). Seit E-4 löst auch
  jedes Verknüpfen über die E-Mail die Übernahme aus.
- **Andere Wahl:** (b) Wünsche aus Seerr bleiben in Slouchcat unsichtbar; doppelte Wünsche fängt der
  Mitwunsch trotzdem ab.

### F-22 Stufen

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), gegen die Empfehlung:** **Ein Release am Schluss** (B-1).
- **Worum:** In welcher Reihenfolge wird gebaut, mit je einem Release? (Die Stufen bleiben die Reihenfolge des
  Baus; veröffentlicht wird einmal, N-104.)
- **Möglichkeiten:** (a) Stufen: Fundament, Anfordern, Spotlight, Serien und Komfort (Anhang F 10), seit Fassung 3
  dazu Stufe 5 «Wünsche-Seiten»; (b) alles in einem Release.
- **Empfehlung: (a)** mit Stufe 5 (Redaktion, Anhang H 4.7 und 8.7). Eröffnen, Passwortabgleich und Limit gehören
  zu Stufe 1, Profil-Tag und Download-Fortschritt zu Stufe 4. Stufe 3 schrumpft nach E-5 auf den Abgleich über TVDB
  und IMDb, die fehlenden Einträge in der Verwaltung und die Reihe «Erfüllte Wünsche». Der Sofort-Import ist nach
  E-11 ein eigener Schritt nach einer Messung (`09-umsetzungsplan.md`, Abschnitt 6a). Stufe 1 ändert für Zuschauer fast nichts und lässt Marco die
  Seerr-Benutzer in Ruhe einrichten; jede Stufe ist für sich nützlich und prüfbar.
- **Andere Wahl:** (b) ein grosser Release mit langer Prüfung; Fehler in der Zuordnung zeigten sich erst am Ende.

### F-23 Für wen Slouchcat einen Seerr-Benutzer eröffnet

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Für jedes Konto ein Seerr-Benutzer; ohne Wunschrecht ohne Rechte; bestehende Konten über «Alle eröffnen».
- **Worum:** Marco will, dass eingeladene Konten in Seerr eröffnet werden (E-1). Gilt das für jedes Konto oder
  nur für Konten mit Wunschrecht?
- **Möglichkeiten:** (a) für jedes Konto, ohne Wunschrecht ohne Rechte in Seerr; (b) nur für Konten mit
  Wunschrecht, beim Einschalten des Rechts.
- **Empfehlung: (a)** (N-9). Ein Konto hat dann immer denselben Login in beiden Diensten; das Wunschrecht schaltet
  nur die Rechte in Seerr. **Wann eröffnet wird (Redaktionsentscheid zu K-15):** Seerr einrichten
  eröffnet bestehende Konten nicht von selbst. Bestehende Konten eröffnet der Administrator mit «Alle eröffnen»
  oder einzeln, wie in E-4 beschrieben. Von selbst eröffnet nur das Annehmen einer Einladung; gibt es die Adresse in Seerr schon,
  verknüpft Slouchcat sofort (Marco, E-8; N-90).
- **Andere Wahl:** (b) weniger Benutzer in Seerr; dafür wird beim späteren Einschalten des Rechts eröffnet,
  und das Passwort folgt erst bei der nächsten Anmeldung.

### F-24 Passwort bei verknüpften, schon bestehenden Benutzern, und wer verknüpft

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), gegen die Empfehlung:** **Passwort nur für eingeladene und von Slouchcat eröffnete Benutzer.** Ein bestehender Seerr-Benutzer, den ein Administrator verknüpft, behält sein Passwort. (Die Verknüpfung beim Annehmen einer Einladung bleibt E-8: sofort, mit Passwort.).
- **Teil «wer verknüpft»: entschieden durch E-8 (Marco).** Gibt es in Seerr schon einen Benutzer mit derselben
  E-Mail, verknüpft Slouchcat ihn sofort, gleich ob ein Administrator handelt oder das Konto die Adresse beim
  Annehmen einer Einladung oder beim Ändern der eigenen E-Mail selbst eingetippt hat (N-90, N-95). Mit der
  Verknüpfung geht das Passwort an den Benutzer wie bei jedem anderen (N-91; folgt aus E-1 und E-8). Nie an
  Benutzer 1, andere Administratoren von Seerr und Benutzer mit Anmeldung über Jellyfin, Emby oder Plex (N-6,
  Redaktion); ein Administrator von Seerr lässt sich so nicht übernehmen. Die frühere Empfehlung (c) der Redaktion
  (Verknüpfung erst nach Bestätigung, N-2a, K-1) ist zurückgezogen.
- **Bekannte Grenze (Marco, E-8; N-92).** Die Adresse prüft niemand. Wer eine Einladung hat und eine fremde
  Adresse eintippt, bekommt den Seerr-Benutzer dieser Person samt ihren Anfragen. Marco kennt das Risiko und hat es
  so gewählt. Einladungen stellt nur ein Administrator aus.
- **Bis zur Antwort offen als Empfehlung der Redaktion** (Marco hat Andere Wahl (b) gewählt, siehe erster Punkt;
  N-103):
  - (a) Passwort auch bei Konten übernehmen, die ein Administrator nachträglich eröffnet oder mit einem
    bestehenden Benutzer verknüpft: bei der nächsten Anmeldung (oder sofort, wenn es gerade im Speicher ist),
    ausser bei Administratoren und bei Anmeldung über Jellyfin, Emby oder Plex (N-4, N-6). **Lesart (K-11):**
    E-1 fragte nach **eingeladenen** Benutzern («Ja, gleiches Passwort»); der Rest ist Lesart der Redaktion.
    Andere Wahl (b): nur eröffnete und eingeladene Benutzer gleichen ab; wer in Seerr schon ein anderes Passwort
    hat und vom Administrator verknüpft wird, behält es.
  - Hinweis an die Administratoren (N-93): Die Übersicht zeigt sieben Tage lang «Neu verknüpft: {{Konto}} mit dem
    bestehenden Seerr-Benutzer {{Name}}», mit Link auf «Seerr-Benutzer», wo sich die Verknüpfung lösen lässt; das
    Protokoll hält sie fest. Andere Wahl: kein Hinweis; eine Verknüpfung nach einer Einladung fiele dann nur auf
    der Seite «Seerr-Benutzer» auf.
  - Das Konto erfährt nicht, ob Seerr die Adresse kannte; die Meldung nach dem Annehmen ist in beiden Fällen
    «In Seerr hast du jetzt ein Konto mit derselben E-Mail-Adresse.» (N-94).

### F-25 Name des Profil-Tags

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Tag `sc-<profilname>`, Präfix einstellbar, je Profil überschreibbar, nie entfernt.
- **Worum:** Wie heisst der Tag, den Slouchcat in Radarr und Sonarr je Profil setzt (E-2)?
- **Möglichkeiten:** (a) `sc-<profilname>`, Präfix einstellbar, je Profil überschreibbar, bei gleichen Namen mit
  Kontoname; (b) nur der Profilname; (c) `<konto>-<profil>`.
- **Empfehlung: (a)** (N-21). Das Präfix trennt die Tags von Slouchcat von anderen; Tags werden nie entfernt
  (N-26), damit Radarr und Sonarr nichts verlieren, was jemand dort nutzt.
- **Andere Wahl:** (b) kürzer, aber leicht verwechselbar mit eigenen Tags; (c) eindeutig, aber lang.

### F-26 Vorgabe des Limits ab Werk

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Limit ab Werk: 10 Filme und 5 Staffeln je 7 Tage.
- **Worum:** Welches Limit gilt für Konten mit «Standard»?
- **Möglichkeiten:** (a) 10 Filme in 7 Tagen, 5 Staffeln in 7 Tagen (Marcos Beispiel); (b) ohne Limit ab Werk;
  (c) andere Werte.
- **Empfehlung: (a)** (N-31). Administratoren haben in Seerr ohnehin kein Limit (N-34); die Oberfläche sagt es.
- **Andere Wahl:** (b) nach dem Update ändert sich in Seerr nichts, bis Marco Werte setzt.

### F-27 Ausnahme «Profil sendet als anderer Seerr-Benutzer»

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), gegen die Empfehlung:** **Keine Ausnahme je Profil.** Alle Profile senden als Benutzer des Kontos. Die Einstellung «Profil sendet als anderer Seerr-Benutzer» wird nicht gebaut.
- **Worum:** Fassung 2 hatte eine Zuordnung je Profil. Bleibt sie als zugeklappte Ausnahme?
- **Möglichkeiten:** (a) ja, zugeklappt, nur bestehende Benutzer; (b) nein, alle Profile senden als Benutzer des
  Kontos.
- **Empfehlung: (a)** (N-41). Sonst sind Kinderprofile im Konto des Administrators ohne Freigabe und ohne Limit,
  weil dessen Benutzer meist Auto-Freigabe und `MANAGE_USERS` hat. Für so ein Profil gelten Freigabe und Limit des
  Ausnahme-Benutzers.
- **Andere Wahl:** (b) einfacher; Kinder im Konto des Administrators fordern ohne Freigabe an.

### F-28 Reihen auf «Entdecken»

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** «Entdecken» mit sieben festen Reihen.
- **Worum:** Welche Reihen zeigt die Seite «Entdecken», die Marco wie bei Seerr will?
- **Möglichkeiten:** (a) sieben feste Reihen: Im Trend, Beliebte Filme, Film-Genres, Demnächst im Kino, Beliebte
  Serien, Serien-Genres, Demnächst als Serie; (b) die Reihen-Einstellung von Seerr (`/settings/discover`)
  übernehmen; (c) alle zwölf Reihen von Seerr.
- **Empfehlung: (a)** (N-51). «Recently Added» und «Recent Requests» zeigt Slouchcat schon anders, «Plex Watchlist»
  gibt es hier nicht, «Studios» und «Networks» brauchen eigene Seiten.
- **Andere Wahl:** (b) Marco stellt in Seerr ein und Slouchcat folgt, aber mit Reihen, die hier nichts
  bedeuten; (c) mehr Bau ohne Nutzen.

### F-29 Seerr auf TVDB-Staffeln umstellen

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Marco stellt Seerr auf TVDB-Staffeln um; Slouchcat ändert die Einstellung nie, der Test zeigt einen Hinweis.
- **Worum:** Marco fragt, ob Seerr TVDB nutzt (Anhang H 1). Antwort: Seerr führt jeden Titel über die TMDB-ID,
  gibt die TVDB-ID an Sonarr weiter und kann die Staffelliste aus TVDB lesen (Anhang H 4.6). Soll Marco das
  einstellen?
- **Möglichkeiten:** (a) ja, in Seerr den Metadaten-Anbieter für Serien auf TVDB stellen; der Verbindungstest
  zeigt einen Hinweis, wenn er auf TMDB steht; (b) nein, TMDB lassen.
- **Empfehlung: (a)** (N-64). Dann zählen Seerr, Sonarr und die Slouchcat-Bibliothek Staffeln gleich. Das
  stellt Marco in Seerr selbst um; Slouchcat ändert die Einstellung nicht.
- **Andere Wahl:** (b) bei Serien, die TMDB und TVDB verschieden aufteilen, passen Staffelnummern im Dialog und in
  der Bibliothek nicht zusammen.

### F-30 Zugang zu Radarr und Sonarr

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Zugang zu Radarr und Sonarr aus Seerr lesen; die Adresse gilt nach einem «Testen», eigene Adresse als Ausweg.
- **Worum:** Für den Profil-Tag (E-2) und den Download-Fortschritt (E-7) braucht Slouchcat Adresse und Schlüssel
  von Radarr und Sonarr.
- **Möglichkeiten:** (a) aus Seerr lesen (`GET /settings/radarr|sonarr`), eigene Adresse nur als Ausweg;
  (b) eigene Felder für Adresse und Schlüssel in Slouchcat.
- **Empfehlung: (a)** (N-23). Der Administrator tippt keinen Schlüssel ein; die Schlüssel stehen nur im Speicher.
  **Vorschlag** dazu: Eine Adresse aus Seerr gilt erst nach einem «Testen» durch den Administrator
  (`03-architektur.md`, Abschnitt 20.2).
- **Andere Wahl:** (b) ein weiterer Ort für dieselben Schlüssel, der veraltet, wenn Marco sie in Radarr erneuert.

### F-31 Seerr-Benutzer eines gelöschten Kontos

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Der Seerr-Benutzer eines gelöschten Kontos bleibt; ein eröffneter ohne Rechte und mit Zufallspasswort, ein verknüpfter unverändert.
- **Worum:** Was geschieht in Seerr, wenn der Administrator ein Konto in Slouchcat löscht?
- **Möglichkeiten:** (a) der Benutzer bleibt; ein eröffneter ohne Rechte, mit Zufallspasswort; (b) Slouchcat löscht ihn; (c) nichts.
- **Empfehlung: (a), nur für eröffnete Benutzer** (N-10). Mit dem Benutzer löschte Seerr seine Requests
  (Anhang D 7.2). **Redaktionsentscheid zu K-2:** Nur einem Benutzer, den Slouchcat eröffnet hat
  (`origin = created`), nimmt Slouchcat die Rechte und setzt ein Zufallspasswort. Verknüpfte und von Hand gewählte
  Benutzer bleiben unverändert, wie N-9 («Rechte von Benutzern, die es schon gab, fasst Slouchcat nicht an»).
- **Andere Wahl:** (b) die Geschichte der Wünsche in Seerr ginge verloren; (c) das alte Login funktionierte
  in Seerr weiter.

### F-32 Name in der Oberfläche

- **Entschieden durch E-9 (Marco): «Seerr».** So heisst das Projekt seit Version 3 (Anhang H 6). Papiere und
  Mockup sind umbenannt; «Jellyseerr» steht nur noch, wo eine Version 2.x gemeint ist, und in den eingefrorenen
  Anhängen A bis G (N-96). Die Texte stehen in `web/src/locales/*.json`.
- **Verworfen:** «Jellyseerr» (vertraut für Nutzer von 2.x) und ein Name je nach erkannter Version (zwei
  Schreibweisen in Tests und Texten).

### F-33 Sofort-Import über Dateipfad und Kennung aus der Webhook-Meldung

- **Entschieden durch E-11 (Marco): «Ja, nach Messung».** Marco will, dass ein neuer Film oder eine neue Serie
  «rasendschnell» in Slouchcat ankommt (Anhang H 8.1). Die Meldung, die heute schon kommt, nennt den genauen
  Dateipfad (`movieFile.path`, `episodeFile.path`) und die Kennungen (TMDB, IMDb, TVDB); Slouchcat liest davon heute
  nur `eventType` und den Ordner und scannt den ganzen Ordner (Anhang H 8.3;
  `server/internal/model/integrations.go:45-51`).
- **Was gilt (N-98, N-88):** eigener Schritt im Bauplan. Zuerst wird gemessen, wo heute die Zeit bleibt
  (`docs/perf/`). Zeigt die Messung keinen Gewinn, entfällt der Schritt. Sonst liest Slouchcat Dateipfad und
  Kennung, liest nur diese Datei mit Vorrang ein, ordnet sie über die Kennung zu (ohne Titelsuche) und prüft
  sofort die offenen Wünsche. Die Webhooks bleiben, wie sie sind (N-80): keine neue Anbindung, keine neue Ausnahme
  von ADR 0010. Einzelheiten `03-architektur.md`, Abschnitt 10.1; Bauschritt `09-umsetzungsplan.md`, Abschnitt 6a.
- **Empfehlung der Redaktion:** was als Gewinn zählt, schreibt der Messschritt 6a.1 fest, bevor gemessen wird.
- **Verworfen:** «Ja, fest einplanen» (ohne Messung) und «Nein» (Empfang wie heute).

### F-34 Startreihe «Erfüllte Wünsche»

- **Entschieden durch E-10 (Marco): «Erfüllte Wünsche».** Die Reihe zeigt nur Wünsche des Profils, die in den
  letzten 14 Tagen wahr wurden, also Titel der Bibliothek (N-74, N-97). Offene Wünsche stehen unter «Wünsche ›
  Meine Wünsche», ihre Zahl am Eintrag «Wünsche» der Seitenleiste. Die Reihe ist ausblendbar wie jede Reihe und
  erscheint leer nicht (Redaktion). Marcos Satz aus der zweiten Nachricht («Nur vorhandene Filme und Serien sollen
  auf der Startseite angezeigt werden») gilt damit auch für diese Reihe.
- **Verworfen:** «Keine Reihe» (Stufe 3 wäre im Vertrag nicht brechend, `08-datenmodell-api.md`, Abschnitt 4.6;
  Erfülltes sähe man nur über die Glocke und «Meine Wünsche») und «Auch offene Wünsche» (widerspräche Marcos Satz).

### Teilfragen T-1 und T-2 (aus der ersten Kritik)

Zwei Festlegungen gehen über Anhang F hinaus und waren in Fassung 1 nicht als Frage gekennzeichnet (Kritik,
Befunde 30 und 40). Sie gehören zu F-3 bzw. F-17.

**T-1 «Wünsche aktiv» nur mit wirksamem `X-API-User` (zu F-3).**

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** «Wünsche aktiv» lässt sich nur nach einem gelungenen Test einschalten; ab Werk aus.
- **Worum:** Darf der Administrator «Wünsche aktiv» einschalten, wenn der letzte Verbindungstest nicht gelang oder
  «Handeln im Namen von Benutzern» nicht wirkt?
- **Möglichkeiten:** (a) nein, der Schalter ist gesperrt, bis der Test gelingt; (b) ja, der Test warnt nur.
- **Empfehlung: (a).** Ohne wirksamen `X-API-User` liefen Wünsche als Benutzer 1 und wären sofort freigegeben.
- **Andere Wahl:** (b) Anhang F verlangt nur die Meldung; der Administrator trüge das Risiko selbst.

Die Vorgabe «Wünsche aktiv = aus» ist ein Redaktionsentscheid vom 17. November 2026 und steht mit den übrigen Empfehlungen zur Freigabe.

**T-2 Frist der Übermittlung, solange «Seerr verwenden» aus ist (zu F-17 und zum Ausgangsfach).**

- **Beantwortet am 17. November 2026 (Marco, Anhang H 11.2), nach Empfehlung:** Die Frist der Übermittlung ruht, solange Seerr in Slouchcat ausgeschaltet ist.
- **Worum:** Wünsche `queued` scheitern nach 24 h. Zählt die Frist weiter, wenn der Administrator Seerr
  bewusst ausschaltet?
- **Möglichkeiten:** (a) die Frist ruht, solange «Seerr verwenden» aus ist; (b) beim Ausschalten werden alle
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
| `:35` Statuszeile Integrationen | «Statusanzeige, Wünschen über Seerr, SABnzbd und die Steuerung der Arr-Dienste zurückgestellt (Runden 12 und 20)» | Zusatz: «Ausnahme seit {{Datum}}: Wünsche über Seerr (Frage 33, Konzept 25, ADR 0037)» |
| `:46` Constraint | «Wünschen von Inhalten erfolgt ausserhalb von Slouchcat direkt in Seerr (Overseerr/Jellyseerr); Slouchcat baut weder eine eigene Wunschverwaltung noch einen Seerr-Bereich (Runde 20).» | «Wünsche verwaltet Seerr. Slouchcat legt sie dort im Namen des zugeordneten Benutzers an, zeigt ihren Zustand und meldet die Verfügbarkeit (Frage 33); eine eigene Wunschverwaltung baut es nicht.» |
| `:65` Non-Goal | «Freigabe-Anfragen von Kinderprofilen an den Hauptbenutzer.» | **bleibt.** Klarstellung in ADR 0037: Die Freigabe eines Wunsches entscheidet Seerr; das Durchreichen von «Freigeben/Ablehnen» für Administratoren (F-16) ist kein Ablauf «Kind fragt Eltern». |
| `:66` Non-Goal | «Eigene Wunschverwaltung und ein Bereich „Wünschen“ über die Seerr-API in Slouchcat; Wünsche entstehen direkt in Seerr (Runde 20).» | Zusatz: «Ausnahme seit {{Datum}}: Wünsche über Seerr (Frage 33, ADR 0037). Seerr bleibt die Wunschverwaltung; Slouchcat legt an, zeigt den Zustand, zieht zurück und reicht für Administratoren Freigeben und Ablehnen durch. Slouchcat eröffnet für jedes Konto einen Benutzer in Seerr mit demselben Login und schreibt dessen Limit; die Seiten «Entdecken», «Filme», «Serien» und «Suche» zeigen Titel zum Anfordern.» |
| `:67` Non-Goal | «Beschaffungsstatus aus Radarr/Sonarr am Titel und in der Suche („gewünscht“, „in Beschaffung“, „verfügbar“) sowie die Benachrichtigung bei Verfügbarkeit (Runde 20, Kandidaten für die Ideensammlung).» | Zusatz mit einer **eng gefassten Ausnahme nur beim Wunsch** (Marco, E-7; Anhang H 8.5): «Ausnahme seit {{Datum}}: der Zustand **eigener Wünsche** aus Seerr, der Download-Fortschritt aus Radarr und Sonarr beim Wunsch («Meine Wünsche», Wunschkarte, Verwaltung) und die Benachrichtigung, wenn ein gewünschter Titel in Slouchcat abspielbar ist (Frage 33). Am Titel der Bibliothek, in Reihen und in der Suche bleibt ein Beschaffungsstatus Nicht-Ziel, ebenso der Suchfilter «in Beschaffung».» |
| `:68` Non-Goal | «Download-Fortschritt aus SABnzbd am Titel (Runde 20).» | **bleibt** |
| `:69` Non-Goal | «Steuerung von Radarr, Sonarr, SABnzbd aus Slouchcat heraus (Qualitätsprofile, Warteschlangen).» | **bleibt mit einer eng gefassten Ausnahme** (Fassung 3, E-2): Zusatz «Ausnahme seit {{Datum}}: Slouchcat legt in Radarr und Sonarr Tags an und hängt sie an Titel, um zu zeigen, welches Profil gewünscht hat (Frage 33, ADR 0037). Warteschlange und Dateistand der gewünschten Titel liest es nur, für den Download-Fortschritt bei Wünschen (E-7), und ändert sie nie. Sonst nichts.» |
| `:159` Hinweis | «I-1 bis I-4 und I-6 … sind in Runde 20 entfallen … Die Nummern bleiben unbesetzt» | bleibt; **Vorschlag:** neue Kriterien I-9 (Wunsch anlegen und Zustand, mit Download-Fortschritt), I-10 (Benachrichtigung bei Verfügbarkeit), I-11 (fehlende Titel in der Suche und auf den Wünsche-Seiten, nie auf der Startseite) in ADR 0037 |
| `:176`, `:182`, `:185` | Annahmen und offene Punkte «Wünschen via Seerr in Runde 20 gestrichen», «Seerr-Einbettung: entfallen» | bleiben als Geschichte; ADR 0037 verweist darauf. Einbettung bleibt entfallen. |
| `:120` Spec E-4, `:149` Spec A-6 | Spec E-4 «… Verfügbarkeit (lokal, Download, in Beschaffung)»; Spec A-6 «… für neue Inhalte, Beschaffungsstatus, Fehler und Einladungen» | veraltete Reste aus Frage 10; **Vorschlag:** Spec A-6 gilt mit den Ereignissen der Wünsche als erfüllt; «in Beschaffung» in Spec E-4 bleibt gestrichen |

### 2.2 Plan, Protokoll, ADRs, `CLAUDE.md`

| Stelle | Wortlaut heute | Neu (Vorschlag) |
|---|---|---|
| `CLAUDE.md:57-59` Regel 11 | «… algorithmische Empfehlungen, Trakt-Login, eigene Wunschverwaltung, Steuerung von Arr/Seerr, Mandanten …» | Abschnitt 6. Seit Fassung 3 mit **zwei eng gefassten Ausnahmen** von «Steuerung von Arr/Seerr»: Benutzer in Seerr anlegen und pflegen (E-Mail, Passwort, Rechte, Limit) und Tags in Radarr und Sonarr setzen (Anhang H 5); dazu die Klarstellung, dass Slouchcat Warteschlange und Dateistand der gewünschten Titel für den Download-Fortschritt nur liest (E-7, K-5) |
| `CLAUDE.md` Regel 10 | «Keine Secrets in Logs … neue Schlüsselnamen mit Secrets in die Redaction-Liste und in den Test» | bleibt; die Redaction-Liste (`server/internal/logging/logging.go:53-55`) und ihr Test bekommen `seerrapikey`, `newpassword`, `radarrapikey`, `sonarrapikey` (`password` steht schon drin). Regel 10 selbst braucht keinen neuen Wortlaut (Anhang H 5) |
| `docs/planning/03-plan-v1.md:30` | «Nicht eingeplant …: … eigene Wunschverwaltung und Seerr-Bereich zum Wünschen, Beschaffungsstatus am Titel, …, Steuerung von Radarr/Sonarr/SABnzbd/Seerr …» | Zusatz «(Ausnahme seit {{Datum}}: Wünsche über Seerr; Benutzer der Konten in Seerr anlegen und pflegen; Profil-Tags in Radarr und Sonarr; Download-Fortschritt beim Wunsch; Frage 33, ADR 0037)» |
| `docs/planning/03-plan-v1.md:286-292` 5.1 | «entfallen … Slouchcat bindet Seerr gar nicht an, weder über die API noch als Einbettung … AN-5 (Wunschrecht) und AN-16 (Kontozuordnung)» | Nachtrag: «Mit Frage 33 neu gefasst: Anbindung über die API ohne Einbettung; AN-5 lebt als Wunschrecht je Konto und Profil, AN-16 als Seerr-Benutzer je Konto mit demselben Login (ADR 0037).» |
| `docs/planning/03-plan-v1.md:308-310` 5.5 | «Ein Ereignis für ein Profil geht an die Kanäle des Kontos mit Profilname als Präfix … Vor dem Versand prüft `notify` über `visibility`, ob das Profil das Item sehen darf» | bleibt; Zusatz: «Bei Wünschen nennt die externe Nachricht eines PIN-Profils keinen Titel (F-15).» |
| `docs/planning/03-plan-v1.md:312-316` 5.6 | «Der Webhook ist die einzige Verbindung zu den Arr-Diensten und rein eingehend.» | Nachtrag: «Zu Seerr baut Slouchcat eine ausgehende Verbindung (ADR 0037). Zu Radarr und Sonarr kommt eine zweite, ausgehende Verbindung nur für Profil-Tags (Tag anlegen, Tag anhängen) und das Lesen von Warteschlange und Dateistand der gewünschten Titel (ADR 0037, Nachtrag ADR 0010). Der Webhook bleibt die einzige Meldung von Radarr und Sonarr an Slouchcat, bringt weiter den Import in die Bibliothek und wird weiter von Hand eingerichtet.» |
| `docs/planning/03-plan-v1.md:164` 3.4 Pkt. 5 | «Ereignis `item.available` an `notify`» (im Code nicht vorhanden) | **Vorschlag:** Hinweis, dass der Haken nach dem Match (`WithMatched`) diese Rolle für Wünsche übernimmt |
| `docs/planning/04-entscheidungen.md:19` Frage 10 | Vorschlag damals «Profile des Hauptkontos ab FSK 12 dürfen wünschen, Gäste nicht; jedem Konto wird ein bestehender Seerr-Benutzer zugeordnet»; Antwort «… Statusanzeige und Wünschen als Kandidaten für die Ideensammlung» | bleibt als Geschichte; Frage 33 sagt, dass Wünschen damit aus der Ideensammlung übernommen ist. Webhooks: zur Art Radarr/Sonarr kommt in Stufe 4 die Art Seerr. |
| `docs/planning/04-entscheidungen.md:37`, `:55` | Frage 5: «Wünschen über eigene Seite via Seerr-API ohne Einbettung»; M9: «I-1, I-2, I-3, I-4, I-6 … ersatzlos gestrichen» | bleiben als Geschichte |
| S-4: `docs/planning/04-entscheidungen.md:20`, `docs/planning/05-plan-spotlight.md:84-85, 291, 1622`, `docs/adr/0011-spotlight.md:99` | «Titel ohne Bibliothekstreffer erscheinen **nie** auf der Startseite (Non-Goal «eigene Wunschverwaltung») …, in der **Verwaltung** aber schon»; ADR 0011 verwirft den Schalter «auch Titel ohne Bibliothekstreffer zeigen» | **bleibt ohne Ausnahme** (Marco, E-5; N-70): Startseite und Spotlight-Liste zeigen nur Titel der Bibliothek; fehlende Einträge der Trakt-Liste nur in der Verwaltung. ADR 0011 verwirft den Schalter weiter. ADR 0038 nennt S-4 und grenzt sich ab; Fassung 2 und Fassung 3 vor der zweiten Rückmeldung wollten S-4 für das Spotlight ablösen. Nachtrag in ADR 0011 nur: vorhandene Titel werden auch über TVDB- und IMDb-ID erkannt (N-73) |
| `docs/adr/0009-administration-benachrichtigungen.md:36` und Nachtrag `:62-67` | Kanal-Ereignisse `new_content`, `errors`, `invitations`, `announcements`; `acquisition` entfiel, «ohne Erzeuger wäre der Kanal eine Schaltfläche ohne Wirkung» | Nachtrag: drei neue Ereignisse `wish_available`, `wish_status`, `wish_review` mit echtem Erzeuger (`wishes`); fünf neue Eingangsarten |
| `docs/adr/0010-webhook-empfang.md:11-13`, `:64-65` | «M9 ist auf den eingehenden Webhook zusammengeschnitten (Frage 10)»; «Die CSRF-Ausnahme ist die erste ihrer Art …» | Nachtrag in Stufe 4: Art `jellyseerr` neben `radarr`/`sonarr` (CHECK in `webhook_integrations`), nur als Anstoss des Abgleichs. Dieselbe Route `/webhooks/{token}`, darum **keine** zweite CSRF-Ausnahme. |
| ADR 0010, Satz «Slouchcat never calls Radarr or Sonarr and registers nothing there» (Anhang C 1) | Slouchcat ruft Radarr und Sonarr nie auf | **Zwei eng gefasste Ausnahmen** (E-2, E-7; Anhang H 8.5): Slouchcat legt Tags an und hängt sie an Titel (N-22), und es liest Warteschlange und Dateistand der gewünschten Titel für den Download-Fortschritt (`queue`, `movie?tmdbId`, `series?tvdbId`; N-81, K-5), beides über das Paket `internal/arr`. Der Import in die Bibliothek kommt weiter über den Webhook; gelesen wird nur für die Anzeige am Wunsch. «Registers nothing there» bleibt: Slouchcat trägt keinen Webhook ein (E-6, N-80). Der eingehende Teil von ADR 0010 (Route, Token, `scan.path`) bleibt unverändert. Nachtrag in Stufe 0 |
| `docs/adr/0008-entdecken-suche-trakt.md:82` | «Offen: Filter «in Beschaffung» (M9) …» | veraltet; keine Änderung nötig |
| `docs/ideas/README.md:31-37` (Index) | kein Eintrag zu Wünschen | **Vorschlag:** neue Zeile «06 Wünsche über Seerr · Übernommen (Konzept 25, Frage 33, ADR 0037/0038)» |
| `server/internal/integrations/integrations.go:5-6`, Text `webhooks.intro` | «Slouchcat never calls Radarr or Sonarr …»; «… fragt die Dienste nie von sich aus ab.» | Der Code-Kommentar stimmt ab Stufe 4 nicht mehr wörtlich: **Vorschlag**, ihn in Stufe 4 auf «… except for profile tags and reading the queue and file state of wished titles (package arr)» zu ergänzen. Die Seite «Webhooks von Radarr und Sonarr» bleibt wörtlich, wie sie ist, auch der Text `webhooks.intro`: kein neuer Satz, auch nicht als Vorschlag (N-80, Redaktionsentscheid zu K-7; AK-196 prüft es streng). Der Text spricht vom Empfang, und der bleibt |

### 2.3 Was ausdrücklich Nicht-Ziel bleibt

| Bleibt Nicht-Ziel | Stelle heute |
|---|---|
| Steuerung von Radarr, Sonarr, SABnzbd (Qualitätsprofile, Warteschlangen ändern, Zielordner, Suchen, Hinzufügen, Löschen, 4K, Tags entfernen, Webhooks eintragen); ausgenommen sind nur der Profil-Tag (anlegen, anhängen) und das Lesen von Warteschlange und Dateistand der gewünschten Titel | Spec `:69`, Anhang H 5, 8.5 |
| Download-Fortschritt aus SABnzbd | Spec `:68` |
| Eigene Beschaffung; Beschaffungsstatus am Titel der Bibliothek, in Reihen und in der Suche, auch für gewünschte Titel ausserhalb des Wunsches; Suchfilter «in Beschaffung» | Spec `:67`, `:120`, Anhang H N-82 |
| Fehlende Titel auf der Startseite (Bühne, Posterreihe), in der Spotlight-Liste und als Reihe unter «Entdecken» | S-4, Marco E-5 |
| Importe über Abfragen statt über den Webhook erkennen oder Webhooks selbst eintragen. Der Import in die Bibliothek kommt weiter über den Webhook; Slouchcat liest nur für die Anzeige am Wunsch (K-5) | Marco E-6 |
| Freigabe-Anfragen von Kinderprofilen an den Hauptbenutzer als eigener Slouchcat-Ablauf | Spec `:65` |
| In Seerr: Benutzer löschen, Rechte von Benutzern ändern, die es schon gab, Administratoren anlegen, übrige Einstellungen ändern (Discover-Reihen, Metadaten-Anbieter, Radarr/Sonarr-Server). Ausgenommen sind nur das Anlegen und Pflegen der Benutzer der Konten (E-Mail, Passwort, Rechte, Limit) | neu in ADR 0037, Anhang H 5 |
| Einbettung von Seerr (iFrame) | Spec `:185` |
| Trakt-Login, Scrobbling, Wünsche aus Trakt-Merklisten | Spec Non-Goals, Regel 11 |
| Algorithmische Empfehlungen: fehlende Titel kommen nur aus der Suche, den öffentlichen Ranglisten der Wünsche-Seiten und, nur für den Administrator, aus der Trakt-Liste des Spotlights | Spec Non-Goals |
| Interaktion zwischen Profilen: kein «wer hat noch gewünscht» | Spec Non-Goals, ADR 0028 |
| eigene Empfehlungen je Profil auf den Wünsche-Seiten; die Listen bleiben öffentliche TMDB-Ranglisten, für alle gleich (N-58). Die Seite «Entdecken» selbst ist seit Fassung 3 kein Nicht-Ziel mehr (Marco, Anhang H 1); Anhang F 2.14 ist überholt | Spec Non-Goals, Anhang H 4.5 |
| Login je Profil in Seerr | Marco, Anhang H 1 |
| Wünsche offline | Anhang F 2.20 |

## 3. Eintrag Frage 33 im Entscheidungsprotokoll

Eingetragen am 17. November 2026 in `docs/planning/04-entscheidungen.md`, Tabelle «Weitere Entscheide während der
Umsetzung», nach Marcos Wort «Ja, eintragen» auf die Frage, ob das Konzept als freigegeben gilt.

Der Eintrag hält fest:

- **Entschieden von Marco:** E-1 bis E-11 (Abschnitt 0), dazu der Bereich «Wünsche» und die Erkennung der
  Spotlight-Serien über die TVDB-ID aus seinen Rückmeldungen.
- **Nicht einzeln beantwortet:** die 29 offenen Fragen und T-1, T-2 (Abschnitt 1). Bis zu Marcos Antwort gilt je
  die Empfehlung dieses Papiers; die Bau-Session legt die Fragen einer Stufe vor deren Bau vor. Der Eintrag
  schreibt Marco keine dieser Antworten zu. Ein früherer Entwurf dieses Abschnitts hatte angenommen, dass er
  allen Empfehlungen folgt; das steht so nicht im Protokoll.
- **Ausnahmen von Nicht-Zielen:** wie in Abschnitt 2 beschrieben. S-4 bleibt ohne Ausnahme.
- **Bau:** sechs Stufen nach `09-umsetzungsplan.md`, erst auf Marcos ausdrückliches Wort.

**Nachtrag vom 17. November 2026.** Marco hat den Bau bestellt und alle offenen Fragen beantwortet. Der Nachtrag zu
Frage 33 im Entscheidungsprotokoll nennt die Antworten, die vier Abweichungen von der Empfehlung (F-4, F-22, F-24,
F-27) und B-1 bis B-4 (Anhang H 11). Er schreibt Marco nur zu, was in Anhang H 11.1 und 11.2 steht; die Folgen
N-100 bis N-104 sind, wo Anhang H es sagt, Redaktion.

## 4. Entwurf ADR 0037 «Wünsche über Seerr»

```markdown
# ADR 0037: Wünsche über Seerr

- Status: Entwurf, nachgeführt auf Marcos Antworten vom 17. November 2026 (Anhang H 11); angenommen mit Stufe 0
  (Frage 33). **Hebt die Spec-Nicht-Ziele Zeile 66
  in Teilen und Zeile 67 nur beim Wunsch auf und fasst Zeile 69 mit einer Ausnahme**; Zeile 65 und 68 bleiben.
  **Nachtrag zu ADR 0010** (zwei Ausnahmen: Profil-Tags setzen, Warteschlange und Dateistand der gewünschten Titel
  lesen; der Webhook-Empfang bleibt).
- Quelle: Konzept `docs/planning/25-medienwuensche/` (Papiere 01 bis 11, Fassung 3), Marcos Entscheide E-1 bis
  E-11 vom 17. November 2026, **Frage 33** in `docs/planning/04-entscheidungen.md` mit den Antworten F-1 bis F-34
- Vertrag und Migrationen: vergibt der Bau; ein Release für alle Stufen (F-22, B-1), der Vertrag steigt einmal,
  brechend (`08-datenmodell-api.md`, Abschnitt 4.6)
- Betroffene Spec-IDs: Spec A-6, Spec I-8; neu Spec I-9, Spec I-10, Spec I-11
- Harte Regeln: 2 (Titelabfragen nur über `visibility`), 3 (kein `os/exec` in `seerr` und `arr`), 5 (Migrationen
  nach Vorlage), 7 (`x-visibility`, Vertragstest), 8 (Tastaturtest), 9 (Texte), 10 (Schlüssel und Passwörter
  redigiert), 11 (Nicht-Ziel mit Ausnahmen), 12 (Entscheid im Server)

## Kontext

Bis 1.40 bindet Slouchcat Seerr nicht an (Frage 10, Plan 5.1): Wer etwas sehen will, das fehlt, wünscht es
in Seerr. Marco will, dass Zuschauer fehlende Filme und Serien in Slouchcat anfordern, den Zustand sehen
und über Slouchcats Kanäle erfahren, wann der Titel da ist. Seerr soll die Wunschverwaltung bleiben:
Freigaben, Limits und die Übergabe an Radarr und Sonarr liegen dort. Nach der Durchsicht der zweiten Fassung
verlangte Marco am 17. November 2026: Eingeladene Konten bekommen in Seerr einen Benutzer mit denselben
Login-Daten; Profile brauchen kein Login, nur einen Tag; ein Limit je Konto; eine Seite für Wünsche wie die
Startseite von Seerr; Serien konsistent über TVDB. Nach der Durchsicht des Mockups fragte er, ob Radarr und Sonarr
direkt angebunden werden sollen, damit ein neuer Titel «rasendschnell» in Slouchcat ankommt, und ob die Webhooks
dafür wegfallen könnten. Radarr und Sonarr melden von sich aus nur per Webhook; ihre API ist rein abfragend.

Zu entscheiden war: wie Slouchcat im Namen einer Person wünscht, obwohl Seerr einen einzigen Schlüssel und
keine Profile kennt; wie ein Wunsch einem Profil gehört; wann ein Titel als verfügbar gilt; wie ein Wunsch
übersteht, dass Seerr nicht erreichbar ist; wie ein Client im Heimnetz zu den Regeln für ausgehende
Aufrufe passt; wie ein Passwort nach Seerr kommt, ohne dass Slouchcat es speichert; wer das Limit zählt; und
wie weit Slouchcat Radarr und Sonarr anfassen darf und wie es von ihnen erfährt.

## Entscheid

1. **Ausnahme vom Nicht-Ziel nur für Wünsche über Seerr, mit zwei eng gefassten Ausnahmen von «Steuerung
   von Arr/Seerr».** Slouchcat legt Wünsche in Seerr an, liest ihren Zustand, zeigt ihn, zieht zurück und
   meldet die Verfügbarkeit. Es legt die Benutzer der Konten in Seerr an und pflegt sie (E-Mail, Passwort,
   Rechte, Limit), und es setzt in Radarr und Sonarr Profil-Tags (anlegen, anhängen). Warteschlange und Dateistand
   der gewünschten Titel (`queue`, `movie?tmdbId`, `series?tvdbId`) liest es nur, für den Download-Fortschritt bei
   Wünschen (Entscheid 19). Der Import in die Bibliothek kommt weiter über den Webhook (Entscheid 20). Keine eigene Beschaffung,
   keine Qualitätsprofile oder Zielordner, keine Änderung der Warteschlange, kein Suchen, Hinzufügen oder Löschen in Radarr und
   Sonarr, kein Entfernen von Tags, kein eigener Freigabe-Ablauf. SABnzbd spricht Slouchcat nie an. Übrige
   Einstellungen von Seerr ändert es nicht; Benutzer löscht es nicht.
2. **Client `internal/seerr`** ohne Datenbank (Vorbild `internal/trakt`): `X-Api-Key` aus den verschlüsselten
   Einstellungen (AES-256-GCM, nie wieder angezeigt, letzte vier Zeichen), Zeitlimit 10 s, höchstens 5 Anfragen/s,
   **keine Weiterleitungen**, nur `http`/`https`. **Ausdrückliche LAN-Erlaubnis:** Der Client läuft nicht über
   `netguard`, weil der Administrator die Adresse setzt (Muster Podcast-Feed des Administrators und SMTP); es
   wird nur diese Adresse angesprochen, nie ein Wert aus einer Antwort.
3. **Im Namen des Benutzers:** `POST /api/v1/request` mit `X-API-User: <Seerr-Benutzer>` und ohne `userId`.
   So gelten Recht, Limit und Auto-Freigabe dieses Benutzers. Der Verbindungstest prüft den Header mit
   `GET /auth/me`; wirkt er nicht, lässt sich die Funktion nicht einschalten (T-1). Beim ersten erfolgreichen
   Test speichert Slouchcat einen Fingerabdruck der Instanz; weicht er später ab, halten Übermittlung und Abgleich
   an, bis der Administrator die Zuordnung bestätigt oder löscht.
4. **Ein Seerr-Benutzer je Konto (E-1, E-4):** Nimmt jemand eine Einladung an, eröffnet Slouchcat im selben
   Zug den Benutzer in Seerr mit gleicher E-Mail und gleichem Passwort (`wish_account_map` mit Herkunft
   eröffnet, verknüpft oder von Hand). Für bestehende Konten gibt es «In Seerr eröffnen» und «Alle
   eröffnen»; das Einrichten von Seerr eröffnet sie nicht von selbst. **Über die E-Mail wird sofort verknüpft
   (E-4, E-8):** Gibt es in Seerr einen Benutzer mit derselben Adresse, verknüpft Slouchcat ihn, gleich ob ein
   Administrator «In Seerr eröffnen» oder «Alle eröffnen» wählt oder das Konto die Adresse beim Annehmen einer
   Einladung oder beim Ändern seiner E-Mail selbst eintippt. Das Passwort geht an ihn nur nach einer Einladung
   oder E-Mail-Änderung (E-8); verknüpft ein Administrator, behält der Benutzer sein Passwort (F-24, N-103);
   nie an Benutzer 1, andere Administratoren von Seerr und Benutzer mit Anmeldung über Jellyfin, Emby oder Plex
   (N-91). Den Auslöser hält `wish_account_map.linked_by` fest; eine Verknüpfung nach Einladung oder
   E-Mail-Änderung zeigt die Übersicht sieben Tage lang als «Neu verknüpft» (Empfehlung, N-93). Das Konto erfährt
   nie, ob Seerr die Adresse kannte (N-94). Profile haben kein Login; sie senden
   als Benutzer des Kontos. Eine Ausnahme je Profil gibt es nicht (F-27, N-100). Konten ohne eigenen Benutzer
   senden über den Sammelbenutzer «slouchcat-gaeste», den Slouchcat anlegt (F-4, N-101, N-102). Ein
   Seerr-Benutzer gehört höchstens einem Konto. Je Wunsch speichert Slouchcat Konto, Profil,
   Seerr-Benutzer (eingefroren) und Request-ID, bei Serien je Staffel. Die Rechte eines eröffneten Benutzers
   folgen dem Wunschrecht (`REQUEST` oder keine); die eines verknüpften fasst Slouchcat nicht an. Wird ein Konto
   gelöscht, bleibt sein Seerr-Benutzer bestehen; nur einem eröffneten nimmt Slouchcat die Rechte und setzt
   ein Zufallspasswort, verknüpfte und von Hand gewählte bleiben unverändert (N-10, F-31).
5. **Wunschrecht** je Konto (`accounts.can_request`, Administrator) als Deckel und je Profil
   (`profiles.can_request`, Hauptprofil). Der Server liefert `canRequest` und einen Grund; der Client entscheidet
   nichts. Konten ohne Seerr-Benutzer senden ab Werk über den Sammelbenutzer; mit «Nicht erlauben» können sie
   nicht anfordern (F-4).
   Fehlende Titel und die Wünsche-Seiten sieht ein Profil nur, wenn es anfordern kann (F-9).
6. **Zustände** `queued`, `pending`, `processing`, `partial`, `available`, `declined`, `failed`, `withdrawn`,
   abgeleitet aus Seerr und der eigenen Bibliothek.
7. **Verfügbar ist, was Slouchcat für das Profil abspielen kann.** Auslöser ist ein Haken nach dem
   Metadaten-Match (IDs geschrieben), dazu stündlich `wishes.reconcile`. Abgleich bei Filmen über TMDB-, dann
   IMDb-ID, bei Serien wie die Bibliothek über TVDB-, dann TMDB-, dann IMDb-ID. «AVAILABLE» in Seerr löst
   allein nichts aus. Ein Film ist verfügbar mit der ersten
   abspielbaren Datei; eine Staffel erst, wenn sie vollständig ist (abspielbare Folgen = Folgenzahl laut TMDB,
   bei laufenden Staffeln die bisher ausgestrahlten). Ob es den Titel gibt und wer ihn sehen darf, beantwortet
   `visibility`; `wishes` fragt `items` nicht ab (Regel 2).
8. **Ausgangsfach:** `createWish` speichert lokal (`queued`), versucht sofort (3 s, ohne Vorprüfung) und sonst
   über `wishes.submit` mit eigener Wiederholungsleiter; nach 24 h `failed` und Fehlerereignis an
   Administratoren. Ab dem zweiten Versuch prüft der Job vor dem Senden, ob der Wunsch in Seerr schon
   besteht; 409 «Request for this media already exists.» und 202 zählen als Erfolg. **Zurückziehen** setzt lokal
   `withdrawn`; der Job `wishes.withdraw` sucht einen eigenen Request des Benutzers und löscht ihn, solange er
   `PENDING` ist. Ist er schon freigegeben, bleibt er stehen und der Wunsch wird wieder geöffnet.
9. **Abgleich** `wishes.sync` alle 5 min: `GET /request?sort=modified` absteigend bis zum letzten Abgleich; der
   Zeiger rückt je Seite vor, ein abgebrochener Lauf setzt dort fort. Der erste Abgleich und der nach einem
   Instanzwechsel laufen als Erstabgleich ohne Seitengrenze im Hintergrund. Dazu `GET /request/{id}` für offene
   Wünsche ohne Änderung seit 24 h, höchstens einmal je 24 h je Wunsch. Die Übernahme bestehender Anfragen
   (F-21) läuft als eigener Job `wishes.import`. Der Seerr-Webhook ist nur ein Anstoss (Stufe 4).
10. **Mitwunsch:** In Slouchcat höchstens ein offener Wunsch je Film bzw. Serienstaffel. Ein weiteres Profil wird
    Mitwünscher, ohne Aufruf an Seerr, und wird ebenfalls benachrichtigt. Zuschauer sehen nie, wer sonst
    gewünscht hat.
11. **Benachrichtigungen** nur über Slouchcat: neue Eingangsarten `wish_available`, `wish_season_available`,
    `wish_approved`, `wish_declined`, `wish_failed`; neue Kanal-Ereignisse `wish_available`, `wish_status`,
    `wish_review` (nur Administratoren). Stufe 2 bringt «verfügbar» für Filme und ganze Serien, Stufe 4 die
    Staffel- und Zustandsmeldungen und `wish_review`. Doppelschutz über `wish_requesters.notified_at` und einen
    eindeutigen Schlüssel je Profil. Eingangseinträge ohne Titel in der Bibliothek hängen am Wunsch und sind für
    Anforderer und Mitwünscher sichtbar. Externe Nachrichten von PIN-Profilen nennen keinen Titel.
12. **Freigeben und Ablehnen** für Administratoren als Durchreichen an `POST /request/{id}/approve|decline`
    (Stufe 4). Das ist kein Ablauf «Kind fragt Eltern» (Spec Zeile 65 bleibt).
13. **Verwaltung:** Integrationen › Seerr (Verbindung, Test, Abgleich, Protokoll) und Medien › Wünsche
    (Anfragen, Seerr-Benutzer, Einstellungen); jede Einstellung genau ein Ort. Verwaltungsantworten tragen keine
    `ItemRef`; `listAccountWishes` ist `scoped` und verlangt das Hauptprofil (Regel 7).
14. **Nur online.** `/api` bleibt NetworkOnly; Wünsche haben keinen Offline-Zustand.
15. **Passwort nur in drei Momenten (E-1).** Slouchcat gibt das Passwort an Seerr weiter, wenn es der Server
    ohnehin sieht: beim Annehmen der Einladung, beim Ändern des Passworts und bei der Anmeldung (dann nur, wenn
    es noch aussteht), und nur für Benutzer, die Slouchcat eröffnet oder nach Einladung oder E-Mail-Änderung
    verknüpft hat (F-24, N-103). Es geht nur im Speicher weiter, nie über eine Job-Nutzlast, und Slouchcat speichert und
    protokolliert es nie. Geht es nicht sofort, bekommt der Benutzer ein Zufallspasswort, und das echte folgt bei
    der nächsten Anmeldung. Benutzer 1, Administratoren von Seerr und Benutzer mit Anmeldung über Jellyfin,
    Emby oder Plex gleicht Slouchcat nicht ab. Ein Passwort, das jemand direkt in Seerr ändert, bemerkt
    Slouchcat nicht. Der
    Verbindungstest warnt bei ausgeschalteter «Lokaler Anmeldung» und bei `http` ausserhalb des eigenen Netzes.
16. **Limit je Konto (E-3).** Je Konto ein Limit für Filme und eines für Staffeln, je Anzahl und Tage, rollierend,
    für alle Profile gemeinsam. Eingestellt wird in Slouchcat (Vorgabe, je Konto «Standard», «Eigenes»,
    «Unbegrenzt»); Slouchcat schreibt es in den Seerr-Benutzer, Seerr zählt und setzt durch, auch für
    Wünsche, die direkt dort gestellt werden. Abgelehnte Wünsche und Mitwünsche zählen nicht. Benutzer mit
    `MANAGE_USERS` haben kein Limit. Slouchcat prüft vor dem Anfordern mit dem zuletzt gelesenen Stand und sagt,
    wann der nächste Platz frei wird.
17. **Profil-Tag in Radarr und Sonarr (E-2).** Ist ein Wunsch «In Beschaffung», legt Slouchcat in Radarr bzw.
    Sonarr den Tag des Profils an (`sc-<profilname>`) und hängt ihn an den Film bzw. die Serie, nachträglich auch
    für Mitwünscher. Adresse und Schlüssel liest es aus Seerr und hält die Schlüssel nur im Speicher, je
    10 Minuten; neu gelesen wird nach Ablauf oder nach einer Antwort 401. Ein
    eigenes Paket `internal/arr` kennt nur diese Aufrufe. Fehler ändern nichts am Wunsch; Tags werden nie
    entfernt.
18. **Wünsche-Seiten.** Unter «Wünsche» zeigen «Entdecken», «Filme», «Serien» und «Suche» Titel zum Anfordern. Die
    Listen holt der Server über Seerr im Namen des Benutzers, mit Zwischenspeicher; Altersstufe und
    Bibliotheksabgleich rechnet Slouchcat (ADR 0038).
19. **Download-Fortschritt nur beim Wunsch (E-7).** Solange ein Wunsch «In Beschaffung» ist, liest der Job
    `wishes.progress` jede Minute die Warteschlange je Dienst (`GET /api/v3/queue`, alle Seiten) und den Dateistand
    der gewünschten Titel (`movie?tmdbId=` bzw. `series?tvdbId=`) und ordnet die Einträge über die ID aus diesen
    Aufrufen zu. Mehr liest es nicht. Vier Anzeigen: «Wird gesucht», «Wird heruntergeladen · n % · noch m
    min», «Wird eingelesen», «Download verzögert»; bei Serien «x von y Folgen da». Gezeigt unter «Meine Wünsche», auf
    der Wunschkarte und in der Verwaltung, nie am Titel der Bibliothek, in Reihen oder in der Suche. Die Meldung von
    Radarr oder Sonarr sieht nur die Verwaltung. Der Fortschritt liegt in einem Zwischenspeicher; kein Seitenaufbau
    ruft Radarr oder Sonarr auf. Er ändert keinen Zustand; «verfügbar» bleibt, was Slouchcat abspielen kann.
    Schalter «Download-Fortschritt zeigen», ab Werk an.
20. **Webhooks wie heute (E-6).** Neue Dateien meldet weiter der Webhook von Radarr und Sonarr, den der
    Administrator von Hand einrichtet. Slouchcat trägt dort keinen Webhook ein. Der Import in die Bibliothek kommt
    weiter über den Webhook; in Radarr und Sonarr liest Slouchcat nur für die Anzeige am Wunsch (Entscheid 19). Der eingehende Teil von ADR 0010 bleibt unverändert. Ob Slouchcat aus derselben Meldung Dateipfad und
    Kennung liest, um schneller einzulesen, entscheidet E-11: als eigener Schritt nach einer Messung, der
    entfällt, wenn die Messung keinen Gewinn zeigt; die Webhooks selbst ändern sich dadurch nicht.

## Verworfen

| Alternative | Grund |
|---|---|
| eigene Wunschverwaltung oder eigener Freigabe-Ablauf in Slouchcat | doppelt zu Seerr; Spec Zeile 65 |
| Radarr/Sonarr für mehr als Tags und das Lesen von Warteschlange und Dateistand der gewünschten Titel ansprechen | Spec Zeile 69; Seerr kann es schon |
| «API und Webhook»: Slouchcat trägt den Webhook in Radarr und Sonarr selbst ein und holt Verpasstes über `GET /api/v3/history/since` nach | Marco hat «Webhooks wie heute» gewählt (E-6). Slouchcat schriebe in die Einstellungen von Radarr und Sonarr, was «registers nothing there» aus ADR 0010 bricht |
| «Nur API, ohne Webhook»: Radarr und Sonarr im Takt nach Importen abfragen | Marco hat «Webhooks wie heute» gewählt (E-6). Die API kann nicht von sich aus melden («Push»); Abfragen im Takt sind langsamer als die Sofortmeldung des Webhooks und erzeugen dauernde Grundlast (Anhang H 8.3, 8.4) |
| Download-Fortschritt am Titel der Bibliothek, in Reihen und in der Suche | Marco: «bei Wünschen» (E-7); Spec Zeile 67 bleibt dort Nicht-Ziel |
| Fortschritt im Seitenaufbau direkt bei Radarr und Sonarr lesen | jede Seite hinge an der Erreichbarkeit von Radarr und Sonarr; der Zwischenspeicher genügt |
| Profil-Tag über `tags` im Body von `POST /request` | braucht `REQUEST_ADVANCED` für jeden Benutzer, ersetzt die Standard-Tags, kann einem Mitwunsch nichts nachreichen, und Seerr legt keine Tags an (Anhang H 4.2) |
| Passwort verschlüsselt in Slouchcat speichern, um es jederzeit nachzureichen | ein zweiter Ort für das Passwort; die drei Momente reichen |
| Limit in Slouchcat zählen | Wünsche direkt in Seerr zählten nicht mit (E-3) |
| `userId` im Body statt `X-API-User` | jeder Wunsch wäre sofort freigegeben, weil Seerr die Auto-Freigabe am Aufrufer prüft |
| Ausnahme je Profil «Profil sendet als anderer Seerr-Benutzer» (`wish_profile_map`) | von der Redaktion empfohlen; Marco hat abgelehnt (F-27). Folge: Kinder im Konto des Administrators erben dessen Auto-Freigabe und sind ohne Limit |
| Seerr als Iframe einbetten | entfiel schon in Runde 20; fremde Oberfläche, eigene Anmeldung |
| Seerr-Benutzer je Profil mit eigenem Login | Marco: Profile brauchen kein Login (Anhang H 1); künstliche E-Mail-Adressen; ein gelöschtes Profil nähme seine Anfragen mit |
| Benutzer eines gelöschten Kontos in Seerr löschen | Seerr löschte seine Requests mit (Anhang D 7.2) |
| «Verknüpfung erst nach Bestätigung»: nach einer Einladung oder E-Mail-Änderung nur ein Vorschlag, den ein Administrator bestätigt (N-2a) | von der Redaktion nach dem Befund K-1 der Kritik empfohlen; Marco hat «Sofort verknüpfen» gewählt (E-8). Das Risiko steht unter «Folgen» als bekannte Grenze |
| Webhook als Grundlage | ein Ziel, keine Signatur, keine Wiederholung, keine Benutzer-ID |
| «verfügbar» nach dem Zustand in Seerr | Meldung vor der Datei in Slouchcat; Kinderprofile sähen den Titel noch nicht |
| Wünsche über `netguard` | sperrt das Heimnetz, in dem Seerr steht |

## Folgen

- Neue Pakete `internal/seerr` und `internal/wishes`; Regel 2 bleibt unberührt (neue Funktionen in `visibility`,
  Haken aus `metadata`).
- Neue Tabellen nach Vorlage; Neubau von `notifications`, `deliveries`, `home_rows` und für `can_request` von
  `accounts` und `profiles`.
- Neue Schlüsselnamen `seerr.api_key`, `newPassword` und die Schlüssel von Radarr und Sonarr in Redaction-Liste
  und Test (Regel 10).
- Neues Paket `internal/arr` für Tags und das Lesen von Warteschlange und Dateistand der gewünschten Titel; ADR 0010
  bekommt den Nachtrag zu beiden.
  Die Seite «Webhooks von Radarr und Sonarr» bleibt, wie sie ist.
- Zwölf neue Jobarten; Abgleich und Prüfung belasten Seerr alle 5 min mit wenigen Aufrufen; `wishes.progress`
  ruft Radarr und Sonarr jede Minute einmal je Dienst auf, nur solange Wünsche in Beschaffung sind.
- Die Felder der Sonarr-Warteschlange (`seriesId`, `episodeId`, `seasonNumber`) sind nicht gegen eine Instanz
  geprüft; fehlen sie, zeigt Slouchcat weniger, nie Falsches.
- Das Passwort verlässt den Server im Klartext Richtung Seerr; ändert jemand es direkt dort, laufen die beiden
  auseinander (Marco kannte die Folge, E-1).
- Benutzerpflege, Passwort-Route und `settings/main` von Seerr sind im Code von Seerr 3.5.2 belegt, aber nicht
  dokumentiert; der Bau prüft sie gegen eine Instanz.
- `X-API-User` ist undokumentiert. Fällt es in einer künftigen Version weg, meldet der Test es, und die Funktion
  bleibt aus, bis neu entschieden ist.
- Seerr verschickt weiter eigene Meldungen, wenn sie dort eingerichtet sind.
- **Bekannte Grenze (Datenschutz, K-19):** Alle Profile eines Kontos teilen das Seerr-Login des Kontos. Wer
  sich damit in Seerr anmeldet, sieht dort die Wünsche aller Profile, auch die eines Profils mit PIN. Die
  PIN-Regel F-15 gilt nur für Slouchcats eigene Nachrichten. Seerr verschickt zudem eigene E-Mails mit dem
  Titel. Empfehlung: in Seerr die eigenen Benachrichtigungen für eröffnete Benutzer ausgeschaltet lassen;
  Wünsche › Einstellungen zeigt dazu einen Hinweis.
- **Bekannte Grenze (Übernahme, E-8, N-92):** Die E-Mail beim Annehmen einer Einladung und beim Ändern des
  eigenen Kontos prüft niemand. Wer eine Einladung hat und eine fremde Adresse eintippt, bekommt den
  Seerr-Benutzer dieser Person samt ihren Anfragen. Marco kennt das Risiko und hat es so gewählt. Es begrenzen:
  Einladungen stellt nur ein Administrator aus; Administratoren von Seerr und Benutzer mit Anmeldung über
  Jellyfin, Emby oder Plex bekommen kein Passwort; die Übersicht zeigt die Verknüpfung sieben Tage lang, das
  Protokoll hält sie fest, und sie lässt sich unter «Seerr-Benutzer» lösen (Empfehlung, N-93).
- Den Sofort-Import gibt es nur, wenn die Messung vorher einen Gewinn zeigt (E-11); sonst bleibt der Empfang
  der Webhooks, wie er ist.

## Spec-Wortlaut, der hier abgelöst wird (ohne die Spec umzuschreiben)

- **Constraint Zeile 46** und **Non-Goal Zeile 66**: Wünsche entstehen auch aus Slouchcat, in Seerr.
- **Non-Goal Zeile 69**: Ausnahme für Profil-Tags in Radarr und Sonarr.
- **Non-Goal Zeile 67**: nur beim Wunsch erlaubt: Zustand eigener Wünsche, Download-Fortschritt und Benachrichtigung
  bei Verfügbarkeit; ein Beschaffungsstatus am Titel der Bibliothek, in Reihen und in der Suche und der Suchfilter
  «in Beschaffung» nicht.
- **Spec A-6**: Die Kanäle tragen die Ereignisse der Wünsche statt des alten «Beschaffungsstatus».
- **Neu Spec I-9 bis I-11** (Nummern I-1 bis I-4 bleiben unbesetzt).
```

## 5. Entwurf ADR 0038 «Titel ausserhalb der Bibliothek»

```markdown
# ADR 0038: Titel ausserhalb der Bibliothek

- Status: Entwurf, nachgeführt auf Marcos Antworten vom 17. November 2026; angenommen mit Stufe 0 (Frage 33). **Löst S-4 nicht ab**: «Titel ohne
  Bibliothekstreffer erscheinen nie auf der Startseite» (Frage 11, ADR 0011) gilt ohne Ausnahme (Marco, E-5).
  **Ergänzt ADR 0034** (Freigabeländer) um Titel ohne Eintrag in `items`.
- Quelle: Konzept `docs/planning/25-medienwuensche/` (vor allem `02-funktionskonzept.md` und
  `05-spotlight-integration.md`, Fassung 3), **Frage 33** (F-9, F-11, F-12, F-28, F-29; F-8 durch E-5 und F-34 durch E-10
  entschieden); Marcos Rückmeldungen vom 17. November 2026 (Wünsche-Seiten, TVDB; keine fehlenden Titel im Spotlight)
- Vertrag und Migrationen: vergibt der Bau
- Betroffene Spec-IDs: Spec E-1, Spec E-4, Spec E-7
- Harte Regeln: 2, 5, 6 (Bilder als `http.Handler`), 7, 12

## Kontext

Bis 1.40 kennt Slouchcat nur Titel der eigenen Bibliothek. Alles hängt an `items`: Freigabe
(`effective_rating`, `item_certifications`), Bilder, Merkliste, Empfehlungen, Eingang. Das Spotlight speichert für
Trakt-Einträge ohne Treffer nur TMDB-ID, Titel und Jahr und zeigt sie nur in der Verwaltung (S-4). Wer einen Titel
anfordern soll, muss ihn aber sehen: mit Poster, Beschreibung, Freigabe und Trailer, in der Suche und auf einer
eigenen Seite, seit Marcos erster Rückmeldung auch auf eigenen Seiten «Entdecken», «Filme», «Serien» und «Suche»
wie bei Seerr. Auf der Startseite will Marco nur vorhandene Titel (zweite Rückmeldung, E-5). Dabei darf kein Profil einen Titel sehen, den es nach seiner Altersstufe nicht sehen
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
   kann, existiert für es nicht: 404 auf der Vorschauseite und bei Bildern, kein Treffer in Suche und auf den
   Wünsche-Seiten (F-9; Ausnahme: Titel mit eigenem Wunsch). Die Regel wohnt in `visibility`. Weil die TMDB-Suche keine
   Freigaben liefert, holt der Server für Profile unter 18 die Details der ersten 20 Treffer.
3. **Bildweg:** Bilder fehlender Titel laufen über Slouchcats eigenen Bildspeicher und die Route
   `GET /titles/{media}/{tmdbId}/images/{kind}/{size}` (`http.Handler`, `scoped`, im Vertrag beschrieben), nie
   direkt von TMDB im Browser. Die Verwaltung nutzt die eigene Route `GET /admin/titles/{media}/{tmdbId}/images/…`
   (`admin`), weil Verwaltungsantworten keine scoped Adressen tragen.
4. **Vertrag:** fehlende Titel als eigenes Schema `ExternalTitleRef` neben `ItemRef` (nicht brechend);
   `getExternalTitle` und `searchExternalTitles` sind `scoped`; die Vorschauseite liegt unter
   `/titles/{movie|show}/{tmdbId}`.
5. **Wo Titel ausserhalb der Bibliothek erscheinen (S-4 bleibt):** nur im Bereich «Wünsche», im Abschnitt «Noch
   nicht in der Bibliothek» der Suche, auf der Vorschauseite und als fehlende Staffeln einer vorhandenen Serie.
   **Nie auf der Startseite** (Bühne, Posterreihe, Reihen) und nie in der Spotlight-Liste, auch nicht für Profile,
   die anfordern können, und nicht als Reihe unter «Entdecken» (E-5). Fehlende Einträge der Trakt-Liste sieht nur
   der Administrator unter Verwaltung › Spotlight, mit Poster, Zustand, IDs und «Anfordern». Die Startreihe
   «Erfüllte Wünsche» zeigt nur Titel der Bibliothek (E-10).
6. **Abgleich über mehrere IDs, in fester Reihenfolge:** Filme über TMDB-, dann IMDb-ID; Serien wie die
   Bibliothek über TVDB-, dann TMDB-, dann IMDb-ID (Marco: «konsistent»). Trakt-Einträge speichern neu TVDB- und
   IMDb-ID; ein Eintrag nur mit TVDB-ID wird über TMDB «find» aufgelöst, sonst gilt er nicht als fehlend.
   Vorhandene Serien, die nur eine TVDB-ID haben, gelten so nicht fälschlich als fehlend. Der Schlüssel zu
   Seerr bleibt die TMDB-ID, bei Serien mit `tvdbId`.
7. **Suche:** Unter der Bibliothekssuche ein Abschnitt «Noch nicht in der Bibliothek» aus der TMDB-Suche über den
   eigenen Schlüssel, gefiltert nach Bibliothek, Freigabe und Wunschrecht, mit Verweis auf die Wunsch-Suche.
8. **Wünsche-Seiten:** Unter «Wünsche» zeigen «Entdecken» (sieben feste Reihen), «Filme», «Serien» und «Suche»
   Titel ausserhalb der Bibliothek. Die Listen kommen über Seerr (Discover- und Suchrouten) mit einem
   Zwischenspeicher von 1 Stunde je Liste, Seite, Sprache und Altersstufe; Titeldaten und Freigaben bleiben in
   `external_titles`. Jeder Treffer läuft durch dieselbe Jugendschutzregel; für Profile mit Altersgrenze filtert
   Seerr mit `certificationLte` vor, Slouchcat prüft nach. Titel aus der Bibliothek erscheinen mit Marke und
   Link; massgeblich ist der Abgleich von Slouchcat, nicht `mediaInfo` von Seerr. Die Listen sind
   öffentliche Ranglisten für alle und keine algorithmische Empfehlung. Auf der Startseite erscheinen sie nicht.
9. **Serien in der Bibliothek:** `series_catalog` hält die Staffelliste laut TMDB, Status und nächste Folge; damit
   zeigt die Serienseite fehlende Staffeln und «2 von 4 Staffeln vorhanden».
10. **Keine Merkliste, kein Gesehen, kein Empfehlen** für fehlende Titel; diese hängen an Bibliothekstiteln.

## Verworfen

| Alternative | Grund |
|---|---|
| fehlende Titel als Zeilen in `items` | bräche `visible_items`, Scans, Zählungen und alle Fremdschlüssel auf `items` |
| Titeldaten über Seerr (`GET /movie/{id}`) | jeder Aufruf geht ohnehin an TMDB; die Anzeige hinge an der Erreichbarkeit von Seerr. Für die Listen der Wünsche-Seiten gilt das bewusst nicht: Sie sollen zeigen, was Seerr anbietet, und haben einen Zwischenspeicher |
| `mediaInfo` von Seerr für «in der Bibliothek» | Seerr setzt «verfügbar» nach seinem eigenen Scan, nicht nach dem Abgleich von Slouchcat (Anhang D 11.1) |
| Reihen-Einstellung von Seerr übernehmen | Reihen ohne Bedeutung in Slouchcat (Plex Watchlist, Studios, Networks), F-28 |
| Bilder direkt von `image.tmdb.org` im Browser | verrät dem Anbieter die Zuschauer, umgeht den Bild-Cache der App |
| fehlende Titel ohne Freigabe zeigen | widerspricht AN-1 (unbekannt = gesperrt unter 18) |
| fehlende Titel im Spotlight mit Schalter und Höchstzahl (Fassung 2, Fassung 3 vor der zweiten Rückmeldung) | Marco: «In Spotlight soll nicht angezeigt werden was fehlt» (E-5) |
| fehlende Einträge der Trakt-Liste als Reihe unter «Entdecken» | Marco: «Nein, nirgends» (E-5) |
| Schalter je Profil für fehlende Titel | das Wunschrecht entscheidet schon (F-9) |

## Folgen

- Neue Tabellen `external_titles`, `external_title_texts`, `external_title_images`, `series_catalog`,
  `wish_discover_pages`; `spotlight_entries` bekommt `tvdb_id` und `imdb_id`, damit vorhandene Titel auch über TVDB
  und IMDb erkannt werden; `spotlight_config` bleibt unverändert.
- Die Wünsche-Seiten belasten Seerr und TMDB; der Zwischenspeicher und der Job `wishes.discover` halten das klein.
- Mehr TMDB-Aufrufe über den bestehenden Limiter; der Bildspeicher wächst um die fehlenden Einträge des Spotlights
  (nur für die Verwaltung) und die Treffer der Wünsche-Seiten.
- **Vorschlag:** Einträge in `external_titles` ohne Bezug (nicht mehr im Spotlight, kein Wunsch, 30 Tage nicht
  gesucht) räumt `titles.refresh` weg.
- Die Spotlight-Verwaltung zeigt für fehlende Einträge Poster, Zustand, IDs und «Anfordern»; im Spotlight ändert
  sich für Zuschauer nur, dass TVDB-zuerst-Serien erscheinen.
- Die Startseite bekommt die neue Reihe «Erfüllte Wünsche» (Marco, E-10): Wünsche des Profils, die in den letzten 14 Tagen
  erfüllt wurden, also nur Titel der Bibliothek (`ItemRef`). Die Titel-IDs liefert `wishes`, die Sichtbarkeit prüft
  `visibility`. Der neue Schlüssel in `HomeRowKey` macht Stufe 3 brechend; ohne die Reihe wäre sie es nicht.
```

## 6. Vorschlag für Regel 11 in `CLAUDE.md`

Heute `CLAUDE.md:57-59`. Die Datei wird erst in Stufe 0 nach der Freigabe geändert.

```diff
 11. **Keine Nicht-Ziele bauen** (Spec "Non-Goals": Musik (ausser Live-Webradio, Frage 23), Fotos, Live-TV, TV-Apps, native Apps, Plugins,
-    algorithmische Empfehlungen, Trakt-Login, eigene Wunschverwaltung, Steuerung von Arr/Seerr, Mandanten,
-    Cloud, YouTube-Downloads). Nichts aus einem späteren Meilenstein vorziehen.
+    algorithmische Empfehlungen, Trakt-Login, eigene Wunschverwaltung (ausser Wünsche über Seerr, Frage 33,
+    ADR 0037: anlegen, Zustand zeigen, zurückziehen, Verfügbarkeit melden; Freigeben/Ablehnen nur als Durchreichen
+    für Administratoren), Steuerung von Arr/Seerr (zwei Ausnahmen nach ADR 0037: die Benutzer der Konten in
+    Seerr anlegen und pflegen, also E-Mail, Passwort, Rechte und Limit; in Radarr und Sonarr Warteschlange
+    und Dateistand der gewünschten Titel lesen (`queue`, `movie?tmdbId`, `series?tvdbId`) und den Profil-Tag
+    setzen. Qualitätsprofile, Warteschlangen ändern, Webhooks eintragen, Suchen, Hinzufügen, Löschen und SABnzbd
+    bleiben verboten), Mandanten, Cloud, YouTube-Downloads). Nichts aus einem späteren Meilenstein vorziehen.
```

Die Regel bleibt ein Punkt in der Liste der harten Regeln. Regel 3 (`os/exec` nur in `transcode`) bleibt unberührt:
`seerr` und `arr` sind reine HTTP-Clients. Regel 10 braucht keinen neuen Wortlaut, nur neue Schlüssel in Liste und
Test (Abschnitt 2.2). Die Zeile zu Regel 2 braucht keine Änderung, solange `wishes` keine Titeltabellen abfragt; käme ein neues Paket in die Liste der erlaubten Pakete, wäre das ein eigener
Entscheid in ADR 0037.

## 7. Kritik und Einarbeitung

### 7.1 Urteil der ersten Kritik

Die erste Kritik (Anhang G, 17. November 2026) prüfte Fassung 1 gegen Anhang F,
die Anhänge A bis D, den Code im Worktree und `CLAUDE.md`. **Urteil: REVISE**, 40 Befunde: 1 Blocker, 13 Major,
26 Minor. Fassung 2 arbeitet alle 40 Befunde ein; wo die Redaktionsentscheide vom 17. November 2026 etwas anderes
sagen, gilt der Redaktionsentscheid. Nur Befund 12 ist nicht übernommen.

**Wer hat entschieden?** Die Redaktionsentscheide hat die Hauptsession beim Einarbeiten der Kritik getroffen, damit
die Papiere und das Mockup dasselbe sagen. **Marco hat dazu noch nichts entschieden.** Sie sind Empfehlungen wie alles
andere in diesem Konzept und stehen zusammen mit F-1 bis F-34 und T-1, T-2 zur Freigabe.

### 7.2 Befunde und wo sie eingearbeitet sind

| Nr. | Stufe | Befund (kurz) | Einarbeitung |
|---|---|---|---|
| 1 | Blocker | `ItemRef` und scoped Bildadressen in Antworten, die nicht `scoped` sind (Regel 7) | eingearbeitet in `08-datenmodell-api.md` 4.2 (`AdminWishTitle`, `AdminWish` ohne `ItemRef`), 4.5 (`listAccountWishes` `scoped` mit Hauptprofil, `getAdminExternalTitleImage`), 4.6, 4.7; `03-architektur.md` 15.2, 16; `02-funktionskonzept.md` 5.3; `05-spotlight-integration.md` 4; `10-abnahmekriterien.md` AK-135, AK-136 |
| 2 | Major | Sieht ein Profil mit Recht, aber ohne Zuordnung, fehlende Titel? Drei Fassungen | eingearbeitet **anders als vorgeschlagen** (Redaktionsentscheid): eine Regel überall, fehlende Titel nur bei Anfordern-Können, ohne Zuordnung nicht sichtbar; in `02-funktionskonzept.md` 3, 4.5, 5.1; `04-benutzer-profilmapping.md` 9, 10, 11; `03-architektur.md` 3; `07-administration.md` 2; F-8, F-9 in Abschnitt 1; AK-113, AK-114 |
| 3 | Major | «Anfordern aus dem Spotlight» aus: keine fehlenden Titel oder nur «Details»? | eingearbeitet in Fassung 2; **seit E-5 gegenstandslos**: das Spotlight zeigt keine fehlenden Titel, der Schalter entfällt (N-72). Es bleiben «aus der Suche» und «auf Detailseiten» in `07-administration.md` 7; AK-129 entfällt |
| 4 | Major | Wann ist eine Staffel «verfügbar»? Zwei Regeln | eingearbeitet nach Redaktionsentscheid (Staffel vollständig, Seerr keine Bedingung) in `06-benachrichtigungen.md` 3.3; `03-architektur.md` 3, 8.1, 9.3; `02-funktionskonzept.md` 2, 4.7, 6; `08-datenmodell-api.md` 2.5, 2.9, 2.10; ADR 0037 Entscheid 7; AK-82, AK-125 |
| 5 | Major | «strengster Wert» widerspricht ADR 0034 und dem Code | eingearbeitet in `01-bestandsanalyse.md` 8; `02-funktionskonzept.md` 7; `03-architektur.md` 3, 8.3; `08-datenmodell-api.md` 2.9; ADR 0038 Entscheid 2; AK-28 |
| 6 | Major | Wunschmeldungen ohne `item_id` verschwinden aus dem Eingang | eingearbeitet nach Redaktionsentscheid (sichtbar für Anforderer und Mitwünscher des Wunsches) in `06-benachrichtigungen.md` 8, 11; `08-datenmodell-api.md` 2.11; `03-architektur.md` 2.1, 12; `09-umsetzungsplan.md` 2.6; AK-133 |
| 7 | Major | Abgleich kann sich festfahren (40-Seiten-Grenze ohne Fortschritt) | eingearbeitet **anders als vorgeschlagen** (Redaktionsentscheid: Zeiger je Seite, eigener Erstabgleich ohne Seitengrenze) in `03-architektur.md` 7.1, 7.2, 13; `08-datenmodell-api.md` 2.12; `09-umsetzungsplan.md` 2.5, 9; AK-52, AK-123 |
| 8 | Major | Übernahme bestehender Anfragen (F-21) geht mit dem beschriebenen Abgleich nicht | eingearbeitet als Job `wishes.import` (Redaktionsentscheid) in `03-architektur.md` 2.3, 7.3, 13; `04-benutzer-profilmapping.md` 8.2; `07-administration.md` 6, 12; `08-datenmodell-api.md` 4.5, 5; `09-umsetzungsplan.md` 4.6; F-21; AK-88 |
| 9 | Major | Serien: ein Wunsch kann mehrere Seerr-Requests haben | eingearbeitet nach Redaktionsentscheid (`wish_seasons.seerr_request_id`, nicht eindeutig) in `08-datenmodell-api.md` 2.4, 2.5; `03-architektur.md` 6.2, 6.3, 6.4; `04-benutzer-profilmapping.md` 1; AK-122. Das vorgeschlagene UNIQUE (`seerr_request_id`, `season_number`) entfällt; es gibt nur einen Index |
| 10 | Major | Zurückziehen erzeugt Geisterbestellungen | eingearbeitet nach Redaktionsentscheid (Job `wishes.withdraw`, Sperre je Wunsch, Wiederöffnen) in `03-architektur.md` 2.3, 6.3, 6.6, 8.1, 8.2, 13, 14; `02-funktionskonzept.md` 4.8; `08-datenmodell-api.md` 2.4, 5; `07-administration.md` 5; `09-umsetzungsplan.md` 2.4; AK-116, AK-117 |
| 11 | Major | Wechsel der Seerr-Instanz weist Wünsche falschen Personen zu | eingearbeitet nach Redaktionsentscheid (Fingerabdruck als **Vorschlag**) in `04-benutzer-profilmapping.md` 8.4; `03-architektur.md` 7.4, 14, 15.1; `07-administration.md` 2, 4.1, 6, 13; `08-datenmodell-api.md` 2.1, 2.12, 4.2, 4.5; `09-umsetzungsplan.md` 1.5; AK-107 |
| 12 | Major | Datenschutz: das Hauptprofil liest die Wünsche von PIN-Profilen | **nicht übernommen**, weil das Hauptprofil das Konto verwaltet (PINs setzen und entfernen, Schauzeit und Einstellungen aller Profile) und darum bewusst die Wünsche aller Profile sieht; F-15 schützt externe Kanäle, die jedes Gerät erreichen. Begründung klargestellt in `06-benachrichtigungen.md` 8, `02-funktionskonzept.md` 5.3 und F-15 |
| 13 | Major | Suche nach fehlenden Titeln: Altersprüfung braucht je Treffer einen Detailabruf | eingearbeitet in `03-architektur.md` 16; `02-funktionskonzept.md` 5.2; `08-datenmodell-api.md` 4.5; `01-bestandsanalyse.md` 20 (TE-23); ADR 0038 Entscheid 2; AK-115, AK-137 |
| 14 | Minor | 403 «unbekannter Benutzer» und «falscher Schlüssel» nicht zu unterscheiden | eingearbeitet in `03-architektur.md` 6.4, 14; `04-benutzer-profilmapping.md` 8.1 |
| 15 | Minor | Rückfüllung von `wish_review`: drei Zeitpunkte | eingearbeitet in `06-benachrichtigungen.md` 6; `08-datenmodell-api.md` 2.11, 3.2; `09-umsetzungsplan.md` 4.3; AK-63, AK-134 |
| 16 | Minor | ADR 0037 «je Stufe additiv» gegen `08` 4.6 «brechend» | eingearbeitet in ADR 0037 (Kopf), `06-benachrichtigungen.md` 11, `09-umsetzungsplan.md` 10 (Fassung 3; in Fassung 2 Abschnitt 9), `README.md` |
| 17 | Minor | Fehlercode bei zu vielen Wünschen | eingearbeitet in `03-architektur.md` 15.4 (`too_many_attempts`) |
| 18 | Minor | Als welches Profil fordert der Administrator in der Spotlight-Verwaltung an? | eingearbeitet in `05-spotlight-integration.md` 4; `07-administration.md` 9 |
| 19 | Minor | Freigeben als Benutzer 1 oder als zugeordneter Administrator? | eingearbeitet in `04-benutzer-profilmapping.md` 2.2 |
| 20 | Minor | Stufe der Eingangsmeldung «Abgelehnt» | eingearbeitet in `02-funktionskonzept.md` 4.4, 4.6; `06-benachrichtigungen.md` 2 (Stufen nach Redaktionsentscheid) |
| 21 | Minor | Zwei Zustandsdiagramme, verschieden | eingearbeitet: `02-funktionskonzept.md` 2 verweist auf `03-architektur.md` 8.2, das einzige Diagramm |
| 22 | Minor | «aufheben» gegen «Ausnahme» bei Spec-Zeile 46 | eingearbeitet in `09-umsetzungsplan.md` 2 (Schritt 0.1), AK-1 |
| 23 | Minor | «Erneut anfordern» nach `arr_failed` führt in eine Schleife | eingearbeitet in `03-architektur.md` 8.1; `02-funktionskonzept.md` 2; `07-administration.md` 5 |
| 24 | Minor | Einzelprüfung trifft jeden Lauf dieselben Wünsche | eingearbeitet in `03-architektur.md` 7.1; `08-datenmodell-api.md` 2.4 (`checked_at`); AK-124 |
| 25 | Minor | Sofortversuch mit zwei Seerr-Aufrufen in 3 s | eingearbeitet in `03-architektur.md` 6.1, 6.3, 6.5, 9.1; `02-funktionskonzept.md` 4.4; ADR 0037 Entscheid 8 |
| 26 | Minor | Prioritäten gegen die Regel der Queue | eingearbeitet in `03-architektur.md` 13; `09-umsetzungsplan.md` 2.2, 4.1 |
| 27 | Minor | Ort des Hakens `WithMatched` | eingearbeitet in `03-architektur.md` 11; `06-benachrichtigungen.md` 3.1; `09-umsetzungsplan.md` 2.5 |
| 28 | Minor | Doppelschutz hängt an Zeilen, die nach 90 Tagen gelöscht werden | eingearbeitet in `06-benachrichtigungen.md` 3.2, 7, 11; `08-datenmodell-api.md` 2.5, 2.6; `03-architektur.md` 12; AK-126 |
| 29 | Minor | Der «stündliche Test» hat keinen Job | eingearbeitet in `03-architektur.md` 7.1, 13, 14; AK-4 |
| 30 | Minor | «Seerr verwenden» aus lässt Wünsche nach 24 h scheitern | eingearbeitet als **Vorschlag** «Frist ruht» mit Teilfrage T-2 (Abschnitt 1) in `03-architektur.md` 6.3, 14; `07-administration.md` 4.6; `02-funktionskonzept.md` 4.4; AK-111 |
| 31 | Minor | Gespeicherter Schlüssel geht an eine neu eingetippte Adresse | eingearbeitet in `03-architektur.md` 15.3; `07-administration.md` 4.1; `08-datenmodell-api.md` 4.5; AK-105 |
| 32 | Minor | Mindestversion «Seerr ≥ 2.0», geprüft ist nur 2.7.3 | eingearbeitet in `03-architektur.md` 15.1; `07-administration.md` 4.1; `08-datenmodell-api.md` 4.2; AK-106 |
| 33 | Minor | Unbelegte API-Details (`filter=all`, `externalIds.tvdbId`, `media.seasons[]`) | eingearbeitet in `03-architektur.md` 7.1 (`filter` weggelassen, «nicht verifiziert»); `08-datenmodell-api.md` 5 |
| 34 | Minor | Gelöschtes Konto: «geschlossen» oder «nicht zuordenbar»? | eingearbeitet **anders als vorgeschlagen** (Redaktionsentscheid: Wünsche werden geschlossen): offene `failed` mit `account_deleted`, Request-IDs in den Verlauf, damit der Teilindex frei wird; in `04-benutzer-profilmapping.md` 10; `03-architektur.md` 8.1; `08-datenmodell-api.md` 2.4, 2.6; `07-administration.md` 8; AK-128 |
| 35 | Minor | Gleichzeitiges Anfordern zweier Profile | eingearbeitet in `03-architektur.md` 6.1, 6.5; `09-umsetzungsplan.md` 2.4; AK-121 |
| 36 | Minor | Tote oder ungenaue Belege | eingearbeitet in `01-bestandsanalyse.md` 1, 7, 8, 9; `03-architektur.md` 2.4, 3, 7.2, 10, 13; `05-spotlight-integration.md` 1; `07-administration.md` 4.4; `08-datenmodell-api.md` 2.7, 2.8, 2.12; F-13 |
| 37 | Minor | Verweise ohne Dateinamen und tote Verweise | eingearbeitet in `02-funktionskonzept.md` (Einleitung, 3, 4.7), `01-bestandsanalyse.md` 1, `README.md` (Jugendschutz «–»). Der Link `mockup-abdeckung.md` bleibt; die Datei entsteht gesondert. Den Ordner `.omc/` im Konzeptordner hat diese Fassung nicht angefasst; er gehört vor dem Commit gelöscht oder in `.gitignore` |
| 38 | Minor | Grundlage im Repo nicht auffindbar | eingearbeitet: Berichte als `anhang/A` bis `anhang/F` im Repo, alle Verweise auf «Bericht», «Bestandsanalyse», «Entscheid-Blatt» und Scratchpad-Pfade umgestellt; `README.md` erklärt den Anhang |
| 39 | Minor | Lücken in den Abnahmekriterien | eingearbeitet in `10-abnahmekriterien.md` AK-105 bis AK-137 sowie AK-4, AK-7, AK-63 |
| 40 | Minor | Unmarkierte Abweichungen von Anhang F | eingearbeitet in `04-benutzer-profilmapping.md` 2.3, `07-administration.md` 4.6, 7, `08-datenmodell-api.md` 2.12 und Teilfrage T-1; «Wünsche aktiv» ab Werk aus ist jetzt Redaktionsentscheid |

Die Verweise auf `05-spotlight-integration.md` sind nach dessen Umbau in der zweiten Rückmeldung nachgeführt.
Abschnitte von `05`, die nur fehlende Titel auf der Startseite beschrieben, gibt es nicht mehr; ihre Befunde sind
dort, wo sie noch gelten, in den übrigen Papieren eingearbeitet.

### 7.3 Abweichungen von Anhang F

Anhang F bleibt als Zwischenstand stehen und wird nicht nachgeführt. Wo die Papiere davon abweichen, gilt das
Papier.

| Stelle in Anhang F | Dort | Jetzt (massgeblich) |
|---|---|---|
| 2.7 Jugendschutz | «Freigabeland-Reihenfolge nach ADR 0034, strengster Wert» | Länderreihenfolge nach ADR 0034, das erste Land mit bekanntem Wert gilt; strengster Wert nur innerhalb eines Lands über die Anbieter; unbekannt = 18 (`03-architektur.md`, Abschnitt 3). Der Wortlaut in Anhang F ist falsch |
| 2.13, 3 Fehlende Titel sehen | «Profile ohne Wunschrecht sehen keine fehlenden Titel»; «Konto ohne Zuordnung» als Fall am Knopf; fehlende Titel im Spotlight | Fehlende Titel sieht nur, wer sie anfordern kann (auch Zuordnung, Art, Alter); kein Hinweis statt Knopf; Ausnahme eigene Wünsche (F-9). Im Spotlight und auf der Startseite gar nicht (E-5) |
| 6 `listAccountWishes` | `account` | `scoped`, zusätzlich Hauptprofil; Verwaltungsantworten ohne `ItemRef`, eigene Admin-Bildroute |
| 4 Version | Seerr ≥ 2.0 | Seerr ≥ 2.7 bzw. Seerr ≥ 3.0; ältere «nicht geprüft» |
| 4 Hintergrundjobs | sechs Jobarten | acht: dazu `wishes.withdraw` und `wishes.import`; Erstabgleich als Lauf von `wishes.sync` |
| 2.9 Abgleich | Abbruch beim Zeiger; Einzelprüfung offener Wünsche nach 24 h | Zeiger je Seite, Erstabgleich ohne Seitengrenze, Einzelprüfung höchstens einmal je 24 h je Wunsch |
| 5 Datenmodell | `wishes.seerr_request_id` je Wunsch; keine Spalten für Doppelschutz und Instanz | Request-ID bei Serien je Staffel; neu `wish_seasons.tmdb_id`, `seerr_request_id`, `notified_at`; `wish_requesters.notified_at`; `wishes.withdraw_pending`, `checked_at`; `seerr_users.created_at`; Tabellen `seerr_log`, `external_title_images` |
| 9 F-21 | Übernahme ohne eigenen Weg | eigener Job `wishes.import` beim Setzen einer Zuordnung und von Hand |
| 10 Stufen der Meldungen | Stufe 4 «Staffel-Meldungen» und «Ereignis Zustand»; `wish_review` ohne Stufe | Stufe 2 «verfügbar» (Film und ganze Serie); Stufe 4 Staffel, `wish_status`, `wish_review` |
| 4 Einstellungen und Zuordnung | ohne Angabe zum Speichern | Einstellungen gelten sofort; Zuordnung direkt in der Liste; Reiter «Wünsche» nach «Pflege»; Reihe «Erfüllte Wünsche» nach «Weiterschauen» (wie im Mockup) |
| neu | – | Fingerabdruck der Instanz (**Vorschlag**); Zurückziehen über einen Job ohne Geisterbestellung; geschlossene Wünsche machen den Titel wieder anforderbar; Konto gelöscht schliesst seine Wünsche |

### 7.4 Fassung 3: Marcos erste Rückmeldung vom 17. November 2026

**Was kam.** Marco hat Fassung 2 durchgesehen («Sieht schonmal sehr gut aus») und vier Ergänzungen verlangt:
Konten in Seerr mit denselben Login-Daten eröffnen, Profile nur mit Tag; ein Limit je Konto, rollierend;
eine Seite für Wünsche wie die Startseite von Seerr mit Discover, Movies, Series und Search; Serien über TVDB
konsistent. Auf vier Rückfragen hat er E-1 bis E-4 geantwortet (Abschnitt 0). Wortlaut, neue Fakten aus dem Code
von Seerr 3.5.2 und die abgeleiteten Regeln N-1 bis N-65 stehen in [Anhang H](anhang/H-nachtrag-17-november.md).

**Wer hat entschieden?** Marco hat in dieser ersten Rückmeldung nur seine Nachricht und E-1 bis E-4 entschieden (die zweite Rückmeldung mit E-5 bis E-7: Abschnitt 7.5). Alles andere aus Fassung 3
(Name des Tags, Vorgabe des Limits, Ablauf der Eröffnung, Zeitpunkt des Tags, Reihen auf «Entdecken», Quelle der
Suche, Reihenfolge des Abgleichs, Stufe 5) ist Redaktion und steht als F-23 bis F-32 oder als **Vorschlag** zur
Freigabe.

**Wie eingearbeitet.**

| Thema | Regeln (Anhang H) | Papiere dieses Teils |
|---|---|---|
| Seerr-Benutzer je Konto, eröffnen, verknüpfen, Passwort | N-1 bis N-12, E-1, E-4 | `03-architektur.md` 2.2, 2.3, 4, 14, 15, 18; `07-administration.md` 1.1, 2, 4.1, 4.7, 6, 7, 8; `08-datenmodell-api.md` 2.1, 2.2, 4.5, 4.6, 5; `09-umsetzungsplan.md` 1.5a, 1.5b; AK-139 bis AK-150 |
| Limit | N-30 bis N-38, E-3 | `03-architektur.md` 6.1, 19; `07-administration.md` 5, 6, 7, 8; `08-datenmodell-api.md` 2.1, 2.13, 4.4; `09-umsetzungsplan.md` 1.5c, 2.4; AK-151 bis AK-155, AK-158 bis AK-165, AK-168 |
| Profil-Tag in Radarr und Sonarr | N-20 bis N-28, E-2 | `03-architektur.md` 2.6, 20; `07-administration.md` 4.8, 5, 7; `08-datenmodell-api.md` 2.14, 6; `09-umsetzungsplan.md` 4.6a bis 4.6c; AK-171 bis AK-178 |
| Ausnahme je Profil | N-40, N-41 | F-2, F-27; `07-administration.md` 6; `08-datenmodell-api.md` 2.2 |
| Bereich «Wünsche» | N-50 bis N-58 | `03-architektur.md` 21; `08-datenmodell-api.md` 2.15, 4.5; `09-umsetzungsplan.md` 7 (Stufe 5); AK-179 bis AK-190 |
| TVDB durchgängig | N-60 bis N-65 | `03-architektur.md` 5, 6.2; `07-administration.md` 9; AK-166, AK-167, AK-169, AK-170 |
| Regeln und ADRs | Anhang H 5 | Abschnitte 2, 3, 4, 5, 6 |

**Wo Fassung 3 von Fassung 2 abweicht.**

| Stelle | Fassung 2 | Fassung 3 |
|---|---|---|
| Zuordnung | Zuordnung C «hybrid» empfohlen; der Administrator ordnet zu, Slouchcat schlägt bei gleicher E-Mail nur vor | ein Seerr-Benutzer je Konto, eröffnet oder automatisch über die E-Mail verknüpft (E-1, E-4); die Zuordnung je Profil ist nur noch zugeklappte Ausnahme (F-27); Seite «Zuordnung» heisst «Seerr-Benutzer» |
| Benutzer in Seerr anlegen | verworfen, Verwaltung von Seerr Nicht-Ziel | Kern der Fassung (E-1); Ausnahme vom Nicht-Ziel, eng gefasst |
| Radarr und Sonarr | Slouchcat spricht sie nie an | nur für den Profil-Tag (E-2) und das Lesen des Download-Fortschritts (E-7); Webhooks wie heute (E-6); ADR 0010 mit Nachtrag |
| Spotlight | fehlende Titel auf Bühne und in der Liste, mit Schalter | nur Bibliothekstitel wie heute (E-5); Abgleich über TVDB und IMDb; fehlende Einträge nur in der Verwaltung |
| Kontingent | nur aus Seerr gelesen und angezeigt | «Limit», in Slouchcat eingestellt und in Seerr geschrieben, Vorprüfung beim Anfordern (E-3) |
| Seite «Entdecken» | Nicht-Ziel (Anhang F 2.14) | Bereich «Wünsche» mit vier neuen Reitern, Stufe 5 (Marcos Nachricht) |
| Abgleich der Serien | TMDB, dann TVDB, dann IMDb | TVDB, dann TMDB, dann IMDb, wie die Bibliothek (N-61) |
| Staffeln im Dialog | aus `external_titles` bzw. `series_catalog` (TMDB) | aus Seerr (N-63); Rückfall TMDB |
| Kein Aufruf an Seerr im Seitenaufbau | ohne Ausnahme | Ausnahmen für die Wünsche-Seiten und die Staffelliste, mit Zwischenspeicher |
| Jobarten | acht | zwölf: dazu `seerr.provision`, `wishes.tag`, `wishes.discover`, `wishes.progress` |
| Stufen | 0 bis 4 | 0 bis 5 |
| Fragen | F-1 bis F-22 | F-1 bis F-34; F-2 und F-4 in der Sache weitgehend durch E-1, E-4 geklärt, F-8 durch E-5 entschieden, F-20 durch F-34 ersetzt, F-11 auf zwei Stellen erweitert, F-33 neu (Abschnitt 7.5) |

**Wo Anhang H nicht eindeutig war und die Redaktion gewählt hat.**

- **S-4 und die Wünsche-Seiten.** Anhang H N-58 sagt, S-4 «bleibt». Fassung 2 löst S-4 mit ADR 0038 für das
  Spotlight ab. Gewählt war zunächst: ADR 0038 löst S-4 nur für das Spotlight ab. **Überholt durch E-5**
  (Abschnitt 7.5): S-4 bleibt ohne Ausnahme.
- **Gelöschtes Konto, verknüpfter Benutzer.** N-10 unterscheidet nicht zwischen eröffneten und verknüpften
  Benutzern; N-9 fasst die Rechte verknüpfter Benutzer nicht an. Gewählt: Rechte weg und Zufallspasswort nur bei
  eröffneten Benutzern; verknüpfte und von Hand gewählte bleiben unverändert (F-31; nach der Kritik an Fassung 3
  überall gleich, K-2, Abschnitt 7.6).
- **Passwort im Job.** Anhang H nennt den Job `seerr.provision` als Auslöser der Eröffnung. Weil die Queue
  Nutzlasten in der Datenbank hält, trägt der Job nie ein Passwort; die Übergabe in den drei Momenten läuft im
  Speicher (`03-architektur.md`, Abschnitt 18.3).
- **«Unbegrenzt» in Seerr.** Anhang H belegt nicht, welcher Wert in Seerr «ohne Limit» heisst; als
  Vorschlag steht 0, nicht verifiziert.
- **Adresse von Radarr und Sonarr aus einer Antwort.** Fassung 2 ruft nie eine Adresse aus einer Antwort von
  Seerr auf; N-23 verlangt genau das. Gewählt: erlaubt, aber erst nach einer Bestätigung durch den
  Administrator (Vorschlag).
- **Grenzen des Limits** (Anzahl 0 bis 100, Tage 1 bis 90) stehen im Mockup, nicht in Anhang H; hier als Vorschlag
  übernommen.

### 7.5 Zweite Rückmeldung vom 17. November 2026

**Was kam.** Marco hat das Mockup der Fassung 3 angesehen und zwei Dinge geändert (Anhang H 8.1): Im Spotlight soll
nicht angezeigt werden, was fehlt; nur vorhandene Filme und Serien sollen auf der Startseite stehen; die
Verwaltung ist «gut so». Dazu fragte er, ob Radarr und Sonarr direkt angebunden werden sollen, damit der Import
«rasendschnell» geht, und ob die Webhooks dann wegfallen. Auf drei Rückfragen hat er E-5 bis E-7 geantwortet
(Abschnitt 0): «Nein, nirgends», «Webhooks wie heute», «Ja, bei Wünschen».

**Wer hat entschieden?** Marco hat nur seine zweite Nachricht und E-5 bis E-7 entschieden. Die Regeln N-70 bis
N-88 in Anhang H 8.4 sind sonst Redaktion: die vier Anzeigen des Fortschritts, der Takt von 1 min, der Schalter,
die Reihe «Erfüllte Wünsche» (F-34) und der Sofort-Import (F-33). Fassung 3 behält ihre Nummer; beide Rückmeldungen
vom 17. November gehören dazu.

**Neue Fakten.** Gegen Marcos Radarr 6.4.4.10685 und Sonarr 4.0.20.3014 nur lesend geprüft (Anhang H 8.3): Radarr
und Sonarr melden von sich aus nur über «Connect» (Webhook); die API ist rein abfragend. Die Warteschlange
`GET /api/v3/queue` trägt `size`, `sizeleft`, `timeleft`, `trackedDownloadStatus`, `errorMessage` und bei Radarr
`movieId`; die Sonarr-Felder `seriesId`, `episodeId`, `seasonNumber` sind nicht geprüft. Die schreibenden Aufrufe
für Tags bleiben nicht geprüft. Im Repo: Die aufgezeichneten Webhook-Meldungen tragen Dateipfad und Kennungen,
Slouchcat liest davon nur `eventType` und den Ordner.

**Wie eingearbeitet.**

| Thema | Regeln (Anhang H 8.4) | Papiere dieses Teils |
|---|---|---|
| Spotlight nur mit Bibliothekstiteln, fehlende Einträge nur in der Verwaltung | N-70 bis N-73, N-75, E-5 | `03-architektur.md` 1 (G-11), 3, 5.1, 16; `07-administration.md` 1.1, 7, 9; `08-datenmodell-api.md` 1.1, 2.11, 2.12, 3.2, 4.6; `09-umsetzungsplan.md` 5 (Stufe 3); AK-69 bis AK-79 (teils «entfällt (E-5)»), AK-191 bis AK-194; F-8; ADR 0038 Entscheid 5 |
| Startreihe «Erfüllte Wünsche» | N-74, F-34 | `03-architektur.md` 2.1; `07-administration.md` 10; `08-datenmodell-api.md` 2.11, 4.6; `09-umsetzungsplan.md` 3.4; AK-78, AK-195; F-20, F-34 |
| Webhooks wie heute | N-80, E-6 | `03-architektur.md` 10; `07-administration.md` 3, 4.8; `08-datenmodell-api.md` 6; AK-196; ADR 0037 Entscheid 20 und «Verworfen» |
| Download-Fortschritt | N-81 bis N-87, E-7 | `03-architektur.md` 2.6, 13, 14, 16, 22; `07-administration.md` 2, 4.2, 4.8, 5, 7, 12, 13; `08-datenmodell-api.md` 2.12, 2.16, 4.2, 4.6, 6; `09-umsetzungsplan.md` 4.6a, 4.6d, 4.6e; AK-197 bis AK-210; ADR 0037 Entscheid 19 |
| Sofort-Import über die Webhook-Meldung | N-88, F-33 | `03-architektur.md` 10.1; `09-umsetzungsplan.md` 6a; AK-212 |
| Regeln und ADRs | Anhang H 8.5 | Abschnitte 2, 3, 4, 5, 6; AK-1, AK-138, AK-211 |

**Was sich gegenüber Fassung 3 vor der zweiten Rückmeldung ändert.**

| Stelle | vorher | jetzt |
|---|---|---|
| S-4 | für das Spotlight durch ADR 0038 abgelöst | gilt ohne Ausnahme (E-5) |
| Spotlight | fehlende Titel auf Bühne, Posterreihe und Spotlight-Liste mit Schalter, Höchstzahl und Reihenfolge | nur Bibliothekstitel; Schalter entfallen; Abgleich über TVDB und IMDb bleibt |
| Verwaltung › Spotlight | Band «Fehlende Titel» mit Schaltern | Band «Fehlende Einträge» ohne Schalter; Einträge und IDs wie vorher |
| Wünsche › Einstellungen | Schalter «Anfordern aus dem Spotlight» | entfällt; neu «Download-Fortschritt zeigen» |
| Startreihe | Reihe mit offenen und kürzlich erfüllten Wünschen (F-20) | «Erfüllte Wünsche», nur Bibliothekstitel (Empfehlung, F-34) |
| Radarr und Sonarr | API nur für den Profil-Tag | API für Profil-Tag und Lesen von Warteschlange und Dateistand der gewünschten Titel (K-5); Webhooks unverändert |
| Spec `:67` | Ausnahme nur für den Zustand eigener Wünsche aus Seerr | dazu der Download-Fortschritt, nur beim Wunsch |
| ADR 0010 | eine Ausnahme (Tag) | zwei Ausnahmen (Tag setzen; Warteschlange und Dateistand der gewünschten Titel lesen); Webhook-Empfang bleibt |
| Stufe 3 | 5 Tage mit Mischregel der Bühne | 2,5 Tage: Abgleich über drei IDs, Verwaltung, Startreihe |
| Stufe 4 | 9 Tage | 10,5 Tage mit dem Fortschritt; dazu wahlweise Schritt 6a (F-33) |

**Wo Anhang H 8 nicht eindeutig war und die Redaktion gewählt hat.**

- **Spec `:69` und das Lesen der Warteschlange.** Anhang H 8.5 nennt für ADR 0010 zwei Ausnahmen, für Spec `:69`
  nichts Neues. Gewählt: Lesen ist keine «Steuerung»; `:69` behält eine Ausnahme (Tag) und sagt, dass
  Warteschlange und Dateistand der gewünschten Titel nur gelesen werden (K-5). Regel 11 nennt das Lesen ausdrücklich.
- **Wann eine Anzeige veraltet.** Anhang H sagt nichts dazu. Gewählt (**Vorschlag**): nach 5 min zeigt die
  Zuschauer-App nichts Zusätzliches.
- **Folgen je Staffel bei Sonarr.** Anhang H 8.3 nennt `statistics` nur für die ganze Serie. Gewählt: «x von y
  Folgen da» je Staffel nur, wenn der Bau die Zahlen in `seasons` gegen Marcos Sonarr bestätigt.
- **Anfordern in der Verwaltung › Spotlight.** N-71 nennt «Anfordern»; als welches Profil, sagt Anhang H 8 nicht.
  Gewählt wie bisher: als Hauptprofil des Administrators.

### 7.6 Kritik an Fassung 3

**Urteil.** Ein Kritiker hat die Papiere der Fassung 3 gegen Anhang H, `CLAUDE.md`, den Vertrag und das Mockup
gelesen. **Urteil: REVISE**, 23 Befunde: 1 Blocker, 4 Major, 18 Minor (Anhang H, Abschnitt 9). Alle 23 sind
eingearbeitet.

**Wer hat entschieden?** Die Entscheide zu K-1 bis K-23 hat die Redaktion getroffen; **Marco hat dazu nichts
entschieden.** Sie sind Empfehlungen wie alles andere in diesem Konzept und stehen mit F-1 bis F-34 und T-1, T-2
zur Freigabe. Wo ein Entscheid eine Antwort von Marco berührt, steht es dabei: K-1 schränkte E-4 für einen Fall
ein, K-11 trennt E-1 von der Lesart der Redaktion.

**Nachtrag: K-1 hat Marco anders entschieden.** In der dritten Runde (Abschnitt 7.7, E-8) wählte Marco «Sofort
verknüpfen» statt der empfohlenen Bestätigung. N-2a ist zurückgezogen (N-90). Die Einarbeitung von K-1 in der
Tabelle unten ist damit überholt; was jetzt gilt, steht in 7.7. Ebenso überholt ist die Zählung in K-10 (jetzt
29 offene Fragen, 7.7).

| Nr. | Stufe | Befund (kurz) | Einarbeitung |
|---|---|---|---|
| K-1 | Blocker | Übernahme eines fremden Seerr-Benutzers über eine frei eingetippte E-Mail (Einladung, E-Mail-Änderung); das Mockup verriet, ob die Adresse in Seerr existiert | N-2a: über die E-Mail verknüpft nur ein Administrator, sonst Zustand «Verknüpfung vorgeschlagen» ohne Passwort und ohne Anfordern; schränkt E-4 für diesen Fall ein. `03-architektur.md` 14, 15.4, 18.1 bis 18.3; `07-administration.md` 2, 4.7, 6 (M-24), 7 (M-25), 8 (M-26), 12, 13, 14; `08-datenmodell-api.md` 2.2, 4.5 (`confirmSeerrAccountLink`), 4.6, 4.7; `09-umsetzungsplan.md` 3 (1.5a, 1.5b, 1.8), 10; AK-141, AK-142, AK-150, neu AK-213 bis AK-218; F-24; ADR 0037 Entscheid 4 und 15 |
| K-2 | Major | Konto löschen: N-9 gegen N-10 | nur ein eröffneter Benutzer verliert Rechte und bekommt ein Zufallspasswort. `03-architektur.md` 18.4; `07-administration.md` 8, 14; `08-datenmodell-api.md` 2.2, 4.6 (`deleteAccount`); `09-umsetzungsplan.md` 1.5a (beide Fälle); AK-148, neu AK-219; F-31; ADR 0037 Entscheid 4 |
| K-3 | Major | Chip «Dein Wunsch ist da» auf der Spotlight-Bühne | Bühne ohne Wunsch-Chip; Chip auf der Detailseite (M-05), erfüllte Wünsche in der Reihe «Erfüllte Wünsche». `10-abnahmekriterien.md` AK-131, M-01, M-05 |
| K-4 | Major | Staffelliste für eine Serie der Bibliothek ohne Weg im Vertrag | `getWishSeasons` auch für eine vorhandene Serie mit fehlenden Staffeln, eigene 404-Fälle. `08-datenmodell-api.md` 4.5; `03-architektur.md` 6.2 |
| K-5 | Major | Ausnahme in Regel 11 und ADR 0010 nannte nur die Warteschlange | N-81 genauer: Warteschlange und Dateistand der gewünschten Titel lesen (`queue`, `movie?tmdbId`, `series?tvdbId`) und Profil-Tag setzen; der Import kommt weiter über den Webhook. `03-architektur.md` Kurz gesagt, 1 (G-1), 2.3, 2.5, 2.6, 10; `07-administration.md` 4.8, 12; `08-datenmodell-api.md` 6; `09-umsetzungsplan.md` 4.6a, 10; AK-176; Abschnitte 0, 2.1, 2.2, 2.3, 3, 6; ADR 0037 Status, Entscheid 1, 19, 20, Folgen |
| K-6 | Minor | `09` zitierte 8.7 mit «Stufe 2» | Satz in `09-umsetzungsplan.md` (Stand) gestrichen; ebenso der Punkt «Stufe des Spotlights» in 7.5 |
| K-7 | Minor | neuer Einleitungssatz für die Seite «Webhooks» | Seite bleibt wörtlich, kein neuer Satz, auch nicht als Vorschlag. `07-administration.md` 3; Abschnitt 2.2; AK-196 bleibt streng |
| K-8 | Minor | `GET /api/v3/queue` liefert Seiten | alle Seiten lesen oder `pageSize` setzen, vor dem Bau prüfen (nicht geprüft). `03-architektur.md` 22.4; `08-datenmodell-api.md` 6; `09-umsetzungsplan.md` 4.6a, DoD Stufe 4, 10 |
| K-9 | Minor | Einschränkungen in `01` Abschnitt 20 hiessen E-n wie Marcos Antworten | dort neu TE-n; Spec-IDs mit «Spec» davor. Abschnitt 2.1 (`:120`, `:149`), ADR 0037 und 0038 «Betroffene Spec-IDs», «Spec-Wortlaut»; 7.2 Befund 13 (TE-23) |
| K-10 | Minor | Zählung der offenen Fragen | F-1 bis F-34; F-8 entschieden, F-20 ersetzt: 32 offen, dazu T-1 und T-2. Kopf dieses Papiers |
| K-11 | Minor | F-24 las E-1 als «gilt für alle Konten» | E-1 fragte nach eingeladenen Benutzern; der Rest ist Lesart der Redaktion. F-24 |
| K-12 | Minor | `07` sagte «bei Abweichung gilt das Mockup» | Papier und Mockup sind angeglichen; bei Abweichung gilt das Papier. `07-administration.md` Stand |
| K-13 | Minor | Routen des Bereichs | `/discover` für «Entdecken», «Filme», «Serien», «Suche», `/wishes` für «Meine Wünsche»; Teil des anderen Redaktionsstrangs (`02-funktionskonzept.md`) |
| K-14 | Minor | «Bis dahin kann das Konto nicht anfordern» war unklar | Anfordern geht, sobald der Seerr-Benutzer besteht; das Passwort spielt keine Rolle. Hauptort im anderen Redaktionsstrang (`02`, `04`); in `03-architektur.md` 15.4 und 18.1 hängt «kein Anfordern» nur am fehlenden Benutzer (Vorschlag nach N-2a), nie am Passwort |
| K-15 | Minor | Eröffnet das Einrichten von Seerr bestehende Konten? | nein; nur das Annehmen einer Einladung eröffnet von selbst. `07-administration.md` 7, 8, 14; F-23; ADR 0037 Entscheid 4; Abschnitt 3 |
| K-16 | Minor | Reihe «Erfüllte Wünsche» braucht Daten, die nur `wishes` lesen darf | Titel-IDs vom Dienst `wishes`, Sichtbarkeit über `visibility`. `03-architektur.md` 2.1; `09-umsetzungsplan.md` 3.4; ADR 0038 Folgen |
| K-17 | Minor | AK-177 liess Mia ohne Wunschrecht mitwünschen | Beispiel mit Noah. AK-177 |
| K-18 | Minor | gleiche Profilnamen: Tags werden nie umbenannt | N-21 genauer: das erste Profil behält `sc-kinder`, ein späteres bekommt `sc-kinder-grosseltern`. `03-architektur.md` 20.1; `08-datenmodell-api.md` 2.14; AK-178 |
| K-19 | Minor | Datenschutz: geteiltes Seerr-Login, eigene E-Mails von Seerr | Hinweis in Wünsche › Einstellungen (`07-administration.md` 7); bekannte Grenze in ADR 0037 «Folgen» mit Empfehlung, die Benachrichtigungen von Seerr für eröffnete Benutzer aus zu lassen; `04`, `06` im anderen Strang |
| K-20 | Minor | ADR 0038 «Folgen» ohne die neue Startreihe | ergänzt (F-34) |
| K-21 | Minor | zurückgezogener Wunsch mit fremdem Request pauschal «In Beschaffung» | Zustand folgt dem Request (`pending` oder `processing`). `03-architektur.md` 6.6 Schritt 6 |
| K-22 | Minor | Abschnitt 0 nannte `internal/arr` «nur für Tags» | Tags und Lesen des Fortschritts. Abschnitt 0, Zeile E-2 |
| K-23 | Minor | Schlüssel von Radarr und Sonarr jede Minute neu aus Seerr | 10 Minuten im Speicher, neu nach Ablauf oder nach 401. `03-architektur.md` 2.6, 20.2, 20.3, 22.4; `08-datenmodell-api.md` 6; `09-umsetzungsplan.md` 4.6a, 10; ADR 0037 Entscheid 17 |

### 7.7 Dritte Runde vom 17. November 2026 (E-8 bis E-11)

Marco fragte nach Fassung 3: «Hast du noch offene Fragen oder Unklarheiten?» Vier Weichen wurden ihm vorgelegt;
seine Antworten sind E-8 bis E-11 (Anhang H 10.1, Abschnitt 0 dieses Papiers). Anhang H 10 geht den Abschnitten 4
bis 9 dort vor.

| Nr. | Marcos Antwort | Regeln (Anhang H 10.2) | Eingearbeitet in |
|---|---|---|---|
| E-8 | «Sofort verknüpfen», auch bei Einladungen | N-90 (N-2a zurückgezogen, kein Zustand «Verknüpfung vorgeschlagen», keine Aktion und keine Operation zum Bestätigen), N-91 (Passwort an den verknüpften Benutzer, nicht bei N-6), N-92 (bekannte Grenze); Redaktion: N-93 (Hinweis «Neu verknüpft» sieben Tage, Protokoll), N-94 (gleiche Meldung in beiden Fällen), N-95 (E-Mail-Änderung) | `03-architektur.md` 7.4, 14, 15.4, 18.1 bis 18.4; `07-administration.md` Kurz gesagt, 2 (M-20), 4.7, 6 (M-24), 7 (M-25), 8 (M-26), 12, 13, 14; `08-datenmodell-api.md` 2.2 (`linked_by`, `linked_at`; ohne `suggested` und `suggested_seerr_user_id`), 4.5 (ohne `confirmSeerrAccountLink`; `provisionSeerrAccounts` antwortet `{link, create}`), 4.6, 4.7; `09-umsetzungsplan.md` 3 (1.5a, 1.5b, 1.8), 10; `10-abnahmekriterien.md` AK-141, AK-142, AK-216, AK-217 geändert, AK-213, AK-214, AK-215, AK-218 entfallen, neu AK-220 bis AK-225, M-20, M-24, M-25, M-35; F-23, F-24; Frage 33; ADR 0037 Entscheid 4 und 15, «Verworfen», «Folgen» |
| E-9 | «Seerr» | N-96 | alle Papiere (Ersetzung durch die Redaktion); «Jellyseerr» nur bei Version 2.x und in den Anhängen A bis G; F-32 |
| E-10 | «Erfüllte Wünsche» | N-97 | `03-architektur.md` 2.1; `07-administration.md` 10, 14; `08-datenmodell-api.md` 2.11, 4.6; `09-umsetzungsplan.md` 3.4; AK-78, AK-195; F-20, F-34; ADR 0038 |
| E-11 | «Ja, nach Messung» | N-98 | `03-architektur.md` 2.1, 10.1; `08-datenmodell-api.md` 4.6; `09-umsetzungsplan.md` Kurz gesagt, 6a, 8, 9, 10; AK-104, AK-212, neu AK-226, AK-227; F-22, F-33; ADR 0037 Entscheid 20, «Folgen» |

**Fragen danach (Anhang H 10.3).** Es gibt F-1 bis F-34. Entschieden sind F-8 (E-5), F-32 (E-9), F-33 (E-11) und
F-34 (E-10); F-20 ist durch F-34 ersetzt. F-24 ist im Teil «Verknüpfen» durch E-8 entschieden; der Rest (Passwort
auch bei nachträglich eröffneten und verknüpften Konten, K-11; Hinweis an die Administratoren, N-93) bleibt
Empfehlung. Offen waren damit 29 Fragen (F-1 bis F-34 ohne F-8, F-20, F-32, F-33, F-34) und T-1, T-2; beantwortet
hat Marco sie mit dem Bauauftrag (Abschnitt 7.8).

**Wer hat entschieden?** E-8 bis E-11 sind Marcos Antworten. N-91 folgt aus E-1 und E-8; die Ausnahme N-6 darin,
N-93, N-94, N-95 und der Zuschnitt von Schritt 6a (was als Gewinn zählt) sind Empfehlungen der Redaktion.

### 7.8 Bauauftrag und Antworten vom 17. November 2026 (Anhang H 11)

Marco hat den Bau bestellt und alle 29 offenen Fragen sowie T-1, T-2 beantwortet (Anhang H 11.2), dazu B-1 bis
B-4 zum Ablauf (Anhang H 11.1). Abschnitt 11 dort geht allen früheren vor.

| Frage | Marcos Antwort | Regel (Anhang H 11.3) | Eingearbeitet in |
|---|---|---|---|
| F-27 | keine Ausnahme je Profil | N-100 | `02` 3, 4.10, 6; `03` 2.1, 7.3, 18.2, 19.1; `04` Kurz gesagt, 1, 4, 5, 6, 6.1, 6.2, 8, 9, 10, 11, 13.6, 17.7, 18 (entfällt), 19; `07` 1, 6, 14; `08` 1, 1.2, 2.2, 3.2, 4.2, 4.5; `09` 1.8, 11; `10` AK-14, AK-108 entfallen, AK-30, AK-35 geändert, neu AK-228; ADR 0037 |
| F-4 | Sammelbenutzer «slouchcat-gaeste» ab Werk | N-101, N-102 | `02` 3, 4.9, 4.10; `03` 2.1, 18.4, 18.5 (neu), 19.1; `04` 6 (Regel 8), 10, 12.3, 17.1; `07` 2, 4, 7, 14; `08` 2.12, 4.2; `09` 1.3, 1.5a, 1.8; `10` AK-109 geändert, neu AK-229, AK-231; ADR 0037 |
| F-24 | Passwort nur für eingeladene und von Slouchcat eröffnete Benutzer | N-103 | `02` 4.9; `03` 18.1, 18.2; `04` 7, 11, 12.1, 12.2, 13, 13.2, 13.4; `07` 4, 6, 14; `08` 2.2, 4.5; `09` 1.5a, 1.5b; `10` AK-12, AK-217 geändert, neu AK-230; ADR 0037 |
| F-22 | ein Release am Schluss (B-1) | N-104 | README; `08` Kurz gesagt, 3.1, 4.6; `09` Kurz gesagt, 1, 1.1, 1.10, 2.11, 3.6, 4.8, 5.7, 7a (neu), 9, 10; `10` Kurz gesagt, neu AK-232; ADR 0037 |
| übrige | nach Empfehlung | – | Antwort als erster Punkt jeder Frage in Abschnitt 1; `04` 19, `06` 13, `07` 14 |

Das Mockup bleibt unverändert; wo es den alten Stand zeigt, steht in `mockup-abdeckung.md`, Abschnitt 11.

