# Stilregeln: Português de Portugal (`pt-PT`)

Europäisches Portugiesisch. Gilt mit dem Glossar und `README.md` in diesem Ordner. Nicht aus `pt-BR`
ableiten: Wortschatz, Satzbau und Anführungszeichen unterscheiden sich.

## Anrede und Ton

- **você** (O-1), aber das Pronomen fast nie ausschreiben: Verb in der 3. Person, Besitz mit «o seu» /
  «a sua»: «Escolha o seu perfil.» Ein ausgeschriebenes «você» klingt in Portugal unhöflich.
- Sachlich wie Netflix Portugal. Keine Ausrufezeichen in Meldungen.
- Wortschatz Portugals: «ecrã», «ficheiro», «transferir», «utilizador», «telemóvel», «Definições»,
  «Guardar», «Eliminar».
- **Kein Gerundium:** «A carregar…», «A guardar…» statt «Carregando…».

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Infinitiv:** «Guardar», «Adicionar pasta», «Saltar genérico».
- **Satzschreibung:** nur das erste Wort gross.

## Anführungs- und Satzzeichen

- Anführungszeichen **« … »**, darin “ … ”.
- Auslassung **…** direkt am Wort.

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}} h {{minutes}} min», «{{seconds}} s».
- Datum: «a {{date}}», «às {{time}}».

## Pluralformen: `one`, `many`, `other`

- `one` nur für 1; `other` für 0 und alle übrigen Zahlen (anders als `pt-BR`).
- `many` gilt für sehr grosse runde Zahlen und verlangt «de»: «{{count}} de títulos».

```json
"episodes_one": "{{count}} episódio",
"episodes_many": "{{count}} de episódios",
"episodes_other": "{{count}} episódios"
```

## Platzhalter und Grammatik

- Kontraktionen vor Platzhaltern vermeiden; ein Nomen einschieben: «na biblioteca {{name}}».
- Partizipien passen zum Nomen: «Cópia de segurança criada».

## Länge

Rund 20–30 % länger als Englisch. «Cópia de segurança» ist lang: in Spaltenköpfen und Reitern genügt
«Cópia», wenn der Zusammenhang klar ist.

## Beispiele

| Schlüssel | de | en | pt-PT |
|---|---|---|---|
| `common.save` | Speichern | Save | Guardar |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Escolha o seu perfil. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | A sua lista está vazia |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Cópia de segurança «{{name}}» criada. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Volume {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | A carregar… |
| `player.skipIntro` | Intro überspringen | Skip intro | Saltar genérico |
