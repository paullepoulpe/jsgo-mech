#!/usr/bin/env python3
"""Prep step 08: no method-level type parameters on virtual methods.

A method that is abstract / overridden / implemented in more than one class
may not declare its own type parameter. Each such family (there are four in
java/src) gets a non-generic virtual method named <name>Untyped whose
signature is the erasure of the old one, and ONE non-virtual generic wrapper
that keeps the old name and the old static types for callers:

  JSType                 <T> T visit(Visitor<T> visitor)
      -> Object visitUntyped(Visitor<?> visitor)                    (root + overrides)
         public final <T> T visit(Visitor<T> visitor)               (JSType only)
  JSType                 <T> T visit(RelationshipVisitor<T> visitor, JSType that)
      -> Object visitUntyped(RelationshipVisitor<?> visitor, JSType that)
         final <T> T visit(RelationshipVisitor<T> visitor, JSType that)  (JSType only)
  AbstractCompiler       <T> T getIndex(Class<T> type)
      -> Object getIndexUntyped(Class<?> type)
         final <T> T getIndex(Class<T> type)                        (AbstractCompiler only)
  Annotatable            <A extends Annotation> A getAnnotation()
      -> Annotation getAnnotationUntyped()
         static <A extends Annotation> A getAnnotation(Annotatable annotatable)

Modifiers and annotations of every rewritten declaration are kept as they
are. Method bodies are kept too, except that a cast to the removed type
parameter is dropped (`return (A) annotation;` -> `return annotation;`, and
the IndexProvider<T> local in Compiler.getIndex becomes IndexProvider<?>).

The three class families keep the generic method as a final method on the
root class, so no caller changes. Annotatable is an interface and cannot
have a final method, so its wrapper is a static interface method and every
call `<recv>.getAnnotation()` becomes `Annotatable.getAnnotation(<recv>)`
(explicit type arguments move along: `a.<X>getAnnotation()` ->
`Annotatable.<X>getAnnotation(a)`), with the import added where the file is
outside com.google.javascript.jscomp.graph. JsDocInfoParserTest is skipped:
its getAnnotation() calls are JSDocInfo.Marker's unrelated method.

Idempotent. Run from the repository root:
  python3 -I tools/prep/08_nongeneric_virtuals.py
"""
import os, re

JSTYPE_DIRS = ("java/src/com/google/javascript/rhino/jstype", "java/test/com/google/javascript/rhino/jstype")
JSTYPE = "java/src/com/google/javascript/rhino/jstype/JSType.java"
ABSTRACT_COMPILER = "java/src/com/google/javascript/jscomp/AbstractCompiler.java"
COMPILER = "java/src/com/google/javascript/jscomp/Compiler.java"
ANNOTATABLE = "java/src/com/google/javascript/jscomp/graph/Annotatable.java"
SKIP_CALLS = ("java/test/com/google/javascript/jscomp/parsing/JsDocInfoParserTest.java",)
GRAPH_PKG = "com.google.javascript.jscomp.graph"
IMPORT = "import %s.Annotatable;" % GRAPH_PKG

# The root declarations: old text -> final generic wrapper + untyped virtual.
ROOTS = {
    JSTYPE: [
        (
            "  public abstract <T> T visit(Visitor<T> visitor);\n",
            "  @SuppressWarnings(\"unchecked\")\n"
            "  public final <T> T visit(Visitor<T> visitor) {\n"
            "    return (T) visitUntyped(visitor);\n"
            "  }\n"
            "\n"
            "  /** The virtual, non-generic form of {@link #visit(Visitor)}. */\n"
            "  public abstract Object visitUntyped(Visitor<?> visitor);\n",
        ),
        (
            "  abstract <T> T visit(RelationshipVisitor<T> visitor, JSType that);\n",
            "  @SuppressWarnings(\"unchecked\")\n"
            "  final <T> T visit(RelationshipVisitor<T> visitor, JSType that) {\n"
            "    return (T) visitUntyped(visitor, that);\n"
            "  }\n"
            "\n"
            "  /** The virtual, non-generic form of {@link #visit(RelationshipVisitor, JSType)}. */\n"
            "  abstract Object visitUntyped(RelationshipVisitor<?> visitor, JSType that);\n",
        ),
    ],
    ABSTRACT_COMPILER: [
        (
            "  abstract <T> T getIndex(Class<T> type);\n",
            "  @SuppressWarnings(\"unchecked\")\n"
            "  final <T> T getIndex(Class<T> type) {\n"
            "    return (T) getIndexUntyped(type);\n"
            "  }\n"
            "\n"
            "  /** The virtual, non-generic form of {@link #getIndex(Class)}. */\n"
            "  abstract Object getIndexUntyped(Class<?> type);\n",
        ),
    ],
    ANNOTATABLE: [
        (
            "  <A extends Annotation> A getAnnotation();\n",
            "  @SuppressWarnings(\"unchecked\")\n"
            "  static <A extends Annotation> A getAnnotation(Annotatable annotatable) {\n"
            "    return (A) annotatable.getAnnotationUntyped();\n"
            "  }\n"
            "\n"
            "  /** The virtual, non-generic form of {@link #getAnnotation(Annotatable)}. */\n"
            "  Annotation getAnnotationUntyped();\n",
        ),
    ],
}

# Overriding declarations (plain text; not applied to the root files, which hold the wrappers).
VISIT_OVERRIDES = [
    ("<T> T visit(Visitor<T> visitor) {", "Object visitUntyped(Visitor<?> visitor) {"),
    ("<T> T visit(RelationshipVisitor<T> visitor, JSType that) {",
     "Object visitUntyped(RelationshipVisitor<?> visitor, JSType that) {"),
]
COMPILER_OVERRIDE = [
    ("  <T> T getIndex(Class<T> key) {\n    IndexProvider<T> indexProvider = (IndexProvider<T>) indexProvidersByType.get(key);\n",
     "  Object getIndexUntyped(Class<?> key) {\n    IndexProvider<?> indexProvider = indexProvidersByType.get(key);\n"),
]
ANNOTATION_OVERRIDE = re.compile(
    r"<A extends Annotation> A getAnnotation\(\) \{(\s*)return \(A\) annotation;")
ANNOTATION_OVERRIDE_THROW = "<A extends Annotation> A getAnnotation() {"

CALL = re.compile(r"\.(<[A-Za-z_.]+>)?getAnnotation\(\)")
IDENT = re.compile(r"[A-Za-z_$][A-Za-z_$0-9]*$")


def receiver_start(s, end):
    """Start of the primary expression (names, calls, dots) that ends at s[end-1]."""
    i = end
    while True:
        if s[i - 1] == ")":
            depth, i = 1, i - 1
            while depth:
                i -= 1
                depth += {")": 1, "(": -1}.get(s[i], 0)
        m = IDENT.search(s, 0, i)
        if not m:
            raise ValueError("unsupported receiver before offset %d" % end)
        i = m.start()
        if s[i - 1] != ".":
            return i
        i -= 1


def rewrite_calls(s):
    while True:
        m = CALL.search(s)
        if not m:
            return s
        start = receiver_start(s, m.start())
        s = "%sAnnotatable.%sgetAnnotation(%s)%s" % (
            s[:start], m.group(1) or "", s[start:m.start()], s[m.end():])


def add_import(s):
    if IMPORT in s or re.search(r"^package %s;" % re.escape(GRAPH_PKG), s, re.M):
        return s
    lines = s.split("\n")
    plain = [i for i, l in enumerate(lines) if l.startswith("import ") and not l.startswith("import static ")]
    after = [i for i in plain if lines[i] > IMPORT]
    lines.insert(after[0] if after else plain[-1] + 1, IMPORT)
    return "\n".join(lines)


files = sites = 0
for root in ("java/src", "java/test"):
    for d, _, fs in os.walk(root):
        for f in sorted(fs):
            if not f.endswith(".java"):
                continue
            p = os.path.join(d, f).replace(os.sep, "/")
            old = s = open(p, encoding="utf-8").read()
            n = 0
            for a, b in ROOTS.get(p, []):
                n += s.count(a)
                s = s.replace(a, b)
            if d.replace(os.sep, "/") in JSTYPE_DIRS and p != JSTYPE:
                for a, b in VISIT_OVERRIDES:
                    n += s.count(a)
                    s = s.replace(a, b)
            if p == COMPILER:
                for a, b in COMPILER_OVERRIDE:
                    n += s.count(a)
                    s = s.replace(a, b)
            s, k = ANNOTATION_OVERRIDE.subn(r"Annotation getAnnotationUntyped() {\1return annotation;", s)
            n += k + s.count(ANNOTATION_OVERRIDE_THROW)
            s = s.replace(ANNOTATION_OVERRIDE_THROW, "Annotation getAnnotationUntyped() {")
            if p not in SKIP_CALLS and CALL.search(s):
                n += len(CALL.findall(s))
                s = add_import(rewrite_calls(s))
            k = s.count("GraphNode#getAnnotation()")
            n += k
            s = s.replace("GraphNode#getAnnotation()", "GraphNode#getAnnotationUntyped()")
            if s != old:
                open(p, "w", encoding="utf-8").write(s)
                files += 1
                sites += n
print("08_nongeneric_virtuals: %d files, %d sites rewritten" % (files, sites))
