# Projekt-Skills von Slouchcat

Hier entstehen die eigenen Skills für Design und Oberflächen. **Noch keiner ist angelegt:** Sie
halten die Regeln des neuen Designs fest und werden erst nach Marcos Freigabe dieses Designs
geschrieben (Auftrag vom 28. November 2026). Bis dahin gelten `CLAUDE.md`, die Tokens in
`packages/design-tokens` und die Bauteile in `web/src/ui` und `android/core/design`.

Ein Skill ist ein Ordner `<name>/SKILL.md`; diese Datei hier ist keiner. Die Skills von aussen
liegen nicht hier, sondern in `~/.claude/skills` (`docs/dev/DESIGN-WERKZEUGE.md`).

## Geplant

Neun statt elf: `slouchcat-layout` geht in `slouchcat-design-system` auf (Raster, Umbrüche und
Abstände hängen an denselben Tokens), `slouchcat-feature-parity` in `slouchcat-visual-qa` (der
Abgleich der Funktionen ist ein Prüfschritt).

| Skill | Inhalt nach der Freigabe | Quelle der Wahrheit | Vorgeladen bei |
|---|---|---|---|
| `slouchcat-design-system` | Farben, Schrift, Abstände, Radien, Raster und Umbrüche, Bauteile und ihre Zustände, Dichte | `packages/design-tokens/tokens/`, `web/src/ui`, `android/core/design` | `design-director`, `ui-engineer`, `design-reviewer` |
| `slouchcat-branding` | Zeichen, Wortmarke, Schutzraum, Mindestgrössen, Asset-Satz je Plattform und wie er entsteht | SVG-Master, `web/scripts/icons.mjs`, `docs/store/grafiken/` | `brand-designer` |
| `slouchcat-media-ui` | Poster, Reihen, Raster, Detailseiten, Bilder und Ersatzbilder, Hörbücher, Podcasts, Radio | Startseite, Bibliotheken, Detailseiten | `design-director`, `ui-engineer` |
| `slouchcat-player-ui` | Video-, Audio- und Radiospieler, Bedienleiste, Menüs, Trickplay, Untertitel | Konzepte 13, 18, 23 | `ui-engineer`, `motion-designer` |
| `slouchcat-admin-ui` | Verwaltung: Seitenleiste, Formulare, Tabellen, Jobs, Zustände | `docs/design/admin-redesign/`, ADR 0035 | `ui-engineer`, `ux-architect` |
| `slouchcat-tv-ui` | 10-Fuss-Oberfläche, D-Pad-Fokus, Fokusreihenfolge, Überscan, Textgrössen | Konzept 26, `android/app-tv` | `ui-engineer`, `design-qa` |
| `slouchcat-mobile-ui` | Handy-App und PWA am Handy: Navigation, Blätter, Gesten, Randlos | Konzept 27, `android/app-mobile` | `ui-engineer`, `motion-designer` |
| `slouchcat-accessibility` | WCAG 2.2 AA im Web, TalkBack und Zielgrössen in den Apps, Tastatur und Fernbedienung | Regel 8 in `CLAUDE.md`, Tokens-Kontrasttest | `ux-architect`, `design-reviewer` |
| `slouchcat-visual-qa` | Prüfmatrix (Grössen, Sprachen, Hell/Dunkel, Geräte), Baselines, Funktionsabgleich alt/neu | `scripts/design-tools-check.sh`, Storybook-Baselines, `funktionsabdeckung.md` | `design-qa`, `design-reviewer` |

Nach dem Anlegen eines Skills: in `.claude/agents/<agent>.md` unter `skills:` eintragen und
in `docs/dev/DESIGN-WERKZEUGE.md` nachführen. Die Skills von aussen treten dann zurück, wo ein
eigener Skill dieselbe Frage beantwortet.

## Vorlage

```markdown
---
name: slouchcat-<bereich>
description: <Wann laden, in einem Satz; Englisch, wie die übrigen Agent-Texte.>
---

<Regeln, je mit Begründung und prüfbar. Verweise auf Dateien statt Abschriften. Englisch.>
```

Regeln für diese Skills: prüfbar statt allgemein, jede Regel mit ihrem Grund, eine Aussage an einem
Ort, Werte nie abschreiben, sondern auf die Token-Datei zeigen (Skill
`oh-my-claudecode:agent-doc-discipline`).
