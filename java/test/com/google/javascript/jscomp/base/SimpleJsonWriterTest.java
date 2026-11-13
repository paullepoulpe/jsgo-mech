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
import static org.junit.Assert.assertThrows;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class SimpleJsonWriterTest {

  private static String string(String value, boolean htmlSafe) {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writer.setHtmlSafe(htmlSafe);
    writer.stringValue(Optional.ofNullable(value));
    return writer.toString();
  }

  @Test
  public void compactByDefault() {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writer.beginObject();
    writer.name("a").beginArray().longValue(1).longValue(-2).endArray();
    writer.name("b").beginObject().endObject();
    writer.name("c").beginArray().endArray();
    writer.name("d").booleanValue(true).name("e").booleanValue(false);
    writer.name("f").nullValue().name("g").stringValue(Optional.of("s")).name("h").rawValue(Optional.of("{\"x\": 1}"));
    writer.endObject();

    assertThat(writer.toString())
        .isEqualTo(
            "{\"a\":[1,-2],\"b\":{},\"c\":[],\"d\":true,\"e\":false,\"f\":null,\"g\":\"s\","
                + "\"h\":{\"x\": 1}}");
  }

  @Test
  public void indented() {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writer.setIndent("  ");
    writer.beginArray();
    writer.beginObject().name("a").longValue(1).name("b").beginArray().endArray().endObject();
    writer.beginArray().stringValue(Optional.of("x")).nullValue().endArray();
    writer.beginObject().endObject();
    writer.endArray();

    assertThat(writer.toString())
        .isEqualTo(
            """
            [
              {
                "a": 1,
                "b": []
              },
              [
                "x",
                null
              ],
              {}
            ]""");
  }

  @Test
  public void longs() {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writer.beginArray().longValue(0).longValue(Long.MIN_VALUE).longValue(Long.MAX_VALUE).endArray();
    assertThat(writer.toString()).isEqualTo("[0,-9223372036854775808,9223372036854775807]");
  }

  @Test
  public void escapes() {
    assertThat(string("\" \\ / \t \b \n \r \f", false))
        .isEqualTo("\"\\\" \\\\ / \\t \\b \\n \\r \\f\"");
    assertThat(string("\u0000\u0001\u000b\u001f\u007f", false))
        .isEqualTo("\"\\u0000\\u0001\\u000b\\u001f\u007f\"");
    assertThat(string("\u2027\u2028\u2029\u202a", false)).isEqualTo("\"\u2027\\u2028\\u2029\u202a\"");
    assertThat(string("caf\u00e9 \ud83d\ude00 \ud83d", false))
        .isEqualTo("\"caf\u00e9 \ud83d\ude00 \ud83d\"");
    assertThat(string("</script> <!-- a=b & 'c'", false)).isEqualTo("\"</script> <!-- a=b & 'c'\"");
  }

  @Test
  public void htmlSafeEscapes() {
    assertThat(string("</script> a=b & 'c'", true))
        .isEqualTo("\"\\u003c/script\\u003e a\\u003db \\u0026 \\u0027c\\u0027\"");
    assertThat(string("\"\n\u2028", true)).isEqualTo("\"\\\"\\n\\u2028\"");

    SimpleJsonWriter writer = new SimpleJsonWriter();
    assertThat(writer.isHtmlSafe()).isFalse();
    writer.setHtmlSafe(true);
    assertThat(writer.isHtmlSafe()).isTrue();
    writer.beginObject().name("<").stringValue(Optional.of(">")).endObject();
    assertThat(writer.toString()).isEqualTo("{\"\\u003c\":\"\\u003e\"}");
  }

  @Test
  public void nullMembersAreWrittenUnlessSerializeNullsIsOff() {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    assertThat(writer.getSerializeNulls()).isTrue();
    writer.beginObject();
    writer.name("a").nullValue().name("b").stringValue(Optional.empty()).name("c").rawValue(Optional.empty());
    writer.setSerializeNulls(false);
    assertThat(writer.getSerializeNulls()).isFalse();
    writer.name("d").nullValue().name("e").stringValue(Optional.empty()).name("f").rawValue(Optional.empty());
    writer.name("g").beginArray().nullValue().stringValue(Optional.empty()).endArray();
    writer.endObject();

    assertThat(writer.toString()).isEqualTo("{\"a\":null,\"b\":null,\"c\":null,\"g\":[null,null]}");
  }

  @Test
  public void settingsMayChangeWhileWriting() {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writer.beginArray().stringValue(Optional.of("<"));
    writer.setHtmlSafe(true);
    writer.stringValue(Optional.of("<"));
    writer.setHtmlSafe(false);
    writer.stringValue(Optional.of("<")).endArray();

    assertThat(writer.toString()).isEqualTo("[\"<\",\"\\u003c\",\"<\"]");
  }

  @Test
  public void severalTopLevelValuesAreWrittenOneAfterTheOther() {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writer.longValue(1).stringValue(Optional.of("a")).beginArray().endArray();
    assertThat(writer.toString()).isEqualTo("1\"a\"[]");
  }

  @Test
  public void takeTextReturnsAndDropsWhatWasWritten() {
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writer.beginArray().longValue(1);
    assertThat(writer.takeText()).isEqualTo("[1");
    assertThat(writer.toString()).isEmpty();
    writer.longValue(2).endArray();
    assertThat(writer.takeText()).isEqualTo(",2]");
    assertThat(writer.takeText()).isEmpty();
  }

  @Test
  public void aSinkGetsTheTextInPiecesThatAddUpToTheWholeText() {
    for (String indent : new String[] {"", "  "}) {
      SimpleJsonWriter whole = new SimpleJsonWriter();
      SimpleJsonWriter pieces = new SimpleJsonWriter();
      List<String> handedOver = new ArrayList<>();
      pieces.setSink(handedOver::add);
      for (SimpleJsonWriter writer : new SimpleJsonWriter[] {whole, pieces}) {
        writer.setIndent(indent);
        writer.beginObject();
        for (int i = 0; i < 30000; i++) {
          writer.name("key<" + i).beginArray().stringValue(Optional.ofNullable("value " + i)).nullValue().endArray();
        }
        writer.endObject();
      }

      assertThat(handedOver.size()).isAtLeast(5);
      for (String piece : handedOver) {
        assertThat(piece.length()).isAtLeast(SimpleJsonWriter.SINK_THRESHOLD);
        assertThat(piece.length()).isLessThan(SimpleJsonWriter.SINK_THRESHOLD + 100);
      }
      assertThat(String.join("", handedOver) + pieces.takeText()).isEqualTo(whole.toString());
    }
  }

  @Test
  public void misuseIsABug() {
    assertThrows(IllegalStateException.class, () -> new SimpleJsonWriter().endArray());
    assertThrows(IllegalStateException.class, () -> new SimpleJsonWriter().beginArray().endObject());
    assertThrows(
        IllegalStateException.class, () -> new SimpleJsonWriter().beginObject().longValue(1));
    assertThrows(
        IllegalStateException.class, () -> new SimpleJsonWriter().beginArray().name("a").longValue(1));
    assertThrows(
        IllegalStateException.class, () -> new SimpleJsonWriter().beginObject().name("a").name("b"));
    assertThrows(
        IllegalStateException.class, () -> new SimpleJsonWriter().beginObject().name("a").endObject());
    assertThrows(NullPointerException.class, () -> new SimpleJsonWriter().beginObject().name(null));
  }
}
