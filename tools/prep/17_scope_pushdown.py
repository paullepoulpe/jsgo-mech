#!/usr/bin/env python3
"""Prep step 17: push the type-parameter-dependent members of the scope and variable base
classes down into their two subclasses.

AbstractScope<S extends AbstractScope<S, V>, V extends AbstractVar<S, V>> and AbstractVar<S, V>
are each instantiated exactly twice (Scope/Var and TypedScope/TypedVar). A translator that erases
S and V to the base classes has to cast at every use in subclass code, as javac does. This step
is the refactoring "push down members", applied mechanically:

  * a non-static member of the base whose code names S or V, or that uses a pushed-down field or
    private member, or a pushed-down method with S or V among its parameters, is pushed down
    (fixpoint);
  * each subclass gets a copy with S and V replaced by its own two types; the unchecked
    thisScope()/thisVar() casts become `this`;
  * the base keeps, for every pushed-down method that is not private and has no S or V among its
    parameters, an abstract declaration with S and V replaced by the base classes, so code that
    holds an AbstractScope or AbstractVar calls it as before; the copies override it with their
    narrower result types;
  * a base constructor parameter of type S that is only stored into a pushed-down field is
    removed, and each subclass constructor stores its argument into its own field;
  * where a subclass already overrides a pushed-down method (TypedScope.getVar), the copy is
    private, named <name>Super, and the subclass's super.<name>(...) calls call it;
  * a private member of the base that pushed-down code uses becomes package-private, and a base
    that was not abstract becomes abstract (nothing instantiates it);
  * the bases lose their type parameters; AbstractScope<?, ?> and AbstractVar<?, ?> elsewhere
    become AbstractScope and AbstractVar.

No behaviour changes: every method body is the one that ran before, in the class of the object
it ran on. Run from the repository root; it rewrites java/src and java/test in place and prints
what it moved.
"""
import os
import re
import sys

ROOT = 'java'
PKG = 'java/src/com/google/javascript/jscomp/'
FAMILY = {
    'AbstractScope': [('Scope', {'S': 'Scope', 'V': 'Var'}), ('TypedScope', {'S': 'TypedScope', 'V': 'TypedVar'})],
    'AbstractVar': [('Var', {'S': 'Scope', 'V': 'Var'}), ('TypedVar', {'S': 'TypedScope', 'V': 'TypedVar'})],
}
ERASED = {'S': 'AbstractScope', 'V': 'AbstractVar'}
SELF = {'AbstractScope': 'thisScope', 'AbstractVar': 'thisVar'}


def strip_code(text):
    """text without comments and string/char literals (for analysis only)."""
    text = re.sub(r'/\*.*?\*/', ' ', text, flags=re.S)
    text = re.sub(r'//[^\n]*', ' ', text)
    text = re.sub(r'"(\\.|[^"\\])*"', '""', text)
    text = re.sub(r"'(\\.|[^'\\])'", "' '", text)
    return text


class Member:
    def __init__(self, lead, decl):
        self.lead, self.decl = lead, decl  # comment/annotation lines; declaration lines
        code = strip_code('\n'.join(decl))
        self.code = code
        head = re.split(r'[{=;]', code, maxsplit=1)[0]
        self.head = head
        self.nested = bool(re.search(r'\b(class|enum|interface|record)\b', head))
        self.static = bool(re.search(r'\bstatic\b', head))
        self.private = bool(re.search(r'\bprivate\b', head))
        self.abstract = bool(re.search(r'\babstract\b', head))
        m = re.search(r'(\w+)\s*\(', head)
        self.method = m is not None and not self.nested
        if self.nested:
            self.name = re.search(r'\b(?:class|enum|interface|record)\s+(\w+)', head).group(1)
        elif m:
            self.name = m.group(1)
        else:
            self.name = re.findall(r'\w+', head)[-1]
        self.params = ''
        if self.method:
            depth, start = 0, head.index('(', m.start())
            for i in range(start, len(head)):
                depth += head[i] == '('
                depth -= head[i] == ')'
                if depth == 0:
                    self.params = head[start:i + 1]
                    break

    def text(self):
        return '\n'.join(self.lead + self.decl)


def parse(path):
    """(lines before the class body, members, lines after) of the top-level class of path."""
    lines = open(path).read().split('\n')
    start = next(i for i, l in enumerate(lines) if re.match(r'^(public |abstract |final )*(class|interface) ', l))
    open_ = next(i for i in range(start, len(lines)) if lines[i].rstrip().endswith('{'))
    close = max(i for i, l in enumerate(lines) if l == '}')
    body = lines[open_ + 1:close]
    members, lead, i = [], [], 0
    while i < len(body):
        l = body[i]
        s = l.strip()
        if s == '' or s.startswith('//') or s.startswith('@') and not re.search(r'[;{]\s*$', s) and re.match(r'^  @\w+(\([^)]*\))?$', l):
            lead.append(l)
            i += 1
            continue
        if s.startswith('/*'):
            while True:
                lead.append(body[i])
                i += 1
                if '*/' in body[i - 1]:
                    break
            continue
        decl, depth = [], 0
        while True:
            decl.append(body[i])
            c = strip_code(body[i])
            depth += c.count('{') + c.count('(') - c.count('}') - c.count(')')
            i += 1
            if depth == 0 and re.search(r'[;}]\s*$', c):
                break
        # Blank lines before a member stay outside it.
        while lead and lead[0].strip() == '':
            members.append(('gap', lead.pop(0)))
        members.append(('member', Member(lead, decl)))
        lead = []
    for l in lead:
        members.append(('gap', l))
    return lines[:open_ + 1], members, lines[close:]


def names_tp(code):
    return bool(re.search(r'\b[SV]\b', code))


def erase(text):
    text = re.sub(r'<([SV])>', lambda m: '<? extends ' + ERASED[m.group(1)] + '>', text)
    return re.sub(r'\b([SV])\b', lambda m: ERASED[m.group(1)], text)


def specialize(text, base, env):
    text = re.sub(r'\b' + base + r'<S, V>', 'S', text)
    text = re.sub(r'\b([SV])\b', lambda m: env[m.group(1)], text)
    return re.sub(r'\b' + SELF[base] + r'\(\)', 'this', text)


def imports_of(lines):
    out = {}
    for l in lines:
        m = re.match(r'^import (static )?([\w.]+)\.(\w+);$', l)
        if m:
            out[m.group(3)] = l
    return out


def main():
    report = []
    for base, subs in FAMILY.items():
        path = PKG + base + '.java'
        pre, members, post = parse(path)
        ms = [m for k, m in members if k == 'member']
        inst = [m for m in ms if not m.static and not m.nested]
        ctor = next(m for m in inst if m.name == base)
        down = {id(m) for m in inst if m is not ctor and names_tp(m.code)}
        changed = True
        while changed:
            changed = False
            for m in inst:
                if id(m) in down or m is ctor:
                    continue
                for d in inst:
                    if id(d) not in down or not re.search(r'\b' + d.name + r'\b', m.code):
                        continue
                    if not d.method or d.private or names_tp(d.params):
                        down.add(id(m))
                        changed = True
                        break
        pushed = [m for m in inst if id(m) in down]
        # The constructor: a parameter of type S stored into a pushed-down field.
        ctor_fields = []  # (parameter index, field name)
        ctext = ctor.text()
        params = [p.strip() for p in re.search(r'\((.*?)\)\s*\{', strip_code(ctext), re.S).group(1).split(',')]
        for i, p in enumerate(params):
            if not names_tp(p):
                continue
            pname = p.split()[-1]
            m = re.search(r'\n\s*this\.(\w+) = ' + pname + r';', ctext)
            assert m and any(d.name == m.group(1) and id(d) in down for d in inst), (base, p)
            ctor_fields.append((i, m.group(1)))
            ctext = ctext.replace(m.group(0), '', 1)
            ctext = re.sub(r'\n\s*(@Nullable )?[SV] ' + pname + r',', '', ctext, count=1)
        assert not names_tp(strip_code(ctext)), ctext
        # A private member that stays and that pushed-down code uses becomes package-private (the
        # subclasses are in the base's package).
        pushed_code = '\n'.join(m.code for m in pushed)
        opened = []
        for m in ms:
            if id(m) not in down and m is not ctor and m.private and re.search(r'\b' + m.name + r'\b', pushed_code):
                for i, l in enumerate(m.decl):
                    if 'private ' in l:
                        m.decl[i] = l.replace('private ', '', 1)
                        break
                opened.append(m.name)
        # The base: independent members, abstract declarations of the pushed-down methods.
        out = []
        for kind, m in members:
            if kind == 'gap':
                out.append(m)
            elif m is ctor:
                out.append(ctext)
            elif id(m) not in down:
                out.append(m.text())
            elif m.abstract:
                out.append('\n'.join(m.lead) + ('\n' if m.lead else '') + erase('\n'.join(m.decl)))
            elif m.method and not m.private and not names_tp(m.params) and m.name != SELF[base]:
                head = '\n'.join(m.decl).split('{', 1)[0].rstrip()
                head = re.sub(r'\bfinal ', '', erase(head))
                head = re.sub(r'^(\s*)((?:public |protected )?)', r'\1\2abstract ', head, count=1)
                lead = [l for l in m.lead if not re.match(r'^\s*@(Override|SuppressWarnings)', l)]
                out.append('\n'.join(lead + [head + ';']))
        header = '\n'.join(pre)
        header = re.sub(r'\b' + base + r'<S extends AbstractScope<S, V>, V extends AbstractVar<S, V>>', base, header)
        if not re.search(r'\babstract class ' + base + r'\b', header):  # it has abstract methods now
            header = re.sub(r'\bclass ' + base + r'\b', 'abstract class ' + base, header)
        body = '\n'.join(out)
        body = re.sub(r'\n{3,}', '\n\n', body)
        used = strip_code(header.split('\npublic ')[-1] + body)
        hdr_lines = [l for l in header.split('\n')
                     if not (re.match(r'^import ', l) and not re.search(r'\b' + re.escape(re.match(r'^import (?:static )?[\w.]+\.(\w+);', l).group(1)) + r'\b', used))]
        open(path, 'w').write('\n'.join(hdr_lines) + '\n' + body.rstrip('\n') + '\n' + '\n'.join(post))
        base_imports = imports_of(pre)
        for sub, env in subs:
            spath = PKG + sub + '.java'
            s = open(spath).read()
            s = re.sub(r'extends ' + base + r'<\w+, \w+>', 'extends ' + base, s)
            code = strip_code(s)
            copies = []
            for m in pushed:
                if m.abstract or m.name == SELF[base]:
                    continue
                t = specialize(m.text(), base, env)
                overridden = m.method and re.search(r'^  (?! )[^\n(=;]*\b' + m.name + r'\(', code, re.M)
                if overridden:
                    t = re.sub(r'\b' + m.name + r'\(', m.name + 'Super(', t, count=1)
                    t = re.sub(r'^(\s*)@Override\n', '', t, flags=re.M)
                    t = re.sub(r'^(\s*)(public |protected )?(final )?', r'\1private ', t.lstrip('\n'), count=1) if not t.lstrip().startswith(('/', '@')) else \
                        re.sub(r'\n  (public |protected )?(final )?(?=[^\n]*\b' + m.name + r'Super\()', '\n  private ', t, count=1)
                    s = re.sub(r'\bsuper\.' + m.name + r'\(', m.name + 'Super(', s)
                    report.append('%s.%s: the copy is %sSuper (the class overrides %s)' % (sub, m.name, m.name, m.name))
                elif m.method and not m.private and not names_tp(m.params) and not any('@Override' in l for l in m.lead):
                    lines = t.split('\n')
                    k = next(i for i, l in enumerate(lines) if l.startswith('  ') and not l.strip().startswith(('/', '*', '@')))
                    lines.insert(k, '  @Override')
                    t = '\n'.join(lines)
                copies.append(t)
            for i, f in sorted(ctor_fields, reverse=True):
                def fix(mm):
                    args, depth, cur = [], 0, ''
                    for ch in mm.group(2):
                        if ch == ',' and depth == 0:
                            args.append(cur)
                            cur = ''
                            continue
                        depth += ch in '(<'
                        depth -= ch in ')>'
                        cur += ch
                    args.append(cur)
                    arg = args.pop(i).strip()
                    return '%ssuper(%s);\n%sthis.%s = %s;' % (mm.group(1), ','.join(args).strip(), mm.group(1), f, arg)
                s, n = re.subn(r'( +)super\(([^;]*)\);', fix, s)
                assert n >= 1, (sub, 'super call')
            end = s.rindex('\n}')
            block = '\n\n  // Members pushed down from ' + base + ' (prep 17), with its type parameters replaced.\n\n' + \
                re.sub(r'\n{3,}', '\n\n', '\n\n'.join(c.strip('\n') for c in copies))
            s = s[:end] + block + s[end:]
            mine = imports_of(s.split('\n'))
            used = strip_code(block)
            add = [l for n, l in base_imports.items() if n not in mine and re.search(r'\b' + n + r'\b', used)]
            if add:
                lines = s.split('\n')
                k = next(i for i, l in enumerate(lines) if l.startswith('import '))
                lines[k:k] = add
                imps = sorted({l for l in lines if l.startswith('import ')}, key=lambda l: (not l.startswith('import static'), l))
                first = next(i for i, l in enumerate(lines) if l.startswith('import '))
                last = max(i for i, l in enumerate(lines) if l.startswith('import '))
                stat = [l for l in imps if l.startswith('import static')]
                rest = [l for l in imps if not l.startswith('import static')]
                lines[first:last + 1] = stat + ([''] if stat and rest else []) + rest
                s = '\n'.join(lines)
            open(spath, 'w').write(s)
        if opened:
            report.append('%s: private members now package-private: %s' % (base, ', '.join(opened)))
        report.append('%s: %d members pushed down (%s); %d stay' % (
            base, len(pushed), ', '.join(m.name for m in pushed), len(inst) - len(pushed)))
    n = 0
    for d, _, fs in os.walk(ROOT):
        for f in fs:
            if not f.endswith('.java'):
                continue
            p = os.path.join(d, f)
            s = open(p).read()
            t = re.sub(r'\b(AbstractScope|AbstractVar)<\?, \?>', r'\1', s)
            if t != s:
                open(p, 'w').write(t)
                n += 1
    report.append('wildcard uses rewritten in %d files' % n)
    print('\n'.join(report))


if __name__ == '__main__':
    sys.exit(main())
