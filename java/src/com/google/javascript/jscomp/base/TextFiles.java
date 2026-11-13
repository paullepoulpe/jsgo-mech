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
package com.google.javascript.jscomp.base;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import org.jspecify.annotations.Nullable;

/** Whole text files: reading and writing them in a charset, and their lines. */
public final class TextFiles {
  private TextFiles() {}

  /** The error of {@link #readStrict} for a file whose bytes are not text in the charset. */
  public static final class DecodingException extends IOException {
    private static final long serialVersionUID = 1L;

    DecodingException(String message) {
      super(message);
    }
  }

  /** The file's text; bytes that are not valid in the charset become U+FFFD. */
  public static Outcome<String> read(String path, String charset) {
    Outcome<byte[]> bytes = Io.readBytes(path);
    if (bytes.isErr()) {
      return Outcome.err(bytes.error());
    }
    return Outcome.ok(Charsets.decode(bytes.get(), charset));
  }

  /**
   * The file's text. Bytes that are not valid in the charset are an error, a {@link
   * DecodingException} with the message "Failed to read: (path), is this input UTF-8 encoded?".
   */
  public static Outcome<String> readStrict(String path, String charset) {
    Outcome<byte[]> bytes = Io.readBytes(path);
    if (bytes.isErr()) {
      return Outcome.err(bytes.error());
    }
    Optional<String> text = Charsets.decodeStrict(bytes.get(), charset);
    if (text.isEmpty()) {
      return Outcome.err(
          new DecodingException("Failed to read: " + path + ", is this input UTF-8 encoded?"));
    }
    return Outcome.ok(text.get());
  }

  public static boolean isDecodingError(@Nullable Throwable error) {
    return error instanceof DecodingException;
  }

  /** Creates or truncates the file; a character the charset does not have becomes '?'. */
  public static Outcome<Void> write(String path, String charset, String text) {
    return Io.writeBytes(path, Charsets.encode(text, charset));
  }

  /**
   * The lines of the text: each ends at "\n", "\r" or "\r\n" (not part of the line) or at the end
   * of the text; there is no empty last line after a final line end.
   */
  public static List<String> splitLines(String text) {
    List<String> lines = new ArrayList<>();
    int start = 0;
    for (int i = 0; i < text.length(); i++) {
      char c = text.charAt(i);
      if (c == '\n' || c == '\r') {
        lines.add(text.substring(start, i));
        if (c == '\r' && i + 1 < text.length() && text.charAt(i + 1) == '\n') {
          i++;
        }
        start = i + 1;
      }
    }
    if (start < text.length()) {
      lines.add(text.substring(start));
    }
    return lines;
  }
}
