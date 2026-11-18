# Stilregeln: Magyar (`hu`)

Ungarisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **Sie-Form ohne Pronomen** (O-1): Verb in der höflichen 3. Person, aber ohne «Ön»: «Válassza ki a
  profilját.» Das Pronomen nur, wenn der Satz sonst unklar wird.
- Sachlich wie Netflix Magyarország. Keine Ausrufezeichen in Meldungen.

## Knöpfe, Überschriften, Menüs

- **Knöpfe als Nomen auf -ás/-és**, wie in ungarischen Apps üblich: «Mentés», «Mégse», «Törlés»,
  «Hozzáadás», «Bevezető átugrása».
- **Satzschreibung:** nur das erste Wort gross («Könyvtár hozzáadása»).

## Anführungs- und Satzzeichen

- Anführungszeichen **„ … ”** (U+201E, U+201D), darin » … «.
- Auslassung **…** direkt am Wort: «Betöltés…».
- Ordnungszahlen mit Punkt: «3. évad».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1 234 567,5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}} óra {{minutes}} perc», «{{seconds}}
  mp».
- Datum: «{{date}}», «{{time}}-kor» vermeiden (Endung hängt an der Zahl); besser «időpont: {{time}}».

## Pluralformen: `one`, `other`

- `one` nur für 1, `other` für alle übrigen Zahlen.
- **Nach Zahlwörtern steht das Nomen im Singular:** «5 cím», nicht «5 címek». `_one` und `_other` sind
  darum oft gleich. Beide Schlüssel trotzdem anlegen.

```json
"episodes_one": "{{count}} epizód",
"episodes_other": "{{count}} epizód"
```

## Platzhalter und Grammatik

- **Artikel a / az** hängt vom Anlaut des Folgeworts ab. Vor Platzhaltern keinen Artikel setzen oder
  ein Nomen einschieben: «a(z) {{name}}» nur als letzter Ausweg; besser «a könyvtár: {{name}}».
- **Endungen** (Vokalharmonie: -ban/-ben, -ba/-be) lassen sich nicht an Platzhalter hängen
  («{{name}}-ban» geht nicht). Die Endung an ein Nomen hängen: «a(z) {{name}} könyvtárban» oder
  «Könyvtár: {{name}}».
- Gleiches gilt für Zahlen mit Endung («{{count}}-szor»): umformulieren.
- **Namen mit Doppelkonsonant am Ende** («Seerr», «Radarr», «Sonarr») nehmen eine Endung mit Bindestrich:
  «Seerr-rel», «Seerr-re», nie «Seerrel» oder «Seerrre».

## Länge

Rund 10–30 % länger als Englisch, Komposita sind lang («Háttérfeladat»). Auf Knöpfen kurze Nomen.

## Beispiele

| Schlüssel | de | en | hu |
|---|---|---|---|
| `common.save` | Speichern | Save | Mentés |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Válassza ki a profilját. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | A lista üres |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | Biztonsági mentés elkészült: „{{name}}”. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Hangerő: {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | Betöltés… |
| `player.skipIntro` | Intro überspringen | Skip intro | Bevezető átugrása |
