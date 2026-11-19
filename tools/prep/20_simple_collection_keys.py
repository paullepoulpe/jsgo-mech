#!/usr/bin/env python3
"""Prep step 20: the lookups of the Simple* collections take the key type.

Like java.util.Map.get(Object), the lookup methods of SimpleListMultimap, SimpleSetMultimap,
SimpleMultiset, SimpleBiMap and SimpleTable were declared with `@Nullable Object` parameters:
get, containsKey, containsEntry, remove, removeAll, count, contains, containsRow, row. Every call
in the compiler passes a key of the collection's own key type, so the parameters are declared
with it (`List<V> get(K key)`); javac checks every call site. The bodies are unchanged: they
pass the key on to the java.util map, whose parameter is Object.

For a translator that keeps the type parameters of these classes (j2go: `SimpleSetMultimap<String,
Node>` is `*SimpleSetMultimap[string, Node]`) a key then reaches the lookup as it is, where an
Object parameter made every call site box it.

Run from the repository root. Exact-text edits; a second run finds nothing to do.
"""
import re
import sys

BASE = 'java/src/com/google/javascript/jscomp/base/'

# class -> method -> the types of its parameters, in order
TABLE = {
    'SimpleListMultimap': {'get': ['K'], 'containsKey': ['K'], 'remove': ['K', 'V'], 'removeAll': ['K']},
    'SimpleSetMultimap': {'get': ['K'], 'containsKey': ['K'], 'containsEntry': ['K', 'V'], 'removeAll': ['K']},
    'SimpleMultiset': {'count': ['E'], 'contains': ['E']},
    'SimpleBiMap': {'get': ['K']},
    'SimpleTable': {'get': ['R', 'C'], 'contains': ['R', 'C'], 'containsRow': ['R'], 'remove': ['R', 'C'], 'row': ['R']},
}

total = 0
for cls, methods in TABLE.items():
    path = BASE + cls + '.java'
    text = open(path).read()
    for name, types in methods.items():
        old = r'(\b%s\()' % name + ', '.join(r'@Nullable Object (\w+)' for _ in types) + r'\) \{'
        found = re.findall(old, text)
        if not found:
            new = r'\b%s\(' % name + ', '.join(r'%s \w+' % t for t in types) + r'\) \{'
            if not re.search(new, text):
                sys.exit(f'{path}: {name}({", ".join(types)}) not found')
            continue
        if len(found) != 1:
            sys.exit(f'{path}: {len(found)} declarations of {name} with Object parameters')

        def repl(m, types=types):
            return m.group(1) + ', '.join(f'{t} {m.group(i + 2)}' for i, t in enumerate(types)) + ') {'

        text = re.sub(old, repl, text)
        total += len(types)
    # SimpleSetMultimap.removeAll: the entry it looks up is now one of the multimap's own type
    text = text.replace('new Entry<Object, Object>(key, value)', 'new Entry<>(key, value)')
    if '@Nullable' not in re.sub(r'import [^\n]*\n', '', text):
        text = text.replace('import org.jspecify.annotations.Nullable;\n', '')
    open(path, 'w').write(text)

print(f'prep 20: {total} parameters' if total else 'prep 20: nothing to do')
