#!/usr/bin/env python3
"""HDR10 test chart and P-6 colour comparison (plan 8, P-6). Standard library only; run with
`python3 -I`.

    python3 -I hdr-quality.py chart OUT.ppm
        Write the HDR10 chart (3840x2160, 16-bit PPM holding PQ-encoded BT.2020 R'G'B') that
        generate-media.sh encodes into "Slouchcat Test HDR10 Chart (2024)". Geometry and values
        are in WEDGE, SKIN and README.md.

    python3 -I hdr-quality.py report --backend B --test DIR [--ref DIR] [--ssim V ...]
        Print the P-6 result as JSON: SSIM min/mean of the frames against the approved reference
        (values from `ffmpeg -lavfi ssim`), and the mean CIEDE2000 of the grey wedge and the skin
        patches of the tonemapped frames in DIR (frame-NN.png, 8-bit sRGB/BT.709) against the
        software frames in --ref. Pass: SSIM min >= 0.95 and both mean dE <= 5.
"""

import argparse
import glob
import json
import math
import os
import struct
import sys
import zlib

W, H = 3840, 2160
SSIM_MIN = 0.95
DELTA_E_MAX = 5.0

# Grey wedge: 11 patches across the top half, PQ signal 0 %, 10 %, ..., 100 % of full scale
# (R' = G' = B'; 10-bit limited range code 64 + 876 * level).
WEDGE = [(round(i * W / 11), 0, round((i + 1) * W / 11), H // 2, i / 10) for i in range(11)]

# Skin patches: 2 rows x 3 columns in the bottom half, 960 x 360 each (x = 240 + 1200 c,
# y = 1200 + 480 r). Colours are 8-bit sRGB references (ColorChecker classic "light skin" and
# "dark skin", Monk Skin Tone scale 2, 4, 6, 8), placed in BT.2020 with sRGB white at 203 cd/m2
# (BT.2408 reference white) and PQ-encoded.
SKIN_COLOURS = [
    ("cc-light-skin", (194, 150, 130)),
    ("cc-dark-skin", (115, 82, 68)),
    ("mst-2", (243, 231, 219)),
    ("mst-4", (234, 218, 186)),
    ("mst-6", (160, 126, 86)),
    ("mst-8", (96, 65, 52)),
]
SKIN = [(240 + 1200 * (k % 3), 1200 + 480 * (k // 3), 240 + 1200 * (k % 3) + 960, 1200 + 480 * (k // 3) + 360, name, rgb)
        for k, (name, rgb) in enumerate(SKIN_COLOURS)]
REFERENCE_WHITE = 203.0  # cd/m2

# Measured area: the inner 60 % of every patch, away from edges and chroma bleeding.
INSET = 0.2

BT709_TO_BT2020 = [
    [0.627404, 0.329283, 0.043313],
    [0.069097, 0.919540, 0.011362],
    [0.016391, 0.088013, 0.895595],
]


def srgb_to_linear(c):
    c /= 255.0
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def pq_encode(nits):
    m1, m2, c1, c2, c3 = 0.1593017578125, 78.84375, 0.8359375, 18.8515625, 18.6875
    y = max(nits, 0.0) / 10000.0
    return ((c1 + c2 * y ** m1) / (1 + c3 * y ** m1)) ** m2


def skin_pq(rgb):
    lin = [srgb_to_linear(c) for c in rgb]
    bt2020 = [sum(m * v for m, v in zip(row, lin)) for row in BT709_TO_BT2020]
    return [pq_encode(v * REFERENCE_WHITE) for v in bt2020]


def chart(out):
    black = struct.pack(">HHH", 0, 0, 0)
    rows = []
    top = bytearray()
    for x0, _, x1, _, level in WEDGE:
        v = round(level * 65535)
        top += struct.pack(">HHH", v, v, v) * (x1 - x0)
    rows.append((H // 2, bytes(top)))
    plain = black * W
    for r in range(2):
        y0 = SKIN[3 * r][1]
        line = bytearray(plain)
        for x0, _, x1, _, _, rgb in SKIN[3 * r : 3 * r + 3]:
            px = struct.pack(">HHH", *(round(v * 65535) for v in skin_pq(rgb)))
            line[x0 * 6 : x1 * 6] = px * (x1 - x0)
        prev_end = H // 2 if r == 0 else SKIN[0][3]
        rows.append((y0 - prev_end, plain))
        rows.append((360, bytes(line)))
    rows.append((H - SKIN[3][3], plain))
    with open(out, "wb") as f:
        f.write(b"P6\n%d %d\n65535\n" % (W, H))
        for n, line in rows:
            for _ in range(n):
                f.write(line)


def read_png(path):
    """Decode an 8-bit RGB or RGBA non-interlaced PNG into (width, height, rows of RGB bytes)."""
    with open(path, "rb") as f:
        data = f.read()
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError(f"{path}: not a PNG")
    pos, idat, w = 8, bytearray(), 0
    while pos < len(data):
        n, kind = struct.unpack(">I4s", data[pos : pos + 8])
        body = data[pos + 8 : pos + 8 + n]
        if kind == b"IHDR":
            w, h, depth, ctype, _, _, interlace = struct.unpack(">IIBBBBB", body)
            if depth != 8 or ctype not in (2, 6) or interlace:
                raise ValueError(f"{path}: need 8-bit RGB(A) without interlace")
            bpp = 3 if ctype == 2 else 4
        elif kind == b"IDAT":
            idat += body
        pos += 12 + n
    raw = zlib.decompress(bytes(idat))
    stride = w * bpp
    prev = bytearray(stride)
    rows = []
    for y in range(h):
        ftype = raw[y * (stride + 1)]
        line = bytearray(raw[y * (stride + 1) + 1 : (y + 1) * (stride + 1)])
        for i in range(stride):
            a = line[i - bpp] if i >= bpp else 0
            b = prev[i]
            c = prev[i - bpp] if i >= bpp else 0
            if ftype == 1:
                line[i] = (line[i] + a) & 0xFF
            elif ftype == 2:
                line[i] = (line[i] + b) & 0xFF
            elif ftype == 3:
                line[i] = (line[i] + (a + b) // 2) & 0xFF
            elif ftype == 4:
                p = a + b - c
                pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
                line[i] = (line[i] + (a if pa <= pb and pa <= pc else b if pb <= pc else c)) & 0xFF
        prev = line
        rows.append(bytes(line) if bpp == 3 else bytes(v for k, v in enumerate(line) if k % 4 != 3))
    return w, h, rows


def mean_rgb(img, box):
    w, h, rows = img
    x0, y0, x1, y1 = box
    dx, dy = (x1 - x0) * INSET, (y1 - y0) * INSET
    xa, xb = int((x0 + dx) * w / W), int((x1 - dx) * w / W)
    ya, yb = int((y0 + dy) * h / H), int((y1 - dy) * h / H)
    s = [0, 0, 0]
    for y in range(ya, yb):
        line = rows[y]
        for x in range(xa, xb):
            for k in range(3):
                s[k] += line[3 * x + k]
    n = (xb - xa) * (yb - ya)
    return [v / n for v in s]


def lab(rgb):
    r, g, b = (srgb_to_linear(c) for c in rgb)
    x = 0.4124564 * r + 0.3575761 * g + 0.1804375 * b
    y = 0.2126729 * r + 0.7151522 * g + 0.0721750 * b
    z = 0.0193339 * r + 0.1191920 * g + 0.9503041 * b

    def f(t):
        return t ** (1 / 3) if t > (6 / 29) ** 3 else t / (3 * (6 / 29) ** 2) + 4 / 29

    fx, fy, fz = f(x / 0.95047), f(y / 1.0), f(z / 1.08883)
    return 116 * fy - 16, 500 * (fx - fy), 200 * (fy - fz)


def ciede2000(lab1, lab2):
    l1, a1, b1 = lab1
    l2, a2, b2 = lab2
    c1, c2 = math.hypot(a1, b1), math.hypot(a2, b2)
    cm = (c1 + c2) / 2
    g = 0.5 * (1 - math.sqrt(cm**7 / (cm**7 + 25**7)))
    a1p, a2p = a1 * (1 + g), a2 * (1 + g)
    c1p, c2p = math.hypot(a1p, b1), math.hypot(a2p, b2)
    h1p = math.degrees(math.atan2(b1, a1p)) % 360 if c1p else 0.0
    h2p = math.degrees(math.atan2(b2, a2p)) % 360 if c2p else 0.0
    dl, dc = l2 - l1, c2p - c1p
    if c1p * c2p == 0:
        dh = 0.0
    else:
        dh = h2p - h1p
        if dh > 180:
            dh -= 360
        elif dh < -180:
            dh += 360
    dH = 2 * math.sqrt(c1p * c2p) * math.sin(math.radians(dh / 2))
    lm, cmp_ = (l1 + l2) / 2, (c1p + c2p) / 2
    if c1p * c2p == 0:
        hm = h1p + h2p
    elif abs(h1p - h2p) <= 180:
        hm = (h1p + h2p) / 2
    else:
        hm = (h1p + h2p + 360) / 2 if h1p + h2p < 360 else (h1p + h2p - 360) / 2
    t = (1 - 0.17 * math.cos(math.radians(hm - 30)) + 0.24 * math.cos(math.radians(2 * hm))
         + 0.32 * math.cos(math.radians(3 * hm + 6)) - 0.20 * math.cos(math.radians(4 * hm - 63)))
    sl = 1 + 0.015 * (lm - 50) ** 2 / math.sqrt(20 + (lm - 50) ** 2)
    sc = 1 + 0.045 * cmp_
    sh = 1 + 0.015 * cmp_ * t
    rt = (-2 * math.sqrt(cmp_**7 / (cmp_**7 + 25**7))
          * math.sin(math.radians(60 * math.exp(-(((hm - 275) / 25) ** 2)))))
    return math.sqrt((dl / sl) ** 2 + (dc / sc) ** 2 + (dH / sh) ** 2 + rt * (dc / sc) * (dH / sh))


def frames(d):
    return sorted(glob.glob(os.path.join(d, "frame-*.png")))


def delta_e(test_dir, ref_dir):
    tests, refs = frames(test_dir), frames(ref_dir)
    if not tests or len(tests) != len(refs):
        raise SystemExit(f"hdr-quality: {test_dir} has {len(tests)} frames, {ref_dir} has {len(refs)}")
    regions = {"wedge": [w[:4] for w in WEDGE], "skin": [s[:4] for s in SKIN]}
    sums = {k: 0.0 for k in regions}
    per_patch = {k: [0.0] * len(v) for k, v in regions.items()}
    for t, r in zip(tests, refs):
        ti, ri = read_png(t), read_png(r)
        for k, boxes in regions.items():
            for i, box in enumerate(boxes):
                d = ciede2000(lab(mean_rgb(ri, box)), lab(mean_rgb(ti, box)))
                per_patch[k][i] += d / len(tests)
                sums[k] += d / (len(tests) * len(boxes))
    return {k: {"mean": round(sums[k], 3), "patches": [round(v, 3) for v in per_patch[k]]} for k in regions}


def report(args):
    res = {"backend": args.backend, "frames": len(frames(args.test)),
           "thresholds": {"ssimMin": SSIM_MIN, "deltaEMean": DELTA_E_MAX}}
    ok = True
    if args.ssim:
        vals = [float(v) for v in args.ssim]
        res["ssim"] = {"min": round(min(vals), 4), "mean": round(sum(vals) / len(vals), 4)}
        ok = ok and min(vals) >= SSIM_MIN
    else:
        res["ssim"] = None  # reference just created
    if args.ref:
        res["deltaE2000"] = delta_e(args.test, args.ref)
        ok = ok and all(v["mean"] <= DELTA_E_MAX for v in res["deltaE2000"].values())
    else:
        res["deltaE2000"] = None  # the software frames are the reference
    res["pass"] = ok
    print(json.dumps(res, indent=2))


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    c = sub.add_parser("chart")
    c.add_argument("out")
    r = sub.add_parser("report")
    r.add_argument("--backend", required=True)
    r.add_argument("--test", required=True)
    r.add_argument("--ref")
    r.add_argument("--ssim", nargs="*")
    args = ap.parse_args()
    if args.cmd == "chart":
        chart(args.out)
    else:
        report(args)


if __name__ == "__main__":
    sys.exit(main())
