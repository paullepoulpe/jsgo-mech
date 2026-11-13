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

import com.google.common.collect.ImmutableList;
import java.util.List;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class SimpleListMultimapTest {

  @Test
  public void keysIterateInFirstInsertionOrder_valuesKeyByKey() {
    SimpleListMultimap<String, Integer> m = SimpleListMultimap.create();
    m.put("b", 2).put("a", 1).put("b", 1).putAll("c", ImmutableList.of(9, 8));
    m.put("a", 3);

    assertThat(m.keySet()).containsExactly("b", "a", "c").inOrder();
    assertThat(m.values()).containsExactly(2, 1, 1, 3, 9, 8).inOrder();
    assertThat(m.asMap().keySet()).containsExactly("b", "a", "c").inOrder();
    assertThat(m.asMap().get("b")).containsExactly(2, 1).inOrder();
    assertThat(m.size()).isEqualTo(6);
    assertThat(m.values()).hasSize(6);
    assertThat(m.toString()).isEqualTo("{b=[2, 1], a=[1, 3], c=[9, 8]}");
  }

  @Test
  public void duplicatesAreKept() {
    SimpleListMultimap<String, Integer> m = SimpleListMultimap.create();
    m.put("a", 1).put("a", 1);

    assertThat(m.get("a")).containsExactly(1, 1);
    assertThat(m.size()).isEqualTo(2);
  }

  @Test
  public void removeTakesFirstOccurrence_andForgetsEmptiedKeys() {
    SimpleListMultimap<String, Integer> m = SimpleListMultimap.create();
    m.put("a", 1).put("a", 2).put("a", 1).put("b", 5);

    assertThat(m.remove("a", 1)).isTrue();
    assertThat(m.get("a")).containsExactly(2, 1).inOrder();
    assertThat(m.remove("a", 7)).isFalse();
    assertThat(m.remove("zz", 1)).isFalse();

    assertThat(m.removeAll("a")).containsExactly(2, 1).inOrder();
    assertThat(m.containsKey("a")).isFalse();
    assertThat(m.removeAll("a")).isEmpty();

    m.put("a", 4);
    assertThat(m.keySet()).containsExactly("b", "a").inOrder();

    assertThat(m.remove("b", 5)).isTrue();
    assertThat(m.keySet()).containsExactly("a");
    assertThat(m.isEmpty()).isFalse();
    m.removeAll("a");
    assertThat(m.isEmpty()).isTrue();
  }

  @Test
  public void replaceValuesKeepsThePositionOfTheKey() {
    SimpleListMultimap<String, Integer> m = SimpleListMultimap.create();
    m.put("a", 1).put("b", 2).put("c", 3);

    m.replaceValues("a", ImmutableList.of(7, 8));
    assertThat(m.toString()).isEqualTo("{a=[7, 8], b=[2], c=[3]}");

    m.replaceValues("b", ImmutableList.of());
    m.replaceValues("d", ImmutableList.of(4));
    assertThat(m.toString()).isEqualTo("{a=[7, 8], c=[3], d=[4]}");
  }

  @Test
  public void viewsAreLiveAndUnmodifiable() {
    SimpleListMultimap<String, Integer> m = SimpleListMultimap.create();
    m.put("a", 1);
    List<Integer> a = m.get("a");
    var keys = m.keySet();
    var values = m.values();
    var map = m.asMap();

    m.put("a", 2).put("b", 3);

    assertThat(a).containsExactly(1, 2).inOrder();
    assertThat(keys).containsExactly("a", "b").inOrder();
    assertThat(values).containsExactly(1, 2, 3).inOrder();
    assertThat(map).hasSize(2);

    assertThat(m.get("absent")).isEmpty();
    assertThrows(UnsupportedOperationException.class, () -> a.add(5));
    assertThrows(UnsupportedOperationException.class, () -> m.get("absent").add(5));
    assertThrows(UnsupportedOperationException.class, () -> keys.remove("a"));
    assertThrows(UnsupportedOperationException.class, () -> map.remove("a"));
    assertThrows(UnsupportedOperationException.class, () -> values.iterator().remove());
    assertThrows(UnsupportedOperationException.class, () -> m.removeAll("a").add(1));
  }

  @Test
  public void immutableCopyIsASnapshotThatRejectsMutation() {
    SimpleListMultimap<String, Integer> m = SimpleListMultimap.create();
    m.put("b", 2).put("a", 1).put("b", 2);
    SimpleListMultimap<String, Integer> copy = m.immutableCopy();
    m.put("c", 3);

    assertThat(copy.toString()).isEqualTo("{b=[2, 2], a=[1]}");
    assertThrows(IllegalStateException.class, () -> copy.put("a", 1));
    assertThrows(IllegalStateException.class, () -> copy.putAll("a", ImmutableList.of(1)));
    assertThrows(IllegalStateException.class, () -> copy.remove("a", 1));
    assertThrows(IllegalStateException.class, () -> copy.removeAll("a"));
    assertThrows(IllegalStateException.class, () -> copy.replaceValues("a", ImmutableList.of()));
  }

  @Test
  public void equalsAndHashCodeIgnoreKeyOrderButNotValueOrder() {
    SimpleListMultimap<String, Integer> m1 = SimpleListMultimap.create();
    m1.put("a", 1).put("a", 2).put("b", 3);
    SimpleListMultimap<String, Integer> m2 = SimpleListMultimap.create();
    m2.put("b", 3).put("a", 1).put("a", 2);
    SimpleListMultimap<String, Integer> m3 = SimpleListMultimap.create();
    m3.put("b", 3).put("a", 2).put("a", 1);

    assertThat(m1).isEqualTo(m2);
    assertThat(m1.hashCode()).isEqualTo(m2.hashCode());
    assertThat(m1).isEqualTo(m1.immutableCopy());
    assertThat(m1).isNotEqualTo(m3);
    assertThat(m1).isNotEqualTo(m1.asMap());
  }
}
