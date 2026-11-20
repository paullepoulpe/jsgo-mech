#!/usr/bin/env bash
# Applies the typed Guava immutables step (round 8, postflip5-r8-imm) after step immnames:
# the immutable collections, then their builders (a builder's build() meets the typed classes
# of the first three). The imm kinds know the Java names of the typed map's and set's methods,
# so tnames -undo runs first and tnames last. A rerun on its own output changes nothing.
# run.sh, the whole collections pipeline, runs the same kinds after colls and native (whose
# tlistImm_ forms they convert): use this script only on a tree run.sh has been run on.
#   tools/postflip/colls/imm.sh [dir of package closure]   (default: go/closure)
set -euo pipefail
cd "$(dirname "$0")/../../.."
dir=$(cd "${1:-go/closure}" && pwd)
cd tools/postflip
go run ./tnames -undo "$dir" 2>&1 | grep -E 'tnames:'
for k in immlist immset immmap immlistb immsetb immmapb; do
  COLLS_CTORS=1 go run ./colls -kind "$k" "$dir" 2>&1 | grep -E 'constructors|become|on the spot|files changed'
done
go run ./tnames "$dir" 2>&1 | grep -E 'left:|tnames:'
(cd "$dir/.." && gofmt -w closure/*.go && go build -o /dev/null ./closure)
