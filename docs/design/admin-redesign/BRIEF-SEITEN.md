# Auftrag Schritte 2 und 3: Seiten der Verwaltung umbauen

Gilt für jeden Seitenschritt (Übersicht/Server/Integrationen, Medien, Nutzer & Geräte, Startseite,
Automatisierung, System). Welche Seiten du hast, steht in deinem Auftrag.

## Lies zuerst

1. `CLAUDE.md` im Wurzelverzeichnis. Gilt ohne Ausnahme.
2. `docs/design/admin-redesign/bauplan.md` (Vorgabe) und `rahmen-ergebnis.md` (was Schritt 1 gebaut
   hat: Bauteile, Props, Token-Stellen, Muster). Beides in deinem Arbeitsbaum.
3. Das Mockup `docs/design/admin-redesign/handoff/mockup/Slouchcat_Verwaltung_V2.html`: die
   Darstellung deiner Seiten im Quelltext lesen (Skript: je Seite eine Render-Funktion) und, wenn
   nötig, im Browser ansehen. Es ist die Gestaltungsvorlage, nicht die Anforderung an Funktionen.
4. Das Inventar deiner Seiten: `docs/design/admin-redesign/inventory/<teil>.md` und `.csv`. **Jede
   Zeile der CSV ist eine Funktion, die nach deinem Umbau noch da sein muss.** Der Abschnitt
   «Abgleich mit dem Mockup» sagt, was am Mockup nur Beispiel ist (nicht bauen) und was im Mockup
   fehlt (bleibt trotzdem).

## Ziel je Seite

Die Seite sieht aus und ist aufgebaut wie ihre Seite im Mockup, mit **allen** Funktionen des
Inventars:

- Seitenkopf über `AdminPage`: Titel, Einleitung, Hauptaktion(en) rechts wie im Mockup (z. B.
  «Bibliothek hinzufügen»). Die Hauptaktion ist eine bestehende Aktion der Seite, die nach oben
  rückt; keine neue erfinden.
- Inhalt in Karten und Tabellen nach den Mustern des Mockups (Karte mit Kopf, Kennzahl-Kacheln,
  Tabelle mit Zustand als Abzeichen und sprechender Aktion am Zeilenende, Filterleiste über der
  Liste, Hinweisband, Speichern-Leiste).
- Wo das Mockup weniger zeigt als der Bestand, bleibt der Bestand und fügt sich in dieselben Muster.
  Wo das Mockup etwas zeigt, das es im Code nicht gibt (Einstufung `Beispiel ohne Basis`), wird es
  **nicht** gebaut. Wo Mockup und Bestand sich widersprechen, gilt der Bestand.
- Die Karte «Optionen & Arbeitsabläufe» des Mockups wird nicht gebaut.
- Nichts wird nur für ein ruhigeres Bild versteckt: Was heute sichtbar ist, bleibt ohne Umweg
  auffindbar (aufklappbare Einzelheiten und Dialoge, die es heute schon gibt, sind in Ordnung).

## Was gleich bleibt

- Adressen und Suchparameter (`?tab=`, `?item=`, `?days=`, Anker), Reiter-IDs, Verweise von und zu
  anderen Seiten.
- Abfragen, Poll-Takte, Mutationen, Neuladen danach, Fehlerzuordnung. Keine neue Datenquelle, kein
  doppelter Zustand. Kein neuer Endpunkt, keine Änderung an `api/openapi.yaml`, am Server oder an
  generiertem Code.
- Bestätigungen, Warnungen, Abfrage bei ungespeicherten Änderungen, ausdrückliches Speichern.
- Berechtigungen (Administrator mit Hauptprofil, Eigentümer-Regeln), Anzeige für
  Nicht-Administratoren.
- Tastatur: Tab-Reihenfolge, Pfeiltasten, Fokus nach Löschen, Fokusrückkehr aus Dialogen, sichtbarer
  Fokus, keine Fokusfalle. Ansagen für Screenreader.
- Lade-, Leer-, Fehler- und Zwischenzustände.
- Handy unter 768 px: kein waagrechtes Überlaufen der Seite (Tabellen dürfen in sich rollen oder zu
  Karten werden, wie heute).

## Regeln

- Nur die Dateien deiner Seiten ändern (`web/src/features/admin/<seite>…`, ihre Tests, ihre
  e2e-Specs, ihre Texte in `web/src/locales/de.json`, `en.json`, `de-DE.json`). Gemeinsame Bauteile
  in `web/src/ui` und der Rahmen gehören Schritt 1. Brauchst du dort etwas (neues Bauteil, neue
  Prop, neue Token-Stelle), baue es **additiv** (bestehende Aufrufer unverändert), mit Story und
  Test, und nenne es in der Rückmeldung; ändere kein bestehendes Verhalten eines gemeinsamen
  Bauteils.
- Farben nur aus Tokens, Texte nur aus den Sprachdateien (`de` und `en`, `de-DE` bei ss-Wörtern und
  «Guillemets»). Bestehende Texte weiterverwenden; neue nur, wo ein Muster einen braucht. Texte, die
  einen Ort in der Verwaltung nennen, an den neuen Ort anpassen.
- Bestehende Tests bleiben. Wo sich die Bedienung absichtlich ändert, den Test anpassen, nicht
  streichen. Jede Funktion, die du verschiebst, hat danach einen Test an der neuen Stelle.
- Jede **neue** Seite braucht einen Playwright-Tastaturtest (Regel 8).

## Tests (Stufe 1)

- `task check:fast` vor jedem Commit.
- Vitest deiner Seiten (läuft in `check:fast` über `vitest related`).
- e2e gezielt mit dem Port aus deinem Auftrag: `E2E_PORT=<port> task e2e:quick -- e2e/<spec> …`
  für jede Spec deiner Seiten (Liste im Inventar, Spalte «Tests heute»), dazu
  `e2e/admin-phone.spec.ts` und `e2e/admin-shell.spec.ts`, wenn deine Seiten dort vorkommen.
- `task storybook:test` nur, wenn du ein Bauteil in `web/src/ui` angefasst oder ergänzt hast.
- Höchstens drei Korrekturversuche je roter Prüfung, dann anhalten und melden. Nichts überspringen,
  keine Timeouts verlängern, keine Grenzen lockern. Kein `task ci`, keine ganze e2e-Suite.

## Commits

Ein Commit je Seite (oder je eng zusammengehörender Seitengruppe) auf deinem Zweig, Titel englisch
mit Bereich (`feat(web): …`), letzte Zeile genau:

`Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>`

Nicht pushen. Nur in deinem Arbeitsbaum arbeiten.

## Bildschirmfotos

Je Seite ein Bild in 1440×900 und 390×844 (Wegwerf-Skript gegen den e2e-Server, nicht committen)
nach
`/tmp/claude-1000/-home-marco--claude-spares-spare-QLalpS/e5de04e4-51ba-44f0-bdf1-b99aeb6b0581/scratchpad/seiten/<seite>-<breite>.png`.

## Rückmeldung

- Commit-SHAs mit je einem Satz.
- Je Prüfung: Befehl und die entscheidenden Zeilen der Ausgabe.
- Je Seite: was umgebaut wurde; welche Inventar-IDs den Ort gewechselt haben (alt → neu); welche
  Tests angepasst oder neu sind, mit Grund.
- Ergänzungen an gemeinsamen Bauteilen.
- Abweichungen vom Mockup mit Grund (für die Abweichungsliste).
- Inventar-Zeilen, die du **nicht** erhalten konntest oder bei denen du unsicher bist. Lieber eine
  Unsicherheit zu viel melden als eine Funktion verlieren.
