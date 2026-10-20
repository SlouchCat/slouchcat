#!/usr/bin/env python3
"""Write a PGS subtitle stream (.sup, codec hdmv_pgs_subtitle) with bitmap captions.

ffmpeg has no PGS encoder, so the test media (generate-media.sh, plan 10.3 row "H.264 + PGS") get
their image subtitles from this script. Standard library only; run with `python3 -I`.

The stream opens with an empty display set at 0 s. Each caption is one display set (PCS with epoch
start, WDS, PDS, ODS, END) at its start time and a clearing display set (PCS without objects, WDS,
END) at its end time. The bitmap is white text from
a built-in 5x7 pixel font, scaled up, on a half-transparent black box, centred near the bottom.

    python3 -I make-pgs.py OUT.sup [--width 1920] [--height 1080] [--caption START END TEXT ...]

Without --caption the two default captions are written: "Slouchcat PGS 1" from 2 to 6 s and
"Slouchcat PGS 2" from 8 to 12 s.
"""

import argparse
import struct
import sys

# 5x7 glyphs, one string of five bits per row. Only the characters the test captions need.
FONT = {
    " ": ["00000"] * 7,
    "S": ["01111", "10000", "10000", "01110", "00001", "00001", "11110"],
    "P": ["11110", "10001", "10001", "11110", "10000", "10000", "10000"],
    "G": ["01110", "10001", "10000", "10111", "10001", "10001", "01111"],
    "l": ["01100", "00100", "00100", "00100", "00100", "00100", "01110"],
    "o": ["00000", "00000", "01110", "10001", "10001", "10001", "01110"],
    "u": ["00000", "00000", "10001", "10001", "10001", "10011", "01101"],
    "c": ["00000", "00000", "01110", "10000", "10000", "10001", "01110"],
    "h": ["10000", "10000", "10110", "11001", "10001", "10001", "10001"],
    "a": ["00000", "00000", "01110", "00001", "01111", "10001", "01111"],
    "t": ["01000", "01000", "11100", "01000", "01000", "01001", "00110"],
    "1": ["00100", "01100", "00100", "00100", "00100", "00100", "01110"],
    "2": ["01110", "10001", "00001", "00010", "00100", "01000", "11111"],
}

SCALE = 8  # pixels per font dot
PAD = 16  # box padding around the text
BOTTOM = 80  # distance of the box from the bottom edge

# Palette entries: id, Y, Cr, Cb, alpha (BT.709 limited range).
TRANSPARENT, BOX, TEXT = 0, 1, 2
PALETTE = [(TRANSPARENT, 16, 128, 128, 0), (BOX, 16, 128, 128, 160), (TEXT, 235, 128, 128, 255)]

PCS, WDS, PDS, ODS, END = 0x16, 0x17, 0x14, 0x15, 0x80


def render(text):
    """Return the caption bitmap as rows of palette indices."""
    for ch in text:
        if ch not in FONT:
            sys.exit(f"make-pgs: no glyph for {ch!r}; the built-in font has {''.join(sorted(FONT))!r}")
    cell = 6 * SCALE  # 5 dots plus one dot of spacing
    width = len(text) * cell - SCALE + 2 * PAD
    height = 7 * SCALE + 2 * PAD
    rows = [[BOX] * width for _ in range(height)]
    for i, ch in enumerate(text):
        for gy, bits in enumerate(FONT[ch]):
            for gx, bit in enumerate(bits):
                if bit != "1":
                    continue
                for dy in range(SCALE):
                    y = PAD + gy * SCALE + dy
                    x0 = PAD + i * cell + gx * SCALE
                    rows[y][x0 : x0 + SCALE] = [TEXT] * SCALE
    return rows


def rle(rows):
    """PGS run-length encoding (each line ends with 00 00)."""
    out = bytearray()
    for row in rows:
        x = 0
        while x < len(row):
            color = row[x]
            run = 1
            while x + run < len(row) and row[x + run] == color and run < 16383:
                run += 1
            if color != 0 and run <= 2:
                out += bytes([color]) * run
            elif color == 0:
                out += bytes([0, run]) if run < 64 else bytes([0, 0x40 | (run >> 8), run & 0xFF])
            elif run < 64:
                out += bytes([0, 0x80 | run, color])
            else:
                out += bytes([0, 0xC0 | (run >> 8), run & 0xFF, color])
            x += run
        out += b"\x00\x00"
    return bytes(out)


def segment(kind, pts, payload):
    return b"PG" + struct.pack(">IIBH", pts, 0, kind, len(payload)) + payload


def display_sets(captions, width, height):
    # An empty display set at 0 s: the stream starts at 0, so muxing it next to a video that starts
    # at 0 keeps the caption times (ffmpeg shifts every input by its start time).
    pcs = struct.pack(">HHBHBBBB", width, height, 0x10, 0, 0x80, 0, 0, 0)
    data = bytearray(segment(PCS, 0, pcs) + segment(END, 0, b""))
    for k, (start, end, text) in enumerate(captions):
        number = k + 1
        rows = render(text)
        w, h = len(rows[0]), len(rows)
        if w > width:
            sys.exit(f"make-pgs: caption {text!r} is wider than the video ({w} > {width})")
        x, y = (width - w) // 2, height - BOTTOM - h
        on, off = round(start * 90000), round(end * 90000)
        window = struct.pack(">BBHHHH", 1, 0, x, y, w, h)
        # Show: epoch start, one object in window 0.
        pcs = struct.pack(">HHBHBBBB", width, height, 0x10, 2 * number, 0x80, 0, 0, 1)
        pcs += struct.pack(">HBBHH", 0, 0, 0, x, y)
        pds = struct.pack(">BB", 0, 0) + b"".join(struct.pack(">BBBBB", *e) for e in PALETTE)
        bitmap = rle(rows)
        ods_len = len(bitmap) + 4
        ods = struct.pack(">HBB", 0, 0, 0xC0) + ods_len.to_bytes(3, "big") + struct.pack(">HH", w, h) + bitmap
        if len(ods) > 0xFFFF:
            sys.exit(f"make-pgs: caption {text!r} is too large for one object segment")
        data += segment(PCS, on, pcs) + segment(WDS, on, window) + segment(PDS, on, pds)
        data += segment(ODS, on, ods) + segment(END, on, b"")
        # Clear: a normal composition without objects.
        pcs = struct.pack(">HHBHBBBB", width, height, 0x10, 2 * number + 1, 0x00, 0, 0, 0)
        data += segment(PCS, off, pcs) + segment(WDS, off, window) + segment(END, off, b"")
    return bytes(data)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("out", help="output .sup file")
    ap.add_argument("--width", type=int, default=1920)
    ap.add_argument("--height", type=int, default=1080)
    ap.add_argument("--caption", nargs=3, action="append", metavar=("START", "END", "TEXT"),
                    help="caption in seconds; repeatable")
    args = ap.parse_args()
    captions = [(float(s), float(e), t) for s, e, t in args.caption] if args.caption else [
        (2.0, 6.0, "Slouchcat PGS 1"),
        (8.0, 12.0, "Slouchcat PGS 2"),
    ]
    for start, end, text in captions:
        if not 0 <= start < end:
            sys.exit(f"make-pgs: caption {text!r} needs 0 <= start < end")
    with open(args.out, "wb") as f:
        f.write(display_sets(captions, args.width, args.height))


if __name__ == "__main__":
    main()
