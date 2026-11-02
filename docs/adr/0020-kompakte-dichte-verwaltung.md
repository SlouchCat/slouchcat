# ADR 0020: Kompakte Dichte der Verwaltung

- Status: angenommen (2. November 2026); **ergänzt ADR 0019** (die kompakte Dichte gilt im
  Inhaltsbereich unter `[data-shell="admin"]`, das Konzept 11 einführt; Verwaltungsleiste und
  App-Kopf behalten ihre Masse)
- Quelle: Konzept `docs/planning/12-verwaltungsseiten.md` (Fassung 3, 1. November 2026,
  Abschnitte 5, 10.1, 11.2, 11.6 und 11.8); **Frage 18** in `docs/planning/04-entscheidungen.md`
  mit Marcos Wahl je Seite, seinen Änderungswünschen und den Antworten vom 2. November 2026;
  Konzept 11 (`docs/planning/11-verwaltung-uebersicht.md`, 11.6, `data-shell="admin"`)
- Betroffene Spec-IDs: A-2
- Harte Regeln: 8 (Tastaturtests der umgebauten Seiten), 9 (keine Farbe und kein Text hart im Code;
  Admin und Zuschauer nutzen dieselben Komponenten aus `web/src/ui`), 11 (keine neue Funktion durch
  die Dichte)

## Kontext

Marco hat am 1. November 2026 über die Verwaltung gesagt: «Im Prinzip möchte ich alle Seiten in
der Verwaltung überarbeiten. Es ist mir alles irgendwie zu gross und zu unübersichtlich.» Die
Verwaltung benutzt heute dieselben Masse wie die Zuschauerseiten, die für Sofa-Abstand und
Fernbedienung gemacht sind: Seitentitel 60 px (`Text variant="display"`), Knöpfe und Felder 44 px
(`--sc-size-control-height`), Panels mit 24 px Innenabstand und 20 px Radius, Abschnitte 40 bis
48 px auseinander, Seitenrand bis 64 px (Konzept 12, 1.1, P-1 bis P-6).

Im Interview hat Marco festgelegt: die kompakte Dichte gilt **nur unter `/admin`**, Dialoge aus
einer Verwaltungsseite sind **ebenfalls kompakt**, `/settings/*` (Profile, Benachrichtigungen)
bekommt das Seitenmuster, aber die heutige Dichte (Konzept 12, 10.1).

Die Bausteine in `web/src/ui` lesen heute die allgemeine Skala (`Panel.module.css:4-9`
`space-6`/`space-5`/`radius-xl`, `Button.module.css:11` `font-size-md`, `DataTable.module.css:28,52`
`space-3`/`space-4`). Regel 9 verlangt, dass Verwaltung und Zuschauer dieselben Komponenten nutzen.
Eine zweite Dichte im Tokensystem ist darum ein Architekturentscheid.

## Entscheid

**Komponententokens mit dem heutigen Wert als Standard.** `packages/design-tokens/tokens/` bekommt
Komponententokens, deren Standardwert der heutige Wert ist: `--sc-control-height`,
`--sc-control-font-size`, `--sc-panel-padding`, `--sc-panel-gap`, `--sc-panel-radius`,
`--sc-table-cell-padding-y`, `--sc-page-title-size`, `--sc-section-gap`. `Button`, `Field`, `Panel`,
`DataTable`, `Dialog` und `Text` (neue Variante `pageTitle`) lesen sie statt der allgemeinen Skala.
`--sc-size-control-height` bleibt unverändert als «mindestens 44 px» dokumentiert
(`tokens/space.json`); die Kompaktgrösse ist ein eigener Token.

**Gruppe `density.compact`.** Eine Dichte-Gruppe liefert die kompakten Werte:

| Grösse | Standard (heute) | Kompakt |
|---|---|---|
| Seitentitel | 60 px, `display` | 24 px, 600 |
| Steuerelement-Höhe | 44 px | 32 px |
| Schrift in Knöpfen und Feldern | 16 px | 14 px |
| Panel Innenabstand / Lücke / Radius | 24 / 20 / 20 px | 16 / 12 / 12 px |
| Abschnittsabstand | 40 bis 48 px | 24 px |
| Tabellenzeile | rund 48 px | 40 px |
| Seitenrand | bis 64 px | 24 bis 32 px |

**Wo sie wirkt.** Nur im Inhaltsbereich (`#content`) und in den Dialogen der Verwaltungsseiten,
unter `[data-shell="admin"]` (Attribut aus Konzept 11) und nur mit
`@media (min-width: 48rem) and (pointer: fine)`. App-Kopf und Verwaltungsleiste behalten ihre
Masse. Dialoge liegen im Portal ausserhalb von `#content`; der Kontext `AdminDensityScope`, den
`AdminPage` setzt, gibt ihnen `data-density="compact"` an ihrem eigenen Portal. Andere Portale auf
`/admin` (Schublade der App, Ankündigungsdialog der Zuschauer) bleiben in heutiger Grösse.
Tablets mit Touch und Handys behalten 44 px; `/settings/*` behält die heutige Dichte.

**Wann sie eingeschaltet wird.** Erst mit der letzten umgebauten Seite (Schritt K12-8). Bis dahin
bauen die Commits die Seiten auf die neuen Bausteine um, in heutiger Grösse; es gibt keine
Zwischenstände mit gemischter Grösse.

## Geprüfte und verworfene Alternativen

- **Eigene Admin-Kopien der Bausteine** (zum Beispiel `AdminButton`, `AdminPanel`): verstösst gegen
  Regel 9 (Admin und Zuschauer nutzen dieselben Komponenten), verdoppelt Pflege, Stories und
  Referenzbilder, und jede Korrektur am Original müsste nachgezogen werden.
- **Die allgemeine Skala umstellen** (`space-*`, `font-size-*`, `radius-*` unter `/admin` kleiner
  setzen): die Skala tragen auch Fliesstext, Chips, Abstände in Listen und Bausteine, die mit der
  Dichte nichts zu tun haben; alles würde mitschrumpfen, auch dort, wo es nicht soll, und die
  Wirkung wäre nicht je Baustein steuerbar.
- **CSS `zoom`** (oder `transform: scale`) auf dem Inhaltsbereich: verkleinert Fokusringe,
  Rahmen und Trefferflächen mit, rechnet Mauspositionen und Pixelgrenzen ungenau, verzerrt
  Bildschirmlupe und Textgrösse des Browsers und ist für Barrierefreiheit nicht prüfbar.

## Barrierefreiheit

32 px Steuerelemente liegen unter der 44-px-Empfehlung für Berührung, aber über WCAG 2.2 AA
(2.5.8 «Target Size (Minimum)», 24 × 24 px). Darum gilt die kompakte Dichte nur mit Maus
(`pointer: fine`) und ab 768 px; Touch-Geräte behalten 44 px. Farben und Kontraste bleiben
unverändert (dieselben Farb-Tokens).

## Folgen

- **Pixelgleiche Referenzbilder nach K12-2a:** der Commit mit den Komponententokens ändert keine
  Story, und alle bestehenden Storybook-Referenzbilder bleiben pixelgleich. Das ist der Beweis,
  dass ausserhalb von `/admin` nichts kleiner wird. Erst die Erweiterungen der Bausteine (K12-2b)
  ändern Bilder.
- **Story je Baustein in beiden Dichten:** ein Storybook-Dekorator `density` rendert jeden
  Baustein in Standard und kompakt, damit die kompakte Dichte eine Bildprüfung hat. Seiten haben
  keine eigenen Referenzbilder.
- Neue Verwaltungsseiten bekommen die Dichte ohne eigenes CSS, sobald sie `AdminPage` und die
  Bausteine aus `web/src/ui` benutzen; die seiteneigenen CSS-Module für Kopf und Abstände entfallen
  (S-12, K12-7).
- stylelint prüft nur Farbeigenschaften; die neuen Tokens sind Masse. Regel 9 bleibt erfüllt.
- K12-8 schaltet die Dichte ein und wiederholt die Messung der Höhe bis zur ersten Arbeitszeile
  auf Hintergrundjobs, Integrationen und Konten (Ziel: höchstens die Hälfte,
  `docs/perf/verwaltungsseiten.md`).

## Restrisiken

- **Viele Bilder ändern sich in K12-8 auf einmal.** Die Komponententokens kommen darum in K12-2a
  mit pixelgleichen Bildern; die Umstellung selbst ist ein eigener Schritt.
- **Ein Baustein liest die Skala weiter direkt** und bleibt in der kompakten Dichte gross. Die
  Story in beiden Dichten zeigt das in der Bildprüfung.

## Nachtrag K12-8 (2. November 2026)

- **`AdminHomePage` setzt den Kontext selbst.** Die Übersicht `/admin` benutzt nicht `AdminPage`,
  öffnet aber eine Rückfrage (Server neu starten). Damit diese Rückfrage wie die Dialoge der
  übrigen Verwaltungsseiten kompakt ist, umschliesst `AdminHomePage` ihren Inhalt ebenfalls mit
  `AdminDensityScope` (`AdminHomePage.test.tsx` prüft `data-density="compact"` am Portal).
- **Zwei Tokens mehr als im Entscheid aufgezählt:**
  - `page.title-line-height` (Komponententoken `--sc-page-title-line-height`): Standard 1.5 (was ein
    h1 heute erbt), kompakt 0. Die Höhe der Kopfzeile bestimmt dann der 24-px-Titel, nicht die
    48 px hohe Zeile, die ein h1 mit seinen 2em sonst mitbringt.
  - `density.compact.page.gutter` (32 px): setzt in `#content` die bestehende Eigenschaft
    `--page-gutter` aus `web/src/ui/global.css` (40 px ab 768 px, 64 px ab 1440 px). Es ist kein
    `--sc-*`-Komponententoken mit Standardwert; ausserhalb der kompakten Dichte gilt weiter
    `global.css`.

## Nachtrag Designer-Abgleich (4. November 2026)

- **`text.body-size`** (Komponententoken `--sc-text-body-size`): Grösse des Fliesstexts (`Text`
  Variante `body`: Hinweise, Leerzustände, Zustand der Speicherleiste). Standard `font.size.lg`
  (20 px, wie bisher), kompakt `font.size.sm` (14 px, wie in den Mockups).
- **`table.row-height`** (Komponententoken `--sc-table-row-height`): Mindesthöhe einer Tabellenzeile
  als Höhe der Zelle. Standard `0px` (Innenabstand und Inhalt bestimmen die Höhe wie bisher), kompakt
  40 px (S-5). Dazu sinkt `density.compact.table.cell-padding-y` von 9 auf 6 px: Faltknopf, Chip und
  Kontrollkästchen (24 px) passen so in 40 px, Textzeilen hält die Mindesthöhe auf 40 px. Die Zellen
  stehen in der Mitte der Zeile (`vertical-align: middle`), auch in der Standarddichte.
- **`setting.help-size`** und **`setting.text-gap`** (Komponententoken `--sc-setting-help-size`,
  `--sc-setting-text-gap`): Hilfezeile einer Einstellungszeile (`SettingRow`) und ihr Abstand zur
  Bezeichnung. Standard 14 px und 4 px wie bisher, kompakt 12 px und 0 wie in den Mockups (Int-A,
  Spo-A, Met-B); eine Einstellung mit einer Zeile Hilfe ist so rund 56 px hoch.
