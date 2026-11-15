# Stilregeln: Suomi (`fi`)

Finnisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **Ohne Pronomen** (O-1): Knöpfe und Aufforderungen im Imperativ («Valitse profiili.»), Meldungen im
  Passiv («Tallennettu.», «Kirjastoa ei löytynyt.»). Kein «sinä», kein «te».
- Sachlich wie Netflix Suomi. Keine Ausrufezeichen in Meldungen.

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Imperativ:** «Tallenna», «Peruuta», «Lisää kansio», «Ohita alkutekstit».
- **Satzschreibung:** nur das erste Wort gross («Lisää kirjasto»).
- Zusammensetzungen zusammenschreiben («taustatyö», «ääniraita»); mit Bindestrich bei Abkürzungen und
  Namen («TMDB-avain»).

## Anführungs- und Satzzeichen

- Anführungszeichen **” … ”** (beide U+201D), darin ’ … ’.
- Auslassung **…** direkt am Wort: «Ladataan…».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`).
- Prozent mit Leerzeichen: «{{percent}} %». Einheiten: «{{hours}} h {{minutes}} min», «{{seconds}} s».
- Datum: «{{date}}», «klo {{time}}».

## Pluralformen: `one`, `other`

- Nach Zahlwörtern ab 2 steht der **Partitiv Singular**: «1 elokuva», «5 elokuvaa».

```json
"episodes_one": "{{count}} jakso",
"episodes_other": "{{count}} jaksoa"
```

## Platzhalter und Grammatik

- Finnisch beugt über Endungen (15 Fälle). An Platzhalter lässt sich keine Endung hängen
  («{{name}}:iin» geht nicht). Die Endung an ein Nomen davor hängen oder mit Doppelpunkt
  umformulieren: «kirjastoon {{name}}», «Kirjasto: {{name}}».
- Für Zahlen mit Endung («{{count}}:ssä») ebenfalls umformulieren.

## Länge

Finnisch ist oft 30–40 % länger als Englisch. Auf Knöpfen die kürzeste Form; Zusammensetzungen auf
Reitern kürzen, ohne Abkürzungspunkt.

## Beispiele

| Schlüssel | de | en | fi |
|---|---|---|---|
| `common.save` | Speichern | Save | Tallenna |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Valitse profiili. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Lista on tyhjä |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Varmuuskopio ”{{name}}” luotu. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Äänenvoimakkuus {{percent}} % |
| `common.loading` | Wird geladen … | Loading … | Ladataan… |
| `player.skipIntro` | Intro überspringen | Skip intro | Ohita alkutekstit |
