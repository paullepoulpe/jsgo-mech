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

package com.google.javascript.jscomp;

import com.google.common.annotations.VisibleForTesting;
import com.google.common.base.Strings;
import com.google.common.collect.ImmutableMap;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.SimpleBiMap;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/** A utility class for generating and parsing id mappings held by {@link ReplaceIdGenerators}. */
public final class IdMappingUtil {

  @VisibleForTesting static final char NEW_LINE = '\n';

  // Prevent instantiation.
  private IdMappingUtil() {}

  /**
   * @return The serialize map of generators and their ids and their replacements.
   */
  static String generateSerializedIdMappings(Map<String, Map<String, String>> idGeneratorMaps) {
    StringBuilder sb = new StringBuilder();
    for (Map.Entry<String, Map<String, String>> replacements : idGeneratorMaps.entrySet()) {
      if (!replacements.getValue().isEmpty()) {
        sb.append('[').append(replacements.getKey()).append(']').append(NEW_LINE).append(NEW_LINE);

        for (Map.Entry<String, String> replacement : replacements.getValue().entrySet()) {
          sb.append(replacement.getKey())
              .append(':')
              .append(replacement.getValue())
              .append(NEW_LINE);
        }
        sb.append(NEW_LINE);
      }
    }
    return sb.toString();
  }

  /** A stateful pull parser for reading id mapping sections and entries line by line. */
  public static class MappingReader {
    private final List<String> lines;
    private int nextLine = 0;
    private String currentSection = null;
    private String currentKey = null;
    private String currentValue = null;
    private int lineIndex = 0;

    public MappingReader(String text) {
      this.lines = Io.splitLines(text);
    }

    public boolean next() {
      String line;
      while (true) {
        if (nextLine == lines.size()) {
          break;
        }
        line = lines.get(nextLine);
        nextLine++;
        lineIndex++;
        if (line.isEmpty()) {
          continue;
        }
        if (line.charAt(0) == '[') {
          currentSection = line.substring(1, line.length() - 1);
          currentKey = null;
          currentValue = null;
          return true;
        } else {
          int split = line.indexOf(':');
          if (split != -1) {
            currentKey = line.substring(0, split);
            currentValue = line.substring(split + 1);
            return true;
          } else {
            throw new IllegalArgumentException(
                String.format("Cannot parse id map.\n Line: %s, lineIndex: %d", line, lineIndex));
          }
        }
      }
      return false;
    }

    public boolean isSection() {
      return currentKey == null;
    }

    public String getSection() {
      return currentSection;
    }

    public String getKey() {
      return currentKey;
    }

    public String getValue() {
      return currentValue;
    }

    public int getLineIndex() {
      return lineIndex;
    }
  }

  public static ImmutableMap<String, String> parseSectionOfBytes(
      byte[] idMappings, String sectionFilter) {
    ImmutableMap.Builder<String, String> mapBuilder = ImmutableMap.builder();

    MappingReader mr = new MappingReader(Io.utf8String(idMappings));
    boolean inSection = false;

    while (mr.next()) {
      if (mr.isSection()) {
        if (inSection) {
          break; // Next section found, stop
        }
        if (mr.getSection().equals(sectionFilter)) {
          inSection = true;
        }
      } else if (inSection) {
        mapBuilder.put(mr.getKey(), mr.getValue());
      }
    }
    return mapBuilder.buildKeepingLast();
  }

  public static Map<String, SimpleBiMap<String, String>> parseSerializedIdMappingsText(
      String idMappings) {
    Map<String, SimpleBiMap<String, String>> resultMap = new LinkedHashMap<>();
    MappingReader mr = new MappingReader(idMappings);
    SimpleBiMap<String, String> currentSectionMap = null;
    Set<String> sectionNames = new LinkedHashSet<>();

    while (mr.next()) {
      if (mr.isSection()) {
        String sectionName = mr.getSection();
        if (!sectionNames.add(sectionName)) {
          throw new IllegalArgumentException(
              String.format(
                  "Cannot parse id map: Duplicate section %s\n lineIndex: %d",
                  sectionName, mr.getLineIndex()));
        }
        currentSectionMap = SimpleBiMap.create();
        resultMap.put(sectionName, currentSectionMap);
      } else {
        if (currentSectionMap == null) {
          throw new IllegalArgumentException("Mapping entry outside of section");
        }
        currentSectionMap.put(mr.getKey(), mr.getValue()); // throws if duplicate values
      }
    }
    return resultMap;
  }

  public static Map<String, SimpleBiMap<String, String>> parseSerializedIdMappings(String idMappings) {
    if (Strings.isNullOrEmpty(idMappings)) {
      return ImmutableMap.of();
    }
    return parseSerializedIdMappingsText(idMappings);
  }
}
