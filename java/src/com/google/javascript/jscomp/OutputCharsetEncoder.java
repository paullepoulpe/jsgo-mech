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
import java.util.Optional;
import org.jspecify.annotations.Nullable;

/**
 * Output charset encoder for {@code CodeGenerator} that delegates to a charset encoder.
 */
final class OutputCharsetEncoder {

  private final Io.@Nullable CharEncoder encoder;

  OutputCharsetEncoder(Optional<String> outputCharset) {
    if (outputCharset.isEmpty() || Io.charsetName(outputCharset.get()).equals(Io.US_ASCII)) {
      // If we want our default (pretending to be UTF-8, but escaping anything
      // outside of straight ASCII), then don't use the encoder, but
      // just special-case the code.  This keeps the normal path through
      // the code identical to how it's been for years.
      this.encoder = null;
    } else {
      this.encoder = Io.newEncoder(outputCharset.get());
    }
  }

  boolean canEncode(char c) {
    return encoder != null && encoder.canEncode(c);
  }
}
