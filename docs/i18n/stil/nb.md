# Stilregeln: Norsk bokmål (`nb`)

Norwegisch (Bokmål, nicht Nynorsk). Gilt mit dem Glossar und `README.md` in diesem Ordner. Browser mit
`no` oder `nn` landen ebenfalls hier (Konzept 22, 4.2).

## Anrede und Ton

- **du** (O-1), klein geschrieben: «Velg profilen din.»
- Kurz und freundlich wie Netflix Norge. Keine Ausrufezeichen in Meldungen.
- Besitzpronomen nach dem Nomen, wie im gesprochenen Bokmål üblich: «listen din», «profilen din».

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Imperativ:** «Lagre», «Avbryt», «Legg til mappe», «Hopp over intro».
- **Satzschreibung:** nur das erste Wort gross («Legg til bibliotek»).
- Zusammensetzungen zusammenschreiben («bakgrunnsjobb», «episodebilde»); Bindestrich nur bei Abkürzungen
  und Namen («TMDB-nøkkel»).

## Anführungs- und Satzzeichen

- Anführungszeichen **« … »**, darin ‘ … ’.
- Auslassung **…** direkt am Wort: «Laster inn…».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`).
- Prozent mit Leerzeichen: «{{percent}} %». Einheiten: «{{hours}} t {{minutes}} min», «{{seconds}} s».
- Datum: «{{date}}», «kl. {{time}}».

## Pluralformen: `one`, `other`

```json
"episodes_one": "{{count}} episode",
"episodes_other": "{{count}} episoder"
```

## Platzhalter und Grammatik

- Bestimmte Form hängt am Nomen («biblioteket»); vor Platzhaltern Nomen einschieben: «biblioteket
  {{name}}».
- Genitiv-s nicht an Platzhalter hängen.

## Länge

Etwa so lang wie Englisch; lange Zusammensetzungen auf Knöpfen vermeiden.

## Beispiele

| Schlüssel | de | en | nb |
|---|---|---|---|
| `common.save` | Speichern | Save | Lagre |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Velg profilen din. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Listen din er tom |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Sikkerhetskopien «{{name}}» er opprettet. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Volum {{percent}} % |
| `common.loading` | Wird geladen … | Loading … | Laster inn… |
| `player.skipIntro` | Intro überspringen | Skip intro | Hopp over intro |
