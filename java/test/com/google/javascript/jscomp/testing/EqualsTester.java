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

package com.google.javascript.jscomp.testing;

import java.util.ArrayList;
import java.util.List;

/**
 * Plain Java replacement for the subset of Guava's {@code EqualsTester} used by the tests.
 *
 * <p>Checks that every object is equal to itself, unequal to null and to an object of an unrelated
 * class, equal (with an equal hash code) to every object of its own group, and unequal to every
 * object of every other group.
 */
public final class EqualsTester {
  private final List<Object[]> groups = new ArrayList<>();

  public EqualsTester addEqualityGroup(Object... group) {
    groups.add(group.clone());
    return this;
  }

  public EqualsTester testEquals() {
    Object unrelated = new Object();
    for (int g = 0; g < groups.size(); g++) {
      for (Object item : groups.get(g)) {
        check(item.equals(item), item + " must be equal to itself");
        check(!item.equals(null), item + " must not be equal to null");
        check(!item.equals(unrelated), item + " must not be equal to an unrelated object");
        check(item.hashCode() == item.hashCode(), item + " must have a stable hash code");
        for (int h = 0; h < groups.size(); h++) {
          for (Object other : groups.get(h)) {
            if (g == h) {
              check(item.equals(other), item + " must be equal to " + other);
              check(
                  item.hashCode() == other.hashCode(),
                  item + " must have the same hash code as " + other);
            } else {
              check(!item.equals(other), item + " must not be equal to " + other);
            }
          }
        }
      }
    }
    return this;
  }

  private static void check(boolean condition, String message) {
    if (!condition) {
      throw new AssertionError(message);
    }
  }
}
