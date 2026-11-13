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

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.jspecify.annotations.Nullable;

/**
 * A JSON value as a tree: an object, an array, a string, a number, a boolean or null.
 *
 * <p>Replaces Gson's {@code JsonElement} family with only the operations the compiler uses. The
 * members of an object keep the order in which their names were first added. A number is kept as
 * the text it was read from, which is also how it is written.
 */
public final class SimpleJsonValue {
  /** The kinds of JSON value. */
  public enum Kind {
    OBJECT,
    ARRAY,
    STRING,
    NUMBER,
    BOOLEAN,
    NULL
  }

  private final Kind tag;
  /** The string, the source text of the number, "true", "false" or "null"; else "". */
  private final String text;
  private final ArrayList<SimpleJsonValue> items = new ArrayList<>();
  private final LinkedHashMap<String, SimpleJsonValue> named = new LinkedHashMap<>();

  private SimpleJsonValue(Kind tag, String text) {
    this.tag = tag;
    this.text = text;
  }

  public static SimpleJsonValue newObject() {
    return new SimpleJsonValue(Kind.OBJECT, "");
  }

  public static SimpleJsonValue newArray() {
    return new SimpleJsonValue(Kind.ARRAY, "");
  }

  public static SimpleJsonValue ofString(String value) {
    return new SimpleJsonValue(Kind.STRING, value);
  }

  /** A number given as its JSON text, e.g. "1", "-0.5" or "1e9". */
  public static SimpleJsonValue ofNumber(String jsonText) {
    return new SimpleJsonValue(Kind.NUMBER, jsonText);
  }

  public static SimpleJsonValue ofBoolean(boolean value) {
    return new SimpleJsonValue(Kind.BOOLEAN, value ? "true" : "false");
  }

  public static SimpleJsonValue ofNull() {
    return new SimpleJsonValue(Kind.NULL, "null");
  }

  public Kind kind() {
    return tag;
  }

  /** The live, insertion-ordered members of an object; empty and not to be changed otherwise. */
  public Map<String, SimpleJsonValue> members() {
    return named;
  }

  /** The live elements of an array; empty and not to be changed otherwise. */
  public List<SimpleJsonValue> elements() {
    return items;
  }

  /** The member of an object with the given name; null if there is none or this is no object. */
  public @Nullable SimpleJsonValue get(String name) {
    return named.get(name);
  }

  /**
   * The value as a string: a string itself, the source text of a number, "true" or "false".
   * Anything else is an error.
   */
  public Outcome<String> asString() {
    if (tag == Kind.STRING || tag == Kind.NUMBER || tag == Kind.BOOLEAN) {
      return Outcome.ok(text);
    }
    return Outcome.err(new SimpleJsonException("Expected a JSON string but found " + tag));
  }

  /**
   * The value as an int: a number without its fraction and wrapped to 32 bits, or a string that
   * {@link Integer#parseInt} accepts. Anything else is an error.
   */
  public Outcome<Integer> asInt() {
    Integer parsed =
        tag == Kind.STRING || tag == Kind.NUMBER ? JSCompStrings.parseIntOrNull(text) : null;
    if (parsed == null && tag == Kind.NUMBER) {
      parsed = wrapToInt(text);
    }
    if (parsed == null) {
      return Outcome.err(new SimpleJsonException("Expected a JSON integer but found " + tag));
    }
    return Outcome.ok(parsed);
  }

  /** The integer part of a JSON number, wrapped to 32 bits like {@code (int)} of a long. */
  private static int wrapToInt(String number) {
    boolean negative = number.charAt(0) == '-';
    int end = number.length();
    long exponent = 0;
    int e = Math.max(number.indexOf('e'), number.indexOf('E'));
    if (e >= 0) {
      end = e;
      boolean negativeExponent = number.charAt(e + 1) == '-';
      for (int i = e + 1; i < number.length() && exponent < 1000000; i++) {
        int digit = number.charAt(i) - '0';
        exponent = digit >= 0 ? exponent * 10 + digit : exponent;
      }
      exponent = negativeExponent ? -exponent : exponent;
    }
    String digits = number.substring(negative ? 1 : 0, end);
    int dot = digits.indexOf('.');
    long integerDigits = (dot < 0 ? digits.length() : dot) + exponent;
    digits = dot < 0 ? digits : digits.substring(0, dot) + digits.substring(dot + 1);
    int value = 0;
    // Each further digit past the given ones multiplies by ten; after 32 of them the value is 0.
    for (int i = 0; i < integerDigits && i < digits.length() + 32; i++) {
      value = value * 10 + (i < digits.length() ? digits.charAt(i) - '0' : 0);
    }
    return negative ? -value : value;
  }

  /** Writes this value with the settings of {@code writer}. */
  public void writeTo(SimpleJsonWriter writer) {
    switch (tag) {
      case OBJECT -> {
        writer.beginObject();
        for (Map.Entry<String, SimpleJsonValue> member : named.entrySet()) {
          writer.name(member.getKey());
          member.getValue().writeTo(writer);
        }
        writer.endObject();
      }
      case ARRAY -> {
        writer.beginArray();
        for (SimpleJsonValue item : items) {
          item.writeTo(writer);
        }
        writer.endArray();
      }
      case STRING -> writer.stringValue(text);
      case NULL -> writer.nullValue();
      case NUMBER, BOOLEAN -> writer.rawValue(text);
    }
  }

  /** Returns the compact JSON text of this value, without HTML escaping. */
  @Override
  public String toString() {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writeTo(writer);
    return writer.toString();
  }
}
