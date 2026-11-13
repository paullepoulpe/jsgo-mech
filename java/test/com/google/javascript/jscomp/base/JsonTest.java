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

import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class JsonTest {

  private static String roundTrip(String json) {
    return Json.parse(json).get().toString();
  }

  @Test
  public void acceptsStandardJson() {
    String[] documents = {
      "{}", "[]", "\"\"", "0", "-0", "true", "false", "null", " \t\r\n[ 1 , 2 ]\n", "{\"a\":{\"b\":[]}}",
      "1.5", "1e9", "1E+9", "1.25e-3", "-12", "12345678901234567890", "1e400",
      "\"\\\" \\\\ \\/ \\b \\f \\n \\r \\t \\u00e9 \\u00E9\"", "\"\u007f\u2028 caf\u00e9 \ud83d\ude00\"",
      "[null,true,false,\"x\",1,{},[]]",
    };
    for (String document : documents) {
      assertWithMessage(document).that(Json.parse(document).isOk()).isTrue();
    }
  }

  @Test
  public void rejectsEverythingElse() {
    String[] documents = {
      "", " ", "{", "[", "}", "[1,]", "[,1]", "[1 2]", "{\"a\":1,}", "{\"a\" 1}", "{\"a\":}", "{a:1}",
      "{'a':1}", "'a'", "abc", "[1;2]", "{\"a\"=1}", "// c\n1", "/* c */1", "# c\n1", "1 // c",
      "NaN", "Infinity", "-Infinity", "+1", "01", "-01", ".5", "1.", "1e", "1e+", "0x10", "--1", "- 1",
      "TRUE", "True", "nulL", "NULL", "tru", "truex", "nullnull", "fals",
      "\"a", "\"a\\\"", "\"\\x\"", "\"\\'\"", "\"\\u12\"", "\"\\u12zz\"", "\"\\U0041\"", "\"a\nb\"",
      "\"a\tb\"", "\"a\u0000b\"", "\"\\\n\"", "1 2", "{} {}", "[] x", "null,", ")]}'\n{}", "\ufeff\ufeff1",
      "[\"a\" \"b\"]", "{\"a\":1 \"b\":2}", "{1:2}", "{\"a\":1}}", "[1]]", "\u00a01", "1\f",
    };
    for (String document : documents) {
      Outcome<SimpleJsonValue> parsed = Json.parse(document);
      assertWithMessage(document).that(parsed.isErr()).isTrue();
      assertThat(parsed.error()).isInstanceOf(SimpleJsonException.class);
      assertThat(parsed.errorMessage()).isEqualTo("Malformed JSON");
    }
  }

  @Test
  public void aLeadingByteOrderMarkIsSkipped() {
    assertThat(roundTrip("\ufeff{\"a\":1}")).isEqualTo("{\"a\":1}");
    assertThat(Json.parse("{\ufeff}").isErr()).isTrue();
  }

  @Test
  public void membersKeepTheirOrderAndARepeatedNameKeepsTheFirstPositionAndTheLastValue() {
    SimpleJsonValue value = Json.parse("{\"b\":1,\"a\":2,\"c\":3,\"a\":[4],\"\":5}").get();
    assertThat(value.members().keySet()).containsExactly("b", "a", "c", "").inOrder();
    assertThat(value.toString()).isEqualTo("{\"b\":1,\"a\":[4],\"c\":3,\"\":5}");
  }

  @Test
  public void numbersKeepTheirSourceText() {
    assertThat(roundTrip("[-0, 1.50, 1E+2, 1e400, 12345678901234567890, 0.1e-7]"))
        .isEqualTo("[-0,1.50,1E+2,1e400,12345678901234567890,0.1e-7]");
    assertThat(Json.parse("12").get().kind()).isEqualTo(SimpleJsonValue.Kind.NUMBER);
    assertThat(Json.parse("\"12\"").get().kind()).isEqualTo(SimpleJsonValue.Kind.STRING);
  }

  @Test
  public void aNumberMustBeShorterThanTheLimit() {
    String longest = "0." + "1".repeat(Json.MAX_NUMBER_LENGTH - 3);
    assertThat(Json.parse(longest).isOk()).isTrue();
    assertThat(Json.parse("[ " + longest + "]").isOk()).isTrue();
    assertThat(Json.parse(longest + "1").isErr()).isTrue();
    assertThat(Json.parse("[ " + longest + "1]").isErr()).isTrue();
    assertThat(Json.parse("1e" + "0".repeat(2000)).isErr()).isTrue();
  }

  @Test
  public void theDigitsBeforeTheFractionAreLimited() {
    String digits = "9".repeat(Json.MAX_INTEGER_DIGITS);
    assertThat(Json.parse(digits).isOk()).isTrue();
    assertThat(Json.parse("[-" + digits + ".5e" + digits + "]").isOk()).isTrue();
    assertThat(Json.parse("0." + digits + digits).isOk()).isTrue();
    assertThat(Json.parse("\"" + digits + digits + "\"").isOk()).isTrue();
    assertThat(Json.parse(digits + "9").isErr()).isTrue();
    assertThat(Json.parse("[1, -" + digits + "9.5]").isErr()).isTrue();
    assertThat(Json.parse("922337203685477580801").isErr()).isTrue();
  }

  @Test
  public void nestingIsLimited() {
    int max = Json.MAX_DEPTH;
    assertThat(Json.parse("[".repeat(max) + "]".repeat(max)).isOk()).isTrue();
    assertThat(Json.parse("[".repeat(max + 1) + "]".repeat(max + 1)).isErr()).isTrue();
    assertThat(Json.parse("{\"a\":".repeat(max) + "1" + "}".repeat(max)).isOk()).isTrue();
    assertThat(Json.parse("{\"a\":".repeat(max + 1) + "1" + "}".repeat(max + 1)).isErr()).isTrue();
    assertThat(Json.parse("[".repeat(100000)).isErr()).isTrue();
  }

  @Test
  public void stringsAreUnescaped() {
    SimpleJsonValue value =
        Json.parse("\"\\\" \\\\ \\/ \\b \\f \\n \\r \\t \\u0041 \\ud83d\\ude00 \ud83d\ude00\"").get();
    assertThat(value.asString().get()).isEqualTo("\" \\ / \b \f \n \r \t A \ud83d\ude00 \ud83d\ude00");
  }

  @Test
  public void aSurrogateWithoutItsPairIsReplaced() {
    assertThat(Json.parse("\"a\\ud83db\"").get().asString().get()).isEqualTo("a\ufffdb");
    assertThat(Json.parse("\"\\ude00\\ud83d\"").get().asString().get()).isEqualTo("\ufffd\ufffd");
    assertThat(Json.parse("\"a\ud83d\"").get().asString().get()).isEqualTo("a\ufffd");
    assertThat(Json.parse("{\"\\ud83d\":1}").get().members().keySet()).containsExactly("\ufffd");
    assertThat(Json.parse("\"\\ud83d\ude00\"").get().asString().get()).isEqualTo("\ud83d\ude00");
  }
}
