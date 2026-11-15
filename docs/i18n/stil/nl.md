# Stilregeln: Nederlands (`nl`)

Niederländisch (Niederlande und Flandern; Wortwahl der Niederlande). Gilt mit dem Glossar und
`README.md` in diesem Ordner.

## Anrede und Ton

- **je / jij** (O-1), ungezwungen wie Netflix Nederland: «Kies je profiel.»
- «je» statt «jij», wo keine Betonung nötig ist. Besitz: «je» oder «jouw» nur betont.
- Keine Ausrufezeichen in Meldungen.

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Infinitiv:** «Opslaan», «Map toevoegen», «Intro overslaan».
- **Satzschreibung:** nur das erste Wort gross («Bibliotheek toevoegen»).
- Zusammensetzungen zusammenschreiben («afleveringsafbeelding», «achtergrondtaak»). Mit Bindestrich nur
  bei Abkürzungen, Namen und Ziffern: «TMDB-sleutel», «4K-versie».

## Anführungs- und Satzzeichen

- Anführungszeichen **“ … ”**, darin ‘ … ’.
- Auslassung **…** direkt am Wort: «Laden…».
- Apostroph im Plural von Wörtern auf Vokal: «foto’s», «video’s» (U+2019).

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1.234.567,5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}} u {{minutes}} min», «{{seconds}} s».
- Datum: «op {{date}}», «om {{time}}».

## Pluralformen: `one`, `other`

- `one` nur für 1, `other` für 0 und alle übrigen Zahlen.

```json
"episodes_one": "{{count}} aflevering",
"episodes_other": "{{count}} afleveringen"
```

## Platzhalter und Grammatik

- «de» oder «het» vor Platzhaltern vermeiden; ein Nomen einschieben: «de bibliotheek {{name}}».
- Wortstellung im Nebensatz (Verb am Ende) gilt auch mit Platzhaltern.
- Gleiche Wörter wie Englisch («Film», «Trailer», «Podcast») sind richtig; `check.ts` meldet sie nur als
  Warnung.

## Länge

Niederländisch ist rund 10–20 % länger als Englisch; lange Zusammensetzungen auf Knöpfen vermeiden.

## Beispiele

| Schlüssel | de | en | nl |
|---|---|---|---|
| `common.save` | Speichern | Save | Opslaan |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Kies je profiel. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Je lijst is leeg |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Back-up “{{name}}” gemaakt. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Volume {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | Laden… |
| `player.skipIntro` | Intro überspringen | Skip intro | Intro overslaan |
