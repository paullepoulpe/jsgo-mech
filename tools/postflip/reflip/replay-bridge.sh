#!/usr/bin/env bash
# Replays the bridge steps (C1...) of reflip/steps.txt as one chain, in a scratch worktree:
# starting from the tree before C1, each step's patch is applied or its command is run with
# the tools of that step's commit, and the resulting go/ must be that commit's go/, byte for
# byte; the chain carries its own tree forward (no step starts from a checked-out tree). The
# last tree is built. Prints one line per step and "CHAIN OK <n> steps" at the end.
#
# usage: tools/postflip/reflip/replay-bridge.sh <scratch dir>   (run from the repository root)
set -euo pipefail
root=$(pwd); T=$1
steps=$root/tools/postflip/reflip/steps.txt; patches=$root/tools/postflip/reflip/patches
t() { local x=$1; shift; (cd tools/postflip && go run ./$x "$@" ../../go/closure) >/dev/null 2>&1; }
f() { gofmt -w go/closure/g_*.go; }
first=$(awk '/^# C1\./{getline; print $1; exit}' "$steps")
rm -rf "$T"; git worktree add -q --detach "$T" "$first~1"
cd "$T"
n=0; label=""
while IFS= read -r line; do
  case $line in
    "# C"*) label=${line#\# }; label=${label%%.*}; continue ;;
    "#"*|"") continue ;;
  esac
  [[ -n $label ]] || continue
  c=${line%% *}; rest=${line#* }; kind=${rest%% *}; arg=${rest#* }
  git checkout -q "$c" -- tools
  case $kind in
    patch) git apply "$patches/$arg" ;;
    run) eval "$arg" ;;
    *) echo "$label: unknown step kind $kind" >&2; exit 1 ;;
  esac
  git add -A -f go
  # g_zconv_jfam.go (written by C2) was kept out of the commits before 80f88eea by .gitignore
  # (see the note in steps.txt): against those it is left out of the comparison.
  spec=(go)
  for g in go/closure/g_zconv_jfam.go go/closure/g_zconv_jfam_keys.go; do
    if [[ -f $g ]] && ! git cat-file -e "$c:$g" 2>/dev/null; then spec+=(":(exclude)$g"); fi
  done
  if ! git diff --cached --quiet "$c" -- "${spec[@]}"; then
    echo "$label $c: the replayed go/ differs from the commit's:" >&2
    git diff --cached --stat "$c" -- "${spec[@]}" | tail -5 >&2
    exit 1
  fi
  n=$((n+1)); echo "$label $c ok"; label=""
done < <(sed -n '/^# C1\./,$p' "$steps")
(cd go && go build ./closure)
echo "CHAIN OK $n steps, last tree builds"
