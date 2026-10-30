#!/usr/bin/env python3
# Seeds finished runs into the jobs table for the measurement of docs/perf/hintergrundjobs.md
# (concept 09, 12.4): the mix of CT 190 after the import, finished_at spread uniformly over the
# last 90 days, 2 % failed, 1 % cancelled, 5 % skipped of the scans and sweeps. Times are Unix
# milliseconds. The server may run meanwhile (WAL). Never run it against a production database.
#
#   python3 -I docs/perf/seed-jobs.py /path/to/slouchcat.db [rows]     (default 50 000)
import random, sqlite3, sys, time
db, n = sys.argv[1], int(sys.argv[2]) if len(sys.argv) > 2 else 50_000
random.seed(9)
mix = [("probe.keyframes", 25_000, 1, 40_000, 400_000), ("image.fetch", 10_000, 3, 300, 4_000),
       ("segments.detect", 8_000, 2, 20_000, 300_000), ("metadata.match", 4_000, 5, 400, 6_000),
       ("metadata.fill", 2_000, 4, 300, 3_000), ("trailer.find", 400, 3, 800, 9_000),
       ("scan.library", 300, 5, 1_000, 2_400_000), ("library.sweep", 300, 1, 2, 900)]
scale = n / sum(m[1] for m in mix)
now = int(time.time() * 1000)
day = 86_400_000
rows = []
for typ, count, prio, dmin, dmax in mix:
    for _ in range(round(count * scale)):
        finished = now - random.randint(60_000, 90 * day)
        dur = random.randint(dmin, dmax)
        started = finished - dur
        r = random.random()
        state = "failed" if r < 0.02 else "cancelled" if r < 0.03 else "done"
        skipped = 1 if typ in ("scan.library", "library.sweep") and random.random() < 0.05 else 0
        payload = '{"libraryId":2}' if typ == "scan.library" else '{}'
        rows.append((typ, prio, payload, state, started - 5_000, 1, 5,
                     "seeded failure" if state == "failed" else None,
                     started - 10_000, finished, started, finished, skipped))
con = sqlite3.connect(db, timeout=30)
con.execute("PRAGMA busy_timeout = 30000")
with con:
    con.executemany("""INSERT INTO jobs (type, priority, payload, state, run_after, attempts, max_attempts,
        last_error, created_at, updated_at, started_at, finished_at, skipped)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""", rows)
print("inserted", len(rows), "total", con.execute("SELECT count(*) FROM jobs").fetchone()[0])
