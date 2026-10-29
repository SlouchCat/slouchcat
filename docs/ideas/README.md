# Ideensammlung für Slouchcat nach Version 1.0

Diese Sammlung hält Ideen fest, die **erst nach Version 1.0** in Slouchcat kommen. Jede Idee wird in einem
Interview geklärt und als Konzept ausgearbeitet, damit sie später ohne erneute Grundsatzdiskussion geplant
werden kann. Nichts hier ist beauftragt.

## Regeln

- Ein Konzept hier ist **keine Anforderung**. Massgeblich bleiben Spec, Plan und Entscheidungsprotokoll
  (`CLAUDE.md`). Kein Agent setzt eine Idee um, bevor sie bewusst in Spec und Plan übernommen wurde.
- Konzepte dürfen Non-Goals der Spec berühren. Das Konzept benennt das ausdrücklich; der Non-Goal fällt erst
  mit einem Eintrag im Entscheidungsprotokoll.
- Jede Idee hat ein Konzept `docs/ideas/NN-<thema>.md` und ein GitHub-Issue mit Label `idea`. Das Konzept ist
  die ausführliche Fassung, das Issue trägt Kurzfassung, Status und Diskussion.
- Status: **Entwurf** (Interview läuft), **Ausgearbeitet** (Konzept vollständig), **Vorgemerkt**
  (Zielversion genannt), **Übernommen** (in Spec und Plan), **Verworfen** (mit Begründung).

## Aufbau eines Konzepts

1. Problem und Ziel
2. Zielgruppe und Nutzen
3. Abgrenzung zur Spec v1 (betroffene Non-Goals, berührte Anforderungen)
4. Nutzerfluss
5. Datenmodell und API-Skizze
6. Berührungspunkte mit den harten Regeln (Sichtbarkeit, Entscheid im Server, i18n, Secrets)
7. Offene Fragen
8. Grobe Aufwandsschätzung und Abhängigkeiten

## Index

| Nr. | Idee | Status | Konzept | Issue |
|---|---|---|---|---|
| 01 | Metadaten- und Scraper-Einstellungen je Bibliothek (Sprachen, Bildarten, Anbieter) | Teil A übernommen (Anbieterliste, feldweises Auffüllen, Herkunft je Feld, über Konzept 07 / ADR 0015, 29. Oktober 2026); Teil B ausgearbeitet | [01-metadaten-einstellungen.md](01-metadaten-einstellungen.md) | [#8](https://github.com/SlouchCat/slouchcat/issues/8) |
| 02 | Ordner-Browser im Bibliotheksformular (Dialog mit Liste statt getipptem Pfad) | Übernommen | [02-ordner-browser.md](02-ordner-browser.md) | [#33](https://github.com/SlouchCat/slouchcat/issues/33) |
