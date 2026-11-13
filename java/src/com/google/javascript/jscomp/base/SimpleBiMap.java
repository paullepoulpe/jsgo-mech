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
import static com.google.common.base.Preconditions.checkArgument;

import com.google.errorprone.annotations.CanIgnoreReturnValue;
import java.io.Serializable;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import org.jspecify.annotations.Nullable;

/**
 * A map whose values are unique, so that it can also be read from value to key.
 *
 * <p>Replaces Guava's {@code HashBiMap} and {@code ImmutableBiMap} with only the operations the
 * compiler uses. Both directions iterate in the order in which the keys were first added.
 *
 * <p>{@link #keySet}, {@link #asMap} and {@link #inverse} are live, unmodifiable views.
 */
public final class SimpleBiMap<K, V> implements Serializable {

  private final LinkedHashMap<K, V> forward = new LinkedHashMap<>();
  private final LinkedHashMap<V, K> backward = new LinkedHashMap<>();

  public static <K, V> SimpleBiMap<K, V> create() {
    return new SimpleBiMap<>();
  }

  /**
   * Returns a new bimap with the entries of {@code map}, in its iteration order.
   *
   * @throws IllegalArgumentException if two keys have the same value
   */
  public static <K, V> SimpleBiMap<K, V> copyOf(Map<? extends K, ? extends V> map) {
    SimpleBiMap<K, V> copy = new SimpleBiMap<>();
    for (Map.Entry<? extends K, ? extends V> e : map.entrySet()) {
      copy.put(e.getKey(), e.getValue());
    }
    return copy;
  }

  /**
   * Associates {@code value} with {@code key}, which keeps its position if it is present, and
   * returns the value the key had before, if any.
   *
   * @throws IllegalArgumentException if another key already has this value
   */
  @CanIgnoreReturnValue
  public @Nullable V put(K key, V value) {
    boolean hadKey = forward.containsKey(key);
    V oldValue = forward.get(key);
    if (hadKey && Objects.equals(oldValue, value)) {
      return oldValue;
    }
    checkArgument(!backward.containsKey(value), "value already present: %s", value);
    forward.put(key, value);
    if (hadKey) {
      // Keep the iteration order of the inverse the same as that of this map.
      backward.clear();
      for (Map.Entry<K, V> e : forward.entrySet()) {
        backward.put(e.getValue(), e.getKey());
      }
    } else {
      backward.put(value, key);
    }
    return oldValue;
  }

  public @Nullable V get(@Nullable Object key) {
    return forward.get(key);
  }

  public void clear() {
    forward.clear();
    backward.clear();
  }

  public Set<K> keySet() {
    return Collections.unmodifiableSet(forward.keySet());
  }

  /** Returns this bimap as a map from each key to its value. */
  public Map<K, V> asMap() {
    return Collections.unmodifiableMap(forward);
  }

  /** Returns the map from each value to its key. */
  public Map<V, K> inverse() {
    return Collections.unmodifiableMap(backward);
  }

  @Override
  public String toString() {
    return forward.toString();
  }
}
