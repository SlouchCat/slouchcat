# Konzept 25 · Verwaltung

> **Kurz gesagt.** Die Verwaltung bekommt zwei neue Orte: die Kachel **Integrationen › Jellyseerr** für die
> Verbindung und die Seite **Medien › Wünsche** mit den Reitern «Anfragen», «Zuordnung» und «Einstellungen»
> (F-19). Sieben bestehende Seiten bekommen einzelne Felder dazu: Übersicht, Integrationen, Konten, Spotlight,
> Startseite, Benachrichtigungen und Hintergrundjobs. Jede Einstellung hat genau einen Ort; andere Seiten zeigen
> nur ihren Wert und verlinken dorthin. Kein heutiges Feld und keine heutige Aktion fällt weg (ADR 0035); die
> Abschnitte unten führen den heutigen Inhalt je Seite vollständig auf.

Grundlage ist [Anhang F](anhang/F-entscheid-blatt.md) (Abschnitt 4, 2.18, 2.19) und das Inventar in
[Anhang A](anhang/A-spotlight-startseite.md) (Abschnitte 3 und 5.2), [Anhang B](anhang/B-konten-benachrichtigungen.md)
(Abschnitte 3 und 7) und [Anhang C](anhang/C-integrationen-jobs-regeln.md) (Abschnitte 2, 5 und 7). Texte von
heute stammen aus `web/src/locales/de.json`. Mockup-Kennungen M-20 bis M-30 nach Anhang F 12; die Routen im
Mockup sind `#/admin/integrations/jellyseerr`, `#/admin/wishes`, `#/admin/wishes/mapping` und
`#/admin/wishes/settings`. Wo dieses Papier und das Mockup sich unterschieden, gilt das Mockup; dieses Papier
ist daran angeglichen (Redaktionsentscheid vom 17. November 2026). Abschnitte mit
**Vorschlag** gehen über Anhang F hinaus. Spotlight im Einzelnen: `05-spotlight-integration.md`,
Abschnitt 8; Zuordnungsregeln: `04-benutzer-profilmapping.md`; Benachrichtigungen:
`06-benachrichtigungen.md`; Jobs und Fehlerbilder: `03-architektur.md`, Abschnitte 13 und 14.

## 1. Grundsätze

### 1.1 Jede Einstellung genau ein Ort

| Einstellung | Einziger Ort | Wird angezeigt und verlinkt auf |
|---|---|---|
| Adresse, Schlüssel, «Jellyseerr verwenden» | Integrationen › Jellyseerr | Übersicht (Zustand), Wünsche › Einstellungen (Zustand) |
| Takt des Abgleichs | Hintergrundjobs, Zeitplan von `wishes.sync` | Integrationen › Jellyseerr, Band «Abgleich» («alle 5 Minuten · Zeitplan ändern») |
| Zuordnung Konto und Profil | Wünsche › Zuordnung | Konten (Zeile «Jellyseerr-Benutzer»), Integrationen › Jellyseerr («Zuordnung: n von m Konten») |
| Wunschrecht des Kontos | Konten, Formular «Zugriff», bei Administratorkonten Block «Wünsche» (und Dialog «Konto einladen» für das neue Konto) | Wünsche › Zuordnung (Chip) |
| Wunschrecht des Profils | Profildialog (Hauptprofil des Kontos) | Konten (Chips der Profile) |
| Funktionsschalter der Wünsche | Wünsche › Einstellungen | – |
| Fehlende Titel im Spotlight | Startseite › Spotlight, Band «Fehlende Titel» | Wünsche › Einstellungen («Fehlende Titel im Spotlight: Aus · Zur Seite Spotlight») |
| Reihe «Deine Wünsche» (Platz, Ausblenden) | Startseite › Startseite | – |
| Kanal-Ereignisse | Benachrichtigungen (Kanalseite des Kontos) | – |

### 1.2 Einordnung in die sechs Bereiche

- **Verbindung unter Automatisierung › Integrationen.** Dort stehen heute alle Dienste ausserhalb von
  Slouchcat mit Schlüsseln, Prüfen und Zustand (Schlüssel der Anbieter, Webhooks, E-Mail, Telegram, Web-Push,
  Server). Jellyseerr ist ein weiterer solcher Dienst. Einrichten macht man einmal.
- **«Wünsche» unter Medien.** Wünsche sind Arbeit an der Bibliothek, wie Pflege, Podcasts und Radio: Man schaut
  regelmässig hinein, gibt frei (Stufe 4), sieht Fehler. Die Zuordnung steht dabei, weil sie nur für Wünsche gilt.
- **Verworfen:**

| Alternative | Warum nicht |
|---|---|
| eigener siebter Bereich «Wünsche» | ADR 0035 legt sechs Bereiche fest; ein Bereich für eine Seite sprengt die Leiste |
| alles unter Integrationen | mischt die tägliche Liste der Anfragen mit dem einmaligen Einrichten; die Ansichten dort sind für Dienste gedacht, nicht für Listen von Titeln |
| Zuordnung unter Nutzer & Geräte › Konten | die Zuordnung braucht die Liste der Jellyseerr-Benutzer und ihre Rechte; auf «Konten» stehen nur Wert und Link |
| Anfragen unter Automatisierung | es geht um Titel, nicht um Abläufe; Jobs bleiben unter Hintergrundjobs |

Der Reiter «Wünsche» steht im Bereich Medien direkt nach «Pflege» (wie im Mockup) und trägt den Zähler
«wartet auf Freigabe» (**Vorschlag**, Muster: Zahlen an Bereichsreitern, ADR 0035). Die Reihenfolge im Bereich
Medien wird damit: Bibliotheken, Metadaten, Pflege, Wünsche, Podcasts, Radio, Hörbuch-Quellen.

### 1.3 Gestaltungsregeln von Marco (16. November 2026)

- Alles Klappbare startet **zugeklappt** und zeigt seine Zusammenfassung im Kopf (Frage 32, P-3). Ausnahmen wie
  dort: Sprunglink, eben Angelegtes, Fehler oder aktuelle Wahl darin.
- **Anlegen und Ändern im selben Formular** (Frage 32, P-2). Für die Zuordnung heisst das: Erstes Zuordnen und
  Ändern geschehen am selben Ort, direkt in der Liste der Seite Zuordnung (Auswahlfeld je Konto und je Profil,
  sofort wirksam), nicht in einem eigenen Dialog.
- **Sichtbare Rahmen, hoher Kontrast:** Rahmen und Bedienelemente mindestens 3 : 1, Text mindestens 7 : 1,
  Gesperrtes an der Form statt an Blässe (Frage 32, P-4).

## 2. Übersicht (M-20)

**Heute** (ADR 0035, Zeile «Übersicht»; Texte `admin.home.*`):

- Hinweisband «Einrichtung nicht abgeschlossen» (wenn offen), Titel «Alles im Blick», Knopf «Aktualisieren».
- Vier Kennzahlen als Verweise: «Aktive Streams», «Laufende Jobs» («n wartend · n pausiert»), «Titel zur
  Pflege», «Speicherplatz» («Belegt: Konfiguration · Cache»).
- «Was benötigt Aufmerksamkeit?» mit allen bisherigen Hinweisen, dazu Integrationsfehler und stille Webhooks;
  «Alles in Ordnung.» wenn leer; je Punkt «Ansehen».
- «Gerade aktiv», «Direkt zu deinen Aufgaben» (Schnellzugriffe), «Server & letzte Aktivitäten» mit «Serverdetails»
  und «Server-Protokoll ansehen». Abfrage alle 5 s.

**Neu:**

| Element | Inhalt | Wirkung |
|---|---|---|
| Kennzahl «Offene Wünsche» | Zahl aller Wünsche in `queued`, `pending`, `processing`, `partial`; Unterzeile «n warten auf Freigabe» | Verweis auf Wünsche › Anfragen. **Vorschlag:** als fünfte Kachel; nur sichtbar, wenn Jellyseerr eingerichtet ist |
| Hinweis «n Wünsche warten auf Freigabe» | Wünsche `pending` | Link auf Anfragen, Filter «Freigabe ausstehend» |
| Hinweis «Jellyseerr nicht erreichbar» | Zustand Nicht erreichbar oder Schlüssel ungültig | Link auf Integrationen › Jellyseerr; Text nennt den Zustand |
| Hinweis «n Konten ohne Zuordnung» | Konten mit Wunschrecht ohne Jellyseerr-Benutzer (und ohne Sammelbenutzer). Ihre Profile sehen keine fehlenden Titel; nur hier und auf den Seiten Zuordnung und Konten fällt der Fall auf | Link auf Wünsche › Zuordnung |
| Hinweis «Andere Jellyseerr-Instanz erkannt» | Fingerabdruck der Instanz weicht ab (`03-architektur.md`, Abschnitt 7.4); Übermittlung und Abgleich halten an | Link auf Integrationen › Jellyseerr, dort «Zuordnung bestätigen» oder «Zuordnung löschen» |
| **Vorschlag:** Hinweis «n Wünsche konnten nicht übermittelt werden» | Wünsche `failed` mit Grund Übermittlung | Link auf Anfragen, Filter «Fehler» |
| **Vorschlag:** Hinweis «Benutzer in Jellyseerr gelöscht» | zugeordnete Benutzer mit `gone_at` | Link auf Zuordnung |

Fehlerbild: Liefert `getSeerrStatus` keine Antwort, bleibt die Kennzahl mit «–» stehen, die übrige Übersicht lädt
wie heute.

## 3. Integrationen (M-21)

**Heute** (Anhang C 2.1 bis 2.7):

- Titel «Integrationen», Einleitung zu Schlüsseln, Benachrichtigungen und Update-Prüfung; nur Administrator mit
  Hauptprofil; Ladezustand und «Erneut versuchen».
- Zeile «{{count}} von {{total}} eingerichtet» und **sechs Kacheln** in dieser Reihenfolge: Schlüssel der
  Anbieter, Webhooks von Radarr und Sonarr, E-Mail-Server, Telegram-Bot, Web-Push, Server. Jede Kachel: Symbol,
  Titel als Link, Beschreibung, Meta, Chip «ungespeichert», Zustand mit Punkt.
- Ansichten mit `?item=`: **keys** (sechs Anbieterkarten TMDB, TVDB, Fanart.tv, YouTube, Trakt, Google Books je
  mit Feldern, «Speichern», «Prüfen», «Löschen …», `.env`-Sperre), **webhooks** («Neue Integration» mit Name und
  Dienst, einmal gezeigte Adresse, Tabelle mit Name, Dienst, Adresse, Letzte Meldung, «Token erneuern»,
  «Löschen», Pfad-Umsetzung, Meldungsprotokoll), **email** (Server und Port, Benutzer, Passwort, Absender,
  Verschlüsselung, «Testmail senden», «Verwerfen», «Löschen …», «Speichern»), **telegram** (Bot, Bot-Token),
  **push** (Schlüsselpaar, Öffentlicher Schlüssel, «Schlüssel neu erzeugen»), **server** («Täglich nach neuen
  Versionen suchen», «Öffentliche Adresse», «Grafikchip für das Umwandeln»).
- Ansichten bleiben gemountet; Rückfrage bei ungespeicherten Eingaben.

**Neu:** eine **siebte Kachel «Jellyseerr»** (`?item=jellyseerr`, im Mockup `#/admin/integrations/jellyseerr`).
Sie steht nach «Webhooks von Radarr und Sonarr» (wie im Mockup). Die Zeile oben («n von m eingerichtet») zählt
dann neun Statusschlüssel: TMDB, TVDB, YouTube, Trakt, E-Mail, Telegram, Web-Push, Update-Prüfung und
Jellyseerr.

| Teil der Kachel | Inhalt |
|---|---|
| Beschreibung | «Wünsche der Zuschauer an Jellyseerr weitergeben und ihren Zustand abgleichen.» (Wortlaut Vorschlag) |
| Meta | «Zuordnung: 3 von 3 Konten» · «5 offene Wünsche» |
| Zustand | Punkt und «Jellyseerr: Verbunden», «Nicht eingerichtet», «Schlüssel ungültig», «Nicht erreichbar», «Andere Instanz erkannt» oder «Ausgeschaltet» |

Die bestehenden sechs Ansichten bleiben unverändert.

## 4. Integrationen › Jellyseerr (M-22)

Neue Ansicht, Verweis «‹ Alle Integrationen». Vier Bänder, alle zugeklappt mit Zusammenfassung im Kopf.

### 4.1 Band «Verbindung»

Kopf: «Verbindung» · «Verbunden · Jellyseerr 2.7.3» bzw. der Zustand.

| Feld | Beschriftung und Hilfe | Werte | Vorgabe | Wirkung |
|---|---|---|---|---|
| Schalter | «Jellyseerr verwenden» · «Aus: Slouchcat ruft Jellyseerr nicht auf. Wünsche bleiben gespeichert.» | an/aus | aus | aus stoppt Übermittlung und Abgleich; Zustand «Ausgeschaltet» |
| Adresse | «Adresse» · «Zum Beispiel http://192.168.1.50:5055. Slouchcat spricht nur diese Adresse an.» | `http`/`https`, ohne Benutzerangaben, ohne `/api/v1` | leer | Ziel aller Aufrufe; LAN erlaubt, keine Weiterleitungen |
| API-Schlüssel | «API-Schlüssel» · «Den Schlüssel zeigt Jellyseerr in seinen Einstellungen. Leer lassen, um den gespeicherten zu behalten.» | geheim | leer | verschlüsselt gespeichert, nie wieder angezeigt; Platzhalter «Gesetzt, endet auf …a1b2». Hat die eingegebene Adresse einen anderen Host als die gespeicherte, muss der Schlüssel neu eingegeben werden; der gespeicherte geht nie an eine neue Adresse |
| Knopf | «Verbindung testen» | – | – | prüft die eingegebene oder gespeicherte Adresse, zeigt die Prüfliste; bei `http` zu einer Adresse ausserhalb des Heimnetzes die Warnung «Schlüssel ginge unverschlüsselt ins Internet» |
| Knöpfe | «Speichern», «Verwerfen» | – | – | Speichern löst `seerr.users` und einen Test aus |

**Vorschlag** wie die anderen Schlüssel (siehe `03-architektur.md`, Abschnitt 4.2): `SLOUCHCAT_SEERR_URL` und
`SLOUCHCAT_SEERR_API_KEY` in `deploy/.env` gewinnen und sperren die Felder mit «In deploy/.env gesetzt»; dazu
«Löschen …» mit Rückfrage «Verbindung zu Jellyseerr löschen? Wünsche bleiben gespeichert, werden aber nicht mehr
übermittelt und abgeglichen.»

**Prüfliste des Tests** (Aufrufe wie `03-architektur.md`, Abschnitt 15.1):

| Prüfpunkt | Ergebnis gut | Ergebnis schlecht |
|---|---|---|
| erreichbar | «Erreichbar» | «Nicht erreichbar: Zeitüberschreitung» / «… Verbindung abgelehnt» / «… Weiterleitung nicht erlaubt» |
| Schlüssel gültig | «Schlüssel gültig» | «Schlüssel ungültig» (403) |
| Version | «Jellyseerr 2.7.3» bzw. «Seerr 3.5.2» | ältere als Jellyseerr 2.7 oder Seerr 3.0: Warnung «Version nicht geprüft: getestet sind Jellyseerr 2.7 und Seerr 3.x» (kein Fehler) |
| Instanz | «Gleiche Instanz wie beim Einrichten» | «Andere Jellyseerr-Instanz erkannt» mit den Knöpfen «Zuordnung bestätigen» und «Zuordnung löschen» (Rückfrage); bis dahin halten Übermittlung und Abgleich an (`04-benutzer-profilmapping.md`, Abschnitt 8.4) |
| Handeln im Namen von Benutzern | «wirkt» (`X-API-User`) | «wirkt nicht: Wünsche würden als Administrator gestellt» (sperrt «Wünsche aktiv», **Vorschlag** T-1) |
| CSRF-Schutz in Jellyseerr aus | «aus» | «an: schreibende Aufrufe scheitern. In den Netzwerk-Einstellungen von Jellyseerr ausschalten.» |
| Radarr und Sonarr in Jellyseerr eingerichtet | «Radarr: 1, Sonarr: 1» | Warnung «Kein Sonarr in Jellyseerr: Serien-Wünsche bleiben liegen.» (kein Fehler) |

**Zustände:**

| Zustand | Bedeutung | Anzeige |
|---|---|---|
| Nicht eingerichtet | keine Adresse oder kein Schlüssel | grauer Punkt; Hinweis «Trage Adresse und Schlüssel ein.» |
| Verbunden | letzter Test oder Abgleich gelang | grüner Punkt, Version |
| Schlüssel ungültig | 401 oder 403 ohne `X-API-User` | roter Punkt; Übersicht meldet es |
| Nicht erreichbar | Netzfehler, Zeitlimit 10 s, 5xx | roter Punkt, Zeit des letzten Erfolgs; Wünsche warten als `queued` |
| Andere Instanz erkannt | Fingerabdruck weicht ab | roter Punkt; Übersicht meldet es; kein Aufruf ausser dem Test |
| Ausgeschaltet | Schalter aus | grauer Punkt «Ausgeschaltet» |

### 4.2 Band «Abgleich»

Kopf: «Abgleich» · «zuletzt vor 3 min · 5 offen».

| Element | Inhalt |
|---|---|
| Letzter erfolgreicher Abgleich | Datum und Zeit; bei Fehlern zusätzlich «Letzter Versuch {{Zeit}}: {{Grund}}» |
| Nächster Lauf | «in 2 min · alle 5 Minuten» und Link «Zeitplan ändern» (Hintergrundjobs, `wishes.sync`) |
| Knopf | «Jetzt abgleichen» (`syncWishesNow`, Priorität 7) |
| Zähler | «Offene Wünsche», «Warten auf Übermittlung», «Nicht zuordenbar», je als Link auf Anfragen mit Filter |
| Link | «Zu den Jobs» |

### 4.3 Band «Sofortmeldung» (Stufe 4, F-18)

| Element | Inhalt |
|---|---|
| Einleitung | «Jellyseerr stösst den Abgleich sofort an, statt bis zu 5 Minuten zu warten. Der Abgleich bleibt die Grundlage.» |
| Knopf | «Adresse erzeugen» bzw. «Token erneuern» (Rückfrage wie bei Radarr/Sonarr) |
| Adresse | einmal gezeigt, «Adresse kopieren», Warnung «Jetzt kopieren, danach ist die Adresse nicht mehr sichtbar.» |
| JSON-Vorlage | zum Kopieren in Jellyseerr (Einstellungen › Benachrichtigungen › Webhook) |
| Hinweis | «Jellyseerr kennt nur ein Webhook-Ziel. Nutzt du es schon für etwas anderes, ersetzt diese Adresse es.» |
| Letzte Meldung | Zeit oder «Hat noch nie etwas gemeldet.» |

### 4.4 Band «Protokoll»

Kopf: «Protokoll» · «20 Einträge · 1 Fehler». Tabelle der letzten 20 Aufrufe: Zeit, Aufruf (Methode und Pfad
ohne Query), Ergebnis (Status), Dauer, Fehler. Nie Header, nie Schlüssel. Eigene Tabelle `seerr_log`
(`08-datenmodell-api.md`, Abschnitt 2.8) mit höchstens 20 Zeilen, gehalten von einem Trigger wie `webhook_events` (`server/migrations/00017_integrations.sql:47-57`); Anhang F nennt
dafür noch keine Tabelle (Abschnitt 5 hat nur `wish_events` je Wunsch).

### 4.5 Verweise

«Zuordnung: n von m Konten» → Medien › Wünsche › Zuordnung. **Vorschlag:** Hinweis, wenn die öffentliche
Adresse (Integrationen › Server) fehlt: «Ohne öffentliche Adresse fehlen Links in E-Mails.» Hinweis, dass
Jellyseerr eigene Benachrichtigungen weiter verschickt (`06-benachrichtigungen.md`, Abschnitt 8).

### 4.6 Lücke: zwei Schalter

«Jellyseerr verwenden» (Verbindung) und «Wünsche aktiv» (Funktion, M-25) wirken beide auf das Anfordern. Ein Ort
je Einstellung bleibt gewahrt, weil sie Verschiedenes schalten. Das Zusammenspiel (Redaktionsentscheid vom 17. November 2026, Lücke in Anhang F):

| Jellyseerr verwenden | Wünsche aktiv | Zuschauer | Hintergrund |
|---|---|---|---|
| aus | egal | kein Anfordern, keine fehlenden Titel; «Meine Wünsche» zeigt bestehende Wünsche nur zum Lesen | kein Aufruf; die 24-h-Frist der Wünsche `queued` ruht (**Vorschlag**, T-2) |
| an | aus | wie oben | Abgleich läuft weiter für bestehende Wünsche; `queued`-Wünsche werden weiter übermittelt |
| an | an | Anfordern nach Recht; fehlende Titel nach der Regel von F-9 | alles läuft |

Vorgabe: beide aus. **Vorschlag** (T-1): «Wünsche aktiv» lässt sich nur einschalten, wenn der Test zuletzt
gelang und «Handeln im Namen von Benutzern» wirkt. Anhang F verlangt nur, dass der Test meldet, wenn es nicht
wirkt.

## 5. Medien › Wünsche › Anfragen (M-23)

Neue Seite `/admin/wishes`. Kopf: «MEDIEN / WÜNSCHE», Titel «Wünsche», Einleitung «Wünsche der Profile und ihr
Zustand in Jellyseerr. Freigeben und Ablehnen geschieht in Jellyseerr (ab Stufe 4 auch hier).» Reiter
«Anfragen», «Zuordnung», «Einstellungen».

| Teil | Inhalt |
|---|---|
| Kennzahlen | «Offen», «Freigabe ausstehend», «In Beschaffung», «Verfügbar (30 Tage)», «Fehler»; je ein Klick setzt den Filter |
| Filter | Zustand (alle Zustände von Anhang F), «Nicht zuordenbar», Medienart; **Vorschlag:** Suche nach Titel |
| Tabelle | «Titel» (Poster über die Admin-Bildroute bzw. `getAdminItemImage`, Titel, Jahr), «Art» (Film, Serie mit Staffeln «S1–S3»), «Gewünscht von» («Konto · Profil» und «+2 Mitwünscher»), «Jellyseerr-Benutzer», «Zustand» (Chip), «Seit»; Kennzeichnung «In Jellyseerr angelegt» bei `origin` = `jellyseerr` |
| Aufklappzeile | **Verlauf** aus `wish_events` (angelegt, übermittelt, freigegeben, eingelesen, gemeldet) und **Diagnose**: Request-ID und Media-ID in Jellyseerr, Rohstatus, Versuche, nächster Versuch, letzter Fehler, zugeordneter Titel, Sichtbarkeit je Anforderer («für Noah nicht sichtbar»), alle Mitwünscher |
| Aktionen je Zeile | «In Jellyseerr öffnen» (neues Fenster), «Zum Titel» (wenn in der Bibliothek), «Erneut übermitteln» (`queued`, `failed`), «Freigeben» und «Ablehnen» (nur `pending`, Stufe 4, F-16) |
| Leer | «Noch keine Wünsche.» bzw. bei ausgeschalteter Funktion «Wünsche sind ausgeschaltet.» mit Link auf «Einstellungen» |

- «Ablehnen» fragt nach: «{{Titel}} ablehnen? Jellyseerr kennt keinen Grund; die Profile sehen nur «Abgelehnt».»
- **Vorschlag:** «Erneut übermitteln» bei einem Request im Zustand FAILED in Jellyseerr (`arr_failed`) ruft dort
  `POST /request/{id}/retry`; das ist für diesen Fall der einzige Weg, weil der Zuschauer nicht erneut anfordern
  kann (`03-architektur.md`, Abschnitt 8.1). Bei einem lokal gescheiterten Wunsch reiht es `wishes.submit` neu ein
  und löst die Zuordnung neu auf (`04-benutzer-profilmapping.md`, Abschnitt 8.1).
- Wieder geöffnete Wünsche (Zurückziehen kam zu spät, `03-architektur.md`, Abschnitt 6.6) tragen im Verlauf
  «Zurückziehen nicht möglich: war schon freigegeben».
- Nicht zuordenbare Anfragen haben nur «In Jellyseerr öffnen» und «Zuordnung einrichten» (Link auf «Zuordnung»
  mit vorgewähltem Benutzer).
- Fehlerbilder: Jellyseerr nicht erreichbar → Band oben «Zustand von {{Zeit}}, Jellyseerr nicht erreichbar»,
  Freigeben/Ablehnen gesperrt mit Grund; 409 beim Freigeben («Only pending requests …») → «Schon entschieden,
  der Abgleich holt den neuen Zustand.»

## 6. Medien › Wünsche › Zuordnung (M-24)

| Teil | Inhalt |
|---|---|
| Kopfzeile | «3 von 3 Konten zugeordnet · 1 Vorschlag · 1 Benutzer gelöscht»; Knöpfe «Benutzerliste neu laden» (`refreshSeerrUsers`, Zeit des letzten Ladens), «Alle Vorschläge übernehmen» und ab Stufe 4 «Anfragen übernehmen» (`wishes.import` für alle zugeordneten Benutzer) |
| Band «Konten» | Tabelle je Konto: «Konto» (E-Mail, Chip «Besitzer»), «Wünsche» (Chip «Erlaubt» / «Nicht erlaubt» / «Zuordnung fehlt»), «Jellyseerr-Benutzer» (**Auswahlfeld**, «Keine Zuordnung» oder ein Benutzer), daneben die Fakten des gewählten Benutzers, «Vorschlag» (Chip «Vorschlag: grosseltern» mit «Übernehmen») |
| Aufklappzeile je Konto | die Profile mit je einem **Auswahlfeld** «Wie Konto» / eigener Benutzer, daneben das Wunschrecht des Profils (nur lesen, setzt das Hauptprofil) |
| Band «Jellyseerr-Benutzer» | alle Benutzer aus `seerr_users`: Name, E-Mail, Art, «Gehört zu» (Konto, Profil oder «Nicht zugeordnet»), Fakten, Zähler «n nicht zuordenbare Anfragen» |

Die Zuordnung ändert man direkt in der Liste, wie im Mockup und in Anhang F 4. Jede Auswahl wirkt sofort
(`setAccountWishMapping`, `setProfileWishMapping`) und bestätigt sich mit der Meldung «Zuordnung geändert».
Einen eigenen Dialog gibt es nicht. Erstes Zuordnen und Ändern geschehen so am selben Ort (Regel von Marco,
Abschnitt 1.3).

**Auswahl:** belegte Benutzer sind gesperrt mit «gehört zu {{Konto}}» (Form statt Blässe: Schloss und Text).
**Fakten je Benutzer:** «darf Filme anfordern», «darf Serien anfordern», «Auto-Freigabe Filme/Serien», «Kontingent
5 Filme je 7 Tage» oder «ohne Kontingent».

**Warnungen:** «Benutzer in Jellyseerr gelöscht» (rot), «darf in Jellyseerr nicht anfordern» bzw. «darf keine
Serien anfordern» (gelb), «Auto-Freigabe für ein Kinderprofil» (**Vorschlag**, gelb, wenn ein Profil unter 18 einen
Benutzer mit Auto-Freigabe nutzt).

**Meldungen und Bestätigung:** Eine Änderung mit Folgen nennt sie in der Meldung: «Zuordnung geändert. 2 offene
Wünsche bleiben bei kinder.» und ab Stufe 4 «3 Anfragen von grosseltern werden übernommen (Profil Oma & Opa).»
Nur «Alle Vorschläge übernehmen» fragt vorher nach und listet alle Paare.

Fehlerbilder: Jellyseerr nicht erreichbar → letzte geladene Liste mit «Stand {{Zeit}}»; belegter Benutzer beim
Ändern (gleichzeitige Änderung) → 409 `seerr_user_taken` mit dem Konto, das Auswahlfeld springt zurück; andere
Instanz erkannt → Band oben mit «Zuordnung bestätigen» und «Zuordnung löschen».

## 7. Medien › Wünsche › Einstellungen (M-25)

Schalter und Auswahlfelder, die **sofort** gelten (wie im Mockup); es gibt kein «Speichern» und kein
«Verwerfen». Jede Änderung bestätigt sich mit einer kurzen Meldung. Bänder zugeklappt mit Zusammenfassung.

| Feld | Hilfe | Werte | Vorgabe | Wirkung |
|---|---|---|---|---|
| «Wünsche aktiv» | «Profile mit Recht können fehlende Titel anfordern.» | an/aus | aus (Redaktionsentscheid; das Mockup zeigt es zur Vorführung an) | Hauptschalter der Funktion; einschaltbar nur mit gelungenem Test (**Vorschlag**, T-1) |
| «Filme erlauben» | «Gilt für alle Profile.» | an/aus | an | aus: kein «Film anfordern» |
| «Serien erlauben» | «Gilt für alle Profile.» | an/aus | an | aus: kein «Serie anfordern», keine «Staffeln anfordern» |
| «Anfordern aus dem Spotlight» | «Fehlende Titel auf der Bühne, in der Posterreihe und in der Spotlight-Liste.» | an/aus | an | aus: keine fehlenden Titel im Spotlight (Startseite und Spotlight-Liste); das Band «Fehlende Titel» der Seite Spotlight zeigt den Hinweis |
| «Anfordern aus der Suche» | «Abschnitt «Noch nicht in der Bibliothek».» | an/aus | an | aus: kein Abschnitt «Noch nicht in der Bibliothek» |
| «Anfordern auf Detailseiten» | «Fehlende Staffeln auf der Seite einer Serie.» | an/aus | an | aus: keine fehlenden Staffeln, keine Zeile «2 von 4 Staffeln vorhanden», kein «Staffeln anfordern». Die Vorschauseite eines fehlenden Titels bleibt erreichbar, solange mindestens eine der drei Stellen an ist |
| «Bereits angeforderte Titel» | «Was ein zweites Profil bei einem schon gewünschten Titel tun kann.» | «Mitwünschen» / «Nur Zustand zeigen» | Mitwünschen (F-7) | Knopf «Benachrichtige mich» oder nur der Zustand |
| «Konten ohne Zuordnung» | «Sammelbenutzer: Alle Konten ohne eigenen Benutzer teilen Kontingent und Auto-Freigabe.» | «Nicht erlauben» / «Sammelbenutzer» + Auswahl | Nicht erlauben (F-4) | Auflösung nach `04-benutzer-profilmapping.md`, Abschnitt 6.1 |
| «Neue Konten dürfen anfordern» | «Vorgabe im Dialog «Konto einladen».» | an/aus | aus (F-5) | Vorgabe des Schalters beim Einladen; ändert bestehende Konten nicht |
| «Zurückziehen erlauben» | «Solange ein Wunsch nicht freigegeben ist.» | an/aus | an (F-17) | Knopf «Zurückziehen» in «Meine Wünsche» |
| «Externe Nachrichten bei PIN-Profilen ohne Titel» | «E-Mail, Push und Telegram nennen dann nur das Profil.» | an/aus | an (F-15) | PIN-Fassung der Texte |
| Verweis | «Fehlende Titel im Spotlight: Aus · Zur Seite Spotlight» | – | – | nur Anzeige |
| Verweis | **Vorschlag:** «Verbindung: Verbunden · Zu Integrationen» | – | – | nur Anzeige |

## 8. Nutzer & Geräte › Konten (M-26)

**Heute** (Anhang B 3.1, alles bleibt):

- Zugang nur Administrator mit Hauptprofil; Kopf «Konten», Hilfetext, Knopf «Konto einladen».
- Zusammenfassung: «Konten», «Administratoren» («davon N Besitzer»), «Profile» («N eigenes»), «Offene
  Einladungen» («Nächste gültig bis …»).
- Reiter «Konten» mit Tabelle: «E-Mail» (klappt auf), «Rolle» (mit «Besitzer»), «Altersstufe», «Bibliotheken»,
  «Profile» («{{name}} (Hauptprofil)»), «Zuletzt gesehen», «Kanäle», «Aktionen». Reiter «Einladungen» mit
  «Notiz», «Zustand», «Altersstufe», «Bibliotheken», «Gültig bis / angenommen», «Widerrufen».
- Aktionsmenü: «Zum Administrator ernennen» / «Rolle entziehen» (nur Besitzer, mit Rückfragen), «Konto entfernen».
- Eigenes Konto: «Konto ändern» (E-Mail-Adresse, Neues Passwort, wiederholen, Aktuelles Passwort; «Verwerfen»,
  «Speichern») und «Meine Profile» (ganze Profilverwaltung: Karten, «Profil hinzufügen», «Bearbeiten», Schauzeit,
  PIN setzen/ändern/entfernen, Löschen; Profildialog mit Name, Altersfreigabe, Sprache, Sprache der
  Beschreibungen, Farbe, Bibliotheken, Wiedergabe, Wiedergabe-Limits mit Schauzeit, PIN beim Anlegen).
- Anderes Konto: Formular «Zugriff» (Höchste Altersfreigabe, Bibliotheken, Wiedergabe-Limits mit Höchste
  Qualität, Bandbreite, Gleichzeitige Streams; «Verwerfen», «Zugriff speichern»), bei Administratoren statt dessen
  der Hinweis «immer ab 18 und alle Bibliotheken»; Block «Profile» nur zum Lesen.
- Dialog «Konto einladen»: «Für wen ist der Link?», «Höchste Altersfreigabe», «Bibliotheken», «Einladung
  erstellen», danach der Einmal-Link. Rückfrage «Einladung widerrufen?». Rückfrage bei ungespeicherten Änderungen.

**Neu:**

| Ort | Element | Werte und Wirkung |
|---|---|---|
| Tabelle «Konten» | Spalte «Wünsche» | Chip «Erlaubt» (Recht und Zuordnung), «Nicht erlaubt» (Recht aus), «Zuordnung fehlt» (Recht an, kein Benutzer) |
| Formular «Zugriff» | Schalter «Darf Titel anfordern» · Hilfe «Gilt als Grenze für alle Profile. Welche Profile anfordern, wählt das Hauptprofil des Kontos.» | an/aus; schreibt `accounts.can_request` über `updateAccountAccess` |
| Formular «Zugriff» | Zeile «Jellyseerr-Benutzer: grosseltern · Zuordnung ändern» | nur Anzeige, Link auf Zuordnung mit diesem Konto |
| Dialog «Konto einladen» | Schalter «Darf Titel anfordern» | Vorgabe aus «Neue Konten dürfen anfordern» |
| Eigenes Konto, «Meine Profile» | Profildialog mit Schalter «Darf Titel anfordern», Chip auf der Karte | wie `/settings/profiles` (M-11) |
| Administratorkonten | Block «Wünsche» im Aufklappfeld mit Schalter «Darf Titel anfordern» und Zeile «Jellyseerr-Benutzer» | Lücke in Anhang F, als Redaktionsentscheid vom 17. November 2026 gesetzt: Administratoren haben kein Formular «Zugriff» (`04-benutzer-profilmapping.md`, Abschnitt 9) |
| «Konto entfernen» | Zusatzsatz «Wünsche bleiben in Jellyseerr bestehen.»; die Wünsche des Kontos werden in Slouchcat geschlossen (`04-benutzer-profilmapping.md`, Abschnitt 10) | – |

## 9. Startseite › Spotlight (M-27)

**Heute** (Anhang A 3, alles bleibt): Kopf «Spotlight» mit Hilfe; Band «Quelle» mit den elf Feldern
(«Auf der Startseite zeigen», «Trakt-Liste», «Adresse der Liste», «Medienart», «Höchstens», «Ausblenden unter»,
«Abruf alle … Stunden», «Spotlight Wechsel Takt», Abrufzustand, «Jetzt abrufen», «Speichern»); Band «Einträge der
Liste» mit «Rang», «Titel», «Status», «Eigene Notiz», «Aktionen» («Anheften»/«Lösen», «Ausblenden»/«Einblenden»,
«Notiz»); Band «Verwaiste Anpassungen» mit «Entfernen»; Fussnote mit Link «Zur Seite «Startseite»». Rückfrage bei
ungespeicherten Änderungen.

**Neu** (Einzelheiten `05-spotlight-integration.md`, Abschnitt 8):

| Element | Inhalt |
|---|---|
| Hilfe im Kopf | **Vorschlag:** «Gezeigt werden Titel aus der eigenen Bibliothek und, wenn eingeschaltet, fehlende Titel zum Anfordern.» |
| «Ausblenden unter» | Hinweis neu: «Fehlende Titel zählen nicht.» |
| Band «Fehlende Titel» | «Fehlende Titel zeigen» (Schalter, aus, F-8); «Höchstens fehlende Titel» (0–10, Vorgabe 5); Reihenfolge «Wie bei Trakt» / «Nach den vorhandenen»; Zustand «7 fehlende Titel dürften erscheinen; 3 sind angefordert»; Hinweis, wenn Wünsche nicht eingerichtet sind oder «Anfordern aus dem Spotlight» aus ist. Gespeichert wird das Band zusammen mit dem Band «Quelle» über dessen «Speichern» |
| Band «Einträge der Liste» | fehlende Einträge mit Poster, Status «Noch nicht in der Bibliothek» oder «Angefordert · In Beschaffung», Aktionen «Anheften», «Ausblenden», «Notiz», «Anfordern» |

**Vorschlag:** «Anfordern» in der Verwaltung fordert als Hauptprofil des Administrators an, über denselben
Dialog wie in der App.

## 10. Startseite › Startseite (M-28)

**Heute** (Anhang A 5.2, alles bleibt): Titel «Startseite» mit Hilfe; Band «Reihen» mit Tabelle «Platz»,
«Reihe», «Zustand», «Ausblenden», Ausklappzeile mit Beschreibung und Link; Grenze 15 sichtbare Reihen mit Meldung;
Band «Vorschau»; Speicherleiste «Speichern»/«Verwerfen»; Fussnote zu Spotlight.

**Neu (F-20):** Reihe «Deine Wünsche» in der Tabelle, mit Platz und Ausblenden. Ausklappzeile: «Wünsche des
Profils, die noch nicht verfügbar oder seit kurzem verfügbar sind. Leer für Profile ohne Wünsche.» und Link «Zur
Seite «Wünsche»». Die Migration setzt die Reihe ab Werk direkt nach «Weiterschauen» (wie im Mockup); sind schon
15 Reihen sichtbar, startet sie ausgeblendet (**Vorschlag**).

## 11. Automatisierung › Benachrichtigungen (M-29)

**Heute** (Anhang B 3.3 und 7.1): Kanalseite des eigenen Kontos in der Verwaltungsleiste, für
Administratoren oben ein Link zu den Kanälen des Servers. Je Kanal (E-Mail, Telegram, Web-Push) ein Band mit
Zustand, Schalter «Aktiv», Ereignissen («Neue Inhalte», «Fehler» nur Administratoren, «Einladungen»,
«Ankündigungen»), E-Mail-Adresse bzw. Telegram-Verknüpfung bzw. angemeldete Geräte, «Speichern»,
«Testnachricht». Ändern nur mit Hauptprofil.

**Neu:** drei Häkchen je Kanal: «Wünsche: verfügbar», «Wünsche: freigegeben, abgelehnt, fehlgeschlagen», nur
Administratoren «Wünsche: Freigabe nötig». Vorgaben nach F-14 (`06-benachrichtigungen.md`, Abschnitt 6). Die
Zusammenfassung im Kopf jedes Kanals zählt sie mit («5 Ereignisse»).

## 12. Automatisierung › Hintergrundjobs (M-30)

**Heute** (Anhang C 5, alles bleibt): Zusammenfassung (Laufend, Wartend, Pausiert, Fehlgeschlagen,
geplant), Arbeiter je Pool, Banner «n Jobs pausiert», «Läuft gerade», Reiter nach Gruppe (Bibliothek, Metadaten,
Zustellung, System, «Fehlschläge») mit Filter, je Jobart Zustand, Letzter und Nächster Lauf, «Jetzt ausführen»,
«Pausieren/Fortsetzen», «Wartende abbrechen», «Alle Läufe abbrechen», Dialoge «Läufe von X», «Zeitplan von X»,
«Zeitfenster von X», Abschnitt «Fehlschläge (alle Arten)».

**Neu:** acht Jobarten. Gruppen, Pools und Prioritäten sind **Vorschlag** aus `03-architektur.md`, Abschnitt 13.

| Jobart | Anzeigename (Vorschlag) | Gruppe | Auslöser | Takt |
|---|---|---|---|---|
| `wishes.submit` | «Wunsch übermitteln» | Zustellung | Ereignis | – |
| `wishes.withdraw` | «Wunsch in Jellyseerr zurückziehen» | Zustellung | Ereignis | – |
| `wishes.sync` | «Wünsche abgleichen» | System | Zeitplan, «Jetzt abgleichen», Webhook; der Erstabgleich läuft als eigener Lauf mit Fortschritt («Seite 12 von 60») | 5 min, einstellbar |
| `wishes.import` | «Anfragen aus Jellyseerr übernehmen» (Stufe 4) | System | Zuordnung gesetzt oder geändert, «Anfragen übernehmen» | – |
| `wishes.reconcile` | «Verfügbare Wünsche prüfen» | System | Zeitplan, nach dem Match | 1 h |
| `seerr.users` | «Jellyseerr-Benutzer laden» | System | Zeitplan, Knopf | 24 h |
| `titles.refresh` | «Fehlende Titel auffrischen» | Metadaten | Zeitplan nach `trakt.refresh` | 24 h |
| `series.catalog` | «Staffelliste der Serien» | Metadaten | Ereignis und Zeitplan | 7 d |

«Zeitplan von X» für `wishes.sync` ist der einzige Ort des Abgleichtakts. **Vorschlag:** Wird `wishes.submit`
pausiert, zeigt der Dialog: «Wünsche bleiben «Wird übermittelt»; nach 24 Stunden gelten sie als fehlgeschlagen.»

## 13. Status und Diagnose

| Frage des Administrators | Ort |
|---|---|
| Ist Jellyseerr verbunden? | Integrationen (Kachel), Integrationen › Jellyseerr Band «Verbindung», Übersicht bei Fehler |
| Ist es noch dieselbe Jellyseerr-Instanz? | Integrationen › Jellyseerr, Prüfpunkt «Instanz»; Übersicht «Andere Jellyseerr-Instanz erkannt» |
| Wann gelang der letzte Abgleich? | Integrationen › Jellyseerr Band «Abgleich»; Hintergrundjobs `wishes.sync` |
| Welche Aufrufe sind gescheitert? | Integrationen › Jellyseerr Band «Protokoll» |
| Welche Konten haben noch keine Zuordnung? | Übersicht «n Konten ohne Zuordnung»; Wünsche › Zuordnung; Konten Spalte «Wünsche» |
| Welche Jellyseerr-Benutzer fehlen oder dürfen nicht anfordern? | Wünsche › Zuordnung, Warnungen |
| Welche Anfragen gehören keinem Profil? | Wünsche › Anfragen, Filter «Nicht zuordenbar»; Zähler im Band «Abgleich» |
| Welche Wünsche warten auf Übermittlung oder sind gescheitert? | Wünsche › Anfragen, Filter «Wird übermittelt» und «Fehler»; Aufklappzeile «Diagnose»; Fehlschläge `wishes.submit` auf Hintergrundjobs |
| Welche Wünsche warten auf Freigabe? | Übersicht; Bereichsreiter «Wünsche» (Zähler); Anfragen, Filter «Freigabe ausstehend» |
| Warum bekam ein Profil keine Meldung? | Anfragen › Diagnose («für Noah nicht sichtbar»); Kanalseite des Kontos; fehlgeschlagene `notify.deliver` auf Hintergrundjobs |
| Warum gilt ein Titel nicht als verfügbar? | Anfragen › Diagnose («In Jellyseerr verfügbar, in Slouchcat nicht gefunden» mit Link zur Pflege, `03-architektur.md`, Abschnitt 11) |

## 14. Offene Entscheide in diesem Papier

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-16 | Freigeben/Ablehnen in der Verwaltung in Stufe 4? | Ja |
| F-18 | Jellyseerr-Webhook als Sofortmeldung in Stufe 4? | Ja |
| F-19 | Seite «Wünsche» unter Medien, Verbindung unter Integrationen? | Ja |
| F-20 | Startreihe «Deine Wünsche»? | Ja |
| F-8 | Band «Fehlende Titel» ab Werk aus, 5 von 20? | Ja |

Begründungen und Folgen anderer Wahl: `11-entscheide-und-adr.md`, Abschnitt 1.
