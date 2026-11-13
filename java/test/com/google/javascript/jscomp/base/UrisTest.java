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

import java.net.URI;
import java.net.URISyntaxException;
import java.util.Optional;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Checks {@link Uris} against java.net.URI on the kinds of text the compiler asks about. */
@RunWith(JUnit4.class)
public final class UrisTest {
  private static final String[] TEXTS = {
    "", "a.js", "./a.js", "../a/b.js", "/a/b.js", "a/b/c.js", "http://example.com/a/b.js",
    "https://example.com:8080/a/b.js?x=1#frag", "file:///a/b.js", "//example.com/a.js", "foo:bar",
    "data:application/json;base64,AAAA", "webpack:///./src/a.js", "c:/dir/a.js", "a b.js",
    "C:\\dir\\a.js", "a%20b.js", "a%2", "a%zz.js", "a|b", "a^b", "a{b}", "a\"b", "<a>", "a`b",
    ":a", "1a:b", "a:", "a/b:c", "?x:y", "#x:y", "@scope/pkg", "@scope/pkg/a.js", "a+b.js",
    "a~b.js", "a'b.js", "a(1).js", "a,b;c=d&e$f!g*h", "\u00e9.js", "a\u00a0b", "a\tb", "x-y+z.1:rest",
    "goog:foo.bar", "/a/b.js?q", "http://example.com", "node_modules/x/index.js",
  };

  @Test
  public void agreesWithJavaNetUri() {
    for (String text : TEXTS) {
      URI jdk = null;
      try {
        jdk = new URI(text);
      } catch (URISyntaxException e) {
        // not valid
      }
      assertWithMessage("isValid %s", text).that(Uris.isValid(text)).isEqualTo(jdk != null);
      if (jdk != null) {
        assertWithMessage("hasScheme %s", text)
            .that(Uris.hasScheme(text))
            .isEqualTo(Optional.ofNullable(jdk.getScheme()).isPresent());
        Optional<String> path = Optional.ofNullable(jdk.getRawPath());
        if (path.isPresent()) {
          assertWithMessage("path %s", text).that(Uris.path(text)).isEqualTo(path.get());
        }
      }
    }
  }

  @Test
  public void pathOfAnOpaqueUriIsWhatFollowsTheScheme() {
    assertThat(Uris.path("goog:foo.bar")).isEqualTo("foo.bar");
    assertThat(Uris.path("a%20b.js")).isEqualTo("a%20b.js");
  }
}
