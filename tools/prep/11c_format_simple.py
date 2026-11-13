#!/usr/bin/env python3
"""Prep step 11c: keep format strings to the conversions Go's fmt has.

Run from the repo root: python3 tools/prep/11c_format_simple.py

Every String.format / PrintStream.printf / PrintStream.format call in
production code (java/src outside */testing/) was listed; the only features
used that fmt.Sprintf lacks are `%n` and `%b`. Rules (idempotent):

  1. `%n` inside the format string of a printf call -> `\\n`
     (FILES_N: the five calls that use it).
  2. `%b` with a boolean argument -> `%s` with String.valueOf(argument)
     (PerformanceTracker, the only call that uses it).

Nothing else is changed: every other format string only uses %s, %d, %c,
%04X, %02d, %03d, %4d, %.1f and %%.
"""
import sys

FILES_N = [
    "java/src/com/google/javascript/jscomp/JvmMetrics.java",
    "java/src/com/google/javascript/jscomp/PrintStreamErrorManager.java",
    "java/src/com/google/javascript/jscomp/PrintStreamErrorReportGenerator.java",
]
N_FORMATS = [
    '"\\t%s jit time: %d ms%n"',
    '"%d error(s), %d warning(s), %.1f%% typed%n"',
    '"%d error(s), %d warning(s)%n"',
]
B_FILE = "java/src/com/google/javascript/jscomp/PerformanceTracker.java"
B_OLD = [
    '"%s,%d,%d,%b,%d,%d,%d,%d,%d,%d\\n",',
    "          stats.changes == 1,\n",
]
B_NEW = [
    '"%s,%d,%d,%s,%d,%d,%d,%d,%d,%d\\n",',
    "          String.valueOf(stats.changes == 1),\n",
]


def main():
    sites = 0
    for path in FILES_N:
        with open(path, encoding="utf-8") as fh:
            text = fh.read()
        for old in N_FORMATS:
            sites += text.count(old)
            text = text.replace(old, old[:-3] + '\\n"')
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(text)
    with open(B_FILE, encoding="utf-8") as fh:
        text = fh.read()
    if B_OLD[0] in text:
        assert text.count(B_OLD[0]) == 1 and text.count(B_OLD[1]) == 1
        for old, new in zip(B_OLD, B_NEW):
            text = text.replace(old, new)
        sites += 1
        with open(B_FILE, "w", encoding="utf-8") as fh:
            fh.write(text)
    print("11c_format_simple: rewrote %d calls" % sites)


if __name__ == "__main__":
    sys.exit(main())
