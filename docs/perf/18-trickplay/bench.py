#!/usr/bin/env python3
# Concept 18 (trickplay), section 3: measure on a local copy of a film or an episode; write OUTDIR outside the repository.
"""Trickplay extraction benchmark: wall time, CPU time and bytes read per variant.

usage: bench.py FILE VARIANT OUTDIR [--interval 10] [--width 320] [--par N] [--hw cuda|vaapi]
variants: full, key, fullhw, keyhw, seek, seekacc, batch
"""
import argparse, re, json, os, subprocess, sys, time, shutil
from concurrent.futures import ThreadPoolExecutor


def run(cmd):
    """Run cmd, return (wall, cpu, rchar, rc)."""
    t0 = time.monotonic()
    p = subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
    # wait without reaping so /proc/<pid>/io is still readable
    os.waitid(os.P_PID, p.pid, os.WEXITED | os.WNOWAIT)
    _, status, ru = os.wait4(p.pid, 0)
    p.returncode = os.waitstatus_to_exitcode(status)
    err = p.stderr.read().decode(errors="replace")
    wall = time.monotonic() - t0
    rchar = sum(int(m) for m in re.findall(r"Statistics: (\d+) bytes read", err))
    if p.returncode != 0:
        sys.stderr.write("\n".join(l for l in err.splitlines() if "rror" in l)[-2000:])
    return wall, ru.ru_utime + ru.ru_stime, rchar, p.returncode


def duration(f):
    out = subprocess.check_output(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", f])
    return float(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("file"); ap.add_argument("variant"); ap.add_argument("outdir")
    ap.add_argument("--interval", type=float, default=10); ap.add_argument("--width", type=int, default=320)
    ap.add_argument("--par", type=int, default=1); ap.add_argument("--hw", default="cuda")
    ap.add_argument("--batch", type=int, default=10); ap.add_argument("--threads", default="0")
    ap.add_argument("--limit", type=int, default=0, help="only the first N points (seek variants)")
    a = ap.parse_args()
    shutil.rmtree(a.outdir, ignore_errors=True); os.makedirs(a.outdir)
    d = duration(a.file)
    n = int(d // a.interval)
    scale = f"scale={a.width}:-2:flags=bilinear"
    base = ["ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "verbose", "-threads", a.threads]
    jpg = ["-q:v", "4"]
    seekbase = ["ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "verbose", "-threads", "1"]
    v = a.variant
    if v in ("full", "key"):
        cmd = base + (["-skip_frame", "nokey"] if v == "key" else []) + ["-i", a.file, "-map", "0:v:0", "-an", "-sn",
              "-vf", f"fps=1/{a.interval}:round=down,{scale}"] + jpg + [f"{a.outdir}/%05d.jpg"]
        wall, cpu, rchar, rc = run(cmd)
    elif v in ("fullhw", "keyhw"):
        if a.hw == "cuda":
            hw = ["-hwaccel", "cuda"]
            vf = f"fps=1/{a.interval}:round=down,{scale}"
        else:
            hw = ["-hwaccel", "vaapi", "-hwaccel_device", "/dev/dri/renderD128", "-hwaccel_output_format", "vaapi"]
            vf = f"fps=1/{a.interval}:round=down,scale_vaapi=w={a.width}:h=-2,hwdownload,format=nv12"
        cmd = base + hw + (["-skip_frame", "nokey"] if v == "keyhw" else []) + ["-i", a.file, "-map", "0:v:0", "-an", "-sn",
              "-vf", vf] + jpg + [f"{a.outdir}/%05d.jpg"]
        wall, cpu, rchar, rc = run(cmd)
    elif v in ("seek", "seekacc", "batch"):
        pts = [i * a.interval for i in range(n)]
        if a.limit:
            pts = pts[: a.limit]
        def one(i, t):
            acc = [] if v == "seekacc" else ["-noaccurate_seek"]
            return run(seekbase + acc + ["-ss", f"{t:.3f}", "-i", a.file, "-map", "0:v:0", "-frames:v", "1", "-fps_mode", "passthrough",
                                     "-vf", scale] + jpg + [f"{a.outdir}/{i:05d}.jpg"])
        def group(chunk):
            cmd = list(seekbase)
            for _, t in chunk:
                cmd += ["-threads", "1", "-noaccurate_seek", "-ss", f"{t:.3f}", "-i", a.file]
            for k, (i, _) in enumerate(chunk):
                cmd += ["-map", f"{k}:v:0", "-frames:v", "1", "-fps_mode", "passthrough", "-vf", scale] + jpg + [f"{a.outdir}/{i:05d}.jpg"]
            return run(cmd)
        t0 = time.monotonic()
        items = list(enumerate(pts))
        with ThreadPoolExecutor(a.par) as ex:
            if v == "batch":
                chunks = [items[k:k + a.batch] for k in range(0, len(items), a.batch)]
                res = list(ex.map(group, chunks))
            else:
                res = list(ex.map(lambda it: one(*it), items))
        wall = time.monotonic() - t0
        cpu = sum(r[1] for r in res); rchar = sum(r[2] for r in res); rc = max(r[3] for r in res)
        n = len(pts)
    else:
        sys.exit("unknown variant")
    imgs = len(os.listdir(a.outdir))
    print(json.dumps({"file": os.path.basename(a.file), "variant": v, "par": a.par, "hw": a.hw if "hw" in v else "",
                      "points": n, "images": imgs, "wall_s": round(wall, 2), "cpu_s": round(cpu, 2),
                      "read_mb": round(rchar / 1e6, 1), "rc": rc, "duration_s": round(d)}))


if __name__ == "__main__":
    main()
