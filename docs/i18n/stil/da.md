# Stilregeln: Dansk (`da`)

Dänisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **du** (O-1), klein geschrieben: «Vælg din profil.»
- Kurz und freundlich wie Netflix Danmark. Keine Ausrufezeichen in Meldungen.
- Gleiche Wörter wie Englisch («Film», «Trailer», «Podcast», «Downloads») sind richtig; `check.ts`
  meldet sie nur als Warnung.

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Imperativ:** «Gem», «Annuller», «Tilføj mappe», «Spring intro over».
- **Satzschreibung:** nur das erste Wort gross («Tilføj bibliotek»).
- Zusammensetzungen zusammenschreiben («baggrundsjob», «afsnitsbillede»); Bindestrich nur bei
  Abkürzungen und Namen («TMDB-nøgle»).

## Anführungs- und Satzzeichen

- Anführungszeichen **» … «**, darin › … ‹.
- Auslassung **…** direkt am Wort: «Indlæser…».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1.234.567,5`).
- Prozent mit Leerzeichen: «{{percent}} %». Einheiten: «{{hours}} t {{minutes}} min», «{{seconds}} sek.».
- Datum: «den {{date}}», «kl. {{time}}».

## Pluralformen: `one`, `other`

- `one` gilt für 1 (und für Bruchzahlen wie 1,5); `other` für alle übrigen Zahlen.

```json
"episodes_one": "{{count}} afsnit",
"episodes_other": "{{count}} afsnit"
```

- «afsnit» ist im Plural gleich; beide Schlüssel trotzdem anlegen.

## Platzhalter und Grammatik

- Bestimmte Form hängt am Nomen («biblioteket»); vor Platzhaltern Nomen einschieben: «biblioteket
  {{name}}».
- Genitiv-s nicht an Platzhalter hängen.

## Länge

Etwa so lang wie Englisch; lange Zusammensetzungen auf Knöpfen vermeiden.

## Beispiele

| Schlüssel | de | en | da |
|---|---|---|---|
| `common.save` | Speichern | Save | Gem |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Vælg din profil. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Din liste er tom |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Sikkerhedskopien »{{name}}« er oprettet. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Lydstyrke {{percent}} % |
| `common.loading` | Wird geladen … | Loading … | Indlæser… |
| `player.skipIntro` | Intro überspringen | Skip intro | Spring intro over |
