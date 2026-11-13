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

import com.google.common.collect.ImmutableList;
import com.google.javascript.jscomp.JSError;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Tests for {@link Transpiler}. */
@RunWith(JUnit4.class)
public final class TranspilerTest {

  private Source.Transformer transpiler;
  private Transpiler.CompilerSupplier compiler;

  FakeCompilerSupplier mockCompiler;

  /** Fake that returns the canned result for the given arguments, like the mock it replaces. */
  private static final class FakeCompilerSupplier extends Transpiler.CompilerSupplier {
    final Map<List<Object>, Transpiler.CompileResult> compileResults = new HashMap<>();
    final Map<String, String> runtimes = new HashMap<>();

    @Override
    public Transpiler.CompileResult compile(Path path, String code) {
      return compileResults.get(ImmutableList.of(path, code));
    }

    @Override
    public String runtime(String library) {
      return runtimes.getOrDefault(library, "");
    }
  }

  private static final Path FOO_JS = Path.of("foo.js");
  private static final Path SOURCE_JS = Path.of("source.js");
  private static final ImmutableList<JSError> NO_ERRORS = ImmutableList.of();

  @Before
  public void setUp() {
    mockCompiler = new FakeCompilerSupplier();
    transpiler = new Transpiler(mockCompiler, "es6_runtime");
    compiler = Transpiler.compilerSupplier();
  }

  // Tests for Transpiler

  @Test
  public void testTranspiler_transpile() {
    mockCompiler.runtimes.put("es6_runtime", "$jscomp.es6();");
    mockCompiler.compileResults.put(
        ImmutableList.of(FOO_JS, "bar"),
        new Transpiler.CompileResult("result", NO_ERRORS, true, "srcmap"));
    assertThat(transpiler.transform(source(FOO_JS, "bar")))
        .isEqualTo(
            Source.builder()
                .setPath(FOO_JS)
                .setOriginalCode("bar")
                .setCode("result")
                .setSourceMap("srcmap")
                .addRuntime("$jscomp.es6();")
                .build());
  }

  @Test
  public void testTranspiler_noTranspilation() {
    mockCompiler.compileResults.put(
        ImmutableList.of(FOO_JS, "bar"),
        new Transpiler.CompileResult("result", NO_ERRORS, false, "srcmap"));
    assertThat(transpiler.transform(source(FOO_JS, "bar"))).isEqualTo(source(FOO_JS, "bar"));
  }

  private static Source source(Path path, String code) {
    return Source.builder().setPath(path).setCode(code).build();
  }

  // Tests for CompilerSupplier

  @Test
  public void testCompilerSupplier_compileChanged() {
    Transpiler.CompileResult result = compiler.compile(SOURCE_JS, "const x = () => 42;");
    assertThat(result.source).isEqualTo("var x = function() {\n  return 42;\n};\n");
    assertThat(result.errors).isEmpty();
    assertThat(result.transformed).isTrue();
    assertThat(result.sourceMap)
        .contains("\"mappings\":\"AAAA,IAAMA,IAAIA,QAAA,EAAM;AAAA,SAAA,EAAA;AAAA,CAAhB;;\"");
  }

  @Test
  public void testCompilerSupplier_compileNoChange() {
    Transpiler.CompileResult result = compiler.compile(SOURCE_JS, "var x = 42;");
    assertThat(result.source).isEqualTo("var x = 42;\n");
    assertThat(result.errors).isEmpty();
    assertThat(result.transformed).isFalse();
    assertThat(result.sourceMap).isEmpty();
  }

  @Test
  public void testCompilerSupplier_runtime() {
    String runtime = compiler.runtime("es6_runtime");
    assertThat(runtime).contains("$jscomp.polyfill(\"Map\"");
    assertThat(runtime).contains("$jscomp.makeIterator");
    assertThat(runtime).contains("$jscomp.inherits");
  }
}
