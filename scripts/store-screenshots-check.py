"""Checks the store photos of scripts/store-screenshots.sh against docs/store/grafiken.md.

    python3 scripts/store-screenshots-check.py <folder> [<set> ...]

<set> is <device>-<language> (phone-de, tablet7-en, tv-de ...); without sets every set of the
folder is checked. Each named set must exist with every motif of its device (MOTIFS); a missing
set or motif is a fault. Every file must be a PNG with 24 bit RGB (no alpha channel; the script
writes PNG only, the store would also take JPEG), at most 8 MB, with the size and aspect ratio of
its device: phone sides 320 to 3840 px with the long side at most 2.3 times the short one and at
least 1080 px on the short side (the player in landscape), tablets 16:9 with sides 1080 to 7680 px,
TV 1920 x 1080. Prints one line per file and exits non-zero when any fault was found.
"""

import os
import struct
import sys

MOTIFS = {
    "phone": ["01-home", "02-detail", "03-player", "04-downloads", "05-profiles", "06-audiobook", "07-tv-link"],
    "tablet7": ["01-home", "02-library", "03-detail", "04-player"],
    "tablet10": ["01-home", "02-library", "03-detail", "04-player"],
    "tv": ["01-home", "02-detail", "03-player-audio", "04-setup-qr"],
}
SIZES = {"tablet7": (1920, 1080), "tablet10": (2560, 1440), "tv": (1920, 1080)}
MAX_BYTES = 8 * 1024 * 1024
PNG_MAGIC = b"\x89PNG\r\n\x1a\n"


def png_header(path):
    """Width, height, bit depth and colour type of a PNG (IHDR), or None for another format."""
    with open(path, "rb") as f:
        head = f.read(33)
    if not head.startswith(PNG_MAGIC) or head[12:16] != b"IHDR":
        return None
    width, height, depth, colour = struct.unpack(">IIBB", head[16:26])
    return width, height, depth, colour


def faults(device, motif, path):
    out = []
    size = os.path.getsize(path)
    if size > MAX_BYTES:
        out.append(f"{size} bytes, more than 8 MB")
    header = png_header(path)
    if header is None:
        return out + ["not a PNG"], None
    width, height, depth, colour = header
    if depth != 8 or colour != 2:
        out.append(f"PNG with bit depth {depth} and colour type {colour}, not 24 bit RGB without alpha")
    long_side, short_side = max(width, height), min(width, height)
    if device == "phone":
        if short_side < 320 or long_side > 3840:
            out.append("a side outside 320 to 3840 px")
        if long_side > 2.3 * short_side:
            out.append("the long side more than 2.3 times the short one")
        if short_side < 1080:
            out.append("shorter than 1080 px (no store recommendation)")
        if motif == "03-player" and width < height:
            out.append("the player is not in landscape")
    else:
        if (width, height) != SIZES[device]:
            out.append(f"not {SIZES[device][0]} x {SIZES[device][1]}")
        if width * 9 != height * 16:
            out.append("not 16:9")
        if short_side < 1080 or long_side > 7680:
            out.append("a side outside 1080 to 7680 px")
    return out, (width, height, size)


def main(root, wanted):
    bad = 0
    present = sorted(d for d in os.listdir(root) if os.path.isdir(os.path.join(root, d)) and d != "logs")
    sets = wanted or present
    if not sets:
        print(f"no photo sets in {root}")
        return 1
    for name in sets:
        device = name.rsplit("-", 1)[0]
        if device not in MOTIFS:
            print(f"{name}: unknown set")
            bad += 1
            continue
        if name not in present:
            print(f"{name}: missing set")
            bad += 1
            continue
        for motif in MOTIFS[device]:
            path = os.path.join(root, name, motif + ".png")
            if not os.path.isfile(path):
                print(f"{name}/{motif}.png: missing")
                bad += 1
                continue
            found, info = faults(device, motif, path)
            if found:
                bad += 1
                print(f"{name}/{motif}.png: " + "; ".join(found))
            else:
                print(f"{name}/{motif}.png: {info[0]} x {info[1]}, {info[2] // 1024} KB, ok")
    print(f"{bad} faults" if bad else "all photos ok")
    return 1 if bad else 0


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("usage: store-screenshots-check.py <folder> [<set> ...]", file=sys.stderr)
        sys.exit(2)
    sys.exit(main(sys.argv[1], sys.argv[2:]))
