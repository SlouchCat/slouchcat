# ADR 0045: Das Wunschrecht folgt der Altersgrenze

- Status: gebaut auf dem Zweig `claude/wunschrecht-standard` (26. November 2026), **angenommen mit
  dem Merge**.
- Quelle: Marcos Entscheid vom 26. November 2026 (Frage 36 in `docs/planning/04-entscheidungen.md`),
  er löst F-5 von Konzept 25 ab (`docs/planning/25-medienwuensche/11-entscheide-und-adr.md`) und
  schränkt F-6 ein; ADR 0037 (Wünsche über Seerr), ADR 0014 (Rollen), ADR 0034 (Freigabeländer)
- Vertrag **1.44.0** (zusätzlich: neuer Wert `age_restricted`, neue Beschreibungen), Migration
  **00086**
- Harte Regeln: 5 (Migration nach Vorlage), 7 (Vertrag), 9 (Texte), 12 (Entscheid im Server)

## Kontext

Bis Vertrag 1.43.0 durften ab Werk nur das Besitzerkonto und die Hauptprofile Titel anfordern;
jedes andere Konto, jedes weitere Profil und jede Einladung begann ohne Recht (F-5), und die
Einstellung «Neue Konten dürfen anfordern» stand auf aus. Das Recht steht als Schalter in
`accounts.can_request` und `profiles.can_request` (Migration 00067); wirksam ist es nur, wenn beide
an sind (das Konto ist der Deckel).

Marco hat am 26. November 2026 entschieden: «Mach doch als Standard, dass alle dürfen nur die mit
einer Altersbeschränkung nicht.» Auf die Rückfragen: (1) Das gilt für neue Profile und neue Konten,
und «Neue Konten dürfen anfordern» steht ab Werk auf an. (2) Bestehende Konten, Profile und offene
Einladungen ohne Altersgrenze bekommen das Recht einmal, die mit Altersgrenze nicht. (3) Das Recht
folgt der Altersgrenze: Bekommt ein Profil eine Altersgrenze, verliert es das Recht von selbst;
fällt sie weg, kommt das Recht zurück. Für ein Profil mit Altersgrenze lässt sich der Schalter
nicht einschalten.

«Ohne Altersgrenze» heisst Stufe 18 (`model.MaxRatingAge`); jede Stufe darunter ist eine
Altersgrenze.

## Entscheid

1. **Der gespeicherte Schalter bleibt die Wahrheit, und die Datenbank führt ihn der Altersgrenze
   nach** (Variante b der Aufgabe). Zwei Trigger der Migration 00086 schreiben `can_request`, sobald
   `max_rating` die Grenze 18 überschreitet: `accounts_request_follows_rating` und
   `profiles_request_follows_rating` (`AFTER UPDATE OF max_rating`, nur wenn alt und neu auf
   verschiedenen Seiten von 18 liegen). Unter 18 wird der Schalter aus, auf 18 wird er an.
   Wächter (je ein `BEFORE INSERT` und ein `BEFORE UPDATE OF can_request` auf `accounts`, `profiles`
   und `invitations`, Meldung `age_restricted`) brechen jede Zeile mit `can_request = 1` und
   `max_rating < 18` ab. So kann kein
   Schreibweg die Regel vergessen, auch kein künftiger.

   **Warum nicht ableiten (Variante a)?** Das wirksame Recht wird an mindestens acht Stellen
   gelesen: `wishes/rights.go`, `queries/wishes/discover.sql` (Profile mit Recht für die
   Entdecken-Seiten), `queries/wishes/mapping.sql` (Zähler «ohne Seerr-Benutzer», Übersicht der
   Konten, Eröffnen in Seerr), die Rechte eines eröffneten Seerr-Benutzers (`syncRights`), die
   Kontenliste der Verwaltung, `Profile.canRequest`, `Invitation.canRequest` und die
   Spotlight-Seite. Jede neue Lesestelle müsste die Regel kennen. Geschrieben wird die Altersgrenze
   dagegen nur an vier Stellen (`UpdateProfile`, `UpdateAccountLimits`, `ClampProfileRatings`,
   `SetAccountRole`), und die Trigger decken sie alle samt jeder künftigen ab. Der Deckel des Kontos
   gilt weiter: Senkt ein Administrator die Grenze eines Kontos, senkt `ClampProfileRatings` die
   Profile mit, und der Trigger der Profile nimmt ihnen das Recht.

2. **Vorgaben beim Anlegen.** Ein neues Profil (`InsertProfile`, auch das Hauptprofil der
   Ersteinrichtung und das eines eingeladenen Kontos) bekommt das Recht genau dann, wenn seine
   Stufe 18 ist. Das Konto der Ersteinrichtung (`InsertAccount`) hat Stufe 18 und das Recht. Eine
   Einladung ohne `canRequest` nimmt «Neue Konten dürfen anfordern» (jetzt ab Werk an), aber nur
   mit Stufe 18; das eingeladene Konto erbt den Wert der Einladung. Der Deckel bleibt: Ein neues
   Profil mit Stufe 18 in einem Konto ohne Recht hat den Schalter an und darf trotzdem nicht.

3. **Einschalten für eine Altersgrenze wird abgewiesen:** 422 `validation_failed` mit dem Feld
   `canRequest` und dem Code `age_restricted` (gleiche Form wie `account_not_allowed`) bei
   `createProfile`, `updateProfile`, `updateAccountAccess` und `createInvitation`. Massgeblich ist
   die Stufe nach der Anfrage und, für ein Profil, die des Kontos. Ausschalten geht immer.

4. **Überschreitet eine Anfrage selbst die Grenze, gewinnt die Stufe.** Senkt `updateProfile` oder
   `updateAccountAccess` die Stufe unter 18 oder hebt sie auf 18, wird ein mitgeschicktes
   `canRequest` nicht beachtet: Der Trigger setzt das Recht, und es gibt keine Abweisung. Grund: Die
   Handy-App 1.0.3 schickt bei jedem Speichern `maxRating` zusammen mit dem alten Stand des
   Schalters. Ohne diese Regel könnte sie die Stufe eines Profils mit Recht nicht senken (sie
   bekäme 422), und nach dem Aufheben einer Grenze schickte sie das alte «aus» und hielte das Recht
   fern. Wer nach dem Aufheben das Recht nicht will, schaltet es danach aus.

5. **Was ein Ausschalten von Hand vor der Grenze wird:** Es wird nicht gemerkt. Fällt die Grenze
   weg, kommt das Recht zurück, auch wenn es vorher jemand von Hand ausgeschaltet hatte. Das ist
   Marcos Antwort 3 wörtlich und braucht keine zusätzliche Spalte; das Hauptprofil (beim Konto ein
   Administrator) sieht den Schalter im selben Formular und kann es wieder ausschalten.

6. **Der Grund heisst `age_restricted`.** `WishRights.reason` und `WishRequestReason` bekommen den
   neuen Wert. `rights.go` liest dafür die Stufen mit: Ist der Schalter des Kontos aus und hat das
   Konto eine Altersgrenze, oder ist der des Profils aus und hat das Profil eine, heisst der Grund
   `age_restricted` statt `account_not_allowed` oder `profile_not_allowed`. Die Reihenfolge der
   Gründe bleibt (`disabled`, Konto, Profil, `no_mapping`, `seerr_user_gone`); ein Seerr-Benutzer
   ist weiter nötig, vorübergehende Gründe behalten das Recht wie bisher.

7. **Seerr folgt mit.** Ändert eine neue Stufe das Recht eines Kontos, läuft derselbe Haken wie
   beim Schalter (`RequestRightChanged`): Ein Seerr-Benutzer, den Slouchcat eröffnet hat, bekommt
   seine Rechte angepasst (N-110). Für die Konten, deren Recht die Migration ändert, setzt sie
   `limit_written_at` der Zuordnung zurück; der nächste Abgleich der Seerr-Benutzer reiht sie zum
   Nachziehen ein.

8. **Migration 00086** schaltet einmal um: Konten mit Stufe 18 an, darunter aus; Profile an, wenn
   Profil und Konto Stufe 18 haben, sonst aus; offene Einladungen ebenso nach ihrer Stufe;
   gespeichertes «Neue Konten dürfen anfordern» auf an. **Auslegung beim Umsetzen, nicht Marcos
   Wort:** Auch ein Konto oder Profil mit Altersgrenze, dem jemand das Recht unter F-5 von Hand
   gegeben hatte, verliert es. Sonst bliebe ein Zustand stehen, den Regel 3 verbietet und den die
   Wächter abweisen.

## Ältere Clients

- **Web-App:** reist mit dem Server; sie zeigt den Schalter gesperrt und mit Grund, solange die
  gewählte Stufe eine Altersgrenze ist, gesperrt und an mit dem Hinweis «kommt zurück», solange die
  Stufe eine Grenze aufhebt, und schickt `canRequest` in beiden Fällen nicht mit (Punkt 4). Ein
  neues Profil beginnt mit «an». Ebenso «Zugriff» eines Kontos und «Konto einladen».
- **Nächste Handy-App:** dasselbe im Blatt «Profil bearbeiten» (`ProfileSettingsViewModel`).
- **Handy-App 1.0.3:** Sie kennt `age_restricted` nicht; der erzeugte Client liest den Wert als
  unbekannt, der Schalter bleibt frei, und der Grund auf den Titeln ist der allgemeine Text. Ein
  neues Profil legt sie mit «aus» an, weil ihr Schalter dort aus beginnt (ein ausdrückliches
  `canRequest: false`). Schaltet jemand das Recht eines Profils mit Altersgrenze ein, bekommt sie
  422 und zeigt die allgemeine Fehlermeldung. Stufenwechsel klappen dank Punkt 4.
  `docs/dev/VERTRAEGLICHKEIT.md` zählt «ein neuer Fehler, wo bisher Erfolg kam» zu den brechenden
  Änderungen, die der Test nicht sieht. Die Regel ist bis zur ersten öffentlichen App-Version
  Übung, 1.0.3 läuft nur im geschlossenen Test, und der Erfolg von früher hätte jetzt Marcos Regel
  verletzt. Die nächste Handy-App sperrt den Schalter selbst.
  Dieselbe Liste zählt auch «ein anderer Vorgabewert eines freiwilligen Feldes» zu den Brüchen.
  Davon gibt es hier zwei: Ein fehlendes `canRequest` bei `createProfile` und `createInvitation`
  heisst jetzt «an» bei Stufe 18 (bisher beim weiteren Profil «aus», bei der Einladung die
  Einstellung, die ab Werk aus war), und ein mitgeschicktes `canRequest` wird nicht beachtet, wenn
  die Stufe in derselben Anfrage die Grenze 18 überschreitet (Punkt 4). Beides gilt bewusst in der
  Übungszeit; `createInvitation` ruft keine App auf.
- **TV-App:** schickt nie `canRequest`; nur der neue Grund kommt dazu (Text aus `web/src/locales`).

## Verworfen

- **Ableiten beim Lesen (Variante a):** zu viele Lesestellen, siehe Punkt 1.
- **Die Regel nur im Go-Code der vier Schreibwege:** Jeder künftige Weg, der eine Stufe schreibt,
  müsste daran denken; die Trigger sind eine Stelle.
- **Ein gemerktes «von Hand aus» (eigene Spalte):** mehr Zustand für einen seltenen Fall, und
  Marcos Antwort sagt «kommt zurück».
- **`canRequest` beim Überschreiten der Grenze beachten:** bricht die Handy-App 1.0.3 (Punkt 4).

## Folgen

- Die Altersgrenze eines Profils ist jetzt auch die Grenze des Wunschrechts; F-6 («Kinderprofile
  mit Recht dürfen anfordern») gilt nur noch für Profile ohne Altersgrenze.
- Tests, die die alten Vorgaben (F-5) prüften, sind auf die neue Regel umgestellt.
- Jede Stufe unter 18 nimmt die Wünsche weg, auch eine knapp darunter: «ab 16», in den USA «R»
  (17), in Grossbritannien «15».
- Hebt ein Administrator die Altersgrenze eines Kontos auf, bekommt das Konto sein Recht zurück,
  seine Profile aber nicht: Die Obergrenze hatte ihre Stufen gesenkt, und die bleiben auf dem
  tieferen Wert, bis das Hauptprofil sie auf 18 hebt; dann kommt auch ihr Recht zurück.
- Erwachsene Gäste ohne eigenen Seerr-Benutzer dürfen jetzt anfordern und senden, solange
  «Konten ohne Seerr-Benutzer» auf «Sammelbenutzer» steht, als der Sammelbenutzer
  «slouchcat-gaeste» mit dem geteilten Standard-Limit.
- Der Wächter der Profile prüft nur die eigene Stufe des Profils, nicht die des Kontos. Das reicht,
  weil ein Profil nie über der Obergrenze seines Kontos steht (`ClampProfileRatings`, die Prüfung
  `above_account_limit`); die Migration nimmt für den Bestand die tiefere der beiden Stufen.
- Die Migration schreibt auch die Geschichte der Einladungen um: Eine benutzte, zurückgezogene
  oder abgelaufene Einladung mit Altersgrenze zeigt danach «ohne Recht», auch wenn sie es unter
  F-5 einmal gab. Das Konto, das sie angelegt hat, folgt ohnehin seiner eigenen Stufe.
- **Hingenommen:** Ein Wunsch, der in der Warteschlange wartet (Seerr nicht erreichbar), und dessen
  Profil inzwischen eine Altersgrenze bekam oder dessen Recht jemand ausschaltete, schlägt beim
  nächsten Versuch mit `seerr_user_gone` fehl, Text «Den verknüpften Seerr-Benutzer gibt es nicht
  mehr.» (`wishes/submit.go`, `orphanSender`; ebenso `repoint` in `wishes/accountgone.go`). Der
  richtige Grund bräuchte einen neuen Wert von `WishFailReason`, und den erlaubt die Prüfung
  `fail_reason IN (…)` der Tabelle `wishes` (Migration 00069) nur nach einem Neubau dieser Tabelle
  mit ihren abhängigen Tabellen und Triggern. Das ist für diesen seltenen Fall zu breit; der Fall
  ist hier festgehalten und kann mit dem nächsten Neubau von `wishes` einen eigenen Grund bekommen.
- Die Entdecken-Seiten und das Anfordern verlangen das Recht; ein Profil unter 18 hat sie darum
  nicht mehr, und die Altersstufen unter 18 im Zwischenspeicher von Entdecken (`max_age`) kommen
  im Betrieb nicht mehr vor. Die Prüfungen nach Alter bleiben als zweite Linie im Server. Damit
  sie getestet bleiben, geben zwei Testaufbauten Profilen unter 18 das Recht wie vor 00086: Die
  Vertragstests der Wünsche (`keepRightsBelow18`) entfernen nur die beiden Wächter auf dem
  Schreiben von `can_request`, die Entdecken-Tests in `internal/wishes` nur den Trigger, der
  einem Profil mit neuer Stufe das Recht nimmt. Die neue Regel prüfen eigene Tests
  (`TestContractRequestRightFollowsRating`, `TestMigration00086RequestRightFollowsRating`,
  `TestCanRequest`); `profile_not_allowed` prüft der Vertragstest an PX (Stufe 18, Schalter aus).
- `updateAccountAccess` liest das Konto in der Schreibtransaktion, damit Prüfung und Schreiben
  dieselbe Stufe sehen und ein Wächter nie als 500 durchschlägt.
- Neue Texte: der Grund `age_restricted` (Titel, Fehler, Formulare) in `de`, `de-DE` und `en`.
