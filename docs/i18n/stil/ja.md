# Stilregeln: 日本語 (`ja`)

Japanisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **です／ます** (O-1) in Sätzen, Meldungen und Hinweisen: «プロフィールを選択してください。»
- Kein Pronomen für den Leser («あなた»), keine Keigo-Übertreibung («〜させていただきます»).
- Ton wie Netflix Japan: höflich, knapp, ohne Ausrufezeichen.

## Knöpfe, Überschriften, Menüs

- **Knöpfe als Nomen oder Verbstamm** ohne です／ます und ohne 。: «保存», «キャンセル», «追加»,
  «フォルダーを追加», «イントロをスキップ».
- Überschriften ohne 。.
- Katakana mit langem Endvokal nach heutiger Schreibung: «フォルダー», «ユーザー», «サーバー»,
  «プレーヤー».

## Anführungs- und Satzzeichen

- Anführungszeichen **「 … 」**, darin 『 … 』.
- Satzzeichen in voller Breite: 。、：（）. Nach 。 und 、 kein Leerzeichen.
- Auslassung **…** direkt am Wort: «読み込み中…».
- Keine Leerzeichen zwischen japanischen Wörtern. Zwischen Japanisch und Platzhaltern, Zahlen oder
  lateinischen Namen ebenfalls keins: «{{count}}件», «TMDBのキー».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1,234,567.5`), Ziffern halbbreit.
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}}時間{{minutes}}分», «{{seconds}}秒».
- Zählwörter: 件 (Einträge), 本 (Filme), 話 (Folgen), シーズン, 個 (Allgemeines), 人 (Personen).

## Pluralformen: nur `other`

- Japanisch hat keine Pluralformen. Jeder pluralisierte Schlüssel bekommt **nur** `_other`, dazu `_zero`,
  wo `en.json` ihn hat.

```json
"episodes_other": "{{count}}話"
```

## Platzhalter und Grammatik

- Wortstellung: Verb am Ende, Platzhalter mit Partikel davor: «{{name}}を削除しますか？».
- Partikel (を、に、が) hängen problemlos an Platzhalter.

## Länge

Japanisch ist meist kürzer als Englisch, braucht aber doppelte Zeichenbreite. `check.ts` zählt Zeichen
in voller Breite doppelt. Knöpfe höchstens rund 1,5× der englischen Breite.

## Beispiele

| Schlüssel | de | en | ja |
|---|---|---|---|
| `common.save` | Speichern | Save | 保存 |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | プロフィールを選択してください。 |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | マイリストは空です |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | バックアップ「{{name}}」を作成しました。 |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | 音量 {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | 読み込み中… |
| `player.skipIntro` | Intro überspringen | Skip intro | イントロをスキップ |

Beim Lautstärke-Beispiel steht das Leerzeichen zwischen Wort und Wert wie in einer Anzeige; im Satz
entfällt es.
