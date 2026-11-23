#!/usr/bin/env python3
"""fnpatch.py PATCH OUT: a runtime patch of the post-flip history rewritten for the translator's rule "function types".

The patches were written when the 15 library functional interfaces (Supplier, Function, Comparator, ...) were erased Go
interfaces. With the rule they are generic func types, and the runtime of main names their erased shape X_e (= X[any, ...],
g_zstubs.go) wherever it wrote X: main's runtime commit 1eb3e4f7 is that substitution over rt_*.go. This script makes the same
substitution in a patch: in the code (not the comments) of every context, removed and added line of the hunks of go/closure/rt_*.go
files, a bare interface name X becomes X_e. Hunks of generated files (g_*.go, in manual patches) are left as they are.

So that `git apply --3way` still works on the result, the pre-image blob each file section names is rewritten the same way and
stored (git hash-object -w), and the section's index line names that blob. Run inside the repository. Prints the number of lines
changed; writes OUT only when it is not 0.
"""
import re, subprocess, sys

NAMES = ("Supplier_base Function_base Predicate_base Supplier Function Predicate Consumer BiFunction BiConsumer BiPredicate "
         "ToIntFunction ToLongFunction IntFunction Comparator Callable").split()
RE = re.compile(r'(?<![\w.&"])(' + '|'.join(NAMES) + r')(?![\w\[".])')

def code(line):
    i = line.find('//')
    head, tail = (line, '') if i < 0 else (line[:i], line[i:])
    return RE.sub(r'\1_e', head) + tail

def blob(h, is_rt):
    r = subprocess.run(['git', 'cat-file', 'blob', h], capture_output=True)
    if r.returncode != 0:
        return None
    text = r.stdout.decode('utf-8', 'surrogateescape')
    if is_rt:
        text = ''.join(code(l) for l in text.splitlines(True))
    w = subprocess.run(['git', 'hash-object', '-w', '--stdin'], input=text.encode('utf-8', 'surrogateescape'), capture_output=True, check=True)
    return w.stdout.decode().strip()

def main():
    src, out = sys.argv[1], sys.argv[2]
    lines = open(src, encoding='utf-8', errors='surrogateescape').read().splitlines(True)
    res, changed, is_rt, inhunk = [], 0, False, False
    for l in lines:
        if l.startswith('diff --git '):
            is_rt = re.search(r' b/go/closure/rt_[^/ ]*\.go$', l.rstrip('\n')) is not None
            inhunk = False
            res.append(l); continue
        if l.startswith('@@'):
            inhunk = True
            res.append(l); continue
        if not inhunk:
            m = re.match(r'index ([0-9a-f]+)\.\.([0-9a-f]+)(.*)$', l.rstrip('\n'))
            if m and is_rt and set(m.group(1)) != {'0'}:
                h = blob(m.group(1), True)
                if h:
                    l = 'index %s..%s%s\n' % (h, m.group(2), m.group(3))
            res.append(l); continue
        if is_rt and l[:1] in ' +-':
            n = l[:1] + code(l[1:])
            if n != l:
                changed += 1
            l = n
        res.append(l)
    print(changed)
    if changed:
        open(out, 'w', encoding='utf-8', errors='surrogateescape').write(''.join(res))

main()
