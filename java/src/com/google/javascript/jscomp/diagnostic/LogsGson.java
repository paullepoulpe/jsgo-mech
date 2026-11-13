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

import com.google.javascript.jscomp.base.SimpleBiMap;
import com.google.javascript.jscomp.base.SimpleJsonWriter;
import com.google.javascript.jscomp.base.SimpleListMultimap;
import com.google.javascript.jscomp.base.SimpleMultiset;
import com.google.javascript.jscomp.base.SimpleSetMultimap;
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
   * SimpleListMultimap}, {@link SimpleSetMultimap} and {@link SimpleBiMap} (written as their
   * {@code asMap()}), {@link Map} (an object whose names are the keys' {@code String.valueOf};
   * entries with null values are left out), {@link Collection} and {@link SimpleMultiset} (an
   * array), {@link String}, {@link Boolean}, {@link Integer}, {@link Long} and {@link Enum} (its
   * name).
   */
  static String toJson(@Nullable Object o) {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writer.setHtmlSafe(true);
    writer.setSerializeNulls(false);
    write(writer, o);
    return writer.toString();
  }

  private static void write(SimpleJsonWriter out, @Nullable Object value) {
    if (value == null) {
      out.nullValue();
    } else if (value instanceof Able able) {
      out.rawValue(toJson(able.toLogsGson()));
    } else if (value instanceof SimpleListMultimap<?, ?> multimap) {
      out.rawValue(toJson(multimap.asMap()));
    } else if (value instanceof SimpleSetMultimap<?, ?> multimap) {
      out.rawValue(toJson(multimap.asMap()));
    } else if (value instanceof SimpleBiMap<?, ?> bimap) {
      out.rawValue(toJson(bimap.asMap()));
    } else if (value instanceof SimpleMultiset<?> multiset) {
      out.rawValue(toJson(multiset.asList()));
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
      out.stringValue(string);
    } else if (value instanceof Boolean bool) {
      out.booleanValue(bool);
    } else if (value instanceof Integer || value instanceof Long) {
      out.longValue(((Number) value).longValue());
    } else if (value instanceof Enum<?> enumValue) {
      out.stringValue(enumValue.name());
    } else {
      throw new IllegalArgumentException(
          "Cannot log a " + value.getClass().getName() + "; it must implement LogsGson.Able");
    }
  }

  private LogsGson() {
    throw new AssertionError();
  }
}
