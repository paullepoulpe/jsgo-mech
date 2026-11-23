#!/bin/bash
# dry-run reports of the overlay's post-flip tools (flip6 tools/postflip, copied) on a scratch copy of tree6h's go/closure.
# Every invocation of reflip/steps.txt (inv2.txt), run without -n on the copy (the tools differ in how they take -n);
# the diff is recorded, then the copy is restored. The copy is compared with tree6h after every run.
S=/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/javaisms6
ORIG=/mnt/data/ubuntu/jm/wt/tree6h/go/closure
export GOFLAGS=-mod=mod GOCACHE=$S/gocache
cd $S/tools_postflip
T=$S/work/closure
while read -r _ tool args; do
  name=$(echo "$tool $args" | tr ' ' '_' | tr -d '-')
  if [ ! -d "$tool" ]; then echo "$name: no such tool in the overlay" ; echo "no such tool" > $S/dry/$name.out; continue; fi
  nflag=
  timeout 600 nice -n 15 go run ./$tool $nflag $args $T > $S/dry/$name.out 2>&1; rc=$?
  if ! diff -rq -x .cache $T $ORIG > /dev/null 2>&1; then
    diff -ru -x .cache $ORIG $T > $S/dry/$name.diff 2>&1
    echo "$name rc=$rc n=$nflag CHANGED THE COPY ($(grep -c '^[-+][^-+]' $S/dry/$name.diff) changed lines); restoring"
    rm -rf $T; cp -r $ORIG $T
  else
    echo "$name rc=$rc n=$nflag unchanged"
  fi
done < $S/inv2.txt
echo ALLDONE
