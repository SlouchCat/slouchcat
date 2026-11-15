# ADR 0033: Oberflächensprachen, Übersetzung und Metadaten-Sprachen

- Status: angenommen (15. November 2026, Stufe 1 «Sprachgerüst» gebaut); **ersetzt ADR 0018 in zwei
  Punkten**: F-1 («keine neuen Oberflächensprachen») und F-9 («die Serverliste der Metadaten-Sprachen
  muss `de` enthalten»). Alles Übrige von ADR 0018 bleibt (Abschnitt «Was von ADR 0018 bleibt»).
- Quelle: Konzept `docs/planning/22-ersteinrichtung-und-sprachen.md` (Fassung 4), Abschnitte 4.1 bis 4.10,
  6 und 7; **Frage 29** in `docs/planning/04-entscheidungen.md` (F-3, F-4, F-5, F-7, F-15, O-2, O-6, O-7,
  O-14, O-16)
- Vertrag **1.34.0** (Lockerung: eine Metadaten-Sprachliste braucht kein `de` mehr; die Liste `Language`
  bleibt `[de, en]`), Migration **00061** (`profiles` ohne `CHECK` auf `language` und `max_rating`)
- Betroffene Spec-IDs: C-3, K-2, B-3
- Harte Regeln: 1 (Codegen), 5 (Migration nach Vorlage), 7 (Vertrag), 8 (Tastaturtest), 9 (Texte nur aus
  den Sprachdateien), 10 (keine Secrets in Logs), 12 (Entscheid im Server)

## Kontext

Bis 1.33 kennt Slouchcat zwei Oberflächensprachen, Deutsch in Schweizer Schreibweise und Englisch. Die zwei
Codes stecken an vielen Stellen: im `CHECK` von `profiles.language`, in fünf Prüfungen im Server, in
`visibility.Profile` (jede andere Sprache wird für die Metadaten zu `de`), in den Server-Texten mit Rückfall
auf Deutsch und in der Web-App, die nur das angefragte Sprachbündel lädt. Alle `Intl`-Aufrufe formatieren mit
`de` und damit deutsche statt schweizerische Zahlen. ADR 0018 hatte 2026 neue Oberflächensprachen
ausgeschlossen (F-1: rund 2300 Texte je Sprache von Hand) und verlangt, dass die Serverliste der
Metadaten-Sprachen `de` enthält (F-9).

Marco will Slouchcat in rund zwei Dutzend Sprachen, «natürlich und sinnvoll übersetzt» (F-4), Deutsch in zwei
Varianten (F-3), und ein Haushalt soll Metadaten ohne Deutsch laden dürfen (F-7). Konzept 22 schneidet das in
fünf Stufen. Diese ADR hält die Entscheide fest; Stufe 1 baut das Gerüst ohne neue wählbare Sprache.

## Entscheid

| Thema | Entscheid |
|---|---|
| **Liste der Sprachen** | Die Vertragsliste `Language` enthält immer genau die Sprachen, deren Paket ausgeliefert ist. Sie wächst mit den Paketen, nicht im Voraus: Stufe 2 nimmt `de-DE` auf, Stufe 4 die übrigen 20. Ein neuer Code ist frühestens ein Release nach der Absicherung in der Web-App wählbar (siehe «Alte PWA»). |
| **Eine Prüfstelle im Server** | `model.UILanguages` (`server/internal/model/languages.go`) ist die einzige Liste, gegen die der Server eine Profilsprache prüft: Einrichtung (`RunSetup`, `CreateOwner`), Einladung, Profil anlegen und ändern. `TestUILanguagesMatchContract` hält sie gleich der Vertragsliste. Ohne Angabe bleibt `de` die Sprache eines neuen Profils (`model.DefaultUILanguage`). |
| **Liste in der Web-App** | `web/src/i18n/languages.ts` führt die ausgelieferten Sprachen mit Code, Namen in der eigenen Sprache und `intlLocale`, geprüft mit `satisfies` und einer Typprüfung gegen den generierten Typ `Language`. Die eigenen Namen sind Daten, nicht Oberflächentext (benannte Ausnahme von Regel 9). Solange nur ein Deutsch wählbar ist, heisst es «Deutsch»; mit `de-DE` in Stufe 2 wird daraus «Deutsch (Schweiz)». |
| **Deutsch in zwei Varianten** | `de` bleibt der Code für Deutsch (Schweiz), in jeder Datenbank und jedem Offline-Profil. `de-DE` ist eine Überlagerung: `web/src/locales/de-DE.json` hat nur die Texte, die in Deutschland anders lauten (ß nach langem Vokal oder Doppellaut, „…“ statt «…», 10.000 statt 10'000). In Stufe 1 sind es 205 Texte; die Datei ist erstellt, aber nicht wählbar. «dass», «lässt», «Passwort», «Schlüssel» behalten ss. |
| **Laden der Kette** | `ensureLanguage` lädt die ganze Kette: die Sprache, bei `de-DE` zusätzlich `de`, und immer `en`. Beim Start und beim Wechsel kommt Englisch nach dem ersten Bild im Hintergrund, damit der Start nicht wartet; bis dahin zeigt ein Text, der der Kette fehlt, seinen Schlüssel. Die Bündel kommen aus `import.meta.glob` über `web/src/locales/*.json`. |
| **Erkennung** | `detectLanguage`: für jede Browser-Sprache auf immer kürzeren Teilcodes zuerst die bekannten Zuordnungen (blosses `de`, `de-AT`/`LU`/`BE` → `de-DE`; `de-CH`/`LI` → `de`; `pt` → `pt-BR`; `zh-Hant`/`HK`/`MO` → `zh-TW`; `zh`/`zh-Hans`/`SG` → `zh-CN`; `no`/`nn` → `nb`), sonst der Code selbst (`fr-CH` → `fr`). Gewählt wird nur eine ausgelieferte Sprache; sonst **Englisch** statt bisher Deutsch (O-2). Solange `de-DE` nicht ausgeliefert ist, steht `de` dafür ein: Ein deutscher Browser bekommt in Stufe 1 also weiter Schweizer Deutsch, nicht Englisch. |
| **Alte PWA** | Eine zwischengespeicherte alte Web-App kann einen Code bekommen, den sie nicht kennt. Ab Stufe 1 zeigt sie dafür Englisch und wirft nicht (`applyProfileLanguage`, `setLanguage`). Ebenso zeigt sie ein Alter ausserhalb der FSK-Stufen als «ab 13» (`ratingLabel`) und behält es im Formular (`ratingOptions`, `ratingOf`), statt es beim Speichern auf eine andere Stufe zu setzen. Das braucht Stufe 3 (Mindestalter je Land). |
| **Datenbank** | Migration 00061 baut `profiles` neu, ohne `CHECK` auf `language` und auf `max_rating`, alles Übrige Zeichen für Zeichen wie vorher; dazu Indizes, Trigger `activity_events_profile_deleted` und `visible_items` (Stand 00056). Ein `CHECK` mit 23 Codes würde jede weitere Sprache zu einem Neubau machen (O-14). Der Server prüft weiter beides. Eingefrorener Stand vor der Migration: `testdata/db/sprachgeruest.sql`. |
| **Metadaten ohne Deutsch (F-7)** | **F-9 von ADR 0018 fällt:** Eine Metadaten-Sprachliste muss nur nicht leer, ohne Doppel und aus bekannten Codes sein, für den Server wie für eine Bibliothek. Der Fehler `ui_lang_required` gibt es nicht mehr. Die Verwaltungsseite lässt jeden Platz entfernen ausser dem letzten. |
| **Rückfallsprache und `MetadataBase`** | `visibility.Profile` macht aus einer fremden Oberflächensprache nicht mehr `de`. Die **Rückfallsprache** (zweites Glied der Kette aus ADR 0018, Sprache der Genres und Sammlungsnamen) ist `de` für `de` und `de-DE`, sonst `en` (`model.FallbackLanguage`). Ohne eigene Metadaten-Sprache liest ein Profil `model.MetadataBase` seiner Oberflächensprache: `de-DE` liest `de`, `nb` liest `no`, alle anderen ihren Code (`pt-BR`, `zh-TW` behalten die Region). Fehlt die Sprache in der Liste, greift die Kette wie bisher. |
| **Ton- und Untertitelsprachen neuer Profile** | `initialPrefs` schickt den Code durch `trackpick.Normalize`: `de-DE` → `de`, `pt-BR` → `pt`, `zh-TW` → `zh`, `nb` → `no`. Sonst speicherte ein neues Profil einen Wert, den das nächste Speichern mit `invalid_value` ablehnt. |
| **Server-Texte** | Die Texte aus `notify/texts.go` liegen eingebettet in `server/internal/notify/locales/{de,en}.json`, im Format der Web-App (i18next v4, `{{name}}`). Das neue Paket `server/internal/i18n` lädt sie und läuft dieselbe Kette (Sprache → `de` bei `de-DE` → `en`). Rückfall ist Englisch, auch in `NotifyAccount`/`NotifyOwner` für ein Konto ohne Profil. Die HTML-Mail trägt `lang`. Nicht übersetzt werden Ankündigungen, API-Fehlertitel und Log-Zeilen. |
| **Formate** | Jeder `Intl`-Aufruf nimmt `intlLocale(i18n.language)`: `de` → `de-CH`, sonst der Code. Deutsch (Schweiz) formatiert damit `1’234’567.5`, `50%` und `1.5×`, Deutsch (Deutschland) später `1.234.567,5`. Auch `{{x, number}}` in Texten folgt der Regel. Eine ESLint-Regel verbietet `i18n.language` als Argument von `Intl`, `toLocale*` und `localeCompare`. |
| **Schrift** | Keine Webfonts. Für `ja`, `zh-CN`, `zh-TW` und `ko` gibt es eigene Schriftstapel als Tokens (`font.family.*` in `typography.json`), gesetzt unter `html:lang(…)`, damit das Gerät die richtige Form eines vereinheitlichten Han-Zeichens wählt; Koreanisch bricht mit `word-break: keep-all`. |
| **Länge** | Die Pseudo-Sprache `en-XA` (jeder Text 40 % länger, in eckigen Klammern) gibt es nur in der Entwicklung (`/?lng=en-XA`) und in Storybook (Werkzeugleiste «locale»). Sie ist nie ausgeliefert und nie im Vertrag. Die Vergleichsbilder bleiben Deutsch. |
| **Rechts nach links** | Nicht in diesem Konzept (O-16). `<html dir="ltr">` ist gesetzt. stylelint verlangt für neues CSS logische Eigenschaften; die 49 Dateien, die heute 144 physische Angaben haben, stehen als Ausnahmen in `web/stylelint.config.js`, und keine Datei kommt dazu. |
| **Neue Texte im Alltag** | `de` und `en` sind in jedem Pull Request Pflicht, `de-DE` sobald ein Text ein ss-Wort oder «Guillemets» enthält; die übrigen Sprachen vor jedem Release (Konzept 22, 4.8; der Release-Schritt kommt mit Stufe 5). |
| **Übersetzung (F-1 von ADR 0018 fällt)** | Die Erstfassung der 20 weiteren Sprachen liefert Claude mit Glossar, Stilregeln und Kontext, geprüft von einem zweiten Agenten; danach öffnet Weblate «Libre» für Korrekturen (Stufen 4 und 5). |

## Schritt 0 der Stufe 1: Befunde

- **Feste `de` im Code (Inventur am Stand 1.33.0):**
  - *Ersetzt:* die fünf Prüfstellen der Profilsprache (jetzt `model.UILanguages`), `visibility.Profile`
    (jetzt `FallbackLanguage`/`MetadataBase`), der Rückfall der Server-Texte und von `ownerLanguage`, der
    `COALESCE(…, 'de')` in `notify_channels.sql`, die Regel «Serverliste braucht `de`» in
    `metadata/settings.go` und auf der Verwaltungsseite `MetadataPage.tsx`.
  - *Bleibt bewusst:* die Vorgaben `["de","en"]` der Metadaten-Einstellungen und ohne Quelle
    (`metadata/settings.go`, `auth.AvailableMetadataLanguages`, `admin.go` Anbieter-Sprachen), die
    Zuordnung `de` → `de-DE` für TMDB, der Zusatz «deutsch» der YouTube-Suche bei `de`, «Staffel N» nur für
    Deutsch (sonst «Season N», ADR 0018), die zweisprachigen Sammlungsnamen `name`/`name_de` (ausgewählt
    über die Rückfallsprache), Artikel beim Sortieren (`model/sortkey.go`, nur de/en, Konzept 22 4.10),
    die Hörbuch-Sprache (`library/audiobook*.go`, `metadata/book_write.go`: Sprache der Dateien),
    `admin/deletion.go` (`de` nur ohne Metadaten-Einstellungen) und in der Web-App `TrailersTab.tsx` (Sprache
    eines neuen Trailers) sowie `catalog/labels.ts` (Namen von `de`/`en`, sonst der Code).
  - *Kein Code setzt eine `de`-Zeile voraus*, die nach F-7 leer bliebe: Genres, Texte und Sammlungen laufen
    über die Kette mit Rang; der Sortierschlüssel nimmt die Sprache des Titels.
- **`nb` oder `no` bei TMDB:** TMDB führt Norwegisch unter `no` (Übersetzungen als `no-NO`, ein `nb`
  erscheint dort nicht; geprüft an der öffentlichen Übersetzungsseite eines Films). `MetadataBase` macht
  darum aus `nb` ein `no`, und `trackpick.Normalize` tut für die Tonspuren dasselbe.
- **Lädt die PWA bei einer neuen Serverversion selbst neu?** Nein. Der Service Worker (`generateSW`,
  `autoUpdate` mit `skipWaiting` und `clientsClaim`) übernimmt die Seite, aber niemand lädt sie neu (kein
  `controllerchange`-Hörer). Ein offener Tab läuft mit dem alten Code weiter, bis er neu geladen oder neu
  gestartet wird. Darum muss die Absicherung ein Release vor jedem neuen Code ausgeliefert sein.
- **Nummern:** Vertrag 1.34.0, Migration 00061 (nach K21 Stufen 4–6 mit 1.33.0 und 00058–00060).
- **Release-Ablauf:** Es gibt kein eigenes Dokument; der Ablauf steckt in `.github/workflows/release.yml`
  und in den Einträgen von `docs/dev/CURRENT_STATUS.md`. Der Schritt «Übersetzungen nachziehen» (4.8) braucht
  ein eigenes `docs/dev/RELEASE.md`; das gehört zu Stufe 5.

## Abweichungen vom Konzept in Stufe 1

- **Blosses `de` und `de-AT` → `de`:** Das Konzept will `de-DE`, das in Stufe 1 noch nicht wählbar ist. Die
  Erkennung nimmt dann Deutsch (Schweiz) statt Englisch; ab Stufe 2 greift die Zuordnung von selbst.
- **Sprachname «Deutsch»** statt «Deutsch (Schweiz)», solange es nur ein Deutsch gibt.
- **`ui_lang_required` entfernt**, im Server, im Vertragstext, in der Fehlerzuordnung und den Texten.
- **Geänderte Texte (Delta für die Übersetzung):** neu `rating.age` und `login.language`; geändert
  `admin.metadata.languages.intro` und `admin.metadata.languages.fallback`; `library.providers.noGerman`
  heisst jetzt `library.providers.missingLanguages` («liefert kein {{languages}}»: die Sprachen der
  Serverliste, die die anderen Quellen liefern und diese nicht); entfernt
  `admin.metadata.errors.ui_lang_required`.
- **Fehler im Text `root_content_changed_body` behoben:** Er nannte den alten Inhalt, wo der neue hingehört
  («liest jetzt <alt> statt <neu>»); mit benannten Platzhaltern steht jetzt «liest jetzt {{to}} statt
  {{from}}».
- **Grenze der ESLint-Regel:** Sie sieht nur `i18n.language` direkt im Aufruf, nicht über eine Variable,
  einen Parameter einer Hilfsfunktion, einen anderen Namen als `i18n` oder `i18n.resolvedLanguage`. Das
  prüft das Review (Kommentar in `web/eslint.config.js`).
- **Bekannte Punkte, bewusst offen:**
  - Das `lang` der HTML-Mail liest `DeliveryLanguage` beim Zustellen; der Text wurde beim Einreihen in der
    Sprache des Eigentümers geschrieben. Ändert jemand dazwischen die Sprache, passt `lang` nicht zum Text.
  - Für die Meldung «Neue Folge» baut `notify.go` das Profil ohne `metadata_language`: Der Titel kommt
    in `MetadataBase` der Oberflächensprache, auch wenn das Profil eine eigene Metadaten-Sprache hat.
  - `profiles.settings.ratingAboveAccount` nennt fest «FSK»; das ändert Stufe 3. *Erledigt in Stufe 3:* der
    Schlüssel heisst jetzt `profiles.settings.ratingAboveCeiling` und nennt die Stufen des Freigabelands
    (ADR 0034).
  - Die festen Texte der Schrittweite im Player («−0,1», «0,1 s früher») schreiben ein Komma, während
    `intlLocale` in `de` jetzt einen Punkt formatiert («1.25×»). Gehört zur Durchsicht der Texte.
- **Fallen beim Bauen:** Der Dateicache von Vitest (`node_modules/.vitest-cache`) übersieht eine neue Datei
  in einem `import.meta.glob`; nach dem Anlegen einer Sprachdatei lokal den Ordner löschen. Node schreibt
  in `de-CH` den Tausender-Apostroph `'`, Browser `’`; Tests rechnen die Zahl darum mit
  `Intl.NumberFormat('de-CH')` aus.

## Stufe 2: Deutsch (Deutschland) wählbar

- Gebaut mit dem geführten Assistenten (Konzept 22 Teil A), Vertrag **1.35.0**: `de-DE` steht in
  `languages.ts`, `model.UILanguages` und der Vertragsliste `Language` (`[de, de-DE, en]`). `de` heisst
  jetzt «Deutsch (Schweiz)». Die Erkennung brauchte keine Änderung: blosses `de`, `de-AT`, `de-LU` und
  `de-BE` landen jetzt von selbst bei `de-DE`; die Ersatzregel aus Stufe 1 ist entfernt.
- Die Profilliste zeigt den Sprachnamen aus `languages.ts` statt eines Texts `language.<code>`.
- `server/internal/notify/locales/de-DE.json` ist leer (`{}`): Kein Server-Text enthält ein ss-Wort mit
  langem Vokal oder «Guillemets». `TestServerTexts` prüft die Überlagerung wie die Web-App (nur
  vorhandene Schlüssel, jeder anders als `de`).
- Durchsicht von `web/src/locales/de-DE.json`: «Appenzell Ausserrhoden» behält das ss (amtlicher Name
  des Kantons); der Eintrag `radio.region.CH-AR` ist entfernt. Neu überlagert sind die fünf Texte des
  Assistenten mit «schliessen», «heisst» und «Passwort vergessen».
- `locales.test.ts` verlangt die Vollständigkeit nicht von `de-DE` (Überlagerung); Platzhalter und
  Pluralformen prüft er weiter.
- **Delta für die Übersetzung (Stufe 4):** neu `setup.*` (ganzer Block des Assistenten ausser
  `setup.library.{name,kind,path,pathPlaceholder,warning}`), `system.gpu.choice.*`,
  `system.gpu.fallback.*`, `admin.home.setupBand.*`; entfernt `setup.title`, `setup.intro`, `setup.step`,
  `setup.account.{title,description}`, `setup.library.{title,description,defaultName,roots,check,
  checking,content,providers}`, `setup.tmdb.*`, `setup.submit`, `setup.submitting`, `setup.system.*`.

## Was von ADR 0018 bleibt

Die geordnete Metadaten-Sprachliste des Servers mit Abweichung je Bibliothek, die Kette Metadaten-Sprache →
Rückfallsprache → Rang, `profiles.metadata_language`, die Bildwahl mit Sprachkette und lokalen Dateien und das
Freigabeland. Neu ist nur, dass die Liste kein Deutsch enthalten muss und das zweite Glied der Kette je
Oberflächensprache `de` oder `en` ist.

## Folgen

- Stufe 2 hat `de-DE` in `languages.ts`, `model.UILanguages` und die Vertragsliste aufgenommen und `de` in
  «Deutsch (Schweiz)» umbenannt (Abschnitt «Stufe 2»).
- Stufe 3 darf Mindestalter wie 13 oder 17 in `profiles.max_rating` schreiben, ohne die Tabelle neu zu bauen.
- Stufe 4 legt je Sprache eine Datei in `web/src/locales/` und `server/internal/notify/locales/` an;
  `locales.test.ts`, `TestServerTexts` und `e2e/languages.spec.ts` nehmen sie von selbst auf.
- Storybook: Der Lauf `task web:storybook:update` nach dem Wechsel auf `intlLocale` hat kein Vergleichsbild
  geändert (526 von 526 gleich). Die Geschichten zeigen keine Zahl, deren Schweizer Form von der deutschen
  abweicht; Zahlen kommen dort als fertige Texte aus den Argumenten.
