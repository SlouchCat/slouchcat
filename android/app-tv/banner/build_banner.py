#!/usr/bin/env python3
"""Builds the TV banner (concept 26, K26 stage 0 checklist TV-LB/TV-BN) as an Android vector drawable,
and from the same layout the two store graphics that show it (docs/store/grafiken.md).

    python3 android/app-tv/banner/build_banner.py <font.ttf> > android/app-tv/src/main/res/drawable/banner.xml
    python3 android/app-tv/banner/build_banner.py <font.ttf> tv-banner > docs/store/grafiken/tv-banner.svg
    python3 android/app-tv/banner/build_banner.py <font.ttf> feature-graphic > docs/store/grafiken/feature-graphic.svg

320 x 180 dp (xhdpi: 320 x 180 px as the TV quality list asks), the colours from
packages/design-tokens (bg, text, accent), the sign of web/public/icon.svg on the left and the
wordmark «Slouchcat» («cat» in the accent) as outlines of a bold sans font (the repo used Noto Sans
Bold, SIL OFL 1.1; any TrueType font with simple glyphs works). Only the standard library: the
glyphs are read from the font's cmap, loca and glyf tables, quadratic curves stay quadratic.

Sign and name sit in the middle of the banner as one group, measured by what is drawn: from the
left edge of the sign to the right edge of the last letter, and each of the two around the
horizontal middle line. The empty canvas around the sign in icon.svg does not count.
"""
import json
import pathlib
import re
import struct
import sys

ROOT = pathlib.Path(__file__).resolve().parents[3]


def token(path):
    data = json.loads((ROOT / "packages/design-tokens/tokens/color.json").read_text())
    node = data
    for part in path.split("."):
        node = node[part]
    value = node["$value"]
    if value.startswith("{"):
        return token(value.strip("{}"))
    return value


class Font:
    def __init__(self, raw):
        self.raw = raw
        count = struct.unpack(">H", raw[4:6])[0]
        self.tables = {}
        for i in range(count):
            tag, _, off, length = struct.unpack(">4sIII", raw[12 + 16 * i: 28 + 16 * i])
            self.tables[tag.decode()] = (off, length)
        head = self.tables["head"][0]
        self.units = struct.unpack(">H", raw[head + 18: head + 20])[0]
        self.long_loca = struct.unpack(">h", raw[head + 50: head + 52])[0] == 1
        hhea = self.tables["hhea"][0]
        self.metrics = struct.unpack(">H", raw[hhea + 34: hhea + 36])[0]
        self.cmap = self._cmap()

    def _cmap(self):
        off = self.tables["cmap"][0]
        n = struct.unpack(">H", self.raw[off + 2: off + 4])[0]
        for i in range(n):
            pid, eid, sub = struct.unpack(">HHI", self.raw[off + 4 + 8 * i: off + 12 + 8 * i])
            base = off + sub
            if struct.unpack(">H", self.raw[base: base + 2])[0] == 4 and pid in (0, 3):
                return self._format4(base)
        raise SystemExit("no cmap format 4")

    def _format4(self, base):
        segs = struct.unpack(">H", self.raw[base + 6: base + 8])[0] // 2
        arr = lambda start: struct.unpack(">%dH" % segs, self.raw[start: start + 2 * segs])
        ends = arr(base + 14)
        starts = arr(base + 16 + 2 * segs)
        deltas = struct.unpack(">%dh" % segs, self.raw[base + 16 + 4 * segs: base + 16 + 6 * segs])
        ro_base = base + 16 + 6 * segs
        ranges = arr(ro_base)
        table = {}
        for s in range(segs):
            for code in range(starts[s], ends[s] + 1):
                if code == 0xFFFF:
                    continue
                if ranges[s] == 0:
                    gid = (code + deltas[s]) & 0xFFFF
                else:
                    at = ro_base + 2 * s + ranges[s] + 2 * (code - starts[s])
                    gid = struct.unpack(">H", self.raw[at: at + 2])[0]
                    if gid:
                        gid = (gid + deltas[s]) & 0xFFFF
                table[code] = gid
        return table

    def advance(self, gid):
        off = self.tables["hmtx"][0]
        i = min(gid, self.metrics - 1)
        return struct.unpack(">H", self.raw[off + 4 * i: off + 4 * i + 2])[0]

    def box(self, gid):
        """xMin, yMin, xMax, yMax of a glyph in font units, None for an empty glyph."""
        loca = self.tables["loca"][0]
        if self.long_loca:
            a, b = struct.unpack(">II", self.raw[loca + 4 * gid: loca + 4 * gid + 8])
        else:
            a, b = (2 * x for x in struct.unpack(">HH", self.raw[loca + 2 * gid: loca + 2 * gid + 4]))
        if a == b:
            return None
        g = self.tables["glyf"][0] + a
        return struct.unpack(">4h", self.raw[g + 2: g + 10])

    def contours(self, gid):
        loca = self.tables["loca"][0]
        if self.long_loca:
            a, b = struct.unpack(">II", self.raw[loca + 4 * gid: loca + 4 * gid + 8])
        else:
            a, b = (2 * x for x in struct.unpack(">HH", self.raw[loca + 2 * gid: loca + 2 * gid + 4]))
        if a == b:
            return []
        g = self.tables["glyf"][0] + a
        n = struct.unpack(">h", self.raw[g: g + 2])[0]
        if n < 0:
            raise SystemExit("compound glyph %d: take another font" % gid)
        ends = struct.unpack(">%dH" % n, self.raw[g + 10: g + 10 + 2 * n])
        points = ends[-1] + 1
        p = g + 10 + 2 * n
        ilen = struct.unpack(">H", self.raw[p: p + 2])[0]
        p += 2 + ilen
        flags = []
        while len(flags) < points:
            f = self.raw[p]
            p += 1
            flags.append(f)
            if f & 8:
                rep = self.raw[p]
                p += 1
                flags.extend([f] * rep)
        def coords(short, same):
            nonlocal p
            out, v = [], 0
            for f in flags:
                if f & short:
                    d = self.raw[p]
                    p += 1
                    v += d if f & same else -d
                elif not f & same:
                    v += struct.unpack(">h", self.raw[p: p + 2])[0]
                    p += 2
                out.append(v)
            return out
        xs = coords(2, 16)
        ys = coords(4, 32)
        result, start = [], 0
        for end in ends:
            result.append([(xs[i], ys[i], bool(flags[i] & 1)) for i in range(start, end + 1)])
            start = end + 1
        return result


def contour_path(points, sx, ox, oy):
    """One closed contour as SVG path commands (M, L, Q, Z) in the drawable's units."""
    pt = lambda x, y: (ox + x * sx, oy - y * sx)
    n = len(points)
    # Start at an on-curve point (or the midpoint of two off-curve points).
    first = next((i for i, q in enumerate(points) if q[2]), None)
    if first is None:
        a, b = points[0], points[1]
        start = ((a[0] + b[0]) / 2, (a[1] + b[1]) / 2, True)
        seq = points[1:] + points[:1]
    else:
        start = points[first]
        seq = points[first + 1:] + points[: first + 1]
    out = ["M%.2f %.2f" % pt(start[0], start[1])]
    pending = None
    for q in seq:
        if q[2]:
            if pending:
                out.append("Q%.2f %.2f %.2f %.2f" % (*pt(*pending), *pt(q[0], q[1])))
                pending = None
            else:
                out.append("L%.2f %.2f" % pt(q[0], q[1]))
        else:
            if pending:
                mid = ((pending[0] + q[0]) / 2, (pending[1] + q[1]) / 2)
                out.append("Q%.2f %.2f %.2f %.2f" % (*pt(*pending), *pt(*mid)))
            pending = (q[0], q[1])
    if pending:
        out.append("Q%.2f %.2f %.2f %.2f" % (*pt(*pending), *pt(start[0], start[1])))
    out.append("Z")
    return "".join(out)


def text_paths(font, text, size, x, baseline):
    scale = size / font.units
    paths = []
    for ch in text:
        gid = font.cmap.get(ord(ch), 0)
        d = "".join(contour_path(c, scale, x, baseline) for c in font.contours(gid))
        paths.append(d)
        x += font.advance(gid) * scale
    return "".join(paths), x


def text_box(font, text, size):
    """What is drawn of a text set at x 0 on the baseline 0: left, top, right, bottom (y grows downwards)."""
    scale = size / font.units
    x, boxes = 0.0, []
    for ch in text:
        gid = font.cmap.get(ord(ch), 0)
        box = font.box(gid)
        if box:
            boxes.append((x + box[0] * scale, -box[3] * scale, x + box[2] * scale, -box[1] * scale))
        x += font.advance(gid) * scale
    return min(b[0] for b in boxes), min(b[1] for b in boxes), max(b[2] for b in boxes), max(b[3] for b in boxes)


def path_box(d):
    """The box of the end points of a path of M, L, H, V, A and Z commands, absolute or relative.

    Enough for the sign: its two arcs are rounded corners, which reach no further than their end points.
    Reads numbers as icon.svg writes them (digits, an optional fraction, a sign); a path packed further,
    say with arc flags run together, stops the build instead of giving a wrong box."""
    x = y = 0.0
    xs, ys = [], []
    for command, args in re.findall(r"([MLHVAZmlhvaz])([^MLHVAZmlhvaz]*)", d):
        numbers = [float(n) for n in re.findall(r"-?\d+(?:\.\d+)?", args)]
        kind = command.upper()
        if kind == "Z":
            continue
        step = {"M": 2, "L": 2, "H": 1, "V": 1, "A": 7}[kind]
        if not numbers or len(numbers) % step:
            raise SystemExit("web/public/icon.svg: cannot read the path command %s%s" % (command, args))
        for i in range(0, len(numbers), step):
            part = numbers[i: i + step]
            if kind == "H":
                x = part[0] + (x if command.islower() else 0)
            elif kind == "V":
                y = part[0] + (y if command.islower() else 0)
            elif command.islower():
                x, y = x + part[-2], y + part[-1]
            else:
                x, y = part[-2], part[-1]
            xs.append(x)
            ys.append(y)
    return min(xs), min(ys), max(xs), max(ys)


def sign():
    """The two paths of web/public/icon.svg, the head and the play triangle, and the head's box."""
    svg = (ROOT / "web/public/icon.svg").read_text()
    paths = re.findall(r'<path d="([^"]+)"', svg)
    if len(paths) != 2:
        raise SystemExit("web/public/icon.svg: expected the head and the play triangle, found %d paths" % len(paths))
    box = path_box(paths[0])
    if (box[0] + box[2], box[1] + box[3]) != (CANVAS, CANVAS):
        raise SystemExit("web/public/icon.svg: the sign is not in the middle of its canvas (box %s)" % (box,))
    return paths[0], paths[1], box


CANVAS = 512        # the canvas of web/public/icon.svg
WIDTH, HEIGHT = 320, 180
SIGN = 96 / CANVAS  # the canvas of the sign as a square of 96
GAP = 37.5          # from the right edge of the sign to the origin of the name (its first letter starts a little later)
SIZE = 30           # font size of the name
EDGE = 16           # what stays free at the left and the right at least


def number(value):
    return ("%.4f" % value).rstrip("0").rstrip(".")


def layout(font):
    """Where sign and name go: both around the middle line, together in the middle of the width."""
    head, play, box = sign()
    _, top, right, bottom = text_box(font, "Slouchcat", SIZE)
    sign_width = (box[2] - box[0]) * SIGN
    drawn = sign_width + GAP + right
    if drawn > WIDTH - 2 * EDGE:
        raise SystemExit("sign and wordmark are %.0f wide, more than the banner holds" % drawn)
    sign_left = (WIDTH - drawn) / 2
    origin = sign_left + sign_width + GAP
    baseline = HEIGHT / 2 - (top + bottom) / 2
    first, x = text_paths(font, "Slouch", SIZE, origin, baseline)
    second, _ = text_paths(font, "cat", SIZE, x, baseline)
    return {
        "bg": token("color.bg.base"),
        "tx": token("color.text.primary"),
        "accent": token("color.accent.ember.base"),
        "x": number(sign_left - box[0] * SIGN),
        "y": number(HEIGHT / 2 - CANVAS / 2 * SIGN),
        "scale": number(SIGN),
        "head": head,
        "play": play,
        "first": first,
        "second": second,
    }


def vector(v):
    print('<?xml version="1.0" encoding="utf-8"?>')
    print("<!-- The TV banner (320 x 180, TV-LB, TV-BN). Generated by android/app-tv/banner/build_banner.py from")
    print("     the design tokens, web/public/icon.svg and the outlines of a bold sans font; do not edit by hand. -->")
    print('<vector xmlns:android="http://schemas.android.com/apk/res/android"')
    print('    android:width="320dp" android:height="180dp" android:viewportWidth="320" android:viewportHeight="180">')
    print('    <path android:fillColor="%s" android:pathData="M0 0h320v180H0z" />' % v["bg"])
    print('    <group android:translateX="%s" android:translateY="%s" android:scaleX="%s" android:scaleY="%s">' % (v["x"], v["y"], v["scale"], v["scale"]))
    print('        <path android:fillColor="%s" android:pathData="%s" />' % (v["accent"], v["head"]))
    print('        <path android:fillColor="%s" android:pathData="%s" />' % (v["bg"], v["play"]))
    print("    </group>")
    print('    <path android:fillColor="%s" android:pathData="%s" />' % (v["tx"], v["first"]))
    print('    <path android:fillColor="%s" android:pathData="%s" />' % (v["accent"], v["second"]))
    print("</vector>")


# The store graphics: the banner enlarged around the middle of their own size.
STORE = {
    "tv-banner": (1280, 720, 4, "TV banner of the store, 1280 x 720 px: the in-app banner (320 x 180) times four."),
    "feature-graphic": (1024, 500, 3, "Feature graphic of the store, 1024 x 500 px: the in-app banner times three, cut to the size."),
}


def store(v, name):
    width, height, scale, what = STORE[name]
    print('<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">' % (width, height, width, height))
    print("  <!-- %s" % what)
    print("       Sign and name «Slouchcat» in the middle. Generated by android/app-tv/banner/build_banner.py")
    print("       from the design tokens (color.bg.base, color.text.primary, color.accent.ember.base),")
    print("       web/public/icon.svg and the outlines of a bold sans font; do not edit by hand. Rendered by")
    print("       scripts/store-graphics.sh. -->")
    print('  <rect width="%d" height="%d" fill="%s" />' % (width, height, v["bg"]))
    print('  <g transform="translate(%s %s) scale(%d)">' % (number((width - WIDTH * scale) / 2), number((height - HEIGHT * scale) / 2), scale))
    print('    <g transform="translate(%s %s) scale(%s)">' % (v["x"], v["y"], v["scale"]))
    print('      <path fill="%s" d="%s" />' % (v["accent"], v["head"]))
    print('      <path fill="%s" d="%s" />' % (v["bg"], v["play"]))
    print("    </g>")
    print('    <path fill="%s" d="%s" />' % (v["tx"], v["first"]))
    print('    <path fill="%s" d="%s" />' % (v["accent"], v["second"]))
    print("  </g>")
    print("</svg>")


def main():
    if len(sys.argv) not in (2, 3) or (len(sys.argv) == 3 and sys.argv[2] not in STORE):
        raise SystemExit("usage: build_banner.py <font.ttf> [%s]" % "|".join(STORE))
    v = layout(Font(pathlib.Path(sys.argv[1]).read_bytes()))
    if len(sys.argv) == 3:
        store(v, sys.argv[2])
    else:
        vector(v)


if __name__ == "__main__":
    main()
