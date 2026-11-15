# Stilregeln: Türkçe (`tr`)

Türkisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **siz** (O-1) in Sätzen und Hinweisen: «Profilinizi seçin.», «Lütfen tekrar deneyin.»
- **Knöpfe** trotzdem in der kurzen Grundform, wie in türkischen Apps üblich: «Kaydet», «İptal», «Sil»,
  «Klasör ekle».
- Sachlich wie Netflix Türkiye. Keine Ausrufezeichen in Meldungen.

## Knöpfe, Überschriften, Menüs

- **Satzschreibung:** nur das erste Wort gross («Kitaplık ekle»).
- **I und İ:** Grossbuchstaben von «i» ist «İ», von «ı» ist «I». In Texten nie von Hand in Versalien
  schreiben; die Oberfläche setzt `lang="tr"`, damit CSS richtig umwandelt.

## Anführungs- und Satzzeichen

- Anführungszeichen **“ … ”**, darin ‘ … ’.
- Auslassung **…** direkt am Wort: «Yükleniyor…».
- Endungen an Eigennamen und Zahlen mit Apostroph: «Slouchcat’e», «TMDB’den» (U+2019).

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1.234.567,5`).
- **Prozentzeichen vor der Zahl:** «%{{percent}}». Einheiten: «{{hours}} sa {{minutes}} dk», «{{seconds}}
  sn».
- Datum: «{{date}}», «{{time}}».

## Pluralformen: `one`, `other`

- `one` nur für 1, `other` für alle übrigen Zahlen.
- **Nach Zahlwörtern steht das Nomen im Singular:** «5 film», nicht «5 filmler». `_one` und `_other` sind
  darum oft gleich. Beide Schlüssel trotzdem anlegen.

```json
"episodes_one": "{{count}} bölüm",
"episodes_other": "{{count}} bölüm"
```

## Platzhalter und Grammatik

- **Endungen nach Vokalharmonie** («-de/-da», «-i/-ı/-u/-ü») passen nicht an Platzhalter, weil der
  letzte Vokal unbekannt ist («{{name}}’de» geht nicht). Die Endung an ein Nomen hängen: «{{name}}
  kitaplığında», «{{name}} adlı profil» oder «Kitaplık: {{name}}».
- Verb am Satzende; Platzhalter dürfen dafür vorne stehen.

## Länge

Rund 10–30 % länger als Englisch durch Endungen. Auf Knöpfen die Grundform.

## Beispiele

| Schlüssel | de | en | tr |
|---|---|---|---|
| `common.save` | Speichern | Save | Kaydet |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | Profilinizi seçin. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | Listeniz boş |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | “{{name}}” yedeği oluşturuldu. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | Ses düzeyi %{{percent}} |
| `common.loading` | Wird geladen … | Loading … | Yükleniyor… |
| `player.skipIntro` | Intro überspringen | Skip intro | Girişi atla |
