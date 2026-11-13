/*
 * Copyright 2009 The Closure Compiler Authors.
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

import static com.google.common.base.Preconditions.checkState;
import static java.nio.charset.StandardCharsets.UTF_8;

import com.google.common.base.Ascii;
import com.google.common.base.Joiner;
import com.google.common.base.Splitter;
import com.google.common.base.Strings;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableMultimap;
import com.google.common.io.Files;
import com.google.errorprone.annotations.InlineMe;
import com.google.errorprone.annotations.Keep;
import com.google.javascript.jscomp.AbstractCommandLineRunner.CommandLineConfig.ErrorFormatOption;
import com.google.javascript.jscomp.CompilerOptions.ChunkOutputType;
import com.google.javascript.jscomp.CompilerOptions.ExtractPrototypeMemberDeclarationsMode;
import com.google.javascript.jscomp.CompilerOptions.InstrumentOption;
import com.google.javascript.jscomp.CompilerOptions.IsolationMode;
import com.google.javascript.jscomp.CompilerOptions.LanguageMode;
import com.google.javascript.jscomp.CompilerOptions.SegmentOfCompilationToRun;
import com.google.javascript.jscomp.DependencyOptions.DependencyMode;
import com.google.javascript.jscomp.SourceMap.LocationMapping;
import com.google.javascript.jscomp.deps.ClosureBundler;
import com.google.javascript.jscomp.deps.ModuleLoader;
import com.google.javascript.jscomp.parsing.parser.FeatureSet;
import com.google.javascript.jscomp.parsing.parser.FeatureSet.Feature;
import com.google.javascript.jscomp.transpile.BaseTranspiler;
import com.google.javascript.jscomp.transpile.BaseTranspiler.CompilerSupplier;
import com.google.javascript.jscomp.transpile.Transpiler;
import com.google.javascript.rhino.TokenStream;
import com.google.protobuf.TextFormat;
import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStreamWriter;
import java.io.PrintStream;
import java.io.StringWriter;
import java.nio.file.FileSystem;
import java.nio.file.FileSystems;
import java.nio.file.FileVisitOption;
import java.nio.file.FileVisitResult;
import java.nio.file.Path;
import java.nio.file.PathMatcher;
import java.nio.file.SimpleFileVisitor;
import java.nio.file.attribute.BasicFileAttributes;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.EnumSet;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.logging.Level;
import java.util.logging.Logger;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.jspecify.annotations.Nullable;

/**
 * CommandLineRunner translates flags into Java API calls on the Compiler.
 *
 * <p>This class may be extended and used to create other Java classes that behave the same as
 * running the Compiler from the command line. If you want to run the compiler in-process in Java,
 * you should look at this class for hints on what API calls to make, but you should not use this
 * class directly.
 *
 * <p>Example:
 *
 * <pre>
 * class MyCommandLineRunner extends CommandLineRunner {
 *   MyCommandLineRunner(String[] args) {
 *     super(args);
 *   }
 *
 *   {@code @Override} protected CompilerOptions createOptions() {
 *     CompilerOptions options = super.createOptions();
 *     addMyCrazyCompilerPassThatOutputsAnExtraFile(options);
 *     return options;
 *   }
 *
 *   public static void main(String[] args) {
 *     MyCommandLineRunner runner = new MyCommandLineRunner(args);
 *     if (runner.shouldRunCompiler()) {
 *       runner.run();
 *     }
 *     if (runner.hasErrors()) {
 *       System.exit(-1);
 *     }
 *   }
 * }
 * </pre>
 *
 * This class is totally not thread-safe.
 */
public class CommandLineRunner extends AbstractCommandLineRunner<Compiler, CompilerOptions> {

  public static final String OUTPUT_MARKER = AbstractCommandLineRunner.OUTPUT_MARKER;

  // UTF-8 BOM is 0xEF, 0xBB, 0xBF, of which character code is 65279.
  public static final int UTF8_BOM_CODE = 65279;

  // Allowable chunk name characters that aren't valid in a JS identifier
  private static final Pattern extraChunkNameChars = Pattern.compile("[-.]+");

  // I don't really care about unchecked warnings in this class.
  @SuppressWarnings("unchecked")
  private static class Flags {
    // These lists belong to one runner: some clients run a few copies of the compiler through
    // CommandLineRunner on parallel threads.
    private final List<FlagEntry<CheckLevel>> guardLevels = new ArrayList<>();
    private final List<FlagEntry<JsSourceType>> mixedJsSources = new ArrayList<>();

    private Integer browserFeaturesetYear = 0;

    private boolean displayHelp = false;

    private boolean printTree = false;

    private boolean printTreeJson = false;

    private boolean printAst = false;

    private boolean emitUseStrict = false;

    private boolean strictModeInput = true;

    // Turn on (very slow) extra validity checks for use when modifying the compiler.
    private CompilerOptions.DevMode jscompDevMode = CompilerOptions.DevMode.OFF;

    private String loggingLevel = Level.WARNING.getName();

    private List<String> externs = new ArrayList<>();

    @Keep
    private List<String> js = new ArrayList<>();

    private @Nullable List<String> unusedJsZip = null;

    private String jsOutputFile = "";

    private List<String> chunk = new ArrayList<>();

    private @Nullable String filenameToSaveTo = null;

    private @Nullable String filenameToRestoreFrom = null;

    private @Nullable SegmentOfCompilationToRun segmentOfCompilationToRun =
        SegmentOfCompilationToRun.ENTIRE_COMPILATION;

    private String variableMapOutputFile = "";

    private String instrumentationMappingOutputFile = "";

    private boolean createNameMapFiles = false;

    private boolean sourceMapIncludeSourcesContent = false;

    private String propertyMapOutputFile = "";

    private boolean thirdParty = false;

    private int summaryDetailLevel = 1;

    private IsolationMode isolationMode = IsolationMode.NONE;

    private String outputWrapper = "";

    private String outputWrapperFile = "";

    private List<String> chunkWrapper = new ArrayList<>();

    private String chunkOutputPathPrefix = "./";

    private String createSourceMap = "";

    private SourceMap.Format sourceMapFormat = SourceMap.Format.DEFAULT;

    private List<String> sourceMapLocationMapping = new ArrayList<>();

    private List<String> sourceMapInputs = new ArrayList<>();

    private Boolean parseInlineSourceMaps = true;

    private boolean applyInputSourceMaps = true;

    // Used to define the flag, values are stored by the handler.
    @SuppressWarnings("unused")
    private List<String> jscompError = new ArrayList<>();

    // Used to define the flag, values are stored by the handler.
    @SuppressWarnings("unused")
    private List<String> jscompWarning = new ArrayList<>();

    // Used to define the flag, values are stored by the handler.
    @SuppressWarnings("unused")
    private List<String> jscompOff = new ArrayList<>();

    private List<String> define = new ArrayList<>();

    private String charset = "";

    private String compilationLevel = "SIMPLE";

    private @Nullable CompilationLevel compilationLevelParsed = null;

    private int numParallelThreads = 1;

    private boolean checksOnly = false;

    private CompilerOptions.IncrementalCheckMode incrementalCheckMode =
        CompilerOptions.IncrementalCheckMode.OFF;

    private boolean continueAfterErrors = false;

    private boolean useTypesForOptimization = true;

    private boolean assumeFunctionWrapper = false;

    private WarningLevel warningLevel = WarningLevel.DEFAULT;

    private boolean debug = false;

    private @Nullable String typedAstOutputFile = null;

    private boolean generateExports = true;

    private boolean exportLocalPropertyDefinitions = true;

    private List<FormattingOption> formatting = new ArrayList<>();

    private boolean processCommonJsModules = false;

    private List<String> moduleRoot = new ArrayList<>();

    private boolean processClosurePrimitives = true;

    private boolean angularPass = false;

    private @Nullable Integer polymerVersion = null;

    private boolean chromePass = false;

    private String j2clPassMode = "AUTO";

    private boolean removeJ2cLAsserts = true;

    private String outputManifest = "";

    private String outputChunkDependencies = "";

    private String languageIn = "STABLE";

    private String languageOut = "ECMASCRIPT_NEXT";

    private boolean version = false;

    private String translationsFile = "";

    private @Nullable String translationsProject = null;

    private List<String> flagFiles = new ArrayList<>();

    private String warningsAllowlistFile = "";

    private List<String> hideWarningsFor = new ArrayList<>();

    private List<String> extraAnnotationName = new ArrayList<>();

    private CompilerOptions.TracerMode tracerMode = CompilerOptions.TracerMode.OFF;

    private @Nullable String renamePrefix = null;

    private @Nullable String renamePrefixNamespace = null;

    private List<String> conformanceConfigs = new ArrayList<>();

    private CompilerOptions.Environment environment = CompilerOptions.Environment.BROWSER;

    private CompilerOptions.JsonStreamMode jsonStreamMode = CompilerOptions.JsonStreamMode.NONE;

    private boolean preserveTypeAnnotations = false;

    private boolean injectLibraries = true;

    private List<String> forceInjectLibraries = new ArrayList<>();

    private @Nullable DependencyMode dependencyMode =
        null; // so we can tell whether it was explicitly set

    private List<String> entryPoint = new ArrayList<>();

    private boolean rewritePolyfills = true;

    private boolean isolatePolyfills = false;

    private boolean printSourceAfterEachPass = false;

    private ModuleLoader.ResolutionMode moduleResolutionMode = ModuleLoader.ResolutionMode.BROWSER;

    private Map<String, String> browserResolverPrefixReplacements = new LinkedHashMap<>();

    private @Nullable String packageJsonEntryNames = null;

    private ErrorFormatOption errorFormat = ErrorFormatOption.STANDARD;

    private boolean renaming = true;

    private boolean helpMarkdown = false;

    private String instrumentForCoverageOption = "NONE";

    private String productionInstrumentationArrayName = "";

    private ChunkOutputType chunkOutputType = ChunkOutputType.GLOBAL_NAMESPACE;

    private InstrumentOption instrumentCodeParsed = InstrumentOption.NONE;

    private boolean allowDynamicImport = true;

    private @Nullable String dynamicImportAlias = null;

    private boolean assumeStaticInheritanceIsNotUsed = true;

    private boolean assumeNoPrototypeMethodEnumeration = false;

    private String variableMapInputFile = "";

    private String propertyMapInputFile = "";

    private List<String> expectedDiagnostics = new ArrayList<>();

    private List<String> arguments = new ArrayList<>();
    private final CmdLineParser parser;

    Flags() {
      parser = new CmdLineParser();
      // The table of flags: one entry per flag, in the order in which the fields are declared.
      parser
          .option("--browser_featureset_year")
          .usage(
              "shortcut for defining "
                  + "goog.FEATURESET_YEAR=YYYY."
                  + " The minimum valid value of the browser year is 2012")
          .integer(() -> browserFeaturesetYear, v -> browserFeaturesetYear = v);
      parser
          .option("--help")
          .usage("Displays this message on stdout and exit")
          .bool(() -> displayHelp, v -> displayHelp = v);
      parser
          .option("--print_tree")
          .hidden(true)
          .usage("Prints out the parse tree and exits")
          .bool(() -> printTree, v -> printTree = v);
      parser
          .option("--print_tree_json")
          .hidden(true)
          .usage("Prints out the parse tree as json and exits")
          .bool(() -> printTreeJson, v -> printTreeJson = v);
      parser
          .option("--print_ast")
          .hidden(true)
          .usage("Prints a dot file describing the internal abstract syntax" + " tree and exits")
          .bool(() -> printAst, v -> printAst = v);
      parser
          .option("--emit_use_strict")
          .usage("Start output with \"'use strict';\".")
          .bool(() -> emitUseStrict, v -> emitUseStrict = v);
      parser
          .option("--strict_mode_input")
          .usage("Assume input sources are to run in strict mode.")
          .bool(() -> strictModeInput, v -> strictModeInput = v);
      parser
          .option("--jscomp_dev_mode")
          .aliases("--dev_mode")
          .hidden(true)
          .enumValue(CompilerOptions.DevMode.values(), () -> jscompDevMode, v -> jscompDevMode = v);
      parser
          .option("--logging_level")
          .hidden(true)
          .usage(
              "The logging level (standard java.util.logging.Level"
                  + " values) for Compiler progress. Does not control errors or"
                  + " warnings for the JavaScript code under compilation")
          .string(() -> loggingLevel, v -> loggingLevel = v);
      parser
          .option("--externs")
          .usage("The file containing JavaScript externs. You may specify" + " multiple")
          .stringList(() -> externs, v -> externs = v);
      parser
          .option("--js")
          .usage(
              "The JavaScript filename. You may specify multiple. "
                  + "The flag name is optional, because args are interpreted as files by default. "
                  + "You may also use minimatch-style glob patterns. For example, use "
                  + "--js='**.js' --js='!**_test.js' to recursively include all "
                  + "js files that do not end in _test.js")
          .stringEach(v -> js = addMultiFlagValue(js, v, JsSourceType.JS, mixedJsSources));
      parser
          .option("--jszip")
          .hidden(true)
          .usage("The JavaScript zip filename. You may specify multiple.")
          .stringEach(
              v ->
                  unusedJsZip =
                      addMultiFlagValue(unusedJsZip, v, JsSourceType.JS_ZIP, mixedJsSources));
      parser
          .option("--js_output_file")
          .usage("Primary output filename. If not specified, output is " + "written to stdout")
          .string(() -> jsOutputFile, v -> jsOutputFile = v);
      parser
          .option("--chunk")
          .usage(
              "A JavaScript chunk specification. The format is "
                  + "<name>:<num-js-files>[:[<dep>,...][:]]]. Chunk names must be "
                  + "unique. Each dep is the name of a chunk that this chunk "
                  + "depends on. Chunks must be listed in dependency order, and JS "
                  + "source files must be listed in the corresponding order. Where "
                  + "--chunk flags occur in relation to --js flags is unimportant. "
                  + "<num-js-files> may be set to 'auto' for the first chunk if it "
                  + "has no dependencies. "
                  + "Provide the value 'auto' to trigger chunk creation from CommonJS "
                  + "modules.")
          .stringList(() -> chunk, v -> chunk = v);
      parser
          .option("--filename_to_save_to")
          .hidden(true)
          .usage("Filename to save state so that the compilation can be resumed later.")
          .string(() -> filenameToSaveTo, v -> filenameToSaveTo = v);
      parser
          .option("--filename_to_restore_from")
          .hidden(true)
          .usage("Filename where a compilation state was previously saved.")
          .string(() -> filenameToRestoreFrom, v -> filenameToRestoreFrom = v);
      parser
          .option("--segment_of_compilation_to_run")
          .hidden(true)
          .usage("Which segment of the compilation to run.")
          .enumValue(
              SegmentOfCompilationToRun.values(),
              () -> segmentOfCompilationToRun,
              v -> segmentOfCompilationToRun = v);
      parser
          .option("--variable_renaming_report")
          .usage(
              "File where the serialized version of the variable "
                  + "renaming map produced should be saved")
          .string(() -> variableMapOutputFile, v -> variableMapOutputFile = v);
      parser
          .option("--instrument_mapping_report")
          .usage(
              "File where the encoded parameters created by Production Instrumentation are mapped to"
                  + " their pre-encoded values. The %outname% placeholder will expand to the name of"
                  + " the output file that the source map corresponds to. Must be used in tandem with"
                  + " --instrument_for_coverage_option=PRODUCTION")
          .string(
              () -> instrumentationMappingOutputFile,
              v -> instrumentationMappingOutputFile = v);
      parser
          .option("--create_renaming_reports")
          .hidden(true)
          .usage(
              "If true, variable renaming and property renaming report "
                  + "files will be produced as {binary name}_vars_renaming_report.out "
                  + "and {binary name}_props_renaming_report.out. Note that this flag "
                  + "cannot be used in conjunction with either variable_renaming_report "
                  + "or property_renaming_report")
          .bool(() -> createNameMapFiles, v -> createNameMapFiles = v);
      parser
          .option("--source_map_include_content")
          .usage(
              "Includes sources content into source map. Greatly increases "
                  + "the size of source maps but offers greater portability")
          .bool(() -> sourceMapIncludeSourcesContent, v -> sourceMapIncludeSourcesContent = v);
      parser
          .option("--property_renaming_report")
          .usage(
              "File where the serialized version of the property "
                  + "renaming map produced should be saved")
          .string(() -> propertyMapOutputFile, v -> propertyMapOutputFile = v);
      parser
          .option("--third_party")
          .usage(
              "Check source validity but do not enforce Closure style " + "rules and conventions")
          .bool(() -> thirdParty, v -> thirdParty = v);
      parser
          .option("--summary_detail_level")
          .hidden(true)
          .usage(
              "Controls how detailed the compilation summary is. Values:"
                  + " 0 (never print summary), 1 (print summary only if there are "
                  + "errors or warnings), 2 (print summary if the 'checkTypes' "
                  + "diagnostic  group is enabled, see --jscomp_warning), "
                  + "3 (always print summary). The default level is 1")
          .integer(() -> summaryDetailLevel, v -> summaryDetailLevel = v);
      parser
          .option("--isolation_mode")
          .usage(
              """
              If set to IIFE the compiler output will follow the form:
                (function(){%output%}).call(this);
              Options: NONE, IIFE\
              """)
          .enumValue(IsolationMode.values(), () -> isolationMode, v -> isolationMode = v);
      parser
          .option("--output_wrapper")
          .usage(
              "Interpolate output into this string at the place denoted"
                  + " by the marker token %output%. Use marker token %output|jsstring%"
                  + " to do js string escaping on the output."
                  + " Consider using the --isolation_mode flag instead.")
          .string(() -> outputWrapper, v -> outputWrapper = v);
      parser
          .option("--output_wrapper_file")
          .usage(
              "Loads the specified file and passes the file contents to the --output_wrapper flag,"
                  + " replacing the value if it exists. This is useful if you want special"
                  + " characters like newline in the wrapper.")
          .string(() -> outputWrapperFile, v -> outputWrapperFile = v);
      parser
          .option("--chunk_wrapper")
          .usage(
              "An output wrapper for a JavaScript chunk (optional). "
                  + "The format is <name>:<wrapper>. The chunk name must correspond "
                  + "with a chunk specified using --chunk. The wrapper must "
                  + "contain %s as the code placeholder. "
                  + "Alternately, %output% can be used in place of %s. "
                  + "%n% can be used to represent a newline. "
                  + "The %basename% placeholder can "
                  + "also be used to substitute the base name of the chunk output file.")
          .stringList(() -> chunkWrapper, v -> chunkWrapper = v);
      parser
          .option("--chunk_output_path_prefix")
          .usage(
              "Prefix for filenames of compiled JS chunks. "
                  + "<chunk-name>.js will be appended to this prefix. Directories "
                  + "will be created as needed. Use with --chunk")
          .string(() -> chunkOutputPathPrefix, v -> chunkOutputPathPrefix = v);
      parser
          .option("--create_source_map")
          .usage(
              "If specified, a source map file mapping the generated "
                  + "source files back to the original source file will be "
                  + "output to the specified path. The %outname% placeholder will "
                  + "expand to the name of the output file that the source map "
                  + "corresponds to.")
          .string(() -> createSourceMap, v -> createSourceMap = v);
      parser
          .option("--source_map_format")
          .hidden(true)
          .usage(
              "The source map format to produce. "
                  + "Options are V3 and DEFAULT, which are equivalent.")
          .enumValue(SourceMap.Format.values(), () -> sourceMapFormat, v -> sourceMapFormat = v);
      parser
          .option("--source_map_location_mapping")
          .usage(
              "Source map location mapping separated by a '|' "
                  + "(i.e. filesystem-path|webserver-path)")
          .stringList(() -> sourceMapLocationMapping, v -> sourceMapLocationMapping = v);
      parser
          .option("--source_map_input")
          .usage(
              "Source map locations for input files, separated by a '|', "
                  + "(i.e. input-file-path|input-source-map)")
          .stringList(() -> sourceMapInputs, v -> sourceMapInputs = v);
      parser
          .option("--parse_inline_source_maps")
          .usage("Parse inline source maps (//# sourceMappingURL=data:...)")
          .bool(() -> parseInlineSourceMaps, v -> parseInlineSourceMaps = v);
      parser
          .option("--apply_input_source_maps")
          .usage(
              "Apply input source maps to the output source map, i.e. have the result map back"
                  + " to original inputs.  Input sourcemaps can be located in 2 ways:\n"
                  + " 1) by the//# sourceMappingURL=<url>. \n"
                  + " 2) using the--source_map_location_mapping flag.\n"
                  + "sourceMappingURL=<url> can read both paths and inline Base64 encoded"
                  + " sourcemaps. For inline Base64 encoded sourcemaps, see"
                  + " --parse_inline_source_maps.")
          .bool(() -> applyInputSourceMaps, v -> applyInputSourceMaps = v);
      parser
          .option("--jscomp_error")
          .usage(
              "Make the named class of warnings an error. Must be one "
                  + "of the error group items. '*' adds all supported.")
          .stringEach(
              v -> jscompError = addMultiFlagValue(jscompError, v, CheckLevel.ERROR, guardLevels));
      parser
          .option("--jscomp_warning")
          .usage(
              "Make the named class of warnings a normal warning. Must be one "
                  + "of the error group items. '*' adds all supported.")
          .stringEach(
              v ->
                  jscompWarning =
                      addMultiFlagValue(jscompWarning, v, CheckLevel.WARNING, guardLevels));
      parser
          .option("--jscomp_off")
          .usage(
              "Turn off the named class of warnings. Must be one "
                  + "of the error group items. '*' adds all supported.")
          .stringEach(
              v -> jscompOff = addMultiFlagValue(jscompOff, v, CheckLevel.OFF, guardLevels));
      parser
          .option("--define")
          .aliases("--D", "-D")
          .usage(
              "Override the value of a variable annotated @define. "
                  + "The format is <name>[=<val>], where <name> is the name of a @define "
                  + "variable and <val> is a boolean, number, or a single-quoted string "
                  + "that contains no single quotes. If [=<val>] is omitted, "
                  + "the variable is marked true")
          .stringList(() -> define, v -> define = v);
      parser
          .option("--charset")
          .usage(
              "Input and output charset for all files. By default, we "
                  + "accept UTF-8 as input and output US_ASCII")
          .string(() -> charset, v -> charset = v);
      parser
          .option("--compilation_level")
          .aliases("-O")
          .usage(
              "Specifies the compilation level to use. Options: "
                  + "BUNDLE, "
                  + "WHITESPACE_ONLY, "
                  + "SIMPLE (default), "
                  + "TRANSPILE_ONLY,"
                  + "ADVANCED")
          .string(() -> compilationLevel, v -> compilationLevel = v);
      parser
          .option("--num_parallel_threads")
          .hidden(true)
          .usage("Use multiple threads to parallelize parts of the compilation.")
          .integer(() -> numParallelThreads, v -> numParallelThreads = v);
      parser
          .option("--checks_only")
          .aliases("--checks-only")
          .usage("Don't generate output. Run checks, but no optimization passes.")
          .bool(() -> checksOnly, v -> checksOnly = v);
      parser
          .option("--incremental_check_mode")
          .usage("Generate or check externs-like .i.js files representing individual libraries.")
          .enumValue(
              CompilerOptions.IncrementalCheckMode.values(),
              () -> incrementalCheckMode,
              v -> incrementalCheckMode = v);
      parser
          .option("--continue_after_errors")
          .usage("Continue trying to compile after an error is encountered.")
          .bool(() -> continueAfterErrors, v -> continueAfterErrors = v);
      parser
          .option("--use_types_for_optimization")
          .usage(
              "Enable or disable the optimizations "
                  + "based on available type information. Inaccurate type annotations "
                  + "may result in incorrect results.")
          .bool(() -> useTypesForOptimization, v -> useTypesForOptimization = v);
      parser
          .option("--assume_function_wrapper")
          .usage(
              "Enable additional optimizations based on the assumption that the output will be"
                  + " wrapped with a function wrapper.  This flag is used to indicate that"
                  + " \"global\" declarations will not actually be global but instead isolated to"
                  + " the compilation unit. This enables additional optimizations.")
          .bool(() -> assumeFunctionWrapper, v -> assumeFunctionWrapper = v);
      parser
          .option("--warning_level")
          .aliases("-W")
          .usage("Specifies the warning level to use. Options: " + "QUIET, DEFAULT, VERBOSE")
          .enumValue(WarningLevel.values(), () -> warningLevel, v -> warningLevel = v);
      parser
          .option("--debug")
          .usage(
              "Enable debugging options. Property renaming uses long mangled names which can be "
                  + "mapped back to the original name.")
          .bool(() -> debug, v -> debug = v);
      parser
          .option("--typed_ast_output_file")
          .hidden(true)
          .usage("Sets file to output in-progress typedAST format. DO NOT USE!")
          .string(() -> typedAstOutputFile, v -> typedAstOutputFile = v);
      parser
          .option("--generate_exports")
          .usage("Generates export code for those marked with @export")
          .bool(() -> generateExports, v -> generateExports = v);
      parser
          .option("--export_local_property_definitions")
          .usage("Generates export code for local properties marked with @export")
          .bool(() -> exportLocalPropertyDefinitions, v -> exportLocalPropertyDefinitions = v);
      parser
          .option("--formatting")
          .usage(
              "Specifies which formatting options, if any, should be "
                  + "applied to the output JS. Options: "
                  + "PRETTY_PRINT, PRINT_INPUT_DELIMITER, SINGLE_QUOTES")
          .enumList(FormattingOption.values(), () -> formatting, v -> formatting = v);
      parser
          .option("--process_common_js_modules")
          .usage("Process CommonJS modules to a concatenable form.")
          .bool(() -> processCommonJsModules, v -> processCommonJsModules = v);
      parser
          .option("--js_module_root")
          .usage("Path prefixes to be removed from ES6 & CommonJS modules.")
          .stringList(() -> moduleRoot, v -> moduleRoot = v);
      parser
          .option("--process_closure_primitives")
          .usage(
              "Processes built-ins from the Closure library, such as "
                  + "goog.require(), goog.provide(), and goog.exportSymbol(). "
                  + "True by default.")
          .bool(() -> processClosurePrimitives, v -> processClosurePrimitives = v);
      parser
          .option("--angular_pass")
          .usage(
              "Generate $inject properties for AngularJS for functions " + "annotated with @ngInject")
          .bool(() -> angularPass, v -> angularPass = v);
      parser
          .option("--polymer_version")
          .usage("Which version of Polymer is being used (1 or 2).")
          .integer(() -> polymerVersion, v -> polymerVersion = v);
      parser
          .option("--chrome_pass")
          .hidden(true)
          .usage("Enable Chrome-specific options for handling cr.* functions.")
          .bool(() -> chromePass, v -> chromePass = v);
      parser
          .option("--j2cl_pass")
          .hidden(true)
          .usage(
              "Rewrite J2CL output to be compiler-friendly if enabled (ON or AUTO). "
                  + "Options:OFF, ON, AUTO(default)")
          .string(() -> j2clPassMode, v -> j2clPassMode = v);
      parser
          .option("--remove_j2cl_asserts")
          .hidden(true)
          .usage("Remove calls to J2CL assertions.")
          .flag(() -> removeJ2cLAsserts, v -> removeJ2cLAsserts = v);
      parser
          .option("--output_manifest")
          .usage(
              "Prints out a list of all the files in the compilation. "
                  + "If --dependency_mode=PRUNE or PRUNE_LEGACY is specified, this will not include "
                  + "files that got dropped because they were not required. "
                  + "The %outname% placeholder expands to the JS output file. "
                  + "If you're using modularization, using %outname% will create "
                  + "a manifest for each module.")
          .string(() -> outputManifest, v -> outputManifest = v);
      parser
          .option("--output_chunk_dependencies")
          .usage("Prints out a JSON file of dependencies between chunks.")
          .string(() -> outputChunkDependencies, v -> outputChunkDependencies = v);
      parser
          .option("--language_in")
          .usage(
              "Sets the language spec to which input sources should conform. "
                  + "Options: ECMASCRIPT3, ECMASCRIPT5, ECMASCRIPT5_STRICT, "
                  + "ECMASCRIPT_2015, ECMASCRIPT_2016, ECMASCRIPT_2017, "
                  + "ECMASCRIPT_2018, ECMASCRIPT_2019, ECMASCRIPT_2020,"
                  + "ECMASCRIPT_2021, STABLE, ECMASCRIPT_NEXT (latest features supported),"
                  + "UNSTABLE (for testing only)")
          .string(() -> languageIn, v -> languageIn = v);
      parser
          .option("--language_out")
          .usage(
              "Sets the language spec to which output should conform. "
                  + "Options: ECMASCRIPT3, ECMASCRIPT5, "
                  + "ECMASCRIPT_2015, ECMASCRIPT_2016, ECMASCRIPT_2017, "
                  + "ECMASCRIPT_2018, ECMASCRIPT_2019, ECMASCRIPT_2020, "
                  + "ECMASCRIPT_2021, STABLE, ECMASCRIPT_NEXT (latest features supported)")
          .string(() -> languageOut, v -> languageOut = v);
      parser
          .option("--version")
          .usage("Prints the compiler version to stdout and exit.")
          .bool(() -> version, v -> version = v);
      parser
          .option("--translations_file")
          .hidden(true)
          .usage("Source of translated messages. Currently only supports XTB.")
          .string(() -> translationsFile, v -> translationsFile = v);
      parser
          .option("--translations_project")
          .hidden(true)
          .usage(
              "Scopes all translations to the specified project."
                  + "When specified, we will use different message ids so that messages "
                  + "in different projects can have different translations.")
          .string(() -> translationsProject, v -> translationsProject = v);
      parser
          .option("--flagfile")
          .hidden(true)
          .usage("A file (or files) containing additional command-line options.")
          .stringList(() -> flagFiles, v -> flagFiles = v);
      parser
          .option("--warnings_allowlist_file")
          .aliases("--warnings_whitelist_file")
          .usage(
              "A file containing warnings to suppress. Each line should be "
                  + "of the form\n"
                  + "<file-name>:<line-number>?  <warning-description>")
          .string(() -> warningsAllowlistFile, v -> warningsAllowlistFile = v);
      parser
          .option("--hide_warnings_for")
          .usage(
              "If specified, files whose path contains this string will "
                  + "have their warnings hidden. You may specify multiple.")
          .stringList(() -> hideWarningsFor, v -> hideWarningsFor = v);
      parser
          .option("--extra_annotation_name")
          .usage("A allowlist of tag names in JSDoc. You may specify multiple")
          .stringList(() -> extraAnnotationName, v -> extraAnnotationName = v);
      parser
          .option("--tracer_mode")
          .hidden(true)
          .usage(
              "Shows the duration of each compiler pass and the impact to "
                  + "the compiled output size. "
                  + "Options: ALL, AST_SIZE, RAW_SIZE, TIMING_ONLY, OFF")
          .enumValue(CompilerOptions.TracerMode.values(), () -> tracerMode, v -> tracerMode = v);
      parser
          .option("--rename_variable_prefix")
          .usage("Specifies a prefix that will be prepended to all variables.")
          .string(() -> renamePrefix, v -> renamePrefix = v);
      parser
          .option("--rename_prefix_namespace")
          .usage(
              "Specifies the name of an object that will be used to store all "
                  + "non-extern globals")
          .string(() -> renamePrefixNamespace, v -> renamePrefixNamespace = v);
      parser
          .option("--conformance_configs")
          .usage("A list of JS Conformance configurations in text protocol buffer format.")
          .stringList(() -> conformanceConfigs, v -> conformanceConfigs = v);
      parser
          .option("--env")
          .usage(
              "Determines the set of builtin externs to load. "
                  + "Options: BROWSER, CUSTOM. Defaults to BROWSER.")
          .enumValue(CompilerOptions.Environment.values(), () -> environment, v -> environment = v);
      parser
          .option("--json_streams")
          .usage(
              "Specifies whether standard input and output streams will be "
                  + "a JSON array of sources. Each source will be an object of the "
                  + "form {path: filename, src: file_contents, source_map: srcmap_contents }. "
                  + "Intended for use by stream-based build systems such as gulpjs. "
                  + "Options: NONE, IN, OUT, BOTH. Defaults to NONE.")
          .enumValue(
              CompilerOptions.JsonStreamMode.values(),
              () -> jsonStreamMode,
              v -> jsonStreamMode = v);
      parser
          .option("--preserve_type_annotations")
          .hidden(true)
          .usage("Preserves type annotations.")
          .bool(() -> preserveTypeAnnotations, v -> preserveTypeAnnotations = v);
      parser
          .option("--inject_libraries")
          .usage("Allow injecting runtime libraries.")
          .bool(() -> injectLibraries, v -> injectLibraries = v);
      parser
          .option("--force_inject_library")
          .usage(
              "Force injection of named runtime libraries. "
                  + "The format is <name> where <name> is the name of a runtime library. "
                  + "Possible libraries include: base, es6_runtime")
          .stringList(() -> forceInjectLibraries, v -> forceInjectLibraries = v);
      parser
          .option("--dependency_mode")
          .usage(
              "Specifies how the compiler should determine the set and order of files for a "
                  + "compilation. Options: NONE the compiler will include all src files in the order "
                  + "listed, SORT_ONLY the compiler will include all source files in dependency "
                  + "order, PRUNE files will only be included if they are transitive dependencies "
                  + "of files listed in the --entry_point flag and then sorted in dependency order, "
                  + "PRUNE_LEGACY same as PRUNE but files that do not goog.provide a namespace and "
                  + "are not modules will be automatically added as --entry_point entries. Defaults "
                  + "to PRUNE_LEGACY if entry points are defined, otherwise to NONE.")
          .enumValue(DependencyMode.values(), () -> dependencyMode, v -> dependencyMode = v);
      parser
          .option("--entry_point")
          .usage(
              "A file or namespace to use as the starting point for determining "
                  + "which src files to include in the compilation. ES6 and CommonJS "
                  + "modules are specified as file paths (without the extension). "
                  + "Closure-library namespaces are specified with a \"goog:\" prefix. "
                  + "Example: --entry_point=goog:goog.Promise")
          .stringList(() -> entryPoint, v -> entryPoint = v);
      parser
          .option("--rewrite_polyfills")
          .usage(
              "Injects polyfills for ES2015+ library classes and methods used in source. See also"
                  + " the \"Polyfills\" GitHub Wiki page.")
          .bool(() -> rewritePolyfills, v -> rewritePolyfills = v);
      parser
          .option("--isolate_polyfills")
          .usage(
              "Hides injected polyfills from the global scope and any external code. See the"
                  + " the \"Polyfills\" GitHub Wiki page for details.")
          .bool(() -> isolatePolyfills, v -> isolatePolyfills = v);
      parser
          .option("--print_source_after_each_pass")
          .hidden(true)
          .usage("Whether to iteratively print resulting JS source per pass.")
          .bool(() -> printSourceAfterEachPass, v -> printSourceAfterEachPass = v);
      parser
          .option("--module_resolution")
          .usage(
              "Specifies how the compiler locates modules. BROWSER requires all module imports "
                  + "to begin with a '.' or '/' and have a file extension. NODE uses the node module "
                  + "rules. WEBPACK looks up modules from a special lookup map.")
          .enumValue(
              ModuleLoader.ResolutionMode.values(),
              () -> moduleResolutionMode,
              v -> moduleResolutionMode = v);
      parser
          .option("--browser_resolver_prefix_replacements")
          .usage(
              "Prefixes to replace in ES6 import paths before resolving. "
                  + "module_resolution must be BROWSER_WITH_TRANSFORMED_PREFIXES to take effect.")
          .map(() -> browserResolverPrefixReplacements, v -> browserResolverPrefixReplacements = v);
      parser
          .option("--package_json_entry_names")
          .usage(
              "Ordered list of entries to look for in package.json files when processing modules"
                  + " with the NODE module resolution strategy (i.e. esnext:main,browser,main)."
                  + " Defaults to a list with the following entries: \"browser\", \"module\","
                  + " \"main\".")
          .string(() -> packageJsonEntryNames, v -> packageJsonEntryNames = v);
      parser
          .option("--error_format")
          .usage("Specifies format for error messages.")
          .enumValue(ErrorFormatOption.values(), () -> errorFormat, v -> errorFormat = v);
      parser
          .option("--renaming")
          .usage("Disables variable renaming. Cannot be used with ADVANCED optimizations.")
          .bool(() -> renaming, v -> renaming = v);
      parser
          .option("--help_markdown")
          .hidden(true)
          .usage("Prints markdown formatted flag usage")
          .bool(() -> helpMarkdown, v -> helpMarkdown = v);
      parser
          .option("--instrument_for_coverage_option")
          .usage(
              """
              Enable code instrumentation to perform code coverage analysis. Options are:
               1. NONE (default)
               2. LINE - Instrument code by line.
               3. BRANCH - Instrument code by branch.
               4. PRODUCTION - Function Instrumentation on compiled JS code.
              """)
          .string(() -> instrumentForCoverageOption, v -> instrumentForCoverageOption = v);
      parser
          .option("--production_instrumentation_array_name")
          .usage(
              "Name of the global array used by production instrumentation. The array name should be"
                  + " declared as an extern so it is not renamed by the compiler. A function that"
                  + " parses the global array should also be included. This flag is to be used in"
                  + " tandem with --instrument_code=PRODUCTION")
          .string(
              () -> productionInstrumentationArrayName,
              v -> productionInstrumentationArrayName = v);
      parser
          .option("--chunk_output_type")
          .usage(
              "Indicates what format the compiler should use for output chunks. GLOBAL_NAMESPACE is "
                  + "typically used in conjunction with --rename_prefix_namespace. ES_MODULES "
                  + "outputs chunks as proper modules with 'import' and 'export' statements.")
          .enumValue(ChunkOutputType.values(), () -> chunkOutputType, v -> chunkOutputType = v);
      parser
          .option("--allow_dynamic_import")
          .usage(
              "Indicates that the compiler should allow dynamic import expressions. Dynamic import "
                  + "expressions are not yet fully supported and may lead to broken output code.")
          .bool(() -> allowDynamicImport, v -> allowDynamicImport = v);
      parser
          .option("--dynamic_import_alias")
          .usage(
              "Instructs the compiler to replace dynamic imports expressions with a function call "
                  + "using the specified name. Allows dynamic import expressions to be externally "
                  + "polyfilled when the output language level does not natively support them. "
                  + "An alias of 'import' is allowed.")
          .string(() -> dynamicImportAlias, v -> dynamicImportAlias = v);
      parser
          .option("--assume_static_inheritance_is_not_used")
          .usage(
              "Assume that static (class-side) inheritance is not being used and that static"
                  + " methods will not be referenced via `this` or through subclasses. This enables"
                  + " optimizations that could break code that did those things.")
          .bool(() -> assumeStaticInheritanceIsNotUsed, v -> assumeStaticInheritanceIsNotUsed = v);
      parser
          .option("--assume_no_prototype_method_enumeration")
          .usage(
              "Assume that prototype method enumeration is not being used. This allows the compiler "
                  + "to move a prototype method declaration into a deeper chunk without creating "
                  + "stub functions in a parent chunk.")
          .bool(
              () -> assumeNoPrototypeMethodEnumeration,
              v -> assumeNoPrototypeMethodEnumeration = v);
      parser
          .option("--variable_map_input_file")
          .usage(
              "File containing the serialized version of the variable "
                  + "renaming map produced by a previous compilation")
          .string(() -> variableMapInputFile, v -> variableMapInputFile = v);
      parser
          .option("--property_map_input_file")
          .usage(
              "File containing the serialized version of the property "
                  + "renaming map produced by a previous compilation")
          .string(() -> propertyMapInputFile, v -> propertyMapInputFile = v);
      parser
          .option("--expected_diagnostics")
          .usage(
              "Expected diagnostics in the format [(line,col)]CODE:regex. "
                  + "CODE is the JSCompiler DiagnosticType key. regex matches the description.")
          .stringList(() -> expectedDiagnostics, v -> expectedDiagnostics = v);
      parser.argument(() -> arguments, v -> arguments = v);
    }

    /** Parse the given args list. */
    private void parse(List<String> args) throws CmdLineException {
      parser.parseArgument(args);

      compilationLevelParsed = CompilationLevel.fromString(Ascii.toUpperCase(compilationLevel));
      if (compilationLevelParsed == null) {
        throw new CmdLineException(
            parser, "Bad value for --compilation_level: " + compilationLevel);
      }

      instrumentCodeParsed =
          InstrumentOption.fromString(Ascii.toUpperCase(instrumentForCoverageOption));
      if (instrumentCodeParsed == null) {
        throw new CmdLineException(
            parser,
            "Bad value for --instrument_for_coverage_option: " + instrumentForCoverageOption);
      }
    }

    private static final ImmutableMultimap<String, String> categories =
        new ImmutableMultimap.Builder<String, String>()
            .putAll(
                "Basic Usage",
                ImmutableList.of(
                    "compilation_level",
                    "env",
                    "externs",
                    "js",
                    "js_output_file",
                    "language_in",
                    "language_out",
                    "warning_level"))
            .putAll(
                "Warning and Error Management",
                ImmutableList.of(
                    "conformance_configs",
                    "error_format",
                    "extra_annotation_name",
                    "hide_warnings_for",
                    "jscomp_error",
                    "jscomp_off",
                    "jscomp_warning",
                    "strict_mode_input",
                    "warnings_allowlist_file"))
            .putAll(
                "Output",
                ImmutableList.of(
                    "assume_function_wrapper",
                    "debug",
                    "emit_use_strict",
                    "export_local_property_definitions",
                    "formatting",
                    "generate_exports",
                    "isolation_mode",
                    "output_wrapper",
                    "output_wrapper_file",
                    "rename_variable_prefix"))
            .putAll("Dependency Management", ImmutableList.of("dependency_mode", "entry_point"))
            .putAll(
                "JS Modules",
                ImmutableList.of(
                    "dynamic_import_alias",
                    "js_module_root",
                    "module_resolution",
                    "process_common_js_modules",
                    "package_json_entry_names"))
            .putAll(
                "Library and Framework Specific",
                ImmutableList.of(
                    "angular_pass",
                    "force_inject_library",
                    "inject_libraries",
                    "polymer_version",
                    "process_closure_primitives",
                    "rewrite_polyfills",
                    "isolate_polyfills"))
            .putAll(
                "Code Splitting",
                ImmutableList.of(
                    "chunk",
                    "chunk_output_path_prefix",
                    "chunk_output_type",
                    "chunk_wrapper",
                    "rename_prefix_namespace"))
            .putAll(
                "Reports",
                ImmutableList.of(
                    "create_source_map",
                    "output_manifest",
                    "output_chunk_dependencies",
                    "property_renaming_report",
                    "source_map_input",
                    "source_map_include_content",
                    "source_map_location_mapping",
                    "variable_renaming_report"))
            .putAll(
                "Miscellaneous",
                ImmutableList.of(
                    "assume_static_inheritance_is_not_used",
                    "browser_featureset_year",
                    "charset",
                    "checks_only",
                    "define",
                    "flagfile",
                    "help",
                    "json_streams",
                    "third_party",
                    "use_types_for_optimization",
                    "version"))
            .build();

    private void printUsage(PrintStream ps) {
      OutputStreamWriter outputStream = new OutputStreamWriter(ps, UTF_8);

      boolean isFirst = true;
      for (Map.Entry<String, Collection<String>> entry : categories.asMap().entrySet()) {
        String prefix = "\n\n";
        String suffix = "";
        if (isFirst) {
          isFirst = false;
          prefix = "";
        }

        if (entry.getKey().equals("Warning and Error Management")) {
          if (helpMarkdown) {
            suffix =
                "\n## Available Error Groups\n\n"
                    + "  - "
                    + DiagnosticGroups.DIAGNOSTIC_GROUP_NAMES.replace(", ", "\n  - ");
          } else {
            suffix =
                "\n"
                    + BOLD_PREFIX
                    + "Available Error Groups: "
                    + NORMAL_PREFIX
                    + DiagnosticGroups.DIAGNOSTIC_GROUP_NAMES;
          }
        }

        if (this.helpMarkdown) {
          // For markdown docs we don't want any line wrapping so we just set a very
          // large line length.
          maxLineLength = 5000;
          parser.setUsageWidth(maxLineLength);
        }

        printCategoryUsage(entry.getKey(), entry.getValue(), outputStream, prefix, suffix);
      }

      ps.flush();
    }

    private static final String BOLD_PREFIX = "\033[1m";
    private static final String NORMAL_PREFIX = "\033[0m";
    private static final String MARKDOWN_CHARS_TO_ESCAPE = "[-*\\`\\[\\]{}\\(\\)#+\\.!<>]";

    private void printCategoryUsage(
        String categoryName,
        final Collection<String> options,
        OutputStreamWriter outputStream,
        String prefix,
        String suffix) {

      try {
        if (prefix != null) {
          printStringLineWrapped(prefix, outputStream);
        }

        if (this.helpMarkdown) {
          outputStream.write("# " + categoryName + "\n");

          for (String optionName : options) {
            StringWriter stringWriter = new StringWriter();
            parser.printUsage(
                stringWriter,
                (option) ->
                    !option.hidden() && optionName.equals(option.name().replaceFirst("^--", "")));
            stringWriter.flush();
            String rawOptionUsage = stringWriter.toString();
            int delimiterIndex = rawOptionUsage.indexOf(" : ");
            if (delimiterIndex > 0) {
              outputStream.write(
                  "\n**" + rawOptionUsage.substring(0, delimiterIndex).trim() + "**  \n");

              String optionDescription =
                  rawOptionUsage
                      .substring(delimiterIndex + 3)
                      .replaceAll(MARKDOWN_CHARS_TO_ESCAPE, "\\\\$0")
                      .trim();
              outputStream.write(optionDescription + "\n");
            } else {
              outputStream.write(rawOptionUsage.replaceAll(MARKDOWN_CHARS_TO_ESCAPE, "\\\\$0"));
            }
            outputStream.flush();
          }
        } else {
          outputStream.write(BOLD_PREFIX + categoryName + ":\n" + NORMAL_PREFIX);
          parser.printUsage(
              outputStream,
              (option) ->
                  !option.hidden() && options.contains(option.name().replaceFirst("^--", "")));
        }

        if (suffix != null) {
          printStringLineWrapped(suffix, outputStream);
        }
      } catch (IOException e) {
        // Ignore.
      }
    }

    private int maxLineLength = 80;
    private static final Pattern WHITESPACE_PATTERN = Pattern.compile("\\s");

    private void printStringLineWrapped(String input, OutputStreamWriter outputStream)
        throws IOException {
      if (input.length() < maxLineLength) {
        outputStream.write(input);
        return;
      }

      int endIndex = maxLineLength;
      String subString = input.substring(0, maxLineLength);
      Matcher whitespaceMatcher = WHITESPACE_PATTERN.matcher(subString);
      boolean foundMatch = false;
      while (whitespaceMatcher.find()) {
        endIndex = whitespaceMatcher.start();
        foundMatch = true;
      }
      outputStream.write(input.substring(0, endIndex) + "\n");
      printStringLineWrapped(
          "    " + input.substring(foundMatch ? endIndex + 1 : endIndex), outputStream);
    }

    private static void printShortUsageAfterErrors(PrintStream ps) {
      ps.print("Sample usage: ");
      ps.println(
          "--compilation_level (-O) VAL --externs VAL --js VAL"
              + " --js_output_file VAL"
              + " --warning_level (-W) [QUIET | DEFAULT | VERBOSE]");
      ps.println("Run with --help for all options and details");
      ps.flush();
    }

    /**
     * Users may specify JS inputs via the {@code --js} flag, as well as via additional arguments to
     * the Closure Compiler. For example, it is convenient to leverage the additional arguments
     * feature when using the Closure Compiler in combination with {@code find} and {@code xargs}:
     *
     * <pre>
     * find MY_JS_SRC_DIR -name '*.js' \
     *     | xargs java -jar compiler.jar --dependency_mode=PRUNE_LEGACY
     * </pre>
     *
     * The {@code find} command will produce a list of '*.js' source files in the {@code
     * MY_JS_SRC_DIR} directory while {@code xargs} will convert them to a single, space-delimited
     * set of arguments that are appended to the {@code java} command to run the Compiler.
     *
     * <p>Note that it is important to use the {@code --dependency_mode=PRUNE or PRUNE_LEGACY}
     * option in this case because the order produced by {@code find} is unlikely to be sorted
     * correctly with respect to {@code goog.provide()} and {@code goog.requires()}.
     */
    protected List<FlagEntry<JsSourceType>> getMixedJsSources()
        throws CmdLineException, IOException {
      List<FlagEntry<JsSourceType>> mixedSources = new ArrayList<>();
      Set<String> excludes = new LinkedHashSet<>();
      for (FlagEntry<JsSourceType> source : mixedJsSources) {
        if (source.getValue().endsWith(".zip")) {
          mixedSources.add(source);
        } else if (source.getValue().startsWith("!")) {
          for (String filename : findJsFiles(ImmutableList.of(source.getValue().substring(1)))) {
            excludes.add(filename);
            mixedSources.remove(new FlagEntry<>(JsSourceType.JS, filename));
          }
        } else {
          for (String filename : findJsFiles(Collections.singletonList(source.getValue()), true)) {
            if (!excludes.contains(filename)) {
              mixedSources.add(new FlagEntry<>(JsSourceType.JS, filename));
            }
          }
        }
      }
      List<String> fromArguments = findJsFiles(arguments);
      for (String filename : fromArguments) {
        mixedSources.add(new FlagEntry<>(JsSourceType.JS, filename));
      }
      if (!mixedJsSources.isEmpty() && !arguments.isEmpty() && mixedSources.isEmpty()) {
        throw new CmdLineException(parser, "No inputs matched");
      }
      return mixedSources;
    }

    List<LocationMapping> getSourceMapLocationMappings() throws CmdLineException {
      ImmutableList.Builder<LocationMapping> locationMappings = ImmutableList.builder();

      ImmutableMap<String, String> split =
          splitPipeParts(sourceMapLocationMapping, "--source_map_location_mapping");
      for (Map.Entry<String, String> mapping : split.entrySet()) {
        locationMappings.add(
            new SourceMap.PrefixLocationMapping(mapping.getKey(), mapping.getValue()));
      }

      return locationMappings.build();
    }

    ImmutableMap<String, String> getSourceMapInputs() throws CmdLineException {
      return splitPipeParts(sourceMapInputs, "--source_map_input");
    }

    private ImmutableMap<String, String> splitPipeParts(Iterable<String> input, String flagName)
        throws CmdLineException {
      ImmutableMap.Builder<String, String> result = new ImmutableMap.Builder<>();

      Splitter splitter = Splitter.on('|').limit(2);
      for (String inputSourceMap : input) {
        List<String> parts = splitter.splitToList(inputSourceMap);
        if (parts.size() != 2) {
          throw new CmdLineException(
              parser, "Bad value for " + flagName + " (duplicate key): " + input);
        }
        result.put(parts.get(0), parts.get(1));
      }

      return result.buildOrThrow();
    }

    List<String> getPackageJsonEntryNames() throws CmdLineException {
      return Splitter.on(',').splitToList(packageJsonEntryNames);
    }

    /**
     * Appends a value to the values of a flag that may be given several times, and records it
     * together with the kind of the flag in the list that preserves the original order of such
     * flags.
     *
     * @return the list of values of the flag, which is created if it was null
     */
    private static <T> List<String> addMultiFlagValue(
        @Nullable List<String> values, String value, T flag, List<FlagEntry<T>> entries) {
      // On windows, some quoted values seem to preserve the quotes as part of the value.
      String normalizedValue = value;
      if (value != null
          && value.length() > 0
          && (value.substring(0, 1).equals("'") || value.substring(0, 1).equals("\""))
          && value.substring(value.length() - 1).equals(value.substring(0, 1))) {
        normalizedValue = value.substring(1, value.length() - 1);
      }
      if (values == null) {
        values = new ArrayList<>();
      }
      values.add(normalizedValue);
      entries.add(new FlagEntry<>(flag, normalizedValue));
      return values;
    }
  }

  /** Set of options that can be used with the --formatting flag. */
  public static enum FormattingOption {
    PRETTY_PRINT,
    PRINT_INPUT_DELIMITER,
    SINGLE_QUOTES;

    private void applyToOptions(CompilerOptions options) {
      switch (this) {
        case PRETTY_PRINT -> options.setPrettyPrint(true);
        case PRINT_INPUT_DELIMITER -> options.setPrintInputDelimiter(true);
        case SINGLE_QUOTES -> options.setPreferSingleQuotes(true);
      }
    }
  }

  private final Flags flags = new Flags();

  private boolean errors = false;

  private boolean runCompiler = false;

  /** Cached error stream to avoid passing it as a parameter to helper functions. */
  private @Nullable PrintStream errorStream;

  /**
   * Create a new command-line runner. You should only need to call the constructor if you're
   * extending this class. Otherwise, the main method should instantiate it.
   */
  protected CommandLineRunner(String[] args) {
    super();
    initConfigFromFlags(args, System.out, System.err);
  }

  protected CommandLineRunner(String[] args, PrintStream out, PrintStream err) {
    super(out, err);
    initConfigFromFlags(args, out, err);
  }

  protected CommandLineRunner(String[] args, InputStream in, PrintStream out, PrintStream err) {
    super(in, out, err);
    initConfigFromFlags(args, out, err);
  }

  private static List<String> processArgs(String[] args) {
    // CmdLineParser has a different format that the old command-line parser.
    // So we use some voodoo to get the args into the format that CmdLineParser
    // expects.
    Pattern argPattern = Pattern.compile("(--?[a-zA-Z_]+)=(.*)", Pattern.DOTALL);
    Pattern quotesPattern = Pattern.compile("^['\"](.*)['\"]$");
    List<String> processedArgs = new ArrayList<>();

    for (String arg : args) {
      Matcher matcher = argPattern.matcher(arg);
      if (matcher.matches()) {
        processedArgs.add(matcher.group(1));

        String value = matcher.group(2);
        Matcher quotesMatcher = quotesPattern.matcher(value);
        if (quotesMatcher.matches()) {
          processedArgs.add(quotesMatcher.group(1));
        } else {
          processedArgs.add(value);
        }
      } else {
        processedArgs.add(arg);
      }
    }

    return processedArgs;
  }

  private void reportError(String message) {
    errors = true;
    errorStream.println(message);
    errorStream.flush();
  }

  private void processFlagFiles() throws CmdLineException {
    for (String flagFile : flags.flagFiles) {
      try {
        processFlagFile(flagFile);
      } catch (IOException ioErr) {
        reportError("ERROR - " + flagFile + " read error.");
      }
    }
  }

  private void processFlagFile(String flagFileString) throws CmdLineException, IOException {
    Path flagFile = Path.of(flagFileString);

    BufferedReader buffer = java.nio.file.Files.newBufferedReader(flagFile, UTF_8);
    // Builds the tokens.
    StringBuilder builder = new StringBuilder();
    // Stores the built tokens.
    List<String> tokens = new ArrayList<>();
    // Indicates if we are in a "quoted" token.
    boolean quoted = false;
    // Indicates if the char being processed has been escaped.
    boolean escaped = false;
    // Indicates whether this is the beginning of the file.
    boolean isFirstCharacter = true;

    int c;

    while (true) {
      c = buffer.read();
      if (c == -1) {
        break;
      }

      // Ignoring the BOM.
      if (isFirstCharacter) {
        isFirstCharacter = false;
        if (c == UTF8_BOM_CODE) {
          continue;
        }
      }

      if (c == 32 || c == 9 || c == 10 || c == 13) {
        if (quoted) {
          builder.append((char) c);
        } else if (builder.length() != 0) {
          tokens.add(builder.toString());
          builder.setLength(0);
        }
      } else if (c == 34) {
        if (escaped) {
          if (quoted) {
            builder.setCharAt(builder.length() - 1, (char) c);
          } else {
            builder.append((char) c);
          }
        } else {
          quoted = !quoted;
        }
      } else {
        builder.append((char) c);
      }

      escaped = c == 92;
    }

    buffer.close();

    if (builder.length() != 0) {
      tokens.add(builder.toString());
    }

    flags.flagFiles = new ArrayList<>();

    tokens = processArgs(tokens.toArray(new String[0]));

    // Command-line warning levels should override flag file settings,
    // which means they should go last.
    List<FlagEntry<CheckLevel>> previousGuardLevels = new ArrayList<>(flags.guardLevels);
    List<FlagEntry<JsSourceType>> previousMixedJsSources = new ArrayList<>(flags.mixedJsSources);
    flags.guardLevels.clear();
    flags.mixedJsSources.clear();
    flags.parse(tokens);
    flags.guardLevels.addAll(previousGuardLevels);
    flags.mixedJsSources.addAll(previousMixedJsSources);

    // Currently we are not supporting this (prevent direct/indirect loops)
    if (!flags.flagFiles.isEmpty()) {
      reportError("ERROR - Arguments in the file cannot contain " + "--flagfile option.");
    }
  }

  @Override
  protected final String getVersionText() {
    return String.join(
        "\n", //
        "Closure Compiler (http://github.com/google/closure-compiler)",
        // CommandLineRunnerVersion refers to class created from CommandLineRunnerVersion.template
        // with its static field COMPILER_VERSION substituted at build time.
        "Version: " + CommandLineRunnerVersion.COMPILER_VERSION);
  }

  private void initConfigFromFlags(String[] args, PrintStream out, PrintStream err) {

    errorStream = err;
    List<String> processedArgs = processArgs(args);

    flags.guardLevels.clear();
    flags.mixedJsSources.clear();

    List<FlagEntry<JsSourceType>> mixedSources = null;
    List<LocationMapping> mappings = null;
    ImmutableMap<String, String> sourceMapInputs = null;
    boolean parseInlineSourceMaps = false;
    boolean applyInputSourceMaps = false;
    try {
      flags.parse(processedArgs);

      processFlagFiles();

      mixedSources = flags.getMixedJsSources();
      mappings = flags.getSourceMapLocationMappings();
      sourceMapInputs = flags.getSourceMapInputs();
      parseInlineSourceMaps = flags.parseInlineSourceMaps;
      applyInputSourceMaps = flags.applyInputSourceMaps;
    } catch (CmdLineException e) {
      reportError(e.getMessage());
    } catch (IOException ioErr) {
      reportError("ERROR - ioException: " + ioErr);
    }

    if (flags.processCommonJsModules) {
      flags.processClosurePrimitives = true;
    }

    if (flags.browserFeaturesetYear != 0 && flags.languageOut != "ECMASCRIPT_NEXT") {
      throw new FlagUsageException(
          "ERROR - both flags `--browser_featureset_year` and `--language_out` specified.");
    }

    if (flags.outputWrapper == null) {
      flags.outputWrapper = "";
    }

    if (flags.outputWrapperFile != null && !flags.outputWrapperFile.isEmpty()) {
      try {
        flags.outputWrapper = Files.asCharSource(new File(flags.outputWrapperFile), UTF_8).read();
      } catch (Exception e) {
        reportError("ERROR - invalid output_wrapper_file specified.");
      }
    }

    if (!flags.outputWrapper.isEmpty()
        && !flags.outputWrapper.contains(CommandLineRunner.OUTPUT_MARKER)) {
      reportError(
          "ERROR - invalid output_wrapper specified. Missing '"
              + CommandLineRunner.OUTPUT_MARKER
              + "'.");
    }

    if (!flags.outputWrapper.isEmpty() && flags.isolationMode != IsolationMode.NONE) {
      reportError("--output_wrapper and --isolation_mode may not be used together.");
    }

    if (flags.isolationMode == IsolationMode.IIFE) {
      flags.outputWrapper = "(function(){%output%}).call(this);";
    }

    // Handle --compilation_level=BUNDLE
    ImmutableList<String> bundleFiles = ImmutableList.of();
    boolean skipNormalOutputs = false;
    if (flags.compilationLevelParsed == CompilationLevel.BUNDLE) {
      if (flags.jsOutputFile.isEmpty()) {
        reportError("--compilation_level=BUNDLE cannot be used without a --js_output_file.");
      } else {
        bundleFiles = ImmutableList.of(flags.jsOutputFile);
        flags.jsOutputFile = "";
        skipNormalOutputs = true;
      }
    }

    CodingConvention conv;
    if (flags.thirdParty) {
      conv = CodingConventions.getDefault();
    } else if (flags.chromePass) {
      conv = new ChromeCodingConvention();
    } else {
      conv = new ClosureCodingConvention();
    }

    if (!flags.renaming
        && flags.compilationLevelParsed == CompilationLevel.ADVANCED_OPTIMIZATIONS) {
      reportError("ERROR - renaming cannot be disabled when ADVANCED_OPTIMIZATIONS is used.");
    }

    DependencyOptions dependencyOptions = null;
    try {
      dependencyOptions =
          DependencyOptions.fromFlags(
              flags.dependencyMode, flags.entryPoint, ImmutableList.of(), null, false, false);
    } catch (FlagUsageException e) {
      reportError(e.getMessage());
    }

    if (errors) {
      Flags.printShortUsageAfterErrors(errorStream);
    } else if (flags.displayHelp || flags.helpMarkdown) {
      flags.printUsage(out);
    } else {
      runCompiler = true;

      final CommandLineConfig config = getCommandLineConfig();
      config
          .setPrintVersion(flags.version)
          .setPrintTree(flags.printTree)
          .setPrintTreeJson(flags.printTreeJson)
          .setPrintAst(flags.printAst)
          .setJscompDevMode(flags.jscompDevMode)
          .setLoggingLevel(flags.loggingLevel)
          .setExterns(flags.externs)
          .setMixedJsSources(mixedSources)
          .setDefaultToStdin()
          .setJsOutputFile(flags.jsOutputFile)
          .setChunk(flags.chunk)
          .setVariableMapOutputFile(flags.variableMapOutputFile)
          .setCreateNameMapFiles(flags.createNameMapFiles)
          .setPropertyMapOutputFile(flags.propertyMapOutputFile)
          .setExpectedDiagnostics(flags.expectedDiagnostics)
          .setPropertyMapInputFile(flags.propertyMapInputFile)
          .setVariableMapInputFile(flags.variableMapInputFile)
          .setInstrumentationMappingFile(flags.instrumentationMappingOutputFile)
          .setCodingConvention(conv)
          .setSummaryDetailLevel(flags.summaryDetailLevel)
          .setOutputWrapper(flags.outputWrapper)
          .setChunkWrapper(flags.chunkWrapper)
          .setChunkOutputPathPrefix(flags.chunkOutputPathPrefix)
          .setCreateSourceMap(flags.createSourceMap)
          .setSourceMapFormat(flags.sourceMapFormat)
          .setSourceMapLocationMappings(mappings)
          .setSourceMapInputFiles(sourceMapInputs)
          .setParseInlineSourceMaps(parseInlineSourceMaps)
          .setApplyInputSourceMaps(applyInputSourceMaps)
          .setWarningGuards(flags.guardLevels)
          .setDefine(flags.define)
          .setBrowserFeaturesetYear(flags.browserFeaturesetYear)
          .setCharset(flags.charset)
          .setDependencyOptions(dependencyOptions)
          .setOutputManifest(ImmutableList.of(flags.outputManifest))
          .setOutputBundle(bundleFiles)
          .setSkipNormalOutputs(skipNormalOutputs)
          .setOutputChunkDependencies(flags.outputChunkDependencies)
          .setProcessCommonJSModules(flags.processCommonJsModules)
          .setModuleRoots(flags.moduleRoot)
          .setWarningsAllowlistFile(flags.warningsAllowlistFile)
          .setHideWarningsFor(flags.hideWarningsFor)
          .setAngularPass(flags.angularPass)
          .setJsonStreamMode(flags.jsonStreamMode)
          .setErrorFormat(flags.errorFormat);

      SegmentOfCompilationToRun segmentOfCompilationToRun = flags.segmentOfCompilationToRun;
      switch (segmentOfCompilationToRun) {
        case CHECKS -> {
          checkState(
              flags.filenameToRestoreFrom == null,
              "Cannot restore and run CHECKS segment of compilation");
          config.setSaveCompilationStateToFilename(flags.filenameToSaveTo, 1);
        }
        case OPTIMIZATIONS -> {
          config.setContinueSavedCompilationFileName(flags.filenameToRestoreFrom, 1);
          config.setSaveCompilationStateToFilename(flags.filenameToSaveTo, 2);
        }
        case FINALIZATIONS -> {
          checkState(
              flags.filenameToSaveTo == null,
              "Cannot run FINALIZATIONS segment of compilation and then save the result");
          config.setContinueSavedCompilationFileName(flags.filenameToRestoreFrom, 2);
        }
        case ENTIRE_COMPILATION -> {}
        default ->
            throw new IllegalStateException(
                "Cannot run %s segment of compilation: " + flags.segmentOfCompilationToRun);
      }
    }

    errorStream = null;
  }

  @Override
  protected void addAllowlistWarningsGuard(CompilerOptions options, File allowlistFile) {
    options.addWarningsGuard(AllowlistWarningsGuard.fromFile(allowlistFile));
  }

  @Override
  protected void checkChunkName(String name) {
    if (!TokenStream.isJSIdentifier(extraChunkNameChars.matcher(name).replaceAll("_"))) {
      throw new FlagUsageException("Invalid chunk name: '" + name + "'");
    }
  }

  @Override
  protected CompilerOptions createOptions() {
    CompilerOptions options = new CompilerOptions();

    if (!flags.languageIn.isEmpty()) {
      LanguageMode languageMode = LanguageMode.fromString(flags.languageIn);
      if (languageMode == LanguageMode.UNSUPPORTED) {
        throw new FlagUsageException(
            "Cannot specify the unsupported set of features for language_in.");
      }
      if (languageMode != null) {
        options.setLanguageIn(languageMode);
      } else {
        throw new FlagUsageException(
            "Unknown language `"
                + flags.languageIn
                + "' specified. Expected one of: "
                + LanguageMode.validCommandLineNames());
      }
    }

    LanguageMode languageMode = LanguageMode.fromString(flags.languageOut);
    if (languageMode == LanguageMode.UNSUPPORTED) {
      throw new FlagUsageException(
          "Cannot specify the unsupported set of features for language_out.");
    }
    if (languageMode != null) {
      options.setLanguageOut(languageMode);
    } else {
      throw new FlagUsageException(
          "Unknown language `"
              + flags.languageOut
              + "' specified. Expected one of: "
              + LanguageMode.validCommandLineNames());
    }

    options.setCodingConvention(new ClosureCodingConvention());

    options.setExtraAnnotationNames(flags.extraAnnotationName);

    CompilationLevel level = flags.compilationLevelParsed;
    level.setOptionsForCompilationLevel(options);

    if (flags.debug) {
      level.setDebugOptionsForCompilationLevel(options);
    }

    options.setNumParallelThreads(flags.numParallelThreads);

    options.setEnvironment(flags.environment);

    options.setChecksOnly(flags.checksOnly);
    if (flags.checksOnly) {
      options.setOutputJs(CompilerOptions.OutputJs.NONE);
    }

    options.setIncrementalChecks(flags.incrementalCheckMode);

    options.setContinueAfterErrors(flags.continueAfterErrors);

    if (flags.useTypesForOptimization) {
      level.setTypeBasedOptimizationOptions(options);
    }

    if (flags.assumeFunctionWrapper
        || flags.isolationMode == IsolationMode.IIFE
        || flags.chunkOutputType == ChunkOutputType.ES_MODULES) {
      level.setWrappedOutputOptimizations(options);
      options.setAssumeGlobalScopeIsIsolated(true);
    }

    if (flags.typedAstOutputFile != null) {
      options.setTypedAstOutputFile(Path.of(flags.typedAstOutputFile));
    }
    options.setGenerateExports(flags.generateExports);
    options.setExportLocalPropertyDefinitions(flags.exportLocalPropertyDefinitions);

    WarningLevel wLevel = flags.warningLevel;
    wLevel.setOptionsForWarningLevel(options);
    for (FormattingOption formattingOption : flags.formatting) {
      formattingOption.applyToOptions(options);
    }

    options.setClosurePass(flags.processClosurePrimitives);

    options.setAngularPass(flags.angularPass);

    options.setPolymerVersion(flags.polymerVersion);

    options.setChromePass(flags.chromePass);

    if (!flags.j2clPassMode.isEmpty()) {
      try {
        CompilerOptions.J2clPassMode j2clPassMode =
            CompilerOptions.J2clPassMode.valueOf(Ascii.toUpperCase(flags.j2clPassMode));
        options.setJ2clPass(j2clPassMode);
      } catch (IllegalArgumentException ex) {
        throw new FlagUsageException(
            "Unknown J2clPassMode `" + flags.j2clPassMode + "' specified.");
      }
    }

    options.setRemoveJ2clAsserts(flags.removeJ2cLAsserts);

    options.setRenamePrefix(flags.renamePrefix);

    options.setRenamePrefixNamespace(flags.renamePrefixNamespace);

    options.setPreserveTypeAnnotations(flags.preserveTypeAnnotations);

    options.setPreventLibraryInjection(!flags.injectLibraries);

    if (!flags.forceInjectLibraries.isEmpty()) {
      options.setForceLibraryInjection(flags.forceInjectLibraries);
    }

    options.setRewritePolyfills(
        flags.rewritePolyfills
            && options.getLanguageIn().toFeatureSet().contains(FeatureSet.ES2015));
    options.setIsolatePolyfills(flags.isolatePolyfills);

    if (!flags.translationsFile.isEmpty()) {
      try {
        options.setMessageBundle(
            new XtbMessageBundle(
                new FileInputStream(flags.translationsFile), flags.translationsProject));
      } catch (IOException e) {
        throw new RuntimeException("Reading XTB file", e);
      }
    } else if (CompilationLevel.ADVANCED_OPTIMIZATIONS == level) {
      // In SIMPLE or WHITESPACE mode, if the user hasn't specified a
      // translations file, they might reasonably try to write their own
      // implementation of goog.getMsg that makes the substitution at
      // run-time.
      //
      // In ADVANCED mode, goog.getMsg is going to be renamed anyway,
      // so we might as well inline it. But shut off the i18n warnings,
      // because the user didn't really ask for i18n.
      options.setMessageBundle(new EmptyMessageBundle());
      options.setWarningLevel(DiagnosticGroups.MSG_CONVENTIONS, CheckLevel.OFF);
    }

    options.setConformanceConfigs(loadConformanceConfigs(flags.conformanceConfigs));

    options.setPrintSourceAfterEachPass(flags.printSourceAfterEachPass);
    options.setTracerMode(flags.tracerMode);
    options.setStrictModeInput(flags.strictModeInput);
    options.setEmitUseStrict(flags.emitUseStrict);
    options.setSourceMapIncludeSourcesContent(flags.sourceMapIncludeSourcesContent);
    options.setModuleResolutionMode(flags.moduleResolutionMode);
    options.setBrowserResolverPrefixReplacements(
        ImmutableMap.copyOf(flags.browserResolverPrefixReplacements));

    if (flags.packageJsonEntryNames != null) {
      try {
        List<String> packageJsonEntryNames = flags.getPackageJsonEntryNames();
        options.setPackageJsonEntryNames(packageJsonEntryNames);
      } catch (CmdLineException e) {
        reportError("ERROR - invalid package_json_entry_names format specified.");
      }
    }

    if (!flags.renaming) {
      options.setVariableRenaming(VariableRenamingPolicy.OFF);
      options.setPropertyRenaming(PropertyRenamingPolicy.OFF);
    }

    if (flags.instrumentCodeParsed == InstrumentOption.PRODUCTION
        && Strings.isNullOrEmpty(flags.instrumentationMappingOutputFile)) {
      throw new FlagUsageException(
          "Expected --instrument_mapping_report to be set when "
              + "--instrument_for_coverage_option is set to Production");
    }

    if (!Strings.isNullOrEmpty(flags.instrumentationMappingOutputFile)
        && flags.instrumentCodeParsed != InstrumentOption.PRODUCTION) {
      throw new FlagUsageException(
          "Expected --instrument_for_coverage_option to be passed with PRODUCTION "
              + "when --instrument_mapping_report is set");
    }

    if (Strings.isNullOrEmpty(flags.productionInstrumentationArrayName)
        && flags.instrumentCodeParsed == InstrumentOption.PRODUCTION) {
      throw new FlagUsageException(
          "Expected --production_instrumentation_array_name to be set when "
              + "--instrument_for_coverage_option is set to Production");
    }
    options.setInstrumentForCoverageOption(flags.instrumentCodeParsed);
    options.setProductionInstrumentationArrayName(flags.productionInstrumentationArrayName);
    options.setAllowDynamicImport(flags.allowDynamicImport);
    options.setDynamicImportAlias(flags.dynamicImportAlias);
    options.setAssumeStaticInheritanceIsNotUsed(flags.assumeStaticInheritanceIsNotUsed);
    options.setCrossChunkCodeMotionNoStubMethods(flags.assumeNoPrototypeMethodEnumeration);

    if (flags.chunkOutputType == ChunkOutputType.ES_MODULES) {
      if (flags.renamePrefixNamespace != null) {
        throw new FlagUsageException(
            "Expected --rename_prefix_namespace not to be specified when "
                + "--chunk_output_type is set to ES_MODULES.");
      }

      if (flags.emitUseStrict) {
        throw new FlagUsageException(
            "Expected --emit_use_strict should not be specified when "
                + "--chunk_output_type is set to ES_MODULES.");
      }

      options.setChunkOutputType(flags.chunkOutputType);
      options.setEmitUseStrict(false);

      if (level == CompilationLevel.ADVANCED_OPTIMIZATIONS) {
        options.setExtractPrototypeMemberDeclarations(
            ExtractPrototypeMemberDeclarationsMode.USE_CHUNK_TEMP);
      }
    }

    return options;
  }

  @Override
  protected Compiler createCompiler() {
    return new Compiler(getErrorPrintStream());
  }

  private ClosureBundler bundler;

  private ClosureBundler getBundler() {
    if (bundler != null) {
      return bundler;
    }

    ImmutableList<String> moduleRoots;
    if (!flags.moduleRoot.isEmpty()) {
      moduleRoots = ImmutableList.copyOf(flags.moduleRoot);
    } else {
      moduleRoots = ImmutableList.of(ModuleLoader.DEFAULT_FILENAME_PREFIX);
    }

    CompilerOptions options = createOptions();
    ClosureBundler newBundler =
        new ClosureBundler(
            Transpiler.NULL,
            new BaseTranspiler(
                new CompilerSupplier(
                    LanguageMode.ECMASCRIPT_NEXT.toFeatureSet().without(Feature.MODULES),
                    options.getModuleResolutionMode(),
                    moduleRoots,
                    options.getBrowserResolverPrefixReplacements()),
                /* runtimeLibraryName= */ ""));
    bundler = newBundler;
    return newBundler;
  }

  @Override
  protected void prepForBundleAndAppendTo(Appendable out, CompilerInput input, String content)
      throws IOException {
    getBundler().withPath(input.getName()).appendTo(out, input, content);
  }

  @Override
  protected void appendRuntimeTo(Appendable out) throws IOException {
    getBundler().appendRuntimeTo(out);
  }

  @Override
  protected List<SourceFile> createExterns(CompilerOptions options) throws IOException {
    List<SourceFile> externs = super.createExterns(options);
    if (isInTestMode()) {
      return externs;
    } else {
      List<SourceFile> builtinExterns = getBuiltinExterns(options.getEnvironment());
      builtinExterns.addAll(externs);
      return builtinExterns;
    }
  }

  private static ImmutableList<ConformanceConfig> loadConformanceConfigs(List<String> configPaths) {
    ImmutableList.Builder<ConformanceConfig> configs = ImmutableList.builder();

    for (String configPath : configPaths) {
      try {
        configs.add(loadConformanceConfig(configPath));
      } catch (IOException e) {
        throw new RuntimeException("Error loading conformance config", e);
      }
    }

    return configs.build();
  }

  private static ConformanceConfig loadConformanceConfig(String configFile) throws IOException {
    String textProto = Files.asCharSource(new File(configFile), UTF_8).read();

    ConformanceConfig.Builder builder = ConformanceConfig.newBuilder();

    // Looking for BOM.
    if (!textProto.isEmpty() && textProto.charAt(0) == UTF8_BOM_CODE) {
      // Stripping the BOM.
      textProto = textProto.substring(1);
    }

    try {
      TextFormat.merge(textProto, builder);
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
    return builder.build();
  }

  @InlineMe(
      replacement = "CommandLineRunner.getBuiltinExterns(CompilerOptions.Environment.BROWSER)",
      imports = {
        "com.google.javascript.jscomp.CommandLineRunner",
        "com.google.javascript.jscomp.CompilerOptions"
      })
  @Deprecated
  public static List<SourceFile> getDefaultExterns() throws IOException {
    return getBuiltinExterns(CompilerOptions.Environment.BROWSER);
  }

  /**
   * Returns all the JavaScript files from the set of patterns. The patterns support globs, such as
   * '*.js' for all JS files in a directory and '**.js' for all JS files within the directory and
   * sub-directories.
   */
  public static List<String> findJsFiles(Collection<String> patterns) throws IOException {
    return findJsFiles(patterns, false);
  }

  /**
   * Returns all the JavaScript files from the set of patterns.
   *
   * @param patterns A collection of filename patterns.
   * @param sortAlphabetically Whether the output filenames should be in alphabetical order.
   * @return The list of JS filenames found by expanding the patterns.
   */
  private static List<String> findJsFiles(Collection<String> patterns, boolean sortAlphabetically)
      throws IOException {
    // A map from normalized absolute paths to original paths. We need to return original paths to
    // support allowlist files that depend on them.
    Map<String, String> allJsInputs = sortAlphabetically ? new TreeMap<>() : new LinkedHashMap<>();
    Set<String> excludes = new LinkedHashSet<>();
    for (String pattern : patterns) {
      if (!pattern.contains("*") && !pattern.startsWith("!")) {
        File matchedFile = new File(pattern);
        if (matchedFile.isDirectory()) {
          matchPaths(new File(matchedFile, "**.js").toString(), allJsInputs, excludes);
        } else {
          Path original = Path.of(pattern);
          String pathStringAbsolute = original.normalize().toAbsolutePath().toString();
          if (!excludes.contains(pathStringAbsolute)) {
            allJsInputs.put(pathStringAbsolute, original.toString());
          }
        }
      } else {
        matchPaths(pattern, allJsInputs, excludes);
      }
    }

    return new ArrayList<>(allJsInputs.values());
  }

  private static void matchPaths(
      String pattern, final Map<String, String> allJsInputs, final Set<String> excludes)
      throws IOException {
    FileSystem fs = FileSystems.getDefault();
    final boolean remove = pattern.indexOf('!') == 0;
    if (remove) {
      pattern = pattern.substring(1);
    }

    String separator = File.separator.equals("\\") ? "\\\\" : File.separator;

    // Split the pattern into two pieces: the globbing part
    // and the non-globbing prefix.
    List<String> patternParts = Splitter.on(File.separator).splitToList(pattern);
    String prefix = ".";
    for (int i = 0; i < patternParts.size(); i++) {
      if (patternParts.get(i).contains("*")) {
        if (i > 0) {
          prefix = Joiner.on(separator).join(patternParts.subList(0, i));
          pattern = Joiner.on(separator).join(patternParts.subList(i, patternParts.size()));
        }
        break;
      }
    }

    final PathMatcher matcher = fs.getPathMatcher("glob:" + prefix + separator + pattern);
    java.nio.file.Files.walkFileTree(
        fs.getPath(prefix),
        EnumSet.of(FileVisitOption.FOLLOW_LINKS),
        Integer.MAX_VALUE,
        new SimpleFileVisitor<Path>() {
          @Override
          public FileVisitResult visitFile(Path p, BasicFileAttributes attrs) {
            if (matcher.matches(p) || matcher.matches(p.normalize())) {
              String pathStringAbsolute = p.normalize().toAbsolutePath().toString();
              if (remove) {
                excludes.add(pathStringAbsolute);
                allJsInputs.remove(pathStringAbsolute);
              } else if (!excludes.contains(pathStringAbsolute)) {
                allJsInputs.put(pathStringAbsolute, p.toString());
              }
            }
            return FileVisitResult.CONTINUE;
          }

          @Override
          public FileVisitResult visitFileFailed(Path file, IOException e) {
            return FileVisitResult.SKIP_SUBTREE;
          }
        });
  }

  /**
   * @return Whether the configuration is valid and specifies to run the compiler.
   */
  public boolean shouldRunCompiler() {
    return this.runCompiler;
  }

  /**
   * @return Whether the configuration has errors.
   */
  public boolean hasErrors() {
    return this.errors;
  }

  private static final Logger phaseLogger = Logger.getLogger(PhaseOptimizer.class.getName());

  /** Runs the Compiler. Exits cleanly in the event of an error. */
  public static void main(String[] args) {
    // disable any logging messages that can interfere with standard error reporting
    if (phaseLogger != null) {
      phaseLogger.setLevel(Level.OFF);
    }
    CommandLineRunner runner = new CommandLineRunner(args);
    if (runner.shouldRunCompiler()) {
      runner.run();
    }
    if (runner.hasErrors()) {
      System.exit(-1);
    }
  }
}
