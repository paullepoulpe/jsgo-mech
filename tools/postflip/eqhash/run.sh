#!/usr/bin/env bash
# Applies the eqhash step: runs the tool, formats and builds.
set -euo pipefail
cd "$(dirname "$0")/../../.."
(cd tools/postflip && go run ./eqhash ../../go/closure)
(cd go && gofmt -w closure/g_*.go && go build -o /dev/null ./closure)
