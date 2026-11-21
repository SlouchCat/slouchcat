#!/usr/bin/env python3
"""Builds the TV banner (concept 26, K26 stage 0 checklist TV-LB/TV-BN) as an Android vector drawable.

    python3 android/app-tv/banner/build_banner.py <font.ttf> > android/app-tv/src/main/res/drawable/banner.xml

320 x 180 dp (xhdpi: 320 x 180 px as the TV quality list asks), the colours from
packages/design-tokens (bg, text, accent), the app icon of web/public/icon.svg on the left and the
wordmark «Slouchcat» («cat» in the accent) as outlines of a bold sans font (the repo used Noto Sans
Bold, SIL OFL 1.1; any TrueType font with simple glyphs works). Only the standard library: the
glyphs are read from the font's cmap, loca and glyf tables, quadratic curves stay quadratic.
"""
import json
import pathlib
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


def main():
    font = Font(pathlib.Path(sys.argv[1]).read_bytes())
    bg = token("color.bg.base")
    tx = token("color.text.primary")
    accent = token("color.accent.ember.base")
    # The app icon of web/public/icon.svg (512 units) scaled into a square of 96 at the left.
    icon = 96 / 512
    ix, iy = 28, 42
    body = "M136 400V136l80 72h80l80-72v264a24 24 0 0 1-24 24H160a24 24 0 0 1-24-24z"
    play = "M228 268v96l84-48z"
    size = 30
    first, x = text_paths(font, "Slouch", size, 136, 101)
    second, end = text_paths(font, "cat", size, x, 101)
    if end > 304:
        raise SystemExit("the wordmark is %.0f wide, more than the banner holds" % (end - 136))
    print('<?xml version="1.0" encoding="utf-8"?>')
    print("<!-- The TV banner (320 x 180, TV-LB, TV-BN). Generated by android/app-tv/banner/build_banner.py from")
    print("     the design tokens, web/public/icon.svg and the outlines of a bold sans font; do not edit by hand. -->")
    print('<vector xmlns:android="http://schemas.android.com/apk/res/android"')
    print('    android:width="320dp" android:height="180dp" android:viewportWidth="320" android:viewportHeight="180">')
    print('    <path android:fillColor="%s" android:pathData="M0 0h320v180H0z" />' % bg)
    print('    <group android:translateX="%s" android:translateY="%s" android:scaleX="%s" android:scaleY="%s">' % (ix, iy, icon, icon))
    print('        <path android:fillColor="%s" android:pathData="M0 0h512v512H0z" />' % bg)
    print('        <path android:fillColor="%s" android:pathData="%s" />' % (accent, body))
    print('        <path android:fillColor="%s" android:pathData="%s" />' % (bg, play))
    print("    </group>")
    print('    <path android:fillColor="%s" android:pathData="%s" />' % (tx, first))
    print('    <path android:fillColor="%s" android:pathData="%s" />' % (accent, second))
    print("</vector>")


if __name__ == "__main__":
    main()
