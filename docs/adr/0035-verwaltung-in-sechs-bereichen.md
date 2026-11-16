# ADR 0035: Verwaltung in sechs Bereichen

- Status: gebaut auf dem Zweig `claude/admin-redesign` (15. November 2026), **angenommen mit dem
  Merge**; **ändert ADR 0019** in zwei Punkten (die Verwaltungsleiste zeigt sechs Einträge statt
  vier Gruppen mit ihren Seiten; die Serverfunktionen der Übersicht stehen auf einer eigenen Seite)
  und **ergänzt ADR 0020** (zur kompakten Dichte kommt ein eigenes Aussehen der Verwaltung).
  Aktivitätsprotokoll und Neustart aus ADR 0019 gelten unverändert.
- Quelle: Übergabepaket des Auftraggebers vom 15. November 2026 (`docs/design/admin-redesign/handoff/`,
  klickbares Mockup `Slouchcat_Verwaltung_V2.html`); **Frage 31** in
  `docs/planning/04-entscheidungen.md`; Bauplan `docs/design/admin-redesign/bauplan.md`
- Vertrag unverändert, keine Migration, kein neuer Endpunkt
- Betroffene Spec-IDs: A-2, A-3, A-5
- Harte Regeln: 8 (Tastaturtest der neuen Seite `/admin/server` und des Rahmens), 9 (Aussehen nur
  aus Tokens, Texte nur aus den Sprachdateien; dieselben Bauteile für Verwaltung und Zuschauer)

## Kontext

Die Verwaltung hatte 19 Seiten in vier Gruppen, alle als Einträge der Seitenleiste (ADR 0019), dazu
die Übersicht mit acht gleich grossen Karten, darunter die ganze Serververwaltung (Updates,
Installation, Neustart, Speicher, Fehlerprotokoll). Marco hat am 15. November 2026 ein Mockup
freigegeben, das die Verwaltung nach Aufgaben ordnet, und den Umbau unter einer Bedingung
beauftragt: Keine bestehende Funktion, Option oder Einstellmöglichkeit darf verloren gehen.

Zu entscheiden war, was über eine einzelne Seite hinausgeht: das Navigationsmodell, der Ort der
Serverfunktionen, das Verhältnis zur Kopfzeile der App und wie das neue Aussehen die Zuschauerseiten
unberührt lässt.

## Entscheid

| Punkt | Entscheid |
|---|---|
| **Navigationsmodell** | Die Seitenleiste zeigt «Zurück zur App», «Übersicht» und fünf Bereiche: Medien, Nutzer & Geräte, Startseite, Automatisierung, System. Die Seiten eines Bereichs stehen als **Bereichsreiter** über jeder Seite des Bereichs. Eine Quelle: `web/src/features/admin/sections.ts` (`adminAreas`, `adminSections`). Aktuell ist die Seite mit dem längsten passenden Pfad. |
| **Adressen** | Alle Adressen `/admin/...` samt `?tab=`, `?item=`, `?days=` bleiben. Neu ist nur `/admin/server`. «Hörbuch-Quellen» bleibt unter `/admin/metadata/audiobooks` und ist zusätzlich ein Reiter des Bereichs Medien. |
| **Zahlen und Hinweise** | Die Zahlen der Seiten (Titel, offene Pflegefälle, Konten, Streams, Jobs, Integrationen) stehen an den Bereichsreitern, mit ihren gesprochenen Namen. Der Bereichseintrag der Seitenleiste zeigt nur, was einen Blick braucht (Pflegefälle, laufende Streams, fehlgeschlagene Jobs, stille Webhooks, «Update»). |
| **Kopfzeile** | Die Kopfzeile der App bleibt in der Verwaltung (Logo, Glocke, Profil). Dazu kommen der Zusatz «Control Center», Brotkrumen und der Knopf «Bereich suchen», der statt des Verweises «Suche» steht; die Mediensuche bleibt über `/` und die Palette erreichbar. |
| **Suchpalette** | `Strg+K` / `⌘K` und der Knopf öffnen einen Dialog mit allen Bereichen und Seiten, «Zurück zur App» und «Mediensuche». Reine Navigation, keine neue Datenquelle. Sie lädt mit dem Verwaltungs-Bündel (P-10). |
| **Handy** | Der Knopf «Verwaltung › Bereich / Seite» über jeder Seite öffnet das Blatt mit allen Seiten nach Bereichen. Die Bereichsreiter stehen auch auf dem Handy und brechen um. |
| **Seitenkopf** | `AdminPage` zeigt im Rahmen der Verwaltung eine Zeile «BEREICH / SEITE», den Titel grösser und die Einleitung offen (höchstens zwei Zeilen, «Mehr» zeigt den Rest) statt hinter dem Hilfe-Knopf. Der Rahmen kommt aus einem Kontext (`web/src/ui/adminFrame.ts`), den `AdminLayout` setzt; ohne ihn (`/settings/*`) bleibt `AdminPage`, wie es war. |
| **Übersicht** | Hinweisband, vier Kennzahlen als Verweise, «Was benötigt Aufmerksamkeit?» (alle bisherigen Hinweise, dazu Integrationsfehler und stille Webhooks), «Gerade aktiv», Schnellzugriffe auf die Bereiche, Kurzstatus des Servers mit Aktivität. Abfrage alle 5 s wie bisher. |
| **Server** | Neue Seite System → Server (`/admin/server`) mit allen Serverfunktionen der bisherigen Übersicht, ungekürzt: Version, Laufzeit, Videokodierung, Dateiwächter, Updates und Update-Installation, Neustart, beide Speicherpfade (Konzept 11, F-4), Aktivität, Fehlerprotokoll mit Ansage (A-3: 5-s-Abfrage). Grafikchip-Wahl, öffentliche Adresse und Update-Prüfung bleiben auf «Integrationen». |
| **Integrationen** | Kacheln je Dienst mit Zustand; jeder Dienst hat eine eigene Ansicht (`?item=keys|webhooks|email|telegram|push|server`). Der Zustand aller Integrationen und Webhooks, bisher auf der Übersicht, steht hier (gleiche Abfrage `getDashboard`). |
| **Aussehen** | Das Aussehen des Mockups (Karte, Tabelle, Kennzahl, Abzeichen, Hinweisband, Knopf, Feld) gilt nur unter `data-shell="admin"`: `web/src/ui/admin-look.css` stellt Komponenten-Tokens um, die die Bauteile mit dem bisherigen Wert als Rückfall lesen. Farben nur aus den vorhandenen Tokens, Akzent ist der Profilakzent. Die kompakte Dichte (ADR 0020) bleibt die Grundlage. |
| **Mockup gegen Bestand** | Wo das Mockup dem Bestand widerspricht oder Beispiele ohne Basis zeigt, gilt der Bestand (Bauplan 7.1). Die Karte «Optionen & Arbeitsabläufe» des Mockups ist ein Gerüst des Prototyps und wird nicht gebaut. Seitentitel bleiben, wie sie im Produkt heissen. |

## Nachweis des Funktionserhalts

Vor dem Umbau wurden alle Funktionen der Verwaltung am Code aufgenommen
(`docs/design/admin-redesign/functional-inventory.csv`, 876 Zeilen). Nach dem Umbau haben vier
Prüfer, die am Umbau nicht beteiligt waren, jede Zeile gegen den neuen Code geprüft
(`docs/design/admin-redesign/abdeckung/`, zusammengeführt in `funktionsabdeckung.csv`). Ihre Befunde
sind behoben und nachgeprüft; der Stand steht in `docs/design/admin-redesign/funktionsabdeckung.md`.

## Folgen

- Mehr Wege mit einem Schritt mehr: von einem Bereich in eine Seite eines anderen Bereichs geht es
  über den Bereichseintrag oder die Palette. Innerhalb eines Bereichs ist der Wechsel ein Schritt.
- Der Seitenkopf ist höher als bisher (Bereichsreiter, Zeile über dem Titel, Einleitung). Am
  Desktop steht die erste Arbeitszeile bei rund 290 px; auf dem Handy brechen die Reiter um.
- Ein Verweis in einen Dienst der Integrationen nennt den Dienst (`serviceHref`), sonst landet er
  auf der Kachelliste.
- Rückweg: Der Umbau ist reine Web-Oberfläche. Ein Image vor dem Umbau läuft mit derselben Datenbank.

## Verworfen

- **Eigene Kopfleiste ohne App-Kopf (wie im Mockup):** Glocke und Profilwechsel wären von den
  Verwaltungsseiten aus nicht mehr erreichbar gewesen.
- **Alle 21 Seiten weiter in der Seitenleiste, nur neu gruppiert:** widerspricht dem freigegebenen
  Mockup (sechs Einträge).
- **Neue Adressen je Bereich (`/admin/media/libraries`):** hätte Verweise, Lesezeichen und Tests
  gebrochen, ohne Nutzen für die Bedienung.
- **Das neue Aussehen in den Bauteilen selbst:** hätte die Zuschauerseiten verändert.
