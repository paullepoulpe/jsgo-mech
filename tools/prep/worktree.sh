#!/usr/bin/env bash
# Makes a separate checkout for working on one prep step, sharing the
# downloaded jars with this one:  tools/prep/worktree.sh <dir> <branch>
set -euo pipefail
cd "$(dirname "$0")/../.."
root=$PWD
git worktree add -q -B "$2" "$1" HEAD
mkdir -p "$1/.cache"
for f in closure-src closure-src.jar closure-unshaded.jar deps e2e; do
  [[ -e "$root/.cache/$f" ]] && ln -sfn "$root/.cache/$f" "$1/.cache/$f"
done
echo "$1 on branch $2"
