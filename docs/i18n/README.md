# Übersetzungen

Slouchcat gibt es in 23 Oberflächensprachen (Konzept 22, Abschnitt 4.1; ADR 0033): Deutsch (Schweiz),
Deutsch (Deutschland), Englisch und zwanzig weitere. Diese Seite sagt, woher die Texte kommen, wer was
pflegt und wie man prüft, ob eine Sprache vollständig ist.

## Die Dateien

| Was | Wo |
|---|---|
| Texte der Web-App | `web/src/locales/<code>.json` (i18next v4) |
| Texte, die der Server selbst schreibt (E-Mail, Web-Push, Telegram) | `server/internal/notify/locales/<code>.json` (gleiches Format) |
| Welche Sprachen wählbar sind | `web/src/i18n/languages.ts`, `model.UILanguages` und die Liste `Language` in `api/openapi.yaml` |
| Feste Begriffe je Sprache | `docs/i18n/glossar.md` |
| Anrede, Anführungszeichen, Schreibung je Sprache | `docs/i18n/stil/<code>.md` |
| Hilfsskripte | `scripts/i18n/` (`check.ts`, `context.ts`, `chunks.ts`) |

- **Quelle ist Deutsch (Schweiz)** (`de.json`). Englisch (`en.json`) ist die Vorlage für die übrigen
  Sprachen, weil die meisten Übersetzer Englisch lesen.
- **`de-DE.json` ist eine Überlagerung.** Sie enthält nur die Texte, die in Deutschland anders lauten
  (ß, „…“ statt «…», 10.000 statt 10'000). Alles Übrige kommt aus `de.json`.
- **Fehlt einer Sprache ein Text, zeigt die App Englisch**, nie den Schlüssel: Die Web-App lädt immer
  die ganze Kette (Sprache, bei `de-DE` zusätzlich `de`, dann `en`), der Server läuft dieselbe Kette.

## Wie die Übersetzungen entstehen

Marco will Slouchcat «natürlich und sinnvoll übersetzt», nicht Wort für Wort (F-4). Die zwanzig
weiteren Sprachen sind so entstanden (Konzept 22, Abschnitt 4.6; F-5):

1. **Erstfassung durch Claude.** Übersetzt wird in Blöcken je Bereich (`scripts/i18n/chunks.ts split`),
   mit Deutsch **und** Englisch als Quelle, dem Glossar, den Stilregeln der Sprache und dem Kontext
   jedes Schlüssels: Datei, Zeile und Komponente, an der er verwendet wird (`scripts/i18n/context.ts`).
   So weiss die Übersetzung, ob «Play» ein Knopf, eine Überschrift oder ein Zustand ist.
2. **Prüfung durch einen zweiten Agenten**, der nicht übersetzt hat: Glossartreue, Platzhalter,
   Pluralformen, Länge der Knöpfe (höchstens rund 1,5× Englisch), Ton und eine Rückübersetzung von
   50 zufälligen Texten ins Deutsche.
3. **Bildprüfung:** ein Storybook-Durchlauf in mehreren Sprachen, auf abgeschnittene und überlaufende
   Texte durchgesehen. Das Ergebnis der Runde steht in `docs/perf/22-uebersetzungen.md`.
4. **Stichproben durch Marco** in Deutsch (Deutschland) und Englisch (F-15). Die übrigen Sprachen liest
   er nicht; für sie gilt, was die Agenten geprüft haben.

**Claudes Texte erscheinen in Weblate als fertig übersetzt.** Das Format i18next JSON kennt kein Merkmal
«ungeprüft». Wer eine der zwanzig Sprachen als Muttersprache spricht, findet also Texte, die noch kein
Mensch gelesen hat. **Korrekturen sind sehr willkommen.** Den Weg über Weblate richtet Stufe 5 von
Konzept 22 ein; bis dahin nimmt das Projekt Korrekturen als Pull Request an.

## Neue Texte im Alltag

Nicht jeder Pull Request soll zwanzig Übersetzungen tragen (Konzept 22, Abschnitt 4.8):

- **Im Pull Request Pflicht:** `de` und `en`. `de-DE` dazu, sobald der neue Text ein ss-Wort mit langem
  Vokal oder Doppellaut («Strasse», «gross», «schliessen») oder «Guillemets» enthält. «dass», «Klasse»
  und «Passwort» bleiben auch in Deutschland bei ss. Dasselbe gilt für die Server-Texte.
- **Die übrigen zwanzig Sprachen** dürfen in `main` Texte vermissen. Die Tests melden das nur als
  Warnung; die App zeigt bis zum nächsten Release Englisch.
- **Vor jedem Release, auch vor einem Patch: «Übersetzungen nachziehen».** Claude übersetzt die
  fehlenden Texte nach dem Ablauf oben, und `task i18n:check` muss grün sein. Erst dann wird getaggt.
  Bei einem Patch-Release (Versionscommit auf dem Fix-Zweig) läuft der Schritt auf dem Fix-Zweig.
- **Welche Sprachen wählbar sind,** steht fest in `languages.ts` und im Vertrag, nicht in einer Zählung
  zur Laufzeit. Eine Sprache verschwindet also nie, weil ein Text fehlt.

## Prüfen

| Befehl | Was er tut |
|---|---|
| `task i18n:status` | Zählt je Sprache die fehlenden Texte der Web-App. |
| `node scripts/i18n/check.ts <code> --missing` | Listet die fehlenden Schlüssel einer Sprache. |
| `node scripts/i18n/check.ts <code>` | Prüft eine Sprache: fehlende Texte als Warnung, Fehler im Aufbau als Fehler. |
| `task i18n:check` | Prüft jede Sprachdatei streng, fehlende Texte eingeschlossen. Das ist das Gate vor dem Release. |

`check.ts` meldet als **Fehler**: Schlüssel, die `en.json` nicht hat, Pluralformen, die nicht genau den
Kategorien der Sprache entsprechen (`Intl.PluralRules`), abweichende Platzhalter, `$t()`-Verweise oder
Markup, leere Texte, eine fehlende Zahl in einer Pluralform, die mehr als 1 abdeckt, und Leerraum am
Anfang oder Ende, der von Englisch abweicht (`" · "`, `" oder "`). Japanisch und Chinesisch sind von
der letzten Regel ausgenommen, weil ihre Trenner ohne Leerzeichen stehen (`"、"`); leer darf der Text
auch dort nicht sein. Koreanisch schreibt Leerzeichen zwischen den Wörtern und hält sich an die Regel. Als **Warnung** meldet es Texte, die gleich wie Englisch lauten («Trailer»
ist in vielen Sprachen richtig), und Beschriftungen, die mehr als 1,5× so lang sind wie Englisch.

In der CI prüfen `web/src/locales/locales.test.ts` (Web-App) und `TestServerTexts` (Server) bei jedem
Commit: `de`, `de-DE` und `en` streng, die übrigen Sprachen mit fehlenden Texten als Warnung, aber mit
Fehlern im Aufbau als Fehler.

## Eine Sprache dazunehmen

Eine weitere Sprache ist eine Codeänderung, kein Eintrag in einer Einstellung: Datei in
`web/src/locales/` und `server/internal/notify/locales/`, Eintrag in `languages.ts` (Name in der eigenen
Sprache), in `model.UILanguages` und in der Liste `Language` des Vertrags (eine neue Minor-Fassung),
Stilregeln, Spalte im Glossar, Vorschläge des Assistenten (`RegionStep.tsx`). Eine zwischengespeicherte
alte Web-App zeigt einen Code, den sie nicht kennt, auf Englisch an. Sprachen, die von rechts nach links
laufen, sind bewusst nicht vorgesehen (Konzept 22, Abschnitt 4.10).
