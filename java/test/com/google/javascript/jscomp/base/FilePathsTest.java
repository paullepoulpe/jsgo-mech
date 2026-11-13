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
import java.nio.file.Path;
import java.util.Optional;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Checks {@link FilePaths} against java.nio.file.Path on a Unix file system. */
@RunWith(JUnit4.class)
public final class FilePathsTest {
  private static final String[] PATHS = {
    "", "/", "a", "/a", "a/", "a/b", "/a/b", "a//b/", "//a", ".", "..", "./a", "../a", "a/..",
    "a/../b", "a/./b", "/..", "/../a", "../../a", "a/b/../../..", "a/b/c.js", "/x/y/z.js", "./",
    "a/../../b", "b/c", "/a/b/c/d", "x.js", "../x.js", "/a/../b/./c",
  };

  @Test
  public void agreesWithNioPath() {
    assume().that(File.separator).isEqualTo("/");
    for (String p : PATHS) {
      Path path = Path.of(p);
      assertWithMessage("clean %s", p).that(FilePaths.clean(p)).isEqualTo(path.toString());
      assertWithMessage("fileName %s", p)
          .that(FilePaths.fileName(p))
          .isEqualTo(path.getFileName() == null ? "" : path.getFileName().toString());
      assertWithMessage("parent %s", p)
          .that(FilePaths.parent(p))
          .isEqualTo(Optional.ofNullable(path.getParent()).map(Path::toString));
      assertWithMessage("normalize %s", p)
          .that(FilePaths.normalize(p))
          .isEqualTo(path.normalize().toString());
      assertWithMessage("toAbsolute %s", p)
          .that(FilePaths.toAbsolute(p))
          .isEqualTo(path.toAbsolutePath().toString());
      assertWithMessage("isAbsolute %s", p)
          .that(FilePaths.isAbsolute(p))
          .isEqualTo(path.isAbsolute());
      for (String q : PATHS) {
        Path other = Path.of(q);
        assertWithMessage("resolve %s %s", p, q)
            .that(FilePaths.resolve(p, q))
            .isEqualTo(path.resolve(other).toString());
        assertWithMessage("join %s %s", p, q)
            .that(FilePaths.join(p, q))
            .isEqualTo(Path.of(p, q).toString());
        Optional<String> expected;
        try {
          expected = Optional.of(path.relativize(other).toString());
        } catch (IllegalArgumentException e) {
          expected = Optional.empty();
        }
        assertWithMessage("relativize %s %s", p, q)
            .that(FilePaths.relativize(p, q))
            .isEqualTo(expected);
      }
    }
  }

  @Test
  public void examples() {
    assertThat(FilePaths.join("a", "b", "", "c/")).isEqualTo("a/b/c");
    assertThat(FilePaths.relativize("a/b", "a/c/d.js")).isEqualTo(Optional.of("../c/d.js"));
    assertThat(FilePaths.relativize("/a", "b")).isEqualTo(Optional.empty());
  }
}
