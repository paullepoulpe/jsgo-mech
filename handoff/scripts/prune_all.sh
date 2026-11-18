#!/usr/bin/env bash
# One full prune pass of the runtime: root all generated code, delete dead runtime functions,
# restore anything still referenced, fix tests.
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cd /home/user/jsgo-mech/go || exit 1
bash ../tools/j2go/regen.sh closure >/dev/null 2>&1
go build ./... || { echo "does not build"; exit 1; }
$S/prune roots closure
printf 'package main\n\nimport "jsgomech/closure"\n\nvar _ = closure.Roots_\n' > cmd/mech/roots_tmp.go
~/go/bin/deadcode ./cmd/mech > $S/dead.txt 2>&1
echo "dead: $(grep -c closure/rt_ $S/dead.txt)"
$S/prune closure < $S/dead.txt
rm -f closure/rt_zroots.go cmd/mech/roots_tmp.go
~/go/bin/goimports -w closure/rt_*.go
$S/restore_loop.sh | tail -2
go build ./... 2>&1 | grep -v "^#" | head -5
