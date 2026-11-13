#!/usr/bin/env bash
# Extracts the non-class resources of the Closure Compiler jar (externs.zip,
# the runtime JS libraries, *.properties, *.txt ...) into go/closure/resources,
# from where the Go runtime embeds them (rt_io_resources.go) and serves them
# through Class.getResource / Class.getResourceAsStream / ResourceBundle.
#   tools/j2go/extract-resources.sh [jar] [outdir]
# Skipped: classes, META-INF, and the trees of bundled third-party libraries.
set -euo pipefail
cd "$(dirname "$0")/../.."
jar=${1:-.cache/closure-unshaded.jar}
out=${2:-go/closure/resources}
[[ -f $jar ]] || { echo "missing $jar" >&2; exit 1; }
rm -rf "$out"
mkdir -p "$out"
# The list is sorted so that the result does not depend on the jar's entry order.
unzip -Z1 "$jar" | LC_ALL=C sort -u | while IFS= read -r name; do
  case "$name" in
    */ | *.class) continue ;;
    META-INF/* | OSGI-OPT/* | OSGI-INF/* | images/*) continue ;;
    org/apache/* | org/kohsuke/* | org/jspecify/* | org/checkerframework/*) continue ;;
    com/google/common/* | com/google/protobuf/* | google/protobuf/* | com/google/gson/*) continue ;;
    com/google/thirdparty/* | com/google/errorprone/* | com/google/j2objc/* | com/google/auto/*) continue ;;
    javax/* | *.java) continue ;;
  esac
  mkdir -p "$out/$(dirname "$name")"
  unzip -p "$jar" "$name" > "$out/$name"
done
echo "extracted $(find "$out" -type f | wc -l) resources ($(du -sh "$out" | cut -f1)) into $out"
