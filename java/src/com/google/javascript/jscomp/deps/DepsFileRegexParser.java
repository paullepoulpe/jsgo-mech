/*
 * Copyright 2008 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.deps;

import static com.google.javascript.jscomp.base.JavaRegex.DOT;
import static com.google.javascript.jscomp.base.JavaRegex.SPACE;

import com.google.common.base.CharMatcher;
import com.google.common.base.Function;
import com.google.common.base.Functions;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.javascript.jscomp.ErrorManager;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.deps.DependencyInfo.Require;
import com.google.re2j.Matcher;
import com.google.re2j.Pattern;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * A parser that can extract dependency information from existing deps.js files.
 *
 * <p>See //javascript/closure/deps.js for an example file.
 */
public final class DepsFileRegexParser extends JsFileLineParser {
  private static final Logger logger = Logger.getLogger(DepsFileRegexParser.class.getName());

  /**
   * Pattern for matching JavaScript string literals. The group is:
   * goog.addDependency({1});
   */
  private final Matcher depMatcher =
      Pattern.compile(SPACE + "*goog" + DOT + "addDependency\\((" + DOT + "*)\\);?" + SPACE + "*")
          .matcher("");

  /**
   * Pattern for matching the args of a goog.addDependency(). The group is:
   * goog.addDependency({1}, {2}, {3}, {4?});
   */
  private final Matcher depArgsMatch =
      Pattern.compile(
              SPACE
                  + "*([^,]*), (\\[[^\\]]*\\]), (\\[[^\\]]*\\])"
                  + "(?:, (true|false|\\{[^{}]*\\}))?"
                  + SPACE
                  + "*")
          .matcher("");

  /**
   * The dependency information extracted from the current file.
   */
  private List<DependencyInfo> depInfos;

  /** Translates paths in different build systems. */
  private final Function<String, String> pathTranslator;

  /**
   * Constructor
   *
   * @param errorManager Handles parse errors.
   */
  public DepsFileRegexParser(ErrorManager errorManager) {
    this(Functions.identity(), errorManager);
  }

  /**
   * @param pathTranslator Translates paths in different build systems.
   * @param errorManager Handles parse errors.
   */
  public DepsFileRegexParser(Function<String, String> pathTranslator, ErrorManager errorManager) {
    super(errorManager);
    this.pathTranslator = pathTranslator;
  }

  /**
   * Parses the given file and returns a list of dependency information that it
   * contained.
   *
   * @param filePath Path to the file to parse.
   * @return A list of DependencyInfo objects, or the error if the file could not be read.
   */
  public Outcome<List<DependencyInfo>> parseFile(String filePath) {
    Outcome<String> fileContents = Io.readText(filePath, Io.UTF_8);
    if (fileContents.isErr()) {
      return Outcome.err(fileContents.error());
    }
    return Outcome.ok(parseFile(filePath, fileContents.get()));
  }

  /**
   * Parses the given file and returns a list of dependency information that it
   * contained.
   * It uses the passed in fileContents instead of reading the file.
   *
   * @param filePath Path to the file to parse.
   * @param fileContents The contents to parse.
   * @return A list of DependencyInfo objects.
   */
  public List<DependencyInfo> parseFile(String filePath, String fileContents) {
    depInfos = new ArrayList<>();
    if (logger.isLoggable(Level.FINE)) {
      logger.fine("Parsing Dep: " + filePath);
    }
    doParse(filePath, fileContents);
    return depInfos;
  }

  /**
   * Extracts dependency information from lines that look like
   *   goog.addDependency('pathRelativeToClosure', ['provides'], ['requires']);
   * Adds the dependencies to depInfos.
   *
   * <p>Returns an error outcome with a ParseException if the given line has a malformed
   *     goog.addDependency().
   */
  @Override
  protected Outcome<Boolean> parseLine(String line) {
    boolean hasDependencies = false;

    // Quick check that will catch most cases. This is a performance win for teams with a lot of JS.
    if (line.contains("addDependency")) {
      depMatcher.reset(line);
      // See if the line looks like: goog.addDependency(...)
      if (depMatcher.matches()) {
        hasDependencies = true;
        String addDependencyParams = depMatcher.group(1);
        depArgsMatch.reset(addDependencyParams);
        // Extract the three parameters.
        if (!depArgsMatch.matches()) {
          // Although we could recover, we mark this as fatal since there should
          // not be problems with generated deps.js files.
          return Outcome.err(
              new ParseException(
                  "Invalid arguments to goog.addDependency(). Found: " + addDependencyParams,
                  true));
        }
        // Parse the file path.
        Outcome<String> relativePathOutcome = parseJsString(depArgsMatch.group(1));
        if (relativePathOutcome.isErr()) {
          return Outcome.err(relativePathOutcome.error());
        }
        String relativePath = relativePathOutcome.get();
        String path = pathTranslator.apply(relativePath);

        Outcome<List<String>> providesOutcome = parseJsStringArray(depArgsMatch.group(2));
        if (providesOutcome.isErr()) {
          return Outcome.err(providesOutcome.error());
        }
        List<String> provides = providesOutcome.get();
        Outcome<Map<String, String>> loadFlagsOutcome = parseLoadFlags(depArgsMatch.group(4));
        if (loadFlagsOutcome.isErr()) {
          return Outcome.err(loadFlagsOutcome.error());
        }
        Map<String, String> loadFlags = loadFlagsOutcome.get();

        // ES6 modules are require'd by path but do not provide them in the addDependency call.
        if ("es6".equals(loadFlags.get("module"))) {
          provides.add(relativePath);
        }

        Outcome<List<String>> requiresOutcome = parseJsStringArray(depArgsMatch.group(3));
        if (requiresOutcome.isErr()) {
          return Outcome.err(requiresOutcome.error());
        }
        DependencyInfo depInfo =
            SimpleDependencyInfo.builder(path, filePath)
                .setProvides(provides)
                .setRequires(requiresParsedFromDeps(requiresOutcome.get()))
                .setLoadFlags(loadFlags)
                .build();

        if (logger.isLoggable(Level.FINE)) {
          logger.fine("Found dep: " + depInfo);
        }
        depInfos.add(depInfo);
      }
    }

    return Outcome.ok(
        !shortcutMode || hasDependencies || CharMatcher.whitespace().matchesAllOf(line));
  }

  private static ImmutableList<Require> requiresParsedFromDeps(List<String> symbols) {
    ImmutableList.Builder<Require> result = ImmutableList.builder();
    for (String symbol : symbols) {
      result.add(Require.parsedFromDeps(symbol));
    }
    return result.build();
  }

  private Outcome<Map<String, String>> parseLoadFlags(String loadFlags) {
    if (loadFlags == null || loadFlags.equals("false")) {
      return Outcome.ok(ImmutableMap.of());
    } else if (loadFlags.equals("true")) {
      return Outcome.ok(ImmutableMap.of("module", "goog"));
    } else {
      return parseJsStringMap(loadFlags);
    }
  }
}
