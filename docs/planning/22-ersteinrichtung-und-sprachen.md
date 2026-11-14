# Konzept 22: Ersteinrichtung und Sprachen

- **Status: Fassung 3 (14. November 2026).** Fassung 2 hat das Kritiker-Review eingearbeitet
  (REVISE: 2 Blocker, 11 Major, 16 Minor; Abschnitt 9). In Fassung 3 hat Marco alle offenen
  Punkte beantwortet (Abschnitt 8), O-20 als letzten. Damit sind **F-1 bis F-16** entschieden
  (Abschnitt 1), und es ist **nichts mehr offen**.
  - Neu ist **Teil C «Freigabeländer»** (Abschnitt 4.11, F-12): Profile bekommen die Altersstufen
    des gewählten Freigabelands.
  - **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Auslöser:** Marco will einen Installations-Assistenten, der beim ersten Start erscheint und die
  wichtigsten Einstellungen abfragt, und Slouchcat in vielen Sprachen anbieten, «etwa so wie Emby
  und Jellyfin».
- **Verankert gegen:** `main` **`46d41072`** («Merge pull request #158», Release 1.32.0),
  Vertragsfassung **1.32.0** (`api/openapi.yaml:4`), letzte Migration **`00057_trailer_size.sql`**,
  letzte ADR **`0032`**, letzte Frage im Entscheidungsprotokoll **28**. Daneben offen: Zweig
  `claude/k21-stufen4-6` mit Vertrag **1.33.0** und den Migrationen **00058** und **00059**.
- **Nummern:** Dieses Papier ist **Konzept 22**. Es reserviert:
  - **Frage 29**
  - **ADR 0033** («Oberflächensprachen, Übersetzung und Metadaten-Sprachen»). Sie ersetzt ADR 0018
    in zwei Punkten (Abschnitt 6).
  - **ADR 0034** («Freigabeländer und Altersstufen je Land»). Sie ändert Entscheid 6 und AN-8.
  - Vertragsfassungen und Migrationsnummern nennt das Papier **nicht fest**. Vergeben wird in
    Bau-Reihenfolge: goose verlangt lückenlose Nummern, die Regel «Vertrag = Tag» eine aufsteigende
    Version.
  - Abschnitt 7 sagt je Stufe nur, **ob** sie Vertrag und Migration braucht.
- `:Zeile`-Angaben beziehen sich auf `46d41072`; **nach Namen suchen, nicht nach Zeile.** Pfade
  ohne Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App), wo es eindeutig ist.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/` (hier vor allem ADR 0005 Konten und Profile, ADR 0018 Sprachen, ADR 0024
  Updater). Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).

> **Dieses Papier ändert keine Zeile Code.** Es hat zwei Teile, die zusammengehören, aber
> getrennt gebaut werden:
> - **Teil A** ersetzt den heutigen Ein-Seiten-Assistenten durch einen geführten Assistenten in
>   neun Schritten (Abschnitt 3).
> - **Teil B** macht aus zwei Oberflächensprachen dreiundzwanzig und löst dafür die Bindung der
>   Metadaten an Deutsch (Abschnitt 4.1 bis 4.10).
> - **Teil C** nimmt die Freigabesysteme der Länder dieser Sprachen auf (Abschnitt 4.11).
>
> Abschnitt 7 schneidet alles in fünf Stufen mit je einem Release.

---

## 1. Was Marco will (entschieden am 14. November 2026)

| Nr. | Frage | Antwort |
|---|---|---|
| **F-1** | Welche Variante des Assistenten (Mockup A «Geführt», B «Zwei Wege», C «Pflicht + Karten»)? | **A «Geführt»:** alle Schritte in einer Leiste links. Pflicht sind Sprache, Konto und Bibliothek, die übrigen lassen sich überspringen; dann gilt die Empfehlung. |
| **F-2** | Was fragt der Assistent zusätzlich zu Sprache, Konto, erster Bibliothek und TMDB ab? | **Alles:** Region und Metadaten, Haushalt-Profile, weitere Dienste (TVDB, Fanart, YouTube, E-Mail), Server und Wiedergabe (öffentliche Adresse, Grafikchip, Trickplay, Update-Prüfung). |
| **F-3** | Wie wird Deutsch angeboten? | **Zwei Varianten:** Deutsch (Schweiz) mit ss wie heute und Deutsch (Deutschland) mit ß. |
| **F-4** | Wie viele Oberflächensprachen im ersten Schritt? | **Rund 22**, und zwar «natürlich und sinnvoll übersetzt», nicht wörtlich. |
| **F-5** | Wer übersetzt? | **Claude liefert die erste Fassung, Weblate wird für Korrekturen aus der Community geöffnet.** |
| **F-6** | Schlüssel im Assistenten | Bei jedem Schlüssel steht **der Link, wo man ihn beantragt**. |
| **F-7** | Muss die Metadaten-Sprachliste des Servers Deutsch enthalten (heute Pflicht, ADR 0018)? | **Nein, Deutsch ist keine Pflicht.** Ein französischer Haushalt lädt Französisch und Englisch, ohne Deutsch. |
| **F-8** | Was tut der Radio-Schalter im Assistenten? | **Land ins Angebot, keine Sender:** Modus «Nur markierte». Die Sender wählt man später in der Verwaltung. |
| **F-9** | PIN für weitere Profile im Assistenten | **Man kann eine PIN angeben, sie ist aber optional.** |
| **F-10** | Grafikchip im Assistenten | **Wählbar, mit Neustart.** Die Wahl wird gespeichert und gilt nach einem Neustart des Containers. |
| **F-11** | Abbruch nach Schritt 2 (O-3) | **Immer weiterleiten**, bei jedem Anmelden, bis die Einrichtung abgeschlossen ist. |
| **F-12** | Freigabeländer (O-5) | **Weitere Länder jetzt**, und zwar alle zu den 23 Sprachen. **Die Profile wählen aus den Stufen des gewählten Landes.** Hat ein Titel keine Freigabe dieses Landes, gilt die deutsche (FSK). |
| **F-13** | Weitere Dienste im Assistenten (O-11) | **Ja, alle:** dazu Google Books, Trakt, Web-Push und Telegram. Jede Karte zeigt, ob sie **empfohlen** oder **optional** ist. Empfohlen sind TMDB und E-Mail; über die übrigen durfte Claude entscheiden. |
| **F-14** | Alte öffentliche Prüf-Endpunkte (O-12) | **Sofort entfernen**, nicht nur als veraltet markieren. |
| **F-15** | Stichproben der Übersetzung durch Marco (O-9) | **Deutsch (Deutschland) und Englisch.** |
| **F-16** | Gespeicherte Freigaben nach einem Wechsel des Freigabelands (O-20) | **Automatisch neu lesen:** Wechselt das Freigabeland, liest Slouchcat alle Freigaben neu und darf dabei auch senken. Von Hand gesetzte Freigaben bleiben unberührt. |

Die übrigen offenen Punkte hat Marco nach Empfehlung beantwortet (Abschnitt 8). Die
dreiundzwanzig Sprachen aus F-3 und F-4 stehen in Abschnitt 4.1. Rechts nach links (Arabisch,
Hebräisch, Persisch) ist bewusst nicht dabei (Abschnitt 4.10).

---

## 2. Heute

### 2.1 Der Assistent

**Aufbau**
- `/setup` ist **eine Seite mit vier Blöcken** (`features/setup/SetupPage.tsx:264-427`):
  1. Hauptkonto
  2. erste Bibliothek (`LibraryFields`)
  3. TMDB-Schlüssel, optional
  4. Systemzustand (`SystemState`), nur Anzeige
- Ein Knopf schickt alles mit einem `POST /setup` (`runSetup`).
- Ob der Assistent erscheint, entscheidet allein `CountAccounts == 0`
  (`internal/auth/auth.go:135-141`). Die Weiche liegt in `app/router.tsx:156-160`.
- Ein angemeldeter Benutzer auf `/setup` wird nach `/` geschickt (`:160`).

**Ablauf von `RunSetup`** (`internal/admin/admin.go:329-384`)
1. Er prüft die Sprache, nur `de`/`en` (`:343`), dann Pfad und Schlüssel.
2. Er legt Konto und Profil in **einer** Transaktion an (`auth.go:181-245`).
3. Er speichert den TMDB-Schlüssel.
4. Er legt die Bibliothek an. Dabei wird der erste Scan schon eingereiht (`admin.go:94,610`).
5. Er meldet an. Die Sitzung hat dann das Hauptprofil.

- Scheitert die Bibliothek, bleiben Konto und Schlüssel bestehen (`admin.go:371-376`).
- Das erste Profil heisst wie der Teil der E-Mail vor dem @, hat Freigabe 18, die Sprache der
  Einrichtung und keine Farbe (`auth.go:226-230`).
- **Die Sprache wird nicht gefragt.** Sie kommt still aus dem Browser (`SetupPage.tsx:246`,
  `i18n/index.ts:13`).

**Prüfungen vor einem Konto**
- Es gibt nur zwei öffentliche Prüfungen: Pfad (`checkSetupPath`, nur innerhalb von
  `SLOUCHCAT_BROWSE_ROOTS`) und TMDB (`checkSetupTmdbKey`).
- TVDB, Fanart und YouTube prüft nur `checkProviderKey`. Das ist ein Admin-Endpunkt und prüft nur
  den **gespeicherten** Schlüssel (`internal/admin/settings.go:713-730`).
- Die Testmail (`sendTestEmail`) gibt es ebenfalls nur für Admins.
- Grenzen: Versuchstopf 10 je Minute und Adresse, gemeinsam mit Anmeldung, PIN und Einladung
  (`internal/auth/limits.go:15-16`), dazu die CSRF-Kopfzeile `X-Requested-With`.

**Anforderungen**
- Spec A-1 (`docs/spec/01-anforderungen-v1.md:144`) verlangt Konto, erste Bibliothek,
  GPU-Erkennung und Metadaten-Schlüssel.
- Plan 3.4 (`docs/planning/03-plan-v1.md:158-160`) verlangt Warnungen für Netzlaufwerke und das
  inotify-Limit.
- `docs/INSTALLATION.md:252-271` beschreibt genau diese vier Blöcke. Profile, Freigaben und
  Einladungen kommen dort «später in der Verwaltung».

### 2.2 Wo die Einstellungen heute liegen

| Thema | Endpunkt heute | Gespeichert | Quelle | Anmerkung |
|---|---|---|---|---|
| Oberflächensprache | `createProfile`/`updateProfile` | `profiles.language` | DB | nur `de`/`en`, Abschnitt 2.3 |
| Freigabeland | `updateMetadataSettings` | `metadata.rating_country` | DB | nur DE, AT, CH, GB, US (`openapi.yaml:13327`) |
| Metadaten-Sprachen | `updateMetadataSettings` | `metadata.text_langs`, `metadata.image_langs.<art>` | DB | Vorgabe `de`, `en`. **Serverliste muss `de` enthalten** (`metadata/settings.go:264`, ADR 0018 F-9). Die Bibliotheksliste braucht `de` oder `en` (`:266`). |
| Zeitzone | **keiner** | – | nur `TZ` (`deploy/compose.yaml:42`, Vorgabe Europe/Zurich) | Jobs nutzen sie (`jobs/scheduler.go:146`) |
| Radio-Angebot | `applyRadioOffer` mit `version` und `steps` | `radio_offer_countries` | DB | `chooseCountry` setzt das Land in den Modus «Alle» (`openapi.yaml:19866`). Der Katalog ist bei einer neuen Installation die ersten 10 Minuten leer (`radio/jobs.go:26`). |
| Schlüssel TMDB, TVDB (+PIN), Fanart (+Client), YouTube | `updateAdminSettings.keys` | verschlüsselte Settings | **Umgebung gewinnt** (`env_wins`, `admin/settings.go:509-526`) | Prüfen mit `checkProviderKey`, nur den gespeicherten Wert |
| E-Mail (SMTP) | `updateAdminSettings.email` | `smtp_*` | Umgebung gewinnt | Testmail mit `sendTestEmail` |
| Öffentliche Adresse | `updateAdminSettings.publicUrl` | `public_url` | Umgebung gewinnt (`SLOUCHCAT_PUBLIC_URL`) | |
| Update-Prüfung | `updateAdminSettings.updateCheckEnabled` | `update_check_enabled` | DB | Vorgabe ein |
| Grafikchip | **keiner** | – | nur `SLOUCHCAT_HW` (auto/nvenc/qsv/vaapi/software, Vorgabe auto, `config/config.go:208`) | Wird beim Start gelesen (`cmd/slouchcat/main.go:690`). auto probiert NVENC, QSV, VA-API (`transcode/backend.go:71-160`). Ein fest gewähltes Backend, das nicht geht, ist ein Startfehler. |
| Trickplay | `createLibrary`/`updateLibrary` | `libraries.trickplay_*` | DB | **nur je Bibliothek**, Vorgabe aus |
| Weitere Profile | `createProfile` | `profiles` | DB | höchstens 10; braucht das aktive Hauptprofil (`auth/profiles.go:101-103`); PIN über `ProfileRequest.pin` (4–8 Ziffern) |
| Bibliotheken | `createLibrary` | `libraries`, `library_roots` | DB | Arten video, audiobooks, podcasts; reiht den ersten Scan ein |

- `MetadataSettingsUpdate` ersetzt **alle** Metadaten-Settings auf einmal (`openapi.yaml:13366`).
  Wer nur Sprachen ändern will, muss lesen, ändern und das Ganze zurückschreiben.
- Den Systemzustand liefert `getSystemInfo` (`GET /admin/system`, `openapi.yaml:6926`). Vor dem
  Konto steht er öffentlich in `SetupStatus.system` (`:10793`).
- Der Vertrag kennt neben `admin` die Sichtbarkeit **`owner`** (`api/session.go:122`): nur der
  Eigentümer mit Hauptprofil. `CLAUDE.md` Regel 7 nennt sie noch nicht.

Die Verwaltung hat für alles fertige Bauteile:
- `features/admin/integrations/`: `ProviderCard`, `EmailForm` mit Testmail, `ServerBlock`,
  `SourceChip`
- `features/shared/`: `LibraryFields`, `PathBrowserButton`, `SystemState`
- `features/admin/libraries/TrickplayFields`
- `features/profiles/ProfileForm` mit `AccentPicker` und `PinField`

Die Links zum Beantragen der Schlüssel stehen schon im Code:

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
- Geladen wird **nur das angefragte Bündel** (`:23-33`). `fallbackLng: 'en'` wirkt nur, wenn
  Englisch schon geladen ist. Sonst zeigt i18next für einen fehlenden Text den **Schlüssel**.
- `de.json` und `en.json` haben je **4318 Texte** in 36 Bereichen.
- Plural im Stil von i18next v4: 187× `_one`, 188× `_other`, 4× `_zero`, kein `_few`/`_many`.
  `admin.home.browse.hanging` hat nur `_other`.
- `locales/locales.test.ts` prüft nur zwei Dinge: gleiche Schlüssel in de und en, und kein `ß`
  in `de.json`.
- `app/session.ts:48` ruft `setLanguage` mit der Profilsprache ohne Prüfung auf. Ein
  unbekannter Code wirft dort einen Fehler.

**Datenbank, Vertrag und Prüfstellen**
- `profiles.language TEXT NOT NULL DEFAULT 'de' CHECK (language IN ('de','en'))`
  (`migrations/00002_accounts_sessions.sql:32`).
  - Spätere Migrationen haben rund zehn Spalten mit eigenem `CHECK` angefügt (`00007`, `00011`,
    `00013`, `00028`, `00032`).
  - Die Sicht `visible_items` liest `FROM profiles` (zuletzt `00056_library_deleting.sql`).
  - Der Trigger `activity_events_profile_deleted` hängt an der Tabelle (`00031`).
  - Indizes: `profiles_account` und `profiles_primary`.
- Im Vertrag ist `Language` `enum: [de, en]` (`openapi.yaml:11420`). Er wird an sechs Stellen
  verwendet: Setup, Profil, Profil anlegen/ändern, Einladung annehmen, Offline-Profil.
- **Diese Stellen kennen nur `de`/`en`:**

| Stelle | Was dort passiert |
|---|---|
| `admin/admin.go:343` | `RunSetup` lehnt andere Sprachen ab |
| `auth/auth.go:191`, `auth/invitations.go:265`, `auth/profiles.go:297` | Prüfung, leer wird `de` |
| `visibility/visibility.go:53` | Jede andere Sprache wird für die Metadaten **zu `de`** |
| `notify/texts.go:88-91` | Rückfall auf `de` |
| `queries/system/notify_channels.sql:36,47` | `COALESCE(…, 'de')` |
| `auth/playback_prefs.go:57-62` | `initialPrefs` kopiert den Code in Ton- und Untertitelsprachen. `trackpick.Valid` nimmt nur Zweibuchstaben-Codes (`trackpick/languages.go:237-247`). |
| `web/src/features/admin/item/TrailersTab.tsx:36` | Verzweigung `de`/`en` |
| `web/src/features/catalog/labels.ts:70` | Verzweigung `de`/`en` |
| `model/model.go:192,354` | nur Kommentare |

- Konten haben keine Sprache. Meldungen ans Konto nehmen die Sprache des Hauptprofils.
- Sammlungen haben nur `name` und `name_de` (`visibility/discover.go:231`).

**Server-Texte**
- Eine Go-Map mit 34 Texten je Sprache in `notify/texts.go:12-92`. Rückfall ist `de`.
- Die HTML-Mail hat kein `lang`-Attribut (`:124-137`).
- Ankündigungen schreibt der Admin selbst; sie werden nicht übersetzt.
- API-Fehler tragen einen festen `code`. Die Web-App übersetzt ihn selbst.

**Formate und Schrift**
- Alle `Intl`-Aufrufe nehmen `i18n.language`, also `de`. Das ergibt **deutsche** Zahlen
  (`1.234.567,5`), nicht schweizerische (`1’234’567.5`).
- Nur die Testkonfigurationen setzen `de-CH` (`web/playwright.config.ts:136`,
  `web/storybook.config.ts:33`).
- Schrift: nur der Systemschriftstapel (`packages/design-tokens/tokens/typography.json:5-17`),
  keine Webfonts, keine `:lang()`-Regeln. In der CI rendert Storybook mit DejaVu.

**Tests**
- e2e: 88 von 91 Specs wählen Elemente über deutsche Texte, mit rund 3385 Selektoren.
- Storybook: 502 Vergleichsbilder.
- Der e2e-Aufbau legt den Eigentümer mit einem echten `POST /setup` samt Bibliothek an
  (`web/e2e/global-setup.ts:302-318`). `web/e2e/setup.spec.ts` prüft die heutige Seite.

**Frühere Entscheide**
- Spec C-3 (`01-anforderungen-v1.md:139`): Oberfläche Deutsch und Englisch.
- ADR 0018 hat 2026 zwei Dinge festgelegt:
  - F-1: keine neuen Oberflächensprachen (`docs/adr/0018-…md:60-61`). Grund war der Aufwand von
    rund 2300 Texten je Sprache.
  - F-9: die Serverliste der Metadaten-Sprachen muss `de` enthalten (`:85`).
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
2. **Danach als Eigentümer:** Die Schritte 3 bis 9 nutzen die **vorhandenen** Endpunkte aus
   Abschnitt 2.2. Jeder Schritt speichert beim «Weiter». Prüfen heisst speichern und dann prüfen:
   `updateAdminSettings`, danach `checkProviderKey`. Für die Mail heisst es speichern und dann
   `sendTestEmail`.

Was das bringt:
- **Weniger öffentliche Fläche als heute:** `checkSetupPath` und `checkSetupTmdbKey` fallen weg
  (F-14). Neu öffentlich ist nur die Zeitzone in `SetupStatus.system` (3.5).
- **Gleiche Rechteprüfung wie in der Verwaltung.** Ab Schritt 3 gilt sie für alles.
- **Volle Ordnerauswahl.** Ab Schritt 3 darf der Assistent den ganzen Ordner-Browser
  (`PathBrowserButton`) zeigen. Vor dem Konto gibt es nur die Wurzeln, aus Sicherheitsgründen
  (`SetupPage.tsx:28-34`).
- **Abbruch ist harmlos.** Was gespeichert ist, bleibt, auch die Trickplay-Vorgabe (3.4). Der
  Assistent setzt beim nächsten Anmelden dort fort.
- **Gleiche Bauteile wie die Verwaltung.** Regel 9 verlangt, dass Admin und Zuschauer dieselben
  Komponenten nutzen. `ProviderCard`, `EmailForm`, `LibraryFields`, `TrickplayFields`,
  `ProfileForm` und `PinField` werden eingebettet, nicht nachgebaut.
- **Umgebungsvariablen werden respektiert.** Ein Wert aus `deploy/.env` erscheint mit dem Hinweis
  «aus .env» (`SourceChip`) und ist nicht änderbar, wie in der Verwaltung.

**Wer zuerst kommt, richtet ein.** Das gilt heute schon. Ruft jemand anderes `/setup` zwischen
Schritt 1 und 2 auf und legt das Konto zuerst an, bekommt Schritt 2 `setup_done`. Der Assistent
zeigt dann «Slouchcat ist schon eingerichtet» mit dem Weg zur Anmeldung (`ErrSetupDone`). Ein
Einrichtungs-Token gegen dieses Risiko gibt es bewusst nicht (O-17).

### 3.2 Reihenfolge der Schritte

Die Reihenfolge weicht vom ersten Mockup ab. Das Mockup ist seit Version 3 nachgeführt.
**Dienste und Wiedergabe kommen vor die Bibliotheken.** `createLibrary` reiht den ersten Scan
sofort ein, also sollen Schlüssel, Metadaten-Sprachen und Trickplay-Vorgabe dann schon stehen.

| # | Schritt | Pflicht | Phase | Speichert über | Überspringen heisst |
|---|---|---|---|---|---|
| 1 | Sprache | ja | öffentlich, danach Eigentümer | im Browser; nach Schritt 2 `updateProfile` | – |
| 2 | Konto | ja | öffentlich | `runSetup` (erweitert, 3.5) | – |
| 3 | Region | nein | Eigentümer | `updateMetadataSettings` (ganzes Objekt), `applyRadioOffer` | Empfehlung aus der Sprache (Tabelle 4.1) |
| 4 | Dienste | nein (TMDB und E-Mail empfohlen) | Eigentümer | `updateAdminSettings.keys/.email/.telegram`, `checkProviderKey`, `sendTestEmail`, Web-Push-Abo des Geräts | keine Schlüssel; Slouchcat zeigt nur Dateinamen |
| 5 | Wiedergabe | nein | Eigentümer | `updateAdminSettings.transcodeHw` (neu), Trickplay-Vorgabe in `setup_wizard` | automatische Wahl, Trickplay aus |
| 6 | Bibliotheken | ja, mindestens eine | Eigentümer | `createLibrary` | – |
| 7 | Haushalt | nein | Eigentümer | `createProfile` (Altersstufen des Freigabelands, ab Stufe 3) | nur das eigene Profil |
| 8 | Zugriff | nein | Eigentümer | `updateAdminSettings.publicUrl/.updateCheckEnabled` | keine Adresse, Update-Prüfung ein |
| 9 | Zusammenfassung | – | Eigentümer | `finishSetup` (neu, 3.5) | – |

- In der Leiste links ist jeder Schritt anklickbar, sobald Schritt 2 erledigt ist. Vorher sind nur
  1 und 2 erreichbar.
- Jeder Schritt hat einen Zustand: offen, erledigt (Haken) oder übersprungen (gestrichelter
  Kreis). Die Zusammenfassung zeigt «Eingerichtet» oder «Empfehlung».

### 3.3 Die Schritte im Einzelnen

**Schritt 1: Sprache.**
- Ein Raster mit allen wählbaren Sprachen (4.4), jede in **ihrer eigenen Sprache** («Français»,
  «日本語»), darunter der englische Name und der Code.
- Ein Suchfeld findet beides, also «Japanese» wie «日本語».
- Vorgewählt ist die erkannte Sprache (4.2). Ein Klick schaltet die ganze Oberfläche sofort um.
- Darunter eine Vorschau mit echtem Datum und echter Zahl in der gewählten Sprache. Bei Deutsch
  (Schweiz) steht `1’234’567.5` (4.4, `intlLocale`).
- Oben rechts steht auf `/setup` und `/login` ein Sprachknopf. Damit lässt sich die Sprache vor
  der Anmeldung wechseln, nicht nur im Assistenten.

**Schritt 2: Konto.**
- Felder: E-Mail, Passwort zweimal (mindestens 10 Zeichen wie heute), **Name und Farbe des
  ersten Profils**.
- Heute heisst das Profil wie der Teil der E-Mail vor dem @. Neu wählt man beides. Die Farbe kommt
  aus `AccentPicker` mit den 8 Akzent-Tokens.
- Die Profilsprache ist die aus Schritt 1.
- «Weiter» ruft `runSetup` auf. Ab hier ist man angemeldet.

**Schritt 3: Region.**
- **Land:** Vorschlag aus der Sprache, sonst aus der Zeitzone des Browsers. Es dient als
  Vorschlag fürs Radio und für die Metadaten-Sprachen.
- **Altersfreigaben nach:** das Freigabeland. Vorschlag aus dem Land, sonst Deutschland.
  - In Stufe 2 sind es die fünf Länder von heute (DE, AT, CH, GB, US).
  - Ab Stufe 3 sind es alle Länder aus Tabelle 4.11 (F-12).
  - Unter der Wahl stehen die Altersstufen dieses Landes, so wie sie danach im Schritt Haushalt
    erscheinen.
- **Sprachen der Metadaten:** geordnete Liste aus `knownLanguages` (ADR 0018), Vorschlag nach
  Tabelle 4.1.
  - **Keine Sprache ist Pflicht (F-7).** Ein französischer Haushalt bekommt `fr → en`.
  - Die Bildsprachen folgen der Textliste.
  - Der Assistent liest die Metadaten-Settings, ändert Freigabeland und Sprachen und schreibt das
    ganze Objekt zurück. Das verlangt `MetadataSettingsUpdate`.
- **Zeitzone des Servers:** nur Anzeige. Sie kommt aus `TZ`; dafür gibt es weder Setting noch
  Endpunkt.
  - Der Assistent vergleicht sie mit der Zeitzone des Browsers
    (`Intl.DateTimeFormat().resolvedOptions().timeZone`).
  - Weichen beide ab, zeigt er die Zeile für `deploy/.env` und den Hinweis auf einen Neustart.
  - Dafür bekommt `SystemInfo` das neue Feld `timeZone`.
- **Radio-Angebot (F-8):** Schalter «<Land> ins Radio-Angebot aufnehmen», Vorgabe ein.
  - Er ruft `applyRadioOffer` mit der aktuellen `version` und den Schritten
    `[chooseCountry, demarkAll]` auf. Das Land steht damit im Modus «Nur markierte», ohne Sender
    und ohne Logo-Downloads.
  - Die Sender markiert man später unter Verwaltung › Radio.
  - Ist der Katalog noch leer (erste 10 Minuten), merkt sich der Assistent den Wunsch in
    `setup_wizard` und führt ihn bei `finishSetup` aus. Ist der Katalog dann immer noch leer,
    erscheint in der Verwaltung ein Hinweis.

**Schritt 4: Dienste (F-6, F-13).**
- Acht Karten in zwei Gruppen. Jede Karte trägt ein Schild **«Empfohlen»** oder **«Optional»** und
  einen Satz, wofür der Dienst gut ist.
- **Empfohlen:**
  - **TMDB:** Titel, Beschreibungen, Poster, Besetzung. Ohne TMDB zeigt Slouchcat nur Dateinamen.
  - **E-Mail:** für Einladungen und «Passwort vergessen». Ein aufklappbarer Block aus `EmailForm`
    mit Server, Port, Verschlüsselung, Benutzer, Passwort, Absender und «Testmail an mich senden».
- **Optional** (zugeklappt, ein Klick öffnet die Felder):
  - **TheTVDB** mit PIN: bessere Staffel- und Folgendaten, vor allem Anime.
  - **Fanart.tv** mit optionalem Client-Schlüssel: Logos und Hintergründe in höherer Auflösung.
  - **YouTube:** Trailer, wo TMDB keine liefert, mit dem Hinweis «Tageslimit 100 Suchen».
  - **Google Books:** Angaben und Cover für Hörbücher.
  - **Trakt:** der Schlüssel für Listen von Trakt (Spotlight). Ein Trakt-Login bleibt ein
    Nicht-Ziel.
  - **Web-Push:** «Meldungen auf diesem Gerät einschalten». Das abonniert den Browser des
    Eigentümers für Push. Die Schlüssel dafür erzeugt der Server beim ersten Start selbst.
  - **Telegram:** Bot-Token von @BotFather. Slouchcat prüft es beim Speichern mit Telegram
    (`getMe`). Das eigene Konto verbindet man danach über den Link aus der Verwaltung.
- **Links zum Beantragen (F-6)** stehen auf jeder Karte und sind sichtbar, bevor man sie aufklappt.
  Es sind die Adressen aus `ProviderCard.PROVIDERS`:
  - Trakt: `https://developer.trakt.tv/apps`
  - Google Books: `https://console.cloud.google.com/apis/credentials`
  - Telegram: `https://t.me/BotFather`
- «Prüfen» speichert den Schlüssel und ruft dann `checkProviderKey` auf. Für Trakt und Google
  Books gibt es diese Prüfung schon.
- **Lehnt der Dienst ab:** Die Meldung steht neben dem Feld. «Weiter» fragt dann «Ungültigen
  Schlüssel entfernen und weiter?». So bleibt kein falscher Schlüssel gespeichert, ohne dass man
  es merkt.
- Jede Karte zeigt «aus .env», wenn die Umgebung den Wert setzt.
- Die Karten sind dieselben Bauteile wie unter Verwaltung › Integrationen (`ProviderCard`,
  `EmailForm`, `TelegramForm`, Push-Schalter aus den Benachrichtigungen).

**Schritt 5: Wiedergabe (F-10).**
- Anzeige aus `SystemState`: Grafikchip, Backend, Hardware-Kodierung, HDR→SDR, inotify-Limit mit
  Warnung über 80 %.
- **Wahl des Backends** als Liste von Knöpfen:
  - «Automatisch (empfohlen)»
  - jedes Backend, dessen Probe beim Start **gelungen** ist (`SystemInfo.gpu.candidates`, neu,
    3.5)
  - «Nur Prozessor»
  - Backends, deren Probe scheiterte, stehen ausgegraut daneben, mit dem Grund («Gerät
    /dev/dri fehlt» …). So kann man nichts wählen, was den Start bricht.
- **Gespeichert** wird die Wahl als neues Setting `transcode_hw` (`updateAdminSettings.transcodeHw`).
- **Vorrang beim Start:**
  1. `SLOUCHCAT_HW`, wenn die Umgebung es **ausdrücklich** setzt, dann «aus .env» im Assistenten
     (`os.LookupEnv`, heute nicht unterscheidbar, weil die Vorgabe `auto` ist)
  2. das Setting
  3. `auto`
- **Sicherung gegen eine Neustart-Schleife:** Scheitert ein über das Setting gewähltes Backend beim
  Start, nimmt der Server `auto`, schreibt eine Warnung ins Log und zeigt einen Hinweis auf dem
  Verwaltungs-Dashboard.
  - Bei einem Wert aus `.env` bleibt es beim heutigen Startfehler. Wer `.env` setzt, will es
    genau so.
- **Neustart:** Weicht die Wahl vom laufenden Backend ab, sagt der Schritt «Gilt nach einem
  Neustart. Slouchcat startet beim Abschliessen einmal neu.»
  - `finishSetup` antwortet zuerst. Dann beendet sich der Server geordnet, und Docker startet ihn
    neu (`restart: unless-stopped`, `deploy/compose.yaml:28`).
  - Die Abschlussseite wartet auf `/api/v1/healthz` und macht dann weiter.
  - Ohne Restart-Policy kommt nach 60 Sekunden der Hinweis «Container von Hand starten» (O-18).
- **Die gleiche Wahl** kommt in die Verwaltung (Systemseite), damit das Setting nicht nur im
  Assistenten änderbar ist.
- **Trickplay-Vorgabe:** ein Schalter, Vorgabe aus. Er belegt im nächsten Schritt die
  Trickplay-Felder jeder neuen Video-Bibliothek vor. Gespeichert wird er in `setup_wizard`, damit
  er einen Abbruch übersteht.

**Schritt 6: Bibliotheken.**
- Je Bibliothek eine Karte aus `LibraryFields`: Name, Art, Ordner mit `PathBrowserButton` und
  «Ordner prüfen», Freigabe, bei Video `TrickplayFields`.
- «+ Weitere Bibliothek» fügt eine Karte an.
- Die Wurzeln aus `SLOUCHCAT_BROWSE_ROOTS` stehen als Knöpfe unter dem Feld wie heute. Die Warnung
  für Netzlaufwerke (Plan 3.4) bleibt.
- «Weiter» legt jede neue Karte mit `createLibrary` an. Das reiht den Scan sofort ein, wie heute.
- Mindestens eine Bibliothek ist Pflicht. «Weiter» und `finishSetup` lehnen null Bibliotheken ab.

**Schritt 7: Haushalt.**
- Liste der Profile, oben das eigene.
- «Profil hinzufügen» mit Name, Altersstufe, Farbe und einem **optionalen
  PIN-Feld** (`PinField`, 4–8 Ziffern, F-9). Die PIN geht mit `ProfileRequest.pin` zusammen mit dem
  Profil an den Server.
- Die Altersstufen sind die des Freigabelands aus Schritt 3: in Stufe 2 0/6/12/16/18, ab Stufe 3
  die Stufen des Landes (4.11).
- Die Sprache neuer Profile ist die des Eigentümers. Höchstens 10 Profile.
- Ein Hinweis nennt «Verwaltung › Konten» für Gäste mit eigenem Konto. Einladungen gehören nicht in
  den Assistenten.

**Schritt 8: Zugriff.**
- Öffentliche Adresse, mit «aus .env», falls gesetzt. Darunter die Adresse, unter der man gerade
  zugreift, als Vorschlag.
- Der Hinweis erklärt, wozu die Adresse dient: Einladungen, E-Mails, Push.
- Schalter «Täglich nach neuen Versionen suchen», Vorgabe ein, mit dem Satz «Slouchcat sendet
  keine Nutzungsdaten».
- Eine Prüfung, ob die Adresse von aussen erreichbar ist, kommt nicht in die erste Fassung (O-4).

**Schritt 9: Zusammenfassung.**
- Je Schritt die gewählten Werte und «Ändern».
- «Einrichtung abschliessen» ruft `finishSetup` auf. Danach kommt eine Abschlussseite mit dem
  Scan-Fortschritt je Bibliothek aus der Jobliste und dem Knopf «Zur Startseite» (O-13).
- Ist ein Neustart fällig (Schritt 5), zeigt die Abschlussseite zuerst «Slouchcat startet neu …».

### 3.4 Fortschritt, Abbruch und bestehende Installationen

- **Der Fortschritt liegt auf dem Server** im neuen Setting `setup_wizard` (JSON):
  - je Schritt `open`, `done` oder `skipped`
  - die Trickplay-Vorgabe
  - einen offenen Radio-Wunsch
  - `finishedAt`
- **Grundregel: Fehlt `setup_wizard`, gilt die Einrichtung als abgeschlossen.**
  - `runSetup` **ohne** `library` (neuer Assistent) schreibt ein offenes `setup_wizard`.
  - `runSetup` **mit** `library` (alte Aufrufer, `e2e/global-setup.ts`) schreibt keins.
  - Dadurch braucht es **keine Datenmigration**. CT 190 und jede bestehende Installation sehen den
    Assistenten nie, und der e2e-Aufbau wird nicht in den Assistenten umgeleitet.
- **Abbruch nach Schritt 2 (F-11):**
  - `SessionInfo` bekommt `setupPending` (nur für den Eigentümer gesetzt, 3.5).
  - Ist es wahr, leitet die Web-App **bei jedem Anmelden** auf den ersten offenen Schritt weiter,
    bis die Einrichtung abgeschlossen ist. Offline leitet sie nicht weiter.
  - «Später fortsetzen» in der Leiste führt für diese Sitzung zur Startseite, ohne abzuschliessen.
    Bis zum nächsten Anmelden zeigt `/admin` ein Band «Einrichtung fortsetzen».
  - Ohne Bibliothek steht die Startseite leer, und das Band erklärt, warum.
- **Andere Konten:** Der Assistent gehört dem Eigentümer. Ein Gast, ein eingeladenes Konto oder ein
  weiterer Administrator sieht ihn nie.

### 3.5 Vertrag und Server

Alle Änderungen sind **additiv**, ausser dem Wegfall der öffentlichen Prüf-Endpunkte (F-14). Eine Vertragsfassung kommt in Stufe 2.

- **`SetupRequest`:**
  - neu `profileName` (optional, 1–60) und `accent` (optional, Akzent-Token)
  - `library` wird **optional**. Mit `library` bleibt alles wie heute (3.4).
  - `tmdbApiKey` bleibt optional.
  - `language` folgt dem neuen `Language` aus Teil B.
- **Neu, `x-visibility: owner`** (der Eigentümer mit Hauptprofil):
  - `getSetupProgress` (`GET /admin/setup`)
  - `updateSetupProgress` (`PUT /admin/setup`): ein Schritt und sein Zustand, dazu die
    Trickplay-Vorgabe und der Radio-Wunsch
  - `finishSetup` (`POST /admin/setup/finish`): prüft «mindestens eine Bibliothek»
    (422 `library_required`), führt einen offenen Radio-Wunsch aus, setzt `finishedAt` und löst bei
    Bedarf den Neustart aus (Schritt 5)
  - Jede Operation bekommt einen Eintrag im Vertragstest.
  - `CLAUDE.md` Regel 7 bekommt `owner` in die Liste.
- **`SessionInfo.setupPending`:** boolesch, nur beim Eigentümer vorhanden.
- **`SystemInfo`:**
  - `timeZone`: die von `admin.ResolveZone` aufgelöste Zone. Sie ist auch vor dem Konto in
    `SetupStatus.system` lesbar. Das ist vertretbar, weil die Zone nichts Geheimes ist.
  - `gpu.candidates`: je Backend `{backend, ok, reason}` aus den Proben beim Start.
- **`AdminSettings` / `AdminSettingsUpdate`:**
  - `transcodeHw` (`auto`, `nvenc`, `qsv`, `vaapi`, `software`) mit `transcodeHwSource`
    (`env`/`stored`/`none`)
  - `env_wins` gilt wie bei den Schlüsseln.
- **`checkSetupTmdbKey` und `checkSetupPath` fallen weg (F-14).** Aus demselben Grund fällt
  `listSetupRoots` weg; ihn braucht der Assistent ebenfalls nicht mehr.
  - Der neue Assistent braucht die ersten beiden nicht mehr. Er zeigt die Wurzeln über den
    Ordner-Browser des Eigentümers.
  - Das ist eine **brechende** Änderung des Vertrags. Sie trifft aber nur eine Web-App, die noch
    den alten Assistenten im Cache hat, während der Server neu und uneingerichtet ist. Das kommt
    praktisch nicht vor, weil eine neue Installation noch keine alte PWA kennt.
  - Wie bei früheren brechenden Fassungen (1.2.0) bleibt es eine Minor-Fassung mit Vermerk in den
    Release-Notes.
  - Gestrichen werden auch die Einträge im Vertragstest und im Ratenlimit-Topf für
    `/setup/roots`.
- **Keine Migration** für Teil A. `setup_wizard` und `transcode_hw` sind Zeilen in der
  bestehenden Settings-Tabelle.

### 3.6 Web-App und Tests

**Routen**
- `/setup/$step` für alle neun Schritte.
- Ohne Konto sind nur `language` und `account` erreichbar.
- Mit Konto ist nur der Eigentümer zugelassen. `/setup/language` ändert dann die Profilsprache.
- Die heutige Regel «angemeldet auf `/setup` → `/`» (`router.tsx:160`) gilt nur noch, wenn
  `setupPending` falsch ist.

**Aufbau**
- Die Leiste ist ein `SideNav`-Muster wie in der Verwaltung. Bei schmaler Breite wird sie zur
  waagrechten Schrittleiste oben, wie im Mockup.
- Der Fuss hat «Zurück», «Überspringen» (nur bei freiwilligen Schritten) und «Weiter» bzw. im
  letzten Schritt «Einrichtung abschliessen».
- `features/setup/SetupPage.tsx` wird aufgeteilt in `SetupLayout`, je Schritt eine Datei und eine
  gemeinsame Zustandsquelle aus `getSetupProgress`.

**Tests**
- Jeder Schritt bekommt seinen **Playwright-Tastaturtest** (Regel 8): Tab-Reihenfolge Leiste →
  Inhalt → Fuss, Enter auf «Weiter», sichtbarer Fokus, keine Fokusfalle. Nach dem
  Schrittwechsel liegt der Fokus auf der Überschrift.
- `web/e2e/setup.spec.ts` wird für den neuen Ablauf neu geschrieben.
- `global-setup.ts` bleibt, wie er ist (3.4). Er ruft nur `POST /setup`.
- Go-Tests:
  - `runSetup` mit und ohne `library`
  - `finishSetup` ohne Bibliothek
  - Vorrang `SLOUCHCAT_HW` vor Setting vor `auto`
  - Rückfall auf `auto` bei gescheiterter Probe eines gespeicherten Backends

### 3.7 Was der Assistent bewusst nicht tut

- Er setzt keine Zeitzone. Sie kommt aus `TZ` und gilt erst nach einem Neustart; der Assistent
  zeigt nur die Zeile für `.env`.
- Er richtet keine Einladungen ein und verbindet kein Konto mit Telegram (F-13: nur der Bot).
- Er legt keine Podcasts an und markiert keine Radiosender.
- Er ändert nichts an `deploy/.env`. Der Server schreibt diese Datei nicht.

---

## 4. Teil B: Dreiundzwanzig Oberflächensprachen

### 4.1 Die Sprachen

| Code | Name (eigene Sprache) | Pluralformen (CLDR) | Schrift | Vorschlag Metadaten (F-7, O-15) | Freigabeland (Vorschlag, 4.11) |
|---|---|---|---|---|---|
| `de` | Deutsch (Schweiz) | one, other | Latein | de → en | CH |
| `de-DE` | Deutsch (Deutschland) | one, other | Latein | de → en | DE |
| `en` | English | one, other | Latein | en | US |
| `fr` | Français | one, many, other | Latein | fr → en | FR |
| `es` | Español | one, many, other | Latein | es → en | ES |
| `it` | Italiano | one, many, other | Latein | it → en | IT |
| `nl` | Nederlands | one, other | Latein | nl → en | NL |
| `pt-BR` | Português (Brasil) | one, many, other | Latein | pt-BR → en | BR |
| `pt-PT` | Português (Portugal) | one, many, other | Latein | pt-PT → en | PT |
| `pl` | Polski | one, few, many, other | Latein | pl → en | PL |
| `sv` | Svenska | one, other | Latein | sv → en | SE |
| `da` | Dansk | one, other | Latein | da → en | DK |
| `nb` | Norsk bokmål | one, other | Latein | nb → en (Schritt 0: `nb` oder `no` bei TMDB) | NO |
| `fi` | Suomi | one, other | Latein | fi → en | FI |
| `cs` | Čeština | one, few, many, other | Latein | cs → en | CZ |
| `hu` | Magyar | one, other | Latein | hu → en | HU |
| `tr` | Türkçe | one, other | Latein (ı/İ) | tr → en | TR |
| `ru` | Русский | one, few, many, other | Kyrillisch | ru → en | RU |
| `uk` | Українська | one, few, many, other | Kyrillisch | uk → en | UA |
| `ja` | 日本語 | other | CJK | ja → en | JP |
| `ko` | 한국어 | other | Hangul | ko → en | KR |
| `zh-CN` | 简体中文 | other | CJK vereinfacht | zh-CN → en | DE (China hat kein System) |
| `zh-TW` | 繁體中文 | other | CJK traditionell | zh-TW → en | TW |

- Die Pluralformen sind die, die `Intl.PluralRules(<code>).resolvedOptions().pluralCategories`
  liefert. i18next fragt genau diese ab. Fehlt eine, zeigt es den Schlüssel, nicht `_other`.
  Abschnitt 4.9 macht daraus einen Test.
- Die vier `_zero`-Texte (`de.json:4131-4151`) gelten als Sonderfall für die Zahl 0 und werden in
  jeder Sprache übersetzt.
- `knownLanguages` führt `nb` und `no` (`metadata/languages.go:22`). Welcher Code bei TMDB Texte
  liefert, prüft Schritt 0 der Stufe 4.

### 4.2 Sprachcodes, Deutsch in zwei Varianten, Erkennung, Laden

**`de` bleibt der Code für Deutsch (Schweiz).** Er ist heute in jeder Datenbank, jedem
Offline-Profil und jedem Vertrag gespeichert. Ein Umbenennen in `de-CH` würde den Vertrag brechen
und jede Installation migrieren, ohne dass jemand etwas davon hat.

**`de-DE` ist eine Überlagerung.** `locales/de-DE.json` enthält **nur** die Texte, die in
Deutschland anders lauten. Die Auflösung geht `de-DE` → `de` → `en`; i18next kann das ohne
`nonExplicitSupportedLngs`.
- Eine Auszählung am Anker findet rund **185** betroffene Texte: 53 mit ss-Wörtern, die mit ß
  geschrieben werden («Strasse», «gross», «schliessen» …), und 133 mit «Guillemets», die in
  Deutschland „so“ stehen.
- Ersetzt wird **nicht mechanisch**: «dass», «Klasse» und «Passwort» bleiben. Claude erstellt die
  Fassung, und ein Test prüft, dass jeder Text in `de-DE.json` sich vom Text in `de.json`
  unterscheidet.

**Laden der ganzen Kette.** Heute lädt `ensureLanguage` nur das angefragte Bündel (2.3). Neu lädt
sie **die ganze Kette**: die Sprache, bei `de-DE` zusätzlich `de`, und immer `en`. Sonst sähe
`de-DE` rund 4130 rohe Schlüssel, und ein fehlender Text in `main` zwischen zwei Releases wäre ein
Schlüssel statt Englisch.
- Englisch wird dafür nach dem ersten Bild im Hintergrund geladen, damit der Start nicht langsamer
  wird.

**Erkennung** (`detectLanguage`, in dieser Reihenfolge):
1. Ganzer Code passt: `pt-BR`, `zh-TW`, `de-DE` …
2. Bekannte Zuordnung:
   - `de-AT`, `de-LU`, `de-BE` → `de-DE`
   - `de-CH`, `de-LI` → `de`
   - `pt` → `pt-BR`
   - `zh-Hant`, `zh-HK`, `zh-MO` → `zh-TW`
   - `zh-Hans`, `zh`, `zh-SG` → `zh-CN`
   - `no`, `nn` → `nb`
3. **Blosses `de`** → `de-DE` (O-6). Schweizer Browser senden fast immer `de-CH`. Ein blosses
   `de` kommt meist aus Deutschland oder Österreich.
4. Grundsprache passt: `fr-CH` → `fr`, `it-CH` → `it`.
5. Sonst `en`. Heute ist der Rückfall `de` (O-2).

Gewählt werden kann nur eine **ausgelieferte** Sprache (4.4). Die Erkennung schlägt nie eine
andere vor.

### 4.3 Datenbank, Vertrag und die Bindung an Deutsch

**Vertrag: Die Liste wächst mit den Paketen, nicht im Voraus.** `Language` enthält immer genau
die Sprachen, deren Paket ausgeliefert ist. Grund: Eine zwischengespeicherte alte PWA kennt einen
neuen Code nicht, und `session.ts:48` wirft dann einen Fehler.
- **Stufe 1** liefert die Absicherung in `session.ts`: unbekannter Code, also Englisch. Dazu das
  Gerüst, die Liste bleibt aber `[de, en]`.
- **Stufe 2** nimmt `de-DE` auf.
- **Stufe 4** nimmt die 20 übrigen Sprachen auf. Das ist je eine neue Minor-Fassung.
- So ist ein neuer Code frühestens **ein Release nach** der Absicherung wählbar.
- Schritt 0 der Stufe 1 prüft, ob die PWA bei einer neuen Serverversion von sich aus neu lädt.

**Prüfung an einer Stelle.**
- `model.UILanguages` in Go ersetzt alle Prüfstellen aus der Tabelle in 2.3, auch
  `RunSetup` (`admin.go:343`).
- Ein Test hält `model.UILanguages` und die Vertragsliste gleich.
- Die Web-App führt die Liste in `i18n/languages.ts` und prüft sie mit `satisfies` gegen den
  generierten Typ `Language`. Den Wert zur Laufzeit aus dem Vertrag zu nehmen geht nicht, weil
  `openapi-typescript` nur Typen erzeugt.
- Eine Inventur aller festen `'de'`/`"de"` im Code gehört zu Schritt 0 der Stufe 1.

**Datenbank: `CHECK` auf `profiles.language` entfernen.** SQLite kann das nur durch einen
**Neubau der Tabelle**. Die Migration folgt der Vorlage, ausgeschrieben, weil sie heikel ist:
1. `-- +goose NO TRANSACTION`, `PRAGMA foreign_keys=OFF`, `BEGIN`.
2. `DROP VIEW visible_items`, weil sie `FROM profiles` liest. Vorbild ist
   `00022_account_roles.sql:30,75`.
3. `CREATE TABLE profiles_new` mit **allen** Spalten in ihrer heutigen Form, auch den später
   angefügten mit ihren `CHECK` aus `00007`, `00011`, `00013`, `00028` und `00032`. Nur `language`
   verliert den `CHECK`.
4. Daten kopieren, `DROP TABLE profiles`, umbenennen.
5. Indizes `profiles_account` und `profiles_primary` neu, Trigger
   `activity_events_profile_deleted` neu, `visible_items` neu (Stand nach `00056`).
6. `PRAGMA foreign_key_check`, `COMMIT`, `PRAGMA foreign_keys=ON`.

Dazu ein **neuer** eingefrorener DB-Stand in `testdata/db/` vor der Migration, mit Profilen,
Fortschritten, Merkliste und Downloads. Danach prüft der Test `foreign_key_check`, die Sicht und ein
Profil mit `ja`.

Ein neuer `CHECK` mit 23 Werten würde jede weitere Sprache zu einem Tabellen-Neubau machen. Darum
gibt es keinen (O-14).

**Bindung der Metadaten an Deutsch lösen (Kritiker B-1, B-2; F-7).**
- `visibility.Profile` (`visibility.go:52-58`) macht heute aus jeder anderen Oberflächensprache
  `de`. Neu gilt:
  - Die **Rückfallsprache** einer Scope ist `de` für `de` und `de-DE`, sonst `en`. Sie ist das
    zweite Glied der Kette in ADR 0018 und die Sprache für Genres und Sammlungsnamen. Sammlungen
    haben nur `name`/`name_de` (`discover.go:231`) und bleiben zweisprachig.
  - Die **Metadaten-Sprache**, wenn `metadata_language` leer ist, kommt aus der neuen Funktion
    `model.MetadataBase(ui)`:
    - `de-DE` → `de`
    - `pt-BR`, `pt-PT`, `zh-CN`, `zh-TW` bleiben
    - `nb` → `nb` oder `no`, nach Schritt 0
    - sonst der Code selbst
  - Liegt diese Sprache nicht in der wirksamen Liste, greift die bestehende Kette: Rückfallsprache,
    dann Rang. Ein französisches Profil auf einem Server mit `de → en` sieht also Deutsch, wo
    Englisch fehlt. Wer Französisch will, nimmt es in die Liste auf; der Assistent schlägt das in
    Schritt 3 vor.
- **F-9 aus ADR 0018 fällt (F-7).** `settings.go:264-267` verlangt nur noch eine nicht-leere
  Liste, für Server wie Bibliothek. ADR 0033 hält das fest.
- Schritt 0 der Stufe 1 prüft, wo der Code sonst noch eine `de`-Zeile voraussetzt (Genres,
  Sammlungen, Sortierschlüssel).
- **Ton- und Untertitelsprachen neuer Profile** (Kritiker M-3): `initialPrefs` schickt den Code
  durch `trackpick.Normalize`. Aus `de-DE` wird `de`, aus `pt-BR` `pt`, aus `zh-TW` `zh`. Sonst
  speichert jedes neue Profil einen ungültigen Wert, und das nächste Speichern scheitert an
  `invalid_value`. Ein Test deckt alle 23 Codes ab.
- `profiles.metadata_language` bleibt, wie es ist (ADR 0018).

### 4.4 Web-App

- **Liste der Sprachen:**
  - `i18n/languages.ts` führt die ausgelieferten Sprachen: Code, Name in der eigenen Sprache,
    `intlLocale`.
  - Die eigenen Namen sind **Daten, nicht Oberflächentext**. Jeder soll seine Sprache in der
    eigenen Schrift finden, egal welche Sprache gerade läuft.
  - `Intl.DisplayNames` liefert dafür keine brauchbaren Namen («Schweizer Hochdeutsch»). Regel 9
    bekommt diese Datei als benannte Ausnahme.
- **Laden:** `loaders` wird aus `import.meta.glob('@/locales/*.json')` gebaut statt von Hand. Jede
  Sprache bleibt ein eigenes Bündel; zum Laden der Kette siehe 4.2.
- **Sprachwahl:** `languageOptions` (`features/shared/choices.ts:17`) nimmt die Liste aus
  `languages.ts`. Sie wird verwendet im Profilformular, bei der Einladung, in Schritt 1 und im
  neuen Sprachknopf auf `/login`.
- **Formate:** neue Hilfsfunktion `intlLocale()`. Sie gibt für `de` `de-CH` zurück, sonst den Code.
  - Jeder `Intl`-Aufruf nimmt sie statt `i18n.language`. Das sind rund 80 Stellen, und eine
    Lint-Regel verbietet `i18n.language` als `Intl`-Argument.
  - Erst damit formatiert Deutsch (Schweiz) `1’234’567.5` und Deutsch (Deutschland)
    `1.234.567,5`.
  - Die Storybook-Vergleichsbilder ändern sich dadurch (Zahlen) und werden einmal neu erzeugt,
    auf dem dokumentierten CT-190-Weg.
  - Die `format.*`-Texte für Laufzeiten werden mitübersetzt.
- **Schrift:** weiter keine Webfonts (Startzeit, Offline-Grösse). Neu kommen `:lang()`-Stapel als
  Tokens in `typography.json`. Sonst wählt das Gerät für Japanisch womöglich chinesische Glyphen
  (Han-Vereinheitlichung).
  - `:lang(ja)`: Hiragino Sans, Yu Gothic UI, Noto Sans JP
  - `:lang(zh-CN)`: PingFang SC, Microsoft YaHei, Noto Sans SC
  - `:lang(zh-TW)`: PingFang TC, Microsoft JhengHei, Noto Sans TC
  - `:lang(ko)`: Apple SD Gothic Neo, Malgun Gothic, Noto Sans KR; dazu `word-break: keep-all`,
    damit Wörter nicht mitten im Silbenblock umbrechen
- **Länge:** Neu ist eine **Pseudo-Sprache `en-XA`**, nur in der Entwicklung und in Storybook.
  - Sie verlängert jeden Text um 40 % und setzt ihn in eckige Klammern.
  - `en-XA` ist ein gültiger Locale-Code. `Intl` wirft nicht, anders als bei einem erfundenen
    Code wie `x-long`.
  - Ein Storybook-Durchlauf in `en-XA` zeigt abgeschnittene Texte, bevor eine echte Sprache sie
    trifft. Er erzeugt keine eigenen Vergleichsbilder (4.9).
- **`<html lang>`** wird schon gesetzt (`i18n/index.ts:46-48`). Neu kommt `dir="ltr"` dazu
  (4.10). Manifest und `index.html` bleiben bei `de`.

### 4.5 Server-Texte

- Die 34 Texte aus `notify/texts.go` wandern in **eingebettete JSON-Dateien**
  `server/internal/notify/locales/<code>.json`.
  - Sie haben dasselbe Format wie die Web-App, also i18next v4 mit `{{name}}` statt `%s`, damit
    Weblate beide gleich behandelt.
  - Ein kleines Go-Paket `internal/i18n` lädt sie (`embed`) und setzt Platzhalter ein.
- Die Rückfallkette ist dieselbe wie im Web: Sprache → `de` bei `de-DE` → `en`. Heute fällt der
  Server auf `de` zurück.
- `notify_channels.sql:36,47` nimmt `'en'` statt `'de'` als Rückfall, wenn ein Konto kein
  Hauptprofil hat. Bestehende Konten haben immer eins, also ändert sich für sie nichts.
- Die HTML-Mail bekommt `lang="<code>"`.
- Die Sprache des Empfängers wird gewählt wie heute (Profil bzw. Hauptprofil).
- Die Go-Tests, die heute deutsche Sätze prüfen (`notify/channels_test.go`, `email_test.go`,
  `push_test.go`), prüfen danach gegen die JSON-Datei.
- **Nicht übersetzt** werden Ankündigungen, Fehlertitel im API (`title` bleibt englisch, die
  Web-App übersetzt den `code`) und Log-Zeilen.

### 4.6 Übersetzen: die Erstfassung durch Claude

Marco will «natürlich und sinnvoll übersetzt» (F-4). Das heisst nicht Wort für Wort, sondern so,
wie eine gute App in dieser Sprache klingt. Dafür braucht es drei Dinge, die vor dem ersten
übersetzten Text stehen.

1. **Glossar** `docs/i18n/glossar.md`:
   - rund 80 Begriffe aus Slouchcat («Profil», «Bibliothek», «Merkliste», «Gesehen», «Sammlung»,
     «Spotlight», «Freigabe», «Verwaltung» …), je Sprache mit einer festen Übersetzung
   - Begriffe, die **nicht** übersetzt werden: Slouchcat, TMDB, Namen von Diensten
   - Fachbegriffe richten sich nach dem, was Streamingdienste in der Sprache verwenden.
2. **Stilregeln je Sprache** `docs/i18n/stil/<code>.md`:
   - Anrede (O-1)
   - Anführungszeichen: de «», de-DE „“, fr « » mit schmalem geschütztem Leerzeichen, ja 「」,
     zh-TW 「」, zh-CN “” …
   - Gross- und Kleinschreibung in Knöpfen
   - Satzzeichen und Zahlwörter
3. **Kontext je Text:** Ein Hilfsskript `scripts/i18n/context.ts` erzeugt zu jedem Schlüssel die
   Datei und Zeile, an der er verwendet wird, und den Komponentennamen. So weiss die Übersetzung,
   ob «Play» ein Knopf, eine Überschrift oder ein Statustext ist.

**Ablauf je Sprache**, als Agentenlauf in Stufe 4:
1. Übersetzen in Blöcken je Bereich (36 Bereiche), mit Deutsch **und** Englisch als Quelle,
   Glossar, Stilregeln und Kontext.
2. **Prüfung durch einen zweiten Agenten**, der nicht übersetzt hat:
   - Glossartreue
   - Platzhalter vollständig und unverändert
   - Pluralformen nach 4.1
   - Länge: Knöpfe höchstens rund 1,5× Englisch, Warnliste
   - Ton
   - Rückübersetzung von 50 zufälligen Texten ins Deutsche. Weicht der Sinn ab, wird
     nachgebessert.
3. **Bildprüfung:** Storybook-Durchlauf je Sprache für 20 Kernseiten (Anmeldung, Startseite,
   Detailseite, Abspielseite, Verwaltung, Assistent), ohne Vergleichsbilder.
   - Ein Agent sucht auf den Bildern abgeschnittene oder überlaufende Texte.
   - Der Container für diesen Lauf bekommt `fonts-noto-cjk`. Sonst erscheinen in der CI mit
     DejaVu für CJK nur Kästchen.
4. Marco prüft Stichproben in Sprachen, die er liest (O-9).

**Umfang:**
- 4318 Web-Texte und 34 Server-Texte, für 20 neue Sprachen, also rund **87'000 Texte**.
- Dazu `de-DE` mit rund 185 Texten.
- Das ist ein mehrtägiger Agentenlauf und wird in Stufe 4 in Sprachgruppen geschnitten
  (Abschnitt 7).

### 4.7 Weblate

- **Gehostetes Weblate, Plan «Libre»:** kostenlos für öffentliche Projekte unter freier Lizenz.
  Slouchcat ist öffentlich (`SlouchCat/slouchcat`) und steht unter AGPL-3.0, erfüllt also beides.
  Die Bedingungen prüft Marco beim Antrag (Stufe 5). Ein selbst betriebenes Weblate wäre ein
  weiterer Dienst zum Pflegen (O-8).
- **Konto und Antrag macht Marco selbst.** Claude legt keine Konten an.
- **Zwei Komponenten:**
  - `web` mit `web/src/locales/*.json`
  - `server` mit `server/internal/notify/locales/*.json`
  - Format jeweils «i18next JSON v4».
  - **Vorlage ist Englisch** (`en.json`), weil die meisten Übersetzer Englisch lesen.
  - Deutsch (`de`, `de-DE`) ist in Weblate gesperrt. Es wird im Repository gepflegt.
- **Weg zurück ins Repository:** Weblate committet auf einen eigenen Zweig und öffnet einen
  Pull Request. Die CI prüft ihn wie jeden anderen. Gemergt wird auf Marcos Wort.
- **Regel gegen Kollisionen:** Ab Stufe 5 ändern Agenten bestehende Übersetzungen der 20 Sprachen
  **nicht** mehr. Sie füllen nur fehlende Texte nach (4.8), und zwar erst, nachdem offene
  Weblate-Änderungen gemergt sind. Korrekturen laufen über Weblate.
- **Kennzeichnung:** Das JSON-Format hat kein «ungeprüft»-Merkmal. Claudes Texte erscheinen in
  Weblate deshalb als fertig übersetzt. `docs/i18n/README.md` sagt das offen und bittet um
  Korrekturen. Das README bekommt eine Weblate-Plakette mit dem Stand je Sprache.

### 4.8 Neue Texte im Alltag

Heute muss jede Änderung `de` und `en` gleichzeitig liefern; `locales.test.ts` setzt das durch.
Mit 23 Sprachen soll nicht jeder Pull Request zwanzig Übersetzungen tragen.

**Vorschlag (O-7):**
- **Im Pull Request:** `de` und `en` sind Pflicht wie heute. `de-DE` ist Pflicht, sobald der neue
  Text ein ss-Wort oder «Guillemets» enthält. Ein Test zeigt die Kandidaten an.
- **Fehlende Texte der übrigen Sprachen** brechen den Test in `main` nicht. `task i18n:status`
  listet sie. Zwischen zwei Releases sieht man dort Englisch, dank der geladenen Kette (4.2).
- **Vor jedem Release, auch vor einem Patch:** Ein Schritt «Übersetzungen nachziehen» im
  Release-Ablauf. Claude übersetzt die fehlenden Texte nach 4.6. `task i18n:check` muss grün
  sein, also keine ausgelieferte Sprache mit fehlendem Text. Erst dann wird getaggt.
  - Bei Patch-Releases (Versionscommit auf dem Fix-Zweig) läuft der Schritt auf dem Fix-Zweig.
- Welche Sprachen wählbar sind, steht fest in `languages.ts` und im Vertrag (4.3), nicht in einer
  Zählung zur Laufzeit. Eine Sprache verschwindet also nie, weil ein Text fehlt.

### 4.9 Tests

**`locales.test.ts`**, je ausgelieferte Sprache. Er läuft in `main` nur für `de`, `de-DE` und `en`
streng, für die übrigen als Warnung. `task i18n:check` vor dem Release ist für alle streng.
- gleiche Schlüssel wie `de`, abzüglich der Pluralformen
- Pluralformen genau nach `Intl.PluralRules`, also bei `ru` `_one`, `_few`, `_many` und `_other`,
  bei `ja` nur `_other`. Ausnahmeliste für Texte, die nur `_other` haben
  (`admin.home.browse.hanging`).
- gleiche Platzhalter je Text
- kein leerer Text
- Text identisch mit Englisch: **nur Warnung**. «Film», «Trailer» und «Podcast» sind in nl, sv und
  da richtig.
- `de`: weiterhin kein ß. `de-DE`: nur Schlüssel, die es in `de` gibt, und jeder Text weicht ab.

**Go**
- dieselben Prüfungen für die Server-Texte
- `model.UILanguages` gleich der Vertragsliste `Language`
- `initialPrefs` mit allen Codes (4.3)
- `visibility.Profile` mit `fr`, `de-DE`, `pt-BR`

**Web**
- Unit-Test für `ensureLanguage`: Bei `de-DE` sind danach `de-DE`, `de` und `en` geladen.
- Unit-Test für `intlLocale`.

**e2e**
- Bleibt auf `de-CH`. Die 3385 Selektoren werden **nicht** umgebaut.
- Neu ist `languages.spec.ts`: Für jede **ausgelieferte** Sprache meldet sie sich an, öffnet
  Startseite und Profilformular und prüft `<html lang>` und einen bekannten Text.
- Dazu ein Schritt im Assistenten mit Sprachwechsel.
- Gestartet wird über `task e2e:quick` wie jede andere Spec.

**Storybook-Vergleichsbilder:** bleiben bei `de`, also keine 23 × 502 Bilder. Die Bildprüfung aus
4.6 ist ein einmaliger Lauf je Übersetzungsrunde, kein Gate.

**Migration:** neuer eingefrorener DB-Stand (4.3).

### 4.10 Bewusst nicht in diesem Konzept

- **Rechts nach links** (Arabisch, Hebräisch, Persisch):
  - Die Web-App nutzt heute 73× `left:` und 52× `right:`, aber nur wenige logische Eigenschaften.
  - Das ist ein eigener Umbau mit eigenem Konzept (O-16).
  - Stufe 1 setzt aber schon `dir="ltr"` am `<html>` und verlangt in stylelint für **neuen** Code
    logische Eigenschaften, damit die Zahl nicht wächst.
- **Artikel beim Sortieren** (`model/sortkey.go:16-17`): kennt nur «der/die/das» und «the». Titel
  kommen in der Metadaten-Sprache. Gehört zu ADR 0018, nicht hierher.
- **Übersetzte Sammlungsnamen und Genres** über Deutsch und Englisch hinaus: Sie bleiben
  zweisprachig (4.3).

### 4.11 Teil C: Freigabeländer und Altersstufen je Land (F-12)

**Heute** (`metadata/ratings.go:5-37`):
- Slouchcat liest die TMDB-Freigabe in der Reihenfolge gewähltes Land → DE → AT → CH → GB → US.
- Es rundet sie auf die **nächsthöhere FSK-Stufe** (0/6/12/16/18). AT 14 wird zum Beispiel FSK 16.
- Diese Stufen sind an vielen Stellen fest:

| Ort | Stelle |
|---|---|
| Vertrag | `Rating` mit `enum [0, 6, 12, 16, 18]`, 25 Verweise |
| Go | `auth/profiles.go:25` (`ratingLevels`), `admin/admin.go:912-913` (`validRating`) |
| Datenbank, `CHECK … IN (0, 6, 12, 16, 18)` | `profiles.max_rating` (`00002`), `accounts.max_rating` (`00022`), `invitations.max_rating` (`00007`), `items.own_rating/effective_rating/min_child_rating` (`00003`), `libraries.default_rating` (`00023`) |
| Web | der Text «FSK {{age}}» (`de.json:704`) |

**Neu (F-12):**
- **Gespeichert wird das Mindestalter**, nicht mehr die gerundete FSK-Stufe. FR «10» wird 10,
  US «PG-13» 13, JP «R15+» 15, GB «15» 15.
- Die Sichtbarkeit vergleicht weiter `Alter des Titels ≤ Stufe des Profils`
  (`visible_items`, `00056:33-38`).
- **Für einen deutschen Haushalt ändert sich nichts.** Bei Profilstufen 0/6/12/16/18 ergibt der
  Vergleich mit dem echten Alter dasselbe wie heute der Vergleich mit dem aufgerundeten Wert:
  14 ≤ 12 ist falsch, 16 ≤ 12 auch; 14 ≤ 16 ist wahr, 16 ≤ 16 auch.
- **Profile wählen aus den Stufen des Freigabelands.** Ein französischer Haushalt hat 0, 10, 12,
  16 und 18. Ein 10-jähriges Kind sieht also, was in Frankreich ab 10 freigegeben ist.
- **Reihenfolge beim Lesen:** gewähltes Land → **DE** (F-12) → AT → CH → GB → US. Hat ein Titel
  keine Freigabe des Landes, gilt die deutsche FSK als Alter.
- **Länder und Stufen.** Dies ist ein Vorschlag. Schritt 0 der Stufe 3 gleicht ihn mit TMDB
  `/certification/movie/list` und `/certification/tv/list` ab, weil TV-Systeme oft abweichen.

| Land | Stufen (Mindestalter, Bezeichnung) |
|---|---|
| DE | 0, 6, 12, 16, 18 (FSK) |
| AT | 0, 6, 10, 12, 14, 16, 18 (JMK) |
| CH | 0, 6, 7, 8, 10, 12, 14, 16, 18 |
| GB | 0 U, 6 PG, 12 12A, 15, 18 (BBFC) |
| US | 0 G/TV-Y, 6 PG/TV-PG, 13 PG-13, 14 TV-14, 17 R, 18 NC-17/TV-MA |
| FR | 0 Tous publics, 10, 12, 16, 18 |
| ES | 0 A, 7, 12, 16, 18 |
| IT | 0 T, 6+, 14+, 18+ |
| NL, BE | 0 AL, 6, 9, 12, 14, 16, 18 (Kijkwijzer) |
| PT | 3 M/3, 6, 12, 14, 16, 18 (M/…) |
| BR | 0 L, 10, 12, 14, 16, 18 |
| PL | 0, 7, 12, 16, 18 |
| SE | 0 Btl, 7, 11, 15 |
| DK | 0 A, 7, 11, 15 |
| NO | 0 A, 6, 9, 12, 15, 18 |
| FI | 0 S, 7, 12, 16, 18 (K-…) |
| CZ | 0 U, 12, 15, 18 |
| HU | 0 KN, 6, 12, 16, 18 |
| TR | 0 Genel, 7+, 13+, 15+, 18+ |
| RU | 0+, 6+, 12+, 16+, 18+ |
| UA | 0, 12, 16, 18 |
| JP | 0 G, 12 PG12, 15 R15+, 18 R18+ |
| KR | 0 ALL, 12, 15, 18 (TV 19 → 18) |
| TW | 0+, 6+, 12+, 15+, 18+ |

  China hat kein Freigabesystem. Für `zh-CN` schlägt der Assistent Deutschland vor.

**Was dafür nötig ist:**
- **Daten statt Text:** Die Stufen und Bezeichnungen je Land stehen als Tabelle in Go
  (`metadata/ratings.go`, Nachfolger von `fskTables`). Der Server liefert sie in
  `SessionInfo.ratingSystem` (`{country, steps: [{age, label}]}`).
  - Wie die Sprachnamen sind das **Daten, keine Oberflächentexte**. Regel 9 bekommt die Tabelle als
    benannte Ausnahme.
  - Nur die Wörter um die Stufe herum («ab {{age}}», «Freigabe») kommen aus den Übersetzungen.
- **Vertrag:**
  - `Rating` wird `integer`, 0 bis 21.
  - `ratingCountry` wird auf die Länder der Tabelle erweitert.
  - neu `SessionInfo.ratingSystem`
  - Wie bei den Sprachen fängt die Web-App ein unbekanntes Alter ab und zeigt «ab {{age}}». Diese
    Absicherung kommt in Stufe 1 mit.
- **Prüfung:** `ratingLevels` und `validRating` prüfen gegen die Stufen des **aktuellen**
  Freigabelands. Bestehende Profile behalten ihren Wert auch nach einem Wechsel des Landes. Die
  Auswahl zeigt ihn dann als «ab {{age}}» zusätzlich zu den Stufen des neuen Landes.
- **Datenbank:**
  - `profiles` verliert den `CHECK` auf `max_rating` schon beim Neubau in Stufe 1. Das kostet dort
    nichts.
  - `accounts` und `invitations` werden in Stufe 3 neu gebaut, nach derselben Anleitung wie
    `profiles` in 4.3: Sichten und Trigger prüfen, `BEGIN`/`COMMIT`, neuer eingefrorener DB-Stand.
  - **`items` und `libraries` werden nicht neu gebaut.** `items` ist die grösste Tabelle, mit
    vielen Sichten, Triggern und angefügten Spalten. Stattdessen kommen neue Spalten ohne `CHECK`
    dazu:
    - `items`: `own_age`, `effective_age`, `min_child_age`
    - `libraries`: `default_age`
    - Die Migration füllt sie aus den alten Spalten. `visible_items` und alle Abfragen lesen danach
      nur noch die neuen.
    - Die alten Spalten bleiben unbenutzt stehen. Ein Test verbietet, sie zu lesen.
- **Anzeige am Titel:** statt «FSK 12» die Bezeichnung des Freigabelands, also «PG-13», «-12» oder
  «R15+». Ist das Alter keine Stufe des Landes, zeigt sie «ab {{age}}». Die Quelle steht wie heute
  im Detail («Freigabe aus: tmdb:FR»).
- **Wechsel des Freigabelands (F-16):** Ändert sich `metadata.rating_country`, liest Slouchcat alle
  Freigaben neu und darf dabei auch **senken**. Das ist eine Ausnahme von K-3 («eine gespeicherte
  Freigabe wird nie gesenkt»).
  - **Von Hand gesetzte Freigaben** (`own_age`, ehemals `own_rating`) bleiben immer unberührt.
  - **Damit das ohne TMDB geht**, speichert Stufe 3 alle Freigaben eines Titels je Land in einer
    neuen Tabelle `item_certifications (item_id, country, certification)`, gefüllt beim
    Metadaten-Abgleich. Ein Landwechsel rechnet dann nur lokal neu, ohne neue Abrufe.
  - **Titel aus der Zeit vor Stufe 3** haben diese Zeilen noch nicht. Beim ersten Landwechsel holt
    ein Hintergrundjob «Freigaben neu lesen» ihre Freigaben einmal von TMDB nach und hält sich an
    die bestehenden Grenzen der Metadaten-Jobs. Bis er fertig ist, gelten die alten Werte; sie sind
    eher zu streng, nie zu locker.
  - **Sichtbar:** Der Speichern-Dialog der Metadaten-Einstellungen sagt vor dem Wechsel «Slouchcat
    liest danach die Freigaben aller Titel neu. Profile können dadurch Titel sehen, die bisher
    verborgen waren.» Der Job erscheint in der Jobliste.
  - **Der automatische Abgleich beim Scan** senkt weiterhin nie (K-3). Nur der Landwechsel tut es.
  - Eine neue Installation stellt das Land im Assistenten vor dem ersten Scan ein. Der Wechsel
    betrifft dort also noch keine Titel.
- **Offline (Regel 12):** Der Server entscheidet weiter. Offline-Pakete tragen die erlaubten Titel
  wie heute, und der Client vergleicht keine Altersstufen.
- **Tests:**
  - Je Land die Tabelle gegen Beispiele aus TMDB.
  - Die Gleichwertigkeit für deutsche Profile mit allen Stufen gegen alle bisherigen Werte. Die
    `vistest`-Bausteine bekommen dafür einen französischen Haushalt.
  - Migration mit eingefrorenem DB-Stand.

---

## 5. Risiken

| Risiko | Folge | Gegenmittel |
|---|---|---|
| Maschinell klingende Übersetzung | Marcos Ziel «natürlich» verfehlt | Glossar, Stilregeln, Kontext, zweiter Prüfagent, Rückübersetzung, Weblate |
| Abgeschnittene Texte in langen Sprachen | unlesbare Knöpfe | Pseudo-Sprache `en-XA`, Bildprüfung je Sprache |
| Neubau der Tabelle `profiles` | Fremdschlüssel, Sicht und Trigger brechen | Schritte in 4.3, neuer eingefrorener DB-Stand, `foreign_key_check`, Sicherung vor Migration (Regel 5) |
| Alte PWA bekommt einen neuen Sprachcode | Absturz beim Anwenden der Sitzung | Absicherung in `session.ts` ein Release vor jedem neuen Code (4.3) |
| Gewählter Grafikchip geht nach einem Treiberwechsel nicht mehr | Neustart-Schleife | gespeichertes Backend fällt auf `auto` zurück, Hinweis im Dashboard (Schritt 5) |
| Container ohne Restart-Policy | Slouchcat bleibt nach «Abschliessen» aus | nach 60 Sekunden Hinweis «von Hand starten» (O-18) |
| Release-Ablauf wird länger | «Übersetzungen nachziehen» kostet bei jedem Release Zeit | Nur neue Texte; typisch 10 bis 50 je Release |
| Weblate-PR ändert einen Platzhalter | Laufzeitfehler | Weblate-Prüfung «Platzhalter» und `locales.test.ts` in der CI |
| Landwechsel senkt Freigaben (F-16) | ein Kind sieht danach Titel, die vorher verborgen waren | Hinweis vor dem Speichern; von Hand gesetzte Freigaben bleiben; Test je Land für die Tabelle |
| Neue Spalten für das Alter in `items` (Teil C) | eine Abfrage liest noch die alte Spalte und filtert falsch | Test verbietet die alten Spalten; `vistest` mit französischem Haushalt; Gleichwertigkeit für deutsche Profile |
| Metadaten ohne Deutsch (F-7) | Code, der stillschweigend eine `de`-Zeile erwartet, zeigt Leeres | Inventur in Schritt 0 der Stufe 1, Tests mit `fr`-Profil auf `fr → en`-Server |

---

## 6. Änderungen an früheren Entscheiden

- **Spec C-3** (Oberfläche Deutsch und Englisch) wird zu «23 Oberflächensprachen nach Konzept 22,
  je Profil wählbar».
- **ADR 0018** wird in zwei Punkten durch **ADR 0033** ersetzt:
  - **F-1** («Nur Metadaten», keine neuen Oberflächensprachen): Der Grund von damals (2300 Texte je
    Sprache von Hand) entfällt durch die Erstfassung durch Claude und Weblate.
  - **F-9** («Serverliste muss `de` enthalten»): fällt nach F-7.
  - Dazu neu: Rückfallsprache `de`/`en` je Oberflächensprache und `model.MetadataBase` (4.3).
  - Der Rest von ADR 0018 bleibt.
- **ADR 0034 (neu), Teil C:** ändert Entscheid 6 und AN-8. Statt der FSK-Rundung wird das
  Mindestalter gespeichert, und die Profile wählen aus den Stufen des Freigabelands. K-3 («eine
  gespeicherte Freigabe wird nie gesenkt») bekommt eine Ausnahme: Ein Wechsel des Freigabelands
  liest alle Freigaben neu und darf senken (F-16). Von Hand gesetzte Freigaben bleiben davon
  ausgenommen.
- **ADR 0001** nennt `{de,en}.json`. Der Satz wird auf `web/src/locales/*.json` geändert.
- **`CLAUDE.md`:**
  - Regel 7: `owner` in der Liste der Sichtbarkeiten.
  - Regel 9: Texte nur aus `web/src/locales/*.json`. Benannte Ausnahmen sind
    `web/src/i18n/languages.ts` (eigene Sprachnamen) und die Freigabetabelle des Servers
    (Bezeichnungen wie «PG-13»). Dazu der Satz: «`de` und `en` in jedem Pull Request, die
    übrigen Sprachen vor jedem Release (Konzept 22, 4.8).»
- **Spec A-1, Plan 3.4:** Der Assistent hat neun Schritte statt vier. Die Warnungen aus Plan 3.4
  bleiben in Schritt 5 und 6. Die GPU wird erkannt **und** ist wählbar (F-10).
- **`docs/INSTALLATION.md` Abschnitt 5** wird für den neuen Ablauf neu geschrieben. Er nennt `TZ`
  als den einen Wert, der nur in `.env` geht, und erklärt `SLOUCHCAT_HW` als Vorrang vor der Wahl im
  Assistenten.
- **Release-Ablauf** (`docs/dev/RELEASE.md` oder wo er steht): neuer Schritt «Übersetzungen
  nachziehen» (4.8).
- **Entscheidungsprotokoll:** Frage 29 bekommt eine Zeile, sobald Marco das Konzept freigibt.

---

## 7. Stufen der Umsetzung

Jede Stufe ist ein eigener Zweig mit Pull Request und Release. Vertrag und Migration werden in
Bau-Reihenfolge nummeriert.

| Stufe | Inhalt | Vertrag | Migration | Sichtbar für Marco |
|---|---|---|---|---|
| **1 «Sprachgerüst»** | 4.2 bis 4.5 und 4.9 ohne neue Sprachen: Absicherung in `session.ts` für Sprache **und Alter**, Laden der Kette, `languages.ts`, `intlLocale` (neue Storybook-Bilder), Tabellen-Neubau `profiles` (ohne `CHECK` auf `language` und `max_rating`), `model.UILanguages`, `MetadataBase` und Rückfallsprache, F-9 fällt, `initialPrefs` normalisiert, Erkennung, Server-Texte als JSON, `:lang()`-Schriften, `en-XA`, neue Tests, Sprachknopf auf `/login`. `de-DE.json` wird erstellt, aber noch nicht wählbar. | ja (F-9-Lockerung) | ja | Zahlen in Schweizer Format; Metadaten-Liste ohne Deutsch möglich |
| **2 «Assistent»** | Teil A ganz, mit den fünf Freigabeländern von heute. Dazu `de-DE` wählbar (Vertragsliste `[de, de-DE, en]`), Grafikchip-Wahl auch in der Verwaltung (O-19), Wegfall der öffentlichen Prüf-Endpunkte (F-14), INSTALLATION.md. | ja (brechend, F-14) | nein | neuer Assistent auf einer frischen Testinstanz; Deutsch (Deutschland) |
| **3 «Freigabeländer»** | Teil C (4.11): Mindestalter statt FSK-Stufe, Ländertabelle, `ratingSystem`, Neubau `accounts` und `invitations`, neue Alter-Spalten in `items` und `libraries`, Tabelle `item_certifications`, Neuberechnung beim Landwechsel mit Job für ältere Titel (F-16), Anzeige am Titel, Assistent mit allen Ländern. | ja | ja | Freigabeland Frankreich mit Stufe 10 im Profil |
| **4 «Übersetzungen»** | 4.6: Glossar, Stilregeln, Kontextskript, dann 20 Sprachen in vier Gruppen (fr/es/it/nl/pt-BR · pt-PT/pl/cs/hu/tr · sv/da/nb/fi · ru/uk/ja/ko/zh-CN/zh-TW), je Gruppe Übersetzen, Prüfen, Bildprüfung. **Ein Release für alle 20** (F-4); die Gruppen sind nur Arbeitspakete. Vertragsliste auf 23. | ja (Liste) | nein | 23 Sprachen wählbar |
| **5 «Weblate»** | 4.7 und 4.8: Weblate-Projekt (Konto durch Marco), Komponenten, `docs/i18n/README.md`, Plakette, Release-Schritt «Übersetzungen nachziehen», `task i18n:status`/`i18n:check`. | nein | nein | Mitarbeit von aussen möglich |

- Stufe 2 hängt an Stufe 1 (Liste, Absicherung).
- Stufe 3 hängt an Stufe 1 (Neubau `profiles`, Absicherung Alter) und an Stufe 2 (Assistent).
- Stufe 4 hängt an Stufe 1; übersetzt werden darf parallel zu Stufe 2 und 3 (O-10). Stufe 4 bringt
  auch die übersetzten Bezeichnungen um die Freigaben mit.
- Stufe 5 braucht Stufe 4.
- **Schritt 0 jeder Stufe:** die «Heute»-Angaben aus Abschnitt 2 am dann aktuellen `main`
  nachprüfen. Bekannt offen sind:
  - Inventur fester `'de'` im Code
  - `nb` oder `no` bei TMDB
  - lädt die PWA bei neuer Serverversion selbst neu?
  - Nummern von Vertrag und Migration nach K21 Stufen 4–6
  - wo der Release-Ablauf dokumentiert ist

---

## 8. Offene Punkte: von Marco beantwortet (14. November 2026)

Alle Punkte sind beantwortet. Wo Marco von der Empfehlung abwich, steht die Antwort auch als F-Nummer
in Abschnitt 1.

| Nr. | Frage | Antwort |
|---|---|---|
| **O-1** | Anrede je Sprache | **Wie üblich je Sprache** (Empfehlung): de du, fr vous, es tú, it tu, nl je, pt-BR você, pt-PT você, pl Imperativ ohne Pronomen, cs vy, ru вы, uk ви, tr siz, hu Sie-Form ohne Pronomen, sv/da/nb du, fi ohne Pronomen, ja です/ます, ko 해요체, zh 你. |
| **O-2** | Rückfall, wenn keine Browsersprache passt | **Englisch** (Empfehlung). |
| **O-3** | Abbruch nach Schritt 2 | **Immer weiterleiten**, bis abgeschlossen (F-11, abweichend). |
| **O-4** | «Erreichbarkeit prüfen» | **Nicht jetzt** (Empfehlung). Nur Formatprüfung `https://…`. |
| **O-5** | Freigabeländer | **Weitere Länder jetzt, Stufen des Landes, Rückfall DE** (F-12, abweichend; Teil C, 4.11). |
| **O-6** | Blosses `de` im Browser | **`de-DE`** (Empfehlung). |
| **O-7** | Neue Texte | **de und en im Pull Request, der Rest vor jedem Release** (Empfehlung). |
| **O-8** | Weblate | **Gehostet, Plan «Libre»** (Empfehlung). |
| **O-9** | Stichproben durch Marco | **de-DE und en** (F-15). Alle übrigen Sprachen prüft der Prüfagent, danach Weblate. |
| **O-10** | Reihenfolge | **Assistent vor Übersetzungen** (Empfehlung). Übersetzen darf parallel laufen. |
| **O-11** | Weitere Dienste im Assistenten | **Ja, alle, mit «Empfohlen»/«Optional»** (F-13, abweichend). Empfohlen sind TMDB und E-Mail. |
| **O-12** | Öffentliche Prüf-Endpunkte | **Sofort entfernen** (F-14, abweichend). |
| **O-13** | Nach «Abschliessen» | **Abschlussseite** mit Scan-Fortschritt (Empfehlung). |
| **O-14** | `CHECK` mit Sprachliste in der DB | **Nein**, Prüfung in Go (Empfehlung). |
| **O-15** | Metadaten-Vorschlag | **Sprache → Englisch** (Empfehlung). |
| **O-16** | Rechts nach links | **Später, eigenes Konzept** (Empfehlung). |
| **O-17** | Einrichtungs-Token | **Nein** (Empfehlung). |
| **O-18** | Neustart nach Grafikchip-Wahl | **Server startet sich selbst neu** (Empfehlung). |
| **O-19** | Grafikchip-Wahl in der Verwaltung | **Ja** (Empfehlung). |
| **O-20** | Gespeicherte Freigaben nach einem Wechsel des Freigabelands (neu in Fassung 3, aus F-12) | **Automatisch beim Landwechsel neu lesen, auch senken; von Hand gesetzte bleiben** (F-16, abweichend von der Empfehlung «nur als Aktion»). |

## 9. Kritiker-Review (Fassung 1 → Fassung 2)

Kritiker (oh-my-claudecode:critic) am 14. November 2026 gegen `b12877cb`: **REVISE**, 2 Blocker,
11 Major, 16 Minor. Alles ist eingearbeitet.

| Befund | Kern | Eingearbeitet in |
|---|---|---|
| B-1 | Serverliste muss `de` enthalten (ADR 0018 F-9); Vorschläge `fr → en` würden abgelehnt | F-7 (Marco: keine Pflicht), 4.3, 6 |
| B-2 | `visibility.Profile` macht jede Sprache zu `de` | 4.3 Rückfallsprache und `MetadataBase` |
| M-1 | Lazy Loading lädt die Rückfälle nie; `de-DE` zeigt Schlüssel | 4.2 Laden der Kette, 4.9 |
| M-2 | Prüfstellen falsch gezählt, `RunSetup` fehlte | 2.3 Tabelle, 4.3 |
| M-3 | `initialPrefs` speichert ungültige Ton- und Untertitelsprachen | 4.3, 4.9 |
| M-4 | Tabellen-Neubau ohne Sicht, Trigger, `BEGIN`/`COMMIT` | 4.3 Schritte 1–6 |
| M-5 | Alte PWA stürzt bei neuem Code ab | 4.3 Liste wächst mit den Paketen, Stufen 1–3 |
| M-6 | e2e würde in den Assistenten umgeleitet | 3.4 Grundregel «fehlt = abgeschlossen», keine Datenmigration |
| M-7 | `applyRadioOffer` setzt Modus «Alle», braucht `version` | F-8, Schritt 3 |
| M-8 | «PIN beim ersten Öffnen» gibt es nicht und wäre unsicher | F-9, Schritt 7 |
| M-9 | Pseudo-Sprache `x-long` bricht `Intl` | 4.4 `en-XA` |
| M-10 | O-6, 4.8 und 4.9 widersprachen sich | 4.8 letzter Punkt, 4.9, O-6 neu |
| M-11 | `de` formatiert deutsche statt Schweizer Zahlen | 4.4 `intlLocale` |
| m-1 | `getSystemInfo`, Zeitzone öffentlich | 2.2, 3.5 |
| m-2 | drei Schritt-0-Fragen schon beantwortbar | 2.1, 2.2, 3.3 Schritt 6, 4.1 |
| m-3 | Sichtbarkeit `owner` statt `admin` | 3.5, 6 |
| m-4 | Router: Schritt 1 nach dem Konto, «einmal» unklar | 3.4, 3.6 |
| m-5 | F-2 verlangte Abfrage des Grafikchips | F-10, Schritt 5 |
| m-6 | Trickplay-Vorgabe nur im Browser; falscher Schlüssel bleibt | 3.4, Schritt 4 |
| m-7 | Sprachnamen und Regel 9 | 4.4, 6 |
| m-8 | Liste nicht zur Laufzeit aus dem Vertrag | 4.3 `satisfies` |
| m-9 | Englisch-gleich nur Warnung; Plural-Ausnahme | 4.9 |
| m-10 | blosses `de` | 4.2, O-6 |
| m-11 | CJK in der CI, Koreanisch `keep-all` | 4.4, 4.6 |
| m-12 | Weblate ohne «ungeprüft»; Patch-Releases | 4.7, 4.8 |
| m-13 | Meldung bei `ErrSetupDone` | 3.1 |
| m-14 | «Später» ohne Bibliothek, `finishSetup` ohne Bibliothek | 3.3 Schritt 6, 3.4, 3.5 |
| m-15 | Zeilenangaben | 2.2, 2.3 |
| m-16 | Migration 00059 auf K21 | Kopf |
| Frage | Einrichtungs-Token | O-17 |

---

## Anhang A: Mockup

- Klickbarer Prototyp: `docs/planning/mockups/22-ersteinrichtung/prototyp.html`. Gleich als
  Artefakt: https://claude.ai/artifact/5cRAoZpb7Th38TABdVaqzr
- Drei Varianten zum Umschalten. **Gewählt ist A** (F-1). B und C bleiben zum Vergleich.
- Ab Version 5 folgt Variante A Fassung 3:
  - acht Dienste mit «Empfohlen»/«Optional»
  - Freigabeländer mit Stufen je Land
  - Weiterleitung bei jedem Anmelden
- Ab Version 4 folgte Variante A Fassung 2:
  - Reihenfolge aus 3.2
  - Grafikchip wählbar mit Neustart-Hinweis
  - optionale PIN
  - Radio als «Land ohne Sender»
  - Zeitzone nur Anzeige
  - fünf Freigabeländer
  - Metadaten-Vorschlag ohne Deutschpflicht
- Kopfzeile, Schrittnamen, Knöpfe und Willkommensseite sind im Mockup in de, de-DE, en, fr, es
  und it übersetzt. Die übrigen Sprachen fallen dort auf Englisch zurück. Die Schrittinhalte
  stehen nur auf Deutsch.
- Prüfungen im Mockup sind gespielt: `/media/…` gilt als gültig, `/mnt/nas/…` als Netzlaufwerk,
  ein TMDB-Schlüssel ab 11 Zeichen als gültig.
