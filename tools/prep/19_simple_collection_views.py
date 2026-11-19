#!/usr/bin/env python3
"""Prep step 19: the Simple* collections are never seen at a wildcard type.

SimpleListMultimap<K, V>, SimpleSetMultimap<K, V>, SimpleMultiset<E> and SimpleBiMap<K, V> are
each used at a wildcard type in two places only: their own equals(), which reads the map of
another instantiation (`o instanceof SimpleListMultimap<?, ?> that && map.equals(that.map)`), and
LogsGson.write, which writes any instantiation (`value instanceof SimpleListMultimap<?, ?>
multimap` ... `multimap.asMap()`). A translator that keeps type parameters (j2go's rule "typed
generics": `SimpleListMultimap<String, Node>` is one Go type, with typed put and get) has no type
for "some instantiation", so one such use erases K and V for the whole program. This step gives
the four classes an interface without type parameters for those two places:

  * new interface jscomp.base.SimpleCollection with one method, `Map<?, ?> untypedMap()`: the
    unmodifiable map that the collection is equal by (asMap() of a multimap or bimap, the counts
    of a multiset);
  * the four classes implement it;
  * equals() tests `o instanceof C` (no type arguments, no variable) and compares its own map
    with `((SimpleCollection) o).untypedMap()`. That is the comparison made before: the other
    map is now behind Collections.unmodifiableMap, whose get/containsKey/size/entrySet, the
    methods that AbstractMap.equals and TreeMap.equals call on their argument, are the map's own;
  * LogsGson.write tests `value instanceof C` and writes `((SimpleCollection) value).untypedMap()`
    for the multimaps and the bimap (asMap() is that map) and
    `ImmutableList.copyOf((Iterable<?>) value)` for a multiset (asList() is the list of its
    iteration);
  * SimpleSetMultimap's one inner class, InsertionOrderedValuesIterator, becomes a local class
    of values(), the only method that makes one (j2go keeps the type parameters of a class with
    local and anonymous classes, not of one with member inner classes).

No behaviour changes. Run from the repository root; exact-text edits, each applied once (a
second run finds nothing to do and says so).
"""
import re
import sys

BASE = 'java/src/com/google/javascript/jscomp/base/'
LOGS = 'java/src/com/google/javascript/jscomp/diagnostic/LogsGson.java'

INTERFACE = '''/*
 * Copyright 2026 The Closure Compiler Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.google.javascript.jscomp.base;

import java.util.Map;

/**
 * What {@link SimpleListMultimap}, {@link SimpleSetMultimap}, {@link SimpleMultiset} and {@link
 * SimpleBiMap} are to code that does not know their type arguments: {@code equals} of two
 * collections, and the logs, which write any of them.
 */
public interface SimpleCollection {
  /**
   * Returns the live, unmodifiable map that this collection is equal by: from each key to its
   * values (a multimap), to its value (a bimap), or from each element to its count (a multiset).
   */
  Map<?, ?> untypedMap();
}
'''

changed = []


def edit(path, pairs):
    text = open(path).read()
    orig = text
    for old, new in pairs:
        if new in text:
            continue  # applied by an earlier run
        if text.count(old) != 1:
            sys.exit(f'{path}: expected exactly one occurrence of:\n{old}')
        text = text.replace(old, new)
    if text != orig:
        open(path, 'w').write(text)
        changed.append(path)


def untyped(field_expr):
    return ('''
  @Override
  public Map<?, ?> untypedMap() {
    return %s;
  }
''' % field_expr)


def collection(name, params, field, view, anchor, also=''):
    """Class `name`: implements SimpleCollection; equals through untypedMap(), declared before `anchor`."""
    wild = ', '.join('?' for _ in params.split(','))
    edit(BASE + name + '.java', [
        (f'public final class {name}<{params}> implements {also}Serializable {{',
         f'public final class {name}<{params}> implements {also}SimpleCollection, Serializable {{'),
        (f'    return o instanceof {name}<{wild}> that && {field}.equals(that.{field});',
         f'    return o instanceof {name} && {field}.equals(((SimpleCollection) o).untypedMap());'),
        (anchor, untyped(view).lstrip('\n') + '\n' + anchor),
    ])


collection('SimpleListMultimap', 'K, V', 'map', 'asMap()', '  /** Returns the number of key-value pairs. */\n')
collection('SimpleSetMultimap', 'K, V', 'map', 'asMap()', '  /** Returns the number of key-value pairs. */\n')
collection('SimpleMultiset', 'E', 'counts', 'Collections.unmodifiableMap(counts)',
           '  /** Returns the number of occurrences of all elements. */\n', also='Iterable<E>, ')
# SimpleBiMap has Object's equals; LogsGson writes its asMap().
edit(BASE + 'SimpleBiMap.java', [
    ('public final class SimpleBiMap<K, V> implements Serializable {',
     'public final class SimpleBiMap<K, V> implements SimpleCollection, Serializable {'),
    ('  /** Returns the map from each value to its key. */\n',
     untyped('asMap()').lstrip('\n') + '\n  /** Returns the map from each value to its key. */\n'),
])

# SimpleSetMultimap.InsertionOrderedValuesIterator: a local class of values().
path = BASE + 'SimpleSetMultimap.java'
text = open(path).read()
m = re.search(r'\n  private final class InsertionOrderedValuesIterator implements Iterator<V> \{\n.*?\n  \}\n', text, re.S)
if m:
    cls = m.group(0)
    text = text.replace(cls, '')
    local = ''.join(('  ' + line if line.strip() else line) for line in cls.lstrip('\n').splitlines(True))
    local = local.replace('    private final class InsertionOrderedValuesIterator', '    final class InsertionOrderedValuesIterator')
    head = '  public Collection<V> values() {\n'
    if text.count(head) != 1:
        sys.exit(path + ': values() not found')
    text = text.replace(head, head + local)
    open(path, 'w').write(text)
    if path not in changed:
        changed.append(path)

edit(LOGS, [
    ('''    } else if (value instanceof SimpleListMultimap<?, ?> multimap) {
      out.rawValue(Optional.of(toJson(multimap.asMap())));
    } else if (value instanceof SimpleSetMultimap<?, ?> multimap) {
      out.rawValue(Optional.of(toJson(multimap.asMap())));
    } else if (value instanceof SimpleBiMap<?, ?> bimap) {
      out.rawValue(Optional.of(toJson(bimap.asMap())));
    } else if (value instanceof SimpleMultiset<?> multiset) {
      out.rawValue(Optional.of(toJson(multiset.asList())));
''',
     '''    } else if (value instanceof SimpleListMultimap
        || value instanceof SimpleSetMultimap
        || value instanceof SimpleBiMap) {
      out.rawValue(Optional.of(toJson(((SimpleCollection) value).untypedMap())));
    } else if (value instanceof SimpleMultiset) {
      out.rawValue(Optional.of(toJson(ImmutableList.copyOf((Iterable<?>) value))));
'''),
    ('import com.google.javascript.jscomp.base.SimpleBiMap;\n',
     'import com.google.common.collect.ImmutableList;\nimport com.google.javascript.jscomp.base.SimpleBiMap;\nimport com.google.javascript.jscomp.base.SimpleCollection;\n'),
])

try:
    open(BASE + 'SimpleCollection.java', 'x').write(INTERFACE)
    changed.append(BASE + 'SimpleCollection.java')
except FileExistsError:
    pass

print('prep 19: ' + (', '.join(changed) if changed else 'nothing to do'))
