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

import com.google.common.collect.ImmutableList;
import com.google.common.collect.Iterables;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
import java.io.Serializable;
import java.util.AbstractCollection;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.jspecify.annotations.Nullable;

/**
 * A multimap whose values for a key form a list (duplicates kept, in insertion order).
 *
 * <p>Replaces Guava's {@code ListMultimap} family ({@code
 * MultimapBuilder.linkedHashKeys().arrayListValues()}, {@code ImmutableListMultimap}, {@code
 * ImmutableMultimap}) with only the operations the compiler uses. Keys iterate in the order in
 * which they were first added; a key whose last value is removed is forgotten and goes to the end
 * if it is added again. {@link #values()} iterates key by key.
 *
 * <p>All views ({@link #get}, {@link #keySet}, {@link #values}, {@link #asMap}) are live and
 * unmodifiable. The view returned by {@code get} is bound to the list the key has at the time of
 * the call: it does not follow a key that is removed and added again, and is a constant empty list
 * for a key that is absent.
 */
public final class SimpleListMultimap<K, V> implements Serializable {

  private final LinkedHashMap<K, List<V>> map = new LinkedHashMap<>();
  private boolean frozen = false;

  private SimpleListMultimap() {}

  public static <K, V> SimpleListMultimap<K, V> create() {
    return new SimpleListMultimap<>();
  }

  /** Returns an immutable copy of this multimap; mutators of the copy throw. */
  public SimpleListMultimap<K, V> immutableCopy() {
    SimpleListMultimap<K, V> copy = new SimpleListMultimap<>();
    for (Map.Entry<K, List<V>> e : map.entrySet()) {
      copy.map.put(e.getKey(), new ArrayList<>(e.getValue()));
    }
    copy.frozen = true;
    return copy;
  }

  /** Adds a key-value pair and returns this multimap, so that calls can be chained. */
  @CanIgnoreReturnValue
  public SimpleListMultimap<K, V> put(K key, V value) {
    checkState(!frozen, "immutable");
    List<V> values = map.get(key);
    if (values == null) {
      values = new ArrayList<>();
      map.put(key, values);
    }
    values.add(value);
    return this;
  }

  @CanIgnoreReturnValue
  public SimpleListMultimap<K, V> putAll(K key, Iterable<? extends V> values) {
    for (V value : values) {
      put(key, value);
    }
    return this;
  }

  /** Replaces the values of {@code key}, which keeps its position if it is present. */
  public void replaceValues(K key, Iterable<? extends V> values) {
    checkState(!frozen, "immutable");
    List<V> newValues = new ArrayList<>();
    Iterables.addAll(newValues, values);
    if (newValues.isEmpty()) {
      map.remove(key);
    } else {
      map.put(key, newValues);
    }
  }

  public List<V> get(@Nullable Object key) {
    List<V> values = map.get(key);
    return values == null ? ImmutableList.of() : Collections.unmodifiableList(values);
  }

  public boolean containsKey(@Nullable Object key) {
    return map.containsKey(key);
  }

  /** Removes the first occurrence of {@code value} from the values of {@code key}. */
  @CanIgnoreReturnValue
  public boolean remove(@Nullable Object key, @Nullable Object value) {
    checkState(!frozen, "immutable");
    List<V> values = map.get(key);
    if (values == null || !values.remove(value)) {
      return false;
    }
    if (values.isEmpty()) {
      map.remove(key);
    }
    return true;
  }

  /** Removes {@code key} and returns the values it had (empty if it was absent). */
  @CanIgnoreReturnValue
  public List<V> removeAll(@Nullable Object key) {
    checkState(!frozen, "immutable");
    List<V> values = map.remove(key);
    return values == null ? ImmutableList.of() : Collections.unmodifiableList(values);
  }

  public Set<K> keySet() {
    return Collections.unmodifiableSet(map.keySet());
  }

  public Collection<V> values() {
    return new AbstractCollection<V>() {
      @Override
      public Iterator<V> iterator() {
        return Iterables.unmodifiableIterable(Iterables.concat(map.values())).iterator();
      }

      @Override
      public int size() {
        return SimpleListMultimap.this.size();
      }
    };
  }

  /** Returns a live view; neither the map nor its lists may be modified. */
  public Map<K, List<V>> asMap() {
    return Collections.unmodifiableMap(map);
  }

  /** Returns the number of key-value pairs. */
  public int size() {
    int size = 0;
    for (List<V> values : map.values()) {
      size += values.size();
    }
    return size;
  }

  public boolean isEmpty() {
    return map.isEmpty();
  }

  @Override
  public boolean equals(@Nullable Object o) {
    return o instanceof SimpleListMultimap<?, ?> that && map.equals(that.map);
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
