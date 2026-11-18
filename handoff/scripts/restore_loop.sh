#!/usr/bin/env bash
# Restores pruned runtime functions that are still referenced, until the build is clean.
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cd /home/user/jsgo-mech/go
for round in $(seq 1 25); do
  bash ../tools/j2go/regen.sh closure >/dev/null 2>&1
  names=$( { grep "j2go stub" closure/g_zstubs.go | sed 's/.*j2go stub: //; s/").*//' | grep -v "^Abstract";
    go build -gcflags=-e ./... 2>&1 | grep -v "^#" | grep -oE "undefined: [A-Za-z_0-9]+|type \*?[A-Za-z_0-9]+ has no field or method [A-Za-z_0-9]+" | sed -E 's/undefined: //; s/type \*?([A-Za-z_0-9]+) has no field or method ([A-Za-z_0-9]+)/\1.\2/'; } | sort -u)
  [ -z "$names" ] && { echo "round $round: nothing to restore"; break; }
  out=$($S/prune restore $S/oldrt closure $names)
  echo "round $round: $(echo "$names" | wc -l) names, $out"
  case "$out" in *"restored 0 "*) echo "$names" | head -40; break;; esac
  ~/go/bin/goimports -w closure/rt_*.go 2>/dev/null
done
