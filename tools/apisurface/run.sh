#!/usr/bin/env bash
# Lists every JDK / Guava / Gson / protobuf / args4j method, constructor and
# field that Closure's production Java calls, into external-api.tsv
# (kind, class, member, number of uses). Needs a JDK 21+.
set -euo pipefail
cd "$(dirname "$0")/../.."
bash tools/fetch.sh
find .cache/closure-src -name '*.java' -not -path '*/ant/*' | LC_ALL=C sort >.cache/prod-files.txt
java --add-modules jdk.compiler -Xmx6g tools/apisurface/ApiSurface.java \
  .cache/closure-unshaded.jar tools/apisurface/external-api.tsv @.cache/prod-files.txt
