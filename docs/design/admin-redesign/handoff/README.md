# Slouchcat – Verwaltungs-Redesign: freigegebenes Design / Claude-Übergabe

**Version:** Handoff 1.0 – 15.11.2026  
**Repository:** https://github.com/SlouchCat/slouchcat (`main` zum Zeitpunkt der Recherche; vor jeder Arbeit aktuellen HEAD lesen).  
**Designstatus:** Vom Auftraggeber als Gestaltungsrichtung freigegeben.  
**Implementierungsstatus:** Die Mockups sind **keine** implementierte React-Verwaltung und nicht mit der Slouchcat-API verbunden.

## In welcher Reihenfolge lesen?

1. `docs/01_CLAUDE_STARTPROMPT.md` – direkt in Claude Code einfügen.
2. `mockup/Slouchcat_Verwaltung_V2.html` – **primäre visuelle und interaktive Referenz**; lokal in Firefox/Chromium öffnen, nicht 1:1 in React kopieren.
3. `docs/02_DESIGN_UND_INFORMATIONSARCHITEKTUR.md` – Layout, Navigation, Styling und Interaktionsprinzipien.
4. `data/19_hauptrouten_mapping.csv` – aus Repository-Navigation zugeordnete Hauptbereiche.
5. `data/69_mockup_detailansichten.csv` – aus dem Prototyp extrahierte Optionen **nur als Mockup-Referenzen**, keine verifizierte Funktionsvollständigkeit.
6. `docs/03_BESTAND_MIT_ZUORDNUNG.md` und `docs/04_IMPLEMENTIERUNG_UND_TESTS.md` – technische Rahmenbedingungen und Tests.
7. `docs/05_ABNAHME_CHECKLISTE.md` – verbindliche Abnahme.
8. `screenshots/` – visuelle Desktop-/Mobilreferenzen; `branding/` – optional die früher gewählte Cat-Loop-Marke (bitte Bestand im Repo prüfen, keine ungefragte Logo-Änderung).

## Kritisch: Wo endet die Aussagekraft der Quellen?

- `web/src/features/admin/sections.ts` enthält **19** reguläre Verwaltungs-Einträge; `/admin` (Übersicht) kommt zusätzlich hinzu.
- Im V2-Mockup gibt es **69** Detailkonzepte auf **20** Themen-Seiten, inklusive Server und Hörbuchquellen-Bezug.
- Einige Bezeichnungen, Standardwerte und Eingabefelder sind **illustrative Mockup-Beispiele** und möglicherweise nicht mit dem aktuellen Servermodell identisch. **Nicht als Backend-Spezifikation verwenden.**
- Der gesamte echte Funktionsumfang kann grösser sein als das Mockup: Aktionen in Row-Details, Bestätigungen, Unterseiten, Rechte, Medienvarianten, Status- und Fehlerfälle sind im Repository nachzuerheben.
- Bei Kollision zwischen Mockup und `docs/planning/04-entscheidungen.md`, Plan, Spec oder API ist das **Repository mit seinen verbindlichen Entscheidungen und dem bestehenden Verhalten führend**. Visuell freigegeben heisst nicht: Funktional neue Regeln genehmigt.

## Nicht tun

- Keine vorhandenen Funktionen, Einstellungen, API-Felder oder Arbeitsabläufe entfernen.
- Keine Demo-Aktionen ungeprüft in produktive APIs übersetzen.
- Keine neue Technologie, Backend-Funktion oder neue Berechtigung nur wegen eines Mockups erfinden.
- Nicht direkt auf `main` entwickeln oder ohne Freigabe mergen/deployen.

## Erfolg

Gleiche oder bessere funktionale Abdeckung und Testbarkeit bei deutlich klarerer Navigation, visuell auf Basis des freigegebenen Designs.
