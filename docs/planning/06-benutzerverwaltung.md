# Konzept: Benutzerverwaltung (Administrator, Hauptbenutzer, Profil)

- **Status: Entwurf.** Marcos vier Entscheide vom 27. Oktober 2026 (Abschnitt 1.3) sind die Grundlage;
  das Konzept als Ganzes wartet auf seine Freigabe. Es wird nichts gebaut, bevor die Fragen in
  Abschnitt 11 beantwortet sind.
- **Datum: 27. Oktober 2026**
- **Wogegen dieses Konzept verankert ist:** Zweig `claude/konzept-benutzerverwaltung`, Stand `0dd4f19`
  («Merge pull request #43»). Jede Zeilenangabe in diesem Papier bezieht sich auf diesen Stand. Wer
  es später liest, prüft zuerst `git rev-parse --short main`; in einem Zweig mit eigenen Änderungen
  zeigen Zeilenangaben zwangsläufig daneben, ohne dass am Konzept etwas falsch wäre.
- **Letzte Migration ist `00021_spotlight.sql`**, die nächste freie Nummer ist darum **00022**
  (`server/migrations/`). **Letzte ADR ist `0013-ci-parallele-jobs.md`**, die nächste freie Nummer ist
  **0014**. **Vertragsfassung ist heute `1.1.0`** (`api/openapi.yaml:4`).
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
- **Jeder Schritt ist ein eigener Commit, und vor dem Abschluss jedes Schritts ist `task ci` grün**
  (`CLAUDE.md`, Abschnitt «Arbeitsweise»). DoD-Belege sind Befehl **und** Ausgabe.

> **Dieses Papier ändert keine Zeile Code.** Es ist ein Konzept, kein Auftrag. Die eigentliche Arbeit
> beginnt mit Schritt 0 aus Abschnitt 10, und auch der ist ein reiner Doku-Commit.

---

## 1. Ausgangslage und Problem

### 1.1 Was das heutige Modell kann

Slouchcat kennt heute genau **zwei Arten von Konten** und darunter beliebig viele Profile.

Die Tabelle `accounts` trägt dafür eine Spalte `role` mit genau zwei erlaubten Werten:

```sql
role TEXT NOT NULL DEFAULT 'owner' CHECK (role IN ('owner', 'guest'))
```

(`server/migrations/00002_accounts_sessions.sql:20`)

| Begriff heute | Was er technisch ist | Was er darf |
|---|---|---|
| **Besitzer** (`role = 'owner'`) | das eine Konto, das der Einrichtungsassistent anlegt (`server/internal/auth/auth.go:161`, `CreateOwner`); die Funktion wirft `model.ErrSetupDone`, sobald irgendein Konto existiert (`server/internal/auth/auth.go:194`) | alles: Server, Bibliotheken, Einstellungen, Jobs, Integrationen, Benachrichtigungen, dazu die eigenen Profile |
| **Gast** (`role = 'guest'`) | ein Konto, das über einen Einladungslink entsteht (`server/queries/app/access.sql:87`, `InsertGuestAccount`) | schauen, und die **eigenen** Profile verwalten; nichts verwalten, was den Server betrifft |
| **Profil** | eine Zeile in `profiles`, ohne E-Mail und ohne Passwort (`server/migrations/00002_accounts_sessions.sql:26`, Spec K-1) | schauen; nur das **Hauptprofil** eines Kontos darf dessen Profile verwalten |

Drei Mechanismen tragen das Ganze:

1. **Die Hauptprofil-Regel.** Jedes Konto hat genau ein Hauptprofil (`profiles.is_default`, im Vertrag
   `isPrimary`), bei der Kontoerstellung angelegt und nicht löschbar; ein eindeutiger Teilindex
   erzwingt die Einzigkeit (`server/migrations/00007_accounts_profiles_access.sql:38`). Nur eine
   Sitzung mit aktivem Hauptprofil darf Profile des Kontos anlegen, ändern, löschen oder PINs setzen,
   sonst `403 primary_profile_required` (ADR 0005, `docs/adr/0005-profilwahl-sitzung.md:34`;
   Fehlerzuordnung in `server/internal/api/session.go:340`, Zweig `model.ErrPrimaryProfileRequired`,
   definiert in `server/internal/model/model.go:68`).
2. **Die Sichtbarkeitsstufen des Vertrags.** Jede Operation in `api/openapi.yaml` trägt
   `x-visibility` mit einem von fünf Werten; die Middleware setzt sie durch
   (`server/internal/api/session.go:86` bis `:131`). Für `admin` verlangt sie heute wörtlich das
   Besitzerkonto **und** dessen Hauptprofil: `case vis == "admin" && (!session.IsOwner() || !session.IsPrimary())`
   (`server/internal/api/session.go:117`). `IsOwner()` ist schlicht `s.Account.Role == "owner"`
   (`server/internal/model/model.go:207`).
3. **Die Obergrenzen des Gastkontos.** Ein Gastkonto trägt eine eigene Freigabe-Obergrenze
   (`accounts.max_rating`) und eine Bibliotheksliste (`account_libraries`); die View `visible_items`
   verrechnet sie mit denen des Profils über `min(p.max_rating, a.max_rating)` und verlangt beide
   Bibliotheksrechte (`server/migrations/00007_accounts_profiles_access.sql:101` bis `:115`,
   die Verrechnung in `:108`).

### 1.2 Wo es für ein öffentliches Produkt nicht reicht

Slouchcat soll öffentlich angeboten werden und gegen Emby und Jellyfin antreten. Das heutige Modell
hat dafür vier harte Kanten:

- **Es gibt genau einen Administrator, und er ist nicht vermehrbar.** Wer den Server betreibt, ist
  derselbe, der den Einrichtungsassistenten ausgeführt hat. Fällt er aus, gibt es keinen Weg zu einem
  zweiten. `CreateOwner` verweigert jedes weitere Konto (`server/internal/auth/auth.go:194`), und kein
  Endpunkt ändert `accounts.role`. Emby und Jellyfin erlauben beide mehrere Administratoren.
- **Das Wort «Gast» ist für ein Produkt falsch.** Ein Gast ist bei Emby jemand mit weniger Rechten;
  bei Slouchcat ist er in Wahrheit ein vollwertiger Haushaltsvorstand mit eigenen Profilen, eigener
  PIN-Verwaltung, eigenen Benachrichtigungskanälen und eigener Offline-Bibliothek. Der Begriff
  verspricht etwas Kleineres, als er bedeutet. Das ist eine Produktaussage, kein Code-Problem — der
  Code tut bereits das Richtige.
- **Die Serververwaltung hängt an einer Kontozeile statt an einer Rolle.** Drei SQL-Abfragen fragen
  wörtlich `role = 'owner'` ab (`server/queries/system/notify_channels.sql:47` und `:95`,
  `server/queries/system/admin.sql:356`), neun weitere `'guest'`. Solange es genau einen Besitzer
  gibt, sind «der Besitzer» und «die Administration» dasselbe; sobald es zwei Administratoren gibt,
  sind sie es nicht mehr, und jede dieser Stellen trifft dann stillschweigend eine andere
  Entscheidung als die beabsichtigte (Abschnitt 12).
- **Es gibt keinen unzerstörbaren Anker.** Würde man heute mehrere Administratoren erlauben, könnte
  sich der Server selbst aussperren: Der letzte Administrator entfernt sein eigenes Konto, und
  niemand kommt mehr an die Verwaltung. Dagegen hilft keine Regel im Code allein — es braucht einen
  Besitzer, der weder löschbar noch degradierbar ist.

### 1.3 Marcos Entscheide vom 27. Oktober 2026

| Frage | Entscheid |
|---|---|
| **Zugang** | Konten entstehen **nur per Einladung durch die Administration** — einmalig, 7 Tage gültig, vorher widerrufbar, genau wie heute (AN-14, `docs/planning/04-entscheidungen.md:15`). **Keine Selbstregistrierung.** |
| **Administratoren** | Ein **Besitzer**, unlöschbar und nicht degradierbar, plus weitere Administratoren, die **nur der Besitzer** ernennen und ihnen die Rolle auch wieder entziehen kann. |
| **Weitergabe** | Ein Hauptbenutzer darf **keine** weiteren Nutzer einladen. Einladungen bleiben allein bei der Administration. |
| **Rollen** | Genau drei Ebenen: **Admin, Hauptbenutzer, Profil.** Die Rolle `guest` entfällt als eigener Begriff; bestehende Gastkonten werden Hauptbenutzer. Ihre Obergrenze (`accounts.max_rating`) und ihre Bibliotheksliste (`account_libraries`) gelten weiterhin **für jedes Nicht-Admin-Konto**. |

---

## 2. Zielbild

Drei Ebenen, von aussen nach innen:

```
Besitzer  ──┐
            ├─ Administrator  (Rolle: verwaltet Server und Bibliotheken)
weitere  ───┘
Administratoren
            │
            └─ Hauptbenutzer  (Konto: verwaltet die eigenen Profile)
                  │
                  └─ Profil    (schaut)
```

Der **Besitzer** ist keine vierte Ebene, sondern ein Administrator mit zwei zusätzlichen Rechten
(ernennen und entziehen) und zwei Schutzschildern (nicht löschbar, nicht degradierbar). Jedes
Administratorkonto ist gleichzeitig ein Konto mit eigenen Profilen — die Ebenen schliessen sich nicht
aus, sie stapeln sich.

### 2.1 Administrator

| Darf | Operationen aus `api/openapi.yaml` |
|---|---|
| Bibliotheken anlegen, ändern, scannen, Wurzeln hinzufügen und entfernen | `listLibraries` (`:2131`), `createLibrary` (`:2157`), `updateLibrary` (`:2227`), `scanLibrary` (`:2268`), `addLibraryRoot` (`:2368`), `removeLibraryRoot` (`:2414`), `browseServerPaths` (`:2479`) |
| Titel pflegen: Freigabe setzen, Zuordnung korrigieren, Trailer ersetzen, Duplikate zusammenführen | `setItemRating` (`:2829`), `setItemMatch` (`:2902`), `createItemTrailer` (`:2974`), `mergeItems` (`:3282`), `listMaintenanceIssues` (`:3194`) |
| Konten verwalten: einladen, widerrufen, Obergrenzen und Bibliotheksrechte setzen | `listInvitations` (`:2564`), `createInvitation` (`:2582`), `revokeInvitation` (`:2613`), `listAccounts` (`:4901`), die heutigen `updateGuest` (`:2655`) und `revokeGuest` (`:2692`) unter neuem Namen (Abschnitt 6) |
| Server betreiben: Einstellungen, Jobs, Sicherung, Update, Statistiken, Integrationen | `getAdminSettings` (`:4532`), `updateAdminSettings` (`:4552`), `retryJob` (`:4476`), `createBackup` (`:4677`), `restoreBackup` (`:4814`), `checkForUpdate` (`:4876`), `getStats` (`:4921`), `listIntegrations` (`:5393`) |
| Kuratieren: Startseite, Spotlight, Cat's Empfehlungen, Sammlungen, Ankündigungen | `setHomeRows` (`:4183`), `setSpotlightConfig` (`:4267`), `createTopPicks` (`:3816`), `setCollectionItems` (`:4125`), `publishAnnouncement` (`:5071`) |
| Laufende Streams sehen und beenden, Geräte verwalten | `listStreams` (`:3415`), `stopStream` (`:3441`), `listDevices` (`:3362`), `deleteDevice` (`:3388`) |

| Darf **nicht** | Warum |
|---|---|
| sich selbst oder andere zu Administratoren ernennen | nur der Besitzer ernennt (Entscheid 1.3). Die neue Operation `setAccountRole` trägt die Stufe `owner`, nicht `admin` (Abschnitt 5) |
| den Besitzer löschen oder degradieren | Besitzerschutz (Abschnitt 9) |
| in den Verwaltungsbereich, solange sein **Kinderprofil** aktiv ist | ADR 0005: `admin` verlangt das Hauptprofil (`docs/adr/0005-profilwahl-sitzung.md:33`). Ein Kind am Elterngerät kommt sonst an die Bibliotheksverwaltung |
| fremde Profile anlegen oder ändern | Profile gehören dem Konto; die Profilverwaltung ist `account` plus Hauptprofil-Regel, nie `admin` |

### 2.2 Hauptbenutzer

| Darf | Operationen aus `api/openapi.yaml` |
|---|---|
| die **eigenen** Profile anlegen, ändern, löschen | `listProfiles` (`:586`), `createProfile` (`:605`), `updateProfile` (`:638`), `deleteProfile` (`:675`) — alle `x-visibility: account` |
| PIN und Offline-PIN je eigenem Profil setzen und entfernen | `setProfilePin` (`:704`), `removeProfilePin` (`:736`), `setOfflinePin` (`:758`), `removeOfflinePin` (`:796`) |
| den Schauzeit-Bericht der eigenen Profile sehen (K-5) | `getWatchReport` (`:819`) |
| die eigene E-Mail und das eigene Passwort ändern | `updateAccount` (`:439`) |
| die Benachrichtigungskanäle des **eigenen** Kontos einstellen | `getMyChannels` (`:5139`), `updateMyChannel` (`:5160`), `testMyChannel` (`:5206`), `createTelegramLink` (`:5258`), `createPushSubscription` (`:5327`) |
| zwischen den eigenen Profilen wechseln, auch mit PIN | `listMyProfiles` (`:499`), `selectProfile` (`:520`) |
| die Bibliotheken sehen, die ihm freigegeben sind, um Profilrechte zu vergeben | `listAccountLibraries` (`:857`) — ausdrücklich ohne Titel, darum `account` (`api/openapi.yaml:861`) |

| Darf **nicht** | Warum |
|---|---|
| Bibliotheken anlegen, scannen, Wurzeln ändern | Serververwaltung, bleibt `admin` |
| weitere Benutzer einladen | Entscheid 1.3 «Weitergabe». `createInvitation` bleibt `admin` |
| die eigene Obergrenze oder Bibliotheksliste anheben | `accounts.max_rating` und `account_libraries` setzt nur die Administration; sonst hebt sich jedes Konto selbst auf FSK 18 |
| Einstellungen, Jobs, Sicherung, Statistiken, Integrationen sehen | alles `admin` |
| Profile eines **anderen** Kontos sehen oder ändern | die Abfragen sind auf `account_id` der Sitzung verankert |
| diese Dinge mit einem Kinderprofil tun | Hauptprofil-Regel, `403 primary_profile_required` |

**Das Wichtigste an dieser Tabelle: sie beschreibt den heutigen Zustand.** Alle 24 Operationen mit
`x-visibility: account` existieren bereits und tun bereits genau das. Die Ebene «Hauptbenutzer» ist im
Vertrag seit M2 fertig gebaut; sie heisst nur anders.

### 2.3 Profil

| Darf | Operationen |
|---|---|
| schauen, suchen, merken, Fortschritt speichern, offline laden | die 39 Operationen mit `x-visibility: scoped` |
| sein eigenes Profil wählen und bei PIN-Schutz entsperren | `selectProfile` (`:520`) |
| Benachrichtigungen des Kontos **lesen** | ADR 0009: «Lesen jedes Profil» (`docs/adr/0009-administration-benachrichtigungen.md:36`) |

| Darf **nicht** | Warum |
|---|---|
| Profile anlegen, ändern, löschen, PINs setzen | Hauptprofil-Regel, `403 primary_profile_required` |
| Kanäle ändern | Entscheid vom 21. Oktober 2026 (`docs/planning/04-entscheidungen.md:46`): sonst leitet ein Kinderprofil die Benachrichtigungen des Kontos mit fremden Titeln auf sein Gerät um |
| irgendetwas Administratives | `admin` verlangt das Hauptprofil |
| Titel über der eigenen Altersstufe sehen — auch nicht im Verwaltungsbereich | K-3, View `visible_items` |

---

## 3. Konflikt mit der Spec

### 3.1 Das Nicht-Ziel im Wortlaut

`docs/spec/01-anforderungen-v1.md:68` sagt:

> Mandantenfähigkeit (mehrere gleichberechtigte Haushalte auf einem Server).

Gelesen als «mehrere gleichberechtigte Benutzer» verbietet dieser Satz genau das, was Marco bestellt
hat. Gelesen als «mehrere voneinander getrennte Haushalte mit je eigenem Bestand» verbietet er etwas
ganz anderes, und das bleibt verboten. Der Satz muss darum präzisiert werden, sonst entscheidet jede
Sitzung neu, welche Lesart gilt.

### 3.2 Was weiterhin **nicht** gilt

Auch nach der Umformulierung baut Slouchcat nichts davon:

| Bleibt Nicht-Ziel | Was das heisst |
|---|---|
| **Getrennte Bibliotheken je Haushalt** | Es gibt einen Medienbestand. Ein Hauptbenutzer bekommt aus diesem Bestand eine Teilmenge freigegeben (`account_libraries`), er bringt keinen eigenen mit und legt keine eigene Bibliothek an. `createLibrary` bleibt `admin`. |
| **Mandantentrennung der Daten** | Es gibt keinen `tenant_id`, keine getrennten Datenbanken, keine getrennten Caches, keine getrennten Transcode-Budgets. `items`, `media_files`, `tracks` und `libraries` sind gemeinsam; getrennt ist nur, **wer was sehen darf**, und das macht seit M2 die eine View `visible_items`. |
| **Ein gemeinsamer Medienbestand** bleibt die Grundannahme | Die Admission Control (Plan 3.5), der Bildcache, der Scanner und die Jobs rechnen mit einem Bestand. Nichts davon wird je Konto vervielfacht. |
| **Selbstregistrierung** | Entscheid 1.3. Jedes Konto entsteht durch eine Einladung der Administration. |
| **Ein Hauptbenutzer lädt weiter ein** | Entscheid 1.3. Keine Weitergabe von Zugang. |
| **Rechte-Vererbung oder Rollen-Baukasten** | Es gibt genau drei Ebenen plus den Besitzer, keine frei zusammensetzbaren Rechte. Ein Plugin- oder Rollensystem ist kein Ziel von Version 1. |

### 3.3 Textvorschlag für die Umformulierung

Der Satz in `docs/spec/01-anforderungen-v1.md:68` würde lauten:

> - Mandantenfähigkeit im Sinn getrennter Haushalte: getrennte Medienbestände, getrennte Bibliotheken
>   oder getrennte Daten je Konto. Es gibt genau einen Bestand; mehrere Konten teilen ihn, und die
>   Administration entscheidet je Konto über Altersstufe und Bibliotheksrechte. Mehrere Konten mit
>   eigenen Profilen (Hauptbenutzer) und mehrere Administratoren sind ausdrücklich **kein** Nicht-Ziel.

### 3.4 Wohin dieser Text gehört — und wohin nicht

`docs/spec/README.md:6` hält fest:

> Die Spec wird nicht nachträglich umgeschrieben; Abweichungen stehen im Entscheidungsprotokoll.

Der Vorschlag aus 3.3 wird darum **nicht in die Spec geschrieben**. Er wird als Teil von **Frage 12**
in `docs/planning/04-entscheidungen.md` festgehalten, zusammen mit Marcos vier Antworten, und in der
ADR 0014 begründet. Genau so ist M11 mit E-1, E-7 und E-9 verfahren
(`docs/planning/05-plan-spotlight.md:44` bis `:48`).

Das ist kein Formalismus, sondern die Rangfolge aus `CLAUDE.md`: **Entscheidungsprotokoll > Plan >
Spec.** Eine Zeile im Protokoll schlägt die Spec-Zeile 68, ohne dass die Spec angefasst werden muss;
eine stillschweigende Änderung der Spec dagegen würde die Rangfolge aushebeln und spätere Leser im
Unklaren lassen, wann und warum sich etwas verschoben hat. Ob Marco ausnahmsweise doch eine Änderung
an der Spec will, steht als Frage F-6 in Abschnitt 11.

---

## 4. Rollenmodell im Datenmodell

### 4.1 Was aus `accounts.role` wird

| Heute | Neu | Bedeutung |
|---|---|---|
| `'owner'` | `'admin'` | verwaltet Server und Bibliotheken |
| `'guest'` | `'user'` | Hauptbenutzer: verwaltet die eigenen Profile |

Die Spalte behält Name und Typ; nur die erlaubten Werte und der Standardwert ändern sich:

```sql
role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user'))
```

**Warum der Standardwert auf `'user'` wechselt:** Heute ist `'owner'` der Standard, weil M1a genau
ein Konto kannte. Ein vergessenes `role` bei einem `INSERT` erzeugt damit still einen Administrator —
das ist für ein öffentliches Produkt der falsche Fehlerfall. Mit `'user'` als Standard erzeugt
derselbe Fehler ein harmloses Konto. Die einzige Stelle, die bewusst einen Administrator anlegt, ist
der Einrichtungsassistent (`server/internal/auth/auth.go:196`, `InsertAccountParams{... Role: roleOwner}`),
und die nennt die Rolle ohnehin ausdrücklich.

**Warum nicht die gespeicherten Werte `'owner'`/`'guest'` behalten und nur im Vertrag umbenennen:**
Verworfen. Es gäbe dauerhaft zwei Vokabulare für dieselbe Sache, und `'owner'` wäre schlicht falsch,
sobald es zwei Administratoren gibt — die SQL-Zeile `WHERE a.role = 'owner'`
(`server/queries/system/notify_channels.sql:47`) liest sich dann als «der Besitzer» und meint «alle
Administratoren». Genau diese Zweideutigkeit ist der Fehler, den dieses Konzept beseitigt.

### 4.2 Wie der Besitzer markiert wird

Zwei Wege stehen zur Wahl.

| | **A: `accounts.is_owner`** | **B: Eintrag in `settings`** |
|---|---|---|
| Form | `is_owner INTEGER NOT NULL DEFAULT 0 CHECK (is_owner IN (0, 1))`, dazu `CREATE UNIQUE INDEX accounts_owner ON accounts (is_owner) WHERE is_owner = 1` | Schlüssel `owner_account_id` in der Tabelle `settings` (`server/migrations/00002_accounts_sessions.sql:10`) |
| Höchstens einer | **von der Datenbank erzwungen** (Teilindex) | nur im Code; zwei gleichzeitige Schreibzugriffe könnten zwei Werte hinterlassen |
| Was bei `DELETE FROM accounts` geschieht | die Markierung verschwindet mit der Zeile, es gibt keine Leiche | `settings` kennt keinen Fremdschlüssel und kein `ON DELETE`; der Eintrag zeigt auf eine gelöschte Zeile, und niemand merkt es |
| Lesbarkeit in Abfragen | `WHERE a.is_owner = 1` steht neben `WHERE a.role = 'admin'`, dieselbe Tabelle, ein Join weniger | jede Abfrage braucht einen Join auf `settings` und eine `CAST`-Umwandlung von `TEXT` nach `INTEGER` |
| Sicherung und Wiederherstellung | reist mit der Kontozeile, kann nicht auseinanderlaufen | reist in einer anderen Tabelle; eine Teilwiederherstellung könnte beides entkoppeln |
| Kosten | Tabellenumbau nötig — der ist für die `CHECK`-Änderung aus 4.1 ohnehin fällig | kein Umbau |
| Besitzerwechsel | eine Transaktion mit zwei `UPDATE` (alt auf 0, neu auf 1); der Teilindex verhindert den Zwischenzustand mit zwei Besitzern nur, wenn die Reihenfolge stimmt — darum in einer Transaktion und mit `UPDATE accounts SET is_owner = CASE id WHEN @new THEN 1 ELSE 0 END` in **einer** Anweisung | ein `UPDATE settings` |

**Empfehlung: A, `accounts.is_owner`.** Der Besitzer ist eine Eigenschaft eines Kontos, keine
Einstellung des Servers. Der entscheidende Punkt ist die referenzielle Wahrheit: `settings` ist eine
Schlüssel-Wert-Tabelle ohne Fremdschlüssel, und ein Zeiger auf eine gelöschte Kontozeile wäre genau
die Art von stiller Inkonsistenz, gegen die Abschnitt 9 ankämpft. Dazu kommt, dass die
`CHECK`-Änderung an `role` (4.1) den Tabellenumbau ohnehin erzwingt — die zusätzliche Spalte ist in
derselben Migration gratis.

Der Besitzer wird dabei **nicht** über `role` ausgedrückt: er trägt `role = 'admin'` **und**
`is_owner = 1`. Zwei Spalten statt eines dritten Rollenwerts, weil sonst jede Abfrage nach «ist
Administrator» zwei Werte prüfen müsste (`role IN ('admin', 'owner')`) — und genau dieses Vergessen
wäre eine Rechteausweitung mit einem einzigen fehlenden Wort.

### 4.3 Was mit `invitations` geschieht

**Nichts an der Struktur.** Die Tabelle (`server/migrations/00007_accounts_profiles_access.sql:80`)
trägt `max_rating`, `all_libraries`, `created_by`, `expires_at`, `used_at`, `used_by`, `revoked_at` —
alle bleiben, alle bedeuten dasselbe. Eine Einladung erzeugt weiterhin ein Konto mit
`role = 'user'`; `InsertGuestAccount` (`server/queries/app/access.sql:87`) schreibt die Rolle als
Literal in die `VALUES`-Zeile (`:89`), und dieses Literal wird von `'guest'` auf `'user'` gezogen.

**Ausdrücklich keine Spalte `role` in `invitations`.** Die Versuchung wäre, eine Einladung «als
Administrator» zu erlauben. Das ist abzulehnen: Eine Einladung ist ein Link, der sieben Tage in einem
Postfach liegt; wer ihn abfängt, bekäme damit die Serververwaltung. Entscheid 1.3 sagt ohnehin, dass
nur der Besitzer ernennt — und er ernennt ein **bestehendes** Konto, dessen E-Mail und Passwort schon
geprüft sind. Diese Abwesenheit einer Spalte ist eine Sicherheitsentscheidung und gehört als solche
in die ADR (Abschnitt 9, Punkt 4).

Was sich ändert, sind die **Texte**: `api/openapi.yaml:70` («Invitation links for guest accounts»),
`:919`, `:921`, `:6299`, `:6377`, `:6409` sprechen von «guest». Das sind Beschreibungen, keine
Struktur.

### 4.4 Was mit `account_libraries` geschieht

**Nichts.** Die Tabelle (`server/migrations/00007_accounts_profiles_access.sql:29`) und ihre Rolle in
`visible_items` (`:113` bis `:115`) bleiben Zeichen für Zeichen. Neu ist nur die Lesart: Sie gilt
nicht mehr «für Gäste», sondern **für jedes Konto mit `all_libraries = 0`**, und das sind nach
Entscheid 1.3 alle Nicht-Admin-Konten, die eine eingeschränkte Liste bekommen haben.

Dasselbe gilt für `accounts.max_rating` (`server/migrations/00007_accounts_profiles_access.sql:23`):
Die Obergrenze eines Kontos deckelt jedes seiner Profile, und die View verrechnet sie weiterhin mit
`min(p.max_rating, a.max_rating)`.

Für **Administratorkonten** kommt eine Bedingung dazu, die heute nur zufällig gilt:

```sql
CHECK (role <> 'admin' OR (max_rating = 18 AND all_libraries = 1))
```

Das ist eine zeilenweise `CHECK`-Bedingung und in SQLite zulässig. Begründung: Die Verwaltungsseiten
lesen systemweit und ohne Profilprüfung — `getAdminItem` (`api/openapi.yaml:2873`),
`searchAdminItems` (`:3941`), `getAdminItemImage` (`:3045`, ausdrücklich «systemweit und ohne
Profilprüfung», `docs/planning/04-entscheidungen.md:66`). Ein Administrator mit `max_rating = 12`
hätte also im Verwaltungsbereich ohnehin alles vor Augen, während die Startseite ihm Titel
vorenthielte. Das wäre ein falsches Versprechen, keine Schutzwirkung. Die Ernennung hebt die
Kontogrenzen darum auf 18 und alle Bibliotheken.

**Die Profile eines Administratorkontos bleiben unberührt.** Ein Kinderprofil mit FSK 6 im Konto
eines Administrators bleibt FSK 6 — es sieht weder mehr Titel noch kommt es an die Verwaltung, weil
`admin` das Hauptprofil verlangt. Die Kontogrenze deckelt, das Profil begrenzt sich selbst.

### 4.5 Wie das Hauptprofil weiter trägt

`profiles.is_default` (`server/migrations/00002_accounts_sessions.sql:35`), im Vertrag `isPrimary`,
bleibt unverändert, samt Teilindex (`00007:38`) und Löschschutz (`model.ErrPrimaryProfile`,
`409 primary_profile`). Es trägt nach dem Umbau **zwei** Lasten statt einer:

1. Wie bisher: Nur mit aktivem Hauptprofil verwaltet eine Sitzung die Profile ihres Kontos und die
   Kanäle ihres Kontos (`403 primary_profile_required`).
2. Neu betont: Nur mit aktivem Hauptprofil erreicht ein **Administrator** die Stufe `admin`. Das
   steht so schon in ADR 0005 (`docs/adr/0005-profilwahl-sitzung.md:33`) und in der Middleware
   (`server/internal/api/session.go:117`), war aber bisher nur für ein einziges Konto relevant.

### 4.6 Migrationsplan 00022 (Schritte, kein fertiges SQL)

Nach `CLAUDE.md` Regel 5 und der Vorlage `server/migrations/TEMPLATE.sql.txt`: nur vorwärts, keine
Down-Migration, `-- +goose NO TRANSACTION`, `PRAGMA foreign_keys=OFF`, Änderungen zwischen `BEGIN`
und `COMMIT`, am Ende `PRAGMA foreign_key_check` und `PRAGMA foreign_keys=ON`. Der Server sichert
vor dem Lauf mit `VACUUM INTO` (letzte fünf bleiben).

1. **View ablegen.** `DROP VIEW visible_items`. Sie liest `accounts` (`00007:104`) und muss nach dem
   Umbau neu entstehen — genau wie es Migration 00007 schon getan hat (`00007:21`).
2. **Neue Tabelle anlegen.** `accounts_new` mit allen heutigen Spalten (`id`, `email`,
   `password_hash`, `role`, `created_at`, `updated_at` aus 00002; `max_rating`, `all_libraries`,
   `invited_by`, `last_seen_at` aus 00007, `:23` bis `:26`; dazu `max_bitrate_kbps`, `max_height`,
   `max_streams`, die `UpdateAccountLimits` schreibt, `server/queries/app/access.sql:103`), mit der neuen
   `CHECK (role IN ('admin', 'user'))`, dem neuen `DEFAULT 'user'`, der neuen Spalte `is_owner` und
   der `CHECK`-Bedingung aus 4.4. **Vor dem Schreiben der Migration ist die Spaltenliste gegen
   `PRAGMA table_info(accounts)` auf dem eingefrorenen M11-Stand abzugleichen** — dieses Konzept
   zählt die Spalten aus den Migrationen 00002 und 00007 und aus den Abfragen auf; eine seither
   hinzugekommene Spalte würde beim Kopieren sonst stumm verschwinden.
3. **Zeilen kopieren und umbenennen.** `INSERT INTO accounts_new SELECT ..., CASE role WHEN 'owner'
   THEN 'admin' ELSE 'user' END, CASE role WHEN 'owner' THEN 1 ELSE 0 END, ... FROM accounts`. Die
   `id` wird übernommen, damit alle Kindtabellen weiter stimmen.
4. **Tauschen.** `DROP TABLE accounts`, `ALTER TABLE accounts_new RENAME TO accounts`, Index
   `accounts_owner` als Teilindex auf `is_owner = 1` anlegen. Der eindeutige Index auf `email`
   (`COLLATE NOCASE UNIQUE`, 00002:18) entsteht mit der neuen Tabelle.
5. **View wiederherstellen.** `visible_items` **wortgleich** wie in 00007 (`:101` bis `:115`). Die
   View ändert sich inhaltlich nicht — sie las nie `role`.
6. **Gegenprobe im selben Lauf.** Eine `SELECT`-Prüfung, dass genau eine Zeile `is_owner = 1` trägt
   und dass jede Zeile mit `is_owner = 1` auch `role = 'admin'` hat. Schlägt sie fehl, bricht die
   Migration ab, bevor `COMMIT` läuft.
7. **Kein Datenverlust an Kindtabellen.** `profiles`, `sessions`, `account_libraries`, `invitations`
   (`created_by`, `used_by`), `account_channels`, `telegram_links`, `push_subscriptions`,
   `deliveries`, `announcements` (`created_by`) und der Selbstbezug `accounts.invited_by` zeigen über
   `id` auf `accounts`. Weil die `id` erhalten bleibt und die Fremdschlüssel während des Laufs
   ausgeschaltet sind, bleibt alles stehen; der abschliessende `PRAGMA foreign_key_check` ist der
   Beleg.

**Was die Migration ausdrücklich nicht tut:** Sie widerruft keine Sitzung, löscht kein Stream-Token
und rührt keine Offline-Daten an. Die Umbenennung nimmt niemandem ein Recht (Abschnitt 8).

---

## 5. Sichtbarkeitsstufen im Vertrag

Heute kennt der Vertrag fünf Stufen (`api/openapi.yaml:9` bis `:13`), die Middleware setzt vier davon
durch und lässt `system` ausdrücklich fallen (`server/internal/api/session.go:109`, `:131`).

### 5.1 Reicht `admin` weiterhin für die Serververwaltung?

**Ja.** Die Stufe bleibt bestehen, ihre Prüfung ändert ihren Inhalt um ein Wort:

| | heute | neu |
|---|---|---|
| Middleware | `!session.IsOwner() \|\| !session.IsPrimary()` (`server/internal/api/session.go:117`) | `!session.IsAdmin() \|\| !session.IsPrimary()` |
| `model.Session` | `IsOwner() { return s.Account.Role == "owner" }` (`server/internal/model/model.go:207`) | `IsAdmin() { return s.Account.Role == "admin" }`, dazu neu `IsOwner() { return s.Account.IsOwner }` |

Die **Hauptprofil-Bedingung bleibt**, und das ist die wichtigste Nicht-Änderung in diesem Konzept.
ADR 0005 hat sie eingeführt, weil alle Profile eines Haushalts dasselbe Konto-Passwort und dasselbe
Cookie teilen (`docs/adr/0005-profilwahl-sitzung.md:16`). Mit mehreren Administratoren gilt das für
mehr Haushalte, nicht für weniger.

Die 87 Operationen mit `x-visibility: admin` behalten ihre Stufe. Keine einzige wandert nach
`account` (Abschnitt 12).

### 5.2 Braucht die Profilverwaltung eine eigene Stufe?

**Nein. Sie bleibt `account` mit der Hauptprofil-Regel.** Begründung:

- `account` ist genau dafür erfunden worden. ADR 0005 nennt in der Entscheidtabelle wörtlich
  «`listMyProfiles`, `selectProfile` und die Profilverwaltung des eigenen Kontos»
  (`docs/adr/0005-profilwahl-sitzung.md:31`).
- Die Bedingung «Hauptprofil» ist **keine** Sichtbarkeitsfrage, sondern eine fachliche Prüfung auf
  der Zeile, die bearbeitet wird. Sie gehört deshalb in den Dienst und nicht in die Middleware; dort
  steht sie heute (`server/internal/api/api.go`, Zweig `model.ErrPrimaryProfileRequired` →
  `403 primary_profile_required`).
- Eine eigene Stufe würde nichts gewinnen: Sie müsste trotzdem noch prüfen, ob das Zielprofil zum
  Konto der Sitzung gehört — und das kann die Middleware nicht, sie kennt den Rumpf der Anfrage
  nicht.

### 5.3 Die neue Stufe `owner`

Vier Operationen dürfen nur dem Besitzer offenstehen: ernennen, entziehen, ein Administratorkonto
löschen und (falls Frage F-3 mit Ja beantwortet wird) die Besitzrolle übergeben. Zwei Wege:

| | **A: neue Stufe `owner`** | **B: Stufe `admin` und `is_owner` im Handler prüfen** |
|---|---|---|
| Durchsetzung | in der Middleware, an **einer** Stelle, fail-closed (`validVisibility`, `server/internal/api/session.go:131`) | in jedem betroffenen Handler, von Hand |
| Was ein Vergessen kostet | nichts: eine Operation ohne gültiges `x-visibility` bekommt `401` und eine `WARN`-Zeile (`server/internal/api/session.go:109`) | **stille Rechteausweitung**: jeder Administrator könnte sich selbst ernennen |
| Vertragstest | `TestContractCoversEveryOperation` (`server/internal/api/contract_access_test.go:336`) erzwingt für jede `owner`-Operation einen Eintrag in einer neuen `ownerRegistry`, und die Testschleife fährt sie gegen jeden Akteur | kein Zwang; der Test sieht nur `admin` und ist zufrieden, sobald ein Administrator durchkommt |
| Kosten | ein Eintrag in `validVisibility`, ein `case` in der Middleware, eine Registry im Test, ein Wert in der Beschreibung `api/openapi.yaml:9` bis `:13`, ein Satz im Schema-Test | null |

**Empfehlung: A.** Der Ausschlag ist nicht der Aufwand, sondern der Fehlerfall: Weg B macht aus einem
vergessenen `if` eine Rechteausweitung, Weg A aus demselben Versehen einen roten Test. Genau diesem
Muster folgt schon ADR 0005, als `account` eine eigene Stufe wurde statt einer Prüfung im Handler.
Damit kennt der Vertrag **sechs** Stufen: `public`, `account`, `scoped`, `system`, `admin`, `owner`.

Die Regel aus `CLAUDE.md` Regel 7 gilt unverändert weiter: **jede** Operation trägt `x-visibility`
und braucht einen Eintrag im Vertragstest `TestContractCoversEveryOperation`
(`server/internal/api/contract_access_test.go:336`). Der Test ordnet heute vier Registries zu
(`profileRegistry`, `accountRegistry`, `adminRegistry`, `publicRegistry`,
`contract_access_test.go:355` bis `:366`) und meldet jede Operation, deren Stufe keine Registry kennt
(`:367`, der `default`-Zweig). Es kommt genau eine `ownerRegistry` dazu.

### 5.4 Fehlercodes

| Code | Status | Wann | Zustand |
|---|---|---|---|
| `profile_required` | 403 | `scoped` ohne gewähltes Profil (`server/internal/api/session.go:121`) | **unverändert** |
| `primary_profile_required` | 403 | Profilverwaltung oder Kanaländerung ohne aktives Hauptprofil (`server/internal/api/api.go`) | **unverändert** |
| `forbidden` | 403 | `admin` ohne Administratorkonto oder ohne Hauptprofil (`server/internal/api/session.go:118`) | Text ändert sich von «Only the main account …» zu «Only an administrator with the primary profile …» |
| `owner_required` | 403 | **neu**: `owner`-Operation von einem Administrator, der nicht der Besitzer ist | **neu** |
| `last_admin` | 409 | **neu**: der letzte Administrator soll degradiert oder gelöscht werden | **neu**, Abschnitt 9 |
| `owner_protected` | 409 | **neu**: der Besitzer soll degradiert oder gelöscht werden | **neu**, Abschnitt 9 |

`owner_required` ist absichtlich ein eigener Code und nicht `forbidden`: Die Oberfläche soll «Nur der
Besitzer darf das» sagen können, ohne den Grund zu raten. Die beiden 409er sind Konflikte, keine
Rechtefragen — der Aufrufer hat das Recht, aber der Zustand verbietet den Schritt.

---

## 6. Auswirkung auf den Vertrag

### 6.1 Schemata

| Schema | Stelle | Änderung | Brechend? |
|---|---|---|---|
| `Account` | `api/openapi.yaml:6026` | `role` von `enum: [owner, guest]` (`:6038`) auf `enum: [admin, user]`; neues Feld `isOwner: boolean` (required) | **ja** |
| `AdminAccount` | `api/openapi.yaml:9827` | `role` von `enum: [owner, guest]` (`:9840`) auf `enum: [admin, user]`; neues Feld `isOwner`; die Felder `limits` aus `Guest` ziehen hierher, damit es nur eine Kontoliste gibt | **ja** |
| `Guest` | `api/openapi.yaml:6414` | entfällt; sein Inhalt geht in `AdminAccount` auf | **ja** |
| `Profile` | `api/openapi.yaml:6052` | **keine Änderung.** `isPrimary` trägt weiter | nein |
| `Invitation`, `InvitationCreated`, `InvitationPreview`, `AcceptInvitationRequest` | `api/openapi.yaml:6317`, `:6351`, `:6365`, `:6391` | nur Beschreibungen: «guest» → «Konto»/«account» | nein |
| `SessionInfo` | `api/openapi.yaml:6011` | keine Strukturänderung; `account` trägt `role` und neu `isOwner`. Die Beschreibung von `updateAvailable` (`:6022`) wechselt von «only for the main account» auf «only for administrators» | nein |
| `AccountUpdate` | `api/openapi.yaml:9865` | keine Änderung | nein |
| **neu** `AccountRoleUpdate` | — | `{ role: admin \| user }` für `setAccountRole` | neu |

### 6.2 Operationen

| Operation | Pfad | heute | neu | Art der Änderung |
|---|---|---|---|---|
| `listAccounts` | `GET /admin/accounts` (`:4901`) | `admin` | `admin` | **erweitert**: wird die **einzige** Kontoliste, trägt `limits` und `isOwner` |
| `listGuests` | `GET /admin/guests` (`:2635`) | `admin` | — | **entfällt**, geht in `listAccounts` auf |
| `updateGuest` | `PATCH /admin/guests/{id}` (`:2655`) | `admin` | `admin` | **umbenannt** zu `updateAccountAccess`, Pfad `PATCH /admin/accounts/{id}` |
| `revokeGuest` | `DELETE /admin/guests/{id}` (`:2692`) | `admin` | `admin` | **umbenannt** zu `deleteAccount`, Pfad `DELETE /admin/accounts/{id}`; neue Konflikte `last_admin`, `owner_protected` |
| `setAccountRole` | `PUT /admin/accounts/{id}/role` | — | **`owner`** | **neu**: ernennen und entziehen |
| `transferOwnership` | `POST /admin/accounts/{id}/ownership` | — | **`owner`** | **neu, nur falls F-3 mit Ja beantwortet wird** |
| `listInvitations` | `GET /admin/invitations` (`:2564`) | `admin` | `admin` | nur Texte |
| `createInvitation` | `POST /admin/invitations` (`:2582`) | `admin` | `admin` | nur Texte; die Stufe bleibt `admin`, weil ein Hauptbenutzer nicht einladen darf (Entscheid 1.3) |
| `revokeInvitation` | `DELETE /admin/invitations/{id}` (`:2613`) | `admin` | `admin` | nur Texte |
| `getInvitation`, `acceptInvitation` | `:878`, `:918` | `public` | `public` | nur Texte; erzeugt weiter ein Konto mit `role = 'user'` |
| die übrigen 80 `admin`-Operationen | — | `admin` | `admin` | **keine Änderung** |
| die 24 `account`-Operationen | — | `account` | `account` | **keine Änderung** |
| die 39 `scoped`-Operationen | — | `scoped` | `scoped` | **keine Änderung** |

### 6.3 Ist das eine brechende Anhebung?

**Ja.** Das Enum in `api/openapi.yaml:6038` (und dasselbe in `:9840`) verliert die Werte `owner` und
`guest`. Für Fremd-Clients (C-5) ist das brechend, genau wie es die Anhebung auf 1.1.0 für
`HomeRowKey` war (`docs/dev/CURRENT_STATUS.md:13`). Dazu kommt der Wegfall von `GET /admin/guests`
und der Umbenennungen.

**Entfernte Enum-Werte werden nie wiederverwendet.** `owner` und `guest` sind nach dieser Anhebung
verbrannt; insbesondere darf ein künftiger dritter Rollenwert nicht `owner` heissen — der Besitzer
steht in `is_owner`, nicht in `role` (4.2).

**Welche Fassung daraus folgt**, ist Frage F-4 in Abschnitt 11. Der Präzedenzfall spricht für
**1.2.0**: M11 hat zwei `HomeRowKey`-Werte entfernt, es ausdrücklich als brechend in die
Release-Notes geschrieben und trotzdem die Nebenstelle angehoben
(`docs/dev/CURRENT_STATUS.md:9` bis `:14`). Weil Slouchcat ab jetzt öffentlich angeboten wird, ist
**2.0.0** die ehrlichere Zahl, und das ist eine Produktentscheidung, keine technische.

---

## 7. Oberfläche

Grundsatz zuerst, weil er alles andere bestimmt: **Der Client prüft kein Recht** (`CLAUDE.md`
Regel 12, Plan-Prinzip 1). Die Oberfläche blendet Bedienelemente aus, die Durchsetzung bleibt im
Server — exakt so, wie ADR 0009 es für die Kanäle formuliert
(`docs/adr/0009-administration-benachrichtigungen.md:57`). Jede Seite unten zeigt oder verbirgt
anhand von `session.account.role` und `session.account.isOwner`, und jede Antwort des Servers gilt
auch dann, wenn jemand die Seite von Hand aufruft.

**Die Web-Hälfte ist der grössere Teil dieser Arbeit, nicht der kleinere.** Gezählt am Stand
`0dd4f19`: `grep -rln "role === 'owner'" web/src` nennt **20 Quelldateien**, dazu kommt
`AdminLayout.tsx` mit `role !== 'owner'` — zusammen **21**. `grep -rn "'owner'\|'guest'" web/src
--include=*.ts --include=*.tsx` zählt **36 Treffer**, davon 13 in Testdateien. In `web/e2e` steht
keiner. **Sechzehn** dieser Dateien tragen wörtlich dieselbe Zeile:

```ts
const allowed = session.account.role === 'owner' && session.profile.isPrimary;
```

Das ist eine mechanische, aber nicht automatische Umstellung: Je Stelle ist zu entscheiden, ob sie
«Administrator» (`role === 'admin'`) oder «Besitzer» (`isOwner`) meint. Alle einundzwanzig meinen
«Administrator». Nur in `AccountsPage.tsx` kommt «Besitzer» neu dazu.

| Seite / Datei | Stelle | Was geschieht |
|---|---|---|
| **Verwaltung → Konten**, `web/src/features/admin/AccountsPage.tsx` | `:143` | **erweitert.** Heute zeigt sie die eigene E-Mail und das eigene Passwort. Neu: die vollständige Kontoliste mit Rolle, Obergrenze, Bibliotheken, Profilen; Zeilenaktion «Zum Administrator ernennen» bzw. «Rolle entziehen», die **nur beim Besitzer** erscheint; Löschen mit den neuen Konflikten |
| **Verwaltung → Zugang**, `web/src/features/admin/AccessPage.tsx` | `:15` | `role === 'owner'` → `role === 'admin'`. `GuestsSection.tsx` geht in der Kontoliste auf, `InvitationsSection.tsx` bleibt und verliert nur das Wort «Gast» |
| **Admin-Rahmen**, `web/src/features/admin/AdminLayout.tsx` | `:21` | `session.account.role !== 'owner'` → `!== 'admin'`; davon hängt ab, ob die Admin-Navigation überhaupt erscheint |
| **App-Rahmen**, `web/src/app/AppLayout.tsx` | `:62` | `const isAdmin = isPrimary && session.account.role === 'owner'` → `=== 'admin'`; davon hängt der Verwaltungseinstieg in der Hauptnavigation ab |
| **Startseite**, `web/src/features/catalog/HomePage.tsx` | `:311` | Verwaltungs-Abkürzung auf der Zuschauerseite |
| **Detailseite**, `web/src/features/catalog/ItemPage.tsx` | `:167` | `canManage` — der Weg zur Titelpflege |
| **Benachrichtigungen**, `web/src/features/notifications/ChannelSettingsPage.tsx` | `:480` | `const isOwner = ...` wird zu `isAdmin`; das Ereignis `errors` steht nur Administratoren offen (`api/openapi.yaml:5197`, `event_not_allowed`) |
| **Vierzehn weitere Admin-Seiten** | `AdminHomePage.tsx:336`, `AdminItemPage.tsx:423`, `AnnouncementsPage.tsx:139`, `BackupPage.tsx:348`, `CollectionsPage.tsx:278`, `DevicesPage.tsx:26`, `HomeRowsPage.tsx:37`, `IntegrationsPage.tsx:642`, `MaintenancePage.tsx:496`, `PicksPage.tsx:321`, `PodcastsPage.tsx:120`, `SpotlightPage.tsx:431`, `StatsPage.tsx:237`, `StreamsPage.tsx:49` | dieselbe Zeile, dieselbe Änderung |
| **Testdateien** | 13 Treffer, darunter `web/src/test/api.tsx:41` (die gemeinsame Sitzungsattrappe) | die Attrappe zuerst, sonst fallen alle anderen Tests gleichzeitig um |
| **Verwaltung → Navigation**, `web/src/features/admin/sections.ts` | `:49`, `:50` | **keine Änderung nötig.** Der Eintrag `access` behält seinen Platz in der Gruppe `accounts`; der Eintrag `profiles` zeigt bereits auf `paths.profileSettings` **ausserhalb** von `/admin` — genau richtig, denn die Profilverwaltung gehört dem Konto, nicht der Administration (`sections.ts:4` bis `:6`) |
| **Profile (Hauptbenutzer)**, `web/src/features/profiles/ProfileSettingsPage.tsx` | `:28` | **keine funktionale Änderung.** Sie prüft nur `session.profile.isPrimary` und hängt an den `account`-Operationen. Zu prüfen ist allein, ob sie für einen Hauptbenutzer ohne Verwaltungszugang erreichbar bleibt |
| **Einladung annehmen**, `web/src/features/invite/InvitePage.tsx` | — | nur Texte: aus «Gastkonto» wird «Konto» |
| **Geräteliste**, `web/src/features/admin/DevicesPage.tsx` | `:55` | `key: 'owner'` ist die **Spalte «Eigentümer»** eines Geräts (`AdminDevice.ownerEmail`, `api/openapi.yaml:7374`) und hat mit Rollen nichts zu tun — **nicht anfassen** |

**Regel 8 — Playwright-Tastaturtest je neuer Seite.** Es entsteht keine vollständig neue Seite;
`AccountsPage` wächst aber um eine Tabelle mit Zeilenaktionen und zwei Dialogen, und das ist in der
Wirkung eine neue Seite. Sie bekommt darum einen eigenen Tastaturtest: Pfeiltasten durch die Tabelle,
Tab in die Zeilenaktionen, Enter öffnet den Rollendialog, Escape schliesst ihn, Zurück landet auf der
Liste, der Fokus bleibt sichtbar, und es gibt keine Fokusfalle. Verweigert der Server eine Aktion
(`owner_required`), muss die Meldung für Screenreader erreichbar sein.

**Regel 9 — keine Farbe und kein Text hart im Code.** Alle neuen Texte gehen nach
`web/src/locales/de.json` und `web/src/locales/en.json`, in beiden Dateien symmetrisch; `pnpm -C web
test locales` prüft die Symmetrie und die Schweizer Schreibweise. Farben kommen aus Tokens; die
Rollen-Abzeichen nutzen das bestehende `Chip`-Bauteil aus `web/src/ui`, das `AccountsPage.tsx:14`
bereits importiert. Administration und Zuschauer teilen sich dieselben Bauteile.

**Die drei Begriffe brauchen in beiden Sprachen einen festen Namen** (Frage F-5): Vorschlag
Deutsch «Administrator», «Hauptbenutzer», «Profil»; Englisch «administrator», «main user»,
«profile». Die API-Werte bleiben davon unberührt und heissen `admin` und `user`.

---

## 8. Migration und Rückwärtsverträglichkeit

### 8.1 Was mit dem Bestand geschieht

| Gegenstand | Was geschieht | Beleg / Begründung |
|---|---|---|
| **Das Besitzerkonto** | `role = 'owner'` → `role = 'admin'`, `is_owner = 1` | Migrationsschritt 3 (4.6) |
| **Bestehende Gastkonten** | `role = 'guest'` → `role = 'user'`, `is_owner = 0`. `max_rating`, `all_libraries` und ihre Zeilen in `account_libraries` bleiben Zeichen für Zeichen | Entscheid 1.3; `00007:28`, `:33` |
| **Profile** | unberührt, samt `is_default`, `pin_hash`, Bibliotheksrechten und Schauzeit-Grenzen | `profiles` wird nicht angefasst |
| **Offene Einladungen** | bleiben gültig, mit unveränderter Frist und unveränderter Obergrenze; sie erzeugen nach wie vor ein Nicht-Admin-Konto | `invitations` wird nicht angefasst (4.3) |
| **Laufende Sitzungen** | bleiben gültig und werden **nicht** widerrufen | die Umbenennung nimmt keiner Sitzung ein Recht; der Besitzer behält genau seine Rechte, jedes Gastkonto genau seine. Ein Widerruf würde alle Haushalte grundlos aussperren |
| **Stream-Tokens** | bleiben gültig | sie hängen an `playback_sessions`, nicht an `accounts.role` (Plan 5.4, `03-plan-v1.md` Abschnitt 5.4) |
| **Offline-Downloads** | bleiben abspielbar; `allowed_profile_ids` ändert sich nicht | die Liste enthält Profile desselben Kontos, die das Item sehen dürfen (Plan 3.3); weder Profil noch Sichtbarkeit ändern sich |
| **Benachrichtigungskanäle** | bleiben eingeschaltet wie sie sind | `account_channels` hängt an `account_id`; die Rückfüllung aus 00016 (`:102` bis `:110`) ist längst gelaufen und wird nicht wiederholt |

**`SessionInfo` ändert sich für jeden angemeldeten Browser**, weil `account.role` einen neuen Wert
liefert. Ein Client, der auf `'owner'` prüft, sähe sich plötzlich als Nicht-Administrator. Das
betrifft die eigene Web-App (sie wird im selben Commit nachgezogen) und jeden Fremd-Client (C-5) —
und ist genau der Grund, weshalb die Vertragsfassung angehoben wird (6.3).

### 8.2 Wo ein Rechteentzug greifen muss

Zum Vergleich die bestehende Entzugskette aus ADR 0005
(`docs/adr/0005-profilwahl-sitzung.md:37`): zuerst **alle Sitzungen widerrufen**, dann **alle Streams
beenden** (auch über die Registry im Speicher), dann **löschen**, dann **noch einmal beenden**, was
zwischendurch geöffnet wurde. Im Code ist das `auth.Service.RevokeAccount`
(`server/internal/auth/guests.go:142`): `RevokeAccountSessions`, `end(...)`, `DeleteAccount`,
`end(...)` — der zweite `end`-Aufruf (`guests.go:158`) ist kein Versehen, sondern genau dieser
Nachschlag.

| Vorgang | Was greifen muss |
|---|---|
| **Konto löschen** (`deleteAccount`, heute `revokeGuest`) | unverändert die ganze Kette oben. Neu davor die zwei Konfliktprüfungen `owner_protected` und `last_admin` (Abschnitt 9) |
| **Rolle entziehen** (`setAccountRole` auf `user`) | **alle Sitzungen dieses Kontos widerrufen.** Sonst behielte eine offene Verwaltungssitzung ihre Seite im Browser, und die nächste Anfrage schlüge mit `403` fehl, während die Oberfläche weiter Verwaltungselemente zeigt. Der Widerruf macht daraus einen sauberen `401` und eine Neuanmeldung. Streams müssen **nicht** beendet werden: die Sichtbarkeit des Kontos ändert sich durch die Degradierung nicht |
| **Rolle entziehen, falls dabei Obergrenzen sinken** | Senkt der Besitzer im selben Zug die Obergrenze oder die Bibliotheksliste, gilt die bestehende Regel: `RecheckVisibility` beendet offene Direktstreams, die dadurch verdeckt werden (`server/internal/playback/streams.go:98`), aufgerufen wie heute in `server/internal/api/profile_handlers.go:214`; HLS prüft bei jeder Anfrage ohnehin. Beim nächsten Offline-Sync verschwindet das Profil aus `allowed_profile_ids` der betroffenen Downloads (Plan 3.3) |
| **Ernennen** (`setAccountRole` auf `admin`) | **kein Widerruf nötig.** Niemand verliert etwas; die nächste Anfrage der offenen Sitzung bekommt die neuen Rechte, weil die Middleware je Anfrage entscheidet. Weil die Ernennung die Kontogrenzen auf 18/alle hebt (4.4), **erweitert** sich die Sichtbarkeit — das braucht keinen Eingriff, nur eine Aktualisierung von `SessionInfo` in der Oberfläche |
| **Besitzerwechsel** (falls F-3 Ja) | Sitzungen beider Konten widerrufen, damit kein Browser mit einem falschen `isOwner` weiterläuft |

### 8.3 Eingefrorene Datenbankstände

`testdata/db/` hält die Stände M1a bis M11; `TestFrozenStatesMigrate` fährt die Migrationen über alle
(`docs/dev/CURRENT_STATUS.md:410`). Migration 00022 muss über **jeden** dieser Stände laufen, und
jeder davon trägt `role IN ('owner', 'guest')` und die alte `CHECK`-Bedingung. Der Test ist damit der
eigentliche Beweis, dass der Tabellenumbau vollständig ist; `PRAGMA foreign_key_check` muss nach
jedem Stand leer sein. Ein neuer eingefrorener Stand kommt nach der Umsetzung dazu, wie nach M11
geschehen (`5bc6ce7`, «test(store): freeze the M11 database state»).

---

## 9. Sicherheit

Rechteausweitung heisst hier: Jemand erreicht die Verwaltung, der sie nicht erreichen soll. Sechs
Riegel, je mit dem Test, der sie belegt.

### 9.1 Der letzte Administrator ist nicht entfernbar

**Regel:** `setAccountRole` auf `user` und `deleteAccount` scheitern mit `409 last_admin`, wenn danach
kein Konto mit `role = 'admin'` übrig bliebe. Geprüft wird **in derselben Schreibtransaktion** wie
die Änderung, mit `SELECT count(*) FROM accounts WHERE role = 'admin' AND id <> @id` — genau so, wie
`CreateOwner` seine Einzigkeitsprüfung schon heute innerhalb der Transaktion macht, damit zwei
gleichzeitige Assistenten nicht beide gewinnen (`server/internal/auth/auth.go:159` bis `:194`).

**Testfall:** Ein Server mit genau einem Administrator (dem Besitzer) und einem Hauptbenutzer.
`deleteAccount` auf den Administrator → `409 last_admin`; danach zählt `SELECT count(*) ... role =
'admin'` immer noch 1. Zweiter Fall mit zwei Administratoren, von denen einer **nicht** der Besitzer
ist: Löschen des Nicht-Besitzers gelingt, ein zweiter Versuch auf den verbleibenden scheitert.

### 9.2 Der Besitzer ist nicht degradierbar und nicht löschbar

**Regel:** Jede Operation, die `role` ändert oder ein Konto löscht, scheitert mit
`409 owner_protected`, wenn das Zielkonto `is_owner = 1` trägt. Die Prüfung steht vor jeder anderen,
damit sie auch dann greift, wenn der Besitzer zufällig der letzte Administrator ist — dann ist
`owner_protected` die aussagekräftigere Meldung als `last_admin`.

**Testfall:** `setAccountRole` auf das Besitzerkonto, einmal vom Besitzer selbst und einmal von einem
zweiten Administrator → beide Male `409 owner_protected` (der zweite bekommt davor schon `403
owner_required`, siehe 9.3; der Test prüft beide Reihenfolgen). `deleteAccount` auf das
Besitzerkonto → `409 owner_protected`. Danach: `is_owner = 1` steht unverändert auf derselben `id`.

### 9.3 Selbst-Ernennung ist unmöglich

**Regel:** `setAccountRole` trägt `x-visibility: owner`. Die Middleware lehnt jeden Aufrufer ab, der
nicht `is_owner = 1` trägt, mit `403 owner_required` — bevor ein Handler das Zielkonto überhaupt
sieht. Zusätzlich lehnt der Dienst ab, wenn Zielkonto und Aufrufer dasselbe sind, weil der Besitzer
sich selbst weder befördern (er ist schon Administrator) noch degradieren kann (9.2).

**Testfall:** Die neue `ownerRegistry` in `server/internal/api/contract_access_test.go` fährt jede
`owner`-Operation gegen die bestehenden Akteure. Erwartung: Hauptprofil des Besitzers → Erfolg;
Hauptprofil eines **zweiten Administrators** → `403 owner_required`; Hauptprofil eines
Hauptbenutzers → `403 owner_required`; Kinderprofil des Besitzers → `403` (9.6); keine Sitzung →
`401`. Ein neuer Akteur `actorSecondAdmin` kommt dafür in den Fixture-Bausatz
(`server/internal/visibility/vistest/vistest.go`, heute `owner@example.test` in `:325` und der Gast
in `:641`).

### 9.4 Eine Einladung erzeugt nie einen Administrator

**Regel:** `invitations` trägt keine Spalte für eine Rolle (4.3), und `InsertGuestAccount`
(`server/queries/app/access.sql:87`, `VALUES`-Zeile `:89`) schreibt `'user'` als **Literal** in der
SQL-Datei, nicht als Parameter. Damit kann kein Rumpf einer Anfrage und kein Fehler in einem Handler
die Rolle beeinflussen.

**Testfall:** Ein Vertragstest auf `acceptInvitation` (`api/openapi.yaml:918`), der nach dem Anlegen
`SELECT role, is_owner FROM accounts WHERE id = @neu` prüft: `('user', 0)`. Dazu ein Schema-Test, der
in `api/openapi.yaml` sicherstellt, dass weder `AcceptInvitationRequest` noch `InvitationCreate` ein
Feld `role` oder `isOwner` tragen — er schlägt fehl, sobald jemand eins hinzufügt.

### 9.5 Niemand hebt seine eigenen Grenzen an

**Regel:** `accounts.max_rating`, `accounts.all_libraries` und `account_libraries` ändert nur
`updateAccountAccess` (`admin`). Es gibt keine `account`-Operation, die sie schreibt;
`updateAccount` (`api/openapi.yaml:439`) ändert ausschliesslich E-Mail und Passwort und verlangt
dafür das aktuelle Passwort (`AccountUpdate.currentPassword`, `:9873`).

**Testfall:** `TestContractAccountOperations`
(`server/internal/api/contract_access_test.go:405`) fährt heute schon jede `account`-Operation gegen
`actorPrimary`, `actorNone` und `actorGuest`. Ergänzt wird eine Prüfung, dass nach dem ganzen Lauf
weder `max_rating` noch `all_libraries` eines Kontos einen anderen Wert haben als vorher.

### 9.6 `admin` verlangt weiterhin das aktive Hauptprofil

**Regel:** unverändert aus ADR 0005 (`docs/adr/0005-profilwahl-sitzung.md:33`), durchgesetzt in
`server/internal/api/session.go:117`. Die neue Stufe `owner` erbt dieselbe Bedingung — ein
Kinderprofil des Besitzers kommt nirgends hin.

**Testfall:** Der bestehende Plan-Punkt 10.2 «Admin-Operationen: Gast- und Profilsitzungen erhalten
403» (`docs/planning/03-plan-v1.md`, Abschnitt 10.2) wird um zwei Akteure erweitert: das
Kinderprofil eines **zweiten Administrators** (→ 403 bei jeder `admin`- und jeder `owner`-Operation)
und das Hauptprofil eines Hauptbenutzers (→ 403 bei jeder `admin`-Operation). Zusätzlich der
Gegenprobe-Fall: das Hauptprofil des zweiten Administrators erreicht jede `admin`-Operation, damit
ein zu scharfer Riegel ebenfalls auffällt.

---

## 10. Schnitt in Schritte

Jeder Schritt ist ein eigener Commit, und vor jedem Abschluss ist `task ci` grün. Die Reihenfolge
folgt der Baubarkeit: erst die Papiere, dann das Schema, dann der Vertrag, dann die Durchsetzung,
dann die Oberfläche.

### Schritt 0 — Entscheide und Papiere
`docs(planning,adr): record the user management decisions and ADR 0014`

**Ziel:** Marcos vier Antworten und die Umformulierung des Nicht-Ziels stehen dort, wo sie
verbindlich sind — im Entscheidungsprotokoll, nicht in einem Konzeptentwurf.

**Berührte Dateien:** `docs/planning/04-entscheidungen.md` (neue **Frage 12** nach dem Muster von
Frage 11, `:20`, dazu eine Zeile in «Weitere Entscheide während der Umsetzung»),
`docs/adr/0014-benutzerverwaltung.md` (neu), `docs/planning/06-benutzerverwaltung.md` (dieses
Papier), ein Ablösungshinweis in `docs/adr/0005-profilwahl-sitzung.md` zur neuen Stufe `owner`.

**DoD:** `git diff --stat docs/spec/` ist leer (die Spec wird nicht umgeschrieben, 3.4). Frage 12
trägt alle vier Antworten **und** den Textvorschlag aus 3.3, je mit Datum. ADR 0014 nennt K-1, K-2,
K-6, K-7, K-8, C-5 und A-2. Die Antworten auf die Fragen F-1 bis F-6 aus Abschnitt 11 stehen
ebenfalls darin — **ohne sie beginnt Schritt 1 nicht.** `git show --stat HEAD` nennt genau diese
Doku-Dateien. `task ci` grün.

### Schritt 1 — Schema und Datenmodell
`feat(store): replace the owner/guest roles with admin/user and mark the owner`

**Ziel:** Die Datenbank kennt die neuen Rollen und den Besitzer; nichts im Vertrag ändert sich noch.

**Berührte Dateien:** `server/migrations/00022_account_roles.sql` (neu, nach `TEMPLATE.sql.txt` und
dem Plan aus 4.6), `server/queries/app/auth.sql`, `server/queries/app/access.sql`,
`server/queries/app/accounts.sql` (`:8` sortiert nach `role = 'guest'`),
`server/internal/model/model.go` (`:207`, `IsOwner` → `IsAdmin`, neues `IsOwner` auf `Account`),
`server/internal/auth/auth.go` (`:44` `roleOwner`, `:196`), `server/internal/auth/guests.go`,
danach `task gen` (sqlc).

**DoD:** `task ci` grün — das deckt den generierten Code mit ab, weil `ci` `gen` und `gen:check`
ausführt und bei Drift abbricht. Migration über **alle** eingefrorenen Stände:
`go test ./internal/store -run TestFrozenStatesMigrate` grün, `PRAGMA foreign_key_check` nach jedem
Stand leer. Neuer Test: genau eine Zeile mit `is_owner = 1`, und jede Zeile mit `is_owner = 1` trägt
`role = 'admin'`. Neuer Test: `INSERT INTO accounts` ohne `role` ergibt `'user'`. Neuer Test: ein
`UPDATE accounts SET role = 'admin'` auf ein Konto mit `max_rating = 12` scheitert an der
`CHECK`-Bedingung aus 4.4. `git grep -n "'owner'\|'guest'" -- server/queries` ist leer.

### Schritt 2 — Vertrag und die Stufe `owner`
`feat(api): three roles in the contract and the owner visibility level`

**Ziel:** Der Vertrag beschreibt die neue Ordnung; der Schema-Test und der Vertragstest erzwingen sie.

**Berührte Dateien:** `api/openapi.yaml` (alles aus 6.1 und 6.2, dazu die Beschreibung der Stufen in
`:8` bis `:13` und die Fassung in `:4`), `server/internal/api/session.go` (`:117` neue Bedingung,
`:131` `validVisibility` um `owner`), `server/internal/api/api.go` (neue Fehlercodes aus 5.4),
`server/internal/api/contract_access_test.go` (neue `ownerRegistry`, neuer Zweig in
`TestContractCoversEveryOperation` `:336`, neuer Akteur `actorSecondAdmin`),
`server/internal/visibility/vistest/vistest.go` (zweiter Administrator im Bausatz), danach `task gen`
für Go **und** Web (`packages/api-client/src/generated/`).

**DoD:** `task ci` grün. `go test ./internal/api -run 'TestContractCoversEveryOperation|TestEveryRouteHasVisibility|TestContractRejectsViolations'`
grün. `grep -n '^  version:' api/openapi.yaml` zeigt die in F-4 entschiedene Fassung.
`git grep -n 'owner\|guest' -- api/openapi.yaml` liefert nur noch Treffer, die den **Besitzer**
meinen (`isOwner`, `owner_required`, `owner_protected`, `ownerEmail`/`ownerProfile` in `AdminDevice`,
`api/openapi.yaml:7374`) — jeder verbleibende Treffer ist im Commit-Body einzeln begründet.
**`api/openapi.yaml` ist die einzige Quelle**: `server/internal/api/gen/` und
`packages/api-client/src/generated/` werden nicht von Hand angefasst (`CLAUDE.md` Regel 1).

### Schritt 3 — Rollen ernennen und entziehen
`feat(auth): let the owner appoint and demote administrators`

**Ziel:** Die neuen Operationen funktionieren, und alle sechs Riegel aus Abschnitt 9 halten.

**Berührte Dateien:** `server/internal/auth/` (`setAccountRole`, Konfliktprüfungen in der
Schreibtransaktion, Sitzungswiderruf beim Entzug), `server/internal/api/guest_handlers.go` →
umbenannt passend zu `updateAccountAccess`/`deleteAccount`, `server/internal/api/admin_handlers.go`,
`server/queries/app/access.sql`.

**DoD:** `task ci` grün. Je ein Test für 9.1 bis 9.6, benannt nach dem Riegel. Beim Entzug ist
nachgewiesen, dass die Sitzungen des Zielkontos `revoked_at` tragen und die nächste Anfrage `401`
bekommt. Beim Ernennen ist nachgewiesen, dass **kein** Stream beendet wurde und dass `max_rating` auf
18 und `all_libraries` auf 1 stehen. `go test ./internal/api -run TestContractOwnerOperations` grün.

### Schritt 4 — Die Abfragen, die «Besitzer» sagen und «Administration» meinen
`fix(notify,admin): address administrators by role and the owner by is_owner`

**Ziel:** Keine Abfrage meint mehr «der Besitzer», wenn sie «die Administration» meint — und
umgekehrt. Dieser Schritt ist klein im Umfang und gross in der Wirkung; er steht darum allein.

**Berührte Dateien:** `server/queries/system/notify_channels.sql` (`:47` `NotifyOwner`,
`:95` `ChannelTargetsOwners`, dazu die drei `CASE WHEN a.role = 'guest'`-Zeilen in `:12`, `:13`,
`:25`, `:26`), `server/queries/system/admin.sql` (`:356` `AdminOwnerProfileIDs`),
`server/internal/notify/`, `server/internal/admin/`. Je Stelle ist zu entscheiden und im Commit-Body
zu begründen, ob sie «alle Administratoren» (`role = 'admin'`) oder «der Besitzer» (`is_owner = 1`)
meint; die Empfehlungen stehen in Abschnitt 12.

**DoD:** `task ci` grün. `git grep -n "role = 'owner'\|role = 'guest'" -- server` ist leer. Je ein
Test: ein Fehlerereignis erreicht **beide** Administratoren; ein Pick erreicht genau die Profile des
Besitzerkontos und keines der Profile des zweiten Administrators; eine Testmail ohne Empfänger geht
an den Besitzer.

### Schritt 5 — Oberfläche
`feat(web): manage accounts and roles in the administration`

**Ziel:** Die Verwaltung zeigt alle Konten mit ihren Rollen; der Besitzer sieht die Rollenaktionen,
alle anderen nicht.

**Berührte Dateien:** die **21 Quelldateien** aus Abschnitt 7 (davon 16 mit derselben Zeile), die
**13 Testtreffer** — zuerst die gemeinsame Sitzungsattrappe `web/src/test/api.tsx:41`, sonst fallen
alle anderen Tests gleichzeitig um —, dazu `web/src/features/admin/GuestsSection.tsx` (geht in der
Kontoliste auf), `web/src/features/admin/InvitationsSection.tsx`,
`web/src/features/invite/InvitePage.tsx`, `web/src/locales/de.json`, `web/src/locales/en.json` und
ein neuer Playwright-Tastaturtest unter `web/e2e/`. **Nicht** anzufassen sind
`web/src/features/admin/sections.ts` und `web/src/features/admin/DevicesPage.tsx:55` (Abschnitt 7).

**Der Commit-Body trennt Mechanik von Inhalt.** Bei rund 35 berührten Dateien kann ein Reviewer
sonst nicht erkennen, was die sechzehnfache Wiederholung derselben Zeile ist und was Entscheidung
(die neue Kontotabelle, die Rollenaktionen, die Besitzerprüfung).

**DoD:** `task ci` grün (enthält `web:lint`, `web:test`, `web:e2e`). Der Tastaturtest nach Regel 8
deckt Pfeiltasten, Tab, Enter, Escape, Zurück, sichtbaren Fokus und das Fehlen einer Fokusfalle ab.
`pnpm -C web test locales` grün (Symmetrie beider Sprachdateien und Schweizer Schreibweise).
Keine harte Farbe, kein harter Text: `pnpm -C web lint:css` und `eslint-plugin-i18next` grün.
Nachweis, dass die Oberfläche nichts durchsetzt: ein Test ruft die Rollenoperation als Nicht-Besitzer
direkt auf und erwartet `403 owner_required`.

### Schritt 6 — Abnahme und Doku
`docs(dev): record the new user management and freeze the database state`

**Ziel:** Der Stand ist belegt, der eingefrorene DB-Stand ist da, die Release-Notes nennen den Bruch.

**Berührte Dateien:** `docs/dev/CURRENT_STATUS.md`, `testdata/db/` (neuer eingefrorener Stand plus
Test, nach dem Muster von `5bc6ce7`), die Abnahmezeilen zu K-1, K-2, K-6, K-7, K-8 und C-5,
`deploy/.env.example` und die weiteren Versionsstellen, falls F-4 eine Anhebung verlangt.

**DoD:** `task ci` grün. `go test ./internal/store -run TestFrozenStatesMigrate` läuft auch über den
neuen Stand. `docs/dev/CURRENT_STATUS.md` nennt die Migration 00022, die neue Vertragsfassung und
den brechenden Enum-Wechsel in Worten, die in die Release-Notes passen. Kein TODO, kein
`test.skip`, kein Stub-Test im Diff (`CLAUDE.md`, Abschnitt «Arbeitsweise»).

---

## 11. Offene Fragen an Marco

Format wie `docs/planning/04-entscheidungen.md`. Nur Fragen, die ich nicht selbst entscheiden kann;
alles andere ist in diesem Papier entschieden und begründet.

| Nr. | Frist | Thema | Empfehlung | Antwort | Datum |
|-----|-------|-------|------------|---------|-------|
| **F-1** | vor Schritt 1 | **Hebt die Ernennung die Grenzen des Kontos?** Ein Hauptbenutzer mit FSK 12 und zwei Bibliotheken wird Administrator. Steigen `accounts.max_rating` auf 18 und `all_libraries` auf 1? | **Ja.** Die Verwaltungsseiten lesen systemweit und ohne Profilprüfung (4.4); eine Kontogrenze wäre dort ein falsches Versprechen statt eines Schutzes. Die **Profile** behalten ihre eigenen Stufen — ein Kinderprofil im Administratorkonto bleibt FSK 6 | offen | — |
| **F-2** | vor Schritt 3 | **Was gilt beim Entzug der Administratorrolle?** Fallen die Grenzen wieder auf den Stand vor der Ernennung, oder bleiben sie bei 18/alle? | **Bleiben bei 18/alle.** Ein automatisches Zurücksetzen müsste den alten Stand aufbewahren (eine neue Spalte, eine neue Fehlerquelle), und ein stilles Verschwinden von Titeln wäre für den Betroffenen unerklärlich. Die Administration setzt die Grenzen danach bewusst auf der Kontoseite | offen | — |
| **F-3** | vor Schritt 2 | **Soll der Besitzer die Besitzrolle übergeben können** (`transferOwnership`)? Heute wäre der Besitzer für immer festgelegt | **Ja, aber klein:** eine Operation der Stufe `owner`, nur auf ein bestehendes Administratorkonto, mit Eingabe des eigenen Passworts, in einer Transaktion und mit Widerruf der Sitzungen beider Konten. Ohne sie gibt es bei einem Besitzerwechsel nur den Weg über die Datenbank von Hand — für ein öffentlich angebotenes Produkt zu wenig | offen | — |
| **F-4** | vor Schritt 2 | **Welche Vertragsfassung?** Das Enum verliert `owner` und `guest`, `GET /admin/guests` verschwindet; das ist für Fremd-Clients brechend (C-5) | **1.2.0**, dem Präzedenzfall M11 folgend (`docs/dev/CURRENT_STATUS.md:9` bis `:14`: brechender Enum-Wechsel bei einer Nebenstellen-Anhebung, ausdrücklich in den Release-Notes genannt). **2.0.0** wäre die ehrlichere Zahl, sobald Slouchcat öffentlich angeboten wird — das ist eine Produktentscheidung | offen | — |
| **F-5** | vor Schritt 5 | **Wie heissen die drei Ebenen in der Oberfläche**, deutsch und englisch? Die API-Werte stehen fest (`admin`, `user`) | «Administrator» / «Hauptbenutzer» / «Profil»; englisch «administrator» / «main user» / «profile». «Besitzer» erscheint nur als Abzeichen am einen Konto, nicht als eigene Ebene | offen | — |
| **F-6** | vor Schritt 0 | **Darf `docs/spec/01-anforderungen-v1.md:68` ausnahmsweise umgeschrieben werden?** `docs/spec/README.md:6` verbietet es | **Nein.** Der Text aus 3.3 geht als Frage 12 ins Entscheidungsprotokoll und in ADR 0014; die Rangfolge Entscheidungsprotokoll > Plan > Spec trägt ihn (3.4). Eine Ausnahme hier macht die Spec-Regel für alle späteren Fälle wertlos | offen | — |

---

## 12. Risiken

### 12.1 Der Umfang: 87 Operationen mit `x-visibility: admin`

Gezählt am Stand `0dd4f19`: `grep -c 'x-visibility: admin' api/openapi.yaml` → **87**. Dazu 39
`scoped`, 24 `account`, 11 `public` und 0 `system`.

| Gruppe | Zahl | Was mit ihr geschieht |
|---|---|---|
| Bibliotheken, Ordner-Browser, Podcasts | 14 | **unverändert** `admin` |
| Titelpflege, Bilder, Wartung, Duplikate, Metadatensuche | 13 | **unverändert** `admin` |
| Kuratierung: Picks, Sammlungen, Startseite, Spotlight, Trakt | 21 | **unverändert** `admin` |
| System, Einstellungen, Jobs, Sicherung, Update, Statistiken | 20 | **unverändert** `admin` |
| Ankündigungen, Integrationen | 11 | **unverändert** `admin` |
| Geräte und Streams | 4 | **unverändert** `admin` |
| **Zugang: Einladungen** (`listInvitations`, `createInvitation`, `revokeInvitation`) | 3 | Stufe bleibt `admin`; nur Beschreibungen ändern sich |
| **Zugang: Konten** (`listGuests`, `updateGuest`, `revokeGuest`, `listAccounts`) | 4 | **berührt**: eine entfällt, zwei werden umbenannt, eine wächst |
| **Neu** (`setAccountRole`, ggf. `transferOwnership`) | 1–2 | neue Stufe `owner` |

**Schätzung: 7 der 87 Operationen werden angefasst, 80 bleiben wörtlich.** Keine einzige wandert von
`admin` nach `account` — die Trennlinie zwischen Serververwaltung und Kontoverwaltung liegt schon
heute dort, wo Marco sie haben will. Die 24 `account`-Operationen sind die Ebene «Hauptbenutzer» und
brauchen keine Änderung.

### 12.2 Wo die Überraschungen lauern

**Das Risiko ist nicht die Zahl 87. Es sind drei SQL-Zeilen, die ihre Bedeutung lautlos ändern,
sobald es zwei Administratoren gibt — und einundzwanzig Web-Dateien, die stumm aufhören zu
funktionieren.** Gezählt am Stand `0dd4f19`: `grep -rn "role = 'owner'" server/queries` → **3**,
`grep -rn "'guest'" server/queries` → **9**, `grep -rn '"owner"\|"guest"' server/internal --include=*.go`
ohne `gen/` und ohne Tests → **5**, in Go-Tests → **37**, in `web/src` → **36**.

| Stelle | Was sie heute tut | Was nach dem Umbau gälte | Empfehlung |
|---|---|---|---|
| `server/queries/system/admin.sql:356` (`AdminOwnerProfileIDs`) | «Profile des Hauptkontos: die einzigen Ziele eines Picks» — die Zielmenge von Cat's Empfehlungen (E-2) | mit `role = 'admin'` erschienen Picks plötzlich auch bei den Kindern des zweiten Administrators | **`is_owner = 1`.** Das Verhalten bleibt exakt wie heute und ist mit mehreren Administratoren eindeutig |
| `server/queries/system/notify_channels.sql:95` (`ChannelTargetsOwners`) | Ereignis `errors` geht nur an Hauptkonten (ADR 0009, `:37`) | ein zweiter Administrator bekäme keine Fehlermeldungen, obwohl er den Server betreibt | **`role = 'admin'`.** Wer verwaltet, muss von Fehlern erfahren |
| `server/queries/system/notify_channels.sql:47` (`NotifyOwner`) | ein einziger Empfänger für die Testmail ohne Adresse und für die Texte der Fehlerereignisse | mit `role = 'admin'` und `LIMIT 1` wäre der Empfänger zufällig | **`is_owner = 1`.** Ein einzelner Empfänger muss ein bestimmter sein |
| `server/queries/system/notify_channels.sql:12`, `:13`, `:25`, `:26` | Standardkanäle neuer Konten: E-Mail an für Gäste, nur Ankündigungen | wörtliche Umbenennung `'guest'` → `'user'` | mechanisch; der Test muss belegen, dass ein neues Hauptbenutzerkonto dieselben drei Zeilen bekommt wie heute ein Gast |
| `server/queries/app/accounts.sql:8` | `ORDER BY role = 'guest', created_at DESC` — Hauptkonto zuoberst | mit mehreren Administratoren ist die Reihenfolge unter ihnen unbestimmt | `ORDER BY is_owner DESC, role = 'user', created_at DESC, id DESC` |
| `server/internal/api/session.go:117` | `admin` verlangt `IsOwner()` | der Kern der Änderung | der eine Ort, an dem die Rolle wirklich entschieden wird — er ist durch `TestEveryRouteHasVisibility` und `TestContractCoversEveryOperation` gedeckt |
| `server/internal/auth/auth.go:194` (`CreateOwner`) | wirft `ErrSetupDone`, sobald **irgendein** Konto existiert | bleibt richtig: der Assistent legt weiterhin genau einen Besitzer an, weitere Administratoren entstehen nur durch Ernennung | unverändert, aber der Kommentar `:159` («Because M1a has exactly one owner») ist überholt und wird nachgezogen |
| die fünf Go-Literale ausserhalb von `gen/` und den Tests | `server/internal/auth/auth.go:44` (`roleOwner = "owner"`), `server/internal/model/model.go:207` (`IsOwner`), `server/internal/api/session.go:268` (`if a.Role == "guest"` beim Bauen von `SessionInfo`), `server/internal/visibility/vistest/vistest.go:325` und `:641` (der Fixture-Bausatz) | alle fünf wandern auf die neuen Werte; `vistest` bekommt zusätzlich den zweiten Administrator aus 9.3 | mechanisch, aber `vistest` zuerst — an ihm hängen die 37 Treffer in den Go-Tests |

### 12.3 Weitere Risiken

| Risiko | Warum es wehtut | Gegenmassnahme |
|---|---|---|
| **Der Tabellenumbau verliert eine Spalte** | `accounts` hat seit 00002 mehrfach Spalten bekommen (00007: vier; dazu `max_bitrate_kbps`, `max_height`, `max_streams` aus W-8). Eine vergessene Spalte verschwindet beim `INSERT ... SELECT` stumm, und die Sicherung vor der Migration ist dann die einzige Rettung | Spaltenliste gegen `PRAGMA table_info(accounts)` auf dem eingefrorenen M11-Stand abgleichen (4.6 Schritt 2); `TestFrozenStatesMigrate` über alle elf Stände; danach ein Test, der jede Spalte des alten Stands im neuen wiederfindet |
| **Die View `visible_items` wird nicht wortgleich wiederhergestellt** | sie ist die **einzige** Stelle für Sichtbarkeit (`CLAUDE.md` Regel 2, Plan 3.3). Ein Tippfehler in `min(p.max_rating, a.max_rating)` öffnet still Titel für Kinderprofile | die View wird aus 00007 (`:100` bis `:117`) kopiert, nicht neu geschrieben; der Kinderprofil-Vertragstest (Plan 10.2) ist der Beweis und läuft in `task ci` |
| **Fremd-Clients brechen ohne Vorwarnung** | C-5; der Enum-Wechsel ist für sie fatal | Fassung anheben (F-4), Bruch in den Release-Notes wörtlich nennen — genau wie bei 1.1.0 (`docs/dev/CURRENT_STATUS.md:13`) |
| **Eine Operation bekommt `x-visibility: owner` und keine Registry** | der Vertragstest fängt es (`contract_access_test.go:367`), **sofern** der neue Zweig dort eingebaut ist | Schritt 2 baut den Zweig **vor** der ersten `owner`-Operation ein; der Test wird einmal absichtlich rot gefahren, und das steht im Commit-Body |
| **Die Oberfläche verlässt sich auf `role` — an 21 Stellen** | `=== 'owner'` träfe nach dem Wechsel nie mehr zu. Die betroffenen Seiten zeigten dann stumm ihre Meldung «nicht erlaubt» statt ihres Inhalts, und zwar **auch dem Besitzer**. Ein Test, der nur auf Statuscodes schaut, bemerkt das nicht: Der Server antwortet korrekt, die Seite fragt ihn gar nicht erst | `git grep -n "'owner'\|'guest'" -- web/src` ist vor dem Abschluss von Schritt 5 leer, mit genau zwei begründeten Ausnahmen: `DevicesPage.tsx:55` (Spalte «Eigentümer») und `ChannelSettingsPage.tsx` (umbenannte Variable). Dazu meldet sich der Playwright-Test als Administrator an und erwartet auf jeder der 21 Seiten den Inhalt, nicht den Hinweis |
| **Die Sitzungsattrappe der Web-Tests kippt alles auf einmal** | `web/src/test/api.tsx:41` setzt `role: 'owner'` für jeden vitest-Lauf. Wird sie zuletzt geändert, sind bis dahin alle Seitentests rot und verdecken echte Fehler; wird sie zuerst geändert, sind alle Seiten rot, bis sie nachgezogen sind | die Attrappe und die 21 Seiten gehen in **einem** Commit (Schritt 5); der Commit-Body nennt die Reihenfolge, und `task ci` läuft erst am Ende |
| **Der Besitzer sperrt sich selbst aus** | er degradiert den einzigen anderen Administrator und löscht dann sein eigenes Konto | 9.1 und 9.2 zusammen schliessen das aus: der Besitzer ist nicht löschbar, Punkt. Es gibt keinen Pfad zu einem Server ohne Administrator |
| **Ein Kinderprofil eines Administrators gilt als Administrator** | der gefährlichste denkbare Fehler: das Kind am Elterngerät erreicht die Bibliotheksverwaltung | die Hauptprofil-Bedingung bleibt unangetastet (5.1, 9.6); der erweiterte Akteur-Satz aus 9.3 prüft genau diesen Fall bei **jeder** `admin`- und `owner`-Operation, nicht nur bei einer Stichprobe |
