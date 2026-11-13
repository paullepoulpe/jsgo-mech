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

import java.io.IOException;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * A zip archive held in memory, read through its central directory: the entries are the ones the
 * directory at the end of the archive lists, in the order it lists them (the order in which a
 * zip tool added them). Entries are stored or deflated; an archive that needs the zip64 format,
 * encryption or another compression method cannot be read. Entry names are UTF-8.
 */
public final class ZipArchive {
  private final byte[] bytes;
  private final List<String> entryNames = new ArrayList<>();

  /** For each name, where its (last) central directory record starts in {@code bytes}. */
  private final Map<String, Integer> records = new LinkedHashMap<>();

  private ZipArchive(byte[] bytes) {
    this.bytes = bytes;
  }

  private static final int END_SIGNATURE = 0x06054b50;
  private static final int END_SIZE = 22;
  private static final int RECORD_SIGNATURE = 0x02014b50;
  private static final int RECORD_SIZE = 46;
  private static final int LOCAL_SIGNATURE = 0x04034b50;
  private static final int LOCAL_SIZE = 30;

  /** The archive of the bytes; empty if they are not a zip archive that can be read. */
  public static Optional<ZipArchive> parse(byte[] bytes) {
    // The end record is the last thing in the archive, but for a comment of up to 65535 bytes.
    int end = bytes.length - END_SIZE;
    int lowest = Math.max(0, end - 0xFFFF);
    while (end >= lowest && int32(bytes, end) != END_SIGNATURE) {
      end--;
    }
    if (end < lowest) {
      return Optional.empty();
    }
    int count = int16(bytes, end + 10);
    int position = int32(bytes, end + 16);
    if (count == 0xFFFF || position == -1) {
      return Optional.empty(); // zip64
    }
    ZipArchive archive = new ZipArchive(bytes);
    for (int i = 0; i < count; i++) {
      if (position < 0
          || position + RECORD_SIZE > bytes.length
          || int32(bytes, position) != RECORD_SIGNATURE) {
        return Optional.empty();
      }
      int nameLength = int16(bytes, position + 28);
      int next =
          position
              + RECORD_SIZE
              + nameLength
              + int16(bytes, position + 30)
              + int16(bytes, position + 32);
      if (next > bytes.length) {
        return Optional.empty();
      }
      String name = Io.utf8String(slice(bytes, position + RECORD_SIZE, nameLength));
      archive.entryNames.add(name);
      archive.records.put(name, position);
      position = next;
    }
    return Optional.of(archive);
  }

  /** The names of the entries (directories, whose names end in "/", too), in directory order. */
  public List<String> names() {
    return entryNames;
  }

  /** The data of the entry; empty if there is no entry of that name or it cannot be read. */
  public Optional<byte[]> read(String name) {
    Integer record = records.get(name);
    if (record == null) {
      return Optional.empty();
    }
    int method = int16(bytes, record + 10);
    int storedSize = int32(bytes, record + 20);
    int size = int32(bytes, record + 24);
    int local = int32(bytes, record + 42);
    if (local < 0
        || local + LOCAL_SIZE > bytes.length
        || int32(bytes, local) != LOCAL_SIGNATURE
        || storedSize < 0
        || size < 0) {
      return Optional.empty();
    }
    int start = local + LOCAL_SIZE + int16(bytes, local + 26) + int16(bytes, local + 28);
    if (start + storedSize > bytes.length) {
      return Optional.empty();
    }
    byte[] stored = slice(bytes, start, storedSize);
    if (method == 0) {
      return Optional.of(stored);
    }
    if (method != 8) {
      return Optional.empty();
    }
    Outcome<byte[]> data = Io.inflate(stored);
    if (data.isErr() || data.get().length != size) {
      return Optional.empty();
    }
    return Optional.of(data.get());
  }

  private static byte[] slice(byte[] bytes, int start, int length) {
    byte[] part = new byte[length];
    System.arraycopy(bytes, start, part, 0, length);
    return part;
  }

  /** The unsigned 16-bit little-endian number at the position. */
  private static int int16(byte[] bytes, int position) {
    return (bytes[position] & 0xFF) | ((bytes[position + 1] & 0xFF) << 8);
  }

  /** The 32-bit little-endian number at the position (negative from 2^31: too big to be used). */
  private static int int32(byte[] bytes, int position) {
    return int16(bytes, position) | (int16(bytes, position + 2) << 16);
  }

  // ---- Zip files on disk

  /** A file's archive, and the modification time the file had when it was read. */
  private static final class Opened {
    final ZipArchive archive;
    final long modified;

    Opened(ZipArchive archive, long modified) {
      this.archive = archive;
      this.modified = modified;
    }
  }

  private static final Map<String, Opened> opened = new LinkedHashMap<>();

  /**
   * The archive of the zip file. It is kept for the next call and read again only when the file's
   * modification time has changed. The error message of a file that is there but is not a zip
   * archive is "Cannot read zip file " and the path.
   */
  public static synchronized Outcome<ZipArchive> open(String path) {
    long modified = Io.lastModified(path);
    Opened known = opened.get(path);
    if (known != null && known.modified == modified) {
      return Outcome.ok(known.archive);
    }
    Outcome<byte[]> bytes = Io.readBytes(path);
    if (bytes.isErr()) {
      return Outcome.err(bytes.error());
    }
    Optional<ZipArchive> archive = parse(bytes.get());
    if (archive.isEmpty()) {
      return Outcome.err(unreadable(path));
    }
    opened.put(path, new Opened(archive.get(), modified));
    return Outcome.ok(archive.get());
  }

  /** The data of one entry of the zip file; an entry that is not there is an error. */
  public static Outcome<byte[]> readEntry(String path, String name) {
    Outcome<ZipArchive> archive = open(path);
    if (archive.isErr()) {
      return Outcome.err(archive.error());
    }
    Optional<byte[]> data = archive.get().read(name);
    if (data.isEmpty()) {
      return Outcome.err(unreadable(path + "!/" + name));
    }
    return Outcome.ok(data.get());
  }

  private static IOException unreadable(String path) {
    return new IOException("Cannot read zip file " + path);
  }
}
