#!/bin/bash
# usage: commit.sh msgfile
cd /home/user/work/p05e
git add -A java tools
printf '\nCo-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>\nClaude-Session: https://claude.ai/code/session_0162Uc5JieSYhVmqiHv9WVbQ\n' >> "$1"
git commit -q -F "$1" && git log --oneline | head -1 && git diff --stat HEAD~1 | tail -1
