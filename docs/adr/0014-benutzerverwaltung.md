# ADR 0014: Drei Ebenen – Administrator, Konto, Profil

- Status: angenommen (28. Oktober 2026)
- Quelle: Konzept `docs/planning/06-benutzerverwaltung.md` (27./28. Oktober 2026); **Frage 12** in
  `docs/planning/04-entscheidungen.md` mit Marcos acht Antworten; `docs/planning/03-plan-v1.md`
  Abschnitte 2.1 (Prinzip 1), 3.3, 10.2; ADR 0005 (Profilwahl und Stufe `account`) und ADR 0009
  (Kanäle, Ereignis `errors`)
- Betroffene Spec-IDs: K-1, K-2, K-6, K-7, K-8, C-5, A-2; das Nicht-Ziel
  `docs/spec/01-anforderungen-v1.md:68` wird **hier abgelöst**, nicht umgeschrieben
  (`docs/spec/README.md:6`)
- Harte Regeln: 1 (generierter Code nur durch Codegen), 2 (Sichtbarkeit nur über `visibility`),
  5 (Migrationen nur vorwärts), 7 (`x-visibility` je Operation und Eintrag im Vertragstest),
  8 (Tastaturtest je Seite), 9 (keine harten Farben und Texte), 11 (keine Nicht-Ziele),
  12 (Entscheid im Server)

## Kontext

Bis M11 kannte Slouchcat genau zwei Arten von Konten: das eine Besitzerkonto, das der
Einrichtungsassistent anlegt (`accounts.role = 'owner'`), und Gastkonten aus einer Einladung
(`'guest'`). Slouchcat soll öffentlich angeboten werden und gegen Emby und Jellyfin antreten. Für ein
solches Produkt hat dieses Modell vier harte Kanten:

1. **Es gibt genau einen Administrator, und er ist nicht vermehrbar.** `CreateOwner` verweigert jedes
   weitere Konto, sobald eines existiert, und kein Endpunkt ändert `accounts.role`. Fällt der
   Betreiber aus, gibt es keinen zweiten.
2. **Das Wort «Gast» verspricht etwas Kleineres, als es bedeutet.** Ein Gastkonto ist in Wahrheit ein
   vollwertiger Haushaltsvorstand mit eigenen Profilen, eigener PIN-Verwaltung, eigenen Kanälen und
   eigener Offline-Bibliothek. Der Code tut bereits das Richtige; nur der Begriff ist falsch.
3. **Die Serververwaltung hängt an einer Kontozeile statt an einer Rolle.** Drei SQL-Abfragen fragen
   wörtlich `role = 'owner'` ab. Solange es genau einen Besitzer gibt, sind «der Besitzer» und «die
   Administration» dasselbe; mit zwei Administratoren sind sie es nicht mehr, und jede dieser Stellen
   träfe dann stillschweigend eine andere Entscheidung als die beabsichtigte.
4. **Es gibt keinen unzerstörbaren Anker.** Mit mehreren Administratoren könnte sich der Server selbst
   aussperren: der letzte Administrator entfernt sein eigenes Konto, und niemand kommt mehr an die
   Verwaltung.

Vier Fragen waren zu entscheiden:

1. Wie viele Ebenen gibt es, und wo verläuft die Grenze zwischen Serververwaltung und Kontoverwaltung?
2. Wie wird der Besitzer markiert, wenn `role` ihn nicht mehr trägt?
3. Wie wird «nur der Besitzer darf das» durchgesetzt – in der Middleware oder im Handler?
4. Was passiert mit den Grenzen eines Kontos (`max_rating`, `all_libraries`), wenn es Administrator
   wird und wenn es die Rolle wieder verliert?

Verworfen: (a) **Die gespeicherten Werte `'owner'`/`'guest'` behalten und nur im Vertrag umbenennen** –
es gäbe dauerhaft zwei Vokabulare für dieselbe Sache, und `'owner'` wäre schlicht falsch, sobald es
zwei Administratoren gibt. (b) **Den Besitzer als dritten Rollenwert `role = 'owner'` neben `'admin'`
führen** – jede Abfrage nach «ist Administrator» müsste zwei Werte prüfen (`role IN ('admin',
'owner')`), und genau dieses Vergessen wäre eine Rechteausweitung mit einem einzigen fehlenden Wort.
(c) **Den Besitzer als Eintrag `owner_account_id` in `settings`** – `settings` kennt keinen
Fremdschlüssel und kein `ON DELETE`; ein Zeiger auf eine gelöschte Kontozeile wäre genau die stille
Inkonsistenz, gegen die dieser Entscheid ankämpft. (d) **Eine Spalte `role` in `invitations`**, damit
eine Einladung «als Administrator» möglich wäre – eine Einladung ist ein Link, der sieben Tage in
einem Postfach liegt; wer ihn abfängt, bekäme die Serververwaltung. (e) **`is_owner` im Handler
prüfen statt eine eigene Sichtbarkeitsstufe** – ein vergessenes `if` wäre dann eine stille
Rechteausweitung statt eines roten Tests. (f) **Eine eigene Sichtbarkeitsstufe für die
Profilverwaltung** – sie müsste trotzdem noch prüfen, ob das Zielprofil zum Konto der Sitzung gehört,
und das kann die Middleware nicht; die Stufe `account` plus Hauptprofil-Regel ist genau dafür
erfunden worden (ADR 0005). (g) **`transferOwnership`** – Marco hat die Übergabe der Besitzrolle für
diese Runde abgelehnt (Frage 12 g).

## Entscheid

| Punkt | Entscheid |
|---|---|
| Drei Ebenen | **Administrator** verwaltet Server, Bibliotheken, Einstellungen, Jobs und Integrationen. **Konto** verwaltet ausschliesslich die eigenen Profile, Kanäle, E-Mail und Passwort. **Profil** schaut und verwaltet nichts. Die Ebenen schliessen sich nicht aus, sie stapeln sich: jedes Administratorkonto ist gleichzeitig ein Konto mit eigenen Profilen |
| Rollenwerte | `accounts.role` kennt nur noch `'admin'` und `'user'`; der Standardwert wechselt von `'owner'` auf `'user'`. Ein vergessenes `role` bei einem `INSERT` erzeugt damit ein harmloses Konto statt still einen Administrator. `'owner'` und `'guest'` sind verbrannt und werden nie wiederverwendet |
| Besitzer | Eine neue Spalte `accounts.is_owner` (0/1) mit dem Teilindex `accounts_owner … WHERE is_owner = 1`: höchstens einer, von der Datenbank erzwungen, und die Markierung verschwindet mit der Kontozeile. Der Besitzer trägt `role = 'admin'` **und** `is_owner = 1`. Er ist nicht löschbar und nicht degradierbar; eine Übergabe gibt es vorerst nicht |
| Zugang | Konten entstehen **nur per Einladung der Administration** (einmalig, 7 Tage, widerrufbar, K-6). Keine Selbstregistrierung, und kein Konto lädt weiter ein. `invitations` bekommt deshalb **keine** Spalte für eine Rolle, und `InsertAccount` schreibt `'user'` als Literal in der SQL-Datei, nicht als Parameter |
| Ernennen und entziehen | Die neue Operation `setAccountRole` (`PUT /admin/accounts/{id}/role`) trägt die neue Stufe **`owner`**. Die Ernennung setzt `max_rating = 18` und `all_libraries = 1`; der Entzug setzt sie **nicht** zurück und widerruft alle Sitzungen des Zielkontos. Die Profile behalten ihre eigenen Stufen |
| Grenzen eines Administratorkontos | `CHECK (role <> 'admin' OR (max_rating = 18 AND all_libraries = 1))`. Die Verwaltungsseiten lesen systemweit und ohne Profilprüfung (`getAdminItem`, `searchAdminItems`, `getAdminItemImage`); eine Kontogrenze wäre dort ein falsches Versprechen statt eines Schutzes |
| Sechste Sichtbarkeitsstufe | Der Vertrag kennt `public`, `account`, `scoped`, `system`, `admin` und neu **`owner`**. Die Middleware setzt sie an einer Stelle durch und fällt geschlossen aus; der Vertragstest erzwingt für jede `owner`-Operation einen Eintrag in `ownerRegistry` und fährt sie gegen jeden Akteur |
| Hauptprofil-Regel | **Unverändert aus ADR 0005**: `admin` verlangt das aktive Hauptprofil, und `owner` erbt dieselbe Bedingung. Ein Kinderprofil im Konto eines Administrators kommt nirgends hin. Das ist die wichtigste Nicht-Änderung dieses Entscheids |
| Fehlercodes | Neu `403 owner_required` (eine `owner`-Operation von einem Administrator, der nicht der Besitzer ist), `409 last_admin` (der letzte Administrator soll degradiert oder gelöscht werden), `409 owner_protected` (der Besitzer soll degradiert oder gelöscht werden) und `409 admin_limits_fixed` (an einem Administratorkonto sollen Altersstufe oder Bibliotheken geändert werden; es ist auf 18 und alle festgelegt, siehe die Zeile «Grenzen eines Administratorkontos»). `profile_required` und `primary_profile_required` bleiben unverändert; der Text von `forbidden` wechselt von «Only the main account …» auf «Only an administrator …» |
| Vertragsfassung | **1.2.0.** Der Bruch – Enum ohne `owner` und `guest`, Wegfall von `GET /admin/guests`, die Umbenennungen `updateGuest` → `updateAccountAccess` und `revokeGuest` → `deleteAccount` – steht wörtlich in den Release-Notes (C-5) |
| Oberfläche | Die Ebenen heissen **«Administrator»**, **«Konto»** und **«Profil»** (englisch `administrator`, `account`, `profile`); «Besitzer» ist nur ein Abzeichen am einen Konto. Die Vertragswerte bleiben `admin` und `user`. Der Client setzt kein Recht durch (Regel 12): er blendet Bedienelemente aus, der Server entscheidet |
| Nicht-Ziel Mandantenfähigkeit | `docs/spec/01-anforderungen-v1.md:68` wird **nicht umgeschrieben**, sondern hier abgelöst. Nicht-Ziel bleiben getrennte Medienbestände, getrennte Bibliotheken und getrennte Daten je Konto: es gibt genau **einen** Bestand, und die Administration entscheidet je Konto über Altersstufe und Bibliotheksrechte. Mehrere Konten mit eigenen Profilen und mehrere Administratoren sind ausdrücklich **kein** Nicht-Ziel |

## Sechs Riegel gegen Rechteausweitung

| Riegel | Regel |
|---|---|
| Der letzte Administrator ist nicht entfernbar | `setAccountRole` auf `user` und `deleteAccount` scheitern mit `409 last_admin`, wenn danach kein Konto mit `role = 'admin'` übrig bliebe. Geprüft **in derselben Schreibtransaktion** wie die Änderung |
| Der Besitzer ist nicht degradierbar und nicht löschbar | `409 owner_protected`, sobald das Zielkonto `is_owner = 1` trägt. Die Prüfung steht vor jeder anderen, damit sie auch dann greift, wenn der Besitzer zufällig der letzte Administrator ist |
| Selbst-Ernennung ist unmöglich | `setAccountRole` trägt `x-visibility: owner`; die Middleware lehnt jeden Aufrufer ohne `is_owner = 1` mit `403 owner_required` ab, bevor ein Handler das Zielkonto sieht |
| Eine Einladung erzeugt nie einen Administrator | keine Rollenspalte in `invitations`, `'user'` als SQL-Literal; ein Schema-Test verbietet ein Feld `role` oder `isOwner` in `AcceptInvitationRequest` und `InvitationCreate` |
| Niemand hebt seine eigenen Grenzen an | `max_rating`, `all_libraries` und `account_libraries` schreibt nur `updateAccountAccess` (Stufe `admin`); `updateAccount` ändert ausschliesslich E-Mail und Passwort und verlangt dafür das aktuelle Passwort |
| `admin` und `owner` verlangen das aktive Hauptprofil | unverändert aus ADR 0005, durchgesetzt in der Middleware |

## Folgen

- **Datenmodell:** Migration `00022_account_roles.sql` baut `accounts` um (SQLite kennt kein
  `ALTER COLUMN` für `CHECK`): View ablegen, neue Tabelle mit allen heutigen Spalten anlegen, Zeilen
  mit `CASE role WHEN 'owner' THEN 'admin' ELSE 'user' END` und `is_owner` kopieren, tauschen, View
  wiederherstellen, Gegenprobe. Die `id` bleibt erhalten, darum stimmen alle Kindtabellen weiter.
  **Die View `visible_items` wird wortgleich aus Migration 00010 wiederhergestellt, nicht aus
  00007:** 00010 hat sie zuletzt neu geschrieben (Sichtbarkeit von Extras unter einem sichtbaren
  Elternteil), und 00007 wäre ein stiller Rückschritt. Sie liest `accounts`, aber nie `role`.
- **Vertrag:** `Account.role` und `AdminAccount.role` verlieren `owner` und `guest` und bekommen
  `admin`/`user`, dazu das neue Pflichtfeld `isOwner`. `Guest` geht in `AdminAccount` auf,
  `GET /admin/guests` entfällt, `listAccounts` wird die einzige Kontoliste. Fassung 1.2.0.
- **Durchsetzung:** `model.Session.IsOwner()` prüft nicht mehr `Role == "owner"`, sondern
  `Account.IsOwner`; neu ist `IsAdmin()` für `Role == "admin"`. Die Middleware prüft `IsAdmin` für
  `admin` und `IsOwner` für `owner`, beide zusammen mit `IsPrimary`.
- **Die drei zweideutigen Abfragen bekommen je eine Bedeutung:** `AdminOwnerProfileIDs` (Zielmenge
  von Cat's Empfehlungen, E-2) und `NotifyOwner` (der eine Empfänger ohne Adresse) meinen den
  **Besitzer** und fragen `is_owner = 1`; `ChannelTargetsOwners` (Ereignis `errors`, ADR 0009) meint
  **alle Administratoren** und fragt `role = 'admin'` – wer verwaltet, muss von Fehlern erfahren.
- **Eine Kontoliste statt zweier.** `GET /admin/guests` entfällt; `listAccounts` trägt Rolle,
  Abzeichen, Grenzen, Profile und Kanäle jedes Kontos. `updateGuest` heisst `updateAccountAccess`
  (`PATCH /admin/accounts/{id}`), `revokeGuest` heisst `deleteAccount`
  (`DELETE /admin/accounts/{id}`); beide antworten 204, die Oberfläche liest die Liste neu. Das
  Schema `Guest` geht in `AdminAccount` auf.
- **Oberfläche:** 21 Web-Dateien prüfen heute `session.account.role === 'owner'`; alle einundzwanzig
  meinen «Administrator» und wechseln auf `'admin'`. Die gemeinsame Sitzungsattrappe
  `web/src/test/api.tsx` geht mit ihnen in **einen** Commit, sonst fallen alle Seitentests
  gleichzeitig um. `DevicesPage.tsx` behält seine Spalte «Eigentümer» (`AdminDevice.ownerEmail`) –
  sie hat mit Rollen nichts zu tun.
- **Bestand:** Laufende Sitzungen, Stream-Tokens, Offline-Downloads, offene Einladungen und
  Benachrichtigungskanäle bleiben gültig; die Umbenennung nimmt niemandem ein Recht. `SessionInfo`
  liefert für jeden angemeldeten Browser einen neuen `role`-Wert – genau deshalb steigt die Fassung.
- **ADR 0005 bleibt in Kraft**, mit zwei Nachträgen: die Stufe `admin` verlangt nicht mehr das eine
  Hauptkonto, sondern **ein Administratorkonto** mit seinem Hauptprofil, und es gibt eine sechste
  Stufe `owner` mit derselben Hauptprofil-Bedingung.
- **Spätere Meilensteine:** Bearer-Tokens für Fremd-Clients (C-5) erben die Rollenprüfung über
  dieselbe Middleware. Ein Besitzerwechsel braucht bis auf Weiteres einen Eingriff in die Datenbank;
  `transferOwnership` bleibt ein Kandidat für die Ideensammlung.
