/*
 * Copyright 2014 The Closure Compiler Authors.
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

package com.google.javascript.refactoring;


import com.google.errorprone.annotations.Immutable;
import java.util.Comparator;

/**
 * Class that represents a set of changes to make to the code.
 *
 * @param getStartPosition Returns the start position within the file that the modification should
 *     be applied starting at.
 * @param getLength Returns how many characters the new content should replace in the original
 *     content.
 * @param getNewContent Returns the new content that should be inserted into the file.
 * @param getSortKey Returns an additional String key that can be used to sort replacements using
 *     lexical ordering.
 */
@Immutable
public record CodeReplacement(
    int getStartPosition, int getLength, String getNewContent, String getSortKey)
    implements Comparable<CodeReplacement> {
  public CodeReplacement {
  }

  @Override
  public String toString() {
    return "CodeReplacement{"
        + "startPosition=" + getStartPosition + ", "
        + "length=" + getLength + ", "
        + "newContent=" + getNewContent + ", "
        + "sortKey=" + getSortKey
        + "}";
  }

  static CodeReplacement create(int startPosition, int length, String newContent) {
    return create(startPosition, length, newContent, "");
  }

  static CodeReplacement create(int startPosition, int length, String newContent, String sortKey) {
    return new CodeReplacement(startPosition, length, newContent, sortKey);
  }

  /**
   * Returns the end position within the file that the modification
   * should be applied starting at.
   */
  public int getEndPosition() {
    return getStartPosition() + getLength();
  }

  @Override
  public final int compareTo(CodeReplacement x) {
    return APPLICATION_ORDER.compare(this, x);
  }

  // Remember to compare every field so that it's consistent with equals.
  private static final Comparator<CodeReplacement> APPLICATION_ORDER =
      Comparator.comparingInt(CodeReplacement::getStartPosition)
          .thenComparingInt(CodeReplacement::getLength)
          .thenComparing(CodeReplacement::getSortKey)
          .thenComparing(CodeReplacement::getNewContent);
}
