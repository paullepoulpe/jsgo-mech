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

import static com.google.common.base.Preconditions.checkArgument;

import com.google.common.base.Joiner;
import com.google.common.base.Preconditions;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSortedSet;
import com.google.common.collect.Iterables;
import com.google.javascript.jscomp.base.Charsets;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.base.SimpleSetMultimap;
import com.google.javascript.jscomp.base.TextFiles;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.Map;
import java.util.Set;

/**
 * Class that applies suggested fixes to code or files.
 */
public final class ApplySuggestedFixes {

  private static final Joiner DOUBLE_LINE_JOINER = Joiner.on("\n\n");

  /**
   * Applies the provided set of suggested fixes to the files listed in the suggested fixes.
   * The fixes can be provided in any order, but they may not have any overlapping modifications
   * for the same file.
   */
  public static Outcome<Void> applySuggestedFixesToFiles(Iterable<SuggestedFix> fixes) {
    Set<String> filenames = new LinkedHashSet<>();
    for (SuggestedFix fix : fixes) {
      filenames.addAll(fix.getReplacements().keySet());
    }

    Map<String, String> filenameToCodeMap = new LinkedHashMap<>();
    for (String filename : filenames) {
      Outcome<String> code = TextFiles.read(filename, Charsets.UTF_8);
      if (code.isErr()) {
        return Outcome.err(code.error());
      }
      filenameToCodeMap.put(filename, code.get());
    }

    ImmutableMap<String, String> newCode = applySuggestedFixesToCode(fixes, filenameToCodeMap);
    for (Map.Entry<String, String> entry : newCode.entrySet()) {
      Outcome<Void> written = TextFiles.write(entry.getKey(), Charsets.UTF_8, entry.getValue());
      if (written.isErr()) {
        return Outcome.err(written.error());
      }
    }
    return Outcome.ok();
  }

  /**
   * Applies all possible options from each {@code SuggestedFixAlternative} to the provided code and
   * returns the new code. This only makes sense if all the SuggestedFixAlternatives come from the
   * same checker, i.e. they offer the same number of choices and the same index corresponds to
   * similar fixes. The {@code filenameToCodeMap} must contain all the files that the provided fixes
   * apply to. The fixes can be provided in any order, but they may not have any overlapping
   * modifications for the same file. This function will return new code only for the files that
   * have been modified.
   */
  public static ImmutableList<ImmutableMap<String, String>> applyAllSuggestedFixChoicesToCode(
      Iterable<SuggestedFix> fixChoices, Map<String, String> fileNameToCodeMap) {
    if (Iterables.isEmpty(fixChoices)) {
      return ImmutableList.of(ImmutableMap.of());
    }
    int alternativeCount = Iterables.getFirst(fixChoices, null).getAlternatives().size();
    Preconditions.checkArgument(
        allHaveAlternativeCount(fixChoices, alternativeCount),
        "All SuggestedFixAlternatives must offer an equal number of choices for this "
            + "utility to make sense");
    ImmutableList.Builder<ImmutableMap<String, String>> result = ImmutableList.builder();
    for (int i = 0; i < alternativeCount; i++) {
      result.add(applySuggestedFixChoicesToCode(fixChoices, i, fileNameToCodeMap));
    }
    return result.build();
  }

  private static boolean allHaveAlternativeCount(
      Iterable<SuggestedFix> fixChoices, int alternativeCount) {
    for (SuggestedFix f : fixChoices) {
      if (f.getAlternatives().size() != alternativeCount) {
        return false;
      }
    }
    return true;
  }

  private static ImmutableMap<String, String> applySuggestedFixChoicesToCode(
      Iterable<SuggestedFix> fixChoices,
      final int choiceIndex,
      Map<String, String> fileNameToCodeMap) {
    ImmutableList.Builder<SuggestedFix> chosenFixesBuilder = ImmutableList.builder();
    for (SuggestedFix choices : fixChoices) {
      chosenFixesBuilder.add(choices.getAlternatives().get(choiceIndex));
    }
    ImmutableList<SuggestedFix> chosenFixes = chosenFixesBuilder.build();
    return applySuggestedFixesToCode(chosenFixes, fileNameToCodeMap);
  }

  /**
   * Applies the provided set of suggested fixes to the provided code and returns the new code,
   * ignoring alternative fixes. The {@code filenameToCodeMap} must contain all the files that the
   * provided fixes apply to. The fixes can be provided in any order, but they may not have any
   * overlapping modifications for the same file. This function will return new code only for the
   * files that have been modified.
   */
  public static ImmutableMap<String, String> applySuggestedFixesToCode(
      Iterable<SuggestedFix> fixes, Map<String, String> filenameToCodeMap) {
    ReplacementMap map = new ReplacementMap();
    for (SuggestedFix fix : fixes) {
      map.putIfNoOverlap(fix);
    }
    ImmutableMap.Builder<String, String> newCodeMap = ImmutableMap.builder();
    for (Map.Entry<String, Set<CodeReplacement>> entry : map.entrySet()) {
      String filename = entry.getKey();
      if (!filenameToCodeMap.containsKey(filename)) {
        throw new IllegalArgumentException("filenameToCodeMap missing code for file: " + filename);
      }
      Set<CodeReplacement> replacements = entry.getValue();
      String newCode = applyCodeReplacements(replacements, filenameToCodeMap.get(filename));
      newCodeMap.put(filename, newCode);
    }
    return newCodeMap.buildOrThrow();
  }

  /**
   * Applies the provided set of code replacements to the code and returns the transformed code. The
   * code replacements may not have any overlap.
   */
  public static String applyCodeReplacements(
      Collection<CodeReplacement> replacements, String code) {
    ImmutableSortedSet<CodeReplacement> sortedReplacements =
        ImmutableSortedSet.copyOf(replacements);
    checkArgument(
        !containsOverlaps(sortedReplacements),
        "Found overlap between code replacements:\n%s",
        DOUBLE_LINE_JOINER.join(sortedReplacements));

    StringBuilder sb = new StringBuilder();
    int lastIndex = 0;
    for (CodeReplacement replacement : sortedReplacements) {
      sb.append(code, lastIndex, replacement.getStartPosition());
      sb.append(replacement.getNewContent());
      lastIndex = replacement.getEndPosition();
    }
    if (lastIndex <= code.length()) {
      sb.append(code, lastIndex, code.length());
    }
    return sb.toString();
  }

  /**
   * Checks whether the `CodeReplacement`s have any overlap.
   *
   * <p>We intentinoally accept a set here beceuse we want to supprt/ignore equivalent
   * `CodeReplacement`s. It's safe if those overlap as long as we only apply one of them.
   */
  private static boolean containsOverlaps(ImmutableSortedSet<CodeReplacement> replacements) {
    int start = -1;
    for (CodeReplacement replacement : replacements) {
      if (replacement.getStartPosition() < start) {
        return true;
      }
      start = replacement.getEndPosition();
    }
    return false;
  }

  private static class ReplacementMap {
    private final SimpleSetMultimap<String, CodeReplacement> map = SimpleSetMultimap.createInsertionOrdered();

    void putIfNoOverlap(SuggestedFix fix) {
      if (canPut(fix)) {
        map.putAll(fix.getReplacements());
      }
    }

    private boolean canPut(SuggestedFix fix) {
      for (String filename : fix.getReplacements().keySet()) {
        ImmutableSortedSet<CodeReplacement> replacements =
            ImmutableSortedSet.<CodeReplacement>naturalOrder()
                .addAll(map.get(filename))
                .addAll(fix.getReplacements().get(filename))
                .build();
        if (containsOverlaps(replacements)) {
          return false;
        }
      }
      return true;
    }

    Set<Map.Entry<String, Set<CodeReplacement>>> entrySet() {
      return map.asMap().entrySet();
    }
  }

  private ApplySuggestedFixes() {}
}
