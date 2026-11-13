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


import com.google.errorprone.annotations.CanIgnoreReturnValue;
import com.google.errorprone.annotations.FormatMethod;
import com.google.errorprone.annotations.FormatString;
import com.google.errorprone.annotations.MustBeClosed;
import com.google.javascript.jscomp.base.Charsets;
import com.google.javascript.jscomp.base.FilePaths;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.base.SimpleJsonWriter;
import java.util.function.Supplier;

/**
 * An implementation that adapts an {@link Io.Output} and writes to a real file.
 *
 * <p>This class is designed to be super-sourceable by J2CL, since JS environments often don't have
 * file APIs. Doing so enables logging statements to be checked-in.
 */
final class WritingLogFile extends LogFile {

  @MustBeClosed
  static LogFile create(String file) {
    Io.mkdirs(FilePaths.parent(file).orElse(null));
    // The log allows the logged string to contain invalid Unicode (it replaces invalid character
    // sequences but doesn't throw). This is useful in cases where the source code contains invalid
    // Unicode (e.g., inside strings).
    Outcome<Io.Output> writer = Io.openOutput(file, Charsets.UTF_8, true);
    if (writer.isErr()) {
      throw new RuntimeException(writer.error());
    }
    return new WritingLogFile(writer.get());
  }

  private final Io.Output writer;

  private WritingLogFile(Io.Output writer) {
    this.writer = writer;
  }

  @Override
  public LogFile logObject(Object value) {
    return logInternal(value.toString());
  }

  @Override
  public LogFile logString(String value) {
    return logInternal(value);
  }

  @Override
  public LogFile log(Supplier<String> value) {
    return logInternal(value.get());
  }

  @Override
  @FormatMethod
  public LogFile log(@FormatString String template, Object... values) {
    return logInternal(String.format(template, values));
  }

  @Override
  public LogFile logJsonObject(Object value) {
    return logInternal(LogsGson.toJson(value));
  }

  @Override
  public LogFile logJsonSupplier(Supplier<Object> value) {
    return logInternal(LogsGson.toJson(value.get()));
  }

  @CanIgnoreReturnValue
  @Override
  public LogFile logJson(StreamedJsonProducer producer) {
    SimpleJsonWriter jsonWriter = new SimpleJsonWriter();
    // The JSON goes to the log piece by piece: some of these logs are too big to build in memory.
    jsonWriter.setSink(text -> this.writer.print(text));
    Outcome<Void> closed;
    try {
      producer.writeJson(jsonWriter);
    } finally {
      // The JSON writer always closed the log it wrote to, whatever it had written.
      this.writer.print(jsonWriter.takeText());
      closed = this.writer.close();
    }
    if (closed.isErr()) {
      throw new AssertionError(closed.error());
    }
    return this;
  }

  @CanIgnoreReturnValue
  private LogFile logInternal(String value) {
    // It's fine to pass a fully rendered string because we know we're going to use it by the
    // time this method is called.
    writer.append(value).append('\n');
    return this;
  }

  @Override
  public void close() {
    Outcome<Void> closed = writer.close();
    if (closed.isErr()) {
      throw new RuntimeException(closed.error());
    }
  }

  @Override
  public boolean isLogging() {
    return true;
  }
}
