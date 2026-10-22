# ADR 0010: Webhook-Empfang – Autorisierung ohne Sitzung, ein Auslöser statt eines zweiten Scan-Wegs

- Status: angenommen (22. Oktober 2026)
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitte 3.1 (Module `integrations`, `ratelimit`), 3.4, 3.6,
  3.8 Punkte 2, 4 und 8, Entscheid 5.6 und M9; Frage 10 in `docs/planning/04-entscheidungen.md`;
  Spec I-8, AN-12, A-3, A-9
- Betroffene Spec-IDs: I-8, A-3, A-9, B-4, AN-12

## Kontext

M9 ist auf den eingehenden Webhook zusammengeschnitten (Frage 10): Radarr und Sonarr melden geänderte Dateien,
damit neue Titel auf Netzfreigaben nicht auf den nächsten Abgleich warten müssen (AN-12). Entscheid 5.6 legt
Adresse, Token, Ereignisse, Pfad-Umsetzung, Protokoll und Dashboard-Kachel fest. Drei Fragen blieben beim Bauen
offen, weil sie Regeln berühren, die sonst überall gelten:

1. Jede zustandsändernde Anfrage braucht den **CSRF-Header** (3.8 Punkt 7). Radarr und Sonarr senden ihn nicht.
2. Der Endpunkt trägt **keine Sitzung**, obwohl jede Operation ein `x-visibility` braucht (Regel 7).
3. Eine Meldung muss einen Scan auslösen, ohne dass **ein zweiter Scan-Weg** neben dem Watcher entsteht (3.4).

## Entscheid

| Punkt | Entscheid |
|---|---|
| CSRF | Die Route `POST {BasePath}/webhooks/{token}` ist von der Header-Pflicht ausgenommen (`csrfExemptRoutes` in `internal/api/webhook_handlers.go`, im Vertrag beschrieben). Der Header schützt davor, dass ein **Browser** eine Anfrage mit dem mitlaufenden Sitzungs-Cookie stellt. Der Handler dieser Route sieht keine Sitzung an: Seine einzige Autorisierung ist das Geheimnis im Pfad, und `SameSite=Lax` hält das Cookie ohnehin von einer fremdseitigen POST-Anfrage fern. (Die Middleware liest das Cookie wie bei jeder Operation — das ist ihr Verhalten, nicht das des Handlers.) Wer das Geheimnis nicht hat, erreicht nichts; wer es hat, braucht keinen Umweg über einen fremden Browser. Der Header würde hier also nichts schützen und den Endpunkt für seinen einzigen Zweck unbrauchbar machen. Die Ausnahme ist eine Liste, kein Schalter: jede weitere Route müsste einzeln eingetragen werden, und spätestens bei der zweiten gehört die Begründung neu geprüft |
| Sichtbarkeit | `x-visibility: public`, weil die Anfrage keine Sitzung trägt — nicht, weil sie offen stünde. Das Token ist ein 256-Bit-Zufallswert, gespeichert als SHA-256 (3.8 Punkt 2), und ein unbekanntes Token wird mit 401 beantwortet, **ohne** Protokolleintrag: Es gibt keine Integration, der man ihn zuschreiben könnte, und ein Protokoll über fremde Versuche wäre ein Kanal, den ein Fremder füllen kann |
| Token im Log | Der Pfadbestandteil wird in jedem String redigiert (`webhookPath` in `internal/logging`), auch in Fehlermeldungen und in der `instance` eines Problem-Dokuments. Der gemeldete Dateipfad bleibt lesbar — er ist die Information, die beim Suchen eines Fehlers zählt (A-9) |
| Wirkung | Eine Meldung erzeugt genau einen `scan.path`-Auftrag über `library.RequestPathScan` — denselben, den der inotify-Watcher erzeugt, mit derselben Zusammenfassung über die Debounce-Regel der Queue (3.6). Kein zweiter Scan-Weg, nur ein zweiter Auslöser; auch Löschungen laufen über denselben Pfad-Scan |
| Paketgrenze | `integrations` greift nie auf `items`, `media_files` oder `tracks` zu (Regel 2, `depguard` setzt es durch). Es liest die Bibliothekswurzeln über `store/appq` und löst Pfade über `paths` auf; alles Weitere ist Sache von `library` |
| Strikt nach Typ | Der Pfad kommt bei einer Radarr-Integration aus `movie.folderPath`, bei einer Sonarr-Integration aus `series.path`. Fehlt er, antwortet der Server 422 und protokolliert `invalid_payload`, statt still auf das andere Feld auszuweichen: Eine an der falschen Stelle eingetragene Adresse soll auffallen, nicht halb funktionieren |
| Ratenbegrenzung | 10 Anfragen je Sekunde und Integration (3.8 Punkt 8) über `ratelimit.Allow`, das nicht blockierende Gegenstück zu `Wait`: Eine eingehende Anfrage wird mit 429 und `Retry-After: 1` abgewiesen, statt gehalten zu werden. Ein Import einer ganzen Staffel feuert mehrere Meldungen auf einmal, darum ist der Burst so gross wie die Rate |
| Protokoll | Die letzten 20 Meldungen je Integration, durchgesetzt von einem Trigger (Migration 00017). Das Protokoll ist ein Fenster, keine Geschichte: Es beantwortet «kommt etwas an und was passiert damit», und dafür reichen 20 Zeilen, die nie wachsen |

Verworfen: (a) Token im `Authorization`-Header statt im Pfad — Radarr und Sonarr erlauben eigene Header erst in
neueren Versionen, und der Pfad ist das, was der Admin ohnehin kopiert. (b) Ein gemeinsamer Webhook mit einer
Kennung im Rumpf — ein Token je Integration lässt sich einzeln erneuern und einzeln begrenzen. (c) Ein eigener
Auftragstyp `webhook.scan` — er müsste die Zusammenfassung, den Abbruch und den Verlustschutz von `scan.path`
nachbauen. (d) Unbekannte Ereignisse mit 4xx ablehnen — in Radarr kreuzt man die Ereignisse grosszügig an; eine
Ablehnung würde dort als Fehler erscheinen, obwohl nichts falsch ist.

## Annahmen, die beim Bauen sichtbar wurden

- **Pfade sind absolut und in der Schreibweise dieses Servers.** Umsetzung und Auflösung rechnen mit
  Schrägstrichen; Slouchcat läuft laut Spec ausschliesslich als Docker-Stack auf Linux. Ein Radarr auf Windows
  würde `C:\Media\Filme\…` melden, und dafür lässt sich keine Umsetzung eintragen — die Quelle müsste
  ebenfalls absolut im Linux-Sinn sein. Solche Meldungen enden mit 422 `path_outside_roots`, dessen Text diese
  Annahme nennt. Unterstützt wird der Fall bewusst nicht: Er kommt in einem Linux-Haushaltsstack nicht vor, und
  eine zweite Pfadschreibweise zöge sich durch Umsetzung, Auflösung und Prüfung. Die `Test`-Nutzlast beider
  Dienste trägt übrigens den fest eingebauten Unsinn-Pfad `C:\testpath` — ein Grund mehr, dass `Test` nie
  einen Scan auslöst.
- **Ein relativer gemeldeter Pfad liegt unter nichts**, auch nicht unter der Quelle `/`. Sonst überlebten seine
  `..`-Teile die Umsetzung und `path.Join` zielte auf ein anderes Verzeichnis als gemeint. Beide Seiten eines
  Vergleichs müssen absolut sein (`under` in `internal/integrations/pathmap.go`).
- **Die Zahl der Dashboard-Kachel zählt Protokollzeilen**, und davon hält jede Integration 20. Die Kachel ist
  eine Lebenszeichen-Anzeige, keine Buchhaltung; der Text sagt darum «im Protokoll der letzten 24 Stunden».

## Folgen

- Migration 00017 legt `webhook_integrations` und `webhook_events` an; Migration 00018 entfernt den
  Benachrichtigungskanal `acquisition`, dessen Erzeuger I-1 entfallen ist (Nachtrag in ADR 0009).
- Der Vertrag erhält sieben Operationen: den Empfänger und sechs Admin-Operationen für Liste, Anlegen,
  Token erneuern, Pfad-Umsetzung, Löschen und Protokoll.
- Die Dashboard-Kachel «Webhooks» hält A-3 «Integrationsstatus» gefüllt, nachdem die Arr-Clients aus I-1
  weggefallen sind.
- Die CSRF-Ausnahme ist die erste ihrer Art. Wächst die Liste, gehört sie in eine eigene Betrachtung;
  solange sie eine Zeile hat, ist sie im Code und hier beschrieben.
