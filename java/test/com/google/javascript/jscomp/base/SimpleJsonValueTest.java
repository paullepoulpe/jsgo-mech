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

import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class SimpleJsonValueTest {

  @Test
  public void kinds() {
    assertThat(SimpleJsonValue.newObject().kind()).isEqualTo(SimpleJsonValue.Kind.OBJECT);
    assertThat(SimpleJsonValue.newArray().kind()).isEqualTo(SimpleJsonValue.Kind.ARRAY);
    assertThat(SimpleJsonValue.ofString("1").kind()).isEqualTo(SimpleJsonValue.Kind.STRING);
    assertThat(SimpleJsonValue.ofNumber("1").kind()).isEqualTo(SimpleJsonValue.Kind.NUMBER);
    assertThat(SimpleJsonValue.ofBoolean(true).kind()).isEqualTo(SimpleJsonValue.Kind.BOOLEAN);
    assertThat(SimpleJsonValue.ofNull().kind()).isEqualTo(SimpleJsonValue.Kind.NULL);
  }

  @Test
  public void membersKeepInsertionOrderAndPutReplacesInPlace() {
    SimpleJsonValue object = SimpleJsonValue.newObject();
    object.members().put("b", SimpleJsonValue.ofNumber("1"));
    object.members().put("a", SimpleJsonValue.ofNull());
    object.members().put("b", SimpleJsonValue.ofBoolean(false));

    assertThat(object.toString()).isEqualTo("{\"b\":false,\"a\":null}");
    assertThat(object.get("a").kind()).isEqualTo(SimpleJsonValue.Kind.NULL);
    assertThat(object.get("zz")).isNull();
    assertThat(SimpleJsonValue.ofString("x").get("a")).isNull();
    assertThat(SimpleJsonValue.ofString("x").members()).isEmpty();
    assertThat(SimpleJsonValue.ofString("x").elements()).isEmpty();
  }

  @Test
  public void toStringIsCompactAndNotHtmlSafe() {
    SimpleJsonValue array = SimpleJsonValue.newArray();
    array.elements().add(SimpleJsonValue.ofString("<a href='x'>&\n\u2028"));
    array.elements().add(SimpleJsonValue.ofNumber("1.50e+3"));
    array.elements().add(SimpleJsonValue.newObject());
    array.elements().add(SimpleJsonValue.newArray());
    array.elements().add(SimpleJsonValue.ofBoolean(true));
    array.elements().add(SimpleJsonValue.ofNull());

    assertThat(array.toString())
        .isEqualTo("[\"<a href='x'>&\\n\\u2028\",1.50e+3,{},[],true,null]");
  }

  @Test
  public void writeToUsesTheSettingsOfTheWriter() {
    SimpleJsonValue object = SimpleJsonValue.newObject();
    object.members().put("a<", SimpleJsonValue.ofString("="));
    object.members().put("n", SimpleJsonValue.ofNull());
    SimpleJsonValue list = SimpleJsonValue.newArray();
    list.elements().add(SimpleJsonValue.ofNumber("1"));
    object.members().put("l", list);
    SimpleJsonWriter writer = new SimpleJsonWriter();
    writer.setIndent("  ");
    writer.setHtmlSafe(true);
    writer.setSerializeNulls(false);

    object.writeTo(writer);

    assertThat(writer.toString())
        .isEqualTo("{\n  \"a\\u003c\": \"\\u003d\",\n  \"l\": [\n    1\n  ]\n}");
  }

  @Test
  public void asString() {
    assertThat(SimpleJsonValue.ofString("a").asString().get()).isEqualTo("a");
    assertThat(SimpleJsonValue.ofNumber("1.0").asString().get()).isEqualTo("1.0");
    assertThat(SimpleJsonValue.ofBoolean(true).asString().get()).isEqualTo("true");
    assertThat(SimpleJsonValue.ofBoolean(false).asString().get()).isEqualTo("false");
    assertThat(SimpleJsonValue.ofNull().asString().errorMessage())
        .isEqualTo("Expected a JSON string but found NULL");
    assertThat(SimpleJsonValue.newArray().asString().error())
        .isInstanceOf(SimpleJsonException.class);
    assertThat(SimpleJsonValue.newObject().asString().isErr()).isTrue();
  }

  @Test
  public void asInt() {
    assertThat(SimpleJsonValue.ofNumber("3").asInt().get()).isEqualTo(3);
    assertThat(SimpleJsonValue.ofNumber("-0").asInt().get()).isEqualTo(0);
    assertThat(SimpleJsonValue.ofNumber("-2147483648").asInt().get()).isEqualTo(Integer.MIN_VALUE);
    assertThat(SimpleJsonValue.ofString("12").asInt().get()).isEqualTo(12);
    assertThat(SimpleJsonValue.ofString("+12").asInt().get()).isEqualTo(12);
    assertThat(SimpleJsonValue.ofString("1.0").asInt().isErr()).isTrue();
    assertThat(SimpleJsonValue.ofString("").asInt().isErr()).isTrue();
    assertThat(SimpleJsonValue.ofBoolean(true).asInt().errorMessage())
        .isEqualTo("Expected a JSON integer but found BOOLEAN");
    assertThat(SimpleJsonValue.ofNull().asInt().isErr()).isTrue();
    assertThat(SimpleJsonValue.newArray().asInt().isErr()).isTrue();
  }

  @Test
  public void asIntDropsTheFractionAndWrapsTo32Bits() {
    assertThat(SimpleJsonValue.ofNumber("3.0").asInt().get()).isEqualTo(3);
    assertThat(SimpleJsonValue.ofNumber("3.99").asInt().get()).isEqualTo(3);
    assertThat(SimpleJsonValue.ofNumber("-3.99").asInt().get()).isEqualTo(-3);
    assertThat(SimpleJsonValue.ofNumber("1e2").asInt().get()).isEqualTo(100);
    assertThat(SimpleJsonValue.ofNumber("1.5E+1").asInt().get()).isEqualTo(15);
    assertThat(SimpleJsonValue.ofNumber("1234.5e-2").asInt().get()).isEqualTo(12);
    assertThat(SimpleJsonValue.ofNumber("5e-1").asInt().get()).isEqualTo(0);
    assertThat(SimpleJsonValue.ofNumber("2147483648").asInt().get()).isEqualTo(Integer.MIN_VALUE);
    assertThat(SimpleJsonValue.ofNumber("4294967299").asInt().get()).isEqualTo(3);
    assertThat(SimpleJsonValue.ofNumber("-4294967299").asInt().get()).isEqualTo(-3);
    assertThat(SimpleJsonValue.ofNumber("123456789012345678901234567890").asInt().get())
        .isEqualTo(new java.math.BigInteger("123456789012345678901234567890").intValue());
    assertThat(SimpleJsonValue.ofNumber("1e400").asInt().get()).isEqualTo(0);
    assertThat(SimpleJsonValue.ofNumber("1e99999999999999999999").asInt().get()).isEqualTo(0);
    assertThat(SimpleJsonValue.ofNumber("1e-99999999999999999999").asInt().get()).isEqualTo(0);
  }
}
