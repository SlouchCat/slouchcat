# Abnahmecheckliste (Claude muss Resultate ausfüllen)

Zustände: `[ ]` offen, `[x]` nachweislich geprüft, `BLOCKER` bei Abweichung. Die folgenden Kästchen werden bewusst nicht vorab als bestanden markiert.

## Bestands-/Funktionsvollständigkeit

- [ ] Alle 19 Haupt-Administrationsabschnitte der aktuellen `sections.ts` sind im neuen Design vorhanden und direkt erreichbar.
- [ ] Alle bestehenden Detailrouten, darunter Podcast, Hörbuch-Quellen und Titeldetails, bleiben erreichbar.
- [ ] Alle bestehenden Formulare inkl. Default, Validierung, Pflichtfeldern, Fehlermeldungen und Speichern/Verwerfen sind erfasst und getestet.
- [ ] Jedes Bestands-UI-Element und jede vorhandene API-Mutation sind in die tatsächliche Soll-Matrix eingetragen, mit Quellenzeile und Testbeleg.
- [ ] **Null** fehlende Bestandsfunktionen/Optionen/Statusfälle; Mockup-Beispiele ohne reale Basis wurden nicht als neue Funktionen interpretiert.
- [ ] Polling, Live-Status, Fehlzustände, leere Listen, Bestätigungen, Rücknavigation und Query-Parameter funktionieren.

## Design / UX

- [ ] Neue Navigation entspricht dem freigegebenen sechs-Bereiche-Konzept.
- [ ] Das Dashboard priorisiert Handlungsbedarf und zeigt nicht redundant dieselben Kennzahlen.
- [ ] Desktop, Tablet, Mobil sind übersichtlich und ohne horizontales Überlaufen; Bedienung per Touch/Keyboard ist möglich.
- [ ] Bestehende Slouchcat-Farb-Tokens, Profilakzente, Typography/Dichte und Komponenten werden respektiert.
- [ ] Keine Funktion wird nur wegen eines ruhigeren Layouts versteckt, die Nutzer nicht mehr auffinden können.
- [ ] Abweichungen vom Mockup sind dokumentiert und fachlich begründet.

## Sicherheit und Robustheit

- [ ] Administrator/Hauptprofil-Zugriff und andere spezifische Account-/Owner-Regeln sind unverändert.
- [ ] Sensitive Schlüssel/Secrets bleiben maskiert und werden nicht geloggt.
- [ ] Riskante Aktionen erfordern die bisherigen Warnungen und Bestätigungen.
- [ ] Keine Änderung an Datenmodell, Migrations- oder API-Semantik ohne Begründung/Entscheid.

## Prüfungen und Übergabe

- [ ] `task check:fast` und gezielte Tests nach relevanten Schritten dokumentiert.
- [ ] Neue/angepasste UI-Primitives haben Storybook und Tastaturnachweis.
- [ ] Alle bestehenden betroffenen E2E bestehen, inklusive Mobile/Deeplinks.
- [ ] `task ci` vor PR bestanden und GitHub CI grün; Testbefehle, Logs und Commit-SHAs dokumentiert.
- [ ] PR enthält Nachweise, Screenshots, Restpunkte (falls vorhanden) und vollständig aktualisierte Bestandsmatrix.
- [ ] Kein Merge/Deployment ohne ausdrückliche Freigabe.

## Kritische Abnahmegrenze

**Es darf nicht „100 % funktionsgleich“ behauptet werden, solange die tatsächliche Vollständigkeitsmatrix und Tests nicht alle bestehenden Funktionen nachvollziehbar abdecken.**
