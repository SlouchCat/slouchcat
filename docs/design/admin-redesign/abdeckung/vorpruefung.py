#!/usr/bin/env python3
"""Mechanical pre-check of the function inventory against the redesign branch.

For every row of ../functional-inventory.csv it resolves the component files and the tests the row
names, and says whether the code changed since the inventory's base commit and whether each named
test is still there. Rows whose code is untouched and whose tests all still exist need no reading;
everything else goes to the independent check (the reviewers fill ../funktionsabdeckung.csv).

    python3 -I docs/design/admin-redesign/abdeckung/vorpruefung.py [BASE] [HEAD]

Writes vorpruefung.csv next to this file.
"""

import csv
import re
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO = Path(
    subprocess.check_output(["git", "rev-parse", "--show-toplevel"], cwd=HERE, text=True).strip()
)
BASE = sys.argv[1] if len(sys.argv) > 1 else "55b72c9a"
HEAD = sys.argv[2] if len(sys.argv) > 2 else "HEAD"

# Where the inventory's short paths may start.
ROOTS = ["", "web/", "web/src/", "web/src/features/admin/", "web/src/features/", "web/src/ui/"]
FILE = re.compile(r"[\w@./-]+\.(?:tsx|ts|css|json|mjs|yaml|html|md)\b")
# Words that mark behaviour the frame changed although the page's own file may be untouched.
FRAME_WORDS = re.compile(
    r"Hilfe|Einleitung|Seitenleiste|Leiste der Verwaltung|Navigation|Handy-Blatt|Blatt «|Brotkrumen",
    re.IGNORECASE,
)


def git(*args: str) -> str:
    return subprocess.run(["git", *args], cwd=REPO, text=True, capture_output=True).stdout


def files_at(rev: str) -> set[str]:
    return set(git("ls-tree", "-r", "--name-only", rev).splitlines())


BASE_FILES = files_at(BASE)
HEAD_FILES = files_at(HEAD)
CHANGED = set(git("diff", "--name-only", BASE, HEAD).splitlines())
_content: dict[str, str] = {}


def content(path: str) -> str:
    if path not in _content:
        _content[path] = git("show", f"{HEAD}:{path}")
    return _content[path]


def resolve(token: str, known: set[str]) -> str | None:
    token = token.lstrip("./")
    for root in ROOTS:
        if root + token in known:
            return root + token
    # A bare file name ("StationsTab.tsx") that exists exactly once.
    hits = [f for f in known if f.endswith("/" + token)]
    return hits[0] if len(hits) == 1 else None


def quoted(text: str) -> list[str]:
    """The texts in «…», outermost level, nested guillemets kept."""
    out, depth, start = [], 0, 0
    for i, ch in enumerate(text):
        if ch == "«":
            if depth == 0:
                start = i + 1
            depth += 1
        elif ch == "»" and depth:
            depth -= 1
            if depth == 0:
                out.append(text[start:i])
    return out


def named(name: str, text: str) -> bool:
    """Whether a test of this name is in the file. The inventory shortens long names with «…» and
    quotes names a loop builds from a path («/admin/x is not wider than the phone»)."""
    parts = [part.strip() for part in name.split("…") if len(part.strip()) >= 8]
    if parts and all(part in text for part in parts):
        return True
    words = name.split(" ", 1)
    return len(words) == 2 and words[0].startswith("/") and words[1] in text


def check_row(row: list[str]) -> dict[str, str]:
    ident, page, component, function, tests, note = row[0], row[2], row[4], row[5], row[10], row[12]

    code_files, code_state = [], []
    for token in dict.fromkeys(FILE.findall(component)):
        at_base = resolve(token, BASE_FILES)
        if at_base is None:
            code_state.append(f"{token}: nicht aufgelöst")
            continue
        code_files.append(at_base)
        if at_base not in HEAD_FILES:
            code_state.append(f"{at_base}: entfernt")
        elif at_base in CHANGED:
            code_state.append(f"{at_base}: geändert")
    code = "unverändert" if not code_state else "; ".join(code_state)
    if not code_files and not code_state:
        code = "keine Datei genannt"

    test_state, test_count, test_found = [], 0, 0
    if "kein Test" not in tests:
        for part in tests.split(";"):
            names = quoted(part)
            files = [resolve(t, HEAD_FILES | BASE_FILES) for t in FILE.findall(part)]
            files = [f for f in files if f]
            for path in files:
                if path not in HEAD_FILES:
                    test_state.append(f"{path}: Datei entfernt")
            live = [f for f in files if f in HEAD_FILES]
            for name in names:
                test_count += 1
                if any(named(name, content(path)) for path in live):
                    test_found += 1
                else:
                    where = ", ".join(live) or "keine Datei"
                    test_state.append(f"«{name[:70]}» fehlt in {where}")
    tests_out = (
        "kein Test"
        if "kein Test" in tests
        else f"{test_found}/{test_count} Namen gefunden" + ("; " + "; ".join(test_state) if test_state else "")
    )

    frame = bool(FRAME_WORDS.search(function) or FRAME_WORDS.search(note))
    reasons = []
    if code != "unverändert":
        reasons.append("Code")
    if test_state:
        reasons.append("Tests")
    if frame:
        reasons.append("Rahmen")
    return {
        "ID": ident,
        "Seite": page,
        "Funktion": function,
        "Code seit Basis": code,
        "Tests": tests_out,
        "Vorprüfung": "prüfen: " + ", ".join(reasons) if reasons else "unverändert",
    }


def main() -> None:
    source = HERE.parent / "functional-inventory.csv"
    with source.open(newline="", encoding="utf-8") as handle:
        rows = list(csv.reader(handle, delimiter=";"))[1:]
    checked = [check_row(row) for row in rows]
    target = HERE / "vorpruefung.csv"
    with target.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=list(checked[0]), delimiter=";")
        writer.writeheader()
        writer.writerows(checked)
    todo = [c for c in checked if c["Vorprüfung"] != "unverändert"]
    by_prefix: dict[str, list[int]] = {}
    for c in checked:
        pair = by_prefix.setdefault(c["ID"].split("-")[0], [0, 0])
        pair[0] += 1
        pair[1] += c["Vorprüfung"] != "unverändert"
    print(f"{len(checked)} Zeilen, {len(todo)} zu prüfen, Basis {BASE}, Stand {git('rev-parse', '--short', HEAD).strip()}")
    for prefix, (total, open_) in by_prefix.items():
        print(f"  {prefix}: {open_} von {total} zu prüfen")


if __name__ == "__main__":
    main()
