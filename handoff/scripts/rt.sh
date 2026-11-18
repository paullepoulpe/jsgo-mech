#!/bin/bash
# usage: rt.sh Class...
R=/home/user/work/p05
cd $R/java && env -u JAVA_TOOL_OPTIONS java -Xmx2g -Xss4m -cp "$R/.cache/build/test:$R/.cache/build/main:$R/java/test:$R/.cache/build/stock-libs.jar:$(ls $R/.cache/deps/*.jar | tr '\n' ':')" org.junit.runner.JUnitCore "$@" 2>&1 | grep -E "^OK|^Tests run|^[0-9]+\) |Failures" | head -40
