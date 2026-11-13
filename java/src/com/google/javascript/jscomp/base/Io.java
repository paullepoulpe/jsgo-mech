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

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.io.PrintStream;
import java.io.PrintWriter;
import java.io.StringWriter;
import java.nio.ByteBuffer;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.TimeUnit;
import java.util.zip.DataFormatException;
import java.util.zip.Deflater;
import java.util.zip.GZIPInputStream;
import java.util.zip.GZIPOutputStream;
import java.util.zip.Inflater;

/**
 * The compiler's only door to the outside world, and the one production class that is written by
 * hand for every platform instead of being translated: this is the only production file that
 * uses java.io, java.nio, java.util.zip, java.net or Guava's io package (tools/prep/14_find_io_uses
 * checks that).
 *
 * <p>It holds only what needs the operating system (files, directories, the standard streams,
 * bundled resources) and the codecs every platform's library has (UTF-8, DEFLATE, gzip, base64).
 * Everything built on top is ordinary code next to this class: {@link Charsets}, {@link
 * TextFiles}, {@link Output}, {@link FilePaths}, {@link Glob}, {@link ZipArchive}, {@link Uris}.
 *
 * <p>The comment of each method is its whole contract; no behaviour of a JDK class is implied.
 * Paths, names and texts are {@code String}s, data is {@code byte[]}. An operation that can fail
 * for reasons outside the program returns an {@link Outcome} whose error is a plain {@link
 * IOException} with exactly the message its comment gives, so that every platform reports the
 * same text; nothing here throws for such a failure.
 */
public final class Io {
  private Io() {}

  // ---- UTF-8 (the other charsets are spelled out in Charsets)

  /** The UTF-8 bytes of the text; half a surrogate pair on its own becomes '?'. */
  public static byte[] utf8Bytes(String s) {
    return s.getBytes(StandardCharsets.UTF_8);
  }

  /**
   * The text of the UTF-8 bytes. Every byte that is not part of a well-formed sequence (the
   * shortest encoding of a code point up to U+10FFFF that is not a surrogate) becomes one U+FFFD.
   */
  public static String utf8String(byte[] bytes) {
    String text = new String(bytes, StandardCharsets.UTF_8);
    if (text.indexOf('\uFFFD') < 0 || utf8StringStrict(bytes).isPresent()) {
      return text; // the bytes are well formed
    }
    StringBuilder repaired = new StringBuilder(bytes.length);
    int i = 0;
    while (i < bytes.length) {
      int length = utf8SequenceLength(bytes, i);
      if (length == 0) {
        repaired.append('\uFFFD');
        i++;
      } else {
        repaired.append(new String(bytes, i, length, StandardCharsets.UTF_8));
        i += length;
      }
    }
    return repaired.toString();
  }

  /** The length of the well-formed sequence that starts at bytes[i]; 0 if there is none. */
  private static int utf8SequenceLength(byte[] bytes, int i) {
    int lead = bytes[i] & 0xFF;
    int length = lead < 0x80 ? 1 : lead < 0xC2 ? 0 : lead < 0xE0 ? 2 : lead < 0xF0 ? 3 : lead < 0xF5 ? 4 : 0;
    if (length == 0 || i + length > bytes.length) {
      return 0;
    }
    for (int k = 1; k < length; k++) {
      int b = bytes[i + k] & 0xFF;
      int low = k > 1 ? 0x80 : lead == 0xE0 ? 0xA0 : lead == 0xF0 ? 0x90 : 0x80;
      int high = k > 1 ? 0xBF : lead == 0xED ? 0x9F : lead == 0xF4 ? 0x8F : 0xBF;
      if (b < low || b > high) {
        return 0;
      }
    }
    return length;
  }

  /** The text of the UTF-8 bytes; empty if a byte is not part of a well-formed sequence. */
  public static Optional<String> utf8StringStrict(byte[] bytes) {
    try {
      return Optional.of(
          StandardCharsets.UTF_8.newDecoder().decode(ByteBuffer.wrap(bytes)).toString());
    } catch (CharacterCodingException e) {
      return Optional.empty();
    }
  }

  // ---- Files and directories

  /**
   * The content of the file. Errors: "(path): No such file" if nothing has that path, "(path):
   * Cannot read file" for every other failure (a directory, no permission, a failed read).
   */
  public static Outcome<byte[]> readBytes(String path) {
    if (!new File(path).exists()) {
      return Outcome.err(new IOException(path + ": No such file"));
    }
    try (InputStream in = new FileInputStream(path)) {
      return Outcome.ok(in.readAllBytes());
    } catch (IOException e) {
      return Outcome.err(new IOException(path + ": Cannot read file"));
    }
  }

  /**
   * Makes the bytes the content of the file, creating it if needed (its directory must exist).
   * Error: "(path): Cannot write file".
   */
  public static Outcome<Void> writeBytes(String path, byte[] bytes) {
    try (OutputStream out = new FileOutputStream(path)) {
      out.write(bytes);
      return Outcome.ok();
    } catch (IOException e) {
      return Outcome.err(new IOException(path + ": Cannot write file"));
    }
  }

  /** Whether the path names something: a file, a directory, or a link to one. */
  public static boolean exists(String path) {
    return new File(path).exists();
  }

  /** Whether the path names a directory, or a link to one. */
  public static boolean isDirectory(String path) {
    return new File(path).isDirectory();
  }

  /**
   * When the file was last modified, in nanoseconds since 1970 as far as the file system records
   * them; 0 if there is no such file.
   */
  public static long lastModified(String path) {
    try {
      return Files.getLastModifiedTime(Path.of(path)).to(TimeUnit.NANOSECONDS);
    } catch (IOException e) {
      return 0;
    }
  }

  /**
   * Creates the directory and its missing parents. A failure is not reported: writing the file
   * that was to go there fails instead.
   */
  public static void mkdirs(String path) {
    new File(path).mkdirs();
  }

  /**
   * The files under the directory that are not directories, each as its path relative to the
   * directory (names joined by "/"). Depth first: the entries of a directory in the order of their
   * names (as strings compare), the files under a subdirectory at the place of its name. Symbolic
   * links are followed; a directory that is a link back to itself or to a directory around it is
   * not entered, and neither is one that cannot be listed. Empty if the path is not a directory.
   */
  public static List<String> listFiles(String directory) {
    List<String> result = new ArrayList<>();
    listFilesInto(new File(directory.isEmpty() ? "." : directory), "", new ArrayList<>(), result);
    return result;
  }

  private static void listFilesInto(
      File directory, String prefix, List<String> around, List<String> result) {
    String[] names = directory.list();
    String canonical;
    try {
      canonical = directory.getCanonicalPath();
    } catch (IOException e) {
      return;
    }
    if (names == null || around.contains(canonical)) {
      return;
    }
    Arrays.sort(names);
    around.add(canonical);
    for (String name : names) {
      File entry = new File(directory, name);
      if (entry.isDirectory()) {
        listFilesInto(entry, prefix + name + "/", around, result);
      } else {
        result.add(prefix + name);
      }
    }
    around.remove(around.size() - 1);
  }

  /** The absolute path of the process's working directory. */
  public static String workingDirectory() {
    return Path.of("").toAbsolutePath().toString();
  }

  // ---- Byte sinks (a file being written, standard output, standard error) and standard input

  /**
   * Somewhere bytes go, piece by piece. Nothing is buffered: each {@link #write} hands its bytes
   * to the operating system. Never throws.
   */
  public static final class Sink {
    private final OutputStream stream;
    // The path of a file; empty for a standard stream, whose failures are ignored.
    private final Optional<String> path;
    private boolean closed = false;
    private boolean failed = false;

    private Sink(OutputStream stream, Optional<String> path) {
      this.stream = stream;
      this.path = path;
    }

    /** Writes the bytes. Ignored after {@link #close}; a failure is remembered for close. */
    public void write(byte[] bytes) {
      if (closed) {
        return;
      }
      try {
        stream.write(bytes, 0, bytes.length);
        if (path.isEmpty()) {
          stream.flush();
        }
      } catch (IOException e) {
        failed = true;
      }
    }

    /**
     * Closes the file or stream; a second close does nothing and succeeds. Error, for a file, if
     * a write or this close failed: "(path): Cannot write file". Closing standard output or
     * standard error always succeeds and makes every later write to that stream, through any
     * sink, do nothing.
     */
    public Outcome<Void> close() {
      if (closed) {
        return Outcome.ok();
      }
      closed = true;
      try {
        stream.close();
      } catch (IOException e) {
        failed = true;
      }
      if (failed && path.isPresent()) {
        return Outcome.err(new IOException(path.get() + ": Cannot write file"));
      }
      return Outcome.ok();
    }
  }

  /** The process's standard output. A failed write to it is ignored. */
  public static Sink stdout() {
    return new Sink(System.out, Optional.empty());
  }

  /** The process's standard error. A failed write to it is ignored. */
  public static Sink stderr() {
    return new Sink(System.err, Optional.empty());
  }

  /**
   * Opens the file for writing now, creating it if needed (its directory must exist) and
   * emptying it or, with {@code append}, keeping its content and adding to it. Error: "(path):
   * Cannot write file".
   */
  public static Outcome<Sink> openFile(String path, boolean append) {
    try {
      return Outcome.ok(new Sink(new FileOutputStream(path, append), Optional.of(path)));
    } catch (IOException e) {
      return Outcome.err(new IOException(path + ": Cannot write file"));
    }
  }

  /** A byte source that is read once, whole: standard input. */
  public static final class Input {
    private final InputStream stream;

    private Input(InputStream stream) {
      this.stream = stream;
    }

    /** Everything up to the end of the input. Error: "stdin: Cannot read file". */
    public Outcome<byte[]> readBytes() {
      try {
        return Outcome.ok(stream.readAllBytes());
      } catch (IOException e) {
        return Outcome.err(new IOException("stdin: Cannot read file"));
      }
    }
  }

  /** The process's standard input. */
  public static Input stdin() {
    return new Input(System.in);
  }

  // ---- Codecs

  /**
   * The data of a raw DEFLATE stream (RFC 1951), which is what a zip entry of method 8 holds.
   * Bytes after the end of the stream are ignored. Error, for a malformed or truncated stream:
   * "Malformed compressed data".
   */
  public static Outcome<byte[]> inflate(byte[] deflated) {
    Inflater inflater = new Inflater(true);
    inflater.setInput(deflated);
    ByteArrayOutputStream data = new ByteArrayOutputStream();
    byte[] buffer = new byte[8192];
    try {
      while (!inflater.finished()) {
        int n = inflater.inflate(buffer);
        if (n == 0 && !inflater.finished()) {
          // More input or a preset dictionary is needed.
          return Outcome.err(new IOException("Malformed compressed data"));
        }
        data.write(buffer, 0, n);
      }
    } catch (DataFormatException e) {
      return Outcome.err(new IOException("Malformed compressed data"));
    } finally {
      inflater.end();
    }
    return Outcome.ok(data.toByteArray());
  }

  /**
   * A gzip file (RFC 1952) with one member that holds the data. Which bytes exactly is up to the
   * platform (compression is quick rather than thorough); {@link #gunzip} gives the data back.
   */
  public static byte[] gzip(byte[] data) {
    ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    try (GZIPOutputStream out =
        new GZIPOutputStream(bytes) {
          {
            def.setLevel(Deflater.BEST_SPEED);
          }
        }) {
      out.write(data);
    } catch (IOException e) {
      throw new AssertionError(e); // writing to memory
    }
    return bytes.toByteArray();
  }

  /**
   * The data of a gzip file (RFC 1952) with one member. Error, if the bytes are not that (a wrong
   * header, malformed or truncated compressed data, a wrong checksum or length): "Malformed gzip
   * data". What bytes after the member lead to is not defined.
   */
  public static Outcome<byte[]> gunzip(byte[] gzipped) {
    try (GZIPInputStream in = new GZIPInputStream(new ByteArrayInputStream(gzipped))) {
      return Outcome.ok(in.readAllBytes());
    } catch (IOException e) {
      return Outcome.err(new IOException("Malformed gzip data"));
    }
  }

  /** Standard base64 (RFC 4648, alphabet A-Z a-z 0-9 + /) with "=" padding. */
  public static String base64Encode(byte[] bytes) {
    return com.google.common.io.BaseEncoding.base64().encode(bytes);
  }

  /**
   * The bytes of standard base64 text, with or without its "=" padding;
   * IllegalArgumentException if the text is anything else.
   */
  public static byte[] base64Decode(String text) {
    return com.google.common.io.BaseEncoding.base64().decode(text);
  }

  // ---- Files bundled with the compiler; the report of an unexpected failure

  /**
   * The content of a file that is bundled with the compiler (externs.zip, the runtime libraries),
   * by its full name with "/" between the parts and none in front, such as
   * "com/google/javascript/jscomp/js/polyfills.txt"; empty if there is no such file.
   */
  public static Optional<byte[]> readResource(String name) {
    try (InputStream in = Io.class.getClassLoader().getResourceAsStream(name)) {
      return in == null ? Optional.empty() : Optional.of(in.readAllBytes());
    } catch (IOException e) {
      return Optional.empty();
    }
  }

  /**
   * The text a platform prints for an uncaught failure: the throwable, where it was thrown, and
   * the same for its causes; it ends with a line feed. The details are the platform's.
   */
  public static String stackTraceText(Throwable t) {
    StringWriter trace = new StringWriter();
    t.printStackTrace(new PrintWriter(trace));
    return trace.toString();
  }

  // ---- For tests and embedders that hold java.io streams (Java only)

  /** An unbuffered UTF-8 text sink that prints to the stream; closing it closes the stream. */
  public static Output outputToStream(PrintStream stream) {
    return Output.to(new Sink(stream, Optional.empty()));
  }

  /** A source that reads the stream. */
  public static Input inputFromStream(InputStream stream) {
    return new Input(stream);
  }
}
