# Konzept 25 · Abnahmekriterien

> **Kurz gesagt.** Jedes Kriterium nennt Voraussetzung, Handlung, erwartetes Ergebnis und wie es geprüft wird:
> mit einem automatischen Test («Test») oder von Hand auf CT 190 mit echtem Jellyseerr («Hand»). Die
> Kriterien sind nach Stufe und Bereich gruppiert und durchnummeriert (AK-1 …). Eine Stufe gilt als
> abgenommen, wenn alle ihre Kriterien erfüllt sind und `task ci` grün ist. Am Schluss steht, was Marco am
> Mockup prüft, bevor gebaut wird.

Grundlage: [Anhang F](anhang/F-entscheid-blatt.md), `03-architektur.md`, `08-datenmodell-api.md`, `09-umsetzungsplan.md`. Beispieldaten
nach Anhang F Abschnitt 11 (Konten marco, grosseltern, gast; Profile Marco, Sandra mit PIN, Noah 12, Mia 6;
Jellyseerr-Benutzer 1 marco bis 5 wohnzimmer-tv). «Mock» heisst Mock-Jellyseerr `seerrtest` über `httptest`.
Messgrössen mit **Vorschlag** sind Zielwerte dieses Papiers, nicht von Anhang F. Kriterien, die nach der ersten
Kritik dazukamen, tragen die Nummern ab AK-105 und stehen bei ihrer Stufe.

## 1. Stufe 0: Papiere

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-1 | Marco hat F-1 bis F-22 und T-1, T-2 beantwortet | Frage 33, ADR 0037, ADR 0038 lesen | Frage 33 ändert Spec `:35` und `:46`, hebt `:66` und `:67` in Teilen auf, bestätigt `:65`, `:68` und `:69`, löst S-4 ab; beide ADR verweisen darauf | Hand (Marco) |
| AK-2 | AK-1 | CLAUDE.md Regel 11 lesen | Wortlaut erlaubt Wünsche über Jellyseerr nach ADR 0037 und verbietet weiter die Steuerung von Radarr/Sonarr | Hand |

## 2. Stufe 1: Fundament

### 2.1 Verbindung

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-3 | Jellyseerr unter `http://192.168.x.y:5055` | Adresse und Schlüssel eintragen, «Verbindung testen» | Prüfliste: erreichbar, Schlüssel gültig, Version, «Handeln im Namen von Benutzern», CSRF-Schutz aus, Radarr und Sonarr; alle grün | Hand; Test mit Mock |
| AK-4 | AK-3 | Schlüssel absichtlich falsch | Zustand «Schlüssel ungültig»; jeder Lauf von `wishes.sync` ruft nur `GET /auth/me`, sonst kein Aufruf; fehlende Titel bleiben für Zuschauer sichtbar; kein Schlüssel im Protokoll | Test (Mock 403) |
| AK-5 | Jellyseerr gestoppt | «Verbindung testen» | Zustand «Nicht erreichbar» innert 10 s; Fehlertext nennt nur Schema und Host | Test (Zeitlimit) |
| AK-6 | Adresse antwortet mit 302 | Test | gilt als Fehler, keine Weiterleitung gefolgt, Schlüssel nicht an das Ziel gesendet | Test |
| AK-7 | Mock ohne Wirkung von `X-API-User` | Test, dann «Wünsche aktiv» einschalten | Prüfpunkt «Handeln im Namen von Benutzern» rot mit Hinweis; «Wünsche aktiv» lässt sich nicht einschalten, API weist ab (**Vorschlag** T-1) | Test |
| AK-8 | CSRF-Schutz in Jellyseerr an (Mock: 403 auf POST) | Test | Prüfpunkt «CSRF-Schutz aus» rot mit Abhilfe | Test; Hand einmal gegen echtes Jellyseerr (Prüfweg ist abgeleitet) |
| AK-9 | Schlüssel gespeichert | Seite neu laden | Feld leer mit «…a1b2»; die API liefert den Schlüssel nie (`writeOnly`) | Test (Vertrag) |
| AK-10 | Schlüssel in `deploy/.env` | Ansicht öffnen | Feld gesperrt, Hinweis «In deploy/.env gesetzt» (**Vorschlag** Umgebungsvariable) | Test |
| AK-105 | Schlüssel gespeichert für `192.168.1.50` | andere Adresse eintragen, Schlüsselfeld leer, «Verbindung testen» | 422 `seerr_key_required`; der gespeicherte Schlüssel geht nicht an die neue Adresse; bei `http` ins Internet Warnung | Test (Mock zählt) |
| AK-106 | Jellyseerr 2.6 im Mock | Test | Prüfpunkt Version als Warnung «nicht geprüft», kein Fehler | Test |
| AK-107 | Verbindung eingerichtet, Fingerabdruck gespeichert | Adresse zeigt auf eine andere Instanz (Mock 2) | «Andere Jellyseerr-Instanz erkannt»; kein `POST`, kein `DELETE`, kein Abgleich bis «Zuordnung bestätigen» oder «Zuordnung löschen»; neue Wünsche bleiben `queued`; nach «Bestätigen» läuft ein Erstabgleich | Test |

### 2.2 Zuordnung

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-11 | Verbindung steht | «Benutzerliste neu laden» | fünf Benutzer mit Rechten, Auto-Freigabe, Kontingent | Hand; Test |
| AK-12 | Konto grosseltern, Benutzer 4 mit gleicher E-Mail | Reiter «Zuordnung» öffnen | Vorschlag «grosseltern» mit «Übernehmen»; ohne Klick bleibt das Konto unzugeordnet; «Übernehmen» wirkt sofort mit Meldung «Zuordnung geändert» | Test |
| AK-13 | Benutzer 1 gehört Konto marco | Benutzer 1 einem anderen Konto zuordnen | Auswahl gesperrt; API 409 `seerr_user_taken` | Test |
| AK-14 | Profil Noah «Eigener Benutzer» kinder | Wunsch-Benutzer auflösen | `kinder`; Profil Marco «Wie Konto» → `marco` | Test |
| AK-15 | Benutzer in Jellyseerr gelöscht | `seerr.users` laufen lassen | Warnung «Benutzer in Jellyseerr gelöscht»; betroffene Profile `canRequest = false`, Grund `seerr_user_gone` | Test |
| AK-16 | «Alle Vorschläge übernehmen» | bestätigen | alle Paare in einem Zug übernommen oder bei Konflikt keines | Test |
| AK-108 | Profil Noah «kinder», 2 offene Wünsche | Noah im Auswahlfeld auf «Wie Konto» stellen | sofort wirksam, Meldung «Zuordnung geändert. 2 offene Wünsche bleiben bei kinder.»; die offenen Wünsche behalten `seerr_user_id` 3; ein neuer Wunsch geht als marco | Test |
| AK-109 | «Konten ohne Zuordnung» = Sammelbenutzer wohnzimmer-tv | Gast (Konto mit Recht, ohne Zuordnung) fordert an | gesendet als wohnzimmer-tv; das Konto sieht fehlende Titel; ohne Sammelbenutzer sieht es keine | Test |

### 2.3 Wunschrecht und Einstellungen

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-17 | frische Migration auf bestehender DB | Konten und Profile ansehen | Besitzerkonto an, andere Konten aus; Hauptprofile an, weitere aus | Test (Migration, eingefrorener Stand) |
| AK-18 | Konto gast «Darf Titel anfordern» aus | Hauptprofil Gast setzt im Profildialog an | Schalter gesperrt mit «Das Konto darf keine Titel anfordern.»; API weist ab | Test |
| AK-19 | Weiteres Profil (Mia) aktiv | Profildialog | kein Zugriff (nur Hauptprofil verwaltet) | Test |
| AK-20 | jede Einstellung aus Anhang F 4 | Seite «Einstellungen» und andere Seiten durchgehen | jede Einstellung hat genau einen Ort, andere Seiten verlinken | Hand |
| AK-21 | «Wünsche aktiv» aus (ab Werk) | Sitzung lesen | `wishes.enabled = false`; Zuschauer sehen nichts Neues | Test |
| AK-110 | Seite «Einstellungen» | einen Schalter umlegen | wirkt sofort ohne «Speichern», Meldung bestätigt; Neuladen zeigt den neuen Wert | e2e |
| AK-111 | «Jellyseerr verwenden» aus, ein Wunsch `queued` seit 23 h | 2 h warten (Uhr-Mock) | kein Aufruf an Jellyseerr; Zuschauer sehen keine fehlenden Titel; der Wunsch bleibt `queued` (Frist ruht, **Vorschlag** T-2) | Test |
| AK-112 | Administratorkonto ohne Formular «Zugriff» | Aufklappfeld auf «Konten» | Block «Wünsche» mit «Darf Titel anfordern» und «Jellyseerr-Benutzer» | e2e |

### 2.4 Sicherheit Stufe 1

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-22 | Debug-Log an | Test, Speichern, Benutzerliste | Log enthält weder `X-Api-Key` noch `seerr.api_key` im Klartext | Test (Redaction) |
| AK-23 | Konto ohne Administratorrolle | `/admin/seerr…`, `/admin/wishes…` aufrufen | 403 `forbidden`; jede Operation steht im Vertragstest | Test |

## 3. Stufe 2: Anfordern

### 3.1 Jugendschutz und Sichtbarkeit

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-24 | Film fehlt, TMDB-Freigabe 16 | Profil Noah (12) ruft Vorschauseite, Bild und `createWish` auf | 404 überall; Suche zeigt ihn nicht | Test |
| AK-25 | Film fehlt, keine Freigabe | Profil Noah | 404 (unbekannt = 18) | Test |
| AK-26 | Film steht in der Bibliothek, für Noah zu hoch | Noah sucht ihn | nicht als «Noch nicht in der Bibliothek»; Vorschauseite 404 | Test |
| AK-27 | Profil Mia ohne Recht | Suche, Vorschauseite per Link | kein Abschnitt «Noch nicht in der Bibliothek»; Vorschauseite 404 (F-9) | Test |
| AK-28 | Freigabeland CH ohne Wert, TMDB-Freigaben DE 12, US 16 | Prüfung | 12 (Länderreihenfolge nach ADR 0034, erstes Land mit bekanntem Wert, wie bei einem Bibliothekstitel) | Test |
| AK-113 | Konto grosseltern mit Recht, ohne Zuordnung | Startseite, Suche, Vorschauseite | keine fehlenden Titel, Vorschauseite 404, kein Hinweis für den Zuschauer; die Übersicht der Verwaltung zeigt «1 Konto ohne Zuordnung» | Test |
| AK-114 | Noah hat einen abgelehnten Wunsch zu X; danach nimmt der Administrator Noah das Recht | Vorschauseite von X | Seite erreichbar, ohne Anforderungsknopf, mit dem letzten Wunsch; der Titel fehlt in Suche und Spotlight | Test |
| AK-115 | Profil Noah (12), TMDB-Suche liefert 25 Treffer | Suche | Details nur für die ersten 20 Treffer geholt und in `external_titles` gespeichert; Treffer ohne bekannte Freigabe fehlen; Profil Marco (18) löst keinen Detailabruf aus | Test (Mock zählt) |

### 3.2 Anfordern

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-29 | Profil Marco, Jellyseerr erreichbar | «Film anfordern», Dialog bestätigen | Dialog nennt «Wird als marco an Jellyseerr übermittelt» und «Wird sofort beschafft»; Zustand «In Beschaffung»; in Jellyseerr ein Request von marco | Hand; Test (Mock) |
| AK-30 | Profil Noah | anfordern | Hinweis «Braucht eine Freigabe»; Zustand «Freigabe ausstehend»; Request in Jellyseerr gehört **kinder**, Status PENDING | Hand; Test |
| AK-31 | Mock | Anfrage mitschneiden | `POST /request` mit `X-API-User`, **ohne** `userId`; bei Serien `tvdbId` und Zahlenliste `seasons` | Test |
| AK-32 | Jellyseerr gestoppt | anfordern | Antwort ≤ 3,5 s, Zustand «Wird übermittelt»; nach dem Start von Jellyseerr wird der Wunsch ohne Zutun übermittelt | Test (Uhr-Mock); Hand |
| AK-33 | Jellyseerr 24 h weg | Uhr vorstellen | `failed`, `unreachable_24h`; Ereignis `errors` an Administratoren | Test |
| AK-34 | erster POST kam an, Antwort ging verloren | Job erneut ausführen | kein zweiter Request in Jellyseerr; Wunsch hängt am ersten | Test |
| AK-35 | Kontingent von kinder erschöpft (Mock 403) | Noah fordert an | «Fehlgeschlagen» mit «Kontingent für Filme erschöpft» | Test |
| AK-36 | Titel in Jellyseerr gesperrt | anfordern | «Fehlgeschlagen», Grund Sperrliste; für 2.x und 3.x Text erkannt | Test |
| AK-37 | Titel in Jellyseerr schon von wohnzimmer-tv angefordert (409) | Marco fordert an | Erfolg, Wunsch hängt am bestehenden Request | Test |
| AK-38 | Serie, alle Staffeln in Jellyseerr schon angefragt (202) | anfordern | Erfolg, angehängt bzw. «wird eingelesen» | Test |
| AK-39 | Serie ohne TVDB-ID bei TMDB | Vorschauseite | kein Knopf, Hinweis; `createWish` 403 `wish_not_allowed`, Grund `tvdb_id_missing` | Test |
| AK-40 | Staffelwahl | Dialog | Staffel 0 und Staffeln ohne Folgen nicht wählbar; «Alle fehlenden Staffeln» vorgewählt | Test; Hand |
| AK-41 | Profil sendet 21 neue Wünsche in einer Stunde | 21. Wunsch | 429 `too_many_attempts` mit `Retry-After` (**Vorschlag** 20/h) | Test |

### 3.3 Mitwunsch, Zurückziehen, Idempotenz

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-42 | Marco hat Film X gewünscht | Sandra öffnet X | Hinweis «Schon angefordert», Knopf «Benachrichtige mich»; danach Mitwunsch, **kein** Aufruf an Jellyseerr | Test (Mock zählt 0 Aufrufe) |
| AK-43 | AK-42 | Sandra sieht «Meine Wünsche» | Wunsch ohne Angabe, wer sonst gewünscht hat | Test |
| AK-44 | Marco fordert X zweimal an | zweiter Aufruf | 200 mit demselben Wunsch, kein zweiter Wunsch | Test |
| AK-45 | Datenbank | zweiten offenen Filmwunsch direkt einfügen | Teilindex verweigert | Test |
| AK-46 | Noahs Wunsch «Freigabe ausstehend» | «Zurückziehen» | sofort «Zurückgezogen»; danach löscht `wishes.withdraw` den Request in Jellyseerr | Test; Hand |
| AK-116 | Sofortversuch läuft ins Zeitlimit, der Request kam in Jellyseerr aber an | Profil zieht sofort zurück | danach kein Request dieses Benutzers in Jellyseerr; keine Beschaffung | Test (Mock) |
| AK-117 | Request in Jellyseerr schon freigegeben, Slouchcat zeigt noch «Freigabe ausstehend» | Zurückziehen | `DELETE` antwortet 401; Wunsch wieder «In Beschaffung», Hinweis «War schon freigegeben» | Test |
| AK-118 | Sandra ist Mitwünscherin bei Marcos Film | Sandra zieht zurück | nur Sandras Zeile fällt weg, kein Aufruf an Jellyseerr; Marcos Wunsch bleibt | Test |
| AK-119 | Marco ist Besitzer, Sandra Mitwünscherin, Zustand «Freigabe ausstehend» | Marco zieht zurück | Sandra wird Besitzerin; der Request in Jellyseerr bleibt | Test |
| AK-120 | «Bereits angeforderte Titel» = «Nur Zustand zeigen» | Sandra öffnet Marcos gewünschten Film | Zustand sichtbar, kein Knopf «Benachrichtige mich»; `createWish` 403 `wish_not_allowed` mit `status_only` | Test |
| AK-121 | Marco und Noah fordern denselben Film gleichzeitig an | zwei parallele `createWish` | ein Wunsch, ein Mitwunsch, ein `POST` an Jellyseerr | Test |
| AK-122 | Serie: Staffel 3 hat in Jellyseerr schon einen fremden Request | Noah fordert Staffeln 2 bis 4 an | Staffeln 2 und 4 tragen die neue Request-ID, Staffel 3 die fremde; Zustand je Staffel richtig | Test (Mock) |
| AK-47 | Wunsch «In Beschaffung» | Zurückziehen | Knopf fehlt; API 409 `wish_not_withdrawable` | Test |
| AK-48 | Wunsch eines anderen Profils | `withdrawWish` mit dessen ID | 404 | Test |

### 3.4 Abgleich

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-49 | Noahs Wunsch PENDING | in Jellyseerr freigeben, ≤ 5 min warten | «In Beschaffung» | Hand; Test |
| AK-50 | in Jellyseerr abgelehnt | Abgleich | «Abgelehnt», ohne Grund | Test |
| AK-51 | Request in Jellyseerr gelöscht | Abgleich nach 24 h ohne Änderung | `failed`, `seerr_request_gone` | Test |
| AK-52 | Abgleich bricht nach Seite 2 ab | nächster Lauf | liest zuerst die neuen Änderungen, setzt dann an Seite 3 fort; keine doppelten Verlaufseinträge oder Meldungen | Test |
| AK-53 | 1000 Requests in Jellyseerr, 3 geändert seit letztem Lauf | Abgleich | höchstens eine Seite (`take=50`) abgerufen | Test (Mock zählt) |
| AK-123 | 3000 Requests, noch nie abgeglichen | Verbindung speichern | Erstabgleich ohne Seitengrenze im Hintergrund mit Fortschritt unter Hintergrundjobs; danach ist der Zeiger gesetzt und der regelmässige Abgleich liest höchstens eine Seite | Test (Mock zählt) |
| AK-124 | Wunsch wartet 3 Tage auf Freigabe | Abgleich alle 5 min | `GET /request/{id}` für diesen Wunsch höchstens einmal je 24 h | Test (Mock zählt) |

### 3.5 Verfügbarkeit

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-54 | Film X gewünscht, Radarr importiert | Webhook, Scan, Match | Zustand «Verfügbar» erst nach dem Match; Link führt auf `/items/{id}` | Hand; Test |
| AK-55 | Jellyseerr meldet AVAILABLE, Slouchcat hat den Titel nicht | Abgleich | Zustand «In Beschaffung · wird eingelesen», keine Meldung | Test |
| AK-56 | Film nach Scan ohne Freigabe, Noah wünschte ihn | Match setzt Freigabe 6 | Meldung an Noah nach dem Match bzw. spätestens nach dem stündlichen Nachlauf | Test |
| AK-57 | Serie TVDB-zuerst ohne `tmdb_id` in der Bibliothek | Wunsch mit TMDB-ID | Treffer über TVDB/IMDb, «Verfügbar» | Test |
| AK-58 | Wunsch `failed`, Titel kommt auf anderem Weg | Nachlauf | «Verfügbar» (**Vorschlag**) | Test |
| AK-59 | Jellyseerr 48 h «verfügbar», kein Treffer | Verwaltung | Diagnose «In Jellyseerr verfügbar, in Slouchcat nicht gefunden» (**Vorschlag**) | Test |
| AK-125 | Serie gewünscht, Staffel 1 mit 10 Folgen laut TMDB; erst 1 Folge eingelesen | Match | Staffel bleibt «In Beschaffung»; erst mit 10 abspielbaren Folgen verfügbar; Jellyseerr-Zustand spielt keine Rolle | Test |

### 3.6 Benachrichtigungen (Stufe 2)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-60 | Wunsch wird verfügbar, Marco und Sandra (Mitwunsch) | Meldung | je ein Eingangseintrag; Kanalmeldungen des Kontos mit `[Marco]` bzw. `[Sandra]` | Test |
| AK-61 | AK-60, Sandra hat PIN | E-Mail lesen | «[Sandra] Ein Wunsch ist jetzt verfügbar.» ohne Titel (F-15) | Test; Hand |
| AK-62 | derselbe Übergang zweimal verarbeitet | Nachlauf erneut | keine zweite Meldung (`wish:<id>:available`) | Test |
| AK-63 | neues Konto und bestehende Konten nach der Migration von Stufe 2 | Kanäle ansehen | `wish_available` nach F-14 an, bei neuen und bei bestehenden Konten; `enabled` unverändert | Test (eingefrorener Stand) |
| AK-126 | Titel verfügbar gemeldet, Eingangseintrag gelesen und nach 90 Tagen weggeräumt; Titel gelöscht und neu eingelesen | Nachlauf | keine zweite Meldung (`notified_at`) | Test |
| AK-127 | Profil Noah mit offenem Wunsch wird gelöscht | Profil löschen | der Wunsch gehört dem Hauptprofil Marco, Verlauf «von gelöschtem Profil übernommen» | Test |
| AK-128 | Konto grosseltern mit offenem Wunsch, ohne Mitwünscher | Konto entfernen | Wunsch geschlossen (`failed`, `account_deleted`), sperrt den Titel nicht mehr; Request bleibt in Jellyseerr; Rückfrage nennt «Wünsche bleiben in Jellyseerr bestehen.» | Test |
| AK-64 | Einreichen eines Wunsches | – | keine Benachrichtigung, nur Rückmeldung am Bildschirm | Test |

### 3.7 Verwaltung Stufe 2

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-65 | Wünsche in allen Zuständen | Reiter «Anfragen» | Kennzahlen, Filter, Spalte «Gewünscht von Konto · Profil» mit Mitwünschern, Verlauf in der Aufklappzeile | Hand; e2e |
| AK-66 | Wunsch `failed` (Übermittlung) | «Erneut übermitteln» | Wunsch wieder `queued`, danach übermittelt | Test |
| AK-67 | offene Wünsche | Übersicht | Kennzahl «Offene Wünsche»; «Jellyseerr nicht erreichbar» bei Ausfall | e2e |
| AK-68 | – | Seite «Hintergrundjobs» | Jobarten `wishes.submit`, `wishes.withdraw`, `wishes.sync`, `wishes.reconcile`, `seerr.users`, `titles.refresh` mit Texten; «Jetzt ausführen» geht; Erstabgleich mit Fortschritt | e2e |

## 4. Stufe 3: Spotlight

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-69 | «Fehlende Titel zeigen» aus (ab Werk) | Startseite | keine fehlenden Titel, Verhalten wie 1.40 | Test |
| AK-70 | an, höchstens 5 | Startseite Profil Marco | höchstens 5 fehlende unter 20; Chip «Noch nicht in der Bibliothek»; Knopf «Film anfordern» | Test; Hand |
| AK-71 | AK-70 | erste Folie | immer ein vorhandener Titel; Prepaint und Vorladen unverändert | Test |
| AK-72 | «Ausblenden unter» 5, nur 4 vorhandene | Startseite | Reihe ausgeblendet (fehlende zählen nicht) | Test |
| AK-73 | Profil Mia ohne Recht | Startseite | keine fehlenden Titel | Test |
| AK-129 | «Anfordern aus dem Spotlight» aus | Startseite und `/lists/spotlight` | keine fehlenden Titel; Band «Fehlende Titel» der Verwaltung zeigt den Hinweis | Test |
| AK-130 | Marcos Wunsch zu X ist abgelehnt | Startseite | X erscheint wieder als fehlender Titel mit «Film anfordern»; die Vorschauseite nennt «Am … abgelehnt» | Test; Hand |
| AK-131 | Marcos Wunsch zu X wurde gestern erfüllt | Startseite Marco und Sandra | Marco sieht den Chip «Dein Wunsch ist da» (bis 14 Tage nach der Erfüllung), Sandra ohne Wunsch nicht | Test |
| AK-74 | Profil Noah, fehlender Titel ab 16 | Startseite | erscheint nicht | Test |
| AK-75 | Trakt-Eintrag einer TVDB-zuerst-Serie in der Bibliothek | Startseite | als vorhanden gezeigt, nicht als fehlend | Test |
| AK-76 | Posterreihe | fehlendes Poster | ohne Gesehen-Haken und ohne Merken; runder Plus-Knopf; Unterzeile «2026 · Noch nicht da» | Storybook; Hand |
| AK-77 | `/lists/spotlight` | Filter «Noch nicht da» | nur fehlende Titel des Profils, alle erlaubten ohne den Deckel N, nur der Medienart des Spotlights | e2e |
| AK-78 | Reihe «Deine Wünsche» | Startseite mit offenen Wünschen | Reihe zeigt sie, ab Werk direkt nach «Weiterschauen»; in der Verwaltung ausblendbar und verschiebbar | e2e |
| AK-79 | Verwaltung Spotlight | Band «Fehlende Titel» | Schalter, Höchstzahl 0–10, Reihenfolge, Zustand «7 fehlende dürften erscheinen; 3 angefordert»; alle elf Felder des Bands «Quelle» unverändert | Hand; e2e |

## 5. Stufe 4: Serien und Komfort

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-80 | Tulsa King, 2 von 4 Staffeln da, Profil Marco | Serienseite | Zeile «2 von 4 Staffeln vorhanden», gedämpfte Karten für 3 und 4, «Staffeln anfordern»; Profil Mia ohne Recht sieht weder die Karten noch die Zeile | Hand; e2e |
| AK-132 | Staffel 3 hat Sandra schon angefordert | Marco öffnet den Dialog | Staffel 3 wählbar mit «Bereits angefordert»; Marco wird für Staffel 3 Mitwünscher, nur Staffel 4 geht an Jellyseerr | Test; e2e |
| AK-81 | laufende Serie | Serienseite | «Läuft noch, nächste Folge am …» | Test |
| AK-82 | Staffeln 3 und 4 gewünscht, Staffel 3 kommt vollständig | Abgleich | Wunsch «Teilweise verfügbar», Meldung `wish_season_available` für Staffel 3; bei einer laufenden Staffel «Die bisher erschienenen Folgen sind da» | Test |
| AK-83 | Kanal `wish_status` | freigegeben, abgelehnt, fehlgeschlagen | Eingang je Art; Kanal ab Werk aus (F-14) | Test |
| AK-133 | Noahs Wunsch zu einem fehlenden Titel wird abgelehnt | Eingang und Glocke | Eingang zeigt «Abgelehnt: …», die Glocke zählt ihn; Mia sieht ihn nicht | Test |
| AK-134 | bestehende Administratorkonten | Migration von Stufe 4 | `wish_review` in ihren Ereignislisten; gewöhnliche Konten unverändert | Test (eingefrorener Stand) |
| AK-84 | Administrator, Wunsch PENDING | «Freigeben» in Slouchcat | Request in Jellyseerr APPROVED; Zustand «In Beschaffung» | Test; Hand |
| AK-85 | Wunsch schon entschieden (3.x: 409) | «Ablehnen» | Meldung «Schon entschieden», Zustand neu gelesen | Test |
| AK-86 | Jellyseerr-Webhook eingerichtet | Freigabe in Jellyseerr | Abgleich innert 30 s statt 5 min; Inhalt der Meldung nicht ausgewertet | Hand; Test |
| AK-87 | ungültiges Token | Webhook | 401 ohne Protokolleintrag | Test |
| AK-88 | Zuordnung grosseltern übernommen, 3 offene Requests in Jellyseerr, alle älter als der Zeiger des Abgleichs | Zuordnung setzen | Meldung nennt «3 Anfragen»; `wishes.import` legt 3 Wünsche bei «Oma & Opa» an, «In Jellyseerr angelegt», keine Meldung für schon verfügbare | Test |
| AK-89 | Requests von wohnzimmer-tv | Verwaltung | Filter «Nicht zuordenbar» zeigt sie; Zuschauer sehen sie nie | Test |

## 6. Sicherheit (alle Stufen)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-90 | Mutation ohne `X-Requested-With` | `createWish` | 403 `csrf_header_missing` | Test |
| AK-91 | ohne Profil | `listWishes` | 403 `profile_required` | Test |
| AK-92 | Antworten von Jellyseerr enthalten Adressen | Abgleich | Slouchcat ruft keine Adresse aus einer Antwort auf | Test (Mock mit Fallen-URL) |
| AK-93 | Bild eines fehlenden Titels | Browser | Bild kommt von Slouchcat, nie von `image.tmdb.org` | Test; Hand (Netzwerk-Ansicht) |
| AK-94 | Administratorseite | Antworten durchsuchen | Schlüssel nie in einer Antwort | Test |
| AK-135 | Verwaltungsantworten der Wünsche (`listAdminWishes`, `getSpotlightEntries`) | Antwort prüfen | keine `ItemRef`, keine scoped Adresse; Poster über `getAdminExternalTitleImage` bzw. `getAdminItemImage` | Test (Vertrag) |
| AK-136 | weiteres Profil (Noah) | `listAccountWishes` | 403 `primary_profile_required`; die Operation ist `scoped` | Test (Vertrag) |

## 7. Performance

| Nr. | Messgrösse | Bedingung | Zielwert | Prüfung |
|---|---|---|---|---|
| AK-95 | Aufrufe an Jellyseerr beim Seitenaufbau | Startseite, Vorschauseite, «Meine Wünsche», Serienseite, Suche | 0 | Test (Mock zählt) |
| AK-96 | Serverzeit `getHome` | Spotlight mit 5 fehlenden, 20 Titel | p95 höchstens 20 ms über dem Wert ohne fehlende Titel, 50 Aufrufe (**Vorschlag**) | Messung `docs/perf/` |
| AK-97 | Bühnenbild der ersten Folie | Prepaint | unverändert gegenüber 1.40 (Messweg P-10) | Messung |
| AK-98 | `createWish` | Jellyseerr erreichbar (Mock) | p95 ≤ 500 ms (**Vorschlag**) | Test |
| AK-99 | `createWish` | Jellyseerr nicht erreichbar | ≤ 3,5 s (Zeitlimit 3 s) | Test |
| AK-100 | `listWishes` | 500 Wünsche im Profil | p95 ≤ 100 ms (**Vorschlag**) | Messung |
| AK-101 | Vorschauseite | Titel im Cache | Serverzeit p95 ≤ 100 ms (**Vorschlag**) | Messung |
| AK-102 | Aufrufe an Jellyseerr | Ruhe, keine offenen Wünsche | höchstens 1 `GET /request` je 5 min | Test |
| AK-103 | Rate-Limit | Burst von 50 Aufrufen | höchstens 5 je Sekunde an Jellyseerr | Test |
| AK-104 | Zeit bis «Verfügbar» | Radarr-Webhook bis Meldung | ≤ Scan + Match + 1 min (Haken), sonst ≤ 1 h (Nachlauf) | Hand |
| AK-137 | `searchExternalTitles` | Profil 12, 20 Treffer ohne Eintrag in `external_titles` | Serverzeit p95 ≤ 3,5 s (Zeitlimit 3 s für die Detailabrufe, **Vorschlag**); zweite gleiche Suche p95 ≤ 100 ms | Messung |

## 8. Abnahme der Mockups

Marco prüft das Mockup (`docs/planning/mockups/25-medienwuensche/index.html`) vor jedem Bau. Bilder liegen nur
lokal in `img/`; ohne sie zeigt das Mockup Platzhalter mit Titel.

| Kennung | Was Marco prüft |
|---|---|
| M-00 | Leiste «Mockup»: Profil wechseln (Marco, Sandra, Noah, Mia, Oma & Opa, Gast), Jellyseerr-Zustand (verbunden, nicht erreichbar), Abläufe vorspulen (freigeben, verfügbar), Handy/Desktop, Zurücksetzen |
| M-01 | Bühne mit fehlendem Titel: Chip, «Film anfordern» mit Plus, «Details», «Trailer»; nach dem Anfordern Zustandsknopf; erste Folie vorhanden; Posterreihe mit Plus-Knopf; Chip «Dein Wunsch ist da»; Reihe «Deine Wünsche» nach «Weiterschauen» |
| M-02 | Spotlight-Liste mit Filter «Alle / In der Bibliothek / Noch nicht da» |
| M-03, M-04 | Vorschauseite Film und Serie wie eine Detailseite, Hauptknopf oder Zustand; letzter geschlossener Wunsch mit Datum und Grund; ohne Recht nicht erreichbar (ausser mit eigenem Wunsch); Serie ohne TVDB-ID |
| M-05 | Detailseite Film (vorhanden): unverändert bis auf den Chip «Dein Wunsch ist da» (14 Tage nach der Erfüllung) |
| M-06 | Serienseite: gedämpfte Karten, «Staffeln anfordern», «2 von 4 Staffeln vorhanden», nächste Folge; Profil ohne Recht ohne diese Teile |
| M-07 | Suche: Bibliothek oben, darunter «Noch nicht in der Bibliothek»; Profil Mia ohne Abschnitt |
| M-08 | «Meine Wünsche»: Filter, Zustände aller acht Schlüssel, Verlauf, Zurückziehen, erneut anfordern, Kennzeichen «In Jellyseerr angelegt» |
| M-09, M-10 | neue Eingangsarten mit Link; neue Ereignisse je Kanal, «Freigabe nötig» nur bei Administratoren |
| M-11 | Profildialog mit «Darf Titel anfordern», gesperrt, wenn das Konto nicht darf; Chip auf der Karte |
| M-12 | Profilwahl unverändert |
| M-13 | Dialog: «Wird als … übermittelt», «Wird sofort beschafft» / «Braucht eine Freigabe», Staffelwahl mit «Bereits angefordert», Mitwunsch «Benachrichtige mich», nicht erreichbar, Kontingent erschöpft |
| M-20 | Übersicht: Kennzahl und die neuen Hinweise (Freigabe, nicht erreichbar, Konten ohne Zuordnung) |
| M-21, M-22 | siebte Kachel «Jellyseerr» nach «Webhooks von Radarr und Sonarr», Zeile oben mit neun Statusschlüsseln; Bänder «Verbindung» (Prüfliste), «Abgleich», «Sofortmeldung», «Protokoll», Verweis «Zuordnung: n von m Konten» (Route `#/admin/integrations/jellyseerr`) |
| M-23 | «Anfragen»: Kennzahlen, Filter, Tabelle, Aufklappzeile, Aktionen |
| M-23 | Route `#/admin/wishes`, Reiter «Wünsche» im Bereich Medien direkt nach «Pflege» |
| M-24 | «Zuordnung» (`#/admin/wishes/mapping`): Auswahlfeld je Konto und je Profil direkt in der Liste, sofort wirksam mit Meldung «Zuordnung geändert»; belegte gesperrt, Vorschlag mit «Übernehmen», Warnungen, «Alle Vorschläge übernehmen» mit Bestätigung |
| M-25 | «Einstellungen» (`#/admin/wishes/settings`): alle Schalter aus Anhang F 4, sofort wirksam ohne «Speichern»; «Wünsche aktiv» steht im Mockup zur Vorführung an, ab Werk ist es aus; Verweis auf Spotlight |
| M-26 | Konten: Spalte «Wünsche», Formular «Zugriff» mit Schalter und Zeile «Jellyseerr-Benutzer»; alle heutigen Felder vorhanden |
| M-27 | Spotlight: Band «Fehlende Titel» (gespeichert mit «Speichern» des Bands «Quelle»), Einträge mit Poster, Zustand und «Anfordern»; alle elf Felder des Bands «Quelle» |
| M-28 | Startseite: Reihe «Deine Wünsche» in der Tabelle |
| M-29 | Benachrichtigungen der Verwaltung: neue Ereignisse |
| M-30 | Hintergrundjobs: acht neue Jobarten mit Takt; Erstabgleich mit Fortschritt |

Für alle Seiten gilt: Gestaltung nach Marcos Regeln vom 16. November 2026 (Klappbares zugeklappt, genug
Kontrast, ein Formular), Texte in Schweizer Schreibweise, keine Funktion der heutigen Seiten fehlt (ADR 0035:21-22),
Handy- und Desktopansicht bedienbar.
