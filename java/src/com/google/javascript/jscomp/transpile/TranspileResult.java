/*
 * Copyright 2016 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.transpile;


import com.google.common.escape.Escaper;
import com.google.common.net.PercentEscaper;
import com.google.javascript.jscomp.base.Io;
import java.util.Objects;

/**
 * The result of transpiling a single file.  Includes transpiled
 * and original source, and an optional source map.
 */
public final class TranspileResult {

  private final String pathField;
  private final String originalField;
  private final String transpiledField;
  private final String sourceMapField;

  public TranspileResult(String path, String original, String transpiled, String sourceMap) {
    this.pathField = path;
    this.originalField = original;
    this.transpiledField = transpiled;
    this.sourceMapField = sourceMap;
  }

  public String path() {
    return pathField;
  }

  // TODO(sdh): how to specify naming scheme for sourcemap and original?
  //  - if we're not embedding then we need to know how to find them.
  //  - probabaly we'll keep 'path' pointing to the original, since that's
  //    what's more important and harder to find - we can add some suffix
  //    on for the transpiled version as necessary.
  public String original() {
    return originalField;
  }

  public String sourceMap() {
    return sourceMapField;
  }

  // TODO(sdh): might be a bit nicer for this to be a suffix rather than a whole path?
  public TranspileResult embedSourcemapUrl(String url) {
    // If there's no sourcemap, don't reference it.
    if (sourceMapField.isEmpty()) {
      return this;
    }
    String embedded = transpiledField + "\n//# sourceMappingURL=" + url + "\n";
    return new TranspileResult(pathField, originalField, embedded, sourceMapField);
  }

  // sourcemaps must escape :, and cannot escape space as +, so we need a custom escaper.
  private static final Escaper ESCAPER = new PercentEscaper("-_.*", /* plusForSpace= */ false);

  public TranspileResult embedSourcemap() {
    if (sourceMapField.isEmpty()) {
      return this;
    }
    String embedded =
        transpiledField + "\n//# sourceMappingURL=data:," + ESCAPER.escape(sourceMapField) + "\n";
    return new TranspileResult(pathField, originalField, embedded, "");
  }

  public TranspileResult embedSourcemapBase64() {
    if (sourceMapField.isEmpty()) {
      return this;
    }
    String embedded =
        transpiledField
            + "\n//# sourceMappingURL=data:application/json;base64,"
            + Io.base64Encode(Io.utf8Bytes(sourceMapField))
            + "\n";
    return new TranspileResult(pathField, originalField, embedded, "");
  }

  public String transpiled() {
    return transpiledField;
  }

  public boolean wasTranspiled() {
    return !transpiledField.equals(originalField);
  }

  @Override
  public boolean equals(Object other) {
    return other instanceof TranspileResult transpileResult
        && transpileResult.pathField.equals(pathField)
        && transpileResult.originalField.equals(originalField)
        && transpileResult.transpiledField.equals(transpiledField)
        && transpileResult.sourceMapField.equals(sourceMapField);
  }

  @Override
  public int hashCode() {
    return Objects.hash(pathField, originalField, transpiledField, sourceMapField);
  }

  @Override
  public String toString() {
    return String.format(
        "TranspileResut{path=%s, original=%s, transpiled=%s, sourceMapURL=%s}",
        pathField, originalField, transpiledField, sourceMapField);
  }
}
