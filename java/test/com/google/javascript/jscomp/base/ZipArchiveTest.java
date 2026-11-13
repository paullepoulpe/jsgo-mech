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

import static com.google.common.truth.Truth.assertThat;
import static java.nio.charset.StandardCharsets.UTF_8;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Optional;
import java.util.Random;
import java.util.zip.CRC32;
import java.util.zip.ZipEntry;
import java.util.zip.ZipOutputStream;
import org.junit.Rule;
import org.junit.Test;
import org.junit.rules.TemporaryFolder;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Reads archives that java.util.zip wrote. */
@RunWith(JUnit4.class)
public final class ZipArchiveTest {
  @Rule public TemporaryFolder folder = new TemporaryFolder();

  private static byte[] zipOf(String comment, String... namesAndTexts) throws IOException {
    ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    try (ZipOutputStream zip = new ZipOutputStream(bytes)) {
      zip.setComment(comment);
      for (int i = 0; i < namesAndTexts.length; i += 2) {
        byte[] data = namesAndTexts[i + 1].getBytes(UTF_8);
        ZipEntry entry = new ZipEntry(namesAndTexts[i]);
        if (namesAndTexts[i].startsWith("stored")) {
          CRC32 crc = new CRC32();
          crc.update(data);
          entry.setMethod(ZipEntry.STORED);
          entry.setSize(data.length);
          entry.setCrc(crc.getValue());
        }
        zip.putNextEntry(entry);
        zip.write(data);
        zip.closeEntry();
      }
    }
    return bytes.toByteArray();
  }

  private static String text(ZipArchive archive, String name) {
    return new String(archive.read(name).get(), UTF_8);
  }

  @Test
  public void readsDeflatedAndStoredEntriesInOrder() throws IOException {
    Random random = new Random(3);
    StringBuilder big = new StringBuilder();
    for (int i = 0; i < 50000; i++) {
      big.append((char) ('a' + random.nextInt(4)));
    }
    byte[] zip =
        zipOf(
            "a comment",
            "z.js", "var z;",
            "dir/", "",
            "dir/a.js", big.toString(),
            "stored.js", "var stored = 'é';",
            "été.js", "",
            "b.txt", "text");
    ZipArchive archive = ZipArchive.parse(zip).get();
    assertThat(archive.names())
        .containsExactly("z.js", "dir/", "dir/a.js", "stored.js", "été.js", "b.txt")
        .inOrder();
    assertThat(text(archive, "z.js")).isEqualTo("var z;");
    assertThat(text(archive, "dir/")).isEmpty();
    assertThat(text(archive, "dir/a.js")).isEqualTo(big.toString());
    assertThat(text(archive, "stored.js")).isEqualTo("var stored = 'é';");
    assertThat(text(archive, "été.js")).isEmpty();
    assertThat(archive.read("nope.js")).isEqualTo(Optional.empty());
  }

  @Test
  public void emptyArchive() throws IOException {
    assertThat(ZipArchive.parse(zipOf("")).get().names()).isEmpty();
  }

  @Test
  public void notAnArchive() throws IOException {
    assertThat(ZipArchive.parse(new byte[0]).isPresent()).isFalse();
    assertThat(ZipArchive.parse("this is not a zip file, it is a text".getBytes(UTF_8)).isPresent())
        .isFalse();
    byte[] zip = zipOf("", "a.js", "var a;");
    byte[] truncated = new byte[zip.length - 10];
    System.arraycopy(zip, 0, truncated, 0, truncated.length);
    assertThat(ZipArchive.parse(truncated).isPresent()).isFalse();
  }

  @Test
  public void corruptEntry() throws IOException {
    StringBuilder text = new StringBuilder();
    for (int i = 0; i < 1000; i++) {
      text.append("line ").append(i).append('\n');
    }
    byte[] zip = zipOf("", "a.js", text.toString(), "b.js", "var b;");
    for (int i = 40; i < 60; i++) {
      zip[i] = (byte) 0xFF;
    }
    ZipArchive archive = ZipArchive.parse(zip).get();
    assertThat(archive.read("a.js")).isEqualTo(Optional.empty());
    assertThat(text(archive, "b.js")).isEqualTo("var b;");
  }

  @Test
  public void fileOnDisk() throws IOException {
    Path path = folder.newFile("x.zip").toPath();
    Files.write(path, zipOf("", "a.js", "var a;"));
    assertThat(new String(ZipArchive.readEntry(path.toString(), "a.js").get(), UTF_8))
        .isEqualTo("var a;");
    assertThat(ZipArchive.readEntry(path.toString(), "b.js").errorMessage().get())
        .isEqualTo("Cannot read zip file " + path + "!/b.js");

    Path notZip = folder.newFile("y.zip").toPath();
    Files.write(notZip, new byte[] {1, 2, 3});
    assertThat(ZipArchive.open(notZip.toString()).errorMessage().get())
        .isEqualTo("Cannot read zip file " + notZip);
    assertThat(ZipArchive.open(folder.getRoot() + "/nope.zip").isErr()).isTrue();
  }
}
