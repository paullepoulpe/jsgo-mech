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
import java.util.Iterator;
import java.util.Set;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class SimpleSetMultimapTest {

  private static void fill(SimpleSetMultimap<String, Integer> m) {
    m.put("b", 2);
    m.put("a", 9);
    m.put("b", 1);
    m.put("a", 3);
    m.put("c", 5);
    m.put("b", 7);
  }

  @Test
  public void create_valuesIterateKeyByKey() {
    SimpleSetMultimap<String, Integer> m = SimpleSetMultimap.create();
    fill(m);

    assertThat(m.keySet()).containsExactly("b", "a", "c").inOrder();
    assertThat(m.get("b")).containsExactly(2, 1, 7).inOrder();
    assertThat(m.values()).containsExactly(2, 1, 7, 9, 3, 5).inOrder();
    assertThat(m.toString()).isEqualTo("{b=[2, 1, 7], a=[9, 3], c=[5]}");
    assertThat(m.size()).isEqualTo(6);
    assertThat(m.values()).hasSize(6);
  }

  @Test
  public void createInsertionOrdered_valuesIterateInInsertionOrderOfPairs() {
    SimpleSetMultimap<String, Integer> m = SimpleSetMultimap.createInsertionOrdered();
    fill(m);

    assertThat(m.keySet()).containsExactly("b", "a", "c").inOrder();
    assertThat(m.get("b")).containsExactly(2, 1, 7).inOrder();
    assertThat(m.values()).containsExactly(2, 9, 1, 3, 5, 7).inOrder();
    assertThat(m.asMap().keySet()).containsExactly("b", "a", "c").inOrder();
    assertThat(m.toString()).isEqualTo("{b=[2, 1, 7], a=[9, 3], c=[5]}");

    m.removeAll("a");
    m.put("a", 3);
    assertThat(m.values()).containsExactly(2, 1, 5, 7, 3).inOrder();
    assertThat(m.keySet()).containsExactly("b", "c", "a").inOrder();

    m.clear();
    assertThat(m.values()).isEmpty();
    assertThat(m.isEmpty()).isTrue();
  }

  @Test
  public void createInsertionOrdered_valuesIteratorRemoves() {
    SimpleSetMultimap<String, Integer> m = SimpleSetMultimap.createInsertionOrdered();
    fill(m);

    Iterator<Integer> values = m.values().iterator();
    while (values.hasNext()) {
      if (values.next() > 4) {
        values.remove();
      }
    }

    assertThat(m.toString()).isEqualTo("{b=[2, 1], a=[3]}");
    assertThat(m.values()).containsExactly(2, 1, 3).inOrder();
    assertThat(m.containsKey("c")).isFalse();
    assertThat(m.containsEntry("a", 9)).isFalse();
  }

  @Test
  public void createSorted_keysAndValuesIterateInNaturalOrder() {
    SimpleSetMultimap<String, Integer> m = SimpleSetMultimap.createSorted();
    fill(m);

    assertThat(m.keySet()).containsExactly("a", "b", "c").inOrder();
    assertThat(m.get("b")).containsExactly(1, 2, 7).inOrder();
    assertThat(m.values()).containsExactly(3, 9, 1, 2, 7, 5).inOrder();
    assertThat(m.toString()).isEqualTo("{a=[3, 9], b=[1, 2, 7], c=[5]}");
  }

  @Test
  public void duplicatePairsAreIgnored() {
    SimpleSetMultimap<String, Integer> m = SimpleSetMultimap.createInsertionOrdered();

    assertThat(m.put("a", 1)).isTrue();
    assertThat(m.put("b", 2)).isTrue();
    assertThat(m.put("a", 1)).isFalse();
    m.putAll("a", ImmutableList.of(1, 3, 3));

    assertThat(m.get("a")).containsExactly(1, 3).inOrder();
    assertThat(m.values()).containsExactly(1, 2, 3).inOrder();
    assertThat(m.containsEntry("a", 3)).isTrue();
    assertThat(m.containsEntry("b", 3)).isFalse();
    assertThat(m.containsEntry("zz", 3)).isFalse();
  }

  @Test
  public void putAllOfAMultimapFollowsTheOrderOfItsValues() {
    SimpleSetMultimap<String, Integer> source = SimpleSetMultimap.createInsertionOrdered();
    fill(source);
    SimpleSetMultimap<String, Integer> grouped = SimpleSetMultimap.create();
    fill(grouped);

    SimpleSetMultimap<String, Integer> m1 = SimpleSetMultimap.createInsertionOrdered();
    m1.putAll(source);
    SimpleSetMultimap<String, Integer> m2 = SimpleSetMultimap.createInsertionOrdered();
    m2.putAll(grouped);

    assertThat(m1.values()).containsExactly(2, 9, 1, 3, 5, 7).inOrder();
    assertThat(m2.values()).containsExactly(2, 1, 7, 9, 3, 5).inOrder();
    assertThat(m1).isEqualTo(m2);
  }

  @Test
  public void removeAllForgetsTheKey() {
    SimpleSetMultimap<String, Integer> m = SimpleSetMultimap.create();
    fill(m);

    assertThat(m.removeAll("b")).containsExactly(2, 1, 7).inOrder();
    assertThat(m.removeAll("b")).isEmpty();
    m.put("b", 4);

    assertThat(m.keySet()).containsExactly("a", "c", "b").inOrder();
  }

  @Test
  public void viewsAreLiveAndUnmodifiable() {
    SimpleSetMultimap<String, Integer> m = SimpleSetMultimap.create();
    m.put("a", 1);
    Set<Integer> a = m.get("a");
    var keys = m.keySet();
    var values = m.values();
    var map = m.asMap();

    m.put("a", 2);
    m.put("b", 3);

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
  }

  @Test
  public void immutableCopyKeepsKeyOrderAndRejectsMutation() {
    SimpleSetMultimap<String, Integer> m = SimpleSetMultimap.createInsertionOrdered();
    fill(m);
    SimpleSetMultimap<String, Integer> copy = m.immutableCopy();
    m.put("d", 4);

    assertThat(copy.toString()).isEqualTo("{b=[2, 1, 7], a=[9, 3], c=[5]}");
    assertThat(copy.values()).containsExactly(2, 1, 7, 9, 3, 5).inOrder();
    assertThrows(IllegalStateException.class, () -> copy.put("a", 1));
    assertThrows(IllegalStateException.class, () -> copy.putAll("a", ImmutableList.of(1)));
    assertThrows(IllegalStateException.class, () -> copy.putAll(m));
    assertThrows(IllegalStateException.class, () -> copy.removeAll("a"));
    assertThrows(IllegalStateException.class, copy::clear);

    SimpleSetMultimap<String, Integer> sortedCopy = SimpleSetMultimap.createSorted();
    fill(sortedCopy);
    assertThat(sortedCopy.immutableCopy().toString()).isEqualTo("{a=[3, 9], b=[1, 2, 7], c=[5]}");
  }

  @Test
  public void inverseSwapsKeysAndValuesKeyByKey() {
    SimpleSetMultimap<String, Integer> m = SimpleSetMultimap.create();
    m.put("b", 2);
    m.put("a", 1);
    m.put("b", 1);
    m.put("c", 2);

    SimpleSetMultimap<Integer, String> inverse = m.inverse();

    assertThat(inverse.toString()).isEqualTo("{2=[b, c], 1=[b, a]}");
    assertThrows(IllegalStateException.class, () -> inverse.put(1, "z"));
  }

  @Test
  public void equalsAndHashCodeIgnoreOrderAndFlavour() {
    SimpleSetMultimap<String, Integer> m1 = SimpleSetMultimap.create();
    fill(m1);
    SimpleSetMultimap<String, Integer> m2 = SimpleSetMultimap.createInsertionOrdered();
    fill(m2);
    SimpleSetMultimap<String, Integer> m3 = SimpleSetMultimap.createSorted();
    fill(m3);

    assertThat(m1).isEqualTo(m2);
    assertThat(m1).isEqualTo(m3);
    assertThat(m1).isEqualTo(m1.immutableCopy());
    assertThat(m1.hashCode()).isEqualTo(m3.hashCode());
    m3.put("z", 0);
    assertThat(m1).isNotEqualTo(m3);
    assertThat(m1).isNotEqualTo(m1.asMap());
  }
}
