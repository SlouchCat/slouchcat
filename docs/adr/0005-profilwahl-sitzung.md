# ADR 0005: Profilwahl in der Sitzung, Sichtbarkeitsstufe `account`

- Status: angenommen (19. Oktober 2026)
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitte 3.3, 3.8 (Punkte 2, 7, 9), 5.4, 10.2 und M2;
  Entscheid 6 in `docs/planning/04-entscheidungen.md` (AN-11, AN-14); Spec K-1, K-2, K-4, K-6, K-7
- Betroffene Spec-IDs: K-1 bis K-4, K-6, K-7, C-3

## Kontext

Bis M1b hatte jedes Konto genau ein Profil; die Anmeldung band die Sitzung fest an dieses Profil. Ab M2
hat ein Konto mehrere Profile (Kinder mit Altersstufe, PIN, Bibliotheksrechten), und Gäste bringen
eigene Konten mit. Drei Fragen waren zu entscheiden:

1. Wie kommt das Profil in die Sitzung, und was gilt zwischen Anmeldung und Profilwahl?
2. Wer darf Profile verwalten, wenn alle Profile eines Haushalts dasselbe Konto-Passwort teilen und ein
   Kind im Profil «P6» dasselbe Cookie hält wie der Elternteil?
3. Wie passt das in die vier Sichtbarkeitsstufen des Vertrags (`public`, `scoped`, `system`, `admin`),
   die der Schema-Test und die Middleware durchsetzen?

Verworfen: (a) Anmeldung bindet weiterhin das Standardprofil und die Oberfläche zeigt danach den
Wähler – dann hätte ein PIN-geschütztes Hauptprofil die PIN nie verlangt (K-4). (b) Ein kurzlebiges
«Login-Ticket» ohne Sitzung bis zur Profilwahl – zweiter Token-Typ, zweite Ablauflogik, kein Gewinn.
(c) Profilwechsel über eine neue Sitzung mit neuem Cookie – sauber, aber unnötig: der Widerruf der
Wiedergabesitzungen reicht, damit Stream-Tokens des alten Profils sofort sterben.

## Entscheid

| Punkt | Entscheid |
|---|---|
| Sitzung ohne Profil | `sessions.profile_id` ist nullable. Die Anmeldung legt eine Sitzung **ohne** Profil an; Ausnahme: genau ein Profil ohne PIN wird sofort gewählt. `SessionInfo.profile` ist optional; der Client zeigt bei `null` den Wähler «Wer schaut?» |
| Neue Stufe `account` | Operationen, die eine gültige Sitzung, aber kein gewähltes Profil brauchen: `getSession`, `logout`, `listMyProfiles`, `selectProfile` und die Profilverwaltung des eigenen Kontos. Der Schema-Test erlaubt den Wert; Antworten mit `ItemRef` müssen weiterhin `scoped` sein, deshalb kann `account` keine Titel preisgeben |
| `scoped` | braucht Sitzung **und** Profil; eine Sitzung ohne Profil erhält `403 profile_required` (kein 401, die Sitzung ist ja gültig) |
| `admin` | braucht das Hauptkonto **und** dessen aktives **Hauptprofil** (`isPrimary`). Ein Kinderprofil des Hauptkontos erhält 403 – die Rolle allein reichte in M1a, weil es nur ein Profil gab |
| Hauptprofil | Jedes Konto hat genau ein Hauptprofil (bisher `is_default`), bei der Kontoerstellung angelegt, nicht löschbar. Nur mit aktivem Hauptprofil darf eine Sitzung Profile des Kontos anlegen, ändern, löschen oder PINs setzen (`403 primary_profile_required`). Gäste verwalten so ihre eigenen Profile; der Elternteil schützt sein Hauptprofil mit einer PIN, sonst ist die Verwaltung für jeden im Haushalt offen (wie bei Netflix «Profilsperre») |
| Profilwechsel | `POST /auth/profiles/{id}/select` setzt `profile_id` **in place**, widerruft alle `playback_sessions` der Sitzung (Stream-Tokens des alten Profils antworten sofort 401), beendet deren Streams und sendet `Clear-Site-Data: "cache"`, damit der Browser-Bildcache des vorherigen Profils fällt (offener M1a-Punkt). Browser beachten `Clear-Site-Data` nur in sicheren Kontexten (HTTPS oder `localhost`); im Heimnetz über reines HTTP bleibt der Bildcache stehen, und dort ist `p=<profileId>` in den Bild-URLs die eigentliche Absicherung: Cache-Einträge treffen nie profilübergreifend, weil die URL das Profil trägt (der Server ignoriert `p`) |
| PIN | 4–8 Ziffern, argon2id wie Passwörter. Sperre je Profil **und Gerät** (AN-11): ab dem 3. Fehlversuch 30 s, dann 60 s, 120 s … bis 15 min, in `pin_attempts` gespeichert (überlebt Neustarts); `429 pin_locked` mit `Retry-After`. Der Fehlversuch, der die Sperre auslöst, antwortet schon mit 429. **Das Gerät ist das der Sitzung** (`sessions.device_id`): Es wird beim Anlegen der Sitzung aus dem Cookie `slouchcat_device` (Zufalls-ID, von der Middleware gesetzt) übernommen, bei fehlendem Cookie zufällig vergeben und danach nie mehr geändert. Wer Cookies löscht oder tauscht, behält so den Zähler seiner Sitzung; wer sich mit neuem Cookie neu anmeldet, bekommt ein neues Gerät, braucht dafür aber das Konto-Passwort. Dagegen gilt zusätzlich eine **Obergrenze je Profil über alle Geräte**: mehr als 20 Fehlversuche innerhalb einer Stunde sperren das Profil für alle 15 min (`429 pin_locked`; Fensterbeginn und Zähler stehen in `pin_attempts` beim reservierten Pseudo-Gerät `ffff…`). Ein Zähler, dessen letzter Fehlversuch länger als **24 h** zurückliegt, beginnt wieder bei null; ein richtiger PIN setzt den Zähler des Geräts zurück. Geräte und Zähler, die 400 Tage unbenutzt sind, räumt dieselbe Aufräumung ab wie abgelaufene Sitzungen. Zusätzlich zählt der IP-Begrenzer (10 Versuche/min je Client-Adresse, nur für PIN-Profile). Eine vierstellige PIN schützt nur gegen Raten über den Server |
| Gastkonten | Einladung einmalig, 7 Tage, widerrufbar (AN-14); das Gastkonto trägt eine Freigabe-Obergrenze (`accounts.max_rating`) und eine Bibliotheksliste (`account_libraries`); die View `visible_items` wendet `min(profile.max_rating, account.max_rating)` und beide Bibliothekslisten an. Entzug (K-7) widerruft zuerst alle Sitzungen (eigene, übergebene Anweisung: die nächste Anfrage ist ein 401 und kein Stream-Token lässt sich mehr prüfen), beendet dann alle Streams des Kontos (auch über die Registry im Speicher, nicht nur über die Zeilen), löscht das Konto und beendet danach nochmals, was zwischendurch noch geöffnet wurde. Wer Freigaben senkt (Obergrenze, Bibliotheksrecht, Profilstufe, Titel-Übersteuerung), beendet damit auch offene Direktstreams, die er verdeckt (`RecheckVisibility`); HLS prüft ohnehin bei jeder Anfrage |

## Folgen

- Vertrag: fünf Sichtbarkeitsstufen; der Vertragstest 10.2 deckt jetzt alle Operationen ab, nicht nur
  die `scoped`-Operationen (`account`: keine Item-IDs in Antworten; `admin`: 403 für Gäste und für
  Nicht-Hauptprofile).
- Datenmodell: `sessions` wurde in Migration 00007 neu aufgebaut (SQLite kennt kein
  `ALTER COLUMN`), `playback_sessions` referenziert weiter `sessions`.
- Web: Routen hinter `_authed` verlangen `session.profile`; sonst Umleitung auf `/profiles`.
- Spätere Meilensteine: Bearer-Tokens für Fremd-Clients (C-5) hängen an derselben Sitzungstabelle und
  erben die Profilwahl; Offline (M5) speichert `allowed_profile_ids` je Download, der Wähler bleibt der
  einzige Ort, an dem ein Profil aktiv wird.

## Nachtrag vom 28. Oktober 2026 (Benutzerverwaltung, ADR 0014)

Zwei Zeilen der Entscheidtabelle oben sind abgelöst, der Rest gilt wörtlich weiter.

- **`admin` verlangt nicht mehr «das Hauptkonto».** Es gibt ab ADR 0014 mehrere Administratoren
  (`accounts.role = 'admin'`); die Stufe verlangt **ein Administratorkonto** mit seinem aktiven
  Hauptprofil. Die Hauptprofil-Bedingung selbst bleibt unangetastet — sie ist der Grund, weshalb ein
  Kinderprofil am Elterngerät nicht an die Bibliotheksverwaltung kommt, und sie gilt jetzt für mehr
  Haushalte statt für weniger.
- **Es gibt eine sechste Stufe `owner`.** Sie trägt die Operationen, die nur der Besitzer ausführen
  darf (`setAccountRole`), und erbt dieselbe Hauptprofil-Bedingung. Der Besitzer steht nicht mehr in
  `role`, sondern in der neuen Spalte `accounts.is_owner`.
- **«Gastkonten» heissen nur noch «Konten».** Der Rollenwert `'guest'` entfällt; die Zeile
  «Gastkonten» oben beschreibt unverändert, was ein Konto ist und wie der Entzug greift (K-7), nur
  ohne das Wort «Gast».
