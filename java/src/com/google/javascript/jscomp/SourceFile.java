/*
 * Copyright 2009 The Closure Compiler Authors.
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

import static com.google.common.base.Preconditions.checkArgument;
import static com.google.common.base.Preconditions.checkState;
import static com.google.common.base.Strings.isNullOrEmpty;
import static java.lang.Math.max;
import static java.lang.Math.min;

import com.google.errorprone.annotations.CanIgnoreReturnValue;
import com.google.errorprone.annotations.InlineMe;
import com.google.javascript.jscomp.base.Charsets;
import com.google.javascript.jscomp.base.FilePaths;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.serialization.SourceFileProto;
import com.google.javascript.jscomp.serialization.SourceFileProto.FileOnDisk;
import com.google.javascript.jscomp.serialization.SourceFileProto.ZipEntryOnDisk;
import com.google.javascript.rhino.StaticSourceFile;
import java.io.IOException;
import java.io.Serializable;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.function.Supplier;
import org.jspecify.annotations.Nullable;

/**
 * An abstract representation of a source file that provides access to language-neutral features.
 *
 * <p>The source file can be loaded from various locations, such as from disk or from a preloaded
 * string. Loading is done as lazily as possible to minimize IO delays and memory cost of source
 * text.
 */
public final class SourceFile implements StaticSourceFile {
  private static final String UTF8_BOM = "\uFEFF";

  private static final String BANG_SLASH = "!" + Platform.getFileSeperator();

  /** Number of lines in the region returned by {@link #getRegion(int)}. This length must be odd. */
  private static final int SOURCE_EXCERPT_REGION_LENGTH = 5;

  /**
   * The file name of the source file.
   *
   * <p>It does not necessarily need to correspond to a real path. But it should be unique. Will
   * appear in warning messages emitted by the compiler.
   */
  private final String fileName;

  private SourceKind kind;

  private final CodeLoader loader;

  private boolean isStubSourceFileForAlreadyProvidedInputField = false;

  // Source Line Information
  private volatile int @Nullable [] lineOffsets = null;

  private volatile Optional<String> code = Optional.empty();

  // Code statistics used for metrics.
  // For both of these, a negative value means we haven't counted yet.
  private volatile int numLines = -1;
  private volatile int numBytes = -1;

  private SourceFile(CodeLoader loader, String fileName, SourceKind kind) {
    if (isNullOrEmpty(fileName)) {
      throw new IllegalArgumentException("a source must have a name");
    }

    if (!"/".equals(Platform.getFileSeperator())) {
      fileName = fileName.replace(Platform.getFileSeperator(), "/");
    }

    this.loader = loader;
    this.fileName = fileName;
    this.kind = kind;
  }

  public boolean isStubSourceFileForAlreadyProvidedInput() {
    return isStubSourceFileForAlreadyProvidedInputField;
  }

  private void setIsStubSourceFileForAlreadyProvidedInput(
      boolean isStubSourceFileForAlreadyProvidedInput) {
    this.isStubSourceFileForAlreadyProvidedInputField = isStubSourceFileForAlreadyProvidedInput;
  }

  @Override
  public int getLineOffset(int lineno) {
    findLineOffsets();
    if (lineno < 1 || lineno > lineOffsets.length) {
      throw new IllegalArgumentException(
          "Expected line number between 1 and " + lineOffsets.length + "\nActual: " + lineno);
    }
    return lineOffsets[lineno - 1];
  }

  /**
   * Whether {@link #getLineOffset} accepts the line number; it throws IllegalArgumentException
   * for any other.
   */
  boolean isValidLineNumber(int lineno) {
    findLineOffsets();
    return lineno >= 1 && lineno <= lineOffsets.length;
  }

  /**
   * Returns the number of lines in the source file.
   *
   * <p>NOTE: this may be stale if the SourceFile has been {@link #clearCachedSource cleared} and
   * the file has changed on disk. If being fully accurate is required call {@link #getCode} and
   * then call this method.
   */
  int getNumLines() {
    if (numLines < 0) {
      // A negative value means we need to read in the code and calculate this information.
      // Otherwise, assume the file hasn't changed since we read it.
      if (readCode().isErr()) {
        // this is consistent with old behavior of this method.
        return 0;
      }
    }
    return numLines;
  }

  /**
   * Returns the number of bytes in the source file
   *
   * <p>Oddly, bytes is defined as 'length of code as a Java String' this is accurate assuming files
   * only use Latin1 characters but may be inaccurate outside of that.
   *
   * <p>NOTE: this may be stale if the SourceFile has been {@link #clearCachedSource cleared} and
   * the file has changed on disk. If being fully accurate is required call {@link #getCode} and
   * then call this method.
   */
  int getNumBytes() {
    if (numBytes < 0) {
      // A negative value means we need to read in the code and calculate this information.
      // Otherwise, assume the file hasn't changed since we read it.
      if (readCode().isErr()) {
        // this is consistent with old behavior of this method.
        return 0;
      }
    }
    return numBytes;
  }

  static boolean isLineTerminator(char ch) {
    return switch (ch) {
      case '\n', // Line Feed
          '\r', // Carriage Return
          '\u2028', // Line Separator
          '\u2029' -> // Paragraph Separator
          true;
      default -> false;
    };
  }

  private void findLineOffsets() {
    if (this.lineOffsets != null) {
      return;
    }

    Optional<String> localCode = this.code;
    if (localCode.isEmpty()) {
      Outcome<String> codeRes = this.readCode();
      if (codeRes.isErr()) {
        localCode = Optional.of("");
        this.lineOffsets = new int[1];
        return;
      }
      localCode = Optional.ofNullable(codeRes.get());
    }

    // getCode() updates numLines, so this is always in sync with the code.
    int[] offsets = new int[this.numLines];
    int index = 1; // start at 1 since the offset for line 0 is always at byte 0
    for (int i = 0; i < localCode.get().length(); i++) {
      char ch = localCode.get().charAt(i);
      if (isLineTerminator(ch)) {
        if (ch == '\r' && i + 1 < localCode.get().length() && localCode.get().charAt(i + 1) == '\n') {
          i++;
        }
        offsets[index] = i + 1;
        index++;
      }
    }
    checkState(index == offsets.length);
    this.lineOffsets = offsets;
  }

  /** Gets all the code in this source file; a failure to read it is an error result. */
  public final Outcome<String> readCode() {
    if (this.code.isEmpty()) {
      // Only synchronize if we need to
      synchronized (this) {
        // Make sure another thread hasn't loaded the code while we waited.
        if (this.code.isEmpty()) {
          Outcome<String> loaded = this.loader.loadUncachedCode();
          if (loaded.isErr()) {
            return Outcome.err(loaded.error());
          }
          this.setCodeAndDoBookkeeping(Optional.ofNullable(loaded.get()));
        }
      }
    }

    return Outcome.ok(this.code.orElse(null));
  }

  @Deprecated
  final void setCodeDeprecated(String code) {
    this.setCodeAndDoBookkeeping(Optional.of(code));
  }

  /**
   * Gets the code in this source file for a single pass over it: if the code is not in memory and
   * comes from a file, the file's text is returned as it is (a byte order mark is not removed) and
   * is not kept in this object.
   */
  public Outcome<String> readCodeWithoutCaching() {
    // Only synchronize if we need to
    if (this.code.isEmpty()) {
      synchronized (this) {
        // Make sure another thread hasn't loaded the code while we waited.
        if (this.code.isEmpty()) {
          Outcome<String> uncachedText = this.loader.loadUncachedText();
          if (uncachedText != null) {
            return uncachedText;
          }
        }
      }
    }

    return this.readCode();
  }

  private void setCodeAndDoBookkeeping(Optional<String> sourceCode) {
    this.code = Optional.empty();
    // Force recalculation of all of these values when they are requested.
    this.lineOffsets = null;

    if (sourceCode.isPresent()) {
      if (sourceCode.get().startsWith(UTF8_BOM)) {
        sourceCode = Optional.of(sourceCode.get().substring(UTF8_BOM.length()));
      }

      // Update numLines and numBytes
      // NOTE: In some edit/refresh development workflows, we may end up re-reading the file
      //       and getting different numbers than we got last time we read it, so we should
      //       not have checkState() calls here to assert they have not changed.
      // Misleading variable name.  This really stores the 'number of utf16' code points which is
      // not the same as number of bytes.
      this.numBytes = sourceCode.get().length();
      int numLines = 1; // there is always at least one line
      for (int i = 0; i < sourceCode.get().length(); i++) {
        char ch = sourceCode.get().charAt(i);
        if (isLineTerminator(ch)) {
          if (ch == '\r' && i + 1 < sourceCode.get().length() && sourceCode.get().charAt(i + 1) == '\n') {
            i++;
          }
          numLines++;
        }
      }
      this.numLines = numLines;
      // Published last: a reader that sees the code also sees its statistics.
      this.code = sourceCode;
    }
  }

  /**
   * @deprecated alias of {@link #getName()}. Use that instead
   */
  @InlineMe(replacement = "this.getName()")
  @Deprecated
  public String getOriginalPath() {
    return this.getName();
  }

  /**
   * For SourceFile types which cache source code that can be regenerated easily, flush the cache.
   *
   * <p>We maintain the cache mostly to speed up generating source when displaying error messages,
   * so dumping the file contents after the compile is a fine thing to do.
   */
  public void clearCachedSource() {
    this.setCodeAndDoBookkeeping(Optional.empty());
  }

  boolean hasSourceInMemory() {
    return code.isPresent();
  }

  /**
   * Returns a unique name for the source file.
   *
   * <p>This name is not required to be an actual file path on disk.
   */
  @Override
  public String getName() {
    return fileName;
  }

  /** Returns the source kind. */
  @Override
  public SourceKind getKind() {
    return kind;
  }

  /** Sets the source kind. */
  public void setKind(SourceKind kind) {
    this.kind = kind;
  }

  private boolean isClosureUnawareCodeField = false;

  @Override
  public boolean isClosureUnawareCode() {
    return isClosureUnawareCodeField;
  }

  public void markAsClosureUnawareCode() {
    this.isClosureUnawareCodeField = true;
  }

  @Override
  public int getLineOfOffset(int offset) {
    checkArgument(offset >= 0, "offset must be non-negative: %s", offset);
    findLineOffsets();
    int search = Arrays.binarySearch(lineOffsets, offset);
    if (search >= 0) {
      return search + 1; // lines are 1-based.
    } else {
      int insertionPoint = -1 * (search + 1);
      return min(insertionPoint - 1, lineOffsets.length - 1) + 1;
    }
  }

  @Override
  public int getColumnOfOffset(int offset) {
    int line = getLineOfOffset(offset);
    return offset - lineOffsets[line - 1];
  }

  /**
   * Gets the source line for the indicated line number.
   *
   * @param lineNumber the line number, 1 being the first line of the file.
   * @return The line indicated. Does not include the newline at the end of the file. Returns {@code
   *     null} if it does not exist, or if there was an IO exception.
   */
  public Optional<String> getLine(int lineNumber) {
    Outcome<String> codeRes = readCode();
    if (codeRes.isErr()) {
      return Optional.empty();
    }
    String js = codeRes.get();
    findLineOffsets();
    if (lineNumber > lineOffsets.length) {
      return Optional.empty();
    }

    if (lineNumber < 1) {
      lineNumber = 1;
    }

    int pos = lineOffsets[lineNumber - 1];

    if (lineNumber < lineOffsets.length) {
      int nextPos = lineOffsets[lineNumber];
      int endPos = nextPos - 1;
      if (js.charAt(endPos) == '\n' && endPos > pos && js.charAt(endPos - 1) == '\r') {
        endPos--;
      }
      return Optional.of(js.substring(pos, endPos));
    } else {
      // If next new line cannot be found, there are two cases
      // 1. pos already reaches the end of file, then null should be returned
      // 2. otherwise, return the contents between pos and the end of file.
      if (pos >= js.length()) {
        return Optional.empty();
      } else {
        return Optional.of(js.substring(pos));
      }
    }
  }

  /**
   * Gets the source lines starting at `lineNumber` and continuing until `length`. Omits any
   * trailing newlines.
   *
   * @param lineNumber the line number, 1 being the first line of the file.
   * @param length the number of characters desired, starting at the 0th character of the specified
   *     line. If negative or 0, returns a single line.
   * @return The line(s) indicated. Returns {@code null} if it does not exist or if there was an IO
   *     exception.
   */
  public @Nullable Region getLines(int lineNumber, int length) {
    Outcome<String> codeRes = readCode();
    if (codeRes.isErr()) {
      return null;
    }
    String js = codeRes.get();
    findLineOffsets();
    if (lineNumber > lineOffsets.length) {
      return null;
    }

    if (lineNumber < 1) {
      lineNumber = 1;
    }
    if (length <= 0) {
      length = 1;
    }

    int pos = lineOffsets[lineNumber - 1];
    if (pos == js.length()) {
      return new SimpleRegion(
          lineNumber, lineNumber, ""); // Happens when asking for the last empty line in a file.
    }
    int endChar = pos;
    int endLine = lineNumber;
    // go through lines until we've reached the end of the file or met the specified length.
    for (; endChar < pos + length && endLine <= lineOffsets.length; endLine++) {
      endChar = (endLine < lineOffsets.length) ? lineOffsets[endLine] : js.length();
    }

    if (endChar > pos) {
      if (js.charAt(endChar - 1) == '\n') {
        if (endChar - 1 > pos && js.charAt(endChar - 2) == '\r') {
          return new SimpleRegion(lineNumber, endLine, js.substring(pos, endChar - 2));
        }
        return new SimpleRegion(lineNumber, endLine, js.substring(pos, endChar - 1));
      } else if (isLineTerminator(js.charAt(endChar - 1))) {
        return new SimpleRegion(lineNumber, endLine, js.substring(pos, endChar - 1));
      }
    }
    return new SimpleRegion(lineNumber, endLine, js.substring(pos, endChar));
  }

  /**
   * Get a region around the indicated line number. The exact definition of a region is
   * implementation specific, but it must contain the line indicated by the line number. A region
   * must not start or end by a carriage return.
   *
   * @param lineNumber the line number, 1 being the first line of the file.
   * @return The line indicated. Returns {@code null} if it does not exist, or if there was an IO
   *     exception.
   */
  public @Nullable Region getRegion(int lineNumber) {
    Outcome<String> codeRes = readCode();
    if (codeRes.isErr()) {
      return null;
    }
    String js = codeRes.get();
    int pos = 0;
    int startLine = max(1, lineNumber - (SOURCE_EXCERPT_REGION_LENGTH + 1) / 2 + 1);
    for (int n = 1; n < startLine; n++) {
      int nextpos = js.indexOf('\n', pos);
      if (nextpos == -1) {
        break;
      }
      pos = nextpos + 1;
    }
    int end = pos;
    int endLine = startLine;
    for (int n = 0; n < SOURCE_EXCERPT_REGION_LENGTH; n++, endLine++) {
      end = js.indexOf('\n', end);
      if (end == -1) {
        break;
      }
      end++;
    }
    if (lineNumber >= endLine) {
      return null;
    }
    if (end == -1) {
      int last = js.length() - 1;
      if (js.charAt(last) == '\n') {
        return new SimpleRegion(startLine, endLine, js.substring(pos, last));
      } else {
        return new SimpleRegion(startLine, endLine, js.substring(pos));
      }
    } else {
      return new SimpleRegion(startLine, endLine, js.substring(pos, end));
    }
  }

  @Override
  public String toString() {
    return fileName;
  }

  public static Outcome<List<SourceFile>> fromZipFile(String zipName, String inputCharset) {
    return fromZipEntryNames(zipName, Io.listZipEntryNames(zipName, inputCharset), inputCharset);
  }

  public static Outcome<List<SourceFile>> fromZipInput(
      String zipName, byte[] zipBytes, String inputCharset) {
    Outcome<List<Io.ZipEntryData>> entries = Io.readZipEntries(zipBytes, inputCharset);
    if (entries.isErr()) {
      return Outcome.err(entries.error());
    }
    List<String> entryNames = new ArrayList<>();
    for (Io.ZipEntryData entry : entries.get()) {
      entryNames.add(entry.name());
    }
    return fromZipEntryNames(zipName, Outcome.ok(entryNames), inputCharset);
  }

  private static Outcome<List<SourceFile>> fromZipEntryNames(
      String zipName, Outcome<List<String>> entryNames, String inputCharset) {
    if (entryNames.isErr()) {
      return Outcome.err(entryNames.error());
    }
    final String absoluteZipPath = FilePaths.toAbsolute(zipName);
    List<SourceFile> sourceFiles = new ArrayList<>();

    for (String entryName : entryNames.get()) {
      if (!entryName.endsWith(".js")) { // Only accept js files
        continue;
      }
      sourceFiles.add(
          builder()
              .withCharset(inputCharset)
              .withOriginalPath(zipName + BANG_SLASH + entryName)
              .withZipEntryPath(absoluteZipPath, entryName)
              .build());
    }
    return Outcome.ok(sourceFiles);
  }

  public static SourceFile fromFile(String fileName, String charset) {
    return builder().withPath(fileName).withCharset(charset).build();
  }

  public static SourceFile fromFile(String fileName) {
    return builder().withPath(fileName).build();
  }

  public static SourceFile fromCode(String fileName, String code, SourceKind kind) {
    return builder().withPath(fileName).withKind(kind).withContent(code).build();
  }

  public static SourceFile fromCode(String fileName, String code) {
    return builder().withPath(fileName).withContent(code).build();
  }

  /**
   * Returns a stub SourceFile with the given name and kind. It is used when the actual sources are
   * provided via TypedASTs to stage2 and stage3.
   */
  public static SourceFile stubSourceFile(String fileName, SourceKind kind) {
    return builder()
        .withPath(fileName)
        .withKind(kind)
        .setIsStubSourceFileForAlreadyProvidedInput()
        .build();
  }

  /**
   * Drops configuration-specific output root prefixes (such as {@code
   * "blaze-out/<config>/{bin,genfiles,testlogs}/"}, {@code
   * "bazel-out/<config>/{bin,genfiles,testlogs}/"}, or bare {@code "bin/"}, {@code "genfiles/"},
   * {@code "testlogs/"}) from the given path.
   *
   * <p>This allows paths to be matched consistently regardless of whether path mapping is enabled
   * (see go/path-mapping-guide).
   */
  public static String stripConfigSegment(String path) {
    // Case 1: Starts with bare config segment (e.g. mapped paths or relative roots)
    if (path.startsWith("bin/")) {
      return path.substring(4);
    }
    if (path.startsWith("genfiles/")) {
      return path.substring(9);
    }
    if (path.startsWith("testlogs/")) {
      return path.substring(9);
    }

    // Case 2: Contains blaze-out/ or bazel-out/
    int blazeOutIdx = path.indexOf("blaze-out/");
    if (blazeOutIdx == -1) {
      blazeOutIdx = path.indexOf("bazel-out/");
    }

    if (blazeOutIdx != -1) {
      // Find the next slash after blaze-out/ or bazel-out/
      int nextSlash = path.indexOf('/', blazeOutIdx + 10);
      if (nextSlash != -1) {
        if (path.startsWith("bin/", nextSlash + 1)) {
          return path.substring(nextSlash + 5);
        }
        if (path.startsWith("genfiles/", nextSlash + 1)) {
          return path.substring(nextSlash + 10);
        }
        if (path.startsWith("testlogs/", nextSlash + 1)) {
          return path.substring(nextSlash + 10);
        }
      }
    }

    return path;
  }

  /**
   * Canonicalizes a file path for matching across compilation stages and TypedAST protos by
   * stripping configuration prefix segments (see {@link #stripConfigSegment(String)} and
   * go/path-mapping-guide).
   */
  public static String canonicalizePathForMatching(String path) {
    return stripConfigSegment(path);
  }

  /**
   * Reconciles serialized state in a {@link SourceFileProto} with the existing state in this file.
   *
   * <p>This should be called whenever initializing a compilation based on TypedAST protos. For
   * these compilations, the compiler initalization methods require creating SourceFiles before
   * deserializing TypedAST protos, so we sometimes get two copies of the same SourceFile.)
   */
  public void restoreCachedStateFrom(SourceFileProto protoSourceFile) {
    checkState(
        protoSourceFile.getFilename().equals(this.getName())
            || canonicalizePathForMatching(protoSourceFile.getFilename())
                .equals(canonicalizePathForMatching(this.getName())),
        "Cannot restore state for %s from %s",
        this.getName(),
        protoSourceFile.getFilename());
    // TypedAST proto information is expected to be more accurate for:
    //  1) whether a SourceFile contains an @extern annotation or not. In non-TypedAST
    //    builds, we allow passing @extern files under the --js flag.  For TypedAST builds, we
    //    could support this, but it's an uncommon pattern and trickier to support than ban.
    //  2) tracking some extra state that is lazily computed in a SourceFile, like the number of
    //     lines and bytes in a file. SourceFile::restoreCachedStateFrom handles this case.
    // Note: the state in the proto might be incorrect in other cases, since some state cannot be
    // computed during library-level typechecking (e.g. what files are in the weak chunk)
    if (protoSourceFile.getSourceKind() == SourceFileProto.SourceKind.EXTERN) {
      checkState(
          this.getKind() == SourceKind.EXTERN,
          "TypedAST compilations must pass all extern files as externs, not js, but found %s",
          this.getName());
    }

    // Restore the number of lines/bytes from the proto, unless we already have cached
    // numLines and numBytes. Offset by 1. the proto "unset" value is 0, where as in SourceFile we
    // use "-1"
    int protoNumLines = protoSourceFile.getNumLinesPlusOne() - 1;
    int protoNumBytes = protoSourceFile.getNumBytesPlusOne() - 1;
    checkState(protoNumLines >= -1, "protoNumLines must be >= -1: %s", protoNumLines);
    checkState(protoNumBytes >= -1, "protoNumBytes must be >= -1: %s", protoNumBytes);
    // It's possible that this.numLines and protoNumLines are both not "-1" but contain
    // conflicting values. This would happen if a file changes on disk after TypedAST
    // proto serialization. We choose the proto value over this.numLines because this
    // data is intended for metrics recording, which should care most about the size of a
    // file when compilation began and parsing ran.
    this.numLines = protoNumLines != -1 ? protoNumLines : this.numLines;
    this.numBytes = protoNumBytes != -1 ? protoNumBytes : this.numBytes;
    this.isClosureUnawareCodeField = protoSourceFile.getIsClosureUnawareCode();
  }

  public static SourceFile fromProto(SourceFileProto protoSourceFile) {
    SourceKind sourceKind = getSourceKindFromProto(protoSourceFile);
    SourceFile sourceFile = fromProto(protoSourceFile, sourceKind);
    // Restore the number of lines/bytes, which are offset by 1 in the proto.
    int protoNumLines = protoSourceFile.getNumLinesPlusOne() - 1;
    int protoNumBytes = protoSourceFile.getNumBytesPlusOne() - 1;
    checkState(protoNumLines >= -1, "protoNumLines must be >= -1: %s", protoNumLines);
    checkState(protoNumBytes >= -1, "protoNumBytes must be >= -1: %s", protoNumBytes);
    sourceFile.numLines = protoNumLines;
    sourceFile.numBytes = protoNumBytes;
    if (protoSourceFile.getIsClosureUnawareCode()) {
      sourceFile.markAsClosureUnawareCode();
    }
    return sourceFile;
  }

  private static SourceFile fromProto(SourceFileProto protoSourceFile, SourceKind sourceKind) {
    switch (protoSourceFile.getLoaderCase()) {
      case PRELOADED_CONTENTS -> {
        return SourceFile.fromCode(
            protoSourceFile.getFilename(), protoSourceFile.getPreloadedContents(), sourceKind);
      }
      case FILE_ON_DISK -> {
        String pathOnDisk =
            protoSourceFile.getFileOnDisk().getActualPath().isEmpty()
                ? protoSourceFile.getFilename()
                : protoSourceFile.getFileOnDisk().getActualPath();
        return SourceFile.builder()
            .withCharset(toCharset(protoSourceFile.getFileOnDisk().getCharset()))
            .withOriginalPath(protoSourceFile.getFilename())
            .withKind(sourceKind)
            .withPath(pathOnDisk)
            .build();
      }
      case ZIP_ENTRY -> {
        SourceFileProto.ZipEntryOnDisk zipEntry = protoSourceFile.getZipEntry();
        return SourceFile.builder()
            .withKind(sourceKind)
            .withOriginalPath(protoSourceFile.getFilename())
            .withCharset(toCharset(zipEntry.getCharset()))
            .withZipEntryPath(zipEntry.getZipPath(), zipEntry.getEntryName())
            .build();
      }
      case STUB_FILE -> {
        return SourceFile.builder()
            .withKind(sourceKind)
            .withOriginalPath(protoSourceFile.getFilename())
            .setIsStubSourceFileForAlreadyProvidedInput()
            .build();
      }
      case LOADER_NOT_SET -> {}
    }
    throw new AssertionError();
  }

  private static SourceKind getSourceKindFromProto(SourceFileProto protoSourceFile) {
    return switch (protoSourceFile.getSourceKind()) {
      case EXTERN -> SourceKind.EXTERN;
      case CODE -> SourceKind.STRONG;
      case NOT_SPECIFIED, UNRECOGNIZED ->
          throw new AssertionError(protoSourceFile.getSourceKind().toString());
    };
  }

  private static String toCharset(String protoCharset) {
    if (protoCharset.isEmpty()) {
      return Charsets.UTF_8;
    }
    return protoCharset;
  }

  /** Create a new builder for source files. */
  public static Builder builder() {
    return new Builder();
  }

  /**
   * A builder interface for source files.
   *
   * <p>Allows users to customize the Charset, and the original path of the source file (if it
   * differs from the path on disk).
   */
  public static final class Builder {
    private SourceKind kind = SourceKind.STRONG;
    private String charset = Charsets.UTF_8;
    private Optional<String> originalPath = Optional.empty();
    private boolean isStubSourceFileForAlreadyProvidedInput = false;

    private Optional<String> path = Optional.empty();
    private Optional<String> zipEntryPath = Optional.empty();

    private @Nullable Supplier<String> lazyContent = null;

    private Builder() {}

    /** Set the source kind. */
    @CanIgnoreReturnValue
    public Builder withKind(SourceKind kind) {
      this.kind = kind;
      return this;
    }

    /** Set the charset to use when reading from an input stream or file. */
    @CanIgnoreReturnValue
    public Builder withCharset(String charset) {
      this.charset = charset;
      return this;
    }

    @CanIgnoreReturnValue
    public Builder withPath(String path) {
      return this.withPathInternal(path);
    }

    @CanIgnoreReturnValue
    public Builder withContent(String x) {
      this.lazyContent = x::toString;
      return this;
    }

    @CanIgnoreReturnValue
    public Builder withZipEntryPath(String zipPath, String entryPath) {
      this.path = Optional.of(zipPath);
      this.zipEntryPath = Optional.of(entryPath);

      return this;
    }

    /**
     * Sets a name for this source file that does not need to correspond to a path on disk.
     *
     * <p>Allow passing a reasonable human-readable name in cases like for zip files and for
     * generated files with unstable artifact prefixes.
     *
     * <p>The name must still be unique.
     */
    @CanIgnoreReturnValue
    public Builder withOriginalPath(String originalPath) {
      this.originalPath = Optional.of(originalPath);
      return this;
    }

    /**
     * Sets that this SourceFile is a stub file for an input that was already provided to the
     * compiler in the form of TypedAST.
     */
    @CanIgnoreReturnValue
    public Builder setIsStubSourceFileForAlreadyProvidedInput() {
      this.isStubSourceFileForAlreadyProvidedInput = true;
      return this;
    }

    public SourceFile build() {
      Optional<String> displayPath =
          (this.originalPath.isPresent())
              ? this.originalPath
              : ((this.zipEntryPath.isEmpty())
                  ? this.path
                  : Optional.of(this.path.orElse("null") + BANG_SLASH + this.zipEntryPath.get()));

      if (this.isStubSourceFileForAlreadyProvidedInput) {
        SourceFile file =
            new SourceFile(new CodeLoader.StubSourceFileCodeLoader(), displayPath.get(), this.kind);
        file.setIsStubSourceFileForAlreadyProvidedInput(true);
        return file;
      }

      if (this.lazyContent != null) {
        return new SourceFile(
            new CodeLoader.Preloaded(this.lazyContent.get()), displayPath.get(), this.kind);
      }

      if (this.zipEntryPath.isPresent()) {
        return new SourceFile(
            new CodeLoader.AtZip(this.path.orElse(null), this.zipEntryPath.get(), this.charset),
            displayPath.get(),
            this.kind);
      }

      return new SourceFile(
          new CodeLoader.OnDisk(FilePaths.clean(this.path.get()), this.charset),
          displayPath.get(),
          this.kind);
    }

    @CanIgnoreReturnValue
    private Builder withPathInternal(String path) {
      // Check if this path should be inferred as a ZIP entry path.
      int bangSlashIndex = path.indexOf(BANG_SLASH);
      if (bangSlashIndex >= 0) {
        String zipPath = path.substring(0, bangSlashIndex);
        String entryPath = path.substring(bangSlashIndex + BANG_SLASH.length());

        if (zipPath.endsWith(".zip")
            && (entryPath.endsWith(".js") || entryPath.endsWith(".js.map"))) {
          return this.withZipEntryPath(zipPath, entryPath);
        }
      }

      this.path = Optional.of(path);

      return this;
    }
  }

  //////////////////////////////////////////////////////////////////////////////
  // Implementations

  private abstract static class CodeLoader implements Serializable {
    /**
     * Return the source text of this file from its original storage.
     *
     * <p>The implementation may be a slow operation such as reading from a file. SourceFile
     * guarantees that this method is only called under synchronization.
     */
    Outcome<String> loadUncachedCode() {
      throw new AssertionError();
    }

    /**
     * Return the text of this file from its original storage for a single pass over it, or null
     * if this loader has nothing better to offer than {@link #loadUncachedCode}.
     *
     * <p>The implementation may be a slow operation such as reading from a file. SourceFile
     * guarantees that this method is only called under synchronization.
     */
    @Nullable Outcome<String> loadUncachedText() {
      return null;
    }

    /**
     * Returns a representation of this loader that can be serialized/deserialized to reconstruct
     * this SourceFile
     */
    abstract SourceFileProto.Builder toProtoLocationBuilder(String fileName);

    static final class Preloaded extends CodeLoader {
      private static final long serialVersionUID = 2L;
      private final String preloadedCode;

      Preloaded(String preloadedCode) {
        super();
        this.preloadedCode = preloadedCode;
      }

      @Override
      Outcome<String> loadUncachedCode() {
        return Outcome.ok(this.preloadedCode);
      }

      @Override
      SourceFileProto.Builder toProtoLocationBuilder(String fileName) {
        return SourceFileProto.newBuilder().setPreloadedContents(this.preloadedCode);
      }
    }

    /* A code loader which throws an error when loading code for a stub SourceFile. */
    static final class StubSourceFileCodeLoader extends CodeLoader {
      private static final long serialVersionUID = 2L;

      StubSourceFileCodeLoader() {
        super();
      }

      @Override
      Outcome<String> loadUncachedCode() {
        throw new UnsupportedOperationException("Attempting to load code from a stub SourceFile.");
      }

      @Override
      SourceFileProto.Builder toProtoLocationBuilder(String fileName) {
        return SourceFileProto.newBuilder().setFilename(fileName).setStubFile(true);
      }
    }

    static final class OnDisk extends CodeLoader {
      private static final long serialVersionUID = 1L;

      private final String serializableCharset;
      private final String relativePath;

      OnDisk(String relativePath, String c) {
        super();
        this.serializableCharset = Charsets.name(c);
        this.relativePath = relativePath;
      }

      @Override
      Outcome<String> loadUncachedCode() {
        Outcome<String> text = Io.readTextStrict(this.relativePath, this.serializableCharset);
        if (Io.isDecodingError(text.error())) {
          return Outcome.err(
              new IOException(
                  "Failed to read: " + this.relativePath + ", is this input UTF-8 encoded?",
                  text.error()));
        }
        return text;
      }

      @Override
      Outcome<String> loadUncachedText() {
        return Io.readTextStrict(this.relativePath, this.serializableCharset);
      }

      @Override
      SourceFileProto.Builder toProtoLocationBuilder(String fileName) {
        String actualPath = this.relativePath;
        return SourceFileProto.newBuilder()
            .setFileOnDisk(
                FileOnDisk.newBuilder()
                    .setActualPath(
                        // to save space, don't serialize the path if equal to the fileName.
                        fileName.equals(actualPath) ? "" : actualPath)
                    // save space by not serializing UTF_8 (the default charset)
                    .setCharset(
                        this.serializableCharset.equals(Charsets.UTF_8) ? "" : this.serializableCharset));
      }
    }

    static final class AtZip extends CodeLoader {
      private static final long serialVersionUID = 1L;
      private final String zipName;
      private final String entryName;
      private final String serializableCharset;

      AtZip(String zipName, String entryName, String c) {
        super();
        this.zipName = zipName;
        this.entryName = entryName;
        this.serializableCharset = Charsets.name(c);
      }

      @Override
      Outcome<String> loadUncachedCode() {
        return Io.readZipEntryText(this.zipName, this.entryName, this.serializableCharset);
      }

      @Override
      Outcome<String> loadUncachedText() {
        return this.loadUncachedCode();
      }

      @Override
      SourceFileProto.Builder toProtoLocationBuilder(String fileName) {
        return SourceFileProto.newBuilder()
            .setFilename(fileName)
            .setZipEntry(
                ZipEntryOnDisk.newBuilder()
                    .setEntryName(this.entryName)
                    .setZipPath(this.zipName)
                    // save space by not serializing UTF_8 (the default charset)
                    .setCharset(
                        this.serializableCharset.equals(Charsets.UTF_8) ? "" : this.serializableCharset)
                    .build());
      }
    }
  }

  public SourceFileProto getProto() {
    return this.loader
        .toProtoLocationBuilder(this.getName())
        .setFilename(this.getName())
        .setSourceKind(sourceKindToProto(this.getKind()))
        .setIsClosureUnawareCode(this.isClosureUnawareCodeField)
        .setNumLinesPlusOne(this.numLines + 1)
        .setNumBytesPlusOne(this.numBytes + 1)
        .build();
  }

  private static SourceFileProto.SourceKind sourceKindToProto(SourceKind sourceKind) {
    return switch (sourceKind) {
      case EXTERN -> SourceFileProto.SourceKind.EXTERN;
      case STRONG, WEAK -> SourceFileProto.SourceKind.CODE;
      case NON_CODE -> throw new AssertionError(sourceKind.toString());
    };
  }
}
