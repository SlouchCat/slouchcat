# Weblate einrichten

Konzept 22, Stufe 5 (Abschnitt 4.7, O-8). Diese Seite ist die Anleitung für Marco: Das Konto bei
Weblate und den Antrag stellt er selbst, Claude legt keine Konten an. Was danach im Repository
geschieht, steht unten unter «Weg zurück ins Repository».

## 1. Konto und Antrag

1. Auf <https://hosted.weblate.org> ein Konto anlegen (am einfachsten «Mit GitHub anmelden»).
2. Ein neues Projekt beantragen und den Plan **«Libre»** wählen. Er ist kostenlos für öffentliche
   Projekte unter freier Lizenz. Slouchcat erfüllt beides: Das Repository `SlouchCat/slouchcat` ist
   öffentlich und steht unter AGPL-3.0. Die Bedingungen beim Antrag selbst lesen und bestätigen.
3. Projektname **Slouchcat**, Projekt-Website `https://github.com/SlouchCat/slouchcat`.

Bis Weblate das Projekt freigibt, kann es einige Tage dauern.

## 2. Zwei Komponenten

Je Komponente die folgenden Werte. Die Feldnamen von Weblate können je nach Version leicht anders
heissen; gemeint ist jeweils das gleichnamige Feld im Formular «Komponente hinzufügen».

| Feld | Komponente `web` | Komponente `server` |
|---|---|---|
| Versionskontrolle | GitHub-Pull-Request | GitHub-Pull-Request |
| Quellcode-Repository | `https://github.com/SlouchCat/slouchcat.git` | gleich |
| Repository-Zweig | `main` | `main` |
| Push-Zweig | `weblate` | `weblate` |
| Dateiformat | i18next JSON file v4 | i18next JSON file v4 |
| Dateimaske | `web/src/locales/*.json` | `server/internal/notify/locales/*.json` |
| Einsprachige Basisdatei | `web/src/locales/en.json` | `server/internal/notify/locales/en.json` |
| Vorlage für neue Übersetzungen | `web/src/locales/en.json` | `server/internal/notify/locales/en.json` |
| Quellsprache | Englisch | Englisch |
| Sprachfilter | `^(?!de$|de-DE$|de_DE$).*$` | gleich |

- **Vorlage ist Englisch**, weil die meisten Übersetzer Englisch lesen. Quelle des Projekts bleibt
  Deutsch (Schweiz) im Repository.
- **Deutsch ist in Weblate ausgeschlossen** (Sprachfilter). `de` und `de-DE` pflegt das Repository
  selbst, so wie die Regel in `docs/i18n/README.md` es verlangt.
- Die Datei `locales.test.ts` liegt im gleichen Ordner wie die Web-Texte. Die Dateimaske `*.json`
  trifft sie nicht.
- **Add-on «JSON-Ausgabe anpassen»:** Einrückung 2 Leerzeichen, Schlüssel nicht sortieren. So bleiben
  die Dateien im Format, das Prettier im Repository erwartet. Danach eine Vorschau prüfen: Weicht die
  Ausgabe trotzdem ab, meldet es die CI im ersten Pull Request (`prettier --check`).
- **Add-on «Git-Commits zusammenfassen»** («Squash Git commits»), damit ein Pull Request von Weblate
  nicht aus hunderten Einzelcommits besteht.

## 3. Weg zurück ins Repository

- Weblate committet auf den Zweig `weblate` und öffnet einen Pull Request gegen `main`.
- Die CI prüft ihn wie jeden anderen. Für die Übersetzungen zählen dabei `scripts/i18n/check.ts --all`
  (Aufbau, Platzhalter, Pluralformen, Leerraum) und `TestServerTexts`.
- **Gemergt wird auf Marcos Wort**, wie jeder Pull Request.
- **Regel gegen Kollisionen (4.7):** Ab jetzt ändern Agenten bestehende Übersetzungen der zwanzig
  Sprachen nicht mehr. Sie füllen vor einem Release nur fehlende Texte nach («Übersetzungen
  nachziehen»), und zwar erst, wenn offene Weblate-Pull-Requests gemergt sind. Korrekturen laufen über
  Weblate.

## 4. Plakette

Sobald das Projekt besteht, kommt die Plakette mit dem Stand je Sprache in `docs/i18n/README.md` und in
das `README.md` des Repositorys. Weblate zeigt den Code unter «Teilen» des Projekts, etwa:

```markdown
[![Übersetzungsstand](https://hosted.weblate.org/widget/slouchcat/multi-auto.svg)](https://hosted.weblate.org/engage/slouchcat/)
```

Die genaue Adresse hängt vom Kurznamen ab, den Weblate dem Projekt gibt. Claude trägt sie ein, sobald
Marco sie nennt.
