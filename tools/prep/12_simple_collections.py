#!/usr/bin/env python3
"""Prep step 12: Guava Multimap / Multiset / Table / BiMap -> Closure's Simple* classes.

Run from the repo root: python3 tools/prep/12_simple_collections.py

Applies the textual part of the step to every production file (java/src
outside */testing/) that imports one of those Guava types or one of the
Simple* classes; no rule matches its own output, so the script is idempotent.
Rules:

  types      ListMultimap<, ImmutableListMultimap<, ImmutableMultimap<,
             ImmutableListMultimap.Builder<, ImmutableMultimap.Builder<  -> SimpleListMultimap<
             SetMultimap<, ImmutableSetMultimap<, LinkedHashMultimap<,
             ImmutableSetMultimap.Builder<                               -> SimpleSetMultimap<
             Multiset<, ImmutableMultiset<, LinkedHashMultiset<,
             ImmutableMultiset.Builder<                                  -> SimpleMultiset<
             Multiset.Entry<E>                                           -> Map.Entry<E, Integer>
             Table<                                                      -> SimpleTable<
             BiMap<, ImmutableBiMap<                                     -> SimpleBiMap<
  factories  MultimapBuilder.linkedHashKeys().arrayListValues().build(),
             LinkedListMultimap.create(), ImmutableListMultimap.builder(),
             ImmutableMultimap.builder()            -> SimpleListMultimap.create()
             MultimapBuilder.linkedHashKeys().linkedHashSetValues().build(),
             ImmutableSetMultimap.builder()         -> SimpleSetMultimap.create()
             LinkedHashMultimap.create()            -> SimpleSetMultimap.createInsertionOrdered()
             TreeMultimap.create()                  -> SimpleSetMultimap.createSorted()
             LinkedHashMultiset.create(), ImmutableMultiset.builder() -> SimpleMultiset.create()
             TreeMultiset.create()                  -> SimpleMultiset.createSorted()
             ImmutableMultiset.of()                 -> SimpleMultiset.of()
             ImmutableMultiset.copyOf(              -> SimpleMultiset.copyOf(
             Tables.newCustomTable(new LinkedHashMap<>(), LinkedHashMap::new) -> SimpleTable.create()
             HashBiMap.create()                     -> SimpleBiMap.create()
             ImmutableBiMap.copyOf(                 -> SimpleBiMap.copyOf(
  calls      Multimaps.asMap(x)                     -> x.asMap()
             ImmutableSetMultimap.copyOf(x)         -> x.immutableCopy()
             (entry).getElement() / .getCount()     -> .getKey() / .getValue()
             (only in files that used Multiset.Entry)
             Multimap<  -> SimpleListMultimap< or SimpleSetMultimap<, whichever the
             creation site makes (per-file table BARE_MULTIMAP)
  imports    the Guava imports are removed and the Simple* imports added.

Left for the hand edits that follow (the compiler points at each one):
`builder.build()` of the Immutable* builders (-> `.immutableCopy()`), the
declared types of the results of get() / asMap() (ImmutableSet<V> -> Set<V>,
Collection<V> -> List<V> ...), imports that become unused, and the handful of
sites listed in the commit messages of step 12.
"""
import os
import re

GUAVA = re.compile(
    r"^import com\.google\.common\.collect\."
    r"(\w*Multimap\w*|\w*Multiset\w*|Multimaps|\w*Table\w*|Tables|\w*BiMap);\n",
    re.M,
)
OURS = re.compile(r"^import com\.google\.javascript\.jscomp\.base\.Simple\w+;\n", re.M)
ARGS = r"(\s*<[^<>()]*(?:<[^<>()]*>[^<>()]*)*>)?"  # optional explicit type arguments
RULES = [
    (r"\bMultiset\.Entry<([^<>]*)>", r"Map.Entry<\1, Integer>"),
    (r"\bImmutable(?:List)?Multimap\.(@Nullable )?Builder<", r"\1SimpleListMultimap<"),
    (r"\bImmutableSetMultimap\.(@Nullable )?Builder<", r"\1SimpleSetMultimap<"),
    (r"\bImmutableMultiset\.(@Nullable )?Builder<", r"\1SimpleMultiset<"),
    (r"\b(?:ListMultimap|ImmutableListMultimap|ImmutableMultimap)<", "SimpleListMultimap<"),
    (r"\b(?:SetMultimap|ImmutableSetMultimap|LinkedHashMultimap)<", "SimpleSetMultimap<"),
    (r"\b(?:Multiset|ImmutableMultiset|LinkedHashMultiset)<", "SimpleMultiset<"),
    (r"\bTable<", "SimpleTable<"),
    (r"\b(?:BiMap|ImmutableBiMap)<", "SimpleBiMap<"),
    (
        r"\bMultimapBuilder\s*\.linkedHashKeys\(\)\s*\.arrayListValues\(\)\s*\." + ARGS + r"build\(\)",
        "SimpleListMultimap.create()",
    ),
    (
        r"\bMultimapBuilder(?:\.SetMultimapBuilder)?\s*\.linkedHashKeys\(\)\s*\.linkedHashSetValues\(\)\s*\."
        + ARGS
        + r"build\(\)",
        "SimpleSetMultimap.create()",
    ),
    (r"\b(?:LinkedListMultimap\.create|Immutable(?:List)?Multimap\." + ARGS + r"builder)\(\)", "SimpleListMultimap.create()"),
    (r"\bImmutableSetMultimap\." + ARGS + r"builder\(\)", "SimpleSetMultimap.create()"),
    (r"\bLinkedHashMultimap\.create\(\)", "SimpleSetMultimap.createInsertionOrdered()"),
    (r"\bTreeMultimap\.create\(\)", "SimpleSetMultimap.createSorted()"),
    (r"\b(?:LinkedHashMultiset\.create|ImmutableMultiset\." + ARGS + r"builder)\(\)", "SimpleMultiset.create()"),
    (r"\bTreeMultiset\.create\(\)", "SimpleMultiset.createSorted()"),
    (r"\bImmutableMultiset\." + ARGS + r"of\(\)", "SimpleMultiset.of()"),
    (r"\bImmutableMultiset\.copyOf\(", "SimpleMultiset.copyOf("),
    (
        r"\bTables\.newCustomTable\(new LinkedHashMap<>\(\), LinkedHashMap::new\)",
        "SimpleTable.create()",
    ),
    (r"\bHashBiMap\.create\(\)", "SimpleBiMap.create()"),
    (r"\bImmutableBiMap\.copyOf\(", "SimpleBiMap.copyOf("),
    (r"\bMultimaps\.asMap\(([\w.]+)\)", r"\1.asMap()"),
    (r"\bImmutableSetMultimap\.copyOf\(([\w.]+)\)", r"\1.immutableCopy()"),
]
ENTRY_RULES = [(r"\.getElement\(\)", ".getKey()"), (r"\.getCount\(\)", ".getValue()")]
# Files with variables typed with the bare Guava `Multimap<`, and the class their creation site
# makes of them (every such file only has one kind).
BARE_MULTIMAP = {
    "AllowlistWarningsGuard.java": "SimpleSetMultimap<",
    "CheckNoMutatedEs6Exports.java": "SimpleSetMultimap<",
    "CheckRequiresSorted.java": "SimpleListMultimap<",
    "ClosureRewriteModule.java": "SimpleSetMultimap<",
    "Compiler.java": "SimpleSetMultimap<",
    "FunctionRewriter.java": "SimpleSetMultimap<",
    "J2clClinitPrunerPass.java": "SimpleSetMultimap<",
    "J2clConstantHoisterPass.java": "SimpleListMultimap<",
    "JSTypeRegistry.java": "SimpleSetMultimap<",
    "JsFileFullParser.java": "SimpleSetMultimap<",
    "PreprocessorSymbolTable.java": "SimpleListMultimap<",
    "PureFunctionIdentifier.java": "SimpleListMultimap<",
    "RemoveUnusedCode.java": "SimpleSetMultimap<",
    "TypedScopeCreator.java": "SimpleSetMultimap<",
}
SIMPLE = ["SimpleBiMap", "SimpleListMultimap", "SimpleMultiset", "SimpleSetMultimap", "SimpleTable"]


def add_import(src, line):
    if line in src:
        return src
    imports = [m for m in re.finditer(r"^import (?!static )[^\n]*\n", src, re.M)]
    for m in imports:
        if m.group(0) > line:
            return src[: m.start()] + line + src[m.start() :]
    return src[: imports[-1].end()] + line + src[imports[-1].end() :]


def rewrite(src, name):
    if name in BARE_MULTIMAP:
        src = re.sub(r"\bMultimap<(?!\?)", BARE_MULTIMAP[name], src)
    if not GUAVA.search(src) and not OURS.search(src) and name not in BARE_MULTIMAP:
        return src
    had_entry = "Multiset.Entry" in src
    src = GUAVA.sub("", src)
    for pattern, replacement in RULES + (ENTRY_RULES if had_entry else []):
        src = re.sub(pattern, replacement, src)
    package = re.search(r"^package ([\w.]+);", src, re.M).group(1)
    for name in SIMPLE:
        if re.search(r"\b%s\b" % name, src) and package != "com.google.javascript.jscomp.base":
            src = add_import(src, "import com.google.javascript.jscomp.base.%s;\n" % name)
    if re.search(r"\bMap\.Entry<", src) and not re.search(r"^import java\.util\.Map;", src, re.M):
        src = add_import(src, "import java.util.Map;\n")
    return src


def main():
    changed = 0
    for root, _, files in os.walk("java/src"):
        if "/testing" in root:
            continue
        for name in sorted(files):
            if not name.endswith(".java"):
                continue
            path = os.path.join(root, name)
            with open(path, encoding="utf-8") as f:
                old = f.read()
            new = rewrite(old, name)
            if new != old:
                with open(path, "w", encoding="utf-8") as f:
                    f.write(new)
                changed += 1
    print("files rewritten: %d" % changed)


if __name__ == "__main__":
    main()
