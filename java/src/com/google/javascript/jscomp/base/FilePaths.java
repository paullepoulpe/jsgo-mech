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
import java.util.List;
import java.util.Optional;

/**
 * Arithmetic on file paths as strings. A path is names separated by "/"; it is absolute if it
 * starts with "/". Nothing here looks at the file system, except {@link #toAbsolute}, which asks
 * for the working directory.
 */
public final class FilePaths {
  private FilePaths() {}

  /** The name separator. */
  public static final String SEPARATOR = "/";

  /**
   * The path in its plain spelling: a run of separators is one separator, and there is no
   * separator at the end (except in "/" itself). Every other function here returns plain
   * spellings and reads its arguments as if they were given in theirs.
   */
  public static String clean(String path) {
    StringBuilder plain = new StringBuilder(path.length());
    for (int i = 0; i < path.length(); i++) {
      char c = path.charAt(i);
      if (c != '/' || plain.length() == 0 || plain.charAt(plain.length() - 1) != '/') {
        plain.append(c);
      }
    }
    if (plain.length() > 1 && plain.charAt(plain.length() - 1) == '/') {
      plain.setLength(plain.length() - 1);
    }
    return plain.toString();
  }

  /** The parts that are not empty, joined by separators, as a plain spelling. */
  public static String join(String first, String... more) {
    StringBuilder path = new StringBuilder(first);
    for (String part : more) {
      if (!part.isEmpty()) {
        if (path.length() > 0) {
          path.append('/');
        }
        path.append(part);
      }
    }
    return clean(path.toString());
  }

  public static boolean isAbsolute(String path) {
    return path.startsWith("/");
  }

  /** The last name of the path; "" for "/" and for "". */
  public static String fileName(String path) {
    String plain = clean(path);
    return plain.substring(plain.lastIndexOf('/') + 1);
  }

  /** The path without its last name; empty if it is a single name, "" or "/". */
  public static Optional<String> parent(String path) {
    String plain = clean(path);
    int slash = plain.lastIndexOf('/');
    if (slash < 0 || plain.equals("/")) {
      return Optional.empty();
    }
    return Optional.of(slash == 0 ? "/" : plain.substring(0, slash));
  }

  /** The names of a plain spelling: none for "/", one empty name for "". */
  private static List<String> names(String plain) {
    List<String> names = new ArrayList<>();
    if (plain.equals("/")) {
      return names;
    }
    int start = plain.startsWith("/") ? 1 : 0;
    while (true) {
      int slash = plain.indexOf('/', start);
      if (slash < 0) {
        names.add(plain.substring(start));
        return names;
      }
      names.add(plain.substring(start, slash));
      start = slash + 1;
    }
  }

  private static String joinNames(List<String> names, int from) {
    StringBuilder path = new StringBuilder();
    for (int i = from; i < names.size(); i++) {
      if (i > from) {
        path.append('/');
      }
      path.append(names.get(i));
    }
    return path.toString();
  }

  /**
   * The path without its "." names and without each "name/.." pair. A ".." with no name before it
   * stays in a relative path and is dropped from an absolute one. "" if nothing is left of a
   * relative path.
   */
  public static String normalize(String path) {
    String plain = clean(path);
    boolean absolute = isAbsolute(plain);
    List<String> kept = new ArrayList<>();
    for (String name : names(plain)) {
      if (name.equals(".")) {
        continue;
      }
      if (!name.equals("..")) {
        kept.add(name);
      } else if (!kept.isEmpty() && !kept.get(kept.size() - 1).equals("..")) {
        kept.remove(kept.size() - 1);
      } else if (!absolute) {
        kept.add(name);
      }
    }
    return (absolute ? "/" : "") + joinNames(kept, 0);
  }

  /** {@code other} if it is absolute or {@code path} is "", else {@code path/other}. */
  public static String resolve(String path, String other) {
    String base = clean(path);
    String target = clean(other);
    if (isAbsolute(target) || base.isEmpty()) {
      return target;
    }
    if (target.isEmpty()) {
      return base;
    }
    return base.equals("/") ? "/" + target : base + "/" + target;
  }

  /**
   * The relative path that leads from {@code path} to {@code other}: {@code resolve(path, result)}
   * names the same file as {@code other}. From "", that is {@code other} as it is; otherwise both
   * are normalized first if either has a "." or ".." name. Empty if there is no such path: only one of the two is absolute, or {@code path} starts
   * with more ".." names than {@code other}.
   */
  public static Optional<String> relativize(String path, String other) {
    String base = clean(path);
    String target = clean(other);
    if (isAbsolute(base) != isAbsolute(target)) {
      return Optional.empty();
    }
    if (base.equals(target)) {
      return Optional.of("");
    }
    if (base.isEmpty()) {
      return Optional.of(target);
    }
    if (hasDots(base) || hasDots(target)) {
      base = normalize(base);
      target = normalize(target);
      if (base.equals(target)) {
        return Optional.of("");
      }
      if (base.isEmpty()) {
        return Optional.of(target);
      }
    }
    List<String> from = names(base);
    List<String> to = target.isEmpty() ? new ArrayList<>() : names(target);
    int common = 0;
    while (common < from.size()
        && common < to.size()
        && from.get(common).equals(to.get(common))) {
      common++;
    }
    StringBuilder result = new StringBuilder();
    for (int i = common; i < from.size(); i++) {
      if (from.get(i).equals("..")) {
        return Optional.empty();
      }
      result.append(result.length() > 0 ? "/.." : "..");
    }
    String rest = joinNames(to, common);
    if (result.length() > 0 && !rest.isEmpty()) {
      result.append('/');
    }
    return Optional.of(result.append(rest).toString());
  }

  private static boolean hasDots(String plain) {
    for (String name : names(plain)) {
      if (name.equals(".") || name.equals("..")) {
        return true;
      }
    }
    return false;
  }

  /** The path itself if it is absolute, else the working directory resolved with it. */
  public static String toAbsolute(String path) {
    return resolve(Io.workingDirectory(), path);
  }
}
