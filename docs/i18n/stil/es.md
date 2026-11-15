# Stilregeln: Español (`es`)

Spanisch aus Spanien (nicht Lateinamerika). Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **tú** (O-1): «Elige tu perfil.»
- Locker, aber nicht flapsig, wie Netflix España. Keine Ausrufezeichen in Meldungen.
- Wortschatz aus Spanien: «Ajustes», «ordenador», «móvil», «vídeo» (mit Akzent), «Añadir» statt
  «Agregar», «Guardar» statt «Salvar».
- Geschlechtsneutral durch Umformulieren («Cada perfil …», «quien tenga el enlace»), nicht mit «@» oder
  «-e».

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Infinitiv:** «Guardar», «Añadir carpeta», «Saltar intro».
- **Satzschreibung:** nur das erste Wort gross («Añadir biblioteca»).

## Anführungs- und Satzzeichen

- Anführungszeichen **« … »**, darin “ … ” (RAE). Kein Leerzeichen innen.
- Frage und Ausruf immer mit **¿ … ?** und **¡ … !**: `¿Eliminar el perfil «{{name}}»?`.
- Auslassung **…** direkt am Wort: «Cargando…».
- Doppelpunkt ohne Leerzeichen davor.

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1.234.567,5`); vierstellige Zahlen ohne Trennzeichen.
- Prozent mit Leerzeichen: «{{percent}} %». Einheiten: «{{hours}} h {{minutes}} min», «{{seconds}} s».
- Datum: «el {{date}}», «a las {{time}}».

## Pluralformen: `one`, `many`, `other`

- `one` nur für 1, `other` für 0 und alle übrigen Zahlen.
- `many` gilt für sehr grosse runde Zahlen (1 000 000) und verlangt «de»: «{{count}} de títulos».

```json
"episodes_one": "{{count}} episodio",
"episodes_many": "{{count}} de episodios",
"episodes_other": "{{count}} episodios"
```

## Platzhalter und Grammatik

- Artikel vor Platzhaltern vermeiden, deren Geschlecht unbekannt ist; ein Nomen einschieben: «la
  biblioteca {{name}}», «el perfil {{name}}».
- Partizipien passen zum Nomen im Satz: «Copia de seguridad creada», «Perfil eliminado».

## Länge

Spanisch ist rund 20–30 % länger als Englisch. Bei Knöpfen kurze Verben: «Omitir», «Quitar».

## Beispiele

| Schlüssel | de | en | es |
|---|---|---|---|
| `common.save` | Speichern | Save | Guardar |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Elige tu perfil. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Tu lista está vacía |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Copia de seguridad «{{name}}» creada. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Volumen {{percent}} % |
| `common.loading` | Wird geladen … | Loading … | Cargando… |
| `player.skipIntro` | Intro überspringen | Skip intro | Saltar intro |
