#!/usr/bin/env bash
# Trial re-flip, preparation: moves the typed runtime family that j2go now emits (TList, TMap,
# TSet, tconv_ and the tc_* conversions, null_/isnull_/unboxT_...) out of the way of the family
# of the same names that the post-flip steps add (patches/003-6f00ebfe-rt.patch and later: the
# two were written apart and are not the same code). Every top-level name of the trial tree that
# the post-flip history also declares (rename-names.txt: names new in the trial flip against
# 01f4f269 that are also new anywhere in postflip5's runtime patches or tip) gets a j/J prefix,
# in the generated files and in the runtime; the four runtime files the post-flip patches would
# create again are renamed rt_j*.go. A word-boundary substitution: the names are distinctive
# (checked: no new name exists before the run), and the same token is renamed wherever it
# stands, so the code is the same program.
set -euo pipefail
cd "$(dirname "$0")/../../.."
names=tools/postflip/reflip/rename-names.txt
expr=$(awk '{ n = $1; p = (n ~ /^[A-Z]/) ? "J" : "j"; printf "s/\\b%s\\b/%s%s/g;", n, p, n }' "$names")
for n in $(awk '{ n = $1; p = (n ~ /^[A-Z]/) ? "J" : "j"; print p n }' "$names"); do
  if grep -qw "$n" go/closure/*.go; then echo "name $n exists already" >&2; exit 1; fi
done
perl -pi -e "$expr" go/closure/*.go
for f in tlist tmap tset tdeque timm tsorted; do git mv go/closure/rt_$f.go go/closure/rt_j$f.go; done
gofmt -w go/closure/*.go
# Second trial: the field typed_ of the erased immutable types (rt_guava.go, rt_guava_map.go:
# the typed twin the translator's typed immutables keep there) is jtyped_; the post-flip
# runtime patch 011-6909e22a gives the same types a field of that name (its typed copy).
perl -pi -e 's/\btyped_\b/jtyped_/g' go/closure/*.go
gofmt -w go/closure/*.go
