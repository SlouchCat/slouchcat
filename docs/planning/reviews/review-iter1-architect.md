# Architect-Review, Iteration 1 – Gesamturteil: tragfähig mit Änderungen

## 1. Steelman-Gegenargument (Antithese): TypeScript durchgehend (Option A)
Slouchcat hat zwingend Regel-Logik auf beiden Seiten der API: O-7 (Altersstufe offline), O-5 (Fortschritt-Merge), O-3 (Auto-Download-Regeln), Teile von W-7 laufen im Browser. Mit Go und TS existiert die K-3-Regel zweimal (Go/SQL im Server, TS im Offline-Client) – genau das verbieten Prinzip 1 und 3.4 Pkt. 2 („genau einmal"). Mit TS wäre die Regel ein geteiltes Modul, und Test 9.2 prüft online und offline dieselbe Funktion.
Überzeichnete Contras in 2.3: RAM (Engpass ist ffmpeg, nicht 60 vs. 120 MB Serverleerlauf; P-2 erfüllt auch Node); native Addons (sharp hat Binaries für x64/arm64; Go hat keinen reinen WebP/AVIF-Encoder, das Bildproblem besteht in beiden Optionen); Event-Loop (schwere Arbeit läuft in ffmpeg-Kindprozessen); dynamische Abfragen (E-4 mit Kysely typsicher; sqlc bei optionalen Filtern sperrig, `(@x IS NULL OR ...)` behindert Indizes); Agenten (eine Sprache, ein Testrunner, keine Codegen-Drift).
Grenze der Antithese: Go gewinnt bei Langzeitstabilität (npm-Churn real), Startzeit, Auslieferung als ein Binary – schwer für Driver 1.

## 2. Zielkonflikt-Spannungen
- **S1 Ein Container vs. Ressourcen-Isolation des Transcodings.** ffmpeg als Kindprozess isoliert Abstürze, nicht CPU/RAM/GPU-Sessions. Auf dem N100 verdrängt ein Software-4K-Transcode die API (Startseite < 300 ms p95). OOM-Killer kann den Go-Prozess treffen. Ab M5 konkurrieren Download-Jobs mit Live-Streams um die GPU. Plan löst das nicht (keine Admission Control, keine Prioritäten).
- **S2 Prinzip 1 vs. O-7 offline.** Offline muss der Client durchsetzen. Der „signierte Profil-Snapshot" (M5 Schritt 3) schützt nicht (Client prüft selbst). Reales Bedrohungsmodell: Kind am geteilten Gerät → PIN offline, nach Profil getrennte Liste bei gemeinsamem OPFS. Der Plan verdeckt den Konflikt statt ihn als Ausnahme zu benennen.
- **S3 SQLite mit einem Schreiber vs. Job-Queue in derselben Datei.** Für einen Haushalt ausreichend, aber: langer Scan-Batch verzögert Heartbeats, abgelaufene Leases führen zu doppelter Ausführung. Fehlt: Lease-Dauer deutlich grösser als längste Transaktion, idempotente Jobs.
- **S4 OPFS-Offline vs. iOS.** Spike misst nur Löschung durch das System. Ungeprüft: kein Background Fetch in iOS-Safari, Abbruch bei Bildschirmsperre, kein Netzwerktyp für „nur WLAN"; Wiedergabe mit Spulen einer 1.5-GB-Datei aus OPFS. O-3 ist auf iOS strukturell nur bei offener App erfüllbar.

## 3. Synthese-Pfad
- Go-Server behalten (Driver 1). Stärke der Antithese übernehmen: Der Client kennt keine Regeln, sondern speichert Entscheide des Servers zwischen. Jeder Download-Datensatz trägt eine serverseitig aus `visibility` berechnete Liste `allowed_profile_ids`, bei jedem Sync aktualisiert. Offline prüft der Client nur Mitgliedschaft plus lokalen PIN-Verifier. Prinzip 1 neu: „Entscheid im Server, offline nur Nachschlagen."
- Fortschritt-Merge (O-5) im Server: Client lädt Roh-Ereignisse hoch, Server entscheidet.
- Rückfall-Kriterium auf Option A in M1 messen (Scanner, ffmpeg-Supervisor, Seeking), nicht in M0.
- sqlc nur für statische Abfragen; Listen mit Scope und Suche über kleinen Query-Builder, gekapselt in `visibility`, der den Scope-Join selbst einfügt.

## 4. Befunde
1. **hoch – 3.4 Pkt. 1/2, 9.2 – K-3 nur für Listen abgesichert.** „Pflichtparameter Scope in sqlc" ist nicht durchsetzbar. Ungeprüft: Personenseiten/Filmografien (E-4), verschachtelte Arrays (Reihen der Startseite) und Zähler, HLS-/Download-/Untertitel-Endpunkte per ID, Benachrichtigungen (E-8, I-3: neue FSK-18-Folge an Kinderprofil), SSE/WebSocket-Ereignisse, Trakt-Reihen. Vorschlag: (a) View/CTE `visible_items(profile_id, item_id)`, nur von `visibility` erzeugt; `go/analysis`-Regel verbietet Zugriff auf `items`/`media_files` ausserhalb `visibility`/`admin`. (b) Test 9.2 auf alle Operationen, mit Schema-Walk für verschachtelte Arrays und Binär-Endpunkte. (c) `notify` filtert Empfänger über `visibility`.
2. **hoch – 3.2, kein Datenmodell-Abschnitt – Identität bei Umbenennung (B-4) und verlorenem Mount.** „Umbenannt" als Löschen+Neu verliert W-5, E-5, Favoriten, Top Picks, B-6-Zuordnungen; nicht gemountetes Share sieht aus wie „alles gelöscht". Vorschlag: generische Tabelle `items` (`kind`, `parent_id`, vererbte Altersstufe; hält Musik/Fotos offen); Fortschritt/Merkliste am Item, nie an der Datei; Datei-Identität über (Gerät, Inode)+Grösse+Teil-Hash; Soft-Delete mit 24 h Karenz; Scan bricht ab, wenn Wurzel fehlt oder > 20 % Dateien verschwinden.
3. **hoch – 3.2 Schritt 6, M1 Schritt 4 – Seeking bei Remux (W-3).** Vorab erzeugte 4-s-Playlist funktioniert nur beim Transcode mit `-force_key_frames`; bei `-c:v copy` liegen Segmentgrenzen auf Quell-Keyframes. Vorschlag: Transcode: Playlist vorab, bei Neustart `-start_number`/`-output_ts_offset`; Remux: Keyframe-Index in `probe.file` speichern, Segmentliste daraus; HEVC-Remux für Safari mit Tag `hvc1`.
4. **hoch – M4 Schritt 2, P-6 – W-3 mit HDR nur mit Herunterskalieren.** Software-Tonemapping in 4K nicht echtzeitfähig. Vorschlag: Handy-Profil skaliert vor Tonemapping auf ≤ 1080p; Pipeline je Backend (`tonemap_cuda`/libplacebo, `vpp_qsv`, `tonemap_vaapi`); Software-Fallback als Ausnahme von W-3 dokumentieren; HDR-Datei schon in M1 auf dem Referenz-NAS messen (M1-DoD misst nur Entwicklungsrechner).
5. **hoch – M1 Schritt 5, 8, O-3 – iOS-Spike misst nur Löschung.** Erweitern um: (a) 1.5-GB-Download bei Bildschirmsperre/App-Wechsel mit Wiederaufnahme; (b) Schreibweg `createSyncAccessHandle` im Worker vs. `createWritable` je iOS-Version; (c) Spulen aus OPFS via Blob-URL und via Range-Antworten des Service Workers; (d) RAM. Auto-Download im Hintergrund und „nur WLAN" auf iOS als dokumentierte Abweichung analog AN-4.
6. **mittel – 3.1 transcode/jobs – keine Admission Control (S1).** Kapazitätsbudget je Backend (N100: 2× 4K-Tonemap); Priorität Live vor Download-Vorbereitung, Download-Jobs pausierbar; ffmpeg mit `nice 10`, `oom_score_adj` erhöhen; W-8-Limit genau hier umsetzen.
7. **mittel – M8 vs. Prinzip 4 – Sicherung vor Migration zu spät.** `VACUUM INTO` vor jedem goose-Lauf schon in M0; eingefrorene DB-Stände ab M1 in CI (heute erst M10).
8. **mittel – 4 – goose und SQLite.** goose kapselt Migrationen in Transaktionen; dort ist `PRAGMA foreign_keys=OFF` wirkungslos. Vorlage mit `-- +goose NO TRANSACTION`, FK explizit aus/an, `PRAGMA foreign_key_check`; Regel in CLAUDE.md.
9. **mittel – 4 Jobs – Umfang der eigenen Queue unbestimmt.** Nötig: Lease mit Ablauf, Idempotenz-Schlüssel, Debounce (Watcher-Stürme), Retry mit Backoff, Priorität, Abbruch, Zeitpläne. Umfang in M0 festschreiben oder bestehende SQLite-Queue (z. B. goqite) prüfen; ADR.
10. **mittel – 3.2 Schritt 1, P-4 – B-4 auf NFS/SMB nicht garantiert.** Verzeichnis-mtime ändert sich nur bei direkten Kindern → alle 45 s jedes Verzeichnis per `stat`; NFS `actimeo` bis 60 s; `max_user_watches` ist Host-Einstellung. Vorschlag: Assistent erkennt Dateisystemtyp (`statfs`) und Watch-Limit, warnt; auf Netzfreigaben Webhooks primär, Polling als Netz; P-4 für NFS/SMB vom Auftraggeber präzisieren lassen.
11. **mittel – 3.1 Bildspeicher – Bildverarbeitung ohne CGO.** Go hat keinen reinen WebP/AVIF-Encoder; `x/image/draw` auf ARM langsam. Vorschlag: ffmpeg übernimmt im Job `image.fetch` Skalierung und WebP; Hash-URLs mit `immutable`.
12. **mittel – fehlt – Secrets für ausgehende Verbindungen.** TMDB/Trakt/YouTube/Arr/Seerr-Schlüssel, SMTP-Passwort, Telegram-Token, VAPID-Key müssen im Klartext nutzbar sein. Verschlüsselung at rest mit `/config/secret.key` ausserhalb der DB; in OpenAPI `writeOnly`; Sicherung (A-4) wahlweise ohne Schlüssel; Env-Variablen als Alternative.
13. **mittel – AN-6 vs. A-5 und 3.3 – privates Repository.** ghcr.io-Images und Releases dann nicht anonym abrufbar; A-5 scheitert; NAS braucht Token. Paket/Release öffentlich; Go per `GOARCH` cross-kompilieren, nur apt-Stufe emuliert.
14. **mittel – 2.3 – Rückfall-Kriterium nicht messbar.** Schätzung je Meilenstein in Stunden; Kriterium nach M1 verlegen.
15. **mittel – M1 – Durchstich überladen.** Vorschlag: M1a Scan, Direct Play, einfache Startseite, iOS-Spike sofort starten; M1b HLS, Seeking, Design-Feinschliff; Storybook und arm64-Build nach M1b.
16. **niedrig – AN-1, 3.2 Schritt 4 – Freigaben-Abbildung unvollständig.** US-TV-Ratings (TV-Y…TV-MA) und BBFC fehlen; R→16 pauschal; Vererbung Serie→Staffel→Folge undefiniert. Reihenfolge DE → AT/CH → GB → US; B-9 erhält Massenzuweisung.
17. **niedrig – 5.1, 8 – Rate-Limits nur als Risiko.** Seerr-Treffer brauchen je Titel TMDB `release_dates`. Gemeinsamer Baustein `ratelimit` in 3.1, Freigaben-Cache 7 Tage.
18. **niedrig – Prinzip 3 – Streaming passt nicht in oapi-codegen strict.** Range-, HLS-, Bild-Endpunkte im Vertrag beschreiben (C-5), aber als `http.Handler` mit `http.ServeContent`; Ausnahme in CLAUDE.md.
19. **niedrig – B-3 – TVDB-Lizenz.** TVDB v4 verlangt Projekt-Lizenz, teils Abo-PIN. Vor M3 klären.
20. **niedrig – 3.3 – `/config` auf Netzlaufwerk.** SQLite WAL auf NFS/SMB beschädigt die DB. Beim Start `statfs` prüfen, klar abbrechen.

Annahmen: AN-2, AN-3, AN-5 tragfähig. AN-4 ehrlich und wegen O-6 unvermeidbar. AN-1 richtig, wenn Befund 16 umgesetzt. AN-6 widerspricht A-5 (Befund 13).

## 5. Prinzipienverletzungen
- Prinzip 1 (mittel): M5 Schritt 3 setzt O-7 im Client durch ohne Ausnahme; Signatur täuscht Sicherheit vor. Lösung: Synthese-Pfad (`allowed_profile_ids`).
- Prinzip 1 / 3.4 Pkt. 1 (niedrig): „Keine ungefilterte Titelabfrage ausserhalb admin" unrealistisch; `library`, `metadata`, `jobs`, `notify`, Entzugsliste in `offline` brauchen Zugriff ohne Filter → expliziter `SystemScope` mit Lint-Allowlist.
- Prinzip 4 (mittel): Sicherung vor Migrationen erst in M8 (Befund 7).
- Prinzip 3 (niedrig): Binär-/Streaming-Endpunkte nicht mit Strict-Server umsetzbar; Ausnahme fehlt (Befund 18).
- Prinzip 2: eingehalten.

## 6. Gesamturteil
Tragfähig mit Änderungen. Der Go-Stack hält, sofern vor M1 im Plan stehen: Datenmodell mit stabiler Identität, Durchsetzung von K-3 als View mit Lint statt nur über Listen, Admission Control für Transcoding, Seeking-Modell für Remux, erweiterter iOS-Spike.
