#!/usr/bin/env bash
# Restores self-test helpers, fixes unit tests, then runs self-test, unit tests and e2e.
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cd /home/user/jsgo-mech || exit 1
for i in 1 2 3 4 5 6; do
  bash tools/j2go/selftest/run.sh > $S/st.out 2>&1
  names=$( { grep "j2go stub" .cache/j2go-selftest/go/closure/g_zstubs.go | sed 's/.*j2go stub: //; s/").*//' | grep -v "^Abstract"; grep -oE "undefined: [A-Za-z_0-9]+|type \*?[A-Za-z_0-9]+ has no field or method [A-Za-z_0-9]+" $S/st.out | sed -E 's/undefined: //; s/type \*?([A-Za-z_0-9]+) has no field or method ([A-Za-z_0-9]+)/\1.\2/'; } | sort -u)
  [ -z "$names" ] && break
  out=$($S/prune restore $S/oldrt go/closure $names)
  case "$out" in *"restored 0 "*) echo "selftest still needs: $names"; break;; esac
  ~/go/bin/goimports -w go/closure/rt_*.go
done
tail -1 $S/st.out
cd go
python3 ../tools/prune/dropbroken.py --tests | tail -1
~/go/bin/goimports -w closure/rt_*_test.go
gofmt -w closure/rt_*.go
go test ./closure 2>&1 | grep -v "Fatal Error" | cut -c1-200 | tail -4
go build -o $S/mech ./cmd/mech || exit 1
cd ..
python3 -I tools/gate0/e2e.py --go $S/mechsafe 2>&1 | tail -4
echo "runtime lines: $(ls go/closure/rt_*.go | grep -v _test | grep -v proto_gen | xargs cat | wc -l)  test lines: $(cat go/closure/rt_*_test.go | wc -l)"
