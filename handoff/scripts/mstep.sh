#!/usr/bin/env bash
# mstep.sh <label> <source sha> <apply command...>: apply on the merge tree, gofmt, build, gate, commit, save binary.
set -uo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cd /home/user/jm-merge
label=$1; src=$2; shift 2
out=$(eval "$@" 2>&1); rc=$?
echo "$out" | tail -5
[[ $rc == 0 ]] || { echo "APPLY FAILED $label"; exit 1; }
(cd go && gofmt -w closure/*.go && go build -o /dev/null ./closure) || { echo "BUILD FAILED $label"; exit 1; }
log=$S/merge-$label.log
flock $S/gate.lock bash tools/prune/verify.sh > $log 2>&1
if [[ $(grep -c ": MATCH" $log) != 4 ]] || ! grep -q "^ok" $log; then tail -12 $log; echo "GATE FAILED $label"; exit 1; fi
cp .cache/prune/mech $S/proto/m_$label
if [[ -n $(git status --porcelain go tools) ]]; then
  git add -A go/closure tools/postflip; git add -f go/closure/g_*.go 2>/dev/null
  subj=$(git log -1 --format=%s $src); body=$(git log -1 --format=%b $src | grep -v "^Co-Authored-By\|^Claude-Session")
  printf '%s\n\nRe-applied on the merged tree (postflip5). Tool output here:\n%s\n\nGate: runtime tests ok, four projects MATCH.\n\nFrom %s; that commit says why the step keeps Java behaviour:\n\n%s\n\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>\nClaude-Session: https://claude.ai/code/session_0162Uc5JieSYhVmqiHv9WVbQ\n' "$subj" "$(echo "$out" | grep -v '^\s*$' | tail -4)" "$src" "$body" > $S/msg.txt
  git commit -q -F $S/msg.txt
fi
git log --oneline -1; grep ": MATCH" $log | cut -c1-60
