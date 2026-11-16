# Implementierung und Tests – konkrete technische Regeln

## Aus dem aktuellen Repository abgeleitet

- Stack: Go-Server, React 19/TypeScript, Vite, TanStack Router und React Query, React-ARIA, i18next, pnpm/Node 24, Taskfile.
- Einstiegsdateien: `web/src/app/router.tsx`, `web/src/app/paths.ts`, `web/src/features/admin/sections.ts`, `web/src/features/admin/AdminShellNav.tsx`, `web/src/features/admin/AdminLayout.tsx`, `web/src/ui/AdminSideNav.tsx`, `web/src/ui/AdminPage.tsx`, `web/src/features/admin/AdminHomePage.tsx`.
- Adminseiten: `web/src/features/admin/` inklusive Unterordner `libraries/`, `maintenance/`, `picks/`, `collections/`, `integrations/`, `jobs/`, `radio/` usw.
- Zentrale UI-/Designregeln: `packages/design-tokens/tokens/color.json`, `web/src/ui/global.css`, `web/src/ui/density.css`, `web/src/ui/aria.ts`, `web/src/app/session.ts`.
- API-/Datenzugriff: `web/src/features/admin/queries.ts` und je Teilbereich spezifische React-Query/Mutation-Komponenten. `api/openapi.yaml` ist die API-Vertragsquelle.
- Zugriff auf Adminseiten ist im Bestand an Administratorrolle **und** Hauptprofil gekoppelt (`session.account.role === 'admin' && session.profile.isPrimary`); verbindliche tatsächliche Server-Authorization nicht auf Frontendbedingung reduzieren.
- Schlüsseltexte: `web/src/locales/*.json`; `de`, `en`, gegebenenfalls `de-DE` und weitere Sprachen prüfen.

## Integrationsregeln

1. **Bestehende Routen beibehalten**. Bei navigativer Umstrukturierung alte Routen und `?tab=`, `?item=`, `?days=` etc. nicht unabsichtlich brechen. Browser-Historie, aktive Nav, Rücksprünge, Links aus anderen Bereichen und Tests absichern.
2. **Keine API-Semantik ändern**, wenn die UI nur neu strukturiert wird. Mutation-/Query-Zustände, Error-Mapping, Invalidierung, Loading-Skeletons, Status-Announcements und Polllimits erhalten.
3. **Neue UI anhand vorhandener Komponenten**. Keine HTML- oder Inline-Farben aus dem Demo übernehmen; Tokens und UI-Primitives nutzen. Neue Bauteile mit Storybook-Story und visueller Prüfung.
4. **Übersetzungen** in bestehender i18n-Struktur; keine hart codierten deutschen UI-Texte, keine Farbwerte im produktiven Component-CSS.
5. **Zugänglichkeit**: Tastatur/Fokusnavigation, Screenreader-Labels und Statusmitteilungen, Tab-Bedienung, Dialog-Fokus und Rückkehr, mobile Navigation und Zustände.
6. **Daten- und Rechtesicherheit**: Secrets nie im Klartext anzeigen/loggen; Kontofunktionen von Admin/Owner-Berechtigungen sauber trennen; bei Lösch-/Restore-/Restart-Aktionen doppelte Schutzmechanismen erhalten.
7. **Feature-Audit zuerst**: Vor Löschen/Ersetzen eines Bestandsbauteils auf vollständige Abdeckung seiner Funktionen verweisen.

## Teststrategie genau wie CLAUDE.md

- **Während Umsetzung:** `task check:fast` und selektive Tests (`task e2e:quick -- <spec>`; `task storybook:test` für UI-Primitives), keine ganze CI nach jeder Änderung.
- **Vor Pull Request:** `task ci` genau einmal auf dem fertigen Branch, Belege dokumentieren. Bei späten Korrekturen anschliessend nur schnelle plus gezielte Tests, GitHub CI prüft PR vollständig.
- **Nach Pull Request:** GitHub CI Gate `ci` grün; keine Merges ohne Freigabe.
- OMC-Regeln: höchstens drei Executor-Agenten, Heavy-Slot-/Port-Regeln befolgen, Testresultate derselben SHA nicht unnötig doppelt ausführen, Fehler nicht mittels Skip/Relaxierungen verdecken.

## Empfohlene Regressionen

- Bestehende Tests in `web/e2e/admin-*.spec.ts`, `web/e2e/admin-jobs.spec.ts`, `web/e2e/admin-phone.spec.ts`, `web/e2e/admin-integrations.spec.ts`, `web/e2e/admin-metadata.spec.ts`, `web/e2e/admin-accounts.spec.ts` usw. nicht kommentarlos entfernen.
- Navigation: 19 Legacy-Abschnitte und Nested-Routen, direkte URLs, Zurück, Reiter-Suchparameter, Desktop- und Mobile-Menü, mindestens 390/768/1440 px.
- Dashboard: ohne Hinweise, mit Fehlern, laufende Jobs/Streams, Server offline, teilweise Daten nicht verfügbar.
- Alle Formulare: Default-Werte, Edit/Save/Discard, Dirty-Guard, serverseitige Validierung, Concurrency und Fehler.
- Rollen: nicht berechtigte Konten/Profile können weder UI noch API-Aktionen auslösen.
- Jobs: Run/Pause/Resume/Cancel/Retry, Schedule, Event-Zeitfenster, Timezone und Filter, Follow-up zu anderen Verwaltungsbereichen.
- Backups: Passphrase und Upload-Flow, Restore mit Neustart. Integration: Provider-Schlüssel inkl. Environment-Overrides und Test, Push, Mail, Telegram und Webhookpfade.
- User-Einstellungen, Profilakzent und Theme: keine Umfärbung auf fixe Ember-Farbe.

## Prüfkriterien pro Feature

`Old UI element → old API / state / permission → new UI location → test id → passed / evidence`.

Jede Bestandsfunktion muss diese Spur haben. Eine gekürzte Liste mit 69 Mockup-Einträgen genügt ausdrücklich nicht.
