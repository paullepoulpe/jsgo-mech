/*
 * Copyright 2016 The Closure Compiler Authors.
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

import com.google.common.collect.ImmutableList;
import com.google.javascript.jscomp.base.Json;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.base.SimpleJsonValue;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.jspecify.annotations.Nullable;

/** Java implementation of the source map parser. */
public class SourceMapObjectParser {
  public static Outcome<SourceMapObject> parse(String contents) {

    SourceMapObject.Builder builder = SourceMapObject.builder();

    Outcome<SimpleJsonValue> parsed = Json.parse(contents);
    if (parsed.isErr()) {
      return Outcome.err(
          new SourceMapParseException("JSON parse exception: " + parsed.errorMessage().orElse("null")));
    }
    SimpleJsonValue sourceMapRoot = parsed.get();
    if (sourceMapRoot.kind() != SimpleJsonValue.Kind.OBJECT) {
      return Outcome.err(new SourceMapParseException("Invalid map format: not a JSON object"));
    }
    Outcome<Integer> version = getInt(sourceMapRoot, "version");
    if (version.isErr()) {
      return Outcome.err(version.error());
    }
    builder.setVersion(version.get());
    Outcome<@Nullable String> file = getStringOrNull(sourceMapRoot, "file");
    if (file.isErr()) {
      return Outcome.err(file.error());
    }
    builder.setFile(Optional.ofNullable(file.get()));
    int lineCount = -1;
    if (sourceMapRoot.get("lineCount") != null) {
      Outcome<Integer> lineCountOutcome = getInt(sourceMapRoot, "lineCount");
      if (lineCountOutcome.isErr()) {
        return Outcome.err(lineCountOutcome.error());
      }
      lineCount = lineCountOutcome.get();
    }
    builder.setLineCount(lineCount);
    Outcome<@Nullable String> mappings = getStringOrNull(sourceMapRoot, "mappings");
    if (mappings.isErr()) {
      return Outcome.err(mappings.error());
    }
    builder.setMappings(Optional.ofNullable(mappings.get()));
    Outcome<@Nullable String> sourceRoot = getStringOrNull(sourceMapRoot, "sourceRoot");
    if (sourceRoot.isErr()) {
      return Outcome.err(sourceRoot.error());
    }
    builder.setSourceRoot(Optional.ofNullable(sourceRoot.get()));

    if (sourceMapRoot.get("sections") != null) {
      Outcome<ImmutableList<SourceMapSection>> sections =
          buildSections(sourceMapRoot.get("sections"));
      if (sections.isErr()) {
        return Outcome.err(sections.error());
      }
      builder.setSections(sections.get());
    }

    Outcome<String @Nullable []> sources = getJavaStringArray(sourceMapRoot, "sources");
    if (sources.isErr()) {
      return Outcome.err(sources.error());
    }
    builder.setSources(sources.get());
    Outcome<String @Nullable []> sourcesContent =
        getJavaStringArray(sourceMapRoot, "sourcesContent");
    if (sourcesContent.isErr()) {
      return Outcome.err(sourcesContent.error());
    }
    builder.setSourcesContent(sourcesContent.get());
    Outcome<String @Nullable []> names = getJavaStringArray(sourceMapRoot, "names");
    if (names.isErr()) {
      return Outcome.err(names.error());
    }
    builder.setNames(names.get());

    Map<String, Object> extensions = new LinkedHashMap<>();
    for (Map.Entry<String, SimpleJsonValue> entry : sourceMapRoot.members().entrySet()) {
      if (entry.getKey().startsWith("x_")) {
        extensions.put(entry.getKey(), entry.getValue());
      }
    }
    builder.setExtensions(Collections.unmodifiableMap(extensions));

    return Outcome.ok(builder.build());
  }

  // TODO: fenghaolw - Migrate everyone to use this method and rename it to parse.
  public static Outcome<SourceMapObject> parseFast(String contents) {
    SourceMapObject.Builder builder = SourceMapObject.builder();
    builder.setLineCount(-1);

    // A catch here used to rewrap every exception of the parse, the parser's own
    // SourceMapParseException included; an error outcome is rewrapped in the same way.
    Outcome<Void> parsed = parseFastInto(builder, contents);
    if (parsed.isErr()) {
      return Outcome.err(new SourceMapParseException("Parse exception: " + parsed.error()));
    }

    return Outcome.ok(builder.build());
  }

  private static Outcome<Void> parseFastInto(SourceMapObject.Builder builder, String contents) {
    SourceMapJsonLexer lexer = new SourceMapJsonLexer(contents);
    Outcome<Void> begun = lexer.beginObject();
    if (begun.isErr()) {
      return Outcome.err(begun.error());
    }
    Map<String, Object> extensions = new LinkedHashMap<>();

    while (lexer.hasNext()) {
      Outcome<String> nameOutcome = lexer.nextName();
      if (nameOutcome.isErr()) {
        return Outcome.err(nameOutcome.error());
      }
      String name = nameOutcome.get();
      switch (name) {
        case "version" -> {
          Outcome<Integer> version = lexer.nextInt();
          if (version.isErr()) {
            return Outcome.err(version.error());
          }
          builder.setVersion(version.get());
        }
        case "file" -> {
          Outcome<@Nullable String> file = lexer.nextStringOrNull();
          if (file.isErr()) {
            return Outcome.err(file.error());
          }
          builder.setFile(Optional.ofNullable(file.get()));
        }
        case "lineCount" -> {
          Outcome<Integer> lineCount = lexer.nextInt();
          if (lineCount.isErr()) {
            return Outcome.err(lineCount.error());
          }
          builder.setLineCount(lineCount.get());
        }
        case "mappings" -> {
          Outcome<@Nullable String> mappings = lexer.nextStringOrNull();
          if (mappings.isErr()) {
            return Outcome.err(mappings.error());
          }
          builder.setMappings(Optional.ofNullable(mappings.get()));
        }
        case "sourceRoot" -> {
          Outcome<@Nullable String> sourceRoot = lexer.nextStringOrNull();
          if (sourceRoot.isErr()) {
            return Outcome.err(sourceRoot.error());
          }
          builder.setSourceRoot(Optional.ofNullable(sourceRoot.get()));
        }
        case "sections" -> {
          Outcome<String> rawSections = lexer.nextRawValue();
          if (rawSections.isErr()) {
            return Outcome.err(rawSections.error());
          }
          Outcome<SimpleJsonValue> sectionsArray = Json.parse(rawSections.get());
          if (sectionsArray.isErr()) {
            return Outcome.err(new SourceMapParseException(sectionsArray.errorMessage().orElse(null)));
          }
          Outcome<ImmutableList<SourceMapSection>> sections = buildSections(sectionsArray.get());
          if (sections.isErr()) {
            return Outcome.err(sections.error());
          }
          builder.setSections(sections.get());
        }
        case "sources" -> {
          Outcome<String @Nullable []> sources = readStringArray(lexer);
          if (sources.isErr()) {
            return Outcome.err(sources.error());
          }
          builder.setSources(sources.get());
        }
        case "sourcesContent" -> {
          Outcome<String @Nullable []> sourcesContent = readStringArray(lexer);
          if (sourcesContent.isErr()) {
            return Outcome.err(sourcesContent.error());
          }
          builder.setSourcesContent(sourcesContent.get());
        }
        case "names" -> {
          Outcome<String @Nullable []> names = readStringArray(lexer);
          if (names.isErr()) {
            return Outcome.err(names.error());
          }
          builder.setNames(names.get());
        }
        default -> {
          if (name.startsWith("x_")) {
            Outcome<String> rawValue = lexer.nextRawValue();
            if (rawValue.isErr()) {
              return Outcome.err(rawValue.error());
            }
            Outcome<SimpleJsonValue> value = Json.parse(rawValue.get());
            if (value.isErr()) {
              return Outcome.err(new SourceMapParseException(value.errorMessage().orElse(null)));
            }
            extensions.put(name, value.get());
          } else {
            Outcome<Void> skipped = lexer.skipValue();
            if (skipped.isErr()) {
              return Outcome.err(skipped.error());
            }
          }
        }
      }
      Outcome<Void> comma = lexer.checkComma();
      if (comma.isErr()) {
        return Outcome.err(comma.error());
      }
    }
    Outcome<Void> ended = lexer.endObject();
    if (ended.isErr()) {
      return Outcome.err(ended.error());
    }
    lexer.skipWhitespace();
    if (lexer.pos < lexer.length) {
      return Outcome.err(new SourceMapParseException("Unexpected trailing characters"));
    }
    builder.setExtensions(Collections.unmodifiableMap(extensions));
    return Outcome.ok();
  }

  private static Outcome<ImmutableList<SourceMapSection>> buildSections(SimpleJsonValue sections) {
    if (sections.kind() != SimpleJsonValue.Kind.ARRAY) {
      return Outcome.err(invalidMember("sections"));
    }
    ImmutableList.Builder<SourceMapSection> listBuilder = ImmutableList.builder();
    for (SimpleJsonValue each : sections.elements()) {
      Outcome<SourceMapSection> section = buildSection(each);
      if (section.isErr()) {
        return Outcome.err(section.error());
      }
      listBuilder.add(section.get());
    }
    return Outcome.ok(listBuilder.build());
  }

  private static Outcome<SourceMapSection> buildSection(SimpleJsonValue section) {
    SimpleJsonValue offset = section.get("offset");
    if (offset == null) {
      return Outcome.err(invalidMember("offset"));
    }
    Outcome<Integer> line = getInt(offset, "line");
    if (line.isErr()) {
      return Outcome.err(line.error());
    }
    Outcome<Integer> column = getInt(offset, "column");
    if (column.isErr()) {
      return Outcome.err(column.error());
    }

    SimpleJsonValue map = section.get("map");
    if (map != null && section.get("url") != null) {
      return Outcome.err(
          new SourceMapParseException(
              "Invalid map format: section may not have both 'map' and 'url'"));
    } else if (section.get("url") != null) {
      Outcome<String> url = section.get("url").asString();
      if (url.isErr()) {
        return Outcome.err(invalidMember("url"));
      }
      return Outcome.ok(SourceMapSection.forURL(url.get(), line.get(), column.get()));
    } else if (map != null) {
      String mapStr =
          map.kind() == SimpleJsonValue.Kind.STRING ? map.asString().get() : map.toString();
      return Outcome.ok(SourceMapSection.forMap(mapStr, line.get(), column.get()));
    }

    return Outcome.err(
        new SourceMapParseException(
            "Invalid map format: section must have either 'map' or 'url'"));
  }

  private static SourceMapParseException invalidMember(String key) {
    return new SourceMapParseException("Invalid map format: missing or malformed '" + key + "'");
  }

  /** The int value of a member that must be present. */
  private static Outcome<Integer> getInt(SimpleJsonValue object, String key) {
    SimpleJsonValue value = object.get(key);
    if (value == null) {
      return Outcome.err(invalidMember(key));
    }
    Outcome<Integer> result = value.asInt();
    if (result.isErr()) {
      return Outcome.err(invalidMember(key));
    }
    return result;
  }

  private static Outcome<@Nullable String> getStringOrNull(SimpleJsonValue object, String key) {
    SimpleJsonValue value = object.get(key);
    if (value == null || value.kind() == SimpleJsonValue.Kind.NULL) {
      return Outcome.ok(null);
    }
    Outcome<String> result = value.asString();
    if (result.isErr()) {
      return Outcome.err(invalidMember(key));
    }
    return Outcome.ok(result.get());
  }

  private static Outcome<String @Nullable []> getJavaStringArray(
      SimpleJsonValue object, String key) {
    SimpleJsonValue array = object.get(key);
    if (array == null || array.kind() == SimpleJsonValue.Kind.NULL) {
      return Outcome.ok(null);
    } else if (array.kind() != SimpleJsonValue.Kind.ARRAY) {
      return Outcome.err(invalidMember(key));
    }
    int len = array.elements().size();
    String[] result = new String[len];
    for (int i = 0; i < len; i++) {
      Outcome<@Nullable String> item = Outcome.ok(null);
      if (array.elements().get(i).kind() != SimpleJsonValue.Kind.NULL) {
        item = array.elements().get(i).asString();
      }
      if (item.isErr()) {
        return Outcome.err(invalidMember(key));
      }
      result[i] = item.get();
    }
    return Outcome.ok(result);
  }

  private static Outcome<String @Nullable []> readStringArray(SourceMapJsonLexer lexer) {
    lexer.skipWhitespace();
    if (lexer.pos < lexer.length && lexer.json.startsWith("null", lexer.pos)) {
      lexer.pos += 4;
      return Outcome.ok(null);
    }
    List<String> list = new ArrayList<>();
    Outcome<Void> begun = lexer.beginArray();
    if (begun.isErr()) {
      return Outcome.err(begun.error());
    }
    while (lexer.hasNext()) {
      Outcome<@Nullable String> item = lexer.nextStringOrNull();
      if (item.isErr()) {
        return Outcome.err(item.error());
      }
      list.add(item.get());
      Outcome<Void> comma = lexer.checkComma();
      if (comma.isErr()) {
        return Outcome.err(comma.error());
      }
    }
    Outcome<Void> ended = lexer.endArray();
    if (ended.isErr()) {
      return Outcome.err(ended.error());
    }
    return Outcome.ok(list.toArray(new String[0]));
  }

  private SourceMapObjectParser() {}
}
