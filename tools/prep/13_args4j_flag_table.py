#!/usr/bin/env python3
"""Prep step 13b: turn args4j @Option/@Argument annotations into explicit flag-table entries.

For each of the four classes under java/src that used args4j, this script

  1. removes every `@Option(...)` annotation (and the `@Argument` marker) from the field it
     annotates; the field declaration itself is left untouched;
  2. replaces the statement `... = new CmdLineParser(this);` by `... = new CmdLineParser();`
     followed by one table entry per former annotation, in the declaration order of the fields:

         parser
             .option(<name>)
             .aliases(<aliases>)        only if the annotation had aliases
             .hidden(true)              only if the annotation had hidden = true
             .required(true)            only if the annotation had required = true
             .usage(<usage>)            only if the annotation had a usage; expression kept verbatim
             .<kind>(() -> <field>, v -> { <field> = v; });

     and, for a field that had `@Argument`, a final `parser.argument(() -> f, v -> { f = v; });`

     <kind> is derived from the annotation's handler, or else from the type of the field:

         handler = BooleanOptionHandler.class          bool        (optional true/false argument)
         boolean / Boolean without a handler           flag        (no argument)
         String                                        string
         List<String>                                  stringList
         int / Integer (also IntOptionHandler)         integer
         Map<String, String>                           map
         List<E>, E an enum                            enumList(E.values(), ...)
         any other type E (an enum)                    enumValue(E.values(), ...)
         handler = one of MULTI_FLAG_HANDLERS          stringEach(v -> f = addMultiFlagValue(f, v,
                                                           <kind>, <order list>))

  3. removes the `import org.kohsuke.args4j...;` lines; a class outside the package
     com.google.javascript.jscomp imports CmdLineException and CmdLineParser from there instead.

Everything else of step 13b (the CmdLineParser and CmdLineException classes that replace args4j,
the removal of the args4j handler classes from CommandLineRunner, the callers of printUsage) was
written by hand.

Idempotent: a file without `@Option(` is left alone.
Run from the repository root: python3 -I tools/prep/13_args4j_flag_table.py
"""
import re

FILES = [
    "java/src/com/google/javascript/jscomp/CommandLineRunner.java",
    "java/src/com/google/javascript/jscomp/LinterMain.java",
    "java/src/com/google/javascript/jscomp/MapDiagnosticToSuppression.java",
    "java/src/com/google/javascript/jscomp/instrumentation/reporter/ProductionInstrumentationReporter.java",
]

# handler class -> the two extra arguments of addMultiFlagValue
MULTI_FLAG_HANDLERS = {
    "WarningGuardErrorOptionHandler": "CheckLevel.ERROR, guardLevels",
    "WarningGuardWarningOptionHandler": "CheckLevel.WARNING, guardLevels",
    "WarningGuardOffOptionHandler": "CheckLevel.OFF, guardLevels",
    "JsOptionHandler": "JsSourceType.JS, mixedJsSources",
    "JsZipOptionHandler": "JsSourceType.JS_ZIP, mixedJsSources",
}

MAX_COLUMN = 100


def skip_literal(s, i):
    """s[i] is a double quote; returns the index just after the string literal or text block."""
    if s.startswith('"""', i):
        return s.index('"""', i + 3) + 3
    i += 1
    while s[i] != '"':
        i += 2 if s[i] == "\\" else 1
    return i + 1


def matching_paren(s, i):
    """s[i] is '('; returns the index of the matching ')'."""
    depth = 0
    while True:
        c = s[i]
        if c == '"':
            i = skip_literal(s, i)
            continue
        if c == "/" and s[i + 1] == "/":
            i = s.index("\n", i)
            continue
        if c in "({":
            depth += 1
        elif c in ")}":
            depth -= 1
            if depth == 0:
                return i
        i += 1


def split_attributes(body):
    """Splits `a = x, b = y` at top-level commas into {a: (offset of x in body, x)}."""
    attrs, depth, i, start = {}, 0, 0, 0
    pieces = []
    while i < len(body):
        c = body[i]
        if c == '"':
            i = skip_literal(body, i)
            continue
        if c == "/" and body[i + 1] == "/":
            end = body.index("\n", i)
            body = body[:i] + " " * (end - i) + body[end:]  # blank out the comment
            continue
        if c in "({":
            depth += 1
        elif c in ")}":
            depth -= 1
        elif c == "," and depth == 0:
            pieces.append((start, body[start:i]))
            start = i + 1
        i += 1
    pieces.append((start, body[start:]))
    for offset, piece in pieces:
        if not piece.strip():
            continue
        key, value = piece.split("=", 1)
        lead = len(value) - len(value.lstrip())
        attrs[key.strip()] = (offset + len(key) + 1 + lead, value.strip())
    return attrs


def reindent(expr, column, indent):
    """Shifts a possibly multi-line expression whose first line started at `column` to `indent`."""
    lines = expr.split("\n")
    delta = indent - column
    out = [" " * indent + lines[0]]
    for line in lines[1:]:
        if not line.strip():
            out.append("")
        elif delta >= 0:
            out.append(" " * delta + line)
        else:
            assert line[:-delta].strip() == "", line
            out.append(line[-delta:])
    return "\n".join(out)


def call(indent, name, args):
    """Formats `.name(args)` on one line if it fits, else with one argument per line."""
    one = " " * indent + "." + name + "(" + ", ".join(args) + ")"
    if len(one) <= MAX_COLUMN - 1 and "\n" not in one:
        return one
    lines = []
    for a in args:
        m = re.fullmatch(r"(v ->) (\w+ =) (.*)", a)
        if m and indent + 4 + len(a) > MAX_COLUMN - 2:
            # An assignment lambda that does not fit on one line is broken after `->` and `=`.
            a = "%s\n%s%s\n%s%s" % (
                m.group(1), " " * (indent + 8), m.group(2), " " * (indent + 12), m.group(3))
        lines.append(" " * (indent + 4) + a)
    return " " * indent + "." + name + "(\n" + ",\n".join(lines) + ")"


def kind_call(indent, handler, type_, field):
    get, set_ = "() -> " + field, "v -> { " + field + " = v; }"
    if handler in MULTI_FLAG_HANDLERS:
        assert type_ == "List<String>", (field, type_)
        add = "v -> { %s = addMultiFlagValue(%s, v, %s); }" % (field, field, MULTI_FLAG_HANDLERS[handler])
        return call(indent, "stringEach", [add])
    if handler == "BooleanOptionHandler":
        assert type_ in ("boolean", "Boolean"), (field, type_)
        return call(indent, "bool", [get, set_])
    assert handler in (None, "IntOptionHandler"), (field, handler)
    if type_ in ("boolean", "Boolean"):
        return call(indent, "flag", [get, set_])
    if type_ == "String":
        return call(indent, "string", [get, set_])
    if type_ == "List<String>":
        return call(indent, "stringList", [get, set_])
    if type_ in ("int", "Integer"):
        return call(indent, "integer", [get, set_])
    if type_ == "Map<String, String>":
        return call(indent, "map", [get, set_])
    m = re.fullmatch(r"List<([\w.]+)>", type_)
    if m:
        return call(indent, "enumList", [m.group(1) + ".values()", get, set_])
    assert re.fullmatch(r"[\w.]+", type_), (field, type_)
    return call(indent, "enumValue", [type_ + ".values()", get, set_])


FIELD = re.compile(r"\s*private\s+(?:@Nullable\s+)?([\w.]+(?:<[\w., ]+>)?)\s+(\w+)\s*[=;]")


def convert(path):
    s = open(path, encoding="utf-8").read()
    if "@Option(" not in s:
        return 0
    m = re.search(r"^( *)(.*)new CmdLineParser\(this\);\n", s, re.M)
    indent = len(m.group(1))
    entries, out, pos = [], [], 0
    while True:
        at = s.find("@Option(", pos)
        if at < 0:
            break
        line_start = s.rindex("\n", 0, at) + 1
        assert s[line_start:at].strip() == "", s[line_start:at]
        open_paren = at + len("@Option")
        close = matching_paren(s, open_paren)
        body = s[open_paren + 1 : close]
        attrs = split_attributes(body)
        assert set(attrs) <= {"name", "aliases", "hidden", "required", "usage", "handler"}, attrs
        field = FIELD.match(s, close + 1)
        assert field, s[close : close + 80]
        type_, name = field.group(1), field.group(2)

        entry = [" " * indent + "parser", " " * (indent + 4) + ".option(" + attrs["name"][1] + ")"]
        if "aliases" in attrs:
            aliases = attrs["aliases"][1]
            assert aliases[0] == "{" and aliases[-1] == "}", aliases
            entry.append(call(indent + 4, "aliases", [a.strip() for a in aliases[1:-1].split(",")]))
        for flag in ("hidden", "required"):
            if flag in attrs:
                assert attrs[flag][1] == "true", attrs
                entry.append(" " * (indent + 4) + "." + flag + "(true)")
        if "usage" in attrs:
            offset, usage = attrs["usage"]
            absolute = open_paren + 1 + offset
            column = absolute - (s.rindex("\n", 0, absolute) + 1)
            one = " " * (indent + 4) + ".usage(" + usage + ")"
            if "\n" not in usage and len(one) <= MAX_COLUMN - 1:
                entry.append(one)
            else:
                entry.append(
                    " " * (indent + 4) + ".usage(\n" + reindent(usage, column, indent + 8) + ")"
                )
        handler = attrs["handler"][1][: -len(".class")] if "handler" in attrs else None
        entry.append(kind_call(indent + 4, handler, type_, name) + ";")
        entries.append("\n".join(entry))

        out.append(s[pos:line_start])
        pos = close + 1
        assert s[pos] == "\n"
        pos += 1
    out.append(s[pos:])
    s = "".join(out)

    m = re.search(r"^( *)@Argument private List<String> (\w+)", s, re.M)
    if m:
        s = s[: m.start()] + m.group(1) + s[m.start() + len(m.group(1)) + len("@Argument ") :]
        entries.append(
            " " * indent + "parser.argument(() -> %s, v -> { %s = v; });" % (m.group(2), m.group(2))
        )

    m = re.search(r"^( *)(.*)new CmdLineParser\(this\);\n", s, re.M)
    s = (
        s[: m.start()]
        + m.group(1)
        + m.group(2)
        + "new CmdLineParser();\n"
        + "\n".join(entries)
        + "\n"
        + s[m.end() :]
    )
    if "\npackage com.google.javascript.jscomp;\n" not in s:
        # The replacement classes live in com.google.javascript.jscomp.
        s = s.replace(
            "import org.kohsuke.args4j.CmdLineException;\nimport org.kohsuke.args4j.CmdLineParser;\n",
            "",
        )
        s = s.replace(
            "import com.google.javascript.jscomp.",
            "import com.google.javascript.jscomp.CmdLineException;\n"
            "import com.google.javascript.jscomp.CmdLineParser;\n"
            "import com.google.javascript.jscomp.",
            1,
        )
    s = re.sub(r"^import org\.kohsuke\.args4j\.[\w.]+;\n", "", s, flags=re.M)
    open(path, "w", encoding="utf-8").write(s)
    return len(entries)


total = 0
for f in FILES:
    n = convert(f)
    print("%s: %d table entries" % (f, n))
    total += n
print("total: %d" % total)
