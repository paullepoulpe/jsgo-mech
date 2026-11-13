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
import static com.google.common.base.Preconditions.checkState;

import com.google.common.collect.ImmutableList;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
import java.io.Serializable;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import org.jspecify.annotations.Nullable;

/**
 * A collection that counts how many times each element was added.
 *
 * <p>Replaces Guava's {@code Multiset} family with only the operations the compiler uses. Elements
 * iterate in the order in which they were first added ({@link #create}, for {@code
 * LinkedHashMultiset} and {@code ImmutableMultiset}) or in their natural order ({@link
 * #createSorted}, for {@code TreeMultiset}); an element whose count drops to zero is forgotten, and
 * goes to the end if it is added again. Iterating the multiset itself yields each element as many
 * times as its count, element by element.
 *
 * <p>{@link #elementSet} and {@link #entrySet} are live, unmodifiable views; the entries map each
 * element to its count.
 */
public final class SimpleMultiset<E> implements Iterable<E>, Serializable {

  private final Map<E, Integer> counts;
  private int totalCount = 0;
  private boolean frozen = false;

  private SimpleMultiset(Map<E, Integer> counts) {
    this.counts = counts;
  }

  public static <E> SimpleMultiset<E> create() {
    return new SimpleMultiset<>(new LinkedHashMap<>());
  }

  public static <E extends Comparable<? super E>> SimpleMultiset<E> createSorted() {
    return new SimpleMultiset<>(new TreeMap<>());
  }

  /** Returns a new immutable empty multiset. */
  public static <E> SimpleMultiset<E> of() {
    return SimpleMultiset.<E>create().immutableCopy();
  }

  /**
   * Returns a new immutable multiset of the given elements, which iterates in the order in which
   * they first occur in {@code elements}.
   */
  public static <E> SimpleMultiset<E> copyOf(Iterable<? extends E> elements) {
    SimpleMultiset<E> copy = create();
    copy.addAll(elements);
    copy.frozen = true;
    return copy;
  }

  /** Returns an immutable copy with the same iteration order; mutators of the copy throw. */
  public SimpleMultiset<E> immutableCopy() {
    SimpleMultiset<E> copy = create();
    copy.counts.putAll(counts);
    copy.totalCount = totalCount;
    copy.frozen = true;
    return copy;
  }

  @CanIgnoreReturnValue
  public boolean add(E element) {
    add(element, 1);
    return true;
  }

  /** Adds {@code occurrences} of {@code element} and returns the count it had before. */
  @CanIgnoreReturnValue
  public int add(E element, int occurrences) {
    checkState(!frozen, "immutable");
    checkArgument(occurrences >= 0);
    int oldCount = count(element);
    if (occurrences > 0) {
      counts.put(element, oldCount + occurrences);
      totalCount += occurrences;
    }
    return oldCount;
  }

  public void addAll(Iterable<? extends E> elements) {
    for (E element : elements) {
      add(element, 1);
    }
  }

  /** Sets the count of {@code element} to {@code newCount} if it is {@code expectedOldCount}. */
  @CanIgnoreReturnValue
  public boolean setCount(E element, int expectedOldCount, int newCount) {
    checkState(!frozen, "immutable");
    checkArgument(expectedOldCount >= 0 && newCount >= 0);
    if (count(element) != expectedOldCount) {
      return false;
    }
    if (newCount == 0) {
      counts.remove(element);
    } else {
      counts.put(element, newCount);
    }
    totalCount += newCount - expectedOldCount;
    return true;
  }

  /** Removes one occurrence of {@code element}, if there is one. */
  @CanIgnoreReturnValue
  public boolean remove(E element) {
    int oldCount = count(element);
    return oldCount > 0 && setCount(element, oldCount, oldCount - 1);
  }

  /** Removes every occurrence of every one of {@code elements}. */
  @CanIgnoreReturnValue
  public boolean removeAll(Iterable<? extends E> elements) {
    boolean changed = false;
    for (E element : elements) {
      int oldCount = count(element);
      if (oldCount > 0) {
        setCount(element, oldCount, 0);
        changed = true;
      }
    }
    return changed;
  }

  public int count(@Nullable Object element) {
    Integer count = counts.get(element);
    return count == null ? 0 : count;
  }

  public boolean contains(@Nullable Object element) {
    return counts.containsKey(element);
  }

  public Set<E> elementSet() {
    return Collections.unmodifiableSet(counts.keySet());
  }

  public Set<Map.Entry<E, Integer>> entrySet() {
    return Collections.unmodifiableMap(counts).entrySet();
  }

  /** Returns the number of occurrences of all elements. */
  public int size() {
    return totalCount;
  }

  public boolean isEmpty() {
    return totalCount == 0;
  }

  /** Returns a snapshot of the occurrences of all elements, element by element. */
  public ImmutableList<E> asList() {
    ImmutableList.Builder<E> list = ImmutableList.builder();
    for (Map.Entry<E, Integer> e : counts.entrySet()) {
      for (int i = 0; i < e.getValue(); i++) {
        list.add(e.getKey());
      }
    }
    return list.build();
  }

  /** Iterates over a snapshot. */
  @Override
  public Iterator<E> iterator() {
    return asList().iterator();
  }

  @Override
  public boolean equals(@Nullable Object o) {
    return o instanceof SimpleMultiset<?> that && counts.equals(that.counts);
  }

  @Override
  public int hashCode() {
    return counts.hashCode();
  }

  @Override
  public String toString() {
    List<String> parts = new ArrayList<>();
    for (Map.Entry<E, Integer> e : counts.entrySet()) {
      String element = String.valueOf(e.getKey());
      parts.add(e.getValue() == 1 ? element : element + " x " + e.getValue());
    }
    return parts.toString();
  }
}
