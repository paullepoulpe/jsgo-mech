#!/bin/bash
# reword.sh <commit> <msgfile>: gives <commit> (a first-parent ancestor of HEAD) a new message without touching the work tree; merges keep their other parents
set -e
cd /home/user/jm-upcast
target=$(git rev-parse $1); msg=$2
chain=$(git rev-list --reverse --first-parent ${target}^..HEAD)
parent=$(git rev-parse ${target}^)
for c in $chain; do
  tree=$(git rev-parse $c^{tree})
  others=""; for p in $(git rev-list --parents -n 1 $c | cut -d' ' -f3-); do others="$others -p $p"; done
  if [ $c = $target ]; then new=$(git commit-tree $tree -p $parent $others -F $msg); else new=$(git log -1 --format=%B $c | git commit-tree $tree -p $parent $others); fi
  parent=$new
done
git reset -q --soft $parent
git log --oneline | head -8 | cut -c1-110
