# Bestandsaufnahme Teil «automatisierung-shell»

Stand: Code im Arbeitsbaum `claude/admin-redesign` auf `main` `55b72c9a` (Release 1.38.0), gelesen am
15. November 2026. Nur gelesen, nichts ausgeführt. Zeilenliste: `automatisierung-shell.csv` (216 Zeilen).

Konventionen: Dateipfade ohne Präfix sind relativ zu `web/src/`; e2e-Specs relativ zu `web/`.
Unit-Tests stehen mit Dateiname und Testname, e2e mit `Datei:Zeile «Testname»`. Operationen mit
`operationId` aus `api/openapi.yaml`. Starthilfe war `konzept-verwaltung-ux/.../02-inventar-c-konten-system.md`
(Stand 1.31.0); jede Zeile ist neu am Code geprüft, Ergänzungen aus den Konzepten 21 und 22 sind markiert.

| Seite | Kürzel | Zeilen |
|---|---|---|
| Navigation und Rahmen | NAV | 32 |
| Übersicht `/admin` (inkl. Neustart und Update) | OVW | 66 |
| Hintergrundjobs `/admin/jobs` | JOB | 43 |
| Integrationen `/admin/integrations` (inkl. Webhooks) | INT | 51 |
| Benachrichtigungen `/admin/notifications`, `/settings/notifications` | NOT | 24 |

---

## 1. Navigation und Rahmen (NAV)

### Aufbau heute

- **Einstieg aus der App:** nur der Eintrag «Verwaltung» (Symbol wrench) im Band «Konto» der App-Seitenleiste
  und derselbe in der Menü-Schublade (`app/AppLayout.tsx:312-325`). Kein Link im App-Kopf, kein eigenes
  Tastenkürzel; «/» öffnet die Suche auch auf `/admin` (`AppLayout.tsx:133-153`). Abzeichen «Update» am Eintrag
  (`session.updateAvailable`).
- **Schale:** `data-shell="admin"` auf `<html>`: zuerst durch das Inline-Skript in `index.html:15-16` (nur nach
  Pfad), dann durch `AppLayout.tsx:164-178` (nur Admin mit Hauptprofil). Davon hängen die reservierte
  Spaltenbreite (`ui/global.css:266-271`) und die kompakte Dichte (`ui/density.css`) ab.
- **Spalte ab 768 px** (`useWideScreen`, `(width >= 768px)`): `AdminShellNav` lazy in `AppLayout.tsx:426-430`
  statt der App-Spalte, auch wenn das Gerät die App-Spalte ausgeblendet hat. Der Menü-Knopf im Kopf öffnet
  dann die App-Schublade (ohne «Seitenleiste einblenden»).
- **Handy unter 768 px:** `AdminLayout.tsx:13-23` setzt per `PageNav` (`ui/Page.tsx:10-29`) über jede Seite den
  Knopf «Verwaltung · {Seite}» (`ui/AdminSideNavMenu.tsx`), der das Blatt «Seiten der Verwaltung» öffnet.
- **Einträge** (`features/admin/AdminShellNav.tsx`, `sections.ts`): «Zurück zur App», «Übersicht» (Abzeichen
  «Update»), dann vier Bänder ohne Überschrift: Bibliothek (Bibliotheken, Podcasts, Radio, Pflege, Metadaten),
  Konten (Konten, Geräte, Streams), Kuratierung (Spotlight, User-Empfehlungen, Sammlungen, Startseite, Meine
  Medien, Ankündigungen), System (Hintergrundjobs, Integrationen, Benachrichtigungen, Statistiken, Sicherung).
  21 Einträge.
- **Zahlen und Pillen** (`AdminShellNav.tsx:71-109`): gedämpft Titel (Bibliotheken), Konten, laufende Jobs;
  Pillen Pflege (warning), Streams (accent), Fehlschläge (danger), stille Webhooks (warning); «x / y»
  eingerichtete Integrationen. Jede Zahl hat einen ausgeschriebenen Sprechnamen (`admin.nav.counts.*`).
- **Seitenkopf** `ui/AdminPage.tsx` (alle Seiten ausser der Übersicht): h1 als Fokusziel, (i) «Hilfe zu dieser
  Seite», «Live, alle N Sekunden», Aktionen, Zustandsleiste; öffnet `AdminDensityScope`.
- **Nicht-Admin oder Nebenprofil:** `AdminLayout` rendert nur `<Outlet />`; jede Seite prüft die Rolle selbst
  und zeigt `admin.access.notAllowed.*`; `/admin/notifications` leitet um.

### Suchparameter und Deeplinks

- `adminSearch` (`app/router.tsx:51-79`): `?tab=` und `?item=` nur als Kennung (Regex `SELECTION`, bis 64
  Zeichen, Punkt erlaubt, keine `..`), Zahl wird Text. In diesem Teil nutzt nur `/admin/jobs` den Validator;
  `/admin`, `/admin/integrations`, `/admin/notifications` haben keine Suchparameter.
- `useUnsavedChanges` behandelt `tab` und `item` als «gleiche Seite» (`ui/useUnsavedChanges.ts:6`): ein Wechsel
  fragt nicht. Neue Reiter als eigene Pfade würden die Abfrage auslösen.

### Abfragen

- `getDashboard` alle 30 s auf jeder `/admin/*`-Seite ausser `/admin` (dort pollt die Übersicht 5 s und teilt
  den Cache, `AdminShellNav.tsx:22,48-51`). Ohne Antwort stehen die Einträge ohne Zahlen.
- `getSession` liefert das Update-Abzeichen (App-Eintrag und «Übersicht»).

### Fallen für den Umbau

1. `ADMIN_OVERVIEW_INDEX = 1` (`AppLayout.tsx:57-58`): der Fokus beim Betreten hängt an der Reihenfolge
   «Zurück zur App», «Übersicht».
2. `focusCarry` lebt als Ref in `AppLayout` und überträgt den Index des fokussierten Links; ändert sich die
   Zahl oder Reihenfolge der Einträge zwischen Seiten (z. B. aufklappende Bereiche), stimmt der Index nicht mehr.
3. Lazy-Load der Leiste ist Absicht (P-10, `ui/aria` nicht im Start-Bundle); ein Chunk-Fehler darf nur die
   Spalte leeren.
4. Dichte und Spaltenbreite hängen an `data-shell="admin"`; das Inline-Skript in `index.html` muss mitwandern.
5. Rollenprüfung ist in jeder Seite doppelt (Layout und Seite); die neue Shell darf sie nicht nur zentral
   machen.
6. «y» der Integrations-Zahl sind die 8 Dashboard-Schlüssel (ohne Fanart.tv, Google Books), die Seite zählt
   «N von 6 Anbietern»: zwei verschiedene Zählungen.
7. Kontrasttest der Pillen je Profilakzent (`AdminSideNav.test.tsx`) muss im neuen Dark-Design bestehen.
8. `admin-phone.spec.ts` deckt `/admin/radio` nicht ab (Lücke, nicht dieser Teil).

### Abgleich mit dem Mockup

Das Mockup hat für die Navigation keine `D0xx`-Zeile. Einstufung der sichtbaren Teile:

- Sechs Bereiche mit Reitern statt 19 Einträgen in vier Bändern: **visuelle Umgruppierung** (Zielorte nach
  `19_hauptrouten_mapping.csv`).
- «Bereich suchen ⌘ K» (Verwaltungssuche): **Beispiel ohne Basis**.
- «Demo-Modus», «Interaktiver Entwurf», «CONTROL CENTER»: **Beispiel ohne Basis** (Prototyp-Kennzeichen).
- Kopf mit «Bereich / Reiter»-Pfad und sichtbarer Beschreibung: **offene Frage** (heute Einleitung hinter (i),
  Entscheid S-2).

### Im Mockup fehlt (muss bleiben)

«Zurück zur App»; Update-Abzeichen an «Übersicht» und am App-Eintrag «Verwaltung»; Zahlen und Alarm-Pillen mit
Sprechnamen; ein Tab-Stopp mit ↑/↓/Home/End; `focusCarry` und Fokus beim Betreten/Verlassen; Handy-Knopf
«Verwaltung · Seite» mit Blatt (Fokusfalle, Escape, Fokus zurück); App-Schublade über den Menü-Knopf; Dichte
nur mit Maus ab 768 px; Abfrage bei ungespeicherten Änderungen; Verhalten für Nicht-Admins und Nebenprofile;
Deeplinks `?tab=`/`?item=`.

---

## 2. Übersicht (OVW)

### Aufbau heute (`features/admin/AdminHomePage.tsx`, 1624 Zeilen)

- Kopf (`:1571-1581`): h1 «Verwaltung», Live-Punkt «Live, alle 5 Sekunden» bzw. «Keine Antwort, neuer Versuch
  alle 5 Sekunden». Kein `AdminPage`, kein Hilfeknopf; `AdminDensityScope` selbst gesetzt.
- Band «Einrichtung nicht abgeschlossen» mit «Einrichtung fortsetzen» (`:1513-1538`, neu seit Konzept 22).
- Raster (`:1484-1503`, `AdminHomePage.module.css:30-43`): acht gleich grosse Karten, eine Spalte unter 44 rem
  Inhaltsbreite, zwei darüber: Server, Jetzt läuft, Aufgaben, Letzte Aktivität, Integrationen, Webhooks,
  Speicher, Hinweise. Darunter das zugeklappte Server-Protokoll (`:1395-1461`).
- Server-Karte (`:929-1077`): Fakten, «Zuletzt geprüft», Updater-veraltet, Fehler, «Nach Updates gesucht.»,
  Installationsblock (`UpdateInstall`, `:651-858`); Fussknöpfe «Nach Updates suchen», «Neu starten».
- «Neu starten» ersetzt nach Bestätigung alle Karten durch `RestartPanel` (`:1587-1593`).
- **Karten der Verwaltungsseiten gibt es nicht mehr** (der Kommentar in `app/paths.ts:76` nennt sie noch);
  die Seiten erreicht man nur über die Leiste.

### Abfragen und Mutationen

| Abfrage / Mutation | Takt | Wofür |
|---|---|---|
| `getDashboard` | 5 s; aus während Neustart-Tafel; pausiert, solange diese Seite eine Installation verfolgt | alle Karten, Hinweise, Protokoll |
| `getSystemInfo` | einmal, 60 s frisch | Version, Laufzeit (Rückfall), GPU, inotify, Hinweise GPU/inotify/Fallback |
| `getHealth` (`useHealth`) | 30 s | «Läuft seit» |
| `getSetupProgress` | einmal (nur Eigentümer mit Hauptprofil) | Einrichtungsband |
| `getHealth` (Neustart, Installation) | 2 s, Neustart gibt nach 90 s auf, Installation meldet nach 70 min | Wartetafel, Installationsverfolgung |
| `getUpdateInstallStatus` | 2 s während Installation, ruht während der Server weg ist | Phasen |
| `checkForUpdate` | sofort | danach `getDashboard` neu |
| `restartServer` | nach roter Bestätigung | 409 `restart_pending` wartet mit |
| `installUpdate` | nach Bestätigung mit Release-Notizen | 409/422/503 je eigener Zustand |
| `dismissMinimumAgeNotice` | sofort («Verstanden») | danach `getDashboard` neu |

Geladen, aber nicht gezeigt: `jobs.recentFailed`, `integrations[].checkedAt`, `systemInfo.timeZone`, `configFs`,
`cacheFs`, `gpu.tonemapper`, `gpu.hwTonemap`, `gpu.ffmpegVersion`, `gpu.devices` (diese zeigt nur der
Einrichtungsassistent über `SystemState`).

### Fallen für den Umbau

1. Texte, die den Ort nennen: `system.gpu.choice.restartAdmin` «… (Übersicht › Neu starten).» und
   `admin.home.browse.*` («Ein Neustart gibt sie frei.») – beim Umzug von «Neu starten» anpassen.
2. Der Hinweis «Gewählter Grafikchip geht nicht» verlinkt mit dem Text «Zu den Schlüsseln» auf
   `/admin/integrations`; das Ziel ist die Grafikchip-Wahl im Band «Server».
3. `admin.home.update.body` sagt «Slouchcat installiert nichts selbst …», `admin.integrations.update.intro`
   «Installiert wird nichts.»: beides widerspricht dem Knopf «Update auf X installieren».
4. `RestartPanel` und `restart.ts` werden auch von `BackupPage` (Wiederherstellen) und `setup/SummaryStep`
   genutzt.
5. Die Installation stoppt das Dashboard-Polling der Übersicht; zieht sie auf eine Server-Seite, braucht die
   Übersicht trotzdem eine Anzeige «Update läuft» oder sie darf in der Zeit nicht pollen (offene Frage).
6. A-3: ein neuer Fehler muss binnen 10 s im Protokoll stehen (`admin-home.spec.ts:264`) – die Server-Seite
   braucht das 5-s-Polling.
7. Die Ansage «N neue(r) Fehler» läuft heute auf der Übersicht auch bei zugeklapptem Protokoll.
8. Die Integrationskarte ist der einzige Ort, der den Zustand «Fehler» mit Grund (`detail`) einer Integration
   zeigt; die Integrationsseite zeigt ihn nicht. Ebenso «Seit sieben Tagen ohne Meldung» (nur Karte und Pille).
9. `relativeTime` rechnet ab dem Zeitpunkt des Polls (`dataUpdatedAt`), nicht ab «jetzt».
10. Fehlschläge je Jobart zeigen den Fehler nur einzeilig mit Tooltip (mit Tastatur nicht erreichbar).

### Zuschnitt: jedes Element der heutigen Übersicht

Ziel nach Mockup: Übersicht knapp (Hinweisband, vier Kennzahlen, «Was benötigt Aufmerksamkeit?», «Gerade
aktiv», Schnellzugriffe, «Server & letzte Aktivitäten»), Serverfunktionen auf neuer Unterseite
«System → Server» (D067–D069).

| Element heute (CSV) | Ort neu | Was nicht verloren gehen darf |
|---|---|---|
| h1, Live-Anzeige, Dashboard-Polling 5 s (OVW-001–003) | Übersicht; Polling auch auf Server-Seite | Live-Zustand und Fehlerzustand; Takt 5 s (A-3) |
| Laden, Fehler mit «Erneut versuchen», Zugang gesperrt (OVW-005–007) | beides | alle drei Zustände je Seite |
| Einrichtungsband (OVW-008) | Übersicht (Hinweisband) | nur für Eigentümer mit Hauptprofil; Text ohne Bibliothek; Link zum ersten offenen Schritt |
| Raster acht Karten (OVW-009) | entfällt als Form | Inhalte nach dieser Tabelle |
| Version, Läuft seit, Videokodierung, Beobachtete Ordner (OVW-010–013) | Server-Seite (D067); Übersicht nur Kurzstatus «Server & letzte Aktivitäten» | Laufzeit aus `/healthz` weitergezählt; «inotify-Limit: unbekannt» |
| Updates-Zeile, Updater-Zeile, «Zuletzt (erfolgreich) geprüft» (OVW-014–017) | Server-Seite (D068); Update verfügbar zusätzlich Übersicht (Hinweisband) | 5 Update-Zustände, 3 Updater-Zustände, beide Daten |
| «Nach Updates suchen» (OVW-018) | Server-Seite | Meldung «Nach Updates gesucht.», Fehler; Dashboard danach neu |
| «Neu starten» mit Bestätigung, Wartetafel (OVW-019–020) | Server-Seite | Streamzahl im Dialog; fehlt während Installation; 409 `restart_pending`; Tafel ersetzt Inhalt, Fokus auf h2, 90 s, «Erneut prüfen», Neuladen |
| Updater veraltet (OVW-021) | Server-Seite | Befehl zum Kopieren, -f-Hinweis |
| Update installieren, Fortschritt, 70 min, Ergebnis, Neuladen, Ablehnung, 409, Handgriffe, Anschluss, Fokus (OVW-022–031) | Server-Seite; Übersicht: Hinweis, dass ein Update läuft (offen) | alle Phasen, Ergebnisse und Gründe; Handgriffe ohne Begleiter; Anschluss bei Neuladen; Fokusführung |
| «Jetzt läuft» mit drei Wiedergaben, leer, «Alle Streams» (OVW-032–034) | Übersicht («Gerade aktiv», Kennzahl «Aktive Streams») | Profil, Gerät, Modus mit Qualität, Fortschritt; Link auf `/admin/streams` |
| Aufgaben: vier Kacheln, laufende, «Als Nächstes», Fehlschläge je Jobart, Links (OVW-035–039) | Übersicht (Kennzahl «Laufende Jobs», «Gerade aktiv», Fehlschläge in «Was benötigt Aufmerksamkeit?») | «Pausiert» nur > 0; nächster Lauf; Link `?tab=failed` |
| Letzte Aktivität, leer (OVW-040–041) | Übersicht («Server & letzte Aktivitäten», gekürzt) und Server-Seite (D069) | alle 14 Arten mit Sätzen und Symbolen, allgemeiner Satz; API liefert nur 6 |
| Integrationen-Karte und Link (OVW-042–043) | offen: Fehler nach «Was benötigt Aufmerksamkeit?», Liste auf Integrationen | 8 Schlüssel mit 4 Zuständen und Grund (`detail`), Zahl «x von y» |
| Webhooks-Karte und Link (OVW-044–045) | offen: stille Webhooks nach «Was benötigt Aufmerksamkeit?», Zahl 24 h auf Integrationen | «Kein Webhook eingerichtet.», Meldungen 24 h, Namen der stillen |
| Speicher (OVW-046) | Server-Seite (D069); Kennzahl auf der Übersicht | **beide** Pfade Konfiguration und Cache mit belegt/frei (Entscheid F-4) |
| Hinweise-Karte mit Zahl (OVW-047) | Übersicht («Was benötigt Aufmerksamkeit?» + Hinweisband «N Punkte …») | Reihenfolge dringlichste zuerst; Ansage der Zahl |
| Hinweise fehlgeschlagen, pausiert, Freigaben neu gelesen, Mindestalter + «Verstanden», Schlüssel abgewiesen, Scan abgebrochen (OVW-048–053) | Übersicht | jeder Link (Jobs `?tab=failed`, Jobs, Integrationen, Bibliotheken); «Verstanden» als Mutation |
| Ordner-Browser blockiert, kein Grafikchip, Grafikchip-Fallback, inotify knapp, Speicher > 90 % (OVW-054–058) | Übersicht (Aufmerksamkeit) und Server-Seite (beim Zustand) | Compose- und sysctl-Befehle zum Kopieren; Hinweis braucht `getSystemInfo` auch auf der Übersicht |
| Neue Version, Entwicklungsstand (OVW-059–060) | Übersicht (Hinweisband) und Server-Seite | Versionshinweise extern; Entwicklungsstand ohne Update-Angebot |
| Befehl kopieren (OVW-061) | beide | «Befehl kopiert.» und markieren bei Fehlschlag |
| Server-Protokoll zu/offen, Ansage neuer Fehler (OVW-062–064) | Server-Seite (D069); Ansage offen | 50 Fehler mit Zeit, Quelle, Job; Zahl im Titel; Ansage auch zugeklappt |
| Tastatur, Dichte (OVW-065–066) | beide | Tab-Reihenfolge ohne Falle; kompakte Dialoge |

### Abgleich mit dem Mockup (D067–D069, Übersicht)

| Zeile | Einstufung | Begründung |
|---|---|---|
| D067 Serverzustand | visuelle Umgruppierung | Version, Laufzeit, Videokodierung und Dateiwächter stehen heute in der Server-Karte. Das Feld «Hardware-Encoder» gibt es als Auswahl auf Integrationen › Server (Optionen auto/NVENC/QSV/VA-API/Software, kein «AMD»): **offene Frage**, wohin sie gehört. «Dateiwächter Grenze 524288» als Eingabe, «Details anzeigen», «Diagnose kopieren»: **Beispiel ohne Basis** (Limit setzt der Host; es gibt nur Anzeige und sysctl-Befehl). |
| D068 Updates & Neustart | visuelle Umgruppierung | «Nach Updates suchen», «Update installieren», «Server neu starten» gibt es auf der Server-Karte; «Updateprüfung aktivieren» steht heute auf Integrationen › Server und gilt sofort. |
| D069 Speicher & Logs | visuelle Umgruppierung, Teil offene Frage | Config, Cache, Fehler bestehen. «Aktivität ansehen» setzt eine längere Liste voraus; die API liefert höchstens 6 Einträge (`Dashboard.activity`, maxItems 6): **offene Frage**. |
| Übersicht: Hinweisband «N Punkte benötigen Aufmerksamkeit» + «Ansehen» | visuelle Umgruppierung | entspricht Zahl und Inhalt der Hinweise-Karte. |
| Übersicht: vier Kennzahlen | visuelle Umgruppierung, Teil Widerspruch | «Aktive Streams», «Laufende Jobs (wartend, pausiert)», «Titel zur Pflege» (= `counts.maintenanceOpen`) bestehen als Daten; «Speicherplatz 72 %» als eine Zahl widerspricht F-4 (siehe unten). |
| Übersicht: «Was benötigt Aufmerksamkeit?» | visuelle Umgruppierung | die Hinweise mit Link; Mockup-Beispiel «Metadatenanbieter nicht erreichbar» entspricht «Schlüssel abgewiesen» bzw. Integrationsfehler. «Scan erneut ausführen» als Aktion: **Beispiel ohne Basis** (heute nur Link zur Seite). |
| Übersicht: «Gerade aktiv» | visuelle Umgruppierung | laufende Aufgaben, Wiedergaben, nächster Lauf. |
| Übersicht: Schnellzugriffe «Bibliotheken verwalten», «Startseite gestalten», «Automatisierung» | Beispiel ohne Basis | heute keine Karten der Seiten auf der Übersicht; reine Verweise, ohne Funktionsverlust baubar. |
| Übersicht: «Server & letzte Aktivitäten» mit «Online», «Hardwareprüfung aktiv» | visuelle Umgruppierung, Teil ohne Basis | Aktivität besteht; «Online» und «Hardwareprüfung aktiv» gibt es so nicht (Live-Zustand, `hardwareEncode`). |
| Übersicht: «Aktualisieren» im Kopf | Beispiel ohne Basis | die Seite pollt alle 5 s. |

**Widerspruch:** Mockup «Speicher & Laufzeit»: «Daten und Cache können auf demselben Datenträger liegen; die
finale Oberfläche sollte sie dann nicht doppelt zählen.» und eine Kennzahl «Speicherplatz 72 %». Marco hat in
Konzept 11 F-4 entschieden: immer beide Pfade, auch auf derselben Platte (`AdminHomePage.tsx:1368`).

### Im Mockup fehlt (muss bleiben)

Einrichtungsband; Updater-Zeile und Updater-veraltet-Hinweis; alle Install-Phasen, 70-min-Meldung, Ergebnis,
Ablehnungsgründe, Handgriffe von Hand, Anschluss an laufende Installation; Wartetafel mit 90 s und «Erneut
prüfen»; Streamzahl im Neustart-Dialog; Integrationskarte mit Grund; Webhooks 24 h und stille Webhooks;
Hinweise «Freigaben werden neu gelesen», «Mindestalter» mit «Verstanden», «Ordner-Browser blockiert»,
Grafikchip-Fallback, inotify-Befehl, Compose-Befehle, Entwicklungsstand; Ansage neuer Fehler; zwei
Speicherpfade.

---

## 3. Hintergrundjobs (JOB)

### Aufbau heute (`features/admin/JobsPage.tsx`, 2148 Zeilen; `jobsFormat.ts`; `jobs/Jobs.module.css`)

- `AdminPage` «Hintergrundjobs», Hilfe, «Live, alle 5 Sekunden» oder «Aktualisierung pausiert».
- Zustandsleiste: Banner pausierte Jobs (je Jobart «Fortsetzen: Name»), Kacheln Laufend, Wartend, Geplant,
  Pausiert, Fehlgeschlagen.
- Klappgruppe «Läuft gerade»: Tabelle aller laufenden Läufe mit «Lauf pausieren» und «Abbrechen», darunter die
  sechs Arbeiter-Pools.
- Reiter Bibliothek, Metadaten, Zustellung, System (je Tabelle der Jobarten, Zeile klappt auf) und
  «Fehlschläge» (alle Arten, seitenweise).
- Aufgeklappte Jobart: Beschreibung; Knöpfe «Jetzt ausführen», «Pausieren»/«Fortsetzen», Menü mit «Wartende
  abbrechen (N)», «Alle Läufe abbrechen»; Fakten «Ausgelöst durch» (mit Code), «Letzter Lauf»; Block
  «Zeitplan» (Schalter, Takt, Zeitfenster, Einmal-Termin, Verwerfen, Speichern) oder Ereignis-Hinweis bzw.
  Ereignis-Zeitfenster; Block «Läufe» mit sechs Zustandsreitern und Sammelaktionen für Fehlgeschlagene.
- 39 Jobarten mit Namen in `de.json` (`admin.jobs.types.*`): radio_catalog_sync, radio_logo_fetch,
  radio_wikidata_sync, scan_library, scan_path, probe_file, probe_keyframes, keyframes_backfill, library_sweep,
  segments_detect, chapters_images, chapters_images_backfill, trickplay_generate, trickplay_series,
  trickplay_backfill, library_relink, metadata_match, library_author_display, metadata_fill,
  metadata_vote_backfill, metadata_ratings_reread, metadata_original_language_backfill, image_fetch,
  people_images, people_details, people_images_backfill, metadata_apply, trailer_find, podcast_fetch,
  podcast_poll, podcast_sync, podcast_sync_all, podcast_download, trakt_refresh, update_check,
  download_prepare, notify_deliver, jobs_prune, library_delete. Die Gruppe liefert der Server.

### Suchparameter und Deeplinks

- `?tab=library|metadata|delivery|system|failed` (`useSearchSelection`, unbekannt → `library`, Adresse
  bereinigt).
- `?item=<jobart>` klappt die Jobart einmal auf und wählt ihre Gruppe (`JobsPage.tsx:513-530`).
- Einstiege: Übersicht `?tab=failed` und ohne Query; `libraries/LibraryDetail.tsx:191`
  `?item=keyframes.backfill`; `libraries/LibrariesPage.tsx:306` `?item=library.delete`; Metadaten (`:333`,
  `:1032`), Hörbuch-Quellen (`:488`), Radio-Katalog (`radio/OverviewTab.tsx:157`) ohne Query.
- Unterreiter der Läufe und Aufklappzustand stehen nicht in der Adresse.

### Abfragen und Mutationen

- `listJobTypes` alle 5 s; `listJobRuns(type, state, limit 200)` je offenem Unterreiter alle 5 s;
  `listJobRuns(state=failed, offset, limit)` seitenweise beim Scrollen, neu bei Änderung der Gesamtzahl.
  Alle ruhen bei offenem Dialog oder Entwurf.
- Mutationen invalidieren `listJobTypes`, `getDashboard` und alle `listJobRuns`: `runJobType`, `pauseJobType`,
  `resumeJobType`, `cancelJobRuns` (queued/all, Bestätigung), `cancelJob` (Bestätigung in «Läuft gerade» und
  beim Löschen eines Fehlschlags, **ohne** Bestätigung im Läufe-Block), `retryJob`, `retryFailedJobs`,
  `dismissFailedJobs` (je Jobart oder alle, Bestätigung mit Anzahl), `updateJobSchedule`, `updateJobTypeWindow`.

### Fallen für den Umbau

1. Ein Dialog für die ganze Seite (Zeilen können während der offenen Frage verschwinden).
2. Bestätigen heisst «Abbrechen», Schliessen «Behalten».
3. Entwürfe von Zeitplan und Fenster leben in der Seite und überleben Reiterwechsel und Zuklappen; das
   Polling ruht dafür.
4. `trakt.refresh` teilt Takt und Ein/Aus mit Spotlight, `update.check` Ein/Aus mit der Update-Prüfung
   (Integrationen › Server); der Hinweistext nennt «Einstellungen».
5. Der stille Abbruch im Läufe-Block (ohne Bestätigung) unterscheidet sich vom Abbruch in «Läuft gerade».
6. Die Uhr ist ein Modul-Singleton (`useSyncExternalStore`).
7. Zeit des Einmal-Termins ist Wall-Time des Servers, nie in die Browserzone umgerechnet (`formatOnce`).

### Abgleich mit dem Mockup

| Zeile | Einstufung | Begründung |
|---|---|---|
| D050 Jobtypen und Warteschlange | bestehende Funktion | Gruppen und Fehlschläge als Reiter, Laufzustände als Unterreiter, «Job öffnen» = Zeile aufklappen. Abweichung: Mockup-Laufstatus nennt «Pausiert» (ist Zustand der Jobart, nicht des Laufs) und lässt «Wartend» aus. |
| D051 Automatischer Zeitplan | bestehende Funktion | Schalter, Takt (zehn Werte), Zeitfenster, Einmal-Termin, Speichern, Pausieren/Fortsetzen bestehen. Mockup-Standard 01:00–06:00 gehört zum Ereignis-Fenster; der Zeitplan beginnt bei 03:00–05:00. |
| D052 Läufe & Fehler | bestehende Funktion | «Jetzt starten» = «Jetzt ausführen», «Erneut versuchen» = «Erneut» / «Alle erneut versuchen», «Lauf abbrechen», «Fehler verwerfen» = «Löschen» / «Alle löschen …»; Auswahl «Einzelner Lauf / Alle des Typs» = Zeilen- und Sammelaktion. |
| D053 Ereignis-Zeitfenster | bestehende Funktion | `updateJobTypeWindow` mit Schalter und Von/Bis. |
| Mockup-Seite: Filter «Alle / Läuft / Fehler / Pausiert», «Fehler anzeigen» | visuelle Umgruppierung | heute Reiter je Gruppe und «Fehlschläge»; ein Filter nach Zustand der Jobart gibt es nicht. |

### Im Mockup fehlt (muss bleiben)

Banner pausierter Jobs mit Wiedergabe-Vorrang; Arbeiter-Pools mit Stau; «Läuft gerade» mit «Lauf pausieren»,
Chips «wird angehalten», «für Wiedergabe»; Restzeit und sekündliche Uhr; Beschreibung, «Ausgelöst durch» mit
Code; «Wartende abbrechen (N)», «Alle Läufe abbrechen»; «Verwerfen» und «Termin entfernen»; Zeitzone; Hinweise
Trakt/Update; Gruppe «Zustellung»; Reiter «Fehlschläge» mit Nachladen; Läufe «Erledigt» und «Abgebrochen»;
Prio, Versuche, «frühestens …», «N insgesamt, die neuesten M gezeigt»; unbekannte Jobart; Live-Pause bei
Entwurf; Deeplinks `?tab=` und `?item=`.

---

## 4. Integrationen (INT)

### Aufbau heute

`features/admin/IntegrationsPage.tsx` (213 Zeilen) mit sechs klappbaren Bändern, alle offen:

1. «Schlüssel der Anbieter» – sechs `ProviderCard` (TMDB, TVDB, Fanart.tv, YouTube, Trakt, Google Books), je
   zugeklappt mit Zweck und Quelle; offen: Felder, Hilfesatz + Link, «Speichern», «Löschen …», «Prüfen».
2. «Webhooks von Radarr und Sonarr» – `WebhooksSection.tsx`: neue Integration, Adresse einmal, Tabelle mit
   Aufklappen (Pfad-Umsetzung, Meldungsprotokoll), «Token erneuern», «Löschen».
3. «E-Mail-Server» – `EmailForm.tsx`: Server und Port, Benutzer, Passwort, Absender, Verschlüsselung;
   Testmail; Verwerfen, Löschen, Speichern.
4. «Telegram-Bot» – `TelegramForm.tsx`: Bot, Bot-Token mit Speichern/Löschen.
5. «Web-Push» – `PushBlock.tsx`: Zustand, öffentlicher Schlüssel, «Schlüssel neu erzeugen».
6. «Server» – `ServerBlock.tsx`: Update-Prüfung (Kontrollkästchen, sofort), öffentliche Adresse, Grafikchip
   für das Umwandeln (`shared/HardwareChoice.tsx`, neu seit Konzept 22).

Quelle jedes Werts: `deploy/.env` sperrt das Feld; Geheimnisse sind nur schreibbar (Hinweis «endet auf …»).

### Suchparameter und Deeplinks

Keine (kein `validateSearch`). Alle Einstiege landen am Seitenanfang: Übersicht (drei Links), Benachrichtigungen
(Kopfaktion, «Integrationen öffnen»), Metadaten (`:908`), Hörbuch-Quellen (`:426`), Spotlight
(`FetchState.tsx:100`, `SourceForm.tsx:195`). Rückweg: E-Mail-Hilfe → `/admin/notifications`.

### Abfragen und Mutationen

- `getAdminSettings`, `listIntegrations`, `getSystemInfo` ohne Takt (nur bei Fensterfokus);
  `listIntegrationEvents` beim Aufklappen einer Webhook-Zeile.
- `updateAdminSettings` für Schlüssel, E-Mail, Telegram, öffentliche Adresse, Update-Prüfung und Grafikchip;
  Antwort ersetzt den Settings-Cache, `getDashboard` wird invalidiert; Update-Prüfung invalidiert auch
  `getSession`; Google Books zusätzlich `getAudiobookSources`, `listMetadataProviders`. Geheimnisse laufen über
  `useTransientMutation` (nicht im Cache).
- `checkProviderKey`, `sendTestEmail`, `rotatePushKeys` (Bestätigung), `createIntegration`,
  `renewIntegrationToken` (Bestätigung), `deleteIntegration` (Bestätigung), `setIntegrationPathMap`.
- Bestätigung auch bei «Löschen …» jedes gespeicherten Werts. Update-Prüfung und Grafikchip speichern sofort.

### Fallen für den Umbau

1. Wiederverwendung im Einrichtungsassistenten: `ProviderCard`, `EmailForm`, `TelegramForm`, `shared` in
   `setup/ServicesStep.tsx`; `ServerBlock` in `setup/AccessStep.tsx`; `HardwareChoice` in
   `setup/PlaybackStep.tsx`; `PushDevices` in `ServicesStep`. Änderungen am Aussehen wirken dort mit.
2. Entwürfe der ganzen Seite in `useDrafts` (Zuklappen behält); bei Unterseiten je Dienst (Mockup) gilt die
   Abfrage vor dem Verlassen auch zwischen ihnen.
3. Einmalige Webhook-Adresse darf nie im Cache oder in einer Route stehen.
4. Konflikterkennung der Pfad-Umsetzung über `base` des Entwurfs.
5. Die Grafikchip-Wahl hat in der Verwaltung keinen eigenen Test (nur im Assistenten).
6. Zustand «Fehler» einer Integration (Dashboard) wird auf dieser Seite nicht gezeigt; Chip «abgewiesen» nur bei
   TMDB/TVDB-Schlüsseln, die der Server beim Speichern prüft.
7. `admin.integrations.intro` nennt keine Webhooks und keinen Grafikchip.

### Abgleich mit dem Mockup

| Zeile | Einstufung | Begründung |
|---|---|---|
| D054 Anbieter-Schlüssel | bestehende Funktion, Teil ohne Basis | sechs Anbieter, neuer Schlüssel, PIN/persönlicher Schlüssel, «Schlüssel speichern», «Verbindung testen» (= «Prüfen») bestehen. Schalter «Anbieter aktivieren»: **Beispiel ohne Basis** (ein Anbieter gilt mit Schlüssel; Fanart.tv schaltet man unter Metadaten ein). |
| D055 Radarr / Sonarr Webhooks | bestehende Funktion | anlegen, Pfad-Umsetzung (mehrere Zeilen), löschen bestehen. |
| D056 E-Mail / SMTP | bestehende Funktion | alle sechs Felder, «Speichern», «Testmail senden». |
| D057 Telegram | bestehende Funktion, Teil ohne Basis | Bot-Token und Speichern bestehen. «Chat-ID» und «Test senden» gehören zum Konto (Benachrichtigungen: Verknüpfung per Code, Testnachricht): hier **Beispiel ohne Basis**. |
| D058 Web Push | Beispiel ohne Basis (Teil bestehend) | Schalter «Web Push aktivieren», editierbarer «VAPID Public Key», «Speichern», «Konfiguration öffnen» gibt es nicht; das Schlüsselpaar entsteht automatisch. Bestehend: Anzeige des öffentlichen Schlüssels, «Schlüssel neu erzeugen» (fehlt im Mockup). |
| D059 Server & Updates | bestehende Funktion, Teil offene Frage | öffentliche Adresse und Update-Prüfung bestehen. «Jetzt nach Updates suchen» steht heute auf der Übersicht (D068): **offene Frage**, ob es an beiden Orten stehen soll. |
| Mockup-Seite: Kacheln je Dienst mit eigener Einstellungsseite | visuelle Umgruppierung | Inhalt gleich, Form neu; Zahlen wie «3 / 4 konfiguriert» sind Beispiele. |

### Im Mockup fehlt (muss bleiben)

Quelle-Chip (deploy/.env, Gespeichert, Nicht gesetzt) und env-Sperre; «endet auf …»; Chip «abgewiesen» und
Warnung beim Speichern; «Löschen …» mit Bestätigung je Wert; Link «Schlüssel bei … holen»; Webhook-Adresse
einmal mit Kopieren; «Token erneuern»; Meldungsprotokoll mit Ergebnis-Chips; letzte Meldung; Konflikt
«anderswo geändert»; E-Mail «Verwerfen», Testmail-Empfänger, Link zu den Benachrichtigungskanälen; Telegram
«Bot: @name»; «Schlüssel neu erzeugen»; Grafikchip-Wahl mit Gründen und Neustart-Hinweis; Abfrage vor dem
Verlassen.

---

## 5. Benachrichtigungen (NOT)

### Aufbau heute

`features/notifications/ChannelSettingsPage.tsx` (259 Zeilen) mit `channels/ChannelPanel.tsx`,
`TelegramLink.tsx`, `PushDevices.tsx`, `ChannelSummary.tsx`, `shared.tsx`, `queries.ts`, `push.ts`.

- `AdminPage` «Benachrichtigungskanäle» mit Hilfe, «Live, alle 3 Sekunden» bei offenem Telegram-Code.
- Hinweis «Nur im Hauptprofil» (Nebenprofil).
- Drei Bänder E-Mail, Telegram, Web-Push: Schalter «Aktiv», E-Mail-Adresse / Telegram-Verknüpfung /
  Geräteliste, Ereignisse (vier Kästchen), «Testnachricht», «Speichern».
- Das sind die Kanäle des **eigenen Kontos**, nicht des Servers (die stehen auf Integrationen).

### Unterschied der beiden Routen

| | `/admin/notifications` | `/settings/notifications` |
|---|---|---|
| Export | `AdminChannelSettingsPage` → `ChannelSettingsPage inAdmin` | `ChannelSettingsPage` |
| Wer | nur Administrator mit Hauptprofil; sonst `Navigate replace` zur Konto-Seite | jedes Profil |
| Rahmen | Verwaltungs-Leiste, kompakte Dichte | App-Spalte, Normaldichte |
| Kopfaktion | «Kanäle des Servers einrichten → Integrationen» | keine |
| Nebenprofil-Hinweis, Nur-lesen | nie erreichbar | ja |
| Einstiege | Leiste, E-Mail-Hilfe auf Integrationen | Eingang «Kanäle einstellen», Wiedergabe-Einstellungen |

### Abfragen und Mutationen

`getMyChannels` (3 s nur bei offenem Code und ohne Halt); `updateMyChannel`, `testMyChannel`,
`createTelegramLink`, `deleteTelegramLink` (Bestätigung), `getPushPublicKey` + `createPushSubscription`,
`deletePushSubscription`. Ereignis «Fehler» nur für Administratoren (Server entscheidet). Beim Abmelden
`leaveThisDevice` (`app/session.ts:160`).

### Fallen für den Umbau

1. Die Seite muss als Konto-Seite weiterleben; der Umbau betrifft nur die Verwaltungs-Route.
2. Abhängigkeit zu Integrationen: ein Kanal ohne Server-Einrichtung ist gesperrt; «Schlüssel neu erzeugen»
   beendet alle Push-Abos.
3. Ereignisse sind je Kanal gespeichert, nicht global (Mockup zeigt sie als eigene Gruppe).

### Abgleich mit dem Mockup

| Zeile | Einstufung | Begründung |
|---|---|---|
| D060 Zustellkanäle | bestehende Funktion | E-Mail, Telegram, Web-Push je mit «Aktiv», Speichern und Testnachricht; nur Hauptprofil. |
| D061 Benachrichtigungsarten | bestehende Funktion mit Beispiel ohne Basis | «Neue Medien» = «Neue Inhalte», «Fehler und Systemereignisse» ≈ «Fehler» (nur Admin). «Empfehlungen»: **Beispiel ohne Basis**. Es fehlen «Einladungen», «Ankündigungen»; Ereignisse gelten je Kanal: **offene Frage**, ob das Mockup sie global meint. |
| Mockup-Seite: Schalter «Push-Benachrichtigungen», «E-Mail-Benachrichtigungen» «Administrative Meldungen per E-Mail zustellen» | visuelle Umgruppierung | entspricht «Aktiv» je Kanal; der Text «administrative Meldungen» trifft nicht zu (gilt für alle Ereignisse des Kontos). |

### Im Mockup fehlt (muss bleiben)

E-Mail-Adresse je Kanal; Telegram-Verknüpfung mit Code, Ablaufzeit, Abfrage, «In Telegram öffnen», lösen /
verwerfen mit Bestätigung; Geräteliste mit «Dieses Gerät» und «Entfernen»; «Auf diesem Gerät aktivieren» mit
Fehlern; Hinweise zu Browser und iOS; Kanal nicht eingerichtet mit «Integrationen öffnen»; Kurzinhalt im
zugeklappten Band; Link «Kanäle des Servers einrichten»; Umleitung für Nicht-Admins; Konto-Route.

---

## 6. Liste «unklar»

1. Ob der Spatial-Fokus (Pfeile zwischen Kopf, Spalte, Inhalt, `AppLayout.tsx:94-102`) die Verwaltungs-Leiste
   als «sidenav» kennt; kein Test für `/admin`.
2. Ob ein Test die Hinweise «Freigaben werden neu gelesen», «Schlüssel abgewiesen», «Ordner-Browser
   blockiert», Grafikchip, Grafikchip-Fallback, inotify und Speicher > 90 % einzeln belegt (nicht gefunden;
   evtl. gemeinsam in «lists what needs doing…»).
3. Was bei «Neu starten» mit laufenden Jobs geschieht; der Dialog nennt nur Streams und Downloads.
4. Ob `Dashboard.streams` Radiohörer absichtlich auslässt (Neustart-Dialog, Pille «Streams»).
5. Warum der Integrationsstatus Fanart.tv und Google Books nicht kennt (Enum mit 8 Schlüsseln).
6. Testmail und Testnachricht nehmen den gespeicherten Stand; Verhalten bei offenem Entwurf nicht erklärt.
7. Welche Fehler das Ereignis «Fehler» auslöst.
8. Ob ausgeschalteter Zeitplan `update.check` auch `updateCheckEnabled` umschaltet (Text sagt «dieselbe
   Einstellung»; im Web nicht sichtbar).
9. `jobs.recentFailed` im Dashboard wird nirgends gelesen.
