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
 * RE2 spellings of the JDK regular expression constructs that mean something else in RE2.
 *
 * <p>The compiler's regular expressions are compiled by RE2 ({@code com.google.re2j}). These
 * fragments keep the patterns that were written for the JDK engine matching exactly the
 * strings they always matched.
 */
public final class JavaRegex {
  /**
   * What {@code .} matches in a JDK regular expression (without DOTALL): any character except a line
   * terminator. In RE2, {@code .} excludes only {@code \n}.
   */
  public static final String DOT = "[^\\n\\r\\x{85}\\x{2028}\\x{2029}]";

  /**
   * What {@code \s} matches in a JDK regular expression: {@code [ \t\n\x0B\f\r]}. In RE2, {@code \s}
   * does not include {@code \x0B}.
   */
  public static final String SPACE = "[\\s\\x0B]";

  /** What {@code \S} matches in a JDK regular expression; see {@link #SPACE}. */
  public static final String NON_SPACE = "[^\\s\\x0B]";

  /**
   * Where {@code $} matches in a JDK regular expression (without MULTILINE): at the end of the input
   * and before a line terminator that ends the input. In RE2, {@code $} matches only at the end of
   * the input. Unlike {@code $} this consumes the line terminator, so it can only stand at the very
   * end of a pattern whose whole-match text is not used.
   */
  public static final String END = "(?:\\r\\n|[\\n\\r\\x{85}\\x{2028}\\x{2029}])?\\z";

  private JavaRegex() {}
}
