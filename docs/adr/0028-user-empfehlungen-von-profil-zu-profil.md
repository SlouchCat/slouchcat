# ADR 0028: User-Empfehlungen von Profil zu Profil

- Status: angenommen (9. November 2026)
- Quelle: Konzept `docs/planning/18-trickplay.md` Teil B (Abschnitte 14.1 bis 14.9, Kritiker-Review
  Abschnitt 15), **Frage 25** in `docs/planning/04-entscheidungen.md` mit Marcos Antworten U-F1 bis
  U-F11; Bauplan Abschnitt 12, Schritte 1 und 2
- Vertrag 1.23.0 (brechend, gemeinsam mit Teil A), Migration 00041
- Harte Regeln: 2 (Sichtbarkeit nur über `visibility`, nicht sichtbar heisst 404), 5 (Migration nach
  Vorlage), 7 (`x-visibility`, Vertragstest), 8 (Tastaturtest), 11 (Nicht-Ziel mit Ausnahme),
  12 (Entscheid im Server)

## Kontext

Bis 1.22.0 pflegt nur der Administrator die Liste «User-Empfehlungen» (bis Konzept 17 «Cat's
Empfehlungen», Spec E-2): Tabelle `top_picks` aus 00003 mit Zielprofil, Notiz, Zeitfenster und
Reihenfolge, gepflegt auf `/admin/picks`. Marco will stattdessen, dass sich die Profile gegenseitig
Titel empfehlen: ein Klick auf der Titelseite, nach 60 Tagen fällt der Titel wieder heraus, die Dauer
steht auf einer Einstellungsseite. Die Spec führt «Interaktion zwischen Profilen (Bewertungen,
Empfehlungen untereinander)» als Nicht-Ziel.

## Entscheid

1. **Ausnahme vom Nicht-Ziel nur für anonyme Empfehlungen** (U-F9). Bewertungen, Kommentare und
   Nachrichten zwischen Profilen bleiben Nicht-Ziele. Die Spec trägt die Ausnahme unter «Non-Goals»
   und in der Statuszeile «Entdecken und Kuratierung»; E-2 ist neu gefasst.

2. **Eine Zeile je Titel und Profil** (Migration 00041). `user_picks (item_id, profile_id,
   created_at)`, Primärschlüssel `(item_id, profile_id)`, `STRICT, WITHOUT ROWID`, Index auf
   `created_at`, beide Fremdschlüssel mit `ON DELETE CASCADE`. Empfiehlt ein Profil denselben Titel
   nochmals, erneuert sich nur der Zeitpunkt (`INSERT … ON CONFLICT DO UPDATE`).
   `picks_config` ist eine Einzeilen-Tabelle (`id = 1`, Muster `spotlight_config`) mit
   `retention_days` (ab Werk 60, `CHECK` 7 bis 365).

3. **Die Admin-Pflege fällt weg, die alten Einträge auch** (U-F1, U-F11). `top_picks` samt Indizes
   geht in 00041 ganz weg; die Liste startet nach dem Update leer. Die Operationen `listTopPicks`,
   `createTopPicks`, `reorderTopPicks`, `updateTopPick`, `deleteTopPick` und die Schemas `AdminPick`,
   `PickRequest`, `PickUpdate`, `PickOrder` mit dem Parameter `PickID` entfallen (brechend).

4. **Ablauf ab der jüngsten Empfehlung** (U-F5). Ein Titel steht in der Liste, solange seine
   jüngste gültige Empfehlung jünger als `retention_days` ist. Alle Abfragen filtern nach Zeit,
   die stündliche `library.sweep` löscht abgelaufene Zeilen (ohne eigene Jobart). Eine kürzere
   Dauer wirkt sofort. Dass ein Profil einen Titel durch Zurücknehmen und erneutes Empfehlen
   beliebig lange oben hält, ist gewollt.

5. **Lesen über `visibility`** (Regel 2). `ListUserPicks` und `CountUserPicks` in
   `queries/scoped/items.sql`: `GROUP BY item_id`, `MAX(created_at)` als Zeitpunkt, über
   `visible_items`, neueste zuerst, mit `LIMIT`/`OFFSET` in SQL. Jeder Titel erscheint einmal, für
   alle Profile aller Konten, die ihn sehen dürfen (U-F2, U-F4, U-F8); Reihe auf der Startseite
   höchstens 20 Titel, Seite `/picks` geblättert.

6. **Empfehlen nur im Besitzerkonto** (U-F6, U-F10). `PUT /items/{id}/pick` (`recommendItem`, 200
   mit `expiresAt` und `retentionDays`) und `DELETE /items/{id}/pick` (`unrecommendItem`, 204),
   beide `scoped`. Erst prüft der Server die Sichtbarkeit des Titels für das aktive Profil, dann
   die Titelart (keine Staffeln und Extras) und das Konto (`accounts.is_owner`); jedes Nein heisst
   404 wie ein unbekannter Titel. Zurücknehmen löscht nur die eigene Zeile (U-F7).

7. **Anonym** (U-F3, 14.6). Die App bekommt je Titel nur `ItemDetail.recommendedByMe` (eigene,
   noch gültige Zeile) und `ItemDetail.canRecommend` (Besitzerkonto und erlaubte Titelart), kein
   «von wem» und kein «wie viele»; `listPicks` und die Reihe `picks` tragen keine Notiz mehr. Die
   Verwaltung zeigt je Titel nur die Anzahl, den **Tag** der jüngsten Empfehlung und die Tage bis
   zum Ablauf, keine Namen. Bewusst hingenommen: Bei einer Anzahl von 1 lässt sich über das
   Aktivitätsprotokoll raten, wer empfohlen hat; wer die eigene Empfehlung zurücknimmt und den Titel
   weiter sieht, weiss, dass ein anderes Profil ihn auch empfohlen hat.

8. **Verwaltung** (14.7). `/admin/picks` wird die Seite «User-Empfehlungen»: `listAdminUserPicks`
   (`GET /admin/picks`), `removeAdminUserPick` (`DELETE /admin/picks/{itemId}`, entfernt alle
   Empfehlungen des Titels), `getPicksSettings` und `updatePicksSettings`
   (`/admin/picks/settings`), alle `admin`.

9. **Zusammenführen und Wiederverknüpfen** verschieben Empfehlungen mit
   `INSERT INTO user_picks SELECT @target, … ON CONFLICT DO UPDATE SET created_at =
   max(created_at, excluded.created_at)` und löschen danach die Zeilen der Quelle; ein einfaches
   `UPDATE` scheiterte am Primärschlüssel, sobald ein Profil beide Kopien empfohlen hat.
   `CountRelinkUserData` zählt `user_picks`, `RelinkUserData.topPicks` heisst `userPicks`.

## Folgen

- Vertrag 1.23.0 bricht die Admin-Operationen der alten Liste und die Felder `topPicks`; die
  Webanwendung zieht im selben Release mit.
- Der Knopf «Zu den User-Empfehlungen» aus Konzept 17 (D-10, F-17) ist der Stern «Empfehlen» /
  «Empfohlen ✓» der Knopfzeile, nicht ein zweiter Knopf.
- Eingefrorene Stände mit `top_picks` (M7, M8, startseite) prüfen neu, dass die Tabelle nach 00041
  fehlt und `user_picks` leer ist.
- Gastkonten (andere Konten als das Besitzerkonto) sehen die Liste, aber keinen Stern.
