/*
 * Copyright 2015 The Closure Compiler Authors.
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

import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.deps.ModuleLoader.ResolutionMode;
import java.util.ArrayList;
import java.util.List;

/**
 * Minimal binary that just runs the "lint" checks which can be run on a single file at a time. This
 * means some checks in the lintChecks DiagnosticGroup are skipped, since they depend on type
 * information.
 */
public final class LinterMain {
  private boolean fix = false;

  private ResolutionMode resolutionMode = ResolutionMode.BROWSER;

  private List<String> files = new ArrayList<>();

  public static void main(String[] args) {
    Outcome<Void> done = new LinterMain().run(args);
    if (done.isErr()) {
      // An I/O failure ends the program the way an uncaught exception does.
      Io.stderr().print("Exception in thread \"main\" ");
      Io.stderr().printStackTrace(done.error());
      System.exit(1);
    }
  }

  private Outcome<Void> run(String[] args) {
    CmdLineParser parser = new CmdLineParser();
    parser
        .option("--fix")
        .usage("Fix lint warnings automatically")
        .flag(() -> fix, v -> { fix = v; });
    parser
        .option("--resolution_mode")
        .usage("Expected path resolution mode for resolving module specifiers.")
        .enumValue(ResolutionMode.values(), () -> resolutionMode, v -> { resolutionMode = v; });
    parser.argument(() -> files, v -> { files = v; });
    parser.parseArgument(args).get();

    Linter linter = Linter.builder().withModuleResolutionMode(resolutionMode).build();

    for (String filename : files) {
      if (fix) {
        Outcome<Void> fixed = linter.fixRepeatedly(filename);
        if (fixed.isErr()) {
          return Outcome.err(fixed.error());
        }
      } else {
        linter.lint(filename);
      }
    }
    return Outcome.ok();
  }
}
