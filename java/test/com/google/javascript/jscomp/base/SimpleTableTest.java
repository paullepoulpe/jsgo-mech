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

import java.util.Map;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class SimpleTableTest {

  private static SimpleTable<String, Integer, String> newTable() {
    SimpleTable<String, Integer, String> t = SimpleTable.create();
    t.put("r2", 1, "a");
    t.put("r1", 2, "b");
    t.put("r2", 0, "c");
    t.put("r1", 1, "d");
    return t;
  }

  @Test
  public void rowsAndColumnsIterateInFirstInsertionOrder() {
    SimpleTable<String, Integer, String> t = newTable();

    assertThat(t.rowKeySet()).containsExactly("r2", "r1").inOrder();
    assertThat(t.row("r2").keySet()).containsExactly(1, 0).inOrder();
    assertThat(t.values()).containsExactly("a", "c", "b", "d").inOrder();
    assertThat(t.toString()).isEqualTo("{r2={1=a, 0=c}, r1={2=b, 1=d}}");
  }

  @Test
  public void putReplacesAndReturnsTheOldValue() {
    SimpleTable<String, Integer, String> t = newTable();

    assertThat(t.put("r2", 1, "z")).isEqualTo("a");
    assertThat(t.put("r3", 1, "y")).isNull();

    assertThat(t.get("r2", 1)).isEqualTo("z");
    assertThat(t.values()).containsExactly("z", "c", "b", "d", "y").inOrder();
  }

  @Test
  public void lookups() {
    SimpleTable<String, Integer, String> t = newTable();

    assertThat(t.get("r1", 2)).isEqualTo("b");
    assertThat(t.get("r1", 0)).isNull();
    assertThat(t.get("zz", 0)).isNull();
    assertThat(t.contains("r1", 2)).isTrue();
    assertThat(t.contains("r1", 0)).isFalse();
    assertThat(t.contains("zz", 0)).isFalse();
    assertThat(t.containsRow("r1")).isTrue();
    assertThat(t.containsRow("zz")).isFalse();
    assertThat(t.isEmpty()).isFalse();
    assertThat(SimpleTable.create().isEmpty()).isTrue();
  }

  @Test
  public void removeForgetsEmptiedRows() {
    SimpleTable<String, Integer, String> t = newTable();

    assertThat(t.remove("r2", 1)).isEqualTo("a");
    assertThat(t.remove("r2", 1)).isNull();
    assertThat(t.remove("zz", 1)).isNull();
    assertThat(t.containsRow("r2")).isTrue();
    assertThat(t.remove("r2", 0)).isEqualTo("c");
    assertThat(t.containsRow("r2")).isFalse();

    t.put("r2", 5, "e");
    assertThat(t.rowKeySet()).containsExactly("r1", "r2").inOrder();
    assertThat(t.values()).containsExactly("b", "d", "e").inOrder();
  }

  @Test
  public void viewsAreLiveAndUnmodifiable() {
    SimpleTable<String, Integer, String> t = newTable();
    Map<Integer, String> row = t.row("r1");
    var rowKeys = t.rowKeySet();
    var values = t.values();

    t.put("r1", 7, "e");
    t.put("r3", 7, "f");

    assertThat(row).containsExactly(2, "b", 1, "d", 7, "e").inOrder();
    assertThat(rowKeys).containsExactly("r2", "r1", "r3").inOrder();
    assertThat(values).containsExactly("a", "c", "b", "d", "e", "f").inOrder();

    assertThat(t.row("absent")).isEmpty();
    assertThrows(UnsupportedOperationException.class, () -> row.put(9, "x"));
    assertThrows(UnsupportedOperationException.class, () -> t.row("absent").put(9, "x"));
    assertThrows(UnsupportedOperationException.class, () -> rowKeys.remove("r1"));
    assertThrows(UnsupportedOperationException.class, () -> values.iterator().remove());
  }
}
