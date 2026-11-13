/*
 * Copyright 2011 The Closure Compiler Authors.
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

import static com.google.common.base.Preconditions.checkNotNull;

import com.google.common.base.Joiner;
import com.google.common.base.Splitter;
import com.google.common.collect.ImmutableSet;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.base.SimpleSetMultimap;
import com.google.re2j.Pattern;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import org.jspecify.annotations.Nullable;

/**
 * An extension of {@code WarningsGuard} that provides functionality to maintain a list of warnings
 * (allowlist). It is subclasses' responsibility to decide what to do with the allowlist by
 * implementing the {@code level} function. Warnings are defined by the name of the JS file and the
 * first line of warnings description.
 */
public class AllowlistWarningsGuard extends WarningsGuard {
  private static final Pattern BLAZE_OUT_DIR_PATTERN =
      Pattern.compile("blaze-out/[^/]*/(bin|genfiles)/");

  private static final Splitter LINE_SPLITTER = Splitter.on('\n');

  /** The set of allowlisted warnings, same format as {@code formatWarning}. */
  private final ImmutableSet<String> allowlist;

  /** Pattern to match line number in error descriptions. */
  private static final Pattern LINE_NUMBER = Pattern.compile(":-?\\d+");

  public AllowlistWarningsGuard() {
    this(ImmutableSet.of());
  }

  /**
   * This class depends on an input set that contains the allowlist. The format of each allowlist
   * string is: {@code <file-name>:<line-number>? <warning-description>} {@code #
   * <optional-comment>}
   *
   * @param allowlist The set of JS-warnings that are allowlisted. This is expected to have similar
   *     format as {@code formatWarning(JSError)}.
   */
  public AllowlistWarningsGuard(Set<String> allowlist) {
    checkNotNull(allowlist);
    this.allowlist = normalizeAllowlist(allowlist);
  }

  /**
   * Loads legacy warnings list from the set of strings. During development line numbers are changed
   * very often - we just cut them and compare without ones.
   *
   * <p>Also remove lines starting with "#" or are blank lines.
   *
   * @return known legacy warnings without line numbers.
   */
  public static ImmutableSet<String> normalizeAllowlist(Set<String> allowlist) {
    Set<String> result = new LinkedHashSet<>();
    for (String line : allowlist) {
      String trimmed = line.trim();
      if (trimmed.isEmpty() || trimmed.charAt(0) == '#') {
        // strip out empty lines and comments.
        continue;
      }

      // Strip line number for matching.
      result.add(LINE_NUMBER.matcher(trimmed).replaceFirst(":"));
    }
    return ImmutableSet.copyOf(result);
  }

  @Override
  public @Nullable CheckLevel level(JSError error) {
    if (error.defaultLevel().equals(CheckLevel.ERROR)) {
      return null;
    }
    if (!allowlist.isEmpty() && containWarning(formatWarning(error))) {
      // If the message matches the guard we use WARNING, so that it
      // - Shows up on stderr, and
      // - Gets caught by the AllowlistBuilder downstream in the pipeline
      return CheckLevel.WARNING;
    }
    return null;
  }

  /**
   * Determines whether a given warning is included in the allowlist.
   *
   * @param formattedWarning the warning formatted by {@code formattedWarning}
   * @return whether the given warning is allowlisted or not.
   */
  protected boolean containWarning(String formattedWarning) {
    return allowlist.contains(formattedWarning);
  }

  @Override
  public int getPriority() {
    return WarningsGuard.Priority.SUPPRESS_BY_ALLOWLIST.getValue();
  }

  /** Creates a warnings guard from a file. */
  public static AllowlistWarningsGuard fromFile(String file) {
    return new AllowlistWarningsGuard(loadAllowlistedJsWarningsFile(file));
  }

  /**
   * Loads legacy warnings list from the file.
   *
   * @return The lines of the file.
   */
  public static Set<String> loadAllowlistedJsWarningsFile(String file) {
    Outcome<String> text = Io.readText(file, Io.UTF_8);
    if (text.isErr()) {
      throw new RuntimeException(text.error());
    }
    return loadAllowlistedJsWarningsText(text.get());
  }

  /**
   * Loads legacy warnings list from the text of the file.
   *
   * @return The lines of the file.
   */
  // TODO(nicksantos): This is a weird API.
  static Set<String> loadAllowlistedJsWarningsText(String text) {
    Set<String> result = new LinkedHashSet<>();

    result.addAll(Io.splitLines(text));

    return result;
  }

  /**
   * If subclasses want to modify the formatting, they should override #formatWarning(JSError,
   * boolean), not this method.
   */
  protected String formatWarning(JSError error) {
    return formatWarning(error, false);
  }

  /**
   * @param withMetaData If true, include metadata that's useful to humans This metadata won't be
   *     used for matching the warning.
   */
  protected String formatWarning(JSError error, boolean withMetaData) {
    StringBuilder sb = new StringBuilder();
    sb.append(normalizeSourceName(error.sourceName()).orElse("null")).append(":");
    if (withMetaData) {
      sb.append(error.getLineNumber());
    }
    List<String> lines = LINE_SPLITTER.splitToList(error.description());
    sb.append("  ").append(lines.get(0));

    // Add the rest of the message as a comment.
    if (withMetaData) {
      for (int i = 1; i < lines.size(); i++) {
        sb.append("\n# ").append(lines.get(i));
      }
      sb.append("\n");
    }

    return sb.toString();
  }

  private Optional<String> normalizeSourceName(Optional<String> sourceName) {
    if (sourceName.isPresent()) {
      // e.g.
      // "blaze-out/k8-fastbuild/genfiles/some/path/foo.js" -> "some/path/foo.js"
      return Optional.ofNullable(BLAZE_OUT_DIR_PATTERN.matcher(sourceName.get()).replaceFirst(""));
    }
    return sourceName;
  }

  public static String getFirstLine(String warning) {
    int lineLength = warning.indexOf('\n');
    if (lineLength > 0) {
      warning = warning.substring(0, lineLength);
    }
    return warning;
  }

  /** Allowlist builder */
  public class AllowlistBuilder implements ErrorHandler {
    private final Set<JSError> warnings = new LinkedHashSet<>();
    private Optional<String> productName = Optional.empty();
    private Optional<String> generatorTarget = Optional.empty();
    private Optional<String> headerNote = Optional.empty();

    /** Fill in your product name to get a fun message! */
    @CanIgnoreReturnValue
    public AllowlistBuilder setProductName(String name) {
      this.productName = Optional.of(name);
      return this;
    }

    /** Fill in instructions on how to generate this allowlist. */
    @CanIgnoreReturnValue
    public AllowlistBuilder setGeneratorTarget(String name) {
      this.generatorTarget = Optional.of(name);
      return this;
    }

    /** A note to include at the top of the allowlist file. */
    @CanIgnoreReturnValue
    public AllowlistBuilder setNote(String note) {
      this.headerNote = Optional.of(note);
      return this;
    }

    @Override
    public void report(CheckLevel level, JSError error) {
      if (error.defaultLevel().equals(CheckLevel.ERROR)) {
        // ERROR-level diagnostics are ignored by AllowlistWarningsGuard (c.f. above getLevel).
        return;
      }
      warnings.add(error);
    }

    /**
     * Writes the warnings collected in a format that the AllowlistWarningsGuard can read back
     * later.
     */
    public Outcome<Void> writeAllowlist(String out) {
      Outcome<Io.Output> stream = Io.openOutput(out, Io.UTF_8, false);
      if (stream.isErr()) {
        return Outcome.err(stream.error());
      }
      appendAllowlist(stream.get());
      return stream.get().close();
    }

    /**
     * Writes the warnings collected in a format that the AllowlistWarningsGuard can read back
     * later.
     */
    public void appendAllowlist(Io.Output out) {
      out.append("# This is a list of legacy warnings that have yet to be fixed.\n");

      if (productName.isPresent() && !productName.get().isEmpty() && !warnings.isEmpty()) {
        out.append(
            "# Please find some time and fix at least one of them "
                + "and it will be the happiest day for "
                + productName.get()
                + ".\n");
      }

      if (generatorTarget.isPresent() && !generatorTarget.get().isEmpty()) {
        out.append("# When you fix any of these warnings, run " + generatorTarget.get() + " task.\n");
      }

      if (headerNote.isPresent()) {
        out.append("#" + Joiner.on("\n# ").join(Splitter.on('\n').split(headerNote.get())) + "\n");
      }

      SimpleSetMultimap<DiagnosticType, String> warningsByType = SimpleSetMultimap.createSorted();
      for (JSError warning : warnings) {
        warningsByType.put(warning.type(), formatWarning(warning, true /* withLineNumber */));
      }

      for (DiagnosticType type : warningsByType.keySet()) {
        if (DiagnosticGroups.DEPRECATED.matchesDiagnosticType(type)) {
          // Deprecation warnings are not raisable to error, so we don't need them in allowlists.
          continue;
        }
        out.append("\n# Warning ")
            .append(type.key)
            .append(": ")
            .println(LINE_SPLITTER.split(type.formatField).iterator().next());

        for (String warning : warningsByType.get(type)) {
          out.println(warning);
        }
      }
      out.flush();
    }
  }
}
