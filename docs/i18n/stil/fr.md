# Stilregeln: Français (`fr`)

Französisch aus Frankreich. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **vous** (O-1), auch für Hinweise an den Administrator: «Choisissez votre profil.»
- Sachlich und freundlich, wie Netflix oder Disney+ in Frankreich. Keine Ausrufezeichen in Meldungen.
- Geschlechtsneutral formulieren statt «utilisateur·rice»: «Toute personne disposant du lien …», «le
  profil» statt «l’utilisateur».

## Knöpfe, Überschriften, Menüs

- **Knöpfe im Infinitiv:** «Enregistrer», «Ajouter un dossier», «Passer l’intro».
- **Satzschreibung** überall: nur das erste Wort und Eigennamen gross («Ajouter une bibliothèque», nicht
  «Ajouter Une Bibliothèque»).
- Menüpunkte und Reiter als Nomen: «Paramètres», «Téléchargements».

## Anführungs- und Satzzeichen

- Anführungszeichen **« … »** mit schmalem geschütztem Leerzeichen (U+202F) innen: `« Ma liste »`.
  Innerhalb davon “ … ”.
- Vor **; ! ?** steht U+202F, vor **:** ein geschütztes Leerzeichen (U+00A0): `Chapitre : {{chapter}}`.
- Auslassung **…** direkt am Wort: «Chargement…».
- Apostroph typografisch **’** (U+2019): «l’épisode», «d’un titre».
- Gedankenstrich mit Leerzeichen « – ».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`). Steht eine Zahl im Text, Tausender mit U+202F.
- Prozent mit U+202F: `{{percent}} %`. Einheiten: «{{hours}} h {{minutes}} min», «{{seconds}} s», «Go»
  und «Mo» statt GB/MB, wo der Text die Einheit selbst nennt.
- Datum und Uhrzeit nur über die Platzhalter; «à» vor Uhrzeiten: «le {{date}} à {{time}}».

## Pluralformen: `one`, `many`, `other`

- `one` gilt für **0 und 1** («0 titre», «1 titre»). Deshalb steht immer `{{count}}`, nie «un».
- `many` gilt für sehr grosse runde Zahlen (1 000 000) und verlangt «de»: «{{count}} de titres».
- `other` für alles Übrige.

```json
"episodes_one": "{{count}} épisode",
"episodes_many": "{{count}} d’épisodes",
"episodes_other": "{{count}} épisodes"
```

## Platzhalter und Grammatik

- Elision vor Platzhaltern geht nicht: nicht «de {{name}}» vor einem Vokal riskieren, sondern ein Nomen
  einschieben: «la bibliothèque {{name}}», «le profil {{name}}».
- Partizipien richten sich nach dem Bezugswort; bei unbekanntem Geschlecht umformulieren: «Profil
  {{name}} supprimé» (Profil ist männlich) statt «{{name}} a été supprimé(e)».

## Länge

Französisch ist rund 20–30 % länger als Englisch. Bei Knöpfen das kürzere Verb wählen («Passer» statt
«Ignorer cette étape»).

## Beispiele

| Schlüssel | de | en | fr |
|---|---|---|---|
| `common.save` | Speichern | Save | Enregistrer |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Choisissez votre profil. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Votre liste est vide |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Sauvegarde « {{name}} » créée. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Volume {{percent}} % |
| `common.loading` | Wird geladen … | Loading … | Chargement… |
| `player.skipIntro` | Intro überspringen | Skip intro | Passer l’intro |

In den Beispielen stehen U+202F und U+00A0 als echte Zeichen; sie sehen aus wie Leerzeichen. In
der JSON-Datei dürfen sie auch als Zeichen stehen, nicht als `\u202f`.
