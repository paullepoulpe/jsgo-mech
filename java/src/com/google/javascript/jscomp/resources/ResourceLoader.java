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

package com.google.javascript.jscomp.resources;


import com.google.common.collect.ImmutableMap;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;

/**
 * Utility class that handles resource loading.
 */
public final class ResourceLoader {
  public static String loadTextResource(Class<?> clazz, String path) {
    Outcome<byte[]> bytes = Io.readResource(clazz, path);
    if (bytes.isErr()) {
      throw new RuntimeException(bytes.error());
    }
    if (bytes.get() == null) {
      if (!resourceExists(clazz, path)) {
        throw new RuntimeException("No such resource: " + path);
      }
      throw new NullPointerException();
    }
    return Io.utf8String(bytes.get());
  }

  public static ImmutableMap<String, String> loadPropertiesMap(
      Class<?> clazz, String resourceName) {
    return PropertiesParser.parse(loadTextResource(clazz, resourceName));
  }

  public static boolean resourceExists(Class<?> clazz, String path) {
    return Io.resourceExists(clazz, path);
  }

  private ResourceLoader() {}
}
