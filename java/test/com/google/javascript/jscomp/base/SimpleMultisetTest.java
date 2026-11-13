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
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class SimpleMultisetTest {

  private static List<String> entries(SimpleMultiset<String> m) {
    List<String> result = new ArrayList<>();
    for (Map.Entry<String, Integer> e : m.entrySet()) {
      result.add(e.getKey() + ":" + e.getValue());
    }
    return result;
  }

  @Test
  public void create_elementsIterateInFirstInsertionOrder() {
    SimpleMultiset<String> m = SimpleMultiset.create();
    m.add("b");
    m.add("a");
    m.add("b");
    m.addAll(ImmutableList.of("c", "a", "b"));

    assertThat(m.elementSet()).containsExactly("b", "a", "c").inOrder();
    assertThat(entries(m)).containsExactly("b:3", "a:2", "c:1").inOrder();
    assertThat(m).containsExactly("b", "b", "b", "a", "a", "c").inOrder();
    assertThat(m.asList()).containsExactly("b", "b", "b", "a", "a", "c").inOrder();
    assertThat(m.size()).isEqualTo(6);
    assertThat(m.count("b")).isEqualTo(3);
    assertThat(m.count("zz")).isEqualTo(0);
    assertThat(m.contains("a")).isTrue();
    assertThat(m.contains("zz")).isFalse();
    assertThat(m.toString()).isEqualTo("[b x 3, a x 2, c]");
  }

  @Test
  public void createSorted_elementsIterateInNaturalOrder() {
    SimpleMultiset<String> m = SimpleMultiset.createSorted();
    m.addAll(ImmutableList.of("b", "c", "a", "b"));

    assertThat(m.elementSet()).containsExactly("a", "b", "c").inOrder();
    assertThat(m).containsExactly("a", "b", "b", "c").inOrder();
    assertThat(m.toString()).isEqualTo("[a, b x 2, c]");
  }

  @Test
  public void addWithOccurrencesReturnsThePreviousCount() {
    SimpleMultiset<String> m = SimpleMultiset.create();

    assertThat(m.add("a", 2)).isEqualTo(0);
    assertThat(m.add("a", 3)).isEqualTo(2);
    assertThat(m.add("b", 0)).isEqualTo(0);

    assertThat(m.count("a")).isEqualTo(5);
    assertThat(m.contains("b")).isFalse();
    assertThat(m.size()).isEqualTo(5);
    assertThrows(IllegalArgumentException.class, () -> m.add("a", -1));
  }

  @Test
  public void setCountOnlyAppliesWhenTheOldCountMatches() {
    SimpleMultiset<String> m = SimpleMultiset.create();

    assertThat(m.setCount("a", 0, 1)).isTrue();
    assertThat(m.setCount("a", 0, 1)).isFalse();
    assertThat(m.count("a")).isEqualTo(1);
    assertThat(m.setCount("a", 1, 4)).isTrue();
    assertThat(m.size()).isEqualTo(4);
    assertThat(m.setCount("a", 4, 0)).isTrue();
    assertThat(m.contains("a")).isFalse();
    assertThat(m.isEmpty()).isTrue();
  }

  @Test
  public void removeTakesOneOccurrence_andForgetsElementsWithoutAny() {
    SimpleMultiset<String> m = SimpleMultiset.create();
    m.addAll(ImmutableList.of("a", "b", "a"));

    assertThat(m.remove("a")).isTrue();
    assertThat(m.count("a")).isEqualTo(1);
    assertThat(m.remove("a")).isTrue();
    assertThat(m.remove("a")).isFalse();
    m.add("a");

    assertThat(m.elementSet()).containsExactly("b", "a").inOrder();
    assertThat(m.size()).isEqualTo(2);
  }

  @Test
  public void removeAllRemovesEveryOccurrence() {
    SimpleMultiset<String> m = SimpleMultiset.create();
    m.addAll(ImmutableList.of("a", "b", "a", "c"));

    assertThat(m.removeAll(ImmutableList.of("a", "zz"))).isTrue();
    assertThat(m.removeAll(ImmutableList.of("a"))).isFalse();

    assertThat(m).containsExactly("b", "c").inOrder();
    assertThat(m.size()).isEqualTo(2);
  }

  @Test
  public void viewsAreLiveAndUnmodifiable() {
    SimpleMultiset<String> m = SimpleMultiset.create();
    m.add("a");
    var elements = m.elementSet();
    var entries = m.entrySet();

    m.add("b");

    assertThat(elements).containsExactly("a", "b").inOrder();
    assertThat(entries).hasSize(2);
    assertThrows(UnsupportedOperationException.class, () -> elements.remove("a"));
    assertThrows(UnsupportedOperationException.class, entries::clear);
    assertThrows(
        UnsupportedOperationException.class, () -> entries.iterator().next().setValue(5));
  }

  @Test
  public void immutableMultisetsRejectMutation() {
    SimpleMultiset<String> m = SimpleMultiset.createSorted();
    m.addAll(ImmutableList.of("b", "a", "b"));
    SimpleMultiset<String> copy = m.immutableCopy();
    m.add("c");

    assertThat(copy).containsExactly("a", "b", "b").inOrder();
    assertThat(copy.size()).isEqualTo(3);
    assertThrows(IllegalStateException.class, () -> copy.add("a"));
    assertThrows(IllegalStateException.class, () -> copy.add("a", 2));
    assertThrows(IllegalStateException.class, () -> copy.addAll(ImmutableList.of("a")));
    assertThrows(IllegalStateException.class, () -> copy.remove("a"));
    assertThrows(IllegalStateException.class, () -> copy.removeAll(ImmutableList.of("a")));
    assertThrows(IllegalStateException.class, () -> copy.setCount("a", 1, 2));

    assertThat(SimpleMultiset.of()).isEmpty();
    assertThrows(IllegalStateException.class, () -> SimpleMultiset.of().add("a"));

    SimpleMultiset<String> copyOf = SimpleMultiset.copyOf(ImmutableList.of("b", "a", "b"));
    assertThat(copyOf).containsExactly("b", "b", "a").inOrder();
    assertThrows(IllegalStateException.class, () -> copyOf.add("a"));
  }

  @Test
  public void equalsAndHashCodeIgnoreOrder() {
    SimpleMultiset<String> m1 = SimpleMultiset.create();
    m1.addAll(ImmutableList.of("b", "a", "b"));
    SimpleMultiset<String> m2 = SimpleMultiset.createSorted();
    m2.addAll(ImmutableList.of("a", "b", "b"));
    SimpleMultiset<String> m3 = SimpleMultiset.create();
    m3.addAll(ImmutableList.of("a", "b"));

    assertThat(m1).isEqualTo(m2);
    assertThat(m1.hashCode()).isEqualTo(m2.hashCode());
    assertThat(m1).isEqualTo(m1.immutableCopy());
    assertThat(m1).isNotEqualTo(m3);
    assertThat(m1).isNotEqualTo(m1.asList());
  }
}
