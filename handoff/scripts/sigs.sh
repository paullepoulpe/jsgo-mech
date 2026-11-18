#!/bin/bash
# usage: sigs.sh file names...
f=$1; shift
for X in "$@"; do
  echo "=== $X"
  awk -v X="$X" '
    $0 ~ "^type "X" (struct|interface)" {p=1}
    p {print; if ($0=="}") p=0}
  ' $f
  grep -E "^func \(this \*$X\) |^func new_${X}(_|\()|^func ${X}_[a-zA-Z0-9_]*\(|^var ${X}_|^func (to|is)_$X\(" $f | grep -vE "\) (as_$X\(\)|self\(\))" | sed 's/ {$//'
done
