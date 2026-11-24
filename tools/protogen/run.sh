#!/usr/bin/env bash
# Regenerates go/closure/rt_proto_gen.go (the Java-style API of every protobuf
# message, builder and enum) from the .proto files under java/src.
#
# The generator reads go/closure/externals.tsv (which overload the translator
# gave the plain name), go/closure/g_0names.go (Go names of Java classes) and
# go/closure/g_zstubs.go (which ...OrBuilder interfaces the translator declares),
# so run it after tools/j2go/regen.sh whenever the translated code starts using
# a new overload, then run regen.sh once more.
#
# Rule T5 (docs/javaisms6/JAVAISMS6.md): next to each erased getFooList() of a repeated message or
# string field it writes the typed getFooList_t() ListT[X], and toBuilder_t() *Foo_Builder for
# each message, which the translator's piece `proto` calls. PROTOGEN_NOT5=1 writes the file
# without them (the output before the rule).
set -euo pipefail
cd "$(dirname "$0")/../.."
root=$(pwd)
(cd tools/protogen && go run . -root "$root" -out "$root/go/closure/rt_proto_gen.go")
gofmt -w go/closure/rt_proto_gen.go
