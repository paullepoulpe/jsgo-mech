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

import org.jspecify.annotations.Nullable;

/**
 * A text sink: standard output or standard error, a file being written piecemeal, or nothing at
 * all. Text is encoded with the charset the sink was made with (a character the charset does not
 * have becomes '?') and handed to an {@link Io.Sink}. Never throws: a failed write is reported by
 * {@link #close}.
 *
 * <p>A file sink keeps up to {@value #BUFFER_SIZE} characters back until {@link #flush} or {@link
 * #close}; a standard output or standard error sink hands every piece of text on at once.
 */
public final class Output implements Appendable {
  private static final int BUFFER_SIZE = 8192;

  /** Where the bytes go; null for the sink that discards everything. */
  private final Io.@Nullable Sink sink;

  private final String charset;
  private final boolean buffered;
  private final StringBuilder pending = new StringBuilder();

  /** UTF-16 starts with a byte order mark: whether it has been written. */
  private boolean started = false;

  private Output(Io.@Nullable Sink sink, String charset, boolean buffered) {
    this.sink = sink;
    this.charset = Charsets.name(charset);
    this.buffered = buffered;
  }

  /** The process's standard output, as UTF-8, unbuffered. */
  public static Output stdout() {
    return new Output(Io.stdout(), Charsets.UTF_8, false);
  }

  /** The process's standard error, as UTF-8, unbuffered. */
  public static Output stderr() {
    return new Output(Io.stderr(), Charsets.UTF_8, false);
  }

  /** A sink that discards what is written to it. */
  public static Output discard() {
    return new Output(null, Charsets.UTF_8, false);
  }

  /** An unbuffered UTF-8 sink that writes to the byte sink. */
  public static Output to(Io.Sink sink) {
    return new Output(sink, Charsets.UTF_8, false);
  }

  /**
   * Opens (creating if needed) the file now, truncating it or, with {@code append}, keeping its
   * content, for buffered text in the charset.
   */
  public static Outcome<Output> open(String path, String charset, boolean append) {
    Outcome<Io.Sink> sink = Io.openFile(path, append);
    if (sink.isErr()) {
      return Outcome.err(sink.error());
    }
    return Outcome.ok(new Output(sink.get(), charset, true));
  }

  /**
   * A buffered sink that encodes with the charset and writes where this sink writes. Closing it
   * closes that destination, for this sink too.
   */
  public Output encodedAs(String charset) {
    return new Output(sink, charset, true);
  }

  public void print(String s) {
    pending.append(s);
    if (!buffered || pending.length() >= BUFFER_SIZE) {
      write(false);
    }
  }

  /** Prints the text and a line feed. */
  public void println(String s) {
    print(s + "\n");
  }

  /** Prints the throwable and its stack trace, as an uncaught exception is reported. */
  public void printStackTrace(Throwable t) {
    print(Io.stackTraceText(t));
  }

  @Override
  public Output append(@Nullable CharSequence s) {
    print(String.valueOf(s));
    return this;
  }

  @Override
  public Output append(@Nullable CharSequence s, int start, int end) {
    print(String.valueOf(s).substring(start, end));
    return this;
  }

  @Override
  public Output append(char c) {
    print(String.valueOf(c));
    return this;
  }

  public void flush() {
    write(false);
  }

  /** Writes what is still kept back and closes the destination; reports a failed write. */
  public Outcome<Void> close() {
    write(true);
    if (sink == null) {
      return Outcome.ok();
    }
    return sink.close();
  }

  /**
   * Hands the pending text on. The first half of a surrogate pair at the end waits for its second
   * half, unless this is the end of all the text.
   */
  private void write(boolean last) {
    int end = pending.length();
    if (!last && end > 0 && Character.isHighSurrogate(pending.charAt(end - 1))) {
      end--;
    }
    if (end == 0) {
      return;
    }
    String text = pending.substring(0, end);
    pending.delete(0, end);
    // Only the first piece of UTF-16 text starts with the byte order mark.
    String encoding = started && charset.equals(Charsets.UTF_16) ? Charsets.UTF_16BE : charset;
    started = true;
    if (sink != null) {
      sink.write(Charsets.encode(text, encoding));
    }
  }
}
