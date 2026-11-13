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

package com.google.javascript.jscomp;

import com.google.common.base.Ascii;
import com.google.common.base.Strings;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableSet;
import com.google.javascript.jscomp.base.Charsets;
import com.google.javascript.jscomp.base.FilePaths;
import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.JSCompStrings;
import com.google.javascript.jscomp.base.Outcome;
import com.google.re2j.Pattern;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.function.Consumer;
import java.util.function.Predicate;
import java.util.function.Supplier;
import org.jspecify.annotations.Nullable;

/**
 * A command-line parser driven by an explicit table of flags.
 *
 * <p>Each flag is declared with {@link #option}, followed by its attributes and finally by the
 * kind of value it takes together with a getter and a setter for the variable that holds the
 * value:
 *
 * <pre>
 * parser
 *     .option("--level")
 *     .aliases("-L")
 *     .usage("How hard to try")
 *     .integer(() -> level, v -> level = v);
 * </pre>
 *
 * <p>The syntax accepted by {@link #parseArgument}, the error messages and the text printed by
 * {@link #printUsage} are those of args4j 2.33 with its default settings, which this class
 * replaces.
 */
public final class CmdLineParser {
  private static final Pattern NEWLINE_PATTERN = Pattern.compile("\\n");


  /** The command-line tokens, as seen by the flag that is being parsed. */
  private interface Parameters {
    /**
     * Returns the token at the given offset: 0 is the first token after the name of the flag.
     *
     * <p>Returns an error outcome with a CmdLineException if there is no such token.
     */
    Outcome<String> getParameter(int idx);

    /** Returns the token at the given offset, or null if there is no such token. */
    Optional<String> getParameterOrNull(int idx);
  }

  /** Parses the value of one occurrence of a flag. */
  private interface Handler {
    /** Stores the value and returns the number of tokens that made it up. */
    Outcome<Integer> parseArguments(Parameters params);
  }

  /** One flag, or the positional arguments. */
  public static final class Option {
    private final CmdLineParser owner;
    private final String nameField;
    private final boolean isArgument;
    private ImmutableList<String> aliasesField = ImmutableList.of();
    private String usageField = "";
    private boolean hiddenField = false;
    private boolean requiredField = false;
    // What follows the name of the flag in the usage text, if anything.
    private Optional<String> metaVariable = Optional.empty();
    // The current value(s), for the "(default: ...)" part of the usage text, if it is shown.
    private @Nullable Supplier<@Nullable List<?>> defaultValues = null;
    private @Nullable Handler handler = null;

    private Option(CmdLineParser owner, String name, boolean isArgument) {
      this.owner = owner;
      this.nameField = name;
      this.isArgument = isArgument;
    }

    public String name() {
      return nameField;
    }

    public boolean hidden() {
      return hiddenField;
    }

    /** Other names of this flag. */
    public Option aliases(String... aliases) {
      for (String alias : aliases) {
        owner.checkOptionNotDefined(alias);
      }
      this.aliasesField = ImmutableList.copyOf(aliases);
      owner.optionsSorted = false;
      return this;
    }

    /** The description shown by {@code printUsage}; a flag without one is never shown. */
    public Option usage(String usage) {
      this.usageField = usage;
      return this;
    }

    public Option hidden(boolean hidden) {
      this.hiddenField = hidden;
      return this;
    }

    /** Makes it an error to leave this flag out. */
    public Option required(boolean required) {
      this.requiredField = required;
      return this;
    }

    /** A flag that takes one argument. */
    public void string(Supplier<@Nullable String> get, Consumer<String> set) {
      this.metaVariable = Optional.of("VAL");
      this.defaultValues = () -> asList(get.get());
      this.handler =
          (Parameters params) -> {
            Outcome<String> value = params.getParameter(0);
            if (value.isErr()) {
              return Outcome.err(value.error());
            }
            set.accept(value.get());
            return Outcome.ok(1);
          };
    }

    /** A flag that takes one argument and may be given several times. */
    public void stringList(Supplier<@Nullable List<String>> get, Consumer<List<String>> set) {
      this.metaVariable = Optional.of("VAL");
      this.defaultValues = () -> get.get();
      this.handler =
          (Parameters params) -> {
            Outcome<String> value = params.getParameter(0);
            if (value.isErr()) {
              return Outcome.err(value.error());
            }
            addValue(get, set, value.get());
            return Outcome.ok(1);
          };
    }

    /**
     * A flag that takes one argument and may be given several times; its values are not shown in
     * the usage text.
     */
    public void stringEach(Consumer<String> add) {
      this.metaVariable = Optional.of("VAL");
      this.defaultValues = null;
      this.handler =
          (Parameters params) -> {
            Outcome<String> value = params.getParameter(0);
            if (value.isErr()) {
              return Outcome.err(value.error());
            }
            add.accept(value.get());
            return Outcome.ok(1);
          };
    }

    /** A flag that takes one integer argument. */
    public void integer(Supplier<@Nullable Integer> get, Consumer<Integer> set) {
      this.metaVariable = Optional.of("N");
      this.defaultValues = () -> asList(get.get());
      this.handler =
          (Parameters params) -> {
            Outcome<String> tokenOutcome = params.getParameter(0);
            if (tokenOutcome.isErr()) {
              return Outcome.err(tokenOutcome.error());
            }
            String token = tokenOutcome.get();
            Integer value = JSCompStrings.parseIntOrNull(Optional.of(token));
            if (value == null) {
              Outcome<String> flagName = params.getParameter(-1);
              if (flagName.isErr()) {
                return Outcome.err(flagName.error());
              }
              return Outcome.err(
                  new CmdLineException(owner, illegalOperand(flagName.get(), token)));
            }
            set.accept(value);
            return Outcome.ok(1);
          };
    }

    /** A flag that takes no argument and sets the value to true. */
    public void flag(Supplier<@Nullable Boolean> get, Consumer<Boolean> set) {
      this.metaVariable = Optional.empty();
      this.defaultValues = () -> asList(get.get());
      this.handler =
          (Parameters params) -> {
            set.accept(true);
            return Outcome.ok(0);
          };
    }

    /**
     * A flag that sets the value to true, or to the value of the next token if that is one of
     * true, on, yes, 1, false, off, no and 0 in any letter case.
     */
    public void bool(Supplier<@Nullable Boolean> get, Consumer<Boolean> set) {
      this.metaVariable = Optional.empty();
      this.defaultValues = () -> asList(get.get());
      this.handler =
          (Parameters params) -> {
            Optional<String> param = params.getParameterOrNull(0);

            if (param.isEmpty()) {
              set.accept(true);
              return Outcome.ok(0);
            } else {
              String lowerParam = Ascii.toLowerCase(param.get());
              if (TRUES.contains(lowerParam)) {
                set.accept(true);
              } else if (FALSES.contains(lowerParam)) {
                set.accept(false);
              } else {
                set.accept(true);
                return Outcome.ok(0);
              }
              return Outcome.ok(1);
            }
          };
    }

    /**
     * A flag that takes the name of one of the given enum values as its argument; letter case is
     * ignored and '-' may be written for '_'.
     */
    public <E extends Enum<E>> void enumValue(
        E[] values, Supplier<@Nullable E> get, Consumer<E> set) {
      this.metaVariable = Optional.of(enumMetaVariable(values));
      this.defaultValues = () -> asList(get.get());
      this.handler =
          (Parameters params) -> {
            Outcome<E> value = parseEnum(values, params);
            if (value.isErr()) {
              return Outcome.err(value.error());
            }
            set.accept(value.get());
            return Outcome.ok(1);
          };
    }

    /** Like {@link #enumValue}, for a flag that may be given several times. */
    public <E extends Enum<E>> void enumList(
        E[] values, Supplier<@Nullable List<E>> get, Consumer<List<E>> set) {
      this.metaVariable = Optional.of(enumMetaVariable(values));
      this.defaultValues = () -> get.get();
      this.handler =
          (Parameters params) -> {
            Outcome<E> value = parseEnum(values, params);
            if (value.isErr()) {
              return Outcome.err(value.error());
            }
            addValue(get, set, value.get());
            return Outcome.ok(1);
          };
    }

    /**
     * A flag that takes an argument of the form key=value and may be given several times. A value
     * that is empty is stored as null.
     */
    public void map(
        Supplier<@Nullable Map<String, String>> get, Consumer<Map<String, String>> set) {
      this.metaVariable = Optional.empty();
      this.defaultValues = () -> asList(get.get());
      this.handler =
          (Parameters params) -> {
            Map<String, String> map = get.get();
            if (map == null) {
              map = new LinkedHashMap<>();
              set.accept(map);
            }
            Outcome<String> argumentOutcome = params.getParameter(0);
            if (argumentOutcome.isErr()) {
              return Outcome.err(argumentOutcome.error());
            }
            String argument = argumentOutcome.get();
            int idx = argument.indexOf('=');
            if (idx == -1) {
              return Outcome.err(
                  new CmdLineException(
                      owner, "An argument for setting a Map must contain a \"=\""));
            }
            String mapKey = argument.substring(0, idx);
            Optional<String> mapValue = Optional.of(argument.substring(idx + 1));
            if (mapValue.get().length() == 0) {
              mapValue = Optional.empty();
            }
            if (mapKey.length() == 0) {
              return Outcome.err(new CmdLineException(owner, "A key must be set."));
            }
            map.put(mapKey, mapValue.orElse(null));
            return Outcome.ok(1);
          };
    }

    private <E extends Enum<E>> Outcome<E> parseEnum(E[] values, Parameters params) {
      Outcome<String> token = params.getParameter(0);
      if (token.isErr()) {
        return Outcome.err(token.error());
      }
      String s = token.get().replace('-', '_');
      for (E value : values) {
        if (value.name().equalsIgnoreCase(s)) {
          return Outcome.ok(value);
        }
      }
      Outcome<String> flagName = params.getParameter(-1);
      if (flagName.isErr()) {
        return Outcome.err(flagName.error());
      }
      return Outcome.err(new CmdLineException(owner, illegalOperand(flagName.get(), s)));
    }

    /** Returns all the values separated by pipes: {@code [ONE | TWO | THREE]}. */
    private static <E extends Enum<E>> String enumMetaVariable(E[] values) {
      StringBuilder rv = new StringBuilder();
      rv.append("[");
      for (E value : values) {
        rv.append(value).append(" | ");
      }
      rv.delete(rv.length() - 3, rv.length());
      rv.append("]");
      return rv.toString();
    }

    private static <T> void addValue(
        Supplier<@Nullable List<T>> get, Consumer<List<T>> set, T value) {
      List<T> list = get.get();
      if (list == null) {
        list = new ArrayList<>();
        set.accept(list);
      }
      list.add(value);
    }

    private static @Nullable List<?> asList(@Nullable Object value) {
      if (value != null) {
        return Collections.singletonList(value);
      }
      return Collections.emptyList();
    }

    private static String illegalOperand(String optionName, String value) {
      return "\"" + value + "\" is not a valid value for \"" + optionName + "\"";
    }

    /** Returns the name with its aliases: {@code --name (-a, -b)}. */
    @Override
    public String toString() {
      if (isArgument) {
        return "";
      }
      if (!aliasesField.isEmpty()) {
        return nameField + " (" + String.join(", ", aliasesField) + ")";
      }
      return nameField;
    }

    private String getNameAndMeta() {
      String str = toString();
      if (metaVariable.isPresent()) {
        if (str.length() > 0) {
          str += " ";
        }
        str += metaVariable.get();
      }
      return str;
    }

    private Optional<String> printDefaultValue() {
      List<?> values = defaultValues.get();
      if (values != null && !values.isEmpty()) {
        StringBuilder buf = new StringBuilder();
        if (values.size() > 1) {
          for (Object v : values) {
            buf.append(buf.length() == 0 ? '[' : ',');
            buf.append(String.valueOf(v));
          }
          buf.append(']');
        } else {
          buf.append(String.valueOf(values.get(0)));
        }
        return Optional.of(buf.toString());
      }
      return Optional.empty();
    }
  }

  private static final ImmutableSet<String> TRUES = ImmutableSet.of("true", "on", "yes", "1");
  private static final ImmutableSet<String> FALSES = ImmutableSet.of("false", "off", "no", "0");

  // Sorted by name before they are used; see sortedOptions().
  private final List<Option> options = new ArrayList<>();
  private boolean optionsSorted = true;
  private @Nullable Option argumentField = null;
  private int usageWidth = 80;
  private @Nullable Option currentOption = null;

  public CmdLineParser() {}

  /** Declares a flag. The call must be completed with the kind of value the flag takes. */
  public Option option(String name) {
    checkOptionNotDefined(name);
    Option option = new Option(this, name, /* isArgument= */ false);
    options.add(option);
    optionsSorted = false;
    return option;
  }

  /** Declares that tokens which do not belong to a flag are allowed, and where they are stored. */
  public void argument(Supplier<@Nullable List<String>> get, Consumer<List<String>> set) {
    if (argumentField != null) {
      throw new IllegalStateException("Argument with index 0 is used more than once");
    }
    argumentField = new Option(this, "", /* isArgument= */ true);
    argumentField.stringList(get, set);
  }

  private void checkOptionNotDefined(String name) {
    if (findOptionByName(name) != null) {
      throw new IllegalStateException("Option name " + name + " is used more than once");
    }
  }

  /** Returns the flags in the order in which they are printed and checked. */
  private List<Option> sortedOptions() {
    if (!optionsSorted) {
      Collections.sort(options, (Option o1, Option o2) -> o1.toString().compareTo(o2.toString()));
      optionsSorted = true;
    }
    return options;
  }

  /** Sets the number of columns that the usage text is wrapped to. */
  public void setUsageWidth(int usageWidth) {
    if (usageWidth < 0) {
      throw new IllegalArgumentException("Usage width is negative");
    }
    this.usageWidth = usageWidth;
  }

  /**
   * Prints one entry for each flag that has a usage text and is selected by the filter, in the
   * order of their names.
   */
  public void printUsage(StringBuilder out, Predicate<Option> filter) {
    StringBuilder w = out;
    // determine the length of the option + metavar first
    int len = 0;
    for (Option option : sortedOptions()) {
      int curLen = option.usageField.length() == 0 ? 0 : option.getNameAndMeta().length();
      len = Math.max(len, curLen);
    }

    // then print
    for (Option option : sortedOptions()) {
      printOption(w, option, len, filter);
    }
  }

  private void printOption(StringBuilder out, Option option, int len, Predicate<Option> filter) {
    // Hiding options without usage information
    if (option.usageField.length() == 0 || !filter.test(option)) {
      return;
    }

    // What is the width of the two data columns
    int widthMetadata = Math.min(len, (usageWidth - 4) / 2);
    int widthUsage = usageWidth - 4 - widthMetadata;

    String defaultValuePart = createDefaultValuePart(option);

    // Line wrapping
    // the 'left' side
    List<String> namesAndMetas = wrapLines(option.getNameAndMeta(), widthMetadata);
    // the 'right' side
    List<String> usages = wrapLines(option.usageField + defaultValuePart, widthUsage);

    // Output
    for (int i = 0; i < Math.max(namesAndMetas.size(), usages.size()); i++) {
      String nameAndMeta = (i >= namesAndMetas.size()) ? "" : namesAndMetas.get(i);
      String usage = (i >= usages.size()) ? "" : usages.get(i);
      String separator = ((nameAndMeta.length() > 0) && (i == 0)) ? " : " : "   ";
      out.append(
              " "
                  + Strings.padEnd(nameAndMeta, widthMetadata, ' ')
                  + separator
                  + Strings.padEnd(usage, 1, ' '))
          .append(System.lineSeparator());
    }
  }

  private static String createDefaultValuePart(Option option) {
    if (!option.requiredField && option.defaultValues != null) {
      Optional<String> v = option.printDefaultValue();
      if (v.isPresent()) {
        return " (default: " + v.get() + ")";
      }
    }
    return "";
  }

  /** Wraps a line so that the resulting parts are no longer than a given maximum length. */
  private static List<String> wrapLines(String line, final int maxLength) {
    List<String> rv = new ArrayList<>();
    for (String restOfLine : NEWLINE_PATTERN.split(line)) {
      while (restOfLine.length() > maxLength) {
        // try to wrap at space, but don't try too hard as some languages don't even have
        // whitespaces.
        int lineLength;
        String candidate = restOfLine.substring(0, maxLength);
        int sp = candidate.lastIndexOf(' ');
        if (sp > maxLength * 3 / 5) {
          lineLength = sp;
        } else {
          lineLength = maxLength;
        }
        rv.add(restOfLine.substring(0, lineLength));
        restOfLine = restOfLine.substring(lineLength).trim();
      }
      rv.add(restOfLine);
    }
    return rv;
  }

  /** The tokens that have not been consumed yet. */
  private final class CmdLineImpl implements Parameters {
    private final String[] args;
    private int pos;

    CmdLineImpl(String[] args) {
      this.args = args;
      pos = 0;
    }

    boolean hasMore() {
      return pos < args.length;
    }

    String getCurrentToken() {
      return args[pos];
    }

    void proceed(int n) {
      pos += n;
    }

    @Override
    public Outcome<String> getParameter(int idx) {
      if (pos + idx >= args.length || pos + idx < 0) {
        return Outcome.err(
            new CmdLineException(
                CmdLineParser.this, "Option \"" + currentOption + "\" takes an operand"));
      }
      return Outcome.ok(args[pos + idx]);
    }

    @Override
    public Optional<String> getParameterOrNull(int idx) {
      if (pos + idx >= args.length || pos + idx < 0) {
        return Optional.empty();
      }
      return Optional.of(args[pos + idx]);
    }

    /** Replaces the current token {@code name=value} by {@code value}. */
    void splitToken() {
      if (pos < args.length && pos >= 0) {
        int idx = args[pos].indexOf("=");
        if (idx > 0) {
          args[pos] = args[pos].substring(idx + 1);
        }
      }
    }
  }

  public Outcome<Void> parseArgumentCollection(Collection<String> args) {
    return parseArgument(args.toArray(new String[0]));
  }

  /**
   * Parses the given command-line tokens and stores the values of the flags.
   *
   * <p>A token that starts with '-' is the name of a flag, optionally followed by '=' and the
   * value. A token that starts with '@' stands for the lines of the file it names.
   *
   * <p>Returns an error outcome with a CmdLineException if the tokens are not a valid command
   * line; the values of the flags seen before the error have been stored.
   */
  public Outcome<Void> parseArgument(final String... args) {
    Outcome<String[]> expandedArgs = expandAtFiles(args);
    if (expandedArgs.isErr()) {
      return Outcome.err(expandedArgs.error());
    }
    CmdLineImpl cmdLine = new CmdLineImpl(expandedArgs.get());

    Set<Option> present = new LinkedHashSet<>();

    while (cmdLine.hasMore()) {
      String arg = cmdLine.getCurrentToken();
      if (arg.startsWith("-")) {
        // '=' is for historical compatibility fallback
        boolean isKeyValuePair = arg.contains(" ") || arg.indexOf('=') != -1;

        // parse this as an option.
        currentOption = isKeyValuePair ? findOptionOfKeyValuePair(arg) : findOptionByName(arg);

        if (currentOption == null) {
          return Outcome.err(
              new CmdLineException(this, "\"" + arg + "\" is not a valid option"));
        }

        // known option; skip its name
        if (isKeyValuePair) {
          cmdLine.splitToken();
        } else {
          cmdLine.proceed(1);
        }
      } else {
        if (argumentField == null) {
          return Outcome.err(new CmdLineException(this, "No argument is allowed: " + arg));
        }

        // known argument
        currentOption = argumentField;
      }
      Outcome<Integer> diff = currentOption.handler.parseArguments(cmdLine);
      if (diff.isErr()) {
        return Outcome.err(diff.error());
      }
      cmdLine.proceed(diff.get());
      present.add(currentOption);
    }

    // make sure that all mandatory options are present
    for (Option option : sortedOptions()) {
      if (option.requiredField && !present.contains(option)) {
        return Outcome.err(
            new CmdLineException(this, "Option \"" + option + "\" is required"));
      }
    }
    return Outcome.ok();
  }

  /** Expands every token of the form {@code @file} to the lines of that file. */
  private Outcome<String[]> expandAtFiles(String[] args) {
    List<String> result = new ArrayList<>();
    for (String arg : args) {
      if (arg.startsWith("@")) {
        String file = FilePaths.clean(arg.substring(1));
        if (!Io.exists(file)) {
          return Outcome.err(new CmdLineException(this, "No such file: " + file));
        }
        Outcome<String> text = Io.readText(file, Charsets.UTF_8);
        if (text.isErr()) {
          return Outcome.err(new CmdLineException(this, "Failed to parse " + file, text.error()));
        }
        result.addAll(Io.splitLines(text.get()));
      } else {
        result.add(arg);
      }
    }
    return Outcome.ok(result.toArray(new String[0]));
  }

  private @Nullable Option findOptionOfKeyValuePair(String name) {
    // Look for key/value pair first.
    int pos = name.indexOf(" ");
    if (pos < 0) {
      pos = name.indexOf('='); // historical compatibility fallback
    }
    if (pos > 0) {
      name = name.substring(0, pos);
    }
    return findOptionByName(name);
  }

  private @Nullable Option findOptionByName(String name) {
    for (Option option : options) {
      if (name.equals(option.nameField)) {
        return option;
      }
      for (String alias : option.aliasesField) {
        if (name.equals(alias)) {
          return option;
        }
      }
    }
    return null;
  }
}
