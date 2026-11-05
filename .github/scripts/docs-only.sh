#!/usr/bin/env bash
# Reads changed paths (one per line, relative to the repository root) on stdin and prints `true`
# when every one of them is documentation no check of the workflow reads, else `false`. The job
# "changes" of ci.yml feeds it the files of a pull request; `true` lets the heavy jobs skip
# (ADR 0013, addendum of 3 November 2026).
#
# Documentation here means docs/, Markdown files and release notes -- with three exceptions,
# because a check does read those:
#
#  - web/ and packages/: `prettier --check web packages` (root package.json, format:check) checks
#    their Markdown files too, so a README there can turn the web job red.
#  - deploy/: the image copies deploy/licenses/FFMPEG-NOTICE.md (deploy/Dockerfile).
#
# No input at all (an empty diff) prints `false`: what cannot be judged runs everything.
set -euo pipefail

seen=0
docs_only=true
while IFS= read -r path; do
  [ -n "$path" ] || continue
  seen=1
  case "$path" in
    web/* | packages/* | deploy/*) docs_only=false ;;
    docs/* | *.md | release-notes* | */release-notes*) ;;
    *) docs_only=false ;;
  esac
done

[ "$seen" = 1 ] || docs_only=false
echo "$docs_only"
