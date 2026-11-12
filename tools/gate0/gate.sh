#!/usr/bin/env bash
# Gate 0: the check every Java prep step must pass. Builds the Java tree, runs
# all of its JUnit suites, and compares its output with the stock release's,
# byte for byte, on real projects.
set -euo pipefail
cd "$(dirname "$0")/../.."
bash tools/gate0/build.sh
bash tools/gate0/test.sh
python3 -I tools/gate0/e2e.py
echo "gate 0: PASS"
