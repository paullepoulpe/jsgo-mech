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

import com.google.common.collect.ImmutableMap;
import java.util.LinkedHashMap;
import java.util.Map;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class SimpleBiMapTest {

  @Test
  public void bothDirectionsIterateInInsertionOrder() {
    SimpleBiMap<String, Integer> m = SimpleBiMap.create();
    m.put("b", 2);
    m.put("a", 9);
    m.put("c", 1);

    assertThat(m.keySet()).containsExactly("b", "a", "c").inOrder();
    assertThat(m.asMap()).containsExactly("b", 2, "a", 9, "c", 1).inOrder();
    assertThat(m.inverse()).containsExactly(2, "b", 9, "a", 1, "c").inOrder();
    assertThat(m.get("a")).isEqualTo(9);
    assertThat(m.get("zz")).isNull();
    assertThat(m.inverse().get(1)).isEqualTo("c");
    assertThat(m.inverse().get(7)).isNull();
    assertThat(m.toString()).isEqualTo("{b=2, a=9, c=1}");
  }

  @Test
  public void putReplacesTheValueOfAKeyInPlace() {
    SimpleBiMap<String, Integer> m = SimpleBiMap.create();
    m.put("b", 2);
    m.put("a", 9);
    m.put("c", 1);

    assertThat(m.put("b", 5)).isEqualTo(2);
    assertThat(m.put("a", 9)).isEqualTo(9);
    assertThat(m.put("d", 2)).isNull();

    assertThat(m.asMap()).containsExactly("b", 5, "a", 9, "c", 1, "d", 2).inOrder();
    assertThat(m.inverse()).containsExactly(5, "b", 9, "a", 1, "c", 2, "d").inOrder();
  }

  @Test
  public void putRejectsAValueThatAnotherKeyHas() {
    SimpleBiMap<String, Integer> m = SimpleBiMap.create();
    m.put("a", 1);
    m.put("b", 2);

    assertThrows(IllegalArgumentException.class, () -> m.put("c", 1));
    assertThrows(IllegalArgumentException.class, () -> m.put("b", 1));

    assertThat(m.asMap()).containsExactly("a", 1, "b", 2).inOrder();
    assertThat(m.inverse()).containsExactly(1, "a", 2, "b").inOrder();
  }

  @Test
  public void copyOfKeepsTheOrderOfTheMapAndRejectsDuplicateValues() {
    Map<String, Integer> source = new LinkedHashMap<>();
    source.put("b", 2);
    source.put("a", 1);

    SimpleBiMap<String, Integer> m = SimpleBiMap.copyOf(source);
    source.put("c", 3);

    assertThat(m.asMap()).containsExactly("b", 2, "a", 1).inOrder();
    assertThat(m.inverse()).containsExactly(2, "b", 1, "a").inOrder();
    assertThrows(
        IllegalArgumentException.class, () -> SimpleBiMap.copyOf(ImmutableMap.of("a", 1, "b", 1)));
  }

  @Test
  public void clearEmptiesBothDirections() {
    SimpleBiMap<String, Integer> m = SimpleBiMap.create();
    m.put("a", 1);

    m.clear();
    m.put("b", 1);

    assertThat(m.asMap()).containsExactly("b", 1);
    assertThat(m.inverse()).containsExactly(1, "b");
  }

  @Test
  public void viewsAreLiveAndUnmodifiable() {
    SimpleBiMap<String, Integer> m = SimpleBiMap.create();
    var keys = m.keySet();
    var map = m.asMap();
    var inverse = m.inverse();

    m.put("a", 1);

    assertThat(keys).containsExactly("a");
    assertThat(map).containsExactly("a", 1);
    assertThat(inverse).containsExactly(1, "a");
    assertThrows(UnsupportedOperationException.class, () -> keys.remove("a"));
    assertThrows(UnsupportedOperationException.class, () -> map.put("b", 2));
    assertThrows(UnsupportedOperationException.class, () -> inverse.put(2, "b"));
  }
}
