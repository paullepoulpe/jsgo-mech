#!/usr/bin/env bash
# Applies the collection steps in order: every colls kind (repeated while a kind still types
# something: a list held in a map can be typed once the map is), then native, which turns the
# typed collections that qualify into slices and Go maps, nilmaps (nil checks where a Go map
# that may be nil is read) and tnames (Go names for the methods of the typed map and set).
# colls and native know the Java names, so tnames -undo runs first. A rerun on its own output
# changes nothing.
#   tools/postflip/colls/run.sh [dir of package closure]   (default: go/closure)
set -euo pipefail
cd "$(dirname "$0")/../../.."
dir=$(cd "${1:-go/closure}" && pwd)
cd tools/postflip
go run ./tnames -undo "$dir" 2>&1 | grep -E 'tnames:'
for round in 1 2 3 4; do
  changed=0
  for k in list map set deque treeset treemap idmap; do
    out=$(COLLS_CTORS=1 go run ./colls -kind "$k" "$dir" 2>&1)
    echo "$out" | grep -E 'constructors:|become|files changed'
    n=$(echo "$out" | sed -n 's/^files changed: //p')
    changed=$((changed + ${n:-0}))
  done
  [ "$changed" -eq 0 ] && break
done
for k in list map set imm; do
  go run ./native -kind "$k" "$dir" 2>&1 | grep -E 'become|files changed'
done
go run ./nilmaps "$dir" 2>&1 | grep -E 'checked:|left:|nilmaps:'
go run ./tnames "$dir" 2>&1 | grep -E 'left:|tnames:'
(cd "$dir/.." && gofmt -w closure/*.go && go build -o /dev/null ./closure)
