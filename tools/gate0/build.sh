#!/usr/bin/env bash
# Compiles the Java tree (java/src, java/test) with plain javac into
# .cache/build/{main,test}. Needs JDK 21+ and Maven (only to download jars).
#
# Libraries (Guava, Gson, protobuf, args4j, re2j), the generated protobuf
# classes and the bundled resources (externs, runtime JS) come from the stock
# release jar with every class that java/src compiles removed from it, so a
# class can never silently fall back to the stock build.
set -euo pipefail
cd "$(dirname "$0")/../.."
bash tools/fetch.sh
mkdir -p .cache/deps .cache/build
if [[ ! -e .cache/deps/.done ]]; then
  mvn -q -B -f tools/gate0/deps.pom.xml dependency:copy-dependencies -DoutputDirectory="$PWD/.cache/deps"
  # The stock jar already bundles these, at the versions Closure pins.
  rm -f .cache/deps/guava-3*.jar .cache/deps/protobuf-java-*.jar .cache/deps/failureaccess-*.jar .cache/deps/listenablefuture-*.jar
  touch .cache/deps/.done
fi
python3 -I - <<'PY'
import os, zipfile
src, out = ".cache/closure-unshaded.jar", ".cache/build/stock-libs.jar"
kept = dropped = 0
with zipfile.ZipFile(src) as zin, zipfile.ZipFile(out + ".tmp", "w", zipfile.ZIP_DEFLATED) as zout:
    for item in zin.infolist():
        n = item.filename
        if n.endswith(".class") and n.startswith(("com/google/javascript/", "com/google/debugging/")):
            top = n[:-6].split("$")[0]
            base = top.rsplit("/", 1)[-1]
            if os.path.exists("java/src/" + top + ".java") or "/testing/" in n or base.startswith(("AutoValue_", "AutoBuilder_")):
                dropped += 1
                continue
        if n.startswith("org/apache/tools/") or n.startswith("META-INF/") and n.upper().endswith((".SF", ".RSA", ".DSA")):
            continue
        zout.writestr(item, zin.read(n))
        kept += 1
os.replace(out + ".tmp", out)
print(f"stock-libs.jar: kept {kept} entries, dropped {dropped} Closure classes that java/src provides")
PY
libs=.cache/build/stock-libs.jar
deps=$(ls .cache/deps/*.jar | tr '\n' ':')
rm -rf .cache/build/main .cache/build/test
# */testing/ under java/src holds test helpers (Truth subjects and the like);
# upstream builds them with the tests, and so do we.
find java/src -name '*.java' -not -path '*/testing/*' | LC_ALL=C sort >.cache/build/main-files.txt
javac -nowarn -Xlint:none -encoding UTF-8 -parameters -d .cache/build/main -cp "$libs:$deps" \
  -proc:none @.cache/build/main-files.txt 2>&1 | grep -v '^Note:' || true
{ find java/test -name '*.java'; find java/src -name '*.java' -path '*/testing/*'; } | LC_ALL=C sort >.cache/build/test-files.txt
javac -nowarn -Xlint:none -encoding UTF-8 -parameters -d .cache/build/test \
  -cp ".cache/build/main:$libs:$deps" -proc:none @.cache/build/test-files.txt 2>&1 | grep -v '^Note:' || true
echo "main classes: $(find .cache/build/main -name '*.class' | wc -l), test classes: $(find .cache/build/test -name '*.class' | wc -l)"
