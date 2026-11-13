#!/usr/bin/env python3
"""Prep step 11b: switch production code from java.util.regex to com.google.re2j.

Run from the repo root: python3 tools/prep/11b_regex_re2.py

Rule (applied to every .java file under java/src that is not in a */testing/
directory; idempotent):

  1. `import java.util.regex.X;` is removed and `import com.google.re2j.X;` is
     inserted at its ASCII-sorted position among the non-static imports.
  2. Any remaining fully qualified `java.util.regex.` reference in the file is
     rewritten to `com.google.re2j.`.

The same rule is applied to the test files listed in TEST_FILES: they hand a
Pattern to, or take one from, production API whose type changed.

The script changes nothing else. The sites that needed a hand edit (String
methods that take a regex, patterns RE2 does not accept, API that re2j lacks)
are listed in the commit message of the step.
"""
import os
import re
import sys

IMPORT = re.compile(r"^import java\.util\.regex\.(\w+);\n", re.M)
ANY_IMPORT = re.compile(r"^import (?!static )([\w.]+);$")


TEST_FILES = [
    "java/test/com/google/javascript/jscomp/CompilerOptionsTest.java",
    "java/test/com/google/javascript/jscomp/ProcessDefinesTest.java",
]


def rewrite(text):
    names = IMPORT.findall(text)
    if names:
        text = IMPORT.sub("", text)
        lines = text.split("\n")
        for name in sorted(names):
            new = "com.google.re2j." + name
            if ("import %s;" % new) in lines:
                continue
            last = None
            at = None
            for i, line in enumerate(lines):
                m = ANY_IMPORT.match(line)
                if not m:
                    continue
                last = i
                if m.group(1) > new:
                    at = i
                    break
            if at is None:
                at = last + 1
            lines.insert(at, "import %s;" % new)
        text = "\n".join(lines)
    return text.replace("java.util.regex.", "com.google.re2j.")


def main():
    paths = list(TEST_FILES)
    for root, _, files in os.walk("java/src"):
        if "/testing" in root:
            continue
        for f in sorted(files):
            if f.endswith(".java"):
                paths.append(os.path.join(root, f))
    changed = 0
    for path in paths:
        with open(path, encoding="utf-8") as fh:
            old = fh.read()
        new = rewrite(old)
        if new != old:
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(new)
            changed += 1
    print("11b_regex_re2: rewrote %d files" % changed)


if __name__ == "__main__":
    sys.exit(main())
