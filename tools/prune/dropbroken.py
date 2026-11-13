#!/usr/bin/env python3
"""Deletes from the handwritten runtime every top-level declaration that no longer compiles.

After a Java prep step removes a library type from the translated code, the runtime
functions that mention that type stop compiling (the type's declaration is generated on
demand). They are dead by construction, so this removes them: each compile error inside a
go/closure/rt_*.go file deletes the enclosing top-level func/type/var/const declaration, and
the build is repeated until it is clean. Errors in generated files are left for the caller.

CAUTION: a declaration can fail to compile because of one dead branch while the rest of it is
live (an equals() that also handled a removed type). Every removed declaration is printed;
check each one that is a method of a type that still exists, and put it back without the
dead branch.

Run from go/:  python3 ../tools/prune/dropbroken.py [--tests]
"""
import collections, re, subprocess, sys

tests = "--tests" in sys.argv
total = 0
for rnd in range(200):
    if tests:
        out = subprocess.run(["go", "vet", "./closure"], capture_output=True, text=True).stderr
    else:
        out = subprocess.run(["go", "build", "-gcflags=-e", "./closure"], capture_output=True, text=True).stderr
    errs = collections.defaultdict(set)
    pat = r"(closure/rt_\w+_test\.go):(\d+):\d+: (.*)" if tests else r"(closure/rt_\w+\.go):(\d+):\d+: (.*)"
    for m in re.finditer(pat, out):
        if not tests and m.group(1).endswith("_test.go"):
            continue
        errs[m.group(1)].add((int(m.group(2)), m.group(3)))
    if not errs:
        break
    for f, es in errs.items():
        lines = open(f).read().split("\n")
        dead = set()
        for ln, msg in es:
            i = ln - 1
            if "imported and not used" in msg:
                dead.add(i)
                continue
            s = i
            while s > 0 and not re.match(r"^(func|type|var|const)\b", lines[s]):
                s -= 1
            e = s
            if not re.match(r"^(func|type|var|const)\b.*[^{(]$", lines[s]) or lines[s].rstrip().endswith(("{", "(")):
                if lines[s].rstrip().endswith(("{", "(")):
                    while e < len(lines) - 1 and not re.match(r"^[})]", lines[e]):
                        e += 1
            while s > 0 and lines[s - 1].startswith("//"):
                s -= 1
            if not dead.issuperset(range(s, e + 1)):
                print("  removed", f, next((l for l in lines[s:e + 1] if not l.startswith("//")), "")[:100])
                total += 1
            dead.update(range(s, e + 1))
        open(f, "w").write("\n".join(l for i, l in enumerate(lines) if i not in dead))
print("removed", total, "declarations in", rnd, "rounds")
