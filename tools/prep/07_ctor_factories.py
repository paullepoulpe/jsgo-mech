#!/usr/bin/env python3
"""Prep step 07, constructors: call sites of same-arity constructors that became static factories.

Where a class had two constructors with the same parameter count, one of them was turned by hand
into a static factory method `createFromX(...)` (see the commit message for the rule). This script
rewrites the call sites: for every constructor in FACTORIES it replaces `new T` of each
`new T(...)` that javac resolved to that constructor with `T.<factory>`. The call sites and their
offsets come from the "CTORUSE" lines of the scanner, which must be run on the tree BEFORE the
constructors are edited:

  java tools/prep/07_find_same_arity_overloads/Scan.java > /tmp/scan07.txt
  python3 tools/prep/07_ctor_factories.py /tmp/scan07.txt

Idempotent: once a constructor is gone the scanner no longer lists it, so nothing is rewritten.
`this(...)` calls and constructor references (T::new) have no `new T(` and are edited by hand.
"""
import collections
import sys

J = "com.google.javascript.jscomp."
R = "com.google.javascript.rhino."
T = J + "DataFlowAnalysisTest"
FACTORIES = {
    J + "CombinedCompilerPass#<init>(" + J + "AbstractCompiler," + J + "NodeTraversal.Callback[])": "createFromCallbacks",
    J + "Compiler#<init>(" + J + "ErrorManager)": "createFromErrorManager",
    J + "CompilerInput#<init>(" + J + "SourceFile," + R + "InputId)": "createFromInputId",
    J + "CompilerInput#<init>(" + J + "SourceFile,java.lang.String,boolean)": "createFromIdName",
    J + "ComposeWarningsGuard#<init>(" + J + "WarningsGuard[])": "createFromGuards",
    J + "ConformanceRules$SecuritySensitiveAttributes#<init>(java.util.Collection)": "createFromBannedAttrs",
    J + "DiagnosticGroup#<init>(" + J + "DiagnosticType)": "createFromType",
    J + "DiagnosticGroup#<init>(" + J + "DiagnosticGroup[])": "createFromGroups",
    J + "DiagnosticGroup#<init>(java.lang.String," + J + "DiagnosticGroup[])": "createFromGroups",
    J + "ErrorPass#<init>(" + J + "AbstractCompiler," + J + "DiagnosticType)": "createFromDiagnosticType",
    J + "GlobalNamespace#<init>(" + J + "diagnostic.LogFile," + J + "AbstractCompiler," + R + "Node)": "createFromDecisionsLog",
    J + "JSChunkGraph#<init>(" + J + "JSChunk[])": "createFromArray",
    J + "LiveVariablesAnalysis$LiveVariableLattice#<init>(" + J + "LiveVariablesAnalysis.LiveVariableLattice)": "createFromLattice",
    J + "MustBeReachingVariableDef$MustDef#<init>(" + J + "MustBeReachingVariableDef.MustDef)": "createFromMustDef",
    J + "PeepholeOptimizationsPass#<init>(" + J + "AbstractCompiler,java.lang.String," + J + "AbstractPeepholeOptimization[])": "createFromOptimizations",
    J + "ShowByPathWarningsGuard#<init>(java.lang.String[])": "createFromPaths",
    J + "ShowByPathWarningsGuard#<init>(java.lang.String," + J + "ShowByPathWarningsGuard.ShowType)": "createFromPath",
    J + "TypedScope#<init>(" + J + "TypedScope," + R + "Node)": "createFromParent",
    J + "bundle.CachedTransformer#<init>(" + J + "bundle.Source.Transformer,java.lang.String)": "createFromSpec",
    R + "jstype.Property#<init>(" + R + "jstype.Property.Key," + R + "jstype.JSType,boolean," + R + "Node)": "createFromKey",
    R + "jstype.TemplateType#<init>(" + R + "jstype.JSTypeRegistry,java.lang.String," + R + "Node)": "createFromTypeTransformation",
    T + "$ArithmeticInstruction#<init>(" + T + ".Variable,int," + T + ".Operation,int)": "createFromNumbers",
    T + "$ArithmeticInstruction#<init>(" + T + ".Variable," + T + ".Value," + T + ".Operation,int)": "createFromValueAndNumber",
    T + "$ArithmeticInstruction#<init>(" + T + ".Variable,int," + T + ".Operation," + T + ".Value)": "createFromNumberAndValue",
    T + "$ConstPropLatticeElement#<init>(boolean)": "createFromIsTop",
    J + "LocaleDataPassesTest$LocaleResult#<init>(java.lang.String,java.lang.String)": "createFromString",
}


def main():
    edits = collections.defaultdict(list)
    seen = set()
    for line in open(sys.argv[1], encoding="utf-8"):
        if not line.startswith("CTORUSE\t"):
            continue
        _, sig, path, start, type_end = line.rstrip("\n").split("\t")
        seen.add(sig)
        if sig in FACTORIES and int(start) >= 0:
            edits[path].append((int(start), int(type_end), FACTORIES[sig]))
    sites = 0
    for path, es in sorted(edits.items()):
        text = open(path, encoding="utf-8").read()
        for start, type_end, factory in sorted(es, reverse=True):
            old = text[start:type_end]
            assert old.startswith("new ") and "<" not in old, (path, old)
            text = text[:start] + old[4:].strip() + "." + factory + text[type_end:]
            sites += 1
        open(path, "w", encoding="utf-8").write(text)
    print(f"files: {len(edits)}, call sites rewritten: {sites}")


if __name__ == "__main__":
    main()
