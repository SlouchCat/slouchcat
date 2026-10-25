# ADR 0012: Browsbare Wurzeln – der Ordner-Browser listet nur unterhalb freigegebener Ordner

- Status: angenommen (25. Oktober 2026)
- Quelle: Konzept `docs/ideas/02-ordner-browser.md` (Entscheide E1 bis E15, Abschnitte 6 und 7),
  Eintrag vom 25. Oktober 2026 in `docs/planning/04-entscheidungen.md`, Issue #33;
  `docs/planning/03-plan-v1.md` Abschnitte 2.1, 3.4 und 3.8 Punkt 6
- Betroffene Spec-IDs: B-1, A-1, A-2, AN-12
- Harte Regeln: 1 (generierter Code), 2 (Sichtbarkeit), 4 (`paths`), 6 (strict server), 7
  (`x-visibility`), 8 (Tastaturtest), 9 (Farben und Texte), 10 (Logs), 12 (Entscheid im Server)

## Kontext

Das Feld «Ordner» im Bibliotheksformular verlangte einen absoluten Pfad **im Container**, getippt
aus dem Gedächtnis. Der Pfad im Container ist aber nicht der Pfad auf dem Host: `compose.yaml`
bildet `${MEDIA_DIR}` auf `/media` ab, und auf dem NAS heisst dieselbe Freigabe noch einmal anders.
Wer sich vertippt, bekam `library_path_invalid` und musste raten.

Ein Ordner-Browser schliesst diese Lücke, öffnet aber eine andere: Er liest Verzeichnisse: etwas,
das dieser Server bisher nur im Scanner tut, und dort nur unterhalb der Bibliothekswurzeln. Das
Bedrohungsmodell vom 25. Oktober 2026 stuft die naheliegende Bauform — absoluter Pfad als Parameter,
freies Blättern — als hohes Risiko ein. Nicht wegen der Berechtigungsstufe, sondern wegen dessen,
was der Container enthält: `/config` mit `slouchcat.db`, den Sicherungen und **`secret.key`**,
`/cache` mit den Offline-Paketen anderer Profile, und in der Dateisystemklasse je Ordner eine
Netzwerkkarte des Haushalts.

Drei Fragen waren damit zu entscheiden, bevor eine Zeile Code entstand: Welche Grenze gilt, wie
bleibt sie eine Grenze, und was passiert, wenn das Dateisystem nicht mehr antwortet.

## Entscheid

| Punkt | Entscheid |
|---|---|
| Grenze | `SLOUCHCAT_BROWSE_ROOTS`, komma-getrennt, Vorgabe `/media`, vereinigt mit den Wurzeln vorhandener Bibliotheken. Fest `/media` sperrte jeden aus, der eine zweite Einhängung hat; die Einhängetabelle abzuleiten ist Heuristik, die zu streng die Medien verfehlt und zu locker `/config` freigibt; nur aus Bibliothekswurzeln abzuleiten ist zirkulär, weil die erste Wurzel gerade angelegt wird. Steht `/` in der Liste, warnt der Start — sichtbar, nicht unmöglich, wie bei `SLOUCHCAT_TRUSTED_PROXIES` |
| Parameterform | Eine Anfrage nennt **`rootId` plus relativen Pfad**, nie einen absoluten. Ein absoluter Pfad benutzte von `paths` nichts: `Resolve` braucht eine Wurzel und lehnt absolute Pfade gerade ab. Übrig bliebe ein `os.OpenRoot` auf genau den Pfad, den der Client nennt — also `path=/config`. Mit `rootId` plus `rel` läuft jede Anfrage durch `paths.Resolve` und dasselbe `*os.Root`, und die Garantie aus `TestOpenAndWalkStayInsideRoot` hält über Anfragegrenzen hinweg (harte Regel 4) |
| Die Grenze bleibt eine | **Eine neue Bibliothek und eine neue Wurzel müssen unter einer konfigurierten Wurzel liegen** (`library.Service.ValidateNewRoot`). Ohne diese Regel wäre die Wurzelliste vom Angreifer bestimmbar: Wer die Admin-Sitzung hat, legt eine Bibliothek auf `/config` an, und weil die Liste Bibliothekswurzeln hineinvereinigt, hat er sich `/config` damit selbst freigeschaltet. Geprüft wird **zweimal**: lexikalisch entlang der Pfadtrenner auf `filepath.Clean`-Ergebnissen (`strings.HasPrefix` auf dem Rohpfad nähme `/mediaX` für `/media` an), und danach auf dem mit `filepath.EvalSymlinks` aufgelösten Pfad — `os.OpenRoot` löst den **Wurzelpfad selbst** samt Symlinks auf, nur was darunter liegt, ist eingesperrt, und `/media/ablage -> /config` besteht jede rein lexikalische Prüfung |
| Bestandsschutz | Pfadgleichheit mit einer bestehenden `library_roots`-Zeile, nie ein Präfix. Ein Update darf eine laufende Installation nicht anhalten; ein Präfix wäre toter Code, weil `Create` überlappende Pfade schon heute ablehnt. Bei **leerer** Wurzelliste lässt die Regel jeden gültigen Pfad durch: sonst könnte eine Installation mit Medien unter `/data` die Erstinstallation nicht mehr abschliessen |
| Fehlerform | Unbekanntes `rootId` **und** `rel`, das nichts bezeichnet: **identische 404** `path_not_browsable`. Ein eigener Code für «existiert, aber ausserhalb» wäre ein Existenzorakel über den ganzen Container. Die Wurzelprüfung läuft lexikalisch und **vor** jedem Dateisystemzugriff, sonst trennte schon die Antwortzeit die beiden Fälle. Nicht lesbar oder kein Verzeichnis **innerhalb** einer Wurzel: 200 mit `problem` (harte Regel 2: nicht sichtbar heisst 404, nie 403) |
| Methode | `browseServerPaths` ist ein **POST**, obwohl es liest. `requestLog` schreibt den Query-String als eigenes Attribut, und nur `path` läuft durch die Redaktion: Medienpfade mit Ordner- und Benutzernamen stünden sonst im Fehlerring des Dashboards, in der Browser-Historie und im `Referer`. Dazu greift bei POST die CSRF-Prüfung, bei GET nicht. Die beiden Wurzellisten bleiben GET: sie tragen keinen Pfad im Query-String |
| Assistent | **Kein Blättern vor dem ersten Konto.** `listSetupRoots` liefert die konfigurierten Wurzeln, parameterlos, nichts darunter, ohne Dateisystemklasse. Eine Auflistung mit Pfadparameter wäre ein Aufzählungsprimitiv, und die Versuchsgrenze zählt je Client-Adresse, die ein falsch gesetztes `SLOUCHCAT_TRUSTED_PROXIES` den Aufrufer selbst wählen lässt. Die Operation zählt **nicht** gegen `AllowAttempt` — dieser Topf ist mit Anmeldung, Kontoanlage, PIN und Einladungen geteilt, und der Assistent holt die Liste bei jedem Laden; sie hat einen eigenen Topf von 60 Anfragen je Minute und Adresse. Und sie antwortet aus einer beim Start gefüllten Liste: die einzige unauthentifizierte Operation, die das Dateisystem berührt, soll es nicht je Anfrage tun |
| Keine Zahlen | Mediendateien werden **nicht** gezählt, auch nicht später. Zählen heisst Gehen, und SMB und NFS sind in dieser Installation der Hauptfall, nicht die Ausnahme: Die Zahl wäre dort immer `–`, das Risiko bliebe. Die Rückmeldung liefern Dateisystem, Überwachungsart und die Marke «gehört zu Bibliothek X» |
| Dateisystemklasse | Sie steht am **aufgelisteten Verzeichnis**, nicht an jeder Zeile, und wird über `paths.FSTypeAt` am Deskriptor des `*os.Root` geholt (`fstatfs`). `paths.DetectFS` ist pfadbasiert, folgt Symlinks und läuft am `*os.Root` vorbei; je Zeile wären das bis zu 500 blockierbare Aufrufe pro Anfrage, bei einer Symlink-Zeile einer auf ein Ziel **ausserhalb** der Wurzel |
| Symlinks | **Anzeigen mit Vermerk, nicht betretbar.** Weglassen wäre ruhiger, aber wer den Ordner im Terminal sieht und im Dialog nicht findet, sucht den Fehler bei Slouchcat. Der Scanner folgt ihnen ebenfalls nicht, die Anzeige deckt sich also mit dem, was später in der Bibliothek landet |
| Kosten einer Seite | Eine Anfrage liest die **Namen** des Verzeichnisses (`paths.ReadDirNames`), sortiert sie und holt dann nur für die Zeilen dieser Seite je ein `lstat` (`paths.LstatAll`), dazu ein `fstatfs` auf dem Verzeichnis. Nicht ein `stat` je Eintrag: `os.File.ReadDir` darf dem `d_type` nicht trauen, wenn die Datei durch einen `*os.Root` geöffnet wurde, und statet jeden Eintrag einzeln — bei 50 000 Einträgen wären das 50 000 Netzrunden je Klick und der 3-s-Wächter verlöre jede Bedeutung. Dass die Seite dadurch weniger Zeilen tragen kann als `limit` (ein Ordner voller Dateien), ist der Preis |
| Hängende Einhängungen | Jeder Dateisystemaufruf läuft in einer eigenen Goroutine mit 3-s-Wächter; serverweit **16** gleichzeitige Auflistungen. Der Platz kommt zurück, **wenn die Goroutine zurückkehrt**, nie auf den Wächter hin: Auf einer toten NFS-Einhängung blockiert `getdents` ununterbrechbar im Kern, `context` hilft dort nicht, und ein auf den Wächter freigegebener Platz machte die Begrenzung wirkungslos und die Thread-Leckage unbegrenzt. Als verloren zählt ein Platz erst, wenn der aufgegebene Aufruf auch nach einer Gnadenfrist von einer Minute nicht zurück ist: ein geschlossener Dialog bricht seine Anfrage ab, und eine langsame, aber lebende Freigabe antwortet verspätet — beides ist kein Fehler und darf weder das Dashboard noch den 50 Zeilen grossen Fehlerring füllen. Sind alle Plätze verloren, antwortet der Browser bis zum Neustart 503, und das Dashboard sagt es. 16 und nicht 4, weil Go zehntausend Threads trägt und vier Hänger eines NAS-Schluckaufs den Dialog sonst dauerhaft abschalteten |
| Paketgrenze | Eigenes Paket `internal/browse`, nicht `internal/admin`: `admin` steht in beiden Ausnahmelisten von harter Regel 2, und ein Verzeichnis-Walker gehört nicht in dasselbe Paket, das ungefilterten Titelzugriff haben darf. `browse` darf nur die Standardbibliothek, `internal/paths`, `internal/model` und `internal/library` sehen; eine `depguard`-Regel setzt es durch |
| Geteilte Regeln | `Ignored`, `WatchMode`, `PathProblem` und `ValidateNewRoot` bleiben in `internal/library` und werden von dort benutzt. Zwei Stellen, die für denselben Zustand verschiedene Codes liefern, fängt kein Vertragstest — und der Dialog soll zeigen, was der Scan auch sähe |

Verworfen: (a) **Absoluter Pfad als Parameter**, auch nur für den ersten Aufruf — er benutzt von
`paths` nichts und macht den Endpunkt zu einem Lesezugriff auf den ganzen Container. (b) **Ein
eigener `http.Handler`** am generierten strict server vorbei — er sähe `authorize` nie und wäre
unauthentifiziert aus dem Internet erreichbar (harte Regel 6). (c) **Wurzelliste aus
`/proc/self/mountinfo`** — Heuristik, die in beide Richtungen danebenliegt. (d) **Rekursives Zählen
mit Zeitbudget und Abbruchmarken** — sauber gedacht, aber auf Netzfreigaben immer leer und trotzdem
riskant. (e) **`library.ValidateDir` um die Wurzelregel erweitern** — die Funktion bedient auch
`checkSetupPath` und `RunSetup`; dort muss ein Pfad ausserhalb der Liste noch beschreibbar sein.
(f) **`SelectableList` wiederverwenden** — eine `CheckboxGroup` mit einem Tab-Halt je Zeile ist
genau nicht das Modell, das harte Regel 8 für eine lange Ordnerliste verlangt.

## Folgen

- Eine bestandsgeschützte Wurzel ausserhalb der Liste wird im Dialog **angezeigt** (damit ein
  Ordner die Marke seiner Bibliothek tragen kann), aber nicht betreten.
- Wer eine Installation mit Medien ausserhalb von `/media` betreibt, muss
  `SLOUCHCAT_BROWSE_ROOTS` setzen, bevor er eine Bibliothek dort anlegen kann. Vorhandene
  Bibliotheken laufen unverändert weiter.
- `checkSetupPath` hat einen eigenen Zweig `not_browsable`, der **vor** jedem `os.Stat` greift.
  Ohne ihn verriete der Assistent einem Aufrufer ohne Konto, ob ein Pfad existiert und ob er eine
  Datei ist.
- **Der Einrichtungsassistent gilt mit.** `RunSetup` und `checkSetupPath` prüfen dieselbe Regel,
  und die Vorgabeliste ist nicht leer (`/media`). Eine Installation mit Medien unter `/data` und
  ungesetzter Variable kann die Einrichtung darum **nicht** abschliessen; sie bekommt
  `not_browsable` mit dem Hinweis auf `SLOUCHCAT_BROWSE_ROOTS`. Die Begründung zu E14 im Konzept
  liest sich anders («bei leerer Wurzelliste lässt `ValidateNewRoot` jeden gültigen Pfad durch»),
  meint aber den Fall ohne Vorgabe. Hier gilt die Regel wie entschieden: lieber eine Fehlermeldung,
  die den Variablennamen nennt, als eine Erstinstallation, die sich selbst `/config` freischaltet.
- Der Browser ist der erste Fall, in dem der Server Verzeichnisse **auflistet**. Jede spätere
  Erweiterung (mehrere Wurzeln je Bibliothek, Löschweg, Bibliothek ohne Ordner) erbt diese
  Entscheide; sie sind in Abschnitt 6.7 des Konzepts beschrieben und noch nicht gebaut.
- Restrisiko, benannt statt wegerklärt: Eine gestohlene Hauptkonto-Sitzung kartiert alles, was in
  der Wurzelliste steht — eine Liste, die der Angreifer wegen E14 nicht mehr erweitern kann. Eine
  tote NFS-Einhängung bindet je Versuch dauerhaft einen OS-Thread; die Begrenzung auf 16 nimmt
  dafür in Kauf, dass der Browser bis zum Neustart 503 antwortet. Und ein Betreiber, der `/` in
  die Liste schreibt, steht wieder am Anfang.
