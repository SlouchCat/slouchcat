# Stilregeln: Svenska (`sv`)

Schwedisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **du** (O-1), klein geschrieben: «Välj din profil.»
- Kurz und freundlich wie Netflix Sverige. Keine Ausrufezeichen in Meldungen.
- Gleiche Wörter wie Englisch («Film», «Trailer», «Podcast» im Zusammenhang) sind richtig; `check.ts`
  meldet sie nur als Warnung.

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Imperativ:** «Spara», «Avbryt», «Lägg till mapp», «Hoppa över intro».
- **Satzschreibung:** nur das erste Wort gross («Lägg till bibliotek»).
- Zusammensetzungen zusammenschreiben («bakgrundsjobb», «avsnittsbild»); Bindestrich nur bei
  Abkürzungen und Namen («TMDB-nyckel»).

## Anführungs- und Satzzeichen

- Anführungszeichen **” … ”** (beide U+201D), darin ’ … ’.
- Auslassung **…** direkt am Wort: «Läser in…».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`).
- Prozent mit Leerzeichen: «{{percent}} %». Einheiten: «{{hours}} tim {{minutes}} min», «{{seconds}} s».
- Datum: «den {{date}}», «kl. {{time}}».

## Pluralformen: `one`, `other`

```json
"episodes_one": "{{count}} avsnitt",
"episodes_other": "{{count}} avsnitt"
```

- «avsnitt», «program» und viele Neutra sind im Plural gleich. Beide Schlüssel trotzdem anlegen.

## Platzhalter und Grammatik

- Bestimmte Form hängt am Nomen («biblioteket»); vor Platzhaltern Nomen einschieben: «biblioteket
  {{name}}».
- Genitiv-s nicht an Platzhalter hängen («{{name}}s»): «profilen {{name}}» oder Doppelpunkt.

## Länge

Etwa so lang wie Englisch; lange Zusammensetzungen auf Knöpfen vermeiden.

## Beispiele

| Schlüssel | de | en | sv |
|---|---|---|---|
| `common.save` | Speichern | Save | Spara |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Välj din profil. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Din lista är tom |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Säkerhetskopian ”{{name}}” har skapats. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Volym {{percent}} % |
| `common.loading` | Wird geladen … | Loading … | Läser in… |
| `player.skipIntro` | Intro überspringen | Skip intro | Hoppa över intro |
