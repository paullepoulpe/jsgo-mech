#!/bin/bash
cd /home/user/work/p05e
R=$PWD
cd java && env -u JAVA_TOOL_OPTIONS java -Xmx2g -Xss4m -cp "$R/.cache/build/test:$R/.cache/build/main:$R/java/test:$R/.cache/build/stock-libs.jar:$(ls $R/.cache/deps/*.jar | tr '\n' ':')" org.junit.runner.JUnitCore "$@" > $R/.cache/last-test.txt 2>&1
grep -E -A${CTX:-6} "^(OK|Tests run|FAILURES|[0-9]+\) )" $R/.cache/last-test.txt | head -${HEADN:-60}
