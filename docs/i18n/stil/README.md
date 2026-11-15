# Stilregeln je Sprache

Je Zielsprache eine Datei `<code>.md` (Konzept 22, Abschnitt 4.6). Sie gilt zusammen mit dem Glossar
`docs/i18n/glossar.md` für die Erstfassung durch Claude, für den Prüfagenten und später für Weblate.

## Was für alle Sprachen gilt

- **Natürlich, nicht wörtlich (F-4).** Übersetzt wird, was der Text tut, so wie eine gute App in der
  Sprache es sagen würde. Deutsch und Englisch sind beide Quelle; bei Zweifel gilt der deutsche Sinn.
- **Kontext zuerst.** Zu jedem Schlüssel nennt `scripts/i18n/context.ts` Datei, Zeile und Komponente.
  Ein «Play» auf einem Knopf ist ein Befehl, in einer Statuszeile ein Zustand.
- **Platzhalter** `{{name}}` bleiben unverändert, auch mit Format (`{{count, number}}`). Ihre Stellung im
  Satz darf sich ändern. Markup wie `<path>…</path>` oder `<page>…</page>` bleibt mit demselben Namen
  und umschliesst das passende Wort.
- **Pluralformen** genau nach `Intl.PluralRules(<code>)`: Jede Kategorie der Sprache bekommt einen
  eigenen Schlüssel (`_one`, `_few`, `_many`, `_other`), dazu `_zero`, wo `en.json` ihn hat. In jeder
  Form steht die Zahl als Platzhalter, nie als Ziffer oder Zahlwort («1», «un», «один»), sobald die
  Kategorie mehr als die Zahl 1 umfasst. `scripts/i18n/check.ts` prüft das.
- **Länge:** Knöpfe, Reiter, Menüpunkte und Spaltenköpfe höchstens rund 1,5× so lang wie Englisch. Ist
  es länger, ein kürzeres Wort wählen, nicht abkürzen mit Punkt.
- **Nicht übersetzen:** Liste im Glossar, Abschnitt 9.
- **Kein Schweizer Sonderfall:** Die Zielsprachen haben keine Schweizer Varianten. `fr` ist Französisch
  aus Frankreich, `it` Italienisch aus Italien.

## Dateien

`fr` `es` `it` `nl` `pt-BR` `pt-PT` `pl` `cs` `hu` `tr` `sv` `da` `nb` `fi` `ru` `uk` `ja` `ko` `zh-CN`
`zh-TW`
