# Stilregeln: Português do Brasil (`pt-BR`)

Brasilianisches Portugiesisch. Gilt mit dem Glossar und `README.md` in diesem Ordner. Die Variante für
Portugal steht in `pt-PT.md`; die beiden werden getrennt übersetzt, nicht voneinander abgeleitet.

## Anrede und Ton

- **você** (O-1), meist ohne das Pronomen: «Escolha seu perfil.»
- Freundlich und direkt wie Netflix Brasil. Keine Ausrufezeichen in Meldungen.
- Wortschatz Brasiliens: «tela», «arquivo», «baixar», «usuário», «celular», «Configurações», «Salvar»,
  «time» nie für «Team».
- Gerundium ist üblich: «Carregando…», «Salvando…».

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Infinitiv:** «Salvar», «Adicionar pasta», «Pular abertura». Wiedergabe-Knopf wie Netflix
  «Assistir», bei Ton «Ouvir».
- **Satzschreibung:** nur das erste Wort gross («Adicionar biblioteca»).

## Anführungs- und Satzzeichen

- Anführungszeichen **“ … ”**, darin ‘ … ’.
- Auslassung **…** direkt am Wort.

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1.234.567,5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}} h {{minutes}} min», «{{seconds}} s».
- Datum: «em {{date}}», «às {{time}}».

## Pluralformen: `one`, `many`, `other`

- `one` gilt für **0 und 1** («0 título», «1 título»). Deshalb immer `{{count}}`, nie «um».
- `many` gilt für sehr grosse runde Zahlen und verlangt «de»: «{{count}} de títulos».
- `other` für alles Übrige.

```json
"episodes_one": "{{count}} episódio",
"episodes_many": "{{count}} de episódios",
"episodes_other": "{{count}} episódios"
```

## Platzhalter und Grammatik

- Kontraktionen («do», «na») vor Platzhaltern vermeiden; ein Nomen einschieben: «na biblioteca
  {{name}}».
- Partizipien passen zum Nomen: «Backup criado», «Biblioteca excluída».

## Länge

Rund 20–30 % länger als Englisch. Kurze Verben auf Knöpfen («Pular», «Remover»).

## Beispiele

| Schlüssel | de | en | pt-BR |
|---|---|---|---|
| `common.save` | Speichern | Save | Salvar |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Escolha seu perfil. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Sua lista está vazia |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Backup “{{name}}” criado. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Volume {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | Carregando… |
| `player.skipIntro` | Intro überspringen | Skip intro | Pular abertura |
