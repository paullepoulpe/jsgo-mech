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
import static java.nio.charset.StandardCharsets.ISO_8859_1;
import static java.nio.charset.StandardCharsets.UTF_16;
import static java.nio.charset.StandardCharsets.UTF_8;

import java.io.BufferedReader;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.PrintStream;
import java.io.StringReader;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import org.junit.Rule;
import org.junit.Test;
import org.junit.rules.TemporaryFolder;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Tests {@link Output}, {@link TextFiles} and the file operations of {@link Io} under them. */
@RunWith(JUnit4.class)
public final class OutputAndTextFilesTest {
  @Rule public TemporaryFolder folder = new TemporaryFolder();

  @Test
  public void streamOutputIsUnbufferedUtf8() {
    ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    Output out = Io.outputToStream(new PrintStream(bytes));
    out.print("aé");
    assertThat(bytes.toString(UTF_8)).isEqualTo("aé");
    out.println("b");
    out.append('c').append("def", 1, 2).append("g");
    assertThat(bytes.toString(UTF_8)).isEqualTo("aéb\nceg");
    // A surrogate pair that arrives in two pieces is one character.
    out.append('\uD83D');
    out.append('\uDE00');
    assertThat(bytes.toString(UTF_8)).isEqualTo("aéb\nceg😀");
    // Half a pair at the very end is not a character.
    out.append('\uD83D');
    assertThat(out.close().isOk()).isTrue();
    assertThat(bytes.toString(UTF_8)).isEqualTo("aéb\nceg😀?");
    // Writing and closing after the close do nothing.
    out.print("more");
    assertThat(out.close().isOk()).isTrue();
    assertThat(bytes.toString(UTF_8)).isEqualTo("aéb\nceg😀?");
  }

  @Test
  public void encodedAsBuffersAndSharesTheDestination() {
    ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    Output out = Io.outputToStream(new PrintStream(bytes));
    Output latin1 = out.encodedAs("ISO-8859-1");
    latin1.print("é€");
    assertThat(bytes.size()).isEqualTo(0);
    latin1.flush();
    assertThat(bytes.toString(ISO_8859_1)).isEqualTo("é?");
    assertThat(latin1.close().isOk()).isTrue();
    out.print("lost");
    assertThat(bytes.toString(ISO_8859_1)).isEqualTo("é?");
  }

  @Test
  public void fileOutput() throws IOException {
    Path path = folder.getRoot().toPath().resolve("out.txt");
    Output out = Output.open(path.toString(), "UTF-16", false).get();
    StringBuilder expected = new StringBuilder();
    for (int i = 0; i < 3000; i++) {
      out.print("line " + i + "\n");
      expected.append("line ").append(i).append('\n');
    }
    assertThat(out.close().isOk()).isTrue();
    // One byte order mark, at the start, although the text went out in several pieces.
    assertThat(new String(Files.readAllBytes(path), UTF_16)).isEqualTo(expected.toString());
    assertThat(Files.size(path)).isEqualTo(2 + 2L * expected.length());

    Output more = Output.open(path.toString(), "UTF-16BE", true).get();
    more.print("x");
    assertThat(more.close().isOk()).isTrue();
    assertThat(new String(Files.readAllBytes(path), UTF_16)).isEqualTo(expected + "x");

    Output.discard().print("nothing");
    assertThat(Output.discard().close().isOk()).isTrue();
  }

  @Test
  public void failureMessages() throws IOException {
    String dir = folder.getRoot().toString();
    assertThat(TextFiles.read(dir + "/nope.txt", "UTF-8").errorMessage().get())
        .isEqualTo(dir + "/nope.txt: No such file");
    assertThat(TextFiles.readStrict(dir, "UTF-8").errorMessage().get())
        .isEqualTo(dir + ": Cannot read file");
    assertThat(Output.open(dir + "/nodir/x.txt", "UTF-8", false).errorMessage().get())
        .isEqualTo(dir + "/nodir/x.txt: Cannot write file");
    assertThat(TextFiles.write(dir + "/nodir/x.txt", "UTF-8", "x").errorMessage().get())
        .isEqualTo(dir + "/nodir/x.txt: Cannot write file");

    Path latin1 = folder.newFile("latin1.js").toPath();
    Files.write(latin1, "var café;".getBytes(ISO_8859_1));
    Outcome<String> strict = TextFiles.readStrict(latin1.toString(), "UTF-8");
    assertThat(strict.errorMessage().get())
        .isEqualTo("Failed to read: " + latin1 + ", is this input UTF-8 encoded?");
    assertThat(TextFiles.isDecodingError(strict.error())).isTrue();
    assertThat(TextFiles.read(latin1.toString(), "UTF-8").get()).isEqualTo("var caf�;");
    assertThat(TextFiles.readStrict(latin1.toString(), "latin1").get()).isEqualTo("var café;");

    assertThat(Io.gunzip(new byte[] {1, 2, 3}).errorMessage().get()).isEqualTo("Malformed gzip data");
    byte[] gzipped = Io.gzip("hello".getBytes(UTF_8));
    assertThat(new String(Io.gunzip(gzipped).get(), UTF_8)).isEqualTo("hello");
    byte[] truncated = new byte[gzipped.length - 6];
    System.arraycopy(gzipped, 0, truncated, 0, truncated.length);
    assertThat(Io.gunzip(truncated).errorMessage().get()).isEqualTo("Malformed gzip data");
    assertThat(Io.inflate(new byte[] {7, 7, 7}).errorMessage().get())
        .isEqualTo("Malformed compressed data");
  }

  @Test
  public void writeAndListFiles() throws IOException {
    String dir = folder.getRoot().toString();
    Io.mkdirs(dir + "/b/c");
    Io.mkdirs(dir + "/a");
    assertThat(TextFiles.write(dir + "/b/c/z.js", "UTF-8", "z").isOk()).isTrue();
    assertThat(TextFiles.write(dir + "/b/y.js", "UTF-8", "y").isOk()).isTrue();
    assertThat(TextFiles.write(dir + "/b.js", "UTF-8", "b").isOk()).isTrue();
    assertThat(TextFiles.write(dir + "/a/x.js", "UTF-8", "x").isOk()).isTrue();
    Files.createSymbolicLink(Path.of(dir, "b", "loop"), Path.of(dir));
    assertThat(Io.listFiles(dir)).containsExactly("a/x.js", "b/c/z.js", "b/y.js", "b.js").inOrder();
    assertThat(Io.listFiles(dir + "/b.js")).isEmpty();
    assertThat(Io.listFiles(dir + "/nope")).isEmpty();
    assertThat(Io.isDirectory(dir + "/b/loop")).isTrue();
    assertThat(Io.exists(dir + "/b.js")).isTrue();
    assertThat(Io.exists(dir + "/c.js")).isFalse();
    assertThat(Io.lastModified(dir + "/c.js")).isEqualTo(0);
    assertThat(Io.lastModified(dir + "/b.js")).isGreaterThan(0L);
  }

  @Test
  public void splitLinesAgreesWithBufferedReader() throws IOException {
    String[] texts = {
      "", "a", "a\n", "a\nb", "a\r\nb\r\n", "a\rb", "\n", "\n\n", "\r\n\r", "a\n\rb", "a\r\r\nb", " \n ",
    };
    for (String text : texts) {
      List<String> expected = new ArrayList<>();
      BufferedReader reader = new BufferedReader(new StringReader(text));
      Optional<String> line = Optional.ofNullable(reader.readLine());
      while (line.isPresent()) {
        expected.add(line.get());
        line = Optional.ofNullable(reader.readLine());
      }
      assertThat(TextFiles.splitLines(text)).isEqualTo(expected);
    }
  }
}
