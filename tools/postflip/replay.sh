#!/usr/bin/env bash
# Replays post-flip steps 3-7 on the current branch (which must hold steps 1-2), running each
# tool, the gates (tools/prune/verify.sh: runtime tests and four byte comparisons with Java)
# and committing the step. Usage: tools/postflip/replay.sh <dir with the final runtime files>
set -euo pipefail
cd "$(dirname "$0")/../.."
rt=$1
P=tools/postflip
trailer=$'\n\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>\nClaude-Session: https://claude.ai/code/session_0162Uc5JieSYhVmqiHv9WVbQ'
gate() {
  (cd go && gofmt -w closure/g_*.go && go build ./closure)
  bash tools/prune/verify.sh > .cache/replay-verify.log 2>&1
  if [[ $(grep -c ": MATCH" .cache/replay-verify.log) != 4 ]] || ! grep -q "^ok" .cache/replay-verify.log; then
    cat .cache/replay-verify.log; echo "GATE FAILED at $1"; exit 1
  fi
  echo "$1: gates pass"
}
commit() { git add -A go/closure tools/postflip; git add -f go/closure/g_zconv_*.go 2>/dev/null || true; git commit -q -m "$(printf "%s" "$1" | sed "1a\\\\")$trailer"; git log --oneline -1; }

cp "$rt/rt_tlist.go" "$rt/rt_tmap.go" go/closure/
out=$(cd $P && go run ./colls -kind list ../../go/closure 2>&1 | grep become); echo "$out"; gate "step 3"
commit "postflip step 3: lists become typed lists across method boundaries
tools/postflip/colls -kind list: $out.
Every list-holding variable, field, parameter and result that exchange values
(assignment, argument, return, comparison; methods across overriding by name and
signature) form a class. A class becomes *TList[T] when its values all start as a new
ArrayList, a copy of a collection, an ImmutableList, nil or another slot of the class;
its slots agree on one element type T (for covariant overrides, the type all stated
ones are assignable to); the runtime does not touch it; and no value of it is boxed into
an Object (Objects are cast back to *ArrayList elsewhere: JSDocInfo property bags).
add, get, set, size, isEmpty, clear, remove(int), add(int, e), addAll from a typed list
and for-each are typed; other methods, and values that leave for erased code, go
through asList(), a view with the list identity. g_zconv_lists.go: element conversions.
rt_tlist.go (handwritten): TList, its view and iterator, mirroring ArrayList.

verify.sh: runtime tests ok, four projects MATCH."

out=$(cd $P && go run ./colls -kind map ../../go/closure 2>&1 | grep become); echo "$out"; gate "step 4"
commit "postflip step 4: maps become typed maps across method boundaries
tools/postflip/colls -kind map: $out.
The analysis of step 3 for maps: *TMap[K, V] when the values all start as a new
LinkedHashMap, a copy of a map, an ImmutableMap, nil or another slot; keys compare in Go
as in Java (strings, handles, classes without equals); not boxed into an Object. get,
put, containsKey, remove, getOrDefault, putIfAbsent, computeIfAbsent, the string-key
helpers and for-each over keySet()/values() are typed; the rest goes through asMap().
rt_tmap.go (handwritten): TMap / TSet in insertion order with erased views.

verify.sh: runtime tests ok, four projects MATCH."

out=$(cd $P && go run ./colls -kind set ../../go/closure 2>&1 | grep become); echo "$out"; gate "step 5"
commit "postflip step 5: sets become typed sets across method boundaries
tools/postflip/colls -kind set: $out.
The analysis of steps 3 and 4 for sets: *TSet[K]; add, contains, remove, addAll from a
typed set, the string-key helpers and for-each are typed; the rest goes through asSet().

verify.sh: runtime tests ok, four projects MATCH."

out=$(cd $P && go run ./dispatch ../../go/closure 2>&1 | grep dispatchers); echo "$out"; gate "step 6"
commit "postflip step 6: dispatch to a single implementation is a concrete type assertion
tools/postflip/dispatch: $out.
When the package declares m_impl exactly once, on a class D that no class extends, every
object reaching the dispatcher is a D: this.self_.(interface{ m_impl(...) }) becomes
this.self_.(*D), one comparison instead of an interface lookup, and a direct call.

verify.sh: runtime tests ok, four projects MATCH."

(cd $P && go run ./optional ../../go/closure 2>&1 | tail -1)
cp "$rt/rt_base.go" "$rt/rt_io.go" "$rt/rt_io_test.go" go/closure/
sed -i 's/^type OptionalOf\[T0 any\] = \*Optional$/type OptionalOf[T0 any] = Optional/' go/closure/g_0aliases.go
sed -i -E 's/\*Optional\b([^_]|$)/Optional\1/g' go/closure/g_*.go
gate "step 7"
commit "postflip step 7: java.util.Optional is a value, not a heap object
Optional_of / Optional_empty / Optional_ofNullable allocated an object each. Optional is
now a two-word value whose zero value is Java null; every method of a null Optional
throws NullPointerException; Optional.empty() is a non-null value with nothing in it.
The type changes once for the declarations spelled OptionalOf[T] (g_0aliases.go) and by
sed for those spelled *Optional; tools/postflip/optional found no nil comparison or nil
Optional in generated code that would change meaning.

verify.sh: runtime tests ok, four projects MATCH. From this step the translator self-test
is not a gate on this branch (it translates fresh Java against this runtime, and j2go,
retired at the flip, still emits *Optional)."
