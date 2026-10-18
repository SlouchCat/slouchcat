#!/usr/bin/env python3
"""Placeholder library tree for scanner load tests over SMB/NFS (plan M0 step 6).

Creates EMPTY files in Plex/Jellyfin naming (Radarr/Sonarr style): one folder per
movie with optional trailer, extras and subtitle sidecars, and series with seasons
and episodes. The defaults match the reference collection of decision log row 3
(about 2'500 movies and 40'000 episodes; at least 45'000 files in 8'000 folders).

Names are deterministic for a given --seed. Standard library only; run with
`python3 -I testdata/generate-library-tree.py [--root DIR] ...`.
"""

from __future__ import annotations

import argparse
import os
import random
import sys
import time
from pathlib import Path

ADJECTIVES = [
    "Silent", "Crimson", "Hidden", "Last", "Golden", "Broken", "Endless", "Northern",
    "Wild", "Frozen", "Burning", "Lost", "Secret", "Electric", "Hollow", "Distant",
    "Iron", "Velvet", "Midnight", "Paper", "Glass", "Little", "Dark", "Bright",
    "Restless", "Quiet", "Savage", "Gentle", "Forgotten", "Rising", "Fallen", "Lucky",
    "Strange", "Sleeping", "Wandering", "Shining", "Grey", "Blue", "Scarlet", "Ancient",
    "Brave", "Hungry", "Lonely", "Perfect", "Final", "Eternal", "Bitter", "Sweet",
    "Grüne", "Schöne", "Süsse", "Café", "Señor", "Naïve", "Über",
]

NOUNS = [
    "River", "Mountain", "City", "Garden", "Shadow", "Empire", "Harbor", "Station",
    "Kingdom", "Island", "Forest", "Machine", "Dragon", "Signal", "Horizon", "Winter",
    "Summer", "Letter", "Promise", "Stranger", "Witness", "Echo", "Bridge", "Tower",
    "Journey", "Detective", "Orchard", "Lighthouse", "Circus", "Frontier", "Archive",
    "Voyage", "Heist", "Planet", "Comet", "Valley", "Desert", "Ocean", "Garage",
    "Hotel", "Diner", "Academy", "Republic", "Covenant", "Paradox", "Labyrinth",
    "Alpenglühen", "Bärgli", "Zürichsee", "Mädchen", "Øresund", "Fjord", "Château",
]

PATTERNS = [
    "The {a} {n}",
    "{a} {n}",
    "{n} of the {a} {n2}",
    "The {n} & the {n2}",
    "{n}",
    "{a} {n} {num}",
    "The {n}'s {n2}",
    "{n} - Part {roman}",
    "{a} {n}, {a2} {n2}",
]

ROMAN = ["I", "II", "III", "IV", "V"]
RESOLUTIONS = ["1080p", "2160p", "720p"]
EPISODE_WORDS = [
    "Pilot", "Arrival", "Reunion", "Departure", "Fallout", "Echoes", "The Offer",
    "Homecoming", "Crossroads", "Aftermath", "Lockdown", "Ghosts", "The Deal",
    "Night Shift", "Old Friends", "Thin Ice", "Blackout", "Revelations", "Exit",
    "Fresh Start", "Undertow", "Family Matters", "Wildfire", "Checkmate",
]


def make_title(rng: random.Random) -> str:
    pattern = rng.choice(PATTERNS)
    return pattern.format(
        a=rng.choice(ADJECTIVES),
        a2=rng.choice(ADJECTIVES),
        n=rng.choice(NOUNS),
        n2=rng.choice(NOUNS),
        num=rng.randint(2, 4),
        roman=rng.choice(ROMAN),
    )


def unique_names(rng: random.Random, count: int, years: tuple[int, int]) -> list[tuple[str, int]]:
    seen: set[str] = set()
    result: list[tuple[str, int]] = []
    while len(result) < count:
        title = make_title(rng)
        year = rng.randint(*years)
        key = f"{title} ({year})"
        if key in seen:
            continue
        seen.add(key)
        result.append((title, year))
    return result


class Tree:
    """Creates folders and empty files and counts them."""

    def __init__(self, root: Path) -> None:
        self.root = root
        self.files = 0
        self.dirs = 0
        self.kinds: dict[str, int] = {}

    def mkdir(self, path: Path) -> Path:
        os.mkdir(path)
        self.dirs += 1
        return path

    def touch(self, path: Path, kind: str) -> None:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o644)
        os.close(fd)
        self.files += 1
        self.kinds[kind] = self.kinds.get(kind, 0) + 1


def build_movies(tree: Tree, rng: random.Random, count: int) -> None:
    base = tree.mkdir(tree.root / "Movies")
    for title, year in unique_names(rng, count, (1950, 2025)):
        name = f"{title} ({year})"
        folder_name = name
        if rng.random() < 0.5:  # Radarr style with TMDB id
            folder_name = f"{name} {{tmdb-{rng.randint(10_000, 1_200_000)}}}"
        folder = tree.mkdir(base / folder_name)
        stem = name
        if rng.random() < 0.3:
            stem = f"{name} - {rng.choice(RESOLUTIONS)}"
        tree.touch(folder / f"{stem}.mkv", "movie")

        roll = rng.random()
        if roll < 0.45:  # trailer in a Trailers subfolder
            trailers = tree.mkdir(folder / "Trailers")
            tree.touch(trailers / f"{name} Official Trailer.mkv", "trailer")
        elif roll < 0.70:  # trailer as -trailer sidecar
            tree.touch(folder / f"{name}-trailer.mkv", "trailer")

        if rng.random() < 0.55:
            featurettes = tree.mkdir(folder / "Featurettes")
            for i in range(rng.randint(1, 3)):
                tree.touch(featurettes / f"Featurette {i + 1}.mkv", "extra")
        if rng.random() < 0.25:
            bts = tree.mkdir(folder / "Behind The Scenes")
            tree.touch(bts / "Making Of.mkv", "extra")

        if rng.random() < 0.6:
            tree.touch(folder / f"{stem}.de.srt", "subtitle")
        if rng.random() < 0.4:
            tree.touch(folder / f"{stem}.en.srt", "subtitle")
        if rng.random() < 0.1:
            tree.touch(folder / f"{stem}.de.forced.srt", "subtitle")


def build_series(tree: Tree, rng: random.Random, series: int, seasons: int, episodes: int) -> None:
    base = tree.mkdir(tree.root / "TV")
    for title, year in unique_names(rng, series, (1990, 2025)):
        show = f"{title} ({year})"
        folder_name = show
        if rng.random() < 0.5:  # Sonarr style with TVDB id
            folder_name = f"{show} {{tvdb-{rng.randint(70_000, 450_000)}}}"
        show_dir = tree.mkdir(base / folder_name)
        season_numbers = list(range(1, seasons + 1))
        specials = rng.random() < 0.4
        if specials:
            season_numbers.insert(0, 0)
        for season in season_numbers:
            season_dir = tree.mkdir(show_dir / ("Specials" if season == 0 else f"Season {season:02d}"))
            count = rng.randint(1, 3) if season == 0 else episodes
            for ep in range(1, count + 1):
                ep_title = rng.choice(EPISODE_WORDS)
                if rng.random() < 0.3:
                    ep_title = f"{ep_title} ({rng.choice(ROMAN[:2])})"
                stem = f"{show} - S{season:02d}E{ep:02d} - {ep_title}"
                tree.touch(season_dir / f"{stem}.mkv", "special" if season == 0 else "episode")
                if rng.random() < 0.25:
                    tree.touch(season_dir / f"{stem}.en.srt", "subtitle")


def main(argv: list[str]) -> int:
    default_root = Path(__file__).resolve().parent / "media" / "placeholder-tree"
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", type=Path, default=default_root, help=f"target folder (default {default_root})")
    parser.add_argument("--movies", type=int, default=2500, help="number of movies (default 2500)")
    parser.add_argument("--series", type=int, default=400, help="number of series (default 400)")
    parser.add_argument("--seasons", type=int, default=5, help="seasons per series (default 5)")
    parser.add_argument("--episodes", type=int, default=20, help="episodes per season (default 20)")
    parser.add_argument("--seed", type=int, default=1, help="random seed for deterministic names (default 1)")
    args = parser.parse_args(argv)

    for name in ("movies", "series", "seasons", "episodes"):
        if getattr(args, name) < 0:
            parser.error(f"--{name} must be >= 0")

    root: Path = args.root
    if root.exists() and any(root.iterdir()):
        print(f"error: {root} exists and is not empty; remove it first (task testdata:clean)", file=sys.stderr)
        return 1
    root.mkdir(parents=True, exist_ok=True)

    started = time.monotonic()
    tree = Tree(root)
    build_movies(tree, random.Random(f"movies-{args.seed}"), args.movies)
    build_series(tree, random.Random(f"series-{args.seed}"), args.series, args.seasons, args.episodes)
    elapsed = time.monotonic() - started

    print(f"root:    {root}")
    print(f"files:   {tree.files}")
    print(f"folders: {tree.dirs} (below root)")
    for kind in sorted(tree.kinds):
        print(f"  {kind:<9} {tree.kinds[kind]}")
    print(f"time:    {elapsed:.2f} s")
    ok = tree.files >= 45_000 and tree.dirs >= 8_000
    print(f"reference size (>= 45000 files, >= 8000 folders): {'reached' if ok else 'not reached'}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
