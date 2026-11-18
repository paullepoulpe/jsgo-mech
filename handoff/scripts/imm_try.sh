#!/bin/bash
# Fresh scratch copy of the worktree package with the new runtime, then the imm kinds.
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
rm -rf $S/immtry/closure $S/immtry/go && git -C /home/user/jm-r7-typed archive eabc54fa go/closure | tar -x -C $S/immtry && mv $S/immtry/go/closure $S/immtry/closure && rm -f $S/immtry/closure/rt_guava_names.go && cp /home/user/jm-r7-typed/go/closure/rt_*.go $S/immtry/closure/
cd /home/user/jm-r7-typed/tools/postflip
for k in ${KINDS:-immlist immset immmap immlistb immsetb immmapb}; do
  COLLS_CTORS=1 go run ./colls -kind $k $S/immtry/closure > $S/immtry/$k.out 2>&1
  grep "become\|files\|on the spot\|constructors:" $S/immtry/$k.out
  go run ./zz_tcheck $S/immtry/closure 2>&1 | head -10
done
cd $S/immtry/closure && python3 $S/census.py | grep -i "immutable\|boxing\|class casts\|interface casts\|unboxing"
