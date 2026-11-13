#!/usr/bin/env python3
"""Prep step 01b: the unordered collections that step 01 did not cover.

Rules, applied to production files under java/src (test helpers excluded):

  ArrayListMultimap.create()   -> MultimapBuilder.linkedHashKeys().arrayListValues().build()
      (an ArrayListMultimap iterates its keys in hash order); a declared type
      ArrayListMultimap<K, V> becomes ListMultimap<K, V>
  Collectors.toSet()           -> Collectors.toCollection(LinkedHashSet::new)
  Collectors.groupingBy(f)     -> Collectors.groupingBy(f, LinkedHashMap::new, Collectors.toList())
  Collectors.toMap(k, v)       -> ImmutableMap.toImmutableMap(k, v)
  for (... : inputSourceMaps.values())  in Compiler.java
                               -> for (... : ImmutableSortedMap.copyOf(inputSourceMaps).values())
      (inputSourceMaps is a ConcurrentHashMap; this is the only place one is iterated)

HashBiMap, IdentityHashMap and the other ConcurrentHashMaps are left alone:
HashBiMap already iterates in insertion order, and the others are only used
for lookups.

Run from the repository root: python3 -I tools/prep/01b_ordered_collections_rest.py
"""
import os, re

def add_import(s, name):
    line = f"import {name};"
    if line in s:
        return s
    imports = [m for m in re.finditer(r"^import (static )?([\w.]+);$", s, re.M)]
    plain = [m for m in imports if not m.group(1)]
    for m in plain:
        if m.group(2) > name:
            return s[: m.start()] + line + "\n" + s[m.start():]
    last = plain[-1]
    return s[: last.end()] + "\n" + line + s[last.end():]

def drop_unused_import(s, name):
    simple = name.rsplit(".", 1)[-1]
    body = s.replace(f"import {name};\n", "")
    return body if not re.search(r"\b%s\b" % simple, body) else s

changed = 0
for d, _, fs in os.walk("java/src"):
    if "/testing" in d:
        continue
    for f in sorted(fs):
        if not f.endswith(".java"):
            continue
        p = os.path.join(d, f)
        old = s = open(p, encoding="utf-8").read()
        if "ArrayListMultimap.create()" in s:
            s = s.replace("ArrayListMultimap.create()", "MultimapBuilder.linkedHashKeys().arrayListValues().build()")
            s = re.sub(r"\bArrayListMultimap<", "ListMultimap<", s)
            s = add_import(s, "com.google.common.collect.MultimapBuilder")
            if re.search(r"\bListMultimap<", s):
                s = add_import(s, "com.google.common.collect.ListMultimap")
            s = drop_unused_import(s, "com.google.common.collect.ArrayListMultimap")
        if "Collectors.toSet()" in s:
            s = s.replace("Collectors.toSet()", "Collectors.toCollection(LinkedHashSet::new)")
            s = add_import(s, "java.util.LinkedHashSet")
        if "Collectors.groupingBy(" in s:
            s = re.sub(r"Collectors\.groupingBy\(\s*((?:[^()]|\((?:[^()]|\([^()]*\))*\))*?)\)\)",
                       lambda m: "Collectors.groupingBy(\n" + " " * 20 + m.group(1).strip() + ",\n" + " " * 20
                       + "LinkedHashMap::new,\n" + " " * 20 + "Collectors.toList()))", s)
            s = add_import(s, "java.util.LinkedHashMap")
        if "Collectors.toMap(" in s:
            s = s.replace("Collectors.toMap(", "ImmutableMap.toImmutableMap(")
            s = add_import(s, "com.google.common.collect.ImmutableMap")
            s = drop_unused_import(s, "java.util.stream.Collectors")
        if f == "Compiler.java":
            s = s.replace(": inputSourceMaps.values())", ": ImmutableSortedMap.copyOf(inputSourceMaps).values())")
            if "ImmutableSortedMap.copyOf(" in s:
                s = add_import(s, "com.google.common.collect.ImmutableSortedMap")
        if s != old:
            open(p, "w", encoding="utf-8").write(s)
            changed += 1
            print(p)
print(changed, "files changed")
