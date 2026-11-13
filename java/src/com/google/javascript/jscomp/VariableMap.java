/*
 * Copyright 2005 The Closure Compiler Authors.
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

import static java.util.Map.Entry.comparingByKey;

import com.google.common.annotations.VisibleForTesting;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSortedMap;
import com.google.common.collect.ImmutableSortedSet;
import com.google.errorprone.annotations.Immutable;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.base.SimpleBiMap;
import com.google.javascript.jscomp.base.TextFiles;
import java.io.IOException;
import java.io.Serializable;
import java.text.ParseException;
import java.util.Map;
import java.util.Optional;

/**
 * Stores the mapping from original variable name to new variable names.
 *
 * @see RenameVars
 */
@Immutable
public final class VariableMap implements Serializable {

  private static final char SEPARATOR = ':';

  /** Maps between original source name to new name */
  private final SimpleBiMap<String, String> map;

  public VariableMap(Map<String, String> map) {
    this.map = SimpleBiMap.copyOf(map);
  }

  /** Given an original variable name, look up new name, may return null if it's not found. */
  public Optional<String> lookupNewName(String sourceName) {
    return Optional.ofNullable(map.get(sourceName));
  }

  /** Given a new variable name, lookup the source name, may return null if it's not found. */
  public String lookupSourceName(Optional<String> newName) {
    return map.inverse().get(newName.orElse(null));
  }

  /** Returns an immutable mapping from original names to new names. */
  public ImmutableMap<String, String> getOriginalNameToNewNameMap() {
    return ImmutableSortedMap.copyOf(map.asMap());
  }

  /** Returns an immutable mapping from new names to original names. */
  public ImmutableMap<String, String> getNewNameToOriginalNameMap() {
    return ImmutableMap.copyOf(map.inverse());
  }

  /** Saves the variable map to a file. */
  public Outcome<Void> save(String filename) {
    return Io.writeBytes(filename, toBytes());
  }

  /** Reads the variable map from a file written via {@link #save(String)}. */
  public static Outcome<VariableMap> load(String filename) {
    Outcome<byte[]> bytes = Io.readBytes(filename);
    if (bytes.isErr()) {
      return Outcome.err(bytes.error());
    }
    Outcome<VariableMap> mapRes = fromBytes(bytes.get());
    if (mapRes.isErr()) {
      // Wrap parse exception for backwards compatibility.
      return Outcome.err(new IOException(mapRes.error()));
    }
    return mapRes;
  }

  /** Serializes the variable map to a byte array. */
  public byte[] toBytes() {
    StringBuilder writer = new StringBuilder();
    // The output order should be stable.
    for (Map.Entry<String, String> entry :
        ImmutableSortedSet.copyOf(comparingByKey(), map.asMap().entrySet())) {
      writer.append(escape(entry.getKey()));
      writer.append(SEPARATOR);
      writer.append(escape(entry.getValue()));
      writer.append('\n');
    }
    return Io.utf8Bytes(writer.toString());
  }

  /** Deserializes the variable map from a byte array returned by {@link #toBytes()}. */
  public static Outcome<VariableMap> fromBytes(byte[] bytes) {
    return fromLines(Io.utf8String(bytes));
  }

  /** Deserializes the variable map from the bytes of a stream of lines. */
  public static Outcome<VariableMap> fromStreamBytes(byte[] bytes) {
    ImmutableMap.Builder<String, String> map = ImmutableMap.builder();
    for (String line : TextFiles.splitLines(Io.utf8String(bytes))) {
      if (line.isEmpty()) {
        continue;
      }
      int pos = findIndexOfUnescapedChar(line, SEPARATOR);
      if (pos <= 0) {
        return Outcome.err(new ParseException("Bad line: " + line, 0));
      }
      map.put(
          unescape(line.substring(0, pos)),
          pos == line.length() - 1 ? "" : unescape(line.substring(pos + 1)));
    }
    return Outcome.ok(new VariableMap(map.buildOrThrow()));
  }

  /** Deserializes the variable map from a string. */
  public static Outcome<VariableMap> fromLines(String string) {
    ImmutableMap.Builder<String, String> map = ImmutableMap.builder();
    int startOfLine = 0;
    while (startOfLine < string.length()) {
      int newLine = string.indexOf('\n', startOfLine);
      if (newLine == -1) {
        newLine = string.length();
      }
      int endOfLine = newLine;
      if (newLine > 0 && string.charAt(newLine - 1) == '\r') {
        newLine--;
      }
      String line = string.substring(startOfLine, newLine);
      startOfLine = endOfLine + 1; // update index for next iteration
      if (line.isEmpty()) {
        continue;
      }
      int pos = findIndexOfUnescapedChar(line, SEPARATOR);
      if (pos <= 0) {
        return Outcome.err(new ParseException("Bad line: " + line, 0));
      }
      map.put(
          unescape(line.substring(0, pos)),
          pos == line.length() - 1 ? "" : unescape(line.substring(pos + 1)));
    }
    return Outcome.ok(new VariableMap(map.buildOrThrow()));
  }

  private static String escape(String value) {
    return value.replace("\\", "\\\\").replace(":", "\\:").replace("\n", "\\n");
  }

  private static int findIndexOfUnescapedChar(String value, char stopChar) {
    int len = value.length();
    for (int i = 0; i < len; ) {
      int stopCharIndex = value.indexOf(stopChar, i);
      if (stopCharIndex == -1) {
        return -1;
      }
      if (stopCharIndex == 0 || value.charAt(stopCharIndex - 1) != '\\') {
        // it isn't escaped, return
        return stopCharIndex;
      }
      i = stopCharIndex + 1;
    }
    return -1;
  }

  private static String unescape(String value) {
    int slashIndex = value.indexOf('\\');
    if (slashIndex == -1) {
      return value;
    }
    StringBuilder sb = new StringBuilder(value.length() - 1);
    sb.append(value, 0, slashIndex);
    int len = value.length();
    for (int i = slashIndex; i < len; i++) {
      char c = value.charAt(i);
      if (c == '\\') {
        ++i;
        if (i < len) {
          c = value.charAt(i);
        }
      }
      sb.append(c);
    }
    return sb.toString();
  }

  /**
   * Initializes the variable map from an existing map.
   *
   * @param map The map to use from original names to generated names. It is copied and changes to
   *     the specified map will not affect the returned object.
   */
  public static VariableMap fromMap(Map<String, String> map) {
    return new VariableMap(map);
  }

  @VisibleForTesting
  ImmutableMap<String, String> toMap() {
    return ImmutableMap.copyOf(map.asMap());
  }
}
