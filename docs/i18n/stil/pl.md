# Stilregeln: Polski (`pl`)

Polnisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **Imperativ ohne Pronomen** (O-1): «Wybierz profil.», «Dodaj folder». Kein «Ty», kein «Pan/Pani».
- Meldungen unpersönlich oder im Perfekt ohne Subjekt: «Zapisano.», «Nie znaleziono biblioteki.»
- Sachlich wie Netflix Polska. Keine Ausrufezeichen in Meldungen.
- Geschlechtsformen der 2. Person Vergangenheit («zapisałeś/zapisałaś») vermeiden; unpersönlich
  umformulieren («Zapisano zmiany»).

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Imperativ:** «Zapisz», «Anuluj», «Dodaj folder», «Pomiń czołówkę».
- **Satzschreibung:** nur das erste Wort gross («Dodaj bibliotekę»).
- Menüpunkte und Reiter als Nomen: «Ustawienia», «Pobrane».

## Anführungs- und Satzzeichen

- Anführungszeichen **„ … ”** (U+201E, U+201D), darin « … ».
- Auslassung **…** direkt am Wort: «Ładowanie…».
- Gedankenstrich mit Leerzeichen « – ».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}} godz. {{minutes}} min», «{{seconds}}
  s».
- Datum: «{{date}}», «o {{time}}».

## Pluralformen: `one`, `few`, `many`, `other`

| Kategorie | Zahlen | Beispiel |
|---|---|---|
| `one` | 1 | 1 tytuł |
| `few` | 2–4, 22–24, 32–34 … | 2 tytuły |
| `many` | 0, 5–21, 25–31 … | 5 tytułów |
| `other` | Bruchzahlen | 1,5 tytułu |

```json
"episodes_one": "{{count}} odcinek",
"episodes_few": "{{count}} odcinki",
"episodes_many": "{{count}} odcinków",
"episodes_other": "{{count}} odcinka"
```

- Fehlt eine Kategorie, zeigt i18next den Schlüssel. Alle vier sind Pflicht.

## Platzhalter und Grammatik

- Polnisch beugt Nomen nach Fall. Ein Platzhalter mit einem Namen («{{name}}») kann nicht gebeugt
  werden: mit Nomen davor oder Doppelpunkt umformulieren: «Usunąć profil {{name}}?», «Biblioteka:
  {{name}}».
- Verb und Partizip passen zum Nomen im Satz: «Kopia zapasowa utworzona».

## Länge

Rund 20–30 % länger als Englisch. Lange Komposita gibt es nicht, aber lange Endungen; auf Knöpfen das
kürzeste Verb.

## Beispiele

| Schlüssel | de | en | pl |
|---|---|---|---|
| `common.save` | Speichern | Save | Zapisz |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Wybierz profil. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Twoja lista jest pusta |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Utworzono kopię zapasową „{{name}}”. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Głośność {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | Ładowanie… |
| `player.skipIntro` | Intro überspringen | Skip intro | Pomiń czołówkę |

«Twoja lista» ist die übliche Ausnahme vom pronomenlosen Stil: Besitzpronomen sind erlaubt, wo
Netflix Polska sie verwendet.
