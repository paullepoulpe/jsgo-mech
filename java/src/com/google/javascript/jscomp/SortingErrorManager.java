/*
 * Copyright 2018 The Closure Compiler Authors.
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

package com.google.javascript.jscomp;


import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableSet;
import java.util.Comparator;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.TreeSet;

/**
 * A customizable error manager that sorts all errors and warnings reported to it, and has
 * customizable output through the {@link ErrorReportGenerator} interface.
 */
public class SortingErrorManager implements ErrorManager {

  private final TreeSet<ErrorWithLevel> messages = new TreeSet<>(new LeveledJSErrorComparator());
  private int originalErrorCount = 0;
  private int promotedErrorCount = 0;
  private int warningCount = 0;
  private double typedPercent = 0.0;

  /** Responsible for generating the report of the errors at the end of compilation */
  ImmutableSet<ErrorReportGenerator> errorReportGenerators;

  public SortingErrorManager(Set<ErrorReportGenerator> errorReportGenerators) {
    this.errorReportGenerators = ImmutableSet.copyOf(errorReportGenerators);
  }

  @Override
  public void report(CheckLevel level, JSError error) {
    ErrorWithLevel e = new ErrorWithLevel(error, level);
    if (messages.add(e)) {
      if (level == CheckLevel.ERROR) {
        if (error.type().level == CheckLevel.ERROR) {
          originalErrorCount++;
        } else {
          promotedErrorCount++;
        }
      } else if (level == CheckLevel.WARNING) {
        warningCount++;
      }
    }
  }

  @Override
  public boolean hasHaltingErrors() {
    return originalErrorCount != 0;
  }

  @Override
  public int getErrorCount() {
    return originalErrorCount + promotedErrorCount;
  }

  @Override
  public int getWarningCount() {
    return warningCount;
  }

  @Override
  public ImmutableList<JSError> getErrors() {
    return filterToListOf(CheckLevel.ERROR);
  }

  @Override
  public ImmutableList<JSError> getWarnings() {
    return filterToListOf(CheckLevel.WARNING);
  }

  Iterable<ErrorWithLevel> getSortedDiagnostics() {
    // TODO(b/114762232): It should be possible to remove the copying here and switch to an
    // unmodifiable collection once we get rid of usages that add warnings during generateReport.
    return ImmutableList.copyOf(messages);
  }

  @Override
  public void setTypedPercent(double typedPercent) {
    this.typedPercent = typedPercent;
  }

  @Override
  public double getTypedPercent() {
    return typedPercent;
  }

  private ImmutableList<JSError> filterToListOf(CheckLevel level) {
    ImmutableList.Builder<JSError> result = ImmutableList.builder();
    for (ErrorWithLevel e : messages) {
      if (e.level == level) {
        result.add(e.error);
      }
    }
    return result.build();
  }

  // TODO(b/114762232): It should be invalid to report errors during the execution of this method;
  // doing so will become impossible once all subclases have migrated to an ErrorReportGenerator.
  @Override
  public void generateReport() {
    for (ErrorReportGenerator generator : this.errorReportGenerators) {
      generator.generateReport(this);
    }
  }

  /** Strategy for customizing the output format of the error report */
  public interface ErrorReportGenerator {
    void generateReport(SortingErrorManager manager);
  }

  /**
   * Comparator of {@link JSError} with an associated {@link CheckLevel}. The ordering is the
   * standard lexical ordering on the quintuple (file name, line number, {@link CheckLevel},
   * character number, description).
   *
   * <p>Note: this comparator imposes orderings that are inconsistent with {@link
   * JSError#equals(Object)}.
   */
  static final class LeveledJSErrorComparator implements Comparator<ErrorWithLevel> {
    private static final int P1_LT_P2 = -1;
    private static final int P1_GT_P2 = 1;

    @Override
    public int compare(ErrorWithLevel p1, ErrorWithLevel p2) {
      // null is the smallest value
      if (p2 == null) {
        if (p1 == null) {
          return 0;
        } else {
          return P1_GT_P2;
        }
      }

      // check level
      if (p1.level != p2.level) {
        return p2.level.compareTo(p1.level);
      }

      // sourceName comparison
      Optional<String> source1 = p1.error.sourceName();
      Optional<String> source2 = p2.error.sourceName();
      if (source1.isPresent() && source2.isPresent()) {
        int sourceCompare = source1.get().compareTo(source2.get());
        if (sourceCompare != 0) {
          return sourceCompare;
        }
      } else if (source1.isEmpty() && source2.isPresent()) {
        return P1_LT_P2;
      } else if (source1.isPresent() && source2.isEmpty()) {
        return P1_GT_P2;
      }
      // lineno comparison
      int lineno1 = p1.error.getLineNumber();
      int lineno2 = p2.error.getLineNumber();
      if (lineno1 != lineno2) {
        return lineno1 - lineno2;
      } else if (lineno1 < 0 && 0 <= lineno2) {
        return P1_LT_P2;
      } else if (0 <= lineno1 && lineno2 < 0) {
        return P1_GT_P2;
      }
      // charno comparison
      int charno1 = p1.error.charno();
      int charno2 = p2.error.charno();
      if (charno1 != charno2) {
        return charno1 - charno2;
      } else if (charno1 < 0 && 0 <= charno2) {
        return P1_LT_P2;
      } else if (0 <= charno1 && charno2 < 0) {
        return P1_GT_P2;
      }
      // description
      return p1.error.description().compareTo(p2.error.description());
    }
  }

  static class ErrorWithLevel {
    final JSError error;
    final CheckLevel level;

    ErrorWithLevel(JSError error, CheckLevel level) {
      this.error = error;
      this.level = level;
    }

    @Override
    public int hashCode() {
      return Objects.hash(
          level, error.description(), error.sourceName().orElse(null), error.getLineNumber(), error.charno());
    }

    @Override
    public boolean equals(Object obj) {
      if (!(obj instanceof ErrorWithLevel e)) {
        return false;
      }
      return Objects.equals(level, e.level)
          && error.description().equals(e.error.description())
          && error.sourceName().equals(e.error.sourceName())
          && error.getLineNumber() == e.error.getLineNumber()
          && error.charno() == e.error.charno();
    }
  }
}
