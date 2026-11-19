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
