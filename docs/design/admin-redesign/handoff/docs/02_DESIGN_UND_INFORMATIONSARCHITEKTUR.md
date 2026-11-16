# Freigegebenes Design und Informationsarchitektur

## Ziel

Die aktuelle Verwaltung hat viele gleichwertige Navigationspositionen und teilweise komplexe Einzelansichten. Die V2 ordnet Aufgaben nach Nutzerintention. Die UI ist ruhig, hierarchisch, symmetrisch und dark-only. Sie **reduziert die Informationsdichte auf der ersten Ebene**, nicht den tatsächlichen Funktionsumfang.

## Sechs Hauptbereiche

| Bereich | Inhalt | Bedienprinzip |
|---|---|---|
| **Übersicht** | Dashboard | Status und Aktion vor Detail |
| **Medien** | Bibliotheken, Podcasts, Webradio, Pflege, Metadaten; Hörbuch-Quellen als Unterseite | Unterseiten/Reiter und kontextbezogene Formulare |
| **Nutzer & Geräte** | Konten, Geräte, Streams | Unterseiten/Reiter und kontextbezogene Formulare |
| **Startseite** | Spotlight, Cat’s Empfehlungen, Sammlungen, Startseitenreihen, Meine Medien, Mitteilungen | Unterseiten/Reiter und kontextbezogene Formulare |
| **Automatisierung** | Hintergrundjobs, Integrationen, Benachrichtigungen | Unterseiten/Reiter und kontextbezogene Formulare |
| **System** | Statistiken, Sicherungen; Serverstatus über die Übersicht und System-Kontext | Unterseiten/Reiter und kontextbezogene Formulare |

## Seitenmuster

- **Verwaltungsübersicht:** Systemwarnungen mit klarer Aktion, Live-KPIs ohne Doppelzählung, unmittelbar laufende Jobs/Streams, letzte Aktivität. Längere Fehlertexte und technische Details liegen auf der jeweiligen Detailseite, bleiben aber verfügbar.
- **Listen:** Einheiten wie Bibliotheken, Konten, Podcasts, Jobs, Geräte und Streams haben kompakte Such-/Filterleisten, erkennbare Statuswerte, sprechende Aktionsmenüs, Bulk-Aktionen falls bisher vorhanden und eine Detailsicht.
- **Bearbeitung:** Ein Bereich pro Problemklasse, kurze klare Feldlabels, Hilfetext nur bei Bedarf, gespeicherter Zustand erkennbar, ungespeicherte Änderungen mit Bestätigungsabfrage. Kein unsichtbares Autosave einführen, wo heute explizit gespeichert wird.
- **Operationen mit Risiken:** Löschen, Datenverlust bestätigen, Zuordnungen zusammenführen, Backups wiederherstellen, Streams beenden und Server neu starten behalten explizite Warnung und Bestätigung.
- **Medien-/Startseitenkuratierung:** Reihenfolge, Sichtbarkeit und Vorschau müssen nachvollziehbar und miteinander konsistent sein; nicht alle Eigenschaften in eine Liste zusammenpressen.
- **Automatisierung:** Status/Fehler auf oberster Ebene; manuelle Aktionen, Zeitpläne, Ausführungsfenster, Historie und Fehlertexte in tieferen Details.

## Visuelles System

Basis ist das vorhandene Slouchcat Token-System, keine frei erfundene Palette:

- Hintergrund: `color.bg.base` – `#0b0c10`
- Elevation: `color.bg.elevated` – `#13151b`
- Surface-1/2/3: `#16181f` / `#1d2029` / `#262a35`
- Text: `#f3f3f5` / `#b6b9c4` / `#9095a3`
- Standardakzent Ember: `#e8794a`; Profilakzente sind **dynamisch**, nicht auf Ember festkleben
- Status: danger `#f2777a`, success `#5fcf8f`, warning `#e8b65a`
- Details wie Abstände, Radius, Dichte, Fokus und responsive Regeln sind aus `packages/design-tokens/` und `web/src/ui` zu beziehen.

## Integrationsbedingungen

Der HTML-Prototyp ist eine **Designreferenz** und keine Vorlage zum Copy-Paste von Styles oder Fake-Daten. Alle realen Komponenten verwenden die vorhandenen Datenquellen, den Router, React-ARIA, i18n und Rechteprüfungen. Der Admin hat sowohl eine Desktop-Navigation als auch ein mobiles Menü. Seitenwechsel und Query-Parameter sind weiterhin deep-linkfähig. Die bestehende visuelle Identität der Zuschauerseiten bleibt erhalten.

## Logo

Die `branding/`-SVGs dokumentieren die bereits ausgewählte Cat-Loop-Form. Die Admin-Neugestaltung verwendet das bestehende Slouchcat-Logo bzw. die im Repo gültige Logo-Komponente. Keine separate Markenmigration ohne Abstimmung; eine ältere PWA- oder Player-Logo-Version ist kein Grund, die Player-Funktionalität jetzt mitzuerweitern.
