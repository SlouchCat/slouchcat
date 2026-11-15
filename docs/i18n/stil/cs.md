# Stilregeln: Čeština (`cs`)

Tschechisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **vy** (vykání, O-1), Verb in der 2. Person Plural: «Vyberte profil.» Pronomen klein («vy», «váš»),
  nicht «Vy» wie im Brief.
- Sachlich wie Netflix Česko. Keine Ausrufezeichen in Meldungen.
- Vergangenheitsformen mit Geschlecht («uložil/uložila jste») vermeiden: «Uloženo.», «Změny byly
  uloženy.»

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Infinitiv:** «Uložit», «Zrušit», «Přidat složku», «Přeskočit úvod».
- **Satzschreibung:** nur das erste Wort gross («Přidat knihovnu»).

## Anführungs- und Satzzeichen

- Anführungszeichen **„ … “** (U+201E, U+201C), darin ‚ … ‘.
- Auslassung **…** direkt am Wort: «Načítání…».
- Einbuchstabige Präpositionen (v, k, s, z, o, u) nicht am Zeilenende: im Text mit U+00A0 an das
  folgende Wort binden, wenn danach Text folgt («v knihovně»).

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`).
- Prozent mit Leerzeichen: «{{percent}} %». Einheiten: «{{hours}} h {{minutes}} min», «{{seconds}} s».
- Datum: «{{date}}», «v {{time}}».

## Pluralformen: `one`, `few`, `many`, `other`

Achtung: Im Tschechischen ist `many` die Form für **Bruchzahlen** und `other` die für 0 und ab 5.

| Kategorie | Zahlen | Beispiel |
|---|---|---|
| `one` | 1 | 1 titul |
| `few` | 2–4 | 2 tituly |
| `many` | Bruchzahlen | 1,5 titulu |
| `other` | 0, 5 und mehr | 5 titulů |

```json
"episodes_one": "{{count}} díl",
"episodes_few": "{{count}} díly",
"episodes_many": "{{count}} dílu",
"episodes_other": "{{count}} dílů"
```

## Platzhalter und Grammatik

- Namen in Platzhaltern lassen sich nicht beugen: mit Nomen davor oder Doppelpunkt umformulieren:
  «Smazat profil {{name}}?», «Knihovna: {{name}}».
- Partizipien passen zum Nomen: «Záloha vytvořena», «Knihovna smazána».

## Länge

Rund 10–20 % länger als Englisch. Auf Knöpfen kurze Infinitive.

## Beispiele

| Schlüssel | de | en | cs |
|---|---|---|---|
| `common.save` | Speichern | Save | Uložit |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Vyberte svůj profil. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Váš seznam je prázdný |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Záloha „{{name}}“ vytvořena. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Hlasitost {{percent}} % |
| `common.loading` | Wird geladen … | Loading … | Načítání… |
| `player.skipIntro` | Intro überspringen | Skip intro | Přeskočit úvod |
