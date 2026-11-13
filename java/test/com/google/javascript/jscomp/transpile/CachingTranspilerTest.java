/*
 * Copyright 2016 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.transpile;

import static com.google.common.truth.Truth.assertThat;

import com.google.common.cache.CacheBuilder;
import com.google.common.collect.ImmutableList;
import java.net.URI;
import java.net.URISyntaxException;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Tests for {@link CachingTranspiler}. */
@RunWith(JUnit4.class)
public final class CachingTranspilerTest {

  private Transpiler transpiler;

  FakeTranspiler delegate;

  /**
   * Fake that returns the canned result for the given arguments and records the calls, like the
   * mock it replaces.
   */
  private static final class FakeTranspiler implements Transpiler {
    final Map<List<Object>, TranspileResult> transpileResults = new HashMap<>();
    String runtimeField = "";
    final List<List<Object>> transpileCalls = new ArrayList<>();
    int runtimeCalls = 0;

    @Override
    public TranspileResult transpile(URI path, String code) {
      transpileCalls.add(ImmutableList.of(path, code));
      return transpileResults.get(ImmutableList.of(path, code));
    }

    @Override
    public String runtime() {
      runtimeCalls++;
      return runtimeField;
    }
  }

  private static final URI FOO_JS;
  private static final URI BAR_JS;
  private static final URI QUX_JS;

  private static final TranspileResult RESULT1;
  private static final TranspileResult RESULT2;
  private static final TranspileResult RESULT3;

  static {
    try {
      FOO_JS = new URI("foo.js");
      BAR_JS = new URI("bar.js");
      QUX_JS = new URI("qux.js");
    } catch (URISyntaxException e) {
      throw new RuntimeException(e);
    }
    RESULT1 = new TranspileResult(FOO_JS, "bar", "baz", "");
    RESULT2 = new TranspileResult(QUX_JS, "qux", "corge", "");
    RESULT3 = new TranspileResult(BAR_JS, "baz", "xyzzy", "");
  }

  @Before
  public void setUp() {
    delegate = new FakeTranspiler();
    transpiler = new CachingTranspiler(delegate, CacheBuilder.newBuilder());
  }

  @Test
  public void testTranspileDelegates() {
    delegate.transpileResults.put(ImmutableList.of(FOO_JS, "bar"), RESULT1);
    assertThat(transpiler.transpile(FOO_JS, "bar")).isSameInstanceAs(RESULT1);
  }

  @Test
  public void testTranspileCaches() {
    delegate.transpileResults.put(ImmutableList.of(FOO_JS, "bar"), RESULT1);
    assertThat(transpiler.transpile(FOO_JS, "bar")).isSameInstanceAs(RESULT1);
    assertThat(transpiler.transpile(FOO_JS, "bar")).isSameInstanceAs(RESULT1);
    assertThat(Collections.frequency(delegate.transpileCalls, ImmutableList.of(FOO_JS, "bar")))
        .isEqualTo(1);
  }

  @Test
  public void testTranspileDependsOnBothPathAndCode() {
    delegate.transpileResults.put(ImmutableList.of(FOO_JS, "bar"), RESULT1);
    delegate.transpileResults.put(ImmutableList.of(BAR_JS, "bar"), RESULT2);
    delegate.transpileResults.put(ImmutableList.of(FOO_JS, "bard"), RESULT3);
    assertThat(transpiler.transpile(FOO_JS, "bar")).isSameInstanceAs(RESULT1);
    assertThat(transpiler.transpile(BAR_JS, "bar")).isSameInstanceAs(RESULT2);
    assertThat(transpiler.transpile(FOO_JS, "bard")).isSameInstanceAs(RESULT3);
  }

  @Test
  public void testRuntimeDelegates() {
    delegate.runtimeField = "xyzzy";
    assertThat(transpiler.runtime()).isSameInstanceAs("xyzzy");
  }

  @Test
  public void testRuntimeCaches() {
    delegate.runtimeField = "xyzzy";
    assertThat(transpiler.runtime()).isSameInstanceAs("xyzzy");
    assertThat(transpiler.runtime()).isSameInstanceAs("xyzzy");
    assertThat(delegate.runtimeCalls).isEqualTo(1);
  }
}
