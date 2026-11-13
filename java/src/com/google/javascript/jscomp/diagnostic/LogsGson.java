/*
 * Copyright 2019 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.diagnostic;

import com.google.common.collect.Multimap;
import com.google.gson.JsonIOException;
import com.google.gson.stream.JsonWriter;
import java.io.IOException;
import java.io.StringWriter;
import java.util.Collection;
import java.util.Map;
import org.jspecify.annotations.Nullable;

/** A JSON stringifier tailored to generating logs output. */
public final class LogsGson {

  /**
   * A conversion to a preferred logging representation.
   *
   * <p>When an implementor is logged, the result of this method will be JSON stringified in place
   * of the implementor.
   *
   * <p>Classes whose instances are logged must implement this interface, unless they are one of the
   * types listed at {@link #toJson}. A class that should be logged as a JSON object returns an
   * insertion-ordered {@code Map} from property name to property value.
   */
  public interface Able {
    Object toLogsGson();
  }

  /**
   * Returns the compact, HTML-safe JSON text for a value made of: null, {@link Able}, {@link
   * Multimap} (written as its {@code asMap()}), {@link Map} (an object whose names are the keys'
   * {@code String.valueOf}; entries with null values are left out), {@link Collection} (an array),
   * {@link String}, {@link Boolean}, {@link Integer}, {@link Long} and {@link Enum} (its name).
   */
  static String toJson(@Nullable Object o) {
    StringWriter out = new StringWriter();
    JsonWriter writer = new JsonWriter(out);
    writer.setLenient(true);
    writer.setHtmlSafe(true);
    writer.setSerializeNulls(false);
    try {
      write(writer, o);
    } catch (IOException e) {
      throw new JsonIOException(e);
    }
    return out.toString();
  }

  private static void write(JsonWriter out, @Nullable Object value) throws IOException {
    if (value == null) {
      out.nullValue();
    } else if (value instanceof Able able) {
      out.jsonValue(toJson(able.toLogsGson()));
    } else if (value instanceof Multimap<?, ?> multimap) {
      out.jsonValue(toJson(multimap.asMap()));
    } else if (value instanceof Map<?, ?> map) {
      out.beginObject();
      for (Map.Entry<?, ?> entry : map.entrySet()) {
        out.name(String.valueOf(entry.getKey()));
        write(out, entry.getValue());
      }
      out.endObject();
    } else if (value instanceof Collection<?> collection) {
      out.beginArray();
      for (Object element : collection) {
        write(out, element);
      }
      out.endArray();
    } else if (value instanceof String string) {
      out.value(string);
    } else if (value instanceof Boolean bool) {
      out.value(bool);
    } else if (value instanceof Integer || value instanceof Long) {
      out.value((Number) value);
    } else if (value instanceof Enum<?> enumValue) {
      out.value(enumValue.name());
    } else {
      throw new IllegalArgumentException(
          "Cannot log a " + value.getClass().getName() + "; it must implement LogsGson.Able");
    }
  }

  private LogsGson() {
    throw new AssertionError();
  }
}
