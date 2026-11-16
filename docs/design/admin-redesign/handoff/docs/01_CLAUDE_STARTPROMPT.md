# Auftrag an Claude Code: Slouchcat-Verwaltung auf Basis des freigegebenen Designprototyps neu umsetzen

Du arbeitest im GitHub-Repository `SlouchCat/slouchcat`. Das Ziel ist eine vollständige **Überarbeitung der gesamten Verwaltungsoberfläche**, nicht nur des Dashboards. Der Auftraggeber hat die visuelle und strukturelle Richtung von `mockup/Slouchcat_Verwaltung_V2.html` freigegeben.

## Eingaben / Reihenfolge

1. Lies **zuerst** die Repo-Datei `CLAUDE.md`, anschliessend `docs/dev/CURRENT_STATUS.md`, `docs/planning/04-entscheidungen.md`, `docs/planning/03-plan-v1.md`, `docs/spec/01-anforderungen-v1.md` und relevante ADRs. Bei Widersprüchen gilt die Repo-Priorität: Entscheidungen > Plan > Spec.
2. Lies aus diesem Übergabepaket `README.md`, `docs/02_DESIGN_UND_INFORMATIONSARCHITEKTUR.md`, `docs/03_BESTAND_MIT_ZUORDNUNG.md`, `docs/04_IMPLEMENTIERUNG_UND_TESTS.md`, `docs/05_ABNAHME_CHECKLISTE.md`, `data/19_hauptrouten_mapping.csv`, `data/69_mockup_detailansichten.csv`.
3. Öffne **unbedingt** `mockup/Slouchcat_Verwaltung_V2.html` interaktiv im Browser. Prüfe die Desktop- und Mobilreferenzen unter `screenshots/`.
4. Lies den aktuellen `main`-Stand neu ein. Die Tabellen aus diesem Paket sind Hilfsmittel, keine Gewähr für den HEAD zum Implementierungszeitpunkt.

## Oberste nicht verhandelbare Bedingung: Null Funktionsverlust

Kein bestehender Button, keine Einstellung, kein Dialog, keine Such-/Filter-/Sortiermöglichkeit, kein Tab, kein Status, keine Fehlermeldung, keine Tastatursteuerung und keine Berechtigungsprüfung darf wegfallen oder unzugänglich werden. Alle Funktionen müssen mindestens gleich gut funktionieren. Der Umfang schliesst Sonderfälle, Lade-/Leer-/Fehlerzustände und direkte Deeplinks ein.

Die **19 Hauptbereiche** aus `web/src/features/admin/sections.ts`, die zusätzlichen Unterseiten (z. B. `/admin/metadata/audiobooks`, `/admin/podcasts/:id`, `/admin/items/:id`) sowie Querverlinkungen und die geräteabhängige Admin-Navigation müssen inventarisiert werden. Die **69 Mockup-Detailansichten** sind Designkandidaten; daraus NICHT ableiten, dass 69 Einträge eine vollständige reale Inventarliste wären.

## Gewünschte Navigationsstruktur

- **Übersicht:** Zustand, Handlungsbedarf, laufende Streams/Jobs, Systeminformationen und Aktivitäten, ohne lange und redundante Listen.
- **Medien:** Bibliotheken, Metadaten, Pflege, Podcasts, Webradio, Hörbuch-Quellen (als thematisch passende Unterseite).
- **Nutzer & Geräte:** Konten, Geräte und Streams.
- **Startseite:** Startseitenreihen, Spotlight, Cat’s Empfehlungen, Sammlungen, Meine Medien, Mitteilungen.
- **Automatisierung:** Hintergrundjobs, Integrationen, Benachrichtigungen.
- **System:** Statistiken, Sicherungen, Serverstatus/-aktionen (bestehende Serverfunktionen aus `/admin` nicht verlieren).

Die bisherigen `/admin/...`-Routen und Query-Parameter (`?tab=`, `?item=`, `?days=` usw.) nach Möglichkeit erhalten, damit Deeplinks, Browser-Zurück, Verweise und Tests weiter funktionieren. Die neue Navigation ist eine UI-Organisation, kein Zwang zu Backend-/URL-Umbenennungen.

## Verbindliches Design

- Ruhiger, hochwertiger Dark-Mode, orientiert an der gelieferten V2-HTML und den Screenshots.
- Bestehende Token-Farben statt hart kodierter Farben: `packages/design-tokens/tokens/color.json`; aktiver Profilakzent mit `--sc-accent`, Standard Ember `#e8794a`.
- Keine neue visuelle Parallelwelt: bestehende `web/src/ui`-Komponenten bevorzugen oder behutsam erweitern; React-ARIA und bestehende Admin-Dichte nutzen.
- Reduzierte, logisch gruppierte Navigation; Übersichtsseite priorisiert Handlungsbedarf, Live-Status, Aufgaben und die nächsten Aktionen.
- Lange Einstellungslisten in strukturierte, klar benannte Abschnitte und kontextbezogene Details aufteilen; fortgeschrittene Optionen bleiben sichtbar auffindbar.
- Tabellen/Listen mit Suche, Filter, Sortierung, Auswahl und Aktionen beibehalten, wo vorhanden; keine Funktionsreduktion zu dekorativen Karten.
- Responsives Verhalten für Desktop, Tablet und Mobil; Bedienbarkeit per Tastatur und sichtbarer Fokus.
- Ausgewähltes Branding «Cat Loop» aus `branding/` ist eine Referenz, **kein Auftrag zur eigenständigen Änderung des bestehenden Play/Pause- oder PWA-Icon-Systems**. Prüfe zuerst vorhandene Logos; nur dort angleichen, wo der Admin-Auftrag es erfordert.

## Vorgehen – direkt anfangen, aber keinen riskanten Blindumbau

### Phase 0: Funktionsaudit als erstes Arbeitsergebnis

- Aktuelle `main`-Revision und vorhandene Änderungen prüfen. Feature-Branch nur auf einer sauberen Ausgangsbasis erstellen; keine Änderungen des Nutzers überschreiben.
- **Vollständige Bestandsmatrix** als Datei im Projekt anlegen (z. B. `docs/design/admin-redesign/functional-inventory.md` und maschinenlesbar `.csv`): Für jede bestehende Funktion aktuelle Route/Komponente, Bedienelement, Sichtbarkeits-/Rollenbedingung, API/Mutation, Zustände, neue Position, Testbeleg.
- Zusätzlich alle existierenden Admin-UI-Tests, Storybook-Stories, Router-Suchparameter und relevante API-Operationen erfassen.
- Jede Mockup-Option gegen das Repository klassifizieren: **bestehende Funktion / zulässige rein visuelle Umgruppierung / Beispiel ohne funktionale Basis / offene Frage**. Beispielwerte oder Aktionen ohne Basis NICHT neu implementieren.
- Wenn ein bestehendes Verhalten im Design fehlt, das Design ergänzen. Bei einem echten fachlichen Widerspruch: STOP für diese Teilentscheidung und Befund mit konkreten Alternativen vorlegen; andere konfliktfreie Teile dürfen weiter bearbeitet werden.

### Phase 1: Architektur und Umsetzungsschritte

- Konkrete Navigations- und Komponentenarchitektur planen; sinnvoll in kleine, testbare Arbeitsschritte aufteilen.
- Besondere Aufmerksamkeit auf die sehr komplexen Seiten `JobsPage`, `MetadataPage`, `LibrariesPage`, `AccountsPage`, `IntegrationsPage` und die bisherigen Live-Kacheln der `AdminHomePage`.
- Bestehende API/Backendlogik primär unverändert weiterverwenden; kein neues State-Duplikat, keine neue Datenquelle, keine Regression bei Polling, Loading, Empty, Error und Offline.
- Den visuellen Vorschlag als Leitlinie nehmen, **nicht** HTML/inline-CSS aus dem Prototyp in produktiven Code kopieren. Repo-Regeln haben Vorrang.

### Phase 2: Implementierung

- Erst die Admin-Navigation und das Shell-Layout, dann Übersicht und Funktionsgruppen schrittweise migrieren.
- Funktionen seitenweise übernehmen und in der Bestandsmatrix als nachweislich umgesetzt markieren.
- Bestätigungsdialoge, Entwürfe/ungespeicherte Änderungen, Zugriffsrechte, Fokusmanagement, Rücknavigation, i18n, Responsiveness und progressive Ladezustände ausdrücklich erhalten.
- Keine generierten Dateien direkt editieren. Farben nur über Tokens, neue Texte in unterstützte Locale-Dateien (v. a. `de`, `en`, erforderliche Varianten).
- Jeder abgeschlossene Planschritt: eigener Commit und fokussierte Prüfungen mit Belegen.

### Phase 3: Review, vollständige Abnahme und PR

- Vor PR die Bestandsmatrix vollständig auf **grün** bringen: alte Funktion → neue Stelle → nachgewiesener Test. Keine unmarkierten Lücken, keine Stilllegungen.
- Screenshots der realen Implementierung in relevanten Breiten und mindestens die wichtigsten Zustände mit dem Mockup vergleichen.
- Bestehende Admin-E2E und UI-/Unit-Tests beibehalten; neue Tastatur-, Navigations- und Regressionstests ergänzen.
- Nach `CLAUDE.md`: während der Arbeit `task check:fast` und gezielte Tests; **einmal** vor PR `task ci`, dann GitHub CI. Nicht bei jeder Datei komplette CI ausführen. Maximal drei Executor-Agenten und Heavy-Slot-Regeln beachten.
- Erst nach bestandener QA und PR-Belegen bereitstellen. **Nicht ohne ausdrückliche Freigabe in `main` mergen, deployen oder produktive Daten verändern.**

## Ergebnis / Definition of Done

Liefere (a) neue funktionierende Verwaltungs-UI, (b) Bestands-/Soll-Matrix für jede Funktion und Einstellung, (c) überarbeitete Tests, (d) Screenshots Desktop/Mobil, (e) Prüfung aller 19 bisherigen Verwaltungsbereiche und zusätzlichen Detailrouten, (f) Liste visueller Abweichungen zum Mockup mit Begründung, (g) nachvollziehbaren PR- und CI-Nachweis. Melde unverifizierte Punkte als offen; keine Aussage «100 % vollständig» ohne überprüfte Belege.

**Arbeitsbeginn ist jetzt freigegeben; die optische Designrichtung ist angenommen.** Prüfe vor der produktiven Implementierung die tatsächliche Funktionsinventur. Keine eigenmächtigen fachlichen Erweiterungen und keine Funktionsverluste.
