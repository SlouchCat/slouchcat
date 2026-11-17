# Konzept 25 · Funktionskonzept: was Zuschauer und Administrator erleben

> **Kurz gesagt.** Ein Profil, das anfordern kann, sieht Filme und Serien, die noch fehlen, im Spotlight, in
> der Suche und auf einer Vorschauseite. Ein Druck auf «Film anfordern» öffnet einen Dialog; Slouchcat
> reicht den Wunsch im Namen des zugeordneten Jellyseerr-Benutzers weiter und zeigt danach seinen
> Zustand. Ob ein Wunsch eine Freigabe braucht, entscheidet allein Jellyseerr. Sobald Slouchcat den Titel
> für das Profil abspielen kann, kommt eine Benachrichtigung. Slouchcat beschafft nichts selbst und
> steuert weder Radarr noch Sonarr.

Grundlage: [Anhang F](anhang/F-entscheid-blatt.md) (Begriffe, Zustände, Seiten). Was heute schon da ist, steht in
`01-bestandsanalyse.md`; das Spotlight im Einzelnen in `05-spotlight-integration.md`. Die Verwaltung steht in
`07-administration.md`, Zuordnung und Rechte in `04-benutzer-profilmapping.md`, die Benachrichtigungen in
`06-benachrichtigungen.md`, Datenmodell und API in `08-datenmodell-api.md`. Abschnitte mit **Vorschlag** gehen
über Anhang F hinaus und brauchen Marcos Ja.

## 1. Begriffe

| Begriff | Bedeutung |
|---|---|
| **Wünsche** | die Funktion und ihre Seiten; Zuschauerseite «Meine Wünsche» (`/wishes`) |
| **Wunsch** | ein Eintrag: ein Film oder eine Serie mit gewählten Staffeln, angefordert von einem Profil |
| **anfordern** | die Tätigkeit auf Knöpfen: «Film anfordern», «Serie anfordern», «Staffeln anfordern» |
| **fehlender Titel** | Film oder Serie, die nicht in der Bibliothek steht; in der Oberfläche «Noch nicht in der Bibliothek» |
| **Vorschauseite** | Detailseite eines fehlenden Titels (`/titles/{movie\|show}/{tmdbId}`) |
| **Mitwunsch** | ein weiteres Profil hängt sich an einen offenen Wunsch und wird ebenfalls benachrichtigt |
| **Jellyseerr** | der Dienst, der Wünsche verwaltet und an Radarr/Sonarr weitergibt (technisch Seerr 3.x, Paket `seerr`) |
| **Jellyseerr-Benutzer** | der Benutzer in Jellyseerr, in dessen Namen Slouchcat einen Wunsch anlegt; der Administrator ordnet ihn einem Konto oder Profil zu |
| **Wunschrecht** | ob ein Profil anfordern darf; Schalter je Konto (Administrator) und je Profil (Hauptprofil) |

Die Begriffe sind offen als F-1.

## 2. Zustände eines Wunsches

Slouchcat bildet die Zustände selbst, aus Jellyseerr und aus der eigenen Bibliothek.

| Schlüssel | Anzeige | Bedeutung | Was der Zuschauer tun kann |
|---|---|---|---|
| `queued` | «Wird übermittelt» | lokal gespeichert, noch nicht bei Jellyseerr | warten; zurückziehen |
| `pending` | «Freigabe ausstehend» | bei Jellyseerr, wartet auf Freigabe | warten; zurückziehen |
| `processing` | «In Beschaffung» | freigegeben, Radarr/Sonarr beschaffen; mit Zusatz «wird eingelesen», wenn Jellyseerr schon «verfügbar» meldet, Slouchcat die Datei aber noch nicht hat | warten |
| `partial` | «Teilweise verfügbar» | Serie: ein Teil der gewünschten Staffeln ist in Slouchcat vollständig abspielbar (Abschnitt 6) | vorhandene Staffeln schauen |
| `available` | «Verfügbar» | in Slouchcat für dieses Profil abspielbar | abspielen |
| `declined` | «Abgelehnt» | in Jellyseerr abgelehnt, ohne Grund (Jellyseerr kennt keinen) | erneut anfordern (Abschnitt 4.6) |
| `failed` | «Fehlgeschlagen» | Übergabe an Radarr/Sonarr gescheitert oder endgültig abgewiesen (Kontingent, Sperrliste, kein Recht) | Klartext lesen; erneut anfordern, ausser die Übergabe an Radarr/Sonarr ist gescheitert (dann wiederholt ein Administrator) |
| `withdrawn` | «Zurückgezogen» | vom Profil zurückgezogen, solange nicht freigegeben | erneut anfordern |

«Verfügbar» heisst: Slouchcat kann den Titel für **dieses** Profil abspielen, nicht der Zustand in
Jellyseerr (F-13). Ein Titel, den Jellyseerr «verfügbar» meldet, bleibt «In Beschaffung · wird
eingelesen», bis Slouchcat ihn zugeordnet hat.

**Geschlossene Wünsche.** Ein abgelehnter, fehlgeschlagener oder zurückgezogener Wunsch ist geschlossen. Der
Titel gilt danach wieder als fehlend und trägt wieder «Film anfordern» bzw. «Serie anfordern». Die
Vorschauseite nennt den letzten Wunsch mit Datum und Grund, etwa «Am 12. November abgelehnt». Ausnahme: Ist
in Jellyseerr die Übergabe an Radarr oder Sonarr gescheitert (`fail_reason = arr_failed`), sperrt der
Request dort den Titel weiter. Dann fehlt «anfordern», und der Zuschauer liest «Ein Administrator kann den
Wunsch wiederholen» (`03-architektur.md`, Abschnitt 8.1).

Den vollständigen Zustandsautomaten mit allen Übergängen zeigt `03-architektur.md`, Abschnitt 8.2. Dieses
Papier wiederholt ihn nicht, damit es nur eine Fassung gibt.

## 3. Wer anfordern darf

Ein Profil darf anfordern, wenn alles zutrifft:

1. Der Administrator hat Jellyseerr eingerichtet und eingeschaltet («Jellyseerr verwenden») und «Wünsche
   aktiv» eingeschaltet (ab Werk aus).
2. Das Konto hat das Wunschrecht (Administrator, Formular «Zugriff»; bei Administratorkonten der Block
   «Wünsche»). Vorgabe: Besitzerkonto an, neue Konten aus.
3. Das Profil hat das Wunschrecht (Hauptprofil, Profildialog). Vorgabe: Hauptprofil an, weitere Profile aus.
4. Profil oder Konto sind einem Jellyseerr-Benutzer zugeordnet, oder der Sammelbenutzer greift. Konten ohne
   Zuordnung dürfen nicht anfordern (Vorgabe; Sammelbenutzer als Wahl, F-4).
5. Die Medienart ist erlaubt («Filme erlauben», «Serien erlauben») und die Stelle ist erlaubt
   («Anfordern aus dem Spotlight», «… aus der Suche», «… auf Detailseiten»).
6. Die Freigabe des Titels passt zur Altersstufe des Profils (Abschnitt 7).

**Eine Regel für das Sehen (F-9):** Fehlende Titel sieht ein Profil nur, wenn es sie auch anfordern kann,
also wenn alle sechs Punkte zutreffen. Sonst existieren sie für das Profil nicht: Sie fehlen in Spotlight und
Suche, und die Vorschauseite antwortet 404. Einen «Hinweis statt Knopf» gibt es nicht. Einzige Ausnahme:
Titel, zu denen das Profil selbst einen Wunsch hat (in jedem Zustand), bleiben auf der Vorschauseite und unter
«Meine Wünsche» sichtbar. Ist Jellyseerr nur vorübergehend nicht erreichbar oder lehnt den Schlüssel ab,
bleiben fehlende Titel sichtbar; neue Wünsche warten im Ausgangsfach (Abschnitt 4.4).

Ein Konto mit Recht, aber ohne Zuordnung bemerkt der Administrator in der Verwaltung (Übersicht, Zuordnung,
Konten), nicht der Zuschauer (`07-administration.md`, Abschnitte 2, 6 und 8).

Der Server liefert `canRequest` und einen Grund; die App zeigt nur an (Regel 12). Recht, Kontingent und
Auto-Freigabe in Jellyseerr gelten so, als hätte der zugeordnete Jellyseerr-Benutzer selbst geklickt
(`X-API-User`, F-3). Zuordnung und Vorgaben im Einzelnen: `04-benutzer-profilmapping.md` (F-2, F-5).

## 4. Abläufe

Jeder Ablauf beginnt bei einem Profil, das anfordern kann (Abschnitt 3), sofern nicht anders gesagt. Mockups: Dialog M-13.

### 4.1 Film aus dem Spotlight anfordern

1. Auf der Startseite (M-01) zeigt die Bühne einen fehlenden Film. Die Metazeile trägt den Chip «Noch
   nicht in der Bibliothek». Knöpfe: «Film anfordern» (Plus-Symbol), «Details», «Trailer».
2. Das Profil drückt «Film anfordern». Der Dialog öffnet sich: Poster, Titel, «Wird als {{Jellyseerr-Benutzer}}
   an Jellyseerr übermittelt», Hinweis «Wird sofort beschafft» oder «Braucht eine Freigabe», Knöpfe
   «Anfordern» und «Abbrechen» (F-10).
3. Das Profil drückt «Anfordern». Slouchcat speichert den Wunsch (`queued`) und versucht die Übermittlung
   sofort (höchstens 3 s).
4. Die Antwort kommt am Bildschirm, nicht als Benachrichtigung: Der Dialog schliesst, eine kurze Meldung
   bestätigt.
5. Der Hauptknopf der Bühne wird zur Zustandsanzeige, etwa «Freigabe ausstehend» oder «In Beschaffung»; er
   führt zu «Meine Wünsche» (M-08). In der Posterreihe zeigt das Poster unten rechts das Zustandssymbol statt
   des Plus-Knopfs.
6. Der Wunsch steht in «Meine Wünsche» und in der Startreihe «Deine Wünsche» (F-20; ab Werk nach
   «Weiterschauen», in der Verwaltung ausblendbar).

Derselbe Ablauf gilt aus der Suche (M-07), der Spotlight-Liste (M-02) und der Vorschauseite (M-03).

### 4.2 Serie oder Staffeln anfordern

1. Das Profil öffnet eine fehlende Serie (Vorschauseite M-04) oder eine vorhandene Serie mit fehlenden
   Staffeln (Detailseite M-06).
2. Es drückt «Serie anfordern» bzw. «Staffeln anfordern». Der Dialog zeigt die Staffelwahl: «Alle fehlenden
   Staffeln» (vorgewählt) oder einzelne Staffeln mit Folgenzahl und Jahr.
3. Nicht wählbar sind: Staffel 0 (Specials), Staffeln ohne Folgen und Staffeln, die schon vorhanden sind.
   Staffeln, die ein anderes Profil schon angefordert hat, sind wählbar und tragen «Bereits angefordert» mit
   ihrem Zustand. Für sie wird das Profil Mitwünscher (Abschnitt 4.3).
4. «Anfordern» legt **einen** Wunsch mit den übrigen gewählten Staffeln an; nur diese gehen an Jellyseerr.
   Weiter wie 4.1 ab Schritt 3.
5. Werden später weitere Staffeln angefordert, entsteht ein weiterer Wunsch nur mit diesen Staffeln
   (Jellyseerr arbeitet so).

Alle Unterscheidungen bei Serien: Abschnitt 6.

### 4.3 Schon angefordert: Mitwunsch

1. Ein zweites Profil öffnet einen Titel, den ein anderes Profil schon angefordert hat. Es sieht den
   Zustand («Freigabe ausstehend», «In Beschaffung»), aber nicht, wer gewünscht hat.
2. Statt «Film anfordern» steht «Benachrichtige mich» (F-7).
3. Ein Druck hängt das Profil als Mitwünscher an. Slouchcat ruft Jellyseerr dabei nicht auf (dort sind
   doppelte Wünsche gesperrt).
4. Der Wunsch erscheint auch in «Meine Wünsche» dieses Profils. Wird der Titel verfügbar, bekommt auch der
   Mitwünscher die Benachrichtigung.
5. Bei Serien gilt der Mitwunsch je Staffel: Im Dialog wählt das Profil auch Staffeln mit «Bereits
   angefordert». Für diese wird es Mitwünscher; nur die übrigen Staffeln gehen als neuer Wunsch an Jellyseerr.

Steht «Bereits angeforderte Titel» auf «Nur Zustand zeigen», fehlt der Knopf; der Zustand bleibt sichtbar.

### 4.4 Jellyseerr nicht erreichbar

1. Das Profil fordert an wie in 4.1.
2. Slouchcat erreicht Jellyseerr nicht innert 3 s. Der Wunsch bleibt `queued` («Wird übermittelt»).
3. Der Dialog sagt: «Wird übermittelt, sobald Jellyseerr erreichbar ist.» Für das Profil ist der Wunsch
   damit angenommen.
4. Der Job `wishes.submit` versucht es mit wachsendem Abstand erneut. Ab dem zweiten Versuch prüft er vorher,
   ob der Wunsch in Jellyseerr schon existiert; dann hängt Slouchcat sich an.
5. Nach 24 h ohne Erfolg wird der Wunsch `failed`; die Administratoren bekommen ein Fehlerereignis. Das Profil
   sieht den Zustand in «Meine Wünsche»; eine Eingangsmeldung «Fehlgeschlagen» kommt erst ab Stufe 4. Die
   Frist ruht, solange der Administrator «Jellyseerr verwenden» ausgeschaltet hat (**Vorschlag**, T-2 in
   `11-entscheide-und-adr.md`).

Die Verwaltung zeigt «Jellyseerr nicht erreichbar» auf der Übersicht (M-20).

### 4.5 Kein Recht

| Fall | Was das Profil sieht |
|---|---|
| Profil oder Konto ohne Wunschrecht | keine fehlenden Titel im Spotlight und in der Suche, keine fehlenden Staffeln (F-9); kein Eintrag «Wünsche» in der Seitenleiste, ausser es hat schon Wünsche |
| Profil ohne Wunschrecht öffnet eine Vorschauseite über einen Link | 404 «nicht gefunden»; Ausnahme: Das Profil hat selbst einen Wunsch zu diesem Titel, dann Seite ohne Anforderungsknopf |
| Konto ohne Zuordnung (und kein Sammelbenutzer) | wie ohne Wunschrecht: keine fehlenden Titel. Der Administrator sieht den Fall in der Verwaltung (Übersicht «n Konten ohne Zuordnung», Zuordnung, Chip «Zuordnung fehlt» auf «Konten») |
| Medienart ausgeschaltet | keine fehlenden Titel dieser Art |
| «Anfordern aus dem Spotlight» aus | das Spotlight zeigt keine fehlenden Titel |
| «Anfordern aus der Suche» aus | kein Abschnitt «Noch nicht in der Bibliothek» |
| «Anfordern auf Detailseiten» aus | keine fehlenden Staffeln auf der Serienseite; die Vorschauseite bleibt erreichbar, solange mindestens eine der drei Stellen an ist |
| Jellyseerr-Benutzer darf in Jellyseerr nicht anfordern oder Kontingent erschöpft | Slouchcat nimmt an, Jellyseerr weist ab: Wunsch `failed` mit Klartext, etwa «Kontingent für Filme erschöpft» |
| Jellyseerr kurz nicht erreichbar oder Schlüssel abgelehnt | fehlende Titel bleiben sichtbar; der Wunsch wartet als «Wird übermittelt» (Abschnitt 4.4) |

### 4.6 Ablehnung

1. Ein Administrator lehnt den Wunsch in Jellyseerr ab (ab Stufe 4 auch in Slouchcat, F-16).
2. Spätestens beim nächsten Abgleich (alle 5 min) wird der Wunsch `declined`.
3. Anfordernde und Mitwünscher bekommen einen Eingangseintrag «Abgelehnt» (ab Stufe 4; vorher zeigt nur
   «Meine Wünsche» den Zustand). Externe Kanäle nur, wenn das Konto «Wünsche: freigegeben, abgelehnt,
   fehlgeschlagen» gewählt hat (ab Werk aus, F-14).
4. Ein Grund fehlt, weil Jellyseerr keinen kennt.
5. Der Wunsch ist geschlossen (Abschnitt 2). Der Titel gilt wieder als fehlend: Bühne, Suche und Vorschauseite
   zeigen wieder «Film anfordern». Die Vorschauseite nennt den letzten Wunsch mit Datum und Grund («Am
   {{Datum}} abgelehnt»). In «Meine Wünsche» steht «Erneut anfordern». Jellyseerr erlaubt einen neuen Wunsch
   nach einer Ablehnung; der Dialog sagt dann «Wurde am {{Datum}} abgelehnt».

### 4.7 Titel wird verfügbar und Benachrichtigung

1. Radarr oder Sonarr legt die Datei ab; Slouchcat liest sie ein (Webhook, Watcher oder Vollscan).
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
7. Im Spotlight wird aus dem fehlenden Titel ein vorhandener (Fall D in `05-spotlight-integration.md`).

Einzelheiten zu Arten, Ereignissen und Doppelschutz: `06-benachrichtigungen.md`.

### 4.8 Wunsch zurückziehen

1. In «Meine Wünsche» hat ein Wunsch im Zustand `queued` oder `pending` den Knopf «Zurückziehen» (F-17;
   Schalter «Zurückziehen erlauben»).
2. Eine Rückfrage bestätigt: «Wunsch zurückziehen?».
3. Slouchcat setzt den Wunsch sofort auf `withdrawn` und merkt sich «in Jellyseerr noch zu löschen». Der
   Wunsch verschwindet aus den offenen Wünschen; der Verlauf bleibt.
4. Ein Job prüft danach in Jellyseerr, ob dort ein Request dieses Benutzers besteht, auch wenn die Übermittlung
   scheinbar nicht geklappt hat. Solange er auf Freigabe wartet, löscht ihn Slouchcat. So bleibt keine
   Bestellung in Jellyseerr zurück, von der Slouchcat nichts weiss.
5. War der Request inzwischen schon freigegeben oder gehört er jemand anderem, bleibt er in Jellyseerr stehen.
   Der Wunsch wird wieder geöffnet («In Beschaffung»), und das Profil liest «War schon freigegeben».
6. Ab `processing` geht Zurückziehen nicht mehr; der Knopf fehlt.
7. **Vorschlag für Mitwünsche:** Ein Mitwünscher zieht nur sich selbst zurück, ohne Aufruf an Jellyseerr.
   Zieht der erste Anforderer zurück, während Mitwünscher da sind, bleibt der Wunsch bestehen; der älteste
   Mitwünscher wird Anforderer. Erst wenn niemand mehr dranhängt, zieht Slouchcat in Jellyseerr zurück.

Technik und Fehlerfälle: `03-architektur.md`, Abschnitt 6.6.

## 5. Zuschauerseiten

| Seite | Route | neu/geändert | Was sich ändert | Mockup |
|---|---|---|---|---|
| Startseite mit Spotlight | `/` | geändert | Bühne und Posterreihe zeigen auch fehlende Titel; neue Reihe «Deine Wünsche», ab Werk nach «Weiterschauen» | M-01 |
| Spotlight-Liste | `/lists/spotlight` | geändert | alle erlaubten Einträge ohne den Deckel N, mit der Medienart des Spotlights; Filter «Alle / In der Bibliothek / Noch nicht da» | M-02 |
| Detailseite Film | `/items/{id}` | fast unverändert | nur vorhandene Filme; neu der Chip «Dein Wunsch ist da» (14 Tage nach der Erfüllung) | M-05 |
| Detailseite Serie | `/items/{id}` | geändert | nur für Profile, die anfordern können: fehlende Staffeln als gedämpfte Karten, «Staffeln anfordern», Zeile «2 von 4 Staffeln vorhanden» | M-06 |
| Vorschauseite | `/titles/{movie\|show}/{tmdbId}` | neu | Aufbau wie Detailseite; Hauptknopf «Film anfordern» / «Serie anfordern» oder Zustand | M-03, M-04 |
| Suche | `/search` | geändert | Abschnitt «Noch nicht in der Bibliothek» unter den Bibliothekstreffern | M-07 |
| Meine Wünsche | `/wishes` | neu | Liste, Filter, Verlauf je Wunsch, Zurückziehen, erneut anfordern | M-08 |
| Benachrichtigungen | `/notifications` | geändert | neue Arten mit Link | M-09 |
| Benachrichtigungskanäle | `/settings/notifications` | geändert | neue Ereignisse je Kanal | M-10 |
| Profile | `/settings/profiles` | geändert | Schalter «Darf Titel anfordern» im Profildialog, Chip auf der Karte | M-11 |
| Profilwahl | `/profiles` | unverändert | im Mockup bedienbar, um Profile zu wechseln | M-12 |
| Dialog «Anfordern» | von M-01 bis M-07 | neu | Film, Serie mit Staffelwahl, Mitwunsch, Sonderfälle | M-13 |

**Seitenleiste:** neuer Eintrag «Wünsche» nach «Gesehen» mit Zähler offener Wünsche; nur wenn das Profil
anfordern darf oder Wünsche hat.

### 5.1 Vorschauseite

- Kopf wie die Detailseite: Hintergrund, Logo oder Titel, Metazeile (Jahr, Laufzeit oder Staffelzahl,
  Genres, Freigabe) mit dem Chip «Noch nicht in der Bibliothek», Beschreibung.
- Knöpfe: Hauptknopf «Film anfordern» / «Serie anfordern» oder die Zustandsanzeige; «Trailer», wenn einer da ist.
- Nach einem geschlossenen Wunsch (abgelehnt, fehlgeschlagen, zurückgezogen) steht wieder «Film anfordern»,
  darunter der letzte Wunsch mit Datum und Grund (Abschnitt 2).
- Erreichbar ist die Seite nur für Profile, die den Titel anfordern können, und nur, solange mindestens eine
  der drei Stellen («Anfordern aus dem Spotlight», «… aus der Suche», «… auf Detailseiten») an ist. Sonst
  antwortet sie 404. Ausnahme: Das Profil hat selbst einen Wunsch zu diesem Titel (Abschnitt 3).
- Bei Serien die Staffelliste laut TMDB mit Folgenzahl; laufende Serie mit «Läuft noch, nächste Folge am …».
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

### 5.3 Meine Wünsche

- Liste der Wünsche des Profils, neueste zuerst; Filter nach Zustand (Vorschlag: «Offen», «Verfügbar»,
  «Abgeschlossen»).
- Je Wunsch: Poster, Titel, bei Serien die Staffeln, Zustand mit Datum, aufklappbarer Verlauf (angefordert,
  übermittelt, freigegeben, verfügbar).
- Aktionen je Zustand: «Zurückziehen», «Erneut anfordern», «Abspielen» bzw. «Zum Titel».
- Wünsche aus Jellyseerr, die Slouchcat übernimmt (F-21, Stufe 4), tragen «In Jellyseerr angelegt».
- Das Hauptprofil sieht über `listAccountWishes` (`scoped`, nur mit Hauptprofil) auch die Wünsche aller
  Profile des Kontos. Das ist gewollt: Das Hauptprofil verwaltet das Konto, setzt und entfernt PINs und sieht
  Schauzeit und Einstellungen aller Profile. Die PIN-Regel F-15 schützt etwas anderes, nämlich externe
  Nachrichten, die jedes Gerät des Kontos erreichen (`06-benachrichtigungen.md`, Abschnitt 8).
- Offline zeigt die Seite «Nur online verfügbar» (Wünsche haben keinen Offline-Zustand).

## 6. Serien und Staffeln

| Fall | Erkennung | Was das Profil sieht |
|---|---|---|
| Serie vollständig vorhanden | alle Staffeln laut `series_catalog` in der Bibliothek | Detailseite wie heute; kein Knopf |
| Serie fehlt ganz | keine Bibliothekszeile | Vorschauseite (M-04), «Serie anfordern», Staffelwahl |
| Serie teilweise vorhanden | einige Staffeln vorhanden, andere laut TMDB nicht | Detailseite (M-06), nur für Profile, die anfordern können: Zeile «2 von 4 Staffeln vorhanden»; fehlende Staffeln als gedämpfte Karten in der Staffelwand; Aktion «Staffeln anfordern». Profile ohne Wunschrecht sehen die Serie wie heute, ohne fehlende Staffeln und ohne diese Zeile |
| Einzelne Staffeln fehlen mitten drin | Lücke in der Staffelliste | wie teilweise vorhanden; Staffelwahl bietet genau die Lücken |
| Staffel schon angefordert | offener Wunsch mit dieser Staffel | gedämpfte Karte mit Zustand; im Dialog wählbar mit «Bereits angefordert»; das Profil wird für diese Staffel Mitwünscher |
| Staffel in Beschaffung | Wunschstaffel im Zustand `processing` | gedämpfte Karte «In Beschaffung» |
| Staffel teilweise eingelesen | einige Folgen der Staffel da | Staffel wie heute schaubar; sie gilt für den Wunsch erst als verfügbar, wenn sie vollständig ist (Abschnitt 4.7); **Vorschlag:** die Karte zeigt «5 von 10 Folgen» |
| Künftige Folgen | TMDB-Status «läuft», nächste Folge mit Datum | Zeile «Läuft noch, nächste Folge am …»; nichts anzufordern |
| Künftige Staffel | angekündigt, aber ohne Folgen | nicht wählbar; **Vorschlag:** Karte «Angekündigt» ohne Knopf |
| Staffel 0 (Specials) | Staffelnummer 0 | nie wählbar |

Einzelne Folgen lassen sich nicht anfordern (Jellyseerr arbeitet staffelweise, F-12). Slouchcat schickt die
TVDB-ID immer mit; ohne sie löscht Jellyseerr den Wunsch beim Senden an Sonarr. Serien, die ganz fehlen,
kennt die Vorschauseite aus `external_titles`; Serien in der Bibliothek ihre Staffelliste aus `series_catalog`.
Die Staffelwand für fehlende Staffeln kommt mit Stufe 4.

## 7. Wunschrecht und Jugendschutz aus Sicht des Benutzers

- **Was ein Profil nicht sehen darf, existiert für es nicht.** Ein fehlender Titel über der Altersstufe
  erscheint weder im Spotlight noch in der Suche, und seine Vorschauseite antwortet «nicht gefunden» (404).
  Er erscheint nicht als «gesperrt». Dasselbe gilt für jedes Profil, das nicht anfordern kann (Abschnitt 3).
- Die Freigabe stammt aus TMDB, gelesen in der Länderreihenfolge nach ADR 0034: gewähltes Freigabeland, dann
  DE, AT, CH, GB, US; das erste Land mit bekanntem Wert gilt (dieselbe Funktion wie für Bibliothekstitel);
  innerhalb eines Lands gilt der strengste Wert über die Anbieter. Verglichen wird mit der tieferen Grenze von
  Profil und Konto. **Unbekannte Freigabe zählt als 18.**
- Ein Kinderprofil mit Wunschrecht darf anfordern (F-6). Ob der Wunsch eine Freigabe braucht, entscheidet die
  Auto-Freigabe des zugeordneten Jellyseerr-Benutzers. Slouchcat baut keine eigene Freigabe.
- Beispiel aus den Mockup-Daten: Noah (12) ist dem Jellyseerr-Benutzer «kinder» zugeordnet (keine
  Auto-Freigabe, Kontingent 5 Filme je 7 Tage). Sein Wunsch zeigt «Freigabe ausstehend», bis Marco ihn in
  Jellyseerr freigibt. Mia (6) darf nicht anfordern und sieht keine fehlenden Titel.
- Die Benachrichtigung «verfügbar» kommt für ein Profil erst, wenn der Titel für **dieses** Profil sichtbar
  ist. Ein Erwachsener mit einem Mitwunsch kann also früher Bescheid bekommen als ein Kind.
- Zuschauer sehen nie, wer sonst gewünscht hat. Die Verwaltung sieht alle Anforderer.

## 8. Was bewusst NICHT gebaut wird

| Nicht gebaut | Warum |
|---|---|
| eigene Beschaffung, Ansteuern von Radarr, Sonarr oder SABnzbd | bleibt Non-Goal; Slouchcat spricht nur Jellyseerr an |
| Wahl von Qualität, 4K, Qualitätsprofil, Zielordner, Tags | gehört Jellyseerr und Radarr/Sonarr |
| einzelne Folgen anfordern | Jellyseerr arbeitet staffelweise |
| eigener Freigabe-Ablauf in Slouchcat (Kind fragt Eltern) | bleibt Non-Goal; die Freigabe liegt in Jellyseerr. Durchreichen von «Freigeben/Ablehnen» für Administratoren (F-16) ist kein eigener Ablauf |
| Seite «Entdecken» | Suche, Spotlight-Liste und Vorschauseite decken es ab |
| Wünsche offline | `/api` ist NetworkOnly; kein Offline-Zustand |
| Ablehnungsgrund | Jellyseerr kennt keinen |
| Anzeige, wer sonst gewünscht hat | Datenschutz zwischen Profilen |
| Merken, Gesehen, Empfehlen für fehlende Titel | hängen an Bibliothekstiteln |
| Wunschliste aus der Trakt-Merkliste, Trakt-Login | Non-Goal (Trakt nur öffentliche Listen) |
