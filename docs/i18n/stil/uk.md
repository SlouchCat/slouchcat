# Stilregeln: Українська (`uk`)

Ukrainisch. Gilt mit dem Glossar und `README.md` in diesem Ordner. Nicht aus `ru` ableiten: Wortschatz
und Wendungen unterscheiden sich (keine Russismen wie «настройки», «скачати» statt «налаштування»,
«завантажити»).

## Anrede und Ton

- **ви** (O-1), klein geschrieben: «Виберіть профіль.»
- Sachlich wie Netflix Україна. Keine Ausrufezeichen in Meldungen.
- Vergangenheit mit Geschlecht bei Personen vermeiden: unpersönlich («Збережено.», «Зміни збережено.»).

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Infinitiv:** «Зберегти», «Скасувати», «Додати папку», «Пропустити заставку».
- **Satzschreibung:** nur das erste Wort gross («Додати бібліотеку»).

## Anführungs- und Satzzeichen

- Anführungszeichen **« … »**, darin „ … “.
- Apostroph in Wörtern **’** (U+2019): «прев’ю», «м’який».
- Auslassung **…** direkt am Wort: «Завантаження…».
- Gedankenstrich mit Leerzeichen « — » im Satz.

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}} год {{minutes}} хв», «{{seconds}} с».
- Datum: «{{date}}», «о {{time}}».

## Pluralformen: `one`, `few`, `many`, `other`

| Kategorie | Zahlen | Beispiel |
|---|---|---|
| `one` | 1, 21, 31, 101 … | 21 фільм |
| `few` | 2–4, 22–24 … | 2 фільми |
| `many` | 0, 5–20, 25–30, 100 … | 5 фільмів |
| `other` | Bruchzahlen | 1,5 фільму |

```json
"episodes_one": "{{count}} серія",
"episodes_few": "{{count}} серії",
"episodes_many": "{{count}} серій",
"episodes_other": "{{count}} серії"
```

- `one` umfasst 21, 31 …: nie «одна» oder «1» schreiben, immer `{{count}}`.

## Platzhalter und Grammatik

- Namen in Platzhaltern lassen sich nicht beugen: mit Nomen davor oder Doppelpunkt umformulieren:
  «Видалити профіль «{{name}}»?», «Бібліотека: {{name}}».
- Präpositionen «у/в», «і/й» wechseln nach Lautumgebung; vor Platzhaltern die Form wählen, die vor
  Konsonant und Vokal trägt («у»).

## Länge

Rund 15–30 % länger als Englisch. Auf Knöpfen kurze Infinitive.

## Beispiele

| Schlüssel | de | en | uk |
|---|---|---|---|
| `common.save` | Speichern | Save | Зберегти |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Виберіть профіль. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Ваш список порожній |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Резервну копію «{{name}}» створено. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Гучність {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | Завантаження… |
| `player.skipIntro` | Intro überspringen | Skip intro | Пропустити заставку |
