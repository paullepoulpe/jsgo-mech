#!/usr/bin/env python3
"""Prep step 01: replace hash-ordered collections with insertion-ordered ones.

Rewrites every production file under java/src (test helpers in */testing/
excluded) with these textual rules, then drops import lines the rewrite
duplicated:

  HashMap, HashSet, HashMultimap, HashMultiset  ->  LinkedHash*  (the bare
      names only: IdentityHashMap, ConcurrentHashMap, LinkedHashMap, ... are
      left alone)
  MultimapBuilder .hashKeys() / .hashSetValues() -> .linkedHashKeys() / .linkedHashSetValues()
  HashBasedTable.create() -> Tables.newCustomTable(new LinkedHashMap<>(), LinkedHashMap::new)
      and the declared type HashBasedTable<...> -> Table<...>

Run from the repository root: python3 -I tools/prep/01_insertion_ordered_collections.py
"""
import os, re

NAME = re.compile(r"(?<![A-Za-z_])Hash(Map|Set|Multimap|Multiset)\b")
changed = 0
for d, _, fs in os.walk("java/src"):
    if "/testing" in d:
        continue
    for f in fs:
        if not f.endswith(".java"):
            continue
        p = os.path.join(d, f)
        old = open(p, encoding="utf-8").read()
        s = NAME.sub(r"LinkedHash\1", old)
        s = s.replace(".hashKeys()", ".linkedHashKeys()").replace(".hashSetValues()", ".linkedHashSetValues()")
        if "HashBasedTable" in s:
            s = s.replace("HashBasedTable.create()", "Tables.newCustomTable(new LinkedHashMap<>(), LinkedHashMap::new)")
            s = s.replace("import com.google.common.collect.HashBasedTable;",
                          "import com.google.common.collect.Table;\nimport com.google.common.collect.Tables;\nimport java.util.LinkedHashMap;")
            s = s.replace("HashBasedTable<", "Table<")
        if s == old:
            continue
        seen, out = set(), []
        for line in s.split("\n"):
            if line.startswith("import "):
                if line in seen:
                    continue
                seen.add(line)
            out.append(line)
        open(p, "w", encoding="utf-8").write("\n".join(out))
        changed += 1
print(f"rewrote {changed} files")
