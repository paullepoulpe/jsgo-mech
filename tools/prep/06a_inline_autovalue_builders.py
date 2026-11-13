#!/usr/bin/env python3
"""Prep step 06a helper: turn @AutoValue.Builder / @AutoBuilder builders into plain classes.

One-shot script (kept for review; it is a no-op once no builder annotation is left).

Rule: for every abstract class or interface `Builder` annotated `@AutoValue.Builder` or
`@AutoBuilder`, take the builder that the AutoValue annotation processor generates for it
(AutoValue_<Outer>_<Class>.Builder or AutoBuilder_<Outer>_<Class>_Builder) and inline it into
the hand-written declaration:

  * the annotation is removed and the declaration becomes `static final class Builder`
    (visibility unchanged);
  * the generated private fields and constructors are inserted at the top of the class
    (`set$0` is spelled `set0`, `fooBuilder$` is spelled `fooBuilder`); the generated
    copy constructor `Builder(X source)` is kept only where the file has a `toBuilder()`
    that calls it;
  * every abstract method keeps its declared signature, in its declared position, and gets
    the generated method body (the generated parameter name is replaced with the declared
    one); methods of an interface builder become `public`;
  * `new AutoValue_X.Builder(...)` / `new AutoBuilder_X_Builder(...)` become
    `new Builder(...)`, and `new AutoValue_X(...)` inside the builder becomes `new X(...)`.

So null checks ("Null foo"), the "Missing required properties:" IllegalStateException, the
"Property \"foo\" has not been set" getters and the collection-builder defaults are exactly
the generated ones.

The generated sources are produced with (from the repo root, after tools/gate0/build.sh):

  javac -proc:only -s <gen-dir> -cp .cache/build/stock-libs.jar:<.cache/deps/*.jar> \
      -processorpath <.cache/deps/*.jar> @.cache/build/main-files.txt
  (and the same with @.cache/build/test-files.txt and .cache/build/main on the classpath)

Usage: python3 tools/prep/06a_inline_autovalue_builders.py <gen-dir> [<gen-dir> ...]
"""
import os
import re
import sys

ANNOT = re.compile(r"^\s*@(AutoValue\.Builder|AutoBuilder)\s*$")


def strip_code(line, state):
    """Returns line with string/char literals and comments blanked; state = in block comment."""
    out = []
    i = 0
    n = len(line)
    while i < n:
        c = line[i]
        if state[0]:
            if line.startswith("*/", i):
                state[0] = False
                i += 2
            else:
                i += 1
            continue
        if line.startswith("/*", i):
            state[0] = True
            i += 2
            continue
        if line.startswith("//", i):
            break
        if c == '"' or c == "'":
            q = c
            i += 1
            while i < n and line[i] != q:
                i += 2 if line[i] == "\\" else 1
            i += 1
            out.append(q + q)
            continue
        out.append(c)
        i += 1
    return "".join(out)


def members(lines, start, indent_hint=None):
    """Splits the class body whose `{` ends line `start` into depth-1 chunks.

    Returns (end_line_index_of_closing_brace, [(first, last)] line ranges).
    """
    depth = 1
    paren = 0
    state = [False]
    chunks = []
    cur = None
    i = start + 1
    while i < len(lines):
        raw = lines[i]
        was_comment = state[0]
        code = strip_code(raw, state)
        at_depth1 = depth == 1
        for ch in code:
            if ch == "{":
                depth += 1
            elif ch == "}":
                depth -= 1
            elif ch == "(":
                paren += 1
            elif ch == ")":
                paren -= 1
        if depth == 0:
            return i, chunks
        s = code.strip()
        if at_depth1 and cur is None and s and not was_comment:
            cur = i
        if cur is not None and depth == 1 and paren == 0 and (s.endswith(";") or s.endswith("}")):
            chunks.append((cur, i))
            cur = None
        i += 1
    raise ValueError("unbalanced")


def split_params(header):
    inner = header[header.index("(") + 1 : header.rindex(")")]
    parts = []
    depth = 0
    cur = ""
    for ch in inner:
        if ch in "<(":
            depth += 1
        elif ch in ">)":
            depth -= 1
        if ch == "," and depth == 0:
            parts.append(cur)
            cur = ""
        else:
            cur += ch
    if cur.strip():
        parts.append(cur)
    return [p.strip().split()[-1] for p in parts]


def method_name(header):
    return re.search(r"(\w+)\s*\(", header).group(1)


def rename_outside_strings(line, mapping):
    if not mapping:
        return line
    pieces = re.split(r'("(?:[^"\\]|\\.)*")', line)
    for k in range(0, len(pieces), 2):
        for old, new in mapping.items():
            pieces[k] = re.sub(r"(?<![\w.$])" + re.escape(old) + r"(?![\w$])", new, pieces[k])
    return "".join(pieces)


def clean(line):
    line = line.replace("set$0", "set0")
    return re.sub(r"(\w+Builder)\$", r"\1", line)


def parse_generated(path, nested):
    """Returns (fields, ctors, {(name, nparams): (params, body_lines)}) with indentation removed."""
    lines = open(path, encoding="utf-8").read().split("\n")
    if nested:
        start = next(i for i, l in enumerate(lines) if re.match(r"\s+static (final )?class Builder extends ", l))
    else:
        start = next(i for i, l in enumerate(lines) if re.match(r"(final )?class AutoBuilder_\w+ (extends|implements) ", l))
    end, chunks = members(lines, start)
    ind = len(lines[start]) - len(lines[start].lstrip()) + 2
    fields, ctors, methods = [], [], {}
    for a, b in chunks:
        text = [clean(l[ind:]) for l in lines[a : b + 1]]
        while text and text[0].strip().startswith("@"):
            text = text[1:]
        head = text[0]
        if re.match(r"private [^(=]*(=.*)?;$", head):
            fields.append(head)
        elif re.match(r"(AutoBuilder_\w+|Builder)\(", head):
            text[0] = re.sub(r"^(AutoBuilder_\w+|Builder)\(", "Builder(", head)
            ctors.append(text)
        else:
            params = split_params(head)
            methods[(method_name(head), len(params))] = (params, text[1:-1])
    return fields, ctors, methods


def gen_file(gen_dirs, pkg, outer_chain, kind):
    if kind == "AutoBuilder":
        name = "AutoBuilder_" + "_".join(outer_chain) + "_Builder.java"
    else:
        name = "AutoValue_" + "_".join(outer_chain) + ".java"
    for d in gen_dirs:
        for cand in (name, "$" + name):
            p = os.path.join(d, pkg.replace(".", "/"), cand)
            if os.path.exists(p) and "class Builder" in open(p, encoding="utf-8").read() or (
                os.path.exists(p) and kind == "AutoBuilder"
            ):
                return p
    raise FileNotFoundError(name)


def enclosing_chain(lines, idx):
    """Names of the type declarations enclosing line idx, outermost first."""
    chain = []
    depth = 0
    state = [False]
    stack = []
    pending = None
    for i in range(idx):
        code = strip_code(lines[i], state)
        m = re.search(r"\b(class|interface|record|enum)\s+(\w+)", code)
        if m:
            pending = m.group(2)
        for ch in code:
            if ch == "{":
                stack.append(pending)
                pending = None
            elif ch == "}":
                stack.pop()
            elif ch == ";":
                pending = None
    return [s for s in stack if s]


def process(path, gen_dirs):
    src = open(path, encoding="utf-8").read()
    lines = src.split("\n")
    changed = 0
    while True:
        idx = next((i for i, l in enumerate(lines) if ANNOT.match(l)), None)
        if idx is None:
            break
        kind = "AutoBuilder" if "AutoBuilder" in lines[idx] else "AutoValue"
        pkg = re.search(r"^package ([\w.]+);", "\n".join(lines), re.M).group(1)
        chain = enclosing_chain(lines, idx)
        gfile = gen_file(gen_dirs, pkg, chain, kind)
        fields, ctors, methods = parse_generated(gfile, nested=(kind == "AutoValue"))
        hdr = idx + 1
        m = re.match(r"^(\s*)((?:public |protected |private )?)(abstract static class|abstract interface|static interface|interface|abstract class) Builder\b(.*)\{\s*$", lines[hdr])
        if not m:
            raise ValueError(f"{path}:{hdr + 1}: unexpected builder header: {lines[hdr]}")
        is_interface = "interface" in m.group(3)
        ind = m.group(1) + "  "
        end, chunks = members(lines, hdr)
        new_body = [ind + f for f in fields]
        has_to_builder = re.search(r"_Builder\(this\)|abstract Builder toBuilder\(\)", src) is not None
        for c in ctors:
            if not c[0].startswith("Builder()") and not has_to_builder:
                continue
            new_body.append("")
            new_body.extend((ind + l) if l else l for l in c)
        new_body.append("")
        out = lines[:idx] + [f"{m.group(1)}{m.group(2)}static final class Builder{m.group(4)}{{"] + new_body
        pos = hdr + 1
        used = set()
        for a, b in chunks:
            # Skip leading annotation lines of the member.
            first = a
            while lines[first].strip().startswith("@") and "(" not in lines[first]:
                first += 1
            decl = " ".join(l.strip() for l in lines[first : b + 1])
            is_abstract = decl.endswith(";") and "(" in decl and "{" not in decl and "=" not in decl
            if not is_abstract:
                continue
            out.extend(lines[pos:first])
            params = split_params(decl)
            key = (method_name(decl), len(params))
            gparams, body = methods[key]
            used.add(key)
            mapping = {g: s for g, s in zip(gparams, params) if g != s}
            head_lines = lines[first : b + 1]
            head_lines[0] = re.sub(r"\babstract ", "", head_lines[0], count=1)
            if is_interface and not head_lines[0].strip().startswith("public "):
                head_lines[0] = ind + "public " + head_lines[0].strip()
            head_lines[-1] = re.sub(r";\s*$", " {", head_lines[-1])
            out.extend(head_lines)
            for l in body:
                l = rename_outside_strings(l, mapping)
                l = re.sub(r"\bnew AutoValue_(\w+)\(", lambda mm: "new " + chain[-1] + "(", l)
                out.append((ind + l) if l else l)
            out.append(ind + "}")
            pos = b + 1
        missing = set(methods) - used
        if missing:
            raise ValueError(f"{path}: generated methods without declaration: {missing}")
        out.extend(lines[pos:])
        lines = out
        changed += 1
    text = "\n".join(lines)
    text2 = re.sub(r"\bnew AutoValue_\w+\.Builder\(", "new Builder(", text)
    text2 = re.sub(r"\bnew AutoBuilder_\w+_Builder\(", "new Builder(", text2)
    if text2 != src:
        open(path, "w", encoding="utf-8").write(text2)
    return changed


def main():
    gen_dirs = sys.argv[1:]
    total = 0
    for root in ("java/src", "java/test"):
        for d, _, files in os.walk(root):
            for f in sorted(files):
                if f.endswith(".java"):
                    p = os.path.join(d, f)
                    if "@Auto" in open(p, encoding="utf-8").read():
                        n = process(p, gen_dirs)
                        if n:
                            print(f"{p}: {n} builder(s)")
                            total += n
    print(f"builders inlined: {total}")


if __name__ == "__main__":
    main()
