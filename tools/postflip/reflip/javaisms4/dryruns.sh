#!/bin/bash
# dry-run (-n) reports of the tree's own post-flip tools on a scratch copy of go/closure
S=/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/javaisms
source /mnt/data/ubuntu/jm/env.sh
export GOFLAGS=-mod=mod GOCACHE=$S/gocache
cd $S/tools_r/postflip
T=$S/tree/closure
run() { out=$1; shift; nice -n 15 go run "$@" $T > $S/dry/$out.out 2>&1; echo "$out rc=$?"; if ! diff -rq -x .cache $T /mnt/data/ubuntu/jm/wt/agQr/go/closure >/dev/null; then echo "$out MODIFIED THE COPY; restoring"; rm -rf $T; cp -r /mnt/data/ubuntu/jm/wt/agQr/go/closure $T; fi; }
for k in list map set deque treeset treemap idmap immlist immset immmap immlistb immsetb immmapb; do run colls-$k ./colls -kind $k -n -v; done
for k in list map set imm; do run native-$k ./native -kind $k -n -v; done
run iters ./iters -n
run casts ./casts -n
run casts-named ./casts -n -named
run castfold-assert ./castfold -assert -n -v
run castfold-instof ./castfold -n -v -instof
run castfold-nilup ./castfold -n -v -nilup
run castfold ./castfold -n -v
for t in icasts instof patinst funcparams fnobj eqhash eqtyped strnative sbuild adapters tfnargs etaconv tostr strview strunits strbytes precond generics passfn unify; do run $t ./$t -n; done
for t in optstr gcasts tviews tfnparams deadtfn lamparams fntypes tgeneric; do run $t ./$t -n -v; done
run anyconv ./anyconv -n -all
echo ALLDONE
diff -rq $T /mnt/data/ubuntu/jm/wt/agQr/go/closure | grep -v "\.cache" | head
