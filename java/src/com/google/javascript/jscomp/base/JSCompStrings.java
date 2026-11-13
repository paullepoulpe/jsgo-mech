/*
 * Copyright 2020 The Closure Compiler Authors.
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

import java.util.Optional;
import org.jspecify.annotations.Nullable;

/** Basic String functions used by JSComp. */
public final class JSCompStrings {

  public static String lines(String... lines) {
    return String.join("\n", lines);
  }

  public static String linesIterable(Iterable<String> lines) {
    return String.join("\n", lines);
  }

  /**
   * Like {@link Integer#parseInt(String)}, but returns null where that throws
   * NumberFormatException: the string must be an optional '+' or '-' followed by one or more
   * digits (as {@link Character#digit} defines them) whose value fits an int.
   */
  public static @Nullable Integer parseIntOrNull(Optional<String> s) {
    if (s.isEmpty() || s.get().isEmpty()) {
      return null;
    }
    boolean negative = s.get().charAt(0) == '-';
    int start = negative || s.get().charAt(0) == '+' ? 1 : 0;
    if (start == s.get().length()) {
      return null;
    }
    long value = 0;
    for (int i = start; i < s.get().length(); i++) {
      int digit = Character.digit(s.get().charAt(i), 10);
      if (digit < 0) {
        return null;
      }
      value = value * 10 + digit;
      if (value > -(long) Integer.MIN_VALUE) {
        return null;
      }
    }
    if (negative) {
      value = -value;
    }
    if (value > Integer.MAX_VALUE) {
      return null;
    }
    return (int) value;
  }

  private JSCompStrings() {
    throw new AssertionError();
  }
}
