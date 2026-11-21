# Verträglichkeit: Operationen, die die Apps aufrufen

Die Web-App reist im selben Container wie der Server; sie passt immer zu ihm. Die Android-Apps
nicht: Der Play Store hebt sie von selbst, den Server hebt jeder Betreiber von Hand. Eine
installierte App muss darum mit älteren und neueren Servern auskommen, und ein neuer Server darf
keine installierte App brechen. Diese Regel legt fest, wie der Vertrag sich dafür ändern darf.
Grundlage: Konzept 27 Abschnitt 11.5 und Frage F-9, Konzept 26 Abschnitt 8.4.

## Ab wann sie gilt

- **Verbindlich** ab der ersten App-Version, die öffentlich im Play Store steht (Konzept 27
  Stufe 10, für die TV-App Konzept 26 Stufe 6).
- **Bis dahin Übung.** Der Test (unten) läuft schon und schlägt bei jedem Bruch an. Solange nur
  interne und geschlossene Tests laufen, darf ein Bruch eine Ausnahme mit dem Tag des Eintrags als
  Ende der Übergangszeit bekommen; der Grund steht dann im Eintrag. Eingetragen wird trotzdem,
  damit der Weg geübt ist.
- Die Regel «Vertrag = Tag» bleibt daneben unverändert: Jede veröffentlichte Vertragsfassung hat
  ihren Tag.

## Welche Operationen

- Jede Operation in `api/openapi.yaml`, die `x-clients` trägt (Regel 7, ADR 0040): alle
  Operationen des Nutzerbereichs (`app`) und die des Fernsehers (`tv`, Konzept 26 Anhang B).
- Dazu die öffentlichen Operationen, die die Apps aufrufen. Öffentliche Operationen tragen kein
  `x-clients`; der Test nennt sie darum in der Liste `appPublicOperations`: `getHealth`,
  `proveServer`, `appLogin` (Handy und Fernseher), `redeemPairingVoucher` (Fernseher),
  `checkOfflineRevocations` (Handy). Ruft eine App eine weitere öffentliche Operation auf, kommt
  sie in diese Liste.

Frei ändern lässt sich nur der Rest, also im Wesentlichen die Verwaltung, die Ersteinrichtung und
die Einladungen.

## Was eine brechende Änderung ist

An einer Operation mit `x-clients` ist brechend:

1. Die Operation verschwindet, bekommt eine andere Methode oder einen anderen Pfad, oder
   `x-clients` verliert eine Art (`app` oder `tv`).
2. Eine Anfrage bekommt ein neues Pflichtfeld oder einen neuen Pflichtparameter, oder der Körper
   wird Pflicht.
3. Ein Feld oder Parameter einer Anfrage ändert seinen Typ.
4. Ein Wert einer Aufzählung in einer Anfrage verschwindet.
5. Eine Erfolgsantwort (2xx) verschwindet oder bekommt einen anderen Statuscode.
6. Ein Feld einer Erfolgsantwort verschwindet oder ändert seinen Typ.
7. Ein Pflichtfeld einer Erfolgsantwort wird freiwillig. Der erzeugte Kotlin-Client erwartet es
   sonst und bricht ab, wenn es fehlt.

Diese sieben prüft der Test. **Ebenso brechend, aber nicht automatisch prüfbar:**

- Ein Feld behält Namen und Typ, bedeutet aber etwas anderes (Einheit, Zeitzone, Zählweise).
- Strengere Prüfung einer Anfrage: kürzere Höchstlänge, engeres Muster, kleinerer Bereich.
- Ein neuer Fehler, wo bisher Erfolg kam, oder ein anderer Fehlercode für denselben Fall. Die Apps
  entscheiden nach dem Code im Fehlerkörper (`code`).
- Ein neuer Pflicht-Kopf in der Anfrage, eine andere Anmeldung, ein strengeres `x-visibility`.
- Ein anderer Vorgabewert eines freiwilligen Feldes.
- Änderungen an den Streaming-Routen (Range, HLS, Bilder), deren Körper kein JSON-Schema hat.

**Nicht brechend** (die Apps vertragen Unbekanntes, Konzept 27 11.5):

- eine neue Operation, ein neues freiwilliges Feld in einer Anfrage, ein neues Feld in einer
  Antwort;
- ein neuer Wert einer Aufzählung, in Anfrage oder Antwort;
- ein Pflichtfeld einer Anfrage wird freiwillig, ein freiwilliges Feld einer Antwort wird Pflicht;
- eine neue Art in `x-clients`.

## Die Übergangszeit

Eine brechende Änderung kommt nie auf einmal. Der Server liefert **alt und neu nebeneinander**,
mindestens so lange, bis beides erfüllt ist:

- **90 Tage** nach dem Release des Servers, das das Neue bringt, **und**
- **30 Tage**, nachdem eine App-Version im Store öffentlich ist, die das Neue nutzt. Betrifft die
  Operation `tv`, gilt das für die TV-Geräteart gesondert.

Massgeblich ist der spätere Tag. Erst danach darf das Alte weg. Ein Bruch ohne Übergangszeit ist
nur bei einer Sicherheitslücke erlaubt, mit einem Satz im Release-Text.

## Wie man eine Änderung zusätzlich macht

Fast jeder Bruch lässt sich in drei Schritte zerlegen:

1. **Neu dazu.** Das Neue kommt als neues Feld, neue Operation oder neuer Wert hinzu; das Alte
   bleibt und bekommt im Vertrag `deprecated: true` und in der Beschreibung «entfällt frühestens am
   <Datum>». Ein Feld umbenennen heisst: das neue Feld dazu, das alte weiter füllen. Ein Feld
   ändert seinen Typ: ein neues Feld mit dem neuen Typ.
2. **Die Apps ziehen nach.** Eine App-Version, die nur noch das Neue nutzt, geht in den Store. Sie
   prüft mit `getHealth.contractVersion`, dass der Server das Neue kann; sonst nimmt sie das Alte
   oder zeigt «Server zu alt».
3. **Das Alte weg,** frühestens am Ende der Übergangszeit: Eintrag in der Ausnahmeliste (unten),
   dann den Stand neu einfrieren.

## Mindestfassungen

| Wer | Wo | Was geschieht |
|---|---|---|
| App braucht einen neueren Server | `Compatibility.MIN_CONTRACT` in `android/core/session` (heute `1.42.0`), verglichen mit `getHealth.contractVersion` | Die App zeigt «Server zu alt, bitte aktualisieren» mit beiden Fassungen |
| Server braucht eine neuere App | `DefaultMinClientVersions` im Server (`server/internal/api/session.go`, heute `0.0.0` für `mobile` und `tv`), verglichen mit dem Kopf `X-Slouchcat-Client` | 426 `app_too_old` auf jeder Operation ausser `getHealth`; die App zeigt «Bitte die App aktualisieren» |

`MIN_CONTRACT` der App steigt nur, wenn die App etwas braucht, das ältere Server nicht haben. Die
Mindestfassung der App im Server steigt nur am Ende einer Übergangszeit, nie früher, und nie über
die älteste App-Version, die noch im Store angeboten wird.

## Der Test

`TestXClientsCompatibility` in `server/internal/api/contract_compat_test.go` vergleicht
`api/openapi.yaml` mit dem eingefrorenen Stand
`server/internal/api/testdata/x-clients-frozen.json` und schlägt bei jedem der sieben Brüche an.
Die Meldung nennt Operation und Änderung, zum Beispiel
`listMyDevices: response 200 drops $[].downloads`.

**Einfrieren.** Der Stand ist der der zuletzt veröffentlichten Vertragsfassung mit `x-clients`:

```bash
task server:freeze:x-clients REF=v1.42.0   # ohne REF: der neueste Tag v*
```

Das Ziel liest `api/openapi.yaml` aus dem Tag, schreibt die Datei neu und meldet die Zahl der
Operationen. Danach die Datei committen. Eingefroren wird nach jedem Release, dessen Vertrag eine
Operation mit `x-clients` neu bringt oder ändert.

**Stand am 21. November 2026:** Die letzte veröffentlichte Fassung ist 1.41.0 (Tag `v1.41.3`); sie
hat noch kein `x-clients`. Eingefroren sind darum nur die zwei öffentlichen Operationen aus
`appPublicOperations`, die es dort schon gibt (`getHealth`, `checkOfflineRevocations`).
`x-clients` kommt mit Vertrag 1.42.0 (Konzept 27 Stufe 1, Zweig `claude/k27-stufe1-c`). **Sobald
dieser Zweig in `main` ist,** den Stand mit `task server:freeze:x-clients REF=main` einfrieren
und committen, nach dem Tag `v1.42.0` noch einmal mit `REF=v1.42.0`. Am Zweig erprobt: 119
Operationen eingefroren (mit den fünf öffentlichen 124), Vergleich mit sich selbst ohne Befund, ein
entferntes Feld in `MyDevice` gefunden; der Stand aus `v1.41.3` gegen den Vertrag des Zweigs ohne
Befund.

**Ausnahmen** stehen in `server/internal/api/testdata/x-clients-exceptions.json`, eine je
bewusstem Bruch:

```json
[
  {
    "operation": "listMyDevices",
    "problem": "response 200 drops $[].downloads",
    "transitionEnds": "2027-03-01",
    "reason": "replaced by downloadCount since 1.50.0 (2026-12-01); app 1.4.0 in the store since 2027-01-15"
  }
]
```

- `problem` ist der Text der Meldung nach dem Doppelpunkt, wörtlich.
- `transitionEnds` ist das Ende der Übergangszeit; ab diesem Tag lässt der Test den Bruch durch,
  vorher schlägt er mit «before the end of its transition period» an.
- `reason` nennt, was das Alte ersetzt, seit welchem Release und welche App-Version es nutzt.
- Eine Ausnahme, die zu keiner Meldung mehr passt, schlägt an. Nach dem nächsten Einfrieren ist
  der Bruch Teil des Stands, und der Eintrag muss weg.

## Prüfliste für Pull Requests

Gilt für jeden Pull Request, der `api/openapi.yaml` ändert:

- [ ] Betrifft die Änderung eine Operation mit `x-clients`? Wenn nein: fertig.
- [ ] Ist sie zusätzlich (Abschnitt «Nicht brechend»)? Dann reicht der grüne Test.
- [ ] Ist sie brechend, auch nach der Liste, die der Test nicht sieht? Dann: in drei Schritte
      zerlegt (Abschnitt «Wie man eine Änderung zusätzlich macht»), das Alte mit `deprecated: true`
      und Datum, und erst nach der Übergangszeit entfernt.
- [ ] Entfernt der Pull Request etwas nach der Übergangszeit: Eintrag in
      `x-clients-exceptions.json` mit `transitionEnds` und `reason`.
- [ ] Braucht die App etwas Neues: `MIN_CONTRACT` der App nur anheben, wenn es ohne nicht geht.
- [ ] Steigt die Mindestfassung der App im Server: erst am Ende der Übergangszeit.
- [ ] Nach dem Release: `task server:freeze:x-clients REF=<Tag>`, Datei committen, überholte
      Ausnahmen entfernen.

## Noch einzutragen

Nichts. Die Daten in den Beispielen sind erfunden.
