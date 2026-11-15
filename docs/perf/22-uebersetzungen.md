# Konzept 22, Stufe 4: Bildprüfung der Übersetzungen

Einmaliger Lauf nach Konzept 22, Abschnitt 4.6, Schritt 3 (kein Gate, keine Vergleichsbilder). Stand:
Zweig `claude/k22-stufe4`, 15. November 2026, mit den Übersetzungen der ersten Runde; die Texte der
Stufen 1 bis 3 waren in einem Teil der Sprachen noch nicht nachgezogen.

## Was geprüft wurde

- **Sprachen:** `de` (Vergleich), `de-DE`, `fi`, `hu`, `ru`, `pt-BR`, `ja`, `zh-TW`.
- **Breiten:** 1280 px (Rechner) und 390 px (Handy).
- **Storybook:** alle 102 Geschichten der 21 Bausteine aus `web/src/ui`, die ihre Texte selbst
  übersetzen (`PlayerChrome`, `AudioPlayer`, `AdminPage`, `AdminSideNavMenu`, `HeroCarousel`, `Row`,
  `SaveBar`, `SeekBar`, `ProviderList`, `TaskRow`, `StreamRow` und weitere), über die Werkzeugleiste
  «locale» (`?globals=locale:<code>`). Chromium im Playwright-Container mit `fonts-noto-cjk`, nur für
  diesen Lauf nachinstalliert. Die übrigen Geschichten bekommen ihre Texte als deutsche Argumente und
  sagen über eine Übersetzung nichts.
- **Echte Seiten** gegen den e2e-Server (Chromium des Rechners, Noto Sans CJK vorhanden): Startseite,
  Detailseite, Suche, Profile, Wiedergabe-Einstellungen, Abspielseite, Verwaltung (Übersicht,
  Bibliotheken, Metadaten, Hintergrundjobs, Konten, Integrationen), Anmeldeseite, Assistent Schritte 1
  und 2.
- **Wie:** je Seite ein Bildschirmfoto und eine Suche im DOM nach Text, der abgeschnitten ist
  (`scrollWidth` grösser als die Box bei `overflow: hidden` oder `text-overflow: ellipsis`), aus seiner
  Box läuft oder über den Rand des Fensters reicht. Gemeldet wurde nur, was in `de` an derselben Stelle
  nicht auftritt; die Bilder dieser Stellen wurden angesehen.

## Behoben (CSS, ohne Texte zu kürzen)

| Stelle | Sprache | Befund | Behebung |
|---|---|---|---|
| Seitenleiste, Rolle unter dem Profilnamen (`SideNav.module.css`, `.profileRole`) | `ru` | «Администратор» ist breiter als die Spalte und läuft unter den Knopf «Сменить». | `overflow-wrap: break-word` und `hyphens: auto`: Das Wort wird in der Sprache der Seite getrennt. |
| Seitenleiste der Verwaltung (`AdminSideNav.module.css`, `.label`) | `pt-BR` | «Tarefas em segundo plano» endet als «Tarefas em segundo pl…». | Die Beschriftung bricht auf eine zweite Zeile um statt abzuschneiden; der Eintrag hat dafür oben und unten etwas Innenabstand. Eine Zeile bleibt gleich hoch. |

## Gesehen, bewusst so belassen

- **Aktivitäten der Übersicht** (`ActivityFeed`): Einträge sind einzeilig mit Auslassungspunkten. In
  `fi`, `hu`, `ru` und `ja` werden längere Meldungen früher gekürzt als in Deutsch; der volle Text steht
  in der Benachrichtigung. Kein Fehler im Aufbau.
- **Tabellen** (Konten, Hintergrundjobs): In `ru`, `pt-BR` und auf dem Handy in `zh-TW` reicht die
  letzte Spalte über den Rand. Die Tabellen scrollen dort waagrecht wie in Deutsch auf schmalen
  Bildschirmen.
- **Brotkrumen der Verwaltung auf dem Handy** (`· Tarefas em segundo plano`, `· Выбор языка`): einzeilig
  mit Auslassungspunkten, wie lange deutsche Namen.
- **Hinweis der Quellenliste** (`ProviderList`, 390 px): einzeilig gekürzt in `hu`, `ru`, `pt-BR`.

## Schriften

- Die `:lang()`-Stapel von Stufe 1 greifen mit den ausgelieferten Codes: `<html lang>` trägt `ja`,
  `ko`, `zh-CN` und `zh-TW`, und `global.css` setzt dafür `--sc-font-family-ja` und die anderen. Die
  Bilder in `ja` und `zh-TW` zeigen japanische und traditionelle Formen, keine Kästchen.
- Bausteine, die keine eigene Schrift setzen, erben über `font: inherit` vom `body`.

## Weitergegeben an die Übersetzung

- `player.audio.plus` und `player.audio.minus` («+0.1», «−0.1») stehen in `fi` mit Punkt, während
  daneben «1,25×» mit Komma formatiert ist. Sprachen mit Dezimalkomma sollten «+0,1» schreiben, wie
  `de-DE`.
- Ein Text, der einer Sprache fehlt, zeigt kurz seinen Schlüssel, bis Englisch nach dem ersten Bild
  geladen ist (ADR 0033, «Laden der Kette»). In Storybook war das für `zh-TW` bei
  `player.audio.plus` zu sehen, bevor die Datei nachgezogen war. Vor einem Release schliesst
  `task i18n:check` das aus.
- In `ru` lautete der Hinweis der Quellenliste noch «no немецкий texts»: Der Text
  `library.providers.missingLanguages` aus Stufe 1 fehlte, Englisch stand ein, mit dem russischen
  Sprachnamen im Platzhalter. Das löst das Nachziehen der Texte der Stufen 1 bis 3.
