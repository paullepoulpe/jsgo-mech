#!/usr/bin/env bash
# The test of step "one typed collection family" (tools/postflip/unify + its runtime patch) made
# while both families exist: builds a package closure that has the translator's typed
# collections (the tree before the step: JTList, JTMap, JTSet, jtconv_ and rt_jt*.go) and,
# beside them, the typed runtime of the tree after the step, runs the scenarios of
# go/closure/rt_tunify_test.go on each family and compares the two traces line by line
# (TestUnifyDiff in jfamily_test.go.in). With -golden the translator family's trace is written
# as go/closure/rt_tunify_golden_test.go of this tree, which TestUnifiedFamily compares the one
# family with after the step.
#
#   tools/postflip/unify/difftest/run.sh [-golden] <commit before the step> <commit after | WORKTREE> <scratch dir>
#
# Both families in one package: every top-level name that rt_jt*.go of the first tree declares
# and the second tree declares too (tsetImm_, TEntry, newTSortedSet_...; and splitToListT_ of
# rt_base_text.go) is prefixed "old" wherever it stands in the first tree (as reflip/rename.sh
# moved the family out of the way at the flip); then the second tree's rt_tlist.go, rt_tmap.go,
# rt_timm.go, rt_tsorted.go, rt_tdeque.go and the tests of the typed runtime are put beside it.
set -euo pipefail
golden=0
if [[ $1 == -golden ]]; then golden=1; shift; fi
before=$1 after=$2 D=$3
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../../.." && pwd)
cd "$root"
get() { # get FILE: the file of the tree after the step
  if [[ $after == WORKTREE ]]; then cat "$root/$1"; else git show "$after:$1"; fi
}
rm -rf "$D" && mkdir -p "$D"
git archive "$before" go | tar -x -C "$D"
C=$D/go/closure
decls() { # the top-level names declared in the files given
  python3 -I - "$@" <<'PY'
import re, sys
names = set()
for f in sys.argv[1:]:
    block = False
    for line in open(f):
        if block:
            if line.startswith(')'):
                block = False
            else:
                m = re.match(r'\t(\w+)', line)
                if m: names.add(m.group(1))
            continue
        if re.match(r'(var|const|type) \($', line):
            block = True
            continue
        m = re.match(r'func (\w+)[\[(]', line) or re.match(r'type (\w+)', line)
        if m:
            names.add(m.group(1))
            continue
        m = re.match(r'(?:var|const) ((?:\w+, )*\w+)', line)
        if m: names.update(m.group(1).split(', '))
print('\n'.join(sorted(names)))
PY
}
decls "$C"/rt_jt*.go > "$D/j-names.txt"
mkdir "$D/after"
for f in $(if [[ $after == WORKTREE ]]; then ls go/closure/*.go; else git ls-tree --name-only "$after" go/closure/ | grep '\.go$'; fi); do
  get "$f" > "$D/after/$(basename "$f")"
done
decls "$D"/after/*.go > "$D/after-names.txt"
{ comm -12 "$D/j-names.txt" "$D/after-names.txt"; echo splitToListT_; } | sort -u > "$D/clash.txt"
expr=$(awk '{ printf "s/\\b%s\\b/old%s/g;", $1, $1 }' "$D/clash.txt")
perl -pi -e "$expr" "$C"/*.go
for f in rt_tlist.go rt_tmap.go rt_timm.go rt_tsorted.go rt_tdeque.go rt_tunify_test.go rt_timm_test.go rt_tnested_test.go; do
  cp "$D/after/$f" "$C/$f"
done
cp "$here/jfamily_test.go.in" "$C/rt_tunify_j_test.go"
echo 'package closure

var unifyGolden_ = []string{}' > "$C/rt_tunify_golden_test.go"
echo "names moved out of the way: $(wc -l < "$D/clash.txt")"
out=
if [[ $golden == 1 ]]; then out=$root/go/closure/rt_tunify_golden_test.go; fi
(cd "$D/go" && UNIFY_GOLDEN=$out go test ./closure -run 'TestUnifyDiff' -count=1 -v 2>&1 | grep -v '^=== RUN' | tail -40)
