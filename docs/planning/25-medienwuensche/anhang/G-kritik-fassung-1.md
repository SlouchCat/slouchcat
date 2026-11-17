# G – Kritik der Papiere Konzept 25 «Wünsche über Jellyseerr»

Geprüft am 17. November 2026: `docs/planning/25-medienwuensche/01` bis `11` und `README.md` im Worktree
`.claude/worktrees/konzept-25` (Anker `ba6efe31`). Massstab: Entscheid-Blatt F, Berichte A bis D, Code im Worktree,
`CLAUDE.md`. Nachgeschlagen wurden über 70 Belege `pfad:zeile`. Fast alle stimmen. Die Ausnahmen stehen in Befund 36.

## Urteil: REVISE

**Zählung: 40 Befunde, davon 1 Blocker, 13 Major, 26 Minor.**

Das Konzept ist breit, gut belegt und nah am Blatt. Die Jellyseerr-Aussagen halten sich an Bericht D: kein
Ablehnungsgrund, kein Sammelabruf, kein `since`, kein Fremdfeld, ein einziges Webhook-Ziel, `X-API-User` statt `userId`.
Vor einer Freigabe muss aber Folgendes behoben sein:

- eine Verletzung von Regel 7 im Vertragsentwurf;
- vier Widersprüche zwischen den Papieren, die Marco sonst falsch entscheiden lässt (Nr. 2 bis 5);
- vier Denkfehler, die zu falschen Zuweisungen oder Geisterbestellungen in Jellyseerr führen (Nr. 8 bis 12).

Leselegende: «Fassung gilt» nennt, welche Stelle massgeblich wird. Im Zweifel gilt das Entscheid-Blatt.

---

## Blocker

### 1. Blocker – `ItemRef` und scoped Bildadressen in Antworten, die nicht `scoped` sind (Regel 7)

- **Wo:** `08-datenmodell-api.md` 4.2 (`Wish.item?` = `ItemRef`, `Wish.external?` = `ExternalTitleRef` mit
  `/api/v1/titles/…/images` und `AdminWish` = `Wish` + …), 4.5 (`listAccountWishes` = `account`; `listAdminWishes` =
  `admin`), 4.6 (`getSpotlightEntries`: «`posterUrl` auch für fehlende»); `03-architektur.md` 15.2. Das Blatt nennt in
  Abschnitt 6 `listAccountWishes` selbst als `account`.
- **Was falsch ist:** `CLAUDE.md` Regel 7 sagt «Antworten mit `ItemRef` sind `scoped`». ADR 0005 Z. 40–42 sagt
  «`account`: keine Item-IDs in Antworten». Für Verwaltungsantworten gilt im Code die Regel «no ItemRef and no scoped
  URL (posters go through getAdminItemImage)» (`server/internal/api/contract_curation_test.go:30-43`). Dieselbe Prüfung
  steht in `contract_player_controls_test.go:80`. Der Vertrag fiele so durch den Schema- und Vertragstest.
- **Korrektur:**
  - (a) In `08` 4.5 `listAccountWishes` auf `scoped` mit `requirePrimary` (`server/internal/auth/profiles.go:40`)
    umstellen. Gleich anpassen: `03` 15.2, Zeile «`account`»; `08` 4.7, Gruppe «scoped» statt
    `TestContractAccountOperations`.
  - (b) `AdminWish` ohne `ItemRef`: neues Schema `AdminWishTitle {itemId?, media, tmdbId, title, year, posterUrl}`.
    Die Poster laufen über eine neue Admin-Route `getAdminExternalTitleImage` (`/admin/titles/{media}/{tmdbId}/images/…`,
    `admin`, `http.Handler`). Vorhandene Titel nutzen `getAdminItemImage`.
  - (c) In `08` 4.6 die Zeile «`getSpotlightEntries` `posterUrl` auch für fehlende» auf diese Admin-Route festlegen.
  - (d) In `08` 4.7 einen Eintrag ergänzen: «Verwaltungsantworten der Wünsche ohne `ItemRef` und ohne scoped Adresse
    (Muster `adminCurationAnswer`)».
  - Im Blatt die Zeile `listAccountWishes` entsprechend nachführen.

## Major

### 2. Major – Sieht ein Profil mit Recht, aber ohne Zuordnung, fehlende Titel? Drei Fassungen

- **Wo und was steht dort:**
  - `05-spotlight-integration.md` 10: «Profil ohne Wunschrecht, Konto ohne Zuordnung | keine fehlenden Titel (S-6)».
  - `04-benutzer-profilmapping.md` 9, vorletzter Punkt: «Profil mit Wunschrecht in einem Konto ohne Zuordnung sieht
    sie, aber der Knopf fehlt, mit Hinweis».
  - `11` F-9 sagt dasselbe wie `04`. Im selben Papier nennt F-8, Absatz «Andere Wahl», genau das als Nachteil.
  - `02` 4.5 nennt den Fall nur für den Knopf.
- **Fassung gilt:** `04`/`11` F-9. Das Blatt (Abschnitt 3) führt «Konto ohne Zuordnung» als Fall im Dialog bzw. am
  Knopf, das Profil sieht den Titel also.
- **Korrektur:**
  - `05` 10, Zeile ersetzen durch: «Profil ohne Wunschrecht | keine fehlenden Titel (S-6)». Neue Zeile: «Konto ohne
    Zuordnung | fehlende Titel erscheinen; Hauptknopf «Details» statt «Film anfordern», auf der Vorschauseite der
    Hinweis «Für dieses Konto ist noch kein Jellyseerr-Benutzer eingerichtet.»».
  - `05` S-6 um den Satz «Das Wunschrecht sind die Schalter; die Zuordnung entscheidet nur über den Knopf» ergänzen.
  - In `11` F-8, Absatz «Andere Wahl», den Halbsatz «ohne Zuordnung sähen Profile Titel …» streichen oder auf «ohne
    Wunschrecht» ändern.

### 3. Major – «Anfordern aus dem Spotlight» aus: keine fehlenden Titel oder nur «Details»?

- **Wo und was steht dort:**
  - `05` S-8: aus ⇒ `canRequest` falsch ⇒ «keine fehlenden Titel».
  - `07-administration.md` 7, Zeile «Anfordern aus dem Spotlight»: «aus: fehlende Titel im Spotlight zeigen nur
    «Details»».
  - `02` 4.5: «Medienart oder Stelle ausgeschaltet | Knopf fehlt an dieser Stelle».
- **Fassung gilt:** `05` S-8. Begründung: `05` S-6 sagt selbst «Ein Knopf ohne Wirkung wäre nur Werbung». Zwei
  Schalter («Fehlende Titel zeigen» und «Anfordern aus dem Spotlight») mit unterschiedlicher Wirkung auf dieselben
  Folien wären schwer zu erklären.
- **Korrektur:**
  - `07` 7, Spalte «Wirkung»: «aus: keine fehlenden Titel im Spotlight (Startseite und Spotlight-Liste); das Band
    «Fehlende Titel» zeigt den Hinweis».
  - `02` 4.5, Zeile teilen: «Spotlight aus: fehlende Titel erscheinen dort nicht; Suche/Detail aus: Abschnitt bzw.
    Hauptknopf fehlt».
  - In `05` 8, Band «Fehlende Titel», Spalte «Hinweis», ergänzen: «oder «Anfordern aus dem Spotlight» aus».

### 4. Major – Wann ist eine Staffel «verfügbar»? Zwei Regeln

- **Wo und was steht dort:**
  - `03` 8.1 (letzter Absatz) und Diagramm 9.3: `partial`, «sobald mindestens eine [Staffel] abspielbar ist». Eine
    eingelesene Folge genügt.
  - `06` 3.3: Die Staffel braucht sichtbare Folgen **und** dazu entweder Jellyseerr `AVAILABLE` oder alle
    ausgestrahlten Folgen in `series_catalog`. Bis Stufe 4 «gilt nur die Meldung aus Jellyseerr».
  - `02` 2 und 4.7.6 folgen `03`.
- **Folge:** Nach `03` geht «Staffel 2 ist verfügbar» bei der ersten von zehn Folgen hinaus. Das ist falsch gegenüber
  Marcos Auftrag («tatsächlich abspielbar»).
- **Fassung gilt:** `06` 3.3. Sie ist mit Blatt 2.8 vereinbar, weil Jellyseerr nur Zusatzbedingung ist.
- **Korrektur:**
  - In `03` 8.1 ersetzen durch: «Serie → Staffel gilt als verfügbar nach `06-benachrichtigungen.md` 3.3; `partial`,
    sobald mindestens eine gewünschte Staffel so verfügbar ist; `available`, wenn alle».
  - Diagramm 9.3, Zeile «M->>W: WithMatched Serie, Staffel 1 eingelesen» ergänzen: «Staffel 1 vollständig (Sonarr
    meldet AVAILABLE)».
  - `02` 2, Zeile `partial` auf «… vollständig abspielbar» ändern.
  - In `08` 2.5 dokumentieren, woher Stufe 2 den Staffelstatus nimmt: `media.seasons[].status` aus dem Abgleich.

### 5. Major – «strengster Wert» widerspricht ADR 0034 und dem Code

- **Wo:** Blatt 2.7; `02` 7; `03` 3, letzter Absatz; `05` S-7; `11` ADR 0038, Entscheid 2; `10` AK-28 («Freigabeland
  CH, Freigaben DE 12, US 16 → strengster Wert gilt»).
- **Was falsch ist:** ADR 0034 Z. 31 liest in der Reihenfolge gewähltes Land → DE → AT → CH → GB → US. «Strengster
  Wert» gilt dort nur **über die Anbieter**. Der Code nimmt das **erste** Land mit bekanntem Wert
  (`server/internal/metadata/ratings.go:113-125`, `ageFromCertifications`).
- **Folgen:**
  - AK-28 verlangt 16, der Code liefert 12.
  - Ein fehlender Titel bekäme eine andere Freigabe als derselbe Titel in der Bibliothek. Genau das erzeugt die Lücke,
    die `03` 8.3 dann mit einer Sieben-Tage-Regel flickt.
- **Korrektur:**
  - Überall diesen Wortlaut setzen: «Freigabe aus TMDB, gelesen in der Länderreihenfolge nach ADR 0034 (erstes Land mit
    bekanntem Wert, dieselbe Funktion wie für Bibliothekstitel); unbekannt = 18».
  - `10` AK-28 ändern: «Freigabeland CH ohne Wert, DE 12, US 16 → 12 (wie bei einem Bibliothekstitel)».
  - In `08` 2.9 festhalten: `min_age` rechnet `metadata` mit `ageFromCertifications`.
  - Im Blatt 2.7 dasselbe nachführen.

### 6. Major – Wunschmeldungen ohne `item_id` verschwinden aus dem Eingang

- **Wo:** `06` 8, erster Punkt («Eingangseinträge … laufen über `visible_items`») und 11, Zeile `notifications`;
  `08` 2.11.
- **Was fehlt:** Die heutige Liste, die Zählung und «gelesen» verbinden den Eingang **innen** mit `visible_items` über
  `item_id` (`server/queries/system/notify.sql:33-52`). Die Papiere machen `item_id` nullable. Damit fielen
  `wish_approved`, `wish_declined` und `wish_failed` zu fehlenden Titeln aus Liste und Glockenzähler. Keine
  Leseabfrage ist geplant.
- **Korrektur:** In `06` 11 eine Zeile «Lesen» ergänzen: «`NotifyList`, `NotifyCount`, `NotifyMarkRead` neu:
  Einträge mit `item_id` wie heute über `visible_items`; Einträge ohne `item_id` über `wish_id` und die
  Altersprüfung fehlender Titel aus `visibility` (`MissingTitle` bzw. `external_titles.min_age`); nicht sichtbar heisst
  nicht gelistet». Dazu in `09` 2.6 einen DoD-Punkt und in `10` ein neues AK: «Eingang zeigt «Abgelehnt» zu einem
  fehlenden Titel und zählt ihn in der Glocke».

### 7. Major – Abgleich kann sich festfahren (40-Seiten-Grenze ohne Fortschritt)

- **Wo:** `03` 7.1 («höchstens 40 Seiten je Lauf, dann Abbruch ohne Zeiger vorzurücken»; beim ersten Lauf ist
  `seit` leer), 7.2; `09` 9, Zeile «Teure Abgleichsaufrufe».
- **Was falsch ist:** Eine Instanz mit mehr als 2000 Requests (40 × 50) schliesst den ersten Lauf nie ab. Der Zeiger
  bleibt leer, und jeder Lauf alle 5 min liest wieder 40 Seiten. Jede Seite lässt Jellyseerr alle Radarr und Sonarr
  abfragen (D 6.1). Dasselbe passiert nach langem Ausfall. AK-53 prüft nur den eingeschwungenen Zustand.
- **Korrektur:** `03` 7.1 so ändern:
  - (a) Der erste Lauf vor Stufe 4 setzt `seerr.last_sync := t0`, ohne zu blättern. Bekannte Wünsche gibt es noch
    keine.
  - (b) Für die Übernahme (Stufe 4) und für lange Ausfälle gibt es einen eigenen, fortsetzbaren Durchlauf mit
    gespeichertem `skip` und eigenem Zeiger. Er rückt je Seite vor und bricht nach 40 Seiten ab, setzt im nächsten Lauf
    aber dort fort.
  - (c) Neues AK: «3000 Requests, erster Lauf: Abgleich endet nach höchstens 2 Läufen mit gesetztem Zeiger».

### 8. Major – Übernahme bestehender Anfragen (F-21) geht mit dem beschriebenen Abgleich nicht

- **Wo:** `04` 8.2 Punkt 5 («beim ersten Speichern einer Zuordnung und danach bei jedem Abgleich»; Rückfrage nennt
  die Zahl «3 Anfragen von grosseltern»); `03` 7.3; `09` 4.6; `10` AK-88.
- **Was falsch ist:** Der inkrementelle Abgleich bricht beim ersten `updatedAt` älter als der Zeiger ab (`03` 7.1).
  Alte Anfragen eines **neu** zugeordneten Benutzers sieht er nie. Die Zahl in der Rückfrage lässt sich so nicht
  bestimmen. Ebenso fehlt der Weg für schon geführte «nicht zuordenbare» Wünsche: Sie laufen in den Zweig «bekannt →
  Zustand ableiten», nicht in «übernehmen».
- **Korrektur:** In `03` 7.3 einen Ablauf ergänzen: «Beim Speichern einer Zuordnung reiht `wishes` den Job
  `wishes.takeover:<seerrUserId>` ein: `GET /request?requestedBy=<id>&take=50&skip=n` ohne `X-API-User` (Benutzer 1 hat
  `MANAGE_REQUESTS`, Bericht D 6.1), fortsetzbar wie Nr. 7. Er übernimmt offene und seit 30 Tagen verfügbare Requests
  und hängt bestehende Wünsche ohne `wish_requesters` an das Zielprofil. Die Rückfrage zählt über denselben Aufruf mit
  `take=1` (`pageInfo.results`)». Dazu je eine Zeile in `08` 5, `09` 4.6 und `13` der Jobtabelle in `03`.

### 9. Major – Serien: ein Wunsch kann mehrere Jellyseerr-Requests haben, das Modell kennt nur einen

- **Wo:** `08` 2.4 (`wishes.seerr_request_id` NULL, **UNIQUE**), 2.5 (`wish_seasons` ohne Request-ID); `03` 6.2 bis
  6.4 (202 «anhängen», Vorprüfung nur «mit allen gewünschten Staffeln»).
- **Was falsch ist:** Jellyseerr filtert schon angefragte Staffeln still heraus und legt für den Rest einen eigenen
  Request an (D 4). Wünscht ein Profil Staffel 2 bis 4 und hat jemand in Jellyseerr direkt Staffel 3 angefragt
  (Slouchcat weiss es vor Stufe 4 nicht), dann:
  - gehört Staffel 3 zu Request A und Staffel 2 und 4 zu Request B;
  - verfolgt Slouchcat nur einen davon;
  - zeigt Slouchcat den Zustand von Staffel 3 falsch und kann sie nicht zurückziehen;
  - kann es bei 202 «anhängen» gar keinen Request setzen. Ein zweites Anhängen an denselben Request verletzt UNIQUE.
- **Korrektur:**
  - `08` 2.5: Spalte `seerr_request_id INTEGER NULL` je Staffelzeile. `wishes.seerr_request_id` bleibt nur bei
    Filmen Pflicht. Das UNIQUE an `wishes.seerr_request_id` entfällt; dafür UNIQUE (`seerr_request_id`,
    `season_number`) an `wish_seasons`.
  - `03` 6.3, Schritt 2: «Vorprüfung liest `mediaInfo.requests[].seasons`; Staffeln eines offenen fremden Requests
    werden lokal Mitwunsch (Zeile mit dessen Request-ID, kein POST); nur der Rest geht in den POST».
  - `03` 6.4, Zeile 202: «je Staffel den Request suchen und setzen».

### 10. Major – Zurückziehen erzeugt Geisterbestellungen

- **Wo:** `03` 6.6 («`queued`: lokal `withdrawn`, Job wird abgebrochen, kein Aufruf»; «`pending`: … 204 oder 404 →
  `withdrawn`»); `02` 4.8.
- **Was falsch ist:**
  - (a) Der Sofortversuch kann nach 3 s abbrechen, obwohl Jellyseerr den Request angelegt hat. Genau diesen Fall nennt
    `03` 6.5, Zeile «Handler». Zieht das Profil vor dem nächsten Versuch zurück, bleibt der Request in Jellyseerr. Bei
    Auto-Freigabe beschafft Radarr den Titel.
  - (b) Ein laufender `wishes.submit` lässt sich nicht «abbrechen», der POST kann schon unterwegs sein.
  - (c) `DELETE` mit `X-API-User` antwortet 401 bei einem fremden Request (Wunsch per 409 angehängt) oder wenn
    Jellyseerr inzwischen freigegeben hat (D 8). Dafür gibt es kein Verhalten.
- **Korrektur:** `03` 6.6 so ersetzen:
  - «`queued`: Zustand `withdrawn` mit Merker `withdraw_pending`. Hat der Wunsch je einen Sendeversuch gehabt, prüft
    `wishes.submit` per Vorprüfung, ob ein Request des eingefrorenen Benutzers besteht, und löscht ihn (`DELETE` mit
    `X-API-User`).»
  - «`Submit` prüft nach dem POST in derselben Transaktion, ob der Wunsch noch `queued` ist; sonst sofort `DELETE`.»
  - «Ist der Request fremd (`requestedBy.id` ≠ `seerr_user_id`), zieht nur Slouchcat zurück (wie Mitwunsch).»
  - «401 → Zustand neu lesen; ist er `APPROVED`, bleibt der Wunsch `processing` mit Hinweis «Schon freigegeben».»
  - Dazu ein AK: «Zeitlimit beim Sofortversuch, Request kam an, Profil zieht zurück → kein Request in Jellyseerr».

### 11. Major – Wechsel der Jellyseerr-Instanz weist Wünsche falschen Personen zu

- **Wo:** fehlt überall. Am nächsten dran sind `04` 8 («Schutz vor falscher Zuweisung») und `07` 4.1 (Adresse
  änderbar).
- **Was fehlt:** Die Zuordnung hängt an `seerr_id`. Zeigt der Administrator auf eine andere oder neu aufgesetzte
  Instanz, sendet Slouchcat mit `X-API-User: 3` an eine ganz andere Person. Bei SQLite-Rowids können nach dem Löschen
  der höchsten ID sogar IDs neu vergeben werden. Ebenso zeigen `seerr_request_id` und `seerr_media_id` ins Leere oder auf
  fremde Requests. Anforderung (3) «nie falsch zuweisen» ist damit nicht gedeckt.
- **Korrektur:** In `04` einen neuen Abschnitt 8.4 «Instanz wechselt» einfügen:
  - Slouchcat speichert einen Fingerabdruck der Instanz (`seerr.instance` = E-Mail und `createdAt` von Benutzer 1 aus
    `GET /auth/me`).
  - Ändert er sich, oder ändert eine bekannte `seerr_id` beim Laden ihre E-Mail bzw. `createdAt`, sperrt Slouchcat
    das Senden («Wünsche aktiv» aus, Warnung «Andere Jellyseerr-Instanz»).
  - Die Zuordnungen kommen in den Zustand «prüfen». Offene Wünsche laufen nicht weiter, bis der Administrator
    bestätigt.
  - Dazu `seerr_users.created_at` in `08` 2.1, die Prüfung in `03` 15.1 und ein AK in `10`.

### 12. Major – Datenschutz: das Hauptprofil liest die Wünsche von PIN-Profilen

- **Wo:** `02` 5.3 («Das Hauptprofil sieht über `listAccountWishes` auch die Wünsche aller Profile»); `08` 4.5;
  `06` 7, Zeile «Mitwunsch» (Bündelung «[Marco, Noah]»).
- **Was falsch ist:** F-15 begründet die PIN-Regel mit «Wer eine PIN setzt, will nicht, dass andere im Haushalt
  mitlesen». `listAccountWishes` zeigt dem Hauptprofil aber Titel, Zustand und Verlauf der Wünsche von Sandra (PIN). Ist
  das Hauptprofil selbst ohne PIN, sieht das jedes Kind am Gerät. Anforderung (5) verlangt eine saubere Empfänger- und
  Datenschutzlogik.
- **Korrektur:** In `02` 5.3 und `08` 4.5 festlegen: «`listAccountWishes` zeigt Wünsche von PIN-Profilen ohne Titel
  und Bild (`hidden: true`, «Wunsch von Sandra»), solange die Einstellung «Externe Nachrichten bei PIN-Profilen ohne
  Titel» an ist.» Diese Einstellung umbenennen in «Wünsche von PIN-Profilen ohne Titel zeigen (Nachrichten und
  Kontoübersicht)». Dazu ein AK.

### 13. Major – Suche nach fehlenden Titeln: Altersprüfung braucht je Treffer einen Detailabruf

- **Wo:** `02` 5.2 («gefiltert: … ohne Titel über der Altersstufe»); `03` 16, Zeile «Suche» («TMDB-Suche live; 10 min
  im Speicher»); `08` 4.5 `searchExternalTitles`.
- **Was fehlt:**
  - Die TMDB-Suche liefert keine Freigaben. Ohne Detailabruf (`release_dates` bzw. `content_ratings`) ist jede
    Freigabe unbekannt, also 18. Ein Profil unter 18 sähe nie einen Treffer, oder der Server müsste je Treffer TMDB
    fragen (20 Treffer = 20 Aufrufe unter dem Limiter von 20/s).
  - Offen ist auch, ob der 10-min-Speicher die rohen Treffer hält und erst danach je Profil filtert. Nur dann ist er
    für Kinder sicher.
- **Korrektur:** In `03` 16 ergänzen:
  - «Treffer ohne Eintrag in `external_titles` holt der Server je Suche gebündelt (höchstens 20, parallel unter dem
    TMDB-Limiter, Zeitlimit 3 s). Sie landen in `external_titles`. Was im Zeitlimit nicht kam, zählt als unbekannt und
    fehlt bei Profilen unter 18.»
  - «Der Speicher hält rohe TMDB-Treffer; der Altersfilter läuft danach je Profil.»
  - Dazu ein Messwert in `10` 7 (Suche p95 mit Profil 12).

## Minor

### 14. Minor – 403 «unbekannter Benutzer» und «falscher Schlüssel» sind nicht zu unterscheiden, und `03` widerspricht sich

- **Wo:** `03` 6.4 («403 … `X-API-User` unbekannt → bleibt `queued`») gegen `03` 14 («noch nicht übermittelte
  Wünsche `failed`, `seerr_user_gone`») und `04` 8.1 («`failed` mit Grund «Jellyseerr-Benutzer fehlt»»).
- **Was falsch ist:** Beide Ursachen liefern dieselbe Meldung (D 2.1).
- **Korrektur:** In `03` 6.4 ersetzen: «403 ohne bekannte Meldung → `GET /auth/me` ohne Header. 200 → Benutzer fehlt
  → `failed`, `seerr_user_gone`. Sonst Schlüssel ungültig → bleibt `queued`.» `03` 14 und `04` 8.1 gelten.

### 15. Minor – Rückfüllung von `wish_review`: drei Zeitpunkte

- **Wo und was steht dort:**
  - `06` 6: Die Migration (Stufe 2) ergänzt `wish_review` bei Administratoren.
  - `08` 2.11: «sobald es in Stufe 4 einen Erzeuger hat … je Stufe».
  - `09`: Stufe 4 hat keinen Schritt dafür.
- **Fassung gilt:** `08`, nach ADR 0009 «kein Ereignis ohne Erzeuger».
- **Korrektur:**
  - `06` 6: «`wish_review` kommt mit Stufe 4 dazu».
  - `09` 4.3: «Datenmigration: `wish_review` in die Ereignislisten der Administratorkonten; `ChannelEnsureDefaults`».
  - `10` AK-63 auf bestehende Konten erweitern.

### 16. Minor – ADR 0037 «je Stufe additiv» gegen `08` 4.6 «brechend»

- **Wo:** `11` 4, ADR 0037, Kopf «Vertrag und Migrationen».
- **Korrektur:** Ersetzen durch «Vertrag und Migrationen: vergibt der Bau; Stufe 1 nicht brechend, Stufen 2 bis 4
  brechend (`08-datenmodell-api.md` 4.6)».

### 17. Minor – Fehlercode bei zu vielen Wünschen

- **Wo:** `03` 15.4 nennt 429 `rate_limited`, `08` 4.1/4.4 und `10` AK-41 nennen `too_many_attempts`.
- **Fassung gilt:** `too_many_attempts`, wie im Vertrag (`api/openapi.yaml:261`).
- **Korrektur:** `03` 15.4 anpassen.

### 18. Minor – Als welches Profil fordert der Administrator in der Spotlight-Verwaltung an?

- **Wo:** `05` 6 sagt «aktives Profil des Administrators», `07` 9 sagt «Hauptprofil».
- **Fassung gilt:** «Hauptprofil». Die Verwaltung verlangt ohnehin das Hauptprofil (`api/openapi.yaml:9-16`), beide
  Aussagen fallen also zusammen.
- **Korrektur:** In `05` 6 «aktives Profil (= Hauptprofil)» schreiben.

### 19. Minor – Freigeben als Benutzer 1 oder als zugeordneter Administrator?

- **Wo:** `04` 2.2 sagt «Freigeben, Ablehnen und Wiederholen» laufen als Benutzer 1. `08` 4.5 (Vorschlag) sagt
  «`X-API-User` des Jellyseerr-Benutzers des handelnden Administratorkontos, sonst Benutzer 1».
- **Fassung gilt:** `08`, weil Jellyseerr dann `modifiedBy` richtig führt.
- **Korrektur:** `04` 2.2 nachziehen.

### 20. Minor – Stufe der Eingangsmeldung «Abgelehnt»

- **Wo:** `02` 4.6 Schritt 3 nennt die Meldung ohne Stufe. In `06` 2, `09` 4.3 und `10` AK-83 kommt sie in Stufe 4.
- **Korrektur:** In `02` 4.6 Schritt 3 ergänzen: «(ab Stufe 4; vorher zeigt nur «Meine Wünsche» den Zustand)».
  Gleich in `02` 4.4 Schritt 5 für `failed`.

### 21. Minor – Zwei Zustandsdiagramme, verschieden

- **Wo:** `02` 2 fehlen gegenüber `03` 8.2 diese Übergänge:
  - `queued/failed/declined → available` (auf anderem Weg);
  - `pending → failed`;
  - `partial → failed`;
  - `failed → processing` (Wiederholen).
- **Korrektur:** `02` 2 durch das Diagramm aus `03` 8.2 ersetzen, oder dort auf `03` 8.2 verweisen.

### 22. Minor – «aufheben» gegen «Ausnahme» bei Spec-Zeile 46

- **Wo:** `09` 0.1 und `10` AK-1 sagen «hebt Spec `:46`, `:66`, `:67` auf». `11` 2.1 und 3 sagen «ändert `:35`,
  `:46`; hebt `:66`, `:67` in Teilen auf». Das Blatt spricht von «Ausnahme».
- **Korrektur:** `09` 0.1 und `10` AK-1 auf den Wortlaut von `11` 3 bringen.

### 23. Minor – «Erneut anfordern» nach `arr_failed` führt in eine Schleife

- **Wo:** `03` 8.1, letzter Absatz (neuer Wunsch für `failed`).
- **Was falsch ist:** Ein FAILED-Request in Jellyseerr sperrt den Titel (nur DECLINED/COMPLETED geben frei, D 3.5).
  Der neue POST bekommt 409, Slouchcat hängt sich an den FAILED-Request, und der Wunsch steht wieder auf `failed`.
- **Korrektur:** «Bei `fail_reason = arr_failed` fehlt «Erneut anfordern»; der Zuschauer sieht «Ein Administrator kann
  den Wunsch wiederholen»; `retryWish` ruft `/retry`.»

### 24. Minor – Einzelprüfung trifft jeden Lauf dieselben Wünsche

- **Wo:** `03` 7.1 («`GET /request/{id}` für offene Wünsche ohne Änderung seit 24 h, höchstens 50»).
- **Was falsch ist:** Ein Wunsch, der wochenlang auf Freigabe wartet, wird alle 5 min erneut geprüft. Das sind bis zu
  14 400 Aufrufe am Tag.
- **Korrektur:** Spalte `wishes.checked_at` und die Regel «höchstens einmal je 24 h je Wunsch».

### 25. Minor – Sofortversuch mit zwei Jellyseerr-Aufrufen in 3 s

- **Wo:** `03` 6.3 Schritt 2 und 9.1.
- **Was falsch ist:** Vorprüfung und POST lösen beide TMDB-Abrufe in Jellyseerr aus (D 3.5, 6.2). Das Zeitlimit wird
  oft reissen, und die Fälle mit verlorener Antwort häufen sich. Beim ersten Versuch ist die Vorprüfung unnötig:
  Jellyseerr meldet ein Doppel selbst mit 409/202.
- **Korrektur:** «Vorprüfung nur ab dem zweiten Versuch (`attempts > 0`)».

### 26. Minor – Prioritäten gegen die Regel der Queue

- **Wo:** `03` 13.
- **Was falsch ist:** `server/internal/jobs/queue.go:11-29` verlangt, dass ein täglicher kurzer Job über den
  Massensprossen (6 und tiefer) steht, und dass die Leiter dort nachgeführt wird. `titles.refresh` (2) und
  `series.catalog` (2) warten bei einem Bibliotheksimport hinter zehntausenden Jobs.
- **Korrektur:** `titles.refresh` und den Zeitplanlauf von `series.catalog` auf 7 setzen, die Ereignisläufe auf 4. In
  `09` 2.2 und 4.1 «Leiter in `queue.go` nachführen» ergänzen.

### 27. Minor – Ort des Hakens `WithMatched`

- **Wo:** `03` 11 («an derselben Stelle wie `relinkQueued`»).
- **Was falsch ist:** `relinkQueued` läuft nicht, wenn `relink` falsch ist (`match.go:318-321`). Bei Serien läuft es
  vor `matchSeasons` (`match.go:300`, Staffeln danach). Bei einem Zwilling endet der Match dort (`:191`, `:300`).
- **Korrektur:** «`WithMatched` ruft der Match nach `relinkQueued`, wenn kein Zwilling gemeldet wurde, auch bei
  `relink = false`; bei Serien nach `matchSeasons`».

### 28. Minor – Doppelschutz hängt an Zeilen, die nach 90 Tagen gelöscht werden

- **Wo:** `06` 7, Zeile «Zustand kippt zurück» («weil der Schlüssel besteht»).
- **Was falsch ist:** Gelesene Eingangseinträge räumt Slouchcat nach 90 Tagen weg (Bericht B 5.3 Punkt 10). Danach
  fehlt der Schlüssel.
- **Korrektur:** «Massgeblich ist `wish_requesters.notified_at` (bzw. ein Feld je Staffel); der `dedupe_key` schützt nur
  gegen gleichzeitige Läufe.» `notified_at` aus «Vorschlag» in die Grundlage heben.

### 29. Minor – Der «stündliche Test» hat keinen Job

- **Wo:** `03` 14, Zeile «Schlüssel ungültig» («keine weiteren Aufrufe ausser dem stündlichen Test»); `10` AK-4.
- **Korrektur:** In `03` 13 festlegen, dass `wishes.sync` im Zustand «Schlüssel ungültig» nur `GET /auth/me` ruft.
  Oder die Zeile streichen.

### 30. Minor – «Jellyseerr verwenden» aus lässt Wünsche nach 24 h scheitern

- **Wo:** `03` 6.3 Schritt 5; `07` 4.6.
- **Was falsch ist:** Die 24 h zählen ab `created_at`, auch wenn der Administrator bewusst ausgeschaltet hat.
- **Korrektur:** «Die Frist ruht, solange «Jellyseerr verwenden» aus ist» oder «beim Ausschalten werden `queued`-Wünsche
  sofort `failed` mit Grund `disabled`». Marco wählt.

### 31. Minor – Gespeicherter Schlüssel geht an eine neu eingetippte Adresse

- **Wo:** `03` 15.3, `08` 4.5 `testSeerrConnection` («optional ungespeicherte `url`, `apiKey`»).
- **Was falsch ist:** Mit leerem Schlüsselfeld schickt der Test den gespeicherten Schlüssel an jede neu eingegebene
  Adresse, auch über `http` ins Internet.
- **Korrektur:** «Weicht der Host der eingegebenen Adresse vom gespeicherten ab, verlangt der Test den Schlüssel neu;
  bei `http` zu einer nicht lokalen Adresse warnt die Prüfliste.»

### 32. Minor – Mindestversion «Jellyseerr ≥ 2.0», geprüft ist aber nur 2.7.3

- **Wo:** `03` 15.1, `07` 4.1, Blatt 4.
- **Was falsch ist:** `X-API-User` ist nur für 2.7.3 und 3.5.2 belegt (D 1).
- **Korrektur:** «Jellyseerr ≥ 2.7 bzw. Seerr ≥ 3.0; ältere Versionen «nicht geprüft» (Warnung)».

### 33. Minor – Unbelegte API-Details

- **Wo:** `08` 5 und `03` 7.1. Bericht D belegt diese Stellen nicht:
  - `filter=all` (D nennt nur «Standard alle»; der Validator weist unbekannte Werte ab, D 2.5);
  - `externalIds.tvdbId` in `GET /tv/{id}`;
  - `media {…, seasons[]}` in den Ergebnissen von `GET /request` (`03` 7.1: «eine Liste genügt»).
- **Korrektur:** Alle drei als «nicht verifiziert» kennzeichnen und gegen eine Instanz prüfen. Sicherer Weg: `filter`
  weglassen.

### 34. Minor – Gelöschtes Konto: «geschlossen» oder «nicht zuordenbar»?

- **Wo:** `04` 10 und `08` 2.6.
- **Was falsch ist:** Die Wünsche bekommen `closed_at`, aber keinen Zustand, und sollen zugleich «nicht zuordenbar»
  weiterlaufen. Sie blockieren den Teilindex weiter.
- **Korrektur:** «Wünsche des Kontos verlieren ihre Zeilen in `wish_requesters` und gelten ab dann als nicht
  zuordenbar; `closed_at` entfällt hier.»

### 35. Minor – Gleichzeitiges Anfordern zweier Profile

- **Wo:** `03` 6.1 Schritt 2 und 3.
- **Was fehlt:** Wie der Dienst die Verletzung des Teilindex beim zweiten INSERT behandelt.
- **Korrektur:** «Konflikt am Teilindex → Transaktion wiederholen; dann greift Schritt 2 (Mitwunsch)». Dazu ein Test in
  `09` 2.4.

### 36. Minor – Tote oder ungenaue Belege

- `01` 7: `docs/adr/0014-benutzerverwaltung.md:140`. Die Datei hat nur 135 Zeilen; richtig ist `:66`.
- `03` 2.4: «ADR 0014:161» ist ebenfalls tot; richtig ist `:87`.
- `01` 9: `server/queries/system/notify_channels.sql:72-83` für die Standardkanäle. Dort steht `ChannelUpsert`; richtig
  ist `:7-18`, wie in `06` und `11`.
- `01` 1 und 5: `docs/adr/0011-spotlight.md:99` nennt nicht den Wortlaut S-4, sondern den verworfenen Schalter. Für den
  Wortlaut `docs/planning/04-entscheidungen.md:20` angeben.
- `01` 7: ADR 0005 `:43` belegt das Hauptprofil nur schwach; richtig ist `:34`.
- `03` und `08`: «ADR 0017:112-115» und «ADR 0019:49-58» ohne Dateinamen. Richtig:
  `docs/adr/0017-scheduler-fortschritt-pause-abbruch.md:112-115` und
  `docs/adr/0019-verwaltungsleiste-aktivitaet-neustart.md:49-58`. Gleich bei «`00017_*.sql`» →
  `server/migrations/00017_integrations.sql:47-57`.
- `11` F-13: «Jellyseerr meldet «verfügbar», sobald Radarr die Datei hat». Laut D 11.1 erst nach dem Radarr-Scan von
  Jellyseerr (Vorgabe täglich 04:00). Wortlaut ändern: «… nach seinem Radarr-/Sonarr-Scan, oft Stunden nach dem
  Import; das sagt nichts darüber, ob Slouchcat die Datei schon eingelesen hat».

### 37. Minor – Verweise ohne Dateinamen und tote Verweise (Form)

- **Themenverweise ohne Datei:**
  - `02` Einleitung (Z. 12–13): «Papiere «Verwaltung», «Zuordnung und Rechte», «Benachrichtigungen», «Datenmodell und
    API»»;
  - `02` 3 (Z. 82) und 4.7 (Z. 181);
  - `05` 9 (Z. 174): «Papier «Datenmodell und API»»;
  - `01` 1 (Z. 29): «Papier «Stufen und Fragen»». Dieses Papier gibt es nicht; gemeint ist `11`.
  - Korrektur: durch `07-administration.md`, `04-benutzer-profilmapping.md`, `06-benachrichtigungen.md`,
    `08-datenmodell-api.md` bzw. `11-entscheide-und-adr.md` ersetzen.
- **Toter Link:** `README.md` verlinkt `mockup-abdeckung.md`, die Datei fehlt. Anlegen oder die Zeile streichen.
- **Falsche Zuordnung:** `README.md`, Tabelle «Entscheide», ordnet Jugendschutz F-9 zu. F-9 betrifft Profile ohne
  Recht; für Jugendschutz gibt es keine Frage. «–» setzen.
- **Fremder Ordner:** Im Konzeptordner liegt `.omc/state/…` mit Sitzungsdaten. Vor dem Commit löschen oder in
  `.gitignore` aufnehmen.

### 38. Minor – Grundlage im Repo nicht auffindbar (Form, Rückverfolgbarkeit)

- **Wo:** `03` (20 Stellen), `08` (17), `06` (9), `04` (8), `07` (6), `09` (5), `11` (5): «Bericht A–D», «Bestandsanalyse
  B 5.5», «D 7.4». Dazu rund 80 Verweise auf das «Entscheid-Blatt».
- **Was falsch ist:** Die Quellen liegen nur im Scratchpad der Sitzung (`/tmp/…`). Im öffentlichen Repo führen sie ins
  Leere. Bei den Jellyseerr-Aussagen ist das heikel: Der Bau soll gegen Bericht D prüfen.
- **Korrektur:** Berichte A–D und das Blatt als `docs/planning/25-medienwuensche/quellen/A-…md` bis `F-…md`
  mitcommitten, ohne Filmbilder und ohne Geheimnisse. Ein Satz in `README.md` erklärt die Kürzel. Oder jeden Verweis
  auf den passenden Abschnitt von `01` (17.x) bzw. auf `GH:`-Stellen umschreiben.

### 39. Minor – Lücken in den Abnahmekriterien

- **Wo:** `10`. Für folgende Verhalten gibt es kein AK:
  - Profil gelöscht (Wünsche gehen an das Hauptprofil) und Konto gelöscht;
  - Rückfüllung der Kanäle bei **bestehenden** Konten (AK-63 nur neue);
  - «Wünsche aktiv» bleibt gesperrt, wenn `X-API-User` nicht wirkt (AK-7 prüft nur die Anzeige);
  - «Nur Zustand zeigen» (`status_only`);
  - Sammelbenutzer;
  - «Jellyseerr verwenden» aus;
  - Zuordnung ändern mit offenen Wünschen (eingefroren);
  - Mitwünscher zieht zurück, Besitzer zieht zurück bei Mitwünschern;
  - die Fälle aus Nr. 6, 7, 10 und 11.
- **Korrektur:** Je ein AK in 2.2, 3.3 und 3.6 ergänzen.

### 40. Minor – Unmarkierte Abweichungen vom Blatt

- `04` 2.3, `11` F-3 und ADR 0037 Entscheid 3 legen fest: wirkt `X-API-User` nicht, «lässt sich «Wünsche aktiv»
  nicht einschalten». `07` 4.6 legt zusätzlich fest: einschaltbar nur nach gelungenem Test. Das Blatt sagt nur «meldet,
  wenn es nicht wirkt». Nur `07` markiert es als Vorschlag.
- Die Vorgabe «Wünsche aktiv» = aus steht nicht im Blatt. `07` 7 markiert sie, `08` 2.12 nicht.
- **Korrektur:** In `04` 2.3, `08` 2.12 und `11` als **Vorschlag** kennzeichnen. In `11` 1 als Teil von F-3 bzw. als
  neue Teilfrage aufnehmen.

---

## Prüfung gegen die Anforderungen (1) bis (8), kurz

| Anf. | Stand | Lücke (Nr.) |
|---|---|---|
| (1) Spotlight | fast vollständig: Fälle A–D, Kennzeichnung, Prepaint, Kuratierung, Schalter | 2, 3, 13 |
| (2) Jellyseerr-API | sauber nach D; keine erfundenen Funktionen | 9, 10, 14, 23, 33 |
| (3) Zuordnung | A/B/C bewertet, C begründet | 8, 11 (falsche Zuweisung), 12 |
| (4) Serien | Fälle tabelliert | 4, 9 |
| (5) Benachrichtigungen | Arten, Vorgaben, PIN-Regel, Doppelschutz | 6, 12, 15, 28 |
| (6) Verwaltung | alle Orte, ein Ort je Einstellung, Diagnose | 3, 18 |
| (7) Technik, 20 Punkte | alle Punkte vorhanden, Mermaid gültig | 1, 7, 9, 10, 25, 26, 27 |
| (8) Entscheide/ADR/Plan/AK | vollständig, Regel 11 nur als Vorschlag | 16, 22, 39, 40 |

Mermaid (von Auge): alle acht Diagramme sind syntaktisch gültig: `stateDiagram-v2`, `flowchart`, `sequenceDiagram`
mit `alt/else/loop/Note`, `erDiagram` mit Mehrfach-PK. Inhaltlich weichen sie ab, siehe Nr. 4 und 21.
Schweizer Schreibweise: kein «ß» gefunden.

## Richtig belegt (Stichprobe, Auswahl)

`api/openapi.yaml:4`, `:9-16`, `:33-41`, `:261`, `:12134-12142`, `:16683`; `trakt.go:270-273`, `:184-192`;
`metadata/trakt.go:278-313`; `scoped/spotlight.sql:9-16`; `visibility/discover.go:395`; `visibility.go:31-32`;
`model.go:997-999`; `HeroCarousel.tsx:210-215`; `match.go:312-330`; `metadata.go:344-349`; `library/service.go:227-232`;
`00015_discover.sql:329-341`; `00018…:21-36`; `00016_admin.sql:27-35`; `00017_integrations.sql:24`;
`notify_channels.sql:7-18`; `channels.go:23`, `:104`, `:207`; `dispatch.go:102-109`; `netguard.go:113-128`;
`update_install.go:107-113`; `logging.go:43-56`; `settings.go:56`, `:630-640`; `queue.go:1-29`, `:76-80`;
`scheduler.go:53`; `contract_access_test.go:494`, `:567`, `:655`, `:747`, `:798`; `vite.config.ts:141-161`;
`sections.ts:59-91`; `services.tsx:12`; `00063…:86-112`; `webhook_handlers.go:45-54`; `model/integrations.go:45-51`;
`receive.go:17-19`, `:88-92`; `tvdb.go:430-434`; `ADR 0034:28-38`; `03-plan-v1.md:30`, `:38`, `:115`, `:164`,
`:286-292`, `:308-316`; `docs/ideas/README.md:9-12`; `HANDOVER.md:120-127`; Spec `:35`, `:46`, `:64-69`, `:120`, `:149`,
`:159`, `:176`, `:182-185`; `testdata/db` 46 Dateien; 23 Server-Sprachen.
