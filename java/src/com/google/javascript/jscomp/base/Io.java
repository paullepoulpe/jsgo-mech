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
import java.nio.charset.CharacterCodingException;
import java.nio.charset.Charset;
import java.nio.charset.CharsetEncoder;
import java.nio.charset.CodingErrorAction;
import java.nio.charset.StandardCharsets;
import java.nio.file.FileSystems;
import java.nio.file.FileVisitOption;
import java.nio.file.FileVisitResult;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.PathMatcher;
import java.nio.file.SimpleFileVisitor;
import java.nio.file.StandardOpenOption;
import java.nio.file.attribute.BasicFileAttributes;
import java.nio.file.attribute.FileTime;
import java.util.ArrayList;
import java.util.EnumSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
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
 * expected failure.
 *
 * <ul>
 *   <li><b>Charsets</b> ({@link #UTF_8}, {@link #isCharsetSupported}, {@link #charsetName}, {@link
 *       #utf8Bytes}, {@link #utf8String}, {@link #newEncoder}): charsets are named, never passed
 *       as objects.
 *   <li><b>Whole files</b> ({@link #readText}, {@link #readTextStrict}, {@link #readBytes}, {@link
 *       #writeText}, {@link #writeBytes}) and <b>file facts</b> ({@link #exists}, {@link
 *       #isDirectory}, {@link #createDirectories}, {@link #fileMkdirs}, {@link #createNewFile},
 *       {@link #delete}, {@link #listDirectory}, {@link #listFilesMatchingGlob}).
 *   <li><b>Text sinks</b> ({@link Output}: {@link #stdout}, {@link #stderr}, {@link #openOutput},
 *       {@link #openLog}) for output that is produced piecemeal, and the one <b>text source</b>
 *       that is not a file ({@link Input}: {@link #stdin}).
 *   <li><b>Archives and compression</b> ({@link #listZipEntryNames}, {@link #readZipEntryText},
 *       {@link #readZipEntries}, {@link #gzipFast}, {@link #gzipSize}, {@link #gunzip}, {@link
 *       #hasGzipHeader}).
 *   <li><b>Classpath resources</b> ({@link #readResource}, {@link #resourceExists}).
 *   <li><b>Path arithmetic</b> on strings, with the semantics of java.io.File ({@code file...})
 *       or of java.nio.file.Path ({@code path...}), whichever the caller always had.
 *   <li><b>URI arithmetic</b> on strings with the semantics of java.net.URI ({@code uri...}).
 *   <li><b>In-memory helpers</b> whose exact behaviour is defined by a JDK I/O class ({@link
 *       #splitLines}, {@link #base64Encode}, {@link #base64Decode}).
 * </ul>
 */
public final class Io {
  private Io() {}

  // ---------------------------------------------------------------------------------------------
  // Charsets

  public static final String UTF_8 = "UTF-8";
  public static final String US_ASCII = "US-ASCII";

  /** Like Charset.isSupported: false for unknown names, IllegalArgumentException for illegal. */
  public static boolean isCharsetSupported(String charset) {
    return Charset.isSupported(charset);
  }

  /** The canonical name of the charset (an alias or differently-cased name is accepted). */
  public static String charsetName(String charset) {
    return Charset.forName(charset).name();
  }

  /** String.getBytes(UTF_8): unpaired surrogates become '?'. */
  public static byte[] utf8Bytes(String s) {
    return s.getBytes(StandardCharsets.UTF_8);
  }

  /** new String(bytes, UTF_8): malformed input becomes U+FFFD. */
  public static String utf8String(byte[] bytes) {
    return new String(bytes, StandardCharsets.UTF_8);
  }

  /** Answers "can this charset represent this char" (CharsetEncoder.canEncode). Not thread-safe. */
  public static final class CharEncoder {
    private final CharsetEncoder encoder;

    private CharEncoder(CharsetEncoder encoder) {
      this.encoder = encoder;
    }

    public boolean canEncode(char c) {
      return encoder.canEncode(c);
    }
  }

  public static CharEncoder newEncoder(String charset) {
    return new CharEncoder(Charset.forName(charset).newEncoder());
  }

  // ---------------------------------------------------------------------------------------------
  // Whole files

  /** The file's text; malformed input is replaced by U+FFFD (an InputStreamReader's behaviour). */
  public static Outcome<String> readText(String path, String charset) {
    try (InputStream in = new FileInputStream(path)) {
      return Outcome.ok(new String(in.readAllBytes(), Charset.forName(charset)));
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /** As {@link #readText} with the platform's default charset (a FileReader's behaviour). */
  public static Outcome<String> readTextDefaultCharset(String path) {
    return readText(path, Charset.defaultCharset().name());
  }

  /**
   * The file's text; malformed input is an error for which {@link #isDecodingError} is true
   * (Files.readString's behaviour).
   */
  public static Outcome<String> readTextStrict(String path, String charset) {
    try {
      return Outcome.ok(Files.readString(Path.of(path), Charset.forName(charset)));
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
  public static Outcome<Boolean> writeText(String path, String charset, String text) {
    return writeBytes(path, text.getBytes(Charset.forName(charset)));
  }

  /**
   * Creates or truncates the file; an unmappable character is an error for which {@link
   * #isDecodingError} is true and leaves the file as it was (Files.writeString's behaviour).
   */
  public static Outcome<Boolean> writeTextStrict(String path, String charset, String text) {
    try {
      Files.writeString(Path.of(path), text, Charset.forName(charset));
      return Outcome.ok(true);
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  public static Outcome<Boolean> writeBytes(String path, byte[] bytes) {
    try (OutputStream out = new FileOutputStream(path)) {
      out.write(bytes);
      return Outcome.ok(true);
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /** As {@link #writeBytes}, opening the file with Files.newOutputStream. */
  public static Outcome<Boolean> writeBytesNio(String path, byte[] bytes) {
    try (OutputStream out = Files.newOutputStream(Path.of(path))) {
      out.write(bytes);
      return Outcome.ok(true);
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  // ---------------------------------------------------------------------------------------------
  // File facts

  public static boolean exists(String path) {
    return new File(path).exists();
  }

  public static boolean isDirectory(String path) {
    return new File(path).isDirectory();
  }

  /** Files.createDirectories. */
  public static Outcome<Boolean> createDirectories(String path) {
    try {
      Files.createDirectories(Path.of(path));
      return Outcome.ok(true);
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /** File.mkdirs: no error reporting, false if nothing was created. */
  public static boolean fileMkdirs(String path) {
    return new File(path).mkdirs();
  }

  /** File.createNewFile: ok(false) if the file exists already. */
  public static Outcome<Boolean> createNewFile(String path) {
    try {
      return Outcome.ok(new File(path).createNewFile());
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /** File.delete. */
  public static boolean delete(String path) {
    return new File(path).delete();
  }

  /** File.listFiles as paths, in the order the file system lists them; null if not a directory. */
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
   * (FileSystem.getPathMatcher("glob:...") syntax). Paths are returned as walked: they start with
   * {@code dir}.
   */
  public static Outcome<List<String>> listFilesMatchingGlob(String dir, String glob) {
    PathMatcher matcher = FileSystems.getDefault().getPathMatcher("glob:" + glob);
    List<String> result = new ArrayList<>();
    try {
      Files.walkFileTree(
          FileSystems.getDefault().getPath(dir),
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

  // ---------------------------------------------------------------------------------------------
  // Text sinks and the text source

  /**
   * A text sink: standard output/error (or another byte stream a test supplies), or a file being
   * written piecemeal. Never throws: a write error is remembered and reported by {@link #close}.
   */
  public static final class Output implements Appendable {
    // Exactly one of these is non-null.
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
        fail(e);
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

    public void println() {
      if (stream != null) {
        stream.println();
      } else {
        print(System.lineSeparator());
      }
    }

    /** Throwable.printStackTrace to this sink. */
    public void printStackTrace(Throwable t) {
      if (stream != null) {
        t.printStackTrace(stream);
      } else {
        PrintWriter printer = new PrintWriter(writer);
        t.printStackTrace(printer);
        printer.flush();
      }
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
        fail(e);
      }
    }

    /** The first error of a write to this sink so far, or null. */
    public @Nullable IOException firstError() {
      return error;
    }

    /** Flushes and closes the sink (and what it writes to); reports the first write error. */
    public Outcome<Boolean> close() {
      if (stream != null) {
        stream.close();
      } else {
        try {
          writer.close();
        } catch (IOException e) {
          fail(e);
        }
      }
      return error == null ? Outcome.ok(true) : Outcome.err(error);
    }

    /**
     * A sink that encodes text with the given charset (unmappable characters become the charset's
     * replacement) and writes the bytes where this stream sink writes its bytes. It buffers:
     * flush or close it.
     */
    public Output encodedAs(String charset) {
      return new Output(
          null, new BufferedWriter(new OutputStreamWriter(stream, Charset.forName(charset))));
    }

    private void fail(IOException e) {
      if (error == null) {
        error = e;
      }
    }
  }

  /** The process's standard output as it is now (System.out). */
  public static Output stdout() {
    return new Output(System.out, null);
  }

  /** The process's standard error as it is now (System.err). */
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
   * Creates or truncates the file now; text is encoded with the platform's default charset and
   * write errors are not reported (new PrintStream(path)).
   */
  public static Outcome<Output> openPrintFile(String path) {
    try {
      return Outcome.ok(new Output(new PrintStream(path), null));
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /** As {@link #openPrintFile}, opening the file with Files.newOutputStream. */
  public static Outcome<Output> openPrintFileNio(String path) {
    try {
      return Outcome.ok(new Output(new PrintStream(Files.newOutputStream(Path.of(path))), null));
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /** Creates or truncates the file now; text is encoded with the charset, buffered. */
  public static Outcome<Output> openOutput(String path, String charset) {
    try {
      return Outcome.ok(
          new Output(
              null,
              new BufferedWriter(
                  new OutputStreamWriter(new FileOutputStream(path), Charset.forName(charset)))));
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  /**
   * Opens the file for appending UTF-8 text (creating it if needed); text that is not valid
   * UTF-16 is written with replacement characters.
   */
  public static Outcome<Output> openLog(String path) {
    try {
      CharsetEncoder encoder =
          StandardCharsets.UTF_8
              .newEncoder()
              .onMalformedInput(CodingErrorAction.REPLACE)
              .onUnmappableCharacter(CodingErrorAction.REPLACE);
      return Outcome.ok(
          new Output(
              null,
              new BufferedWriter(
                  new OutputStreamWriter(
                      Files.newOutputStream(
                          Path.of(path),
                          StandardOpenOption.CREATE,
                          StandardOpenOption.APPEND,
                          StandardOpenOption.WRITE),
                      encoder))));
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
        return Outcome.ok(new String(stream.readAllBytes(), Charset.forName(charset)));
      } catch (IOException e) {
        return Outcome.err(e);
      }
    }

    public Outcome<Boolean> close() {
      try {
        stream.close();
        return Outcome.ok(true);
      } catch (IOException e) {
        return Outcome.err(e);
      }
    }
  }

  /** The process's standard input as it is now (System.in). */
  public static Input stdin() {
    return new Input(System.in);
  }

  /** For tests and embedders that hold a java.io stream. */
  public static Input inputFromStream(InputStream stream) {
    return new Input(stream);
  }

  // ---------------------------------------------------------------------------------------------
  // Archives and compression

  /** One entry of a zip archive, read whole. */
  public record ZipEntryData(String name, byte[] data) {}

  /**
   * The names of the entries of the zip file, in file order (a sequential ZipInputStream scan,
   * entry names decoded with the charset).
   */
  public static Outcome<List<String>> listZipEntryNames(String zipPath, String charset) {
    try (InputStream input = new FileInputStream(zipPath)) {
      return listZipEntryNamesOf(input, charset);
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  private static Outcome<List<String>> listZipEntryNamesOf(InputStream input, String charset)
      throws IOException {
    List<String> names = new ArrayList<>();
    try (ZipInputStream in = new ZipInputStream(input, Charset.forName(charset))) {
      for (ZipEntry entry = in.getNextEntry(); entry != null; entry = in.getNextEntry()) {
        names.add(entry.getName());
      }
    }
    return Outcome.ok(names);
  }

  /**
   * Every entry of the zip bytes (directories included, with no data), in file order; entry names
   * are decoded with the charset.
   */
  public static Outcome<List<ZipEntryData>> readZipEntries(byte[] zipBytes, String charset) {
    List<ZipEntryData> entries = new ArrayList<>();
    try (ZipInputStream in =
        new ZipInputStream(new ByteArrayInputStream(zipBytes), Charset.forName(charset))) {
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
   * is kept for the next call and reopened when the file's modification time changes, so a
   * long-lived process sees a rewritten zip. A missing entry is a NullPointerException (a bug in
   * the caller, which got the entry name from the same zip).
   */
  public static Outcome<String> readZipEntryText(String zipPath, String entryName, String charset) {
    OpenZip zip;
    synchronized (openZips) {
      zip = openZips.get(zipPath);
      if (zip == null) {
        zip = new OpenZip();
        openZips.put(zipPath, zip);
      }
    }
    try {
      return Outcome.ok(new String(zip.read(zipPath, entryName), Charset.forName(charset)));
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  private static final int ZIP_CACHE_SIZE =
      Integer.parseInt(System.getProperty("jscomp.zipfile.cachesize", "1000"));

  // Least-recently-used first; an archive is closed when it is evicted.
  private static final Map<String, OpenZip> openZips =
      new LinkedHashMap<String, OpenZip>(16, 0.75f, true) {
        @Override
        protected boolean removeEldestEntry(Map.Entry<String, OpenZip> eldest) {
          if (size() <= ZIP_CACHE_SIZE) {
            return false;
          }
          eldest.getValue().close();
          return true;
        }
      };

  private static final class OpenZip {
    private @Nullable ZipFile zipFile;
    private @Nullable FileTime lastModified;

    synchronized byte[] read(String zipPath, String entryName) throws IOException {
      Path path = Path.of(zipPath);
      FileTime newLastModified = Files.getLastModifiedTime(path);
      if (!newLastModified.equals(lastModified)) {
        close();
        zipFile = new ZipFile(path.toFile());
        lastModified = newLastModified;
      }
      ZipEntry entry = zipFile.getEntry(entryName);
      if (entry == null) {
        throw new NullPointerException(path + "!/" + entryName);
      }
      try (InputStream in = zipFile.getInputStream(entry)) {
        return in.readAllBytes();
      }
    }

    synchronized void close() {
      if (zipFile != null) {
        try {
          zipFile.close();
        } catch (IOException e) {
          throw new RuntimeException(e);
        }
        zipFile = null;
        lastModified = null;
      }
    }
  }

  /** GZIPOutputStream at Deflater.BEST_SPEED. */
  public static byte[] gzipFast(byte[] data) {
    return gzipAtLevel(data, Deflater.BEST_SPEED);
  }

  private static byte[] gzipAtLevel(byte[] data, int level) {
    ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    try (GZIPOutputStream out =
        new GZIPOutputStream(bytes) {
          {
            def.setLevel(level);
          }
        }) {
      out.write(data);
    } catch (IOException e) {
      throw new AssertionError(e); // writing to memory
    }
    return bytes.toByteArray();
  }

  /** The length of the data gzipped (GZIPOutputStream at the default compression level). */
  public static int gzipSize(byte[] data) {
    return gzipAtLevel(data, Deflater.DEFAULT_COMPRESSION).length;
  }

  /** Whether the data starts with a gzip header that GZIPInputStream accepts. */
  public static boolean hasGzipHeader(byte[] data) {
    try (GZIPInputStream in = new GZIPInputStream(new ByteArrayInputStream(data))) {
      return true;
    } catch (IOException e) {
      return false;
    }
  }

  public static Outcome<byte[]> gunzip(byte[] data) {
    try (GZIPInputStream in = new GZIPInputStream(new ByteArrayInputStream(data))) {
      return Outcome.ok(in.readAllBytes());
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  // ---------------------------------------------------------------------------------------------
  // Classpath resources

  /** Class.getResource(name) != null. */
  public static boolean resourceExists(Class<?> relativeTo, String name) {
    return relativeTo.getResource(name) != null;
  }

  /** Class.getResourceAsStream(name), read whole; ok(null) if there is no such resource. */
  public static Outcome<byte[]> readResource(Class<?> relativeTo, String name) {
    try (InputStream in = relativeTo.getResourceAsStream(name)) {
      if (in == null) {
        return Outcome.ok(null);
      }
      return Outcome.ok(in.readAllBytes());
    } catch (IOException e) {
      return Outcome.err(e);
    }
  }

  // ---------------------------------------------------------------------------------------------
  // Path arithmetic: "file..." has java.io.File semantics, "path..." java.nio.file.Path semantics.
  // A syntactically invalid path is an InvalidPathException, as it always was.

  /** File.separator. */
  public static String fileSeparator() {
    return File.separator;
  }

  /** new File(path).getPath(): the path in java.io.File's normal spelling. */
  public static String filePath(String path) {
    return new File(path).getPath();
  }

  /** new File(path).getName(). */
  public static String fileName(String path) {
    return new File(path).getName();
  }

  /** new File(path).getParent(); null if the path has no parent. */
  public static @Nullable String fileParent(String path) {
    return new File(path).getParent();
  }

  /** new File(path).getAbsolutePath(). */
  public static String fileAbsolutePath(String path) {
    return new File(path).getAbsolutePath();
  }

  /** new File(parent, child).toString(). */
  public static String fileChild(String parent, String child) {
    return new File(parent, child).toString();
  }

  /** Path.of(first, more).toString(): the path in the file system's normal spelling. */
  public static String pathString(String first, String... more) {
    return Path.of(first, more).toString();
  }

  /** Path.getParent; null if the path has no parent. */
  public static @Nullable String pathParent(String path) {
    Path parent = Path.of(path).getParent();
    return parent == null ? null : parent.toString();
  }

  /** Path.normalize: "." and "name/.." elements removed. */
  public static String pathNormalize(String path) {
    return Path.of(path).normalize().toString();
  }

  /** Path.toAbsolutePath. */
  public static String pathToAbsolute(String path) {
    return Path.of(path).toAbsolutePath().toString();
  }

  /** Path.resolve: {@code other} if it is absolute, else {@code path/other}. */
  public static String pathResolve(String path, String other) {
    return Path.of(path).resolve(other).toString();
  }

  /** Path.resolveSibling: {@code other} resolved against the parent of {@code path}. */
  public static String pathResolveSibling(String path, String other) {
    return Path.of(path).resolveSibling(other).toString();
  }

  /** Path.relativize: the relative path from {@code path} to {@code other}. */
  public static String pathRelativize(String path, String other) {
    return Path.of(path).relativize(Path.of(other)).toString();
  }

  // ---------------------------------------------------------------------------------------------
  // URI arithmetic (java.net.URI semantics)

  /** new URI(uri): ok(uri) if it is a syntactically valid URI, else the URISyntaxException. */
  public static Outcome<String> uriParse(String uri) {
    try {
      return Outcome.ok(new URI(uri).toString());
    } catch (URISyntaxException e) {
      return Outcome.err(e);
    }
  }

  /** URI.isAbsolute (has a scheme) of a valid URI. */
  public static boolean uriIsAbsolute(String uri) {
    return URI.create(uri).isAbsolute();
  }

  /** URI.getScheme of a valid URI; null if it has none. */
  public static @Nullable String uriScheme(String uri) {
    return URI.create(uri).getScheme();
  }

  /** URI.getPath (decoded) of a valid URI; null if it is opaque. */
  public static @Nullable String uriPath(String uri) {
    return URI.create(uri).getPath();
  }

  /** URI.equals of two valid URIs. */
  public static boolean uriEquals(String uri1, String uri2) {
    return URI.create(uri1).equals(URI.create(uri2));
  }

  /** URI.hashCode of a valid URI. */
  public static int uriHashCode(String uri) {
    return URI.create(uri).hashCode();
  }

  // ---------------------------------------------------------------------------------------------
  // In-memory helpers

  /**
   * The lines of the text as BufferedReader.readLine returns them: terminated by \n, \r or \r\n,
   * terminators dropped, no empty last line after a final terminator.
   */
  public static List<String> splitLines(String text) {
    List<String> lines = new ArrayList<>();
    BufferedReader reader = new BufferedReader(new StringReader(text));
    try {
      for (String line = reader.readLine(); line != null; line = reader.readLine()) {
        lines.add(line);
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
