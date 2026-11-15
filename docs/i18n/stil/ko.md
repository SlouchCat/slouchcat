# Stilregeln: 한국어 (`ko`)

Koreanisch. Gilt mit dem Glossar und `README.md` in diesem Ordner.

## Anrede und Ton

- **해요체** (O-1) in Sätzen und Meldungen: «프로필을 선택하세요.», «저장했어요.»
- Kein Pronomen für den Leser («당신»). Ton wie Netflix Korea: freundlich, knapp, ohne Ausrufezeichen.

## Knöpfe, Überschriften, Menüs

- **Knöpfe als Nomen** ohne Satzende und ohne Punkt: «저장», «취소», «추가», «폴더 추가»,
  «오프닝 건너뛰기».
- Überschriften ohne Punkt.

## Anführungs- und Satzzeichen

- Anführungszeichen **“ … ”**, darin ‘ … ’.
- Satzzeichen halbbreit (. , ? :), wie in koreanischen Apps üblich.
- Auslassung **…** direkt am Wort: «불러오는 중…».
- **Wortabstand (띄어쓰기)** wie in der Rechtschreibung; die Oberfläche bricht dank `word-break:
  keep-all` nur an Leerzeichen um. Darum lange Wörter ohne Leerzeichen vermeiden.

## Zahlen und Einheiten

- Zahlen formatiert `Intl` (`1,234,567.5`).
- Prozent ohne Leerzeichen: «{{percent}}%». Einheiten: «{{hours}}시간 {{minutes}}분», «{{seconds}}초».
- Zählwörter: 개 (Allgemeines), 편 (Filme), 회 (Folgen), 시즌, 명 (Personen).

## Pluralformen: nur `other`

- Koreanisch hat keine Pluralformen. Jeder pluralisierte Schlüssel bekommt **nur** `_other`, dazu `_zero`,
  wo `en.json` ihn hat.

```json
"episodes_other": "{{count}}회"
```

## Platzhalter und Grammatik

- **Partikeln nach Platzhaltern** hängen vom letzten Laut ab (을/를, 이/가, 은/는, 으로/로). Am besten
  umformulieren, sodass kein Partikel direkt folgt («{{name}} 프로필을 삭제할까요?»). Wenn nicht anders
  möglich, beide Formen: «{{name}}을(를)».
- Verb am Satzende.

## Länge

Koreanisch ist meist kürzer als Englisch, braucht aber doppelte Zeichenbreite. `check.ts` zählt Zeichen
in voller Breite doppelt.

## Beispiele

| Schlüssel | de | en | ko |
|---|---|---|---|
| `common.save` | Speichern | Save | 저장 |
| `profiles.picker.intro` | Wähle dein Profil. | Choose your profile. | 프로필을 선택하세요. |
| `watchlist.empty.title` | Deine Merkliste ist leer | Your watchlist is empty | 찜 목록이 비어 있어요 |
| `admin.backup.created` | Sicherung „{{name}}“ erstellt. | Backup “{{name}}” created. | “{{name}}” 백업을 만들었어요. |
| `player.volume` | Lautstärke {{percent}} % | Volume {{percent}} % | 볼륨 {{percent}}% |
| `common.loading` | Wird geladen … | Loading … | 불러오는 중… |
| `player.skipIntro` | Intro überspringen | Skip intro | 오프닝 건너뛰기 |
