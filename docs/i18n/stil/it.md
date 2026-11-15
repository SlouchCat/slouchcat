# Stilregeln: Italiano (`it`)

Italienisch aus Italien. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **tu** (O-1): «Scegli il tuo profilo.»
- Freundlich und knapp, wie Netflix Italia. Keine Ausrufezeichen in Meldungen.
- Geschlechtsneutral durch Umformulieren («Chi ha il link …», «ogni profilo»), nicht mit «*» oder «ə».
- Englische Lehnwörter nur, wo sie im Italienischen üblich sind («Home», «trailer», «download»,
  «streaming»); sie bleiben im Plural unverändert («i trailer», «due download»).

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Imperativ (tu):** «Salva», «Aggiungi cartella», «Salta intro», «Annulla».
- **Satzschreibung:** nur das erste Wort gross («Aggiungi libreria»).

## Anführungs- und Satzzeichen

- Anführungszeichen **« … »**, darin “ … ”. Kein Leerzeichen innen.
- Apostroph typografisch **’** (U+2019): «dall’inizio», «l’episodio».
- Auslassung **…** direkt am Wort: «Caricamento…».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1.234.567,5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}} h {{minutes}} min», «{{seconds}} s».
- Datum: «il {{date}}», «alle {{time}}».

## Pluralformen: `one`, `many`, `other`

- `one` nur für 1, `other` für 0 und alle übrigen Zahlen.
- `many` gilt für sehr grosse runde Zahlen (1 000 000) und verlangt «di»: «{{count}} di titoli».

```json
"episodes_one": "{{count}} episodio",
"episodes_many": "{{count}} di episodi",
"episodes_other": "{{count}} episodi"
```

## Platzhalter und Grammatik

- Präposition mit Artikel («della», «nell’») vor Platzhaltern vermeiden; ein Nomen einschieben: «nella
  libreria {{name}}», «del profilo {{name}}».
- Partizipien passen zum Nomen: «Backup creato», «Libreria eliminata».

## Länge

Italienisch ist rund 15–25 % länger als Englisch. Bei Knöpfen kurze Imperative.

## Beispiele

| Schlüssel | de | en | it |
|---|---|---|---|
| `common.save` | Speichern | Save | Salva |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Scegli il tuo profilo. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | La tua lista è vuota |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Backup «{{name}}» creato. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Volume {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | Caricamento… |
| `player.skipIntro` | Intro überspringen | Skip intro | Salta intro |
