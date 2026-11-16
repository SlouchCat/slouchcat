#!/usr/bin/env python3
"""Joins the function inventory with the reviewers' findings.

    python3 -I docs/design/admin-redesign/abdeckung/zusammenfuehren.py

Reads ../functional-inventory.csv and the four part files next to this script (medien1,
medien2-nutzer, startseite-system, automatisierung-shell) and writes ../funktionsabdeckung.csv: one
row per function with where it stood, where it stands now, the test that proves it and its status.
Prints the counts per page and status; stops when a row of the inventory has no finding.
"""

import collections
import csv
from pathlib import Path

HERE = Path(__file__).resolve().parent
PARTS = ["medien1", "medien2-nutzer", "startseite-system", "automatisierung-shell"]


def read(path: Path) -> list[dict[str, str]]:
    with path.open(newline="", encoding="utf-8") as handle:
        return list(csv.DictReader(handle, delimiter=";"))


def main() -> None:
    inventory = read(HERE.parent / "functional-inventory.csv")
    findings: dict[str, dict[str, str]] = {}
    for part in PARTS:
        for row in read(HERE / f"{part}.csv"):
            if row["ID"] in findings:
                raise SystemExit(f"{row['ID']} steht zweimal in den Befunden")
            findings[row["ID"]] = row
    missing = [row["ID"] for row in inventory if row["ID"] not in findings]
    if missing:
        raise SystemExit(f"ohne Befund: {', '.join(missing)}")

    fields = [
        "ID",
        "Seite",
        "Funktion",
        "Typ",
        "Route und Query",
        "Komponente vorher",
        "API",
        "Neue Stelle",
        "Aenderung",
        "Beleg",
        "Status",
        "Bemerkung",
    ]
    counts: dict[str, collections.Counter[str]] = collections.defaultdict(collections.Counter)
    with (HERE.parent / "funktionsabdeckung.csv").open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fields, delimiter=";")
        writer.writeheader()
        for row in inventory:
            finding = findings[row["ID"]]
            counts[row["ID"].split("-")[0]][finding["Status"]] += 1
            writer.writerow(
                {
                    "ID": row["ID"],
                    "Seite": row["Seite"],
                    "Funktion": row["Funktion"],
                    "Typ": row["Typ"],
                    "Route und Query": row["Route und Query"],
                    "Komponente vorher": row["Komponente"],
                    "API": row["API"],
                    "Neue Stelle": finding["Neue Stelle"],
                    "Aenderung": finding["Aenderung"],
                    "Beleg": finding["Beleg"],
                    "Status": finding["Status"],
                    "Bemerkung": finding["Bemerkung"],
                }
            )
    states = sorted({state for counter in counts.values() for state in counter})
    print("| Seite | Zeilen | " + " | ".join(states) + " |")
    print("|---|---:|" + "---:|" * len(states))
    total: collections.Counter[str] = collections.Counter()
    for prefix, counter in counts.items():
        total += counter
        cells = " | ".join(str(counter.get(state, 0)) for state in states)
        print(f"| {prefix} | {sum(counter.values())} | {cells} |")
    cells = " | ".join(str(total.get(state, 0)) for state in states)
    print(f"| **zusammen** | **{sum(total.values())}** | {cells} |")


if __name__ == "__main__":
    main()
