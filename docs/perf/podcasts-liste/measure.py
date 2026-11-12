#!/usr/bin/env python3
"""Time the queries behind GET /admin/podcasts (docs/perf/podcasts-liste.md).

Two uses, standard library only:

  # On CT 190, read only: the queries travel on standard input, nothing is written there.
  python3 docs/perf/podcasts-liste/measure.py emit | ssh root@192.168.1.43 python3 - /opt/slouchcat/deploy/config/slouchcat.db

  # On a local copy of that database, before and after migration 00052.
  python3 docs/perf/podcasts-liste/measure.py local /path/to/copy/slouchcat.db
  python3 docs/perf/podcasts-liste/measure.py local /path/to/copy/slouchcat.db --apply-00052

The queries are read from server/queries/system/*.sql (sqlc.narg becomes NULL, as the list calls them).
"""
import json
import pathlib
import re
import sys

NAMES = [
    "PodcastList",
    "PodcastEffectiveSettings",
    "PodcastDownloadStats",
    "PodcastNewestRoots",
    "PodcastAllEpisodesWithoutFile",
    "PodcastAllFolders",
    "PodcastAllFolderFiles",
    "PodcastAllFileEpisodesWithoutGUID",
]
REPO = pathlib.Path(__file__).resolve().parents[3]

RUNNER = r'''
import sqlite3, sys, time
def run(c, queries):
    total = 0.0
    for name, q in queries.items():
        start = time.time()
        rows = c.execute(q).fetchall()
        took = time.time() - start
        total += took
        bad = [r[3] for r in c.execute("EXPLAIN QUERY PLAN " + q) if "media_files_missing" in r[3]]
        print("%-36s rows=%6d %8.3f s%s" % (name, len(rows), took, "  " + "; ".join(bad) if bad else ""))
    print("%-36s             %8.3f s" % ("total", total))
'''


def queries():
    out = {}
    for f in sorted((REPO / "server/queries/system").glob("*.sql")):
        text = f.read_text()
        for m in re.finditer(r"-- name: (\w+) [^\n]*\n(.*?)(?=\n-- name: |\Z)", text, re.S):
            if m.group(1) in NAMES:
                body = "\n".join(line for line in m.group(2).split("\n") if not line.startswith("--"))
                out[m.group(1)] = re.sub(r"sqlc\.narg\('\w+'\)", "NULL", body.strip().rstrip(";"))
    return {n: out[n] for n in NAMES}


def main():
    if len(sys.argv) >= 2 and sys.argv[1] == "emit":
        print(RUNNER)
        print("Q = " + json.dumps(queries()))
        print("c = sqlite3.connect('file:' + sys.argv[1] + '?mode=ro', uri=True)")
        print("run(c, Q)")
        return
    if len(sys.argv) >= 3 and sys.argv[1] == "local":
        import sqlite3
        ns = {}
        exec(RUNNER, ns)
        c = sqlite3.connect(sys.argv[2])
        if "--apply-00052" in sys.argv:
            c.isolation_level = None
            c.executescript((REPO / "server/migrations/00052_media_files_missing_partial.sql").read_text())
        print(c.execute("SELECT sql FROM sqlite_master WHERE name = 'media_files_missing'").fetchone()[0])
        ns["run"](c, queries())
        return
    sys.exit(__doc__)


if __name__ == "__main__":
    main()
