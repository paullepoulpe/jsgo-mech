#!/usr/bin/env bash
# One prune pass over the handwritten runtime (go/closure/rt_*.go):
#   1. drop runtime declarations that no longer compile (they mention library types the
#      translated code stopped using) -- every removal is printed, audit them;
#   2. root every translated function, ask deadcode which runtime functions are unreachable,
#      delete those (rules in main.go);
#   3. put back anything the translated code, the remaining runtime or the self-test still
#      references, taking the text from the last commit.
# Needs: tools/j2go/run.sh has generated go/closure/g_*.go; `go install
# golang.org/x/tools/cmd/deadcode@latest golang.org/x/tools/cmd/goimports@latest`.
# Run tools/prune/verify.sh afterwards.
set -uo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
bin=$root/.cache/prune
mkdir -p "$bin"
(cd "$root/tools/prune" && go build -o "$bin/prune" .) || exit 1
gobin=$(go env GOPATH)/bin
old=$bin/oldrt
rm -rf "$old" && mkdir -p "$old"
for f in $(git -C "$root" ls-tree --name-only HEAD go/closure/ | grep 'rt_.*\.go$' | grep -v '_test\.go$\|rt_proto_gen\.go$'); do
  git -C "$root" show "HEAD:$f" >"$old/$(basename "$f")"
done
cd "$root/go" || exit 1

restore() { # repeats until generated code and runtime reference nothing that is missing
  for round in $(seq 1 25); do
    bash "$root/tools/j2go/regen.sh" closure >/dev/null 2>&1
    names=$( { grep "j2go stub" closure/g_zstubs.go | sed 's/.*j2go stub: //; s/").*//' | grep -v "^Abstract"
      go build -gcflags=-e ./... 2>&1 | grep -v "^#" | grep -oE "undefined: [A-Za-z_0-9]+|type \*?[A-Za-z_0-9]+ has no field or method [A-Za-z_0-9]+" |
        sed -E 's/undefined: //; s/type \*?([A-Za-z_0-9]+) has no field or method ([A-Za-z_0-9]+)/\1.\2/'; } | sort -u)
    [ -z "$names" ] && return 0
    out=$("$bin/prune" restore "$old" closure $names)
    echo "restore round $round: $(echo "$names" | wc -l) names, $out"
    case "$out" in *"restored 0 "*) echo "cannot restore (not in the last commit; implement them):"; echo "$names" | head -60; return 1 ;; esac
    "$gobin/goimports" -w closure/rt_*.go 2>/dev/null
  done
}

bash "$root/tools/j2go/regen.sh" closure >/dev/null 2>&1
python3 "$root/tools/prune/dropbroken.py"
"$gobin/goimports" -w closure/rt_*.go 2>/dev/null
restore || exit 1
go build ./... || { echo "does not build"; exit 1; }
"$bin/prune" roots closure
printf 'package main\n\nimport "jsgomech/closure"\n\nvar _ = closure.Roots_\n' >cmd/mech/roots_tmp.go
# deadcode must be built with the Go toolchain that go/go.mod names (an older one cannot
# type-check the standard library and finds nothing); DEADCODE=<binary> uses another build.
"${DEADCODE:-$gobin/deadcode}" ./cmd/mech >"$bin/dead.txt" 2>&1
if grep -q "packages contain errors" "$bin/dead.txt"; then
  head -3 "$bin/dead.txt"; rm -f closure/rt_zroots.go cmd/mech/roots_tmp.go
  echo "deadcode failed: rebuild it with $(go version | cut -d' ' -f3)"; exit 1
fi
echo "unreachable runtime functions: $(grep -c closure/rt_ "$bin/dead.txt")"
"$bin/prune" closure <"$bin/dead.txt"
rm -f closure/rt_zroots.go cmd/mech/roots_tmp.go
"$gobin/goimports" -w closure/rt_*.go
restore || exit 1
# the self-test uses a few library members the compiler does not (its main method prints to
# System.out, a PrintStream; main is not run from Go, so that one stays a placeholder)
cd "$root"
for i in 1 2 3 4 5 6; do
  bash tools/j2go/selftest/run.sh >"$bin/st.out" 2>&1
  names=$( { grep "j2go stub" .cache/j2go-selftest/go/closure/g_zstubs.go | sed 's/.*j2go stub: //; s/").*//' | grep -v "^Abstract\|^PrintStream\."
    grep -oE "undefined: [A-Za-z_0-9]+|type \*?[A-Za-z_0-9]+ has no field or method [A-Za-z_0-9]+" "$bin/st.out" |
      sed -E 's/undefined: //; s/type \*?([A-Za-z_0-9]+) has no field or method ([A-Za-z_0-9]+)/\1.\2/'; } | sort -u)
  [ -z "$names" ] && break
  out=$("$bin/prune" restore "$old" go/closure $names)
  case "$out" in *"restored 0 "*) echo "self-test still needs: $names"; break ;; esac
  "$gobin/goimports" -w go/closure/rt_*.go
done
gofmt -w go/closure/rt_*.go
(cd go && go build ./... ) && echo "runtime lines: $(ls go/closure/rt_*.go | grep -v '_test\|proto_gen' | xargs cat | wc -l)"
