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
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.Iterables;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
import java.io.Serializable;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Set;
import org.jspecify.annotations.Nullable;

/**
 * A map from pairs of a row key and a column key to values.
 *
 * <p>Replaces Guava's {@code Table} (as created by {@code Tables.newCustomTable} over linked hash
 * maps) with only the operations the compiler uses. Rows iterate in the order in which they were
 * first added and the columns of a row in the order in which they were first added to that row; a
 * row whose last cell is removed is forgotten.
 *
 * <p>{@link #row}, {@link #rowKeySet} and {@link #values} are live, unmodifiable views. The view
 * returned by {@code row} is bound to the map the row has at the time of the call: it does not
 * follow a row that is removed and added again, and is a constant empty map for a row that is
 * absent.
 */
public final class SimpleTable<R, C, V> implements Serializable {

  private final LinkedHashMap<R, Map<C, V>> rows = new LinkedHashMap<>();

  public static <R, C, V> SimpleTable<R, C, V> create() {
    return new SimpleTable<>();
  }

  /** Associates the value with the keys and returns the value they had before, if any. */
  @CanIgnoreReturnValue
  public @Nullable V put(R rowKey, C columnKey, V value) {
    Map<C, V> row = rows.get(rowKey);
    if (row == null) {
      row = new LinkedHashMap<>();
      rows.put(rowKey, row);
    }
    return row.put(columnKey, value);
  }

  public @Nullable V get(@Nullable Object rowKey, @Nullable Object columnKey) {
    Map<C, V> row = rows.get(rowKey);
    return row == null ? null : row.get(columnKey);
  }

  public boolean contains(@Nullable Object rowKey, @Nullable Object columnKey) {
    Map<C, V> row = rows.get(rowKey);
    return row != null && row.containsKey(columnKey);
  }

  public boolean containsRow(@Nullable Object rowKey) {
    return rows.containsKey(rowKey);
  }

  /** Removes the cell with the given keys and returns the value it had, if any. */
  @CanIgnoreReturnValue
  public @Nullable V remove(@Nullable Object rowKey, @Nullable Object columnKey) {
    Map<C, V> row = rows.get(rowKey);
    if (row == null) {
      return null;
    }
    V value = row.remove(columnKey);
    if (row.isEmpty()) {
      rows.remove(rowKey);
    }
    return value;
  }

  public Map<C, V> row(@Nullable Object rowKey) {
    Map<C, V> row = rows.get(rowKey);
    return row == null ? ImmutableMap.of() : Collections.unmodifiableMap(row);
  }

  public Set<R> rowKeySet() {
    return Collections.unmodifiableSet(rows.keySet());
  }

  /** Returns the values of all cells, row by row. */
  public Iterable<V> values() {
    return Iterables.unmodifiableIterable(
        Iterables.concat(Iterables.transform(rows.values(), Map::values)));
  }

  public boolean isEmpty() {
    return rows.isEmpty();
  }

  @Override
  public String toString() {
    return rows.toString();
  }
}
