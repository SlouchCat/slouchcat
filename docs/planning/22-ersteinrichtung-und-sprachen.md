# Konzept 22: Ersteinrichtung und Sprachen

- **Status: Fassung 1 (14. November 2026).** Marco hat am 14. November 2026 sechs Fragen
  beantwortet (**F-1 bis F-6**, Abschnitt 1), eine davon am klickbaren Mockup (Anhang A). Offen
  sind die Punkte **O-1 bis O-16** (Abschnitt 8), jeder mit Empfehlung; bis zu Marcos Antwort
  gilt die Empfehlung. **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Auslöser:** Marco will einen Installations-Assistenten, der beim ersten Start erscheint und die
  wichtigsten Einstellungen abfragt, und Slouchcat in vielen Sprachen anbieten, «etwa so wie Emby
  und Jellyfin».
- **Verankert gegen:** `main` **`46d41072`** («Merge pull request #158», Release 1.32.0),
  Vertragsfassung **1.32.0** (`api/openapi.yaml:4`), letzte Migration **`00057_trailer_size.sql`**,
  letzte ADR **`0032`**, letzte Frage im Entscheidungsprotokoll **28**. Daneben offen: Zweig
  `claude/k21-stufen4-6` mit Vertrag **1.33.0** und Migration **00058**.
- **Nummern:** Dieses Papier ist **Konzept 22**, reserviert **Frage 29** und **ADR 0033**
  («Oberflächensprachen und Übersetzung», ersetzt ADR 0018 F-1 in einem Punkt, Abschnitt 6).
  Vertragsfassungen und Migrationsnummern nennt das Papier **nicht fest**; vergeben wird in
  Bau-Reihenfolge (goose verlangt lückenlose Nummern, die Regel «Vertrag = Tag» eine aufsteigende
  Version). Abschnitt 7 sagt je Stufe nur, **ob** sie Vertrag und Migration braucht.
- `:Zeile`-Angaben beziehen sich auf `46d41072`; **nach Namen suchen, nicht nach Zeile.** Pfade
  ohne Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App), wo es eindeutig ist.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/` (hier vor allem ADR 0005 Konten und Profile, ADR 0018 Sprachen). Bei Widerspruch
  gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).

> **Dieses Papier ändert keine Zeile Code.** Es hat zwei Teile, die zusammengehören, aber
> getrennt gebaut werden: **Teil A** ersetzt den heutigen Ein-Seiten-Assistenten durch einen
> geführten Assistenten in neun Schritten (Abschnitt 3). **Teil B** macht aus zwei
> Oberflächensprachen dreiundzwanzig (Abschnitt 4). Abschnitt 7 schneidet beides in vier Stufen
> mit je einem Release.

---

## 1. Was Marco will (entschieden am 14. November 2026)

| Nr. | Frage | Antwort |
|---|---|---|
| **F-1** | Welche Variante des Assistenten (Mockup A «Geführt», B «Zwei Wege», C «Pflicht + Karten»)? | **A «Geführt»:** alle Schritte in einer Leiste links, Pflicht sind Sprache, Konto und Bibliothek, die übrigen lassen sich überspringen; dann gilt die Empfehlung. |
| **F-2** | Was fragt der Assistent zusätzlich zu Sprache, Konto, erster Bibliothek und TMDB ab? | **Alles:** Region und Metadaten, Haushalt-Profile, weitere Dienste (TVDB, Fanart, YouTube, E-Mail), Server und Wiedergabe (öffentliche Adresse, Grafikchip, Trickplay, Update-Prüfung). |
| **F-3** | Wie wird Deutsch angeboten? | **Zwei Varianten:** Deutsch (Schweiz) mit ss wie heute und Deutsch (Deutschland) mit ß. |
| **F-4** | Wie viele Oberflächensprachen im ersten Schritt? | **Rund 22**, und zwar «natürlich und sinnvoll übersetzt», nicht wörtlich. |
| **F-5** | Wer übersetzt? | **Claude liefert die erste Fassung, Weblate wird für Korrekturen aus der Community geöffnet.** |
| **F-6** | Schlüssel im Assistenten | Bei jedem Schlüssel steht **der Link, wo man ihn beantragt** (Nachtrag Marco, gleicher Tag). |

Die dreiundzwanzig Sprachen aus F-3 und F-4 stehen in Abschnitt 4.1. Rechts nach links (Arabisch,
Hebräisch, Persisch) ist bewusst nicht dabei (Abschnitt 4.10).

---

## 2. Heute

### 2.1 Der Assistent

- `/setup` ist **eine Seite mit vier Blöcken** (`features/setup/SetupPage.tsx:264-427`):
  1. Hauptkonto
  2. erste Bibliothek (`LibraryFields`)
  3. TMDB-Schlüssel, optional
  4. Systemzustand (`SystemState`), nur Anzeige
- Ein Knopf schickt alles mit einem `POST /setup` (`runSetup`).
- Ob der Assistent erscheint, entscheidet allein `CountAccounts == 0` (`internal/auth/auth.go:135-141`).
  Die Weiche liegt in `app/router.tsx:156-160`.
- `RunSetup` (`internal/admin/admin.go:329-384`) läuft in dieser Reihenfolge:
  1. Er prüft Pfad und Schlüssel.
  2. Er legt Konto und Profil in **einer** Transaktion an (`auth.go:181-245`).
  3. Er speichert den TMDB-Schlüssel.
  4. Er legt die Bibliothek an.
  5. Er meldet an.
- Scheitert die Bibliothek, bleiben Konto und Schlüssel bestehen (`admin.go:371-376`).
- Das erste Profil heisst wie der Teil der E-Mail vor dem @, hat Freigabe 18, die Sprache der
  Einrichtung und keine Farbe (`auth.go:226-230`).
- **Die Sprache wird nicht gefragt.** Sie kommt still aus dem Browser (`SetupPage.tsx:246`,
  `i18n/index.ts:13`).
- Vor einem Konto gibt es nur zwei öffentliche Prüfungen: Pfad (`checkSetupPath`, nur innerhalb
  von `SLOUCHCAT_BROWSE_ROOTS`) und TMDB (`checkSetupTmdbKey`).
  - TVDB, Fanart und YouTube prüft nur `checkProviderKey`. Das ist ein Admin-Endpunkt und prüft nur
    den **gespeicherten** Schlüssel (`internal/admin/settings.go:713-730`).
  - Die Testmail (`sendTestEmail`) gibt es ebenfalls nur für Admins.
- Grenzen: Versuchstopf 10 je Minute und Adresse, gemeinsam mit Anmeldung, PIN und Einladung
  (`internal/auth/limits.go:15-16`). Dazu die CSRF-Kopfzeile `X-Requested-With`.
- Spec A-1 (`docs/spec/01-anforderungen-v1.md:144`) verlangt Konto, erste Bibliothek,
  GPU-Erkennung und Metadaten-Schlüssel. Plan 3.4 (`docs/planning/03-plan-v1.md:158-160`)
  verlangt Warnungen für Netzlaufwerke und das inotify-Limit.
- `docs/INSTALLATION.md:252-271` beschreibt genau diese vier Blöcke. Profile, Freigaben und
  Einladungen kommen dort «später in der Verwaltung».

### 2.2 Wo die Einstellungen heute liegen

| Thema | Endpunkt heute | Gespeichert | Quelle | Anmerkung |
|---|---|---|---|---|
| Oberflächensprache | `createProfile`/`updateProfile` | `profiles.language` | DB | nur `de`/`en`, Abschnitt 2.3 |
| Freigabeland | `updateMetadataSettings` | `metadata.rating_country` | DB | nur DE, AT, CH, GB, US (`openapi.yaml:13324`) |
| Metadaten-Sprachen | `updateMetadataSettings` | `metadata.text_langs`, `metadata.image_langs.<art>` | DB | Vorgabe `de`, `en` (`metadata/settings.go:84-99`) |
| Zeitzone | **keiner** | – | nur `TZ` (`deploy/compose.yaml:42`, Vorgabe Europe/Zurich) | Jobs nutzen sie (`jobs/scheduler.go:146`) |
| Radio-Angebot | `previewRadioOffer`/`applyRadioOffer` | `radio_offer_countries` | DB | Profil-Land: Zeitzone → Freigabeland → DE (`radio/catalog_profile.go:284-299`) |
| Schlüssel TMDB, TVDB (+PIN), Fanart (+Client), YouTube | `updateAdminSettings.keys` | verschlüsselte Settings | **Umgebung gewinnt** (`env_wins`, `admin/settings.go:509-526`) | Prüfen: `checkProviderKey` |
| E-Mail (SMTP) | `updateAdminSettings.email` | `smtp_*` | Umgebung gewinnt | Testmail: `sendTestEmail` |
| Öffentliche Adresse | `updateAdminSettings.publicUrl` | `public_url` | Umgebung gewinnt (`SLOUCHCAT_PUBLIC_URL`) | |
| Update-Prüfung | `updateAdminSettings.updateCheckEnabled` | `update_check_enabled` | DB | Vorgabe ein |
| Grafikchip | **keiner** | – | nur `SLOUCHCAT_HW` (auto/nvenc/qsv/vaapi/software), beim Start gelesen (`cmd/slouchcat/main.go:690`) | auto probiert NVENC, QSV, VA-API (`transcode/backend.go:71-160`) |
| Trickplay | `createLibrary`/`updateLibrary` | `libraries.trickplay_*` | DB | **nur je Bibliothek**, Vorgabe aus |
| Weitere Profile | `createProfile` | `profiles` | DB | höchstens 10; braucht das aktive Hauptprofil (`auth/profiles.go:101-103`) |
| Bibliotheken | `createLibrary` | `libraries`, `library_roots` | DB | Arten video, audiobooks, podcasts |

Die Verwaltung hat für alles fertige Bauteile:
- `features/admin/integrations/`: `ProviderCard`, `EmailForm` mit Testmail, `ServerBlock`, `SourceChip`
- `features/shared/`: `LibraryFields`, `PathBrowserButton`, `SystemState`
- `features/admin/libraries/TrickplayFields`
- `features/profiles/ProfileForm` mit `AccentPicker` und `PinField`

Die Links zum Beantragen der Schlüssel stehen heute schon im Code (F-6):

| Dienst | Link |
|---|---|
| TMDB | `https://www.themoviedb.org/settings/api` |
| TheTVDB | `https://thetvdb.com/dashboard/account/apikey` |
| Fanart.tv | `https://fanart.tv/get-an-api-key/` |
| YouTube | `https://console.cloud.google.com/apis/credentials` |

### 2.3 Sprachen

**Web-App**
- `supportedLanguages = ['de', 'en']`, Vorgabe `de` (`i18n/index.ts:4-6`).
- Die Erkennung vergleicht nur die Grundsprache. `fr-CH` wird `fr`, ist unbekannt und landet bei
  Deutsch (`:13-19`).
- Jede Sprache wird erst bei Bedarf geladen (`:23-33`). Rückfall ist `en`.
- `de.json` und `en.json` haben je **4318 Texte** in 36 Bereichen.
- Plural im Stil von i18next v4: 187× `_one`, 188× `_other`, 4× `_zero`, kein `_few`/`_many`.
  Platzhalter wie `{{count}}` stehen 309×.
- `locales/locales.test.ts` prüft nur zwei Dinge: gleiche Schlüssel in de und en, und kein `ß`
  in `de.json`.

**Datenbank und Vertrag**
- `profiles.language TEXT NOT NULL DEFAULT 'de' CHECK (language IN ('de','en'))`
  (`migrations/00002_accounts_sessions.sql:32`). Seither wurde die Tabelle nie neu gebaut.
- Go prüft `de`/`en` an fünf Stellen (`auth/auth.go:188`, `auth/invitations.go:261`,
  `auth/profiles.go:296`, `model/model.go:192,354`).
- Im Vertrag ist `Language` `enum: [de, en]` (`openapi.yaml:11420`). Er wird an sechs Stellen
  verwendet: Setup, Profil, Profil anlegen/ändern, Einladung annehmen, Offline-Profil.
- Konten haben keine Sprache. Meldungen ans Konto nehmen die Sprache des Hauptprofils
  (`queries/system/notify_channels.sql:35-47`).

**Server-Texte**
- Eine Go-Map mit 34 Texten je Sprache in `notify/texts.go:12-92`. Rückfall ist `de`.
- Die HTML-Mail hat kein `lang`-Attribut (`:124-137`).
- Ankündigungen schreibt der Admin selbst; sie werden nicht übersetzt.
- API-Fehler tragen einen festen `code`. Die Web-App übersetzt ihn selbst (`de.json`, Bereich
  `errors`).

**Formate**
- Alle `Intl`-Aufrufe nehmen die aktuelle Sprache, keiner hat eine feste Locale: 56× NumberFormat,
  12× DateTimeFormat, 9× DisplayNames, je 2× ListFormat und Collator, 1× RelativeTimeFormat.
- Nur die Testkonfigurationen setzen `de-CH` fest (`web/playwright.config.ts:136`,
  `web/storybook.config.ts:33`).

**Schrift**
- Nur der Systemschriftstapel (`packages/design-tokens/tokens/typography.json:5-17`). Keine
  Webfonts, keine `:lang()`-Regeln.
- Chinesisch, Japanisch und Koreanisch hängen also an den Schriften des Geräts.

**Tests**
- e2e: 88 von 91 Specs wählen Elemente über deutsche Texte, mit rund 3385 Selektoren.
- Storybook: 502 Vergleichsbilder.
- Playwright läuft in `de-CH`, Storybook und die Unit-Tests auf `de`.

**Frühere Entscheide**
- Spec C-3 (`01-anforderungen-v1.md:139`): Oberfläche Deutsch und Englisch, je Profil
  umschaltbar.
- ADR 0018 hat «neue Oberflächensprachen» 2026 ausdrücklich abgelehnt
  (`docs/adr/0018-…md:60-61`, Konzept 10 F-1 «Nur Metadaten»). Grund war der Aufwand von damals
  rund 2300 Texten je Sprache.
- `CLAUDE.md` Regel 9 nennt `web/src/locales/{de,en}.json`.

---

## 3. Teil A: Der geführte Assistent

### 3.1 Grundsatz: zwei Phasen

Heute sammelt der Assistent alles und schickt es auf einmal, **bevor** es ein Konto gibt. Das
geht nicht mehr gut, sobald er Dienste prüfen, Profile anlegen und mehrere Bibliotheken
einrichten soll. Jede Prüfung vor dem Konto wäre ein neuer **öffentlicher** Endpunkt, also ein
Weg, über den Fremde Slouchcat externe Dienste abfragen lassen könnten.

**Vorschlag: Der Assistent arbeitet in zwei Phasen.**

1. **Öffentlich, solange es kein Konto gibt:** nur Schritt 1 (Sprache) und Schritt 2 (Konto). Am
   Ende von Schritt 2 legt `POST /setup` Konto und erstes Profil an und meldet an, wie heute.
2. **Danach mit der Admin-Sitzung:** Die Schritte 3 bis 9 nutzen die **vorhandenen**
   Admin-Endpunkte aus Abschnitt 2.2. Jeder Schritt speichert beim «Weiter». Prüfen heisst
   speichern und dann prüfen: `updateAdminSettings`, danach `checkProviderKey`. Für die Mail
   heisst es speichern und dann `sendTestEmail`.

Was das bringt:
- **Kein neuer öffentlicher Endpunkt.** Ab Schritt 3 gilt dieselbe Rechteprüfung wie in der
  Verwaltung.
- **Volle Ordnerauswahl.** Ab Schritt 3 darf der Assistent den ganzen Ordner-Browser
  (`PathBrowserButton`) zeigen. Vor dem Konto gibt es nur die Wurzeln, aus Sicherheitsgründen
  (`SetupPage.tsx:28-34`).
- **Abbruch ist harmlos.** Was gespeichert ist, bleibt. Der Assistent setzt beim nächsten Anmelden
  dort fort (Abschnitt 3.4).
- **Gleiche Bauteile wie die Verwaltung.** Regel 9 verlangt, dass Admin und Zuschauer dieselben
  Komponenten nutzen. `ProviderCard`, `EmailForm`, `LibraryFields`, `TrickplayFields` und
  `ProfileForm` werden eingebettet, nicht nachgebaut.
- **Umgebungsvariablen werden respektiert.** Ein Schlüssel aus `deploy/.env` erscheint mit dem
  Hinweis «aus .env» (`SourceChip`) und ist nicht änderbar, wie in der Verwaltung.

### 3.2 Reihenfolge der Schritte

Die Reihenfolge weicht vom ersten Mockup ab. Das Mockup ist seit Version 3 nachgeführt.
**Dienste und Wiedergabe kommen vor die Bibliotheken**, damit der erste Scan gleich mit
Schlüssel, Metadaten-Sprachen und Trickplay-Vorgabe läuft. Sonst würde Slouchcat die Titel
zuerst ohne TMDB anlegen und später neu abgleichen.

| # | Schritt | Pflicht | Phase | Speichert über | Überspringen heisst |
|---|---|---|---|---|---|
| 1 | Sprache | ja | öffentlich | nur Browser, dann mit Schritt 2 | – |
| 2 | Konto | ja | öffentlich | `runSetup` (erweitert, 3.5) | – |
| 3 | Region | nein | Admin | `updateMetadataSettings`, `applyRadioOffer` | Empfehlung aus der Sprache (Tabelle 4.1) |
| 4 | Dienste | nein (TMDB empfohlen) | Admin | `updateAdminSettings.keys/.email`, `checkProviderKey`, `sendTestEmail` | keine Schlüssel; Slouchcat zeigt nur Dateinamen |
| 5 | Wiedergabe | nein | Admin | nur die Trickplay-Vorgabe im Browser | Trickplay aus |
| 6 | Bibliotheken | ja, mindestens eine | Admin | `createLibrary` | – |
| 7 | Haushalt | nein | Admin | `createProfile` | nur das eigene Profil |
| 8 | Zugriff | nein | Admin | `updateAdminSettings.publicUrl/.updateCheckEnabled` | keine Adresse, Update-Prüfung ein |
| 9 | Zusammenfassung | – | Admin | `finishSetup` (neu, 3.5) | – |

In der Leiste links ist jeder Schritt anklickbar, sobald Schritt 2 erledigt ist. Vorher sind
nur 1 und 2 erreichbar. Jeder Schritt hat einen Zustand: offen, erledigt (Haken) oder
übersprungen (gestrichelter Kreis). Die Zusammenfassung zeigt «Eingerichtet» oder «Empfehlung».

### 3.3 Die Schritte im Einzelnen

**Schritt 1: Sprache.**
- Ein Raster mit allen 23 Sprachen, jede in **ihrer eigenen Sprache** («Français», «日本語»),
  darunter der englische Name und der Code.
- Ein Suchfeld findet beides, also «Japanese» wie «日本語».
- Vorgewählt ist die erkannte Sprache (4.2). Ein Klick schaltet die ganze Oberfläche sofort um.
- Darunter eine Vorschau mit echtem Datum und echter Zahl in der gewählten Sprache (`Intl`). Bei
  de-CH steht zum Beispiel `1’234’567.5`.
- Oben rechts steht auf `/setup` und `/login` ein Sprachknopf. Damit lässt sich die Sprache vor
  der Anmeldung wechseln, nicht nur im Assistenten.

**Schritt 2: Konto.**
- Felder: E-Mail, Passwort zweimal (mindestens 10 Zeichen wie heute), **Name und Farbe des
  ersten Profils**.
- Heute heisst das Profil wie der Teil der E-Mail vor dem @ und hat keine Farbe. Neu wählt man
  beides. Die Farbe kommt aus `AccentPicker` mit den 8 Akzent-Tokens.
- Die Profilsprache ist die aus Schritt 1.
- «Weiter» ruft `runSetup` auf. Ab hier ist man angemeldet.
- Wer danach zu Schritt 1 zurückgeht und die Sprache ändert, ändert sie am Profil
  (`updateProfile`).

**Schritt 3: Region.**
- **Land:** Vorschlag aus der Sprache, sonst aus der Zeitzone des Browsers. Es dient als
  Vorschlag fürs Radio und für die Metadaten-Sprachen.
- **Altersfreigaben nach:** nur die fünf Länder, die Slouchcat kennt (DE, AT, CH, GB, US).
  Weitere Länder sind ein eigenes Konzept (O-5).
- **Sprachen der Metadaten:** geordnete Liste aus `knownLanguages` (ADR 0018), Vorschlag nach
  Tabelle 4.1. Die Bildsprachen folgen der Textliste nach derselben Regel wie die heutige Vorgabe.
  Schritt 0 prüft, ob `updateMetadataSettings` dafür eigene Werte braucht.
- **Zeitzone des Servers:** nur Anzeige. Sie kommt aus `TZ`, und dafür gibt es weder Setting noch
  Endpunkt. Der Assistent vergleicht sie mit der Zeitzone des Browsers
  (`Intl.DateTimeFormat().resolvedOptions().timeZone`). Weichen beide ab, zeigt er die Zeile für
  `deploy/.env` und den Hinweis auf einen Neustart.
  - Dafür bekommt `SystemInfo` ein neues Feld `timeZone`, die von `admin.ResolveZone` aufgelöste
    Zone. Die Änderung ist additiv.
- **Radio-Angebot:** Schalter «Radio-Angebot aus <Land> vorbereiten», Vorgabe ein. Er ruft
  `applyRadioOffer` mit diesem Land auf. Die Sender wählt man später selbst aus (Konzept 19
  Teil B).

**Schritt 4: Dienste.**
- Eine Karte je Dienst: TMDB (Rahmen in Akzentfarbe, «Empfohlen»), TheTVDB mit PIN, Fanart.tv
  mit optionalem Client-Schlüssel, YouTube für Trailer, mit dem Hinweis «Tageslimit 100 Suchen».
- **Jede Karte trägt den Link zum Beantragen (F-6).** Der Link ist sichtbar, bevor man die Karte
  aufklappt.
- «Prüfen» speichert den Schlüssel und ruft dann `checkProviderKey` auf. Lehnt der Dienst ab,
  steht die Meldung neben dem Feld, und der Schlüssel lässt sich mit «Entfernen» wieder löschen.
- **E-Mail** als aufklappbarer Block aus `EmailForm`: Server, Port, Verschlüsselung, Benutzer,
  Passwort, Absender und «Testmail an mich senden».
- Web-Push, Telegram, Trakt und Google Books gehören nicht in den Assistenten (O-11).
- Jede Karte zeigt «aus .env», wenn die Umgebung den Wert setzt.

**Schritt 5: Wiedergabe.**
- Anzeige aus `SystemState` (`SetupStatus.system` bzw. `getSystem`): Grafikchip, Backend,
  Hardware-Kodierung, HDR→SDR, inotify-Limit mit Warnung über 80 %.
- **Keine Wahl des Backends.** `SLOUCHCAT_HW` wird nur beim Start gelesen, und auto wählt schon
  das Beste. Der Text erklärt das und nennt `SLOUCHCAT_HW` in `deploy/.env` als Weg für eine
  andere Wahl.
- Fehlt der Grafikchip, zeigt `SystemState` heute schon den Befehl mit dem passenden
  Compose-Override.
- **Trickplay-Vorgabe:** ein Schalter, Vorgabe aus. Er belegt im nächsten Schritt die
  Trickplay-Felder jeder neuen Video-Bibliothek vor. Ein eigenes Setting gibt es dafür nicht,
  weil Trickplay je Bibliothek gilt.

**Schritt 6: Bibliotheken.**
- Je Bibliothek eine Karte aus `LibraryFields`: Name, Art, Ordner mit `PathBrowserButton` und
  «Ordner prüfen», Freigabe, bei Video `TrickplayFields`.
- «+ Weitere Bibliothek» fügt eine Karte an.
- Die Wurzeln aus `SLOUCHCAT_BROWSE_ROOTS` stehen als Knöpfe unter dem Feld wie heute. Die
  Warnung für Netzlaufwerke (Plan 3.4) bleibt.
- «Weiter» legt jede neue Karte mit `createLibrary` an. Schritt 0 prüft, ob `createLibrary` den
  Scan sofort startet. Wenn ja, verschiebt der Assistent den Scan nicht künstlich.
- Mindestens eine Bibliothek ist Pflicht, wie heute.

**Schritt 7: Haushalt.**
- Liste der Profile, oben das eigene.
- «Profil hinzufügen» mit Name, Altersstufe (0, 6, 12, 16, 18), Farbe und dem Schalter «Wechsel
  mit PIN schützen». Ist er an, legt das Profil die PIN beim ersten Öffnen selbst fest
  (`PinSetDialog`).
- Die Sprache neuer Profile ist die des Administrators. Höchstens 10 Profile.
- Ein Hinweis nennt «Verwaltung › Konten» für Gäste mit eigenem Konto. Einladungen gehören nicht
  in den Assistenten.

**Schritt 8: Zugriff.**
- Öffentliche Adresse, mit «aus .env», falls gesetzt. Darunter die Adresse, unter der man gerade
  zugreift, als Vorschlag.
- Der Hinweis erklärt, wozu die Adresse dient: Einladungen, E-Mails, Push.
- Schalter «Täglich nach neuen Versionen suchen», Vorgabe ein, mit dem Satz «Slouchcat sendet
  keine Nutzungsdaten».
- Eine Prüfung, ob die Adresse von aussen erreichbar ist, kommt nicht in die erste Fassung (O-4).

**Schritt 9: Zusammenfassung.**
- Je Schritt die gewählten Werte und «Ändern».
- «Einrichtung abschliessen» ruft `finishSetup` auf. Der Assistent zeigt danach eine
  Abschlussseite mit dem Scan-Fortschritt je Bibliothek aus der Jobliste und den Knopf «Zur
  Startseite» (O-13).

### 3.4 Fortschritt, Abbruch und bestehende Installationen

- Der Fortschritt liegt auf dem Server, im neuen Setting `setup_wizard`:
  - je Schritt `open`, `done` oder `skipped`
  - dazu `finishedAt`
- **Abbruch nach Schritt 2:**
  - Meldet sich der Administrator an und ist `finishedAt` leer, leitet die Web-App **einmal** auf
    den ersten offenen Schritt weiter.
  - Danach zeigt `/admin` ein Band «Einrichtung fortsetzen», bis sie abgeschlossen ist.
  - In der Leiste steht «Später fortsetzen». Der Knopf führt zur Startseite, ohne abzuschliessen
    (O-3).
- **Bestehende Installationen:** Die Migration der Stufe 2 schreibt `setup_wizard` mit
  `finishedAt = jetzt`, wenn schon ein Konto existiert. CT 190 und jede andere Installation sehen
  den Assistenten also **nie**.
- **Andere Konten:** Der Assistent gehört dem Administrator. Ein Gast oder ein eingeladenes Konto
  sieht ihn nie.

### 3.5 Vertrag und Server

Alle Änderungen sind **additiv**, und eine Vertragsfassung kommt in Stufe 2.

- **`SetupRequest`:**
  - neu `profileName` (optional, 1–60) und `accent` (optional, Akzent-Token)
  - `library` wird **optional**. Alte Aufrufer wie `e2e/global-setup.ts:302-318` schicken sie
    weiter und bekommen das bisherige Verhalten.
  - `tmdbApiKey` bleibt optional.
  - `language` folgt dem neuen `Language` aus Teil B.
- **Neu, `x-visibility: admin`:**
  - `getSetupProgress` (`GET /admin/setup`)
  - `updateSetupProgress` (`PUT /admin/setup`, ein Schritt und sein Zustand)
  - `finishSetup` (`POST /admin/setup/finish`). Er setzt `finishedAt` und reiht für jede
    Bibliothek ohne abgeschlossenen Scan einen ein.
  - Jede Operation bekommt einen Eintrag im Vertragstest.
- **`SystemInfo.timeZone`:** neues Feld, nur lesend.
- **`checkSetupTmdbKey` und `checkSetupPath`:** Der neue Assistent braucht sie nicht mehr. Sie
  bleiben im Vertrag, werden als `deprecated` markiert und fallen erst in einer brechenden
  Fassung weg (O-12).
- **Migration:** nur Daten, keine Tabelle. Sie fügt `setup_wizard` für bestehende Installationen
  ein, nach Vorlage.

### 3.6 Web-App

- Routen: `/setup` (Schritte 1 und 2, öffentlich) und `/setup/$step` (Schritte 3 bis 9, nur
  Administrator).
- Die Weiche in `router.tsx` lernt zusätzlich: angemeldet, Administrator, `finishedAt` leer und
  noch nicht weitergeleitet, dann auf den ersten offenen Schritt.
- Die Leiste ist ein `SideNav`-Muster wie in der Verwaltung. Bei schmaler Breite wird sie zur
  waagrechten Schrittleiste oben, wie im Mockup.
- Der Fuss hat «Zurück», «Überspringen» (nur bei freiwilligen Schritten) und «Weiter» bzw. im
  letzten Schritt «Einrichtung abschliessen».
- Jeder Schritt bekommt seinen **Playwright-Tastaturtest** (Regel 8): Tab-Reihenfolge Leiste →
  Inhalt → Fuss, Enter auf «Weiter», sichtbarer Fokus, keine Fokusfalle.
- Nach dem Schrittwechsel liegt der Fokus auf der Überschrift.
- `features/setup/SetupPage.tsx` wird aufgeteilt in `SetupLayout`, je Schritt eine Datei und eine
  gemeinsame Zustandsquelle aus `getSetupProgress`.

### 3.7 Was der Assistent bewusst nicht tut

- Er wählt kein Transcoding-Backend und setzt keine Zeitzone. Beides kommt aus der Umgebung und
  gilt erst nach einem Neustart.
- Er richtet keine Einladungen, kein Web-Push, kein Telegram und kein Trakt ein.
- Er legt keine Podcasts an und wählt keine Radiosender aus, sondern bereitet nur das Angebot
  vor.
- Er ändert nichts an `deploy/.env`. Der Server schreibt diese Datei nicht.

---

## 4. Teil B: Dreiundzwanzig Oberflächensprachen

### 4.1 Die Sprachen

| Code | Name (eigene Sprache) | Pluralformen (CLDR) | Schrift | Vorschlag Metadaten | Freigabeland |
|---|---|---|---|---|---|
| `de` | Deutsch (Schweiz) | one, other | Latein | de → en | CH |
| `de-DE` | Deutsch (Deutschland) | one, other | Latein | de → en | DE |
| `en` | English | one, other | Latein | en | US |
| `fr` | Français | one, many, other | Latein | fr → en | US (O-5) |
| `es` | Español | one, many, other | Latein | es → en | US |
| `it` | Italiano | one, many, other | Latein | it → en | US |
| `nl` | Nederlands | one, other | Latein | nl → en | US |
| `pt-BR` | Português (Brasil) | one, many, other | Latein | pt-BR → en | US |
| `pt-PT` | Português (Portugal) | one, many, other | Latein | pt-PT → en | US |
| `pl` | Polski | one, few, many, other | Latein | pl → en | US |
| `sv` | Svenska | one, other | Latein | sv → en | US |
| `da` | Dansk | one, other | Latein | da → en | US |
| `nb` | Norsk bokmål | one, other | Latein | no → en | US |
| `fi` | Suomi | one, other | Latein | fi → en | US |
| `cs` | Čeština | one, few, many, other | Latein | cs → en | US |
| `hu` | Magyar | one, other | Latein | hu → en | US |
| `tr` | Türkçe | one, other | Latein (ı/İ) | tr → en | US |
| `ru` | Русский | one, few, many, other | Kyrillisch | ru → en | US |
| `uk` | Українська | one, few, many, other | Kyrillisch | uk → en | US |
| `ja` | 日本語 | other | CJK | ja → en | US |
| `ko` | 한국어 | other | Hangul | ko → en | US |
| `zh-CN` | 简体中文 | other | CJK vereinfacht | zh-CN → en | US |
| `zh-TW` | 繁體中文 | other | CJK traditionell | zh-TW → en | US |

- Die Pluralformen sind die, die `Intl.PluralRules(<code>).resolvedOptions().pluralCategories`
  liefert. i18next fragt genau diese ab, und Abschnitt 4.9 macht daraus einen Test.
- Die vier `_zero`-Texte (`de.json:4131-4151`) gelten als Sonderfall für die Zahl 0 und werden in
  jeder Sprache übersetzt.
- Bei TMDB heisst Norwegisch Bokmål `no`. Schritt 0 prüft, ob `knownLanguages` `nb` oder `no`
  führt.

### 4.2 Sprachcodes, Deutsch in zwei Varianten, Erkennung

- **`de` bleibt der Code für Deutsch (Schweiz).** Er ist heute in jeder Datenbank, jedem
  Offline-Profil und jedem Vertrag gespeichert. Ein Umbenennen in `de-CH` würde den Vertrag
  brechen und jede Installation migrieren, ohne dass jemand etwas davon hat.
- **`de-DE` ist eine Überlagerung.** `locales/de-DE.json` enthält **nur** die Texte, die in
  Deutschland anders lauten. i18next löst `de-DE` von sich aus über `de` auf. Fehlt ein Text in
  `de-DE.json`, nimmt es den aus `de.json`.
  - Eine Auszählung am Anker findet rund **185** betroffene Texte: 53 mit ss-Wörtern, die mit ß
    geschrieben werden («Strasse», «gross», «schliessen» …), und 133 mit «Guillemets», die in
    Deutschland „so“ stehen.
  - Ersetzt wird **nicht mechanisch**. «dass», «Klasse» und «Passwort» bleiben. Claude erstellt
    die Fassung, und ein Test prüft, dass jeder Text in `de-DE.json` sich vom Text in `de.json`
    unterscheidet.
- **Erkennung** (`detectLanguage`, neu in dieser Reihenfolge):
  1. Ganzer Code passt: `pt-BR`, `zh-TW`, `de-DE` …
  2. Bekannte Zuordnung:
     - `de-AT`, `de-LU`, `de-BE` → `de-DE`
     - `de-CH`, `de-LI` → `de`
     - `pt` → `pt-BR`
     - `zh-Hant`, `zh-HK`, `zh-MO` → `zh-TW`
     - `zh-Hans`, `zh`, `zh-SG` → `zh-CN`
     - `no`, `nn` → `nb`
  3. Grundsprache passt: `fr-CH` → `fr`, `it-CH` → `it`.
  4. Sonst `en`. Heute ist der Rückfall `de` (O-2).
- **Rückfallkette beim Laden:** Sprache → (bei `de-DE` `de`) → `en`. So bleibt ein vergessener
  Text lesbar. Der Release-Test aus 4.8 verhindert, dass das im Release vorkommt.

### 4.3 Datenbank und Vertrag

- **Vertrag:** `Language` wird von `enum [de, en]` auf die 23 Codes aus 4.1 erweitert. Für
  Aufrufer, die eine Sprache **schicken**, ist das additiv. Ein Client, der eine **empfangene**
  Sprache streng prüft, könnte stolpern; das betrifft nur die eigene Web-App, die mit dem
  Vertrag zusammen gebaut wird. Neue Minor-Fassung.
- **Datenbank:** Der `CHECK` auf `profiles.language` muss weg. SQLite kann einen `CHECK` nur durch
  einen **Neubau der Tabelle** entfernen.
  - Die Migration folgt der Vorlage: `-- +goose NO TRANSACTION`, `PRAGMA foreign_keys=OFF`, neue
    Tabelle, Daten kopieren, alte löschen, umbenennen, Indizes und Trigger neu,
    `PRAGMA foreign_key_check`, `PRAGMA foreign_keys=ON`.
  - Die Liste der erlaubten Codes lebt danach **an einer Stelle in Go**, in einem neuen
    `model.UILanguages`. Die fünf Prüfstellen aus 2.3 nutzen sie.
  - Ein neuer `CHECK` mit 23 Werten würde jede weitere Sprache zu einem Tabellen-Neubau machen.
    Darum gibt es keinen (O-14).
  - Der eingefrorene DB-Stand in `testdata/db/` bekommt den Fall «Profil mit `en`» für die
    Migrationsprüfung.
- **`profiles.metadata_language`** bleibt, wie es ist (ADR 0018). Neu ist nur, dass NULL («wie
  Oberfläche») jetzt auch `fr`, `ja` usw. bedeuten kann. Liegt die Sprache nicht in der Liste der
  Metadaten-Sprachen, greift die bestehende Kette in `visibility/mapping.go:45-58`.

### 4.4 Web-App

- **Laden:**
  - `loaders` wird aus `import.meta.glob('@/locales/*.json')` gebaut statt von Hand.
  - `supportedLanguages` kommt aus dem Vertrag (generierter Typ `Language`), damit beide nie
    auseinanderlaufen.
  - Jede Sprache bleibt ein eigenes, nur bei Bedarf geladenes Bündel. Der Start lädt wie heute
    nur eine Sprache.
- **Sprachwahl:**
  - `languageOptions` (`features/shared/choices.ts:17`) zeigt die Namen aus der Tabelle 4.1 in
    der jeweiligen Sprache. Diese Namen stehen als feste Liste im Code, nicht in den
    Übersetzungen, damit jeder seine Sprache findet.
  - Verwendet wird die Wahl im Profilformular, bei der Einladung, in Schritt 1 und im neuen
    Sprachknopf auf `/login`.
- **Formate:** Nichts zu tun. Alle `Intl`-Aufrufe nehmen schon die aktuelle Sprache. Die
  `format.*`-Texte für Laufzeiten (`de.json:706-720`) werden mitübersetzt.
- **Schrift:** Weiter keine Webfonts (Startzeit, Offline-Grösse). Neu kommen `:lang()`-Regeln für
  die CJK-Stapel. Sonst wählt das Gerät für Japanisch womöglich chinesische Glyphen, ein bekanntes
  Problem der Han-Vereinheitlichung.
  - `:lang(ja)`: Hiragino Sans, Yu Gothic UI, Noto Sans JP, sans-serif
  - `:lang(zh-CN)`: PingFang SC, Microsoft YaHei, Noto Sans SC
  - `:lang(zh-TW)`: PingFang TC, Microsoft JhengHei, Noto Sans TC
  - `:lang(ko)`: Apple SD Gothic Neo, Malgun Gothic, Noto Sans KR
  - Die Stapel kommen als Tokens in `typography.json`. Regel 9 gilt auch für Schriften.
- **Länge:** Finnisch, Deutsch und Französisch sind bis 40 % länger als Englisch. CJK ist kürzer,
  braucht aber mehr Zeilenhöhe.
  - Neu ist eine **Pseudo-Sprache** `x-long`, nur in der Entwicklung und in Storybook. Sie
    verlängert jeden Text um 40 % und setzt ihn in eckige Klammern.
  - Ein Storybook-Durchlauf in `x-long` zeigt abgeschnittene Texte, bevor eine echte Sprache sie
    trifft. Er erzeugt keine eigenen Vergleichsbilder (4.9).
- **`<html lang>`** wird schon gesetzt (`i18n/index.ts:46-48`). Das Manifest (`vite.config.ts:104`)
  und `index.html` bleiben bei `de`; der Browser liest sie nur vor dem Start.

### 4.5 Server-Texte

- Die 34 Texte aus `notify/texts.go` wandern in **eingebettete JSON-Dateien**
  `server/internal/notify/locales/<code>.json`.
  - Sie haben dasselbe Format wie die Web-App, also i18next v4 mit `{{name}}` statt `%s`, damit
    Weblate beide gleich behandelt.
  - Ein kleines Go-Paket `internal/i18n` lädt sie (`embed`) und setzt Platzhalter ein.
  - Die Rückfallkette ist dieselbe wie im Web: Sprache → `de` bei `de-DE` → `en`. Heute fällt der
    Server auf `de` zurück.
- Die HTML-Mail bekommt `lang="<code>"`.
- Die Sprache des Empfängers wird gewählt wie heute (Profil bzw. Hauptprofil, `events.go:31`).
- Die Go-Tests, die heute deutsche Sätze prüfen (`notify/channels_test.go`, `email_test.go`,
  `push_test.go`), prüfen danach gegen die JSON-Datei statt gegen einen festen Satz.
- **Nicht übersetzt** werden Ankündigungen (Text des Admins), Fehlertitel im API (`title` bleibt
  englisch, die Web-App übersetzt den `code`) und Log-Zeilen.

### 4.6 Übersetzen: die Erstfassung durch Claude

Marco will «natürlich und sinnvoll übersetzt» (F-4). Das heisst nicht Wort für Wort, sondern so,
wie eine gute App in dieser Sprache klingt. Dafür braucht es drei Dinge, die vor dem ersten
übersetzten Text stehen.

1. **Glossar** `docs/i18n/glossar.md`:
   - rund 80 Begriffe aus Slouchcat («Profil», «Bibliothek», «Merkliste», «Gesehen», «Sammlung»,
     «Trickplay», «Spotlight», «Freigabe», «Verwaltung» …), je Sprache mit einer festen
     Übersetzung
   - Begriffe, die **nicht** übersetzt werden: Slouchcat, TMDB, Trickplay als Fachbegriff, Namen
     von Diensten
   - Fachbegriffe richten sich nach dem, was Streamingdienste in der Sprache verwenden. Zum
     Beispiel heisst die Merkliste bei Netflix auf Französisch «Ma liste».
2. **Stilregeln je Sprache** `docs/i18n/stil/<code>.md`:
   - Anrede (O-1)
   - Anführungszeichen: de «», de-DE „“, fr « » mit schmalem geschütztem Leerzeichen, ja 「」,
     zh-TW 「」, zh-CN “” …
   - Gross- und Kleinschreibung in Knöpfen (en Title Case oder Sentence case?)
   - Satzzeichen, Zahlwörter, Datum (kommt aus `Intl`)
3. **Kontext je Text:** Ein Hilfsskript `scripts/i18n/context.ts` erzeugt zu jedem Schlüssel die
   Datei und Zeile, an der er verwendet wird, und den Komponentennamen. So weiss die Übersetzung,
   ob «Play» ein Knopf, eine Überschrift oder ein Statustext ist.

**Ablauf je Sprache**, als Agentenlauf in Stufe 3:
1. Übersetzen in Blöcken je Bereich (36 Bereiche), mit Deutsch **und** Englisch als Quelle,
   Glossar, Stilregeln und Kontext.
2. **Prüfung durch einen zweiten Agenten**, der nicht übersetzt hat:
   - Glossartreue
   - Platzhalter (`{{count}}` …) vollständig und unverändert
   - Pluralformen nach 4.1
   - Länge: Knöpfe höchstens rund 1,5× Englisch, Warnliste
   - Ton
   - Rückübersetzung von 50 zufälligen Texten ins Deutsche. Weicht der Sinn ab, wird
     nachgebessert.
3. **Bildprüfung:** Storybook-Durchlauf je Sprache für 20 Kernseiten (Anmeldung, Startseite,
   Detailseite, Abspielseite, Verwaltung, Assistent), ohne Vergleichsbilder. Ein Agent sucht auf
   den Bildern abgeschnittene oder überlaufende Texte.
4. Marco prüft Stichproben in Sprachen, die er liest (O-9).

**Umfang:**
- 4318 Web-Texte und 34 Server-Texte, für 20 neue Sprachen, also rund **87'000 Texte**.
- Dazu `de-DE` mit rund 185 Texten.
- Das ist ein mehrtägiger Agentenlauf und wird in Stufe 3 in Sprachgruppen geschnitten (Abschnitt
  7).

### 4.7 Weblate

- **Gehostetes Weblate, Plan «Libre»:** kostenlos für öffentliche Projekte unter freier Lizenz.
  Slouchcat ist öffentlich (`SlouchCat/slouchcat`) und steht unter AGPL-3.0, erfüllt also beides.
  Ein selbst betriebenes Weblate wäre ein weiterer Dienst zum Pflegen (O-8).
- **Konto und Antrag macht Marco selbst.** Claude legt keine Konten an.
- **Zwei Komponenten:**
  - `web` mit `web/src/locales/*.json`
  - `server` mit `server/internal/notify/locales/*.json`
  - Format jeweils «i18next JSON v4».
  - **Vorlage ist Englisch** (`en.json`), weil die meisten Übersetzer Englisch lesen.
  - Deutsch (`de`, `de-DE`) ist in Weblate gesperrt. Es wird im Repository gepflegt.
- **Weg zurück ins Repository:** Weblate committet auf einen eigenen Zweig und öffnet einen
  Pull Request. Die CI prüft ihn wie jeden anderen. Gemergt wird auf Marcos Wort.
- **Regel gegen Kollisionen:** Ab Stufe 4 ändern Agenten bestehende Übersetzungen der 20 Sprachen
  **nicht** mehr direkt. Sie füllen nur fehlende Texte nach (4.8). Korrekturen laufen über
  Weblate.
- `docs/i18n/README.md` erklärt, wie man mithilft. Das README bekommt eine Weblate-Plakette mit
  dem Stand je Sprache.

### 4.8 Neue Texte im Alltag

Heute muss jede Änderung `de` und `en` gleichzeitig liefern; `locales.test.ts` setzt das durch.
Mit 23 Sprachen soll nicht jeder Pull Request zwanzig Übersetzungen tragen.

**Vorschlag (O-7):**
- **Im Pull Request:** `de` und `en` sind Pflicht wie heute. `de-DE` ist Pflicht, sobald der
  neue Text ein ss-Wort oder «Guillemets» enthält. Ein Test zeigt die Kandidaten an.
- **Fehlende Texte der übrigen Sprachen** brechen den Test nicht. `task i18n:status` listet sie.
- **Vor jedem Release:** Ein neuer Schritt «Übersetzungen nachziehen» im Release-Ablauf. Claude
  übersetzt die fehlenden Texte nach 4.6. `task i18n:check` muss grün sein, also keine Sprache
  mit fehlendem Text. Erst dann wird getaggt.
- So hat jedes Release alle Sprachen vollständig, und zwischen zwei Releases sieht man in `main`
  höchstens englische Rückfälle.

### 4.9 Tests

Neu oder geändert:
- **`locales.test.ts`, je Sprache:**
  - gleiche Schlüssel wie `de`, abzüglich der Pluralformen
  - Pluralformen genau nach `Intl.PluralRules`, also bei `ru` `_one`, `_few`, `_many` und
    `_other`, bei `ja` nur `_other`
  - gleiche Platzhalter je Text
  - kein leerer Text
  - kein Text identisch mit Englisch, ausser auf einer Ausnahmeliste (Markennamen, «OK», Codes)
- `de`: weiterhin kein ß. `de-DE`: nur Schlüssel, die es in `de` gibt, und jeder Text weicht ab.
- **Go:** dieselben Prüfungen für die Server-Texte. Dazu ein Test, dass `model.UILanguages` und
  die Vertragsliste `Language` gleich sind.
- **e2e:** bleibt auf `de-CH`. Die 3385 Selektoren werden **nicht** umgebaut.
  - Neu ist eine Spec `languages.spec.ts`: Für jede der 23 Sprachen meldet sie sich an, öffnet
    Startseite und Profilformular und prüft `<html lang>` und einen bekannten Text.
  - Dazu ein Schritt im Assistenten mit Sprachwechsel.
  - Gestartet wird über `task e2e:quick` wie jede andere Spec.
- **Storybook-Vergleichsbilder:** bleiben bei `de`, also keine 23 × 502 Bilder. Die Bildprüfung
  aus 4.6 ist ein einmaliger Lauf je Übersetzungsrunde, kein Gate.
- **Migration:** Ein eingefrorener DB-Stand vor der Migration, danach `foreign_key_check` und ein
  Profil mit `ja` speichern.

### 4.10 Bewusst nicht in diesem Konzept

- **Rechts nach links** (Arabisch, Hebräisch, Persisch):
  - Die Web-App nutzt heute 73× `left:` und 52× `right:`, aber nur wenige logische Eigenschaften.
  - Das ist ein eigener Umbau mit eigenem Konzept (O-16).
  - Stufe 1 setzt aber schon `dir="ltr"` am `<html>` und verlangt in stylelint für **neuen** Code
    logische Eigenschaften, damit die Zahl nicht wächst.
- **Artikel beim Sortieren** (`model/sortkey.go:16-17`): kennt nur «der/die/das» und «the». Titel
  kommen in der Metadaten-Sprache, nicht in der Oberflächensprache. Gehört in Konzept 10/ADR
  0018, nicht hierher.
- **Weitere Freigabeländer** (FR, ES, IT, JP …): Das ist ein eigenes Konzept, weil die
  Altersstufen der Profile heute nach FSK aufgebaut sind (0/6/12/16/18).
- **Übersetzte Metadaten:** Die kommen von TMDB, TVDB und anderen und sind ADR 0018.

---

## 5. Risiken

| Risiko | Folge | Gegenmittel |
|---|---|---|
| Maschinell klingende Übersetzung | Marcos Ziel «natürlich» verfehlt | Glossar, Stilregeln, Kontext, zweiter Prüfagent, Rückübersetzung, Weblate |
| Abgeschnittene Texte in langen Sprachen | unlesbare Knöpfe | Pseudo-Sprache `x-long`, Bildprüfung je Sprache |
| Neubau der Tabelle `profiles` | Fremdschlüssel von Fortschritten, Merklisten, Downloads | Vorlage mit `foreign_key_check`, eingefrorener DB-Stand, Sicherung vor Migration (Regel 5) |
| Release-Ablauf wird länger | «Übersetzungen nachziehen» kostet bei jedem Release Zeit | Nur neue Texte; typisch 10 bis 50 je Release |
| Weblate-PR ändert einen Platzhalter | Laufzeitfehler | Weblate-Prüfung «Platzhalter» und `locales.test.ts` in der CI |
| Assistent verweist auf `.env`, Marco ändert sie | Neustart nötig, Verwirrung | Text sagt «danach neu starten»; Updater-Begleiter startet nicht von selbst |
| Bestehende Installation sieht plötzlich den Assistenten | Ärger | Migration setzt `finishedAt`, Test mit eingefrorenem DB-Stand |

---

## 6. Änderungen an früheren Entscheiden

- **Spec C-3** (Oberfläche Deutsch und Englisch) wird zu «23 Oberflächensprachen nach Konzept 22,
  je Profil wählbar».
- **ADR 0018, Konzept 10 F-1** («Nur Metadaten», keine neuen Oberflächensprachen) wird in diesem
  einen Punkt durch **ADR 0033** ersetzt. Der Grund von damals (2300 Texte je Sprache von Hand)
  entfällt durch die Erstfassung durch Claude und Weblate. Der Rest von ADR 0018 bleibt.
- **ADR 0001** nennt `{de,en}.json`. Der Satz wird auf `web/src/locales/*.json` geändert.
- **`CLAUDE.md` Regel 9:** Texte nur aus `web/src/locales/*.json`. Dazu der Satz: «`de` und `en`
  in jedem Pull Request, die übrigen Sprachen vor dem Release (Konzept 22, 4.8).»
- **Spec A-1, Plan 3.4:** Der Assistent hat neun Schritte statt vier. Die Warnungen aus Plan 3.4
  bleiben in Schritt 5 und 6.
- **`docs/INSTALLATION.md` Abschnitt 5** wird für den neuen Ablauf neu geschrieben und nennt
  `TZ` und `SLOUCHCAT_HW` als die beiden Werte, die nur in `.env` gehen.
- **Entscheidungsprotokoll:** Frage 29 bekommt eine Zeile, sobald Marco das Konzept freigibt.

---

## 7. Stufen der Umsetzung

Jede Stufe ist ein eigener Zweig mit Pull Request und Release. Vertrag und Migration werden in
Bau-Reihenfolge nummeriert.

| Stufe | Inhalt | Vertrag | Migration | Sichtbar für Marco |
|---|---|---|---|---|
| **1 «Sprachgerüst»** | 4.2 bis 4.5 und 4.9: `Language` mit 23 Codes, Tabellen-Neubau `profiles`, `model.UILanguages`, Erkennung und Rückfall, `de-DE.json` (rund 185 Texte), Server-Texte als JSON, `:lang()`-Schriften, Pseudo-Sprache, neue Tests, Sprachknopf auf `/login`. Wählbar sind nur Sprachen mit vollständigem Paket, also `de`, `de-DE` und `en`. | ja | ja | Deutsch (Deutschland) wählbar |
| **2 «Assistent»** | Teil A ganz: zwei Phasen, neun Schritte in Variante A, `setup_wizard`, neue Admin-Endpunkte, `SystemInfo.timeZone`, Tastaturtests, Migration für bestehende Installationen, INSTALLATION.md. | ja | ja (nur Daten) | neuer Assistent auf einer frischen Testinstanz |
| **3 «Übersetzungen»** | 4.6: Glossar, Stilregeln, Kontextskript, dann 20 Sprachen in vier Gruppen (fr/es/it/nl/pt-BR · pt-PT/pl/cs/hu/tr · sv/da/nb/fi · ru/uk/ja/ko/zh-CN/zh-TW), je Gruppe Übersetzen, Prüfen, Bildprüfung. **Ein Release für alle 20** (F-4), die Gruppen sind nur Arbeitspakete. | nein | nein | 23 Sprachen wählbar |
| **4 «Weblate»** | 4.7 und 4.8: Weblate-Projekt (Konto durch Marco), Komponenten, `docs/i18n/README.md`, Plakette, Release-Schritt «Übersetzungen nachziehen», `task i18n:status`/`i18n:check`. | nein | nein | Mitarbeit von aussen möglich |

Stufe 2 hängt nur am neuen `Language` aus Stufe 1. Sie kann auch vor Stufe 3 oder parallel dazu
laufen (O-10). Stufe 4 braucht Stufe 3.

**Schritt 0 jeder Stufe:** die «Heute»-Angaben aus Abschnitt 2 am dann aktuellen `main`
nachprüfen. Bekannt offen sind:
- startet `createLibrary` den Scan sofort?
- `nb` oder `no` in `knownLanguages`?
- braucht `updateMetadataSettings` für Bildsprachen eigene Werte?
- Nummern von Vertrag und Migration nach K21 Stufen 4–6

---

## 8. Offene Punkte (Empfehlung gilt bis zu Marcos Antwort)

| Nr. | Frage | Empfehlung |
|---|---|---|
| **O-1** | Anrede je Sprache | **Wie die grossen Plattformen in der Sprache:** de du (wie heute), fr vous, es tú, it tu, nl je, pt-BR você, pt-PT você, pl Imperativ ohne Pronomen, cs vy, ru вы, uk ви, tr siz, hu Sie-Form ohne Pronomen, sv/da/nb du, fi sinä ohne Pronomen, ja です/ます, ko 해요체, zh 你. Festgehalten in den Stilregeln. |
| **O-2** | Rückfall, wenn keine Browsersprache passt | **Englisch** statt Deutsch wie heute. Wer Deutsch spricht, hat einen deutschen Browser. |
| **O-3** | Abbruch nach Schritt 2 | **Einmal weiterleiten**, danach ein Band in `/admin`, bis abgeschlossen. |
| **O-4** | «Erreichbarkeit prüfen» für die öffentliche Adresse | **Nicht in Stufe 2.** Der Server müsste sich selbst über das Internet erreichen, und das scheitert an vielen Routern (NAT-Hairpin), obwohl alles stimmt. |
| **O-5** | Freigabeland für Sprachen ausserhalb DE/AT/CH/GB/US | **US als Vorschlag**, frei wählbar unter den fünf. Weitere Länder in einem eigenen Konzept. |
| **O-6** | Sprachen mit unvollständiger Übersetzung | **Nicht wählbar.** Die Liste zeigt nur Sprachen ohne fehlenden Text; der Release-Test aus 4.8 hält das ohnehin ein. |
| **O-7** | Neue Texte: Pflicht im Pull Request oder vor dem Release? | **de und en im Pull Request, der Rest vor dem Release** (4.8). |
| **O-8** | Weblate gehostet oder selbst betrieben? | **Gehostet, Plan «Libre».** Kein weiterer Dienst auf CT 190. |
| **O-9** | Wer prüft die Qualität? | **Prüfagent je Sprache, Marco Stichproben** in den Sprachen, die er liest (de-DE, en, fr, it?), danach Weblate. |
| **O-10** | Reihenfolge der Stufen 2 und 3 | **Assistent vor Übersetzungen.** Er ist kleiner, und der Sprachschritt zeigt Stufe 3 danach gleich mit. |
| **O-11** | Trakt, Google Books, Web-Push, Telegram im Assistenten? | **Nein.** Trakt ist Sache des Kontos, Google Books nur für Hörbücher. Push und Telegram brauchen ein Gerät bzw. einen Bot. Alles bleibt in der Verwaltung. |
| **O-12** | Öffentliche Prüf-Endpunkte `checkSetupPath`, `checkSetupTmdbKey` | **Als `deprecated` behalten**, in der nächsten brechenden Fassung entfernen. |
| **O-13** | Nach «Abschliessen»: Abschlussseite mit Scan-Fortschritt oder direkt Startseite? | **Abschlussseite** wie im Mockup. Eine leere Startseite direkt nach dem Abschluss wirkt kaputt. |
| **O-14** | `CHECK` mit 23 Werten in der Datenbank? | **Nein**, Prüfung in Go an einer Stelle. Sonst wird jede neue Sprache ein Tabellen-Neubau. |
| **O-15** | Metadaten-Vorschlag je Sprache wie in Tabelle 4.1 | **Ja**, immer mit Englisch als zweiter Sprache. |
| **O-16** | Rechts nach links (ar, he, fa) | **Eigenes Konzept nach Stufe 4.** Bis dahin nur logische CSS-Eigenschaften für neuen Code. |

---

## Anhang A: Mockup

- Klickbarer Prototyp: `docs/planning/mockups/22-ersteinrichtung/prototyp.html`. Gleich als
  Artefakt: https://claude.ai/artifact/5cRAoZpb7Th38TABdVaqzr
- Drei Varianten zum Umschalten. **Gewählt ist A** (F-1). B und C bleiben zum Vergleich.
- Seit Version 3 folgt Variante A der Reihenfolge aus 3.2. Hardware und Zeitzone sind dort nur
  Anzeige, und das Freigabeland ist auf die fünf bekannten Länder beschränkt.
- Kopfzeile, Schrittnamen, Knöpfe und Willkommensseite sind im Mockup in de, de-DE, en, fr, es
  und it übersetzt. Die übrigen Sprachen fallen dort auf Englisch zurück. Die Schrittinhalte
  stehen nur auf Deutsch.
- Prüfungen im Mockup sind gespielt: `/media/…` gilt als gültig, `/mnt/nas/…` als Netzlaufwerk,
  ein TMDB-Schlüssel ab 11 Zeichen als gültig.
