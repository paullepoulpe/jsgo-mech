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
import static com.google.common.truth.TruthJUnit.assume;

import java.io.File;
import java.nio.file.FileSystems;
import java.nio.file.Path;
import java.nio.file.PathMatcher;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Checks {@link Glob} against the JDK's "glob:" path matcher on a Unix file system. */
@RunWith(JUnit4.class)
public final class GlobTest {
  private static final String[] GLOBS = {
    "*.js", "**.js", "./**.js", "./*.js", "sub/**.js", "sub/*.js", "sub/**", "**/c2.js",
    "./**_test.js", "sub/deep/**", "a?.js", "?.js", "[ab].js", "[a-c].js", "[!a].js", "[!a-c]*.js",
    "*.{js,ts}", "{a,b}/*.js", "sub/{deep,x}/**.js", "**/*.js", "*", "**", "a.js", "./a.js",
    "sub/*/*.js", "*/*", "a\\*.js", "\\[a].js", "./mods/**", "sub/**/d.js", "**/deep/*",
    "{sub,mods}/**.{js,mjs}", "[a-z]*/[!x]*.js", "*.j?", "**b.js", "*b*", "s*b/c.js", "a*b*c*.js",
    "./sub/**_test.js", "**x**", "**/", "sub/", "[-a].js", "[a-].js", "{a.js,b.js}", "x{a,b}",
  };

  private static final String[] PATHS = {
    "a.js", "b.js", "c.js", "ab.js", "abc.js", "./a.js", "./b.js", "./sub/c.js", "sub/c.js",
    "sub/c2.js", "sub/z_test.js", "./sub/z_test.js", "sub/deep/d.js", "./sub/deep/d.js",
    "mods/main.js", "./mods/lib/helper.js", "a.ts", "a.mjs", "sub/x/y.mjs", "a*.js", "[a].js",
    "sub", "a", "a/b.js", "b/b.js", "sub/b/c.js", "sub/deep/x/d.js", "-.js", "xa", "xb", "axbxc.js",
    ".hidden.js", "sub/.hidden.js", "d/e/f/g.js",
  };

  @Test
  public void agreesWithJdkPathMatcher() {
    assume().that(File.separator).isEqualTo("/");
    for (String glob : GLOBS) {
      PathMatcher jdk = FileSystems.getDefault().getPathMatcher("glob:" + glob);
      for (String path : PATHS) {
        assertWithMessage("glob %s path %s", glob, path)
            .that(Glob.matches(glob, path))
            .isEqualTo(jdk.matches(Path.of(path)));
      }
    }
  }

  @Test
  public void unclosedBracketsAreOrdinaryCharacters() {
    assertThat(Glob.matches("a[b", "a[b")).isTrue();
    assertThat(Glob.matches("a{b,c", "a{b,c")).isTrue();
    assertThat(Glob.matches("a{b,c", "ab")).isFalse();
    assertThat(Glob.matches("a\\", "a\\")).isTrue();
  }
}
