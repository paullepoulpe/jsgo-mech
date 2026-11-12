#!/usr/bin/env bash
# Fetches the pinned Closure Compiler into .cache/:
#   closure-src/        the production .java sources (Maven sources jar)
#   closure-unshaded.jar  the npm build of the same release. Unlike Maven's
#                       jar it bundles Guava, Gson, protobuf and args4j under
#                       their real package names, so javac can resolve the
#                       sources' imports against it.
set -euo pipefail
cd "$(dirname "$0")/.."
source pin.env
mkdir -p .cache
if [[ ! -d .cache/closure-src ]]; then
  url=https://repo1.maven.org/maven2/com/google/javascript/closure-compiler/$CLOSURE_VERSION/closure-compiler-$CLOSURE_VERSION-sources.jar
  curl -fsSL -o .cache/closure-src.jar.tmp "$url"
  echo "$CLOSURE_SRC_SHA256  .cache/closure-src.jar.tmp" | sha256sum -c --quiet
  mv .cache/closure-src.jar.tmp .cache/closure-src.jar
  rm -rf .cache/closure-src.tmp && mkdir .cache/closure-src.tmp
  (cd .cache/closure-src.tmp && unzip -q ../closure-src.jar)
  mv .cache/closure-src.tmp .cache/closure-src
fi
if [[ ! -s .cache/closure-unshaded.jar ]]; then
  ver=${CLOSURE_VERSION#v}.0.0
  curl -fsSL "https://registry.npmjs.org/google-closure-compiler-java/-/google-closure-compiler-java-$ver.tgz" |
    tar -xzO package/compiler.jar >.cache/closure-unshaded.jar.tmp
  mv .cache/closure-unshaded.jar.tmp .cache/closure-unshaded.jar
fi
