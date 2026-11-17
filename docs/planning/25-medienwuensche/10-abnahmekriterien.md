# Konzept 25 · Abnahmekriterien

> **Kurz gesagt.** Jedes Kriterium nennt Voraussetzung, Handlung, erwartetes Ergebnis und wie es geprüft wird:
> mit einem automatischen Test («Test») oder von Hand auf CT 190 mit echtem Seerr («Hand»). Die
> Kriterien sind nach Stufe und Bereich gruppiert und durchnummeriert (AK-1 …). Weil alle Stufen als ein Release
> erscheinen (Marco, F-22 und B-1), wird einmal abgenommen: wenn alle Kriterien erfüllt sind und `task ci` auf dem
> fertigen Zweig grün ist. Am Schluss steht, was Marco am Mockup geprüft hat.

Grundlage: [Anhang F](anhang/F-entscheid-blatt.md), `03-architektur.md`, `08-datenmodell-api.md`, `09-umsetzungsplan.md`. Beispieldaten
nach Anhang F Abschnitt 11 (Konten marco, grosseltern, gast; Profile Marco, Sandra mit PIN, Noah 12, Mia 6;
Seerr-Benutzer 1 marco bis 5 wohnzimmer-tv). «Mock» heisst Mock-Seerr `seerrtest` über `httptest`.
Messgrössen mit **Vorschlag** sind Zielwerte dieses Papiers, nicht von Anhang F. Kriterien, die nach der ersten
Kritik dazukamen, tragen die Nummern ab AK-105 und stehen bei ihrer Stufe. Kriterien aus Fassung 3 (Marcos
Rückmeldung vom 17. November 2026, [Anhang H](anhang/H-nachtrag-17-november.md)) tragen die Nummern AK-138 bis
AK-190 und stehen ebenfalls bei ihrer Stufe; Stufe 5 ist neu. Kriterien aus Marcos zweiter Rückmeldung vom
selben Tag (E-5 bis E-7, Anhang H Abschnitt 8) tragen die Nummern AK-191 bis AK-212. Kriterien aus der Kritik an
Fassung 3 (Anhang H Abschnitt 9, Redaktionsentscheide) tragen die Nummern AK-213 bis AK-219. Kriterien aus Marcos
dritter Rückmeldung (E-8 bis E-11, Anhang H Abschnitt 10) tragen die Nummern AK-220 bis AK-227. Kriterien, die fehlende Titel
auf der Startseite oder in der Spotlight-Liste prüften, sind nicht umnummeriert, sondern als «entfällt (E-5)»
markiert; ebenso die Kriterien zum zurückgezogenen Vorschlag einer Verknüpfung (N-2a) als «entfällt (E-8)».
Kriterien, die unter derselben Nummer auf die neue Regel umgeschrieben sind, tragen den Zusatz «geändert (E-8)».
Kriterien aus Marcos Antworten auf die offenen Fragen (Anhang H Abschnitt 11, N-100 bis N-104) tragen die Nummern
AK-228 bis AK-232; ältere Kriterien, die nur für die Ausnahme je Profil galten, sind als «entfällt (F-27)»
markiert, umgeschriebene tragen «geändert (F-4)», «geändert (F-24)» oder «geändert (F-27)». «Mock-Arr» heisst Mock-Radarr/Sonarr `arrtest`.

## 1. Stufe 0: Papiere

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-1 | Marco hat die 29 offenen Fragen (F-1 bis F-34 ohne F-8, F-20, F-32, F-33, F-34) und T-1, T-2 beantwortet (17. November 2026, Anhang H 11); E-1 bis E-11 stehen schon fest | Frage 33 mit Nachtrag, ADR 0037, ADR 0038 lesen | Frage 33 nennt E-1 bis E-11 mit Datum, der Nachtrag die Antworten auf alle Fragen und B-1 bis B-4 mit den vier Abweichungen F-4, F-22, F-24, F-27, ändert Spec `:35` und `:46`, hebt `:66` in Teilen und `:67` nur beim Wunsch auf, bestätigt `:65` und `:68`, fasst `:69` mit der Ausnahme Profil-Tag; S-4 bleibt ohne Ausnahme; beide ADR verweisen darauf | Hand (Marco) |
| AK-2 | AK-1 | CLAUDE.md Regel 11 lesen | Wortlaut erlaubt Wünsche über Seerr nach ADR 0037 und genau zwei Ausnahmen von «Steuerung von Arr/Seerr»: Benutzer in Seerr anlegen und pflegen (E-Mail, Passwort, Rechte, Limit) und Tags in Radarr und Sonarr setzen; Qualitätsprofile, Warteschlangen, Suchen und Löschen bleiben verboten | Hand |
| AK-138 | AK-1 | ADR 0010 und Regel 10 lesen | ADR 0010 hat den Nachtrag mit zwei eng gefassten Ausnahmen «Slouchcat legt Tags an und hängt sie an Titel» und «Slouchcat liest die Warteschlange für den Download-Fortschritt»; der Webhook-Empfang bleibt unverändert; Regel 10 bzw. die Redaction-Liste nennt `password`, `newPassword` und die Schlüssel von Radarr und Sonarr | Hand |
| AK-211 | AK-1 | ADR 0037, ADR 0038, Frage 33 lesen | ADR 0037 nennt «API und Webhook» und «Nur API» unter «Verworfen» mit dem Grund, dass Marco «Webhooks wie heute» gewählt hat; ADR 0038 löst S-4 nicht ab und schliesst fehlende Titel auf der Startseite aus; F-33 und F-34 sind beantwortet, F-8 steht als «entschieden durch E-5» | Hand (Marco) |
| AK-232 | Zweig mit allen Stufen fertig | Pull Request, Versionen und `testdata/db/` lesen | ein Zweig, ein Pull Request, eine Version für alle Stufen; genau ein neuer eingefrorener Datenbankstand am Schluss; Merge und Tag erst auf Marcos Wort (F-22, B-1, B-2; N-104) | Hand |

## 2. Stufe 1: Fundament

### 2.1 Verbindung

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-3 | Seerr unter `http://192.168.x.y:5055` | Adresse und Schlüssel eintragen, «Verbindung testen» | Prüfliste: erreichbar, Schlüssel gültig, Version, «Handeln im Namen von Benutzern», CSRF-Schutz aus, Radarr und Sonarr; alle grün | Hand; Test mit Mock |
| AK-4 | AK-3 | Schlüssel absichtlich falsch | Zustand «Schlüssel ungültig»; jeder Lauf von `wishes.sync` ruft nur `GET /auth/me`, sonst kein Aufruf; fehlende Titel bleiben für Zuschauer sichtbar; kein Schlüssel im Protokoll | Test (Mock 403) |
| AK-5 | Seerr gestoppt | «Verbindung testen» | Zustand «Nicht erreichbar» innert 10 s; Fehlertext nennt nur Schema und Host | Test (Zeitlimit) |
| AK-6 | Adresse antwortet mit 302 | Test | gilt als Fehler, keine Weiterleitung gefolgt, Schlüssel nicht an das Ziel gesendet | Test |
| AK-7 | Mock ohne Wirkung von `X-API-User` | Test, dann «Wünsche aktiv» einschalten | Prüfpunkt «Handeln im Namen von Benutzern» rot mit Hinweis; «Wünsche aktiv» lässt sich nicht einschalten, API weist ab (**Vorschlag** T-1) | Test |
| AK-8 | CSRF-Schutz in Seerr an (Mock: 403 auf POST) | Test | Prüfpunkt «CSRF-Schutz aus» rot mit Abhilfe | Test; Hand einmal gegen echtes Seerr (Prüfweg ist abgeleitet) |
| AK-9 | Schlüssel gespeichert | Seite neu laden | Feld leer mit «…a1b2»; die API liefert den Schlüssel nie (`writeOnly`) | Test (Vertrag) |
| AK-10 | Schlüssel in `deploy/.env` | Ansicht öffnen | Feld gesperrt, Hinweis «In deploy/.env gesetzt» (**Vorschlag** Umgebungsvariable) | Test |
| AK-105 | Schlüssel gespeichert für `192.168.1.50` | andere Adresse eintragen, Schlüsselfeld leer, «Verbindung testen» | 422 `seerr_key_required`; der gespeicherte Schlüssel geht nicht an die neue Adresse; bei `http` ins Internet Warnung | Test (Mock zählt) |
| AK-106 | Jellyseerr 2.6 im Mock | Test | Prüfpunkt Version als Warnung «nicht geprüft», kein Fehler | Test |
| AK-107 | Verbindung eingerichtet, Fingerabdruck gespeichert | Adresse zeigt auf eine andere Instanz (Mock 2) | «Andere Seerr-Instanz erkannt»; kein `POST`, kein `DELETE`, kein Abgleich bis «Zuordnung bestätigen» oder «Zuordnung löschen»; neue Wünsche bleiben `queued`; nach «Bestätigen» läuft ein Erstabgleich | Test |

### 2.2 Seerr-Benutzer der Konten und Sammelbenutzer (Ausnahme je Profil entfällt, F-27)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-11 | Verbindung steht | «Benutzerliste neu laden» | fünf Benutzer mit Rechten, Auto-Freigabe, Limit | Hand; Test |
| AK-12 | Konto grosseltern, Benutzer 4 mit gleicher E-Mail (andere Gross-/Kleinschreibung), Zustand «Nicht eröffnet» | Reiter «Seerr-Benutzer», «In Seerr eröffnen» | Konto ist mit Benutzer 4 **verknüpft**, Zustand «Verknüpft», kein `POST /user`; Meldung «grosseltern in Seerr verknüpft.» (E-4, Fassung 2 sah nur einen Vorschlag vor). Geändert (F-24): kein `…/settings/password`, Passwort «Wird nicht abgeglichen» (N-103) | Test |
| AK-13 | Benutzer 1 gehört Konto marco | Benutzer 1 einem anderen Konto zuordnen | Auswahl gesperrt; API 409 `seerr_user_taken` | Test |
| AK-14 | – | – | entfällt (F-27): prüfte die Ausnahme je Profil; dass jedes Profil als Benutzer des Kontos sendet, prüft AK-228 | – |
| AK-15 | Benutzer in Seerr gelöscht | `seerr.users` laufen lassen | Warnung «Benutzer in Seerr gelöscht»; betroffene Profile `canRequest = false`, Grund `seerr_user_gone` | Test |
| AK-16 | drei Konten «Nicht eröffnet», eines mit gleicher E-Mail in Seerr | «Alle eröffnen», bestätigen | Rückfrage nennt «1 verknüpfen, 2 neu anlegen»; danach alle drei mit Seerr-Benutzer; ersetzt «Alle Vorschläge übernehmen» aus Fassung 2 | Test |
| AK-108 | – | – | entfällt (F-27): prüfte das Umstellen der Ausnahme; das Einfrieren des Benutzers je Wunsch prüft AK-231 | – |
| AK-109 | geändert (F-4): «Konten ohne Seerr-Benutzer» ab Werk = «Sammelbenutzer», Sammelbenutzer «slouchcat-gaeste» angelegt (AK-229) | Gast (Konto mit Recht, ohne Seerr-Benutzer) fordert an | gesendet als slouchcat-gaeste (`X-API-User` dessen ID); das Konto sieht fehlende Titel; mit «Nicht erlauben» sieht es keine (Grund `no_mapping`) | Test |
| AK-228 | Konto marco mit Benutzer 1; Profile Marco, Sandra, Noah mit Wunschrecht | jedes Profil fordert an | alle Wünsche gehen mit `X-API-User: 1`; es gibt keine Tabelle `wish_profile_map`, keine Operation `setProfileWishMapping` und kein Auswahlfeld «Sendet als» (F-27, N-100) | Test (Vertrag, Migration) |
| AK-229 | Seerr eingerichtet, kein Benutzer `slouchcat-gaeste@slouchcat.invalid` in Seerr | «Wünsche aktiv» einschalten; danach ein zweites Mal ein- und ausschalten | genau ein `POST /user` mit dieser E-Mail und Zufallspasswort, Rechte aus `defaultPermissions` der Instanz, kein Aufruf `…/settings/permissions` (geändert I-F2, N-111), Standard-Limit über `…/settings/main` (10/7, 5/7); Seerr-ID in `wishes.shared_seerr_user`; beim zweiten Mal kein weiterer Benutzer; die Seite «Einstellungen» zeigt «Sammelbenutzer» und «Nicht erlauben», keine Auswahl eines Benutzers (F-4, N-101) | Test (Mock); Hand auf CT 190 (B-3: Testbenutzer danach löschen) |

### 2.3 Wunschrecht und Einstellungen

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-17 | frische Migration auf bestehender DB | Konten und Profile ansehen | Besitzerkonto an, andere Konten aus; Hauptprofile an, weitere aus | Test (Migration, eingefrorener Stand) |
| AK-18 | Konto gast «Darf Titel anfordern» aus | Hauptprofil Gast setzt im Profildialog an | Schalter gesperrt mit «Das Konto darf keine Titel anfordern.»; API weist ab | Test |
| AK-19 | Weiteres Profil (Mia) aktiv | Profildialog | kein Zugriff (nur Hauptprofil verwaltet) | Test |
| AK-20 | jede Einstellung aus Anhang F 4 | Seite «Einstellungen» und andere Seiten durchgehen | jede Einstellung hat genau einen Ort, andere Seiten verlinken | Hand |
| AK-21 | «Wünsche aktiv» aus (ab Werk) | Sitzung lesen | `wishes.enabled = false`; Zuschauer sehen nichts Neues | Test |
| AK-110 | Seite «Einstellungen» | einen Schalter umlegen | wirkt sofort ohne «Speichern», Meldung bestätigt; Neuladen zeigt den neuen Wert | e2e |
| AK-111 | «Seerr verwenden» aus, ein Wunsch `queued` seit 23 h | 2 h warten (Uhr-Mock) | kein Aufruf an Seerr; Zuschauer sehen keine fehlenden Titel; der Wunsch bleibt `queued` (Frist ruht, **Vorschlag** T-2) | Test |
| AK-112 | Administratorkonto ohne Formular «Zugriff» | Aufklappfeld auf «Konten» | Block «Wünsche» mit «Darf Titel anfordern» und «Seerr-Benutzer» | e2e |

### 2.4 Sicherheit Stufe 1

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-22 | Debug-Log an | Test, Speichern, Benutzerliste | Log enthält weder `X-Api-Key` noch `seerr.api_key` im Klartext | Test (Redaction) |
| AK-23 | Konto ohne Administratorrolle | `/admin/seerr…`, `/admin/wishes…` aufrufen | 403 `forbidden`; jede Operation steht im Vertragstest | Test |

### 2.5 Seerr-Benutzer und Passwort (Fassung 3)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-139 | «Neue Konten in Seerr eröffnen» an, Einladung mit «Darf Titel anfordern» an, keine E-Mail `gast@…` in Seerr | Gast nimmt die Einladung an | in Seerr ein lokaler Benutzer mit derselben E-Mail, `username` = Teil vor dem @, Rechte wie `defaultPermissions` der Instanz, kein `…/settings/permissions` (I-F1, N-110); Zustand «Eröffnet», Passwort «Abgeglichen»; der Gast meldet sich in Seerr mit E-Mail und Passwort aus Slouchcat an (N-1) | Test (Mock); Hand auf CT 190 |
| AK-140 | wie AK-139, Einladung ohne Wunschrecht | annehmen | Benutzer eröffnet mit `permissions = 0` (N-9, F-23) | Test |
| AK-141 | in Seerr gibt es `Gast@…` schon (lokaler Benutzer ohne Administratorrechte) | Gast nimmt die Einladung an | geändert (E-8): **sofort verknüpft**, Zustand «Verknüpft», kein zweiter Benutzer, Passwort gesetzt, Rechte unverändert; das Konto kann anfordern (N-90, N-91; Einzelheiten AK-220 bis AK-224) | Test |
| AK-142 | `GET /user?q=` findet nichts, `POST /user` antwortet 409 | Gast nimmt die Einladung an; danach dasselbe über «In Seerr eröffnen» | geändert (E-8): in beiden Fällen neu gelesen und verknüpft; beim Annehmen `linked_by = invite`, auf Knopfdruck `linked_by = admin` (N-2, N-90) | Test (Mock) |
| AK-143 | Seerr gestoppt | Gast nimmt die Einladung an | Konto in Slouchcat angelegt, Anmeldung wie heute; Zustand «Wartet»; nach dem Start von Seerr eröffnet der nächste Abgleich den Benutzer mit Zufallspasswort, Stand «Folgt bei der nächsten Anmeldung»; bei der nächsten Anmeldung in Slouchcat setzt Slouchcat das Passwort, Stand «Abgeglichen» (N-4) | Test (Uhr-Mock) |
| AK-144 | Konto mit eröffnetem Benutzer | eigenes Passwort in «Konto ändern» ändern; E-Mail ändern | sofort `POST /user/{id}/settings/password` mit dem neuen Passwort; neue E-Mail über `settings/main`, alle übrigen Felder unverändert zurückgeschickt; Meldung «Das neue Passwort gilt auch in Seerr.» (N-5) | Test |
| AK-145 | AK-139, AK-143 und AK-144 durchlaufen | Volltextsuche nach dem Testpasswort über alle Tabellen (auch `jobs`), alle Protokolle und alle API-Antworten | kein Treffer; in Slouchcat nur der Argon2id-Hash wie heute (N-3, N-12) | Test |
| AK-146 | Konto marco ist mit Benutzer 1 verknüpft; Konto grosseltern mit einem Administrator von Seerr; Konto gast mit einem Jellyfin-Benutzer | Passwort ändern, anmelden | kein Aufruf der Passwort-Route; Stand «Bleibt in Seerr (Administrator)» bzw. «Anmeldung über Jellyfin» (N-6) | Test |
| AK-147 | eröffneter Benutzer, Konto mit Wunschrecht | Administrator nimmt das Wunschrecht weg, dann gibt er es zurück | Rechte in Seerr `0`, dann der aktuelle Wert von `defaultPermissions` (geändert I-F1, N-110); bei einem verknüpften Benutzer kein Aufruf und der Hinweis «… behält in Seerr seine Rechte» (N-9) | Test |
| AK-148 | Konto mit eröffnetem Benutzer (`origin = created`) und zwei Requests in Seerr | «Konto entfernen» | Benutzer bleibt in Seerr samt Requests, Rechte `0`, neues Zufallspasswort; kein `DELETE /user`; Rückfrage nennt es (N-10, F-31). Den Fall eines verknüpften Benutzers prüft AK-219 | Test |
| AK-149 | Stand «Abgeglichen», Passwort in Seerr direkt geändert | «Passwort bei der nächsten Anmeldung neu übernehmen», dann in Slouchcat anmelden | bei der Anmeldung setzt Slouchcat das Slouchcat-Passwort in Seerr; «Passwort nicht abgleichen» → kein Aufruf mehr (N-7) | Test |
| AK-150 | Seerr antwortet 10 s nicht | `acceptInvitation`, `login`, `updateAccount` | Antwortzeit und Antwortinhalt wie ohne Seerr (Abweichung p95 ≤ 50 ms, **Vorschlag**); die Antworten verraten nicht, ob die E-Mail in Seerr existiert (mit gleicher Adresse in Seerr: AK-216) | Test |
| AK-213 | – | – | entfällt (E-8): prüfte den Zustand «Verknüpfung vorgeschlagen» nach einer Einladung; ersetzt durch AK-220, AK-221, AK-224 | – |
| AK-214 | – | – | entfällt (E-8): prüfte den Vorschlag nach einer E-Mail-Änderung; ersetzt durch AK-225 | – |
| AK-215 | – | – | entfällt (E-8): prüfte, dass ein vorgeschlagener Benutzer kein Passwort bekommt; den Zustand gibt es nicht mehr. Das Passwort prüfen AK-221 und AK-222 | – |
| AK-216 | zwei Durchläufe: in Seerr gibt es die Adresse, und es gibt sie nicht | `acceptInvitation` und `updateAccount` mit dieser Adresse; danach die Seiten des Kontos in der Zuschauer-App | geändert (E-8): Antworten byte-gleich, Antwortzeit gleich (Abweichung p95 ≤ 50 ms, **Vorschlag**); keine Meldung, kein Feld und kein Text verrät, ob Seerr die Adresse kannte (N-94; Meldung nach dem Annehmen: AK-223) | Test (Vertrag); e2e |
| AK-217 | ein Konto «Nicht eröffnet» mit gleicher Adresse in Seerr, ein Konto «Nicht eröffnet» ohne | «Alle eröffnen» | geändert (E-8): Rückfrage nennt «1 verknüpfen, 1 neu anlegen»; danach das erste Konto verknüpft (`linked_by = admin`, Passwort «Wird nicht abgeglichen», geändert F-24), das zweite eröffnet (Passwort «Folgt bei der nächsten Anmeldung»); kein Hinweis «Neu verknüpft» in der Übersicht (N-2, N-93); Antwort `{link: 1, create: 1}` | Test; e2e |
| AK-218 | – | – | entfällt (E-8): prüfte «Verknüpfung bestätigen» (`confirmSeerrAccountLink`); die Aktion und die Operation gibt es nicht mehr (N-90) | – |
| AK-219 | Konto mit verknüpftem Benutzer (`origin = linked`); zweites Konto mit von Hand gewähltem Benutzer (`manual`) | «Konto entfernen» bei beiden | kein Aufruf an `…/settings/permissions` und `…/settings/password`; Rechte und Passwort beider Benutzer unverändert; die Benutzer bleiben samt Requests; die Rückfrage sagt «Der Seerr-Benutzer bleibt unverändert.» (N-10, Redaktionsentscheid zu K-2) | Test |
| AK-220 | in Seerr gibt es einen lokalen Benutzer `nachbar@…` ohne Administratorrechte mit zwei Requests; «Neue Konten in Seerr eröffnen» an; Einladung mit Wunschrecht | ein Gast nimmt die Einladung mit der Adresse `Nachbar@…` an | sofort verknüpft: Zustand «Verknüpft», `origin = linked`, `linked_by = invite`, `linked_at` gesetzt; kein `POST /user`, kein `…/settings/permissions` (Rechte unverändert, N-9); Limit über `…/settings/main` geschrieben; `canRequest` wahr; ab Stufe 4 reiht `wishes.import` die zwei Requests ein (N-90) | Test (Mock); e2e |
| AK-221 | wie AK-220 | Gast nimmt die Einladung an; danach meldet er sich in Seerr mit E-Mail und Passwort aus Slouchcat an | ein `POST /user/{id}/settings/password` mit dem eingegebenen Passwort; Stand «Abgeglichen»; die Anmeldung in Seerr gelingt; Volltextsuche nach dem Testpasswort wie AK-145 ohne Treffer (N-91, N-3) | Test (Mock); Hand auf CT 190 |
| AK-222 | drei Durchläufe: die Adresse gehört in Seerr Benutzer 1, einem anderen Administrator, einem Benutzer mit Anmeldung über Jellyfin | Gast nimmt die Einladung mit dieser Adresse an; meldet sich später in Slouchcat an | verknüpft, aber **kein** Aufruf der Passwort-Route, weder beim Annehmen noch bei der Anmeldung; Stand «Bleibt in Seerr (Administrator)» bzw. «Anmeldung über Jellyfin»; Rechte unverändert; ein Administrator von Seerr bekommt so kein neues Passwort (N-91, N-6) | Test (Mock) |
| AK-223 | zwei Durchläufe: in Seerr gibt es die Adresse, und es gibt sie nicht | Gast nimmt die Einladung an | die Seite zeigt in beiden Fällen denselben Text «In Seerr hast du jetzt ein Konto mit derselben E-Mail-Adresse.»; kein anderer Text, kein Feld und keine Antwortzeit unterscheidet die Fälle (N-94, AK-216) | e2e; Test (Vertrag) |
| AK-224 | AK-220 durchlaufen; Uhr-Mock | Administrator öffnet die Übersicht am selben Tag, nach 6 Tagen und nach 8 Tagen; folgt dem Link | am selben Tag und nach 6 Tagen der Hinweis «Neu verknüpft: gast mit dem bestehenden Seerr-Benutzer nachbar», nach 8 Tagen nicht mehr; der Link öffnet Wünsche › Seerr-Benutzer mit dem Band dieses Kontos, das Auslöser und Datum nennt; «Verknüpfung lösen» geht dort; das Protokoll hat einen Eintrag mit Konto-ID, Seerr-ID und Auslöser, ohne E-Mail und Passwort. Nach einer Verknüpfung durch einen Administrator (AK-12, AK-217) erscheint kein Hinweis (N-93, Empfehlung der Redaktion) | Test (Uhr-Mock); e2e |
| AK-225 | Konto gast ohne Seerr-Benutzer («Wartet» bzw. «Nicht eröffnet»); zweites Konto oma mit eröffnetem Benutzer; in Seerr gibt es `nachbar@…` | gast ändert in «Konto ändern» seine E-Mail auf `nachbar@…`; oma ändert ihre E-Mail ebenfalls auf `nachbar@…`, Seerr lehnt ab | gast: sofort verknüpft, `linked_by = email_change`, Hinweis «Neu verknüpft» wie AK-224; das Passwort folgt bei der nächsten Anmeldung. oma: ihr Seerr-Benutzer bleibt, wie er ist, keine zweite Verknüpfung; die Verwaltung zeigt am Konto, dass die Adresse in Seerr vergeben ist. Beide Antworten an das Konto wie ohne Treffer (N-95, N-94) | Test (Mock) |
| AK-230 | Konto gast mit bestehendem Seerr-Benutzer, vom Administrator verknüpft («In Seerr eröffnen» mit gleicher E-Mail oder «Anderen Benutzer wählen»); Stand «Wird nicht abgeglichen» | gast ändert sein Passwort, meldet sich an; danach schaltet der Administrator «Passwort bei der nächsten Anmeldung neu übernehmen» ein, gast meldet sich wieder an | zuerst kein Aufruf der Passwort-Route; nach dem Einschalten (`setSeerrPasswordMode next_login`) bei der Anmeldung genau ein `…/settings/password`, Stand «Abgeglichen» (F-24, N-103). Beim Annehmen einer Einladung mit bestehender Adresse wird das Passwort dagegen gesetzt (AK-221) | Test (Mock) |
| AK-231 | Konto gast ohne Seerr-Benutzer hat über den Sammelbenutzer zwei offene Wünsche | «In Seerr eröffnen» für gast; danach fordert Profil Gast einen neuen Titel an; `wishes.import` läuft | die zwei Wünsche behalten `seerr_user_id` des Sammelbenutzers und bleiben in «Meine Wünsche» des Profils; der neue Wunsch geht als Benutzer von gast; Anfragen des Sammelbenutzers übernimmt der Import nicht (N-102) | Test |
| AK-226 | Bau von Schritt 6a | Reihenfolge der Commits lesen | der Commit 6a.1 mit der Messung in `docs/perf/` und der festgeschriebenen Schwelle für einen Gewinn liegt vor dem ersten Commit von 6a.2 (N-98) | Hand |
| AK-227 | Messung 6a.1 zeigt keinen Gewinn | Zweig nach Schritt 6a lesen | 6a.2 bis 6a.4 fehlen; `docs/perf/` hat eine Notiz mit dem Grund; `receiveWebhook`, `model.WebhookPayload` und der Vertrag sind unverändert; die Webhooks von Radarr und Sonarr verhalten sich wie heute (N-98, N-80) | Hand; `git diff` |

### 2.6 Limit einstellen (Fassung 3)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-151 | Vorgabe ab Werk, neues Konto eröffnet | `settings/main` im Mock ansehen | `movieQuotaLimit 10`, `movieQuotaDays 7`, `tvQuotaLimit 5`, `tvQuotaDays 7`; alle zuvor gelesenen Felder (Name, E-Mail, Sprache, Regionen) unverändert im Body (N-31, Anhang H 3.1) | Test |
| AK-152 | Konto grosseltern | Formular «Zugriff»: Limit «Eigenes» 3 Filme in 7 Tagen, 2 Staffeln in 14 Tagen; danach «Unbegrenzt»; danach «Standard» | Werte jeweils in Seerr geschrieben; «Standard» schreibt wieder die Vorgabe; die Vorgabe zu ändern ändert alle Konten mit «Standard» (N-31) | Test |
| AK-153 | geändert (I-F3): verknüpfter Benutzer hatte in Seerr 20 Filme in 30 Tagen, Konto ohne eigenes Limit; zweiter Fall: beide Limits 0; dritter Fall: alle Felder leer; vierter Fall: Konto hat schon ein eigenes Limit | verknüpfen | 1: «Eigenes» 20 Filme in 30 Tagen, Staffeln aus der Vorgabe; 2: «Unbegrenzt»; 3: «Standard», Vorgabe geschrieben; 4: das Limit aus Slouchcat gewinnt und wird geschrieben (N-112) | Test |
| AK-154 | Benutzer mit `MANAGE_USERS` (Benutzer 1 von Konto marco) | Limit setzen | keine Limit-Felder geschrieben; Konten und Seerr-Benutzer zeigen «Kein Limit (Administrator)»; Feld gesperrt (N-34) | Test; e2e |
| AK-155 | Anzahl 101 oder Tage 0 | speichern | 422 `validation_failed`, Feld markiert | Test |

### 2.7 Verbindungstest, neue Prüfpunkte (Fassung 3)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-156 | «Lokale Anmeldung» in Seerr aus; Staffelquelle `tmdb`; Adresse `http://` ausserhalb des eigenen Netzes | «Verbindung testen» | drei gelbe Prüfpunkte mit den Hinweisen aus `07-administration.md`, Abschnitt 4.1; der Test gilt als gelungen, «Wünsche aktiv» lässt sich einschalten; die Kachel zeigt den ersten Hinweis (N-11, N-12, N-64) | Test |
| AK-157 | Schlüssel eines Benutzers ohne `MANAGE_USERS` (Mock) | Test | «Benutzer anlegen und ändern» rot; Eröffnen und Limit nicht möglich | Test |

## 3. Stufe 2: Anfordern

### 3.1 Jugendschutz und Sichtbarkeit

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-24 | Film fehlt, TMDB-Freigabe 16 | Profil Noah (12) ruft Vorschauseite, Bild und `createWish` auf | 404 überall; Suche zeigt ihn nicht | Test |
| AK-25 | Film fehlt, keine Freigabe | Profil Noah | 404 (unbekannt = 18) | Test |
| AK-26 | Film steht in der Bibliothek, für Noah zu hoch | Noah sucht ihn | nicht als «Noch nicht in der Bibliothek»; Vorschauseite 404 | Test |
| AK-27 | Profil Mia ohne Recht | Suche, Vorschauseite per Link | kein Abschnitt «Noch nicht in der Bibliothek»; Vorschauseite 404 (F-9) | Test |
| AK-28 | Freigabeland CH ohne Wert, TMDB-Freigaben DE 12, US 16 | Prüfung | 12 (Länderreihenfolge nach ADR 0034, erstes Land mit bekanntem Wert, wie bei einem Bibliothekstitel) | Test |
| AK-113 | Konto grosseltern mit Recht, ohne Seerr-Benutzer (Zustand «Nicht eröffnet») | Suche, Vorschauseite | keine fehlenden Titel, Vorschauseite 404, kein Hinweis für den Zuschauer; die Übersicht der Verwaltung zeigt «1 Konto ohne Seerr-Benutzer» | Test |
| AK-114 | Noah hat einen abgelehnten Wunsch zu X; danach nimmt der Administrator Noah das Recht | Vorschauseite von X | Seite erreichbar, ohne Anforderungsknopf, mit dem letzten Wunsch; der Titel fehlt in der Suche | Test |
| AK-115 | Profil Noah (12), TMDB-Suche liefert 25 Treffer | Suche | Details nur für die ersten 20 Treffer geholt und in `external_titles` gespeichert; Treffer ohne bekannte Freigabe fehlen; Profil Marco (18) löst keinen Detailabruf aus | Test (Mock zählt) |

### 3.2 Anfordern

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-29 | Profil Marco, Seerr erreichbar | «Film anfordern», Dialog bestätigen | Dialog nennt «Wird als marco an Seerr übermittelt» und «Wird sofort beschafft»; Zustand «In Beschaffung»; in Seerr ein Request von marco | Hand; Test (Mock) |
| AK-30 | geändert (F-27): Profil Oma & Opa (Konto grosseltern, Benutzer ohne Auto-Freigabe) | anfordern | Hinweis «Braucht eine Freigabe»; Zustand «Freigabe ausstehend»; Request in Seerr gehört **grosseltern**, Status PENDING. Profil Noah im Konto marco: «Wird sofort beschafft», Request gehört marco (N-100) | Hand; Test |
| AK-31 | Mock | Anfrage mitschneiden | `POST /request` mit `X-API-User`, **ohne** `userId`; bei Serien `tvdbId` und Zahlenliste `seasons` | Test |
| AK-32 | Seerr gestoppt | anfordern | Antwort ≤ 3,5 s, Zustand «Wird übermittelt»; nach dem Start von Seerr wird der Wunsch ohne Zutun übermittelt | Test (Uhr-Mock); Hand |
| AK-33 | Seerr 24 h weg | Uhr vorstellen | `failed`, `unreachable_24h`; Ereignis `errors` an Administratoren | Test |
| AK-34 | erster POST kam an, Antwort ging verloren | Job erneut ausführen | kein zweiter Request in Seerr; Wunsch hängt am ersten | Test |
| AK-35 | geändert (F-27): Limit von grosseltern erreicht, Stand in Slouchcat noch alt (Mock 403) | Oma & Opa fordert an | «Fehlgeschlagen» mit «Limit erreicht: Seerr hat den Wunsch nicht angenommen.»; Stand danach neu gelesen | Test |
| AK-36 | Titel in Seerr gesperrt | anfordern | «Fehlgeschlagen», Grund Sperrliste; für 2.x und 3.x Text erkannt | Test |
| AK-37 | Titel in Seerr schon von wohnzimmer-tv angefordert (409) | Marco fordert an | Erfolg, Wunsch hängt am bestehenden Request | Test |
| AK-38 | Serie, alle Staffeln in Seerr schon angefragt (202) | anfordern | Erfolg, angehängt bzw. «wird eingelesen» | Test |
| AK-39 | Serie ohne TVDB-ID bei TMDB | Vorschauseite | kein Knopf, Hinweis; `createWish` 403 `wish_not_allowed`, Grund `tvdb_id_missing` | Test |
| AK-40 | Staffelwahl | Dialog | Staffel 0 und Staffeln ohne Folgen nicht wählbar; «Alle fehlenden Staffeln» vorgewählt | Test; Hand |
| AK-41 | Profil sendet 21 neue Wünsche in einer Stunde | 21. Wunsch | 429 `too_many_attempts` mit `Retry-After` (**Vorschlag** 20/h) | Test |

### 3.3 Mitwunsch, Zurückziehen, Idempotenz

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-42 | Marco hat Film X gewünscht | Sandra öffnet X | Hinweis «Schon angefordert», Knopf «Benachrichtige mich»; danach Mitwunsch, **kein** Aufruf an Seerr | Test (Mock zählt 0 Aufrufe) |
| AK-43 | AK-42 | Sandra sieht «Meine Wünsche» | Wunsch ohne Angabe, wer sonst gewünscht hat | Test |
| AK-44 | Marco fordert X zweimal an | zweiter Aufruf | 200 mit demselben Wunsch, kein zweiter Wunsch | Test |
| AK-45 | Datenbank | zweiten offenen Filmwunsch direkt einfügen | Teilindex verweigert | Test |
| AK-46 | Noahs Wunsch «Freigabe ausstehend» | «Zurückziehen» | sofort «Zurückgezogen»; danach löscht `wishes.withdraw` den Request in Seerr | Test; Hand |
| AK-116 | Sofortversuch läuft ins Zeitlimit, der Request kam in Seerr aber an | Profil zieht sofort zurück | danach kein Request dieses Benutzers in Seerr; keine Beschaffung | Test (Mock) |
| AK-117 | Request in Seerr schon freigegeben, Slouchcat zeigt noch «Freigabe ausstehend» | Zurückziehen | `DELETE` antwortet 401; Wunsch wieder «In Beschaffung», Hinweis «War schon freigegeben» | Test |
| AK-118 | Sandra ist Mitwünscherin bei Marcos Film | Sandra zieht zurück | nur Sandras Zeile fällt weg, kein Aufruf an Seerr; Marcos Wunsch bleibt | Test |
| AK-119 | Marco ist Besitzer, Sandra Mitwünscherin, Zustand «Freigabe ausstehend» | Marco zieht zurück | Sandra wird Besitzerin; der Request in Seerr bleibt | Test |
| AK-120 | «Bereits angeforderte Titel» = «Nur Zustand zeigen» | Sandra öffnet Marcos gewünschten Film | Zustand sichtbar, kein Knopf «Benachrichtige mich»; `createWish` 403 `wish_not_allowed` mit `status_only` | Test |
| AK-121 | Marco und Noah fordern denselben Film gleichzeitig an | zwei parallele `createWish` | ein Wunsch, ein Mitwunsch, ein `POST` an Seerr | Test |
| AK-122 | Serie: Staffel 3 hat in Seerr schon einen fremden Request | Noah fordert Staffeln 2 bis 4 an | Staffeln 2 und 4 tragen die neue Request-ID, Staffel 3 die fremde; Zustand je Staffel richtig | Test (Mock) |
| AK-47 | Wunsch «In Beschaffung» | Zurückziehen | Knopf fehlt; API 409 `wish_not_withdrawable` | Test |
| AK-48 | Wunsch eines anderen Profils | `withdrawWish` mit dessen ID | 404 | Test |

### 3.4 Abgleich

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-49 | Noahs Wunsch PENDING | in Seerr freigeben, ≤ 5 min warten | «In Beschaffung» | Hand; Test |
| AK-50 | in Seerr abgelehnt | Abgleich | «Abgelehnt», ohne Grund | Test |
| AK-51 | Request in Seerr gelöscht | Abgleich nach 24 h ohne Änderung | `failed`, `seerr_request_gone` | Test |
| AK-52 | Abgleich bricht nach Seite 2 ab | nächster Lauf | liest zuerst die neuen Änderungen, setzt dann an Seite 3 fort; keine doppelten Verlaufseinträge oder Meldungen | Test |
| AK-53 | 1000 Requests in Seerr, 3 geändert seit letztem Lauf | Abgleich | höchstens eine Seite (`take=50`) abgerufen | Test (Mock zählt) |
| AK-123 | 3000 Requests, noch nie abgeglichen | Verbindung speichern | Erstabgleich ohne Seitengrenze im Hintergrund mit Fortschritt unter Hintergrundjobs; danach ist der Zeiger gesetzt und der regelmässige Abgleich liest höchstens eine Seite | Test (Mock zählt) |
| AK-124 | Wunsch wartet 3 Tage auf Freigabe | Abgleich alle 5 min | `GET /request/{id}` für diesen Wunsch höchstens einmal je 24 h | Test (Mock zählt) |

### 3.5 Verfügbarkeit

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-54 | Film X gewünscht, Radarr importiert | Webhook, Scan, Match | Zustand «Verfügbar» erst nach dem Match; Link führt auf `/items/{id}` | Hand; Test |
| AK-55 | Seerr meldet AVAILABLE, Slouchcat hat den Titel nicht | Abgleich | Zustand «In Beschaffung · wird eingelesen», keine Meldung | Test |
| AK-56 | Film nach Scan ohne Freigabe, Noah wünschte ihn | Match setzt Freigabe 6 | Meldung an Noah nach dem Match bzw. spätestens nach dem stündlichen Nachlauf | Test |
| AK-57 | Serie TVDB-zuerst ohne `tmdb_id` in der Bibliothek | Wunsch mit TMDB-ID | Treffer über TVDB/IMDb, «Verfügbar» | Test |
| AK-58 | Wunsch `failed`, Titel kommt auf anderem Weg | Nachlauf | «Verfügbar» (**Vorschlag**) | Test |
| AK-59 | Seerr 48 h «verfügbar», kein Treffer | Verwaltung | Diagnose «In Seerr verfügbar, in Slouchcat nicht gefunden» (**Vorschlag**) | Test |
| AK-125 | Serie gewünscht, Staffel 1 mit 10 Folgen laut TMDB; erst 1 Folge eingelesen | Match | Staffel bleibt «In Beschaffung»; erst mit 10 abspielbaren Folgen verfügbar; Seerr-Zustand spielt keine Rolle | Test |

### 3.6 Benachrichtigungen (Stufe 2)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-60 | Wunsch wird verfügbar, Marco und Sandra (Mitwunsch) | Meldung | je ein Eingangseintrag; Kanalmeldungen des Kontos mit `[Marco]` bzw. `[Sandra]` | Test |
| AK-61 | AK-60, Sandra hat PIN | E-Mail lesen | «[Sandra] Ein Wunsch ist jetzt verfügbar.» ohne Titel (F-15) | Test; Hand |
| AK-62 | derselbe Übergang zweimal verarbeitet | Nachlauf erneut | keine zweite Meldung (`wish:<id>:available`) | Test |
| AK-63 | neues Konto und bestehende Konten nach der Migration von Stufe 2 | Kanäle ansehen | `wish_available` nach F-14 an, bei neuen und bei bestehenden Konten; `enabled` unverändert | Test (eingefrorener Stand) |
| AK-126 | Titel verfügbar gemeldet, Eingangseintrag gelesen und nach 90 Tagen weggeräumt; Titel gelöscht und neu eingelesen | Nachlauf | keine zweite Meldung (`notified_at`) | Test |
| AK-127 | Profil Noah mit offenem Wunsch wird gelöscht | Profil löschen | der Wunsch gehört dem Hauptprofil Marco, Verlauf «von gelöschtem Profil übernommen» | Test |
| AK-128 | Konto grosseltern mit offenem Wunsch, ohne Mitwünscher | Konto entfernen | Wunsch geschlossen (`failed`, `account_deleted`), sperrt den Titel nicht mehr; Request bleibt in Seerr; Rückfrage nennt «Wünsche bleiben in Seerr bestehen.» | Test |
| AK-64 | Einreichen eines Wunsches | – | keine Benachrichtigung, nur Rückmeldung am Bildschirm | Test |

### 3.7 Verwaltung Stufe 2

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-65 | Wünsche in allen Zuständen | Reiter «Anfragen» | Kennzahlen, Filter, Spalte «Gewünscht von Konto · Profil» mit Mitwünschern, Verlauf in der Aufklappzeile | Hand; e2e |
| AK-66 | Wunsch `failed` (Übermittlung) | «Erneut übermitteln» | Wunsch wieder `queued`, danach übermittelt | Test |
| AK-67 | offene Wünsche | Übersicht | Kennzahl «Offene Wünsche»; «Seerr nicht erreichbar» bei Ausfall | e2e |
| AK-68 | – | Seite «Hintergrundjobs» | Jobarten `wishes.submit`, `wishes.withdraw`, `wishes.sync`, `wishes.reconcile`, `seerr.users`, `seerr.provision`, `titles.refresh` mit Texten; «Jetzt ausführen» geht; Erstabgleich mit Fortschritt | e2e |

### 3.8 Limit beim Anfordern, Staffeln, Abgleich (Fassung 3)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-158 | Konto grosseltern, Limit 10 Filme in 7 Tagen; im Mock 9 Requests, davon 4 direkt in Seerr gestellt | Profil Oma & Opa öffnet den Dialog, fordert an, öffnet einen zweiten Dialog | Dialog zeigt «1 von 10 frei»; der erste Wunsch geht; danach «Limit erreicht» (E-3: Seerr zählt alles) | Test (Mock); Hand |
| AK-159 | Limit erreicht | zweiten Film anfordern | Knopf gesperrt mit «Der nächste Platz wird am {{Datum}} frei.»; der Titel bleibt sichtbar; API 409 `quota_exceeded` mit `nextFreeAt`; kein Wunsch, kein Aufruf an Seerr (N-36) | Test; e2e |
| AK-160 | Limit erreicht; Marco hat Film X gewünscht | Sandra (gleiches Konto) wünscht X mit | 200, Rolle `joined`; zählt nicht (N-33) | Test |
| AK-161 | Limit 5 Staffeln, 2 frei | Serie mit 3 gewählten Staffeln anfordern; eine davon ist schon von einem anderen Profil gewünscht | 2 Staffeln zählen (die Mitwunsch-Staffel nicht): Wunsch geht. Mit 3 neuen Staffeln: «Zu viele Staffeln: 2 frei», 409 (N-36) | Test |
| AK-162 | ein Wunsch wird in Seerr abgelehnt; ein anderer zurückgezogen | Stand neu lesen | beide zählen nicht mehr (abgelehnt: Seerr; zurückgezogen: Request gelöscht); ein fehlgeschlagener Request zählt weiter (N-33) | Test (Mock) |
| AK-163 | zwei Wünsche des Kontos `queued` (Seerr nicht erreichbar), Limit 10, Stand 8 | dritten anfordern | 409 `quota_exceeded`: wartende Wünsche zählen mit (N-35) | Test |
| AK-164 | Stand in Slouchcat sagt «frei», jemand hat inzwischen direkt in Seerr angefordert | anfordern | Mock 403 `Movie Quota exceeded.` → `failed`, `quota`; Stand danach neu gelesen | Test |
| AK-165 | Profil Marco, Benutzer 1 mit `ADMIN` | 15 Filme in einer Woche | keine Vorprüfung, kein 409; «Kein Limit (Administrator)» in «Meine Wünsche» (N-34) | Test |
| AK-166 | Serie, Seerr auf TVDB-Staffeln (Mock: 3 Staffeln, TMDB kennt 2) | Dialog öffnen | angeboten werden die 3 Staffeln aus Seerr; Seerr nicht erreichbar → die 2 aus TMDB mit Hinweis «Staffeln laut TMDB»; Öffnen ein zweites Mal in 1 h ohne Aufruf (N-63) | Test |
| AK-167 | Serie in der Bibliothek mit TVDB-ID 1 und einer TMDB-ID, die bei TMDB zu einer anderen TVDB-ID gehört | Abgleich eines Wunsches mit TVDB-ID 1 | Treffer über TVDB (TVDB zuerst, dann TMDB, dann IMDb); Filme weiter TMDB, dann IMDb (N-61) | Test |
| AK-168 | – | «Meine Wünsche» | Anzeige «Filme: 7 von 10 in 7 Tagen · Staffeln: 2 von 5»; ohne Limit «Ohne Limit»; kein Aufruf an Seerr beim Seitenaufbau | Test; e2e |

## 4. Stufe 3: Spotlight und Startseite

Marco: Im Spotlight nicht anzeigen, was fehlt; auf der Startseite nur vorhandene Filme und Serien; die Verwaltung
«ist gut so» (E-5, Anhang H 8.1). Die Kriterien zu fehlenden Titeln auf Bühne, Posterreihe und Spotlight-Liste
entfallen darum.

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-69 | Wünsche aktiv; Trakt-Liste mit 20 Einträgen, davon 3 nicht in der Bibliothek; Profil Marco mit Recht | Startseite | nur Titel der Bibliothek, Verhalten wie 1.40; kein Chip «Noch nicht in der Bibliothek», kein «Film anfordern» | Test |
| AK-70 | entfällt (E-5) | – | – | – |
| AK-71 | AK-69 | Bühne laden | Prepaint und Vorladen unverändert gegenüber 1.40 | Test |
| AK-72 | entfällt (E-5) | – | – | – |
| AK-73 | entfällt (E-5; abgedeckt durch AK-191) | – | – | – |
| AK-129 | entfällt (E-5; der Schalter «Anfordern aus dem Spotlight» entfällt, N-72) | – | – | – |
| AK-130 | entfällt (E-5) | – | – | – |
| AK-131 | Marcos Wunsch zu X wurde gestern erfüllt, X steht im Spotlight | Startseite und Detailseite von X, je Marco und Sandra | Bühne für beide wie 1.40: kein Wunsch-Chip, kein Staffelhinweis (N-70, Redaktionsentscheid zu K-3). Marco sieht den Chip «Dein Wunsch ist da» auf der Detailseite von X (bis 14 Tage nach der Erfüllung) und X in der Reihe «Erfüllte Wünsche»; Sandra ohne Wunsch sieht beides nicht | Test |
| AK-74 | entfällt (E-5) | – | – | – |
| AK-75 | Trakt-Eintrag einer TVDB-zuerst-Serie in der Bibliothek, ohne `tmdb_id` am Bibliothekstitel | Startseite | die Serie erscheint im Spotlight (heute fehlt sie, weil nur über TMDB verglichen wird); in der Verwaltung nicht unter «Fehlende Einträge» (N-73) | Test |
| AK-76 | entfällt (E-5) | – | – | – |
| AK-77 | entfällt (E-5; die Spotlight-Liste bleibt ohne Filter «Noch nicht da») | – | – | – |
| AK-78 | Reihe «Erfüllte Wünsche» | Startseite mit einem vor 3 Tagen erfüllten und einem offenen Wunsch | Reihe zeigt nur den erfüllten Titel, ab Werk direkt nach «Weiterschauen»; in der Verwaltung ausblendbar und verschiebbar (N-74, N-97; Marco, E-10) | e2e |
| AK-79 | Verwaltung Spotlight | Band «Fehlende Einträge» | ohne Schalter; Liste der fehlenden Einträge mit Poster, Zustand und «Anfordern»; alle elf Felder des Bands «Quelle» unverändert (N-71, N-72) | Hand; e2e |
| AK-169 | Trakt-Eintrag einer Serie nur mit TVDB-ID | Abruf | ID über TMDB «find» aufgelöst; gelingt es nicht, gilt der Eintrag nicht als fehlend, erscheint Zuschauern nicht und steht in der Verwaltung unter «nicht zuordenbar» (N-62) | Test |
| AK-170 | Verwaltung Spotlight | «Einträge der Liste», Band «Abgleich über TMDB, TVDB und IMDb» | je Eintrag die drei IDs; das Band nennt die Reihenfolge (Filme TMDB, IMDb; Serien TVDB, TMDB, IMDb) und listet nicht zuordenbare Einträge | e2e |
| AK-191 | wie AK-69; Profile Marco und Noah mit Recht, Mia ohne | Startseite (Bühne, Posterreihe), `/lists/spotlight`, `getHome`, `listHomeRow` | für alle drei Profile nur `ItemRef`, nie ein `ExternalTitleRef`; die 3 fehlenden Einträge erscheinen nirgends in der Zuschauer-App (N-70) | Test (Vertrag); e2e |
| AK-192 | wie AK-69, Bereich «Wünsche» an | «Entdecken» | keine Reihe mit den fehlenden Einträgen der Trakt-Liste (E-5 «Nein, nirgends») | e2e |
| AK-193 | wie AK-69 | Verwaltung › Spotlight | Band «Fehlende Einträge»: die 3 Einträge mit Poster über die Admin-Bildroute, Zustand «Noch nicht in der Bibliothek» und «Anfordern»; Antwort ohne `ItemRef` (N-71) | Test; e2e |
| AK-194 | AK-193, Wünsche aktiv | «Anfordern» an einem fehlenden Eintrag | Dialog wie in der App, als Hauptprofil des Administrators; danach Zustand «Angefordert · In Beschaffung»; die Startseite ändert sich nicht | Test; Hand |
| AK-195 | Wünsche von Noah: einer vor 3 Tagen erfüllt, einer vor 15 Tagen, einer offen | Startseite Noah | «Erfüllte Wünsche» zeigt nur den vor 3 Tagen erfüllten; ohne solchen Wunsch erscheint die Reihe nicht; der offene steht unter «Wünsche › Meine Wünsche», seine Zahl am Eintrag «Wünsche» der Seitenleiste (N-74, N-97; Marco, E-10) | Test; e2e |

## 5. Stufe 4: Serien und Komfort

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-80 | Tulsa King, 2 von 4 Staffeln da, Profil Marco | Serienseite | Zeile «2 von 4 Staffeln vorhanden», gedämpfte Karten für 3 und 4, «Staffeln anfordern»; Profil Mia ohne Recht sieht weder die Karten noch die Zeile | Hand; e2e |
| AK-132 | Staffel 3 hat Sandra schon angefordert | Marco öffnet den Dialog | Staffel 3 wählbar mit «Bereits angefordert»; Marco wird für Staffel 3 Mitwünscher, nur Staffel 4 geht an Seerr | Test; e2e |
| AK-81 | laufende Serie | Serienseite | «Läuft noch, nächste Folge am …» | Test |
| AK-82 | Staffeln 3 und 4 gewünscht, Staffel 3 kommt vollständig | Abgleich | Wunsch «Teilweise verfügbar», Meldung `wish_season_available` für Staffel 3; bei einer laufenden Staffel «Die bisher erschienenen Folgen sind da» | Test |
| AK-83 | Kanal `wish_status` | freigegeben, abgelehnt, fehlgeschlagen | Eingang je Art; Kanal ab Werk aus (F-14) | Test |
| AK-133 | Noahs Wunsch zu einem fehlenden Titel wird abgelehnt | Eingang und Glocke | Eingang zeigt «Abgelehnt: …», die Glocke zählt ihn; Mia sieht ihn nicht | Test |
| AK-134 | bestehende Administratorkonten | Migration von Stufe 4 | `wish_review` in ihren Ereignislisten; gewöhnliche Konten unverändert | Test (eingefrorener Stand) |
| AK-84 | Administrator, Wunsch PENDING | «Freigeben» in Slouchcat | Request in Seerr APPROVED; Zustand «In Beschaffung» | Test; Hand |
| AK-85 | Wunsch schon entschieden (3.x: 409) | «Ablehnen» | Meldung «Schon entschieden», Zustand neu gelesen | Test |
| AK-86 | Seerr-Webhook eingerichtet | Freigabe in Seerr | Abgleich innert 30 s statt 5 min; Inhalt der Meldung nicht ausgewertet | Hand; Test |
| AK-87 | ungültiges Token | Webhook | 401 ohne Protokolleintrag | Test |
| AK-88 | Konto grosseltern mit dem bestehenden Benutzer verknüpft, 3 offene Requests in Seerr, alle älter als der Zeiger des Abgleichs | verknüpfen | Meldung nennt «3 Anfragen»; `wishes.import` legt 3 Wünsche bei «Oma & Opa» an, «In Seerr angelegt», keine Meldung für schon verfügbare | Test |
| AK-89 | Requests von wohnzimmer-tv | Verwaltung | Filter «Nicht zuordenbar» zeigt sie; Zuschauer sehen sie nie | Test |

### Profil-Tag in Radarr und Sonarr (Stufe 4, Fassung 3)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-171 | Radarr in Seerr eingerichtet, Adresse noch nicht bestätigt | Integrationen › Seerr, Band «Radarr und Sonarr (Profil-Tags und Fortschritt)», «Testen» | Adresse aus Seerr angezeigt, ohne Schlüssel; erst nach «Testen» geht ein Aufruf mit Schlüssel an diesen Host; meldet Seerr später einen anderen Host, halten `wishes.tag` und `wishes.progress` an bis zum nächsten «Testen»; Schlüssel nie in Antwort, DB oder Log (N-23) | Test (Mock, Mock-Arr) |
| AK-172 | Adresse aus Seerr ist ein Docker-interner Name | «Testen», dann «Eigene Adresse» eintragen | erst Warnung «nicht erreichbar», mit eigener Adresse grün; der Schlüssel kommt weiter aus Seerr; Fakt «Tag Requests: an/aus» sichtbar (N-23, N-28, F-30) | Test; Hand |
| AK-173 | Noahs Filmwunsch wird «In Beschaffung», Film in Radarr vorhanden | nächster Abgleich | Tag `sc-noah` in Radarr angelegt (falls neu) und an den Film gehängt mit `applyTags: "add"`; Anfragen zeigt «sc-noah» als gesetzt; bei einer Serie dasselbe in Sonarr an der Serie (N-24) | Test (Mock-Arr); Hand |
| AK-174 | Wunsch «In Beschaffung», Film noch nicht in Radarr | Abgleich | Tag bleibt «noch nicht gesetzt»; sobald der Film in Radarr steht, setzt der nächste Lauf ihn | Test |
| AK-175 | Radarr gestoppt | Abgleich | Wunsch unverändert; «Tag noch nicht gesetzt», Hinweis in der Übersicht; nach dem Start setzt der nächste Lauf den Tag (N-25) | Test |
| AK-176 | Tag gesetzt | Wunsch zurückziehen, Konto löschen, Präfix ändern | kein Aufruf, der einen Tag entfernt oder umbenennt; Mock-Arr zählt nur die sechs Tag-Endpunkte (darunter die lesenden `movie?tmdbId` und `series?tvdbId` für den Dateistand) und das lesende `GET /api/v3/queue` aus `08-datenmodell-api.md`, Abschnitt 6, nie `remove` oder `replace` (N-22, N-26) | Test (Mock-Arr zählt) |
| AK-177 | Marcos Wunsch hat einen Tag; Noah (mit Wunschrecht) wünscht mit; ein Wunsch wurde direkt in Seerr gestellt | Abgleich | Noah bekommt `sc-noah` nachträglich am selben Titel; der direkt gestellte Wunsch bekommt keinen Profil-Tag, auch nachdem «Anfragen übernehmen» ihn dem Hauptprofil zugeordnet hat (N-24, N-27; Beispiel nach K-17) | Test |
| AK-178 | zwei Profile «Kinder», zuerst im Konto marco, später im Konto grosseltern angelegt | Tags ansehen; einen Namen auf `Kids!` setzen | das erste behält `sc-kinder`, das später angelegte bekommt `sc-kinder-grosseltern`; kein bestehender Tag wird umbenannt; der ungültige Name wird abgewiesen (422), `sc-kids` geht (N-21, Redaktionsentscheid zu K-18) | Test |

### Webhooks von Radarr und Sonarr (Stufe 4, zweite Rückmeldung)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-196 | Stufe 4 eingespielt, Webhook in Radarr von Hand eingerichtet | Integrationen › Webhooks von Radarr und Sonarr; Radarr meldet einen Import | Seite, Felder, Aktionen und Meldungsprotokoll wie in 1.40; die Meldung reiht genau einen `scan.path` ein wie heute; Mock-Arr zählt keinen Aufruf an `/api/v3/notification` und `/api/v3/history`; Slouchcat trägt in Radarr und Sonarr keinen Webhook ein (N-80, E-6) | Test (Mock-Arr zählt); e2e (bestehender Test unverändert grün) |

### Download-Fortschritt (Stufe 4, zweite Rückmeldung)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-197 | Noahs Filmwunsch «In Beschaffung», Film in Radarr ohne Datei, kein Eintrag in der Warteschlange | nächster Lauf von `wishes.progress` | «Meine Wünsche» und Wunschkarte der Vorschauseite zeigen «Wird gesucht» (N-83) | Test (Mock-Arr) |
| AK-198 | Eintrag in der Warteschlange mit `movieId` des Films, `size` 1000, `sizeleft` 550, `timeleft` 12 min | nächster Lauf | «Wird heruntergeladen · 45 % · noch 12 min»; dieselbe Anzeige in Verwaltung › Anfragen (N-83) | Test; Hand auf CT 190 mit echtem Radarr |
| AK-199 | Radarr meldet `hasFile = true`, Slouchcat hat den Film noch nicht eingelesen | nächster Lauf | «Wird eingelesen»; der Wunsch bleibt «In Beschaffung», bis Slouchcat den Film abspielen kann, dann «Verfügbar» wie heute (N-83, N-87) | Test |
| AK-200 | Eintrag mit `trackedDownloadStatus` `warning` und `errorMessage` | nächster Lauf | Zuschauer: «Download verzögert», **ohne** die Meldung; Verwaltung: «Download verzögert», die Meldung in der Aufklappzeile, Hinweis in der Übersicht, Filter «Download verzögert» (N-83) | Test (Vertrag: `WishProgress` ohne `message`); e2e |
| AK-201 | Serienwunsch Staffeln 1 und 2, 8 von 10 Folgen in Sonarr vorhanden | nächster Lauf | Zusatz «8 von 10 Folgen da»; ohne `seriesId` im Eintrag der Warteschlange kein Download-Fortschritt, aber keine falsche Anzeige (Anhang H 8.3: Sonarr-Felder nicht geprüft) | Test; Hand, sobald in Marcos Sonarr ein Download läuft |
| AK-202 | Wunsch zu Film X mit Fortschritt; Radarr lädt auch Titel, die in der Bibliothek, in Reihen oder in der Suche stehen | Detailseite eines Bibliothekstitels, Startseite, Suche, Bereich «Entdecken» | kein Fortschritt ausser beim Wunsch (N-82) | e2e |
| AK-203 | Warteschlange enthält einen Download eines Titels, den niemand über Slouchcat gewünscht hat | Lauf, dann Verwaltung › Anfragen und «Meine Wünsche» | der Eintrag wird nicht gespeichert und nirgends gezeigt; jeder Wunsch zeigt nur den Eintrag mit seiner Radarr- bzw. Sonarr-ID | Test |
| AK-204 | Radarr gestoppt | Lauf, dann Seiten ansehen | Zuschauer: Wunsch «In Beschaffung» ohne Zusatz (nach spätestens 5 min auch keine alte Anzeige mehr); Verwaltung «Fortschritt nicht lesbar» mit Zeit; nach dem Start von Radarr wieder Anzeige (N-85) | Test (Uhr-Mock) |
| AK-205 | «Download-Fortschritt zeigen» aus | eine Stunde mit Wünschen in Beschaffung | kein Aufruf `GET /api/v3/queue`; keine Anzeige in App und Verwaltung; der Profil-Tag läuft weiter (N-86) | Test (Mock-Arr zählt) |
| AK-206 | zwei Wünsche in Beschaffung (Film, Serie) | 10 min laufen lassen, dabei «Meine Wünsche», Vorschauseite und Anfragen je 20-mal laden | höchstens ein `GET /api/v3/queue` je Minute und Dienst; kein Aufruf an Radarr oder Sonarr beim Seitenaufbau; ohne Wünsche in Beschaffung gar kein Aufruf (N-84) | Test (Mock-Arr zählt) |
| AK-207 | Fortschritt springt zwischen «Wird heruntergeladen» und «Download verzögert» | Lauf | Zustand des Wunsches unverändert `processing`, kein Eintrag in `wish_events`, keine Benachrichtigung (N-87) | Test |
| AK-208 | Mock-Arr ohne `size` und `timeleft` im Eintrag | Lauf | «Wird heruntergeladen» ohne Prozent und ohne Restzeit; Diagnose der Verwaltung «Fortschritt unvollständig» | Test |
| AK-209 | Stufe 4 | Volltextsuche über DB, Log und Antworten nach Pfaden aus Warteschlange und `movieFile.path` und nach dem Schlüssel von Radarr | kein Treffer | Test |

## 5a. Stufe 5: Wünsche-Seiten (Fassung 3)

Die Nummer 5a hält die Nummern der folgenden Abschnitte stabil, auf die andere Papiere verweisen.

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-179 | Profil Marco mit Recht | Eintrag «Wünsche» | Reiter «Entdecken», «Filme», «Serien», «Suche», «Meine Wünsche»; «Entdecken» zeigt die sieben Reihen aus Anhang H N-51; Daten vom Server, nie direkt von Seerr oder TMDB im Browser (Netzwerk-Ansicht) | e2e; Hand |
| AK-180 | Profil Mia ohne Recht; Konto ohne Seerr-Benutzer | Eintrag «Wünsche», Aufruf der Seiten per Adresse | nur «Meine Wünsche»; die vier Operationen antworten 404 (F-9, N-56) | Test; e2e |
| AK-181 | Profil Noah (12); Mock liefert trotz `certificationLte` einen Titel ab 16 und einen ohne Freigabe | «Entdecken», «Filme», «Suche» | Anfrage an Seerr trägt `certificationLte`; beide Titel fehlen in jeder Liste; Freigaben über `external_titles` geprüft (N-56) | Test |
| AK-182 | Film X steht in der Bibliothek und ist für Marco sichtbar; Film Y steht dort, ist für Noah zu hoch; Mock meldet für Z `mediaInfo` «verfügbar», Slouchcat hat Z nicht | «Entdecken» für Marco und Noah | X mit Marke «In der Bibliothek» und Link `/items/{id}`; Y fehlt für Noah; Z erscheint als fehlend mit «Film anfordern» (N-55) | Test |
| AK-183 | Serie in der Bibliothek nur mit TVDB-ID | erscheint in «Serien» | als vorhanden markiert (Abgleich TVDB zuerst) | Test |
| AK-184 | – | Suche nach `tvdb:81189`, `imdb:tt0903747`, `tmdb:1396` und nach einem Personennamen | die drei Kennungen finden den Titel; Personen erscheinen nicht (N-53) | Test (Mock) |
| AK-185 | «Filme» geöffnet | Sortierung «Neueste zuerst», Genre «Drama», «Vorhandene ausblenden» | Liste entsprechend; der Filter ist zugeklappt und zeigt die Wahl im Kopf; «Mehr laden» holt Seite 2 (N-52) | e2e |
| AK-186 | Liste in den letzten 60 min geladen | gleiche Liste, gleiche Sprache, gleiche Altersstufe erneut; dann mit Profil Noah | kein Aufruf an Seerr; für Noah ein eigener Eintrag; Seerr gestoppt → älterer Eintrag mit «Stand von {{Zeit}}» (N-54) | Test (Mock zählt) |
| AK-187 | «Seiten Entdecken, Filme, Serien und Suche» aus | Seitenleiste, Bibliothekssuche | nur «Meine Wünsche»; die Bibliothekssuche verweist nicht auf die Wunsch-Suche; `wishes.discover` ruht (N-57) | e2e |
| AK-188 | neue Seiten | Tastatur | Tastaturtest je Seite (Pfeiltasten in Reihen und Raster, Tab, Enter, Zurück, sichtbarer Fokus, keine Fokusfalle im Filter; Regel 8) | e2e |

## 6. Sicherheit (alle Stufen)

| Nr. | Voraussetzung | Handlung | Erwartet | Prüfung |
|---|---|---|---|---|
| AK-90 | Mutation ohne `X-Requested-With` | `createWish` | 403 `csrf_header_missing` | Test |
| AK-91 | ohne Profil | `listWishes` | 403 `profile_required` | Test |
| AK-92 | Antworten von Seerr enthalten Adressen | Abgleich, Entdecken, Suche | Slouchcat ruft keine Adresse aus einer Antwort auf; einzige Ausnahme ist die Adresse von Radarr bzw. Sonarr aus `/settings/radarr|sonarr`, und die erst nach «Testen» durch den Administrator (AK-171) | Test (Mock mit Fallen-URL) |
| AK-93 | Bild eines fehlenden Titels | Browser | Bild kommt von Slouchcat, nie von `image.tmdb.org` | Test; Hand (Netzwerk-Ansicht) |
| AK-94 | Administratorseite | Antworten durchsuchen | Schlüssel nie in einer Antwort | Test |
| AK-135 | Verwaltungsantworten der Wünsche (`listAdminWishes`, `getSpotlightEntries`) | Antwort prüfen | keine `ItemRef`, keine scoped Adresse; Poster über `getAdminExternalTitleImage` bzw. `getAdminItemImage` | Test (Vertrag) |
| AK-136 | weiteres Profil (Noah) | `listAccountWishes` | 403 `primary_profile_required`; die Operation ist `scoped` | Test (Vertrag) |

## 7. Performance

| Nr. | Messgrösse | Bedingung | Zielwert | Prüfung |
|---|---|---|---|---|
| AK-95 | Aufrufe an Seerr, Radarr und Sonarr beim Seitenaufbau | Startseite, Vorschauseite, «Meine Wünsche», Serienseite, Suche, Verwaltung › Anfragen | 0 (Radarr und Sonarr auch für den Fortschritt, AK-206) | Test (Mock und Mock-Arr zählen) |
| AK-96 | Serverzeit `getHome` | Spotlight mit 20 Einträgen, Abgleich über TMDB, TVDB und IMDb | p95 höchstens 10 ms über dem Wert von 1.40 mit nur TMDB, 50 Aufrufe (**Vorschlag**) | Messung `docs/perf/` |
| AK-97 | Bühnenbild der ersten Folie | Prepaint | unverändert gegenüber 1.40 (Messweg P-10) | Messung |
| AK-98 | `createWish` | Seerr erreichbar (Mock) | p95 ≤ 500 ms (**Vorschlag**) | Test |
| AK-99 | `createWish` | Seerr nicht erreichbar | ≤ 3,5 s (Zeitlimit 3 s) | Test |
| AK-100 | `listWishes` | 500 Wünsche im Profil | p95 ≤ 100 ms (**Vorschlag**) | Messung |
| AK-101 | Vorschauseite | Titel im Cache | Serverzeit p95 ≤ 100 ms (**Vorschlag**) | Messung |
| AK-102 | Aufrufe an Seerr | Ruhe, keine offenen Wünsche | höchstens 1 `GET /request` je 5 min | Test |
| AK-103 | Rate-Limit | Burst von 50 Aufrufen | höchstens 5 je Sekunde an Seerr | Test |
| AK-104 | Zeit bis «Verfügbar» | Radarr-Webhook bis Meldung | ≤ Scan + Match + 1 min (Haken), sonst ≤ 1 h (Nachlauf); gemessen je Abschnitt, als Grundlage für Schritt 6a (E-11) | Hand; Messung `docs/perf/` |
| AK-137 | `searchExternalTitles` | Profil 12, 20 Treffer ohne Eintrag in `external_titles` | Serverzeit p95 ≤ 3,5 s (Zeitlimit 3 s für die Detailabrufe, **Vorschlag**); zweite gleiche Suche p95 ≤ 100 ms | Messung |
| AK-189 | `getWishDiscover` | Zwischenspeicher warm, Profil 18 | Serverzeit p95 ≤ 100 ms (**Vorschlag**) | Messung |
| AK-190 | `getWishDiscover` | kalt, Profil 12 (Vorfilter und Nachprüfung) | Serverzeit p95 ≤ 5 s (Zeitlimit, **Vorschlag**); höchstens 20 TMDB-Detailabrufe je Seite | Messung; Test (Mock zählt) |
| AK-210 | Lauf von `wishes.progress` | 20 Wünsche in Beschaffung, Warteschlange mit 50 Einträgen | Laufzeit p95 ≤ 2 s, höchstens 2 Aufrufe `GET /api/v3/queue` und keine Titelsuche, sobald die IDs bekannt sind (**Vorschlag**) | Messung `docs/perf/` |
| AK-212 | Schritt 6a wird gebaut, weil die Messung 6a.1 einen Gewinn zeigt (E-11) | Zeit von der Webhook-Meldung bis «Verfügbar», Film und Folge, vor und nach Schritt 6a | Messung vorher liegt in `docs/perf/`, bevor 6a.2 beginnt (AK-226); nachher kürzer als vorher, sonst bleibt der Empfang wie heute (N-98) | Messung |

## 8. Abnahme der Mockups

Marco prüft das Mockup (`docs/planning/mockups/25-medienwuensche/index.html`) vor jedem Bau. Bilder liegen nur
lokal in `img/`; ohne sie zeigt das Mockup Platzhalter mit Titel.

| Kennung | Was Marco prüft |
|---|---|
| M-00 | Leiste «Mockup»: Profil wechseln (Marco, Sandra, Noah, Mia, Oma & Opa, Gast), Seerr-Zustand (verbunden, nicht erreichbar), Abläufe vorspulen (freigeben, verfügbar), Handy/Desktop, Zurücksetzen; neu: Limit ausschöpfen, Radarr/Sonarr ausfallen lassen, Staffelquelle von Seerr umstellen, Einladung annehmen, Anmeldung spielen; aus der zweiten Rückmeldung «Download weiter» und «Download hängt» |
| M-01 | Startseite: Spotlight (Bühne und Posterreihe) nur mit Titeln der Bibliothek, wie heute, auch für Profile, die anfordern können; Bühne ohne Wunsch-Chip und ohne Staffelhinweis (K-3); Reihe «Erfüllte Wünsche» nach «Weiterschauen», nur mit erfüllten Wünschen (Anhang H 8.8) |
| M-02 | Spotlight-Liste unverändert gegenüber heute: nur Titel der Bibliothek, kein Filter «Noch nicht da» (Anhang H 8.8) |
| M-03, M-04 | Vorschauseite Film und Serie wie eine Detailseite, Hauptknopf oder Zustand; letzter geschlossener Wunsch mit Datum und Grund; ohne Recht nicht erreichbar (ausser mit eigenem Wunsch); Serie ohne TVDB-ID; Wunschkarte mit Download-Fortschritt (vier Anzeigen, bei Serien «x von y Folgen da») |
| M-05 | Detailseite Film (vorhanden): unverändert bis auf den Chip «Dein Wunsch ist da» (14 Tage nach der Erfüllung). Dieser Chip steht nur hier; erfüllte Wünsche stehen sonst in der Reihe «Erfüllte Wünsche», nie auf der Bühne (K-3) |
| M-06 | Serienseite: gedämpfte Karten, «Staffeln anfordern», «2 von 4 Staffeln vorhanden», nächste Folge; Profil ohne Recht ohne diese Teile |
| M-07 | Suche: Bibliothek oben, darunter «Noch nicht in der Bibliothek»; Profil Mia ohne Abschnitt; neu der Verweis auf die Wunsch-Suche (M-34) |
| M-08 | «Meine Wünsche»: Filter, Zustände aller acht Schlüssel, Verlauf, Zurückziehen, erneut anfordern, Kennzeichen «In Seerr angelegt»; neu als fünfter Reiter des Bereichs «Wünsche» mit Anzeige des Limits; bei Wünschen in Beschaffung der Download-Fortschritt («Wird gesucht», «Wird heruntergeladen · n % · noch m min», «Wird eingelesen», «Download verzögert»), ohne Meldung von Radarr oder Sonarr |
| M-09, M-10 | neue Eingangsarten mit Link; neue Ereignisse je Kanal, «Freigabe nötig» nur bei Administratoren |
| M-11 | Profildialog mit «Darf Titel anfordern», gesperrt, wenn das Konto nicht darf; Chip auf der Karte |
| M-12 | Profilwahl unverändert |
| M-13 | Dialog: «Wird als … übermittelt», «Wird sofort beschafft» / «Braucht eine Freigabe», Staffelwahl mit «Bereits angefordert» (Staffeln aus Seerr), Mitwunsch «Benachrichtige mich», nicht erreichbar; Limit: frei, erreicht mit Datum des nächsten freien Platzes, zu viele Staffeln |
| M-20 | Übersicht: Kennzahl und die neuen Hinweise (Freigabe, nicht erreichbar, Konten ohne Seerr-Benutzer, «Neu verknüpft: {{Konto}} mit dem bestehenden Seerr-Benutzer {{Name}}» mit Link auf «Seerr-Benutzer» (N-93), Limit erreicht, Tag nicht gesetzt, Download verzögert); kein Zähler für vorgeschlagene Verknüpfungen |
| M-21, M-22 | siebte Kachel «Seerr» nach «Webhooks von Radarr und Sonarr» mit Hinweisen im Zustand, Zeile oben mit neun Statusschlüsseln; Bänder «Verbindung» (Prüfliste mit den neuen Prüfpunkten, Warnungen gelb, Test trotzdem gelungen), «Abgleich» (mit dem Fakt zum Fortschritt), «Benutzer und Limit», «Radarr und Sonarr (Profil-Tags und Fortschritt)» mit Adresse aus Seerr, eigener Adresse, «Testen», «Tag Requests», Fakt «Download-Fortschritt» und Link auf die unveränderte Seite «Webhooks von Radarr und Sonarr», Prüfpunkte «Radarr/Sonarr für Profil-Tags und Fortschritt», «Sofortmeldung», «Protokoll» (Route `#/admin/integrations/jellyseerr`); die Seite «Webhooks von Radarr und Sonarr» unverändert |
| M-23 | «Anfragen»: Kennzahlen, Filter (auch «Tag nicht gesetzt» und «Download verzögert»), Tabelle mit Profil-Tag je Wunsch und Zustand samt Fortschritt, Aufklappzeile mit der Meldung von Radarr oder Sonarr, Aktionen, Band «Limit je Konto» |
| M-23 | Route `#/admin/wishes`, Reiter «Wünsche» im Bereich Medien direkt nach «Pflege» |
| M-24 | «Seerr-Benutzer» (vorher «Zuordnung», `#/admin/wishes/mapping`): je Konto ein zugeklapptes Band (keine Tabelle), im Kopf E-Mail, Rolle, Zustands-Chip, Benutzername und Limit kurz; aufgeklappt Benutzer mit Name und ID, Zustand («Von Slouchcat eröffnet», «Verknüpft über gleiche E-Mail», «Von Hand zugeordnet», «Wird eröffnet, sobald Seerr erreichbar ist», «Nicht eröffnet»; bei «Verknüpft» der Auslöser mit Datum), Passwort, Rechte in Seerr, Limit inklusive direkter Wünsche in Seerr, Aktionen «In Seerr eröffnen», «Alle eröffnen», wählen, lösen, Passwortmodus; neutraler Hinweis bei einem Konto, das beim Annehmen der Einladung verknüpft wurde (N-93); kein Zustand «Verknüpfung vorgeschlagen» und keine Aktion «Verknüpfung bestätigen» (E-8); bei ausgeschaltetem Eröffnen beide Eröffnen-Knöpfe gesperrt mit Grund und Link; je Profil Tag-Name und zugeklappt die Ausnahme; Liste der Seerr-Benutzer ohne Konto; sofort wirksam |
| M-25 | «Einstellungen» (`#/admin/wishes/settings`): alle Schalter aus Anhang F 4, sofort wirksam ohne «Speichern»; «Wünsche aktiv» steht im Mockup zur Vorführung an, ab Werk ist es aus; neu die Bänder «Limit ab Werk», «Konten in Seerr» (Hilfe zu «Neue Konten in Seerr eröffnen»: bei gleicher E-Mail sofort verknüpft, die Adresse tippt das neue Konto selbst ein, die Übersicht zeigt jede solche Verknüpfung sieben Tage lang), «Radarr und Sonarr: Profil-Tag und Fortschritt» mit dem Schalter «Download-Fortschritt zeigen» und der Schalter für die Seiten Entdecken, Filme, Serien und Suche; **kein** Schalter «Aus dem Spotlight» und kein Verweis auf fehlende Titel im Spotlight |
| M-26 | Konten: Spalte «Wünsche», Formular «Zugriff» mit Schalter, Limit («Standard», «Eigenes», «Unbegrenzt») und Zeile «Seerr-Benutzer»; Einladung mit Limit und Hinweis auf dasselbe Login; Passwortänderung mit Hinweis auf Seerr; Rückfrage beim Löschen (Rechte und Zufallspasswort nur bei eröffneten Benutzern, sonst «bleibt unverändert», K-2); alle heutigen Felder vorhanden |
| M-27 | Spotlight: Band «Fehlende Einträge» ohne Schalter; Einträge mit Poster, Zustand, IDs und «Anfordern» wie in Fassung 3; Band «Abgleich über TMDB, TVDB und IMDb»; alle elf Felder des Bands «Quelle» ohne neue Hinweise |
| M-28 | Startseite: Reihe «Erfüllte Wünsche» in der Tabelle, Ausklappzeile nennt die Regel «in den letzten 14 Tagen erfüllt» |
| M-29 | Benachrichtigungen der Verwaltung: neue Ereignisse |
| M-30 | Hintergrundjobs: zwölf neue Jobarten mit Takt (neu `seerr.provision`, `wishes.tag`, `wishes.discover`, `wishes.progress` jede Minute); Erstabgleich mit Fortschritt |
| M-31 | Wünsche · Entdecken (neu): Reihen, Genre-Kacheln, Suchfeld; Bibliothekstitel mit Marke |
| M-32 | Wünsche · Filme (neu): Raster, Filter zugeklappt mit Wahl im Kopf, Sortierung, «Vorhandene ausblenden», «Mehr laden» |
| M-33 | Wünsche · Serien (neu): wie M-32 |
| M-34 | Wünsche · Suche (neu): Suche, auch nach Kennung `tmdb:`, `tvdb:`, `imdb:`; keine Personen |
| M-35 | Einladung annehmen (heute vorhanden, im Mockup neu): Hinweis «Dein Login gilt auch für Seerr», Eröffnung beim Absenden; über dem Formular die Fakten der Einladung (Notiz, Altersstufe, ob das Konto Titel anfordern darf und mit welchem Limit, gültig bis), darunter der Hinweis «Dein Login gilt auch für Seerr»; nach dem Absenden in beiden Fällen (Adresse in Seerr neu oder schon vorhanden) dieselbe Meldung «In Seerr hast du jetzt ein Konto mit derselben E-Mail-Adresse.»; das Mockup verknüpft eine vorhandene Adresse sofort (E-8, N-94) |

Seit Marcos Antworten vom 17. November 2026 baut der Bau an drei Stellen anders, als das Mockup zeigt: keine
Ausnahme je Profil in M-24 (F-27), Sammelbenutzer «slouchcat-gaeste» ab Werk ohne Auswahl in M-25 (F-4), Passwort
«Wird nicht abgeglichen» bei vom Administrator verknüpften Konten in M-24 und M-26 (F-24). Das Mockup bleibt
unverändert; die Liste steht in `mockup-abdeckung.md`, Abschnitt «Abweichungen des Mockups».

Für alle Seiten gilt: Gestaltung nach Marcos Regeln vom 16. November 2026 (Klappbares zugeklappt, genug
Kontrast, ein Formular), Texte in Schweizer Schreibweise, keine Funktion der heutigen Seiten fehlt (ADR 0035:21-22),
Handy- und Desktopansicht bedienbar.
