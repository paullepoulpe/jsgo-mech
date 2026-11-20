#!/usr/bin/env bash
# Applies the generic-classes step (generics -classes): runs the tool, formats and builds.
set -euo pipefail
cd "$(dirname "$0")/../../.."
(cd tools/postflip && go run ./generics -classes ../../go/closure)
(cd go && gofmt -w closure/g_*.go closure/rt_*.go && go build -o /dev/null ./closure)
