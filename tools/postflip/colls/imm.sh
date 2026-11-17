#!/usr/bin/env bash
# Applies the typed Guava immutables step (round 8, postflip5-r8-imm) after step immnames:
# the immutable collections, then their builders (a builder's build() meets the typed classes
# of the first three). A rerun on its own output changes nothing.
set -euo pipefail
cd "$(dirname "$0")/../../.."
for k in immlist immset immmap immlistb immsetb immmapb; do
  (cd tools/postflip && COLLS_CTORS=1 go run ./colls -kind "$k" ../../go/closure 2>&1 | grep -E 'constructors|become|on the spot|files changed')
done
(cd go && gofmt -w closure/g_*.go && true)
