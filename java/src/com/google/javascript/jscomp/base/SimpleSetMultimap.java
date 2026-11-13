/*
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

import static com.google.common.base.Preconditions.checkState;

import com.google.common.collect.ImmutableSet;
import com.google.common.collect.Iterables;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
import java.io.Serializable;
import java.util.AbstractCollection;
import java.util.Collection;
import java.util.Collections;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;
import org.jspecify.annotations.Nullable;

/**
 * A multimap whose values for a key form a set (adding an existing key-value pair does nothing).
 *
 * <p>Replaces Guava's {@code SetMultimap} family with only the operations the compiler uses. There
 * are three flavours, which differ in iteration order only:
 *
 * <ul>
 *   <li>{@link #create}, for {@code MultimapBuilder.linkedHashKeys().linkedHashSetValues()} and
 *       {@code ImmutableSetMultimap}: keys iterate in the order in which they were first added,
 *       the values of a key in insertion order, and {@link #values()} key by key.
 *   <li>{@link #createInsertionOrdered}, for {@code LinkedHashMultimap}: the same, except that
 *       {@link #values()} iterates in the order in which the key-value pairs were added,
 *       regardless of their keys.
 *   <li>{@link #createSorted}, for {@code TreeMultimap}: keys and the values of a key iterate in
 *       their natural order, and {@link #values()} key by key.
 * </ul>
 *
 * <p>A key whose last value is removed is forgotten, and goes to the end if it is added again.
 *
 * <p>All views ({@link #get}, {@link #keySet}, {@link #values}, {@link #asMap}) are live and
 * unmodifiable, except that the iterator of {@code values()} of an insertion-ordered multimap
 * supports {@code remove()}. The view returned by {@code get} is bound to the set the key has at
 * the time of the call: it does not follow a key that is removed and added again, and is a constant
 * empty set for a key that is absent.
 */
public final class SimpleSetMultimap<K, V> implements Serializable {

  private record Entry<K, V>(K key, V value) implements Serializable {}

  private final Map<K, Set<V>> map;
  private final boolean sorted;
  // Every key-value pair, in insertion order; null unless this multimap is insertion-ordered.
  private final @Nullable LinkedHashSet<Entry<K, V>> entryOrder;
  private boolean frozen = false;

  private SimpleSetMultimap(boolean sorted, boolean insertionOrdered) {
    this.map = sorted ? new TreeMap<>() : new LinkedHashMap<>();
    this.sorted = sorted;
    this.entryOrder = insertionOrdered ? new LinkedHashSet<>() : null;
  }

  public static <K, V> SimpleSetMultimap<K, V> create() {
    return new SimpleSetMultimap<>(false, false);
  }

  public static <K, V> SimpleSetMultimap<K, V> createInsertionOrdered() {
    return new SimpleSetMultimap<>(false, true);
  }

  public static <K extends Comparable<? super K>, V extends Comparable<? super V>>
      SimpleSetMultimap<K, V> createSorted() {
    return new SimpleSetMultimap<>(true, false);
  }

  /**
   * Returns an immutable copy of this multimap, with the same iteration order of keys and of the
   * values of each key; mutators of the copy throw.
   */
  public SimpleSetMultimap<K, V> immutableCopy() {
    SimpleSetMultimap<K, V> copy = new SimpleSetMultimap<>(false, false);
    for (Map.Entry<K, Set<V>> e : map.entrySet()) {
      copy.map.put(e.getKey(), new LinkedHashSet<>(e.getValue()));
    }
    copy.frozen = true;
    return copy;
  }

  /**
   * Returns an immutable multimap with a pair (v, k) for every pair (k, v) of this one, added key
   * by key.
   */
  public SimpleSetMultimap<V, K> inverse() {
    SimpleSetMultimap<V, K> inverse = new SimpleSetMultimap<>(false, false);
    for (Map.Entry<K, Set<V>> e : map.entrySet()) {
      for (V value : e.getValue()) {
        inverse.put(value, e.getKey());
      }
    }
    inverse.frozen = true;
    return inverse;
  }

  @CanIgnoreReturnValue
  public boolean put(K key, V value) {
    checkState(!frozen, "immutable");
    Set<V> values = map.get(key);
    if (values == null) {
      values = sorted ? new TreeSet<>() : new LinkedHashSet<>();
      values.add(value);
      map.put(key, values);
    } else if (!values.add(value)) {
      return false;
    }
    if (entryOrder != null) {
      entryOrder.add(new Entry<>(key, value));
    }
    return true;
  }

  public void putAll(K key, Iterable<? extends V> values) {
    for (V value : values) {
      put(key, value);
    }
  }

  /** Adds every key-value pair of {@code other}, in the iteration order of its values. */
  public <K2 extends K, V2 extends V> void putAll(SimpleSetMultimap<K2, V2> other) {
    if (other.entryOrder != null) {
      for (Entry<K2, V2> e : other.entryOrder) {
        put(e.key(), e.value());
      }
    } else {
      for (Map.Entry<K2, Set<V2>> e : other.map.entrySet()) {
        putAll(e.getKey(), e.getValue());
      }
    }
  }

  public Set<V> get(@Nullable Object key) {
    Set<V> values = map.get(key);
    return values == null ? ImmutableSet.of() : Collections.unmodifiableSet(values);
  }

  public boolean containsKey(@Nullable Object key) {
    return map.containsKey(key);
  }

  public boolean containsEntry(@Nullable Object key, @Nullable Object value) {
    Set<V> values = map.get(key);
    return values != null && values.contains(value);
  }

  /** Removes {@code key} and returns the values it had (empty if it was absent). */
  @CanIgnoreReturnValue
  public Set<V> removeAll(@Nullable Object key) {
    checkState(!frozen, "immutable");
    Set<V> values = map.remove(key);
    if (values == null) {
      return ImmutableSet.of();
    }
    if (entryOrder != null) {
      for (V value : values) {
        entryOrder.remove(new Entry<Object, Object>(key, value));
      }
    }
    return Collections.unmodifiableSet(values);
  }

  public void clear() {
    checkState(!frozen, "immutable");
    map.clear();
    if (entryOrder != null) {
      entryOrder.clear();
    }
  }

  public Set<K> keySet() {
    return Collections.unmodifiableSet(map.keySet());
  }

  public Collection<V> values() {
    return new AbstractCollection<V>() {
      @Override
      public Iterator<V> iterator() {
        return entryOrder != null
            ? new InsertionOrderedValuesIterator()
            : Iterables.unmodifiableIterable(Iterables.concat(map.values())).iterator();
      }

      @Override
      public int size() {
        return SimpleSetMultimap.this.size();
      }
    };
  }

  private final class InsertionOrderedValuesIterator implements Iterator<V> {
    private final Iterator<Entry<K, V>> entries = entryOrder.iterator();
    private @Nullable Entry<K, V> last = null;

    @Override
    public boolean hasNext() {
      return entries.hasNext();
    }

    @Override
    public V next() {
      last = entries.next();
      return last.value();
    }

    @Override
    public void remove() {
      checkState(!frozen, "immutable");
      entries.remove();
      Set<V> values = map.get(last.key());
      values.remove(last.value());
      if (values.isEmpty()) {
        map.remove(last.key());
      }
    }
  }

  /** Returns a live view; neither the map nor its sets may be modified. */
  public Map<K, Set<V>> asMap() {
    return Collections.unmodifiableMap(map);
  }

  /** Returns the number of key-value pairs. */
  public int size() {
    int size = 0;
    for (Set<V> values : map.values()) {
      size += values.size();
    }
    return size;
  }

  public boolean isEmpty() {
    return map.isEmpty();
  }

  @Override
  public boolean equals(@Nullable Object o) {
    return o instanceof SimpleSetMultimap<?, ?> that && map.equals(that.map);
  }

  @Override
  public int hashCode() {
    return map.hashCode();
  }

  @Override
  public String toString() {
    return map.toString();
  }
}
