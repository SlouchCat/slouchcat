#!/usr/bin/env python3
"""Zeichnet die Webhook-Nutzlasten von Radarr und Sonarr auf (Slouchcat I-8, Plan 5.6).

Der Empfänger nimmt jede POST-Anfrage mit 200 an und legt den Rumpf als Datei ab, benannt nach
Dienst, Version und Ereignis: <dienst>-<version>-<ereignis>.json. Mehr macht er nicht; er prüft
kein Token und gehört nie in den Betrieb.

    python3 record.py --service radarr --version 5.14.0.9383 --out .
    python3 record.py --service sonarr --version 4.0.10.2544 --out .

Danach in Radarr/Sonarr unter «Einstellungen → Verbindungen → + → Webhook» die Adresse
http://<diese-maschine>:9099/ eintragen, Methode POST, und die Ereignisse ankreuzen. «Test»
schreibt sofort eine Datei; Import, Umbenennung und Löschung entstehen beim nächsten echten
Vorgang.

Die Versionsnummer steht in Radarr/Sonarr unter «System → Status».
"""

import argparse
import json
import pathlib
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MAX_BODY = 1 << 20  # 1 MiB; eine Arr-Nutzlast ist ein paar Kilobyte gross.


class Recorder(BaseHTTPRequestHandler):
    service = "unknown"
    version = "unknown"
    out = pathlib.Path(".")

    def do_POST(self):  # noqa: N802 (Name von BaseHTTPRequestHandler vorgegeben)
        length = min(int(self.headers.get("Content-Length") or 0), MAX_BODY)
        raw = self.rfile.read(length)
        try:
            payload = json.loads(raw)
            event = str(payload.get("eventType", "unknown"))
        except (ValueError, AttributeError):
            payload, event = None, "unparsable"
        name = f"{self.service}-{self.version}-{slug(event)}.json"
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

    def log_message(self, fmt, *args):
        pass  # Die Zeile oben sagt schon alles.


def slug(s):
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-") or "unknown"


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--service", required=True, choices=["radarr", "sonarr"])
    p.add_argument("--version", required=True, help="Version aus «System → Status», z. B. 5.14.0.9383")
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
