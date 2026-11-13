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

/**
 * The little the compiler needs to know about a URI reference (a URL, or just a path): whether
 * it is written with legal characters, whether it has a scheme, and its path.
 */
public final class Uris {
  private Uris() {}

  /** The ASCII characters, other than letters and digits, that a URI is written with. */
  private static final String LEGAL_PUNCTUATION = "_-!.~'()*;/?:@&=+$,%#";

  /**
   * Whether the text is written like a URI: its ASCII characters are letters, digits and {@code
   * _-!.~'()*;/?:@&=+$,#%}, each "%" is followed by two hexadecimal digits, the other characters
   * are neither spaces nor control characters, and if there is a ":" before the first "/", "?" or
   * "#", a scheme stands before it and something after it.
   */
  public static boolean isValid(String uri) {
    for (int i = 0; i < uri.length(); i++) {
      char c = uri.charAt(i);
      if (c >= 0x80) {
        if (Character.isSpaceChar(c) || Character.isISOControl(c)) {
          return false;
        }
      } else if (!isAsciiLetterOrDigit(c) && LEGAL_PUNCTUATION.indexOf(c) < 0) {
        return false;
      }
      if (c == '%'
          && (i + 2 >= uri.length()
              || !isHexDigit(uri.charAt(i + 1))
              || !isHexDigit(uri.charAt(i + 2)))) {
        return false;
      }
    }
    int colon = firstColon(uri);
    return colon < 0 || (colon > 0 && schemeLength(uri) == colon && colon + 1 < uri.length());
  }

  /** Whether the text starts with a scheme: a letter, then letters, digits, "+", "-", ".", then ":". */
  public static boolean hasScheme(String uri) {
    return schemeLength(uri) > 0;
  }

  /**
   * The path of the URI: what follows the scheme and, if "//" comes next, the authority (which
   * ends before the next "/"), up to the first "?" or "#". Percent escapes stay as they are.
   */
  public static String path(String uri) {
    int start = hasScheme(uri) ? schemeLength(uri) + 1 : 0;
    if (uri.startsWith("//", start)) {
      start += 2;
      while (start < uri.length() && "/?#".indexOf(uri.charAt(start)) < 0) {
        start++;
      }
    }
    int end = start;
    while (end < uri.length() && "?#".indexOf(uri.charAt(end)) < 0) {
      end++;
    }
    return uri.substring(start, end);
  }

  /** The index of a ":" that comes before every "/", "?" and "#"; -1 if there is none. */
  private static int firstColon(String uri) {
    for (int i = 0; i < uri.length(); i++) {
      char c = uri.charAt(i);
      if (c == ':') {
        return i;
      }
      if (c == '/' || c == '?' || c == '#') {
        return -1;
      }
    }
    return -1;
  }

  /** The length of the scheme the text starts with; 0 if it does not start with a scheme. */
  private static int schemeLength(String uri) {
    int i = 0;
    while (i < uri.length()) {
      char c = uri.charAt(i);
      if (c == ':') {
        return i;
      }
      boolean letter = (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z');
      boolean other = (c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.';
      if (!letter && !(other && i > 0)) {
        return 0;
      }
      i++;
    }
    return 0;
  }

  private static boolean isAsciiLetterOrDigit(char c) {
    return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9');
  }

  private static boolean isHexDigit(char c) {
    return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F');
  }
}
