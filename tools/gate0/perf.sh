#!/usr/bin/env bash
# Performance check for a prep step: the stock release against the Java tree
# as built by build.sh, on jsgo's benchmark tasks, with jsgo's own harness
# (tools/bench/bench.py there: interleaved cold runs, byte-for-byte output
# gate). Needs a jsgo checkout with its inputs fetched (tools/bench/fetch-inputs.sh).
#   JSGO_DIR=../jsgo tools/gate0/perf.sh [runs] [tasks]
set -euo pipefail
cd "$(dirname "$0")/../.."
jsgo=$(cd "${JSGO_DIR:-../jsgo}" && pwd)
runs=${1:-5}
tasks=${2:-typed-app,downlevel-simple,checks-only,generate-ijs,incremental-ijs-check,conformance-checks,shaka-blockly,bundle,typed-full}
b=$PWD/.cache/build
# One jar laid out like the stock one, so both start the same way.
rm -rf "$b/candidate" && mkdir "$b/candidate"
(cd "$b/candidate" && unzip -q ../stock-libs.jar && cp -r ../main/. .)
printf "Main-Class: com.google.javascript.jscomp.CommandLineRunner\nMulti-Release: true\n" >"$b/manifest.txt"
jar --create --file "$b/candidate.jar" --manifest "$b/manifest.txt" -C "$b/candidate" . 2>/dev/null
rm -rf "$b/candidate"
printf '#!/bin/sh\nexec env -u JAVA_TOOL_OPTIONS -u _JAVA_OPTIONS -u JDK_JAVA_OPTIONS java -jar %s "$@"\n' "$b/candidate.jar" >"$b/candidate.sh" && chmod +x "$b/candidate.sh"
out=$PWD/.cache/perf/$(git rev-parse --short HEAD)$(git diff --quiet || echo -dirty)
rm -rf "$out" && mkdir -p "$(dirname "$out")"
(cd "$jsgo" && python3 -I tools/bench/bench.py run --out "$out" --tasks "$tasks" --runs "$runs" \
  --skip-warm --skip-trace --jsgo "$b/candidate.sh") >"$out.log" 2>&1 || true
python3 -I - "$out" <<'PY'
import glob, json, os, statistics, sys
out = sys.argv[1]
tot = [0.0, 0.0]
print(f"{'task':24}{'stock':>9}{'built':>9}{'change':>9}  outputs")
for d in sorted(glob.glob(out + "/*/")):
    name = os.path.basename(d[:-1])
    try:
        a = json.load(open(d + "java-cold.json"))["runs"]; b = json.load(open(d + "jsgo.json"))["runs"]
        g = json.load(open(d + "gate.json"))
    except FileNotFoundError:
        continue
    wall = lambda rs: statistics.median(r["wall_s"] for r in rs)
    x, y = wall(a), wall(b); tot[0] += x; tot[1] += y
    print(f"{name:24}{x:8.2f}s{y:8.2f}s{(y / x - 1) * 100:+8.1f}%  {g.get('status', g)}")
print(f"{'sum':24}{tot[0]:8.2f}s{tot[1]:8.2f}s{(tot[1] / tot[0] - 1) * 100:+8.1f}%")
PY
