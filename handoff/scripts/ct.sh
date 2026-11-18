#!/usr/bin/env bash
# compile tests only (against .cache/build/main), print errors
cd /home/user/work/p14
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
rm -rf $S/ct && mkdir -p $S/ct
{ find java/test -name '*.java'; find java/src -name '*.java' -path '*/testing/*'; } | LC_ALL=C sort > $S/ct-files.txt
env -u JAVA_TOOL_OPTIONS javac -nowarn -Xlint:none -encoding UTF-8 -parameters -Xmaxerrs 1000 -d $S/ct -cp ".cache/build/main:.cache/build/stock-libs.jar:$(ls .cache/deps/*.jar | tr '\n' ':')" -proc:none @$S/ct-files.txt 2>&1 | grep -v '^Note:'
