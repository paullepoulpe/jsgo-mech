#!/usr/bin/env bash
# Applies the lambdas step: runs the tool, and while the compiler reports a hoisted lambda in an
# initialization cycle, restores the generated files, adds it to keep.txt and runs it again.
set -euo pipefail
cd "$(dirname "$0")/../../.."
keep=tools/postflip/lambdas/keep.txt
: > "$keep"
while :; do
  (cd tools/postflip && go run ./lambdas -keep "../../$keep" ../../go/closure)
  (cd go && gofmt -w closure/g_*.go)
  new=$( (cd go && go build -o /dev/null ./closure 2>&1 || true) | sed -n 's/.*initialization cycle for \(lambda_[A-Za-z0-9_]*\).*/\1/p' | sort -u)
  [[ -z $new ]] && break
  echo "$new" >> "$keep"; sort -u -o "$keep" "$keep"
  git checkout -q -- go/closure
done
(cd go && go build -o /dev/null ./closure)
echo "kept: $(wc -l < "$keep")"
