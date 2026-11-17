# Konzept 25 · Funktionskonzept: was Zuschauer und Administrator erleben

> **Kurz gesagt.** Ein Profil, das anfordern kann, findet Filme und Serien, die noch fehlen, im Bereich «Wünsche»
> mit den Reitern «Entdecken», «Filme», «Serien», «Suche» und «Meine Wünsche», in der Suche, auf einer
> Vorschauseite und als fehlende Staffeln auf Serienseiten. Startseite und Spotlight zeigen wie heute nur Titel
> der Bibliothek (Marco, E-5). Ein Druck auf «Film anfordern» öffnet einen Dialog; er zeigt auch, wie viel vom
> Limit des Kontos noch frei ist. Slouchcat reicht den Wunsch im Namen des Seerr-Benutzers des Kontos
> weiter und zeigt danach seinen Zustand, während der Beschaffung mit dem Download-Fortschritt aus Radarr oder
> Sonarr (Marco, E-7). Den Seerr-Benutzer eröffnet Slouchcat selbst, sobald jemand eine Einladung annimmt,
> mit demselben Login. Ob ein Wunsch eine Freigabe braucht, entscheidet allein Seerr. Sobald Slouchcat den
> Titel für das Profil abspielen kann, kommt eine Benachrichtigung. Slouchcat beschafft nichts selbst; in Radarr
> und Sonarr setzt es nur den Profil-Tag und liest den Download-Fortschritt.

Grundlage: [Anhang F](anhang/F-entscheid-blatt.md) (Begriffe, Zustände, Seiten). Was heute schon da ist, steht in
`01-bestandsanalyse.md`; das Spotlight im Einzelnen in `05-spotlight-integration.md`. Die Verwaltung steht in
`07-administration.md`, Seerr-Benutzer, Profil-Tag, Limit und Rechte in `04-benutzer-profilmapping.md`, die Benachrichtigungen in
`06-benachrichtigungen.md`, Datenmodell und API in `08-datenmodell-api.md`. Was Fassung 3 dazubringt, steht in
[Anhang H](anhang/H-nachtrag-17-november.md): Marcos drei Runden vom 17. November 2026, seine Antworten
E-1 bis E-11 und die Regeln N-1 bis N-98 (Abschnitt 10 geht allen früheren vor, Abschnitt 8 den Abschnitten 4
bis 7). Als Marcos Entscheid gilt
nur, was dort als seine Nachricht oder Antwort steht;
alles andere ist Empfehlung. Abschnitte mit **Vorschlag** gehen über Anhang F und Anhang H hinaus und brauchen
Marcos Ja.

## 1. Begriffe

| Begriff | Bedeutung |
|---|---|
| **Wünsche** | die Funktion und ihre Seiten |
| **Bereich «Wünsche»** | der Eintrag «Wünsche» der Seitenleiste mit fünf Reitern: «Entdecken», «Filme», «Serien», «Suche» (unter `/discover`) und «Meine Wünsche» (`/wishes`) |
| **Wunsch** | ein Eintrag: ein Film oder eine Serie mit gewählten Staffeln, angefordert von einem Profil |
| **anfordern** | die Tätigkeit auf Knöpfen: «Film anfordern», «Serie anfordern», «Staffeln anfordern» |
| **fehlender Titel** | Film oder Serie, die nicht in der Bibliothek steht; in der Oberfläche «Noch nicht in der Bibliothek» |
| **Vorschauseite** | Detailseite eines fehlenden Titels (`/titles/{movie\|show}/{tmdbId}`) |
| **Mitwunsch** | ein weiteres Profil hängt sich an einen offenen Wunsch und wird ebenfalls benachrichtigt; kostet kein Limit |
| **Seerr** | der Dienst, der Wünsche verwaltet und an Radarr/Sonarr weitergibt (Seerr 3.x, Paket `seerr`); bis Version 2.x hiess er «Jellyseerr» |
| **Seerr-Benutzer** | der Benutzer in Seerr, in dessen Namen Slouchcat einen Wunsch anlegt; im Normalfall der Benutzer des Kontos, mit gleicher E-Mail und gleichem Passwort |
| **eröffnen** | Slouchcat sorgt dafür, dass ein Konto einen Seerr-Benutzer hat: von selbst beim Annehmen der Einladung, sonst mit «In Seerr eröffnen» oder «Alle eröffnen» des Administrators |
| **verknüpfen** | Slouchcat nimmt einen Seerr-Benutzer, den es mit derselben E-Mail schon gibt, statt einen zweiten anzulegen; das geschieht sofort, auch wenn das Konto die Adresse selbst eingetippt hat (Einladung, eigene E-Mail geändert; Marco, E-8) |
| **Limit** | wie viele Filme und Staffeln ein Konto in einem rollierenden Zeitraum anfordern kann, für alle Profile gemeinsam; Seerr nennt es «Quota» |
| **Profil-Tag** | Kennzeichen wie `sc-noah` am Film in Radarr oder an der Serie in Sonarr; zeigt dort, welches Profil gewünscht hat |
| **Download-Fortschritt** | was Radarr oder Sonarr zu einem Wunsch in Beschaffung melden, etwa «Wird heruntergeladen · 45 % · noch 12 min»; nur Anzeige beim Wunsch (Abschnitt 5.3) |
| **Erfüllte Wünsche** | Startreihe mit den Wünschen des Profils, die in den letzten 14 Tagen wahr wurden (Marco, E-10) |
| **Wunschrecht** | ob ein Profil anfordern darf; Schalter je Konto (Administrator) und je Profil (Hauptprofil) |

Die Begriffe sind offen als F-1. In der Oberfläche heisst der Dienst «Seerr» (Marco, E-9).

## 2. Zustände eines Wunsches

Slouchcat bildet die Zustände selbst, aus Seerr und aus der eigenen Bibliothek.

| Schlüssel | Anzeige | Bedeutung | Was der Zuschauer tun kann |
|---|---|---|---|
| `queued` | «Wird übermittelt» | lokal gespeichert, noch nicht bei Seerr | warten; zurückziehen |
| `pending` | «Freigabe ausstehend» | bei Seerr, wartet auf Freigabe | warten; zurückziehen |
| `processing` | «In Beschaffung» | freigegeben, Radarr/Sonarr beschaffen; dazu der Download-Fortschritt (Abschnitt 5.3); «Wird eingelesen», wenn Radarr, Sonarr oder Seerr den Titel schon als da melden, Slouchcat ihn aber noch nicht hat | warten |
| `partial` | «Teilweise verfügbar» | Serie: ein Teil der gewünschten Staffeln ist in Slouchcat vollständig abspielbar (Abschnitt 6) | vorhandene Staffeln schauen |
| `available` | «Verfügbar» | in Slouchcat für dieses Profil abspielbar | abspielen |
| `declined` | «Abgelehnt» | in Seerr abgelehnt, ohne Grund (Seerr kennt keinen) | erneut anfordern (Abschnitt 4.6) |
| `failed` | «Fehlgeschlagen» | Übergabe an Radarr/Sonarr gescheitert oder endgültig abgewiesen (Limit, Sperrliste, kein Recht) | Klartext lesen; erneut anfordern, ausser die Übergabe an Radarr/Sonarr ist gescheitert (dann wiederholt ein Administrator) |
| `withdrawn` | «Zurückgezogen» | vom Profil zurückgezogen, solange nicht freigegeben | erneut anfordern |

«Verfügbar» heisst: Slouchcat kann den Titel für **dieses** Profil abspielen, nicht der Zustand in
Seerr (F-13). Ein Titel, den Seerr «verfügbar» meldet, bleibt «In Beschaffung · wird
eingelesen», bis Slouchcat ihn zugeordnet hat.

**Geschlossene Wünsche.** Ein abgelehnter, fehlgeschlagener oder zurückgezogener Wunsch ist geschlossen. Der
Titel gilt danach wieder als fehlend und trägt wieder «Film anfordern» bzw. «Serie anfordern». Die
Vorschauseite nennt den letzten Wunsch mit Datum und Grund, etwa «Am 12. November abgelehnt». Ausnahme: Ist
in Seerr die Übergabe an Radarr oder Sonarr gescheitert (`fail_reason = arr_failed`), sperrt der
Request dort den Titel weiter. Dann fehlt «anfordern», und der Zuschauer liest «Ein Administrator kann den
Wunsch wiederholen» (`03-architektur.md`, Abschnitt 8.1).

Den vollständigen Zustandsautomaten mit allen Übergängen zeigt `03-architektur.md`, Abschnitt 8.2. Dieses
Papier wiederholt ihn nicht, damit es nur eine Fassung gibt.

## 3. Wer anfordern darf

Ein Profil darf anfordern, wenn alles zutrifft:

1. Der Administrator hat Seerr eingerichtet und eingeschaltet («Seerr verwenden») und «Wünsche
   aktiv» eingeschaltet (ab Werk aus).
2. Das Konto hat das Wunschrecht (Administrator, Formular «Zugriff»; bei Administratorkonten der Block
   «Wünsche»). Vorgabe: Besitzerkonto an, neue Konten aus.
3. Das Profil hat das Wunschrecht (Hauptprofil, Profildialog). Vorgabe: Hauptprofil an, weitere Profile aus.
4. Das Konto hat einen Seerr-Benutzer (von Slouchcat eröffnet, verknüpft oder von Hand zugeordnet), das
   Profil hat eine Ausnahme, oder der Sammelbenutzer greift. Konten ohne Seerr-Benutzer dürfen nicht
   anfordern (Vorgabe; Sammelbenutzer als Wahl, F-4). Das gilt auch, solange ein Konto noch «wird eröffnet»
   (Ablauf 4.9). Sobald der Benutzer besteht, geht das Anfordern;
   das Passwort spielt dafür keine Rolle.
5. Die Medienart ist erlaubt («Filme erlauben», «Serien erlauben») und die Stelle ist erlaubt
   («Anfordern aus der Suche», «Anfordern auf Detailseiten», Schalter «Seiten Entdecken, Filme, Serien und
   Suche»). Einen Schalter für das Spotlight gibt es nicht, weil dort nichts anzufordern ist.
6. Die Freigabe des Titels passt zur Altersstufe des Profils (Abschnitt 7).

**Eine Regel für das Sehen (F-9):** Fehlende Titel sieht ein Profil nur, wenn es sie auch anfordern kann,
also wenn alle sechs Punkte zutreffen. Sonst existieren sie für das Profil nicht: Sie fehlen in der Suche, auf
Serienseiten und im Bereich «Wünsche», und die Vorschauseite antwortet 404. Startseite und Spotlight zeigen
fehlende Titel niemandem (Marco, E-5; N-70). Die Regel gilt in Fassung 3 unverändert. Einen «Hinweis statt Knopf» gibt es nicht. Einzige Ausnahme:
Titel, zu denen das Profil selbst einen Wunsch hat (in jedem Zustand), bleiben auf der Vorschauseite und unter
«Meine Wünsche» sichtbar. Ist Seerr nur vorübergehend nicht erreichbar oder lehnt den Schlüssel ab,
bleiben fehlende Titel sichtbar; neue Wünsche warten im Ausgangsfach (Abschnitt 4.4).

Ein Konto mit Recht, aber ohne Seerr-Benutzer bemerkt der Administrator in der Verwaltung (Übersicht,
Seite «Seerr-Benutzer», Konten), nicht der Zuschauer (`07-administration.md`, Abschnitte 2, 6 und 8).

Das **Limit** ändert nichts daran, was ein Profil sieht. Ist es erreicht, bleiben fehlende Titel sichtbar; der
Dialog sperrt nur den Knopf (Abschnitt 4.10).

Der Server liefert `canRequest`, einen Grund und den Stand des Limits; die App zeigt nur an (Regel 12). Recht,
Limit und Auto-Freigabe in Seerr gelten so, als hätte der Seerr-Benutzer selbst geklickt
(`X-API-User`, F-3). Seerr-Benutzer, Limit und Vorgaben im Einzelnen: `04-benutzer-profilmapping.md`
(F-2, F-5, F-23 bis F-27).

## 4. Abläufe

Jeder Ablauf beginnt bei einem Profil, das anfordern kann (Abschnitt 3), sofern nicht anders gesagt. Mockups: Dialog M-13.
Neu in Fassung 3 sind die Abläufe 4.9 (Einladung annehmen), 4.10 (Anfordern mit Limit) und 4.11 (Stöbern im
Bereich «Wünsche»).

### 4.1 Film anfordern

1. Das Profil findet einen fehlenden Film im Bereich «Wünsche» (Entdecken, Filme, Suche; M-31, M-32, M-34), im
   Abschnitt «Noch nicht in der Bibliothek» der Suche (M-07) oder auf seiner Vorschauseite (M-03). Am Poster
   steht ein runder Plus-Knopf, auf der Vorschauseite der Hauptknopf «Film anfordern».
2. Das Profil drückt den Plus-Knopf oder «Film anfordern». Der Dialog öffnet sich: Poster, Titel, «Wird als {{Seerr-Benutzer}}
   an Seerr übermittelt», Hinweis «Wird sofort beschafft» oder «Braucht eine Freigabe», das Limit des
   Kontos («Noch 8 von 10 Filmen frei in 7 Tagen» oder «ohne Limit»), Knöpfe «Anfordern» und «Abbrechen»
   (F-10). Ist das Limit erreicht, gilt Ablauf 4.10.
3. Das Profil drückt «Anfordern». Slouchcat speichert den Wunsch (`queued`) und versucht die Übermittlung
   sofort (höchstens 3 s).
4. Die Antwort kommt am Bildschirm, nicht als Benachrichtigung: Der Dialog schliesst, eine kurze Meldung
   bestätigt.
5. Der Knopf wird zur Zustandsanzeige, etwa «Freigabe ausstehend» oder «In Beschaffung»: auf der Vorschauseite
   der Hauptknopf, der zu «Meine Wünsche» (M-08) führt, am Poster das Zustandssymbol statt des Plus-Knopfs.
6. Der Wunsch steht in «Meine Wünsche»; der Zähler am Eintrag «Wünsche» der Seitenleiste zählt ihn mit. Wird
   er erfüllt, erscheint er 14 Tage lang in der Startreihe «Erfüllte Wünsche» (Marco, E-10; ab Werk
   nach «Weiterschauen», in der Verwaltung ausblendbar: Empfehlung).

Derselbe Ablauf gilt aus jeder Seite im Bereich «Wünsche» (M-31 bis M-34), der Suche (M-07) und der
Vorschauseite (M-03). Startseite (M-01) und Spotlight-Liste (M-02) bieten nichts zum Anfordern: Sie zeigen nur
Titel der Bibliothek, wie heute (Marco, E-5; N-70). Fehlende Einträge der Trakt-Liste fordert nur der
Administrator unter Verwaltung › Spotlight an (`05-spotlight-integration.md`, Abschnitt 4). Wird der Wunsch «In
Beschaffung», setzt Slouchcat den Profil-Tag am Film in Radarr (`04-benutzer-profilmapping.md`, Abschnitt 16)
und zeigt beim Wunsch den Download-Fortschritt (Abschnitt 5.3).

### 4.2 Serie oder Staffeln anfordern

1. Das Profil öffnet eine fehlende Serie (Vorschauseite M-04, aus dem Bereich «Wünsche» oder der Suche) oder
   eine vorhandene Serie mit fehlenden Staffeln (Detailseite M-06).
2. Es drückt «Serie anfordern» bzw. «Staffeln anfordern». Der Dialog zeigt die Staffelwahl: «Alle fehlenden
   Staffeln» (vorgewählt) oder einzelne Staffeln mit Folgenzahl und Jahr. Die Staffelliste kommt aus Seerr
   (`GET /tv/{tmdbId}`): Angeboten wird genau, was Seerr bestellen kann (Empfehlung N-63). Jede gewählte
   Staffel zählt einzeln zum Limit.
3. Nicht wählbar sind: Staffel 0 (Specials), Staffeln ohne Folgen und Staffeln, die schon vorhanden sind.
   Staffeln, die ein anderes Profil schon angefordert hat, sind wählbar und tragen «Bereits angefordert» mit
   ihrem Zustand. Für sie wird das Profil Mitwünscher (Abschnitt 4.3).
4. «Anfordern» legt **einen** Wunsch mit den übrigen gewählten Staffeln an; nur diese gehen an Seerr.
   Weiter wie 4.1 ab Schritt 3.
5. Werden später weitere Staffeln angefordert, entsteht ein weiterer Wunsch nur mit diesen Staffeln
   (Seerr arbeitet so).

Alle Unterscheidungen bei Serien: Abschnitt 6.

### 4.3 Schon angefordert: Mitwunsch

1. Ein zweites Profil öffnet einen Titel, den ein anderes Profil schon angefordert hat. Es sieht den
   Zustand («Freigabe ausstehend», «In Beschaffung»), aber nicht, wer gewünscht hat.
2. Statt «Film anfordern» steht «Benachrichtige mich» (F-7).
3. Ein Druck hängt das Profil als Mitwünscher an. Slouchcat ruft Seerr dabei nicht auf (dort sind
   doppelte Wünsche gesperrt). Ein Mitwunsch kostet kein Limit (Marco, E-3) und geht auch, wenn das Limit
   erreicht ist. Sobald der Titel «In Beschaffung» ist, bekommt er auch den Profil-Tag des Mitwünschers.
4. Der Wunsch erscheint auch in «Meine Wünsche» dieses Profils. Wird der Titel verfügbar, bekommt auch der
   Mitwünscher die Benachrichtigung.
5. Bei Serien gilt der Mitwunsch je Staffel: Im Dialog wählt das Profil auch Staffeln mit «Bereits
   angefordert». Für diese wird es Mitwünscher; nur die übrigen Staffeln gehen als neuer Wunsch an Seerr.

Steht «Bereits angeforderte Titel» auf «Nur Zustand zeigen», fehlt der Knopf; der Zustand bleibt sichtbar.

### 4.4 Seerr nicht erreichbar

1. Das Profil fordert an wie in 4.1.
2. Slouchcat erreicht Seerr nicht innert 3 s. Der Wunsch bleibt `queued` («Wird übermittelt»).
3. Der Dialog sagt: «Wird übermittelt, sobald Seerr erreichbar ist.» Für das Profil ist der Wunsch
   damit angenommen.
4. Der Job `wishes.submit` versucht es mit wachsendem Abstand erneut. Ab dem zweiten Versuch prüft er vorher,
   ob der Wunsch in Seerr schon existiert; dann hängt Slouchcat sich an.
5. Nach 24 h ohne Erfolg wird der Wunsch `failed`; die Administratoren bekommen ein Fehlerereignis. Das Profil
   sieht den Zustand in «Meine Wünsche»; eine Eingangsmeldung «Fehlgeschlagen» kommt erst ab Stufe 4. Die
   Frist ruht, solange der Administrator «Seerr verwenden» ausgeschaltet hat (**Vorschlag**, T-2 in
   `11-entscheide-und-adr.md`).

Die Verwaltung zeigt «Seerr nicht erreichbar» auf der Übersicht (M-20). Sind nur Radarr oder Sonarr nicht
erreichbar, betrifft das allein den Download-Fortschritt und den Profil-Tag (Abschnitt 5.3).

**Im Bereich «Wünsche»** (Empfehlung): «Entdecken», «Filme» und «Serien» zeigen die Listen vom letzten Abruf
aus dem Zwischenspeicher, mit dem Hinweis, dass sie nicht aktuell sind. Die Wunsch-Suche braucht Seerr; ist
es nicht erreichbar, sagt sie das und verweist auf die Bibliothekssuche, deren Abschnitt «Noch nicht in der
Bibliothek» über Slouchcats eigenen TMDB-Schlüssel weiter geht (F-11). Das Limit zeigt den letzten bekannten
Stand; wartende Wünsche rechnet Slouchcat dazu (N-35). Ein Konto, das gerade eröffnet werden soll, wartet, bis
Seerr wieder erreichbar ist (Ablauf 4.9).

### 4.5 Kein Recht

| Fall | Was das Profil sieht |
|---|---|
| Profil oder Konto ohne Wunschrecht | keine fehlenden Titel in der Suche, keine fehlenden Staffeln (F-9); im Bereich «Wünsche» keine Reiter «Entdecken», «Filme», «Serien», «Suche»; kein Eintrag «Wünsche» in der Seitenleiste, ausser es hat schon Wünsche (dann nur «Meine Wünsche») |
| Profil ohne Wunschrecht öffnet eine Vorschauseite über einen Link | 404 «nicht gefunden»; Ausnahme: Das Profil hat selbst einen Wunsch zu diesem Titel, dann Seite ohne Anforderungsknopf |
| Konto ohne Seerr-Benutzer (und kein Sammelbenutzer), auch solange es «wird eröffnet» | wie ohne Wunschrecht: keine fehlenden Titel. Der Administrator sieht den Fall in der Verwaltung (Übersicht «n Konten ohne Seerr-Benutzer», Seite «Seerr-Benutzer», Chip «Kein Seerr-Benutzer» bzw. «Wird in Seerr eröffnet» auf «Konten») |
| Medienart ausgeschaltet | keine fehlenden Titel dieser Art |
| «Anfordern aus der Suche» aus | kein Abschnitt «Noch nicht in der Bibliothek» |
| «Seiten Entdecken, Filme, Serien und Suche» aus | der Bereich «Wünsche» zeigt nur «Meine Wünsche» |
| «Anfordern auf Detailseiten» aus | keine fehlenden Staffeln auf der Serienseite; die Vorschauseite bleibt erreichbar, solange mindestens eine der Stellen an ist (Abschnitt 5.1) |
| Limit des Kontos erreicht | fehlende Titel bleiben sichtbar; der Dialog sperrt «Anfordern» und nennt den Zeitpunkt, an dem der nächste Platz frei wird; Mitwünschen geht (Ablauf 4.10) |
| Seerr-Benutzer darf in Seerr nicht anfordern, oder Seerr lehnt trotz Vorprüfung wegen des Limits ab | Slouchcat nimmt an, Seerr weist ab: Wunsch `failed` mit Klartext; beim Limit «Limit erreicht: Seerr hat den Wunsch nicht angenommen.» (Grund `quota`, für Filme und Staffeln derselbe Satz) |
| Seerr kurz nicht erreichbar oder Schlüssel abgelehnt | fehlende Titel bleiben sichtbar; der Wunsch wartet als «Wird übermittelt» (Abschnitt 4.4) |

### 4.6 Ablehnung

1. Ein Administrator lehnt den Wunsch in Seerr ab (ab Stufe 4 auch in Slouchcat, F-16).
2. Spätestens beim nächsten Abgleich (alle 5 min) wird der Wunsch `declined`.
3. Anfordernde und Mitwünscher bekommen einen Eingangseintrag «Abgelehnt» (ab Stufe 4; vorher zeigt nur
   «Meine Wünsche» den Zustand). Externe Kanäle nur, wenn das Konto «Wünsche: freigegeben, abgelehnt,
   fehlgeschlagen» gewählt hat (ab Werk aus, F-14).
4. Ein Grund fehlt, weil Seerr keinen kennt.
5. Der Wunsch ist geschlossen (Abschnitt 2). Der Titel gilt wieder als fehlend: Bereich «Wünsche», Suche und
   Vorschauseite zeigen wieder «Film anfordern». Die Vorschauseite nennt den letzten Wunsch mit Datum und Grund («Am
   {{Datum}} abgelehnt»). In «Meine Wünsche» steht «Erneut anfordern». Seerr erlaubt einen neuen Wunsch
   nach einer Ablehnung; der Dialog sagt dann «Wurde am {{Datum}} abgelehnt».

### 4.7 Titel wird verfügbar und Benachrichtigung

1. Radarr oder Sonarr legt die Datei ab; Slouchcat liest sie ein (Webhook wie heute, Watcher oder Vollscan).
   Bis dahin zeigt «Meine Wünsche» den Download-Fortschritt und danach «Wird eingelesen» (Abschnitt 5.3). Wie
   lange das Einlesen dauert, hängt am Scan. Den schnelleren Weg über Kennung und Dateipfad aus der
   Webhook-Meldung baut ein eigener Schritt, erst nach einer Messung; zeigt sie keinen Gewinn, entfällt er
   (Marco, E-11; Anhang H, N-98). Die Webhooks bleiben, wie sie sind.
2. Nach dem Metadaten-Match stehen die IDs fest. Slouchcat prüft offene Wünsche mit dieser TMDB-, TVDB- oder
   IMDb-ID. Dazu läuft stündlich `wishes.reconcile` als Netz.
3. Für jedes anfordernde Profil und jeden Mitwünscher prüft der Server, ob das Profil den Titel sehen darf.
4. Erst dann wird der Wunsch für dieses Profil `available` und es bekommt den Eingangseintrag «Verfügbar:
   {{Titel}}» mit Link auf die Detailseite. Ist ein Profil (noch) nicht berechtigt, etwa weil die Freigabe
   fehlt, wartet die Meldung für dieses Profil.
5. Kanäle des Kontos: E-Mail, Web-Push, Telegram mit `[Profilname]` im Betreff, wenn «Wünsche: verfügbar»
   gewählt ist (ab Werk an, F-14). Bei Profilen mit PIN nennt die Nachricht den Titel nicht (F-15).
6. Bei Serien gilt eine Staffel erst als verfügbar, wenn sie vollständig ist: Slouchcat kann ihre Folgen
   abspielen, und die Zahl der abspielbaren Folgen erreicht die Folgenzahl laut TMDB (bei laufenden Staffeln:
   alle bisher ausgestrahlten Folgen). Ab Stufe 4 meldet jede so verfügbare Staffel einzeln («Staffel 2 ist
   verfügbar»; bei einer laufenden Staffel «Die bisher erschienenen Folgen sind da»; weitere Folgen meldet die
   bestehende Benachrichtigung der Merkliste, wenn die Serie dort steht). Der Wunsch steht auf `partial`, bis alle gewünschten Staffeln
   da sind. Regel im Einzelnen: `06-benachrichtigungen.md`, Abschnitt 3.3.
7. Der Titel ist jetzt ein Titel der Bibliothek. Steht er in der Trakt-Liste, erscheint er im Spotlight wie jeder
   vorhandene Titel; in der Verwaltung › Spotlight zählt er nicht mehr zu den fehlenden Einträgen
   (`05-spotlight-integration.md`, Abschnitt 2). Für das anfordernde Profil und die Mitwünscher steht er 14 Tage
   in der Startreihe «Erfüllte Wünsche» (Marco, E-10; N-74).

Einzelheiten zu Arten, Ereignissen und Doppelschutz: `06-benachrichtigungen.md`.

### 4.8 Wunsch zurückziehen

1. In «Meine Wünsche» hat ein Wunsch im Zustand `queued` oder `pending` den Knopf «Zurückziehen» (F-17;
   Schalter «Zurückziehen erlauben»).
2. Eine Rückfrage bestätigt: «Wunsch zurückziehen?».
3. Slouchcat setzt den Wunsch sofort auf `withdrawn` und merkt sich «in Seerr noch zu löschen». Der
   Wunsch verschwindet aus den offenen Wünschen; der Verlauf bleibt.
4. Ein Job prüft danach in Seerr, ob dort ein Request dieses Benutzers besteht, auch wenn die Übermittlung
   scheinbar nicht geklappt hat. Solange er auf Freigabe wartet, löscht ihn Slouchcat. So bleibt keine
   Bestellung in Seerr zurück, von der Slouchcat nichts weiss. Ein gelöschter Request zählt nicht mehr zum
   Limit (Anhang H 3.2).
5. War der Request inzwischen schon freigegeben oder gehört er jemand anderem, bleibt er in Seerr stehen.
   Der Wunsch wird wieder geöffnet («In Beschaffung»), und das Profil liest «War schon freigegeben».
6. Ab `processing` geht Zurückziehen nicht mehr; der Knopf fehlt.
7. **Vorschlag für Mitwünsche:** Ein Mitwünscher zieht nur sich selbst zurück, ohne Aufruf an Seerr.
   Zieht der erste Anforderer zurück, während Mitwünscher da sind, bleibt der Wunsch bestehen; der älteste
   Mitwünscher wird Anforderer. Erst wenn niemand mehr dranhängt, zieht Slouchcat in Seerr zurück.

Technik und Fehlerfälle: `03-architektur.md`, Abschnitt 6.6.

### 4.9 Einladung annehmen: das Konto entsteht auch in Seerr

Marco hat am 17. November 2026 verlangt, dass ein eingeladener Benutzer «auch in Seerr eröffnet» wird, «mit den
gleichen Login Daten» (Nachricht), und dass er sich dort mit E-Mail und Passwort von Slouchcat anmelden kann
(E-1), und dass ein bestehender Seerr-Benutzer mit gleicher E-Mail sofort verknüpft wird, auch bei Einladungen
(E-8). Der Ablauf im Einzelnen ist Empfehlung (N-1 bis N-5, N-90 bis N-95):

1. Die eingeladene Person öffnet den Link (Seite M-35, heute `/invite/$token`). Neu steht dort der Hinweis «Dein
   Login gilt auch für Seerr».
2. Sie füllt die Seite aus wie heute (unter anderem E-Mail und Passwort) und schickt ab.
3. Slouchcat legt Konto und Hauptprofil an wie heute und eröffnet im selben Zug den Seerr-Benutzer: gleiche
   E-Mail, gleiches Passwort, Benutzername aus dem Teil der E-Mail vor dem @.
4. **Gibt es in Seerr schon einen Benutzer mit dieser E-Mail**, verknüpft Slouchcat ihn sofort (Marco, E-8;
   N-90) und setzt ihm das Passwort des Kontos, wie bei jedem anderen Benutzer (N-91). Ausgenommen sind
   Benutzer 1, andere Administratoren von Seerr und Benutzer mit Anmeldung über Jellyfin, Emby oder Plex: Sie
   werden verknüpft, ihr Passwort bleibt (N-6). Die Übersicht der Verwaltung zeigt sieben Tage lang den Hinweis
   «Neu verknüpft» (N-93, Empfehlung). **Bekannte Grenze:** Niemand prüft die eingetippte Adresse. Wer eine
   Einladung hat und eine fremde Adresse eintippt, bekommt den Seerr-Benutzer dieser Person samt ihren
   Anfragen; Marco hat das so gewählt (E-8; N-92, `04-benutzer-profilmapping.md`, Abschnitt 12.5).
5. Das Limit aus der Einladung (oder die Vorgabe) schreibt Slouchcat in den neuen Benutzer.
6. Ist Seerr gerade nicht erreichbar, entsteht das Konto trotzdem. Der Benutzer in Seerr folgt beim
   nächsten Abgleich mit einem Zufallspasswort; das richtige Passwort übernimmt Slouchcat bei der nächsten
   Anmeldung (N-4). Bis der Seerr-Benutzer besteht, kann das Konto nicht anfordern. Sobald er besteht,
   geht das Anfordern; das Passwort spielt dafür keine Rolle.
7. **Was das Konto erfährt:** Ob Slouchcat den Benutzer neu eröffnet oder einen bestehenden verknüpft hat,
   die Seite sagt nach dem Absenden in beiden Fällen dasselbe: «In Seerr hast du jetzt ein Konto mit derselben
   E-Mail und demselben Passwort.» Das Konto erfährt so nie, ob Seerr die Adresse schon kannte (N-94,
   Empfehlung). Ist Seerr nicht erreichbar, sagt die Seite: Der Zugang zu Seerr ist noch nicht bereit; bis
   dahin kann das Konto nichts anfordern.

Ändert ein Konto ohne Seerr-Benutzer später seine E-Mail in Slouchcat auf eine Adresse, die Seerr schon
kennt, gilt dasselbe wie bei der Einladung: Slouchcat verknüpft sofort. Hat das Konto schon einen
Seerr-Benutzer, ändert Slouchcat dessen E-Mail; lehnt Seerr ab, weil die Adresse vergeben ist, meldet
Slouchcat das und lässt den Benutzer, wie er ist (N-95, Empfehlung).

Ändert die Person später ihr Passwort oder ihre E-Mail in Slouchcat, gleicht Slouchcat Seerr ab. Ändert sie
das Passwort direkt in Seerr, laufen die beiden auseinander (Marco kannte die Folge, E-1). Einzelheiten,
Ausnahmen und Schalter: `04-benutzer-profilmapping.md`, Abschnitte 12 und 13.

### 4.10 Anfordern mit Limit

Marco hat am 17. November 2026 ein Limit je Konto verlangt (Nachricht, Beispiel 10 Filme und 5 Staffeln pro
Woche, rollierend) und entschieden, dass alles zählt und Seerr zählt (E-3). Wie es sich für den Zuschauer
zeigt, ist Empfehlung (N-35, N-36):

1. Die Seiten im Bereich «Wünsche» zeigen oben das Limit des Kontos, je für Filme und Staffeln: «2 / 10 in 7
   Tagen». Administratoren sehen «Limit: keines (Administrator)».
2. Der Dialog «Anfordern» zeigt, wie viel frei ist. Mitgezählt sind auch Wünsche, die jemand im Konto direkt in
   Seerr gestellt hat, und Wünsche, die noch auf die Übermittlung warten.
3. **Limit erreicht:** Der Titel bleibt sichtbar. Der Dialog sperrt «Anfordern» und sagt: «Das Limit ist
   erreicht: 10 Filme in 7 Tagen. Der nächste Platz wird am … frei.»
4. **Zu viele Staffeln gewählt:** Jede gewählte Staffel zählt. Sind weniger frei als gewählt, sagt der Dialog
   «Noch 2 von 5 Staffeln frei. Wähle höchstens 2 Staffeln.» und sperrt «Anfordern», bis die Wahl passt.
5. **Mitwunsch:** «Benachrichtige mich» kostet nichts und geht immer.
6. Abgelehnte und zurückgezogene Wünsche geben ihren Platz frei; fehlgeschlagene zählen weiter, weil Seerr
   sie zählt (Anhang H 3.2).
7. Lehnt Seerr trotz der Vorprüfung ab (etwa weil zur gleichen Zeit jemand direkt in Seerr wünscht),
   wird der Wunsch `failed` mit dem Klartext «Limit erreicht: Seerr hat den Wunsch nicht angenommen.» Der Satz gilt für Filme
   und Staffeln gleich (Redaktionsentscheid, `mockup-abdeckung.md` Abschnitt 10.2, Nr. 28). Eine Benachrichtigung über das erreichte Limit gibt
   es nicht; der Dialog sagt es (`06-benachrichtigungen.md`, Abschnitt 4).

Das Limit gilt für alle Profile eines Kontos gemeinsam; ein Teil-Limit je Profil gibt es nicht. Ein Profil mit
Ausnahme hat das Limit seines Ausnahme-Benutzers (`04-benutzer-profilmapping.md`, Abschnitte 17 und 18).

### 4.11 Stöbern im Bereich «Wünsche»

1. Das Profil wählt «Wünsche» in der Seitenleiste und landet auf «Entdecken» (M-31).
2. «Entdecken» zeigt Reihen wie Seerr: Im Trend, Beliebte Filme, Film-Genres, Demnächst im Kino, Beliebte
   Serien, Serien-Genres, Demnächst als Serie (Empfehlung N-51, F-28). Ein Suchfeld führt zu «Suche».
3. «Filme» (M-32) und «Serien» (M-33) sind Posterraster mit Sortierung, Genre-Filter und «Vorhandene
   ausblenden»; «Mehr laden» holt die nächste Seite.
4. Ein Poster eines fehlenden Titels öffnet die Vorschauseite, sein Plus-Knopf den Dialog. Ein Titel, der schon
   in der Bibliothek steht, trägt eine Marke und führt zur Detailseite.
5. Angefordert wird wie in Ablauf 4.1 oder 4.2.

Aufbau der Seiten: Abschnitt 5.4.

## 5. Zuschauerseiten

| Seite | Route | neu/geändert | Was sich ändert | Mockup |
|---|---|---|---|---|
| Startseite mit Spotlight | `/` | geändert (nur neue Reihe) | Spotlight wie heute: Bühne und Posterreihe nur mit Titeln der Bibliothek, die Bühne ohne Wunsch-Chip und ohne Staffelhinweis (Marco, E-5; N-70). Neu die Reihe «Erfüllte Wünsche» mit den Wünschen des Profils, die in den letzten 14 Tagen wahr wurden, ab Werk nach «Weiterschauen»; dort steht der Chip «Dein Wunsch ist da» (Reihe: Marco, E-10; Chip und Ort: Empfehlung N-74) | M-01 |
| Spotlight-Liste | `/lists/spotlight` | unverändert | nur Titel der Bibliothek, wie heute | M-02 |
| Detailseite Film | `/items/{id}` | fast unverändert | nur vorhandene Filme; neu der Chip «Dein Wunsch ist da» (14 Tage nach der Erfüllung). Ausser hier und in der Reihe «Erfüllte Wünsche» steht der Chip nirgends, auch nicht auf der Spotlight-Bühne | M-05 |
| Detailseite Serie | `/items/{id}` | geändert | nur für Profile, die anfordern können: fehlende Staffeln als gedämpfte Karten, «Staffeln anfordern», Zeile «2 von 4 Staffeln vorhanden» | M-06 |
| Vorschauseite | `/titles/{movie\|show}/{tmdbId}` | neu | Aufbau wie Detailseite; Hauptknopf «Film anfordern» / «Serie anfordern» oder Zustand; Wunschkarte mit Zustand und Download-Fortschritt | M-03, M-04 |
| Suche | `/search` | geändert | Abschnitt «Noch nicht in der Bibliothek» unter den Bibliothekstreffern; neu ein Verweis auf die Wunsch-Suche (N-57) | M-07 |
| Wünsche · Entdecken | `/discover` (Redaktionsentscheid, wie im Mockup) | neu | Reihen wie auf der Startseite von Seerr, Genre-Kacheln, Suchfeld | M-31 |
| Wünsche · Filme | `/discover/movies` | neu | Posterraster mit Sortierung, Genre-Filter, «Vorhandene ausblenden», «Mehr laden» | M-32 |
| Wünsche · Serien | `/discover/series` | neu | wie «Filme» | M-33 |
| Wünsche · Suche | `/discover/search` | neu | Suche über alles, was TMDB kennt, auch nach Kennung `tmdb:`, `tvdb:`, `imdb:` | M-34 |
| Wünsche · Meine Wünsche | `/wishes` | neu | fünfter Reiter des Bereichs; Liste, Filter, Verlauf je Wunsch, Download-Fortschritt, Zurückziehen, erneut anfordern, Anzeige des Limits | M-08 |
| Einladung annehmen | `/invite/$token` | geändert (heute vorhanden, im Mockup neu) | Hinweis «Dein Login gilt auch für Seerr»; beim Absenden wird der Seerr-Benutzer eröffnet; kennt Seerr die Adresse schon, wird der bestehende Benutzer sofort verknüpft (Marco, E-8). Die Meldung ist in beiden Fällen dieselbe (Ablauf 4.9) | M-35 |
| Benachrichtigungen | `/notifications` | geändert | neue Arten mit Link | M-09 |
| Benachrichtigungskanäle | `/settings/notifications` | geändert | neue Ereignisse je Kanal | M-10 |
| Profile | `/settings/profiles` | geändert | Schalter «Darf Titel anfordern» im Profildialog, Chip auf der Karte | M-11 |
| Profilwahl | `/profiles` | unverändert | im Mockup bedienbar, um Profile zu wechseln | M-12 |
| Dialog «Anfordern» | von M-03, M-04, M-06, M-07 und M-31 bis M-34; von M-08 («Erneut anfordern») und von M-27 («Anfordern» des Administrators, im aktiven Profil) | neu | Film, Serie mit Staffelwahl, Mitwunsch, Limit (frei, erreicht, zu viele Staffeln), Sonderfälle | M-13 |

**Seitenleiste:** neuer Eintrag «Wünsche» nach «Gesehen» mit Zähler offener Wünsche. Offene Wünsche stehen
damit nicht auf der Startseite, sondern unter «Meine Wünsche» (N-74). Er führt in den Bereich
«Wünsche» (Abschnitt 5.4): auf «Entdecken» (`/discover`), bei einem Profil, das nur Wünsche hat, auf «Meine
Wünsche» (`/wishes`). Er erscheint nur, wenn das Profil anfordern darf oder Wünsche hat; ein Profil, das
nicht anfordern kann, aber Wünsche hat, sieht dort nur «Meine Wünsche».

### 5.1 Vorschauseite

- Kopf wie die Detailseite: Hintergrund, Logo oder Titel, Metazeile (Jahr, Laufzeit oder Staffelzahl,
  Genres, Freigabe) mit dem Chip «Noch nicht in der Bibliothek», Beschreibung.
- Knöpfe: Hauptknopf «Film anfordern» / «Serie anfordern» oder die Zustandsanzeige; «Trailer», wenn einer da ist.
- **Wunschkarte:** Hat das Profil einen Wunsch zu diesem Titel, zeigt eine Karte unter dem Kopf Zustand, Datum
  und, solange der Wunsch «In Beschaffung» ist, den Download-Fortschritt, gleich wie «Meine Wünsche»
  (Abschnitt 5.3).
- Nach einem geschlossenen Wunsch (abgelehnt, fehlgeschlagen, zurückgezogen) steht wieder «Film anfordern»,
  darunter der letzte Wunsch mit Datum und Grund (Abschnitt 2).
- Erreichbar ist die Seite nur für Profile, die den Titel anfordern können, und nur, solange mindestens eine
  der Stellen («Anfordern aus der Suche», «Anfordern auf Detailseiten», «Seiten Entdecken, Filme, Serien und
  Suche») an ist. Sonst antwortet sie 404. Ausnahme: Das Profil hat selbst einen Wunsch zu
  diesem Titel (Abschnitt 3).
- Bei Serien die Staffelliste laut TMDB mit Folgenzahl; laufende Serie mit «Läuft noch, nächste Folge am …».
  Welche Staffeln der Dialog anbietet, kommt aus Seerr (Abschnitt 6).
- Kein «Abspielen», kein «Merken», kein Gesehen-Haken, kein «Empfehlen», kein «Herunterladen»: diese hängen
  an Bibliothekstiteln.
- **Vorschlag:** Steht der Titel inzwischen in der Bibliothek und darf das Profil ihn sehen, leitet die
  Vorschauseite auf die Detailseite `/items/{id}` weiter.

### 5.2 Suche

- Die Bibliothekssuche bleibt unverändert oben.
- Darunter der Abschnitt «Noch nicht in der Bibliothek» aus der TMDB-Suche über Slouchcats eigenen
  TMDB-Schlüssel (F-11), gefiltert: ohne Titel aus der Bibliothek, ohne Titel über der Altersstufe, nur für
  Profile, die anfordern können, und nur, wenn «Anfordern aus der Suche» an ist.
- Die TMDB-Suche liefert keine Freigaben. Für Profile unter 18 holt der Server darum je Treffer die Details
  (höchstens die ersten 20 Treffer) und speichert sie in `external_titles`. Treffer ohne bekannte Freigabe
  gelten als 18 und erscheinen diesen Profilen nicht. Für Profile ab 18 genügt die Suchantwort
  (`03-architektur.md`, Abschnitt 16).
- Treffer als Poster mit Unterzeile «2026 · Noch nicht da» oder Zustand und rundem Plus-Knopf.
- **Neu (Empfehlung N-57):** Unter den Treffern verweist die Seite auf die Wunsch-Suche im Bereich «Wünsche»
  (M-34), mit demselben Suchbegriff. Die Bibliothekssuche selbst bleibt, wie oben beschrieben.

### 5.3 Meine Wünsche

- Fünfter Reiter des Bereichs «Wünsche» (Abschnitt 5.4). Für ein Profil, das nicht anfordern kann, aber Wünsche
  hat, ist es der einzige Reiter.
- Oben das Limit des Kontos (Ablauf 4.10).
- Liste der Wünsche des Profils, neueste zuerst; Filter nach Zustand (Vorschlag: «Offen», «Verfügbar»,
  «Abgeschlossen»).
- Je Wunsch: Poster, Titel, bei Serien die Staffeln, Zustand mit Datum, aufklappbarer Verlauf (angefordert,
  übermittelt, freigegeben, verfügbar). Solange der Wunsch «In Beschaffung» ist, dazu der Download-Fortschritt
  (unten).
- Aktionen je Zustand: «Zurückziehen», «Erneut anfordern», «Abspielen» bzw. «Zum Titel».
- Wünsche aus Seerr, die Slouchcat übernimmt (F-21, Stufe 4), tragen «In Seerr angelegt».
- Das Hauptprofil sieht über `listAccountWishes` (`scoped`, nur mit Hauptprofil) auch die Wünsche aller
  Profile des Kontos. Das ist gewollt: Das Hauptprofil verwaltet das Konto, setzt und entfernt PINs und sieht
  Schauzeit und Einstellungen aller Profile. Die PIN-Regel F-15 schützt etwas anderes, nämlich externe
  Nachrichten, die jedes Gerät des Kontos erreichen (`06-benachrichtigungen.md`, Abschnitt 8).
- Offline zeigt die Seite «Nur online verfügbar» (Wünsche haben keinen Offline-Zustand).

**Download-Fortschritt.** Marco will bei Wünschen sehen, wie weit Radarr oder Sonarr sind, «unter «Meine
Wünsche» und in der Verwaltung» (E-7). Dafür liest Slouchcat die Warteschlange von Radarr und Sonarr über
denselben Zugang wie für den Profil-Tag (E-6, N-81). Die Einzelheiten sind Empfehlung (N-82 bis N-87):

| Anzeige | Wann |
|---|---|
| «Wird gesucht» | Wunsch «In Beschaffung», in der Warteschlange steht noch nichts |
| «Wird heruntergeladen · 45 % · noch 12 min» | die Warteschlange meldet einen Download mit Anteil und Restzeit |
| «Wird eingelesen» | Radarr oder Sonarr haben importiert, Slouchcat hat den Titel noch nicht |
| «Download verzögert» | die Warteschlange meldet eine Warnung oder einen Fehler |

- Bei Serien steht dazu «x von y Folgen da».
- Der Fortschritt steht nur beim Wunsch: unter «Meine Wünsche», auf der Wunschkarte der Vorschauseite (M-03,
  M-04) und in der Verwaltung unter Anfragen. Nicht an Titeln der Bibliothek, nicht in Reihen, nicht in der
  Suche (N-82). Die Spec führt den Beschaffungsstand als Nicht-Ziel; das ist eine eng gefasste Ausnahme nur beim
  Wunsch (Anhang H 8.5).
- Die Zuschauer-App zeigt nur diese Worte. Was Radarr oder Sonarr genau melden (etwa eine Fehlermeldung des
  Download-Programms), sieht nur die Verwaltung (N-83).
- Der Server fragt die Warteschlange jede Minute ab, nur solange es Wünsche in Beschaffung gibt, ein Abruf je
  Dienst (Job `wishes.progress`). Die Seite liest den Zwischenspeicher; beim Seitenaufbau ruft niemand Radarr
  oder Sonarr auf (N-84).
- **Radarr oder Sonarr nicht erreichbar:** Die Zuschauer-App zeigt nichts Zusätzliches. Der Wunsch steht
  weiter auf «In Beschaffung», ohne Fortschritt und ohne Fehlerhinweis. Die Verwaltung zeigt «Fortschritt nicht
  lesbar» (N-85).
- Der Fortschritt ist nur Anzeige. Er ändert keinen Zustand des Wunsches; «verfügbar» heisst weiter, dass
  Slouchcat den Titel abspielen kann (N-87, F-13). Er löst keine Benachrichtigung aus, auch «Download verzögert»
  nicht (`06-benachrichtigungen.md`, Abschnitt 4).
- Schalter «Download-Fortschritt zeigen» unter Wünsche › Einstellungen, ab Werk an (N-86). Ist er aus, steht nur
  «In Beschaffung».

### 5.4 Bereich «Wünsche»: Entdecken, Filme, Serien, Suche

Marco hat am 17. November 2026 «zusätzlich eine Seite für Wünsche» verlangt, «analog der Startseite von Seerr.
Discover, Movies, Series und Search» (Nachricht). Der Aufbau ist Empfehlung (N-50 bis N-58):

| Reiter | Inhalt | Mockup |
|---|---|---|
| «Entdecken» | Reihen wie Seerr: Im Trend, Beliebte Filme, Film-Genres (Kacheln), Demnächst im Kino, Beliebte Serien, Serien-Genres (Kacheln), Demnächst als Serie; dazu ein Suchfeld. Die Reihen «Recently Added», «Recent Requests», «Plex Watchlist», «Studios», «Networks» von Seerr entfallen (F-28). | M-31 |
| «Filme» | Posterraster mit Sortierung (Beliebtheit, Neueste zuerst, Titel), Genre-Filter und «Vorhandene ausblenden»; der Filter ist zugeklappt und zeigt seine Wahl im Kopf; «Mehr laden» | M-32 |
| «Serien» | wie «Filme» | M-33 |
| «Suche» | sucht in allem, was TMDB kennt, auch nach Kennung: `tmdb:`, `tvdb:`, `imdb:`. Personen zeigt sie nicht. | M-34 |
| «Meine Wünsche» | eigene Wünsche (Abschnitt 5.3); fünfter Reiter ist Redaktionsentscheid, Marco nannte vier | M-08 |

- **Woher die Daten kommen:** Der Server holt sie über die Discover- und Suchrouten von Seerr
  (`GET /discover/trending`, `/discover/movies`, `/discover/tv`, `/discover/movies/upcoming`,
  `/discover/tv/upcoming`, `/discover/genreslider/…`, `GET /search`; Anhang H 3.3), im Namen des
  Seerr-Benutzers, mit Zwischenspeicher (1 Stunde je Liste, Seite, Sprache und Altersstufe; Job
  `wishes.discover`). Der Client spricht nie mit Seerr oder TMDB (N-54).
- **Vorhandene Titel** erscheinen auch, mit Marke und Link zur Detailseite. Massgeblich ist der Abgleich von
  Slouchcat (bei Serien TVDB zuerst, dann TMDB, dann IMDb; `05-spotlight-integration.md`, Abschnitt 3.3), nicht
  der Zustand, den Seerr mitliefert (N-55).
- **Wer den Bereich sieht:** Es gilt F-9 unverändert: «Entdecken», «Filme», «Serien» und «Suche» sieht nur ein
  Profil, das anfordern kann (Abschnitt 3). Alle anderen sehen höchstens «Meine Wünsche». Die Altersstufe gilt
  für jeden Titel, unbekannt zählt als 18; für Profile mit Altersgrenze fragt der Server Seerr mit
  `certificationLte` und prüft jeden Treffer selbst nach (N-56).
- **Schalter:** «Seiten Entdecken, Filme, Serien und Suche» unter Wünsche › Einstellungen (N-57).
- **Kein Verstoss gegen das Nicht-Ziel «algorithmische Empfehlungen»:** Die Listen sind öffentliche
  TMDB-Ranglisten, für alle gleich. S-4 («Titel ohne Bibliothekstreffer nie auf der Startseite») bleibt ohne
  Ausnahme, die Seiten liegen unter «Wünsche» (N-58, N-70).
- **Keine Reihe aus dem Spotlight:** Fehlende Einträge der Trakt-Liste erscheinen auch unter «Entdecken» nicht
  (Marco, E-5).
- **Seerr nicht erreichbar:** Listen vom letzten Abruf, Suche nicht möglich (Ablauf 4.4).

### 5.5 Dialog «Anfordern» (M-13)

| Zustand | Was der Dialog zeigt |
|---|---|
| Limit frei | «Noch 8 von 10 Filmen frei in 7 Tagen»; «Anfordern» aktiv |
| ohne Limit | «ohne Limit» bzw. bei Administratoren «keines (Administrator)» |
| Limit erreicht | «Das Limit ist erreicht: 10 Filme in 7 Tagen. Der nächste Platz wird am … frei.»; «Anfordern» gesperrt; «Benachrichtige mich» bleibt, wo es einen offenen Wunsch gibt |
| zu viele Staffeln | «Noch 2 von 5 Staffeln frei. Wähle höchstens 2 Staffeln.»; «Anfordern» gesperrt, bis die Wahl passt |

Dazu wie bisher: Poster, Titel, «Wird als {{Seerr-Benutzer}} an Seerr übermittelt», «Wird sofort
beschafft» oder «Braucht eine Freigabe», bei Serien die Staffelwahl aus Seerr (Abschnitt 6).

### 5.6 Einladung annehmen (M-35)

Die Seite gibt es heute (`/invite/$token`, `web/src/app/router.tsx:237`); im Mockup ist sie neu. Neu sind der
Hinweis «Dein Login gilt auch für Seerr» und die Eröffnung beim Absenden (Ablauf 4.9). Fehlt Seerr
oder ist «Neue Konten in Seerr eröffnen» aus, entfällt der Hinweis (Vorschlag).

Nach dem Absenden sagt die Seite nur zwei Dinge: Der Zugang zu Seerr ist bereit («In Seerr hast du jetzt ein
Konto mit derselben E-Mail-Adresse.»), oder er ist noch nicht bereit, weil Seerr nicht erreichbar
ist, und das Konto kann bis dahin nichts anfordern. Die erste Meldung ist dieselbe, ob Slouchcat den Benutzer
neu eröffnet oder einen bestehenden verknüpft hat (Marco, E-8, sofort verknüpfen; gleiche Meldung: N-94,
Empfehlung). Ob Seerr die eingetippte Adresse kannte, zeigt die Seite nie.

## 6. Serien und Staffeln

| Fall | Erkennung | Was das Profil sieht |
|---|---|---|
| Serie vollständig vorhanden | alle Staffeln laut `series_catalog` in der Bibliothek | Detailseite wie heute; kein Knopf |
| Serie fehlt ganz | keine Bibliothekszeile | Vorschauseite (M-04), «Serie anfordern», Staffelwahl |
| Serie teilweise vorhanden | einige Staffeln vorhanden, andere laut TMDB nicht | Detailseite (M-06), nur für Profile, die anfordern können: Zeile «2 von 4 Staffeln vorhanden»; fehlende Staffeln als gedämpfte Karten in der Staffelwand; Aktion «Staffeln anfordern». Profile ohne Wunschrecht sehen die Serie wie heute, ohne fehlende Staffeln und ohne diese Zeile |
| Einzelne Staffeln fehlen mitten drin | Lücke in der Staffelliste | wie teilweise vorhanden; Staffelwahl bietet die Lücken, soweit Seerr sie kennt |
| Staffel schon angefordert | offener Wunsch mit dieser Staffel | gedämpfte Karte mit Zustand; im Dialog wählbar mit «Bereits angefordert»; das Profil wird für diese Staffel Mitwünscher |
| Staffel in Beschaffung | Wunschstaffel im Zustand `processing` | gedämpfte Karte «In Beschaffung» |
| Staffel teilweise eingelesen | einige Folgen der Staffel da | Staffel wie heute schaubar; sie gilt für den Wunsch erst als verfügbar, wenn sie vollständig ist (Abschnitt 4.7); **Vorschlag:** die Karte zeigt «5 von 10 Folgen» |
| Künftige Folgen | TMDB-Status «läuft», nächste Folge mit Datum | Zeile «Läuft noch, nächste Folge am …»; nichts anzufordern |
| Künftige Staffel | angekündigt, aber ohne Folgen | nicht wählbar; **Vorschlag:** Karte «Angekündigt» ohne Knopf |
| Staffel 0 (Specials) | Staffelnummer 0 | nie wählbar |

Einzelne Folgen lassen sich nicht anfordern (Seerr arbeitet staffelweise, F-12). Slouchcat schickt die
TVDB-ID immer mit; ohne sie löscht Seerr den Wunsch beim Senden an Sonarr. Serien, die ganz fehlen,
kennt die Vorschauseite aus `external_titles`; Serien in der Bibliothek ihre Staffelliste aus `series_catalog`.
Die Staffelwand für fehlende Staffeln kommt mit Stufe 4.

**Staffeln im Dialog kommen aus Seerr** (Empfehlung N-63): Der Dialog fragt `GET /tv/{tmdbId}` und bietet
genau die Staffeln an, die Seerr bestellen kann. Seerr liest die Staffelliste ab Werk aus TMDB; auf
Wunsch aus TVDB (Einstellung «Metadaten-Anbieter»). TMDB und TVDB teilen manche Serien verschieden auf, Sonarr
und die Bibliothek zählen nach TVDB. Empfohlen ist darum, Seerr auf TVDB zu stellen (N-64, F-29); dann
zählen Dialog, Sonarr und Bibliothek gleich (`05-spotlight-integration.md`, Abschnitt 3).

## 7. Wunschrecht und Jugendschutz aus Sicht des Benutzers

- **Was ein Profil nicht sehen darf, existiert für es nicht.** Ein fehlender Titel über der Altersstufe
  erscheint weder im Bereich «Wünsche» noch in der Suche, und seine Vorschauseite antwortet «nicht gefunden» (404).
  Er erscheint nicht als «gesperrt». Dasselbe gilt für jedes Profil, das nicht anfordern kann (Abschnitt 3).
- Die Freigabe stammt aus TMDB, gelesen in der Länderreihenfolge nach ADR 0034: gewähltes Freigabeland, dann
  DE, AT, CH, GB, US; das erste Land mit bekanntem Wert gilt (dieselbe Funktion wie für Bibliothekstitel);
  innerhalb eines Lands gilt der strengste Wert über die Anbieter. Verglichen wird mit der tieferen Grenze von
  Profil und Konto. **Unbekannte Freigabe zählt als 18.**
- Ein Kinderprofil mit Wunschrecht darf anfordern (F-6). Ob der Wunsch eine Freigabe braucht, entscheidet die
  Auto-Freigabe des Seerr-Benutzers, als der es sendet. Im Normalfall ist das der Benutzer des Kontos; im
  Konto des Administrators wäre der Wunsch damit sofort freigegeben und ohne Limit. Dafür gibt es die
  zugeklappte Ausnahme «Profil sendet als anderer Seerr-Benutzer» (Empfehlung F-27,
  `04-benutzer-profilmapping.md`, Abschnitt 18). Slouchcat baut keine eigene Freigabe.
- Beispiel aus den Mockup-Daten: Noah (12) sendet mit einer Ausnahme als Seerr-Benutzer «kinder» (keine
  Auto-Freigabe, eigenes Limit 5 Filme und 2 Staffeln je 7 Tage). Sein Wunsch zeigt «Freigabe ausstehend», bis
  Marco ihn in Seerr freigibt. Mia (6) darf nicht anfordern und sieht keine fehlenden Titel.
- Die Benachrichtigung «verfügbar» kommt für ein Profil erst, wenn der Titel für **dieses** Profil sichtbar
  ist. Ein Erwachsener mit einem Mitwunsch kann also früher Bescheid bekommen als ein Kind.
- Zuschauer sehen nie, wer sonst gewünscht hat. Die Verwaltung sieht alle Anforderer; in Radarr und Sonarr
  sieht man es am Profil-Tag (Marco, E-2).

## 8. Was bewusst NICHT gebaut wird

| Nicht gebaut | Warum |
|---|---|
| eigene Beschaffung, Ansteuern von Radarr, Sonarr oder SABnzbd | bleibt Non-Goal. Zwei eng gefasste Ausnahmen: Slouchcat legt in Radarr und Sonarr den Profil-Tag an und hängt ihn an den Titel (Marco, E-2), und es liest die Warteschlange für den Download-Fortschritt (Marco, E-7). Es fügt nichts hinzu, löscht nichts, sucht nichts und ändert keine Qualität |
| Webhooks in Radarr und Sonarr selbst eintragen, Abfragen statt Webhook | von Marco nicht gewählt (E-6): Die Webhooks bleiben, wie sie heute sind, von Hand eingerichtet; Slouchcat trägt keinen Webhook ein und ersetzt ihn nicht durch Abfragen |
| fehlende Titel im Spotlight, auf der Startseite oder als Reihe unter «Entdecken» | Marco, E-5: Startseite und Spotlight zeigen nur Titel der Bibliothek |
| Beschaffungsstand an Titeln der Bibliothek, in Reihen oder in der Suche | bleibt Nicht-Ziel; der Download-Fortschritt steht nur beim Wunsch (N-82) |
| Wahl von Qualität, 4K, Qualitätsprofil, Zielordner, eigene Tags beim Anfordern | gehört Seerr und Radarr/Sonarr; den Profil-Tag setzt Slouchcat selbst, der Zuschauer wählt nichts |
| Limit je Profil | das Limit gilt je Konto für alle Profile gemeinsam (Empfehlung N-38) |
| Personen in der Wunsch-Suche, Reihen «Studios», «Networks», «Plex Watchlist» | nicht nötig zum Anfordern (Empfehlung N-51, N-53) |
| einzelne Folgen anfordern | Seerr arbeitet staffelweise |
| eigener Freigabe-Ablauf in Slouchcat (Kind fragt Eltern) | bleibt Non-Goal; die Freigabe liegt in Seerr. Durchreichen von «Freigeben/Ablehnen» für Administratoren (F-16) ist kein eigener Ablauf |
| Wünsche offline | `/api` ist NetworkOnly; kein Offline-Zustand |
| Ablehnungsgrund | Seerr kennt keinen |
| Anzeige, wer sonst gewünscht hat | Datenschutz zwischen Profilen |
| Merken, Gesehen, Empfehlen für fehlende Titel | hängen an Bibliothekstiteln |
| Wunschliste aus der Trakt-Merkliste, Trakt-Login | Non-Goal (Trakt nur öffentliche Listen) |
