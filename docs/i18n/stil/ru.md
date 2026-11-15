# Stilregeln: Русский (`ru`)

Russisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **вы** (O-1), klein geschrieben: «Выберите профиль.»
- Sachlich wie Кинопоиск oder Netflix in russischer Fassung. Keine Ausrufezeichen in Meldungen.
- Vergangenheit mit Geschlecht bei Personen vermeiden («вы сохранили/сохранила»): unpersönlich
  («Сохранено.», «Изменения сохранены.»).
- **е statt ё**, ausser wo es ohne ё missverständlich wäre («всё» vs. «все»).

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Infinitiv:** «Сохранить», «Отмена», «Добавить папку», «Пропустить заставку».
- **Satzschreibung:** nur das erste Wort gross («Добавить библиотеку»).

## Anführungs- und Satzzeichen

- Anführungszeichen **« … »**, darin „ … “.
- Auslassung **…** direkt am Wort: «Загрузка…».
- Gedankenstrich mit Leerzeichen « — » (U+2014) im Satz; Bindestrich in Wörtern («интернет-радио»).

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`).
- Prozent mit Leerzeichen: «{{percent}} %». Einheiten: «{{hours}} ч {{minutes}} мин», «{{seconds}} с».
- Datum: «{{date}}», «в {{time}}».

## Pluralformen: `one`, `few`, `many`, `other`

| Kategorie | Zahlen | Beispiel |
|---|---|---|
| `one` | 1, 21, 31, 101 … | 21 фильм |
| `few` | 2–4, 22–24, 32–34 … | 2 фильма |
| `many` | 0, 5–20, 25–30, 100 … | 5 фильмов |
| `other` | Bruchzahlen | 1,5 фильма |

```json
"episodes_one": "{{count}} серия",
"episodes_few": "{{count}} серии",
"episodes_many": "{{count}} серий",
"episodes_other": "{{count}} серии"
```

- `one` umfasst 21, 31 …: nie «одна», «1» oder «единственный» schreiben, immer `{{count}}`. Ein Text, der
  im Englischen nur für genau 1 gedacht ist («The only episode …»), wird so umformuliert, dass er für 21
  auch stimmt.

## Platzhalter und Grammatik

- Namen in Platzhaltern lassen sich nicht beugen: mit Nomen davor oder Doppelpunkt umformulieren:
  «Удалить профиль «{{name}}»?», «Библиотека: {{name}}».
- Verben in der Vergangenheit passen zum Nomen im Satz, nicht zum Platzhalter: «Профиль {{name}}
  удален».

## Länge

Russisch ist rund 15–30 % länger als Englisch, mit langen Wörtern. Auf Knöpfen kurze Infinitive
(«Скрыть», «Убрать»).

## Beispiele

| Schlüssel | de | en | ru |
|---|---|---|---|
| `common.save` | Speichern | Save | Сохранить |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Выберите профиль. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Ваш список пуст |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Резервная копия «{{name}}» создана. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Громкость {{percent}} % |
| `common.loading` | Wird geladen … | Loading … | Загрузка… |
| `player.skipIntro` | Intro überspringen | Skip intro | Пропустить заставку |
