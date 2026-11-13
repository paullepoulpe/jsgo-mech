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

import com.google.gson.stream.JsonReader;
import com.google.gson.stream.JsonToken;
import java.io.IOException;
import java.io.StringReader;
import org.jspecify.annotations.Nullable;

/**
 * Parses standard JSON. This is the only class that may use the JSON library.
 *
 * <p>The accepted language is exactly RFC 8259, with these additions and limits: one byte order
 * mark before the text is skipped; nesting deeper than {@link #MAX_DEPTH}, a number of {@link
 * #MAX_NUMBER_LENGTH} or more characters and a number with more than {@link #MAX_INTEGER_DIGITS}
 * digits before the fraction are malformed; a surrogate that is not part of a pair (written as
 * such or as a {@code \}{@code u} escape) is read as U+FFFD. No comments, no single quotes, no
 * unquoted names, no trailing commas, no NaN or Infinity, nothing but whitespace after the value.
 */
public final class Json {
  /** Arrays and objects may be nested this deep. */
  public static final int MAX_DEPTH = 1000;

  /** A number must be shorter than this many characters. */
  public static final int MAX_NUMBER_LENGTH = 1024;

  /** A number may have this many digits before its fraction (the library misreads some longer). */
  public static final int MAX_INTEGER_DIGITS = 20;

  /**
   * Parses a text that is one JSON value. A number keeps its source text. A member whose name is
   * repeated in an object replaces the earlier one, at the earlier one's position.
   *
   * @return the value, or an error with a {@link SimpleJsonException} that says only "Malformed
   *     JSON": the text is not standard JSON
   */
  public static Outcome<SimpleJsonValue> parse(String text) {
    String json = text.startsWith("﻿") ? text.substring(1) : text;
    // The library would skip a second byte order mark.
    if (json.startsWith("\ufeff") || hasNonStandardToken(json)) {
      return malformed();
    }
    JsonReader reader = new JsonReader(new StringReader(json));
    try {
      SimpleJsonValue value = read(reader, 0);
      if (value == null || reader.peek() != JsonToken.END_DOCUMENT) {
        return malformed();
      }
      return Outcome.ok(value);
    } catch (IOException | NumberFormatException e) {
      // The library reports a malformed text with these.
      return malformed();
    }
  }

  private static Outcome<SimpleJsonValue> malformed() {
    return Outcome.err(new SimpleJsonException("Malformed JSON"));
  }

  /**
   * Whether the text has something that the library's strict mode accepts although it is not
   * standard JSON: a keyword not in lower case, a control character in a string, or an escape
   * other than {@code \" \\ \/ \b \f \n \r \t} and {@code \}{@code u}; or a number with too
   * many digits before the fraction.
   */
  private static boolean hasNonStandardToken(String json) {
    boolean inString = false;
    boolean inNumber = false;
    for (int i = 0; i < json.length(); i++) {
      char c = json.charAt(i);
      boolean wasInNumber = inNumber;
      inNumber = !inString && "0123456789.eE+-".indexOf(c) >= 0;
      if (inNumber && !wasInNumber) {
        int digits = c == '-' ? i + 1 : i;
        int end = digits;
        while (end < json.length() && json.charAt(end) >= '0' && json.charAt(end) <= '9') {
          end++;
        }
        if (end - digits > MAX_INTEGER_DIGITS) {
          return true;
        }
      }
      if (inString && c == '\\') {
        i++;
        if (i == json.length() || "\"\\/bfnrtu".indexOf(json.charAt(i)) < 0) {
          return true;
        }
      } else if (inString && c < ' ') {
        return true;
      } else if (c == '"') {
        inString = !inString;
      } else if (!inString && (c == 't' || c == 'f' || c == 'n')) {
        String keyword = c == 't' ? "true" : c == 'f' ? "false" : "null";
        if (!json.startsWith(keyword, i)) {
          return true;
        }
        i += keyword.length() - 1;
      } else if (!inString && c >= 'A' && c <= 'Z' && c != 'E') {
        return true;
      }
    }
    return false;
  }

  /** Reads the next value, which is inside {@code depth} arrays or objects; null if too deep. */
  private static @Nullable SimpleJsonValue read(JsonReader reader, int depth) throws IOException {
    JsonToken token = reader.peek();
    if (depth == MAX_DEPTH && (token == JsonToken.BEGIN_ARRAY || token == JsonToken.BEGIN_OBJECT)) {
      return null;
    }
    if (token == JsonToken.BEGIN_ARRAY) {
      SimpleJsonValue array = SimpleJsonValue.newArray();
      reader.beginArray();
      while (reader.hasNext()) {
        SimpleJsonValue element = read(reader, depth + 1);
        if (element == null) {
          return null;
        }
        array.elements().add(element);
      }
      reader.endArray();
      return array;
    } else if (token == JsonToken.BEGIN_OBJECT) {
      SimpleJsonValue object = SimpleJsonValue.newObject();
      reader.beginObject();
      while (reader.hasNext()) {
        String name = wellFormed(reader.nextName());
        SimpleJsonValue member = read(reader, depth + 1);
        if (member == null) {
          return null;
        }
        object.members().put(name, member);
      }
      reader.endObject();
      return object;
    } else if (token == JsonToken.STRING) {
      return SimpleJsonValue.ofString(wellFormed(reader.nextString()));
    } else if (token == JsonToken.NUMBER) {
      return SimpleJsonValue.ofNumber(reader.nextString());
    } else if (token == JsonToken.BOOLEAN) {
      return SimpleJsonValue.ofBoolean(reader.nextBoolean());
    }
    reader.nextNull();
    return SimpleJsonValue.ofNull();
  }

  /** Returns s with every surrogate that is not part of a pair replaced by U+FFFD. */
  private static String wellFormed(String s) {
    StringBuilder fixed = null;
    for (int i = 0; i < s.length(); i++) {
      char c = s.charAt(i);
      if (Character.isHighSurrogate(c) && i + 1 < s.length()
          && Character.isLowSurrogate(s.charAt(i + 1))) {
        i++;
      } else if (Character.isSurrogate(c)) {
        fixed = fixed == null ? new StringBuilder(s) : fixed;
        fixed.setCharAt(i, '�');
      }
    }
    return fixed == null ? s : fixed.toString();
  }

  private Json() {}
}
