#!/bin/bash
# upgate.sh <label>: gate the tree as it is (already translated), then build proto/m_<label> and the census
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cd /home/user/jm-upcast
export GOCACHE=$S/upcache
rm -rf $S/upcache/??
{
flock $S/gate.lock bash -c "$S/waitmem.sh 6; bash tools/prune/verify.sh"
echo "tests: $(cat go/closure/rt_*_test.go | grep -c '^func Test')"
$S/waitmem.sh 6
(cd go && go build -o $S/proto/m_$1 ./cmd/mech) && echo BUILT
(cd go/closure && python3 /home/user/jm-merge/tools/postflip/census.py > $S/up_census_$1.txt)
echo GATEDONE
} > $S/upgate_$1.log 2>&1
