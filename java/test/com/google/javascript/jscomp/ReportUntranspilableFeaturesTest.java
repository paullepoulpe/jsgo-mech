/*
 * Copyright 2014 The Closure Compiler Authors.
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
package com.google.javascript.jscomp;

import static com.google.javascript.jscomp.CheckRegExp.MALFORMED_REGEXP;
import static com.google.javascript.jscomp.ReportUntranspilableFeatures.UNSUPPORTED_FEATURE_PRESENT;
import static com.google.javascript.jscomp.ReportUntranspilableFeatures.UNTRANSPILABLE_FEATURE_PRESENT;

import com.google.common.collect.ImmutableList;
import com.google.javascript.jscomp.CompilerOptions.BrowserFeaturesetYear;
import com.google.javascript.jscomp.CompilerOptions.LanguageMode;
import org.jspecify.annotations.Nullable;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public class ReportUntranspilableFeaturesTest extends CompilerTestCase {

  private LanguageMode languageOutField;
  private @Nullable BrowserFeaturesetYear browserFeaturesetYearField;

  @Override
  @Before
  public void setUp() throws Exception {
    super.setUp();
    languageOutField = LanguageMode.ECMASCRIPT3;
    browserFeaturesetYearField = null;
    disableTypeCheck();
    enableRunTypeCheckAfterProcessing();
  }

  @Override
  protected CompilerPass getProcessor(Compiler compiler) {
    return PeepholeTranspilationsPass.create(
        compiler,
        ImmutableList.of(
            new ReportUntranspilableFeatures(
                compiler,
                browserFeaturesetYearField,
                (browserFeaturesetYearField != null
                    ? browserFeaturesetYearField.getFeatureSet()
                    : languageOutField.toFeatureSet()))));
  }

  @Test
  public void testEs2018RegexFlagS() {
    languageOutField = LanguageMode.ECMASCRIPT_2018;
    testSame("const a = /asdf/;");
    testSame("const a = /asdf/g;");
    testSame("const a = /asdf/s;");
    testSame("const a = /asdf/gs;");

    languageOutField = LanguageMode.ECMASCRIPT_2017;
    testSame("const a = /asdf/;");
    testSame("const a = /asdf/g;");
    testError(
        "const a = /asdf/s;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp flag 's'" to targeted output language. Feature requires\
         at minimum ECMASCRIPT_2018. Consider targeting a more modern output.\
        """);
    testError(
        "const a = /asdf/gs;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp flag 's'" to targeted output language. Feature requires\
         at minimum ECMASCRIPT_2018. Consider targeting a more modern output.\
        """);
  }

  @Test
  public void testInvalidRegExpReportsWarning() {
    testWarning("const a = /([0-9a-zA-Z_\\-]{20,}/", MALFORMED_REGEXP);
  }

  @Test
  public void testEs2018RegexLookbehind() {
    languageOutField = LanguageMode.ECMASCRIPT_2018;
    testSame("const a = /(?<=asdf)/;");
    testSame("const a = /(?<!asdf)/;");

    languageOutField = LanguageMode.ECMASCRIPT_2017;
    testSame("const a = /(?=asdf)/;"); // Lookaheads are fine
    testSame("const a = /(?!asdf)/g;"); // Lookaheads are fine
    testError(
        "const a = /(?<=asdf)/;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp Lookbehind" to targeted output language. Feature requires\
         at minimum ECMASCRIPT_2018. Consider targeting a more modern output.\
        """);
    testError(
        "const a = /(?<!asdf)/;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp Lookbehind" to targeted output language. Feature requires\
         at minimum ECMASCRIPT_2018. Consider targeting a more modern output.\
        """);
  }

  @Test
  public void testEs2018RegexUnicodePropertyEscape() {
    languageOutField = LanguageMode.ECMASCRIPT_2018;
    testSame("const a = /\\p{Script=Greek}/u;");
    testSame("const a = /\\P{Script=Greek}/u;");
    testSame("const a = /\\p{Script=Greek}/;"); // Without u flag, /\p/ is same as /p/
    testSame("const a = /\\P{Script=Greek}/;"); // Without u flag, /\p/ is same as /p/

    languageOutField = LanguageMode.ECMASCRIPT_2017;
    testSame("const a = /\\p{Script=Greek}/;"); // Without u flag, /\p/ is same as /p/
    testSame("const a = /\\P{Script=Greek}/;"); // Without u flag, /\p/ is same as /p/
    testError(
        "const a = /\\p{Script=Greek}/u;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp unicode property escape" to targeted output language.\
         Feature requires at minimum ECMASCRIPT_2018. Consider targeting a more modern\
         output.\
        """);
    testError(
        "const a = /\\P{Script=Greek}/u;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp unicode property escape" to targeted output language.\
         Feature requires at minimum ECMASCRIPT_2018. Consider targeting a more modern\
         output.\
        """);
  }

  @Test
  public void testRegExpConstructorCalls() {
    languageOutField = LanguageMode.ECMASCRIPT_2017;
    // TODO(bradfordcsmith): report errors from RegExp in this form
    testSame("const a = new RegExp('asdf', 'gs');");
  }

  @Test
  public void testEs2018RegexNamedCaptureGroups() {
    languageOutField = LanguageMode.ECMASCRIPT_2018;
    testSame("const a = /(?<name>)/u;");

    languageOutField = LanguageMode.ECMASCRIPT_2017;
    testError(
        "const a = /(?<name>)/;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp named groups" to targeted output language. Feature\
         requires at minimum ECMASCRIPT_2018. Consider targeting a more modern output.\
        """);
    testError(
        "const a = /(?<$var>).*/u;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp named groups" to targeted output language. Feature\
         requires at minimum ECMASCRIPT_2018. Consider targeting a more modern output.\
        """);
    // test valid regex with '<' or '>' that is not named capture group
    testSame("const a = /(<name>)/;");
    testSame("const a = /(>.>)/u;");
  }

  @Test
  public void testEs2018RegexNamedCaptureGroupsBackReferencing() {
    languageOutField = LanguageMode.ECMASCRIPT_2018;
    testSame("const a = /^(?<half>.*).\\k<half>$/u;");

    languageOutField = LanguageMode.ECMASCRIPT_2017;
    testError(
        "const a = /^(?<half>.*).\\k<half>$/u;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp named groups" to targeted output language. Feature\
         requires at minimum ECMASCRIPT_2018. Consider targeting a more modern output.\
        """);
  }

  @Test
  public void testEs2018RegexNamedCaptureGroupsBackReferencing_usingBrowserFeaturesetYear() {
    browserFeaturesetYearField = BrowserFeaturesetYear.YEAR_2021;
    testSame("const a = /^(?<half>.*).\\k<half>$/u;");

    browserFeaturesetYearField = BrowserFeaturesetYear.YEAR_2020;
    testError(
        "const a = /^(?<half>.*).\\k<half>$/u;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp named groups" to targeted output language. Feature\
         requires at minimum browser featureset year 2021. Consider targeting a more modern\
         output.
        Current browser featureset year: 2020\
        """);
  }

  @Test
  public void reportErrorWithBigIntLiteralTranspilation() {
    languageOutField = LanguageMode.ECMASCRIPT_2021;
    testSame("1234n");

    languageOutField = LanguageMode.ECMASCRIPT3;
    testError("1234n", ReportUntranspilableFeatures.UNTRANSPILABLE_FEATURE_PRESENT);
  }

  @Test
  public void reportErrorWithBigIntLiteralTranspilation_browserFeaturesetYear() {
    browserFeaturesetYearField = BrowserFeaturesetYear.YEAR_2021;
    testSame("1234n");

    browserFeaturesetYearField = BrowserFeaturesetYear.YEAR_2020;
    testError(
        "1234n",
        ReportUntranspilableFeatures.UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "bigint" to targeted output language. Feature requires at minimum\
         browser featureset year 2021. Consider targeting a more modern output.
        Current browser featureset year: 2020\
        """);
  }

  @Test
  public void noErrorWithBigIntConstructorTranspilation() {
    // Do not report an error for use of the `BigInt()` method, just the literal form.
    languageOutField = LanguageMode.ECMASCRIPT3;
    testSame("BigInt(1234)");
  }

  @Test
  public void testEs2022RegexFlagD() {
    languageOutField = LanguageMode.ECMASCRIPT_NEXT;
    testSame("const a = /^(?<half>.*).\\k<half>$/d;");

    languageOutField = LanguageMode.ECMASCRIPT_2021;
    testError(
        "const a = /^(?<half>.*).\\k<half>$/d;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp flag 'd'" to targeted output language. Feature requires at \
        minimum ECMASCRIPT_2022. Consider targeting a more modern output.\
        """);
  }

  @Test
  public void testEs2022RegexFlagD_usingBrowserFeaturesetYear() {
    browserFeaturesetYearField = BrowserFeaturesetYear.YEAR_2022;
    testError(
        "const a = /^(?<half>.*).\\k<half>$/d;",
        UNTRANSPILABLE_FEATURE_PRESENT,
        """
        Cannot convert feature "RegExp flag 'd'" to targeted output language. Feature requires at \
        minimum browser featureset year 2023. Consider targeting a more modern output.
        Current browser featureset year: 2022\
        """);
  }

  @Test
  public void testEs2022TopLevelAwait() {
    Runnable testTopLevelAwaitUnsupported =
        () ->
            testError(
                "export const x = await 5;",
                UNSUPPORTED_FEATURE_PRESENT,
                """
                The feature "Top-level await" is currently unsupported for transpilation.
                """);

    languageOutField = LanguageMode.ECMASCRIPT_NEXT;
    testTopLevelAwaitUnsupported.run();

    languageOutField = LanguageMode.ECMASCRIPT_2022;
    testTopLevelAwaitUnsupported.run();

    languageOutField = LanguageMode.ECMASCRIPT_2021;
    testTopLevelAwaitUnsupported.run();

    languageOutField = LanguageMode.ECMASCRIPT3; // Reset

    browserFeaturesetYearField = BrowserFeaturesetYear.YEAR_2023;
    testTopLevelAwaitUnsupported.run();

    browserFeaturesetYearField = BrowserFeaturesetYear.YEAR_2022;
    testTopLevelAwaitUnsupported.run();
  }

  @Test
  public void testEs2022TopLevelForAwaitOf() {
    Runnable testTopLevelAwaitUnsupported =
        () ->
            testError(
                "export const x = 1; for await (let a of b) foo();",
                UNSUPPORTED_FEATURE_PRESENT,
                """
                The feature "Top-level await" is currently unsupported for transpilation.
                """);

    languageOutField = LanguageMode.ECMASCRIPT_NEXT;
    testTopLevelAwaitUnsupported.run();

    languageOutField = LanguageMode.ECMASCRIPT_2022;
    testTopLevelAwaitUnsupported.run();

    languageOutField = LanguageMode.ECMASCRIPT_2021;
    testTopLevelAwaitUnsupported.run();

    languageOutField = LanguageMode.ECMASCRIPT3; // Reset

    browserFeaturesetYearField = BrowserFeaturesetYear.YEAR_2023;
    testTopLevelAwaitUnsupported.run();

    browserFeaturesetYearField = BrowserFeaturesetYear.YEAR_2022;
    testTopLevelAwaitUnsupported.run();
  }

  @Test
  public void testCannotConvertEs5GetterToEs3() {
    this.languageOutField = LanguageMode.ECMASCRIPT3;
    testError("var o = {get x() {}};", UNTRANSPILABLE_FEATURE_PRESENT);
  }

  @Test
  public void testCannotConvertEs5SetterToEs3() {
    this.languageOutField = LanguageMode.ECMASCRIPT3;
    testError("var o = {set x(val) {}};", UNTRANSPILABLE_FEATURE_PRESENT);
  }

  @Test
  public void testAllowsEs5GetterSetterWithEs5Out() {
    this.languageOutField = LanguageMode.ECMASCRIPT5;
    testNoWarning(
        """
        var o = {
          get x() {},
          set x(val) {}
        };
        """);
  }
}
