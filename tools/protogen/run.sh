#!/usr/bin/env bash
# Regenerates go/closure/rt_proto_gen.go (the Java-style API of every protobuf
# message, builder and enum) from the .proto files under java/src.
#
# The generator reads go/closure/externals.tsv (which overload the translator
# gave the plain name), go/closure/g_0names.go (Go names of Java classes) and
# go/closure/g_zstubs.go (which ...OrBuilder interfaces the translator declares),
# so run it after tools/j2go/regen.sh whenever the translated code starts using
# a new overload, then run regen.sh once more.
set -euo pipefail
cd "$(dirname "$0")/../.."
root=$(pwd)
(cd tools/protogen && go run . -root "$root" -out "$root/go/closure/rt_proto_gen.go")
gofmt -w go/closure/rt_proto_gen.go
