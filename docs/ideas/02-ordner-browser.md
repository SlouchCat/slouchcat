# Idee 02: Ordner-Browser im Bibliotheksformular

- Status: Übernommen (Marco hat die Umsetzung am 25. Oktober 2026 beauftragt; Eintrag im
  Entscheidungsprotokoll `docs/planning/04-entscheidungen.md`). Alle Fragen aus Abschnitt 9 sind
  beantwortet
- Frühestens: nach Version 1.0
- Grundlage: Mockup-Entwurf A vom 23. Oktober 2026 (Dialog mit Liste), geprüft am 25. Oktober 2026
  durch eine Code-Bestandsaufnahme, einen Architekturentwurf, ein Bedrohungsmodell und eine
  Gegenprüfung des Papiers gegen den Code; F-1 bis F-9 von Marco beantwortet am 25. Oktober 2026; zweite Gegenprüfung (Kritik, Sicherheit, Belege) am selben Tag eingearbeitet
- Vorbild: Ordnerwahl in Jellyfin und Emby, jedoch ohne deren freies Blättern durch das ganze Dateisystem

## 1. Problem und Ziel

Das Feld «Ordner» im Formular «Bibliothek hinzufügen» verlangt heute einen absoluten Pfad **im
Container**, eingetippt aus dem Gedächtnis. Der Pfad im Container ist aber nicht der Pfad auf dem Host:
`deploy/compose.yaml:29` bildet `${MEDIA_DIR:-/srv/media}` auf `/media` ab, und auf dem NAS heisst
dieselbe Freigabe noch einmal anders. Wer sich vertippt, bekommt `errors.code.library_path_invalid` und
muss raten. Der Einrichtungsassistent hat dafür den Knopf «Prüfen» (`checkSetupPath`), die Admin-Seite
hat gar nichts.

Ziel: Neben dem Ordnerfeld steht «Durchsuchen …». Der Dialog zeigt Brotkrumen und die Unterordner des
aktuellen Verzeichnisses. Die Kopfzeile nennt Dateisystem, Überwachungsart und die Warnung für
Netzfreigaben des aktuellen Verzeichnisses, jede Zeile die Marke «gehört schon zu Bibliothek X» und,
wo es zutrifft, «Verknüpfung» oder «nicht lesbar». Der Administrator klickt sich zum Ordner statt ihn zu
erraten. Das Textfeld bleibt gleichwertig bestehen — der Dialog ist eine Abkürzung, kein Ersatz.

**Was das Ziel ausdrücklich nicht ist:** ein Dateimanager. Es wird nie durch das ganze Dateisystem des
Containers geblättert, sondern nur unterhalb ausdrücklich freigegebener Wurzeln (Abschnitt 4, E4).

## 2. Zielgruppe und Nutzen

- **Hauptbenutzer als Administrator:** legt eine Bibliothek an, ohne die Abbildung Host → Container im
  Kopf zu haben. Sieht vor dem Anlegen, ob der Ordner auf einer Netzfreigabe liegt (dann `poll` statt
  `inotify`, AN-12 und Entscheid 3) und ob er schon zu einer Bibliothek gehört.
- **Profile und Gäste:** kein Nutzen, kein Zugriff. Die Operation ist `admin`.
- **Künftige Clients:** keiner. Die Operation gehört zur Verwaltung.

Der Nutzen ist kleiner als er wirkt, und das soll hier stehen: Der Pfad wird genau einmal je Bibliothek
eingegeben, bei Marco also vier- bis fünfmal im Leben des Servers. Was den Aufwand trotzdem rechtfertigt,
ist nicht die Tipparbeit, sondern die **Rückmeldung vor dem Anlegen** — Dateisystem und
Überwachungsart des gewählten Ordners, und je Zeile die Überschneidung mit bestehenden Bibliotheken. Wer das nicht braucht, braucht diese Idee nicht.

## 3. Abgrenzung zur Spec v1

| Quelle | Heute | Konzept |
|---|---|---|
| B-1 «Der Admin legt Bibliotheken auf Ordner an» | Pfad wird eingetippt | Pfad wird gewählt; Eintippen bleibt gleichwertig |
| A-1 Einrichtungsassistent | Knopf «Prüfen» ruft `checkSetupPath` | unverändert; **kein** Blättern im Assistenten (Abschnitt 4, E5) |
| A-2 «Verwaltung mit derselben Design-Sorgfalt» | Formular ohne Ordnerwahl | Dialog aus denselben Bausteinen wie die Zuschauerseite |
| `Library.roots` als Liste (`api/openapi.yaml:7346`) | Schema erlaubt mehrere, `LibraryRequest.path` ist einzeln (:7295), `LibraryUpdate` hat kein Pfadfeld (:7309), `Create` legt genau eine Wurzel an | mehrere Wurzeln sind eine **eigene** Fähigkeit, nicht Teil dieser Idee (Abschnitt 4, E8) |
| Harte Regel 4 (`paths`) | gilt für Medienzugriff | gilt unverändert; der Browser ist der erste Fall, der *Verzeichnisse auflistet* |

**Berührte Non-Goals:** keine. Der Browser baut keine Funktion aus der Non-Goal-Liste der Spec und zieht
nichts aus einem Meilenstein vor — Version 1.0 ist veröffentlicht.

**Bewusst ausgeschlossen:**

- Dateien anzeigen (nur Verzeichnisse; sonst wäre eine Liste mit 50'000 Zeilen der Normalfall).
- Ordner anlegen, umbenennen oder löschen. `deploy/compose.yaml:29` bindet die Medien ohnehin
  schreibgeschützt ein (`:ro`); ein schreibender Browser wäre im Normalbetrieb wirkungslos.
- Blättern ausserhalb der freigegebenen Wurzeln, auch nicht «nur lesend».
- Ein Browser für Hostpfade (Radarr/Sonarr-Umsetzungstabelle, M9) — eigene Sache.

## 4. Entwurfsentscheide

Die Prüfungen (Bestandsaufnahme, Architektur, Sicherheit) widersprachen sich an drei Stellen; die
Gegenprüfung fand danach acht weitere Schwächen. Die Auflösung steht hier. Die Zeilen mit Datum hat Marco
entschieden, die übrigen sind Entwurfsentscheide aus der Prüfung.

| Nr. | Frage | Entscheid | Begründung |
|---|---|---|---|
| **E1** | Parameterform | **`rootId` plus relativer Pfad**, kein absoluter Pfad. Der leere `rel` wird vor `paths.Resolve` zu `"."` normalisiert | Ein absoluter Pfad im Parameter benutzt von `paths` nichts: `Resolve` braucht eine Wurzel und lehnt absolute Pfade gerade ab (`paths.go:36`). Übrig bliebe ein `os.OpenRoot` auf genau den Pfad, den der Client nennt — also `path=/config`. Mit `rootId` plus `rel` läuft jede Anfrage durch `paths.Resolve` und dasselbe `*os.Root`, und die Garantie aus `TestOpenAndWalkStayInsideRoot` hält über Anfragegrenzen hinweg. Die Normalisierung ist nötig, weil `Resolve` den leeren `rel` ablehnt, `"."` aber annimmt |
| **E2** | Methode | **`browseServerPaths` als POST mit CSRF-Kopf**, wie `checkSetupPath`. Die beiden Wurzellisten bleiben GET | Für `browse` zählt das Logging-Argument: `requestLog` schreibt den Query-String als Attribut `query` (`middleware.go:64`), und nur `path` läuft durch `RedactString`. Medienpfade mit Benutzernamen stünden sonst im Fehlerring des Dashboards, in der Browser-Historie und im `Referer`. Dazu greift bei POST `csrfOK` (`session.go:70`), bei GET nicht. Für die Wurzellisten greift beides nicht: sie tragen keinen Pfad im Query-String, und ein Auslösen von aussen kostet nichts, weil sie nicht auf das Dateisystem greifen |
| **E3** | Mediendateien zählen | **Gar nicht** (Marco, 25. Oktober 2026; auch nicht später) | Der Architekturentwurf schlägt ein Zeitbudget von 1500 ms je Anfrage mit Abbruchmarken vor. Das ist sauber, aber Entscheid 3 hält fest: SMB und NFS sind bei Marco der **Hauptfall**, nicht die Ausnahme — und auf Netzdateisystemen wird aus gutem Grund nicht gezählt. Die Zahl wäre auf genau seinem Aufbau immer `–`, das Risiko bliebe. Darum fällt sie ganz weg; die Rückmeldung liefern Dateisystem, Überwachungsart und Bibliotheksmarke |
| **E4** | Wurzelliste | **`SLOUCHCAT_BROWSE_ROOTS`, komma-getrennt, Vorgabe `/media`, vereinigt mit den Wurzeln vorhandener Bibliotheken.** Zusammen mit E14, das die Liste erst zur echten Grenze macht | Fest `/media` sperrt jeden aus, der eine zweite Einhängung hat, ohne Abhilfe ausser Code. Die Einhängetabelle (`/proc/self/mountinfo`) abzuleiten ist Heuristik: zu streng fehlen die Medien, zu locker steht `/config` mit Datenbank und `secret.key` im Dialog. Nur aus Bibliothekswurzeln abzuleiten ist zirkulär, weil die erste Wurzel ja gerade angelegt wird |
| **E5** | Einrichtungsassistent | **Kein Blättern.** Stattdessen `GET /setup/roots`: die konfigurierten Wurzeln, parameterlos, nichts darunter, **ohne** Anrechnung auf `AllowAttempt` | `checkSetupPath` ist ein Ja/Nein-Orakel über einen Pfad, den der Betreiber selbst getippt hat; bei 10 Versuchen je Minute ist Raten aussichtslos. Eine Auflistung ist ein Aufzählungsprimitiv: jede Anfrage liefert eine ganze Ebene. Dazu zwei Verschärfungen: die Drossel zählt je Client-Adresse, und bei `SLOUCHCAT_TRUSTED_PROXIES=private` mit veröffentlichtem Port 8096 wählt der Angreifer diese Adresse selbst (Befund M-1 des Sicherheitsreviews 1.0); und das Fenster vor dem ersten Konto ist der verletzlichste Moment des Servers, weil `POST /setup` dort `public` ist. Die Anrechnung fällt weg, weil `AllowAttempt` **denselben** Topf benutzt wie Anmeldung, Kontoanlage, PIN und Einladungen (`auth.go:245`, `account.go:48`, `pin.go:237`, `invitations.go:213`): eine Liste, die beim Laden des Assistenten geholt wird, hätte nach zehn Neuladen die Einrichtung selbst mit 429 blockiert. Die Antwort ist parameterlos und unveränderlich, also kein Orakel. **Zwei Auflagen:** `listSetupRoots` antwortet aus einer beim Start gefüllten Liste und macht **keinen** Systemaufruf im Anfragepfad — sonst wäre ausgerechnet die einzige unauthentifizierte Operation, die das Dateisystem berührt, auch die einzige ohne Drossel und ohne Semaphor. Und sie bekommt einen **eigenen** Drosseltopf (etwa 60 Anfragen je Minute und Client-Adresse), nicht den von `AllowAttempt`. Was bleibt, ist ehrlich zu benennen: Vor dem ersten Konto erfährt jeder die Pfade der Einhängungen. Das ist hinnehmbar, weil `/media` in `docs/INSTALLATION.md` ohnehin öffentlich steht; der Dateisystemtyp wird im Assistenten darum **nicht** mitgeliefert |
| **E6** | Fehlerform | Unbekanntes `rootId` **und** `rel`, das nichts bezeichnet: **identische 404** `path_not_browsable`. Nicht lesbar oder kein Verzeichnis **innerhalb** einer Wurzel: **200 mit `problem`** | Harte Regel 2: «Nicht sichtbar heisst 404, nie 403.» Ein eigener Code für «existiert, aber ausserhalb» wäre ein Existenzorakel über den ganzen Container. Die Unterscheidung `missing`/`not_directory`/`not_readable` bleibt bei `checkSetupPath`, wo sie hingehört. Die Wurzelprüfung muss **lexikalisch und vor jedem Dateisystemzugriff** laufen, sonst trennt schon die Antwortzeit die beiden Fälle |
| **E7** | Inhalt der Liste | Nur Verzeichnisse, nach Namen sortiert, Seiten über `cursor`/`nextCursor` (Vorgabe 200, Höchstwert 500), `truncated` ab 50'000 gelesenen Einträgen | `getdents` liefert unsortiert, also muss der Handler für stabile Seiten alle Namen lesen. 50'000 Namen sind rund 5 MB. Darüber bricht er ab und sagt es; der Pfad lässt sich weiterhin eintippen. `library.Ignored` (`names.go:51`) filtert `@eaDir`, `#recycle`, `lost+found`, `$RECYCLE.BIN`, `BDMV`, `VIDEO_TS` und Verstecktes weg, damit die Liste zeigt, was der Scan auch sähe |
| **E8** | Mehrere Wurzeln je Bibliothek | **Anschlussfähigkeit, eigene Schritte 12 bis 14**, seit der Antwort auf F-6 beziffert | Das Mockup legte nahe, das sei bloss Freilegen von Vorhandenem. Stimmt zur Hälfte: `Library.roots` ist ein Array, aber `LibraryRequest` trägt ein einzelnes `path`, `LibraryUpdate` gar keines, `library.Service.Create` legt genau eine Wurzel an, und es gibt keine Operation zum Hinzufügen oder Entfernen. Der Löschweg ist mit E12 und E13 geklärt |
| **E9** | Hängende Netzeinhängungen | Jeder Dateisystemaufruf in einer eigenen Goroutine, Wächter mit 3 s, sonst 503 `path_unreachable`. Serverweit höchstens **16** gleichzeitige Browse-Anfragen, darüber 503 `browse_busy`. Der Platz wird freigegeben, **wenn die Goroutine zurückkehrt**; ein hart blockierter Aufruf kehrt nie zurück und behält ihn damit von selbst | Auf einer toten NFS-Einhängung blockiert `getdents` oder `statfs` ununterbrechbar im Kern; `context.WithTimeout` hilft dort **nicht**, Go kann einen blockierten Systemaufruf nicht abbrechen. Die Goroutine ist verloren. Gäbe der Wächter den Platz frei, wäre die Begrenzung wirkungslos und die Thread-Leckage unbegrenzt. Darum die andere Hälfte des Handels, die Marco abgenommen hat (F-9): **nach so vielen hängenden Aufrufen antwortet der Browser bis zum Neustart nur noch 503**, mit einer Dashboard-Warnung, damit es nicht wie ein Fehler aussieht. Ein toter Browser ist besser als ein Server, der unter Threads erstickt. Die Obergrenze ist 16 und nicht 4, weil Go zehntausend Threads trägt und vier Hänger eines NAS-Schluckaufs den Dialog sonst dauerhaft abschalten. Der Verlust eines Platzes wird **einmal** auf `Error` protokolliert, `browse_busy` auf `Debug`: der Fehlerring fasst 50 Einträge, ein Dialog im Sekundentakt spülte echte Fehler heraus. Verschärfend: `http.Server` setzt heute nur `ReadHeaderTimeout: 10 * time.Second` und `IdleTimeout: 2 * time.Minute`, **kein `WriteTimeout`** (`main.go:369`) |
| **E10** | Listenbaustein | Neue `web/src/ui/PathList.tsx` über `ListBox`/`ListBoxItem` aus `react-aria-components` | `SelectableList` ist eine `CheckboxGroup` mit einem Tab-Halt je Zeile — das ist genau nicht das Modell, das harte Regel 8 verlangt. `ListBox` bringt wandernden `tabindex`, Pfeiltasten, Home/End und `onAction` bei Enter mit; die Abhängigkeit ist mit 1.22.1 vorhanden, **wird aber im Projekt noch nirgends benutzt** — es ist ein neuer Primitivbaustein, kein Wiederverwenden |
| **E11** | Dialograhmen | `FormDialog` bekommt eine neue, optionale Eigenschaft `initialFocus` (Ref oder Selektor) und eine `onClose`-Rückgabestelle für den Fokus | `focusFirstField` (`Dialog.tsx:94-103`) sucht `input, textarea, select, input:checked` und fällt sonst auf `input, button` zurück. Eine `ListBoxItem` ist keines von beiden, der Fokus landete also auf der ersten Brotkrume statt auf der Zeile. Und react-aria gibt den Fokus beim Schliessen an den Auslöser zurück (so zugesagt in `Dialog.tsx:79-82`), während der Nutzerfluss ihn im Textfeld haben will. Beides ist Arbeit an einer geteilten Komponente und muss budgetiert sein |
| **E12** | Wurzel entfernen, an der Titel hängen | **Löschen, mit Sicherheitsabfrage und abgetipptem Pfad** (Marco, 25. Oktober 2026). In **einer** Transaktion, unter der Scan-Sperre der Bibliothek: Titel der Wurzel ohne Datei unter einer anderen Wurzel weich löschen, dann leere Staffeln und Serien, dann die Wurzelzeile löschen. Was das Löschen **nicht** mitnimmt, steht in 6.7 | Marcos Entscheid gegen die beiden Alternativen «wie Verlustschutz» (24 Stunden Gnadenfrist, Wiederherstellung beim erneuten Hinzufügen) und «Verbot, solange Titel da sind». Die Reihenfolge in der Transaktion ist nicht Geschmack: `media_files.root_id` trägt `ON DELETE CASCADE` (`00003_libraries_items.sql:99`), und `SoftDeleteItemsMissingSince` verlangt `AND EXISTS (SELECT 1 FROM media_files …)` (`queries/system/items.sql:53`). Wer nur die Wurzelzeile löscht, hinterlässt Titel mit null Dateien, die der stündliche Sweep **nie** anfasst — unverfügbar, nicht gelöscht, für immer in Zählung und Listen. Ein Titel mit Dateien unter einer zweiten Wurzel derselben Bibliothek bleibt verfügbar. Die Scan-Sperre muss sein, weil `scanLibrary` die Wurzeln einmal lädt und danach `media_files` mit dieser `root_id` schreibt — eine Wurzel, die mitten im Lauf verschwindet, lässt den Scan in einen Fremdschlüsselfehler laufen |
| **E13** | Letzte Wurzel einer Film-, Serien- oder Hörbuchbibliothek | **Entfernen erlaubt; die Bibliothek bleibt ohne Ordner bestehen** (Marco, 25. Oktober 2026) | Damit gibt es den Zustand «Videobibliothek ohne Wurzel», den heute nur Podcasts kennen. Scanner, Watcher, Dashboard und Oberfläche müssen ihn aushalten: kein Scan, keine Überwachung, kein `root_missing`-Abbruch, und ein eigener Text statt `admin.libraries.noRoots` («Kein Ordner: Die Titel kommen aus abonnierten Feeds»), der hier falsch wäre |
| **E14** | Pfad beim Anlegen | **Neue Bibliotheken und neue Wurzeln müssen unter einer konfigurierten Wurzel liegen** (Marco, 25. Oktober 2026), geprüft **lexikalisch und danach auf dem mit `filepath.EvalSymlinks` aufgelösten Pfad**. Die Regel sitzt **nicht** in `library.ValidateDir`, sondern als `Service.ValidateNewRoot` eine Ebene höher. Bestandsschutz heisst: Pfadgleichheit mit einer bestehenden `library_roots`-Zeile, nie ein Präfix | Ohne das ist die Wurzelliste keine Grenze: Wer die Admin-Sitzung hat, legt eine Bibliothek auf `/config` an, und weil E4 Bibliothekswurzeln in die Browse-Liste hineinvereinigt, hat er sich damit `/config` selbst freigeschaltet. Mit E14 ist die Kette durchtrennt — aber nur mit der Realpfad-Prüfung: `paths.Open` ruft `os.OpenRoot(root.dir)`, und der Wurzelpfad **selbst** wird vom Kern samt Symlinks aufgelöst. Eine rein lexikalische Prüfung liesse `/media/ablage -> /config` durch. Die Regel sitzt im Dienst und nicht in `ValidateDir`, weil `ValidateDir` auch `checkSetupPath` und `RunSetup` bedient: in `ValidateDir` könnte eine Installation mit Medien unter `/data` und ungesetzter Variable die Einrichtung gar nicht mehr abschliessen. Bei leerer Wurzelliste lässt `ValidateNewRoot` darum jeden gültigen Pfad durch und der Start schreibt eine Warnung; Bestandsschutz als Präfix wäre ohnehin toter Code, weil `Create` überlappende Pfade schon heute ablehnt. Zwei Folgen, die dastehen müssen: eine bestandsgeschützte Wurzel ausserhalb der Liste wird im Dialog **angezeigt** (für die Marke «gehört zu …»), aber nicht betreten; und wer sie mit E12 entfernt, kann sie ohne Konfigurationsänderung nicht wieder hinzufügen |
| **E15** | Symlinks | **Anzeigen mit dem Vermerk «Verknüpfung, wird nicht durchsucht», nicht betretbar** (Marco, 25. Oktober 2026) | Betreten führte aus der Wurzel heraus; tragend ist dabei nicht `*os.Root` (das verweigert nur Symlinks, die die Wurzel **verlassen**), sondern dass `fs.WalkDir` ihnen nicht folgt. Weglassen wäre ruhiger, aber wer den Ordner im Terminal sieht und im Dialog nicht findet, sucht den Fehler bei Slouchcat. Der Scanner folgt ihnen ebenfalls nicht, die Anzeige deckt sich also mit dem, was später in der Bibliothek landet |

## 5. Nutzerfluss

1. Der Administrator öffnet **Verwaltung → Bibliotheken**, füllt «Name», wählt «Inhalt».
2. Er drückt **«Durchsuchen …»** neben dem Ordnerfeld. Der Dialog öffnet, der Fokus landet in der Liste:
   auf der Zeile, die zum aktuellen Feldwert passt, sonst auf der ersten (E11).
3. Ohne Pfad im Feld zeigt der Dialog die **Wurzeln** (konfiguriert plus Bibliothekswurzeln). Mit Pfad
   zeigt er dessen Verzeichnis.
4. Pfeiltasten bewegen, **Enter** oder **Pfeil rechts** steigt ab, **Rücktaste** oder **Pfeil links**
   geht hinauf, an einer Wurzel zurück zur Wurzelliste. Die Kopfzeile zeigt immer Pfad, Dateisystem und
   Überwachungsart des **aktuellen** Verzeichnisses — auch wenn es keine Unterordner hat.
5. Zeilen tragen bei Bedarf eine Marke: «gehört zu Bibliothek Filme», «Verknüpfung, wird nicht
   durchsucht» (Symlinks erscheinen, sind aber nicht betretbar), «nicht lesbar». Dateisystem und
   Netzfreigabe-Warnung stehen in der Kopfzeile, nicht je Zeile: sie je Zeile zu ermitteln hiesse bis zu
   500 blockierbare `statfs`-Aufrufe pro Anfrage (6.4).
6. **«Übernehmen»** schreibt den Pfad ins Feld, schliesst den Dialog und setzt den Fokus **auf das
   Textfeld**, nicht zurück auf den Knopf — sonst muss der Benutzer zurück-tabben, um nachzubessern.
7. **Escape** oder «Abbrechen» schliesst ohne Änderung; der Fokus geht zurück auf «Durchsuchen …».
8. Abschicken wie heute. Überschneidet der Pfad eine bestehende Wurzel, antwortet der Server wie bisher
   mit `422`; die Marke im Dialog hat das vorher sichtbar gemacht.

Ist die Wurzelliste leer (keine konfigurierte Wurzel vorhanden, keine Bibliothek angelegt), bleibt der
Knopf sichtbar, der Dialog zeigt einen eigenen Leerzustand mit dem Namen der Umgebungsvariablen. Der
Knopf verschwindet nicht, sonst sucht der Administrator eine Funktion, die ihm beschrieben wurde.

Im **Einrichtungsassistenten** ändert sich nur eines: Unter dem Ordnerfeld steht die Liste der
konfigurierten Wurzeln als anklickbare Vorschläge (aus `listSetupRoots`). Kein Abstieg, keine
Unterordner. Der Knopf «Prüfen» bleibt.

## 6. Datenmodell und API-Skizze

Kein neues Datenmodell, keine Migration, keine neue SQL-Abfrage: Die Bibliothekswurzeln liefert
`Libraries.List` (`admin.go:62`, `model.Library.Roots` mit `Path`, `FSType`, `Watch`, `Warning`).

### 6.1 Konfiguration

`SLOUCHCAT_BROWSE_ROOTS`, **komma-getrennt** wie `SLOUCHCAT_TRUSTED_PROXIES` (`config.go:349`), Vorgabe
`/media`, mit Flagge `--browse-roots` wie jeder andere Wert in `config.Load`. Feld in `config.Config`,
Aufnahme in `Config.LogValue`. Beim Start wird jede Wurzel geprüft (absolut, vorhanden, Verzeichnis);
eine fehlende wird protokolliert und übersprungen, **nicht** zum Startfehler, sonst hindert ein
Tippfehler den Server am Hochfahren. Steht `/` in der Liste, schreibt der Start eine Warnung, wie es
`config.TrustedProxyWarning` vormacht.

### 6.2 Admin-Operationen

```yaml
  /admin/fs/browse:
    post:
      operationId: browseServerPaths
      summary: Directories below a browsable root, for the folder field (A-2)
      description: |
        Lists the subdirectories of a directory below a browsable root, so the administrator picks a
        library folder instead of typing it. The request names a root by its id and a path relative to
        it, never an absolute path: the handler resolves it with the `paths` package and reads it
        through the `*os.Root` of that root, so a symlink that leaves the root is not browsable. An
        empty `rel` lists the root itself; `listBrowseRoots` names the roots. Every row carries its file
        system, the watch mode the scanner would choose, the warning for network shares and the library
        the directory already belongs to. Directories are sorted by name and never counted; `cursor` and
        `nextCursor` page through a large directory, `truncated` says that it holds more entries than
        the server reads. A path below no root and a path that does not exist answer alike, so the
        endpoint tells nothing about the filesystem outside the roots.
      tags: [admin]
      x-visibility: admin
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/BrowseRequest"
      responses:
        "200":
          description: The directory with its subdirectories.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/DirectoryListing"
        "401":
          $ref: "#/components/responses/Unauthenticated"
        "403":
          $ref: "#/components/responses/AdminOnly"
        "404":
          description: |
            No such root, or `rel` names nothing below it (`path_not_browsable`). Both answer alike.
          content:
            application/problem+json:
              schema:
                $ref: "#/components/schemas/Problem"
        "422":
          description: |
            Validation failed (`validation_failed`): `rel` is absolute, holds a `..` segment or a NUL
            byte, is longer than 4096 bytes, or `cursor` does not belong to this directory.
          content:
            application/problem+json:
              schema:
                $ref: "#/components/schemas/Problem"
        "503":
          description: |
            The file system did not answer within three seconds (`path_unreachable`; a dead network
            mount blocks in the kernel and cannot be cancelled), or too many listings are already
            running (`browse_busy`).
          content:
            application/problem+json:
              schema:
                $ref: "#/components/schemas/Problem"

  /admin/fs/roots:
    get:
      operationId: listBrowseRoots
      summary: The roots the folder picker may browse (A-2)
      description: |
        The configured roots (`SLOUCHCAT_BROWSE_ROOTS`) together with the roots of the libraries that
        exist; the dialog starts here and `browseServerPaths` goes below them. An empty list means that
        no configured root exists and no library has been created yet.
      tags: [admin]
      x-visibility: admin
      responses:
        "200":
          description: The browsable roots.
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/BrowseRoot"
        "401":
          $ref: "#/components/responses/Unauthenticated"
        "403":
          $ref: "#/components/responses/AdminOnly"
```

### 6.3 Assistent-Operation

```yaml
  /setup/roots:
    get:
      operationId: listSetupRoots
      summary: The configured media roots, for the wizard (A-1)
      description: |
        The roots the wizard offers below the folder field (`SLOUCHCAT_BROWSE_ROOTS`), so the operator
        need not guess how the mount is named inside the container. It takes no parameter and lists
        nothing below a root: before an account exists, a listing with a path parameter would be an
        enumeration primitive, and the attempt limit is per client address, which a misconfigured
        `SLOUCHCAT_TRUSTED_PROXIES` lets the caller choose. The answer is the same for every caller and
        never changes while the server runs, so it does not count against the attempt limit either —
        that limit is shared with signing in, and the wizard fetches this list on every load. Allowed
        only while no account exists.
      tags: [setup]
      x-visibility: public
      security: []
      responses:
        "200":
          description: The configured roots.
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/BrowseRoot"
        "409":
          description: Setup already completed (`setup_done`).
          content:
            application/problem+json:
              schema:
                $ref: "#/components/schemas/Problem"
```

### 6.4 Schemata

Die Aufzählungen `watch` und `warning` kommen mehrfach vor und werden darum als eigene Schemata
herausgezogen. **Sie stehen nur am aufgelisteten Verzeichnis, nicht an jeder Zeile:** `paths.DetectFS`
ist `syscall.Statfs` auf einem Pfad (`fstype_linux.go:10`), folgt also Symlinks und läuft am `*os.Root`
vorbei. Je Zeile hiesse das bis zu 500 solche Aufrufe pro Anfrage, jeder einzeln blockierbar — und bei
einer Symlink-Zeile läse er den Dateisystemtyp des Ziels **ausserhalb** der Wurzel. Unterordner erben
das Dateisystem ihres Elternverzeichnisses bis auf Einhängepunkte ohnehin. **Ohne eigene `x-enum-varnames` bricht `task gen`:** oapi-codegen erzeugt aus einem Enum
ohne Varnames paketweite, unpräfixierte Konstanten, und `Inotify`, `InotifyPoll`, `FuseUnknown`,
`NetworkShare` sind in `server/internal/api/gen/api.gen.go:855-857` und `:1047-1048` bereits vergeben.
Genau darum trägt `PathCheck.watchMode` schon heute `x-enum-varnames` (`openapi.yaml:8425`), während
`LibraryRoot.watch` ohne auskommt — es war zuerst da.

```yaml
    BrowseWatch:
      type: string
      enum: [inotify, poll, inotify_poll]
      x-enum-varnames: [BrowseWatchInotify, BrowseWatchPoll, BrowseWatchInotifyPoll]
      description: Change detection the scanner would choose for this directory (plan 3.4).

    BrowseWarning:
      type: string
      enum: [network_share, fuse_unknown]
      x-enum-varnames: [BrowseWarnNetworkShare, BrowseWarnFUSEUnknown]
      description: A hint, not an error; the library works with polling.

    BrowseRequest:
      type: object
      required: [rootId]
      properties:
        rootId:
          type: string
          description: Id of a root from `listBrowseRoots`; stable for the lifetime of the server process.
          maxLength: 64
        rel:
          type: string
          description: |
            Directory below the root, slash-separated and relative; empty or absent lists the root
            itself. Absolute paths, `..` segments and NUL bytes are rejected (`validation_failed`).
          maxLength: 4096
          examples: ["Filme/Action"]
        cursor:
          type: string
          description: "`nextCursor` of the previous page; without it the first page is returned."
          maxLength: 4096
        limit:
          type: integer
          minimum: 1
          maximum: 500
          default: 200

    BrowseRoot:
      type: object
      required: [id, path, source]
      properties:
        id:
          type: string
          description: Opaque id for `BrowseRequest.rootId`.
        path:
          type: string
          description: Absolute path of the root, as it is shown to the administrator.
        browsable:
          type: boolean
          description: |
            False for a grandfathered library root that lies below no configured root: the dialog shows
            it so a directory can carry its library's name, but it cannot be entered.
        source:
          type: string
          enum: [configured, library]
          x-enum-varnames: [BrowseRootConfigured, BrowseRootLibrary]
          description: "`configured`: from `SLOUCHCAT_BROWSE_ROOTS`; `library`: a root of an existing library."
        fsType:
          type: string
          description: Detected file system class (`ext4`, `xfs`, `btrfs`, `zfs`, `fuse`, `nfs`, `smb`, `unknown`), as in `LibraryRoot`.
        warning:
          $ref: "#/components/schemas/BrowseWarning"
          description: Omitted by `listSetupRoots`, which answers before any account exists.

    DirectoryListing:
      type: object
      description: One directory below a browsable root, with its subdirectories.
      required: [rootId, rel, atRoot, entries]
      properties:
        rootId:
          type: string
        rel:
          type: string
          description: The listed directory relative to the root; empty at the root itself.
        path:
          type: string
          description: Absolute path of the listed directory, ready for the folder field.
        parentRel:
          type: string
          description: One level up, relative to the root; absent at the root.
        atRoot:
          type: boolean
          description: "`rel` is empty: there is no way further up, the dialog goes back to the roots."
        fsType:
          type: string
        watch:
          $ref: "#/components/schemas/BrowseWatch"
        warning:
          $ref: "#/components/schemas/BrowseWarning"
        problem:
          type: string
          enum: [not_readable, not_directory]
          x-enum-varnames: [BrowseNotReadable, BrowseNotDirectory]
          description: |
            The directory lies below a root but could not be listed; `entries` is then empty. A path
            below no root answers 404 instead.
        truncated:
          type: boolean
          description: The directory holds more than 50 000 entries; the server read the first 50 000.
        nextCursor:
          type: string
          description: Pass as `cursor` for the next page; absent on the last page.
        entries:
          type: array
          items:
            $ref: "#/components/schemas/DirectoryEntry"

    DirectoryEntry:
      type: object
      description: One subdirectory as the folder picker shows it.
      required: [name, rel]
      properties:
        name:
          type: string
          description: Name of the subdirectory, without its path.
        rel:
          type: string
          description: Path of the subdirectory relative to the root, for the next `BrowseRequest`.
        path:
          type: string
          description: Absolute path of the subdirectory, ready for the folder field.
        symlink:
          type: boolean
          description: |
            A symlinked directory: it is listed, because the scanner sees it too, but it cannot be
            entered (`path_not_browsable`), because that would leave the root.
        libraryId:
          type: integer
          format: int64
          description: The library this directory already belongs to; absent otherwise. A new library may not overlap it.
        libraryName:
          type: string
```

### 6.5 Server-Pakete

Neues Paket **`server/internal/browse`**, nicht in `internal/admin`. Grund: `admin` steht in beiden
Ausnahmelisten von harter Regel 2 (`server/.golangci.yml:39` und `server/tools/itemsaccess/itemsaccess.go:28`). Ein
Verzeichnis-Walker gehört nicht in dasselbe Paket, das ungefilterten Titelzugriff haben darf; diese
Liste soll klein und begründbar bleiben.

`browse` soll `internal/paths`, `internal/model`, die Standardbibliothek und aus `internal/library` nur
`Ignored` und `WatchMode` benutzen. **Achtung:** Ein Go-Import zieht das ganze Paket `library` mit —
Scanner, Watcher, `fsnotify` und dessen eigene Importe. Und die Beschränkung setzt heute **kein** Linter
durch: `server/.golangci.yml` verbietet nur `systemq` ausserhalb der sieben Pakete und `os/exec` ausserhalb
`transcode`. Wenn die Grenze halten soll, braucht `browse` eine eigene `depguard`-Regel — oder `Ignored`
und `WatchMode` wandern in ein kleines Paket ohne Abhängigkeiten. Das ist Teil von Schritt 4.

`browse` fragt nichts ab; die Wurzeln reicht der Aufrufer herein:

```go
// Root is a directory the picker may browse, with the library it already belongs to.
type Root struct {
    ID          string
    Dir         string
    LibraryID   int64  // 0 for a configured root that is no library root
    LibraryName string
}

func List(ctx context.Context, roots []Root, req Request) (model.DirectoryListing, error)
```

Die Richtlinie (welche Wurzeln, welches Budget, der Semaphor) bleibt in `internal/admin/browse.go`.

In `internal/paths` fehlt **eine Ebene lesen**. `Walk` läuft nur ab `"."` und steigt rekursiv ab
(`root.go:33`) — auf Marcos Baum mit rund 8'000 Ordnern wäre das je Dialogklick ein Vollscan. Gebraucht
wird kein Walker, sondern:

```go
// ReadDir reads one level of the subtree rel of root (slash-separated, "." for the root itself)
// through the root's *os.Root, so a symlink that leaves the root is not followed.
func ReadDir(root Root, rel string) ([]fs.DirEntry, error)
```

Das Muster steht bereits in `library.ValidateDir` (`service.go:146-160`): `paths.Open(root)` →
`r.Open(".")` → `dir.ReadDir(n)`. Es in `paths` zu heben hält harte Regel 4 und
`internal/paths/doc.go` ein.

Dazu eine zweite neue Funktion, damit auch die Dateisystemklasse des aufgelisteten Verzeichnisses nicht
über einen zusammengesetzten Pfad ermittelt wird:

```go
// FSTypeAt classifies the file system of rel through the root's own descriptor (fstatfs), so no
// path is resolved and no symlink is followed.
func FSTypeAt(root Root, rel string) (FSType, error)
```

`paths.DetectFS` bleibt für die konfigurierten Wurzeln beim Start; im Anfragepfad wird es nicht mehr
aufgerufen.

Ausserdem: `admin.pathProblem` (`setup_checks.go:74`) ist nicht exportiert und liefert `missing`,
`not_directory`, `not_readable`. `browse` braucht dieselben Codes. Vorschlag: nach
`library.PathProblem` verschieben und `setup_checks.go` darauf umstellen — zwei Stellen, die für
denselben Zustand verschiedene Codes liefern, fängt kein Vertragstest.

`Cache-Control: no-store` auf der 200er-Antwort setzt `problem.go:44` **nicht** (dort nur für
Problem-Antworten). Am strict server braucht es dafür einen handgeschriebenen `Visit…Response`, Vorbild
`auth_handlers.go:70-84`.

### 6.6 Web-Seite

Alles unter `web/src/features/shared/`, weil Assistent und Admin-Seite dieselbe Sache brauchen, genau
wie `LibraryFields.tsx` heute schon geteilt ist.

- `web/src/features/shared/PathBrowserDialog.tsx` — Dialograhmen, Brotkrumen, Fusszeile
- `web/src/features/shared/PathBrowserButton.tsx` — der Knopf samt Zustand `isOpen`
- `web/src/features/shared/pathBrowser.ts` — `infiniteQueryOptions`, Typen
- `web/src/ui/PathList.tsx` + `.module.css` + `.stories.tsx` — die Liste
- `web/src/ui/Dialog.tsx` — neue Eigenschaft `initialFocus` und Fokusrückgabe (E11)

**Keine neue Eigenschaft an `LibraryFields`:** der Knopf geht durch den vorhandenen Schlitz `pathAddon`
(`LibraryFields.tsx:51`), den der Assistent für «Prüfen» bereits benutzt (`SetupPage.tsx:265`).
**Aber:** `AddLibraryForm` übergibt heute weder `path` noch `onPathChange` noch `pathAddon`
(`LibrariesPage.tsx:60-65`), das Ordnerfeld ist dort unkontrolliert. «Übernehmen schreibt den Pfad ins
Feld» verlangt also, dass die Admin-Seite den Pfad in den Zustand nimmt — so wie der Assistent es schon
tut. Das ist Teil von Schritt 8.

Wiederverwendet werden `FormDialog`, `AriaButton`, `Icon` (`folder`, `warning`, `chevronRight`, `back`
sind vorhanden, `Icon.tsx:19,21,39,45`), `Notice`, `Skeleton`, `Text`.

`PathList` wird aus `web/src/ui/aria.ts` exportiert, **nicht** aus `index.ts`: react-aria bleibt laut
Kommentar dort aus dem Startbündel (P-10).

Abfrageschlüssel `['fs','browse',scope,rootId,rel]`, `staleTime: 30_000`,
`placeholderData: keepPreviousData` — ein leerer Rahmen beim Wechsel nimmt der `ListBox` den Fokus, und
der Tastaturtest sieht Fokusverlust. Entwertet wird nur im `onSuccess` von `createLibrary`, damit die
Marke «gehört zu …» beim nächsten Öffnen stimmt. `enabled: isOpen`, damit das Schliessen die laufende
Anfrage über `signal` abbricht.

Texte unter `library.browse.*` in `de.json` und `en.json`: `title`, `open`, `apply`, `upOneLevel`,
`roots`, `empty`, `emptyHint`, `loadMore`, `truncated`, `symlink`, `notReadable`, `inLibrary`,
`unreachable`, `busy`, dazu `errors.code.path_not_browsable`.

Der generierte TypeScript-Client zieht drei Operationen und sechs Schemata nach
`packages/api-client/src/generated/schema.d.ts` — nur über `task gen`, nie von Hand (harte Regel 1).

### 6.7 Mehrere Wurzeln und der Löschweg (E8, E12, E13)

Eigene Fähigkeit, eigene Schritte. Sie braucht zwei Operationen; die zweite ist die erste destruktive
Operation an einer Bibliothek — `deleteLibrary` gibt es bis heute nicht. Destruktiver Code existiert
sonst nur in der Pflege (`AdminDeleteItem` beim Zusammenführen, `maintenance.go:513`, und `IgnoreFiles`,
`service.go:361`); von dort stammt das Muster.

```yaml
  /admin/libraries/{id}/roots:
    post:
      operationId: addLibraryRoot
      summary: Add a folder to a library (B-1)
      description: |
        Adds a second or further directory to a library; the scan and the watcher pick it up at once.
        The path must lie below a configured browsable root (E14), must not overlap a root of this or
        another library, and a podcast library takes no root.
      tags: [admin]
      x-visibility: admin
      responses:
        "201": { description: The library with its new root. }
        "422": { description: "`library_path_invalid`: not absolute, not readable, below no browsable root, or overlapping an existing root." }

  /admin/libraries/{id}/roots/{rootId}/removal:
    post:
      operationId: removeLibraryRoot
      summary: Remove a folder from a library and delete its titles (B-1)
      description: |
        Removes the directory from the library. The titles that live only there leave the library:
        they are soft-deleted, so they vanish from every list, and their offline downloads are dropped
        on the devices that hold them. Their progress, watchlist entries and picks stay attached to the
        soft-deleted title, as they do for a detached disk; a title that carries a provider id comes
        back with all of it when the same folder is added again within 30 days, a title without one
        comes back as a new title. `confirmPath` must repeat the root's path exactly, so a mis-aimed
        click cannot do it (`confirm_mismatch`). The library may lose its last root and then holds no
        folder at all; its settings stay. A POST, not a DELETE, because the confirmation travels in
        the body and a DELETE body is dropped by some proxies and HTTP clients.
      tags: [admin]
      x-visibility: admin
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [confirmPath]
              properties:
                confirmPath:
                  type: string
                  description: The root's path, repeated by the administrator.
                  maxLength: 4096
      responses:
        "200": { description: The library without the root. }
        "404": { description: "No such library or root (`not_found`)." }
        "422": { description: "`confirmPath` does not match the root (`confirm_mismatch`)." }
        "409": { description: "A scan of this library is running (`scan_running`); it holds the roots it started with." }
```

Der Handler nimmt zuerst die **Scan-Sperre** der Bibliothek (`scanLocks`) und macht dann in **einer**
Transaktion und in dieser Reihenfolge:

1. Titel der Wurzel weich löschen, die keine Datei unter einer anderen Wurzel derselben Bibliothek
   haben (`items.deleted_at`), danach leere Staffeln und Serien — dieselben Abfragen, die
   `library.Sweep` benutzt. **Neu** ist eine wurzelgefilterte Fassung; `SoftDeleteItemWithoutFiles`
   (`queries/system/library.sql:16`) kommt nahe, führt aber nur `movie`, `episode`, `extra` und muss
   `audiobook` mitnehmen, sonst bleiben genau in der Hörbuchbibliothek Nullzeiler zurück.
2. Downloads der **überlebenden** Titel auf ihre verbleibende Datei umhängen (Vorbild
   `AdminMoveDownloads`, `queries/system/admin.sql:258`). Ohne das reisst Schritt 3 sie mit:
   `downloads.media_file_id` hängt mit `ON DELETE CASCADE` an `media_files`, und der Tombstone-Trigger
   (`00013_offline.sql:97`) liesse das Gerät ein Paket löschen, dessen Titel weiterhin sichtbar ist.
3. Die Wurzelzeile löschen. `ON DELETE CASCADE` räumt `media_files` ab, die Trigger setzen
   `items.available`.
4. Watcher dieser Wurzel beenden, `pathPending` und `orphans` dieser `rootID` verwerfen, eingereihte
   `scan.path`-Aufträge mit dieser `rootID` fallenlassen, den Zähler der überwachten Verzeichnisse
   auffrischen.

Die Reihenfolge ist Pflicht, nicht Stil: ohne Schritt 1 vorweg bleiben Titel mit null Dateien übrig, die
`SoftDeleteItemsMissingSince` wegen `AND EXISTS (SELECT 1 FROM media_files …)` nie wieder anfasst. Ein
Test muss genau das festnageln: nach `removeLibraryRoot` gibt es in der Bibliothek **keinen** Titel mit
`deleted_at IS NULL` und null Dateien.

**Was nicht geschieht, und darum nicht versprochen werden darf:** Die weiche Löschung lässt alle 22
Kaskaden auf `items(id)` unberührt. `progress`, `progress_events`, `progress_days`, `watchlist`,
`top_picks`, `collection_items`, `item_texts`, `images`, `credits`, `chapters`, `trailers`,
`notifications` und `playback_sessions` bleiben stehen — unsichtbar, aber vorhanden, genau wie bei einer
abgehängten Platte. Wirklich weg sind nur die Downloads. Und «unwiderruflich» stimmt nur halb:
`relinkWindow = 30 * 24 * time.Hour` (`library/scanner.go:40`) holt einen Titel mit TMDB-, TVDB- oder
IMDb-Id samt Benutzerdaten zurück, sobald eine Datei dazu wieder auftaucht (`library/write.go:415`).
Neu angelegt werden nur Titel ohne Provider-Id.

**Spuren:** Das Entfernen schreibt eine Zeile auf `Warn` (Bibliothek, Wurzel, Zahl der gelöschten Titel
und Dateien) und meldet über `notify`, wie es der Verlustschutz für `root_missing` und
`loss_over_threshold` schon tut. Wer 4000 Titel löscht, soll mindestens so viel Aufmerksamkeit erzeugen
wie ein abgezogenes NAS-Kabel. Eine Audit-Tabelle gibt es im Projekt nicht; ohne diese Zeile wäre der
Vorgang spurlos.

**Laufende Nutzung:** Eine Wiedergabe aus der entfernten Wurzel bricht mitten im Strom ab. Die
Umsetzungstabelle der Radarr- und Sonarr-Integration (`integrations/pathmap.go`) löst gegen alle
registrierten Wurzeln auf; nach dem Entfernen beantwortet der Server die Meldungen zu diesem Pfad mit
`ErrWebhookPathOutside`, und die zugehörigen Paare bleiben als Leichen stehen. Beides gehört in den Text
der Sicherheitsabfrage, zusammen mit der Zahl der betroffenen Titel.

Oberfläche: Die Wurzelliste steht auf `LibraryCard`, «Ordner hinzufügen» öffnet denselben Browser-Dialog.
Das Entfernen braucht eine **neue Variante von `ConfirmDialog` mit Eingabefeld** («Tippe den Pfad, um zu
bestätigen») — abgetippte Bestätigungen gibt es im Projekt heute nirgends. Nach `addLibraryRoot` und
`removeLibraryRoot` muss der Abfrageschlüssel `['fs','browse',…]` entwertet werden, sonst stimmt die
Marke «gehört zu Bibliothek X» nicht mehr.

## 7. Sicherheit

Das Bedrohungsmodell vom 25. Oktober 2026 stuft das Vorhaben **in der ursprünglichen Form (absoluter
Pfad, rekursives Zählen) als hohes Risiko** ein — nicht wegen der Berechtigungsstufe, sondern wegen
zweier Konstruktionsdetails. Beide sind im Entwurf billig geschlossen, nach der Umsetzung teuer. Die
Entscheide E1 bis E3, E5, E6 und E9 sind genau diese Schliessung.

**Was der Container preisgibt:** `/config` mit `slouchcat.db`, den Sicherungen in `/config/backups` und
**`secret.key`** (der Schlüssel, mit dem `keyring.Encrypt` den TMDB-Schlüssel und die SMTP-Daten
verschlüsselt); `/cache` mit den Offline-Paketen anderer Profile; `/media` mit der Ordnerstruktur des
NAS. Die Auflistung liest keine Dateiinhalte — aber sie zeigt, wo sie liegen. Der Dateisystemtyp ist
zusätzlich eine Netzwerkkarte: wer sieht, dass `/media/Filme` auf NFS und `/media/Serien` auf SMB liegt,
kennt den nächsten Schritt nach einem Einbruch in den Container.

**Was die Berechtigungsstufe trägt:** `authorize` prüft nach dem Routing und fällt geschlossen aus — ohne
Sitzung 401, bei `admin` ohne `IsOwner() && IsPrimary()` 403 (`session.go:117`), bei fehlendem oder
unbekanntem Marker ebenfalls 401. Ein Gast und auch ein **Zweitprofil des Hauptkontos** kommen nicht
durch; dieser zweite Fall gehört in den Test, weil man ihn beim Nachbauen am ehesten vergisst.

**Die Eskalation, die E14 schliesst:** `library.ValidateDir` prüft heute nur «absolut, existiert,
Verzeichnis, lesbar» (`service.go:143`) — **keine Wurzelliste**. Wer die Admin-Sitzung hat, legt also
eine Bibliothek auf `/config` an. Weil E4 die Bibliothekswurzeln in die Browse-Wurzelliste
hineinvereinigt, hat er damit `/config` selbst freigeschaltet. **Die Wurzelliste wäre also nicht die
Grenze, sondern vom Angreifer bestimmbar.** Darum verlangt E14, dass auch `createLibrary` und `addLibraryRoot` den Pfad
unter einer browsbaren Wurzel sehen wollen. Bestehende Wurzeln behalten Bestandsschutz, sonst hielte ein
Update eine laufende Installation an.

**Was ausdrücklich nicht getan werden darf:**

- Kein 403 für einen Pfad ausserhalb der Wurzeln (harte Regel 2).
- Kein `missing`/`not_directory`/`not_readable` für Pfade ausserhalb der Wurzeln. **Heute verletzt das
  `CheckSetupPath` selbst:** schlägt `ValidateDir` fehl, ruft es `pathProblem(path)` mit `os.Stat` auf
  dem rohen Pfad (`setup_checks.go:74`). Mit E14 fallen Pfade ausserhalb der Wurzeln dort früher durch,
  landen aber im selben Zweig. Darum braucht `CheckSetupPath` einen eigenen Zweig mit dem Code
  `not_browsable`, vor `pathProblem` und ohne `os.Stat`.
- Keine Wurzel ohne aufgelösten Realpfad. `os.OpenRoot` löst den Wurzelpfad **selbst** samt Symlinks
  auf; nur was darunter liegt, ist eingesperrt. `/media/ablage -> /config` besteht jede lexikalische
  Prüfung. Darum E14 mit `filepath.EvalSymlinks` nach der lexikalischen Prüfung.
- Kein `syscall.Statfs` auf einem aus der Anfrage zusammengesetzten Pfad — dieselbe Klasse wie
  `os.Stat`. Für das aufgelistete Verzeichnis `paths.FSTypeAt` über den Deskriptor des `*os.Root`,
  für Symlink-Zeilen gar nichts.
- Kein absoluter Pfad als Parameter, auch nicht «nur für den ersten Aufruf».
- Keine Pfadprüfung mit `strings.HasPrefix` auf dem Rohpfad: `/mediaX` beginnt mit `/media`, und
  `/media/../config` beginnt es auch. Vergleich entlang der Pfadtrenner auf `filepath.Clean`-Ergebnissen.
- Kein eigener `http.Handler` am generierten strict server vorbei. Harte Regel 6 erlaubt rohe Handler für
  Streaming; ihre zweite Hälfte — alle JSON-Endpunkte über den strict server — gilt hier und wird
  befolgt. Ein Handler daneben sähe `authorize` nie und wäre unauthentifiziert aus dem Internet
  erreichbar.
- Kein `os/exec` für `find`, `du` oder `df` (harte Regel 3, `forbidigo`).
- Kein rekursives Zählen ohne Obergrenze, auch nicht «nur beim ersten Mal, danach gecacht» — der Cache
  wird mit dem ersten Aufruf gefüllt, und der ist der teure.

**Restrisiko nach diesen Massnahmen:**

1. Eine gestohlene Hauptkonto-Sitzung kartiert alles, was in der Wurzelliste steht. Mit E14 ist das die
   konfigurierte Liste plus die Wurzeln, die vor der Umstellung schon bestanden — eine Liste, die der
   Angreifer nicht mehr erweitern kann.
2. Eine tote NFS-Einhängung bindet je Versuch dauerhaft einen OS-Thread; E9 begrenzt es auf 16 und
   nimmt dafür in Kauf, dass der Browser bis zum Neustart 503 antwortet.
3. Ein Betreiber, der `/` in `SLOUCHCAT_BROWSE_ROOTS` schreibt, steht wieder am Anfang. Die Startwarnung
   macht es sichtbar, nicht unmöglich — dieselbe Restschwäche wie bei `SLOUCHCAT_TRUSTED_PROXIES`.

**Prüfbare Zusagen** (jede eine Testbehauptung):

| Zusage | Test |
|---|---|
| `rel` mit `..`, `a/../b`, `/etc/passwd`, NUL ergibt je 422 | `internal/browse` Tabellentest |
| Unbekanntes `rootId` und nicht vorhandenes `rel` ergeben denselben Problemkörper | `internal/api` Handlertest (`instance` ist bei POST ohnehin identisch, `problem.go:52`) |
| Kein Testfall dieses Endpunkts erwartet 403 für einen **Pfad** (403 bleibt `forbidden` und `csrf_header_missing`) | im Test festgeschrieben |
| Nach `<root>/aussen -> /` erscheint `aussen` in der Antwort, aber `rel=aussen/config` ergibt 404, und der Körper enthält nirgends `/config` | `internal/browse` mit `t.TempDir()` |
| `/mediaX` wird von der Wurzel `/media` nicht akzeptiert; vor der Ablehnung findet kein `os.Stat` und kein `syscall.Statfs` statt | Zähler in einer eingeschleusten Dateisystem-Schnittstelle |
| POST ohne `X-Requested-With: Slouchcat` ergibt 403 `csrf_header_missing`; nach erfolgreichem Aufruf enthält die Log-Ausgabe den Pfad nicht | `internal/api` plus Log-Prüfung |
| Ein Verzeichnis mit 50'000 Einträgen liefert genau `limit` Zeilen plus Marke; je Zeile fällt höchstens ein `stat` und ein `statfs` an | `internal/browse` mit Zähler |
| Gegen eine blockierende Dateisystem-Schnittstelle antwortet der Endpunkt binnen 3 s mit 503 `path_unreachable`; nach 16 solchen Aufrufen antwortet er 503 `browse_busy`, ohne eine weitere Goroutine zu starten | `internal/browse` mit fester Uhr |
| Gast, Zweitprofil des Hauptkontos, ohne Sitzung: 403, 403, 401 | `adminRegistry`-Eintrag, `TestContractAdminOperations` |
| `listSetupRoots` zehnmal hintereinander lässt `POST /setup` weiterhin durch | `internal/api` |
| Die 200er-Antwort trägt `Cache-Control: no-store` | `internal/api` |
| Eine Bibliothek auf `<wurzel>/ablage -> /config` wird mit 422 abgelehnt; die Wurzelliste enthält danach kein `/config` | `internal/library` mit `t.TempDir()` |
| `checkSetupPath` auf einen Pfad ausserhalb der Wurzeln antwortet `not_browsable` und macht dabei kein `os.Stat` | `internal/admin` mit Zähler |
| `listSetupRoots` macht im Anfragepfad keinen Systemaufruf | `internal/admin` mit Zähler |
| Nach `removeLibraryRoot` ist der Download eines Titels, der unter einer zweiten Wurzel weiterlebt, noch da | `internal/api` plus `internal/offline` |

## 8. Berührungspunkte mit den harten Regeln

| Regel | Berührung |
|---|---|
| 1 Generierter Code | Quelle ist `api/openapi.yaml`; danach `task gen`, Beleg `task gen:check`. Betroffen sind `server/internal/api/gen/` und `packages/api-client/src/generated/`. Für den Browser (Schritte 1–11) keine Migration und keine neue sqlc-Abfrage; Schritt 12 braucht `DeleteLibraryRoot` und eine wurzelgefilterte Weichlöschung, beide nach `server/queries/system/`, weil sie `items` und `media_files` anfassen (harte Regel 2) |
| 2 Sichtbarkeit | `browseServerPaths` und `listBrowseRoots` sind `admin`, `listSetupRoots` ist `public`. Keine Antwort enthält `ItemRef`, also kein `scoped`. «Nicht sichtbar heisst 404, nie 403» gilt hier für Pfade (E6) |
| 3 `os/exec` | kommt nicht vor; `forbidigo` setzt es ohnehin durch |
| 4 Dateizugriff über `paths` | der Kern des Entwurfs: `rootId` plus `rel`, `paths.Resolve`, Lesen durch `paths.Open` und das `*os.Root`; neu `paths.ReadDir` für eine Ebene und `paths.FSTypeAt` über den Deskriptor. Niemals `os.ReadDir`, `filepath.Walk`, `os.Stat` oder `syscall.Statfs` auf einem Pfad aus der Anfrage. Der Wurzelpfad selbst ist die Ausnahme, die `os.OpenRoot` nicht abdeckt: dort prüft E14 zusätzlich den mit `filepath.EvalSymlinks` aufgelösten Pfad |
| 5 Migrationen | keine |
| 6 Streaming als `http.Handler` | der Ausnahmeteil gilt **nicht**; der Pflichtteil («alle JSON-Endpunkte über den generierten strict server») gilt und wird befolgt |
| 7 `x-visibility` | drei neue Operationen, je ein Eintrag im Vertragstest (`adminRegistry` beziehungsweise `publicRegistry` in `contract_access_test.go`) |
| 8 Tastaturtest | neuer Playwright-Test für den Dialog: Pfeiltasten, Enter steigt ab, Rücktaste hinauf, Escape gibt den Fokus zurück, sichtbarer Ring je Zeile. `PathList.module.css` muss `outline` auf `[data-focused]` setzen (ohne Hex-Werte, `color-no-hex`), weil `expectVisibleFocus` `outlineStyle !== 'none'` **und** `outlineWidth > 0` prüft (`keyboard.ts:25-41`). `expectNoFocusTrap` läuft über das Dokument, nicht im Dialog — ein modaler Dialog **soll** den Fokus halten |
| 9 Keine Farbe, kein Text hart | Namensraum `library.browse.*` mit der Schlüsselliste in 6.6, in `de.json` und `en.json`; Bausteine aus `web/src/ui` |
| 10 Keine Secrets in Logs | Pfade sind keine Secrets, aber Benutzernamen und Struktur. Darum POST für `browse` (E2), damit der Pfad nicht über `requestLog`s Attribut `query` in den Fehlerring des Dashboards, in die Browser-Historie und in den `Referer` gelangt |
| 11 Keine Nicht-Ziele | keine berührt |
| 12 Entscheid im Server | der Client zeigt nur, was er bekommt; die Wurzelliste und jede Pfadentscheidung liegen im Server |

## 9. Beantwortete Fragen

Alle neun Fragen hat Marco am 25. Oktober 2026 beantwortet. Die Antworten sind als Entscheide E12 bis
E15 und in den betroffenen Abschnitten eingearbeitet; diese Liste bleibt als Begründungsspur stehen.

| Nr. | Antwort | Folge im Papier |
|---|---|---|
| **F-1 Wurzelvorgabe** | Auf CT 190 hängt nur der Medienordner. `/media` bleibt die Vorgabe | `SLOUCHCAT_BROWSE_ROOTS` muss im Normalfall nie gesetzt werden; die Variable existiert für abweichende Installationen |
| **F-2 Assistent** | Der Assistent bekommt die erlaubten Ordner als anklickbare Vorschlagsliste, kein Blättern | E5 bleibt wie beschrieben: `listSetupRoots`, parameterlos, ohne Anrechnung auf die Versuchsgrenze |
| **F-3 Zählung** | Gar keine Zahl der Mediendateien, auch nicht später | E3 ist endgültig; die Grenzwerte und das Zeitbudget aus dem Architekturentwurf entfallen ersatzlos |
| **F-4 Symlinks** | Anzeigen mit Vermerk, nicht betretbar | E15 |
| **F-5 Pfad beim Anlegen** | Ja, neue Bibliotheken und neue Wurzeln müssen unter einer browsbaren Wurzel liegen; bestehende behalten Bestandsschutz | E14, eigener Schritt 3. Macht die Wurzelliste zur echten Grenze (Abschnitt 7) |
| **F-6 Wurzel entfernen** | Löschen mit Sicherheitsabfrage und abgetipptem Pfad; die letzte Wurzel darf weg, die Bibliothek bleibt ohne Ordner | E12 und E13, Abschnitt 6.7, Schritte 12 bis 14. **Nachtrag nach der Prüfung:** «Löschen» heisst weiches Löschen der Titel; Fortschritt, Merkliste und Picks bleiben an ihnen hängen, weg sind nur die Downloads, und ein Titel mit Provider-Id kommt binnen 30 Tagen samt Benutzerdaten zurück (`relinkWindow`). Marco hat den Entscheid am 25. Oktober 2026 auf dieser Grundlage **bestätigt**: Die Titel sollen sofort verschwinden statt erst nach 24 Stunden, und die Sicherheitsabfrage bleibt. Der Dialogtext sagt, was bleibt und was geht |
| **F-7 Form** | Idee bleibt Idee, mit Issue auf GitHub (Label `idea`) | Kein Eintrag in `docs/planning/04-entscheidungen.md`; nichts ist beauftragt, bis das Vorhaben bewusst in Spec und Plan übernommen wird |
| **F-8 ADR** | Ja, `docs/adr/0012-browsbare-wurzeln.md` | Schritt 11, fällig erst bei Umsetzung |
| **F-9 Semaphor** | Platz nicht freigeben; sind alle Plätze von hängenden Aufrufen belegt, antwortet der Browser bis zum Neustart 503, das Dashboard zeigt eine Warnung | E9, Dashboard-Warnung in Schritt 7. **Nachtrag nach der Prüfung:** die Obergrenze steht auf 16 statt 4 (Go trägt zehntausend Threads), und ein Platz kommt zurück, falls die Goroutine doch zurückkehrt — ein hart blockierter Aufruf tut das nie, die Garantie bleibt also |

Offen bleibt damit nur, was sich erst beim Bauen zeigt: wie viele Stellen den Zustand «Videobibliothek
ohne Wurzel» annehmen müssen (Schritt 14).

## 10. Aufwand und Abhängigkeiten

Jeder Schritt ein eigener Commit, vor dem Abschluss `task ci` grün. Die genannte Zeile ist der Beleg,
der **diesen** Schritt beweist.

| # | Commit | DoD-Beleg | Hängt an |
|---|---|---|---|
| 1 | `feat(paths): read one level of a root and classify it through its descriptor` — `paths.ReadDir` und `paths.FSTypeAt`, Test mit Symlink aus der Wurzel heraus | `task server:test` (neue Fälle in `internal/paths/paths_test.go`) | — |
| 2 | `feat(config): make the browsable roots configurable` — `SLOUCHCAT_BROWSE_ROOTS` und `--browse-roots`, komma-getrennt, Vorgabe `/media`, fehlende Wurzel protokolliert statt fatal, Warnung bei `/`, Aufnahme in `LogValue` | `task server:test` (`internal/config/config_test.go`) | — |
| 3 | `feat(library): require new library paths below a browsable root` — E14 als `Service.ValidateNewRoot` mit `filepath.EvalSymlinks`, Bestandsschutz als Pfadgleichheit, Durchlass bei leerer Wurzelliste, eigener Zweig `not_browsable` in `CheckSetupPath` samt Text in `de.json` und `en.json` | `task server:ci`, `task web:lint`; Tests: Pfad ausserhalb ergibt 422, `<wurzel>/ablage -> /config` ebenfalls, eine vorher angelegte Wurzel ausserhalb scannt weiter, `checkSetupPath` macht dabei kein `os.Stat` | 2 |
| 4 | `refactor(library): share the path rules with the browser` — `pathProblem` → `library.PathProblem`; `Ignored` und `WatchMode` so stellen, dass `browse` sie ohne den Scanner bekommt; `depguard`-Regel für `browse` | `task server:lint`, `task server:test`; `internal/admin/setup_checks_test.go` bleibt unverändert grün | — |
| 5 | `feat(browse): list directories below the browsable roots` — Paket ohne HTTP, Semaphor mit 16 Plätzen, Wächter, Symlinkzeilen nach E15 | `go test ./internal/browse/...`: 300 Unterordner (Blättern), `@eaDir`, Symlink nach aussen erscheint und ist nicht betretbar, nicht lesbarer Ordner, blockierende Dateisystem-Schnittstelle mit fester Uhr; dazu `GOOS=darwin go build ./...` | 1, 4 |
| 6 | `feat(api): describe the directory browser in the contract` — drei Operationen, Schemata mit eigenen `x-enum-varnames`, `task gen`, drei Vertragseinträge, **Stub-Handler**, damit `gen.StrictServerInterface` erfüllt bleibt | `task gen:check`; `task server:ci` (ohne Stubs wäre der Commit rot, `internal/api/api.go:153`) | — |
| 7 | `feat(admin): browse directories for the library form` — `admin.Browse*`, Stubs gefüllt, `Cache-Control: no-store`, Wurzelklassen beim Start statt im Anfragepfad, eigener Drosseltopf für `listSetupRoots`, Dashboard-Warnung für blockierte Wurzeln | `task server:ci`; neue Fälle: 404 ausserhalb, 403 für Gast und Zweitprofil, 200 mit `problem`, 503 bei Überlast, `listSetupRoots` ohne Syscall und ohne Anrechnung auf `AllowAttempt` | 2, 5, 6 |
| 8 | `feat(web): a directory list and a dialog that can focus it` — `PathList` samt Story und CSS, `initialFocus` an `FormDialog`, Fokusrückgabe ins Textfeld nach «Übernehmen», Pfad im Zustand von `AddLibraryForm` | `task web:lint`, `task web:test`, `task storybook:test` | — |
| 9 | `feat(web): browse the server's folders from the library form` — Dialog, Abfragen, Einbau in `LibrariesPage.tsx`, Vorschlagsliste im Assistenten, Texte de und en | `task web:test`, `task web:lint` | 6, 8 |
| 10 | `test(e2e): keyboard test for the folder browser` | `task web:e2e` | 7, 9; **Tor nach harter Regel 8, ohne diesen Schritt ist die Funktion nicht fertig** |
| 11 | `docs: the folder browser and SLOUCHCAT_BROWSE_ROOTS` — `docs/INSTALLATION.md`, Kommentar in `deploy/compose.yaml`, `docs/dev/CURRENT_STATUS.md`, ADR `docs/adr/0012-browsbare-wurzeln.md` | `task ci` | 2 |
| 12 | `feat(api): add and remove library roots` — Vertrag, `library.AddRoot`/`RemoveRoot`, Scan-Sperre, Löschweg in einer Transaktion samt Umhängen der Downloads, wurzelgefilterte Weichlöschung mit `audiobook`, `confirmPath`, Warn-Log und `notify`-Ereignis | `task server:ci`; Tests: nach `removeLibraryRoot` kein Titel mit `deleted_at IS NULL` und null Dateien; ein Titel mit Datei unter einer zweiten Wurzel bleibt verfügbar **und behält seinen Download**; falscher `confirmPath` ergibt 422; laufender Scan ergibt 409 | 3, 7 |
| 13 | `feat(web): several folders per library` — Wurzelliste auf `LibraryCard`, «Ordner hinzufügen» über denselben Dialog, `ConfirmDialog` mit Eingabefeld samt Folgenliste, Entwertung des Browse-Schlüssels, Texte de und en | `task web:test`, `task web:lint` | 9, 12 |
| 14 | `feat: a video library without a folder` — Scanner, Watcher, Knopf «Scan starten», Dashboard und Oberfläche halten den Zustand aus, eigener Text statt `admin.libraries.noRoots` | `task server:ci`, `task web:e2e` | 12 |

| Teil | Arbeitstage |
|---|---|
| Schritte 1–4 (Pfadpaket, Konfiguration, Wurzelzwang mit Realpfad und eigenem Fehlertext, geteilte Pfadregeln) | 3–4 |
| Schritte 5–7 (Paket `browse`, Vertrag mit Stubs, Handler, Drosseln, Dashboard-Warnung) | 3–4 |
| Schritte 8–10 (neuer Primitivbaustein `PathList`, Dialogarbeit, zwei Seiten, zwei Sprachdateien, Tastaturtest) | 4–5 |
| Schritt 11 und ADR | 1 |
| **Summe Ordner-Browser** | **11–14** |
| Schritte 12–14 (mehrere Wurzeln, Löschweg mit Downloads und Spuren, Bibliothek ohne Ordner) | 4–5 |
| **Summe mit Anschluss** | **15–19** |

Die Schätzung für 8–10 liegt bewusst hoch: `ListBox` wird im Projekt heute nirgends benutzt, es ist also
ein neuer Primitivbaustein samt Story, CSS unter `declaration-strict-value` und `color-no-hex`,
Fokusring und Tastaturtest — dazu die Änderung an `FormDialog` (E11) und die Umstellung des Ordnerfelds
der Admin-Seite auf einen kontrollierten Wert.

Die Schritte 12 bis 14 sind eine eigene Fähigkeit (E8), hängen aber am Browser: Schritt 12 braucht den
Wurzelzwang aus Schritt 3 und die Handler aus 7, Schritt 13 den Dialog aus 9. Ohne den Browser liesse
sich nur ein abgespecktes «Ordner hinzufügen» mit Textfeld bauen. Schritt 14 steckt die verbliebene
Unsicherheit: «Videobibliothek ohne Wurzel» ist aus Sicherheitssicht unauffällig — Scanner, Watcher,
Verlustschutz, Webhook-Abbildung und Offline tragen null Wurzeln bereits —, aber es ist Oberflächen- und
Textarbeit an mehreren Stellen, und ein Scan meldet dort `done` mit null Dateien und sieht damit wie ein
erfolgreicher Lauf aus.

Abhängigkeiten: `internal/paths` (M1a), `library.ValidateDir` und `library.WatchMode` (M1a/M8),
`FormDialog` (M2), `react-aria-components` (vorhanden, `ListBox` ungenutzt; vor Schritt 8 ein
`pnpm audit`, weil das Paket erstmals in diesen Bereich gezogen wird). Bei Übernahme fällig: der ADR aus
F-8 und ein Eintrag im Entscheidungsprotokoll — beides erst, wenn das Vorhaben bewusst in Spec und Plan
übernommen wird (F-7).
