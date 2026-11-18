#!/bin/bash
# show.sh <file of "path:line:col: reason"> <pattern>
grep ": .*$2" "$1" | grep closure/ | while IFS=: read f l c rest; do f=/home/user/jm-r7-strings/go/closure/$(basename $f); echo "$(basename $f):$l: $(sed -n ${l}p $f | sed 's/^[ \t]*//' | cut -c1-180)"; done
