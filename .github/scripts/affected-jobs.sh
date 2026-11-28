#!/usr/bin/env bash
# Reads the changed paths of a pull request (one per line, relative to the repository root) on
# stdin and prints, for every job group of ci.yml, `run_<group>=true` or `run_<group>=false`: which
# jobs a change can turn red (docs/perf/ci-optimierungsplan.md, M9; ADR 0013, addendum M9). The job
# "changes" writes the lines to its outputs: on a pull request from its paths; a push to main, the
# weekly run and a run by hand call it with no input, which runs everything.
#
# Groups and their jobs:
#   server          server-lint, server-groups, server-test
#   relay           relay
#   android         android (both halves)
#   android_server  android-contract, android-emulator (the apps against bin/slouchcat and relay)
#   web             web (both halves), storybook
#   e2e             build, e2e
#   image           image
#
# A path is mapped by what reads it, transitively: the generators (api/openapi.yaml into the server,
# the TypeScript client and the Kotlin client; web/src/locales into the Android strings;
# packages/design-tokens into CSS, TypeScript and Kotlin), test data read across modules, and the
# binaries the e2e suite and the Android contract tests start. affected-jobs.test.sh holds the map
# against the files the tests and builds actually read; a new reading across modules turns it red.
#
# Fail-safe: a path this script does not know, the workflows and their scripts, the task runner's
# files, the tool versions and the lock files run everything. Documentation no check reads
# (.github/scripts/docs-only.sh) runs nothing. No input at all runs everything.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
groups=(server relay android android_server web e2e image)
declare -A on=()
add() { local g; for g in "$@"; do on[$g]=1; done; }

seen=0
while IFS= read -r path; do
  [ -n "$path" ] || continue
  seen=1
  # Documentation no check reads: the same rule as for a whole pull request, path by path.
  if [ "$("$here/docs-only.sh" <<<"$path")" = true ]; then
    continue
  fi
  case "$path" in
    # The checks themselves, the tools and the contract: everything.
    .github/* | Taskfile.yml | */Taskfile.yml | .tool-versions | pnpm-lock.yaml | pnpm-workspace.yaml | \
      package.json | tsconfig*.json | .prettierrc* | .prettierignore | api/* | scripts/heavy-slot*)
      add "${groups[@]}" ;;
    # golangci-lint for relay comes from server/go.mod (relay/Taskfile.yml).
    server/go.mod | server/go.sum) add server relay e2e image android_server ;;
    # The Android contract and emulator jobs build and start bin/slouchcat (android/Taskfile.yml).
    server/*) add server e2e image android_server ;;
    # ... and the relay services (android/Taskfile.yml, :relay:build).
    relay/*) add relay android_server ;;
    deploy/relay/*) add relay ;;
    # The launcher icons are held to the logo by a web test as well (web/src/ui/logoMark.test.ts).
    android/app-*/src/main/res/drawable/*) add android android_server web ;;
    android/* | scripts/android-*) add android android_server ;;
    # The Android strings are generated from the web locales (android/core/strings).
    web/src/locales/*) add web e2e image android android_server ;;
    # The server's track choice reads the player's labels (internal/trackpick, languages_test.go).
    # pin.ts derives the offline PIN's verifier the server checks (internal/auth, TestPBKDF2Vectors
    # names it); no server test reads the file, the edge is kept as a precaution.
    web/src/features/player/labels.ts | web/src/features/offline/pin.ts) add web e2e image server ;;
    # The mock servers of the e2e suite; the Android contract and emulator jobs start them too
    # (scripts/android-helpers.sh: Seerr, TMDB, radio; the Seerr mock imports the Arr mock). A
    # pattern, not a list: a mock one of them imports (a sibling file, which the map test cannot
    # see) is covered too, at the price of start-server.mjs running the Android jobs as well.
    web/e2e/*-server.mjs) add web e2e image android_server ;;
    # The app icon; the Android launcher test reads it (app-mobile, LauncherIconTest).
    web/public/icon.svg) add web e2e image android ;;
    web/*) add web e2e image ;;
    # The tokens are generated into Kotlin as well (android/core/design).
    packages/design-tokens/*) add web e2e image android android_server ;;
    packages/*) add web e2e image ;;
    # Read by the server, relay and Android tests and by the e2e suite (media, feeds, radio).
    testdata/*) add server relay android android_server e2e ;;
    # The image, and the updater's handling of the deploy files.
    deploy/*) add image server ;;
    # The images of the server and of the relay services copy it (deploy/Dockerfile,
    # deploy/relay/*/Dockerfile).
    LICENSE) add image server relay ;;
    # The store graphics are held to the logo by a web test (web/src/ui/logoMark.test.ts);
    # .github/scripts/docs-only.sh does not count them as documentation.
    docs/store/grafiken/*) add web ;;
    scripts/i18n/*) add web ;;
    # Run by task ci:workflows in the job "server (generators, linters)".
    scripts/ci-failures*) add server ;;
    *) add "${groups[@]}" ;;
  esac
done

[ "$seen" = 1 ] || add "${groups[@]}"
for g in "${groups[@]}"; do
  if [ -n "${on[$g]:-}" ]; then echo "run_$g=true"; else echo "run_$g=false"; fi
done
