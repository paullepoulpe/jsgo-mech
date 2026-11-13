/*
 * Copyright 2017 The Closure Compiler Authors.
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

package com.google.javascript.jscomp.bundle;

import static com.google.common.truth.Truth.assertThat;

import com.google.common.cache.CacheBuilder;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Tests for {@link Source} and its nested classes. */
@RunWith(JUnit4.class)
public final class CachedTransformerTest {

  private static final Source FOO = Source.builder().setCode("foo").build();
  private static final Source BAR = Source.builder().setCode("bar").build();
  private static final Source BAZ = Source.builder().setCode("baz").build();
  private static final Source QUX = Source.builder().setCode("qux").build();

  FakeDelegate delegate;

  /**
   * Fake that returns the canned results for the given argument (the last one repeating) and
   * records the calls, like the mock it replaces.
   */
  private static final class FakeDelegate implements Function<Source, Source> {
    final Map<Source, List<Source>> applyResults = new HashMap<>();
    final List<Source> applyCalls = new ArrayList<>();

    @Override
    public Source apply(Source input) {
      applyCalls.add(input);
      List<Source> results = applyResults.get(input);
      if (results == null) {
        return null;
      }
      return results.get(Math.min(Collections.frequency(applyCalls, input), results.size()) - 1);
    }
  }

  @Before
  public void setUp() {
    delegate = new FakeDelegate();
  }

  @Test
  public void testDelegates() {
    Source.Transformer cached =
        new CachedTransformer(Source.Transformer.of(delegate), CacheBuilder.newBuilder());
    delegate.applyResults.put(FOO, Arrays.asList(BAR));
    delegate.applyResults.put(BAZ, Arrays.asList(QUX));

    assertThat(cached.transform(FOO)).isSameInstanceAs(BAR);
    assertThat(cached.transform(BAZ)).isSameInstanceAs(QUX);
  }

  @Test
  public void testCaches() {
    Source.Transformer cached =
        new CachedTransformer(Source.Transformer.of(delegate), CacheBuilder.newBuilder());
    delegate.applyResults.put(FOO, Arrays.asList(BAR, null));

    assertThat(cached.transform(FOO)).isSameInstanceAs(BAR);
    assertThat(cached.transform(FOO)).isSameInstanceAs(BAR);
    assertThat(Collections.frequency(delegate.applyCalls, FOO)).isEqualTo(1);
  }
}
