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

import com.google.errorprone.annotations.CanIgnoreReturnValue;
import java.util.ArrayList;
import java.util.Optional;
import java.util.function.Consumer;
import org.jspecify.annotations.Nullable;

/**
 * Builds a JSON text in memory; {@link #toString} returns what was written so far.
 *
 * <p>Replaces Gson's {@code JsonWriter} with only the operations the compiler uses, and writes the
 * same text: compact unless an indent is set; in a string, {@code "}, {@code \}, the characters
 * below U+0020, U+2028 and U+2029 are escaped, and so are {@code < > & = '} when HTML-safe. Calls
 * that do not form a JSON text (a value without a name in an object, an unbalanced end) are bugs.
 */
public final class SimpleJsonWriter {
  private static final int EMPTY_ARRAY = 1;
  private static final int NONEMPTY_ARRAY = 2;
  private static final int EMPTY_OBJECT = 3;
  private static final int DANGLING_NAME = 4;
  private static final int NONEMPTY_OBJECT = 5;
  private static final int DOCUMENT = 6;

  private static final String HEX_DIGITS = "0123456789abcdef";

  /** With a sink, about this many characters are buffered before they are handed over. */
  public static final int SINK_THRESHOLD = 1 << 16;

  private final StringBuilder out = new StringBuilder();
  private final ArrayList<Integer> stack = new ArrayList<>();
  private String indent = "";
  private boolean htmlSafe = false;
  private boolean serializeNulls = true;
  private Optional<String> deferredName = Optional.empty();
  private @Nullable Consumer<String> sink = null;

  public SimpleJsonWriter() {
    stack.add(DOCUMENT);
  }

  /** Sets the indentation of each nesting level; the empty string (the default) means compact. */
  public void setIndent(String indent) {
    this.indent = indent;
  }

  /** Whether {@code < > & = '} are escaped in strings and names. Off by default. */
  public void setHtmlSafe(boolean htmlSafe) {
    this.htmlSafe = htmlSafe;
  }

  public boolean isHtmlSafe() {
    return htmlSafe;
  }

  /** Whether an object member whose value is null is written (the default) or left out. */
  public void setSerializeNulls(boolean serializeNulls) {
    this.serializeNulls = serializeNulls;
  }

  public boolean getSerializeNulls() {
    return serializeNulls;
  }

  @CanIgnoreReturnValue
  public SimpleJsonWriter beginArray() {
    return open(EMPTY_ARRAY, '[');
  }

  @CanIgnoreReturnValue
  public SimpleJsonWriter endArray() {
    return close(EMPTY_ARRAY, NONEMPTY_ARRAY, ']');
  }

  @CanIgnoreReturnValue
  public SimpleJsonWriter beginObject() {
    return open(EMPTY_OBJECT, '{');
  }

  @CanIgnoreReturnValue
  public SimpleJsonWriter endObject() {
    return close(EMPTY_OBJECT, NONEMPTY_OBJECT, '}');
  }

  /** Sets the name of the next value, which must be a member of an object. */
  @CanIgnoreReturnValue
  public SimpleJsonWriter name(String name) {
    if (deferredName.isPresent()) {
      throw new IllegalStateException();
    }
    deferredName = Optional.of(name);
    return this;
  }

  /** Writes a string, or null for a null reference. */
  @CanIgnoreReturnValue
  public SimpleJsonWriter stringValue(Optional<String> value) {
    if (value.isEmpty()) {
      return nullValue();
    }
    beforeValue();
    string(value.get());
    return this;
  }

  /** Writes {@code json}, which must be a JSON value, as it is; or null for a null reference. */
  @CanIgnoreReturnValue
  public SimpleJsonWriter rawValue(Optional<String> json) {
    if (json.isEmpty()) {
      return nullValue();
    }
    beforeValue();
    out.append(json.get());
    return this;
  }

  @CanIgnoreReturnValue
  public SimpleJsonWriter nullValue() {
    if (deferredName.isPresent() && !serializeNulls) {
      deferredName = Optional.empty(); // skip the name and the value
      return this;
    }
    beforeValue();
    out.append("null");
    return this;
  }

  @CanIgnoreReturnValue
  public SimpleJsonWriter booleanValue(boolean value) {
    beforeValue();
    out.append(value ? "true" : "false");
    return this;
  }

  @CanIgnoreReturnValue
  public SimpleJsonWriter longValue(long value) {
    beforeValue();
    out.append(value);
    return this;
  }

  /**
   * Makes the writer hand the text written so far to {@code sink} whenever about {@link
   * #SINK_THRESHOLD} characters have piled up, so that a very large text never is in memory as a
   * whole. The pieces, followed by a final {@link #takeText}, are the text in order.
   */
  public void setSink(Consumer<String> sink) {
    this.sink = sink;
  }

  /** Returns the JSON text written since the last call (or hand-over to the sink) and drops it. */
  public String takeText() {
    String text = out.toString();
    out.setLength(0);
    return text;
  }

  /** Returns the JSON text written so far that was not taken or handed to the sink. */
  @Override
  public String toString() {
    return out.toString();
  }

  private SimpleJsonWriter open(int empty, char openBracket) {
    beforeValue();
    stack.add(empty);
    out.append(openBracket);
    return this;
  }

  private SimpleJsonWriter close(int empty, int nonempty, char closeBracket) {
    int context = stack.get(stack.size() - 1);
    if (context != nonempty && context != empty) {
      throw new IllegalStateException("Nesting problem.");
    }
    if (deferredName.isPresent()) {
      throw new IllegalStateException("Dangling name: " + deferredName.get());
    }
    stack.remove(stack.size() - 1);
    if (context == nonempty) {
      newline();
    }
    out.append(closeBracket);
    return this;
  }

  /** Writes the pending name, if any, and the separator that precedes a value in this context. */
  private void beforeValue() {
    if (sink != null && out.length() >= SINK_THRESHOLD) {
      sink.accept(takeText());
    }
    int top = stack.size() - 1;
    int context = stack.get(top);
    if (deferredName.isPresent()) {
      if (context == NONEMPTY_OBJECT) {
        out.append(',');
      } else if (context != EMPTY_OBJECT) {
        throw new IllegalStateException("Nesting problem.");
      }
      newline();
      string(deferredName.get());
      deferredName = Optional.empty();
      out.append(indent.isEmpty() ? ":" : ": ");
      stack.set(top, NONEMPTY_OBJECT);
    } else if (context == EMPTY_ARRAY) {
      stack.set(top, NONEMPTY_ARRAY);
      newline();
    } else if (context == NONEMPTY_ARRAY) {
      out.append(',');
      newline();
    } else if (context != DOCUMENT) {
      throw new IllegalStateException("Nesting problem.");
    }
  }

  private void newline() {
    if (indent.isEmpty()) {
      return;
    }
    out.append('\n');
    for (int i = 1; i < stack.size(); i++) {
      out.append(indent);
    }
  }

  private void string(String value) {
    out.append('"');
    for (int i = 0; i < value.length(); i++) {
      char c = value.charAt(i);
      switch (c) {
        case '"' -> out.append("\\\"");
        case '\\' -> out.append("\\\\");
        case '\t' -> out.append("\\t");
        case '\b' -> out.append("\\b");
        case '\n' -> out.append("\\n");
        case '\r' -> out.append("\\r");
        case '\f' -> out.append("\\f");
        default -> {
          boolean html = c == '<' || c == '>' || c == '&' || c == '=' || c == '\'';
          if (c < ' ' || c == ' ' || c == ' ' || (html && htmlSafe)) {
            out.append("\\u");
            for (int shift = 12; shift >= 0; shift -= 4) {
              out.append(HEX_DIGITS.charAt((c >> shift) & 0xf));
            }
          } else {
            out.append(c);
          }
        }
      }
    }
    out.append('"');
  }
}
