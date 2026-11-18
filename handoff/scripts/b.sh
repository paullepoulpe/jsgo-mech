#!/bin/bash
cd /home/user/work/p05e
bash tools/gate0/build.sh 2>&1 | grep -v JAVA_TOOL | grep -v "^Note:" > .cache/last-build.txt
if grep -q "^[0-9]* errors\?$" .cache/last-build.txt; then
  awk '/^[0-9]+ errors?$/{print; exit} {print}' .cache/last-build.txt | head -${HEADN:-70}
else tail -1 .cache/last-build.txt; fi
