#!/usr/bin/env bash
# Applies the enums step: folds the constant classes of enums whose constants have bodies into
# their enum (-bodies), then makes every enum declared in generated code an integer type.
# Rerunning it on its own output changes nothing.
set -euo pipefail
cd "$(dirname "$0")/../../.."
(cd tools/postflip && go run ./enums -bodies ../../go/closure && go run ./enums ../../go/closure)
(cd go && gofmt -w closure/g_*.go && go build -o /dev/null ./closure)
