#!/usr/bin/env bash
# compile main only into scratch dir, print errors
cd /home/user/work/p14
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
rm -rf $S/cm && mkdir -p $S/cm
find java/src -name '*.java' -not -path '*/testing/*' | LC_ALL=C sort > $S/cm-files.txt
env -u JAVA_TOOL_OPTIONS javac -nowarn -Xlint:none -encoding UTF-8 -parameters -Xmaxerrs 400 -d $S/cm -cp ".cache/build/stock-libs.jar:$(ls .cache/deps/*.jar | tr '\n' ':')" -proc:none @$S/cm-files.txt 2>&1 | grep -v '^Note:'
