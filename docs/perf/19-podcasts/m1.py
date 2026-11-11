#!/usr/bin/env python3
"""Measurement M-1 of concept 19 part A (docs/planning/19-podcasts-und-radio.md, section 8).

Read-only: opens the Slouchcat database with mode=ro and lists the podcast folder on the NAS. It
never writes a file. Run on the CT 190 host, fed over stdin:

    ssh root@192.168.1.43 'python3 - /opt/slouchcat/deploy/config/slouchcat.db \
        /srv/media/nas/Audiobookshelf/Podcast' < docs/perf/19-podcasts/m1.py

Prints JSON with counts only (no feed address, no enclosure URL; hosts only).
"""
import json
import os
import re
import sqlite3
import statistics
import sys
import unicodedata
from collections import Counter, defaultdict
from urllib.parse import urlsplit

DB, ROOT = sys.argv[1], sys.argv[2]
AUDIO = {"m4b", "m4a", "mp3", "flac", "ogg", "opus"}
# Audiobookshelf globals.SupportedAudioTypes (server/utils/globals.js), for the extension rule.
ABS_AUDIO = {"m4b", "mp3", "m4a", "flac", "opus", "ogg", "oga", "mp4", "aac", "wma", "aiff", "aif", "wav",
             "webm", "webma", "mka", "awb", "caf", "mpeg", "mpg"}
UUID_SUFFIX = re.compile(r" \([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\)$")
DAY_MS = 24 * 3600 * 1000


def key(s):
    """Comparison key of section 8: NFC, lower case, letters and digits only."""
    s = unicodedata.normalize("NFC", s).lower()
    return "".join(ch for ch in s if ch.isalnum())


def abs_sanitize(filename):
    """Audiobookshelf sanitizeFilename (server/utils/fileUtils.js), colonReplacement ' - '."""
    s = unicodedata.normalize("NFC", filename)
    s = s.replace(":", " - ", 1)
    s = re.sub(r'[/?<>\\:*|"]', "", s)
    s = re.sub(r"[\x00-\x1f\x80-\x9f]", "", s)
    s = re.sub(r"^\.+$", "", s)
    s = re.sub(r"[\n\r]", "", s)
    s = re.sub(r"^(con|prn|aux|nul|com[0-9]|lpt[0-9])(\..*)?$", "", s, flags=re.I)
    s = re.sub(r"[. ]+$", "", s)
    s = re.sub(r"\s+", " ", s)
    stem, ext = os.path.splitext(s)
    if len((stem + ext).encode("utf-16-le")) > 255:
        budget = 255 - len(ext.encode("utf-16-le"))
        out, used = "", 0
        for ch in stem:
            used += len(ch.encode("utf-16-le"))
            if used > budget:
                break
            out += ch
        s = out.strip() + ext
    return s


def slouchcat_name(title):
    """Planned rule of section 6.3 (stem only)."""
    s = re.sub(r'[\x00-\x1f\x7f/\\:*?"<>|]', " ", title)
    s = re.sub(r" +", " ", s).strip()
    s = re.sub(r"[. ]+$", "", s)
    b = s.encode("utf-8")
    if len(b) > 180:
        s = b[:180].decode("utf-8", "ignore")
    return s


def abs_ext(url):
    path = urlsplit(url or "").path
    ext = os.path.splitext(path)[1][1:].lower()
    return ext if ext in ABS_AUDIO else "mp3"


con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
con.row_factory = sqlite3.Row
podcasts = {r["feed_url"]: r for r in con.execute(
    "SELECT id, title, feed_url, feed_origin FROM items WHERE kind = 'podcast' AND deleted_at IS NULL"
    " AND feed_url IS NOT NULL")}
episodes = defaultdict(list)
for r in con.execute(
        "SELECT id, parent_id, title, guid, enclosure_url, enclosure_length, published_at, duration_ms"
        " FROM items WHERE kind = 'podcast_episode' AND deleted_at IS NULL AND guid IS NOT NULL"):
    episodes[r["parent_id"]].append(r)
with_file = {r[0] for r in con.execute(
    "SELECT DISTINCT i.id FROM items i JOIN media_files m ON m.item_id = i.id WHERE i.kind = 'podcast_episode'")}
roots = [dict(r) for r in con.execute("SELECT id, path, content FROM library_roots")]

res = Counter()
hosts = Counter()
hosts_per_podcast = []
mtime_delta_h = []
abs_vs_slouchcat_diff = 0
title_dupes = 0
folders_without_feed = 0
folders_unknown_feed = 0
per_podcast = []
exts = Counter()

for name in sorted(os.listdir(ROOT)):
    folder = os.path.join(ROOT, name)
    if name.startswith(".") or not os.path.isdir(folder):
        continue
    res["folders"] += 1
    feed = None
    meta = os.path.join(folder, "metadata.json")
    if os.path.isfile(meta):
        try:
            with open(meta, encoding="utf-8") as fh:
                data = json.load(fh) or {}
            feed = data.get("feedURL") or data.get("feedUrl")
        except Exception:
            feed = None
    files = []
    for fn in os.listdir(folder):
        if fn.startswith("."):
            continue
        stem, ext = os.path.splitext(fn)
        ext = ext[1:].lower()
        if ext not in AUDIO:
            continue
        st = os.stat(os.path.join(folder, fn))
        files.append({"name": fn, "stem": stem, "key": key(UUID_SUFFIX.sub("", stem)), "mtime": st.st_mtime_ns // 1_000_000,
                      "size": st.st_size, "used": False})
        exts[ext] += 1
    res["files"] += len(files)
    if not feed:
        folders_without_feed += 1
        res["files_in_folders_without_feed"] += len(files)
        continue
    pod = podcasts.get(feed)
    if pod is None:
        folders_unknown_feed += 1
        res["files_in_folders_unknown_feed"] += len(files)
        continue
    eps = episodes.get(pod["id"], [])
    by_key = defaultdict(list)
    by_name = {}
    for f in files:
        by_key[f["key"]].append(f)
        by_name[f["name"]] = f
        by_name.setdefault(UUID_SUFFIX.sub("", f["stem"]) + os.path.splitext(f["name"])[1], f)
    titles = Counter(key(e["title"] or "") for e in eps)
    title_dupes += sum(n - 1 for n in titles.values() if n > 1)
    ph = Counter()
    row = Counter()
    for e in eps:
        res["feed_episodes"] += 1
        row["feed"] += 1
        h = (urlsplit(e["enclosure_url"] or "").hostname or "").lower()
        if h:
            hosts[h] += 1
            ph[h] += 1
        if e["id"] in with_file:
            res["feed_episodes_with_file_row"] += 1
        title = (e["title"] or "").strip()
        abs_name = abs_sanitize(f"{title}.{abs_ext(e['enclosure_url'])}")
        if os.path.splitext(abs_name)[0] != slouchcat_name(title):
            abs_vs_slouchcat_diff += 1
        k = key(title)
        cands = by_key.get(k, []) if k else []
        abs_hit = abs_name in by_name
        if abs_hit:
            res["pair_abs_name"] += 1
        if len(cands) == 1:
            res["pair_key_one"] += 1
            row["key_one"] += 1
            cands[0]["used"] = True
            if e["published_at"]:
                mtime_delta_h.append((cands[0]["mtime"] - e["published_at"]) / 3_600_000)
            if not abs_hit:
                res["pair_key_not_abs"] += 1
        elif len(cands) > 1:
            res["pair_key_many"] += 1
            row["key_many"] += 1
            for c in cands:
                c["used"] = True
        else:
            if abs_hit:
                res["pair_abs_not_key"] += 1
            near = e["published_at"] and any(
                abs(f["mtime"] - e["published_at"]) <= DAY_MS for f in files if not f["used"])
            if near:
                res["no_key_but_file_within_24h"] += 1
                row["near"] += 1
            else:
                res["no_candidate"] += 1
                row["none"] += 1
                if e["enclosure_length"]:
                    res["no_candidate_bytes_known"] += e["enclosure_length"]
                    res["no_candidate_with_length"] += 1
    res["files_without_feed_episode"] += sum(1 for f in files if not f["used"])
    hosts_per_podcast.append(len(ph))
    per_podcast.append({"feed": row["feed"], "files": len(files), "key_one": row["key_one"],
                        "key_many": row["key_many"], "near": row["near"], "none": row["none"],
                        "hosts": len(ph), "title": pod["title"]})

res["podcasts_db"] = len(podcasts)
res["podcasts_matched_to_folder"] = len(per_podcast)

out = {
    "roots": [{"id": r["id"], "content": r["content"]} for r in roots],
    "counts": dict(res),
    "folders_without_feed": folders_without_feed,
    "folders_feed_not_subscribed": folders_unknown_feed,
    "extensions": dict(exts),
    "title_duplicates_within_podcast": title_dupes,
    "abs_name_differs_from_planned_name": abs_vs_slouchcat_diff,
    "hosts_distinct": len(hosts),
    "hosts_top": hosts.most_common(15),
    "hosts_per_podcast": {"max": max(hosts_per_podcast or [0]), "median": statistics.median(hosts_per_podcast or [0]),
                          "one": sum(1 for n in hosts_per_podcast if n == 1)},
    "mtime_minus_published_hours": {
        "n": len(mtime_delta_h),
        "median": statistics.median(mtime_delta_h) if mtime_delta_h else None,
        "within_24h": sum(1 for d in mtime_delta_h if abs(d) <= 24),
        "p10": statistics.quantiles(mtime_delta_h, n=10)[0] if len(mtime_delta_h) > 10 else None,
        "p90": statistics.quantiles(mtime_delta_h, n=10)[-1] if len(mtime_delta_h) > 10 else None,
    },
    "per_podcast": sorted(per_podcast, key=lambda p: -p["feed"]),
}
print(json.dumps(out, ensure_ascii=False, indent=1))
