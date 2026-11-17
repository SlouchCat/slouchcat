# Konzept 25 · Verwaltung

> **Kurz gesagt.** Die Verwaltung bekommt zwei neue Orte: die Kachel **Integrationen › Seerr** für die
> Verbindung und die Seite **Medien › Wünsche** mit den Reitern «Anfragen», «Seerr-Benutzer» und
> «Einstellungen» (F-19). Sieben bestehende Seiten bekommen einzelne Felder dazu: Übersicht, Integrationen, Konten,
> Spotlight, Startseite, Benachrichtigungen und Hintergrundjobs. Neu in Fassung 3: Konten werden in Seerr
> eröffnet oder über die E-Mail sofort verknüpft, auch bei einer Einladung (Marco, E-8); die Übersicht zeigt eine
> solche Verknüpfung sieben Tage lang als «Neu verknüpft». Das Limit je Konto steht unter Konten, die Vorgabe unter Einstellungen,
> und Profil-Tag und Download-Fortschritt aus Radarr und Sonarr haben ein gemeinsames Band. Die Seite Spotlight
> listet die fehlenden Einträge der Trakt-Liste nur für den Administrator; die Startseite zeigt weiter nur Titel der
> Bibliothek. Die Webhooks von Radarr und Sonarr bleiben unverändert. Jede Einstellung hat genau einen Ort; andere
> Seiten zeigen nur ihren Wert und verlinken dorthin. Kein heutiges Feld und keine heutige Aktion fällt weg
> (ADR 0035); die Abschnitte unten führen den heutigen Inhalt je Seite vollständig auf.

Stand: Fassung 3 vom 17. November 2026 mit allen drei Rückmeldungen von Marco an diesem Tag (E-1 bis E-11,
[Anhang H](anhang/H-nachtrag-17-november.md); Abschnitt 10 dort geht den Abschnitten 4 bis 9 vor, Abschnitt 8
den Abschnitten 4 bis 7); was dort
**Redaktion** heisst, ist hier Empfehlung. Grundlage ist [Anhang F](anhang/F-entscheid-blatt.md)
(Abschnitt 4, 2.18, 2.19) und das Inventar in
[Anhang A](anhang/A-spotlight-startseite.md) (Abschnitte 3 und 5.2), [Anhang B](anhang/B-konten-benachrichtigungen.md)
(Abschnitte 3 und 7) und [Anhang C](anhang/C-integrationen-jobs-regeln.md) (Abschnitte 2, 5 und 7). Texte von
heute stammen aus `web/src/locales/de.json`. Mockup-Kennungen M-20 bis M-30 nach Anhang F 12; die Routen im
Mockup sind `#/admin/integrations/jellyseerr`, `#/admin/wishes`, `#/admin/wishes/mapping` (Reiter
«Seerr-Benutzer», vorher «Zuordnung») und `#/admin/wishes/settings`. Papier und Mockup sind angeglichen; bei Abweichung gilt das Papier (Redaktionsentscheid zu K-12). Abschnitte mit
**Vorschlag** gehen über Anhang F hinaus. Spotlight im Einzelnen: `05-spotlight-integration.md`,
Abschnitt 4; Zuordnungsregeln: `04-benutzer-profilmapping.md`; Benachrichtigungen:
`06-benachrichtigungen.md`; Jobs und Fehlerbilder: `03-architektur.md`, Abschnitte 13 und 14.

## 1. Grundsätze

### 1.1 Jede Einstellung genau ein Ort

| Einstellung | Einziger Ort | Wird angezeigt und verlinkt auf |
|---|---|---|
| Adresse, Schlüssel, «Seerr verwenden» | Integrationen › Seerr | Übersicht (Zustand), Wünsche › Einstellungen (Zustand) |
| Takt des Abgleichs | Hintergrundjobs, Zeitplan von `wishes.sync` | Integrationen › Seerr, Band «Abgleich» («alle 5 Minuten · Zeitplan ändern») |
| Seerr-Benutzer des Kontos (eröffnen, verknüpfen, von Hand, lösen) und Ausnahme je Profil | Wünsche › Seerr-Benutzer | Konten (Zeile «Seerr-Benutzer»), Integrationen › Seerr (Band «Benutzer und Limit»: «n von m Konten») |
| Passwortabgleich je Konto («Passwort bei der nächsten Anmeldung neu übernehmen», «nicht abgleichen») | Wünsche › Seerr-Benutzer | Konten (Zeile «Seerr-Benutzer») |
| Wunschrecht des Kontos | Konten, Formular «Zugriff», bei Administratorkonten Block «Wünsche» (und Dialog «Konto einladen» für das neue Konto) | Wünsche › Seerr-Benutzer (Chip, Rechte in Seerr) |
| Wunschrecht des Profils | Profildialog (Hauptprofil des Kontos) | Konten (Chips der Profile) |
| Limit je Konto («Standard», «Eigenes», «Unbegrenzt») | Konten, Formular «Zugriff» bzw. Block «Wünsche» (und Dialog «Konto einladen») | Wünsche › Seerr-Benutzer (Stand), Wünsche › Anfragen (Band «Limit je Konto») |
| Limit ab Werk | Wünsche › Einstellungen, Band «Limit ab Werk» | Konten («Standard: 10 Filme, 5 Staffeln in 7 Tagen») |
| Neue Konten in Seerr eröffnen, Passwort abgleichen | Wünsche › Einstellungen, Band «Konten in Seerr» | Integrationen › Seerr, Band «Benutzer und Limit» |
| Präfix des Profil-Tags, Schalter «Profil-Tag in Radarr und Sonarr setzen» | Wünsche › Einstellungen, Band «Radarr und Sonarr: Profil-Tag und Fortschritt» | Integrationen › Seerr, Band «Radarr und Sonarr (Profil-Tags und Fortschritt)» |
| Schalter «Download-Fortschritt zeigen» | Wünsche › Einstellungen, Band «Radarr und Sonarr: Profil-Tag und Fortschritt» | Integrationen › Seerr, Band «Radarr und Sonarr (Profil-Tags und Fortschritt)» (Wert) |
| Name des Profil-Tags je Profil | Wünsche › Seerr-Benutzer, Band des Kontos | Wünsche › Anfragen (Tag je Wunsch) |
| Eigene Adresse von Radarr bzw. Sonarr (für Tag und Fortschritt) | Integrationen › Seerr, Band «Radarr und Sonarr (Profil-Tags und Fortschritt)» | – |
| Webhooks von Radarr und Sonarr | Integrationen › Webhooks von Radarr und Sonarr, unverändert (N-80) | Integrationen › Seerr, Band «Radarr und Sonarr (Profil-Tags und Fortschritt)» (nur Link) |
| Seiten «Entdecken», «Filme», «Serien», «Suche» | Wünsche › Einstellungen | – |
| Funktionsschalter der Wünsche | Wünsche › Einstellungen | – |
| Reihe «Erfüllte Wünsche» (Platz, Ausblenden) | Startseite › Startseite | – |
| Kanal-Ereignisse | Benachrichtigungen (Kanalseite des Kontos) | – |

### 1.2 Einordnung in die sechs Bereiche

- **Verbindung unter Automatisierung › Integrationen.** Dort stehen heute alle Dienste ausserhalb von
  Slouchcat mit Schlüsseln, Prüfen und Zustand (Schlüssel der Anbieter, Webhooks, E-Mail, Telegram, Web-Push,
  Server). Seerr ist ein weiterer solcher Dienst. Einrichten macht man einmal.
- **«Wünsche» unter Medien.** Wünsche sind Arbeit an der Bibliothek, wie Pflege, Podcasts und Radio: Man schaut
  regelmässig hinein, gibt frei (Stufe 4), sieht Fehler. Die Seite Seerr-Benutzer steht dabei, weil die Benutzer
  in Seerr nur für Wünsche gebraucht werden.
- **Verworfen:**

| Alternative | Warum nicht |
|---|---|
| eigener siebter Bereich «Wünsche» | ADR 0035 legt sechs Bereiche fest; ein Bereich für eine Seite sprengt die Leiste |
| alles unter Integrationen | mischt die tägliche Liste der Anfragen mit dem einmaligen Einrichten; die Ansichten dort sind für Dienste gedacht, nicht für Listen von Titeln |
| Seerr-Benutzer unter Nutzer & Geräte › Konten | die Seite braucht die Liste der Seerr-Benutzer, ihre Rechte, den Stand des Passworts und des Limits; auf «Konten» stehen nur das Limit des Kontos (dessen einziger Ort), der Wert und der Link |
| Anfragen unter Automatisierung | es geht um Titel, nicht um Abläufe; Jobs bleiben unter Hintergrundjobs |

Der Reiter «Wünsche» steht im Bereich Medien direkt nach «Pflege» (wie im Mockup) und trägt den Zähler
«wartet auf Freigabe» (**Vorschlag**, Muster: Zahlen an Bereichsreitern, ADR 0035). Die Reihenfolge im Bereich
Medien wird damit: Bibliotheken, Metadaten, Pflege, Wünsche, Podcasts, Radio, Hörbuch-Quellen.

### 1.3 Gestaltungsregeln von Marco (16. November 2026)

- Alles Klappbare startet **zugeklappt** und zeigt seine Zusammenfassung im Kopf (Frage 32, P-3). Ausnahmen wie
  dort: Sprunglink, eben Angelegtes, Fehler oder aktuelle Wahl darin.
- **Anlegen und Ändern im selben Formular** (Frage 32, P-2). Für die Seerr-Benutzer heisst das: Eröffnen,
  Verknüpfen, Lösen und die Ausnahme je Profil geschehen am selben Ort, direkt in der Liste der Seite
  Seerr-Benutzer (Knöpfe und Auswahlfeld je Konto und je Profil, sofort wirksam), nicht in einem eigenen
  Dialog. Das Limit je Konto steht im selben Formular wie das Wunschrecht («Zugriff»).
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
| Kennzahl «Offene Wünsche» | Zahl aller Wünsche in `queued`, `pending`, `processing`, `partial`; Unterzeile «n warten auf Freigabe» | Verweis auf Wünsche › Anfragen. **Vorschlag:** als fünfte Kachel; nur sichtbar, wenn Seerr eingerichtet ist |
| Hinweis «n Wünsche warten auf Freigabe» | Wünsche `pending` | Link auf Anfragen, Filter «Freigabe ausstehend» |
| Hinweis «Seerr nicht erreichbar» | Zustand Nicht erreichbar oder Schlüssel ungültig | Link auf Integrationen › Seerr; Text nennt den Zustand |
| Hinweis «n Konten ohne Seerr-Benutzer» | Konten mit Wunschrecht im Zustand «nicht eröffnet» oder «wartet» (und ohne Sammelbenutzer, F-4). Ihre Profile sehen keine fehlenden Titel; nur hier und auf den Seiten Seerr-Benutzer und Konten fällt der Fall auf. Text nennt, ob Seerr gerade nicht erreichbar ist («Die Konten warten und werden beim nächsten Abgleich eröffnet.») | Link auf Wünsche › Seerr-Benutzer mit «Alle eröffnen» |
| Hinweis «Neu verknüpft» (neu, Stufe 1) | Konten, die beim Annehmen einer Einladung oder nach einer E-Mail-Änderung sofort mit einem bestehenden Seerr-Benutzer verknüpft wurden (`linked_by` = `invite` oder `email_change`), sieben Tage ab `linked_at`. Text: «Neu verknüpft: {{Konto}} mit dem bestehenden Seerr-Benutzer {{Name}}» (N-93, Empfehlung der Redaktion). Die Adresse hat das Konto selbst eingetippt; niemand hat sie geprüft (N-92). Eröffnet Slouchcat einen neuen Benutzer oder verknüpft ein Administrator, erscheint kein Hinweis | Link auf Wünsche › Seerr-Benutzer; das Band dieses Kontos zeigt dort den Hinweis und «Verknüpfung lösen» |
| Hinweis «Limit erreicht: {{Konto}}» | Konten, deren Limit für Filme oder Staffeln ausgeschöpft ist, laut letztem Stand aus Seerr (`03-architektur.md`, Abschnitt 19.3). Text: «Filme: 10 von 10 in 7 Tagen, der nächste Platz wird am … frei» | Link auf Konten, Formular «Zugriff» dieses Kontos |
| Hinweis «n Wünsche ohne Profil-Tag» (Stufe 4) | Zeilen in `wish_tags` mit Zustand `failed`, also Tags, die nicht gesetzt werden konnten (Redaktionsentscheid, `mockup-abdeckung.md` Abschnitt 10.2, Nr. 2); ein Tag, der noch auf den Titel in Radarr wartet, zählt nicht. Text nennt Titel und Dienst («Dune (Radarr)») und den Grund (Radarr nicht erreichbar, Titel noch nicht dort) | Link auf Wünsche › Anfragen, Filter «Tag nicht gesetzt» |
| Hinweis «n Downloads verzögert» (neu, Stufe 4) | Wünsche, deren Download-Fortschritt «Download verzögert» meldet (`trackedDownloadStatus` Warnung oder Fehler, `03-architektur.md`, Abschnitt 22.2). Text nennt Titel und Dienst («Dune (Radarr): Download verzögert»), nicht die Meldung von Radarr | Link auf Wünsche › Anfragen, Filter «Download verzögert»; die Meldung steht dort in der Aufklappzeile |
| Hinweis «Andere Seerr-Instanz erkannt» | Fingerabdruck der Instanz weicht ab (`03-architektur.md`, Abschnitt 7.4); Übermittlung und Abgleich halten an | Link auf Integrationen › Seerr, dort «Zuordnung bestätigen» oder «Zuordnung löschen» |
| **Vorschlag:** Hinweis «n Wünsche konnten nicht übermittelt werden» | Wünsche `failed` mit Grund Übermittlung | Link auf Anfragen, Filter «Fehler» |
| **Vorschlag:** Hinweis «Benutzer in Seerr gelöscht» | zugeordnete Benutzer mit `gone_at` | Link auf Seerr-Benutzer |

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

**Neu:** eine **siebte Kachel «Seerr»** (`?item=jellyseerr`, im Mockup `#/admin/integrations/jellyseerr`).
Sie steht nach «Webhooks von Radarr und Sonarr» (wie im Mockup). Die Zeile oben («n von m eingerichtet») zählt
dann neun Statusschlüssel: TMDB, TVDB, YouTube, Trakt, E-Mail, Telegram, Web-Push, Update-Prüfung und
Seerr.

| Teil der Kachel | Inhalt |
|---|---|
| Beschreibung | «Wünsche der Zuschauer an Seerr weitergeben und ihren Zustand abgleichen. Konten erhalten dort einen Benutzer mit demselben Login.» (Wortlaut Vorschlag) |
| Meta | «Seerr-Benutzer: 3 von 3 Konten» · «5 offene Wünsche» |
| Zustand | Punkt und «Seerr: Verbunden», «Nicht eingerichtet», «Schlüssel ungültig», «Nicht erreichbar», «Andere Instanz erkannt» oder «Ausgeschaltet» |
| Hinweise im Zustand | **Vorschlag:** Bei «Verbunden» mit gelber Warnung im Test zeigt die Kachel einen gelben Punkt und den ersten Hinweis, etwa «Lokale Anmeldung in Seerr aus», «Staffeln aus TMDB», «Sonarr für Profil-Tags und Fortschritt nicht erreichbar» oder «1 Konto ohne Seerr-Benutzer». Der Zustand bleibt «Verbunden»; eine Warnung ist kein Fehler |

Die bestehenden sechs Ansichten bleiben unverändert, auch die Ansicht **webhooks** («Webhooks von Radarr und
Sonarr»): Marco hat «Webhooks wie heute» gewählt (E-6, N-80). Der Administrator richtet die Webhooks weiter von Hand
in Radarr und Sonarr ein; Slouchcat trägt dort keinen Webhook ein. Die Seite bleibt wörtlich, wie sie ist, auch der
Text `webhooks.intro`; sie bekommt keinen neuen Satz, auch nicht als Vorschlag (Redaktionsentscheid zu K-7; AK-196).
Der Text spricht vom Empfang der Webhooks, und der bleibt unverändert; was Slouchcat für Profil-Tag und Fortschritt
liest, steht im Band «Radarr und Sonarr (Profil-Tags und Fortschritt)» (4.8).

## 4. Integrationen › Seerr (M-22)

Neue Ansicht, Verweis «‹ Alle Integrationen». Sechs Bänder, alle zugeklappt mit Zusammenfassung im Kopf:
«Verbindung», «Abgleich», «Benutzer und Limit» (neu, 4.7), «Radarr und Sonarr (Profil-Tags und Fortschritt)» (neu,
Stufe 4, 4.8),
«Sofortmeldung», «Protokoll».

### 4.1 Band «Verbindung»

Kopf: «Verbindung» · «Verbunden · Jellyseerr 2.7.3» bzw. der Zustand.

| Feld | Beschriftung und Hilfe | Werte | Vorgabe | Wirkung |
|---|---|---|---|---|
| Schalter | «Seerr verwenden» · «Aus: Slouchcat ruft Seerr nicht auf. Wünsche bleiben gespeichert.» | an/aus | aus | aus stoppt Übermittlung und Abgleich; Zustand «Ausgeschaltet» |
| Adresse | «Adresse» · «Zum Beispiel http://192.168.1.50:5055. Slouchcat spricht nur diese Adresse an.» | `http`/`https`, ohne Benutzerangaben, ohne `/api/v1` | leer | Ziel aller Aufrufe; LAN erlaubt, keine Weiterleitungen |
| API-Schlüssel | «API-Schlüssel» · «Den Schlüssel zeigt Seerr in seinen Einstellungen. Leer lassen, um den gespeicherten zu behalten.» | geheim | leer | verschlüsselt gespeichert, nie wieder angezeigt; Platzhalter «Gesetzt, endet auf …a1b2». Hat die eingegebene Adresse einen anderen Host als die gespeicherte, muss der Schlüssel neu eingegeben werden; der gespeicherte geht nie an eine neue Adresse |
| Knopf | «Verbindung testen» | – | – | prüft die eingegebene oder gespeicherte Adresse, zeigt die Prüfliste; bei `http` zu einer Adresse ausserhalb des Heimnetzes die Warnung «Schlüssel ginge unverschlüsselt ins Internet» |
| Knöpfe | «Speichern», «Verwerfen» | – | – | Speichern löst `seerr.users` und einen Test aus |

**Vorschlag** wie die anderen Schlüssel (siehe `03-architektur.md`, Abschnitt 4.2): `SLOUCHCAT_SEERR_URL` und
`SLOUCHCAT_SEERR_API_KEY` in `deploy/.env` gewinnen und sperren die Felder mit «In deploy/.env gesetzt»; dazu
«Löschen …» mit Rückfrage «Verbindung zu Seerr löschen? Wünsche bleiben gespeichert, werden aber nicht mehr
übermittelt und abgeglichen.»

**Prüfliste des Tests** (Aufrufe wie `03-architektur.md`, Abschnitt 15.1):

| Prüfpunkt | Ergebnis gut | Ergebnis schlecht |
|---|---|---|
| erreichbar | «Erreichbar» | «Nicht erreichbar: Zeitüberschreitung» / «… Verbindung abgelehnt» / «… Weiterleitung nicht erlaubt» |
| Schlüssel gültig | «Schlüssel gültig» | «Schlüssel ungültig» (403) |
| Version | «Jellyseerr 2.7.3» bzw. «Seerr 3.5.2» | ältere als Jellyseerr 2.7 oder Seerr 3.0: Warnung «Version nicht geprüft: getestet sind Jellyseerr 2.7 und Seerr 3.x» (kein Fehler) |
| Instanz | «Gleiche Instanz wie beim Einrichten» | «Andere Seerr-Instanz erkannt» mit den Knöpfen «Zuordnung bestätigen» und «Zuordnung löschen» (Rückfrage); bis dahin halten Übermittlung und Abgleich an (`04-benutzer-profilmapping.md`, Abschnitt 8.4) |
| Handeln im Namen von Benutzern | «wirkt» (`X-API-User`) | «wirkt nicht: Wünsche würden als Administrator gestellt» (sperrt «Wünsche aktiv», **Vorschlag** T-1) |
| CSRF-Schutz in Seerr aus | «aus» | «an: schreibende Aufrufe scheitern. In den Netzwerk-Einstellungen von Seerr ausschalten.» |
| Radarr und Sonarr in Seerr eingerichtet | «Radarr: 1, Sonarr: 1» | Warnung «Kein Sonarr in Seerr: Serien-Wünsche bleiben liegen.» (kein Fehler) |
| Benutzer anlegen und ändern (neu) | «Der Schlüssel des Administrators darf Konten in Seerr eröffnen und Rechte, Limit und Passwort setzen.» | Fehler «Der Schlüssel darf keine Benutzer anlegen: Konten lassen sich nicht eröffnen.» |
| Lokale Anmeldung in Seerr (neu) | «Eröffnete Benutzer melden sich in Seerr mit E-Mail und Passwort aus Slouchcat an.» | Warnung «Eröffnete Benutzer können sich in Seerr nicht anmelden. In Seerr unter Einstellungen › Benutzer «Lokale Anmeldung» einschalten.» (N-11) |
| Staffeln aus TVDB (neu) | «Seerr, Sonarr und die Slouchcat-Bibliothek zählen Staffeln gleich.» | Warnung «Seerr liest Staffeln aus TMDB. Empfohlen ist TVDB, wie in Sonarr und in der Slouchcat-Bibliothek, sonst können Staffelnummern abweichen.» (N-64, F-29) |
| Radarr für Profil-Tags und Fortschritt (neu, Stufe 4) | «Radarr erreichbar unter {{Adresse}} · Warteschlange lesbar · Tag Requests: an» | Warnung «Radarr nicht erreichbar: Profil-Tags werden nachgeholt, sobald es geht; der Download-Fortschritt fehlt so lange. Eigene Adresse eintragen?» |
| Sonarr für Profil-Tags und Fortschritt (neu, Stufe 4) | wie Radarr | wie Radarr |
| Passwort verschlüsselt (neu) | keine Zeile, wenn `https` oder Adresse im eigenen Netz | Warnung «Passwörter gingen unverschlüsselt ins Internet. Nutze https.» (N-12) |

**Eine Warnung lässt den Test nicht scheitern.** Der Test gilt als gelungen, wenn kein Prüfpunkt rot ist; gelbe
Prüfpunkte stehen als Hinweis in der Kachel (Abschnitt 3) und im Kopf des Bands. Für T-1 zählen nur «erreichbar»,
«Schlüssel gültig» und «Handeln im Namen von Benutzern» (`03-architektur.md`, Abschnitt 15.1).

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
| Fakt «Download-Fortschritt» (Stufe 4) | «Fortschritt aus Radarr und Sonarr: jede Minute, solange Wünsche in Beschaffung sind · zuletzt vor 40 s» bzw. «aus» oder «nicht lesbar». Der Abgleich mit Seerr bleibt davon unberührt; der Fortschritt ändert keinen Zustand (N-84, N-87). Link «Zum Band Radarr und Sonarr» |
| Link | «Zu den Jobs» |

### 4.3 Band «Sofortmeldung» (Stufe 4, F-18)

| Element | Inhalt |
|---|---|
| Einleitung | «Seerr stösst den Abgleich sofort an, statt bis zu 5 Minuten zu warten. Der Abgleich bleibt die Grundlage.» |
| Knopf | «Adresse erzeugen» bzw. «Token erneuern» (Rückfrage wie bei Radarr/Sonarr) |
| Adresse | einmal gezeigt, «Adresse kopieren», Warnung «Jetzt kopieren, danach ist die Adresse nicht mehr sichtbar.» |
| JSON-Vorlage | zum Kopieren in Seerr (Einstellungen › Benachrichtigungen › Webhook) |
| Hinweis | «Seerr kennt nur ein Webhook-Ziel. Nutzt du es schon für etwas anderes, ersetzt diese Adresse es.» |
| Letzte Meldung | Zeit oder «Hat noch nie etwas gemeldet.» |

### 4.4 Band «Protokoll»

Kopf: «Protokoll» · «20 Einträge · 1 Fehler». Tabelle der letzten 20 Aufrufe: Zeit, Aufruf (Methode und Pfad
ohne Query), Ergebnis (Status), Dauer, Fehler. Nie Header, nie Schlüssel. Eigene Tabelle `seerr_log`
(`08-datenmodell-api.md`, Abschnitt 2.8) mit höchstens 20 Zeilen, gehalten von einem Trigger wie `webhook_events` (`server/migrations/00017_integrations.sql:47-57`); Anhang F nennt
dafür noch keine Tabelle (Abschnitt 5 hat nur `wish_events` je Wunsch).

### 4.5 Verweise

«Seerr-Benutzer: n von m Konten» → Medien › Wünsche › Seerr-Benutzer. **Vorschlag:** Hinweis, wenn die öffentliche
Adresse (Integrationen › Server) fehlt: «Ohne öffentliche Adresse fehlen Links in E-Mails.» Hinweis, dass
Seerr eigene Benachrichtigungen weiter verschickt (`06-benachrichtigungen.md`, Abschnitt 8).

### 4.6 Lücke: zwei Schalter

«Seerr verwenden» (Verbindung) und «Wünsche aktiv» (Funktion, M-25) wirken beide auf das Anfordern. Ein Ort
je Einstellung bleibt gewahrt, weil sie Verschiedenes schalten. Das Zusammenspiel (Redaktionsentscheid vom 17. November 2026, Lücke in Anhang F):

| Seerr verwenden | Wünsche aktiv | Zuschauer | Hintergrund |
|---|---|---|---|
| aus | egal | kein Anfordern, keine fehlenden Titel; «Meine Wünsche» zeigt bestehende Wünsche nur zum Lesen | kein Aufruf; die 24-h-Frist der Wünsche `queued` ruht (**Vorschlag**, T-2) |
| an | aus | wie oben | Abgleich läuft weiter für bestehende Wünsche; `queued`-Wünsche werden weiter übermittelt |
| an | an | Anfordern nach Recht; fehlende Titel nach der Regel von F-9 | alles läuft |

Vorgabe: beide aus. **Vorschlag** (T-1): «Wünsche aktiv» lässt sich nur einschalten, wenn der Test zuletzt
gelang und «Handeln im Namen von Benutzern» wirkt. Anhang F verlangt nur, dass der Test meldet, wenn es nicht
wirkt.

### 4.7 Band «Benutzer und Limit» (neu, Stufe 1)

Kopf: «Benutzer und Limit» · «3 von 3 Konten in Seerr · Limit 10 Filme, 5 Staffeln in 7 Tagen». Das Band
stellt nichts ein; es zeigt den Stand und verlinkt auf die Orte der Einstellungen (Abschnitt 1.1).

| Element | Inhalt |
|---|---|
| Einleitung | «Jedes Konto hat in Seerr einen eigenen Benutzer mit demselben Login. Seine Profile fordern als dieser Benutzer an und tragen einen Tag. Rechte, Limit und Freigabe kommen von diesem Benutzer.» (Wortlaut Mockup) |
| Zeile «Konten in Seerr» | «3 von 3 Konten: 2 eröffnet, 1 verknüpft» bzw. «1 wartet, bis Seerr erreichbar ist»; Link «Zu den Seerr-Benutzern» |
| Zeile «Neue Konten» | Wert von «Neue Konten in Seerr eröffnen» und «Passwort abgleichen», Link «Einstellungen» |
| Zeile «Passwort» | «Slouchcat gibt das Passwort an Seerr weiter, wenn es gesetzt oder geändert wird und bei der Anmeldung in Slouchcat. Slouchcat speichert es nie im Klartext. Ein Passwort, das jemand direkt in Seerr ändert, bemerkt Slouchcat nicht.» (N-3, N-7) |
| Zeile «Limit ab Werk» | Wert und Link «Einstellungen»; Hinweis «Administratoren in Seerr haben kein Limit.» (N-34) |
| Fakten | «Lokale Anmeldung: an» bzw. die Warnung aus dem Test |

### 4.8 Band «Radarr und Sonarr (Profil-Tags und Fortschritt)» (neu, Stufe 4)

Kopf: «Radarr und Sonarr (Profil-Tags und Fortschritt)» · «Radarr erreichbar · Sonarr erreichbar · 2 Tags offen ·
3 Downloads» bzw. «aus · Profil-Tags und Fortschritt ausgeschaltet».

| Element | Inhalt |
|---|---|
| Einleitung | «Slouchcat legt in Radarr und Sonarr je Profil einen Tag an und hängt ihn an den Titel, sobald Seerr ihn dorthin übergeben hat. Dazu liest es die Warteschlange und den Dateistand der gewünschten Titel, um bei Wünschen den Download-Fortschritt zu zeigen. Sonst ändert Slouchcat in Radarr und Sonarr nichts. Neue Titel meldet weiter der Webhook.» (Wortlaut **Vorschlag**, N-22, N-81, K-5) |
| Je Dienst: Adresse | «Adresse aus Seerr: http://radarr:7878» (aus `GET /settings/radarr`, Standardserver ohne 4K, N-23); nur Anzeige |
| Je Dienst: «Eigene Adresse» | Feld, leer = Adresse aus Seerr; Hilfe «Nur nötig, wenn Slouchcat die Adresse aus Seerr nicht erreicht, etwa bei Docker-internen Namen.» Der Schlüssel kommt immer aus Seerr; es gibt kein Schlüsselfeld (F-30). Gespeichert mit «Speichern» des Bands |
| Je Dienst: Knopf «Testen» | prüft Erreichbarkeit und Schlüssel (`GET /api/v3/tag`) und ob die Warteschlange lesbar ist (`GET /api/v3/queue`); bestätigt den Host (**Vorschlag**, `03-architektur.md`, Abschnitt 20.2) |
| Je Dienst: Fakt «Tag Requests» | «Tag Requests in Seerr: an» bzw. «aus · Empfohlen: dann trägt jeder Titel auch den Tag des Kontos» (N-28) |
| Zähler | «Tags gesetzt (30 Tage)», «offen», «fehlgeschlagen», je als Link auf Anfragen mit Filter |
| Fakt «Download-Fortschritt» | «an · 3 Wünsche mit Fortschritt · zuletzt gelesen vor 40 s» bzw. «aus · Einstellungen» oder «Fortschritt nicht lesbar: Sonarr nicht erreichbar» (N-85, N-86) |
| Verweise | «Präfix sc- und Download-Fortschritt · Einstellungen»; «Namen je Profil · Seerr-Benutzer»; «Webhooks von Radarr und Sonarr · unverändert, von Hand eingerichtet» (Link auf die Ansicht **webhooks**, N-80) |

Fehlerbilder: Adresse aus Seerr nicht erreichbar → gelber Punkt mit «Eigene Adresse eintragen»; Seerr
meldet einen anderen Host als den bestätigten → «Radarr-Adresse in Seerr geändert. Testen, um sie zu
übernehmen.»; bis dahin halten `wishes.tag` und `wishes.progress` an.

## 5. Medien › Wünsche › Anfragen (M-23)

Neue Seite `/admin/wishes`. Kopf: «MEDIEN / WÜNSCHE», Titel «Wünsche», Einleitung «Wünsche der Profile und ihr
Zustand in Seerr. Freigeben und Ablehnen geschieht in Seerr (ab Stufe 4 auch hier).» Reiter
«Anfragen», «Seerr-Benutzer», «Einstellungen».

| Teil | Inhalt |
|---|---|
| Kennzahlen | «Offen», «Freigabe ausstehend», «In Beschaffung», «Verfügbar (30 Tage)», «Fehler»; je ein Klick setzt den Filter |
| Filter | Zustand (alle Zustände von Anhang F), «Nicht zuordenbar», Medienart, ab Stufe 4 «Tag nicht gesetzt» und «Download verzögert»; **Vorschlag:** Suche nach Titel |
| Tabelle | «Titel» (Poster über die Admin-Bildroute bzw. `getAdminItemImage`, Titel, Jahr), «Art» (Film, Serie mit Staffeln «S1–S3»), «Gewünscht von» («Konto · Profil» und «+2 Mitwünscher»), «Seerr-Benutzer», «Zustand» (Chip), «Seit»; Kennzeichnung «In Seerr angelegt» bei `origin` = `jellyseerr`. Ab Stufe 4 unter «Gewünscht von» je Profil der **Profil-Tag** als Chip mit Zustand: «sc-noah» (gesetzt), «sc-noah · noch nicht gesetzt» (offen, Hilfe «Der Tag folgt, sobald der Titel in Radarr ist»), «sc-noah · fehlgeschlagen» (mit Grund). Wünsche, die direkt in Seerr gestellt wurden, haben keinen Profil-Tag, auch wenn «Anfragen übernehmen» sie später einem Profil zuordnet (N-27). Ab Stufe 4 unter «Zustand» bei «In Beschaffung» der **Download-Fortschritt**: «Wird gesucht», «Wird heruntergeladen · 45 % · noch 12 min», «Wird eingelesen» oder «Download verzögert», bei Serien dazu «8 von 10 Folgen da»; «Fortschritt nicht lesbar», wenn Radarr oder Sonarr nicht antworten (N-83, N-85) |
| Aufklappzeile | **Verlauf** aus `wish_events` (angelegt, übermittelt, freigegeben, eingelesen, gemeldet, Tag gesetzt) und **Diagnose**: Request-ID und Media-ID in Seerr, Rohstatus, Versuche, nächster Versuch, letzter Fehler, zugeordneter Titel, IDs TMDB, TVDB, IMDb, Sichtbarkeit je Anforderer («für Noah nicht sichtbar»), alle Mitwünscher, je Profil der Tag mit Dienst und letztem Versuch; ab Stufe 4 **Fortschritt** mit Dienst, Zeit des letzten Lesens und der **Meldung von Radarr oder Sonarr** (`errorMessage`, `statusMessages`, Download-Programm; ohne Pfade). Die Meldung sieht nur die Verwaltung, nie ein Zuschauer (N-83) |
| Band «Limit je Konto» (neu) | zugeklappt; Kopf «1 Konto am Limit». Tabelle je Konto: «Konto», «Filme» («7 von 10 in 7 Tagen»), «Staffeln» («5 von 5 in 7 Tagen · nächster Platz am 20.11.»), «Art» («Standard», «Eigenes», «Unbegrenzt», «Kein Limit (Administrator)»). Gezählt wird wie in Seerr, auch Wünsche, die direkt dort gestellt wurden (E-3); Stand des letzten Abgleichs. Link «Limit ändern» auf Konten. Nur Anzeige |
| Aktionen je Zeile | «In Seerr öffnen» (neues Fenster), «Zum Titel» (wenn in der Bibliothek), «Erneut übermitteln» (`queued`, `failed`), «Freigeben» und «Ablehnen» (nur `pending`, Stufe 4, F-16) |
| Leer | «Noch keine Wünsche.» bzw. bei ausgeschalteter Funktion «Wünsche sind ausgeschaltet.» mit Link auf «Einstellungen» |

- «Ablehnen» fragt nach: «{{Titel}} ablehnen? Seerr kennt keinen Grund; die Profile sehen nur «Abgelehnt».»
- **Vorschlag:** «Erneut übermitteln» bei einem Request im Zustand FAILED in Seerr (`arr_failed`) ruft dort
  `POST /request/{id}/retry`; das ist für diesen Fall der einzige Weg, weil der Zuschauer nicht erneut anfordern
  kann (`03-architektur.md`, Abschnitt 8.1). Bei einem lokal gescheiterten Wunsch reiht es `wishes.submit` neu ein
  und löst die Zuordnung neu auf (`04-benutzer-profilmapping.md`, Abschnitt 8.1).
- Wieder geöffnete Wünsche (Zurückziehen kam zu spät, `03-architektur.md`, Abschnitt 6.6) tragen im Verlauf
  «Zurückziehen nicht möglich: war schon freigegeben».
- Nicht zuordenbare Anfragen haben nur «In Seerr öffnen» und «Benutzer zuordnen» (Link auf
  «Seerr-Benutzer» mit vorgewähltem Benutzer).
- Fehlerbilder: Seerr nicht erreichbar → Band oben «Zustand von {{Zeit}}, Seerr nicht erreichbar»,
  Freigeben/Ablehnen gesperrt mit Grund; 409 beim Freigeben («Only pending requests …») → «Schon entschieden,
  der Abgleich holt den neuen Zustand.»

## 6. Medien › Wünsche › Seerr-Benutzer (M-24)

Der Reiter hiess in Fassung 2 «Zuordnung». Seit Fassung 3 eröffnet oder verknüpft Slouchcat die Benutzer selbst
(Marco, E-1 und E-4). Gibt es die E-Mail in Seerr schon, verknüpft Slouchcat sofort, auch wenn das Konto die
Adresse beim Annehmen einer Einladung selbst eingetippt hat (Marco, E-8; N-90). Die Seite zeigt darum je Konto, welchen Seerr-Benutzer es hat und wie es dazu kam.
Route im Mockup weiter `#/admin/wishes/mapping`.

**Heute:** gibt es nicht (neue Seite).

| Teil | Inhalt |
|---|---|
| Kopfzeile | vier Kennzahlen: «Konten», «In Seerr» (eröffnet oder verknüpft), «Ohne Benutzer» (mit «wartet»), «Passwort noch offen» (folgt bei der nächsten Anmeldung). Gelöschte Benutzer nennen der Hinweis darunter und das Band des Kontos; Knöpfe «Alle eröffnen» (`provisionSeerrAccounts`: verknüpft vorhandene Benutzer mit gleicher E-Mail und legt für die übrigen einen neuen an, N-2), «Benutzerliste neu laden» (`refreshSeerrUsers`, Zeit des letzten Ladens) und ab Stufe 4 «Anfragen übernehmen» (`wishes.import` für alle Benutzer der Konten) |
| Band «Konten» | Je Konto ein eigenes Band, **zugeklappt** (Gestaltungsregel, Abschnitt 1.3; Redaktionsentscheid zum Abgleich mit dem Mockup, `mockup-abdeckung.md` Abschnitt 10.2, Nr. 14). Kopf des Bandes, kurz: E-Mail des Kontos, Rolle (Chip «Administrator» oder «Konto», dazu «Besitzer»), Chip mit dem Zustand, Name des Seerr-Benutzers und die Ausschöpfung des Limits («Filme 7/10 · Staffeln 2/5»). Aufgeklappt zeigt das Band eine Liste aus Begriff und Wert: «Seerr-Benutzer» (Name und ID, E-Mail, Art und Herkunft «von Slouchcat eröffnet» oder «bestand schon in Seerr»; ohne Benutzer «Keiner»), «Zustand» («Von Slouchcat eröffnet», «Verknüpft über gleiche E-Mail», «Von Hand zugeordnet», «Wird eröffnet, sobald Seerr erreichbar ist», «Nicht eröffnet»; bei «Verknüpft über gleiche E-Mail» dazu der Auslöser: «beim Annehmen der Einladung», «nach Änderung der E-Mail» oder «durch einen Administrator», mit Datum), «Passwort» («Gleich wie in Slouchcat» mit «abgeglichen am …», «Wird bei der nächsten Anmeldung in Slouchcat übernommen», «Wird nicht abgeglichen», «Bleibt in Seerr (Administrator)», «Anmeldung über Jellyfin oder Plex»; ohne Benutzer «Folgt mit dem Benutzer»), «Rechte in Seerr» («Darf anfordern» = `REQUEST`, «Keine Rechte», bei verknüpften die Fakten aus Seerr; ohne Benutzer «Folgen dem Wunschrecht des Kontos»), «Limit» («Filme 7 von 10 · Staffeln 2 von 5 in 7 Tagen», inklusive der Wünsche, die direkt in Seerr gestellt wurden, E-3; «Kein Limit (Administrator)», N-34; ohne Benutzer «Wird beim Eröffnen gesetzt» mit Link «Limit ändern»). Darunter die Aktionen und die Profile |
| Aktionen im Band | «In Seerr eröffnen» (`provisionSeerrAccount`; bei gleicher E-Mail verknüpft es, Hinweis «Gleiche E-Mail-Adresse: «In Seerr eröffnen» verknüpft diesen Benutzer, statt einen zweiten anzulegen.»), «Anderen Benutzer wählen» (Auswahlfeld, `setAccountWishMapping`, Zustand «Von Hand»), «Verknüpfung lösen» (`setAccountWishMapping` mit leer; der Benutzer bleibt in Seerr), «Passwort bei der nächsten Anmeldung neu übernehmen» und «Passwort nicht abgleichen» (`setSeerrPasswordMode`, N-7). Ist das Konto beim Annehmen einer Einladung oder nach einer E-Mail-Änderung verknüpft worden, steht über den Aktionen ein neutraler Hinweis: «Beim Annehmen der Einladung mit dem bestehenden Seerr-Benutzer {{Name}} verknüpft, weil die E-Mail gleich ist. Gehört er nicht zu diesem Konto, löse die Verknüpfung.» bzw. «… nach Änderung der E-Mail …» (Wortlaut **Vorschlag**, N-93) |
| Profile im Band | die Profile des Kontos mit je **Profil-Tag** (Name, z. B. `sc-noah`, Feld «Eigener Name» zum Überschreiben, `setProfileTagName`, N-21) und dem Wunschrecht des Profils (nur lesen, setzt das Hauptprofil). Darunter zugeklappt die **Ausnahme** «Profil sendet als anderer Seerr-Benutzer» mit Auswahlfeld «Wie Konto» / Benutzer (`setProfileWishMapping`, N-41, F-27) und dem Hinweis «Es gelten Rechte, Limit und Freigabe dieses Benutzers.» |
| Band «Seerr-Benutzer ohne Konto» | Benutzer aus `seerr_users`, die keinem Konto gehören: Name, E-Mail, Art (lokal, Jellyfin, Emby, Plex), Fakten, Zähler «n nicht zuordenbare Anfragen»; Aktion «Einem Konto zuordnen» |

Alles wirkt sofort und bestätigt sich mit einer Meldung, etwa «grosseltern in Seerr verknüpft.», «noah: Tag
heisst jetzt sc-noah-k.» oder «Ausnahme gesetzt: Profil Noah sendet als kinder. Es gelten Rechte, Limit und
Freigabe dieses Benutzers.» Einen eigenen Dialog gibt es nicht (Regel von Marco, Abschnitt 1.3). Nur «Alle
eröffnen» fragt vorher nach und listet, was geschieht («2 verknüpfen, 1 neu anlegen»).

**Verknüpft beim Annehmen einer Einladung (Marco, E-8).** Findet Slouchcat beim Annehmen einer Einladung oder
nach einer E-Mail-Änderung des Kontos in Seerr einen Benutzer mit derselben Adresse, verknüpft es ihn sofort und
setzt ihm das Passwort (N-90, N-91, N-95); Benutzer 1, andere Administratoren von Seerr und Benutzer mit Anmeldung
über Jellyfin, Emby oder Plex behalten ihr Passwort (N-6). Die Adresse hat das Konto selbst eingetippt; niemand hat
sie geprüft. Wer eine fremde Adresse eintippt, bekommt so den Seerr-Benutzer dieser Person samt ihren Anfragen.
Marco kennt das Risiko und hat es gewählt (N-92). Damit es nicht unbemerkt bleibt, zeigt die Übersicht sieben
Tage lang «Neu verknüpft» (Abschnitt 2), das Band des Kontos nennt den Auslöser, und das Protokoll hält die
Verknüpfung fest (N-93, Empfehlung der Redaktion). Lösen geht mit «Verknüpfung lösen». Das Konto selbst erfährt
nie, ob Seerr die Adresse kannte (N-94).

**Eröffnen ausgeschaltet.** Ist unter Wünsche › Einstellungen «Neue Konten in Seerr eröffnen» aus, sind die Knöpfe «In Seerr eröffnen» und «Alle eröffnen» gesperrt. Daneben stehen der Grund «Neue Konten in Seerr eröffnen ist ausgeschaltet.» und ein Link auf die Einstellung (Wortlaut **Vorschlag**). Der Server lehnt beide Aufrufe dann mit 409 `provision_disabled` ab (`08-datenmodell-api.md`, Abschnitt 4.5).

**Auswahl:** belegte Benutzer sind gesperrt mit «gehört zu {{Konto}}» (Form statt Blässe: Schloss und Text).
**Fakten je Benutzer:** «darf Filme anfordern», «darf Serien anfordern», «Auto-Freigabe Filme/Serien», «Limit
5 Filme in 7 Tagen» oder «ohne Limit».

**Warnungen:** «Benutzer in Seerr gelöscht» (rot), «darf in Seerr nicht anfordern» bzw. «darf keine
Serien anfordern» (gelb, nur bei verknüpften Benutzern; eröffnete führt Slouchcat selbst), «Auto-Freigabe für ein
Kinderprofil» (**Vorschlag**, gelb, wenn ein Profil unter 18 einen Benutzer mit Auto-Freigabe nutzt), «Lokale
Anmeldung in Seerr aus: das übernommene Passwort nützt dort nichts» (gelb, N-11), «Tag-Name ungültig»
(nur `a–z`, `0–9`, `-`).

**Meldungen mit Folgen:** «Benutzer geändert. 2 offene Wünsche bleiben bei kinder.» und ab Stufe 4 «3 Anfragen von
grosseltern werden übernommen (Profil Oma & Opa).»

Fehlerbilder: Seerr nicht erreichbar → letzte geladene Liste mit «Stand {{Zeit}}»; «In Seerr eröffnen»
setzt den Zustand «Wartet» («Die Konten warten dann und werden beim nächsten Abgleich eröffnet.»); belegter
Benutzer beim Ändern (gleichzeitige Änderung) → 409 `seerr_user_taken` mit dem Konto, das Auswahlfeld springt
zurück; «Neue Konten in Seerr eröffnen» aus → «In Seerr eröffnen» und «Alle eröffnen» eröffnen nichts
(Server: 409 `provision_disabled`); andere Instanz erkannt → Band oben mit «Zuordnung bestätigen» und «Zuordnung
löschen».

## 7. Medien › Wünsche › Einstellungen (M-25)

Schalter und Auswahlfelder, die **sofort** gelten (wie im Mockup); es gibt kein «Speichern» und kein
«Verwerfen». Jede Änderung bestätigt sich mit einer kurzen Meldung. Bänder zugeklappt mit Zusammenfassung.

| Feld | Hilfe | Werte | Vorgabe | Wirkung |
|---|---|---|---|---|
| «Wünsche aktiv» | «Profile mit Recht können fehlende Titel anfordern.» | an/aus | aus (Redaktionsentscheid; das Mockup zeigt es zur Vorführung an) | Hauptschalter der Funktion; einschaltbar nur mit gelungenem Test (**Vorschlag**, T-1) |
| «Filme erlauben» | «Gilt für alle Profile.» | an/aus | an | aus: kein «Film anfordern» |
| «Serien erlauben» | «Gilt für alle Profile.» | an/aus | an | aus: kein «Serie anfordern», keine «Staffeln anfordern» |
| «Anfordern aus der Suche» | «Abschnitt «Noch nicht in der Bibliothek».» | an/aus | an | aus: kein Abschnitt «Noch nicht in der Bibliothek» |
| «Anfordern auf Detailseiten» | «Fehlende Staffeln auf der Seite einer Serie.» | an/aus | an | aus: keine fehlenden Staffeln, keine Zeile «2 von 4 Staffeln vorhanden», kein «Staffeln anfordern». Die Vorschauseite eines fehlenden Titels bleibt erreichbar, solange mindestens eine der drei Stellen an ist |
| «Bereits angeforderte Titel» | «Was ein zweites Profil bei einem schon gewünschten Titel tun kann.» | «Mitwünschen» / «Nur Zustand zeigen» | Mitwünschen (F-7) | Knopf «Benachrichtige mich» oder nur der Zustand |
| «Konten ohne Seerr-Benutzer» | «Sammelbenutzer: Alle Konten ohne eigenen Benutzer teilen Limit und Auto-Freigabe.» | «Nicht erlauben» / «Sammelbenutzer» + Auswahl | Nicht erlauben (F-4) | Auflösung nach `04-benutzer-profilmapping.md`, Abschnitt 6.1. Seit Fassung 3 betrifft das nur noch Konten, die (noch) keinen Seerr-Benutzer haben (Zustand «Wartet» oder «Nicht eröffnet») |
| «Neue Konten dürfen anfordern» | «Vorgabe im Dialog «Konto einladen».» | an/aus | aus (F-5) | Vorgabe des Schalters beim Einladen; ändert bestehende Konten nicht |
| «Zurückziehen erlauben» | «Solange ein Wunsch nicht freigegeben ist.» | an/aus | an (F-17) | Knopf «Zurückziehen» in «Meine Wünsche» |
| «Externe Nachrichten bei PIN-Profilen ohne Titel» | «E-Mail, Push und Telegram nennen dann nur das Profil.» | an/aus | an (F-15) | PIN-Fassung der Texte |
| «Seiten Entdecken, Filme, Serien und Suche» (neu, Stufe 5) | «Reiter im Bereich «Wünsche». Aus: nur «Meine Wünsche».» | an/aus | an (**Vorschlag**) | aus: die vier Reiter fehlen, `wishes.discover` ruht; die Bibliothekssuche verweist nicht mehr auf die Wunsch-Suche (N-57) |
| Verweis | **Vorschlag:** «Verbindung: Verbunden · Zu Integrationen» | – | – | nur Anzeige |

**Band «Limit ab Werk» (neu, Stufe 1).** Gilt für alle Konten mit «Standard» (N-31). Hilfe: «Gilt für alle Profile
eines Kontos zusammen. Seerr zählt rollierend und rechnet auch Wünsche mit, die direkt dort gestellt werden.
Abgelehnte Wünsche und Mitwünsche zählen nicht. Administratoren haben kein Limit.» (Wortlaut Mockup; E-3, N-33,
N-34).

| Feld | Werte | Vorgabe | Wirkung |
|---|---|---|---|
| «Filme» Anzahl und «in … Tagen» | Anzahl 0 bis 100, Tage 1 bis 90 (Grenzen wie im Mockup, **Vorschlag**); 0 = ohne Limit | 10 in 7 Tagen (Marcos Beispiel, F-26) | Slouchcat schreibt den Wert in jeden Seerr-Benutzer eines Kontos mit «Standard» (`seerr.provision`) |
| «Staffeln» Anzahl und «in … Tagen» | wie oben | 5 in 7 Tagen (F-26) | wie oben |

**Band «Konten in Seerr» (neu, Stufe 1).**

| Feld | Hilfe | Werte | Vorgabe | Wirkung |
|---|---|---|---|---|
| «Neue Konten in Seerr eröffnen» | «Beim Annehmen einer Einladung eröffnet Slouchcat das Konto auch in Seerr: gleiche E-Mail, gleiches Passwort. Gibt es dort schon einen Benutzer mit derselben E-Mail, verknüpft Slouchcat ihn sofort. Die Adresse tippt das neue Konto selbst ein; niemand prüft sie. Darum zeigt die Übersicht jede solche Verknüpfung sieben Tage lang. Bestehende Konten eröffnest du unter Seerr-Benutzer mit «Alle eröffnen» oder einzeln.» (Wortlaut **Vorschlag**, N-90, N-92, N-93, K-15) | an/aus | an, sobald Seerr eingerichtet ist (N-8) | aus: Einladungen eröffnen nichts; «In Seerr eröffnen» und «Alle eröffnen» unter Seerr-Benutzer sind gesperrt mit Grund und Link hierher (Server: 409 `provision_disabled`) |
| «Passwort abgleichen» | «Slouchcat gibt das Passwort an Seerr weiter, wenn es gesetzt oder geändert wird und bei der Anmeldung. Slouchcat speichert es nie im Klartext. Dafür braucht Seerr die «Lokale Anmeldung».» | an/aus | an (N-8) | aus: kein Passwort geht an Seerr; eröffnete Benutzer bekommen ein Zufallspasswort |
| Hinweis (neu, nur Anzeige) | «Wer sich mit dem Login eines Kontos in Seerr anmeldet, sieht dort die Wünsche aller Profile dieses Kontos, auch die eines Profils mit PIN. Seerr verschickt zudem eigene Benachrichtigungen mit dem Titel; lass sie für eröffnete Benutzer in Seerr ausgeschaltet.» (Wortlaut **Vorschlag**, Redaktionsentscheid zu K-19) | – | – | – |

**Was das Einrichten auslöst (Redaktionsentscheid zu K-15).** Seerr einrichten und diese Schalter einschalten
eröffnet bestehende Konten nicht von selbst. Bestehende Konten eröffnet der Administrator unter
Seerr-Benutzer mit «Alle eröffnen» oder einzeln mit «In Seerr eröffnen» (wie in E-4 beschrieben). Von
selbst eröffnet nur das Annehmen einer Einladung.

Den Schalter «Anfordern aus dem Spotlight» aus Fassung 3 vor der zweiten Rückmeldung gibt es nicht mehr: Das
Spotlight zeigt keine fehlenden Titel (E-5, N-72). Fehlende Einträge der Trakt-Liste fordert der Administrator auf
der Seite Spotlight an (Abschnitt 9).

**Band «Radarr und Sonarr: Profil-Tag und Fortschritt» (neu, Stufe 4).**

| Feld | Hilfe | Werte | Vorgabe | Wirkung |
|---|---|---|---|---|
| «Profil-Tag in Radarr und Sonarr setzen» | «Hängt den Tag des Profils an den Titel, sobald Seerr ihn an Radarr oder Sonarr übergeben hat, und holt Tags nach, die nicht gesetzt werden konnten. Ändert sonst nichts in Radarr und Sonarr.» (Wortlaut Mockup) | an/aus | an (**Vorschlag**) | aus: `wishes.tag` ruht; gesetzte Tags bleiben (N-26) |
| «Präfix» | «Kleinbuchstaben, Ziffern und Bindestrich. Beispiel: sc-noah.» | Text, 1–10 Zeichen (**Vorschlag**) | `sc-` (N-21, F-25) | gilt für neue Tags; bestehende Tags bleiben, wie sie sind |
| «Download-Fortschritt zeigen» (neu) | «Zeigt bei Wünschen in Beschaffung, ob der Titel gesucht, heruntergeladen oder eingelesen wird. Slouchcat liest dafür jede Minute die Warteschlange von Radarr und Sonarr, solange Wünsche in Beschaffung sind.» (Wortlaut **Vorschlag**) | an/aus | an (N-86) | aus: `wishes.progress` ruht, keine Anzeige in «Meine Wünsche», auf der Vorschauseite und unter Anfragen |
| Verweis | «Namen je Profil · Seerr-Benutzer»; «Zugang zu Radarr und Sonarr · Integrationen › Seerr» | – | – | nur Anzeige |

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
| Tabelle «Konten» | Spalte «Wünsche» | Chip «Erlaubt» (Recht und Seerr-Benutzer), «Nicht erlaubt» (Recht aus), «Ohne Seerr-Benutzer» (Recht an, Zustand «Wartet» oder «Nicht eröffnet») |
| Formular «Zugriff» | Schalter «Darf Titel anfordern» · Hilfe «Gilt als Grenze für alle Profile. Welche Profile anfordern, wählt das Hauptprofil des Kontos.» | an/aus; schreibt `accounts.can_request` über `updateAccountAccess` |
| Formular «Zugriff» | Feld «Limit» (neu): «Standard» (zeigt die Vorgabe, «10 Filme, 5 Staffeln in 7 Tagen»), «Eigenes» (dann je «Filme» und «Staffeln» Anzahl und Tage, 0 = ohne Limit), «Unbegrenzt» | einziger Ort des Limits je Konto (N-31); gespeichert mit «Zugriff speichern» (`setAccountWishLimit`); Slouchcat schreibt es in den Seerr-Benutzer. Ohne Benutzer: «Noch kein Seerr-Benutzer: Slouchcat setzt das Limit, sobald das Konto dort eröffnet ist.» Hat der Benutzer in Seerr `MANAGE_USERS`: Feld gesperrt mit «Kein Limit (Administrator)» (N-34) |
| Formular «Zugriff» | Zeile «Seerr-Benutzer: grosseltern · eröffnet · Passwort abgeglichen · Ändern» bzw. «Seerr-Benutzer: grosseltern · verknüpft beim Annehmen der Einladung · Ändern» | nur Anzeige, Link auf Wünsche › Seerr-Benutzer mit diesem Konto; geändert oder gelöst wird nur dort. Ohne Benutzer eröffnet das Speichern des Formulars nichts von selbst (K-15) |
| Formular «Zugriff» | Hinweis beim Ändern des Wunschrechts | «Die Rechte von «grosseltern» in Seerr werden angepasst.» bei eröffneten Benutzern; bei verknüpften «… behält in Seerr seine Rechte, weil Slouchcat ihn nicht eröffnet hat.» (N-9) |
| Dialog «Konto einladen» | Schalter «Darf Titel anfordern» | Vorgabe aus «Neue Konten dürfen anfordern» |
| Dialog «Konto einladen» | Feld «Limit» (neu): «Standard», «Eigenes», «Unbegrenzt» | geht beim Annehmen an das neue Konto (**Vorschlag**: Spalten an `invitations`, `08-datenmodell-api.md`, Abschnitt 2.3) |
| Dialog «Konto einladen» | Hinweis (neu) | «Beim Annehmen eröffnet Slouchcat das Konto auch in Seerr: gleiche E-Mail, gleiches Passwort.» (Wortlaut Mockup; nur wenn «Neue Konten in Seerr eröffnen» an ist). Gibt es dort schon einen Benutzer mit der Adresse, die der Gast eintippt, verknüpft Slouchcat ihn sofort; die Übersicht zeigt das sieben Tage lang (N-90, N-93). Der Gast sieht in beiden Fällen dieselbe Meldung (N-94) |
| Eigenes Konto, «Konto ändern» | Hinweis unter «Neues Passwort» (neu) | «Das neue Passwort gilt auch in Seerr.» Nach dem Speichern je nach Stand: «In Seerr folgt das neue Passwort bei deiner nächsten Anmeldung: Seerr ist gerade nicht erreichbar.» bzw. «Dein Passwort in Seerr bleibt, wie es ist: Du bist dort Administrator.» (N-5, N-6). Eine geänderte E-Mail geht ebenfalls an Seerr. Hat das Konto dort noch keinen Benutzer und gibt es die neue Adresse in Seerr schon, verknüpft Slouchcat ihn sofort, wie bei der Einladung (N-95). Hat das Konto schon einen Benutzer und lehnt Seerr die neue Adresse ab, weil sie vergeben ist, bleibt der Benutzer, wie er ist, und die Verwaltung sieht einen Hinweis am Konto (N-95). Die Meldung an das Konto ist in jedem Fall dieselbe und verrät nicht, ob Seerr die Adresse kennt (N-94) |
| Eigenes Konto, «Meine Profile» | Profildialog mit Schalter «Darf Titel anfordern», Chip auf der Karte | wie `/settings/profiles` (M-11) |
| Administratorkonten | Block «Wünsche» im Aufklappfeld mit Schalter «Darf Titel anfordern», Feld «Limit» und Zeile «Seerr-Benutzer» | Lücke in Anhang F, als Redaktionsentscheid vom 17. November 2026 gesetzt: Administratoren haben kein Formular «Zugriff» (`04-benutzer-profilmapping.md`, Abschnitt 9) |
| «Konto entfernen» | Zusatzsatz «Wünsche bleiben in Seerr bestehen. Der Seerr-Benutzer bleibt, verliert seine Rechte und bekommt ein neues Zufallspasswort.» nur bei einem Benutzer, den Slouchcat eröffnet hat (`origin = created`); bei einem verknüpften oder von Hand gewählten Benutzer «Der Seerr-Benutzer bleibt unverändert.» (Redaktionsentscheid zu K-2); die Wünsche des Kontos werden in Slouchcat geschlossen (`04-benutzer-profilmapping.md`, Abschnitt 10; N-10, F-31) | – |

## 9. Startseite › Spotlight (M-27)

**Heute** (Anhang A 3, alles bleibt): Kopf «Spotlight» mit Hilfe; Band «Quelle» mit den elf Feldern
(«Auf der Startseite zeigen», «Trakt-Liste», «Adresse der Liste», «Medienart», «Höchstens», «Ausblenden unter»,
«Abruf alle … Stunden», «Spotlight Wechsel Takt», Abrufzustand, «Jetzt abrufen», «Speichern»); Band «Einträge der
Liste» mit «Rang», «Titel», «Status», «Eigene Notiz», «Aktionen» («Anheften»/«Lösen», «Ausblenden»/«Einblenden»,
«Notiz»); Band «Verwaiste Anpassungen» mit «Entfernen»; Fussnote mit Link «Zur Seite «Startseite»». Rückfrage bei
ungespeicherten Änderungen.

**Neu** (Einzelheiten `05-spotlight-integration.md`, Abschnitt 4). Marco: «In Spotlight soll nicht angezeigt
werden was fehlt. Nur vorhandene Filme und Serien sollen auf der Startseite angezeigt werden. In der Verwaltung ist
es gut so» (Anhang H 8.1, E-5). Die Seite stellt darum nichts für fehlende Titel ein; sie zeigt sie nur (N-71,
N-72). Die Felder des Bands «Quelle» bleiben alle elf, ohne neue Hinweise.

| Element | Inhalt |
|---|---|
| Hilfe im Kopf | **Vorschlag:** «Die Startseite zeigt nur Titel aus der eigenen Bibliothek. Einträge der Liste, die fehlen, stehen nur hier; du kannst sie anfordern.» |
| Band «Fehlende Einträge» | zugeklappt; nur Anzeige, **ohne Schalter**. Kopf «2 fehlen · 1 angefordert». Liste der Einträge, die nicht in der Bibliothek stehen, mit Poster, Zustand («Noch nicht in der Bibliothek», «Angefordert · In Beschaffung») und «Anfordern»; Hinweis, wenn Wünsche nicht eingerichtet sind («Anfordern geht erst, wenn Wünsche eingerichtet sind.», wie in `05-spotlight-integration.md`, Abschnitt 4). Einträge, die sich nicht zuordnen lassen (N-62), gelten nicht als fehlend: Sie stehen in «Einträge der Liste» und im Band «Abgleich über TMDB, TVDB und IMDb». Die Schalter «Fehlende Titel zeigen», «Höchstens fehlende Titel» und «Reihenfolge» aus Fassung 3 vor der zweiten Rückmeldung entfallen (N-72) |
| Band «Einträge der Liste» | wie in Fassung 3: fehlende Einträge mit Poster, Status «Noch nicht in der Bibliothek» oder «Angefordert · In Beschaffung», Aktionen «Anheften», «Ausblenden», «Notiz», «Anfordern» (Anheften und Ausblenden wirken auf der Startseite erst, wenn der Titel in der Bibliothek steht). Neu je Eintrag die **IDs** «TMDB 1061474 · TVDB – · IMDb tt…» (aus Trakt, sonst TMDB `external_ids`, N-60) und, wenn kein Abgleich gelang, der Status «Nicht zuordenbar: nur TVDB-ID bekannt, bei TMDB nicht gefunden» ohne «Anfordern» (N-62) |
| Band «Abgleich über TMDB, TVDB und IMDb» (neu) | zugeklappt; nur Anzeige. Kopf «18 in der Bibliothek · 2 fehlen · 0 nicht zuordenbar». Text: «Filme gleicht Slouchcat über TMDB, dann IMDb ab. Serien wie die Bibliothek: TVDB zuerst, dann TMDB, dann IMDb. Seerr führt jeden Titel über die TMDB-ID und gibt bei Serien die TVDB-ID an Sonarr weiter.» (N-61, Anhang H 4.6). Darunter die Liste der nicht zuordenbaren Einträge |

**Vorschlag:** «Anfordern» in der Verwaltung fordert als Hauptprofil des Administrators an, über denselben
Dialog wie in der App.

## 10. Startseite › Startseite (M-28)

**Heute** (Anhang A 5.2, alles bleibt): Titel «Startseite» mit Hilfe; Band «Reihen» mit Tabelle «Platz»,
«Reihe», «Zustand», «Ausblenden», Ausklappzeile mit Beschreibung und Link; Grenze 15 sichtbare Reihen mit Meldung;
Band «Vorschau»; Speicherleiste «Speichern»/«Verwerfen»; Fussnote zu Spotlight.

**Neu (Marco, E-10; N-97):** Reihe «Erfüllte Wünsche» in der Tabelle, mit Platz und Ausblenden. Ausklappzeile: «Wünsche
des Profils, die in den letzten 14 Tagen erfüllt wurden, also Titel der Bibliothek. Offene Wünsche stehen unter
«Wünsche › Meine Wünsche». Leer erscheint die Reihe nicht.» (Wortlaut **Vorschlag**) und Link «Zur Seite
«Wünsche»». Marco hat die Reihe so gewählt (E-10, F-34 entschieden); sie zeigt nur Titel der Bibliothek, wie
sein Satz «Nur vorhandene Filme und Serien sollen auf der Startseite angezeigt werden» es verlangt (N-74). Die Migration setzt die Reihe ab Werk direkt nach «Weiterschauen» (wie im Mockup); sind schon
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

**Neu:** zwölf Jobarten (acht aus Fassung 2, vier aus Fassung 3). Gruppen, Pools und Prioritäten sind **Vorschlag**
aus `03-architektur.md`, Abschnitt 13.

| Jobart | Anzeigename (Vorschlag) | Gruppe | Auslöser | Takt |
|---|---|---|---|---|
| `wishes.submit` | «Wunsch übermitteln» | Zustellung | Ereignis | – |
| `wishes.withdraw` | «Wunsch in Seerr zurückziehen» | Zustellung | Ereignis | – |
| `wishes.sync` | «Wünsche abgleichen» | System | Zeitplan, «Jetzt abgleichen», Webhook; der Erstabgleich läuft als eigener Lauf mit Fortschritt («Seite 12 von 60») | 5 min, einstellbar |
| `wishes.import` | «Anfragen aus Seerr übernehmen» (Stufe 4) | System | Seerr-Benutzer verknüpft, von Hand gesetzt oder geändert, «Anfragen übernehmen» | – |
| `wishes.reconcile` | «Verfügbare Wünsche prüfen» | System | Zeitplan, nach dem Match | 1 h |
| `seerr.users` | «Seerr-Benutzer laden» | System | Zeitplan, Knopf | 24 h |
| `seerr.provision` | «Konten in Seerr eröffnen» (Stufe 1) | System | Einladung angenommen, Wunschrecht, Limit oder Passwortmodus geändert, Konto gelöscht, «In Seerr eröffnen» und «Alle eröffnen», Abgleich für wartende Konten. Beschreibung (**Vorschlag**): «Konten in Seerr eröffnen und Rechte, Limit, Passwort nachziehen. Ein Benutzer mit derselben E-Mail wird verknüpft.» | – |
| `wishes.tag` | «Profil-Tag setzen» (Stufe 4) | System | nach jedem Abgleich, Wunsch «In Beschaffung», neuer Mitwunsch. Beschreibung (Mockup): «Hängt den Tag des Profils an den Titel, sobald Seerr ihn an Radarr oder Sonarr übergeben hat, und holt Tags nach, die nicht gesetzt werden konnten. Ändert sonst nichts in Radarr und Sonarr.» | mit dem Abgleich |
| `wishes.progress` | «Download-Fortschritt lesen» (Stufe 4) | System | Zeitplan; läuft nur, solange Wünsche in Beschaffung sind und «Download-Fortschritt zeigen» an ist. Beschreibung (**Vorschlag**): «Liest die Warteschlange und den Dateistand der gewünschten Titel in Radarr und Sonarr für Wünsche in Beschaffung. Ändert nichts in Radarr und Sonarr.» | 1 min |
| `wishes.discover` | «Wünsche-Seiten vorladen» (Stufe 5) | Metadaten | Zeitplan; ruht, wenn die Seiten aus sind | 1 h |
| `titles.refresh` | «Fehlende Titel auffrischen» | Metadaten | Zeitplan nach `trakt.refresh` | 24 h |
| `series.catalog` | «Staffelliste der Serien» | Metadaten | Ereignis und Zeitplan | 7 d |

«Zeitplan von X» für `wishes.sync` ist der einzige Ort des Abgleichtakts. **Vorschlag:** Wird `wishes.submit`
pausiert, zeigt der Dialog: «Wünsche bleiben «Wird übermittelt»; nach 24 Stunden gelten sie als fehlgeschlagen.»

## 13. Status und Diagnose

| Frage des Administrators | Ort |
|---|---|
| Ist Seerr verbunden? | Integrationen (Kachel), Integrationen › Seerr Band «Verbindung», Übersicht bei Fehler |
| Ist es noch dieselbe Seerr-Instanz? | Integrationen › Seerr, Prüfpunkt «Instanz»; Übersicht «Andere Seerr-Instanz erkannt» |
| Wann gelang der letzte Abgleich? | Integrationen › Seerr Band «Abgleich»; Hintergrundjobs `wishes.sync` |
| Welche Aufrufe sind gescheitert? | Integrationen › Seerr Band «Protokoll» |
| Welche Konten haben noch keinen Seerr-Benutzer? | Übersicht «n Konten ohne Seerr-Benutzer»; Wünsche › Seerr-Benutzer, Zustand «Wartet» oder «Nicht eröffnet»; Konten Spalte «Wünsche» |
| Welche Konten wurden zuletzt beim Annehmen einer Einladung oder nach einer E-Mail-Änderung mit einem bestehenden Seerr-Benutzer verknüpft? | Übersicht «Neu verknüpft» (sieben Tage); Wünsche › Seerr-Benutzer, im Band des Kontos «Zustand» mit Auslöser und Datum; Protokoll (N-93) |
| Welche Seerr-Benutzer fehlen oder dürfen nicht anfordern? | Wünsche › Seerr-Benutzer, Warnungen |
| Wie kam ein Konto zu seinem Seerr-Benutzer, und ist das Passwort abgeglichen? | Wünsche › Seerr-Benutzer, im Band des Kontos «Zustand» und «Passwort»; Konten, Zeile «Seerr-Benutzer» |
| Kann man sich in Seerr mit dem Login von Slouchcat anmelden? | Integrationen › Seerr, Prüfpunkt «Lokale Anmeldung in Seerr»; Band «Benutzer und Limit» |
| Welches Konto ist am Limit, und wann wird wieder ein Platz frei? | Übersicht «Limit erreicht»; Wünsche › Anfragen, Band «Limit je Konto»; Wünsche › Seerr-Benutzer, im Band des Kontos «Limit» |
| Welche Profil-Tags fehlen in Radarr oder Sonarr, und warum? | Übersicht «n Wünsche ohne Profil-Tag»; Anfragen, Filter «Tag nicht gesetzt» und Diagnose; Integrationen › Seerr, Band «Radarr und Sonarr (Profil-Tags und Fortschritt)»; Hintergrundjobs `wishes.tag` |
| Wie weit ist der Download eines Wunsches, und warum hängt er? | Anfragen, Spalte «Zustand» (Fortschritt), Filter «Download verzögert», Aufklappzeile mit der Meldung von Radarr oder Sonarr; Übersicht «n Downloads verzögert»; Hintergrundjobs `wishes.progress` |
| Kann Slouchcat den Fortschritt lesen? | Integrationen › Seerr, Band «Radarr und Sonarr (Profil-Tags und Fortschritt)», Fakt «Download-Fortschritt» und Prüfpunkte «Radarr/Sonarr für Profil-Tags und Fortschritt»; Anfragen «Fortschritt nicht lesbar» |
| Kommen die Webhooks von Radarr und Sonarr an? | Integrationen › Webhooks von Radarr und Sonarr, wie heute (Meldungsprotokoll, «Letzte Meldung»); Übersicht «stille Webhooks» |
| Zählen Seerr, Sonarr und Slouchcat die Staffeln gleich? | Integrationen › Seerr, Prüfpunkt «Staffeln aus TVDB» |
| Warum gilt ein Spotlight-Eintrag als nicht zuordenbar? | Startseite › Spotlight, Band «Abgleich über TMDB, TVDB und IMDb» |
| Welche Einträge der Trakt-Liste fehlen in der Bibliothek? | Startseite › Spotlight, Band «Fehlende Einträge» (nur hier; die Startseite zeigt sie nicht) |
| Welche Anfragen gehören keinem Profil? | Wünsche › Anfragen, Filter «Nicht zuordenbar»; Zähler im Band «Abgleich» |
| Welche Wünsche warten auf Übermittlung oder sind gescheitert? | Wünsche › Anfragen, Filter «Wird übermittelt» und «Fehler»; Aufklappzeile «Diagnose»; Fehlschläge `wishes.submit` auf Hintergrundjobs |
| Welche Wünsche warten auf Freigabe? | Übersicht; Bereichsreiter «Wünsche» (Zähler); Anfragen, Filter «Freigabe ausstehend» |
| Warum bekam ein Profil keine Meldung? | Anfragen › Diagnose («für Noah nicht sichtbar»); Kanalseite des Kontos; fehlgeschlagene `notify.deliver` auf Hintergrundjobs |
| Warum gilt ein Titel nicht als verfügbar? | Anfragen › Diagnose («In Seerr verfügbar, in Slouchcat nicht gefunden» mit Link zur Pflege, `03-architektur.md`, Abschnitt 11) |

## 14. Offene Entscheide in diesem Papier

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-16 | Freigeben/Ablehnen in der Verwaltung in Stufe 4? | Ja |
| F-18 | Seerr-Webhook als Sofortmeldung in Stufe 4? | Ja |
| F-19 | Seite «Wünsche» unter Medien, Verbindung unter Integrationen? | Ja |
| F-4 | Konten ohne Seerr-Benutzer (Sammelbenutzer)? | nicht erlauben als Vorgabe; betrifft seit Fassung 3 nur Konten, die noch keinen Benutzer haben |
| F-23 | Für wen eröffnet Slouchcat einen Seerr-Benutzer? | für jedes Konto; von selbst nur beim Annehmen einer Einladung, bestehende Konten eröffnet der Administrator mit «Alle eröffnen» oder einzeln (K-15); ohne Wunschrecht ohne Rechte |
| F-24 | Passwort bei verknüpften Benutzern übernehmen? | Die Verknüpfung bei der Einladung hat Marco entschieden: sofort (E-8). Offen als Empfehlung: Passwort auch bei Konten übernehmen, die ein Administrator nachträglich eröffnet oder verknüpft, bei der nächsten Anmeldung, ausser Administratoren und Anmeldung über Jellyfin, Emby oder Plex (K-11); Hinweis «Neu verknüpft» in der Übersicht (N-93) |
| F-25 | Name des Profil-Tags | `sc-<profilname>`, Präfix einstellbar, je Profil überschreibbar; nie entfernen |
| F-26 | Limit ab Werk | 10 Filme und 5 Staffeln in 7 Tagen; Administratoren ohne Limit |
| F-27 | Ausnahme «Profil sendet als anderer Seerr-Benutzer» behalten? | ja, zugeklappt |
| F-30 | Radarr und Sonarr (Profil-Tag und Fortschritt): Zugang aus Seerr lesen oder eigene Felder? | aus Seerr, eigene Adresse nur als Ausweg |
| F-31 | Seerr-Benutzer eines gelöschten Kontos? | bleibt; nur ein von Slouchcat eröffneter verliert die Rechte und bekommt ein Zufallspasswort, verknüpfte und von Hand gewählte bleiben unverändert (K-2) |

Was Marco am 17. November 2026 entschieden hat (E-1 bis E-4: gleiches Passwort, Tag in Radarr und Sonarr,
Seerr zählt alles, Verknüpfen über die E-Mail; E-5 bis E-7: keine fehlenden Titel im Spotlight und nirgends
in der Zuschauer-App, Webhooks wie heute, Download-Fortschritt bei Wünschen; E-8 bis E-11: bei der Einladung
sofort verknüpfen, Name «Seerr», Startreihe «Erfüllte Wünsche», Sofort-Import nach Messung), steht in
`11-entscheide-und-adr.md`, Abschnitt 0. Entschieden sind damit F-8 (E-5: Das Band «Fehlende Einträge» hat keine
Schalter), F-32 (E-9), F-33 (E-11; ändert auf diesen Seiten nur zwei Spalten im Meldungsprotokoll der Webhooks,
und nur, wenn die Messung einen Gewinn zeigt) und F-34 (E-10); F-20 ist durch F-34 ersetzt. Die übrigen Punkte
dieses Papiers aus Fassung 3 sind Empfehlungen der Redaktion.

Begründungen und Folgen anderer Wahl: `11-entscheide-und-adr.md`, Abschnitt 1.
