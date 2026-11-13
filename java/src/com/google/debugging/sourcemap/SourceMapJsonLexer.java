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
package com.google.debugging.sourcemap;

import com.google.common.primitives.Ints;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
import com.google.javascript.jscomp.base.Outcome;
import org.jspecify.annotations.Nullable;

/**
 * A JSON lexer optimized for large string extraction. By using String.substring() on the source
 * contents, it avoids the massive GC churn caused by JsonReader's internal StringBuilder resizing
 * when parsing large Base64 VLQ mapping strings.
 */
final class SourceMapJsonLexer {
  final String json;
  int pos = 0;
  final int length;

  SourceMapJsonLexer(String json) {
    this.json = json;
    this.length = json.length();
  }

  void skipWhitespace() {
    while (pos < length) {
      char c = json.charAt(pos);
      if (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
        pos++;
      } else {
        break;
      }
    }
  }

  boolean hasNext() {
    skipWhitespace();
    if (pos >= length) {
      return false;
    }
    char c = json.charAt(pos);
    return c != ']' && c != '}';
  }

  Outcome<Void> beginObject() {
    skipWhitespace();
    if (pos < length && json.charAt(pos) == '{') {
      pos++;
    } else {
      return Outcome.err(new SourceMapParseException("Expected '{'"));
    }
    return Outcome.ok();
  }

  Outcome<Void> endObject() {
    skipWhitespace();
    if (pos < length && json.charAt(pos) == '}') {
      pos++;
    } else {
      return Outcome.err(new SourceMapParseException("Expected '}'"));
    }
    return Outcome.ok();
  }

  Outcome<Void> beginArray() {
    skipWhitespace();
    if (pos < length && json.charAt(pos) == '[') {
      pos++;
    } else {
      return Outcome.err(new SourceMapParseException("Expected '['"));
    }
    return Outcome.ok();
  }

  Outcome<Void> endArray() {
    skipWhitespace();
    if (pos < length && json.charAt(pos) == ']') {
      pos++;
    } else {
      return Outcome.err(new SourceMapParseException("Expected ']'"));
    }
    return Outcome.ok();
  }

  Outcome<String> nextName() {
    Outcome<String> nameOutcome = nextString();
    if (nameOutcome.isErr()) {
      return Outcome.err(nameOutcome.error());
    }
    String name = nameOutcome.get();
    skipWhitespace();
    if (pos < length && json.charAt(pos) == ':') {
      pos++;
    } else {
      return Outcome.err(new SourceMapParseException("Expected ':'"));
    }
    return Outcome.ok(name);
  }

  Outcome<@Nullable String> nextStringOrNull() {
    skipWhitespace();
    if (pos < length && json.startsWith("null", pos)) {
      pos += 4;
      return Outcome.ok(null);
    }
    return nextString();
  }

  Outcome<String> nextString() {
    skipWhitespace();
    if (pos >= length || json.charAt(pos) != '"') {
      return Outcome.err(new SourceMapParseException("Expected string"));
    }
    pos++; // skip quote
    int start = pos;
    boolean hasEscape = false;
    while (pos < length) {
      char c = json.charAt(pos);
      if (c == '"') {
        break;
      }
      if (c == '\\') {
        hasEscape = true;
        pos++;
      }
      pos++;
    }
    if (pos >= length) {
      return Outcome.err(new SourceMapParseException("Unterminated string"));
    }
    String result;
    if (!hasEscape) {
      result = json.substring(start, pos);
    } else {
      Outcome<String> unescaped = unescape(json, start, pos);
      if (unescaped.isErr()) {
        return Outcome.err(unescaped.error());
      }
      result = unescaped.get();
    }
    pos++; // skip quote
    return Outcome.ok(result);
  }

  Outcome<Integer> nextInt() {
    skipWhitespace();
    int start = pos;
    if (pos < length && json.charAt(pos) == '-') {
      pos++;
    }
    while (pos < length && json.charAt(pos) >= '0' && json.charAt(pos) <= '9') {
      pos++;
    }
    if (start == pos) {
      return Outcome.err(new SourceMapParseException("Expected integer"));
    }
    // The token is an optional '-' and ASCII digits, on which Ints.tryParse and Integer.parseInt
    // agree.
    Integer value = Ints.tryParse(json.substring(start, pos));
    if (value == null) {
      return Outcome.err(
          new SourceMapParseException("Invalid integer: " + json.substring(start, pos)));
    }
    return Outcome.ok(value);
  }

  Outcome<String> nextRawValue() {
    skipWhitespace();
    int start = pos;
    Outcome<Void> skipped = skipValue();
    if (skipped.isErr()) {
      return Outcome.err(skipped.error());
    }
    return Outcome.ok(json.substring(start, pos));
  }

  Outcome<Void> skipValue() {
    skipWhitespace();
    if (pos >= length) {
      return Outcome.err(new SourceMapParseException("Unexpected end of input"));
    }
    char c = json.charAt(pos);
    if (c == '{') {
      pos++;
      while (hasNext()) {
        Outcome<String> name = nextName();
        if (name.isErr()) {
          return Outcome.err(name.error());
        }
        Outcome<Void> skipped = skipValue();
        if (skipped.isErr()) {
          return Outcome.err(skipped.error());
        }
        Outcome<Void> comma = checkComma();
        if (comma.isErr()) {
          return Outcome.err(comma.error());
        }
      }
      Outcome<Void> ended = endObject();
      if (ended.isErr()) {
        return Outcome.err(ended.error());
      }
    } else if (c == '[') {
      pos++;
      while (hasNext()) {
        Outcome<Void> skipped = skipValue();
        if (skipped.isErr()) {
          return Outcome.err(skipped.error());
        }
        Outcome<Void> comma = checkComma();
        if (comma.isErr()) {
          return Outcome.err(comma.error());
        }
      }
      Outcome<Void> ended = endArray();
      if (ended.isErr()) {
        return Outcome.err(ended.error());
      }
    } else if (c == '"') {
      Outcome<String> string = nextString();
      if (string.isErr()) {
        return Outcome.err(string.error());
      }
    } else if (c == 't' && json.startsWith("true", pos)) {
      pos += 4;
    } else if (c == 'f' && json.startsWith("false", pos)) {
      pos += 5;
    } else if (c == 'n' && json.startsWith("null", pos)) {
      pos += 4;
    } else if (c == '-' || (c >= '0' && c <= '9')) {
      if (c == '-') {
        pos++;
      }
      while (pos < length
          && ((json.charAt(pos) >= '0' && json.charAt(pos) <= '9')
              || json.charAt(pos) == '.'
              || json.charAt(pos) == 'e'
              || json.charAt(pos) == 'E'
              || json.charAt(pos) == '+'
              || json.charAt(pos) == '-')) {
        pos++;
      }
    } else {
      return Outcome.err(new SourceMapParseException("Unexpected character: " + c));
    }
    return Outcome.ok();
  }

  Outcome<Void> checkComma() {
    boolean hasComma = tryConsumeComma();
    if (hasComma && !hasNext()) {
      return Outcome.err(new SourceMapParseException("Unexpected trailing comma"));
    }
    if (!hasComma && hasNext()) {
      return Outcome.err(new SourceMapParseException("Expected ','"));
    }
    return Outcome.ok();
  }

  @CanIgnoreReturnValue
  boolean tryConsumeComma() {
    skipWhitespace();
    if (pos < length && json.charAt(pos) == ',') {
      pos++;
      return true;
    }
    return false;
  }

  /**
   * Whether {@code Integer.parseInt(s, start, start + 4, 16)} succeeds: the four chars are hex
   * digits (as {@link Character#digit} defines them), the first of which may be a sign instead.
   */
  private static boolean isParsableHex4(String s, int start) {
    for (int i = start; i < start + 4; i++) {
      char c = s.charAt(i);
      if (Character.digit(c, 16) < 0 && !(i == start && (c == '+' || c == '-'))) {
        return false;
      }
    }
    return true;
  }

  private static Outcome<String> unescape(String s, int start, int end) {
    StringBuilder sb = new StringBuilder(end - start);
    for (int i = start; i < end; i++) {
      char c = s.charAt(i);
      if (c == '\\' && i + 1 < end) {
        i++;
        char next = s.charAt(i);
        switch (next) {
          case '"' -> sb.append('"');
          case '\\' -> sb.append('\\');
          case '/' -> sb.append('/');
          case 'b' -> sb.append('\b');
          case 'f' -> sb.append('\f');
          case 'n' -> sb.append('\n');
          case 'r' -> sb.append('\r');
          case 't' -> sb.append('\t');
          case 'u' -> {
            if (i + 4 >= end) {
              return Outcome.err(
                  new SourceMapParseException("Invalid unicode escape sequence"));
            }
            if (!isParsableHex4(s, i + 1)) {
              return Outcome.err(
                  new SourceMapParseException(
                      "Invalid unicode escape: " + s.substring(i + 1, i + 5)));
            }
            sb.append((char) Integer.parseInt(s, i + 1, i + 5, 16));
            i += 4;
          }
          default -> sb.append(next);
        }
      } else {
        sb.append(c);
      }
    }
    return Outcome.ok(sb.toString());
  }
}
