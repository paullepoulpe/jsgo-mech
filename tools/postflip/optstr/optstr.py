# Post-flip step: Optional<String> holds its string unboxed.
#
# Prep step 10 made nullable Strings Optional<String>. The runtime Optional is a value, but
# its content is an `any`, so every Optional.of(string) boxed the string. The runtime keeps a
# string content unboxed (Optional_ofStr_, getStr_, orElseStr_); this rewrites the call sites:
#   Optional_of(strobj_(E))         -> Optional_ofStr_(E)
#   Optional_ofNullable(strobj_(E)) -> Optional_ofNullableStr_(E)
#   str_(X.get())                   -> X.getStr_()          (X an Optional)
#   str_(X.orElse(strobj_(E)))      -> X.orElseStr_(E)      (X an Optional)
# For the last two the receiver's type is not known here: every match is rewritten, the package
# is compiled, and each site the compiler rejects (X not an Optional) is put back.
#
# usage: python3 optstr.py <dir of package closure>
import glob, os, re, subprocess, sys
d = os.path.abspath(sys.argv[1])

def balanced(s, i):
    """s[i] is '('; the index just past its matching ')'."""
    depth = 0
    while True:
        c = s[i]
        if c == '"':
            i += 1
            while s[i] != '"':
                i += 2 if s[i] == '\\' else 1
        elif c == '(':
            depth += 1
        elif c == ')':
            depth -= 1
            if depth == 0:
                return i + 1
        i += 1

def rewrite_calls(s, head, make):
    out, i, n = [], 0, 0
    for m in re.finditer(re.escape(head), s):
        if m.start() < i:
            continue
        open_ = m.end() - 1
        end = balanced(s, open_)
        inner = s[open_ + 1:end - 1]
        r = make(inner)
        if r is None:
            continue
        out.append(s[i:m.start()]); out.append(r); i, n = end, n + 1
    out.append(s[i:])
    return ''.join(out), n

def strobj_arg(inner):
    if inner.startswith('strobj_(') and balanced(inner, len('strobj_')) == len(inner):
        return inner[len('strobj_('):-1]
    return None

# receiver-typed rewrites keep a record so that rejected sites can be put back
sites = []  # (file, new text, old text)

def recv_get(inner):
    # inner is the argument of str_(...): X.get()  or  X.orElse(strobj_(E))
    if inner.endswith('.get()'):
        return inner[:-len('.get()')] + '.getStr_()'
    m = re.search(r'\.orElse\($', inner[:inner.rfind('.orElse(') + len('.orElse(')]) if '.orElse(' in inner else None
    k = inner.rfind('.orElse(')
    if k > 0 and balanced(inner, k + len('.orElse')) == len(inner):
        e = strobj_arg(inner[k + len('.orElse('):-1])
        if e is not None:
            return inner[:k] + '.orElseStr_(' + e + ')'
    return None

total = {}
for f in sorted(glob.glob(os.path.join(d, 'g_*.go'))):
    s = open(f).read()
    s, a = rewrite_calls(s, 'Optional_of(', lambda x: None if strobj_arg(x) is None else 'Optional_ofStr_(' + strobj_arg(x) + ')')
    s, b = rewrite_calls(s, 'Optional_ofNullable(', lambda x: None if strobj_arg(x) is None else 'Optional_ofNullableStr_(' + strobj_arg(x) + ')')
    def mk(inner, f=f):
        r = recv_get(inner)
        if r is not None:
            sites.append((f, r, 'str_(' + inner + ')'))
        return r
    s, c = rewrite_calls(s, 'str_(', mk)
    total['of'] = total.get('of', 0) + a; total['ofNullable'] = total.get('ofNullable', 0) + b; total['reads'] = total.get('reads', 0) + c
    open(f, 'w').write(s)

# put back the reads whose receiver is not an Optional, as the compiler reports them
for rnd in range(20):
    r = subprocess.run(['go', 'build', '-gcflags=-e', '.'], cwd=d, capture_output=True, text=True)
    bad = set()
    for line in r.stderr.splitlines():
        m = re.match(r'\./(g_\w+\.go):(\d+):\d+: .*(getStr_|orElseStr_)', line)
        if m:
            bad.add((os.path.join(d, m.group(1)), int(m.group(2))))
    if not bad:
        break
    for f, ln in sorted(bad):
        lines = open(f).read().split('\n')
        for (sf, new, old) in sites:
            if sf == f and new in lines[ln - 1]:
                lines[ln - 1] = lines[ln - 1].replace(new, old, 1)
                total['reads'] -= 1
                break
        open(f, 'w').write('\n'.join(lines))
print('Optional.of(string): %d, ofNullable(string): %d, string reads: %d' % (total['of'], total['ofNullable'], total['reads']))
r = subprocess.run(['go', 'build', '.'], cwd=d, capture_output=True, text=True)
print(r.stderr[:2000] or 'builds')
