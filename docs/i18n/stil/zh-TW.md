# Stilregeln: 繁體中文 (`zh-TW`)

Chinesisch, traditionelle Schrift, Wortschatz Taiwans. Gilt mit dem Glossar und `README.md` in diesem
Ordner. Browser mit `zh-Hant`, `zh-HK` und `zh-MO` landen ebenfalls hier (Konzept 22, 4.2). Nicht
automatisch aus `zh-CN` umwandeln.

## Anrede und Ton

- **你** (O-1), nicht 您: «選擇你的個人資料。» Wo es geht, ganz ohne Pronomen: «請選擇個人資料。»
- Ton wie Netflix Taiwan: knapp und klar, ohne Ausrufezeichen.
- Wortschatz Taiwans: 影片, 檔案, 資料夾, 設定, 網路, 伺服器, 預設, 螢幕, 帳戶, 影集, 品質 (nicht 质量).

## Knöpfe, Überschriften, Menüs

- **Knöpfe kurz**, meist zwei bis vier Zeichen, ohne Satzzeichen: «儲存», «取消», «新增資料夾»,
  «跳過片頭».
- Überschriften ohne 。.

## Anführungs- und Satzzeichen

- Anführungszeichen **「 … 」**, darin 『 … 』.
- Satzzeichen in voller Breite: ，。：；！？（）. Nach ihnen kein Leerzeichen.
- Auslassung im Ladezustand **…** wie in Apps üblich: «載入中…».
- **Leerzeichen zwischen Chinesisch und Latein, Ziffern oder Platzhaltern:** «共 {{count}} 部作品»,
  «TMDB 金鑰». Zwischen chinesischen Zeichen keine Leerzeichen.

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1,234,567.5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}} 小時 {{minutes}} 分鐘», «{{seconds}}
  秒».
- Zählwörter: 部 (Filme, Serien), 集 (Folgen), 季 (Staffeln), 個 (Allgemeines), 位 (Personen).

## Pluralformen: nur `other`

- Chinesisch hat keine Pluralformen. Jeder pluralisierte Schlüssel bekommt **nur** `_other`, dazu `_zero`,
  wo `en.json` ihn hat.

```json
"episodes_other": "{{count}} 集"
```

## Platzhalter und Grammatik

- Satzbau frei anpassen; Platzhalter dürfen vorne stehen: «已建立備份「{{name}}」。».

## Länge

Chinesisch ist deutlich kürzer als Englisch, braucht aber doppelte Zeichenbreite. `check.ts` zählt
Zeichen in voller Breite doppelt.

## Beispiele

| Schlüssel | de | en | zh-TW |
|---|---|---|---|
| `common.save` | Speichern | Save | 儲存 |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | 選擇你的個人資料。 |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | 你的片單是空的 |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | 已建立備份「{{name}}」。 |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | 音量 {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | 載入中… |
| `player.skipIntro` | Intro überspringen | Skip intro | 跳過片頭 |
