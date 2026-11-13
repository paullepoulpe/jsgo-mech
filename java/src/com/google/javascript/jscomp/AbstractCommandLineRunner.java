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

import static com.google.common.base.Preconditions.checkArgument;
import static com.google.common.base.Preconditions.checkNotNull;
import static com.google.common.base.Preconditions.checkState;
import static java.lang.Math.min;

import com.google.common.annotations.VisibleForTesting;
import com.google.common.base.Ascii;
import com.google.common.base.Function;
import com.google.common.base.Joiner;
import com.google.common.base.Strings;
import com.google.common.base.Supplier;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import com.google.common.collect.Iterables;
import com.google.common.primitives.Doubles;
import com.google.errorprone.annotations.CanIgnoreReturnValue;
import com.google.javascript.jscomp.CodePrinter.LicenseTracker;
import com.google.javascript.jscomp.Compiler.ChunkGraphAwareLicenseTracker;
import com.google.javascript.jscomp.Compiler.ScriptNodeLicensesOnlyTracker;
import com.google.javascript.jscomp.Compiler.SingleBinaryLicenseTracker;
import com.google.javascript.jscomp.CompilerOptions.JsonStreamMode;
import com.google.javascript.jscomp.CompilerOptions.OutputJs;
import com.google.javascript.jscomp.CompilerOptions.SegmentOfCompilationToRun;
import com.google.javascript.jscomp.CompilerOptions.TweakProcessing;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.JSCompStrings;
import com.google.javascript.jscomp.base.Json;
import com.google.javascript.jscomp.base.Outcome;
import com.google.javascript.jscomp.base.SimpleJsonException;
import com.google.javascript.jscomp.base.SimpleJsonValue;
import com.google.javascript.jscomp.base.SimpleJsonWriter;
import com.google.javascript.jscomp.deps.ModuleLoader;
import com.google.javascript.jscomp.deps.SourceCodeEscapers;
import com.google.javascript.jscomp.ijs.IjsErrors;
import com.google.javascript.jscomp.js.RuntimeJsLibManager;
import com.google.javascript.jscomp.parsing.Config;
import com.google.javascript.rhino.Node;
import com.google.javascript.rhino.StaticSourceFile.SourceKind;
import com.google.javascript.rhino.TokenStream;
import com.google.re2j.Pattern;
import java.io.IOException;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.logging.Level;
import org.jspecify.annotations.Nullable;

/**
 * Implementations of AbstractCommandLineRunner translate flags into Java API calls on the Compiler.
 * AbstractCompiler contains common flags and logic to make that happen.
 *
 * <p>This class may be extended and used to create other Java classes that behave the same as
 * running the Compiler from the command line. Example:
 *
 * <pre>
 * class MyCommandLineRunner extends
 *     AbstractCommandLineRunner&lt;MyCompiler, MyOptions&gt; {
 *   MyCommandLineRunner(String[] args) {
 *     super(args);
 *   }
 *
 *   &#064;Override
 *   protected MyOptions createOptions() {
 *     MyOptions options = new MyOptions();
 *     CompilerFlagTranslator.setOptionsFromFlags(options);
 *     addMyCrazyCompilerPassThatOutputsAnExtraFile(options);
 *     return options;
 *   }
 *
 *   &#064;Override
 *   protected MyCompiler createCompiler() {
 *     return new MyCompiler();
 *   }
 *
 *   public static void main(String[] args) {
 *     (new MyCommandLineRunner(args)).run();
 *   }
 * }
 * </pre>
 */
public abstract class AbstractCommandLineRunner<A extends Compiler, B extends CompilerOptions> {
  private static final Pattern EQUALS_PATTERN = Pattern.compile("=");
  private static final Pattern COLON_PATTERN = Pattern.compile(":");
  private static final Pattern COMMA_PATTERN = Pattern.compile(",");

  static final DiagnosticType OUTPUT_SAME_AS_INPUT_ERROR =
      DiagnosticType.error(
          "JSC_OUTPUT_SAME_AS_INPUT_ERROR", "Bad output file (already listed as input file): {0}");

  static final DiagnosticType COULD_NOT_SERIALIZE_AST =
      DiagnosticType.error("JSC_COULD_NOT_SERIALIZE_AST", "Could not serialize ast to: {0}");

  static final DiagnosticType COULD_NOT_DESERIALIZE_AST =
      DiagnosticType.error("JSC_COULD_NOT_DESERIALIZE_AST", "Could not deserialize ast from: {0}");

  static final DiagnosticType NO_TREE_GENERATED_ERROR =
      DiagnosticType.error(
          "JSC_NO_TREE_GENERATED_ERROR", "Code contains errors. No tree was generated.");
  static final DiagnosticType INVALID_CHUNK_SOURCEMAP_PATTERN =
      DiagnosticType.error(
          "JSC_INVALID_CHUNK_SOURCEMAP_PATTERN",
          "When using --chunk or --module flags, the --create_source_map flag must contain "
              + "%outname% in the value.");

  static final DiagnosticType EXPECTED_DIAGNOSTIC_NOT_FOUND =
      DiagnosticType.error(
          "JSC_EXPECTED_DIAGNOSTIC_NOT_FOUND", "Expected diagnostic not found: {0}");

  static final DiagnosticType AMBIGUOUS_EXPECTATION =
      DiagnosticType.error(
          "JSC_AMBIGUOUS_EXPECTATION",
          "Multiple expected diagnostics matched the error: \"{0}\". Matches: \"{1}\"");

  static final String WAITING_FOR_INPUT_WARNING = "The compiler is waiting for input via stdin.";

  private final CommandLineConfig config;

  private final Io.Input in;

  private final Io.Output defaultJsOutput;

  private final Io.Output err;

  private A compiler;

  private String inputCharset;

  // NOTE(nicksantos): JSCompiler has always used ASCII as the default
  // output charset. This was done to solve legacy problems with
  // bad proxies, etc. We are not sure if these issues are still problems,
  // and changing the encoding would require a big obnoxious migration plan.
  //
  // New outputs should use outputCharset2, which is how I would have
  // designed this if I had a time machine.
  private Optional<String> outputCharset2 = Optional.empty();

  private Optional<String> legacyOutputCharset = Optional.empty();

  private boolean testMode = false;

  private @Nullable Supplier<List<SourceFile>> externsSupplierForTesting = null;

  private @Nullable Supplier<List<SourceFile>> inputsSupplierForTesting = null;

  private @Nullable Supplier<List<JSChunk>> chunksSupplierForTesting = null;

  private Function<Integer, Void> exitCodeReceiver = SystemExitCodeReceiver.INSTANCE;

  private @Nullable Map<String, String> rootRelativePathsMap = null;

  private @Nullable Map<String, String> parsedChunkWrappers = null;

  private @Nullable ImmutableMap<String, String> parsedChunkOutputFiles = null;

  private @Nullable ImmutableMap<String, String> parsedChunkConformanceFiles = null;


  static final String OUTPUT_MARKER = "%output%";
  private static final String OUTPUT_MARKER_JS_STRING = "%output|jsstring%";

  private final List<JsonFileSpec> filesToStreamOut = new ArrayList<>();

  AbstractCommandLineRunner() {
    this(Io.stdin(), Io.stdout(), Io.stderr());
  }

  AbstractCommandLineRunner(Io.Output out, Io.Output err) {
    this(Io.stdin(), out, err);
  }

  AbstractCommandLineRunner(Io.Input in, Io.Output out, Io.Output err) {
    this.config = new CommandLineConfig();
    this.in = checkNotNull(in);
    this.defaultJsOutput = checkNotNull(out);
    this.err = checkNotNull(err);
  }

  /**
   * Put the command line runner into test mode. In test mode, all outputs will be blackholed.
   *
   * @param externsSupplier A provider for externs.
   * @param inputsSupplier A provider for source inputs.
   * @param chunksSupplier A provider for chunks. Only one of inputsSupplier and chunksSupplier may
   *     be non-null.
   * @param exitCodeReceiver A receiver for the status code that would have been passed to
   *     System.exit in non-test mode.
   */
  @VisibleForTesting
  void enableTestMode(
      Supplier<List<SourceFile>> externsSupplier,
      Supplier<List<SourceFile>> inputsSupplier,
      Supplier<List<JSChunk>> chunksSupplier,
      Function<Integer, Void> exitCodeReceiver) {
    checkArgument(inputsSupplier == null != (chunksSupplier == null));
    testMode = true;
    this.externsSupplierForTesting = externsSupplier;
    this.inputsSupplierForTesting = inputsSupplier;
    this.chunksSupplierForTesting = chunksSupplier;
    this.exitCodeReceiver = exitCodeReceiver;
  }

  /**
   * @param newExitCodeReceiver receives a non-zero integer to indicate a problem during execution
   *     or 0i to indicate success.
   */
  public void setExitCodeReceiver(Function<Integer, Void> newExitCodeReceiver) {
    this.exitCodeReceiver = checkNotNull(newExitCodeReceiver);
  }

  /** Returns whether we're in test mode. */
  protected boolean isInTestMode() {
    return testMode;
  }

  /** Returns whether output should be a JSON stream. */
  private boolean isOutputInJson() {
    return config.jsonStreamMode == JsonStreamMode.OUT
        || config.jsonStreamMode == JsonStreamMode.BOTH;
  }

  /** Get the command line config, so that it can be initialized. */
  protected CommandLineConfig getCommandLineConfig() {
    return config;
  }

  /** Returns the instance of the Compiler to use when {@link #run()} is called. */
  protected abstract A createCompiler();

  /**
   * Performs any transformation needed on the given compiler input and appends it to the given
   * output bundle.
   */
  protected abstract Outcome<Void> prepForBundleAndAppendTo(
      Appendable out, CompilerInput input, String content) throws IOException;

  /** Writes whatever runtime libraries are needed to bundle. */
  protected abstract Outcome<Void> appendRuntimeTo(Appendable out) throws IOException;

  /**
   * Returns the instance of the Options to use when {@link #run()} is called. createCompiler() is
   * called before createOptions(), so getCompiler() will not return null when createOptions() is
   * called.
   */
  protected abstract Outcome<B> createOptions();

  /** The warning classes that are available from the command-line. */
  protected DiagnosticGroups getDiagnosticGroups() {
    if (compiler == null) {
      return new DiagnosticGroups();
    }
    return compiler.getDiagnosticGroups();
  }

  protected abstract void addAllowlistWarningsGuard(CompilerOptions options, String allowlistFile);

  protected static Outcome<Void> setWarningGuardOptions(
      CompilerOptions options,
      ArrayList<FlagEntry<CheckLevel>> warningGuards,
      DiagnosticGroups diagnosticGroups) {
    if (warningGuards != null) {
      final ImmutableSet<String> groupNames = DiagnosticGroups.getRegisteredGroups().keySet();
      for (FlagEntry<CheckLevel> entry : warningGuards) {
        if ("*".equals(entry.value)) {
          for (String groupName : groupNames) {
            if (!DiagnosticGroups.wildcardExcludedGroups.contains(groupName)) {
              diagnosticGroups.setWarningLevel(options, groupName, entry.flag);
            }
          }
        } else if (groupNames.contains(entry.value)) {
          diagnosticGroups.setWarningLevel(options, entry.value, entry.flag);
        } else {
          return Outcome.err(
              new FlagUsageException("Unknown diagnostic group: '" + entry.value + "'"));
        }
      }
    }
    return Outcome.ok();
  }

  /**
   * Sets options based on the configurations set flags API. Called during the run() run() method.
   * If you want to ignore the flags API, or interpret flags your own way, then you should override
   * this method.
   */
  protected Outcome<Void> setRunOptions(CompilerOptions options) throws IOException {
    DiagnosticGroups diagnosticGroups = getDiagnosticGroups();

    if (config.shouldSaveAfterStage1() || config.shouldContinueCompilation()) {
      if (options.isChecksOnly()) {
        return Outcome.err(new FlagUsageException(
            "checks_only mode is incompatible with multi-stage compilation"));
      }
      if (options.getExternExportsPath().isPresent()) {
        return Outcome.err(new FlagUsageException(
            "generating externs from exports is incompatible with multi-stage compilation"));
      }
    }

    Outcome<Void> setWarningGuardOptionsOutcome =
        setWarningGuardOptions(options, config.warningGuards, diagnosticGroups);
    if (setWarningGuardOptionsOutcome.isErr()) {
      return Outcome.err(setWarningGuardOptionsOutcome.error());
    }

    if (!config.warningsAllowFile.isEmpty()) {
      addAllowlistWarningsGuard(options, Io.pathString(config.warningsAllowFile));
    }

    if (!config.hideWarningsFor.isEmpty()) {
      options.addWarningsGuard(
          new ShowByPathWarningsGuard(
              config.hideWarningsFor.toArray(new String[] {}),
              ShowByPathWarningsGuard.ShowType.EXCLUDE));
    }

    List<String> define = new ArrayList<>(config.define);
    if (config.browserFeaturesetYear != 0) {
      Outcome<CompilerOptions.BrowserFeaturesetYear> yearRes =
          CompilerOptions.BrowserFeaturesetYear.fromResult(config.browserFeaturesetYear);
      if (yearRes.isErr()) {
        return Outcome.err(new FlagUsageException(yearRes.errorMessage().orElseThrow()));
      }
      options.setBrowserFeaturesetYearEnum(yearRes.get());
    }

    createDefineReplacements(define, options);

    options.setTweakProcessing(config.tweakProcessing);

    // TODO(tjgq): Unconditionally set the options.
    if (config.dependencyOptions != null) {
      options.setDependencyOptions(config.dependencyOptions);
    }

    options.setDevMode(config.jscompDevMode);
    options.setCodingConvention(config.codingConvention);
    options.setSummaryDetailLevel(config.summaryDetailLevel);
    options.setTrustedStrings(true);

    Outcome<String> legacyOutputCharsetOutcome = getLegacyOutputCharset();
    if (legacyOutputCharsetOutcome.isErr()) {
      return Outcome.err(legacyOutputCharsetOutcome.error());
    }
    legacyOutputCharset = Optional.ofNullable(legacyOutputCharsetOutcome.get());
    options.setOutputCharset(legacyOutputCharset);
    Outcome<String> outputCharset2Outcome = getOutputCharset2();
    if (outputCharset2Outcome.isErr()) {
      return Outcome.err(outputCharset2Outcome.error());
    }
    outputCharset2 = Optional.ofNullable(outputCharset2Outcome.get());
    Outcome<String> inputCharsetOutcome = getInputCharset();
    if (inputCharsetOutcome.isErr()) {
      return Outcome.err(inputCharsetOutcome.error());
    }
    inputCharset = inputCharsetOutcome.get();

    if (config.jsOutputFile.length() > 0) {
      if (config.skipNormalOutputs) {
        return Outcome.err(new FlagUsageException(
            "skip_normal_outputs and js_output_file cannot be used together."));
      }
    }

    if (config.skipNormalOutputs && config.printAst) {
      return Outcome.err(
          new FlagUsageException("skip_normal_outputs and print_ast cannot be used together."));
    }

    if (config.skipNormalOutputs && config.printTree) {
      return Outcome.err(
          new FlagUsageException("skip_normal_outputs and print_tree cannot be used together."));
    }

    if (config.skipNormalOutputs && config.printTreeJson) {
      return Outcome.err(new FlagUsageException(
          "skip_normal_outputs and print_tree_json_path cannot be used together."));
    }

    if (config.createSourceMap.length() > 0) {
      options.setSourceMapOutputPath(Optional.of(config.createSourceMap));
    } else if (isOutputInJson()) {
      options.setSourceMapOutputPath(Optional.of("%outname%"));
    }
    options.setSourceMapDetailLevel(config.sourceMapDetailLevel);
    options.setSourceMapFormat(config.sourceMapFormat);
    options.setSourceMapLocationMappings(config.sourceMapLocationMappings);
    options.setParseInlineSourceMaps(config.parseInlineSourceMaps);
    options.setApplyInputSourceMaps(config.applyInputSourceMaps);

    ImmutableMap.Builder<String, SourceMapInput> inputSourceMaps = new ImmutableMap.Builder<>();
    for (Map.Entry<String, String> files : config.sourceMapInputFiles.entrySet()) {
      SourceFile sourceMap =
          SourceFile.builder().withKind(SourceKind.NON_CODE).withPath(files.getValue()).build();
      inputSourceMaps.put(files.getKey(), new SourceMapInput(sourceMap));
    }
    options.setInputSourceMaps(inputSourceMaps.buildOrThrow());

    if (!config.variableMapInputFile.isEmpty()) {
      Outcome<VariableMap> variableMap = VariableMap.load(config.variableMapInputFile);
      if (variableMap.isErr()) {
        return Outcome.err(variableMap.error());
      }
      options.setInputVariableMap(variableMap.get());
    }

    if (!config.propertyMapInputFile.isEmpty()) {
      Outcome<VariableMap> propertyMap = VariableMap.load(config.propertyMapInputFile);
      if (propertyMap.isErr()) {
        return Outcome.err(propertyMap.error());
      }
      options.setInputPropertyMap(propertyMap.get());
    }

    if (!config.outputManifests.isEmpty()) {
      Set<String> uniqueNames = new LinkedHashSet<>();
      for (String filename : config.outputManifests) {
        if (!uniqueNames.add(filename)) {
          return Outcome.err(new FlagUsageException(
              "output_manifest flags specify duplicate file names: " + filename));
        }
      }
    }

    if (!config.outputBundles.isEmpty()) {
      Set<String> uniqueNames = new LinkedHashSet<>();
      for (String filename : config.outputBundles) {
        if (!uniqueNames.add(filename)) {
          return Outcome.err(new FlagUsageException(
              "output_bundle flags specify duplicate file names: " + filename));
        }
      }
    }

    options.setProcessCommonJSModules(config.processCommonJSModules);
    options.setModuleRoots(config.moduleRoots);
    options.setAngularPass(config.angularPass);

    if (!config.jsonWarningsFile.isEmpty()) {
      Outcome<Io.Output> jsonWarnings = Io.openOutput(config.jsonWarningsFile, Io.UTF_8, false);
      if (jsonWarnings.isErr()) {
        return Outcome.err(jsonWarnings.error());
      }
      options.addReportGenerator(new JsonErrorReportGenerator(jsonWarnings.get(), compiler));
    }

    if (config.errorFormat == CommandLineConfig.ErrorFormatOption.JSON) {
      JsonErrorReportGenerator errorGenerator =
          new JsonErrorReportGenerator(getErrorPrintStream(), compiler);
      compiler.setErrorManager(new SortingErrorManager(ImmutableSet.of(errorGenerator)));
    }
    if (config.printTree || config.printTreeJson) {
      options.setParseJsDocDocumentation(Config.JsDocParsing.INCLUDE_ALL_COMMENTS);
    }
    if (config.skipNormalOutputs) {
      // If skipping normal outputs, it's unnecessary to do a full AST parse of each input file.
      // The regex parser may still run if ordering/pruning inputs.
      compiler.setPreferRegexParser(true);
    }
    return Outcome.ok();
  }

  protected final A getCompiler() {
    return compiler;
  }

  /**
   * @return a mutable list
   */
  public static Outcome<List<SourceFile>> getBuiltinExterns(CompilerOptions.Environment env) {
    Outcome<byte[]> input = getExternsInput();
    if (input.isErr()) {
      return Outcome.err(input.error());
    }
    Outcome<List<Io.ZipEntryData>> zip = Io.readZipEntries(input.get(), Io.UTF_8);
    if (zip.isErr()) {
      return Outcome.err(zip.error());
    }
    String envPrefix = Ascii.toLowerCase(env.toString()) + "/";
    Map<String, SourceFile> mapFromExternsZip = new LinkedHashMap<>();
    for (Io.ZipEntryData entry : zip.get()) {
      String filename = entry.name();

      // Always load externs in the root folder.
      // If the non-core-JS externs are organized in subfolders, only load
      // the ones in a subfolder matching the specified environment. Strip the subfolder.
      if (filename.contains("/")) {
        if (!filename.startsWith(envPrefix)) {
          continue;
        }
        filename = filename.substring(envPrefix.length()); // remove envPrefix, including '/'
      }

      mapFromExternsZip.put(
          filename,
          // Give the files an odd prefix, so that they do not conflict
          // with the user's files.
          SourceFile.builder()
              .withPath("externs.zip//" + filename)
              .withContent(Io.utf8String(entry.data()))
              .build());
    }
    return Outcome.ok(DefaultExterns.prepareExterns(env, mapFromExternsZip));
  }

  /**
   * Some text identifying this binary and its version.
   *
   * <p>At minimum, this is what will be printed when `--version` is passed.
   */
  protected abstract String getVersionText();

  private static Outcome<byte[]> getExternsInput() {
    Outcome<byte[]> input = Io.readResource(AbstractCommandLineRunner.class, "/externs.zip");
    if (input.isOk() && input.get() == null) {
      // In some environments, the externs.zip is relative to this class.
      input = Io.readResource(AbstractCommandLineRunner.class, "externs.zip");
    }
    if (input.isOk()) {
      checkNotNull(input.get());
    }
    return input;
  }

  /** Runs the Compiler and calls System.exit() with the exit status of the compiler. */
  public final void run() {
    int result;
    try {
      Outcome<Integer> resultOutcome = doRun();
      if (resultOutcome.isErr()) {
        if (resultOutcome.error() instanceof FlagUsageException e) {
          err.println(e.getMessage());
          result = -1;
        } else {
          // An I/O failure: reported like any unexpected failure of the run.
          err.printStackTrace(resultOutcome.error());
          result = -2;
        }
      } else {
        result = resultOutcome.get();
      }
    } catch (Throwable t) {
      err.printStackTrace(t);
      result = -2;
    }

    exitCodeReceiver.apply(result);
  }

  /** Returns the PrintStream for writing errors associated with this AbstractCommandLineRunner. */
  protected final Io.Output getErrorPrintStream() {
    return err;
  }

  public Outcome<List<JsonFileSpec>> parseJsonFilesFromInputStream() throws IOException {
    List<JsonFileSpec> jsonFiles = new ArrayList<>();
    Outcome<String> json = this.in.readText(inputCharset);
    if (json.isErr()) {
      return Outcome.err(json.error());
    }
    Outcome<SimpleJsonValue> parsed = Json.parse(json.get());
    if (parsed.isErr()) {
      return Outcome.err(new SimpleJsonException("Malformed JSON in the JSON stream input"));
    }
    if (parsed.get().kind() != SimpleJsonValue.Kind.ARRAY) {
      return Outcome.err(new SimpleJsonException("Malformed JSON stream: expected an array"));
    }
    for (SimpleJsonValue element : parsed.get().elements()) {
      Outcome<JsonFileSpec> jsonFile = JsonFileSpec.readJson(element);
      if (jsonFile.isErr()) {
        return Outcome.err(jsonFile.error());
      }
      jsonFiles.add(jsonFile.get());
    }
    return Outcome.ok(jsonFiles);
  }

  /**
   * Creates inputs from a list of files.
   *
   * @param files A list of flag entries indicates js and zip file names.
   * @param allowStdIn Whether '-' is allowed appear as a filename to represent stdin. If true, '-'
   *     is only allowed to appear once.
   * @param jsChunkSpecs A list chunk specs.
   * @return An array of inputs
   */
  private Outcome<List<SourceFile>> createInputs(
      List<FlagEntry<JsSourceType>> files, boolean allowStdIn, List<JsChunkSpec> jsChunkSpecs)
      throws IOException {
    return createInputs(files, /* jsonFiles= */ null, allowStdIn, jsChunkSpecs);
  }

  /**
   * Creates inputs from a list of source files and json files.
   *
   * @param files A list of flag entries indicates js and zip file names.
   * @param jsonFiles A list of json encoded files.
   * @param jsChunkSpecs A list chunk specs.
   * @return An array of inputs
   */
  private Outcome<List<SourceFile>> createInputsList(
      List<FlagEntry<JsSourceType>> files,
      List<JsonFileSpec> jsonFiles,
      List<JsChunkSpec> jsChunkSpecs)
      throws IOException {
    return createInputs(files, jsonFiles, /* allowStdIn= */ false, jsChunkSpecs);
  }

  /**
   * Creates inputs from a list of source files, zips and json files.
   *
   * <p>Can be overridden by subclasses who want to pull files from different places.
   *
   * @param files A list of flag entries indicates js and zip file names
   * @param jsonFiles A list of json encoded files.
   * @param allowStdIn Whether '-' is allowed appear as a filename to represent stdin. If true, '-'
   *     is only allowed to appear once.
   * @param jsChunkSpecs A list chunk specs.
   * @return An array of inputs
   */
  protected Outcome<List<SourceFile>> createInputs(
      List<FlagEntry<JsSourceType>> files,
      @Nullable List<JsonFileSpec> jsonFiles,
      boolean allowStdIn,
      List<JsChunkSpec> jsChunkSpecs)
      throws IOException {
    List<SourceFile> inputs = new ArrayList<>(files.size());
    boolean usingStdin = false;
    int jsChunkIndex = 0;
    JsChunkSpec jsChunkSpec = Iterables.getFirst(jsChunkSpecs, null);
    int cumulatedInputFilesExpected =
        jsChunkSpec == null ? Integer.MAX_VALUE : jsChunkSpec.getNumInputs();
    for (int i = 0; i < files.size(); i++) {
      FlagEntry<JsSourceType> file = files.get(i);
      String filename = file.value;
      if (file.flag == JsSourceType.JS_ZIP) {
        if (this.config.typedAstListInputFilename.isPresent()) {
          return Outcome.err(new FlagUsageException("Can't use TypedASTs with --zip."));
        }

        if (!"-".equals(filename)) {
          Outcome<List<SourceFile>> newFilesRes = SourceFile.fromZipFile(filename, inputCharset);
          if (newFilesRes.isErr()) {
            return Outcome.err(newFilesRes.error());
          }
          List<SourceFile> newFiles = newFilesRes.get();
          // Update the manifest maps for new zip entries.
          if (rootRelativePathsMap.containsKey(filename)) {
            String rootFilename = rootRelativePathsMap.get(filename);
            for (SourceFile zipEntry : newFiles) {
              String zipEntryName = zipEntry.getName();
              checkState(zipEntryName.contains(filename));
              String zipmap = zipEntryName.replace(filename, rootFilename);
              rootRelativePathsMap.put(zipEntryName, zipmap);
            }
          }

          inputs.addAll(newFiles);
          if (jsChunkSpec != null) {
            jsChunkSpec.numJsFiles += newFiles.size() - 1;
          }
        }
      } else if (!"-".equals(filename)) {
        SourceKind kind = file.flag == JsSourceType.WEAKDEP ? SourceKind.WEAK : SourceKind.STRONG;
        SourceFile newFile =
            SourceFile.builder()
                .withPath(filename)
                .withCharset(inputCharset)
                .withKind(kind)
                .build();
        inputs.add(newFile);
      } else {
        if (this.config.typedAstListInputFilename.isPresent()) {
          return Outcome.err(new FlagUsageException("Can't use TypedASTs with stdin."));
        }
        if (!config.defaultToStdin) {
          return Outcome.err(new FlagUsageException("Can't specify stdin."));
        }
        if (usingStdin) {
          return Outcome.err(new FlagUsageException("Can't specify stdin twice."));
        }

        if (!config.outputManifests.isEmpty()) {
          return Outcome.err(new FlagUsageException(
              "Manifest files cannot be generated when the input is from stdin."));
        }
        if (!config.outputBundles.isEmpty()) {
          return Outcome.err(new FlagUsageException(
              "Bundle files cannot be generated when the input is from stdin."));
        }

        this.err.println(WAITING_FOR_INPUT_WARNING);
        Outcome<String> stdinCode = this.in.readText(inputCharset);
        if (stdinCode.isErr()) {
          throw new RuntimeException(stdinCode.error());
        }
        inputs.add(
            SourceFile.builder()
                .withPath("stdin")
                .withContent(stdinCode.get())
                .withCharset(inputCharset)
                .build());
        usingStdin = true;
      }
      if (i >= cumulatedInputFilesExpected - 1) {
        jsChunkIndex++;
        if (jsChunkIndex < jsChunkSpecs.size()) {
          jsChunkSpec = jsChunkSpecs.get(jsChunkIndex);
          cumulatedInputFilesExpected += jsChunkSpec.getNumInputs();
        }
      }
    }
    if (jsonFiles != null) {
      for (JsonFileSpec jsonFile : jsonFiles) {
        inputs.add(SourceFile.fromCode(jsonFile.getPath().get(), jsonFile.getSrc().orElseThrow()));
      }
    }

    return Outcome.ok(inputs);
  }

  /**
   * Removes input --ijs files whose basename matches an input --js or --weakdep file.
   *
   * <p>Returns an error outcome with a FlagUsageException if there are both input --ijs files and
   * chunk specs.
   */
  private Outcome<ImmutableList<JSError>> deduplicateIjsFiles(
      List<FlagEntry<JsSourceType>> files, List<String> moduleRoots, boolean hasChunkSpecs) {
    ImmutableList.Builder<JSError> errors = ImmutableList.builder();

    // First pass: collect the (module root relative) names of --js and --weakdep files.
    Map<String, String> relativeToAbsoluteName = new LinkedHashMap<>();
    for (FlagEntry<JsSourceType> file : files) {
      // TODO(tjgq): Handle zip files.
      if (file.flag == JsSourceType.JS || file.flag == JsSourceType.WEAKDEP) {
        String absoluteName = file.value;
        String relativeName = getModuleRootRelativeName(absoluteName, moduleRoots);
        relativeToAbsoluteName.put(relativeName, absoluteName);
      }
    }

    // Second pass: drop --ijs files whose (module root relative) name matches a --js or --weakdep
    // file.
    Iterator<FlagEntry<JsSourceType>> iterator = files.iterator();
    while (iterator.hasNext()) {
      FlagEntry<JsSourceType> file = iterator.next();
      if (file.flag == JsSourceType.IJS) {
        if (hasChunkSpecs) {
          return Outcome.err(
              new FlagUsageException("--ijs is incompatible with --chunk or --module."));
        }
        String absoluteName = file.value;
        if (!absoluteName.endsWith(".i.js")) {
          errors.add(JSError.make(IjsErrors.BAD_IJS_FILE_NAME, absoluteName));
          continue;
        }
        String relativeName = getModuleRootRelativeName(absoluteName, moduleRoots);
        String relativeNonIjsName =
            relativeName.substring(0, relativeName.length() - ".i.js".length());
        if (relativeToAbsoluteName.containsKey(relativeNonIjsName)) {
          errors.add(
              JSError.make(
                  IjsErrors.CONFLICTING_IJS_FILE,
                  relativeToAbsoluteName.get(relativeNonIjsName),
                  absoluteName));
          iterator.remove();
        }
      }
    }

    return Outcome.ok(errors.build());
  }

  private String getModuleRootRelativeName(String filename, List<String> moduleRoots) {
    for (String moduleRoot : moduleRoots) {
      if (filename.startsWith(moduleRoot + "/")) {
        return filename.substring(moduleRoot.length() + 1);
      }
    }
    return filename;
  }

  /** Creates JS source code inputs from a list of files. */
  private Outcome<@Nullable List<SourceFile>> createSourceInputs(
      List<JsChunkSpec> jsChunkSpecs,
      List<FlagEntry<JsSourceType>> files,
      List<JsonFileSpec> jsonFiles,
      List<String> moduleRoots)
      throws IOException {
    if (isInTestMode()) {
      return Outcome.ok(inputsSupplierForTesting != null ? inputsSupplierForTesting.get() : null);
    }
    if (files.isEmpty() && jsonFiles == null && config.defaultToStdin) {
      // Request to read from stdin.
      files = ImmutableList.of(new FlagEntry<JsSourceType>(JsSourceType.JS, "-"));
    }

    Outcome<ImmutableList<JSError>> ijsErrors =
        deduplicateIjsFiles(files, moduleRoots, !jsChunkSpecs.isEmpty());
    if (ijsErrors.isErr()) {
      return Outcome.err(ijsErrors.error());
    }
    for (JSError error : ijsErrors.get()) {
      compiler.report(error);
    }

    Outcome<List<SourceFile>> inputsRes;
    if (jsonFiles != null) {
      inputsRes = createInputsList(files, jsonFiles, jsChunkSpecs);
    } else {
      inputsRes = createInputs(files, true, jsChunkSpecs);
    }
    if (inputsRes.isErr() && inputsRes.error() instanceof IOException) {
      return Outcome.err(inputsRes.error());
    }
    if (inputsRes.isErr()) {
      return Outcome.err(new FlagUsageException("Bad --js flag. " + inputsRes.errorMessage().orElse("null")));
    }
    return Outcome.ok(inputsRes.get());
  }

  /** Creates JS extern inputs from a list of files. */
  private Outcome<List<SourceFile>> createExternInputs(List<String> files) throws IOException {
    List<FlagEntry<JsSourceType>> externFiles = new ArrayList<>();
    for (String file : files) {
      externFiles.add(new FlagEntry<JsSourceType>(JsSourceType.EXTERN, file));
    }
    Outcome<List<SourceFile>> inputsRes =
        createInputs(externFiles, false, new ArrayList<JsChunkSpec>());
    if (inputsRes.isErr() && inputsRes.error() instanceof IOException) {
      return Outcome.err(inputsRes.error());
    }
    if (inputsRes.isErr()) {
      return Outcome.err(new FlagUsageException("Bad --externs flag. " + inputsRes.errorMessage().orElse("null")));
    }
    return Outcome.ok(inputsRes.get());
  }

  /**
   * Creates chunk objects from a list of chunk specifications.
   *
   * @param specs A list of chunk specifications, not null or empty.
   * @param inputs A list of JS file paths, not null
   * @return An array of chunk objects
   */
  public static Outcome<List<JSChunk>> createJsChunks(
      List<JsChunkSpec> specs, List<CompilerInput> inputs) {
    checkState(specs != null);
    checkState(!specs.isEmpty());
    checkState(inputs != null);

    List<String> chunkNames = new ArrayList<>(specs.size());
    Map<String, JSChunk> chunksByName = new LinkedHashMap<>();
    Map<String, Integer> chunksFileCountMap = new LinkedHashMap<>();
    int numJsFilesExpected = 0;
    int minJsFilesRequired = 0;
    for (JsChunkSpec spec : specs) {
      if (chunksByName.containsKey(spec.name)) {
        return Outcome.err(new FlagUsageException("Duplicate chunk name: " + spec.name));
      }
      JSChunk chunk = new JSChunk(spec.name);

      for (String dep : spec.deps) {
        JSChunk other = chunksByName.get(dep);
        if (other == null) {
          return Outcome.err(new FlagUsageException(
              "Chunk '"
                  + spec.name
                  + "' depends on unknown chunk '"
                  + dep
                  + "'. Be sure to list chunks in dependency order."));
        }
        chunk.addDependency(other);
      }

      // We will allow chunks of zero input.
      if (spec.numJsFiles < 0) {
        numJsFilesExpected = -1;
      } else {
        minJsFilesRequired += spec.numJsFiles;
      }

      if (numJsFilesExpected >= 0) {
        numJsFilesExpected += spec.numJsFiles;
      }

      // Add chunks in reverse order so that source files are allocated to
      // chunks in reverse order. This allows the first chunk
      // (presumably the base chunk) to have a size of 'auto'
      chunkNames.add(0, spec.name);
      chunksFileCountMap.put(spec.name, spec.numJsFiles);
      chunksByName.put(spec.name, chunk);
    }

    final int totalNumJsFiles = inputs.size();

    if (numJsFilesExpected >= 0 || minJsFilesRequired > totalNumJsFiles) {
      if (minJsFilesRequired > totalNumJsFiles) {
        numJsFilesExpected = minJsFilesRequired;
      }

      if (numJsFilesExpected > totalNumJsFiles) {
        return Outcome.err(new FlagUsageException(
            "Not enough JS files specified. Expected "
                + numJsFilesExpected
                + " but found "
                + totalNumJsFiles));
      } else if (numJsFilesExpected < totalNumJsFiles) {
        return Outcome.err(new FlagUsageException(
            "Too many JS files specified. Expected "
                + numJsFilesExpected
                + " but found "
                + totalNumJsFiles));
      }
    }

    int numJsFilesLeft = totalNumJsFiles;
    int chunkIndex = 0;
    for (String chunkName : chunkNames) {
      // Parse chunk inputs.
      int numJsFiles = chunksFileCountMap.get(chunkName);
      JSChunk chunk = chunksByName.get(chunkName);

      // Check if the first chunk specified 'auto' for the number of files
      if (chunkIndex == chunkNames.size() - 1 && numJsFiles == -1) {
        numJsFiles = numJsFilesLeft;
      }

      List<CompilerInput> chunkFiles = inputs.subList(numJsFilesLeft - numJsFiles, numJsFilesLeft);
      for (CompilerInput input : chunkFiles) {
        chunk.addCompilerInput(input);
      }
      numJsFilesLeft -= numJsFiles;
      chunkIndex++;
    }

    return Outcome.ok(new ArrayList<>(chunksByName.values()));
  }

  /**
   * Validates the chunk name. Can be overridden by subclasses.
   *
   * @param name The chunk name
   */
  protected Outcome<Void> checkChunkName(String name) {
    if (!TokenStream.isJSIdentifier(name)) {
      return Outcome.err(new FlagUsageException("Invalid chunk name: '" + name + "'"));
    }
    return Outcome.ok();
  }

  /**
   * Parses chunk wrapper specifications.
   *
   * @param specs A list of chunk wrapper specifications, not null. The spec format is: <code>
   *     name:wrapper</code>. Wrappers.
   * @param chunks The JS chunks whose wrappers are specified
   * @return A map from chunk name to chunk wrapper. Chunks with no wrapper will have the empty
   *     string as their value in this map.
   */
  public static Outcome<Map<String, String>> parseChunkWrappers(
      List<String> specs, Iterable<JSChunk> chunks) {
    checkState(specs != null);

    Map<String, String> wrappers = new LinkedHashMap<>();

    // Prepopulate the map with chunk names.
    for (JSChunk c : chunks) {
      wrappers.put(c.getName(), "");
    }

    for (String spec : specs) {
      // Format is "<name>:<wrapper>".
      int pos = spec.indexOf(':');
      if (pos == -1) {
        return Outcome.err(new FlagUsageException(
            "Expected chunk wrapper to have " + "<name>:<wrapper> format: " + spec));
      }

      // Parse chunk name.
      String name = spec.substring(0, pos);
      if (!wrappers.containsKey(name)) {
        return Outcome.err(new FlagUsageException("Unknown chunk: '" + name + "'"));
      }
      String wrapper = spec.substring(pos + 1);
      // Support for %n% and %output%
      wrapper = wrapper.replace("%output%", "%s").replace("%n%", "\n");
      if (!wrapper.contains("%s")) {
        return Outcome.err(
            new FlagUsageException("No %s placeholder in chunk wrapper: '" + wrapper + "'"));
      }

      wrappers.put(name, wrapper);
    }
    return Outcome.ok(wrappers);
  }

  /**
   * Parses chunk output name specifications.
   *
   * @param specs A list of chunk output name specifications, not null. The spec format is: {@code
   *     name:output_file}.
   * @return A map from chunk name to chunk output file name, if declared. Chunks with no
   *     predeclared output file name will have no entry in this map.
   */
  private static Outcome<ImmutableMap<String, String>> parseChunkOutputFiles(List<String> specs) {
    checkArgument(specs != null);

    ImmutableMap.Builder<String, String> outputFilesBuilder = ImmutableMap.builder();

    for (String spec : specs) {
      // Format is "<name>:<output_file>".
      int pos = spec.indexOf(':');
      if (pos == -1) {
        return Outcome.err(new FlagUsageException(
            "Expected chunk_output_file to have " + "<name>:<output_file> format: " + spec));
      }

      String name = spec.substring(0, pos);
      String filename = spec.substring(pos + 1);
      outputFilesBuilder.put(name, filename);
    }
    return Outcome.ok(outputFilesBuilder.buildOrThrow());
  }

  /**
   * Returns the output file name for a chunk.
   *
   * <p>For chunks with predeclared output file names specified using {@code --chunk_output_file},
   * the output file name is {@code <outputPathPrefix>/<output_file>}
   *
   * <p>Otherwise, the output file name is {@code <outputPathPrefix>/<chunkName>.js}
   */
  @VisibleForTesting
  Outcome<String> getChunkOutputFileName(JSChunk m) {
    if (parsedChunkOutputFiles == null) {
      Outcome<ImmutableMap<String, String>> parsedChunkOutputFilesOutcome =
          parseChunkOutputFiles(config.chunkOutputFiles);
      if (parsedChunkOutputFilesOutcome.isErr()) {
        return Outcome.err(parsedChunkOutputFilesOutcome.error());
      }
      parsedChunkOutputFiles = parsedChunkOutputFilesOutcome.get();
    }
    Optional<String> outputFile = Optional.ofNullable(parsedChunkOutputFiles.get(m.getName()));
    if (outputFile.isPresent()) {
      return Outcome.ok(config.chunkOutputPathPrefix + outputFile.get());
    }
    return Outcome.ok(config.chunkOutputPathPrefix + m.getName() + ".js");
  }

  /** Returns the conformance file name for a chunk. */
  @VisibleForTesting
  Outcome<String> getChunkConformanceFileName(JSChunk m) {
    if (parsedChunkConformanceFiles == null) {
      Outcome<ImmutableMap<String, String>> parsedChunkConformanceFilesOutcome =
          parseChunkOutputFiles(config.chunkConformanceFiles);
      if (parsedChunkConformanceFilesOutcome.isErr()) {
        return Outcome.err(parsedChunkConformanceFilesOutcome.error());
      }
      parsedChunkConformanceFiles = parsedChunkConformanceFilesOutcome.get();
    }
    return Outcome.ok(parsedChunkConformanceFiles.get(m.getName()));
  }

  @VisibleForTesting
  Outcome<Void> writeChunkOutput(String fileName, Appendable out, LicenseTracker lt, JSChunk m)
      throws IOException {
    if (parsedChunkWrappers == null) {
      Outcome<Map<String, String>> parsedChunkWrappersOutcome = parseChunkWrappers(
              config.chunkWrapper, ImmutableList.copyOf(compiler.getChunkGraph().getAllChunks()));
      if (parsedChunkWrappersOutcome.isErr()) {
        return Outcome.err(parsedChunkWrappersOutcome.error());
      }
      parsedChunkWrappers = parsedChunkWrappersOutcome.get();
    }

    if (!isOutputInJson()) {
      maybeCreateDirsForPath(fileName);
    }
    String baseName = Io.pathFileName(fileName);
    writeOutput(
        out,
        compiler,
        lt,
        m,
        parsedChunkWrappers.get(m.getName()).replace("%basename%", baseName),
        "%s",
        null,
        fileName);
    return Outcome.ok();
  }

  /**
   * Writes code to an output stream, optionally wrapping it in an arbitrary wrapper that contains a
   * placeholder where the code should be inserted.
   *
   * @param chunk Which chunk to write. If this is null, write the entire AST.
   */
  void writeOutput(
      Appendable out,
      Compiler compiler,
      LicenseTracker licenseTracker,
      @Nullable JSChunk chunk,
      String wrapper,
      String codePlaceholder,
      @Nullable Function<String, String> escaper,
      String filename)
      throws IOException {
    if (compiler.getOptions().getOutputJs() == OutputJs.SENTINEL) {
      out.append("// No JS output because the compiler was run in checks-only mode.\n");
      return;
    }
    checkState(compiler.getOptions().getOutputJs() == OutputJs.NORMAL);

    String code = chunk == null ? compiler.toSource() : compiler.toSource(licenseTracker, chunk);
    writeOutput(out, compiler, code, wrapper, codePlaceholder, escaper, filename);
  }

  /**
   * Writes code to an output stream, optionally wrapping it in an arbitrary wrapper that contains a
   * placeholder where the code should be inserted.
   */
  @VisibleForTesting
  void writeOutput(
      Appendable out,
      Compiler compiler,
      String code,
      String wrapper,
      String codePlaceholder,
      @Nullable Function<String, String> escaper,
      String filename)
      throws IOException {
    int pos = wrapper.indexOf(codePlaceholder);
    if (pos != -1) {
      String prefix = "";

      if (pos > 0) {
        prefix = wrapper.substring(0, pos);
        out.append(prefix);
      }

      out.append(escaper == null ? code : escaper.apply(code));

      int suffixStart = pos + codePlaceholder.length();
      if (suffixStart != wrapper.length()) {
        // Something after placeholder?
        out.append(wrapper.substring(suffixStart));
      }
      if (getCommandLineConfig().includeTrailingNewline) {
        out.append('\n');
      }

      // If we have a source map, adjust its offsets to match
      // the code WITHIN the wrapper.
      if (compiler != null && compiler.getSourceMap() != null) {
        compiler.getSourceMap().setWrapperPrefix(prefix);
      }
    } else {
      out.append(code);
      if (getCommandLineConfig().includeTrailingNewline) {
        out.append('\n');
      }
    }
  }

  /** Creates any directories necessary to write a file that will have a given path prefix. */
  private static void maybeCreateDirsForPath(String pathPrefix) {
    if (!Strings.isNullOrEmpty(pathPrefix)) {
      Optional<String> parent = Io.pathParent(pathPrefix);
      if (parent.isPresent()) {
        Io.mkdirs(parent.get());
      }
    }
  }

  private Outcome<Appendable> createDefaultOutput() {
    boolean writeOutputToFile = !config.jsOutputFile.isEmpty();
    if (writeOutputToFile) {
      Outcome<Io.Output> output = fileNameToLegacyOutputWriter(Optional.of(config.jsOutputFile));
      if (output.isErr()) {
        return Outcome.err(output.error());
      }
      return Outcome.ok(output.get());
    } else {
      return Outcome.ok(streamToLegacyOutputWriter(defaultJsOutput));
    }
  }

  private static Outcome<Void> closeAppendable(Appendable output) {
    if (output instanceof Io.Output out) {
      out.flush();
      return closeOutput(out);
    }
    return Outcome.ok();
  }

  // I/O failures in this class are error outcomes carrying the IOException, like the flag usage
  // errors; run() tells the two apart and ends the run on an I/O failure with a stack trace and
  // exit status -2, as it always did.

  /** Closes the output, if there is one; a failed write or close of it is an I/O failure. */
  private static Outcome<Void> closeOutput(Io.@Nullable Output output) {
    if (output != null) {
      return output.close();
    }
    return Outcome.ok();
  }

  /**
   * Parses command-line arguments and runs the compiler.
   *
   * @return system exit status
   */
  protected Outcome<Integer> doRun() throws IOException {
    CompileMetricsRecorderInterface metricsRecorder = getCompileMetricsRecorder();
    metricsRecorder.recordActionStart();
    if (this.config.printVersion) {
      this.defaultJsOutput.println(this.getVersionText());
      this.defaultJsOutput.flush();
      return Outcome.ok(0);
    }

    Compiler.setLoggingLevel(Level.parse(config.loggingLevel));

    compiler = createCompiler();
    if (!config.expectedDiagnostics.isEmpty()) {
      compiler.setErrorManager(
          new VerifyingErrorManager(compiler.getErrorManager(), config.expectedDiagnostics));
    }
    Outcome<B> optionsOutcome = createOptions();
    if (optionsOutcome.isErr()) {
      return Outcome.err(optionsOutcome.error());
    }
    B options = optionsOutcome.get();
    Outcome<Void> setRunOptionsOutcome = setRunOptions(options);
    if (setRunOptionsOutcome.isErr()) {
      return Outcome.err(setRunOptionsOutcome.error());
    }

    Optional<String> typedAstListInputFilename = config.typedAstListInputFilename;

    Outcome<List<SourceFile>> externsOutcome = createExterns(options);
    if (externsOutcome.isErr()) {
      return Outcome.err(externsOutcome.error());
    }
    List<SourceFile> externs = externsOutcome.get();
    List<JSChunk> chunks = null;

    rootRelativePathsMap = constructRootRelativePathsMap();

    boolean writeOutputToFile = !config.jsOutputFile.isEmpty();
    List<String> outputFileNames = new ArrayList<>();
    if (writeOutputToFile) {
      outputFileNames.add(config.jsOutputFile);
    }

    boolean createCommonJsModules = false;
    if (options.getProcessCommonJSModules()
        && (config.chunk.size() == 1 && "auto".equals(config.chunk.get(0)))) {
      createCommonJsModules = true;
      config.chunk.remove(0);
    }

    List<JsChunkSpec> jsChunkSpecs = new ArrayList<>();
    for (int i = 0; i < config.chunk.size(); i++) {
      Outcome<JsChunkSpec> jsChunkSpec = JsChunkSpec.create(config.chunk.get(i), i == 0);
      if (jsChunkSpec.isErr()) {
        return Outcome.err(jsChunkSpec.error());
      }
      jsChunkSpecs.add(jsChunkSpec.get());
    }
    List<JsonFileSpec> jsonFiles = null;

    if (config.jsonStreamMode == JsonStreamMode.IN
        || config.jsonStreamMode == JsonStreamMode.BOTH) {
      Outcome<List<JsonFileSpec>> jsonFilesOutcome = parseJsonFilesFromInputStream();
      if (jsonFilesOutcome.isErr()) {
        return Outcome.err(jsonFilesOutcome.error());
      }
      jsonFiles = jsonFilesOutcome.get();

      ImmutableMap.Builder<String, SourceMapInput> inputSourceMaps = new ImmutableMap.Builder<>();
      ImmutableMap.Builder<String, String> inputPathByWebpackId = new ImmutableMap.Builder<>();

      boolean foundJsonInputSourceMap = false;
      for (JsonFileSpec jsonFile : jsonFiles) {
        if (jsonFile.getSourceMap().isPresent() && jsonFile.getSourceMap().get().length() > 0) {
          String sourceMapPath = jsonFile.getPath().orElse("null") + ".map";
          SourceFile sourceMap = SourceFile.fromCode(sourceMapPath, jsonFile.getSourceMap().get());
          inputSourceMaps.put(jsonFile.getPath().get(), new SourceMapInput(sourceMap));
          foundJsonInputSourceMap = true;
        }
        if (jsonFile.getWebpackId().isPresent()) {
          inputPathByWebpackId.put(jsonFile.getWebpackId().get(), jsonFile.getPath().get());
        }
      }

      if (foundJsonInputSourceMap) {
        inputSourceMaps.putAll(options.getInputSourceMaps());
        options.setInputSourceMaps(inputSourceMaps.buildOrThrow());
      }

      compiler.initWebpackMap(inputPathByWebpackId.buildOrThrow());
    } else {
      ImmutableMap<String, String> emptyMap = ImmutableMap.of();
      compiler.initWebpackMap(emptyMap);
    }

    options.setDoLateLocalization(config.shouldDoLateLocalization());
    options.setAlwaysGatherSourceMapInfo(config.shouldAlwaysGatherSourceMapInfo());

    compiler.initOptions(options);

    Outcome<List<SourceFile>> sourcesOutcome =
        createSourceInputs(jsChunkSpecs, config.mixedJsSources, jsonFiles, config.moduleRoots);
    if (sourcesOutcome.isErr()) {
      return Outcome.err(sourcesOutcome.error());
    }
    List<SourceFile> sources =
        sourcesOutcome.get();

    if (!jsChunkSpecs.isEmpty()) {
      if (isInTestMode()) {
        chunks = chunksSupplierForTesting.get();
      } else {
        if (JsonStreamMode.IN.equals(config.jsonStreamMode)
            || JsonStreamMode.NONE.equals(config.jsonStreamMode)) {
          for (JsChunkSpec m : jsChunkSpecs) {
            Outcome<Void> checkChunkNameOutcome = checkChunkName(m.getName());
            if (checkChunkNameOutcome.isErr()) {
              return Outcome.err(checkChunkNameOutcome.error());
            }
          }
        }
        ImmutableList.Builder<CompilerInput> inputs = ImmutableList.builder();
        for (SourceFile source : sources) {
          inputs.add(new CompilerInput(source, /* isExtern= */ false));
        }
        Outcome<List<JSChunk>> chunksOutcome = createJsChunks(jsChunkSpecs, inputs.build());
        if (chunksOutcome.isErr()) {
          return Outcome.err(chunksOutcome.error());
        }
        chunks = chunksOutcome.get();
      }
      for (JSChunk m : chunks) {
        Outcome<String> chunkOutputFileName = getChunkOutputFileName(m);
        if (chunkOutputFileName.isErr()) {
          return Outcome.err(chunkOutputFileName.error());
        }
        outputFileNames.add(chunkOutputFileName.get());
      }

      if (typedAstListInputFilename.isPresent()) {
        this.initChunksWithTypedAstFilesystem(externs, chunks, options, typedAstListInputFilename.get());
      } else {
        compiler.initChunks(externs, chunks, options);
      }
    } else {
      if (typedAstListInputFilename.isPresent()) {
        this.initWithTypedAstFilesystem(externs, sources, options, typedAstListInputFilename.get());
      } else {
        compiler.init(externs, sources, options);
      }
    }

    // Release temporary data structures now that the compiler has
    // been initialized
    jsChunkSpecs = null;
    jsonFiles = null;
    externs = null;
    sources = null;

    final Result result;
    if (config.skipNormalOutputs) {
      metricsRecorder.recordActionName("skip normal outputs");
      // TODO(bradfordcsmith): Should we be ignoring possible init/initChunks() errors here?
      compiler.orderInputsWithLargeStack();
      result = null;
    } else if (compiler.hasErrors()) {
      metricsRecorder.recordActionName("initialization errors occurred");
      // init() or initChunks() encountered an error.
      compiler.generateReport();
      result = compiler.getResult();
    } else {
      try {
        // This is the common case - we're actually compiling something.
        Outcome<Void> compiled = performCompilation(metricsRecorder);
        if (compiled.isErr()) {
          return Outcome.err(compiled.error());
        }

        // If we're finished with compilation (i.e. we're not saving state), /and/ the compiler was
        // restored from a previous state, then we need to re-initialize the set of chunks.
        // TODO(lharker): figure out if this is still needed.
        boolean refresh =
            chunks != null
                && config.restoredCompilationStage != -1
                && config.saveAfterCompilationStage == -1;
        if (refresh) {
          chunks = ImmutableList.copyOf(compiler.getChunks());
        }
      } finally {
        // Make sure we generate a report of errors and warnings even if the compiler throws an
        // exception somewhere.
        compiler.generateReport();
      }
      result = compiler.getResult();
    }

    if (createCommonJsModules) {
      // For CommonJS modules, construct chunks from actual inputs.
      chunks = ImmutableList.copyOf(compiler.getChunks());
      for (JSChunk c : chunks) {
        Outcome<String> chunkOutputFileName = getChunkOutputFileName(c);
        if (chunkOutputFileName.isErr()) {
          return Outcome.err(chunkOutputFileName.error());
        }
        outputFileNames.add(chunkOutputFileName.get());
      }
    }

    for (String outputFileName : outputFileNames) {
      if (compiler.getSourceFileByName(Optional.of(outputFileName)) != null) {
        compiler.report(JSError.make(OUTPUT_SAME_AS_INPUT_ERROR, outputFileName));
        return Outcome.ok(1);
      }
    }

    // We won't want to process results for cases where compilation is only partially done.
    boolean shouldProcessResults = config.getSaveCompilationStateToFilename().isEmpty();
    int exitStatus;
    if (shouldProcessResults) {
      Outcome<Integer> exitStatusOutcome = processResults(result, chunks, options);
      if (exitStatusOutcome.isErr()) {
        return Outcome.err(exitStatusOutcome.error());
      }
      exitStatus = exitStatusOutcome.get();
    } else {
      exitStatus = getExitStatusForResult(result);
    }
    metricsRecorder.recordResultMetrics(compiler, result);
    return Outcome.ok(exitStatus);
  }

  private Outcome<Void> performCompilation(CompileMetricsRecorderInterface metricsRecorder) {
    // Parse, restore from a save file, or initialize from a TypedAST list.
    Outcome<Void> initialized = initializeStateBeforeCompilation();
    if (initialized.isErr()) {
      return Outcome.err(initialized.error());
    }

    // Do the interesting work - checks, optimizations, etc.
    runCompilerPasses(metricsRecorder);

    // Save state unless there was an exception or error - we won't go on to the next stage if there
    // was an error, and saving state itself might throw an exception if the compiler is in a bad
    // state.
    if (!compiler.hasErrors()) {
      saveState();
    }
    // Perform post-compilation tasks even if compiler.hasErrors()
    compiler.performPostCompilationTasks();
    return Outcome.ok();
  }

  private void runCompilerPasses(CompileMetricsRecorderInterface metricsRecorder) {
    boolean runStage1 = false;
    boolean runStage2 = false;
    boolean runStage3 = false;
    boolean instrumentForCoverage = false;
    final String actionMetricsName;
    if (compiler.getOptions().getInstrumentForCoverageOnly()) {
      actionMetricsName = "instrument for coverage";
      instrumentForCoverage = true;
    } else if (config.shouldSaveAfterStage1()) {
      actionMetricsName = "stage 1";
      runStage1 = true;
    } else if (config.shouldRestoreTypedAstsPerformStage2AndSave()) {
      actionMetricsName = "parse & optimize";
      runStage2 = true;
    } else if (config.shouldRestoreTypedAstsPerformStages2And3()) {
      actionMetricsName = "skip-checks compile";
      runStage2 = true;
      runStage3 = true;
    } else if (config.shouldRestoreAndPerformStage2AndSave()) {
      actionMetricsName = "stage 2/3";
      runStage2 = true;
    } else if (config.shouldRestoreAndPerformStages2And3()) {
      // From the outside this looks like the second stage of a 2-stage compile.
      actionMetricsName = "stage 2/2";
      runStage2 = true;
      runStage3 = true;
    } else if (config.shouldRestoreAndPerformStage3()) {
      actionMetricsName = "stage 3/3";
      runStage3 = true;
    } else {
      // This is the code path taken when "building" a library by just checking it for errors
      // and generating an .ijs file and also when doing a full compilation.
      actionMetricsName = compiler.getOptions().isChecksOnly() ? "checks-only" : "full compile";
      runStage1 = true;
      runStage2 = true;
      runStage3 = true;
    }

    metricsRecorder.recordActionName(actionMetricsName);
    if (compiler.hasErrors()) {
      return;
    }
    metricsRecorder.recordStartState(compiler);
    if (runStage1) {
      compiler.stage1Passes();
      if (compiler.hasErrors()) {
        return;
      }
    }
    if (runStage2) {
      compiler.stage2Passes(SegmentOfCompilationToRun.OPTIMIZATIONS);
      if (compiler.hasErrors()) {
        return;
      }
    }
    if (runStage3) {
      compiler.stage3Passes();
      if (compiler.hasErrors()) {
        return;
      }
    }
    if (instrumentForCoverage) {
      compiler.instrumentForCoverage();
    }
  }

  /**
   * Child classes should override this if they want to actually record metrics about the
   * compilation.
   */
  protected CompileMetricsRecorderInterface getCompileMetricsRecorder() {
    return new DummyCompileMetricsRecorder();
  }

  private void initWithTypedAstFilesystem(
      List<SourceFile> externs,
      List<SourceFile> sources,
      CompilerOptions options,
      String filename) {
    Outcome<byte[]> typedAstList = readTypedAstList(filename);
    if (typedAstList.isErr()) {
      compiler.report(JSError.make(COULD_NOT_DESERIALIZE_AST, filename));
      return;
    }
    compiler.initWithTypedAstFilesystem(externs, sources, options, typedAstList.get());
  }

  private void initChunksWithTypedAstFilesystem(
      List<SourceFile> externs, List<JSChunk> chunks, CompilerOptions options, String filename) {
    Outcome<byte[]> typedAstList = readTypedAstList(filename);
    if (typedAstList.isErr()) {
      compiler.report(JSError.make(COULD_NOT_DESERIALIZE_AST, filename));
      return;
    }
    compiler.initChunksWithTypedAstFilesystem(externs, chunks, options, typedAstList.get());
  }

  /** Reads and unzips a gzipped TypedAst.List file. */
  private static Outcome<byte[]> readTypedAstList(String filename) {
    Outcome<byte[]> gzipped = Io.readBytes(filename);
    if (gzipped.isErr()) {
      return gzipped;
    }
    return Io.gunzip(gzipped.get());
  }

  /**
   * Call at the beginning of compilation to initialize the compiler state.
   *
   * <p>Compiler state should be initialized no matter what the compilation stage is, but this
   * method handles the different ways it might be initialized - whether from parsing actual JS
   * files, from reading library-level TypedASTs, or from restoring a previous compilation state.
   */
  private Outcome<Void> initializeStateBeforeCompilation() {
    if (config.restoredCompilationStage != -1) {
      Outcome<Void> restored = restoreState(config.getContinueSavedCompilationFileName().orElse(null));
      if (restored.isErr()) {
        return Outcome.err(restored.error());
      }
    } else if (config.typedAstListInputFilename.isPresent()) {
      // we did this elsewhere
    } else {
      // parsing!
      compiler.parseForCompilation();
    }
    return Outcome.ok();
  }

  private Outcome<Void> restoreState(String filename) {
    Outcome<byte[]> state = Io.readBytes(filename);
    if (state.isErr()) {
      compiler.report(JSError.make(COULD_NOT_DESERIALIZE_AST, filename));
      return Outcome.ok();
    }
    return compiler.restoreState(state.get());
  }

  /** Call at the end of compilation to save the compiler state if applicable. */
  private void saveState() {
    if (config.getSaveCompilationStateToFilename().isEmpty()) {
      // nothing to save to.
      return;
    }
    String filename = config.getSaveCompilationStateToFilename().get();
    // Create (or empty) the file first: if that fails there is no point in serializing.
    Outcome<Void> created = Io.writeBytes(filename, new byte[0]);
    if (created.isErr()) {
      compiler.report(JSError.make(COULD_NOT_SERIALIZE_AST, filename));
      return;
    }
    Outcome<Void> saved = Io.writeBytes(filename, compiler.saveState());
    if (saved.isErr()) {
      compiler.report(JSError.make(COULD_NOT_SERIALIZE_AST, filename));
    }
  }

  /** Processes the results of the compile job, and returns an error code. */
  Outcome<Integer> processResults(Result result, List<JSChunk> chunks, B options)
      throws IOException {
    if (config.printAst) {
      if (compiler.getRoot() == null) {
        return Outcome.ok(1);
      } else {
        Outcome<Appendable> jsOutputOutcome = createDefaultOutput();
        if (jsOutputOutcome.isErr()) {
          return Outcome.err(jsOutputOutcome.error());
        }
        Appendable jsOutput = jsOutputOutcome.get();
        ControlFlowGraph<Node> cfg = compiler.computeCFG();
        DotFormatter.appendDot(compiler.getRoot().getLastChild(), cfg, jsOutput);
        jsOutput.append('\n');
        Outcome<Void> jsOutputClosed = closeAppendable(jsOutput);
        if (jsOutputClosed.isErr()) {
          return Outcome.err(jsOutputClosed.error());
        }
        return Outcome.ok(0);
      }
    }

    if (config.printTree) {
      if (compiler.getRoot() == null) {
        compiler.report(JSError.make(NO_TREE_GENERATED_ERROR));
        return Outcome.ok(1);
      } else {
        Outcome<Appendable> jsOutputOutcome = createDefaultOutput();
        if (jsOutputOutcome.isErr()) {
          return Outcome.err(jsOutputOutcome.error());
        }
        Appendable jsOutput = jsOutputOutcome.get();
        compiler.getJsRoot().appendStringTree(jsOutput);
        jsOutput.append("\n");
        Outcome<Void> jsOutputClosed = closeAppendable(jsOutput);
        if (jsOutputClosed.isErr()) {
          return Outcome.err(jsOutputClosed.error());
        }
        return Outcome.ok(0);
      }
    }

    if (config.printTreeJson) {
      if (compiler.getRoot() == null) {
        compiler.report(JSError.make(NO_TREE_GENERATED_ERROR));
        return Outcome.ok(1);
      } else {
        Outcome<Appendable> jsOutputOutcome = createDefaultOutput();
        if (jsOutputOutcome.isErr()) {
          return Outcome.err(jsOutputOutcome.error());
        }
        Appendable jsOutput = jsOutputOutcome.get();
        compiler.getJsRoot().appendJsonTree(jsOutput);
        Outcome<Void> jsOutputClosed = closeAppendable(jsOutput);
        if (jsOutputClosed.isErr()) {
          return Outcome.err(jsOutputClosed.error());
        }
        return Outcome.ok(0);
      }
    }

    if (config.skipNormalOutputs) {
      // Output the manifest and bundle files if requested.
      Outcome<Void> outputManifestOutcome = outputManifest();
      if (outputManifestOutcome.isErr()) {
        return Outcome.err(outputManifestOutcome.error());
      }
      Outcome<Void> outputBundleOutcome = outputBundle();
      if (outputBundleOutcome.isErr()) {
        return Outcome.err(outputBundleOutcome.error());
      }
      Outcome<Void> outputChunkGraphJsonOutcome1 = outputChunkGraphJson();
      if (outputChunkGraphJsonOutcome1.isErr()) {
        return Outcome.err(outputChunkGraphJsonOutcome1.error());
      }
      return Outcome.ok(0);
    } else if (options.getOutputJs() != OutputJs.NONE && result.success) {
      Outcome<Void> outputChunkGraphJsonOutcome2 = outputChunkGraphJson();
      if (outputChunkGraphJsonOutcome2.isErr()) {
        return Outcome.err(outputChunkGraphJsonOutcome2.error());
      }
      if (chunks == null) {
        Outcome<Void> outputSingleBinaryOutcome3 = outputSingleBinary(options);
        if (outputSingleBinaryOutcome3.isErr()) {
          return Outcome.err(outputSingleBinaryOutcome3.error());
        }

        // Output the source map if requested.
        // If output files are being written to stdout as a JSON string,
        // outputSingleBinary will have added the sourcemap to the output file
        if (!isOutputInJson()) {
          Outcome<Void> outputSourceMapOutcome = outputSourceMap(options, config.jsOutputFile);
          if (outputSourceMapOutcome.isErr()) {
            return Outcome.err(outputSourceMapOutcome.error());
          }
        }
      } else {
        Outcome<DiagnosticType> errorOutcome =
            outputChunkBinaryAndSourceMaps(compiler.getChunkGraph(), options);
        if (errorOutcome.isErr()) {
          return Outcome.err(errorOutcome.error());
        }
        DiagnosticType error = errorOutcome.get();
        if (error != null) {
          compiler.report(JSError.make(error));
          return Outcome.ok(1);
        }
      }

      // Output the externs if required.
      if (options.getExternExportsPath().isPresent()) {
        Outcome<Io.Output> eeOutOutcome = openExternExportsStream(options, config.jsOutputFile);
        if (eeOutOutcome.isErr()) {
          return Outcome.err(eeOutOutcome.error());
        }
        Io.Output eeOut = eeOutOutcome.get();
        Outcome<Void> eeOutClosed;
        try {
          eeOut.append(result.externExport.orElse(null));
        } finally {
          eeOutClosed = closeOutput(eeOut);
        }
        if (eeOutClosed.isErr()) {
          return Outcome.err(eeOutClosed.error());
        }
      }

      // Output the variable and property name maps if requested.
      Outcome<Void> outputNameMapsOutcome = outputNameMaps();
      if (outputNameMapsOutcome.isErr()) {
        return Outcome.err(outputNameMapsOutcome.error());
      }

      // Output the ReplaceStrings map if requested
      Outcome<Void> outputStringMapOutcome4 = outputStringMap();
      if (outputStringMapOutcome4.isErr()) {
        return Outcome.err(outputStringMapOutcome4.error());
      }

      // Output the manifest and bundle files if requested.
      Outcome<Void> outputManifestOutcome2 = outputManifest();
      if (outputManifestOutcome2.isErr()) {
        return Outcome.err(outputManifestOutcome2.error());
      }
      Outcome<Void> outputBundleOutcome2 = outputBundle();
      if (outputBundleOutcome2.isErr()) {
        return Outcome.err(outputBundleOutcome2.error());
      }

      // Output the production instrumentation param mapping if requested.
      Outcome<Void> outputInstrumentationMappingOutcome = outputInstrumentationMapping();
      if (outputInstrumentationMappingOutcome.isErr()) {
        return Outcome.err(outputInstrumentationMappingOutcome.error());
      }

      if (isOutputInJson()) {
        Outcome<Void> jsonStreamOutcome = outputJsonStream();
        if (jsonStreamOutcome.isErr()) {
          return Outcome.err(jsonStreamOutcome.error());
        }
      }
    }

    return Outcome.ok(getExitStatusForResult(result));
  }

  private static int getExitStatusForResult(Result result) {
    // return 0 if no errors, the error count otherwise
    return min(result.errors.size(), 0x7f);
  }

  Function<String, String> getJavascriptEscaper() {
    return SourceCodeEscapers.javascriptEscaper().asFunction();
  }

  Outcome<Void> outputSingleBinary(B options) throws IOException {
    Function<String, String> escaper = null;
    String marker = OUTPUT_MARKER;
    if (config.outputWrapper.contains(OUTPUT_MARKER_JS_STRING)) {
      marker = OUTPUT_MARKER_JS_STRING;
      escaper = getJavascriptEscaper();
    }

    if (isOutputInJson()) {
      this.filesToStreamOut.add(createJsonFile(options, marker, escaper));
    } else {
      if (!config.jsOutputFile.isEmpty()) {
        maybeCreateDirsForPath(config.jsOutputFile);
      }

      Outcome<Appendable> jsOutputOutcome = createDefaultOutput();
      if (jsOutputOutcome.isErr()) {
        return Outcome.err(jsOutputOutcome.error());
      }
      Appendable jsOutput = jsOutputOutcome.get();
      writeOutput(
          jsOutput,
          compiler,
          // So long as the JSChunk arg is null the compiler will write all sources to jsOutput
          // Use single-binary license tracking to dedupe licenses among all the inputs
          new SingleBinaryLicenseTracker(compiler),
          (JSChunk) null,
          config.outputWrapper,
          marker,
          escaper,
          config.jsOutputFile);
      Outcome<Void> jsOutputClosed = closeAppendable(jsOutput);
      if (jsOutputClosed.isErr()) {
        return Outcome.err(jsOutputClosed.error());
      }
    }
    return Outcome.ok();
  }

  /** Save the compiler output to a JsonFileSpec to be later written to stdout */
  JsonFileSpec createJsonFile(B options, String outputMarker, Function<String, String> escaper)
      throws IOException {
    Appendable jsOutput = new StringBuilder();
    writeOutput(
        jsOutput,
        compiler,
        // So long as the JSChunk arg is null the compiler will write all sources to jsOutput
        // Use single-binary license tracking to dedupe licenses among all the inputs
        new SingleBinaryLicenseTracker(compiler),
        (JSChunk) null,
        config.outputWrapper,
        outputMarker,
        escaper,
        config.jsOutputFile);

    JsonFileSpec jsonOutput =
        new JsonFileSpec(
            jsOutput.toString(),
            Strings.isNullOrEmpty(config.jsOutputFile) ? "compiled.js" : config.jsOutputFile);

    if (options.shouldGatherSourceMapInfo()) {
      StringBuilder sourcemap = new StringBuilder();
      compiler.getSourceMap().appendTo(sourcemap, jsonOutput.getPath());
      jsonOutput.setSourceMap(sourcemap.toString());
    }

    return jsonOutput;
  }

  Outcome<Void> outputJsonStream() throws IOException {
    SimpleJsonWriter jsonWriter = new SimpleJsonWriter();
    jsonWriter.setHtmlSafe(false);
    jsonWriter.setSerializeNulls(false);
    jsonWriter.beginArray();
    for (JsonFileSpec jsonFile : this.filesToStreamOut) {
      jsonFile.writeJson(jsonWriter);
    }
    jsonWriter.endArray();
    // The JSON goes to the output stream as UTF-8, and the stream is closed.
    Io.Output out = defaultJsOutput.encodedAs(Io.UTF_8);
    out.print(jsonWriter.toString());
    Outcome<Void> outClosed1 = closeOutput(out);
    if (outClosed1.isErr()) {
      return Outcome.err(outClosed1.error());
    }
    return Outcome.ok();
  }

  private Outcome<@Nullable DiagnosticType> outputChunkBinaryAndSourceMaps(
      JSChunkGraph chunkGraph, B options) throws IOException {
    Iterable<JSChunk> chunks = chunkGraph.getAllChunks();
    Outcome<Map<String, String>> parsedChunkWrappersOutcome =
        parseChunkWrappers(config.chunkWrapper, chunks);
    if (parsedChunkWrappersOutcome.isErr()) {
      return Outcome.err(parsedChunkWrappersOutcome.error());
    }
    parsedChunkWrappers = parsedChunkWrappersOutcome.get();
    if (!isOutputInJson()) {
      // make sure the method generates all dirs up to the latest /
      maybeCreateDirsForPath(config.chunkOutputPathPrefix + "dummy");
    }

    // If the source map path is in fact a pattern for each
    // chunk, create a stream per-chunk. Otherwise, create
    // a single source map.
    Io.Output mapFileOut = null;

    // When the json_streams flag is specified, sourcemaps are always generated
    // per chunk
    if (!(shouldGenerateMapPerChunk(options)
        || !options.shouldGatherSourceMapInfo()
        || config.jsonStreamMode == JsonStreamMode.OUT
        || config.jsonStreamMode == JsonStreamMode.BOTH)) {
      // warn that this is not supported
      return Outcome.ok(INVALID_CHUNK_SOURCEMAP_PATTERN);
    }

    ChunkGraphAwareLicenseTracker mlicenseTracker = new ChunkGraphAwareLicenseTracker(compiler);
    for (JSChunk m : chunks) {
      if (m.getName().equals(JSChunk.WEAK_CHUNK_NAME)) {
        // Skip the weak chunk, which is always empty.
        continue;
      }
      if (isOutputInJson()) {
        Outcome<JsonFileSpec> jsonFile = createJsonFileFromChunk(m);
        if (jsonFile.isErr()) {
          return Outcome.err(jsonFile.error());
        }
        this.filesToStreamOut.add(jsonFile.get());
      } else {
        if (shouldGenerateMapPerChunk(options)) {
          Outcome<String> sourceMapPath = expandSourceMapPath(options, m);
          if (sourceMapPath.isErr()) {
            return Outcome.err(sourceMapPath.error());
          }
          Outcome<Io.Output> mapFileOutOutcome = fileNameToOutputWriter2(Optional.ofNullable(sourceMapPath.get()));
          if (mapFileOutOutcome.isErr()) {
            return Outcome.err(mapFileOutOutcome.error());
          }
          mapFileOut = mapFileOutOutcome.get();
        }

        Outcome<String> chunkFilenameOutcome = getChunkOutputFileName(m);
        if (chunkFilenameOutcome.isErr()) {
          return Outcome.err(chunkFilenameOutcome.error());
        }
        String chunkFilename = chunkFilenameOutcome.get();
        maybeCreateDirsForPath(chunkFilename);
        Outcome<Io.Output> writerOutcome = fileNameToLegacyOutputWriter(Optional.of(chunkFilename));
        if (writerOutcome.isErr()) {
          return Outcome.err(writerOutcome.error());
        }
        Io.Output writer = writerOutcome.get();
        Outcome<Void> writerClosed;
        try {
          if (options.shouldGatherSourceMapInfo()) {
            compiler.resetAndIntitializeSourceMap();
          }
          mlicenseTracker.setCurrentChunkContext(m);
          Outcome<Void> writeChunkOutputOutcome =
              writeChunkOutput(chunkFilename, writer, mlicenseTracker, m);
          if (writeChunkOutputOutcome.isErr()) {
            return Outcome.err(writeChunkOutputOutcome.error());
          }
          if (options.shouldGatherSourceMapInfo()) {
            compiler.getSourceMap().appendTo(mapFileOut, Optional.of(chunkFilename));
          }
        } finally {
          writerClosed = closeOutput(writer);
        }
        if (writerClosed.isErr()) {
          return Outcome.err(writerClosed.error());
        }

        if (shouldGenerateMapPerChunk(options) && mapFileOut != null) {
          Outcome<Void> mapFileOutClosed2 = closeOutput(mapFileOut);
          if (mapFileOutClosed2.isErr()) {
            return Outcome.err(mapFileOutClosed2.error());
          }
          mapFileOut = null;
        }
      }
    }

    if (mapFileOut != null) {
      Outcome<Void> mapFileOutClosed3 = closeOutput(mapFileOut);
      if (mapFileOutClosed3.isErr()) {
        return Outcome.err(mapFileOutClosed3.error());
      }
    }
    return Outcome.ok(null);
  }

  /** Given an output chunk, convert it to a JSONFileSpec with associated sourcemap */
  private Outcome<JsonFileSpec> createJsonFileFromChunk(JSChunk chunk) throws IOException {
    compiler.resetAndIntitializeSourceMap();

    Outcome<String> filenameOutcome = getChunkOutputFileName(chunk);
    if (filenameOutcome.isErr()) {
      return Outcome.err(filenameOutcome.error());
    }
    String filename = filenameOutcome.get();
    StringBuilder output = new StringBuilder();
    Outcome<Void> writeChunkOutputOutcome =
        writeChunkOutput(filename, output, new ScriptNodeLicensesOnlyTracker(compiler), chunk);
    if (writeChunkOutputOutcome.isErr()) {
      return Outcome.err(writeChunkOutputOutcome.error());
    }

    JsonFileSpec jsonFile = new JsonFileSpec(output.toString(), filename);

    StringBuilder chunkSourceMap = new StringBuilder();

    Outcome<String> chunkOutputFileName = getChunkOutputFileName(chunk);
    if (chunkOutputFileName.isErr()) {
      return Outcome.err(chunkOutputFileName.error());
    }
    compiler.getSourceMap().appendTo(chunkSourceMap, Optional.ofNullable(chunkOutputFileName.get()));

    jsonFile.setSourceMap(chunkSourceMap.toString());

    return Outcome.ok(jsonFile);
  }

  /**
   * Query the flag for the input charset, and return a Charset object representing the selection.
   *
   * @return Charset to use when reading inputs
   * <p>Returns an error outcome with a FlagUsageException if flag is not a valid Charset name.
   */
  private Outcome<String> getInputCharset() {
    if (!config.charset.isEmpty()) {
      if (!Io.isCharsetSupported(config.charset)) {
        return Outcome.err(
            new FlagUsageException(config.charset + " is not a valid charset name."));
      }
      return Outcome.ok(Io.charsetName(config.charset));
    }
    return Outcome.ok(Io.UTF_8);
  }

  /**
   * Query the flag for the output charset.
   *
   * <p>Let the outputCharset be the same as the input charset... except if we're reading in UTF-8
   * by default. By tradition, we've always output ASCII to avoid various hiccups with different
   * browsers, proxies and firewalls.
   *
   * @return Name of the charset to use when writing outputs. Guaranteed to be a supported charset.
   * <p>Returns an error outcome with a FlagUsageException if flag is not a valid Charset name.
   */
  private Outcome<String> getLegacyOutputCharset() {
    if (!config.charset.isEmpty()) {
      if (!Io.isCharsetSupported(config.charset)) {
        return Outcome.err(
            new FlagUsageException(config.charset + " is not a valid charset name."));
      }
      return Outcome.ok(Io.charsetName(config.charset));
    }
    return Outcome.ok(Io.US_ASCII);
  }

  /**
   * Query the flag for the output charset. Defaults to UTF-8.
   *
   * <p>Returns an error outcome with a FlagUsageException if flag is not a valid Charset name.
   */
  private Outcome<String> getOutputCharset2() {
    if (!config.charset.isEmpty()) {
      if (!Io.isCharsetSupported(config.charset)) {
        return Outcome.err(
            new FlagUsageException(config.charset + " is not a valid charset name."));
      }
      return Outcome.ok(Io.charsetName(config.charset));
    }
    return Outcome.ok(Io.UTF_8);
  }

  protected Outcome<List<SourceFile>> createExterns(CompilerOptions options) throws IOException {
    if (isInTestMode()) {
      return Outcome.ok(externsSupplierForTesting.get());
    }
    return createExternInputs(config.externs);
  }

  /**
   * Returns true if and only if a source map file should be generated for each chunk, as opposed to
   * one unified map. This is specified by having the source map pattern include the %outname%
   * variable.
   */
  protected boolean shouldGenerateMapPerChunk(B options) {
    return options.shouldGatherSourceMapInfo()
        && options.getSourceMapOutputPath().get().contains("%outname%");
  }

  /**
   * Returns a stream for outputting the generated externs file.
   *
   * @param options The options to the Compiler.
   * @param path The path of the generated JS source file.
   * @return The stream or null if no extern-ed exports are being generated.
   */
  private Outcome<Io.@Nullable Output> openExternExportsStream(B options, String path) {
    final Optional<String> externExportsPath = options.getExternExportsPath();
    if (externExportsPath.isEmpty()) {
      return Outcome.ok(null);
    }

    String exPath = externExportsPath.get();

    if (!exPath.contains(Io.separator())) {
      exPath = Io.pathParent(path).orElse("null") + Io.separator() + exPath;
    }

    return fileNameToOutputWriter2(Optional.of(exPath));
  }

  /**
   * Expand a file path specified on the command-line.
   *
   * <p>Most file paths on the command-line allow an %outname% placeholder. The placeholder will
   * expand to a different value depending on the current output mode. There are three scenarios:
   *
   * <p>1) Single JS output, single extra output: sub in jsOutputPath. 2) Multiple JS output, single
   * extra output: sub in the base chunk name. 3) Multiple JS output, multiple extra output: sub in
   * the module output file.
   *
   * <p>Passing a JSChunk to this function automatically triggers case #3. Otherwise, we'll use
   * strategy #1 or #2 based on the current output mode.
   */
  private Outcome<String> expandCommandLinePath(String path, @Nullable JSChunk forChunk) {
    String sub;
    if (forChunk != null) {
      Outcome<String> subOutcome = getChunkOutputFileName(forChunk);
      if (subOutcome.isErr()) {
        return Outcome.err(subOutcome.error());
      }
      sub = subOutcome.get();
    } else if (!config.chunk.isEmpty()) {
      sub = config.chunkOutputPathPrefix;
    } else {
      sub = config.jsOutputFile;
    }
    return Outcome.ok(path.replace("%outname%", sub));
  }

  /** Expansion function for source map. */
  @VisibleForTesting
  Outcome<@Nullable String> expandSourceMapPath(B options, @Nullable JSChunk forChunk) {
    if (!options.shouldGatherSourceMapInfo()) {
      return Outcome.ok(null);
    }
    return expandCommandLinePath(options.getSourceMapOutputPath().get(), forChunk);
  }

  /**
   * Converts a file name into an output taking in account the output charset. Returns null if the
   * file name is null.
   */
  private Outcome<Io.@Nullable Output> fileNameToLegacyOutputWriter(Optional<String> fileName) {
    if (fileName.isEmpty()) {
      return Outcome.ok(null);
    }
    if (isInTestMode()) {
      return Outcome.ok(Io.nullOutput());
    }

    return openOutputFile(fileName.get(), legacyOutputCharset.orElse(Io.UTF_8));
  }

  /**
   * Converts a file name into an output taking in account the output charset. Returns null if the
   * file name is null.
   */
  private Outcome<Io.@Nullable Output> fileNameToOutputWriter2(Optional<String> fileName) {
    if (fileName.isEmpty()) {
      return Outcome.ok(null);
    }
    if (isInTestMode()) {
      return Outcome.ok(Io.nullOutput());
    }

    return openOutputFile(fileName.get(), outputCharset2.orElse(Io.UTF_8));
  }

  /** Creates (or empties) the file and returns a buffered output that encodes with the charset. */
  private static Outcome<Io.Output> openOutputFile(String fileName, String charset) {
    return Io.openOutput(fileName, charset, false);
  }

  /** Create a writer with the legacy output charset. */
  private Io.Output streamToLegacyOutputWriter(Io.Output stream) {
    if (legacyOutputCharset.isEmpty()) {
      return stream.encodedAs(Io.UTF_8);
    } else {
      return stream.encodedAs(legacyOutputCharset.get());
    }
  }

  /**
   * Outputs the source map found in the compiler to the proper path if one exists.
   *
   * @param options The options to the Compiler.
   */
  private Outcome<Void> outputSourceMap(B options, String associatedName) throws IOException {
    if (!options.shouldGatherSourceMapInfo()
        || options.getSourceMapOutputPath().get().equals("/dev/null")) {
      return Outcome.ok();
    }

    Outcome<String> outNameOutcome = expandSourceMapPath(options, null);
    if (outNameOutcome.isErr()) {
      return Outcome.err(outNameOutcome.error());
    }
    String outName = outNameOutcome.get();
    maybeCreateDirsForPath(outName);
    Outcome<Io.Output> outOutcome = fileNameToOutputWriter2(Optional.of(outName));
    if (outOutcome.isErr()) {
      return Outcome.err(outOutcome.error());
    }
    Io.Output out = outOutcome.get();
    Outcome<Void> outClosed;
    try {
      compiler.getSourceMap().appendTo(out, Optional.of(associatedName));
    } finally {
      outClosed = closeOutput(out);
    }
    if (outClosed.isErr()) {
      return Outcome.err(outClosed.error());
    }
    return Outcome.ok();
  }

  /**
   * Returns the path at which to output map file(s) based on the path at which the JS binary will
   * be placed.
   *
   * @return The path in which to place the generated map file(s).
   */
  private String getMapPath(String outputFile) {
    String basePath = "";

    if (outputFile.isEmpty()) {
      // If we have a js_module_binary rule, output the maps
      // at chunkname_props_map.out, etc.
      if (!config.chunkOutputPathPrefix.isEmpty()) {
        basePath = config.chunkOutputPathPrefix;
      } else {
        basePath = "jscompiler";
      }
    } else {
      // Get the path of the output file.
      String outputFileName = Io.pathFileName(outputFile);

      // Strip the .js from the name.
      if (outputFileName.endsWith(".js")) {
        outputFileName = outputFileName.substring(0, outputFileName.length() - 3);
      }

      Optional<String> fileParent = Io.pathParent(outputFile);
      if (fileParent.isEmpty()) {
        basePath = outputFileName;
      } else {
        basePath = fileParent.get() + Io.separator() + outputFileName;
      }
    }

    return basePath;
  }

  /**
   * Outputs the variable and property name maps for the specified compiler if the proper FLAGS are
   * set.
   */
  private Outcome<Void> outputNameMaps() throws IOException {

    Optional<String> propertyMapOutputPath = Optional.empty();
    Optional<String> variableMapOutputPath = Optional.empty();

    // Check the create_name_map_files FLAG.
    if (config.createNameMapFiles) {
      String basePath = getMapPath(config.jsOutputFile);

      propertyMapOutputPath = Optional.of(basePath + "_props_map.out");
      variableMapOutputPath = Optional.of(basePath + "_vars_map.out");
    }

    // Check the individual FLAGS.
    if (!config.variableMapOutputFile.isEmpty()) {
      if (variableMapOutputPath.isPresent()) {
        return Outcome.err(new FlagUsageException(
            "The flags variable_map_output_file and "
                + "create_name_map_files cannot both be used simultaneously."));
      }

      variableMapOutputPath = Optional.of(config.variableMapOutputFile);
    }

    if (!config.propertyMapOutputFile.isEmpty()) {
      if (propertyMapOutputPath.isPresent()) {
        return Outcome.err(new FlagUsageException(
            "The flags property_map_output_file and "
                + "create_name_map_files cannot both be used simultaneously."));
      }

      propertyMapOutputPath = Optional.of(config.propertyMapOutputFile);
    }

    // Output the maps.
    if (variableMapOutputPath.isPresent() && compiler.getVariableMap() != null) {
      Outcome<Void> savedVariableMap = compiler.getVariableMap().save(variableMapOutputPath.get());
      if (savedVariableMap.isErr()) {
        return Outcome.err(savedVariableMap.error());
      }
    }

    if (propertyMapOutputPath.isPresent() && compiler.getPropertyMap() != null) {
      Outcome<Void> savedPropertyMap = compiler.getPropertyMap().save(propertyMapOutputPath.get());
      if (savedPropertyMap.isErr()) {
        return Outcome.err(savedPropertyMap.error());
      }
    }
    return Outcome.ok();
  }

  /**
   * Outputs the string map generated by the {@link ReplaceStrings} pass if an output path exists.
   */
  private Outcome<Void> outputStringMap() {
    if (!config.stringMapOutputPath.isEmpty()) {
      if (compiler.getStringMap() == null) {
        // Ensure an empty file is created if there is no string map.
        // This avoids confusing some build tools that expect to see the file, even if it is empty.
        Outcome<Boolean> created = Io.createNewFile(config.stringMapOutputPath);
        if (created.isErr()) {
          return Outcome.err(created.error());
        }
        if (!created.get()) {
          return Outcome.err(
              new IOException("Could not create file: " + config.stringMapOutputPath));
        }
      } else {
        Outcome<Void> savedStringMap = compiler.getStringMap().save(config.stringMapOutputPath);
        if (savedStringMap.isErr()) {
          return Outcome.err(savedStringMap.error());
        }
      }
    }
    return Outcome.ok();
  }

  /**
   * Create a map of constant names to constant values from a textual description of the map.
   *
   * @param definitions A list of overriding definitions for defines in the form {@code
   *     <name>[=<val>]}, where {@code <val>} is a number, boolean, or single-quoted string without
   *     single quotes.
   */
  @VisibleForTesting
  public static void createDefineReplacements(List<String> definitions, CompilerOptions options) {
    // Parse the definitions
    for (String override : definitions) {
      String[] assignment = EQUALS_PATTERN.split(override, 2);
      String defName = assignment[0];

      if (defName.length() > 0) {
        String defValue = assignment.length == 1 ? "true" : assignment[1];

        boolean isTrue = defValue.equals("true");
        boolean isFalse = defValue.equals("false");
        if (isTrue || isFalse) {
          options.setDefineToBooleanLiteral(defName, isTrue);
          continue;
        } else if (defValue.length() > 1
            && ((defValue.charAt(0) == '\'' && defValue.charAt(defValue.length() - 1) == '\'')
                || (defValue.charAt(0) == '\"'
                    && defValue.charAt(defValue.length() - 1) == '\"'))) {
          // If the value starts and ends with a single quote,
          // we assume that it's a string.
          String maybeStringVal = defValue.substring(1, defValue.length() - 1);
          if (maybeStringVal.indexOf(defValue.charAt(0)) == -1) {
            options.setDefineToStringLiteral(defName, maybeStringVal);
            continue;
          }
        } else {
          // Double.parseDouble trims its argument; other than that Doubles.tryParse accepts the
          // same.
          Double value = Doubles.tryParse(defValue.trim());
          if (value != null) {
            options.setDefineToDoubleLiteral(defName, value);
            continue;
          }
          // Otherwise do nothing, it will be caught at the end

          if (defValue.length() > 0) {
            options.setDefineToStringLiteral(defName, defValue);
            continue;
          }
        }
      }

      throw new RuntimeException("--define flag syntax invalid: " + override);
    }
  }

  /**
   * Returns true if and only if a manifest or bundle should be generated for each chunk, as opposed
   * to one unified manifest.
   */
  private boolean shouldGenerateOutputPerChunk(Optional<String> output) {
    return !config.chunk.isEmpty() && output.isPresent() && output.get().contains("%outname%");
  }

  private Outcome<Void> outputManifest() throws IOException {
    Outcome<Void> outputManifestOrBundleOutcome =
        outputManifestOrBundle(config.outputManifests, true);
    if (outputManifestOrBundleOutcome.isErr()) {
      return Outcome.err(outputManifestOrBundleOutcome.error());
    }
    return Outcome.ok();
  }

  private Outcome<Void> outputBundle() throws IOException {
    Outcome<Void> outputManifestOrBundleOutcome =
        outputManifestOrBundle(config.outputBundles, false);
    if (outputManifestOrBundleOutcome.isErr()) {
      return Outcome.err(outputManifestOrBundleOutcome.error());
    }
    return Outcome.ok();
  }

  private Outcome<Void> outputInstrumentationMapping() throws IOException {
    if (!Strings.isNullOrEmpty(config.instrumentationMappingFile)) {
      Outcome<String> pathOutcome =
          expandCommandLinePath(config.instrumentationMappingFile, /* forChunk= */ null);
      if (pathOutcome.isErr()) {
        return Outcome.err(pathOutcome.error());
      }
      String path = pathOutcome.get();
      Outcome<Void> savedMapping = compiler.getInstrumentationMapping().save(path);
      if (savedMapping.isErr()) {
        return Outcome.err(savedMapping.error());
      }
    }
    return Outcome.ok();
  }

  /**
   * Writes the manifest or bundle of all compiler input files that were included as controlled by
   * --dependency_mode, if requested.
   */
  private Outcome<Void> outputManifestOrBundle(List<String> outputFiles, boolean isManifest)
      throws IOException {
    if (outputFiles.isEmpty()) {
      return Outcome.ok();
    }

    for (String output : outputFiles) {
      if (output.isEmpty()) {
        continue;
      }

      if (shouldGenerateOutputPerChunk(Optional.of(output))) {
        // Generate per-chunk manifests or bundles.
        Iterable<JSChunk> chunks = compiler.getChunkGraph().getAllChunks();
        for (JSChunk chunk : chunks) {
          Outcome<String> chunkOutputPath = expandCommandLinePath(output, chunk);
          if (chunkOutputPath.isErr()) {
            return Outcome.err(chunkOutputPath.error());
          }
          Outcome<Io.Output> outOutcome = fileNameToOutputWriter2(Optional.ofNullable(chunkOutputPath.get()));
          if (outOutcome.isErr()) {
            return Outcome.err(outOutcome.error());
          }
          Io.Output out = outOutcome.get();
          Outcome<Void> outClosed;
          try {
            if (isManifest) {
              printManifestTo(chunk, out);
            } else {
              Outcome<Void> printBundleToOutcome = printBundleTo(chunk, out);
              if (printBundleToOutcome.isErr()) {
                return Outcome.err(printBundleToOutcome.error());
              }
            }
          } finally {
            outClosed = closeOutput(out);
          }
          if (outClosed.isErr()) {
            return Outcome.err(outClosed.error());
          }
        }
      } else {
        // Generate a single file manifest or bundle.
        Outcome<String> outputPath = expandCommandLinePath(output, null);
        if (outputPath.isErr()) {
          return Outcome.err(outputPath.error());
        }
        Outcome<Io.Output> outOutcome = fileNameToOutputWriter2(Optional.ofNullable(outputPath.get()));
        if (outOutcome.isErr()) {
          return Outcome.err(outOutcome.error());
        }
        Io.Output out = outOutcome.get();
        Outcome<Void> outClosed;
        try {
          if (config.chunk.isEmpty()) {
            // For a single-chunk compilation, generate a single headerless manifest or bundle
            // containing only the strong files.
            JSChunk chunk = compiler.getChunkGraph().getChunkByName(JSChunk.STRONG_CHUNK_NAME);
            if (isManifest) {
              printManifestTo(chunk, out);
            } else {
              Outcome<Void> printBundleToOutcome2 = printBundleTo(chunk, out);
              if (printBundleToOutcome2.isErr()) {
                return Outcome.err(printBundleToOutcome2.error());
              }
            }
          } else {
            // For a multi-chunk compilation, generate a single manifest file with chunk headers.
            Outcome<Void> printChunkGraphManifestOrBundleToOutcome =
                printChunkGraphManifestOrBundleTo(compiler.getChunkGraph(), out, isManifest);
            if (printChunkGraphManifestOrBundleToOutcome.isErr()) {
              return Outcome.err(printChunkGraphManifestOrBundleToOutcome.error());
            }
          }
        } finally {
          outClosed = closeOutput(out);
        }
        if (outClosed.isErr()) {
          return Outcome.err(outClosed.error());
        }
      }
    }
    return Outcome.ok();
  }

  /** Creates a file containing the current chunk graph in JSON serialization. */
  private Outcome<Void> outputChunkGraphJson() throws IOException {
    if (config.outputChunkDependencies.isPresent() && config.outputChunkDependencies.get().length() != 0) {
      Outcome<Io.Output> outOutcome = fileNameToOutputWriter2(config.outputChunkDependencies);
      if (outOutcome.isErr()) {
        return Outcome.err(outOutcome.error());
      }
      Io.Output out = outOutcome.get();
      Outcome<Void> outClosed;
      try {
        printChunkGraphJsonTo(out);
      } finally {
        outClosed = closeOutput(out);
      }
      if (outClosed.isErr()) {
        return Outcome.err(outClosed.error());
      }
    }
    return Outcome.ok();
  }

  /** Prints the current chunk graph as JSON. */
  @VisibleForTesting
  void printChunkGraphJsonTo(Appendable out) throws IOException {
    out.append(compiler.getChunkGraph().toJson().toString());
  }

  /** Prints a set of chunks to the manifest or bundle file. */
  @VisibleForTesting
  Outcome<Void> printChunkGraphManifestOrBundleTo(
      JSChunkGraph graph, Appendable out, boolean isManifest)
      throws IOException {
    Joiner commas = Joiner.on(",");
    boolean requiresNewline = false;
    for (JSChunk chunk : graph.getAllChunks()) {
      if (!isManifest && chunk.isWeak()) {
        // Skip the weak chunk on a multi-chunk bundle, but not a multi-chunk manifest.
        continue;
      }

      if (requiresNewline) {
        out.append("\n");
      }

      if (isManifest) {
        // See CommandLineRunnerTest to see what the format of this
        // manifest looks like.
        String dependencies = commas.join(chunk.getSortedDependencyNames());
        out.append(
            String.format(
                "{%s%s}\n", chunk.getName(), dependencies.isEmpty() ? "" : ":" + dependencies));
        printManifestTo(chunk, out);
      } else {
        Outcome<Void> printBundleToOutcome = printBundleTo(chunk, out);
        if (printBundleToOutcome.isErr()) {
          return Outcome.err(printBundleToOutcome.error());
        }
      }
      requiresNewline = true;
    }
    return Outcome.ok();
  }

  /**
   * Prints a list of input names (using root-relative paths), delimited by newlines, to the
   * manifest file.
   */
  @VisibleForTesting
  void printManifestTo(JSChunk chunk, Appendable out) throws IOException {
    for (CompilerInput input : chunk.getInputs()) {
      String name = input.getName();

      // Ignore fill files created by the compiler to facilitate cross-chunk code motion.
      if (Compiler.isFillFileName(name)) {
        continue;
      }

      Optional<String> rootRelativePath = Optional.ofNullable(rootRelativePathsMap.get(name));
      String displayName = rootRelativePath.orElse(name);
      out.append(displayName);
      out.append("\n");
    }
  }

  /**
   * Prints all the input contents, starting with a comment that specifies the input file name
   * (using root-relative paths) before each file.
   */
  @VisibleForTesting
  Outcome<Void> printBundleTo(JSChunk chunk, Appendable out) throws IOException {
    ImmutableList<CompilerInput> inputs = chunk.getInputs();
    CompilerOptions options = compiler.getOptions();
    // Prebuild ASTs before they're needed in getLoadFlags, for performance and because
    // StackOverflowErrors can be hit if not prebuilt.
    if (options.getNumParallelThreads() > 1) {
      new PrebuildAst(compiler, compiler.getOptions().getNumParallelThreads()).prebuild(inputs);
    }
    if (options.getRuntimeLibraryMode() == RuntimeJsLibManager.RuntimeLibraryMode.INJECT) {
      // ES6 modules will need a runtime in a bundle. Skip appending this runtime if there are no
      // ES6 modules to cut down on size.
      for (CompilerInput input : inputs) {
        if (input.isEs6Module()) {
          Outcome<Void> appendRuntimeToOutcome = appendRuntimeTo(out);
          if (appendRuntimeToOutcome.isErr()) {
            return Outcome.err(appendRuntimeToOutcome.error());
          }
          break;
        }
      }
    }

    for (CompilerInput input : inputs) {
      String name = input.getName();
      Outcome<String> codeRes = input.getSourceFile().readCode();
      if (codeRes.isErr()) {
        return Outcome.err(codeRes.error());
      }
      String code = codeRes.get();

      // Ignore empty fill files created by the compiler to facilitate cross-chunk code motion.
      // Note that non-empty fill files (ones whose code has actually been moved into) are still
      // emitted. In particular, this ensures that if there are no (real) inputs the bundle will be
      // empty.
      if (Compiler.isFillFileName(name) && code.isEmpty()) {
        continue;
      }

      Optional<String> rootRelativePath = Optional.ofNullable(rootRelativePathsMap.get(name));
      String displayName = rootRelativePath.isPresent() ? rootRelativePath.get() : input.getName();
      out.append("//");
      out.append(displayName);
      out.append("\n");

      Outcome<Void> prepForBundleAndAppendToOutcome = prepForBundleAndAppendTo(out, input, code);
      if (prepForBundleAndAppendToOutcome.isErr()) {
        return Outcome.err(prepForBundleAndAppendToOutcome.error());
      }

      out.append("\n");
    }
    return Outcome.ok();
  }

  /**
   * Construct and return the input root path map. The key is the exec path of each input file, and
   * the value is the corresponding root relative path.
   */
  private Map<String, String> constructRootRelativePathsMap() {
    Map<String, String> rootRelativePathsMap = new LinkedHashMap<>();
    for (String mapString : config.manifestMaps) {
      int colonIndex = mapString.indexOf(':');
      checkState(colonIndex > 0);
      String execPath = mapString.substring(0, colonIndex);
      String rootRelativePath = mapString.substring(colonIndex + 1);
      checkState(rootRelativePath.indexOf(':') == -1);
      rootRelativePathsMap.put(execPath, rootRelativePath);
    }
    return rootRelativePathsMap;
  }

  /**
   * Configurations for the command line configs. Designed for easy building, so that we can
   * decouple the flags-parsing library from the actual configuration options.
   *
   * <p>TODO(tjgq): Investigate whether this class is really needed to mediate between the
   * CompilerOptions and runner implementations. An alternative would be for the runners to fill in
   * the CompilerOptions directly, but that conflicts with the latter's mutability and the desire to
   * reuse the same options across multiple compilations.
   */
  protected static class CommandLineConfig {

    private boolean printVersion;

    @CanIgnoreReturnValue
    CommandLineConfig setPrintVersion(boolean x) {
      this.printVersion = x;
      return this;
    }

    private boolean printTree = false;
    private boolean printTreeJson = false;

    /** Prints out the parse tree and exits */
    @CanIgnoreReturnValue
    CommandLineConfig setPrintTree(boolean printTree) {
      this.printTree = printTree;
      return this;
    }

    /** Prints out the parse tree and exits */
    @CanIgnoreReturnValue
    CommandLineConfig setPrintTreeJson(boolean printTreeJson) {
      this.printTreeJson = printTreeJson;
      return this;
    }

    private boolean printAst = false;

    /** Prints a dot file describing the internal abstract syntax tree and exits */
    @CanIgnoreReturnValue
    public CommandLineConfig setPrintAst(boolean printAst) {
      this.printAst = printAst;
      return this;
    }

    private CompilerOptions.DevMode jscompDevMode = CompilerOptions.DevMode.OFF;

    /** Turns on extra validity checks */
    @CanIgnoreReturnValue
    public CommandLineConfig setJscompDevMode(CompilerOptions.DevMode jscompDevMode) {
      this.jscompDevMode = jscompDevMode;
      return this;
    }

    private String loggingLevel = Level.WARNING.getName();

    /**
     * The logging level (standard java.util.logging.Level values) for Compiler progress. Does not
     * control errors or warnings for the JavaScript code under compilation
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setLoggingLevel(String loggingLevel) {
      this.loggingLevel = loggingLevel;
      return this;
    }

    private final List<String> externs = new ArrayList<>();

    /** The file containing JavaScript externs. You may specify multiple. */
    @CanIgnoreReturnValue
    public CommandLineConfig setExterns(List<String> externs) {
      this.externs.clear();
      this.externs.addAll(externs);
      return this;
    }

    private final List<FlagEntry<JsSourceType>> mixedJsSources = new ArrayList<>();

    /** The JavaScript source file names, including .js and .zip files. You may specify multiple. */
    @CanIgnoreReturnValue
    public CommandLineConfig setMixedJsSources(List<FlagEntry<JsSourceType>> mixedJsSources) {
      this.mixedJsSources.clear();
      this.mixedJsSources.addAll(mixedJsSources);
      return this;
    }

    private boolean defaultToStdin = false;

    /**
     * Whether to read a single source file from standard input if no input files are explicitly
     * specified.
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setDefaultToStdin() {
      this.defaultToStdin = true;
      return this;
    }

    private String jsOutputFile = "";

    /** Primary output filename. If not specified, output is written to stdout */
    @CanIgnoreReturnValue
    public CommandLineConfig setJsOutputFile(String jsOutputFile) {
      this.jsOutputFile = jsOutputFile;
      return this;
    }

    /**
     * When non-null specifies a file containing saved compiler state to restore and continue
     * compiling.
     */
    private Optional<String> continueSavedCompilationFileName = Optional.empty();

    /**
     * When > 0 indicates the stage at which compilation stopped for the compilation state that is
     * being restored from a save file.
     */
    private int restoredCompilationStage = -1;

    /**
     * When > 0 indicates the stage at which compilation should stop and the compilation state be
     * stored into a save file.
     */
    private int saveAfterCompilationStage = -1;

    /** Set the compiler to resume a saved compilation state from a file. */
    @CanIgnoreReturnValue
    public CommandLineConfig setContinueSavedCompilationFileName(
        Optional<String> fileName, int restoredCompilationStage) {
      if (fileName.isPresent()) {
        checkArgument(
            restoredCompilationStage > 0 && restoredCompilationStage < 3,
            "invalid compilation stage: %s",
            restoredCompilationStage);
        continueSavedCompilationFileName = fileName;
        this.restoredCompilationStage = restoredCompilationStage;
      }
      return this;
    }

    boolean shouldSaveAfterStage1() {
      if (saveCompilationStateToFilename.isPresent() && saveAfterCompilationStage == 1) {
        checkState(
            restoredCompilationStage < 0,
            "cannot perform stage 1 after restoring from stage %s",
            restoredCompilationStage);
        checkState(
            continueSavedCompilationFileName.isEmpty(),
            "cannot restore a saved compilation and also save after stage 1");
        return true;
      } else {
        return false;
      }
    }

    Optional<String> getContinueSavedCompilationFileName() {
      return continueSavedCompilationFileName;
    }

    boolean shouldRestoreAndPerformStages2And3() {
      // We have a saved compilations state to restore
      return shouldContinueCompilation()
          // the restored compilation stopped at stage 1
          && restoredCompilationStage == 1
          // we want to complete the rest of the compilation stages
          && saveAfterCompilationStage < 0;
    }

    private boolean shouldContinueCompilation() {
      if (continueSavedCompilationFileName.isPresent()) {
        checkState(
            restoredCompilationStage > 0 && restoredCompilationStage < 3,
            "invalid restored compilation stage: %s",
            restoredCompilationStage);
        return true;
      } else {
        return false;
      }
    }

    boolean shouldRestoreTypedAstsPerformStage2AndSave() {
      // We have a typed ast input list to parse
      return typedAstListInputFilename.isPresent()
          // we want to stop and save after optimizations
          && saveAfterCompilationStage == 2;
    }

    boolean shouldRestoreTypedAstsPerformStages2And3() {
      // We have a typed ast input list to parse
      return typedAstListInputFilename.isPresent()
          // we do not want to stop and save after optimizations
          && saveAfterCompilationStage == -1;
    }

    boolean shouldRestoreAndPerformStage2AndSave() {
      // We have a saved compilations state to restore
      return shouldContinueCompilation()
          // the restored compilation stopped at stage 1
          && restoredCompilationStage == 1
          // we want to stop and save after stage 2
          && saveAfterCompilationStage == 2;
    }

    boolean shouldRestoreAndPerformStage3() {
      if (shouldContinueCompilation() && restoredCompilationStage == 2) {
        // We have a saved compilations state from stage 2 to restore and continue
        checkState(
            saveAfterCompilationStage < 0,
            "request to save after stage %s is invalid when restoring from stage 2",
            saveAfterCompilationStage);
        return true;
      } else {
        return false;
      }
    }

    boolean shouldDoLateLocalization() {
      // The point of dividing checks, optimizations, and finalizations into different stages is to
      // localization work for last, so we avoid doing checks and optimizations separately for every
      // locale.
      // If we aren't doing finalizations as a separate final stage, then late localization is just
      // doing more work for a possibly-bigger compiled output (because code gets added after
      // optimizations have already executed).
      return shouldRestoreAndPerformStage2AndSave()
          || shouldRestoreAndPerformStage3()
          || shouldRestoreTypedAstsPerformStage2AndSave();
    }

    boolean shouldAlwaysGatherSourceMapInfo() {
      // If we're doing a partial compilation that isn't the final stage, we need to always gather
      // source map info in case the final stage requires it.
      return shouldSaveAfterStage1()
          || shouldRestoreAndPerformStage2AndSave()
          || shouldRestoreTypedAstsPerformStage2AndSave();
    }

    private Optional<String> typedAstListInputFilename = Optional.empty();

    @CanIgnoreReturnValue
    public CommandLineConfig setTypedAstListInputFilename(Optional<String> fileName) {
      this.typedAstListInputFilename = fileName;
      return this;
    }

    private Optional<String> saveCompilationStateToFilename = Optional.empty();

    /** Set the compiler to perform the first phase and save the intermediate result to a file. */
    @CanIgnoreReturnValue
    public CommandLineConfig setSaveCompilationStateToFilename(
        Optional<String> fileName, int saveAfterCompilationStage) {
      saveCompilationStateToFilename = fileName;
      this.saveAfterCompilationStage = saveAfterCompilationStage;
      return this;
    }

    public Optional<String> getSaveCompilationStateToFilename() {
      return saveCompilationStateToFilename;
    }

    private final List<String> chunk = new ArrayList<>();

    /**
     * A JavaScript chunk specification. The format is {@code
     * <name>:<num-js-files>[:[<dep>,...][:]]]}. Chunk names must be unique. Each dep is the name of
     * a chunk that this chunk depends on. Chunks must be listed in dependency order, and JS source
     * files must be listed in the corresponding order. Where --chunk flags occur in relation to
     * --js flags is unimportant
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setChunk(List<String> chunk) {
      this.chunk.clear();
      this.chunk.addAll(chunk);
      return this;
    }

    private Map<String, String> sourceMapInputFiles = new LinkedHashMap<>();

    @CanIgnoreReturnValue
    public CommandLineConfig setSourceMapInputFiles(Map<String, String> sourceMapInputFiles) {
      this.sourceMapInputFiles = sourceMapInputFiles;
      return this;
    }

    private boolean parseInlineSourceMaps = false;

    @CanIgnoreReturnValue
    public CommandLineConfig setParseInlineSourceMaps(boolean parseInlineSourceMaps) {
      this.parseInlineSourceMaps = parseInlineSourceMaps;
      return this;
    }

    private final List<String> expectedDiagnostics = new ArrayList<>();

    /**
     * Expected diagnostics in the format [(line,col)]CODE:regex. CODE is the JSCompiler
     * DiagnosticType key. regex matches the description.
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setExpectedDiagnostics(List<String> expectedDiagnostics) {
      this.expectedDiagnostics.clear();
      this.expectedDiagnostics.addAll(expectedDiagnostics);
      return this;
    }

    private String variableMapInputFile = "";

    /**
     * File containing the serialized version of the variable renaming map produced by a previous
     * compilation
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setVariableMapInputFile(String variableMapInputFile) {
      this.variableMapInputFile = variableMapInputFile;
      return this;
    }

    private String propertyMapInputFile = "";

    /**
     * File containing the serialized version of the property renaming map produced by a previous
     * compilation
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setPropertyMapInputFile(String propertyMapInputFile) {
      this.propertyMapInputFile = propertyMapInputFile;
      return this;
    }

    private String variableMapOutputFile = "";

    /** File where the serialized version of the variable renaming map produced should be saved */
    @CanIgnoreReturnValue
    public CommandLineConfig setVariableMapOutputFile(String variableMapOutputFile) {
      this.variableMapOutputFile = variableMapOutputFile;
      return this;
    }

    private boolean createNameMapFiles = false;

    /**
     * If true, variable renaming and property renaming map files will be produced as {binary
     * name}_vars_map.out and {binary name}_props_map.out. Note that this flag cannot be used in
     * conjunction with either variable_map_output_file or property_map_output_file
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setCreateNameMapFiles(boolean createNameMapFiles) {
      this.createNameMapFiles = createNameMapFiles;
      return this;
    }

    private String propertyMapOutputFile = "";

    /** File where the serialized version of the property renaming map produced should be saved */
    @CanIgnoreReturnValue
    public CommandLineConfig setPropertyMapOutputFile(String propertyMapOutputFile) {
      this.propertyMapOutputFile = propertyMapOutputFile;
      return this;
    }

    private String stringMapOutputPath = "";

    /**
     * File where the serialized version of the string map produced by the ReplaceStrings pass
     * should be saved.
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setStringMapOutputFile(String stringMapOutputPath) {
      this.stringMapOutputPath = stringMapOutputPath;
      return this;
    }

    private String instrumentationMappingFile = "";

    @CanIgnoreReturnValue
    public CommandLineConfig setInstrumentationMappingFile(String instrumentationMappingFile) {
      this.instrumentationMappingFile = instrumentationMappingFile;
      return this;
    }

    private CodingConvention codingConvention = CodingConventions.getDefault();

    /** Sets rules and conventions to enforce. */
    @CanIgnoreReturnValue
    public CommandLineConfig setCodingConvention(CodingConvention codingConvention) {
      this.codingConvention = codingConvention;
      return this;
    }

    private int summaryDetailLevel = 1;

    /**
     * Controls how detailed the compilation summary is. Values: 0 (never print summary), 1 (print
     * summary only if there are errors or warnings), 2 (print summary if type checking is on), 3
     * (always print summary). The default level is 1
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setSummaryDetailLevel(int summaryDetailLevel) {
      this.summaryDetailLevel = summaryDetailLevel;
      return this;
    }

    private String outputWrapper = "";

    /**
     * Interpolate output into this string at the place denoted by the marker token %output%, or
     * %output|jsstring%
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setOutputWrapper(String outputWrapper) {
      this.outputWrapper = outputWrapper;
      return this;
    }

    private final List<String> chunkWrapper = new ArrayList<>();

    /**
     * An output wrapper for a JavaScript module (optional). See the flag description for formatting
     * requirements.
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setChunkWrapper(List<String> chunkWrapper) {
      this.chunkWrapper.clear();
      this.chunkWrapper.addAll(chunkWrapper);
      return this;
    }

    private String chunkOutputPathPrefix = "";

    /**
     * Prefix for filenames of compiled chunks. {@code <chunk-name>.js} will be appended to this
     * prefix. Directories will be created as needed. Use with --chunk
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setChunkOutputPathPrefix(String chunkOutputPathPrefix) {
      this.chunkOutputPathPrefix = chunkOutputPathPrefix;
      return this;
    }

    private final List<String> chunkOutputFiles = new ArrayList<>();

    /**
     * The output file name for a JavaScript chunk (optional). See the flag description for
     * formatting requirements.
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setChunkOutputFiles(List<String> chunkOutputFiles) {
      this.chunkOutputFiles.clear();
      this.chunkOutputFiles.addAll(chunkOutputFiles);
      return this;
    }

    private final List<String> chunkConformanceFiles = new ArrayList<>();

    /**
     * The conformance report file name for a JavaScript chunk (optional). See the flag description
     * for formatting requirements.
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setChunkConformanceFiles(List<String> chunkConformanceFiles) {
      this.chunkConformanceFiles.clear();
      this.chunkConformanceFiles.addAll(chunkConformanceFiles);
      return this;
    }

    private String createSourceMap = "";

    /**
     * If specified, a source map file mapping the generated source files back to the original
     * source file will be output to the specified path. The %outname% placeholder will expand to
     * the name of the output file that the source map corresponds to.
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setCreateSourceMap(String createSourceMap) {
      this.createSourceMap = createSourceMap;
      return this;
    }

    private SourceMap.DetailLevel sourceMapDetailLevel = SourceMap.DetailLevel.ALL;

    /** The detail supplied in the source map file, if generated. */
    @CanIgnoreReturnValue
    public CommandLineConfig setSourceMapDetailLevel(SourceMap.DetailLevel level) {
      this.sourceMapDetailLevel = level;
      return this;
    }

    private SourceMap.Format sourceMapFormat = SourceMap.Format.DEFAULT;

    /** The source map format to use, if generated. */
    @CanIgnoreReturnValue
    public CommandLineConfig setSourceMapFormat(SourceMap.Format format) {
      this.sourceMapFormat = format;
      return this;
    }

    private ImmutableList<SourceMap.LocationMapping> sourceMapLocationMappings = ImmutableList.of();

    /** The source map location mappings to use, if generated. */
    @CanIgnoreReturnValue
    public CommandLineConfig setSourceMapLocationMappings(
        List<SourceMap.LocationMapping> locationMappings) {

      this.sourceMapLocationMappings = ImmutableList.copyOf(locationMappings);
      return this;
    }

    private boolean applyInputSourceMaps = false;

    /**
     * Whether to apply input source maps to the output, i.e. map back to original inputs from input
     * files that have source maps applied to them.
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setApplyInputSourceMaps(boolean applyInputSourceMaps) {
      this.applyInputSourceMaps = applyInputSourceMaps;
      return this;
    }

    private final ArrayList<FlagEntry<CheckLevel>> warningGuards = new ArrayList<>();

    /** Add warning guards. */
    @CanIgnoreReturnValue
    public CommandLineConfig setWarningGuards(List<FlagEntry<CheckLevel>> warningGuards) {
      this.warningGuards.clear();
      this.warningGuards.addAll(warningGuards);
      return this;
    }

    private final List<String> define = new ArrayList<>();

    /**
     * Override the value of a variable annotated @define. The format is {@code <name>[=<val>]},
     * where {@code <name>} is the name of a @define variable and {@code <val>} is a boolean,
     * number, or a single-quoted string that contains no single quotes. If {@code [=<val>]} is
     * omitted, the variable is marked true
     */
    @CanIgnoreReturnValue
    public CommandLineConfig setDefine(List<String> define) {
      this.define.clear();
      this.define.addAll(define);
      return this;
    }

    private int browserFeaturesetYear = 0;

    /** Indicates target browser's year */
    @CanIgnoreReturnValue
    public CommandLineConfig setBrowserFeaturesetYear(Integer browserFeaturesetYear) {
      this.browserFeaturesetYear = browserFeaturesetYear;
      return this;
    }

    private TweakProcessing tweakProcessing = TweakProcessing.OFF;

    /** Sets the kind of processing to do for goog.tweak functions. */
    @CanIgnoreReturnValue
    public CommandLineConfig setTweakProcessing(TweakProcessing tweakProcessing) {
      this.tweakProcessing = tweakProcessing;
      return this;
    }

    private String charset = "";

    /** Input charset for all files. */
    @CanIgnoreReturnValue
    public CommandLineConfig setCharset(String charset) {
      this.charset = charset;
      return this;
    }

    private @Nullable DependencyOptions dependencyOptions = null;

    /** Sets the dependency management options. */
    @CanIgnoreReturnValue
    public CommandLineConfig setDependencyOptions(@Nullable DependencyOptions dependencyOptions) {
      this.dependencyOptions = dependencyOptions;
      return this;
    }

    private List<String> outputManifests = ImmutableList.of();

    /** Sets whether to print output manifest files. Filter out empty file names. */
    @CanIgnoreReturnValue
    public CommandLineConfig setOutputManifest(List<String> outputManifests) {
      this.outputManifests = new ArrayList<>();
      for (String manifestName : outputManifests) {
        if (!manifestName.isEmpty()) {
          this.outputManifests.add(manifestName);
        }
      }
      this.outputManifests = ImmutableList.copyOf(this.outputManifests);
      return this;
    }

    private Optional<String> outputChunkDependencies = Optional.empty();

    /** Sets whether a JSON file representing the dependencies between modules should be created. */
    @CanIgnoreReturnValue
    public CommandLineConfig setOutputChunkDependencies(String outputChunkDependencies) {
      this.outputChunkDependencies = Optional.of(outputChunkDependencies);
      return this;
    }

    private List<String> outputBundles = ImmutableList.of();

    /** Sets whether to print output bundle files. */
    @CanIgnoreReturnValue
    public CommandLineConfig setOutputBundle(List<String> outputBundles) {
      this.outputBundles = outputBundles;
      return this;
    }

    private boolean skipNormalOutputs = false;

    /** Sets whether the normal outputs of compilation should be skipped. */
    @CanIgnoreReturnValue
    public CommandLineConfig setSkipNormalOutputs(boolean skipNormalOutputs) {
      this.skipNormalOutputs = skipNormalOutputs;
      return this;
    }

    private List<String> manifestMaps = ImmutableList.of();

    /** Sets the execPath:rootRelativePath mappings */
    @CanIgnoreReturnValue
    public CommandLineConfig setManifestMaps(List<String> manifestMaps) {
      this.manifestMaps = manifestMaps;
      return this;
    }

    private boolean processCommonJSModules = false;

    /** Sets whether to process CommonJS modules,. */
    @CanIgnoreReturnValue
    public CommandLineConfig setProcessCommonJSModules(boolean processCommonJSModules) {
      this.processCommonJSModules = processCommonJSModules;
      return this;
    }

    private List<String> moduleRoots = ImmutableList.of(ModuleLoader.DEFAULT_FILENAME_PREFIX);

    /** Sets the module roots. */
    @CanIgnoreReturnValue
    public CommandLineConfig setModuleRoots(List<String> jsModuleRoots) {
      this.moduleRoots = jsModuleRoots;
      return this;
    }

    private String warningsAllowFile = "";

    /** Sets a allowlist file that suppresses warnings. */
    @CanIgnoreReturnValue
    public CommandLineConfig setWarningsAllowlistFile(String fileName) {
      this.warningsAllowFile = fileName;
      return this;
    }

    private List<String> hideWarningsFor = ImmutableList.of();

    /** Sets the paths for which warnings will be hidden. */
    @CanIgnoreReturnValue
    public CommandLineConfig setHideWarningsFor(List<String> hideWarningsFor) {
      this.hideWarningsFor = hideWarningsFor;
      return this;
    }

    private boolean angularPass = false;

    /** Sets whether to process AngularJS-specific annotations. */
    @CanIgnoreReturnValue
    public CommandLineConfig setAngularPass(boolean angularPass) {
      this.angularPass = angularPass;
      return this;
    }

    private JsonStreamMode jsonStreamMode = JsonStreamMode.NONE;

    @CanIgnoreReturnValue
    public CommandLineConfig setJsonStreamMode(JsonStreamMode mode) {
      this.jsonStreamMode = mode;
      return this;
    }

    /** Set of options that can be used with the --formatting flag. */
    protected enum ErrorFormatOption {
      STANDARD,
      JSON
    }

    private ErrorFormatOption errorFormat = ErrorFormatOption.STANDARD;

    @CanIgnoreReturnValue
    public CommandLineConfig setErrorFormat(ErrorFormatOption errorFormat) {
      this.errorFormat = errorFormat;
      return this;
    }

    private String jsonWarningsFile = "";

    @CanIgnoreReturnValue
    public CommandLineConfig setJsonWarningsFile(String jsonWarningsFile) {
      this.jsonWarningsFile = jsonWarningsFile;
      return this;
    }

    private boolean includeTrailingNewline = true;

    @CanIgnoreReturnValue
    public CommandLineConfig setIncludeTrailingNewline(boolean includeTrailingNewline) {
      this.includeTrailingNewline = includeTrailingNewline;
      return this;
    }
  }

  /** Representation of a source file from an encoded json stream input */
  public static class JsonFileSpec {
    private final Optional<String> src;
    private final Optional<String> path;

    private Optional<String> sourceMap = Optional.empty();
    private final Optional<String> webpackId;

    /**
     * Reads one file spec from a JSON object. Unknown properties are skipped; "sourceMap" and
     * "webpackId" are accepted as alternate names of "source_map" and "webpack_id".
     */
    private static Outcome<JsonFileSpec> readJson(SimpleJsonValue object) {
      if (object.kind() != SimpleJsonValue.Kind.OBJECT) {
        return Outcome.err(new SimpleJsonException("Malformed JSON stream: expected an object"));
      }
      Optional<String> src = Optional.empty();
      Optional<String> path = Optional.empty();
      Optional<String> sourceMap = Optional.empty();
      Optional<String> webpackId = Optional.empty();
      for (Map.Entry<String, SimpleJsonValue> member : object.members().entrySet()) {
        String name = member.getKey();
        boolean known =
            switch (name) {
              case "src", "path", "source_map", "sourceMap", "webpack_id", "webpackId" -> true;
              default -> false;
            };
        if (!known) {
          continue;
        }
        Outcome<@Nullable String> value = readJsonString(member.getValue());
        if (value.isErr()) {
          return Outcome.err(value.error());
        }
        switch (name) {
          case "src" -> src = Optional.ofNullable(value.get());
          case "path" -> path = Optional.ofNullable(value.get());
          case "source_map", "sourceMap" -> sourceMap = Optional.ofNullable(value.get());
          default -> webpackId = Optional.ofNullable(value.get());
        }
      }
      if (src.isEmpty() || path.isEmpty()) {
        return Outcome.err(
            new SimpleJsonException("Malformed JSON stream: a file needs \"src\" and \"path\""));
      }
      return Outcome.ok(new JsonFileSpec(src, path, sourceMap, webpackId));
    }

    /** Reads a string or null; a boolean or a number is read as its text. */
    private static Outcome<@Nullable String> readJsonString(SimpleJsonValue value) {
      if (value.kind() == SimpleJsonValue.Kind.NULL) {
        return Outcome.ok(null);
      }
      return value.asString();
    }

    /** Writes this file spec as an object; a property whose value is null is left out. */
    private void writeJson(SimpleJsonWriter writer) {
      writer.beginObject();
      writer.name("src").stringValue(this.src);
      writer.name("path").stringValue(this.path);
      writer.name("source_map").stringValue(this.sourceMap);
      writer.name("webpack_id").stringValue(this.webpackId);
      writer.endObject();
    }

    public JsonFileSpec(String src, String path) {
      this(Optional.of(src), Optional.of(path), Optional.empty(), Optional.empty());
    }

    public JsonFileSpec(String src, String path, String sourceMap) {
      this(Optional.of(src), Optional.of(path), Optional.of(sourceMap), Optional.empty());
    }

    public JsonFileSpec(
        Optional<String> src,
        Optional<String> path,
        Optional<String> sourceMap,
        Optional<String> webpackId) {
      this.src = src;
      this.path = path;
      this.sourceMap = sourceMap;
      this.webpackId = webpackId;
    }

    public Optional<String> getSrc() {
      return this.src;
    }

    public Optional<String> getPath() {
      return this.path;
    }

    public Optional<String> getSourceMap() {
      return this.sourceMap;
    }

    public Optional<String> getWebpackId() {
      return this.webpackId;
    }

    public void setSourceMap(String map) {
      this.sourceMap = Optional.of(map);
    }
  }

  /** Flag types for JavaScript source files. */
  protected enum JsSourceType {
    EXTERN("extern"),
    JS("js"),
    JS_ZIP("jszip"),
    WEAKDEP("weakdep"),
    IJS("ijs");

    @VisibleForTesting final String flagName;

    private JsSourceType(String flagName) {
      this.flagName = flagName;
    }
  }

  /** A pair from flag to its value. */
  protected static class FlagEntry<T> {
    private final T flag;
    private final String value;

    protected FlagEntry(T flag, String value) {
      this.flag = flag;
      this.value = value;
    }

    @Override
    public boolean equals(Object o) {
      if (o instanceof FlagEntry) {
        FlagEntry<?> that = (FlagEntry<?>) o;
        return that.flag.equals(this.flag) && that.value.equals(this.value);
      }
      return false;
    }

    @Override
    public int hashCode() {
      return flag.hashCode() + value.hashCode();
    }

    @Override
    public String toString() {
      return this.flag + "=" + this.value;
    }

    public T getFlag() {
      return flag;
    }

    public String getValue() {
      return value;
    }
  }

  /** Represents a specification for a serving chunk. */
  public static class JsChunkSpec {
    private final String name;
    // Number of input files, including js and zip files.
    private final int numInputs;
    private final ImmutableList<String> deps;
    // Number of input js files. All zip files should be expanded.
    private int numJsFiles;

    private JsChunkSpec(String name, int numInputs, ImmutableList<String> deps) {
      this.name = name;
      this.numInputs = numInputs;
      this.deps = deps;
      this.numJsFiles = numInputs;
    }

    /**
     * Returns a parsed chunk spec.
     *
     * @param specString The spec format is: <code>name:num-js-files[:[dep,...][:]]</code>. Chunk
     *     names must not contain the ':' character.
     * @param isFirstChunk Whether the spec is for the first chunk.
     */
    public static Outcome<JsChunkSpec> create(String specString, boolean isFirstChunk) {
      // Format is "<name>:<num-js-files>[:[<dep>,...][:]]".
      String[] parts = COLON_PATTERN.split(specString);
      if (parts.length < 2 || parts.length > 4) {
        return Outcome.err(new FlagUsageException(
            "Expected 2-4 colon-delimited parts in " + "chunk spec: " + specString));
      }

      // Parse chunk name.
      String name = parts[0];

      // Parse chunk dependencies.
      String[] deps =
          parts.length > 2 && parts[2].length() > 0 ? COMMA_PATTERN.split(parts[2]) : new String[0];

      // Parse chunk inputs.
      int numInputs = -1;
      Integer parsedNumInputs = JSCompStrings.parseIntOrNull(Optional.of(parts[1]));
      if (parsedNumInputs != null) {
        numInputs = parsedNumInputs;
      }

      // We will allow chunks of zero input.
      if (numInputs < 0) {
        // A size of 'auto' is only allowed on the base chunk if
        // and it must also be the first chunk
        if (parts.length == 2 && "auto".equals(parts[1])) {
          if (!isFirstChunk) {
            return Outcome.err(new FlagUsageException(
                "Invalid JS file count '"
                    + parts[1]
                    + "' for chunk: "
                    + name
                    + ". Only the first chunk may specify "
                    + "a size of 'auto' and it must have no dependencies."));
          }
        } else {
          return Outcome.err(new FlagUsageException(
              "Invalid JS file count '" + parts[1] + "' for chunk: " + name));
        }
      }

      return Outcome.ok(new JsChunkSpec(name, numInputs, ImmutableList.copyOf(deps)));
    }

    public String getName() {
      return name;
    }

    public int getNumInputs() {
      return numInputs;
    }

    public ImmutableList<String> getDeps() {
      return deps;
    }

    public int getNumJsFiles() {
      return numJsFiles;
    }
  }

  static final class SystemExitCodeReceiver implements Function<Integer, Void> {
    static final SystemExitCodeReceiver INSTANCE = new SystemExitCodeReceiver();

    private SystemExitCodeReceiver() {
      // singleton
    }

    @Override
    public Void apply(Integer exitCode) {
      int exitCodeValue = checkNotNull(exitCode);
      // Don't spuriously report success.
      // Posix conventions only guarantee that 8b are significant.
      byte exitCodeByte = (byte) exitCodeValue;
      if (exitCodeByte == 0 && exitCodeValue != 0) {
        exitCodeByte = (byte) -1;
      }
      System.exit(exitCodeByte);
      return null;
    }
  }
}
