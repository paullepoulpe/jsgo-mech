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

import com.google.common.base.Ascii;
import java.util.Optional;

/**
 * The character encodings the compiler reads and writes, by name: exactly the seven below. Every
 * rule of every encoding is spelled out here; only UTF-8 itself is delegated ({@link
 * Io#utf8String}, {@link Io#utf8Bytes}).
 */
public final class Charsets {
  private Charsets() {}

  public static final String UTF_8 = "UTF-8";
  public static final String US_ASCII = "US-ASCII";
  public static final String ISO_8859_1 = "ISO-8859-1";
  public static final String WINDOWS_1252 = "windows-1252";

  /** Reading: byte order from a leading byte order mark, else big endian. Writing: FE FF first. */
  public static final String UTF_16 = "UTF-16";

  public static final String UTF_16BE = "UTF-16BE";
  public static final String UTF_16LE = "UTF-16LE";

  /** Lower-case name or alias, canonical name; in pairs. */
  private static final String[] NAMES = {
    "utf-8", UTF_8,
    "utf8", UTF_8,
    "us-ascii", US_ASCII,
    "ascii", US_ASCII,
    "iso-8859-1", ISO_8859_1,
    "iso8859-1", ISO_8859_1,
    "iso8859_1", ISO_8859_1,
    "latin1", ISO_8859_1,
    "windows-1252", WINDOWS_1252,
    "cp1252", WINDOWS_1252,
    "utf-16", UTF_16,
    "utf16", UTF_16,
    "utf-16be", UTF_16BE,
    "utf16be", UTF_16BE,
    "utf-16le", UTF_16LE,
    "utf16le", UTF_16LE,
  };

  /** The characters of the windows-1252 bytes 0x80..0x9F; 0xFFFD where the byte is unassigned. */
  private static final String WINDOWS_1252_HIGH =
      "€�‚ƒ„…†‡ˆ‰Š‹Œ�Ž�"
          + "�‘’“”•–—˜™š›œ�žŸ";

  private static final char REPLACEMENT = '�';

  /**
   * The canonical name of the charset the name or alias (in any letter case) stands for; empty if
   * it is not one of the seven.
   */
  public static Optional<String> canonicalName(String name) {
    String lower = Ascii.toLowerCase(name);
    for (int i = 0; i < NAMES.length; i += 2) {
      if (NAMES[i].equals(lower)) {
        return Optional.of(NAMES[i + 1]);
      }
    }
    return Optional.empty();
  }

  public static boolean isSupported(String name) {
    return canonicalName(name).isPresent();
  }

  /** The canonical name; a name that {@link #isSupported} rejects is a programming error. */
  public static String name(String name) {
    Optional<String> canonical = canonicalName(name);
    if (canonical.isEmpty()) {
      throw new IllegalArgumentException("Unsupported charset: " + name);
    }
    return canonical.get();
  }

  /** The text of the bytes; every byte or unit that is not valid in the charset is U+FFFD. */
  public static String decode(byte[] bytes, String charset) {
    String canonical = name(charset);
    if (canonical.equals(UTF_8)) {
      return Io.utf8String(bytes);
    }
    return decodeOther(bytes, canonical, false).get();
  }

  /** The text of the bytes; empty if a byte or unit is not valid in the charset. */
  public static Optional<String> decodeStrict(byte[] bytes, String charset) {
    String canonical = name(charset);
    if (canonical.equals(UTF_8)) {
      return Io.utf8StringStrict(bytes);
    }
    return decodeOther(bytes, canonical, true);
  }

  private static Optional<String> decodeOther(byte[] bytes, String canonical, boolean strict) {
    if (canonical.equals(UTF_16) || canonical.equals(UTF_16BE) || canonical.equals(UTF_16LE)) {
      return decodeUtf16(bytes, canonical, strict);
    }
    boolean ascii = canonical.equals(US_ASCII);
    boolean windows = canonical.equals(WINDOWS_1252);
    StringBuilder text = new StringBuilder(bytes.length);
    for (byte signed : bytes) {
      int b = signed & 0xFF;
      char c = (char) b;
      if (b >= 0x80 && ascii) {
        c = REPLACEMENT;
      } else if (b >= 0x80 && b < 0xA0 && windows) {
        c = WINDOWS_1252_HIGH.charAt(b - 0x80);
      }
      if (c == REPLACEMENT && strict) {
        return Optional.empty();
      }
      text.append(c);
    }
    return Optional.of(text.toString());
  }

  /** Invalid: half a surrogate pair on its own, and a single last byte. */
  private static Optional<String> decodeUtf16(byte[] bytes, String canonical, boolean strict) {
    boolean little = canonical.equals(UTF_16LE);
    int start = 0;
    if (canonical.equals(UTF_16) && bytes.length >= 2) {
      int mark = ((bytes[0] & 0xFF) << 8) | (bytes[1] & 0xFF);
      if (mark == 0xFEFF || mark == 0xFFFE) {
        little = mark == 0xFFFE;
        start = 2;
      }
    }
    StringBuilder text = new StringBuilder(bytes.length / 2 + 1);
    for (int i = start; i + 1 < bytes.length; i += 2) {
      int first = bytes[little ? i + 1 : i] & 0xFF;
      int second = bytes[little ? i : i + 1] & 0xFF;
      text.append((char) ((first << 8) | second));
    }
    boolean valid = (bytes.length - start) % 2 == 0;
    if (!valid) {
      text.append(REPLACEMENT);
    }
    for (int i = 0; i < text.length(); i++) {
      char c = text.charAt(i);
      if (Character.isHighSurrogate(c)
          && i + 1 < text.length()
          && Character.isLowSurrogate(text.charAt(i + 1))) {
        i++;
      } else if (Character.isSurrogate(c)) {
        text.setCharAt(i, REPLACEMENT);
        valid = false;
      }
    }
    if (strict && !valid) {
      return Optional.empty();
    }
    return Optional.of(text.toString());
  }

  /**
   * The bytes of the text. A character the charset does not have becomes '?'; in the UTF-16
   * charsets, where only half a surrogate pair on its own is such a character, U+FFFD. UTF-16
   * starts a non-empty text with the byte order mark FE FF.
   */
  public static byte[] encode(String text, String charset) {
    String canonical = name(charset);
    if (canonical.equals(UTF_8)) {
      return Io.utf8Bytes(text);
    }
    if (canonical.equals(UTF_16) || canonical.equals(UTF_16BE) || canonical.equals(UTF_16LE)) {
      return encodeUtf16(text, canonical);
    }
    // One byte for each character; a surrogate pair is one character.
    byte[] bytes = new byte[text.codePointCount(0, text.length())];
    int n = 0;
    for (int i = 0; i < text.length(); i++) {
      char c = text.charAt(i);
      if (Character.isHighSurrogate(c)
          && i + 1 < text.length()
          && Character.isLowSurrogate(text.charAt(i + 1))) {
        i++;
        bytes[n] = '?';
      } else {
        int b = singleByte(c, canonical);
        bytes[n] = (byte) (b < 0 ? '?' : b);
      }
      n++;
    }
    return bytes;
  }

  private static byte[] encodeUtf16(String text, String canonical) {
    boolean little = canonical.equals(UTF_16LE);
    boolean mark = canonical.equals(UTF_16) && !text.isEmpty();
    int start = mark ? 2 : 0;
    byte[] bytes = new byte[start + 2 * text.length()];
    if (mark) {
      bytes[0] = (byte) 0xFE;
      bytes[1] = (byte) 0xFF;
    }
    for (int i = 0; i < text.length(); i++) {
      char c = text.charAt(i);
      boolean firstOfPair =
          Character.isHighSurrogate(c)
              && i + 1 < text.length()
              && Character.isLowSurrogate(text.charAt(i + 1));
      boolean secondOfPair =
          Character.isLowSurrogate(c) && i > 0 && Character.isHighSurrogate(text.charAt(i - 1));
      if (Character.isSurrogate(c) && !firstOfPair && !secondOfPair) {
        c = REPLACEMENT;
      }
      bytes[start + 2 * i + (little ? 1 : 0)] = (byte) (c >> 8);
      bytes[start + 2 * i + (little ? 0 : 1)] = (byte) c;
    }
    return bytes;
  }

  /** The byte of the char in a single-byte charset; -1 if the charset does not have it. */
  private static int singleByte(char c, String canonical) {
    if (c < 0x80 || (c < 0x100 && canonical.equals(ISO_8859_1))) {
      return c;
    }
    if (canonical.equals(WINDOWS_1252)) {
      if (c >= 0xA0 && c < 0x100) {
        return c;
      }
      int high = c == REPLACEMENT ? -1 : WINDOWS_1252_HIGH.indexOf(c);
      if (high >= 0) {
        return 0x80 + high;
      }
    }
    return -1;
  }

  /**
   * Whether the charset (a canonical name) can represent the char on its own: never half a
   * surrogate pair; every other char in the UTF charsets; in a single-byte charset, the chars it
   * has a byte for.
   */
  public static boolean canEncode(char c, String canonical) {
    if (Character.isSurrogate(c)) {
      return false;
    }
    if (canonical.equals(US_ASCII)
        || canonical.equals(ISO_8859_1)
        || canonical.equals(WINDOWS_1252)) {
      return singleByte(c, canonical) >= 0;
    }
    return true;
  }
}
