#!/usr/bin/env bash
# Replays the whole post-flip half of the plan: from the flip commit (the frozen j2go output)
# to the post-flip tip, by running every post-flip tool in the order of the history and
# applying the handwritten runtime and the manual edits as patches. Nothing else touches go/.
#
#   tools/postflip/replay-all.sh [-w DIR] [-base COMMIT] [-to COMMIT] [-noverify] [-trim] [-list]
#                                [-steps FILE] [-overlay DIR] [-toolfix SCRIPT] [-check CMD]
#                                [-verifyin RANGE]
#
#   -w DIR       the replay worktree (default ../<repo>-replay-w); made with `git worktree add`
#                at the base commit when it does not exist. A rerun resumes after the last
#                replayed commit (each replayed commit is a commit of the worktree whose
#                message ends "Replays: <commit>").
#   -base C      the flip commit to start from (default 01f4f269). On a re-flip, give the new
#                flip commit and -noverify.
#   -to C        stop after replaying commit C.
#   -noverify    do not compare go/ with the original commit after each step.
#   -list        print the counts of steps.txt and exit.
#   -steps FILE  the list of steps to follow instead of steps.txt (a re-flip's edited copy:
#                steps dropped as no-ops, tools of an adapted commit).
#   -overlay DIR a directory of patches that replace the patches of the same name in patches/
#                (a manual or runtime patch regenerated for the re-flip's text).
#   -toolfix SCRIPT  a script run at the worktree root after each step's tools are checked out
#                and before they run (a re-flip's fixes to the tools of every step). Idempotent.
#   -check CMD   a command run at the worktree root after each commit's lines (a type check of
#                go/closure, say); the replay stops when it fails.
#   -verifyin RANGE  compare go/ only for the commits of the git range RANGE (a re-flip:
#                `<new flip>..<tip>`, the steps made on the re-flip's own branch); the steps of
#                the old history, replayed on the new flip's text, cannot match their commits and
#                are not compared. Implies the comparison for those commits even with -noverify.
#   -trim        after each step, delete the files over 8 MB from $GOCACHE (the steps that
#                build go/closure leave about 300 MB each). Only with a GOCACHE of your own.
#
# steps.txt lists, per original commit and in the order of the history, what made its go/ tree
# from its parent's:
#   <commit> tools <rev>      the tools the commit's `run` lines use: tools/postflip of <rev>
#                             (default: of the commit itself, i.e. the tools as they were when
#                             the step was made)
#   <commit> patch <file>     git apply patches/<file>: -rt (handwritten runtime, anything in
#                             go/ but go/closure/g_*.go), -manual (a "postflip (manual)" commit's
#                             edit of generated files), -residue (what a step commit holds beyond
#                             what its tool run writes: a provenance gap, listed in POSTFLIP.md)
#   <commit> run <command>    a shell command run at the worktree root, with the helpers below
# After the last line of a commit the worktree's go/ must be that commit's go/, byte for byte
# (unless -noverify), and is committed.
#
# Helpers for `run` lines:
#   t TOOL ARGS...     (cd tools/postflip && go run ./TOOL ARGS... ../../go/closure)
#   f                  gofmt -w go/closure/g_*.go        fa: gofmt -w go/closure/*.go
#   restore C          go/ as it was after replaying original commit C
#   retool C OLD NEW CMD...
#                      a tool change applied to code the tool's earlier version already wrote:
#                      runs CMD on go/ as of original commit C twice, with tools/postflip of
#                      OLD and of NEW, and applies the difference of the two outputs to go/.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
steps=$here/steps.txt
patches=$here/patches
W=$(dirname "$repo")/$(basename "$repo")-replay-w
base=01f4f269
to=
verify=1
trim=0
overlay=
toolfix=
check=
verifyin=
while [[ $# -gt 0 ]]; do
  case $1 in
    -w) W=$2; shift 2 ;;
    -base) base=$2; shift 2 ;;
    -to) to=$2; shift 2 ;;
    -noverify) verify=0; shift ;;
    -trim) trim=1; shift ;;
    -steps) steps=$(realpath "$2"); shift 2 ;;
    -overlay) overlay=$(realpath "$2"); shift 2 ;;
    -toolfix) toolfix=$(realpath "$2"); shift 2 ;;
    -check) check=$2; shift 2 ;;
    -verifyin) verifyin=$2; shift 2 ;;
    -list)
      awk '!/^#/ && NF {
             if ($2 == "run") run[$1] = 1
             if ($2 == "patch") { if ($3 ~ /-rt\.patch$/) rt++; else if ($3 ~ /-manual\.patch$/) man++; else res++ }
             c[$1] = 1 }
           END { for (k in c) n++; for (k in run) r++
                 printf "commits replayed: %d; with tool runs: %d; runtime patches: %d; manual patches: %d; residue patches: %d\n", n, r, rt, man, res }' "$steps"
      exit 0 ;;
    *) echo "usage: $0 [-w DIR] [-base COMMIT] [-to COMMIT] [-noverify] [-trim] [-list] [-steps FILE] [-overlay DIR] [-toolfix SCRIPT] [-check CMD] [-verifyin RANGE]" >&2; exit 2 ;;
  esac
done

if [[ ! -d $W ]]; then git -C "$repo" worktree add -q --detach "$W" "$base"; fi
cd "$W"
git reset -q --hard
git clean -q -fdx go tools/postflip
last=$(git log -1 --format=%B | sed -n 's/^Replays: //p')
log=$W/.replay.log
declare -A inrange=()
if [[ -n $verifyin ]]; then
  for r in $(git rev-list "$verifyin"); do inrange[$r]=1; done
fi
nverified=0

# The helpers of the run lines (a string, so that each command runs in a fresh shell).
prelude='
set -euo pipefail
t() { local x=$1; shift; (cd tools/postflip && go run ./$x "$@" ../../go/closure); }
f() { gofmt -w go/closure/g_*.go; }
fa() { gofmt -w go/closure/*.go; }
replayed() { local r; r=$(git log --format=%H --grep="^Replays: $1" -1); [[ -n $r ]] || { echo "no replay of $1" >&2; exit 1; }; echo "$r"; }
restore() { local r; r=$(replayed "$1"); git rm -rqf --cached go; rm -rf go; git checkout -q "$r" -- go; }
retool() {
  local c=$1 old=$2 new=$3 r d; shift 3
  r=$(replayed "$c"); d=$(mktemp -d)
  for v in old new; do
    mkdir "$d/$v"; git archive "$r" go | tar -x -C "$d/$v"
    git archive "${!v}" tools/postflip | tar -x -C "$d/$v"
    (cd "$d/$v" && git init -q . && git add -A -f go && eval "$*")
    rm -rf "$d/$v/.git" "$d/$v/tools"
  done
  (cd "$d" && diff -ruN old/go new/go || true) > "$d/retool.patch"
  patch -s -p1 --no-backup-if-mismatch < "$d/retool.patch"
  rm -rf "$d"
}
'

settools() { # tools/postflip as of commit $1
  local want; want=$(git rev-parse "$1:tools/postflip")
  if [[ $(git write-tree --prefix=tools/postflip/ 2>/dev/null || true) != "$want" ]]; then
    git rm -rqf --cached tools/postflip 2>/dev/null || true
    rm -rf tools/postflip
    git checkout -q "$1" -- tools/postflip
  fi
}

finish() { # $1: the original commit whose lines are done
  local c=$1
  if [[ -n $check ]] && ! bash -c "$check" >> "$log" 2>&1; then
    echo "CHECK FAILED after $c ($(git log -1 --format=%s "$c")): $check (log: $log)" >&2; tail -8 "$log" >&2; exit 1
  fi
  git add -A -f go tools/postflip
  local cmp=$verify full
  if [[ -n $verifyin ]]; then
    full=$(git rev-parse "$c^{commit}")
    if [[ -n ${inrange[$full]:-} ]]; then cmp=1; else cmp=0; fi
  fi
  [[ $cmp == 1 ]] && nverified=$((nverified + 1))
  if [[ $cmp == 1 ]] && ! git diff --cached --quiet "$c" -- go; then
    echo "MISMATCH after replaying $c ($(git log -1 --format=%s "$c")):" >&2
    git diff --cached --stat "$c" -- go | tail -5 >&2
    echo "the worktree $W is left as the step wrote it; log: $log" >&2
    exit 1
  fi
  git commit -q --allow-empty -m "replay: $(git log -1 --format=%s "$c")" -m "Replays: $c"
  n=$((n + 1))
  if [[ $trim == 1 && -n ${GOCACHE:-} ]]; then find "$GOCACHE" -type f -size +8M -delete 2>/dev/null || true; fi
  echo "ok $c $([[ $cmp == 1 ]] && echo '(go/ identical)' || echo '(not compared)') $(git log -1 --format=%s "$c" | cut -c1-80)"
}

cur= ; n=0; skipping=0
[[ -n $last ]] && skipping=1 && echo "resuming after $last"
toolsof=
while read -r c action arg; do
  [[ -z $c || $c == \#* ]] && continue
  if [[ $c != "$cur" ]]; then
    if [[ -n $cur && $skipping == 0 ]]; then
      finish "$cur"
      if [[ $cur == "$to" ]]; then echo "stopped after $to"; exit 0; fi
    fi
    if [[ $skipping == 1 && $cur == "$last" ]]; then skipping=0; fi
    cur=$c; toolsof=$c
  fi
  [[ $skipping == 1 ]] && continue
  case $action in
    tools) toolsof=$arg ;;
    patch)
      pf=$patches/$arg
      [[ -n $overlay && -f $overlay/$arg ]] && pf=$overlay/$arg
      if ! git apply --whitespace=nowarn "$pf"; then echo "FAILED at $c: patch $pf" >&2; exit 1; fi ;;
    run)
      settools "$toolsof"
      if [[ -n $toolfix ]]; then bash "$toolfix" >> "$log" 2>&1 || { echo "FAILED at $c: toolfix" >&2; tail -5 "$log" >&2; exit 1; }; fi
      echo "=== $c: $arg" >> "$log"
      if ! bash -c "$prelude$arg" >> "$log" 2>&1; then
        echo "FAILED at $c: $arg (log: $log)" >&2; tail -20 "$log" >&2; exit 1
      fi ;;
    *) echo "steps.txt: unknown action '$action' at $c" >&2; exit 2 ;;
  esac
done < "$steps"
if [[ -n $cur && $skipping == 0 ]]; then finish "$cur"; fi
if [[ $skipping == 1 && $cur != "$last" ]]; then echo "the worktree's last replayed commit $last is not in steps.txt" >&2; exit 1; fi
echo "replayed $n commits ($nverified compared with their commit's go/); go/ tree: $(git rev-parse HEAD:go); go/closure tree: $(git rev-parse HEAD:go/closure)"
