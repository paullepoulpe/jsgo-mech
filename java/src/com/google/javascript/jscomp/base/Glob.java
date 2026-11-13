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

import java.util.ArrayList;
import java.util.List;

/**
 * File name patterns. A pattern matches a whole path, character by character:
 *
 * <ul>
 *   <li>{@code *} matches any number of characters of one name (it does not match "/");
 *   <li>{@code **} matches any number of characters, "/" included;
 *   <li>{@code ?} matches one character that is not "/";
 *   <li>{@code [abc]}, {@code [a-z]} match one character of the set, {@code [!abc]} one character
 *       that is not in the set; neither matches "/". The set has at least one character, so a
 *       "]" right after the "[" or "[!" belongs to the set;
 *   <li>{@code {a,b}} matches what any of the alternatives matches (alternatives are patterns
 *       without braces);
 *   <li>{@code \} makes the next character an ordinary one;
 *   <li>every other character matches itself. A "[" or "{" that is never closed is an ordinary
 *       character too.
 * </ul>
 */
public final class Glob {
  private Glob() {}

  public static boolean matches(String glob, String path) {
    for (String alternative : expandBraces(glob)) {
      if (matchesWithoutBraces(alternative, path)) {
        return true;
      }
    }
    return false;
  }

  /** The patterns without braces that together match what the glob matches. */
  private static List<String> expandBraces(String glob) {
    List<String> result = new ArrayList<>();
    int open = -1;
    for (int i = 0; i < glob.length() && open < 0; i++) {
      char c = glob.charAt(i);
      if (c == '\\') {
        i++;
      } else if (c == '{') {
        open = i;
      }
    }
    int close = open < 0 ? -1 : findUnescaped(glob, '}', open + 1);
    if (close < 0) {
      result.add(glob);
      return result;
    }
    String head = glob.substring(0, open);
    List<String> tails = expandBraces(glob.substring(close + 1));
    int start = open + 1;
    while (start <= close) {
      int comma = findUnescaped(glob, ',', start);
      int end = comma < 0 || comma > close ? close : comma;
      for (String tail : tails) {
        result.add(head + glob.substring(start, end) + tail);
      }
      start = end + 1;
    }
    return result;
  }

  /** The index of the first c at or after from that has no backslash before it; -1 if none. */
  private static int findUnescaped(String glob, char c, int from) {
    for (int i = from; i < glob.length(); i++) {
      if (glob.charAt(i) == '\\') {
        i++;
      } else if (glob.charAt(i) == c) {
        return i;
      }
    }
    return -1;
  }

  private static boolean matchesWithoutBraces(String glob, String path) {
    // reached[p]: the part of the glob read so far can match the first p characters of the path.
    boolean[] reached = new boolean[path.length() + 1];
    reached[0] = true;
    int i = 0;
    while (i < glob.length()) {
      char c = glob.charAt(i);
      boolean[] next = new boolean[path.length() + 1];
      if (c == '*') {
        boolean acrossNames = i + 1 < glob.length() && glob.charAt(i + 1) == '*';
        i += acrossNames ? 2 : 1;
        boolean open = false;
        for (int p = 0; p <= path.length(); p++) {
          if (p > 0 && path.charAt(p - 1) == '/' && !acrossNames) {
            open = false;
          }
          open = open || reached[p];
          next[p] = open;
        }
      } else {
        int end = i + 1; // the end of this element of the glob
        int classClose = c == '[' ? glob.indexOf(']', i + 2) : -1;
        if (classClose >= 0) {
          end = classClose + 1;
        } else if (c == '\\' && i + 1 < glob.length()) {
          end = i + 2;
        }
        for (int p = 0; p < path.length(); p++) {
          if (reached[p]) {
            char actual = path.charAt(p);
            if (classClose >= 0) {
              next[p + 1] = actual != '/' && classHas(glob.substring(i + 1, classClose), actual);
            } else if (c == '?') {
              next[p + 1] = actual != '/';
            } else {
              next[p + 1] = actual == glob.charAt(end - 1);
            }
          }
        }
        i = end;
      }
      reached = next;
    }
    return reached[path.length()];
  }

  /** Whether the class (the text between the brackets) has the character. */
  private static boolean classHas(String body, char c) {
    boolean negated = body.startsWith("!");
    boolean found = false;
    int i = negated ? 1 : 0;
    while (i < body.length()) {
      char low = body.charAt(i);
      char high = low;
      if (i + 2 < body.length() && body.charAt(i + 1) == '-') {
        high = body.charAt(i + 2);
        i += 2;
      }
      found = found || (c >= low && c <= high);
      i++;
    }
    return found != negated;
  }
}
