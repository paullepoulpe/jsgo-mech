#!/usr/bin/env bash
# mstep2.sh <label> <source sha> <apply...>: apply, gofmt, build the binary, commit (gate per group, see gate.sh)
set -uo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cd /home/user/jm-merge; label=$1; src=$2; shift 2
out=$(eval "$@" 2>&1); rc=$?; echo "$out" | tail -3
if [[ $rc != 0 ]]; then
  if [[ -n $(git diff --name-only --diff-filter=U) ]]; then echo "CONFLICT in $label: taking the cherry-picked side"; python3 $S/theirs.py || { echo "APPLY FAILED $label"; exit 1; }
  else echo "APPLY FAILED $label"; exit 1; fi
fi
python3 $S/enumfix.py
(cd go && gofmt -w closure/*.go && go build -o $S/proto/m_$label ./cmd/mech) || { echo "BUILD FAILED $label"; exit 1; }
if [[ -n $(git status --porcelain go tools) ]]; then
  git add -A go/closure tools/postflip; git add -f go/closure/g_*.go 2>/dev/null
  subj=$(git log -1 --format=%s $src); body=$(git log -1 --format=%b $src | grep -v "^Co-Authored-By\|^Claude-Session")
  printf '%s\n\nRe-applied on the merged tree (postflip5); gated with its group (see the group'"'"'s last commit). Tool output here:\n%s\n\nFrom %s:\n\n%s\n\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>\nClaude-Session: https://claude.ai/code/session_0162Uc5JieSYhVmqiHv9WVbQ\n' "$subj" "$(echo "$out" | grep -v '^\s*$' | tail -4)" "$src" "$body" > $S/msg.txt
  git commit -q -F $S/msg.txt
fi
git log --oneline -1 | cut -c1-100
