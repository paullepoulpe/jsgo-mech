#!/usr/bin/env bash
# Round-7 merge onto postflip5: tool/runtime commits cherry-picked, steps re-run on the merged tree.
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cd /home/user/jm-merge
M=$S/mstep2.sh
cp() { bash $M "$1" "$2" "git cherry-pick -x $2" || exit 1; }       # label sha
run() { local l=$1 src=$2; shift 2; bash $M "$l" "$src" "$*" || exit 1; }
T="cd tools/postflip &&"
case $1 in
erased)
  for c in c8af8eb0 8e9f1084 23367e70 ba822192 aade0151 a4657007 8ab98458 68732427 1260786e; do cp e_$c $c; done
  run e_colls 45c0a294 "bash tools/postflip/colls/run.sh"
  bash $S/gate.sh r7erased || exit 1;;
typed)
  for c in 38e0a088 6931f41e 85b6b0ab 38889996 a34447d0 10eb7596 df4f2aed; do cp t_$c $c; done
  run t_index bad5b1a2 "($T go run ./ranges -index ../../go/closure)"
  run t_entries 6bdf91aa "($T go run ./ranges -entries ../../go/closure)"
  run t_iters1 b068cc7b "($T go run ./iters ../../go/closure)"
  run t_iters2 33e64059 "($T go run ./iters ../../go/closure)"
  bash $S/gate.sh r7typed || exit 1;;
strings)
  cp s_3b0dbb14 3b0dbb14; cp s_10cfcc48 10cfcc48
  run s_native a3a7e323 "($T go run ./strnative ../../go/closure)"
  cp s_f876c58b f876c58b
  run s_cuts ecd3b321 "bash tools/postflip/strcuts/run.sh"
  cp s_2a11d765 2a11d765; cp s_997ce95d 997ce95d
  run s_units 92b315ae "($T go run ./strunits ../../go/closure)"
  bash $S/gate.sh r7strings || exit 1;;
misc)
  cp m_06a7ae47 06a7ae47; cp m_941dff8d 941dff8d
  run m_nn 418292dd "($T go run ./nnchecks ../../go/closure)"
  run m_exc 2a0e4066 "($T go run ./exceptions ../../go/closure)"
  cp m_0942e54c 0942e54c
  run m_nn2 619fa876 "($T go run ./nnchecks ../../go/closure)"
  cp m_2c62d0d9 2c62d0d9; cp m_1207767a 1207767a
  run m_optof 3928337b "($T go run ./optof ../../go/closure)"
  cp m_5885ee20 5885ee20
  run m_instof 8e8ebbbc "($T go run ./instof ../../go/closure)"
  cp m_875e93eb 875e93eb; cp m_e56405b5 e56405b5
  run m_eqtyped 6cef1460 "($T go run ./eqtyped ../../go/closure)"
  cp m_32f8ae27 32f8ae27
  cp m_a18ad0cf a18ad0cf; cp m_b2de3300 b2de3300
  run m_passfn e32a934c "($T go run ./passfn ../../go/closure)"
  run m_funcparams 8eaecaaa "($T go run ./funcparams ../../go/closure)"
  bash $S/gate.sh r7misc || exit 1;;
generics)
  cp g_5a0a6530 5a0a6530; cp g_1e9455f2 1e9455f2; cp g_33ebec65 33ebec65
  run g_classes 95f6c07c "bash tools/postflip/generics/classes.sh"
  run g_fold dce4d90f "($T go run ./castfold ../../go/closure && go run ./castfold -instof ../../go/closure && go run ./casts -named ../../go/closure && go run ./anyconv -all ../../go/closure && go run ./objslots ../../go/closure)"
  bash $S/gate.sh r7generics || exit 1;;
esac
echo "R7 $1 DONE"
