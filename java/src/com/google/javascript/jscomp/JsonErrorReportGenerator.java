/*
 * Copyright 2017 The Closure Compiler Authors.
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

import static java.lang.Math.max;
import static java.lang.Math.min;

import com.google.debugging.sourcemap.proto.Mapping.OriginalMapping;
import com.google.javascript.jscomp.LightweightMessageFormatter.LineNumberingFormatter;
import com.google.javascript.jscomp.SortingErrorManager.ErrorReportGenerator;
import com.google.javascript.jscomp.SortingErrorManager.ErrorWithLevel;
import com.google.javascript.jscomp.SourceExcerptProvider.SourceExcerpt;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.SimpleJsonWriter;
import com.google.javascript.rhino.Node;
import com.google.javascript.rhino.TokenUtil;
import java.util.Optional;

/**
 * An error report generator that prints error and warning data to the print stream as an array of
 * JSON objects.
 */
public class JsonErrorReportGenerator implements ErrorReportGenerator {
  private final Io.Output stream;
  private final SourceExcerptProvider sourceExcerptProvider;
  private static final LineNumberingFormatter excerptFormatter = new LineNumberingFormatter();

  /**
   * Creates an error manager.
   *
   * @param stream the stream on which the errors and warnings should be printed. This class does
   *     not close the stream
   * @param sourceExcerptProvider used to retrieve the source context which generated the error
   */
  public JsonErrorReportGenerator(Io.Output stream, SourceExcerptProvider sourceExcerptProvider) {
    this.stream = stream;
    this.sourceExcerptProvider = sourceExcerptProvider;
  }

  @Override
  public void generateReport(SortingErrorManager manager) {
    SimpleJsonWriter jsonWriter = new SimpleJsonWriter();
    {
      jsonWriter.beginArray();
      for (ErrorWithLevel message : manager.getSortedDiagnostics()) {
        Optional<String> sourceName = message.error.sourceName();
        int lineNumber = message.error.getLineNumber();
        int charno = message.error.charno();

        jsonWriter.beginObject();
        jsonWriter
            .name("level")
            .stringValue(message.level == CheckLevel.ERROR ? Optional.of("error") : Optional.of("warning"));
        jsonWriter.name("description").stringValue(Optional.of(message.error.description()));
        jsonWriter.name("key").stringValue(Optional.of(message.error.type().key));
        if (message.error.requirement() != null) {
          jsonWriter.name("requirement").beginObject();
          jsonWriter.name("ruleId").stringValue(Optional.ofNullable(message.error.requirement().getRuleId()));
          jsonWriter.name("configFiles").beginArray();
          for (String configFile : message.error.requirement().getConfigFileList()) {
            jsonWriter.stringValue(Optional.of(configFile));
          }
          jsonWriter.endArray();
          jsonWriter.endObject();
        }
        jsonWriter.name("source").stringValue(sourceName);
        jsonWriter.name("line").longValue(lineNumber);
        jsonWriter.name("column").longValue(charno);
        Node node = message.error.node();
        int regionLength = message.error.length();
        if (node != null && regionLength > 0) {
          jsonWriter.name("length").longValue(message.error.length());
        }

        // extract source excerpt
        Optional<String> sourceExcerpt =
            SourceExcerpt.LINE.get(sourceExcerptProvider, sourceName, lineNumber, excerptFormatter);
        if (sourceExcerpt.isPresent()) {
          StringBuilder b = new StringBuilder(sourceExcerpt.get());
          b.append("\n");

          // padding equal to the excerpt and arrow at the end
          // charno == sourceExcerpt.length() means something is missing
          // at the end of the line
          if (0 <= charno && charno <= sourceExcerpt.get().length()) {
            for (int i = 0; i < charno; i++) {
              char c = sourceExcerpt.get().charAt(i);
              if (TokenUtil.isWhitespace(c)) {
                b.append(c);
              } else {
                b.append(' ');
              }
            }
            if (node == null) {
              b.append("^");
            } else {
              int length = max(1, min(regionLength, sourceExcerpt.get().length() - charno));
              for (int i = 0; i < length; i++) {
                b.append("^");
              }
            }
          }

          jsonWriter.name("context").stringValue(Optional.of(b.toString()));
        }

        OriginalMapping mapping =
            sourceExcerptProvider.getSourceMapping(
                sourceName, message.error.getLineNumber(), message.error.charno());

        if (mapping != null) {
          jsonWriter.name("originalLocation").beginObject();
          jsonWriter.name("source").stringValue(Optional.ofNullable(mapping.getOriginalFile()));
          jsonWriter.name("line").longValue(mapping.getLineNumber());
          jsonWriter.name("column").longValue(mapping.getColumnPosition());
          jsonWriter.endObject();
        }

        jsonWriter.endObject();
      }

      StringBuilder summaryBuilder = new StringBuilder();
      if (manager.getTypedPercent() > 0.0) {
        summaryBuilder.append(
            String.format(
                "%d error(s), %d warning(s), %.1f%% typed",
                manager.getErrorCount(), manager.getWarningCount(), manager.getTypedPercent()));
      } else {
        summaryBuilder.append(
            String.format(
                "%d error(s), %d warning(s)", manager.getErrorCount(), manager.getWarningCount()));
      }
      jsonWriter.beginObject();
      jsonWriter.name("level").stringValue(Optional.of("info"));
      jsonWriter.name("description").stringValue(Optional.of(summaryBuilder.toString()));
      jsonWriter.endObject();

      jsonWriter.endArray();
    }
    // The text as it comes back from the UTF-8 bytes it used to be written as.
    stream.append(Io.utf8String(Io.utf8Bytes(jsonWriter.toString())));
    stream.flush();
  }
}
