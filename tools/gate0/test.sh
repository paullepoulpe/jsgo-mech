#!/usr/bin/env bash
# Runs every JUnit suite of the Java tree (after tools/gate0/build.sh) in
# parallel JVMs and prints one summary line; exit status 0 only if all pass.
#   tools/gate0/test.sh [shards]      default: number of CPUs
# Logs: .cache/gate0/shard-N.log
set -euo pipefail
cd "$(dirname "$0")/../.."
shards=${1:-$(nproc)}
root=$PWD
cp="$root/.cache/build/test:$root/.cache/build/main:$root/java/test:$root/.cache/build/stock-libs.jar:$(ls "$root"/.cache/deps/*.jar | tr '\n' ':')"
rm -rf .cache/gate0 && mkdir -p .cache/gate0
# A suite is a top-level, non-abstract class in a file that mentions @Test or @RunWith.
grep -rlE '@Test\b|@RunWith' java/test --include='*.java' | LC_ALL=C sort | while read -r f; do
  name=$(basename "$f" .java)
  grep -qE "^(public )?abstract class $name\b" "$f" && continue
  echo "$f" | sed -e 's|^java/test/||' -e 's|\.java$||' -e 's|/|.|g'
done >.cache/gate0/suites.txt
split -n "r/$shards" -d .cache/gate0/suites.txt .cache/gate0/shard-
pids=()
for s in .cache/gate0/shard-[0-9]*; do
  (cd java && java -Xmx2g -Xss4m -cp "$cp" org.junit.runner.JUnitCore $(cat "$root/$s") >"$root/$s.log" 2>&1) &
  pids+=($!)
done
status=0
for p in "${pids[@]}"; do wait "$p" || status=1; done
python3 -I - <<'PY'
import glob, re
run = fail = 0
for f in sorted(glob.glob(".cache/gate0/shard-*.log")):
    t = open(f, errors="replace").read()
    m = re.search(r"^OK \((\d+) tests?\)", t, re.M)
    if m:
        run += int(m.group(1)); continue
    m = re.search(r"^Tests run: (\d+),\s+Failures: (\d+)", t, re.M)
    if m:
        run += int(m.group(1)); fail += int(m.group(2))
    else:
        print(f"{f}: no JUnit summary (crashed?)"); fail += 1
print(f"suites: {sum(1 for _ in open('.cache/gate0/suites.txt'))}, tests run: {run}, failures: {fail}")
PY
exit $status
