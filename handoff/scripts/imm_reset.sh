#!/bin/bash
# Reset generated files of the worktree to HEAD (keep runtime and tool changes).
cd /home/user/jm-r7-typed
files=$(git diff --name-only -- 'go/closure/g_*.go')
[ -n "$files" ] && git checkout -- $files
git ls-files --others -- 'go/closure/g_*.go' | xargs -r rm -f
git status --short
