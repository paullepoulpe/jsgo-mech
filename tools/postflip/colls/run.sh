#!/usr/bin/env bash
# Applies the collection steps of round 7 (postflip5-r7-erased) in order: every colls kind, then
# native, which turns the typed collections that qualify into slices and Go maps. Each kind works
# on what the earlier ones left; a rerun on its own output changes nothing.
set -euo pipefail
cd "$(dirname "$0")/../../.."
for k in list map set deque treeset treemap idmap; do
  (cd tools/postflip && COLLS_CTORS=1 go run ./colls -kind "$k" ../../go/closure 2>&1 | grep -E 'constructors:|become|files changed')
done
for k in list map set imm; do
  (cd tools/postflip && go run ./native -kind "$k" ../../go/closure 2>&1 | grep -E 'become|files changed')
done
(cd go && gofmt -w closure/g_*.go && go build -o /dev/null ./closure)
