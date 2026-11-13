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
import static com.google.common.truth.Truth.assertWithMessage;

import java.nio.charset.Charset;
import java.nio.charset.CharsetEncoder;
import java.util.Optional;
import java.util.Random;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Checks {@link Charsets} against the JDK's charsets of the same names. */
@RunWith(JUnit4.class)
public final class CharsetsTest {
  private static final String[] ALL = {
    "UTF-8", "US-ASCII", "ISO-8859-1", "windows-1252", "UTF-16", "UTF-16BE", "UTF-16LE"
  };

  @Test
  public void names() {
    assertThat(Charsets.canonicalName("utf8")).isEqualTo(Optional.of("UTF-8"));
    assertThat(Charsets.canonicalName("Utf-8")).isEqualTo(Optional.of("UTF-8"));
    assertThat(Charsets.canonicalName("ascii")).isEqualTo(Optional.of("US-ASCII"));
    assertThat(Charsets.canonicalName("LATIN1")).isEqualTo(Optional.of("ISO-8859-1"));
    assertThat(Charsets.canonicalName("iso8859-1")).isEqualTo(Optional.of("ISO-8859-1"));
    assertThat(Charsets.canonicalName("cp1252")).isEqualTo(Optional.of("windows-1252"));
    assertThat(Charsets.canonicalName("utf-16le")).isEqualTo(Optional.of("UTF-16LE"));
    for (String name : ALL) {
      assertThat(Charsets.name(name)).isEqualTo(name);
      assertThat(Charset.forName(name).name()).isEqualTo(name);
    }
    assertThat(Charsets.isSupported("Shift_JIS")).isFalse();
    assertThat(Charsets.isSupported("bad*name")).isFalse();
    assertThat(Charsets.isSupported("")).isFalse();
  }

  @Test
  public void canEncodeAgreesWithJdk() {
    for (String name : ALL) {
      CharsetEncoder encoder = Charset.forName(name).newEncoder();
      for (int c = 0; c <= 0xFFFF; c++) {
        assertWithMessage("%s U+%s", name, Integer.toHexString(c))
            .that(Charsets.canEncode((char) c, name))
            .isEqualTo(encoder.canEncode((char) c));
      }
    }
  }

  @Test
  public void encodeAgreesWithJdk() {
    Random random = new Random(1);
    for (String name : ALL) {
      Charset jdk = Charset.forName(name);
      for (int round = 0; round < 2000; round++) {
        StringBuilder text = new StringBuilder();
        int length = random.nextInt(8);
        for (int i = 0; i < length; i++) {
          switch (random.nextInt(5)) {
            case 0 -> text.append((char) random.nextInt(0x80));
            case 1 -> text.append((char) random.nextInt(0x300));
            case 2 -> text.appendCodePoint(0x10000 + random.nextInt(0x1000));
            case 3 -> text.append("€™�ž".charAt(random.nextInt(4)));
            default -> text.append((char) (0x2000 + random.nextInt(0x100)));
          }
        }
        String s = text.toString();
        assertWithMessage("%s %s", name, s)
            .that(Charsets.encode(s, name))
            .isEqualTo(s.getBytes(jdk));
      }
    }
    // Half a surrogate pair on its own.
    assertThat(Charsets.encode("a\uD800b", "UTF-8")).isEqualTo(new byte[] {'a', '?', 'b'});
    assertThat(Charsets.encode("a\uDC00", "ISO-8859-1")).isEqualTo(new byte[] {'a', '?'});
    assertThat(Charsets.encode("\uD800", "UTF-16BE")).isEqualTo(new byte[] {(byte) 0xFF, (byte) 0xFD});
  }

  @Test
  public void decodeOfValidBytesAgreesWithJdk() {
    Random random = new Random(2);
    for (String name : ALL) {
      Charset jdk = Charset.forName(name);
      for (int round = 0; round < 2000; round++) {
        StringBuilder text = new StringBuilder();
        int length = random.nextInt(8);
        for (int i = 0; i < length; i++) {
          if (random.nextInt(4) == 0) {
            text.appendCodePoint(0x10000 + random.nextInt(0x1000));
          } else {
            text.append((char) (0x20 + random.nextInt(0x250)));
          }
        }
        byte[] bytes = text.toString().getBytes(jdk);
        String expected = new String(bytes, jdk);
        assertWithMessage("%s %s", name, expected)
            .that(Charsets.decode(bytes, name))
            .isEqualTo(expected);
        if (expected.indexOf('�') < 0 && bytes.length > 0) {
          assertThat(Charsets.decodeStrict(bytes, name)).isEqualTo(Optional.of(expected));
        }
      }
    }
  }

  @Test
  public void singleByteCharsetsAgreeWithJdkOnEveryByte() {
    for (String name : new String[] {"US-ASCII", "ISO-8859-1", "windows-1252"}) {
      Charset jdk = Charset.forName(name);
      for (int b = 0; b < 256; b++) {
        byte[] bytes = {(byte) b};
        String expected = new String(bytes, jdk);
        assertThat(Charsets.decode(bytes, name)).isEqualTo(expected);
        assertThat(Charsets.decodeStrict(bytes, name).isPresent())
            .isEqualTo(!expected.equals("�"));
      }
    }
  }

  @Test
  public void malformedUtf8() {
    // Each byte that is not part of a well-formed sequence is one U+FFFD.
    byte[] truncated = {'a', (byte) 0xE2, (byte) 0x82, 'b'};
    assertThat(Charsets.decode(truncated, "UTF-8")).isEqualTo("a��b");
    assertThat(Charsets.decodeStrict(truncated, "UTF-8")).isEqualTo(Optional.empty());
    byte[] latin1 = {'c', 'a', 'f', (byte) 0xE9};
    assertThat(Charsets.decode(latin1, "UTF-8")).isEqualTo("caf�");
    byte[] surrogate = {(byte) 0xED, (byte) 0xA0, (byte) 0x80};
    assertThat(Charsets.decode(surrogate, "UTF-8")).isEqualTo("���");
    byte[] overlong = {(byte) 0xC0, (byte) 0x80};
    assertThat(Charsets.decode(overlong, "UTF-8")).isEqualTo("��");
    byte[] mixed = {(byte) 0xE2, (byte) 0x82, (byte) 0xAC, (byte) 0xFF, (byte) 0xEF, (byte) 0xBF, (byte) 0xBD};
    assertThat(Charsets.decode(mixed, "UTF-8")).isEqualTo("€��");
    assertThat(Charsets.decodeStrict(new byte[] {(byte) 0xEF, (byte) 0xBF, (byte) 0xBD}, "UTF-8"))
        .isEqualTo(Optional.of("�"));
  }

  @Test
  public void utf16ByteOrderMark() {
    byte[] little = {(byte) 0xFF, (byte) 0xFE, 'a', 0};
    assertThat(Charsets.decode(little, "UTF-16")).isEqualTo("a");
    byte[] big = {(byte) 0xFE, (byte) 0xFF, 0, 'a'};
    assertThat(Charsets.decode(big, "UTF-16")).isEqualTo("a");
    assertThat(Charsets.decode(new byte[] {0, 'a'}, "UTF-16")).isEqualTo("a");
    assertThat(Charsets.decode(big, "UTF-16BE")).isEqualTo("﻿a");
    assertThat(Charsets.decodeStrict(new byte[] {0, 'a', 0}, "UTF-16")).isEqualTo(Optional.empty());
    assertThat(Charsets.decode(new byte[] {0, 'a', 0}, "UTF-16")).isEqualTo("a�");
    assertThat(Charsets.encode("", "UTF-16")).isEqualTo(new byte[0]);
  }
}
