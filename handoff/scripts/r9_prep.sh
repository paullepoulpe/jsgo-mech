#!/bin/bash
# resets the scratch copy of go/closure and applies the runtime additions
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; W=$S/r9w/go
rm -rf $W/closure && cp -r /home/user/jm-r7-erased/go/closure $W/closure && bash $S/r9_rt.sh $W/closure
