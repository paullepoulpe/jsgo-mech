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

package com.google.javascript.jscomp;

import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.regex.Pattern;

/**
 * An ErrorManager that verifies expected diagnostics and reports missing or unexpected ones.
 *
 * <p>It wraps a delegate ErrorManager. Expected diagnostics (passed as regexes) are swallowed if
 * matched. Unexpected diagnostics are passed to the delegate. At the end of compilation (when
 * generateReport is called), any unmatched expectations are reported as errors to the delegate.
 */
public final class VerifyingErrorManager extends ThreadSafeDelegatingErrorManager {

  private final List<Pattern> unmatchedExpectations;
  private final LightweightMessageFormatter formatter;

  public VerifyingErrorManager(ErrorManager delegate, List<String> expectedDiagnostics) {
    super(delegate);
    this.formatter = LightweightMessageFormatter.withoutSource();
    this.unmatchedExpectations = new ArrayList<>();
    for (String exp : expectedDiagnostics) {
      this.unmatchedExpectations.add(Pattern.compile(exp));
    }
  }

  @Override
  public synchronized void report(CheckLevel level, JSError error) {
    String formattedError = formatErrorForMatching(level, error);

    Pattern matchedPattern = null;
    Iterator<Pattern> iterator = unmatchedExpectations.iterator();
    while (iterator.hasNext()) {
      Pattern p = iterator.next();
      if (p.matcher(formattedError).find()) {
        if (matchedPattern == null) {
          matchedPattern = p;
          iterator.remove();
          continue;
        }

        if (matchedPattern.toString().equals(p.toString())) {
          // Allow duplicates that are identical.
          continue;
        }

        super.report(
            CheckLevel.ERROR,
            JSError.make(
                AbstractCommandLineRunner.AMBIGUOUS_EXPECTATION,
                formattedError,
                matchedPattern.toString(),
                p.toString()));
        iterator.remove();
      }
    }

    if (matchedPattern == null) {
      super.report(level, error);
    }
  }

  @Override
  public synchronized void generateReport() {
    reportMissingExpectations();
    super.generateReport();
  }

  private synchronized void reportMissingExpectations() {
    for (Pattern p : unmatchedExpectations) {
      super.report(
          CheckLevel.ERROR,
          JSError.make(AbstractCommandLineRunner.EXPECTED_DIAGNOSTIC_NOT_FOUND, p.toString()));
    }
    unmatchedExpectations.clear();
  }

  private String formatErrorForMatching(CheckLevel level, JSError error) {
    return level == CheckLevel.ERROR
        ? formatter.formatError(error)
        : formatter.formatWarning(error);
  }
}
