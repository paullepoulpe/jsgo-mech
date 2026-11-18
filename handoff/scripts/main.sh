#!/bin/bash
# compile main only into scratch dir; print errors
cd /home/user/work/p12
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
find java/src -name '*.java' -not -path '*/testing/*' | LC_ALL=C sort > $S/main-files.txt
rm -rf $S/mainout; mkdir -p $S/mainout
env -u JAVA_TOOL_OPTIONS javac -nowarn -Xlint:none -Xmaxerrs 500 -encoding UTF-8 -parameters -d $S/mainout -cp ".cache/build/stock-libs.jar:$(ls .cache/deps/*.jar | tr '\n' ':')" -proc:none @$S/main-files.txt 2>&1 | grep -v '^Note:'
