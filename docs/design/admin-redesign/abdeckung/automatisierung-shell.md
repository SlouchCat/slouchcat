# Funktionsprüfung Teil «automatisierung-shell»

Geprüfter Stand: Commit `4251cf7a` auf `claude/admin-redesign`, verglichen mit `55b72c9a`. Nur gelesen,
nichts ausgeführt (kein Vitest, kein e2e). Zeilenliste: `automatisierung-shell.csv` (216 Zeilen,
Kürzel NAV, OVW, JOB, INT, NOT).

## Zählung

| Kürzel | ok | ok, ohne Test | LÜCKE | LÜCKE Test | unklar | Summe |
|---|---|---|---|---|---|---|
| NAV | 30 | 2 | 0 | 0 | 0 | 32 |
| OVW | 60 | 5 | 0 | 1 | 0 | 66 |
| JOB | 40 | 3 | 0 | 0 | 0 | 43 |
| INT | 45 | 5 | 1 | 0 | 0 | 51 |
| NOT | 23 | 1 | 0 | 0 | 0 | 24 |
| **Summe** | **198** | **16** | **1** | **1** | **0** | **216** |

Alle 16 «ok, ohne Test» hatten schon vor dem Umbau keinen Test für diese Aussage.

## LÜCKE, LÜCKE Test

### INT-051 (LÜCKE): Schlüssel-Links anderer Seiten landen auf der Kachelliste

Vor dem Umbau landete jeder Einstieg auf `/admin/integrations` am Seitenanfang. Dort stand das Band
«Schlüssel der Anbieter» als erstes und offen (`55b72c9a` `IntegrationsPage.tsx:120`). Heute zeigt
`/admin/integrations` ohne `?item=` nur die Kacheln. Bis zum Schlüssel braucht es also einen Klick
mehr. Das trifft diese Links:

- `web/src/features/admin/MetadataPage.tsx:908` («Schlüssel»)
- `web/src/features/admin/AudiobookSourcesPage.tsx:429` (Google Books)
- `web/src/features/admin/spotlight/FetchState.tsx:100` und `spotlight/SourceForm.tsx:195` (Trakt)

Übersicht, Server-Seite und Benachrichtigungen verlinken schon die Ansicht des Dienstes. Zu tun: an den
vier Stellen `serviceHref('keys')` aus `integrations/services.tsx` statt `paths.adminIntegrations`.
Die zugehörigen e2e-Erwartungen (z. B. `e2e/admin-spotlight.spec.ts:138`) müssen mitziehen. Die
Metadaten- und Spotlight-Seiten gehören zu anderen Prüfteilen. Die Lücke steht hier, weil
INT-051 die Einstiege der Integrationsseite aufführt.

### OVW-063 (LÜCKE Test): Leerzustand des Server-Protokolls ohne Test

Vorher prüfte `AdminHomePage.test.tsx` «when all is quiet…» (Stand `55b72c9a`, Zeilen 476-481)
diese Aussagen:

- «Server-Protokoll · keine Fehler»
- den verborgenen Satz «Keine Fehler seit dem Start.»
- die Live-Region, die vor dem ersten Fehler leer vorhanden ist

Der Test lebt in der neuen `AdminHomePage.test.tsx` weiter, prüft aber nur noch «keine Fehler» im
Kurzstatus der Übersicht. `ServerPage.test.tsx` hat keinen ruhigen Fall. Die Funktion selbst ist
vorhanden (`server/parts.tsx:1085-1121`). Zu tun: in `ServerPage.test.tsx` einen Fall mit leerem
Protokoll ergänzen, der diese drei Aussagen prüft.

## Funde ohne Inventarzeile

1. **e2e-Erwartung veraltet (wird rot).** `web/e2e/admin-server.spec.ts:46-51` erwartet für
   «Grafikchip, öffentliche Adresse und Update-Prüfung: Integrationen» `href` genau
   `/admin/integrations`. Seit `4251cf7a` verlinkt `ServerPage.tsx:108` aber
   `/admin/integrations?item=server`, und `ServerPage.test.tsx` erwartet das auch. `toHaveAttribute`
   mit Zeichenkette vergleicht genau, darum schlägt der Test fehl. Zu tun: die Erwartung auf
   `/admin/integrations?item=server` setzen.
2. **e2e A-3 leicht gelockert.** `e2e/admin-server.spec.ts:69` prüft den Fehler im Protokoll weiter
   binnen 10 s nach dem Auslösen. Danach wechselt der Test auf `/admin` und gibt «Trakt: Fehler»
   noch einmal 10 s ab dem Seitenwechsel. Vorher stand die Integrationskarte auf derselben Seite
   und musste binnen 10 s ab dem Auslösen stimmen (`55b72c9a` `admin-home.spec.ts:285`). A-3
   bleibt für das Protokoll erfüllt. Der Weg «Übersicht zeigt den Integrationsfehler» hat jetzt
   bis zu etwa 20 s. Das ist Absicht oder bewusst zu entscheiden.
3. **Integrationsseite ohne Dashboard: keine Meldung.** `IntegrationsPage.tsx:69` liest das
   Dashboard ohne eigenen Takt (die Leiste pollt alle 30 s). Antwortet es nicht, fehlen die Zustände
   an den Kacheln ohne Hinweis. Das ist so getestet und gewollt
   («the page works without the dashboard and shows no states»). Vorher zeigte die Übersicht in
   diesem Fall eine Fehlermeldung, die heute auf der Übersicht weiter steht.
4. **Hinweis «Integration mit Fehler» verlinkt die Kachelliste.** `server/parts.tsx:336`
   («Integrationen einrichten») führt auf `/admin/integrations`, nicht auf die Ansicht des
   betroffenen Dienstes. Der Grund steht an der Kachel, darum ist das kein Verlust. Ein Vorschlag:
   je Schlüssel die passende Ansicht (`keys`, `email`, `telegram`, `push`, `server`).
5. **Linktext beim Grafikchip.** Der Hinweis «Gewählter Grafikchip geht nicht» führt jetzt richtig
   auf `?item=server` (`server/parts.tsx:411`). Der Text heisst weiter «Zu den Schlüsseln». Das war
   schon vor dem Umbau falsch (Falle 2 des Inventars), ist also nicht schlechter geworden.
6. **Übersicht während eines Updates.** Bauplan 7.2: Die Übersicht zeigt «Update auf X läuft» mit
   Verweis, folgt der Installation aber nicht. Während der Server weg ist, zeigt sie deshalb den
   Fehlerzustand des Dashboards. Danach lädt sie nicht von selbst neu. Vorher tat die Übersicht
   beides, heute tut es die Server-Seite. Das ist laut Bauplan gewollt und steht nur der
   Vollständigkeit halber hier.
7. **Übersetzungen.** In `de.json` fallen nur `admin.nav.groups.*` weg (Bauplan 1). Geändert sind
   `system.gpu.choice.restartAdmin` («System › Server › Neu starten», Bauplan 7.2) und
   `libraryHint` («Pflege» statt «Wartung»). Kein Text des Bestands fehlt. In den anderen 19
   Sprachen fehlen je sechs Schlüssel, nämlich die Gruppen (gewollt).
8. In den geänderten Tests und Specs dieses Teils gibt es kein `.skip` und kein `.only`. Die
   Timeouts der verschobenen Tests sind unverändert. Entfernte `aria-`-Angaben gibt es nicht. Die
   Live-Regionen (Hinweiszahl `countLive`, Fehleransage, Installationszustand) bestehen weiter.

## Abgleich der Tests der alten Übersicht

### `AdminHomePage.test.tsx` (Stand `55b72c9a`)

Die alte Datei hat 51 `it(`/`it.each(`. Neu sind es 21 in `AdminHomePage.test.tsx` und 39 in
`ServerPage.test.tsx`.

- **Übersicht (15 alt):** Alle Namen sind wieder da. Drei sind umbenannt oder neu gefasst:
  - «shows eight cards…» heisst jetzt «shows the band, four key figures…» (Übersicht) und
    «shows the server, the updates with the restart…» (Server).
  - «when all is quiet: …no hints…» heisst jetzt «…all is well…».
  - «names a development build…» steht auf beiden Seiten.

  Auf der Server-Seite liegen diese Tests: «builds a sentence…», «builds the sentence of every
  kind…», «keeps the server log collapsed…», die fünf Neustart-Tests und
  ««Nach Updates suchen»…». «a failing dashboard request offers a retry» steht auf beiden Seiten.
  Inhaltlich ausgehöhlt ist nur der Leerzustand des Protokolls (OVW-063). Die Angaben der alten
  Karten «Integrationen» und «Webhooks» prüfen jetzt `IntegrationsPage.test.tsx` «shows the intro
  open below the title and a tile per service with its state» und `admin-webhooks.spec.ts`.
- **Update-Installation (26 alt):** Alle 26 stehen in `ServerPage.test.tsx`, auch alle `it.each`.
  Der Inhalt ist zeilengleich, nur die Route `/admin/server` und die Karte «Updates & Neustart»
  sind geändert.
- **Seitenleiste (7 alt):** Alle 7 sind da, angepasst an sechs Einträge:
  - «21 entries» heisst jetzt «7 entries».
  - «Enter follows a link» heisst jetzt «Enter follows a tab».
  - «the button «Verwaltung · page»» heisst jetzt «Verwaltung › area / page».

  Neu ist «puts the area’s pages as tabs above the page, with their numbers».
- **Update-Abzeichen (3 alt):** Alle 3 sind da. Neu ist «puts the badge on the tab «Server»…».
- **Fehlend:** keiner.

### `e2e/admin-home.spec.ts` (Stand `55b72c9a`)

Die alte Datei hat 9 Tests. Neu sind es 8 in `admin-home.spec.ts` und 6 in `admin-server.spec.ts`.

- `:80`, `:112`, `:470` bleiben (`:77`, `:109`, `:294`).
- `:132` (21 Einträge) wird `:129` (7 Einträge). `:179` (jede Seite über die Leiste) wird `:178`
  (jeder Bereich). Jede der 21 Seiten prüft `admin-shell.spec.ts:111` über die Bereichsreiter.
- `:198` (acht Karten) wird `admin-home.spec.ts:197` und `admin-server.spec.ts:34`.
- `:264` (A-3) wird `admin-server.spec.ts:69` (siehe Fund 2).
- `:306` (Tab, Neustart-Dialog) wird `admin-server.spec.ts:113` (Neustart bis «Abbrechen»,
  Protokoll) und `admin-home.spec.ts:257` (Kopf, Kennzahlen, «Serverdetails»).
- `:362` (Update-Karte) wird `admin-server.spec.ts:172`.
- Neu ist `admin-server.spec.ts:280` (Handy).
- **Fehlend:** keiner. Eine Erwartung ist veraltet (Fund 1).

## Geprüft wie verlangt

Jedes Element der alten Übersicht lebt auf der Übersicht oder auf der Server-Seite weiter:

- jede Phase (`server/parts.tsx:496-499`, sechs Phasen in drei Schritten) und jeder Ablehnungsgrund (zeilengleich
  übernommen)
- die 70-Minuten-Meldung (`:530`, `:846`)
- der Anschluss an eine laufende Installation (`:715-721`)
- die Neustart-Tafel (`ServerPage.tsx:262-268`)
- jeder Hinweis mit Befehl zum Kopieren (inotify, Compose)
- «Verstanden» (`server/parts.tsx:200-208`)
- das Einrichtungsband (`AdminHomePage.tsx:370-395`)
- die Ansage neuer Fehler (`server/parts.tsx:1115-1121`, nur Server-Seite, Bauplan 7.2)

Die beiden Speicherpfade stehen auf der Server-Seite und in der Kennzahl (F-4). Die Seitenleiste
pollt auf `/admin` und `/admin/server` nicht zusätzlich.

Stichproben der «unverändert»-Zeilen haben den Code an der Stelle bestätigt: NAV-004
(`index.html:15`), NAV-027 (`density.css:15`), INT-014 (`ProviderCard.tsx:228`), INT-024
(`WebhooksSection.tsx:519`), NOT-012 (`TelegramLink.tsx:114`). Die Bausteine der Ansichten
(ProviderCard, WebhooksSection, EmailForm, TelegramForm, PushBlock, ServerBlock, shared,
TelegramLink, PushDevices) sind seit `55b72c9a` unverändert.
