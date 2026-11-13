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

package com.google.debugging.sourcemap;

import static com.google.common.base.Preconditions.checkArgument;

import com.google.javascript.jscomp.base.Outcome;
import org.jspecify.annotations.Nullable;

/** Detect and parse the provided source map. */
public final class SourceMapConsumerFactory {

  /** not constructible */
  private SourceMapConsumerFactory() {}

  /**
   * @param contents The string representing the source map file contents.
   * @return The parsed source map.
   */
  public static Outcome<SourceMapping> parse(String contents) {
     return parse(contents, null);
  }

  /**
   * @param contents The string representing the source map file contents.
   * @param supplier A supplier for any referenced maps.
   * @return The parsed source map.
   */
  public static Outcome<SourceMapping> parse(
      String contents, @Nullable SourceMapSupplier supplier) {
    // Version 1, starts with a magic string
    if (contents.startsWith("/** Begin line maps. **/")) {
      return Outcome.err(
          new SourceMapParseException(
              "This appears to be a V1 SourceMap, which is not supported."));
    } else if (contents.startsWith("{")){
      Outcome<SourceMapObject> sourceMapObjectOutcome = SourceMapObjectParser.parse(contents);
      if (sourceMapObjectOutcome.isErr()) {
        return Outcome.err(sourceMapObjectOutcome.error());
      }
      SourceMapObject sourceMapObject = sourceMapObjectOutcome.get();

      // Check basic assertions about the format.
      switch (sourceMapObject.getVersion()) {
        case 3 -> {
          SourceMapConsumerV3 consumer = new SourceMapConsumerV3();
          Outcome<Void> parsed = consumer.parse(sourceMapObject, supplier);
          if (parsed.isErr()) {
            return Outcome.err(parsed.error());
          }
          return Outcome.ok(consumer);
        }
        default -> {
          return Outcome.err(
              new SourceMapParseException(
                  "Unknown source map version:" + sourceMapObject.getVersion()));
        }
      }
    }

    return Outcome.err(new SourceMapParseException("unable to detect source map format"));
  }

  /**
   * Using the fast parser to parse the source map. This only supports version 3 source maps.
   *
   * @param contents The string representing the source map file contents.
   * @return The parsed source map.
   */
  public static Outcome<SourceMapConsumerV3> parseFast(String contents) {
    if (contents.startsWith("{")) {
      Outcome<SourceMapObject> sourceMapObjectOutcome = SourceMapObjectParser.parseFast(contents);
      if (sourceMapObjectOutcome.isErr()) {
        return Outcome.err(sourceMapObjectOutcome.error());
      }
      SourceMapObject sourceMapObject = sourceMapObjectOutcome.get();
      checkArgument(
          sourceMapObject.getVersion() == 3,
          "Unknown source map version:%s",
          sourceMapObject.getVersion());
      SourceMapConsumerV3 consumer = new SourceMapConsumerV3();
      Outcome<Void> parsed = consumer.parse(sourceMapObject, null);
      if (parsed.isErr()) {
        return Outcome.err(parsed.error());
      }
      return Outcome.ok(consumer);
    }
    return Outcome.err(new SourceMapParseException("unable to detect source map format"));
  }
}
