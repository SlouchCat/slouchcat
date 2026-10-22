#!/usr/bin/env python3
"""Zeichnet die Webhook-Nutzlasten von Radarr und Sonarr auf (Slouchcat I-8, Plan 5.6).

Der Empfänger nimmt jede POST-Anfrage mit 200 an und legt den Rumpf als Datei ab, benannt nach
Dienst, Version und Ereignis: <dienst>-<version>-<ereignis>.json. Mehr macht er nicht; er prüft
kein Token und gehört nie in den Betrieb.

    python3 record.py

Dienst und Version kommen aus dem User-Agent der Meldung (Radarr und Sonarr senden dort
«Radarr/5.14.0.9383 (linux x86_64)»), es ist also nichts nachzuschlagen. Nur wenn ein Dienst
keinen brauchbaren User-Agent schickt, helfen --service und --version nach.

Danach in Radarr/Sonarr unter «Einstellungen → Verbindungen → + → Webhook» die Adresse
http://<diese-maschine>:9099/ eintragen, Methode POST, und die Ereignisse ankreuzen. «Test»
schreibt sofort eine Datei; Import, Umbenennung und Löschung entstehen beim nächsten echten
Vorgang.
"""

import argparse
import json
import pathlib
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MAX_BODY = 1 << 20  # 1 MiB; eine Arr-Nutzlast ist ein paar Kilobyte gross.

# "Radarr/5.14.0.9383 (linux x86_64)" → Dienst und Version.
AGENT = re.compile(r"\b(radarr|sonarr)/([0-9][0-9.]*)", re.IGNORECASE)


class Recorder(BaseHTTPRequestHandler):
    service = None  # Rückfall, wenn der User-Agent nichts hergibt
    version = None
    out = pathlib.Path(".")

    def do_POST(self):  # noqa: N802 (Name von BaseHTTPRequestHandler vorgegeben)
        length = min(int(self.headers.get("Content-Length") or 0), MAX_BODY)
        raw = self.rfile.read(length)
        try:
            payload = json.loads(raw)
            event = str(payload.get("eventType", "unknown"))
        except (ValueError, AttributeError):
            payload, event = None, "unparsable"
        service, version = self.identify()
        name = f"{service}-{version}-{slug(event)}.json"
        path = self.out / name
        if payload is None:
            path = path.with_suffix(".raw")
            path.write_bytes(raw)
        else:
            path.write_text(json.dumps(payload, indent=2, ensure_ascii=False, sort_keys=True) + "\n")
        print(f"{event} → {path}")
        self.send_response(200)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def identify(self):
        """Dienst und Version aus dem User-Agent; die Flaggen sind nur der Rückfall."""
        match = AGENT.search(self.headers.get("User-Agent") or "")
        if match:
            return match.group(1).lower(), match.group(2)
        if self.service and self.version:
            return self.service, self.version
        print(
            f"  (kein Radarr/Sonarr im User-Agent {self.headers.get('User-Agent')!r};"
            " mit --service und --version nachhelfen)"
        )
        return self.service or "unknown", self.version or "unknown"

    def log_message(self, fmt, *args):
        pass  # Die Zeile oben sagt schon alles.


def slug(s):
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-") or "unknown"


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--service", choices=["radarr", "sonarr"], help="nur nötig ohne brauchbaren User-Agent")
    p.add_argument("--version", help="nur nötig ohne brauchbaren User-Agent, z. B. 5.14.0.9383")
    p.add_argument("--out", default=".", help="Zielordner für die JSON-Dateien")
    p.add_argument("--port", type=int, default=9099)
    args = p.parse_args()

    Recorder.service, Recorder.version = args.service, args.version
    Recorder.out = pathlib.Path(args.out)
    Recorder.out.mkdir(parents=True, exist_ok=True)

    server = ThreadingHTTPServer(("0.0.0.0", args.port), Recorder)  # noqa: S104 (nur lokal, von Hand)
    print(f"Empfänger läuft auf http://0.0.0.0:{args.port}/ — mit Strg-C beenden.")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print()


if __name__ == "__main__":
    main()
