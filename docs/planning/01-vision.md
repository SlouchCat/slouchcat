# Vision

Stand: 18. Oktober 2026, Ergebnis des Deep Interviews (19 Runden, Restmehrdeutigkeit 10 %).
Die vollständige Spezifikation mit allen Abnahmekriterien liegt unter
`docs/spec/01-anforderungen-v1.md` (Arbeitskopie für OMC: `.omc/specs/deep-interview-slouchcat-vision.md`)
und wird in der Planungsphase in `docs/spec/` überführt.

## Was Slouchcat ist

Ein selbst gehosteter, vollständig eigenständiger Media-Server für einen Haushalt und dessen eingeladene
Gäste. Nutzer erleben ihn wie einen persönlichen Streamingdienst: Netflix-artige Profile mit Altersfreigabe
und PIN, eine kuratierte Startseite mit „Cat's Empfehlungen“ des Hauptbenutzers, verlässliche Wiedergabe mit
Transcoding und echte Offline-Wiedergabe. Server und Produkt sind gleichwertig: Die Admin-Oberfläche
bekommt dieselbe Design-Sorgfalt wie die Zuschauerseite, und die API ist von Anfang an für Fremd-Clients
dokumentiert.

## Warum eigenständig und nicht auf Jellyfin aufgesetzt

Entscheidung vom 18. Oktober 2026: komplette eigene Lösung, kein Client auf einem Fremdserver. Volle
Kontrolle über Datenmodell, Wiedergabe und Produktentwicklung; dafür bewusst in Kauf genommen, dass
Scanner, Metadaten, Transcoding und Streaming selbst gebaut werden. Alle wichtigen Funktionen von Jellyfin
werden übernommen.

## Die neun Bausteine von Version 1

1. **Konten, Profile und Zugriff.** Hauptbenutzer mit E-Mail-Konto, Profile nur mit Namen, Altersstufen
   (FSK 0 bis 18), PIN, Bibliotheksrechte, Schauzeit-Limits mit Bericht, Gäste per Einladungslink,
   Zugriff von unterwegs.
2. **Video-Bibliothek und Metadaten.** Filme und Serien aus Ordnern, Plex/Jellyfin-Benennung und NFO,
   TMDB und TVDB in Deutsch und Englisch, Änderungen in unter einer Minute sichtbar, Versionen als ein
   Titel, manuelle Korrektur, Extras, automatische YouTube-Trailer mit wählbarer Sprache, Pflegewerkzeuge.
3. **Hörbücher und Podcasts.** Kapitel, Feeds, Fortschritt je Profil, Geschwindigkeit.
4. **Wiedergabe und Transcoding.** Direct Play zuerst, sonst GPU-Transcoding zu HLS mit Tonemapping;
   4K-HEVC startet im Handy-Browser in unter fünf Sekunden; Spur- und Untertitelwechsel; Qualitätsregeln
   je Profil und Gast; Intro überspringen, nächste Folge, Kapitel; Geräteübersicht mit Codec-Fähigkeiten.
5. **Entdecken und Kuratierung.** Karussell „Cat's Empfehlungen“ oben, darunter Weiterschauen, Neu,
   Merkliste, Sammlungen und Genres sowie „Trending bei Trakt“ und „Beliebt bei Trakt“ mit Titeln aus der
   eigenen Bibliothek; Suche mit Filtern; Serien als Favorit mit Benachrichtigung bei neuer Folge.
   Übersichtlich: höchstens acht Reihen, keine leeren. Keine Algorithmus-Empfehlungen.
6. **Offline-Wiedergabe.** Einzeltitel, ganze Staffeln, Auto-Downloads, Flugmodus-Wiedergabe,
   Fortschritt-Sync, kein Ablaufdatum, Profilregeln gelten offline.
7. **Clients und Design.** Eine Web-App als PWA für Desktop, Handy und Tablet mit eigenem Design-System;
   Deutsch und Englisch; Tastatur- und Fernbedienungs-Navigation; OpenAPI-Dokument.
8. **Server-Administration.** Einrichtungsassistent, zentrale Verwaltung, Dashboard, Sicherung und
   Wiederherstellung, Update-Hinweis, Benachrichtigungskanäle (E-Mail, Push, Telegram), Statistiken,
   Ankündigungen an die Nutzer.
9. **Integrationen.** Radarr und Sonarr lesend (Beschaffungsstatus am Titel, Benachrichtigung bei
   Verfügbarkeit), eigener Wunsch-Bereich über Seerr, Untertitel über Bazarr, SABnzbd optional, Trakt nur
   mit öffentlichen Listen.

## Grenzen

- Betrieb nur als Docker-Compose-Stack auf Linux, GPU optional.
- Ein Server pro Haushalt, kein Mandantenmodell.
- Genau ein Client: die PWA. Offline-Speicher liegt im Browser, seine Grenzen (Quota, iOS) sind früh zu
  prüfen.
- YouTube-Trailer werden eingebettet, nie heruntergeladen.
- Solo-Entwicklung mit Claude Code und oh-my-claudecode, keine feste Deadline. Erstes Etappenziel ist der
  vertikale Durchstich: Bibliothek scannen, Film abspielen, Startseite sehen.
- Design-Richtung: dunkel und kinoartig, grosse Bildflächen, ruhige Typografie, weiche Bewegung, eine
  Akzentfarbe je Profil, Posterfarben färben den Hintergrund. Sehr modern, schön und vor allem ordentlich
  und übersichtlich.
- Sprache, Framework und Datenbank entscheidet der Plan nach drei Kriterien: sehr stabil, läuft überall
  gut, performant. Die Begründung muss ohne Entwicklerwissen verständlich sein.
- Offline auf iOS: PWA zuerst mit ehrlichem Hinweis auf die Apple-Grenzen; eine native iOS-App wird nur
  bei Bedarf nach Version 1 eingeplant.

## Nicht-Ziele von Version 1

Musik, Fotos, Live-TV und DVR. TV-Apps und native Mobile-Apps. Plugin-System. Algorithmische
Empfehlungen, Interaktion zwischen Profilen sowie Assistent, Zufallsauswahl, Trailer-Reel und automatische
Themenlisten (Kandidaten für 1.x). Trakt-Konten je Profil und Scrobbling. Freigabe-Anfragen von Kinderprofilen. Eigene
Wunschverwaltung und Steuerung von Radarr, Sonarr oder SABnzbd. Eigene Untertitel-Beschaffung.
Cloud-Betrieb und native Installationspakete.

## Offene Punkte für die Planung

1. Seerr-Bereich über die API mit eigener Oberfläche (Empfehlung) oder als Einbettung.
2. Ankündigungen über dieselben Kanäle wie die Benachrichtigungen (Empfehlung) oder nur In-App und E-Mail.
3. Trailer-Suche über die YouTube-API (Empfehlung) oder über eine KI-API als austauschbaren Anbieter.
