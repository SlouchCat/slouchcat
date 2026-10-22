# Plan: Slouchcat Version 1

Status: Freigegeben von Marco am 18. Oktober 2026 (Konsens, Iteration 2 final)

- Modus: RALPLAN-DR, SHORT
- Quelle der Anforderungen: `.omc/specs/deep-interview-slouchcat-vision.md` (im Folgenden "Spec"). Die Spec bleibt massgeblich; dieser Plan zitiert ihre IDs.
- Grundlage Iteration 2: `.omc/drafts/review-iter1-critic.md` (REVISE), `.omc/drafts/review-iter1-architect.md` (tragfähig mit Änderungen); Verbesserungsdurchgang nach Konsens: `.omc/drafts/review-iter2-critic.md` (APPROVED), `.omc/drafts/review-iter2-architect.md` (tragfähig, keine volle Iteration nötig); Umsetzung je Befund im Changelog (Abschnitt 14).
- Stand: 18. Oktober 2026. Sprache: Deutsch (Schweizer Schreibweise), technische Begriffe und Bezeichner Englisch.

---

## 1. Zusammenfassung der Anforderungen

Slouchcat ist ein eigenständiger, selbst gehosteter Media-Server für einen Haushalt plus eingeladene Gäste (Spec "Goal", "Constraints"), ohne Jellyfin-Unterbau (Vision, "Warum eigenständig").

| Baustein | Spec-IDs | Kern in einem Satz |
|---|---|---|
| Konten, Profile, Zugriff | K-1 … K-8 | Hauptbenutzer per E-Mail, Profile mit FSK-Stufe, PIN, Bibliotheksrechten, Schauzeit-Limit; Gäste per Einladung; Remote via HTTPS |
| Video-Bibliothek, Metadaten | B-1 … B-9 | Filme/Serien aus read-only Ordnern, TMDB/TVDB de/en, Änderungen < 60 s sichtbar, Versionen, Extras, YouTube-Trailer, Pflegewerkzeuge |
| Hörbücher, Podcasts | H-1 … H-4 | Kapitel, Feeds, Fortschritt je Profil, gleiche Profil-/Offline-Regeln wie Video |
| Wiedergabe, Transcoding | W-1 … W-10 | Direct Play zuerst, sonst HLS mit GPU und Tonemapping, 4K-HEVC-Start < 5 s, Spurwechsel, Qualitätsregeln, Geräteprofile |
| Entdecken, Kuratierung | E-1 … E-9 | Karussell "Cat's Empfehlungen", max. 8 Reihen, Suche ab 2. Zeichen, Trakt-Listen nur aus eigener Bibliothek, Favoriten-Benachrichtigung |
| Offline | O-1 … O-7 | Downloads im Browser-Speicher, Staffeln, Auto-Downloads, Flugmodus, kein Ablauf, Profilregeln offline |
| Clients, Design | C-1 … C-5 | Eine PWA, Design-System mit Tokens, de/en, Tastatur-/Fernbedienungs-Navigation, OpenAPI |
| Administration | A-1 … A-9 | Assistent, zentrale Verwaltung, Dashboard, Sicherung, Update-Hinweis, Kanäle, Statistiken, Ankündigungen, keine Secrets in Logs |
| Integrationen | I-5, I-7, I-8 | Radarr/Sonarr-Webhooks für schnelles Einlesen, Bazarr-Untertitel, Trakt öffentlich |

Harte Rahmenbedingungen (Spec "Constraints"): nur Docker Compose auf Linux, GPU optional; genau ein Client (PWA); Medientypen Filme, Serien, Hörbücher, Podcasts; Solo mit Claude Code und oh-my-claudecode, keine Deadline; erster Meilenstein vertikaler Durchstich "innerhalb der ersten Wochen"; Stack nach "sehr stabil, läuft überall gut, performant" mit Klartext-Begründung.

Nicht eingeplant (Spec "Non-Goals"): Musik, Fotos, Live-TV/DVR, TV-Apps, native Apps, Plugin-System, algorithmische Empfehlungen, Profil-Interaktion, Assistent/Zufall/Trailer-Reel/Themenlisten, Trakt-Login/Scrobbling, Freigabe-Anfragen, eigene Wunschverwaltung und Seerr-Bereich zum Wünschen, Beschaffungsstatus am Titel, SABnzbd-Fortschritt, Steuerung von Radarr/Sonarr/SABnzbd/Seerr, eigene Untertitel-Beschaffung, Mandanten, Cloud/native Pakete/Windows/macOS-Server, YouTube-Downloads.

---

## 2. RALPLAN-DR-Summary

### 2.1 Prinzipien

1. **Entscheid im Server, offline nur Nachschlagen.** Jede Sichtbarkeits-, Alters- und Rechteentscheidung fällt serverseitig in `visibility` (K-3, K-8, E-3, H-4). Offline speichert der Client nur Entscheide des Servers (`allowed_profile_ids` je Download) und schlägt nach (O-7); er kennt keine eigene Altersregel. Zwei deklarierte Ausnahmen, beide nur offline und beide mit serverseitig gelieferten Werten: lokale PIN-Prüfung (3.3, AN-19) und Durchsetzung des Schauzeit-Tagesrests (3.3, AN-18).
2. **Ein Container, wenig bewegliche Teile.** Ein Prozess, eine eingebettete Datenbank, keine Pflicht-Zusatzdienste; Last wird im Prozess über Admission Control geregelt, nicht über mehr Container (Spec "Constraints").
3. **Vertrag vor Code.** `api/openapi.yaml` ist die Quelle für Server-Schnittstellen und Web-Client (C-5); generierter Code wird nie von Hand geändert. Ausnahme: Binär- und Streaming-Endpunkte (Range, HLS, Bilder) sind im Vertrag beschrieben, aber als `http.Handler` umgesetzt.
4. **Jeder Meilenstein ist benutzbar und rückholbar.** Ab M1a läuft ein Stack, der etwas Echtes abspielt; ab M0 sichert jede Migration vorher die Datenbank. Klartext: "Zurückholen" heisst, die automatische Sicherung von vor dem Update wiederherzustellen; was seit dem Update geändert wurde (Fortschritte, neue Profile, Top Picks), geht dabei verloren.
5. **Übersichtlichkeit vor Effekt.** Jede Farbe und jeder Text kommt aus Tokens bzw. Sprachdateien (C-2, C-3); jede neue Seite ist ab dem ersten Tag per Tastatur bedienbar (C-4).

### 2.2 Decision Drivers (Top 3, gewichtet)

1. **Betriebsstabilität auf schwacher Hardware (Gewicht 40 %):** Leerlauf-RSS < 150 MB, Container-Start < 5 s, amd64 + arm64, keine Zusatzdienste, Langzeit-Kompatibilität ohne erzwungene Umbauten.
2. **Zuverlässige Prozess- und Streamingsteuerung (35 %):** viele gleichzeitige ffmpeg-Prozesse, Range, HLS, Neustart beim Spulen, Admission Control (W-2, W-3, W-6, W-8).
3. **Solo-Wartbarkeit mit Agenten (25 %):** eine Regel an einer Stelle, wenig Abhängigkeits-Churn, gut bekannte Werkzeuge für Claude-Code-Agenten.

Begründung der Gewichte: Driver 1 ist das ausdrückliche Kriterium des Auftraggebers (Runde 15); Driver 2 ist das grösste technische Risiko (Abschnitt 9); Driver 3 betrifft die Entwicklung, nicht den Betrieb.

### 2.3 Viable Options (Stack)

| | Option A: TypeScript durchgehend | Option B: Go-Server + TypeScript-Web | Option C: .NET-Server + TypeScript-Web |
|---|---|---|---|
| Server | Node.js (aktive LTS, heute 24) + Fastify, Kysely | Go (eine der zwei neuesten Versionen) + `net/http` | ASP.NET Core (aktuelle LTS) |
| Pro | Eine Sprache, ein Testrunner, keine Codegen-Drift zwischen Server und Client; Kysely macht dynamische Abfragen (E-4) typsicher; Agenten sehr sicher | Ein statisch gelinktes Binary (~25 MB), Start < 1 s; Go-1-Kompatibilitätsversprechen seit 2012; Goroutines und `context` für ffmpeg-Supervision; Cross-Compile per `GOARCH` | Sehr performant; reife Bibliotheken (EF Core, Migrationen); Domäne durch Jellyfin bewiesen |
| Contra | Node-LTS-Wechsel alle ~30 Monate und Server-Abhängigkeiten aus npm erhöhen den Wartungsaufwand über Jahre; Laufzeit + `node_modules` ~+80–120 MB im Image | Zwei Sprachen und ein Codegen-Schritt je API-Änderung; dynamische Abfragen brauchen einen eigenen Query-Builder; kein reiner Go-Encoder für WebP/AVIF (gelöst über ffmpeg) | Runtime ~+80–100 MB im Image; LTS-Wechsel alle 2–3 Jahre; Nähe zu Jellyfin verleitet zum Nachbau fremder Muster |

Korrekturen gegenüber Iteration 1: Der Web-Teil (React, Vite, pnpm) hat in allen drei Optionen denselben npm-Churn; das Argument zählt nur für den Server-Teil. `sharp` liefert Binaries für x64 und arm64 und ist kein Multi-Arch-Problem. Schwere Arbeit läuft in allen Optionen in ffmpeg-Kindprozessen, der Event-Loop ist kein Argument. Der Image-Vergleich wird von ffmpeg und Treibern dominiert (~350–450 MB in jeder Option); der Unterschied liegt bei 80–120 MB.

**Abwägung nach Drivern** (Punkte 1–5, gewichtet):

| Driver | A | B | C |
|---|---|---|---|
| 1 Betrieb (40 %) | 3 | 5 | 4 |
| 2 Prozesse/Streaming (35 %) | 4 | 5 | 5 |
| 3 Solo mit Agenten (25 %) | 5 | 4 | 3 |
| **Summe** | **3.85** | **4.75** | **4.10** |

**Sensitivität:** Die Punkte für A bei Driver 1 sind streng; ehrlicher wäre 4 (der Unterschied beträgt nur 80–120 MB Image, P-2 erreicht A auch). Mit A = (4, 4, 5) ergibt sich 4.25, mit A = (4, 5, 5) 4.60; B bleibt mit 4.75 vorne. A gewinnt nur, wenn A und B bei Driver 1 und 2 gleich bewertet werden (5, 5, 5 → 5.00). Das Ergebnis ist damit robust, der Abstand aber kleiner, als die Haupttabelle zeigt.

Driver 3 spricht für A: eine Sprache, eine Regelstelle, keine Drift. B verliert hier einen Punkt; der Nachteil wird begrenzt, weil der Client nach Prinzip 1 keine Regeln mehr implementiert (die Architect-Antithese "K-3 existiert zweimal" entfällt) und CI die Codegen-Drift prüft.

**Invalidiert:** Rust (Entwicklungstempo solo zu tief für 68 Kriterien), Python (Leistung bei parallelem Streaming/Scan, Packaging auf arm64).

**Empfehlung: Option B.** Rückfall auf Option A siehe 2.5.

### 2.4 Warum Go und nicht TypeScript oder .NET (Klartext)

Der Server ist ein einziges, in sich geschlossenes Programm, das auf einem kleinen Heimserver schnell startet und wenig Speicher braucht. Programme in dieser Sprache laufen erfahrungsgemäss viele Jahre ohne Umbau weiter, weil ihre Hersteller das zusichern. Die beiden Alternativen wären auch machbar, verlangen aber häufigere Pflichtwechsel der Grundlage und brauchen etwas mehr Platz und Speicher.

### 2.5 Rückfallkriterium auf Option A

Gemessen am Ende von M1b (Scanner, ffmpeg-Supervisor und Seeking sind dann gebaut). Gemessen wird nur, was die Sprachwahl betrifft: Jede Aufgabe wird im Aufgabenprotokoll des Repos mit `server` oder `web` getaggt und mit Stunden erfasst; Web, Design, iOS-Spike, Hardware und Gerätetests wären in Option A gleich teuer und zählen nicht.

- **Signal 1 (Aufwand):** Der mit `server` getaggte Aufwand von M0 + M1a + M1b übersteigt **26 Arbeitstage** (Schätzanteil Server 13–20 Tage, Schwelle = 1.3 × 20, passend zur Warnschwelle in Abschnitt 9).
- **Signal 2 (Ursache):** Mehr als 30 % der `server`-Aufgaben brauchen mehr als zwei Korrekturrunden aus Go-spezifischen Gründen. Die Einstufung macht der Code-Review-Agent je Aufgabe nach fester Checkliste: *Go-spezifisch* sind Fehler durch Nebenläufigkeit (Goroutine-Leck, Race, `context`-Abbruch), Grenzen des Typsystems oder der Generics, fehlende Bibliothek mit nötigem Eigenbau, Einschränkungen von sqlc oder oapi-codegen; *nicht* Go-spezifisch sind Fachlogik, ffmpeg-Argumente, Browser- und Safari-Verhalten, Spec-Unklarheiten.

Trifft ein Signal zu, baut ein Agent einen Prototyp in Option A mit **festem Umfang**: Scanner für einen Film-Ordner plus HLS-Transcode mit Seeking (Timebox 5 Arbeitstage). Verglichen wird mit dem protokollierten Go-Aufwand für genau diesen Umfang. Ein Wechsel wird dem Auftraggeber nur empfohlen, wenn der Prototyp den Umfang mit höchstens 60 % dieses Aufwands erreicht und die M1b-Tests für diesen Umfang besteht.

---

## 3. Architektur

### 3.1 Komponenten

Ein Server-Prozess `slouchcat` (Go), der auch die gebaute Web-App ausliefert. Module unter `server/internal/`; Pakete greifen nur über Service-Schnittstellen aufeinander zu.

| Modul | Spec | Verantwortung |
|---|---|---|
| `auth` | K-1, K-4, K-6, K-7, K-8 | Konten, Sitzungen, argon2id, PIN-Bremse, Einladungen, Entzug, Stream-Tokens (5.4) |
| `profiles` | K-2, K-5, C-3 | Profile, Altersstufe, Sprache, Akzentfarbe, Schauzeit-Konto |
| `visibility` | K-3, E-3, H-4, O-7 | Einzige Stelle für Sichtbarkeit: View `visible_items`, Query-Builder für Listen und Suche, `SystemScope`, Berechnung `allowed_profile_ids` (3.3) |
| `library` | B-1, B-2, B-4, B-5, B-7, B-9 | Bibliotheken, Scanner, Watcher, Parser, Datei-Identität, Versionen, Extras, Pflege |
| `metadata` | B-3, B-6, B-8, E-6 | TMDB, TVDB, Trailer-Kette, Freigaben-Abbildung, Bildbeschaffung |
| `audio` | H-1 … H-3 | Hörbuch-Erkennung, Kapitel, Podcast-Feeds |
| `playback` | W-1, W-3 … W-9, K-5 | Entscheidung Direct Play/Remux/Transcode, Sitzungen, Fortschritt-Merge, Streams |
| `transcode` | W-2, W-4, W-8 | Einziger Adapter um `ffmpeg`/`ffprobe`; HW-Erkennung; HLS; Admission Control (3.5) |
| `discover` | E-1 … E-9 | Startseite, Reihen, Suche (FTS5), Merkliste, Favoriten, Top Picks, Sammlungen |
| `offline` | O-1 … O-7 | Download-Aufträge und -Profile, Regeln, Entzugsliste |
| `notify` | A-6, A-7, E-8 | Ereignisse, Kanäle, Ankündigungen; Empfänger über `visibility` gefiltert (5.5) |
| `admin` | A-1 … A-5, A-8 | Assistent, Dashboard, Sicherung, Update-Prüfung, Statistiken |
| `integrations` | I-8 | Webhook-Empfang von Radarr und Sonarr, Pfad-Umsetzung, Ereignisprotokoll; löst über `library` einen `scan.path`-Auftrag aus und greift selbst nie auf `items` zu |
| `ratelimit` | B-3, E-7, I-8 | Gemeinsame Ratenbegrenzer je Anbieter (TMDB ≤ 20/s, TVDB ≤ 5/s, Trakt ≤ 1/s, YouTube nach Tageskontingent), Backoff bei 429 |
| `secrets` | A-9, 3.8 | Verschlüsselung ausgehender Zugangsdaten at rest |
| `jobs` | alle | Persistente Queue (3.6) |
| `paths` | B-1, 3.8 | Einzige Stelle für Dateisystempfade |
| `store` | alle | SQLite, sqlc (statische Abfragen), goose, Backup-Primitive |
| `api` | C-5 | Generierte Handler-Schnittstellen, Auth-Middleware, Fehlerformat |

Weitere Komponenten: Web-Client `web/` (React-SPA als PWA, Service Worker, OPFS, Player); Design-System `packages/design-tokens` + `web/src/ui`, gemeinsam für Zuschauer und Admin (C-2, A-2); API-Client `packages/api-client` (generiert); Datenbank: eine SQLite-Datei in `/config` (WAL, 1 Schreibverbindung, N Leseverbindungen); Cache: In-Process-LRU (TTL 30–300 s, Freigaben-Cache 7 Tage); Bildspeicher: `/cache/images`, intern inhaltsadressiert; Transcode-Arbeitsbereich `/cache/transcode` (optional `tmpfs`); vorbereitete Downloads `/cache/downloads` (AN-20).

**Bildverarbeitung ohne CGO (ab M1b):** Job `image.fetch` lädt das Original und ruft über den `transcode`-Adapter ffmpeg auf: Skalierung (Poster 200/400/800 px, Backdrop 780/1280/1920 px), Ausgabe WebP (libwebp) plus JPEG-Fallback. Akzentfarbe für den Hintergrund-Tint: `palettegen=max_colors=8` auf das verkleinerte Poster, gewählt wird die Farbe mit der höchsten Sättigung bei mittlerer Helligkeit (eine Durchschnittsfarbe wäre trüb). In M1a werden TMDB-Bilder in einer festen Grösse unverändert abgelegt.

**Bildauslieferung mit Scope:** Bilder sind nur über `/items/{id}/images/{type}/{size}?v=<hash>` abrufbar; der Handler prüft den Scope (nicht sichtbar → 404), Antwort mit `Cache-Control: private, immutable`. Der Inhalts-Hash dient nur dem Cache-Busting, eine Auslieferung allein über den Hash gibt es nicht.

### 3.2 Datenmodell (Kern)

| Tabelle | Wichtige Felder | Regeln |
|---|---|---|
| `items` | `id` (ULID), `kind` (`movie`, `series`, `season`, `episode`, `extra`, `audiobook`, `podcast`, `podcast_episode`), `parent_id`, `library_id`, `own_rating`, `effective_rating`, `min_child_rating`, `available`, `rating_source`, Provider-IDs, `enclosure_url` (Podcast), `locked_fields`, `deleted_at` | Generisch, damit Musik/Fotos später nur neue `kind`-Werte brauchen (Spec "Constraints"); `effective_rating`, `min_child_rating` und `available` werden ausschliesslich per Trigger gepflegt (siehe unten); unbekannt = `NULL` |
| `media_files` | `item_id`, `root_id`, `rel_path`, `dev`, `inode`, `size`, `partial_hash` (SHA-256 über erste und letzte 64 KiB + Grösse), `version_label`, `probe` (JSON), `keyframes` (Blob), `missing_since` | Identität: (`dev`, `inode`, `size`), sonst `partial_hash`; Umbenennung ändert nur `rel_path` |
| `tracks` | `media_file_id`, Typ, Codec, Sprache, Quelle (eingebettet/Sidecar) | Sidecar-Untertitel (I-5) |
| `progress`, `watchlist`, `favorites`, `top_picks`, `downloads` | immer `item_id`, nie `media_file_id` | Benutzerdaten hängen am Item; Umbenennung und Versionswechsel verlieren nichts |
| `downloads` | `profile_id`, `device_id`, `item_id`, Qualität, Grösse, Zustand, `allowed_profile_ids` | Kein Ablauffeld (O-6); `allowed_profile_ids` vom Server berechnet (3.3) |
| `jobs` | Typ, Priorität, Idempotenz-Schlüssel (unique), `lease_until`, Versuche, `run_after` | siehe 3.6 |

Noch nicht in der Tabelle und mit ihrem Meilenstein spezifiziert: `accounts`, `profiles`, `devices` (folgen in M2; M1a arbeitet mit einem festen Hauptbenutzer-Profil), `progress_events` (Roh-Ereignisse, Grundform M1a, offline-fähig M5; nach 90 Tagen zu Tageswerten je Profil und Item verdichtet), `playback_sessions` (M1b, Träger von Ablauf und Widerruf der Stream-Tokens, 5.4).

**Freigaben ohne veraltete Kopie (K-3):** `effective_rating` ist eine abgeleitete Spalte, die nur Trigger schreiben, und zwar in derselben Transaktion wie die auslösende Änderung. Ein Trigger `AFTER UPDATE OF own_rating, parent_id` und `AFTER INSERT` auf `items` berechnet per rekursiver CTE alle Nachkommen neu: `effective_rating = max(own_rating, parent.effective_rating)`, wobei eine unbekannte Elternstufe unbekannt bleibt (AN-9). Ebenso per Trigger: `available` (Blatt: vorhandene Datei bzw. `enclosure_url`; Container wie Serie, Staffel, Podcast: mindestens ein verfügbares Kind) und `min_child_rating` (kleinste Stufe der verfügbaren Kinder). Gewählt wurde der Trigger statt einer rekursiven CTE in der View, weil jede Liste die View liest und die Startseite < 300 ms p95 bleiben muss (P-11). Absicherung: Test "Freigabe der Serie ändern → Folgen sofort unsichtbar" (10.2) und nächtlicher Job `rating.verify`, der gespeicherte gegen neu berechnete Werte vergleicht und Abweichungen im Dashboard meldet. Nach jeder Freigabe-Änderung stösst `visibility` an: Neuberechnung von `allowed_profile_ids` der betroffenen Downloads (Job mit hoher Priorität, wirksam beim nächsten Sync des Geräts, analog AN-4) und Invalidierung des Startseiten-Caches.

Löschschutz:

- Fehlt eine Datei, setzt der Scanner `missing_since`; das Item verschwindet aus Zuschauerlisten, bleibt aber **24 h** erhalten und kehrt bei Wiederauftauchen mit allen Benutzerdaten zurück. Danach Soft-Delete (`deleted_at`); Benutzerdaten bleiben 30 Tage und werden bei gleicher Provider-ID wieder verknüpft.
- **Scan-Abbruch:** Fehlt die Bibliothekswurzel, ist sie leer, oder verschwinden in einem Lauf mehr als **20 %** der Dateien einer Bibliothek, bricht der Scan ohne Änderungen ab und meldet einen Fehler im Dashboard (A-3). War das Aufräumen gewollt, bestätigt der Admin dort "Verlust akzeptieren" für diese Bibliothek; erst dann übernimmt der nächste Scan die Löschungen.

### 3.3 Sichtbarkeit (K-3) und SystemScope

- **View** `visible_items(profile_id, item_id)`: verbindet Profile, Bibliotheksrechte und Items; Bedingung `deleted_at IS NULL`, `available = 1`, `COALESCE(effective_rating, 18) <= profile.max_rating` (AN-1) und bei Containern zusätzlich `min_child_rating <= profile.max_rating` (keine Serie, deren Folgen alle über der Stufe liegen). Die View wird nur durch Migrationen im Paket `visibility` definiert.
- **Abfragen:** sqlc nur für statische Abfragen. Listen mit Scope und Suche laufen über einen kleinen Query-Builder in `visibility`, der den Join auf `visible_items` selbst einfügt; Aufrufer können ihn nicht weglassen.
- **Durchsetzung im Code:** sqlc-Abfragen liegen getrennt in `queries/scoped/` (Test prüft: jede enthält `visible_items`) und `queries/system/` (generiertes Paket `store/systemq`). `depguard` erlaubt den Import von `store/systemq` nur in `visibility`, `admin`, `library`, `metadata`, `jobs`, `notify` und `offline` (Entzugsliste). Eine eigene `go/analysis`-Regel (`itemsaccess`) verbietet SQL-Literale mit `items`, `media_files` oder `tracks` ausserhalb dieser Pakete.
- **SystemScope:** `visibility.System(reason string)` erzeugt einen ungefilterten Scope; der Analyzer erlaubt den Aufruf nur in den genannten Paketen, der Grund wird im Debug-Log mitgeschrieben.
- **Offline:** Beim Anlegen und bei jedem Sync berechnet der Server für jeden Download `allowed_profile_ids` (alle Profile desselben Kontos, die das Item sehen dürfen). Der Client zeigt einen Download nur, wenn das aktive Profil enthalten ist.
- **Ausnahme 1, Offline-PIN (AN-19):** Ein PBKDF2-Verifier (WebCrypto, 600 000 Iterationen, Salz je Profil) geht nur an Geräte, die Downloads des betroffenen Profils halten. Der Server kennt nur den argon2id-Hash; der Verifier entsteht deshalb im Client bei der nächsten erfolgreichen Online-PIN-Eingabe auf diesem Gerät und wird mit dem Download-Sync abgelegt. Eine 4-stellige PIN ist aus dem Verifier offline in Minuten zu erraten; der Admin kann darum je Profil eine separate, längere Offline-PIN (≥ 6 Stellen) setzen. Schutzziel: neugieriges Kind am geteilten Gerät, nicht Manipulation des Browsers.
- **Ausnahme 2, Schauzeit offline (K-5, AN-18):** Beim Sync liefert der Server je Profil den Tagesrest in Minuten (Stichtag in Server-Zeitzone). Offline zieht der Client die gespielte Zeit nach Geräteuhr ab und stoppt bei 0 mit Hinweis. Beim nächsten Kontakt bucht der Server die Roh-Ereignisse nach; eine Überschreitung erscheint im Verlaufsbericht des Profils.

### 3.4 Datenfluss Scan → Metadaten → Wiedergabe

1. **Erkennen:** Beim Anlegen einer Bibliothek bestimmt `statfs` den Dateisystemtyp. Lokal (ext4, xfs, btrfs, zfs): inotify, zusätzlich voller Abgleich alle 6 h. FUSE (z. B. Unraid `shfs`, mergerfs): Typ "unbekannt", inotify plus adaptives Polling, Warnung erst, wenn das Polling Änderungen findet, die der Watcher nicht gemeldet hat. Netzfreigabe (NFS, SMB/CIFS): Warnung im Assistenten und Dashboard; primär Radarr/Sonarr-Webhooks (5.6), dazu ein inkrementeller `stat`-Durchlauf aller Verzeichnisse, dessen Intervall sich nach gemessener Dauer richtet (Intervall = max(45 s, 3 × Dauer)). Grenzwert: ein Durchlauf über 10 000 Dateien muss auf dem Referenz-NAS über SMB in < 15 s fertig sein, sonst gilt B-4 für diese Freigabe nur gemäss AN-12. Der Assistent prüft `fs.inotify.max_user_watches` und warnt bei > 80 % Belegung.
2. **Parsen:** `paths` normalisiert; `library` parst Namen (Plex/Jellyfin-Konvention) und NFO (B-2); gleiche Provider-ID mit mehreren Dateien → Versionen (B-5); Unterordner `Trailers/`, `Featurettes/`, `Deleted Scenes/`, `Behind The Scenes/` und Suffixe wie `-trailer` → Extras (B-7); Sidecar-Untertitel → `tracks` (I-5).
3. **Proben:** Job `probe.file` (ffprobe): Codecs, Auflösung, HDR-Typ, Spuren, Kapitel und **Keyframe-Index** des Videos (für Remux-Seeking, 3.5).
4. **Anreichern:** Job `metadata.match` über TMDB (Filme, Serien) bzw. TVDB (optional, AN-15), de und en (B-3); fixierte Zuordnungen und Felder werden nie überschrieben (B-6). Freigabe nach AN-8. Danach Jobs `image.fetch` und `trailer.find` (5.3).
5. **Indexieren:** FTS5 für Titel, Personen, Sammlungen (E-4); Startseiten-Cache invalidieren; Ereignis `item.available` an `notify`.
6. **Wiedergabe:** Client sendet Abspielwunsch mit Geräte-ID; `playback` vergleicht Datei, Fähigkeitsprofil (W-7) und Limits (W-8, K-5) und wählt Direct Play (Range über `paths`), Remux oder Transcode (3.5). Text-Untertitel als WebVTT, Bild-Untertitel eingebrannt, wenn das Gerät sie nicht kann (W-4).
7. **Fortschritt:** Client sendet alle 10 s und bei Pause/Ende Ereignisse (Position, Gerätezeit, laufende Sequenznummer, gespielte Dauer). Offline werden sie gepuffert. Der Server führt zusammen (O-5, AN-10) und bucht Schauzeit (K-5).

### 3.5 Wiedergabe: Seeking, HDR, Admission Control

**Seeking (W-3):**

- *Transcode:* Playlist wird beim Start aus der Dauer vollständig erzeugt (4-s-Segmente, `-force_key_frames expr:gte(t,n_forced*4)`). Spulen ausserhalb des erzeugten Bereichs beendet ffmpeg und startet mit `-ss` an der Segmentgrenze sowie `-start_number N` und `-output_ts_offset`; bereits erzeugte Segmente bleiben gültig.
- *Remux* (`-c copy`): Segmentgrenzen liegen auf Quell-Keyframes. Die Segmentliste wird aus dem Keyframe-Index von `probe.file` berechnet (Segmente ≥ 4 s, an Keyframes geschnitten). HEVC-Remux für Safari in fMP4 mit Tag `hvc1`.

**HDR → SDR (W-2):** Für Handy- und Tablet-Profile wird vor dem Tonemapping auf ≤ 1080p skaliert.

| Backend | Pipeline | Status |
|---|---|---|
| NVIDIA (NVENC) | `scale_cuda` → `tonemap_cuda` (oder `libplacebo` über Vulkan) → `h264_nvenc` | getestet (Entwicklungsrechner) |
| Intel (QSV) | `vpp_qsv` (Skalierung + Tonemapping) → `h264_qsv` | getestet (Referenz-NAS, AN-7) |
| Intel/AMD (VAAPI) | `scale_vaapi` → `tonemap_vaapi` (Intel) bzw. `tonemap_opencl` (AMD) → `h264_vaapi` | **best effort, ungetestet** ohne AMD-Hardware |
| Software | `zscale` → `tonemap` (bt.2390) → `libx264 -preset veryfast` | **Dokumentierte Ausnahme von W-3:** 4K-HDR nicht echtzeitfähig; Ziel Start < 10 s bei 1080p-Ausgabe |

Dolby Vision Profil 5/8: Wiedergabe über HDR10-Basisschicht bzw. Tonemapping wie HDR10 (verschiebbar, Abschnitt 9).

**Admission Control und Prioritäten (W-8, Architect S1):**

- Kosten richten sich nach **Quelle und Pipeline**, nicht nach der Ausgabequalität, weil Decodieren und Tonemapping der Quelle den Grossteil ausmachen. Startwerte in Einheiten:

  | Quelle | Pipeline | Kosten |
  |---|---|---|
  | 4K HDR | Decode + Tonemap + Encode (jede Ausgabegrösse) | 4 |
  | 4K SDR | Decode + Skalierung + Encode | 3 |
  | ≤ 1080p SDR | Decode + Encode | 2 (Ausgabe ≤ 720p: 1.5) |
  | beliebig | Remux | 0.5 |
  | beliebig | nur Audio-Transcode | 0.25 |

  Budget je Backend: Startwerte N100/QSV 8, NVENC 12, Software = Kerne / 2. **Kalibrierung:** In M1b werden Kosten und Budget auf Entwicklungsrechner und Referenz-NAS gemessen (gleichzeitige Streams bis zum ersten Segment-Verzug > 1 s) und in `docs/perf/M1b.md` festgehalten; diese Werte ersetzen die Startwerte. Im Admin überschreibbar.
- Prioritäten: Live-Wiedergabe > Live-Audio > Download-Vorbereitung (O-1) > Probe/Bilder > Trailer/Trakt. Download-Jobs sind pausierbar, weil sie segmentweise schreiben (Format siehe M5 und AN-20).
- Reicht das Budget nicht: bei **SDR-Quellen** bietet `playback` die nächstniedrigere Qualität an (spart Encode-Kosten); bei **HDR-Quellen** spart eine niedrigere Ausgabe nichts, deshalb wartet der Start bis zu 10 s auf frei werdendes Budget (zuerst werden Download-Jobs pausiert) und lehnt sonst mit Hinweis ab. W-8-Limits (max. Qualität, Bandbreite, gleichzeitige Streams je Profil und Gast) werden an genau dieser Stelle geprüft.
- ffmpeg läuft mit `nice 10` und erhöhtem `oom_score_adj` (500), damit der OOM-Killer zuerst Transcodes trifft, nicht den Server.

### 3.6 Jobs

Festgeschriebener Umfang der eigenen Queue in SQLite: Lease mit Ablauf (5 min, Heartbeat alle 60 s; längste Schreibtransaktion < 2 s durch Batches à 500 Zeilen, damit Lease ≫ Transaktion), Idempotenz-Schlüssel (unique), Debounce (gleiche `scan.path`-Jobs innert 2 s zusammengefasst), Retry mit exponentiellem Backoff (max. 5), Priorität 0–9, Abbruch, Zeitpläne (cron-artig). Jobs sind idempotent geschrieben, weil abgelaufene Leases zu doppelter Ausführung führen können. In M0 wird `goqite` mit 1 Tag Timebox geprüft; fehlen Priorität, Idempotenz oder Zeitpläne, bleibt die Eigenlösung (ADR).

### 3.7 Deployment (Docker Compose)

| Container | Pflicht | Inhalt |
|---|---|---|
| `slouchcat` | ja | Go-Binary (per `GOARCH` cross-kompiliert, nur die apt-Stufe emuliert), eingebettete Web-App, ffmpeg/ffprobe (jellyfin-ffmpeg-Build), Intel-/AMD-Mediatreiber; Basis `debian:trixie-slim` (aktuelles Debian stable; jellyfin-ffmpeg liefert trixie-Pakete, wird in M0 verifiziert; fehlt ein Paket für eine Architektur, bleibt diese auf `bookworm-slim` bis zum nächsten jellyfin-ffmpeg-Release, dokumentiert in der ADR); amd64 ab M0, arm64 ab M1b; Lizenzordner `/licenses` mit GPL-Quellcode-Hinweis für ffmpeg |
| `caddy` | optional (Profil `https`) | Reverse Proxy mit Let's-Encrypt-Zertifikat (K-8) |

Volumes: `/config` (DB, Einstellungen, Sicherungen, `secret.key`), `/cache` (regenerierbar), `/media/...` read-only (B-1). Beim Start prüft der Server per `statfs`, ob `/config` auf NFS/SMB/CIFS liegt, und bricht mit klarer Meldung ab (SQLite-WAL auf Netzlaufwerken beschädigt die Datenbank).

```yaml
# compose.yaml (Auszug)
services:
  slouchcat:
    image: ghcr.io/<owner>/slouchcat:1
    volumes: ["./config:/config", "./cache:/cache", "/srv/media:/media:ro"]
    ports: ["8096:8096"]
# compose.nvidia.yaml: deploy.resources.reservations.devices: [{driver: nvidia, count: 1, capabilities: [gpu, video]}]
# compose.intel.yaml / compose.amd.yaml: devices: ["/dev/dri:/dev/dri"], group_add: ["<render-gid>"]
```

Veröffentlichung (löst AN-6 vs. A-5): Quellcode-Repository darf privat bleiben; das Container-Paket auf ghcr.io ist öffentlich, und A-5 liest ein öffentliches Release-Manifest (GitHub Release in einem öffentlichen Repository `slouchcat-releases`), damit ein NAS ohne Token aktualisieren kann.

### 3.8 Sicherheitsgrundsätze

1. **Rechte nur serverseitig** (K-8): jeder Handler erhält einen Scope; Durchsetzung gemäss 3.3.
2. **Tokens gehasht:** Sitzungs-, Einladungs-, Webhook- und API-Tokens sind 256-Bit-Zufallswerte, gespeichert als SHA-256. Passwörter und PINs mit argon2id. PIN-Sperre gemäss AN-11.
3. **Ausgehende Secrets at rest verschlüsselt:** TMDB-, TVDB-, Trakt-, YouTube-Schlüssel, SMTP-Passwort, Telegram-Token, VAPID-Schlüssel mit AES-256-GCM; Schlüssel in `/config/secret.key` (beim ersten Start erzeugt, Rechte 0600), nicht in der DB. In OpenAPI `writeOnly`, nie in Antworten. Sicherung (A-4) wahlweise ohne Schlüssel (Standard; Secrets nach Wiederherstellung neu eingeben) oder mit Schlüssel. Eine Sicherung mit Schlüssel wäre faktisch Klartext, darum gibt es sie nur passwortgeschützt: das ganze Archiv wird mit einer Passphrase verschlüsselt (age-Format, `filippo.io/age`, reines Go, mindestens 12 Zeichen), ohne Passphrase ist die Option gesperrt. Alternativ per Umgebungsvariable (`SLOUCHCAT_TMDB_API_KEY` usw.).
4. **Keine Secrets in Logs** (A-9): `slog` mit Redaction-Handler (`password`, `pin`, `token`, `apiKey`, `Authorization`, `X-Api-Key`, Query-Parameter `api_key`, `st`); Test mit Fake-Secrets über alle Integrations-Clients.
5. **Externe Prozesse nur über Adapter:** `os/exec` nur im Paket `transcode` (`forbidigo`), immer `exec.CommandContext` mit Argumentliste, nie Shell.
6. **Pfade nur über Pfad-Service:** `paths` löst relativ zu registrierten Wurzeln auf, verwirft `..`, folgt Symlinks nur innerhalb der Wurzeln; Lint verbietet Dateizugriff mit Benutzerdaten ausserhalb `paths`.
7. **Sitzungen:** Web per Cookie (`HttpOnly`, `Secure`, `SameSite=Lax`, CSRF-Header-Prüfung), Fremd-Clients per Bearer (C-5); Medien-URLs mit Stream-Token (5.4); Entzug löscht alle Sitzungen und Stream-Tokens eines Kontos sofort (K-7).
8. **Webhooks:** eingehend, je Integration eine eigene Adresse mit eigenem 256-Bit-Token im Pfad (gehasht gespeichert wie jedes andere eingehende Token, Punkt 2), darum nur einmal anzeigbar; Ratenbegrenzung 10 Anfragen/s je Integration; der Pfadbestandteil mit dem Token wird in allen Logs redigiert (Punkt 4), ebenso der gemeldete Dateipfad bleibt sichtbar, das Token nie.
9. **Nicht sichtbar heisst nicht vorhanden:** ID-Zugriffe auf nicht sichtbare Items liefern HTTP 404, nicht 403.

---

## 4. Stack-Entscheid

Update-Regel: Go immer eine der zwei neuesten unterstützten Versionen (Stand Oktober 2026: 1.26 oder 1.27); Node.js (nur für Build-Werkzeuge) immer die aktive LTS (heute 24); Web-Bibliotheken auf der aktuellen Hauptversion, Upgrades gesammelt einmal pro Meilenstein. Die Versionen werden in `go.mod`, `package.json` und `.tool-versions` gepinnt.

| Bereich | Entscheid | Fachbegründung | Klartext |
|---|---|---|---|
| Server-Sprache | **Go** | Statisch gelinkt, kleiner RAM, Goroutines für Streams und ffmpeg, Kompatibilitätsversprechen | Der Server ist ein einziges Programm, das schnell startet und wenig Speicher braucht. |
| HTTP | `net/http` + `chi` | Produktionsreif ohne Framework | Das Fundament stammt vom Go-Team selbst. |
| API-Vertrag | OpenAPI 3.1, `oapi-codegen` (strict server) für JSON-Endpunkte, `http.Handler` für Range/HLS/Bilder; `openapi-typescript` + `openapi-fetch` | Spec-first erzwingt C-5; CI prüft Codegen-Drift und `x-visibility` | Eine schriftliche Beschreibung aller Funktionen; Server und App entstehen daraus und passen immer zusammen. |
| Datenbank | **SQLite** (WAL), `modernc.org/sqlite` (ohne CGO) | Eingebettet, Sicherung per `VACUUM INTO`, FTS5 | Die Datenbank ist eine einzige Datei; eine Sicherung ist eine Kopie davon. |
| Abfragen | `sqlc` für statische Abfragen; Query-Builder in `visibility` für Listen und Suche | Typsicher, Sichtbarkeit nicht umgehbar | Jede Liste geht zwingend durch den Altersfilter. |
| Migrationen | `goose`, nur vorwärts, eingebettet; Vorlage mit `-- +goose NO TRANSACTION`, `PRAGMA foreign_keys=OFF`, `PRAGMA foreign_key_check`, `PRAGMA foreign_keys=ON`; vor jedem Lauf `VACUUM INTO` (letzte 5 behalten) | goose kapselt sonst in Transaktionen, in denen `foreign_keys=OFF` wirkungslos ist | Vor jeder Änderung am Datenmodell wird automatisch eine Sicherung angelegt. |
| Jobs | Eigene Queue gemäss 3.6 (goqite geprüft) | Kein Zusatzdienst | Hintergrundarbeiten überleben einen Neustart. |
| Transcoding, Bilder | ffmpeg/ffprobe (jellyfin-ffmpeg-Build) hinter `transcode` | HW-Tonemapping und Treiberpatches; WebP-Encoder; austauschbar gegen BtbN-Builds | Das bewährte Werkzeug zum Umwandeln von Videos und Bildern. Slouchcat ruft es nur auf. |
| Web | React (aktuell 19) + TypeScript + Vite, `@tanstack/react-router`, `@tanstack/react-query`; reine SPA | Grösstes Ökosystem, Agenten sehr sicher, Offline einfacher ohne SSR | Die App nutzt das verbreitetste Werkzeug für Web-Oberflächen. |
| UI-Grundbausteine | `react-aria-components`, `@noriginmedia/norigin-spatial-navigation` | Fokus und Pfeiltasten-Navigation (C-4) | Bedienbar mit Pfeiltasten wie mit einer Fernbedienung. |
| PWA / Offline | `vite-plugin-pwa` (Workbox) für App-Shell; Downloads in OPFS, Metadaten in IndexedDB (`idb`) | Schreibweg je iOS-Version im Spike bestimmt (M1a) | Die App lässt sich installieren und spielt Geladenes ohne Internet. |
| Player | `<video>`/`<audio>` + `hls.js`; natives HLS in Safari; eigene Bedienoberfläche | Klein, stabil | Der eingebaute Browser-Player, ergänzt um das Fehlende. |
| Design-Tooling | Tokens im DTCG-Format, Style Dictionary (aktuelle Hauptversion) → CSS Custom Properties + TS-Typen; CSS Modules; Storybook ab M1b; selbst gehostete variable Schrift | `stylelint` (`color-no-hex`, `declaration-strict-value`), `eslint-plugin-i18next` | Farben, Schriften, Abstände und Bewegung sind an einer Stelle definiert. |
| i18n | `i18next` + `react-i18next`, `web/src/locales/{de,en}.json` | C-3 | Alle Texte in einer deutschen und einer englischen Datei. |
| Tests | Go `testing` + `testify`; Vitest + Testing Library; Playwright (Chromium, Firefox, WebKit); Test-Medien per ffmpeg `lavfi`; aufgezeichnete Webhook-Nutzlasten von Radarr und Sonarr (gepinnte Versionen, in `testdata/`) | Reproduzierbar, keine geschützten Medien | Jede Funktion wird automatisch geprüft. |
| Lint / Sicherheit | `golangci-lint` (`forbidigo`, `depguard`, `gosec`), Analyzer `itemsaccess`, `govulncheck`, ESLint, Prettier, stylelint, `gitleaks` | Setzt 3.3 und 3.8 als Regeln durch | Werkzeuge prüfen den Code automatisch auf Fehler und Lecks. |
| Monorepo / CI | pnpm Workspaces + Go-Modul `server/`; Taskfile (`task ci`, `task dev`); GitHub Actions ruft `task ci` | Lokal und CI identisch | Ein Befehl baut und prüft alles. |

Repo-Struktur (M0):

```
api/openapi.yaml            # Vertrag (Quelle)
server/                     # cmd/slouchcat, internal/<modul>, migrations/, queries/{scoped,system}/, tools/itemsaccess/
web/                        # src/ui, src/features/<baustein>, src/locales
packages/design-tokens/  packages/api-client/   # generiert, nicht von Hand ändern
deploy/                     # Dockerfile, compose*.yaml, Caddyfile
testdata/                   # Generatoren für Test-Medien, DB-Stände als SQL-Dumps (db/<meilenstein>.sql)
docs/spec/ docs/adr/ docs/perf/
CLAUDE.md Taskfile.yml
```

---

## 5. Entscheidungen

Die Entscheide 5.1–5.3 sind **bestätigungspflichtig** durch den Auftraggeber (spätestens vor M2, siehe 5.7); 5.1 ist mit dem Zuschnitt vom 22. Oktober 2026 entfallen. 5.4–5.6 beantworten die offenen Fragen des Critic.

### 5.1 Seerr-Einbettung (Spec "Offene Punkte" 1) — entfallen

**Entscheid vom 22. Oktober 2026 (Entscheidungsprotokoll Frage 10):** gegenstandslos. Slouchcat bindet Seerr
gar nicht an, weder über die API noch als Einbettung. Wünsche entstehen direkt in Seerr. I-4 ist gestrichen
und steht in den Non-Goals der Spec, ebenso AN-5 (Wunschrecht) und AN-16 (Kontozuordnung).

Der frühere Entscheid lautete: eigener Bereich "Wünschen" mit Slouchcat-Oberfläche über die Seerr-API, keine
iFrame-Einbettung, Kontozuordnung durch den Admin. Er wurde nie umgesetzt und ist mit dem Zuschnitt von M9
hinfällig geworden.

### 5.2 Ankündigungskanäle (Spec "Offene Punkte" 2, A-7)

**Entscheid:** Dieselben Kanäle wie A-6 (In-App, E-Mail, Web-Push, Telegram). Jedes Konto wählt den Ereignistyp "Ankündigungen" je Kanal; Standard: In-App immer, E-Mail für Gastkonten an, übrige aus. **Begründung:** Der Dispatcher aus A-6 existiert ohnehin (M8).

### 5.3 Trailer-Beschaffung (Spec "Offene Punkte" 3, B-8, W-10)

**Entscheid:** Kette hinter `TrailerProvider`: (1) TMDB `videos` (Typ `Trailer`, YouTube, Sprache aus `iso_639_1`), (2) YouTube Data API v3 als Fallback (Heuristik: offizieller Kanal, Dauer 60–300 s, Titelübereinstimmung; 100 Einheiten je Suche, max. 100 Suchen/Tag), (3) optionaler KI-Anbieter, standardmässig aus. Abspielen nur im YouTube-IFrame-Player (`youtube-nocookie.com`); manuelle Auswahl fixiert den Trailer; offline kein Trailer-Knopf. **Begründung:** Spec-Empfehlung, ergänzt um TMDB als kostenlose erste Quelle.

### 5.4 HLS-Autorisierung für Fremd-Clients (Critic, offene Frage 1)

Native Player senden keinen Bearer-Header. Der Stream-Start (`POST /playback/sessions`, authentifiziert per Cookie oder Bearer) liefert URLs mit einem **Stream-Token** `st` (ab M1b; M1a nutzt für Direct Play das Sitzungs-Cookie). Das Token ist ein HMAC-SHA256 nur über Sitzungs-ID und Item, ohne Ablaufzeit, weil Safari Playlist-URLs während der ganzen Wiedergabe behält und eine Pause über Nacht sonst abbräche. Ablauf und Widerruf stehen in der Tabelle `playback_sessions` (`last_activity`, `expires_at` = letzte Aktivität + 12 h, `revoked_at`); jede Playlist-, Segment-, Range- und Untertitel-Anfrage prüft die Zeile per Primärschlüssel über den Lese-Pool. Widerruf bei Stream-Ende, Admin-Abbruch (W-6) und Entzug (K-7) setzt `revoked_at`; damit gilt "401 innert 1 s" auch für laufende Streams. Alle Antworten tragen `Referrer-Policy: no-referrer`, damit das Token nicht über Referer-Header abfliesst. Der Parameter `st` wird in allen Logs redigiert (3.8 Pkt. 4). Die Web-App nutzt denselben Weg.

### 5.5 Benachrichtigungen für Kinderprofile (Critic, offene Frage 2)

Kanäle gehören dem Konto (A-6), Favoriten dem Profil (E-8). Ein Ereignis für ein Profil geht an die Kanäle des Kontos mit Profilname als Präfix ("[Lena] Neue Folge …") und in den In-App-Eingang nur dieses Profils. Vor dem Versand prüft `notify` über `visibility`, ob das Profil das Item sehen darf; sonst entfällt die Benachrichtigung.

### 5.6 Radarr/Sonarr-Webhooks (Critic, offene Frage 3; erweitert am 22. Oktober 2026, I-8)

Slouchcat registriert keine Webhooks selbst (Steuerung ist Non-Goal) und ruft Radarr/Sonarr auch nicht ab —
seit dem Wegfall von I-1 gibt es kein Polling mehr. Der Webhook ist die einzige Verbindung zu den Arr-Diensten
und rein eingehend.

- **Adresse je Integration:** Der Admin legt eine Integration an (Name, Typ Radarr oder Sonarr). Slouchcat
  erzeugt ein 256-Bit-Token und zeigt die vollständige Adresse `POST {BasePath}/webhooks/{token}` **einmal**
  zum Kopieren (gehasht gespeichert, 3.8 Punkt 2); später nur noch die letzten vier Zeichen, mit Knopf
  "Token erneuern". Der Admin trägt die Adresse in Radarr/Sonarr unter "Connect → Webhook" ein.
- **Ereignisse:** `Download` (Import und Upgrade), `Rename`, `MovieFileDelete`/`EpisodeFileDelete`,
  `MovieDelete`/`SeriesDelete`. `Test` wird quittiert und protokolliert, damit der Test-Knopf in Radarr grün
  wird. Alle übrigen Ereignisse (`Grab`, `Health`, `ApplicationUpdate` …) werden mit 200 angenommen und
  ignoriert, damit eine grosszügige Auswahl in Radarr nichts kaputt macht.
- **Pfad:** aus `movie.folderPath` (Radarr) bzw. `series.path` (Sonarr). Je Integration gibt es eine
  Umsetzungstabelle aus Präfix-Paaren (`/movies` → `/media/Filme`), angewandt mit dem längsten passenden
  Präfix; Standard leer. Das Ergebnis muss über `paths` unter einer registrierten Bibliothekswurzel liegen,
  sonst HTTP 422 mit beiden Pfaden im Ereignisprotokoll.
- **Wirkung:** ein `scan.path`-Auftrag auf das gemeldete Verzeichnis — derselbe Auftrag, den der
  inotify-Watcher erzeugt (3.4), mit derselben Zusammenfassung mehrerer Meldungen. Kein zweiter Scan-Weg,
  nur ein zweiter Auslöser. Löschungen laufen über denselben Pfad-Scan.
- **Sichtbarkeit:** `x-visibility: public`, weil die Anfrage keine Sitzung trägt; die Autorisierung ist das
  Token. Als JSON-Endpunkt läuft sie über den generierten strict server (Prinzip 3) mit Eintrag im
  Vertragstest.
- **Admin:** Liste der Integrationen mit letztem Kontakt, Ereignisprotokoll der letzten 20 Meldungen je
  Integration (Zeit, Ereignis, gemeldeter Pfad, umgesetzter Pfad, Ergebnis) und eine Dashboard-Kachel
  (Anzahl 24 h, Warnung nach 7 Tagen ohne Meldung) — damit bleibt A-3 "Integrationsstatus" nach dem Wegfall
  von I-1 gefüllt.
- **Ohne Webhook** bleibt der reguläre Abgleich aus 3.4 die einzige Quelle; es geht nichts verloren, es dauert
  nur länger (AN-12).

### 5.7 Annahmen und stillschweigende Entscheide (zur Bestätigung)

| # | Annahme / Entscheid | Betrifft | Spätestens bestätigt vor |
|---|---|---|---|
| AN-1 | Unbekannte Freigabe = gesperrt für Profile unter FSK 18; B-9 listet solche Titel und erlaubt Massenzuweisung | K-3, B-9 | M2 |
| AN-2 | Hörbücher/Podcasts: Standard-Altersstufe je Bibliothek, überschreibbar je Titel; `itunes:explicit=true` → FSK 18 | H-4 | M6 |
| AN-3 | Trakt: Top-100 laden, erste 20 Treffer aus der Bibliothek zeigen; unter 5 Treffern Reihe ausblenden | E-7, E-9 | M7 |
| AN-4 | K-7 bei Downloads greift beim nächsten Serverkontakt; ein dauerhaft offline gehaltenes Gerät ist wegen O-6 nicht sperrbar; Hinweis im Admin | K-7, O-6 | M5 |
| AN-6 | Quellcode privat möglich, Container-Paket und Release-Manifest öffentlich (3.7) | A-5 | M0 |
| AN-7 | Testhardware (siehe unten) steht zur Verfügung | W-2, W-3, O-x | M1a |
| AN-8 | Freigaben-Reihenfolge DE → AT → CH → GB → US. AT und CH bleiben drin, weil der Haushalt in der Schweiz steht und TMDB bei deutschsprachigen Veröffentlichungen oft nur AT- oder CH-Freigaben führt. Regel für alle Länder: Aufrunden auf die nächsthöhere FSK-Stufe (0/6/12/16/18), unbekannte Werte → `NULL`. AT: 0→0, 6→6, 10→12, 12→12, 14→16, 16→16, 18→18. CH: 0→0, 6→6, 7/8/10→12, 12→12, 14→16, 16→16, 18→18. BBFC: U→0, PG→6, 12/12A→12, 15→16, 18/R18→18. US-Kino: G→0, PG→6, PG-13→12, R→16, NC-17→18. US-TV: TV-Y/TV-G→0, TV-Y7/TV-PG→6, TV-14→16, TV-MA→18 | K-3 | M2 |
| AN-9 | Vererbung: Folge und Staffel erben von der Serie, Extras vom Titel; eigene Stufe darf nur höher sein | K-3 | M3 |
| AN-10 | O-5-Regel: je Profil und Item gewinnt das Ereignis mit der jüngsten serverseitig korrigierten Zeit; "fertig gesehen" bleibt bestehen, bis ein neueres Ereignis mit Position < 90 % kommt | O-5 | M5 |
| AN-11 | PIN-Sperre ab 3. Fehlversuch: 30 s, 60 s, 120 s …, max. 15 min, je Profil und Gerät | K-4 | M2 |
| AN-12 | B-4 (< 60 s) gilt garantiert für lokale Dateisysteme und Netzfreigaben mit eingerichtetem Arr-Webhook (I-8, ab M9 verfügbar); sonst gemäss Messung in 3.4 | B-4 | M1a |
| AN-13 | iOS: Auto-Downloads (O-3) nur bei geöffneter App, "nur WLAN" auf iOS nicht erkennbar (Einstellung dort ausgeblendet mit Hinweis) | O-3 | M5 |
| AN-14 | Einladungslinks: einmalig, gültig 7 Tage, widerrufbar | K-6 | M2 |
| AN-15 | Jede Installation hinterlegt eigene Schlüssel für TMDB, Trakt, YouTube (kostenlos); TVDB optional, nur mit eigener Lizenz bzw. Abo-PIN; Serien primär über TMDB | B-3, E-7, B-8 | M3 |
| AN-17 | Software-Transcoding ist eine dokumentierte Ausnahme von W-3 (3.5) | W-3 | M1b |
| AN-18 | Schauzeit-Limit offline: Server liefert beim Sync den Tagesrest je Profil, Client stoppt offline nach Geräteuhr bei 0; Nachbuchung und Überschreitung im Bericht beim nächsten Kontakt (deklarierte Ausnahme 2 von Prinzip 1, 3.3) | K-5, H-4, O-7 | M5 |
| AN-19 | Offline-PIN: Verifier nur auf Geräten mit Downloads des Profils, erzeugt bei der nächsten Online-PIN-Eingabe; optional separate Offline-PIN ≥ 6 Stellen; Schutz nur gegen neugierige Mitbenutzer (deklarierte Ausnahme 1 von Prinzip 1, 3.3) | K-4, O-7 | M5 |
| AN-20 | Vorbereitete Downloads: Format fragmentiertes MP4 (segmentweise erzeugt, pausierbar, mit `sidx`-Index für Spulen); Ablage `/cache/downloads`, gelöscht 24 h nach erfolgreichem Abruf durch das Gerät; Budget im Admin (Standard 50 GB), bei vollem Budget warten neue Vorbereitungen. Fallback, falls der Spike (10.4) Spulen in fMP4 aus OPFS verwirft: Segmente zusammenfügen plus `faststart` (braucht kurzzeitig doppelten Platz) | O-1, O-4 | M5 |

**AN-7 Testhardware und Ersatzweg:**

| Gerät | Zweck | Ersatz, falls nicht vorhanden |
|---|---|---|
| iPhone (aktuelles und vorheriges iOS) | Spike, O-x, W-3 | Leihgerät; für Wiedergabe ohne Offline: Real-Device-Cloud (z. B. BrowserStack, kostenpflichtig) |
| iPad | C-1, O-x | iPhone + Safari-Responsive-Modus, Abweichung dokumentiert |
| Android-Handy | O-3 "nur WLAN", W-3 | Leihgerät oder Real-Device-Cloud |
| Mac mit Safari | Safari Desktop | WebKit in Playwright, Abweichung dokumentiert |
| Intel-N100-Mini-PC (QSV, 8 GB) | Referenz-NAS | Kauf empfohlen (~150–200 CHF); sonst gilt "läuft auf schwachem NAS" als ungeprüft |
| Raspberry Pi 5 (8 GB) | arm64 | Start und Direct Play in QEMU/Cloud-arm64-VM; Leistung ungeprüft |
| AMD-GPU | VAAPI/AMD | keine; "best effort, ungetestet" |

---

## 6. Roadmap

### 6.1 Aufwand und Abhängigkeiten

Ein Arbeitstag = ca. 6 h aktive Arbeit des Auftraggebers mit Agenten. Kalenderzeit hängt von der Verfügbarkeit ab. Schätzung ehrlich mit Spanne; Abweichungen werden je Meilenstein im Changelog des Repos protokolliert.

| Meilenstein | Arbeitstage | Kumuliert (oben) | Abhängig von |
|---|---|---|---|
| M0 Fundament | 4–6 | 6 | – |
| M1a Durchstich: Scan, Direct Play, Startseite, iOS-Spike-Start | 6–10 | 16 | M0 |
| M1b Durchstich: HLS, Seeking, Stream-Token, Bildpipeline, Design, Spike-Auswertung | 10–16 | 32 | M1a |
| M2 Konten, Profile, Zugriff | 8–12 | 44 | M1b |
| M3 Bibliothek komplett | 12–18 | 62 | M2 |
| M4 Wiedergabe komplett | 18–28 | 90 | M3 (nicht parallel: beide ändern Scanner, Probe und Datenmodell) |
| M5 Offline | 14–20 | 110 | M4 |
| M6 Hörbücher, Podcasts | 6–10 | 120 | M5 |
| M7 Entdecken, Kuratierung | 8–12 | 132 | M6 |
| M8 Administration, Benachrichtigungen | 10–15 | 147 | M7 |
| M9 Schnelles Einlesen über Webhooks | 2–3 | 150 | M8 |
| M10 Härtung, Release 1.0 | 10–14 | 164 | alle |
| **Summe** | **108–164** | | |

Gegenüber dem ersten Entwurf dieser Iteration angehoben: M1b (+Stream-Token und Bildpipeline aus M1a), M4 (Long Tail Safari/hls.js und Gerätetests), M5 (fMP4-Downloads, Offline-Schauzeit), M10 (Verifikationsaufwand). Die Kalenderzeit wird eher vom Testen auf echten Geräten begrenzt als vom Schreiben des Codes.

Durchstich-Ziel "innerhalb der ersten Wochen" (Spec "Constraints"): M0 + M1a = 10–16 Arbeitstage, d. h. bei Vollzeit 2–4 Wochen bis zum ersten abgespielten Film.

### 6.2 Gemeinsame Definition of Done (gilt für jeden Meilenstein)

1. Alle Zeilen der Abnahmematrix (Abschnitt 7) für diesen Meilenstein grün.
2. `task ci` grün; Kinderprofil-Vertragstest (10.2) grün.
3. Jede in diesem Meilenstein neue oder geänderte Seite hat einen Playwright-Tastaturtest (P-15).
4. Neuer eingefrorener DB-Stand `testdata/db/<Mx>.sql` (ab M1a); Migrationstest aller früheren Stände grün.
5. Messwerte in `docs/perf/<Mx>.md`; Gerätecheckliste (10.1) ausgefüllt.
6. Verifier-Agent bestätigt die DoD anhand von Belegen.

### M0 – Projektfundament (4–6 Tage)

- **Ziel:** Schlankes Gerüst, auf dem Agenten sicher arbeiten.
- **Spec-IDs:** C-2 (Token-Pipeline), C-3 (i18n-Gerüst), C-5 (OpenAPI-Pipeline), A-4 (Sicherung vor Migration), A-9 (Redaction).
- **Schritte:**
  1. Repo-Struktur (Abschnitt 4); Spec nach `docs/spec/`, ADR nach `docs/adr/0001-stack.md`.
  2. `CLAUDE.md`: Doku de-CH, Code en; generierter Code tabu; Titelabfragen nur über `visibility` bzw. `store/systemq` in erlaubten Paketen; `os/exec` nur in `transcode`; Dateizugriff nur über `paths`; Migrationen nur vorwärts nach Vorlage (3.2/4); Streaming-Endpunkte als `http.Handler` mit OpenAPI-Beschreibung; jede Operation mit `x-visibility`; jede Seite mit Tastaturtest; keine Farbe/kein Text hart im Code; keine Nicht-Ziele bauen.
  3. Go-Server mit `/healthz`, Konfiguration, `slog` + Redaction, SQLite + goose (mit `VACUUM INTO` vor jedem Lauf) + sqlc, `statfs`-Prüfung von `/config`, Job-Queue-Grundgerüst und goqite-Timebox (1 Tag).
  4. `api/openapi.yaml` mit Health, Fehlerformat; Codegen beidseitig; Schema-Test "jede Operation hat `x-visibility`"; Analyzer `itemsaccess` als Gerüst.
  5. Web-Gerüst (Vite, React, Router, Query, i18n, Manifest), Token-Paket (dunkle Farben, Typografie, Abstände, Bewegung, 8 Akzentfarben mit Kontrast ≥ 4.5:1), Lint-Regeln.
  6. Dockerfile amd64, `compose.yaml` + GPU-Overrides, Test-Medien-Generator (H.264/AAC-MP4, HEVC-10-bit-MKV, HDR10-HEVC 4K 30 s, AC3/DTS, SRT/ASS, Kapitel).
- **DoD (zusätzlich zu 6.2):** Container `healthy` in < 5 s; Leerlauf-RSS < 60 MB; Redaction-Test grün; Lint schlägt je in einem Negativfall an (Hex-Wert in CSS, Literaltext in JSX, `os/exec` ausserhalb `transcode`, Operation ohne `x-visibility`); Migration auf Testdatenbank erzeugt vorher eine Sicherung.

### M1a – Durchstich Teil 1: Scan, Direct Play, Startseite (6–10 Tage)

- **Ziel:** Film-Bibliothek scannen, Film per Direct Play abspielen, einfache Startseite; iOS-Spike am ersten Tag starten, weil er 8 Kalendertage braucht.
- **Spec-IDs:** A-1 (minimal: Konto, Bibliothek, TMDB-Schlüssel), K-1 (nur Hauptbenutzer), B-1, B-2 (Filme), B-3 (TMDB), B-4, W-1, W-5, E-1 (Karussell, "Weiterschauen", "Neu hinzugefügt"), C-1 (installierbar), C-2, C-4 (neue Seiten); Spike für O-1, O-4, O-6, O-7.
- **Schritte:**
  1. **iOS-Spike starten (Tag 1–2):** statischer Download-Endpunkt für eine fertige 1.5-GB-Datei in zwei Varianten (fragmentiertes MP4 mit `sidx` und `faststart`-MP4, vorab mit ffmpeg erzeugt, AN-20); Test-PWA mit beiden Schreibwegen (`createSyncAccessHandle` im Worker, `createWritable`); Protokoll gemäss 10.4.
  2. `secrets`-Modul; minimaler Assistent; Login; Datenmodell `items`/`media_files` mit Datei-Identität und Löschschutz (3.2); View `visible_items` (vorerst nur Hauptbenutzer-Profil).
  3. Scanner (Namensparser Filme, NFO), Watcher mit `statfs`-Erkennung (3.4), Probe inkl. Keyframe-Index, TMDB de/en über `ratelimit`; TMDB-Bilder in fester Grösse unverändert abgelegt (Pipeline folgt in M1b), ausgeliefert über `/items/{id}/images/...` mit Scope.
  4. Startseite (Karussell + 2 Reihen) und Detailseite im Design-System; Tastaturnavigation.
  5. Direct Play (Range über `paths`, autorisiert per Sitzungs-Cookie; Stream-Token folgt in M1b), Fortschritt speichern (`progress_events` in Grundform) und fortsetzen; PWA-Manifest und Service Worker.
- **DoD (zusätzlich zu 6.2):** frischer Stack: Assistent → 20 generierte + 5 eigene Filme in < 5 min mit Postern; Umbenennung einer Datei behält Fortschritt; abgehängte Bibliothekswurzel löscht nichts (Test); Direct Play startet in Chrome, Firefox, Safari in < 2 s; Startseite LCP < 2.5 s (Lighthouse Mobile); Spike läuft.

### M1b – Durchstich Teil 2: HLS, Seeking, Design (10–16 Tage)

- **Ziel:** Ein Transcoding-Fall und Remux mit verlässlichem Spulen; Design-Feinschliff; Spike-Entscheid; Rückfall-Prüfung (2.5).
- **Spec-IDs:** W-2 (SDR-Fall), W-3 (gemessen, kein Gate), C-2, C-4; Spike-Auswertung O-1, O-4, O-6, O-7.
- **Schritte:**
  1. Transcode **HEVC-MKV → H.264/AAC-fMP4-HLS** für Firefox (NVENC + Software) nach Seeking-Modell 3.5; Remux mit Keyframe-Segmentliste, `hvc1` für Safari.
  2. `playback_sessions` und Stream-Token (5.4) für HLS und Direct Play; `Referrer-Policy: no-referrer`.
  3. Admission Control Grundstufe (Kosten nach Quelle und Pipeline, Prioritäten, `nice`, `oom_score_adj`) und **Kalibrierung** der Kosten und Budgets auf Entwicklungsrechner und Referenz-NAS (3.5).
  4. Messung HDR10-4K → 1080p-SDR auf Referenz-NAS (QSV) und Entwicklungsrechner (NVENC), Startzeit am Handy (dokumentiert, noch kein Gate).
  5. Bildpipeline über ffmpeg (Grössen, WebP, `palettegen`-Akzentfarbe, 3.1); Design-Feinschliff: Poster-Tint, Fokusbewegung ≤ 200 ms, Akzentfarbe; Storybook; arm64-Build.
  6. iOS-Spike auswerten (Go/No-Go nach 10.4, inkl. Download-Format AN-20); Rückfallkriterium 2.5 prüfen.
- **DoD (zusätzlich zu 6.2):** HEVC-Transcode startet in Firefox in < 5 s (1080p); Spulen auf 20/50/80 % spielt in < 3 s weiter (Transcode und Remux); Remux-HEVC spielt in Safari; HDR-Messwerte für NAS und Entwicklungsrechner liegen vor; kalibrierte Kosten und Budgets in `docs/perf/M1b.md`; Stream-Token nach Widerruf → 401 innert 1 s; Spike-Bericht `docs/perf/M1-ios-offline.md` mit Entscheid; Images amd64 und arm64 starten.

### M2 – Konten, Profile, Zugriff (8–12 Tage)

- **Ziel:** Profile mit Alters- und Rechteschutz, Gäste, Zugriff von aussen.
- **Spec-IDs:** K-1, K-2, K-3, K-4, K-6, K-7 (Sitzungen), K-8, C-3, B-3 (Sprache je Profil).
- **Schritte:** (1) Datenmodell Konto/Profil/Einladung/Gerät, Profilwahl, PIN (AN-11); (2) `visibility` vollständig inkl. AN-1/AN-8, SystemScope, Lint aktiv, Vertragstest 10.2 als CI-Pflicht; (3) Einladungen (AN-14), Gastkonten, Entzug; (4) Profilsprache und Akzentfarbe; (5) Compose-Profil `https` mit Caddy, Anleitung für eigenen Proxy.
- **DoD (zusätzlich zu 6.2):** Matrixzeilen K-1–K-4, K-6–K-8, C-3 grün; Vertragstest deckt 100 % der Operationen ab.

### M3 – Bibliothek komplett (12–18 Tage)

- **Ziel:** Filme und Serien auf Jellyfin-Niveau plus Trailer, Extras, Pflege.
- **Spec-IDs:** B-1 … B-9, W-10.
- **Schritte:** (1) Serien/Staffeln/Folgen inkl. Specials, Mehrfachfolgen, Vererbung AN-9; optional TVDB (AN-15); (2) Versionen und Extras; (3) manuelle Zuordnung, Trailer-Kette 5.3 mit Sprachwahl; (4) Pflegewerkzeuge inkl. Titel ohne Freigabe und Massenzuweisung; (5) Lasttest 10 000 Dateien (Schreib-Batches, Lease-Verhalten).
- **DoD (zusätzlich zu 6.2):** Matrixzeilen B-1–B-9, W-10 grün; Lasttest: Scan von 10 000 Dateien ohne doppelt ausgeführte Jobs, Startseiten-API bleibt währenddessen < 300 ms p95.

### M4 – Wiedergabe komplett (18–28 Tage)

- **Ziel:** Verlässliche Wiedergabe auf allen Zielgeräten mit Regeln und Übersicht.
- **Spec-IDs:** W-1 … W-9, K-5, I-5.
- **Schritte:** (1) Fähigkeitsprofile, Geräteübersicht, Entscheidungstabelle; (2) HDR-Pipelines je Backend (3.5), Dolby Vision über Basisschicht; (3) Spur-/Untertitelwechsel, Einbrennen, Sidecars; (4) W-8-Limits und Admission Control vollständig, K-5 Limit und Bericht; (5) Intro/Abspann (Audio-Fingerprint je Staffel + Kapitelnamen), Auto-Next, Kapitel; Stream-Übersicht mit Beenden.
- **DoD (zusätzlich zu 6.2):** Matrixzeilen W-1–W-9, K-5, I-5 grün; Testmatrix 10.3 grün; P-6 erfüllt auf NVENC und QSV.

### M5 – Offline-Wiedergabe (14–20 Tage)

- **Ziel:** Echte Offline-Wiedergabe in der PWA auf Basis des Spike-Entscheids.
- **Spec-IDs:** O-1 … O-7 (Video), C-1, K-4 (Offline-PIN), K-5 (Offline-Schauzeit), K-7 (Downloads).
- **Schritte:** (0) Am ersten Tag den 30-Tage-Langzeittest für O-6 auf Android und Desktop starten (abgehakt in M7). (1) Download-Profile (H.264/AAC als fragmentiertes MP4 nach AN-20, 3 Qualitätsstufen, gewählte Tonspur, WebVTT-Sidecars) als pausierbarer, segmentweise schreibender Job in `/cache/downloads` mit Budget und 24-h-Aufräumregel; Ablage in OPFS mit Wiederaufnahme; (2) Staffel-Download, Auto-Download-Regeln (AN-13); (3) `allowed_profile_ids`, Offline-PIN (AN-19), Offline-Schauzeit (AN-18), Speicheranzeige und Apple-Hinweis; (4) Roh-Ereignisse hochladen, Merge im Server (AN-10); Entzugsliste (AN-4).
- **DoD (zusätzlich zu 6.2):** Matrixzeilen O-1 (Video), O-2–O-5, O-7, C-1 und K-5 (offline) grün; O-6 Langzeittest läuft (Abschluss M7), iOS-8-Tage-Test bestanden.

### M6 – Hörbücher und Podcasts (6–10 Tage)

- **Ziel:** Audio-Medien mit denselben Regeln wie Video.
- **Spec-IDs:** H-1 … H-4; O-1, O-3 für Audio.
- **Schritte:** (1) Bibliothekstyp Hörbücher: Ordner- und M4B-Erkennung, Kapitel, Autor/Sprecher/Cover; (2) Podcasts per Feed-URL, Abruf alle 60 min (H-2); (3) Audio-Player mit Kapiteln, Geschwindigkeit 0.5–3.0, Fortschritt im Kapitel; Altersregeln AN-2; Downloads und Auto-Downloads für Audio.
- **DoD (zusätzlich zu 6.2):** Matrixzeilen H-1–H-4, O-1 (Audio), O-3 (Podcast) grün.

### M7 – Entdecken und Kuratierung (8–12 Tage)

- **Ziel:** Vollständige Startseite, Suche, Merkliste, Favoriten, Trakt-Reihen.
- **Spec-IDs:** E-1 … E-9, I-7.
- **Schritte:** (1) Cat's Empfehlungen mit Zielprofilen, Reihenfolge, Notiz, Zeitraum; (2) Reihen inkl. Merkliste je Profil (E-5) und Sammlungen (E-6), Reihenfolge durch Hauptbenutzer, max. 8, keine leeren; (3) Suche FTS5 über Query-Builder mit Filtern; (4) Trakt-Job täglich (AN-3); (5) Favoriten mit In-App-Benachrichtigung.
- **DoD (zusätzlich zu 6.2):** Matrixzeilen E-1–E-9, I-7 grün; O-6-Langzeittest aus M5 nach 30 Tagen abgehakt.

### M8 – Administration und Benachrichtigungen (10–15 Tage)

- **Ziel:** Admin-Oberfläche in gleicher Design-Qualität, Kanäle, Sicherung.
- **Spec-IDs:** A-1 … A-9, E-8 (Kanäle), Entscheide 5.2 und 5.5.
- **Schritte:** (1) Assistent vollständig inkl. GPU-Erkennung, Compose-Override-Vorschlag, Dateisystem- und Watch-Limit-Warnung; (2) zentrale Verwaltung und Dashboard; (3) Sicherung/Wiederherstellung per Knopf ohne Schlüssel oder mit Schlüssel nur passwortgeschützt (3.8), Update-Hinweis über Release-Manifest; (4) Kanäle E-Mail, Web-Push (iOS nur installierte PWA), Telegram; Ankündigungen; Statistiken.
- **DoD (zusätzlich zu 6.2):** Matrixzeilen A-1–A-9, E-8 (Kanäle) grün.

### M9 – Schnelles Einlesen über Webhooks (2–3 Tage)

- **Ziel:** Radarr und Sonarr melden Dateiänderungen, damit neue Titel ohne Verzögerung erscheinen — vor allem
  auf Netzfreigaben, wo der reguläre Abgleich langsam ist (AN-12).
- **Spec-IDs:** I-8, Entscheid 5.6.
- **Schritte:** (1) Tabellen, Migration und Dienst für Webhook-Integrationen (Token gehasht, Pfad-Umsetzung,
  Ereignisprotokoll); (2) Endpunkt `POST /webhooks/{token}` über den strict server mit Ratenbegrenzung,
  Nutzlast-Erkennung für Radarr und Sonarr und `scan.path` als Wirkung (5.6); (3) Admin-Seite: Integration
  anlegen, Adresse einmal anzeigen, Token erneuern, Pfad-Umsetzung pflegen, Ereignisprotokoll, Dashboard-Kachel,
  Playwright-Tastaturtest; (4) Benachrichtigungskanal "Beschaffung" vollständig entfernen (Oberfläche, Texte,
  Code, Vorwärts-Migration auf `account_channels`), weil sein Erzeuger I-1 entfallen ist.
- **DoD (zusätzlich zu 6.2):** Matrixzeile I-8 grün; kein Token im Log-Scan (A-9); die Einstellungsseite zeigt
  vier Ereignistypen statt fünf.

### M10 – Härtung und Release 1.0 (10–14 Tage)

- **Ziel:** Version 1 vollständig, gemessen, dokumentiert.
- **Spec-IDs:** C-4 (Gesamtaudit), C-5 (Nachweis), Constraints Betrieb.
- **Schritte:** (1) Fremd-Client-Test (P-16); (2) Leistung auf Referenz-NAS und arm64; (3) Migrationstest aller DB-Stände M1a → 1.0; (4) Sicherheitsreview (Agent `security-reviewer`), Installationsanleitung de-CH; (5) Tag `v1.0.0`.
- **DoD (zusätzlich zu 6.2):** alle 64 Matrixzeilen grün oder mit Abweichung nur gemäss AN-4, AN-12, AN-13, AN-17, AN-7 (Prüfung mangels Gerät "ungeprüft", einzeln aufgeführt) und VAAPI/AMD "best effort, ungetestet" (3.5); P-1–P-16 erfüllt.

---

## 7. Abnahmematrix (alle 64 Spec-IDs)

"Mx" = Meilenstein, in dem die Zeile grün sein muss. E2E = Playwright gegen den gebauten Container. Fixtures gemäss 10.2.

| Spec-ID | Mx | Test oder Messgrösse |
|---|---|---|
| K-1 | M1a, M2 | E2E: Login mit E-Mail/Passwort, falsches Passwort → 401; Schema-Test: Profil hat kein E-Mail- und kein Passwortfeld; Pflichtfeld nur `name`, `image` optional |
| K-2 | M2 | E2E Admin: Profil anlegen, ändern, löschen; jede Stufe 0/6/12/16/18 setzbar; PIN setzen/entfernen; Bibliotheksrecht entziehen → Bibliothek in 10.2 unsichtbar |
| K-3 | M2 ff. | Vertragstest 10.2: 0 Lecks über alle Operationen, Profile P0–P18, positiv und negativ |
| K-4 | M2, M5 | Test: PIN verlangt beim Wechsel; Sperrkurve AN-11 mit Uhr-Mock; offline: Verifier lehnt falsche PIN ab |
| K-5 | M4, M5 | Test mit Uhr-Mock: Limit 30 min → Stopp innert 30 s nach Erreichen mit Hinweis; Bericht zeigt Minuten je Profil und Tag, Abweichung zu Roh-Ereignissen ≤ 1 min; offline (M5): Tagesrest 10 min → Stopp nach 10 min ±30 s im Flugmodus, Nachbuchung beim nächsten Kontakt (AN-18) |
| K-6 | M2 | E2E: Einladung → Gastkonto → eigenes Profil; Gast erhält 403 auf alle `x-visibility: admin`-Operationen und sieht nur freigegebene Bibliotheken; benutzter/abgelaufener Link → 410 |
| K-7 | M2, M5 | Test: nach Entzug 401 auf allen Sitzungen und Stream-Tokens innert 1 s; Downloads des Gastes beim nächsten Kontakt gelöscht (AN-4) |
| K-8 | M2 | Manuell: HTTPS-Domain mit gültigem Zertifikat (Caddy-Profil); automatisiert: 10.2 läuft direkt gegen die API ohne UI |
| B-1 | M1a | E2E: Bibliothek Typ Film und Serie anlegen; Schreibversuch des Server-Prozesses auf `/media` schlägt fehl (`:ro`, Test im Container) |
| B-2 | M1a, M3 | Namenskorpus ≥ 150 Pfadmuster + 20 NFO-Fälle: ≥ 98 % korrekt als Film/Serie/Staffel/Folge |
| B-3 | M1a, M2, M3 | Test: Profil de/en erhält Titel und Beschreibung in seiner Sprache, Fallback en; Poster für 25 Referenzfilme vorhanden |
| B-4 | M1a | Test ext4: neu/umbenannt/gelöscht sichtbar < 60 s; Umbenennung behält Fortschritt; Netzfreigaben gemäss AN-12 mit Messwert |
| B-5 | M3 | E2E: 4K- und 1080p-Datei desselben Films → ein Titel; Versionswahl im Player spielt die gewählte Datei (Stream-Info zeigt `media_file_id`) |
| B-6 | M3 | Test: manuelle Zuordnung übersteht 3 Voll-Scans und einen Metadaten-Refresh |
| B-7 | M3 | Fixture mit `Trailers/`, `Featurettes/`, `Deleted Scenes/`, `Behind The Scenes/`, Suffix `-trailer` → Extras am Titel, kein eigener Titel |
| B-8 | M3 | ≥ 90 % von 25 Referenzfilmen mit automatischem Trailer; Profil de und en erhalten verschiedene Trailer, wo beide existieren; manueller Ersatz übersteht Refresh |
| B-9 | M3 | Je präpariertem Fehlerfall (Duplikat, ohne Bild, verwaist, ffprobe-Fehler, ohne Freigabe) Eintrag in der Liste und Bereinigung nach Bestätigung |
| H-1 | M6 | Fixture M4B mit 12 Kapiteln und Ordner mit 10 MP3: Kapitelanzahl exakt, Startzeiten ±1 s; Autor, Sprecher, Cover angezeigt |
| H-2 | M6 | Lokaler Feed-Server: neue Episode erscheint < 65 min nach Veröffentlichung |
| H-3 | M6 | Fortschritt mitten im Kapitel geräteübergreifend ±2 s; Geschwindigkeit 0.5–3.0 einstellbar und je Titel gespeichert |
| H-4 | M6 | 10.2 umfasst Hörbuch- und Podcast-Operationen; Audio-Download trägt `allowed_profile_ids` |
| W-1 | M1a | Test: H.264/AAC-MP4 in Chrome → 0 ffmpeg-Prozesse, Antworten HTTP 206 |
| W-2 | M1b, M4 | Testmatrix 10.3; HDR → SDR gemäss P-6 auf NVENC und QSV |
| W-3 | M1b (gemessen), M4 | 4K-HEVC-HDR10 startet auf iPhone-Safari und Android-Chrome in < 5 s (Median aus 5, HW); Spulen < 3 s; Software gemäss AN-17 |
| W-4 | M4 | E2E: Tonspurwechsel während Wiedergabe mit ≤ 3 s Unterbruch; SRT/ASS als WebVTT; PGS in Firefox eingebrannt |
| W-5 | M1a | E2E mit zwei Browser-Kontexten: Fortsetzung ±10 s |
| W-6 | M4 | E2E: zwei Streams im Admin mit Profil, Gerät, Titel, Qualität, Modus (Direct/Remux/Transcode, HW/SW); Beenden stoppt Client innert 5 s, ffmpeg-Prozess endet |
| W-7 | M4 | Test: gemeldete Fähigkeiten erscheinen beim Gerät (Eigentümer, letzter Kontakt ±1 min, Container, Video-/Audio-Codecs, max. Auflösung, HDR) |
| W-8 | M4 | Test: max. 720p → Ausgabe ≤ 720p; max. 1 Stream → zweiter Start abgelehnt mit Hinweis; Limit 4 Mbit/s → Bitrate ≤ 4 Mbit/s; manuelle Qualitätswahl wechselt die Variante |
| W-9 | M4 | Intro-Erkennung ±2 s bei ≥ 80 % der Folgen einer Test-Staffel; Auto-Next nach 10-s-Countdown; Kapitelsprung ±1 s |
| W-10 | M3 | E2E: Trailer-Knopf startet YouTube-IFrame (`youtube-nocookie.com`); offline ausgeblendet |
| E-1 | M1a, M7 | Test: Reihenfolge Karussell, Weiterschauen, Neu hinzugefügt, Merkliste, Sammlungen, Genres |
| E-2 | M7 | E2E: Pick für alle und für ein Profil; Reihenfolge per Maus und Tastatur; Notiz sichtbar; vor Start und nach Ende des Zeitraums unsichtbar (Uhr-Mock) |
| E-3 | M7 | 10.2: FSK-18-Pick "für alle" fehlt bei P0–P16, erscheint bei P18 |
| E-4 | M7 | Test: 2 Zeichen finden Titel, Person, Sammlung; jeder Filter einzeln und kombiniert (Genre, Jahr, Freigabe, Medientyp, Verfügbarkeit lokal/Download); < 200 ms p95 bei 10 000 Titeln |
| E-5 | M7 | E2E: Merkliste von P1 und P2 getrennt; hinzufügen/entfernen; Reihe erscheint bzw. verschwindet |
| E-6 | M7 | Fixture TMDB-Reihe (3 Filme) → Sammlung automatisch; manuelle Sammlung durch Admin; Mitglieder und Zähler nur sichtbare Titel |
| E-7 | M7 | Trakt-Mock: nur Bibliothekstitel, max. 20, täglicher Lauf (Zeitplan-Test), AN-3 |
| E-8 | M7, M8 | Test: Favorit, neue Folge gescannt → In-App-Meldung beim Profil < 2 min; ab M8 Kanal-Mocks erhalten Nachricht mit Profilname (5.5) |
| E-9 | M7 | Test über alle Profil-Fixtures: ≤ 8 Reihen, keine leere; Umordnung durch Hauptbenutzer wirkt |
| O-1 | M5, M6 | E2E: Film, Folge, Hörbuch, Podcast-Episode in 3 Qualitätsstufen; Grösse ±10 % der angezeigten Schätzung |
| O-2 | M5 | E2E: Staffel mit 6 Folgen per 1 Aktion → 6 Downloads |
| O-3 | M5, M6 | Test: Folge n beendet → n+1 lädt; Max.-Anzahl eingehalten; neue Podcast-Episode lädt; "nur WLAN" auf Android-Chrome geprüft; iOS gemäss AN-13 |
| O-4 | M5 | Playwright offline + manuell Flugmodus (iPhone, iPad, Android, Desktop): Wiedergabe mit Untertitel und gewählter Tonspur |
| O-5 | M5 | Test: 10 min offline gesehen, Wiederverbindung → zweites Gerät zeigt Position ±10 s innert 10 s; Konfliktfälle AN-10 |
| O-6 | M5 (Start), M7 (abgehakt) | Schema-Test: kein Ablauffeld; Download nach 30 Tagen (Android, Desktop) bzw. 8 Tagen (iOS) abspielbar; nur Löschen durch Profil oder Entzug entfernt ihn |
| O-7 | M5 | Playwright: P6 sieht FSK-18-Download von P18 auf demselben Gerät nicht; Speicheranzeige = `estimate()` ±5 %; iOS-Hinweis sichtbar |
| C-1 | M1a, M5 | Lighthouse "installierbar"; Kaltstart im Flugmodus zeigt Offline-Bibliothek < 3 s |
| C-2 | M0 ff. | Lint 0 Verstösse; Admin und Zuschauer importieren dieselben `web/src/ui`-Komponenten (Import-Regel) |
| C-3 | M2 | Test: Schlüsselvergleich de/en = 0 fehlende Schlüssel; Profilwechsel wechselt die Sprache ohne Neuladen |
| C-4 | M1a ff. | Playwright-Tastatursuite je Seite (P-15) |
| C-5 | M0, M10 | Codegen-Drift = 0; Fremd-Client-Test (P-16) |
| A-1 | M1a, M8 | E2E frischer Stack: Konto, Bibliothek, GPU erkannt (NVENC auf Entwicklungsrechner, QSV auf NAS), TMDB-Schlüssel geprüft (ungültiger Schlüssel → Meldung) |
| A-2 | M8 | Playwright: jede Admin-Seite aus P-15 über eine Navigation erreichbar, besteht dieselben Lint- und Tastaturchecks; jede verwendete `web/src/ui`-Komponente hat eine Storybook-Story, deren Playwright-Screenshot höchstens 0.1 % Pixel vom freigegebenen Referenzbild abweicht |
| A-3 | M8 | Test: Dashboard zeigt Streams, Jobs (wartend/laufend/fehlgeschlagen), freien Speicher `/config` und `/cache`, Integrationsstatus, letzte 50 Fehler; provozierter Fehler erscheint < 10 s |
| A-4 | M0, M8 | Restore auf frischem Stack ergibt identische Konten, Profile, Fortschritte (Tabellen-Hash); Archiv enthält keine Mediendatei; Variante ohne Schlüssel verlangt Secrets neu |
| A-5 | M8 | Test mit Mock-Manifest: neuere Version → Hinweis, gleiche → keiner; abschaltbar |
| A-6 | M8, M9 | Kanal-Mocks je Ereignistyp (neue Inhalte, Fehler, Einladung, Ankündigung); manuell je 1 echtes Gerät pro Kanal; ab M9 ohne den Typ "Beschaffung" |
| A-7 | M8 | E2E: Ankündigung → beim nächsten Start sichtbar, nach Quittierung nicht mehr; Gast mit E-Mail-Opt-in erhält Mail (SMTP-Mock) |
| A-8 | M8 | Test mit Fixtures: meistgesehene Titel, Schauzeit je Profil, Transcoding-Anteil, Geräteverteilung stimmen mit Rohdaten überein |
| A-9 | M0, M8 | Log-Scan (`gitleaks`-Muster + Fake-Secrets) über einen kompletten E2E-Lauf: 0 Treffer |
| I-5 | M4 | Sidecar `.de.srt` ablegen → im Player < 60 s |
| I-7 | M7 | Trakt-Mock HTTP 503 → nur die zwei Reihen fehlen; Schema-Test: kein Trakt-Konto je Profil |
| I-8 | M9 | Integrationstest mit aufgezeichneten Radarr- und Sonarr-Nutzlasten: Import → Titel sichtbar < 60 s ohne Voll-Scan; Umbenennen behält Fortschritt; Löschen → Titel verschwindet; `Test` → 200 und Eintrag im Protokoll; unbekanntes Ereignis → 200 ohne Wirkung; falsches Token → 401 und Log-Scan ohne Treffer; Pfad ausserhalb jeder Wurzel → 422 mit beiden Pfaden im Protokoll; 11 Anfragen in einer Sekunde → 429; Playwright-Tastaturtest der Admin-Seite |

64 von 64 Zeilen haben einen konkreten Test oder eine Messgrösse. (Ursprünglich 68: I-1 bis I-4 und I-6 sind am 22. Oktober 2026 entfallen, I-8 ist dazugekommen.)

---

## 8. Abnahmekriterien des Plans (übergreifend)

| ID | Kriterium (testbar) | Spec-IDs | Prüfung |
|---|---|---|---|
| P-1 | `docker compose up` auf frischem Linux-Host: `healthy` in < 5 s; Images amd64 und arm64 | Constraints | CI + Referenz-NAS |
| P-2 | Leerlauf-RSS < 150 MB mit 5 000 Titeln; Image < 600 MB | Constraints | `docs/perf/` |
| P-3 | Vertragstest 10.2 grün in jedem Meilenstein ab M2 | K-3, E-3, H-4, O-7 | CI |
| P-4 | Änderungen < 60 s sichtbar gemäss AN-12; Scan-Abbruch bei fehlender Wurzel oder > 20 % Verlust | B-4, I-5, I-8 | Integrationstest |
| P-5 | Direct Play ohne ffmpeg-Prozess, wenn das Fähigkeitsprofil passt | W-1, W-7 | Prozesszähler |
| P-6 | HDR10 → SDR je Backend (NVENC, QSV, Software): (a) Referenzbilder je Backend werden einmal per Sichtprüfung freigegeben, danach Regression SSIM ≥ 0.95 an 10 Testframes; (b) gegen die Software-Referenz (bt.2390) mittleres ΔE2000 ≤ 5 nur auf Graustufenkeil und Hautton-Feldern, weil Hardware-Backends eigene Kurven nutzen; Start am Handy < 5 s (Median aus 5, HW) | W-2, W-3 | Skript in `testdata/` + Messung |
| P-7 | Spulen im laufenden Transcode und Remux < 3 s | W-3 | Playwright-Messung |
| P-8 | Fortschritt (Video, Audio, offline) geräteübergreifend ±10 s | W-5, H-3, O-5 | E2E zwei Kontexte |
| P-9 | Download im Flugmodus abspielbar; Prüfdauer 30 Tage (Android, Desktop; Start M5, Abschluss M7) und 8 Tage (iOS, innerhalb M5) ohne Öffnen | O-4, O-6 | E2E + Geräteprotokoll |
| P-10 | Startseite: Karussell + ≤ 8 Reihen, keine leere, LCP < 2.5 s (Lighthouse Mobile) | E-1, E-9 | Test + Lighthouse |
| P-11 | Suche ab 2 Zeichen < 200 ms p95 bei 10 000 Titeln; Startseiten-API < 300 ms p95, auch während eines Scans | E-4 | Benchmark |
| P-12 | Lint: 0 Farbwerte ausserhalb Tokens, 0 UI-Literaltexte, 0 Operationen ohne `x-visibility`, 0 `os/exec` ausserhalb `transcode` | C-2, C-3, C-5 | CI |
| P-13 | Sicherung + Wiederherstellung identisch; automatische Sicherung vor jeder Migration; Migrationstest aller DB-Stände grün; Sicherung mit Schlüssel nur mit Passphrase erzeugbar (Test) | A-4 | CI |
| P-14 | 0 Secrets in Logs über einen kompletten E2E-Lauf | A-9 | Log-Scan |
| P-15 | Tastatursuite (Pfeiltasten, Tab, Enter, Zurück per Escape/Backspace/Browser-Zurück; sichtbarer Fokus; keine Fokusfalle) besteht auf allen Seiten. **Zuschauer:** Profilwahl, PIN-Eingabe, Startseite, Detail Film, Detail Serie/Staffel, Video-Player, Audio-Player, Suche, Merkliste, Downloads, Wünschen, Profil-Einstellungen, Benachrichtigungen, Ankündigungen. **Admin:** Assistent, Dashboard, Bibliotheken, Konten und Profile, Gäste und Einladungen, Geräte, Streams, Integrationen, Cat's Empfehlungen, Startseiten-Reihen, Sammlungen, Pflegewerkzeuge, Benachrichtigungskanäle, Ankündigungen, Statistiken, Sicherung | C-4, A-2 | Playwright |
| P-16 | Fremd-Client nur aus `openapi.yaml` führt alle Zuschauerfunktionen aus: Login, Profilwahl mit PIN, Startseiten-Reihen, Suche mit Filtern, Detail, Merkliste, Favoriten, Abspielstart mit Fähigkeitsprofil, Abruf Direct Play/HLS mit Stream-Token, Spur- und Untertitelwahl, Fortschritt melden, Download vorbereiten und abrufen, Offline-Ereignisse hochladen, Benachrichtigungen und Ankündigungen lesen, Wünschen, Trailer-Info, Hörbuch-Kapitel, Podcast-Episoden | C-5 | Fremd-Client-Test M10 |

Alle 16 Kriterien haben ein Mass oder eine abschliessende Aufzählung.

---

## 9. Risiken und Gegenmassnahmen

| Risiko | W. | Wirkung | Gegenmassnahme |
|---|---|---|---|
| Transcoding-Edge-Cases (HDR10+, Dolby Vision, HW-Treiber, Interlaced, exotische Codecs, PGS) | hoch | hoch | Abschliessende Matrix 10.3; HDR-Pipeline je Backend (3.5); Messung auf NAS ab M1b; ausserhalb der Matrix → B-9 "nicht abspielbar" statt Blocker; DV über Basisschicht |
| PWA-Offline auf iOS (Löschung, Quote, Abbruch bei Sperre, kein Hintergrund) | mittel | hoch | Spike ab Tag 1 von M1a mit Schwellen 10.4; AN-13; bei No-Go native iOS-App direkt nach V1 (Spec "Constraints"), V1 wird nicht verzögert |
| YouTube-Nutzungsbedingungen / Kontingent | mittel | mittel | Nur IFrame-Player; TMDB zuerst; Tageskontingent im Job; fehlender Trailer blendet Knopf aus |
| Trakt/TMDB-Rate-Limits | mittel | mittel | Modul `ratelimit`, Backoff bei 429, Cache 24 h (Freigaben 7 Tage), Trakt 1 Lauf/Tag |
| Solo-Kapazität / Scope | hoch | hoch | Zeitbudget 6.1; **Schnittlinie** (zuerst verschiebbar nach 1.x, nur mit Zustimmung des Auftraggebers): 1. Intro/Abspann-Erkennung (W-9 teilweise), 2. Dolby-Vision-Sonderbehandlung, 3. Statistiken (A-8), 4. Telegram-Kanal (A-6 teilweise), 5. KI-Trailer-Anbieter; **Warnschwelle:** Ist-Aufwand kumuliert > 1.3 × obere Schätzung am Ende eines Meilensteins → Schnittlinie mit Auftraggeber besprechen; **Abbruch- und Neuplanungskriterium:** nach M4 kumuliert > 135 Arbeitstage (1.5 × 90) → V1-Umfang neu planen |
| Datenmodell-Migrationen | mittel | hoch | goose-Vorlage mit `NO TRANSACTION` und `foreign_key_check`; `VACUUM INTO` vor jedem Lauf ab M0; eingefrorene DB-Stände ab M1a in CI |
| Sichtbarkeitsleck (K-3) | mittel | hoch | View `visible_items`, `depguard`, Analyzer `itemsaccess`, Schema-Test `x-visibility`, Vertragstest 10.2, 404 statt 403; Freigaben nur per Trigger in derselben Transaktion plus nächtlicher Job `rating.verify` (3.2); Bilder nur über Item-ID mit Scope |
| Datenverlust durch abgehängte Freigabe | mittel | hoch | Scan-Abbruch bei fehlender Wurzel oder > 20 % Verlust; 24-h-Karenz; Benutzerdaten am Item |
| Überlast auf schwachem NAS | mittel | mittel | Admission Control, Prioritäten, `nice`/`oom_score_adj`; P-11 während Scan gemessen |
| Testhardware fehlt | mittel | mittel | AN-7 mit Ersatzweg; fehlende Prüfung wird als "ungeprüft" dokumentiert, nicht als erfüllt |
| TVDB-Lizenz / Schlüssel | mittel | niedrig | TMDB als Primärquelle auch für Serien; TVDB optional mit eigener Lizenz (AN-15); vor M3 klären |
| GPL-Pflichten für ffmpeg im Image | niedrig | mittel | Lizenzordner im Image, Link auf den passenden jellyfin-ffmpeg-Quellcode und schriftliches Quellcode-Angebot im Release |
| Aufwand des `stat`-Durchlaufs auf SMB/NFS | hoch | mittel | Adaptives Intervall (3.4), Grenzwert < 15 s für 10 000 Dateien, Webhooks primär (I-8, ab M9), AN-12 |
| SQLite-Queue: doppelte Ausführung durch abgelaufene Leases | niedrig | mittel | Lease ≫ längste Transaktion, Batches, idempotente Jobs (3.6) |

---

## 10. Verifikationsschritte

### 10.1 Je Meilenstein

1. `task ci` grün (Lint, Unit-, Integrations-, Vertrags-, Migrations-Tests, Codegen-Drift, Build).
2. Playwright-E2E gegen den gebauten Container.
3. Kinderprofil-Vertragstest (10.2) grün.
4. Messwerte in `docs/perf/<Mx>.md` (Median aus 5, Hardware angegeben).
5. Gerätecheckliste je Zielgerät (iPhone-PWA, Android-Chrome, iPad, Desktop Chrome/Firefox/Safari), je Punkt bestanden/nicht bestanden: PWA installieren; Login; Profilwechsel mit PIN; Startseite lädt < 3 s; Direct Play startet; Transcode startet; Spulen auf 50 %; Ton- und Untertitelwechsel; (ab M5) Download und Wiedergabe im Flugmodus; Tastatur bzw. Fernbedienungsersatz auf der neuen Seite.
6. Verifier-Agent prüft DoD gegen Belege.

### 10.2 Kinderprofil-Vertragstest (Pflicht für jede Operation)

- **Fixture:** je Medientyp (Film, Serie mit Staffeln und Folgen, Hörbuch, Podcast) Titel mit FSK 0/6/12/16/18 und einer ohne Freigabe; Extras, Trailer, Bilder, Untertitel je Titel; eine Sammlung mit gemischten Stufen; eine Person mit Rollen in FSK-6- und FSK-18-Filmen; Bibliothek X ohne Recht für "Kind-6". Profile **P0, P6, P12, P16, P18** und ein Profil ohne Recht auf X.
- **Markierung:** jede OpenAPI-Operation trägt `x-visibility: scoped | system | admin | public`; der Schema-Test schlägt fehl, wenn eine Operation keine Markierung hat oder eine Antwort mit `ItemRef` (auch verschachtelt) nicht `scoped` ist.
- **Antwort-Walk:** Für jede `scoped`-Operation (GET und POST, z. B. Suche) sammelt ein rekursiver Walk alle Item-IDs in der Antwort, auch in verschachtelten Listen (Startseiten-Reihen, Serie mit Folgen, Person mit Filmografie, Sammlung mit Titeln) und Zählern. Erfolg negativ: keine ID über der Profilstufe, keine ohne Freigabe (ausser P18), keine aus X. Erfolg positiv: Titel genau auf der Profilstufe (z. B. FSK 12 für P12) erscheinen, damit Off-by-one-Fehler auffallen.
- **ID-Endpunkte:** Mit der ID eines nicht sichtbaren Items liefert jede Operation HTTP 404: Detail, Stream-Start, HLS-Master- und Varianten-Playlist, Segmente, Direct-Play-Range, Untertitel, Download vorbereiten und abrufen, Bilder (`/items/{id}/images/...`; eine Auslieferung nur über Inhalts-Hash existiert nicht, Schema-Test prüft, dass keine Operation ohne Item-ID Bilder liefert), Trailer-Info, Merkliste/Favorit hinzufügen, Top-Pick-Detail.
- **Freigabe-Änderung:** Admin hebt die Serie aus der Fixture von FSK 12 auf 16 → ab der ersten Anfrage nach der Änderung sind Serie, Staffeln und Folgen für P12 in allen Listen unsichtbar und per ID 404; der Startseiten-Cache liefert sie nicht mehr; beim nächsten Sync fehlt P12 in `allowed_profile_ids` der betroffenen Downloads. Umgekehrt (16 → 12) erscheinen sie wieder. Gleiches für eine Folge mit eigener höherer Stufe: Die Serie bleibt sichtbar, nur diese Folge nicht.
- **Container-Verfügbarkeit:** Serie ohne verfügbare Folge, Podcast mit nur Feed-Episoden (Enclosure, keine Datei) und Staffel, deren Folgen alle über der Profilstufe liegen, verhalten sich gemäss 3.2/3.3 (Podcast sichtbar, die beiden anderen nicht).
- **Benachrichtigungen und SSE:** Ereignis `item.available` für einen FSK-18-Titel erzeugt für P6 weder In-App-Eintrag noch Kanalnachricht (Dispatcher-Mock); ein SSE-Abonnement von P6 empfängt kein Ereignis zu diesem Titel.
- **Offline:** Playwright prüft die Offline-Liste mit `allowed_profile_ids` für P6 und P18 auf demselben Gerät.
- **Admin-Operationen:** Gast- und Profilsitzungen erhalten 403.
- Neue Operationen sind automatisch erfasst, weil der Test die OpenAPI-Datei liest.

### 10.3 Wiedergabe-Testmatrix (M1b Teil, M4 vollständig)

| Datei (generiert) | Desktop Chrome | Desktop Firefox | Safari macOS/iOS | Android Chrome |
|---|---|---|---|---|
| H.264/AAC MP4 1080p | Direct Play | Direct Play | Direct Play | Direct Play |
| HEVC 10-bit SDR MKV | Remux oder Transcode je Fähigkeit | Transcode | Remux (`hvc1`) | je Gerät |
| HEVC HDR10 4K MKV | Transcode + Tonemap (≤ 1080p) | Transcode + Tonemap | Remux (HDR) oder Tonemap | Transcode + Tonemap |
| H.264 + DTS-Ton | Audio-Transcode | Audio-Transcode | Audio-Transcode | Audio-Transcode |
| H.264 + PGS | Einbrennen | Einbrennen | Einbrennen | Einbrennen |
| H.264 + SRT/ASS | WebVTT | WebVTT | WebVTT | WebVTT |

Jede Zelle: Unit-Test der Entscheidungslogik plus E2E-Start-Test (Safari iOS und Android manuell). Backends: NVENC und QSV getestet, VAAPI/AMD best effort, Software mit AN-17.

### 10.4 iOS-Offline-Spike (M1a Start, M1b Entscheid)

Gerät: installierte PWA auf iPhone (aktuelles iOS, wenn verfügbar zusätzlich Vorversion), ≥ 20 GB frei; Vergleich Android-Chrome.

| Prüfung | Schwelle |
|---|---|
| `navigator.storage.persist()` | true (in installierter PWA) |
| `navigator.storage.estimate().quota` | ≥ 5 GB |
| Schreibweg | `createSyncAccessHandle` im Worker oder `createWritable` funktioniert für 1.5 GB; gewählter Weg je iOS-Version dokumentiert |
| Download 1.5 GB mit Bildschirmsperre und App-Wechsel | abgeschlossen mit Wiederaufnahme ab letztem Block, ≤ 3 Wiederaufnahmen |
| Wiedergabe aus OPFS per Blob-URL und per Range-Antwort des Service Workers | Spulen auf 3 Positionen je < 2 s mit mindestens einem der beiden Wege |
| Download-Format (AN-20) | dieselbe Spulprüfung mit fragmentiertem MP4 (mit `sidx`) und mit `faststart`-MP4; bestanden mit fMP4 → AN-20 gilt, sonst Fallback `faststart` |
| Speicher während 30 min Wiedergabe | kein Neuladen oder Absturz der PWA |
| Bestand nach 1, 3, 8 Kalendertagen ohne Öffnen | Datei vorhanden und abspielbar |

**Go:** alle Schwellen erfüllt. **No-Go:** Datei fehlt nach 8 Tagen, `persist()` liefert auch in der installierten PWA false, Quote < 5 GB oder Spulen aus OPFS scheitert mit beiden Wegen → native iOS-App wird als erste Ergänzung nach V1 eingeplant, V1 liefert iOS-Offline mit Hinweis. Scheitert nur der Download bei Sperre: dokumentierte Abweichung (Download nur bei geöffneter App), kein No-Go.

---

## 11. Pre-Mortem

1. **"Transcoding hat V1 gefressen."** HDR auf dem NAS spielt nach einem Jahr noch falsch. *Gegenmittel:* abschliessende Matrix 10.3, NAS-Messung ab M1b, Software als dokumentierte Ausnahme, Schnittlinie (DV, Intro-Erkennung) greift bei der Warnschwelle.
2. **"Die Downloads auf dem iPhone sind weg."** *Gegenmittel:* Spike mit Schwellen ab Tag 1, ehrliche Speicheranzeige und Hinweis, native iOS-App nach V1 bei No-Go.
3. **"Das Kind sah einen FSK-18-Film."** Über eine Personenseite, eine Benachrichtigung oder eine HLS-URL. *Gegenmittel:* View + Lint statt Disziplin, Vertragstest über alle Operationen inkl. ID-Endpunkte, SSE und Benachrichtigungen, 404 statt 403, unbekannte Freigabe = gesperrt.

---

## 12. ADR

- **Decision:** Slouchcat V1 wird als ein Go-Server (`net/http` + chi, SQLite via `modernc.org/sqlite`, sqlc für statische Abfragen, Query-Builder in `visibility`, goose mit Sicherung vor jeder Migration, eigene Job-Queue) mit eingebetteter React/TypeScript-PWA (Vite, TanStack, react-aria-components, hls.js, OPFS) gebaut, spec-first über `api/openapi.yaml` mit `x-visibility`. Auslieferung als ein Docker-Container mit jellyfin-ffmpeg-Build (auch für Bilder), optional Caddy. Sichtbarkeit nur über die View `visible_items` mit per Trigger gepflegten Freigaben; offline schlägt der Client serverseitig berechnete `allowed_profile_ids` nach. Medien-Autorisierung über Stream-Token, deren Gültigkeit in `playback_sessions` liegt. Von den Arr-Diensten nur eingehende Webhooks (5.6), Ankündigungen über alle A-6-Kanäle, Trailer TMDB → YouTube → optional KI.
- **Drivers:** (1) Betriebsstabilität auf schwacher Hardware (40 %); (2) Prozess- und Streamingsteuerung (35 %); (3) Solo-Wartbarkeit mit Agenten (25 %).
- **Alternatives considered:** Option A TypeScript durchgehend (Punkte 3.85; stärkste Alternative wegen Driver 3, verworfen wegen Langzeit-Wartung von Node-LTS und Server-npm-Abhängigkeiten sowie grösserem Image; bleibt Rückfall nach 2.5). Option C .NET (4.10; grösseres Image, LTS-Zwang, Jellyfin-Nähe). Rust und Python invalidiert. PostgreSQL und Redis verworfen (Zusatzcontainer ohne Nutzen für einen Haushalt). `goqite` als Queue geprüft in M0 (verworfen, falls Priorität, Idempotenz oder Zeitpläne fehlen). Bildverarbeitung mit `govips` (CGO) oder `x/image` verworfen (CGO bzw. kein WebP, langsam auf ARM). Separater Transcode-Container verworfen zugunsten Admission Control (Prinzip 2). Seerr-iFrame und Erzeugen von Seerr-Benutzern verworfen. Signierter Profil-Snapshot offline verworfen (täuscht Sicherheit vor). Rekursive CTE in der View verworfen zugunsten von Triggern (Lesekosten jeder Liste). Ablaufzeit im Stream-Token verworfen (bricht lange Wiedergaben ab). Shaka Player zurückgestellt.
- **Why chosen:** Option B liefert das kleinste, langlebigste Betriebsartefakt und das beste Modell für nebenläufige Prozesse. Der Nachteil bei Driver 3 ist begrenzt, weil der Client keine Regeln implementiert und CI die Codegen-Drift prüft.
- **Consequences:** Zwei Sprachen und ein Codegen-Schritt je API-Änderung; eigener Query-Builder für dynamische Listen; SQLite mit einem Schreiber (Mandanten ausgeschlossen wie Spec "Non-Goals"); ffmpeg als GPL-Binary im Image mit Quellcode-Angebot; Offline-PIN und Offline-Schauzeit sind bewusste Client-Ausnahmen mit serverseitig gelieferten Werten; jede Segment-Anfrage kostet einen Primärschlüssel-Lesezugriff; Freigabe-Trigger müssen bei jeder Schema-Änderung an `items` mitgetestet werden; iOS-Offline bleibt von Apple abhängig; Software-Transcoding erfüllt W-3 nicht (AN-17).
- **Follow-ups:** Spike-Entscheid M1b; Rückfall-Prüfung 2.5 nach M1b; Bestätigung AN-1 bis AN-20 und 5.1–5.3 zu den Fristen in 5.7 (Abschnitt 13); Basis-Image `trixie-slim` in M0 verifizieren; Design-Brief mit Schrift und Palette in M1b; Projektlizenz festlegen; TVDB-Lizenz vor M3 klären; Kauf oder Leihe der Testhardware vor M1a.

---

## 13. Offene Fragen an den Auftraggeber

Was Marco bestätigen oder entscheiden muss, sortiert nach Frist. Ohne Rückmeldung gilt der Vorschlag des Plans.

1. **Vor M0 – Wo der Code liegt (AN-6):** Der Quellcode darf privat bleiben; das fertige Programmpaket und die Versionsinfo müssen aber öffentlich sein, damit dein NAS Updates ohne Passwort findet. Dazu: unter welcher Lizenz soll Slouchcat stehen?
2. **Vor M1a – Testgeräte (AN-7):** Stehen iPhone, iPad, Android-Handy und Mac zur Verfügung, und wird ein kleiner Intel-N100-Rechner (~150–200 CHF) als Muster-NAS gekauft? Ohne ihn bleibt "läuft gut auf schwachem NAS" ungeprüft.
3. **Vor M1a – Netzlaufwerke (AN-12):** Neue Dateien erscheinen garantiert innert einer Minute bei Festplatten direkt im Server oder wenn Radarr/Sonarr Slouchcat Bescheid geben. Bei Netzlaufwerken ohne diese Meldung kann es länger dauern. Einverstanden?
4. **Vor M1b – Ohne Grafikchip (AN-17):** Ohne Grafikkarte startet ein 4K-HDR-Film am Handy nicht in 5 Sekunden, sondern in bis zu 10 Sekunden, und nur in reduzierter Auflösung. Einverstanden?
5. **Vor M2 – Drei Grundsatzentscheide (5.1–5.3):** (a) Wünschen über eine eigene Slouchcat-Seite statt eingebetteter Seerr-Oberfläche, auch nicht übergangsweise; (b) Ankündigungen über alle Benachrichtigungswege, Gäste standardmässig per E-Mail; (c) Trailer zuerst aus TMDB, dann YouTube-Suche, KI nur optional.
6. **Vor M2 – Altersfreigaben (AN-1, AN-8, AN-11, AN-14):** Titel ohne bekannte Freigabe sind für alle Profile unter 18 gesperrt; ausländische Freigaben werden immer auf die nächsthöhere FSK-Stufe aufgerundet (z. B. CH 14 → FSK 16); nach drei falschen PINs wächst die Sperre von 30 s bis 15 min; Einladungslinks gelten einmal und 7 Tage.
7. **Vor M3 – Serien und Zugangsschlüssel (AN-9, AN-15):** Folgen erben die Freigabe der Serie und können nur strenger werden; jede Installation trägt eigene, kostenlose Schlüssel für TMDB, Trakt und YouTube ein; TVDB nur, wenn du dafür eine Lizenz hast.
8. **Vor M5 – Offline-Regeln (AN-4, AN-10, AN-13, AN-18, AN-19, AN-20):** Entzug eines Gastes löscht seine Downloads erst, wenn sein Gerät wieder online ist; auf dem iPhone laden automatische Downloads nur bei geöffneter App; das Schauzeit-Limit gilt offline mit dem Tagesrest vom letzten Kontakt; die PIN offline schützt nur vor neugierigen Mitbenutzern (für mehr Schutz: separate, längere Offline-PIN); bei Fortschritt von zwei Geräten gewinnt der neuere; vorbereitete Downloads belegen auf dem Server bis zu 50 GB und werden 24 h nach dem Laden gelöscht.
9. **Vor M6/M7 – Hörbücher, Podcasts, Trakt (AN-2, AN-3):** Altersstufe je Hörbuch-/Podcast-Bibliothek, Podcasts mit "explicit" gelten als FSK 18; die Trakt-Reihen zeigen die ersten 20 Titel aus deiner Bibliothek und verschwinden bei weniger als 5 Treffern.
10. **Vor M9 – Zuschnitt der Integrationen (I-8):** beantwortet am 22. Oktober 2026 (Entscheidungsprotokoll Frage 10). Von M9 bleibt nur der Webhook-Empfang; Statusanzeige, Benachrichtigung bei Verfügbarkeit, der Bereich "Wünschen" über Seerr und SABnzbd entfallen ersatzlos.

---

## 14. Changelog

### Nachtrag 22. Oktober 2026 – M9 auf den Webhook-Empfang zugeschnitten

Entscheid des Auftraggebers, protokolliert als Frage 10 in `docs/planning/04-entscheidungen.md`.

- **Gestrichen:** I-1 (Arr-Clients mit API-Schlüssel und Polling), I-2 (Beschaffungsstatus am Titel), I-3
  (Benachrichtigung bei Verfügbarkeit), I-4 (Bereich "Wünschen" über Seerr), I-6 (SABnzbd-Fortschritt),
  der Suchfilter "in Beschaffung" aus E-4, AN-5, AN-16 und Entscheid 5.1. Alles steht in den Non-Goals der
  Spec; Statusanzeige und Wünschen sind Kandidaten für die Ideensammlung.
- **Neu:** I-8 (Webhook-Empfang von Radarr und Sonarr mit Pfad-Umsetzung, Ereignisprotokoll und
  Dashboard-Kachel), Entscheid 5.6 ausgebaut, M9 neu zugeschnitten auf 2–3 Tage, Abnahmematrix 68 → 64 Zeilen,
  Gesamtaufwand 112–171 → 108–164 Arbeitstage.
- **Folgearbeit in M9:** der in M8 angelegte Benachrichtigungskanal "Beschaffung" wird entfernt, weil sein
  Erzeuger (I-1) entfallen ist.

### Iteration 2 (18. Oktober 2026)

Umgesetzt (C = Critic, A = Architect):

- C-1 (blockierend): Abnahmematrix für alle 68 Spec-IDs (Abschnitt 7); DoDs verweisen auf Matrixzeilen; alte P-22 entfällt.
- C-2 (blockierend) + A-1: Vertragstest auf alle Operationen inkl. verschachtelter Listen, POST-Suche, Seerr-Proxy, ID-Endpunkte, Benachrichtigungen, SSE; `x-visibility` mit Schema-Test; Profile P0–P18 positiv und negativ; View `visible_items`, `depguard`, Analyzer `itemsaccess`; `SystemScope` mit Allowlist (3.3, 10.2).
- A-Synthese: Go behalten; Prinzip 1 neu; `allowed_profile_ids`; O-5-Merge im Server aus Roh-Ereignissen; sqlc nur statisch, Query-Builder in `visibility`; signierter Profil-Snapshot gestrichen.
- C-3 + C-14 + A-14: Aufwand je Meilenstein in Arbeitstagen (6.1); Rückfallkriterium absolut (> 40 Tage bis Ende M1b) und nach M1b (2.5).
- C-4 + A-Antithese: Driver gewichtet, Driver 3 ausdrücklich gegen B abgewogen; einseitige Contras korrigiert (npm-Churn, sharp, Event-Loop, Image-Grösse); Bildverarbeitung über ffmpeg ohne CGO (A-11).
- C-5: Versionen durch Update-Regel ersetzt (Go eine der zwei neuesten, Node aktive LTS 24, Web-Bibliotheken aktuelle Hauptversion).
- C-6: AN-7 Testhardware mit Ersatzweg.
- C-7: Risiko Scope mit Schnittlinie, Warnschwelle und Neuplanungskriterium; neue Risiken Testhardware, TVDB-Lizenz, Seerr-Unterschiede, GPL, `stat`-Aufwand auf SMB, Datenverlust bei abgehängter Freigabe, Überlast, Lease-Doppelausführung; generisches Agenten-Risiko gestrichen (durch Lint- und Vertragsregeln ersetzt).
- C-8: Annahmen mit Frist; AN-8 bis AN-17 neu (Freigaben inkl. US-TV und BBFC, Vererbung, O-5-Regel, PIN-Kurve, B-4 auf Netzfreigaben, iOS-Auto-Download, Einladungsablauf, Schlüssel/TVDB, Seerr-Zuordnung, Software-Ausnahme); 5.1–5.3 bestätigungspflichtig; Seerr: zuordnen statt erzeugen.
- C-9 + A-5: iOS-Spike erweitert (Sperre, Schreibweg, Spulen aus OPFS, RAM) mit Go/No-Go-Schwellen (10.4); Start an Tag 1 von M1a.
- C-10: Tastaturtest als DoD jeder neuen Seite ab M1a (6.2).
- C-11: P-6 mit SSIM/ΔE, P-9 mit Prüfdauer, P-15 und P-16 mit abschliessenden Listen; Gerätecheckliste in 10.1.
- C-12: Klartext-Absatz 2.4.
- C-13: VAAPI/AMD als best effort, ungetestet (3.5, AN-7).
- C-Constraint (a) + A-15: M1 geteilt in M1a/M1b; Storybook und arm64 nach M1b; M0 verschlankt.
- A-2: Datenmodell-Abschnitt 3.2 (generische `items`, Datei-Identität, 24-h-Karenz, Scan-Abbruch).
- A-3: Seeking-Modell Transcode/Remux, Keyframe-Index, `hvc1` (3.5).
- A-4: HDR-Pipeline je Backend, Skalierung ≤ 1080p vor Tonemapping, Software-Ausnahme, NAS-Messung in M1b.
- A-6 + A-S1: Admission Control, Prioritäten, `nice`, `oom_score_adj`, W-8 an dieser Stelle (3.5).
- A-7: `VACUUM INTO` vor jeder Migration ab M0; DB-Stände ab M1a in CI.
- A-8: goose-Vorlage mit `NO TRANSACTION` und `foreign_key_check`; Regel in `CLAUDE.md`.
- A-9 + A-S3: Umfang der Queue festgeschrieben, goqite-Timebox, ADR-Alternative (3.6).
- A-10 + C-14: `statfs`-Erkennung, Warnung bei Netzfreigaben, Webhooks primär, adaptives Intervall mit Grenzwert, AN-12.
- A-12: Secrets at rest mit `/config/secret.key`, `writeOnly`, Sicherung mit/ohne Schlüssel (3.8).
- A-13: AN-6 vs. A-5 aufgelöst (öffentliches Paket und Release-Manifest), Cross-Compile per `GOARCH` (3.7).
- A-16: Freigaben-Abbildung vollständig mit Reihenfolge und Massenzuweisung (AN-8, AN-1).
- A-17: Modul `ratelimit`, Freigaben-Cache 7 Tage.
- A-18: Streaming-Endpunkte als `http.Handler`, Ausnahme in Prinzip 3 und `CLAUDE.md`.
- A-19: TVDB-Lizenz als Annahme AN-15 und Risiko.
- A-20: `statfs`-Prüfung von `/config` beim Start (3.7).
- A-S2: Offline-Ausnahme offen benannt (Offline-PIN), Synthese umgesetzt.
- A-S4: iOS-Hintergrund und "nur WLAN" als AN-13.
- Critic-Fragen: HLS-Autorisierung (5.4), Kinderprofil-Benachrichtigungen (5.5), Webhooks (5.6).

Begründet abgelehnt oder abgewandelt:

- A-Antithese "TypeScript durchgehend": nicht übernommen; Option B bleibt (Abwägung 2.3), A ist Rückfall nach 2.5.
- A-14 "Schätzung in Stunden": abgewandelt zu Arbeitstagen (Vorgabe des Koordinators, Solo-Planung in Tagen realistischer).
- C-9-Beispielschwelle "`persist()` false = No-Go": abgewandelt; gilt nur, wenn auch die installierte PWA false liefert, weil Safari ausserhalb der installierten App anders entscheidet.
- C-2 "Profile je Stufe 0/6/12/16": erweitert um P18, damit die Positivprüfung für unbekannte Freigaben und FSK 18 möglich ist.
- Spec-Empfehlung "Seerr-Einbettung als Übergang im Durchstich": nicht übernommen (5.1), bestätigungspflichtig.

### Iteration 2, Verbesserungsdurchgang nach Konsens (18. Oktober 2026)

Umgesetzt (C2 = Critic Iteration 2, A2 = Architect Iteration 2):

- A2-N1 (hoch): `effective_rating`, `available`, `min_child_rating` nur per Trigger in derselben Transaktion (rekursive CTE im Trigger), nächtlicher Job `rating.verify`, Invalidierung von `allowed_profile_ids` und Startseiten-Cache; Testfall "Freigabe der Serie ändern" in 10.2 (3.2, 3.3).
- A2-N2: Verfügbarkeit je `kind` (Blatt: Datei oder Enclosure; Container: verfügbares Kind, `min_child_rating`) (3.2, 3.3, 10.2).
- A2-N3: Stream-Token signiert nur Sitzungs-ID und Item; Ablauf und Widerruf in `playback_sessions`; `Referrer-Policy: no-referrer` (5.4).
- A2-N4: Admission-Control-Kosten nach Quelle und Pipeline; Herabstufung nur bei SDR; HDR wartet bis 10 s oder lehnt ab; Kalibrierung in M1b (3.5).
- A2-N5: Download-Format fragmentiertes MP4 (pausierbar, `sidx`), Fallback `faststart`; im Spike geprüft (AN-20, 10.4, M5).
- A2-N6: P-6 mit Referenzbildern je Backend plus ΔE nur auf Graustufen und Hauttönen (8).
- A2-N7 + Prinzipienverletzung 1: Schauzeit offline über Tagesrest als deklarierte Ausnahme 2 (Prinzip 1, 3.3, AN-18, Matrix K-5).
- A2-N8: Offline-PIN-Verifier nur auf Geräten mit Downloads, Erzeugung bei Online-Eingabe, separate Offline-PIN; Roh-Ereignisse nach 90 Tagen verdichtet (3.2, 3.3, AN-19).
- A2-N9: Stream-Token und Bildpipeline nach M1b; M4 nicht parallel zu M3; Schätzung neu 112–171 Arbeitstage, Neuplanungsschwelle 135 Tage (6.1, 9).
- A2-N10: FUSE als "unbekannt", Warnung erst bei Abweichung Watcher/Polling (3.4).
- A2-N11 + C2-N5: Akzentfarbe über `palettegen` statt Durchschnitt; Bilder nur über `/items/{id}/images/...` mit Scope und `Cache-Control: private, immutable` (3.1, 10.2).
- A2-N12: Abbruch bei > 20 % Verlust per "Verlust akzeptieren" im Dashboard aufhebbar (3.2).
- A2 Abschnitt 2: Sensitivitätszeile zur Stack-Abwägung (2.3); Rückfallkriterium präzisiert: nur `server`-getaggter Aufwand (> 26 Tage), Einstufung "Go-spezifisch" durch Code-Review-Agent nach Checkliste, Prototyp mit festem Umfang und 60-%-Schwelle (2.5).
- A2 Restpunkt Befund 2: fehlende Tabellen `accounts`, `profiles`, `devices`, `playback_sessions`, `progress_events` mit Meilenstein benannt (3.2).
- A2 Restpunkt Befund 12: Sicherung mit Schlüssel nur passwortgeschützt (age-Format) (3.8, M8, P-13).
- A2 Prinzipienverletzung 4: Klartext zu "rückholbar" mit Datenverlust seit Migration (Prinzip 4).
- C2-N1 (wichtig): AN-8 mit Abbildung für AT und CH, Aufrunden auf die nächsthöhere FSK-Stufe; AT/CH bleiben wegen Haushalt in der Schweiz und häufiger TMDB-Daten für deutschsprachige Veröffentlichungen.
- C2-N2 + Rest Befund 5: Basis `debian:trixie-slim`, Verifikation in M0, Rückfall je Architektur auf `bookworm-slim` begründet (3.7).
- C2-N3: K-1-Schema-Test korrigiert (kein E-Mail- und kein Passwortfeld).
- C2-N4: O-6-Langzeittest startet in M5, Abschluss M7 (Matrix, P-9, M5, M7).
- C2-N6 + Rest Befund 13: M10-Abweichungsliste um AN-7 "ungeprüft" und VAAPI best effort ergänzt.
- C2-N7: vorbereitete Downloads in `/cache/downloads`, 24-h-Aufräumregel, Budget 50 GB (AN-20, M5).
- C2 Stichprobe A-2: "Storybook-Checks" definiert (Screenshot-Abweichung ≤ 0.1 %).
- Abschnitt 13 neu: zehn Fragen an den Auftraggeber in Klartext, nach Frist sortiert.

Begründete Abweichungen:

- A2-N1: Trigger statt rekursiver CTE in der View gewählt (beide Varianten waren vorgeschlagen), weil jede Liste die View liest und P-11 (< 300 ms p95) sonst gefährdet ist; das Veralten wird durch Trigger in derselben Transaktion und `rating.verify` ausgeschlossen.
- A2-N5: Beide vorgeschlagenen Formate bleiben im Plan; fMP4 ist gesetzt, `faststart` nur Fallback nach Spike-Ergebnis.
- A2-N3: Ablauf "letzte Aktivität + 12 h" statt fester 4 h, damit eine pausierte Wiedergabe über Nacht nicht abbricht.

### Iteration 1

Erster Entwurf.
