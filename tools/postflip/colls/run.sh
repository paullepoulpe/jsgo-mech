#!/usr/bin/env bash
# Applies the collection steps in order: every colls kind (repeated while a kind still types
# something: a list held in a map can be typed once the map is), then native, which turns the
# typed collections that qualify into slices and Go maps, the colls imm kinds (the typed
# Guava immutables and their builders, imm.go; after step immnames: they convert the
# tlistImm_ forms colls and native leave), nilmaps (nil checks where a Go map that may be nil
# is read) and tnames (Go names for the methods of the typed map and set). colls, native and
# the imm kinds know the Java names, so tnames -undo runs first and tnames last. A rerun on
# its own output changes nothing. The three groups (colls, native, imm kinds) are repeated as a
# whole (pass 1, 2...) until a pass changes nothing: a collection held in a typed immutable or
# its builder can be typed once that is, and native makes slices and Go maps of the typed
# immutables that qualify.
#   tools/postflip/colls/run.sh [dir of package closure]   (default: go/closure)
set -euo pipefail
cd "$(dirname "$0")/../../.."
dir=$(cd "${1:-go/closure}" && pwd)
cd tools/postflip
go run ./tnames -undo "$dir" 2>&1 | grep -E 'tnames:'
# run <tool> <grep pattern> <args>: runs the tool, prints the lines of its output that match,
# and adds the files it changed to $changed.
changed=0
run() {
  local tool=$1 pat=$2 out n
  shift 2
  out=$(COLLS_CTORS=1 go run "./$tool" "$@" "$dir" 2>&1) || { echo "$out" | tail -20; exit 1; }
  echo "$out" | grep -E "$pat" || true
  n=$(echo "$out" | sed -n 's/^files changed: //p')
  changed=$((changed + ${n:-0}))
}
for pass in 1 2 3 4 5; do
  echo "pass $pass"
  passChanged=0
  for round in 1 2 3 4; do
    changed=0
    for k in list map set deque treeset treemap idmap; do
      run colls 'constructors:|become|files changed' -kind "$k"
    done
    passChanged=$((passChanged + changed))
    [ "$changed" -eq 0 ] && break
  done
  changed=0
  for k in list map set imm; do
    run native 'become|files changed' -kind "$k"
  done
  for k in immlist immset immmap immlistb immsetb immmapb; do
    run colls 'constructors|become|on the spot|files changed' -kind "$k"
  done
  passChanged=$((passChanged + changed))
  [ "$passChanged" -eq 0 ] && break
done
go run ./nilmaps "$dir" 2>&1 | grep -E 'checked:|left:|nilmaps:'
go run ./tnames "$dir" 2>&1 | grep -E 'left:|tnames:'
(cd "$dir/.." && gofmt -w closure/*.go && go build -o /dev/null ./closure)
