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

import com.google.common.base.Joiner;
import com.google.common.collect.ImmutableList;
import com.google.debugging.sourcemap.SourceMapConsumerV3;
import com.google.debugging.sourcemap.proto.Mapping.OriginalMapping;
import com.google.debugging.sourcemap.proto.Mapping.OriginalMapping.Precision;
import com.google.javascript.jscomp.JSError;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Tests for {@link CoverageInstrumenter}. */
@RunWith(JUnit4.class)
public final class CoverageInstrumenterTest {

  private Source.Transformer instrumenter;
  private CoverageInstrumenter.CompilerSupplier compiler;

  FakeCompilerSupplier mockCompiler;

  /** Fake that returns the canned result for the given arguments, like the mock it replaces. */
  private static final class FakeCompilerSupplier extends CoverageInstrumenter.CompilerSupplier {
    final Map<List<Object>, CoverageInstrumenter.CompileResult> compileResults = new HashMap<>();
    final Map<String, String> runtimes = new HashMap<>();

    @Override
    public CoverageInstrumenter.CompileResult compile(Path path, String code) {
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
    instrumenter = new CoverageInstrumenter(mockCompiler);
    compiler = CoverageInstrumenter.compilerSupplier();
  }

  // Tests for CoverageInstrumenter

  @Test
  public void testCoverageInstrumenter_instrument() {
    mockCompiler.compileResults.put(
        ImmutableList.of(FOO_JS, "bar"),
        new CoverageInstrumenter.CompileResult("result", NO_ERRORS, true, "srcmap"));
    assertThat(instrumenter.transform(source(FOO_JS, "bar")))
        .isEqualTo(
            Source.builder()
                .setPath(FOO_JS)
                .setOriginalCode("bar")
                .setCode("result")
                .setSourceMap("srcmap")
                .build());
  }

  @Test
  public void testCoverageInstrumenter_noInstrumentation() {
    mockCompiler.compileResults.put(
        ImmutableList.of(FOO_JS, "bar"),
        new CoverageInstrumenter.CompileResult("result", NO_ERRORS, false, "srcmap"));
    assertThat(instrumenter.transform(source(FOO_JS, "bar"))).isEqualTo(source(FOO_JS, "bar"));
  }

  private static Source source(Path path, String code) {
    return Source.builder().setPath(path).setCode(code).build();
  }

  // Tests for CompilerSupplier

  @Test
  public void testCompilerSupplier_instruments() throws Exception {
    CoverageInstrumenter.CompileResult result = compiler.compile(SOURCE_JS, "var x = 42;");
    String[] expected =
        new String[] {
          """
          (function(self){if(!self.window){self.window=self;self.window.top=self}})(typeof \
          self!=="undefined"?self:globalThis);\
          """,
          "var __jscov=window.top[\"__jscov\"]||",
          """
          (window.top["__jscov"]={"fileNames":[],"instrumentedLines":[],\
          "executedLines":[]});\
          """,
          "var JSCompiler_lcov_data_source_js=[];",
          "__jscov[\"executedLines\"].push(JSCompiler_lcov_data_source_js);",
          "__jscov[\"instrumentedLines\"].push(\"01\");",
          "__jscov[\"fileNames\"].push(\"source.js\");",
          "JSCompiler_lcov_data_source_js[0]=true;var x=42;"
        };

    assertThat(result.source).isEqualTo(Joiner.on("").join(expected));
    assertThat(result.errors).isEmpty();
    assertThat(result.transformed).isTrue();

    // Ensure that source map for "x" is correct.
    SourceMapConsumerV3 sourceMap = new SourceMapConsumerV3();
    sourceMap.parse(result.sourceMap).get();
    OriginalMapping mappingFoX = sourceMap.getMappingForLine(1, result.source.indexOf("x=42") + 1);
    assertThat(mappingFoX)
        .isEqualTo(
            OriginalMapping.newBuilder()
                .setOriginalFile(SOURCE_JS.toString())
                .setLineNumber(1)
                .setColumnPosition(5)
                .setIdentifier("x")
                .setPrecision(Precision.EXACT)
                .build());
  }
}
