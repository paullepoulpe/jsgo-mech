/*
 * Copyright 2020 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.instrumentation.reporter;

import static com.google.common.base.Preconditions.checkState;
import static com.google.javascript.jscomp.base.JavaRegex.DOT;
import static java.util.Comparator.comparing;

import com.google.common.base.Splitter;
import com.google.debugging.sourcemap.Base64VLQ;
import com.google.javascript.jscomp.base.Charsets;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Json;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.base.SimpleJsonException;
import com.google.javascript.jscomp.base.SimpleJsonValue;
import com.google.javascript.jscomp.instrumentation.reporter.proto.FileProfile;
import com.google.javascript.jscomp.instrumentation.reporter.proto.InstrumentationPoint;
import com.google.javascript.jscomp.instrumentation.reporter.proto.InstrumentationPointStats;
import com.google.javascript.jscomp.instrumentation.reporter.proto.InstrumentationPointStats.Presence;
import com.google.javascript.jscomp.instrumentation.reporter.proto.ReportProfile;
import com.google.re2j.Matcher;
import com.google.re2j.Pattern;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;

/**
 * Class that helps to decode reports received from JS binaries that have been instrumented. Report
 * is usually a map of String => Long. Where key is encoded instrumentation point and value is
 * number of times that code was executed. To decode report beside report itself we need mapping
 * file that was produced when JS binary was compiled.
 *
 * <p>Expected usage: <code>{@code
 * Map<String, InstrumentationPoint> mapping = ReportDecoder.parseMappingFromFile("mapping.txt");
 * Map<String, Long> encodedReport = readFile("report.txt");
 * ReportProfile report = ReportDecoder.decodeReport(mapping, encodedReport);
 * }
 * </code>
 */
public final class ReportDecoder {

  private ReportDecoder() {}

  public static Outcome<Map<String, InstrumentationPoint>> parseMappingFromFile(
      String mappingFilePath) {

    Outcome<String> fileContent = Io.readTextStrict(mappingFilePath, Charsets.UTF_8);
    if (fileContent.isErr()) {
      return Outcome.err(fileContent.error());
    }
    return ReportDecoder.parseMapping(fileContent.get());
  }

  /**
   * Parses the file found at location mappingFilePath and populates the properties of this class
   */
  public static Outcome<Map<String, InstrumentationPoint>> parseMapping(
      String mappingFileContent) {

    Map<String, InstrumentationPoint> mapping = new LinkedHashMap<>();

    List<String> linesOfFile = Splitter.on('\n').omitEmptyStrings().splitToList(mappingFileContent);

    // Report should contain at least three lines
    checkState(linesOfFile.size() >= 3, "Malformed report %s", linesOfFile);

    String fileNamesLine = linesOfFile.get(0).trim();
    String functionNamesLine = linesOfFile.get(1).trim();
    String typesLine = linesOfFile.get(2).trim();

    checkState(fileNamesLine.startsWith("FileNames:"));
    checkState(functionNamesLine.startsWith("FunctionNames:"));
    checkState(typesLine.startsWith("Types:"));

    String fileNamesAsJsonArray = fileNamesLine.substring(fileNamesLine.indexOf(":") + 1);
    String functionNamesAsJsonArray =
        functionNamesLine.substring(functionNamesLine.indexOf(":") + 1);
    String typesAsJsonArray = typesLine.substring(typesLine.indexOf(":") + 1);

    Outcome<List<String>> localFileNamesOutcome = parseJsonStringList(fileNamesAsJsonArray);
    if (localFileNamesOutcome.isErr()) {
      return Outcome.err(localFileNamesOutcome.error());
    }
    List<String> localFileNames = localFileNamesOutcome.get();
    Outcome<List<String>> functionNamesOutcome = parseJsonStringList(functionNamesAsJsonArray);
    if (functionNamesOutcome.isErr()) {
      return Outcome.err(functionNamesOutcome.error());
    }
    List<String> functionNames = functionNamesOutcome.get();

    Outcome<List<String>> typesAsStringList = parseJsonStringList(typesAsJsonArray);
    if (typesAsStringList.isErr()) {
      return Outcome.err(typesAsStringList.error());
    }
    List<InstrumentationPoint.Type> types = new ArrayList<>();
    for (String typeAsString : typesAsStringList.get()) {
      types.add(InstrumentationPoint.Type.valueOf(typeAsString));
    }

    for (int i = 3; i < linesOfFile.size(); ++i) {
      String lineItem = linesOfFile.get(i);
      String id = lineItem.substring(0, lineItem.indexOf(':'));
      String encodedDetails = lineItem.substring(lineItem.indexOf(':') + 1);

      StringCharIterator encodedDetailsAsCharIt = new StringCharIterator(encodedDetails);

      InstrumentationPoint temp =
          InstrumentationPoint.newBuilder()
              .setFileName(localFileNames.get(Base64VLQ.decode(encodedDetailsAsCharIt)))
              .setFunctionName(functionNames.get(Base64VLQ.decode(encodedDetailsAsCharIt)))
              .setType(types.get(Base64VLQ.decode(encodedDetailsAsCharIt)))
              .setLineNumber(Base64VLQ.decode(encodedDetailsAsCharIt))
              .setColumnNumber(Base64VLQ.decode(encodedDetailsAsCharIt))
              .build();
      mapping.putIfAbsent(id, temp);
    }

    return Outcome.ok(mapping);
  }

  /**
   * Parses a JSON document that must be an array of strings. A null element stays null, and a
   * boolean or a number is read as its text.
   */
  private static Outcome<List<String>> parseJsonStringList(String json) {
    Outcome<SimpleJsonValue> parsed = Json.parse(json);
    if (parsed.isErr()) {
      return Outcome.err(parsed.error());
    }
    if (parsed.get().kind() != SimpleJsonValue.Kind.ARRAY) {
      return Outcome.err(new SimpleJsonException("Malformed mapping: expected a JSON array"));
    }
    List<String> list = new ArrayList<>();
    for (SimpleJsonValue element : parsed.get().elements()) {
      if (element.kind() == SimpleJsonValue.Kind.NULL) {
        list.add(null);
        continue;
      }
      Outcome<String> string = element.asString();
      if (string.isErr()) {
        return Outcome.err(string.error());
      }
      list.add(string.get());
    }
    return Outcome.ok(list);
  }

  public static ReportProfile decodeReport(
      Map<String, InstrumentationPoint> mapping, Map<String, Long> frequencies) {
    // Decode each entry into InstrumentionPoint.
    List<InstrumentationPointStats> instrumentationPoints = new ArrayList<>();
    for (Map.Entry<String, InstrumentationPoint> entry : mapping.entrySet()) {
      instrumentationPoints.add(
          InstrumentationPointStats.newBuilder()
              .setPoint(entry.getValue())
              .setTimesExecuted(frequencies.getOrDefault(entry.getKey(), 0L))
              .build());
    }

    return ReportDecoder.createReportProfile(instrumentationPoints);
  }

  /**
   * This function builds a report for given JS binary analyzing its content. If an instrumentation
   * point is present in the JS binary - it will be marked as executed once. If the instrumentation
   * point was removed by the compiler as dead code - it will be marked as executed zero times. This
   * function doesn't give 100% guarantee for finding unused code but it provides good enough
   * approximation to be useful.
   *
   * @param mapping Parsed instrumentation mapping. Use {@link #parseMapping} function to create it.
   * @param fileContent Content of a JS binary compiled with production instrumentation enabled.
   * @param instrumentationArrayName Name of the array that was passed as
   *     --production_instrumentation_array_name flag when compiling JS binary.
   * @return Report containing all instrumentation points. If a point times_executed is zero - it
   *     means the function or branch was removed as dead code. If it's one - it's present in the JS
   *     binary.
   */
  public static ReportProfile createProfileOfStaticallyUsedCode(
      Map<String, InstrumentationPoint> mapping,
      String fileContent,
      String instrumentationArrayName) {
    Matcher matcher =
        Pattern.compile(instrumentationArrayName + "\\.push\\(['\"](" + DOT + "*?)['\"]\\)")
            .matcher(fileContent);
    final LinkedHashSet<String> encodedPoints = new LinkedHashSet<>();
    while (matcher.find()) {
      encodedPoints.add(matcher.group(1));
    }
    List<InstrumentationPointStats> instrumentationPoints = new ArrayList<>();
    for (Map.Entry<String, InstrumentationPoint> entry : mapping.entrySet()) {
      instrumentationPoints.add(
          InstrumentationPointStats.newBuilder()
              .setPoint(entry.getValue())
              .setTimesExecuted(0)
              .setPointPresence(
                  encodedPoints.contains(entry.getKey())
                      ? Presence.PRESENT
                      : Presence.STATICALLY_REMOVED)
              .build());
    }
    return ReportDecoder.createReportProfile(instrumentationPoints);
  }

  /**
   * Given list of profiles merges them, summing up execution counts for instrumentation points used
   * across multiple profiles. Be aware that instrumentation points are compared as equality so
   * profiles should come from JS binaries compiled with at the same commit, otherwise changes in
   * source code will cause differences in some instrumentation points.
   */
  public static ReportProfile mergeProfiles(List<ReportProfile> profiles) {
    Map<InstrumentationPoint, Long> frequencies = new LinkedHashMap<>();
    Map<InstrumentationPoint, Presence> presences = new LinkedHashMap<>();
    for (ReportProfile profile : profiles) {
      for (FileProfile fileProfile : profile.getFileProfileList()) {
        for (InstrumentationPointStats pointStats :
            fileProfile.getInstrumentationPointsStatsList()) {
          frequencies.compute(
              pointStats.getPoint(),
              (p, existing) -> (existing == null ? 0 : existing) + pointStats.getTimesExecuted());
          // Logic of merging presence field.
          // 1. PRESENT overrides STATICALLY_REMOVED and UNKNOWN.
          // 2. STATICALLY_REMOVED overrides UNKNOWN.
          presences.compute(
              pointStats.getPoint(),
              (p, existing) ->
                  (pointStats.getPointPresence() == Presence.PRESENT
                          || existing == Presence.PRESENCE_UNKNOWN
                          || existing == null)
                      ? pointStats.getPointPresence()
                      : existing);
        }
      }
    }
    List<InstrumentationPointStats> finalPoints = new ArrayList<>();
    for (Map.Entry<InstrumentationPoint, Long> entry : frequencies.entrySet()) {
      finalPoints.add(
          InstrumentationPointStats.newBuilder()
              .setPoint(entry.getKey())
              .setTimesExecuted(entry.getValue())
              .setPointPresence(presences.get(entry.getKey()))
              .build());
    }
    return ReportDecoder.createReportProfile(finalPoints);
  }

  /**
   * Groups instrumentation points by files and convert them to FileProfile object, one per file and
   * return as ReportProfile.
   */
  private static ReportProfile createReportProfile(
      List<InstrumentationPointStats> instrumentationPoints) {
    List<InstrumentationPointStats> sortedPoints = new ArrayList<>(instrumentationPoints);
    sortedPoints.sort(
        comparing(
                (InstrumentationPointStats pointStats) -> pointStats.getPoint().getFileName())
            .thenComparingInt(
                (InstrumentationPointStats pointStats) -> pointStats.getPoint().getLineNumber()));
    LinkedHashMap<String, List<InstrumentationPointStats>> pointsByFileName =
        new LinkedHashMap<>();
    for (InstrumentationPointStats pointStats : sortedPoints) {
      pointsByFileName
          .computeIfAbsent(pointStats.getPoint().getFileName(), (k) -> new ArrayList<>())
          .add(pointStats);
    }
    List<FileProfile> fileProfiles = new ArrayList<>();
    for (Map.Entry<String, List<InstrumentationPointStats>> entry : pointsByFileName.entrySet()) {
      fileProfiles.add(
          FileProfile.newBuilder()
              .setFileName(entry.getKey())
              .addAllInstrumentationPointsStats(entry.getValue())
              .build());
    }
    fileProfiles.sort(comparing(FileProfile::getFileName));

    return ReportProfile.newBuilder().addAllFileProfile(fileProfiles).build();

  }

  /**
   * A implementation of the Base64VLQ CharIterator used for decoding the mappings encoded in the
   * JSON string.
   */
  private static class StringCharIterator implements Base64VLQ.CharIterator {

    final String content;
    final int length;
    int current = 0;

    StringCharIterator(String content) {
      this.content = content;
      this.length = content.length();
    }

    @Override
    public char next() {
      int index = current;
      current++;
      return content.charAt(index);
    }

    @Override
    public boolean hasNext() {
      return current < length;
    }
  }
}
