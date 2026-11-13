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
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonNull;
import com.google.gson.JsonObject;
import com.google.gson.JsonParseException;
import com.google.gson.JsonParser;
import com.google.gson.JsonSyntaxException;
import com.google.gson.stream.JsonReader;
import com.google.gson.stream.JsonToken;
import com.google.javascript.jscomp.base.Outcome;
import java.io.EOFException;
import java.io.IOException;
import java.io.StringReader;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.jspecify.annotations.Nullable;

/** Java implementation of the source map parser. */
public class SourceMapObjectParser {
  public static Outcome<SourceMapObject> parse(String contents) {

    SourceMapObject.Builder builder = SourceMapObject.builder();

    try {
      JsonObject sourceMapRoot = parseJsonObject(contents);
      builder.setVersion(sourceMapRoot.get("version").getAsInt());
      builder.setFile(getStringOrNull(sourceMapRoot, "file"));
      builder.setLineCount(
          sourceMapRoot.has("lineCount") ? sourceMapRoot.get("lineCount").getAsInt() : -1);
      builder.setMappings(getStringOrNull(sourceMapRoot, "mappings"));
      builder.setSourceRoot(getStringOrNull(sourceMapRoot, "sourceRoot"));

      if (sourceMapRoot.has("sections")) {
        ImmutableList.Builder<SourceMapSection> listBuilder = ImmutableList.builder();
        for (JsonElement each : sourceMapRoot.get("sections").getAsJsonArray()) {
          Outcome<SourceMapSection> section = buildSection(each.getAsJsonObject());
          if (section.isErr()) {
            return Outcome.err(section.error());
          }
          listBuilder.add(section.get());
        }
        builder.setSections(listBuilder.build());
      }

      builder.setSources(getJavaStringArray(sourceMapRoot.get("sources")));
      builder.setSourcesContent(getJavaStringArray(sourceMapRoot.get("sourcesContent")));
      builder.setNames(getJavaStringArray(sourceMapRoot.get("names")));

      Map<String, Object> extensions = new LinkedHashMap<>();
      for (Map.Entry<String, JsonElement> entry : sourceMapRoot.entrySet()) {
        if (entry.getKey().startsWith("x_")) {
          extensions.put(entry.getKey(), entry.getValue());
        }
      }
      builder.setExtensions(Collections.unmodifiableMap(extensions));

    } catch (JsonParseException ex) {
      return Outcome.err(new SourceMapParseException("JSON parse exception: " + ex));
    }

    return Outcome.ok(builder.build());
  }

  // TODO: fenghaolw - Migrate everyone to use this method and rename it to parse.
  public static Outcome<SourceMapObject> parseFast(String contents) {
    SourceMapObject.Builder builder = SourceMapObject.builder();
    builder.setLineCount(-1);

    // The catch below used to rewrap every exception of the parse, the parser's own
    // SourceMapParseException included; an error outcome is rewrapped in the same way.
    Outcome<Void> parsed;
    try {
      parsed = parseFastInto(builder, contents);
    } catch (Exception ex) {
      return Outcome.err(new SourceMapParseException("Parse exception: " + ex));
    }
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
          builder.setFile(file.get());
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
          builder.setMappings(mappings.get());
        }
        case "sourceRoot" -> {
          Outcome<@Nullable String> sourceRoot = lexer.nextStringOrNull();
          if (sourceRoot.isErr()) {
            return Outcome.err(sourceRoot.error());
          }
          builder.setSourceRoot(sourceRoot.get());
        }
        case "sections" -> {
          Outcome<String> rawSections = lexer.nextRawValue();
          if (rawSections.isErr()) {
            return Outcome.err(rawSections.error());
          }
          JsonArray sectionsArray = parseJsonArray(rawSections.get());
          ImmutableList.Builder<SourceMapSection> listBuilder = ImmutableList.builder();
          for (JsonElement each : sectionsArray) {
            Outcome<SourceMapSection> section = buildSection(each.getAsJsonObject());
            if (section.isErr()) {
              return Outcome.err(section.error());
            }
            listBuilder.add(section.get());
          }
          builder.setSections(listBuilder.build());
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
            extensions.put(name, parseJsonElement(rawValue.get()));
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

  private static Outcome<SourceMapSection> buildSection(JsonObject section) {
    JsonObject offset = section.get("offset").getAsJsonObject();
    int line = offset.get("line").getAsInt();
    int column = offset.get("column").getAsInt();

    if (section.has("map") && section.has("url")) {
      return Outcome.err(
          new SourceMapParseException(
              "Invalid map format: section may not have both 'map' and 'url'"));
    } else if (section.has("url")) {
      return Outcome.ok(SourceMapSection.forURL(section.get("url").getAsString(), line, column));
    } else if (section.has("map")) {
      JsonElement map = section.get("map");
      String mapStr =
          map.isJsonPrimitive() && map.getAsJsonPrimitive().isString()
              ? map.getAsString()
              : map.toString();
      return Outcome.ok(SourceMapSection.forMap(mapStr, line, column));
    }

    return Outcome.err(
        new SourceMapParseException(
            "Invalid map format: section must have either 'map' or 'url'"));
  }

  private static @Nullable String getStringOrNull(JsonObject object, String key) {
    return (object.has(key) && !object.get(key).isJsonNull())
        ? object.get(key).getAsString()
        : null;
  }

  private static String @Nullable [] getJavaStringArray(JsonElement element) {
    if (element == null || element instanceof JsonNull) {
      return null;
    }
    JsonArray array = element.getAsJsonArray();
    int len = array.size();
    String[] result = new String[len];
    for (int i = 0; i < len; i++) {
      JsonElement item = array.get(i);
      result[i] = item.isJsonNull() ? null : item.getAsString();
    }
    return result;
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

  /**
   * Parses a whole JSON document that must be an object. Returns null for an empty document.
   *
   * <p>The three {@code parseJson*} methods accept and reject exactly the documents that Gson's
   * {@code fromJson(String, Class)} did for the corresponding {@link JsonElement} class.
   */
  private static @Nullable JsonObject parseJsonObject(String json) {
    JsonReader reader = new JsonReader(new StringReader(json));
    JsonElement element = readJsonElement(reader);
    if (element != null && !(element instanceof JsonObject)) {
      throw unexpectedJsonElement("com.google.gson.JsonObject", element, reader);
    }
    checkFullyConsumed(element, reader);
    return (JsonObject) element;
  }

  /** Parses a whole JSON document that must be an array. Returns null for an empty document. */
  private static @Nullable JsonArray parseJsonArray(String json) {
    JsonReader reader = new JsonReader(new StringReader(json));
    JsonElement element = readJsonElement(reader);
    if (element != null && !(element instanceof JsonArray)) {
      throw unexpectedJsonElement("com.google.gson.JsonArray", element, reader);
    }
    checkFullyConsumed(element, reader);
    return (JsonArray) element;
  }

  /** Parses a whole JSON document of any kind. Returns null for an empty document. */
  private static @Nullable JsonElement parseJsonElement(String json) {
    JsonReader reader = new JsonReader(new StringReader(json));
    JsonElement element = readJsonElement(reader);
    checkFullyConsumed(element, reader);
    return element;
  }

  /** Reads the next value in lenient mode. Returns null if there is no value at all. */
  private static @Nullable JsonElement readJsonElement(JsonReader reader) {
    reader.setLenient(true);
    try {
      reader.peek();
    } catch (EOFException e) {
      return null;
    } catch (IOException e) {
      throw new JsonSyntaxException(e);
    }
    JsonElement element = JsonParser.parseReader(reader);
    reader.setLenient(false);
    return element;
  }

  private static JsonSyntaxException unexpectedJsonElement(
      String expectedClassName, JsonElement element, JsonReader reader) {
    return new JsonSyntaxException(
        "Expected a "
            + expectedClassName
            + " but was "
            + element.getClass().getName()
            + "; at path "
            + reader.getPreviousPath());
  }

  /** Rejects anything but whitespace after the value that was read; this check is strict. */
  private static void checkFullyConsumed(@Nullable JsonElement element, JsonReader reader) {
    try {
      if (element != null && reader.peek() != JsonToken.END_DOCUMENT) {
        throw new JsonSyntaxException("JSON document was not fully consumed.");
      }
    } catch (IOException e) {
      throw new JsonSyntaxException(e);
    }
  }

  private SourceMapObjectParser() {}
}
