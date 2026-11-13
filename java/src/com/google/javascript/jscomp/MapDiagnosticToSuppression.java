/*
 * Copyright 2019 The Closure Compiler Authors.
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

import com.google.javascript.jscomp.DiagnosticToSuppressionMapper.OutputFormat;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.parsing.ParserRunner;

/**
 * Prints out a map from diagnostic id to suppression.
 *
 * <p>This can be used by tooling to automatically suppress errors given a list of diagnostics or to
 * generate documentation.
 *
 * <p>A few potentially confusing aspects of this class:
 *
 * <ul>
 *   <li>Some diagnostic ids are not suppressible, and so are not included in the map.
 *   <li>Some diagnostics have multiple possible suppressions. We arbitrarily choose one
 *       suppression, with two exceptions: use visibility over accessControls, and use anything over
 *       missingSourcesWarnings. TODO(lharker): we could also make this configurable from the
 *       command line.
 *   <li>Some suppression groups are not usable in at-suppress annotations, as configured in the
 *       Parser, and are not included in the map.
 * </ul>
 */
public final class MapDiagnosticToSuppression {

  private OutputFormat output = OutputFormat.JSON;

  private boolean help = false;

  private MapDiagnosticToSuppression() {}

  public static void main(String[] args) {
    new MapDiagnosticToSuppression().printSuppressions(args);
  }

  private static void printHelp() {
    Io.stdout().println("Usage: MapDiagnosticToSuppression --output={json,md}");
  }

  private void printSuppressions(String[] args) {
    CmdLineParser parser = new CmdLineParser();
    parser
        .option("--output")
        .usage("Sets the desired output format.\n" + "Options: md, json" + "Default: json")
        .enumValue(OutputFormat.values(), () -> output, v -> { output = v; });
    parser
        .option("--help")
        .usage("Usage")
        .flag(() -> help, v -> { help = v; });
    Outcome<Void> parsed = parser.parseArgument(args);
    if (parsed.isErr()) {
      CmdLineException ex = (CmdLineException) parsed.error();
      Io.stderr().println("Could not parse arguments: " + ex);
      printHelp();
      return;
    }

    if (help) {
      printHelp();
      return;
    }

    new DiagnosticToSuppressionMapper(
            ParserRunner.getSuppressionNames(), DiagnosticGroups.getRegisteredGroups())
        .printSuppressions(output);
  }
}
