# Analyse B: Konten, Profile, Berechtigungen, Jugendschutz, Benachrichtigungen

Stand: Haupt-Checkout, Zweig `main` (HEAD `ba6efe31`, Vertrag `api/openapi.yaml` 1.40.0), gelesen am 16.11.2026.
Nur gelesen, nichts geändert, nichts gebaut oder getestet.

Kennzeichnung: **[gelesen]** = im Code oder in den Papieren belegt. **[Vermutung]** = abgeleitet, nicht belegt.
Pfade sind relativ zum Repo `/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat`.

---

## 0. Vorab: Wünschen ist heute ein ausdrückliches Nicht-Ziel

Das betrifft das ganze Konzept, deshalb steht es vor Abschnitt 1. **[gelesen]**

- `CLAUDE.md` Regel 11 nennt «eigene Wunschverwaltung» und «Steuerung von Arr/Seerr» als Nicht-Ziele.
- `docs/spec/01-anforderungen-v1.md:46`: «Wünschen von Inhalten erfolgt ausserhalb von Slouchcat direkt in Seerr …;
  Slouchcat baut weder eine eigene Wunschverwaltung noch einen Seerr-Bereich (Runde 20).»
- `docs/spec/01-anforderungen-v1.md:66-67`: Non-Goals «Eigene Wunschverwaltung und ein Bereich „Wünschen“ über die
  Seerr-API» sowie «Beschaffungsstatus … sowie die Benachrichtigung bei Verfügbarkeit (Runde 20, Kandidaten für die
  Ideensammlung)».
- `docs/spec/01-anforderungen-v1.md:159`: I-1 bis I-4 und I-6 sind entfallen, die Nummern bleiben frei.
- `docs/planning/03-plan-v1.md:286-292`: Entscheid 5.1 (Seerr) ist «entfallen». Der frühere Entscheid lautete: eigener
  Bereich «Wünschen» mit Slouchcat-Oberfläche über die Seerr-API, Kontozuordnung durch den Admin.
- `docs/planning/04-entscheidungen.md:19` (Frage 10, 22.10.2026): Der Vorschlag lautete «Profile des Hauptkontos ab FSK 12
  dürfen wünschen, Gäste nicht; jedem Konto wird ein bestehender Seerr-Benutzer zugeordnet». Gestrichen wurden I-3
  (Benachrichtigung bei Verfügbarkeit), I-4 (Bereich «Wünschen» über Seerr), AN-5 (Wunschrecht) und AN-16
  (Kontozuordnung). Die Statusanzeige und das Wünschen sind «Kandidaten für die Ideensammlung».
- Vorbild, wie ein Nicht-Ziel aufgehoben wird: ADR 0028 (Ausnahme nur für anonyme Empfehlungen,
  `docs/adr/0028-user-empfehlungen-von-profil-zu-profil.md:23-25`), ADR 0032 (hebt B-1 in einem Punkt auf,
  `docs/adr/0032-dateien-loeschen.md:3-7`) und ADR 0014 (löst ein Nicht-Ziel ab, ohne die Spec umzuschreiben,
  `docs/adr/0014-benutzerverwaltung.md:151`).

---

## 1. Modell Administrator / Konto / Profil

### 1.1 Tabellen und Spalten [gelesen]

**`accounts`** (zuletzt neu gebaut in `server/migrations/00063_minimum_age_accounts.sql:27-45`):
`id`, `email` (UNIQUE, NOCASE), `password_hash` (argon2id), `role` (`'admin' | 'user'`, Standard `'user'`),
`is_owner` (0/1), `created_at`, `updated_at`, `max_rating` (0–18, Standard 18), `all_libraries` (0/1),
`invited_by` (FK accounts), `last_seen_at`, `max_bitrate_kbps`, `max_height` (360/480/720/1080/2160), `max_streams` (1–16).
- CHECK `role <> 'admin' OR (max_rating = 18 AND all_libraries = 1)` und CHECK `is_owner = 0 OR role = 'admin'` (`:43-44`).
- Teilindex `accounts_owner … WHERE is_owner = 1`: höchstens ein Besitzer (`:58`).
- Es gibt **keine** Spalte für Sprache, Wunschrecht oder Seerr-Benutzer am Konto.

**`account_libraries`** (`server/migrations/00007_accounts_profiles_access.sql:29`): die Bibliotheken eines Kontos mit `all_libraries = 0`.

**`profiles`** (zuletzt neu gebaut in `server/migrations/00061_profiles_open_language.sql:34-57`):
`id`, `account_id` (FK, CASCADE), `name` (1–60), `image`, `max_rating`, `language` (Oberfläche, Standard `'de'`),
`accent` (Farbe), `all_libraries`, `is_default` (= Hauptprofil), `created_at`, `updated_at`, `pin_hash`,
`max_bitrate_kbps`, `max_height`, `max_streams`, `watch_limit_minutes` (1–1440, K-5), `pin_salt`, `offline_pin` (JSON),
`metadata_language`, `audio_languages` (JSON-Array), `subtitle_mode` (`off|forced|foreign|always`), `subtitle_languages`.
- `profiles_primary … WHERE is_default = 1`: genau ein Hauptprofil je Konto (`:73`).
- **Es gibt kein Merkmal «Kinderprofil».** Ein «Kinderprofil» ist ein Profil mit niedriger `max_rating`. Weder die
  Tabelle noch `model.Profile` (`server/internal/model/model.go:197-219`) kennen ein Feld `isKid` oder Ähnliches.

**`profile_libraries`** (neu gebaut in `00055_library_and_title_deletion.sql:33`, mit FK zu `libraries`).

**`sessions`** (`00007…:57ff.`): `profile_id` nullable, `device_id`. **`devices`** (`00007…:40-45`): `id`, `user_agent`,
`created_at`, `last_seen_at`. **`pin_attempts`** (`00007…:47-55`): Sperre je Profil und Gerät.

**`invitations`** (`00063…:68-80`): `token_hash`, `note`, `max_rating`, `all_libraries`, `created_by`, `expires_at`,
`used_at`, `used_by`, `revoked_at`, dazu `invitation_libraries`. Bewusst **ohne Rollenspalte**
(`docs/adr/0014-benutzerverwaltung.md:143,160`).

**Sichtbarkeit** über die View `visible_items` (`00063…:86-105`): Freigabe `COALESCE(i.effective_rating, 18) <= min(p.max_rating, a.max_rating)`,
für Serien, Staffeln und Podcasts zusätzlich `min_child_rating`, Bibliotheken über Profil- **und** Kontoliste, keine
gelöschten oder nicht verfügbaren Titel, keine Bibliothek im Löschen.

### 1.2 Rollen und Ebenen [gelesen]

- Drei Ebenen, die sich stapeln: **Administrator** (Server, Bibliotheken, Einstellungen, Jobs, Integrationen),
  **Konto** (nur eigene Profile, Kanäle, E-Mail, Passwort), **Profil** (schaut, verwaltet nichts)
  (`docs/adr/0014-benutzerverwaltung.md:140`).
- Rollenwerte nur `admin` und `user` (`:141`). Der **Besitzer** ist `role = 'admin'` und `is_owner = 1`, nicht
  löschbar und nicht degradierbar (`:142`). In der Oberfläche ist «Besitzer» nur ein Abzeichen (`:150`).
- **Hauptprofil** (`is_default`): wird mit dem Konto angelegt und ist nicht löschbar. Nur mit aktivem Hauptprofil
  darf eine Sitzung Profile anlegen, ändern, löschen oder PINs setzen (`docs/adr/0005-profilwahl-sitzung.md:43`).
  Ohne PIN auf dem Hauptprofil kann jeder im Haushalt verwalten (ebenda).
- `model.Session.IsAdmin/IsOwner/HasProfile/IsPrimary`: `server/internal/model/model.go:276-286`.

### 1.3 Was zum Konto gehört und was zum Profil [gelesen]

| Merkmal | Konto | Profil | Beleg |
|---|---|---|---|
| E-Mail, Passwort | ja | – | `00063…:29-30` |
| Rolle, Besitzer | ja | – | `00063…:31-32` |
| Altersobergrenze | `max_rating` als Deckel | `max_rating` (min. beider gilt) | View `visible_items`, `00063…:92` |
| Bibliotheken | `all_libraries` und `account_libraries` | `all_libraries` und `profile_libraries` (beide gelten) | `00063…:99-104` |
| Bandbreite, Höhe, Streams (W-8) | Deckel für alle Profile | eigene Werte | `model.go:229-236`, `AccessForm.tsx:87-88` |
| Schauzeit pro Tag (K-5) | – | `watch_limit_minutes` | `00061…:51` |
| PIN, Offline-PIN | – | `pin_hash`, `pin_salt`, `offline_pin` | `00061…:48,52-53` |
| Oberflächensprache, Sprache der Beschreibungen | – (die Sprache eines Kontos ist die des Hauptprofils) | `language`, `metadata_language` | `00061…:42`, `queries/system/notify_channels.sql:97-104` |
| Farbe, Bild | – | `accent`, `image` | `00061…:40,43` |
| Ton- und Untertitelregeln | – | `audio_languages`, `subtitle_mode`, `subtitle_languages` | `00061…:54-56` |
| Benachrichtigungskanäle (E-Mail, Push, Telegram) | ja (`account_channels`) | – | `00016_admin.sql:27-35` |
| In-App-Eingang, Ankündigungs-Quittung | – | `notifications`, `announcement_acks` | `00015_discover.sql:329-338`, `00016…:91-96` |
| Merkliste (löst «neue Folge» aus) | – | `watchlist` | `00054_watchlist_follow.sql:23-29` |

### 1.4 Wie viele Rechte-Ebenen, welche Einzelschalter [gelesen]

Es gibt **keine Einzelberechtigungen** im Sinn von Schaltern wie «darf löschen» oder «darf herunterladen». Die Rechte
ergeben sich aus diesen vier Dingen:

1. **Rolle** des Kontos (`admin`/`user`) und Besitzerabzeichen, nur zusammen mit dem aktiven Hauptprofil wirksam.
2. **Hauptprofil ja/nein** (Profile und Kanäle verwalten, Schauzeit-Bericht lesen).
3. **Sichtgrenzen:** Altersstufe und Bibliotheken je Konto und je Profil.
4. **Wiedergabe-Grenzen:** W-8 je Konto und je Profil, K-5 je Profil.

Was es zu den genannten Beispielen gibt:
- **Dateien löschen (ADR 0032):** Das ist **kein Schalter**. Löschen darf nur der Server-Administrator
  (`x-visibility: admin`, in der Web-App nur das Hauptprofil des Administrators). Der Schutz liegt im `:ro`-Mount des
  Betreibers. Eine Einstellung «Dateien löschen erlauben» wurde ausdrücklich verworfen
  (`docs/adr/0032-dateien-loeschen.md:34-36,89-91`).
- **Empfehlen (ADR 0028):** Das ist **kein Schalter**. Fest gilt: Empfehlen darf jedes Profil des **Besitzerkontos**
  (`accounts.is_owner`), andere Konten sehen keinen Stern. Geprüft wird im Server, und ein Nein antwortet 404
  (`docs/adr/0028-…md:51-55,84`). Der Vertrag kennt dafür `ItemDetail.canRecommend` (`api/openapi.yaml:11832`).
- **Downloads/Offline:** Dafür gibt es keinen Schalter je Konto oder Profil. Im Vertrag fand ich kein `can*`-Feld
  ausser `canRecommend` (Suche nach `^        can[A-Z]` in `api/openapi.yaml`). Jedes Profil hat Downloads, wo der
  Browser es kann (`web/src/app/AppLayout.tsx:346-354`).

**Wo gesetzt:** Kontogrenzen über `updateAccountAccess` (`PATCH /admin/accounts/{id}`, `admin`), auf der Seite
«Konten» im Formular «Zugriff» (`web/src/features/admin/accounts/AccessForm.tsx`). Die Rolle über `setAccountRole`
(`owner`). Die Profilwerte über `createProfile` und `updateProfile` (`account`, nur mit Hauptprofil), auf der Seite
`/settings/profiles` oder im eigenen Konto unter `/admin/accounts?item=me#meine-profile`.

---

## 2. Sitzung und aktives Profil

### 2.1 Ablauf je API-Aufruf [gelesen]

- Die Middleware `authorize` (`server/internal/api/session.go:88-133`) läuft nach dem Routing. Sie liest das Cookie
  `slouchcat_session` und löst es über `AuthService.Resolve` zu einem `model.Session` auf (`:176-189`), das Konto,
  aktives Profil und Gerät enthält (`model.go:266-272`). Danach legt sie die Sitzung in den Kontext (`:100-102`).
- Die Stufe kommt aus `x-visibility` der Operation (`visibilityOf`, `:57-66`). So wird sie durchgesetzt (`:108-129`):
  - `public`: geht durch.
  - unbekannt oder `system`: **401** und ein Log-Eintrag. `system` ist also über HTTP nie erreichbar; es steht nur für
    interne Aufrufer (`api/openapi.yaml:12`).
  - ohne Sitzung: 401 `unauthenticated`.
  - `admin`: `IsAdmin() && IsPrimary()`, sonst 403 `forbidden`.
  - `owner`: `IsOwner() && IsPrimary()`, sonst 403 `owner_required`.
  - `scoped`: `HasProfile()`, sonst 403 `profile_required`.
  - `account`: eine gültige Sitzung genügt.
- `validVisibility` = public, account, scoped, admin, owner (`session.go:137`).
- Verteilung im Vertrag: 77 `scoped`, 24 `account`, 151 `admin`, 4 `owner`, 8 `public` (gezählt mit grep).
- Profilwechsel: `selectProfile` setzt `profile_id` in place, widerruft die Wiedergabesitzungen und schickt
  `Clear-Site-Data` (`docs/adr/0005-…md:44`).
- Ein Schema-Test verlangt `x-visibility`, und jede Operation braucht einen Eintrag im Vertragstest
  (CLAUDE.md Regel 7). Antworten mit `ItemRef` müssen `scoped` sein (`docs/adr/0005-…md:40`).

### 2.2 Welche Stufe «Wunsch anlegen» bräuchte

**`scoped`** [Vermutung, begründet aus den gelesenen Regeln]:
- Der Wunsch gehört einem **Profil**: wer ihn bekommt, Altersprüfung, In-App-Eingang. `account` kennt kein aktives
  Profil und darf keine `ItemRef` liefern (`docs/adr/0005-…md:40`).
- Ist ein gewünschter Titel schon vorhanden, wäre die natürliche Antwort ein Verweis auf den Titel, also eine `ItemRef`.
  Das verlangt `scoped`.
- Die Altersprüfung muss im Server laufen (Regel 12). Für einen Titel, der noch **nicht** in `items` steht, kann
  `visible_items` nichts prüfen. Es braucht dann eine eigene Regel für die Freigabe aus TMDB, und das ist neu
  (siehe «Folgen»).
- Eine Wunschliste aller Profile des Kontos für den Hauptprofil-Inhaber wäre `account` plus `requirePrimary` (Muster
  `ListProfiles`, `server/internal/auth/profiles.go:79-84`). Für die Verwaltung aller Wünsche wäre es `admin`.

---

## 3. Verwaltung: Konten, Profile, Geräte (vollständige Liste)

Die Texte stammen aus `web/src/locales/de.json` und sind hier mit ihrem Schlüssel genannt. Die Seiten liegen im
Bereich «Nutzer & Geräte» (`admin.nav.areas.people`; `web/src/features/admin/sections.ts:74-75`).

### 3.1 Seite «Konten» `/admin/accounts` (`web/src/features/admin/AccountsPage.tsx`) [gelesen]

- **Zugang:** nur Administrator mit Hauptprofil. Sonst erscheint der Hinweis «Nur für Administratoren» / «Diesen
  Bereich verwaltet das Hauptprofil eines Administratorkontos.» (`:64-73`, `admin.access.notAllowed.*`).
- **Kopf:** Titel «Konten», Hilfetext `admin.accounts.intro`, Knopf **«Konto einladen»** (`admin.accounts.invitations.invite`,
  `:475-483`; gesperrt, bis die Bibliotheksliste geladen ist).
- **Statusleiste «Zusammenfassung»** (`:428-466`) mit vier Kacheln:
  - «Konten»: Anzahl.
  - «Administratoren»: Anzahl, Zusatz «davon N Besitzer».
  - «Profile»: Anzahl, Zusatz «N eigenes».
  - «Offene Einladungen»: Anzahl, Zusatz «Nächste gültig bis {{date}}».
- **Reiter** «Bereiche der Konten» (`?tab=accounts|invitations`, `:504-585`):
  - **«Konten»** mit Zähler «N Konto/Konten». Tabelle «Konten» (`accounts/columns.tsx:29-103`) mit den Spalten:
    «E-Mail» (Zeilenkopf, klappt auf), «Rolle» (Administrator/Konto und Abzeichen **«Besitzer»**), «Altersstufe»,
    «Bibliotheken» («Alle Bibliotheken» oder «N Bibliothek(en)»), «Profile» (Namen, Hauptprofil markiert «{{name}}
    (Hauptprofil)»), «Zuletzt gesehen» (oder «Noch nie»), «Kanäle» (eingeschaltete als Chips E-Mail/Web-Push/Telegram,
    sonst «Keine») und «Aktionen».
  - **«Einladungen»** mit Zähler «N offen». Liste «Bestehende Einladungen» (`accounts/Invitations.tsx:226-288`) mit den
    Spalten «Notiz» (oder «Ohne Notiz»), «Zustand» (Offen/Benutzt/Abgelaufen/Widerrufen), «Altersstufe»,
    «Bibliotheken», «Gültig bis / angenommen» («Gültig bis {{date}}» oder «Angenommen von {{email}}») und «Aktionen»
    mit dem Knopf «Widerrufen». Ohne Einladung steht «Noch keine Einladung erstellt.»
- **Aktionsmenü einer Kontozeile** («Weitere Aktionen», `:322-349`). Das eigene Konto und der Besitzer haben keines.
  - «Zum Administrator ernennen» oder «Rolle entziehen», **nur für den Besitzer**. Bestätigung: «{{email}} zum
    Administrator ernennen?» mit dem Text zu «ab 18 und alle Bibliotheken» und dem Knopf «Ernennen», bzw.
    «{{email}} die Administratorrolle entziehen?» mit dem Hinweis, dass das Konto überall abgemeldet wird, und dem
    Knopf «Entziehen».
  - «Konto entfernen» (Gefahr). Bestätigung: «Konto {{email}} entfernen?». Der Text sagt, dass Konto, Profile und
    Fortschritt gelöscht werden, Sitzungen und Wiedergaben sofort enden und Downloads beim nächsten Kontakt
    verschwinden. Knopf «Konto entfernen».
- **Aufklappbereich des eigenen Kontos** (`accounts/AccountDetail.tsx:30-96`):
  - **«Konto ändern»** (`accounts/OwnAccountForm.tsx:110-183`) mit dem Hinweis `admin.accounts.edit.intro` und den
    Feldern «E-Mail-Adresse», «Neues Passwort» (Hinweis «Leer lassen … mindestens 10 Zeichen»), «Neues Passwort
    wiederholen» und «Aktuelles Passwort» («Bestätigt, dass du es bist.»). Knöpfe «Verwerfen» und «Speichern».
    Meldungen: «Ändere die E-Mail-Adresse oder gib ein neues Passwort ein.», «Das aktuelle Passwort stimmt nicht.»
    und «Dein Konto ist gespeichert.»
  - **«Meine Profile»** (`#meine-profile`): die ganze Profilverwaltung aus 4.2 (Komponente `ProfileList`).
- **Aufklappbereich eines anderen Kontos** (`AccountDetail.tsx:104-157`):
  - Für ein Konto **ohne Verwaltung**: das Formular **«Zugriff»** (`accounts/AccessForm.tsx:94-153`) mit
    «Höchste Altersfreigabe» (Auswahl aus den Stufen des Freigabelands, höchstens 18), «Bibliotheken» (Alle oder
    Auswahl mit Häkchen «Bibliotheken wählen», Prüfung «Wähle mindestens eine Bibliothek.») und dem Feldsatz
    **«Wiedergabe-Limits»** mit dem Hinweis «Gilt für alle Profile dieses Kontos.»: «Höchste Qualität» (Ohne Limit,
    360p bis 2160p), «Bandbreite (kbit/s)» und «Gleichzeitige Streams», jedes mit «{{min}} bis {{max}}. Leer heisst
    ohne Limit.». Die Schauzeit fehlt hier (`withWatch={false}`). Knöpfe «Verwerfen» und «Zugriff speichern».
  - Für einen **Administrator**: statt des Formulars der Hinweis `admin.accounts.access.adminHint` («immer ab 18 und
    alle Bibliotheken»).
  - Block «Profile»: die Profilnamen als Chips, nur zum Lesen, mit dem Hinweis «Profile verwaltet jedes Konto selbst.»
- **Dialog «Konto einladen»** (`Invitations.tsx:183-200`) mit dem Hinweis «Ein Link gilt einmal und sieben Tage …».
  Felder: «Für wen ist der Link?» (Notiz, «Die Notiz sieht auch die eingeladene Person.»), «Höchste Altersfreigabe»
  und «Bibliotheken». Knopf «Einladung erstellen». Danach kommt der Einmal-Link: Hinweis «Kopiere den Link jetzt: Er
  wird nur dieses eine Mal angezeigt.», Feld «Einladungslink», Knöpfe «Link kopieren» und «Fertig», Meldungen
  «Link kopiert.» und «Kopieren nicht möglich …».
- **Bestätigung «Einladung widerrufen?»** mit «Der Link funktioniert danach nicht mehr.» und dem Knopf «Widerrufen».
- **Ungespeicherte Änderungen:** Beim Verlassen kommt eine Rückfrage (`useUnsavedChanges`, `:119,599`). Die Entwürfe
  bleiben je Konto erhalten.
- `/admin/access` leitet auf `/admin/accounts` um (`web/src/app/router.tsx:471-480`; ADR 0014 `:188-194`).
- **Annahme der Einladung** (öffentliche Seite `/invite/$token`): «E-Mail-Adresse», «Passwort» («Mindestens 10
  Zeichen.»), «Profilname», «Sprache», Knopf «Konto anlegen» (`invite.form.*`).

### 3.2 Seite «Geräte» `/admin/devices` (`web/src/features/admin/DevicesPage.tsx`) [gelesen; die Texte kommen aus den `admin.devices.*`-Schlüsseln, den Ablauf habe ich nur überflogen]

- Titel «Geräte» mit dem Hinweis «Browser und Geräte, die sich angemeldet haben, mit dem Fähigkeitsprofil ihres
  letzten Wiedergabestarts.»
- Reiter «Geräte» (gruppiert nach Typ: Computer, Telefon, Tablet, TV, Anderes, Unbekannt) und «Fähigkeiten».
- Spalten: «Gerät», «Eigentümer» («{{email}} · {{profile}}» oder «Noch keine Wiedergabe»), «Letzter Kontakt»,
  «Container», «Video», «Audio», «Max. Auflösung», «HDR», «Streams» und «Aktionen».
- Aktion «Vergessen». Bestätigung «Gerät „{{name}}“ vergessen?»: laufende Streams enden, Sitzungen werden abgemeldet.
  Knopf «Gerät vergessen» (`deleteDevice`, `DELETE /admin/devices/{id}`, `admin`).

### 3.3 Weitere Seiten zu Konten und Profilen in der Verwaltung [gelesen]

- **«Benachrichtigungen»** `/admin/notifications` zeigt dieselbe Kanalseite wie `/settings/notifications`, nur in der
  Verwaltungsleiste. Ein Administrator bekommt dort oben zusätzlich einen Link zu den Kanälen des Servers
  (`web/src/app/router.tsx:603-611`, `features/notifications/ChannelSettingsPage.tsx:57,146,256-262`; Frage 15 F-7 in
  `04-entscheidungen.md:87`).
- **«Ankündigungen»**: siehe 7.3.
- **«Streams»** und **«Statistiken»** zeigen Profile mit Namen (siehe 10).

---

## 4. Profileinstellungen in der Zuschauer-App [gelesen]

Seitenleiste, Band «Konto» (`web/src/app/AppLayout.tsx:356-386`):
- «Einstellungen» → `/settings/playback`, für jedes Profil.
- «Profile» → `/settings/profiles`, **nur mit Hauptprofil**.
- «Verwaltung» → `/admin`, nur für Administratoren, mit Abzeichen bei einem Update.
- Die Profilkarte oben zeigt Name und Rolle («Server-Admin», «Hauptprofil» oder «Weiteres Profil»,
  `nav.profileRole.*`) und den Knopf «Wechseln» → `/profiles`. Unten steht «Abmelden».
- «Downloads» → `/downloads`, wo der Browser es kann (`:346-354`).

### 4.1 `/profiles`: Profilwahl «Wer schaut?» (`features/profiles/ProfilePickerPage.tsx`)
- Liste der Profile mit den Markierungen «aktiv», «mit PIN geschützt», «Hauptprofil» und «Hauptprofil · aktiv».
  Knopf «Profile verwalten». Texte `profiles.picker.*`.
- PIN-Dialog «PIN für {{name}}» (`PinDialog.tsx:79-110`): Feld «PIN» («4 bis 8 Ziffern.»), Knöpfe «Öffnen» und
  «Abbrechen». Bei einer Sperre «Zu viele Fehlversuche. Neuer Versuch in mm:ss».

### 4.2 `/settings/profiles`: «Profile» (`ProfileSettingsPage.tsx`, `ProfileList.tsx`, `ProfileForm.tsx`)
- Ohne Hauptprofil kommt der Hinweis «Nur im Hauptprofil» mit dem Knopf «Profil wechseln» (`ProfileSettingsPage.tsx:21-35`).
- Kopf: Hinweis `profiles.settings.intro` und Knopf **«Profil hinzufügen»**.
- **Karte je Profil** (`ProfileList.tsx:197-300`): Farbe und Buchstabe, Name, Chips «Hauptprofil», Altersstufe (oder
  «{{rating}} · Konto höchstens {{max}}»), Sprache, «Sprache der Beschreibungen: …», «PIN» und die wirksamen Limits.
  Darunter die Bibliotheken. Knöpfe:
  - «Bearbeiten».
  - «Schauzeit anzeigen/ausblenden»: Bericht «Schauzeit von {{name}}», «Minuten pro Tag in den letzten 30 Tagen.»,
    «Heute / Total / Limit», Diagramm. Abruf über `getWatchReport` (nur Hauptprofil, `server/internal/playback/limits.go:302-311`).
  - Menü: «PIN setzen/ändern», «PIN entfernen» (Bestätigung «PIN von {{name}} entfernen?») und «Löschen» (nicht beim
    Hauptprofil; Bestätigung «Profil „{{name}}“ löschen?»: Fortschritt und Einstellungen gehen verloren).
- **Dialog «Neues Profil» / «Profil bearbeiten»** (`ProfileForm.tsx:150-240`) mit den Feldern:
  - «Name» (Pflicht, höchstens 60 Zeichen).
  - «Altersfreigabe» («Titel mit höherer Freigabe bleiben für dieses Profil unsichtbar.», höchstens die Grenze des
    Kontos).
  - «Sprache» (Oberfläche).
  - «Sprache der Beschreibungen» (nur wenn es mehrere gibt, «Wie Oberfläche» oder eine verfügbare Sprache).
  - «Farbe».
  - «Bibliotheken» (Alle oder Auswahl).
  - Feldsatz «Wiedergabe» mit Tonsprachen in Rangfolge, Untertitelregel und Untertitelsprachen (`PlaybackPrefsFields`).
  - Feldsatz «Wiedergabe-Limits»: «Höchste Qualität», «Bandbreite», «Gleichzeitige Streams» und **«Schauzeit pro Tag
    (Minuten)»**, dazu bei Bedarf «Das Konto begrenzt dieses Profil: …».
  - Nur beim Anlegen: «PIN (optional)».
  - Knöpfe «Abbrechen» und «Profil anlegen» bzw. «Speichern».
- Dialog «PIN für {{name}}» (`PinSetDialog.tsx`): «Neue PIN» («4 bis 8 Ziffern. Ohne die PIN lässt sich das Profil
  nicht öffnen.»), Knopf «PIN speichern».
- **Offline-PIN:** Die Operationen `setOfflinePin`/`removeOfflinePin` (`PUT/DELETE /profiles/{id}/offline-pin`,
  `account`) gibt es (`api/openapi.yaml:700,738`). In `web/src` fand ich **keinen Aufruf**, nur die Prüfung im
  Offline-PIN-Dialog (`features/offline/OfflinePinDialog.tsx`). [gelesen, Suche nach `offline-pin`]

### 4.3 `/settings/playback`: «Einstellungen» (`PlaybackSettingsPage.tsx`)
- Hinweis «Ton und Untertitel für das Profil {{name}}, auf jedem Gerät.»
- Abschnitt «Sprachen und Untertitel»: dieselben Felder wie im Profil, Knopf «Speichern» (`putMyPlaybackPrefs`, `scoped`).
- Abschnitt «Serienmerker»: Knopf «Serienmerker zurücksetzen» mit Bestätigung (`resetMySeriesTracks`).
- Abschnitt «Benachrichtigungen» mit einem Link «Zu den Benachrichtigungskanälen» (`:116-121`).

### 4.4 `/settings/notifications`: «Benachrichtigungskanäle»
Siehe 7.1.

### 4.5 «Mein Konto» (E-Mail und Passwort)
- **Ein Konto ohne Verwaltung hat in der Web-App keinen Ort, um E-Mail oder Passwort zu ändern.** `updateAccount`
  (`PUT /auth/account`, `account`) ruft im Web nur `features/admin/accounts/OwnAccountForm.tsx:73` auf, und das steht
  nur auf der Verwaltungsseite «Konten» (nur für Administratoren). [gelesen, Suche nach `'/auth/account'`]
- Eine Route `/settings/account` oder Ähnliches gibt es nicht (Routen in `web/src/app/router.tsx:613-640`).

---

## 5. Benachrichtigungssystem

### 5.1 Ereignisarten [gelesen]

**Kanal-Ereignisse** (`model.Event*`, `server/internal/model/notify.go:42-47`; Vertrag `EventType`, `api/openapi.yaml:18835-18842`;
Liste `eventNames`, `server/internal/notify/channels.go:21-24`):

| Ereignis | Erzeuger | Empfänger | Beleg |
|---|---|---|---|
| `new_content` | `notify.Service.Dispatch` für jeden neuen Eingangseintrag (neue Folge oder Podcast-Episode eines Titels auf der Merkliste); Haken `library.WithNewEpisodes` und `metadata.WithNewEpisodes` | Kanäle des **Kontos** des Profils, mit dem Präfix `[Profilname]` | `notify/dispatch.go:38-59`, `cmd/slouchcat/main.go:341,361` |
| `errors` | `JobFailed` (jobs, letzter Versuch), `ScanAborted`, `RootRemoved`, `RootContentChanged` (library), `UpdateInstallFailed` (admin) | alle Konten mit `role = 'admin'` | `notify/events.go:37-124`, `queries/system/notify_channels.sql:166-176` |
| `invitations` | `InvitationAccepted` (auth) | das einladende Konto | `notify/events.go:21-35` |
| `announcements` | `PublishAnnouncement` → `sendAnnouncement` | alle Konten, die das Ereignis gewählt haben | `notify/announce.go:132-158`, `notify_channels.sql:149-155` |

`acquisition` (Beschaffung) ist mit M9 entfallen (`docs/adr/0009-…md:270-278`, Migration 00018).

**In-App-Eingang:** nur die Art `new_episode` (`KindNewEpisode`, `notify/notify.go:37-38`). Datenbank:
`CHECK (kind IN ('new_episode'))` (`00015_discover.sql:332`). Vertrag: `Notification.kind enum: [new_episode]`
(`api/openapi.yaml:16688-16690`). Ankündigungen laufen in der App **nicht** über den Eingang, sondern über
`announcements`/`announcement_acks` und einen Startdialog (`notify/announce.go:173-205`, `features/notifications/AnnouncementDialog.tsx`).

### 5.2 Kanäle [gelesen]

In-App (immer an, kein Kanal), **E-Mail**, **Web-Push** und **Telegram** (`model/notify.go:34-39`; Vertrag `Channel`,
`api/openapi.yaml:18830-18833`: «the in-app inbox is always on and no channel»). Weitere Kanäle gibt es nicht.

### 5.3 Dispatcher und Zustellung [gelesen]

1. Ein Erzeuger ruft `Emit(ev)` auf (`notify/dispatch.go:74-121`). Ohne Queue tut `Emit` nichts. Ein unbekannter
   Ereignisname ergibt einen Fehler.
2. Nur bei `errors`: Dedupe über `DeliveryRecentExists` je `event` und `source` in 60 Minuten, serialisiert mit
   `errorsMu` (`:81-97`).
3. Empfänger bestimmen (`targets`, `:124-156`): eingeschaltete Kanäle (`enabled = 1`), deren Ereignisliste den Namen
   enthält (`instr(events, '"name"')`, `notify_channels.sql:149-176`).
4. Betreff: Bei `ProfileID != 0` kommt `"[" + Profilname + "] "` davor (`:102-109`).
5. Je Ziel wird übersprungen, wenn der Server den Kanal nicht eingerichtet hat oder bei Telegram keine Chat-ID da ist
   (`:111-118`). Sonst schreibt `queueDelivery` eine Zeile `deliveries` (Betreff höchstens 200, Text höchstens 4000
   Zeichen) und einen Job `notify.deliver` (Priorität 3, **5 Versuche**, Idempotenzschlüssel `notify.deliver:<id>`)
   (`:158-180`, Konstanten `:18-28`). Scheitert das Einreihen, wird die Zeile `failed` («not queued»).
6. Der Job `deliver` (`notify/deliver.go:50-110`) läuft in einem eigenen Pool `PoolNotify` (`:41-43`). Er lädt die
   Zeile, beansprucht sie (`DeliveryClaim`) und prüft, ob der Kanal noch an ist. Dann sendet er (`sendOn`, `:123-150`):
   - **E-Mail** an `config.address` oder sonst die Konto-Adresse, mit Text- und HTML-Teil und dem Link unten (`texts.go:75-93`).
   - **Telegram** an `config.chat_id`.
   - **Push** an **alle** `push_subscriptions` des Kontos (`:152-206`). 404/410 löscht ein Abo. Es gilt als Erfolg,
     wenn mindestens ein Abo die Nachricht nahm.
7. Ergebnis: `sent`, oder `skipped` mit Grund (Kanal aus, nicht eingerichtet, kein Empfänger), oder Fehler. Bei einem
   Fehler bleibt die Zeile `queued` bis zum letzten Versuch, dann `failed`. Die Queue wiederholt mit Backoff. Ein
   Administrator kann den Job neu starten, dann wird `failed` wieder `queued` (`:62-70`).
8. Ein Fehlschlag von `notify.deliver` löst **kein** `errors`-Ereignis aus (keine Schleife, `notify/events.go:40-43`).
9. **Betreff und Text werden beim `Emit` festgelegt.** Beim Senden prüft der Server die Sichtbarkeit nicht erneut.
   `Event.ItemID` wird gesetzt (`dispatch.go:57`), aber nicht in `deliveries` gespeichert (`DeliveryInsert`,
   `notify_channels.sql:235-238`). [gelesen]
10. Aufräumen: fertige Zustellungen nach 30 Tagen (`notify/telegram_link.go:28,133`), gelesene Eingangseinträge
    nach 90 Tagen (`server/internal/offline/offline.go:95`, `offline/cleanup.go:111`). Ungelesene bleiben.

### 5.4 Tabellen [gelesen]

- `notifications` (`00015_discover.sql:329-341`; Index `notifications_parent` aus `00055…:57`): `profile_id`, `kind`,
  `item_id` (NOT NULL, FK items, CASCADE), `parent_id`, `created_at`, `read_at`, **UNIQUE (profile_id, kind, item_id)**.
- `account_channels` (`00016_admin.sql:27-35`): PK (account_id, channel), `enabled`, `events` (JSON-Array), `config`
  (JSON: `address`, `chat_id`; `notify/channels.go:28-33`).
- `telegram_links` (`00016…:37-42`): Code mit 6 Zeichen, 15 Minuten gültig (`telegram_link.go:22`).
- `push_subscriptions` (`00016…:44-58`): `account_id`, `device_id` (FK devices, SET NULL), `endpoint` UNIQUE, Schlüssel,
  `device_name`, `failures`.
- `deliveries` (zuletzt `00018_drop_acquisition_channel.sql:21-36`): `account_id`, `profile_id` (SET NULL), `channel`,
  **`event CHECK IN ('new_content','errors','invitations','announcements')`**, `source`, `subject`, `body`, `link`,
  `state` (`queued|sent|failed|skipped`), `attempts`, `last_error`.
- `announcements`, `announcement_acks` (`00016…:80-96`).

### 5.5 Dedupe [gelesen]

- Eingang: `INSERT OR IGNORE` mit UNIQUE (profile_id, kind, item_id) (`queries/system/notify.sql:25-28`). Bei 0
  betroffenen Zeilen wird nichts versandt (`notify/notify.go:203-205`). Eine Folge meldet sich also höchstens einmal je
  Profil.
- Je Aufruf meldet eine Serie oder ein Podcast höchstens **50** Folgen (`MaxPerParent`, `notify.go:124-126,167-169`).
- Kanäle: nur `errors` über `source` und 60 Minuten. Für die anderen Ereignisse gibt es keine Dedupe in `deliveries`;
  das übernimmt die Eingangs-Dedupe.
- Bekannt offen: eine Mehrfachzustellung, wenn das Einreihen nach dem Einfügen scheitert (`docs/adr/0009-…md:267`),
  und ein doppelter Push bei einer Wiederholung nach einem Teilerfolg (`deliver.go:121-122`).

### 5.6 Vorlagen, Texte, i18n [gelesen]

- Die Servertexte sind eingebettete i18next-JSON-Dateien, 23 Sprachen in `server/internal/notify/locales/*.json`
  (`notify/texts.go:12-33`). Kette: Sprache, dann de für de-DE, dann en.
- Die Sprache ist die **des Profils** bei Profil-Ereignissen und die **des Hauptprofils** bei Konto-Ereignissen
  (`texts.go:13-15`; `NotifyAccount`, `notify_channels.sql:97-104`). `errors` wird in der Sprache des Besitzers
  geschrieben (`events.go:142-152`).
- Schlüssel für neue Inhalte: `new_episode` = «Neue Folge: {{title}}», `new_podcast_episode` = «Neue Episode: {{title}}»
  (`notify/locales/de.json`). Text: «SxxEyy · Titel» (`dispatch.go:51-54`).
- Ankündigungen werden nicht übersetzt; es sind die Worte des Admins (`texts.go:16`).
- Die App-Texte des Eingangs stehen unter `notifications.*` in `web/src/locales/de.json`, z. B.
  `notifications.newEpisode` = «Neue Folge von {{parent}}».

### 5.7 Deep-Links [gelesen]

- `new_content`: `Link: "/items/" + episodeID` (`dispatch.go:57`). Bei E-Mail und Push wird daraus mit der
  **öffentlichen Adresse** (`publicUrl`) eine absolute URL (`texts.go:61-72`). Ohne `publicUrl` steht in der E-Mail
  kein Link, Push nimmt den relativen Pfad oder `/` (`deliver.go:164-170`).
- Andere Links: Einladung `/admin/access` (alt, leitet um; `events.go:33`), Fehler `/admin` bzw. `/admin/libraries`,
  Ankündigung `/`.
- In der App führt ein Eingangseintrag auf `paths.item(entry.item.id)`, also die Folge (`features/discover/NotificationsPage.tsx`, `InboxEntry href`).

---

## 6. Empfängerlogik Konto gegenüber Profil [gelesen]

- **In-App-Einträge gehören dem Profil** (`notifications.profile_id`). Lesen und Gelesen-Markieren sind `scoped`
  (`api/openapi.yaml:5954,5989,6015`) und laufen über `visible_items` (`queries/system/notify.sql:30-60`). Ein Eintrag,
  dessen Titel das Profil nicht mehr sehen darf, verschwindet aus Liste und Zähler und kommt unverändert zurück, wenn
  die Sichtbarkeit zurückkommt.
- **E-Mail, Push und Telegram gehören dem Konto** (`account_channels`, PK account_id und channel). Push-Abos hängen an
  Konto **und** Gerät (`push_subscriptions.device_id`), gesendet wird aber an **alle** Abos des Kontos
  (`deliver.go:152-158`). Ein Kanal je Profil wurde verworfen, weil ein Kind sonst eigene Zugangsdaten verwalten müsste
  (`docs/adr/0009-…md:233-235`).
- **Profilbezogene Meldung extern (Regel 5.5):** Die Meldung geht an die Kanäle des Kontos mit dem Präfix
  `[Profilname]`, vorher prüft `notify` die Sichtbarkeit für **dieses** Profil (`docs/planning/03-plan-v1.md:308-310`;
  `docs/adr/0009-…md:246`; Code `notify.go:186-220`, `dispatch.go:38-59,102-109`).
- **Ändern der Kanäle** (Kanal ein/aus, Ereignisse, Adresse, Push-Abo, Telegram, Testnachricht) darf nur eine Sitzung
  ohne Profil oder mit Hauptprofil (`requireManager`, `notify/channels.go:100-109`). Lesen darf jedes Profil. Grund:
  «ein Kinderprofil könnte sonst die Benachrichtigungen des Kontos (mit Titeln anderer Profile) auf sein Gerät
  umleiten» (`04-entscheidungen.md:50`).
- **`errors`** dürfen nur Administratoren wählen (`ErrEventNotAllowed`, `channels.go:206-208`). Die App blendet die
  Wahl aus (`features/notifications/channels/ChannelPanel.tsx:104`).
- **Datenschutzregeln, die ich fand:** nur die Sichtbarkeitsprüfung vor dem Versand und die Hauptprofil-Regel beim
  Ändern. **Nicht gefunden:** eine Regel für PIN-geschützte Profile, eine Regel «Titel extern nur, wenn …», eine
  Unterdrückung für Kinderprofile, eine Regel je Profil im Konto. In `server/internal/notify/*.go` kommen PIN und
  Altersstufe nicht vor. [gelesen, grep]
- **Folge für den Datenschutz** [Vermutung, abgeleitet]: Eine Meldung zum Titel eines Erwachsenenprofils erreicht per
  Push jedes Gerät des Kontos mit einem Abo, also auch das Tablet des Kindes. Im Betreff stehen Profilname und
  Serientitel. So ist es heute für `new_content` gewollt (5.5).

---

## 7. Einstellbarkeit

### 7.1 Benutzer: `/settings/notifications` «Benachrichtigungskanäle» [gelesen]

`features/notifications/ChannelSettingsPage.tsx`, `channels/ChannelPanel.tsx`, `channels/TelegramLink.tsx`,
`channels/PushDevices.tsx`; Abfragen `features/notifications/queries.ts:14-48`.
- Die Einstellungen gelten für das **ganze Konto** (`settings.notifications.enabledHelp` «Gilt für das ganze Konto mit
  allen Profilen.»). Ohne Hauptprofil sieht man sie ohne Bedienelemente, mit dem Hinweis «Nur im Hauptprofil» und dem
  Knopf «Profil wechseln».
- **Je Kanal ein Block** (E-Mail, Telegram, Web-Push) mit dem Zustand «Aktiv», «Aus» oder «Nicht eingerichtet»:
  - Schalter «Aktiv».
  - **Ereignisse je Kanal** (Häkchen): «Neue Inhalte», «Fehler» (nur Administratoren), «Einladungen», «Ankündigungen».
    Hilfetext «Wofür eine Nachricht kommt.» Die Matrix Ereignis × Kanal liegt also in `account_channels.events` je Kanal.
  - E-Mail: Feld «E-Mail-Adresse» («Standard ist die Adresse deines Kontos.»).
  - Telegram: «Verknüpfen», «Code erneuern», «Verknüpfung lösen», «In Telegram öffnen», Anleitung «Sende dem Bot @{{bot}}
    die Nachricht /start {{code}}», Ablaufzeit, Abfrage jede Sekunde. Bestätigungen «Verknüpfung lösen?» und «Code verwerfen?».
  - Web-Push: «Angemeldete Geräte» mit «Dieses Gerät», «Angemeldet am …», «Entfernen», «Auf diesem Gerät aktivieren»,
    dazu Hinweise zu iOS, blockierten Mitteilungen und fehlendem HTTPS.
  - Knöpfe «Speichern» und «Testnachricht». Fehlt ein Kanal auf dem Server: «Der Hauptbenutzer richtet diesen Kanal in
    der Verwaltung unter Integrationen ein.» mit dem Knopf «Integrationen öffnen».
- Standard bei einem neuen Konto (`ChannelEnsureDefaults`, `notify_channels.sql:72-83`): Für `role = 'user'` ist
  E-Mail an mit `["announcements"]`, Push und Telegram sind aus mit `["announcements"]`. Für einen Administrator sind
  alle Kanäle aus mit `["new_content","errors","invitations","announcements"]`.
  **Ein gewöhnliches Konto bekommt `new_content` also ab Werk auf keinem Kanal.** [gelesen]
- **Es gibt keinen Schalter je Profil** (z. B. «für Profil X keine externen Meldungen») und keinen Schalter für den
  In-App-Eingang.

### 7.2 Administrator: Kanäle des Servers, Seite «Integrationen» `/admin/integrations` [gelesen]

`features/admin/IntegrationsPage.tsx:273-291`. Die Blöcke stehen auch im Einrichtungsassistenten
(`features/setup/ServicesStep.tsx:149-182`). Vertrag `AdminSettings`/`AdminSettingsUpdate` (`api/openapi.yaml:18382-18560`),
`getAdminSettings`/`updateAdminSettings`/`sendTestEmail`/`rotatePushKeys`, alle `admin`.
- **«E-Mail-Server»** (`integrations/EmailForm.tsx`): «Server und Port» (Host, «Port»), «Benutzer», «Passwort»
  (verschlüsselt, «Leer lassen, um das gespeicherte Passwort zu behalten.»), «Absender» («Slouchcat <media@example.ch>»),
  «Verschlüsselung» (STARTTLS, TLS, Keine), Knopf «Testmail senden» («Die Testmail geht an {{email}}.»), «Speichern»,
  «Verwerfen» und ein Link «Zu den Benachrichtigungskanälen». Werte aus `deploy/.env` gewinnen und sind gesperrt
  (`admin.integrations.envHint`, `env_wins`).
- **«Telegram-Bot»** (`integrations/TelegramForm.tsx`): «Bot» («Bot: @{{name}}» oder «Kein Bot eingerichtet.»),
  «Bot-Token» (Prüfung bei Telegram, «Ein neues Token ersetzt das gespeicherte.»).
- **«Web-Push»** (`integrations/PushBlock.tsx`): «Schlüsselpaar vorhanden.» oder «Noch kein Schlüsselpaar …», «Öffentlicher
  Schlüssel», Knopf «Schlüssel neu erzeugen» mit der Bestätigung, dass alle Geräte ihr Abo verlieren.
- **«Öffentliche Adresse»** (`integrations/ServerBlock.tsx:93-117`): «Adresse» («Zum Beispiel https://media.example.ch»).
  Die Links in E-Mails und Nachrichten hängen davon ab.
- Ausserdem auf der Seite: Schlüssel der Anbieter, Update-Prüfung, Transcode-Wahl, Webhooks (Radarr/Sonarr).

### 7.3 Verwaltung «Ankündigungen» `/admin/announcements` [gelesen, Texte aus `admin.announcements.*`]

`features/admin/AnnouncementsPage.tsx`, `announcements/AnnouncementDetail.tsx`.
- Abschnitte «Entwürfe N» und «Veröffentlicht N». Je Eintrag: «Erstellt am», «Veröffentlicht», «Gelesen: N von M
  Profilen» bzw. «Noch nicht verschickt».
- Knopf «Neue Ankündigung». Formular: «Titel» (höchstens 120, mit Zähler), «Text» (höchstens 4000,
  Zeilenumbrüche bleiben), «Veröffentlichung»: «Sofort veröffentlichen», Knopf «Als Entwurf speichern». Vorschau «So
  sehen es die Profile».
- Aktionen: «Veröffentlichen» (Bestätigung, danach nicht mehr änderbar), «Löschen» (schon verschickte Nachrichten
  bleiben) und «Verwerfen».
- Wirkung: Jedes Profil sieht die Ankündigung beim nächsten Start (Dialog mit «Verstanden» und «Später»,
  `announcements.*`), und sie geht an die Kanäle jedes Kontos, das `announcements` gewählt hat.

### 7.4 Verwaltung «Benachrichtigungen» `/admin/notifications`
Das ist die Kanalseite des eigenen Kontos in der Verwaltungsleiste (siehe 3.3), keine eigene Admin-Logik. Es gibt
**keine** Verwaltungsseite mit einer Übersicht über Zustellungen (`deliveries`) aller Konten. Nur `admin/accounts.go:29`
liest die Kanal-Zusammenfassung für die Spalte «Kanäle». Fehlgeschlagene `notify.deliver`-Jobs sieht man auf der
Seite «Hintergrundjobs». [gelesen; Seite Hintergrundjobs nicht im Einzelnen geprüft]

---

## 8. `new_content` als Vorbild

### 8.1 Erzeuger und Zeitpunkt [gelesen]

- **Serienfolgen:** In `library` sammelt `batchWriter` neu angelegte Folgen (`server/internal/library/write.go:576-598`).
  Folgen, die mit Serie oder Staffel entfernt wurden, zählen nicht (`:595-597`). Nach dem **Commit** des Scan-Batches
  ruft `announceEpisodes` den Haken auf, losgelöst vom Scan-Kontext und mit 30 s Zeitlimit (`write.go:259-266,280-295`).
  Das gilt für volle und für Pfad-Scans, also auch für den Radarr/Sonarr-Webhook (`library/service.go:227-232`).
- **Podcast-Folgen im Ordner:** nur bei einem Podcast, den es vor diesem Scan schon gab (`library/podcast_folder.go:280-300`).
- **Podcast-Feed:** `metadata` nach der Transaktion des Feed-Abrufs (`metadata/podcast.go:556,840-856`).
- Der Zeitpunkt ist also **direkt nach dem Scan**, **vor** dem Metadaten-Match. Eine neue Folge erbt beim Einfügen die
  Freigabe der Serie über den Trigger `items_rating_insert` (`00062_minimum_age_columns.sql:53-62`). Deshalb ist sie
  sofort für dieselben Profile sichtbar wie die Serie.
- **Einen Haken für neue Filme oder neue Serien gibt es nicht.** Gemeldet werden nur Folgen von Titeln, die schon auf
  der Merkliste stehen.

### 8.2 Empfänger und Sichtbarkeit [gelesen]

- `NotifyHasFollowed`: ohne gefolgte Serie oder gefolgten Podcast wird nichts weiter gelesen (`notify.sql:4-9`, `notify.go:142-148`).
- `NotifyFollowerTargets`: Folge → Staffel → Serie (oder Podcast) → `watchlist` → `profiles` (`notify.sql:11-23`).
- Je Profil: `vis.Visible(visibility.Profile(p), episodeID)`. Bei `ErrNotFound` gibt es weder Eintrag noch Nachricht
  (`notify.go:186-194`, `visibility/playback.go:18-25`). Die Prüfung läuft **einmal**, beim Erzeugen.
- Dann `NotifyInsert` und, nur bei einer neuen Zeile, `Dispatch` → `Emit(new_content, AccountID, ProfileID)`
  (`notify.go:196-219`, `dispatch.go:38-59`). Ein Fehler des Dispatchers wird geloggt, der Eintrag bleibt.

### 8.3 Doppelmeldungen [gelesen]
UNIQUE (profile_id, kind, item_id) mit `INSERT OR IGNORE`, ein doppeltes id im Aufruf meldet einmal (`notify.go:165`),
höchstens 50 je Serie und Aufruf.

---

## 9. Benachrichtigungs-UI für Zuschauer [gelesen]

- **Glocke** im Kopf: `ui/AppHeader.tsx:190-206` (`InboxLink`, Symbol `bell`, `NotificationBadge` in der Profilfarbe
  mit `{{max}}+`, `ui/NotificationBadge.tsx`). Der Zähler kommt aus `GET /notifications?limit=1` alle **30 s**
  (`features/discover/queries.ts:106-115`). Server-Sent Events und Push in die App gibt es nicht.
- **Seite** `/notifications` (`web/src/app/router.tsx:732-737`, `features/discover/NotificationsPage.tsx`):
  - Titel «Benachrichtigungen» mit «N ungelesen».
  - Knöpfe «Zurück», «Kanäle einstellen» (→ `/settings/notifications`) und «Alle als gelesen markieren» (nur bei
    ungelesenen Einträgen).
  - Virtuelle Liste aus `InboxEntry`: Titel «S01E03 · Pilot», Kontext «Neue Folge von {{parent}}», Zeit,
    ungelesen-Markierung. Ein Klick öffnet `/items/{id}` und markiert den Eintrag gelesen.
  - Leer: «Keine Benachrichtigungen» mit «Setze Serien oder Podcasts auf deine Merkliste, dann erfährst du hier von
    neuen Folgen.»
- **Die App setzt die Art fest voraus:** `entryContext`/`entryTitle` gehen immer von «Neue Folge» und Folgennummer aus.
  Eine neue Art braucht eigene Darstellung und Texte.
- Mutationen: `useMarkReadMutation` (PUT `/notifications/{id}/read`) und `useMarkAllReadMutation` (POST
  `/notifications/read-all`) (`discover/queries.ts:258-276`).
- **Ankündigungen:** Startdialog `features/notifications/AnnouncementDialog.tsx` und `Announcements.tsx`, Daten aus
  `GET /announcements` und `POST /announcements/{id}/ack`.

---

## 10. Datenschutz zwischen Profilen [gelesen]

**Profil gegenüber Profil im selben Konto:**
- Jedes Profil sieht die **Namen** (Farbe, Bild, PIN ja/nein, Hauptprofil) der Profile seines Kontos in der Profilwahl
  (`listMyProfiles`, `account`, `auth/profiles.go:70-74`).
- Einstellungen, Schauzeit-Bericht und PINs aller Profile sieht nur das **Hauptprofil** (`ListProfiles` mit
  `requirePrimary`, `auth/profiles.go:79-84`; `WatchReport`, `playback/limits.go:302-311`).
- Verlauf, Fortschritt, Merkliste, «Gesehen» und Eingang sind `scoped` und gehören nur dem aktiven Profil. Ein
  Endpunkt, der einem Profil den Verlauf eines anderen zeigt, ist mir nicht begegnet. [nicht vollständig geprüft]
- **Kanalnachrichten** des ganzen Kontos tragen `[Profilname] Titel`. Wer die E-Mail, das Telegram oder ein Gerät mit
  Push-Abo des Kontos hat, sieht die Titel **aller** Profile, die sichtbar waren (siehe 6).

**Profil gegenüber anderen Konten:**
- User-Empfehlungen sind **anonym**: kein «von wem», kein «wie viele». Die Verwaltung zeigt nur Anzahl, Tag und
  Ablauf. Bewusst hingenommen ist, dass sich bei Anzahl 1 über das Aktivitätsprotokoll raten lässt
  (`docs/adr/0028-…md:57-63`). Bewertungen, Kommentare und Nachrichten zwischen Profilen bleiben Nicht-Ziele (`:23-25`).
- Die Profile anderer Konten sind «Sache des Kontos» (`x-visibility: account`, `AccountDetail.tsx:99-102`).

**Was der Administrator sieht:**
- «Konten»: E-Mail, Rolle, Grenzen, **Profilnamen**, letzter Kontakt und eingeschaltete Kanäle aller Konten, aber
  **nicht** deren Profileinstellungen (`AdminAccount`, `api/openapi.yaml:18752-18796`).
- «Geräte»: Eigentümer «E-Mail · Profil».
- «Statistiken»: Schauzeit und Wiedergaben **je Profil mit Konto-E-Mail** und die meistgesehenen Titel
  (`queries/system/admin_stats.sql:1-2,65-83`: «administration sees every profile»).
- «Letzte Aktivität»: `playback_started/completed/stopped` mit Profil und Titel, 30 Tage, höchstens 2000 Zeilen
  (`admin/activity.go:16-47`, `00031_activity_events.sql:20-30`).
- «Streams»: laufende Wiedergaben. Beim Löschen eines Titels liefert `Problem.playingProfiles` die Profile, die gerade
  abspielen (`docs/adr/0032-…md:11`).

---

## Folgen für das Wunsch-Konzept

**Einschränkungen (zwingend):**
1. **Das Nicht-Ziel aufheben.** Wünschen, Seerr-Anbindung und «Benachrichtigung bei Verfügbarkeit» sind Nicht-Ziele
   (Spec `:46,66-67`, CLAUDE.md Regel 11, Frage 10). Das Konzept braucht eine Frage an Marco und eine ADR, die die
   Ausnahme genau zuschneidet, nach dem Muster von ADR 0028 und ADR 0032. Die freien Nummern I-3 und I-4 und der alte
   Vorschlag aus Frage 10 («Hauptkonto ab FSK 12, Gäste nicht, Seerr-Benutzer je Konto») sind Anknüpfungspunkte.
   Die Spec selbst wird nach `docs/spec/README.md:6` nicht umgeschrieben (Vorgehen wie bei ADR 0014).
2. **Für das Recht zu wünschen gibt es kein Modell.** Weder Konto noch Profil haben einen Schalter. Neu nötig wäre
   eine Spalte oder Tabelle. Das Muster «Grenze je Konto, Feinwahl je Profil» liefern `max_rating` und `all_libraries`.
   Nur `updateAccountAccess` (`admin`) darf Kontogrenzen schreiben; das ist der Riegel «niemand hebt seine eigenen
   Grenzen an» (ADR 0014 `:161`). Ein Wunschrecht am Profil dürfte dann nur das Hauptprofil setzen.
3. **Jugendschutz ohne `items`-Zeile.** `visible_items` prüft nur Titel in der Bibliothek. Ein Wunsch nach einem Titel,
   der noch fehlt, braucht eine **Serverregel** für die TMDB-Freigabe gegen `min(profile.max_rating, account.max_rating)`
   (Regel 12; Mindestalter nach ADR 0034, unbekannt = 18 nach AN-1). Wo diese Regel wohnen darf, ist offen. Die Logik
   für Freigaben liegt heute in `model/ratings.go`, `metadata/ratings.go` und `visibility`.
   [Vermutung] Die Suche nach wünschbaren Titeln darf einem Kinderprofil keine Titel über seiner Stufe zeigen, auch
   nicht als «nicht erlaubt».
4. **Eine neue Eingangsart braucht eine Migration.** `notifications.kind CHECK (kind IN ('new_episode'))`
   (`00015…:332`) und der Vertrag `Notification.kind enum: [new_episode]` (`openapi.yaml:16688-16690`) müssen
   erweitert werden. Die Tabelle wird nach Vorlage neu gebaut (Regel 5). Das Vertragsenum zu erweitern ist für
   Fremd-Clients heikel; ob das brechend ist, wäre zu entscheiden (ADR 0014 `:149` als Präzedenz).
5. **Ein neues Kanal-Ereignis braucht eine Migration.** `deliveries.event CHECK` (`00018…:26`) wird neu gebaut, wie
   in 00018. Dazu `model.Event*`, `eventNames` (`channels.go:23`), `EventType` im Vertrag, die Standardwerte in
   `ChannelEnsureDefaults` und eine Rückfüllung der bestehenden Konten, Texte in 23 Server-Sprachen und App-Texte
   (`settings.notifications.events.*`). Die Alternative ist, `new_content` wiederzuverwenden; dann fehlt aber ein
   eigener Schalter.
6. **Die Standardkanäle fangen nichts auf.** Ein gewöhnliches Konto hat `new_content` ab Werk auf keinem Kanal (7.1).
   Ein Wunsch-Ereignis müsste beim Anlegen oder Rückfüllen bewusst eingeschaltet werden, sonst kommt nur die
   In-App-Meldung an.
7. **Externe Kanäle gehören dem Konto.** Eine Meldung «Dein Wunsch ist da» geht an E-Mail, Telegram und **alle**
   Push-Geräte des Kontos mit `[Profilname]`. Der Titel eines Erwachsenenprofils erreicht so auch das Gerät des Kindes.
   Eine Regel je Profil gibt es nicht (6). Das Konzept muss entscheiden, ob 5.5 auch für Wünsche reicht.
8. **Die Sichtbarkeit wird nur einmal geprüft.** `new_content` prüft beim Erzeugen. Ein neu gescannter **Film** hat
   bis zum Metadaten-Match `effective_rating = NULL`, zählt also als 18 und ist für Profile unter 18 unsichtbar
   [gelesen: View `COALESCE(…,18)`, Trigger `items_rating_insert`; die Folge für Filme ist **Vermutung**].
   Löst der Haken schon nach dem Scan aus, fällt die Meldung für ein Kinderprofil still weg. Der Auslöser muss nach
   Match und Freigabe kommen, oder er muss später erneut prüfen.
9. **Für neue Filme und Serien gibt es keinen Haken.** Gemeldet werden heute nur neue **Folgen** von Titeln auf der
   Merkliste (8.1). Für «gewünschter Film ist da» braucht es einen neuen Auslöser, z. B. nach `metadata.match` mit
   TMDB-ID oder TVDB-ID als Schlüssel zum Seerr-Wunsch. [Vermutung zum Ort]
10. **Betreff und Text werden beim Emit festgelegt** und beim Senden nicht neu geprüft (5.3 Punkt 9). Eine
    Sichtbarkeitsänderung zwischen Emit und Versand erreicht die externen Kanäle nicht. Das ist heute so hingenommen.
11. **Konten ohne Verwaltung haben keine «Mein Konto»-Seite** (4.5). Muss ein Wunsch einer Seerr-Identität oder einer
    E-Mail zugeordnet werden, hat ein gewöhnliches Konto heute keinen Ort dafür.
12. **Regeln der Stufen:** `scoped` für Wunsch anlegen und eigene Wünsche lesen; `account` mit `requirePrimary` für
    «Wünsche aller Profile meines Kontos»; `admin` für die Verwaltung aller Wünsche und die Seerr-Integration. Jede
    Operation braucht einen Eintrag im Vertragstest. Nicht sichtbar heisst 404 (Regel 2, ADR 0028 Punkt 6).

**Anknüpfungspunkte:**
- `notify.Service` mit `Emit`, `Dispatch`, `queueDelivery` und `deliver` ist allgemein. Ein neuer Erzeuger braucht
  nur ein `model.Event` mit `AccountID`, `ProfileID` und `Link`.
- Das Muster «Eintrag je Profil, UNIQUE (profile_id, kind, item_id), dann Dispatch» aus `notifyProfile` passt für
  «Wunsch verfügbar», sobald es ein `item_id` gibt. Deep-Link: `/items/{id}`.
- Ein Recht wie bei ADR 0028 (`canRecommend` am `ItemDetail`, Prüfung im Server, 404 bei Nein) passt als Vorbild für
  ein Feld `canRequest` in `SessionInfo` oder in der Suche.
- Die Verwaltungsseite «Konten», Formular «Zugriff» (`AccessForm.tsx`), ist der Ort für ein Wunschrecht je Konto. Für
  je Profil ist es der Profildialog (`ProfileForm.tsx`). Im Mockup dürfen die Felder aus 3.1 und 4.2 nicht wegfallen.
- Der Ort für Seerr-Zugangsdaten wäre die Seite «Integrationen» (Schlüssel verschlüsselt in `settings`, `env_wins`,
  Prüfknopf nach dem Muster `checkProviderKey`). Webhooks sind nur für Radarr und Sonarr vorgesehen (Frage 10 b).
- Die Ankündigungen zeigen, wie ein Ereignis an alle Konten geht und je Profil quittiert wird.

---

## Unsicher / nicht geprüft

- Ob Telegram- und E-Mail-Texte für `new_content` noch an anderer Stelle (Vorlagen) gebaut werden. Gelesen habe ich
  nur `texts.go` und `notify/locales/de.json`.
- `DevicesPage.tsx`, `AnnouncementsPage.tsx` und `ChannelPanel.tsx`/`TelegramLink.tsx`/`PushDevices.tsx` habe ich
  über die Text-Schlüssel und Kommentare erfasst, nicht Zeile für Zeile. Einzelne Zustandsmeldungen können fehlen.
- Die Spalte `profiles.image`: In der Profilverwaltung des Webs fand ich kein Feld dafür. Nicht weiter geprüft.
- Offline-PIN setzen: Im Web fand ich keine Oberfläche (Suche nach `offline-pin`). Ein anderer Weg ist möglich, aber
  nicht gefunden.
- Ob ein neu gescannter Film vor dem Match tatsächlich `effective_rating = NULL` hat. Das gilt, wenn keine NFO eine
  Freigabe liefert; der NFO-Weg in `library/nfo.go` ist nicht im Einzelnen geprüft.
- Wie die Queue die Wiederholungen von `notify.deliver` genau verzögert (Backoff in `internal/jobs`). Nicht gelesen.
- Konzept 06 (`docs/planning/06-benutzerverwaltung.md`) habe ich nur über die Gliederung und ADR 0014 erfasst.
- Den Seerr-Teil (API, Benutzerzuordnung, Webhook von Jellyseerr) habe ich nicht untersucht; er gehört nicht zu diesem Teil.
