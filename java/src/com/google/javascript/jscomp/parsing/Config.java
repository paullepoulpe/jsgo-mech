/*
 * Copyright 2009 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.parsing;

import static java.util.Objects.requireNonNull;

import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import com.google.errorprone.annotations.Immutable;
import com.google.javascript.jscomp.parsing.parser.FeatureSet;

/**
 * Configuration for the AST factory. Should be shared across AST creation for all files of a
 * compilation process.
 *
 * @param languageMode Language level to accept.
 * @param strictMode Whether to assume input is strict mode compliant.
 * @param jsDocParsingMode How to parse the descriptions of JsDoc comments.
 * @param runMode Whether to keep going after encountering a parse error.
 * @param annotations Recognized JSDoc annotations, mapped from their name to their internal
 *     representation.
 * @param suppressionNames Set of recognized names in a {@code @suppress} tag.
 * @param closurePrimitiveNames Set of recognized names in a {@code @closurePrimitive} tag.
 * @param parseInlineSourceMaps Whether to parse inline source maps (//# sourceMappingURL=data:...).
 */
@Immutable
public record Config(
    LanguageMode languageMode,
    StrictMode strictMode,
    JsDocParsing jsDocParsingMode,
    RunMode runMode,
    ImmutableMap<String, Annotation> annotations,
    ImmutableSet<String> suppressionNames,
    ImmutableSet<String> closurePrimitiveNames,
    boolean parseInlineSourceMaps) {
  public Config {
    requireNonNull(languageMode, "languageMode");
    requireNonNull(strictMode, "strictMode");
    requireNonNull(jsDocParsingMode, "jsDocParsingMode");
    requireNonNull(runMode, "runMode");
    requireNonNull(annotations, "annotations");
    requireNonNull(suppressionNames, "suppressionNames");
    requireNonNull(closurePrimitiveNames, "closurePrimitiveNames");
  }

  @Override
  public String toString() {
    return "Config{"
        + "languageMode=" + languageMode + ", "
        + "strictMode=" + strictMode + ", "
        + "jsDocParsingMode=" + jsDocParsingMode + ", "
        + "runMode=" + runMode + ", "
        + "annotations=" + annotations + ", "
        + "suppressionNames=" + suppressionNames + ", "
        + "closurePrimitiveNames=" + closurePrimitiveNames + ", "
        + "parseInlineSourceMaps=" + parseInlineSourceMaps
        + "}";
  }

  /**
   * Level of language strictness required for the input source code.
   */
  public enum StrictMode {
    STRICT, SLOPPY;

    public boolean isStrict() {
      return this == STRICT;
    }
  }

  /** JavaScript mode */
  public enum LanguageMode {

    // Note that minimumRequiredFor() relies on these being defined in order from fewest features to
    // most features, and _STRICT versions should be supplied after unspecified strictness.
    ECMASCRIPT3(FeatureSet.ES3),
    ECMASCRIPT5(FeatureSet.ES5),
    ECMASCRIPT_2015(FeatureSet.ES2015_MODULES),
    ECMASCRIPT_2016(FeatureSet.ES2016_MODULES),
    ECMASCRIPT_2017(FeatureSet.ES2017_MODULES),
    ECMASCRIPT_2018(FeatureSet.ES2018_MODULES),
    ECMASCRIPT_2019(FeatureSet.ES2019_MODULES),
    ECMASCRIPT_2020(FeatureSet.ES2020_MODULES),
    ECMASCRIPT_2021(FeatureSet.ES2021_MODULES),
    ECMASCRIPT_2022(FeatureSet.ES2022_MODULES),
    // NOTE: When adding a new language level here, also update latestEcmaScript() below to return
    // it.
    ES_NEXT(FeatureSet.ES_NEXT),
    UNSTABLE(FeatureSet.ES_UNSTABLE),
    UNSUPPORTED(FeatureSet.ES_UNSUPPORTED);

    public final FeatureSet featureSet;

    LanguageMode(FeatureSet featureSet) {
      this.featureSet = featureSet;
    }

    /**
     * Returns the lowest {@link LanguageMode} that supports the specified feature.
     */
    public static LanguageMode minimumRequiredFor(FeatureSet.Feature feature) {
      // relies on the LanguageMode enums being in the right order
      for (LanguageMode mode : LanguageMode.values()) {
        if (mode.featureSet.has(feature)) {
          return mode;
        }
      }
      throw new IllegalStateException("No input language mode supports feature: " + feature);
    }

    /** Returns the lowest {@link LanguageMode} that supports the specified feature set. */
    public static LanguageMode minimumRequiredForSet(FeatureSet featureSet) {
      for (LanguageMode mode : LanguageMode.values()) {
        if (mode.featureSet.containsFeatureSet(featureSet)) {
          return mode;
        }
      }
      throw new IllegalStateException("No input language mode supports feature set: " + featureSet);
    }

    public static LanguageMode latestEcmaScript() {
      return ECMASCRIPT_2021;
    }
  }

  /**
   * Whether to parse the descriptions of JsDoc comments.
   */
  public enum JsDocParsing {
    TYPES_ONLY,
    INCLUDE_DESCRIPTIONS_NO_WHITESPACE,
    INCLUDE_DESCRIPTIONS_WITH_WHITESPACE,
    INCLUDE_ALL_COMMENTS,
    LICENSE_COMMENTS_ONLY;

    boolean shouldParseDescriptions() {
      return this != TYPES_ONLY;
    }

    boolean shouldPreserveWhitespace() {
      return this == INCLUDE_DESCRIPTIONS_WITH_WHITESPACE
          || this == INCLUDE_ALL_COMMENTS
          || this == LICENSE_COMMENTS_ONLY;
    }
  }

  /**
   * Whether to keep going after encountering a parse error.
   */
  public enum RunMode {
    STOP_AFTER_ERROR,
    KEEP_GOING,
  }

  final ImmutableSet<String> annotationNames() {
    return annotations().keySet();
  }

  public Builder toBuilder() {
    return new Builder(this);
  }

  public static Builder builder() {
    return new Builder()
        .setLanguageMode(LanguageMode.UNSUPPORTED)
        .setStrictMode(StrictMode.STRICT)
        .setJsDocParsingMode(JsDocParsing.TYPES_ONLY)
        .setRunMode(RunMode.STOP_AFTER_ERROR)
        .setExtraAnnotationNames(ImmutableSet.<String>of())
        .setSuppressionNames(ImmutableSet.<String>of())
        .setClosurePrimitiveNames(ImmutableSet.of())
        .setParseInlineSourceMaps(false);
  }

  /** Builder for a Config. */
  public static final class Builder {
    private Config.LanguageMode languageMode;
    private Config.StrictMode strictMode;
    private Config.JsDocParsing jsDocParsingMode;
    private Config.RunMode runMode;
    private ImmutableMap<String, Annotation> annotations;
    private ImmutableSet<String> suppressionNames;
    private ImmutableSet<String> closurePrimitiveNames;
    private boolean parseInlineSourceMaps;
    private byte set0;

    Builder() {
    }

    Builder(Config source) {
      this.languageMode = source.languageMode();
      this.strictMode = source.strictMode();
      this.jsDocParsingMode = source.jsDocParsingMode();
      this.runMode = source.runMode();
      this.annotations = source.annotations();
      this.suppressionNames = source.suppressionNames();
      this.closurePrimitiveNames = source.closurePrimitiveNames();
      this.parseInlineSourceMaps = source.parseInlineSourceMaps();
      set0 = (byte) 1;
    }

    public Builder setLanguageMode(LanguageMode mode) {
      if (mode == null) {
        throw new NullPointerException("Null languageMode");
      }
      this.languageMode = mode;
      return this;
    }

    public Builder setStrictMode(StrictMode mode) {
      if (mode == null) {
        throw new NullPointerException("Null strictMode");
      }
      this.strictMode = mode;
      return this;
    }

    public Builder setJsDocParsingMode(JsDocParsing mode) {
      if (mode == null) {
        throw new NullPointerException("Null jsDocParsingMode");
      }
      this.jsDocParsingMode = mode;
      return this;
    }

    public Builder setRunMode(RunMode mode) {
      if (mode == null) {
        throw new NullPointerException("Null runMode");
      }
      this.runMode = mode;
      return this;
    }

    public Builder setParseInlineSourceMaps(boolean parseInlineSourceMaps) {
      this.parseInlineSourceMaps = parseInlineSourceMaps;
      set0 |= (byte) 1;
      return this;
    }

    public Builder setSuppressionNames(Iterable<String> names) {
      this.suppressionNames = ImmutableSet.copyOf(names);
      return this;
    }

    Builder setClosurePrimitiveNames(Iterable<String> names) {
      this.closurePrimitiveNames = ImmutableSet.copyOf(names);
      return this;
    }

    final Builder setExtraAnnotationNames(Iterable<String> names) {
      return setAnnotations(buildAnnotations(names));
    }

    public Config build() {
      if (set0 != 1
          || this.languageMode == null
          || this.strictMode == null
          || this.jsDocParsingMode == null
          || this.runMode == null
          || this.annotations == null
          || this.suppressionNames == null
          || this.closurePrimitiveNames == null) {
        StringBuilder missing = new StringBuilder();
        if (this.languageMode == null) {
          missing.append(" languageMode");
        }
        if (this.strictMode == null) {
          missing.append(" strictMode");
        }
        if (this.jsDocParsingMode == null) {
          missing.append(" jsDocParsingMode");
        }
        if (this.runMode == null) {
          missing.append(" runMode");
        }
        if (this.annotations == null) {
          missing.append(" annotations");
        }
        if (this.suppressionNames == null) {
          missing.append(" suppressionNames");
        }
        if (this.closurePrimitiveNames == null) {
          missing.append(" closurePrimitiveNames");
        }
        if ((set0 & 1) == 0) {
          missing.append(" parseInlineSourceMaps");
        }
        throw new IllegalStateException("Missing required properties:" + missing);
      }
      return new Config(
          this.languageMode,
          this.strictMode,
          this.jsDocParsingMode,
          this.runMode,
          this.annotations,
          this.suppressionNames,
          this.closurePrimitiveNames,
          this.parseInlineSourceMaps);
    }

    // The following is intended to be used internally only (but isn't private due to AutoValue).
    public Builder setAnnotations(ImmutableMap<String, Annotation> names) {
      if (names == null) {
        throw new NullPointerException("Null annotations");
      }
      this.annotations = names;
      return this;
    }
  }

  /** Create the annotation names from the user-specified annotation allowlist. */
  private static ImmutableMap<String, Annotation> buildAnnotations(Iterable<String> allowlist) {
    ImmutableMap.Builder<String, Annotation> annotationsBuilder = ImmutableMap.builder();
    annotationsBuilder.putAll(Annotation.recognizedAnnotations);
    for (String unrecognizedAnnotation : allowlist) {
      if (!unrecognizedAnnotation.isEmpty()
          && !Annotation.recognizedAnnotations.containsKey(unrecognizedAnnotation)) {
        annotationsBuilder.put(unrecognizedAnnotation, Annotation.NOT_IMPLEMENTED);
      }
    }
    return annotationsBuilder.buildOrThrow();
  }
}
