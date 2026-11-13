/*
 *
 * ***** BEGIN LICENSE BLOCK *****
 * Version: MPL 1.1/GPL 2.0
 *
 * The contents of this file are subject to the Mozilla Public License Version
 * 1.1 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * http://www.mozilla.org/MPL/
 *
 * Software distributed under the License is distributed on an "AS IS" basis,
 * WITHOUT WARRANTY OF ANY KIND, either express or implied. See the License
 * for the specific language governing rights and limitations under the
 * License.
 *
 * The Original Code is Rhino code, released
 * May 6, 1999.
 *
 * The Initial Developer of the Original Code is
 * Netscape Communications Corporation.
 * Portions created by the Initial Developer are Copyright (C) 1997-1999
 * the Initial Developer. All Rights Reserved.
 *
 * Contributor(s):
 *   Bob Jervis
 *   Google Inc.
 *
 * Alternatively, the contents of this file may be used under the terms of
 * the GNU General Public License Version 2 or later (the "GPL"), in which
 * case the provisions of the GPL are applicable instead of those above. If
 * you wish to allow use of your version of this file only under the terms of
 * the GPL and not to allow others to use your version of this file under the
 * MPL, indicate your decision by deleting the provisions above and replacing
 * them with the notice and other provisions required by the GPL. If you do
 * not delete the provisions above, a recipient may use your version of this
 * file under either the MPL or the GPL.
 *
 * ***** END LICENSE BLOCK ***** */

package com.google.javascript.rhino.testing;

import static com.google.common.base.Preconditions.checkNotNull;
import static com.google.common.truth.Fact.fact;

import com.google.common.base.Splitter;
import com.google.common.collect.ImmutableList;
import com.google.common.truth.Fact;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
import java.util.ArrayList;
import java.util.List;

/**
 * Builds Fact objects for use with the Truth library that represent the difference between two text
 * strings.
 */
public final class TextDiffFactsBuilder {

  private final String title;
  private String expectedTextField;
  private String actualTextField;

  public TextDiffFactsBuilder(String title) {
    this.title = title;
  }

  @CanIgnoreReturnValue
  public TextDiffFactsBuilder expectedText(String expectedText) {
    this.expectedTextField = expectedText;
    return this;
  }

  @CanIgnoreReturnValue
  public TextDiffFactsBuilder actualText(String actualText) {
    this.actualTextField = actualText;
    return this;
  }

  /**
   * Returns one or more Fact objects representing the difference between the expected and actual
   * text strings, which must be specified by calling their methods before calling this one.
   */
  public ImmutableList<Fact> build() {
    // Split the text into lines and diff them with a plain longest-common-subsequence table.
    final Splitter lineSplitter = Splitter.on('\n');
    final List<String> expectedLines = lineSplitter.splitToList(checkNotNull(expectedTextField));
    final List<String> actualLines = lineSplitter.splitToList(checkNotNull(actualTextField));
    final List<String> unifiedDiff =
        generateUnifiedDiff(expectedLines, actualLines, /* contextSize= */ 10);
    return ImmutableList.of(fact(title, String.join("\n", unifiedDiff)));
  }

  /**
   * Returns the unified diff ("--- expected", "+++ actual", then "@@" hunks whose lines are
   * prefixed with ' ', '-' or '+') that turns {@code expected} into {@code actual}, or an empty
   * list when they are equal.
   */
  private static List<String> generateUnifiedDiff(
      List<String> expected, List<String> actual, int contextSize) {
    // Lines before `prefix` and after the last `suffix` lines are common to both texts.
    int prefix = 0;
    while (prefix < expected.size()
        && prefix < actual.size()
        && expected.get(prefix).equals(actual.get(prefix))) {
      prefix++;
    }
    int suffix = 0;
    while (suffix < expected.size() - prefix
        && suffix < actual.size() - prefix
        && expected
            .get(expected.size() - 1 - suffix)
            .equals(actual.get(actual.size() - 1 - suffix))) {
      suffix++;
    }
    final int n = expected.size() - prefix - suffix;
    final int m = actual.size() - prefix - suffix;
    // lcs[i][j] is the length of the longest common subsequence of the middle parts from i and j.
    final int[][] lcs = new int[n + 1][m + 1];
    for (int i = n - 1; i >= 0; i--) {
      for (int j = m - 1; j >= 0; j--) {
        lcs[i][j] =
            expected.get(prefix + i).equals(actual.get(prefix + j))
                ? lcs[i + 1][j + 1] + 1
                : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
      }
    }
    // One entry per output line: its tag (' ', '-' or '+') and its text.
    final List<Character> tags = new ArrayList<>();
    final List<String> lines = new ArrayList<>();
    for (int i = 0; i < prefix; i++) {
      tags.add(' ');
      lines.add(expected.get(i));
    }
    int i = 0;
    int j = 0;
    while (i < n || j < m) {
      if (i < n && j < m && expected.get(prefix + i).equals(actual.get(prefix + j))) {
        tags.add(' ');
        lines.add(expected.get(prefix + i));
        i++;
        j++;
      } else if (i < n && (j == m || lcs[i + 1][j] >= lcs[i][j + 1])) {
        tags.add('-');
        lines.add(expected.get(prefix + i));
        i++;
      } else {
        tags.add('+');
        lines.add(actual.get(prefix + j));
        j++;
      }
    }
    for (int k = expected.size() - suffix; k < expected.size(); k++) {
      tags.add(' ');
      lines.add(expected.get(k));
    }

    final List<String> result = new ArrayList<>();
    int expectedLine = 1; // 1-based line number in `expected` of entry `pos`
    int actualLine = 1; // 1-based line number in `actual` of entry `pos`
    int pos = 0;
    while (pos < tags.size()) {
      if (tags.get(pos) == ' ') {
        expectedLine++;
        actualLine++;
        pos++;
        continue;
      }
      // A hunk starts contextSize lines before a change and ends contextSize lines after the last
      // change that is followed by more than 2 * contextSize common lines.
      final int hunkStart = Math.max(0, pos - contextSize);
      int lastChange = pos;
      for (int k = pos + 1; k < tags.size() && k <= lastChange + 2 * contextSize + 1; k++) {
        if (tags.get(k) != ' ') {
          lastChange = k;
        }
      }
      final int hunkEnd = Math.min(tags.size(), lastChange + 1 + contextSize);
      int expectedCount = 0;
      int actualCount = 0;
      for (int k = hunkStart; k < hunkEnd; k++) {
        if (tags.get(k) != '+') {
          expectedCount++;
        }
        if (tags.get(k) != '-') {
          actualCount++;
        }
      }
      if (result.isEmpty()) {
        result.add("--- expected");
        result.add("+++ actual");
      }
      result.add(
          "@@ -"
              + (expectedLine - (pos - hunkStart))
              + ","
              + expectedCount
              + " +"
              + (actualLine - (pos - hunkStart))
              + ","
              + actualCount
              + " @@");
      for (int k = hunkStart; k < hunkEnd; k++) {
        result.add(tags.get(k) + lines.get(k));
      }
      for (int k = pos; k < hunkEnd; k++) {
        if (tags.get(k) != '+') {
          expectedLine++;
        }
        if (tags.get(k) != '-') {
          actualLine++;
        }
      }
      pos = hunkEnd;
    }
    return result;
  }
}
