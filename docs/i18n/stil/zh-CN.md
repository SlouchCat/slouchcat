# Stilregeln: 简体中文 (`zh-CN`)

Chinesisch, vereinfachte Schrift, Wortschatz Festlandchinas. Gilt mit dem Glossar und `README.md` in
diesem Ordner. Nicht automatisch aus `zh-TW` umwandeln: Wortschatz unterscheidet sich (文件夹/資料夾,
视频/影片, 设置/設定).

## Anrede und Ton

- **你** (O-1), nicht 您: «选择你的个人资料。» Wo es geht, ganz ohne Pronomen: «请选择个人资料。»
- Ton wie die chinesischen Fassungen grosser Dienste: knapp und klar, ohne Ausrufezeichen.

## Knöpfe, Überschriften, Menüs

- **Knöpfe kurz**, meist zwei bis vier Zeichen, ohne Satzzeichen: «保存», «取消», «添加文件夹»,
  «跳过片头».
- Überschriften ohne 。.

## Anführungs- und Satzzeichen

- Anführungszeichen **“ … ”**, darin ‘ … ’.
- Satzzeichen in voller Breite: ，。：；！？（）. Nach ihnen kein Leerzeichen.
- Auslassung im Ladezustand **…** wie in Apps üblich: «正在加载…».
- **Leerzeichen zwischen Chinesisch und Latein, Ziffern oder Platzhaltern:** «共 {{count}} 部作品»,
  «TMDB 密钥». Zwischen chinesischen Zeichen keine Leerzeichen. Füllt die App einen Platzhalter mit
  chinesischen Wörtern (ein Zustand wie 已开启, eine Wendung wie 由管理员), steht davor und danach kein
  Leerzeichen: «心愿{{enabled}}», «{{date}} {{by}}关联».

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1,234,567.5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}} 小时 {{minutes}} 分钟», «{{seconds}}
  秒».
- Zählwörter: 部 (Filme, Serien), 集 (Folgen), 季 (Staffeln), 个 (Allgemeines), 位 (Personen).

## Pluralformen: nur `other`

- Chinesisch hat keine Pluralformen. Jeder pluralisierte Schlüssel bekommt **nur** `_other`, dazu `_zero`,
  wo `en.json` ihn hat.

```json
"episodes_other": "{{count}} 集"
```

## Platzhalter und Grammatik

- Satzbau frei anpassen; Platzhalter dürfen vorne stehen: «已创建备份“{{name}}”。».
- Kein 们 für Plural, wo die Zahl dabeisteht.

## Länge

Chinesisch ist deutlich kürzer als Englisch, braucht aber doppelte Zeichenbreite. `check.ts` zählt
Zeichen in voller Breite doppelt.

## Beispiele

| Schlüssel | de | en | zh-CN |
|---|---|---|---|
| `common.save` | Speichern | Save | 保存 |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | 选择你的个人资料。 |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | 你的片单是空的 |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | 已创建备份“{{name}}”。 |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | 音量 {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | 正在加载… |
| `player.skipIntro` | Intro überspringen | Skip intro | 跳过片头 |
