# ADR 0034: Freigabeländer und Altersstufen je Land

- Status: angenommen (15. November 2026, Stufe 3 «Freigabeländer» von Konzept 22 gebaut); **ändert
  Entscheid 6 (AN-8)**, **Entscheid 13** («Freigaben nie gesenkt») und den **Nachtrag zu Entscheid 6 aus
  Frage 16** (Konzept 10, Abschnitt 6.6). Spec K-3 bleibt unverändert gültig.
- Quelle: Konzept `docs/planning/22-ersteinrichtung-und-sprachen.md` (Fassung 4), Abschnitt 4.11 (Teil C),
  F-12, F-16, F-17, Review 2 (B2-1, B2-2, M2-1 bis M2-4, m2-5 bis m2-7, m2-11); **Frage 29** in
  `docs/planning/04-entscheidungen.md`
- Vertrag **1.36.0**, Migrationen **00062** bis **00064**, eingefrorener Stand `testdata/db/freigabelaender.sql`
- Betroffene Spec-IDs: K-2, K-3, B-6
- Harte Regeln: 1 (Codegen), 2 (Titelabfragen), 5 (Migration nach Vorlage), 7 (Vertrag), 8 (Tastaturtest),
  9 (Texte nur aus den Sprachdateien), 12 (Entscheid im Server)

## Kontext

Bis 1.35 kennt Slouchcat eine Altersskala: die FSK-Stufen 0, 6, 12, 16, 18. Freigaben anderer Länder
werden darauf umgerechnet, und zwar nicht immer nach oben: AT 14 wird 16, aber US PG-13 wird 12 und R wird 16.
Gelesen wird in der Reihenfolge gewähltes Land → DE → AT → CH → GB → US; wählbar sind nur diese fünf Länder.
Die Stufen stehen fest in sieben Datenbank-`CHECK`s, im Vertrag (`Rating` als `enum`), in zwei Prüfungen im
Server und in der Web-App.

Marco will die Freigabeländer aller 23 Oberflächensprachen, und die Profile sollen aus den Stufen des
gewählten Landes wählen (F-12). Überall gilt das echte Mindestalter, auch für die USA (F-17), und ein Wechsel
des Freigabelands liest alle Freigaben neu, auch nach unten (F-16).

## Entscheid

| Punkt | Entscheid |
|---|---|
| **Gespeichert wird das Mindestalter** | `own_rating`, `effective_rating`, `min_child_rating`, `max_rating` und `default_rating` halten das Mindestalter der Freigabe (0 bis 18), nicht mehr die gerundete FSK-Stufe: PG-13 = 13, R = 17, TV-MA = 17, TV-14 = 14, TV-Y7 = 7, GB 15 = 15, AT 14 = 14, FR «-10» = 10. Freigaben ohne festes Alter bekommen einen Wert aus der Tabelle (US PG und TV-PG 6, GB PG 6, GB 12A 12, IT BA 6). Höchstwert bleibt **18**: Unbekannt zählt als 18 (AN-1), Administratoren haben 18 (`00022`), Koreas TV-Stufe 19 wird 18. |
| **Länder und Stufen** | 25 Freigabeländer: die Länder der 23 Sprachen (Tabelle 4.1) und Belgien. Die Tabelle steht in Go: Stufen, Bezeichnungen und die Stelle, die bewertet, in `server/internal/model/ratings.go` (`model.RatingSystems`); welche Zeichenkette von TMDB und TVDB zu welchem Alter gehört, in `server/internal/metadata/ratings.go` (`certificationAges`). Jedes Land endet mit der Stufe 18, auch Schweden und Dänemark, deren höchste Freigabe 15 ist: Mit 18 sieht ein Profil alles, Unbekanntes eingeschlossen. |
| **Reihenfolge beim Lesen** | gewähltes Land → **DE** (F-12) → AT → CH → GB → US. Über alle Anbieter gilt wie bisher der strengste Wert. TVDB kennt alle 25 Länder (`tvdbCountries`). |
| **Freigaben je Land gespeichert** | Neue Tabelle `item_certifications (item_id, provider, country, certification, fetched_at)`. Match und Fill schreiben sie für TMDB und TVDB, nur für die Länder der Tabelle. Hat ein Anbieter für kein Land eine Freigabe, steht eine Zeile mit Land `''`: «gefragt, nichts». Lesen und schreiben darf sie nur das Paket `metadata`: Ihre Abfragen liegen im eigenen sqlc-Paket `store/certq`, und die depguard-Regel `certq` lässt es nur dort zu. |
| **Job «Freigaben neu lesen»** (`metadata.ratings_reread`) | Nach dem Update und bei jedem Wechsel des Freigabelands, wenn es Titel gibt. Die Einstellung `metadata.ratings_reread_done` nennt das Land, für das zuletzt eine Kette alle Titel gelesen hat; solange sie vom Freigabeland abweicht (Kette nicht eingereiht, fehlgeschlagen, abgebrochen), reihen der nächste Start und das nächste Speichern der Einstellungen sie wieder ein. Die Kette trägt das Land in Nutzlast und Schlüssel; wechselt es während des Laufs, beginnt sie beim ersten Titel neu. Er liest Filme und Serien mit `rating_source` `tmdb:*` oder `tvdb:*` oder ganz ohne Freigabe, **ohne Sperre** `own_rating`. NFO, von Hand, Bibliothek, Tags, Feed und AniList bleiben unberührt (M2-1). Mit gespeicherten Zeilen rechnet er lokal; ohne fragt er den Anbieter einmal (eine Anfrage mit der ersten Textsprache, die Grenzen der Anbieter gelten) und speichert die Antwort. Er darf senken und erhöhen. Ist ein Anbieter mit der Id des Titels nicht lesbar (kein Schlüssel, abgewiesener Schlüssel), bleibt der Wert; ein Titel, dessen Anbieter einen Fehler melden, bleibt ebenso und hält die Kette nicht an. Je Stapel von 200 Titeln mit Änderung beendet er offene Streams, die nicht mehr sichtbar sind; Downloads folgen beim nächsten Abgleich (Regel 12). |
| **Der Abgleich beim Scan** | Der Fill nimmt weiter nur den strengeren Wert (Entscheid 13). Der Match darf den Wert desselben Anbieters ändern, auch aus einem anderen Land; er liefert damit denselben Wert wie der Job (die Ausnahme von Konzept 10, 6.6 fällt). |
| **Profile, Konten, Einladungen, Titel** | Eine Obergrenze ist eine Stufe des Freigabelands oder ein Wert, den die Zeile schon hat (`model.RatingSystem.Allows`): Ein Profil behält sein «ab 12» nach einem Wechsel von DE auf US, die Auswahl zeigt es dann als «ab 12» neben den US-Stufen. Ein Titel nimmt von Hand eine Stufe des Landes oder sein bisheriges Alter. |
| **Datenbank** | Die Spaltennamen bleiben (B2-1). `items` und `libraries` werden nicht neu gebaut: je Spalte neue Spalte anfügen, kopieren, alte löschen (ihr `CHECK` geht mit), umbenennen (00062); die vier Freigabe-Trigger und `visible_items` kommen Wort für Wort zurück. `accounts` und `invitations` werden neu gebaut (00063), weil `accounts` einen Tabellen-`CHECK` hat; die beiden Tabellen-`CHECK`s von `accounts` (Admin = 18 und alle Bibliotheken; Eigentümer ist Administrator) bleiben unverändert. Die neuen Spalten-`CHECK`s halten nur den Bereich 0 bis 18. |
| **Vertrag 1.36.0** | `Rating` ist `integer` 0 bis 18. `ratingCountry` nimmt alle 25 Länder. Neu: `RatingSystem` (`country`, `name`, `steps: [{age, label}]`) in `SessionInfo.ratingSystem`, `MetadataSettings.ratingSystems` und `InvitationPreview.ratingSystem`; `DashboardAttention.minimumAgeNotice` und `ratingsRereading`; `dismissMinimumAgeNotice` (`DELETE /admin/dashboard/minimum-age-notice`, `admin`). |
| **Anzeige** | Statt «FSK 12» steht die Bezeichnung des Freigabelands («PG-13», «-12», «R15+»). Ist ein Alter keine Stufe des Landes oder hat die Stufe keine eigene Bezeichnung (eine blosse Zahl wie in Österreich oder der Schweiz), steht «ab {{age}}». Filter und Facetten zeigen «bis {{rating}}» und meinen «Alter ≤ Wert». |
| **Hinweise in der Verwaltung** | Nach dem Update einer Datenbank mit Freigaben eines Anbieters zeigt die Übersicht einmal «Freigaben sind jetzt Mindestalter» (Einstellung `metadata.minimum_age_notice` aus 00064, bis «Verstanden»). Solange der Job läuft, steht dort «Freigaben werden neu gelesen». Vor dem Speichern eines neuen Freigabelands fragt die Seite Metadaten nach, wenn es betroffene Titel gibt. |

## Bezeichnungen sind Daten, keine Texte

- Die Bezeichnungen der Stufen («FSK 12», «PG-13», «Tous publics», «Genel İzleyici») und die Namen der
  bewertenden Stellen («FSK», «BBFC», «MPA», «CNC», «Kijkwijzer») sind Namen. Sie werden nicht übersetzt und
  stehen in der Tabelle des Servers (`model/ratings.go`). Der Server liefert sie im Vertrag; die Web-App hat
  keine Bezeichnung im Code.
- Für Regel 9 ist die Tabelle eine **benannte Ausnahme**, wie `web/src/i18n/languages.ts` für die eigenen
  Sprachnamen. Übersetzt werden nur die Wörter um die Stufe herum: `rating.age` («ab {{age}}»),
  `rating.upTo` («bis {{rating}}»), `rating.system` («{{country}} ({{name}})»).
- Ländernamen kommen aus `Intl.DisplayNames` in der Oberflächensprache. Die fünf Texte
  `admin.metadata.rating.countries.*` sind entfernt.
- Eine Stufe, deren Freigabe nur die Zahl ist (Österreich, Schweiz, Polen, die Zahlen von Spanien,
  Brasilien, Kijkwijzer …), hat keine Bezeichnung. Die Web-App schreibt sie mit den Wörtern der Sprache
  («ab 12», «Ages 12+»).

## Schritt 0 der Stufe 3: Befunde

Die «Heute»-Angaben von 4.11 (Zeilen von `46d41072`) am Stand der Stufe 2 (`15805446`) nachgeprüft:

- **Bestätigt:** Reihenfolge mit gewähltem Land zuerst (`ratingOrder`), die Rundungen (AT 14 → 16, PG-13 →
  12, R → 16, TV-Y7 → 6), der strengste Wert im Fill (`strictestRating`), TVDB mit fünf Ländern, die
  Schutzregeln für NFO, von Hand und Sperre (`match.go`, `fill.go`, `library/audiobook_write.go`,
  `podcast.go`), `VerifyRatings` (`admin/maintenance.go:158`), die sieben «FSK»-Texte in `de.json`, die
  Filter in `visibility/search.go:120` und `visibility/library.go:208,362,452` (sie vergleichen schon `≤`
  und brauchten keine Änderung).
- **Abweichend:** `items_rating_update` stammt zuletzt aus `00010` (nicht `00008:86`); `libraries.default_rating`
  hatte seinen `CHECK` aus `00014` und `00023`; `validRating` steht in `admin/admin.go:928`.
- **Nicht im Konzept, mit geändert:** die URL-Prüfung der Filter `RATINGS` in `discover/searchParams.ts` und
  `catalog/libraryParams.ts` (jetzt jedes Alter 0 bis 18), die festen Stufen von `SearchPage.tsx`, die
  Einladungsseite ohne Sitzung (bekommt das Freigabeland in `InvitationPreview`), der `enum` von
  `MetadataSettings.ratingCountry` und die Ländertexte der Seite Metadaten.
- **Unverändert gelassen:** `library/nfo.go` `GermanRating` liest nur deutsche FSK-Angaben aus NFO-Dateien und
  kennt darum nur 0, 6, 12, 16, 18; das bleibt richtig.
- **TMDB-Listen:** `/certification/movie/list` und `/certification/tv/list` brauchen einen Schlüssel. In
  dieser Sitzung stand keiner zur Verfügung, ohne Marcos Instanz anzufassen. Die Tabelle beruht auf den
  bekannten Listen von TMDB und TVDB; `TestCertificationAges` hält jede bekannte Schreibweise fest. Eine
  unbekannte Zeichenkette ist «unbekannt» und fällt auf das nächste Land der Reihenfolge, wie bisher.

## Abweichungen vom Konzept in Stufe 3

- **Spalten-`CHECK` 0 bis 18 statt keinem:** Das Konzept fügt die neuen Spalten ohne `CHECK` an. Ein Bereich
  0 bis 18 kostet nichts und hält die Obergrenze 18 (m2-5) auch in der Datenbank fest.
- **Jedes Land endet mit 18:** Die Tabelle 4.11 nennt für Schweden und Dänemark nur 0 bis 15. Ohne Stufe 18
  könnte dort kein Profil alles sehen, Administratoren hätten keine Stufe.
- **Portugal beginnt bei 3** (M/3), wie in der Tabelle; ein Profil «ab 0» gibt es dort nicht.
- **Titel ohne Freigabe** liest der Job mit: Hat ein Titel bei TMDB keine Freigabe der fünf alten Länder,
  aber eine des neuen Freigabelands, bekommt er sie.
- **Der Match senkt bei einem anderen Land** (Konzept 10, 6.6 umgedreht), damit Match und Job dasselbe
  rechnen.
- **Hinweis «Freigaben werden neu gelesen»** steht als Hinweis der Übersicht mit dem Link zu den
  Hintergrundjobs; den Fortschritt zeigt die Seite Hintergrundjobs (`jobs.Progress`).
- **Die Einladungsseite** bekommt das Freigabeland mit der Vorschau der Einladung, damit «FSK 12» dort nicht zu
  «ab 12» wird.
- **`CLAUDE.md` Regel 9** nennt die Tabelle noch nicht als Ausnahme (Abschnitt 6 des Konzepts). Diese ADR
  hält die Ausnahme fest; die Zeile in `CLAUDE.md` ändert der Koordinator.

## Folgen

- **Strenger als bisher (F-17).** In einem deutschen Haushalt sieht ein Profil «ab 12» keinen PG-13-Titel
  mehr, ein Profil «ab 16» keinen R-Titel; ebenso US-Serien mit TV-14 für «ab 12». Wer das will, stellt das
  Profil höher. Release-Notes und der Hinweis der Übersicht sagen das.
- **Bis der Job fertig ist**, gelten die alten Werte. Sie können strenger oder lockerer sein.
- **Dauer** (`docs/perf/22-freigabelaender.md`): Nach dem Update fragt der Job jeden Titel einmal bei TMDB,
  begrenzt auf 20 Anfragen je Sekunde (Serien mit TVDB zusätzlich 5 je Sekunde): rund 8 Minuten für 10'000
  Filme. Jeder spätere Landwechsel rechnet lokal, rund 4 Sekunden für 10'000 Titel.
- **Offline (Regel 12):** Der Server entscheidet weiter. Offline-Pakete tragen die erlaubten Profile, der
  Client vergleicht keine Altersstufen.
- **Übersetzungen (Stufe 4):** neue und geänderte Texte stehen in der Übergabe der Stufe 3; die
  Bezeichnungen der Stufen übersetzt niemand.
