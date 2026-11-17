# Konzept 25 · Wünsche über Seerr

- **Status: Entwurf zur Freigabe, Fassung 3 (nach Marcos drei Runden), 17. November 2026. Nichts ist
  gebaut.** Fassung 2 arbeitete die 40 Befunde der ersten Kritik (Anhang G) und die Redaktionsentscheide vom
  17. November 2026 ein (`11-entscheide-und-adr.md`, Abschnitt 7). Fassung 3 umfasst drei Runden Marcos vom
  17. November 2026 mit seinen Antworten E-1 bis E-11 ([Anhang H](anhang/H-nachtrag-17-november.md)). Die
  erste: Seerr-Benutzer je Konto mit gleichem Login, Profil-Tag in Radarr und Sonarr, Limit je Konto,
  Bereich «Wünsche» wie die Startseite von Seerr, TVDB durchgängig. Die zweite, nach der Durchsicht des
  Mockups: keine fehlenden Titel im Spotlight, Webhooks von Radarr und Sonarr wie heute, Download-Fortschritt
  bei Wünschen. Die dritte (E-8 bis E-11): bestehende Seerr-Benutzer mit gleicher E-Mail sofort verknüpfen,
  auch bei Einladungen; der Dienst heisst «Seerr»; Startreihe «Erfüllte Wünsche»; schneller Import nach einer
  Messung. Was darüber hinausgeht, ist Empfehlung der Redaktion. Eine Kritik an Fassung 3 (Urteil REVISE,
  1 Blocker, 4 Major, 18 Minor) ist eingearbeitet; die Entscheide dazu hat die Redaktion getroffen, nicht Marco
  (Anhang H, Abschnitt 9, K-1 bis K-23). Den Blocker K-1 hat Marco mit E-8 anders entschieden (Anhang H,
  Abschnitt 10). Marco prüft Konzept und Mockups, beantwortet die offenen Fragen und gibt frei
  (`11-entscheide-und-adr.md`). Es gibt die Fragen F-1 bis F-34; entschieden sind F-8 (E-5), F-32 (E-9),
  F-33 (E-11) und F-34 (E-10), F-20 ist durch F-34 ersetzt, F-24 ist für die Verknüpfung durch E-8
  entschieden. Offen sind damit 29 Fragen, dazu die Teilfragen T-1 und T-2. Erst danach folgt ein eigener
  Auftrag für den Bau.
- **Auslöser:** Marcos Auftrag vom 16. November 2026: fehlende Filme und Serien direkt in Slouchcat anfordern,
  vor allem aus dem Spotlight. Seit seiner zweiten Rückmeldung (E-5) zeigt das Spotlight keine fehlenden Titel
  mehr; angefordert wird im Bereich «Wünsche», in der Suche und auf Serienseiten. Seerr bleibt bestehen und
  verwaltet die Wünsche. Seerr hiess bis Version 2.x «Jellyseerr». Slouchcat zeigt den Zustand und
  benachrichtigt über die eigenen Kanäle, sobald der Titel abspielbar ist.
- **Verankert gegen:** `main` `ba6efe31` (Release 1.40.1), Vertrag 1.40.0, letzte Migration 00065, letzte
  ADR 0036, letzte Frage im Entscheidungsprotokoll 32.
- **Nummern:** Dieses Papier ist Konzept 25 und reserviert Frage 33, ADR 0037 («Wünsche über Seerr») und
  ADR 0038 («Titel ausserhalb der Bibliothek»). Migrations- und Versionsnummern vergibt der Bau.
- **Dieses Konzept ändert keine Zeile Code und keine Regel.** Wünschen ist heute ein Nicht-Ziel (Spec,
  Frage 10, `CLAUDE.md` Regel 11), den Beschaffungsstand zeigt Slouchcat heute nicht (Spec, Non-Goals), und
  Slouchcat spricht heute Radarr und Sonarr nie an (ADR 0010). S-4 («Titel ohne Bibliothekstreffer nie auf der
  Startseite») bleibt unverändert. Welche Stellen sich mit einer Freigabe ändern, steht in
  `11-entscheide-und-adr.md` Abschnitt 2 und in Anhang H 5 und 8.5.

## In einem Absatz

Ein Profil, das anfordern kann, findet fehlende Titel im neuen Bereich «Wünsche» mit den Reitern «Entdecken»,
«Filme», «Serien», «Suche» und «Meine Wünsche», in der Suche, auf einer Vorschauseite und als fehlende Staffeln
auf Serienseiten, und fordert sie mit einem Dialog an. Wer nicht anfordern kann, sieht sie nicht. Startseite und
Spotlight zeigen wie heute nur Titel der Bibliothek; fehlende Einträge der Trakt-Liste sieht nur der
Administrator in der Verwaltung. Jedes Konto hat in Seerr einen eigenen
Benutzer mit demselben Login: Slouchcat eröffnet ihn, sobald jemand eine Einladung annimmt. Gibt es in Seerr
schon einen Benutzer mit gleicher E-Mail, verknüpft Slouchcat ihn sofort. Je Konto gilt ein Limit für Filme und
Staffeln, rollierend, für alle Profile gemeinsam; Slouchcat stellt es ein, Seerr zählt. Slouchcat reicht den
Wunsch im Namen dieses Benutzers weiter (Header `X-API-User`), speichert selbst, welches Konto und welches Profil gewünscht hat, und hängt einen
Profil-Tag wie `sc-noah` an den Film in Radarr oder die Serie in Sonarr. Es gleicht den Zustand alle fünf Minuten
ab. Solange ein Wunsch in Beschaffung ist, zeigt Slouchcat beim Wunsch den Download-Fortschritt aus Radarr oder
Sonarr, etwa «Wird heruntergeladen · 45 % · noch 12 min». «Verfügbar» meldet es erst, wenn die eigene Bibliothek
den Titel für das Profil abspielen kann. Slouchcat beschafft nichts selbst; in Radarr und Sonarr setzt es nur
den Tag und liest den Fortschritt. Die Webhooks von Radarr und Sonarr bleiben, wie sie heute sind. Neu auf der
Startseite ist nur die Reihe «Erfüllte Wünsche» mit Titeln, die in den letzten 14 Tagen eingetroffen sind.

## Die Papiere

| Datei | Inhalt |
|---|---|
| [01-bestandsanalyse.md](01-bestandsanalyse.md) | Was es heute gibt, was die Seerr-API wirklich kann, welche Einschränkungen das Konzept bestimmen |
| [02-funktionskonzept.md](02-funktionskonzept.md) | Begriffe, Zustände eines Wunsches, Abläufe (auch Einladung und Limit), Seiten der Zuschauer-App mit dem Bereich «Wünsche», Serien und Staffeln |
| [03-architektur.md](03-architektur.md) | Pakete, Anbindung, Ausgangsfach, Abgleich, Verfügbarkeit, Jobs, Fehler, Sicherheit, Diagramme |
| [04-benutzer-profilmapping.md](04-benutzer-profilmapping.md) | Seerr-Benutzer je Konto (eröffnen, verknüpfen, Passwort, Rechte), Profil-Tag, Limit, Ausnahme je Profil, Varianten A, B, C, Sonderfälle, Wunschrecht |
| [05-spotlight-integration.md](05-spotlight-integration.md) | Spotlight: was wie heute bleibt (nur Titel der Bibliothek), Abgleich über TVDB, TMDB und IMDb, fehlende Einträge in der Verwaltung, Verworfenes |
| [06-benachrichtigungen.md](06-benachrichtigungen.md) | Ereignisse, Empfänger, Datenschutz, Doppelschutz, Standardwerte; was bewusst keine Meldung ist (Limit, Download-Fortschritt) |
| [07-administration.md](07-administration.md) | Jede betroffene Verwaltungsseite: heutiger Inhalt und was dazukommt, Status und Diagnose |
| [08-datenmodell-api.md](08-datenmodell-api.md) | Tabellen, Migrationen, Operationen, verwendete Seerr-Endpunkte |
| [09-umsetzungsplan.md](09-umsetzungsplan.md) | Stufen 0 bis 5, Schritte, Tests, Risiken |
| [10-abnahmekriterien.md](10-abnahmekriterien.md) | Prüfbare Kriterien je Stufe, Abnahme der Mockups |
| [11-entscheide-und-adr.md](11-entscheide-und-adr.md) | Offene Entscheide F-1 bis F-34 und T-1, T-2 mit Empfehlung, geänderte frühere Entscheide, Entwürfe Frage 33, ADR 0037, ADR 0038, Regel 11, Einarbeitung der Kritik und Abweichungen von Anhang F |
| [anhang/](anhang/) | Grundlagen: [A](anhang/A-spotlight-startseite.md) Bestand Spotlight und Startseite; [B](anhang/B-konten-benachrichtigungen.md) Bestand Konten, Profile und Benachrichtigungen; [C](anhang/C-integrationen-jobs-regeln.md) Bestand Integrationen, Jobs und harte Regeln; [D](anhang/D-jellyseerr-api.md) was die Seerr-API wirklich kann; [E](anhang/E-design-sheet.md) Gestaltungsvorgaben für das Mockup; [F](anhang/F-entscheid-blatt.md) Entscheid-Blatt als Zwischenstand (wo die Papiere abweichen, gilt das Papier); [G](anhang/G-kritik-fassung-1.md) erste Kritik mit 40 Befunden; [H](anhang/H-nachtrag-17-november.md) Marcos drei Runden vom 17. November 2026 mit seinen Antworten E-1 bis E-11, neuen Fakten (auch lesend gegen seine Radarr- und Sonarr-Instanzen geprüft) und den Regeln N-1 bis N-98. In den Papieren heissen sie kurz «Anhang A» bis «Anhang H» |
| [mockup-abdeckung.md](mockup-abdeckung.md) | Abdeckungsmatrix: Seite, heutige Funktion, Änderung, Rolle, Einstellungen, Operationen, Mockup, Zustände, Umsetzung |

Lesereihenfolge für Marco: dieses Blatt, dann das Mockup, dann `02`, `05`, `04`, `06`, `07` und zum Schluss
`11` mit den Fragen. Was Fassung 3 aus deinen drei Runden macht, steht kurz in Anhang H (Abschnitte 4, 8 und
10). `01`, `03`, `08`,
`09` und `10` sind für den Bau.

## Das Mockup

Klickbar, ohne Server, ohne Seerr, ohne echte Anfrage. Es liegt getrennt vom Produkt in
[`../mockups/25-medienwuensche/`](../mockups/25-medienwuensche/) und teilt keinen Code mit der App.

- **Einstieg:** `docs/planning/mockups/25-medienwuensche/index.html` im Browser öffnen. Die erste Seite
  listet alle Mockup-Seiten (M-01 bis M-35). Neu in Fassung 3 sind M-31 bis M-34 (Bereich «Wünsche»:
  Entdecken, Filme, Serien, Suche) und M-35 (Einladung annehmen); M-24 heisst jetzt «Seerr-Benutzer».
  M-35 verknüpft einen bestehenden Seerr-Benutzer sofort und zeigt in beiden Fällen dieselbe Meldung; die
  Übersicht der Verwaltung (M-20) zeigt den Hinweis «Neu verknüpft». Überall steht «Seerr».
  Startseite (M-01) und Spotlight-Liste (M-02) zeigen wie heute nur Titel der Bibliothek; neu auf der
  Startseite ist nur die Reihe «Erfüllte Wünsche». «Meine Wünsche» (M-08) und die Wunschkarte der Vorschauseite
  (M-03, M-04) zeigen den Download-Fortschritt.
- **Steuerung:** Die gelbe Schaltfläche «Mockup» unten rechts ersetzt Seerr, Radarr, Sonarr und die Zeit:
  Profil wechseln, Seerr ausfallen lassen, Wünsche vorspulen (übermitteln, freigeben, Datei trifft ein,
  scheitern), einen Download weiterlaufen oder hängen lassen («Download weiter», «Download hängt»), das Limit
  ausschöpfen, Radarr oder Sonarr ausfallen lassen, umstellen, woher Seerr die
  Staffeln liest (TMDB oder TVDB), eine Einladung annehmen und eine Anmeldung spielen, Ansicht Desktop, Tablet
  oder Handy, alles zurücksetzen.
- **Bilder:** Poster und Szenenbilder stammen von TMDB und liegen nur lokal im Ordner `img/`. Das Repo ist
  öffentlich, darum stehen sie nicht im Repo (`.gitignore`). Ohne Bilder zeigt das Mockup Platzhalter mit
  dem Titel.
- **Zustand:** Was du anklickst, bleibt im Browser gespeichert (`localStorage`), bis du zurücksetzt.

## Die wichtigsten Entscheide in Kürze

«Marco (Nachricht)» und «Marco (E-n)» heissen: von Marco am 17. November 2026 so verlangt oder entschieden, in
einer seiner Nachrichten oder in den Antworten E-1 bis E-11 (Anhang H, Abschnitte 1, 2, 8.1, 8.2 und 10.1). Alles
andere ist Empfehlung und offen bis zur Freigabe.

| Thema | Empfehlung oder Entscheid | Frage |
|---|---|---|
| Wer verwaltet Wünsche | Seerr; Slouchcat legt an, liest, zeigt und meldet | – |
| Im Namen von wem | Header `X-API-User` mit dem Seerr-Benutzer: seine Rechte, sein Limit, seine Auto-Freigabe gelten | F-3 |
| Seerr-Benutzer | je Konto einer, beim Annehmen der Einladung eröffnet, gleiche E-Mail und gleiches Passwort; bestehende Benutzer mit gleicher E-Mail werden sofort verknüpft, auch wenn das Konto die Adresse beim Annehmen einer Einladung selbst eingetippt hat: **Marco (E-1, E-4, E-8)**. Bekannte Grenze, von Marco so gewählt: Wer eine Einladung hat und eine fremde Adresse eintippt, bekommt den Seerr-Benutzer dieser Person samt ihren Anfragen (**Marco, E-8**). Administratoren von Seerr und Benutzer mit Anmeldung über Jellyfin, Emby oder Plex bekommen kein Passwort; Hinweis «Neu verknüpft» sieben Tage in der Übersicht; dieselbe Meldung an das Konto in beiden Fällen; eigene E-Mail geändert wie Einladung; für welche Konten, Passwort bei nachträglich verknüpften Benutzern, was beim Löschen eines Kontos geschieht: Empfehlung | F-23, F-24 (Verknüpfung durch E-8 entschieden), F-31 |
| Profile | kein Login in Seerr, Profil am Wunsch und als Tag: **Marco (Nachricht)**. F-2 ist damit in der Sache entschieden; die Ausnahme «Profil sendet als anderer Seerr-Benutzer» bleibt zugeklappt (Empfehlung) | F-2, F-27 |
| Profil-Tag | Tag am Film in Radarr oder an der Serie in Sonarr; Slouchcat spricht dafür Radarr und Sonarr direkt an: **Marco (E-2)**. Name `sc-<profilname>`, nur Tag anlegen und anhängen, Zugang aus Seerr gelesen: Empfehlung | F-25, F-30 |
| Limit | je Konto für Filme und Staffeln, rollierend, für alle Profile gemeinsam; Slouchcat stellt ein, Seerr zählt, auch direkte Wünsche: **Marco (Nachricht, E-3)**. Vorgabe 10 Filme und 5 Staffeln in 7 Tagen (Marcos Beispiel), Administratoren ohne Limit: Empfehlung | F-26 |
| Bereich «Wünsche» | Seiten wie die Startseite von Seerr: Entdecken, Filme, Serien, Suche: **Marco (Nachricht)**. «Meine Wünsche» als fünfter Reiter, Auswahl der Reihen, Daten über Seerr: Empfehlung | F-28 |
| TVDB | Serien mit TVDB unterstützen, «konsistent»: **Marco (Nachricht)**. Abgleich bei Serien TVDB zuerst, Staffeln im Dialog aus Seerr, Seerr auf TVDB-Staffeln umstellen: Empfehlung | F-29 |
| Name in der Oberfläche | «Seerr»; «Jellyseerr» nur, wo Version 2.x gemeint ist: **Marco (E-9)** | F-32 (durch E-9 entschieden) |
| Wunschrecht | Schalter je Konto (Administrator) und je Profil (Hauptprofil) | F-5 |
| Jugendschutz | TMDB-Freigabe in der Länderreihenfolge nach ADR 0034 (erstes Land mit bekanntem Wert) gegen die Altersstufe, unbekannt zählt als 18, Entscheid im Server | – |
| Wer fehlende Titel sieht | nur wer sie anfordern kann (Recht, Seerr-Benutzer, Art, Alter); Ausnahme eigene Wünsche; gilt für den Bereich «Wünsche», die Suche, die Vorschauseite und fehlende Staffeln | F-9 |
| Verfügbar | wenn Slouchcat den Titel für das Profil abspielen kann | F-13 |
| Schon angefordert | Mitwunsch statt Doppelbestellung | F-7 |
| Spotlight | Startseite und Spotlight-Liste nur mit Titeln der Bibliothek, wie heute; fehlende Einträge der Trakt-Liste nirgends in der Zuschauer-App, nur unter Verwaltung › Spotlight mit Poster, Zustand und «Anfordern»: **Marco (Nachricht, E-5)**. Vorhandene Serien auch über TVDB- und IMDb-ID erkennen, TVDB zuerst; Band «Fehlende Einträge» ohne Schalter: Empfehlung | F-8 (durch E-5 entschieden) |
| Radarr und Sonarr | Webhooks bleiben wie heute, von Hand eingerichtet; Slouchcat trägt keinen Webhook ein. Die API nutzt Slouchcat nur für den Profil-Tag und das Lesen des Download-Fortschritts, angezeigt bei Wünschen: **Marco (E-6, E-7)**. Gelesen werden Warteschlange und Dateistand der gewünschten Titel; der Import kommt weiter über den Webhook (Empfehlung). Vier Anzeigen («Wird gesucht», «Wird heruntergeladen · 45 % · noch 12 min», «Wird eingelesen», «Download verzögert»), Abruf jede Minute, Schalter «Download-Fortschritt zeigen»: Empfehlung | – |
| Schneller Import | Pfad und Kennung aus der Webhook-Meldung nutzen, nur diese Datei mit Vorrang einlesen und über die Kennung zuordnen; eigener Schritt im Bauplan, erst nach einer Messung; zeigt sie keinen Gewinn, entfällt der Schritt; die Webhooks bleiben, wie sie sind: **Marco (E-11)** | F-33 (durch E-11 entschieden) |
| Startreihe | «Erfüllte Wünsche»: nur Wünsche, die in den letzten 14 Tagen wahr wurden; offene Wünsche stehen unter «Wünsche»: **Marco (E-10)**. Ihre Zahl an «Wünsche» in der Seitenleiste: Empfehlung | F-34 (ersetzt F-20, durch E-10 entschieden) |
| Verwaltung | Kachel Seerr nach «Webhooks von Radarr und Sonarr»; Reiter «Wünsche» unter Medien nach «Pflege»; Reihe «Erfüllte Wünsche» nach «Weiterschauen»; «Wünsche aktiv» ab Werk aus | F-19 |
| Benachrichtigung | Eingang des Profils und Kanäle des Kontos, bei PIN-Profilen ohne Titel | F-14, F-15 |
| Bau | Stufen 0 bis 4 wie in Fassung 2, neu Stufe 5 «Wünsche-Seiten»; Eröffnung, Passwort und Limit in Stufe 1; Stufe 3 «Spotlight» nur noch Abgleich über TVDB und IMDb und fehlende Einträge in der Verwaltung; Profil-Tag und Download-Fortschritt in Stufe 4 (Anhang H 4.7, 8.7); je Stufe ein Release; Stufe 1 im Vertrag nicht brechend, Stufen 2 bis 4 brechend (`08-datenmodell-api.md`, Abschnitt 4.6) | F-22 |
