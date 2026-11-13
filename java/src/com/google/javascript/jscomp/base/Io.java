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

import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.io.OutputStreamWriter;
import java.io.PrintStream;
import java.io.PrintWriter;
import java.io.StringReader;
import java.io.StringWriter;
import java.io.Writer;
import java.net.URI;
import java.net.URISyntaxException;
import java.nio.ByteBuffer;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.Charset;
import java.nio.charset.StandardCharsets;
import java.nio.file.FileSystems;
import java.nio.file.FileVisitOption;
import java.nio.file.FileVisitResult;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.PathMatcher;
import java.nio.file.SimpleFileVisitor;
import java.nio.file.attribute.BasicFileAttributes;
import java.nio.file.attribute.FileTime;
import java.util.ArrayList;
import java.util.EnumSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.zip.Deflater;
import java.util.zip.GZIPInputStream;
import java.util.zip.GZIPOutputStream;
import java.util.zip.ZipEntry;
import java.util.zip.ZipFile;
import java.util.zip.ZipInputStream;
import org.jspecify.annotations.Nullable;

/**
 * The compiler's only door to the outside world: every file-system, stream, zip, charset, classpath
 * resource and URI operation of the production code is a static method here, and this is the only
 * production file that uses java.io, java.nio.file, java.nio.charset, java.util.zip, java.net or
 * Guava's io package (tools/prep/14_find_io_uses checks that).
 *
 * <p>Operations work on whole values (a file's text, a zip entry's bytes) and name things by
 * {@code String}: paths, charset names, URIs. An operation that can fail for reasons outside the
 * program returns an {@link Outcome} carrying the {@link IOException}; nothing here throws for an
 * expected failure. The groups, in order: charsets; whole files and file facts; the text sinks
 * ({@link Output}) and the text source ({@link Input}); zip and gzip; classpath resources; path
 * and URI arithmetic on strings; in-memory helpers whose behaviour a JDK I/O class defines.
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

  // ---- Whole files and file facts

  /** The file's text; malformed input becomes U+FFFD. */
  public static Outcome<String> readText(String path, String charset) {
    Outcome<byte[]> bytes = readBytes(path);
    if (bytes.isErr()) {
      return Outcome.err(bytes.error());
    }
    return Outcome.ok(Charsets.decode(bytes.get(), charset));
  }

  /** The file's text; malformed input is an error for which {@link #isDecodingError} is true. */
  public static Outcome<String> readTextStrict(String path, String charset) {
    try {
      Optional<String> text = Charsets.decodeStrict(Files.readAllBytes(Path.of(path)), charset);
      if (text.isEmpty()) {
        return Outcome.err(new CharacterCodingException());
      }
      return Outcome.ok(text.get());
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  public static boolean isDecodingError(@Nullable Throwable error) {
    return error instanceof CharacterCodingException;
  }

  public static Outcome<byte[]> readBytes(String path) {
    try (InputStream in = new FileInputStream(path)) {
      return Outcome.ok(in.readAllBytes());
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /** Creates or truncates the file; unmappable characters become the charset's replacement. */
  public static Outcome<Void> writeText(String path, String charset, String text) {
    return writeBytes(path, Charsets.encode(text, charset));
  }

  /** Creates or truncates the file. */
  public static Outcome<Void> writeBytes(String path, byte[] bytes) {
    try (OutputStream out = new FileOutputStream(path)) {
      out.write(bytes);
      return Outcome.ok();
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  public static boolean exists(String path) {
    return new File(path).exists();
  }

  public static boolean isDirectory(String path) {
    return new File(path).isDirectory();
  }

  /** Creates the directory and its missing parents; false (no error) if it creates nothing. */
  public static boolean mkdirs(String path) {
    return new File(path).mkdirs();
  }

  /** Creates an empty file; ok(false) if the file exists already. */
  public static Outcome<Boolean> createNewFile(String path) {
    try {
      return Outcome.ok(new File(path).createNewFile());
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /** The paths of the directory's entries, in the file system's order; null if not a directory. */
  public static @Nullable List<String> listDirectory(String path) {
    File[] files = new File(path).listFiles();
    if (files == null) {
      return null;
    }
    List<String> result = new ArrayList<>();
    for (File f : files) {
      result.add(f.getPath());
    }
    return result;
  }

  /**
   * Walks the tree under {@code dir} (Files.walkFileTree order, following links, skipping what
   * cannot be visited) and returns every file whose path, or normalized path, matches the glob
   * (FileSystem.getPathMatcher("glob:...") syntax). The paths are as walked: they start with dir.
   */
  public static Outcome<List<String>> listFilesMatchingGlob(String dir, String glob) {
    PathMatcher matcher = FileSystems.getDefault().getPathMatcher("glob:" + glob);
    List<String> result = new ArrayList<>();
    try {
      Files.walkFileTree(
          Path.of(dir),
          EnumSet.of(FileVisitOption.FOLLOW_LINKS),
          Integer.MAX_VALUE,
          new SimpleFileVisitor<Path>() {
            @Override
            public FileVisitResult visitFile(Path p, BasicFileAttributes attrs) {
              if (matcher.matches(p) || matcher.matches(p.normalize())) {
                result.add(p.toString());
              }
              return FileVisitResult.CONTINUE;
            }

            @Override
            public FileVisitResult visitFileFailed(Path file, IOException e) {
              return FileVisitResult.SKIP_SUBTREE;
            }
          });
    } catch (IOException e) {
      return Outcome.err(e);
    }
    return Outcome.ok(result);
  }

  // ---- Text sinks and the text source

  /**
   * A text sink: standard output/error (or another byte stream a test supplies), or a file being
   * written piecemeal. Never throws: a write error is remembered and reported by {@link #close}.
   */
  public static final class Output implements Appendable {
    // Exactly one of stream and writer is non-null.
    private final @Nullable PrintStream stream;
    private final @Nullable Writer writer;
    private @Nullable IOException error;

    private Output(@Nullable PrintStream stream, @Nullable Writer writer) {
      this.stream = stream;
      this.writer = writer;
    }

    public void print(String s) {
      if (stream != null) {
        stream.print(s);
        return;
      }
      try {
        writer.write(String.valueOf(s));
      } catch (IOException e) {
        error = error == null ? e : error;
      }
    }

    /** Prints the text and the platform's line separator. */
    public void println(String s) {
      if (stream != null) {
        stream.println(s);
      } else {
        print(String.valueOf(s) + System.lineSeparator());
      }
    }

    /** Prints the throwable and its stack trace, as Throwable.printStackTrace does. */
    public void printStackTrace(Throwable t) {
      StringWriter trace = new StringWriter();
      t.printStackTrace(new PrintWriter(trace));
      print(trace.toString());
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
      if (stream != null) {
        stream.flush();
        return;
      }
      try {
        writer.flush();
      } catch (IOException e) {
        error = error == null ? e : error;
      }
    }

    /** Flushes and closes the sink (and what it writes to); reports the first write error. */
    public Outcome<Void> close() {
      if (stream != null) {
        stream.close();
      } else {
        try {
          writer.close();
        } catch (IOException e) {
          error = error == null ? e : error;
        }
      }
      return error == null ? Outcome.ok() : Outcome.err(error);
    }

    /**
     * A buffering sink that encodes text with the charset (unmappable characters become its
     * replacement) and writes the bytes where this stream sink writes its bytes.
     */
    public Output encodedAs(String charset) {
      return new Output(
          null, new BufferedWriter(new OutputStreamWriter(stream, Charset.forName(Charsets.name(charset)))));
    }
  }

  /** The process's standard output as it is now. */
  public static Output stdout() {
    return new Output(System.out, null);
  }

  /** The process's standard error as it is now. */
  public static Output stderr() {
    return new Output(System.err, null);
  }

  /** A sink that keeps what is written to it to itself. */
  public static Output nullOutput() {
    return new Output(null, new StringWriter());
  }

  /** For tests and embedders that hold a java.io stream: the sink that prints to it. */
  public static Output outputToStream(PrintStream stream) {
    return new Output(stream, null);
  }

  /**
   * Opens (creating if needed) the file now, truncating it or, with {@code append}, keeping its
   * content. Text is buffered and encoded with the charset, replacing what cannot be encoded.
   */
  public static Outcome<Output> openOutput(String path, String charset, boolean append) {
    try {
      return Outcome.ok(
          new Output(
              null,
              new BufferedWriter(
                  new OutputStreamWriter(
                      new FileOutputStream(path, append), Charset.forName(Charsets.name(charset))))));
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /** A byte source that is read once, whole: standard input (or a stream a test supplies). */
  public static final class Input {
    private final InputStream stream;

    private Input(InputStream stream) {
      this.stream = stream;
    }

    /** Everything up to end of input, decoded (malformed input becomes U+FFFD); not closed. */
    public Outcome<String> readText(String charset) {
      try {
        return Outcome.ok(Charsets.decode(stream.readAllBytes(), charset));
      } catch (IOException e) {
        return Outcome.err(e);
      }
    }
  }

  /** The process's standard input as it is now. */
  public static Input stdin() {
    return new Input(System.in);
  }

  /** For tests and embedders that hold a java.io stream: the source that reads it. */
  public static Input inputFromStream(InputStream stream) {
    return new Input(stream);
  }

  // ---- Zip and gzip

  /** One entry of a zip archive, read whole. */
  public record ZipEntryData(String name, byte[] data) {}

  /** The names of the zip file's entries, in file order, decoded with the charset. */
  public static Outcome<List<String>> listZipEntryNames(String zipPath, String charset) {
    List<String> names = new ArrayList<>();
    try (ZipInputStream in =
        new ZipInputStream(new FileInputStream(zipPath), Charset.forName(Charsets.name(charset)))) {
      for (ZipEntry entry = in.getNextEntry(); entry != null; entry = in.getNextEntry()) {
        names.add(entry.getName());
      }
    } catch (IOException e) {
      return Outcome.err(e);
    }
    return Outcome.ok(names);
  }

  /** Every entry of the zip bytes (directories too, with no data), in file order. */
  public static Outcome<List<ZipEntryData>> readZipEntries(byte[] zipBytes, String charset) {
    List<ZipEntryData> entries = new ArrayList<>();
    try (ZipInputStream in =
        new ZipInputStream(new ByteArrayInputStream(zipBytes), Charset.forName(Charsets.name(charset)))) {
      for (ZipEntry entry = in.getNextEntry(); entry != null; entry = in.getNextEntry()) {
        entries.add(new ZipEntryData(entry.getName(), in.readAllBytes()));
      }
    } catch (IOException e) {
      return Outcome.err(e);
    }
    return Outcome.ok(entries);
  }

  /**
   * The text of one entry of a zip file on disk; malformed input becomes U+FFFD. The open archive
   * is kept for the next call and reopened when the file's modification time changes. A missing
   * entry is a NullPointerException (the caller got the entry name from the same zip).
   */
  public static Outcome<String> readZipEntryText(String zipPath, String entryName, String charset) {
    synchronized (openZips) {
      try {
        Path path = Path.of(zipPath);
        FileTime lastModified = Files.getLastModifiedTime(path);
        OpenZip zip = openZips.get(zipPath);
        if (zip == null || !zip.lastModified.equals(lastModified)) {
          if (zip != null) {
            zip.file.close();
          }
          zip = new OpenZip(new ZipFile(path.toFile()), lastModified);
          openZips.put(zipPath, zip);
        }
        ZipEntry entry = zip.file.getEntry(entryName);
        if (entry == null) {
          throw new NullPointerException(path + "!/" + entryName);
        }
        try (InputStream in = zip.file.getInputStream(entry)) {
          return Outcome.ok(Charsets.decode(in.readAllBytes(), charset));
        }
      } catch (IOException e) {
        return Outcome.err(e);
      }
    }
  }

  private record OpenZip(ZipFile file, FileTime lastModified) {}

  private static final Map<String, OpenZip> openZips = new LinkedHashMap<>();

  /** Gzips at the fastest compression level. */
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

  public static Outcome<byte[]> gunzip(byte[] data) {
    try (GZIPInputStream in = new GZIPInputStream(new ByteArrayInputStream(data))) {
      return Outcome.ok(in.readAllBytes());
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  // ---- Classpath resources

  public static boolean resourceExists(Class<?> relativeTo, String name) {
    return relativeTo.getResource(name) != null;
  }

  /** The resource's bytes; ok(null) if there is no such resource. */
  public static Outcome<byte[]> readResource(Class<?> relativeTo, String name) {
    try (InputStream in = relativeTo.getResourceAsStream(name)) {
      return Outcome.ok(in == null ? null : in.readAllBytes());
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  // ---- Path arithmetic on strings (a syntactically invalid path is an InvalidPathException)

  /** The name separator of paths: "/" or "\\". */
  public static String separator() {
    return File.separator;
  }

  /** The path in its normal spelling: redundant separators removed. */
  public static String pathString(String first, String... more) {
    return Path.of(first, more).toString();
  }

  /** The last name of the path; "" if it has none. */
  public static String pathFileName(String path) {
    Path name = Path.of(path).getFileName();
    return name == null ? "" : name.toString();
  }

  /** The path without its last name; null if that leaves nothing. */
  public static Optional<String> pathParent(String path) {
    Path parent = Path.of(path).getParent();
    return parent == null ? Optional.empty() : Optional.ofNullable(parent.toString());
  }

  /** The path with "." and "name/.." elements removed. */
  public static String pathNormalize(String path) {
    return Path.of(path).normalize().toString();
  }

  /** The path resolved against the working directory if it is relative. */
  public static String pathToAbsolute(String path) {
    return Path.of(path).toAbsolutePath().toString();
  }

  /** {@code other} if it is absolute, else {@code path/other}. */
  public static String pathResolve(String path, String other) {
    return Path.of(path).resolve(other).toString();
  }

  /** The relative path from {@code path} to {@code other}. */
  public static String pathRelativize(String path, String other) {
    return Path.of(path).relativize(Path.of(other)).toString();
  }

  // ---- URI arithmetic on strings (java.net.URI semantics)

  /** ok(uri) if it is a syntactically valid URI, else the URISyntaxException. */
  public static Outcome<String> uriParse(String uri) {
    try {
      return Outcome.ok(new URI(uri).toString());
    } catch (URISyntaxException e) {
      return Outcome.err(e);
    }
  }

  /** The scheme of a valid URI; null if it has none (it is not absolute). */
  public static Optional<String> uriScheme(String uri) {
    return Optional.ofNullable(URI.create(uri).getScheme());
  }

  /** The decoded path of a valid URI; null if it is opaque. */
  public static Optional<String> uriPath(String uri) {
    return Optional.ofNullable(URI.create(uri).getPath());
  }

  // ---- In-memory helpers

  /**
   * The lines of the text as BufferedReader.readLine returns them: terminated by \n, \r or \r\n,
   * terminators dropped, no empty last line after a final terminator.
   */
  public static List<String> splitLines(String text) {
    List<String> lines = new ArrayList<>();
    BufferedReader reader = new BufferedReader(new StringReader(text));
    try {
      for (Optional<String> line = Optional.ofNullable(reader.readLine()); line.isPresent(); line = Optional.ofNullable(reader.readLine())) {
        lines.add(line.orElse(null));
      }
    } catch (IOException e) {
      throw new AssertionError(e); // reading from memory
    }
    return lines;
  }

  /** Standard base64 with padding. */
  public static String base64Encode(byte[] bytes) {
    return com.google.common.io.BaseEncoding.base64().encode(bytes);
  }

  /** Decodes standard base64 (padding optional); IllegalArgumentException if it is not base64. */
  public static byte[] base64Decode(String text) {
    return com.google.common.io.BaseEncoding.base64().decode(text);
  }
}
